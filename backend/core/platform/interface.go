package platform

import (
	"noyo/core/types"
	"time"
)

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

// IMediaTrack represents a media track (e.g., WebRTC video track)
type IMediaTrack interface {
	WriteSample(data []byte, duration time.Duration) error
}

// IPeerConnection represents a generic media peer connection
type IPeerConnection interface {
	OnConnectionStateChange(f func(state string))
	ConnectionState() string
	Close() error
}

// IWebRTCService is an optional interface that a WebRTC platform plugin can implement
// to provide streaming capabilities to protocol plugins (like GB28181, ONVIF)
type IWebRTCService interface {
	// CreateConnection creates a new WebRTC peer connection and returns the SDP answer and a media track
	CreateConnection(deviceCode, offer string) (answer string, track IMediaTrack, pc IPeerConnection, err error)
}

// IGatewayRouter defines an interface for a plugin that manages gateway routing
type IGatewayRouter interface {
	IsGatewayDevice(gwSn string) bool
	SendCommandToGateway(gwSn string, cmdID string, payload []byte) (interface{}, error)
}

// IDataChannelBroadcaster defines an interface for broadcasting data over WebRTC datachannels
type IDataChannelBroadcaster interface {
	BroadcastDataChannel(deviceCode string, data []byte)
}

