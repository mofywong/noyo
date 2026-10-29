package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"noyo/core/store"
)

type WorkOrderRoutingContext struct {
	Location   string `json:"location"`
	DeviceCode string `json:"device_code"`
	IssueCode  string `json:"issue_code"`
}

type WorkOrderPolicyRoute struct {
	NodeID       string   `json:"node_id"`
	Location     string   `json:"location"`
	DeviceCodes  []string `json:"device_codes"`
	IssueCode    string   `json:"issue_code"`
	RoleID       uint     `json:"role_id"`
	DeliveryMode string   `json:"delivery_mode"`
}

type WorkOrderSLAPolicy struct {
	ResponseMinutes   int      `json:"response_minutes"`
	ResolutionMinutes int      `json:"resolution_minutes"`
	WarningMinutes    int      `json:"warning_minutes"`
	Timezone          string   `json:"timezone"`
	Weekdays          []int    `json:"weekdays"`
	StartHour         int      `json:"start_hour"`
	EndHour           int      `json:"end_hour"`
	Holidays          []string `json:"holidays"`
	PauseReasons      []string `json:"pause_reasons"`
}

type WorkOrderPolicyDefinition struct {
	TemplateID      uint                   `json:"template_id"`
	FormVersion     int                    `json:"form_version"`
	WorkflowVersion int                    `json:"workflow_version"`
	Routes          []WorkOrderPolicyRoute `json:"routes"`
	SLA             *WorkOrderSLAPolicy    `json:"sla,omitempty"`
}

type WorkOrderPolicyPublishInput struct {
	Code            string                    `json:"code"`
	Name            string                    `json:"name"`
	ExpectedVersion int                       `json:"expected_version"`
	Definition      WorkOrderPolicyDefinition `json:"definition"`
}

type WorkOrderPolicyDetail struct {
	Policy   store.WorkOrderHandlingPolicy          `json:"policy"`
	Versions []store.WorkOrderHandlingPolicyVersion `json:"versions"`
}

func policyScope(db *gorm.DB, scope WorkOrderScope) *gorm.DB {
	return db.Where("tenant_id = ? AND project_id = ?", scope.TenantID, scope.ProjectID)
}

func (s *WorkOrderService) ListWorkOrderPolicies(scope WorkOrderScope) ([]WorkOrderPolicyDetail, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	var heads []store.WorkOrderHandlingPolicy
	if err := policyScope(s.db, scope).Order("id DESC").Find(&heads).Error; err != nil {
		return nil, err
	}
	result := make([]WorkOrderPolicyDetail, 0, len(heads))
	if len(heads) == 0 {
		return result, nil
	}
	var versions []store.WorkOrderHandlingPolicyVersion
	if err := policyScope(s.db, scope).Order("version DESC").Find(&versions).Error; err != nil {
		return nil, err
	}
	for _, head := range heads {
		item := WorkOrderPolicyDetail{Policy: head, Versions: []store.WorkOrderHandlingPolicyVersion{}}
		for _, version := range versions {
			if version.PolicyID == head.ID {
				item.Versions = append(item.Versions, version)
			}
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *WorkOrderService) loadWorkOrderPolicy(tx *gorm.DB, scope WorkOrderScope, versionID uint, allowDisabled ...bool) (*store.WorkOrderHandlingPolicyVersion, WorkOrderPolicyDefinition, error) {
	var version store.WorkOrderHandlingPolicyVersion
	var definition WorkOrderPolicyDefinition
	if err := policyScope(tx, scope).First(&version, versionID).Error; err != nil {
		return nil, definition, err
	}
	var head store.WorkOrderHandlingPolicy
	if err := policyScope(tx, scope).First(&head, version.PolicyID).Error; err != nil {
		return nil, definition, err
	}
	if !head.Enabled && !(len(allowDisabled) > 0 && allowDisabled[0]) {
		return nil, definition, fmt.Errorf("handling policy is disabled")
	}
	if err := json.Unmarshal([]byte(version.Definition), &definition); err != nil {
		return nil, definition, err
	}
	return &version, definition, nil
}

// Publish appends a definition and atomically advances the head. A retry must
// supply the new revision; an old worker continues using its pinned version.
func (s *WorkOrderService) PublishWorkOrderPolicy(scope WorkOrderScope, policyID uint, input WorkOrderPolicyPublishInput) (*store.WorkOrderHandlingPolicyVersion, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	input.Code, input.Name = strings.TrimSpace(input.Code), strings.TrimSpace(input.Name)
	if scope.ActorUserID == 0 || input.Code == "" || len(input.Code) > 64 || input.Name == "" || len([]rune(input.Name)) > 128 {
		return nil, fmt.Errorf("policy code, name and author are required")
	}
	var result store.WorkOrderHandlingPolicyVersion
	err := s.db.Transaction(func(tx *gorm.DB) error {
		definition := input.Definition
		if err := s.validateWorkOrderPolicy(tx, scope, &definition); err != nil {
			return err
		}
		head := store.WorkOrderHandlingPolicy{TenantID: scope.TenantID, ProjectID: scope.ProjectID, Code: input.Code, Name: input.Name, Enabled: true}
		if policyID == 0 {
			if input.ExpectedVersion != 0 {
				return ErrWorkOrderVersionConflict
			}
			if err := tx.Create(&head).Error; err != nil {
				return err
			}
		} else {
			if err := policyScope(tx, scope).First(&head, policyID).Error; err != nil {
				return err
			}
			if head.Code != input.Code {
				return fmt.Errorf("published policy code cannot change")
			}
			if head.CurrentVersion != input.ExpectedVersion {
				return ErrWorkOrderVersionConflict
			}
		}
		raw, err := json.Marshal(definition)
		if err != nil {
			return err
		}
		result = store.WorkOrderHandlingPolicyVersion{TenantID: scope.TenantID, ProjectID: scope.ProjectID, PolicyID: head.ID, Version: head.CurrentVersion + 1, Definition: string(raw), PublishedBy: scope.ActorUserID}
		changed := policyScope(tx.Model(&store.WorkOrderHandlingPolicy{}), scope).Where("id = ? AND current_version = ?", head.ID, head.CurrentVersion).Updates(map[string]any{"current_version": result.Version, "name": input.Name})
		if changed.Error != nil {
			return changed.Error
		}
		if changed.RowsAffected != 1 {
			return ErrWorkOrderVersionConflict
		}
		return tx.Create(&result).Error
	})
	return &result, err
}

func (s *WorkOrderService) SetWorkOrderPolicyEnabled(scope WorkOrderScope, id uint, expected int, enabled bool) error {
	if err := validateWorkOrderScope(scope); err != nil {
		return err
	}
	changed := policyScope(s.db.Model(&store.WorkOrderHandlingPolicy{}), scope).Where("id = ? AND current_version = ?", id, expected).Update("enabled", enabled)
	if changed.Error != nil {
		return changed.Error
	}
	if changed.RowsAffected != 1 {
		return ErrWorkOrderVersionConflict
	}
	return nil
}

func (s *WorkOrderService) validateWorkOrderPolicy(tx *gorm.DB, scope WorkOrderScope, def *WorkOrderPolicyDefinition) error {
	template, err := s.findTemplate(tx, scope, def.TemplateID)
	if err != nil {
		return err
	}
	if !template.Enabled {
		return fmt.Errorf("template is disabled")
	}
	if def.FormVersion == 0 {
		def.FormVersion = template.CurrentFormVersion
	}
	if def.WorkflowVersion == 0 {
		def.WorkflowVersion = template.CurrentWorkflowVersion
	}
	var form store.WorkOrderFormVersion
	if err := policyScope(tx, scope).Where("template_id = ? AND version = ?", template.ID, def.FormVersion).First(&form).Error; err != nil {
		return err
	}
	var workflow store.WorkOrderWorkflowVersion
	if err := policyScope(tx, scope).Where("template_id = ? AND version = ?", template.ID, def.WorkflowVersion).First(&workflow).Error; err != nil {
		return err
	}
	graph, err := decodeWorkOrderWorkflowDefinition(workflow.Definition)
	if err != nil {
		return err
	}
	nodes := map[string]WorkOrderWorkflowNode{}
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	if len(def.Routes) > 200 {
		return fmt.Errorf("at most 200 responsibility routes are supported")
	}
	for i := range def.Routes {
		route := &def.Routes[i]
		route.Location = strings.TrimSpace(route.Location)
		route.IssueCode = strings.TrimSpace(route.IssueCode)
		node, ok := nodes[route.NodeID]
		if !ok || node.Type != WorkOrderWorkflowNodeUserTask {
			return fmt.Errorf("route must target a user task: %s", route.NodeID)
		}
		if _, err := workOrderAssignmentRole(tx, scope, route.RoleID); err != nil {
			return err
		}
		if route.DeliveryMode == "" {
			route.DeliveryMode = "broadcast"
		}
		if route.DeliveryMode != "broadcast" && route.DeliveryMode != "claim" {
			return fmt.Errorf("invalid delivery mode")
		}
		if route.DeliveryMode == "claim" && node.CompletionMode != WorkOrderApprovalAny {
			return fmt.Errorf("claim delivery requires an any-completion node")
		}
		if len(route.DeviceCodes) > 200 {
			return fmt.Errorf("at most 200 devices per route")
		}
		for j, code := range route.DeviceCodes {
			route.DeviceCodes[j] = strings.TrimSpace(code)
			if err := validatePolicyDevice(tx, scope, route.DeviceCodes[j]); err != nil {
				return err
			}
		}
		for j := 0; j < i; j++ {
			if routesOverlap(def.Routes[j], *route) {
				return fmt.Errorf("ambiguous responsibility routes for node %s", route.NodeID)
			}
		}
	}
	return validateWorkOrderSLAPolicy(def.SLA)
}

func validatePolicyDevice(tx *gorm.DB, scope WorkOrderScope, code string) error {
	if code == "" {
		return fmt.Errorf("device code is required")
	}
	var count int64
	if err := policyScope(tx.Model(&store.Device{}), scope).Where("code = ?", code).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("device is outside the current project: %s", code)
	}
	return nil
}

func routeWeight(route WorkOrderPolicyRoute) int {
	n := 0
	if route.Location != "" {
		n++
	}
	if route.IssueCode != "" {
		n++
	}
	if len(route.DeviceCodes) > 0 {
		n++
	}
	return n
}
func containsPolicyValue(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func routesOverlap(a, b WorkOrderPolicyRoute) bool {
	if a.NodeID != b.NodeID || routeWeight(a) != routeWeight(b) {
		return false
	}
	if a.Location != "" && b.Location != "" && a.Location != b.Location {
		return false
	}
	if a.IssueCode != "" && b.IssueCode != "" && a.IssueCode != b.IssueCode {
		return false
	}
	if len(a.DeviceCodes) > 0 && len(b.DeviceCodes) > 0 {
		for _, v := range a.DeviceCodes {
			if containsPolicyValue(b.DeviceCodes, v) {
				return true
			}
		}
		return false
	}
	return true
}

func effectiveWorkOrderNode(order *store.WorkOrder, node WorkOrderWorkflowNode) (WorkOrderWorkflowNode, error) {
	if order.PolicySnapshot == "" {
		return node, nil
	}
	var def WorkOrderPolicyDefinition
	var ctx WorkOrderRoutingContext
	if err := json.Unmarshal([]byte(order.PolicySnapshot), &def); err != nil {
		return node, err
	}
	if err := json.Unmarshal([]byte(order.RoutingContext), &ctx); err != nil {
		return node, err
	}
	best := -1
	for _, route := range def.Routes {
		if route.NodeID != node.ID || (route.Location != "" && route.Location != ctx.Location) || (route.IssueCode != "" && route.IssueCode != ctx.IssueCode) || (len(route.DeviceCodes) > 0 && !containsPolicyValue(route.DeviceCodes, ctx.DeviceCode)) {
			continue
		}
		weight := routeWeight(route)
		if weight < best {
			continue
		}
		if weight == best {
			return node, fmt.Errorf("ambiguous responsibility routes")
		}
		best = weight
		node.Assignment = &WorkOrderAssignment{Kind: "project_role", RoleID: route.RoleID, DeliveryMode: route.DeliveryMode}
		node.ParticipantUserIDs = nil
	}
	return node, nil
}

func (s *WorkOrderService) prepareWorkOrderPolicy(tx *gorm.DB, input *WorkOrderCreateInput) (string, string, error) {
	if input.PolicyVersionID == 0 {
		return "", "", nil
	}
	version, def, err := s.loadWorkOrderPolicy(tx, input.Scope, input.PolicyVersionID)
	if err != nil {
		return "", "", err
	}
	if input.TemplateID != 0 && input.TemplateID != def.TemplateID {
		return "", "", fmt.Errorf("template does not match handling policy")
	}
	input.TemplateID, input.FormVersion, input.WorkflowVersion = def.TemplateID, def.FormVersion, def.WorkflowVersion
	ctx := input.RoutingContext
	if ctx.DeviceCode != "" {
		if err := validatePolicyDevice(tx, input.Scope, ctx.DeviceCode); err != nil {
			return "", "", err
		}
	}
	if len(ctx.Location) > 256 || len(ctx.IssueCode) > 128 {
		return "", "", fmt.Errorf("routing context exceeds maximum length")
	}
	raw, err := json.Marshal(ctx)
	return version.Definition, string(raw), err
}

type WorkOrderPolicyPreview struct {
	NodeID       string `json:"node_id"`
	RoleID       uint   `json:"role_id"`
	UserIDs      []uint `json:"user_ids"`
	DeliveryMode string `json:"delivery_mode"`
	Warning      string `json:"warning,omitempty"`
}

func (s *WorkOrderService) PreviewWorkOrderPolicy(scope WorkOrderScope, versionID uint, ctx WorkOrderRoutingContext) ([]WorkOrderPolicyPreview, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return nil, err
	}
	version, def, err := s.loadWorkOrderPolicy(s.db, scope, versionID)
	if err != nil {
		return nil, err
	}
	if ctx.DeviceCode != "" {
		if err := validatePolicyDevice(s.db, scope, ctx.DeviceCode); err != nil {
			return nil, err
		}
	}
	var versionRow store.WorkOrderWorkflowVersion
	if err := policyScope(s.db, scope).Where("template_id = ? AND version = ?", def.TemplateID, def.WorkflowVersion).First(&versionRow).Error; err != nil {
		return nil, err
	}
	workflow, err := decodeWorkOrderWorkflowDefinition(versionRow.Definition)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(ctx)
	order := &store.WorkOrder{TenantID: scope.TenantID, ProjectID: scope.ProjectID, PolicySnapshot: version.Definition, RoutingContext: string(raw)}
	result := []WorkOrderPolicyPreview{}
	for _, node := range workflow.Nodes {
		if node.Type != WorkOrderWorkflowNodeUserTask {
			continue
		}
		effective, err := effectiveWorkOrderNode(order, node)
		if err != nil {
			return nil, err
		}
		item := WorkOrderPolicyPreview{NodeID: node.ID, UserIDs: []uint{}, DeliveryMode: "broadcast"}
		if effective.Assignment != nil {
			item.RoleID = effective.Assignment.RoleID
			item.DeliveryMode = effective.Assignment.DeliveryMode
		}
		users, err := s.resolveGraphTaskParticipants(s.db, order, effective)
		var unavailable *workOrderAssignmentUnavailable
		if err != nil {
			if !errors.As(err, &unavailable) {
				return nil, err
			}
			item.Warning = unavailable.Reason
		}
		for _, user := range users {
			item.UserIDs = append(item.UserIDs, user.ID)
		}
		result = append(result, item)
	}
	return result, nil
}

// SLA is opt-in. No deadlines are inferred from priority or inspection type.
func validateWorkOrderSLAPolicy(p *WorkOrderSLAPolicy) error {
	if p == nil {
		return nil
	}
	if p.ResponseMinutes < 1 || p.ResolutionMinutes < p.ResponseMinutes || p.ResolutionMinutes > 525600 || p.WarningMinutes < 0 || p.WarningMinutes >= p.ResponseMinutes {
		return fmt.Errorf("invalid SLA durations")
	}
	if _, err := time.LoadLocation(p.Timezone); err != nil {
		return fmt.Errorf("invalid SLA timezone: %w", err)
	}
	if len(p.Weekdays) > 0 && (p.StartHour < 0 || p.EndHour > 24 || p.EndHour <= p.StartHour) {
		return fmt.Errorf("invalid SLA working hours")
	}
	for _, day := range p.Weekdays {
		if day < 0 || day > 6 {
			return fmt.Errorf("invalid SLA weekday")
		}
	}
	for _, day := range p.Holidays {
		if _, err := time.Parse("2006-01-02", day); err != nil {
			return err
		}
	}
	return nil
}

func (s *WorkOrderService) GetPinnedWorkOrderForm(scope WorkOrderScope, templateID uint, version int) (WorkOrderFormDefinition, error) {
	if err := validateWorkOrderScope(scope); err != nil {
		return WorkOrderFormDefinition{}, err
	}
	var row store.WorkOrderFormVersion
	if err := policyScope(s.db, scope).Where("template_id = ? AND version = ?", templateID, version).First(&row).Error; err != nil {
		return WorkOrderFormDefinition{}, err
	}
	return decodeWorkOrderFormDefinition(row.Definition)
}

type WorkOrderPolicyVersionDetail struct {
	Definition         WorkOrderPolicyDefinition   `json:"definition"`
	FormDefinition     WorkOrderFormDefinition     `json:"form_definition"`
	WorkflowDefinition WorkOrderWorkflowDefinition `json:"workflow_definition"`
}

func (s *WorkOrderService) GetWorkOrderPolicyVersionDetail(scope WorkOrderScope, id uint) (WorkOrderPolicyVersionDetail, error) {
	result := WorkOrderPolicyVersionDetail{}
	if err := validateWorkOrderScope(scope); err != nil {
		return result, err
	}
	_, def, err := s.loadWorkOrderPolicy(s.db, scope, id, true)
	if err != nil {
		return result, err
	}
	result.Definition = def
	result.FormDefinition, err = s.GetPinnedWorkOrderForm(scope, def.TemplateID, def.FormVersion)
	if err != nil {
		return result, err
	}
	var row store.WorkOrderWorkflowVersion
	if err := policyScope(s.db, scope).Where("template_id = ? AND version = ?", def.TemplateID, def.WorkflowVersion).First(&row).Error; err != nil {
		return result, err
	}
	result.WorkflowDefinition, err = decodeWorkOrderWorkflowDefinition(row.Definition)
	return result, err
}
