package core

import (
	"encoding/json"
	"noyo/core/protocol"
	"noyo/core/types"
	"time"

	"go.uber.org/zap"
)

// ProtocolContextImpl implements protocol.Context
type ProtocolContextImpl struct {
	pluginName string
	server     *Server
	config     map[string]interface{}
	logger     *zap.Logger
}

func NewProtocolContext(name string, s *Server, config interface{}) *ProtocolContextImpl {
	cfgMap := make(map[string]interface{})
	if config != nil {
		// Avoid double JSON serialization by type assertion
		if m, ok := config.(map[string]interface{}); ok {
			cfgMap = m
		} else {
			// Fallback: use JSON for complex types
			b, _ := json.Marshal(config)
			json.Unmarshal(b, &cfgMap)
		}
	}

	return &ProtocolContextImpl{
		pluginName: name,
		server:     s,
		config:     cfgMap,
		logger:     s.Logger.With(zap.String("plugin", name), zap.String("type", "protocol")),
	}
}

func (c *ProtocolContextImpl) ReportProperty(device types.DeviceMeta, key string, value interface{}) error {
	return c.server.DeviceManager.ReportDeviceProperties(device, map[string]interface{}{key: value})
}

func (c *ProtocolContextImpl) ReportBatchProperties(device types.DeviceMeta, values map[string]interface{}) error {
	return c.server.DeviceManager.ReportDeviceProperties(device, values)
}

func (c *ProtocolContextImpl) ReportEvent(device types.DeviceMeta, eventCode string, params map[string]interface{}) error {
	return c.server.DeviceManager.ReportDeviceEvent(device, eventCode, params)
}

func (c *ProtocolContextImpl) ReportStatus(device types.DeviceMeta, status string) error {
	c.server.DeviceManager.ReportDeviceStatus(device.DeviceCode, DeviceStatus{
		Online:     status == "online",
		LastActive: time.Now(),
		LastReport: time.Now(),
		LastStatus: status,
	})
	return nil
}

func (c *ProtocolContextImpl) ReportOnline(device types.DeviceMeta, online bool) error {
	status := "offline"
	if online {
		status = "online"
	}
	return c.ReportStatus(device, status)
}

func (c *ProtocolContextImpl) GetConfig() map[string]interface{} {
	return c.config
}

func (c *ProtocolContextImpl) GetDeviceStatus(deviceCode string) (*protocol.DeviceStatus, bool) {
	status, ok := c.server.DeviceManager.GetStatus(deviceCode)
	if !ok {
		return nil, false
	}
	// Convert core.DeviceStatus to protocol.DeviceStatus
	return &protocol.DeviceStatus{
		Online:     status.Online,
		LastActive: status.LastActive.Format(time.RFC3339),
	}, true
}

func (c *ProtocolContextImpl) GetProduct(productCode string) (*types.ProductMeta, bool) {
	prod, ok := c.server.DeviceManager.GetProduct(productCode)
	if !ok {
		return nil, false
	}
	// Convert core.ProductMeta to types.ProductMeta
	// Actually core.ProductMeta is alias to types.ProductMeta now?
	// Let's check imports. contexts.go imports core. Core imports types.
	// core.ProductMeta IS types.ProductMeta via alias in plugin.go?
	// No, plugin.go is in core.
	// We need to check types/meta.go again.
	// Explicit conversion or cast if same type.
	return &prod, true
}

func (c *ProtocolContextImpl) GetLogger() *zap.Logger {
	return c.server.Logger.With(zap.String("plugin", c.pluginName))
}

func (c *ProtocolContextImpl) ReloadRegistry() error {
	return c.server.DeviceManager.Registry.Reload()
}

func (c *ProtocolContextImpl) RegisterHTTPHandler(path string, handler interface{}) error {
	c.server.WebServer.BindHandler(path, handler)
	return nil
}

func (c *ProtocolContextImpl) GetPlugin(name string) interface{} {
	return c.server.Manager.GetPlugin(name)
}

func (c *ProtocolContextImpl) LogDebug(msg string, fields ...interface{}) {
	c.logger.Sugar().Debugw(msg, fields...)
}

func (c *ProtocolContextImpl) LogInfo(msg string, fields ...interface{}) {
	// Convert fields to zap fields if possible, or just log
	// Simple wrapper implementation
	c.logger.Sugar().Infow(msg, fields...)
}

func (c *ProtocolContextImpl) LogError(msg string, err error) {
	c.logger.Error(msg, zap.Error(err))
}

// PlatformContextImpl implements platform.Context
type PlatformContextImpl struct {
	pluginName string
	server     *Server
	config     map[string]interface{}
	logger     *zap.Logger
}

func NewPlatformContext(name string, s *Server, config interface{}) *PlatformContextImpl {
	cfgMap := make(map[string]interface{})
	if config != nil {
		// Avoid double JSON serialization by type assertion
		if m, ok := config.(map[string]interface{}); ok {
			cfgMap = m
		} else {
			// Fallback: use JSON for complex types
			b, _ := json.Marshal(config)
			json.Unmarshal(b, &cfgMap)
		}
	}

	return &PlatformContextImpl{
		pluginName: name,
		server:     s,
		config:     cfgMap,
		logger:     s.Logger.With(zap.String("plugin", name), zap.String("type", "platform")),
	}
}

func (c *PlatformContextImpl) IssueCommand(deviceCode string, cmdCode string, params map[string]interface{}) (interface{}, error) {
	if cmdCode == "set_properties" {
		err := c.server.DeviceManager.SetDeviceProperties(deviceCode, params)
		return nil, err
	}
	return c.server.DeviceManager.CallDeviceService(deviceCode, cmdCode, params)
}

func (c *PlatformContextImpl) GetConfig() map[string]interface{} {
	return c.config
}

func (c *PlatformContextImpl) GetCoreServer() interface{} {
	return c.server
}

func (c *PlatformContextImpl) LogInfo(msg string, fields ...interface{}) {
	c.logger.Sugar().Infow(msg, fields...)
}

func (c *PlatformContextImpl) LogError(msg string, err error) {
	c.logger.Error(msg, zap.Error(err))
}

func (c *PlatformContextImpl) GetLogger() *zap.Logger {
	return c.server.Logger.With(zap.String("plugin", c.pluginName))
}

func (c *PlatformContextImpl) RegisterHTTPHandler(path string, handler interface{}) error {
	c.server.WebServer.BindHandler(path, handler)
	return nil
}

func (c *PlatformContextImpl) GetOnlineDevices() ([]types.DeviceMeta, error) {
	codes := c.server.DeviceManager.GetOnlineDeviceCodes()
	var metas []types.DeviceMeta
	for _, code := range codes {
		// Use GetDeviceMeta from DeviceManager or Store
		meta, _, err := c.server.DeviceManager.GetDeviceMeta(code)
		if err != nil {
			continue
		}
		metas = append(metas, *meta)
	}
	return metas, nil
}

func (c *PlatformContextImpl) GetDeviceData(deviceCode string) map[string]interface{} {
	data := c.server.DeviceManager.GetLatestData(deviceCode)
	if data == nil {
		return make(map[string]interface{})
	}
	return data
}

func (c *PlatformContextImpl) GetEnabledProtocols() ([]types.PluginMeta, error) {
	var protocols []types.PluginMeta
	for _, p := range c.server.Manager.ProtocolPlugins {
		if p.IsEnabled() {
			protocols = append(protocols, *p.GetMeta())
		}
	}
	return protocols, nil
}

func (c *PlatformContextImpl) SubscribeEvent(eventType types.EventType, handler func(event types.Event)) uint64 {
	return c.server.DeviceManager.EventBus.SubscribeWithID(eventType, handler)
}

func (c *PlatformContextImpl) UnsubscribeEvent(eventType types.EventType, id uint64) {
	c.server.DeviceManager.EventBus.Unsubscribe(eventType, id)
}

func (c *PlatformContextImpl) PublishEvent(event types.Event) {
	c.server.DeviceManager.EventBus.Publish(event)
}

func (c *PlatformContextImpl) ReportDeviceProperties(deviceCode string, properties map[string]interface{}) error {
	meta, _, err := c.server.DeviceManager.GetDeviceMeta(deviceCode)
	if err != nil {
		return err
	}
	return c.server.DeviceManager.ReportDeviceProperties(*meta, properties)
}

func (c *PlatformContextImpl) ReportDeviceEvent(deviceCode string, eventId string, params map[string]interface{}) error {
	meta, _, err := c.server.DeviceManager.GetDeviceMeta(deviceCode)
	if err != nil {
		return err
	}
	return c.server.DeviceManager.ReportDeviceEvent(*meta, eventId, params)
}
