package store

import "time"

// Conversations are private to an author within a screen and its scope.
type IOCScreenMessage struct {
	ID             uint   `gorm:"primaryKey"`
	TenantID       uint   `gorm:"uniqueIndex:idx_ioc_message;not null"`
	ProjectID      uint   `gorm:"uniqueIndex:idx_ioc_message;not null"`
	ScreenID       uint   `gorm:"uniqueIndex:idx_ioc_message;not null;index"`
	UserID         uint   `gorm:"uniqueIndex:idx_ioc_message;not null"`
	MessageID      string `gorm:"uniqueIndex:idx_ioc_message;size:128;not null"`
	ConversationID string `gorm:"size:128;not null;default:'';index"`
	Payload        string `gorm:"type:text;not null"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (IOCScreenMessage) TableName() string { return "ioc_screen_messages" }

// A conversation is private to one author, screen, project and tenant. Empty
// titles are translated by the client until the first user message names it.
type IOCScreenConversation struct {
	ID             uint   `gorm:"primaryKey"`
	TenantID       uint   `gorm:"uniqueIndex:idx_ioc_conversation;not null"`
	ProjectID      uint   `gorm:"uniqueIndex:idx_ioc_conversation;not null"`
	ScreenID       uint   `gorm:"uniqueIndex:idx_ioc_conversation;not null;index"`
	UserID         uint   `gorm:"uniqueIndex:idx_ioc_conversation;not null"`
	ConversationID string `gorm:"uniqueIndex:idx_ioc_conversation;size:128;not null"`
	Title          string `gorm:"size:320;not null;default:''"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (IOCScreenConversation) TableName() string { return "ioc_screen_conversations" }
