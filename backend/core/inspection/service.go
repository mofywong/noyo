package inspection

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrConflict = errors.New("inspection_conflict")
var ErrInvalid = errors.New("inspection_invalid")
var ErrWatch = errors.New("inspection_watch_required")
var ErrOwner = errors.New("inspection_owner_required")

type Service struct {
	DB      *gorm.DB
	Now     func() time.Time
	mu      sync.Mutex
	domains sync.Map
}

func New(db *gorm.DB) (*Service, error) {
	if db == nil {
		return nil, errors.New("inspection database unavailable")
	}
	if err := db.AutoMigrate(&ExecutionGrant{}, &OrderIntentRecovery{}, &Plan{}, &Run{}, &Item{}, &Episode{}, &OrderIntent{}, &ReviewEvent{}, &ReportVersion{}, &ReportPDF{}, &Finding{}, &problemObservation{}, &ScheduleOccurrence{}, &EvidenceAsset{}, &evidenceReference{}, &ManualDraft{}); err != nil {
		return nil, err
	}
	if err := normalizeScheduleTimes(db); err != nil {
		return nil, err
	}
	if err := migrateExecutionCounts(db); err != nil {
		return nil, err
	}
	s := &Service{DB: db, Now: time.Now}
	if err := s.migrateDeadlines(); err != nil {
		return nil, err
	}
	if err := s.migrateFindings(); err != nil {
		return nil, err
	}
	if err := s.migrateEvidence(); err != nil {
		return nil, err
	}
	if err := s.migrateAbsenceTiming(); err != nil {
		return nil, err
	}
	return s, nil
}
func scoped(db *gorm.DB, scope Scope) *gorm.DB {
	return db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID)
}
func encoded(v any) string { b, _ := json.Marshal(v); return string(b) }
func Validate(spec *Spec) error {
	if spec.ReviewPolicy == "" {
		spec.ReviewPolicy = "required"
	}
	if spec.ReviewPolicy != "required" && spec.ReviewPolicy != "automatic" {
		return ErrInvalid
	}
	spec.Name = strings.TrimSpace(spec.Name)
	if spec.Name == "" || len([]rune(spec.Name)) > 128 || (spec.Mode != "manual" && spec.Mode != "ai") || len(spec.Cameras) == 0 || len(spec.Cameras) > 200 {
		return ErrInvalid
	}
	if spec.MinimumWatchSeconds < 0 || spec.MinimumWatchSeconds > 300 || len(spec.Issues) > 40 {
		return ErrInvalid
	}
	spec.Timezone = ServerTimezoneName()
	if err := validateSchedules(spec); err != nil {
		return err
	}
	if spec.AutoOrder && spec.TemplateID == 0 {
		return ErrInvalid
	}
	seen := map[string]bool{}
	for ci := range spec.Cameras {
		c := &spec.Cameras[ci]
		if c.Code == "" || seen[c.Code] || c.Domain == "" {
			return ErrInvalid
		}
		seen[c.Code] = true
		if len(c.Scenes) > 20 || (spec.Mode == "ai" && len(c.Scenes) == 0) {
			return ErrInvalid
		}
		sceneKeys := map[string]bool{}
		for si := range c.Scenes {
			scene := &c.Scenes[si]
			if err := NormalizeScene(scene); err != nil {
				return err
			}
			key := scene.Code + ":" + scene.Target
			if sceneKeys[key] {
				return ErrInvalid
			}
			sceneKeys[key] = true
			if !IsObjectDetectionScene(scene.Code) && spec.ReviewPolicy == "automatic" {
				return ErrInvalid
			}
		}
	}
	seen = map[string]bool{}
	for _, i := range spec.Issues {
		if i.Code == "" || len(i.Code) > 64 || i.Name == "" || seen[i.Code] {
			return ErrInvalid
		}
		seen[i.Code] = true
	}
	return nil
}
func (s *Service) SavePlan(scope Scope, actor Actor, id string, revision uint, spec Spec) (Plan, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 || actor.UserID == 0 {
		return Plan{}, ErrInvalid
	}
	if err := Validate(&spec); err != nil {
		return Plan{}, err
	}
	p := Plan{ID: id, Scope: scope, Revision: revision + 1, Name: spec.Name, Mode: spec.Mode, Enabled: spec.Enabled, SpecJSON: encoded(spec), NextDueAt: nextDue(spec, s.Now()), CreatedBy: actor.UserID}
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		p.ID = uuid.NewString()
		p.Revision = 1
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			if err := s.issuePlanGrant(tx, &p, actor); err != nil {
				return err
			}
			return tx.Create(&p).Error
		})
		return p, err
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.issuePlanGrant(tx, &p, actor); err != nil {
			return err
		}
		result := scoped(tx.Model(&Plan{}), scope).Where("id = ? AND revision = ? AND mode = ?", id, revision, spec.Mode).Updates(map[string]any{"revision": p.Revision, "name": p.Name, "mode": p.Mode, "enabled": p.Enabled, "spec_json": p.SpecJSON, "next_due_at": p.NextDueAt, "updated_at": s.Now(), "execution_grant_id": p.ExecutionGrantID})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	})
	if err != nil {
		return Plan{}, err
	}
	return s.Plan(scope, id)
}
func (s *Service) Plans(scope Scope) ([]Plan, error) {
	var rows []Plan
	err := scoped(s.DB, scope).Order("updated_at DESC").Find(&rows).Error
	return rows, err
}
func (s *Service) Plan(scope Scope, id string) (Plan, error) {
	var row Plan
	err := scoped(s.DB, scope).First(&row, "id = ?", id).Error
	return row, err
}
func (s *Service) Start(scope Scope, actor Actor, planID, slot string) (Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.Plan(scope, planID)
	if err != nil {
		return Run{}, err
	}
	if !p.Enabled {
		return Run{}, ErrInvalid
	}
	return s.startPlan(p, actor, slot)
}
func (s *Service) startPlan(p Plan, actor Actor, slot string, source ...string) (Run, error) {
	return s.startPlanIn(s.DB, p, actor, slot, source...)
}
func (s *Service) startPlanIn(db *gorm.DB, p Plan, actor Actor, slot string, source ...string) (Run, error) {
	if slot == "" {
		slot = uuid.NewString()
	}
	r := Run{ID: uuid.NewString(), Scope: p.Scope, PlanID: p.ID, Revision: p.Revision, Name: p.Name, Mode: p.Mode, Status: "queued", Snapshot: p.SpecJSON, Actor: actor, Slot: slot, Version: 1, StartedAt: s.Now()}
	if len(source) == 1 {
		r.SourceRunID = source[0]
	}
	if r.Mode == "manual" {
		if actor.UserID == 0 {
			return Run{}, ErrInvalid
		}
		r.Status = "running"
	} else {
		q := s.Now().Add(5 * time.Minute)
		d := q.Add(runBudget(p.Spec()))
		r.QueueDeadlineAt = &q
		r.DeadlineAt = &d
	}
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := s.issueRunGrant(tx, p, &r); err != nil {
			return err
		}
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		for n, c := range p.Spec().Cameras {
			item := Item{ID: uuid.NewString(), RunID: r.ID, Position: n, CameraCode: c.Code, CameraName: c.Name, Domain: c.Domain, Status: "pending", Stage: "queued", IssuesJSON: "[]"}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return scoped(tx.Model(&Plan{}), p.Scope).Where("id = ?", p.ID).UpdateColumn("execution_count", gorm.Expr("execution_count + 1")).Error
	})
	return r, err
}
func (s *Service) Runs(scope Scope) ([]Run, error) {
	var rows []Run
	err := scoped(s.DB, scope).Order("created_at DESC").Limit(100).Find(&rows).Error
	if err == nil {
		err = PopulateRunSummaries(s.DB, rows)
	}
	return rows, err
}
func (s *Service) Report(scope Scope, id string) (Report, error) {
	return reportIn(s.DB, scope, id)
}
func reportIn(db *gorm.DB, scope Scope, id string) (Report, error) {
	var report Report
	err := scoped(db, scope).First(&report.Run, "id = ?", id).Error
	if err != nil {
		return report, err
	}
	report.Spec = report.Run.Spec()
	if err = db.Where("run_id = ?", id).Order("position").Find(&report.Items).Error; err != nil {
		return report, err
	}
	runs := []Run{report.Run}
	if err = PopulateRunSummaries(db, runs); err != nil {
		return report, err
	}
	report.Run = runs[0]
	episodes := []string{}
	ids := make([]string, 0, len(report.Items))
	for _, item := range report.Items {
		ids = append(ids, item.ID)
	}
	var findings []Finding
	if len(ids) > 0 {
		if err = db.Where("item_id IN ?", ids).Order("code").Find(&findings).Error; err != nil {
			return report, err
		}
	}
	byItem := map[string][]Finding{}
	var episodeRows []Episode
	if len(findings) > 0 {
		fid := []string{}
		for _, f := range findings {
			fid = append(fid, f.EpisodeID)
		}
		if err = db.Where("id IN ?", fid).Find(&episodeRows).Error; err != nil {
			return report, err
		}
	}
	legacyByID := map[string]string{}
	for _, e := range episodeRows {
		legacyByID[e.ID] = e.LegacyID
	}
	for _, f := range findings {
		f.LegacyEpisodeID = legacyByID[f.EpisodeID]
		byItem[f.ItemID] = append(byItem[f.ItemID], f)
	}
	for n := range report.Items {
		item := &report.Items[n]
		item.Findings = byItem[item.ID]
		for _, finding := range item.Findings {
			episodes = append(episodes, finding.EpisodeID)
		}
		if item.EpisodeID != "" {
			episodes = append(episodes, item.EpisodeID)
		}
	}
	if len(episodes) > 0 {
		var legacy []string
		if err = db.Model(&Episode{}).Where("id IN ? AND legacy_id <> ?", episodes, "").Pluck("legacy_id", &legacy).Error; err != nil {
			return report, err
		}
		episodes = append(episodes, legacy...)
		err = scoped(db, scope).Where("id IN ? OR episode_id IN ?", episodes, episodes).Order("created_at,id").Find(&report.Orders).Error
	}
	if err == nil {
		err = db.Where("run_id = ?", id).Order("created_at,id").Find(&report.Reviews).Error
	}
	return report, err
}
func (s *Service) manual(tx *gorm.DB, scope Scope, id string, actor Actor, version uint) (Run, Item, error) {
	var r Run
	var item Item
	if err := scoped(tx, scope).First(&r, "id = ?", id).Error; err != nil {
		return r, item, err
	}
	if r.Actor.UserID != actor.UserID {
		return r, item, ErrOwner
	}
	if r.Mode != "manual" || r.Status != "running" || r.Version != version {
		return r, item, ErrConflict
	}
	err := tx.First(&item, "run_id = ? AND position = ?", id, r.Current).Error
	return r, item, err
}
func (s *Service) Observe(scope Scope, id string, actor Actor, version uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		_, i, err := s.manual(tx, scope, id, actor, version)
		if err != nil {
			return err
		}
		return tx.Model(&Item{}).Where("id = ?", i.ID).Update("observed_at", s.Now()).Error
	})
}
func (s *Service) Submit(scope Scope, id string, actor Actor, input Submission) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		r, i, err := s.manual(tx, scope, id, actor, input.Version)
		if err != nil {
			return err
		}
		spec := r.Spec()
		if input.Result != "pass" && input.Result != "issue" && input.Result != "unable" {
			return ErrInvalid
		}
		if len([]rune(input.Detail)) > 2000 || len(input.Issues) > 40 {
			return ErrInvalid
		}
		if input.Result == "unable" {
			if strings.TrimSpace(input.Detail) == "" {
				return ErrInvalid
			}
		} else if i.ObservedAt == nil || s.Now().Sub(*i.ObservedAt) < time.Duration(spec.MinimumWatchSeconds)*time.Second {
			return ErrWatch
		}
		allowed := map[string]bool{}
		for _, v := range spec.Issues {
			allowed[v.Code] = true
		}
		selected := map[string]bool{}
		for _, v := range input.Issues {
			if !allowed[v] || selected[v] {
				return ErrInvalid
			}
			selected[v] = true
		}
		if (input.Result == "issue") != (len(input.Issues) > 0) {
			return ErrInvalid
		}
		if input.EvidenceJPEG != "" {
			if len(input.EvidenceJPEG) > 350000 {
				return ErrInvalid
			}
			data, err := base64.StdEncoding.DecodeString(input.EvidenceJPEG)
			if err != nil {
				return ErrInvalid
			}
			image, err := jpeg.DecodeConfig(bytes.NewReader(data))
			if err != nil || image.Width > 1280 || image.Height > 1280 {
				return ErrInvalid
			}
			i.EvidenceJSON = encoded(map[string]any{"jpeg_base64": input.EvidenceJPEG, "source": "manual_browser"})
		}
		now := s.Now()
		i.Status = "done"
		i.Result = input.Result
		i.IssuesJSON = encoded(input.Issues)
		i.Detail = input.Detail
		i.CheckedAt = &now
		if err := s.episode(tx, r, &i); err != nil {
			return err
		}
		if i.Result == "unable" {
			i.Coverage = "none"
		} else {
			i.Coverage = "complete"
		}
		if err := s.storeEvidence(tx, r, &i); err != nil {
			return err
		}
		if err := tx.Save(&i).Error; err != nil {
			return err
		}
		if err := tx.Where("run_id = ? AND item_id = ? AND user_id = ?", id, i.ID, actor.UserID).Delete(&ManualDraft{}).Error; err != nil {
			return err
		}
		updates := map[string]any{"current": r.Current + 1, "version": r.Version + 1}
		if r.Current+1 == len(spec.Cameras) {
			updates["status"] = completionStatus(tx, r.ID)
			updates["finished_at"] = now
		}
		res := tx.Model(&Run{}).Where("id = ? AND version = ?", r.ID, r.Version).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	})
}
func completionStatus(tx *gorm.DB, id string) string {
	var n int64
	tx.Model(&Item{}).Where("run_id = ? AND (result = ? OR status <> ? OR coverage = ?)", id, "unable", "done", "partial").Count(&n)
	if n > 0 {
		return "partial"
	}
	return "completed"
}
func fingerprint(r Run, i Item) string {
	spec := r.Spec()
	c := spec.Cameras[i.Position]
	checks := []string{}
	if r.Mode == "manual" {
		for _, issue := range spec.Issues {
			checks = append(checks, issue.Code)
		}
	} else {
		for _, scene := range c.Scenes {
			checks = append(checks, encoded(scene))
		}
	}
	sort.Strings(checks)
	raw := fmt.Sprintf("%d/%d/%s/%s/%s", r.TenantID, r.ProjectID, r.Mode, c.Code, encoded(checks))
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
func (s *Service) Pause(scope Scope, id string, actor Actor, version uint, resume bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	from, to := "running", "paused"
	if resume {
		from, to = to, from
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var r Run
		if err := scoped(tx, scope).First(&r, "id = ?", id).Error; err != nil {
			return err
		}
		if r.Actor.UserID != actor.UserID {
			return ErrOwner
		}
		if r.Mode != "manual" || r.Status != from || r.Version != version {
			return ErrConflict
		}
		if err := tx.Model(&Item{}).Where("run_id = ? AND position = ?", id, r.Current).Update("observed_at", nil).Error; err != nil {
			return err
		}
		return tx.Model(&r).Updates(map[string]any{"status": to, "version": version + 1}).Error
	})
}

// Stable order is used only inside a resource domain. Domains may run concurrently.
func GroupCameras(cameras []Camera) map[string][]Camera {
	out := map[string][]Camera{}
	for _, c := range cameras {
		out[c.Domain] = append(out[c.Domain], c)
	}
	return out
}
func SortedDomains(groups map[string][]Camera) []string {
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
