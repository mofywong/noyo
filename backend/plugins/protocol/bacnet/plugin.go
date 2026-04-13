package bacnet

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"noyo/core"
	"noyo/core/protocol"
	"noyo/core/types"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alexbeltran/gobacnet"
	bactypes "github.com/alexbeltran/gobacnet/types"
	"go.uber.org/zap"
)

//go:embed icon.svg
var icon []byte

// PointConfig defines the point configuration for BACnet
type PointConfig struct {
	ObjectType int `json:"object_type"` // e.g., 0 (AI), 1 (AO)
	PropertyID int `json:"property_id"` // Default 85 (Present_Value)
}

// Config defines the global configuration for BACnet Plugin
type Config struct {
	Interface string `yaml:"interface" json:"interface" title_en:"Network Interface" title_zh:"网卡接口" desc_en:"Select network interface" desc_zh:"选择网卡接口"`
	Port      int    `yaml:"port" json:"port" title_en:"BACnet Port" title_zh:"监听端口" default:"47808" desc_en:"Default 47808 (BAC0)" desc_zh:"默认 47808 (BAC0)"`
}

// BacnetPlugin implements the IProtocolPlugin interface
type BacnetPlugin struct {
	protocol.BaseProtocolPlugin
	Config Config
	mu     sync.Mutex
	Logger *zap.Logger
	client *gobacnet.Client

	// Tasks: DeviceCode -> StopChan
	tasks map[string]chan struct{}
}

func init() {
	core.InstallPlugin[BacnetPlugin](core.PluginMeta{
		Name:     "BACnet",
		Category: types.PluginCategoryProtocol,
	})
}

// Init implements IProtocolPlugin
func (p *BacnetPlugin) Init(ctx protocol.Context) error {
	p.BaseProtocolPlugin.Init(ctx)
	p.Logger = ctx.GetLogger()
	p.tasks = make(map[string]chan struct{})
	return nil
}

// Start implements IProtocolPlugin
func (p *BacnetPlugin) Start() error {
	p.Logger.Info("Starting BACnet Plugin V3 (DEBUG-FILE)")

	// Get Config
	cfg := p.Ctx.GetConfig()
	// Usually cfg comes as map[string]interface{}.
	// If we use p.Config (struct), we should populate it from cfg or trust core to do it if it injected it.
	// But core inserts into BaseProtocolPlugin? No.
	// We should parse cfg into p.Config if we want to use the struct,
	// OR just continue using the map as before since it is dynamic.
	// However, for the Schema to work via reflection in GetPluginConfigSchema, p.Config MUST be populated or at least exist.
	// The core.GetPluginConfigSchema uses reflection on the plugin instance.
	// So defining the struct in BacnetPlugin is enough for the UI schema.
	// For runtime, we can still use the map or map it to struct.

	interfaceName, _ := cfg["interface"].(string)
	portVal, _ := cfg["port"].(float64)
	port := int(portVal)
	if port == 0 {
		port = 47808
	}

	// Auto-detect IP if interface not specified or "0.0.0.0" (which doesn't work well for broadcast)
	if interfaceName == "" || interfaceName == "0.0.0.0" {
		localIP, err := getOutboundIP()
		if err != nil {
			p.Logger.Warn("Failed to detect local IP, falling back to 0.0.0.0", zap.Error(err))
			interfaceName = "0.0.0.0"
		} else {
			ipStr := localIP.String()
			p.Logger.Info("Auto-detected local IP", zap.String("ip", ipStr))

			// Resolve Interface Name from IP
			ifName, err := getInterfaceNameByIP(ipStr)
			if err != nil {
				p.Logger.Warn("Failed to resolve interface name for IP, using IP string", zap.String("ip", ipStr), zap.Error(err))
				interfaceName = ipStr
			} else {
				interfaceName = ifName
				p.Logger.Info("Resolved interface name for BACnet binding", zap.String("interface", interfaceName))
			}
		}
	}

	// Initialize Client (With Fallback)
	var err error
	p.client, err = gobacnet.NewClient(interfaceName, port)
	if err != nil {
		p.Logger.Warn("Failed to create BACnet client with specific interface, retrying with 0.0.0.0",
			zap.String("interface", interfaceName),
			zap.Error(err))

		// Fallback to wildcard
		p.client, err = gobacnet.NewClient("0.0.0.0", port)
		if err != nil {
			p.Logger.Error("Failed to create BACnet client with 0.0.0.0", zap.Error(err))
			return err
		}
		p.Logger.Info("BACnet Plugin Started with fallback to 0.0.0.0")
	} else {
		p.Logger.Info("BACnet Plugin Started", zap.String("interface", interfaceName), zap.Int("port", port))
	}
	return nil
}

// Helper to find preferred outbound IP
func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP, nil
}

// Helper to find interface name by IP
func getInterfaceNameByIP(targetIP string) (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip.String() == targetIP {
				return i.Name, nil
			}
		}
	}
	return "", fmt.Errorf("interface not found for IP %s", targetIP)
}

// Stop implements IProtocolPlugin
func (p *BacnetPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, stop := range p.tasks {
		close(stop)
		delete(p.tasks, id)
	}

	if p.client != nil {
		p.client.Close()
	}
	p.Logger.Info("BACnet Plugin Stopped")
	return nil
}

// GetMeta implements IProtocolPlugin
func (p *BacnetPlugin) GetMeta() *types.PluginMeta {
	return &types.PluginMeta{
		Name: "BACnet",
		Title: map[string]string{
			"en": "BACnet/IP",
			"zh": "BACnet/IP协议",
		},
		Description: map[string]string{
			"en": "BACnet/IP Protocol Support",
			"zh": "BACnet/IP 协议支持",
		},
		Category: types.PluginCategoryProtocol,
		Icon:     icon,
	}
}

// SubDeviceConfigCustomizable implements IProtocolPlugin
func (p *BacnetPlugin) SubDeviceConfigCustomizable() bool {
	return false
}

// GetConfigSchema implements IConfigSchemaProvider
func (p *BacnetPlugin) GetConfigSchema() *core.PluginConfigSchema {
	meta := p.GetMeta()
	schema := &core.PluginConfigSchema{
		PluginName:  meta.Name,
		Title:       meta.Title,
		Description: meta.Description,
		Fields:      make([]core.ConfigField, 0),
	}

	// 1. Enabled Field
	schema.Fields = append(schema.Fields, core.ConfigField{
		Name:  "enabled",
		Type:  "switch",
		Title: map[string]string{"en": "Enable Plugin", "zh": "启用插件"},
		Value: p.IsEnabled(),
	})

	// 2. Interface Field (Select)
	ifaces, _ := net.Interfaces()
	options := []map[string]string{}
	// Add 0.0.0.0 option
	options = append(options, map[string]string{"label": "Auto / 0.0.0.0", "value": "0.0.0.0"})

	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		ipStr := ""
		for _, addr := range addrs {
			if ip, ok := addr.(*net.IPNet); ok && ip.IP.To4() != nil {
				ipStr = ip.IP.String()
				break
			}
		}
		label := i.Name
		if ipStr != "" {
			label = fmt.Sprintf("%s (%s)", i.Name, ipStr)
		}
		options = append(options, map[string]string{"label": label, "value": i.Name})
	}

	schema.Fields = append(schema.Fields, core.ConfigField{
		Name:    "interface",
		Type:    "select",
		Title:   map[string]string{"en": "Network Interface", "zh": "网卡接口"},
		Options: options,
		Value:   p.Config.Interface,
	})

	// 3. Port Field
	schema.Fields = append(schema.Fields, core.ConfigField{
		Name:        "port",
		Type:        "int",
		Title:       map[string]string{"en": "BACnet Port", "zh": "监听端口"},
		Description: map[string]string{"en": "Default 47808 (BAC0)", "zh": "默认 47808 (BAC0)"},
		Value:       p.Config.Port,
	})

	return schema
}

func (p *BacnetPlugin) GetDeviceConfigSchema(config types.DeviceMeta) ([]byte, error) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ip": map[string]interface{}{
				"type":     "string",
				"title":    "IP Address",
				"title_zh": "IP地址",
				"format":   "ipv4",
				"default":  "127.0.0.1",
			},
			"port": map[string]interface{}{
				"type":     "integer",
				"title":    "Port",
				"title_zh": "端口",
				"default":  47808,
				"minimum":  1,
				"maximum":  65535,
			},
			"device_id": map[string]interface{}{
				"type":     "integer",
				"title":    "Device Instance ID",
				"title_zh": "设备实例号",
				"minimum":  0,
				"maximum":  4194303,
			},
		},
		"required": []string{"ip", "device_id"},
	}
	return json.Marshal(schema)
}

func (p *BacnetPlugin) GetPointConfigSchema() ([]byte, error) {
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"object_type": map[string]interface{}{
				"type":     "integer",
				"title":    "Object Type",
				"title_zh": "对象类型",
				"enum":     []int{0, 1, 2, 3, 4, 5, 8, 13, 14, 19},
				"enumNames": []string{
					"Analog Input (AI)",
					"Analog Output (AO)",
					"Analog Value (AV)",
					"Binary Input (BI)",
					"Binary Output (BO)",
					"Binary Value (BV)",
					"Device",
					"Multi-state Input (MI)",
					"Multi-state Output (MO)",
					"Multi-state Value (MV)",
				},
				"default": 0,
			},
			"instance_id": map[string]interface{}{
				"type":     "integer",
				"title":    "Instance ID",
				"title_zh": "实例号",
				"minimum":  0,
				"default":  1,
			},
			"property_id": map[string]interface{}{
				"type":        "integer",
				"title":       "Property ID",
				"title_zh":    "属性ID",
				"default":     85,
				"description": "Default is 85 (Present_Value)",
			},
			"read_expr": map[string]interface{}{
				"type":        "string",
				"title":       "Read Expression",
				"title_zh":    "读取表达式",
				"description": "Expression to transform raw value. Variable: 'x'. Example: x * 0.1",
			},
			"poll_interval": map[string]interface{}{
				"type":        "integer",
				"title":       "Poll Interval (ms)",
				"title_zh":    "读取周期 (ms)",
				"default":     5000,
				"description": "Polling interval in milliseconds (default 5000)",
				"minimum":     100,
			},
			"write_expr": map[string]interface{}{
				"type":        "string",
				"title":       "Write Expression",
				"title_zh":    "写入表达式",
				"description": "Expression to transform value before write. Variable: 'x'. Example: x * 10",
			},
			"enable_write": map[string]interface{}{
				"type":     "boolean",
				"title":    "Enable Write",
				"title_zh": "允许写入",
				"default":  false,
			},
			"write_priority": map[string]interface{}{
				"type":     "integer",
				"title":    "Write Priority",
				"title_zh": "写入优先级",
				"default":  16,
				"minimum":  1,
				"maximum":  16,
			},
		},
		"required": []string{"object_type", "instance_id"},
	}
	return json.Marshal(schema)
}

// BatchAddDevice implements IProtocolPlugin
func (p *BacnetPlugin) BatchAddDevice(devices []types.DeviceMeta) error {
	fmt.Fprintf(os.Stderr, ">>> [DEBUG-STDERR] BatchAddDevice Called with %d devices\n", len(devices))
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, dev := range devices {
		p.addDevice(dev)
	}
	return nil
}

// RemoveDevice implements IProtocolPlugin
func (p *BacnetPlugin) RemoveDevice(deviceCode string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if stop, ok := p.tasks[deviceCode]; ok {
		close(stop)
		delete(p.tasks, deviceCode)
	}
	return nil
}

// UpdateDevice implements IProtocolPlugin
func (p *BacnetPlugin) UpdateDevice(device types.DeviceMeta) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop existing task
	if stop, ok := p.tasks[device.DeviceCode]; ok {
		close(stop)
		delete(p.tasks, device.DeviceCode)
	}

	// Start new task
	p.addDevice(device)
	return nil
}

// WriteProperty implements IProtocolPlugin
func (p *BacnetPlugin) WriteProperty(device types.DeviceMeta, propName string, value interface{}) error {
	// Parse Device Config
	extras := device.Extras
	ip, _ := extras["ip"].(string)
	portVal, _ := extras["port"].(float64)
	port := int(portVal)
	if port == 0 {
		port = 47808
	}
	deviceIDVal, _ := extras["device_id"].(float64)
	deviceID := uint32(deviceIDVal)

	if ip == "" {
		return fmt.Errorf("missing device IP")
	}

	// Find Point Config
	pointsList, ok := extras["points"].([]interface{})
	if !ok {
		return fmt.Errorf("no points configuration found")
	}

	var objType, instID, propID int
	var writePriority int = 16
	found := false

	for _, ptObj := range pointsList {
		ptMap, ok := ptObj.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := ptMap["name"].(string)
		if name == propName {
			objTypeVal, _ := ptMap["object_type"].(float64)
			objType = int(objTypeVal)
			instIDVal, _ := ptMap["instance_id"].(float64)
			instID = int(instIDVal)
			propIDVal, _ := ptMap["property_id"].(float64)
			propID = int(propIDVal)
			if propID == 0 {
				propID = 85 // Present_Value
			}

			// Check enable_write
			enableWrite, _ := ptMap["enable_write"].(bool)
			if !enableWrite {
				return fmt.Errorf("write disabled for point %s", propName)
			}

			// Get Priority
			prioVal, _ := ptMap["write_priority"].(float64)
			if prioVal > 0 {
				writePriority = int(prioVal)
			}

			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("point %s not found in configuration", propName)
	}

	// Check if writable object type
	// Analog Output(1), Analog Value(2), Binary Output(4), Binary Value(5), Multi-state Output(14), Multi-state Value(19)
	// We allow writing to any, generally, but these are standard writable.

	// Perform Write
	return p.performWriteProperty(ip, port, deviceID, objType, instID, propID, value, uint8(writePriority))
}

// WritePoint implements IProtocolPlugin
func (p *BacnetPlugin) WritePoint(device types.DeviceMeta, pointCode string, value interface{}) error {
	return p.WriteProperty(device, pointCode, value)
}

// Discover performs a WhoIs scan
func (p *BacnetPlugin) Discover(params map[string]interface{}) ([]protocol.DiscoveredDevice, error) {
	if p.client == nil {
		return nil, fmt.Errorf("client not started")
	}
	// Broadcast WhoIs (low=-1, high=-1 for all)
	devices, err := p.client.WhoIs(-1, -1)
	if err != nil {
		return nil, err
	}

	var result []protocol.DiscoveredDevice
	for _, d := range devices {
		// Extract IP and Port
		udpAddr, _ := d.Addr.UDPAddr()
		ip := udpAddr.IP.String()
		port := udpAddr.Port

		instanceID := uint32(d.ID.Instance)

		res := protocol.DiscoveredDevice{
			ExternalID: fmt.Sprintf("%d", instanceID),
			Name:       fmt.Sprintf("BACnet Device %d", instanceID),
			Config: map[string]interface{}{
				"device_id": instanceID,
				"ip":        ip,
				"port":      port,
			},
		}

		// Optional: We could try to read Model Name here, but it might be slow.
		// For auto-provisioning, we might do it later or in a separate step.

		// Add Vendor ID to config if useful
		res.Config["vendor_id"] = d.Vendor

		result = append(result, res)
	}
	return result, nil
}

// Helper for file debug
func logDebugFile(msg string) {
	f, err := os.OpenFile("debug_bacnet.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(time.Now().Format(time.RFC3339) + " " + msg + "\n")
}

func (p *BacnetPlugin) addDevice(device types.DeviceMeta) {
	logDebugFile(fmt.Sprintf("addDevice Called: Code=%s Extras=%v", device.DeviceCode, device.Extras))
	fmt.Fprintf(os.Stderr, ">>> [DEBUG-STDERR] addDevice Called: %s Extras: %v\n", device.DeviceCode, device.Extras)
	// Parse Config
	extras := device.Extras
	// Fallback to host if ip is empty
	ipRaw, _ := extras["ip"].(string)
	if ipRaw == "" {
		ipRaw, _ = extras["host"].(string)
	}
	ip := strings.TrimSpace(ipRaw)

	portVal, _ := extras["port"].(float64)
	port := int(portVal)
	if port == 0 {
		port = 47808
	}
	deviceIDVal, _ := extras["device_id"].(float64)
	deviceID := int(deviceIDVal)

	p.Logger.Info("Adding BACnet Device",
		zap.String("device", device.DeviceCode),
		zap.String("ip", ip),
		zap.Int("port", port),
		zap.Int("deviceID", deviceID),
		zap.Any("extras", extras))

	if ip == "" || ip == "0.0.0.0" || deviceID == 0 {
		logDebugFile(fmt.Sprintf("Skipping Invalid Config: IP='%s' ID=%d", ip, deviceID))
		p.Logger.Error("Skipping invalid BACnet device config (missing or invalid IP/ID)",
			zap.String("device", device.DeviceCode),
			zap.String("ip", ip),
			zap.Int("deviceID", deviceID))
		return
	}

	// Create Polling Task
	stop := make(chan struct{})
	p.tasks[device.DeviceCode] = stop

	go p.pollingLoop(device, ip, port, uint32(deviceID), stop)
}

func (p *BacnetPlugin) pollingLoop(meta types.DeviceMeta, ip string, port int, deviceID uint32, stop <-chan struct{}) {
	logDebugFile(fmt.Sprintf("pollingLoop Start: Device=%s IP='%s' Port=%d", meta.DeviceCode, ip, port))

	// Construct BACnet Device Object
	destAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		p.Logger.Error("Invalid BACnet polling address", zap.String("device", meta.DeviceCode), zap.Error(err))
		return
	}

	// EMERGENCY GUARD: Check for 0.0.0.0
	if destAddr.IP.IsUnspecified() || destAddr.IP.String() == "0.0.0.0" {
		logDebugFile(fmt.Sprintf("BLOCKING 0.0.0.0: Device=%s IP='%s' ResIP='%s'", meta.DeviceCode, ip, destAddr.IP.String()))
		fmt.Fprintf(os.Stderr, ">>> [DEBUG-STDERR] BLOCKING 0.0.0.0 in pollingLoop! Device: %s IP input: '%s'\n", meta.DeviceCode, ip)
		p.Logger.Error("Polling Loop: IP is 0.0.0.0 (Unspecified)! Exiting loop.", zap.String("device", meta.DeviceCode))
		return
	}

	// Fix for gobacnet: Ensure IP is 4 bytes if it's IPv4
	if ipv4 := destAddr.IP.To4(); ipv4 != nil {
		destAddr.IP = ipv4
	}

	bacAddr := bactypes.UDPToAddress(destAddr)
	logDebugFile(fmt.Sprintf("BACnet Address Constructed: %v (IP: %v, Port: %v)", bacAddr, destAddr.IP, destAddr.Port))

	bacDevice := bactypes.Device{
		ID: bactypes.ObjectID{
			Type:     8, // Device Object Type = 8
			Instance: bactypes.ObjectInstance(deviceID),
		},
		Addr:         bacAddr,
		MaxApdu:      1476,
		Segmentation: 0, // No segmentation assumed for simple queries
		Vendor:       0,
	}

	// Point polling configuration block
	type pointSched struct {
		Name     string
		ObjType  int
		InstID   int
		PropID   int
		Interval time.Duration
		NextRun  time.Time
	}
	var schedules []*pointSched

	// Initial parse
	pointsList, ok := meta.Extras["points"].([]interface{})
	if ok {
		for _, ptObj := range pointsList {
			ptMap, ok := ptObj.(map[string]interface{})
			if !ok {
				continue
			}

			name, _ := ptMap["name"].(string)
			objTypeVal, _ := ptMap["object_type"].(float64)
			instIDVal, _ := ptMap["instance_id"].(float64)

			propIDVal, _ := ptMap["property_id"].(float64)
			propID := int(propIDVal)
			if propID == 0 {
				propID = 85 // Present_Value
			}

			intervalVal, _ := ptMap["poll_interval"].(float64)
			intervalMs := int(intervalVal)
			if intervalMs <= 0 {
				intervalMs = 5000 // default 5s
			}

			schedules = append(schedules, &pointSched{
				Name:     name,
				ObjType:  int(objTypeVal),
				InstID:   int(instIDVal),
				PropID:   propID,
				Interval: time.Duration(intervalMs) * time.Millisecond,
				NextRun:  time.Now(), // Execute immediately on first run
			})
		}
	}

	if len(schedules) == 0 {
		p.Logger.Warn("No points configured for polling", zap.String("device", meta.DeviceCode))
		return
	}

	for {
		now := time.Now()
		var nextWait time.Duration = time.Hour // Default large wait
		var ptToRun *pointSched

		// Find the point that needs to be run next (or immediately)
		for _, s := range schedules {
			wait := s.NextRun.Sub(now)
			if wait <= 0 {
				ptToRun = s
				// Found a point to run immediately, no need to wait
				nextWait = 0
				break
			}
			if wait < nextWait {
				nextWait = wait
				// ptToRun is not set here because we will sleep first
			}
		}

		if ptToRun == nil && nextWait > 0 {
			// Sleep until the next point is ready (or stop is called)
			timer := time.NewTimer(nextWait)
			select {
			case <-stop:
				timer.Stop()
				return
			case <-timer.C:
				// Timer expired, just continue the loop to re-evaluate schedules
			}
			continue
		}

		if ptToRun != nil {
			// Execute Read
			val, err := p.readPoint(bacDevice, ptToRun.ObjType, ptToRun.InstID, ptToRun.PropID)
			if err != nil {
				p.Logger.Warn("Read BACnet Point Failed",
					zap.String("device", meta.DeviceCode),
					zap.String("point", ptToRun.Name),
					zap.Error(err))
			} else {
				// Report
				p.Ctx.ReportProperty(meta, ptToRun.Name, val)
			}

			// Update next run time
			ptToRun.NextRun = time.Now().Add(ptToRun.Interval)
		} else {
			// Fallback (should ideally not happen, but prevents tight loop bug)
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

func (p *BacnetPlugin) readPoint(dev bactypes.Device, objType int, instID int, propID int) (interface{}, error) {
	req := bactypes.ReadPropertyData{
		Object: bactypes.Object{
			ID: bactypes.ObjectID{
				Type:     bactypes.ObjectType(objType),
				Instance: bactypes.ObjectInstance(instID),
			},
			Properties: []bactypes.Property{
				{
					Type:       uint32(propID),
					ArrayIndex: bactypes.ArrayAll,
				},
			},
		},
	}

	resp, err := p.client.ReadProperty(dev, req)
	if err != nil {
		return nil, err
	}

	// Extract Value
	// Resp should contain the object with populated properties
	if len(resp.Object.Properties) == 0 {
		return nil, fmt.Errorf("no property data returned")
	}

	// Extract Value
	if len(resp.Object.Properties) == 0 {
		return nil, fmt.Errorf("no property data returned")
	}

	rawVal := resp.Object.Properties[0].Data
	return p.castReadValue(rawVal), nil
}

func (p *BacnetPlugin) castReadValue(v interface{}) interface{} {
	switch val := v.(type) {
	case float32:
		return float64(val)
	case float64:
		return val
	case int:
		return float64(val)
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint32:
		return float64(val)
	case uint64:
		return float64(val)
	case bool:
		if val {
			return 1.0
		}
		return 0.0
	// Handle strings if needed
	case string:
		return val
	default:
		// Try string conversion or keep as is
		return val
	}
}
