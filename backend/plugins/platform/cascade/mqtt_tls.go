package cascade

import (
	"crypto/tls"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func applyMQTTTLSOptions(opts *mqtt.ClientOptions, cfg *Config) {
	if opts == nil || cfg == nil {
		return
	}
	if !cfg.EnableTLS && !cfg.InsecureSkipVerify && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(cfg.MqttUrl)), "tls://") {
		return
	}
	opts.SetTLSConfig(&tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	})
}
