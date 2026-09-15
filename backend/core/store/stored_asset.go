package store

import "time"

// StoredAsset records provenance; filenames never imply authorization.
type StoredAsset struct {
	Path string `gorm:"primaryKey;size:512"`
	TenantID uint `gorm:"index"`
	ProjectID uint `gorm:"index"`
	OwnerID uint `gorm:"index"`
	DeviceCode string `gorm:"index"`
	CreatedAt time.Time
}
