package store

import "gorm.io/gorm"

type Position struct {
	gorm.Model
	TenantID    uint   `gorm:"uniqueIndex:idx_position_tenant_code;not null" json:"tenant_id"`
	Code        string `gorm:"size:64;not null;uniqueIndex:idx_position_tenant_code" json:"code"`
	Name        string `gorm:"size:64;not null" json:"name"`
	Description string `json:"description"`
	Status      int    `gorm:"default:1" json:"status"`
}

type UserPosition struct {
	gorm.Model
	UserID     uint `gorm:"not null;uniqueIndex:idx_user_position" json:"user_id"`
	PositionID uint `gorm:"not null;uniqueIndex:idx_user_position" json:"position_id"`
}

type PositionRole struct {
	gorm.Model
	PositionID uint `gorm:"not null;uniqueIndex:idx_position_role" json:"position_id"`
	RoleID     uint `gorm:"not null;uniqueIndex:idx_position_role" json:"role_id"`
}
