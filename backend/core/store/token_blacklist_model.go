package store

import "time"

// TokenBlacklist stores revoked JWT tokens persistently.
type TokenBlacklist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TokenHash string    `gorm:"uniqueIndex;size:64;not null" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
