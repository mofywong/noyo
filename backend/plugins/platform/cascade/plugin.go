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

func (p *CascadePlugin) GetSetupSchema(mode string) *core.PluginSetupSchema {
	mode = core.NormalizeSetupMode(mode)
	if mode == core.SetupModeLocalProject {
		return nil
	}

	meta := p.GetMeta()
	if meta == nil {
		return nil
	}

	fields := []core.PluginSetupField{
		{
			Name:        "mqtt_url",
			Type:        "string",
			Title:       map[string]string{"en": "Cascade MQTT URL", "zh": "级联 MQTT 地址"},
			Description: map[string]string{"en": "Shared broker used by platform and managed gateways, e.g. tcp://127.0.0.1:1883.", "zh": "平台与托管网关共用的级联 Broker，例如 tcp://127.0.0.1:1883。"},
			Required:    mode == core.SetupModePlatformGateway,
		},
		{
			Name:  "username",
			Type:  "string",
			Title: map[string]string{"en": "MQTT Username", "zh": "MQTT 用户名"},
		},
		{
			Name:      "password",
			Type:      "password",
			Title:     map[string]string{"en": "MQTT Password", "zh": "MQTT 密码"},
			Sensitive: true,
		},
	}

	if mode == core.SetupModePlatformGateway {
		fields = append(fields,
			core.PluginSetupField{
				Name:  "gateway_sn",
				Type:  "string",
				Title: map[string]string{"en": "Platform Gateway SN", "zh": "平台注册网关 SN"},
				Description: map[string]string{
					"en": "Must match the gateway device code that the platform pre-registered under the target project.",
					"zh": "必须与平台在目标项目下预登记的网关设备编码一致，平台据此完成项目绑定。",
				},
				Required: true,
			},
			core.PluginSetupField{
				Name:        "gateway_name",
				Type:        "string",
				Title:       map[string]string{"en": "Gateway Display Name", "zh": "网关显示名称"},
				Description: map[string]string{"en": "Used only as the reported display name during registration.", "zh": "仅作为注册上报时的显示名称。"},
			},
		)
	}

	return &core.PluginSetupSchema{
		PluginName:  meta.Name,
		Title:       meta.Title,
		Description: meta.Description,
		Modes:       []string{mode},
		Fields:      fields,
	}
}

// Init implements IPlatformPlugin Init lifecycle
func (p *CascadePlugin) Init(ctx platform.Context) error {
	if err := p.BasePlatformPlugin.Init(ctx); err != nil {
		return err
	}
	p.Logger = ctx.GetLogger()
	p.ctx = ctx

	// Ensure gateway product exists unconditionally
	EnsureGatewayProduct(p.Logger)

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
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/stream", p.handleStream)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways", p.handleGatewayList)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/plugins", p.handleGatewayPlugins)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/plugins/:pluginName", p.handleGatewayPluginConfig)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/plugins/:pluginName/config", p.handleGatewayPluginConfigSave)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/plugins/:pluginName/status", p.handleGatewayPluginStatusSave)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/system/config", p.handleGatewaySystemConfig)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/license/status", p.handleGatewayLicenseStatus)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/license/upload", p.handleGatewayLicenseUpload)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/system/log/files", p.handleGatewayLogFiles)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/system/log/file", p.handleGatewayLogFile)
	_ = ctx.RegisterHTTPHandler("/api/extension/cascade/gateways/:gwSn/system/log/tail", p.handleGatewayLogTail)

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

func (p *CascadePlugin) handleStream(r *ghttp.Request) {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	r.Response.Header().Set("X-Accel-Buffering", "no")

	r.Response.Write("event: connected\ndata: {}\n\n")
	r.Response.Flush()

	ctx := r.Context()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	lastConnected := false
	first := true

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			connected := false
			if p.Config.Mode == "platform" && p.PlatformEngine != nil {
				if engine, ok := p.PlatformEngine.(*platformEngineImpl); ok && engine.client != nil {
					connected = engine.client.IsConnected()
				}
			} else if p.Config.Mode == "gateway" && p.GatewayEngine != nil {
				if engine, ok := p.GatewayEngine.(*gatewayEngineImpl); ok && engine.client != nil {
					connected = engine.client.IsConnected()
				}
			}

			if first || connected != lastConnected {
				data := map[string]interface{}{
					"plugin":       "cascade",
					"status":       map[bool]string{true: "connected", false: "disconnected"}[connected],
					"connected":    connected,
					"mode":         p.Config.Mode,
					"broker":       p.Config.MqttUrl,
					"gateway_code": p.Config.GatewaySn,
					"ts":           time.Now(),
				}
				b, _ := json.Marshal(data)
				msg := fmt.Sprintf("event: status\ndata: %s\n\n", string(b))
				if _, err := r.Response.Writer.Write([]byte(msg)); err != nil {
					return
				}
				r.Response.Flush()
				lastConnected = connected
				first = false
			} else {
				if _, err := r.Response.Writer.Write([]byte("event: heartbeat\ndata: {}\n\n")); err != nil {
					return
				}
				r.Response.Flush()
			}
		}
	}
}

func (p *CascadePlugin) handleGatewayList(r *ghttp.Request) {
	if p.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway management is only available in platform mode"})
		return
	}

	coreServer, _ := p.ctx.GetCoreServer().(*core.Server)
	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()
	devices, _, err := store.ListDevices(0, 0, tenantID, projectID)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	projectNames := map[uint]string{}
	if tenantID > 0 {
		var projects []store.Project
		query := store.DB.Model(&store.Project{}).Where("tenant_id = ?", tenantID)
		if projectID > 0 {
			query = query.Where("id = ?", projectID)
		}
		if err := query.Find(&projects).Error; err == nil {
			for _, project := range projects {
				projectNames[project.ID] = project.Name
			}
		}
	}

	type gatewayItem struct {
		SN          string `json:"sn"`
		Name        string `json:"name"`
		ProjectID   uint   `json:"projectId"`
		ProjectName string `json:"projectName"`
		ProductCode string `json:"productCode"`
		Enabled     bool   `json:"enabled"`
		Status      string `json:"status"`
		Online      bool   `json:"online"`
		OnlineAt    int64  `json:"onlineAt"`
		UpdatedAt   int64  `json:"updatedAt"`
	}

	items := make([]gatewayItem, 0)
	for _, dev := range devices {
		product, err := store.GetProduct(dev.ProductCode)
		if err != nil || product == nil || product.ProtocolName != "cascade" {
			continue
		}

		statusStr := types.DeviceStatusOffline
		online := false
		onlineAt := int64(0)
		if coreServer != nil && coreServer.DeviceManager != nil {
			if status, ok := coreServer.DeviceManager.GetStatus(dev.Code); ok {
				statusStr = status.LastStatus
				online = status.Online
				if status.Online && !status.LastActive.IsZero() {
					onlineAt = status.LastActive.UnixMilli()
				}
			}
		}

		items = append(items, gatewayItem{
			SN:          dev.Code,
			Name:        dev.Name,
			ProjectID:   dev.ProjectID,
			ProjectName: projectNames[dev.ProjectID],
			ProductCode: dev.ProductCode,
			Enabled:     dev.Enabled,
			Status:      statusStr,
			Online:      online,
			OnlineAt:    onlineAt,
			UpdatedAt:   dev.UpdatedAt.UnixMilli(),
		})
	}

	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": items})
}

func (p *CascadePlugin) handleGatewayPlugins(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	engine, ok := p.remotePluginEngine(r, gwSn)
	if !ok {
		return
	}
	data, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodList, "", nil)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": gatewayPluginStateCache.List(gwSn), "message": err.Error()})
		return
	}
	summaries, err := decodeRemotePluginSummaries(data)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
		return
	}
	for i := range summaries {
		gatewayPluginStateCache.SaveSnapshot(gwSn, summaries[i], time.Now())
		summaries[i] = gatewayPluginStateCache.MergeSummary(gwSn, summaries[i])
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaries})
}

func (p *CascadePlugin) handleGatewayPluginConfig(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	pluginName := p.getRouteParam(r, "pluginName")
	engine, ok := p.remotePluginEngine(r, gwSn)
	if !ok {
		return
	}
	data, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodConfigGet, pluginName, nil)
	if err != nil {
		if item, found := gatewayPluginStateCache.Get(gwSn, pluginName); found {
			r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item), "message": err.Error()})
			return
		}
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	summary, err := decodeRemotePluginSummary(data)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
		return
	}
	if item, found := gatewayPluginStateCache.Get(gwSn, pluginName); found && item.SyncState == remotePluginSyncPending {
		if _, conflict := gatewayPluginStateCache.MarkConflictIfGatewayChanged(gwSn, pluginName, summary.ConfigVersion); conflict {
			if conflictItem, ok := gatewayPluginStateCache.Get(gwSn, pluginName); ok {
				r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(conflictItem)})
				return
			}
		}
		if item.DesiredConfig != nil {
			applied, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodConfigSet, pluginName, map[string]interface{}{
				"config":       item.DesiredConfig,
				"base_version": item.BaseVersion,
			})
			if err == nil {
				if appliedSummary, decodeErr := decodeRemotePluginSummary(applied); decodeErr == nil {
					synced := gatewayPluginStateCache.MarkSynced(gwSn, *appliedSummary, time.Now())
					r.Response.WriteJson(map[string]interface{}{"code": 0, "data": synced})
					return
				}
			}
		}
	}
	gatewayPluginStateCache.SaveSnapshot(gwSn, *summary, time.Now())
	merged := gatewayPluginStateCache.MergeSummary(gwSn, *summary)
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": merged})
}

func (p *CascadePlugin) handleGatewayPluginConfigSave(r *ghttp.Request) {
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	pluginName := p.getRouteParam(r, "pluginName")
	var config map[string]interface{}
	if err := json.Unmarshal(r.GetBody(), &config); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	baseVersion := extractRemoteBaseVersion(config)
	enabled := desiredEnabledForConfig(gwSn, pluginName, config)
	engine, ok := p.remotePluginEngine(r, gwSn)
	if !ok {
		return
	}
	if r.Get("resolve").String() != "override" && baseVersion > 0 {
		if live, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodConfigGet, pluginName, nil); err == nil {
			if liveSummary, decodeErr := decodeRemotePluginSummary(live); decodeErr == nil && liveSummary.ConfigVersion > baseVersion {
				item, _ := gatewayPluginStateCache.SaveDesired(gwSn, pluginName, config, enabled, baseVersion, time.Now())
				gatewayPluginStateCache.MarkConflictIfGatewayChanged(gwSn, pluginName, liveSummary.ConfigVersion)
				if conflictItem, found := gatewayPluginStateCache.Get(gwSn, pluginName); found {
					item = conflictItem
				}
				r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item)})
				return
			}
		}
	}
	data, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodConfigSet, pluginName, map[string]interface{}{
		"config":       config,
		"base_version": baseVersion,
	})
	if err != nil {
		item, _ := gatewayPluginStateCache.SaveDesired(gwSn, pluginName, config, enabled, baseVersion, time.Now())
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item), "message": err.Error()})
		return
	}
	summary, err := decodeRemotePluginSummary(data)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
		return
	}
	synced := gatewayPluginStateCache.MarkSynced(gwSn, *summary, time.Now())
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": synced})
}

func (p *CascadePlugin) handleGatewayPluginStatusSave(r *ghttp.Request) {
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	pluginName := p.getRouteParam(r, "pluginName")
	var body struct {
		Enabled     bool  `json:"enabled"`
		BaseVersion int64 `json:"base_version"`
	}
	if err := json.Unmarshal(r.GetBody(), &body); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	baseVersion := body.BaseVersion
	config := map[string]interface{}{"enabled": body.Enabled}
	engine, ok := p.remotePluginEngine(r, gwSn)
	if !ok {
		return
	}
	if baseVersion > 0 {
		if live, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodConfigGet, pluginName, nil); err == nil {
			if liveSummary, decodeErr := decodeRemotePluginSummary(live); decodeErr == nil && liveSummary.ConfigVersion > baseVersion {
				item, _ := gatewayPluginStateCache.SaveDesired(gwSn, pluginName, config, body.Enabled, baseVersion, time.Now())
				gatewayPluginStateCache.MarkConflictIfGatewayChanged(gwSn, pluginName, liveSummary.ConfigVersion)
				if conflictItem, found := gatewayPluginStateCache.Get(gwSn, pluginName); found {
					item = conflictItem
				}
				r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item)})
				return
			}
		}
	}
	data, err := engine.SendRemotePluginCommand(gwSn, remotePluginMethodStatusSet, pluginName, map[string]interface{}{
		"enabled":      body.Enabled,
		"base_version": baseVersion,
	})
	if err != nil {
		item, _ := gatewayPluginStateCache.SaveDesired(gwSn, pluginName, config, body.Enabled, baseVersion, time.Now())
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item), "message": err.Error()})
		return
	}
	summary, err := decodeRemotePluginSummary(data)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
		return
	}
	synced := gatewayPluginStateCache.MarkSynced(gwSn, *summary, time.Now())
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": synced})
}

func (p *CascadePlugin) writeRemotePluginCommandResponse(r *ghttp.Request, method, pluginName string, params map[string]interface{}) {
	if p.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway plugin management is only available in platform mode"})
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	if gwSn == "" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway sn is required"})
		return
	}
	engine, ok := p.PlatformEngine.(*platformEngineImpl)
	if !ok || engine == nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": "platform engine not available"})
		return
	}
	data, err := engine.SendRemotePluginCommand(gwSn, method, pluginName, params)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
}

func (p *CascadePlugin) remotePluginEngine(r *ghttp.Request, gwSn string) (*platformEngineImpl, bool) {
	if p.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway plugin management is only available in platform mode"})
		return nil, false
	}
	if gwSn == "" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway sn is required"})
		return nil, false
	}
	if !p.ensureGatewayInRequestScope(r, gwSn) {
		return nil, false
	}
	engine, ok := p.PlatformEngine.(*platformEngineImpl)
	if !ok || engine == nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": "platform engine not available"})
		return nil, false
	}
	return engine, true
}

func (p *CascadePlugin) ensureGatewayInRequestScope(r *ghttp.Request, gwSn string) bool {
	device, err := store.GetDevice(gwSn)
	if err != nil || device == nil {
		r.Response.WriteJson(map[string]interface{}{"code": 404, "message": "gateway not found"})
		return false
	}
	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 && device.TenantID != tenantID {
		r.Response.WriteJson(map[string]interface{}{"code": 403, "message": "gateway is outside current tenant"})
		return false
	}
	projectID := r.GetCtxVar("project_id").Uint()
	if projectID > 0 && device.ProjectID != projectID {
		r.Response.WriteJson(map[string]interface{}{"code": 403, "message": "gateway is outside current project"})
		return false
	}
	return true
}

func decodeRemotePluginSummary(data interface{}) (*remotePluginSummary, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var summary remotePluginSummary
	if err := json.Unmarshal(payload, &summary); err != nil {
		return nil, err
	}
	return &summary, nil
}

func decodeRemotePluginSummaries(data interface{}) ([]remotePluginSummary, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var summaries []remotePluginSummary
	if err := json.Unmarshal(payload, &summaries); err != nil {
		return nil, err
	}
	return summaries, nil
}

func extractRemoteBaseVersion(config map[string]interface{}) int64 {
	var baseVersion int64
	if raw, ok := config["base_version"]; ok {
		switch v := raw.(type) {
		case float64:
			baseVersion = int64(v)
		case int64:
			baseVersion = v
		case int:
			baseVersion = int64(v)
		case json.Number:
			baseVersion, _ = v.Int64()
		}
		delete(config, "base_version")
	}
	return baseVersion
}

func desiredEnabledForConfig(gwSn, pluginName string, config map[string]interface{}) bool {
	if raw, ok := config["enabled"]; ok {
		if enabled, ok := raw.(bool); ok {
			return enabled
		}
	}
	if item, ok := gatewayPluginStateCache.Get(gwSn, pluginName); ok {
		if item.SummarySnapshot != nil {
			return item.SummarySnapshot.Status == "running"
		}
		return item.DesiredEnabled
	}
	return false
}

func summaryFromCacheItem(item *remotePluginCacheItem) remotePluginSummary {
	if item == nil {
		return remotePluginSummary{SyncState: remotePluginSyncPending, IsOfflineEditable: true}
	}
	if item.SummarySnapshot != nil {
		summary := *cloneRemotePluginSummary(item.SummarySnapshot)
		summary.SyncState = valueOrDefault(item.SyncState, remotePluginSyncPending)
		summary.BaseVersion = item.BaseVersion
		summary.GatewayVersion = item.GatewayVersion
		summary.EnabledAt = item.EnabledAt
		summary.LastSyncedAt = item.LastSyncedAt
		summary.IsOfflineEditable = true
		applyConfigToSchema(&summary, item.DesiredConfig)
		return summary
	}
	status := "stopped"
	if item.DesiredEnabled {
		status = "running"
	}
	return remotePluginSummary{
		Name:              item.PluginName,
		Status:            status,
		ConfigVersion:     item.BaseVersion,
		UpdatedAt:         item.UpdatedAt,
		UpdatedBy:         "platform",
		SyncState:         valueOrDefault(item.SyncState, remotePluginSyncPending),
		BaseVersion:       item.BaseVersion,
		GatewayVersion:    item.GatewayVersion,
		EnabledAt:         item.EnabledAt,
		LastSyncedAt:      item.LastSyncedAt,
		IsOfflineEditable: true,
	}
}

func (p *CascadePlugin) getRouteParam(r *ghttp.Request, name string) string {
	if v := r.Get(name).String(); v != "" {
		return v
	}
	switch name {
	case "gwSn":
		if v := r.Get("gw_sn").String(); v != "" {
			return v
		}
		return r.Get("gw").String()
	case "pluginName":
		if v := r.Get("plugin_name").String(); v != "" {
			return v
		}
		return r.Get("name").String()
	default:
		return ""
	}
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
