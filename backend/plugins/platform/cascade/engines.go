package cascade

// PlatformEngine handles platform-side logic (central node)
type PlatformEngine interface {
	Start() error
	Stop() error
}

// GatewayEngine handles gateway-side logic (edge node)
type GatewayEngine interface {
	Start() error
	Stop() error
}
