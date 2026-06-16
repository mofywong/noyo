package cascade

// Config defines the configuration for the Cascade Plugin
type Config struct {
	Mode string `json:"mode" yaml:"mode" v:"required#Mode is required (platform or gateway)"` // platform or gateway

	MqttUrl            string `json:"mqtt_url" yaml:"mqtt_url" v:"required#MQTT URL is required"`
	EnableTLS          bool   `json:"enable_tls" yaml:"enable_tls" title_en:"Enable TLS" title_zh:"是否启用TLS" desc_en:"Enable TLS encryption for MQTT connection" desc_zh:"启用 MQTT TLS 加密连接"`
	InsecureSkipVerify bool   `json:"insecure_skip_verify" yaml:"insecure_skip_verify" title_en:"Skip Certificate Verification" title_zh:"是否跳过证书验证" desc_en:"Skip TLS certificate verification when enabled" desc_zh:"启用后不验证服务端证书"`
	Username           string `json:"username" yaml:"username"`
	Password           string `json:"password" yaml:"password"`

	GatewaySn   string `json:"gateway_sn" yaml:"gateway_sn"`     // Required for gateway mode
	GatewayName string `json:"gateway_name" yaml:"gateway_name"` // Gateway name to report on registration

	TenantName  string `json:"tenant_name" yaml:"tenant_name"`
	ProjectName string `json:"project_name" yaml:"project_name"`
}
