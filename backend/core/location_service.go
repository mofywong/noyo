package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"noyo/core/store"
	"noyo/core/types"

	"go.uber.org/zap"
)

// 语义解析缓存的存活时间。物模型变更不是实时语义，60s 的滞后可以接受；
// 同时在产品更新时提供显式失效（InvalidateProductSemantics），保证配置改动立刻生效。
const locationSemanticsTTL = 60 * time.Second

// 位置未变化时的最小落库间隔（心跳写）。避免 1s 轮询的设备把 SQLite 写成热点，
// 同时保证"最后定位时间"仍以可接受的粒度保持新鲜。
const locationHeartbeatInterval = 30 * time.Second

type semanticsCacheEntry struct {
	sem      LocationSemantics
	loadedAt time.Time
}

type lastLocationWrite struct {
	longitude float64
	latitude  float64
	at        int64 // ms
}

// LocationService 负责设备位置的写入校验、快照落库与查询。
//
// 它是**所有**位置写入的唯一入口，无论来源是遥测属性还是人工标注 ——
// 这样范围校验与哨兵识别只需实现一次。
type LocationService struct {
	server *Server

	semMu    sync.RWMutex
	semCache map[string]semanticsCacheEntry // productCode -> semantics

	writeMu   sync.Mutex
	lastWrite map[string]lastLocationWrite // deviceCode -> last persisted
}

// NewLocationService 构造定位服务。
func NewLocationService(server *Server) *LocationService {
	return &LocationService{
		server:    server,
		semCache:  make(map[string]semanticsCacheEntry),
		lastWrite: make(map[string]lastLocationWrite),
	}
}

// InvalidateProductSemantics 让某产品的语义缓存立刻失效（产品物模型保存后调用）。
func (s *LocationService) InvalidateProductSemantics(productCode string) {
	if s == nil {
		return
	}
	s.semMu.Lock()
	defer s.semMu.Unlock()
	delete(s.semCache, productCode)
}

// SemanticsForProduct 解析（并缓存）某产品的定位语义。
func (s *LocationService) SemanticsForProduct(productCode string) LocationSemantics {
	if s == nil || productCode == "" {
		return LocationSemantics{}
	}

	s.semMu.RLock()
	if entry, ok := s.semCache[productCode]; ok && time.Since(entry.loadedAt) < locationSemanticsTTL {
		s.semMu.RUnlock()
		return entry.sem
	}
	s.semMu.RUnlock()

	var sem LocationSemantics
	product, err := store.GetProduct(productCode)
	if err == nil && product != nil {
		sem = ResolveLocationSemantics(product.Config)
	}

	s.semMu.Lock()
	s.semCache[productCode] = semanticsCacheEntry{sem: sem, loadedAt: time.Now()}
	s.semMu.Unlock()
	return sem
}

// SemanticsForDeviceDetailed 在语义之外附带跨层校验告警（设备详情 / 产品详情用）。
//
// 跨层校验：物模型语义属性 ↔ 协议映射点 precision。
// precision 落在 (0,6) 会静默毁掉坐标（precision=2 ≈ 1.1 km），
// 而映射编辑器的占位提示写的是 "e.g. 2" —— 照抄模板必踩，所以必须主动检查。
func (s *LocationService) SemanticsForDeviceDetailed(deviceCode string) (LocationSemantics, error) {
	sem, err := s.SemanticsForDevice(deviceCode)
	if err != nil {
		return sem, err
	}
	if !sem.Resolved() {
		return sem, nil
	}

	var configs []string
	if device, err := store.GetDevice(deviceCode); err == nil && device != nil {
		if device.Config != "" {
			configs = append(configs, device.Config)
		}
		if device.ProtocolProfileCode != "" {
			if profile, err := store.GetProtocolProfile(device.ProtocolProfileCode); err == nil && profile != nil && profile.Config != "" {
				configs = append(configs, profile.Config)
			}
		}
	}
	sem.PrecisionWarnings = ValidateLocationMapping(sem, configs...)
	return sem, nil
}

// deviceByCode 优先走内存注册表，避免高频属性上报路径上反复查库。
func (s *LocationService) deviceByCode(deviceCode string) (*store.Device, error) {
	if s != nil && s.server != nil && s.server.DeviceManager != nil && s.server.DeviceManager.Registry != nil {
		if device, ok := s.server.DeviceManager.Registry.GetDevice(deviceCode); ok && device != nil {
			return device, nil
		}
	}
	device, err := store.GetDevice(deviceCode)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("%w: device %s", ErrDeviceNotFound, deviceCode)
	}
	return device, nil
}

// SemanticsForDevice 解析设备所属产品的定位语义。
func (s *LocationService) SemanticsForDevice(deviceCode string) (LocationSemantics, error) {
	if s == nil || deviceCode == "" {
		return LocationSemantics{}, errors.New("device code is required")
	}
	device, err := s.deviceByCode(deviceCode)
	if err != nil {
		return LocationSemantics{}, err
	}
	return s.SemanticsForProduct(device.ProductCode), nil
}

// ErrDeviceNotFound 表示设备不存在。
var ErrDeviceNotFound = errors.New("device not found")

// RecordTelemetry 处理一次遥测属性上报中的定位信息。
//
// 由 DeviceManager.ReportDeviceProperties 调用，是属性上行链路的一部分，
// 所以任何失败都只能记录、不能向上返回错误 —— 位置落库不能影响主数据链路。
func (s *LocationService) RecordTelemetry(deviceCode string, properties map[string]interface{}) {
	if s == nil || deviceCode == "" || len(properties) == 0 {
		return
	}

	sem, err := s.SemanticsForDevice(deviceCode)
	if err != nil || !sem.Resolved() {
		return // 静态设备（无定位语义）或设备不存在
	}

	sample, ok := ExtractLocationSample(sem, properties)
	if !ok {
		return
	}

	if !s.shouldPersist(deviceCode, sample) {
		return
	}

	loc := &store.DeviceLocation{
		DeviceCode: deviceCode,
		Longitude:  sample.Longitude,
		Latitude:   sample.Latitude,
		Altitude:   sample.Altitude,
		AccuracyM:  sample.AccuracyM,
		SpeedMPS:   sample.SpeedMPS,
		HeadingDeg: sample.HeadingDeg,
		Source:     store.LocationSourceTelemetry,
		ReceivedAt: time.Now().UnixMilli(),
	}
	if sample.FixAt > 0 {
		loc.FixAt = sample.FixAt
	}
	if err := store.UpsertDeviceLocation(loc); err != nil {
		if s.server != nil && s.server.Logger != nil {
			s.server.Logger.Warn("Failed to persist telemetry device location",
				zap.String("device", deviceCode), zap.Error(err))
		}
		return
	}
	s.afterLocationChanged(deviceCode, loc)
}

// shouldPersist 判断本次上报是否值得落库：位置变化，或已超过心跳间隔。
func (s *LocationService) shouldPersist(deviceCode string, sample LocationSample) bool {
	now := time.Now().UnixMilli()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	last, ok := s.lastWrite[deviceCode]
	if !ok {
		return true
	}
	if last.longitude != sample.Longitude || last.latitude != sample.Latitude {
		return true
	}
	if now-last.at >= locationHeartbeatInterval.Milliseconds() {
		return true
	}
	return false
}

func (s *LocationService) markWritten(deviceCode string, loc *store.DeviceLocation) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.lastWrite[deviceCode] = lastLocationWrite{
		longitude: loc.Longitude,
		latitude:  loc.Latitude,
		at:        time.Now().UnixMilli(),
	}
}

func (s *LocationService) afterLocationChanged(deviceCode string, loc *store.DeviceLocation) {
	s.markWritten(deviceCode, loc)
	if s.server != nil && s.server.Geofence != nil {
		s.server.Geofence.EvaluateDevice(deviceCode, loc)
	}
	s.publishLocationChanged(deviceCode, loc)
}

// publishLocationChanged 把位置变更推给订阅方（地图页等）。
//
// 放在 afterLocationChanged 而不是各个写入点：遥测解析、人工标注、清除都从这里
// 经过，新增写入通道时不会漏推送。频率由 shouldPersist 的心跳（30s）兜住，
// 静止设备不会打出事件风暴。
//
// 推送失败不影响位置落库这条主链路 —— 事件总线是尽力而为的旁路。
func (s *LocationService) publishLocationChanged(deviceCode string, loc *store.DeviceLocation) {
	if s == nil || s.server == nil || s.server.DeviceManager == nil || loc == nil {
		return
	}
	payload := map[string]interface{}{
		"device_code": deviceCode,
		"longitude":   loc.Longitude,
		"latitude":    loc.Latitude,
		"source":      loc.Source,
		"fix_at":      loc.FixAt,
		"received_at": loc.ReceivedAt,
	}
	if loc.AccuracyM != nil {
		payload["accuracy_m"] = *loc.AccuracyM
	}
	s.server.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceLocationChanged,
		Topic:     deviceCode,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	})
}

// SetManualLocation 写入人工标注的位置。
//
// 来源互斥：若设备所属物模型带有经纬度语义属性（遥测设备），
// 人工写入会被下一次上报静默覆盖 —— 所以这里直接拒绝，并由 UI 禁用入口。
func (s *LocationService) SetManualLocation(deviceCode string, longitude, latitude float64, updatedBy string) error {
	if s == nil || deviceCode == "" {
		return errors.New("device code is required")
	}
	if err := ValidateLocationCoordinates(longitude, latitude); err != nil {
		return err
	}

	sem, err := s.SemanticsForDevice(deviceCode)
	if err != nil {
		return err
	}
	if sem.Resolved() {
		return ErrLocationManagedByTelemetry
	}

	now := time.Now().UnixMilli()
	loc := &store.DeviceLocation{
		DeviceCode: deviceCode,
		Longitude:  longitude,
		Latitude:   latitude,
		Source:     store.LocationSourceManual,
		FixAt:      now,
		ReceivedAt: now,
		UpdatedBy:  updatedBy,
	}
	// 手动标注没有 GPS 精度可言，但判定引擎不能把它当零误差 —— 给一个保守默认半径。
	defaultAccuracy := store.ManualAccuracyDefaultM
	loc.AccuracyM = &defaultAccuracy

	if err := store.UpsertDeviceLocation(loc); err != nil {
		return err
	}
	s.afterLocationChanged(deviceCode, loc)
	return nil
}

// ErrLocationManagedByTelemetry 表示设备位置由物模型遥测点位管理，禁止人工写入。
var ErrLocationManagedByTelemetry = errors.New("device location is managed by telemetry points; manual editing is disabled")

// ClearLocation 清除设备位置（仅限手动来源）。
func (s *LocationService) ClearLocation(deviceCode string) error {
	if s == nil || deviceCode == "" {
		return errors.New("device code is required")
	}
	loc, err := store.GetDeviceLocation(deviceCode)
	if err != nil {
		return err
	}
	if loc == nil {
		return nil
	}
	if loc.Source == store.LocationSourceTelemetry {
		return ErrLocationManagedByTelemetry
	}
	if err := store.DeleteDeviceLocation(deviceCode); err != nil {
		return err
	}
	s.writeMu.Lock()
	delete(s.lastWrite, deviceCode)
	s.writeMu.Unlock()
	if s.server != nil && s.server.Geofence != nil {
		s.server.Geofence.ClearDevice(deviceCode)
	}
	return nil
}

// DeviceMapItem 是地图页需要的一条设备记录（设备信息 + 可选位置）。
type DeviceMapItem struct {
	DeviceCode  string `json:"device_code"`
	DeviceName  string `json:"device_name"`
	ProductCode string `json:"product_code"`
	Enabled     bool   `json:"enabled"`
	Online      bool   `json:"online"`

	// 协议标识必须跟着设备一起下发：
	// 悬停卡要判断"这台能不能点播视频"，而判据来自协议插件注册的 deviceActions
	// （condition 读 protocol_name / protocol_profile_code）。少这两个字段，
	// 前端只能为每个 hover 再发一次设备详情请求，或者退回按 product_code 硬编码。
	ProtocolName        string `json:"protocol_name,omitempty"`
	ProtocolProfileCode string `json:"protocol_profile_code,omitempty"`

	// Mode 决定设备详情里手动标注入口是否可用：telemetry 时禁用。
	Mode string `json:"mode"`

	HasLocation bool     `json:"has_location"`
	Longitude   *float64 `json:"longitude,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Altitude    *float64 `json:"altitude,omitempty"`
	AccuracyM   *float64 `json:"accuracy_m,omitempty"`

	Source     string `json:"source,omitempty"`
	FixAt      int64  `json:"fix_at,omitempty"`
	ReceivedAt int64  `json:"received_at,omitempty"`
	UpdatedBy  string `json:"updated_by,omitempty"`
}

// ListMapDevices 拉取地图页所需的设备与位置。
//
// 同时返回"有位置"和"无位置"的设备 —— 后者用于右侧面板的「待标注」分组，
// 以及空状态判定（全部设备都没有位置时才显示空态）。
func (s *LocationService) ListMapDevices(filter store.DeviceQueryFilter) ([]DeviceMapItem, error) {
	devices, _, err := store.ListDevicesWithFilter(0, 0, filter)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return []DeviceMapItem{}, nil
	}

	codes := make([]string, 0, len(devices))
	productCodes := make([]string, 0, len(devices))
	seenProduct := make(map[string]bool)
	for _, device := range devices {
		codes = append(codes, device.Code)
		if device.ProductCode != "" && !seenProduct[device.ProductCode] {
			seenProduct[device.ProductCode] = true
			productCodes = append(productCodes, device.ProductCode)
		}
	}

	locations, err := store.ListDeviceLocationsByCodes(codes)
	if err != nil {
		return nil, err
	}

	// 语义按产品解析并缓存，避免逐设备重复解 JSON。
	modes := make(map[string]string, len(productCodes))
	for _, code := range productCodes {
		modes[code] = s.SemanticsForProduct(code).Mode()
	}

	items := make([]DeviceMapItem, 0, len(devices))
	for _, device := range devices {
		mode := modes[device.ProductCode]
		if mode == "" {
			mode = store.LocationSourceManual
		}
		item := DeviceMapItem{
			DeviceCode:          device.Code,
			DeviceName:          device.Name,
			ProductCode:         device.ProductCode,
			ProtocolName:        device.ProtocolName,
			ProtocolProfileCode: device.ProtocolProfileCode,
			Enabled:             device.Enabled,
			Mode:                mode,
		}
		if s.server != nil && s.server.DeviceManager != nil {
			if status, ok := s.server.DeviceManager.GetStatus(device.Code); ok {
				item.Online = status.Online
			}
		}
		if loc, ok := locations[device.Code]; ok && loc != nil {
			lng, lat := loc.Longitude, loc.Latitude
			item.HasLocation = true
			item.Longitude = &lng
			item.Latitude = &lat
			item.Altitude = loc.Altitude
			item.AccuracyM = loc.AccuracyM
			item.Source = loc.Source
			item.FixAt = loc.FixAt
			item.ReceivedAt = loc.ReceivedAt
			item.UpdatedBy = loc.UpdatedBy
		}
		items = append(items, item)
	}
	return items, nil
}

// DeviceLocationView 返回单台设备的位置视图（设备详情用）。
func (s *LocationService) DeviceLocationView(deviceCode string) (*DeviceMapItem, error) {
	device, err := store.GetDevice(deviceCode)
	if err != nil {
		return nil, err
	}
	if device == nil {
		return nil, fmt.Errorf("%w: device %s", ErrDeviceNotFound, deviceCode)
	}
	item := &DeviceMapItem{
		DeviceCode:  device.Code,
		DeviceName:  device.Name,
		ProductCode: device.ProductCode,
		Enabled:     device.Enabled,
		Mode:        s.SemanticsForProduct(device.ProductCode).Mode(),
	}
	if s.server != nil && s.server.DeviceManager != nil {
		if status, ok := s.server.DeviceManager.GetStatus(device.Code); ok {
			item.Online = status.Online
		}
	}
	loc, err := store.GetDeviceLocation(deviceCode)
	if err != nil {
		return nil, err
	}
	if loc != nil {
		lng, lat := loc.Longitude, loc.Latitude
		item.HasLocation = true
		item.Longitude = &lng
		item.Latitude = &lat
		item.Altitude = loc.Altitude
		item.AccuracyM = loc.AccuracyM
		item.Source = loc.Source
		item.FixAt = loc.FixAt
		item.ReceivedAt = loc.ReceivedAt
		item.UpdatedBy = loc.UpdatedBy
	}
	return item, nil
}

// LocationSemanticsView 是物模型定位语义的对外视图（前端用于显示"已自动识别"等提示）。
type LocationSemanticsView struct {
	LocationSemantics
	Mode string `json:"mode"`
}

// ProductLocationSemantics 返回某产品的定位语义视图。
func (s *LocationService) ProductLocationSemantics(productCode string) LocationSemanticsView {
	sem := s.SemanticsForProduct(productCode)
	return LocationSemanticsView{LocationSemantics: sem, Mode: sem.Mode()}
}
