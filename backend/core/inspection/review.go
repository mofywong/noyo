package inspection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReviewEvent struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	RunID      string    `gorm:"index" json:"run_id"`
	ItemID     string    `json:"item_id"`
	Version    uint      `json:"version"`
	Status     string    `json:"status"`
	IssuesJSON string    `gorm:"type:text" json:"issues_json"`
	Note       string    `json:"note"`
	Actor      Actor     `gorm:"embedded;embeddedPrefix:actor_" json:"actor"`
	CreatedAt  time.Time `json:"created_at"`
}

func (ReviewEvent) TableName() string { return "inspection_reviews" }

type ReviewInput struct {
	Version uint     `json:"version"`
	Status  string   `json:"status"`
	Issues  []string `json:"issues"`
	Note    string   `json:"note"`
}

func ReviewRequired(r Run, i Item) bool {
	return r.Mode == "ai" && i.Result == "issue" && (i.ReviewStatus == "pending" || i.ReviewStatus == "")
}
func (s *Service) Review(scope Scope, runID, itemID string, actor Actor, input ReviewInput) error {
	if actor.UserID == 0 || (input.Status != "confirmed" && input.Status != "rejected") || len([]rune(input.Note)) > 2000 || (input.Status == "rejected" && strings.TrimSpace(input.Note) == "") {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var r Run
		var item Item
		if err := scoped(tx, scope).First(&r, "id = ?", runID).Error; err != nil {
			return err
		}
		if err := tx.First(&item, "id = ? AND run_id = ?", itemID, runID).Error; err != nil {
			return err
		}
		if !ReviewRequired(r, item) || item.ReviewVersion != input.Version {
			return ErrConflict
		}
		if item.Position < 0 || item.Position >= len(r.Spec().Cameras) {
			return ErrInvalid
		}
		allowed := map[string]bool{}
		for _, issue := range r.Spec().Issues {
			allowed[issue.Code] = true
		}
		for _, scene := range r.Spec().Cameras[item.Position].Scenes {
			allowed[scene.Code+":"+scene.Target] = true
		}
		if input.Status == "confirmed" && len(input.Issues) == 0 {
			return ErrInvalid
		}
		if len(input.Issues) > 40 {
			return ErrInvalid
		}
		seen := map[string]bool{}
		for _, code := range input.Issues {
			if !allowed[code] || seen[code] {
				return ErrInvalid
			}
			seen[code] = true
		}
		if input.Status == "rejected" {
			input.Issues = nil
		}
		now := s.Now()
		item.ReviewStatus = input.Status
		item.ReviewVersion++
		item.ReviewedBy = actor
		item.ReviewedAt = &now
		item.ReviewNote = input.Note
		item.ReviewedIssuesJSON = encoded(input.Issues)
		if input.Status == "confirmed" {
			effective := item
			if err := s.episode(tx, r, &effective); err != nil {
				return err
			}
			item.EpisodeID = effective.EpisodeID
			item.Findings = effective.Findings
			// 人工确认即满足建单前置条件；是否建单只看计划里的 auto_order。
			if r.Spec().AutoOrder {
				if err := s.queueAutoOrder(tx, r, item); err != nil {
					return err
				}
			}
		} else {
			if err := s.episode(tx, r, &item); err != nil {
				return err
			}
		}
		if err := tx.Save(&item).Error; err != nil {
			return err
		}
		if err := tx.Create(&ReviewEvent{ID: uuid.NewString(), RunID: runID, ItemID: itemID, Version: item.ReviewVersion, Status: input.Status, IssuesJSON: item.ReviewedIssuesJSON, Note: input.Note, Actor: actor, CreatedAt: now}).Error; err != nil {
			return err
		}
		return tx.Model(&r).Update("version", gorm.Expr("version + 1")).Error
	})
}
// orderBlocksRebuild reports whether a problem still owns an unfinished work
// order. A new order is only allowed once every order of the same problem has
// reached a terminal state; pending intents, unknown statuses and deleted
// orders referenced by a live intent still count as occupied so that a single
// inspection pass cannot enqueue duplicates.
func orderBlocksRebuild(tx *gorm.DB, intents []OrderIntent) (bool, OrderIntent) {
	for _, intent := range intents {
		status := intent.WorkOrderStatus
		if intent.OrderID != 0 {
			var row struct{ Status string }
			res := tx.Table("work_orders").Select("status").Where("id = ? AND deleted_at IS NULL", intent.OrderID).Scan(&row)
			if res.Error == nil {
				if res.RowsAffected == 0 {
					status = "unavailable"
				} else if row.Status != "" {
					status = row.Status
				}
			}
		}
		switch status {
		case "closed", "resolved", "cancelled", "unavailable":
			continue
		}
		return true, intent
	}
	return false, OrderIntent{}
}

func (s *Service) queueAutoOrder(tx *gorm.DB, r Run, i Item) error {
	spec := r.Spec()
	for _, finding := range i.Findings {
		var e Episode
		if err := tx.First(&e, "id = ?", finding.EpisodeID).Error; err != nil {
			return err
		}
		if e.ActiveKey == nil {
			continue
		}
		ids := []string{e.ID}
		if e.LegacyID != "" {
			ids = append(ids, e.LegacyID)
		}
		var previous []OrderIntent
		if err := tx.Where("episode_id IN ? OR id IN ?", ids, ids).Order("created_at DESC, id DESC").Find(&previous).Error; err != nil {
			return err
		}
		if blocked, _ := orderBlocksRebuild(tx, previous); blocked {
			continue
		}
		binding := spec.OrderBinding
		if override, ok := spec.IssueOrderBindings[finding.Code]; ok {
			binding = override
		}
		if binding.TemplateID == 0 {
			binding.TemplateID = spec.TemplateID
		}
		actorID := binding.AuthorizedBy
		if actorID == 0 {
			actorID = r.Actor.UserID
		}
		form := spec.OrderFormData
		if binding.FormData != nil {
			form = binding.FormData
		}
		// 同一问题的工单全部结束后，本轮巡检重新建单：主键必须换新，否则会与
		// 已存在的意图主键冲突（首单历史上以 episode_id 作主键）。
		id := e.ID
		reason := ""
		if len(previous) > 0 {
			id = uuid.NewString()
			reason = "上一工单已结束，本轮巡检仍检出该问题"
			if code := previous[0].OrderCode; code != "" {
				reason = "上一工单 " + code + " 已结束，本轮巡检仍检出该问题"
			}
		}
		// 标题带上问题码，同一摄像机检出多个问题时工单才可区分。
		title := r.Name + " · " + i.CameraName + " · " + finding.Code
		if len([]rune(title)) > 256 {
			title = string([]rune(title)[:256])
		}
		if err := tx.Create(&OrderIntent{PolicyVersionID: binding.PolicyVersionID, ExecutionGrantID: binding.ExecutionGrantID, FormVersion: binding.FormVersion, WorkflowVersion: binding.WorkflowVersion, ID: id, EpisodeID: e.ID, IssueCode: finding.Code, Scope: r.Scope, RunID: r.ID, ItemID: i.ID, TemplateID: binding.TemplateID, ActorID: actorID, Title: title, FormJSON: encoded(form), Reason: reason, Status: "pending", NextAttemptAt: s.Now()}).Error; err != nil {
			return err
		}
		if err := touchEpisodeRuns(tx, []string{e.ID}); err != nil {
			return err
		}
	}
	return nil
}

type ReportVersion struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	Scope     `gorm:"embedded"`
	RunID     string    `gorm:"uniqueIndex:inspection_report_version" json:"run_id"`
	Version   uint      `gorm:"uniqueIndex:inspection_report_version" json:"version"`
	Hash      string    `json:"hash"`
	Snapshot  string    `gorm:"type:text" json:"-"`
	Actor     Actor     `gorm:"embedded;embeddedPrefix:actor_" json:"actor"`
	CreatedAt time.Time `json:"created_at"`
	PDF       []byte    `gorm:"type:blob" json:"-"`
	PDFHash   string    `json:"pdf_hash,omitempty"`
}

func (ReportVersion) TableName() string { return "inspection_report_versions" }
func (s *Service) Versions(scope Scope, runID string) ([]ReportVersion, error) {
	var r Run
	if err := scoped(s.DB, scope).First(&r, "id = ?", runID).Error; err != nil {
		return nil, err
	}
	var rows []ReportVersion
	err := scoped(s.DB, scope).Omit("snapshot", "pdf").Where("run_id = ?", runID).Order("version DESC").Find(&rows).Error
	return rows, err
}
func (s *Service) Freeze(scope Scope, runID string, actor Actor) (ReportVersion, error) {
	var version ReportVersion
	if actor.UserID == 0 {
		return version, ErrOwner
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		report, err := reportIn(tx, scope, runID)
		if err != nil {
			return err
		}
		if report.Run.Status != "completed" && report.Run.Status != "partial" {
			return ErrConflict
		}
		for _, item := range report.Items {
			if ReviewRequired(report.Run, item) {
				return errors.New("inspection_review_required")
			}
		}
		snapshot := encoded(report)
		sum := sha256.Sum256([]byte(snapshot))
		hash := hex.EncodeToString(sum[:])
		var last ReportVersion
		err = scoped(tx, scope).Omit("snapshot", "pdf").Where("run_id = ?", runID).Order("version DESC").First(&last).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if last.ID != "" && last.Hash == hash {
			version = last
			return nil
		}
		version = ReportVersion{ID: uuid.NewString(), Scope: scope, RunID: runID, Version: last.Version + 1, Hash: hash, Snapshot: snapshot, Actor: actor, CreatedAt: s.Now()}
		if err := tx.Create(&version).Error; err != nil {
			return err
		}
		return s.pinEvidence(tx, version.ID, report.Items)
	})
	return version, err
}
func (s *Service) Archived(scope Scope, runID string, revision uint) (ReportVersion, Report, error) {
	var row ReportVersion
	var report Report
	err := scoped(s.DB, scope).First(&row, "run_id = ? AND version = ?", runID, revision).Error
	if err == nil {
		err = json.Unmarshal([]byte(row.Snapshot), &report)
	}
	if err == nil {
		err = s.normalizeArchivedEvidence(row.ID, &report)
	}
	if err == nil {
		metadata := row
		metadata.Snapshot = ""
		metadata.PDF = nil
		report.Archive = &metadata
	}
	return row, report, err
}
func (s *Service) ArchivePDF(scope Scope, id string, pdf []byte) ([]byte, error) {
	if len(pdf) < 5 || len(pdf) > 20*1024*1024 || string(pdf[:5]) != "%PDF-" {
		return nil, ErrInvalid
	}
	sum := sha256.Sum256(pdf)
	result := scoped(s.DB.Model(&ReportVersion{}), scope).Where("id = ? AND (pdf IS NULL OR length(pdf) = 0)", id).Updates(map[string]any{"pdf": pdf, "pdf_hash": hex.EncodeToString(sum[:])})
	if result.Error != nil {
		return nil, result.Error
	}
	var row ReportVersion
	if err := scoped(s.DB, scope).Select("id", "pdf").First(&row, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return row.PDF, nil
}

// Reinspect preserves the original plan revision and creates a separate run for
// incompletely inspected cameras. Its token makes a network retry safe.
func (s *Service) Reinspect(scope Scope, runID string, actor Actor, token string) (Run, error) {
	if actor.UserID == 0 || len(token) < 8 || len(token) > 64 || strings.ContainsAny(token, ":/ ") {
		return Run{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	report, err := s.Report(scope, runID)
	if err != nil {
		return Run{}, err
	}
	slot := runID + ":retry:" + token
	var existing Run
	err = scoped(s.DB, scope).First(&existing, "slot = ? AND actor_user_id = ?", slot, actor.UserID).Error
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Run{}, err
	}
	if report.Run.Status != "completed" && report.Run.Status != "partial" {
		return Run{}, ErrConflict
	}
	spec := report.Spec
	spec.Cameras = nil
	spec.Schedules = nil
	for _, item := range report.Items {
		if NeedsReinspection(item) {
			spec.Cameras = append(spec.Cameras, report.Spec.Cameras[item.Position])
		}
	}
	if len(spec.Cameras) == 0 {
		return Run{}, ErrInvalid
	}
	plan := Plan{ID: report.Run.PlanID, Scope: scope, Revision: report.Run.Revision, Name: report.Run.Name, Mode: report.Run.Mode, SpecJSON: encoded(spec)}
	return s.startPlan(plan, actor, slot, runID)
}
