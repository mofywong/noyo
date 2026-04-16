package aiot

import (
	_ "embed"
	"noyo/core"
	"noyo/core/platform"
	"noyo/core/types"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

//go:embed icon.svg
var icon []byte

// Config defines the configuration for AIoT Plugin
type Config struct {
	EnableTLS          bool   `yaml:"enable_tls" json:"enable_tls" title_en:"Enable TLS" title_zh:"开启TLS" desc_en:"Enable TLS encryption for MQTT connection" desc_zh:"开启MQTT连接的TLS加密"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" json:"insecure_skip_verify" title_en:"Skip Verify" title_zh:"跳过证书验证" desc_en:"Skip TLS certificate verification (insecure)" desc_zh:"跳过TLS证书验证（不安全）"`
	Broker             string `yaml:"broker" json:"broker" title_en:"Broker Address" title_zh:"Broker地址" desc_en:"MQTT Broker Address (e.g., localhost:1883)" desc_zh:"MQTT代理地址 (例如: localhost:1883)"`
	Username           string `yaml:"username" json:"username" title_en:"Username" title_zh:"用户名" desc_en:"MQTT Username" desc_zh:"MQTT连接用户名"`
	Password           string `yaml:"password" json:"password" title_en:"Password" title_zh:"密码" desc_en:"MQTT Password" desc_zh:"MQTT连接密码"`
	ClientID           string `yaml:"client_id" json:"client_id" title_en:"Client ID" title_zh:"客户端ID" desc_en:"MQTT Client ID (Leave empty to auto-generate)" desc_zh:"MQTT客户端ID (留空则自动生成)"`
	GatewayCode        string `yaml:"gateway_code" json:"gateway_code" title_en:"Gateway Code" title_zh:"网关编码" desc_en:"Unique identifier for this gateway" desc_zh:"本网关的唯一标识编码"`
}

// AiotPlugin implements the IPlugin interface for AIoT Platform
type AiotPlugin struct {
	platform.BasePlatformPlugin
	Config Config
	client mqtt.Client
	Logger *zap.Logger
}

func init() {
	core.InstallPlugin[AiotPlugin](core.PluginMeta{
		Name: "AIoT",
		Title: map[string]string{
			"en": "AIoT Platform",
			"zh": "AIoT 物联网平台",
		},
		Description: map[string]string{
			"en": "Uplink plugin for AIoT Platform via MQTT",
			"zh": "通过 MQTT 接入 AIoT 物联网平台",
		},
		Category: types.PluginCategoryPlatform,
		DefaultYaml: `
enable_tls: false
broker: "broker.emqx.io:1883"
username: ""
password: ""
client_id: ""
gateway_code: "gateway_001"
`,
		Icon: icon,
	})
}

// Init implements IPlatformPlugin
func (p *AiotPlugin) Init(ctx platform.Context) error {
	if err := p.BasePlatformPlugin.Init(ctx); err != nil {
		return err
	}
	p.Logger = ctx.GetLogger()

	// Register API
	if err := ctx.RegisterHTTPHandler("/api/extension/aiot/status", p.API_Status); err != nil {
		p.Ctx.LogError("Failed to register status API", err)
	}

	// Load Config to p.Config?
	// BasePlatformPlugin loads config into p.ConfigMap
	// We need to map it to struct or use map.
	// Since AIoT uses struct, let's load it manually or rely on Base.
	// Base Init sets Ctx.
	// We should load config from ctx.GetConfig() into p.Config structure.
	// Or use json unmarshal/mapstructure.
	// Load Config logic moved to Start
	// But p.Config definitions have yaml tags, not json tags?
	// Struct has yaml tags. JSON tags are implicit.
	// Let's assume generic map->struct binding.
	// core/utils might have it, or just use ConfigMap if Config is not complex.
	// AIoT Config has yaml tags.
	// Let's assume we can use the map directly or we need a helper.
	// For now, I'll skip complex config loading and assume defaults/empty if mapping fails, or implement manual mapping.
	// Actually, BasePlugin in legacy did `p.Plugin.Config = &p.Config`.
	// For new plugin, we fetch config from Context.
	// I'll leave Config loading for Start, or do it here.

	return nil
}

// API_Status implements a test API: GET /api/extension/aiot/status
func (p *AiotPlugin) API_Status(r *ghttp.Request) {
	// Fetch the latest active plugin instance to avoid stale reference after hot-reload
	server := p.Ctx.GetCoreServer().(*core.Server)
	activePlugin, ok := server.Manager.GetPlugin("AIoT").(*AiotPlugin)
	if !ok || activePlugin == nil {
		r.Response.WriteJson(map[string]interface{}{
			"plugin": "aiot",
			"status": "disconnected",
		})
		return
	}

	status := "disconnected"
	if activePlugin.client != nil && activePlugin.client.IsConnected() {
		status = "connected"
	}

	r.Response.WriteJson(map[string]interface{}{
		"plugin":       "aiot",
		"status":       status,
		"broker":       activePlugin.Config.Broker,
		"gateway_code": activePlugin.Config.GatewayCode,
		"ts":           time.Now(),
	})
}

// Start and Stop are implemented in mqtt.go

// PushData implements IPlatformPlugin
func (p *AiotPlugin) PushData(data *platform.DataModel) error {
	// Map DataModel to legacy ReportX logic
	meta := core.DeviceMeta{
		DeviceCode:  data.DeviceCode,
		ProductCode: data.ProductCode,
	}

	switch data.Type {
	case types.DataTypeProperty:
		return p.ReportBatchProperties(meta, data.Payload)
	case types.DataTypeEvent:
		// UniqueId is EventCode
		return p.ReportEvent(meta, data.UniqueId, data.Payload)
	case types.DataTypeStatus:
		if status, ok := data.Payload["status"].(string); ok {
			return p.ReportStatus(meta, status)
		}
	}
	return nil
}
