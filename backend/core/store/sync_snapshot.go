package store

// Staging is durable and never replaces live rows until the complete manifest
// has been validated. One in-flight snapshot is bound to both log generations.
type SyncSnapshotStage struct {
	PeerCode      string `gorm:"primaryKey;size:128"`
	RequestCode   string `gorm:"size:128;not null"`
	PeerInstance  string `gorm:"size:64;not null"`
	LocalInstance string `gorm:"size:64;not null"`
	SessionToken  string `gorm:"size:128;not null"`
	Header        string `gorm:"type:text"`
	Chunks        string `gorm:"type:text"`
	Completed     bool
}
