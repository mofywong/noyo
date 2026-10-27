package core

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"noyo/core/dataquery"
	"noyo/core/screen"
	"noyo/core/store"
)

const screenPreviewBindingLimit = 32
const screenPreviewDeadline = 3 * time.Second

type screenPreviewValue struct {
	Value      interface{} `json:"value"`
	Quality    string      `json:"quality"`
	Timezone   string      `json:"timezone,omitempty"`
	ReceivedAt int64       `json:"receivedAt,omitempty"`
}
type screenPreviewJob func(context.Context) screenPreviewValue

// Read-only registry for candidate bindings. Identity/scope checks happen before
// any work is shared; no request object escapes to asynchronous data providers.
func (s *Server) handleScreenBindingPreview(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, _, err := screenReadDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), screenPreviewDeadline)
	defer cancel()
	jobs := map[string]screenPreviewJob{}
	ids := make([]string, 0, len(doc.Bindings))
	for id := range doc.Bindings {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// Authorization work is bounded by the same budget as provider work. The
	// remaining contracts are explicitly unchecked, never optimistically passed.
	authDoc := doc
	authDoc.Bindings = map[string]screen.Binding{}
	for index, id := range ids {
		if index >= screenPreviewBindingLimit {
			break
		}
		authDoc.Bindings[id] = doc.Bindings[id]
	}
	allowed := screenAllowedBindings(r, row, authDoc)
	// One live snapshot supplies all live kinds, preserving zero/false and the
	// runtime's quality semantics rather than rebuilding a second data model.
	liveDoc := doc
	liveDoc.Bindings = map[string]screen.Binding{}
	for index, id := range ids {
		if index >= screenPreviewBindingLimit {
			continue
		}
		binding := doc.Bindings[id]
		if allowed[id] && screenPreviewLiveKind(binding.Kind) {
			liveDoc.Bindings[id] = binding
		}
	}
	var liveOnce sync.Once
	var live map[string]screenRuntimeSample
	for index, id := range ids {
		binding, bindingID := doc.Bindings[id], id
		if index >= screenPreviewBindingLimit {
			jobs[id] = nil
			continue
		}
		if !allowed[id] {
			jobs[id] = func(context.Context) screenPreviewValue { return screenPreviewValue{Quality: "access_denied"} }
			continue
		}
		switch binding.Kind {
		case "point", "aggregate", "deviceStatus", "deviceSummary", "location":
			jobs[id] = func(context.Context) screenPreviewValue {
				liveOnce.Do(func() {
					live = s.screenRuntimeBatch(liveDoc, "proposal-preview", allowed)["bindings"].(map[string]screenRuntimeSample)
				})
				value := live[bindingID]
				return screenPreviewValue{Value: value.Value, Quality: value.Quality, ReceivedAt: value.ReceivedAt}
			}
		case "history", "eventList":
			jobs[id] = func(ctx context.Context) screenPreviewValue {
				value, readErr := s.screenReadBindingHistory(ctx, row, binding, 0, 0, 240)
				if readErr != nil {
					return screenPreviewError(readErr)
				}
				return screenPreviewValue{Value: value.Value, Quality: value.Quality, ReceivedAt: value.ReceivedAt}
			}
		case "alarmList":
			jobs[id] = func(ctx context.Context) screenPreviewValue { return screenPreviewAlarms(ctx, row, binding) }
		case "dataset":
			job, prepareErr := s.screenPreparePreviewDataset(r, row, binding)
			if prepareErr != nil {
				value := screenPreviewError(prepareErr)
				jobs[id] = func(context.Context) screenPreviewValue { return value }
			} else {
				jobs[id] = job
			}
		default:
			jobs[id] = nil
		}
	}
	screenWriteData(r, map[string]interface{}{"bindings": screenRunPreviewJobs(ctx, jobs), "serverTime": time.Now().UnixMilli(), "scope": "candidate", "maxBindings": screenPreviewBindingLimit})
}

func screenPreviewLiveKind(kind string) bool {
	switch kind {
	case "point", "aggregate", "deviceStatus", "deviceSummary", "location":
		return true
	}
	return false
}

// At most four provider calls, 32 bindings and one shared deadline. Buffered
// completion prevents a late provider from touching a returned result map.
func screenRunPreviewJobs(ctx context.Context, jobs map[string]screenPreviewJob) map[string]screenPreviewValue {
	ids := make([]string, 0, len(jobs))
	for id := range jobs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	values := map[string]screenPreviewValue{}
	selected := []string{}
	for index, id := range ids {
		values[id] = screenPreviewValue{Quality: "unchecked"}
		if index < screenPreviewBindingLimit && jobs[id] != nil {
			selected = append(selected, id)
			values[id] = screenPreviewValue{Quality: "waiting"}
		}
	}
	type completion struct {
		id    string
		value screenPreviewValue
	}
	completed := make(chan completion, len(selected))
	queue := make(chan string, len(selected))
	for _, id := range selected {
		queue <- id
	}
	close(queue)
	for worker := 0; worker < min(4, len(selected)); worker++ {
		go func() {
			for id := range queue {
				if ctx.Err() != nil {
					return
				}
				value := jobs[id](ctx)
				select {
				case completed <- completion{id, value}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	for remaining := len(selected); remaining > 0; remaining-- {
		select {
		case item := <-completed:
			values[item.id] = item.value
		case <-ctx.Done():
			return values
		}
	}
	return values
}

func screenPreviewError(err error) screenPreviewValue {
	quality := "query_error"
	var apiErr screenAPIError
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		quality = "waiting"
	} else if errors.As(err, &apiErr) && apiErr.status == 403 {
		quality = "access_denied"
	}
	return screenPreviewValue{Quality: quality}
}

func (s *Server) screenPreparePreviewDataset(r *ghttp.Request, row *store.IOCScreen, binding screen.Binding) (screenPreviewJob, error) {
	plan, err := screenDatasetPlan(row, binding)
	if err != nil {
		return nil, err
	}
	if plan.MaxRows == 0 || plan.MaxRows > 1000 {
		plan.MaxRows = 1000
	}
	now := time.Now().UnixMilli()
	input, err := dataquery.Resolve(plan, binding.Parameters, 0, now/5000*5000, now)
	if err != nil {
		return nil, err
	}
	prepared, _, err := screenPrepareData(r, row, plan, input)
	if err != nil {
		return nil, err
	}
	auth := requestAuthContext(r)
	reader := s.screenDataReader(prepared, ScreenDataScope{TenantID: row.TenantID, ProjectID: row.ProjectID, UserID: auth.UserID, AppID: auth.AppID})
	return func(ctx context.Context) screenPreviewValue {
		result, readErr := dataquery.Execute(ctx, plan, input, reader)
		if readErr != nil {
			return screenPreviewError(readErr)
		}
		quality := result.Meta.Quality
		if quality == "good" && len(result.Rows) == 0 {
			quality = "no_data"
		}
		return screenPreviewValue{Value: result.Rows, Quality: quality, Timezone: result.Meta.Timezone, ReceivedAt: result.Meta.GeneratedAt}
	}, nil
}

func screenPreviewAlarms(ctx context.Context, row *store.IOCScreen, binding screen.Binding) screenPreviewValue {
	devices := map[string]bool{}
	for _, code := range screenBindingDevices(binding) {
		devices[code] = true
	}
	var alarms []store.AlarmInstance
	err := store.DB.WithContext(ctx).Model(&store.AlarmInstance{}).Select("public_id", "title", "severity", "condition_status", "handling_status", "last_occurred_at", "evidence_snapshot").Where("tenant_id = ? AND project_id = ? AND handling_status <> ?", row.TenantID, row.ProjectID, AlarmHandlingClosed).Order("last_occurred_at DESC").Order("id DESC").Limit(1000).Find(&alarms).Error
	if err != nil {
		return screenPreviewError(err)
	}
	limit := binding.MaxPoints
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	result := []map[string]interface{}{}
	for _, alarm := range alarms {
		var evidence struct {
			DeviceCode string `json:"device_code"`
			DeviceName string `json:"device_name"`
		}
		if json.Unmarshal([]byte(alarm.EvidenceSnapshot), &evidence) != nil || !devices[evidence.DeviceCode] {
			continue
		}
		result = append(result, map[string]interface{}{"id": alarm.PublicID, "deviceCode": evidence.DeviceCode, "deviceName": evidence.DeviceName, "title": alarm.Title, "severity": alarm.Severity, "conditionStatus": alarm.ConditionStatus, "handlingStatus": alarm.HandlingStatus, "occurredAt": alarm.LastOccurredAt})
		if len(result) >= limit {
			break
		}
	}
	quality := "good"
	if len(result) == 0 {
		quality = "no_data"
	}
	return screenPreviewValue{Value: result, Quality: quality, ReceivedAt: time.Now().UnixMilli()}
}
