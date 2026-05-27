package store

import "time"

type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index;not null" json:"tenant_id"`
	UserID    uint      `gorm:"index" json:"user_id"`
	Username  string    `gorm:"size:64" json:"username"`
	AppID     string    `gorm:"size:64" json:"app_id"`
	Module    string    `gorm:"size:32;index" json:"module"`
	Action    string    `gorm:"size:32" json:"action"`
	Resource  string    `gorm:"size:128" json:"resource"`
	Detail    string    `gorm:"type:text" json:"detail"`
	IP        string    `gorm:"size:64" json:"ip"`
	UserAgent string    `gorm:"size:256" json:"user_agent"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
