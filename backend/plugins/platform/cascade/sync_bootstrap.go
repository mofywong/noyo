package cascade

import (
	"encoding/json"
	"errors"
	"time"

	"noyo/core/store"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type bootstrapResource struct {
	kind, code, owner, gateway, target, revColumn string
	tenantID, projectID                           uint
	revision                                      int64
	value                                         interface{}
	model                                         interface{}
}

// bootstrapExistingResources records a one-time, atomic baseline for resources
// that were initialized before the sync hooks started. The retained log then
// covers a new peer or a rebuilt peer from sequence zero without guessing what
// happened before its first connection.
func bootstrapExistingResources(nodeID, instanceID string) error {
	return store.DB.Transaction(func(tx *gorm.DB) error {
		var marker store.SyncBootstrap
		err := tx.Where("node_id = ?", nodeID).Take(&marker).Error
		if err == nil && marker.InstanceID == instanceID {
			return nil
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		resources := make([]bootstrapResource, 0)
		var products []store.Product
		if err := tx.Find(&products).Error; err != nil {
			return err
		}
		for i := range products {
			p := &products[i]
			resources = append(resources, bootstrapResource{kind: store.SyncEntityProduct, code: p.Code, owner: p.OwnerNode, tenantID: p.TenantID, projectID: p.ProjectID, revision: p.Rev, revColumn: "rev", value: p, model: &store.Product{}})
		}
		var drivers []store.ProtocolProfile
		if err := tx.Find(&drivers).Error; err != nil {
			return err
		}
		for i := range drivers {
			p := &drivers[i]
			resources = append(resources, bootstrapResource{kind: store.SyncEntityDriver, code: p.Code, owner: p.OwnerNode, tenantID: p.TenantID, projectID: p.ProjectID, revision: p.Rev, revColumn: "rev", value: p, model: &store.ProtocolProfile{}})
		}
		var groups []store.RuleGroup
		if err := tx.Find(&groups).Error; err != nil {
			return err
		}
		for i := range groups {
			g := &groups[i]
			resources = append(resources, bootstrapResource{kind: store.SyncEntityRuleGroup, code: g.Code, tenantID: g.TenantID, projectID: g.ProjectID, revision: g.Rev, revColumn: "rev", value: g, model: &store.RuleGroup{}})
		}
		var devices []store.Device
		if err := tx.Find(&devices).Error; err != nil {
			return err
		}
		for i := range devices {
			d := &devices[i]
			owner := d.OwnerNode
			if owner == "" && d.ProductCode == defaultGatewayProductCode {
				owner = d.Code
			}
			resources = append(resources, bootstrapResource{kind: store.SyncEntityDevice, code: d.Code, owner: owner, tenantID: d.TenantID, projectID: d.ProjectID, revision: d.Rev, revColumn: "rev", value: d, model: &store.Device{}})
		}
		var rules []store.Rule
		if err := tx.Find(&rules).Error; err != nil {
			return err
		}
		for i := range rules {
			r := &rules[i]
			resources = append(resources, bootstrapResource{kind: store.SyncEntityRule, code: r.Code, gateway: r.GatewaySN, tenantID: r.TenantID, projectID: r.ProjectID, revision: r.Version, revColumn: "version", value: r, model: &store.Rule{}})
		}

		// Shared resources keep an empty target so the peer resolver can route
		// this baseline to gateways added after the platform first started.
		for i := range resources {
			if nodeID != cloudSyncNodeID {
				resources[i].target = cloudSyncNodeID
				continue
			}
			if resources[i].owner != "" {
				resources[i].target = resources[i].owner
				continue
			}
			if resources[i].gateway != "" {
				resources[i].target = resources[i].gateway
				continue
			}
		}

		for _, resource := range resources {
			if resource.code == "" {
				continue
			}
			var lifecycle store.ResourceLifecycle
			err := tx.Where("entity_type = ? AND code = ?", resource.kind, resource.code).Take(&lifecycle).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				lifecycle = store.ResourceLifecycle{EntityType: resource.kind, Code: resource.code, Version: 1}
			} else if err != nil {
				return err
			}
			if lifecycle.Deleted {
				lifecycle.Version++
			}
			if lifecycle.Version < 1 {
				lifecycle.Version = 1
			}
			revision := resource.revision
			if revision < 1 {
				revision = 1
			}
			if revision < lifecycle.LastRev {
				revision = lifecycle.LastRev
			}
			lifecycle.LastRev, lifecycle.Deleted = revision, false
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "entity_type"}, {Name: "code"}},
				DoUpdates: clause.AssignmentColumns([]string{"version", "last_rev", "deleted"}),
			}).Create(&lifecycle).Error; err != nil {
				return err
			}
			if resource.revision != revision {
				if err := tx.Model(resource.model).Set(store.SyncApplyingKey, true).Where("code = ?", resource.code).Update(resource.revColumn, revision).Error; err != nil {
					return err
				}
			}
			payload, err := json.Marshal(resource.value)
			if err != nil {
				return err
			}
			var document map[string]interface{}
			if err := json.Unmarshal(payload, &document); err != nil {
				return err
			}
			document[resource.revColumn] = revision
			document["sync_state"] = store.SyncStateSynced
			delete(document, "ID")
			delete(document, "CreatedAt")
			delete(document, "UpdatedAt")
			delete(document, "DeletedAt")
			payload, err = json.Marshal(document)
			if err != nil {
				return err
			}
			change := &store.SyncChange{
				EntityType: resource.kind, EntityCode: resource.code, Op: store.SyncOpUpsert,
				BaseRev: 0, NewRev: revision, LifecycleVersion: lifecycle.Version,
				Payload: string(payload), OriginNode: nodeID, OriginInstance: instanceID,
				TargetNode: resource.target, TenantID: resource.tenantID, ProjectID: resource.projectID,
			}
			if err := collectConfigDependenciesTx(tx, change); err != nil {
				return err
			}
			if _, err := store.AppendSyncChangeTx(tx, change); err != nil {
				return err
			}
		}
		marker = store.SyncBootstrap{NodeID: nodeID, InstanceID: instanceID, CompletedAt: time.Now().UnixMilli()}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "node_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"instance_id", "completed_at"}),
		}).Create(&marker).Error
	})
}
