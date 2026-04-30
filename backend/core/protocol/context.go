package protocol

import (
	"noyo/core/types"
	// Added context import
	// Added noyo/core/importer import
	"go.uber.org/zap"
)

// DeviceStatus represents the runtime status of a device
type DeviceStatus struct {
	Online     bool
	LastActive string // ISO8601 or similar timestamp
}

// Context provides interaction capabilities for Protocol Plugins with the Core
type Context interface {
	// ReportProperty reports a single property value
	ReportProperty(device types.DeviceMeta, key string, value interface{}) error

	// ReportBatchProperties reports multiple properties
	ReportBatchProperties(device types.DeviceMeta, values map[string]interface{}) error

	// ReportEvent reports an event
	ReportEvent(device types.DeviceMeta, eventCode string, params map[string]interface{}) error

	// ReportStatus reports device status (e.g. online, offline, error)
	ReportStatus(device types.DeviceMeta, status string) error

	// ReportOnline reports device online/offline status (boolean wrapper)
	ReportOnline(device types.DeviceMeta, online bool) error

	// GetConfig returns the plugin configuration
	GetConfig() map[string]interface{}

	// Logger returns a logger instance (implementation specific)
	// For simplicity, we might wrap it or simple expose a Log method
	LogInfo(msg string, fields ...interface{})
	LogError(msg string, err error)
	LogDebug(msg string, fields ...interface{})

	// GetDeviceStatus returns the status of a device
	GetDeviceStatus(deviceCode string) (*DeviceStatus, bool)

	// GetProduct returns the product metadata
	GetProduct(productCode string) (*types.ProductMeta, bool)

	// GetLogger returns the underlying logger (zap)
	GetLogger() *zap.Logger

	// ReloadRegistry triggers a reload of the device registry cache
	ReloadRegistry() error

	// RegisterHTTPHandler registers a custom HTTP handler for the plugin
	RegisterHTTPHandler(path string, handler interface{}) error
}
