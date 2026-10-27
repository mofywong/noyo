package core

import (
	"context"
	"sort"
	"time"

	"noyo/core/store"
)

const screenAssistantValueContract = `Generate for the declared thing-model capabilities and user goals across any industry. Do not assume a fire-safety scene. Profile opportunities are possible questions, not proof that history, coordinates, thresholds or events exist. Field role/counter labels with name_heuristic provenance are suggestions, not verified operational meaning. Respect physical quantities, exact units and field aggregations. Never sum temperatures, pressures, percentages or numeric state codes. A cumulative reading is not interval consumption; do not invent deltas, reset handling, health scores, utilization, thresholds or causal diagnoses.
In the metrics stage include questionId, question, reason and priority (0..3, higher is more important) for each metric. Choose nonredundant questions about current conditions, meaningful changes, device states, connectivity, events or location according to declared capabilities and the application scenario. Use deviceSummary for actual scoped connectivity statistics: no keys, explicit deviceCodes, aggregate total|online|offline|unknown|onlineRate; onlineRate is percent of all scoped devices, with unknown states retained in the denominator. Numeric aggregate.count counts reporting samples, not devices or events. Keep important unknowns visible. Choose the scope from readable project devices, never just the first device.
In presentation use only the final metricPlan and selected trials. metricEvidence explains current quality, scope, status/reason, distinct historical points and observed span. An unresolved scope has deviceCount=-1, not zero. empty_scope means no eligible devices, awaiting_observations means a resolved scope has not supplied usable observations, and query_error means execution failed; never describe these as a normal zero. Scope alternatives are not permission to expand a binding. This evidence only determines ready/waiting/stale display states. Preserve all selected schema-supported views and bindings even before the device first reports, including history and location views. Do not replace a meaningful trend with a latest-value widget because history is currently empty. Keep valid zeroes and stable series meaningful. Do not impose KPI/panel quotas or make empty charts to fill space. A current value and its schema-supported trend can answer distinct questions; repeated views of the same current value add no information.
Use optional top-level composition overview|equipment|compact: overview for multiple operational questions with a dominant main condition view, equipment for comparable assets/states, compact for sparse profiles. Each semantic section should state question and priority (0..3). Omit section.span unless the user explicitly requests a manual layout, so the host can apply the chosen composition. Main questions deserve visual emphasis; keep units, time windows, coverage and important unknowns understandable. deviceSummary counts use default/count/text forms without a unit; only onlineRate may use percent/progress and its value is always 0..100, with optional unit %. The host compiles geometry, decorative frames and themes. All generated prose follows locale. Review remains JSON-only and must preserve all frozen data bindings; never claim to have inspected a rendered screenshot.`

type screenAssistantSeriesEvidence struct {
	Key     string   `json:"key"`
	Points  int      `json:"points"`
	FirstAt int64    `json:"firstAt"`
	LastAt  int64    `json:"lastAt"`
	Min     *float64 `json:"min,omitempty"`
	Max     *float64 `json:"max,omitempty"`
}

type screenAssistantHistoryEvidence struct {
	Eligible      bool                            `json:"eligibleForTrend"`
	Reason        string                          `json:"reason"`
	WindowMs      int64                           `json:"windowMs"`
	MinimumSpanMs int64                           `json:"minimumSpanMs"`
	Series        []screenAssistantSeriesEvidence `json:"series"`
}

func screenAssistantEvidenceStatus(quality, scopeState string) (string, string) {
	switch quality {
	case "good":
		return "good", "observations_available"
	case "stale":
		return "stale", "observations_expired"
	case "error", "query_error":
		return "query_error", "query_execution_failed"
	case "not_evaluated":
		return "not_evaluated", "preview_not_executed"
	case "no_data":
		if scopeState == "empty" {
			return "no_data", "empty_scope"
		}
		if scopeState == "unresolved" {
			return "no_data", "scope_unresolved"
		}
		return "no_data", "awaiting_observations"
	}
	return quality, "quality_unclassified"
}

// Only claim an exact count for scopes whose membership can be established.
// Compound/parameterized scopes stay unresolved instead of becoming zero.
func screenAssistantEvidenceScope(ctx context.Context, row *store.IOCScreen, metric screenAssistantMetric, counts map[string]int64) (screenAssistantMetricScope, error) {
	scope := screenAssistantMetricScope{State: "resolved", DeviceCount: int64(len(metric.DeviceCodes)), SourceRef: metric.ID}
	if metric.Binding != nil && metric.Binding.Query != nil {
		scope.State, scope.DeviceCount = "unresolved", -1
		sources := metric.Binding.Query.Sources
		if len(sources) != 1 {
			return scope, nil
		}
		selector := sources[0].Selector
		if len(selector.TagIDs) != 0 || selector.ProductParameter != "" {
			return scope, nil
		}
		if selector.ProductCode != "" {
			scope.ProductCode = selector.ProductCode
			if row.TenantID == 0 || row.ProjectID == 0 || len(selector.DeviceCodes) != 0 {
				return scope, nil
			}
			count, ok := counts[selector.ProductCode]
			if !ok {
				if err := store.DB.WithContext(ctx).Model(&store.Device{}).Where("tenant_id = ? AND project_id = ? AND product_code = ?", row.TenantID, row.ProjectID, selector.ProductCode).Count(&count).Error; err != nil {
					return scope, err
				}
				counts[selector.ProductCode] = count
			}
			scope.State, scope.DeviceCount = "resolved", count
		} else if len(selector.DeviceCodes) > 0 {
			unique := map[string]bool{}
			for _, code := range selector.DeviceCodes {
				unique[code] = true
			}
			scope.State, scope.DeviceCount = "resolved", int64(len(unique))
		}
	}
	if scope.DeviceCount == 0 {
		scope.State = "empty"
	}
	return scope, nil
}

// This is a declared presentation policy for the bounded preview, not a claim
// about raw sample completeness. Unknown reporting cadence cannot yield a valid
// missing-sample percentage. Flat observations remain eligible.
func screenAssistantAssessHistory(metric screenAssistantMetric, rows []map[string]interface{}) screenAssistantHistoryEvidence {
	result := screenAssistantHistoryEvidence{Eligible: len(metric.Keys) > 0, Reason: "sufficient_preview", WindowMs: metric.WindowMs, MinimumSpanMs: metric.WindowMs / 4, Series: []screenAssistantSeriesEvidence{}}
	for _, key := range metric.Keys {
		series := screenAssistantSeriesEvidence{Key: key}
		seen := map[int64]bool{}
		for _, row := range rows {
			stamp, ok := screenNumber(row["ts"])
			value, valid := screenNumber(row[key])
			if !ok || !valid || stamp <= 0 || seen[int64(stamp)] {
				continue
			}
			ts := int64(stamp)
			seen[ts] = true
			if series.Points == 0 || ts < series.FirstAt {
				series.FirstAt = ts
			}
			if series.Points == 0 || ts > series.LastAt {
				series.LastAt = ts
			}
			if series.Min == nil || value < *series.Min {
				v := value
				series.Min = &v
			}
			if series.Max == nil || value > *series.Max {
				v := value
				series.Max = &v
			}
			series.Points++
		}
		if series.Points < 3 {
			result.Eligible, result.Reason = false, "insufficient_distinct_points"
		} else if series.LastAt-series.FirstAt < result.MinimumSpanMs {
			result.Eligible = false
			if result.Reason == "sufficient_preview" {
				result.Reason = "short_observed_span"
			}
		}
		result.Series = append(result.Series, series)
	}
	return result
}

func screenAssistantHistoryRows(value interface{}) []map[string]interface{} {
	if rows, ok := value.([]map[string]interface{}); ok {
		return rows
	}
	rows := []map[string]interface{}{}
	if items, ok := value.([]interface{}); ok {
		for _, item := range items {
			if row, valid := item.(map[string]interface{}); valid {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func screenAssistantMetricQuestion(metric screenAssistantMetric, zh bool) screenAssistantMetric {
	if metric.QuestionID == "" {
		switch metric.Kind {
		case "deviceSummary", "deviceStatus":
			metric.QuestionID = "coverage"
		case "history":
			metric.QuestionID = "change"
		case "alarmList":
			metric.QuestionID = "events"
		case "location":
			metric.QuestionID = "location"
		default:
			metric.QuestionID = "conditions"
		}
	}
	if metric.Question == "" {
		questions := map[string][2]string{
			"coverage":   {"哪些设备在线、离线或状态未知？", "Which devices are online, offline or unknown?"},
			"change":     {"有效观测中发生了什么变化？", "What changed in valid observations?"},
			"events":     {"当前有哪些未关闭告警需要关注？", "Which open alarms need attention?"},
			"location":   {"这些设备位于哪里？", "Where are these devices?"},
			"conditions": {"当前设备工况怎样？", "What are the current equipment conditions?"},
		}
		pair, ok := questions[metric.QuestionID]
		if !ok {
			pair = questions["conditions"]
		}
		metric.Question = pair[1]
		if zh {
			metric.Question = pair[0]
		}
	}
	return metric
}

// Finalise candidates before presentation locks all selected bindings. Current
// observations qualify the display state, not the declared device capability.
func (s *Server) screenAssistantSelectMetrics(ctx context.Context, row *store.IOCScreen, candidates screenAssistantMetrics, trials map[string]interface{}, zh bool) (screenAssistantMetrics, map[string]interface{}, map[string]interface{}, error) {
	selected := screenAssistantMetrics{Metrics: []screenAssistantMetric{}, Limitations: append([]string{}, candidates.Limitations...)}
	evidence, seen := map[string]interface{}{}, map[string]bool{}
	ordered := append([]screenAssistantMetric{}, candidates.Metrics...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Priority > ordered[j].Priority })
	now := time.Now().UnixMilli()
	waiting, references := 0, 0
	finalTrials := map[string]interface{}{}
	scopeCounts := map[string]int64{}
	add := func(metric screenAssistantMetric, trial screenRuntimeSample, scope screenAssistantMetricScope) bool {
		signature := screenAssistantBindingSignature(metric.binding())
		if seen[signature] {
			return true
		}
		if len(selected.Metrics) >= 24 || references+len(metric.DeviceCodes) > 500 {
			return false
		}
		metric = screenAssistantMetricQuestion(metric, zh)
		selected.Metrics = append(selected.Metrics, metric)
		seen[signature] = true
		references += len(metric.DeviceCodes)
		detail := map[string]interface{}{"questionId": metric.QuestionID, "question": metric.Question, "selectionReason": metric.Reason, "sourceKind": metric.Kind, "eligibleDevices": len(metric.DeviceCodes), "asOf": now, "quality": trial.Quality, "receivedAt": trial.ReceivedAt, "selected": true}
		detail["eligibleDevices"], detail["scope"] = scope.DeviceCount, scope
		detail["status"], detail["reason"] = screenAssistantEvidenceStatus(trial.Quality, scope.State)
		if metric.Kind == "history" && metric.Binding == nil {
			detail["history"] = screenAssistantAssessHistory(metric, screenAssistantHistoryRows(trial.Value))
		}
		if metric.Kind == "deviceStatus" || metric.Kind == "deviceSummary" {
			known := 0
			if s.DeviceManager != nil {
				for _, code := range metric.DeviceCodes {
					if s.DeviceManager.GetLatestSnapshot(code).StatusReceivedAt != 0 {
						known++
					}
				}
			}
			detail["knownDevices"], detail["unknownDevices"] = known, len(metric.DeviceCodes)-known
		} else if metric.Kind == "point" || metric.Kind == "aggregate" {
			reported, fresh := 0, 0
			if s.DeviceManager != nil {
				for _, code := range metric.DeviceCodes {
					snapshot := s.DeviceManager.GetLatestSnapshot(code)
					point := snapshot.Samples[metric.Keys[0]]
					if point.ReceivedAt != 0 && point.Value != nil {
						reported++
						if screenSampleQuality(snapshot, point, metric.binding(), now) == "good" {
							fresh++
						}
					}
				}
			}
			detail["reportedDevices"], detail["freshDevices"] = reported, fresh
		}
		evidence[metric.ID] = detail
		finalTrials[metric.ID] = trial
		return true
	}
	for _, metric := range ordered {
		if err := ctx.Err(); err != nil {
			return selected, nil, nil, err
		}
		trial, ok := trials[metric.ID].(screenRuntimeSample)
		if !ok {
			return selected, nil, nil, errScreenAssistantInvalid
		}
		if metric.Binding == nil && metric.Kind == "history" && (trial.Quality != "good" || !screenAssistantAssessHistory(metric, screenAssistantHistoryRows(trial.Value)).Eligible) {
			// Sample coverage qualifies the empty state, never the device's
			// declared capability. Keep the history binding ready for first data.
			if trial.Quality == "good" {
				trial.Quality = "no_data"
			}
		}
		if trial.Quality != "good" && trial.Quality != "not_evaluated" {
			waiting++
		}
		scope, err := screenAssistantEvidenceScope(ctx, row, metric, scopeCounts)
		if err != nil {
			return selected, nil, nil, err
		}
		if !add(metric, trial, scope) {
			return selected, nil, nil, errScreenAssistantInvalid
		}
	}
	if waiting > 0 {
		note := "Schema-defined views and bindings are retained; unavailable observations display a waiting-for-data state."
		if zh {
			note = "已按物模型保留展示与数据绑定；尚未上报的数据以等待数据状态展示。"
		}
		if len(selected.Limitations) >= 16 {
			selected.Limitations = selected.Limitations[:15]
		}
		selected.Limitations = append(selected.Limitations, note)
	}
	return selected, finalTrials, evidence, nil
}
