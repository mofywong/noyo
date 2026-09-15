package store

import "time"

// AuthSession persists the refresh-token family. Only the hash is stored.
type AuthSession struct {
	ID          string    `gorm:"primaryKey;size:36"`
	UserID      uint      `gorm:"index"`
	AppDBID     uint      `gorm:"index"`
	RefreshHash string    `gorm:"size:64"`
	ExpiresAt   time.Time `gorm:"index"`
	Revoked     bool
}
