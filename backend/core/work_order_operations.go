package core

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"noyo/core/store"
	"strings"
	"time"
)

const WorkOrderOutboxDead = "dead"
const WorkOrderOutboxStopped = "stopped"
const workOrderOutboxMaxAttempts = 5

// A connector can disappear on its last lease. Settle that failure even when
// the connector never polls again, so the operations queue remains accurate.
func (s *WorkOrderService) TickWorkOrderOperations(ctx context.Context, now time.Time) error {
	if err := s.db.WithContext(ctx).Model(&store.WorkOrderOutboxEvent{}).
		Where("attempt_count >= ? AND (status = ? OR (status = ? AND locked_until <= ?))", workOrderOutboxMaxAttempts, WorkOrderOutboxPending, WorkOrderOutboxDelivering, now.UTC()).
		Updates(map[string]any{"status": WorkOrderOutboxDead, "lock_token": "", "locked_at": nil, "locked_until": nil}).Error; err != nil {
		return err
	}
	return s.TickWorkOrderSLAs(ctx, now.UTC())
}

type WorkOrderOperationsSummary struct {
	PendingAssignment  int64      `json:"pending_assignment"`
	Claimable          int64      `json:"claimable"`
	SLABreached        int64      `json:"sla_breached"`
	FailedDelivery     int64      `json:"failed_delivery"`
	OldestAssignmentAt *time.Time `json:"oldest_assignment_at"`
}

func (s *WorkOrderService) GetWorkOrderOperationsSummary(scope WorkOrderScope) (WorkOrderOperationsSummary, error) {
	result := WorkOrderOperationsSummary{}
	if err := validateWorkOrderScope(scope); err != nil {
		return result, err
	}
	blocked := func() *gorm.DB {
		return policyScope(s.db.Model(&store.WorkOrderNodeInstance{}), scope).Where("status = ? AND EXISTS (SELECT 1 FROM work_orders WHERE work_orders.id = work_order_node_instances.work_order_id AND work_orders.workflow_cycle = work_order_node_instances.workflow_cycle AND work_orders.closed_at IS NULL AND work_orders.deleted_at IS NULL)", WorkOrderNodeInstancePendingAssignment)
	}
	if err := blocked().Count(&result.PendingAssignment).Error; err != nil {
		return result, err
	}
	var oldest store.WorkOrderNodeInstance
	if result.PendingAssignment > 0 {
		if err := blocked().Order("created_at,id").First(&oldest).Error; err != nil {
			return result, err
		}
		result.OldestAssignmentAt = &oldest.CreatedAt
	}
	if err := policyScope(s.db.Model(&store.WorkOrderTask{}), scope).Where("delivery_mode = 'claim' AND assignee_user_id = 0 AND EXISTS (SELECT 1 FROM work_order_task_candidates c WHERE c.task_id = work_order_tasks.id AND c.user_id = ? AND c.deleted_at IS NULL) AND status = ? AND EXISTS (SELECT 1 FROM work_orders WHERE work_orders.id = work_order_tasks.work_order_id AND work_orders.workflow_cycle = work_order_tasks.workflow_cycle AND work_orders.closed_at IS NULL AND work_orders.deleted_at IS NULL)", scope.ActorUserID, WorkOrderTaskPending).Count(&result.Claimable).Error; err != nil {
		return result, err
	}
	if err := policyScope(s.db.Model(&store.WorkOrderSLAInstance{}), scope).Where("completed_at IS NULL AND (response_breached = ? OR resolution_breached = ?) AND EXISTS (SELECT 1 FROM work_orders WHERE work_orders.id = work_order_sla_instances.work_order_id AND work_orders.workflow_cycle = work_order_sla_instances.workflow_cycle AND work_orders.closed_at IS NULL AND work_orders.deleted_at IS NULL)", true, true).Count(&result.SLABreached).Error; err != nil {
		return result, err
	}
	err := policyScope(s.db.Model(&store.WorkOrderOutboxEvent{}), scope).Where("status = ?", WorkOrderOutboxDead).Count(&result.FailedDelivery).Error
	return result, err
}

// Dead letters retain the original event id. Consumers must deduplicate by it.
func (s *WorkOrderService) RecoverWorkOrderDelivery(scope WorkOrderScope, id uint, expectedAttempts int, stop bool, reason string) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	if scope.ActorUserID == 0 || reason == "" || len([]rune(reason)) > 2000 {
		return fmt.Errorf("actor and recovery reason are required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var event store.WorkOrderOutboxEvent
		if err := policyScope(tx, scope).First(&event, id).Error; err != nil {
			return err
		}
		if event.AttemptCount != expectedAttempts || (event.Status != WorkOrderOutboxDead && event.Status != WorkOrderOutboxStopped) {
			return ErrWorkOrderVersionConflict
		}
		status := WorkOrderOutboxPending
		attempts := 0
		if stop {
			status = WorkOrderOutboxStopped
			attempts = event.AttemptCount
		}
		changed := policyScope(tx.Model(&store.WorkOrderOutboxEvent{}), scope).Where("id = ? AND status = ? AND attempt_count = ?", id, event.Status, expectedAttempts).Updates(map[string]any{"status": status, "attempt_count": attempts, "available_at": time.Now().UTC(), "lock_token": "", "locked_at": nil, "locked_until": nil})
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		order, err := s.findWorkOrder(tx, scope, event.WorkOrderID)
		if err != nil {
			return err
		}
		return s.appendWorkOrderEvent(tx, order, "delivery.recovered", scope.ActorUserID, map[string]any{"outbox_id": id, "event_id": event.EventID, "previous_attempts": expectedAttempts, "action": status, "reason": reason})
	})
}

func (s *WorkOrderService) ListFailedWorkOrderDeliveries(scope WorkOrderScope) ([]store.WorkOrderOutboxEvent, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var rows []store.WorkOrderOutboxEvent
	err := policyScope(s.db, scope).Where("status IN ?", []string{WorkOrderOutboxDead, WorkOrderOutboxStopped}).Order("id DESC").Limit(100).Find(&rows).Error
	return rows, err
}
