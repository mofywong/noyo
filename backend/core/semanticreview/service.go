package semanticreview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// Limits are enforced before a request is reserved. Money is expressed in
// micro-units of the operator's configured billing currency, never a guessed
// price obtained from a provider name. A zero limit means unlimited.
type Limits struct {
	Enabled                 bool  `json:"enabled" yaml:"enabled"`
	QueueSize               int   `json:"queue_size" yaml:"queue_size"`
	Concurrent              int   `json:"concurrent" yaml:"concurrent"`
	MaxFrameBytes           int   `json:"max_frame_bytes" yaml:"max_frame_bytes"`
	MaxEvidenceBytes        int   `json:"max_evidence_bytes" yaml:"max_evidence_bytes"`
	MaxQueueBytes           int64 `json:"max_queue_bytes" yaml:"max_queue_bytes"`
	PlatformDailyCostMicros int64 `json:"platform_daily_cost_micros" yaml:"platform_daily_cost_micros"`
	TenantDailyCostMicros   int64 `json:"tenant_daily_cost_micros" yaml:"tenant_daily_cost_micros"`
	PlatformDailyTokens     int64 `json:"platform_daily_tokens" yaml:"platform_daily_tokens"`
	TenantDailyTokens       int64 `json:"tenant_daily_tokens" yaml:"tenant_daily_tokens"`
}

func (l Limits) normalized() Limits {
	if l.QueueSize > 100 {
		l.QueueSize = 100
	}
	if l.Concurrent > 2 {
		l.Concurrent = 2
	}
	if l.MaxFrameBytes > 1024*1024 {
		l.MaxFrameBytes = 1024 * 1024
	}
	if l.MaxEvidenceBytes > 4*1024*1024 {
		l.MaxEvidenceBytes = 4 * 1024 * 1024
	}
	if l.MaxQueueBytes > 64*1024*1024 {
		l.MaxQueueBytes = 64 * 1024 * 1024
	}
	return l
}

func (l Limits) Validate() error {
	if l.QueueSize < 0 || l.Concurrent < 0 || l.MaxFrameBytes < 0 || l.MaxEvidenceBytes < 0 || l.MaxQueueBytes < 0 || l.PlatformDailyCostMicros < 0 || l.TenantDailyCostMicros < 0 || l.PlatformDailyTokens < 0 || l.TenantDailyTokens < 0 {
		return errors.New("invalid_semantic_limits")
	}
	return nil
}

// Combine scoped caps without turning an unlimited parent into a zero budget.
func minimumLimit(a, b int64) int64 {
	if a == 0 {
		return b
	}
	if b == 0 {
		return a
	}
	return min(a, b)
}

type Policy struct {
	ConfigVersion                            string // exact provider policy plus the platform master revision
	Provider, Model                          string
	ExecutorEpoch                            string
	MaxInputTokens, MaxOutputTokens          int
	WorstCostMicros                          int64
	TenantDailyCostMicros, TenantDailyTokens int64
	Timeout                                  time.Duration
	Verifier                                 Verifier
}
type Verifier interface {
	Verify(context.Context, Candidate) (Result, error)
}
type ScopeResolver func(string) (Scope, error)
type PolicyResolver func(Scope) (Policy, error)

type job struct {
	ID                  string `gorm:"primaryKey;size:64"`
	Dedupe              string `gorm:"uniqueIndex;size:64"`
	Incident            string `gorm:"index;size:64"`
	Target              string `gorm:"index;size:64"`
	DeviceCode          string `gorm:"index;size:64"`
	TenantID, ProjectID uint
	Round               int
	TotalAttempts       int
	CreatedAt           time.Time `gorm:"index"`
	ExpiresAt           time.Time
	Status              string `gorm:"index;size:32"`
	Payload             []byte
	Result              []byte
	Bytes               int64
	Owner               string `gorm:"size:64"`
	LeaseUntil          time.Time
}

func (job) TableName() string { return "semantic_review_jobs" }

type attempt struct {
	ID                         uint      `gorm:"primaryKey"`
	JobID                      string    `gorm:"uniqueIndex;size:64"`
	Incident, DeviceCode       string    `gorm:"index;size:64"`
	TenantID                   uint      `gorm:"index"`
	CreatedAt                  time.Time `gorm:"index"`
	Provider, Model, RequestID string
	CostMicros, Tokens         int64
	Status                     string
	Request, Response          []byte
	HTTPStatus                 int
	ErrorMessage               string
}

func (attempt) TableName() string { return "semantic_review_attempts" }

type Service struct {
	db           *gorm.DB
	limits       Limits
	scope        ScopeResolver
	policy       PolicyResolver
	fallback     func(Candidate) Submission
	result       func(Result)
	queue        []Candidate
	queueReady   chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	mu           sync.Mutex // only in-memory snapshots; never held during IO
	reservations sync.Mutex // serializes database admission and budget transactions
	queueBytes   int64
	started      bool
	owner        string
	recent       []Result // bounded metadata for requests rejected before durable admission
}

func NewService(db *gorm.DB, limits Limits, scope ScopeResolver, policy PolicyResolver, result func(Result)) (*Service, error) {
	if db == nil {
		return nil, errors.New("semantic review database unavailable")
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	limits = limits.normalized()
	if err := db.AutoMigrate(&job{}, &attempt{}, &recordEvidence{}); err != nil {
		return nil, err
	}
	// Upgrade earlier durable jobs without releasing already reserved calls.
	if err := db.Model(&job{}).Where("COALESCE(total_attempts,0) = 0 AND id IN (?)", db.Model(&attempt{}).Select("job_id")).Update("total_attempts", gorm.Expr("round")).Error; err != nil {
		return nil, err
	}
	if err := db.Model(&job{}).Where("COALESCE(total_attempts,0) = 0 AND round > 1").Update("total_attempts", gorm.Expr("round - 1")).Error; err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Service{db: db, limits: limits, scope: scope, policy: policy, result: result, queueReady: make(chan struct{}, 1), ctx: ctx, cancel: cancel, owner: fmt.Sprintf("%d", time.Now().UnixNano())}
	// An interrupted request might already have been billed. Retain its reserved
	// attempt, and never silently invoke the model again after process restart.
	now := time.Now().UTC()
	var interrupted []job
	if err := db.Where("status = ? AND lease_until < ?", "running", now).Find(&interrupted).Error; err != nil {
		return nil, err
	}
	for _, j := range interrupted {
		var c Candidate
		_ = json.Unmarshal(j.Payload, &c)
		r := NewResult(c, "result_unknown")
		r.AttemptsUsed = requestAttempts(j, c)
		r.ActualExecutorEpoch = candidateExecutionEpoch(c)
		r.ReasonCodes = []string{"executor_interrupted"}
		b, _ := json.Marshal(r)
		db.Model(&job{}).Where("id = ? AND status = ? AND lease_until < ?", j.ID, "running", now).Updates(map[string]any{"status": r.Status, "result": b, "payload": metadataPayload(c), "bytes": 0})
	}
	return s, nil
}
func (s *Service) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started {
		return
	}
	s.started = true
	s.wg.Add(1)
	go s.admitLoop()
	workers := s.limits.Concurrent
	if !s.limits.Enabled || workers == 0 {
		workers = 1
	} // Keep lease/queued-evidence cleanup alive while calls are disabled.
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}
}
func (s *Service) Stop() { s.cancel(); s.wg.Wait() }
func (s *Service) SubmitCandidate(c Candidate) Submission {
	if !s.limits.Enabled {
		return s.declined(c, "disabled", "semantic_review_disabled")
	}
	if s.ctx.Err() != nil {
		return s.declined(c, "failed", "service_stopped")
	}
	if err := ValidateCandidate(c, s.limits, time.Now()); err != nil {
		return s.declined(c, "invalid_evidence", err.Error())
	}
	// The hot path only copies an immutable snapshot into memory. Device
	// lookup, image validation, persistence and provider IO run on workers.
	n := int64(0)
	for _, f := range c.Evidence {
		n += int64(len(f.Data))
	}
	s.mu.Lock()
	if s.limits.MaxQueueBytes > 0 && s.queueBytes+n > s.limits.MaxQueueBytes {
		s.mu.Unlock()
		return s.declined(c, "queue_full", "queue_byte_limit")
	}
	if s.limits.QueueSize > 0 && len(s.queue) >= s.limits.QueueSize {
		s.mu.Unlock()
		return s.declined(c, "queue_full", "queue_count_limit")
	}
	c.Scores = append([]Score(nil), c.Scores...)
	if c.CooldownSeconds != nil {
		seconds := *c.CooldownSeconds
		c.CooldownSeconds = &seconds
	}
	c.TargetBoxes = append([][4]float32(nil), c.TargetBoxes...)
	c.Evidence = append([]Frame(nil), c.Evidence...)
	for i := range c.Evidence {
		c.Evidence[i].Data = append([]byte(nil), c.Evidence[i].Data...)
		c.Evidence[i].ROI = append([][2]float32(nil), c.Evidence[i].ROI...)
	}
	s.queue = append(s.queue, c)
	s.queueBytes += n
	s.mu.Unlock()
	select {
	case s.queueReady <- struct{}{}:
	default:
	}
	return Submission{c.ID, "pending", "awaiting_persistence"}
}
func (s *Service) declined(c Candidate, status, reason string) Submission {
	// Invalid oversized identities are never cached. No image bytes are retained.
	valid := len(c.Scores) <= 32
	for _, score := range c.Scores {
		if len(score.Type) > 64 || len(score.Model) > 128 {
			valid = false
		}
	}
	for _, v := range []string{c.ID, c.DeviceCode, c.RuleID, c.RuleRevision, c.IncidentID, c.Scene, c.StreamSession, c.TargetID, c.TargetGeneration, c.ExecutorEpoch, c.LocalResult, c.GatewayCode, c.InferenceSite} {
		if len(v) > 256 {
			valid = false
		}
	}
	if valid {
		r := NewResult(c, status)
		r.ReasonCodes = []string{reason}
		r.Scores = append([]Score(nil), c.Scores...)
		s.mu.Lock()
		s.recent = append(s.recent, r)
		if len(s.recent) > 200 {
			s.recent = append([]Result(nil), s.recent[len(s.recent)-200:]...)
		}
		s.mu.Unlock()
	}
	return Submission{ID: c.ID, Status: status, Reason: reason}
}
func ValidateCandidate(c Candidate, l Limits, now time.Time) error {
	if err := l.Validate(); err != nil {
		return err
	}
	l = l.normalized()
	for _, v := range []string{c.ID, c.IncidentID, c.DeviceCode, c.RuleID, c.RuleRevision, c.TargetID, c.TargetGeneration, c.StreamSession, c.ExecutorEpoch} {
		if strings.TrimSpace(v) == "" || len(v) > 256 {
			return errors.New("missing_or_oversized_identity")
		}
	}
	if c.Execution != "platform" && c.Execution != "local_first" {
		return errors.New("unsupported_executor")
	}
	if c.Round < 1 || c.Round > 2 {
		return errors.New("round_limit")
	}
	if c.AttemptsUsed < 0 || c.AttemptsUsed > 2 || c.Round-1+c.AttemptsUsed > 2 || len(c.LocalExecutorEpoch) > 256 || len(c.PlatformExecutorEpoch) > 256 || len(c.FallbackReason) > 64 {
		return errors.New("attempt_or_route_limit")
	}
	if c.LocalResult != "uncertain" && c.LocalResult != "confirmed" && c.LocalResult != "unknown" {
		return errors.New("invalid_local_result")
	}
	if c.EnterConfirmSeconds < 0 || c.EnterConfirmSeconds > 86400 {
		return errors.New("invalid_confirmation_time")
	}
	if c.CooldownSeconds != nil && (*c.CooldownSeconds < 0 || (*c.CooldownSeconds > 0 && *c.CooldownSeconds < 30)) {
		return errors.New("invalid_review_cooldown")
	}
	maxAge := 5 * time.Minute
	maxFrames := 8
	switch c.Scene {
	case "smoking":
		maxFrames = 4
	case "sleep_on_duty":
		maxFrames = 6
	case "person_fall", "fighting":
		maxAge = 2 * time.Minute
	default:
		return errors.New("unsupported_scene")
	}
	if c.ObservedAt.IsZero() || c.ObservedAt.After(now.Add(5*time.Second)) || !c.ExpiresAt.After(now) || c.ExpiresAt.After(c.ObservedAt.Add(maxAge)) {
		return errors.New("expired_or_invalid_deadline")
	}
	if len(c.Evidence) == 0 || len(c.Evidence) > maxFrames || len(c.Scores) > 32 {
		return errors.New("image_or_score_count_limit")
	}
	if c.TargetCoordScale != 10000 || len(c.TargetBoxes) > 8 || (c.Scene != "smoking" && len(c.TargetBoxes) == 0) {
		return errors.New("invalid_target_boxes")
	}
	for _, box := range c.TargetBoxes {
		for _, v := range box {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < 0 || v > 10000 {
				return errors.New("invalid_target_box")
			}
		}
		if box[2] <= 0 || box[3] <= 0 || box[0]+box[2] > 10000 || box[1]+box[3] > 10000 {
			return errors.New("invalid_target_box")
		}
	}
	total := 0
	ids := map[string]bool{}
	for _, f := range c.Evidence {
		if f.ID == "" || len(f.ID) > 128 || ids[f.ID] || f.StreamSession != c.StreamSession || f.CapturedAt.IsZero() || f.CapturedAt.After(now.Add(5*time.Second)) || f.CapturedAt.After(c.ObservedAt.Add(5*time.Second)) {
			return errors.New("invalid_frame_identity")
		}
		ids[f.ID] = true
		if f.Width < 1 || f.Height < 1 || f.Width > 16384 || f.Height > 16384 || f.Crop[0] < 0 || f.Crop[1] < 0 || f.Crop[2] <= f.Crop[0] || f.Crop[3] <= f.Crop[1] || f.Crop[2] > f.Width || f.Crop[3] > f.Height {
			return errors.New("invalid_frame_geometry")
		}
		if len(f.Data) == 0 || (l.MaxFrameBytes > 0 && len(f.Data) > l.MaxFrameBytes) || len(f.SHA256) != 64 || (f.MIME != "image/jpeg" && f.MIME != "image/png") {
			return errors.New("invalid_frame_bytes")
		}
		total += len(f.Data)
		maxEvidenceAge := 2 * time.Minute
		if c.Scene == "sleep_on_duty" {
			maxEvidenceAge = max(maxEvidenceAge, c.SleepConfirmationDuration()+5*time.Second)
		}
		if len(f.ROI) > 64 || f.CapturedAt.Before(c.ObservedAt.Add(-maxEvidenceAge)) {
			return errors.New("invalid_frame_roi_or_age")
		}
		for _, point := range f.ROI {
			for _, v := range point {
				if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) || v < 0 || v > 10000 {
					return errors.New("invalid_roi_coordinate")
				}
			}
		}
	}
	if l.MaxEvidenceBytes > 0 && total > l.MaxEvidenceBytes {
		return errors.New("evidence_byte_limit")
	}
	for _, v := range c.Scores {
		if len(v.Type) > 64 || len(v.Model) > 128 || v.Type == "" || math.IsNaN(float64(v.Value)) || math.IsInf(float64(v.Value), 0) || v.Value < 0 || v.Value > 1 {
			return errors.New("invalid_typed_score")
		}
	}
	return nil
}
func validateImages(c Candidate) error {
	seen := map[string]bool{}
	for _, f := range c.Evidence {
		h := sha256.Sum256(f.Data)
		if hex.EncodeToString(h[:]) != strings.ToLower(f.SHA256) {
			return errors.New("frame_hash_mismatch")
		}
		key := f.CapturedAt.UTC().Format(time.RFC3339Nano)
		if seen[key] {
			return errors.New("repeated_observation")
		}
		seen[key] = true
		cfg, kind, err := image.DecodeConfig(bytes.NewReader(f.Data))
		if err != nil || cfg.Width*cfg.Height > 16*1024*1024 || cfg.Width != f.Crop[2]-f.Crop[0] || cfg.Height != f.Crop[3]-f.Crop[1] || (f.MIME == "image/jpeg" && kind != "jpeg") || (f.MIME == "image/png" && kind != "png") {
			return errors.New("frame_format_or_size_mismatch")
		}
	}
	return nil
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func metadataPayload(c Candidate) []byte {
	c.Evidence = append([]Frame(nil), c.Evidence...)
	for i := range c.Evidence {
		c.Evidence[i].Data = nil
	}
	b, _ := json.Marshal(c)
	return b
}
func (s *Service) admitLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.queueReady:
			for s.ctx.Err() == nil {
				s.mu.Lock()
				if len(s.queue) == 0 {
					s.mu.Unlock()
					break
				}
				c := s.queue[0]
				s.queue[0] = Candidate{}
				s.queue = s.queue[1:]
				if len(s.queue) == 0 {
					s.queue = nil
				}
				for _, f := range c.Evidence {
					s.queueBytes -= int64(len(f.Data))
				}
				s.mu.Unlock()
				if err := s.admit(c); err != nil {
					s.emitFailure(c, "failed", "persistence_failed")
				}
			}
		}
	}
}
func (s *Service) emitFailure(c Candidate, status, reason string) {
	_ = s.declined(c, status, reason)
	r := NewResult(c, status)
	r.ReasonCodes = []string{reason}
	if s.result != nil {
		s.result(r)
	}
}
func (s *Service) admit(c Candidate) error {
	scope, err := s.scope(c.DeviceCode)
	if err != nil || scope.TenantID == 0 || scope.ProjectID == 0 {
		s.emitFailure(c, "failed", "invalid_device_scope")
		return nil
	}
	if c.GatewayCode != "" && c.GatewayCode != scope.GatewayCode {
		s.emitFailure(c, "failed", "gateway_scope_mismatch")
		return nil
	}
	c.GatewayCode = scope.GatewayCode
	if err := validateImages(c); err != nil {
		s.emitFailure(c, "invalid_evidence", err.Error())
		return nil
	}
	identity := []any{scope.TenantID, scope.ProjectID, scope.GatewayCode, c.DeviceCode, c.RuleID, c.RuleRevision, c.Scene, c.StreamSession, c.TargetID, c.TargetGeneration, c.IncidentID}
	incident := digest(identity)
	target := digest(identity[:10])
	frames := make([]string, len(c.Evidence))
	for i, f := range c.Evidence {
		frames[i] = f.SHA256 + f.CapturedAt.UTC().Format(time.RFC3339Nano)
	}
	dedupe := digest([]any{identity, c.ExecutorEpoch, frames})
	b, _ := json.Marshal(c)
	j := job{ID: digest([]any{scope.TenantID, scope.ProjectID, c.ID}), Dedupe: dedupe, Incident: incident, Target: target, DeviceCode: c.DeviceCode, TenantID: scope.TenantID, ProjectID: scope.ProjectID, Round: c.Round, TotalAttempts: c.Round - 1 + c.AttemptsUsed, CreatedAt: time.Now().UTC(), ExpiresAt: c.ExpiresAt, Status: "queued", Payload: b, Bytes: int64(len(b))}
	s.reservations.Lock()
	var emitted *Result
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing job
		if tx.Where("id = ? OR dedupe = ?", j.ID, j.Dedupe).First(&existing).Error == nil {
			if existing.ID != j.ID {
				r := NewResult(c, "merged")
				r.ReasonCodes = []string{"duplicate_evidence"}
				emitted = &r
				return nil
			}
			var old Result
			if json.Unmarshal(existing.Result, &old) == nil {
				emitted = &old
			}
			return nil
		}
		var used int
		tx.Model(&job{}).Where("incident = ?", incident).Select("COALESCE(MAX(total_attempts),0)").Scan(&used)
		j.TotalAttempts = max(j.TotalAttempts, used)
		status, reason := "", ""
		if j.TotalAttempts >= 2 {
			status, reason = "rate_limited", "event_round_limit"
		}
		var last job
		if tx.Where("target = ?", target).Order("created_at desc").First(&last).Error == nil {
			if last.Status == "queued" || last.Status == "running" || last.Status == "forwarding" {
				status, reason = "merged", "event_inflight"
			} else if cooldown := c.ReviewCooldownDuration(); cooldown > 0 && time.Since(last.CreatedAt) < cooldown {
				status, reason = "rate_limited", "target_cooldown"
			}
		}
		var pending int64
		tx.Model(&job{}).Where("status IN ?", []string{"queued", "running", "forwarding"}).Count(&pending)
		var total int64
		tx.Model(&job{}).Select("COALESCE(SUM(bytes),0)").Scan(&total)
		if (s.limits.QueueSize > 0 && pending >= int64(s.limits.QueueSize)) || (s.limits.MaxQueueBytes > 0 && total+j.Bytes > s.limits.MaxQueueBytes) {
			status, reason = "queue_full", "durable_queue_limit"
		}
		if status != "" {
			r := NewResult(c, status)
			r.AttemptsUsed = min(2-(c.Round-1), requestAttempts(j, c))
			r.ReasonCodes = []string{reason}
			j.Status = status
			j.Result, _ = json.Marshal(r)
			j.Payload = metadataPayload(c)
			j.Bytes = 0
		}
		if err := tx.Create(&j).Error; err != nil {
			return err
		}
		if j.Status == "queued" {
			if err := archiveRecordEvidence(tx, j.ID, j.CreatedAt, c.Evidence); err != nil {
				return err
			}
		}
		// Keep a bounded seven-day history. A retained attempt prevents recharging
		// the same event even after its heavy evidence is removed.
		tx.Where("created_at < ? AND status NOT IN ?", time.Now().Add(-7*24*time.Hour), []string{"queued", "running", "forwarding"}).Delete(&job{})
		tx.Where("created_at < ?", time.Now().Add(-7*24*time.Hour)).Delete(&attempt{})
		var totalJobs int64
		tx.Model(&job{}).Count(&totalJobs)
		if totalJobs > 1000 {
			var stale []job
			tx.Where("status NOT IN ?", []string{"queued", "running", "forwarding"}).Order("created_at").Limit(int(totalJobs - 1000)).Find(&stale)
			for _, old := range stale {
				tx.Delete(&job{}, "id = ?", old.ID)
			}
		}
		// Evidence follows the retained history, including its row-count cap.
		if err := tx.Where("job_id NOT IN (?)", tx.Model(&job{}).Select("id")).Delete(&recordEvidence{}).Error; err != nil {
			return err
		}
		if len(j.Result) > 0 {
			var r Result
			_ = json.Unmarshal(j.Result, &r)
			emitted = &r
		}
		return nil
	})
	s.reservations.Unlock()
	if err == nil && emitted != nil && s.result != nil {
		s.result(*emitted)
	}
	if err == nil && emitted != nil && emitted.Status == "merged" {
		_ = s.declined(c, "merged", "duplicate_evidence")
	}
	return err
}
func (s *Service) worker() {
	defer s.wg.Done()
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-tick.C:
			if s.limits.Concurrent == 0 && s.limits.Enabled {
				var queued int64
				s.db.Model(&job{}).Where("status = ?", "queued").Count(&queued)
				for i := int64(0); i < max(queued, 1) && s.ctx.Err() == nil; i++ {
					s.runNext(true)
				}
			} else {
				s.runOne()
			}
		}
	}
}
func (s *Service) runOne() {
	s.runNext(false)
}
func (s *Service) runNext(async bool) {
	s.replayForwarding()
	// Recover an interrupted owner's lease even when it was still live at startup.
	now := time.Now().UTC()
	var stale []job
	s.db.Where("status = ? AND lease_until < ?", "running", now).Limit(100).Find(&stale)
	for _, old := range stale {
		var c Candidate
		_ = json.Unmarshal(old.Payload, &c)
		r := NewResult(c, "result_unknown")
		r.AttemptsUsed = requestAttempts(old, c)
		r.ActualExecutorEpoch = candidateExecutionEpoch(c)
		r.ReasonCodes = []string{"executor_interrupted"}
		b, _ := json.Marshal(r)
		cas := s.db.Model(&job{}).Where("id = ? AND status = ? AND lease_until < ?", old.ID, "running", now).Updates(map[string]any{"status": r.Status, "result": b, "payload": metadataPayload(c), "bytes": 0})
		if cas.Error == nil && cas.RowsAffected == 1 && s.result != nil {
			s.result(r)
		}
	}
	if !s.limits.Enabled {
		var queued []job
		if s.db.Where("status = ?", "queued").Limit(100).Find(&queued).Error == nil {
			for _, old := range queued {
				var c Candidate
				if json.Unmarshal(old.Payload, &c) == nil {
					if s.forward(old, c, "local_disabled") {
						continue
					}
					s.finish(old, c, NewResult(c, "disabled"))
				}
			}
		}
		return
	}
	s.reservations.Lock()
	var j job
	err := s.db.Where("status = ?", "queued").Order("created_at").First(&j).Error
	if err != nil {
		s.reservations.Unlock()
		return
	}
	var c Candidate
	if json.Unmarshal(j.Payload, &c) != nil {
		s.reservations.Unlock()
		return
	}
	j.TotalAttempts = max(j.TotalAttempts, c.Round-1+c.AttemptsUsed)
	scope := Scope{TenantID: j.TenantID, ProjectID: j.ProjectID, GatewayCode: c.GatewayCode}
	p, err := s.policy(scope)
	if err != nil {
		s.reservations.Unlock()
		if s.forward(j, c, FallbackReason(err)) {
			return
		}
		r := NewResult(c, "no_provider")
		r.ReasonCodes = []string{"scoped_visual_provider_not_ready"}
		s.finish(j, c, r)
		return
	}
	if !time.Now().Before(j.ExpiresAt) {
		s.reservations.Unlock()
		s.finish(j, c, NewResult(c, "expired"))
		return
	}
	if expected := candidateExecutionEpoch(c); expected != "" && expected != p.ExecutorEpoch {
		s.reservations.Unlock()
		r := NewResult(c, "result_unknown")
		r.ReasonCodes = []string{"executor_configuration_changed"}
		s.finish(j, c, r)
		return
	}
	if c.Execution == "platform" {
		c.PlatformExecutorEpoch = p.ExecutorEpoch
	} else {
		c.LocalExecutorEpoch = p.ExecutorEpoch
	}
	if p.Verifier == nil || p.WorstCostMicros < 0 || p.MaxInputTokens < 0 || p.MaxOutputTokens < 0 || p.TenantDailyTokens < 0 || p.TenantDailyCostMicros < 0 {
		s.reservations.Unlock()
		r := NewResult(c, "no_provider")
		r.ReasonCodes = []string{"capability_budget_or_epoch_not_configured"}
		s.finish(j, c, r)
		return
	}
	if p.Timeout <= 0 || p.Timeout > 25*time.Second {
		p.Timeout = 25 * time.Second
	}
	if validator, ok := p.Verifier.(interface{ ValidateCandidate(Candidate) error }); ok {
		if err := validator.ValidateCandidate(c); err != nil {
			s.reservations.Unlock()
			r := NewResult(c, "invalid_evidence")
			r.ReasonCodes = []string{"provider_input_limits"}
			s.finish(j, c, r)
			return
		}
	}
	current, scopeErr := s.scope(c.DeviceCode)
	if scopeErr != nil || current != scope {
		s.reservations.Unlock()
		r := NewResult(c, "failed")
		r.ReasonCodes = []string{"device_scope_changed"}
		s.finish(j, c, r)
		return
	}
	reservedCost := p.WorstCostMicros
	reservedTokens := int64(p.MaxInputTokens) + int64(p.MaxOutputTokens)
	unlimitedTokens := p.MaxInputTokens == 0 || p.MaxOutputTokens == 0
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Atomic reservation occurs before network IO and survives timeout/restart.
		now := time.Now().UTC()
		day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		var used int
		if err := tx.Model(&job{}).Where("incident = ?", j.Incident).Select("COALESCE(MAX(total_attempts),0)").Scan(&used).Error; err != nil {
			return err
		}
		j.TotalAttempts = max(j.TotalAttempts, used)
		if j.TotalAttempts >= 2 {
			return errors.New("event_round_limit")
		}
		checks := []struct {
			query string
			args  []any
			limit int64
		}{
			{"incident = ?", []any{j.Incident}, 2},
			{"device_code = ? AND created_at >= ?", []any{j.DeviceCode, now.Add(-time.Hour)}, 6},
			{"device_code = ? AND created_at >= ?", []any{j.DeviceCode, day}, 30},
			{"tenant_id = ? AND created_at >= ?", []any{j.TenantID, day}, 300},
			{"created_at >= ?", []any{now.Add(-time.Minute)}, 10},
		}
		for _, check := range checks {
			var n int64
			if err := tx.Model(&attempt{}).Where(check.query, check.args...).Count(&n).Error; err != nil {
				return err
			}
			if n >= check.limit {
				return errors.New("attempt_limit")
			}
		}
		var active int64
		if err := tx.Model(&job{}).Where("status = ? AND lease_until >= ?", "running", now).Count(&active).Error; err != nil {
			return err
		}
		if s.limits.Concurrent > 0 && active >= int64(s.limits.Concurrent) {
			return errors.New("concurrent_limit")
		}
		var remainingCost, remainingTokens int64
		for _, budget := range []struct {
			tenant       bool
			cost, tokens int64
		}{{false, s.limits.PlatformDailyCostMicros, s.limits.PlatformDailyTokens}, {true, minimumLimit(s.limits.TenantDailyCostMicros, p.TenantDailyCostMicros), minimumLimit(s.limits.TenantDailyTokens, p.TenantDailyTokens)}} {
			q := tx.Model(&attempt{}).Where("created_at >= ?", day)
			if budget.tenant {
				q = q.Where("tenant_id = ?", j.TenantID)
			}
			var totals struct {
				Cost   int64
				Tokens int64
			}
			if err := q.Select("COALESCE(SUM(cost_micros),0) AS cost, COALESCE(SUM(tokens),0) AS tokens").Scan(&totals).Error; err != nil {
				return err
			}
			if totals.Cost < 0 || totals.Tokens < 0 || (budget.cost > 0 && (totals.Cost >= budget.cost || p.WorstCostMicros > budget.cost-totals.Cost)) || (budget.tokens > 0 && (totals.Tokens >= budget.tokens || int64(p.MaxInputTokens)+int64(p.MaxOutputTokens) > budget.tokens-totals.Tokens)) {
				return errors.New("budget_limit")
			}
			if budget.cost > 0 {
				remainingCost = minimumLimit(remainingCost, budget.cost-totals.Cost)
			}
			if budget.tokens > 0 {
				remainingTokens = minimumLimit(remainingTokens, budget.tokens-totals.Tokens)
			}
		}
		// An unknown per-call ceiling occupies the remaining daily allowance.
		// This reservation is not a guessed provider price or request cap.
		if p.WorstCostMicros == 0 {
			reservedCost = remainingCost
		}
		if unlimitedTokens && remainingTokens > 0 {
			reservedTokens = remainingTokens
		}
		payload, _ := json.Marshal(c)
		cas := tx.Model(&job{}).Where("id = ? AND status = ?", j.ID, "queued").Updates(map[string]any{"status": "running", "owner": s.owner, "lease_until": now.Add(p.Timeout + 5*time.Second), "total_attempts": j.TotalAttempts + 1, "payload": payload})
		if cas.Error != nil {
			return cas.Error
		}
		if cas.RowsAffected != 1 {
			return errors.New("lease_lost")
		}
		return tx.Create(&attempt{JobID: j.ID, Incident: j.Incident, DeviceCode: j.DeviceCode, TenantID: j.TenantID, CreatedAt: now, Provider: p.Provider, Model: p.Model, CostMicros: reservedCost, Tokens: reservedTokens, Status: "reserved"}).Error
	})
	s.reservations.Unlock()
	if err != nil {
		if err.Error() == "lease_lost" {
			return
		}
		if err.Error() == "concurrent_limit" {
			return
		}
		r := NewResult(c, "rate_limited")
		r.AttemptsUsed = min(2-(c.Round-1), requestAttempts(j, c))
		r.ReasonCodes = []string{err.Error()}
		s.finish(j, c, r)
		return
	}
	j.TotalAttempts++
	if async {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.executeAttempt(j, c, scope, p)
		}()
	} else {
		s.executeAttempt(j, c, scope, p)
	}
}
func (s *Service) executeAttempt(j job, c Candidate, scope Scope, p Policy) {
	ctx, cancel := context.WithTimeout(s.ctx, p.Timeout)
	defer cancel()
	ctx = WithExchangeRecorder(ctx, func(e Exchange) error {
		updates := map[string]any{}
		if len(e.Request) > 0 {
			updates["request"] = []byte(e.Request)
		}
		if len(e.Response) > 0 {
			updates["response"] = []byte(e.Response)
		}
		if e.HTTPStatus > 0 {
			updates["http_status"] = e.HTTPStatus
		}
		if len(updates) == 0 {
			return nil
		}
		return s.db.Model(&attempt{}).Where("job_id = ?", j.ID).Updates(updates).Error
	})
	started := time.Now()
	r, verifyErr := p.Verifier.Verify(ctx, c)
	usageReported := r.UsageReported
	usageTokens := int64(r.InputTokens) + int64(r.OutputTokens)
	validUsage := r.InputTokens >= 0 && r.OutputTokens >= 0
	if verifyErr != nil {
		status := "failed"
		if errors.Is(verifyErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			status = "timeout"
		}
		r = NewResult(c, status)
		r.ReasonCodes = []string{"provider_request_failed"}
	} else if validationErr := ValidateResult(c, r); validationErr != nil {
		r = NewResult(c, "invalid_result")
		r.ReasonCodes = []string{validationErr.Error()}
	} else {
		r.Status = r.Verdict
		r.DecisionSource = "local_model"
		if r.Verdict == "confirmed" {
			r.DecisionSource = "llm_semantic"
			if c.LocalResult == "confirmed" {
				r.DecisionSource = "local_model_llm_verified"
			}
		} else if r.Verdict == "rejected" && c.LocalResult == "confirmed" {
			r.Status = "review_conflict"
		}
	}
	// A request may outlive a provider/configuration change. Its reservation
	// remains, but an answer from the old executor must never upgrade an alarm.
	currentScope, currentScopeErr := s.scope(c.DeviceCode)
	currentPolicy, currentPolicyErr := s.policy(scope)
	if currentScopeErr != nil || currentScope != scope || currentPolicyErr != nil || currentPolicy.ExecutorEpoch != p.ExecutorEpoch || currentPolicy.Provider != p.Provider || currentPolicy.Model != p.Model || currentPolicy.ConfigVersion != p.ConfigVersion || s.ctx.Err() != nil {
		r = NewResult(c, "result_unknown")
		r.ReasonCodes = []string{"executor_configuration_changed"}
	}
	r.Provider = p.Provider
	r.Model = p.Model
	r.ActualExecutorEpoch = p.ExecutorEpoch
	r.AttemptsUsed = requestAttempts(j, c)
	if c.Execution == "local_first" {
		r.Execution = "local"
	} else {
		r.Execution = "platform"
	}
	r.DurationMs = time.Since(started).Milliseconds()
	// Conservative worst-case reservation remains when usage/price is unknown.
	errorMessage := ""
	if verifyErr != nil {
		errorMessage = verifyErr.Error()
		if len(errorMessage) > 4096 {
			errorMessage = errorMessage[:4096]
		}
	}
	updates := map[string]any{"status": r.Status, "request_id": r.RequestID, "error_message": errorMessage}
	if verifyErr == nil && usageReported && validUsage && (p.MaxInputTokens == 0 || p.MaxOutputTokens == 0) {
		updates["tokens"] = usageTokens
	}
	s.db.Model(&attempt{}).Where("job_id = ?", j.ID).Updates(updates)
	if r.Status != "result_unknown" && verifyErr != nil && s.forward(j, c, FallbackReason(verifyErr)) {
		return
	}
	s.finish(j, c, r)
}
func ValidateResult(c Candidate, r Result) error {
	if r.Scene != c.Scene || (r.Verdict != "confirmed" && r.Verdict != "rejected" && r.Verdict != "insufficient_evidence") {
		return errors.New("invalid_verdict_or_scene")
	}
	if r.Visibility != "adequate" && r.Visibility != "partial" && r.Visibility != "poor" {
		return errors.New("invalid_visibility")
	}
	if strings.TrimSpace(r.SummaryZH) == "" || strings.TrimSpace(r.SummaryEN) == "" || len([]rune(r.SummaryZH)) > 240 || len([]rune(r.SummaryEN)) > 400 || len(r.PostureType) > 64 || len(r.ReasonCodes) > 8 {
		return errors.New("invalid_explanation")
	}
	ids := map[string]Frame{}
	for _, f := range c.Evidence {
		ids[f.ID] = f
	}
	seen := map[string]bool{}
	var times []time.Time
	if len(r.EvidenceFrameIDs) == 0 || len(r.EvidenceFrameIDs) > len(c.Evidence) {
		return errors.New("invalid_evidence_ids")
	}
	for _, id := range r.EvidenceFrameIDs {
		f, ok := ids[id]
		if !ok || seen[id] {
			return errors.New("unknown_or_repeated_evidence_id")
		}
		seen[id] = true
		times = append(times, f.CapturedAt)
	}
	for _, code := range r.ReasonCodes {
		if len(code) > 64 {
			return errors.New("invalid_reason_code")
		}
	}
	if r.Verdict == "confirmed" {
		if r.Visibility == "poor" {
			return errors.New("insufficient_visibility")
		}
		sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })
		span := times[len(times)-1].Sub(times[0])
		switch c.Scene {
		case "sleep_on_duty":
			if len(times) < 2 || span < c.SleepConfirmationDuration() {
				return errors.New("insufficient_sleep_time_coverage")
			}
		case "person_fall", "fighting":
			if len(times) < 2 || span < 500*time.Millisecond {
				return errors.New("insufficient_transition_coverage")
			}
		}
	}
	return nil
}
func (s *Service) finish(j job, c Candidate, r Result) {
	s.finishResult(j, c, r, true)
}
func (s *Service) finishResult(j job, c Candidate, r Result, publish bool) {
	// Only server-supplied identity is allowed to reach consumers.
	base := NewResult(c, r.Status)
	base.Verdict = r.Verdict
	base.SummaryZH = r.SummaryZH
	base.SummaryEN = r.SummaryEN
	base.EvidenceFrameIDs = r.EvidenceFrameIDs
	base.ReasonCodes = r.ReasonCodes
	base.Visibility = r.Visibility
	base.PostureType = r.PostureType
	base.Provider = r.Provider
	base.Model = r.Model
	base.RequestID = r.RequestID
	base.InputTokens = r.InputTokens
	base.OutputTokens = r.OutputTokens
	base.UsageReported = r.UsageReported
	base.DurationMs = r.DurationMs
	if r.ActualExecutorEpoch != "" {
		base.ActualExecutorEpoch = r.ActualExecutorEpoch
	}
	base.AttemptsUsed = max(requestAttempts(j, c), r.AttemptsUsed)
	if r.Execution == "local" || r.Execution == "platform" {
		base.Execution = r.Execution
	}
	if r.PlatformExecutorEpoch != "" {
		base.PlatformExecutorEpoch = r.PlatformExecutorEpoch
	}
	if r.DecisionSource != "" {
		base.DecisionSource = r.DecisionSource
	}
	if time.Now().After(c.ExpiresAt) {
		base.Status = "expired"
		base.Verdict = ""
		base.DecisionSource = "local_model"
	}
	b, _ := json.Marshal(base)
	cas := s.db.Model(&job{}).Where("id = ? AND (status IN ? OR (status = ? AND owner = ?))", j.ID, []string{"queued", "forwarding"}, "running", s.owner).Updates(map[string]any{"status": base.Status, "result": b, "payload": metadataPayload(c), "bytes": 0, "total_attempts": max(j.TotalAttempts, c.Round-1+base.AttemptsUsed)})
	if cas.Error == nil && cas.RowsAffected == 1 && publish && s.result != nil {
		s.result(base)
	}
}
func (s *Service) GetResults(device string) []Result {
	var jobs []job
	s.db.Where("device_code = ?", device).Order("created_at desc").Limit(50).Find(&jobs)
	s.mu.Lock()
	out := make([]Result, 0, len(jobs)+50)
	for i := len(s.recent) - 1; i >= 0 && len(out) < 50; i-- {
		if s.recent[i].DeviceCode == device {
			out = append(out, s.recent[i])
		}
	}
	s.mu.Unlock()
	seen := map[string]int{}
	for i, r := range out {
		seen[r.ID] = i
	}
	for _, j := range jobs {
		var r Result
		if len(j.Result) > 0 {
			if json.Unmarshal(j.Result, &r) != nil {
				continue
			}
		} else {
			var c Candidate
			if json.Unmarshal(j.Payload, &c) != nil {
				continue
			}
			r = NewResult(c, j.Status)
			r.CompletedAt = time.Time{}
		}
		if index, exists := seen[r.ID]; exists {
			if r.CompletedAt.After(out[index].CompletedAt) {
				out[index] = r
			}
		} else {
			out = append(out, r)
			seen[r.ID] = len(out) - 1
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ObservedAt.After(out[j].ObservedAt) })
	if len(out) > 50 {
		out = out[:50]
	}
	return out
}
func (s *Service) SemanticStatus(device string) Status {
	status := "ready"
	if !s.limits.Enabled {
		status = "disabled"
	}
	var pending int64
	s.db.Model(&job{}).Where("device_code = ? AND status IN ?", device, []string{"queued", "running", "forwarding"}).Count(&pending)
	return Status{Enabled: s.limits.Enabled, Status: status, Pending: int(pending), Execution: "local_first", Results: s.GetResults(device)}
}
