package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"noyo/core/store"
	"noyo/core/workorder"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strconv"
)

func (s *Server) RegisterWorkOrderRoutes(group *ghttp.RouterGroup) {
	permissionGET(group, "/work-order-participants", "work_order:list", s.handleListWorkOrderParticipants)
	permissionGET(group, "/work-order-templates", "work_order:list", s.handleListWorkOrderTemplates)
	permissionPOST(group, "/work-order-templates", "work_order:manage", s.handleCreateWorkOrderTemplate)
	permissionPOST(group, "/work-order-templates/:id/copy", "work_order:manage", s.handleCopyWorkOrderTemplate)
	permissionGET(group, "/work-order-templates/:id", "work_order:list", s.handleGetWorkOrderTemplate)
	permissionPUT(group, "/work-order-templates/:id/form-draft", "work_order:manage", s.handleSaveWorkOrderFormDraft)
	permissionPOST(group, "/work-order-templates/:id/form-versions", "work_order:manage", s.handlePublishWorkOrderFormVersion)
	permissionPOST(group, "/work-order-templates/:id/form-schema/validate", "work_order:manage", s.handleValidateWorkOrderFormSchema)
	permissionPUT(group, "/work-order-templates/:id/workflow-draft", "work_order:manage", s.handleSaveWorkOrderWorkflowDraft)
	permissionPOST(group, "/work-order-templates/:id/workflow-versions", "work_order:manage", s.handlePublishWorkOrderWorkflowVersion)

	permissionPUT(group, "/work-order-templates/:id", "work_order:manage", s.handleUpdateWorkOrderTemplate)
	permissionDELETE(group, "/work-order-templates/:id", "work_order:manage", s.handleDeleteWorkOrderTemplate)
	permissionPUT(group, "/work-orders/:id", "work_order:process", s.handleUpdateWorkOrder)
	permissionDELETE(group, "/work-orders/:id", "work_order:manage", s.handleDeleteWorkOrder)
	permissionGET(group, "/work-orders", "work_order:list", s.handleListWorkOrders)
	permissionPOST(group, "/work-orders", "work_order:create", s.handleCreateWorkOrder)
	permissionPOST(group, "/work-orders/images", "work_order:create", s.handleUploadWorkOrderImage)
	permissionPOST(group, "/work-orders/attachments", "work_order:process", s.handleUploadWorkOrderAttachment)
	permissionPOST(group, "/work-orders/from-alarm", "work_order:create", s.handleCreateWorkOrderFromAlarm)
	permissionPOST(group, "/external-work-orders", "work_order:integration", s.handleCreateExternalWorkOrder)
	permissionGET(group, "/work-orders/public/:public_id", "work_order:list", s.handleGetWorkOrderByPublicID)
	permissionGET(group, "/work-orders/:id", "work_order:list", s.handleGetWorkOrder)
	permissionPOST(group, "/work-orders/:id/claim", "work_order:process", s.handleClaimWorkOrder)
	permissionPOST(group, "/work-orders/:id/transfer", "work_order:process", s.handleTransferWorkOrder)
	permissionPOST(group, "/work-orders/:id/transitions", "work_order:process", s.handleTransitionWorkOrder)
	permissionPOST(group, "/work-orders/:id/approve", "work_order:approve", s.handleApproveWorkOrder)
	permissionPOST(group, "/work-orders/:id/reject", "work_order:approve", s.handleRejectWorkOrder)
	permissionPOST(group, "/work-orders/:id/comments", "work_order:process", s.handleCommentWorkOrder)
	permissionPOST(group, "/work-orders/:id/cc", "work_order:process", s.handleCCWorkOrder)
	permissionGET(group, "/work-orders/:id/events", "work_order:list", s.handleListWorkOrderEvents)
	permissionGET(group, "/work-orders/:id/approval-tasks", "work_order:list", s.handleListWorkOrderApprovalTasks)
	permissionGET(group, "/work-orders/:id/tasks", "work_order:list", s.handleListWorkOrderTasks)
	permissionPOST(group, "/work-orders/:id/tasks/:task_public_id/complete", "work_order:process", s.handleCompleteWorkOrderTask)
	permissionPOST(group, "/work-orders/:id/tasks/:task_public_id/reject", "work_order:approve", s.handleRejectWorkOrderTask)
	permissionGET(group, "/work-order-notifications", "work_order:list", s.handleListMyWorkOrderNotifications)
	permissionPOST(group, "/work-order-notifications/:public_id/read", "work_order:list", s.handleMarkWorkOrderNotificationRead)
	permissionGET(group, "/work-order-notifications/stream", "work_order:list", s.handleWorkOrderNotificationStream)
	permissionPOST(group, "/work-orders/:id/links", "work_order:process", s.handleLinkWorkOrder)
	permissionPOST(group, "/work-order-outbox/claim", "work_order:integration", s.handleClaimWorkOrderOutbox)
	permissionPOST(group, "/work-order-outbox/:id/ack", "work_order:integration", s.handleAcknowledgeWorkOrderOutbox)
	permissionPOST(group, "/work-order-outbox/:id/retry", "work_order:integration", s.handleRetryWorkOrderOutbox)
	permissionGET(group, "/work-order-integrations/connectors", "work_order:integration", s.handleListWorkOrderIntegrationConnectors)
	permissionPOST(group, "/work-order-integrations/connectors", "work_order:integration", s.handleSaveWorkOrderIntegrationConnector)
	permissionGET(group, "/work-order-integrations/bindings", "work_order:integration", s.handleListWorkOrderIntegrationBindings)
	permissionPOST(group, "/work-order-integrations/bindings", "work_order:integration", s.handleSaveWorkOrderIntegrationBinding)
	permissionGET(group, "/work-order-integrations/bindings/:id/mappings", "work_order:integration", s.handleListWorkOrderIntegrationMappings)
	permissionPOST(group, "/work-order-integrations/bindings/:id/mappings", "work_order:integration", s.handleSaveWorkOrderIntegrationMapping)
	permissionPOST(group, "/work-order-integrations/inbox", "work_order:integration", s.handleWorkOrderIntegrationInbox)
}

func (s *Server) handleUploadWorkOrderImage(r *ghttp.Request) {
	file := r.GetUploadFile("file")
	if file == nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "No image uploaded"})
		return
	}
	extension := strings.ToLower(filepath.Ext(file.Filename))
	allowedExtensions := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true}
	if file.Size <= 0 || file.Size > 10*1024*1024 || !allowedExtensions[extension] || !strings.HasPrefix(strings.ToLower(file.Header.Get("Content-Type")), "image/") {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Image must be JPG, PNG, GIF, WebP, or BMP and no larger than 10 MB"})
		return
	}
	const imageDirectory = "./data/images"
	if err := os.MkdirAll(imageDirectory, 0755); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to prepare image storage"})
		return
	}
	filename, err := file.Save(imageDirectory, true)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to save image"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"url": "/data/images/" + filepath.Base(filename)}})
}

func (s *Server) handleUploadWorkOrderAttachment(r *ghttp.Request) {
	file := r.GetUploadFile("file")
	if file == nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "No file uploaded"})
		return
	}
	extension := strings.ToLower(filepath.Ext(file.Filename))
	allowedExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".txt": true, ".csv": true, ".zip": true, ".rar": true, ".7z": true,
	}
	if file.Size <= 0 || file.Size > 25*1024*1024 || !allowedExtensions[extension] {
		r.Response.WriteJson(g.Map{"code": 400, "message": "File format not supported or exceeds 25 MB size limit"})
		return
	}
	const attachmentDirectory = "./data/attachments"
	if err := os.MkdirAll(attachmentDirectory, 0755); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to prepare attachment storage"})
		return
	}
	filename, err := file.Save(attachmentDirectory, true)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to save attachment"})
		return
	}
	savedBaseName := filepath.Base(filename)
	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"name": file.Filename,
			"url":  "/data/attachments/" + savedBaseName,
			"size": file.Size,
			"type": extension,
		},
	})
}

func (s *Server) handleListWorkOrderParticipants(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	page := r.Get("page", 1).Int()
	pageSize := r.Get("page_size", r.Get("pageSize", 50).Int()).Int()
	participants, total, err := s.workOrderService().ListWorkOrderParticipants(scope, r.Get("query").String(), page, pageSize)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"items": participants, "page": page, "page_size": pageSize, "total": total}})
}

func (s *Server) handleListWorkOrderTemplates(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	templates, err := s.workOrderService().ListTemplates(scope)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": templates, "total": len(templates)})
}

func (s *Server) handleCreateWorkOrderTemplate(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload WorkOrderTemplateInput
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	template, err := s.workOrderService().CreateTemplate(scope, payload)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": template})
}

func (s *Server) handleCopyWorkOrderTemplate(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	template, err := s.workOrderService().CopyWorkOrderTemplate(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": template})
}

func (s *Server) handleGetWorkOrderTemplate(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	detail, err := s.workOrderService().GetTemplateDetail(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": detail})
}

func (s *Server) handlePublishWorkOrderFormVersion(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload workOrderFormSchemaRequest
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	definition, err := payload.legacyDefinition()
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	version, err := s.workOrderService().PublishFormVersion(scope, r.Get("id").Uint(), definition)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": version})
}

func (s *Server) handleSaveWorkOrderFormDraft(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload workOrderFormSchemaRequest
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	definition, err := payload.legacyDefinition()
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	template, err := s.workOrderService().SaveFormDraft(scope, r.Get("id").Uint(), definition)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": template})
}

func (s *Server) handleValidateWorkOrderFormSchema(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload workOrderFormSchemaRequest
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	definition, err := payload.portDefinition()
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	compiled, err := workorder.CompileFormDefinition(definition)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"tenant_id": scope.TenantID, "project_id": scope.ProjectID, "checksum": compiled.Checksum}})
}

type workOrderFormSchemaRequest struct {
	Definition WorkOrderFormDefinition `json:"definition"`
	Schema     map[string]any          `json:"schema"`
	UISchema   map[string]any          `json:"ui_schema"`
	Defaults   map[string]any          `json:"defaults"`
	Required   []string                `json:"required"`
	FieldOrder []string                `json:"field_order"`
}

func (p workOrderFormSchemaRequest) portDefinition() (workorder.FormDefinition, error) {
	if len(p.Schema) == 0 {
		legacy, err := p.legacyDefinition()
		if err != nil {
			return workorder.FormDefinition{}, err
		}
		properties := map[string]any{}
		required := []string{}
		order := []string{}
		for _, field := range legacy.Fields {
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
			if len(field.Options) > 0 && field.Type == WorkOrderFormFieldMultiSelect {
				values := make([]any, len(field.Options))
				for i, option := range field.Options {
					values[i] = option
				}
				property["items"] = map[string]any{"type": "string", "enum": values}
			} else if len(field.Options) > 0 {
				values := make([]any, len(field.Options))
				for i, option := range field.Options {
					values[i] = option
				}
				property["enum"] = values
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
			}
			properties[field.Key] = property
			order = append(order, field.Key)
			if field.Required {
				required = append(required, field.Key)
			}
		}
		reqValues := make([]any, len(required))
		for i, value := range required {
			reqValues[i] = value
		}
		return workorder.FormDefinition{Schema: map[string]any{"type": "object", "properties": properties, "required": reqValues}, UISchema: map[string]any{"order": stringValuesAsAny(order)}, Defaults: p.Defaults, Required: required, FieldOrder: order}, nil
	}
	return workorder.FormDefinition{Schema: p.Schema, UISchema: p.UISchema, Defaults: p.Defaults, Required: p.Required, FieldOrder: p.FieldOrder}, nil
}

func (p workOrderFormSchemaRequest) legacyDefinition() (WorkOrderFormDefinition, error) {
	if len(p.Definition.Fields) > 0 || len(p.Schema) == 0 {
		return p.Definition, nil
	}
	port, err := p.portDefinition()
	if err != nil {
		return WorkOrderFormDefinition{}, err
	}
	properties, _ := port.Schema["properties"].(map[string]any)
	fields := make([]WorkOrderFormField, 0, len(properties))
	order := port.FieldOrder
	if len(order) == 0 {
		for key := range properties {
			order = append(order, key)
		}
		sort.Strings(order)
	}
	required := map[string]bool{}
	for _, key := range port.Required {
		required[key] = true
	}
	for _, key := range order {
		property, _ := properties[key].(map[string]any)
		field := WorkOrderFormField{Key: key, Label: key, Type: WorkOrderFormFieldText, Required: required[key]}
		if title, ok := property["title"].(string); ok && title != "" {
			field.Label = title
		}
		if resource, ok := property["x-noyo-resource"].(string); ok {
			field.Type = resource
		}
		if property["x-noyo-attachment"] == true {
			field.Type = WorkOrderFormFieldAttachment
		}
		if property["x-noyo-images"] == true {
			field.Type = WorkOrderFormFieldImages
		}
		if typ, ok := property["type"].(string); ok {
			switch typ {
			case "number":
				field.Type = WorkOrderFormFieldNumber
			case "integer":
				field.Type = WorkOrderFormFieldInteger
			case "boolean":
				field.Type = WorkOrderFormFieldBoolean
			case "enum":
				field.Type = WorkOrderFormFieldSelect
			case "date-time":
				field.Type = WorkOrderFormFieldDateTime
			case "array":
				items, _ := property["items"].(map[string]any)
				if items["type"] == "string" {
					if options, ok := items["enum"].([]any); ok {
						field.Type = WorkOrderFormFieldMultiSelect
						for _, option := range options {
							if text, ok := option.(string); ok {
								field.Options = append(field.Options, text)
							}
						}
					}
				}
			}
		}
		if options, ok := property["enum"].([]any); ok {
			field.Type = WorkOrderFormFieldSelect
			for _, option := range options {
				if text, ok := option.(string); ok {
					field.Options = append(field.Options, text)
				}
			}
		}
		if value, ok := property["default"]; ok {
			field.DefaultValue = value
		}
		fields = append(fields, field)
	}
	return WorkOrderFormDefinition{Fields: fields}, nil
}

func stringValuesAsAny(values []string) []any {
	result := make([]any, len(values))
	for i, value := range values {
		result[i] = value
	}
	return result
}

func (s *Server) handlePublishWorkOrderWorkflowVersion(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Definition WorkOrderWorkflowDefinition `json:"definition"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	version, err := s.workOrderService().PublishWorkflowVersion(scope, r.Get("id").Uint(), payload.Definition)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": version})
}

func (s *Server) handleSaveWorkOrderWorkflowDraft(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Definition WorkOrderWorkflowDefinition `json:"definition"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	template, err := s.workOrderService().SaveWorkflowDraft(scope, r.Get("id").Uint(), payload.Definition)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": template})
}

func (s *Server) handleListWorkOrders(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	orders, total, err := s.workOrderService().ListWorkOrders(scope, WorkOrderListOptions{
		Page:       r.Get("page", 1).Int(),
		PageSize:   r.Get("pageSize", 20).Int(),
		Search:     r.Get("search").String(),
		Status:     r.Get("status").String(),
		SourceType: r.Get("source_type").String(),
		TemplateID: r.Get("template_id").Uint(),
		AssigneeID: r.Get("assignee_user_id").Uint(),
		Relation:   r.Get("relation").String(),
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	data, err := s.workOrderListResponses(orders)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": data, "total": total})
}

func (s *Server) handleCreateWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		TemplateID uint            `json:"template_id"`
		Title      string          `json:"title"`
		Summary    string          `json:"summary"`
		Priority   string          `json:"priority"`
		Source     WorkOrderSource `json:"source"`
		FormData   map[string]any  `json:"form_data"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	if sourceType := strings.TrimSpace(payload.Source.Type); sourceType != "" && sourceType != WorkOrderSourceManual {
		writeWorkOrderError(r, fmt.Errorf("manual work order endpoint only accepts manual source type"))
		return
	}
	payload.Source = normalizeHTTPWorkOrderSource(r, scope, payload.Source)
	commandResult, err := s.workOrderCommandPort().Create(r.Context(), workorder.CreateCommand{
		Scope:        workOrderCommandScope(scope),
		Meta:         workorder.CommandMeta{IdempotencyKey: payload.Source.IdempotencyKey, CorrelationID: r.Header.Get("X-Correlation-ID")},
		TemplateCode: strconv.FormatUint(uint64(payload.TemplateID), 10), Title: payload.Title, Summary: payload.Summary,
		Priority: payload.Priority, SourceType: payload.Source.Type, SourceRef: payload.Source.ID,
		SourceSnapshot: payload.Source.Snapshot, FormData: payload.FormData,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	order, err := s.workOrderService().GetWorkOrderByPublicID(scope, commandResult.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	response, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response, "created": commandResult.Created})
}

func (s *Server) handleCreateWorkOrderFromAlarm(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		TemplateID      uint           `json:"template_id"`
		Title           string         `json:"title"`
		Summary         string         `json:"summary"`
		Priority        string         `json:"priority"`
		AlarmID         string         `json:"alarm_id"`
		AlarmInstanceID string         `json:"alarm_instance_id"`
		Generation      *int           `json:"generation"`
		IdempotencyKey  string         `json:"idempotency_key"`
		Snapshot        map[string]any `json:"snapshot"`
		FormData        map[string]any `json:"form_data"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	payload.AlarmID = strings.TrimSpace(payload.AlarmID)
	payload.AlarmInstanceID = strings.TrimSpace(payload.AlarmInstanceID)
	payload.IdempotencyKey = strings.TrimSpace(payload.IdempotencyKey)
	if payload.AlarmInstanceID == "" {
		writeWorkOrderError(r, fmt.Errorf("server-issued alarm_instance_id is required"))
		return
	}
	if s.AlarmInstances == nil {
		writeWorkOrderError(r, fmt.Errorf("alarm instance service is unavailable"))
		return
	}
	// The client may select an alarm occurrence, but it can neither invent its
	// evidence nor change its generation. The persisted instance is the sole
	// authority for alarm-to-work-order conversion.
	instance, resolveErr := s.AlarmInstances.Get(workOrderCommandScope(scope), payload.AlarmInstanceID)
	if resolveErr != nil {
		writeWorkOrderError(r, resolveErr)
		return
	}
	if payload.Generation != nil && *payload.Generation != instance.Generation {
		writeWorkOrderError(r, fmt.Errorf("alarm generation does not match the selected alarm instance"))
		return
	}
	payload.AlarmID = instance.ID
	payload.Generation = &instance.Generation
	payload.Snapshot = instance.EvidenceSnapshot
	if payload.IdempotencyKey == "" {
		payload.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	if payload.IdempotencyKey == "" {
		payload.IdempotencyKey = "alarm:" + payload.AlarmID
	}
	commandResult, err := s.workOrderCommandPort().Create(r.Context(), workorder.CreateCommand{
		Scope:        workOrderCommandScope(scope),
		Meta:         workorder.CommandMeta{IdempotencyKey: payload.IdempotencyKey, CorrelationID: r.Header.Get("X-Correlation-ID")},
		TemplateCode: strconv.FormatUint(uint64(payload.TemplateID), 10), Title: payload.Title, Summary: payload.Summary,
		Priority: payload.Priority, SourceType: WorkOrderSourceAlarm, SourceRef: payload.AlarmID,
		SourceSnapshot: payload.Snapshot, FormData: payload.FormData,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	order, err := s.workOrderService().GetWorkOrderByPublicID(scope, commandResult.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	if _, err := s.AlarmInstances.BindWorkOrder(r.Context(), workOrderCommandScope(scope), instance.ID, workorder.AlarmWorkOrderBinding{
		WorkOrderPublicID: commandResult.PublicID,
		SourceSnapshot:    payload.Snapshot,
		CorrelationID:     r.Header.Get("X-Correlation-ID"),
		CausationID:       r.Header.Get("X-Causation-ID"),
	}); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	response, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response, "created": commandResult.Created})
}

func (s *Server) handleCreateExternalWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Provider    string         `json:"provider"`
		ExternalRef string         `json:"external_ref"`
		Revision    string         `json:"revision"`
		TemplateID  uint           `json:"template_id"`
		Title       string         `json:"title"`
		Summary     string         `json:"summary"`
		Priority    string         `json:"priority"`
		Snapshot    map[string]any `json:"snapshot"`
		FormData    map[string]any `json:"form_data"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	payload.Provider = strings.TrimSpace(payload.Provider)
	payload.ExternalRef = strings.TrimSpace(payload.ExternalRef)
	payload.Revision = strings.TrimSpace(payload.Revision)
	if !isWorkOrderIdentifier(payload.Provider) || payload.ExternalRef == "" || len(payload.ExternalRef) > 191 {
		writeWorkOrderError(r, fmt.Errorf("valid provider and external_ref are required"))
		return
	}
	if payload.Revision == "" {
		payload.Revision = "current"
	}
	sourceID := payload.Provider + ":" + payload.ExternalRef
	commandResult, err := s.workOrderCommandPort().Create(r.Context(), workorder.CreateCommand{
		Scope:        workOrderCommandScope(scope),
		Meta:         workorder.CommandMeta{IdempotencyKey: sourceID + ":" + payload.Revision, CorrelationID: r.Header.Get("X-Correlation-ID")},
		TemplateCode: strconv.FormatUint(uint64(payload.TemplateID), 10), Title: payload.Title, Summary: payload.Summary,
		Priority: payload.Priority, SourceType: WorkOrderSourceExternal, SourceRef: sourceID,
		SourceSnapshot: payload.Snapshot, FormData: payload.FormData,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	order, err := s.workOrderService().GetWorkOrderByPublicID(scope, commandResult.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	if _, err := s.workOrderService().LinkWorkOrder(scope, order.ID, WorkOrderLinkInput{
		RelationType: "external_binding",
		ExternalRef:  sourceID,
		Metadata:     map[string]any{"provider": payload.Provider, "external_ref": payload.ExternalRef},
	}); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	response, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response, "created": commandResult.Created})
}

func (s *Server) handleGetWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	order, err := s.workOrderService().GetWorkOrder(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	response, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response})
}

func (s *Server) handleGetWorkOrderByPublicID(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	order, err := s.workOrderService().GetWorkOrderByPublicID(scope, r.Get("public_id").String())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	response, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response})
}

func (s *Server) handleClaimWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		ExpectedVersion *int `json:"expected_version"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().ClaimWorkOrderWithVersion(scope, r.Get("id").Uint(), payload.ExpectedVersion)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	writeWorkOrderResponse(r, order)
}

func (s *Server) handleTransferWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		TargetUserID    uint   `json:"target_user_id"`
		TaskPublicID    string `json:"task_public_id"`
		Comment         string `json:"comment"`
		ExpectedVersion *int   `json:"expected_version"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().TransferWorkOrderWithVersion(scope, r.Get("id").Uint(), WorkOrderTransferInput{
		TargetUserID: payload.TargetUserID,
		TaskPublicID: payload.TaskPublicID,
		Comment:      payload.Comment,
	}, payload.ExpectedVersion)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	writeWorkOrderResponse(r, order)
}

func (s *Server) handleTransitionWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Key             string                   `json:"key"`
		Comment         string                   `json:"comment"`
		ExpectedVersion *int                     `json:"expected_version"`
		Resolution      workorder.Resolution     `json:"resolution"`
		Attachments     []workorder.AttachmentItem `json:"attachments"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().GetWorkOrder(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		key = "http:transition:" + uuid.NewString()
	}
	result, err := s.workOrderCommandPort().Execute(r.Context(), workorder.ExecuteCommand{
		Scope: workOrderCommandScope(scope), Meta: workorder.CommandMeta{IdempotencyKey: key, ExpectedVersion: payload.ExpectedVersion},
		WorkOrderPublicID: order.PublicID, Action: payload.Key, Comment: payload.Comment, Resolution: payload.Resolution, Attachments: payload.Attachments,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	updated, err := s.workOrderService().GetWorkOrderByPublicID(scope, result.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	s.recordArchivedWorkOrderMemoryCandidate(updated)
	writeWorkOrderResponse(r, updated)
}

func (s *Server) handleApproveWorkOrder(r *ghttp.Request) {
	s.handleWorkOrderApproval(r, true)
}

func (s *Server) handleRejectWorkOrder(r *ghttp.Request) {
	s.handleWorkOrderApproval(r, false)
}

func (s *Server) handleWorkOrderApproval(r *ghttp.Request, approve bool) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Comment         string `json:"comment"`
		ExpectedVersion *int   `json:"expected_version"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().GetWorkOrder(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		key = "http:approval:" + uuid.NewString()
	}
	command := workorder.ApprovalCommand{Scope: workOrderCommandScope(scope), Meta: workorder.CommandMeta{IdempotencyKey: key, ExpectedVersion: payload.ExpectedVersion}, WorkOrderPublicID: order.PublicID, Comment: payload.Comment}
	var result workorder.CommandResult
	if approve {
		result, err = s.workOrderCommandPort().Approve(r.Context(), command)
	} else {
		result, err = s.workOrderCommandPort().Reject(r.Context(), command)
	}
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	updated, err := s.workOrderService().GetWorkOrderByPublicID(scope, result.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	s.recordArchivedWorkOrderMemoryCandidate(updated)
	writeWorkOrderResponse(r, updated)
}

func (s *Server) handleCommentWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Comment string `json:"comment"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	if err := s.workOrderService().AddComment(scope, r.Get("id").Uint(), payload.Comment); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleCCWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		UserIDs []uint `json:"user_ids"`
		Comment string `json:"comment"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	if len(payload.UserIDs) == 0 {
		writeWorkOrderError(r, fmt.Errorf("user_ids is required"))
		return
	}
	if err := s.workOrderService().CCWorkOrder(scope, r.Get("id").Uint(), payload.UserIDs, payload.Comment); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleListWorkOrderEvents(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	events, err := s.workOrderService().ListWorkOrderEvents(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	data, err := workOrderEventResponses(s.workOrderService().db, scope, events)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": data})
}

func workOrderEventResponses(db *gorm.DB, scope WorkOrderScope, events []store.WorkOrderEvent) ([]g.Map, error) {
	actorIDs := make([]uint, 0, len(events))
	seen := make(map[uint]bool, len(events))
	for _, event := range events {
		if event.ActorUserID != 0 && !seen[event.ActorUserID] {
			seen[event.ActorUserID] = true
			actorIDs = append(actorIDs, event.ActorUserID)
		}
	}
	usersByID := make(map[uint]string, len(actorIDs))
	if len(actorIDs) > 0 {
		var users []store.User
		if err := db.Unscoped().Where("tenant_id = ? AND id IN ?", scope.TenantID, actorIDs).Find(&users).Error; err != nil {
			return nil, fmt.Errorf("load work order event actors: %w", err)
		}
		for _, user := range users {
			name := strings.TrimSpace(user.DisplayName)
			if name == "" {
				name = strings.TrimSpace(user.Username)
			}
			if name != "" {
				usersByID[user.ID] = name
			}
		}
	}
	data := make([]g.Map, 0, len(events))
	for _, event := range events {
		payload := make(map[string]any)
		if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil {
			return nil, fmt.Errorf("decode work order event payload: %w", err)
		}
		actorName := "系统 / System"
		if event.ActorUserID != 0 {
			actorName = usersByID[event.ActorUserID]
			if actorName == "" {
				actorName = "已删除或不可用用户 / Deleted or unavailable user"
			}
		}
		data = append(data, g.Map{"event": event, "payload": payload, "actor_name": actorName})
	}
	return data, nil
}

func (s *Server) handleListWorkOrderApprovalTasks(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	tasks, err := s.workOrderService().ListApprovalTasks(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": tasks})
}

func (s *Server) handleListWorkOrderTasks(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	tasks, err := s.workOrderService().ListWorkOrderTasks(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": tasks})
}

func (s *Server) handleCompleteWorkOrderTask(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Comment         string                   `json:"comment"`
		ExpectedVersion *int                     `json:"expected_version"`
		Attachments     []workorder.AttachmentItem `json:"attachments"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().GetWorkOrder(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		key = "http:work-order-task-complete:" + uuid.NewString()
	}
	result, err := s.workOrderCommandPort().Execute(r.Context(), workorder.ExecuteCommand{
		Scope:             workOrderCommandScope(scope),
		Meta:              workorder.CommandMeta{IdempotencyKey: key, ExpectedVersion: payload.ExpectedVersion},
		WorkOrderPublicID: order.PublicID,
		TaskPublicID:      r.Get("task_public_id").String(),
		Action:            "complete",
		Comment:           payload.Comment,
		Attachments:       payload.Attachments,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	updated, err := s.workOrderService().GetWorkOrderByPublicID(scope, result.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	s.recordArchivedWorkOrderMemoryCandidate(updated)
	writeWorkOrderResponse(r, updated)
}

func (s *Server) handleRejectWorkOrderTask(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Comment         string `json:"comment"`
		ExpectedVersion *int   `json:"expected_version"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().GetWorkOrder(scope, r.Get("id").Uint())
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if key == "" {
		key = "http:work-order-task-reject:" + uuid.NewString()
	}
	result, err := s.workOrderCommandPort().Reject(r.Context(), workorder.ApprovalCommand{
		Scope:             workOrderCommandScope(scope),
		Meta:              workorder.CommandMeta{IdempotencyKey: key, ExpectedVersion: payload.ExpectedVersion},
		WorkOrderPublicID: order.PublicID,
		TaskPublicID:      r.Get("task_public_id").String(),
		Comment:           payload.Comment,
	})
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	updated, err := s.workOrderService().GetWorkOrderByPublicID(scope, result.PublicID)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	writeWorkOrderResponse(r, updated)
}

func (s *Server) handleListMyWorkOrderNotifications(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	page := r.Get("page", 1).Int()
	pageSize := r.Get("page_size", r.Get("pageSize", 20).Int()).Int()
	notifications, total, err := s.workOrderService().ListMyWorkOrderNotifications(scope, r.Get("unread").Bool(), page, pageSize)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"items": notifications, "page": page, "page_size": pageSize, "total": total}})
}

func (s *Server) handleMarkWorkOrderNotificationRead(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	if err := s.workOrderService().MarkWorkOrderNotificationRead(scope, r.Get("public_id").String()); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleWorkOrderNotificationStream(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	service := s.workOrderService()
	latest, _, err := service.ListMyWorkOrderNotifications(scope, false, 1, 1)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	snapshot, unreadTotal, err := service.ListMyWorkOrderNotifications(scope, true, 1, 100)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	cursor := uint(0)
	if len(latest) > 0 {
		cursor = latest[0].ID
	}
	for _, notification := range snapshot {
		if notification.ID > cursor {
			cursor = notification.ID
		}
	}
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("X-Accel-Buffering", "no")
	if err := writeWorkOrderSSE(r, "snapshot", 0, g.Map{"items": snapshot, "unread_total": unreadTotal}); err != nil {
		return
	}
	pollTicker := time.NewTicker(time.Second)
	heartbeatTicker := time.NewTicker(15 * time.Second)
	defer pollTicker.Stop()
	defer heartbeatTicker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-pollTicker.C:
			notifications, err := service.ListMyWorkOrderNotificationsAfterID(scope, cursor, 100)
			if err != nil {
				return
			}
			for _, notification := range notifications {
				cursor = notification.ID
				if err := writeWorkOrderSSE(r, "notification", notification.ID, notification); err != nil {
					return
				}
			}
		case <-heartbeatTicker.C:
			if err := writeWorkOrderSSE(r, "heartbeat", 0, g.Map{}); err != nil {
				return
			}
		}
	}
}

func writeWorkOrderSSE(r *ghttp.Request, eventType string, eventID uint, data any) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	message := ""
	if eventID > 0 {
		message += fmt.Sprintf("id: %d\n", eventID)
	}
	message += fmt.Sprintf("event: %s\ndata: %s\n\n", eventType, encoded)
	if _, err := r.Response.Writer.Write([]byte(message)); err != nil {
		return err
	}
	r.Response.Flush()
	return nil
}

func (s *Server) handleLinkWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload WorkOrderLinkInput
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	link, err := s.workOrderService().LinkWorkOrder(scope, r.Get("id").Uint(), payload)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": link})
}

func (s *Server) handleClaimWorkOrderOutbox(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		Limit        int    `json:"limit"`
		LockToken    string `json:"lock_token"`
		LeaseSeconds int    `json:"lease_seconds"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	events, err := s.workOrderService().ClaimWorkOrderOutboxEvents(scope, payload.Limit, payload.LockToken, time.Duration(payload.LeaseSeconds)*time.Second)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": events})
}

func (s *Server) handleAcknowledgeWorkOrderOutbox(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		LockToken string `json:"lock_token"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	if err := s.workOrderService().AcknowledgeWorkOrderOutboxEvent(scope, r.Get("id").Uint(), payload.LockToken); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleRetryWorkOrderOutbox(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	var payload struct {
		LockToken    string `json:"lock_token"`
		Failure      string `json:"failure"`
		RetrySeconds int    `json:"retry_seconds"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	if err := s.workOrderService().RetryWorkOrderOutboxEvent(scope, r.Get("id").Uint(), payload.LockToken, payload.Failure, time.Duration(payload.RetrySeconds)*time.Second); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) workOrderService() *WorkOrderService {
	if s.WorkOrderService == nil {
		s.WorkOrderService = NewWorkOrderService(store.DB)
	}
	return s.WorkOrderService
}

func (s *Server) workOrderCommandPort() workorder.CommandPort {
	if s.WorkOrderCommands == nil {
		s.WorkOrderCommands = NewLocalCommandAdapter(s.workOrderService())
	}
	return s.WorkOrderCommands
}

func workOrderCommandScope(scope WorkOrderScope) workorder.Scope {
	return workorder.Scope{
		TenantID:  scope.TenantID,
		ProjectID: scope.ProjectID,
		Principal: workorder.Principal{Type: "user", Ref: strconv.FormatUint(uint64(scope.ActorUserID), 10)},
	}
}

func workOrderScopeForRequest(r *ghttp.Request) (WorkOrderScope, bool) {
	tenantID, projectID, err := currentTenantProjectScope(r)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return WorkOrderScope{}, false
	}
	return WorkOrderScope{TenantID: tenantID, ProjectID: projectID, ActorUserID: requestUserID(r)}, true
}

func decodeWorkOrderRequest(r *ghttp.Request, target any) bool {
	body := bytes.TrimSpace(r.GetBody())
	if len(body) == 0 {
		return true
	}
	if err := json.Unmarshal(body, target); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "invalid JSON request: " + err.Error()})
		return false
	}
	return true
}

func normalizeHTTPWorkOrderSource(r *ghttp.Request, scope WorkOrderScope, source WorkOrderSource) WorkOrderSource {
	source.Type = strings.TrimSpace(source.Type)
	source.ID = strings.TrimSpace(source.ID)
	source.IdempotencyKey = strings.TrimSpace(source.IdempotencyKey)
	if source.Type == "" {
		source.Type = WorkOrderSourceManual
	}
	if source.Type == WorkOrderSourceManual && source.ID == "" {
		source.ID = fmt.Sprintf("manual:%d", scope.ActorUserID)
	}
	if source.IdempotencyKey == "" {
		source.IdempotencyKey = strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	}
	if source.IdempotencyKey == "" {
		source.IdempotencyKey = "request:" + uuid.NewString()
	}
	return source
}

func writeWorkOrderResponse(r *ghttp.Request, order *store.WorkOrder) {
	response, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response})
}

type workOrderResponsibilityUser struct {
	UserID      uint   `json:"user_id"`
	DisplayName string `json:"display_name"`
}

type workOrderResponsibility struct {
	CurrentHandlers []workOrderResponsibilityUser `json:"current_handlers"`
	LastHandler     *workOrderResponsibilityUser  `json:"last_handler,omitempty"`
	ClaimedHandler  *workOrderResponsibilityUser  `json:"claimed_handler,omitempty"`
}

type workOrderHandlerCandidate struct {
	userID      uint
	displayName string
	occurredAt  time.Time
}

func (s *Server) workOrderListResponses(orders []store.WorkOrder) ([]g.Map, error) {
	responsibilities, err := loadWorkOrderResponsibilities(s.workOrderService().db, orders)
	if err != nil {
		return nil, err
	}
	responses, err := workOrderResponses(orders)
	if err != nil {
		return nil, err
	}
	for index, order := range orders {
		responses[index]["responsibility"] = responsibilities[order.ID]
	}
	return responses, nil
}

func loadWorkOrderResponsibilities(db *gorm.DB, orders []store.WorkOrder) (map[uint]workOrderResponsibility, error) {
	responsibilities := make(map[uint]workOrderResponsibility, len(orders))
	if len(orders) == 0 {
		return responsibilities, nil
	}
	orderIDs := make([]uint, 0, len(orders))
	userIDs := make(map[uint]struct{})
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
		responsibilities[order.ID] = workOrderResponsibility{CurrentHandlers: []workOrderResponsibilityUser{}}
		if order.AssigneeUserID > 0 {
			userIDs[order.AssigneeUserID] = struct{}{}
		}
	}

	var pendingTasks []store.WorkOrderTask
	if err := db.Where("work_order_id IN ? AND status = ?", orderIDs, WorkOrderTaskPending).Order("work_order_id ASC, id ASC").Find(&pendingTasks).Error; err != nil {
		return nil, fmt.Errorf("load pending work order tasks: %w", err)
	}
	for _, task := range pendingTasks {
		responsibility := responsibilities[task.WorkOrderID]
		if !containsWorkOrderResponsibilityUser(responsibility.CurrentHandlers, task.AssigneeUserID) {
			responsibility.CurrentHandlers = append(responsibility.CurrentHandlers, workOrderResponsibilityUser{UserID: task.AssigneeUserID, DisplayName: strings.TrimSpace(task.AssigneeDisplayName)})
			userIDs[task.AssigneeUserID] = struct{}{}
		}
		responsibilities[task.WorkOrderID] = responsibility
	}

	var pendingApprovals []store.WorkOrderApprovalTask
	if err := db.Where("work_order_id IN ? AND status = ?", orderIDs, WorkOrderApprovalPending).Order("work_order_id ASC, id ASC").Find(&pendingApprovals).Error; err != nil {
		return nil, fmt.Errorf("load pending work order approvals: %w", err)
	}
	for _, approval := range pendingApprovals {
		responsibility := responsibilities[approval.WorkOrderID]
		if !containsWorkOrderResponsibilityUser(responsibility.CurrentHandlers, approval.ApproverUserID) {
			responsibility.CurrentHandlers = append(responsibility.CurrentHandlers, workOrderResponsibilityUser{UserID: approval.ApproverUserID})
			userIDs[approval.ApproverUserID] = struct{}{}
		}
		responsibilities[approval.WorkOrderID] = responsibility
	}

	lastCandidates := make(map[uint]workOrderHandlerCandidate)
	var completedTasks []store.WorkOrderTask
	if err := db.Where("work_order_id IN ? AND status = ?", orderIDs, WorkOrderTaskCompleted).Order("completed_at DESC, id DESC").Find(&completedTasks).Error; err != nil {
		return nil, fmt.Errorf("load completed work order tasks: %w", err)
	}
	for _, task := range completedTasks {
		occurredAt := task.UpdatedAt
		if task.CompletedAt != nil {
			occurredAt = *task.CompletedAt
		}
		updateWorkOrderHandlerCandidate(lastCandidates, task.WorkOrderID, workOrderHandlerCandidate{userID: task.AssigneeUserID, displayName: strings.TrimSpace(task.AssigneeDisplayName), occurredAt: occurredAt})
		userIDs[task.AssigneeUserID] = struct{}{}
	}

	var approvedTasks []store.WorkOrderApprovalTask
	if err := db.Where("work_order_id IN ? AND status = ?", orderIDs, WorkOrderApprovalApproved).Order("decided_at DESC, id DESC").Find(&approvedTasks).Error; err != nil {
		return nil, fmt.Errorf("load completed work order approvals: %w", err)
	}
	for _, approval := range approvedTasks {
		occurredAt := approval.UpdatedAt
		if approval.DecidedAt != nil {
			occurredAt = *approval.DecidedAt
		}
		updateWorkOrderHandlerCandidate(lastCandidates, approval.WorkOrderID, workOrderHandlerCandidate{userID: approval.ApproverUserID, occurredAt: occurredAt})
		userIDs[approval.ApproverUserID] = struct{}{}
	}

	var completionEvents []store.WorkOrderEvent
	if err := db.Where("work_order_id IN ? AND type IN ?", orderIDs, []string{"transitioned", "approval.completed", "workflow.completed"}).Order("created_at DESC, id DESC").Find(&completionEvents).Error; err != nil {
		return nil, fmt.Errorf("load terminal work order events: %w", err)
	}
	for _, event := range completionEvents {
		updateWorkOrderHandlerCandidate(lastCandidates, event.WorkOrderID, workOrderHandlerCandidate{userID: event.ActorUserID, occurredAt: event.CreatedAt})
		userIDs[event.ActorUserID] = struct{}{}
	}

	names, err := loadWorkOrderResponsibilityNames(db, userIDs)
	if err != nil {
		return nil, err
	}
	for _, order := range orders {
		responsibility := responsibilities[order.ID]
		for index := range responsibility.CurrentHandlers {
			if strings.TrimSpace(responsibility.CurrentHandlers[index].DisplayName) == "" {
				responsibility.CurrentHandlers[index].DisplayName = names[responsibility.CurrentHandlers[index].UserID]
			}
		}
		if order.AssigneeUserID > 0 {
			responsibility.ClaimedHandler = &workOrderResponsibilityUser{UserID: order.AssigneeUserID, DisplayName: names[order.AssigneeUserID]}
		}
		if candidate, ok := lastCandidates[order.ID]; ok && candidate.userID > 0 {
			displayName := candidate.displayName
			if displayName == "" {
				displayName = names[candidate.userID]
			}
			responsibility.LastHandler = &workOrderResponsibilityUser{UserID: candidate.userID, DisplayName: displayName}
		}
		responsibilities[order.ID] = responsibility
	}
	return responsibilities, nil
}

func containsWorkOrderResponsibilityUser(users []workOrderResponsibilityUser, userID uint) bool {
	for _, user := range users {
		if user.UserID == userID {
			return true
		}
	}
	return false
}

func updateWorkOrderHandlerCandidate(candidates map[uint]workOrderHandlerCandidate, workOrderID uint, candidate workOrderHandlerCandidate) {
	if candidate.userID == 0 {
		return
	}
	if current, exists := candidates[workOrderID]; !exists || candidate.occurredAt.After(current.occurredAt) {
		candidates[workOrderID] = candidate
	}
}

func loadWorkOrderResponsibilityNames(db *gorm.DB, userIDs map[uint]struct{}) (map[uint]string, error) {
	names := make(map[uint]string, len(userIDs))
	ids := make([]uint, 0, len(userIDs))
	for userID := range userIDs {
		if userID > 0 {
			ids = append(ids, userID)
		}
	}
	if len(ids) == 0 {
		return names, nil
	}
	var users []store.User
	if err := db.Where("id IN ?", ids).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("load work order handler names: %w", err)
	}
	for _, user := range users {
		names[user.ID] = strings.TrimSpace(user.DisplayName)
	}
	return names, nil
}

func workOrderResponses(orders []store.WorkOrder) ([]g.Map, error) {
	responses := make([]g.Map, 0, len(orders))
	for _, order := range orders {
		response, err := workOrderResponse(order)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func workOrderResponse(order store.WorkOrder) (g.Map, error) {
	formData := make(map[string]any)
	if err := json.Unmarshal([]byte(order.FormData), &formData); err != nil {
		return nil, fmt.Errorf("decode work order form data: %w", err)
	}
	sourceSnapshot := make(map[string]any)
	if strings.TrimSpace(order.SourceSnapshot) != "" {
		if err := json.Unmarshal([]byte(order.SourceSnapshot), &sourceSnapshot); err != nil {
			return nil, fmt.Errorf("decode work order source snapshot: %w", err)
		}
	}
	formDefinition, err := decodeWorkOrderFormDefinition(order.FormDefinitionSnapshot)
	if err != nil {
		return nil, err
	}
	workflow, err := decodeWorkOrderWorkflowDefinition(order.WorkflowSnapshot)
	if err != nil {
		return nil, err
	}
	resolution := WorkOrderResolution{}
	if strings.TrimSpace(order.ResolutionJSON) != "" {
		if err := json.Unmarshal([]byte(order.ResolutionJSON), &resolution); err != nil {
			return nil, fmt.Errorf("decode work order resolution: %w", err)
		}
	}
	return g.Map{
		"work_order":        order,
		"form_data":         formData,
		"source_snapshot":   sourceSnapshot,
		"form_definition":   formDefinition,
		"workflow":          workflow,
		"resolution":        resolution,
		"available_actions": availableWorkOrderActions(order, workflow),
	}, nil
}

func availableWorkOrderActions(order store.WorkOrder, workflow WorkOrderWorkflowDefinition) g.Map {
	status, ok := workflow.statusByKey(order.Status)
	actions := make([]g.Map, 0)
	if ok && !isGraphWorkOrderWorkflow(workflow) {
		for _, transition := range workflow.Transitions {
			if transition.From == order.Status {
				actions = append(actions, g.Map{"key": transition.Key, "name": transition.Name, "requires_resolution": transition.RequireResolution, "requires_approval": len(transition.ApproverUserIDs) > 0})
			}
		}
	}
	claimable := ok && order.AssigneeUserID == 0 && (status.Category == WorkOrderStatusOpen || status.Category == WorkOrderStatusInProgress)
	editable := ok && status.Category == WorkOrderStatusOpen
	return g.Map{"claim": claimable, "edit": editable, "delete": editable, "transitions": actions}
}

func writeWorkOrderError(r *ghttp.Request, err error) {
	message := err.Error()
	code := 400
	errorCode := ""
	var commandError *workorder.Error
	if errors.As(err, &commandError) {
		errorCode = commandError.Code
	}
	if errors.Is(err, ErrWorkOrderVersionConflict) {
		errorCode = workorder.CodeVersionConflict
	}
	if strings.Contains(message, "not found") {
		code = 404
	}
	r.Response.WriteJson(g.Map{"code": code, "message": message, "error_code": errorCode})
}

func (s *Server) handleUpdateWorkOrderTemplate(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	id := r.Get("id").Uint()
	var payload struct {
		Name string `json:"name"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	template, err := s.workOrderService().UpdateWorkOrderTemplate(scope, id, payload.Name)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "success", "data": template})
}

func (s *Server) handleDeleteWorkOrderTemplate(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	id := r.Get("id").Uint()
	if err := s.workOrderService().DeleteWorkOrderTemplate(scope, id); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "success"})
}

func (s *Server) handleUpdateWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	id := r.Get("id").Uint()
	var payload struct {
		Title           string `json:"title"`
		Summary         string `json:"summary"`
		Priority        string `json:"priority"`
		ExpectedVersion *int   `json:"expected_version"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	order, err := s.workOrderService().UpdateWorkOrderWithVersion(scope, id, payload.Title, payload.Summary, payload.Priority, payload.ExpectedVersion)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	data, err := workOrderResponse(*order)
	if err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "success", "data": data})
}

func (s *Server) handleDeleteWorkOrder(r *ghttp.Request) {
	scope, ok := workOrderScopeForRequest(r)
	if !ok {
		return
	}
	id := r.Get("id").Uint()
	var expectedVersion *int
	if raw := strings.TrimSpace(r.Get("expected_version").String()); raw != "" {
		version, err := strconv.Atoi(raw)
		if err != nil {
			writeWorkOrderError(r, fmt.Errorf("expected_version must be an integer"))
			return
		}
		expectedVersion = &version
	}
	if err := s.workOrderService().DeleteWorkOrderWithVersion(scope, id, expectedVersion); err != nil {
		writeWorkOrderError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "success"})
}
