package platform

import (
	"noyo/core/types"

	"go.uber.org/zap"
)

// Context provides interaction capabilities for Platform Plugins with the Core
type Context interface {
	// IssueCommand sends a command to a device (Platform -> Core -> Protocol)
	IssueCommand(deviceCode string, cmdCode string, params map[string]interface{}) (interface{}, error)

	// GetConfig returns the plugin configuration
	GetConfig() map[string]interface{}

	// GetCoreServer returns the core server instance
	GetCoreServer() interface{}

	// LogInfo logs an info message
	LogInfo(msg string, fields ...interface{})
	// LogError logs an error message
	LogError(msg string, err error)

	// GetLogger returns the raw zap logger
	GetLogger() *zap.Logger

	// RegisterHTTPHandler registers a custom HTTP handler
	RegisterHTTPHandler(path string, handler interface{}) error

	// GetOnlineDevices returns a list of all online devices
	GetOnlineDevices() ([]types.DeviceMeta, error)

	// GetDeviceData returns the latest real-time data for a device
	GetDeviceData(deviceCode string) map[string]interface{}

	// GetEnabledProtocols returns a list of all enabled protocol plugins
	GetEnabledProtocols() ([]types.PluginMeta, error)

	// SubscribeEvent subscribes to a specific system event
	SubscribeEvent(eventType types.EventType, handler func(event types.Event)) uint64

	// UnsubscribeEvent unsubscribes a handler by its ID
	UnsubscribeEvent(eventType types.EventType, id uint64)

	// PublishEvent publishes a system event
	PublishEvent(event types.Event)

	// ReportDeviceProperties saves properties to TSDB and publishes EventPropertyReported
	ReportDeviceProperties(deviceCode string, properties map[string]interface{}) error

	// ReportDeviceEvent saves event to TSDB and publishes EventEventReported
	ReportDeviceEvent(deviceCode string, eventId string, params map[string]interface{}) error
}
