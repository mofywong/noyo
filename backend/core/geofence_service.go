package core

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"noyo/core/store"
	"noyo/core/types"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// 围栏列表的缓存存活时间。判定发生在每次位置落库之后，而位置的落库已被
// 心跳间隔节流（30s），所以这个缓存主要避免"每设备每围栏一次全表查"。
const geofenceCacheTTL = 10 * time.Second

// 迟滞推进循环的节拍。
//
// 迟滞是**时间**维度的"必须持续满足 N 秒"，但位置上报是**事件**维度的。
// 只在位置写入时推进，就要求"每个 dwell 周期内至少再来一次上报"——
// 遥测设备满足，人工标注只写一次，于是 pending 永远悬着、告警永远不出。
// 所以必须有一个按时间跑的推进器。
const geofenceDwellInterval = 10 * time.Second

// GeofenceService 负责围栏的空间判定与状态机推进。
//
// ⚠️ 核心设计：**判定必须有状态 + 迟滞**。GPS 漂移会让 inside/outside 秒级反复翻转，
// 不加迟滞的围栏功能上线后最常见的投诉就是"告警疯狂抖动"。
type GeofenceService struct {
	server *Server

	fenceMu  sync.RWMutex
	fences   []fenceEntry
	loadedAt time.Time
}

// fenceEntry 是围栏的**判定视图**：定义 + 已解析几何 + 绑定设备集合。
//
// 几何必须在这里解析一次并缓存。行政区边界是上万个顶点的 MultiPolygon，
// 每次位置落库都重新 Unmarshal 一遍会让遥测链路明显变慢。
type fenceEntry struct {
	fence store.Geofence
	// shape 仅 polygon 围栏有值；circle 走距离判定，不需要解析几何。
	shape *FenceShape
	// assigned 为空表示未绑定任何设备 ⇒ 沿用旧行为，对全部已定位设备生效。
	assigned map[string]bool
}

// appliesTo 判断该围栏是否应当评估给定设备。
func (e *fenceEntry) appliesTo(deviceCode string) bool {
	if len(e.assigned) == 0 {
		return true
	}
	return e.assigned[deviceCode]
}

// NewGeofenceService 构造围栏服务。
func NewGeofenceService(server *Server) *GeofenceService {
	return &GeofenceService{server: server}
}

// InvalidateCache 让围栏缓存立刻失效（围栏增删改后调用）。
func (s *GeofenceService) InvalidateCache() {
	if s == nil {
		return
	}
	s.fenceMu.Lock()
	defer s.fenceMu.Unlock()
	s.loadedAt = time.Time{}
}

func (s *GeofenceService) enabledFences() []fenceEntry {
	if s == nil {
		return nil
	}
	s.fenceMu.RLock()
	if !s.loadedAt.IsZero() && time.Since(s.loadedAt) < geofenceCacheTTL {
		entries := s.fences
		s.fenceMu.RUnlock()
		return entries
	}
	s.fenceMu.RUnlock()

	rows, err := store.ListEnabledGeofences()
	if err != nil {
		if s.server != nil && s.server.Logger != nil {
			s.server.Logger.Warn("Failed to list geofences", zap.Error(err))
		}
		return nil
	}
	assignments, err := store.ListGeofenceDeviceMap()
	if err != nil {
		if s.server != nil && s.server.Logger != nil {
			s.server.Logger.Warn("Failed to list geofence assignments", zap.Error(err))
		}
		assignments = map[uint]map[string]bool{}
	}

	entries := make([]fenceEntry, 0, len(rows))
	for i := range rows {
		entry := fenceEntry{fence: rows[i], assigned: assignments[rows[i].ID]}
		if rows[i].Kind == store.FenceKindPolygon {
			shape, parseErr := ParseFenceShape(rows[i].Geometry)
			if parseErr != nil {
				// 几何坏了不能让整批围栏失效：跳过这一条，其余照常判定。
				if s.server != nil && s.server.Logger != nil {
					s.server.Logger.Warn("Skipped geofence with invalid geometry",
						zap.Uint("fence", rows[i].ID), zap.String("name", rows[i].Name), zap.Error(parseErr))
				}
				continue
			}
			entry.shape = shape
		}
		entries = append(entries, entry)
	}

	s.fenceMu.Lock()
	s.fences = entries
	s.loadedAt = time.Now()
	s.fenceMu.Unlock()
	return entries
}

// GeofenceTransition 是一次状态跃迁的描述。
type GeofenceTransition struct {
	DeviceCode string  `json:"device_code"`
	FenceID    uint    `json:"fence_id"`
	FenceName  string  `json:"fence_name"`
	RuleType   string  `json:"rule_type"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Longitude  float64 `json:"longitude"`
	Latitude   float64 `json:"latitude"`
	At         int64   `json:"at"`
}

// EvaluateDevice 对一台设备的新位置做全量围栏判定。
//
// 由 LocationService 在位置落库后调用。任何错误都不向上返回：
// 围栏判定不能影响位置写入这条主链路。
func (s *GeofenceService) EvaluateDevice(deviceCode string, loc *store.DeviceLocation) {
	if s == nil || deviceCode == "" || loc == nil {
		return
	}
	entries := s.enabledFences()
	if len(entries) == 0 {
		return
	}

	accuracy := store.ManualAccuracyDefaultM
	if loc.AccuracyM != nil && *loc.AccuracyM > 0 {
		accuracy = *loc.AccuracyM
	}

	for i := range entries {
		entry := entries[i]
		// 绑定过滤：围栏若指定了设备，就对其他设备完全无感 —— 不做判定、
		// 不落状态，地图上也不会为无关设备点亮告警。
		if !entry.appliesTo(deviceCode) {
			continue
		}
		observed, uncertain := observeFenceState(&entry, loc.Longitude, loc.Latitude, accuracy)
		if err := s.advanceState(deviceCode, &entry.fence, observed, uncertain, loc); err != nil {
			if s.server != nil && s.server.Logger != nil {
				s.server.Logger.Warn("Failed to advance geofence state",
					zap.String("device", deviceCode), zap.Uint("fence", entry.fence.ID), zap.Error(err))
			}
		}
	}
}

// RunDwellLoop 每 geofenceDwellInterval 推进一次到期的待确认状态。
//
// 这是"时间驱动"的那一半：位置上报只负责把观测态写进 pending_state，
// 真正到期提交由这个循环完成。没有它，一次人工标注改完位置之后就再也没人
// 推进状态机，dwell 永远走不完。
func (s *GeofenceService) RunDwellLoop() {
	if s == nil {
		return
	}
	ticker := time.NewTicker(geofenceDwellInterval)
	defer ticker.Stop()
	for range ticker.C {
		s.AdvanceDueDwell()
	}
}

// AdvanceDueDwell 推进所有已到期（或已不再成立）的待确认状态。导出以便单测直接调。
func (s *GeofenceService) AdvanceDueDwell() {
	if s == nil || s.server == nil || store.DB == nil {
		return
	}
	pending, err := store.ListPendingFenceStates()
	if err != nil {
		s.logWarn("Failed to list pending geofence states", zap.Error(err))
		return
	}
	if len(pending) == 0 {
		return
	}

	// 一次 tick 只加载一次围栏视图：行政区边界解析很贵，绝不能按记录重复加载。
	entries := s.enabledFences()
	byID := make(map[uint]*fenceEntry, len(entries))
	for i := range entries {
		byID[entries[i].fence.ID] = &entries[i]
	}

	now := time.Now().UnixMilli()
	for i := range pending {
		st := &pending[i]
		entry, ok := byID[st.FenceID]
		if !ok {
			// 围栏被删/被禁用：状态行由删除路径清理，这里不负责。
			continue
		}
		if !entry.appliesTo(st.DeviceCode) {
			// 设备被解绑了，这条 pending 不会再有人处理，直接放弃。
			st.PendingState = ""
			st.PendingSince = 0
			_ = store.UpsertDeviceFenceState(st)
			continue
		}

		dwellSeconds := entry.fence.ExitDwellS
		if st.PendingState == store.FenceStateInside {
			dwellSeconds = entry.fence.EnterDwellS
		}
		if dwellSeconds < 0 {
			dwellSeconds = 0
		}
		// 未到期：什么都不做（也不要写库，避免每 10s 刷一遍 last_eval_at）。
		if st.PendingSince <= 0 || now-st.PendingSince < int64(dwellSeconds)*1000 {
			continue
		}

		loc, err := store.GetDeviceLocation(st.DeviceCode)
		if err != nil || loc == nil {
			continue
		}

		accuracy := store.ManualAccuracyDefaultM
		if loc.AccuracyM != nil && *loc.AccuracyM > 0 {
			accuracy = *loc.AccuracyM
		}
		// 用**最新位置**复核一次，而不是拿 PendingState 直接提交：
		// 这期间设备可能已经回到围栏内，那样提交就是一次假告警。
		observed, uncertain := observeFenceState(entry, loc.Longitude, loc.Latitude, accuracy)

		if err := s.advanceState(st.DeviceCode, &entry.fence, observed, uncertain, loc); err != nil {
			s.logWarn("Failed to advance due geofence state",
				zap.String("device", st.DeviceCode), zap.Uint("fence", st.FenceID), zap.Error(err))
		}
	}
}

func (s *GeofenceService) logWarn(message string, fields ...zap.Field) {
	if s != nil && s.server != nil && s.server.Logger != nil {
		s.server.Logger.Warn(message, fields...)
	}
}

// ClearDevice 清除某设备的全部围栏判定状态（位置被清除时调用）。
func (s *GeofenceService) ClearDevice(deviceCode string) {
	if s == nil || deviceCode == "" {
		return
	}
	_ = store.DB.Transaction(func(tx *gorm.DB) error {
		return store.DeleteFenceStatesForDevice(tx, deviceCode)
	})
}

// observeFenceState 判断点相对围栏的观测态。
//
// uncertain=true 表示点落在边界 ±accuracy 的不确定带内 —— 此时**不推进状态机**，
// 这是对抗 GPS 漂移的第一道防线（第二道是时间迟滞）。
func observeFenceState(entry *fenceEntry, lng, lat, accuracy float64) (string, bool) {
	if entry == nil {
		return store.FenceStateUnknown, true
	}
	fence := &entry.fence
	switch fence.Kind {
	case store.FenceKindCircle:
		if fence.CenterLng == nil || fence.CenterLat == nil || fence.RadiusM == nil {
			return store.FenceStateUnknown, true
		}
		distance := haversineMeters(*fence.CenterLng, *fence.CenterLat, lng, lat)
		radius := *fence.RadiusM
		if absFloat(distance-radius) <= accuracy {
			return store.FenceStateUnknown, true
		}
		if distance <= radius {
			return store.FenceStateInside, false
		}
		return store.FenceStateOutside, false

	case store.FenceKindPolygon:
		shape := entry.shape
		if shape.IsEmpty() {
			return store.FenceStateUnknown, true
		}
		minLng, minLat, maxLng, maxLat := shape.BBox()
		// 用「按精度膨胀后的包围盒」做 O(1) 排除：盒外必然在围栏外，
		// 且到边界的距离一定大于 accuracy ⇒ 不存在不确定带。
		cosLat := math.Cos(lat * math.Pi / 180)
		if math.Abs(cosLat) < 1e-6 {
			cosLat = 1e-6
		}
		padLat := accuracy / metersPerDegreeLat
		padLng := accuracy / (metersPerDegreeLat * math.Abs(cosLat))
		if lng < minLng-padLng || lng > maxLng+padLng || lat < minLat-padLat || lat > maxLat+padLat {
			return store.FenceStateOutside, false
		}
		if shape.DistanceToBoundaryMeters(lng, lat) <= accuracy {
			return store.FenceStateUnknown, true
		}
		if shape.Contains(lng, lat) {
			return store.FenceStateInside, false
		}
		return store.FenceStateOutside, false

	default:
		return store.FenceStateUnknown, true
	}
}

// advanceState 推进单台设备对单个围栏的状态机。
func (s *GeofenceService) advanceState(deviceCode string, fence *store.Geofence, observed string, uncertain bool, loc *store.DeviceLocation) error {
	state, err := store.GetDeviceFenceState(deviceCode, fence.ID)
	if err != nil {
		return err
	}
	if state == nil {
		state = &store.DeviceFenceState{
			DeviceCode: deviceCode,
			FenceID:    fence.ID,
			State:      store.FenceStateUnknown,
		}
	}

	now := time.Now().UnixMilli()
	state.LastEvalAt = now
	state.LastFixAt = loc.FixAt

	// 不确定带内：保持现有状态，不推进也不清零历史（清 pending 避免"悬空待确认"）
	if uncertain || observed == store.FenceStateUnknown {
		if state.PendingState != "" {
			state.PendingState = ""
			state.PendingSince = 0
			return store.UpsertDeviceFenceState(state)
		}
		return store.UpsertDeviceFenceState(state)
	}

	// 观测态与当前状态一致：清掉待确认
	if state.State == observed {
		if state.PendingState != "" {
			state.PendingState = ""
			state.PendingSince = 0
		}
		return store.UpsertDeviceFenceState(state)
	}

	// 需要迟滞：目标态需要持续满足 dwell 秒才提交
	dwellSeconds := fence.ExitDwellS
	if observed == store.FenceStateInside {
		dwellSeconds = fence.EnterDwellS
	}

	if state.PendingState != observed {
		state.PendingState = observed
		state.PendingSince = now
		if dwellSeconds == 0 {
			return s.commitTransition(deviceCode, fence, state, observed, loc)
		}
		return store.UpsertDeviceFenceState(state)
	}

	if now-state.PendingSince < int64(dwellSeconds)*1000 {
		return store.UpsertDeviceFenceState(state)
	}

	return s.commitTransition(deviceCode, fence, state, observed, loc)
}

// commitTransition 提交一次状态跃迁，并在"确实是进出事件"时发布事件。
//
// from=unknown 时不触发事件：这是状态的**初始化**（新增设备、新增围栏、平台重启后
// 首次判定），不是"进入/离开"。若把它当作进入事件，平台一重启所有在围栏内的设备
// 都会同时告警 —— 这是必须避免的告警风暴。
func (s *GeofenceService) commitTransition(deviceCode string, fence *store.Geofence, state *store.DeviceFenceState, observed string, loc *store.DeviceLocation) error {
	previous := state.State
	now := time.Now().UnixMilli()
	state.State = observed
	state.Since = now
	state.PendingState = ""
	state.PendingSince = 0
	if err := store.UpsertDeviceFenceState(state); err != nil {
		return err
	}
	if previous != store.FenceStateUnknown && previous != observed {
		s.emitTransition(deviceCode, fence, previous, observed, loc)
	}
	// 告警与"是否首次判定"分开处理：围栏事件用于联动（规则引擎/SSE），
	// 告警是给人看的处置对象。绑定设备在绑定那一刻就已经在违规侧时，
	// 运营者需要知道这件事，因此 unknown→违规 也要落地一条告警实例。
	s.syncFenceAlarm(deviceCode, fence, previous, observed, loc)
	return nil
}

// fenceViolated 判断某个判定状态对该围栏规则而言是否构成违规。
//
// 只有 forbid（进入禁区）与 keep_in（走出指定范围）算违规 —— 这两个是"越界"，
// 必须出告警。notify 只是"进出提示"，只在地图上点亮状态色，不产生告警，
// 否则一个通知型围栏会让告警中心常年有活。
func fenceViolated(ruleType, state string) bool {
	switch ruleType {
	case store.FenceRuleForbid:
		return state == store.FenceStateInside
	case store.FenceRuleKeepIn:
		return state == store.FenceStateOutside
	default:
		return false
	}
}

// syncFenceAlarm 在违规边界跃迁时投递告警，回到合规侧时恢复。
//
// 复用既有的设备事件 → 告警实例链路（ReportDeviceEvent），不另起一套告警体系：
// 这样告警中心、工单联动、通知策略对围栏告警一视同仁。
// fingerprint 用 `geofence:<id>`，告警实例按 (设备, fingerprint) 去重 ——
// 同一条围栏的同一次越界只会有一条活动告警，重复投递只累加发生次数。
func (s *GeofenceService) syncFenceAlarm(deviceCode string, fence *store.Geofence, previous, observed string, loc *store.DeviceLocation) {
	if s == nil || s.server == nil || s.server.DeviceManager == nil || fence == nil {
		return
	}
	wasViolating := fenceViolated(fence.RuleType, previous)
	nowViolating := fenceViolated(fence.RuleType, observed)
	if wasViolating == nowViolating {
		return
	}

	// 围栏告警不能反过来影响位置写入主链路：任何 panic 都必须被吞掉。
	defer func() {
		if recovered := recover(); recovered != nil && s.server.Logger != nil {
			s.server.Logger.Warn("Panic while raising geofence alarm",
				zap.String("device", deviceCode), zap.Uint("fence", fence.ID), zap.Any("panic", recovered))
		}
	}()

	meta, _, err := s.server.DeviceManager.GetDeviceMeta(deviceCode)
	if err != nil || meta == nil {
		if s.server.Logger != nil {
			s.server.Logger.Warn("Geofence alarm skipped: device metadata unavailable",
				zap.String("device", deviceCode), zap.Error(err))
		}
		return
	}

	phase := "triggered"
	if !nowViolating {
		phase = "recovered"
	}
	payload := map[string]interface{}{
		"alarm_fingerprint": fmt.Sprintf("geofence:%d", fence.ID),
		"alarm_phase":       phase,
		"alarm_severity":    "critical",
		// 独立的来源类型：告警中心据此把围栏告警与普通设备事件分开显示，
		// 告警策略也能用 source_type=geofence 单独控制级别/工单要求。
		"alarm_source_type": AlarmSourceGeofence,
		// 标题用围栏名（用户自定义、单一语言），方向与规则放在参数里由前端本地化，
		// 避免在告警正文里拼出中英混排的句子。
		"alarm_title": fence.Name,
		"title":       fence.Name,
		"content":     fmt.Sprintf("%.6f, %.6f", loc.Longitude, loc.Latitude),
		"deviceCode":  deviceCode,
		"eventId":     "geofence_breach",
		"fence_id":    fence.ID,
		"fence_name":  fence.Name,
		"rule_type":   fence.RuleType,
		"from":        previous,
		"to":          observed,
		"longitude":   loc.Longitude,
		"latitude":    loc.Latitude,
		"at":          time.Now().UnixMilli(),
	}
	if err := s.server.DeviceManager.ReportDeviceEvent(*meta, "geofence_breach", payload); err != nil && s.server.Logger != nil {
		s.server.Logger.Warn("Failed to report geofence alarm",
			zap.String("device", deviceCode), zap.Uint("fence", fence.ID), zap.Error(err))
	}
}

// emitTransition 发布围栏状态跃迁事件。
//
// 后续接入既有规则引擎作为一等触发源（复用条件/动作/去重链路），
// 避免出现两套并行的告警体系。
func (s *GeofenceService) emitTransition(deviceCode string, fence *store.Geofence, from, to string, loc *store.DeviceLocation) {
	if s == nil || s.server == nil || s.server.DeviceManager == nil {
		return
	}
	transition := GeofenceTransition{
		DeviceCode: deviceCode,
		FenceID:    fence.ID,
		FenceName:  fence.Name,
		RuleType:   fence.RuleType,
		From:       from,
		To:         to,
		Longitude:  loc.Longitude,
		Latitude:   loc.Latitude,
		At:         time.Now().UnixMilli(),
	}
	if s.server.Logger != nil {
		s.server.Logger.Info("Geofence state changed",
			zap.String("device", deviceCode),
			zap.String("fence", fence.Name),
			zap.String("from", from),
			zap.String("to", to))
	}
	s.server.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventGeofenceTriggered,
		Topic:     deviceCode,
		Payload:   transition,
		Timestamp: transition.At,
	})
}

// ===================== 围栏校验 =====================

// ErrInvalidGeofence 表示围栏定义非法。
var ErrInvalidGeofence = errors.New("invalid geofence")

// NormalizeGeofence 校验并补全围栏定义（几何合法性、包围盒、默认值）。
func NormalizeGeofence(fence *store.Geofence) error {
	if fence == nil {
		return fmt.Errorf("%w: nil", ErrInvalidGeofence)
	}
	if fence.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidGeofence)
	}
	switch fence.Kind {
	case store.FenceKindCircle:
		// 圆形围栏已从前端移除（只保留多边形），这里保留解析能力仅为了
		// 兼容历史数据：旧围栏仍要能显示、能判定、能被编辑改成多边形。
		if fence.CenterLng == nil || fence.CenterLat == nil || fence.RadiusM == nil {
			return fmt.Errorf("%w: circle needs center and radius", ErrInvalidGeofence)
		}
		if err := ValidateLocationCoordinates(*fence.CenterLng, *fence.CenterLat); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidGeofence, err)
		}
		if *fence.RadiusM <= 0 {
			return fmt.Errorf("%w: radius must be positive", ErrInvalidGeofence)
		}
		minLng, minLat, maxLng, maxLat := CircleBBox(*fence.CenterLng, *fence.CenterLat, *fence.RadiusM)
		fence.BboxMinLng, fence.BboxMinLat = &minLng, &minLat
		fence.BboxMaxLng, fence.BboxMaxLat = &maxLng, &maxLat
		fence.Geometry = ""

	case store.FenceKindPolygon:
		// 多边形可以是用户手绘的单环，也可以是按行政区名称取回来的
		// MultiPolygon（含洞），统一交给 FenceShape 校验。
		shape, err := ParseFenceShape(fence.Geometry)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidGeofence, err)
		}
		minLng, minLat, maxLng, maxLat := shape.BBox()
		if err := ValidateLocationCoordinates(minLng, minLat); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidGeofence, err)
		}
		if err := ValidateLocationCoordinates(maxLng, maxLat); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidGeofence, err)
		}
		fence.BboxMinLng, fence.BboxMinLat = &minLng, &minLat
		fence.BboxMaxLng, fence.BboxMaxLat = &maxLng, &maxLat
		fence.CenterLng, fence.CenterLat, fence.RadiusM = nil, nil, nil

	default:
		return fmt.Errorf("%w: unsupported kind %q", ErrInvalidGeofence, fence.Kind)
	}

	switch fence.Source {
	case "", store.FenceSourceManual:
		fence.Source = store.FenceSourceManual
		fence.RegionName = ""
		fence.RegionLevel = ""
	case store.FenceSourceRegion:
		if strings.TrimSpace(fence.RegionName) == "" {
			return fmt.Errorf("%w: region geofence needs a region name", ErrInvalidGeofence)
		}
	default:
		return fmt.Errorf("%w: unsupported source %q", ErrInvalidGeofence, fence.Source)
	}

	switch fence.RuleType {
	case store.FenceRuleForbid, store.FenceRuleKeepIn, store.FenceRuleNotify:
	case "":
		fence.RuleType = store.FenceRuleNotify
	default:
		return fmt.Errorf("%w: unsupported rule type %q", ErrInvalidGeofence, fence.RuleType)
	}

	if fence.EnterDwellS < 0 || fence.ExitDwellS < 0 {
		return fmt.Errorf("%w: dwell seconds must be >= 0", ErrInvalidGeofence)
	}
	return nil
}

// CreateGeofence 校验后新建围栏。
func (s *GeofenceService) CreateGeofence(fence *store.Geofence) error {
	if err := NormalizeGeofence(fence); err != nil {
		return err
	}
	if err := store.CreateGeofence(fence); err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}

// UpdateGeofence 校验后更新围栏。
//
// **不再删除判定状态**：删掉会让所有绑定设备回到 unknown，下一次判定变成
// "初始化"，于是几何微调一次就可能整批闪出/吞掉告警。保留旧裁决反而更准确 ——
// 新几何算出的裁决与旧裁决不同时，状态机会正常产生一次跃迁和一条告警。
func (s *GeofenceService) UpdateGeofence(fence *store.Geofence) error {
	if err := NormalizeGeofence(fence); err != nil {
		return err
	}
	if err := store.UpdateGeofence(fence); err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}

// DeleteGeofence 删除围栏（连带清理判定状态）。
func (s *GeofenceService) DeleteGeofence(id uint) error {
	if err := store.DeleteGeofence(id); err != nil {
		return err
	}
	s.InvalidateCache()
	return nil
}
