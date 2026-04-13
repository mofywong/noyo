package platform

import "noyo/core/types"

// DataModel represents a standardized data packet to be pushed to the platform
type DataModel struct {
	DeviceCode  string
	ProductCode string
	Type        string                 // "property", "event", "status"
	Payload     map[string]interface{} // The actual data
	UniqueId    string                 // Message ID if needed
	Timestamp   int64                  // Unix timestamp
}

// IPlatformPlugin defines the interface for northbound platform plugins
// Responsible for uploading data to clouds/systems and receiving commands
type IPlatformPlugin interface {
	// Init initializes the plugin with a context
	Init(ctx Context) error

	// Start starts the plugin (e.g. connect to MQTT broker)
	Start() error

	// Stop stops the plugin
	Stop() error

	// LogError logs an error message
	LogError(msg string, err error)

	// PushData sends data to the platform
	// The Core calls this when device data is received and fully processed
	PushData(data *DataModel) error

	// OnEvent handles internal system events if needed (optional)
	// OnEvent(event interface{})

	// GetMeta returns the plugin metadata
	GetMeta() *types.PluginMeta

	// SetMeta sets the plugin metadata (called by Manager)
	SetMeta(meta *types.PluginMeta)

	// IsEnabled returns true if the plugin is enabled
	IsEnabled() bool
}
