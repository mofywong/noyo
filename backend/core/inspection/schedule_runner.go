package inspection

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type ScheduleOccurrence struct {
	Slot      string `gorm:"primaryKey;size:150" json:"slot"`
	Scope     `gorm:"embedded"`
	PlanID    string    `gorm:"index" json:"plan_id"`
	Revision  uint      `json:"revision"`
	DueAt     time.Time `json:"due_at"`
	RunID     string    `json:"run_id,omitempty"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

func (ScheduleOccurrence) TableName() string { return "inspection_schedule_occurrences" }
func runBudget(spec Spec) time.Duration {
	// A conservative bound also works when all gateway permits are contended.
	budget := time.Minute
	for _, camera := range spec.Cameras {
		budget += CameraTimeout(camera)
	}
	return budget
}

func (s *Service) migrateDeadlines() error {
	var runs []Run
	if err := s.DB.Where("mode = ? AND status IN ? AND deadline_at IS NULL", "ai", []string{"queued", "running"}).Find(&runs).Error; err != nil {
		return err
	}
	for _, r := range runs {
		base := r.CreatedAt
		if base.IsZero() {
			base = r.StartedAt
		}
		if base.IsZero() {
			base = s.Now()
		}
		q := base.Add(5 * time.Minute)
		d := q.Add(runBudget(r.Spec()))
		if err := s.DB.Model(&Run{}).Where("id = ? AND deadline_at IS NULL", r.ID).Updates(map[string]any{"queue_deadline_at": q, "deadline_at": d}).Error; err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) Schedule() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Now().UTC()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.expireRuns(tx, now); err != nil {
			return err
		}
		var plans []Plan
		if err := tx.Where("mode = ? AND enabled = ? AND next_due_at <= ?", "ai", true, now).Order("next_due_at,id").Limit(100).Find(&plans).Error; err != nil {
			return err
		}
		for _, p := range plans {
			slot := fmt.Sprintf("%s:%d:%d", p.ID, p.Revision, p.NextDueAt.Unix())
			var count int64
			if err := tx.Model(&ScheduleOccurrence{}).Where("slot = ?", slot).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				o := ScheduleOccurrence{Slot: slot, Scope: p.Scope, PlanID: p.ID, Revision: p.Revision, DueAt: *p.NextDueAt, Status: "skipped_overlap", CreatedAt: now}
				var previous Run
				previousErr := tx.First(&previous, "slot = ?", slot).Error
				if previousErr != nil && !errors.Is(previousErr, gorm.ErrRecordNotFound) {
					return previousErr
				}
				if err := tx.Model(&Run{}).Where("plan_id = ? AND status IN ?", p.ID, []string{"queued", "running"}).Count(&count).Error; err != nil {
					return err
				}
				if previousErr == nil {
					o.RunID = previous.ID
					o.Status = previous.Status
				} else if count == 0 {
					r, err := s.startPlanIn(tx, p, Actor{Name: "巡检服务 / Inspection service", Kind: "service"}, slot)
					if err != nil {
						return err
					}
					o.RunID = r.ID
					o.Status = "queued"
				}
				if err := tx.Create(&o).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&Plan{}).Where("id = ? AND revision = ?", p.ID, p.Revision).Update("next_due_at", nextDue(p.Spec(), now)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Service) expireRuns(tx *gorm.DB, now time.Time) error {
	var runs []Run
	if err := tx.Where("mode = ? AND ((status = ? AND queue_deadline_at <= ?) OR (status = ? AND deadline_at <= ?))", "ai", "queued", now, "running", now).Find(&runs).Error; err != nil {
		return err
	}
	for _, r := range runs {
		reason := "execution_deadline_exceeded"
		if r.Status == "queued" {
			reason = "queue_deadline_exceeded"
		}
		if err := s.failRun(tx, r, reason); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) failRun(tx *gorm.DB, r Run, reason string) error {
	var items []Item
	if err := tx.Where("run_id = ? AND status <> ?", r.ID, "done").Find(&items).Error; err != nil {
		return err
	}
	now := s.Now()
	for _, i := range items {
		i.Status = "done"
		i.Result = "unable"
		finishProgress(&i, now)
		i.Coverage = "none"
		i.Detail = reason
		i.CheckedAt = &now
		if err := s.episode(tx, r, &i); err != nil {
			return err
		}
		if err := tx.Save(&i).Error; err != nil {
			return err
		}
	}
	return tx.Model(&Run{}).Where("id = ? AND status IN ?", r.ID, []string{"queued", "running"}).Updates(map[string]any{"status": "partial", "detail": reason, "finished_at": now, "version": gorm.Expr("version + 1")}).Error
}
