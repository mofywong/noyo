package core

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coreassistant "noyo/core/assistant"
	"noyo/core/store"

	"github.com/gogf/gf/v2/net/ghttp"
)

type aiBrainEvidenceRecorder interface {
	RecordAssistantEvidence(scope coreassistant.Scope, payload map[string]any) (any, error)
}

type aiBrainSystemEvidenceRecorder interface {
	RecordSystemEvidence(scope coreassistant.Scope, payload map[string]any) (any, error)
}

func (s *Server) recordDeviceControlEvidence(r *ghttp.Request, device store.Device, pointID string, pointConfig map[string]interface{}, value interface{}) {
	if s == nil || s.Manager == nil {
		return
	}
	plugin := s.Manager.GetPlugin("ai_brain")
	recorder, ok := plugin.(aiBrainEvidenceRecorder)
	if !ok {
		return
	}
	authCtx := requestAuthContext(r)
	userID := uint(0)
	if authCtx != nil && authCtx.SubjectType != "app" {
		userID = authCtx.UserID
	}
	payload := buildDeviceControlEvidencePayload(device, pointID, pointConfig, value)
	_, _ = recorder.RecordAssistantEvidence(coreassistant.Scope{
		TenantID:  device.TenantID,
		ProjectID: device.ProjectID,
		UserID:    userID,
	}, map[string]any{
		"source_type": "device_control",
		"source_id":   fmt.Sprintf("device_write:%s:%s:%d", device.Code, pointID, time.Now().UnixNano()),
		"entity_type": "device",
		"entity_id":   device.Code,
		"summary":     fmt.Sprintf("User set %s on %s.", payload["property_name"], payload["device_name"]),
		"payload":     payload,
		"weight":      1.0,
	})
}

func (s *Server) recordRuleExecutionFailureEvidence(log *store.RuleExecLog) {
	if s == nil || s.Manager == nil {
		return
	}
	plugin := s.Manager.GetPlugin("ai_brain")
	recorder, ok := plugin.(aiBrainSystemEvidenceRecorder)
	if !ok {
		return
	}
	recordRuleExecutionFailureEvidence(recorder, log)
}

func (s *Server) recordArchivedWorkOrderMemoryCandidate(order *store.WorkOrder) {
	if s == nil || s.Manager == nil {
		return
	}
	plugin := s.Manager.GetPlugin("ai_brain")
	recorder, ok := plugin.(aiBrainSystemEvidenceRecorder)
	if !ok {
		return
	}
	recordArchivedWorkOrderMemoryCandidate(recorder, order)
}

func recordArchivedWorkOrderMemoryCandidate(recorder aiBrainSystemEvidenceRecorder, order *store.WorkOrder) {
	if recorder == nil || order == nil || order.Status != "closed" || order.TenantID == 0 || order.ProjectID == 0 {
		return
	}
	var resolution struct {
		ActualProblem   string `json:"actual_problem"`
		RootCause       string `json:"root_cause"`
		HandlingProcess string `json:"handling_process"`
		HandlingResult  string `json:"handling_result"`
	}
	if err := json.Unmarshal([]byte(order.ResolutionJSON), &resolution); err != nil {
		return
	}
	if strings.TrimSpace(resolution.RootCause) == "" || strings.TrimSpace(resolution.HandlingProcess) == "" || strings.TrimSpace(resolution.HandlingResult) == "" {
		return
	}
	publicID := firstNonEmptyString(order.PublicID, order.Code)
	if publicID == "" {
		return
	}
	sourceEventID := fmt.Sprintf("work_order:%s:archived:v%d", publicID, order.Version)
	summary := strings.TrimSpace(resolution.HandlingResult)
	_, _ = recorder.RecordSystemEvidence(coreassistant.Scope{
		TenantID:  order.TenantID,
		ProjectID: order.ProjectID,
	}, map[string]any{
		"source_type":     "work_order_resolution",
		"source_event_id": sourceEventID,
		"entity_type":     "work_order",
		"entity_id":       publicID,
		"event_time":      order.UpdatedAt,
		"summary":         summary,
		"payload": map[string]any{
			"work_order_id":    order.ID,
			"public_id":        order.PublicID,
			"code":             order.Code,
			"title":            order.Title,
			"source_type":      order.SourceType,
			"source_id":        order.SourceID,
			"actual_problem":   resolution.ActualProblem,
			"root_cause":       resolution.RootCause,
			"handling_process": resolution.HandlingProcess,
			"handling_result":  resolution.HandlingResult,
		},
		"weight":           1.0,
		"create_candidate": true,
		"candidate": map[string]any{
			"scope_type":  "project",
			"category":    "work_order_resolution",
			"title":       firstNonEmptyString(order.Title, order.Code),
			"summary":     summary,
			"entity_type": "work_order",
			"entity_id":   publicID,
			"source_type": "work_order_resolution",
			"source_id":   sourceEventID,
			"confidence":  0.8,
			"details": map[string]any{
				"actual_problem":   resolution.ActualProblem,
				"root_cause":       resolution.RootCause,
				"handling_process": resolution.HandlingProcess,
				"handling_result":  resolution.HandlingResult,
				"work_order_code":  order.Code,
				"work_order_title": order.Title,
			},
		},
	})
}

func recordRuleExecutionFailureEvidence(recorder aiBrainSystemEvidenceRecorder, log *store.RuleExecLog) {
	if recorder == nil || log == nil || log.Success || log.TenantID == 0 || log.ProjectID == 0 {
		return
	}
	executedAt := time.Now()
	if log.ExecutedAt > 0 {
		executedAt = time.UnixMilli(log.ExecutedAt)
	}
	ruleLabel := firstNonEmptyString(log.RuleName, log.RuleCode)
	_, _ = recorder.RecordSystemEvidence(coreassistant.Scope{
		TenantID:  log.TenantID,
		ProjectID: log.ProjectID,
	}, map[string]any{
		"source_type":     "rule_execution_failure",
		"source_event_id": fmt.Sprintf("rule_exec:%d", log.ID),
		"entity_type":     "rule",
		"entity_id":       log.RuleCode,
		"event_time":      executedAt,
		"summary":         fmt.Sprintf("规则 %s 执行失败：%s", ruleLabel, firstNonEmptyString(log.ErrorMessage, "未知错误")),
		"payload": map[string]any{
			"rule_id":          log.RuleID,
			"rule_code":        log.RuleCode,
			"rule_name":        log.RuleName,
			"rule_version":     log.RuleVersion,
			"scope":            log.Scope,
			"gateway_sn":       log.GatewaySN,
			"trigger_id":       log.TriggerID,
			"trigger_type":     log.TriggerType,
			"trigger_detail":   log.TriggerDetail,
			"trace_id":         log.TraceID,
			"chain_depth":      log.ChainDepth,
			"condition_result": log.ConditionResult,
			"condition_detail": log.ConditionDetail,
			"action_results":   log.ActionResults,
			"error":            log.ErrorMessage,
			"duration_ms":      log.DurationMs,
			"executed_as":      log.ExecutedAs,
		},
		"weight": 1.0,
	})
}

func buildDeviceControlEvidencePayload(device store.Device, pointID string, pointConfig map[string]interface{}, value interface{}) map[string]any {
	deviceName := firstNonEmptyString(device.Name, device.Code)
	propertyName := firstNonEmptyString(
		stringMapValue(pointConfig, "title"),
		stringMapValue(pointConfig, "label"),
		stringMapValue(pointConfig, "display_name"),
		stringMapValue(pointConfig, "name"),
		pointID,
	)
	return map[string]any{
		"action":        "set_property",
		"device_code":   device.Code,
		"device_name":   deviceName,
		"property_key":  pointID,
		"property_name": propertyName,
		"value":         value,
	}
}

func stringMapValue(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	if value, ok := values[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
