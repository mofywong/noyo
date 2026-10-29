package core

import (
	"fmt"
	"gorm.io/gorm"
	"noyo/core/store"
	"strings"
)

// Claim is a compare-and-swap on both the aggregate and the unassigned task.
// Candidate membership is frozen at activation, while account access is live.
func (s *WorkOrderService) ClaimWorkOrderTask(scope WorkOrderScope, orderID, taskID uint, expected int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 || expected < 1 {
		return nil, fmt.Errorf("actor and expected version are required")
	}
	var result *store.WorkOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, orderID)
		if err != nil {
			return err
		}
		if order.Version != expected {
			return ErrWorkOrderVersionConflict
		}
		if order.ClosedAt != nil {
			return fmt.Errorf("work order is closed")
		}
		var task store.WorkOrderTask
		if err := policyScope(tx, scope).Where("work_order_id = ? AND workflow_cycle = ? AND delivery_mode = ? AND status = ? AND assignee_user_id = 0", order.ID, order.WorkflowCycle, "claim", WorkOrderTaskPending).First(&task, taskID).Error; err != nil {
			return err
		}
		var candidate store.WorkOrderTaskCandidate
		if err := tx.Where("task_id = ? AND user_id = ?", task.ID, scope.ActorUserID).First(&candidate).Error; err != nil {
			return fmt.Errorf("actor is not a candidate")
		}
		users, err := s.loadGraphTaskParticipants(tx, order, []uint{scope.ActorUserID})
		if err != nil {
			return err
		}
		var node store.WorkOrderNodeInstance
		if err := policyScope(tx, scope).Where("work_order_id = ? AND workflow_cycle = ? AND status = ?", order.ID, order.WorkflowCycle, WorkOrderNodeInstanceActive).First(&node, task.WorkOrderNodeInstanceID).Error; err != nil {
			return err
		}
		if err := s.updateGraphWorkOrderState(tx, order, order.Status, order.WorkflowCycle, order.ClosedAt); err != nil {
			return err
		}
		name := strings.TrimSpace(users[0].DisplayName)
		if name == "" {
			name = users[0].Username
		}
		changed := tx.Model(&store.WorkOrderTask{}).Where("id = ? AND status = ? AND assignee_user_id = 0", task.ID, WorkOrderTaskPending).Updates(map[string]any{"assignee_user_id": scope.ActorUserID, "assignee_display_name": name})
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		if err := s.appendWorkOrderEvent(tx, order, "workflow.task.claimed", scope.ActorUserID, map[string]any{"task_id": task.ID, "node_id": task.NodeID}); err != nil {
			return err
		}
		result = order
		return nil
	})
	return result, err
}
