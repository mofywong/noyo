package platform

import (
	"noyo/core/types"
	"time"
)

// ICECandidate is the transport-neutral representation of a WebRTC ICE
// candidate.  It intentionally mirrors RTCIceCandidateInit so it can travel
// through the cloud-edge signalling path without exposing media packets.
type ICECandidate struct {
	Candidate        string  `json:"candidate"`
	SDPMid           *string `json:"sdpMid,omitempty"`
	SDPMLineIndex    *uint16 `json:"sdpMLineIndex,omitempty"`
	UsernameFragment *string `json:"usernameFragment,omitempty"`
}

// MediaSignal carries only WebRTC signalling between the browser and a
// gateway.  The camera RTP and WebRTC media always remain off this path.
type MediaSignal struct {
	SessionID       string        `json:"session_id"`
	DeviceCode      string        `json:"device_code,omitempty"`
	Candidate       *ICECandidate `json:"candidate,omitempty"`
	EndOfCandidates bool          `json:"end_of_candidates,omitempty"`
}

// ICEServerConfig is the safe browser-facing representation of an ICE server.
// TURN credentials are deliberately short-lived when issued by the platform.
type ICEServerConfig struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

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
	OnKeyframeRequest(f func())
}

// IWebRTCService is an optional interface that a WebRTC platform plugin can implement
// to provide streaming capabilities to protocol plugins (like GB28181, ONVIF)
type IWebRTCService interface {
	// CreateConnection creates a new WebRTC peer connection and returns the SDP answer and a media track
	CreateConnection(deviceCode, offer string) (answer string, track IMediaTrack, pc IPeerConnection, err error)
}

// IWebRTCICEConfigProvider exposes only the ICE configuration needed by an authorized player.
type IWebRTCICEConfigProvider interface {
	GetPlaybackICEServers() ([]ICEServerConfig, error)
}

// MediaNetworkSnapshot is the immutable, browser-facing ICE configuration for
// one playback session.  Core owns the session lifetime; the media plugin owns
// how this configuration is stored, validated and issued.
type MediaNetworkSnapshot struct {
	ICEServers          []ICEServerConfig
	CredentialExpiresAt int64
	Source              string
	Revision            string
}

// GatewayMediaNetwork is the transport-neutral network configuration that a
// platform can synchronize to a gateway.  It deliberately carries no plugin
// identity so a gateway router never needs to know which media plugin consumes
// it.
type GatewayMediaNetwork struct {
	StunURLs     string `json:"stun_urls"`
	TurnURLs     string `json:"turn_urls"`
	TurnUsername string `json:"turn_username"`
	TurnPassword string `json:"turn_password"`
}

// IPlatformMediaNetworkProvider is an optional capability implemented by the
// media plugin that owns platform WebRTC network configuration.
type IPlatformMediaNetworkProvider interface {
	CreatePlatformMediaNetwork(identity string, now time.Time) (MediaNetworkSnapshot, error)
	GetGatewayMediaNetwork() (GatewayMediaNetwork, bool)
	SetGatewayMediaNetwork(GatewayMediaNetwork)
}

// IWebRTCConfiguredService applies an explicit per-playback ICE snapshot without
// changing the plugin's configuration. An empty non-nil slice means host-only.
type IWebRTCConfiguredService interface {
	CreateConnectionWithICE(deviceCode, offer string, iceServers []ICEServerConfig) (answer string, track IMediaTrack, pc IPeerConnection, err error)
}

// ITrickleWebRTCService is implemented by WebRTC services that support
// immediate SDP answers and asynchronous ICE candidate exchange.
type ITrickleWebRTCService interface {
	CreateTrickleConnection(deviceCode, sessionID, offer string, iceServers []ICEServerConfig, onCandidate func(ICECandidate), onComplete func()) (answer string, track IMediaTrack, pc IPeerConnection, err error)
	AddICECandidate(sessionID string, candidate *ICECandidate, endOfCandidates bool) error
}

// IGatewayRouter defines an interface for a plugin that manages gateway routing
type IGatewayRouter interface {
	IsGatewayDevice(gwSn string) bool
	SendCommandToGateway(gwSn string, cmdID string, payload []byte) (interface{}, error)
}

// IMediaSignalRouter is the cloud-edge signalling seam.  Platform mode sends
// browser candidates down and receives gateway candidates up; gateway mode
// does the inverse.  Implementations must never relay video through it.
type IMediaSignalRouter interface {
	PublishMediaSignal(gatewayCode string, signal MediaSignal) error
	SetMediaSignalHandler(handler func(MediaSignal))
}

// IDataChannelBroadcaster defines an interface for broadcasting data over WebRTC datachannels
type IDataChannelBroadcaster interface {
	BroadcastDataChannel(deviceCode string, data []byte)
	// HasActiveChannels returns true if there are active DataChannels for the given device
	HasActiveChannels(deviceCode string) bool
}
