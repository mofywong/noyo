package cascade

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"noyo/core"
	"noyo/core/platform"
	"noyo/core/store"
	"noyo/core/types"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type platformEngineImpl struct {
	ctx         platform.Context
	logger      *zap.Logger
	config      *Config
	client      mqtt.Client
	pendingCmds sync.Map
	cancel      context.CancelFunc
	syncMu      sync.Mutex // 防止并发 sync 操作
}

func NewPlatformEngine(ctx platform.Context, logger *zap.Logger, cfg *Config) PlatformEngine {
	return &platformEngineImpl{
		ctx:    ctx,
		logger: logger,
		config: cfg,
	}
}

func (e *platformEngineImpl) Start() error {
	e.logger.Info("Platform Engine Started", zap.String("mqtt_url", e.config.MqttUrl))

	opts := mqtt.NewClientOptions().AddBroker(e.config.MqttUrl)
	// 使用带唯一后缀的 ClientID，避免插件重载或多实例时 broker 因同 ClientID 踢掉旧连接触发 LWT
	clientID := fmt.Sprintf("noyo-platform-cascade-%s", uuid.New().String()[:8])
	opts.SetClientID(clientID)
	e.logger.Info("Platform MQTT ClientID", zap.String("client_id", clientID))
	opts.SetUsername(e.config.Username)
	opts.SetPassword(e.config.Password)
	opts.SetAutoReconnect(true)
	opts.SetMaxReconnectInterval(1 * time.Second)
	opts.SetKeepAlive(60 * time.Second)   // 增加到 60s，避免系统负载高时 PING 超时触发 LWT
	opts.SetPingTimeout(20 * time.Second) // 增加 PING 超时容忍度
	opts.SetOrderMatters(false)           // 允许消息回调并行处理，避免慢回调（如 sync）阻塞 keepalive

	// Set Last Will and Testament (LWT) for Platform offline status
	// 使用固定时间戳 0，便于区分 LWT 触发的 offline 和主动发布的 offline
	willPayload := `{"status":"offline","timestamp":0,"type":"will"}`
	opts.SetWill("noyo/cascade/platform/status", willPayload, 1, true)

	opts.OnConnect = func(c mqtt.Client) {
		e.logger.Info("Platform Engine Connected to MQTT Broker", zap.String("client_id", clientID))

		// 先订阅主题，确保不会遗漏网关的状态上报
		e.subscribeTopics(c)

		// 发布平台 Online 状态（retained=true），通知所有网关平台已上线
		// 网关收到后会上报一次全量设备状态
		onlinePayload := fmt.Sprintf(`{"status":"online","timestamp":%d}`, time.Now().UnixMilli())
		c.Publish("noyo/cascade/platform/status", 1, true, []byte(onlinePayload))
	}
	opts.OnConnectionLost = func(c mqtt.Client, err error) {
		e.logger.Error("Platform Engine MQTT Connection Lost! LWT will be triggered by broker.",
			zap.String("client_id", clientID),
			zap.Error(err))
	}

	e.client = mqtt.NewClient(opts)
	if token := e.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("platform engine failed to connect to mqtt broker: %w", token.Error())
	}

	// Auto-create gateway product if not exists
	e.ensureGatewayProduct()

	// Reset all cascade devices to offline on startup
	e.resetAllCascadeDevicesOffline()

	// Register DB Hooks for config changes
	e.registerDBHooks()

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go e.statusLoop(ctx)

	return nil
}

func (e *platformEngineImpl) statusLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if e.client != nil && e.client.IsConnected() {
				onlinePayload := fmt.Sprintf(`{"status":"online","timestamp":%d}`, time.Now().UnixMilli())
				e.client.Publish("noyo/cascade/platform/status", 1, true, []byte(onlinePayload))
			}
		}
	}
}

func (e *platformEngineImpl) ensureGatewayProduct() {
	const gwProductCode = "noyo-gw"
	expectedName := "Noyo边缘网关"
	expectedConfig := `{"tsl":{"properties":[{"identifier":"sys_cpu","name":"系统CPU","dataType":{"type":"double","specs":{"unit":"%"}},"accessMode":"r"},{"identifier":"sys_mem_percent","name":"系统内存","dataType":{"type":"double","specs":{"unit":"%"}},"accessMode":"r"},{"identifier":"sys_mem_total","name":"系统总内存","dataType":{"type":"double","specs":{"unit":"MB"}},"accessMode":"r"},{"identifier":"sys_mem_used","name":"系统已用内存","dataType":{"type":"double","specs":{"unit":"MB"}},"accessMode":"r"},{"identifier":"sys_disk_percent","name":"系统磁盘","dataType":{"type":"double","specs":{"unit":"%"}},"accessMode":"r"},{"identifier":"sys_disk_total","name":"系统总磁盘","dataType":{"type":"double","specs":{"unit":"GB"}},"accessMode":"r"},{"identifier":"sys_disk_used","name":"系统已用磁盘","dataType":{"type":"double","specs":{"unit":"GB"}},"accessMode":"r"},{"identifier":"svc_cpu","name":"服务CPU","dataType":{"type":"double","specs":{"unit":"%"}},"accessMode":"r"},{"identifier":"svc_mem","name":"服务内存","dataType":{"type":"double","specs":{"unit":"MB"}},"accessMode":"r"},{"identifier":"sys_uptime","name":"运行时间","dataType":{"type":"int","specs":{"unit":"s"}},"accessMode":"r"},{"identifier":"sys_ip","name":"IP地址","dataType":{"type":"text","specs":{"length":"64"}},"accessMode":"r"},{"identifier":"sys_os","name":"操作系统","dataType":{"type":"text","specs":{"length":"64"}},"accessMode":"r"},{"identifier":"sys_arch","name":"系统架构","dataType":{"type":"text","specs":{"length":"64"}},"accessMode":"r"},{"identifier":"gw_version","name":"网关版本","dataType":{"type":"text","specs":{"length":"64"}},"accessMode":"r"},{"identifier":"gw_go_version","name":"Go版本","dataType":{"type":"text","specs":{"length":"64"}},"accessMode":"r"},{"identifier":"gw_goroutine","name":"协程数","dataType":{"type":"int","specs":{"unit":"个"}},"accessMode":"r"}]}}`

	p, err := store.GetProduct(gwProductCode)
	if err == nil && p != nil {
		// Update existing if name, protocol, or config is outdated
		if p.Name != expectedName || p.ProtocolName != "cascade" || p.Config != expectedConfig {
			p.Name = expectedName
			p.ProtocolName = "cascade" // 设置为 cascade 协议，避免直连设备无协议的报错
			p.Config = expectedConfig
			if err := store.SaveProduct(p); err != nil {
				e.logger.Error("Failed to update default gateway product", zap.Error(err))
			} else {
				e.logger.Info("Updated default gateway product", zap.String("code", gwProductCode))
			}
		}
		return // already exists
	}

	e.logger.Info("Creating default gateway product", zap.String("code", gwProductCode))
	newProd := &store.Product{
		Code:         gwProductCode,
		Name:         expectedName,
		ProtocolName: "cascade", // 不需要额外的子设备配置参数，但需要满足直连设备有协议的要求
		Config:       expectedConfig,
	}
	if err := store.SaveProduct(newProd); err != nil {
		e.logger.Error("Failed to create default gateway product", zap.Error(err))
	}
}

func (e *platformEngineImpl) Stop() error {
	e.logger.Info("Platform Engine Stopped")

	if e.cancel != nil {
		e.cancel()
	}

	if e.client != nil && e.client.IsConnected() {
		// Explicitly publish Offline status before graceful disconnect
		offlinePayload := fmt.Sprintf(`{"status":"offline","timestamp":%d}`, time.Now().UnixMilli())
		token := e.client.Publish("noyo/cascade/platform/status", 1, true, []byte(offlinePayload))
		token.WaitTimeout(2 * time.Second)

		e.client.Disconnect(250)
	}
	return nil
}

func (e *platformEngineImpl) registerDBHooks() {
	store.DB.Callback().Update().After("gorm:update").Register("cascade:after_update", e.onDBChange)
	store.DB.Callback().Create().After("gorm:create").Register("cascade:after_create", e.onDBChange)
	store.DB.Callback().Delete().After("gorm:delete").Register("cascade:after_delete", e.onDBChange)
}

func (e *platformEngineImpl) onDBChange(db *gorm.DB) {
	if db.Statement.Schema == nil {
		return
	}
	tableName := db.Statement.Schema.Table
	if tableName != "products" && tableName != "devices" {
		return
	}

	var codes []string

	// Try to extract from Dest
	if dest := db.Statement.Dest; dest != nil {
		codes = append(codes, e.extractCodes(dest)...)
	}

	// Try to extract from Model
	if model := db.Statement.Model; model != nil {
		codes = append(codes, e.extractCodes(model)...)
	}

	affectedGwSns := make(map[string]bool)

	if len(codes) == 0 {
		e.logger.Warn("Could not extract codes from DB statement", zap.String("table", tableName))
		return
	}

	// deduplicate codes
	codeMap := make(map[string]bool)
	for _, c := range codes {
		codeMap[c] = true
	}

	// Create a new session to avoid polluting the current statement context
	queryDB := db.Session(&gorm.Session{NewDB: true})

	if tableName == "devices" {
		// 优先从 Statement.Dest/Model 中直接提取 ParentCode 信息，
		// 避免在 GORM after_create 回调中因事务未提交而重新查询数据库失败的问题。
		parentCodeMap := make(map[string]string) // code → parentCode
		if dest := db.Statement.Dest; dest != nil {
			for k, v := range e.extractDeviceParentCodes(dest) {
				parentCodeMap[k] = v
			}
		}
		if model := db.Statement.Model; model != nil {
			for k, v := range e.extractDeviceParentCodes(model) {
				if _, exists := parentCodeMap[k]; !exists {
					parentCodeMap[k] = v
				}
			}
		}

		for code := range codeMap {
			if parentCode, ok := parentCodeMap[code]; ok && parentCode != "" {
				// 子设备变更：通过 Dest 中的 ParentCode 向上追溯，因为父节点肯定已经在数据库中了
				gwSn := e.findTopLevelGateway(queryDB, parentCode)
				if gwSn != "" {
					affectedGwSns[gwSn] = true
				} else {
					// 如果找不到顶层（可能父节点也在同一个未提交的事务中），退而求其次.
					affectedGwSns[parentCode] = true
				}
			} else {
				// 可能是网关自身或删除操作等，回退到数据库查询
				e.findAffectedGwByDevice(queryDB, code, affectedGwSns)
			}
		}
	} else if tableName == "products" {
		for code := range codeMap {
			e.findAffectedGwByProduct(queryDB, code, affectedGwSns)
		}
	}

	for gwSn := range affectedGwSns {
		e.logger.Info("Detected DB change, broadcasting config_changed to gateway", zap.String("gw_sn", gwSn))
		payload := fmt.Sprintf(`{"timestamp":%d}`, time.Now().Unix())
		if e.client != nil && e.client.IsConnected() {
			topic := fmt.Sprintf("noyo/cascade/gw/%s/config_changed", gwSn)
			e.client.Publish(topic, 1, false, []byte(payload))
		}
	}
}

func (e *platformEngineImpl) extractCodes(dest interface{}) []string {
	var codes []string
	switch v := dest.(type) {
	case *store.Device:
		if v.Code != "" {
			codes = append(codes, v.Code)
		}
	case store.Device:
		if v.Code != "" {
			codes = append(codes, v.Code)
		}
	case []store.Device:
		for _, item := range v {
			if item.Code != "" {
				codes = append(codes, item.Code)
			}
		}
	case []*store.Device:
		for _, item := range v {
			if item.Code != "" {
				codes = append(codes, item.Code)
			}
		}
	case *store.Product:
		if v.Code != "" {
			codes = append(codes, v.Code)
		}
	case store.Product:
		if v.Code != "" {
			codes = append(codes, v.Code)
		}
	case []store.Product:
		for _, item := range v {
			if item.Code != "" {
				codes = append(codes, item.Code)
			}
		}
	case []*store.Product:
		for _, item := range v {
			if item.Code != "" {
				codes = append(codes, item.Code)
			}
		}
	}
	return codes
}

// extractDeviceParentCodes 从 GORM Statement.Dest 或 Model 中提取设备的 code → parentCode 映射。
// 这样在 after_create 回调中不需要重新查询数据库即可知道设备的 ParentCode，
// 避免了因 GORM 事务未提交导致查询结果为空的问题。
func (e *platformEngineImpl) extractDeviceParentCodes(dest interface{}) map[string]string {
	result := make(map[string]string)
	switch v := dest.(type) {
	case *store.Device:
		if v.Code != "" {
			result[v.Code] = v.ParentCode
		}
	case store.Device:
		if v.Code != "" {
			result[v.Code] = v.ParentCode
		}
	case []store.Device:
		for _, item := range v {
			if item.Code != "" {
				result[item.Code] = item.ParentCode
			}
		}
	case []*store.Device:
		for _, item := range v {
			if item.Code != "" {
				result[item.Code] = item.ParentCode
			}
		}
	}
	return result
}

func (e *platformEngineImpl) findTopLevelGateway(db *gorm.DB, deviceCode string) string {
	currentCode := deviceCode
	visited := make(map[string]bool)
	for currentCode != "" {
		if visited[currentCode] {
			return ""
		}
		visited[currentCode] = true

		var d store.Device
		if err := db.Unscoped().Where("code = ? OR code LIKE ?", currentCode, currentCode+"_del_%").First(&d).Error; err != nil {
			return ""
		}
		if d.ParentCode == "" {
			var p store.Product
			if err := db.Unscoped().Where("code = ?", d.ProductCode).First(&p).Error; err == nil {
				if p.ProtocolName == "cascade" {
					return d.Code
				}
			}
			return ""
		}
		currentCode = d.ParentCode
	}
	return ""
}

func (e *platformEngineImpl) findAffectedGwByDevice(db *gorm.DB, deviceCode string, affectedGwSns map[string]bool) {
	gwSn := e.findTopLevelGateway(db, deviceCode)
	if gwSn != "" {
		affectedGwSns[gwSn] = true
	}
}

func (e *platformEngineImpl) findAffectedGwByProduct(db *gorm.DB, productCode string, affectedGwSns map[string]bool) {
	var devices []store.Device
	if err := db.Unscoped().Where("product_code = ?", productCode).Find(&devices).Error; err == nil {
		for _, d := range devices {
			gwSn := e.findTopLevelGateway(db, d.Code)
			if gwSn != "" {
				affectedGwSns[gwSn] = true
			}
		}
	}
}

func (e *platformEngineImpl) subscribeTopics(c mqtt.Client) {
	c.Subscribe("noyo/cascade/gw/+/provision/request", 1, e.handleProvisionRequest)
	c.Subscribe("noyo/cascade/gw/+/register/request", 1, e.handleRegisterRequest)
	c.Subscribe("noyo/cascade/gw/+/sync/request", 1, e.handleSyncRequest)
	c.Subscribe("noyo/cascade/gw/+/telemetry/up", 1, e.handleTelemetryUp)
	c.Subscribe("noyo/cascade/gw/+/command/request_reply", 1, e.handleCommandReply)
	c.Subscribe("noyo/cascade/gw/+/status", 1, e.handleGatewayStatus)
}

func (e *platformEngineImpl) handleCommandReply(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	var reply map[string]interface{}
	if err := json.Unmarshal(msg.Payload(), &reply); err != nil {
		return
	}
	id, ok := reply["id"].(string)
	if !ok {
		return
	}
	if ch, ok := e.pendingCmds.Load(id); ok {
		ch.(chan map[string]interface{}) <- reply
	}
}

func (e *platformEngineImpl) SendCommand(gwSn string, cmdId string, payload []byte) (interface{}, error) {
	topic := fmt.Sprintf("noyo/cascade/gw/%s/command/request", gwSn)

	replyChan := make(chan map[string]interface{}, 1)
	e.pendingCmds.Store(cmdId, replyChan)
	defer e.pendingCmds.Delete(cmdId)

	if e.client == nil || !e.client.IsConnected() {
		return nil, fmt.Errorf("platform MQTT client not connected")
	}

	token := e.client.Publish(topic, 1, false, payload)
	token.Wait()
	if token.Error() != nil {
		return nil, token.Error()
	}

	select {
	case reply := <-replyChan:
		code, _ := reply["code"].(float64)
		if code == 200 {
			return reply["data"], nil
		}
		return nil, fmt.Errorf("command failed: %v", reply["message"])
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("command timeout")
	}
}

func parseTopicGwSn(topic, pattern string) (bool, string) {
	// Remove leading slash if present
	topic = strings.TrimPrefix(topic, "/")
	parts := strings.Split(topic, "/")
	// After TrimPrefix, parts[0] is "noyo", parts[1] is "cascade", parts[2] is "gw"
	if len(parts) >= 4 && parts[0] == "noyo" && parts[1] == "cascade" && parts[2] == "gw" {
		return true, parts[3]
	}
	return false, ""
}

func (e *platformEngineImpl) handleProvisionRequest(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	var req struct {
		GwSn        string `json:"gw_sn"`
		PreAuthCode string `json:"pre_auth_code"`
	}
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		e.logger.Error("Failed to parse provision request", zap.Error(err))
		return
	}

	e.logger.Info("Received provision request", zap.String("gw_sn", req.GwSn))

	clientID := fmt.Sprintf("gw-%s-%s", req.GwSn, uuid.New().String()[:8])
	dynamicUser := fmt.Sprintf("user-%s", req.GwSn)
	dynamicPwd := uuid.New().String()

	resp := struct {
		ClientID    string `json:"client_id"`
		DynamicUser string `json:"dynamic_user"`
		DynamicPwd  string `json:"dynamic_pwd"`
		Timestamp   int64  `json:"timestamp"`
	}{
		ClientID:    clientID,
		DynamicUser: dynamicUser,
		DynamicPwd:  dynamicPwd,
		Timestamp:   time.Now().Unix(),
	}

	respBytes, _ := json.Marshal(resp)

	respTopic := fmt.Sprintf("noyo/cascade/gw/%s/provision/response", req.GwSn)
	token := e.client.Publish(respTopic, 1, false, respBytes)
	token.Wait()
	if token.Error() != nil {
		e.logger.Error("Failed to send provision response", zap.Error(token.Error()))
	} else {
		e.logger.Info("Sent provision response", zap.String("gw_sn", req.GwSn), zap.String("client_id", clientID))
	}
}

func (e *platformEngineImpl) handleTelemetryUp(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		// Ignore retained telemetry messages to prevent processing stale events (e.g., from old broker state)
		return
	}

	var event types.Event
	if err := json.Unmarshal(msg.Payload(), &event); err != nil {
		e.logger.Error("Failed to parse telemetry event", zap.Error(err))
		return
	}

	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok {
		return
	}

	deviceCode := event.Topic // In our convention, Topic is the deviceCode

	switch event.Type {
	case types.EventDeviceStatusChanged:
		e.logger.Info("Received telemetry EventDeviceStatusChanged", zap.String("topic", deviceCode), zap.Any("payload", event.Payload))
		statusStr, ok := event.Payload.(string)
		if ok {
			status := core.DeviceStatus{
				Online:     statusStr == types.DeviceStatusOnline,
				LastActive: time.Now(),
				LastReport: time.Now(),
				LastStatus: statusStr,
			}
			coreServer.DeviceManager.ReportDeviceStatus(deviceCode, status)
			e.logger.Info("Reported device status", zap.String("device", deviceCode), zap.Bool("online", status.Online))
		} else {
			e.logger.Warn("Event payload is not a string", zap.Any("payload", event.Payload))
		}
	case types.EventPropertyReported:
		if props, ok := event.Payload.(map[string]interface{}); ok {
			e.logger.Info("Received telemetry EventPropertyReported",
				zap.String("device", deviceCode), zap.Any("properties", props))
			if err := e.ctx.ReportDeviceProperties(deviceCode, props); err != nil {
				e.logger.Warn("ReportDeviceProperties failed (non-fatal, data still cached)",
					zap.String("device", deviceCode), zap.Error(err))
			}
			coreServer.DeviceManager.UpdateLatestData(deviceCode, props)
		} else {
			e.logger.Warn("EventPropertyReported payload is not map[string]interface{}",
				zap.String("device", deviceCode), zap.Any("payload", event.Payload))
		}
	case types.EventEventReported:
		if data, ok := event.Payload.(map[string]interface{}); ok {
			eventId, _ := data["eventId"].(string)
			params, _ := data["params"].(map[string]interface{})
			if eventId != "" {
				e.ctx.ReportDeviceEvent(deviceCode, eventId, params)
			}
		}
	}
}

func (e *platformEngineImpl) handleRegisterRequest(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	topic := msg.Topic()
	match, gwSn := parseTopicGwSn(topic, "noyo/cascade/gw/%s/register/request")
	if !match {
		return
	}

	var req struct {
		GatewayName string `json:"gateway_name"`
	}
	_ = json.Unmarshal(msg.Payload(), &req)

	gwName := req.GatewayName
	if gwName == "" {
		gwName = "Auto-registered Gateway " + gwSn
	}

	e.logger.Info("Handling register request", zap.String("gw_sn", gwSn), zap.String("name", gwName))

	resp := struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "pending",
		Message: "waiting for approval",
	}

	gwDevice, err := store.GetDevice(gwSn)
	if err != nil || gwDevice == nil {
		e.logger.Info("Gateway device not found, registering as pending", zap.String("gw_sn", gwSn))

		// Auto-register the gateway device as "pending" (Enabled = false)
		newDev := &store.Device{
			Code:        gwSn,
			Name:        gwName,
			ProductCode: "noyo-gw",
			Enabled:     false, // Pending registration approval
		}
		if err := store.SaveDevice(newDev); err != nil {
			e.logger.Error("Failed to auto-register gateway device", zap.Error(err))
		}
	} else if gwDevice.Enabled {
		resp.Status = "success"
		resp.Message = "approved"
	}

	respBytes, _ := json.Marshal(resp)
	respTopic := fmt.Sprintf("noyo/cascade/gw/%s/register/response", gwSn)
	client.Publish(respTopic, 1, false, respBytes)
}

func (e *platformEngineImpl) handleSyncRequest(client mqtt.Client, msg mqtt.Message) {
	if msg.Retained() {
		return
	}
	topic := msg.Topic()
	match, gwSn := parseTopicGwSn(topic, "noyo/cascade/gw/%s/sync/request")
	if !match {
		return
	}

	var req struct {
		LastSyncTime int64 `json:"last_sync_time"`
	}
	if err := json.Unmarshal(msg.Payload(), &req); err != nil {
		e.logger.Error("Failed to parse sync request", zap.Error(err))
		return
	}

	// 在独立 goroutine 中执行 sync，避免阻塞 paho 的消息回调 goroutine。
	// 如果 sync 在消息回调中通过 token.Wait() 阻塞，会导致 paho 内部的消息处理
	// goroutine 被占用，最终 keepalive ping 无法按时发送，broker 在 1.5×KeepAlive
	// 后触发 LWT 导致平台状态变为 offline。
	go e.doSyncRequest(gwSn, req.LastSyncTime)
}

func (e *platformEngineImpl) doSyncRequest(gwSn string, lastSyncTime int64) {
	// 使用互斥锁防止并发 sync 操作（网关可能频繁发送 sync 请求）
	if !e.syncMu.TryLock() {
		e.logger.Info("Sync already in progress, skipping", zap.String("gw_sn", gwSn))
		return
	}
	defer e.syncMu.Unlock()

	e.logger.Info("Handling sync request", zap.String("gw_sn", gwSn), zap.Int64("last_sync_time", lastSyncTime))

	gwDevice, err := store.GetDevice(gwSn)
	if err != nil || gwDevice == nil {
		e.logger.Info("Gateway device not found, skip sync", zap.String("gw_sn", gwSn))
		return
	}

	if !gwDevice.Enabled {
		e.logger.Info("Gateway device is pending registration, skip sync", zap.String("gw_sn", gwSn))
		return
	}

	// 获取所有层级的子设备
	var subDevices []store.Device
	queue := []string{gwSn}
	for len(queue) > 0 {
		currentParent := queue[0]
		queue = queue[1:]
		children, _ := store.ListDevicesByParent(currentParent)
		if len(children) > 0 {
			subDevices = append(subDevices, children...)
			for i := range children {
				queue = append(queue, children[i].Code)
			}
		}
	}

	productMap := make(map[string]*store.Product)
	for _, sub := range subDevices {
		if _, ok := productMap[sub.ProductCode]; !ok {
			if p, err := store.GetProduct(sub.ProductCode); err == nil && p != nil {
				productMap[p.Code] = p
			}
		}
	}

	products := make([]*store.Product, 0, len(productMap))
	for _, p := range productMap {
		products = append(products, p)
	}

	allDevices := make([]*store.Device, 0, len(subDevices))
	for i := range subDevices {
		d := subDevices[i] // 复制一份
		if d.ParentCode == gwSn {
			d.ParentCode = "" // 在网关侧，直接挂在网关下的设备视作直连设备
		}
		allDevices = append(allDevices, &d)
	}

	resp := struct {
		Timestamp int64            `json:"timestamp"`
		Products  []*store.Product `json:"products"`
		Devices   []*store.Device  `json:"devices"`
	}{
		Timestamp: time.Now().Unix(),
		Products:  products,
		Devices:   allDevices,
	}

	respBytes, err := json.Marshal(resp)
	if err != nil {
		e.logger.Error("Failed to marshal sync response", zap.Error(err))
		return
	}

	e.logger.Info("Platform sending sync config to gateway", zap.String("gw_sn", gwSn), zap.Int("products_count", len(resp.Products)), zap.Int("devices_count", len(resp.Devices)))

	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("sync_%s_%d.json", gwSn, time.Now().UnixNano()))
	if err := os.WriteFile(tmpFile, respBytes, 0644); err != nil {
		e.logger.Error("Failed to write sync temp file", zap.Error(err))
		return
	}
	defer os.Remove(tmpFile)

	publishFunc := func(topic string, p []byte) error {
		token := e.client.Publish(topic, 1, false, p)
		token.Wait()
		return token.Error()
	}

	sender, err := NewFileSender(tmpFile, publishFunc, e.logger)
	if err != nil {
		e.logger.Error("Failed to create file sender", zap.Error(err))
		return
	}
	sender.Info.FileName = "sync_config.json"

	if err := sender.Send(gwSn); err != nil {
		e.logger.Error("Failed to send sync config", zap.Error(err))
	} else {
		e.logger.Info("Successfully sent sync config", zap.String("gw_sn", gwSn), zap.Int("bytes", len(respBytes)))
	}
}

func (e *platformEngineImpl) handleGatewayStatus(client mqtt.Client, msg mqtt.Message) {
	match, gwSn := parseTopicGwSn(msg.Topic(), "")
	if !match {
		return
	}

	var payload struct {
		Status    string `json:"status"`
		Timestamp int64  `json:"timestamp"`
	}
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		e.logger.Error("Failed to parse gateway status event", zap.Error(err))
		return
	}

	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok {
		return
	}

	statusStr := types.DeviceStatusOffline
	if payload.Status == "online" {
		statusStr = types.DeviceStatusOnline
	}

	lastActive := time.UnixMilli(payload.Timestamp)
	if payload.Timestamp == 0 {
		lastActive = time.Now()
	}

	status := core.DeviceStatus{
		Online:     payload.Status == "online",
		LastActive: lastActive,
		LastReport: time.Now(),
		LastStatus: statusStr,
	}
	coreServer.DeviceManager.ReportDeviceStatus(gwSn, status)

	// If gateway goes offline, mark all its sub-devices offline as well
	if statusStr == types.DeviceStatusOffline {
		e.setSubDevicesOffline(gwSn, coreServer.DeviceManager)
	}
}

func (e *platformEngineImpl) setSubDevicesOffline(gwSn string, dm *core.DeviceManager) {
	allDevices := dm.Registry.GetAllDevices()
	for _, dev := range allDevices {
		if e.isDeviceUnderGateway(dev, gwSn, dm.Registry) {
			status := core.DeviceStatus{
				Online:     false,
				LastActive: time.Now(),
				LastReport: time.Now(),
				LastStatus: types.DeviceStatusOffline,
			}
			dm.ReportDeviceStatus(dev.Code, status)
		}
	}
}

func (e *platformEngineImpl) isDeviceUnderGateway(dev *store.Device, gwSn string, reg *core.DeviceRegistry) bool {
	current := dev
	for current.ParentCode != "" {
		if current.ParentCode == gwSn {
			return true
		}
		parentDev, ok := reg.GetDevice(current.ParentCode)
		if !ok {
			break
		}
		current = parentDev
	}
	return false
}

// resetAllCascadeDevicesOffline 在平台启动时将所有级联网关及其子设备的状态重置为 offline
// 这确保了平台不会残留上次运行时的旧状态。
// 网关连接后会主动上报全量设备在线状态来恢复正确的状态。
func (e *platformEngineImpl) resetAllCascadeDevicesOffline() {
	coreServer, ok := e.ctx.GetCoreServer().(*core.Server)
	if !ok {
		e.logger.Warn("Core server not available, skipping cascade device status reset")
		return
	}

	e.logger.Info("Resetting all cascade sub-devices to offline on startup")

	// 直接从数据库中加载设备和产品，因为此时 DeviceManager 可能还未 Init 完成
	var devices []store.Device
	if err := store.DB.Find(&devices).Error; err != nil {
		e.logger.Error("Failed to query devices for status reset", zap.Error(err))
		return
	}

	var products []store.Product
	if err := store.DB.Find(&products).Error; err != nil {
		e.logger.Error("Failed to query products for status reset", zap.Error(err))
		return
	}

	productMap := make(map[string]*store.Product)
	for i := range products {
		productMap[products[i].Code] = &products[i]
	}

	deviceMap := make(map[string]*store.Device)
	for i := range devices {
		deviceMap[devices[i].Code] = &devices[i]
	}

	resetCount := 0

	for _, dev := range devices {
		product, ok := productMap[dev.ProductCode]
		if !ok {
			continue
		}

		isCascadeGateway := product.ProtocolName == "cascade"
		isCascadeSubDevice := false

		// Check if any ancestor is a cascade gateway
		currentParent := dev.ParentCode
		for currentParent != "" {
			parentDev, parentOk := deviceMap[currentParent]
			if parentOk {
				parentProduct, ppOk := productMap[parentDev.ProductCode]
				if ppOk && parentProduct.ProtocolName == "cascade" {
					isCascadeSubDevice = true
					break
				}
				currentParent = parentDev.ParentCode
			} else {
				break
			}
		}

		if isCascadeGateway || isCascadeSubDevice {
			status := core.DeviceStatus{
				Online:     false,
				LastActive: time.Now(),
				LastReport: time.Now(),
				LastStatus: "offline",
			}
			coreServer.DeviceManager.ReportDeviceStatus(dev.Code, status)
			resetCount++
		}
	}

	e.logger.Info("Cascade device status reset complete", zap.Int("reset_count", resetCount))
}
