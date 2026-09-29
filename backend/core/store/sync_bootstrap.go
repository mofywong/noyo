package store

// SyncBootstrap marks the atomic initial outbox baseline for one persistent
// change-log instance. Normal restarts must not append that baseline again.
type SyncBootstrap struct {
	NodeID      string `gorm:"primaryKey;size:128"`
	InstanceID  string `gorm:"size:64;not null"`
	CompletedAt int64  `gorm:"not null"`
}
