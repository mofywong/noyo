package inspection

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func trimReason(s string) string { return strings.TrimSpace(s) }

func (s *Service) RequestOrder(scope Scope, runID, itemID string, actor Actor, templateID uint, form map[string]any) (OrderIntent, error) {
	return s.RequestFindingOrder(scope, runID, itemID, "", actor, templateID, form, "")
}

// Upgrade existing observations without deleting or combining historical orders.
// A legacy grouped order remains reachable from each split problem cycle.
func (s *Service) migrateFindings() error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var items []Item
		if err := tx.Where("status = ? AND NOT EXISTS (SELECT 1 FROM inspection_problem_observations o WHERE o.item_id = inspection_items.id)", "done").Order("checked_at,id").Find(&items).Error; err != nil {
			return err
		}
		for _, i := range items {
			var r Run
			if err := tx.First(&r, "id = ?", i.RunID).Error; err != nil {
				return err
			}
			if i.EpisodeID != "" {
				var legacy Episode
				if err := tx.First(&legacy, "id = ?", i.EpisodeID).Error; err != nil {
					return err
				}
				var codes []string
				raw := i.IssuesJSON
				if i.ReviewStatus == "confirmed" {
					raw = i.ReviewedIssuesJSON
				}
				_ = json.Unmarshal([]byte(raw), &codes)
				sort.Strings(codes)
				checks := problemChecks(r, i)
				for _, code := range codes {
					key, ok := checks[code]
					if !ok {
						continue
					}
					e := legacy
					if legacy.IssueCode != "" && (legacy.IssueCode != code || legacy.Fingerprint != key) {
						e = Episode{}
						err := tx.First(&e, "legacy_id = ? AND fingerprint = ?", legacy.ID, key).Error
						if errors.Is(err, gorm.ErrRecordNotFound) {
							e = Episode{ID: uuid.NewString(), Scope: r.Scope, LegacyID: legacy.ID, CreatedAt: legacy.CreatedAt, Passes: legacy.Passes}
							if err = tx.Create(&e).Error; err != nil {
								return err
							}
						} else if err != nil {
							return err
						}
					}
					e.Fingerprint = key
					e.IssueCode = code
					e.CameraCode = i.CameraCode
					e.ActiveKey = nil
					if err := tx.Save(&e).Error; err != nil {
						return err
					}
					if e.ID == legacy.ID {
						legacy = e
					}
					obs := problemObservation{ItemID: i.ID, Fingerprint: key, Code: code, OccurredAt: observationTime(i, s.Now()), Result: "issue", EpisodeID: e.ID}
					if err := tx.Create(&obs).Error; err != nil {
						return err
					}
				}
			}
			if err := s.episode(tx, r, &i); err != nil {
				return err
			}
			if err := tx.Model(&Item{}).Where("id = ?", i.ID).Update("episode_id", i.EpisodeID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Finding associates a single normalized problem with its own recovery cycle.
// EpisodeID on Item is retained only for older clients with one problem.
type Finding struct {
	ItemID          string `gorm:"primaryKey;size:36" json:"item_id"`
	Code            string `gorm:"primaryKey;size:160" json:"code"`
	EpisodeID       string `gorm:"index" json:"episode_id"`
	Fingerprint     string `gorm:"index" json:"fingerprint"`
	LegacyEpisodeID string `gorm:"-" json:"legacy_episode_id,omitempty"`
}

func (Finding) TableName() string { return "inspection_findings" }

// An observation is keyed by the item and the equivalent check. Review changes
// its judgement, never its observation time. Pending/rejected findings cannot
// become a successful check merely because review arrived late.
type problemObservation struct {
	ItemID      string `gorm:"primaryKey;size:36"`
	Fingerprint string `gorm:"primaryKey;size:64;index:inspection_observation_time"`
	Code        string
	OccurredAt  time.Time `gorm:"index:inspection_observation_time"`
	Result      string
	EpisodeID   string
}

func (problemObservation) TableName() string { return "inspection_problem_observations" }

func problemChecks(r Run, i Item) map[string]string {
	out := map[string]string{}
	if i.Position < 0 || i.Position >= len(r.Spec().Cameras) {
		return out
	}
	c := r.Spec().Cameras[i.Position]
	if r.Mode == "manual" {
		for _, issue := range r.Spec().Issues {
			out[issue.Code] = "manual:" + issue.Code
		}
	} else {
		for _, scene := range c.Scenes {
			out[scene.Code+":"+scene.Target] = encoded(scene)
		}
	}
	for code, semantics := range out {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%d/%d/%s/%s/%s/%s", r.TenantID, r.ProjectID, r.Mode, c.Code, code, semantics)))
		out[code] = hex.EncodeToString(sum[:])
	}
	return out
}

func observationTime(i Item, fallback time.Time) time.Time {
	if i.CapturedAt != nil {
		return *i.CapturedAt
	}
	if i.CheckedAt != nil {
		return *i.CheckedAt
	}
	return fallback
}

func (s *Service) episode(tx *gorm.DB, r Run, i *Item) error {
	checks := problemChecks(r, *i)
	var issues []string
	_ = json.Unmarshal([]byte(i.IssuesJSON), &issues)
	detected := map[string]bool{}
	for _, code := range issues {
		detected[code] = true
	}
	if r.Mode == "ai" && i.ReviewStatus == "confirmed" {
		_ = json.Unmarshal([]byte(i.ReviewedIssuesJSON), &issues)
	}
	hit := map[string]bool{}
	for _, code := range issues {
		hit[code] = true
	}
	scenes := map[string]SceneResult{}
	var results []SceneResult
	_ = json.Unmarshal([]byte(i.SceneResultsJSON), &results)
	for _, result := range results {
		scenes[result.Code] = result
	}
	codes := make([]string, 0, len(checks))
	for code := range checks {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		judgement := "unable"
		if result, ok := scenes[code]; ok {
			judgement = result.Result
			if judgement == "issue" {
				judgement = "unable"
			}
		} else if i.Result == "pass" || (i.Result == "issue" && i.Coverage != "partial") {
			judgement = "pass"
		}
		if hit[code] {
			judgement = "issue"
			if r.Mode == "ai" && i.ReviewStatus != "confirmed" {
				judgement = "pending"
			}
		}
		if r.Mode == "ai" && i.ReviewStatus == "confirmed" && detected[code] && !hit[code] {
			judgement = "unable"
		}
		if r.Mode == "ai" && i.ReviewStatus == "rejected" && i.Result == "issue" {
			judgement = "unable"
		}
		obs := problemObservation{ItemID: i.ID, Fingerprint: checks[code], Code: code, OccurredAt: observationTime(*i, s.Now()), Result: judgement}
		var prior problemObservation
		priorErr := tx.First(&prior, "item_id = ? AND fingerprint = ?", i.ID, checks[code]).Error
		if priorErr != nil && !errors.Is(priorErr, gorm.ErrRecordNotFound) {
			return priorErr
		}
		if priorErr == nil && prior.Result == judgement {
			if judgement != "issue" {
				continue
			}
			var linked int64
			if err := tx.Model(&Finding{}).Where("item_id = ? AND code = ?", i.ID, code).Count(&linked).Error; err != nil {
				return err
			}
			if linked > 0 {
				continue
			}
		}
		appendOnly := false
		if errors.Is(priorErr, gorm.ErrRecordNotFound) {
			var latest problemObservation
			err := tx.Where("fingerprint = ?", checks[code]).Order("occurred_at DESC,item_id DESC").First(&latest).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			appendOnly = errors.Is(err, gorm.ErrRecordNotFound) || obs.OccurredAt.After(latest.OccurredAt)
		}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "item_id"}, {Name: "fingerprint"}}, DoUpdates: clause.AssignmentColumns([]string{"result"})}).Create(&obs).Error; err != nil {
			return err
		}
		var err error
		if appendOnly {
			err = s.appendObservation(tx, r, i.CameraCode, obs)
		} else {
			err = s.reconcileProblem(tx, r, i.CameraCode, code, checks[code])
		}
		if err != nil {
			return err
		}
	}
	if err := tx.Where("item_id = ?", i.ID).Order("code").Find(&i.Findings).Error; err != nil {
		return err
	}
	i.EpisodeID = ""
	if len(i.Findings) > 0 {
		i.EpisodeID = i.Findings[0].EpisodeID
	}
	return nil
}

// The usual chronological path touches only the active cycle. Replay is needed
// only for a changed review or an out-of-order observation, not every poll/run.
func (s *Service) appendObservation(tx *gorm.DB, r Run, camera string, o problemObservation) error {
	var e Episode
	err := tx.First(&e, "active_key = ?", o.Fingerprint).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if o.Result != "issue" {
			return nil
		}
		key := o.Fingerprint
		e = Episode{ID: uuid.NewString(), Scope: r.Scope, Fingerprint: key, ActiveKey: &key, CameraCode: camera, IssueCode: o.Code}
		if err = tx.Create(&e).Error; err != nil {
			return err
		}
	}
	if o.Result == "pass" {
		e.Passes++
		if e.Passes >= 2 {
			e.ActiveKey = nil
		}
	} else {
		e.Passes = 0
	}
	if err = tx.Save(&e).Error; err != nil {
		return err
	}
	if o.Result != "issue" {
		return nil
	}
	if err = tx.Model(&problemObservation{}).Where("item_id = ? AND fingerprint = ?", o.ItemID, o.Fingerprint).Update("episode_id", e.ID).Error; err != nil {
		return err
	}
	return tx.Create(&Finding{ItemID: o.ItemID, Code: o.Code, EpisodeID: e.ID, Fingerprint: o.Fingerprint}).Error
}

// Replaying only this equivalent check makes delayed review independent of
// arrival order. Existing episode IDs and work orders are preserved.
func (s *Service) reconcileProblem(tx *gorm.DB, r Run, camera, code, key string) error {
	var rows []problemObservation
	if err := tx.Where("fingerprint = ?", key).Order("occurred_at,item_id").Find(&rows).Error; err != nil {
		return err
	}
	if err := tx.Model(&Episode{}).Where("fingerprint = ?", key).Update("active_key", nil).Error; err != nil {
		return err
	}
	start := 0
	for start < len(rows) {
		for start < len(rows) && rows[start].Result == "pass" {
			start++
		}
		if start == len(rows) {
			break
		}
		end, passes := start, 0
		for end < len(rows) {
			if rows[end].Result == "pass" {
				passes++
			} else {
				passes = 0
			}
			end++
			if passes >= 2 {
				break
			}
		}
		var selected string
		hasIssue := false
		for _, o := range rows[start:end] {
			if o.Result == "issue" {
				hasIssue = true
				if selected == "" {
					selected = o.EpisodeID
				}
			}
		}
		if hasIssue {
			var e Episode
			if selected != "" {
				if err := tx.First(&e, "id = ?", selected).Error; err != nil {
					return err
				}
			} else {
				e = Episode{ID: uuid.NewString(), Scope: r.Scope, Fingerprint: key, CameraCode: camera, IssueCode: code}
				if err := tx.Create(&e).Error; err != nil {
					return err
				}
			}
			e.Passes = passes
			e.ActiveKey = nil
			if passes < 2 {
				e.ActiveKey = &key
			}
			if err := tx.Save(&e).Error; err != nil {
				return err
			}
			for _, o := range rows[start:end] {
				if o.Result != "issue" {
					continue
				}
				// Never rewrite a historic association that already created an order.
				linkedID := e.ID
				if o.EpisodeID != "" {
					linkedID = o.EpisodeID
				}
				if err := tx.Model(&problemObservation{}).Where("item_id = ? AND fingerprint = ?", o.ItemID, key).Update("episode_id", linkedID).Error; err != nil {
					return err
				}
				f := Finding{ItemID: o.ItemID, Code: code, EpisodeID: linkedID, Fingerprint: key}
				if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&f).Error; err != nil {
					return err
				}
			}
		}
		start = end
	}
	return nil
}

func (s *Service) RequestFindingOrder(scope Scope, runID, itemID, code string, actor Actor, templateID uint, form map[string]any, reason string, bindings ...OrderBinding) (OrderIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var intent OrderIntent
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var r Run
		var i Item
		if err := scoped(tx, scope).First(&r, "id = ?", runID).Error; err != nil {
			return err
		}
		if err := tx.First(&i, "id = ? AND run_id = ?", itemID, runID).Error; err != nil {
			return err
		}
		if i.Result != "issue" || templateID == 0 || actor.UserID == 0 {
			return ErrInvalid
		}
		if r.Mode == "ai" && i.ReviewStatus != "confirmed" {
			return errors.New("inspection_review_required")
		}
		var fs []Finding
		q := tx.Where("item_id = ?", itemID)
		if code != "" {
			q = q.Where("code = ?", code)
		}
		if err := q.Find(&fs).Error; err != nil {
			return err
		}
		if len(fs) != 1 {
			return errors.New("inspection_issue_required")
		}
		var e Episode
		if err := tx.First(&e, "id = ?", fs[0].EpisodeID).Error; err != nil {
			return err
		}
		err := tx.Where("episode_id = ? OR id = ?", e.ID, e.ID).Order("created_at DESC,id DESC").First(&intent).Error
		if errors.Is(err, gorm.ErrRecordNotFound) && e.LegacyID != "" {
			err = tx.First(&intent, "id = ?", e.LegacyID).Error
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil {
			terminal := intent.WorkOrderStatus == "closed" || intent.WorkOrderStatus == "resolved" || intent.WorkOrderStatus == "cancelled" || intent.WorkOrderStatus == "unavailable"
			if terminal {
				if len([]rune(reason)) > 2000 || len([]rune(trimReason(reason))) == 0 {
					return errors.New("inspection_order_reason_required")
				}
				if e.ActiveKey == nil {
					return ErrConflict
				}
			} else {
				if intent.Status == "failed" && intent.OrderID == 0 {
					if intent.CommandJSON != "" || intent.FormVersion > 0 {
						return errors.New("inspection_order_recovery_required")
					}
					if intent.FormVersion > 0 && intent.TemplateID != templateID {
						return ErrConflict
					}
					intent.Status = "pending"
					intent.ActorID = actor.UserID
					if intent.FormVersion == 0 {
						intent.TemplateID = templateID
						intent.FormJSON = encoded(form)
					}
					intent.Attempts = 0
					intent.Error = ""
					intent.NextAttemptAt = s.Now()
					if err := tx.Save(&intent).Error; err != nil {
						return err
					}
					return touchEpisodeRuns(tx, []string{e.ID})
				}
				return nil
			}
		}
		id := e.ID
		if intent.ID != "" {
			id = uuid.NewString()
		}
		intent = OrderIntent{ID: id, EpisodeID: e.ID, IssueCode: fs[0].Code, Scope: scope, RunID: runID, ItemID: itemID, ActorID: actor.UserID, TemplateID: templateID, Title: r.Name + " · " + i.CameraName + " · " + fs[0].Code, FormJSON: encoded(form), Reason: trimReason(reason), Status: "pending", NextAttemptAt: s.Now()}
		if len(bindings) > 0 {
			binding := bindings[0]
			intent.PolicyVersionID = binding.PolicyVersionID
			intent.ExecutionGrantID = binding.ExecutionGrantID
			intent.FormVersion = binding.FormVersion
			intent.WorkflowVersion = binding.WorkflowVersion
		}
		if len([]rune(intent.Title)) > 256 {
			intent.Title = string([]rune(intent.Title)[:256])
		}
		if err := tx.Create(&intent).Error; err != nil {
			return err
		}
		return touchEpisodeRuns(tx, []string{e.ID})
	})
	return intent, err
}

// Live reports include intents shared by equivalent findings in other runs.
// Invalidate every referring report when that shared information changes.
func touchEpisodeRuns(tx *gorm.DB, ids []string) error {
	var eps []Episode
	if err := tx.Where("id IN ? OR legacy_id IN ?", ids, ids).Find(&eps).Error; err != nil {
		return err
	}
	for _, e := range eps {
		ids = append(ids, e.ID)
		if e.LegacyID != "" {
			ids = append(ids, e.LegacyID)
		}
	}
	return tx.Model(&Run{}).Where("id IN (SELECT i.run_id FROM inspection_items i JOIN inspection_findings f ON f.item_id = i.id WHERE f.episode_id IN ?)", ids).Update("version", gorm.Expr("version + 1")).Error
}
