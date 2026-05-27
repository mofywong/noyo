package store

import "gorm.io/gorm"

type App struct {
	gorm.Model
	TenantID    uint   `gorm:"index;not null" json:"tenant_id"`
	AppID       string `gorm:"uniqueIndex;size:64;not null" json:"app_id"`
	AppKey      string `gorm:"size:128;not null" json:"-"`
	Name        string `gorm:"size:64;not null" json:"name"`
	Description string `json:"description"`
	Status      int    `gorm:"default:1" json:"status"`
	RateLimit   int    `gorm:"default:0" json:"rate_limit"`
}

type AppRole struct {
	gorm.Model
	AppID  uint `gorm:"not null;uniqueIndex:idx_app_role" json:"app_id"`
	RoleID uint `gorm:"not null;uniqueIndex:idx_app_role" json:"role_id"`
}
