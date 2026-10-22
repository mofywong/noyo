package core

import (
	"encoding/json"
	"sort"

	"noyo/core/screen"
)

// Presentation is recoverable once intent and immutable metrics are validated.
// Never use this path for an existing canvas: an edit must preserve its content.
// This composer reads no model-authored bindings, values, operations or geometry.
func screenAssistantComposeApproved(doc screen.Envelope, selected []screenAssistantMetricCandidate, zh bool) (string, bool) {
	components, _ := doc.Content["componentList"].([]interface{})
	if len(components) > 0 || len(selected) == 0 {
		return "", false
	}
	title, summary := "IOC monitoring dashboard", "Organized the approved monitoring metrics by priority and data shape."
	if zh {
		title, summary = "IOC 运行监控大屏", "已根据确认的监控指标、业务优先级和数据类型自动编排大屏。"
	}
	sections := []interface{}{map[string]interface{}{"role": "header", "title": title}}
	ordered := append([]screenAssistantMetricCandidate(nil), selected...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Metric.Priority < ordered[j].Metric.Priority })
	var kpi map[string]interface{}
	for _, c := range ordered {
		if !c.Supported {
			continue
		}
		role := map[string]string{"scalar": "kpi", "series": "trend", "distribution": "distribution", "events": "eventTable", "alarms": "alarmTable", "map": "map"}[c.Shape]
		if role == "" {
			return "", false
		}
		section := map[string]interface{}{"role": role, "title": c.Label, "priority": c.Metric.Priority, "question": c.Metric.Question}
		if role == "kpi" {
			items, _ := kpi["items"].([]interface{})
			if kpi == nil || len(items) >= 4 || kpi["priority"] != c.Metric.Priority {
				heading := "Key monitoring metrics"
				if zh {
					heading = "重点监控指标"
				}
				kpi = map[string]interface{}{"role": "kpi", "title": heading, "priority": c.Metric.Priority, "items": []interface{}{}}
				sections = append(sections, kpi)
				items = nil
			}
			form := "count"
			if c.Unit == "%" {
				form = "percent"
			}
			kpi["items"] = append(items, map[string]interface{}{"metricId": c.ID, "label": c.Label, "form": form, "precision": 2})
			continue
		}
		kpi = nil
		if role == "trend" || role == "distribution" {
			viz := "line"
			if role == "distribution" {
				viz = "bar"
			}
			section["viz"] = viz
			section["items"] = []interface{}{map[string]interface{}{"metricId": c.ID, "label": c.Label}}
		} else {
			section["metricId"] = c.ID
		}
		sections = append(sections, section)
	}
	encoded, err := json.Marshal(map[string]interface{}{"summary": summary, "composition": "overview", "sections": sections})
	return string(encoded), err == nil
}
