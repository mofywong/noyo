// Package screen defines the versioned, declarative document contract shared by
// the editor, persistence and runtime. It never evaluates editor-supplied code.
package screen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"noyo/core/dataquery"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	SchemaVersion = "noyo.screen/1"
	// EngineBuildID pins the adapter generation stamped into new documents. The
	// adapter version rises whenever the child app changes, and the previous
	// generation stays loadable: its assets are retained and its drafts keep
	// validating, so an upgrade never strands an open editor.
	EngineBuildID    = "goview-5ab05b4-noyo21"
	UpstreamSHA      = "5ab05b439703ffda6d050e5626969459b3e3c1cf"
	MaxDocumentBytes = 5 << 20
	MaxComponents    = 200
	MaxBindings      = 1000
)

// LegacyEngineBuildIDs are adapter generations that shipped and may still be
// referenced by drafts and release snapshots.
var LegacyEngineBuildIDs = map[string]bool{"goview-5ab05b4-noyo1": true, "goview-5ab05b4-noyo2": true, "goview-5ab05b4-noyo3": true, "goview-5ab05b4-noyo4": true, "goview-5ab05b4-noyo5": true, "goview-5ab05b4-noyo6": true, "goview-5ab05b4-noyo7": true, "goview-5ab05b4-noyo8": true, "goview-5ab05b4-noyo9": true, "goview-5ab05b4-noyo10": true, "goview-5ab05b4-noyo11": true, "goview-5ab05b4-noyo12": true, "goview-5ab05b4-noyo13": true, "goview-5ab05b4-noyo14": true, "goview-5ab05b4-noyo15": true, "goview-5ab05b4-noyo16": true, "goview-5ab05b4-noyo17": true, "goview-5ab05b4-noyo18": true, "goview-5ab05b4-noyo19": true, "goview-5ab05b4-noyo20": true}

func knownEngineBuildID(id string) bool {
	return id == EngineBuildID || LegacyEngineBuildIDs[id]
}

type Engine struct {
	Name        string `json:"name"`
	BuildID     string `json:"buildId"`
	UpstreamSHA string `json:"upstreamSha"`
}

type Binding struct {
	Query            *dataquery.Plan        `json:"query,omitempty"`
	Dataset          *DatasetRef            `json:"dataset,omitempty"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Mapping          *DataMapping           `json:"mapping,omitempty"`
	Kind             string                 `json:"kind"`
	DeviceCode       string                 `json:"deviceCode,omitempty"`
	DeviceCodes      []string               `json:"deviceCodes,omitempty"`
	Key              string                 `json:"key,omitempty"`
	Keys             []string               `json:"keys,omitempty"`
	ValueType        string                 `json:"valueType,omitempty"`
	StaleAfterMs     int64                  `json:"staleAfterMs,omitempty"`
	Aggregate        string                 `json:"aggregate,omitempty"`
	MaxPoints        int                    `json:"maxPoints,omitempty"`
	WindowMs         int64                  `json:"windowMs,omitempty"`
	LatitudeKey      string                 `json:"latitudeKey,omitempty"`
	LongitudeKey     string                 `json:"longitudeKey,omitempty"`
	LocationSource   string                 `json:"locationSource,omitempty"`
	CoordinateSystem string                 `json:"coordinateSystem,omitempty"`
	Scale            *float64               `json:"scale,omitempty"`
	Offset           *float64               `json:"offset,omitempty"`
	Decimals         *int                   `json:"decimals,omitempty"`
	Unit             string                 `json:"unit,omitempty"`
}

type DatasetRef struct {
	ID      uint   `json:"id"`
	Version uint64 `json:"version"`
}
type DataMapping struct {
	Value    string   `json:"value,omitempty"`
	Category string   `json:"category,omitempty"`
	Series   string   `json:"series,omitempty"`
	Columns  []string `json:"columns,omitempty"`
}

type Target struct {
	ComponentID string `json:"componentId"`
	BindingID   string `json:"bindingId"`
	Target      string `json:"target"`
}

type Envelope struct {
	SchemaVersion string                 `json:"schemaVersion"`
	Engine        Engine                 `json:"engine"`
	Canvas        map[string]interface{} `json:"canvas,omitempty"`
	Content       map[string]interface{} `json:"content"`
	Bindings      map[string]Binding     `json:"bindings"`
	Targets       []Target               `json:"targets"`
	ThemeRefs     map[string]interface{} `json:"themeRefs"`
}

type ResourceRef struct {
	Type     string
	ID       string
	FieldKey string
}

var identifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:\-]{0,127}$`)
var htmlTag = regexp.MustCompile(`(?i)<\s*[a-z!/][^>]*>`)
var safeTargets = map[string]bool{"metric.value": true, "metric.status": true, "text.value": true, "chart.dataset": true, "table.rows": true, "map.points": true, "progress.value": true}

func DefaultDocument() Envelope {
	return Envelope{SchemaVersion: SchemaVersion, Engine: Engine{Name: "goview", BuildID: EngineBuildID, UpstreamSHA: UpstreamSHA}, Canvas: map[string]interface{}{"width": 1920, "height": 1080, "scaleMode": "contain"}, Content: map[string]interface{}{"editCanvasConfig": map[string]interface{}{}, "componentList": []interface{}{}, "requestGlobalConfig": map[string]interface{}{}}, Bindings: map[string]Binding{}, Targets: []Target{}, ThemeRefs: map[string]interface{}{}}
}

// Validate validates both the native content and the typed envelope. References
// are derived here, never accepted as a client-maintained authorization list.
func Validate(data []byte) (Envelope, []ResourceRef, error) {
	var doc Envelope
	if len(data) == 0 || len(data) > MaxDocumentBytes {
		return doc, nil, errors.New("document exceeds the 5 MiB limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return doc, nil, fmt.Errorf("invalid screen document: %w", err)
	}
	if decoder.Decode(new(interface{})) != io.EOF {
		return doc, nil, errors.New("document must contain one JSON value")
	}
	if doc.SchemaVersion != SchemaVersion || doc.Engine.Name != "goview" || !knownEngineBuildID(doc.Engine.BuildID) || doc.Engine.UpstreamSHA != UpstreamSHA {
		return doc, nil, errors.New("unsupported screen schema or engine build")
	}
	if doc.Content == nil || doc.Bindings == nil || doc.Targets == nil || doc.ThemeRefs == nil {
		return doc, nil, errors.New("content, bindings, targets and themeRefs are required")
	}
	if len(doc.Bindings) > MaxBindings {
		return doc, nil, errors.New("too many bindings")
	}
	list, ok := doc.Content["componentList"].([]interface{})
	if !ok {
		return doc, nil, errors.New("componentList must be an array")
	}
	ids := map[string]bool{}
	count := 0
	if err := validateComponents(list, ids, &count); err != nil {
		return doc, nil, err
	}
	refs := map[ResourceRef]bool{}
	if err := walkSafe(doc.Content, "content", 0, refs); err != nil {
		return doc, nil, err
	}
	if err := walkSafe(doc.Canvas, "canvas", 0, refs); err != nil {
		return doc, nil, err
	}
	for id, b := range doc.Bindings {
		if !identifier.MatchString(id) {
			return doc, nil, errors.New("invalid binding id")
		}
		if err := validateBinding(b, refs); err != nil {
			return doc, nil, fmt.Errorf("binding %s: %w", id, err)
		}
	}
	for _, target := range doc.Targets {
		if !ids[target.ComponentID] || !safeTargets[target.Target] {
			return doc, nil, errors.New("unknown component or unsupported binding target")
		}
		if _, ok := doc.Bindings[target.BindingID]; !ok {
			return doc, nil, errors.New("target references an unknown binding")
		}
	}
	if err := validateMapTargets(doc); err != nil {
		return doc, nil, err
	}
	if err := validateThemeRefs(doc.ThemeRefs); err != nil {
		return doc, nil, err
	}
	result := make([]ResourceRef, 0, len(refs))
	for ref := range refs {
		result = append(result, ref)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		return a.Type+"\x00"+a.ID+"\x00"+a.FieldKey < b.Type+"\x00"+b.ID+"\x00"+b.FieldKey
	})
	return doc, result, nil
}

func validateComponents(items []interface{}, ids map[string]bool, count *int) error {
	for _, item := range items {
		*count++
		if *count > MaxComponents {
			return errors.New("too many components")
		}
		c, ok := item.(map[string]interface{})
		if !ok {
			return errors.New("invalid component")
		}
		id, _ := c["id"].(string)
		if !identifier.MatchString(id) || ids[id] {
			return errors.New("component ids must be unique stable identifiers")
		}
		ids[id] = true
		if group, _ := c["isGroup"].(bool); group {
			children, ok := c["groupList"].([]interface{})
			if !ok {
				return errors.New("invalid component group")
			}
			if err := validateComponents(children, ids, count); err != nil {
				return err
			}
			continue
		}
		config, _ := c["chartConfig"].(map[string]interface{})
		key, _ := config["key"].(string)
		if !SafeWidgetKeys[key] {
			return fmt.Errorf("unsupported screen widget: %s", key)
		}
		if chartKey, _ := config["chartKey"].(string); chartKey != "V"+key {
			return errors.New("widget renderer does not match the reviewed key")
		}
		if conKey, _ := config["conKey"].(string); conKey != "VC"+key {
			return errors.New("widget configuration does not match the reviewed key")
		}
		if pkg, _ := config["package"].(string); pkg != widgetPackages[key] {
			return errors.New("widget package does not match the reviewed key")
		}
		if key == "LocationMap" {
			if err := validateMapOption(c["option"]); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateBinding(b Binding, refs map[ResourceRef]bool) error {
	if b.Kind == "dataset" {
		if (b.Query == nil) == (b.Dataset == nil) {
			return errors.New("Choose a query or a pinned dataset / 请选择查询或固定版本数据集")
		}
		if b.Query != nil {
			if err := dataquery.Validate(*b.Query); err != nil {
				return err
			}
		}
		if b.Dataset != nil {
			if b.Dataset.ID == 0 || b.Dataset.Version == 0 {
				return errors.New("Dataset version is required / 需要数据集版本")
			}
			refs[ResourceRef{Type: "dataset", ID: strconv.FormatUint(uint64(b.Dataset.ID), 10), FieldKey: strconv.FormatUint(b.Dataset.Version, 10)}] = true
		}
		if len(b.Parameters) > 16 || b.Mapping == nil || len(b.Mapping.Columns) > 64 {
			return errors.New("Invalid dataset mapping / 数据集映射无效")
		}
		for _, key := range append([]string{b.Mapping.Value, b.Mapping.Category, b.Mapping.Series}, b.Mapping.Columns...) {
			if key != "" && (!identifier.MatchString(key) || key == "constructor" || key == "prototype" || strings.Contains(key, "__proto__")) {
				return errors.New("Invalid dataset field / 数据集字段无效")
			}
		}
		return nil
	}
	switch b.Kind {
	case "point", "deviceStatus", "deviceSummary", "history", "aggregate", "location", "alarmList", "eventList":
	default:
		return errors.New("unsupported binding kind")
	}
	if b.StaleAfterMs < 0 || b.StaleAfterMs > 86400000 || b.WindowMs < 0 || b.WindowMs > 31*86400000 || b.MaxPoints < 0 || b.MaxPoints > 1000 {
		return errors.New("binding range exceeds limits")
	}
	if b.Decimals != nil && (*b.Decimals < 0 || *b.Decimals > 12) {
		return errors.New("invalid decimal precision")
	}
	if b.ValueType != "" && b.ValueType != "number" && b.ValueType != "string" && b.ValueType != "boolean" {
		return errors.New("invalid value type")
	}
	if b.Kind == "deviceSummary" {
		if !DeviceSummaryAggregate(b.Aggregate) || b.DeviceCode != "" || len(b.Keys) != 0 || b.Key != "" || b.WindowMs != 0 || b.Scale != nil || b.Offset != nil {
			return errors.New("invalid device summary definition")
		}
		seen := map[string]bool{}
		for _, code := range b.DeviceCodes {
			if seen[code] {
				return errors.New("duplicate device summary scope")
			}
			seen[code] = true
		}
	} else if b.Kind == "eventList" && (b.Key != "" || len(b.Keys) != 0 || b.Aggregate != "" || b.Scale != nil || b.Offset != nil) {
		return errors.New("event lists use declared events, not telemetry fields or aggregations")
	} else if b.Aggregate != "" && b.Aggregate != "avg" && b.Aggregate != "min" && b.Aggregate != "max" && b.Aggregate != "sum" && b.Aggregate != "count" && b.Aggregate != "median" {
		return errors.New("invalid aggregate")
	}
	devices := append([]string{}, b.DeviceCodes...)
	if b.DeviceCode != "" {
		devices = append(devices, b.DeviceCode)
	}
	if len(devices) == 0 || len(devices) > 200 {
		return errors.New("a bounded device set is required")
	}
	if b.Kind != "aggregate" && b.Kind != "deviceSummary" && b.Kind != "location" && b.Kind != "alarmList" && b.Kind != "eventList" && len(devices) != 1 {
		return errors.New("this binding kind requires exactly one device")
	}
	keys := append([]string{}, b.Keys...)
	if b.Key != "" {
		keys = append(keys, b.Key)
	}
	if b.Kind == "location" {
		switch b.LocationSource {
		case "device":
			if b.LatitudeKey != "" || b.LongitudeKey != "" {
				return errors.New("device location does not use telemetry keys")
			}
			if b.CoordinateSystem != "" && b.CoordinateSystem != "wgs84" {
				return errors.New("device location coordinates must be WGS84")
			}
		case "", "telemetry":
			if b.LatitudeKey == "" || b.LongitudeKey == "" {
				return errors.New("location keys are required")
			}
			if b.CoordinateSystem != "" && b.CoordinateSystem != "wgs84" && b.CoordinateSystem != "gcj02" {
				return errors.New("unsupported coordinate system")
			}
			keys = append(keys, b.LatitudeKey, b.LongitudeKey)
		default:
			return errors.New("unsupported location source")
		}
	} else if b.LocationSource != "" {
		return errors.New("location source requires a location binding")
	}
	if (b.Kind == "point" || b.Kind == "history" || b.Kind == "aggregate") && len(keys) == 0 {
		return errors.New("field keys are required")
	}
	if len(keys) > 64 {
		return errors.New("too many binding keys")
	}
	for _, device := range devices {
		if !identifier.MatchString(device) {
			return errors.New("invalid device code")
		}
		refs[ResourceRef{Type: "device", ID: device}] = true
		for _, key := range keys {
			if !identifier.MatchString(key) || strings.Contains(key, "__proto__") {
				return errors.New("invalid field key")
			}
			refs[ResourceRef{Type: "device", ID: device, FieldKey: key}] = true
		}
	}
	return nil
}

// DeviceSummaryAggregate is the shared allowlist for server-computed connection
// statistics. These count device status, never numeric telemetry samples.
func DeviceSummaryAggregate(value string) bool {
	switch value {
	case "total", "online", "offline", "unknown", "onlineRate":
		return true
	}
	return false
}

func emptyCapability(v interface{}) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(x) == ""
	case bool:
		return !x
	case float64:
		return x == 0
	case []interface{}:
		for _, v := range x {
			if !emptyCapability(v) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		for _, v := range x {
			if !emptyCapability(v) {
				return false
			}
		}
		return true
	}
	return false
}

func walkSafe(value interface{}, path string, depth int, refs map[ResourceRef]bool) error {
	if depth > 64 {
		return errors.New("document nesting exceeds limit")
	}
	switch x := value.(type) {
	case map[string]interface{}:
		widgetKey := ""
		if config, ok := x["chartConfig"].(map[string]interface{}); ok {
			widgetKey, _ = config["key"].(string)
		}
		for key, v := range x {
			k := strings.ToLower(key)
			next := path + "." + key
			if k == "__proto__" || k == "prototype" || k == "constructor" {
				return errors.New("prototype keys are forbidden")
			}
			if k == "filter" || k == "filterfunction" || k == "script" || k == "javascript" || k == "events" || k == "lifecycle" || k == "baseevent" || k == "advancedevents" || k == "requesturl" || k == "url" || k == "href" || k == "src" || k == "iframe" || k == "html" || k == "remoteurl" {
				if !emptyCapability(v) {
					return fmt.Errorf("executable or external capability is disabled: %s", next)
				}
			}
			if k == "requestdatatype" {
				if n, ok := v.(float64); !ok || n != 0 {
					return errors.New("only STATIC component data is supported")
				}
			}
			if k == "assetid" {
				s, ok := v.(string)
				if !ok || !identifier.MatchString(s) {
					return errors.New("invalid asset reference")
				}
				refs[ResourceRef{Type: "asset", ID: s}] = true
			}
			if k == "assetids" {
				ids, ok := v.([]interface{})
				if !ok || len(ids) > 200 {
					return errors.New("invalid asset references")
				}
				for _, id := range ids {
					s, ok := id.(string)
					if !ok || !identifier.MatchString(s) {
						return errors.New("invalid asset reference")
					}
					refs[ResourceRef{Type: "asset", ID: s}] = true
				}
			}
			if key == "option" && (widgetKey == "Iframe" || widgetKey == "Video") {
				if err := walkSafeMediaOption(v, next, depth+1, refs, widgetKey); err != nil {
					return err
				}
				continue
			}
			if err := walkSafe(v, next, depth+1, refs); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, v := range x {
			if err := walkSafe(v, path, depth+1, refs); err != nil {
				return err
			}
		}
	case string:
		lower := strings.ToLower(strings.TrimSpace(x))
		if strings.Contains(lower, "javascript:") || htmlTag.MatchString(lower) || strings.Contains(lower, "url(") || strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") || strings.HasPrefix(lower, "//") || strings.HasPrefix(lower, "data:") || strings.HasPrefix(lower, "blob:") {
			return fmt.Errorf("external or active content is disabled: %s", path)
		}
	}
	return nil
}

// Only the sandboxed media adapters consume these dedicated URL fields. The
// general URL, src, HTML, event and request prohibitions still apply elsewhere.
func walkSafeMediaOption(value interface{}, path string, depth int, refs map[ResourceRef]bool, widgetKey string) error {
	option, ok := value.(map[string]interface{})
	if !ok {
		return walkSafe(value, path, depth, refs)
	}
	field := "mediaUrl"
	if widgetKey == "Iframe" {
		field = "iframeUrl"
	}
	other := make(map[string]interface{}, len(option))
	for key, v := range option {
		if key != field {
			other[key] = v
			continue
		}
		s, ok := v.(string)
		if !ok || len(s) > 2048 {
			return errors.New("invalid media URL")
		}
		if s == "" {
			continue
		}
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || strings.ContainsAny(s, "\r\n\t<>") {
			return errors.New("media URL must be an absolute HTTP or HTTPS URL without credentials")
		}
	}
	return walkSafe(other, path, depth, refs)
}

func validateThemeRefs(v interface{}) error {
	switch x := v.(type) {
	case map[string]interface{}:
		for k, v := range x {
			if k == "__proto__" || k == "constructor" || k == "prototype" {
				return errors.New("invalid theme token")
			}
			if err := validateThemeRefs(v); err != nil {
				return err
			}
		}
	case []interface{}:
		for _, v := range x {
			if err := validateThemeRefs(v); err != nil {
				return err
			}
		}
	case string:
		if !identifier.MatchString(x) {
			return errors.New("theme references must be token identifiers")
		}
	default:
		return errors.New("invalid theme references")
	}
	return nil
}
