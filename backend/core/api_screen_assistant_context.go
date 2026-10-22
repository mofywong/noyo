package core

import (
	"crypto/sha256"
	"fmt"
	"sort"
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
	ID          string   `json:"id"`
	ProductCode string   `json:"productCode,omitempty"`
	Name        string   `json:"name"`
	Count       int      `json:"count"`
	Dynamic     bool     `json:"dynamic"`
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
	fields := []screen.ProfileField{}
	for _, property := range loadProductTSLProperties(code) {
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
	byProduct := map[string]store.Product{}
	for _, product := range visible {
		byProduct[product.Code] = product
	}
	selectedProducts := map[string]bool{}
	if defaultAll {
		for _, product := range visible {
			selectedProducts[product.Code] = true
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
		result.Groups = append(result.Groups, screenAssistantContextGroup{ID: screenAssistantContextID("product", code), ProductCode: code, Name: screenAssistantText(product.Name, 128), Count: -1, Dynamic: true})
		if !profileProducts[code] {
			definition := screen.BuildSceneProfile([]screen.ProfileInput{{ProductCode: code, ProductName: product.Name, Fields: screenAssistantContextFields(code)}}, 64).Products[0]
			definition.Devices = 0
			definition.Events = screenAssistantEventMetadata(product.Config)
			result.Profile.Products = append(result.Profile.Products, definition)
		}
	}
	for _, device := range result.Devices {
		result.Groups = append(result.Groups, screenAssistantContextGroup{ID: screenAssistantContextID("device", device.Code), ProductCode: device.ProductCode, Name: device.Name, Count: 1, Dynamic: false, DeviceCodes: []string{device.Code}})
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
	limit := dataquery.MaxDevices
	if kind == "deviceSummary" || kind == "aggregate" || kind == "distribution" || kind == "alarmList" {
		limit = screenAssistantContextDeviceBudget
	}
	if (group.Dynamic && group.ProductCode == "") || (!group.Dynamic && len(group.DeviceCodes) == 0) || group.Count > limit {
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
		plan.Steps = []dataquery.Step{{ID: "reported", Op: "filter", Input: "raw", Expression: fmt.Sprintf("row[%q] != nil", key)}, {ID: "stats", Op: "aggregate", Input: "reported", GroupBy: []string{key}, Measures: []dataquery.Measure{{ID: "value", Function: "count_distinct", Field: "device_code"}}}}
		mapping.Category = key
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
