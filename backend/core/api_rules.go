package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type rulePayload struct {
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	GroupID       *uint               `json:"group_id"`
	Triggers      []RuleTrigger       `json:"triggers"`
	Conditions    *RuleConditionGroup `json:"conditions"`
	Actions       []RuleAction        `json:"actions"`
	EffectiveTime *RuleEffectiveTime  `json:"effective_time"`
	ThrottleSec   int                 `json:"throttle_sec"`
	MaxPerHour    int                 `json:"max_per_hour"`
	RetryCount    int                 `json:"retry_count"`
	Priority      int                 `json:"priority"`
	Enable        bool                `json:"enable"`
}

type ruleGroupPayload struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

func (s *Server) RegisterRuleRoutes(group *ghttp.RouterGroup) {
	permissionGET(group, "/rules", "rule:list", s.handleListRules)
	permissionPOST(group, "/rules", "rule:create", s.handleCreateRule)
	permissionPOST(group, "/rules/analyze", "rule:list", s.handleAnalyzeRule)
	permissionGET(group, "/rules/device-options", "rule:list", s.handleRuleDeviceOptions)
	permissionGET(group, "/rules/:code", "rule:detail", s.handleGetRule)
	permissionPUT(group, "/rules/:code", "rule:edit", s.handleUpdateRule)
	permissionDELETE(group, "/rules/:code", "rule:delete", s.handleDeleteRule)
	permissionPUT(group, "/rules/:code/enable", "rule:enable", s.handleEnableRule)
	permissionPUT(group, "/rules/:code/disable", "rule:enable", s.handleDisableRule)
	permissionGET(group, "/rules/:code/logs", "rule:log", s.handleRuleLogs)

	permissionGET(group, "/rule-groups", "rule:list", s.handleListRuleGroups)
	permissionPOST(group, "/rule-groups", "rule_group:manage", s.handleCreateRuleGroup)
	permissionPUT(group, "/rule-groups/:id", "rule_group:manage", s.handleUpdateRuleGroup)
	permissionDELETE(group, "/rule-groups/:id", "rule_group:manage", s.handleDeleteRuleGroup)
}

func (s *Server) handleListRules(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	rules, total, err := store.ListRules(page, pageSize, tenantID, projectID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": rules, "total": total})
}

func (s *Server) handleGetRule(r *ghttp.Request) {
	rule, ok := s.ruleForRequest(r)
	if !ok {
		return
	}
	def, err := RuleDefinitionFromStore(rule)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"rule":           rule,
		"triggers":       def.Triggers,
		"conditions":     def.Conditions,
		"actions":        def.Actions,
		"effective_time": def.EffectiveTime,
	}})
}

func (s *Server) handleCreateRule(r *ghttp.Request) {
	payload, ok := parseRulePayload(r)
	if !ok {
		return
	}
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	rule, err := s.buildRuleFromPayload("", payload, tenantID, projectID, requestUserID(r))
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	if payload.Enable {
		if err := s.validateRuleControlPermission(r, rule); err != nil {
			r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
			return
		}
		rule.Enabled = true
		rule.Status = RuleStatusEnabled
	}
	if err := store.SaveRule(rule); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.reloadRules()
	r.Response.WriteJson(g.Map{"code": 0, "data": rule})
}

func (s *Server) handleUpdateRule(r *ghttp.Request) {
	existing, ok := s.ruleForRequest(r)
	if !ok {
		return
	}
	payload, ok := parseRulePayload(r)
	if !ok {
		return
	}
	rule, err := s.buildRuleFromPayload(existing.Code, payload, existing.TenantID, existing.ProjectID, existing.EnabledBy)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	rule.ID = existing.ID
	rule.CreatedAt = existing.CreatedAt
	rule.Enabled = existing.Enabled
	rule.Status = existing.Status
	rule.LastTriggeredAt = existing.LastTriggeredAt
	rule.TriggerCount = existing.TriggerCount
	rule.ErrorMessage = existing.ErrorMessage
	rule.EnabledBy = existing.EnabledBy
	if payload.Enable {
		if err := s.validateRuleControlPermission(r, rule); err != nil {
			r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
			return
		}
		rule.Enabled = true
		rule.Status = RuleStatusEnabled
		rule.EnabledBy = requestUserID(r)
	} else if !rule.Enabled {
		rule.Status = RuleStatusDraft
	}
	if err := store.SaveRule(rule); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.reloadRules()
	r.Response.WriteJson(g.Map{"code": 0, "data": rule})
}

func (s *Server) handleDeleteRule(r *ghttp.Request) {
	rule, ok := s.ruleForRequest(r)
	if !ok {
		return
	}
	if rule.Enabled {
		r.Response.WriteJson(g.Map{"code": 400, "message": "enabled rule cannot be deleted"})
		return
	}
	if err := store.DeleteRule(rule.Code); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.reloadRules()
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleEnableRule(r *ghttp.Request) {
	rule, ok := s.ruleForRequest(r)
	if !ok {
		return
	}
	if err := s.validateRuleControlPermission(r, rule); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}
	rule.Enabled = true
	rule.Status = RuleStatusEnabled
	rule.EnabledBy = requestUserID(r)
	s.applyRuleScope(rule)
	if err := store.SaveRule(rule); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.reloadRules()
	r.Response.WriteJson(g.Map{"code": 0, "data": rule})
}

func (s *Server) handleDisableRule(r *ghttp.Request) {
	rule, ok := s.ruleForRequest(r)
	if !ok {
		return
	}
	rule.Enabled = false
	rule.Status = RuleStatusDisabled
	rule.SyncState = "disabled"
	if err := store.SaveRule(rule); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.reloadRules()
	r.Response.WriteJson(g.Map{"code": 0, "data": rule})
}

func (s *Server) handleAnalyzeRule(r *ghttp.Request) {
	payload, ok := parseRulePayload(r)
	if !ok {
		return
	}
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	rule, err := s.buildRuleFromPayload("", payload, tenantID, projectID, requestUserID(r))
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"scope": rule.Scope, "gateway_sn": rule.GatewaySN, "sync_state": rule.SyncState}})
}

func (s *Server) handleRuleLogs(r *ghttp.Request) {
	rule, ok := s.ruleForRequest(r)
	if !ok {
		return
	}
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	logs, total, err := store.ListRuleExecLogs(rule.Code, page, pageSize)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": logs, "total": total})
}

func (s *Server) handleRuleDeviceOptions(r *ghttp.Request) {
	devices := s.DeviceManager.Registry.GetAllDevices()
	items := make([]g.Map, 0, len(devices))
	for _, device := range devices {
		if device == nil || !canAccessDevice(r, device) {
			continue
		}
		product, _ := s.DeviceManager.Registry.GetProduct(device.ProductCode)
		productName := ""
		productConfig := ""
		protocolName := ""
		if product != nil {
			productName = product.Name
			productConfig = product.Config
		}
		protocolName, _ = s.DeviceManager.Registry.GetEffectiveProtocol(device.Code)
		tsl, _ := ParseProductTSL(productConfig)
		status, _ := s.DeviceManager.GetStatus(device.Code)
		gatewaySN := ""
		if s.RuleEngine != nil && s.RuleEngine.distributor != nil {
			gatewaySN = s.RuleEngine.distributor.getDeviceGateway(device)
		}
		items = append(items, g.Map{
			"code":          device.Code,
			"name":          device.Name,
			"product_code":  device.ProductCode,
			"product_name":  productName,
			"protocol_name": protocolName,
			"parent_code":   device.ParentCode,
			"gateway_sn":    gatewaySN,
			"enabled":       device.Enabled,
			"online":        status.Online,
			"properties":    tsl.Properties,
			"events":        tsl.Events,
			"services":      tsl.Services,
		})
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": items})
}

func (s *Server) handleListRuleGroups(r *ghttp.Request) {
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	groups, err := store.ListRuleGroups(tenantID, projectID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": groups})
}

func (s *Server) handleCreateRuleGroup(r *ghttp.Request) {
	payload, ok := parseRuleGroupPayload(r)
	if !ok {
		return
	}
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	group := &store.RuleGroup{TenantID: tenantID, ProjectID: projectID, Name: strings.TrimSpace(payload.Name), Description: payload.Description, SortOrder: payload.SortOrder}
	if group.Name == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "group name is required"})
		return
	}
	if err := store.SaveRuleGroup(group); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": group})
}

func (s *Server) handleUpdateRuleGroup(r *ghttp.Request) {
	payload, ok := parseRuleGroupPayload(r)
	if !ok {
		return
	}
	id := r.Get("id").Uint()
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	var group store.RuleGroup
	if err := store.DB.Where("id = ? AND tenant_id = ? AND project_id = ?", id, tenantID, projectID).First(&group).Error; err != nil {
		writeRuleLookupError(r, err)
		return
	}
	group.Name = strings.TrimSpace(payload.Name)
	group.Description = payload.Description
	group.SortOrder = payload.SortOrder
	if group.Name == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "group name is required"})
		return
	}
	if err := store.SaveRuleGroup(&group); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": group})
}

func (s *Server) handleDeleteRuleGroup(r *ghttp.Request) {
	id := r.Get("id").Uint()
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return
	}
	var count int64
	if err := store.DB.Model(&store.Rule{}).Where("group_id = ? AND tenant_id = ? AND project_id = ?", id, tenantID, projectID).Count(&count).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	if count > 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "group is used by rules"})
		return
	}
	if err := store.DeleteRuleGroup(id); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func parseRulePayload(r *ghttp.Request) (rulePayload, bool) {
	var payload rulePayload
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return payload, false
	}
	return payload, true
}

func parseRuleGroupPayload(r *ghttp.Request) (ruleGroupPayload, bool) {
	var payload ruleGroupPayload
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return payload, false
	}
	return payload, true
}

func (s *Server) buildRuleFromPayload(code string, payload rulePayload, tenantID, projectID, enabledBy uint) (*store.Rule, error) {
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	def := RuleDefinition{Triggers: payload.Triggers, Conditions: payload.Conditions, Actions: payload.Actions, EffectiveTime: payload.EffectiveTime}
	if err := ValidateRuleDefinition(def); err != nil {
		return nil, err
	}
	triggersJSON, err := EncodeRulePart(payload.Triggers)
	if err != nil {
		return nil, err
	}
	conditionsJSON := ""
	if payload.Conditions != nil {
		conditionsJSON, err = EncodeRulePart(payload.Conditions)
		if err != nil {
			return nil, err
		}
	}
	actionsJSON, err := EncodeRulePart(payload.Actions)
	if err != nil {
		return nil, err
	}
	effectiveTimeJSON := ""
	if payload.EffectiveTime != nil {
		effectiveTimeJSON, err = EncodeRulePart(payload.EffectiveTime)
		if err != nil {
			return nil, err
		}
	}
	if code == "" {
		code = "rule_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	rule := &store.Rule{
		TenantID:      tenantID,
		ProjectID:     projectID,
		Code:          code,
		Name:          payload.Name,
		Description:   payload.Description,
		GroupID:       payload.GroupID,
		Priority:      payload.Priority,
		Status:        RuleStatusDraft,
		Triggers:      triggersJSON,
		Conditions:    conditionsJSON,
		Actions:       actionsJSON,
		EffectiveTime: effectiveTimeJSON,
		ThrottleSec:   payload.ThrottleSec,
		MaxPerHour:    payload.MaxPerHour,
		RetryCount:    payload.RetryCount,
		EnabledBy:     enabledBy,
	}
	if rule.Priority <= 0 {
		rule.Priority = 50
	}
	if rule.ThrottleSec <= 0 {
		rule.ThrottleSec = 60
	}
	if rule.MaxPerHour <= 0 {
		rule.MaxPerHour = 60
	}
	s.applyRuleScope(rule)
	return rule, nil
}

func (s *Server) applyRuleScope(rule *store.Rule) {
	if s.RuleEngine == nil || s.RuleEngine.distributor == nil {
		rule.Scope = RuleScopePlatform
		rule.GatewaySN = ""
		rule.SyncState = "local"
		return
	}
	scope, gatewaySN := s.RuleEngine.distributor.AnalyzeScope(rule)
	rule.Scope = scope
	rule.GatewaySN = gatewaySN
	if scope == RuleScopeGateway {
		rule.SyncState = "pending"
	} else {
		rule.SyncState = "local"
	}
}

func (s *Server) validateRuleControlPermission(r *ghttp.Request, rule *store.Rule) error {
	def, err := RuleDefinitionFromStore(rule)
	if err != nil {
		return err
	}
	for _, code := range collectRuleDeviceCodes(def) {
		device, err := store.GetDevice(code)
		if err != nil {
			return fmt.Errorf("device %s not found", code)
		}
		if !canAccessDevice(r, device) {
			return fmt.Errorf("no access to device %s", code)
		}
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil {
		return fmt.Errorf("auth context not found")
	}
	for _, code := range collectWritableRuleDeviceCodes(def.Actions) {
		device, err := store.GetDevice(code)
		if err != nil {
			return fmt.Errorf("device %s not found", code)
		}
		if !authCtx.HasProjectPermission("device:control", device.ProjectID) {
			return fmt.Errorf("missing device control permission for %s", code)
		}
		allowed, err := canWriteDeviceByTagPermission(authCtx, currentDeviceTagScope(r), code)
		if err != nil {
			return err
		}
		if !allowed {
			return fmt.Errorf("no write permission for device %s", code)
		}
	}
	return nil
}

func collectWritableRuleDeviceCodes(actions []RuleAction) []string {
	seen := map[string]bool{}
	var walk func([]RuleAction)
	walk = func(items []RuleAction) {
		for _, action := range items {
			switch action.Type {
			case RuleActionSetProperty, RuleActionCallService:
				addDeviceCode(seen, action.DeviceCode)
			case RuleActionAlarm:
				if action.AlarmDevice != "" && action.AlarmDevice != "trigger" {
					addDeviceCode(seen, action.AlarmDevice)
				}
			case RuleActionParallelGroup:
				walk(action.SubActions)
			}
		}
	}
	walk(actions)
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	return codes
}

func (s *Server) ruleForRequest(r *ghttp.Request) (*store.Rule, bool) {
	code := r.Get("code").String()
	rule, err := store.GetRule(code)
	if err != nil {
		writeRuleLookupError(r, err)
		return nil, false
	}
	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": scopeErr.Error()})
		return nil, false
	}
	if (tenantID > 0 && rule.TenantID != tenantID) || (projectID > 0 && rule.ProjectID != projectID) {
		r.Response.WriteJson(g.Map{"code": 404, "message": "rule not found"})
		return nil, false
	}
	return rule, true
}

func writeRuleLookupError(r *ghttp.Request, err error) {
	if err == gorm.ErrRecordNotFound {
		r.Response.WriteJson(g.Map{"code": 404, "message": "not found"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
}

func requestUserID(r *ghttp.Request) uint {
	if authCtx := requestAuthContext(r); authCtx != nil {
		return authCtx.UserID
	}
	return 0
}

func (s *Server) reloadRules() {
	if s.RuleEngine != nil {
		if err := s.RuleEngine.LoadRules(); err != nil && s.Logger != nil {
			s.Logger.Warn("failed to reload rules", zap.Error(err))
		}
	}
}
