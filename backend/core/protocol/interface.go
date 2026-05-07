package protocol

import (
	"context"
	"time"

	"noyo/core/importer"
	"noyo/core/types"
)

// DeviceConfig is replaced by types.DeviceMeta usage
// But we might keep it for specific config schema methods if strict typing is needed.
// However, Schema methods usually take a map or struct.
// Let's use types.DeviceMeta everywhere for consistency.

// DiscoveredDevice represents a device found during discovery
type DiscoveredDevice struct {
	ExternalID  string
	Name        string
	ProductCode string
	Config      map[string]interface{}
}

// IProtocolPlugin defines the interface for southbound protocol plugins
// Responsible for device communication, data collection, and control
type IProtocolPlugin interface {
	// Init initializes the plugin with a context
	Init(ctx Context) error

	// Start starts the plugin (e.g. start polling, connect to gateway)
	Start() error

	// Stop stops the plugin
	Stop() error

	// BatchAddDevice notifies the plugin to manage these devices
	// Called on system startup or when configuration changes
	BatchAddDevice(devices []types.DeviceMeta) error

	// RemoveDevice notifies the plugin to stop managing a device
	RemoveDevice(deviceCode string) error

	// WritePoint handles write commands (Platform -> Device)
	WritePoint(device types.DeviceMeta, pointCode string, value interface{}) error

	// Config Schemas
	GetProductConfigSchema() ([]byte, error)
	GetDeviceConfigSchema(config types.DeviceMeta) ([]byte, error)
	GetPointConfigSchema() ([]byte, error)

	// SubDeviceConfigCustomizable 返回子设备配置参数是否可由用户在产品上自定义
	// 返回 true 表示用户可以自定义子设备配置 Schema（如 Script 协议）
	// 返回 false 表示子设备配置由协议固定（如 Modbus 协议）
	SubDeviceConfigCustomizable() bool

	// ProtocolMappingRequired 返回设备是否需要协议映射（物模型点位映射）
	// 返回 true 表示需要映射（如 Modbus 等标准协议）
	// 返回 false 表示不需要映射（如 Script 万能协议，自行处理数据）
	ProtocolMappingRequired() bool

	// Batch Import Support
	GetImportTemplateLayout(lang string) []importer.SheetLayout
	ResolveImportData(ctx context.Context, rawData importer.ImportRawData) (*importer.ImportResult, error)
	GetImportSampleData(products []types.ProductMeta) (*importer.ImportRawData, error)

	// Discover performs device discovery (Optional)
	// params can contain network range, or specific target details
	Discover(params map[string]interface{}) ([]DiscoveredDevice, error)

	// WriteProperty writes a property to a device
	WriteProperty(device types.DeviceMeta, propName string, value interface{}) error

	// CallService invokes a service/command on the device
	CallService(device types.DeviceMeta, serviceCode string, params map[string]interface{}) (interface{}, error)

	// GetMeta returns the plugin metadata
	GetMeta() *types.PluginMeta

	// SetMeta sets the plugin metadata (called by Manager)
	SetMeta(meta *types.PluginMeta)

	// IsEnabled returns true if the plugin is enabled
	IsEnabled() bool
}

// IRawStreamProvider defines an interface for protocol plugins that provide raw video frames
type IRawStreamProvider interface {
	AttachRawStream(deviceCode string, viewerID string, onFrame func(frame []byte, duration time.Duration), onClose func()) (cleanup func(), err error)
}

