package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"noyo/core/store"
	workorderport "noyo/core/workorder"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	DeviceMaintenanceWorkOrderTemplateCode = "ai_device_maintenance"

	WorkOrderSourceManual   = "manual"
	WorkOrderSourceRule     = "rule"
	WorkOrderSourceAlarm    = "alarm"
	WorkOrderSourceAI       = "ai"
	WorkOrderSourceExternal = "external"

	WorkOrderFormFieldText        = "text"
	WorkOrderFormFieldTextarea    = "textarea"
	WorkOrderFormFieldNumber      = "number"
	WorkOrderFormFieldInteger     = "integer"
	WorkOrderFormFieldBoolean     = "boolean"
	WorkOrderFormFieldSelect      = "select"
	WorkOrderFormFieldMultiSelect = "multi_select"
	WorkOrderFormFieldDate        = "date"
	WorkOrderFormFieldDateTime    = "datetime"
	WorkOrderFormFieldDevice      = "device"
	WorkOrderFormFieldUser        = "user"
	WorkOrderFormFieldImages      = "images"
	WorkOrderFormFieldAttachment  = "attachment"

	WorkOrderStatusOpen       = "open"
	WorkOrderStatusInProgress = "in_progress"
	WorkOrderStatusResolved   = "resolved"
	WorkOrderStatusClosed     = "closed"
	WorkOrderStatusCancelled  = "cancelled"

	WorkOrderApprovalPending   = "pending"
	WorkOrderApprovalApproved  = "approved"
	WorkOrderApprovalRejected  = "rejected"
	WorkOrderApprovalCancelled = "cancelled"
	WorkOrderApprovalAny       = "any"
	WorkOrderApprovalAll       = "all"

	WorkOrderWorkflowSchemaV2 = 2

	WorkOrderWorkflowNodeStart         = "start"
	WorkOrderWorkflowNodeUserTask      = "user_task"
	WorkOrderWorkflowNodeCCTask        = "cc_task"
	WorkOrderWorkflowNodeParallelSplit = "parallel_split"
	WorkOrderWorkflowNodeParallelJoin  = "parallel_join"
	WorkOrderWorkflowNodeEnd           = "end"

	WorkOrderTaskKindHandle  = "handle"
	WorkOrderTaskKindApprove = "approve"

	WorkOrderNodeInstanceActive    = "active"
	WorkOrderNodeInstanceCompleted = "completed"
	WorkOrderNodeInstanceCancelled = "cancelled"

	WorkOrderTaskPending   = "pending"
	WorkOrderTaskCompleted = "completed"
	WorkOrderTaskRejected  = "rejected"
	WorkOrderTaskCancelled = "cancelled"

	WorkOrderNotificationUnread = "unread"
	WorkOrderNotificationRead   = "read"
	WorkOrderNotificationTask   = "work_order_task_assigned"

	WorkOrderOutboxPending    = "pending"
	WorkOrderOutboxDelivering = "delivering"
	WorkOrderOutboxDelivered  = "delivered"
)

// ErrWorkOrderApprovalPending is returned when a plain transition is attempted
// while the current workflow cycle still has an unresolved approval task.
var ErrWorkOrderApprovalPending = errors.New("work order has pending approval")

// ErrWorkOrderVersionConflict is kept private to the legacy service boundary;
// the workorder port maps it to its stable VERSION_CONFLICT code.
var ErrWorkOrderVersionConflict = errors.New("work order version conflict")

type WorkOrderService struct {
	db *gorm.DB
}

func NewWorkOrderService(db *gorm.DB) *WorkOrderService {
	return &WorkOrderService{db: db}
}

type WorkOrderScope struct {
	TenantID    uint
	ProjectID   uint
	ActorUserID uint
}

type WorkOrderParticipant struct {
	ID          uint   `json:"id"`
	DisplayName string `json:"display_name"`
	Username    string `json:"username"`
}

func (s *WorkOrderService) ListWorkOrderParticipants(scope WorkOrderScope, search string, page, pageSize int) ([]WorkOrderParticipant, int64, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := s.db.Model(&store.User{}).
		Joins("JOIN user_role_bindings AS participant_bindings ON participant_bindings.user_id = users.id AND participant_bindings.tenant_id = users.tenant_id").
		Where("users.tenant_id = ? AND users.status = ? AND (participant_bindings.project_id = ? OR participant_bindings.project_id = 0)", scope.TenantID, 1, scope.ProjectID)
	search = strings.TrimSpace(search)
	if search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(users.display_name) LIKE ? OR LOWER(users.username) LIKE ?", pattern, pattern)
	}
	var total int64
	if err := query.Distinct("users.id").Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count work order participants: %w", err)
	}
	var users []store.User
	if err := query.Select("users.*").Distinct().Order("users.display_name ASC, users.username ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list work order participants: %w", err)
	}
	participants := make([]WorkOrderParticipant, 0, len(users))
	for _, user := range users {
		participants = append(participants, WorkOrderParticipant{
			ID: user.ID, DisplayName: user.DisplayName, Username: user.Username,
		})
	}
	return participants, total, nil
}

type WorkOrderTemplateInput struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type WorkOrderFormDefinition struct {
	Fields []WorkOrderFormField `json:"fields"`
}

type WorkOrderFormField struct {
	Key          string   `json:"key"`
	Label        string   `json:"label"`
	Type         string   `json:"type"`
	Required     bool     `json:"required"`
	Options      []string `json:"options,omitempty"`
	DefaultValue any      `json:"default_value,omitempty"`
}

type WorkOrderWorkflowDefinition struct {
	SchemaVersion int                           `json:"schema_version,omitempty"`
	StartNodeID   string                        `json:"start_node_id,omitempty"`
	Nodes         []WorkOrderWorkflowNode       `json:"nodes,omitempty"`
	Edges         []WorkOrderWorkflowEdge       `json:"edges,omitempty"`
	Layout        map[string]any                `json:"layout,omitempty"`
	InitialStatus string                        `json:"initial_status"`
	Statuses      []WorkOrderWorkflowStatus     `json:"statuses"`
	Transitions   []WorkOrderWorkflowTransition `json:"transitions"`
}

type WorkOrderWorkflowNode struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Name               string `json:"name,omitempty"`
	TaskKind           string `json:"task_kind,omitempty"`
	ParticipantUserIDs []uint `json:"participant_user_ids,omitempty"`
	CompletionMode     string `json:"completion_mode,omitempty"`
	PairID             string `json:"pair_id,omitempty"`
	RejectTargetNodeID string `json:"reject_target_node_id,omitempty"`
	Result             string `json:"result,omitempty"`
}

type WorkOrderWorkflowEdge struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	BranchID string `json:"branch_id,omitempty"`
}

type WorkOrderWorkflowStatus struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type WorkOrderWorkflowTransition struct {
	Key               string `json:"key"`
	Name              string `json:"name"`
	From              string `json:"from"`
	To                string `json:"to"`
	RequireResolution bool   `json:"require_resolution,omitempty"`
	ApproverUserIDs   []uint `json:"approver_user_ids,omitempty"`
	ApprovalMode      string `json:"approval_mode,omitempty"`
	RejectTo          string `json:"reject_to,omitempty"`
}

type WorkOrderSource struct {
	Type           string         `json:"type"`
	ID             string         `json:"id"`
	IdempotencyKey string         `json:"idempotency_key"`
	Snapshot       map[string]any `json:"snapshot,omitempty"`
}

type WorkOrderCreateInput struct {
	Scope      WorkOrderScope
	TemplateID uint
	Title      string
	Summary    string
	Priority   string
	Source     WorkOrderSource
	FormData   map[string]any
}

type WorkOrderTransitionInput struct {
	Key        string              `json:"key"`
	Comment    string              `json:"comment"`
	Resolution WorkOrderResolution `json:"resolution"`
}

type WorkOrderResolution struct {
	ActualProblem   string   `json:"actual_problem"`
	RootCause       string   `json:"root_cause"`
	HandlingProcess string   `json:"handling_process"`
	HandlingResult  string   `json:"handling_result"`
	AttachmentIDs   []string `json:"attachment_ids,omitempty"`
}

type WorkOrderApprovalInput struct {
	Comment string `json:"comment"`
}

func requiredProcessingOpinion(comment string) (string, error) {
	opinion := strings.TrimSpace(comment)
	if opinion == "" {
		return "", fmt.Errorf("processing opinion is required")
	}
	return opinion, nil
}

type WorkOrderTaskCompletionInput struct {
	Comment string `json:"comment"`
}

type WorkOrderTransferInput struct {
	TargetUserID uint   `json:"target_user_id"`
	TaskPublicID string `json:"task_public_id,omitempty"`
	Comment      string `json:"comment"`
}

type WorkOrderListOptions struct {
	Page             int
	PageSize         int
	Search           string
	Status           string
	Statuses         []string
	SourceType       string
	TemplateID       uint
	AssigneeID       uint
	Relation         string
	LinkRelationType string
	LinkExternalRef  string
}

type WorkOrderLinkInput struct {
	RelationType string         `json:"relation_type"`
	ExternalRef  string         `json:"external_ref"`
	Metadata     map[string]any `json:"metadata"`
}

type WorkOrderTemplateDetail struct {
	Template                store.WorkOrderTemplate        `json:"template"`
	FormVersion             store.WorkOrderFormVersion     `json:"form_version"`
	FormDefinition          WorkOrderFormDefinition        `json:"form_definition"`
	FormDraftDefinition     WorkOrderFormDefinition        `json:"form_draft_definition"`
	HasFormDraft            bool                           `json:"has_form_draft"`
	WorkflowVersion         store.WorkOrderWorkflowVersion `json:"workflow_version"`
	WorkflowDefinition      WorkOrderWorkflowDefinition    `json:"workflow_definition"`
	WorkflowDraftDefinition WorkOrderWorkflowDefinition    `json:"workflow_draft_definition"`
	HasWorkflowDraft        bool                           `json:"has_workflow_draft"`
}

func (s *WorkOrderService) CreateTemplate(scope WorkOrderScope, input WorkOrderTemplateInput) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	input.Code = strings.TrimSpace(input.Code)
	input.Name = strings.TrimSpace(input.Name)
	if input.Code == DeviceMaintenanceWorkOrderTemplateCode {
		return nil, fmt.Errorf("template code %q is reserved for the AI maintenance adapter", input.Code)
	}
	if !isWorkOrderIdentifier(input.Code) {
		return nil, fmt.Errorf("template code must contain 2-64 lowercase letters, digits, hyphens, or underscores")
	}
	if input.Name == "" || len([]rune(input.Name)) > 128 {
		return nil, fmt.Errorf("template name must contain 1-128 characters")
	}
	template := &store.WorkOrderTemplate{
		TenantID:    scope.TenantID,
		ProjectID:   scope.ProjectID,
		Code:        input.Code,
		Name:        input.Name,
		Description: strings.TrimSpace(input.Description),
		Enabled:     true,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(template).Error; err != nil {
			return fmt.Errorf("create work order template: %w", err)
		}
		formDefinition := WorkOrderFormDefinition{Fields: []WorkOrderFormField{}}
		workflowDefinition := defaultWorkOrderWorkflowDefinition()
		compiledForm, err := compileLegacyWorkOrderForm(formDefinition)
		if err != nil {
			return err
		}
		formJSON, err := json.Marshal(formDefinition)
		if err != nil {
			return fmt.Errorf("encode default work order form: %w", err)
		}
		workflowJSON, err := json.Marshal(workflowDefinition)
		if err != nil {
			return fmt.Errorf("encode default work order workflow: %w", err)
		}
		if err := tx.Create(&store.WorkOrderFormVersion{
			TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: template.ID,
			Version: 1, Definition: string(formJSON), Checksum: compiledForm.Checksum,
			SchemaJSON: string(compiledSnapshot(compiledForm, "schema")), UISchemaJSON: string(compiledSnapshot(compiledForm, "ui_schema")),
			DefaultsJSON: string(compiledSnapshot(compiledForm, "defaults")), RequiredJSON: string(compiledSnapshot(compiledForm, "required")),
			FieldOrderJSON: string(compiledSnapshot(compiledForm, "field_order")), ResourceFieldsJSON: string(compiledSnapshot(compiledForm, "resource_fields")), PublishedBy: scope.ActorUserID,
		}).Error; err != nil {
			return fmt.Errorf("create default work order form version: %w", err)
		}
		if err := tx.Create(&store.WorkOrderWorkflowVersion{
			TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: template.ID,
			Version: 1, Definition: string(workflowJSON), PublishedBy: scope.ActorUserID,
		}).Error; err != nil {
			return fmt.Errorf("create default work order workflow version: %w", err)
		}
		template.CurrentFormVersion = 1
		template.CurrentWorkflowVersion = 1
		return tx.Save(template).Error
	}); err != nil {
		return nil, err
	}
	return template, nil
}

// CopyWorkOrderTemplate creates an independently editable copy of the latest
// form and workflow versions. The source remains unchanged, including when it
// is a system-managed template.
func (s *WorkOrderService) CopyWorkOrderTemplate(scope WorkOrderScope, templateID uint) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var copied store.WorkOrderTemplate
	err := s.db.Transaction(func(tx *gorm.DB) error {
		source, err := s.findTemplate(tx, scope, templateID)
		if err != nil {
			return err
		}
		if source.CurrentFormVersion == 0 {
			return fmt.Errorf("work order template has no published form")
		}
		var sourceForm store.WorkOrderFormVersion
		if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, source.ID, source.CurrentFormVersion).First(&sourceForm).Error; err != nil {
			return fmt.Errorf("load source form version: %w", err)
		}

		code, err := nextCopiedWorkOrderTemplateCode(tx, scope, source.Code)
		if err != nil {
			return err
		}
		copied = store.WorkOrderTemplate{
			TenantID:                scope.TenantID,
			ProjectID:               scope.ProjectID,
			Code:                    code,
			Name:                    copiedWorkOrderTemplateName(source.Name),
			Description:             source.Description,
			Enabled:                 source.Enabled,
			FormDraftDefinition:     source.FormDraftDefinition,
			WorkflowDraftDefinition: source.WorkflowDraftDefinition,
		}
		if err := tx.Create(&copied).Error; err != nil {
			return fmt.Errorf("create copied work order template: %w", err)
		}
		copiedForm := store.WorkOrderFormVersion{
			TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: copied.ID,
			Version: 1, Definition: sourceForm.Definition, Checksum: sourceForm.Checksum,
			SchemaJSON: sourceForm.SchemaJSON, UISchemaJSON: sourceForm.UISchemaJSON,
			DefaultsJSON: sourceForm.DefaultsJSON, RequiredJSON: sourceForm.RequiredJSON,
			FieldOrderJSON: sourceForm.FieldOrderJSON, ResourceFieldsJSON: sourceForm.ResourceFieldsJSON,
			PublishedBy: scope.ActorUserID,
		}
		if err := tx.Create(&copiedForm).Error; err != nil {
			return fmt.Errorf("create copied work order form version: %w", err)
		}
		copied.CurrentFormVersion = copiedForm.Version

		if source.CurrentWorkflowVersion > 0 {
			var sourceWorkflow store.WorkOrderWorkflowVersion
			if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, source.ID, source.CurrentWorkflowVersion).First(&sourceWorkflow).Error; err != nil {
				return fmt.Errorf("load source workflow version: %w", err)
			}
			copiedWorkflow := store.WorkOrderWorkflowVersion{
				TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: copied.ID,
				Version: 1, Definition: sourceWorkflow.Definition, PublishedBy: scope.ActorUserID,
			}
			if err := tx.Create(&copiedWorkflow).Error; err != nil {
				return fmt.Errorf("create copied work order workflow version: %w", err)
			}
			copied.CurrentWorkflowVersion = copiedWorkflow.Version
		}
		if err := tx.Save(&copied).Error; err != nil {
			return fmt.Errorf("save copied work order template: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &copied, nil
}

func copiedWorkOrderTemplateName(sourceName string) string {
	const suffix = "-副本"
	name := []rune(strings.TrimSpace(sourceName))
	limit := 128 - len([]rune(suffix))
	if len(name) > limit {
		name = name[:limit]
	}
	return string(name) + suffix
}

func nextCopiedWorkOrderTemplateCode(tx *gorm.DB, scope WorkOrderScope, sourceCode string) (string, error) {
	const suffixLength = len("-copy-") + 8
	base := strings.TrimSpace(sourceCode)
	if len(base) > 64-suffixLength {
		base = base[:64-suffixLength]
	}
	for attempt := 0; attempt < 8; attempt++ {
		code := base + "-copy-" + uuid.NewString()[:8]
		var count int64
		if err := tx.Unscoped().Model(&store.WorkOrderTemplate{}).Where("tenant_id = ? AND project_id = ? AND code = ?", scope.TenantID, scope.ProjectID, code).Count(&count).Error; err != nil {
			return "", fmt.Errorf("check copied work order template code: %w", err)
		}
		if count == 0 {
			return code, nil
		}
	}
	return "", fmt.Errorf("generate unique work order template copy code")
}

func (s *WorkOrderService) ListTemplates(scope WorkOrderScope) ([]store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var templates []store.WorkOrderTemplate
	if err := s.db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID).
		Order("id ASC").Find(&templates).Error; err != nil {
		return nil, fmt.Errorf("list work order templates: %w", err)
	}
	return templates, nil
}

func (s *WorkOrderService) GetTemplate(scope WorkOrderScope, templateID uint) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	return s.findTemplate(s.db, scope, templateID)
}

func (s *WorkOrderService) GetTemplateDetail(scope WorkOrderScope, templateID uint) (*WorkOrderTemplateDetail, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var detail WorkOrderTemplateDetail
	err := s.db.Transaction(func(tx *gorm.DB) error {
		template, err := s.findTemplate(tx, scope, templateID)
		if err != nil {
			return err
		}
		if template.CurrentFormVersion == 0 {
			return fmt.Errorf("work order template has no published form")
		}
		var formVersion store.WorkOrderFormVersion
		if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, template.ID, template.CurrentFormVersion).First(&formVersion).Error; err != nil {
			return fmt.Errorf("load current form version: %w", err)
		}
		formDefinition, err := decodeWorkOrderFormDefinition(formVersion.Definition)
		if err != nil {
			return err
		}
		var workflowVersion store.WorkOrderWorkflowVersion
		workflowDefinition := WorkOrderWorkflowDefinition{}
		if template.CurrentWorkflowVersion > 0 {
			if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, template.ID, template.CurrentWorkflowVersion).First(&workflowVersion).Error; err != nil {
				return fmt.Errorf("load current workflow version: %w", err)
			}
			workflowDefinition, err = decodeWorkOrderWorkflowDefinition(workflowVersion.Definition)
			if err != nil {
				return err
			}
		}
		hasFormDraft := strings.TrimSpace(template.FormDraftDefinition) != ""
		formDraftDefinition := WorkOrderFormDefinition{}
		if hasFormDraft {
			formDraftDefinition, err = decodeWorkOrderFormDraft(template.FormDraftDefinition)
			if err != nil {
				return fmt.Errorf("decode form draft: %w", err)
			}
		}
		hasWorkflowDraft := strings.TrimSpace(template.WorkflowDraftDefinition) != ""
		workflowDraftDefinition := WorkOrderWorkflowDefinition{}
		if hasWorkflowDraft {
			workflowDraftDefinition, err = decodeWorkOrderWorkflowDraft(template.WorkflowDraftDefinition)
			if err != nil {
				return fmt.Errorf("decode workflow draft: %w", err)
			}
			workflowDraftDefinition = normalizeWorkOrderWorkflowDefinition(workflowDraftDefinition)
		}
		if template.CurrentWorkflowVersion == 0 && !hasWorkflowDraft {
			return fmt.Errorf("work order template has no workflow configuration")
		}
		detail = WorkOrderTemplateDetail{
			Template:                *template,
			FormVersion:             formVersion,
			FormDefinition:          formDefinition,
			FormDraftDefinition:     formDraftDefinition,
			HasFormDraft:            hasFormDraft,
			WorkflowVersion:         workflowVersion,
			WorkflowDefinition:      workflowDefinition,
			WorkflowDraftDefinition: workflowDraftDefinition,
			HasWorkflowDraft:        hasWorkflowDraft,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &detail, nil
}

func (s *WorkOrderService) SaveFormDraft(scope WorkOrderScope, templateID uint, definition WorkOrderFormDefinition) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("encode form draft: %w", err)
	}
	var template *store.WorkOrderTemplate
	err = s.db.Transaction(func(tx *gorm.DB) error {
		current, err := s.findTemplate(tx, scope, templateID)
		if err != nil {
			return err
		}
		if current.SystemManaged {
			return fmt.Errorf("system-managed work order template cannot save a custom form draft")
		}
		current.FormDraftDefinition = string(encoded)
		if err := tx.Save(current).Error; err != nil {
			return fmt.Errorf("save form draft: %w", err)
		}
		template = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return template, nil
}

func (s *WorkOrderService) SaveWorkflowDraft(scope WorkOrderScope, templateID uint, definition WorkOrderWorkflowDefinition) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	definition = normalizeWorkOrderWorkflowDefinition(definition)
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("encode workflow draft: %w", err)
	}
	var template *store.WorkOrderTemplate
	err = s.db.Transaction(func(tx *gorm.DB) error {
		current, err := s.findTemplate(tx, scope, templateID)
		if err != nil {
			return err
		}
		if current.SystemManaged && current.Code != DeviceMaintenanceWorkOrderTemplateCode {
			return fmt.Errorf("system-managed work order template cannot save a custom workflow draft")
		}
		current.WorkflowDraftDefinition = string(encoded)
		if err := tx.Save(current).Error; err != nil {
			return fmt.Errorf("save workflow draft: %w", err)
		}
		template = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return template, nil
}

func clearUnpublishedAIDeviceMaintenanceWorkflowVersions(tx *gorm.DB, scope WorkOrderScope, template *store.WorkOrderTemplate) error {
	if template.Code != DeviceMaintenanceWorkOrderTemplateCode || template.CurrentWorkflowVersion != 0 {
		return nil
	}
	if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ? AND template_id = ?", scope.TenantID, scope.ProjectID, template.ID).
		Delete(&store.WorkOrderWorkflowVersion{}).Error; err != nil {
		return fmt.Errorf("delete unpublished AI device maintenance workflow versions: %w", err)
	}
	return nil
}

// EnsureDeviceMaintenanceTemplate creates the scoped baseline used by the AI
// suggestion adapter exactly once. Administrators can publish later versions
// through the normal template APIs; existing versions are never overwritten.
func (s *WorkOrderService) EnsureDeviceMaintenanceTemplate(scope WorkOrderScope) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var template store.WorkOrderTemplate
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("tenant_id = ? AND project_id = ? AND code = ?", scope.TenantID, scope.ProjectID, DeviceMaintenanceWorkOrderTemplateCode).
			First(&template).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			template = store.WorkOrderTemplate{
				TenantID:      scope.TenantID,
				ProjectID:     scope.ProjectID,
				Code:          DeviceMaintenanceWorkOrderTemplateCode,
				Name:          "AI大脑建议",
				Description:   "AI 大脑建议生成工单的内置表单；请配置并发布流转流程后使用。",
				Enabled:       true,
				SystemManaged: true,
			}
			if err := tx.Create(&template).Error; err != nil {
				return fmt.Errorf("create AI device maintenance template: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("load AI device maintenance template: %w", err)
		}
		legacyTemplateName := template.Name == "AI 设备维护工单"
		if !template.SystemManaged {
			return fmt.Errorf("AI device maintenance template must be system-managed")
		}
		if !template.Enabled {
			return fmt.Errorf("AI device maintenance template is disabled")
		}
		template.Name = "AI大脑建议"
		template.Description = "AI 大脑建议生成工单的内置表单；请配置并发布流转流程后使用。"
		if template.CurrentFormVersion == 0 {
			definition := defaultDeviceMaintenanceFormDefinition()
			if err := ValidateWorkOrderFormDefinition(definition); err != nil {
				return err
			}
			encoded, err := json.Marshal(definition)
			if err != nil {
				return fmt.Errorf("encode AI device maintenance form: %w", err)
			}
			compiledForm, err := compileLegacyWorkOrderForm(definition)
			if err != nil {
				return err
			}
			version := store.WorkOrderFormVersion{
				TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: template.ID,
				Version: 1, Definition: string(encoded), Checksum: compiledForm.Checksum,
				SchemaJSON: string(compiledSnapshot(compiledForm, "schema")), UISchemaJSON: string(compiledSnapshot(compiledForm, "ui_schema")),
				DefaultsJSON: string(compiledSnapshot(compiledForm, "defaults")), RequiredJSON: string(compiledSnapshot(compiledForm, "required")),
				FieldOrderJSON: string(compiledSnapshot(compiledForm, "field_order")), ResourceFieldsJSON: string(compiledSnapshot(compiledForm, "resource_fields")), PublishedBy: scope.ActorUserID,
			}
			if err := tx.Create(&version).Error; err != nil {
				return fmt.Errorf("create AI device maintenance form version: %w", err)
			}
			template.CurrentFormVersion = version.Version
		}
		defaultWorkflow := defaultDeviceMaintenanceWorkflowDefinition()
		if err := ValidateWorkOrderWorkflowDefinition(defaultWorkflow); err != nil {
			return err
		}
		encodedWorkflow, err := json.Marshal(defaultWorkflow)
		if err != nil {
			return fmt.Errorf("encode AI device maintenance workflow: %w", err)
		}
		if template.CurrentWorkflowVersion == 0 {
			if strings.TrimSpace(template.WorkflowDraftDefinition) == "" {
				template.WorkflowDraftDefinition = string(encodedWorkflow)
			}
		} else if legacyTemplateName && strings.TrimSpace(template.WorkflowDraftDefinition) == "" {
			var currentWorkflow store.WorkOrderWorkflowVersion
			if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, template.ID, template.CurrentWorkflowVersion).First(&currentWorkflow).Error; err != nil {
				return fmt.Errorf("load AI device maintenance workflow: %w", err)
			}
			if currentWorkflow.Definition == string(encodedWorkflow) {
				template.CurrentWorkflowVersion = 0
				template.WorkflowDraftDefinition = string(encodedWorkflow)
			}
		}
		if err := clearUnpublishedAIDeviceMaintenanceWorkflowVersions(tx, scope, &template); err != nil {
			return err
		}
		return tx.Save(&template).Error
	})
	if err != nil {
		return nil, err
	}
	return &template, nil
}

func (s *WorkOrderService) PublishFormVersion(scope WorkOrderScope, templateID uint, definition WorkOrderFormDefinition) (*store.WorkOrderFormVersion, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if err := ValidateWorkOrderFormDefinition(definition); err != nil {
		return nil, err
	}
	compiled, err := compileLegacyWorkOrderForm(definition)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("encode form definition: %w", err)
	}
	var version store.WorkOrderFormVersion
	err = s.db.Transaction(func(tx *gorm.DB) error {
		template, err := s.findTemplate(tx, scope, templateID)
		if err != nil {
			return err
		}
		if template.SystemManaged {
			return fmt.Errorf("system-managed work order template cannot publish a custom form")
		}
		if template.CurrentFormVersion > 0 {
			var current store.WorkOrderFormVersion
			if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, template.ID, template.CurrentFormVersion).First(&current).Error; err == nil && current.Definition == string(encoded) {
				version = current
				template.FormDraftDefinition = ""
				return tx.Save(template).Error
			}
		}
		version = store.WorkOrderFormVersion{
			TenantID:           scope.TenantID,
			ProjectID:          scope.ProjectID,
			TemplateID:         template.ID,
			Version:            template.CurrentFormVersion + 1,
			Definition:         string(encoded),
			SchemaJSON:         string(compiledSnapshot(compiled, "schema")),
			UISchemaJSON:       string(compiledSnapshot(compiled, "ui_schema")),
			DefaultsJSON:       string(compiledSnapshot(compiled, "defaults")),
			RequiredJSON:       string(compiledSnapshot(compiled, "required")),
			FieldOrderJSON:     string(compiledSnapshot(compiled, "field_order")),
			ResourceFieldsJSON: string(compiledSnapshot(compiled, "resource_fields")),
			Checksum:           compiled.Checksum,
			PublishedBy:        scope.ActorUserID,
		}
		if err := tx.Create(&version).Error; err != nil {
			return fmt.Errorf("create form version: %w", err)
		}
		template.CurrentFormVersion = version.Version
		template.FormDraftDefinition = ""
		return tx.Save(template).Error
	})
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func (s *WorkOrderService) PublishWorkflowVersion(scope WorkOrderScope, templateID uint, definition WorkOrderWorkflowDefinition) (*store.WorkOrderWorkflowVersion, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	definition = normalizeWorkOrderWorkflowDefinition(definition)
	if err := ValidateWorkOrderWorkflowDefinition(definition); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return nil, fmt.Errorf("encode workflow definition: %w", err)
	}
	var version store.WorkOrderWorkflowVersion
	err = s.db.Transaction(func(tx *gorm.DB) error {
		template, err := s.findTemplate(tx, scope, templateID)
		if err != nil {
			return err
		}
		if template.SystemManaged && template.Code != DeviceMaintenanceWorkOrderTemplateCode {
			return fmt.Errorf("system-managed work order template cannot publish a custom workflow")
		}
		if err := clearUnpublishedAIDeviceMaintenanceWorkflowVersions(tx, scope, template); err != nil {
			return err
		}
		if template.CurrentWorkflowVersion > 0 {
			var current store.WorkOrderWorkflowVersion
			if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?", scope.TenantID, scope.ProjectID, template.ID, template.CurrentWorkflowVersion).First(&current).Error; err != nil {
				return fmt.Errorf("load current workflow version: %w", err)
			}
			if current.Definition == string(encoded) {
				version = current
				template.WorkflowDraftDefinition = ""
				return tx.Save(template).Error
			}
		}
		if err := s.validateWorkflowParticipants(tx, scope, definition); err != nil {
			return err
		}
		version = store.WorkOrderWorkflowVersion{
			TenantID:    scope.TenantID,
			ProjectID:   scope.ProjectID,
			TemplateID:  template.ID,
			Version:     template.CurrentWorkflowVersion + 1,
			Definition:  string(encoded),
			PublishedBy: scope.ActorUserID,
		}
		if err := tx.Create(&version).Error; err != nil {
			return fmt.Errorf("create workflow version: %w", err)
		}
		template.CurrentWorkflowVersion = version.Version
		template.WorkflowDraftDefinition = ""
		return tx.Save(template).Error
	})
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func (s *WorkOrderService) validateWorkflowParticipants(db *gorm.DB, scope WorkOrderScope, definition WorkOrderWorkflowDefinition) error {
	if definition.SchemaVersion != WorkOrderWorkflowSchemaV2 {
		return nil
	}
	participantIDs := make(map[uint]bool)
	for _, node := range definition.Nodes {
		if node.Type != WorkOrderWorkflowNodeUserTask {
			continue
		}
		for _, participantID := range node.ParticipantUserIDs {
			participantIDs[participantID] = true
		}
	}
	if len(participantIDs) == 0 {
		return nil
	}
	ids := make([]uint, 0, len(participantIDs))
	for participantID := range participantIDs {
		ids = append(ids, participantID)
	}
	var users []store.User
	if err := db.Model(&store.User{}).
		Joins("JOIN user_role_bindings AS participant_bindings ON participant_bindings.user_id = users.id AND participant_bindings.tenant_id = users.tenant_id").
		Where("users.tenant_id = ? AND users.status = ? AND users.id IN ? AND (participant_bindings.project_id = ? OR participant_bindings.project_id = 0)", scope.TenantID, 1, ids, scope.ProjectID).
		Select("users.*").Distinct().Find(&users).Error; err != nil {
		return fmt.Errorf("load workflow participants: %w", err)
	}
	active := make(map[uint]bool, len(users))
	for _, user := range users {
		active[user.ID] = true
	}
	for _, participantID := range ids {
		if !active[participantID] {
			return fmt.Errorf("workflow participant %d is not an active member of this project", participantID)
		}
	}
	return nil
}

func (s *WorkOrderService) UpdateWorkOrderTemplate(scope WorkOrderScope, templateID uint, name string) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("template name cannot be empty")
	}
	var template store.WorkOrderTemplate
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND tenant_id = ? AND project_id = ?", templateID, scope.TenantID, scope.ProjectID).First(&template).Error; err != nil {
			return err
		}
		if template.SystemManaged {
			return fmt.Errorf("system-managed work order template cannot be renamed")
		}
		template.Name = name
		return tx.Save(&template).Error
	})
	return &template, err
}

func (s *WorkOrderService) DeleteWorkOrderTemplate(scope WorkOrderScope, templateID uint) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		var template store.WorkOrderTemplate
		if err := tx.Where("id = ? AND tenant_id = ? AND project_id = ?", templateID, scope.TenantID, scope.ProjectID).First(&template).Error; err != nil {
			return err
		}
		if template.SystemManaged {
			return fmt.Errorf("system-managed work order template cannot be deleted")
		}
		// Soft delete template
		return tx.Delete(&template).Error
	})
}

func (s *WorkOrderService) UpdateWorkOrder(scope WorkOrderScope, workOrderID uint, title, summary, priority string) (*store.WorkOrder, error) {
	return s.updateWorkOrder(scope, workOrderID, title, summary, priority, nil)
}

func (s *WorkOrderService) UpdateWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, title, summary, priority string, expectedVersion *int) (*store.WorkOrder, error) {
	return s.updateWorkOrder(scope, workOrderID, title, summary, priority, expectedVersion)
}

func (s *WorkOrderService) updateWorkOrder(scope WorkOrderScope, workOrderID uint, title, summary, priority string, expectedVersion *int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, fmt.Errorf("work order title cannot be empty")
	}
	var order store.WorkOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ? AND tenant_id = ? AND project_id = ?", workOrderID, scope.TenantID, scope.ProjectID).First(&order).Error; err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		status, ok := workflow.statusByKey(order.Status)
		if !ok || status.Category != WorkOrderStatusOpen {
			return fmt.Errorf("only unprocessed work orders can be edited")
		}
		previousVersion := order.Version
		if previousVersion <= 0 {
			previousVersion = 1
		}
		if expectedVersion != nil && *expectedVersion != previousVersion {
			return ErrWorkOrderVersionConflict
		}
		order.Title = title
		order.Summary = strings.TrimSpace(summary)
		if priority = strings.TrimSpace(priority); priority != "" {
			order.Priority = normalizeWorkOrderPriority(priority)
		}
		updates := map[string]any{
			"title":    order.Title,
			"summary":  order.Summary,
			"priority": order.Priority,
			"version":  previousVersion + 1,
		}
		query := tx.Model(&store.WorkOrder{}).
			Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, order.TenantID, order.ProjectID, previousVersion).
			Updates(updates)
		if query.Error != nil {
			return fmt.Errorf("update work order: %w", query.Error)
		}
		if query.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		order.Version = previousVersion + 1
		return s.appendWorkOrderEvent(tx, &order, "updated", scope.ActorUserID, map[string]any{
			"title":    order.Title,
			"summary":  order.Summary,
			"priority": order.Priority,
		})
	})
	return &order, err
}

func (s *WorkOrderService) DeleteWorkOrder(scope WorkOrderScope, workOrderID uint) error {
	return s.deleteWorkOrder(scope, workOrderID, nil)
}

func (s *WorkOrderService) DeleteWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, expectedVersion *int) error {
	return s.deleteWorkOrder(scope, workOrderID, expectedVersion)
}

func (s *WorkOrderService) deleteWorkOrder(scope WorkOrderScope, workOrderID uint, expectedVersion *int) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		status, ok := workflow.statusByKey(order.Status)
		if !ok || status.Category != WorkOrderStatusOpen {
			return fmt.Errorf("only unprocessed work orders can be deleted")
		}
		previousVersion := order.Version
		if previousVersion <= 0 {
			previousVersion = 1
		}
		if expectedVersion != nil && *expectedVersion != previousVersion {
			return ErrWorkOrderVersionConflict
		}
		sourceUpdate := tx.Model(&store.WorkOrder{}).
			Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, scope.TenantID, scope.ProjectID, previousVersion).
			Updates(map[string]any{
				"source_id":       "deleted:" + order.PublicID,
				"idempotency_key": "deleted:" + order.PublicID,
			})
		if sourceUpdate.Error != nil {
			return fmt.Errorf("release work order source idempotency: %w", sourceUpdate.Error)
		}
		if sourceUpdate.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		for name, model := range map[string]any{
			"resource references": &store.WorkOrderResourceReference{},
			"approval tasks":      &store.WorkOrderApprovalTask{},
			"node instances":      &store.WorkOrderNodeInstance{},
			"workflow tasks":      &store.WorkOrderTask{},
			"notifications":       &store.WorkOrderNotification{},
			"links":               &store.WorkOrderLink{},
			"events":              &store.WorkOrderEvent{},
			"outbox events":       &store.WorkOrderOutboxEvent{},
		} {
			if err := tx.Where("tenant_id = ? AND project_id = ? AND work_order_id = ?", scope.TenantID, scope.ProjectID, order.ID).Delete(model).Error; err != nil {
				return fmt.Errorf("delete work order %s: %w", name, err)
			}
		}
		if err := tx.Unscoped().Where("tenant_id = ? AND project_id = ? AND response_json LIKE ?", scope.TenantID, scope.ProjectID, "%"+order.PublicID+"%").Delete(&store.WorkOrderCommandReceipt{}).Error; err != nil {
			return fmt.Errorf("delete work order command receipts: %w", err)
		}
		if err := tx.Model(&store.AlarmInstance{}).Where("tenant_id = ? AND project_id = ? AND work_order_public_id = ?", scope.TenantID, scope.ProjectID, order.PublicID).Update("work_order_public_id", "").Error; err != nil {
			return fmt.Errorf("clear alarm work order binding: %w", err)
		}
		deleteResult := tx.Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, scope.TenantID, scope.ProjectID, previousVersion).Delete(&store.WorkOrder{})
		if deleteResult.Error != nil {
			return fmt.Errorf("delete work order: %w", deleteResult.Error)
		}
		if deleteResult.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		return nil
	})
}

func (s *WorkOrderService) CreateWorkOrder(input WorkOrderCreateInput) (*store.WorkOrder, bool, error) {
	if err := validateWorkOrderScope(input.Scope); err != nil {
		return nil, false, err
	}
	input.Title = strings.TrimSpace(input.Title)
	if input.TemplateID == 0 || input.Title == "" || len([]rune(input.Title)) > 256 {
		return nil, false, fmt.Errorf("template and a 1-256 character title are required")
	}
	source, err := normalizeWorkOrderSource(input.Source)
	if err != nil {
		return nil, false, err
	}
	priority := normalizeWorkOrderPriority(input.Priority)
	sourceSnapshot := []byte("{}")
	if source.Snapshot != nil {
		var err error
		sourceSnapshot, err = json.Marshal(source.Snapshot)
		if err != nil {
			return nil, false, fmt.Errorf("encode work order source snapshot: %w", err)
		}
	}
	var result store.WorkOrder
	created := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var existing store.WorkOrder
		err := tx.Where("tenant_id = ? AND project_id = ? AND source_type = ? AND source_id = ? AND idempotency_key = ?",
			input.Scope.TenantID, input.Scope.ProjectID, source.Type, source.ID, source.IdempotencyKey).First(&existing).Error
		if err == nil {
			result = existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("lookup idempotent work order: %w", err)
		}

		template, err := s.findTemplate(tx, input.Scope, input.TemplateID)
		if err != nil {
			return err
		}
		if !template.Enabled || template.CurrentFormVersion == 0 || template.CurrentWorkflowVersion == 0 {
			return fmt.Errorf("work order template must have enabled form and workflow versions")
		}
		var formVersion store.WorkOrderFormVersion
		if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?",
			input.Scope.TenantID, input.Scope.ProjectID, template.ID, template.CurrentFormVersion).First(&formVersion).Error; err != nil {
			return fmt.Errorf("load published form version: %w", err)
		}
		var workflowVersion store.WorkOrderWorkflowVersion
		if err := tx.Where("tenant_id = ? AND project_id = ? AND template_id = ? AND version = ?",
			input.Scope.TenantID, input.Scope.ProjectID, template.ID, template.CurrentWorkflowVersion).First(&workflowVersion).Error; err != nil {
			return fmt.Errorf("load published workflow version: %w", err)
		}
		formDefinition, err := decodeWorkOrderFormDefinition(formVersion.Definition)
		if err != nil {
			return err
		}
		compiledForm, err := compileLegacyWorkOrderForm(formDefinition)
		if err != nil {
			return err
		}
		workflowDefinition, err := decodeWorkOrderWorkflowDefinition(workflowVersion.Definition)
		if err != nil {
			return err
		}
		formData, err := ValidateAndNormalizeWorkOrderFormData(formDefinition, input.FormData)
		if err != nil {
			return err
		}
		encodedData, err := json.Marshal(formData)
		if err != nil {
			return fmt.Errorf("encode work order form data: %w", err)
		}
		resourceRefs := workorderport.ExtractResourceReferences(compiledForm, formData)
		if len(resourceRefs) > 0 {
			validator := workorderport.NewResourceValidator(workorderport.NewGormResourceResolver(tx), workorderport.StoreAuditWriter{DB: tx, UserID: input.Scope.ActorUserID})
			if err := validator.Validate(context.Background(), workorderport.Scope{TenantID: input.Scope.TenantID, ProjectID: input.Scope.ProjectID}, resourceRefs); err != nil {
				return err
			}
		}
		initialStatus := workflowDefinition.InitialStatus
		workflowCycle := 0
		if isGraphWorkOrderWorkflow(workflowDefinition) {
			initialStatus = WorkOrderStatusOpen
			workflowCycle = 1
		}
		result = store.WorkOrder{
			PublicID:               uuid.NewString(),
			Version:                1,
			TenantID:               input.Scope.TenantID,
			ProjectID:              input.Scope.ProjectID,
			Code:                   nextWorkOrderCode(),
			TemplateID:             template.ID,
			FormVersion:            formVersion.Version,
			FormSchemaVersionID:    formVersion.PublicID,
			FormSchemaChecksum:     compiledForm.Checksum,
			FormSnapshotJSON:       string(compiledForm.SnapshotJSON),
			WorkflowVersion:        workflowVersion.Version,
			Title:                  input.Title,
			Summary:                strings.TrimSpace(input.Summary),
			Priority:               priority,
			Status:                 initialStatus,
			WorkflowCycle:          workflowCycle,
			CreatedBy:              input.Scope.ActorUserID,
			SourceType:             source.Type,
			SourceID:               source.ID,
			IdempotencyKey:         source.IdempotencyKey,
			SourceSnapshot:         string(sourceSnapshot),
			FormData:               string(encodedData),
			FormDefinitionSnapshot: formVersion.Definition,
			WorkflowSnapshot:       workflowVersion.Definition,
		}
		if err := tx.Create(&result).Error; err != nil {
			if isUniqueConstraintError(err) {
				if findErr := tx.Where("tenant_id = ? AND project_id = ? AND source_type = ? AND source_id = ? AND idempotency_key = ?",
					input.Scope.TenantID, input.Scope.ProjectID, source.Type, source.ID, source.IdempotencyKey).First(&result).Error; findErr == nil {
					return nil
				}
			}
			return fmt.Errorf("create work order: %w", err)
		}
		if len(resourceRefs) > 0 {
			resolver := workorderport.NewGormResourceResolver(tx)
			for _, ref := range resourceRefs {
				resolved, resolveErr := resolver.Resolve(context.Background(), workorderport.Scope{TenantID: input.Scope.TenantID, ProjectID: input.Scope.ProjectID}, ref)
				if resolveErr != nil {
					return resolveErr
				}
				snapshot, marshalErr := json.Marshal(resolved)
				if marshalErr != nil {
					return marshalErr
				}
				if err := tx.Create(&store.WorkOrderResourceReference{TenantID: result.TenantID, ProjectID: result.ProjectID, WorkOrderID: result.ID, FieldKey: ref.Field, ResourceType: ref.Type, ResourceRef: ref.ID, ResourceSnapshot: string(snapshot)}).Error; err != nil {
					return fmt.Errorf("create work order resource reference: %w", err)
				}
			}
		}
		created = true
		if err := s.appendWorkOrderEvent(tx, &result, "created", input.Scope.ActorUserID, map[string]any{
			"source_type": source.Type,
			"source_id":   source.ID,
			"status":      result.Status,
		}); err != nil {
			return err
		}
		if err := s.upsertSourceLink(tx, &result, source); err != nil {
			return err
		}
		if isGraphWorkOrderWorkflow(workflowDefinition) {
			return s.startGraphWorkOrderWorkflow(tx, &result, workflowDefinition, input.Scope.ActorUserID)
		}
		return s.createPendingApprovalTasks(tx, &result, workflowDefinition, input.Scope.ActorUserID)
	})
	if err != nil {
		return nil, false, err
	}
	return &result, created, nil
}

func (s *WorkOrderService) GetWorkOrder(scope WorkOrderScope, workOrderID uint) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	return s.findWorkOrder(s.db, scope, workOrderID)
}

func (s *WorkOrderService) GetWorkOrderByPublicID(scope WorkOrderScope, publicID string) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	return s.findWorkOrderByPublicID(s.db, scope, publicID)
}

func (s *WorkOrderService) ListWorkOrders(scope WorkOrderScope, options WorkOrderListOptions) ([]store.WorkOrder, int64, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, 0, err
	}
	page := options.Page
	if page < 1 {
		page = 1
	}
	pageSize := options.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := s.db.Model(&store.WorkOrder{}).Where("work_orders.tenant_id = ? AND work_orders.project_id = ?", scope.TenantID, scope.ProjectID)
	if relation := strings.TrimSpace(options.Relation); relation != "" {
		if scope.ActorUserID == 0 {
			return nil, 0, fmt.Errorf("work order relation view requires a user identity")
		}
		actorUserID := scope.ActorUserID
		switch relation {
		case "created":
			query = query.Where("work_orders.created_by = ?", actorUserID)
		case "assigned":
			query = query.Where("work_orders.assignee_user_id = ?", actorUserID)
		case "pending":
			query = query.Where(`(work_orders.assignee_user_id = ? AND work_orders.closed_at IS NULL) OR EXISTS (
				SELECT 1 FROM work_order_tasks AS pending_handle_tasks
				WHERE pending_handle_tasks.work_order_id = work_orders.id
				AND pending_handle_tasks.tenant_id = work_orders.tenant_id
				AND pending_handle_tasks.project_id = work_orders.project_id
				AND pending_handle_tasks.assignee_user_id = ?
				AND pending_handle_tasks.task_kind = ?
				AND pending_handle_tasks.status = ?
				AND pending_handle_tasks.deleted_at IS NULL
			)`, actorUserID, actorUserID, WorkOrderTaskKindHandle, WorkOrderTaskPending)
		case "pending_approval":
			query = query.Where(`EXISTS (
				SELECT 1 FROM work_order_approval_tasks AS pending_approval_tasks
				WHERE pending_approval_tasks.work_order_id = work_orders.id
				AND pending_approval_tasks.tenant_id = work_orders.tenant_id
				AND pending_approval_tasks.project_id = work_orders.project_id
				AND pending_approval_tasks.approver_user_id = ?
				AND pending_approval_tasks.status = ?
				AND pending_approval_tasks.deleted_at IS NULL
			) OR EXISTS (
				SELECT 1 FROM work_order_tasks AS pending_graph_approval_tasks
				WHERE pending_graph_approval_tasks.work_order_id = work_orders.id
				AND pending_graph_approval_tasks.tenant_id = work_orders.tenant_id
				AND pending_graph_approval_tasks.project_id = work_orders.project_id
				AND pending_graph_approval_tasks.assignee_user_id = ?
				AND pending_graph_approval_tasks.task_kind = ?
				AND pending_graph_approval_tasks.status = ?
				AND pending_graph_approval_tasks.deleted_at IS NULL
			)`, actorUserID, WorkOrderApprovalPending, actorUserID, WorkOrderTaskKindApprove, WorkOrderTaskPending)
		case "cc":
			query = query.Where(`EXISTS (
				SELECT 1 FROM work_order_notifications AS cc_notifications
				WHERE cc_notifications.work_order_id = work_orders.id
				AND cc_notifications.tenant_id = work_orders.tenant_id
				AND cc_notifications.project_id = work_orders.project_id
				AND cc_notifications.recipient_user_id = ?
				AND cc_notifications.task_kind = 'cc'
				AND cc_notifications.deleted_at IS NULL
			)`, actorUserID)
		case "processed":
			query = query.Where(`EXISTS (
				SELECT 1 FROM work_order_tasks AS processed_tasks
				WHERE processed_tasks.work_order_id = work_orders.id
				AND processed_tasks.tenant_id = work_orders.tenant_id
				AND processed_tasks.project_id = work_orders.project_id
				AND processed_tasks.assignee_user_id = ?
				AND processed_tasks.status = ?
				AND processed_tasks.deleted_at IS NULL
			) OR EXISTS (
				SELECT 1 FROM work_order_approval_tasks AS processed_approvals
				WHERE processed_approvals.work_order_id = work_orders.id
				AND processed_approvals.tenant_id = work_orders.tenant_id
				AND processed_approvals.project_id = work_orders.project_id
				AND processed_approvals.approver_user_id = ?
				AND processed_approvals.status IN ?
				AND processed_approvals.deleted_at IS NULL
			) OR EXISTS (
				SELECT 1 FROM work_order_events AS processed_events
				WHERE processed_events.work_order_id = work_orders.id
				AND processed_events.tenant_id = work_orders.tenant_id
				AND processed_events.project_id = work_orders.project_id
				AND processed_events.actor_user_id = ?
				AND processed_events.type IN ?
				AND processed_events.deleted_at IS NULL
			)`, actorUserID, WorkOrderTaskCompleted, actorUserID, []string{WorkOrderApprovalApproved, WorkOrderApprovalRejected}, actorUserID, []string{"transitioned", "approval.approved", "approval.rejected", "approval.completed", "workflow.task.completed", "workflow.task.rejected"})
		case "participated":
			query = query.Where(`work_orders.created_by = ? OR work_orders.assignee_user_id = ? OR EXISTS (
				SELECT 1 FROM work_order_tasks AS participant_tasks
				WHERE participant_tasks.work_order_id = work_orders.id
				AND participant_tasks.tenant_id = work_orders.tenant_id
				AND participant_tasks.project_id = work_orders.project_id
				AND participant_tasks.assignee_user_id = ?
				AND participant_tasks.deleted_at IS NULL
			) OR EXISTS (
				SELECT 1 FROM work_order_approval_tasks AS participant_approvals
				WHERE participant_approvals.work_order_id = work_orders.id
				AND participant_approvals.tenant_id = work_orders.tenant_id
				AND participant_approvals.project_id = work_orders.project_id
				AND participant_approvals.approver_user_id = ?
				AND participant_approvals.deleted_at IS NULL
			) OR EXISTS (
				SELECT 1 FROM work_order_notifications AS participant_notifications
				WHERE participant_notifications.work_order_id = work_orders.id
				AND participant_notifications.tenant_id = work_orders.tenant_id
				AND participant_notifications.project_id = work_orders.project_id
				AND participant_notifications.recipient_user_id = ?
				AND participant_notifications.task_kind = 'cc'
				AND participant_notifications.deleted_at IS NULL
			) OR EXISTS (
				SELECT 1 FROM work_order_events AS participant_events
				WHERE participant_events.work_order_id = work_orders.id
				AND participant_events.tenant_id = work_orders.tenant_id
				AND participant_events.project_id = work_orders.project_id
				AND participant_events.actor_user_id = ?
				AND participant_events.deleted_at IS NULL
			)`, actorUserID, actorUserID, actorUserID, actorUserID, actorUserID, actorUserID)
		default:
			return nil, 0, fmt.Errorf("unsupported work order relation view %q", relation)
		}
	}
	if relationType, externalRef := strings.TrimSpace(options.LinkRelationType), strings.TrimSpace(options.LinkExternalRef); relationType != "" || externalRef != "" {
		if relationType == "" || externalRef == "" {
			return nil, 0, fmt.Errorf("work order link relation type and external reference must be provided together")
		}
		query = query.Joins("JOIN work_order_links AS filter_links ON filter_links.work_order_id = work_orders.id AND filter_links.tenant_id = work_orders.tenant_id AND filter_links.project_id = work_orders.project_id").
			Where("filter_links.relation_type = ? AND filter_links.external_ref = ?", relationType, externalRef)
	}
	if len(options.Statuses) > 0 {
		statuses := make([]string, 0, len(options.Statuses))
		for _, status := range options.Statuses {
			status = strings.TrimSpace(status)
			if status != "" {
				statuses = append(statuses, status)
			}
		}
		if len(statuses) > 0 {
			query = query.Where("work_orders.status IN ?", statuses)
		}
	} else if status := strings.TrimSpace(options.Status); status != "" {
		switch status {
		case WorkOrderStatusClosed:
			// Graph workflows completed before the archive-state correction were
			// persisted as resolved with closed_at set. Keep those records visible
			// in the archived filter without rewriting historical audit data.
			query = query.Where("(work_orders.status = ? OR (work_orders.status = ? AND work_orders.closed_at IS NOT NULL))", WorkOrderStatusClosed, WorkOrderStatusResolved)
		case WorkOrderStatusResolved:
			query = query.Where("work_orders.status = ? AND work_orders.closed_at IS NULL", WorkOrderStatusResolved)
		default:
			query = query.Where("work_orders.status = ?", status)
		}
	}
	if sourceType := strings.TrimSpace(options.SourceType); sourceType != "" {
		query = query.Where("work_orders.source_type = ?", sourceType)
	}
	if options.TemplateID > 0 {
		query = query.Where("work_orders.template_id = ?", options.TemplateID)
	}
	if options.AssigneeID > 0 {
		query = query.Where("work_orders.assignee_user_id = ?", options.AssigneeID)
	}
	if search := strings.TrimSpace(options.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(work_orders.code) LIKE ? OR LOWER(work_orders.title) LIKE ? OR LOWER(work_orders.summary) LIKE ?", pattern, pattern, pattern)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count work orders: %w", err)
	}
	var orders []store.WorkOrder
	if err := query.Order("work_orders.created_at DESC, work_orders.id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("list work orders: %w", err)
	}
	return orders, total, nil
}

func (s *WorkOrderService) TransitionWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderTransitionInput) (*store.WorkOrder, error) {
	return s.transitionWorkOrder(scope, workOrderID, input, nil)
}

func (s *WorkOrderService) TransitionWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, input WorkOrderTransitionInput, expectedVersion *int) (*store.WorkOrder, error) {
	return s.transitionWorkOrder(scope, workOrderID, input, expectedVersion)
}

func (s *WorkOrderService) transitionWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderTransitionInput, expectedVersion *int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	input.Key = strings.TrimSpace(input.Key)
	if input.Key == "" {
		return nil, fmt.Errorf("transition key is required")
	}
	opinion, err := requiredProcessingOpinion(input.Comment)
	if err != nil {
		return nil, err
	}
	input.Comment = opinion
	var result store.WorkOrder
	err = s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		transition, ok := workflow.transitionByKey(input.Key)
		if !ok || transition.From != order.Status {
			return fmt.Errorf("transition %q is not allowed from status %q", input.Key, order.Status)
		}
		if len(transition.ApproverUserIDs) > 0 {
			return fmt.Errorf("transition %q requires approval", input.Key)
		}
		var pendingApprovals int64
		if err := tx.Model(&store.WorkOrderApprovalTask{}).
			Where("work_order_id = ? AND workflow_cycle = ? AND status = ?", order.ID, order.WorkflowCycle, WorkOrderApprovalPending).
			Count(&pendingApprovals).Error; err != nil {
			return fmt.Errorf("count pending approval tasks: %w", err)
		}
		if pendingApprovals > 0 {
			return ErrWorkOrderApprovalPending
		}
		payload := map[string]any{
			"transition_key": transition.Key,
			"comment":        strings.TrimSpace(input.Comment),
		}
		if transition.RequireResolution {
			resolution, err := normalizeWorkOrderResolution(input.Resolution)
			if err != nil {
				return err
			}
			resolutionJSON, err := json.Marshal(resolution)
			if err != nil {
				return fmt.Errorf("encode work order resolution: %w", err)
			}
			payload["resolution"] = resolution
			payload["_resolution_json"] = string(resolutionJSON)
		}
		if err := s.applyWorkOrderStatusWithExpectedVersion(tx, order, workflow, transition.To, "transitioned", scope.ActorUserID, payload, expectedVersion); err != nil {
			return err
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) ApproveWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderApprovalInput) (*store.WorkOrder, error) {
	return s.approveWorkOrder(scope, workOrderID, input, nil)
}

func (s *WorkOrderService) ApproveWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, input WorkOrderApprovalInput, expectedVersion *int) (*store.WorkOrder, error) {
	return s.approveWorkOrder(scope, workOrderID, input, expectedVersion)
}

func (s *WorkOrderService) approveWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderApprovalInput, expectedVersion *int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("approver identity is required")
	}
	opinion, err := requiredProcessingOpinion(input.Comment)
	if err != nil {
		return nil, err
	}
	input.Comment = opinion
	var result store.WorkOrder
	err = s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		var task store.WorkOrderApprovalTask
		if err := tx.Where("work_order_id = ? AND workflow_cycle = ? AND approver_user_id = ? AND status = ?", order.ID, order.WorkflowCycle, scope.ActorUserID, WorkOrderApprovalPending).
			Order("id ASC").First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("no pending approval task is assigned to the current user")
			}
			return fmt.Errorf("load approval task: %w", err)
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		transition, ok := workflow.transitionByKey(task.TransitionKey)
		if !ok || transition.From != order.Status || len(transition.ApproverUserIDs) == 0 {
			return fmt.Errorf("approval task does not match the current workflow state")
		}
		now := time.Now().UTC()
		task.Status = WorkOrderApprovalApproved
		task.Comment = strings.TrimSpace(input.Comment)
		task.DecidedAt = &now
		if err := tx.Save(&task).Error; err != nil {
			return fmt.Errorf("save approval task: %w", err)
		}
		if err := s.appendWorkOrderEvent(tx, order, "approval.approved", scope.ActorUserID, map[string]any{
			"transition_key": transition.Key,
			"comment":        task.Comment,
		}); err != nil {
			return err
		}
		if transition.ApprovalMode == WorkOrderApprovalAny {
			if err := tx.Model(&store.WorkOrderApprovalTask{}).
				Where("work_order_id = ? AND workflow_cycle = ? AND transition_key = ? AND status = ?", order.ID, order.WorkflowCycle, transition.Key, WorkOrderApprovalPending).
				Updates(map[string]any{"status": WorkOrderApprovalCancelled, "decided_at": now}).Error; err != nil {
				return fmt.Errorf("cancel redundant approval tasks: %w", err)
			}
		} else {
			var pending int64
			if err := tx.Model(&store.WorkOrderApprovalTask{}).
				Where("work_order_id = ? AND workflow_cycle = ? AND transition_key = ? AND status = ?", order.ID, order.WorkflowCycle, transition.Key, WorkOrderApprovalPending).
				Count(&pending).Error; err != nil {
				return fmt.Errorf("count pending approval tasks: %w", err)
			}
			if pending > 0 {
				result = *order
				return nil
			}
		}
		if err := s.applyWorkOrderStatusWithExpectedVersion(tx, order, workflow, transition.To, "approval.completed", scope.ActorUserID, map[string]any{
			"transition_key": transition.Key,
		}, expectedVersion); err != nil {
			return err
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) RejectWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderApprovalInput) (*store.WorkOrder, error) {
	return s.rejectWorkOrder(scope, workOrderID, input, nil)
}

func (s *WorkOrderService) RejectWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, input WorkOrderApprovalInput, expectedVersion *int) (*store.WorkOrder, error) {
	return s.rejectWorkOrder(scope, workOrderID, input, expectedVersion)
}

func (s *WorkOrderService) rejectWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderApprovalInput, expectedVersion *int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("approver identity is required")
	}
	opinion, err := requiredProcessingOpinion(input.Comment)
	if err != nil {
		return nil, err
	}
	input.Comment = opinion
	var result store.WorkOrder
	err = s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		var task store.WorkOrderApprovalTask
		if err := tx.Where("work_order_id = ? AND workflow_cycle = ? AND approver_user_id = ? AND status = ?", order.ID, order.WorkflowCycle, scope.ActorUserID, WorkOrderApprovalPending).
			Order("id ASC").First(&task).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return fmt.Errorf("no pending approval task is assigned to the current user")
			}
			return fmt.Errorf("load approval task: %w", err)
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		transition, ok := workflow.transitionByKey(task.TransitionKey)
		if !ok || transition.From != order.Status || transition.RejectTo == "" {
			return fmt.Errorf("approval task cannot be rejected in the current workflow state")
		}
		now := time.Now().UTC()
		task.Status = WorkOrderApprovalRejected
		task.Comment = strings.TrimSpace(input.Comment)
		task.DecidedAt = &now
		if err := tx.Save(&task).Error; err != nil {
			return fmt.Errorf("save approval task: %w", err)
		}
		if err := tx.Model(&store.WorkOrderApprovalTask{}).
			Where("work_order_id = ? AND workflow_cycle = ? AND transition_key = ? AND status = ?", order.ID, order.WorkflowCycle, transition.Key, WorkOrderApprovalPending).
			Updates(map[string]any{"status": WorkOrderApprovalCancelled, "decided_at": now}).Error; err != nil {
			return fmt.Errorf("cancel pending approval tasks: %w", err)
		}
		if err := s.applyWorkOrderStatusWithExpectedVersion(tx, order, workflow, transition.RejectTo, "approval.rejected", scope.ActorUserID, map[string]any{
			"transition_key": transition.Key,
			"comment":        task.Comment,
		}, expectedVersion); err != nil {
			return err
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) ClaimWorkOrder(scope WorkOrderScope, workOrderID uint) (*store.WorkOrder, error) {
	return s.claimWorkOrder(scope, workOrderID, nil)
}

func (s *WorkOrderService) ClaimWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, expectedVersion *int) (*store.WorkOrder, error) {
	return s.claimWorkOrder(scope, workOrderID, expectedVersion)
}

func (s *WorkOrderService) claimWorkOrder(scope WorkOrderScope, workOrderID uint, expectedVersion *int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("claiming user identity is required")
	}
	var result store.WorkOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		status, ok := workflow.statusByKey(order.Status)
		if !ok || (status.Category != WorkOrderStatusOpen && status.Category != WorkOrderStatusInProgress) {
			return fmt.Errorf("work order cannot be claimed in status %q", order.Status)
		}
		if order.AssigneeUserID != 0 && order.AssigneeUserID != scope.ActorUserID {
			return fmt.Errorf("work order is already assigned")
		}
		if order.AssigneeUserID == scope.ActorUserID {
			result = *order
			return nil
		}
		previousVersion := order.Version
		if previousVersion <= 0 {
			previousVersion = 1
		}
		expected := previousVersion
		if expectedVersion != nil {
			expected = *expectedVersion
		}
		if expected <= 0 {
			return ErrWorkOrderVersionConflict
		}
		query := tx.Model(&store.WorkOrder{}).
			Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, order.TenantID, order.ProjectID, expected).
			Updates(map[string]any{"assignee_user_id": scope.ActorUserID, "version": expected + 1})
		if query.Error != nil {
			return fmt.Errorf("claim work order: %w", query.Error)
		}
		if query.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		order.AssigneeUserID = scope.ActorUserID
		order.Version = expected + 1
		if err := s.appendWorkOrderEvent(tx, order, "claimed", scope.ActorUserID, map[string]any{"assignee_user_id": scope.ActorUserID}); err != nil {
			return err
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) TransferWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderTransferInput) (*store.WorkOrder, error) {
	return s.transferWorkOrder(scope, workOrderID, input, nil)
}

func (s *WorkOrderService) TransferWorkOrderWithVersion(scope WorkOrderScope, workOrderID uint, input WorkOrderTransferInput, expectedVersion *int) (*store.WorkOrder, error) {
	return s.transferWorkOrder(scope, workOrderID, input, expectedVersion)
}

func (s *WorkOrderService) transferWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderTransferInput, expectedVersion *int) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("transferring user identity is required")
	}
	if input.TargetUserID == 0 || input.TargetUserID == scope.ActorUserID {
		return nil, fmt.Errorf("transfer target must be another project member")
	}
	comment := strings.TrimSpace(input.Comment)
	if comment == "" {
		return nil, fmt.Errorf("transfer comment is required")
	}

	var result store.WorkOrder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		if !isGraphWorkOrderWorkflow(workflow) {
			status, ok := workflow.statusByKey(order.Status)
			if !ok || (status.Category != WorkOrderStatusOpen && status.Category != WorkOrderStatusInProgress) {
				return fmt.Errorf("work order cannot be transferred in status %q", order.Status)
			}
		} else if order.ClosedAt != nil {
			return fmt.Errorf("work order cannot be transferred after the workflow is closed")
		}
		participants, err := s.loadGraphTaskParticipants(tx, order, []uint{input.TargetUserID})
		if err != nil {
			return err
		}
		target := participants[0]
		targetDisplayName := strings.TrimSpace(target.DisplayName)
		if targetDisplayName == "" {
			targetDisplayName = target.Username
		}
		previousVersion := order.Version
		if previousVersion <= 0 {
			previousVersion = 1
		}
		expected := previousVersion
		if expectedVersion != nil {
			expected = *expectedVersion
		}
		if expected <= 0 {
			return ErrWorkOrderVersionConflict
		}
		advanceVersion := func() error {
			update := tx.Model(&store.WorkOrder{}).
				Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, order.TenantID, order.ProjectID, expected).
				Update("version", expected+1)
			if update.Error != nil {
				return fmt.Errorf("advance work order version for transfer: %w", update.Error)
			}
			if update.RowsAffected != 1 {
				return ErrWorkOrderVersionConflict
			}
			order.Version = expected + 1
			return nil
		}

		if isGraphWorkOrderWorkflow(workflow) {
			taskPublicID := strings.TrimSpace(input.TaskPublicID)
			if taskPublicID == "" {
				return fmt.Errorf("task_public_id is required when transferring a workflow task")
			}
			task, err := s.findWorkOrderTaskByPublicID(tx, scope, taskPublicID)
			if err != nil {
				return err
			}
			if task.WorkOrderID != order.ID || task.Status != WorkOrderTaskPending || task.AssigneeUserID != scope.ActorUserID {
				return fmt.Errorf("workflow task is not pending for the current user")
			}
			node, ok := workflow.graphNode(task.NodeID)
			if !ok || node.Type != WorkOrderWorkflowNodeUserTask || node.TaskKind != WorkOrderTaskKindHandle || task.TaskKind != WorkOrderTaskKindHandle {
				return fmt.Errorf("only pending handling tasks can be transferred")
			}
			if err := advanceVersion(); err != nil {
				return err
			}
			update := tx.Model(&store.WorkOrderTask{}).
				Where("id = ? AND work_order_id = ? AND assignee_user_id = ? AND status = ?", task.ID, order.ID, scope.ActorUserID, WorkOrderTaskPending).
				Updates(map[string]any{"assignee_user_id": target.ID, "assignee_display_name": targetDisplayName})
			if update.Error != nil {
				return fmt.Errorf("transfer workflow task: %w", update.Error)
			}
			if update.RowsAffected != 1 {
				return fmt.Errorf("workflow task is no longer pending for the current user")
			}
			task.AssigneeUserID = target.ID
			task.AssigneeDisplayName = targetDisplayName
			if err := s.appendWorkOrderEvent(tx, order, "workflow.task.transferred", scope.ActorUserID, map[string]any{
				"task_public_id": task.PublicID,
				"node_id":        node.ID,
				"task_kind":      task.TaskKind,
				"from_user_id":   scope.ActorUserID,
				"to_user_id":     target.ID,
				"to_user_name":   targetDisplayName,
				"comment":        comment,
			}); err != nil {
				return err
			}
			if err := s.createWorkOrderTransferNotification(tx, order, target, task, node, comment); err != nil {
				return err
			}
			result = *order
			return nil
		}

		if strings.TrimSpace(input.TaskPublicID) != "" {
			return fmt.Errorf("task_public_id is only supported by graph workflows")
		}
		if order.AssigneeUserID != scope.ActorUserID {
			return fmt.Errorf("work order is not assigned to the current user")
		}
		if err := advanceVersion(); err != nil {
			return err
		}
		update := tx.Model(&store.WorkOrder{}).
			Where("id = ? AND tenant_id = ? AND project_id = ? AND assignee_user_id = ?", order.ID, order.TenantID, order.ProjectID, scope.ActorUserID).
			Update("assignee_user_id", target.ID)
		if update.Error != nil {
			return fmt.Errorf("transfer work order: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("work order is no longer assigned to the current user")
		}
		order.AssigneeUserID = target.ID
		if err := s.appendWorkOrderEvent(tx, order, "transferred", scope.ActorUserID, map[string]any{
			"from_user_id": scope.ActorUserID,
			"to_user_id":   target.ID,
			"to_user_name": targetDisplayName,
			"comment":      comment,
		}); err != nil {
			return err
		}
		if err := s.createWorkOrderTransferNotification(tx, order, target, nil, WorkOrderWorkflowNode{}, comment); err != nil {
			return err
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) createWorkOrderTransferNotification(tx *gorm.DB, order *store.WorkOrder, recipient store.User, task *store.WorkOrderTask, node WorkOrderWorkflowNode, comment string) error {
	notification := store.WorkOrderNotification{
		TenantID:          order.TenantID,
		ProjectID:         order.ProjectID,
		RecipientUserID:   recipient.ID,
		WorkOrderID:       order.ID,
		WorkOrderPublicID: order.PublicID,
		WorkOrderCode:     order.Code,
		WorkOrderTitle:    order.Title,
		TaskKind:          WorkOrderTaskKindHandle,
		Type:              WorkOrderNotificationTask,
		Title:             "Work order transferred to you",
		Content:           comment,
		Status:            WorkOrderNotificationUnread,
	}
	if task != nil {
		notification.WorkOrderTaskID = task.ID
		notification.TaskPublicID = task.PublicID
		notification.NodeID = node.ID
		notification.Content = fmt.Sprintf("%s: %s", node.Name, comment)
	}
	if err := tx.Create(&notification).Error; err != nil {
		return fmt.Errorf("create transfer notification: %w", err)
	}
	return nil
}

func (s *WorkOrderService) AddComment(scope WorkOrderScope, workOrderID uint, comment string) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return fmt.Errorf("comment is required")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		return s.appendWorkOrderEvent(tx, order, "commented", scope.ActorUserID, map[string]any{"comment": comment})
	})
}

func (s *WorkOrderService) CCWorkOrder(scope WorkOrderScope, workOrderID uint, userIDs []uint, comment string) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	recipientIDs := make([]uint, 0, len(userIDs))
	seen := make(map[uint]bool, len(userIDs))
	for _, userID := range userIDs {
		if userID == 0 || userID == scope.ActorUserID || seen[userID] {
			continue
		}
		seen[userID] = true
		recipientIDs = append(recipientIDs, userID)
	}
	if len(recipientIDs) == 0 {
		return fmt.Errorf("cc user list cannot be empty")
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		order, err := s.findWorkOrder(tx, scope, workOrderID)
		if err != nil {
			return err
		}
		participants, err := s.loadGraphTaskParticipants(tx, order, recipientIDs)
		if err != nil {
			return err
		}
		if len(participants) != len(recipientIDs) {
			return fmt.Errorf("cc users must be active project participants")
		}
		if err := s.appendWorkOrderEvent(tx, order, "cc", scope.ActorUserID, map[string]any{"user_ids": recipientIDs, "comment": strings.TrimSpace(comment)}); err != nil {
			return err
		}
		for _, userID := range recipientIDs {
			notification := store.WorkOrderNotification{
				TenantID:          scope.TenantID,
				ProjectID:         scope.ProjectID,
				RecipientUserID:   userID,
				WorkOrderID:       order.ID,
				WorkOrderPublicID: order.PublicID,
				WorkOrderCode:     order.Code,
				WorkOrderTitle:    order.Title,
				Type:              "cc:" + uuid.NewString(),
				TaskKind:          "cc",
				Title:             "工单抄送",
				Content:           strings.TrimSpace(comment),
				Status:            "unread",
			}
			if err := tx.Create(&notification).Error; err != nil {
				return fmt.Errorf("create cc notification: %w", err)
			}
		}
		return nil
	})
}

func (s *WorkOrderService) ListApprovalTasks(scope WorkOrderScope, workOrderID uint) ([]store.WorkOrderApprovalTask, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if _, err := s.findWorkOrder(s.db, scope, workOrderID); err != nil {
		return nil, err
	}
	var tasks []store.WorkOrderApprovalTask
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND work_order_id = ?", scope.TenantID, scope.ProjectID, workOrderID).
		Order("workflow_cycle ASC, id ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list approval tasks: %w", err)
	}
	return tasks, nil
}

func (s *WorkOrderService) ListWorkOrderTasks(scope WorkOrderScope, workOrderID uint) ([]store.WorkOrderTask, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if _, err := s.findWorkOrder(s.db, scope, workOrderID); err != nil {
		return nil, err
	}
	var tasks []store.WorkOrderTask
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND work_order_id = ?", scope.TenantID, scope.ProjectID, workOrderID).
		Order("workflow_cycle ASC, created_at ASC, id ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("list work order tasks: %w", err)
	}
	return tasks, nil
}

func (s *WorkOrderService) ListMyWorkOrderNotifications(scope WorkOrderScope, unreadOnly bool, page, pageSize int) ([]store.WorkOrderNotification, int64, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, 0, err
	}
	if scope.ActorUserID == 0 {
		return nil, 0, fmt.Errorf("notification recipient identity is required")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	query := s.db.Model(&store.WorkOrderNotification{}).Where("tenant_id = ? AND project_id = ? AND recipient_user_id = ?", scope.TenantID, scope.ProjectID, scope.ActorUserID)
	if unreadOnly {
		query = query.Where("status = ?", WorkOrderNotificationUnread)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count work order notifications: %w", err)
	}
	var notifications []store.WorkOrderNotification
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&notifications).Error; err != nil {
		return nil, 0, fmt.Errorf("list work order notifications: %w", err)
	}
	return notifications, total, nil
}

func (s *WorkOrderService) MarkWorkOrderNotificationRead(scope WorkOrderScope, notificationPublicID string) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	if scope.ActorUserID == 0 {
		return fmt.Errorf("notification recipient identity is required")
	}
	notificationPublicID = strings.TrimSpace(notificationPublicID)
	if notificationPublicID == "" {
		return fmt.Errorf("work order notification public id is required")
	}
	now := time.Now().UTC()
	result := s.db.Model(&store.WorkOrderNotification{}).
		Where("public_id = ? AND tenant_id = ? AND project_id = ? AND recipient_user_id = ?", notificationPublicID, scope.TenantID, scope.ProjectID, scope.ActorUserID).
		Updates(map[string]any{"status": WorkOrderNotificationRead, "read_at": now})
	if result.Error != nil {
		return fmt.Errorf("mark work order notification read: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("work order notification not found")
	}
	return nil
}

func (s *WorkOrderService) ListMyWorkOrderNotificationsAfterID(scope WorkOrderScope, afterID uint, limit int) ([]store.WorkOrderNotification, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("notification recipient identity is required")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	var notifications []store.WorkOrderNotification
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND recipient_user_id = ? AND id > ?", scope.TenantID, scope.ProjectID, scope.ActorUserID, afterID).
		Order("id ASC").Limit(limit).Find(&notifications).Error; err != nil {
		return nil, fmt.Errorf("list new work order notifications: %w", err)
	}
	return notifications, nil
}

func (s *WorkOrderService) CompleteWorkOrderTask(scope WorkOrderScope, taskPublicID string, input WorkOrderTaskCompletionInput) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("task actor identity is required")
	}
	opinion, err := requiredProcessingOpinion(input.Comment)
	if err != nil {
		return nil, err
	}
	input.Comment = opinion
	var result store.WorkOrder
	err = s.db.Transaction(func(tx *gorm.DB) error {
		task, err := s.findWorkOrderTaskByPublicID(tx, scope, taskPublicID)
		if err != nil {
			return err
		}
		if task.AssigneeUserID != scope.ActorUserID {
			return fmt.Errorf("work order task is not assigned to the current user")
		}
		order, err := s.findWorkOrder(tx, scope, task.WorkOrderID)
		if err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		if !isGraphWorkOrderWorkflow(workflow) {
			return fmt.Errorf("work order does not use a graph workflow")
		}
		node, ok := workflow.graphNode(task.NodeID)
		if !ok || node.Type != WorkOrderWorkflowNodeUserTask {
			return fmt.Errorf("work order task references an invalid workflow node")
		}
		var instance store.WorkOrderNodeInstance
		if err := tx.Where("id = ? AND work_order_id = ? AND workflow_cycle = ? AND node_id = ?", task.WorkOrderNodeInstanceID, order.ID, order.WorkflowCycle, task.NodeID).First(&instance).Error; err != nil {
			return fmt.Errorf("load work order node instance: %w", err)
		}
		if instance.Status != WorkOrderNodeInstanceActive {
			return fmt.Errorf("work order task node is no longer active")
		}
		now := time.Now().UTC()
		comment := strings.TrimSpace(input.Comment)
		update := tx.Model(&store.WorkOrderTask{}).Where("id = ? AND status = ?", task.ID, WorkOrderTaskPending).
			Updates(map[string]any{"status": WorkOrderTaskCompleted, "comment": comment, "completed_at": now})
		if update.Error != nil {
			return fmt.Errorf("complete work order task: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("work order task is no longer pending")
		}
		task.Status = WorkOrderTaskCompleted
		task.Comment = comment
		task.CompletedAt = &now
		if err := s.appendWorkOrderEvent(tx, order, "workflow.task.completed", scope.ActorUserID, map[string]any{
			"task_public_id": task.PublicID,
			"node_id":        node.ID,
			"task_kind":      node.TaskKind,
			"comment":        comment,
		}); err != nil {
			return err
		}

		shouldAdvance := node.CompletionMode == WorkOrderApprovalAny
		if node.CompletionMode == WorkOrderApprovalAny {
			if err := tx.Model(&store.WorkOrderTask{}).
				Where("work_order_node_instance_id = ? AND id <> ? AND status = ?", instance.ID, task.ID, WorkOrderTaskPending).
				Updates(map[string]any{"status": WorkOrderTaskCancelled, "completed_at": now}).Error; err != nil {
				return fmt.Errorf("cancel redundant work order tasks: %w", err)
			}
		} else {
			var pending int64
			if err := tx.Model(&store.WorkOrderTask{}).Where("work_order_node_instance_id = ? AND status = ?", instance.ID, WorkOrderTaskPending).Count(&pending).Error; err != nil {
				return fmt.Errorf("count pending work order tasks: %w", err)
			}
			shouldAdvance = pending == 0
		}
		if !shouldAdvance {
			result = *order
			return nil
		}
		instanceUpdate := tx.Model(&store.WorkOrderNodeInstance{}).Where("id = ? AND status = ?", instance.ID, WorkOrderNodeInstanceActive).
			Updates(map[string]any{"status": WorkOrderNodeInstanceCompleted, "completed_at": now})
		if instanceUpdate.Error != nil {
			return fmt.Errorf("complete work order node instance: %w", instanceUpdate.Error)
		}
		if instanceUpdate.RowsAffected == 1 {
			if err := s.appendWorkOrderEvent(tx, order, "workflow.node.completed", scope.ActorUserID, map[string]any{"node_id": node.ID, "branch_id": instance.BranchID}); err != nil {
				return err
			}
			if err := s.continueGraphWorkOrderWorkflow(tx, order, workflow, node.ID, instance.BranchID, scope.ActorUserID); err != nil {
				return err
			}
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) RejectWorkOrderTask(scope WorkOrderScope, taskPublicID string, input WorkOrderTaskCompletionInput) (*store.WorkOrder, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if scope.ActorUserID == 0 {
		return nil, fmt.Errorf("task actor identity is required")
	}
	opinion, err := requiredProcessingOpinion(input.Comment)
	if err != nil {
		return nil, err
	}
	input.Comment = opinion
	var result store.WorkOrder
	err = s.db.Transaction(func(tx *gorm.DB) error {
		task, err := s.findWorkOrderTaskByPublicID(tx, scope, taskPublicID)
		if err != nil {
			return err
		}
		if task.AssigneeUserID != scope.ActorUserID {
			return fmt.Errorf("work order task is not assigned to the current user")
		}
		order, err := s.findWorkOrder(tx, scope, task.WorkOrderID)
		if err != nil {
			return err
		}
		workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
		if err != nil {
			return err
		}
		if !isGraphWorkOrderWorkflow(workflow) {
			return fmt.Errorf("work order does not use a graph workflow")
		}
		node, ok := workflow.graphNode(task.NodeID)
		if !ok || node.Type != WorkOrderWorkflowNodeUserTask || node.TaskKind != WorkOrderTaskKindApprove || strings.TrimSpace(node.RejectTargetNodeID) == "" {
			return fmt.Errorf("work order task cannot be rejected")
		}
		var instance store.WorkOrderNodeInstance
		if err := tx.Where("id = ? AND work_order_id = ? AND workflow_cycle = ? AND node_id = ? AND status = ?", task.WorkOrderNodeInstanceID, order.ID, order.WorkflowCycle, task.NodeID, WorkOrderNodeInstanceActive).First(&instance).Error; err != nil {
			return fmt.Errorf("load active work order node instance: %w", err)
		}
		now := time.Now().UTC()
		comment := strings.TrimSpace(input.Comment)
		update := tx.Model(&store.WorkOrderTask{}).Where("id = ? AND status = ?", task.ID, WorkOrderTaskPending).
			Updates(map[string]any{"status": WorkOrderTaskRejected, "comment": comment, "completed_at": now})
		if update.Error != nil {
			return fmt.Errorf("reject work order task: %w", update.Error)
		}
		if update.RowsAffected != 1 {
			return fmt.Errorf("work order task is no longer pending")
		}
		if err := tx.Model(&store.WorkOrderTask{}).Where("work_order_id = ? AND workflow_cycle = ? AND status = ?", order.ID, order.WorkflowCycle, WorkOrderTaskPending).
			Updates(map[string]any{"status": WorkOrderTaskCancelled, "completed_at": now}).Error; err != nil {
			return fmt.Errorf("cancel pending work order tasks after rejection: %w", err)
		}
		if err := tx.Model(&store.WorkOrderNodeInstance{}).Where("work_order_id = ? AND workflow_cycle = ? AND status = ?", order.ID, order.WorkflowCycle, WorkOrderNodeInstanceActive).
			Updates(map[string]any{"status": WorkOrderNodeInstanceCancelled, "completed_at": now}).Error; err != nil {
			return fmt.Errorf("cancel active work order node instances after rejection: %w", err)
		}
		previousCycle := order.WorkflowCycle
		if err := s.updateGraphWorkOrderState(tx, order, WorkOrderStatusOpen, previousCycle+1, nil); err != nil {
			return err
		}
		if err := s.appendWorkOrderEvent(tx, order, "workflow.task.rejected", scope.ActorUserID, map[string]any{
			"task_public_id": task.PublicID,
			"node_id":        node.ID,
			"reject_target":  node.RejectTargetNodeID,
			"comment":        comment,
			"previous_cycle": previousCycle,
		}); err != nil {
			return err
		}
		if err := s.activateGraphWorkOrderNode(tx, order, workflow, node.RejectTargetNodeID, instance.BranchID, scope.ActorUserID); err != nil {
			return err
		}
		result = *order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *WorkOrderService) ListWorkOrderEvents(scope WorkOrderScope, workOrderID uint) ([]store.WorkOrderEvent, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	if _, err := s.findWorkOrder(s.db, scope, workOrderID); err != nil {
		return nil, err
	}
	var events []store.WorkOrderEvent
	if err := s.db.Where("tenant_id = ? AND project_id = ? AND work_order_id = ?", scope.TenantID, scope.ProjectID, workOrderID).
		Order("sequence ASC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list work order events: %w", err)
	}
	return events, nil
}

func (s *WorkOrderService) ListWorkOrderOutboxEvents(scope WorkOrderScope, status string) ([]store.WorkOrderOutboxEvent, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	query := s.db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID)
	if status = strings.TrimSpace(status); status != "" {
		switch status {
		case WorkOrderOutboxPending, WorkOrderOutboxDelivering, WorkOrderOutboxDelivered:
			query = query.Where("status = ?", status)
		default:
			return nil, fmt.Errorf("unsupported work order outbox status")
		}
	}
	var events []store.WorkOrderOutboxEvent
	if err := query.Order("id ASC").Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list work order outbox events: %w", err)
	}
	return events, nil
}

func (s *WorkOrderService) ClaimWorkOrderOutboxEvents(scope WorkOrderScope, limit int, lockToken string, lease time.Duration) ([]store.WorkOrderOutboxEvent, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	lockToken = strings.TrimSpace(lockToken)
	if lockToken == "" || len(lockToken) > 128 {
		return nil, fmt.Errorf("outbox lock token is required and must not exceed 128 characters")
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	if lease <= 0 || lease > time.Hour {
		lease = 5 * time.Minute
	}
	now := time.Now().UTC()
	lockedUntil := now.Add(lease)
	claimed := make([]store.WorkOrderOutboxEvent, 0, limit)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var candidates []store.WorkOrderOutboxEvent
		if err := tx.Where("tenant_id = ? AND project_id = ? AND ((status = ? AND available_at <= ?) OR (status = ? AND locked_until IS NOT NULL AND locked_until <= ?))",
			scope.TenantID, scope.ProjectID, WorkOrderOutboxPending, now, WorkOrderOutboxDelivering, now).
			Order("id ASC").Limit(limit).Find(&candidates).Error; err != nil {
			return fmt.Errorf("find claimable work order outbox events: %w", err)
		}
		for _, candidate := range candidates {
			result := tx.Model(&store.WorkOrderOutboxEvent{}).
				Where("id = ? AND ((status = ? AND available_at <= ?) OR (status = ? AND locked_until IS NOT NULL AND locked_until <= ?))",
					candidate.ID, WorkOrderOutboxPending, now, WorkOrderOutboxDelivering, now).
				Updates(map[string]any{
					"status":        WorkOrderOutboxDelivering,
					"attempt_count": gorm.Expr("attempt_count + ?", 1),
					"locked_at":     now,
					"locked_until":  lockedUntil,
					"lock_token":    lockToken,
					"last_error":    "",
				})
			if result.Error != nil {
				return fmt.Errorf("claim work order outbox event: %w", result.Error)
			}
			if result.RowsAffected != 1 {
				continue
			}
			if err := tx.First(&candidate, candidate.ID).Error; err != nil {
				return fmt.Errorf("reload claimed work order outbox event: %w", err)
			}
			claimed = append(claimed, candidate)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func (s *WorkOrderService) AcknowledgeWorkOrderOutboxEvent(scope WorkOrderScope, outboxID uint, lockToken string) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	lockToken = strings.TrimSpace(lockToken)
	if outboxID == 0 || lockToken == "" {
		return fmt.Errorf("outbox event id and lock token are required")
	}
	now := time.Now().UTC()
	result := s.db.Model(&store.WorkOrderOutboxEvent{}).
		Where("id = ? AND tenant_id = ? AND project_id = ? AND status = ? AND lock_token = ?", outboxID, scope.TenantID, scope.ProjectID, WorkOrderOutboxDelivering, lockToken).
		Updates(map[string]any{"status": WorkOrderOutboxDelivered, "delivered_at": now, "locked_at": nil, "locked_until": nil, "lock_token": ""})
	if result.Error != nil {
		return fmt.Errorf("acknowledge work order outbox event: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("work order outbox event is not claimed by this connector")
	}
	return nil
}

func (s *WorkOrderService) RetryWorkOrderOutboxEvent(scope WorkOrderScope, outboxID uint, lockToken, failure string, retryAfter time.Duration) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	lockToken = strings.TrimSpace(lockToken)
	failure = strings.TrimSpace(failure)
	if outboxID == 0 || lockToken == "" || failure == "" {
		return fmt.Errorf("outbox event id, lock token, and failure reason are required")
	}
	if retryAfter < 0 || retryAfter > 7*24*time.Hour {
		return fmt.Errorf("outbox retry delay must be between 0 and seven days")
	}
	result := s.db.Model(&store.WorkOrderOutboxEvent{}).
		Where("id = ? AND tenant_id = ? AND project_id = ? AND status = ? AND lock_token = ?", outboxID, scope.TenantID, scope.ProjectID, WorkOrderOutboxDelivering, lockToken).
		Updates(map[string]any{
			"status":       WorkOrderOutboxPending,
			"available_at": time.Now().UTC().Add(retryAfter),
			"locked_at":    nil,
			"locked_until": nil,
			"lock_token":   "",
			"last_error":   failure,
		})
	if result.Error != nil {
		return fmt.Errorf("retry work order outbox event: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("work order outbox event is not claimed by this connector")
	}
	return nil
}

func (s *WorkOrderService) LinkWorkOrder(scope WorkOrderScope, workOrderID uint, input WorkOrderLinkInput) (*store.WorkOrderLink, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	input.RelationType = strings.TrimSpace(input.RelationType)
	input.ExternalRef = strings.TrimSpace(input.ExternalRef)
	if !isWorkOrderIdentifier(input.RelationType) || input.ExternalRef == "" || len(input.ExternalRef) > 191 {
		return nil, fmt.Errorf("valid relation type and external reference are required")
	}
	metadata, err := json.Marshal(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("encode work order link metadata: %w", err)
	}
	var link store.WorkOrderLink
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := s.findWorkOrder(tx, scope, workOrderID); err != nil {
			return err
		}
		link = store.WorkOrderLink{
			TenantID:     scope.TenantID,
			ProjectID:    scope.ProjectID,
			WorkOrderID:  workOrderID,
			RelationType: input.RelationType,
			ExternalRef:  input.ExternalRef,
			Metadata:     string(metadata),
		}
		return tx.Where("work_order_id = ? AND relation_type = ? AND external_ref = ?", workOrderID, input.RelationType, input.ExternalRef).
			Assign(link).FirstOrCreate(&link).Error
	})
	if err != nil {
		return nil, fmt.Errorf("link work order: %w", err)
	}
	return &link, nil
}

func ValidateWorkOrderFormDefinition(definition WorkOrderFormDefinition) error {
	seen := make(map[string]bool, len(definition.Fields))
	for _, field := range definition.Fields {
		field.Key = strings.TrimSpace(field.Key)
		field.Label = strings.TrimSpace(field.Label)
		if !isWorkOrderIdentifier(field.Key) {
			return fmt.Errorf("form field key %q is invalid", field.Key)
		}
		if seen[field.Key] {
			return fmt.Errorf("form field key %q is duplicated", field.Key)
		}
		seen[field.Key] = true
		if field.Label == "" || len([]rune(field.Label)) > 128 {
			return fmt.Errorf("form field %q must have a 1-128 character label", field.Key)
		}
		if !isSupportedWorkOrderFormFieldType(field.Type) {
			return fmt.Errorf("form field %q has unsupported type %q", field.Key, field.Type)
		}
		if field.Type == WorkOrderFormFieldSelect || field.Type == WorkOrderFormFieldMultiSelect {
			if len(field.Options) == 0 {
				return fmt.Errorf("option field %q must define options", field.Key)
			}
			optionSet := make(map[string]bool, len(field.Options))
			for _, option := range field.Options {
				option = strings.TrimSpace(option)
				if option == "" || optionSet[option] {
					return fmt.Errorf("option field %q has invalid options", field.Key)
				}
				optionSet[option] = true
			}
		} else if len(field.Options) > 0 {
			return fmt.Errorf("only option fields can define options")
		}
		if field.DefaultValue != nil {
			if err := validateWorkOrderFieldValue(field, field.DefaultValue); err != nil {
				return fmt.Errorf("default value for %q: %w", field.Key, err)
			}
		}
	}
	return nil
}

func compileLegacyWorkOrderForm(definition WorkOrderFormDefinition) (workorderport.CompiledForm, error) {
	properties := make(map[string]any, len(definition.Fields))
	defaults := make(map[string]any)
	required := make([]string, 0)
	order := make([]string, 0, len(definition.Fields))
	for _, field := range definition.Fields {
		fieldType := "string"
		switch field.Type {
		case WorkOrderFormFieldNumber:
			fieldType = "number"
		case WorkOrderFormFieldInteger:
			fieldType = "integer"
		case WorkOrderFormFieldBoolean:
			fieldType = "boolean"
		case WorkOrderFormFieldSelect:
			fieldType = "enum"
		case WorkOrderFormFieldMultiSelect:
			fieldType = "array"
		case WorkOrderFormFieldImages:
			fieldType = "array"
		case WorkOrderFormFieldDateTime:
			fieldType = "date-time"
		}
		property := map[string]any{"type": fieldType, "title": field.Label}
		if field.Type == WorkOrderFormFieldMultiSelect {
			options := make([]any, len(field.Options))
			for i, option := range field.Options {
				options[i] = option
			}
			property["items"] = map[string]any{"type": "string", "enum": options}
		} else if len(field.Options) > 0 {
			options := make([]any, len(field.Options))
			for i, option := range field.Options {
				options[i] = option
			}
			property["enum"] = options
		}
		if field.Type == WorkOrderFormFieldImages {
			property["items"] = map[string]any{"type": "string"}
			property["x-noyo-images"] = true
		}
		if field.Type == WorkOrderFormFieldDevice || field.Type == WorkOrderFormFieldUser {
			property["x-noyo-resource"] = field.Type
		}
		if field.Type == WorkOrderFormFieldAttachment {
			property["x-noyo-attachment"] = true
		}
		if field.DefaultValue != nil {
			property["default"] = field.DefaultValue
			defaults[field.Key] = field.DefaultValue
		}
		properties[field.Key] = property
		order = append(order, field.Key)
		if field.Required {
			required = append(required, field.Key)
		}
	}
	orderValues := make([]any, len(order))
	for i, value := range order {
		orderValues[i] = value
	}
	requiredValues := make([]any, len(required))
	for i, value := range required {
		requiredValues[i] = value
	}
	return workorderport.CompileFormDefinition(workorderport.FormDefinition{
		Schema:   map[string]any{"type": "object", "properties": properties, "required": requiredValues},
		UISchema: map[string]any{"order": orderValues}, Defaults: defaults, Required: required, FieldOrder: order,
	})
}

func compiledSnapshot(compiled workorderport.CompiledForm, key string) []byte {
	var value any
	switch key {
	case "schema":
		value = compiled.Definition.Schema
	case "ui_schema":
		value = compiled.Definition.UISchema
	case "defaults":
		value = compiled.Definition.Defaults
	case "required":
		value = compiled.Definition.Required
	case "field_order":
		value = compiled.Definition.FieldOrder
	case "resource_fields":
		value = compiled.ResourceFields
	default:
		value = nil
	}
	encoded, _ := json.Marshal(value)
	return encoded
}

func ValidateAndNormalizeWorkOrderFormData(definition WorkOrderFormDefinition, data map[string]any) (map[string]any, error) {
	if err := ValidateWorkOrderFormDefinition(definition); err != nil {
		return nil, err
	}
	normalized := make(map[string]any, len(data)+len(definition.Fields))
	for key, value := range data {
		normalized[key] = value
	}
	allowed := make(map[string]WorkOrderFormField, len(definition.Fields))
	for _, field := range definition.Fields {
		allowed[field.Key] = field
		value, exists := normalized[field.Key]
		if !exists && field.DefaultValue != nil {
			value = field.DefaultValue
			normalized[field.Key] = value
			exists = true
		}
		if field.Required && (!exists || !workOrderFormValuePresent(value)) {
			return nil, fmt.Errorf("form field %q is required", field.Key)
		}
		if exists && workOrderFormValuePresent(value) {
			if err := validateWorkOrderFieldValue(field, value); err != nil {
				return nil, fmt.Errorf("form field %q: %w", field.Key, err)
			}
		}
	}
	for key := range normalized {
		if _, ok := allowed[key]; !ok {
			return nil, fmt.Errorf("form field %q is not defined", key)
		}
	}
	return normalized, nil
}

func ValidateWorkOrderWorkflowDefinition(definition WorkOrderWorkflowDefinition) error {
	if definition.SchemaVersion != 0 {
		if definition.SchemaVersion != WorkOrderWorkflowSchemaV2 {
			return fmt.Errorf("unsupported workflow schema version %d", definition.SchemaVersion)
		}
		return validateWorkOrderWorkflowGraphDefinition(definition)
	}
	definition.InitialStatus = strings.TrimSpace(definition.InitialStatus)
	if !isWorkOrderIdentifier(definition.InitialStatus) {
		return fmt.Errorf("workflow initial status is required")
	}
	if len(definition.Statuses) == 0 {
		return fmt.Errorf("workflow must define at least one status")
	}
	statuses := make(map[string]WorkOrderWorkflowStatus, len(definition.Statuses))
	for _, status := range definition.Statuses {
		status.Key = strings.TrimSpace(status.Key)
		status.Name = strings.TrimSpace(status.Name)
		status.Category = strings.TrimSpace(status.Category)
		if !isWorkOrderIdentifier(status.Key) || status.Name == "" || !isSupportedWorkOrderStatusCategory(status.Category) {
			return fmt.Errorf("workflow status %q is invalid", status.Key)
		}
		if _, exists := statuses[status.Key]; exists {
			return fmt.Errorf("workflow status %q is duplicated", status.Key)
		}
		statuses[status.Key] = status
	}
	if _, exists := statuses[definition.InitialStatus]; !exists {
		return fmt.Errorf("workflow initial status %q is not defined", definition.InitialStatus)
	}
	transitionKeys := make(map[string]bool, len(definition.Transitions))
	approvalByStatus := make(map[string]bool)
	plainByStatus := make(map[string]bool)
	for _, transition := range definition.Transitions {
		transition.Key = strings.TrimSpace(transition.Key)
		transition.From = strings.TrimSpace(transition.From)
		transition.To = strings.TrimSpace(transition.To)
		transition.RejectTo = strings.TrimSpace(transition.RejectTo)
		if !isWorkOrderIdentifier(transition.Key) || transition.Name == "" || !isWorkOrderIdentifier(transition.From) || !isWorkOrderIdentifier(transition.To) {
			return fmt.Errorf("workflow transition is invalid")
		}
		if transitionKeys[transition.Key] {
			return fmt.Errorf("workflow transition %q is duplicated", transition.Key)
		}
		transitionKeys[transition.Key] = true
		if _, exists := statuses[transition.From]; !exists {
			return fmt.Errorf("workflow transition %q has unknown source status", transition.Key)
		}
		if _, exists := statuses[transition.To]; !exists {
			return fmt.Errorf("workflow transition %q has unknown target status", transition.Key)
		}
		if len(transition.ApproverUserIDs) > 0 {
			if plainByStatus[transition.From] {
				return fmt.Errorf("workflow status %q cannot have both approval and plain transitions", transition.From)
			}
			if approvalByStatus[transition.From] {
				return fmt.Errorf("workflow status %q may have only one approval transition", transition.From)
			}
			approvalByStatus[transition.From] = true
			if transition.ApprovalMode != WorkOrderApprovalAny && transition.ApprovalMode != WorkOrderApprovalAll {
				return fmt.Errorf("workflow transition %q has invalid approval mode", transition.Key)
			}
			if transition.RejectTo == "" {
				return fmt.Errorf("workflow approval transition %q must define reject_to", transition.Key)
			}
			if _, exists := statuses[transition.RejectTo]; !exists {
				return fmt.Errorf("workflow approval transition %q has unknown reject status", transition.Key)
			}
			users := make(map[uint]bool, len(transition.ApproverUserIDs))
			for _, userID := range transition.ApproverUserIDs {
				if userID == 0 || users[userID] {
					return fmt.Errorf("workflow transition %q has invalid approvers", transition.Key)
				}
				users[userID] = true
			}
		} else if transition.ApprovalMode != "" || transition.RejectTo != "" {
			return fmt.Errorf("workflow transition %q defines approval settings without approvers", transition.Key)
		} else {
			if approvalByStatus[transition.From] {
				return fmt.Errorf("workflow status %q cannot have both approval and plain transitions", transition.From)
			}
			plainByStatus[transition.From] = true
		}
	}
	// Published workflows must be executable from the initial node and must
	// contain at least one terminal status; otherwise a published graph can
	// strand every work order indefinitely.
	adjacency := make(map[string][]string, len(statuses))
	for _, transition := range definition.Transitions {
		adjacency[transition.From] = append(adjacency[transition.From], transition.To)
	}
	reachable := map[string]bool{}
	var visit func(string)
	visit = func(status string) {
		if reachable[status] {
			return
		}
		reachable[status] = true
		for _, next := range adjacency[status] {
			visit(next)
		}
	}
	visit(definition.InitialStatus)
	terminal := false
	for key, status := range statuses {
		if !reachable[key] {
			return fmt.Errorf("workflow status %q is unreachable", key)
		}
		if isTerminalWorkOrderStatus(status.Category) {
			terminal = true
		}
	}
	if !terminal {
		return fmt.Errorf("workflow must define at least one terminal status")
	}
	return nil
}

func validateWorkOrderWorkflowGraphDefinition(definition WorkOrderWorkflowDefinition) error {
	definition.StartNodeID = strings.TrimSpace(definition.StartNodeID)
	if !isWorkOrderIdentifier(definition.StartNodeID) {
		return fmt.Errorf("workflow start node is required")
	}
	if len(definition.Nodes) == 0 {
		return fmt.Errorf("workflow must define nodes")
	}
	nodes := make(map[string]WorkOrderWorkflowNode, len(definition.Nodes))
	startCount := 0
	endCount := 0
	splitPairs := make(map[string]string)
	joinPairs := make(map[string]string)
	for _, node := range definition.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		node.Type = strings.TrimSpace(node.Type)
		node.Name = strings.TrimSpace(node.Name)
		if !isWorkOrderIdentifier(node.ID) {
			return fmt.Errorf("workflow node id %q is invalid", node.ID)
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("workflow node %q is duplicated", node.ID)
		}
		switch node.Type {
		case WorkOrderWorkflowNodeStart:
			startCount++
			if node.ID != definition.StartNodeID {
				return fmt.Errorf("workflow start node does not match start_node_id")
			}
		case WorkOrderWorkflowNodeEnd:
			endCount++
			if node.Result != "completed" && node.Result != WorkOrderStatusClosed && node.Result != WorkOrderStatusCancelled {
				return fmt.Errorf("workflow end node %q has invalid result", node.ID)
			}
		case WorkOrderWorkflowNodeUserTask:
			if node.Name == "" || (node.TaskKind != WorkOrderTaskKindHandle && node.TaskKind != WorkOrderTaskKindApprove) {
				return fmt.Errorf("workflow user task %q is invalid", node.ID)
			}
			if node.CompletionMode != WorkOrderApprovalAny && node.CompletionMode != WorkOrderApprovalAll {
				return fmt.Errorf("workflow user task %q has invalid completion mode", node.ID)
			}
			participants := make(map[uint]bool, len(node.ParticipantUserIDs))
			for _, participantID := range node.ParticipantUserIDs {
				if participantID == 0 || participants[participantID] {
					return fmt.Errorf("workflow user task %q has invalid participants", node.ID)
				}
				participants[participantID] = true
			}
			if len(participants) == 0 {
				return fmt.Errorf("workflow user task %q must define participants", node.ID)
			}
		case WorkOrderWorkflowNodeCCTask:
			participants := make(map[uint]bool, len(node.ParticipantUserIDs))
			for _, participantID := range node.ParticipantUserIDs {
				if participantID == 0 || participants[participantID] {
					return fmt.Errorf("workflow cc task %q has invalid participants", node.ID)
				}
				participants[participantID] = true
			}
			if len(participants) == 0 {
				return fmt.Errorf("workflow cc task %q must define participants", node.ID)
			}
		case WorkOrderWorkflowNodeParallelSplit:
			node.PairID = strings.TrimSpace(node.PairID)
			if !isWorkOrderIdentifier(node.PairID) || splitPairs[node.PairID] != "" {
				return fmt.Errorf("workflow parallel split %q has invalid pair", node.ID)
			}
			splitPairs[node.PairID] = node.ID
		case WorkOrderWorkflowNodeParallelJoin:
			node.PairID = strings.TrimSpace(node.PairID)
			if !isWorkOrderIdentifier(node.PairID) || joinPairs[node.PairID] != "" {
				return fmt.Errorf("workflow parallel join %q has invalid pair", node.ID)
			}
			joinPairs[node.PairID] = node.ID
		default:
			return fmt.Errorf("workflow node %q has unsupported type %q", node.ID, node.Type)
		}
		nodes[node.ID] = node
	}
	if startCount != 1 || endCount == 0 {
		return fmt.Errorf("workflow must define exactly one start node and at least one end node")
	}
	for pairID, splitID := range splitPairs {
		if joinPairs[pairID] == "" {
			return fmt.Errorf("workflow parallel split %q has no matching join", splitID)
		}
	}
	for pairID, joinID := range joinPairs {
		if splitPairs[pairID] == "" {
			return fmt.Errorf("workflow parallel join %q has no matching split", joinID)
		}
	}

	edgeIDs := make(map[string]bool, len(definition.Edges))
	incoming := make(map[string]int, len(nodes))
	incomingByTarget := make(map[string][]string, len(nodes))
	outgoing := make(map[string][]WorkOrderWorkflowEdge, len(nodes))
	for _, edge := range definition.Edges {
		edge.ID = strings.TrimSpace(edge.ID)
		edge.Source = strings.TrimSpace(edge.Source)
		edge.Target = strings.TrimSpace(edge.Target)
		if !isWorkOrderIdentifier(edge.ID) || edgeIDs[edge.ID] {
			return fmt.Errorf("workflow edge %q is invalid", edge.ID)
		}
		if _, exists := nodes[edge.Source]; !exists {
			return fmt.Errorf("workflow edge %q has unknown source", edge.ID)
		}
		if _, exists := nodes[edge.Target]; !exists {
			return fmt.Errorf("workflow edge %q has unknown target", edge.ID)
		}
		edgeIDs[edge.ID] = true
		incoming[edge.Target]++
		incomingByTarget[edge.Target] = append(incomingByTarget[edge.Target], edge.Source)
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	if incoming[definition.StartNodeID] != 0 || len(outgoing[definition.StartNodeID]) != 1 {
		return fmt.Errorf("workflow start node must have no incoming edge and one outgoing edge")
	}
	parallelSplitByBranchNode := workOrderWorkflowParallelSplitByBranchNode(nodes, outgoing)
	for nodeID, node := range nodes {
		switch node.Type {
		case WorkOrderWorkflowNodeEnd:
			if incoming[nodeID] != 1 {
				return fmt.Errorf("workflow end node %q must have one incoming edge", nodeID)
			}
			if len(outgoing[nodeID]) != 0 {
				return fmt.Errorf("workflow end node %q must not have outgoing edges", nodeID)
			}
		case WorkOrderWorkflowNodeParallelSplit:
			if incoming[nodeID] != 1 {
				return fmt.Errorf("workflow parallel split %q must have one incoming edge", nodeID)
			}
			branchIDs := make(map[string]bool, len(outgoing[nodeID]))
			if len(outgoing[nodeID]) < 2 {
				return fmt.Errorf("workflow parallel split %q must have at least two branches", nodeID)
			}
			for _, edge := range outgoing[nodeID] {
				edge.BranchID = strings.TrimSpace(edge.BranchID)
				if !isWorkOrderIdentifier(edge.BranchID) || branchIDs[edge.BranchID] {
					return fmt.Errorf("workflow parallel split %q has invalid branches", nodeID)
				}
				branchIDs[edge.BranchID] = true
			}
		case WorkOrderWorkflowNodeParallelJoin:
			if incoming[nodeID] < 2 {
				return fmt.Errorf("workflow parallel join %q must have at least two incoming edges", nodeID)
			}
			if len(outgoing[nodeID]) != 1 {
				return fmt.Errorf("workflow parallel join %q must have one outgoing edge", nodeID)
			}
		case WorkOrderWorkflowNodeUserTask:
			if incoming[nodeID] != 1 || len(outgoing[nodeID]) != 1 {
				return fmt.Errorf("workflow user task %q must have one incoming and one outgoing edge", nodeID)
			}
			if node.TaskKind == WorkOrderTaskKindApprove {
				targetID := strings.TrimSpace(node.RejectTargetNodeID)
				if !isWorkOrderIdentifier(targetID) {
					return fmt.Errorf("workflow approval task %q must define reject target", nodeID)
				}
				validTarget := false
				for _, candidate := range workOrderWorkflowRejectTargetCandidates(nodeID, nodes, incomingByTarget, parallelSplitByBranchNode) {
					if candidate.ID == targetID {
						validTarget = true
						break
					}
				}
				if !validTarget {
					return fmt.Errorf("workflow approval task %q has invalid reject target", nodeID)
				}
			}
		case WorkOrderWorkflowNodeCCTask:
			if incoming[nodeID] != 1 || len(outgoing[nodeID]) != 1 {
				return fmt.Errorf("workflow cc task %q must have one incoming and one outgoing edge", nodeID)
			}
		}
	}
	if err := validateGraphParallelPairPaths(nodes, outgoing, splitPairs, joinPairs); err != nil {
		return err
	}

	visiting := make(map[string]bool, len(nodes))
	visited := make(map[string]bool, len(nodes))
	var visit func(string) error
	visit = func(nodeID string) error {
		if visiting[nodeID] {
			return fmt.Errorf("workflow graph must not contain cycles")
		}
		if visited[nodeID] {
			return nil
		}
		visiting[nodeID] = true
		for _, edge := range outgoing[nodeID] {
			if err := visit(edge.Target); err != nil {
				return err
			}
		}
		visiting[nodeID] = false
		visited[nodeID] = true
		return nil
	}
	if err := visit(definition.StartNodeID); err != nil {
		return err
	}
	for nodeID := range nodes {
		if !visited[nodeID] {
			return fmt.Errorf("workflow node %q is unreachable", nodeID)
		}
	}
	return nil
}

func validateGraphParallelPairPaths(nodes map[string]WorkOrderWorkflowNode, outgoing map[string][]WorkOrderWorkflowEdge, splitPairs, joinPairs map[string]string) error {
	for pairID, splitID := range splitPairs {
		joinID := joinPairs[pairID]
		if len(outgoing[splitID]) != len(workflowIncomingEdgesForNode(outgoing, joinID)) {
			return fmt.Errorf("workflow parallel pair %q must have matching branch and join counts", pairID)
		}
		for _, branch := range outgoing[splitID] {
			visited := make(map[string]bool)
			current := branch.Target
			for current != joinID {
				if visited[current] {
					return fmt.Errorf("workflow parallel pair %q contains a branch cycle", pairID)
				}
				visited[current] = true
				node := nodes[current]
				if node.Type == WorkOrderWorkflowNodeEnd {
					return fmt.Errorf("workflow parallel branch %q must reach its paired join", branch.ID)
				}
				if node.Type == WorkOrderWorkflowNodeParallelSplit || node.Type == WorkOrderWorkflowNodeParallelJoin {
					return fmt.Errorf("workflow parallel pair %q cannot contain nested parallel gateways", pairID)
				}
				next := outgoing[current]
				if len(next) != 1 {
					return fmt.Errorf("workflow parallel branch %q must have a single path to its paired join", branch.ID)
				}
				current = next[0].Target
			}
		}
	}
	return nil
}

func workflowIncomingEdgesForNode(outgoing map[string][]WorkOrderWorkflowEdge, nodeID string) []WorkOrderWorkflowEdge {
	incoming := make([]WorkOrderWorkflowEdge, 0)
	for _, edges := range outgoing {
		for _, edge := range edges {
			if edge.Target == nodeID {
				incoming = append(incoming, edge)
			}
		}
	}
	return incoming
}

func (s *WorkOrderService) findTemplate(db *gorm.DB, scope WorkOrderScope, templateID uint) (*store.WorkOrderTemplate, error) {
	var template store.WorkOrderTemplate
	if err := db.Where("id = ? AND tenant_id = ? AND project_id = ?", templateID, scope.TenantID, scope.ProjectID).First(&template).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("work order template not found")
		}
		return nil, fmt.Errorf("load work order template: %w", err)
	}
	return &template, nil
}

func (s *WorkOrderService) findWorkOrder(db *gorm.DB, scope WorkOrderScope, workOrderID uint) (*store.WorkOrder, error) {
	if workOrderID == 0 {
		return nil, fmt.Errorf("work order id is required")
	}
	var order store.WorkOrder
	if err := db.Where("id = ? AND tenant_id = ? AND project_id = ?", workOrderID, scope.TenantID, scope.ProjectID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("work order not found")
		}
		return nil, fmt.Errorf("load work order: %w", err)
	}
	return &order, nil
}

func (s *WorkOrderService) findWorkOrderByPublicID(db *gorm.DB, scope WorkOrderScope, publicID string) (*store.WorkOrder, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, fmt.Errorf("work order public id is required")
	}
	var order store.WorkOrder
	if err := db.Where("public_id = ? AND tenant_id = ? AND project_id = ?", publicID, scope.TenantID, scope.ProjectID).First(&order).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("work order not found")
		}
		return nil, fmt.Errorf("load work order: %w", err)
	}
	return &order, nil
}

func (s *WorkOrderService) applyWorkOrderStatus(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, toStatus, eventType string, actorUserID uint, payload map[string]any) error {
	return s.applyWorkOrderStatusWithExpectedVersion(tx, order, workflow, toStatus, eventType, actorUserID, payload, nil)
}

func (s *WorkOrderService) applyWorkOrderStatusWithExpectedVersion(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, toStatus, eventType string, actorUserID uint, payload map[string]any, expectedVersion *int) error {
	targetStatus, ok := workflow.statusByKey(toStatus)
	if !ok {
		return fmt.Errorf("target status %q is not defined", toStatus)
	}
	if targetStatus.Category == WorkOrderStatusClosed {
		if err := ensureLinkedAlarmsReadyToClose(tx, order); err != nil {
			return err
		}
	}
	previousStatus := order.Status
	previousVersion := order.Version
	if previousVersion <= 0 {
		previousVersion = 1
	}
	expected := previousVersion
	if expectedVersion != nil {
		expected = *expectedVersion
	}
	if expected <= 0 {
		return ErrWorkOrderVersionConflict
	}
	nextCycle := order.WorkflowCycle + 1
	var closedAt *time.Time
	if isClosedWorkOrderStatus(targetStatus.Category) {
		now := time.Now().UTC()
		closedAt = &now
	}
	updates := map[string]any{"status": toStatus, "workflow_cycle": nextCycle, "closed_at": closedAt, "version": expected + 1}
	var resolvedAt *time.Time
	if resolutionJSON, ok := payload["_resolution_json"].(string); ok {
		now := time.Now().UTC()
		updates["resolution_json"] = resolutionJSON
		resolvedAt = &now
		updates["resolved_at"] = resolvedAt
		delete(payload, "_resolution_json")
	}
	query := tx.Model(&store.WorkOrder{}).Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, order.TenantID, order.ProjectID, expected)
	if err := query.Updates(updates).Error; err != nil {
		return fmt.Errorf("save work order status: %w", err)
	}
	if query.RowsAffected != 1 {
		return ErrWorkOrderVersionConflict
	}
	order.Status = toStatus
	order.WorkflowCycle = nextCycle
	order.ClosedAt = closedAt
	if resolvedAt != nil {
		order.ResolvedAt = resolvedAt
	}
	order.Version = expected + 1
	if payload == nil {
		payload = make(map[string]any)
	}
	payload["from_status"] = previousStatus
	payload["to_status"] = toStatus
	if err := s.appendWorkOrderEvent(tx, order, eventType, actorUserID, payload); err != nil {
		return err
	}
	if targetStatus.Category == WorkOrderStatusResolved || targetStatus.Category == WorkOrderStatusClosed {
		comment, _ := payload["comment"].(string)
		if err := closeLinkedAlarmsForWorkOrder(tx, order, actorUserID, comment); err != nil {
			return err
		}
	}
	return s.createPendingApprovalTasks(tx, order, workflow, actorUserID)
}

func (s *WorkOrderService) createPendingApprovalTasks(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, actorUserID uint) error {
	transition, ok := workflow.approvalTransitionFrom(order.Status)
	if !ok {
		return nil
	}
	for _, approverUserID := range transition.ApproverUserIDs {
		task := store.WorkOrderApprovalTask{
			TenantID:       order.TenantID,
			ProjectID:      order.ProjectID,
			WorkOrderID:    order.ID,
			TransitionKey:  transition.Key,
			WorkflowCycle:  order.WorkflowCycle,
			ApproverUserID: approverUserID,
			Status:         WorkOrderApprovalPending,
		}
		if err := tx.Create(&task).Error; err != nil {
			return fmt.Errorf("create approval task: %w", err)
		}
	}
	return s.appendWorkOrderEvent(tx, order, "approval.requested", actorUserID, map[string]any{
		"transition_key":    transition.Key,
		"approver_user_ids": transition.ApproverUserIDs,
		"approval_mode":     transition.ApprovalMode,
	})
}

func isGraphWorkOrderWorkflow(workflow WorkOrderWorkflowDefinition) bool {
	return workflow.SchemaVersion == WorkOrderWorkflowSchemaV2
}

func (workflow WorkOrderWorkflowDefinition) graphNode(nodeID string) (WorkOrderWorkflowNode, bool) {
	nodeID = strings.TrimSpace(nodeID)
	for _, node := range workflow.Nodes {
		if node.ID == nodeID {
			return node, true
		}
	}
	return WorkOrderWorkflowNode{}, false
}

func (workflow WorkOrderWorkflowDefinition) graphOutgoingEdges(nodeID string) []WorkOrderWorkflowEdge {
	edges := make([]WorkOrderWorkflowEdge, 0, 1)
	for _, edge := range workflow.Edges {
		if edge.Source == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func (workflow WorkOrderWorkflowDefinition) graphIncomingEdges(nodeID string) []WorkOrderWorkflowEdge {
	edges := make([]WorkOrderWorkflowEdge, 0, 1)
	for _, edge := range workflow.Edges {
		if edge.Target == nodeID {
			edges = append(edges, edge)
		}
	}
	return edges
}

func (s *WorkOrderService) startGraphWorkOrderWorkflow(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, actorUserID uint) error {
	if !isGraphWorkOrderWorkflow(workflow) {
		return fmt.Errorf("work order workflow is not a graph workflow")
	}
	return s.activateGraphWorkOrderNode(tx, order, workflow, workflow.StartNodeID, "", actorUserID)
}

func (s *WorkOrderService) activateGraphWorkOrderNode(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, nodeID, branchID string, actorUserID uint) error {
	node, ok := workflow.graphNode(nodeID)
	if !ok {
		return fmt.Errorf("workflow node %q does not exist", nodeID)
	}
	branchID = strings.TrimSpace(branchID)
	switch node.Type {
	case WorkOrderWorkflowNodeUserTask:
		instance, created, err := s.createGraphNodeInstance(tx, order, node, branchID, WorkOrderNodeInstanceActive)
		if err != nil || !created {
			return err
		}
		participants, err := s.loadGraphTaskParticipants(tx, order, node.ParticipantUserIDs)
		if err != nil {
			return err
		}
		taskPublicIDs := make([]string, 0, len(participants))
		for _, participant := range participants {
			displayName := strings.TrimSpace(participant.DisplayName)
			if displayName == "" {
				displayName = participant.Username
			}
			task := store.WorkOrderTask{
				TenantID:                order.TenantID,
				ProjectID:               order.ProjectID,
				WorkOrderID:             order.ID,
				WorkOrderNodeInstanceID: instance.ID,
				WorkflowCycle:           order.WorkflowCycle,
				NodeID:                  node.ID,
				TaskKind:                node.TaskKind,
				CompletionMode:          node.CompletionMode,
				AssigneeUserID:          participant.ID,
				AssigneeDisplayName:     displayName,
				Status:                  WorkOrderTaskPending,
			}
			if err := tx.Create(&task).Error; err != nil {
				return fmt.Errorf("create work order task: %w", err)
			}
			taskPublicIDs = append(taskPublicIDs, task.PublicID)
			if err := s.createGraphTaskNotification(tx, order, node, task); err != nil {
				return err
			}
		}
		return s.appendWorkOrderEvent(tx, order, "workflow.task.assigned", actorUserID, map[string]any{
			"node_id":         node.ID,
			"node_name":       node.Name,
			"task_kind":       node.TaskKind,
			"completion_mode": node.CompletionMode,
			"branch_id":       branchID,
			"task_public_ids": taskPublicIDs,
		})
	case WorkOrderWorkflowNodeStart, WorkOrderWorkflowNodeParallelSplit:
		_, created, err := s.createGraphNodeInstance(tx, order, node, branchID, WorkOrderNodeInstanceCompleted)
		if err != nil || !created {
			return err
		}
		return s.continueGraphWorkOrderWorkflow(tx, order, workflow, node.ID, branchID, actorUserID)
	case WorkOrderWorkflowNodeCCTask:
		_, created, err := s.createGraphNodeInstance(tx, order, node, branchID, WorkOrderNodeInstanceCompleted)
		if err != nil || !created {
			return err
		}
		participants, err := s.loadGraphTaskParticipants(tx, order, node.ParticipantUserIDs)
		if err != nil {
			return err
		}
		userIDs := make([]uint, 0, len(participants))
		for _, p := range participants {
			userIDs = append(userIDs, p.ID)
		}
		if len(userIDs) > 0 {
			if err := s.appendWorkOrderEvent(tx, order, "cc", actorUserID, map[string]any{"user_ids": userIDs, "comment": "来自流程节点自动抄送"}); err != nil {
				return err
			}
			for _, userID := range userIDs {
				notification := store.WorkOrderNotification{
					TenantID:          order.TenantID,
					ProjectID:         order.ProjectID,
					RecipientUserID:   userID,
					WorkOrderID:       order.ID,
					WorkOrderPublicID: order.PublicID,
					WorkOrderCode:     order.Code,
					WorkOrderTitle:    order.Title,
					Type:              "cc:" + uuid.NewString(),
					TaskKind:          "cc",
					Title:             "工单抄送",
					Content:           "来自流程节点自动抄送",
					Status:            "unread",
				}
				if err := tx.Create(&notification).Error; err != nil {
					return fmt.Errorf("create cc notification: %w", err)
				}
			}
		}
		return s.continueGraphWorkOrderWorkflow(tx, order, workflow, node.ID, branchID, actorUserID)
	case WorkOrderWorkflowNodeParallelJoin:
		return s.arriveAtGraphParallelJoin(tx, order, workflow, node, branchID, actorUserID)
	case WorkOrderWorkflowNodeEnd:
		_, created, err := s.createGraphNodeInstance(tx, order, node, branchID, WorkOrderNodeInstanceCompleted)
		if err != nil || !created {
			return err
		}
		return s.finishGraphWorkOrderWorkflow(tx, order, node, actorUserID)
	default:
		return fmt.Errorf("workflow node %q has unsupported type %q", node.ID, node.Type)
	}
}

func (s *WorkOrderService) continueGraphWorkOrderWorkflow(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, nodeID, branchID string, actorUserID uint) error {
	edges := workflow.graphOutgoingEdges(nodeID)
	if len(edges) == 0 {
		return fmt.Errorf("workflow node %q has no outgoing edge", nodeID)
	}
	for _, edge := range edges {
		nextBranchID := branchID
		if candidate := strings.TrimSpace(edge.BranchID); candidate != "" {
			nextBranchID = candidate
		}
		if err := s.activateGraphWorkOrderNode(tx, order, workflow, edge.Target, nextBranchID, actorUserID); err != nil {
			return err
		}
	}
	return nil
}

func (s *WorkOrderService) arriveAtGraphParallelJoin(tx *gorm.DB, order *store.WorkOrder, workflow WorkOrderWorkflowDefinition, node WorkOrderWorkflowNode, branchID string, actorUserID uint) error {
	if branchID == "" {
		return fmt.Errorf("workflow parallel join %q requires a branch identifier", node.ID)
	}
	_, created, err := s.createGraphNodeInstance(tx, order, node, branchID, WorkOrderNodeInstanceCompleted)
	if err != nil || !created {
		return err
	}
	incoming := workflow.graphIncomingEdges(node.ID)
	var arrived int64
	if err := tx.Model(&store.WorkOrderNodeInstance{}).
		Where("work_order_id = ? AND workflow_cycle = ? AND node_id = ? AND status = ? AND branch_id <> ''", order.ID, order.WorkflowCycle, node.ID, WorkOrderNodeInstanceCompleted).
		Count(&arrived).Error; err != nil {
		return fmt.Errorf("count parallel join branches: %w", err)
	}
	if arrived < int64(len(incoming)) {
		return nil
	}
	_, aggregateCreated, err := s.createGraphNodeInstance(tx, order, node, "", WorkOrderNodeInstanceCompleted)
	if err != nil || !aggregateCreated {
		return err
	}
	return s.continueGraphWorkOrderWorkflow(tx, order, workflow, node.ID, "", actorUserID)
}

func (s *WorkOrderService) finishGraphWorkOrderWorkflow(tx *gorm.DB, order *store.WorkOrder, node WorkOrderWorkflowNode, actorUserID uint) error {
	if order.ClosedAt != nil {
		return nil
	}
	status, err := graphWorkflowEndStatus(node.Result)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if err := s.updateGraphWorkOrderState(tx, order, status, order.WorkflowCycle, &now); err != nil {
		return err
	}
	if err := s.appendWorkOrderEvent(tx, order, "workflow.completed", actorUserID, map[string]any{"end_node_id": node.ID, "result": node.Result}); err != nil {
		return err
	}
	if status == WorkOrderStatusResolved || status == WorkOrderStatusClosed {
		return closeLinkedAlarmsForWorkOrder(tx, order, actorUserID, "")
	}
	return nil
}

func graphWorkflowEndStatus(result string) (string, error) {
	switch strings.TrimSpace(result) {
	case "completed":
		return WorkOrderStatusClosed, nil
	case WorkOrderStatusClosed:
		return WorkOrderStatusClosed, nil
	case WorkOrderStatusCancelled:
		return WorkOrderStatusCancelled, nil
	default:
		return "", fmt.Errorf("unsupported graph workflow end result %q", result)
	}
}

func (s *WorkOrderService) createGraphNodeInstance(tx *gorm.DB, order *store.WorkOrder, node WorkOrderWorkflowNode, branchID, status string) (*store.WorkOrderNodeInstance, bool, error) {
	now := time.Now().UTC()
	instance := store.WorkOrderNodeInstance{
		TenantID: order.TenantID, ProjectID: order.ProjectID, WorkOrderID: order.ID,
		WorkflowCycle: order.WorkflowCycle, NodeID: node.ID, NodeType: node.Type,
		BranchID: strings.TrimSpace(branchID), Status: status, ActivatedAt: now,
	}
	if status == WorkOrderNodeInstanceCompleted {
		instance.CompletedAt = &now
	}
	if err := tx.Create(&instance).Error; err != nil {
		if !isUniqueConstraintError(err) {
			return nil, false, fmt.Errorf("create work order node instance: %w", err)
		}
		if err := tx.Where("work_order_id = ? AND workflow_cycle = ? AND node_id = ? AND branch_id = ?", order.ID, order.WorkflowCycle, node.ID, instance.BranchID).First(&instance).Error; err != nil {
			return nil, false, fmt.Errorf("load existing work order node instance: %w", err)
		}
		return &instance, false, nil
	}
	return &instance, true, nil
}

func (s *WorkOrderService) loadGraphTaskParticipants(tx *gorm.DB, order *store.WorkOrder, userIDs []uint) ([]store.User, error) {
	ids := make([]uint, 0, len(userIDs))
	seen := make(map[uint]bool, len(userIDs))
	for _, userID := range userIDs {
		if userID != 0 && !seen[userID] {
			ids = append(ids, userID)
			seen[userID] = true
		}
	}
	var users []store.User
	if err := tx.Model(&store.User{}).
		Joins("JOIN user_role_bindings AS participant_bindings ON participant_bindings.user_id = users.id AND participant_bindings.tenant_id = users.tenant_id").
		Where("users.tenant_id = ? AND users.status = ? AND users.id IN ? AND (participant_bindings.project_id = ? OR participant_bindings.project_id = 0)", order.TenantID, 1, ids, order.ProjectID).
		Select("users.*").Distinct().Find(&users).Error; err != nil {
		return nil, fmt.Errorf("load graph workflow participants: %w", err)
	}
	byUserID := make(map[uint]store.User, len(users))
	for _, user := range users {
		byUserID[user.ID] = user
	}
	ordered := make([]store.User, 0, len(ids))
	for _, userID := range ids {
		user, ok := byUserID[userID]
		if !ok {
			return nil, fmt.Errorf("workflow participant %d is not an active member of this project", userID)
		}
		ordered = append(ordered, user)
	}
	return ordered, nil
}

func (s *WorkOrderService) createGraphTaskNotification(tx *gorm.DB, order *store.WorkOrder, node WorkOrderWorkflowNode, task store.WorkOrderTask) error {
	title := "New work order task"
	if node.TaskKind == WorkOrderTaskKindApprove {
		title = "Work order approval required"
	}
	notification := store.WorkOrderNotification{
		TenantID:          order.TenantID,
		ProjectID:         order.ProjectID,
		RecipientUserID:   task.AssigneeUserID,
		WorkOrderID:       order.ID,
		WorkOrderPublicID: order.PublicID,
		WorkOrderCode:     order.Code,
		WorkOrderTitle:    order.Title,
		WorkOrderTaskID:   task.ID,
		TaskPublicID:      task.PublicID,
		NodeID:            node.ID,
		TaskKind:          node.TaskKind,
		Type:              WorkOrderNotificationTask,
		Title:             title,
		Content:           fmt.Sprintf("%s: %s", order.Code, node.Name),
		Status:            WorkOrderNotificationUnread,
	}
	if err := tx.Create(&notification).Error; err != nil {
		return fmt.Errorf("create work order task notification: %w", err)
	}
	return nil
}

func (s *WorkOrderService) updateGraphWorkOrderState(tx *gorm.DB, order *store.WorkOrder, status string, workflowCycle int, closedAt *time.Time) error {
	previousVersion := order.Version
	if previousVersion <= 0 {
		previousVersion = 1
	}
	query := tx.Model(&store.WorkOrder{}).Where("id = ? AND tenant_id = ? AND project_id = ? AND version = ?", order.ID, order.TenantID, order.ProjectID, previousVersion).
		Updates(map[string]any{"status": status, "workflow_cycle": workflowCycle, "closed_at": closedAt, "version": previousVersion + 1})
	if query.Error != nil {
		return fmt.Errorf("save graph work order state: %w", query.Error)
	}
	if query.RowsAffected != 1 {
		return ErrWorkOrderVersionConflict
	}
	order.Status = status
	order.WorkflowCycle = workflowCycle
	order.ClosedAt = closedAt
	order.Version = previousVersion + 1
	return nil
}

func (s *WorkOrderService) findWorkOrderTaskByPublicID(db *gorm.DB, scope WorkOrderScope, publicID string) (*store.WorkOrderTask, error) {
	publicID = strings.TrimSpace(publicID)
	if publicID == "" {
		return nil, fmt.Errorf("work order task public id is required")
	}
	var task store.WorkOrderTask
	if err := db.Where("public_id = ? AND tenant_id = ? AND project_id = ?", publicID, scope.TenantID, scope.ProjectID).First(&task).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("work order task not found")
		}
		return nil, fmt.Errorf("load work order task: %w", err)
	}
	return &task, nil
}

func (s *WorkOrderService) appendWorkOrderEvent(tx *gorm.DB, order *store.WorkOrder, eventType string, actorUserID uint, payload map[string]any) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode work order event payload: %w", err)
	}
	var last store.WorkOrderEvent
	sequence := 1
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND project_id = ? AND work_order_id = ?", order.TenantID, order.ProjectID, order.ID).Order("sequence DESC").First(&last).Error; err == nil {
		sequence = last.Sequence + 1
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read work order event sequence: %w", err)
	}
	event := store.WorkOrderEvent{
		TenantID:    order.TenantID,
		ProjectID:   order.ProjectID,
		WorkOrderID: order.ID,
		Sequence:    sequence,
		Type:        eventType,
		ActorUserID: actorUserID,
		Payload:     string(encoded),
	}
	if err := tx.Create(&event).Error; err != nil {
		return fmt.Errorf("create work order event: %w", err)
	}
	return s.enqueueWorkOrderOutboxEvent(tx, order, event, payload)
}

func (s *WorkOrderService) enqueueWorkOrderOutboxEvent(tx *gorm.DB, order *store.WorkOrder, event store.WorkOrderEvent, eventPayload map[string]any) error {
	eventID := fmt.Sprintf("work-order:%d:event:%d", order.ID, event.Sequence)
	payload := map[string]any{
		"specversion":     "1.0",
		"type":            "com.noyo.work-order." + event.Type,
		"source":          "/noyo/work-orders",
		"id":              eventID,
		"time":            event.CreatedAt.UTC().Format(time.RFC3339Nano),
		"subject":         order.Code,
		"datacontenttype": "application/json",
		"data": map[string]any{
			"work_order": map[string]any{
				"id": order.ID, "code": order.Code, "tenant_id": order.TenantID, "project_id": order.ProjectID,
				"template_id": order.TemplateID, "status": order.Status, "priority": order.Priority,
				"source_type": order.SourceType, "source_id": order.SourceID,
			},
			"event": map[string]any{
				"sequence": event.Sequence, "type": event.Type, "actor_user_id": event.ActorUserID, "payload": eventPayload,
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode work order outbox event: %w", err)
	}
	outbox := store.WorkOrderOutboxEvent{
		TenantID:    order.TenantID,
		ProjectID:   order.ProjectID,
		WorkOrderID: order.ID,
		EventID:     eventID,
		EventType:   "com.noyo.work-order." + event.Type,
		Payload:     string(encoded),
		Status:      WorkOrderOutboxPending,
		AvailableAt: time.Now().UTC(),
	}
	if err := tx.Create(&outbox).Error; err != nil {
		return fmt.Errorf("create work order outbox event: %w", err)
	}
	return nil
}

func (s *WorkOrderService) upsertSourceLink(tx *gorm.DB, order *store.WorkOrder, source WorkOrderSource) error {
	metadata, err := json.Marshal(map[string]any{
		"idempotency_key": source.IdempotencyKey,
		"snapshot":        source.Snapshot,
	})
	if err != nil {
		return fmt.Errorf("encode source link metadata: %w", err)
	}
	link := store.WorkOrderLink{
		TenantID:     order.TenantID,
		ProjectID:    order.ProjectID,
		WorkOrderID:  order.ID,
		RelationType: source.Type,
		ExternalRef:  source.ID,
		Metadata:     string(metadata),
	}
	if err := tx.Create(&link).Error; err != nil {
		return fmt.Errorf("create work order source link: %w", err)
	}
	return nil
}

func decodeWorkOrderFormDefinition(encoded string) (WorkOrderFormDefinition, error) {
	var definition WorkOrderFormDefinition
	if err := json.Unmarshal([]byte(encoded), &definition); err != nil {
		return WorkOrderFormDefinition{}, fmt.Errorf("decode form definition: %w", err)
	}
	if err := ValidateWorkOrderFormDefinition(definition); err != nil {
		return WorkOrderFormDefinition{}, fmt.Errorf("stored form definition is invalid: %w", err)
	}
	return definition, nil
}

func decodeWorkOrderFormDraft(encoded string) (WorkOrderFormDefinition, error) {
	var definition WorkOrderFormDefinition
	if err := json.Unmarshal([]byte(encoded), &definition); err != nil {
		return WorkOrderFormDefinition{}, fmt.Errorf("decode form draft: %w", err)
	}
	return definition, nil
}

func decodeWorkOrderWorkflowDefinition(encoded string) (WorkOrderWorkflowDefinition, error) {
	var definition WorkOrderWorkflowDefinition
	if err := json.Unmarshal([]byte(encoded), &definition); err != nil {
		return WorkOrderWorkflowDefinition{}, fmt.Errorf("decode workflow definition: %w", err)
	}
	if err := ValidateWorkOrderWorkflowDefinition(definition); err != nil {
		return WorkOrderWorkflowDefinition{}, fmt.Errorf("stored workflow definition is invalid: %w", err)
	}
	return definition, nil
}

func decodeWorkOrderWorkflowDraft(encoded string) (WorkOrderWorkflowDefinition, error) {
	var definition WorkOrderWorkflowDefinition
	if err := json.Unmarshal([]byte(encoded), &definition); err != nil {
		return WorkOrderWorkflowDefinition{}, fmt.Errorf("decode workflow draft: %w", err)
	}
	return definition, nil
}

func normalizeWorkOrderWorkflowDefinition(definition WorkOrderWorkflowDefinition) WorkOrderWorkflowDefinition {
	if definition.SchemaVersion != WorkOrderWorkflowSchemaV2 {
		return definition
	}
	definition.Nodes = append([]WorkOrderWorkflowNode(nil), definition.Nodes...)
	definition.Edges = append([]WorkOrderWorkflowEdge(nil), definition.Edges...)
	nodesByID := make(map[string]WorkOrderWorkflowNode, len(definition.Nodes))
	incomingByTarget := make(map[string][]string, len(definition.Nodes))
	outgoingBySource := make(map[string][]WorkOrderWorkflowEdge, len(definition.Nodes))
	for _, node := range definition.Nodes {
		nodesByID[node.ID] = node
	}
	for _, node := range definition.Nodes {
		if node.Type != WorkOrderWorkflowNodeParallelSplit {
			continue
		}
		usedBranchIDs := make(map[string]bool)
		preservedEdgeIndexes := make(map[int]bool)
		for edgeIndex := range definition.Edges {
			edge := definition.Edges[edgeIndex]
			branchID := strings.TrimSpace(edge.BranchID)
			if edge.Source == node.ID && isWorkOrderIdentifier(branchID) && !usedBranchIDs[branchID] {
				usedBranchIDs[branchID] = true
				preservedEdgeIndexes[edgeIndex] = true
			}
		}
		nextBranchNumber := 1
		for edgeIndex := range definition.Edges {
			edge := &definition.Edges[edgeIndex]
			if edge.Source != node.ID {
				continue
			}
			if preservedEdgeIndexes[edgeIndex] {
				edge.BranchID = strings.TrimSpace(edge.BranchID)
				continue
			}
			branchID := strings.TrimSpace(edge.BranchID)
			for {
				branchID = fmt.Sprintf("branch_%d", nextBranchNumber)
				nextBranchNumber++
				if !usedBranchIDs[branchID] {
					break
				}
			}
			edge.BranchID = branchID
			usedBranchIDs[branchID] = true
		}
	}
	for _, edge := range definition.Edges {
		incomingByTarget[edge.Target] = append(incomingByTarget[edge.Target], edge.Source)
		outgoingBySource[edge.Source] = append(outgoingBySource[edge.Source], edge)
	}
	parallelSplitByBranchNode := workOrderWorkflowParallelSplitByBranchNode(nodesByID, outgoingBySource)
	for index := range definition.Nodes {
		node := &definition.Nodes[index]
		if node.Type != WorkOrderWorkflowNodeUserTask || node.TaskKind != WorkOrderTaskKindApprove {
			continue
		}
		candidates := workOrderWorkflowRejectTargetCandidates(node.ID, nodesByID, incomingByTarget, parallelSplitByBranchNode)
		isValidTarget := false
		for _, candidate := range candidates {
			if candidate.ID == node.RejectTargetNodeID {
				isValidTarget = true
				break
			}
		}
		if !isValidTarget {
			if len(candidates) > 0 {
				node.RejectTargetNodeID = candidates[0].ID
			} else {
				node.RejectTargetNodeID = ""
			}
		}
	}
	return definition
}

func workOrderWorkflowParallelSplitByBranchNode(nodesByID map[string]WorkOrderWorkflowNode, outgoingBySource map[string][]WorkOrderWorkflowEdge) map[string]string {
	joinByPairID := make(map[string]string)
	for _, node := range nodesByID {
		if node.Type == WorkOrderWorkflowNodeParallelJoin {
			joinByPairID[node.PairID] = node.ID
		}
	}
	parallelSplitByBranchNode := make(map[string]string)
	splitIDs := make([]string, 0)
	for _, node := range nodesByID {
		if node.Type == WorkOrderWorkflowNodeParallelSplit {
			splitIDs = append(splitIDs, node.ID)
		}
	}
	sort.Strings(splitIDs)
	for _, splitID := range splitIDs {
		split := nodesByID[splitID]
		joinID := joinByPairID[split.PairID]
		queue := make([]string, 0, len(outgoingBySource[split.ID]))
		for _, edge := range outgoingBySource[split.ID] {
			queue = append(queue, edge.Target)
		}
		visited := map[string]bool{split.ID: true}
		for len(queue) > 0 {
			currentID := queue[0]
			queue = queue[1:]
			if currentID == joinID || visited[currentID] {
				continue
			}
			visited[currentID] = true
			parallelSplitByBranchNode[currentID] = split.ID
			for _, edge := range outgoingBySource[currentID] {
				queue = append(queue, edge.Target)
			}
		}
	}
	return parallelSplitByBranchNode
}

func workOrderWorkflowRejectTargetCandidates(nodeID string, nodesByID map[string]WorkOrderWorkflowNode, incomingByTarget map[string][]string, parallelSplitByBranchNode map[string]string) []WorkOrderWorkflowNode {
	candidates := make([]WorkOrderWorkflowNode, 0)
	candidateIDs := make(map[string]bool)
	visited := map[string]bool{nodeID: true}
	restartFromID := nodeID
	if splitID := parallelSplitByBranchNode[nodeID]; splitID != "" {
		if split, ok := nodesByID[splitID]; ok {
			candidates = append(candidates, split)
			candidateIDs[splitID] = true
			visited[splitID] = true
			restartFromID = splitID
		}
	}
	queue := append([]string(nil), incomingByTarget[restartFromID]...)
	splitByPairID := make(map[string]string)
	for _, node := range nodesByID {
		if node.Type == WorkOrderWorkflowNodeParallelSplit {
			splitByPairID[node.PairID] = node.ID
		}
	}
	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		if visited[currentID] {
			continue
		}
		visited[currentID] = true
		node, ok := nodesByID[currentID]
		if !ok {
			continue
		}
		if node.Type == WorkOrderWorkflowNodeParallelJoin {
			if splitID := splitByPairID[node.PairID]; splitID != "" && !candidateIDs[splitID] {
				if split, exists := nodesByID[splitID]; exists {
					candidates = append(candidates, split)
					candidateIDs[splitID] = true
					queue = append(queue, incomingByTarget[splitID]...)
				}
			}
			continue
		}
		if node.Type == WorkOrderWorkflowNodeStart || node.Type == WorkOrderWorkflowNodeUserTask || node.Type == WorkOrderWorkflowNodeParallelSplit {
			if !candidateIDs[node.ID] {
				candidates = append(candidates, node)
				candidateIDs[node.ID] = true
			}
		}
		queue = append(queue, incomingByTarget[currentID]...)
	}
	return candidates
}

func (definition WorkOrderWorkflowDefinition) transitionByKey(key string) (WorkOrderWorkflowTransition, bool) {
	for _, transition := range definition.Transitions {
		if transition.Key == key {
			return transition, true
		}
	}
	return WorkOrderWorkflowTransition{}, false
}

func (definition WorkOrderWorkflowDefinition) approvalTransitionFrom(status string) (WorkOrderWorkflowTransition, bool) {
	for _, transition := range definition.Transitions {
		if transition.From == status && len(transition.ApproverUserIDs) > 0 {
			return transition, true
		}
	}
	return WorkOrderWorkflowTransition{}, false
}

func (definition WorkOrderWorkflowDefinition) statusByKey(key string) (WorkOrderWorkflowStatus, bool) {
	for _, status := range definition.Statuses {
		if status.Key == key {
			return status, true
		}
	}
	return WorkOrderWorkflowStatus{}, false
}

func validateWorkOrderScope(scope WorkOrderScope) error {
	if scope.TenantID == 0 || scope.ProjectID == 0 {
		return fmt.Errorf("tenant and project scope are required")
	}
	return nil
}

func normalizeWorkOrderSource(source WorkOrderSource) (WorkOrderSource, error) {
	source.Type = strings.TrimSpace(source.Type)
	source.ID = strings.TrimSpace(source.ID)
	source.IdempotencyKey = strings.TrimSpace(source.IdempotencyKey)
	if !isSupportedWorkOrderSource(source.Type) || source.ID == "" || source.IdempotencyKey == "" {
		return WorkOrderSource{}, fmt.Errorf("supported source type, source id, and idempotency key are required")
	}
	if len(source.ID) > 191 || len(source.IdempotencyKey) > 191 {
		return WorkOrderSource{}, fmt.Errorf("source id and idempotency key must not exceed 191 characters")
	}
	return source, nil
}

func normalizeWorkOrderPriority(priority string) string {
	switch strings.TrimSpace(priority) {
	case "low", "normal", "high", "urgent":
		return strings.TrimSpace(priority)
	default:
		return "normal"
	}
}

func normalizeWorkOrderResolution(resolution WorkOrderResolution) (WorkOrderResolution, error) {
	resolution.ActualProblem = strings.TrimSpace(resolution.ActualProblem)
	resolution.RootCause = strings.TrimSpace(resolution.RootCause)
	resolution.HandlingProcess = strings.TrimSpace(resolution.HandlingProcess)
	resolution.HandlingResult = strings.TrimSpace(resolution.HandlingResult)
	if resolution.ActualProblem == "" || resolution.RootCause == "" || resolution.HandlingProcess == "" || resolution.HandlingResult == "" {
		return WorkOrderResolution{}, fmt.Errorf("actual problem, root cause, handling process, and handling result are required to resolve a work order")
	}
	attachmentIDs := make([]string, 0, len(resolution.AttachmentIDs))
	for _, attachmentID := range resolution.AttachmentIDs {
		attachmentID = strings.TrimSpace(attachmentID)
		if attachmentID != "" {
			attachmentIDs = append(attachmentIDs, attachmentID)
		}
	}
	resolution.AttachmentIDs = attachmentIDs
	return resolution, nil
}

func nextWorkOrderCode() string {
	return "WO-" + strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:16])
}

func isUniqueConstraintError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique constraint") || strings.Contains(message, "duplicate key")
}

func isWorkOrderIdentifier(value string) bool {
	if len(value) < 2 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func isSupportedWorkOrderSource(value string) bool {
	switch value {
	case WorkOrderSourceManual, WorkOrderSourceRule, WorkOrderSourceAlarm, WorkOrderSourceAI, WorkOrderSourceExternal:
		return true
	default:
		return false
	}
}

func defaultDeviceMaintenanceFormDefinition() WorkOrderFormDefinition {
	return WorkOrderFormDefinition{Fields: []WorkOrderFormField{
		{Key: "device_code", Label: "设备编码", Type: WorkOrderFormFieldText, Required: true},
		{Key: "fault_type", Label: "故障类型", Type: WorkOrderFormFieldText, Required: true},
		{Key: "severity", Label: "严重程度", Type: WorkOrderFormFieldSelect, Required: true, Options: []string{"low", "normal", "high", "urgent"}, DefaultValue: "normal"},
		{Key: "description", Label: "故障描述", Type: WorkOrderFormFieldTextarea},
		{Key: "diagnostic_checks", Label: "诊断检查项", Type: WorkOrderFormFieldTextarea},
		{Key: "ai_suggestion_id", Label: "AI 建议编号", Type: WorkOrderFormFieldText, Required: true},
	}}
}

func defaultWorkOrderWorkflowDefinition() WorkOrderWorkflowDefinition {
	definition := WorkOrderWorkflowDefinition{
		InitialStatus: "open",
		Statuses: []WorkOrderWorkflowStatus{
			{Key: "open", Name: "待处理", Category: WorkOrderStatusOpen},
			{Key: "in_progress", Name: "处理中", Category: WorkOrderStatusInProgress},
			{Key: "resolved", Name: "已解决", Category: WorkOrderStatusResolved},
			{Key: "cancelled", Name: "已取消", Category: WorkOrderStatusCancelled},
			{Key: "paused", Name: "暂停处理", Category: WorkOrderStatusInProgress},
			{Key: "closed", Name: "已归档", Category: WorkOrderStatusClosed},
		},
		Transitions: []WorkOrderWorkflowTransition{
			{Key: "pause", Name: "暂停处理", From: "in_progress", To: "paused"},
			{Key: "resume", Name: "恢复处理", From: "paused", To: "in_progress"},
			{Key: "close", Name: "验收并归档", From: "resolved", To: "closed"},
			{Key: "reopen", Name: "撤销归档", From: "closed", To: "open"},
			{Key: "reopen_resolved", Name: "驳回解决", From: "resolved", To: "open"},
			{Key: "cancel_paused", Name: "取消工单", From: "paused", To: "cancelled"},
			{Key: "start", Name: "开始处理", From: "open", To: "in_progress"},
			{Key: "resolve", Name: "完成处理", From: "in_progress", To: "resolved"},
			{Key: "cancel_open", Name: "取消工单", From: "open", To: "cancelled"},
			{Key: "cancel_processing", Name: "取消工单", From: "in_progress", To: "cancelled"},
		},
	}
	for index := range definition.Transitions {
		if definition.Transitions[index].Key == "resolve" {
			definition.Transitions[index].RequireResolution = true
		}
	}
	return definition
}

func defaultDeviceMaintenanceWorkflowDefinition() WorkOrderWorkflowDefinition {
	return defaultWorkOrderWorkflowDefinition()
}

func isSupportedWorkOrderFormFieldType(value string) bool {
	switch value {
	case WorkOrderFormFieldText, WorkOrderFormFieldTextarea, WorkOrderFormFieldNumber, WorkOrderFormFieldInteger, WorkOrderFormFieldBoolean,
		WorkOrderFormFieldSelect, WorkOrderFormFieldMultiSelect, WorkOrderFormFieldDate, WorkOrderFormFieldDateTime, WorkOrderFormFieldDevice,
		WorkOrderFormFieldUser, WorkOrderFormFieldImages, WorkOrderFormFieldAttachment:
		return true
	default:
		return false
	}
}

func isSupportedWorkOrderStatusCategory(value string) bool {
	switch value {
	case WorkOrderStatusOpen, WorkOrderStatusInProgress, WorkOrderStatusResolved, WorkOrderStatusClosed, WorkOrderStatusCancelled:
		return true
	default:
		return false
	}
}

func isClosedWorkOrderStatus(category string) bool {
	return category == WorkOrderStatusClosed || category == WorkOrderStatusCancelled
}

func isTerminalWorkOrderStatus(category string) bool {
	return category == WorkOrderStatusResolved || isClosedWorkOrderStatus(category)
}

func workOrderFormValuePresent(value any) bool {
	if value == nil {
		return false
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) != ""
	}
	if values, ok := value.([]any); ok {
		return len(values) > 0
	}
	if values, ok := value.([]string); ok {
		return len(values) > 0
	}
	return true
}

func validateWorkOrderFieldValue(field WorkOrderFormField, value any) error {
	switch field.Type {
	case WorkOrderFormFieldText, WorkOrderFormFieldTextarea, WorkOrderFormFieldDevice, WorkOrderFormFieldUser:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("must be a string")
		}
	case WorkOrderFormFieldNumber:
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		default:
			return fmt.Errorf("must be a number")
		}
	case WorkOrderFormFieldInteger:
		if !isWorkOrderIntegerValue(value) {
			return fmt.Errorf("must be an integer")
		}
	case WorkOrderFormFieldBoolean:
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("must be a boolean")
		}
	case WorkOrderFormFieldSelect:
		option, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a string option")
		}
		for _, candidate := range field.Options {
			if option == candidate {
				return nil
			}
		}
		return fmt.Errorf("must match a configured option")
	case WorkOrderFormFieldMultiSelect:
		var values []string
		switch raw := value.(type) {
		case []string:
			values = raw
		case []any:
			values = make([]string, 0, len(raw))
			for _, item := range raw {
				text, ok := item.(string)
				if !ok {
					return fmt.Errorf("must contain string options")
				}
				values = append(values, text)
			}
		default:
			return fmt.Errorf("must be an array of configured options")
		}
		allowed := make(map[string]bool, len(field.Options))
		for _, option := range field.Options {
			allowed[option] = true
		}
		selected := make(map[string]bool, len(values))
		for _, option := range values {
			if !allowed[option] || selected[option] {
				return fmt.Errorf("must contain distinct configured options")
			}
			selected[option] = true
		}
	case WorkOrderFormFieldDate:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be a date string")
		}
		if _, err := time.Parse("2006-01-02", text); err != nil {
			return fmt.Errorf("must use YYYY-MM-DD")
		}
	case WorkOrderFormFieldDateTime:
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("must be an RFC3339 date-time string")
		}
		if _, err := time.Parse(time.RFC3339, text); err != nil {
			return fmt.Errorf("must use RFC3339")
		}
	case WorkOrderFormFieldAttachment:
		switch value.(type) {
		case string, []string, []any:
		default:
			return fmt.Errorf("must be an attachment reference or list")
		}
	case WorkOrderFormFieldImages:
		var references []string
		switch raw := value.(type) {
		case []string:
			references = raw
		case []any:
			references = make([]string, 0, len(raw))
			for _, item := range raw {
				reference, ok := item.(string)
				if !ok {
					return fmt.Errorf("must contain image references")
				}
				references = append(references, reference)
			}
		default:
			return fmt.Errorf("must be an array of image references")
		}
		seen := make(map[string]bool, len(references))
		for _, reference := range references {
			reference = strings.TrimSpace(reference)
			if reference == "" || seen[reference] {
				return fmt.Errorf("must contain distinct non-empty image references")
			}
			seen[reference] = true
		}
	}
	return nil
}

func ensureLinkedAlarmsReadyToClose(tx *gorm.DB, order *store.WorkOrder) error {
	var alarms []store.AlarmInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND project_id = ? AND work_order_public_id = ? AND handling_status <> ?", order.TenantID, order.ProjectID, order.PublicID, AlarmHandlingClosed).Find(&alarms).Error; err != nil {
		return fmt.Errorf("load linked alarms before closing work order: %w", err)
	}
	now := time.Now().UTC()
	for _, alarm := range alarms {
		if normalizedCondition(alarm.ConditionStatus) != AlarmConditionRecovered {
			return fmt.Errorf("linked alarm %s is still firing and blocks work order closure", alarm.PublicID)
		}
		if alarm.VerificationAfter != nil && now.Before(*alarm.VerificationAfter) {
			return fmt.Errorf("linked alarm %s is still within its recovery verification window", alarm.PublicID)
		}
	}
	return nil
}

func closeLinkedAlarmsForWorkOrder(tx *gorm.DB, order *store.WorkOrder, actorUserID uint, comment string) error {
	var alarms []store.AlarmInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND project_id = ? AND work_order_public_id = ? AND handling_status <> ?", order.TenantID, order.ProjectID, order.PublicID, AlarmHandlingClosed).Find(&alarms).Error; err != nil {
		return fmt.Errorf("load linked alarms after closing work order: %w", err)
	}
	now := time.Now().UTC()
	for index := range alarms {
		alarm := &alarms[index]
		if err := updateAlarmInstance(tx, alarm, map[string]any{
			"condition_status":   AlarmConditionRecovered,
			"handling_status":    AlarmHandlingClosed,
			"recovered_at":       &now,
			"verification_after": nil,
			"closed_at":          &now,
			"closed_by":          actorUserID,
			"close_disposition":  AlarmCloseWorkOrder,
			"status":             workorderport.AlarmStatusCleared,
		}); err != nil {
			return fmt.Errorf("close linked alarm %s: %w", alarm.PublicID, err)
		}
		if err := appendAlarmCenterEvent(tx, alarm, workorderport.AlarmEventClosed, map[string]any{
			"disposition":   AlarmCloseWorkOrder,
			"comment":       comment,
			"actor_user_id": actorUserID,
			"work_order_id": order.PublicID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func isWorkOrderIntegerValue(value any) bool {
	switch number := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		parsed := float64(number)
		return !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed == math.Trunc(parsed)
	case float64:
		return !math.IsNaN(number) && !math.IsInf(number, 0) && number == math.Trunc(number)
	case json.Number:
		parsed, err := number.Float64()
		return err == nil && !math.IsNaN(parsed) && !math.IsInf(parsed, 0) && parsed == math.Trunc(parsed)
	default:
		return false
	}
}
