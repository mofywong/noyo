package cascade

import (
	"encoding/json"
	"errors"

	"noyo/core/store"

	"gorm.io/gorm"
)

const syncBasePayloadKey = "cascade:sync_base_payload"

// captureSyncBase runs before the SQL update. Capturing in the write
// transaction makes the request's base content durable in the same outbox row
// as the resulting payload, including when the peer is offline.
func (r *changeRecorder) captureSyncBase(kind string) func(*gorm.DB) {
	return func(db *gorm.DB) {
		if r.suppressed(db) || db.Error != nil {
			return
		}
		var code string
		var value interface{}
		switch kind {
		case store.SyncEntityDevice:
			code, value = deviceCodeFrom(db), &store.Device{}
		case store.SyncEntityProduct:
			code, value = productCodeFrom(db), &store.Product{}
		case store.SyncEntityDriver:
			code, value = driverCodeFrom(db), &store.ProtocolProfile{}
		case store.SyncEntityRule:
			code, value = ruleCodeFrom(db), &store.Rule{}
		case store.SyncEntityRuleGroup:
			code, value = ruleGroupCodeFrom(db), &store.RuleGroup{}
		default:
			return
		}
		if code == "" {
			return
		}
		err := db.Session(&gorm.Session{NewDB: true}).Where("code = ?", code).First(value).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		if err != nil {
			db.AddError(err)
			return
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			db.AddError(err)
			return
		}
		db.InstanceSet(syncBasePayloadKey, string(encoded))
	}
}

func capturedSyncBase(db *gorm.DB) string {
	if db == nil {
		return ""
	}
	value, ok := db.InstanceGet(syncBasePayloadKey)
	if !ok {
		return ""
	}
	base, _ := value.(string)
	return base
}
