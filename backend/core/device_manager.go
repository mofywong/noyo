package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"noyo/core/protocol"
	"noyo/core/store"
	"noyo/core/tsdb"
	"noyo/core/types"
	"os"
	"reflect"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

type DeviceStatus struct {
	Online     bool
	LastActive time.Time
	LastReport time.Time
	LastStatus string
}

// DeviceRuntime holds the runtime state for a single device with its own lock
type DeviceRuntime struct {
	mu                    sync.RWMutex
	Status                DeviceStatus
	Properties            map[string]interface{}
	PropertiesInitialized bool
}

// DeviceManager manages the lifecycle of devices and their tasks
type DeviceManager struct {
	Server *Server
	// mu     sync.RWMutex // Removed global lock

	// Components
	Registry  *DeviceRegistry
	Scheduler *TaskScheduler
	EventBus  *EventBus

	// Runtime State (Thread-Safe Map)
	// Key: DeviceCode, Value: *DeviceRuntime
	runtimes sync.Map

	TSDB *tsdb.TSDBManager
}

type LegacyTaskPlugin interface {
	CreateTasks(device DeviceMeta, product ProductMeta) ([]TaskDefinition, error)
}

func NewDeviceManager(server *Server) *DeviceManager {
	dm := &DeviceManager{
		Server:   server,
		Registry: NewDeviceRegistry(server.Logger),
		EventBus: NewEventBus(server.Logger),
	}
	// Initialize Scheduler with callback
	dm.Scheduler = NewTaskScheduler(server.Logger, dm.UpdateStatus)
	return dm
}

// Init initializes sub-components
func (dm *DeviceManager) Init() error {
	// 1. Init Registry (Load from DB)
	if err := dm.Registry.Init(); err != nil {
		return err
	}

	// 2. Init Scheduler
	dm.Scheduler.Init()

	// 3. Init EventBus subscriptions (if any)
	// Example: dm.EventBus.Subscribe(types.EventDeviceStatusChanged, dm.handleStatusChange)

	return nil
}

// StartAll starts all enabled devices
func (dm *DeviceManager) StartAll() error {
	devices := dm.Registry.GetAllDevices()
	dm.Server.Logger.Info("StartAll: Found devices", zap.Int("count", len(devices)))

	for _, d := range devices {
		if d.Enabled {
			dm.Server.Logger.Info("StartAll: Starting device", zap.String("code", d.Code))
			if err := dm.StartDevice(d.Code); err != nil {
				dm.Server.Logger.Error("Failed to start device", zap.String("code", d.Code), zap.Error(err))
			}
		} else {
			dm.Server.Logger.Info("StartAll: Device disabled", zap.String("code", d.Code))
		}
	}
	return nil
}

// StopAll stops all devices
func (dm *DeviceManager) StopAll() {
	dm.Scheduler.Shutdown()
}

// GetDeviceMeta delegates to Registry
func (dm *DeviceManager) GetDeviceMeta(deviceCode string) (*DeviceMeta, *store.Device, error) {
	device, ok := dm.Registry.GetDevice(deviceCode)
	if !ok {
		return nil, nil, fmt.Errorf("%w: device %s", types.ErrNotFound, deviceCode)
	}
	meta, err := dm.Registry.GetDeviceMeta(deviceCode)
	if err != nil {
		return nil, nil, err
	}
	return meta, device, nil
}

// GetProducts delegates to Registry
func (dm *DeviceManager) GetProducts(codes []string) ([]ProductMeta, error) {
	var metas []ProductMeta // alias of types.ProductMeta
	for _, code := range codes {
		if m, err := dm.Registry.GetProductMeta(code); err == nil {
			metas = append(metas, m)
		} else {
			return nil, err
		}
	}
	return metas, nil
}

// GetProduct delegates to Registry
func (dm *DeviceManager) GetProduct(code string) (types.ProductMeta, bool) {
	meta, err := dm.Registry.GetProductMeta(code)
	return meta, err == nil
}

// StartDevice starts a single device
func (dm *DeviceManager) StartDevice(deviceCode string) error {
	// 1. Check if running
	if dm.Scheduler.IsRunning(deviceCode) {
		return fmt.Errorf("%w: device %s is already running", types.ErrBusy, deviceCode)
	}

	// 2. Load Meta
	deviceMetaPtr, deviceModel, err := dm.GetDeviceMeta(deviceCode)
	if err != nil {
		return fmt.Errorf("%w: %v", types.ErrNotFound, err)
	}
	if !deviceModel.Enabled {
		return fmt.Errorf("device %s is disabled", deviceCode)
	}
	deviceMeta := *deviceMetaPtr

	productMeta, err := dm.Registry.GetProductMeta(deviceModel.ProductCode)
	if err != nil {
		return fmt.Errorf("%w: %v", types.ErrNotFound, err)
	}

	// 3. Get Protocol Plugin (子设备从父设备获取协议)
	effectiveProtocol, err := dm.Registry.GetEffectiveProtocol(deviceCode)
	if err != nil {
		return fmt.Errorf("failed to get effective protocol: %w", err)
	}
	plugin := dm.Server.Manager.GetPlugin(effectiveProtocol)
	if plugin == nil {
		return fmt.Errorf("%w: protocol plugin %s", types.ErrNotFound, effectiveProtocol)
	}

	// 4. Create Task Definitions
	var taskDefs []TaskDefinition

	if pp, ok := plugin.(protocol.IProtocolPlugin); ok {
		// New Protocol Plugin: Internal Task Management
		if err := pp.BatchAddDevice([]types.DeviceMeta{deviceMeta}); err != nil {
			return fmt.Errorf("plugin failed to add device: %w", err)
		}
		// Create a dummy task to keep "IsRunning" true
		// 不触发 StatusUpdater：协议插件通过 ReportOnline 自行管理在线状态
		taskDefs = []TaskDefinition{
			{
				Name:             "KeepAlive",
				Interval:         "@every 60s",
				Handler:          func() error { return nil },
				SkipStatusUpdate: true,
			},
		}
	} else if legacy, ok := plugin.(LegacyTaskPlugin); ok {
		// Legacy Plugin
		defs, err := legacy.CreateTasks(deviceMeta, ProductMeta(productMeta))
		if err != nil {
			return fmt.Errorf("failed to create tasks: %w", err)
		}
		taskDefs = defs
	} else {
		// No task support
		taskDefs = []TaskDefinition{
			{
				Name:             "KeepAlive",
				Interval:         "@every 60s",
				Handler:          func() error { return nil },
				SkipStatusUpdate: true,
			},
		}
	}

	dm.Server.Logger.Info("Starting tasks", zap.String("device", deviceCode), zap.Int("task_count", len(taskDefs)))

	// 5. Schedule Tasks
	return dm.Scheduler.StartDeviceTasks(deviceCode, taskDefs)
}

// RestartDevice restarts a device
func (dm *DeviceManager) RestartDevice(deviceCode string) error {
	dm.StopDevice(deviceCode)
	return dm.StartDevice(deviceCode)
}

// StopDevice stops a single device
func (dm *DeviceManager) StopDevice(deviceCode string) error {
	// 1. Stop Tasks
	if err := dm.Scheduler.StopDeviceTasks(deviceCode); err != nil {
		return err
	}

	// 2. Notify Plugin to remove device
	// 使用 GetEffectiveProtocol 获取协议（子设备从父设备获取）
	if effectiveProtocol, err := dm.Registry.GetEffectiveProtocol(deviceCode); err == nil {
		if p := dm.Server.Manager.GetPlugin(effectiveProtocol); p != nil {
			if pp, ok := p.(protocol.IProtocolPlugin); ok {
				pp.RemoveDevice(deviceCode)
			}
		}
	}

	// 3. Mark Offline
	dm.ReportDeviceStatus(deviceCode, DeviceStatus{Online: false})

	dm.Server.Logger.Info("Device stopped", zap.String("code", deviceCode))
	return nil
}

// IsRunning checks if a device is running
func (dm *DeviceManager) IsRunning(deviceCode string) bool {
	return dm.Scheduler.IsRunning(deviceCode)
}

// getRuntime retrieves or creates a runtime for a device
func (dm *DeviceManager) getRuntime(deviceCode string) *DeviceRuntime {
	newRt := &DeviceRuntime{
		Properties: make(map[string]interface{}),
	}
	v, _ := dm.runtimes.LoadOrStore(deviceCode, newRt)
	return v.(*DeviceRuntime)
}

// ReportDeviceStatus report status
func (dm *DeviceManager) ReportDeviceStatus(deviceCode string, status DeviceStatus) {
	rt := dm.getRuntime(deviceCode)

	rt.mu.Lock()
	rt.Status = status
	rt.mu.Unlock()

	// Notify EventBus every time, no filtering
	statusStr := types.DeviceStatusOffline
	if status.Online {
		statusStr = types.DeviceStatusOnline
	}

	dm.EventBus.Publish(types.Event{
		Type:      types.EventDeviceStatusChanged,
		Topic:     deviceCode,
		Payload:   statusStr,
		Timestamp: time.Now().UnixMilli(),
	})
}

// UpdateStatus updates the online status of a device (called by Scheduler)
func (dm *DeviceManager) UpdateStatus(deviceCode string, err error) {
	rt := dm.getRuntime(deviceCode)

	rt.mu.Lock() // Lock only this device

	newOnline := err == nil
	now := time.Now()

	if newOnline {
		rt.Status.Online = true
		rt.Status.LastActive = now
	} else {
		rt.Status.Online = false
	}

	rt.Status.LastReport = now
	if newOnline {
		rt.Status.LastStatus = types.DeviceStatusOnline
	} else {
		rt.Status.LastStatus = types.DeviceStatusOffline
	}

	// Create copy for publishing to avoid holding lock during Publish
	statusCopy := rt.Status
	rt.mu.Unlock()

	statusStr := types.DeviceStatusOffline
	if statusCopy.Online {
		statusStr = types.DeviceStatusOnline
	}

	dm.EventBus.Publish(types.Event{
		Type:      types.EventDeviceStatusChanged,
		Topic:     deviceCode,
		Payload:   statusStr,
		Timestamp: now.UnixMilli(),
	})
}

// GetStatus returns the online status of a device
func (dm *DeviceManager) GetStatus(deviceCode string) (DeviceStatus, bool) {
	if v, ok := dm.runtimes.Load(deviceCode); ok {
		rt := v.(*DeviceRuntime)
		rt.mu.RLock()
		defer rt.mu.RUnlock()
		return rt.Status, true
	}
	return DeviceStatus{}, false
}

// GetOnlineDeviceCodes returns a list of all currently online device codes
func (dm *DeviceManager) GetOnlineDeviceCodes() []string {
	var codes []string
	dm.runtimes.Range(func(key, value interface{}) bool {
		rt := value.(*DeviceRuntime)
		rt.mu.RLock()
		online := rt.Status.Online
		rt.mu.RUnlock()
		if online {
			codes = append(codes, key.(string))
		}
		return true
	})
	return codes
}

// GetLatestData returns the latest data for a device
func (dm *DeviceManager) GetLatestData(deviceCode string) map[string]interface{} {
	if v, ok := dm.runtimes.Load(deviceCode); ok {
		rt := v.(*DeviceRuntime)
		rt.mu.RLock()
		defer rt.mu.RUnlock()

		// Return copy to be safe
		result := make(map[string]interface{}, len(rt.Properties))
		for k, v := range rt.Properties {
			result[k] = v
		}
		return result
	}
	return make(map[string]interface{})
}

// UpdateLatestData updates the latest data for a device
func (dm *DeviceManager) UpdateLatestData(deviceCode string, data map[string]interface{}) {
	dm.mergeLatestData(deviceCode, data)

	// Also mark as online (async)
	go dm.UpdateStatus(deviceCode, nil)
}

func (dm *DeviceManager) mergeLatestData(deviceCode string, data map[string]interface{}) (map[string]interface{}, bool) {
	rt := dm.getRuntime(deviceCode)

	rt.mu.Lock()
	defer rt.mu.Unlock()

	if rt.Properties == nil {
		rt.Properties = make(map[string]interface{})
	}
	isInitial := !rt.PropertiesInitialized
	changed := make(map[string]interface{})
	for k, v := range data {
		if old, exists := rt.Properties[k]; !exists || !propertyValuesEqual(old, v) {
			changed[k] = v
		}
		rt.Properties[k] = v
	}
	rt.PropertiesInitialized = true

	if isInitial {
		// Do not return changed properties on the very first state initialization
		return make(map[string]interface{}), true
	}
	return changed, false
}

func propertyValuesEqual(left, right interface{}) bool {
	if reflect.DeepEqual(left, right) {
		return true
	}
	leftNum, leftOK := numericValue(left)
	rightNum, rightOK := numericValue(right)
	return leftOK && rightOK && leftNum == rightNum
}

// SetDeviceProperties sets properties for a device
func (dm *DeviceManager) SetDeviceProperties(deviceCode string, properties map[string]interface{}) error {
	// 1. Get Device Info
	_, ok := dm.Registry.GetDevice(deviceCode)
	if !ok {
		return fmt.Errorf("%w: device", types.ErrNotFound)
	}

	// 2. Prepare Meta
	meta, _, err := dm.GetDeviceMeta(deviceCode)
	if err != nil {
		return err
	}

	// 3. Get Protocol Plugin (子设备从父设备获取协议)
	effectiveProtocol, err := dm.Registry.GetEffectiveProtocol(deviceCode)
	if err != nil {
		return fmt.Errorf("failed to get effective protocol: %w", err)
	}

	plugin := dm.Server.Manager.GetPlugin(effectiveProtocol)
	if plugin == nil {
		return fmt.Errorf("%w: protocol plugin %s", types.ErrNotFound, effectiveProtocol)
	}

	// 4. Call SetProperty
	// Define lightweight interfaces to support both WriteProperty and WritePoint
	type propertyWriter interface {
		WriteProperty(device types.DeviceMeta, propName string, value interface{}) error
	}
	type pointWriter interface {
		WritePoint(device types.DeviceMeta, pointCode string, value interface{}) error
	}

	if writer, ok := plugin.(propertyWriter); ok {
		for k, v := range properties {
			if err := writer.WriteProperty(*meta, k, v); err != nil {
				return err
			}
		}
		return nil
	} else if writer, ok := plugin.(pointWriter); ok {
		for k, v := range properties {
			if err := writer.WritePoint(*meta, k, v); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("%w: plugin %s does not support SetProperty", types.ErrNotImplemented, effectiveProtocol)
}

// CallDeviceService calls a service on a device
func (dm *DeviceManager) CallDeviceService(deviceCode string, serviceId string, params map[string]interface{}) (result interface{}, err error) {
	// 1. Get Device Info
	_, ok := dm.Registry.GetDevice(deviceCode)
	if !ok {
		return nil, fmt.Errorf("%w: device", types.ErrNotFound)
	}

	// 2. Prepare Meta
	meta, _, err := dm.GetDeviceMeta(deviceCode)
	if err != nil {
		return nil, err
	}
	startedAt := time.Now()
	defer func() {
		if dm.EventBus != nil {
			dm.EventBus.Publish(buildDeviceServiceResultEvent(deviceCode, serviceId, params, result, err, time.Since(startedAt)))
		}
	}()

	// 3. Get Protocol Plugin
	effectiveProtocol, err := dm.Registry.GetEffectiveProtocol(deviceCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get effective protocol: %w", err)
	}

	plugin := dm.Server.Manager.GetPlugin(effectiveProtocol)
	if plugin == nil {
		return nil, fmt.Errorf("%w: protocol plugin %s", types.ErrNotFound, effectiveProtocol)
	}

	// 4. Call CallService
	// Define a lightweight interface to check if the plugin supports CallService.
	// This allows both Protocol Plugins and specific Platform Plugins (like Cascade) to intercept commands.
	type serviceCaller interface {
		CallService(device types.DeviceMeta, serviceCode string, params map[string]interface{}) (interface{}, error)
	}

	if caller, ok := plugin.(serviceCaller); ok {
		result, err = caller.CallService(*meta, serviceId, params)
		return result, err
	}
	err = fmt.Errorf("%w: plugin %s does not support CallService", types.ErrNotImplemented, effectiveProtocol)
	return nil, err
}

func buildDeviceServiceResultEvent(deviceCode, serviceID string, params map[string]interface{}, result interface{}, callErr error, duration time.Duration) types.Event {
	payload := map[string]interface{}{
		"service_id":  serviceID,
		"params":      cloneEventParams(params),
		"success":     callErr == nil,
		"duration_ms": duration.Milliseconds(),
	}
	if callErr != nil {
		payload["error"] = callErr.Error()
	} else {
		payload["result"] = result
	}
	return types.Event{
		Type:      types.EventDeviceServiceResult,
		Topic:     deviceCode,
		Payload:   payload,
		Timestamp: time.Now().UnixMilli(),
	}
}

// ReportDeviceEvent handles event reporting
func (dm *DeviceManager) ReportDeviceEvent(meta DeviceMeta, eventId string, params map[string]interface{}) error {
	if base64Str, ok := params["snapshot_base64"].(string); ok && len(base64Str) > 0 {
		b64Data, ext := normalizeSnapshotBase64(base64Str)
		imgData, err := base64.StdEncoding.DecodeString(b64Data)
		if err == nil {
			filename := fmt.Sprintf("%s_%s_%d%s", meta.DeviceCode, eventId, time.Now().UnixNano(), ext)
			if err := os.MkdirAll("./data/images", 0755); err == nil && os.WriteFile("./data/images/"+filename, imgData, 0644) == nil {
				params["snapshot_url"] = "/data/images/" + filename
				delete(params, "snapshot_base64")
			}
		}
	}

	eventTs := time.Now().UnixMilli()

	// Publish first so UI/SSE subscribers and rule engine workers do not wait for TSDB serialization.
	dm.EventBus.Publish(types.Event{
		Type:      types.EventEventReported,
		Topic:     meta.DeviceCode,
		Payload:   map[string]interface{}{"eventId": eventId, "params": params},
		Timestamp: eventTs,
	})
	dm.pushDeviceEventToTSDB(meta, eventId, params, eventTs)

	return nil
}

func (dm *DeviceManager) pushDeviceEventToTSDB(meta DeviceMeta, eventId string, params map[string]interface{}, eventTs int64) {
	if dm.TSDB == nil {
		return
	}
	paramsForTSDB := cloneEventParams(params)
	go func() {
		data := map[string]interface{}{
			"event_id": eventId,
			"params":   paramsForTSDB,
		}
		payload, err := json.Marshal(data)
		if err != nil {
			dm.Server.Logger.Error("Failed to marshal event for TSDB", zap.Error(err))
			return
		}
		dm.TSDB.Push(&tsdb.Record{
			Ts:         eventTs,
			DeviceCode: meta.DeviceCode,
			Type:       tsdb.TypeEvent,
			Payload:    payload,
		})
	}()
}

func cloneEventParams(params map[string]interface{}) map[string]interface{} {
	if params == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(params))
	for key, value := range params {
		cloned[key] = value
	}
	return cloned
}

func normalizeSnapshotBase64(raw string) (string, string) {
	data := strings.TrimSpace(raw)
	ext := ".jpg"
	if strings.HasPrefix(data, "data:image/") {
		if comma := strings.Index(data, ","); comma >= 0 {
			header := strings.ToLower(data[:comma])
			switch {
			case strings.Contains(header, "image/png"):
				ext = ".png"
			case strings.Contains(header, "image/webp"):
				ext = ".webp"
			}
			data = data[comma+1:]
		}
	}
	return strings.TrimSpace(data), ext
}

// ReportDeviceProperties handles data reporting
func (dm *DeviceManager) ReportDeviceProperties(meta DeviceMeta, properties map[string]interface{}) error {
	// 1. Update Local Cache
	changedProperties, isInitial := dm.mergeLatestData(meta.DeviceCode, properties)
	go dm.UpdateStatus(meta.DeviceCode, nil)

	// 2. Persist to TSDB
	if dm.TSDB != nil {
		payload, err := json.Marshal(properties)
		if err == nil {
			record := &tsdb.Record{
				Ts:         time.Now().UnixMilli(),
				DeviceCode: meta.DeviceCode,
				Type:       tsdb.TypeTelemetry,
				Payload:    payload,
			}
			dm.TSDB.Push(record)
		} else {
			dm.Server.Logger.Error("Failed to marshal properties for TSDB", zap.Error(err))
		}
	}

	// 3. Publish Event
	dm.EventBus.Publish(types.Event{
		Type:      types.EventPropertyReported,
		Topic:     meta.DeviceCode,
		Payload:   properties,
		Metadata:  map[string]interface{}{"changedProperties": changedProperties, "isInitial": isInitial},
		Timestamp: time.Now().UnixMilli(),
	})

	// 4. Broadcast to Platform Plugins -> Removed, handled by DispatchService via EventBus
	return nil
}
