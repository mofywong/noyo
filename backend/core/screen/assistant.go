package screen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Assistant plans are proposals only. The editor builds a candidate document
// and uses the normal validation endpoint before applying one locally.
type AssistantWidget struct {
	Key          string   `json:"key"`
	Zh           string   `json:"zh"`
	En           string   `json:"en"`
	Shape        string   `json:"shape"`
	Kinds        []string `json:"kinds"`
	Target       string   `json:"target"`
	OptionFields []string `json:"optionFields,omitempty"`
}

type AssistantCanvas struct {
	Width  *float64 `json:"width,omitempty"`
	Height *float64 `json:"height,omitempty"`
}

type AssistantOperation struct {
	Type    string                 `json:"type"`
	ID      string                 `json:"id"`
	Key     string                 `json:"key,omitempty"`
	X       *float64               `json:"x,omitempty"`
	Y       *float64               `json:"y,omitempty"`
	W       *float64               `json:"w,omitempty"`
	H       *float64               `json:"h,omitempty"`
	Title   *string                `json:"title,omitempty"`
	Option  map[string]interface{} `json:"option,omitempty"`
	Binding json.RawMessage        `json:"binding,omitempty"`
}

type AssistantSectionItem struct {
	Placeholder bool `json:"placeholder,omitempty"`
	// Binding is filled by the workflow compiler from an approved metric ID.
	// It lets product selectors remain dynamic without expanding device IDs.
	Binding   *Binding `json:"binding,omitempty"`
	Label     string   `json:"label,omitempty"`
	Device    string   `json:"device,omitempty"`
	Devices   []string `json:"devices,omitempty"`
	Key       string   `json:"key,omitempty"`
	Keys      []string `json:"keys,omitempty"`
	Unit      string   `json:"unit,omitempty"`
	Precision *float64 `json:"precision,omitempty"`
	Tone      string   `json:"tone,omitempty"`
	Form      string   `json:"form,omitempty"`
	Viz       string   `json:"viz,omitempty"`
	Aggregate string   `json:"aggregate,omitempty"`
	Kind      string   `json:"kind,omitempty"`
}

// AssistantSection is a semantic request ("show alarm counts, then a 24h level
// trend") with no geometry. The client compiles it to rectangles on a 12-column
// grid, so the model never has to solve layout and cannot emit overlapping or
// off-canvas components.
type AssistantSection struct {
	Binding  *Binding               `json:"binding,omitempty"`
	Role     string                 `json:"role"`
	Question string                 `json:"question,omitempty"`
	Priority int                    `json:"priority,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Subtitle string                 `json:"subtitle,omitempty"`
	Span     *float64               `json:"span,omitempty"`
	Viz      string                 `json:"viz,omitempty"`
	Window   string                 `json:"window,omitempty"`
	Items    []AssistantSectionItem `json:"items,omitempty"`
	Devices  []string               `json:"devices,omitempty"`
}

type AssistantPlan struct {
	ResultStatus  string               `json:"resultStatus,omitempty"`
	Limitations   []string             `json:"limitations,omitempty"`
	Summary       string               `json:"summary"`
	Composition   string               `json:"composition,omitempty"`
	Clarification string               `json:"clarification,omitempty"`
	Canvas        *AssistantCanvas     `json:"canvas,omitempty"`
	Operations    []AssistantOperation `json:"operations,omitempty"`
	Sections      []AssistantSection   `json:"sections,omitempty"`
}

// AssistantCatalog contains only readable device codes and real property keys.
type AssistantCatalog map[string]map[string]bool

var assistantID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)
var assistantField = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}$`)
var assistantColorToken = regexp.MustCompile(`^var\(--[A-Za-z0-9_-]+\)$`)

var assistantWindow = regexp.MustCompile(`^(\d+)\s*(m|h|d)$`)

// Match the editor's history window contract exactly; accepting durations that
// the client silently interprets as 24h would invalidate the server trial.
func AssistantWindowMs(value string) (int64, error) {
	if strings.TrimSpace(value) == "" {
		return 86400000, nil
	}
	match := assistantWindow.FindStringSubmatch(strings.ToLower(strings.TrimSpace(value)))
	if match == nil {
		return 0, errors.New("unsupported history window")
	}
	amount, err := strconv.ParseInt(match[1], 10, 64)
	unit := map[string]int64{"m": 60000, "h": 3600000, "d": 86400000}[match[2]]
	if err != nil || amount < 1 || amount > 31*86400000/unit {
		return 0, errors.New("history window is out of range")
	}
	return amount * unit, nil
}

// Keep binding semantics pinned to the reviewed widget adapters. Client labels
// and option field names are descriptive data, never executable instructions.
func assistantWidgetShape(key string) string {
	switch key {
	case "BarCommon", "BarCrossrange", "BarLine", "CapsuleChart", "LineCommon", "LineGradients", "LineGradientSingle", "LineLinearSingle", "ScatterCommon", "ScatterLogarithmicRegression":
		return "chart"
	case "Funnel", "PieCommon":
		return "pie"
	case "Radar":
		return "radar"
	case "Heatmap":
		return "heatmap"
	case "TreeMap":
		return "treemap"
	case "WordCloud":
		return "words"
	case "Dial", "WaterPolo", "FlipperNumber", "Number":
		return "number"
	case "PieCircle":
		return "percent"
	case "Process":
		return "progress"
	case "TableList":
		return "table"
	case "TablesBasic":
		return "table-basic"
	case "TableScrollBoard":
		return "table-grid"
	case "TextBarrage", "TextCommon", "TextGradient":
		return "text"
	case "LocationMap":
		return "map"
	}
	if strings.HasPrefix(key, "VChart") {
		return "vchart"
	}
	return "static"
}

func assistantShapeBinding(shape string) (string, []string) {
	switch shape {
	case "chart", "pie", "vchart":
		return "chart.dataset", []string{"history", "point", "aggregate"}
	case "radar", "heatmap", "treemap", "words":
		return "chart.dataset", []string{"history", "point"}
	case "number", "percent":
		return "metric.value", []string{"point", "aggregate", "deviceSummary"}
	case "progress":
		return "progress.value", []string{"point", "aggregate", "deviceSummary"}
	case "table", "table-basic", "table-grid":
		return "table.rows", []string{"history", "aggregate", "alarmList", "eventList"}
	case "text":
		return "text.value", []string{"point", "aggregate", "deviceStatus", "deviceSummary"}
	case "map":
		return "map.points", []string{"location"}
	}
	return "", []string{}
}

func ValidateAssistantWidgets(widgets []AssistantWidget) error {
	if len(widgets) == 0 || len(widgets) > len(SafeWidgetKeys) {
		return errors.New("invalid widget catalogue")
	}
	seen := map[string]bool{}
	for _, widget := range widgets {
		shape := assistantWidgetShape(widget.Key)
		target, kinds := assistantShapeBinding(shape)
		// deviceSummary is additive: an older editor can keep advertising the
		// original exact catalogue until it upgrades its numeric adapters.
		legacyKinds := len(kinds) > 0 && kinds[len(kinds)-1] == "deviceSummary" && len(widget.Kinds) == len(kinds)-1
		if !SafeWidgetKeys[widget.Key] || seen[widget.Key] || widget.Shape != shape || widget.Target != target || (len(widget.Kinds) != len(kinds) && !legacyKinds) || len(widget.Zh) > 256 || len(widget.En) > 256 || len(widget.OptionFields) > 128 {
			return errors.New("unsupported widget metadata")
		}
		seen[widget.Key] = true
		provided := map[string]bool{}
		for _, kind := range widget.Kinds {
			provided[kind] = true
		}
		for _, kind := range kinds {
			if legacyKinds && kind == "deviceSummary" {
				continue
			}
			if !provided[kind] {
				return errors.New("unsupported widget binding metadata")
			}
		}
		for _, field := range widget.OptionFields {
			if !assistantField.MatchString(field) {
				return errors.New("invalid option field metadata")
			}
		}
	}
	return nil
}

func assistantFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func assistantNumber(value interface{}, fallback float64) float64 {
	switch n := value.(type) {
	case float64:
		if assistantFinite(n) {
			return n
		}
	case int:
		return float64(n)
	}
	return fallback
}

func assistantComponents(doc Envelope) map[string]map[string]interface{} {
	result := map[string]map[string]interface{}{}
	var visit func([]interface{})
	visit = func(items []interface{}) {
		for _, item := range items {
			component, _ := item.(map[string]interface{})
			id, _ := component["id"].(string)
			if id != "" {
				result[id] = component
			}
			if children, ok := component["groupList"].([]interface{}); ok {
				visit(children)
			}
		}
	}
	items, _ := doc.Content["componentList"].([]interface{})
	visit(items)
	return result
}

func strictAssistantJSON(data []byte, target interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid assistant JSON")
	}
	if decoder.Decode(new(interface{})) != io.EOF {
		return errors.New("assistant JSON must contain one value")
	}
	return nil
}

func ParseAssistantPlan(data []byte, doc Envelope, widgets []AssistantWidget, catalog AssistantCatalog, allowAlarms bool) (AssistantPlan, error) {
	var plan AssistantPlan
	if len(data) == 0 || len(data) > 512<<10 {
		return plan, errors.New("assistant plan exceeds limit")
	}
	if err := strictAssistantJSON(data, &plan); err != nil {
		return plan, err
	}
	for index := range plan.Sections {
		plan.Sections[index].Role = assistantSectionRole(plan.Sections[index].Role)
	}
	return plan, ValidateAssistantPlan(plan, doc, widgets, catalog, allowAlarms)
}

func ValidateAssistantPlan(plan AssistantPlan, doc Envelope, widgets []AssistantWidget, catalog AssistantCatalog, allowAlarms bool) error {
	if plan.ResultStatus != "" && plan.ResultStatus != "ready" && plan.ResultStatus != "structure_only" && plan.ResultStatus != "blocked" {
		return errors.New("unsupported assistant result status")
	}
	if err := ValidateAssistantWidgets(widgets); err != nil {
		return err
	}
	if len(plan.Summary) > 4000 || len(plan.Clarification) > 4000 || len(plan.Operations) > MaxComponents || len(plan.Sections) > 40 || len(plan.Limitations) > 32 {
		return errors.New("invalid assistant plan limits")
	}
	for _, limitation := range plan.Limitations {
		if len(limitation) > 1000 {
			return errors.New("invalid assistant limitation")
		}
	}
	if plan.Composition != "" && plan.Composition != "overview" && plan.Composition != "equipment" && plan.Composition != "compact" {
		return errors.New("unsupported assistant composition")
	}
	if len(plan.Sections) > 0 && len(plan.Operations) > 0 {
		return errors.New("an assistant plan must use either sections or operations")
	}
	if len(plan.Sections) == 0 && plan.Operations == nil {
		return errors.New("an assistant plan requires sections or operations")
	}
	if strings.TrimSpace(plan.Clarification) != "" {
		if len(plan.Operations) != 0 || len(plan.Sections) != 0 || plan.Canvas != nil {
			return errors.New("clarification must not modify the screen")
		}
		return nil
	}
	width, height := assistantNumber(doc.Canvas["width"], 1920), assistantNumber(doc.Canvas["height"], 1080)
	if plan.Canvas != nil {
		if plan.Canvas.Width != nil {
			width = *plan.Canvas.Width
		}
		if plan.Canvas.Height != nil {
			height = *plan.Canvas.Height
		}
	}
	if !assistantFinite(width) || !assistantFinite(height) || width < 64 || width > 16384 || height < 64 || height > 16384 {
		return errors.New("invalid assistant canvas size")
	}
	if len(plan.Sections) > 0 {
		return validateAssistantSections(plan.Sections, catalog, allowAlarms)
	}
	components := assistantComponents(doc)
	available := map[string]AssistantWidget{}
	for _, widget := range widgets {
		available[widget.Key] = widget
	}
	seen := map[string]bool{}
	count := len(components)
	for _, op := range plan.Operations {
		if !assistantID.MatchString(op.ID) || seen[op.ID] {
			return errors.New("invalid or duplicate operation id")
		}
		seen[op.ID] = true
		component, exists := components[op.ID]
		key := op.Key
		switch op.Type {
		case "add":
			if exists || !SafeWidgetKeys[key] {
				return errors.New("new component requires a unique id and reviewed widget")
			}
			count++
		case "update", "remove":
			if !exists || op.Key != "" {
				return errors.New("operation requires an existing component without changing its key")
			}
			config, _ := component["chartConfig"].(map[string]interface{})
			key, _ = config["key"].(string)
			if op.Type == "remove" {
				if op.X != nil || op.Y != nil || op.W != nil || op.H != nil || op.Title != nil || op.Option != nil || op.Binding != nil {
					return errors.New("remove cannot contain component edits")
				}
				count--
				continue
			}
		default:
			return errors.New("unsupported assistant operation")
		}
		widget, supported := available[key]
		if !supported {
			return errors.New("widget is missing from the current catalogue")
		}
		x, y, w, h := float64(24), float64(24), float64(320), float64(200)
		if exists {
			attr, _ := component["attr"].(map[string]interface{})
			x, y, w, h = assistantNumber(attr["x"], 0), assistantNumber(attr["y"], 0), assistantNumber(attr["w"], 320), assistantNumber(attr["h"], 200)
		}
		for i, number := range []*float64{op.X, op.Y, op.W, op.H} {
			if number != nil {
				if !assistantFinite(*number) {
					return errors.New("nonfinite component geometry")
				}
				switch i {
				case 0:
					x = *number
				case 1:
					y = *number
				case 2:
					w = *number
				case 3:
					h = *number
				}
			}
		}
		if x < 0 || y < 0 || w < 1 || h < 1 || x+w > width || y+h > height {
			return errors.New("component geometry exceeds the canvas")
		}
		if op.Title != nil && (len(*op.Title) > 256 || walkSafe(*op.Title, "title", 0, map[ResourceRef]bool{}) != nil) {
			return errors.New("invalid component title")
		}
		if err := walkSafe(op.Option, "option", 0, map[ResourceRef]bool{}); err != nil {
			return errors.New("unsafe assistant option")
		}
		if err := assistantSafeOption(op.Option); err != nil {
			return err
		}
		if len(widget.OptionFields) > 0 {
			fields := map[string]bool{}
			for _, field := range widget.OptionFields {
				fields[field] = true
			}
			for field := range op.Option {
				if !fields[field] {
					return errors.New("option field is absent from the widget catalogue")
				}
			}
		}
		if err := assistantSafeColors(op.Option, false); err != nil {
			return err
		}
		if len(op.Binding) > 0 && string(bytes.TrimSpace(op.Binding)) != "null" {
			var binding Binding
			if err := strictAssistantJSON(op.Binding, &binding); err != nil {
				return err
			}
			allowed := binding.Kind == "dataset"
			for _, kind := range widget.Kinds {
				allowed = allowed || kind == binding.Kind
			}
			if !allowed || widget.Target == "" || (binding.Kind == "alarmList" && !allowAlarms) {
				return errors.New("binding is incompatible with the component or permissions")
			}
			refs := map[ResourceRef]bool{}
			if err := validateBinding(binding, refs); err != nil {
				return err
			}
			if (binding.Scale != nil && !assistantFinite(*binding.Scale)) || (binding.Offset != nil && !assistantFinite(*binding.Offset)) || (binding.Kind == "history" && (binding.Aggregate == "sum" || binding.Aggregate == "count")) {
				return errors.New("unsupported binding transform or history aggregation")
			}
			for ref := range refs {
				fields, readable := catalog[ref.ID]
				if !readable || (ref.FieldKey != "" && !fields[ref.FieldKey]) {
					return errors.New("binding device or field is outside the readable catalogue")
				}
			}
		}
	}
	if count > MaxComponents {
		return errors.New("assistant would create too many components")
	}
	return nil
}

func assistantSafeOption(value interface{}) error {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			lower := strings.ToLower(key)
			for _, forbidden := range []string{"request", "event", "script", "lifecycle", "url", "href", "src", "filterfunction", "javascript"} {
				if strings.Contains(lower, forbidden) {
					return errors.New("active option capabilities are forbidden")
				}
			}
			if err := assistantSafeOption(item); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, item := range v {
			if err := assistantSafeOption(item); err != nil {
				return err
			}
		}
	case string:
		lower := strings.ToLower(v)
		if strings.Contains(lower, "function(") || strings.Contains(lower, "function (") || strings.Contains(lower, "=>") || strings.HasPrefix(strings.TrimSpace(lower), "//") {
			return errors.New("option code is forbidden")
		}
	}
	return nil
}

func assistantSafeColors(value interface{}, color bool) error {
	switch v := value.(type) {
	case map[string]interface{}:
		for key, item := range v {
			if err := assistantSafeColors(item, strings.Contains(strings.ToLower(key), "color")); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, item := range v {
			if err := assistantSafeColors(item, color); err != nil {
				return err
			}
		}
	case string:
		if color && v != "" && !assistantColorToken.MatchString(v) {
			return errors.New("assistant colors must use CSS design tokens")
		}
	}
	return nil
}

var assistantSectionRoles = map[string]bool{
	"header": true, "kpi": true, "trend": true, "distribution": true,
	"statuswall": true, "alarmtable": true, "eventtable": true, "list": true, "map": true,
}

// The model rarely repeats an enum verbatim. The client compiler already accepts
// these synonyms; keeping a second, stricter list here meant a plan that compiled
// fine in the editor was rejected by the server with no usable explanation.
var assistantRoleAliases = map[string]string{
	"header": "header", "title": "header", "banner": "header",
	"kpi": "kpi", "metric": "kpi", "metrics": "kpi", "stat": "kpi", "stats": "kpi", "summary": "kpi", "indicator": "kpi",
	"trend": "trend", "chart": "trend", "series": "trend", "timeseries": "trend", "line": "trend", "history": "trend",
	"distribution": "distribution", "pie": "distribution", "breakdown": "distribution", "composition": "distribution",
	"status": "statuswall", "statuswall": "statuswall", "connectivity": "statuswall", "health": "statuswall", "devices": "statuswall",
	"alarmtable": "alarmtable", "alarms": "alarmtable", "alarm": "alarmtable", "table": "alarmtable", "alert": "alarmtable", "alerts": "alarmtable",
	"eventtable": "eventtable", "event": "eventtable", "events": "eventtable",
	"list": "list", "ranking": "list", "top": "list",
	"map": "map", "geo": "map", "location": "map",
}

var assistantItemForms = map[string]bool{"": true, "count": true, "percent": true, "progress": true, "text": true}
var assistantItemViz = map[string]bool{"": true, "line": true, "area": true, "bar": true, "pie": true, "donut": true}
var assistantItemTones = map[string]bool{"": true, "primary": true, "danger": true, "warning": true, "success": true, "info": true, "neutral": true}

// Section roles arrive in whatever casing or separator style the model favours,
// so they are normalised the same way the client compiler does it: strip
// everything but letters, then resolve a documented synonym.
func assistantSectionRole(role string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(role) {
		if r >= 'a' && r <= 'z' {
			builder.WriteRune(r)
		}
	}
	normalised := builder.String()
	if alias, ok := assistantRoleAliases[normalised]; ok {
		return alias
	}
	return normalised
}

func assistantSafeText(value, field string) error {
	if len(value) > 256 {
		return errors.New("assistant section text exceeds limit")
	}
	return walkSafe(value, field, 0, map[ResourceRef]bool{})
}

// Sections carry no geometry and no options, so validation is about intent:
// known roles, bounded sizes, and every device/property inside the readable
// catalogue. The compiled document is still validated by the normal document
// endpoint before it can be applied.
func validateAssistantSections(sections []AssistantSection, catalog AssistantCatalog, allowAlarms bool) error {
	devicesSeen := 0
	for _, section := range sections {
		if section.Binding != nil {
			if err := validateBinding(*section.Binding, map[ResourceRef]bool{}); err != nil {
				return err
			}
		}
		if section.Priority < 0 || section.Priority > 3 {
			return errors.New("assistant section priority is out of range")
		}
		if err := assistantSafeText(section.Question, "sectionQuestion"); err != nil {
			return err
		}
		if len(section.Devices) > 200 {
			return errors.New("assistant section has too many devices")
		}
		for _, code := range section.Devices {
			if _, ok := catalog[code]; !ok {
				return fmt.Errorf("device %q is not readable", code)
			}
		}
		role := assistantSectionRole(section.Role)
		if !assistantSectionRoles[role] {
			return fmt.Errorf("unsupported section role %q", section.Role)
		}
		if err := assistantSafeText(section.Title, "sectionTitle"); err != nil {
			return err
		}
		if err := assistantSafeText(section.Subtitle, "sectionSubtitle"); err != nil {
			return err
		}
		if section.Span != nil && (!assistantFinite(*section.Span) || *section.Span < 1 || *section.Span > 12) {
			return errors.New("assistant section span is out of range")
		}
		if len(section.Window) > 16 {
			return errors.New("assistant section window is too long")
		}
		if role == "trend" || role == "distribution" || role == "eventtable" {
			if _, err := AssistantWindowMs(section.Window); err != nil {
				return err
			}
		}
		if !assistantItemViz[strings.ToLower(section.Viz)] {
			return fmt.Errorf("unsupported section visualization %q", section.Viz)
		}
		limit := 24
		if role == "kpi" {
			limit = 8
		}
		if len(section.Items) > limit {
			return errors.New("assistant section has too many items")
		}
		if role == "alarmtable" && !allowAlarms {
			return errors.New("assistant alarm table requires alarm permission")
		}
		switch role {
		case "kpi", "trend", "distribution", "statuswall":
			if len(section.Items) == 0 {
				return fmt.Errorf("section %q requires items", role)
			}
		}
		needsKeys := role == "kpi" || role == "trend" || role == "distribution"
		for _, item := range section.Items {
			if item.Placeholder {
				if item.Binding != nil || item.Device != "" || len(item.Devices) > 0 || item.Key != "" || len(item.Keys) > 0 {
					return errors.New("placeholder cannot bind data")
				}
				if err := assistantSafeText(item.Label, "itemLabel"); err != nil {
					return err
				}
				continue
			}
			if item.Binding != nil {
				if err := validateBinding(*item.Binding, map[ResourceRef]bool{}); err != nil {
					return err
				}
				if item.Binding.Kind != "dataset" {
					return errors.New("compiled item binding must be a dataset")
				}
				if role != "kpi" && role != "trend" && role != "distribution" {
					return errors.New("unsupported dataset presentation")
				}
				if err := assistantSafeText(item.Label, "itemLabel"); err != nil {
					return err
				}
				if err := assistantSafeText(item.Unit, "itemUnit"); err != nil {
					return err
				}
				if !assistantItemForms[strings.ToLower(item.Form)] || !assistantItemViz[strings.ToLower(item.Viz)] || !assistantItemTones[strings.ToLower(item.Tone)] {
					return errors.New("unsupported dataset presentation metadata")
				}
				continue
			}
			if item.Kind != "" && item.Kind != "point" && item.Kind != "aggregate" && item.Kind != "history" && item.Kind != "deviceStatus" && item.Kind != "deviceSummary" {
				return errors.New("unsupported item binding kind")
			}
			if item.Kind == "deviceSummary" {
				if role != "kpi" || !DeviceSummaryAggregate(item.Aggregate) {
					return errors.New("device summary requires a KPI and a supported statistic")
				}
				if item.Aggregate == "onlineRate" {
					if item.Unit != "" && item.Unit != "%" {
						return errors.New("device online rate unit must be percent")
					}
				} else if item.Unit != "" || strings.EqualFold(item.Form, "percent") || strings.EqualFold(item.Form, "progress") {
					return errors.New("device summary counts require a numeric or text form without a unit")
				}
			} else if item.Aggregate != "" && item.Aggregate != "sum" && item.Aggregate != "avg" && item.Aggregate != "min" && item.Aggregate != "max" && item.Aggregate != "count" {
				return errors.New("unsupported item aggregation")
			}
			if err := assistantSafeText(item.Label, "itemLabel"); err != nil {
				return err
			}
			if err := assistantSafeText(item.Unit, "itemUnit"); err != nil {
				return err
			}
			if !assistantItemForms[strings.ToLower(item.Form)] || !assistantItemViz[strings.ToLower(item.Viz)] || !assistantItemTones[strings.ToLower(item.Tone)] {
				return fmt.Errorf("unsupported item metadata form=%q viz=%q tone=%q", item.Form, item.Viz, item.Tone)
			}
			if item.Precision != nil && (!assistantFinite(*item.Precision) || *item.Precision < 0 || *item.Precision > 6) {
				return errors.New("assistant item precision is out of range")
			}
			codes := make([]string, 0, len(item.Devices)+1)
			if item.Device != "" {
				codes = append(codes, item.Device)
			}
			codes = append(codes, item.Devices...)
			keys := make([]string, 0, len(item.Keys)+1)
			if item.Key != "" {
				keys = append(keys, item.Key)
			}
			keys = append(keys, item.Keys...)
			if item.Kind == "deviceSummary" {
				if err := validateBinding(Binding{Kind: item.Kind, DeviceCodes: codes, Keys: keys, Aggregate: item.Aggregate}, map[ResourceRef]bool{}); err != nil {
					return err
				}
			}
			if role == "kpi" && item.Kind != "deviceSummary" && (len(keys) != 1 || (item.Kind != "" && item.Kind != "point" && item.Kind != "aggregate") || (len(codes) > 1 && item.Kind != "" && item.Kind != "aggregate")) {
				return errors.New("KPI requires one key and a point or aggregate binding")
			}
			if role == "trend" && item.Kind != "" && item.Kind != "history" {
				return errors.New("trend requires history")
			}
			if role == "distribution" && item.Kind != "" && item.Kind != "history" && item.Kind != "aggregate" {
				return errors.New("unsupported distribution binding")
			}
			if (role == "trend" || (role == "distribution" && item.Kind != "aggregate")) && (item.Aggregate == "sum" || item.Aggregate == "count") {
				return errors.New("unsupported history aggregation")
			}
			if item.Kind == "aggregate" && len(keys) != 1 {
				return errors.New("aggregate requires one key")
			}
			if role == "statuswall" && (len(keys) > 0 || (item.Kind != "" && item.Kind != "deviceStatus")) {
				return errors.New("status wall requires deviceStatus")
			}
			if len(codes) == 0 {
				if needsKeys || len(keys) > 0 {
					return errors.New("assistant item requires a device")
				}
				continue
			}
			devicesSeen += len(codes)
			for _, code := range codes {
				fields, readable := catalog[code]
				if !readable {
					return fmt.Errorf("device %q is not readable", code)
				}
				for _, key := range keys {
					if key != "" && !fields[key] {
						return fmt.Errorf("property %q is not readable on %q", key, code)
					}
				}
			}
			if needsKeys && item.Kind != "deviceSummary" && len(keys) == 0 {
				return fmt.Errorf("item on device %q needs a property key", codes[0])
			}
		}
	}
	if devicesSeen > 500 {
		return errors.New("assistant plan references too many devices")
	}
	return nil
}
