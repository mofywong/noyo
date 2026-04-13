package types

import (
	"reflect"
)

// PluginMeta describes the plugin metadata
type PluginMeta struct {
	Name        string
	Title       map[string]string // Display Name for UI (i18n)
	Description map[string]string // Description for UI (i18n)
	Version     string
	Type        reflect.Type
	Category    string // types.PluginCategoryPlatform or types.PluginCategoryProtocol
	DefaultYaml string
	Icon        []byte // Plugin icon content (e.g. SVG or PNG)
}

// DeviceMeta encapsulates the device identity information
type DeviceMeta struct {
	// Standard fields: cover 90% of mainstream IoT platform models (Product/Device)
	ProductCode string
	DeviceCode  string
	ParentCode  string

	// Extended fields: store non-standard fields for special platforms (e.g., GroupID, Region, AccessToken)
	// Plugins can retrieve special parameters from here as needed
	Extras map[string]interface{}

	// Context fields (injected by Core)
	SubDevices []DeviceMeta // List of sub-devices (if this is a gateway)
	Parent     *DeviceMeta  // Parent device (if this is a sub-device)
}

// ProductMeta encapsulates the product information
type ProductMeta struct {
	Name         string
	Code         string
	ProtocolName string                 // e.g. "Modbus", "Sagoo"
	Config       map[string]interface{} // Protocol-specific config (e.g. polling groups)
}

// TaskDefinition defines a task to be scheduled
type TaskDefinition struct {
	UID              string // Unique ID for the task
	Name             string
	Interval         string // Cron expression (e.g. "@every 1s") or standard cron
	Handler          func() error
	SkipStatusUpdate bool // 为 true 时，任务执行结果不触发在线/离线状态上报
}
