package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"noyo/core/store"
	"noyo/core/workorder"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) RegisterAlarmCenterRoutes(group *ghttp.RouterGroup) {
	permissionGET(group, "/alarm-instances/:id/media", "alarm:list", s.handleAlarmMedia)
	permissionGET(group, "/alarm-instances/:id/media/:recording/:kind", "alarm:list", s.handleAlarmMediaFile)
	permissionGET(group, "/alarm-instances/stats", "alarm:list", s.handleAlarmStats)
	permissionGET(group, "/alarm-instances", "alarm:list", s.handleListAlarmInstances)
	permissionGET(group, "/alarm-instances/:id", "alarm:list", s.handleGetAlarmInstance)
	permissionGET(group, "/alarm-instances/:id/events", "alarm:list", s.handleListAlarmInstanceEvents)
	permissionPOST(group, "/alarm-instances/:id/acknowledge", "alarm:handle", s.handleAcknowledgeAlarmInstance)
	permissionPOST(group, "/alarm-instances/:id/assign", "alarm:handle", s.handleAssignAlarmInstance)
	permissionPOST(group, "/alarm-instances/:id/shelve", "alarm:handle", s.handleShelveAlarmInstance)
	permissionPOST(group, "/alarm-instances/:id/unshelve", "alarm:handle", s.handleUnshelveAlarmInstance)
	permissionPOST(group, "/alarm-instances/:id/close", "alarm:handle", s.handleCloseAlarmInstance)
	permissionGET(group, "/alarm-policies", "alarm:manage", s.handleListAlarmPolicies)
	permissionPOST(group, "/alarm-policies", "alarm:manage", s.handleSaveAlarmPolicy)
	permissionDELETE(group, "/alarm-policies/:id", "alarm:manage", s.handleDeleteAlarmPolicy)
}

func (s *Server) alarmCenter() (*AlarmCenterService, error) {
	if s.AlarmCenter == nil {
		return nil, fmt.Errorf("alarm center service is unavailable")
	}
	return s.AlarmCenter, nil
}

func alarmScopeForRequest(r *ghttp.Request) (workorder.Scope, uint, bool) {
	tenantID, projectID, err := currentTenantProjectScope(r)
	if err != nil {
		r.Response.WriteStatus(403)
		r.Response.WriteJson(g.Map{"code": 403, "error_code": "scope_forbidden", "message": err.Error()})
		r.ExitAll()
		return workorder.Scope{}, 0, false
	}
	actorUserID := requestUserID(r)
	return workorder.Scope{TenantID: tenantID, ProjectID: projectID, Principal: workorder.Principal{Type: "user", Ref: strconv.FormatUint(uint64(actorUserID), 10)}}, actorUserID, true
}

func (s *Server) handleAlarmStats(r *ghttp.Request) {
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	stats, err := center.Stats(scope)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": stats})
}

func (s *Server) handleListAlarmInstances(r *ghttp.Request) {
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	options := AlarmListOptions{
		Page:          r.Get("page", 1).Int(),
		PageSize:      r.Get("page_size", 10).Int(),
		Condition:     r.Get("condition").String(),
		Handling:      r.Get("handling").String(),
		Severity:      r.Get("severity").String(),
		SourceType:    r.Get("source_type").String(),
		WorkOrderID:   r.Get("work_order_id").String(),
		OwnerUserID:   r.Get("owner_user_id").Uint(),
		Search:        r.Get("search").String(),
		IncludeClosed: r.Get("include_closed").Bool(),
		ClosedToday:   r.Get("closed_today").Bool(),
	}
	items, total, err := center.List(scope, options)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	handlers, err := loadAlarmCurrentHandlers(center.db, scope, items)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	responses, err := alarmInstanceResponses(items, handlers)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"items": responses, "total": total, "page": options.Page, "page_size": options.PageSize}})
}

func (s *Server) handleGetAlarmInstance(r *ghttp.Request) {
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	instance, err := center.Get(scope, r.Get("id").String())
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	handlers, err := loadAlarmCurrentHandlers(center.db, scope, []store.AlarmInstance{*instance})
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	response, err := alarmInstanceResponse(*instance, handlers)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": response})
}

func (s *Server) handleListAlarmInstanceEvents(r *ghttp.Request) {
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	events, err := center.Events(scope, r.Get("id").String())
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	data := make([]g.Map, 0, len(events))
	for _, event := range events {
		payload, err := decodeAlarmJSON(event.Payload)
		if err != nil {
			writeAlarmCenterError(r, err)
			return
		}
		evidence, err := decodeAlarmJSON(event.EvidenceSnapshot)
		if err != nil {
			writeAlarmCenterError(r, err)
			return
		}
		data = append(data, g.Map{"event": event, "created_at": event.CreatedAt, "payload": payload, "evidence": evidence})
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": data})
}

func (s *Server) handleAcknowledgeAlarmInstance(r *ghttp.Request) {
	var payload struct {
		Comment string `json:"comment"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, actorUserID, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	if err := center.Acknowledge(scope, r.Get("id").String(), actorUserID, payload.Comment); err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleAssignAlarmInstance(r *ghttp.Request) {
	var payload struct {
		OwnerUserID uint   `json:"owner_user_id"`
		Comment     string `json:"comment"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, actorUserID, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	if err := center.Assign(scope, r.Get("id").String(), actorUserID, payload.OwnerUserID, payload.Comment); err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleShelveAlarmInstance(r *ghttp.Request) {
	var payload struct {
		Until         string `json:"until"`
		Reason        string `json:"reason"`
		SourceAlarmID string `json:"source_alarm_instance_id"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	var until *time.Time
	if strings.TrimSpace(payload.Until) != "" {
		value, err := time.Parse(time.RFC3339, payload.Until)
		if err != nil {
			writeAlarmCenterError(r, fmt.Errorf("until must be an RFC3339 timestamp"))
			return
		}
		until = &value
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, actorUserID, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	suppression, err := center.Shelve(scope, r.Get("id").String(), actorUserID, until, payload.Reason, payload.SourceAlarmID)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": suppression})
}

func (s *Server) handleUnshelveAlarmInstance(r *ghttp.Request) {
	var payload struct {
		Comment string `json:"comment"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, actorUserID, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	if err := center.Unshelve(scope, r.Get("id").String(), actorUserID, payload.Comment); err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleCloseAlarmInstance(r *ghttp.Request) {
	var payload struct {
		Disposition string `json:"disposition"`
		Comment     string `json:"comment"`
	}
	if !decodeWorkOrderRequest(r, &payload) {
		return
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, actorUserID, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	if err := center.Close(scope, r.Get("id").String(), actorUserID, payload.Disposition, payload.Comment); err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

func (s *Server) handleListAlarmPolicies(r *ghttp.Request) {
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	policies, err := center.ListPolicies(scope)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": policies})
}

func (s *Server) handleSaveAlarmPolicy(r *ghttp.Request) {
	var policy store.AlarmPolicy
	if !decodeWorkOrderRequest(r, &policy) {
		return
	}
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	saved, err := center.SavePolicy(scope, policy)
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": saved})
}

func (s *Server) handleDeleteAlarmPolicy(r *ghttp.Request) {
	center, err := s.alarmCenter()
	if err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	scope, _, ok := alarmScopeForRequest(r)
	if !ok {
		return
	}
	if err := center.DeletePolicy(scope, r.Get("id").Uint()); err != nil {
		writeAlarmCenterError(r, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0})
}

type alarmCurrentHandler struct {
	UserID      uint   `json:"user_id"`
	DisplayName string `json:"display_name,omitempty"`
}

func loadAlarmCurrentHandlers(db *gorm.DB, scope workorder.Scope, instances []store.AlarmInstance) (map[string][]alarmCurrentHandler, error) {
	result := make(map[string][]alarmCurrentHandler)
	publicIDs := make([]string, 0, len(instances))
	seenPublicIDs := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		publicID := strings.TrimSpace(instance.WorkOrderPublicID)
		if publicID == "" {
			continue
		}
		if _, exists := seenPublicIDs[publicID]; exists {
			continue
		}
		seenPublicIDs[publicID] = struct{}{}
		publicIDs = append(publicIDs, publicID)
		result[publicID] = []alarmCurrentHandler{}
	}
	if len(publicIDs) == 0 {
		return result, nil
	}

	var orders []store.WorkOrder
	if err := db.Where("tenant_id = ? AND project_id = ? AND public_id IN ?", scope.TenantID, scope.ProjectID, publicIDs).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("load linked work orders for alarm handlers: %w", err)
	}
	orderPublicIDs := make(map[uint]string, len(orders))
	orderIDs := make([]uint, 0, len(orders))
	seenHandlers := make(map[string]map[uint]struct{}, len(orders))
	appendHandler := func(publicID string, handler alarmCurrentHandler) {
		if handler.UserID == 0 {
			return
		}
		if seenHandlers[publicID] == nil {
			seenHandlers[publicID] = make(map[uint]struct{})
		}
		if _, exists := seenHandlers[publicID][handler.UserID]; exists {
			return
		}
		seenHandlers[publicID][handler.UserID] = struct{}{}
		result[publicID] = append(result[publicID], handler)
	}
	for _, order := range orders {
		orderPublicIDs[order.ID] = order.PublicID
		orderIDs = append(orderIDs, order.ID)
	}
	if len(orderIDs) == 0 {
		return result, nil
	}

	var tasks []store.WorkOrderTask
	if err := db.Where("tenant_id = ? AND project_id = ? AND work_order_id IN ? AND status = ?", scope.TenantID, scope.ProjectID, orderIDs, WorkOrderTaskPending).
		Order("created_at ASC, id ASC").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("load pending work order tasks for alarm handlers: %w", err)
	}
	for _, task := range tasks {
		appendHandler(orderPublicIDs[task.WorkOrderID], alarmCurrentHandler{UserID: task.AssigneeUserID, DisplayName: strings.TrimSpace(task.AssigneeDisplayName)})
	}

	var approvals []store.WorkOrderApprovalTask
	if err := db.Where("tenant_id = ? AND project_id = ? AND work_order_id IN ? AND status = ?", scope.TenantID, scope.ProjectID, orderIDs, WorkOrderApprovalPending).
		Order("created_at ASC, id ASC").Find(&approvals).Error; err != nil {
		return nil, fmt.Errorf("load pending work order approvals for alarm handlers: %w", err)
	}
	for _, approval := range approvals {
		appendHandler(orderPublicIDs[approval.WorkOrderID], alarmCurrentHandler{UserID: approval.ApproverUserID})
	}

	for _, order := range orders {
		if len(result[order.PublicID]) == 0 {
			appendHandler(order.PublicID, alarmCurrentHandler{UserID: order.AssigneeUserID})
		}
	}

	missingUserIDs := make([]uint, 0)
	missingSet := make(map[uint]struct{})
	for _, handlers := range result {
		for _, handler := range handlers {
			if strings.TrimSpace(handler.DisplayName) != "" {
				continue
			}
			if _, exists := missingSet[handler.UserID]; !exists {
				missingSet[handler.UserID] = struct{}{}
				missingUserIDs = append(missingUserIDs, handler.UserID)
			}
		}
	}
	if len(missingUserIDs) == 0 {
		return result, nil
	}
	var users []store.User
	if err := db.Where("tenant_id = ? AND id IN ?", scope.TenantID, missingUserIDs).Find(&users).Error; err != nil {
		return nil, fmt.Errorf("load alarm handler names: %w", err)
	}
	userNames := make(map[uint]string, len(users))
	for _, user := range users {
		name := strings.TrimSpace(user.DisplayName)
		if name == "" {
			name = strings.TrimSpace(user.Username)
		}
		userNames[user.ID] = name
	}
	for publicID, handlers := range result {
		for index := range handlers {
			if strings.TrimSpace(handlers[index].DisplayName) == "" {
				handlers[index].DisplayName = userNames[handlers[index].UserID]
			}
		}
		result[publicID] = handlers
	}
	return result, nil
}

func alarmInstanceResponses(instances []store.AlarmInstance, handlers map[string][]alarmCurrentHandler) ([]g.Map, error) {
	responses := make([]g.Map, 0, len(instances))
	for _, instance := range instances {
		response, err := alarmInstanceResponse(instance, handlers)
		if err != nil {
			return nil, err
		}
		responses = append(responses, response)
	}
	return responses, nil
}

func alarmInstanceResponse(instance store.AlarmInstance, handlers map[string][]alarmCurrentHandler) (g.Map, error) {
	evidence, err := decodeAlarmJSON(instance.EvidenceSnapshot)
	if err != nil {
		return nil, err
	}
	clearedEvidence, err := decodeAlarmJSON(instance.ClearedEvidenceSnapshot)
	if err != nil {
		return nil, err
	}
	policy, err := decodeAlarmJSON(instance.PolicySnapshot)
	if err != nil {
		return nil, err
	}
	currentHandlers := []alarmCurrentHandler{}
	if linkedHandlers, exists := handlers[strings.TrimSpace(instance.WorkOrderPublicID)]; exists {
		currentHandlers = linkedHandlers
	}
	return g.Map{"alarm": instance, "evidence": evidence, "cleared_evidence": clearedEvidence, "policy": policy, "current_handlers": currentHandlers}, nil
}

func decodeAlarmJSON(value string) (map[string]any, error) {
	result := map[string]any{}
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(value), &result); err != nil {
		return nil, fmt.Errorf("decode persisted alarm JSON: %w", err)
	}
	return result, nil
}

func writeAlarmCenterError(r *ghttp.Request, err error) {
	status, code, errorCode := 500, 500, "internal_error"
	switch {
	case errors.Is(err, ErrAlarmNotFound):
		status, code, errorCode = 404, 404, "alarm_not_found"
	case errors.Is(err, ErrAlarmConflict):
		status, code, errorCode = 409, 409, "alarm_version_conflict"
	case errors.Is(err, ErrAlarmInvalid):
		status, code, errorCode = 422, 422, "alarm_validation_failed"
	case strings.Contains(strings.ToLower(err.Error()), "not found"):
		status, code, errorCode = 404, 404, "not_found"
	case strings.Contains(strings.ToLower(err.Error()), "unavailable"):
		status, code, errorCode = 503, 503, "service_unavailable"
	case strings.Contains(strings.ToLower(err.Error()), "required") || strings.Contains(strings.ToLower(err.Error()), "invalid"):
		status, code, errorCode = 422, 422, "alarm_validation_failed"
	}
	r.Response.WriteStatus(status)
	r.Response.WriteJson(g.Map{"code": code, "error_code": errorCode, "message": err.Error()})
}
