package store

import (
	"errors"

	"gorm.io/gorm"
)

// ResourceLifecycle is the permanent high-water mark for a business code.
// Deleting an entity never deletes this compact tombstone, so a delayed message
// for an earlier incarnation cannot target a later entity with the same code.
type ResourceLifecycle struct {
	EntityType string `gorm:"primaryKey;size:16"`
	Code       string `gorm:"primaryKey;size:128"`
	Version    int64  `gorm:"not null"`
	LastRev    int64  `gorm:"not null"`
	Deleted    bool   `gorm:"not null;default:false"`
}

// AdvanceLocalResourceLifecycleTx runs in the same transaction as the business
// write and its outbox record. proposedRev is the entity's next content version.
func AdvanceLocalResourceLifecycleTx(tx *gorm.DB, entityType, code string, created, deleted bool, proposedRev int64) (int64, int64, error) {
	tx = tx.Session(&gorm.Session{NewDB: true})
	var state ResourceLifecycle
	err := tx.Where("entity_type = ? AND code = ?", entityType, code).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if proposedRev < 1 {
			proposedRev = 1
		}
		state = ResourceLifecycle{EntityType: entityType, Code: code, Version: 1, LastRev: proposedRev, Deleted: deleted}
		return state.Version, state.LastRev, tx.Create(&state).Error
	}
	if err != nil {
		return 0, 0, err
	}
	if created && state.Deleted {
		state.Version++
	}
	if proposedRev <= state.LastRev {
		proposedRev = state.LastRev + 1
	}
	state.LastRev = proposedRev
	state.Deleted = deleted
	err = tx.Model(&ResourceLifecycle{}).Where("entity_type = ? AND code = ?", entityType, code).
		Updates(map[string]interface{}{"version": state.Version, "last_rev": state.LastRev, "deleted": state.Deleted}).Error
	return state.Version, state.LastRev, err
}

// CheckRemoteResourceLifecycleTx rejects stale messages before resource replay.
// A newer incarnation is accepted and installed only after the resource write
// succeeds, in the same transaction as the idempotency marker.
func CheckRemoteResourceLifecycleTx(tx *gorm.DB, ch SyncChange) (bool, error) {
	if ch.LifecycleVersion == 0 {
		return true, nil
	}
	tx = tx.Session(&gorm.Session{NewDB: true})
	var state ResourceLifecycle
	err := tx.Where("entity_type = ? AND code = ?", ch.EntityType, ch.EntityCode).Take(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if ch.LifecycleVersion < state.Version ||
		(ch.LifecycleVersion == state.Version && state.Deleted && ch.Op != SyncOpDelete) {
		return false, nil
	}
	return true, nil
}

func AcceptRemoteResourceLifecycleTx(tx *gorm.DB, ch SyncChange) error {
	if ch.LifecycleVersion == 0 {
		return nil
	}
	tx = tx.Session(&gorm.Session{NewDB: true})
	state := ResourceLifecycle{EntityType: ch.EntityType, Code: ch.EntityCode,
		Version: ch.LifecycleVersion, LastRev: ch.NewRev, Deleted: ch.Op == SyncOpDelete}
	var existing ResourceLifecycle
	err := tx.Where("entity_type = ? AND code = ?", ch.EntityType, ch.EntityCode).Take(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return tx.Create(&state).Error
	}
	if err != nil {
		return err
	}
	if existing.Version > state.Version {
		return nil
	}
	if existing.Version == state.Version && existing.LastRev > state.LastRev {
		state.LastRev = existing.LastRev
	}
	return tx.Model(&ResourceLifecycle{}).Where("entity_type = ? AND code = ?", ch.EntityType, ch.EntityCode).
		Updates(map[string]interface{}{"version": state.Version, "last_rev": state.LastRev, "deleted": state.Deleted}).Error
}
