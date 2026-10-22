package store

import "time"

// IOCScreenAssistantJob is private to the author, screen and project. Input and
// checkpoints contain validated design artifacts, never provider credentials.
type IOCScreenAssistantJob struct {
	ID               uint   `gorm:"primaryKey"`
	TaskID           string `gorm:"uniqueIndex;size:64;not null"`
	TenantID         uint   `gorm:"uniqueIndex:idx_ioc_job_request;not null"`
	ProjectID        uint   `gorm:"uniqueIndex:idx_ioc_job_request;not null"`
	ScreenID         uint   `gorm:"uniqueIndex:idx_ioc_job_request;not null;index"`
	UserID           uint   `gorm:"uniqueIndex:idx_ioc_job_request;not null"`
	RequestID        string `gorm:"uniqueIndex:idx_ioc_job_request;size:128;not null"`
	ConversationID   string `gorm:"size:128;not null;default:'';index"`
	InputHash        string `gorm:"size:64;not null"`
	Input            string `gorm:"type:text;not null"`
	Checkpoints      string `gorm:"type:text;not null;default:'{}'"`
	Result           string `gorm:"type:text;not null;default:''"`
	State            string `gorm:"size:24;not null;index"`
	Stage            string `gorm:"size:32;not null;default:''"`
	Summary          string `gorm:"type:text;not null;default:''"`
	ErrorKey         string `gorm:"size:32;not null;default:''"`
	Version          uint64 `gorm:"not null;default:1"`
	Attempt          uint   `gorm:"not null;default:0"`
	LeaseToken       string `gorm:"size:64;not null;default:''"`
	LeaseUntil       time.Time
	AttemptStartedAt time.Time
	SpentMs          int64 `gorm:"not null;default:0"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (IOCScreenAssistantJob) TableName() string { return "ioc_screen_assistant_jobs" }
