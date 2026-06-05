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
	AppID     uint `gorm:"not null;uniqueIndex:idx_app_role_scope" json:"app_id"`
	RoleID    uint `gorm:"not null;uniqueIndex:idx_app_role_scope" json:"role_id"`
	TenantID  uint `gorm:"not null;uniqueIndex:idx_app_role_scope;index" json:"tenant_id"`
	ProjectID uint `gorm:"not null;default:0;uniqueIndex:idx_app_role_scope;index" json:"project_id"`
}

type AppProjectAccess struct {
	gorm.Model
	AppID     uint `gorm:"not null;uniqueIndex:idx_app_project_access;index" json:"app_id"`
	TenantID  uint `gorm:"not null;uniqueIndex:idx_app_project_access;index" json:"tenant_id"`
	ProjectID uint `gorm:"not null;uniqueIndex:idx_app_project_access;index" json:"project_id"`
}

type AppPermission struct {
	gorm.Model
	AppID        uint `gorm:"not null;uniqueIndex:idx_app_project_permission;index" json:"app_id"`
	TenantID     uint `gorm:"not null;uniqueIndex:idx_app_project_permission;index" json:"tenant_id"`
	ProjectID    uint `gorm:"not null;uniqueIndex:idx_app_project_permission;index" json:"project_id"`
	PermissionID uint `gorm:"not null;uniqueIndex:idx_app_project_permission" json:"permission_id"`
}

type AppDeviceTagPermission struct {
	gorm.Model
	AppID      uint   `gorm:"not null;uniqueIndex:idx_app_project_tag;index" json:"app_id"`
	TenantID   uint   `gorm:"not null;uniqueIndex:idx_app_project_tag;index" json:"tenant_id"`
	ProjectID  uint   `gorm:"not null;uniqueIndex:idx_app_project_tag;index" json:"project_id"`
	TagID      uint   `gorm:"not null;uniqueIndex:idx_app_project_tag" json:"tag_id"`
	Permission string `gorm:"size:16;not null;default:'read'" json:"permission"`
}
