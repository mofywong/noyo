package store

import "gorm.io/gorm"

// WithAutomaticConfigWrite scopes the maintenance exemption to this DB session.
// Administrative cleanup and runtime state changes are not manual inventory
// edits; their writes still emit normal sync changes.
func WithAutomaticConfigWrite(db *gorm.DB) *gorm.DB {
	return db.Session(&gorm.Session{}).Set(automaticConfigWriteKey, true)
}
