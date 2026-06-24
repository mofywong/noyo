package core

import (
	"encoding/json"
	"noyo/core/store"
	"sort"
)

type RuleDistributor struct {
	registry *DeviceRegistry
}

func NewRuleDistributor(registry *DeviceRegistry) *RuleDistributor {
	return &RuleDistributor{registry: registry}
}

func (rd *RuleDistributor) AnalyzeScope(rule *store.Rule) (string, string) {
	if rule == nil || rd == nil || rd.registry == nil {
		return RuleScopePlatform, ""
	}
	def, err := DecodeRuleDefinition(rule.Triggers, rule.Conditions, rule.Actions)
	if err != nil {
		return RuleScopePlatform, ""
	}
	if hasNonDeviceAction(def.Actions) {
		return RuleScopePlatform, ""
	}
	deviceCodes := collectRuleDeviceCodes(def)
	if len(deviceCodes) == 0 {
		return RuleScopePlatform, ""
	}
	gateways := map[string]bool{}
	for _, code := range deviceCodes {
		device, ok := rd.registry.GetDevice(code)
		if !ok || device == nil {
			return RuleScopePlatform, ""
		}
		gatewaySN := rd.getDeviceGateway(device)
		if gatewaySN == "" {
			return RuleScopePlatform, ""
		}
		gateways[gatewaySN] = true
	}
	if len(gateways) == 1 {
		for sn := range gateways {
			return RuleScopeGateway, sn
		}
	}
	return RuleScopePlatform, ""
}

func (rd *RuleDistributor) getDeviceGateway(device *store.Device) string {
	if device == nil {
		return ""
	}
	if device.ParentCode == "" {
		product, ok := rd.registry.GetProduct(device.ProductCode)
		if ok && product.ProtocolName == "cascade" {
			return device.Code
		}
		return ""
	}
	current := device
	for current.ParentCode != "" {
		parent, ok := rd.registry.GetDevice(current.ParentCode)
		if !ok || parent == nil {
			return ""
		}
		product, ok := rd.registry.GetProduct(parent.ProductCode)
		if ok && product.ProtocolName == "cascade" {
			return parent.Code
		}
		current = parent
	}
	return ""
}

func hasNonDeviceAction(actions []RuleAction) bool {
	for _, action := range actions {
		switch action.Type {
		case RuleActionSetProperty, RuleActionCallService:
			continue
		case RuleActionParallelGroup:
			if hasNonDeviceAction(action.SubActions) {
				return true
			}
		case RuleActionSequenceGroup:
			if hasNonDeviceAction(action.SubActions) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func collectRuleDeviceCodes(def RuleDefinition) []string {
	seen := map[string]bool{}
	for _, trigger := range def.Triggers {
		addDeviceCode(seen, trigger.DeviceCode)
	}
	if def.Conditions != nil {
		collectConditionDeviceCodes(seen, *def.Conditions)
	}
	collectActionDeviceCodes(seen, def.Actions)
	codes := make([]string, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func collectConditionDeviceCodes(seen map[string]bool, group RuleConditionGroup) {
	for _, condition := range group.Conditions {
		addDeviceCode(seen, condition.DeviceCode)
	}
	for _, nested := range group.Groups {
		collectConditionDeviceCodes(seen, nested)
	}
}

func collectActionDeviceCodes(seen map[string]bool, actions []RuleAction) {
	for _, action := range actions {
		addDeviceCode(seen, action.DeviceCode)
		if action.AlarmDevice != "" && action.AlarmDevice != "trigger" {
			addDeviceCode(seen, action.AlarmDevice)
		}
		if len(action.SubActions) > 0 {
			collectActionDeviceCodes(seen, action.SubActions)
		}
	}
}

func addDeviceCode(seen map[string]bool, code string) {
	if code != "" {
		seen[code] = true
	}
}

func RuleDefinitionFromStore(rule *store.Rule) (RuleDefinition, error) {
	def, err := DecodeRuleDefinition(rule.Triggers, rule.Conditions, rule.Actions)
	if err != nil {
		return def, err
	}
	if rule.EffectiveTime != "" {
		var effective RuleEffectiveTime
		if err := json.Unmarshal([]byte(rule.EffectiveTime), &effective); err != nil {
			return def, err
		}
		def.EffectiveTime = &effective
	}
	return def, nil
}

func EncodeRulePart(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
