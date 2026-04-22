package cascade

// Config defines the configuration for the Cascade Plugin
type Config struct {
	Mode string `json:"mode" yaml:"mode" v:"required#Mode is required (platform or gateway)"` // platform or gateway

	MqttUrl  string `json:"mqtt_url" yaml:"mqtt_url" v:"required#MQTT URL is required"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`

	GatewaySn   string `json:"gateway_sn" yaml:"gateway_sn"`     // Required for gateway mode
	GatewayName string `json:"gateway_name" yaml:"gateway_name"` // Gateway name to report on registration
}
