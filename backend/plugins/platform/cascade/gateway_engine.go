package cascade

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"noyo/core"
	"noyo/core/platform"
	"noyo/core/store"
	"noyo/core/system"
	"noyo/core/types"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

type gatewayEngineImpl struct {
	alarmVideoAcks     chan alarmVideoAck
	ctx                platform.Context
	logger             *zap.Logger
	config             *Config
	gatewayCode        string
	client             mqtt.Client
	receivers          map[string]*FileReceiver
	receiversMux       sync.Mutex
	cancel             context.CancelFunc
	isRegistered       atomic.Bool
	platformOnline     atomic.Bool
	configVersion      atomic.Int64
	localEventSubs     map[types.EventType]uint64
	mediaSignalMu      sync.RWMutex
	mediaSignalHandler func(platform.MediaSignal)
	server             *core.Server
}

func (e *gatewayEngineImpl) keepGatewayLocalDevice(device *store.Device) bool {
	return e.server != nil && e.server.Manager.IsGatewayOwnedDevice(device)
}
func (e *gatewayEngineImpl) prepareGatewayDiscoveredDevice(gateway, device *store.Device) *store.Device {
	if e.server == nil {
		return nil
	}
	return e.server.Manager.PrepareGatewayDevice(gateway, device)
}
func (e *gatewayEngineImpl) prepareGatewayTelemetryEvent(gatewayCode string, event types.Event, device *store.Device) types.Event {
	if prepared, err := attachGatewaySnapshot(event); err != nil {
		if e.logger != nil {
			e.logger.Warn("Failed to attach gateway alarm snapshot", zap.String("device", event.Topic), zap.Error(err))
		}
	} else {
		event = prepared
	}
	if e.server == nil {
		return event
	}
	return e.server.Manager.PrepareGatewayTelemetry(gatewayCode, event, device)
}

func NewGatewayEngine(ctx platform.Context, logger *zap.Logger, cfg *Config) GatewayEngine {
	engine := &gatewayEngineImpl{
		ctx:            ctx,
		logger:         logger,
		config:         cfg,
		receivers:      make(map[string]*FileReceiver),
		localEventSubs: make(map[types.EventType]uint64),
	}
	engine.server, _ = ctx.GetCoreServer().(*core.Server)
	if code, err := cfg.GatewayCodeValue(); err == nil {
		engine.gatewayCode = code
	}
	return engine
}

func (e *gatewayEngineImpl) Start() error {
	e.alarmVideoAcks = make(chan alarmVideoAck, 8)
	e.logger.Info("Gateway Engine Started", zap.String("mqtt_url", e.config.MqttUrl))

	if e.config.MqttUrl == "" {
		return fmt.Errorf("gateway mqtt_url is empty")
	}
	if e.gatewayCode == "" {
		gatewayCode, err := e.config.GatewayCodeValue()
		if err != nil {
			return err
		}
		e.gatewayCode = gatewayCode
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(e.config.MqttUrl)
	// Use deterministic ClientID so broker properly handles session takeover and LWT
	opts.SetClientID(fmt.Sprintf("noyo-gw-cascade-%s", e.gatewayCode))
	opts.SetUsername(e.config.Username)
	opts.SetPassword(e.config.Password)
	applyMQTTTLSOptions(opts, e.config)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)   // 增加到 60s，避免系统负载高时 PING 超时触发 LWT
	opts.SetPingTimeout(20 * time.Second) // 增加 PING 超时容忍度

	// Set Last Will and Testament (LWT) for Gateway offline status
	// 使用固定时间戳 0，便于区分 LWT 触发的 offline 和主动发布的 offline
	willPayload := `{"status":"offline","timestamp":0}`
	opts.SetWill(fmt.Sprintf("noyo/cascade/gw/%s/status", e.gatewayCode), willPayload, 1, true)

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		e.logger.Info("Gateway MQTT Connected")
		e.subscribeTopics(c)
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		e.logger.Warn("Gateway MQTT Connection Lost", zap.Error(err))
	})

	e.client = mqtt.NewClient(opts)

	go func() {
		if token := e.client.Connect(); token.Wait() && token.Error() != nil {
			e.logger.Error("Gateway MQTT Connect Failed", zap.Error(token.Error()))
		}
	}()

	// Start telemetry reporting loop
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go e.telemetryLoop(ctx)
	go e.alarmVideoUploadLoop(ctx)

	// Subscribe to local core events for telemetry routing to Platform
	e.localEventSubs[types.EventDeviceStatusChanged] = e.ctx.SubscribeEvent(types.EventDeviceStatusChanged, e.handleLocalEvent)
	e.localEventSubs[types.EventPropertyReported] = e.ctx.SubscribeEvent(types.EventPropertyReported, e.handleLocalEvent)
	e.localEventSubs[types.EventEventReported] = e.ctx.SubscribeEvent(types.EventEventReported, e.handleLocalEvent)

	return nil
}

func (e *gatewayEngineImpl) telemetryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !e.isRegistered.Load() {
				// Retry registration periodically if not registered
				e.sendRegisterRequest()
				continue
			}
			if e.client != nil && e.client.IsConnected() {
				stats, err := system.GetStats()
				if err == nil && stats != nil {
					props := map[string]interface{}{
						"sys_cpu":          math.Round(stats.CPU*100) / 100,
						"sys_mem_percent":  math.Round(stats.MemoryPercent*100) / 100,
						"sys_mem_total":    math.Round((float64(stats.MemoryTotal)/(1024*1024))*100) / 100,
						"sys_mem_used":     math.Round((float64(stats.MemoryUsed)/(1024*1024))*100) / 100,
						"sys_disk_percent": math.Round(stats.DiskPercent*100) / 100,
						"sys_disk_total":   math.Round((float64(stats.DiskTotal)/(1024*1024*1024))*100) / 100,
						"sys_disk_used":    math.Round((float64(stats.DiskUsed)/(1024*1024*1024))*100) / 100,
						"svc_cpu":          math.Round(stats.ServiceCPU*100) / 100,
						"svc_mem":          math.Round((float64(stats.ServiceMemory)/(1024*1024))*100) / 100,
						"sys_uptime":       stats.Uptime,
						"sys_ip":           stats.IP,
						"sys_os":           stats.OS,
						"sys_arch":         stats.Arch,
						"gw_version":       stats.Version,
						"gw_go_version":    stats.GoVersion,
						"gw_goroutine":     stats.NumGoroutine,
					}
					event := types.Event{
						Type:      types.EventPropertyReported,
						Topic:     e.gatewayCode,
						Payload:   props,
						Timestamp: time.Now().UnixMilli(),
					}
					payloadBytes, _ := json.Marshal(event)
					e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.gatewayCode), 1, false, payloadBytes)
				}
			}
		}
	}
}

func (e *gatewayEngineImpl) Stop() error {
	e.logger.Info("Gateway Engine Stopped")

	// Unsubscribe from all local core events to prevent memory leak
	for eventType, id := range e.localEventSubs {
		e.ctx.UnsubscribeEvent(eventType, id)
	}
	e.localEventSubs = make(map[types.EventType]uint64)

	if e.cancel != nil {
		e.cancel()
	}
	if e.client != nil && e.client.IsConnected() {
		// Explicitly publish Offline status before graceful disconnect
		// to ensure the retained "online" message is cleared.
		offlinePayload := fmt.Sprintf(`{"status":"offline","timestamp":%d}`, time.Now().UnixMilli())
		token := e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/status", e.gatewayCode), 1, true, []byte(offlinePayload))
		token.WaitTimeout(2 * time.Second)

		e.client.Disconnect(250)
	}
	return nil
}

func (e *gatewayEngineImpl) handleLocalEvent(event types.Event) {
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	if !e.isRegistered.Load() {
		return
	}
	device, _ := store.GetDevice(event.Topic)
	event = e.prepareGatewayTelemetryEvent(e.gatewayCode, event, device)
	topic := fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.gatewayCode)
	payloadBytes, err := json.Marshal(event)
	if err == nil {
		e.client.Publish(topic, 1, false, payloadBytes)
	}
}

func (e *gatewayEngineImpl) subscribeTopics(c mqtt.Client) {
	c.Subscribe(fmt.Sprintf("noyo/cascade/gw/%s/alarm-video/down", e.gatewayCode), 1, func(_ mqtt.Client, msg mqtt.Message) {
		if msg.Retained() || len(msg.Payload()) > 4096 {
			return
		}
		var ack alarmVideoAck
		if json.Unmarshal(msg.Payload(), &ack) == nil {
			select {
			case e.alarmVideoAcks <- ack:
			default:
			}
		}
	})
	configTopic := fmt.Sprintf("noyo/cascade/gw/%s/config/version", e.gatewayCode)
	c.Subscribe(configTopic, 1, e.handleConfigVersion)

	cmdTopic := fmt.Sprintf("noyo/cascade/gw/%s/command/request", e.gatewayCode)
	c.Subscribe(cmdTopic, 1, e.handleCommand)

	mediaSignalTopic := fmt.Sprintf("noyo/cascade/gw/%s/media/signal/down", e.gatewayCode)
	c.Subscribe(mediaSignalTopic, 1, e.handleMediaSignalDown)

	platformStatusTopic := "noyo/cascade/platform/status"
	c.Subscribe(platformStatusTopic, 1, e.handlePlatformStatus)

	regRespTopic := fmt.Sprintf("noyo/cascade/gw/%s/register/response", e.gatewayCode)
	if token := c.Subscribe(regRespTopic, 1, e.handleRegisterResponse); token.Wait() && token.Error() != nil {
		e.logger.Error("Failed to subscribe to register response topic", zap.Error(token.Error()))
	}

	e.sendRegisterRequest()

	fileMetaTopic := fmt.Sprintf("noyo/cascade/gw/%s/file/meta", e.gatewayCode)
	c.Subscribe(fileMetaTopic, 1, func(client mqtt.Client, msg mqtt.Message) {
		if msg.Retained() {
			return
		}
		var info FileTransferInfo
		if err := json.Unmarshal(msg.Payload(), &info); err == nil {
			e.logger.Info("Received file metadata", zap.String("file_id", info.FileID), zap.String("name", info.FileName))
			e.receiversMux.Lock()
			destDir := filepath.Join(os.TempDir(), "noyo_cascade")
			recv, err := NewFileReceiver(info, destDir, e.logger, func(finalPath string) {
				e.logger.Info("File transfer complete", zap.String("path", finalPath))
				e.receiversMux.Lock()
				delete(e.receivers, info.FileID)
				e.receiversMux.Unlock()
				go e.handleReceivedFile(info.FileName, finalPath)
			})
			if err == nil {
				e.receivers[info.FileID] = recv
			} else {
				e.logger.Error("Failed to init file receiver", zap.Error(err))
			}
			e.receiversMux.Unlock()
		}
	})

	fileChunkTopic := fmt.Sprintf("noyo/cascade/gw/%s/file/chunk", e.gatewayCode)
	c.Subscribe(fileChunkTopic, 1, func(client mqtt.Client, msg mqtt.Message) {
		if msg.Retained() {
			return
		}
		var chunk FileChunk
		if err := json.Unmarshal(msg.Payload(), &chunk); err == nil {
			e.receiversMux.Lock()
			recv, ok := e.receivers[chunk.FileID]
			e.receiversMux.Unlock()

			if ok {
				if err := recv.ReceiveChunk(chunk); err != nil {
					e.logger.Error("Error receiving chunk", zap.Error(err))
					e.receiversMux.Lock()
					delete(e.receivers, chunk.FileID)
					e.receiversMux.Unlock()
				}
			}
		}
	})

}

func (e *gatewayEngineImpl) SetMediaSignalHandler(handler func(platform.MediaSignal)) {
	e.mediaSignalMu.Lock()
	e.mediaSignalHandler = handler
	e.mediaSignalMu.Unlock()
}

// PublishMediaSignal sends a gateway-discovered ICE candidate to the platform.
func (e *gatewayEngineImpl) PublishMediaSignal(signal platform.MediaSignal) error {
	if signal.SessionID == "" {
		return fmt.Errorf("media session id is required")
	}
	if e.client == nil || !e.client.IsConnected() {
		return fmt.Errorf("gateway MQTT client not connected")
	}
	payload, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("marshal media signal: %w", err)
	}
	topic := fmt.Sprintf("noyo/cascade/gw/%s/media/signal/up", e.gatewayCode)
	token := e.client.Publish(topic, 1, false, payload)
	token.Wait()
	return token.Error()
}

func (e *gatewayEngineImpl) handleMediaSignalDown(_ mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	var signal platform.MediaSignal
	if err := json.Unmarshal(msg.Payload(), &signal); err != nil || signal.SessionID == "" {
		if err != nil {
			e.logger.Warn("Invalid platform media signal", zap.Error(err))
		}
		return
	}
	e.mediaSignalMu.RLock()
	handler := e.mediaSignalHandler
	e.mediaSignalMu.RUnlock()
	if handler != nil {
		handler(signal)
	}
}

func (e *gatewayEngineImpl) handleReceivedFile(fileName, filePath string) {
	if fileName == "sync_config.json" {
		e.processSyncConfig(filePath)
	} else {
		e.logger.Info("Unhandled file type received", zap.String("name", fileName))
	}
}

func (e *gatewayEngineImpl) processSyncConfig(filePath string) {
	e.logger.Info("Processing sync config file", zap.String("path", filePath))
	defer os.Remove(filePath)

	data, err := os.ReadFile(filePath)
	if err != nil {
		e.logger.Error("Failed to read sync config file", zap.Error(err))
		return
	}

	var syncData struct {
		Timestamp    int64             `json:"timestamp"`
		Products     []*store.Product  `json:"products"`
		Devices      []*store.Device   `json:"devices"`
		MediaNetwork *SyncMediaNetwork `json:"media_network,omitempty"`
	}

	if err := json.Unmarshal(data, &syncData); err != nil {
		e.logger.Error("Failed to unmarshal sync config", zap.Error(err))
		return
	}

	e.logger.Info("Gateway received sync config from platform", zap.Int("products_count", len(syncData.Products)), zap.Int("devices_count", len(syncData.Devices)))

	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok {
		e.logger.Error("Core server not available")
		return
	}

	if syncData.MediaNetwork != nil {
		e.logger.Info("Gateway received media network config from platform",
			zap.String("stun_urls", syncData.MediaNetwork.StunURLs),
			zap.String("turn_urls", syncData.MediaNetwork.TurnURLs))
		if coreServer != nil && coreServer.Manager != nil {
			if provider := coreServer.Manager.MediaNetworkProvider(); provider != nil {
				provider.SetGatewayMediaNetwork(platform.GatewayMediaNetwork{
					StunURLs: syncData.MediaNetwork.StunURLs, TurnURLs: syncData.MediaNetwork.TurnURLs,
					TurnUsername: syncData.MediaNetwork.TurnUsername, TurnPassword: syncData.MediaNetwork.TurnPassword,
				})
			}
		}
	}

	// 1. Sync Products
	for _, p := range syncData.Products {
		p.ID = 0 // Clear Platform ID to avoid local SQLite primary key conflicts

		// 检查产品信息是否有变化，避免无变化时触发插件重载
		productChanged := true
		if existingP, err := store.GetProduct(p.Code); err == nil && existingP != nil {
			if existingP.Name == p.Name && existingP.Config == p.Config {
				productChanged = false
			}
		}

		if err := store.SaveProduct(p); err != nil {
			e.logger.Error("Failed to sync product", zap.String("code", p.Code), zap.Error(err))
		} else {
			if !productChanged {
				e.logger.Debug("Product unchanged, skip reload", zap.String("code", p.Code))
				continue
			}

			e.logger.Info("Synced product", zap.String("code", p.Code))
			coreServer.DeviceManager.Registry.UpdateProduct(p)
		}
	}

	// 2. Sync Devices
	syncedDeviceCodes := make(map[string]bool)
	parentsToRestart := make(map[string]bool)
	for _, d := range syncData.Devices {
		d.ID = 0 // Clear Platform ID to avoid local SQLite primary key conflicts
		syncedDeviceCodes[d.Code] = true

		deviceChanged := true
		if existingD, err := store.GetDevice(d.Code); err == nil && existingD != nil {
			if existingD.Name == d.Name && existingD.ProductCode == d.ProductCode && existingD.ParentCode == d.ParentCode && existingD.Enabled == d.Enabled && existingD.Config == d.Config {
				deviceChanged = false
			}
		}

		if err := store.SaveDevice(d); err != nil {
			e.logger.Error("Failed to sync device", zap.String("code", d.Code), zap.Error(err))
		} else {
			e.logger.Info("Synced device", zap.String("code", d.Code))
			coreServer.DeviceManager.Registry.UpdateDevice(d)

			if deviceChanged {
				if d.ParentCode != "" {
					parentsToRestart[d.ParentCode] = true
				}
				if d.Enabled {
					e.logger.Info("Device changed, restarting", zap.String("code", d.Code))
					_ = coreServer.DeviceManager.RestartDevice(d.Code)
				} else {
					e.logger.Info("Device changed to disabled, stopping", zap.String("code", d.Code))
					_ = coreServer.DeviceManager.StopDevice(d.Code)
				}
			} else {
				e.logger.Debug("Device unchanged, skip restart", zap.String("code", d.Code))
			}
		}
	}

	// 3. Clean up deleted devices
	localDevices, _, err := store.ListDevices(0, 0, 0, 0)
	if err != nil {
		e.logger.Error("Failed to list local devices for cleanup", zap.Error(err))
	} else {
		for _, ld := range localDevices {
			// Skip gateway device itself
			if ld.Code == e.config.GatewaySn {
				continue
			}
			// GB28181 cameras are discovered and owned by the gateway. They are
			// reported upward after registration, not provisioned by a platform
			// configuration snapshot.
			if e.keepGatewayLocalDevice(&ld) {
				continue
			}
			// If local device is not in sync data, it was deleted on platform
			if !syncedDeviceCodes[ld.Code] {
				e.logger.Info("Deleting local device not present in sync data", zap.String("code", ld.Code))
				if ld.ParentCode != "" {
					parentsToRestart[ld.ParentCode] = true
				}
				_ = coreServer.DeviceManager.StopDevice(ld.Code)
				coreServer.DeviceManager.Registry.RemoveDevice(ld.Code)
				if err := store.DeleteDevice(ld.Code); err != nil {
					e.logger.Error("Failed to delete local device", zap.String("code", ld.Code), zap.Error(err))
				}
			}
		}
	}

	// 4. Restart affected parents
	for pCode := range parentsToRestart {
		if pCode != "" && coreServer.DeviceManager.IsRunning(pCode) {
			e.logger.Info("Restarting parent device due to sub-device changes", zap.String("parent", pCode))
			if err := coreServer.DeviceManager.RestartDevice(pCode); err != nil {
				e.logger.Error("Failed to restart parent device", zap.String("parent", pCode), zap.Error(err))
			}
		}
	}

	// Update local config version
	e.configVersion.Store(syncData.Timestamp)
	e.logger.Info("Config sync complete", zap.Int("products", len(syncData.Products)), zap.Int("devices", len(syncData.Devices)), zap.Int64("version", syncData.Timestamp))
}

func (e *gatewayEngineImpl) handleConfigVersion(client mqtt.Client, msg mqtt.Message) {
	var payload struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		return
	}

	localVer := e.configVersion.Load()
	if payload.Version > localVer {
		e.logger.Info("Config version updated, requesting sync", zap.Int64("local", localVer), zap.Int64("remote", payload.Version))
		// Always verify registration to handle case where gateway was disabled
		e.sendRegisterRequest()
		e.sendSyncRequest()
	} else {
		e.logger.Debug("Config version matches or older, skipping sync", zap.Int64("local", localVer), zap.Int64("remote", payload.Version))
	}
}

func (e *gatewayEngineImpl) handlePlatformStatus(client mqtt.Client, msg mqtt.Message) {
	var payload struct {
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		e.logger.Error("Failed to parse platform status", zap.Error(err))
		return
	}

	e.logger.Info("Received platform status broadcast", zap.String("status", payload.Status))

	if payload.Status == "online" {
		// Platform is back online, we need to ensure our state is synchronized
		if !e.isRegistered.Load() {
			e.logger.Info("Platform online: triggering register request")
			e.sendRegisterRequest()
		} else {
			wasOnline := e.platformOnline.Swap(true)
			if !wasOnline {
				e.logger.Info("Platform online: triggering sync request and replaying gateway state")
				e.publishGatewayStateReplay()

				// 仅当没有有效的本地版本时才全量同步
				if e.configVersion.Load() == 0 {
					e.sendSyncRequest()
				}
			} else {
				e.logger.Debug("Received duplicate platform online status, ignoring")
			}
		}
	} else if payload.Status == "offline" {
		e.platformOnline.Store(false)
		e.logger.Info("Platform is offline")
	}
}

func (e *gatewayEngineImpl) sendRegisterRequest() {
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	topic := fmt.Sprintf("noyo/cascade/gw/%s/register/request", e.gatewayCode)

	req := map[string]interface{}{
		"gateway_name": e.config.GatewayName,
		"gateway_sn":   e.config.GatewaySn,
		"tenant_id":    e.config.TenantID,
		"project_id":   e.config.ProjectID,
		"tenant_name":  e.config.TenantName,
		"project_name": e.config.ProjectName,
	}
	reqBytes, _ := json.Marshal(req)

	e.client.Publish(topic, 1, false, reqBytes)
	e.logger.Info("Sent register request", zap.String("name", e.config.GatewayName), zap.Uint("tenant_id", e.config.TenantID), zap.Uint("project_id", e.config.ProjectID), zap.String("gateway_code", e.gatewayCode))
}

func (e *gatewayEngineImpl) handleRegisterResponse(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	var resp struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(msg.Payload(), &resp); err != nil {
		e.logger.Error("Failed to parse register response", zap.Error(err))
		return
	}

	if resp.Status == "success" {
		e.logger.Info("Gateway registered successfully")
		e.isRegistered.Store(true)

		// Wait for old session's LWT to be published before we publish our online status
		time.Sleep(500 * time.Millisecond)

		// Publish Online status
		onlinePayload := fmt.Sprintf(`{"status":"online","timestamp":%d}`, time.Now().UnixMilli())
		e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/status", e.gatewayCode), 1, true, []byte(onlinePayload))

		e.publishGatewayStateReplay()

		// 仅当没有有效的本地版本时才全量同步，离线变更将通过 retained 的 config/version 触发
		if e.configVersion.Load() == 0 {
			e.sendSyncRequest()
		}
	} else {
		e.logger.Info("Gateway registration pending or failed", zap.String("status", resp.Status), zap.String("message", resp.Message))
		e.isRegistered.Store(false)

		// Explicitly publish Offline status if registration fails
		// This clears the retained "Online" state if the device was previously registered
		offlinePayload := fmt.Sprintf(`{"status":"offline","timestamp":%d}`, time.Now().UnixMilli())
		e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/status", e.gatewayCode), 1, true, []byte(offlinePayload))
	}
}

// buildGatewayStateReplayEvents rebuilds the gateway's telemetry snapshot after
// the platform becomes reachable. Gateway-local GB28181 devices use a local
// code, so every replayed event must go through prepareGatewayTelemetryEvent to
// keep the platform-side device identity stable.
func buildGatewayStateReplayEvents(gatewayCode, gatewaySN string, timestamp int64, gatewayProperties map[string]interface{}, devices []*store.Device, getStatus func(string) (core.DeviceStatus, bool), getLatestData func(string) map[string]interface{}, prepareTelemetry func(string, types.Event, *store.Device) types.Event) []types.Event {
	events := []types.Event{{
		Type:      types.EventDeviceStatusChanged,
		Topic:     gatewayCode,
		Payload:   types.DeviceStatusOnline,
		Timestamp: timestamp,
	}}
	if len(gatewayProperties) > 0 {
		events = append(events, types.Event{
			Type:      types.EventPropertyReported,
			Topic:     gatewayCode,
			Payload:   gatewayProperties,
			Timestamp: timestamp,
		})
	}

	for _, device := range devices {
		if device == nil || device.Code == gatewaySN {
			continue
		}
		if status, ok := getStatus(device.Code); ok {
			statusPayload := types.DeviceStatusOffline
			if status.Online {
				statusPayload = types.DeviceStatusOnline
			}
			event := types.Event{
				Type:      types.EventDeviceStatusChanged,
				Topic:     device.Code,
				Payload:   statusPayload,
				Timestamp: timestamp,
			}
			events = append(events, prepareTelemetry(gatewayCode, event, device))
		}
		if properties := getLatestData(device.Code); len(properties) > 0 {
			event := types.Event{
				Type:      types.EventPropertyReported,
				Topic:     device.Code,
				Payload:   properties,
				Timestamp: timestamp,
			}
			events = append(events, prepareTelemetry(gatewayCode, event, device))
		}
	}
	return events
}

func (e *gatewayEngineImpl) publishGatewayStateReplay() {
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok {
		e.logger.Warn("Cannot replay gateway state because the core server is unavailable")
		return
	}

	events := buildGatewayStateReplayEvents(
		e.gatewayCode,
		e.config.GatewaySn,
		time.Now().UnixMilli(),
		coreServer.DeviceManager.GetLatestData(e.gatewayCode),
		coreServer.DeviceManager.Registry.GetAllDevices(),
		coreServer.DeviceManager.GetStatus,
		coreServer.DeviceManager.GetLatestData,
		e.prepareGatewayTelemetryEvent,
	)
	topic := fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.gatewayCode)
	for _, event := range events {
		payload, err := json.Marshal(event)
		if err != nil {
			e.logger.Warn("Failed to marshal gateway state replay event", zap.String("device", event.Topic), zap.Error(err))
			continue
		}
		// State replays are deliberately not retained: stale telemetry must never
		// overwrite a later MQTT last-will or a newer property report.
		e.client.Publish(topic, 1, false, payload)
	}
}

func (e *gatewayEngineImpl) sendSyncRequest() {
	if e.client == nil || !e.client.IsConnected() {
		return
	}
	if !e.isRegistered.Load() {
		return
	}
	req := struct {
		LastSyncTime int64 `json:"last_sync_time"`
	}{
		LastSyncTime: 0,
	}
	reqBytes, _ := json.Marshal(req)
	topic := fmt.Sprintf("noyo/cascade/gw/%s/sync/request", e.gatewayCode)
	e.client.Publish(topic, 1, false, reqBytes)
	e.logger.Info("Sent sync request")
}

func (e *gatewayEngineImpl) handleCommand(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	e.logger.Info("Received command", zap.Int("len", len(msg.Payload())))

	var cmd struct {
		ID          string      `json:"id"`
		Version     string      `json:"version"`
		DeviceCode  string      `json:"deviceCode"`
		ProductCode string      `json:"productCode"`
		Method      string      `json:"method"`
		Params      interface{} `json:"params"`
	}

	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		e.logger.Error("Failed to parse command", zap.Error(err))
		return
	}

	reply := map[string]interface{}{
		"id":      cmd.ID,
		"code":    200,
		"message": "success",
		"data":    nil,
	}

	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok {
		reply["code"] = 500
		reply["message"] = "core server not found"
		goto SEND_REPLY
	}

	if cmd.Method == remotePluginMethodList {
		plugins, err := listGatewayPlugins(coreServer)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = plugins
		}
	} else if cmd.Method == remotePluginMethodConfigGet {
		req, err := parseRemotePluginConfigGetParams(cmd.Params)
		if err != nil {
			reply["code"] = 400
			reply["message"] = err.Error()
			goto SEND_REPLY
		}
		data, err := getGatewayPluginConfig(coreServer, req.Plugin)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remotePluginMethodConfigSet {
		req, err := parseRemotePluginConfigSetParams(cmd.Params)
		if err != nil {
			reply["code"] = 400
			reply["message"] = err.Error()
			goto SEND_REPLY
		}
		data, err := setGatewayPluginConfig(coreServer, req)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remotePluginMethodStatusSet {
		req, err := parseRemotePluginStatusSetParams(cmd.Params)
		if err != nil {
			reply["code"] = 400
			reply["message"] = err.Error()
			goto SEND_REPLY
		}
		data, err := setGatewayPluginStatus(coreServer, req)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodConfigGet {
		data, err := getGatewaySystemConfig(coreServer)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodConfigSet {
		data, err := setGatewaySystemConfig(coreServer, cmd.Params)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodLicenseStatus {
		data, err := getGatewayLicenseStatus(coreServer)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodLicenseUpload {
		data, err := uploadGatewayLicense(coreServer, cmd.Params)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodLogFiles {
		data, err := listGatewayLogFiles(coreServer)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodLogFile {
		data, err := readGatewayLogFile(coreServer, cmd.Params)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == remoteSystemMethodLogTail {
		data, err := tailGatewayLog(coreServer, cmd.Params)
		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = data
		}
	} else if cmd.Method == "service_invoke" || cmd.Method == "property_set" {
		paramsMap, ok := cmd.Params.(map[string]interface{})
		if !ok {
			reply["code"] = 400
			reply["message"] = "invalid params"
			goto SEND_REPLY
		}

		serviceId, _ := paramsMap["service_id"].(string)
		invokeParams, _ := paramsMap["params"].(map[string]interface{})

		var res interface{}
		var err error

		if cmd.Method == "property_set" {
			serviceId = "set_properties"
			invokeParams = paramsMap
			err = coreServer.DeviceManager.SetDeviceProperties(cmd.DeviceCode, invokeParams)
		} else {
			res, err = coreServer.DeviceManager.CallDeviceService(cmd.DeviceCode, serviceId, invokeParams)
		}

		if err != nil {
			reply["code"] = 500
			reply["message"] = err.Error()
		} else {
			reply["data"] = res
		}
	} else {
		reply["code"] = 404
		reply["message"] = "method not supported"
	}

SEND_REPLY:
	replyBytes, _ := json.Marshal(reply)
	replyTopic := msg.Topic() + "_reply"
	client.Publish(replyTopic, 1, false, replyBytes)
}
