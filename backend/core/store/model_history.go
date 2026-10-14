package store

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

const (
	ModelHistoryAdd    = "add"
	ModelHistoryDelete = "delete"
	ModelHistorySwitch = "switch"
)

// ModelHistory is an append-only operation record. Actor fields are snapshots,
// not foreign keys: deleting a user, a model or its files must retain history.
// TenantID records the operation's origin, not ownership of the shared model.
type ModelHistory struct {
	ID               uint      `gorm:"primaryKey;index:idx_model_history_scope,priority:4,sort:desc" json:"id"`
	EventID          string    `gorm:"size:128;not null;uniqueIndex" json:"-"`
	ModelRoot        string    `gorm:"not null;index:idx_model_history_scope,priority:1" json:"-"`
	PackageID        string    `gorm:"size:128;not null;index:idx_model_history_scope,priority:2" json:"packageId"`
	Version          string    `gorm:"size:128;not null" json:"version"`
	PreviousVersion  string    `gorm:"size:128" json:"previousVersion"`
	Action           string    `gorm:"size:16;not null" json:"action"`
	OperatorUserID   uint      `json:"operatorUserId,omitempty"`
	OperatorUsername string    `gorm:"size:64" json:"operatorUsername,omitempty"`
	OperatorName     string    `gorm:"size:128" json:"operatorName"`
	TenantID         uint      `json:"tenantId,omitempty"`
	IP               string    `gorm:"size:64" json:"ip,omitempty"`
	CreatedAt        time.Time `gorm:"not null;index:idx_model_history_scope,priority:3,sort:desc" json:"createdAt"`
}

type ModelHistoryPage struct {
	Items    []ModelHistory `json:"items"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

func modelHistoryRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("model history requires a model root")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("normalize model history root: %w", err)
	}
	root = filepath.Clean(root)
	if runtime.GOOS == "windows" {
		root = strings.ToLower(root)
	}
	return root, nil
}

// CreateModelHistory writes a new permanent record and returns all validation
// and database failures. Callers must handle the error before reporting success.
func CreateModelHistory(entry *ModelHistory) error {
	if DB == nil {
		return errors.New("model history database is not initialized")
	}
	if entry == nil {
		return errors.New("model history entry is required")
	}
	if entry.ID != 0 {
		return errors.New("model history only accepts new records")
	}
	next := *entry
	if strings.TrimSpace(next.EventID) == "" {
		return errors.New("model history event ID is required")
	}
	var err error
	next.ModelRoot, err = modelHistoryRoot(next.ModelRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(next.PackageID) == "" || strings.TrimSpace(next.Version) == "" {
		return errors.New("model history requires a package ID and version")
	}
	switch next.Action {
	case ModelHistoryAdd, ModelHistoryDelete, ModelHistorySwitch:
	default:
		return errors.New("unsupported model history action")
	}
	if next.OperatorName == "" {
		next.OperatorName = next.OperatorUsername
	}
	if next.CreatedAt.IsZero() {
		next.CreatedAt = time.Now().UTC()
	} else {
		next.CreatedAt = next.CreatedAt.UTC()
	}
	write := DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&next)
	if write.Error != nil {
		return fmt.Errorf("write model history: %w", write.Error)
	}
	if write.RowsAffected == 0 {
		if err = DB.Where("event_id = ?", next.EventID).First(&next).Error; err != nil {
			return fmt.Errorf("read existing model history: %w", err)
		}
	}
	*entry = next
	return nil
}

// ListModelHistory includes every operation on every release of this shared
// model, regardless of the actor's tenant. Authorization belongs to the API.
func ListModelHistory(modelRoot, packageID string, page, pageSize int) (ModelHistoryPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}
	result := ModelHistoryPage{Items: []ModelHistory{}, Page: page, PageSize: pageSize}
	if DB == nil {
		return result, errors.New("model history database is not initialized")
	}
	root, err := modelHistoryRoot(modelRoot)
	if err != nil {
		return result, err
	}
	if strings.TrimSpace(packageID) == "" {
		return result, errors.New("model history requires a package ID")
	}
	if page-1 > int(^uint(0)>>1)/pageSize {
		return result, errors.New("model history page is too large")
	}
	query := DB.Model(&ModelHistory{}).Where("model_root = ? AND package_id = ?", root, packageID)
	if err = query.Count(&result.Total).Error; err != nil {
		return result, fmt.Errorf("count model history: %w", err)
	}
	if err = query.Order("created_at DESC").Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&result.Items).Error; err != nil {
		return result, fmt.Errorf("list model history: %w", err)
	}
	return result, nil
}
