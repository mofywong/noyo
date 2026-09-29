package cascade

import (
	"encoding/json"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"noyo/core/store"
)

func configEntity(kind string) interface{} {
	switch kind {
	case store.SyncEntityDevice:
		return &store.Device{}
	case store.SyncEntityProduct:
		return &store.Product{}
	case store.SyncEntityDriver:
		return &store.ProtocolProfile{}
	case store.SyncEntityRule:
		return &store.Rule{}
	case store.SyncEntityRuleGroup:
		return &store.RuleGroup{}
	}
	return nil
}

// Platform edits propose a next revision; only the gateway commits that revision.
func (r *changeRecorder) nextResourceVersionTx(tx *gorm.DB, kind, code string, created, deleted bool, proposed int64) (int64, int64, error) {
	if !r.separateIntents {
		return store.AdvanceLocalResourceLifecycleTx(tx, kind, code, created, deleted, proposed)
	}
	var state store.ResourceLifecycle
	err := tx.Session(&gorm.Session{NewDB: true}).Where("entity_type = ? AND code = ?", kind, code).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 1, max(int64(1), proposed), nil
	}
	if err != nil {
		return 0, 0, err
	}
	version := state.Version
	if created && state.Deleted {
		version++
	}
	return version, max(proposed, state.LastRev+1), nil
}

func platformOwnedIntent(ch *store.SyncChange) bool {
	return ch != nil && ch.TargetNode != "" && ch.TargetNode != cloudSyncNodeID
}

// This executes before the outer GORM transaction commits. The business row is
// restored to confirmed state; the user proposal survives only in its outbox.
func (r *changeRecorder) stagePlatformIntentTx(tx *gorm.DB, ch *store.SyncChange) error {
	if !r.separateIntents {
		return nil
	}
	if !platformOwnedIntent(ch) {
		return store.AcceptRemoteResourceLifecycleTx(tx, *ch)
	}
	query := tx.Session(&gorm.Session{NewDB: true}).Set(store.SyncApplyingKey, true)
	if err := store.SaveConfigIntentTx(query, *ch); err != nil {
		return err
	}
	if ch.Op == store.SyncOpDelete {
		return nil
	}
	model := configEntity(ch.EntityType)
	if model == nil {
		return fmt.Errorf("unsupported configuration projection %s", ch.EntityType)
	}
	if ch.BasePayload == "" {
		return query.Unscoped().Where("code = ?", ch.EntityCode).Delete(model).Error
	}
	// Rule IDs are intentionally absent from JSON. Load the internal identity
	// before restoring the baseline so Save updates the existing row rather
	// than attempting a second insert with the same business code.
	if err := tx.Session(&gorm.Session{NewDB: true}).Unscoped().Where("code = ?", ch.EntityCode).Take(model).Error; err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(ch.BasePayload), model); err != nil {
		return err
	}
	// Internal primary keys never become a resource addressing contract.
	return query.Unscoped().Save(model).Error
}

func (r *changeRecorder) stagePlatformDeleteTx(tx *gorm.DB, ch *store.SyncChange, base interface{}) error {
	if !r.separateIntents {
		return nil
	}
	if !platformOwnedIntent(ch) {
		return store.AcceptRemoteResourceLifecycleTx(tx, *ch)
	}
	encoded, err := json.Marshal(base)
	if err != nil {
		return err
	}
	ch.BasePayload = string(encoded)
	if err := tx.Session(&gorm.Session{NewDB: true}).Model(&store.SyncChange{}).Where("seq = ?", ch.Seq).Update("base_payload", ch.BasePayload).Error; err != nil {
		return err
	}
	return store.SaveConfigIntentTx(tx, *ch)
}

func recordConfirmedProjectionTx(tx *gorm.DB, ch store.SyncChange) error {
	if !tx.Migrator().HasTable(&store.ConfigProjection{}) {
		return nil
	}
	return store.SaveConfigProjectionTx(tx, ch.OriginNode, ch)
}
