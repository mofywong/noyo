package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"noyo/core/store"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// conflictRecord is stored on an entity whose local and peer versions diverged.
// Both versions are preserved so an operator can arbitrate without losing data;
// nothing is ever overwritten silently.
type conflictRecord struct {
	Op   string `json:"op"`
	Node string `json:"node"`
	Rev  int64  `json:"rev"`
	At   int64  `json:"at"`
	// Payload holds the peer's proposed entity. It is absent when the peer's
	// proposal was a deletion, which lets the UI distinguish "peer wants to
	// change this" from "peer wants to remove this".
	Payload json.RawMessage `json:"payload,omitempty"`
}

// syncApplyHooks lets an engine add role-specific behaviour to a replay.
type syncApplyHooks struct {
	// beforeApply vets a change before it is applied. Returning false records
	// the change as skipped, which still acknowledges it: a change that can
	// never apply must not block the peer's cursor.
	beforeApply func(ch store.SyncChange) bool
	// authorizeApply validates the sender's resource scope in the replay
	// transaction. Invalid changes are consumed as skipped; database errors are
	// retried rather than silently losing a legitimate change.
	authorizeApply func(tx *gorm.DB, ch store.SyncChange) (bool, error)
	// deferApply temporarily rejects the change without consuming its cursor.
	// It runs in the same transaction as replay and the idempotency marker.
	deferApply    func(tx *gorm.DB, ch store.SyncChange) (bool, error)
	localNode     string
	localInstance string
	preferRemote  bool
	// beforeMutationTx captures receiver state after authorization, before any
	// lifecycle reconciliation or business-row mutation.
	beforeMutationTx func(tx *gorm.DB, change *store.SyncChange) error
	// afterApplyTx records dependent changes in the same transaction as the
	// replay. It must not publish until after applySyncChange commits.
	afterApplyTx func(tx *gorm.DB, applied store.SyncChange, status string) error
	// Dependencies become visible to the runtime before the referencing resource.
	afterDependencies func(committed []store.SyncChange)
	// afterApply runs once per change that was not skipped. The platform uses
	// it to relay shared entities to the other gateways in scope; on the
	// gateway there is nothing to relay.
	afterApply func(applied store.SyncChange, status string)
}

// applySyncChange replays one change received from a peer and returns the status
// reported back in the acknowledgement.
//
// The replay and its idempotency marker are committed in a single transaction,
// so a crash mid-apply can never leave state applied without the marker (which
// would re-apply on retry) or the marker without the state (which would drop
// the change silently).
func applySyncChange(nodeID string, hooks syncApplyHooks, logger *zap.Logger, ch store.SyncChange) string {
	changeID := ch.ChangeID()
	if changeID == ":" || ch.OriginNode == "" {
		logSyncIssue(logger, "rejecting change with no origin node", ch, nil)
		return syncStatusRejected
	}
	if ch.OriginNode == nodeID {
		// A change produced by this node must never be replayed here; that would
		// be an echo of our own write and would corrupt the revision chain.
		logSyncIssue(logger, "ignoring reflected change originated by this node", ch, nil)
		return syncStatusSkipped
	}

	applied, err := store.IsChangeApplied(changeID)
	if err != nil {
		logSyncIssue(logger, "failed to read idempotency marker", ch, err)
		return syncStatusRejected
	}
	if applied {
		status, err := store.GetAppliedChangeStatus(changeID)
		if err != nil {
			logSyncIssue(logger, "failed to read applied change outcome", ch, err)
			return syncStatusRejected
		}
		if status != "" {
			return status
		}
		return syncStatusSkipped
	}
	if hooks.beforeApply != nil && !hooks.beforeApply(ch) {
		// Consume the change without applying it. Marking it applied keeps the
		// peer's cursor moving; leaving it unmarked would make the same rejected
		// change arrive again on every handshake.
		if err := store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
			if _, err := store.MarkChangeAppliedTx(tx, changeID); err != nil {
				return err
			}
			return store.SetAppliedChangeStatusTx(tx, changeID, syncStatusSkipped)
		}); err != nil {
			logSyncIssue(logger, "failed to consume a rejected change", ch, err)
			return syncStatusRejected
		}
		return syncStatusSkipped
	}
	status := syncStatusApplied
	var committedDependencies []store.SyncChange
	err = store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		// Keep the replay context on a clean baseline so hooks and entity
		// queries cannot leak their model or WHERE clauses into later queries.
		tx = tx.Set(syncLocalNodeKey, hooks.localNode).Set(syncLocalInstanceKey, hooks.localInstance).
			Set(syncPreferRemoteKey, hooks.preferRemote).Session(&gorm.Session{})
		if hooks.deferApply != nil {
			deferred, err := hooks.deferApply(tx, ch)
			if err != nil {
				return err
			}
			if deferred {
				status = syncStatusRejected
				return nil
			}
		}
		if hooks.authorizeApply != nil {
			allowed, err := hooks.authorizeApply(tx, ch)
			if err != nil {
				return err
			}
			if !allowed {
				status = syncStatusSkipped
				if _, err := store.MarkChangeAppliedTx(tx, changeID); err != nil {
					return err
				}
				return store.SetAppliedChangeStatusTx(tx, changeID, status)
			}
		}
		if err := validateSyncRequest(ch); err != nil {
			return err
		}
		ch, err = normalizeDeviceTopology(nodeID, ch)
		if err != nil {
			return err
		}
		currentLifecycle, err := store.CheckRemoteResourceLifecycleTx(tx, ch)
		if err != nil {
			return err
		}
		if !currentLifecycle {
			status = syncStatusSkipped
			if _, err := store.MarkChangeAppliedTx(tx, changeID); err != nil {
				return err
			}
			return store.SetAppliedChangeStatusTx(tx, changeID, status)
		}
		lineage, err := resolveIntentPredecessorTx(tx, &ch)
		if err != nil {
			return err
		}
		dependencies, err := validateConfigDependenciesTx(tx, ch)
		if err != nil {
			return err
		}
		fresh, err := store.MarkChangeAppliedTx(tx, changeID)
		if err != nil {
			return err
		}
		if !fresh {
			status = syncStatusSkipped
			return nil
		}
		if hooks.beforeMutationTx != nil {
			if err := hooks.beforeMutationTx(tx, &ch); err != nil {
				return err
			}
		}
		handled, resolved, err := resolveNewerPlatformLifecycleTx(tx, ch)
		if err != nil {
			return err
		}
		if !handled {
			resolved, err = applyChangeTx(tx, ch)
		}
		if err != nil {
			return err
		}
		status = resolved
		if status == syncStatusApplied {
			if err := store.AcceptRemoteResourceLifecycleTx(tx, ch); err != nil {
				return err
			}
			for _, dependency := range dependencies {
				outcome, err := applyChangeTx(tx, dependency)
				if err != nil {
					return err
				}
				if outcome != syncStatusApplied {
					return invalidSync("DEPENDENCY_CHANGED", "configuration dependency %s %s changed during apply", dependency.EntityType, dependency.EntityCode)
				}
				if err := store.AcceptRemoteResourceLifecycleTx(tx, dependency); err != nil {
					return err
				}
				commit, err := commitDependencyTx(tx, dependency, hooks)
				if err != nil {
					return err
				}
				committedDependencies = append(committedDependencies, commit)
			}
		}
		if hooks.afterApplyTx != nil && status != syncStatusSkipped {
			if err := hooks.afterApplyTx(tx, ch, status); err != nil {
				return err
			}
		}
		if err := recordIntentResultTx(tx, ch, lineage, status); err != nil {
			return err
		}
		return store.SetAppliedChangeStatusTx(tx, changeID, status)
	})
	if err != nil {
		logSyncIssue(logger, "failed to apply sync change", ch, err)
		var failure *syncValidationError
		if errors.As(err, &failure) {
			failedStatus, ledgerErr := recordSyncBusinessFailure(ch, failure)
			if ledgerErr == nil {
				return failedStatus
			}
			logSyncIssue(logger, "failed to persist sync failure", ch, ledgerErr)
		}
		return syncStatusRejected
	}

	if len(committedDependencies) > 0 && hooks.afterDependencies != nil {
		hooks.afterDependencies(committedDependencies)
	}
	if status != syncStatusSkipped && hooks.afterApply != nil {
		hooks.afterApply(ch, status)
	}
	return status
}

// Gateway roots are local connections on the edge and children of the gateway
// on the platform. Normalize before comparison as well as persistence so a
// round trip does not create a spurious revision conflict.
func normalizeDeviceTopology(nodeID string, ch store.SyncChange) (store.SyncChange, error) {
	if ch.EntityType != store.SyncEntityDevice || ch.Op == store.SyncOpDelete || ch.Payload == "" {
		return ch, nil
	}
	var device store.Device
	if err := json.Unmarshal([]byte(ch.Payload), &device); err != nil {
		return ch, err
	}
	if nodeID == cloudSyncNodeID {
		if device.Code != ch.OriginNode {
			if device.OwnerNode == "" {
				device.OwnerNode = ch.OriginNode
			}
			if device.ParentCode == "" {
				device.ParentCode = device.OwnerNode
			}
		}
	} else if device.ParentCode == nodeID {
		device.ParentCode = ""
	}
	payload, err := json.Marshal(&device)
	if err != nil {
		return ch, err
	}
	ch.Payload = string(payload)
	return ch, nil
}

func applyChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	switch ch.EntityType {
	case store.SyncEntityPluginConfig, store.SyncEntityNodeConfig:
		return applyProtectedConfigChangeTx(tx, ch)
	case store.SyncEntityDevice:
		return applyDeviceChangeTx(tx, ch)
	case store.SyncEntityProduct:
		return applyProductChangeTx(tx, ch)
	case store.SyncEntityDriver:
		return applyDriverChangeTx(tx, ch)
	case store.SyncEntityRule:
		return applyRuleChangeTx(tx, ch)
	case store.SyncEntityRuleGroup:
		return applyRuleGroupChangeTx(tx, ch)
	default:
		return syncStatusRejected, fmt.Errorf("unsupported sync entity type %q", ch.EntityType)
	}
}

// syncReplaySession clears query clauses without losing the replay context.
// GORM's NewDB mode discards Statement.Settings on the next operation, which
// otherwise makes site commits look like local edits and loses reconciliation
// identity and authority. Clone the clean baseline for each subsequent query.
func syncReplaySession(tx *gorm.DB) *gorm.DB {
	query := tx.Session(&gorm.Session{NewDB: true})
	for _, key := range []string{store.SyncApplyingKey, syncLocalNodeKey, syncLocalInstanceKey, syncPreferRemoteKey} {
		if value, ok := tx.Get(key); ok {
			query = query.Set(key, value)
		}
	}
	return query.Session(&gorm.Session{})
}

// --- Devices ---

func applyDeviceChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	tx = syncReplaySession(tx)
	var existing store.Device
	lookupErr := tx.Unscoped().Where("code = ?", ch.EntityCode).First(&existing).Error
	missing := errors.Is(lookupErr, gorm.ErrRecordNotFound)
	if lookupErr != nil && !missing {
		return syncStatusRejected, lookupErr
	}

	if ch.Op == store.SyncOpDelete {
		return applyDeviceDeleteTx(tx, ch, &existing, missing)
	}

	if ch.Payload == "" {
		return syncStatusRejected, fmt.Errorf("device upsert change %s carries no payload", ch.ChangeID())
	}
	var incoming store.Device
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		return syncStatusRejected, err
	}

	if missing {
		return syncStatusApplied, adoptDeviceTx(tx, ch, &incoming)
	}

	// Identical content is agreement, not conflict. Revisions still have to line
	// up, otherwise the next change from this peer would compare its BaseRev
	// against a stale local Rev and be misread as a conflict.
	if deviceContentEqual(&existing, &incoming) {
		if ch.NewRev > existing.Rev {
			return syncStatusApplied, tx.Unscoped().Model(&store.Device{}).
				Where("code = ?", existing.Code).Update("rev", ch.NewRev).Error
		}
		return syncStatusApplied, nil
	}

	if ch.BaseRev != existing.Rev {
		if preferRemoteChange(tx) {
			return syncStatusApplied, overwriteDeviceTx(tx, &existing, ch, &incoming)
		}
		return reconcileStaleDeviceTx(tx, ch, &existing)
	}

	return syncStatusApplied, overwriteDeviceTx(tx, &existing, ch, &incoming)
}

func applyDeviceDeleteTx(tx *gorm.DB, ch store.SyncChange, existing *store.Device, missing bool) (string, error) {
	if missing {
		return syncStatusApplied, nil
	}
	// A peer deletion is authoritative as a deletion event, but a local edit
	// made concurrently is preserved as a conflict rather than discarded.
	if ch.BaseRev > 0 && existing.Rev > ch.BaseRev {
		if preferRemoteChange(tx) {
			return syncStatusApplied, deleteDeviceTx(tx, ch.EntityCode)
		}
		payload, err := buildDevicePayload(existing)
		if err != nil {
			return syncStatusRejected, err
		}
		return syncStatusConflict, appendGatewayCanonicalTx(tx, ch, payload, existing.Rev, existing.Rev)
	}
	return syncStatusApplied, deleteDeviceTx(tx, ch.EntityCode)
}

// adoptDeviceTx creates a device that exists on the peer but not locally. The
// peer's primary key and timestamps are meaningless here, so the local row is
// built fresh from content alone.
func adoptDeviceTx(tx *gorm.DB, ch store.SyncChange, incoming *store.Device) error {
	incoming.ID = 0
	incoming.CreatedAt = time.Time{}
	incoming.UpdatedAt = time.Time{}
	incoming.DeletedAt = gorm.DeletedAt{}
	clearConflictState(&incoming.SyncMeta)
	incoming.Rev = ch.NewRev
	incoming.SyncState = store.SyncStateSynced
	incoming.SyncWriter = ch.OriginNode
	return tx.Unscoped().Create(incoming).Error
}

func overwriteDeviceTx(tx *gorm.DB, existing *store.Device, ch store.SyncChange, incoming *store.Device) error {
	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = time.Now()
	incoming.DeletedAt = gorm.DeletedAt{}
	clearConflictState(&incoming.SyncMeta)
	incoming.Rev = ch.NewRev
	incoming.SyncState = store.SyncStateSynced
	incoming.SyncWriter = ch.OriginNode
	return tx.Unscoped().Save(incoming).Error
}

func recordDeviceConflictTx(tx *gorm.DB, existing *store.Device, ch store.SyncChange, incoming *store.Device) (string, error) {
	encoded, err := encodeConflictRecord(ch, incoming)
	if err != nil {
		return syncStatusRejected, err
	}
	updates := map[string]interface{}{
		"sync_state":       store.SyncStateConflict,
		"conflict_payload": encoded,
		"conflict_rev":     ch.NewRev,
		"conflict_node":    ch.OriginNode,
		"sync_writer":      ch.OriginNode,
	}
	if err := tx.Unscoped().Model(&store.Device{}).
		Where("code = ?", existing.Code).Updates(updates).Error; err != nil {
		return syncStatusRejected, err
	}
	return syncStatusConflict, nil
}

func deleteDeviceTx(tx *gorm.DB, code string) error {
	var device store.Device
	if err := tx.Where("code = ?", code).First(&device).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if err := tx.Unscoped().Where("device_code = ?", code).Delete(&store.DeviceTagBinding{}).Error; err != nil {
		return err
	}
	// Renaming releases the unique index on code so the same code can be
	// re-created later, matching the local deletion semantics.
	newCode := fmt.Sprintf("%s_del_%d", device.Code, time.Now().Unix())
	if err := tx.Model(&device).Update("code", newCode).Error; err != nil {
		return err
	}
	return tx.Delete(&device).Error
}

// --- Products ---

func applyProductChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	tx = syncReplaySession(tx)
	var existing store.Product
	lookupErr := tx.Unscoped().Where("code = ?", ch.EntityCode).First(&existing).Error
	missing := errors.Is(lookupErr, gorm.ErrRecordNotFound)
	if lookupErr != nil && !missing {
		return syncStatusRejected, lookupErr
	}

	if ch.Op == store.SyncOpDelete {
		if missing || existing.DeletedAt.Valid {
			return syncStatusApplied, nil
		}
		if ch.BaseRev > 0 && existing.Rev > ch.BaseRev {
			if preferRemoteChange(tx) {
				newCode := fmt.Sprintf("%s_del_%d", existing.Code, time.Now().UnixNano())
				if err := tx.Model(&existing).Update("code", newCode).Error; err != nil {
					return syncStatusRejected, err
				}
				return syncStatusApplied, tx.Delete(&existing).Error
			}
			payload, err := buildProductPayload(&existing)
			if err != nil {
				return syncStatusRejected, err
			}
			return syncStatusConflict, appendGatewayCanonicalTx(tx, ch, payload, existing.Rev, existing.Rev)
		}
		newCode := fmt.Sprintf("%s_del_%d", existing.Code, time.Now().UnixNano())
		if err := tx.Model(&existing).Update("code", newCode).Error; err != nil {
			return syncStatusRejected, err
		}
		return syncStatusApplied, tx.Delete(&existing).Error
	}

	if ch.Payload == "" {
		return syncStatusRejected, fmt.Errorf("product upsert change %s carries no payload", ch.ChangeID())
	}
	var incoming store.Product
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		return syncStatusRejected, err
	}

	if missing {
		incoming.ID = 0
		incoming.CreatedAt = time.Time{}
		incoming.UpdatedAt = time.Time{}
		incoming.DeletedAt = gorm.DeletedAt{}
		clearConflictState(&incoming.SyncMeta)
		incoming.Rev = ch.NewRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		return syncStatusApplied, tx.Unscoped().Create(&incoming).Error
	}

	if productContentEqual(&existing, &incoming) {
		if ch.NewRev > existing.Rev {
			return syncStatusApplied, tx.Unscoped().Model(&store.Product{}).
				Where("code = ?", existing.Code).Update("rev", ch.NewRev).Error
		}
		return syncStatusApplied, nil
	}

	if ch.BaseRev != existing.Rev {
		if preferRemoteChange(tx) {
			incoming.ID = existing.ID
			incoming.CreatedAt = existing.CreatedAt
			incoming.UpdatedAt = time.Now()
			incoming.DeletedAt = gorm.DeletedAt{}
			clearConflictState(&incoming.SyncMeta)
			incoming.Rev = ch.NewRev
			incoming.SyncState = store.SyncStateSynced
			incoming.SyncWriter = ch.OriginNode
			return syncStatusApplied, tx.Unscoped().Save(&incoming).Error
		}
		return reconcileStaleProductTx(tx, ch, &existing)
	}

	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = time.Now()
	incoming.DeletedAt = gorm.DeletedAt{}
	clearConflictState(&incoming.SyncMeta)
	incoming.Rev = ch.NewRev
	incoming.SyncState = store.SyncStateSynced
	incoming.SyncWriter = ch.OriginNode
	return syncStatusApplied, tx.Unscoped().Save(&incoming).Error
}

// --- Shared helpers ---

func clearConflictState(meta *store.SyncMeta) {
	meta.ConflictPayload = ""
	meta.ConflictRev = 0
	meta.ConflictNode = ""
}

func encodeConflictRecord[T any](ch store.SyncChange, incoming *T) (string, error) {
	record := conflictRecord{
		Op:   ch.Op,
		Node: ch.OriginNode,
		Rev:  ch.NewRev,
		At:   time.Now().UnixMilli(),
	}
	if incoming != nil {
		payload, err := json.Marshal(incoming)
		if err != nil {
			return "", err
		}
		record.Payload = payload
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func deviceContentEqual(a, b *store.Device) bool {
	if a == nil || b == nil {
		return false
	}
	return a.TenantID == b.TenantID &&
		a.ProjectID == b.ProjectID &&
		a.Code == b.Code &&
		a.Name == b.Name &&
		a.ProductCode == b.ProductCode &&
		a.ParentCode == b.ParentCode &&
		a.ProtocolName == b.ProtocolName &&
		a.ProtocolProfileCode == b.ProtocolProfileCode &&
		a.Enabled == b.Enabled &&
		a.Config == b.Config
}

func productContentEqual(a, b *store.Product) bool {
	if a == nil || b == nil {
		return false
	}
	return a.TenantID == b.TenantID &&
		a.ProjectID == b.ProjectID &&
		a.Code == b.Code &&
		a.Name == b.Name &&
		a.Config == b.Config
}

// buildDevicePayload renders the snapshot carried by a change. Identity and
// conflict fields are stripped: primary keys mean nothing on the peer, and
// shipping a conflict record would propagate an unresolved state instead of
// content. Ownership is preserved because it is routing metadata, not content.
func buildDevicePayload(d *store.Device) (string, error) {
	if d == nil {
		return "", fmt.Errorf("device is required")
	}
	snapshot := *d
	snapshot.SyncMeta.ConflictPayload = ""
	snapshot.SyncMeta.ConflictRev = 0
	snapshot.SyncMeta.ConflictNode = ""
	snapshot.SyncMeta.SyncState = store.SyncStateSynced
	snapshot.ID = 0
	snapshot.CreatedAt = time.Time{}
	snapshot.UpdatedAt = time.Time{}
	snapshot.DeletedAt = gorm.DeletedAt{}
	encoded, err := json.Marshal(&snapshot)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func buildProductPayload(p *store.Product) (string, error) {
	if p == nil {
		return "", fmt.Errorf("product is required")
	}
	snapshot := *p
	snapshot.SyncMeta.ConflictPayload = ""
	snapshot.SyncMeta.ConflictRev = 0
	snapshot.SyncMeta.ConflictNode = ""
	snapshot.SyncMeta.SyncState = store.SyncStateSynced
	snapshot.ID = 0
	snapshot.CreatedAt = time.Time{}
	snapshot.UpdatedAt = time.Time{}
	snapshot.DeletedAt = gorm.DeletedAt{}
	encoded, err := json.Marshal(&snapshot)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func logSyncIssue(logger *zap.Logger, message string, ch store.SyncChange, err error) {
	if logger == nil {
		return
	}
	fields := []zap.Field{
		zap.String("change_id", ch.ChangeID()),
		zap.String("entity_type", ch.EntityType),
		zap.String("entity_code", ch.EntityCode),
		zap.String("op", ch.Op),
	}
	if err != nil {
		fields = append(fields, zap.Error(err))
	}
	logger.Warn(message, fields...)
}
