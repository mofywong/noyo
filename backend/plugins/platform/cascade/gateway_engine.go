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
	"github.com/pion/stun/v3"
	"go.uber.org/zap"
)

type gatewayEngineImpl struct {
	ctx            platform.Context
	logger         *zap.Logger
	config         *Config
	client         mqtt.Client
	receivers      map[string]*FileReceiver
	receiversMux   sync.Mutex
	cancel         context.CancelFunc
	isRegistered   atomic.Bool
	platformOnline atomic.Bool
	configVersion  atomic.Int64
	localEventSubs map[types.EventType]uint64
}

func NewGatewayEngine(ctx platform.Context, logger *zap.Logger, cfg *Config) GatewayEngine {
	return &gatewayEngineImpl{
		ctx:       ctx,
		logger:    logger,
		config:         cfg,
		receivers:      make(map[string]*FileReceiver),
		localEventSubs: make(map[types.EventType]uint64),
	}
}

func (e *gatewayEngineImpl) Start() error {
	e.logger.Info("Gateway Engine Started", zap.String("mqtt_url", e.config.MqttUrl))

	if e.config.MqttUrl == "" {
		return fmt.Errorf("gateway mqtt_url is empty")
	}
	if e.config.GatewaySn == "" {
		return fmt.Errorf("gateway_sn is empty")
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(e.config.MqttUrl)
	// Use deterministic ClientID so broker properly handles session takeover and LWT
	opts.SetClientID(fmt.Sprintf("noyo-gw-cascade-%s", e.config.GatewaySn))
	opts.SetUsername(e.config.Username)
	opts.SetPassword(e.config.Password)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)   // 增加到 60s，避免系统负载高时 PING 超时触发 LWT
	opts.SetPingTimeout(20 * time.Second) // 增加 PING 超时容忍度

	// Set Last Will and Testament (LWT) for Gateway offline status
	// 使用固定时间戳 0，便于区分 LWT 触发的 offline 和主动发布的 offline
	willPayload := `{"status":"offline","timestamp":0}`
	opts.SetWill(fmt.Sprintf("noyo/cascade/gw/%s/status", e.config.GatewaySn), willPayload, 1, true)

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
						Topic:     e.config.GatewaySn,
						Payload:   props,
						Timestamp: time.Now().UnixMilli(),
					}
					payloadBytes, _ := json.Marshal(event)
					e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.config.GatewaySn), 1, false, payloadBytes)
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
		token := e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/status", e.config.GatewaySn), 1, true, []byte(offlinePayload))
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
	topic := fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.config.GatewaySn)
	payloadBytes, err := json.Marshal(event)
	if err == nil {
		e.client.Publish(topic, 1, false, payloadBytes)
	}
}

func (e *gatewayEngineImpl) subscribeTopics(c mqtt.Client) {
	configTopic := fmt.Sprintf("noyo/cascade/gw/%s/config/version", e.config.GatewaySn)
	c.Subscribe(configTopic, 1, e.handleConfigVersion)

	cmdTopic := fmt.Sprintf("noyo/cascade/gw/%s/command/request", e.config.GatewaySn)
	c.Subscribe(cmdTopic, 1, e.handleCommand)

	platformStatusTopic := "noyo/cascade/platform/status"
	c.Subscribe(platformStatusTopic, 1, e.handlePlatformStatus)

	regRespTopic := fmt.Sprintf("noyo/cascade/gw/%s/register/response", e.config.GatewaySn)
	if token := c.Subscribe(regRespTopic, 1, e.handleRegisterResponse); token.Wait() && token.Error() != nil {
		e.logger.Error("Failed to subscribe to register response topic", zap.Error(token.Error()))
	}

	e.sendRegisterRequest()

	fileMetaTopic := fmt.Sprintf("noyo/cascade/gw/%s/file/meta", e.config.GatewaySn)
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

	fileChunkTopic := fmt.Sprintf("noyo/cascade/gw/%s/file/chunk", e.config.GatewaySn)
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


// getPublicIP returns the gateway's public IP via STUN discovery.
func (e *gatewayEngineImpl) getPublicIP() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stunServers := []string{
		"stun:stun.l.google.com:19302",
		"stun:stun.cloudflare.com:3478",
	}
	for _, uriStr := range stunServers {
		uri, err := stun.ParseURI(uriStr)
		if err != nil {
			continue
		}
		client, err := stun.DialURI(uri, &stun.DialConfig{})
		if err != nil {
			continue
		}
		message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
		var publicIP string
		err = client.Do(message, func(res stun.Event) {
			if res.Error != nil {
				return
			}
			var xorAddr stun.XORMappedAddress
			if xorAddr.GetFrom(res.Message) == nil {
				publicIP = xorAddr.IP.String()
			} else {
				var mappedAddr stun.MappedAddress
				if mappedAddr.GetFrom(res.Message) == nil {
					publicIP = mappedAddr.IP.String()
				}
			}
		})
		client.Close()
		if err == nil && publicIP != "" {
			return publicIP
		}
		select {
		case <-ctx.Done():
			return ""
		default:
		}
	}
	return ""
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
		Timestamp int64            `json:"timestamp"`
		Products  []*store.Product `json:"products"`
		Devices   []*store.Device  `json:"devices"`
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

	// 1. Sync Products
	for _, p := range syncData.Products {
		p.ID = 0 // Clear Platform ID to avoid local SQLite primary key conflicts

		// 检查产品信息是否有变化，避免无变化时触发插件重载
		productChanged := true
		if existingP, err := store.GetProduct(p.Code); err == nil && existingP != nil {
			if existingP.Name == p.Name && existingP.ProtocolName == p.ProtocolName && existingP.Config == p.Config {
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

			// Automatically enable and reload the protocol plugin used by this product
			if p.ProtocolName != "" {
				pluginModel, _ := store.GetPlugin(p.ProtocolName)
				needsReload := false
				if pluginModel == nil {
					pluginModel = &store.PluginModel{
						Name:    p.ProtocolName,
						Enabled: true,
						Config:  "{}",
					}
					if err := store.DB.Save(pluginModel).Error; err != nil {
						e.logger.Error("Failed to save enabled state for protocol plugin", zap.String("plugin", p.ProtocolName), zap.Error(err))
					} else {
						needsReload = true
					}
				} else if !pluginModel.Enabled {
					pluginModel.Enabled = true
					if err := store.DB.Save(pluginModel).Error; err != nil {
						e.logger.Error("Failed to enable protocol plugin", zap.String("plugin", p.ProtocolName), zap.Error(err))
					} else {
						needsReload = true
					}
				}

				if needsReload {
					e.logger.Info("Auto-enabling and reloading protocol plugin for synced product", zap.String("plugin", p.ProtocolName), zap.String("product", p.Code))
					if err := coreServer.Manager.ReloadPlugin(p.ProtocolName); err != nil {
						e.logger.Error("Failed to reload protocol plugin automatically", zap.String("plugin", p.ProtocolName), zap.Error(err))
					}
				} else {
					// Check if it's already loaded in memory (e.g. system started with it disabled)
					if coreServer.Manager.GetPlugin(p.ProtocolName) == nil {
						if err := coreServer.Manager.LoadPlugin(p.ProtocolName); err != nil {
							e.logger.Error("Failed to load protocol plugin automatically", zap.String("plugin", p.ProtocolName), zap.Error(err))
						}
					}
				}
			}
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
	localDevices, _, err := store.ListDevices(0, 0)
	if err != nil {
		e.logger.Error("Failed to list local devices for cleanup", zap.Error(err))
	} else {
		for _, ld := range localDevices {
			// Skip gateway device itself
			if ld.Code == e.config.GatewaySn {
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
				e.logger.Info("Platform online: triggering sync request and re-publishing status")
				// Publish own online status again to ensure platform knows we are here
				onlineEvent := types.Event{
					Type:      types.EventDeviceStatusChanged,
					Topic:     e.config.GatewaySn,
					Payload:   types.DeviceStatusOnline,
					Timestamp: time.Now().UnixMilli(),
				}
				onlineBytes, _ := json.Marshal(onlineEvent)
				// 使用 retained=false，避免 broker 缓存网关旧的在线状态导致与遗嘱消息冲突
				e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.config.GatewaySn), 1, false, onlineBytes)
	
				// Also publish statuses of all sub-devices
				if coreServer, ok := e.ctx.GetCoreServer().(*core.Server); ok {
					allDevices := coreServer.DeviceManager.Registry.GetAllDevices()
					for _, dev := range allDevices {
						if dev.Code == e.config.GatewaySn {
							continue
						}
						if status, ok := coreServer.DeviceManager.GetStatus(dev.Code); ok {
							statusPayload := types.DeviceStatusOffline
							if status.Online {
								statusPayload = types.DeviceStatusOnline
							}
							devEvent := types.Event{
								Type:      types.EventDeviceStatusChanged,
								Topic:     dev.Code,
								Payload:   statusPayload,
								Timestamp: time.Now().UnixMilli(),
							}
							devBytes, _ := json.Marshal(devEvent)
							// 使用 retained=false，避免 broker 缓存旧的子设备状态
							// 平台重启时不应收到这些过期的 retained 消息
							e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.config.GatewaySn), 1, false, devBytes)
						}
					}
				}
	
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
	topic := fmt.Sprintf("noyo/cascade/gw/%s/register/request", e.config.GatewaySn)

	req := map[string]interface{}{
		"gateway_name": e.config.GatewayName,
	}
	reqBytes, _ := json.Marshal(req)

	e.client.Publish(topic, 1, false, reqBytes)
	e.logger.Info("Sent register request", zap.String("name", e.config.GatewayName))
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
		e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/status", e.config.GatewaySn), 1, true, []byte(onlinePayload))

		// Also publish statuses of all sub-devices
		if coreServer, ok := e.ctx.GetCoreServer().(*core.Server); ok {
			allDevices := coreServer.DeviceManager.Registry.GetAllDevices()
			for _, dev := range allDevices {
				if dev.Code == e.config.GatewaySn {
					continue
				}
				if status, ok := coreServer.DeviceManager.GetStatus(dev.Code); ok {
					statusPayload := types.DeviceStatusOffline
					if status.Online {
						statusPayload = types.DeviceStatusOnline
					}
					devEvent := types.Event{
						Type:      types.EventDeviceStatusChanged,
						Topic:     dev.Code,
						Payload:   statusPayload,
						Timestamp: time.Now().UnixMilli(),
					}
					devBytes, _ := json.Marshal(devEvent)
					// 使用 retained=false，避免 broker 缓存旧的子设备状态
					// 平台重启时不应收到这些过期的 retained 消息
					e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/telemetry/up", e.config.GatewaySn), 1, false, devBytes)
				}
			}
		}

		// STUN 公网 IP 发现并上报给平台
		e.discoverAndPublishMediaAddress()

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
		e.client.Publish(fmt.Sprintf("noyo/cascade/gw/%s/status", e.config.GatewaySn), 1, true, []byte(offlinePayload))
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
	topic := fmt.Sprintf("noyo/cascade/gw/%s/sync/request", e.config.GatewaySn)
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

	if cmd.Method == "service_invoke" || cmd.Method == "property_set" {
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

// discoverAndPublishMediaAddress uses STUN to discover the gateway's public IP
// and publishes it to the platform via MQTT so that the platform can use it
// in SDP offers for cameras behind this gateway.
func (e *gatewayEngineImpl) discoverAndPublishMediaAddress() {
	if e.client == nil || !e.client.IsConnected() {
		return
	}

	go func() {
		stunServers := []string{
			"stun:stun.l.google.com:19302",
			"stun:stun1.l.google.com:19302",
			"stun:stun.cloudflare.com:3478",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		publicIP := ""
		for _, uriStr := range stunServers {
			uri, err := stun.ParseURI(uriStr)
			if err != nil {
				continue
			}
			client, err := stun.DialURI(uri, &stun.DialConfig{})
			if err != nil {
				continue
			}
			message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)
			err = client.Do(message, func(res stun.Event) {
				if res.Error != nil {
					client.Close()
					return
				}
				var xorAddr stun.XORMappedAddress
				if xorAddr.GetFrom(res.Message) == nil {
					publicIP = xorAddr.IP.String()
				} else {
					var mappedAddr stun.MappedAddress
					if mappedAddr.GetFrom(res.Message) == nil {
						publicIP = mappedAddr.IP.String()
					}
				}
			})
			client.Close()
			if err == nil && publicIP != "" {
				break
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}

		if publicIP == "" {
			e.logger.Warn("STUN discovery failed, gateway media address not published")
			return
		}

		e.logger.Info("STUN discovery succeeded", zap.String("public_ip", publicIP))

		payload := map[string]interface{}{
			"public_ip": publicIP,
			"local_ip":  system.GetOutboundIP(),
			"timestamp": time.Now().UnixMilli(),
		}
		payloadBytes, _ := json.Marshal(payload)
		topic := fmt.Sprintf("noyo/cascade/gw/%s/media/address", e.config.GatewaySn)
		e.client.Publish(topic, 1, true, payloadBytes) // retained=true
		e.logger.Info("Published gateway media address", zap.String("topic", topic), zap.String("public_ip", publicIP))
	}()
}
