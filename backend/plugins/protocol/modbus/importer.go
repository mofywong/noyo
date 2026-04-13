package modbus

import (
	"context"
	"fmt"
	"strings"

	"noyo/core"
	"noyo/core/importer"
	"noyo/core/types"
	// "go.uber.org/zap"
)

// -----------------------------------------------------------------------------
// Data Structures (Excel Rows)
// -----------------------------------------------------------------------------

type DeviceRow struct {
	Code           string `import:"code"`
	Name           string `import:"name"`
	ProductCode    string `import:"product_code"`
	ParentCode     string `import:"parent_code"`
	Enabled        string `import:"enabled"`
	ProtocolType   string `import:"protocol_type"`
	IP             string `import:"ip"`
	Port           string `import:"port"`
	SlaveID        string `import:"slave_id"`
	CollectionMode string `import:"collection_mode"`
	Timeout        string `import:"timeout_ms"`
	MaxGroupLength string `import:"max_group_length"`
	MaxAddressGap  string `import:"max_address_gap"`
}

type PollingGroupRow struct {
	DeviceCode   string `import:"device_code"`
	Name         string `import:"name"`
	Enabled      string `import:"enabled"`
	SlaveID      string `import:"slave_id"`
	FunctionCode string `import:"function_code"`
	StartAddress string `import:"start_address"`
	Length       string `import:"length"`
	Interval     string `import:"interval"`
}

type ManualPointRow struct {
	DeviceCode        string `import:"device_code"`
	Name              string `import:"display_name"`
	Identifier        string `import:"identifier"`
	DataType          string `import:"data_type"`
	ReadExpr          string `import:"read_expr"`
	Precision         string `import:"precision"`
	BitIndex          string `import:"bit_index"`
	PollingGroup      string `import:"polling_group"`
	Offset            string `import:"offset"`
	ByteOrder         string `import:"byte_order"`
	EnableWrite       string `import:"enable_write"`
	WriteFunctionCode string `import:"write_function_code"`
	WriteMode         string `import:"write_mode"`
	WriteExpr         string `import:"write_expr"`
	WriteAddress      string `import:"write_address"`
	WriteSlaveID      string `import:"write_slave_id"`
}

type DirectPointRow struct {
	DeviceCode        string `import:"device_code"`
	Name              string `import:"display_name"`
	Identifier        string `import:"identifier"`
	SlaveID           string `import:"slave_id"`
	FunctionCode      string `import:"function_code"`
	Address           string `import:"address"`
	Interval          string `import:"interval"`
	DataType          string `import:"data_type"`
	ReadExpr          string `import:"read_expr"`
	Precision         string `import:"precision"`
	BitIndex          string `import:"bit_index"`
	ByteOrder         string `import:"byte_order"`
	EnableWrite       string `import:"enable_write"`
	WriteFunctionCode string `import:"write_function_code"`
	WriteMode         string `import:"write_mode"`
	WriteExpr         string `import:"write_expr"`
	WriteAddress      string `import:"write_address"`
	WriteSlaveID      string `import:"write_slave_id"`
}

type EventRuleRow struct {
	DeviceCode     string `import:"device_code"`
	Name           string `import:"name"`
	Identifier     string `import:"identifier"`
	TriggerLogic   string `import:"trigger_logic"`
	Triggers       string `import:"triggers"`
	ConditionLogic string `import:"condition_logic"`
	Conditions     string `import:"conditions"`
	ReportInterval string `import:"report_interval"`
	Params         string `import:"params"`
}

type OnlineRuleRow struct {
	DeviceCode           string `import:"device_code"`
	Strategy             string `import:"strategy"`
	Point                string `import:"point"`
	Operator             string `import:"operator"`
	Value                string `import:"value"`
	OfflineDebounce      string `import:"offline_debounce"`
	MaxUnchangedInterval string `import:"max_unchanged_interval"`
}

// -----------------------------------------------------------------------------
// Plugin Implementation
// -----------------------------------------------------------------------------

// GetImportTemplateLayout returns the 6-sheet Excel structure
func (p *ModbusPlugin) GetImportTemplateLayout(lang string) []importer.SheetLayout {
	isEn := lang == "en"

	// Helper for header text
	h := func(zh, en string) string {
		if isEn {
			return en
		}
		return zh
	}

	return []importer.SheetLayout{
		{
			Name: "Devices",
			Columns: []importer.ColumnMeta{
				{Key: "code", Header: h("设备编码", "Device Code"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "unique"}},
				{Key: "name", Header: h("设备名称", "Device Name"), Width: 20, Style: "required"},
				{Key: "product_code", Header: h("所属产品", "Product"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "provider", ProviderKey: "target_products"}},
				{Key: "enabled", Header: h("是否启用", "Enabled"), Width: 10, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"是", "否"}}},
				{Key: "parent_code", Header: h("父设备", "Parent Device"), Width: 20, Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Devices", RefColumnKey: "code", DisplayFormat: "name_code"}},
				{Key: "protocol_type", Header: h("协议类型", "Protocol Type"), Width: 20, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"Modbus-TCP", "Modbus-RTU-Over-TCP"}}},
				{Key: "ip", Header: h("IP地址", "IP Address"), Width: 20},
				{Key: "port", Header: h("端口", "Port"), Width: 10},
				{Key: "slave_id", Header: h("从站号", "Slave ID"), Width: 15},
				{Key: "collection_mode", Header: h("采集策略", "Collection Mode"), Width: 20, Validation: &importer.ValidationMeta{Type: "list", Options: []string{h("手动分组", "manual"), h("点位直连", "auto")}}},
				{Key: "timeout_ms", Header: h("超时时间(ms)", "Timeout(ms)"), Width: 20},
				{Key: "max_group_length", Header: h("最大分组长度", "Max Group Length"), Width: 20},
				{Key: "max_address_gap", Header: h("最大地址间隙", "Max Address Gap"), Width: 20},
			},
		},
		{
			Name: "Polling Groups",
			Columns: []importer.ColumnMeta{
				{Key: "device_code", Header: h("设备", "Device"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Devices", RefColumnKey: "code", DisplayFormat: "name_code"}},
				{Key: "name", Header: h("分组名称", "Group Name"), Width: 20, Style: "required"},
				{Key: "enabled", Header: h("是否启用", "Enabled"), Width: 10, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"是", "否"}}},
				{Key: "slave_id", Header: h("从站号", "Slave ID"), Width: 10, Style: "required"},
				{Key: "function_code", Header: h("功能码", "Function Code"), Width: 10, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"01", "02", "03", "04"}}},
				{Key: "start_address", Header: h("起始地址", "Start Address"), Width: 15, Style: "required"},
				{Key: "length", Header: h("长度", "Length"), Width: 10, Style: "required"},
				{Key: "interval", Header: h("采集间隔(ms)", "Interval(ms)"), Width: 15, Style: "required"},
			},
		},
		{
			Name: "Points (Manual)",
			Columns: []importer.ColumnMeta{
				{Key: "device_code", Header: h("设备", "Device"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Devices", RefColumnKey: "code", DisplayFormat: "name_code"}},
				{Key: "display_name", Header: h("点位名称", "Point Name"), Width: 20, Style: "required"},
				{Key: "identifier", Header: h("标识符", "Identifier"), Width: 20, Style: "required"},
				{Key: "polling_group", Header: h("所属分组", "Polling Group"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Polling Groups", RefColumnKey: "name"}},
				{Key: "offset", Header: h("偏移量", "Offset"), Width: 10, Style: "required"},
				{Key: "data_type", Header: h("原始数据类型", "Data Type"), Width: 15, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"int16", "uint16", "int32", "uint32", "float32", "float64", "bool"}}},
				{Key: "byte_order", Header: h("字节序", "Byte Order"), Width: 15, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"ABCD", "BADC", "CDAB", "DCBA"}}},
				{Key: "read_expr", Header: h("读取表达式", "Read Expression"), Width: 20},
				{Key: "precision", Header: h("精度", "Precision"), Width: 10},
				{Key: "bit_index", Header: h("位索引", "Bit Index"), Width: 10},
				{Key: "enable_write", Header: h("启用写入", "Enable Write"), Width: 10, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"是", "否"}}},
				{Key: "write_mode", Header: h("写入模式", "Write Mode"), Width: 15, Validation: &importer.ValidationMeta{
					Type:    "list",
					Options: []string{h("读写同址", "SameAsRead"), h("自定义", "Custom")},
				}},
				{Key: "write_expr", Header: h("写入表达式", "Write Expression"), Width: 20},
				{Key: "write_function_code", Header: h("写入功能码", "Write Function"), Width: 15, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"05", "06", "15", "16"}}},
				{Key: "write_address", Header: h("写入地址", "Write Address"), Width: 15},
				{Key: "write_slave_id", Header: h("写入从站号", "Write Slave ID"), Width: 20},
			},
		},
		{
			Name: "Points (Direct)",
			Columns: []importer.ColumnMeta{
				{Key: "device_code", Header: h("设备", "Device"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Devices", RefColumnKey: "code", DisplayFormat: "name_code"}},
				{Key: "display_name", Header: h("点位名称", "Name"), Width: 20, Style: "required"},
				{Key: "identifier", Header: h("标识符", "Identifier"), Width: 20, Style: "required"},
				{Key: "slave_id", Header: h("从站号", "Slave ID"), Width: 10, Style: "required"},
				{Key: "function_code", Header: h("功能码", "Function Code"), Width: 15, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"01", "02", "03", "04"}}},
				{Key: "address", Header: h("地址", "Address"), Width: 15, Style: "required"},
				{Key: "interval", Header: h("采集频率(ms)", "Interval(ms)"), Width: 15},
				{Key: "data_type", Header: h("原始数据类型", "Data Type"), Width: 15, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"int16", "uint16", "int32", "uint32", "float32", "bool"}}},
				{Key: "byte_order", Header: h("字节序", "Byte Order"), Width: 15, Style: "required", Validation: &importer.ValidationMeta{Type: "list", Options: []string{"ABCD", "BADC", "CDAB", "DCBA"}}},
				{Key: "read_expr", Header: h("读取表达式", "Read Expression"), Width: 20},
				{Key: "precision", Header: h("精度", "Precision"), Width: 10},
				{Key: "bit_index", Header: h("位索引", "Bit Index"), Width: 10},
				{Key: "enable_write", Header: h("启用写入", "Enable Write"), Width: 15, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"是", "否"}}},
				{Key: "write_mode", Header: h("写入模式", "Write Mode"), Width: 15, Validation: &importer.ValidationMeta{
					Type:    "list",
					Options: []string{h("读写同址", "SameAsRead"), h("自定义", "Custom")},
				}},
				{Key: "write_expr", Header: h("写入表达式", "Write Expression"), Width: 20},
				{Key: "write_function_code", Header: h("写入功能码", "Write Function Code"), Width: 15, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"05", "06", "15", "16"}}},
				{Key: "write_address", Header: h("写入地址", "Write Address"), Width: 15},
				{Key: "write_slave_id", Header: h("写入从站号", "Write Slave ID"), Width: 20},
			},
		},
		{
			Name: "Event Rules",
			Columns: []importer.ColumnMeta{
				{Key: "device_code", Header: h("设备编码", "Device Code"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Devices", RefColumnKey: "code", DisplayFormat: "name_code"}},
				{Key: "name", Header: h("规则名称", "Rule Name"), Width: 20, Style: "required"},
				{Key: "identifier", Header: h("规则标识", "Rule Identifier"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "unique"}},
				{Key: "trigger_logic", Header: h("触发逻辑", "Trigger Logic"), Width: 10, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"OR", "AND"}}},
				{Key: "triggers", Header: h("触发条件", "Triggers"), Width: 40, Style: "wrap"},
				{Key: "condition_logic", Header: h("前提逻辑", "Condition Logic"), Width: 10, Validation: &importer.ValidationMeta{Type: "list", Options: []string{"AND", "OR"}}},
				{Key: "conditions", Header: h("前提条件", "Conditions"), Width: 40, Style: "wrap"},
				{Key: "report_interval", Header: h("报警间隔(秒)", "Report Interval(s)"), Width: 15},
				{Key: "params", Header: h("输出参数", "Output Params"), Width: 30, Style: "wrap"},
			},
		},
		{
			Name: "Online Rules",
			Columns: []importer.ColumnMeta{
				{Key: "device_code", Header: h("设备", "Device"), Width: 20, Style: "required", Validation: &importer.ValidationMeta{Type: "reference", RefSheetName: "Devices", RefColumnKey: "code", DisplayFormat: "name_code"}},
				{Key: "strategy", Header: h("判定策略", "Strategy"), Width: 20, Validation: &importer.ValidationMeta{
					Type: "list",
					Options: []string{
						h("基于通信状态", "Communication Based"),
						h("基于数值活跃度", "Value Activity Monitor"),
						h("基于自定义规则", "Custom Point Rule"),
					},
				}},
				{Key: "point", Header: h("关联点位", "Point"), Width: 20},
				{Key: "operator", Header: h("运算符", "Operator"), Width: 10, Validation: &importer.ValidationMeta{Type: "list", Options: []string{">", "<", "=", ">=", "<="}}},
				{Key: "value", Header: h("阈值", "Value"), Width: 15},
				{Key: "offline_debounce", Header: h("离线防抖", "Offline Debounce"), Width: 10},
				{Key: "max_unchanged_interval", Header: h("最大无变化时间(s)", "Max Unchanged(s)"), Width: 20},
			},
		},
	}
}

// GetImportSampleData provides example data
func (p *ModbusPlugin) GetImportSampleData(products []core.ProductMeta) (*importer.ImportRawData, error) {
	data := make(importer.ImportRawData)

	// Sample Device
	data["Devices"] = []map[string]string{
		{
			"code":            "Device01",
			"name":            "测试设备",
			"product_code":    "ProductA",
			"enabled":         "是",
			"protocol_type":   "Modbus-TCP",
			"ip":              "192.168.1.100",
			"port":            "502",
			"slave_id":        "1",
			"collection_mode": "手动分组",
			"timeout_ms":      "2000",
		},
	}

	data["Polling Groups"] = []map[string]string{
		{
			"device_code":   "Device01",
			"name":          "Group1",
			"enabled":       "是",
			"slave_id":      "1",
			"function_code": "03",
			"start_address": "0",
			"length":        "10",
			"interval":      "1000",
		},
	}

	data["Points (Manual)"] = []map[string]string{
		{
			"device_code":   "Device01",
			"display_name":  "温度",
			"identifier":    "temp",
			"polling_group": "Group1",
			"offset":        "0",
			"data_type":     "float32",
			"byte_order":    "ABCD",
			"enable_write":  "否",
		},
		{
			"device_code":   "Device01",
			"display_name":  "湿度",
			"identifier":    "humid",
			"polling_group": "Group1",
			"offset":        "2",
			"data_type":     "float32",
			"byte_order":    "ABCD",
			"enable_write":  "否",
		},
	}

	data["Points (Direct)"] = []map[string]string{}

	// Rich Example for Event Rules
	data["Event Rules"] = []map[string]string{
		{
			"device_code":     "Device01",
			"name":            "高温报警",
			"identifier":      "high_temp_alarm",
			"trigger_logic":   "OR",
			"triggers":        "temp > 50\nhumid >= 80", // Temp > 50 OR Humid >= 80
			"condition_logic": "AND",
			"conditions":      "status == 1\nmode != maintenance", // Status is 1 AND Mode is not maintenance
			"report_interval": "-1",                               // -1: Rising Edge Only, 0: Always, >0: Throttle
			"params":          "t=temp\nh=humid",                  // Bind t to temp, h to humid
		},
	}

	data["Online Rules"] = []map[string]string{}

	return &data, nil
}

// parseNameCodeFormat parses "Name (Code)" to "Code"
func parseNameCodeFormat(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if len(input) > 0 && input[len(input)-1] == ')' {
		for i := len(input) - 1; i >= 0; i-- {
			if input[i] == '(' {
				return strings.TrimSpace(input[i+1 : len(input)-1])
			}
		}
	}
	return input
}

func parseWriteMode(val string) string {
	val = strings.TrimSpace(val)
	if val == "自定义" || val == "Custom" {
		return "custom"
	}
	return "same_as_read"
}

func parseCollectionMode(val string) string {
	val = strings.TrimSpace(val)
	// Ensure "点位直连" maps to "auto" to match task_factory and frontend expectations
	if val == "点位直连" || val == "AutoReport" || val == "auto" {
		return "auto"
	}
	return "manual"
}

func parseProtocolType(val string) string {
	val = strings.TrimSpace(val)
	if val == "Modbus-RTU-Over-TCP" {
		return "Modbus-RTU over TCP"
	}
	return val
}

func parseOnlineStrategy(val string) string {
	val = strings.TrimSpace(val)
	// Map to internal codes: communication, value_change, custom_point

	// Communication
	if val == "communication" || val == "基于通信状态" || val == "Communication Based" {
		return "communication"
	}

	// Custom Point (formerly value_check)
	if val == "value_check" || val == "基于自定义规则" || val == "Custom Point Rule" || val == "custom_point" {
		return "custom_point"
	}

	// Value Change (formerly data_update)
	if val == "data_update" || val == "基于数值活跃度" || val == "Value Activity Monitor" || val == "value_change" {
		return "value_change"
	}

	// Default fallback
	return "communication"
}

// ResolveImportData parses and validates data
func (p *ModbusPlugin) ResolveImportData(ctx context.Context, raw importer.ImportRawData) (*importer.ImportResult, error) {
	// p.Logger.Info("ModbusPlugin: Starting ResolveImportData", zap.Int("sheet_count", len(raw)))

	result := &importer.ImportResult{}
	var devices []DeviceRow
	var groups []PollingGroupRow
	var manualPoints []ManualPointRow
	var directPoints []DirectPointRow
	var eventRules []EventRuleRow
	var onlineRules []OnlineRuleRow

	// 1. Bind Data
	// Debug: Print available sheets
	sheetNames := make([]string, 0, len(raw))
	for k := range raw {
		sheetNames = append(sheetNames, k)
	}
	// p.Logger.Info("ModbusPlugin: Available Sheets in Normalized Data", zap.Strings("sheets", sheetNames))

	if err := importer.Bind(raw, "Devices", &devices); err != nil {
		// p.Logger.Error("ModbusPlugin: Failed to bind Devices", zap.Error(err))
		return nil, fmt.Errorf("bind devices failed: %v", err)
	}
	// p.Logger.Info("ModbusPlugin: Bound Devices", zap.Int("count", len(devices)))

	// Dump first device for debug
	if len(devices) > 0 {
		// p.Logger.Info("ModbusPlugin: First Device Dump", zap.Any("device", devices[0]))
	}

	importer.Bind(raw, "Polling Groups", &groups)
	// p.Logger.Info("ModbusPlugin: Bound Polling Groups", zap.Int("count", len(groups)))
	if len(groups) > 0 {
		// p.Logger.Info("ModbusPlugin: First Group Dump", zap.Any("group", groups[0]))
	}

	importer.Bind(raw, "Points (Manual)", &manualPoints)
	// p.Logger.Info("ModbusPlugin: Bound Points (Manual)", zap.Int("count", len(manualPoints)))
	if len(manualPoints) > 0 {
		// p.Logger.Info("ModbusPlugin: First Manual Point Dump", zap.Any("point", manualPoints[0]))
	}

	importer.Bind(raw, "Points (Direct)", &directPoints)
	// p.Logger.Info("ModbusPlugin: Bound Points (Direct)", zap.Int("count", len(directPoints)))

	importer.Bind(raw, "Event Rules", &eventRules)
	// p.Logger.Info("ModbusPlugin: Bound Event Rules", zap.Int("count", len(eventRules)))

	importer.Bind(raw, "Online Rules", &onlineRules)
	// p.Logger.Info("ModbusPlugin: Bound Online Rules", zap.Int("count", len(onlineRules)))

	// 2. Index Data
	groupMap := make(map[string][]map[string]interface{})
	pointMap := make(map[string][]map[string]interface{})
	eventMap := make(map[string][]map[string]interface{})
	onlineMap := make(map[string]map[string]interface{})

	// Helper
	addPoint := func(code string, pt map[string]interface{}) {
		pointMap[code] = append(pointMap[code], pt)
	}

	// Process Groups
	for _, row := range groups {
		if row.DeviceCode == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Group with empty DeviceCode", zap.Int("index", i), zap.Any("row", row))
			continue
		}
		devCode := parseNameCodeFormat(row.DeviceCode)
		if devCode == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Group with empty DeviceCode", zap.Int("index", i))
			continue
		}

		enabledStr := strings.TrimSpace(row.Enabled)
		isEnabled := enabledStr == "是"
		// p.Logger.Info("ModbusPlugin: Processing Group",
		// zap.Int("index", i),
		// zap.String("name", row.Name),
		// zap.String("enabled_raw", row.Enabled),
		// zap.String("enabled_trimmed", enabledStr),
		// zap.Bool("is_enabled", isEnabled))

		grp := map[string]interface{}{
			"name":          strings.TrimSpace(row.Name),
			"enable":        isEnabled, // Frontend uses 'enable'
			"enabled":       isEnabled, // Keep for backward compatibility
			"slave_id":      importToInt(row.SlaveID),
			"function_code": importToInt(row.FunctionCode),
			"start_address": importToInt(row.StartAddress),
			"length":        importToInt(row.Length),
			"interval":      importToInt(row.Interval),
		}
		groupMap[devCode] = append(groupMap[devCode], grp)
	}

	// Process Manual Points
	for _, row := range manualPoints {
		devCode := parseNameCodeFormat(row.DeviceCode)
		if devCode == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Manual Point with empty DeviceCode", zap.Int("index", i), zap.Any("row", row))
			continue
		}

		identifier := strings.TrimSpace(row.Identifier)
		displayName := strings.TrimSpace(row.Name)

		// Filter empty rows (user might have filled DeviceCode but left other columns empty)
		if identifier == "" && displayName == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Manual Point with empty Identifier and Name", zap.Int("index", i))
			continue
		}

		pollingGroup := strings.TrimSpace(row.PollingGroup)
		if pollingGroup == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Manual Point with empty Polling Group", zap.Int("index", i), zap.String("identifier", identifier))
			continue
		}

		pt := map[string]interface{}{
			"name":          identifier,
			"display_name":  displayName,
			"is_property":   true,
			"polling_group": row.PollingGroup,
			"offset":        importToInt(row.Offset),
			"data_type":     row.DataType,
			"read_expr":     row.ReadExpr,
			"precision":     importToInt(row.Precision),
			"bit_index":     importToInt(row.BitIndex),
			"byte_order":    strings.TrimSpace(row.ByteOrder),
			"access_mode":   parseAccessMode(row.EnableWrite),
		}

		if strings.TrimSpace(row.EnableWrite) == "是" {
			pt["write_function_code"] = importToInt(row.WriteFunctionCode)
			pt["write_mode"] = parseWriteMode(row.WriteMode)
			pt["write_expr"] = row.WriteExpr
			pt["write_address"] = importToInt(row.WriteAddress)
			pt["write_slave_id"] = importToInt(row.WriteSlaveID)
		}
		addPoint(devCode, pt)
	}

	// Process Direct Points
	for _, row := range directPoints {
		devCode := parseNameCodeFormat(row.DeviceCode)
		if devCode == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Direct Point with empty DeviceCode", zap.Int("index", i), zap.Any("row", row))
			continue
		}

		identifier := strings.TrimSpace(row.Identifier)
		displayName := strings.TrimSpace(row.Name)

		if identifier == "" && displayName == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping Direct Point with empty Identifier and Name", zap.Int("index", i))
			continue
		}

		pt := map[string]interface{}{
			"name":          identifier,
			"display_name":  displayName,
			"is_property":   true,
			"slave_id":      importToInt(row.SlaveID),
			"function_code": importToInt(row.FunctionCode),
			"address":       importToInt(row.Address),
			"interval":      importToInt(row.Interval),
			"data_type":     row.DataType,
			"read_expr":     row.ReadExpr,
			"precision":     importToInt(row.Precision),
			"bit_index":     importToInt(row.BitIndex),
			"byte_order":    strings.TrimSpace(row.ByteOrder),
			"access_mode":   parseAccessMode(row.EnableWrite),
		}
		if strings.TrimSpace(row.EnableWrite) == "是" {
			pt["write_function_code"] = importToInt(row.WriteFunctionCode)
			pt["write_mode"] = parseWriteMode(row.WriteMode)
			pt["write_expr"] = row.WriteExpr
			pt["write_address"] = importToInt(row.WriteAddress)
			pt["write_slave_id"] = importToInt(row.WriteSlaveID)
		}
		addPoint(devCode, pt)
	}

	// Process Event Rules
	for _, row := range eventRules {
		if row.DeviceCode == "" {
			continue
		}
		devCode := parseNameCodeFormat(row.DeviceCode)

		evt := map[string]interface{}{
			"name":            row.Name,
			"identifier":      row.Identifier,
			"trigger_logic":   strings.ToLower(row.TriggerLogic),
			"triggers":        parseTriggers(row.Triggers),
			"condition_logic": strings.ToLower(row.ConditionLogic),
			"conditions":      parseConditions(row.Conditions),
			"report_interval": parseReportInterval(row.ReportInterval),
			"params":          parseParams(row.Params),
		}
		if evt["trigger_logic"] == "" {
			evt["trigger_logic"] = "or"
		}
		if evt["condition_logic"] == "" {
			evt["condition_logic"] = "and"
		}
		eventMap[devCode] = append(eventMap[devCode], evt)
	}

	// Process Online Rules
	for _, row := range onlineRules {
		if row.DeviceCode == "" {
			continue
		}
		devCode := parseNameCodeFormat(row.DeviceCode)

		// Map strategy from Excel to Internal
		strategy := parseOnlineStrategy(row.Strategy)

		rule := map[string]interface{}{
			"strategy":               strategy,
			"point":                  row.Point,
			"operator":               row.Operator,
			"value":                  importToFloat(row.Value),
			"online_debounce":        1, // Default value
			"offline_debounce":       importToInt(row.OfflineDebounce),
			"max_unchanged_interval": importToInt(row.MaxUnchangedInterval),
		}
		// Ensure camelCase key for frontend consistency and snake_case for backend compatibility
		onlineMap[devCode] = rule
	}

	// 3. Load TSL Definitions for all referenced products
	// Optimization: Batch load products first to get TSL
	prodCodes := make([]string, 0)
	for _, dev := range devices {
		if dev.ProductCode != "" {
			prodCodes = append(prodCodes, parseNameCodeFormat(dev.ProductCode))
		}
	}

	// 4. Batch Load TSL
	tslMap := make(map[string]map[string]bool)
	if len(prodCodes) > 0 {
		// Dedup
		uniqueProds := make(map[string]bool)
		for _, pc := range prodCodes {
			uniqueProds[pc] = true
		}
		prodList := make([]string, 0, len(uniqueProds))
		for pc := range uniqueProds {
			prodList = append(prodList, pc)
		}

		// Query DB via Context
		var products []types.ProductMeta
		for pc := range uniqueProds {
			if prod, ok := p.Ctx.GetProduct(pc); ok && prod != nil {
				products = append(products, *prod)
			} else {
				// p.Logger.Warn("ModbusPlugin: Product not found during import TSL check", zap.String("code", pc))
			}
		}

		if len(products) > 0 { // Emulate successful batch call structure
			for _, prod := range products {
				tslMap[prod.Code] = make(map[string]bool)

				// Parse TSL from Config map
				if tsl, ok := prod.Config["tsl"].(map[string]interface{}); ok {
					if props, ok := tsl["properties"].([]interface{}); ok {
						for _, prop := range props {
							if pMap, ok := prop.(map[string]interface{}); ok {
								if id, ok := pMap["identifier"].(string); ok {
									tslMap[prod.Code][id] = true
								}
							}
						}
					}
				}
			}
		}
	}

	// Build Result
	for _, dev := range devices {
		// Validations
		if dev.Code == "" || dev.Name == "" {
			// p.Logger.Warn("ModbusPlugin: Skipping invalid device row (Missing Code/Name)", zap.String("code", dev.Code), zap.String("name", dev.Name))
			continue
		}

		devCode := strings.TrimSpace(dev.Code)

		// Parse Product Code
		realProdCode := parseNameCodeFormat(dev.ProductCode)

		// Check TSL vs Custom Logic
		finalPoints := make([]map[string]interface{}, 0)
		pts := pointMap[devCode]

		// Filter points based on Collection Mode
		collectionMode := parseCollectionMode(dev.CollectionMode)
		var filteredPts []map[string]interface{}
		for _, pt := range pts {
			_, hasGroup := pt["polling_group"]
			// Polling Mode: Keep points with polling_group
			// AutoReport Mode: Keep points without polling_group (Direct Points)
			if collectionMode == "manual" {
				if hasGroup {
					filteredPts = append(filteredPts, pt)
				}
			} else {
				if !hasGroup {
					filteredPts = append(filteredPts, pt)
				}
			}
		}
		pts = filteredPts

		// Debug: Log TSL Map availability
		_ = tslMap[realProdCode] != nil
		// p.Logger.Info("ModbusPlugin: Checking TSL for Device",
		// zap.String("device", devCode),
		// zap.String("product", realProdCode),
		// zap.String("collection_mode", collectionMode),
		// zap.Int("points_count", len(pts)),
		// zap.Bool("has_tsl", hasTSL),
		// zap.Int("tsl_prop_count", len(tslMap[realProdCode])))

		for _, pt := range pts {
			identifier := pt["name"].(string)
			// Check if it exists in TSL
			if tslMap[realProdCode] != nil && tslMap[realProdCode][identifier] {
				// Case 1: TSL Property -> Mapping
				pt["is_property"] = true
				// p.Logger.Info("ModbusPlugin: Point Matched TSL", zap.String("point", identifier))
			} else {
				// Case 2: Custom Point -> Add as custom
				pt["is_property"] = false // Mark as custom point (not TSL property)
				// p.Logger.Warn("ModbusPlugin: Point NOT in TSL (treated as Custom)",
				// zap.String("device", devCode),
				// zap.String("point", identifier),
				// zap.String("product", realProdCode))
			}
			finalPoints = append(finalPoints, pt)
		}

		config := map[string]interface{}{
			"protocol_type":    parseProtocolType(dev.ProtocolType),
			"ip":               dev.IP,
			"port":             importToInt(dev.Port),
			"slave_id":         importToInt(dev.SlaveID),
			"timeout":          importToInt(dev.Timeout),
			"max_group_length": importToInt(dev.MaxGroupLength),
			"max_address_gap":  importToInt(dev.MaxAddressGap),
			"polling_groups":   groupMap[devCode],
			"points":           finalPoints,
			"groups":           groupMap[devCode],
			"events":           eventMap[devCode],
			"onlineRule":       onlineMap[devCode], // Use camelCase for frontend
			"online_rule":      onlineMap[devCode], // Use snake_case for backend
		}

		if collectionMode == "auto" {
			config["collection_mode"] = "auto"
		}

		// Create point models for core display
		var pointModels []importer.PointImportModel
		for _, pt := range finalPoints {
			pointModels = append(pointModels, importer.PointImportModel{
				Name:   pt["display_name"].(string),
				Code:   pt["name"].(string),
				Config: pt,
			})
		}

		model := importer.DeviceImportModel{
			Name:        dev.Name,
			Code:        devCode,
			ProductCode: realProdCode,
			ParentCode:  parseNameCodeFormat(dev.ParentCode),
			Enabled:     strings.TrimSpace(dev.Enabled) == "是",
			Config:      config,
			// Points:      pointModels,
		}
		result.Devices = append(result.Devices, model)
	}

	return result, nil
}

func parseAccessMode(enableWrite string) string {
	if strings.TrimSpace(enableWrite) == "是" {
		return "rw"
	}
	return "r"
}

func importToInt(s string) int {
	var i int
	fmt.Sscanf(s, "%d", &i)
	return i
}

func importToFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func parseTriggers(input string) []map[string]interface{} {
	var triggers []map[string]interface{}
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '\n' || r == ';'
	})

	ops := []string{">=", "<=", "==", "!=", "change", ">", "<"}

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var op string
		var opIdx int = -1

		minIdx := len(part)
		matchedOp := ""

		for _, o := range ops {
			idx := strings.Index(part, o)
			if idx != -1 {
				if idx < minIdx {
					minIdx = idx
					matchedOp = o
				} else if idx == minIdx {
					if len(o) > len(matchedOp) {
						matchedOp = o
					}
				}
			}
		}

		if matchedOp == "" {
			continue
		}

		op = matchedOp
		opIdx = minIdx

		point := strings.TrimSpace(part[:opIdx])
		rest := strings.TrimSpace(part[opIdx+len(op):])

		var val float64

		if op != "change" {
			fields := strings.Fields(rest)
			if len(fields) > 0 {
				fmt.Sscanf(fields[0], "%f", &val)
			}
		}

		trigger := map[string]interface{}{
			"point":    point,
			"operator": op,
			"value":    val,
		}
		triggers = append(triggers, trigger)
	}
	return triggers
}

func parseConditions(input string) []map[string]interface{} {
	return parseTriggers(input)
}

func parseParams(input string) map[string]string {
	params := make(map[string]string)
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '\n' || r == ';'
	})

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			k := strings.TrimSpace(kv[0])
			v := strings.TrimSpace(kv[1])
			if k != "" && v != "" {
				params[k] = v
			}
		}
	}
	return params
}

func parseReportInterval(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	var i int
	if _, err := fmt.Sscanf(s, "%d", &i); err != nil {
		return -1
	}
	return i
}
