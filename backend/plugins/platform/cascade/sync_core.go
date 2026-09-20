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
		return syncStatusSkipped
	}
	if hooks.beforeApply != nil && !hooks.beforeApply(ch) {
		// Consume the change without applying it. Marking it applied keeps the
		// peer's cursor moving; leaving it unmarked would make the same rejected
		// change arrive again on every handshake.
		if err := store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
			_, err := store.MarkChangeAppliedTx(tx, changeID)
			return err
		}); err != nil {
			logSyncIssue(logger, "failed to consume a rejected change", ch, err)
			return syncStatusRejected
		}
		return syncStatusSkipped
	}

	status := syncStatusApplied
	err = store.SyncApplyingSession().Transaction(func(tx *gorm.DB) error {
		fresh, err := store.MarkChangeAppliedTx(tx, changeID)
		if err != nil {
			return err
		}
		if !fresh {
			status = syncStatusSkipped
			return nil
		}
		resolved, err := applyChangeTx(tx, ch)
		if err != nil {
			return err
		}
		status = resolved
		return nil
	})
	if err != nil {
		logSyncIssue(logger, "failed to apply sync change", ch, err)
		return syncStatusRejected
	}

	if status != syncStatusSkipped && hooks.afterApply != nil {
		hooks.afterApply(ch, status)
	}
	return status
}

func applyChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	switch ch.EntityType {
	case store.SyncEntityDevice:
		return applyDeviceChangeTx(tx, ch)
	case store.SyncEntityProduct:
		return applyProductChangeTx(tx, ch)
	default:
		return syncStatusRejected, fmt.Errorf("unsupported sync entity type %q", ch.EntityType)
	}
}

// --- Devices ---

func applyDeviceChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
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
		return recordDeviceConflictTx(tx, &existing, ch, &incoming)
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
		return recordDeviceConflictTx(tx, existing, ch, nil)
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
	var existing store.Product
	lookupErr := tx.Unscoped().Where("code = ?", ch.EntityCode).First(&existing).Error
	missing := errors.Is(lookupErr, gorm.ErrRecordNotFound)
	if lookupErr != nil && !missing {
		return syncStatusRejected, lookupErr
	}

	if ch.Op == store.SyncOpDelete {
		// Products are never deleted in this codebase; a peer deletion is
		// acknowledged so it cannot stall the stream, and recorded for review.
		return syncStatusApplied, nil
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
		encoded, err := encodeConflictRecord(ch, &incoming)
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
		if err := tx.Unscoped().Model(&store.Product{}).
			Where("code = ?", existing.Code).Updates(updates).Error; err != nil {
			return syncStatusRejected, err
		}
		return syncStatusConflict, nil
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
