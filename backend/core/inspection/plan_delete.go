package inspection

import (
	"errors"

	"gorm.io/gorm"
)

// ErrPlanInProgress rejects deletion while an inspection belonging to the plan
// is still queued, running or paused. Revoking the plan authority would abort a
// run that is still in flight, and a paused run could never be resumed once its
// plan is gone.
var ErrPlanInProgress = errors.New("inspection_plan_in_progress")

// activeRunStatuses are the states in which a run still depends on its plan.
var activeRunStatuses = []string{"queued", "running", "paused"}

// DeletePlan removes a plan together with its execution authority.
// Runs keep their own snapshot and actor, so finished inspection history,
// reports and evidence stay readable after the plan is gone.
func (s *Service) DeletePlan(scope Scope, actor Actor, id string, revision uint) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 || actor.UserID == 0 || id == "" || revision == 0 {
		return ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.Plan(scope, id)
	if err != nil {
		return err
	}
	if plan.Revision != revision {
		return ErrConflict
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var active int64
		if err := scoped(tx.Model(&Run{}), scope).Where("plan_id = ? AND status IN ?", id, activeRunStatuses).Count(&active).Error; err != nil {
			return err
		}
		if active > 0 {
			return ErrPlanInProgress
		}
		// Keep the grant rows as revoked records: the audit trail must show that
		// this plan once held execution authority and who withdrew it.
		if err := scoped(tx.Model(&ExecutionGrant{}), scope).Where("plan_id = ? AND revoked_at IS NULL", id).Updates(map[string]any{"revoked_at": s.Now(), "revoked_by": actor.UserID}).Error; err != nil {
			return err
		}
		result := scoped(tx, scope).Where("id = ? AND revision = ?", id, revision).Delete(&Plan{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrConflict
		}
		return nil
	})
}
