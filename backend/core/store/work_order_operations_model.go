package store

import (
	"gorm.io/gorm"
	"time"
)

// Published definitions are append-only. Enabled and CurrentVersion live on
// the policy head so revocation does not rewrite historical definitions.
type WorkOrderHandlingPolicy struct {
	gorm.Model
	TenantID       uint   `gorm:"not null;uniqueIndex:idx_wo_policy_code" json:"tenant_id"`
	ProjectID      uint   `gorm:"not null;uniqueIndex:idx_wo_policy_code" json:"project_id"`
	Code           string `gorm:"size:64;not null;uniqueIndex:idx_wo_policy_code" json:"code"`
	Name           string `gorm:"size:128;not null" json:"name"`
	CurrentVersion int    `json:"current_version"`
	Enabled        bool   `json:"enabled"`
}

type WorkOrderHandlingPolicyVersion struct {
	gorm.Model
	TenantID    uint   `gorm:"not null;index" json:"tenant_id"`
	ProjectID   uint   `gorm:"not null;index" json:"project_id"`
	PolicyID    uint   `gorm:"not null;uniqueIndex:idx_wo_policy_version" json:"policy_id"`
	Version     int    `gorm:"not null;uniqueIndex:idx_wo_policy_version" json:"version"`
	Definition  string `gorm:"type:text;not null" json:"definition"`
	PublishedBy uint   `json:"published_by"`
}

type WorkOrderTaskCandidate struct {
	gorm.Model
	TaskID uint `gorm:"not null;uniqueIndex:idx_wo_candidate" json:"task_id"`
	UserID uint `gorm:"not null;uniqueIndex:idx_wo_candidate;index" json:"user_id"`
}

// Grants bind automation to a published policy and an explicit device set.
// Current permissions of the authorizer are still checked by the adapter.
type WorkOrderExecutionGrant struct {
	gorm.Model
	TenantID        uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID       uint       `gorm:"not null;index" json:"project_id"`
	PolicyVersionID uint       `gorm:"not null;index" json:"policy_version_id"`
	AuthorizedBy    uint       `gorm:"not null" json:"authorized_by"`
	DeviceCodes     string     `gorm:"type:text;not null" json:"device_codes"`
	ExpiresAt       time.Time  `gorm:"not null" json:"expires_at"`
	RevokedAt       *time.Time `json:"revoked_at"`
	RevokedBy       uint       `json:"revoked_by"`
	RevokeReason    string     `gorm:"type:text" json:"revoke_reason"`
}

type WorkOrderSLAInstance struct {
	gorm.Model
	TenantID           uint       `gorm:"not null;index" json:"tenant_id"`
	ProjectID          uint       `gorm:"not null;index" json:"project_id"`
	WorkOrderID        uint       `gorm:"not null;uniqueIndex:idx_wo_sla_cycle" json:"work_order_id"`
	WorkflowCycle      int        `gorm:"not null;uniqueIndex:idx_wo_sla_cycle" json:"workflow_cycle"`
	Version            int        `gorm:"not null;default:1" json:"version"`
	PolicySnapshot     string     `gorm:"type:text;not null" json:"policy_snapshot"`
	StartedAt          time.Time  `json:"started_at"`
	ResponseDueAt      time.Time  `gorm:"index" json:"response_due_at"`
	ResolutionDueAt    time.Time  `gorm:"index" json:"resolution_due_at"`
	RespondedAt        *time.Time `json:"responded_at"`
	CompletedAt        *time.Time `json:"completed_at"`
	PausedAt           *time.Time `json:"paused_at"`
	PauseDuration      int64      `gorm:"not null;default:0" json:"pause_duration_ns"`
	PauseReason        string     `json:"pause_reason"`
	ResponseWarning    bool       `json:"response_warning"`
	ResolutionWarning  bool       `json:"resolution_warning"`
	ResponseBreached   bool       `json:"response_breached"`
	ResolutionBreached bool       `json:"resolution_breached"`
}
