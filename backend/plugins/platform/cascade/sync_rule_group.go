package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"noyo/core/store"

	"gorm.io/gorm"
)

var ruleGroupContentColumns = map[string]bool{
	"code": true, "tenant_id": true, "project_id": true, "name": true,
	"description": true, "sort_order": true,
}

func ruleGroupCodeFrom(db *gorm.DB) string {
	if db == nil || db.Statement == nil {
		return ""
	}
	if group, ok := db.Statement.Dest.(*store.RuleGroup); ok && group != nil {
		return group.Code
	}
	if group, ok := db.Statement.Model.(*store.RuleGroup); ok && group != nil {
		return group.Code
	}
	return ""
}

func (r *changeRecorder) recordRuleGroupCreated(db *gorm.DB) {
	if !r.suppressed(db) {
		r.recordRuleGroupUpsert(db, ruleGroupCodeFrom(db), true)
	}
}

func (r *changeRecorder) recordRuleGroupUpdated(db *gorm.DB) {
	if !r.suppressed(db) && contentChanged(db.Statement.Dest, ruleGroupContentColumns) {
		r.recordRuleGroupUpsert(db, ruleGroupCodeFrom(db), false)
	}
}

func (r *changeRecorder) recordRuleGroupUpsert(db *gorm.DB, code string, created bool) {
	if code == "" {
		return
	}
	tx := db.Session(&gorm.Session{NewDB: true})
	var recorded *store.SyncChange
	err := func() error {
		var group store.RuleGroup
		if err := tx.Where("code = ?", code).First(&group).Error; err != nil {
			return err
		}
		baseRev, newRev := group.Rev, group.Rev+1
		if created {
			baseRev, newRev = 0, 1
		}
		lifecycle, advancedRev, err := store.AdvanceLocalResourceLifecycleTx(tx, store.SyncEntityRuleGroup, code, created, false, newRev)
		if err != nil {
			return err
		}
		newRev = advancedRev
		snapshot := group
		snapshot.ID = 0
		snapshot.CreatedAt = 0
		snapshot.UpdatedAt = 0
		snapshot.Rev = newRev
		snapshot.SyncState = store.SyncStateSynced
		snapshot.SyncWriter = r.nodeID
		payload, err := json.Marshal(&snapshot)
		if err != nil {
			return err
		}
		ackTargets := ""
		syncState := store.SyncStatePending
		if r.nodeID == cloudSyncNodeID {
			ackTargets = r.routeScope(group.TenantID, group.ProjectID)
			if ackTargets == "" {
				syncState = store.SyncStateSynced
			}
		}
		if err := tx.Model(&store.RuleGroup{}).Set(store.SyncApplyingKey, true).Where("code = ?", code).
			Updates(map[string]interface{}{"rev": newRev, "sync_state": syncState, "sync_writer": r.nodeID}).Error; err != nil {
			return err
		}
		target := ""
		if r.nodeID != cloudSyncNodeID {
			target = cloudSyncNodeID
		}
		recorded = &store.SyncChange{
			EntityType: store.SyncEntityRuleGroup, EntityCode: code, Op: store.SyncOpUpsert,
			BaseRev: baseRev, NewRev: newRev, LifecycleVersion: lifecycle, Payload: string(payload), BasePayload: capturedSyncBase(db),
			OriginNode: r.nodeID, OriginInstance: r.instanceID,
			TargetNode: target, TenantID: group.TenantID, ProjectID: group.ProjectID,
		}
		recorded.AckTargets = ackTargets
		_, err = store.AppendSyncChangeTx(tx, recorded)
		return err
	}()
	r.finishTx(db, recorded, "rule_group", code, err)
}

func (r *changeRecorder) RuleGroupDeletedTx(tx *gorm.DB, group *store.RuleGroup) (*store.SyncChange, error) {
	if group == nil || group.Code == "" {
		return nil, nil
	}
	lifecycle, newRev, err := store.AdvanceLocalResourceLifecycleTx(tx, store.SyncEntityRuleGroup, group.Code, false, true, group.Rev+1)
	if err != nil {
		return nil, err
	}
	target := ""
	if r.nodeID != cloudSyncNodeID {
		target = cloudSyncNodeID
	}
	ch := &store.SyncChange{
		EntityType: store.SyncEntityRuleGroup, EntityCode: group.Code, Op: store.SyncOpDelete,
		BaseRev: group.Rev, NewRev: newRev, LifecycleVersion: lifecycle,
		OriginNode: r.nodeID, OriginInstance: r.instanceID,
		TargetNode: target, TenantID: group.TenantID, ProjectID: group.ProjectID,
	}
	if r.nodeID == cloudSyncNodeID {
		ch.AckTargets = r.routeScope(group.TenantID, group.ProjectID)
	}
	_, err = store.AppendSyncChangeTx(tx, ch)
	if err == nil {
		err = r.queueLocalRuntimeTx(tx, ch)
	}
	return ch, err
}

func applyRuleGroupChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	tx = syncReplaySession(tx)
	var existing store.RuleGroup
	err := tx.Where("code = ?", ch.EntityCode).First(&existing).Error
	missing := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !missing {
		return syncStatusRejected, err
	}
	if ch.Op == store.SyncOpDelete {
		if missing {
			return syncStatusApplied, nil
		}
		if existing.Rev > ch.BaseRev && !preferRemoteChange(tx) {
			payload, err := json.Marshal(&existing)
			if err != nil {
				return syncStatusRejected, err
			}
			return syncStatusConflict, appendGatewayCanonicalTx(tx, ch, string(payload), existing.Rev, existing.Rev)
		}
		return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Delete(&existing).Error
	}
	var incoming store.RuleGroup
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		return syncStatusRejected, err
	}
	if incoming.Code != ch.EntityCode {
		return syncStatusRejected, fmt.Errorf("rule group code mismatch")
	}
	if missing {
		incoming.ID = 0
		incoming.CreatedAt = 0
		incoming.UpdatedAt = 0
		incoming.Rev = ch.NewRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Create(&incoming).Error
	}
	if existing.Name == incoming.Name && existing.Description == incoming.Description && existing.SortOrder == incoming.SortOrder {
		if ch.NewRev > existing.Rev {
			return syncStatusApplied, tx.Model(&existing).Set(store.SyncApplyingKey, true).Update("rev", ch.NewRev).Error
		}
		return syncStatusApplied, nil
	}
	if ch.BaseRev != existing.Rev && !preferRemoteChange(tx) {
		payload, err := json.Marshal(&existing)
		if err != nil {
			return syncStatusRejected, err
		}
		return syncStatusConflict, appendGatewayCanonicalTx(tx, ch, string(payload), existing.Rev, existing.Rev)
	}
	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = time.Now().UnixMilli()
	incoming.Rev = ch.NewRev
	incoming.SyncState = store.SyncStateSynced
	incoming.SyncWriter = ch.OriginNode
	return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error
}
