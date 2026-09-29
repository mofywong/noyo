package cascade

import (
	"encoding/json"
	"errors"
	"time"

	"noyo/core/config"
	"noyo/core/store"

	"gorm.io/gorm"
)

// Upgrade existing committed settings without treating initialization as an edit.
// Subsequent writes still pass through RecordProtectedIntentTx and its fence.
func bootstrapProtectedGatewayConfigsTx(tx *gorm.DB, gateway, instance string, running *config.GlobalConfig) error {
	tenant, project, err := protectedScopeTx(tx, gateway)
	if err != nil {
		return err
	}
	seed := func(value store.ProtectedConfig) error {
		if _, err := store.GetProtectedConfigTx(tx, value.Code); err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if value.Revision == 0 {
			value.Revision = 1
		}
		value.SyncState = store.SyncStatePending
		if err := store.SaveProtectedConfigTx(tx, value); err != nil {
			return err
		}
		payload, err := json.Marshal(value)
		if err != nil {
			return err
		}
		change := store.SyncChange{EntityType: protectedConfigEntity(value), EntityCode: value.Code, Op: store.SyncOpUpsert, NewRev: value.Revision, LifecycleVersion: 1, Payload: string(payload), OriginNode: gateway, OriginInstance: instance, TargetNode: cloudSyncNodeID, TenantID: tenant, ProjectID: project, CreatedAt: time.Now().UnixMilli()}
		_, err = store.AppendSyncChangeTx(tx.Session(&gorm.Session{NewDB: true}), &change)
		return err
	}
	if running == nil {
		var row store.SystemConfig
		if err := tx.Session(&gorm.Session{NewDB: true}).Where("key = ?", "global_config").Take(&row).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		} else if err != nil {
			return err
		}
		running = new(config.GlobalConfig)
		if err := json.Unmarshal([]byte(row.Value), running); err != nil {
			return err
		}
	}
	fields, err := systemConfigFields(running)
	if err != nil {
		return err
	}
	return seed(store.ProtectedConfig{Code: store.ProtectedConfigCode(gateway, "system", ""), GatewaySN: gateway, Kind: "system", TenantID: tenant, ProjectID: project, Revision: 1, Config: fields})
}
