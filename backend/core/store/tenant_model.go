package store

import (
	"time"

	"gorm.io/gorm"
)

type Tenant struct {
	gorm.Model
	Code        string     `gorm:"uniqueIndex;size:64;not null" json:"code"`
	Name        string     `gorm:"size:128;not null" json:"name"`
	Contact     string     `gorm:"size:64" json:"contact"`
	Phone       string     `gorm:"size:32" json:"phone"`
	Email       string     `gorm:"size:128" json:"email"`
	Description string     `json:"description"`
	Logo        string     `gorm:"type:text" json:"logo"`
	LoginSuffix string     `gorm:"uniqueIndex:idx_login_suffix,where:login_suffix != '';size:64" json:"login_suffix"`
	MaxUsers    int        `gorm:"default:0" json:"max_users"`
	MaxDevices  int        `gorm:"default:0" json:"max_devices"`
	ExpiresAt   *time.Time `json:"expires_at"`
}
