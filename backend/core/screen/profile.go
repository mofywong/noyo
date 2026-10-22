package screen

import (
	"sort"
	"strings"
)

// Scene profiles exist because the raw device catalogue is the wrong shape for a
// model: 300 devices x 40 properties is mostly repeated product metadata, and the
// repetition buries the handful of properties that can actually carry a metric.
// A profile is the same information folded by product and annotated with what
// each property is *for*, which raises the semantic density of the prompt by an
// order of magnitude at a fraction of the size.
const (
	RoleAlarmCount       = "alarmCount"
	RoleFaultCount       = "faultCount"
	RoleSupervisoryCount = "supervisoryCount"
	RoleShieldCount      = "shieldCount"
	RolePercent          = "percent"
	RoleMetric           = "metric"
	RoleStatus           = "status"
	RoleCounter          = "counter"
	RoleLocation         = "location"
	RoleIgnore           = "ignore"
)

// Lower sorts first: a model building an operations screen wants the alarm
// counters at the top of the list, not channel identifiers.
var rolePriority = map[string]int{
	RoleAlarmCount: 1, RoleFaultCount: 2, RoleSupervisoryCount: 3, RoleShieldCount: 4,
	RolePercent: 5, RoleMetric: 6, RoleStatus: 7, RoleCounter: 8, RoleLocation: 9, RoleIgnore: 90,
}

type ProfileField struct {
	Key          string            `json:"key"`
	Zh           string            `json:"zh,omitempty"`
	En           string            `json:"en,omitempty"`
	Role         string            `json:"role"`
	Type         string            `json:"type,omitempty"`
	Unit         string            `json:"unit,omitempty"`
	Values       map[string]string `json:"values,omitempty"`
	SemanticKind string            `json:"semanticKind,omitempty"`
	Quantity     string            `json:"quantity,omitempty"`
	Provenance   string            `json:"provenance,omitempty"`
	Aggregations []string          `json:"aggregations,omitempty"`
}

type ProfileProduct struct {
	Code              string         `json:"code"`
	Name              string         `json:"name,omitempty"`
	Devices           int            `json:"devices"`
	Fields            []ProfileField `json:"fields"`
	Events            []ProfileEvent `json:"events,omitempty"`
	FieldCount        int            `json:"fieldCount,omitempty"`
	FieldsOmitted     int            `json:"fieldsOmitted,omitempty"`
	DefinitionVersion int64          `json:"definitionVersion,omitempty"`
}

// Events describe declared capabilities, not observed occurrences or counters.
type ProfileEvent struct {
	Key    string         `json:"key"`
	Zh     string         `json:"zh,omitempty"`
	En     string         `json:"en,omitempty"`
	Type   string         `json:"type,omitempty"`
	Fields []ProfileField `json:"fields,omitempty"`
}

// ProfileDevice keeps only the identifiers a binding needs. Property metadata
// lives on the product so it is sent once instead of once per device.
type ProfileDevice struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Product string `json:"productCode"`
	State   string `json:"state"`
}

type SceneProfile struct {
	Products      []ProfileProduct     `json:"products"`
	Devices       []ProfileDevice      `json:"devices"`
	Online        int                  `json:"online"`
	Offline       int                  `json:"offline"`
	Unknown       int                  `json:"unknown"`
	Opportunities []ProfileOpportunity `json:"opportunities"`
}

// ProfileInput is the per-device slice of the catalogue the profile builder
// needs; the API layer assembles it from the readable device set.
type ProfileInput struct {
	Code        string
	Name        string
	ProductCode string
	ProductName string
	State       string
	Fields      []ProfileField
}

func fieldRoleText(key, zh, en string) string {
	return strings.ToLower(strings.Join([]string{key, zh, en}, " "))
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func isNumericType(dataType string) bool {
	switch strings.ToLower(strings.TrimSpace(dataType)) {
	case "int", "int32", "int64", "float", "float32", "float64", "double", "number", "integer", "long", "short":
		return true
	}
	return false
}

// ClassifyFieldRole exposes the classifier so the field browser and the scene
// profile agree on which properties are worth showing. One rule set, two callers.
func ClassifyFieldRole(key, zh, en, dataType, unit string) string {
	return classifyFieldRole(key, zh, en, dataType, unit)
}

// channelIdentifier matches keys that only name a wiring channel. They are real
// properties but carry no operational meaning, and listing them trains the model
// to bind a screen tile to an arbitrary channel.
var channelNameParts = []string{"aisle", "channel", "chan", "port", "slot"}

func classifyFieldRole(key, zh, en, dataType, unit string) string {
	lowerKey := strings.ToLower(strings.TrimSpace(key))
	text := fieldRoleText(key, zh, en)
	if lowerKey == "" {
		return RoleIgnore
	}
	if containsAny(text, []string{"latitude", "longitude", "经度", "纬度"}) {
		return RoleLocation
	}
	// Alarm-family counters come first: they are the metrics an operations screen
	// is actually built from, and they share wording with less useful fields.
	// Supervisory and shield signals are checked before the alarm family: their
	// Chinese labels ("监管报警", "屏蔽") also contain the word for alarm, and the
	// more specific classification is the correct one.
	if containsAny(text, []string{"supervisory", "监管"}) {
		return RoleSupervisoryCount
	}
	if containsAny(text, []string{"shield", "bypass", "屏蔽"}) {
		return RoleShieldCount
	}
	if containsAny(text, []string{"alarm", "火警", "报警", "fire"}) {
		if containsAny(text, []string{"count", "total", "num", "数量", "总数", "数"}) {
			return RoleAlarmCount
		}
	}
	if containsAny(text, []string{"fault", "error", "故障"}) {
		if containsAny(text, []string{"count", "total", "num", "数量", "总数", "数"}) {
			return RoleFaultCount
		}
	}
	if strings.TrimSpace(unit) == "%" || containsAny(text, []string{"percent", "ratio", "百分比", "占比", "浓度"}) {
		return RolePercent
	}
	lowerType := strings.ToLower(strings.TrimSpace(dataType))
	if lowerType == "enum" || lowerType == "bool" || lowerType == "boolean" {
		return RoleStatus
	}
	if containsAny(text, []string{"total", "accumulate", "cumulative", "累计", "总量", "电度"}) {
		return RoleCounter
	}
	if isNumericType(lowerType) {
		// A bare channel identifier with no unit and no translated name is noise.
		translated := strings.TrimSpace(zh) != "" && zh != key
		if strings.TrimSpace(unit) == "" && !translated && containsAny(lowerKey, channelNameParts) {
			return RoleIgnore
		}
		return RoleMetric
	}
	if containsAny(lowerKey, channelNameParts) && strings.TrimSpace(zh) == key {
		return RoleIgnore
	}
	return RoleMetric
}

// BuildSceneProfile folds the readable device set by product. Ordering is fully
// deterministic (products by device count then code, fields by role priority then
// key) so two runs over the same project produce the same prompt.
func BuildSceneProfile(devices []ProfileInput, maxFieldsPerProduct int) SceneProfile {
	if maxFieldsPerProduct <= 0 {
		maxFieldsPerProduct = 12
	}
	profile := SceneProfile{Products: []ProfileProduct{}, Devices: []ProfileDevice{}}
	type bucket struct {
		name    string
		devices int
		fields  map[string]ProfileField
		order   []string
	}
	byProduct := map[string]*bucket{}
	for _, device := range devices {
		state := device.State
		switch state {
		case "online":
			profile.Online++
		case "offline":
			profile.Offline++
		default:
			state = "unknown"
			profile.Unknown++
		}
		profile.Devices = append(profile.Devices, ProfileDevice{Code: device.Code, Name: device.Name, Product: device.ProductCode, State: state})
		entry := byProduct[device.ProductCode]
		if entry == nil {
			entry = &bucket{name: device.ProductName, fields: map[string]ProfileField{}}
			byProduct[device.ProductCode] = entry
		}
		entry.devices++
		for _, field := range device.Fields {
			role := classifyFieldRole(field.Key, field.Zh, field.En, field.Type, field.Unit)
			if role == RoleIgnore {
				continue
			}
			if _, exists := entry.fields[field.Key]; exists {
				continue
			}
			field.Role = role
			field = DescribeField(field)
			entry.fields[field.Key] = field
			entry.order = append(entry.order, field.Key)
		}
	}
	codes := make([]string, 0, len(byProduct))
	for code := range byProduct {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		left, right := byProduct[codes[i]], byProduct[codes[j]]
		if left.devices != right.devices {
			return left.devices > right.devices
		}
		return codes[i] < codes[j]
	})
	for _, code := range codes {
		entry := byProduct[code]
		fields := make([]ProfileField, 0, len(entry.order))
		for _, key := range entry.order {
			fields = append(fields, entry.fields[key])
		}
		sort.SliceStable(fields, func(i, j int) bool {
			left, right := rolePriority[fields[i].Role], rolePriority[fields[j].Role]
			if left != right {
				return left < right
			}
			return fields[i].Key < fields[j].Key
		})
		fieldCount := len(fields)
		fields = diverseProfileFields(fields, maxFieldsPerProduct)
		profile.Products = append(profile.Products, ProfileProduct{Code: code, Name: entry.name, Devices: entry.devices, Fields: fields, FieldCount: fieldCount, FieldsOmitted: fieldCount - len(fields)})
	}
	profile.Opportunities = profileOpportunities(profile.Products)
	return profile
}
