package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"noyo/core/store"

	"gorm.io/gorm"
)

func applyDriverChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	tx = syncReplaySession(tx)
	var existing store.ProtocolProfile
	err := tx.Unscoped().Where("code = ?", ch.EntityCode).First(&existing).Error
	missing := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !missing {
		return syncStatusRejected, err
	}
	if ch.Op == store.SyncOpDelete {
		if missing || existing.DeletedAt.Valid {
			return syncStatusApplied, nil
		}
		if existing.Rev > ch.BaseRev {
			if preferRemoteChange(tx) {
				newCode := fmt.Sprintf("%s_del_%d", existing.Code, time.Now().UnixNano())
				if err := tx.Model(&existing).Set(store.SyncApplyingKey, true).Update("code", newCode).Error; err != nil {
					return syncStatusRejected, err
				}
				return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Delete(&existing).Error
			}
			payload, err := driverCanonicalPayload(&existing)
			if err != nil {
				return syncStatusRejected, err
			}
			return syncStatusConflict, appendGatewayCanonicalTx(tx, ch, payload, existing.Rev, existing.Rev)
		}
		newCode := fmt.Sprintf("%s_del_%d", existing.Code, time.Now().UnixNano())
		if err := tx.Model(&existing).Set(store.SyncApplyingKey, true).Update("code", newCode).Error; err != nil {
			return syncStatusRejected, err
		}
		return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Delete(&existing).Error
	}
	if ch.Payload == "" {
		return syncStatusRejected, fmt.Errorf("driver %s has no payload", ch.ChangeID())
	}
	var incoming store.ProtocolProfile
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		return syncStatusRejected, err
	}
	if incoming.Code != ch.EntityCode {
		return syncStatusRejected, fmt.Errorf("driver code mismatch")
	}
	if missing {
		incoming.ID = 0
		incoming.CreatedAt = time.Time{}
		incoming.UpdatedAt = time.Time{}
		incoming.DeletedAt = gorm.DeletedAt{}
		incoming.Rev = ch.NewRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Create(&incoming).Error
	}
	if driverConfigEqual(existing, incoming) {
		if ch.NewRev > existing.Rev {
			return syncStatusApplied, tx.Model(&existing).Set(store.SyncApplyingKey, true).Update("rev", ch.NewRev).Error
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
			return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error
		}
		return reconcileStaleDriverTx(tx, ch, &existing)
	}
	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = time.Now()
	incoming.DeletedAt = gorm.DeletedAt{}
	clearConflictState(&incoming.SyncMeta)
	incoming.Rev = ch.NewRev
	incoming.SyncState = store.SyncStateSynced
	incoming.SyncWriter = ch.OriginNode
	return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error
}

func driverConfigEqual(a, b store.ProtocolProfile) bool {
	return a.Code == b.Code && a.TenantID == b.TenantID && a.ProjectID == b.ProjectID &&
		a.Name == b.Name && a.ProtocolName == b.ProtocolName && a.ProductCode == b.ProductCode &&
		a.Description == b.Description && a.Config == b.Config
}

func applyRuleChangeTx(tx *gorm.DB, ch store.SyncChange) (string, error) {
	tx = syncReplaySession(tx)
	var existing store.Rule
	err := tx.Where("code = ?", ch.EntityCode).First(&existing).Error
	missing := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !missing {
		return syncStatusRejected, err
	}
	if ch.Op == store.SyncOpDelete {
		if missing {
			return syncStatusApplied, nil
		}
		if existing.Version > ch.BaseRev {
			if preferRemoteChange(tx) {
				return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Delete(&existing).Error
			}
			payload, err := ruleCanonicalPayload(&existing)
			if err != nil {
				return syncStatusRejected, err
			}
			return syncStatusConflict, appendGatewayCanonicalTx(tx, ch, payload, existing.Version, existing.Version)
		}
		return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Delete(&existing).Error
	}
	if ch.Payload == "" {
		return syncStatusRejected, fmt.Errorf("rule %s has no payload", ch.ChangeID())
	}
	var incoming store.Rule
	if err := json.Unmarshal([]byte(ch.Payload), &incoming); err != nil {
		return syncStatusRejected, err
	}
	if incoming.Code != ch.EntityCode {
		return syncStatusRejected, fmt.Errorf("rule code mismatch")
	}
	if preferRemoteChange(tx) && incoming.Scope == "gateway" && incoming.GatewaySN == "" {
		incoming.GatewaySN = ch.OriginNode
	}
	if missing {
		incoming.ID = 0
		incoming.CreatedAt = 0
		incoming.UpdatedAt = 0
		incoming.Version = ch.NewRev
		incoming.SyncState = store.SyncStateSynced
		incoming.SyncWriter = ch.OriginNode
		return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Create(&incoming).Error
	}
	if ruleConfigEqual(existing, incoming) {
		if ch.NewRev > existing.Version {
			return syncStatusApplied, tx.Model(&existing).Set(store.SyncApplyingKey, true).Update("version", ch.NewRev).Error
		}
		return syncStatusApplied, nil
	}
	if ch.BaseRev != existing.Version {
		if preferRemoteChange(tx) {
			incoming.ID = existing.ID
			incoming.CreatedAt = existing.CreatedAt
			incoming.UpdatedAt = 0
			incoming.Version = ch.NewRev
			incoming.SyncState = store.SyncStateSynced
			incoming.SyncWriter = ch.OriginNode
			incoming.ConflictPayload = ""
			incoming.ConflictRev = 0
			incoming.ConflictNode = ""
			return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error
		}
		return reconcileStaleRuleTx(tx, ch, &existing)
	}
	incoming.ID = existing.ID
	incoming.CreatedAt = existing.CreatedAt
	incoming.UpdatedAt = 0
	incoming.Version = ch.NewRev
	incoming.SyncState = store.SyncStateSynced
	incoming.SyncWriter = ch.OriginNode
	incoming.ConflictPayload = ""
	incoming.ConflictRev = 0
	incoming.ConflictNode = ""
	return syncStatusApplied, tx.Set(store.SyncApplyingKey, true).Save(&incoming).Error
}

func ruleConfigEqual(a, b store.Rule) bool {
	clean := func(rule store.Rule) store.Rule {
		rule.ID = 0
		rule.Version = 0
		rule.SyncState = ""
		rule.SyncError = ""
		rule.SyncWriter = ""
		rule.ConflictPayload = ""
		rule.ConflictRev = 0
		rule.ConflictNode = ""
		rule.LastTriggeredAt = nil
		rule.TriggerCount = 0
		rule.ErrorMessage = ""
		rule.CreatedAt = 0
		rule.UpdatedAt = 0
		return rule
	}
	return reflect.DeepEqual(clean(a), clean(b))
}
