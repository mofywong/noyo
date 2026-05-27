package store

import (
	"gorm.io/gorm"
)

type Permission struct {
	gorm.Model
	Code       string `gorm:"uniqueIndex;size:128;not null" json:"code"`
	Name       string `gorm:"size:64;not null" json:"name"`
	Module     string `gorm:"size:32;not null;index" json:"module"`
	Type       string `gorm:"size:16;not null" json:"type"` // menu, button, api
	ParentCode string `gorm:"size:128" json:"parent_code"`
	SortOrder  int    `gorm:"default:0" json:"sort_order"`
}

// UserRoleBinding maps a user to a role within a specific scope (tenant or project)
type UserRoleBinding struct {
	gorm.Model
	UserID    uint `gorm:"uniqueIndex:idx_user_role_binding;not null" json:"user_id"`
	RoleID    uint `gorm:"uniqueIndex:idx_user_role_binding;not null" json:"role_id"`
	TenantID  uint `gorm:"uniqueIndex:idx_user_role_binding;not null" json:"tenant_id"`
	ProjectID uint `gorm:"uniqueIndex:idx_user_role_binding;default:0" json:"project_id"` // 0 means all projects in the tenant
}

// RolePermission maps a role to a permission
type RolePermission struct {
	gorm.Model
	RoleID       uint   `gorm:"uniqueIndex:idx_role_permission" json:"role_id"`
	PermissionID uint   `gorm:"uniqueIndex:idx_role_permission" json:"permission_id"`
}


