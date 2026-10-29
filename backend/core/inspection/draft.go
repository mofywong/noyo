package inspection

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

type DraftData struct {
	OrderBinding  OrderBinding   `json:"order_binding"`
	Issues        []string       `json:"issues"`
	Detail        string         `json:"detail"`
	TemplateID    uint           `json:"template_id"`
	OrderFormData map[string]any `json:"order_form_data"`
	CreateNow     bool           `json:"create_now"`
}
type DraftInput struct {
	RunVersion uint      `json:"run_version"`
	Version    uint      `json:"version"`
	Data       DraftData `json:"data"`
}
type ManualDraft struct {
	Scope      `gorm:"embedded"`
	RunID      string    `gorm:"primaryKey;size:36" json:"run_id"`
	ItemID     string    `gorm:"primaryKey;size:36" json:"item_id"`
	UserID     uint      `gorm:"primaryKey" json:"user_id"`
	Version    uint      `json:"version"`
	RunVersion uint      `json:"run_version"`
	DataJSON   string    `gorm:"type:text" json:"-"`
	Data       DraftData `gorm:"-" json:"data"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (ManualDraft) TableName() string           { return "inspection_manual_drafts" }
func (d *ManualDraft) AfterFind(*gorm.DB) error { return json.Unmarshal([]byte(d.DataJSON), &d.Data) }

func draftItem(tx *gorm.DB, scope Scope, runID, itemID string, actor Actor) (Run, error) {
	var r Run
	if err := scoped(tx, scope).First(&r, "id = ?", runID).Error; err != nil {
		return r, err
	}
	if actor.UserID == 0 || r.Actor.UserID != actor.UserID {
		return r, ErrOwner
	}
	if r.Mode != "manual" || (r.Status != "running" && r.Status != "paused") {
		return r, ErrConflict
	}
	var i Item
	if err := tx.First(&i, "id = ? AND run_id = ? AND position = ? AND status = ?", itemID, runID, r.Current, "pending").Error; err != nil {
		return r, err
	}
	return r, nil
}
func (s *Service) LoadDraft(scope Scope, runID, itemID string, actor Actor) (ManualDraft, error) {
	r, err := draftItem(s.DB, scope, runID, itemID, actor)
	if err != nil {
		return ManualDraft{}, err
	}
	d := ManualDraft{Scope: scope, RunID: runID, ItemID: itemID, UserID: actor.UserID, RunVersion: r.Version}
	err = scoped(s.DB, scope).First(&d, "run_id = ? AND item_id = ? AND user_id = ?", runID, itemID, actor.UserID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return d, nil
	}
	return d, err
}
func (s *Service) SaveDraft(scope Scope, runID, itemID string, actor Actor, input DraftInput) (ManualDraft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out ManualDraft
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		r, err := draftItem(tx, scope, runID, itemID, actor)
		if err != nil {
			return err
		}
		if r.Version != input.RunVersion {
			return ErrConflict
		}
		if len([]rune(input.Data.Detail)) > 2000 || len(input.Data.Issues) > 40 || len(encoded(input.Data)) > 128*1024 {
			return ErrInvalid
		}
		allowed := map[string]bool{}
		for _, i := range r.Spec().Issues {
			allowed[i.Code] = true
		}
		seen := map[string]bool{}
		for _, code := range input.Data.Issues {
			if !allowed[code] || seen[code] {
				return ErrInvalid
			}
			seen[code] = true
		}
		out = ManualDraft{Scope: scope, RunID: runID, ItemID: itemID, UserID: actor.UserID, Version: input.Version + 1, RunVersion: r.Version, Data: input.Data, DataJSON: encoded(input.Data), UpdatedAt: s.Now()}
		if input.Version == 0 {
			var count int64
			if err := tx.Model(&ManualDraft{}).Where("run_id = ? AND item_id = ? AND user_id = ?", runID, itemID, actor.UserID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return ErrConflict
			}
			return tx.Create(&out).Error
		}
		res := tx.Model(&ManualDraft{}).Where("run_id = ? AND item_id = ? AND user_id = ? AND version = ?", runID, itemID, actor.UserID, input.Version).Updates(map[string]any{"version": out.Version, "run_version": r.Version, "data_json": out.DataJSON, "updated_at": out.UpdatedAt})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	})
	return out, err
}
