package cascade

import "noyo/core/platform"

// PlatformEngine handles platform-side logic (central node)
type PlatformEngine interface {
	Start() error
	Stop() error
	PublishMediaSignal(gatewayCode string, signal platform.MediaSignal) error
	SetMediaSignalHandler(handler func(platform.MediaSignal))
}

// GatewayEngine handles gateway-side logic (edge node)
type GatewayEngine interface {
	Start() error
	Stop() error
	PublishMediaSignal(signal platform.MediaSignal) error
	SetMediaSignalHandler(handler func(platform.MediaSignal))
}
