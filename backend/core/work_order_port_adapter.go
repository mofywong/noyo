package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"noyo/core/store"
	"noyo/core/workorder"

	"gorm.io/gorm"
)

const (
	commandReceiptSuccess = "success"
	commandReceiptError   = "error"
)

// LocalCommandAdapter is the compatibility bridge from the stable command
// port to the existing WorkOrderService. It intentionally returns DTOs only.
type LocalCommandAdapter struct {
	service *WorkOrderService
	mu      sync.Mutex
}

func NewLocalCommandAdapter(service *WorkOrderService) *LocalCommandAdapter {
	return &LocalCommandAdapter{service: service}
}

func (a *LocalCommandAdapter) Create(ctx context.Context, command workorder.CreateCommand) (workorder.CommandResult, error) {
	if err := validateCommandContext(ctx); err != nil {
		return workorder.CommandResult{}, err
	}
	if err := validateCommandScope(command.Scope, "work_order:create"); err != nil {
		return workorder.CommandResult{}, err
	}
	if strings.TrimSpace(command.TemplateCode) == "" || strings.TrimSpace(command.Title) == "" {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "template code and title are required")
	}
	return a.withReceipt(command.Scope, command.Meta, "create", command, func() (workorder.CommandResult, error) {
		templateID, err := a.templateID(command.Scope, command.TemplateCode)
		if err != nil {
			return workorder.CommandResult{}, err
		}
		sourceType := strings.TrimSpace(command.SourceType)
		if sourceType == "" {
			sourceType = WorkOrderSourceManual
		}
		sourceRef := strings.TrimSpace(command.SourceRef)
		if sourceRef == "" {
			sourceRef = "command:" + command.Meta.IdempotencyKey
		}
		order, created, err := a.service.CreateWorkOrder(WorkOrderCreateInput{
			Scope:      a.serviceScope(command.Scope),
			TemplateID: templateID,
			Title:      command.Title,
			Summary:    command.Summary,
			Priority:   command.Priority,
			Source: WorkOrderSource{
				Type:           sourceType,
				ID:             sourceRef,
				IdempotencyKey: command.Meta.IdempotencyKey,
				Snapshot:       command.SourceSnapshot,
			},
			FormData: command.FormData,
		})
		if err != nil {
			return workorder.CommandResult{}, mapCommandError(err)
		}
		return a.resultFromOrder(order, created)
	})
}

func (a *LocalCommandAdapter) Execute(ctx context.Context, command workorder.ExecuteCommand) (workorder.CommandResult, error) {
	if err := validateCommandContext(ctx); err != nil {
		return workorder.CommandResult{}, err
	}
	if err := validateCommandScope(command.Scope, "work_order:process"); err != nil {
		return workorder.CommandResult{}, err
	}
	if strings.TrimSpace(command.WorkOrderPublicID) == "" || strings.TrimSpace(command.Action) == "" {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "work order public id and action are required")
	}
	if !strings.EqualFold(strings.TrimSpace(command.Action), "claim") && strings.TrimSpace(command.Comment) == "" {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "processing opinion is required")
	}
	return a.withReceipt(command.Scope, command.Meta, "execute", command, func() (workorder.CommandResult, error) {
		serviceScope := a.serviceScope(command.Scope)
		order, err := a.service.findWorkOrderByPublicID(a.service.db, serviceScope, command.WorkOrderPublicID)
		if err != nil {
			return workorder.CommandResult{}, mapCommandError(err)
		}
		if command.Meta.ExpectedVersion != nil && *command.Meta.ExpectedVersion != order.Version {
			return workorder.CommandResult{}, mapCommandError(ErrWorkOrderVersionConflict)
		}
		var updated *store.WorkOrder
		if taskPublicID := strings.TrimSpace(command.TaskPublicID); taskPublicID != "" {
			if !strings.EqualFold(strings.TrimSpace(command.Action), "complete") {
				return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "graph work order tasks only support the complete action")
			}
			task, taskErr := a.service.findWorkOrderTaskByPublicID(a.service.db, serviceScope, taskPublicID)
			if taskErr != nil {
				return workorder.CommandResult{}, mapCommandError(taskErr)
			}
			if task.WorkOrderID != order.ID {
				return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "work order task does not belong to the work order")
			}
			attachments := make([]WorkOrderAttachment, 0, len(command.Attachments))
			for _, att := range command.Attachments {
				attachments = append(attachments, WorkOrderAttachment{Name: att.Name, URL: att.URL, Size: att.Size, Type: att.Type})
			}
			updated, err = a.service.CompleteWorkOrderTask(serviceScope, taskPublicID, WorkOrderTaskCompletionInput{Comment: command.Comment, Attachments: attachments})
		} else if strings.EqualFold(strings.TrimSpace(command.Action), "claim") {
			updated, err = a.service.ClaimWorkOrderWithVersion(serviceScope, order.ID, command.Meta.ExpectedVersion)
		} else {
			attachments := make([]WorkOrderAttachment, 0, len(command.Attachments))
			for _, att := range command.Attachments {
				attachments = append(attachments, WorkOrderAttachment{Name: att.Name, URL: att.URL, Size: att.Size, Type: att.Type})
			}
			updated, err = a.service.TransitionWorkOrderWithVersion(serviceScope, order.ID, WorkOrderTransitionInput{
				Key: command.Action, Comment: command.Comment,
				Resolution: WorkOrderResolution{
					ActualProblem: command.Resolution.ActualProblem, RootCause: command.Resolution.RootCause,
					HandlingProcess: command.Resolution.HandlingProcess, HandlingResult: command.Resolution.HandlingResult,
					AttachmentIDs: command.Resolution.AttachmentIDs,
				},
				Attachments: attachments,
			}, command.Meta.ExpectedVersion)
		}
		if err != nil {
			return workorder.CommandResult{}, mapCommandError(err)
		}
		return a.resultFromOrder(updated, false)
	})
}

func (a *LocalCommandAdapter) Approve(ctx context.Context, command workorder.ApprovalCommand) (workorder.CommandResult, error) {
	return a.decide(ctx, command, false)
}

func (a *LocalCommandAdapter) Reject(ctx context.Context, command workorder.ApprovalCommand) (workorder.CommandResult, error) {
	return a.decide(ctx, command, true)
}

func (a *LocalCommandAdapter) decide(ctx context.Context, command workorder.ApprovalCommand, reject bool) (workorder.CommandResult, error) {
	if err := validateCommandContext(ctx); err != nil {
		return workorder.CommandResult{}, err
	}
	if err := validateCommandScope(command.Scope, "work_order:approve"); err != nil {
		return workorder.CommandResult{}, err
	}
	if strings.TrimSpace(command.WorkOrderPublicID) == "" {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "work order public id is required")
	}
	if strings.TrimSpace(command.Comment) == "" {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "processing opinion is required")
	}
	commandType := "approve"
	if reject {
		commandType = "reject"
	}
	return a.withReceipt(command.Scope, command.Meta, commandType, command, func() (workorder.CommandResult, error) {
		serviceScope := a.serviceScope(command.Scope)
		order, err := a.service.findWorkOrderByPublicID(a.service.db, serviceScope, command.WorkOrderPublicID)
		if err != nil {
			return workorder.CommandResult{}, mapCommandError(err)
		}
		if command.Meta.ExpectedVersion != nil && *command.Meta.ExpectedVersion != order.Version {
			return workorder.CommandResult{}, mapCommandError(ErrWorkOrderVersionConflict)
		}
		var updated *store.WorkOrder
		if taskPublicID := strings.TrimSpace(command.TaskPublicID); taskPublicID != "" {
			task, taskErr := a.service.findWorkOrderTaskByPublicID(a.service.db, serviceScope, taskPublicID)
			if taskErr != nil {
				return workorder.CommandResult{}, mapCommandError(taskErr)
			}
			if task.WorkOrderID != order.ID {
				return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "work order task does not belong to the work order")
			}
			if reject {
				updated, err = a.service.RejectWorkOrderTask(serviceScope, taskPublicID, WorkOrderTaskCompletionInput{Comment: command.Comment})
			} else {
				updated, err = a.service.CompleteWorkOrderTask(serviceScope, taskPublicID, WorkOrderTaskCompletionInput{Comment: command.Comment})
			}
		} else if reject {
			input := WorkOrderApprovalInput{Comment: command.Comment}
			updated, err = a.service.RejectWorkOrderWithVersion(serviceScope, order.ID, input, command.Meta.ExpectedVersion)
		} else {
			input := WorkOrderApprovalInput{Comment: command.Comment}
			updated, err = a.service.ApproveWorkOrderWithVersion(serviceScope, order.ID, input, command.Meta.ExpectedVersion)
		}
		if err != nil {
			return workorder.CommandResult{}, mapCommandError(err)
		}
		return a.resultFromOrder(updated, false)
	})
}

func (a *LocalCommandAdapter) templateID(scope workorder.Scope, code string) (uint, error) {
	if numericID, parseErr := strconv.ParseUint(strings.TrimSpace(code), 10, 32); parseErr == nil && numericID > 0 {
		template, err := a.service.GetTemplate(a.serviceScope(scope), uint(numericID))
		if err != nil {
			return 0, mapCommandError(err)
		}
		return template.ID, nil
	}
	templates, err := a.service.ListTemplates(a.serviceScope(scope))
	if err != nil {
		return 0, mapCommandError(err)
	}
	for _, template := range templates {
		if template.Code == strings.TrimSpace(code) {
			return template.ID, nil
		}
	}
	return 0, workorder.NewError(workorder.CodeWorkOrderNotFound, "work order template not found")
}

func (a *LocalCommandAdapter) resultFromOrder(order *store.WorkOrder, created bool) (workorder.CommandResult, error) {
	if order == nil {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeWorkOrderNotFound, "work order not found")
	}
	result := workorder.CommandResult{PublicID: order.PublicID, Code: order.Code, Version: order.Version, Created: created}
	workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
	if err != nil {
		return workorder.CommandResult{}, mapCommandError(err)
	}
	if isGraphWorkOrderWorkflow(workflow) {
		result.StatusKey = order.Status
		result.StatusCategory = order.Status
		return result, nil
	}
	if status, ok := workflow.statusByKey(order.Status); ok {
		result.StatusKey = status.Key
		result.StatusCategory = status.Category
	}
	for _, transition := range workflow.Transitions {
		if transition.From == order.Status {
			result.AvailableActions = append(result.AvailableActions, transition.Key)
		}
	}
	return result, nil
}

func (a *LocalCommandAdapter) withReceipt(scope workorder.Scope, meta workorder.CommandMeta, commandType string, command any, fn func() (workorder.CommandResult, error)) (workorder.CommandResult, error) {
	// Serialize commands through the same local adapter while the durable
	// receipt is being completed. The database uniqueness constraint still
	// protects multiple server instances; this lock prevents a second request
	// in one process from observing the transient empty response.
	a.mu.Lock()
	defer a.mu.Unlock()
	if strings.TrimSpace(meta.IdempotencyKey) == "" {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "idempotency key is required")
	}
	digestBytes, err := json.Marshal(command)
	if err != nil {
		return workorder.CommandResult{}, workorder.NewError(workorder.CodeValidationFailed, "cannot encode command")
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(digestBytes))
	var result workorder.CommandResult
	var operationErr error
	var receipt store.WorkOrderCommandReceipt
	lookup := a.service.db.Where("tenant_id = ? AND project_id = ? AND principal_type = ? AND principal_ref = ? AND idempotency_key = ?", scope.TenantID, scope.ProjectID, scope.Principal.Type, scope.Principal.Ref, meta.IdempotencyKey)
	if findErr := lookup.First(&receipt).Error; findErr == nil {
		if receipt.RequestDigest != digest {
			return workorder.CommandResult{}, workorder.NewError(workorder.CodeIdempotencyConflict, "idempotency key was already used with a different command")
		}
		if receipt.Status == commandReceiptError {
			return workorder.CommandResult{}, workorder.NewError(receipt.ErrorCode, "command previously failed")
		}
		if decodeErr := json.Unmarshal([]byte(receipt.ResponseJSON), &result); decodeErr != nil {
			return workorder.CommandResult{}, decodeErr
		}
		return result, nil
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return workorder.CommandResult{}, mapCommandError(findErr)
	}
	receipt = store.WorkOrderCommandReceipt{TenantID: scope.TenantID, ProjectID: scope.ProjectID, PrincipalType: scope.Principal.Type, PrincipalRef: scope.Principal.Ref, IdempotencyKey: meta.IdempotencyKey, RequestDigest: digest, CommandType: commandType, Status: commandReceiptSuccess}
	if createErr := a.service.db.Create(&receipt).Error; createErr != nil {
		// Another caller may have won the unique receipt race. Re-read it and
		// return its stable result rather than executing the command twice.
		if reloadErr := lookup.First(&receipt).Error; reloadErr == nil {
			if receipt.RequestDigest != digest {
				return workorder.CommandResult{}, workorder.NewError(workorder.CodeIdempotencyConflict, "idempotency key was already used with a different command")
			}
			if receipt.Status == commandReceiptSuccess {
				if decodeErr := json.Unmarshal([]byte(receipt.ResponseJSON), &result); decodeErr == nil {
					return result, nil
				}
			}
		}
		return workorder.CommandResult{}, mapCommandError(createErr)
	}
	result, operationErr = fn()
	if operationErr != nil {
		var commandErr *workorder.Error
		if errors.As(operationErr, &commandErr) {
			receipt.Status = commandReceiptError
			receipt.ErrorCode = commandErr.Code
		}
	} else if encoded, encodeErr := json.Marshal(result); encodeErr != nil {
		return workorder.CommandResult{}, encodeErr
	} else {
		receipt.ResponseJSON = string(encoded)
	}
	if saveErr := a.service.db.Save(&receipt).Error; saveErr != nil {
		return workorder.CommandResult{}, mapCommandError(saveErr)
	}
	if err != nil {
		return workorder.CommandResult{}, mapCommandError(err)
	}
	return result, operationErr
}

func validateCommandContext(ctx context.Context) error {
	if ctx == nil {
		return workorder.NewError(workorder.CodeValidationFailed, "context is required")
	}
	return nil
}

func validateCommandScope(scope workorder.Scope, permission string) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return workorder.NewError(workorder.CodeValidationFailed, "tenant and project scope are required")
	}
	if strings.TrimSpace(scope.Principal.Type) == "" || strings.TrimSpace(scope.Principal.Ref) == "" {
		return workorder.NewError(workorder.CodeValidationFailed, "principal identity is required")
	}
	if len(scope.AllowedProjectIDs) > 0 {
		allowed := false
		for _, projectID := range scope.AllowedProjectIDs {
			if projectID == scope.ProjectID {
				allowed = true
				break
			}
		}
		if !allowed {
			return workorder.NewError(workorder.CodePermissionDenied, "principal is not allowed in the project scope")
		}
	}
	if scope.Principal.Permissions != nil && !scope.Principal.Permissions[permission] {
		return workorder.NewError(workorder.CodePermissionDenied, "principal lacks command permission")
	}
	return nil
}

func (a *LocalCommandAdapter) serviceScope(scope workorder.Scope) WorkOrderScope {
	actor := uint(0)
	if parsed, err := strconv.ParseUint(scope.Principal.Ref, 10, 32); err == nil {
		actor = uint(parsed)
	}
	return WorkOrderScope{TenantID: scope.TenantID, ProjectID: scope.ProjectID, ActorUserID: actor}
}

func mapCommandError(err error) error {
	if err == nil {
		return nil
	}
	var commandErr *workorder.Error
	if errors.As(err, &commandErr) {
		return err
	}
	if errors.Is(err, ErrWorkOrderVersionConflict) {
		return workorder.NewError(workorder.CodeVersionConflict, err.Error())
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "not found"):
		return workorder.NewError(workorder.CodeWorkOrderNotFound, err.Error())
	case strings.Contains(message, "not allowed") || strings.Contains(message, "transition") || strings.Contains(message, "approval"):
		return workorder.NewError(workorder.CodeInvalidTransition, err.Error())
	case strings.Contains(message, "permission") || strings.Contains(message, "assigned"):
		return workorder.NewError(workorder.CodePermissionDenied, err.Error())
	default:
		return workorder.NewError(workorder.CodeValidationFailed, err.Error())
	}
}
