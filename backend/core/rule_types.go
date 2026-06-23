package core

import (
	"encoding/json"
	"errors"
	"fmt"
)

const (
	RuleScopePlatform = "platform"
	RuleScopeGateway  = "gateway"

	RuleStatusDraft    = "draft"
	RuleStatusEnabled  = "enabled"
	RuleStatusDisabled = "disabled"
	RuleStatusError    = "error"

	RuleTriggerProperty     = "property_change"
	RuleTriggerEvent        = "event"
	RuleTriggerDeviceStatus = "device_status"
	RuleTriggerCron         = "cron"

	RuleActionSetProperty   = "set_property"
	RuleActionCallService   = "call_service"
	RuleActionNotification  = "notification"
	RuleActionAlarm         = "alarm"
	RuleActionDelay         = "delay"
	RuleActionLLM           = "llm"
	RuleActionVoicePlayback = "voice_playback"
	RuleActionParallelGroup = "parallel_group"
	RuleActionSequenceGroup = "sequence_group"

	RuleEffectiveDaily   = "daily"
	RuleEffectiveWeekly  = "weekly"
	RuleEffectiveMonthly = "monthly"
	RuleEffectiveWorkday = "workday"
	RuleEffectiveHoliday = "holiday"
	RuleEffectiveCustom  = "custom"
	RuleEffectiveAlways  = "always"
)

type RuleTrigger struct {
	ID          string              `json:"id"`
	Type        string              `json:"type"`
	DeviceCode  string              `json:"deviceCode,omitempty"`
	DeviceName  string              `json:"deviceName,omitempty"`
	ProductCode string              `json:"productCode,omitempty"`
	PropertyKey string              `json:"propertyKey,omitempty"`
	Operator    string              `json:"operator,omitempty"`
	Value       any                 `json:"value,omitempty"`
	EventID     string              `json:"eventId,omitempty"`
	EventFilter []PropertyCondition `json:"eventFilter,omitempty"`
	StatusValue string              `json:"statusValue,omitempty"`
	CronExpr    string              `json:"cronExpr,omitempty"`
	CronDesc    string              `json:"cronDesc,omitempty"`
}

type RuleConditionGroup struct {
	Logic      string               `json:"logic"`
	Conditions []RuleCondition      `json:"conditions,omitempty"`
	Groups     []RuleConditionGroup `json:"groups,omitempty"`
}

type RuleCondition struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	DeviceCode  string `json:"deviceCode,omitempty"`
	DeviceName  string `json:"deviceName,omitempty"`
	PropertyKey string `json:"propertyKey,omitempty"`
	Operator    string `json:"operator,omitempty"`
	Value       any    `json:"value,omitempty"`
	StatusValue string `json:"statusValue,omitempty"`
	StartTime   string `json:"startTime,omitempty"`
	EndTime     string `json:"endTime,omitempty"`
	Weekdays    []int  `json:"weekdays,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
}

type PropertyCondition struct {
	Key      string `json:"key"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

type RuleAction struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	SubActions    []RuleAction   `json:"subActions,omitempty"`
	DeviceCode    string         `json:"deviceCode,omitempty"`
	DeviceName    string         `json:"deviceName,omitempty"`
	PropertyKey   string         `json:"propertyKey,omitempty"`
	Value         any            `json:"value,omitempty"`
	ServiceCode   string         `json:"serviceCode,omitempty"`
	ServiceParams map[string]any `json:"serviceParams,omitempty"`
	NotifyTitle   string         `json:"notifyTitle,omitempty"`
	NotifyContent string         `json:"notifyContent,omitempty"`
	AlarmLevel    string         `json:"alarmLevel,omitempty"`
	AlarmTitle    string         `json:"alarmTitle,omitempty"`
	AlarmContent  string         `json:"alarmContent,omitempty"`
	AlarmDevice   string         `json:"alarmDevice,omitempty"`
	DelaySec          int            `json:"delaySec,omitempty"`
	LLMPrompt         string         `json:"llmPrompt,omitempty"`
	LLMPlayAudio      bool           `json:"llmPlayAudio,omitempty"`
	LLMIncludeContext bool           `json:"llmIncludeContext,omitempty"`
	OutputSchema      map[string]any `json:"outputSchema,omitempty"`
	VoiceText         string         `json:"voiceText,omitempty"`
}

type RuleEffectiveTime struct {
	Mode      string                `json:"mode,omitempty"`
	StartTime string                `json:"startTime,omitempty"`
	EndTime   string                `json:"endTime,omitempty"`
	Windows   []RuleEffectiveWindow `json:"windows,omitempty"`
	Weekdays  []int                 `json:"weekdays,omitempty"`
	MonthDays []int                 `json:"monthDays,omitempty"`
	Months    []int                 `json:"months,omitempty"`
	Timezone  string                `json:"timezone,omitempty"`
}

type RuleEffectiveWindow struct {
	MonthDays []int  `json:"monthDays,omitempty"`
	StartTime string `json:"startTime,omitempty"`
	EndTime   string `json:"endTime,omitempty"`
}

type RuleDefinition struct {
	Triggers      []RuleTrigger       `json:"triggers"`
	Conditions    *RuleConditionGroup `json:"conditions,omitempty"`
	Actions       []RuleAction        `json:"actions"`
	EffectiveTime *RuleEffectiveTime  `json:"effective_time,omitempty"`
}

type RuleRuntime struct {
	Code          string
	Name          string
	Version       int64
	Triggers      []RuleTrigger
	Conditions    *RuleConditionGroup
	Actions       []RuleAction
	EffectiveTime *RuleEffectiveTime
}

func DecodeRuleDefinition(triggersJSON, conditionsJSON, actionsJSON string) (RuleDefinition, error) {
	var def RuleDefinition
	if err := json.Unmarshal([]byte(triggersJSON), &def.Triggers); err != nil {
		return def, fmt.Errorf("invalid triggers: %w", err)
	}
	if conditionsJSON != "" {
		var group RuleConditionGroup
		if err := json.Unmarshal([]byte(conditionsJSON), &group); err != nil {
			return def, fmt.Errorf("invalid conditions: %w", err)
		}
		def.Conditions = &group
	}
	if err := json.Unmarshal([]byte(actionsJSON), &def.Actions); err != nil {
		return def, fmt.Errorf("invalid actions: %w", err)
	}
	return def, ValidateRuleDefinition(def)
}

func ValidateRuleDefinition(def RuleDefinition) error {
	if len(def.Triggers) == 0 {
		return errors.New("at least one trigger is required")
	}
	if len(def.Actions) == 0 {
		return errors.New("at least one action is required")
	}
	if def.Conditions != nil {
		if err := validateConditionGroup(*def.Conditions, 1); err != nil {
			return err
		}
	}
	for _, action := range def.Actions {
		if err := validateAction(action, 1); err != nil {
			return err
		}
	}
	return nil
}

func validateConditionGroup(group RuleConditionGroup, depth int) error {
	if group.Logic != "and" && group.Logic != "or" {
		return fmt.Errorf("condition group logic must be and or or, got %q", group.Logic)
	}
	if depth > 2 {
		return errors.New("condition groups support at most two nested levels")
	}
	for _, nested := range group.Groups {
		if err := validateConditionGroup(nested, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func validateAction(action RuleAction, depth int) error {
	if action.ID == "" {
		return errors.New("action id is required")
	}
	switch action.Type {
	case RuleActionSetProperty, RuleActionCallService, RuleActionNotification, RuleActionAlarm, RuleActionLLM, RuleActionVoicePlayback:
		return nil
	case RuleActionDelay:
		if action.DelaySec < 0 || action.DelaySec > 300 {
			return fmt.Errorf("delaySec must be between 0 and 300, got %d", action.DelaySec)
		}
	case RuleActionParallelGroup, RuleActionSequenceGroup:
		if depth > 5 {
			return errors.New("action groups support at most five nested levels")
		}
		if len(action.SubActions) == 0 {
			return errors.New("action group must contain subActions")
		}
		if len(action.SubActions) > 20 {
			return errors.New("action group cannot contain more than 20 subActions")
		}
		for _, sub := range action.SubActions {
			if err := validateAction(sub, depth+1); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported action type %q", action.Type)
	}
	return nil
}


