package core

import (
	"encoding/json"
	"errors"
	"fmt"

	"noyo/core/store"

	"gorm.io/gorm"
)

// InspectionReportWorkOrderTemplateCode identifies the built-in template that
// receives video inspection findings. Its field keys are a contract with the
// inspection plugin, which fills them by name when it raises an order:
// device_code, device_name, description and the first images field receive the
// camera, the finding text and the captured evidence.
const InspectionReportWorkOrderTemplateCode = "inspection_report"

func inspectionReportFormDefinition() WorkOrderFormDefinition {
	return WorkOrderFormDefinition{Fields: []WorkOrderFormField{
		{Key: "device_code", Label: "设备编码", Type: WorkOrderFormFieldText, Required: true},
		{Key: "device_name", Label: "设备名称", Type: WorkOrderFormFieldText},
		{Key: "description", Label: "巡检结果", Type: WorkOrderFormFieldTextarea, Required: true},
		{Key: "inspection_images", Label: "现场截图", Type: WorkOrderFormFieldImages},
		{Key: "handle_note", Label: "处理说明", Type: WorkOrderFormFieldTextarea},
	}}
}

// inspectionReportWorkflowDefinition publishes a ready-to-use flow instead of a
// draft: inspection work orders are raised unattended, so a template that still
// needs a manual publish step would produce orders nobody can process.
func inspectionReportWorkflowDefinition(participantIDs []uint) WorkOrderWorkflowDefinition {
	handle := WorkOrderWorkflowNode{
		ID:                 "handle",
		Type:               WorkOrderWorkflowNodeUserTask,
		Name:               "处理任务",
		TaskKind:           "handle",
		ParticipantUserIDs: participantIDs,
		CompletionMode:     WorkOrderApprovalAll,
	}
	return WorkOrderWorkflowDefinition{
		SchemaVersion: WorkOrderWorkflowSchemaV2,
		StartNodeID:   "start",
		Nodes: []WorkOrderWorkflowNode{
			{ID: "start", Type: WorkOrderWorkflowNodeStart, Name: "开始"},
			handle,
			{ID: "end", Type: WorkOrderWorkflowNodeEnd, Name: "结束", Result: "completed"},
		},
		Edges: []WorkOrderWorkflowEdge{
			{ID: "edge-start-handle", Source: "start", Target: "handle"},
			{ID: "edge-handle-end", Source: "handle", Target: "end"},
		},
		Layout: map[string]any{"nodes": []map[string]any{
			{"id": "start", "x": 250, "y": 50},
			{"id": "handle", "x": 250, "y": 200},
			{"id": "end", "x": 250, "y": 350},
		}},
	}
}

// inspectionReportParticipantIDs picks the default handler for the built-in
// inspection flow: the longest-standing enabled member of the tenant. The graph
// validator requires a concrete participant, so a tenant without an active user
// cannot get a usable flow; the caller reports that instead of publishing a task
// nobody could handle.
func inspectionReportParticipantIDs(tx *gorm.DB, scope WorkOrderScope) []uint {
	var ids []uint
	if err := tx.Model(&store.User{}).
		Where("tenant_id = ? AND status = ?", scope.TenantID, 1).
		Order("id").Limit(1).Pluck("id", &ids).Error; err != nil {
		return nil
	}
	return ids
}

// EnsureInspectionReportTemplate makes the built-in inspection template exist
// with a published form and flow. It is idempotent and never overwrites a
// template the operator has already customised: only missing pieces are filled
// in, so enabling the plugin repeatedly stays safe.
func (s *WorkOrderService) EnsureInspectionReportTemplate(scope WorkOrderScope) (*store.WorkOrderTemplate, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	formDefinition := inspectionReportFormDefinition()
	if err := ValidateWorkOrderFormDefinition(formDefinition); err != nil {
		return nil, err
	}
	compiled, err := compileLegacyWorkOrderForm(formDefinition)
	if err != nil {
		return nil, err
	}
	encodedForm, err := json.Marshal(formDefinition)
	if err != nil {
		return nil, fmt.Errorf("encode inspection report form: %w", err)
	}

	var template store.WorkOrderTemplate
	err = s.db.Transaction(func(tx *gorm.DB) error {
		loadErr := tx.Where("tenant_id = ? AND project_id = ? AND code = ?", scope.TenantID, scope.ProjectID, InspectionReportWorkOrderTemplateCode).
			First(&template).Error
		switch {
		case errors.Is(loadErr, gorm.ErrRecordNotFound):
			template = store.WorkOrderTemplate{
				TenantID:    scope.TenantID,
				ProjectID:   scope.ProjectID,
				Code:        InspectionReportWorkOrderTemplateCode,
				Name:        "巡检上报",
				Description: "视频巡检问题工单，自动带出设备、巡检结果与现场截图。",
				Enabled:     true,
			}
			if err := tx.Create(&template).Error; err != nil {
				return fmt.Errorf("create inspection report template: %w", err)
			}
		case loadErr != nil:
			return fmt.Errorf("load inspection report template: %w", loadErr)
		}

		// Publish the real definition as version 1 rather than the empty default,
		// so the first inspection order already carries the contract keys.
		if template.CurrentFormVersion == 0 {
			version := store.WorkOrderFormVersion{
				TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: template.ID,
				Version: 1, Definition: string(encodedForm), Checksum: compiled.Checksum,
				SchemaJSON: string(compiledSnapshot(compiled, "schema")), UISchemaJSON: string(compiledSnapshot(compiled, "ui_schema")),
				DefaultsJSON: string(compiledSnapshot(compiled, "defaults")), RequiredJSON: string(compiledSnapshot(compiled, "required")),
				FieldOrderJSON: string(compiledSnapshot(compiled, "field_order")), ResourceFieldsJSON: string(compiledSnapshot(compiled, "resource_fields")),
				PublishedBy: scope.ActorUserID,
			}
			if err := tx.Create(&version).Error; err != nil {
				return fmt.Errorf("create inspection report form version: %w", err)
			}
			template.CurrentFormVersion = version.Version
		}

		if template.CurrentWorkflowVersion == 0 {
			participantIDs := inspectionReportParticipantIDs(tx, scope)
			if len(participantIDs) == 0 {
				return fmt.Errorf("inspection report template needs at least one active user in tenant %d to own the handling task", scope.TenantID)
			}
			workflowDefinition := inspectionReportWorkflowDefinition(participantIDs)
			if err := ValidateWorkOrderWorkflowDefinition(workflowDefinition); err != nil {
				return err
			}
			encodedWorkflow, err := json.Marshal(workflowDefinition)
			if err != nil {
				return fmt.Errorf("encode inspection report workflow: %w", err)
			}
			if err := tx.Create(&store.WorkOrderWorkflowVersion{
				TenantID: scope.TenantID, ProjectID: scope.ProjectID, TemplateID: template.ID,
				Version: 1, Definition: string(encodedWorkflow), PublishedBy: scope.ActorUserID,
			}).Error; err != nil {
				return fmt.Errorf("create inspection report workflow version: %w", err)
			}
			template.CurrentWorkflowVersion = 1
		}

		return tx.Save(&template).Error
	})
	if err != nil {
		return nil, err
	}
	return &template, nil
}
