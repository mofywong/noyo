package inspection

import (
	"encoding/json"

	"gorm.io/gorm"
)

// SetPlanEnabled changes only scheduling state. In particular, legacy plans
// need not pass today's complete editor validation just to stop a schedule.
func (s *Service) SetPlanEnabled(scope Scope, actor Actor, id string, revision uint, enabled bool) (Plan, error) {
	if scope.TenantID == 0 || scope.ProjectID == 0 || actor.UserID == 0 || id == "" || revision == 0 {
		return Plan{}, ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	plan, err := s.Plan(scope, id)
	if err != nil {
		return Plan{}, err
	}
	if plan.Revision != revision {
		return Plan{}, ErrConflict
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(plan.SpecJSON), &raw); err != nil || raw == nil {
		return Plan{}, ErrInvalid
	}
	raw["enabled"] = json.RawMessage("false")
	if enabled {
		raw["enabled"] = json.RawMessage("true")
	}
	encodedSpec, err := json.Marshal(raw)
	if err != nil {
		return Plan{}, err
	}
	plan.SpecJSON, plan.Enabled = string(encodedSpec), enabled
	plan.NextDueAt = nextDue(plan.Spec(), s.Now())
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if enabled {
			if err := s.issuePlanGrant(tx, &plan, actor); err != nil {
				return err
			}
		}
		result := scoped(tx.Model(&Plan{}), scope).Where("id = ? AND revision = ?", id, revision).Updates(map[string]any{
			"enabled": enabled, "spec_json": plan.SpecJSON, "next_due_at": plan.NextDueAt,
			"revision": gorm.Expr("revision + 1"), "updated_at": s.Now(), "execution_grant_id": plan.ExecutionGrantID,
		})
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
