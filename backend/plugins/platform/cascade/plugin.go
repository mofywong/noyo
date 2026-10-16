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
	"strings"
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

	ctx      platform.Context
	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
}

// getActivePlugin dynamically returns the latest active CascadePlugin instance
// registered in PluginManager, preventing stale pointer references in GoFrame HTTP route closures after hot-reload.
func (p *CascadePlugin) getActivePlugin() *CascadePlugin {
	if p == nil || p.ctx == nil {
		return p
	}
	server, ok := p.ctx.GetCoreServer().(*core.Server)
	if !ok || server == nil || server.Manager == nil {
		return p
	}
	if active, ok := server.Manager.GetPlugin("cascade").(*CascadePlugin); ok && active != nil {
		return active
	}
	return p
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
enable_tls: false
insecure_skip_verify: false
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

func (p *CascadePlugin) gatewayCommandDeviceCode(device types.DeviceMeta) string {
	if p.ctx == nil {
		return device.DeviceCode
	}
	if coreServer, ok := p.ctx.GetCoreServer().(*core.Server); ok && coreServer.Manager != nil {
		return coreServer.Manager.GatewayCommandDeviceCode(device)
	}
	return device.DeviceCode
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
		"deviceCode":  p.gatewayCommandDeviceCode(device),
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
	return dev.ProductCode == defaultGatewayProductCode
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

// PublishMediaSignal implements platform.IMediaSignalRouter.  It transfers
// only ICE signalling; video continues to flow directly between browser and
// gateway (or through TURN when direct connectivity is impossible).
func (p *CascadePlugin) PublishMediaSignal(gatewayCode string, signal platform.MediaSignal) error {
	if p.Config.Mode == "platform" {
		if p.PlatformEngine == nil {
			return fmt.Errorf("platform engine not available")
		}
		return p.PlatformEngine.PublishMediaSignal(gatewayCode, signal)
	}
	if p.Config.Mode == "gateway" {
		if p.GatewayEngine == nil {
			return fmt.Errorf("gateway engine not available")
		}
		return p.GatewayEngine.PublishMediaSignal(signal)
	}
	return fmt.Errorf("cascade mode is not configured")
}

func (p *CascadePlugin) SetMediaSignalHandler(handler func(platform.MediaSignal)) {
	if p.Config.Mode == "platform" && p.PlatformEngine != nil {
		p.PlatformEngine.SetMediaSignalHandler(handler)
	}
	if p.Config.Mode == "gateway" && p.GatewayEngine != nil {
		p.GatewayEngine.SetMediaSignalHandler(handler)
	}
}

// IsGatewayMediaNode lets protocol plugins register a downlink handler only
// in gateway mode. Platform mode reserves its handler for browser sessions.
func (p *CascadePlugin) IsGatewayMediaNode() bool {
	return p.Config.Mode == "gateway"
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
				Name:        "enable_tls",
				Type:        "switch",
				Title:       map[string]string{"en": "Enable TLS", "zh": "是否启用TLS"},
				Description: map[string]string{"en": "Enable TLS encryption for the MQTT connection.", "zh": "启用 MQTT TLS 加密连接。"},
				Value:       p.Config.EnableTLS,
			},
			{
				Name:        "insecure_skip_verify",
				Type:        "switch",
				Title:       map[string]string{"en": "Skip Certificate Verification", "zh": "是否跳过证书验证"},
				Description: map[string]string{"en": "Skip server certificate verification when TLS is enabled.", "zh": "启用后不验证服务端证书。"},
				Value:       p.Config.InsecureSkipVerify,
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
			Name:        "enable_tls",
			Type:        "switch",
			Title:       map[string]string{"en": "Enable TLS", "zh": "是否启用TLS"},
			Description: map[string]string{"en": "Enable TLS encryption for the cascade MQTT connection.", "zh": "启用级联 MQTT TLS 加密连接。"},
		},
		{
			Name:        "insecure_skip_verify",
			Type:        "switch",
			Title:       map[string]string{"en": "Skip Certificate Verification", "zh": "是否跳过证书验证"},
			Description: map[string]string{"en": "Skip server certificate verification when TLS is enabled.", "zh": "启用后不验证服务端证书。"},
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
				Name:        "tenant_id",
				Type:        "number",
				Title:       map[string]string{"en": "Platform Tenant ID", "zh": "平台租户 ID"},
				Description: map[string]string{"en": "Copy the target tenant ID from the platform gateway registration information.", "zh": "请填写平台网关预登记信息中的目标租户 ID。"},
				Required:    true,
			},
			core.PluginSetupField{
				Name:        "project_id",
				Type:        "number",
				Title:       map[string]string{"en": "Platform Project ID", "zh": "平台项目 ID"},
				Description: map[string]string{"en": "Copy the target project ID from the platform gateway registration information.", "zh": "请填写平台网关预登记信息中的目标项目 ID。"},
				Required:    true,
			},
			core.PluginSetupField{
				Name:  "gateway_sn",
				Type:  "string",
				Title: map[string]string{"en": "Gateway Physical SN", "zh": "网关物理 SN"},
				Description: map[string]string{
					"en": "Must match the SN that was pre-registered in the specified platform tenant and project.",
					"zh": "必须与指定平台租户和项目下预登记的网关物理 SN 一致。",
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
	p.stopCh = make(chan struct{})

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
	bind := func(path string, handler func(*ghttp.Request)) {
		_ = ctx.RegisterHTTPHandler(path, p.authorizeHTTP(path, handler))
	}
	bind("/api/extension/cascade/status", p.handleGetStatus)
	bind("/api/extension/cascade/maintenance", p.handleLocalMaintenance)
	bind("/api/extension/cascade/gateways/:gwSn/maintenance", p.handleGatewayMaintenance)
	bind("/api/extension/cascade/gateways/:gwSn/runtime-results", p.handleGatewayRuntimeResults)
	bind("/api/extension/cascade/configuration/resources", p.handleConfigurationStates)
	bind("/api/extension/cascade/stream", p.handleStream)
	bind("/api/extension/cascade/sync/conflicts", p.handleSyncConflicts)
	bind("/api/extension/cascade/sync/conflicts/resolve", p.handleSyncConflictResolve)
	bind("/api/extension/cascade/gateways", p.handleGatewayList)
	bind("/api/extension/cascade/gateways/:gwSn/plugins", p.handleGatewayPlugins)
	bind("/api/extension/cascade/gateways/:gwSn/plugins/:pluginName", p.handleGatewayPluginConfig)
	bind("/api/extension/cascade/gateways/:gwSn/plugins/:pluginName/config", p.handleGatewayPluginConfigSave)
	bind("/api/extension/cascade/gateways/:gwSn/plugins/:pluginName/status", p.handleGatewayPluginStatusSave)
	bind("/api/extension/cascade/gateways/:gwSn/system/config", p.handleGatewaySystemConfig)
	bind("/api/extension/cascade/gateways/:gwSn/license/status", p.handleGatewayLicenseStatus)
	bind("/api/extension/cascade/gateways/:gwSn/license/upload", p.handleGatewayLicenseUpload)
	bind("/api/extension/cascade/gateways/:gwSn/system/log/files", p.handleGatewayLogFiles)
	bind("/api/extension/cascade/gateways/:gwSn/system/log/file", p.handleGatewayLogFile)
	bind("/api/extension/cascade/gateways/:gwSn/system/log/tail", p.handleGatewayLogTail)

	return nil
}

func (p *CascadePlugin) handleGetStatus(r *ghttp.Request) {
	active := p.getActivePlugin()
	connected := false
	if active.Config.Mode == "platform" && active.PlatformEngine != nil {
		engine, ok := active.PlatformEngine.(*platformEngineImpl)
		if ok && engine.client != nil {
			connected = engine.client.IsConnected()
		}
	} else if active.Config.Mode == "gateway" && active.GatewayEngine != nil {
		engine, ok := active.GatewayEngine.(*gatewayEngineImpl)
		if ok && engine.client != nil {
			connected = engine.client.IsConnected()
		}
	}

	data := p.statusPayload(r, connected, active.Config.Mode, active.Config.MqttUrl, active.Config.GatewaySn)
	data["ts"] = time.Now()
	r.Response.WriteJson(data)
}

func (p *CascadePlugin) handleStream(r *ghttp.Request) {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	r.Response.Header().Set("X-Accel-Buffering", "no")

	r.Response.Write("event: connected\ndata: {}\n\n")
	r.Response.Flush()

	checkStatus := func() (connected bool, mode, broker, gwCode string) {
		active := p.getActivePlugin()
		if active.Config.Mode == "platform" && active.PlatformEngine != nil {
			if engine, ok := active.PlatformEngine.(*platformEngineImpl); ok && engine.client != nil {
				connected = engine.client.IsConnected()
			}
		} else if active.Config.Mode == "gateway" && active.GatewayEngine != nil {
			if engine, ok := active.GatewayEngine.(*gatewayEngineImpl); ok && engine.client != nil {
				connected = engine.client.IsConnected()
			}
		}
		return connected, active.Config.Mode, active.Config.MqttUrl, active.Config.GatewaySn
	}

	sendEventStatus := func(connected bool, mode, broker, gwCode string) error {
		data := p.statusPayload(r, connected, mode, broker, gwCode)
		data["ts"] = time.Now()
		b, _ := json.Marshal(data)
		msg := fmt.Sprintf("event: status\ndata: %s\n\n", string(b))
		if _, err := r.Response.Writer.Write([]byte(msg)); err != nil {
			return err
		}
		r.Response.Flush()
		return nil
	}

	// Send initial status immediately upon connection
	lastConnected, lastMode, lastBroker, lastGwCode := checkStatus()
	_ = sendEventStatus(lastConnected, lastMode, lastBroker, lastGwCode)

	ctx := r.Context()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopCh:
			// Old instance stopped during reload; end this SSE stream so the client reconnects to the new active instance
			return
		case <-ticker.C:
			connected, mode, broker, gwCode := checkStatus()

			if connected != lastConnected || mode != lastMode || broker != lastBroker || gwCode != lastGwCode {
				if err := sendEventStatus(connected, mode, broker, gwCode); err != nil {
					return
				}
				lastConnected = connected
				lastMode = mode
				lastBroker = broker
				lastGwCode = gwCode
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
	active := p.getActivePlugin()
	if r.Method == "POST" {
		active.handleGatewayCreate(r)
		return
	}
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	if active.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway management is only available in platform mode"})
		return
	}

	coreServer, _ := active.ctx.GetCoreServer().(*core.Server)
	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()
	principal := core.RequestAuthContext(r)
	if principal == nil {
		writeCascadeAccessDenied(r)
		return
	}
	if !principal.IsSystemAdmin {
		tenantID = principal.TenantID
	}
	filter := store.DeviceQueryFilter{TenantID: tenantID, ProjectID: projectID}
	if projects, restricted := principal.ProjectIDsForTenantQuery(); restricted {
		if len(projects) == 0 {
			r.Response.WriteJson(map[string]interface{}{"code": 0, "data": []interface{}{}})
			return
		}
		filter.AllowedProjectIDs = projects
	}
	devices, _, err := store.ListDevicesWithFilter(0, 0, filter)
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
		SN              string                    `json:"sn"`
		SerialNumber    string                    `json:"serialNumber"`
		Name            string                    `json:"name"`
		ProjectID       uint                      `json:"projectId"`
		ProjectName     string                    `json:"projectName"`
		ProductCode     string                    `json:"productCode"`
		Enabled         bool                      `json:"enabled"`
		Status          string                    `json:"status"`
		Online          bool                      `json:"online"`
		OnlineAt        int64                     `json:"onlineAt"`
		UpdatedAt       int64                     `json:"updatedAt"`
		RuntimeFailures int64                     `json:"runtimeFailures"`
		Maintenance     *store.GatewayMaintenance `json:"maintenance"`
	}
	var runtimeCounts []struct {
		GatewayCode string
		Count       int64
	}
	if err := currentGatewayRuntimeFailures(store.DB).
		Select("gateway_code, COUNT(*) AS count").
		Group("gateway_code").Scan(&runtimeCounts).Error; err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	runtimeFailures := make(map[string]int64, len(runtimeCounts))
	for _, item := range runtimeCounts {
		runtimeFailures[item.GatewayCode] = item.Count
	}
	var maintenanceRows []store.GatewayMaintenance
	if err := store.DB.Find(&maintenanceRows).Error; err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	maintenanceStates := make(map[string]*store.GatewayMaintenance, len(maintenanceRows))
	for i := range maintenanceRows {
		maintenanceStates[maintenanceRows[i].GatewayCode] = &maintenanceRows[i]
	}

	items := make([]gatewayItem, 0)
	for _, dev := range devices {
		if dev.ProductCode != defaultGatewayProductCode || !cascadeResourceAllowed(r, dev.TenantID, dev.ProjectID, "gateway:list") {
			continue
		}
		maintenance := maintenanceStates[dev.Code]
		if maintenance == nil {
			maintenance = &store.GatewayMaintenance{GatewayCode: dev.Code, Mode: store.GatewayModeNormal}
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
			SN:              dev.Code,
			SerialNumber:    gatewaySerialNumber(&dev),
			Name:            dev.Name,
			ProjectID:       dev.ProjectID,
			ProjectName:     projectNames[dev.ProjectID],
			ProductCode:     dev.ProductCode,
			RuntimeFailures: runtimeFailures[dev.Code],
			Maintenance:     maintenance,
			Enabled:         dev.Enabled,
			Status:          statusStr,
			Online:          online,
			OnlineAt:        onlineAt,
			UpdatedAt:       dev.UpdatedAt.UnixMilli(),
		})
	}

	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": items})
}

func (p *CascadePlugin) handleGatewayCreate(r *ghttp.Request) {
	if p.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway management is only available in platform mode"})
		return
	}
	authCtx := core.RequestAuthContext(r)
	if authCtx == nil || !authCtx.HasPermission("device:create") {
		r.Response.WriteJson(map[string]interface{}{"code": 403, "message": "Permission denied: device:create"})
		return
	}
	tenantID, projectID, err := core.CurrentTenantProjectScope(r)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": err.Error()})
		return
	}
	var body struct {
		SN   string `json:"sn"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(r.GetBody(), &body); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	serialNumber := strings.TrimSpace(body.SN)
	gatewayCode, err := core.BuildCascadeGatewayCode(tenantID, projectID, serialNumber)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": err.Error()})
		return
	}
	if existing, err := store.GetDevice(gatewayCode); err == nil && existing != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 409, "message": "Gateway SN is already registered for this tenant and project"})
		return
	}
	device := newPendingGatewayDevice(gatewayCode, valueOrDefaultGatewayName(body.Name, serialNumber), tenantID, projectID)
	device.Enabled = true
	config, _ := json.Marshal(map[string]string{"gateway_sn": serialNumber})
	device.Config = string(config)
	if err := store.SaveDevice(device); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	if coreServer, ok := p.ctx.GetCoreServer().(*core.Server); ok && coreServer.DeviceManager != nil {
		coreServer.DeviceManager.Registry.UpdateDevice(device)
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": map[string]interface{}{
		"gatewayCode":  gatewayCode,
		"serialNumber": serialNumber,
		"tenantId":     tenantID,
		"projectId":    projectID,
	}})
}

func gatewaySerialNumber(device *store.Device) string {
	if device == nil {
		return ""
	}
	var config struct {
		GatewaySN string `json:"gateway_sn"`
	}
	if err := json.Unmarshal([]byte(device.Config), &config); err == nil && strings.TrimSpace(config.GatewaySN) != "" {
		return strings.TrimSpace(config.GatewaySN)
	}
	return device.Code
}

func valueOrDefaultGatewayName(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}

// activeSyncContext exposes the synchronization identity and push hook of the
// running engine, so HTTP handlers do not have to branch on the mode themselves.
func (p *CascadePlugin) activeSyncContext() (nodeID, origin string, notify func(store.SyncChange)) {
	active := p.getActivePlugin()
	switch active.Config.Mode {
	case "platform":
		engine, ok := active.PlatformEngine.(*platformEngineImpl)
		if !ok || engine == nil {
			return "", "", nil
		}
		return cloudSyncNodeID, cloudSyncNodeID, func(ch store.SyncChange) {
			if engine.syncHub != nil {
				engine.syncHub.notifyChange(ch)
			}
		}
	case "gateway":
		engine, ok := active.GatewayEngine.(*gatewayEngineImpl)
		if !ok || engine == nil {
			return "", "", nil
		}
		return engine.gatewayCode, gatewayOrigin, func(ch store.SyncChange) {
			if engine.syncHub != nil {
				engine.syncHub.notifyChange(ch)
			}
		}
	}
	return "", "", nil
}

// handleSyncConflicts lists the entities awaiting manual arbitration. The
// synchronization layer never overwrites a divergent version on its own, so
// these records are the operator's queue rather than an error state.
func (p *CascadePlugin) handleSyncConflicts(r *ghttp.Request) {
	if r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	active := p.getActivePlugin()
	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()
	principal := core.RequestAuthContext(r)
	if principal == nil {
		writeCascadeAccessDenied(r)
		return
	}
	if !principal.IsSystemAdmin {
		tenantID = principal.TenantID
	}
	items, err := collectSyncConflicts(tenantID, projectID)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	visible := make([]syncConflictView, 0, len(items))
	for _, item := range items {
		if syncConflictVisible(r, item) {
			visible = append(visible, item)
		}
	}
	items = visible
	nodeID, _, _ := active.activeSyncContext()
	data := map[string]interface{}{
		"node_id": nodeID,
		"mode":    active.Config.Mode,
		"total":   len(items),
		"items":   items,
	}
	if active.Config.Mode == "gateway" {
		if engine, ok := active.GatewayEngine.(*gatewayEngineImpl); ok && engine != nil {
			data["sync"] = engine.syncDiagnostics()
		}
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
}

// handleSyncConflictResolve records an operator's decision and pushes it to the
// peer. A single decision may be posted directly, or many at once through
// "items" when clearing the queue in bulk.
func (p *CascadePlugin) handleSyncConflictResolve(r *ghttp.Request) {
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	active := p.getActivePlugin()

	var body struct {
		syncResolveRequestItem
		Items []syncResolveRequestItem `json:"items"`
	}
	if err := json.Unmarshal(r.GetBody(), &body); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	requests := body.Items
	if body.EntityCode != "" {
		requests = append(requests, body.syncResolveRequestItem)
	}
	if len(requests) == 0 {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "no resolutions supplied"})
		return
	}
	for _, item := range requests {
		if !syncResolutionAllowed(r, item) {
			writeCascadeAccessDenied(r)
			return
		}
	}

	nodeID, origin, notify := active.activeSyncContext()
	if nodeID == "" {
		r.Response.WriteJson(map[string]interface{}{"code": 503, "message": "cascade engine is not running"})
		return
	}

	results := make([]map[string]interface{}, 0, len(requests))
	succeeded, failed := 0, 0
	for _, item := range requests {
		entry := map[string]interface{}{
			"entity_type": item.EntityType,
			"entity_code": item.EntityCode,
			"resolution":  item.Resolution,
		}
		change, err := resolveEntityConflict(item, nodeID, origin, active.Logger)
		if err != nil {
			entry["ok"] = false
			entry["message"] = err.Error()
			failed++
		} else {
			entry["ok"] = true
			succeeded++
			if change != nil && notify != nil {
				recorded := *change
				go notify(recorded)
			}
		}
		results = append(results, entry)
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": map[string]interface{}{
		"succeeded": succeeded,
		"failed":    failed,
		"results":   results,
	}})
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
			cached := summaryFromCacheItem(item)
			r.Response.WriteJson(map[string]interface{}{"code": 0, "data": cached, "message": err.Error()})
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
		// The reconciler compares fields against the saved base, handles a lost
		// response, and serializes retries. A plain version mismatch is not by
		// itself a conflict.
		engine.reconcilePendingPluginConfig(gwSn)
		if latest, ok := gatewayPluginStateCache.Get(gwSn, pluginName); ok && latest.SyncState != remotePluginSyncPending {
			r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(latest)})
			return
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
	var fields map[string]interface{}
	if err := json.Unmarshal(r.GetBody(), &fields); err != nil || fields == nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	baseVersion := extractRemoteBaseVersion(fields)
	if _, ok := p.remotePluginEngine(r, gwSn); !ok {
		return
	}
	item, err := gatewayPluginStateCache.SaveDesired(gwSn, pluginName, fields, desiredEnabledForConfig(gwSn, pluginName, fields), baseVersion, time.Now())
	if err != nil {
		r.Response.WriteStatus(503, map[string]interface{}{"code": 503, "message": err.Error()})
		return
	}
	store.NotifyProtectedConfigCommitted(store.ProtectedConfigCode(gwSn, "plugin", pluginName))
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item)})
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
	if _, ok := p.remotePluginEngine(r, gwSn); !ok {
		return
	}
	item, err := gatewayPluginStateCache.SaveDesired(gwSn, pluginName, map[string]interface{}{"enabled": body.Enabled}, body.Enabled, body.BaseVersion, time.Now())
	if err != nil {
		r.Response.WriteStatus(503, map[string]interface{}{"code": 503, "message": err.Error()})
		return
	}
	store.NotifyProtectedConfigCommitted(store.ProtectedConfigCode(gwSn, "plugin", pluginName))
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": summaryFromCacheItem(item)})
}

func (p *CascadePlugin) writeRemotePluginCommandResponse(r *ghttp.Request, method, pluginName string, params map[string]interface{}) {
	active := p.getActivePlugin()
	if active.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway plugin management is only available in platform mode"})
		return
	}
	gwSn := p.getRouteParam(r, "gwSn")
	if gwSn == "" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway sn is required"})
		return
	}
	engine, ok := active.PlatformEngine.(*platformEngineImpl)
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
	active := p.getActivePlugin()
	if active.Config.Mode != "platform" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway plugin management is only available in platform mode"})
		return nil, false
	}
	if gwSn == "" {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "gateway sn is required"})
		return nil, false
	}
	if !active.ensureGatewayInRequestScope(r, gwSn) {
		return nil, false
	}
	engine, ok := active.PlatformEngine.(*platformEngineImpl)
	if !ok || engine == nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": "platform engine not available"})
		return nil, false
	}
	return engine, true
}

func (p *CascadePlugin) ensureGatewayInRequestScope(r *ghttp.Request, gwSn string, permissions ...string) bool {
	device, err := store.GetDevice(gwSn)
	if err != nil || device == nil || device.ProductCode != defaultGatewayProductCode {
		r.Response.WriteJson(map[string]interface{}{"code": 404, "message": "gateway not found"})
		return false
	}
	permission := ""
	if len(permissions) > 0 {
		permission = permissions[0]
	}
	if !cascadeResourceAllowed(r, device.TenantID, device.ProjectID, permission) {
		writeCascadeAccessDenied(r)
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

	p.stopOnce.Do(func() {
		if p.stopCh != nil {
			close(p.stopCh)
		}
	})

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
