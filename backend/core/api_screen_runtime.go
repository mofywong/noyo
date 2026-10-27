package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
	"noyo/core/screen"
	"noyo/core/store"
	"noyo/core/tsdb"
	"noyo/core/types"
	"noyo/core/utils"
)

var screenRuntimeEpoch = uuid.NewString()

type screenRuntimeSample struct {
	Value         interface{} `json:"value"`
	Quality       string      `json:"quality"`
	ReceivedAt    int64       `json:"receivedAt"`
	SampleVersion uint64      `json:"sampleVersion"`
}

// Authorization is evaluated against the stored document. Clients can request
// a binding ID, never supply a replacement device code or field list.
func screenBindingDevices(b screen.Binding) []string {
	result := append([]string{}, b.DeviceCodes...)
	if b.DeviceCode != "" {
		result = append(result, b.DeviceCode)
	}
	seen := map[string]bool{}
	unique := result[:0]
	for _, code := range result {
		if !seen[code] {
			seen[code] = true
			unique = append(unique, code)
		}
	}
	return unique
}

func screenBindingKeys(b screen.Binding) []string {
	if b.Kind == "location" && b.LocationSource == "device" {
		return nil
	}
	keys := append([]string{}, b.Keys...)
	if b.Key != "" {
		keys = append(keys, b.Key)
	}
	if b.Kind == "location" {
		keys = append(keys, b.LatitudeKey, b.LongitudeKey)
	}
	return keys
}

func screenReadableDevice(r *ghttp.Request, row *store.IOCScreen, code string) (*store.Device, bool) {
	auth := requestAuthContext(r)
	if auth == nil || !auth.HasProjectPermission("device:list", row.ProjectID) {
		return nil, false
	}
	device, err := store.GetDevice(code)
	if err != nil || device.TenantID != row.TenantID || device.ProjectID != row.ProjectID || !canAccessDevice(r, device) {
		return nil, false
	}
	return device, true
}

// Reading a product's TSL is the expensive half of the per-viewer permission
// refresh: a screen that binds 200 devices of one model unmarshals the same
// document 200 times in a single call and repeats that for every viewer every
// 15 seconds. One process-wide cache bounded by a short TTL keeps a product edit
// visible within a minute without the per-device parse.
const screenProductFieldsTTL = 60 * time.Second

var screenProductFieldsCache struct {
	sync.Mutex
	entries map[string]screenProductFieldsEntry
}

type screenProductFieldsEntry struct {
	fields  map[string]bool
	expires time.Time
}

func screenProductFields(productCode string) map[string]bool {
	code := strings.TrimSpace(productCode)
	if code == "" {
		return nil
	}
	now := time.Now()
	screenProductFieldsCache.Lock()
	if entry, ok := screenProductFieldsCache.entries[code]; ok && now.Before(entry.expires) {
		fields := entry.fields
		screenProductFieldsCache.Unlock()
		return fields
	}
	screenProductFieldsCache.Unlock()
	fields := map[string]bool{}
	for _, prop := range loadProductTSLProperties(code) {
		fields[propertyStringField(prop, "identifier")] = true
	}
	screenProductFieldsCache.Lock()
	if screenProductFieldsCache.entries == nil {
		screenProductFieldsCache.entries = map[string]screenProductFieldsEntry{}
	}
	screenProductFieldsCache.entries[code] = screenProductFieldsEntry{fields: fields, expires: now.Add(screenProductFieldsTTL)}
	screenProductFieldsCache.Unlock()
	return fields
}

func screenAllowedBindings(r *ghttp.Request, row *store.IOCScreen, doc screen.Envelope) map[string]bool {
	allowed := map[string]bool{}
	devices := map[string]bool{}
	fields := map[string]map[string]bool{}
	for id, binding := range doc.Bindings {
		allowed[id] = true
		if binding.Kind == "alarmList" {
			auth := requestAuthContext(r)
			allowed[id] = auth != nil && auth.HasProjectPermission("alarm:list", row.ProjectID)
		}
		for _, code := range screenBindingDevices(binding) {
			if _, checked := devices[code]; !checked {
				device, readable := screenReadableDevice(r, row, code)
				devices[code] = readable
				fields[code] = nil
				if readable {
					fields[code] = screenProductFields(device.ProductCode)
				}
			}
			if !devices[code] {
				allowed[id] = false
				break
			}
			for _, key := range screenBindingKeys(binding) {
				if !fields[code][key] {
					allowed[id] = false
					break
				}
			}
		}
	}
	return allowed
}

func screenNumber(value interface{}) (float64, bool) {
	var n float64
	switch v := value.(type) {
	case float64:
		n = v
	case float32:
		n = float64(v)
	case int:
		n = float64(v)
	case int8:
		n = float64(v)
	case int16:
		n = float64(v)
	case int32:
		n = float64(v)
	case int64:
		n = float64(v)
	case uint:
		n = float64(v)
	case uint8:
		n = float64(v)
	case uint16:
		n = float64(v)
	case uint32:
		n = float64(v)
	case uint64:
		n = float64(v)
	case json.Number:
		var err error
		n, err = v.Float64()
		if err != nil {
			return 0, false
		}
	default:
		return 0, false
	}
	return n, !math.IsNaN(n) && !math.IsInf(n, 0)
}

func screenTransform(value interface{}, b screen.Binding) interface{} {
	n, ok := screenNumber(value)
	if !ok {
		// A point can carry text or a boolean. Unsupported telemetry (including
		// nonfinite numbers) must never escape into an entire JSON/SSE batch.
		switch value.(type) {
		case nil, string, bool:
			return value
		default:
			return nil
		}
	}
	if b.Scale != nil {
		n *= *b.Scale
	}
	if b.Offset != nil {
		n += *b.Offset
	}
	if b.Decimals != nil {
		scale := math.Pow10(*b.Decimals)
		n = math.Round(n*scale) / scale
	}
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return nil
	}
	return n
}

func screenSampleQuality(snapshot ScreenDeviceSnapshot, sample ScreenSample, binding screen.Binding, now int64) string {
	if sample.ReceivedAt == 0 {
		return "no_data"
	}
	if !snapshot.Status.Online {
		return "offline"
	}
	stale := binding.StaleAfterMs
	if stale == 0 {
		stale = 60000
	}
	if now-sample.ReceivedAt > stale {
		return "stale"
	}
	return "good"
}

// Live-screen timing budget. A wall display is a passive consumer: sending the
// same payload five times a second only forces every chart to redraw, and a
// per-second freshness query per viewer is pure waste on a screen that has not
// changed. Both are replaced by "push when the bytes change".
const (
	screenPushCoalesce        = 200 * time.Millisecond
	screenReleaseCheckPeriod  = 2 * time.Second
	screenHeartbeatPeriod     = 15 * time.Second
	screenLocationCachePeriod = 2 * time.Second
)

// Positions and device names live in SQLite, while the batch they feed is
// rebuilt on every telemetry event. A very short cache keeps that read off the
// hot path; device.location.changed invalidates it so a manual annotation still
// reaches the wall within one refresh.
//
// Only the handler goroutine touches locations/names/err/at; `stale` is the sole
// field written by the event bus. A cache belongs to one connection, whose
// binding set (and therefore device code set) never changes.
type screenLocationCache struct {
	locations map[string]*store.DeviceLocation
	names     map[string]string
	err       error
	at        time.Time
	stale     atomic.Bool
}

func (c *screenLocationCache) invalidate() {
	if c != nil {
		c.stale.Store(true)
	}
}

func (c *screenLocationCache) read(codes []string) (map[string]*store.DeviceLocation, map[string]string, error) {
	if c == nil {
		locations, err := store.ListDeviceLocationsByCodes(codes)
		names, _ := store.LoadDeviceNames(codes)
		return locations, names, err
	}
	// Clearing the flag before the read keeps an invalidation that lands while
	// the query is in flight.
	fresh := c.stale.Swap(false) || c.at.IsZero() || time.Since(c.at) >= screenLocationCachePeriod
	if !fresh {
		return c.locations, c.names, c.err
	}
	locations, err := store.ListDeviceLocationsByCodes(codes)
	names, _ := store.LoadDeviceNames(codes)
	c.locations, c.names, c.err, c.at = locations, names, err, time.Now()
	return locations, names, err
}

// screenRuntimeBatch always re-reads storage. It is the right choice for a
// one-shot snapshot; the stream uses screenRuntimeBatchLive so a burst of
// telemetry does not turn into a burst of position queries.
func (s *Server) screenRuntimeBatch(doc screen.Envelope, revision string, allowed map[string]bool) map[string]interface{} {
	batch, _ := s.screenRuntimeBatchLive(doc, revision, allowed, nil)
	return batch
}

// The second return value is the next instant at which a "good" sample turns
// "stale". That transition has no event to announce it, so the stream sleeps
// until this deadline instead of recomputing the batch every second.
func (s *Server) screenRuntimeBatchLive(doc screen.Envelope, revision string, allowed map[string]bool, locations *screenLocationCache) (map[string]interface{}, int64) {
	now := time.Now().UnixMilli()
	deadline := int64(0)
	bindings := map[string]screenRuntimeSample{}
	cache := map[string]ScreenDeviceSnapshot{}
	// Reuse the device map's canonical snapshot, including manual locations and
	// telemetry persisted before a restart. Load authorized codes in one query.
	locationCodes := []string{}
	seenLocations := map[string]bool{}
	for id, binding := range doc.Bindings {
		if !allowed[id] || binding.Kind != "location" || binding.LocationSource != "device" {
			continue
		}
		for _, code := range screenBindingDevices(binding) {
			if !seenLocations[code] {
				seenLocations[code] = true
				locationCodes = append(locationCodes, code)
			}
		}
	}
	deviceLocations, deviceNames, locationErr := locations.read(locationCodes)
	for id, binding := range doc.Bindings {
		sample := screenRuntimeSample{Quality: "no_data"}
		if !allowed[id] {
			sample.Quality = "access_denied"
			bindings[id] = sample
			continue
		}
		// Historical data is delivered through the bounded history RPC. Omitting
		// it here prevents live cache updates from erasing the chart's time window.
		if binding.Kind == "history" || binding.Kind == "alarmList" || binding.Kind == "eventList" || binding.Kind == "dataset" {
			continue
		}
		codes := screenBindingDevices(binding)
		sample.Quality = "good"
		values := []float64{}
		locations := []map[string]interface{}{}
		for deviceIndex, code := range codes {
			snapshot, found := cache[code]
			if !found {
				snapshot = ScreenDeviceSnapshot{Samples: map[string]ScreenSample{}}
				if s.DeviceManager != nil {
					snapshot = s.DeviceManager.GetLatestSnapshot(code)
				}
				cache[code] = snapshot
			}
			if binding.Kind == "deviceSummary" {
				continue
			}
			if binding.Kind == "deviceStatus" {
				sample.ReceivedAt, sample.SampleVersion = snapshot.StatusReceivedAt, snapshot.StatusVersion
				sample.Quality = "good"
				sample.Value = "offline"
				if snapshot.StatusReceivedAt == 0 {
					sample.Quality = "no_data"
					sample.Value = "unknown"
				} else if snapshot.Status.Online {
					sample.Value = "online"
				}
				continue
			}
			key := binding.Key
			if key == "" && len(binding.Keys) > 0 {
				key = binding.Keys[0]
			}
			point := snapshot.Samples[key]
			quality := screenSampleQuality(snapshot, point, binding, now)
			if binding.Kind == "location" && binding.LocationSource == "device" {
				point, quality = ScreenSample{}, "no_data"
				if locationErr != nil {
					quality = "error"
				} else if location := deviceLocations[code]; location != nil {
					point.ReceivedAt = location.ReceivedAt
					point.SampleVersion = uint64(max(int64(0), location.ReceivedAt, location.UpdatedAt.UnixMilli()))
					quality = screenSampleQuality(snapshot, point, binding, now)
					// A manual position is a saved installation location, not a
					// telemetry sample; it remains valid while a device is offline.
					if location.Source == store.LocationSourceManual {
						quality = "good"
					}
					if err := ValidateLocationCoordinates(location.Longitude, location.Latitude); err != nil {
						quality = "error"
					} else {
						locations = append(locations, map[string]interface{}{"deviceCode": code, "deviceName": deviceNames[code], "latitude": location.Latitude, "longitude": location.Longitude, "coordinateSystem": "wgs84", "source": location.Source, "quality": quality})
					}
				}
			} else if binding.Kind == "location" {
				lat, lon := snapshot.Samples[binding.LatitudeKey], snapshot.Samples[binding.LongitudeKey]
				point = lat
				if lon.ReceivedAt < lat.ReceivedAt {
					point = lon
				}
				// Freshness uses the older coordinate; order uses the newest sample.
				point.SampleVersion = max(lat.SampleVersion, lon.SampleVersion)
				quality = screenSampleQuality(snapshot, point, binding, now)
				latitude, latOK := screenNumber(lat.Value)
				longitude, lonOK := screenNumber(lon.Value)
				if latOK && lonOK && latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180 {
					crs := binding.CoordinateSystem
					if crs == "" {
						crs = "wgs84"
					}
					locations = append(locations, map[string]interface{}{"deviceCode": code, "deviceName": deviceNames[code], "latitude": latitude, "longitude": longitude, "coordinateSystem": crs, "quality": quality})
				} else if lat.ReceivedAt != 0 && lon.ReceivedAt != 0 {
					quality = "error"
				}
			}
			sample.Quality = screenWorseQuality(sample.Quality, quality)
			if deviceIndex == 0 || point.ReceivedAt < sample.ReceivedAt {
				sample.ReceivedAt = point.ReceivedAt
			}
			// Per-device counters are independent. Their sum advances when any
			// member updates, even if another device has a much larger counter.
			sample.SampleVersion += point.SampleVersion
			if binding.Kind != "location" {
				sample.Value = screenTransform(point.Value, binding)
				if point.Value != nil && sample.Value == nil {
					sample.Quality = screenWorseQuality(sample.Quality, "error")
				}
				if n, ok := screenNumber(point.Value); ok {
					values = append(values, n)
				} else if binding.Kind == "aggregate" && point.ReceivedAt != 0 {
					sample.Quality = screenWorseQuality(sample.Quality, "error")
				}
			}
		}
		if binding.Kind == "aggregate" {
			sample.Value = nil
			if sample.Quality != "error" && len(values) == len(codes) {
				sample.Value = screenTransform(screenAggregate(values, binding.Aggregate), binding)
				if len(values) > 0 && sample.Value == nil {
					sample.Quality = screenWorseQuality(sample.Quality, "error")
				}
			}
		}
		if binding.Kind == "deviceSummary" {
			sample = screenDeviceSummary(codes, cache, binding.Aggregate)
		}
		if binding.Kind == "location" {
			sample.Value = locations
		}
		bindings[id] = sample
		if sample.Quality == "good" {
			if expires := screenSampleExpiry(sample, binding, now); expires > 0 && (deadline == 0 || expires < deadline) {
				deadline = expires
			}
		}
	}
	return map[string]interface{}{"revision": revision, "runtimeEpoch": screenRuntimeEpoch, "serverTime": now, "bindings": bindings}, deadline
}

// Connection summaries use the same authoritative status as deviceStatus.
// Unknown devices remain in the denominator, so a missing report cannot improve
// an availability rate. Counts of observed states are retained but qualified.
func screenDeviceSummary(codes []string, snapshots map[string]ScreenDeviceSnapshot, aggregate string) screenRuntimeSample {
	sample := screenRuntimeSample{Quality: "good"}
	online, offline, unknown := 0, 0, 0
	for _, code := range codes {
		snapshot := snapshots[code]
		sample.SampleVersion += snapshot.StatusVersion
		if snapshot.StatusReceivedAt == 0 {
			unknown++
			continue
		}
		if sample.ReceivedAt == 0 || snapshot.StatusReceivedAt < sample.ReceivedAt {
			sample.ReceivedAt = snapshot.StatusReceivedAt
		}
		if snapshot.Status.Online {
			online++
		} else {
			offline++
		}
	}
	switch aggregate {
	case "total":
		sample.Value = float64(len(codes))
	case "unknown":
		sample.Value = float64(unknown)
	case "online", "offline", "onlineRate":
		if aggregate == "offline" {
			sample.Value = float64(offline)
		} else if aggregate == "online" {
			sample.Value = float64(online)
		} else if online+offline > 0 && len(codes) > 0 {
			sample.Value = 100 * float64(online) / float64(len(codes))
		}
		if unknown > 0 || len(codes) == 0 {
			sample.Quality = "no_data"
		}
	default:
		sample.Quality = "error"
	}
	return sample
}

// screenSampleExpiry reports when a currently healthy sample becomes stale. An
// online/offline status and a manually annotated position do not age out, so
// they contribute no deadline.
func screenSampleExpiry(sample screenRuntimeSample, binding screen.Binding, now int64) int64 {
	if binding.Kind == "deviceStatus" || binding.Kind == "deviceSummary" || sample.ReceivedAt == 0 {
		return 0
	}
	stale := binding.StaleAfterMs
	if stale == 0 {
		stale = 60000
	}
	if expires := sample.ReceivedAt + stale; expires > now {
		return expires
	}
	return 0
}

func screenWorseQuality(a, b string) string {
	order := map[string]int{"good": 0, "stale": 1, "offline": 2, "no_data": 3, "error": 4, "access_denied": 5}
	if order[b] > order[a] {
		return b
	}
	return a
}

func screenAggregate(values []float64, method string) interface{} {
	if len(values) == 0 {
		return nil
	}
	sort.Float64s(values)
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	switch method {
	case "count":
		return float64(len(values))
	case "sum":
		return sum
	case "min":
		return values[0]
	case "max":
		return values[len(values)-1]
	case "median":
		mid := len(values) / 2
		if len(values)%2 == 0 {
			return (values[mid-1] + values[mid]) / 2
		}
		return values[mid]
	default:
		return sum / float64(len(values))
	}
}

func (s *Server) handleScreenRuntimeSnapshot(r *ghttp.Request) {
	row, doc, revision, err := screenAuthorizedDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, s.screenRuntimeBatch(doc, revision, screenAllowedBindings(r, row, doc)))
}

func (s *Server) handleScreenRuntimeHistory(r *ghttp.Request) {
	row, doc, _, err := screenAuthorizedDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var request struct {
		BindingID string `json:"bindingId"`
		StartTime int64  `json:"startTime"`
		EndTime   int64  `json:"endTime"`
		MaxPoints int    `json:"maxPoints"`
	}
	if err = screenReadJSON(r, &request); err != nil {
		screenWriteError(r, err)
		return
	}
	binding, exists := doc.Bindings[request.BindingID]
	if !exists || (binding.Kind != "history" && binding.Kind != "eventList") {
		screenWriteError(r, screenFail(400, "Unknown history binding / 未知历史数据绑定"))
		return
	}
	if !screenAllowedBindings(r, row, doc)[request.BindingID] {
		screenWriteData(r, map[string]interface{}{"bindingId": request.BindingID, "value": []interface{}{}, "quality": "access_denied"})
		return
	}
	sample, err := s.screenReadBindingHistory(r.Context(), row, binding, request.StartTime, request.EndTime, request.MaxPoints)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"bindingId": request.BindingID, "value": sample.Value, "quality": sample.Quality, "receivedAt": sample.ReceivedAt})
}

// Both saved runtime and detached proposal previews share range limits and transforms.
// The caller authorizes the binding before entering this read-only function.
func (s *Server) screenReadBindingHistory(ctx context.Context, row *store.IOCScreen, binding screen.Binding, start, end int64, limit int) (screenRuntimeSample, error) {
	empty := screenRuntimeSample{}
	codes := screenBindingDevices(binding)
	if binding.Kind == "history" && len(codes) != 1 {
		return empty, screenFail(400, "History requires one device / 历史绑定仅支持单设备")
	}
	now := time.Now().UnixMilli()
	if end == 0 {
		end = now
	}
	window := binding.WindowMs
	if window == 0 {
		window = 3600000
	}
	if start == 0 {
		start = end - window
	}
	if start < 0 || end < start || end > now+60000 || end-start > 31*86400000 {
		return empty, screenFail(400, "History time range exceeds limits / 历史查询时间范围超限")
	}
	if limit <= 0 {
		limit = binding.MaxPoints
	}
	if limit <= 0 {
		limit = 300
	}
	if limit > 1000 {
		limit = 1000
	}
	if binding.MaxPoints > 0 && limit > binding.MaxPoints {
		limit = binding.MaxPoints
	}
	if binding.Kind == "eventList" {
		result, queryErr := s.screenEventRows(ctx, row, binding, start, end, limit)
		if queryErr != nil {
			return empty, screenFail(503, "Event history unavailable / 事件历史暂不可用")
		}
		quality := "good"
		if len(result) == 0 {
			quality = "no_data"
		}
		return screenRuntimeSample{Value: result, Quality: quality, ReceivedAt: now}, nil
	}
	keys := screenBindingKeys(binding)
	method := binding.Aggregate
	if method == "" {
		method = "avg"
	}
	if method == "sum" || method == "count" {
		return empty, screenFail(400, "Unsupported history aggregation / 不支持此历史聚合方式")
	}
	result := []map[string]interface{}{}
	if s.TSDB != nil {
		response, queryErr := s.TSDB.Query(tsdb.QueryRequest{Context: ctx, DeviceCode: codes[0], StartTime: start, EndTime: end, Type: tsdb.TypeTelemetry, Keys: keys, Aggregate: true, MaxPoints: limit, AggMethod: method, Page: 1, PageSize: limit})
		if queryErr != nil {
			return empty, screenFail(503, "History service unavailable / 历史服务暂不可用")
		}
		if response != nil {
			for _, entry := range response.List {
				data, ok := entry.(map[string]interface{})
				if !ok {
					continue
				}
				filtered := map[string]interface{}{"ts": data["ts"]}
				for _, key := range keys {
					if value, ok := data[key]; ok {
						filtered[key] = screenTransform(value, binding)
					}
				}
				result = append(result, filtered)
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, _ := screenNumber(result[i]["ts"])
		b, _ := screenNumber(result[j]["ts"])
		return a < b
	})
	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	quality := "good"
	if len(result) == 0 {
		quality = "no_data"
	}
	return screenRuntimeSample{Value: result, Quality: quality, ReceivedAt: now}, nil
}

func (s *Server) handleScreenBindingCatalog(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	// The picker is a paged directory, independent of definition-only AI context.
	// Browsing devices must never implicitly select them for a generation task.
	search := strings.ToLower(strings.TrimSpace(r.GetQuery("search").String()))
	var deviceCount, matched int64
	devices := []screenAssistantDevice{}
	if auth := requestAuthContext(r); auth != nil && auth.HasProjectPermission("device:list", row.ProjectID) {
		query := store.DB.WithContext(r.Context()).Model(&store.Device{}).Where("tenant_id = ? AND project_id = ?", row.TenantID, row.ProjectID)
		if err := query.Count(&deviceCount).Error; err != nil {
			screenWriteError(r, screenFail(503, "Device catalogue unavailable / 设备目录暂不可用"))
			return
		}
		if search != "" {
			escaped := strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(search)
			query = query.Where("LOWER(name) LIKE ? ESCAPE '!' OR LOWER(code) LIKE ? ESCAPE '!'", "%"+escaped+"%", "%"+escaped+"%")
		}
		var page []store.Device
		if err := query.Count(&matched).Select("code").Order("code ASC").Limit(500).Find(&page).Error; err != nil {
			screenWriteError(r, screenFail(503, "Device catalogue unavailable / 设备目录暂不可用"))
			return
		}
		codes := make([]string, 0, len(page))
		for _, device := range page {
			codes = append(codes, device.Code)
		}
		devices, _, err = s.screenAssistantCatalog(r, row, screen.DefaultDocument(), &codes)
		if err != nil {
			screenWriteError(r, err)
			return
		}
	}
	products, err := screenAssistantBindingProducts(r, row, devices)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	result := []interface{}{}
	for _, device := range devices {
		fields := []interface{}{}
		for index, field := range device.Fields {
			if index >= 64 {
				break
			}
			fields = append(fields, map[string]interface{}{
				"key": field.Key, "name": field.Zh, "nameZh": field.Zh, "nameEn": field.En,
				"type": field.Type, "unit": field.Unit, "role": field.Role,
			})
		}
		result = append(result, map[string]interface{}{
			"code": device.Code, "name": device.Name, "productCode": device.ProductCode,
			"productName": device.ProductName, "state": device.State,
			"online": device.State == "online", "fields": fields,
		})
	}
	screenWriteData(r, map[string]interface{}{"devices": result, "products": products, "limit": 500,
		"deviceCount": deviceCount, "matchedDeviceCount": matched, "truncated": matched > int64(len(result))})
}

// Long-lived requests cannot keep an authorization snapshot indefinitely.
func (s *Server) screenRefreshStreamAuth(r *ghttp.Request, row *store.IOCScreen) bool {
	if s.Config == nil {
		return false
	}
	parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	claims, err := utils.ParseToken(parts[1], s.Config.Auth.JWTSecret)
	if err != nil || claims.TokenUse == "refresh" || isTokenBlacklisted(parts[1]) || !activeTokenSession(claims) {
		return false
	}
	var auth *AuthContext
	if claims.SubjectType == "app" {
		auth, err = resolveAppAuthContextFromClaims(claims, row.ProjectID)
	} else {
		auth, err = ResolveUserAuthContext(claims.UserID, row.TenantID, row.ProjectID)
	}
	if err != nil {
		return false
	}
	if _, _, err = screenResolveScope(auth, row.TenantID, row.ProjectID); err != nil || !screenHasPermission(auth, row.ProjectID, "screen:view") {
		return false
	}
	if r.GetQuery("revision").String() == "draft" && !screenHasPermission(auth, row.ProjectID, "screen:edit") {
		return false
	}
	r.SetCtxVar(authContextKey, auth)
	return true
}

func (s *Server) handleScreenRuntimeStream(r *ghttp.Request) {
	row, doc, revision, err := screenAuthorizedDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if s.DeviceManager == nil || s.DeviceManager.EventBus == nil {
		screenWriteError(r, screenFail(503, "Runtime service unavailable / 实时服务暂不可用"))
		return
	}
	allowed := screenAllowedBindings(r, row, doc)
	codes := map[string]bool{}
	for _, binding := range doc.Bindings {
		for _, code := range screenBindingDevices(binding) {
			codes[code] = true
		}
	}
	hints := make(chan struct{}, 1)
	locations := &screenLocationCache{}
	var overflow atomic.Bool
	wake := func() {
		select {
		case hints <- struct{}{}:
		default:
			overflow.Store(true)
		}
	}
	// Subscribe before taking the first snapshot. The callback never blocks the
	// device ingestion path; overflow forces a fresh full snapshot.
	subscription := s.DeviceManager.EventBus.SubscribeWithID(screenRuntimeDeviceUpdated, func(e types.Event) {
		if codes[e.Topic] {
			wake()
		}
	})
	defer s.DeviceManager.EventBus.Unsubscribe(screenRuntimeDeviceUpdated, subscription)
	// A moved device must repaint the map even when no telemetry arrives.
	locationSubscription := s.DeviceManager.EventBus.SubscribeWithID(types.EventDeviceLocationChanged, func(e types.Event) {
		if codes[e.Topic] {
			locations.invalidate()
			wake()
		}
	})
	defer s.DeviceManager.EventBus.Unsubscribe(types.EventDeviceLocationChanged, locationSubscription)
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "private, no-store")
	r.Response.Header().Set("X-Accel-Buffering", "no")
	r.Response.Header().Set("X-Content-Type-Options", "nosniff")
	send := func(event string, payload interface{}) bool {
		data, err := json.Marshal(payload)
		if err != nil {
			// Close the stream so the client reconnects and resynchronizes,
			// instead of emitting an empty data frame followed by heartbeats.
			return false
		}
		r.Response.Write(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))
		r.Response.Flush()
		return true
	}
	// The payload is only resent when its bindings actually differ from the one
	// already on screen. A silent screen therefore costs nothing, and the quality
	// deadline covers the single transition no event announces (good -> stale).
	var lastBindings []byte
	var deadline int64
	push := func(event string) bool {
		batch, next := s.screenRuntimeBatchLive(doc, revision, allowed, locations)
		bindings, err := json.Marshal(batch["bindings"])
		if err != nil {
			return false
		}
		deadline = next
		if event == "patch" && bytes.Equal(bindings, lastBindings) {
			return true
		}
		lastBindings = bindings
		return send(event, batch)
	}
	if !push("snapshot") {
		return
	}
	ticker := time.NewTicker(screenPushCoalesce)
	defer ticker.Stop()
	check := time.NewTicker(screenReleaseCheckPeriod)
	defer check.Stop()
	heartbeat := time.NewTicker(screenHeartbeatPeriod)
	defer heartbeat.Stop()
	dirty := false
	for {
		select {
		case <-r.Context().Done():
			return
		case <-hints:
			dirty = true
		case <-ticker.C:
			if !dirty && (deadline == 0 || time.Now().UnixMilli() < deadline) {
				continue
			}
			event := "patch"
			if overflow.Swap(false) {
				event = "snapshot"
			}
			if !push(event) {
				return
			}
			dirty = false
		case <-check.C:
			var current store.IOCScreen
			if store.DB.Select("id", "tenant_id", "project_id", "draft_version", "published_release_id").Where("id = ? AND tenant_id = ? AND project_id = ?", row.ID, row.TenantID, row.ProjectID).First(&current).Error != nil {
				send("access_revoked", map[string]string{"reason": "unavailable"})
				return
			}
			if strings.HasPrefix(revision, "draft-") {
				if current.DraftVersion != row.DraftVersion {
					send("release_changed", map[string]string{"reason": "draft_changed"})
					return
				}
			} else if !screenHasPermission(requestAuthContext(r), row.ProjectID, "screen:edit") || r.GetQuery("revision").String() == "published" || r.GetQuery("revision").String() == "" {
				if current.PublishedReleaseID == nil {
					send("access_revoked", map[string]string{"reason": "unpublished"})
					return
				}
				if strconv.FormatUint(uint64(*current.PublishedReleaseID), 10) != revision {
					send("release_changed", map[string]string{"reason": "published"})
					return
				}
			}
		case <-heartbeat.C:
			if !s.screenRefreshStreamAuth(r, row) {
				send("access_revoked", map[string]string{"reason": "authorization"})
				return
			}
			allowed = screenAllowedBindings(r, row, doc)
			locations.invalidate()
			if !send("heartbeat", map[string]interface{}{"serverTime": time.Now().UnixMilli()}) {
				return
			}
			// Re-evaluated permissions may change a binding to access_denied.
			dirty = true
		}
	}
}
