package inspection

import (
	"encoding/json"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

type OrderIntentRecovery struct {
	ID               uint `gorm:"primaryKey" json:"id"`
	Scope            `gorm:"embedded"`
	IntentID         string    `gorm:"index" json:"intent_id"`
	ActorID          uint      `json:"actor_id"`
	PreviousGrantID  uint      `json:"previous_grant_id"`
	GrantID          uint      `json:"grant_id"`
	PreviousAttempts int       `json:"previous_attempts"`
	Reason           string    `gorm:"type:text" json:"reason"`
	CreatedAt        time.Time `json:"created_at"`
}

func (s *Service) RetryFailedOrder(scope Scope, intentID string, actor Actor, expectedAttempts int, grantID uint, reason string) (OrderIntent, error) {
	var intent OrderIntent
	reason = strings.TrimSpace(reason)
	if actor.UserID == 0 || reason == "" || len([]rune(reason)) > 2000 {
		return intent, ErrInvalid
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := scoped(tx, scope).First(&intent, "id = ?", intentID).Error; err != nil {
			return err
		}
		if intent.Status != "failed" || intent.OrderID != 0 || intent.Attempts != expectedAttempts {
			return ErrConflict
		}
		var item Item
		var run Run
		if err := scoped(tx, scope).First(&run, "id = ?", intent.RunID).Error; err != nil {
			return err
		}
		if err := tx.First(&item, "id = ? AND run_id = ?", intent.ItemID, intent.RunID).Error; err != nil {
			return err
		}
		if item.Result != "issue" || (run.Mode == "ai" && item.ReviewStatus != "confirmed") {
			return ErrConflict
		}
		var finding Finding
		if err := tx.Where("item_id = ? AND code = ?", intent.ItemID, intent.IssueCode).First(&finding).Error; err != nil {
			return err
		}
		effectiveGrant := grantID
		if effectiveGrant == 0 {
			effectiveGrant = intent.ExecutionGrantID
		}
		recovery := OrderIntentRecovery{Scope: scope, IntentID: intent.ID, ActorID: actor.UserID, PreviousGrantID: intent.ExecutionGrantID, GrantID: effectiveGrant, PreviousAttempts: intent.Attempts, Reason: reason, CreatedAt: s.Now()}
		updates := map[string]any{"status": "pending", "attempts": 0, "error": "", "next_attempt_at": s.Now(), "lease_token": "", "lease_until": nil, "recovered_by": actor.UserID, "recovery_reason": reason}
		if grantID != 0 && grantID != intent.ExecutionGrantID {
			if intent.PolicyVersionID == 0 {
				return ErrInvalid
			}
			updates["execution_grant_id"] = grantID
			if intent.CommandJSON != "" {
				var command map[string]any
				if err := json.Unmarshal([]byte(intent.CommandJSON), &command); err != nil {
					return err
				}
				command["execution_grant_id"] = grantID
				if target, ok := command["scope"].(map[string]any); ok {
					target["principal"] = map[string]any{"type": "automation", "ref": strconv.FormatUint(uint64(grantID), 10), "permissions": map[string]bool{"work_order:create": true}}
				}
				if snapshot, ok := command["source_snapshot"].(map[string]any); ok {
					snapshot["execution_grant_id"] = grantID
				}
				updates["command_json"] = encoded(command)
			}
		}
		changed := scoped(tx.Model(&OrderIntent{}), scope).Where("id = ? AND status = ? AND attempts = ? AND order_id = 0", intentID, "failed", expectedAttempts).Updates(updates)
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrConflict
		}
		if err := tx.Create(&recovery).Error; err != nil {
			return err
		}
		if err := touchEpisodeRuns(tx, []string{intent.EpisodeID}); err != nil {
			return err
		}
		return scoped(tx, scope).First(&intent, "id = ?", intentID).Error
	})
	return intent, err
}
