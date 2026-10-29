package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"noyo/core/store"

	"gorm.io/gorm"
)

// WorkOrderAssignment is pinned in the workflow snapshot. Membership is resolved
// when the node activates; changing membership never rewrites assigned tasks.
type WorkOrderAssignment struct {
	Kind         string `json:"kind"`
	RoleID       uint   `json:"role_id"`
	DeliveryMode string `json:"delivery_mode,omitempty"`
}

type WorkOrderAssignmentRole struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type WorkOrderDispatchInput struct {
	ExpectedVersion int    `json:"expected_version"`
	UserIDs         []uint `json:"user_ids"`
	Comment         string `json:"comment"`
}

type workOrderAssignmentUnavailable struct{ Reason string }

func (e *workOrderAssignmentUnavailable) Error() string {
	return "work order assignment unavailable: " + e.Reason
}

func workOrderAssignableRoles(db *gorm.DB, scope WorkOrderScope) *gorm.DB {
	return db.Model(&store.Role{}).Where(
		"(tenant_id = ? AND (project_id = ? OR (project_id = 0 AND is_inherited = ?))) OR (tenant_id = 0 AND project_id = 0 AND is_builtin = ? AND code IN ?)",
		scope.TenantID, scope.ProjectID, true, true, assignableBuiltinRoleCodes)
}

func workOrderAssignmentRole(db *gorm.DB, scope WorkOrderScope, roleID uint) (*store.Role, error) {
	var role store.Role
	err := workOrderAssignableRoles(db, scope).Where("id = ?", roleID).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, &workOrderAssignmentUnavailable{Reason: "role_unavailable"}
	}
	return &role, err
}

func (s *WorkOrderService) ListWorkOrderAssignmentRoles(scope WorkOrderScope) ([]WorkOrderAssignmentRole, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	roles := []WorkOrderAssignmentRole{}
	err := workOrderAssignableRoles(s.db, scope).Select("id, code, name").Order("name, id").Scan(&roles).Error
	return roles, err
}

func (s *WorkOrderService) resolveGraphTaskParticipants(tx *gorm.DB, order *store.WorkOrder, node WorkOrderWorkflowNode) ([]store.User, error) {
	if node.Assignment == nil {
		return s.loadGraphTaskParticipants(tx, order, node.ParticipantUserIDs)
	}
	if _, err := workOrderAssignmentRole(tx, WorkOrderScope{TenantID: order.TenantID, ProjectID: order.ProjectID}, node.Assignment.RoleID); err != nil {
		return nil, err
	}
	users := []store.User{}
	err := tx.Model(&store.User{}).
		Joins("JOIN user_role_bindings AS assignment_bindings ON assignment_bindings.user_id = users.id AND assignment_bindings.tenant_id = users.tenant_id AND assignment_bindings.deleted_at IS NULL").
		Where("users.tenant_id = ? AND users.status = ? AND assignment_bindings.role_id = ? AND (assignment_bindings.project_id = ? OR assignment_bindings.project_id = 0)", order.TenantID, 1, node.Assignment.RoleID, order.ProjectID).
		Select("users.*").Distinct().Order("users.id").Find(&users).Error
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, &workOrderAssignmentUnavailable{Reason: "no_active_members"}
	}
	return users, nil
}

func (s *WorkOrderService) dispatchGraphWorkOrderTask(tx *gorm.DB, order *store.WorkOrder, instance *store.WorkOrderNodeInstance, node WorkOrderWorkflowNode, actorUserID uint, overrideUserIDs []uint, comment string) error {
	var routeErr error
	node, routeErr = effectiveWorkOrderNode(order, node)
	if routeErr != nil {
		return routeErr
	}
	var participants []store.User
	var err error
	if len(overrideUserIDs) > 0 {
		participants, err = s.loadGraphTaskParticipants(tx, order, overrideUserIDs)
	} else {
		participants, err = s.resolveGraphTaskParticipants(tx, order, node)
	}
	var unavailable *workOrderAssignmentUnavailable
	if err != nil && (!errors.As(err, &unavailable) || len(overrideUserIDs) > 0) {
		return err
	}
	status, reason := WorkOrderNodeInstanceActive, ""
	if unavailable != nil {
		status, reason = WorkOrderNodeInstancePendingAssignment, unavailable.Reason
	}
	userIDs := make([]uint, 0, len(participants))
	for _, user := range participants {
		userIDs = append(userIDs, user.ID)
	}
	snapshot, err := json.Marshal(map[string]any{"rule": node.Assignment, "fixed_user_ids": node.ParticipantUserIDs, "user_ids": userIDs, "override": len(overrideUserIDs) > 0})
	if err != nil {
		return err
	}
	// CAS serializes competing retries and invalidates stale management dialogs.
	if err := s.updateGraphWorkOrderState(tx, order, order.Status, order.WorkflowCycle, order.ClosedAt); err != nil {
		return err
	}
	updated := tx.Model(&store.WorkOrderNodeInstance{}).Where("id = ? AND status = ?", instance.ID, instance.Status).
		Updates(map[string]any{"status": status, "dispatch_error": reason, "assignment_snapshot": string(snapshot)})
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != 1 {
		return ErrWorkOrderVersionConflict
	}
	payload := map[string]any{"node_id": node.ID, "node_name": node.Name, "branch_id": instance.BranchID,
		"task_kind": node.TaskKind, "completion_mode": node.CompletionMode, "assignment": json.RawMessage(snapshot), "comment": comment}
	if unavailable != nil {
		payload["reason"] = reason
		return s.appendWorkOrderEvent(tx, order, "workflow.assignment.blocked", actorUserID, payload)
	}
	taskPublicIDs := make([]string, 0, len(participants))
	if node.Assignment != nil && node.Assignment.DeliveryMode == "claim" && len(overrideUserIDs) == 0 {
		if node.CompletionMode != WorkOrderApprovalAny {
			return fmt.Errorf("claim delivery requires any-completion")
		}
		task := store.WorkOrderTask{TenantID: order.TenantID, ProjectID: order.ProjectID, WorkOrderID: order.ID,
			WorkOrderNodeInstanceID: instance.ID, WorkflowCycle: order.WorkflowCycle, NodeID: node.ID, TaskKind: node.TaskKind,
			CompletionMode: node.CompletionMode, DeliveryMode: "claim", Status: WorkOrderTaskPending}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		for _, participant := range participants {
			if err := tx.Create(&store.WorkOrderTaskCandidate{TaskID: task.ID, UserID: participant.ID}).Error; err != nil {
				return err
			}
			notificationTask := task
			notificationTask.AssigneeUserID = participant.ID
			if err := s.createGraphTaskNotification(tx, order, node, notificationTask); err != nil {
				return err
			}
		}
		payload["task_public_ids"] = []string{task.PublicID}
		payload["delivery_mode"] = "claim"
		return s.appendWorkOrderEvent(tx, order, "workflow.task.claimable", actorUserID, payload)
	}
	for _, participant := range participants {
		displayName := strings.TrimSpace(participant.DisplayName)
		if displayName == "" {
			displayName = participant.Username
		}
		task := store.WorkOrderTask{TenantID: order.TenantID, ProjectID: order.ProjectID, WorkOrderID: order.ID,
			WorkOrderNodeInstanceID: instance.ID, WorkflowCycle: order.WorkflowCycle, NodeID: node.ID, TaskKind: node.TaskKind,
			CompletionMode: node.CompletionMode, AssigneeUserID: participant.ID, AssigneeDisplayName: displayName, Status: WorkOrderTaskPending}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		taskPublicIDs = append(taskPublicIDs, task.PublicID)
		if err := s.createGraphTaskNotification(tx, order, node, task); err != nil {
			return err
		}
	}
	payload["task_public_ids"] = taskPublicIDs
	return s.appendWorkOrderEvent(tx, order, "workflow.task.assigned", actorUserID, payload)
}

func (s *WorkOrderService) ListWorkOrderAssignmentBlocks(scope WorkOrderScope, workOrderID uint) ([]store.WorkOrderNodeInstance, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	order, err := s.findWorkOrder(s.db, scope, workOrderID)
	if err != nil {
		return nil, err
	}
	instances := []store.WorkOrderNodeInstance{}
	err = s.db.Where("tenant_id = ? AND project_id = ? AND work_order_id = ? AND workflow_cycle = ? AND status = ?",
		scope.TenantID, scope.ProjectID, order.ID, order.WorkflowCycle, WorkOrderNodeInstancePendingAssignment).Order("id").Find(&instances).Error
	return instances, err
}

func loadWorkOrderAssignmentBlockCounts(db *gorm.DB, orders []store.WorkOrder) (map[uint]int64, error) {
	counts := make(map[uint]int64, len(orders))
	if len(orders) == 0 {
		return counts, nil
	}
	ids := make([]uint, 0, len(orders))
	cycles := make(map[uint]int, len(orders))
	for _, order := range orders {
		ids = append(ids, order.ID)
		cycles[order.ID] = order.WorkflowCycle
	}
	var rows []struct {
		WorkOrderID   uint
		WorkflowCycle int
		Count         int64
	}
	if err := db.Model(&store.WorkOrderNodeInstance{}).Select("work_order_id, workflow_cycle, COUNT(*) AS count").
		Where("work_order_id IN ? AND status = ?", ids, WorkOrderNodeInstancePendingAssignment).
		Group("work_order_id, workflow_cycle").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if cycles[row.WorkOrderID] == row.WorkflowCycle {
			counts[row.WorkOrderID] = row.Count
		}
	}
	return counts, nil
}

// DispatchWorkOrderNode is an administrative recovery operation. The HTTP
// boundary requires work_order:manage; normal task permissions cannot reassign
// a blocked approval or change the published rule.
func (s *WorkOrderService) DispatchWorkOrderNode(scope WorkOrderScope, workOrderID, instanceID uint, input WorkOrderDispatchInput) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 || input.ExpectedVersion < 1 || len(input.UserIDs) > 100 {
		return nil, fmt.Errorf("actor and expected version are required; at most 100 participants are allowed")
	}
	comment := strings.TrimSpace(input.Comment)
	if len([]rune(comment)) > 2000 || (len(input.UserIDs) > 0 && comment == "") {
		return nil, fmt.Errorf("manual assignment requires a reason of at most 2000 characters")
	}
	seen := map[uint]bool{}
	for _, id := range input.UserIDs {
		if id == 0 || seen[id] {
			return nil, fmt.Errorf("assignment participants must be unique nonzero user ids")
		}
		seen[id] = true
	}
	var result *store.WorkOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		if order.Version != input.ExpectedVersion {
			return ErrWorkOrderVersionConflict
		}
		if order.ClosedAt != nil {
			return fmt.Errorf("terminal work order cannot be dispatched")
		}
		var instance store.WorkOrderNodeInstance
		if err := tx.Where("id = ? AND tenant_id = ? AND project_id = ? AND work_order_id = ? AND workflow_cycle = ? AND status = ?",
			instanceID, scope.TenantID, scope.ProjectID, order.ID, order.WorkflowCycle, WorkOrderNodeInstancePendingAssignment).First(&instance).Error; err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		node, ok := workflow.graphNode(instance.NodeID)
		if !ok || node.Type != WorkOrderWorkflowNodeUserTask {
			return fmt.Errorf("blocked node is not a user task")
		}
		if err := s.dispatchGraphWorkOrderTask(tx, order, &instance, node, scope.ActorUserID, input.UserIDs, comment); err != nil {
			return err
		}
		result = order
		return nil
	})
	return result, err
}
