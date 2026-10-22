package screen

import "strings"

// Field semantics describe observable capabilities, not an inferred industry or
// safety assessment. Name-derived counter meanings remain explicitly heuristic.
func DescribeField(field ProfileField) ProfileField {
	if field.Role == "" {
		field.Role = ClassifyFieldRole(field.Key, field.Zh, field.En, field.Type, field.Unit)
	}
	field.SemanticKind, field.Provenance = "text", "type"
	field.Quantity = ""
	dataType := strings.ToLower(strings.TrimSpace(field.Type))
	switch {
	case field.Role == RoleLocation:
		field.SemanticKind, field.Provenance = "location", "name_heuristic"
	case len(field.Values) > 0 || dataType == "enum" || dataType == "bool" || dataType == "boolean":
		field.SemanticKind = "state"
	case isNumericType(field.Type):
		field.SemanticKind = "gauge"
		if field.Role == RoleCounter {
			field.SemanticKind, field.Provenance = "counter", "name_heuristic"
		}
	}
	unit := strings.ToLower(strings.TrimSpace(field.Unit))
	switch unit {
	case "°c", "℃", "°f", "℉", "k":
		field.Quantity = "temperature"
	case "pa", "kpa", "mpa", "bar", "mbar", "psi":
		field.Quantity = "pressure"
	case "w", "kw", "mw":
		field.Quantity = "power"
	case "wh", "kwh", "mwh", "j", "kj":
		field.Quantity = "energy"
	case "a", "ma", "ua", "μa":
		field.Quantity = "current"
	case "v", "mv", "kv":
		field.Quantity = "voltage"
	case "%", "%rh", "%lel", "ppm", "ppb":
		field.Quantity = "ratio_or_concentration"
	case "m³/h", "m3/h", "l/min", "l/s":
		field.Quantity = "flow"
	}
	field.Aggregations = nil
	if field.SemanticKind == "gauge" || field.SemanticKind == "counter" {
		field.Aggregations = []string{"avg", "min", "max", "count"}
		// Only known additive quantities or explicitly named count fields can
		// form totals. A pressure, temperature or percentage total is misleading.
		if field.Quantity == "power" || field.Quantity == "energy" || field.Quantity == "flow" ||
			field.Role == RoleAlarmCount || field.Role == RoleFaultCount || field.Role == RoleSupervisoryCount || field.Role == RoleShieldCount {
			field.Aggregations = append(field.Aggregations, "sum")
		}
	}
	return field
}

type ProfileOpportunity struct {
	ID    string   `json:"id"`
	Zh    string   `json:"zh"`
	En    string   `json:"en"`
	Kinds []string `json:"kinds"`
}

func profileOpportunities(products []ProfileProduct) []ProfileOpportunity {
	result := []ProfileOpportunity{{ID: "coverage", Zh: "哪些设备离线或状态未知？", En: "Which devices are offline or unknown?", Kinds: []string{"deviceSummary", "deviceStatus"}}}
	numeric, state, counter, location := false, false, false, false
	for _, product := range products {
		for _, field := range product.Fields {
			switch field.SemanticKind {
			case "gauge", "counter":
				numeric = true
				counter = counter || field.SemanticKind == "counter"
			case "state":
				state = true
			case "location":
				location = true
			}
		}
	}
	if numeric {
		result = append(result, ProfileOpportunity{ID: "conditions", Zh: "当前工况怎样，同类设备有何差异？", En: "What are current conditions and peer differences?", Kinds: []string{"point", "aggregate"}},
			ProfileOpportunity{ID: "change", Zh: "有效历史中发生了什么变化？", En: "What changed in valid historical observations?", Kinds: []string{"history"}})
	}
	if state {
		result = append(result, ProfileOpportunity{ID: "states", Zh: "设备报告了哪些业务状态？", En: "Which operational states are reported?", Kinds: []string{"point"}})
	}
	if counter {
		result = append(result, ProfileOpportunity{ID: "counters", Zh: "累计读数如何变化？", En: "How are cumulative readings changing?", Kinds: []string{"point", "history"}})
	}
	if location {
		result = append(result, ProfileOpportunity{ID: "location", Zh: "设备位于哪里？", En: "Where are the devices?", Kinds: []string{"location"}})
	}
	return result
}

// Keep different physical quantities represented when a wide product exceeds
// the prompt budget. Preserve existing deterministic order within the selection.
func diverseProfileFields(fields []ProfileField, limit int) []ProfileField {
	if len(fields) <= limit {
		return fields
	}
	chosen, groups := map[int]bool{}, map[string]bool{}
	for index, field := range fields {
		group := field.SemanticKind + ":" + field.Quantity
		if field.Quantity == "" {
			group += ":" + field.Role
		}
		if !groups[group] && len(chosen) < limit {
			groups[group], chosen[index] = true, true
		}
	}
	for index := range fields {
		if len(chosen) >= limit {
			break
		}
		chosen[index] = true
	}
	result := make([]ProfileField, 0, limit)
	for index, field := range fields {
		if chosen[index] {
			result = append(result, field)
		}
	}
	return result
}
