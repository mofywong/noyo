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
	ID        string                      `json:"id"`
	Group     string                      `json:"group"`
	Label     string                      `json:"label"`
	Kind      string                      `json:"kind"`
	Key       string                      `json:"key,omitempty"`
	Unit      string                      `json:"unit,omitempty"`
	Aggregate string                      `json:"aggregate,omitempty"`
	WindowMs  int64                       `json:"windowMs,omitempty"`
	Shape     string                      `json:"shape"`
	Supported bool                        `json:"supported"`
	Field     *screen.ProfileField        `json:"field,omitempty"`
	Events    []screen.ProfileEvent       `json:"events,omitempty"`
	Metric    screenAssistantMetric       `json:"metric"`
	Scope     *screenAssistantMetricScope `json:"scope,omitempty"`
	// Server-only alternatives are drawn from the same authorized context.
	ScopeAlternatives []string `json:"scopeAlternatives,omitempty"`
}

type screenAssistantMetricScope struct {
	State               string   `json:"state"`
	DeviceCount         int64    `json:"deviceCount"`
	ProductCode         string   `json:"productCode,omitempty"`
	OriginalProductCode string   `json:"originalProductCode,omitempty"`
	Reason              string   `json:"reason,omitempty"`
	SourceRef           string   `json:"sourceRef,omitempty"`
	Alternatives        []string `json:"alternatives,omitempty"`
}

type screenAssistantMetricChoice struct {
	ID       string `json:"id"`
	Question string `json:"question,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Task     string `json:"task,omitempty"`
}

type screenAssistantMetricSelection struct {
	Selected    []screenAssistantMetricChoice `json:"selected"`
	Limitations []string                      `json:"limitations,omitempty"`
}

const screenAssistantCandidatePrompt = `WORKFLOW metrics. Select a useful nonredundant set from metricCandidates to answer intent and the user's operational questions. Output ONLY {"selected":[{"id":"exact candidate ID","question":"decision this supports","reason":"why useful","priority":0,"task":"one of this candidate's tasks"}],"limitations":[]}. Choose at most 12, normally fewer for sparse capabilities. Priority is 0..3, with 3 the primary operational question and 0 background detail. Differentiate priorities when the user's goal has a clear focus; do not automatically promote connectivity above business measurements. Candidates whose scope has no productCode are project-wide totals: they are the only cross-device figures, so an overview screen should include the ones its goal depends on. Select task from each candidate's advertised tasks: current=present value/state, change=time evolution, compare=compare magnitudes, composition=mutually exclusive category counts as parts of a whole, respond=events/alarms, locate=real positions, diagnose=aligned history. A task adds no data capability. metricFields contains shared field evidence referenced by fieldRef. Candidates are the complete executable contract; do not supply queries, device IDs, fields or other metric definitions. Do not combine incompatible quantities or imply sums are time consumption. Supported=false means a schema-supported view cannot currently execute within platform capacity; keep an important requested view and explain its limitation instead of substituting fake data. No observations is different from no capability. Preserve valid schema-supported trends before first reporting. Do not require the user to choose widgets, dimensions or metrics. Use the requested locale.`

const screenAssistantBlueprintPrompt = `WORKFLOW generating. Produce an AssistantPlan using the validated intent and selectedMetrics. On a new or redesigned screen use semantic sections and no pixel geometry. Each KPI/trend/distribution item MUST contain metricId pointing to a selected metric, e.g. {"label":"...","metricId":"m_...","form":"count"}. Event/alarm/map sections MUST carry metricId at section level. In sections and items do not output binding, device, devices, key, keys, kind, aggregate, unit, window, task or categoryCount: the host compiles these and priorities from the approved candidate. Use shape=scalar as KPI, series as trend, distribution as distribution, events as eventTable, alarms as alarmTable, map as map, states as statusWall. Unsupported selected views can be described in a subtitle; do not invent values. Select a compact, coherent composition with one main operational question. SelectedMetrics priorities and questions determine emphasis; do not give every panel equal importance. Omit viz unless the user requested a specific chart; the host matches comparison versus composition to compatible components and available space. A percentage unit alone does not require a ring or progress gauge: prefer a numeric value unless the goal explicitly needs that form. All selected executable metrics must be represented exactly once unless current value and history answer different questions. The host selects compatible widgets and exact readable rectangles. On a local edit use minimal operations referencing existing IDs, omit unchanged bindings. To explicitly change a binding put metricId on the operation; never supply raw binding JSON. Preserve manual locks and unrelated content. Use the requested locale and design tokens. Output only JSON, with a concise summary and no technical implementation details.`

func screenAssistantCandidateID(group, kind, key, aggregate string, window int64) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d", group, kind, key, aggregate, window)))
	return "m_" + hex.EncodeToString(h[:8])
}

// Write-only command properties (DO_/BO_ prefixes) describe an instruction the
// screen could send, not a measurement the device reports. Putting one on a KPI
// or a distribution would display the command echo as if it were state.
func screenAssistantCommandField(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	return strings.HasPrefix(upper, "DO_") || strings.HasPrefix(upper, "BO_")
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
		var definition *screen.ProfileField
		for _, field := range products[group.ProductCode].Fields {
			if field.Key == key {
				copied := field
				definition = &copied
				break
			}
		}
		supported := true
		if group.Dynamic || kind == "distribution" {
			b, ok := screenAssistantGroupQueryField(group, kind, key, aggregate, window, definition, locale)
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
		candidate := screenAssistantMetricCandidate{ID: id, Group: group.ID, Label: label, Kind: kind, Key: key, Unit: unit, Aggregate: aggregate, WindowMs: window, Shape: shape, Supported: supported, Field: definition, Metric: m}
		state := "resolved"
		count := int64(group.Count)
		if group.Dynamic && group.Scope != "project" {
			state, count = "unresolved", -1
		}
		candidate.Scope = &screenAssistantMetricScope{State: state, DeviceCount: count, ProductCode: group.ProductCode, SourceRef: id}
		if group.Dynamic && strings.TrimSpace(group.Name) != "" {
			for _, alternative := range context.Groups {
				if alternative.Dynamic && alternative.ProductCode != group.ProductCode && strings.EqualFold(strings.TrimSpace(group.Name), strings.TrimSpace(alternative.Name)) && screenAssistantCompatibleProducts(products[group.ProductCode], products[alternative.ProductCode]) {
					candidate.ScopeAlternatives = append(candidate.ScopeAlternatives, alternative.ProductCode)
				}
			}
			sort.Strings(candidate.ScopeAlternatives)
		}
		if kind == "eventList" {
			candidate.Events = products[group.ProductCode].Events
		}
		result = append(result, candidate)
	}
	for _, group := range context.Groups {
		p := products[group.ProductCode]
		project := group.Scope == "project"
		prefix := group.Name
		if prefix == "" {
			prefix = group.ProductCode
		}
		// Project labels stand alone: "All devices devices" reads as a mistake.
		projectLabels := map[string]string{"total": "All devices", "online": "Devices online", "offline": "Devices offline", "unknown": "Unknown connectivity", "onlineRate": "Online rate"}
		if locale == "zh" {
			projectLabels = map[string]string{"total": "全部设备总数", "online": "在线设备", "offline": "离线设备", "unknown": "连接状态未知", "onlineRate": "在线率"}
		}
		if project {
			add(group, "deviceSummary", "", "total", "", projectLabels["total"], "scalar", 0)
		} else if locale == "zh" {
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
			if project {
				add(group, "deviceSummary", "", stat, unit, projectLabels[stat], "scalar", 0)
				continue
			}
			add(group, "deviceSummary", "", stat, unit, prefix+" · "+label, "scalar", 0)
		}
		if project {
			// Incident totals span products, which no per-product counter can do.
			window := int64(86400000)
			if locale == "zh" {
				add(group, "eventCount", "", "", "", "全项目事件总数", "scalar", window)
			} else {
				add(group, "eventCount", "", "", "", "Project events", "scalar", window)
			}
			if allowAlarms {
				if locale == "zh" {
					add(group, "alarmCount", "", "", "", "全项目告警总数", "scalar", window)
					add(group, "alarmList", "", "", "", "全项目告警", "alarms", window)
				} else {
					add(group, "alarmCount", "", "", "", "Project alarms", "scalar", window)
					add(group, "alarmList", "", "", "", "Project alarms", "alarms", window)
				}
			}
			if locale == "zh" {
				add(group, "eventList", "", "", "", "全项目事件", "events", window)
			} else {
				add(group, "eventList", "", "", "", "Project events", "events", window)
			}
			continue
		}
		for _, f := range p.Fields {
			d := screen.DescribeField(f)
			if d.Role == screen.RoleIgnore || screenAssistantPrivateFieldKey(f.Key) || screenAssistantCommandField(f.Key) {
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

// Cover product x executable task before filling additional fields or alternate
// aggregations. A current value and its history answer different questions;
// neither may disappear behind many latest-value fields of duplicate products.
func screenAssistantLimitCandidates(candidates []screenAssistantMetricCandidate, limit int) []screenAssistantMetricCandidate {
	if limit <= 0 {
		return []screenAssistantMetricCandidate{}
	}
	if len(candidates) <= limit {
		return candidates
	}
	groups := []string{}
	tasks := []string{}
	knownTasks := map[string]bool{}
	knownGroups := map[string]bool{}
	buckets := map[string][]screenAssistantMetricCandidate{}
	deferred := map[string][]screenAssistantMetricCandidate{}
	seen := map[string]bool{}
	// Project-scope candidates are the only cross-device totals and there are
	// only a handful of them. A large product catalogue must never crowd them out
	// of the bounded catalogue, so they are emitted before the round-robin.
	priority := []screenAssistantMetricCandidate{}
	for _, c := range candidates {
		if c.Scope != nil && c.Scope.ProductCode == "" {
			priority = append(priority, c)
			seen[c.ID] = true
		}
	}
	for _, c := range candidates {
		if seen[c.ID] {
			continue
		}
		if !knownGroups[c.Group] {
			groups = append(groups, c.Group)
			knownGroups[c.Group] = true
		}
		task := screenAssistantCandidateTask(c)
		if !knownTasks[task] {
			tasks = append(tasks, task)
			knownTasks[task] = true
		}
		bucket := c.Group + "|" + task
		signal := bucket + "|" + c.Key
		if c.Key == "" {
			signal += "|" + c.Kind
		}
		if seen[signal] {
			deferred[bucket] = append(deferred[bucket], c)
		} else {
			buckets[bucket] = append(buckets[bucket], c)
			seen[signal] = true
		}
	}
	for bucket := range buckets {
		buckets[bucket] = append(buckets[bucket], deferred[bucket]...)
	}
	result := make([]screenAssistantMetricCandidate, 0, limit)
	for _, c := range priority {
		if len(result) == limit {
			return result
		}
		result = append(result, c)
	}
	// Give each product a first opportunity before spending the remaining
	// budget on more tasks for early products. Rotate the first task so a large
	// catalogue still contains current, historical and response capabilities.
	for groupIndex, group := range groups {
		for offset := range tasks {
			task := tasks[(groupIndex+offset)%len(tasks)]
			key := group + "|" + task
			if len(buckets[key]) == 0 {
				continue
			}
			if len(result) == limit {
				return result
			}
			result = append(result, buckets[key][0])
			buckets[key] = buckets[key][1:]
			break
		}
	}
	for round := 0; len(result) < limit; round++ {
		added := false
		for _, group := range groups {
			for _, task := range tasks {
				bucket := buckets[group+"|"+task]
				if round < len(bucket) {
					result = append(result, bucket[round])
					added = true
					if len(result) == limit {
						return result
					}
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
		if c.Scope != nil {
			projected["scope"] = c.Scope
		}
		projected["tasks"] = screenAssistantCandidateTasksFor(c)
		if c.Metric.QuestionID != "" || c.Metric.Question != "" || c.Metric.Task != "" {
			projected["question"] = c.Metric.Question
			projected["priority"] = c.Metric.Priority
			projected["task"] = screenAssistantCandidateTask(c)
		}
		if count := screenAssistantCategoryCount(c); count > 0 {
			projected["categoryCount"] = count
		}
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
		if choice.Task != "" && !screenAssistantContains(screenAssistantCandidateTasks(c.Shape), choice.Task) {
			return metrics, nil, errScreenAssistantInvalid
		}
		c.Metric.QuestionID, c.Metric.Question, c.Metric.Reason, c.Metric.Priority = c.ID, choice.Question, choice.Reason, choice.Priority
		c.Metric.Task = choice.Task
		// The advertised task is narrowed by the field's own semantics, so a state
		// distribution is never promoted to a part-to-whole composition.
		c.Metric.Task = screenAssistantCandidateTask(c)
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

// Resolve only selected scopes. Empty templates remain valid waiting views.
// Redirect only to a unique deployed equivalent from the original authorized
// context, after checking its current name and complete schema again.
func screenAssistantResolveProductMetrics(ctx context.Context, row *store.IOCScreen, metrics screenAssistantMetrics, selected []screenAssistantMetricCandidate) (screenAssistantMetrics, []screenAssistantMetricCandidate, error) {
	if row.TenantID == 0 || row.ProjectID == 0 {
		return metrics, selected, nil
	}
	counts := map[string]int64{}
	countFor := func(code string) (int64, error) {
		if count, ok := counts[code]; ok {
			return count, nil
		}
		var count int64
		err := store.DB.WithContext(ctx).Model(&store.Device{}).Where("tenant_id = ? AND project_id = ? AND product_code = ?", row.TenantID, row.ProjectID, code).Count(&count).Error
		if err == nil {
			counts[code] = count
		}
		return count, err
	}
	projectTotal := int64(-1)
	projectCount := func() (int64, error) {
		if projectTotal >= 0 {
			return projectTotal, nil
		}
		var count int64
		err := store.DB.WithContext(ctx).Model(&store.Device{}).Where("tenant_id = ? AND project_id = ?", row.TenantID, row.ProjectID).Count(&count).Error
		if err == nil {
			projectTotal = count
		}
		return projectTotal, err
	}
	allowed := map[string]bool{}
	resolved := map[string]screenAssistantMetric{}
	for i := range selected {
		candidate := &selected[i]
		binding := candidate.Metric.Binding
		if candidate.Supported && binding != nil && binding.Query != nil {
			// Keep the immutable candidate catalogue and other selected bindings
			// intact when replacing a selector in this selected view.
			encoded, _ := json.Marshal(binding)
			var copy screen.Binding
			if err := json.Unmarshal(encoded, &copy); err != nil {
				return metrics, selected, err
			}
			binding = &copy
			candidate.Metric.Binding = binding
			for sourceIndex := range binding.Query.Sources {
				source := &binding.Query.Sources[sourceIndex]
				code := source.Selector.ProductCode
				if code == "" {
					if len(source.Selector.DeviceCodes) > 0 || source.Kind == "static" {
						continue
					}
					// Project-wide scope: neither a product nor an explicit device
					// list, so the runtime resolves every device in the project and
					// caps that resolution at MaxDevices.
					count, err := projectCount()
					if err != nil {
						return metrics, selected, err
					}
					scope := &screenAssistantMetricScope{State: "resolved", DeviceCount: count, SourceRef: candidate.ID, Reason: "project_membership"}
					candidate.Scope = scope
					if count == 0 {
						scope.State, scope.Reason = "empty", "empty_project_scope"
						candidate.Supported = false
					} else if count > int64(dataquery.MaxDevices) {
						scope.Reason = "project_exceeds_selector_capacity"
						candidate.Supported = false
					}
					continue
				}
				count, err := countFor(code)
				if err != nil {
					return metrics, selected, err
				}
				scope := &screenAssistantMetricScope{State: "resolved", DeviceCount: count, ProductCode: code, SourceRef: candidate.ID, Reason: "exact_project_membership"}
				candidate.Scope = scope
				if count == 0 {
					scope.State, scope.Reason = "empty", "empty_product_scope"
					if len(candidate.ScopeAlternatives) > 0 {
						var products []store.Product
						codes := append([]string{code}, candidate.ScopeAlternatives...)
						if err := store.DB.WithContext(ctx).Where("(tenant_id = 0 OR (tenant_id = ? AND (project_id = ? OR project_id = 0))) AND code IN ?", row.TenantID, row.ProjectID, codes).Find(&products).Error; err != nil {
							return metrics, selected, err
						}
						var original screen.ProfileProduct
						for _, product := range products {
							if product.Code == code {
								original = screenAssistantProductSchema(product)
							}
						}
						for _, product := range products {
							if product.Code == code || !strings.EqualFold(strings.TrimSpace(original.Name), strings.TrimSpace(product.Name)) || !screenAssistantCompatibleProducts(original, screenAssistantProductSchema(product)) {
								continue
							}
							deployed, err := countFor(product.Code)
							if err != nil {
								return metrics, selected, err
							}
							if deployed > 0 {
								scope.Alternatives = append(scope.Alternatives, product.Code)
							}
						}
						if len(scope.Alternatives) == 1 {
							code = scope.Alternatives[0]
							count = counts[code]
							scope.OriginalProductCode, scope.ProductCode = source.Selector.ProductCode, code
							scope.State, scope.Reason, scope.DeviceCount = "resolved", "unique_deployed_schema_equivalent", count
							source.Selector.ProductCode = code
						} else if len(scope.Alternatives) > 1 {
							scope.Reason = "ambiguous_deployed_alternatives"
						}
					}
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
		resolved[candidate.ID] = candidate.Metric
	}
	filtered := []screenAssistantMetric{}
	for _, metric := range metrics.Metrics {
		if allowed[metric.ID] {
			filtered = append(filtered, resolved[metric.ID])
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
	var resolved []screenAssistantMetricCandidate
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
		resolved = append(resolved, c)
		v["priority"] = c.Metric.Priority
		v["task"] = screenAssistantCandidateTask(c)
		delete(v, "categoryCount")
		if count := screenAssistantCategoryCount(c); !section && count > 0 {
			v["categoryCount"] = count
		}
		if !c.Supported {
			if section {
				v["role"] = "header"
				delete(v, "task")
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
			resolved = nil
			section, ok := raw.(map[string]interface{})
			if !ok {
				return "", errScreenAssistantInvalid
			}
			delete(section, "categoryCount") // Cardinality is compiler-owned, on items.
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
			// Emphasis comes from approved metrics, not a second model's guess.
			// Item priorities also survive collapsed KPI strips on the client.
			if len(resolved) > 0 {
				primary := resolved[0]
				for _, c := range resolved[1:] {
					if c.Metric.Priority > primary.Metric.Priority {
						primary = c
					}
				}
				section["priority"] = primary.Metric.Priority
				if question := screenAssistantDesignQuestion(primary.Metric.Question); question != "" {
					section["question"] = question
				}
				if section["role"] != "header" {
					section["task"] = screenAssistantCandidateTask(primary)
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
