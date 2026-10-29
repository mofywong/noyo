package inspection

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Recover never silently repeats a camera operation that may have completed
// before a crash. Operators can inspect the partial report and start a new run.
func (s *Service) Recover() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var runs []Run
		if err := tx.Where("mode = ? AND status = ?", "ai", "running").Find(&runs).Error; err != nil {
			return err
		}
		for _, r := range runs {
			if err := s.failRun(tx, r, "execution_interrupted"); err != nil {
				return err
			}
		}
		// Require a fresh playback confirmation after service restart.
		return tx.Model(&Item{}).Where("status = ?", "pending").Update("observed_at", nil).Error
	})
}

// Execute claims durably before contacting a provider. The caller bounds the
// number of active runs. A shared semaphore bounds gateway work across all runs.
func (s *Service) Execute(ctx context.Context, id string, provider Provider, slots chan struct{}) (resultErr error) {
	var r Run
	defer func() {
		if resultErr != nil && r.ID != "" {
			s.mu.Lock()
			defer s.mu.Unlock()
			err := s.DB.Transaction(func(tx *gorm.DB) error { return s.failRun(tx, r, "execution_failed") })
			resultErr = errors.Join(resultErr, err)
		}
	}()
	res := s.DB.Model(&Run{}).Where("id = ? AND mode = ? AND status = ?", id, "ai", "queued").Updates(map[string]any{"status": "running", "started_at": s.Now()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return ErrConflict
	}
	if err := s.DB.First(&r, "id = ?", id).Error; err != nil {
		return err
	}
	if r.QueueDeadlineAt != nil && !r.QueueDeadlineAt.After(s.Now()) {
		return errors.New("queue_deadline_exceeded")
	}
	if r.DeadlineAt != nil {
		remaining := r.DeadlineAt.Sub(s.Now())
		if remaining <= 0 {
			return errors.New("execution_deadline_exceeded")
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, remaining)
		defer cancel()
	}
	groups := GroupCameras(r.Spec().Cameras)
	var wg sync.WaitGroup
	var errsMu sync.Mutex
	var failures []error
	for _, domain := range SortedDomains(groups) {
		cameras := groups[domain]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.executeDomain(ctx, r, domain, cameras, provider, slots); err != nil {
				errsMu.Lock()
				failures = append(failures, err)
				errsMu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		return tx.Model(&Run{}).Where("id = ? AND status = ?", id, "running").Updates(map[string]any{"status": completionStatus(tx, id), "finished_at": s.Now(), "version": gorm.Expr("version + 1")}).Error
	})
}
func (s *Service) executeDomain(ctx context.Context, r Run, domain string, cameras []Camera, provider Provider, slots chan struct{}) error {
	codes := make([]string, 0, len(cameras))
	for _, camera := range cameras {
		codes = append(codes, camera.Code)
	}
	if err := s.progress(r, codes, "waiting"); err != nil {
		return err
	}
	value, _ := s.domains.LoadOrStore(domain, make(chan struct{}, 1))
	lock := value.(chan struct{})
	select {
	case lock <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-lock }()
	select {
	case slots <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-slots }()
	if err := s.progress(r, codes, "preparing"); err != nil {
		return err
	}
	var batch Batch
	var openErr error
	if provider == nil {
		openErr = errors.New("provider_unavailable")
	} else {
		batch, openErr = provider.Open(ctx, domain, cameras)
	}
	if batch != nil {
		defer batch.Close()
	}
	for _, camera := range cameras {
		analysis := Analysis{Result: "unable", Detail: "provider_unavailable"}
		if openErr != nil {
			analysis.Detail = openErr.Error()
		} else if batch == nil {
			analysis.Detail = "provider_unavailable"
		} else {
			itemCtx, cancel := context.WithTimeout(ctx, CameraTimeout(camera))
			itemCtx = WithObservationProgress(itemCtx, func(progress ObservationProgress) error {
				return s.observationProgress(r, camera.Code, progress)
			})
			var stageMu sync.Mutex
			lastStage := ""
			itemCtx = WithProgress(itemCtx, func(stage string) error {
				stageMu.Lock()
				defer stageMu.Unlock()
				if lastStage == stage {
					return nil
				}
				if err := s.progress(r, []string{camera.Code}, stage); err != nil {
					return err
				}
				lastStage = stage
				return nil
			})
			if err := ReportProgress(itemCtx, "sampling"); err != nil {
				cancel()
				return err
			}
			var err error
			analysis, err = batch.Analyze(itemCtx, camera)
			cancel()
			if err != nil {
				analysis.Result = "unable"
				analysis.Detail = err.Error()
				analysis.Issues = nil
			}
		}
		if analysis.Result != "pass" && analysis.Result != "issue" {
			analysis.Result = "unable"
			analysis.Issues = nil
		}
		if analysis.Result == "issue" && len(analysis.Issues) == 0 {
			analysis.Result = "unable"
			analysis.Detail = "invalid_provider_result"
		}
		if analysis.Result != "unable" && (analysis.CapturedAt.IsZero() || s.Now().Sub(analysis.CapturedAt) > time.Minute || analysis.CapturedAt.After(s.Now().Add(5*time.Second))) {
			analysis.Result = "unable"
			analysis.Detail = "stale_or_missing_frame"
			analysis.Issues = nil
		}
		if err := s.progress(r, []string{camera.Code}, "saving"); err != nil {
			return err
		}
		if err := s.saveAnalysis(r, camera, analysis); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) saveAnalysis(r Run, c Camera, a Analysis) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var current Run
		if err := tx.First(&current, "id = ?", r.ID).Error; err != nil {
			return err
		}
		if current.Status != "running" && current.Status != "queued" {
			return ErrConflict
		}
		var i Item
		if err := tx.First(&i, "run_id = ? AND camera_code = ?", r.ID, c.Code).Error; err != nil {
			return err
		}
		now := s.Now()
		i.Status = "done"
		i.Result = a.Result
		finishProgress(&i, now)
		i.Detail = a.Detail
		i.IssuesJSON = encoded(a.Issues)
		i.CheckedAt = &now
		i.ModelVersion = a.ModelVersion
		i.EvidenceJSON = encoded(a.Evidence)
		i.Coverage = a.Coverage
		if i.Coverage == "" {
			if i.Result == "unable" {
				i.Coverage = "none"
			} else {
				i.Coverage = "complete"
			}
		}
		i.SceneResultsJSON = encoded(a.SceneResults)
		if !a.CapturedAt.IsZero() {
			i.CapturedAt = &a.CapturedAt
		}
		if i.Result == "issue" {
			i.ReviewStatus = "pending"
			if r.Spec().ReviewPolicy == "automatic" {
				i.ReviewStatus = "confirmed"
				i.ReviewedIssuesJSON = i.IssuesJSON
				i.ReviewVersion = 1
				i.ReviewedBy = Actor{Name: "规则自动确认 / Automatic rule confirmation", Kind: "service"}
				i.ReviewedAt = &now
				if err := tx.Create(&ReviewEvent{ID: uuid.NewString(), RunID: r.ID, ItemID: i.ID, Version: 1, Status: "confirmed", IssuesJSON: i.IssuesJSON, Note: "automatic_rule_policy", Actor: i.ReviewedBy, CreatedAt: now}).Error; err != nil {
					return err
				}
			}
		}
		if err := s.episode(tx, r, &i); err != nil {
			return err
		}
		if i.ReviewStatus != "pending" {
			// 自动建单只由计划配置决定：识别端不再替计划做「是否需要工单」的判断。
			if r.Spec().AutoOrder && i.Result == "issue" {
				if err := s.queueAutoOrder(tx, r, i); err != nil {
					return err
				}
			}
		}
		if err := s.storeEvidence(tx, r, &i); err != nil {
			return err
		}
		if err := tx.Save(&i).Error; err != nil {
			return err
		}
		return tx.Model(&Run{}).Where("id = ?", r.ID).Update("version", gorm.Expr("version + 1")).Error
	})
}
