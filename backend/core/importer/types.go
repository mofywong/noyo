package importer

// SheetLayout 定义一个 Excel 页签
type SheetLayout struct {
	Name         string       // 页签名称 (e.g. "Device List")
	Instructions string       // 顶部说明文案
	Columns      []ColumnMeta // 列定义
	IsHidden     bool         // 是否隐藏 (e.g. Reference Sheet)
}

// ColumnMeta 定义一列
type ColumnMeta struct {
	Key    string  // 字段Key (对应 Struct Tag)
	Header string  // 显示名称 (e.g. "从站地址")
	Width  float64 // 列宽

	// [Style] 语义化样式
	// "required": 必填 (Core渲染为蓝底粗体)
	// "readonly": 只读 (Core渲染为灰底)
	// "default":  默认 (白底)
	Style string

	// [Validation] 数据验证与交互
	Validation *ValidationMeta
}

// ValidationMeta 定义下拉框与校验
type ValidationMeta struct {
	// 类型: "list"(静态), "provider"(系统), "reference"(引用)
	Type string

	// Case 1: 静态列表 (Type="list")
	Options []string

	// Case 2: 系统数据源 (Type="provider")
	// 插件声明想要的数据，Core 负责根据 Context 注入
	// - "target_products": 目标产品列表 (受 product_ids 过滤)
	// - "protocol_subtypes": 协议子类型
	ProviderKey string

	// Case 3: 跨表引用 (Type="reference")
	// 引用另一个 Sheet 的某一列
	RefSheetName  string
	RefColumnKey  string // Core 自动查找该 Key 对应的列号
	DisplayFormat string // 显示格式: "name_code" (e.g. "Device Name (Device Code)")
}

// ImportRawData 中间格式数据：Sheet名 -> 行列表 -> 列Key -> 值
type ImportRawData map[string][]map[string]string

// ImportResult 导入结果
type ImportResult struct {
	Devices []DeviceImportModel
	Errors  []string
}

// DeviceImportModel Core 标准设备模型
type DeviceImportModel struct {
	Name        string
	Code        string
	ProductCode string
	ParentCode  string
	Enabled     bool                   // 是否启用
	Config      map[string]interface{} // 存入 driver_config
	Points      []PointImportModel
}

// PointImportModel Core 标准点位模型
type PointImportModel struct {
	Name   string
	Code   string
	Config map[string]interface{} // 存入 point_config
}
