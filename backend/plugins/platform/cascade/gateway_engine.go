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
	"noyo/core/inspection"
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
	// syncHub owns the synchronization session with the platform.
	syncHub *syncHub
	// recorder turns local writes into the propagation log.
	recorder *changeRecorder
	// Serializes runtime loading with retry so an older revision cannot finish
	// after a newer one and leave the process running stale configuration.
	runtimeApplyMu sync.Mutex
	// Serializes configuration replay and durable remote commands with mode transitions.
	configMutationMu    sync.Mutex
	maintenanceWakeOnce sync.Once
	maintenanceWake     chan struct{}
	semantic            *semanticGateway
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

	opts := newGatewayMQTTOptions()
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
		// Re-handshake after every reconnect so a gap that opened while the link
		// was down is discovered by comparing cursors rather than by guessing.
		if e.syncHub != nil {
			e.syncHub.onConnected()
		}
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		e.logger.Warn("Gateway MQTT Connection Lost", zap.Error(err))
	})

	e.client = mqtt.NewClient(opts)
	// Install the session before MQTT can invoke OnConnect. A fast local broker
	// can complete the connection before the former late initSync call, leaving
	// the first connection without a handshake or catch-up.
	if err := e.initSync(); err != nil {
		return fmt.Errorf("gateway engine failed to initialize sync: %w", err)
	}
	if err := e.initSemanticGateway(); err != nil {
		return err
	}

	go func() {
		if token := e.client.Connect(); token.Wait() && token.Error() != nil {
			e.logger.Error("Gateway MQTT Connect Failed", zap.Error(token.Error()))
		}
	}()

	// Start telemetry reporting loop
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.semantic.wg.Add(1)
	go e.semantic.loop(ctx)
	go e.telemetryLoop(ctx)
	go e.alarmVideoUploadLoop(ctx)
	go e.runtimeApplyRetryLoop(ctx)
	go e.maintenanceHandoverLoop(ctx)

	// Subscribe to local core events for telemetry routing to Platform
	e.localEventSubs[types.EventDeviceStatusChanged] = e.ctx.SubscribeEvent(types.EventDeviceStatusChanged, e.handleLocalEvent)
	e.localEventSubs[types.EventPropertyReported] = e.ctx.SubscribeEvent(types.EventPropertyReported, e.handleLocalEvent)
	e.localEventSubs[types.EventEventReported] = e.ctx.SubscribeEvent(types.EventEventReported, e.handleLocalEvent)

	// Keep the full change and idempotency history until snapshot recovery is
	// available; an offline gateway may reconnect after the retention window.

	return nil
}

func newGatewayMQTTOptions() *mqtt.ClientOptions {
	// Sync callbacks publish replies and wait for their QoS 1 receipts. Ordered
	// callbacks can block the MQTT reader behind a burst of inbound messages,
	// preventing those same receipts from arriving. Sync sessions serialize
	// cursor updates themselves; MQTT dispatch must remain free to receive.
	return mqtt.NewClientOptions().SetOrderMatters(false)
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

func (e *gatewayEngineImpl) runtimeApplyRetryLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// A crash after committing a synchronized change but before activating the
	// runtime leaves a pending row. Pick it up as soon as the engine starts.
	e.retryRuntimeApplies()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.retryRuntimeApplies()
		}
	}
}

func (e *gatewayEngineImpl) retryRuntimeApplies() {
	rows, err := store.ListUnappliedRuntime(100)
	if err != nil {
		e.logger.Warn("Could not list gateway runtime apply retries", zap.Error(err))
		return
	}
	for _, row := range rows {
		e.runtimeApplyMu.Lock()
		_ = e.activateGatewayRuntimeLocked(row.EntityType, row.EntityCode, row.ChangeID, true)
		e.runtimeApplyMu.Unlock()
	}
	e.flushUnreportedRuntime()
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
	if e.semantic != nil {
		e.semantic.wg.Wait()
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
	c.Subscribe(fmt.Sprintf("noyo/cascade/gw/%s/semantic/down", e.gatewayCode), 1, e.handleSemanticDown)
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

	// The change stream carries every device and product mutation in both
	// directions. Deletion travels as an explicit record in this stream, which
	// is what removes any need to infer it from a snapshot's contents.
	syncDownTopicName := syncDownTopic(e.gatewayCode)
	c.Subscribe(syncDownTopicName, 1, e.handleSyncDown)

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

// handleConfigVersion reacts to the platform's configuration nudge.
//
// It deliberately no longer pulls the whole configuration. A nudge only
// re-triggers the handshake, after which the platform replays exactly the
// changes this gateway has not acknowledged. Pulling a full snapshot instead
// would overwrite local edits with platform state and — before the change stream
// existed — delete local devices merely because they were absent from it.
func (e *gatewayEngineImpl) handleConfigVersion(client mqtt.Client, msg mqtt.Message) {
	var payload struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		return
	}

	localVer := e.configVersion.Load()
	if payload.Version <= localVer {
		return
	}
	e.logger.Info("Platform configuration changed; catching up from the change stream",
		zap.Int64("local_version", localVer),
		zap.Int64("platform_version", payload.Version))
	e.configVersion.Store(payload.Version)
	// Always verify registration to handle the case where the gateway was disabled
	e.sendRegisterRequest()
	e.announceSyncPosition()
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
				e.logger.Info("Platform online: re-handshaking to replay unsynchronized changes")
				e.publishGatewayStateReplay()
				e.announceSyncPosition()
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

		// Announce our position and let the platform replay whatever we have not
		// acknowledged. A fresh gateway has an empty change log, so this also
		// performs the initial provisioning without any full-snapshot comparison.
		e.announceSyncPosition()
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

	if protectedRemoteConfigMutation(cmd.Method) {
		e.configMutationMu.Lock()
		defer e.configMutationMu.Unlock()
		reply["code"] = 409
		reply["message"] = "Configuration writes require durable synchronization"
		goto SEND_REPLY
	}
	if cmd.Method == "video_inspection" {
		var request inspection.RemoteCommand
		body, err := json.Marshal(cmd.Params)
		if err != nil || len(body) > 512*1024 || json.Unmarshal(body, &request) != nil || request.Gateway != e.gatewayCode || !e.isRegistered.Load() {
			reply["code"], reply["message"] = 403, "inspection_gateway_denied"
			goto SEND_REPLY
		}
		handler, ok := e.ctx.GetPlugin("video_inspection").(inspection.RemoteHandler)
		if !ok {
			reply["code"], reply["message"] = 503, "inspection_unavailable"
			goto SEND_REPLY
		}
		data, err := handler.HandleInspectionCommand(request)
		if err != nil {
			reply["code"], reply["message"] = 409, err.Error()
		} else {
			reply["data"] = data
		}
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
