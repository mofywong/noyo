package store

import "time"

// Versions are immutable: published screens always resolve the pinned version.
type IOCDataSet struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TenantID  uint      `gorm:"index:idx_ioc_dataset_scope" json:"-"`
	ProjectID uint      `gorm:"index:idx_ioc_dataset_scope" json:"-"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Version   uint64    `gorm:"not null" json:"version"`
	CreatedBy uint      `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
type IOCDataSetVersion struct {
	ID        uint      `gorm:"primaryKey" json:"-"`
	DatasetID uint      `gorm:"uniqueIndex:idx_ioc_dataset_version" json:"datasetId"`
	Version   uint64    `gorm:"uniqueIndex:idx_ioc_dataset_version" json:"version"`
	PlanJSON  string    `gorm:"type:text;not null" json:"-"`
	CreatedAt time.Time `json:"createdAt"`
}
