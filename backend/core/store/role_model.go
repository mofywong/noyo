package store

import (
	"gorm.io/gorm"
)

type Role struct {
	gorm.Model
	TenantID    uint   `gorm:"uniqueIndex:idx_role_tenant_project_code;not null;default:0" json:"tenant_id"`
	ProjectID   uint   `gorm:"uniqueIndex:idx_role_tenant_project_code;not null;default:0" json:"project_id"`
	Code        string `gorm:"size:64;not null;uniqueIndex:idx_role_tenant_project_code" json:"code"`
	Name        string `gorm:"size:64;not null" json:"name"`
	Description string `json:"description"`
	DataScope   int    `gorm:"default:5" json:"data_scope"` // 1=All, 2=Project, 3=ProjectAndChildren, 4=Custom, 5=Personal
	IsBuiltin   bool   `gorm:"default:false" json:"is_builtin"`
	IsInherited bool   `gorm:"default:false" json:"is_inherited"`
}

type RoleDeviceTagPermission struct {
	gorm.Model
	RoleID     uint   `gorm:"uniqueIndex:idx_role_project_tag;not null" json:"role_id"`
	ProjectID  uint   `gorm:"uniqueIndex:idx_role_project_tag;not null;default:0;index" json:"project_id"`
	TagID      uint   `gorm:"uniqueIndex:idx_role_project_tag;not null" json:"tag_id"`
	Permission string `gorm:"size:16;not null;default:'read'" json:"permission"` // read, write
}
