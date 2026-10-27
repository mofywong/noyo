package core

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"noyo/core/dataquery"
	"noyo/core/screen"
	"noyo/core/store"
)

// Product context is definition-only. Membership is resolved after metric
// selection, and is never expanded into the model's context.
const screenAssistantContextDeviceBudget = 10000

type screenAssistantContextScope struct {
	Mode         string   `json:"mode"`
	ProductCodes []string `json:"productCodes"`
	DeviceCodes  []string `json:"deviceCodes"`
}

type screenAssistantContextGroup struct {
	ID string `json:"id"`
	// Scope names what the group selects: a product's live membership, one
	// explicitly chosen device, or every device in the project. A project group
	// is the only scope that can carry a genuine cross-device total.
	Scope       string   `json:"scope,omitempty"`
	ProductCode string   `json:"productCode,omitempty"`
	Name        string   `json:"name"`
	Count       int      `json:"count"`
	Dynamic     bool     `json:"dynamic"`
	ScopeState  string   `json:"scopeState,omitempty"`
	DeviceCodes []string `json:"-"`
}

type screenAssistantContext struct {
	Devices []screenAssistantDevice
	Catalog screen.AssistantCatalog
	Profile screen.SceneProfile
	Groups  []screenAssistantContextGroup
	Scope   screenAssistantContextScope
}

func screenAssistantContextID(kind, code string) string {
	sum := sha256.Sum256([]byte(code))
	return fmt.Sprintf("%s_%x", kind, sum[:6])
}

func screenAssistantContextCodes(values *[]string, limit int) ([]string, error) {
	result := []string{}
	if values == nil {
		return result, nil
	}
	if len(*values) > limit {
		return nil, screenFail(400, "Context selection exceeds the supported budget / 所选上下文超出支持的预算")
	}
	seen := map[string]bool{}
	for _, value := range *values {
		code := strings.TrimSpace(value)
		if code == "" || len(code) > 128 {
			return nil, screenFail(400, "Invalid context identifier / 上下文标识无效")
		}
		if !seen[code] {
			result = append(result, code)
			seen[code] = true
		}
	}
	sort.Strings(result)
	return result, nil
}

// Only project products and explicitly shared definitions are selectable. A
// readable device can reference a shared definition; another project's product
// cannot be used as a way to discover that project's assets.
func screenAssistantContextProducts(r *ghttp.Request, row *store.IOCScreen) ([]store.Product, error) {
	var products []store.Product
	err := store.DB.WithContext(r.Context()).Where("tenant_id = 0 OR (tenant_id = ? AND (project_id = ? OR project_id = 0))", row.TenantID, row.ProjectID).Order("code ASC").Limit(1001).Find(&products).Error
	if err != nil {
		return nil, screenFail(503, "Product catalogue unavailable / 产品目录暂不可用")
	}
	if len(products) > 1000 {
		return nil, screenFail(422, "Product context capacity is exceeded / 产品上下文超出当前支持容量")
	}
	result := []store.Product{}
	for _, product := range products {
		if canAccessProduct(r, &product) {
			result = append(result, product)
		}
	}
	return result, nil
}

func screenAssistantContextFields(code string) []screen.ProfileField {
	return screenAssistantContextPropertyFields(loadProductTSLProperties(code))
}

// Deployed device counts are the only evidence that a product definition can
// carry a live metric. A definition with no device is a valid catalogue entry,
// but binding a discovered panel to it can only ever return no_data.
func screenAssistantProductDeviceCounts(r *ghttp.Request, row *store.IOCScreen) (map[string]int, int, error) {
	counts := []struct {
		ProductCode string
		Total       int
	}{}
	err := store.DB.WithContext(r.Context()).Model(&store.Device{}).
		Select("product_code, COUNT(*) AS total").
		Where("tenant_id = ? AND project_id = ?", row.TenantID, row.ProjectID).
		Group("product_code").Scan(&counts).Error
	if err != nil {
		return nil, 0, screenFail(503, "Device catalogue unavailable / 设备目录暂不可用")
	}
	result := map[string]int{}
	total := 0
	for _, item := range counts {
		if item.ProductCode == "" {
			continue
		}
		result[item.ProductCode] = item.Total
		total += item.Total
	}
	return result, total, nil
}

func screenAssistantContextPropertyFields(properties []map[string]any) []screen.ProfileField {
	fields := []screen.ProfileField{}
	for _, property := range properties {
		key := propertyStringField(property, "identifier")
		if key == "" || len(key) > 128 || screenAssistantPrivateFieldKey(key) {
			continue
		}
		field := screenAssistantFieldMetadata(property, key)
		field.Role = screen.ClassifyFieldRole(field.Key, field.Zh, field.En, field.Type, field.Unit)
		if field.Role != screen.RoleIgnore {
			fields = append(fields, screen.DescribeField(field))
		}
	}
	return fields
}

func screenAssistantProductSchema(product store.Product) screen.ProfileProduct {
	var config map[string]any
	_ = json.Unmarshal([]byte(product.Config), &config)
	if tsl, ok := config["tsl"].(map[string]any); ok {
		config = tsl
	}
	properties := []map[string]any{}
	if raw, ok := config["properties"].([]any); ok {
		for _, value := range raw {
			if property, ok := value.(map[string]any); ok {
				properties = append(properties, property)
			}
		}
	}
	return screen.ProfileProduct{Code: product.Code, Name: product.Name, Fields: screenAssistantContextPropertyFields(properties), Events: screenAssistantEventMetadata(product.Config)}
}

// Unset discovers accessible product definitions. Explicit device context is
// independent: selecting a product never loads or implicitly selects its devices.
func (s *Server) screenAssistantContext(r *ghttp.Request, row *store.IOCScreen, _ screen.Envelope, productSelection, deviceSelection *[]string) (*screenAssistantContext, error) {
	products, err := screenAssistantContextCodes(productSelection, 128)
	if err != nil {
		return nil, err
	}
	codes, err := screenAssistantContextCodes(deviceSelection, 500)
	if err != nil {
		return nil, err
	}
	defaultAll := productSelection == nil && deviceSelection == nil
	result := &screenAssistantContext{Devices: []screenAssistantDevice{}, Catalog: screen.AssistantCatalog{}, Groups: []screenAssistantContextGroup{}, Scope: screenAssistantContextScope{Mode: "selection", ProductCodes: products, DeviceCodes: codes}}
	if defaultAll {
		result.Scope.Mode = "all"
	}
	auth := requestAuthContext(r)
	if auth == nil || !auth.HasProjectPermission("device:list", row.ProjectID) {
		if len(products)+len(codes) > 0 {
			return nil, screenFail(403, "Device context permission denied / 无设备上下文读取权限")
		}
		result.Profile = screenAssistantProfile(nil)
		return result, nil
	}
	visible, err := screenAssistantContextProducts(r, row)
	if err != nil {
		return nil, err
	}
	counts, totalDevices, err := screenAssistantProductDeviceCounts(r, row)
	if err != nil {
		return nil, err
	}
	byProduct := map[string]store.Product{}
	for _, product := range visible {
		byProduct[product.Code] = product
	}
	selectedProducts := map[string]bool{}
	if defaultAll {
		// Discovery must not advertise definitions that cannot carry data: an
		// empty product would otherwise become a permanently blank panel.
		for _, product := range visible {
			if counts[product.Code] > 0 {
				selectedProducts[product.Code] = true
			}
		}
	}
	for _, code := range products {
		if _, ok := byProduct[code]; !ok {
			return nil, screenFail(403, "Selected product is unavailable / 所选产品不可用")
		}
		selectedProducts[code] = true
	}
	var devices []store.Device
	if len(codes) > 0 {
		query := store.DB.WithContext(r.Context()).Select("id", "tenant_id", "project_id", "code", "name", "product_code", "enabled").Where("tenant_id = ? AND project_id = ? AND code IN ?", row.TenantID, row.ProjectID, codes)
		if err := query.Order("code ASC").Find(&devices).Error; err != nil {
			return nil, screenFail(503, "Device catalogue unavailable / 设备目录暂不可用")
		}
	}
	fields := map[string][]screen.ProfileField{}
	events := map[string][]screen.ProfileEvent{}
	readable := map[string]bool{}
	for _, device := range devices {
		if !canAccessDevice(r, &device) {
			continue
		}
		product, ok := byProduct[device.ProductCode]
		if !ok {
			continue
		}
		if _, loaded := fields[product.Code]; !loaded {
			fields[product.Code] = screenAssistantContextFields(product.Code)
			events[product.Code] = screenAssistantEventMetadata(product.Config)
		}
		item := screenAssistantDevice{Code: device.Code, Name: screenAssistantText(device.Name, 128), ProductCode: product.Code, ProductName: screenAssistantText(product.Name, 128), Enabled: device.Enabled, State: s.screenAssistantDeviceState(device.Code), Fields: fields[product.Code], Events: events[product.Code]}
		result.Devices = append(result.Devices, item)
		result.Catalog[device.Code] = map[string]bool{}
		for _, field := range fields[product.Code] {
			result.Catalog[device.Code][field.Key] = true
		}
		readable[device.Code] = true
	}
	for _, code := range codes {
		if !readable[code] {
			return nil, screenFail(403, "Selected device is unavailable / 所选设备不可用")
		}
	}
	result.Profile = screenAssistantProfile(result.Devices)
	profileProducts := map[string]bool{}
	for _, product := range result.Profile.Products {
		profileProducts[product.Code] = true
	}
	for code := range selectedProducts {
		product := byProduct[code]
		// -1 means unresolved, never a claim that this product has no devices.
		group := screenAssistantContextGroup{ID: screenAssistantContextID("product", code), Scope: "product", ProductCode: code, Name: screenAssistantText(product.Name, 128), Count: -1, Dynamic: true, ScopeState: "unresolved"}
		if !defaultAll && counts[code] == 0 {
			// Explicitly selected and currently undeployed. The scope stays a
			// resolvable waiting view, but it is reported as empty so the plan does
			// not present it as a measured result.
			group.ScopeState = "empty"
		}
		result.Groups = append(result.Groups, group)
		if !profileProducts[code] {
			definition := screen.BuildSceneProfile([]screen.ProfileInput{{ProductCode: code, ProductName: product.Name, Fields: screenAssistantContextFields(code)}}, 64).Products[0]
			definition.Devices = 0
			definition.Events = screenAssistantEventMetadata(product.Config)
			result.Profile.Products = append(result.Profile.Products, definition)
		}
	}
	// Every single-device KPI answers "what is this box reading". A screen also
	// needs project totals that no product scope can express. An explicitly
	// cleared scope stays empty: a project-wide total is a scope grant, so it is
	// offered only for discovery or when products were selected.
	if totalDevices > 0 && (defaultAll || len(selectedProducts) > 0) {
		result.Groups = append(result.Groups, screenAssistantContextGroup{
			ID:         screenAssistantContextID("project", fmt.Sprintf("%d:%d", row.TenantID, row.ProjectID)),
			Scope:      "project",
			Count:      totalDevices,
			Dynamic:    true,
			ScopeState: "project",
		})
	}
	for _, device := range result.Devices {
		result.Groups = append(result.Groups, screenAssistantContextGroup{ID: screenAssistantContextID("device", device.Code), Scope: "device", ProductCode: device.ProductCode, Name: device.Name, Count: 1, Dynamic: false, ScopeState: "resolved", DeviceCodes: []string{device.Code}})
	}
	sort.Slice(result.Groups, func(i, j int) bool { return result.Groups[i].ID < result.Groups[j].ID })
	sort.Slice(result.Profile.Products, func(i, j int) bool { return result.Profile.Products[i].Code < result.Profile.Products[j].Code })
	for index := range result.Profile.Products {
		result.Profile.Products[index].DefinitionVersion = byProduct[result.Profile.Products[index].Code].Rev
	}
	// Keep the full authorized catalogue above for binding validation; the model
	// never needs this per-device array when it can select group/candidate refs.
	result.Profile.Devices = nil
	return result, nil
}

// A shared name is only a discovery hint. Require the complete executable
// property/event schema to agree before offering an in-context alternative.
// Truncated or empty schemas cannot establish this equivalence.
func screenAssistantCompatibleProducts(a, b screen.ProfileProduct) bool {
	if a.FieldsOmitted != 0 || b.FieldsOmitted != 0 || len(a.Fields)+len(a.Events) == 0 || len(b.Fields)+len(b.Events) == 0 {
		return false
	}
	contract := func(product screen.ProfileProduct) string {
		normalize := func(fields []screen.ProfileField) []screen.ProfileField {
			result := append([]screen.ProfileField{}, fields...)
			for i := range result {
				result[i].Zh, result[i].En, result[i].Provenance = "", "", ""
				result[i].Aggregations = append([]string{}, result[i].Aggregations...)
				sort.Strings(result[i].Aggregations)
			}
			sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
			return result
		}
		events := append([]screen.ProfileEvent{}, product.Events...)
		for i := range events {
			events[i].Zh, events[i].En = "", ""
			events[i].Fields = normalize(events[i].Fields)
		}
		sort.Slice(events, func(i, j int) bool { return events[i].Key < events[j].Key })
		value, _ := json.Marshal(struct {
			Fields []screen.ProfileField
			Events []screen.ProfileEvent
		}{normalize(product.Fields), events})
		return string(value)
	}
	return contract(a) == contract(b)
}

func screenAssistantBindingProducts(r *ghttp.Request, row *store.IOCScreen, devices []screenAssistantDevice) ([]interface{}, error) {
	result := []interface{}{}
	if auth := requestAuthContext(r); auth == nil || !auth.HasProjectPermission("device:list", row.ProjectID) {
		return result, nil
	}
	products, err := screenAssistantContextProducts(r, row)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, device := range devices {
		counts[device.ProductCode]++
	}
	for _, product := range products {
		fields := screenAssistantContextFields(product.Code)
		fieldCount := len(fields)
		if len(fields) > 64 {
			fields = fields[:64]
		}
		result = append(result, map[string]interface{}{"code": product.Code, "name": product.Name, "deviceCount": counts[product.Code], "fields": fields, "fieldCount": fieldCount, "fieldsOmitted": fieldCount - len(fields), "events": screenAssistantEventMetadata(product.Config), "definitionVersion": product.Rev})
	}
	return result, nil
}

// Query compilation reuses the shared dataquery contract. Product membership is
// resolved and authorized again by screenPrepareData on every runtime refresh.
// A false result means a real capability/capacity gap, never "waiting for data".
func screenAssistantGroupQuery(group screenAssistantContextGroup, kind, key, aggregate string, windowMs int64) (screen.Binding, bool) {
	return screenAssistantGroupQueryField(group, kind, key, aggregate, windowMs, nil, "zh")
}

func screenAssistantGroupQueryField(group screenAssistantContextGroup, kind, key, aggregate string, windowMs int64, field *screen.ProfileField, locale string) (screen.Binding, bool) {
	project := group.Scope == "project"
	limit := dataquery.MaxDevices
	if kind == "deviceSummary" || kind == "aggregate" || kind == "distribution" || kind == "alarmList" {
		limit = screenAssistantContextDeviceBudget
	}
	if project {
		// An empty selector is resolved against the whole project at runtime, and
		// that resolution is capped at MaxDevices. Advertising a wider capacity
		// here would only fail later on every refresh.
		limit = dataquery.MaxDevices
	}
	if (!group.Dynamic && len(group.DeviceCodes) == 0) || (group.Dynamic && group.ProductCode == "" && !project) || group.Count > limit {
		return screen.Binding{}, false
	}
	plan := dataquery.Plan{SchemaVersion: dataquery.SchemaVersion, Sources: []dataquery.Source{{ID: "raw", Kind: "latest", Selector: dataquery.Selector{ProductCode: group.ProductCode}}}, Output: "stats", WindowMs: windowMs}
	if !group.Dynamic {
		plan.Sources[0].Selector = dataquery.Selector{DeviceCodes: group.DeviceCodes}
	}
	mapping := &screen.DataMapping{Value: "value"}
	count := dataquery.Measure{ID: "value", Function: "count", Field: "device_code"}
	switch kind {
	case "deviceSummary":
		if aggregate != "" {
			key = aggregate
		}
		plan.Sources[0].Kind = "devices"
		input := "raw"
		switch key {
		case "total":
		case "online", "offline", "unknown":
			expression := map[string]string{"online": "online == true", "offline": "online == false", "unknown": "online == nil"}[key]
			plan.Steps = append(plan.Steps, dataquery.Step{ID: "selected", Op: "filter", Input: "raw", Expression: expression})
			input = "selected"
		case "onlineRate":
			plan.Steps = []dataquery.Step{{ID: "flags", Op: "derive", Input: "raw", Field: "online_flag", Expression: "ifelse(online == true, 1, 0)"}, {ID: "counts", Op: "aggregate", Input: "flags", Measures: []dataquery.Measure{{ID: "total", Function: "count", Field: "device_code"}, {ID: "online_count", Function: "sum", Field: "online_flag"}}}, {ID: "stats", Op: "derive", Input: "counts", Field: "value", Expression: "ifelse(total > 0, online_count * 100 / total, nil)"}}
		default:
			return screen.Binding{}, false
		}
		if key != "onlineRate" {
			plan.Steps = append(plan.Steps, dataquery.Step{ID: "stats", Op: "aggregate", Input: input, Measures: []dataquery.Measure{count}})
		}
	case "alarmCount", "eventCount":
		// A cross-device incident total. It counts incident records, never device
		// properties, so it stays a project-level figure rather than a sum of
		// per-device counters.
		field := "id"
		plan.Sources[0].Kind = "alarms"
		if kind == "eventCount" {
			field = "ts"
			plan.Sources[0].Kind = "events"
		}
		plan.Steps = []dataquery.Step{{ID: "stats", Op: "aggregate", Input: "raw", Measures: []dataquery.Measure{{ID: "value", Function: "count", Field: field}}}}
	case "aggregate", "history":
		if key == "" || (aggregate != "avg" && aggregate != "min" && aggregate != "max" && aggregate != "sum" && aggregate != "count") {
			return screen.Binding{}, false
		}
		plan.Sources[0].Fields = []string{key}
		measure := dataquery.Measure{ID: "value", Function: aggregate, Field: key}
		if kind == "history" {
			if windowMs < 60000 || windowMs > 31*86400000 || (aggregate != "avg" && aggregate != "min" && aggregate != "max") {
				return screen.Binding{}, false
			}
			plan.Sources[0].Kind = "telemetry"
			interval := windowMs / 60
			if interval < 60000 {
				interval = 60000
			}
			plan.Steps = []dataquery.Step{{ID: "buckets", Op: "bucket", Input: "raw", Field: "ts", IntervalMs: interval}, {ID: "peers", Op: "aggregate", Input: "buckets", GroupBy: []string{"ts", "device_code"}, Measures: []dataquery.Measure{measure}}, {ID: "stats", Op: "aggregate", Input: "peers", GroupBy: []string{"ts"}, Measures: []dataquery.Measure{{ID: "value", Function: aggregate, Field: "value"}}}, {ID: "ordered", Op: "sort", Input: "stats", Orders: []dataquery.Order{{Field: "ts"}}}}
			plan.Output, mapping.Category = "ordered", "ts"
		} else {
			plan.Steps = []dataquery.Step{{ID: "stats", Op: "aggregate", Input: "raw", Measures: []dataquery.Measure{measure}}}
		}
	case "distribution":
		if key == "" {
			return screen.Binding{}, false
		}
		plan.Sources[0].Fields = []string{key}
		mapping.Category = key
		plan.Steps = []dataquery.Step{{ID: "reported", Op: "filter", Input: "raw", Expression: fmt.Sprintf("row[%q] != nil", key)}, {ID: "stats", Op: "aggregate", Input: "reported", GroupBy: []string{key}, Measures: []dataquery.Measure{{ID: "value", Function: "count_distinct", Field: "device_code"}}}}
		if field != nil {
			// A state is a per-device reading, not a magnitude. Grouping the raw
			// boolean produces unreadable true/false categories and drops every
			// state no device currently reports, which turns a two-state breakdown
			// into a single-colour circle. Label the declared states and always
			// emit all of them.
			if categories := screenAssistantStateCategories(*field, locale); len(categories) > 0 {
				if screenAssistantLabelDistribution(&plan, key, categories) {
					mapping.Category = "category"
				}
			}
		}
	case "eventList", "alarmList":
		plan.Sources[0].Kind = "events"
		mapping.Columns = []string{"ts", "device_name", "event_zh", "event_type", "parameters"}
		if kind == "alarmList" {
			plan.Sources[0].Kind = "alarms"
			mapping.Columns = []string{"ts", "device_code", "title", "severity", "handling_status"}
		}
		plan.Steps = []dataquery.Step{{ID: "recent", Op: "sort", Input: "raw", Orders: []dataquery.Order{{Field: "ts", Desc: true}}}, {ID: "stats", Op: "limit", Input: "recent", Limit: 120}}
	default:
		return screen.Binding{}, false
	}
	if err := dataquery.Validate(plan); err != nil {
		return screen.Binding{}, false
	}
	return screen.Binding{Kind: "dataset", Query: &plan, Mapping: mapping, WindowMs: windowMs}, true
}

// A declared state list is authoritative. A boolean without declared labels still
// has exactly two states, so its readable wording comes from the property name.
type screenAssistantStateCategory struct {
	Match interface{}
	Label string
}

var screenAssistantStatePairs = []struct {
	needles []string
	zh      [2]string
	en      [2]string
}{
	{[]string{"手自动", "手动自动", "auto"}, [2]string{"自动", "手动"}, [2]string{"Auto", "Manual"}},
	{[]string{"释放", "闭锁", "release"}, [2]string{"已释放", "未释放"}, [2]string{"Released", "Latched"}},
	{[]string{"切断", "cut off"}, [2]string{"已切断", "未切断"}, [2]string{"Cut off", "Open"}},
	{[]string{"复位", "reset"}, [2]string{"已复位", "未复位"}, [2]string{"Reset", "Not reset"}},
	{[]string{"消音", "mute", "silence"}, [2]string{"已消音", "未消音"}, [2]string{"Silenced", "Audible"}},
	{[]string{"充电", "浮充", "charg"}, [2]string{"充电中", "未充电"}, [2]string{"Charging", "Idle"}},
	{[]string{"应急", "强启", "emergency"}, [2]string{"应急点亮", "正常"}, [2]string{"Emergency on", "Normal"}},
	{[]string{"就绪", "ready"}, [2]string{"就绪", "未就绪"}, [2]string{"Ready", "Not ready"}},
	{[]string{"动作", "actuat", "tripped"}, [2]string{"已动作", "未动作"}, [2]string{"Actuated", "Idle"}},
	{[]string{"开启", "开门", "打开", "open"}, [2]string{"已开启", "已关闭"}, [2]string{"Open", "Closed"}},
	{[]string{"关闭", "closed", "close limit"}, [2]string{"已关闭", "未关闭"}, [2]string{"Closed", "Open"}},
	{[]string{"运行", "run"}, [2]string{"运行", "停止"}, [2]string{"Running", "Stopped"}},
	{[]string{"故障", "断线", "短路", "fault", "error"}, [2]string{"故障", "正常"}, [2]string{"Fault", "Normal"}},
	{[]string{"报警", "火警", "超限", "超温", "欠压", "过压", "缺相", "高限", "低限", "低液位", "溢流", "预警", "alarm", "fire"}, [2]string{"报警", "正常"}, [2]string{"Alarm", "Normal"}},
	{[]string{"正常", "normal"}, [2]string{"正常", "异常"}, [2]string{"Normal", "Abnormal"}},
	{[]string{"在线", "在位", "online"}, [2]string{"在线", "离线"}, [2]string{"Online", "Offline"}},
}

func screenAssistantBoolLabels(field screen.ProfileField, locale string) (string, string) {
	text := strings.ToLower(strings.Join([]string{field.Zh, field.En}, " "))
	for _, pair := range screenAssistantStatePairs {
		for _, needle := range pair.needles {
			if strings.Contains(text, needle) {
				if locale == "zh" {
					return pair.zh[0], pair.zh[1]
				}
				return pair.en[0], pair.en[1]
			}
		}
	}
	if locale == "zh" {
		return "是", "否"
	}
	return "Yes", "No"
}

func screenAssistantStateLiteral(value string) interface{} {
	if number, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
		return number
	}
	return value
}

// Category values arrive as booleans, numbers or strings depending on the device
// firmware, so a match accepts the declared value in both its typed and textual
// form. Only literals are emitted; no field value is interpolated as code.
func screenAssistantStateMatch(key string, match interface{}) string {
	literal := ""
	switch value := match.(type) {
	case bool:
		literal = strconv.FormatBool(value)
	case float64:
		literal = strconv.FormatFloat(value, 'g', -1, 64)
		return fmt.Sprintf("(row[%q] == %s || row[%q] == %q)", key, literal, key, literal)
	case string:
		literal = strconv.Quote(value)
	default:
		return "nil"
	}
	return fmt.Sprintf("row[%q] == %s", key, literal)
}

func screenAssistantStateCategories(field screen.ProfileField, locale string) []screenAssistantStateCategory {
	if len(field.Values) > 0 {
		keys := make([]string, 0, len(field.Values))
		for key := range field.Values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) > 8 {
			keys = keys[:8]
		}
		result := make([]screenAssistantStateCategory, 0, len(keys))
		for _, key := range keys {
			label := screenAssistantText(field.Values[key], 32)
			if label == "" {
				label = screenAssistantText(key, 32)
			}
			result = append(result, screenAssistantStateCategory{Match: screenAssistantStateLiteral(key), Label: label})
		}
		return result
	}
	switch strings.ToLower(strings.TrimSpace(field.Type)) {
	case "bool", "boolean":
		on, off := screenAssistantBoolLabels(field, locale)
		return []screenAssistantStateCategory{{Match: true, Label: on}, {Match: false, Label: off}}
	}
	return nil
}

// Rewrite the distribution plan so every declared state appears once with a
// readable label and a device count. Static zero rows are unioned in, then
// re-aggregated, so a state with no reporting device is a real zero rather than
// a missing category.
func screenAssistantLabelDistribution(plan *dataquery.Plan, key string, categories []screenAssistantStateCategory) bool {
	expression := "nil"
	for index := len(categories) - 1; index >= 0; index-- {
		expression = fmt.Sprintf("ifelse(%s, %s, %s)", screenAssistantStateMatch(key, categories[index].Match), strconv.Quote(categories[index].Label), expression)
	}
	zeros := make([]dataquery.Row, 0, len(categories))
	for _, category := range categories {
		zeros = append(zeros, dataquery.Row{"category": category.Label, "value": float64(0)})
	}
	plan.Sources = append(plan.Sources, dataquery.Source{ID: "declared", Kind: "static", Rows: zeros})
	plan.Steps = []dataquery.Step{
		{ID: "reported", Op: "filter", Input: "raw", Expression: fmt.Sprintf("row[%q] != nil", key)},
		{ID: "labeled", Op: "derive", Input: "reported", Field: "category", Expression: expression},
		{ID: "known", Op: "filter", Input: "labeled", Expression: `row["category"] != nil`},
		{ID: "counted", Op: "aggregate", Input: "known", GroupBy: []string{"category"}, Measures: []dataquery.Measure{{ID: "value", Function: "count_distinct", Field: "device_code"}}},
		{ID: "merged", Op: "union", Input: "counted", Right: "declared"},
		{ID: "stats", Op: "aggregate", Input: "merged", GroupBy: []string{"category"}, Measures: []dataquery.Measure{{ID: "value", Function: "sum", Field: "value"}}},
		{ID: "ordered", Op: "sort", Input: "stats", Orders: []dataquery.Order{{Field: "value", Desc: true}}},
	}
	plan.Output = "ordered"
	return dataquery.Validate(*plan) == nil
}
