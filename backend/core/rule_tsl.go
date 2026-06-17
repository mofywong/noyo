package core

import (
	"encoding/json"
	"fmt"
)

type RuleTSLOptions struct {
	Properties []RulePropertyOption `json:"properties"`
	Events     []RuleEventOption    `json:"events"`
	Services   []RuleServiceOption  `json:"services"`
}

type RulePropertyOption struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	DataType   string `json:"data_type"`
	AccessMode string `json:"access_mode"`
	Specs      any    `json:"specs,omitempty"`
}

type RuleEventOption struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type RuleServiceOption struct {
	Key      string `json:"key"`
	Name     string `json:"name"`
	CallType string `json:"call_type"`
}

func ParseProductTSL(configText string) (RuleTSLOptions, error) {
	if configText == "" {
		return RuleTSLOptions{}, nil
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(configText), &config); err != nil {
		return RuleTSLOptions{}, fmt.Errorf("invalid product config json: %w", err)
	}
	tsl := config
	if rawTSL, ok := config["tsl"].(map[string]any); ok {
		tsl = rawTSL
	}
	return RuleTSLOptions{
		Properties: extractRuleProperties(tsl["properties"]),
		Events:     extractRuleEvents(tsl["events"]),
		Services:   extractRuleServices(tsl["services"]),
	}, nil
}

func extractRuleProperties(raw any) []RulePropertyOption {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]RulePropertyOption, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		dataType, _ := m["dataType"].(map[string]any)
		out = append(out, RulePropertyOption{
			Key:        stringValue(m["identifier"]),
			Name:       stringValue(m["name"]),
			DataType:   stringValue(dataType["type"]),
			AccessMode: stringValue(m["accessMode"]),
			Specs:      dataType["specs"],
		})
	}
	return out
}

func extractRuleEvents(raw any) []RuleEventOption {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]RuleEventOption, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, RuleEventOption{
			Key:  stringValue(m["identifier"]),
			Name: stringValue(m["name"]),
			Type: stringValue(m["type"]),
		})
	}
	return out
}

func extractRuleServices(raw any) []RuleServiceOption {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]RuleServiceOption, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, RuleServiceOption{
			Key:      stringValue(m["identifier"]),
			Name:     stringValue(m["name"]),
			CallType: stringValue(m["callType"]),
		})
	}
	return out
}

func stringValue(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}
