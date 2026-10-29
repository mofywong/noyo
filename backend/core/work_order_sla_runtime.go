package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"noyo/core/store"
)

// Each interval uses local calendar boundaries but elapsed durations use UTC,
// including DST changes. Seconds are preserved; nonworking days are skipped.
func workOrderBusinessIntervals(p WorkOrderSLAPolicy, start, end time.Time, visit func(time.Time, time.Time) bool) error {
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return err
	}
	cursor := start.In(loc)
	for days := 0; days < 100000; days++ {
		y, m, d := cursor.Date()
		dayStart := time.Date(y, m, d, 0, 0, 0, 0, loc)
		next := time.Date(y, m, d+1, 0, 0, 0, 0, loc)
		working := len(p.Weekdays) == 0
		for _, v := range p.Weekdays {
			if v == int(dayStart.Weekday()) {
				working = true
			}
		}
		if containsPolicyValue(p.Holidays, dayStart.Format("2006-01-02")) {
			working = false
		}
		if working {
			a, b := dayStart, next
			if len(p.Weekdays) > 0 {
				a = time.Date(y, m, d, p.StartHour, 0, 0, 0, loc)
				b = time.Date(y, m, d, p.EndHour, 0, 0, 0, loc)
			}
			if a.Before(cursor) {
				a = cursor
			}
			if !end.IsZero() && b.After(end) {
				b = end
			}
			if b.After(a) && !visit(a, b) {
				return nil
			}
		}
		if !end.IsZero() && !next.Before(end) {
			return nil
		}
		cursor = next
	}
	return fmt.Errorf("SLA calendar exceeds supported horizon")
}

func addWorkOrderBusinessTime(p WorkOrderSLAPolicy, start time.Time, duration time.Duration) (time.Time, error) {
	if duration == 0 {
		return start, nil
	}
	if duration < 0 {
		return time.Time{}, fmt.Errorf("negative business duration")
	}
	var result time.Time
	err := workOrderBusinessIntervals(p, start, time.Time{}, func(a, b time.Time) bool {
		available := b.Sub(a)
		if duration <= available {
			result = a.Add(duration)
			return false
		}
		duration -= available
		return true
	})
	return result, err
}

func (s *WorkOrderService) applyWorkOrderSLALifecycle(tx *gorm.DB, order *store.WorkOrder, eventType string) error {
	if order.PolicySnapshot == "" || strings.HasPrefix(eventType, "sla.") {
		return nil
	}
	var def WorkOrderPolicyDefinition
	if err := json.Unmarshal([]byte(order.PolicySnapshot), &def); err != nil {
		return err
	}
	if def.SLA == nil {
		return nil
	}
	now := time.Now().UTC()
	var instance store.WorkOrderSLAInstance
	err := tx.Where("work_order_id = ? AND workflow_cycle = ?", order.ID, order.WorkflowCycle).First(&instance).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Legacy workflow_cycle increments on every transition. Carry its clock
		// forward until completion; graph cycles represent actual restarts.
		var workflow WorkOrderWorkflowDefinition
		if err := json.Unmarshal([]byte(order.WorkflowSnapshot), &workflow); err != nil {
			return err
		}
		if !isGraphWorkOrderWorkflow(workflow) {
			var previous store.WorkOrderSLAInstance
			lookup := tx.Where("work_order_id = ? AND workflow_cycle < ?", order.ID, order.WorkflowCycle).Order("workflow_cycle DESC").First(&previous).Error
			if lookup != nil && !errors.Is(lookup, gorm.ErrRecordNotFound) {
				return lookup
			}
			if lookup == nil && previous.CompletedAt == nil {
				previous.Model = gorm.Model{}
				previous.WorkflowCycle = order.WorkflowCycle
				previous.Version = 1
				if err := tx.Create(&previous).Error; err != nil {
					return err
				}
				instance = previous
			}
		}
		if instance.ID == 0 {
			response, err := addWorkOrderBusinessTime(*def.SLA, now, time.Duration(def.SLA.ResponseMinutes)*time.Minute)
			if err != nil {
				return err
			}
			resolution, err := addWorkOrderBusinessTime(*def.SLA, now, time.Duration(def.SLA.ResolutionMinutes)*time.Minute)
			if err != nil {
				return err
			}
			raw, _ := json.Marshal(def.SLA)
			instance = store.WorkOrderSLAInstance{TenantID: order.TenantID, ProjectID: order.ProjectID, WorkOrderID: order.ID, WorkflowCycle: order.WorkflowCycle, Version: 1, PolicySnapshot: string(raw), StartedAt: now, ResponseDueAt: response, ResolutionDueAt: resolution}
			if err := tx.Create(&instance).Error; err != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	}
	updates := map[string]any{}
	if instance.RespondedAt == nil && (eventType == "claimed" || eventType == "workflow.task.claimed" || eventType == "workflow.task.completed" || eventType == "transitioned") {
		updates["responded_at"] = now
	}
	// A new workflow cycle starts a new clock. Old cycles remain historical and
	// are excluded by the scanner's join to the aggregate's current cycle.
	var workflow WorkOrderWorkflowDefinition
	if err := json.Unmarshal([]byte(order.WorkflowSnapshot), &workflow); err != nil {
		return err
	}
	status, _ := workflow.statusByKey(order.Status)
	if order.ClosedAt != nil || order.Status == WorkOrderStatusResolved || status.Category == WorkOrderStatusResolved {
		updates["completed_at"] = now
		updates["paused_at"] = nil
	}
	if len(updates) == 0 {
		return nil
	}
	updates["version"] = instance.Version + 1
	changed := tx.Model(&store.WorkOrderSLAInstance{}).Where("id = ? AND version = ?", instance.ID, instance.Version).Updates(updates)
	if changed.Error != nil {
		return changed.Error
	}
	if changed.RowsAffected != 1 {
		return ErrWorkOrderVersionConflict
	}
	return nil
}

func (s *WorkOrderService) GetWorkOrderSLA(scope WorkOrderScope, id uint) (*store.WorkOrderSLAInstance, error) {
	order, err := s.findWorkOrder(s.db, scope, id)
	if err != nil {
		return nil, err
	}
	var result store.WorkOrderSLAInstance
	err = policyScope(s.db, scope).Where("work_order_id = ? AND workflow_cycle = ?", id, order.WorkflowCycle).First(&result).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &result, err
}

func (s *WorkOrderService) PauseWorkOrderSLA(scope WorkOrderScope, id uint, expected int, pause bool, reason string) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if scope.ActorUserID == 0 || expected < 1 || reason == "" || len([]rune(reason)) > 2000 {
		return nil, fmt.Errorf("actor, version and reason are required")
	}
	var result *store.WorkOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, id)
		if err != nil {
			return err
		}
		if order.Version != expected {
			return ErrWorkOrderVersionConflict
		}
		var instance store.WorkOrderSLAInstance
		if err := policyScope(tx, scope).Where("work_order_id = ? AND workflow_cycle = ?", id, order.WorkflowCycle).First(&instance).Error; err != nil {
			return err
		}
		if order.ClosedAt != nil || instance.CompletedAt != nil {
			return fmt.Errorf("SLA is completed")
		}
		var policy WorkOrderSLAPolicy
		if err := json.Unmarshal([]byte(instance.PolicySnapshot), &policy); err != nil {
			return err
		}
		now := time.Now().UTC()
		updates := map[string]any{"version": instance.Version + 1}
		eventType := "sla.resumed"
		if pause {
			if instance.PausedAt != nil {
				return fmt.Errorf("SLA is already paused")
			}
			if !containsPolicyValue(policy.PauseReasons, reason) {
				return fmt.Errorf("pause reason is not allowed by the pinned policy")
			}
			updates["paused_at"] = now
			updates["pause_reason"] = reason
			eventType = "sla.paused"
		} else {
			if instance.PausedAt == nil {
				return fmt.Errorf("SLA is not paused")
			}
			duration := time.Duration(0)
			if err := workOrderBusinessIntervals(policy, *instance.PausedAt, now, func(a, b time.Time) bool { duration += b.Sub(a); return true }); err != nil {
				return err
			}
			response, err := addWorkOrderBusinessTime(policy, instance.ResponseDueAt, duration)
			if err != nil {
				return err
			}
			resolution, err := addWorkOrderBusinessTime(policy, instance.ResolutionDueAt, duration)
			if err != nil {
				return err
			}
			updates["pause_duration"] = instance.PauseDuration + int64(duration)
			updates["paused_at"] = nil
			updates["pause_reason"] = ""
			updates["response_due_at"] = response
			updates["resolution_due_at"] = resolution
		}
		if err := s.updateGraphWorkOrderState(tx, order, order.Status, order.WorkflowCycle, order.ClosedAt); err != nil {
			return err
		}
		changed := tx.Model(&store.WorkOrderSLAInstance{}).Where("id = ? AND version = ?", instance.ID, instance.Version).Updates(updates)
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		if err := s.appendWorkOrderEvent(tx, order, eventType, scope.ActorUserID, map[string]any{"reason": reason}); err != nil {
			return err
		}
		result = order
		return nil
	})
	return result, err
}

// Tick is safe across processes: each signal is protected by the SLA revision,
// and the audit event plus outbox are committed with the signal flag.
func (s *WorkOrderService) TickWorkOrderSLAs(ctx context.Context, now time.Time) error {
	var lastID uint
	for {
		var rows []store.WorkOrderSLAInstance
		err := s.db.WithContext(ctx).Model(&store.WorkOrderSLAInstance{}).
			Joins("JOIN work_orders ON work_orders.id = work_order_sla_instances.work_order_id AND work_orders.workflow_cycle = work_order_sla_instances.workflow_cycle AND work_orders.deleted_at IS NULL AND work_orders.closed_at IS NULL").
			Where("work_order_sla_instances.completed_at IS NULL AND work_order_sla_instances.paused_at IS NULL AND ((work_order_sla_instances.responded_at IS NULL AND work_order_sla_instances.response_breached = ?) OR work_order_sla_instances.resolution_breached = ?)", false, false).
			Where("work_order_sla_instances.id > ?", lastID).Order("work_order_sla_instances.id").Limit(200).Select("work_order_sla_instances.*").Find(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		lastID = rows[len(rows)-1].ID
		for _, row := range rows {
			if err := ctx.Err(); err != nil {
				return err
			}
			err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				var current store.WorkOrderSLAInstance
				if err := tx.First(&current, row.ID).Error; err != nil {
					return err
				}
				if current.PausedAt != nil || current.CompletedAt != nil {
					return nil
				}
				var policy WorkOrderSLAPolicy
				if err := json.Unmarshal([]byte(current.PolicySnapshot), &policy); err != nil {
					return err
				}
				scope := WorkOrderScope{TenantID: current.TenantID, ProjectID: current.ProjectID}
				order, err := s.findWorkOrder(tx, scope, current.WorkOrderID)
				if err != nil {
					return err
				}
				if order.ClosedAt != nil || order.WorkflowCycle != current.WorkflowCycle {
					return nil
				}
				signals := []string{}
				updates := map[string]any{}
				for _, kind := range []string{"response", "resolution"} {
					due, warned, breached := current.ResolutionDueAt, current.ResolutionWarning, current.ResolutionBreached
					minutes := policy.ResolutionMinutes
					if kind == "response" {
						if current.RespondedAt != nil {
							continue
						}
						due, warned, breached = current.ResponseDueAt, current.ResponseWarning, current.ResponseBreached
						minutes = policy.ResponseMinutes
					}
					if !breached && !now.Before(due) {
						updates[kind+"_breached"] = true
						signals = append(signals, "sla."+kind+".breached")
					} else if !warned && !breached && policy.WarningMinutes > 0 {
						threshold, err := addWorkOrderBusinessTime(policy, current.StartedAt, time.Duration(minutes-policy.WarningMinutes)*time.Minute)
						if err != nil {
							return err
						}
						// Due dates include pauses. Shift the warning by the same business time.
						if current.PauseDuration > 0 {
							threshold, err = addWorkOrderBusinessTime(policy, threshold, time.Duration(current.PauseDuration))
							if err != nil {
								return err
							}
						}
						if !now.Before(threshold) {
							updates[kind+"_warning"] = true
							signals = append(signals, "sla."+kind+".warning")
						}
					}
				}
				if len(signals) == 0 {
					return nil
				}
				updates["version"] = current.Version + 1
				changed := tx.Model(&store.WorkOrderSLAInstance{}).Where("id = ? AND version = ? AND paused_at IS NULL AND completed_at IS NULL", current.ID, current.Version).Updates(updates)
				if changed.Error != nil {
					return changed.Error
				}
				if changed.RowsAffected == 0 {
					return nil
				}
				for _, signal := range signals {
					if err := s.appendWorkOrderEvent(tx, order, signal, 0, map[string]any{"sla_instance_id": current.ID, "workflow_cycle": current.WorkflowCycle, "response_due_at": current.ResponseDueAt, "resolution_due_at": current.ResolutionDueAt}); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
}

func (s *WorkOrderService) RunWorkOrderOperations(ctx context.Context) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		if err := s.TickWorkOrderOperations(ctx, time.Now().UTC()); err != nil && ctx.Err() == nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *WorkOrderService) notifyWorkOrderOperation(tx *gorm.DB, order *store.WorkOrder, eventType string, actor uint, payload map[string]any) error {
	if !strings.HasPrefix(eventType, "sla.") || eventType == "sla.paused" || eventType == "sla.resumed" {
		return nil
	}
	recipients := map[uint]bool{}
	for _, userID := range []uint{order.CreatedBy, order.AssigneeUserID, actor} {
		if userID > 0 {
			recipients[userID] = true
		}
	}
	var tasks []store.WorkOrderTask
	if err := tx.Where("work_order_id = ? AND workflow_cycle = ? AND status = ?", order.ID, order.WorkflowCycle, WorkOrderTaskPending).Find(&tasks).Error; err != nil {
		return err
	}
	for _, task := range tasks {
		if task.AssigneeUserID > 0 {
			recipients[task.AssigneeUserID] = true
		}
	}
	for userID := range recipients {
		notification := store.WorkOrderNotification{TenantID: order.TenantID, ProjectID: order.ProjectID, RecipientUserID: userID, WorkOrderID: order.ID, WorkOrderPublicID: order.PublicID, WorkOrderCode: order.Code, WorkOrderTitle: order.Title, Type: fmt.Sprintf("%s.%d.%d", eventType, order.ID, order.WorkflowCycle), Title: BilingualText("工单时限提醒", "Work order SLA alert"), Content: order.Code + ": " + BilingualText("请查看处理时限", "Review the handling deadline"), Status: WorkOrderNotificationUnread}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification).Error; err != nil {
			return err
		}
	}
	return nil
}
