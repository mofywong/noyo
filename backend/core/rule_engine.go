package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"noyo/core/store"
	"noyo/core/types"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type RuleEngine struct {
	server        *Server
	eventBus      *EventBus
	deviceManager *DeviceManager
	registry      *DeviceRegistry
	executor      *ActionExecutor
	distributor   *RuleDistributor
	cron          *cron.Cron
	cronMu        sync.Mutex
	cronEntries   map[string]cron.EntryID
	controlMu     sync.Mutex
	controls      map[string]*ruleExecutionControl
	rules         sync.Map
	execQueue     chan *RuleExecContext
	workerCount   int
	ruleTimeout   time.Duration
	actionTimeout time.Duration
	maxParallel   int
	depthLimit    int
	cancel        context.CancelFunc
}

func NewRuleEngine(server *Server) *RuleEngine {
	re := &RuleEngine{
		server:        server,
		eventBus:      server.DeviceManager.EventBus,
		deviceManager: server.DeviceManager,
		registry:      server.DeviceManager.Registry,
		cron:          newRuleCronScheduler(),
		cronEntries:   make(map[string]cron.EntryID),
		controls:      make(map[string]*ruleExecutionControl),
		execQueue:     make(chan *RuleExecContext, 2000),
		workerCount:   20,
		ruleTimeout:   60 * time.Second,
		actionTimeout: 10 * time.Second,
		maxParallel:   20,
		depthLimit:    5,
	}
	re.executor = NewActionExecutor(re.deviceManager, server.Logger)
	re.distributor = NewRuleDistributor(re.registry)
	return re
}

func newRuleCronScheduler() *cron.Cron {
	parser := cron.NewParser(
		cron.SecondOptional |
			cron.Minute |
			cron.Hour |
			cron.Dom |
			cron.Month |
			cron.Dow |
			cron.Descriptor,
	)
	return cron.New(cron.WithParser(parser))
}

func (re *RuleEngine) Start() error {
	if err := re.LoadRules(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())
	re.cancel = cancel
	for i := 0; i < re.workerCount; i++ {
		go re.worker(ctx)
	}
	re.eventBus.Subscribe(types.EventPropertyReported, re.handleEvent)
	re.eventBus.Subscribe(types.EventEventReported, re.handleEvent)
	re.eventBus.Subscribe(types.EventDeviceStatusChanged, re.handleEvent)
	re.cron.Start()
	return nil
}

func (re *RuleEngine) Stop() {
	if re.cancel != nil {
		re.cancel()
	}
	if re.cron != nil {
		re.cron.Stop()
	}
}

func (re *RuleEngine) LoadRules() error {
	var rules []store.Rule
	if err := store.DB.Where("enabled = ? AND status = ?", true, RuleStatusEnabled).Find(&rules).Error; err != nil {
		return err
	}
	re.clearCronEntries()
	re.rules.Range(func(key, value any) bool {
		re.rules.Delete(key)
		return true
	})
	for _, rule := range rules {
		if err := re.loadRule(rule); err != nil && re.server != nil && re.server.Logger != nil {
			re.server.Logger.Warn("failed to load rule", zap.String("rule", rule.Code), zap.Error(err))
		}
	}
	return nil
}

func (re *RuleEngine) loadRule(rule store.Rule) error {
	def, err := RuleDefinitionFromStore(&rule)
	if err != nil {
		return err
	}
	rt := &RuleRuntime{
		Code:          rule.Code,
		Name:          rule.Name,
		Description:   rule.Description,
		Version:       rule.Version,
		Priority:      rule.Priority,
		ThrottleSec:   rule.ThrottleSec,
		MaxPerHour:    rule.MaxPerHour,
		RetryCount:    rule.RetryCount,
		Triggers:      def.Triggers,
		Conditions:    def.Conditions,
		Actions:       def.Actions,
		EffectiveTime: def.EffectiveTime,
	}
	if rt.Priority <= 0 {
		rt.Priority = 50
	}
	re.rules.Store(rule.Code, rt)
	for _, trigger := range rt.Triggers {
		if trigger.Type == RuleTriggerCron && trigger.CronExpr != "" {
			ruleCode := rule.Code
			triggerCopy := trigger
			entryID, err := re.cron.AddFunc(trigger.CronExpr, func() {
				re.enqueue(ruleCode, triggerCopy, types.Event{Type: types.EventType(RuleTriggerCron), Timestamp: time.Now().UnixMilli()}, 1)
			})
			if err != nil {
				return err
			}
			re.cronMu.Lock()
			re.cronEntries[rule.Code+":"+trigger.ID] = entryID
			re.cronMu.Unlock()
		}
	}
	return nil
}

func (re *RuleEngine) clearCronEntries() {
	if re == nil || re.cron == nil {
		return
	}
	re.cronMu.Lock()
	defer re.cronMu.Unlock()
	for key, entryID := range re.cronEntries {
		re.cron.Remove(entryID)
		delete(re.cronEntries, key)
	}
}

func (re *RuleEngine) handleEvent(event types.Event) {
	// Skip processing rules for initial property state load on boot
	if event.Type == types.EventPropertyReported && event.Metadata != nil {
		if isInitial, ok := event.Metadata["isInitial"].(bool); ok && isInitial {
			return
		}
	}

	for _, match := range re.matchingRules(event) {
		re.enqueue(match.ruleCode, match.trigger, event, 1)
	}
}

type ruleExecutionControl struct {
	inFlight []time.Time
}

type matchedRuleTrigger struct {
	ruleCode string
	priority int
	trigger  RuleTrigger
}

func (re *RuleEngine) matchingRules(event types.Event) []matchedRuleTrigger {
	matches := make([]matchedRuleTrigger, 0)
	re.rules.Range(func(_, value any) bool {
		rule := value.(*RuleRuntime)
		for _, trigger := range rule.Triggers {
			if re.triggerMatches(trigger, event) {
				matches = append(matches, matchedRuleTrigger{
					ruleCode: rule.Code,
					priority: rule.Priority,
					trigger:  trigger,
				})
			}
		}
		return true
	})
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].priority == matches[j].priority {
			return matches[i].ruleCode < matches[j].ruleCode
		}
		return matches[i].priority < matches[j].priority
	})
	return matches
}

func (re *RuleEngine) enqueue(ruleCode string, trigger RuleTrigger, event types.Event, depth int) {
	if depth > re.depthLimit {
		return
	}
	value, ok := re.rules.Load(ruleCode)
	if !ok {
		return
	}
	rule := *value.(*RuleRuntime)
	ctx := &RuleExecContext{
		Context:       context.Background(),
		Rule:          rule,
		TemplateVars:  re.buildTemplateVars(rule, trigger, event),
		NodeResults:   make(map[string]any),
		SessionID:     uuid.New().String(),
		ActionTimeout: re.actionTimeout,
		RuleTimeout:   re.ruleTimeout,
		MaxParallel:   re.maxParallel,
	}
	select {
	case re.execQueue <- ctx:
	default:
		re.writeLog(ctx, trigger, event, false, nil, "execution queue full", 0, depth)
	}
}

func (re *RuleEngine) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case execCtx := <-re.execQueue:
			re.execute(execCtx)
		}
	}
}

func (re *RuleEngine) execute(execCtx *RuleExecContext) {
	start := time.Now()
	trigger, event := triggerFromVars(execCtx.TemplateVars)
	now := time.Now()
	if !ruleEffectiveAtPtr(execCtx.Rule.EffectiveTime, now) {
		re.writeLog(execCtx, trigger, event, false, nil, "rule is outside effective time", time.Since(start).Milliseconds(), 1)
		return
	}
	conditionOK := re.evaluateConditions(execCtx.Rule.Conditions)
	var results []ActionResult
	if conditionOK {
		if execCtx.TemplateVars == nil {
			execCtx.TemplateVars = make(map[string]any)
		}
		execCtx.TemplateVars["condition_values"] = re.collectConditionValues(execCtx.Rule.Conditions)
		allowed, reason, release := re.reserveRuleExecution(execCtx.Rule, now)
		if !allowed {
			if re.server != nil && re.server.Logger != nil {
				re.server.Logger.Debug("rule execution skipped by control limit", zap.String("rule", execCtx.Rule.Code), zap.String("reason", reason))
			}
			return
		}
		if release != nil {
			defer release()
		}
		results = re.executeActionsWithRetry(execCtx)
	}
	success := conditionOK
	errorMessage := ""
	if actionError := firstFailedActionError(results); actionError != "" {
		success = false
		errorMessage = actionError
	}
	re.writeLog(execCtx, trigger, event, success, results, errorMessage, time.Since(start).Milliseconds(), 1)
}

func (re *RuleEngine) executeActionsWithRetry(execCtx *RuleExecContext) []ActionResult {
	attempts := execCtx.Rule.RetryCount + 1
	if attempts < 1 {
		attempts = 1
	}
	var results []ActionResult
	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			execCtx.NodeResultsMu.Lock()
			execCtx.NodeResults = make(map[string]any)
			execCtx.NodeResultsMu.Unlock()
		}
		results = re.executor.Execute(execCtx)
		if firstFailedActionError(results) == "" {
			return results
		}
	}
	return results
}

func firstFailedActionError(results []ActionResult) string {
	for _, result := range results {
		if result.Status == "failed" {
			return result.Error
		}
	}
	return ""
}

func (re *RuleEngine) reserveRuleExecution(rule RuleRuntime, now time.Time) (bool, string, func()) {
	persistedLast, persistedCount := re.persistedRuleExecutionState(rule, now)
	re.controlMu.Lock()
	defer re.controlMu.Unlock()
	if re.controls == nil {
		re.controls = make(map[string]*ruleExecutionControl)
	}
	state := re.controls[rule.Code]
	if state == nil {
		state = &ruleExecutionControl{}
		re.controls[rule.Code] = state
	}
	state.inFlight = pruneRuleExecutionReservations(state.inFlight, now.Add(-time.Hour))

	if rule.ThrottleSec > 0 {
		if !persistedLast.IsZero() && now.Before(persistedLast.Add(time.Duration(rule.ThrottleSec)*time.Second)) {
			return false, "rule is throttled", nil
		}
	}
	if rule.MaxPerHour > 0 {
		if persistedCount+int64(len(state.inFlight)) >= int64(rule.MaxPerHour) {
			return false, "rule exceeds hourly execution limit", nil
		}
	}
	state.inFlight = append(state.inFlight, now)
	return true, "", func() {
		re.controlMu.Lock()
		defer re.controlMu.Unlock()
		current := re.controls[rule.Code]
		if current == nil {
			return
		}
		current.inFlight = removeRuleExecutionReservation(current.inFlight, now)
	}
}

func (re *RuleEngine) persistedRuleExecutionState(rule RuleRuntime, now time.Time) (time.Time, int64) {
	if store.DB == nil || rule.Code == "" {
		return time.Time{}, 0
	}
	var last time.Time
	if rule.ThrottleSec > 0 {
		model, err := store.GetRule(rule.Code)
		if err == nil && model != nil && model.LastTriggeredAt != nil {
			last = time.UnixMilli(*model.LastTriggeredAt)
		}
	}
	var count int64
	if rule.MaxPerHour > 0 {
		since := now.Add(-time.Hour).UnixMilli()
		_ = store.DB.Model(&store.RuleExecLog{}).
			Where("rule_code = ? AND success = ? AND executed_at >= ?", rule.Code, true, since).
			Count(&count).Error
	}
	return last, count
}

func pruneRuleExecutionReservations(values []time.Time, cutoff time.Time) []time.Time {
	filtered := values[:0]
	for _, value := range values {
		if value.After(cutoff) || value.Equal(cutoff) {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func removeRuleExecutionReservation(values []time.Time, target time.Time) []time.Time {
	for index, value := range values {
		if value.Equal(target) {
			return append(values[:index], values[index+1:]...)
		}
	}
	return values
}

func (re *RuleEngine) triggerMatches(trigger RuleTrigger, event types.Event) bool {
	if trigger.DeviceCode != "" && event.Topic != trigger.DeviceCode {
		return false
	}
	switch trigger.Type {
	case RuleTriggerProperty:
		if event.Type != types.EventPropertyReported {
			return false
		}
		props := eventProperties(event)
		if props == nil {
			return false
		}
		if trigger.PropertyKey == "" {
			if trigger.Operator == "changed" {
				return len(eventChangedProperties(event)) > 0
			}
			return len(props) > 0
		}
		value, exists := props[trigger.PropertyKey]
		if trigger.Operator == "changed" {
			_, changed := eventChangedProperties(event)[trigger.PropertyKey]
			return changed
		}
		return exists && compareValues(value, trigger.Operator, trigger.Value)
	case RuleTriggerEvent:
		if event.Type != types.EventEventReported {
			return false
		}
		payload, _ := event.Payload.(map[string]interface{})
		return stringValue(payload["eventId"]) == trigger.EventID
	case RuleTriggerDeviceStatus:
		if event.Type != types.EventDeviceStatusChanged {
			return false
		}
		if trigger.StatusValue == "" || trigger.StatusValue == "any" {
			return true
		}
		payload, _ := event.Payload.(map[string]interface{})
		return stringValue(payload["status"]) == trigger.StatusValue
	default:
		return false
	}
}

func (re *RuleEngine) evaluateConditions(group *RuleConditionGroup) bool {
	if group == nil {
		return true
	}
	results := make([]bool, 0, len(group.Conditions)+len(group.Groups))
	for _, condition := range group.Conditions {
		results = append(results, re.evaluateCondition(condition))
	}
	for _, nested := range group.Groups {
		results = append(results, re.evaluateConditions(&nested))
	}
	if group.Logic == "or" {
		for _, result := range results {
			if result {
				return true
			}
		}
		return len(results) == 0
	}
	for _, result := range results {
		if !result {
			return false
		}
	}
	return true
}

func (re *RuleEngine) evaluateCondition(condition RuleCondition) bool {
	switch condition.Type {
	case "property":
		data := re.deviceManager.GetLatestData(condition.DeviceCode)
		value, ok := data[condition.PropertyKey]
		return ok && compareValues(value, condition.Operator, condition.Value)
	case "device_status":
		status, ok := re.deviceManager.GetStatus(condition.DeviceCode)
		if !ok {
			return false
		}
		return (condition.StatusValue == "online" && status.Online) || (condition.StatusValue == "offline" && !status.Online)
	case "time_range":
		return timeInConditionRange(time.Now(), condition)
	default:
		return false
	}
}

func (re *RuleEngine) collectConditionValues(group *RuleConditionGroup) []map[string]any {
	if group == nil {
		return nil
	}
	values := make([]map[string]any, 0, len(group.Conditions))
	for _, condition := range group.Conditions {
		item := map[string]any{
			"id":           condition.ID,
			"type":         condition.Type,
			"deviceCode":   condition.DeviceCode,
			"deviceName":   re.resolveDeviceName(condition.DeviceCode, condition.DeviceName),
			"propertyKey":  condition.PropertyKey,
			"propertyName": re.resolvePropertyName(condition.DeviceCode, condition.PropertyKey, condition.PropertyKey),
			"operator":     condition.Operator,
			"expected":     condition.Value,
			"statusValue":  condition.StatusValue,
			"startTime":    condition.StartTime,
			"endTime":      condition.EndTime,
			"weekdays":     condition.Weekdays,
			"timezone":     condition.Timezone,
			"matched":      re.evaluateCondition(condition),
		}
		if condition.DeviceCode != "" {
			item["properties"] = re.deviceManager.GetLatestData(condition.DeviceCode)
			if status, ok := re.deviceManager.GetStatus(condition.DeviceCode); ok {
				if status.Online {
					item["deviceStatus"] = "online"
				} else {
					item["deviceStatus"] = "offline"
				}
			}
		}
		switch condition.Type {
		case "property":
			data := re.deviceManager.GetLatestData(condition.DeviceCode)
			if actual, ok := data[condition.PropertyKey]; ok {
				item["actualValue"] = actual
				item["propertyValue"] = actual
			}
		case "device_status":
			if status, ok := re.deviceManager.GetStatus(condition.DeviceCode); ok {
				if status.Online {
					item["actualValue"] = "online"
				} else {
					item["actualValue"] = "offline"
				}
			}
		case "time_range":
			item["actualValue"] = time.Now().Format("15:04:05")
		}
		values = append(values, item)
	}
	for _, nested := range group.Groups {
		values = append(values, re.collectConditionValues(&nested)...)
	}
	return values
}

func (re *RuleEngine) writeLog(execCtx *RuleExecContext, trigger RuleTrigger, event types.Event, success bool, results []ActionResult, errorMessage string, durationMs int64, depth int) {
	actionJSON, _ := json.Marshal(results)
	triggerJSON, _ := json.Marshal(map[string]any{
		"eventType": event.Type,
		"topic":     event.Topic,
		"payload":   event.Payload,
	})
	model, _ := store.GetRule(execCtx.Rule.Code)
	log := &store.RuleExecLog{
		RuleCode:      execCtx.Rule.Code,
		RuleName:      execCtx.Rule.Name,
		RuleVersion:   execCtx.Rule.Version,
		TriggerID:     trigger.ID,
		TriggerType:   trigger.Type,
		TriggerDetail: string(triggerJSON),
		TraceID:       uuid.NewString(),
		ChainDepth:    depth,
		Success:       success,
		ErrorMessage:  errorMessage,
		ActionResults: string(actionJSON),
		DurationMs:    durationMs,
		ExecutedAt:    time.Now().UnixMilli(),
	}
	if model != nil {
		log.TenantID = model.TenantID
		log.ProjectID = model.ProjectID
		log.RuleID = model.ID
		log.Scope = model.Scope
		log.GatewaySN = model.GatewaySN
		log.ExecutedAs = model.EnabledBy
	}
	if err := store.CreateRuleExecLog(log); err == nil && success {
		now := time.Now().UnixMilli()
		_ = store.DB.Model(&store.Rule{}).Where("code = ?", execCtx.Rule.Code).Updates(map[string]any{
			"last_triggered_at": &now,
			"trigger_count":     gorm.Expr("trigger_count + ?", 1),
			"error_message":     "",
		}).Error
	} else if errorMessage != "" {
		_ = store.DB.Model(&store.Rule{}).Where("code = ?", execCtx.Rule.Code).Update("error_message", errorMessage).Error
	}
}

func compareValues(left any, operator string, right any) bool {
	switch operator {
	case "eq", "==":
		return fmt.Sprint(left) == fmt.Sprint(right)
	case "neq", "!=":
		return fmt.Sprint(left) != fmt.Sprint(right)
	case "gt", ">":
		l, lok := numericValue(left)
		r, rok := numericValue(right)
		return lok && rok && l > r
	case "gte", ">=":
		l, lok := numericValue(left)
		r, rok := numericValue(right)
		return lok && rok && l >= r
	case "lt", "<":
		l, lok := numericValue(left)
		r, rok := numericValue(right)
		return lok && rok && l < r
	case "lte", "<=":
		l, lok := numericValue(left)
		r, rok := numericValue(right)
		return lok && rok && l <= r
	case "contains":
		return strings.Contains(fmt.Sprint(left), fmt.Sprint(right))
	default:
		return false
	}
}

func numericValue(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil && !math.IsNaN(f)
	default:
		return 0, false
	}
}

func timeInConditionRange(now time.Time, condition RuleCondition) bool {
	if condition.StartTime == "" || condition.EndTime == "" {
		return true
	}
	startSec, err1 := parseTimeOfDaySeconds(condition.StartTime)
	endSec, err2 := parseTimeOfDaySeconds(condition.EndTime)
	if err1 != nil || err2 != nil {
		return false
	}
	currentSec := now.Hour()*3600 + now.Minute()*60 + now.Second()
	if startSec <= endSec {
		return currentSec >= startSec && currentSec <= endSec
	}
	return currentSec >= startSec || currentSec <= endSec
}

func ruleEffectiveAtPtr(effective *RuleEffectiveTime, now time.Time) bool {
	if effective == nil || effective.Mode == "" {
		return true
	}
	return ruleEffectiveAt(*effective, now)
}

func ruleEffectiveAt(effective RuleEffectiveTime, now time.Time) bool {
	if effective.Timezone != "" {
		if loc, err := time.LoadLocation(effective.Timezone); err == nil {
			now = now.In(loc)
		}
	}
	if effective.Mode == RuleEffectiveAlways {
		return true
	}
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	monthDay := now.Day()
	month := int(now.Month())
	dateOK := false
	switch effective.Mode {
	case "", RuleEffectiveDaily:
		dateOK = true
	case RuleEffectiveWeekly:
		dateOK = len(effective.Weekdays) == 0 || containsInt(effective.Weekdays, weekday)
	case RuleEffectiveMonthly:
		dateOK = len(effective.MonthDays) == 0 || containsInt(effective.MonthDays, monthDay)
	case RuleEffectiveWorkday:
		dateOK = weekday >= 1 && weekday <= 5
	case RuleEffectiveHoliday:
		dateOK = weekday == 6 || weekday == 7
	case RuleEffectiveCustom:
		if len(effective.Months) > 0 && !containsInt(effective.Months, month) {
			return false
		}
		if len(effective.MonthDays) > 0 && !containsInt(effective.MonthDays, monthDay) {
			return false
		}
		if len(effective.Weekdays) > 0 && !containsInt(effective.Weekdays, weekday) {
			return false
		}
		dateOK = true
	default:
		return false
	}
	return dateOK && effectiveInAnyWindow(now, effective)
}

func effectiveInAnyWindow(now time.Time, effective RuleEffectiveTime) bool {
	if len(effective.Windows) == 0 {
		return timeInWindow(now, effective.StartTime, effective.EndTime)
	}
	for _, window := range effective.Windows {
		if len(window.MonthDays) > 0 && !containsInt(window.MonthDays, now.Day()) {
			continue
		}
		if timeInWindow(now, window.StartTime, window.EndTime) {
			return true
		}
	}
	return false
}

func timeInWindow(now time.Time, startText, endText string) bool {
	if startText == "" || endText == "" {
		return true
	}
	startSec, err1 := parseTimeOfDaySeconds(startText)
	endSec, err2 := parseTimeOfDaySeconds(endText)
	if err1 != nil || err2 != nil {
		return false
	}
	currentSec := now.Hour()*3600 + now.Minute()*60 + now.Second()
	if startSec <= endSec {
		return currentSec >= startSec && currentSec <= endSec
	}
	return currentSec >= startSec || currentSec <= endSec
}

func parseTimeOfDaySeconds(text string) (int, error) {
	parts := strings.Split(strings.TrimSpace(text), ":")
	if len(parts) != 2 && len(parts) != 3 {
		return 0, fmt.Errorf("invalid time %q", text)
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	second := 0
	if len(parts) == 3 {
		second, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, err
		}
	}
	if hour == 24 && minute == 0 && second == 0 {
		return 24 * 3600, nil
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 || second < 0 || second > 59 {
		return 0, fmt.Errorf("invalid time %q", text)
	}
	return hour*3600 + minute*60 + second, nil
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func (re *RuleEngine) buildTemplateVars(rule RuleRuntime, trigger RuleTrigger, event types.Event) map[string]any {
	deviceCode := trigger.DeviceCode
	if deviceCode == "" {
		deviceCode = event.Topic
	}
	if deviceCode != "" && re != nil && re.deviceManager != nil {
		trigger.DeviceCode = deviceCode
		trigger.DeviceName = re.resolveDeviceName(deviceCode, trigger.DeviceName)
		trigger.PropertyName = re.resolvePropertyName(deviceCode, trigger.PropertyKey, trigger.PropertyName)
		trigger.EventName = re.resolveEventName(deviceCode, trigger.EventID, trigger.EventName)
		trigger.Properties = mergeRuntimeProperties(re.deviceManager.GetLatestData(deviceCode), eventProperties(event))
		if trigger.PropertyKey != "" && trigger.Properties != nil {
			if value, ok := trigger.Properties[trigger.PropertyKey]; ok {
				trigger.TriggerValue = value
			}
		}
		if status, ok := re.deviceManager.GetStatus(deviceCode); ok {
			if status.Online {
				trigger.DeviceStatus = "online"
			} else {
				trigger.DeviceStatus = "offline"
			}
		}
	}
	trigger.TriggerTime = event.Timestamp
	trigger.TriggerTimeText = formatSystemTimeMillis(event.Timestamp)
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if params, ok := payload["params"].(map[string]interface{}); ok {
			trigger.EventParams = params
		}
	}
	return map[string]any{
		"rule":    rule,
		"trigger": trigger,
		"event":   event,
	}
}

func (re *RuleEngine) resolveDeviceName(deviceCode string, fallback string) string {
	if deviceCode == "" || re == nil || re.deviceManager == nil || re.deviceManager.Registry == nil {
		return fallback
	}
	if device, ok := re.deviceManager.Registry.GetDevice(deviceCode); ok {
		if device.Name != "" {
			return device.Name
		}
	}
	return fallback
}

func (re *RuleEngine) resolvePropertyName(deviceCode string, propertyKey string, fallback string) string {
	if deviceCode == "" || propertyKey == "" || re == nil || re.deviceManager == nil || re.deviceManager.Registry == nil {
		return fallback
	}
	device, ok := re.deviceManager.Registry.GetDevice(deviceCode)
	if !ok {
		return fallback
	}
	product, err := re.deviceManager.Registry.GetProductMeta(device.ProductCode)
	if err != nil {
		return fallback
	}
	tsl, ok := product.Config["tsl"].(map[string]any)
	if !ok {
		tsl = product.Config
	}
	props, ok := tsl["properties"].([]any)
	if !ok {
		return fallback
	}
	for _, raw := range props {
		prop, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(prop["identifier"]) == propertyKey {
			if name := stringValue(prop["name"]); name != "" {
				return name
			}
			return fallback
		}
	}
	return fallback
}

func formatSystemTimeMillis(timestamp int64) string {
	if timestamp <= 0 {
		return ""
	}
	return time.UnixMilli(timestamp).In(time.Local).Format("2006-01-02 15:04:05")
}

func (re *RuleEngine) resolveEventName(deviceCode string, eventID string, fallback string) string {
	if deviceCode == "" || eventID == "" || re == nil || re.deviceManager == nil || re.deviceManager.Registry == nil {
		return fallback
	}
	device, ok := re.deviceManager.Registry.GetDevice(deviceCode)
	if !ok {
		return fallback
	}
	product, err := re.deviceManager.Registry.GetProductMeta(device.ProductCode)
	if err != nil {
		return fallback
	}
	tsl, ok := product.Config["tsl"].(map[string]any)
	if !ok {
		tsl = product.Config
	}
	events, ok := tsl["events"].([]any)
	if !ok {
		return fallback
	}
	for _, raw := range events {
		event, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if stringValue(event["identifier"]) == eventID {
			if name := stringValue(event["name"]); name != "" {
				return name
			}
			return fallback
		}
	}
	return fallback
}

func mergeRuntimeProperties(latest map[string]interface{}, reported map[string]interface{}) map[string]any {
	if latest == nil && reported == nil {
		return nil
	}
	result := make(map[string]any, len(latest)+len(reported))
	for k, v := range latest {
		result[k] = v
	}
	for k, v := range reported {
		result[k] = v
	}
	return result
}

func triggerFromVars(vars map[string]any) (RuleTrigger, types.Event) {
	trigger, _ := vars["trigger"].(RuleTrigger)
	event, _ := vars["event"].(types.Event)
	return trigger, event
}

func eventProperties(event types.Event) map[string]interface{} {
	props, ok := event.Payload.(map[string]interface{})
	if ok {
		return props
	}
	return nil
}

func eventChangedProperties(event types.Event) map[string]interface{} {
	if event.Metadata == nil {
		return map[string]interface{}{}
	}
	raw, ok := event.Metadata["changedProperties"]
	if !ok {
		return map[string]interface{}{}
	}
	if changed, ok := raw.(map[string]interface{}); ok {
		return changed
	}
	return map[string]interface{}{}
}

func eventChangedPropertiesContains(event types.Event, key string) bool {
	_, ok := eventChangedProperties(event)[key]
	return ok
}
