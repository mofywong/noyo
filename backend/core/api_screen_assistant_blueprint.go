package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"noyo/core/dataquery"
	"noyo/core/screen"
	"noyo/core/store"
)

// Candidates are a server artifact, never a model-authored query. The model
// receives a projection without bindings or expanded scope membership.
type screenAssistantMetricCandidate struct {
	ID        string                `json:"id"`
	Group     string                `json:"group"`
	Label     string                `json:"label"`
	Kind      string                `json:"kind"`
	Key       string                `json:"key,omitempty"`
	Unit      string                `json:"unit,omitempty"`
	Aggregate string                `json:"aggregate,omitempty"`
	WindowMs  int64                 `json:"windowMs,omitempty"`
	Shape     string                `json:"shape"`
	Supported bool                  `json:"supported"`
	Field     *screen.ProfileField  `json:"field,omitempty"`
	Events    []screen.ProfileEvent `json:"events,omitempty"`
	Metric    screenAssistantMetric `json:"metric"`
}

type screenAssistantMetricChoice struct {
	ID       string `json:"id"`
	Question string `json:"question,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

type screenAssistantMetricSelection struct {
	Selected    []screenAssistantMetricChoice `json:"selected"`
	Limitations []string                      `json:"limitations,omitempty"`
}

const screenAssistantCandidatePrompt = `WORKFLOW metrics. Select a useful nonredundant set from metricCandidates to answer intent and the user's operational questions. Output ONLY {"selected":[{"id":"exact candidate ID","question":"decision this supports","reason":"why useful","priority":0}],"limitations":[]}. Choose at most 12, normally fewer for sparse capabilities. Priority is 0..3. Candidates are the complete executable contract; do not supply queries, device IDs, fields or other metric definitions. Do not combine incompatible quantities or imply sums are time consumption. Supported=false means a schema-supported view cannot currently execute within platform capacity; keep an important requested view and explain its limitation instead of substituting fake data. No observations is different from no capability. Preserve valid schema-supported trends before first reporting. Do not require the user to choose widgets, dimensions or metrics. Use the requested locale.`

const screenAssistantBlueprintPrompt = `WORKFLOW generating. Produce an AssistantPlan using the validated intent and selectedMetrics. On a new or redesigned screen use semantic sections and no pixel geometry. Each KPI/trend/distribution item MUST contain metricId pointing to a selected metric, e.g. {"label":"...","metricId":"m_...","form":"count"}. Event/alarm/map sections MUST carry metricId at section level. In sections and items do not output binding, device, devices, key, keys, kind, aggregate, unit or window: the host compiles these from the approved candidate. Use shape=scalar as KPI, series as trend, distribution as distribution, events as eventTable, alarms as alarmTable, map as map, states as statusWall. Unsupported selected views can be described in a subtitle; do not invent values. Select a compact, coherent composition with one main operational question. All selected executable metrics must be represented exactly once unless current value and history answer different questions. The host selects compatible widgets and exact readable rectangles. On a local edit use minimal operations referencing existing IDs, omit unchanged bindings. To explicitly change a binding put metricId on the operation; never supply raw binding JSON. Preserve manual locks and unrelated content. Use the requested locale and design tokens. Output only JSON, with a concise summary and no technical implementation details.`

func screenAssistantCandidateID(group, kind, key, aggregate string, window int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d", group, kind, key, aggregate, window)))
	return "m_" + hex.EncodeToString(h[:8])
}

func screenAssistantCandidates(context *screenAssistantContext, locale, prompt string, allowAlarms bool) []screenAssistantMetricCandidate {
	products := map[string]screen.ProfileProduct{}
	for _, p := range context.Profile.Products {
		products[p.Code] = p
	}
	result := []screenAssistantMetricCandidate{}
	add := func(group screenAssistantContextGroup, kind, key, aggregate, unit, label, shape string, window int64) {
		id := screenAssistantCandidateID(group.ID, kind, key, aggregate, window)
		m := screenAssistantMetric{ID: id, Kind: kind, Label: label, Aggregate: aggregate, WindowMs: window}
		if key != "" {
			m.Keys = []string{key}
		}
		supported := true
		if group.Dynamic || kind == "distribution" {
			b, ok := screenAssistantGroupQuery(group, kind, key, aggregate, window)
			supported = ok
			if ok {
				b.Unit = unit
				if kind == "eventList" {
					title := "event_zh"
					if locale != "zh" {
						title = "event_en"
					}
					b.Mapping.Columns = []string{"ts", "device_name", title, "event_type", "parameters"}
				}
				m.Binding = &b
			}
		} else {
			m.DeviceCodes = append([]string{}, group.DeviceCodes...)
			supported = len(m.DeviceCodes) > 0 && len(m.DeviceCodes) <= 200
		}
		candidate := screenAssistantMetricCandidate{ID: id, Group: group.ID, Label: label, Kind: kind, Key: key, Unit: unit, Aggregate: aggregate, WindowMs: window, Shape: shape, Supported: supported, Metric: m}
		for _, field := range products[group.ProductCode].Fields {
			if field.Key == key {
				definition := field
				candidate.Field = &definition
				break
			}
		}
		if kind == "eventList" {
			candidate.Events = products[group.ProductCode].Events
		}
		result = append(result, candidate)
	}
	for _, group := range context.Groups {
		p := products[group.ProductCode]
		prefix := group.Name
		if prefix == "" {
			prefix = group.ProductCode
		}
		if locale == "zh" {
			add(group, "deviceSummary", "", "total", "", prefix+"设备总数", "scalar", 0)
		} else {
			add(group, "deviceSummary", "", "total", "", prefix+" devices", "scalar", 0)
		}
		for _, stat := range []string{"online", "offline", "unknown", "onlineRate"} {
			label := map[string]string{"online": "Online", "offline": "Offline", "unknown": "Unknown connectivity", "onlineRate": "Online rate"}[stat]
			if locale == "zh" {
				label = map[string]string{"online": "在线设备", "offline": "离线设备", "unknown": "连接状态未知", "onlineRate": "在线率"}[stat]
			}
			unit := ""
			if stat == "onlineRate" {
				unit = "%"
			}
			add(group, "deviceSummary", "", stat, unit, prefix+" · "+label, "scalar", 0)
		}
		for _, f := range p.Fields {
			d := screen.DescribeField(f)
			if d.Role == screen.RoleIgnore || screenAssistantPrivateFieldKey(f.Key) {
				continue
			}
			label := f.En
			if locale == "zh" {
				label = f.Zh
			}
			if label == "" {
				label = f.Key
			}
			label = prefix + " · " + label
			if len(f.Values) > 0 || d.Role == screen.RoleStatus || f.Type == "bool" || f.Type == "boolean" {
				add(group, "distribution", f.Key, "count", "", label, "distribution", 0)
			}
			if len(group.DeviceCodes) == 1 && !group.Dynamic {
				add(group, "point", f.Key, "", f.Unit, label, "scalar", 0)
			}
			// Enumeration state codes are never numeric measures. Only functions
			// explicitly permitted by the field semantic descriptor are offered.
			for _, aggregate := range d.Aggregations {
				if aggregate == "count" {
					continue
				}
				add(group, "aggregate", f.Key, aggregate, f.Unit, label+" ("+aggregate+")", "scalar", 0)
			}
			if screenAssistantContains(d.Aggregations, "avg") {
				add(group, "history", f.Key, "avg", f.Unit, label, "series", 86400000)
			}
		}
		if len(p.Events) > 0 {
			add(group, "eventList", "", "", "", prefix, "events", 86400000)
		}
		if allowAlarms {
			add(group, "alarmList", "", "", "", prefix, "alarms", 86400000)
		}
	}
	// Relevance ranking is deterministic and never changes query semantics. It
	// bounds the catalog, rather than dumping every field of every product.
	text := strings.ToLower(prompt)
	score := func(c screenAssistantMetricCandidate) int {
		n := 0
		if c.Key != "" && strings.Contains(text, strings.ToLower(c.Key)) {
			n += 20
		}
		for _, word := range strings.Fields(strings.ToLower(c.Label)) {
			if len([]rune(word)) > 1 && strings.Contains(text, word) {
				n += 10
			}
		}
		if c.Key != "" || c.Kind == "eventList" || c.Kind == "alarmList" {
			// Schema-defined business measurements take precedence over generic
			// connectivity counts when the user asks for a useful operations view.
			n += 2
		}
		if c.Supported {
			n++
		}
		return n
	}
	sort.SliceStable(result, func(i, j int) bool { return score(result[i]) > score(result[j]) })
	return result
}

// Bound context by taking turns across products and distinct schema signals.
// A long first product or five connectivity variants per product must not hide
// every later product's measurements. Alternate aggregations remain available
// after the distinct signals have been offered.
func screenAssistantLimitCandidates(candidates []screenAssistantMetricCandidate, limit int) []screenAssistantMetricCandidate {
	if len(candidates) <= limit {
		return candidates
	}
	groups := []string{}
	buckets := map[string][]screenAssistantMetricCandidate{}
	deferred := map[string][]screenAssistantMetricCandidate{}
	seen := map[string]bool{}
	for _, c := range candidates {
		if _, ok := buckets[c.Group]; !ok {
			groups = append(groups, c.Group)
			buckets[c.Group] = nil
		}
		signal := c.Group + "|" + c.Key
		if c.Key == "" {
			signal += "|" + c.Kind
		}
		if seen[signal] {
			deferred[c.Group] = append(deferred[c.Group], c)
		} else {
			buckets[c.Group] = append(buckets[c.Group], c)
			seen[signal] = true
		}
	}
	for _, group := range groups {
		buckets[group] = append(buckets[group], deferred[group]...)
	}
	result := make([]screenAssistantMetricCandidate, 0, limit)
	for round := 0; len(result) < limit; round++ {
		added := false
		for _, group := range groups {
			if round < len(buckets[group]) {
				result = append(result, buckets[group][round])
				added = true
				if len(result) == limit {
					return result
				}
			}
		}
		if !added {
			break
		}
	}
	return result
}

func screenAssistantReadCandidates(payload map[string]interface{}) []screenAssistantMetricCandidate {
	if v, ok := payload["_candidates"].([]screenAssistantMetricCandidate); ok {
		return v
	}
	encoded, _ := json.Marshal(payload["_candidates"])
	var result []screenAssistantMetricCandidate
	_ = json.Unmarshal(encoded, &result)
	return result
}

func screenAssistantCandidateProjection(candidates []screenAssistantMetricCandidate) []map[string]interface{} {
	result := []map[string]interface{}{}
	for _, c := range candidates {
		projected := map[string]interface{}{"id": c.ID, "group": c.Group, "label": c.Label, "kind": c.Kind, "shape": c.Shape, "supported": c.Supported}
		if c.Unit != "" {
			projected["unit"] = c.Unit
		}
		if c.Aggregate != "" {
			projected["aggregate"] = c.Aggregate
		}
		if c.WindowMs != 0 {
			projected["windowMs"] = c.WindowMs
		}
		if c.Field != nil {
			f := c.Field
			// The localized label already contains the product and property name.
			// Repeating both schema languages per aggregation wastes the bounded
			// metric budget without changing the executable candidate contract.
			field := map[string]interface{}{"key": f.Key, "type": f.Type}
			for key, value := range map[string]string{"role": f.Role, "quantity": f.Quantity, "semanticKind": f.SemanticKind} {
				if value != "" {
					field[key] = value
				}
			}
			projected["field"] = field
			keys := []string{}
			for key := range f.Values {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			values := map[string]string{}
			for _, key := range keys {
				if len(values) == 16 {
					break
				}
				values[key] = screenAssistantText(f.Values[key], 128)
			}
			if len(values) > 0 {
				projected["values"] = values
			}
		}
		if len(c.Events) > 0 {
			events := []map[string]string{}
			for _, event := range c.Events {
				if len(events) == 16 {
					break
				}
				events = append(events, map[string]string{"key": event.Key, "zh": event.Zh, "en": event.En, "type": event.Type})
			}
			projected["events"] = events
		}
		result = append(result, projected)
	}
	return result
}

func screenAssistantResolveSelection(value string, candidates []screenAssistantMetricCandidate) (screenAssistantMetrics, []screenAssistantMetricCandidate, error) {
	var selected screenAssistantMetricSelection
	if screenAssistantDecode(value, &selected) != nil || len(selected.Selected) > 24 || len(selected.Limitations) > 16 {
		return screenAssistantMetrics{}, nil, errScreenAssistantInvalid
	}
	index := map[string]screenAssistantMetricCandidate{}
	for _, c := range candidates {
		index[c.ID] = c
	}
	metrics := screenAssistantMetrics{Metrics: []screenAssistantMetric{}, Limitations: selected.Limitations}
	chosen := []screenAssistantMetricCandidate{}
	seen := map[string]bool{}
	for _, choice := range selected.Selected {
		c, ok := index[choice.ID]
		if !ok || seen[c.ID] || choice.Priority < 0 || choice.Priority > 3 || len(choice.Question) > 500 || len(choice.Reason) > 1000 {
			return metrics, nil, errScreenAssistantInvalid
		}
		seen[c.ID] = true
		c.Metric.QuestionID, c.Metric.Question, c.Metric.Reason, c.Metric.Priority = c.ID, choice.Question, choice.Reason, choice.Priority
		chosen = append(chosen, c)
		if c.Supported {
			metrics.Metrics = append(metrics.Metrics, c.Metric)
		}
	}
	for _, note := range selected.Limitations {
		if len(note) > 1000 {
			return metrics, nil, errScreenAssistantInvalid
		}
	}
	return metrics, chosen, nil
}

// Resolve only the product scopes that survived metric selection. Counts are a
// server-side capacity check, never device context or model-authored selectors.
func screenAssistantResolveProductMetrics(ctx context.Context, row *store.IOCScreen, metrics screenAssistantMetrics, selected []screenAssistantMetricCandidate) (screenAssistantMetrics, []screenAssistantMetricCandidate, error) {
	if row.TenantID == 0 || row.ProjectID == 0 {
		return metrics, selected, nil
	}
	counts := map[string]int64{}
	allowed := map[string]bool{}
	for i := range selected {
		candidate := &selected[i]
		binding := candidate.Metric.Binding
		if candidate.Supported && binding != nil && binding.Query != nil {
			for _, source := range binding.Query.Sources {
				code := source.Selector.ProductCode
				if code == "" {
					continue
				}
				count, ok := counts[code]
				if !ok {
					if err := store.DB.WithContext(ctx).Model(&store.Device{}).Where("tenant_id = ? AND project_id = ? AND product_code = ?", row.TenantID, row.ProjectID, code).Count(&count).Error; err != nil {
						return metrics, selected, err
					}
					counts[code] = count
				}
				limit := int64(screenAssistantContextDeviceBudget)
				if candidate.Kind == "history" || candidate.Kind == "eventList" {
					limit = dataquery.MaxDevices
				}
				if count > limit {
					candidate.Supported = false
				}
			}
		}
		allowed[candidate.ID] = candidate.Supported
	}
	filtered := []screenAssistantMetric{}
	for _, metric := range metrics.Metrics {
		if allowed[metric.ID] {
			filtered = append(filtered, metric)
		}
	}
	metrics.Metrics = filtered
	return metrics, selected, nil
}

// Resolve metric references before strict plan validation. Any raw binding or
// telemetry scope supplied by a model in this mode is rejected, not trusted.
func screenAssistantCompileBlueprint(value string, selected []screenAssistantMetricCandidate) (string, error) {
	var plan map[string]interface{}
	if err := json.Unmarshal([]byte(value), &plan); err != nil {
		return "", errScreenAssistantInvalid
	}
	index := map[string]screenAssistantMetricCandidate{}
	for _, c := range selected {
		index[c.ID] = c
	}
	resolve := func(v map[string]interface{}, section bool, role string) error {
		for _, key := range []string{"binding", "device", "devices", "key", "keys", "kind", "aggregate", "unit", "placeholder"} {
			if _, ok := v[key]; ok {
				return errScreenAssistantInvalid
			}
		}
		id, _ := v["metricId"].(string)
		delete(v, "metricId")
		if id == "" {
			return nil
		}
		c, ok := index[id]
		if !ok {
			return errScreenAssistantInvalid
		}
		expected := map[string]string{"scalar": "kpi", "series": "trend", "distribution": "distribution", "events": "eventTable", "alarms": "alarmTable", "map": "map", "states": "statusWall"}[c.Shape]
		if expected == "" || role != expected || (section && (role == "kpi" || role == "trend" || role == "distribution")) {
			return errScreenAssistantInvalid
		}
		if !c.Supported {
			if section {
				v["role"] = "header"
				v["title"] = c.Label
				v["subtitle"] = "数据能力尚未接通 / Data capability is not connected"
				return nil
			}
			v["placeholder"] = true
			v["label"] = c.Label
			return nil
		}
		b := c.Metric.binding()
		if section {
			v["binding"] = b
			if b.WindowMs > 0 {
				v["window"] = fmt.Sprintf("%dm", b.WindowMs/60000)
			}
			return nil
		}
		v["unit"] = c.Unit
		if b.Kind == "dataset" {
			v["binding"] = b
			return nil
		}
		v["kind"] = c.Kind
		v["aggregate"] = c.Aggregate
		v["devices"] = c.Metric.DeviceCodes
		if len(c.Metric.Keys) == 1 {
			v["key"] = c.Metric.Keys[0]
		} else if len(c.Metric.Keys) > 0 {
			v["keys"] = c.Metric.Keys
		}
		return nil
	}
	if sections, ok := plan["sections"].([]interface{}); ok {
		for _, raw := range sections {
			section, ok := raw.(map[string]interface{})
			if !ok {
				return "", errScreenAssistantInvalid
			}
			role, _ := section["role"].(string)
			// A single chart metric has an unambiguous item representation. Accept
			// this common model shorthand without changing its role or data scope.
			if id, ok := section["metricId"].(string); ok && id != "" && (role == "kpi" || role == "trend" || role == "distribution") {
				items, _ := section["items"].([]interface{})
				if c, exists := index[id]; exists && len(items) == 0 {
					section["items"] = []interface{}{map[string]interface{}{"metricId": id, "label": c.Label}}
					delete(section, "metricId")
				}
			}
			if err := resolve(section, true, role); err != nil {
				return "", err
			}
			if items, ok := section["items"].([]interface{}); ok {
				for _, rawItem := range items {
					item, ok := rawItem.(map[string]interface{})
					if !ok {
						return "", errScreenAssistantInvalid
					}
					id, _ := item["metricId"].(string)
					if c, ok := index[id]; ok && c.WindowMs > 0 {
						section["window"] = fmt.Sprintf("%dm", c.WindowMs/60000)
					}
					if err := resolve(item, false, role); err != nil {
						return "", err
					}
				}
			}
			// A history section's immutable window is derived from its metric.
			if items, ok := section["items"].([]interface{}); ok {
				for _, rawItem := range items {
					item := rawItem.(map[string]interface{})
					if binding, ok := item["binding"].(screen.Binding); ok && binding.WindowMs > 0 {
						section["window"] = fmt.Sprintf("%dm", binding.WindowMs/60000)
					}
				}
			}
		}
	}
	if operations, ok := plan["operations"].([]interface{}); ok {
		for _, raw := range operations {
			op, ok := raw.(map[string]interface{})
			if !ok {
				return "", errScreenAssistantInvalid
			}
			if _, exists := op["binding"]; exists {
				return "", errScreenAssistantInvalid
			}
			id, _ := op["metricId"].(string)
			delete(op, "metricId")
			if id != "" {
				c, ok := index[id]
				if !ok || !c.Supported {
					return "", errScreenAssistantInvalid
				}
				op["binding"] = c.Metric.binding()
			}
		}
	}
	encoded, err := json.Marshal(plan)
	return string(encoded), err
}
