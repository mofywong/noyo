package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"noyo/core/screen"
)

// IOCScreenApplyReceipt is committed in the same transaction as its draft.
// Keeping the accepted document makes retries safe even after a restart or a
// subsequent edit. A retry never writes the old candidate into the latest draft.
type IOCScreenApplyReceipt struct {
	ID           uint   `gorm:"primaryKey"`
	ScreenID     uint   `gorm:"not null;uniqueIndex:idx_ioc_screen_apply_request"`
	RequestID    string `gorm:"size:128;not null;uniqueIndex:idx_ioc_screen_apply_request"`
	RequestHash  string `gorm:"size:64;not null"`
	DocumentJSON string `gorm:"type:text;not null"`
	DraftVersion uint64 `gorm:"not null"`
	CreatedBy    uint   `gorm:"not null"`
	CreatedAt    time.Time
}

func (IOCScreenApplyReceipt) TableName() string { return "ioc_screen_apply_receipts" }

// The engine fingerprint is compatibility metadata, not an authoring command.
// Typed binding JSON also gives base comparisons the same optional-field
// normalization as the normal draft save path (without removing valid zeroes).
func screenAuthorDocumentJSON(doc screen.Envelope) (string, error) {
	doc.Engine = screen.Engine{Name: doc.Engine.Name}
	content := make(map[string]interface{}, len(doc.Content))
	for key, value := range doc.Content {
		if key != "noyoAssistantContext" {
			content[key] = value
		}
	}
	doc.Content = content
	return ScreenDocumentJSON(doc)
}

func ScreenApplyDraft(db *gorm.DB, tenantID, projectID, id, actor uint, base uint64, requestID string, baseDoc, candidate screen.Envelope, refs []screen.ResourceRef) (*IOCScreen, bool, error) {
	if base == 0 || !validScreenRequestID(requestID) {
		return nil, false, errors.New("draftVersion and a requestId of 8–128 characters are required")
	}
	baseJSON, err := screenAuthorDocumentJSON(baseDoc)
	if err != nil {
		return nil, false, err
	}
	data, err := ScreenDocumentJSON(candidate)
	if err != nil {
		return nil, false, err
	}
	requestHash := screenHash(fmt.Sprintf("apply:%d:%d:%s:%s", actor, base, baseJSON, data))
	var accepted *IOCScreen
	replayed := false
	findReceipt := func(tx *gorm.DB, row *IOCScreen) error {
		var receipt IOCScreenApplyReceipt
		if err := tx.Where("screen_id = ? AND request_id = ?", id, requestID).First(&receipt).Error; err != nil {
			return err
		}
		if receipt.RequestHash != requestHash {
			return ErrScreenRequestConflict
		}
		copy := *row
		copy.DraftJSON, copy.DraftVersion = receipt.DocumentJSON, receipt.DraftVersion
		accepted, replayed = &copy, true
		return nil
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		row, err := ScreenFind(tx, tenantID, projectID, id)
		if err != nil {
			return err
		}
		if err = findReceipt(tx, row); err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if row.DraftVersion != base {
			return ErrScreenConflict
		}
		var persisted screen.Envelope
		if err = json.Unmarshal([]byte(row.DraftJSON), &persisted); err != nil {
			return err
		}
		persistedJSON, err := screenAuthorDocumentJSON(persisted)
		if err != nil {
			return err
		}
		if baseJSON != persistedJSON {
			return ErrScreenConflict
		}
		result := tx.Model(&IOCScreen{}).Where("id = ? AND tenant_id = ? AND project_id = ? AND draft_version = ?", id, tenantID, projectID, base).
			Updates(map[string]interface{}{"draft_json": data, "draft_version": base + 1, "updated_by": actor, "updated_at": time.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrScreenConflict
		}
		row.DraftJSON, row.DraftVersion, row.UpdatedBy = data, base+1, actor
		if err = screenReplaceRefs(tx, row, "draft", id, refs); err != nil {
			return err
		}
		receipt := IOCScreenApplyReceipt{ScreenID: id, RequestID: requestID, RequestHash: requestHash, DocumentJSON: data, DraftVersion: base + 1, CreatedBy: actor}
		if err = tx.Create(&receipt).Error; err != nil {
			return err
		}
		accepted = row
		return nil
	})
	// Two identical requests can race across connections. Once the winning
	// transaction is committed, return its receipt rather than applying twice.
	if err != nil {
		if row, lookupErr := ScreenFind(db, tenantID, projectID, id); lookupErr == nil {
			if lookupErr = findReceipt(db, row); lookupErr == nil || errors.Is(lookupErr, ErrScreenRequestConflict) {
				return accepted, replayed, lookupErr
			}
		}
	}
	return accepted, replayed, err
}

func ScreenFindApplyReceipt(db *gorm.DB, screenID, actor uint, requestID string) (*IOCScreenApplyReceipt, error) {
	if !validScreenRequestID(requestID) {
		return nil, errors.New("invalid apply request id")
	}
	var receipt IOCScreenApplyReceipt
	err := db.Where("screen_id = ? AND created_by = ? AND request_id = ?", screenID, actor, requestID).First(&receipt).Error
	return &receipt, err
}
