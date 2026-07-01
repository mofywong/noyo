package core

import (
	"context"
	"encoding/json"
	"fmt"
	"noyo/core/types"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

var varRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

func interpolateString(text string, vars map[string]any) string {
	if text == "" {
		return text
	}
	return varRegex.ReplaceAllStringFunc(text, func(match string) string {
		path := match[2 : len(match)-1]
		val := getValueByPath(vars, path)
		if val != nil {
			return formatTemplateValue(val)
		}
		return match
	})
}

func formatTemplateValue(val any) string {
	switch v := val.(type) {
	case bool:
		if v {
			return "成功"
		}
		return "失败"
	default:
		return fmt.Sprintf("%v", val)
	}
}

func getValueByPath(data map[string]any, path string) any {
	parts := strings.Split(path, ".")
	var current any = data
	for _, part := range parts {
		next, ok := getPathPart(current, part)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func getPathPart(current any, part string) (any, bool) {
	if current == nil {
		return nil, false
	}
	if m, ok := current.(map[string]any); ok {
		v, exists := m[part]
		return v, exists
	}

	value := reflect.ValueOf(current)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Slice, reflect.Array:
		index, err := strconv.Atoi(part)
		if err != nil || index < 0 || index >= value.Len() {
			return nil, false
		}
		return value.Index(index).Interface(), true
	case reflect.Struct:
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if field.PkgPath != "" {
				continue
			}
			jsonName := strings.Split(field.Tag.Get("json"), ",")[0]
			if jsonName == "" {
				jsonName = field.Name
			}
			if jsonName == part || strings.EqualFold(field.Name, part) {
				return value.Field(i).Interface(), true
			}
		}
	}
	return nil, false
}

type ActionResult struct {
	ActionID   string `json:"actionId"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

type RuleExecContext struct {
	Context       context.Context
	Rule          RuleRuntime
	TemplateVars  map[string]any
	NodeResults   map[string]any
	NodeResultsMu sync.RWMutex
	SessionID     string
	ActionTimeout time.Duration
	RuleTimeout   time.Duration
	MaxParallel   int
}

type ActionExecutor struct {
	deviceManager *DeviceManager
	logger        *zap.Logger
}

func NewActionExecutor(deviceManager *DeviceManager, logger *zap.Logger) *ActionExecutor {
	return &ActionExecutor{deviceManager: deviceManager, logger: logger}
}

func (ae *ActionExecutor) Execute(ctx *RuleExecContext) []ActionResult {
	if ctx.Context == nil {
		ctx.Context = context.Background()
	}
	if ctx.RuleTimeout <= 0 {
		ctx.RuleTimeout = 60 * time.Second
	}
	if ctx.ActionTimeout <= 0 {
		ctx.ActionTimeout = 10 * time.Second
	}
	if ctx.MaxParallel <= 0 {
		ctx.MaxParallel = 20
	}
	execCtx, cancel := context.WithTimeout(ctx.Context, ctx.RuleTimeout)
	defer cancel()

	return ae.executeActionList(execCtx, ctx, ctx.Rule.Actions, true)
}

func (ae *ActionExecutor) executeActionList(execCtx context.Context, ctx *RuleExecContext, actions []RuleAction, inferVoiceOutput bool) []ActionResult {
	results := make([]ActionResult, 0, len(actions))
	for i, action := range actions {
		if execCtx.Err() != nil {
			results = append(results, ActionResult{ActionID: action.ID, Type: action.Type, Status: "skipped", Error: execCtx.Err().Error()})
			continue
		}
		outputMode := ""
		if inferVoiceOutput && action.Type == RuleActionLLM && nextActionUsesLLMResult(actions, i) {
			outputMode = "voice"
		}
		results = append(results, ae.executeAction(execCtx, ctx, action, outputMode)...)
	}
	return results
}

func nextActionUsesLLMResult(actions []RuleAction, index int) bool {
	if index < 0 || index+1 >= len(actions) {
		return false
	}
	return actionUsesLLMResultForVoice(actions[index+1])
}

func actionUsesLLMResultForVoice(action RuleAction) bool {
	switch action.Type {
	case RuleActionVoicePlayback:
		return action.VoiceText == "" || strings.Contains(action.VoiceText, "${llm_result}")
	case RuleActionParallelGroup, RuleActionSequenceGroup:
		for _, subAction := range action.SubActions {
			if actionUsesLLMResultForVoice(subAction) {
				return true
			}
		}
	}
	return false
}

func (ae *ActionExecutor) executeAction(execCtx context.Context, ctx *RuleExecContext, action RuleAction, llmOutputMode string) []ActionResult {
	switch action.Type {
	case RuleActionParallelGroup:
		return ae.executeParallelGroup(execCtx, ctx, action)
	case RuleActionSequenceGroup:
		return ae.executeSequenceGroup(execCtx, ctx, action)
	default:
		return []ActionResult{ae.executeSingleAction(execCtx, ctx, action, llmOutputMode)}
	}
}

func (ae *ActionExecutor) executeSequenceGroup(execCtx context.Context, ctx *RuleExecContext, group RuleAction) []ActionResult {
	return ae.executeActionList(execCtx, ctx, group.SubActions, true)
}

func (ae *ActionExecutor) executeParallelGroup(execCtx context.Context, ctx *RuleExecContext, group RuleAction) []ActionResult {
	maxParallel := ctx.MaxParallel
	if maxParallel <= 0 {
		maxParallel = 20
	}
	results := make([][]ActionResult, len(group.SubActions))
	sem := make(chan struct{}, maxParallel)
	var wg sync.WaitGroup
	for i, action := range group.SubActions {
		wg.Add(1)
		go func(idx int, act RuleAction) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-execCtx.Done():
				results[idx] = []ActionResult{{ActionID: act.ID, Type: act.Type, Status: "skipped", Error: execCtx.Err().Error()}}
				return
			}
			results[idx] = ae.executeAction(execCtx, ctx, act, "")
		}(i, action)
	}
	wg.Wait()
	flat := make([]ActionResult, 0, len(group.SubActions))
	for _, groupResults := range results {
		flat = append(flat, groupResults...)
	}
	return flat
}

func (ae *ActionExecutor) executeSingleAction(execCtx context.Context, ctx *RuleExecContext, action RuleAction, llmOutputMode string) ActionResult {
	start := time.Now()
	result := ActionResult{ActionID: action.ID, Type: action.Type, Status: "success"}
	actionCtx, cancel := context.WithTimeout(execCtx, ctx.ActionTimeout)
	defer cancel()

	var err error
	var actionOutput any

	mergedVars := make(map[string]any)
	if ctx.TemplateVars != nil {
		for k, v := range ctx.TemplateVars {
			mergedVars[k] = v
		}
	}
	nodeResults := snapshotNodeResults(ctx)
	if nodeResults != nil {
		mergedVars["node"] = nodeResults
		// Keep flat mapping for backward compatibility if needed, but prefer 'node' object
		for k, v := range nodeResults {
			mergedVars[k] = v
		}
	}

	action.NotifyTitle = interpolateString(action.NotifyTitle, mergedVars)
	action.NotifyContent = interpolateString(action.NotifyContent, mergedVars)
	action.AlarmTitle = interpolateString(action.AlarmTitle, mergedVars)
	action.AlarmContent = interpolateString(action.AlarmContent, mergedVars)
	action.LLMPrompt = interpolateString(action.LLMPrompt, mergedVars)
	action.VoiceText = interpolateString(action.VoiceText, mergedVars)

	switch action.Type {
	case RuleActionDelay:
		if action.DelaySec > 0 {
			timer := time.NewTimer(time.Duration(action.DelaySec) * time.Second)
			select {
			case <-timer.C:
			case <-actionCtx.Done():
				timer.Stop()
				err = actionCtx.Err()
			}
		}
	case RuleActionSetProperty:
		if ae.deviceManager == nil {
			break
		}
		value, normalizeErr := normalizeRuleActionValue(ae.deviceManager, action.DeviceCode, action.PropertyKey, action.Value)
		if normalizeErr != nil {
			err = normalizeErr
			break
		}
		err = ae.deviceManager.SetDeviceProperties(action.DeviceCode, map[string]any{action.PropertyKey: value})
	case RuleActionCallService:
		if ae.deviceManager == nil {
			break
		}
		_, err = ae.deviceManager.CallDeviceService(action.DeviceCode, action.ServiceCode, action.ServiceParams)
	case RuleActionNotification:
		if ae.deviceManager != nil && ae.deviceManager.EventBus != nil {
			ae.deviceManager.EventBus.Publish(types.Event{
				Type:  types.EventType("rule.notification"),
				Topic: ctx.Rule.Code,
				Payload: map[string]any{
					"title":   action.NotifyTitle,
					"content": action.NotifyContent,
				},
				Timestamp: time.Now().UnixMilli(),
			})
		}
	case RuleActionAlarm:
		if ae.deviceManager == nil {
			break
		}
		deviceCode := action.AlarmDevice
		if deviceCode == "" || deviceCode == "trigger" {
			if event, ok := ctx.TemplateVars["event"].(types.Event); ok {
				deviceCode = event.Topic
			}
		}
		if deviceCode == "" {
			err = fmt.Errorf("alarm action requires a device")
			break
		}
		meta, _, metaErr := ae.deviceManager.GetDeviceMeta(deviceCode)
		if metaErr != nil {
			err = metaErr
			break
		}
		alarmPayload := map[string]interface{}{
			"rule_code":  ctx.Rule.Code,
			"rule_name":  ctx.Rule.Name,
			"title":      action.AlarmTitle,
			"content":    action.AlarmContent,
			"level":      action.AlarmLevel,
			"deviceCode": deviceCode,
			"eventId":    "rule_alarm",
		}
		actionOutput = alarmPayload
		err = ae.deviceManager.ReportDeviceEvent(*meta, "rule_alarm", alarmPayload)
	case RuleActionLLM:
		if ae.deviceManager != nil && ae.deviceManager.EventBus != nil {
			responseTopic := fmt.Sprintf("rule.action.llm.response.%s.%d", ctx.Rule.Code, time.Now().UnixNano())
			responseChan := make(chan string, 1)

			subID := ae.deviceManager.EventBus.SubscribeWithID(types.EventType(responseTopic), func(e types.Event) {
				if text, ok := e.Payload.(map[string]any)["text"].(string); ok {
					responseChan <- text
				} else {
					responseChan <- ""
				}
			})
			defer ae.deviceManager.EventBus.Unsubscribe(types.EventType(responseTopic), subID)

			ae.deviceManager.EventBus.Publish(types.Event{
				Type:  types.EventType("rule.action.llm"),
				Topic: ctx.Rule.Code,
				Payload: map[string]any{
					"rule_code":        ctx.Rule.Code,
					"rule_name":        ctx.Rule.Name,
					"rule":             ctx.Rule,
					"trigger":          ctx.TemplateVars["trigger"],
					"triggers":         ctx.Rule.Triggers,
					"conditions":       ctx.Rule.Conditions,
					"condition_values": ctx.TemplateVars["condition_values"],
					"prompt":           action.LLMPrompt,
					"play_audio":       action.LLMPlayAudio,
					"include_context":  true,
					"output_mode":      llmOutputMode,
					"output_schema":    action.OutputSchema,
					"priority":         ctx.Rule.Priority,
					"trigger_time":     time.Now().Format(time.RFC3339),
					"trigger_event":    ctx.TemplateVars["event"],
					"response_topic":   responseTopic,
				},
				Timestamp: time.Now().UnixMilli(),
			})

			select {
			case resText := <-responseChan:
				if ctx.TemplateVars == nil {
					ctx.TemplateVars = make(map[string]any)
				}
				ctx.TemplateVars["llm_result"] = resText
				var jsonRes map[string]any
				if err := json.Unmarshal([]byte(resText), &jsonRes); err == nil {
					actionOutput = jsonRes
				} else {
					actionOutput = resText
				}
			case <-time.After(30 * time.Second):
				err = fmt.Errorf("llm action timeout")
			}
		}
	case RuleActionVoicePlayback:
		if ae.deviceManager != nil && ae.deviceManager.EventBus != nil {
			text := action.VoiceText
			if text == "" || text == "${llm_result}" {
				if res, ok := ctx.TemplateVars["llm_result"].(string); ok {
					text = res
				}
			} else {
				if res, ok := ctx.TemplateVars["llm_result"].(string); ok {
					text = strings.ReplaceAll(text, "${llm_result}", res)
				}
			}

			ae.deviceManager.EventBus.Publish(types.Event{
				Type:  types.EventType("rule.action.voice_playback"),
				Topic: ctx.Rule.Code,
				Payload: map[string]any{
					"text":     text,
					"priority": ctx.Rule.Priority,
				},
				Timestamp: time.Now().UnixMilli(),
			})
		}
	default:
		err = fmt.Errorf("unsupported action type %s", action.Type)
	}
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
	}
	result.DurationMs = time.Since(start).Milliseconds()
	setActionNodeResult(ctx, result, actionOutput)
	return result
}

func setActionNodeResult(ctx *RuleExecContext, result ActionResult, data any) {
	if ctx == nil || result.ActionID == "" {
		return
	}
	ctx.NodeResultsMu.Lock()
	defer ctx.NodeResultsMu.Unlock()
	if ctx.NodeResults == nil {
		ctx.NodeResults = make(map[string]any)
	}
	nodeValue := map[string]any{
		"status":     actionStatusLabel(result.Status),
		"durationMs": result.DurationMs,
	}
	if result.Error != "" {
		nodeValue["error"] = result.Error
	}
	if data != nil {
		nodeValue["data"] = data
		nodeValue["output"] = data
		if dataMap, ok := data.(map[string]any); ok {
			for key, value := range dataMap {
				if _, exists := nodeValue[key]; !exists {
					nodeValue[key] = value
				}
			}
		}
	}
	ctx.NodeResults[result.ActionID] = nodeValue
}

func actionStatusLabel(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "ok", "true":
		return "成功"
	case "failed", "failure", "error", "false":
		return "失败"
	case "skipped":
		return "跳过"
	default:
		return ""
	}
}

func snapshotNodeResults(ctx *RuleExecContext) map[string]any {
	if ctx == nil {
		return nil
	}
	ctx.NodeResultsMu.RLock()
	defer ctx.NodeResultsMu.RUnlock()
	if ctx.NodeResults == nil {
		return nil
	}
	result := make(map[string]any, len(ctx.NodeResults))
	for key, value := range ctx.NodeResults {
		result[key] = value
	}
	return result
}

func normalizeRuleActionValue(dm *DeviceManager, deviceCode, propertyKey string, value any) (any, error) {
	if dm == nil || dm.Registry == nil || deviceCode == "" || propertyKey == "" {
		return value, nil
	}
	device, ok := dm.Registry.GetDevice(deviceCode)
	if !ok {
		return value, nil
	}
	product, ok := dm.Registry.GetProduct(device.ProductCode)
	if !ok {
		return value, nil
	}
	tsl, err := ParseProductTSL(product.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse target device TSL: %w", err)
	}
	for _, property := range tsl.Properties {
		if property.Key == propertyKey {
			return normalizeValueByType(value, property.DataType)
		}
	}
	return value, nil
}

func normalizeValueByType(value any, dataType string) (any, error) {
	dataType = strings.ToLower(strings.TrimSpace(dataType))
	switch dataType {
	case "double", "float", "float32", "float64":
		return toFloat64(value)
	case "int", "integer", "long", "int32", "int64", "uint", "uint32", "uint64":
		return toInt64(value)
	case "bool", "boolean":
		return toBool(value)
	default:
		return value, nil
	}
}

func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case json.Number:
		return v.Float64()
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return 0, fmt.Errorf("property value %q is not a number", v)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("property value %[1]T cannot be converted to number", value)
	}
}

func toInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int64:
		return v, nil
	case float64:
		return int64(v), nil
	case json.Number:
		return v.Int64()
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err == nil {
			return parsed, nil
		}
		floatValue, floatErr := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if floatErr != nil {
			return 0, fmt.Errorf("property value %q is not an integer", v)
		}
		return int64(floatValue), nil
	default:
		return 0, fmt.Errorf("property value %[1]T cannot be converted to integer", value)
	}
}

func toBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case string:
		normalized := strings.ToLower(strings.TrimSpace(v))
		switch normalized {
		case "true", "1", "yes", "on":
			return true, nil
		case "false", "0", "no", "off":
			return false, nil
		default:
			return false, fmt.Errorf("property value %q is not a boolean", v)
		}
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, fmt.Errorf("property value %[1]T cannot be converted to boolean", value)
	}
}
