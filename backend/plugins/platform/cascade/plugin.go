package cascade

import (
	_ "embed"
	"encoding/json"

	"fmt"
	"noyo/core"
	"noyo/core/platform"
	"noyo/core/protocol"
	"noyo/core/store"
	"noyo/core/types"
	"sync"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/gogf/gf/v2/util/gconv"
	"go.uber.org/zap"
)

//go:embed icon.svg
var icon []byte

// CascadePlugin implements the IPlugin interface for cascading platform and gateway
type CascadePlugin struct {
	platform.BasePlatformPlugin
	Config Config
	Logger *zap.Logger

	// Engines based on the selected mode
	PlatformEngine PlatformEngine
	GatewayEngine  GatewayEngine

	ctx platform.Context
	wg  sync.WaitGroup
}

func init() {
	core.InstallPlugin[CascadePlugin](core.PluginMeta{
		Name: "cascade",
		Title: map[string]string{
			"en": "Cascade",
			"zh": "级联插件",
		},
		Description: map[string]string{
			"en": "Cloud-Edge synergy cascade plugin for managing sub-gateways and devices.",
			"zh": "云边协同级联插件，支持平台与网关两种模式，实现设备统一纳管与大文件分发。",
		},
		Category: types.PluginCategoryPlatform,
		DefaultYaml: `
mode: "platform"
mqtt_url: "tcp://127.0.0.1:1883"
username: "admin"
password: "password"
gateway_sn: "GW-AUTO-001"
`,
		Icon: icon,
	})
}

// GetDeviceConfigSchema implements IProtocolPlugin
func (p *CascadePlugin) GetDeviceConfigSchema(productCode string) ([]byte, error) {
	return nil, nil
}

// GetSubDeviceConfigSchema implements IProtocolPlugin
func (p *CascadePlugin) GetSubDeviceConfigSchema(productCode string) ([]byte, error) {
	return nil, nil
}

// GetImportTemplateLayout implements IProtocolPlugin
func (p *CascadePlugin) GetImportTemplateLayout(lang string) interface{} {
	return nil
}

// ProtocolMappingRequired implements IProtocolPlugin
// Cascade (作为网关通道) 自身不需要物模型映射配置，因此返回 false，避免前端强制校验
func (p *CascadePlugin) ProtocolMappingRequired() bool {
	return false
}

// Discover implements IProtocolPlugin
func (p *CascadePlugin) Discover(params map[string]interface{}) ([]protocol.DiscoveredDevice, error) {
	return nil, fmt.Errorf("discovery not supported")
}

// BatchAddDevice implements IProtocolPlugin
func (p *CascadePlugin) BatchAddDevice(devices []types.DeviceMeta) error {
	return nil
}

// BatchRemoveDevice implements IProtocolPlugin
func (p *CascadePlugin) BatchRemoveDevice(deviceCodes []string) error {
	return nil
}

// StartDevice implements IProtocolPlugin
func (p *CascadePlugin) StartDevice(deviceCode string) error {
	return nil
}

// StopDevice implements IProtocolPlugin
func (p *CascadePlugin) StopDevice(deviceCode string) error {
	return nil
}

// WritePoint implements IProtocolPlugin
func (p *CascadePlugin) WritePoint(device types.DeviceMeta, pointCode string, value interface{}) error {
	return fmt.Errorf("write point not supported directly")
}

// CallService implements IProtocolPlugin. It intercepts commands on the Platform and forwards them to the Gateway.
func (p *CascadePlugin) CallService(device types.DeviceMeta, serviceCode string, params map[string]interface{}) (interface{}, error) {
	if p.Config.Mode != "platform" {
		return nil, fmt.Errorf("CallService is only valid in platform mode")
	}

	// 1. Determine Gateway SN
	// 注意：device.ParentCode 可能被 GetDeviceMeta 的 cascade 特殊处理抹掉了，
	// 所以需要从 store 查询原始设备的 ParentCode 来确定网关 SN。
	gwSn := device.DeviceCode // Default: if it's the gateway itself
	if rawDevice, err := store.GetDevice(device.DeviceCode); err == nil && rawDevice != nil && rawDevice.ParentCode != "" {
		gwSn = rawDevice.ParentCode
	}

	p.Logger.Info("CallService routing command",
		zap.String("device", device.DeviceCode),
		zap.String("gateway_sn", gwSn),
		zap.String("service", serviceCode))

	// 2. Prepare Command Payload
	cmdId := fmt.Sprintf("%d", time.Now().UnixNano())
	payload := map[string]interface{}{
		"id":          cmdId,
		"version":     "1.0",
		"deviceCode":  device.DeviceCode,
		"productCode": device.ProductCode,
		"method":      "service_invoke",
		"params": map[string]interface{}{
			"service_id": serviceCode,
			"params":     params,
		},
		"timestamp": time.Now().UnixMilli(),
	}

	if serviceCode == "set_properties" {
		payload["method"] = "property_set"
		payload["params"] = params
	}

	payloadBytes, _ := json.Marshal(payload)

	// 3. Publish to MQTT via Engine
	engine, ok := p.PlatformEngine.(*platformEngineImpl)
	if !ok || engine.client == nil || !engine.client.IsConnected() {
		return nil, fmt.Errorf("platform MQTT client not connected")
	}

	return engine.SendCommand(gwSn, cmdId, payloadBytes)
}

// IsGatewayDevice implements IGatewayRouter
func (p *CascadePlugin) IsGatewayDevice(gwSn string) bool {
	dev, err := store.GetDevice(gwSn)
	if err != nil || dev == nil {
		return false
	}
	product, err := store.GetProduct(dev.ProductCode)
	if err != nil || product == nil {
		return false
	}
	return product.ProtocolName == "cascade"
}

// SendCommandToGateway implements IGatewayRouter
func (p *CascadePlugin) SendCommandToGateway(gwSn string, cmdID string, payload []byte) (interface{}, error) {
	if p.PlatformEngine == nil {
		return nil, fmt.Errorf("platform engine not available")
	}
	engine, ok := p.PlatformEngine.(*platformEngineImpl)
	if !ok {
		return nil, fmt.Errorf("platform engine is not of correct type")
	}
	return engine.SendCommand(gwSn, cmdID, payload)
}

func (p *CascadePlugin) GetConfigSchema() *core.PluginConfigSchema {
	meta := p.GetMeta()
	return &core.PluginConfigSchema{
		PluginName:  meta.Name,
		Title:       meta.Title,
		Description: meta.Description,
		Fields: []core.ConfigField{
			{
				Name:        "mode",
				Type:        "select",
				Title:       map[string]string{"en": "Operating Mode", "zh": "运行模式"},
				Description: map[string]string{"en": "Select platform or gateway mode", "zh": "选择平台或网关模式"},
				Value:       p.Config.Mode,
				Options: []map[string]string{
					{"label": "平台端 (Platform)", "value": "platform"},
					{"label": "下级网关 (Gateway)", "value": "gateway"},
				},
			},
			{
				Name:        "mqtt_url",
				Type:        "string",
				Title:       map[string]string{"en": "MQTT Server URL", "zh": "MQTT连接地址"},
				Description: map[string]string{"en": "e.g., tcp://127.0.0.1:1883", "zh": "例如：tcp://127.0.0.1:1883"},
				Value:       p.Config.MqttUrl,
			},
			{
				Name:        "username",
				Type:        "string",
				Title:       map[string]string{"en": "Username", "zh": "MQTT用户名"},
				Description: map[string]string{"en": "MQTT Broker username", "zh": "MQTT Broker用户名"},
				Value:       p.Config.Username,
			},
			{
				Name:        "password",
				Type:        "password",
				Title:       map[string]string{"en": "Password", "zh": "MQTT密码"},
				Description: map[string]string{"en": "MQTT Broker password", "zh": "MQTT Broker密码"},
				Value:       p.Config.Password,
			},
			{
				Name:        "gateway_sn",
				Type:        "string",
				Title:       map[string]string{"en": "Gateway SN", "zh": "网关序列号 (SN)"},
				Description: map[string]string{"en": "Required only in Gateway mode", "zh": "仅在网关模式下需要填写"},
				Value:       p.Config.GatewaySn,
			},
			{
				Name:        "gateway_name",
				Type:        "string",
				Title:       map[string]string{"en": "Gateway Name", "zh": "网关名称"},
				Description: map[string]string{"en": "Gateway Name for registration", "zh": "网关名称，注册时上报"},
				Value:       p.Config.GatewayName,
			},
		},
	}
}

// Init implements IPlatformPlugin Init lifecycle
func (p *CascadePlugin) Init(ctx platform.Context) error {
	if err := p.BasePlatformPlugin.Init(ctx); err != nil {
		return err
	}
	p.Logger = ctx.GetLogger()
	p.ctx = ctx

	// Load Configuration
	if cfgMap := ctx.GetConfig(); cfgMap != nil {
		if err := gconv.Scan(cfgMap, &p.Config); err != nil {
			p.Logger.Error("Failed to parse cascade config", zap.Error(err))
		}
	}

	// Initialize Engine based on Mode
	if p.Config.Mode == "platform" {
		p.PlatformEngine = NewPlatformEngine(p.ctx, p.Logger, &p.Config)
	} else if p.Config.Mode == "gateway" {
		p.GatewayEngine = NewGatewayEngine(p.ctx, p.Logger, &p.Config)
	}

	// Register HTTP routes
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/status", p.handleGetStatus)

	return nil
}

func (p *CascadePlugin) handleGetStatus(r *ghttp.Request) {
	connected := false
	if p.Config.Mode == "platform" && p.PlatformEngine != nil {
		engine, ok := p.PlatformEngine.(*platformEngineImpl)
		if ok && engine.client != nil {
			connected = engine.client.IsConnected()
		}
	} else if p.Config.Mode == "gateway" && p.GatewayEngine != nil {
		engine, ok := p.GatewayEngine.(*gatewayEngineImpl)
		if ok && engine.client != nil {
			connected = engine.client.IsConnected()
		}
	}

	r.Response.WriteJson(map[string]interface{}{
		"plugin":       "cascade",
		"status":       map[bool]string{true: "connected", false: "disconnected"}[connected],
		"connected":    connected,
		"mode":         p.Config.Mode,
		"broker":       p.Config.MqttUrl,
		"gateway_code": p.Config.GatewaySn,
		"ts":           time.Now(),
	})
}

// Start hooks into the Plugin Start lifecycle
func (p *CascadePlugin) Start() error {
	if err := p.BasePlatformPlugin.Start(); err != nil {
		return err
	}

	p.Logger.Info("Cascade plugin starting", zap.String("mode", p.Config.Mode))

	if p.Config.Mode == "platform" && p.PlatformEngine != nil {
		return p.PlatformEngine.Start()
	} else if p.Config.Mode == "gateway" && p.GatewayEngine != nil {
		return p.GatewayEngine.Start()
	}

	return nil
}

// Stop hooks into the Plugin Stop lifecycle
func (p *CascadePlugin) Stop() error {
	p.Logger.Info("Cascade plugin stopping")

	if p.PlatformEngine != nil {
		p.PlatformEngine.Stop()
	}
	if p.GatewayEngine != nil {
		p.GatewayEngine.Stop()
	}

	return p.BasePlatformPlugin.Stop()
}

// PushData is a no-op for cascade plugin usually, data flows via MQTT internally
func (p *CascadePlugin) PushData(data *platform.DataModel) error {
	return nil
}

// Status returns the current status of the plugin
func (p *CascadePlugin) Status() string {
	if p.Config.Mode == "platform" {
		return "Platform Mode Running"
	}
	if p.Config.Mode == "gateway" {
		return "Gateway Mode Running"
	}
	return "Unconfigured"
}
