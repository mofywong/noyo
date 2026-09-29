package cascade

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"noyo/core/store"

	"gorm.io/gorm"
)

const (
	syncLocalNodeKey     = "cascade:local_node"
	syncLocalInstanceKey = "cascade:local_instance"
	syncPreferRemoteKey  = "cascade:prefer_remote"
)

var syncMergeGroups = map[string][][]string{
	store.SyncEntityDevice: {
		{"name", "enabled"},
		{"protocol_name", "protocol_profile_code", "config"},
		{"product_code", "parent_code"},
	},
	store.SyncEntityProduct: {{"name"}, {"config"}},
	store.SyncEntityDriver:  {{"name", "description"}, {"protocol_name", "product_code", "config"}},
	// A rule graph is an indivisible conflict unit.
	store.SyncEntityRule: {{"name", "description", "group_code", "enabled", "priority", "status", "triggers", "conditions", "actions", "effective_time", "throttle_sec", "max_per_hour", "silent_start", "silent_end", "retry_count", "scope", "gateway_sn", "source_type", "source_id"}},
}

func preferRemoteChange(tx *gorm.DB) bool {
	v, _ := tx.Get(syncPreferRemoteKey)
	preferred, _ := v.(bool)
	return preferred
}

// mergeStalePayload merges independent platform field groups as one request.
// Any conflicting group preserves the entire gateway document; a request
// must never partially commit while reporting a conflict.
func mergeStalePayload(ch store.SyncChange, local interface{}) (string, bool, bool, error) {
	localBytes, err := json.Marshal(local)
	if err != nil {
		return "", false, false, err
	}
	if ch.BasePayload == "" || ch.Payload == "" {
		return string(localBytes), false, true, nil
	}
	var base, remote, merged map[string]json.RawMessage
	if err := json.Unmarshal([]byte(ch.BasePayload), &base); err != nil {
		return "", false, false, err
	}
	if err := json.Unmarshal([]byte(ch.Payload), &remote); err != nil {
		return "", false, false, err
	}
	if err := json.Unmarshal(localBytes, &merged); err != nil {
		return "", false, false, err
	}
	applied := false
	for _, group := range syncMergeGroups[ch.EntityType] {
		remoteChanged, localChanged, different := false, false, false
		for _, field := range group {
			if !bytes.Equal(remote[field], base[field]) {
				remoteChanged = true
			}
			if !bytes.Equal(merged[field], base[field]) {
				localChanged = true
			}
			if !bytes.Equal(merged[field], remote[field]) {
				different = true
			}
		}
		if !remoteChanged || !different {
			continue
		}
		if localChanged {
			return string(localBytes), false, true, nil
		}
		for _, field := range group {
			if value, ok := remote[field]; ok {
				merged[field] = value
			}
		}
		applied = true
	}
	encoded, err := json.Marshal(merged)
	return string(encoded), applied, false, err
}

// appendGatewayCanonicalTx guarantees the platform ultimately sees the
// gateway's committed content even when its older request was superseded.
func appendGatewayCanonicalTx(tx *gorm.DB, ch store.SyncChange, payload string, baseRev, newRev int64) error {
	node, _ := tx.Get(syncLocalNodeKey)
	instance, _ := tx.Get(syncLocalInstanceKey)
	nodeCode, _ := node.(string)
	instanceID, _ := instance.(string)
	if nodeCode == "" || instanceID == "" {
		return fmt.Errorf("gateway reconciliation identity is unavailable")
	}
	query := tx.Session(&gorm.Session{NewDB: true})
	lifecycle := ch.LifecycleVersion
	if lifecycle > 0 {
		var state store.ResourceLifecycle
		if err := query.Where("entity_type = ? AND code = ?", ch.EntityType, ch.EntityCode).Take(&state).Error; err == nil {
			lifecycle = state.Version
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
	}
	correction := &store.SyncChange{
		EntityType: ch.EntityType, EntityCode: ch.EntityCode, Op: store.SyncOpUpsert,
		BaseRev: baseRev, NewRev: newRev, LifecycleVersion: lifecycle, Payload: payload,
		OriginNode: nodeCode, OriginInstance: instanceID, TargetNode: cloudSyncNodeID,
		TenantID: ch.TenantID, ProjectID: ch.ProjectID, CreatedAt: time.Now().UnixMilli(),
	}
	_, err := store.AppendSyncChangeTx(query, correction)
	return err
}

// A platform-side delete/recreate is a different incarnation even if it reuses
// the same code. When the gateway still has an active local incarnation, keep
// the site's resource and publish it under a newer lifecycle so the platform
// cannot discard the correction as an old event.
func resolveNewerPlatformLifecycleTx(tx *gorm.DB, ch store.SyncChange) (bool, string, error) {
	if ch.LifecycleVersion == 0 || preferRemoteChange(tx) {
		return false, "", nil
	}
	query := tx.Session(&gorm.Session{NewDB: true})
	var state store.ResourceLifecycle
	if err := query.Where("entity_type = ? AND code = ?", ch.EntityType, ch.EntityCode).Take(&state).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", nil
		}
		return false, "", err
	}
	if state.Deleted || ch.LifecycleVersion <= state.Version {
		return false, "", nil
	}
	var entity interface{}
	revColumn := "rev"
	switch ch.EntityType {
	case store.SyncEntityDevice:
		entity = &store.Device{}
	case store.SyncEntityProduct:
		entity = &store.Product{}
	case store.SyncEntityDriver:
		entity = &store.ProtocolProfile{}
	case store.SyncEntityRule:
		entity, revColumn = &store.Rule{}, "version"
	case store.SyncEntityRuleGroup:
		entity = &store.RuleGroup{}
	default:
		return false, "", nil
	}
	if err := query.Where("code = ?", ch.EntityCode).Take(entity).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return false, "", nil
		}
		return false, "", err
	}
	var localRev int64
	switch value := entity.(type) {
	case *store.Device:
		localRev = value.Rev
	case *store.Product:
		localRev = value.Rev
	case *store.ProtocolProfile:
		localRev = value.Rev
	case *store.Rule:
		localRev = value.Version
	case *store.RuleGroup:
		localRev = value.Rev
	}
	newRev := localRev + 1
	if newRev <= ch.NewRev {
		newRev = ch.NewRev + 1
	}
	if err := query.Model(entity).Set(store.SyncApplyingKey, true).
		Where("code = ?", ch.EntityCode).Updates(map[string]interface{}{revColumn: newRev, "sync_state": store.SyncStatePending}).Error; err != nil {
		return false, "", err
	}
	if err := query.Model(&store.ResourceLifecycle{}).
		Where("entity_type = ? AND code = ?", ch.EntityType, ch.EntityCode).
		Updates(map[string]interface{}{"version": ch.LifecycleVersion + 1, "last_rev": newRev, "deleted": false}).Error; err != nil {
		return false, "", err
	}
	if err := query.Where("code = ?", ch.EntityCode).Take(entity).Error; err != nil {
		return false, "", err
	}
	payload, err := json.Marshal(entity)
	if err != nil {
		return false, "", err
	}
	if err := appendGatewayCanonicalTx(tx, ch, string(payload), localRev, newRev); err != nil {
		return false, "", err
	}
	return true, syncStatusConflict, nil
}

func reconcileStaleDeviceTx(tx *gorm.DB, ch store.SyncChange, existing *store.Device) (string, error) {
	merged, changed, conflicted, err := mergeStalePayload(ch, existing)
	if err != nil {
		return syncStatusRejected, err
	}
	snapshot := *existing
	newRev := existing.Rev
	if changed {
		var incoming store.Device
		if err := json.Unmarshal([]byte(merged), &incoming); err != nil {
			return syncStatusRejected, err
		}
		newRev++
		applied := ch
		applied.NewRev = newRev
		if err := overwriteDeviceTx(tx, existing, applied, &incoming); err != nil {
			return syncStatusRejected, err
		}
		snapshot = incoming
	}
	snapshot.Rev = newRev
	payload, err := buildDevicePayload(&snapshot)
	if err != nil {
		return syncStatusRejected, err
	}
	if err := appendGatewayCanonicalTx(tx, ch, payload, existing.Rev, newRev); err != nil {
		return syncStatusRejected, err
	}
	if conflicted {
		return syncStatusConflict, nil
	}
	return syncStatusApplied, nil
}

func reconcileStaleProductTx(tx *gorm.DB, ch store.SyncChange, existing *store.Product) (string, error) {
	merged, changed, conflicted, err := mergeStalePayload(ch, existing)
	if err != nil {
		return syncStatusRejected, err
	}
	snapshot := *existing
	newRev := existing.Rev
	if changed {
		var incoming store.Product
		if err := json.Unmarshal([]byte(merged), &incoming); err != nil {
			return syncStatusRejected, err
		}
		newRev++
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		incoming.UpdatedAt = time.Now()
		incoming.DeletedAt = gorm.DeletedAt{}
		clearConflictState(&incoming.SyncMeta)
		incoming.Rev = newRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		if err := tx.Unscoped().Save(&incoming).Error; err != nil {
			return syncStatusRejected, err
		}
		snapshot = incoming
	}
	snapshot.Rev = newRev
	payload, err := buildProductPayload(&snapshot)
	if err != nil {
		return syncStatusRejected, err
	}
	if err := appendGatewayCanonicalTx(tx, ch, payload, existing.Rev, newRev); err != nil {
		return syncStatusRejected, err
	}
	if conflicted {
		return syncStatusConflict, nil
	}
	return syncStatusApplied, nil
}

func driverCanonicalPayload(profile *store.ProtocolProfile) (string, error) {
	snapshot := *profile
	snapshot.ID = 0
	snapshot.CreatedAt = time.Time{}
	snapshot.UpdatedAt = time.Time{}
	snapshot.DeletedAt = gorm.DeletedAt{}
	clearConflictState(&snapshot.SyncMeta)
	snapshot.SyncState = store.SyncStateSynced
	encoded, err := json.Marshal(&snapshot)
	return string(encoded), err
}

func reconcileStaleDriverTx(tx *gorm.DB, ch store.SyncChange, existing *store.ProtocolProfile) (string, error) {
	merged, changed, conflicted, err := mergeStalePayload(ch, existing)
	if err != nil {
		return syncStatusRejected, err
	}
	snapshot := *existing
	newRev := existing.Rev
	if changed {
		var incoming store.ProtocolProfile
		if err := json.Unmarshal([]byte(merged), &incoming); err != nil {
			return syncStatusRejected, err
		}
		newRev++
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		incoming.UpdatedAt = time.Now()
		incoming.DeletedAt = gorm.DeletedAt{}
		clearConflictState(&incoming.SyncMeta)
		incoming.Rev = newRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		if err := tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error; err != nil {
			return syncStatusRejected, err
		}
		snapshot = incoming
	}
	snapshot.Rev = newRev
	payload, err := driverCanonicalPayload(&snapshot)
	if err != nil {
		return syncStatusRejected, err
	}
	if err := appendGatewayCanonicalTx(tx, ch, payload, existing.Rev, newRev); err != nil {
		return syncStatusRejected, err
	}
	if conflicted {
		return syncStatusConflict, nil
	}
	return syncStatusApplied, nil
}

func ruleCanonicalPayload(rule *store.Rule) (string, error) {
	snapshot := *rule
	snapshot.ID = 0
	snapshot.CreatedAt = 0
	snapshot.UpdatedAt = 0
	snapshot.SyncState = store.SyncStateSynced
	snapshot.ConflictPayload = ""
	snapshot.ConflictRev = 0
	snapshot.ConflictNode = ""
	encoded, err := json.Marshal(&snapshot)
	return string(encoded), err
}

func reconcileStaleRuleTx(tx *gorm.DB, ch store.SyncChange, existing *store.Rule) (string, error) {
	merged, changed, conflicted, err := mergeStalePayload(ch, existing)
	if err != nil {
		return syncStatusRejected, err
	}
	snapshot := *existing
	newRev := existing.Version
	if changed {
		var incoming store.Rule
		if err := json.Unmarshal([]byte(merged), &incoming); err != nil {
			return syncStatusRejected, err
		}
		newRev++
		incoming.ID = existing.ID
		incoming.CreatedAt = existing.CreatedAt
		incoming.UpdatedAt = 0
		incoming.Version = newRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		incoming.ConflictPayload = ""
		incoming.ConflictRev = 0
		incoming.ConflictNode = ""
		if err := tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error; err != nil {
			return syncStatusRejected, err
		}
		snapshot = incoming
	}
	snapshot.Version = newRev
	payload, err := ruleCanonicalPayload(&snapshot)
	if err != nil {
		return syncStatusRejected, err
	}
	if err := appendGatewayCanonicalTx(tx, ch, payload, existing.Version, newRev); err != nil {
		return syncStatusRejected, err
	}
	if conflicted {
		return syncStatusConflict, nil
	}
	return syncStatusApplied, nil
}
