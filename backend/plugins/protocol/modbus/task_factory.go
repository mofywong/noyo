package modbus

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"noyo/core/types"
	"noyo/plugins/protocol/modbus/codec"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
	// "go.uber.org/zap"
)

// PollingGroupConfig defines a group of registers to read together
type PollingGroupConfig struct {
	Name         string `json:"name"`
	Enable       bool   `json:"enable"`        // Enable this group
	SlaveID      uint8  `json:"slave_id"`      // Target Slave ID for this group
	Interval     int    `json:"interval"`      // Interval in milliseconds
	FunctionCode int    `json:"function_code"` // 1, 2, 3, 4
	StartAddress uint16 `json:"start_address"`
	Length       uint16 `json:"length"`
	Description  string `json:"description"`
}

// EventTrigger defines the condition for an event
type EventTrigger struct {
	Point    string  `json:"point"`
	Operator string  `json:"operator"` // >, <, ==, !=, >=, <=, change
	Value    float64 `json:"value"`
	Debounce int     `json:"debounce"` // ms
}

// EventRule defines a rule to trigger an event
type EventRule struct {
	Identifier     string            `json:"identifier"`
	Name           string            `json:"name"`
	Trigger        EventTrigger      `json:"trigger"`         // Deprecated: use Triggers
	Triggers       []EventTrigger    `json:"triggers"`        // List of triggers
	TriggerLogic   string            `json:"trigger_logic"`   // "or" (default) or "and"
	Conditions     []EventTrigger    `json:"conditions"`      // List of conditions
	ConditionLogic string            `json:"condition_logic"` // "and" (default) or "or"
	ReportInterval *int              `json:"report_interval"` // nil/-1: Rising Edge, 0: Always, >0: Throttle
	Params         map[string]string `json:"params"`          // ParamID -> PointName
}

// OnlineRule defines how to determine if a device is online
type OnlineRule struct {
	Strategy             string  `json:"strategy"` // communication, custom_point
	Point                string  `json:"point"`    // Point name to check (if strategy=custom_point)
	Operator             string  `json:"operator"` // ==, !=, >, bit_and
	Value                float64 `json:"value"`
	OnlineDebounce       int     `json:"online_debounce"`        // Consecutive successes to mark online
	OfflineDebounce      int     `json:"offline_debounce"`       // Consecutive failures to mark offline
	EnableValueCheck     bool    `json:"enable_value_check"`     // Enable value change check
	MonitorPoint         string  `json:"monitor_point"`          // Point to monitor for changes
	MaxUnchangedInterval int     `json:"max_unchanged_interval"` // Max seconds without change
}

// DeviceConfig defines the connection parameters (Gateway)
type DeviceConfig struct {
	ProtocolType   string               `json:"protocol_type"` // TCP or RTU_OVER_TCP
	IP             string               `json:"ip"`
	Port           int                  `json:"port"`
	SlaveID        uint8                `json:"slave_id"` // Default SlaveID for Gateway itself (optional)
	TimeoutMS      int                  `json:"timeout_ms"`
	MaxGroupLength int                  `json:"max_group_length"`
	MaxAddressGap  int                  `json:"max_address_gap"`
	CollectionMode string               `json:"collection_mode"` // manual or auto
	PollingGroups  []PollingGroupConfig `json:"polling_groups"`
	Points         []ChildPoint         `json:"points"`
	Events         []EventRule          `json:"events"`
	OnlineRule     *OnlineRule          `json:"online_rule"`
}

// ProductConfig defines the protocol configuration (Polling Groups)
type ProductConfig struct {
	// PollingGroups moved to DeviceConfig
}

// ChildPoint defines the point configuration for a sub-device
type ChildPoint struct {
	Name         string `json:"name"`
	PollingGroup string `json:"polling_group"`

	// V2.0 New Fields
	ExtractRule codec.ExtractRule `json:"extract_rule"`
	ReadExpr    string            `json:"read_expr"`
	Precision   int               `json:"precision"`
	WriteConfig *WriteConfig      `json:"write_config"`

	// Direct Mode Fields
	SlaveID      uint8  `json:"slave_id"`
	FunctionCode int    `json:"function_code"`
	Address      uint16 `json:"address"`
	Interval     int    `json:"interval"`    // Unit: ms
	IsProperty   *bool  `json:"is_property"` // Default true
	codec.PointConfig
}

// WriteConfig defines writing control logic
type WriteConfig struct {
	Enable       bool        `json:"enable"`
	FunctionCode int         `json:"function_code"` // 05, 06, 15, 16
	Limit        *WriteLimit `json:"limit"`
}

type WriteLimit struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// ChildDeviceConfig defines the sub-device configuration
type ChildDeviceConfig struct {
	SlaveID    int         `json:"slave_id"`
	Points     interface{} `json:"points"`
	Events     []EventRule `json:"events"`
	OnlineRule *OnlineRule `json:"online_rule"`
}

// ChildMeta holds the pre-processed configuration for a child device (or self)
type ChildMeta struct {
	Meta         types.DeviceMeta
	Config       ChildDeviceConfig
	ParsedPoints []ChildPoint
	Events       []EventRule // Parsed Events
	OnlineRule   *OnlineRule // Parsed OnlineRule
}

// addDevice generates and starts polling tasks
func (p *ModbusPlugin) addDevice(device types.DeviceMeta) error {
	// 1. Strategy Check: If this is a SubDevice (has Parent), we don't create networking tasks for it.
	// The Gateway (Parent) will handle the polling.
	if device.ParentCode != "" {
		return nil
	}

	normalizeExtras(device.Extras)

	// Check if IP is missing but Host exists
	if val, ok := device.Extras["host"]; ok && val != "" {
		if device.Extras["ip"] == "" || device.Extras["ip"] == nil {
			// p.Logger.Warn("Forcing host to ip mapping", zap.Any("host", val))
			device.Extras["ip"] = val
		}
	}

	// 2. Parse Gateway Config
	// We use p.ParseConfig helper if available or manual unmarshal
	// Core has ParseConfig generic, but we want to avoid core dependency?
	// We can just use json.Unmarshal since we have the struct.
	// But Extras is map[string]interface{}.
	// Helper:
	var devConfig DeviceConfig
	b, _ := json.Marshal(device.Extras)
	json.Unmarshal(b, &devConfig)

	// Defaults
	if devConfig.TimeoutMS == 0 {
		devConfig.TimeoutMS = 1000
	}
	if devConfig.Port == 0 {
		devConfig.Port = 502
	}
	if devConfig.MaxGroupLength == 0 {
		devConfig.MaxGroupLength = 120
	}
	if devConfig.MaxAddressGap == 0 {
		devConfig.MaxAddressGap = 20
	}

	// Validate Gateway Config
	if devConfig.IP == "" {
		// If it's a standalone device without IP (e.g. just created), skip
		// p.Logger.Warn("Invalid gateway config: IP is required", zap.String("device", device.DeviceCode))
		return nil
	}

	// 3. Process Children
	childrenMetas := make([]ChildMeta, 0, len(device.SubDevices))
	for _, childMeta := range device.SubDevices {
		var cConfig ChildDeviceConfig
		if len(childMeta.Extras) > 0 {
			normalizeExtras(childMeta.Extras)
			cb, _ := json.Marshal(childMeta.Extras)
			json.Unmarshal(cb, &cConfig)
		}

		parsedPoints := parsePoints(cConfig.Points)

		childrenMetas = append(childrenMetas, ChildMeta{
			Meta:         childMeta,
			Config:       cConfig,
			ParsedPoints: parsedPoints,
			OnlineRule:   cConfig.OnlineRule,
			Events:       cConfig.Events,
		})
	}

	// Include Gateway itself
	{
		var selfConfig ChildDeviceConfig
		if pointsData, ok := device.Extras["points"]; ok {
			selfConfig.Points = pointsData
		}
		// Also parse events/rules from root
		// ... (mapping logic)

		parsedPoints := parsePoints(selfConfig.Points)
		if len(parsedPoints) > 0 {
			childrenMetas = append(childrenMetas, ChildMeta{
				Meta:         device,
				Config:       selfConfig,
				ParsedPoints: parsedPoints,
				OnlineRule:   devConfig.OnlineRule,
				Events:       devConfig.Events,
			})
		}
	}

	// 4. Auto Collection Mode
	if devConfig.CollectionMode == "auto" || devConfig.CollectionMode == "AutoReport" {
		autoGroups, err := p.generateAutoGroupsAndAssign(&childrenMetas, devConfig.MaxGroupLength, devConfig.MaxAddressGap)
		if err != nil {
			// p.Logger.Error("Failed to generate auto groups", zap.Error(err))
			return err
		}
		devConfig.PollingGroups = autoGroups
	} else {
		// Optimize Manual Groups
		var optimizedGroups []PollingGroupConfig
		for _, group := range devConfig.PollingGroups {
			if !group.Enable {
				continue
			}
			if int(group.Length) > devConfig.MaxGroupLength {
				// Split Logic
				remaining := int(group.Length)
				currentStart := int(group.StartAddress)

				for remaining > 0 {
					chunkSize := devConfig.MaxGroupLength
					if remaining < chunkSize {
						chunkSize = remaining
					}

					newGroup := group // copy
					newGroup.StartAddress = uint16(currentStart)
					newGroup.Length = uint16(chunkSize)
					newGroup.Name = fmt.Sprintf("%s_%d", group.Name, currentStart)

					optimizedGroups = append(optimizedGroups, newGroup)

					currentStart += chunkSize
					remaining -= chunkSize
				}
			} else {
				optimizedGroups = append(optimizedGroups, group)
			}
		}
		devConfig.PollingGroups = optimizedGroups
	}

	// 5. Start Tasks
	for _, group := range devConfig.PollingGroups {
		if !group.Enable {
			continue
		}

		// Interval Check
		if group.Interval <= 0 {
			group.Interval = 10000
		}
		if group.Interval < 100 {
			group.Interval = 100
		}

		taskID := fmt.Sprintf("%s-%s", device.DeviceCode, group.Name)
		stopChan := make(chan struct{})

		p.mu.Lock()
		// Stop existing if any (shouldn't happen on fresh add, but safe)
		if oldStop, exists := p.tasks[taskID]; exists {
			close(oldStop)
		}
		p.tasks[taskID] = stopChan
		p.mu.Unlock()

		// Launch Goroutine
		go p.runPollingLoop(device, devConfig, group, childrenMetas, stopChan)
	}

	return nil
}

type autoGroupKey struct {
	SlaveID      uint8
	FunctionCode int
	Interval     int
}

func (p *ModbusPlugin) generateAutoGroupsAndAssign(childrenMetas *[]ChildMeta, maxLen, maxGap int) ([]PollingGroupConfig, error) {
	pointMap := make(map[autoGroupKey][]*ChildPoint)

	// Defaults
	if maxLen <= 0 {
		maxLen = 120
	}
	if maxGap <= 0 {
		maxGap = 20
	}

	// 1. Collect all points
	for i := range *childrenMetas {
		child := &(*childrenMetas)[i]

		defaultSlaveID := uint8(child.Config.SlaveID)
		if defaultSlaveID == 0 {
			defaultSlaveID = 1
		}

		for j := range child.ParsedPoints {
			point := &child.ParsedPoints[j]

			sid := point.SlaveID
			if sid == 0 {
				sid = defaultSlaveID
			}
			point.SlaveID = sid

			fc := point.FunctionCode
			if fc == 0 {
				fc = 3
			}
			point.FunctionCode = fc

			interval := point.Interval
			if interval <= 0 {
				interval = 1000
			}

			key := autoGroupKey{SlaveID: sid, FunctionCode: fc, Interval: interval}
			pointMap[key] = append(pointMap[key], point)
		}
	}

	var newGroups []PollingGroupConfig

	// 2. Process each group
	for key, points := range pointMap {
		if len(points) == 0 {
			continue
		}

		sort.Slice(points, func(i, j int) bool {
			return points[i].Address < points[j].Address
		})

		var currentStart uint16
		var currentEnd uint16
		var chunkPoints []*ChildPoint

		flushChunk := func() {
			if len(chunkPoints) == 0 {
				return
			}
			length := currentEnd - currentStart

			groupName := fmt.Sprintf("Auto_%d_%d_%d_%d_%d", key.SlaveID, key.FunctionCode, key.Interval, currentStart, length)

			// Interval is already known from key
			interval := key.Interval

			pg := PollingGroupConfig{
				Name:         groupName,
				Enable:       true,
				SlaveID:      key.SlaveID,
				FunctionCode: key.FunctionCode,
				StartAddress: currentStart,
				Length:       length,
				Interval:     interval,
				Description:  "Auto Generated",
			}
			newGroups = append(newGroups, pg)

			for _, pt := range chunkPoints {
				pt.PollingGroup = groupName
				if key.FunctionCode == 3 || key.FunctionCode == 4 {
					pt.Offset = int(pt.Address - currentStart)
				} else {
					pt.Offset = int(pt.Address - currentStart)
				}
			}

			chunkPoints = nil
		}

		for _, pt := range points {
			var ptLen uint16
			if key.FunctionCode == 1 || key.FunctionCode == 2 {
				ptLen = 1
			} else {
				bytes := codec.GetTypeLength(pt.DataType)
				ptLen = uint16((bytes + 1) / 2)
				if ptLen == 0 {
					ptLen = 1
				}
			}

			if chunkPoints == nil {
				currentStart = pt.Address
				currentEnd = pt.Address + ptLen
				chunkPoints = append(chunkPoints, pt)
				continue
			}

			// Force separate requests for Coils to avoid bit packing issues in codec
			// Optimization: Allow grouping for Coils/Inputs (FC 1/2) to improve read efficiency
			// We will handle the bit extraction in dispatchData
			/*
				if key.FunctionCode == 1 || key.FunctionCode == 2 {
					flushChunk()
					currentStart = pt.Address
					currentEnd = pt.Address + ptLen
					chunkPoints = append(chunkPoints, pt)
					continue
				}
			*/

			if pt.Address > currentEnd+uint16(maxGap) {
				flushChunk()
				currentStart = pt.Address
				currentEnd = pt.Address + ptLen
				chunkPoints = append(chunkPoints, pt)
				continue
			}

			newEnd := pt.Address + ptLen
			if newEnd < currentEnd {
				newEnd = currentEnd
			}

			newLen := newEnd - currentStart
			if newLen > uint16(maxLen) {
				flushChunk()
				currentStart = pt.Address
				currentEnd = pt.Address + ptLen
				chunkPoints = append(chunkPoints, pt)
				continue
			}

			chunkPoints = append(chunkPoints, pt)
			currentEnd = newEnd
		}
		flushChunk()
	}

	return newGroups, nil
}

// dispatchData parses the raw Modbus data and maps it to devices (children and self)
func (p *ModbusPlugin) dispatchData(data []byte, children []ChildMeta, group PollingGroupConfig) (map[string]map[string]interface{}, map[string]bool) {
	results := make(map[string]map[string]interface{})
	onlineStates := make(map[string]bool)

	for _, child := range children {
		childValues := make(map[string]interface{})
		pointsFound := 0

		// Find points belonging to this group
		for _, point := range child.ParsedPoints {
			if point.PollingGroup == group.Name {
				pointsFound++
				// Create a copy of PointConfig to modify Offset without affecting original
				pc := point.PointConfig

				// Adjust Offset based on Function Code
				// Modbus Register (FC 3, 4) is 2 bytes per offset
				_ = pc.Offset // originalOffset

				// Optimization Variables for Coils (FC 1/2)
				var coilByteIndex int
				var coilBitIndex int

				if group.FunctionCode == 3 || group.FunctionCode == 4 {
					pc.Offset = point.PointConfig.Offset * 2
				} else if group.FunctionCode == 1 || group.FunctionCode == 2 {
					// Optimization: Handle packed bits for Coils/Inputs
					// point.PointConfig.Offset is the coil index relative to the group start
					coilByteIndex = point.PointConfig.Offset / 8
					coilBitIndex = point.PointConfig.Offset % 8
					pc.Offset = coilByteIndex
				}

				// Debug Log for Offset 2 Issue
				if pc.Offset == 4 || point.Name == "offset_2" || point.PointConfig.Offset == 2 {
					// p.Logger.Info("Decoding Point",
					// zap.String("point", point.Name),
					// zap.Int("original_offset", originalOffset),
					// zap.Int("adjusted_offset", pc.Offset),
					// zap.String("data_type", pc.DataType),
					// zap.Int("data_len", len(data)),
					// zap.String("data_hex", fmt.Sprintf("%x", data)),
					// )
				}

				// Decode (V2 or V1)
				var val interface{}
				var err error

				if group.FunctionCode == 1 || group.FunctionCode == 2 {
					// Fast path for Coils/Inputs (FC 1/2) - Handle packed bits directly
					if coilByteIndex < len(data) {
						val = (data[coilByteIndex]>>coilBitIndex)&0x01 == 1
					} else {
						err = fmt.Errorf("data index out of range for coil: byteIndex %d, len %d", coilByteIndex, len(data))
					}
				} else if point.ExtractRule.DataType != "" {
					rule := point.ExtractRule
					// Sync RegisterIndex with Group Offset
					// (Offset is calculated relative to Group Start in Registers)
					if rule.RegisterIndex == 0 && point.PointConfig.Offset > 0 {
						rule.RegisterIndex = point.PointConfig.Offset
					}

					// For Coils (FC1) and Discrete Inputs (FC2), both RegisterIndex and BitIndex are always 0
					// because we force separate requests for each point in these function codes.
					if group.FunctionCode == 1 || group.FunctionCode == 2 {
						rule.RegisterIndex = 0
						rule.BitIndex = 0
					}

					// Special handling for bool type with ReadExpr:
					// If it's a register-based function code (3 or 4), we extract the whole register first,
					// apply the expression, and THEN extract the bit in a later step.
					// This allows expressions like "x + 2" to work on the register value before bit extraction.
					isRegisterBoolWithExpr := (group.FunctionCode == 3 || group.FunctionCode == 4) &&
						rule.DataType == "bool" &&
						point.ReadExpr != ""

					if isRegisterBoolWithExpr {
						rule.DataType = "uint16"
					}

					val, err = codec.Extract(data, rule)
					// p.Logger.Debug("Modbus V2 Extract",
					// zap.String("device", child.Meta.DeviceCode),
					// zap.String("point", point.Name),
					// zap.Any("rule", rule),
					// zap.Any("rawVal", val),
					// zap.Error(err),
					// )
				} else {
					// V1 Legacy Decode
					val, err = codec.Decode(data, pc)
					// p.Logger.Debug("Modbus V1 Decode",
					// zap.String("device", child.Meta.DeviceCode),
					// zap.String("point", point.Name),
					// zap.Any("pc", pc),
					// zap.Any("rawVal", val),
					// zap.Error(err),
					// )
				}

				// V2 Expression Evaluation
				if err == nil && point.ReadExpr != "" {
					val, err = codec.EvaluateRead(point.ReadExpr, val)
					// p.Logger.Debug("Modbus Expression Eval",
					// zap.String("device", child.Meta.DeviceCode),
					// zap.String("point", point.Name),
					// zap.String("expr", point.ReadExpr),
					// zap.Any("before", oldVal),
					// zap.Any("after", val),
					// zap.Error(err),
					// )
				}

				// V2 Precision Rounding
				if err == nil && point.Precision > 0 {
					if fv, ok := toFloat(val); ok {
						val = round(fv, point.Precision)
					}
				}

				// Ensure Bool Type Consistency
				// If DataType is bool, ensure the result is a boolean value
				if err == nil && (point.ExtractRule.DataType == "bool" || point.PointConfig.DataType == "bool") {
					if _, ok := val.(bool); !ok {
						// If it's a number (e.g. from expression), convert to bool.
						// If it was a register-based bool with expression, we extract the specific bit.
						// Otherwise, we just treat 0 as false and non-zero as true.
						if vf, ok := toFloat(val); ok {
							isRegisterBoolWithExpr := (group.FunctionCode == 3 || group.FunctionCode == 4) &&
								point.ExtractRule.DataType == "bool" &&
								point.ReadExpr != ""

							if isRegisterBoolWithExpr {
								bitIndex := point.ExtractRule.BitIndex
								val = (uint64(vf) & (1 << uint(bitIndex))) != 0
								// p.Logger.Debug("Modbus Post-Expr Bit Extraction",
								// zap.String("device", child.Meta.DeviceCode),
								// zap.String("point", point.Name),
								// zap.Int("bitIndex", bitIndex),
								// zap.Any("before", oldVal),
								// zap.Any("after", val),
								// )
							} else {
								val = (vf != 0)
							}
						}
						// p.Logger.Debug("Modbus Bool Consistency Fix",
						// zap.String("device", child.Meta.DeviceCode),
						// zap.String("point", point.Name),
						// zap.Any("before", oldVal),
						// zap.Any("after", val),
						// )
					}
				}

				if err == nil {
					// p.Logger.Debug("Modbus Read Response",
					// zap.String("device", child.Meta.DeviceCode),
					// zap.String("group", group.Name),
					// zap.Int("fc", group.FunctionCode),
					// zap.String("data_hex", fmt.Sprintf("%x", data)),
					// )

					// 2. Update Cache
					p.updateDeviceDataCache(child.Meta.DeviceCode, point.Name, val)

					// 3. Check IsProperty (Default true)
					isProp := true
					if point.IsProperty != nil {
						isProp = *point.IsProperty
					}

					if isProp {
						childValues[point.Name] = val
					}
				}
			}
		}

		if len(childValues) > 0 {
			results[child.Meta.DeviceCode] = childValues
		} else if pointsFound > 0 {
			// It is possible that points were found but no values were added (e.g. if they are not properties, and only used for online check or events)
			// But we still want to return result if we found something?
			// Actually dispatchData returns `results` which is values to REPORT.
			// If we only have online status check or event check, we might not have values to report to TSL.
			// That is fine.
		}

		// 3. Evaluate Online Status
		state := p.getOrCreateDeviceState(child.Meta.DeviceCode)

		// DEBUG: Log Online Rule state
		// strategy := "nil"
		// if child.OnlineRule != nil {
		// 	strategy = child.OnlineRule.Strategy
		// }
		// p.Logger.Info("Evaluating Online Status",
		// 	zap.String("device", child.Meta.DeviceCode),
		// 	zap.String("strategy", strategy),
		// 	zap.Any("rule", child.OnlineRule),
		// )

		// 1. Determine "Raw" Status based on Strategy
		rawStatus := false
		if child.OnlineRule == nil || child.OnlineRule.Strategy == "" || child.OnlineRule.Strategy == "communication" {
			// Strategy: Communication (default)
			// Since we are here, it means communication is successful (err == nil)
			rawStatus = true
		} else if child.OnlineRule.Strategy == "custom_point" {
			// Strategy: Custom Point Rule
			val := p.getDeviceData(child.Meta.DeviceCode, child.OnlineRule.Point)

			// DEBUG LOG: Diagnose custom point strategy
			// p.Logger.Info("DEBUG: Custom Point Strategy Check",
			// zap.String("device", child.Meta.DeviceCode),
			// zap.String("point", child.OnlineRule.Point),
			// zap.Any("val_raw", val),
			// )

			if val != nil {
				if vFloat, ok := toFloat(val); ok {
					rVal := child.OnlineRule.Value

					// DEBUG LOG: Compare values
					// p.Logger.Info("DEBUG: Custom Point Value Compare",
					// zap.Float64("vFloat", vFloat),
					// zap.Float64("rVal", rVal),
					// zap.String("operator", child.OnlineRule.Operator),
					// )

					switch child.OnlineRule.Operator {
					case "==":
						rawStatus = (vFloat == rVal)
					case "!=":
						rawStatus = (vFloat != rVal)
					case ">":
						rawStatus = (vFloat > rVal)
					case "bit_and":
						rawStatus = (int64(vFloat) & int64(rVal)) != 0
					default:
						rawStatus = (vFloat == rVal)
					}
				} else {
					// ADDED: Log if toFloat failed
					// p.Logger.Warn("DEBUG: Custom Point toFloat Failed",
					// zap.Any("val", val),
					// zap.String("type", fmt.Sprintf("%T", val)))
				}
			} else {
				// ADDED: Log if val is nil
				// p.Logger.Warn("DEBUG: Custom Point Value is Nil (Not Found in Cache)")
			}
		} else if child.OnlineRule.Strategy == "value_change" {
			// Strategy: Value Activity Monitor
			// 只要数值在变，就视为在线。如果数值不变且超时，视为离线。
			// 这里先默认给 true，具体的超时判断在后面做
			rawStatus = true
		}

		// 2. Apply Value Change Logic (Only for 'value_change' strategy)
		if child.OnlineRule != nil && child.OnlineRule.Strategy == "value_change" {
			hbVal := p.getDeviceData(child.Meta.DeviceCode, child.OnlineRule.MonitorPoint)
			if hbVal != nil {
				// Initialize LastChangeValue if it's the first time
				if state.LastChangeValue == nil {
					state.LastChangeValue = hbVal
					state.LastChangeTime = time.Now()
					// First read is always "alive" because we just got data
					rawStatus = true
					// p.Logger.Info("ValueMonitor Init",
					// zap.String("dev", child.Meta.DeviceCode),
					// zap.Any("val", hbVal))
				} else {
					// Use string comparison to avoid type mismatch
					currentValStr := fmt.Sprintf("%v", hbVal)
					lastValStr := fmt.Sprintf("%v", state.LastChangeValue)

					// Default interval
					interval := 60
					if child.OnlineRule.MaxUnchangedInterval > 0 {
						interval = child.OnlineRule.MaxUnchangedInterval
					}
					timeSince := time.Since(state.LastChangeTime).Seconds()

					// p.Logger.Info("ValueMonitor Check",
					// zap.String("dev", child.Meta.DeviceCode),
					// zap.String("curr", currentValStr),
					// zap.String("last", lastValStr),
					// zap.Float64("since", timeSince),
					// zap.Int("limit", interval))

					if currentValStr != lastValStr {
						// Value changed, update timestamp
						state.LastChangeValue = hbVal
						state.LastChangeTime = time.Now()
						rawStatus = true // Alive
						// p.Logger.Info("ValueMonitor Changed", zap.String("dev", child.Meta.DeviceCode))
					} else {
						// Value same, check time
						if timeSince > float64(interval) {
							rawStatus = false // Timeout -> Offline
							// p.Logger.Info("ValueMonitor Timeout", zap.String("dev", child.Meta.DeviceCode))
						} else {
							rawStatus = true // Within interval -> Online
						}
					}
				}
			} else {
				// Can't read monitor point -> Offline
				rawStatus = false
				// p.Logger.Warn("ValueMonitor NoData", zap.String("dev", child.Meta.DeviceCode))
			}
		}

		// 3. Apply Debounce (Universal for all strategies)
		// For Online Debounce: We override it to 1 (Immediate Online) as per user request.
		// For Offline Debounce: We use the configured value (default 2).
		if rawStatus {
			state.ConsecutiveFailure = 0
			state.ConsecutiveSuccess++
			threshold := 1
			// Hardcoded to 1 as requested: "Online Debounce not needed"
			if state.ConsecutiveSuccess >= threshold {
				state.CurrentStatus = true
			}
		} else {
			state.ConsecutiveSuccess = 0
			state.ConsecutiveFailure++
			threshold := 2 // Default offline debounce
			if child.OnlineRule != nil && child.OnlineRule.OfflineDebounce > 0 {
				threshold = child.OnlineRule.OfflineDebounce
			}
			if state.ConsecutiveFailure >= threshold {
				state.CurrentStatus = false
			}
		}

		// Apply Heartbeat (Only if currently Online and Heartbeat enabled)
		// Deprecated: Heartbeat logic is now integrated into 'value_change' strategy.
		// The old 'EnableValueCheck' logic is removed to avoid conflict.

		onlineStates[child.Meta.DeviceCode] = state.CurrentStatus

		// 4. Evaluate Events
		p.evaluateEvents(child)
	}

	return results, onlineStates
}

// runPollingLoop executes the polling logic in a goroutine
func (p *ModbusPlugin) runPollingLoop(device types.DeviceMeta, devConfig DeviceConfig, group PollingGroupConfig, childrenMetas []ChildMeta, stopChan <-chan struct{}) {
	ticker := time.NewTicker(time.Duration(group.Interval) * time.Millisecond)
	defer ticker.Stop()

	// Initial run? Maybe configurable. For now, strictly follow ticker.
	// Or run once immediately?
	// Ticker waits for first tick. original code had Start() which ran immediately?
	// gotask usually runs immediately.
	// Let's run once immediately in a goroutine to avoid blocking start?
	// Or just wait. Ticker behavior is fine.

	for {
		select {
		case <-stopChan:
			return
		case <-ticker.C:
			// A. Execute Modbus Read
			data, err := p.readRegisters(devConfig, group)
			if err != nil {
				// Gateway Offline Logic
				statusMeta := types.DeviceMeta{DeviceCode: device.DeviceCode}
				p.Ctx.ReportStatus(statusMeta, "offline")

				// Mark sub-devices offline
				for _, child := range childrenMetas {
					state := p.getOrCreateDeviceState(child.Meta.DeviceCode)
					state.ConsecutiveSuccess = 0
					state.ConsecutiveFailure++

					threshold := 2
					if child.OnlineRule != nil && child.OnlineRule.OfflineDebounce > 0 {
						threshold = child.OnlineRule.OfflineDebounce
					}

					if state.ConsecutiveFailure >= threshold {
						state.CurrentStatus = false
						childStatusMeta := types.DeviceMeta{DeviceCode: child.Meta.DeviceCode}
						p.Ctx.ReportStatus(childStatusMeta, "offline")
					}
				}
				// p.Logger.Warn("Modbus Read Error", zap.String("device", device.DeviceCode), zap.Error(err))
				continue
			}

			// Gateway Online (Default Strategy)
			gatewayHasPoints := false
			if _, ok := device.Extras["points"]; ok {
				gatewayHasPoints = true
			}

			useDefaultStrategy := true
			if devConfig.OnlineRule != nil && devConfig.OnlineRule.Strategy != "" && devConfig.OnlineRule.Strategy != "communication" {
				useDefaultStrategy = false
			}

			if useDefaultStrategy && !gatewayHasPoints {
				statusMeta := types.DeviceMeta{DeviceCode: device.DeviceCode}
				p.Ctx.ReportStatus(statusMeta, "online")
			}

			// B. Dispatch Data
			dispatched, onlineStates := p.dispatchData(data, childrenMetas, group)

			// C. Report Data
			for deviceCode, values := range dispatched {
				var meta types.DeviceMeta
				found := false
				for _, child := range childrenMetas {
					if child.Meta.DeviceCode == deviceCode {
						meta = child.Meta
						found = true
						break
					}
				}
				if !found {
					continue
				}

				if len(values) > 0 {
					if err := p.reportData(meta, values); err != nil {
						// p.Logger.Error("Failed to report data", zap.String("device", deviceCode), zap.Error(err))
					}
				}
			}

			// D. Report Online Status
			for _, child := range childrenMetas {
				isChildInGroup := false
				if child.Meta.DeviceCode == device.DeviceCode {
					isChildInGroup = true
				} else {
					for _, pt := range child.ParsedPoints {
						if pt.PollingGroup == group.Name {
							isChildInGroup = true
							break
						}
					}
				}

				if !isChildInGroup {
					continue
				}

				deviceCode := child.Meta.DeviceCode
				onlineRule := child.OnlineRule

				shouldReportStatus := false
				finalStatus := false

				if onlineRule == nil || onlineRule.Strategy == "" || onlineRule.Strategy == "communication" {
					shouldReportStatus = true
					finalStatus = true
				} else if onlineRule.Strategy == "custom_point" || onlineRule.Strategy == "value_change" {
					if status, ok := onlineStates[deviceCode]; ok {
						shouldReportStatus = true
						finalStatus = status
					}
				}

				if shouldReportStatus {
					statusStr := "offline"
					if finalStatus {
						statusStr = "online"
					}

					// Mismatch check removed as we don't have access to DeviceManager state easily
					// We rely on ReportStatus to handle deduplication upstream if needed
					// But we still track local state for change detection

					state := p.getOrCreateDeviceState(deviceCode)
					statusChanged := state.CurrentStatus != finalStatus
					state.CurrentStatus = finalStatus

					shouldReport := statusChanged || time.Since(state.LastReportTime) > 30*time.Second

					if shouldReport {
						if statusChanged {
							// p.Logger.Info("Device Status Changed", zap.String("device", deviceCode), zap.String("status", statusStr))
						}

						reportMeta := types.DeviceMeta{DeviceCode: deviceCode}
						if err := p.Ctx.ReportStatus(reportMeta, statusStr); err == nil {
							state.LastReportTime = time.Now()
						}
					}
				}
			}
		}
	}
}

func (p *ModbusPlugin) readRegisters(cfg DeviceConfig, group PollingGroupConfig) ([]byte, error) {
	address := net.JoinHostPort(cfg.IP, strconv.Itoa(cfg.Port))
	conn, err := net.DialTimeout("tcp", address, time.Duration(cfg.TimeoutMS)*time.Millisecond)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Use the SlaveID defined in the Polling Group (Target Slave), fallback to Device Config
	targetSlaveID := group.SlaveID
	if targetSlaveID == 0 {
		targetSlaveID = cfg.SlaveID
	}
	if targetSlaveID == 0 {
		targetSlaveID = 1 // Default default
	}

	if cfg.ProtocolType == "Modbus-RTU over TCP" || cfg.ProtocolType == "RTU_OVER_TCP" {
		return p.readRegistersRTU(conn, targetSlaveID, group, cfg.TimeoutMS)
	}
	return p.readRegistersTCP(conn, targetSlaveID, group, cfg.TimeoutMS)
}

func (p *ModbusPlugin) readRegistersTCP(conn net.Conn, slaveID uint8, group PollingGroupConfig, timeout int) ([]byte, error) {
	// MBAP Header (7 bytes) + PDU (5 bytes for Read)
	// Transaction ID (2), Protocol (2), Length (2), Unit ID (1), Func Code (1), Start Addr (2), Quantity (2)
	req := make([]byte, 12)
	binary.BigEndian.PutUint16(req[0:], 0) // Trans ID
	binary.BigEndian.PutUint16(req[2:], 0) // Proto ID
	binary.BigEndian.PutUint16(req[4:], 6) // Length (UnitID + FC + Addr + Val)
	req[6] = slaveID
	req[7] = byte(group.FunctionCode)
	binary.BigEndian.PutUint16(req[8:], group.StartAddress)
	binary.BigEndian.PutUint16(req[10:], group.Length)

	conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	// Read Header
	header := make([]byte, 9) // MBAP(7) + Func(1) + ByteCount(1)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	// Check Error Response
	if header[7] > 0x80 {
		return nil, fmt.Errorf("modbus exception: %x", header[8])
	}

	byteCount := int(header[8])
	data := make([]byte, byteCount)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}

	return data, nil
}

func (p *ModbusPlugin) readRegistersRTU(conn net.Conn, slaveID uint8, group PollingGroupConfig, timeout int) ([]byte, error) {
	// RTU Frame: [SlaveID(1)][Func(1)][StartAddr(2)][Length(2)][CRC(2)]
	req := make([]byte, 8)
	req[0] = slaveID
	req[1] = byte(group.FunctionCode)
	binary.BigEndian.PutUint16(req[2:], group.StartAddress)
	binary.BigEndian.PutUint16(req[4:], group.Length)

	// CRC Calculation
	crc := codec.CalculateCRC16(req[:6])
	binary.LittleEndian.PutUint16(req[6:], crc)

	conn.SetDeadline(time.Now().Add(time.Duration(timeout) * time.Millisecond))
	if _, err := conn.Write(req); err != nil {
		return nil, err
	}

	// Read Response
	// RTU Response: [SlaveID(1)][Func(1)][ByteCount(1)][Data...][CRC(2)]
	// Read Header first: 3 bytes
	header := make([]byte, 3)
	if _, err := io.ReadFull(conn, header); err != nil {
		return nil, err
	}

	// Check Error Response
	if header[1] > 0x80 {
		// Read Error Code + CRC
		errBuf := make([]byte, 2)
		io.ReadFull(conn, errBuf)
		return nil, fmt.Errorf("modbus exception: %x", header[2])
	}

	byteCount := int(header[2])
	// Read Data + CRC
	totalLen := byteCount + 2
	buf := make([]byte, totalLen)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}

	// Verify CRC (Optional but recommended)
	// fullFrame = header + buf
	// ...

	return buf[:byteCount], nil
}

func (p *ModbusPlugin) writeSingleItem(cfg DeviceConfig, slaveID uint8, functionCode uint8, address uint16, valUint16 uint16) error {
	addressStr := net.JoinHostPort(cfg.IP, strconv.Itoa(cfg.Port))
	conn, err := net.DialTimeout("tcp", addressStr, time.Duration(cfg.TimeoutMS)*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Use target SlaveID (usually from Device Config or direct argument)
	targetSlaveID := slaveID
	if targetSlaveID == 0 {
		targetSlaveID = cfg.SlaveID
	}
	if targetSlaveID == 0 {
		targetSlaveID = 1
	}

	if cfg.ProtocolType == "Modbus-RTU over TCP" || cfg.ProtocolType == "RTU_OVER_TCP" {
		return p.writeSingleItemRTU(conn, targetSlaveID, functionCode, address, valUint16, cfg.TimeoutMS)
	}
	return p.writeSingleItemTCP(conn, targetSlaveID, functionCode, address, valUint16, cfg.TimeoutMS)
}

// Helper
func normalizeExtras(extras map[string]interface{}) {
	// Map camelCase to snake_case for known keys
	mappings := map[string]string{
		"protocolType":   "protocol_type",
		"slaveId":        "slave_id",
		"timeoutMs":      "timeout_ms",
		"pollingGroups":  "polling_groups",
		"host":           "ip",
		"collectionMode": "collection_mode",
		"timeout":        "timeout_ms",
		"onlineRule":     "online_rule",
		"OnlineRule":     "online_rule",
	}

	for camel, snake := range mappings {
		if val, ok := extras[camel]; ok {
			// Always overwrite to ensure source of truth (Schema/User Input) is used
			extras[snake] = val
		}
	}

	// Normalize Polling Groups
	if groups, ok := extras["polling_groups"].([]interface{}); ok {
		for _, g := range groups {
			if groupMap, ok := g.(map[string]interface{}); ok {
				normalizeGroup(groupMap)
				// Default Enable to true if missing
				if _, exists := groupMap["enable"]; !exists {
					groupMap["enable"] = true
				}
			}
		}
	}
}

func normalizeGroup(m map[string]interface{}) {
	mappings := map[string]string{
		"slaveId":      "slave_id",
		"functionCode": "function_code",
		"startAddress": "start_address",
	}
	for camel, snake := range mappings {
		if val, ok := m[camel]; ok {
			if _, exists := m[snake]; !exists {
				m[snake] = val
			}
		}
	}

	if val, ok := m["interval"]; ok {
		switch v := val.(type) {
		case string:
			intervalStr := strings.TrimSpace(v)
			if strings.HasPrefix(intervalStr, "@every ") {
				intervalStr = strings.TrimPrefix(intervalStr, "@every ")
			}
			if d, err := time.ParseDuration(intervalStr); err == nil {
				m["interval"] = int(d.Milliseconds())
			} else if n, err := strconv.Atoi(intervalStr); err == nil {
				if n > 0 && n < 1000 {
					m["interval"] = n * 1000
				} else {
					m["interval"] = n
				}
			}
		case float64:
			n := int(v)
			if n > 0 && n < 1000 {
				m["interval"] = n * 1000
			}
		case int:
			if v > 0 && v < 1000 {
				m["interval"] = v * 1000
			}
		case json.Number:
			if n64, err := v.Int64(); err == nil {
				n := int(n64)
				if n > 0 && n < 1000 {
					m["interval"] = n * 1000
				} else {
					m["interval"] = n
				}
			}
		}
	}
}

func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func normalizePoint(m map[string]interface{}) {
	mappings := map[string]string{
		"slaveId":      "slave_id",
		"functionCode": "function_code",
		"dataType":     "data_type",
		"byteOrder":    "byte_order",
		"pollingGroup": "polling_group",
		"isProperty":   "is_property",
		"startAddress": "address", // Sometimes UI sends startAddress
		"registerAddr": "address",
		// V2 Mappings
		"extractRule": "extract_rule",
		"readExpr":    "read_expr",
		"writeConfig": "write_config",
		// Nested V2 Mappings
		"registerIndex": "register_index",
		"bitIndex":      "bit_index",
		"writeExpr":     "write_expr",
	}
	for camel, snake := range mappings {
		if val, ok := m[camel]; ok {
			if _, exists := m[snake]; !exists {
				m[snake] = val
			}
		}
	}

	// Helper: Ensure ExtractRule inherits from root if present (for mixed V1/V2 usage)
	// If user sets bit_index (V2), they expect data_type to be inherited if not explicitly set in extract_rule
	if extractRule, ok := m["extract_rule"].(map[string]interface{}); ok {
		if _, hasType := extractRule["data_type"]; !hasType {
			if rootType, ok := m["data_type"]; ok {
				extractRule["data_type"] = rootType
			}
		}
		if _, hasOrder := extractRule["byte_order"]; !hasOrder {
			if rootOrder, ok := m["byte_order"]; ok {
				extractRule["byte_order"] = rootOrder
			}
		}
	}

	// Recursion for nested maps (e.g. write_config, extract_rule)
	for _, v := range m {
		if subMap, ok := v.(map[string]interface{}); ok {
			normalizePoint(subMap)
		}
	}
}

func parsePoints(raw interface{}) []ChildPoint {
	var points []ChildPoint
	if raw == nil {
		return points
	}

	// Case 1: Slice
	if list, ok := raw.([]interface{}); ok {
		// Normalize each item in list if it's a map
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				normalizePoint(m)
			}
		}
		b, _ := json.Marshal(list)
		json.Unmarshal(b, &points)
		return points
	}

	// Case 2: Map
	if m, ok := raw.(map[string]interface{}); ok {
		list := make([]interface{}, 0, len(m))
		for _, v := range m {
			if vm, ok := v.(map[string]interface{}); ok {
				normalizePoint(vm)
			}
			list = append(list, v)
		}
		b, _ := json.Marshal(list)
		json.Unmarshal(b, &points)
		return points
	}

	return points
}

// --- Event Engine & Data Cache Helpers ---

func (p *ModbusPlugin) updateDeviceDataCache(deviceCode string, pointName string, value interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.deviceDataCache == nil {
		p.deviceDataCache = make(map[string]map[string]interface{})
	}
	if p.deviceDataCache[deviceCode] == nil {
		p.deviceDataCache[deviceCode] = make(map[string]interface{})
	}
	p.deviceDataCache[deviceCode][pointName] = value
}

func (p *ModbusPlugin) getDeviceData(deviceCode string, pointName string) interface{} {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.deviceDataCache == nil {
		return nil
	}
	if deviceData, ok := p.deviceDataCache[deviceCode]; ok {
		return deviceData[pointName]
	}
	return nil
}

func (p *ModbusPlugin) evaluateEvents(child ChildMeta) {
	if len(child.Events) == 0 {
		return
	}

	// p.Logger.Debug("Evaluating Events", zap.String("device", child.Meta.DeviceCode), zap.Int("count", len(child.Events)))

	for _, event := range child.Events {
		// 1. Evaluate Triggers
		isTriggered := false

		// Use new Triggers array if available
		if len(event.Triggers) > 0 {
			// Determine Logic: Default to OR
			logic := event.TriggerLogic
			if logic == "" {
				logic = "or"
			}

			if logic == "and" {
				// AND Logic: Start true, one false fails all
				isTriggered = true
				for _, trigger := range event.Triggers {
					if !p.checkCondition(child.Meta.DeviceCode, trigger) {
						isTriggered = false
						break
					}
				}
			} else {
				// OR Logic: Start false, one true passes
				isTriggered = false
				for _, trigger := range event.Triggers {
					if p.checkCondition(child.Meta.DeviceCode, trigger) {
						isTriggered = true
						break
					}
				}
			}
		} else {
			// Fallback to legacy single Trigger
			isTriggered = p.checkCondition(child.Meta.DeviceCode, event.Trigger)
		}

		// 2. Evaluate Conditions
		// Only check conditions if triggered is true (optimization)
		if isTriggered && len(event.Conditions) > 0 {
			// Determine Logic: Default to AND
			logic := event.ConditionLogic
			if logic == "" {
				logic = "and"
			}

			if logic == "or" {
				// OR Logic: Start false, one true passes
				// However, if list is empty (handled by len check), result is technically false in OR mode,
				// but here we are gating an existing 'true'.
				// Actually, if conditions exist, we check them.

				passed := false
				for _, cond := range event.Conditions {
					if p.checkCondition(child.Meta.DeviceCode, cond) {
						passed = true
						break
					}
				}
				if !passed {
					isTriggered = false
				}
			} else {
				// AND Logic: Start true, one false fails all
				for _, cond := range event.Conditions {
					if !p.checkCondition(child.Meta.DeviceCode, cond) {
						isTriggered = false
						break
					}
				}
			}
		}

		// Edge Detection / State Tracking
		p.mu.Lock()
		if p.lastEventStates == nil {
			p.lastEventStates = make(map[string]map[string]bool)
		}
		if p.lastEventStates[child.Meta.DeviceCode] == nil {
			p.lastEventStates[child.Meta.DeviceCode] = make(map[string]bool)
		}
		if p.lastEventReportTimes == nil {
			p.lastEventReportTimes = make(map[string]map[string]time.Time)
		}
		if p.lastEventReportTimes[child.Meta.DeviceCode] == nil {
			p.lastEventReportTimes[child.Meta.DeviceCode] = make(map[string]time.Time)
		}

		lastState := p.lastEventStates[child.Meta.DeviceCode][event.Identifier]
		lastReportTime := p.lastEventReportTimes[child.Meta.DeviceCode][event.Identifier]

		// Update State immediately
		p.lastEventStates[child.Meta.DeviceCode][event.Identifier] = isTriggered
		p.mu.Unlock()

		// Determine effective interval
		interval := -1
		if event.ReportInterval != nil {
			interval = *event.ReportInterval
		}

		if isTriggered != lastState || isTriggered {
			// p.Logger.Debug("Evaluate Event Rule",
			// zap.String("device", child.Meta.DeviceCode),
			// zap.String("event", event.Identifier),
			// zap.Bool("triggered", isTriggered),
			// zap.Bool("last_state", lastState),
			// zap.Int("interval", interval),
			// )
		}

		shouldReport := false

		if isTriggered {
			if !lastState {
				// 1. Rising Edge: Always Report
				shouldReport = true
			} else {
				// 2. State maintained (True -> True)
				if interval == -1 {
					// -1: Rising Edge Only (Do nothing)
					shouldReport = false
				} else if interval == 0 {
					// 0: Always Report
					shouldReport = true
				} else if interval > 0 {
					// >0: Throttle (Check interval)
					if time.Since(lastReportTime).Seconds() >= float64(interval) {
						shouldReport = true
					}
				}
			}
		}

		if shouldReport {
			// Update LastReportTime
			p.mu.Lock()
			p.lastEventReportTimes[child.Meta.DeviceCode][event.Identifier] = time.Now()
			p.mu.Unlock()

			p.reportEvent(child.Meta, event)
		}
	}
}

func (p *ModbusPlugin) checkCondition(deviceCode string, condition EventTrigger) bool {
	if condition.Point == "" {
		return false
	}

	valRaw := p.getDeviceData(deviceCode, condition.Point)
	if valRaw == nil {
		// Only log if we are debugging specific issues, otherwise it might be too noisy?
		// But user asked for logs.
		// p.Logger.Debug("checkCondition: Point data not found",
		// zap.String("device", deviceCode),
		// zap.String("point", condition.Point))
		return false
	}

	// Convert to float64 for comparison
	val, ok := toFloat(valRaw)
	if !ok {
		// p.Logger.Warn("checkCondition: Failed to convert value to float",
		// zap.String("device", deviceCode),
		// zap.String("point", condition.Point),
		// zap.Any("val_raw", valRaw))
		return false
	}

	result := false
	switch condition.Operator {
	case ">":
		result = val > condition.Value
	case "<":
		result = val < condition.Value
	case "==":
		result = val == condition.Value
	case "!=":
		result = val != condition.Value
	case ">=":
		result = val >= condition.Value
	case "<=":
		result = val <= condition.Value
	case "change":
		// TODO: Implement change detection
		result = false
	}

	if result {
		// p.Logger.Debug("checkCondition: Match",
		// zap.String("device", deviceCode),
		// zap.String("point", condition.Point),
		// zap.Float64("val", val),
		// zap.String("op", condition.Operator),
		// zap.Float64("target", condition.Value))
	}

	return result
}

// reportData sends the parsed data to the core for processing
func (p *ModbusPlugin) reportData(device types.DeviceMeta, values map[string]interface{}) error {
	return p.Ctx.ReportBatchProperties(device, values)
}

// --- Minimal Modbus TCP Client ---

// ... (existing readRegisters and other client code) ...
// ... (existing helper functions) ...

func (p *ModbusPlugin) reportEvent(device types.DeviceMeta, event EventRule) {
	// 1. Collect Parameters
	params := make(map[string]interface{})
	for paramID, pointName := range event.Params {
		val := p.getDeviceData(device.DeviceCode, pointName)
		if val != nil {
			params[paramID] = val
		}
	}

	// p.Logger.Info("Reporting Event",
	// zap.String("device", device.DeviceCode),
	// zap.String("event", event.Identifier),
	// zap.Any("params", params))

	if err := p.Ctx.ReportEvent(device, event.Identifier, params); err != nil {
		// p.Logger.Error("Failed to report event",
		// zap.String("device", device.DeviceCode),
		// zap.String("event", event.Identifier),
		// zap.Error(err))
	} else {
		// p.Logger.Info("Event reported successfully",
		// zap.String("device", device.DeviceCode),
		// zap.String("event", event.Identifier))
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint16:
		return float64(val), true
	case bool:
		if val {
			return 1, true
		}
		return 0, true
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

func round(val float64, precision int) float64 {
	p := math.Pow10(precision)
	return math.Round(val*p) / p
}
