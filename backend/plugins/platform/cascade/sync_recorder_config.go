package cascade

import (
	"encoding/json"
	"time"

	"noyo/core/store"

	"gorm.io/gorm"
)

var driverContentColumns = map[string]bool{
	"code": true, "tenant_id": true, "project_id": true, "name": true,
	"protocol_name": true, "product_code": true, "description": true, "config": true,
}

var ruleContentColumns = map[string]bool{
	"code": true, "tenant_id": true, "project_id": true, "name": true,
	"description": true, "group_code": true, "enabled": true, "priority": true,
	"status": true, "triggers": true, "conditions": true, "actions": true,
	"effective_time": true, "throttle_sec": true, "max_per_hour": true,
	"silent_start": true, "silent_end": true, "retry_count": true,
	"scope": true, "gateway_sn": true, "enabled_by": true,
	"source_type": true, "source_id": true,
}

func driverCodeFrom(db *gorm.DB) string {
	if db == nil || db.Statement == nil {
		return ""
	}
	if p, ok := db.Statement.Dest.(*store.ProtocolProfile); ok && p != nil {
		return p.Code
	}
	if p, ok := db.Statement.Model.(*store.ProtocolProfile); ok && p != nil {
		return p.Code
	}
	return ""
}

func ruleCodeFrom(db *gorm.DB) string {
	if db == nil || db.Statement == nil {
		return ""
	}
	if rule, ok := db.Statement.Dest.(*store.Rule); ok && rule != nil {
		return rule.Code
	}
	if rule, ok := db.Statement.Model.(*store.Rule); ok && rule != nil {
		return rule.Code
	}
	return ""
}

func (r *changeRecorder) recordDriverCreated(db *gorm.DB) {
	if !r.suppressed(db) {
		r.recordDriverUpsert(db, driverCodeFrom(db), true)
	}
}

func (r *changeRecorder) recordDriverUpdated(db *gorm.DB) {
	if !r.suppressed(db) && !isCodeOnlyUpdate(db.Statement.Dest) && contentChanged(db.Statement.Dest, driverContentColumns) {
		r.recordDriverUpsert(db, driverCodeFrom(db), false)
	}
}

func (r *changeRecorder) recordDriverUpsert(db *gorm.DB, code string, created bool) {
	if code == "" {
		return
	}
	tx := db.Session(&gorm.Session{NewDB: true})
	var recorded *store.SyncChange
	err := func() error {
		var profile store.ProtocolProfile
		if err := tx.Where("code = ?", code).First(&profile).Error; err != nil {
			return err
		}
		if created && r.ownerNode != "" && profile.OwnerNode == "" {
			profile.OwnerNode = r.ownerNode
			if err := tx.Model(&store.ProtocolProfile{}).Set(store.SyncApplyingKey, true).
				Where("code = ?", code).Update("owner_node", r.ownerNode).Error; err != nil {
				return err
			}
		}
		baseRev, newRev := profile.Rev, profile.Rev+1
		if created {
			baseRev, newRev = 0, 1
		}
		lifecycle, advancedRev, err := r.nextResourceVersionTx(tx, store.SyncEntityDriver, code, created, false, newRev)
		if err != nil {
			return err
		}
		newRev = advancedRev
		payloadProfile := profile
		payloadProfile.ID = 0
		payloadProfile.CreatedAt = time.Time{}
		payloadProfile.UpdatedAt = time.Time{}
		payloadProfile.DeletedAt = gorm.DeletedAt{}
		payloadProfile.Rev = newRev
		payloadProfile.SyncState = store.SyncStateSynced
		payloadProfile.SyncWriter = r.nodeID
		payload, err := json.Marshal(&payloadProfile)
		if err != nil {
			return err
		}
		ackTargets := ""
		syncState := store.SyncStatePending
		if r.nodeID == cloudSyncNodeID && profile.OwnerNode == "" {
			ackTargets = r.routeScope(profile.TenantID, profile.ProjectID)
			if ackTargets == "" {
				syncState = store.SyncStateSynced
			}
		}
		if err := tx.Model(&store.ProtocolProfile{}).Set(store.SyncApplyingKey, true).
			Where("code = ? AND rev = ?", code, profile.Rev).
			Updates(map[string]interface{}{"rev": newRev, "sync_state": syncState, "sync_writer": r.nodeID}).Error; err != nil {
			return err
		}
		target := profile.OwnerNode
		if r.nodeID != cloudSyncNodeID {
			target = cloudSyncNodeID
		}
		recorded = &store.SyncChange{
			EntityType: store.SyncEntityDriver, EntityCode: code, Op: store.SyncOpUpsert,
			BaseRev: baseRev, NewRev: newRev, LifecycleVersion: lifecycle, Payload: string(payload), BasePayload: capturedSyncBase(db),
			OriginNode: r.nodeID, OriginInstance: r.instanceID, TargetNode: target,
			TenantID: profile.TenantID, ProjectID: profile.ProjectID,
		}
		recorded.AckTargets = ackTargets
		if err := collectConfigDependenciesTx(tx, recorded); err != nil {
			return err
		}
		_, err = store.AppendSyncChangeTx(tx, recorded)
		return err
	}()
	r.finishTx(db, recorded, "driver", code, err)
}

func (r *changeRecorder) recordRuleCreated(db *gorm.DB) {
	if !r.suppressed(db) {
		r.recordRuleUpsert(db, ruleCodeFrom(db), true)
	}
}

func (r *changeRecorder) recordRuleUpdated(db *gorm.DB) {
	if !r.suppressed(db) && contentChanged(db.Statement.Dest, ruleContentColumns) {
		r.recordRuleUpsert(db, ruleCodeFrom(db), false)
	}
}

func (r *changeRecorder) recordRuleUpsert(db *gorm.DB, code string, created bool) {
	if code == "" {
		return
	}
	tx := db.Session(&gorm.Session{NewDB: true})
	var recorded *store.SyncChange
	err := func() error {
		var rule store.Rule
		if err := tx.Where("code = ?", code).First(&rule).Error; err != nil {
			return err
		}
		if r.nodeID == cloudSyncNodeID && rule.Scope != "gateway" {
			return nil
		}
		if r.nodeID != cloudSyncNodeID && rule.Scope == "gateway" && rule.GatewaySN == "" {
			rule.GatewaySN = r.nodeID
			if err := tx.Model(&store.Rule{}).Set(store.SyncApplyingKey, true).Where("code = ?", code).Update("gateway_sn", r.nodeID).Error; err != nil {
				return err
			}
		}
		baseRev, newRev := rule.Version-1, rule.Version
		if created {
			baseRev, newRev = 0, 1
		}
		if newRev < 1 {
			newRev = 1
		}
		lifecycle, advancedRev, err := r.nextResourceVersionTx(tx, store.SyncEntityRule, code, created, false, newRev)
		if err != nil {
			return err
		}
		newRev = advancedRev
		snapshot := rule
		snapshot.ID = 0
		snapshot.CreatedAt = 0
		snapshot.UpdatedAt = 0
		snapshot.Version = newRev
		snapshot.SyncState = store.SyncStateSynced
		snapshot.SyncWriter = r.nodeID
		payload, err := json.Marshal(&snapshot)
		if err != nil {
			return err
		}
		if err := tx.Model(&store.Rule{}).Set(store.SyncApplyingKey, true).
			Where("code = ?", code).
			Updates(map[string]interface{}{"version": newRev, "sync_state": store.SyncStatePending, "sync_writer": r.nodeID}).Error; err != nil {
			return err
		}
		target := rule.GatewaySN
		if r.nodeID != cloudSyncNodeID {
			target = cloudSyncNodeID
		}
		recorded = &store.SyncChange{
			EntityType: store.SyncEntityRule, EntityCode: code, Op: store.SyncOpUpsert,
			BaseRev: baseRev, NewRev: newRev, LifecycleVersion: lifecycle, Payload: string(payload), BasePayload: capturedSyncBase(db),
			OriginNode: r.nodeID, OriginInstance: r.instanceID, TargetNode: target,
			TenantID: rule.TenantID, ProjectID: rule.ProjectID,
		}
		_, err = store.AppendSyncChangeTx(tx, recorded)
		return err
	}()
	r.finishTx(db, recorded, "rule", code, err)
}

func (r *changeRecorder) DriverDeletedTx(tx *gorm.DB, profile *store.ProtocolProfile) (*store.SyncChange, error) {
	if profile == nil || profile.Code == "" {
		return nil, nil
	}
	target := ""
	if r.nodeID != cloudSyncNodeID {
		target = cloudSyncNodeID
	} else if profile.OwnerNode != "" {
		target = profile.OwnerNode
	}
	lifecycle, newRev, err := r.nextResourceVersionTx(tx, store.SyncEntityDriver, profile.Code, false, true, profile.Rev+1)
	if err != nil {
		return nil, err
	}
	ch := &store.SyncChange{
		EntityType: store.SyncEntityDriver, EntityCode: profile.Code,
		Op: store.SyncOpDelete, BaseRev: profile.Rev, NewRev: newRev, LifecycleVersion: lifecycle,
		OriginNode: r.nodeID, OriginInstance: r.instanceID, TargetNode: target,
		TenantID: profile.TenantID, ProjectID: profile.ProjectID,
	}
	if r.nodeID == cloudSyncNodeID && profile.OwnerNode == "" {
		ch.AckTargets = r.routeScope(profile.TenantID, profile.ProjectID)
	}
	_, err = store.AppendSyncChangeTx(tx, ch)
	if err == nil {
		err = r.stagePlatformDeleteTx(tx, ch, profile)
	}
	if err == nil {
		err = r.queueLocalRuntimeTx(tx, ch)
	}
	return ch, err
}

func (r *changeRecorder) RuleDeletedTx(tx *gorm.DB, rule *store.Rule) (*store.SyncChange, error) {
	if rule == nil || rule.Code == "" {
		return nil, nil
	}
	if r.nodeID == cloudSyncNodeID && rule.Scope != "gateway" {
		return nil, nil
	}
	target := rule.GatewaySN
	if r.nodeID != cloudSyncNodeID || rule.Scope != "gateway" {
		target = cloudSyncNodeID
	}
	lifecycle, newRev, err := r.nextResourceVersionTx(tx, store.SyncEntityRule, rule.Code, false, true, rule.Version+1)
	if err != nil {
		return nil, err
	}
	ch := &store.SyncChange{
		EntityType: store.SyncEntityRule, EntityCode: rule.Code,
		Op: store.SyncOpDelete, BaseRev: rule.Version, NewRev: newRev, LifecycleVersion: lifecycle,
		OriginNode: r.nodeID, OriginInstance: r.instanceID, TargetNode: target,
		TenantID: rule.TenantID, ProjectID: rule.ProjectID,
	}
	_, err = store.AppendSyncChangeTx(tx, ch)
	if err == nil {
		err = r.stagePlatformDeleteTx(tx, ch, rule)
	}
	if err == nil {
		err = r.queueLocalRuntimeTx(tx, ch)
	}
	return ch, err
}
