package inspection

import (
	"context"
	"time"

	"gorm.io/gorm"
)

type progressContextKey struct{}
type observationProgressContextKey struct{}

// ObservationProgress exposes real frame processing, independently of the
// camera lifecycle stage. Counts never imply a business conclusion.
type ObservationProgress struct {
	SampleCount int             `json:"sample_count"`
	Scenes      []SceneProgress `json:"scenes"`
}
type SceneProgress struct {
	Code             string `json:"code"`
	Stage            string `json:"stage"`
	Result           string `json:"result,omitempty"`
	SampleCount      int    `json:"sample_count"`
	ValidSampleCount int    `json:"valid_sample_count"`
	MinFrames        int    `json:"min_frames"`
	WindowSeconds    int    `json:"window_seconds"`
	ConfirmSeconds   int    `json:"confirm_seconds,omitempty"`
	WarmupSeconds    int    `json:"warmup_seconds,omitempty"`
	ObservedSeconds  int    `json:"observed_seconds"`
	Reason           string `json:"reason,omitempty"`
}

func WithObservationProgress(ctx context.Context, update func(ObservationProgress) error) context.Context {
	return context.WithValue(ctx, observationProgressContextKey{}, update)
}
func ReportObservationProgress(ctx context.Context, progress ObservationProgress) error {
	if update, ok := ctx.Value(observationProgressContextKey{}).(func(ObservationProgress) error); ok {
		return update(progress)
	}
	return nil
}

func (s *Service) observationProgress(run Run, code string, progress ObservationProgress) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var current Run
		if err := tx.First(&current, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if current.Status != "running" {
			return ErrConflict
		}
		value := encoded(progress)
		result := tx.Model(&Item{}).Where("run_id = ? AND camera_code = ? AND status = ? AND (progress_json IS NULL OR progress_json <> ?)", run.ID, code, "pending", value).Updates(map[string]any{"progress_json": value, "stage_updated_at": s.Now()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&Run{}).Where("id = ?", run.ID).UpdateColumn("version", gorm.Expr("version + 1")).Error
	})
}

// WithProgress preserves the provider interface while allowing local and remote
// providers to report actual sampling/inference boundaries.
func WithProgress(ctx context.Context, update func(string) error) context.Context {
	return context.WithValue(ctx, progressContextKey{}, update)
}

func ReportProgress(ctx context.Context, stage string) error {
	switch stage {
	case "waiting", "preparing", "sampling", "analyzing", "saving":
	default:
		return ErrInvalid
	}
	if update, ok := ctx.Value(progressContextKey{}).(func(string) error); ok {
		return update(stage)
	}
	return nil
}

func (s *Service) progress(run Run, codes []string, stage string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var current Run
		if err := tx.First(&current, "id = ?", run.ID).Error; err != nil {
			return err
		}
		if current.Status != "running" {
			return ErrConflict
		}
		var items []Item
		if err := tx.Where("run_id = ? AND camera_code IN ? AND status = ?", run.ID, codes, "pending").Find(&items).Error; err != nil {
			return err
		}
		changed := false
		for _, item := range items {
			if item.Stage == stage {
				continue
			}
			values := map[string]any{"stage": stage, "stage_started_at": now, "stage_updated_at": now}
			if err := tx.Model(&Item{}).Where("id = ? AND status = ?", item.ID, "pending").Updates(values).Error; err != nil {
				return err
			}
			changed = true
		}
		if changed {
			return tx.Model(&Run{}).Where("id = ?", run.ID).UpdateColumn("version", gorm.Expr("version + 1")).Error
		}
		return nil
	})
}

func finishProgress(item *Item, now time.Time) {
	item.Stage = "completed"
	if item.Result == "unable" {
		item.Stage = "failed"
	}
	item.StageStartedAt, item.StageUpdatedAt = &now, &now
}
