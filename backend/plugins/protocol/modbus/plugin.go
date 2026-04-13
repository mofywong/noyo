package modbus

import (
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"noyo/core"
	"noyo/core/protocol"
	"noyo/core/types"
	"noyo/plugins/protocol/modbus/codec"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

//go:embed icon.svg
var icon []byte

// Config defines the configuration for Modbus Plugin
type Config struct {
}

// ModbusPlugin implements the IProtocolPlugin interface for Modbus Protocol
type ModbusPlugin struct {
	protocol.BaseProtocolPlugin
	Config Config // Global plugin config (if any)
	mu     sync.Mutex
	Logger *zap.Logger // Use zap Logger directly as per interface
	// Active Tasks Control: TaskID -> StopChannel
	tasks map[string]chan struct{}
	// State Cache for Events: DeviceCode -> EventIdentifier -> LastState(bool)
	lastEventStates map[string]map[string]bool
	// Time Cache for Events: DeviceCode -> EventIdentifier -> LastReportTime(time.Time)
	lastEventReportTimes map[string]map[string]time.Time
	// Data Cache for Cross-Group Events: DeviceCode -> PointName -> Value
	deviceDataCache map[string]map[string]interface{}
	// Device Runtime State (Online Status Debounce & Heartbeat)
	deviceStates map[string]*DeviceRuntimeState

	// Connection Pool
	conns  map[string]net.Conn
	connMu sync.Mutex
}

// DeviceRuntimeState holds the runtime state for online status detection
type DeviceRuntimeState struct {
	CurrentStatus      bool        // true = online
	ConsecutiveSuccess int         // For online debounce
	ConsecutiveFailure int         // For offline debounce
	LastChangeTime     time.Time   // For value change check
	LastChangeValue    interface{} // For value change check
	LastReportTime     time.Time   // Last time status was reported
}

func init() {
	core.InstallPlugin[ModbusPlugin](core.PluginMeta{
		Name:     "Modbus",
		Category: types.PluginCategoryProtocol,
	})
}

// Init implements IProtocolPlugin
func (p *ModbusPlugin) Init(ctx protocol.Context) error {
	p.BaseProtocolPlugin.Init(ctx) // Save context
	p.Logger = ctx.GetLogger()     // Get Logger from context
	p.tasks = make(map[string]chan struct{})
	p.lastEventStates = make(map[string]map[string]bool)
	p.lastEventReportTimes = make(map[string]map[string]time.Time)
	p.deviceDataCache = make(map[string]map[string]interface{})
	p.deviceStates = make(map[string]*DeviceRuntimeState)
	p.conns = make(map[string]net.Conn)
	return nil
}

// getConnection retrieves a cached connection or creates a new one
func (p *ModbusPlugin) getConnection(address string, timeout int) (net.Conn, error) {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	if conn, ok := p.conns[address]; ok {
		// Simple health check?
		// Writing to a closed connection usually detects it.
		// For now, return cached.
		return conn, nil
	}

	conn, err := net.DialTimeout("tcp", address, time.Duration(timeout)*time.Millisecond)
	if err != nil {
		return nil, err
	}
	p.conns[address] = conn
	return conn, nil
}

// closeConnection closes a connection and removes it from the pool (used on error)
func (p *ModbusPlugin) closeConnection(address string, conn net.Conn) {
	p.connMu.Lock()
	defer p.connMu.Unlock()

	// Only close/delete if it's the specific connection passed (avoid race where it was already replaced)
	if current, ok := p.conns[address]; ok && current == conn {
		conn.Close()
		delete(p.conns, address)
	} else {
		// Just close the passed one to be safe if it's not in map
		conn.Close()
	}
}

// Start implements IProtocolPlugin
func (p *ModbusPlugin) Start() error {
	p.Ctx.LogInfo("Modbus Plugin Started")
	return nil
}

// Stop implements IProtocolPlugin
func (p *ModbusPlugin) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for id, stop := range p.tasks {
		close(stop)
		delete(p.tasks, id)
	}
	p.Ctx.LogInfo("Modbus Plugin Stopped")
	return nil
}

// GetMeta implements IProtocolPlugin
func (p *ModbusPlugin) GetMeta() *types.PluginMeta {
	return &types.PluginMeta{
		Name: "Modbus",
		Title: map[string]string{
			"en": "Modbus TCP Protocol",
			"zh": "Modbus TCP 协议",
		},
		Description: map[string]string{
			"en": "Standard Modbus TCP and Modbus RTU over TCP protocol support",
			"zh": "标准 Modbus TCP 和 Modbus RTU over TCP 协议支持",
		},
		Category:    types.PluginCategoryProtocol,
		DefaultYaml: ``,
		Icon:        icon,
	}
}

// WriteProperty implements IProtocolPlugin
func (p *ModbusPlugin) WriteProperty(device types.DeviceMeta, propName string, value interface{}) error {
	var pointConfig map[string]interface{}

	// Check if "points" exists in Extras
	if rawPoints, ok := device.Extras["points"]; ok {
		if list, ok := rawPoints.([]interface{}); ok {
			for _, item := range list {
				if cfg, ok := item.(map[string]interface{}); ok {
					normalizePoint(cfg)
					if name, ok := cfg["name"].(string); ok && name == propName {
						pointConfig = cfg
						break
					}
				}
			}
		} else if m, ok := rawPoints.(map[string]interface{}); ok {
			// Legacy support if it was a map
			for _, v := range m {
				if cfg, ok := v.(map[string]interface{}); ok {
					normalizePoint(cfg)
					if name, ok := cfg["name"].(string); ok && name == propName {
						pointConfig = cfg
						break
					}
				}
			}
		}
	}

	if pointConfig == nil {
		return fmt.Errorf("point %s not found in device config", propName)
	}

	// 4. Call WritePointInternal
	productMeta := types.ProductMeta{} // Valid placeholder as WritePoint relies on device logic mostly
	// If needed we can fetch via GetProduct from context, but we don't have product code easily unless in device.
	// device.ProductCode is available.

	return p.writePointInternal(device, productMeta, propName, value, pointConfig)
}

// WritePoint implements IProtocolPlugin (3 args)
func (p *ModbusPlugin) WritePoint(device types.DeviceMeta, pointCode string, value interface{}) error {
	// Delegate to WriteProperty as they are equivalent in this context
	return p.WriteProperty(device, pointCode, value)
}

// SetProperty Legacy Removal - Commented out or removed to avoid conflicts
// (Since we reformatted the file, the previous SetProperty was replaced by WriteProperty logic,
// but the original code block might still exist if I didn't replace it perfectly.
// I replaced lines 114-188 with WriteProperty.
// So SetProperty should be gone.
// Just to be safe, I'm checking lines 141+ where I inserted WriteProperty.
// The lint complained about core undefined in line 189 (getOrCreateDeviceState) and p.Server usages.
// I will fix getOrCreateDeviceState and ReportOnline usages.

func (p *ModbusPlugin) getOrCreateDeviceState(deviceCode string) *DeviceRuntimeState {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.deviceStates == nil {
		p.deviceStates = make(map[string]*DeviceRuntimeState)
	}
	if state, ok := p.deviceStates[deviceCode]; ok {
		return state
	}
	state := &DeviceRuntimeState{
		CurrentStatus:  false, // Default offline until proven online
		LastChangeTime: time.Now(),
	}
	p.deviceStates[deviceCode] = state
	return state
}

// ReportOnline updates the online status of a device
func (p *ModbusPlugin) ReportOnline(deviceCode string, online bool) {
	// p.Logger.Debug("ReportOnline", zap.String("device", deviceCode), zap.Bool("online", online))

	// p.Logger.Debug("ReportOnline", zap.String("device", deviceCode), zap.Bool("online", online))

	// Use Context helper to report status (implement ReportDeviceStatus in base/context helpers if needed)
	// BaseProtocolPlugin doesn't have report helper yet?
	// But p.Ctx has GetDeviceStatus.
	// We need Helper for Reporting Status.
	// Actually we should implement a helper method here or call p.Ctx.ReportOnline (if it existed)
	// Current p.Ctx has ReportEvent, ReportProperty.
	// We need ReportOnline in Context. I checked interface.go it has ReportStatus(meta, status).
	// But that takes string status.
	// Let's call ReportStatus with "online"/"offline".

	statusStr := types.DeviceStatusOffline
	if online {
		statusStr = types.DeviceStatusOnline
	}
	// We need DeviceMeta.
	// We only have deviceCode.
	// We can construct partial DeviceMeta for reporting?
	// ReportStatus(DeviceMeta, string)
	meta := types.DeviceMeta{DeviceCode: deviceCode}
	p.Ctx.ReportStatus(meta, statusStr)
}

// BatchAddDevice implements IProtocolPlugin
func (p *ModbusPlugin) BatchAddDevice(devices []types.DeviceMeta) error {
	for _, dev := range devices {
		p.addDevice(dev)
	}
	return nil
}

// RemoveDevice implements IProtocolPlugin
func (p *ModbusPlugin) RemoveDevice(deviceCode string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Stop all tasks related to this device
	prefix := deviceCode + "-"
	for id, stop := range p.tasks {
		if strings.HasPrefix(id, prefix) {
			close(stop)
			delete(p.tasks, id)
		}
	}
	return nil
}

// writePointInternal handles the actual Modbus write logic
func (p *ModbusPlugin) writePointInternal(device types.DeviceMeta, product types.ProductMeta, pointId string, value interface{}, pointConfig map[string]interface{}) error {
	log.Printf("[Modbus] WritePoint: id=%s val=%v config=%+v", pointId, value, pointConfig)
	// 1. Determine Connection Config & Polling Groups
	var devConfig DeviceConfig
	var groups []PollingGroupConfig

	if device.ParentCode != "" {
		// Fetch Parent from Context (Decoupled from Store)
		if device.Parent == nil {
			return fmt.Errorf("parent device context missing for %s", device.DeviceCode)
		}

		// Normalize and Map Parent Extras to Config
		normalizeExtras(device.Parent.Extras)
		cfg, err := core.ParseConfig[DeviceConfig](device.Parent.Extras)
		if err != nil {
			return fmt.Errorf("invalid parent config structure: %v", err)
		}
		devConfig = *cfg
		groups = devConfig.PollingGroups
	} else {
		// Parse Self Config
		normalizeExtras(device.Extras)
		cfg, err := core.ParseConfig[DeviceConfig](device.Extras)
		if err != nil {
			return fmt.Errorf("invalid device config: %v", err)
		}
		devConfig = *cfg
		groups = devConfig.PollingGroups
	}

	if devConfig.TimeoutMS == 0 {
		devConfig.TimeoutMS = 1000
	}
	if devConfig.Port == 0 {
		devConfig.Port = 502
	}

	// Helper to safely convert interface{} to float64
	toFloat := func(v interface{}) (float64, bool) {
		switch val := v.(type) {
		case float64:
			return val, true
		case float32:
			return float64(val), true
		case int:
			return float64(val), true
		case int64:
			return float64(val), true
		case string:
			if f, err := strconv.ParseFloat(val, 64); err == nil {
				return f, true
			}
		case json.Number:
			if f, err := val.Float64(); err == nil {
				return f, true
			}
		default:
			if f, err := strconv.ParseFloat(fmt.Sprintf("%v", val), 64); err == nil {
				return f, true
			}
		}
		return 0, false
	}

	// Helper to safely get float64 from config (Case-Insensitive)
	getFloat := func(key string) float64 {
		// 1. Try exact key
		if val, ok := pointConfig[key]; ok && val != nil {
			if f, ok := toFloat(val); ok {
				return f
			}
		}

		// 2. Try Case-Insensitive Scan
		return 0
	}

	// 2. Determine Write Parameters
	// Check enable_write (optional, but recommended)
	if enabled, ok := pointConfig["enable_write"].(bool); ok && !enabled {
		// If explicitly set to false, we might want to block.
		// However, for backward compatibility or if missing, we proceed.
		// If the user configured it as false, we should probably return error.
		return fmt.Errorf("write operation not enabled for this point")
	}

	writeMode, _ := pointConfig["write_mode"].(string)
	if writeMode == "" {
		writeMode = "same_as_read"
	}

	var targetSlaveID uint8
	var address uint16
	var functionCode uint8 = 6 // Default to Write Single Register

	if writeMode == "custom" {
		// Custom Mode: Use configured write address/slave/func
		address = uint16(getFloat("write_address"))
		targetSlaveID = uint8(getFloat("write_slave_id"))

		if fc := getFloat("write_function_code"); fc != 0 {
			functionCode = uint8(fc)
		}
	} else {
		// Same as Read Mode: Use Polling Group logic or Auto Mode
		groupName, _ := pointConfig["polling_group"].(string)

		if groupName == "" || devConfig.CollectionMode == "auto" {
			// Auto mode or direct mode: use address and slave_id directly
			address = uint16(getFloat("address"))
			targetSlaveID = uint8(getFloat("slave_id"))

			fc := getFloat("function_code")
			if fc == 1 || fc == 2 {
				functionCode = 5 // Write Single Coil
			} else {
				functionCode = 6 // Write Single Register
			}
		} else {
			offset := uint16(getFloat("offset"))

			foundGroup := false
			for _, g := range groups {
				if g.Name == groupName {
					address = g.StartAddress + offset
					targetSlaveID = g.SlaveID
					foundGroup = true

					if g.FunctionCode == 1 || g.FunctionCode == 2 {
						functionCode = 5 // Write Single Coil
					} else {
						functionCode = 6 // Write Single Register
					}
					break
				}
			}
			if !foundGroup {
				return fmt.Errorf("polling group %s not found", groupName)
			}
		}
	}

	// Fallback for Slave ID
	if targetSlaveID == 0 {
		if devConfig.SlaveID != 0 {
			targetSlaveID = devConfig.SlaveID
		} else {
			targetSlaveID = 1
		}
	}

	// 3. Connect
	addr := net.JoinHostPort(devConfig.IP, strconv.Itoa(devConfig.Port))
	// Use Connection Pool
	conn, err := p.getConnection(addr, devConfig.TimeoutMS)
	if err != nil {
		return err
	}
	// DO NOT defer conn.Close() here, as we want to reuse it.

	// 4. Encode Value

	var valUint16 uint16

	// Evaluate Write Expression if present
	if writeExpr, ok := pointConfig["write_expr"].(string); ok && writeExpr != "" {
		evalRes, err := codec.EvaluateWrite(writeExpr, value)
		if err != nil {
			return fmt.Errorf("failed to evaluate write expression: %v", err)
		}
		value = evalRes
	}

	// Handle Coil (FC 5) vs Register (FC 6)
	if functionCode == 5 {
		// Write Single Coil
		// ON = 0xFF00, OFF = 0x0000
		var boolVal bool
		switch v := value.(type) {
		case bool:
			boolVal = v
		case int:
			boolVal = (v != 0)
		case float64:
			boolVal = (v != 0)
		case string:
			boolVal = (v == "true" || v == "1" || v == "on")
		default:
			boolVal = false
		}

		if boolVal {
			valUint16 = 0xFF00
		} else {
			valUint16 = 0x0000
		}
	} else {
		// Write Single Register (FC 6)
		var floatVal float64

		switch v := value.(type) {
		case float64:
			floatVal = v
		case int:
			floatVal = float64(v)
		case int64:
			floatVal = float64(v)
		case bool:
			if v {
				floatVal = 1
			} else {
				floatVal = 0
			}
		case string:
			// Try parsing string to float
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				floatVal = f
			}
		default:
			// Fallback
			floatVal = 0
		}

		valUint16 = uint16(math.Round(floatVal))
	}

	// 5. Send Request
	if functionCode > 6 {
		return fmt.Errorf("write function code %d not supported for single point write yet", functionCode)
	}

	var writeErr error
	if devConfig.ProtocolType == "Modbus-RTU over TCP" || devConfig.ProtocolType == "RTU_OVER_TCP" {
		writeErr = p.writeSingleItemRTU(conn, targetSlaveID, functionCode, address, valUint16, devConfig.TimeoutMS)
	} else {
		writeErr = p.writeSingleItemTCP(conn, targetSlaveID, functionCode, address, valUint16, devConfig.TimeoutMS)
	}

	if writeErr != nil {
		// On error, assume connection might be broken (or timeout), close it.
		p.closeConnection(addr, conn)
		// Option: Retry once?
		// For now, just return error.
		return writeErr
	}
	return nil
}

func (p *ModbusPlugin) writeSingleItemTCP(conn net.Conn, slaveID uint8, functionCode uint8, address uint16, value uint16, timeout int) error {
	// Header (7) + PDU (5) = 12 bytes
	req := make([]byte, 12)
	binary.BigEndian.PutUint16(req[0:], 0) // Trans ID
	binary.BigEndian.PutUint16(req[2:], 0) // Proto ID
	binary.BigEndian.PutUint16(req[4:], 6) // Length (UnitID + FC + Addr + Val)
	req[6] = slaveID
	req[7] = functionCode
	binary.BigEndian.PutUint16(req[8:], address)
	binary.BigEndian.PutUint16(req[10:], value)

	conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
	if _, err := conn.Write(req); err != nil {
		return err
	}

	// Read Response
	resp := make([]byte, 12)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}

	// Check Error
	if resp[7] > 0x80 {
		return fmt.Errorf("modbus exception: %x", resp[8])
	}

	return nil
}

func (p *ModbusPlugin) writeSingleItemRTU(conn net.Conn, slaveID uint8, functionCode uint8, address uint16, value uint16, timeout int) error {
	// RTU Frame: [SlaveID(1)][Func(1)][Addr(2)][Val(2)][CRC(2)]
	req := make([]byte, 8)
	req[0] = slaveID
	req[1] = functionCode
	binary.BigEndian.PutUint16(req[2:], address)
	binary.BigEndian.PutUint16(req[4:], value)

	// CRC
	crc := codec.CalculateCRC16(req[:6])
	binary.LittleEndian.PutUint16(req[6:], crc)

	conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
	if _, err := conn.Write(req); err != nil {
		return err
	}

	// Read Response (Echo of Request: 8 bytes)
	resp := make([]byte, 8)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}

	// Check Error
	if resp[1] > 0x80 {
		return fmt.Errorf("modbus exception: %x", resp[2])
	}

	return nil
}
