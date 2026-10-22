package core

import (
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"noyo/core/store"
)

// This read-only projection exposes no evidence, handlers or alarm actions.
// Device identity comes from the immutable snapshot written by DeviceManager.
func (s *Server) handleScreenRuntimeAlarms(r *ghttp.Request) {
	row, doc, _, err := screenAuthorizedDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var request struct {
		BindingID string `json:"bindingId"`
	}
	if err = screenReadJSON(r, &request); err != nil {
		screenWriteError(r, err)
		return
	}
	binding, exists := doc.Bindings[request.BindingID]
	if !exists || binding.Kind != "alarmList" {
		screenWriteError(r, screenFail(400, "Unknown alarm binding / 未知告警绑定"))
		return
	}
	result := make([]map[string]interface{}, 0)
	respond := func(quality string, truncated bool) {
		screenWriteData(r, map[string]interface{}{"bindingId": request.BindingID, "value": result, "quality": quality, "truncated": truncated, "receivedAt": time.Now().UnixMilli()})
	}
	if !screenAllowedBindings(r, row, doc)[request.BindingID] {
		respond("access_denied", false)
		return
	}
	devices := map[string]bool{}
	for _, code := range screenBindingDevices(binding) {
		devices[code] = true
	}
	// Bound the database work independently of the output limit. Filtering
	// snapshots in Go keeps the query portable across supported SQL drivers.
	//
	// Only the projected columns are read. `alarm_instances` also carries the
	// policy and cleared-evidence snapshots, which can be tens of kilobytes per
	// row; pulling a thousand of those to show at most a hundred titles was the
	// single largest allocation this endpoint made.
	var alarms []store.AlarmInstance
	err = store.DB.Model(&store.AlarmInstance{}).
		Select("public_id", "title", "severity", "condition_status", "handling_status", "last_occurred_at", "evidence_snapshot").
		Where("tenant_id = ? AND project_id = ? AND handling_status <> ?", row.TenantID, row.ProjectID, AlarmHandlingClosed).
		Order("last_occurred_at DESC").Order("id DESC").Limit(1000).Find(&alarms).Error
	if err != nil {
		screenWriteError(r, err)
		return
	}
	limit := binding.MaxPoints
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	truncated := len(alarms) == 1000
	for _, alarm := range alarms {
		var evidence struct {
			DeviceCode string `json:"device_code"`
			DeviceName string `json:"device_name"`
		}
		if json.Unmarshal([]byte(alarm.EvidenceSnapshot), &evidence) != nil || !devices[evidence.DeviceCode] {
			continue
		}
		if len(result) == limit {
			truncated = true
			break
		}
		// device_name comes from the same immutable snapshot, so an operator
		// reading the wall screen sees the plant name instead of a code.
		result = append(result, map[string]interface{}{"id": alarm.PublicID, "deviceCode": evidence.DeviceCode, "deviceName": evidence.DeviceName, "title": alarm.Title, "severity": alarm.Severity, "conditionStatus": alarm.ConditionStatus, "handlingStatus": alarm.HandlingStatus, "occurredAt": alarm.LastOccurredAt})
	}
	respond("good", truncated)
}
