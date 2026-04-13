package mqtt_api

import (
	"context"
	_ "embed"
	"noyo/core"
	"noyo/core/platform"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

//go:embed icon.svg
var icon []byte

// PublishJob represents a message to be published
type PublishJob struct {
	Topic    string
	QoS      byte
	Retained bool
	Payload  interface{} // []byte or string
}

// Config defines the configuration for MQTTAPI Plugin
type Config struct {
	EnableTLS          bool   `yaml:"enable_tls" title_en:"Enable TLS" title_zh:"开启TLS" desc_en:"Enable TLS encryption for MQTT connection" desc_zh:"开启MQTT连接的TLS加密"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify" title_en:"Skip Verify" title_zh:"跳过证书验证" desc_en:"Skip TLS certificate verification (insecure)" desc_zh:"跳过TLS证书验证（不安全）"`
	Broker             string `yaml:"broker" title_en:"Broker Address" title_zh:"Broker地址" desc_en:"MQTT Broker Address (e.g., localhost:1883)" desc_zh:"MQTT代理地址 (例如: localhost:1883)"`
	Username           string `yaml:"username" title_en:"Username" title_zh:"用户名" desc_en:"MQTT Username" desc_zh:"MQTT连接用户名"`
	Password           string `yaml:"password" title_en:"Password" title_zh:"密码" desc_en:"MQTT Password" desc_zh:"MQTT连接密码"`
	ClientID           string `yaml:"client_id" title_en:"Client ID" title_zh:"客户端ID" desc_en:"MQTT Client ID (Leave empty to auto-generate)" desc_zh:"MQTT客户端ID (留空则自动生成)"`
	GatewayCode        string `yaml:"gateway_code" title_en:"Gateway Code" title_zh:"网关编码" desc_en:"Unique identifier for this gateway" desc_zh:"本网关的唯一标识编码"`
}

// MQTTAPIPlugin implements the IPlugin interface for MQTT API Platform
type MQTTAPIPlugin struct {
	platform.BasePlatformPlugin
	Config  Config
	client  mqtt.Client
	Logger  *zap.Logger
	msgChan chan PublishJob
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func init() {
	core.InstallPlugin[MQTTAPIPlugin](core.PluginMeta{
		Name: "MQTT_API",
		Title: map[string]string{
			"en": "MQTT API Platform",
			"zh": "MQTT接口",
		},
		Description: map[string]string{
			"en": "Uplink plugin for MQTT API Platform",
			"zh": "通过 MQTT 提供统一接口",
		},
		Category: "platform",
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
func (p *MQTTAPIPlugin) Init(ctx platform.Context) error {
	if err := p.BasePlatformPlugin.Init(ctx); err != nil {
		return err
	}
	p.Logger = ctx.GetLogger()

	// Init Async Publisher
	p.msgChan = make(chan PublishJob, 2000) // Buffer size 2000
	p.ctx, p.cancel = context.WithCancel(context.Background())

	// Register API
	if err := ctx.RegisterHTTPHandler("/api/extension/mqtt_api/status", p.API_Status); err != nil {
		p.Ctx.LogError("Failed to register status API", err)
	}

	return nil
}

// API_Status implements a test API: GET /api/extension/mqtt_api/status
func (p *MQTTAPIPlugin) API_Status(r *ghttp.Request) {
	status := "disconnected"
	if p.client != nil && p.client.IsConnected() {
		status = "connected"
	}

	r.Response.WriteJson(map[string]interface{}{
		"plugin":       "mqtt_api",
		"status":       status,
		"broker":       p.Config.Broker,
		"gateway_code": p.Config.GatewayCode,
		"ts":           time.Now(),
	})
}

// Start and Stop are implemented in mqtt.go

// PushData implements IPlatformPlugin
func (p *MQTTAPIPlugin) PushData(data *platform.DataModel) error {
	// Map DataModel to legacy ReportX logic
	meta := core.DeviceMeta{
		DeviceCode:  data.DeviceCode,
		ProductCode: data.ProductCode,
	}

	switch data.Type {
	case "property":
		return p.ReportBatchProperties(meta, data.Payload)
	case "event":
		// UniqueId is EventCode
		return p.ReportEvent(meta, data.UniqueId, data.Payload)
	case "status":
		if status, ok := data.Payload["status"].(string); ok {
			return p.ReportStatus(meta, status)
		}
	}
	return nil
}
