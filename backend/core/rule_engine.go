package core

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
	cronEntries   map[string]cron.EntryID
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
		Version:       rule.Version,
		Triggers:      def.Triggers,
		Conditions:    def.Conditions,
		Actions:       def.Actions,
		EffectiveTime: def.EffectiveTime,
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
			re.cronEntries[rule.Code+":"+trigger.ID] = entryID
		}
	}
	return nil
}

func (re *RuleEngine) handleEvent(event types.Event) {
	re.rules.Range(func(_, value any) bool {
		rule := value.(*RuleRuntime)
		for _, trigger := range rule.Triggers {
			if re.triggerMatches(trigger, event) {
				re.enqueue(rule.Code, trigger, event, 1)
			}
		}
		return true
	})
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
		TemplateVars:  buildTemplateVars(rule, trigger, event),
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
	if !ruleEffectiveAtPtr(execCtx.Rule.EffectiveTime, time.Now()) {
		re.writeLog(execCtx, trigger, event, false, nil, "rule is outside effective time", time.Since(start).Milliseconds(), 1)
		return
	}
	conditionOK := re.evaluateConditions(execCtx.Rule.Conditions)
	var results []ActionResult
	if conditionOK {
		results = re.executor.Execute(execCtx)
	}
	success := conditionOK
	errorMessage := ""
	for _, result := range results {
		if result.Status == "failed" {
			success = false
			errorMessage = result.Error
			break
		}
	}
	re.writeLog(execCtx, trigger, event, success, results, errorMessage, time.Since(start).Milliseconds(), 1)
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

func buildTemplateVars(rule RuleRuntime, trigger RuleTrigger, event types.Event) map[string]any {
	return map[string]any{
		"rule":    rule,
		"trigger": trigger,
		"event":   event,
	}
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
