package store

import (
	"gorm.io/gorm"
)

type Project struct {
	gorm.Model
	TenantID    uint   `gorm:"index;not null" json:"tenant_id"`
	Code        string `gorm:"size:64;not null;uniqueIndex:idx_project_tenant_code" json:"code"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Status      int    `gorm:"default:1" json:"status"`
	Description string `json:"description"`
}
