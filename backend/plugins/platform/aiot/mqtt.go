package aiot

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	// "go.uber.org/zap"
)

func (p *AiotPlugin) Start() error {
	// Load Config from Context if needed (or assume Init did it?)
	// Init DOES NOT do it currently because I removed the block in plugin.go.
	// So I should reload config here.
	cfg := p.Ctx.GetConfig()
	if v, ok := cfg["enable_tls"].(bool); ok {
		p.Config.EnableTLS = v
	}
	// Add support for insecure_skip_verify
	p.Config.InsecureSkipVerify = true
	if v, ok := cfg["insecure_skip_verify"].(bool); ok {
		p.Config.InsecureSkipVerify = v
	}

	if v, ok := cfg["broker"].(string); ok {
		p.Config.Broker = v
	}
	if v, ok := cfg["username"].(string); ok {
		p.Config.Username = v
	}
	if v, ok := cfg["password"].(string); ok {
		p.Config.Password = v
	}
	if v, ok := cfg["client_id"].(string); ok {
		p.Config.ClientID = v
	}
	if v, ok := cfg["gateway_code"].(string); ok {
		p.Config.GatewayCode = v
	}

	if p.Config.Broker == "" {
		// p.Logger.Warn("MQTT Broker address is empty, skipping connection")
		return nil
	}

	brokerAddress := p.Config.Broker
	// Clean up protocol prefixes
	brokerAddress = strings.TrimPrefix(brokerAddress, "tcp://")
	brokerAddress = strings.TrimPrefix(brokerAddress, "ssl://")
	brokerAddress = strings.TrimPrefix(brokerAddress, "tls://")
	brokerAddress = strings.TrimPrefix(brokerAddress, "mqtt://")
	brokerAddress = strings.TrimPrefix(brokerAddress, "mqtts://")

	// Add correct protocol
	if p.Config.EnableTLS {
		brokerAddress = "ssl://" + brokerAddress
	} else {
		brokerAddress = "tcp://" + brokerAddress
	}

	// p.Logger.Info("Configured MQTT Broker", zap.String("final_address", brokerAddress))

	opts := mqtt.NewClientOptions()
	opts.AddBroker(brokerAddress)

	// Set ClientID
	if p.Config.ClientID != "" {
		opts.SetClientID(p.Config.ClientID)
	} else {
		opts.SetClientID(fmt.Sprintf("%s_%d", p.Config.GatewayCode, time.Now().Unix()))
	}

	// Set Auth
	if p.Config.Username != "" {
		opts.SetUsername(p.Config.Username)
	}
	if p.Config.Password != "" {
		opts.SetPassword(p.Config.Password)
	}

	// Set TLS
	if p.Config.EnableTLS {
		// Create a custom TLS config
		tlsConfig := &tls.Config{
			InsecureSkipVerify: p.Config.InsecureSkipVerify,
			ClientAuth:         tls.NoClientCert,
		}
		opts.SetTLSConfig(tlsConfig)
	}

	opts.SetOnConnectHandler(func(c mqtt.Client) {
		// p.Logger.Info("@@@ MQTT Connected")
		p.OnConnect()
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		// p.Logger.Error("@@@ MQTT Connection Lost", zap.Error(err))
		p.OnDisconnect()
	})
	opts.SetAutoReconnect(true)

	p.client = mqtt.NewClient(opts)

	// Move connection to background with eponential backoff retry support
	go func() {
		// Base delay 1s, max 60s
		delay := 1 * time.Second
		maxDelay := 60 * time.Second

		for {
			if p.client.IsConnected() {
				break
			}

			if token := p.client.Connect(); token.Wait() && token.Error() != nil {
				// p.Logger.Error("Failed to connect to MQTT",
					// zap.Error(token.Error()),
					// zap.Duration("retry_in", delay))

				time.Sleep(delay)

				// Exponential backoff
				delay *= 2
				if delay > maxDelay {
					delay = maxDelay
				}
			} else {
				// p.Logger.Info("MQTT Initial Connection Successful")
				break
			}
		}
	}()

	return nil
}

func (p *AiotPlugin) Stop() error {
	if p.client != nil && p.client.IsConnected() {
		// p.Logger.Info("@@@ MQTT Disconnecting...")
		p.client.Disconnect(250)
		// p.Logger.Info("@@@ MQTT Disconnected")
	}
	return nil
}

func (p *AiotPlugin) OnDisconnect() {
	// p.Logger.Info("Disconnected from AIoT Platform")
}

func (p *AiotPlugin) OnConnect() {
	// p.Logger.Info("Connected to AIoT Platform",
		// zap.String("broker", p.Config.Broker),
		// zap.String("client_id", p.Config.ClientID),
	// )
	// Subscribe to topics
	p.subscribeToCommands()

	// Sync Online Status (Run in background to avoid blocking MQTT loop)
	go func() {
		// Give some time for subscriptions to settle
		time.Sleep(500 * time.Millisecond)

		if onlineDevices, err := p.Ctx.GetOnlineDevices(); err == nil {
			// p.Logger.Info("Syncing online status for devices", zap.Int("count", len(onlineDevices)))

			// Rate limiter: 10ms per device (approx 100 TPS)
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			ctx := context.Background()

			for _, meta := range onlineDevices {
				select {
				case <-ticker.C:
					// Proceed
				case <-ctx.Done():
					return
				}

				if err := p.ReportStatus(meta, "online"); err != nil {
					// p.Logger.Error("Failed to sync online status", zap.String("code", meta.DeviceCode), zap.Error(err))
				}
			}
		} else {
			p.Ctx.LogError("Failed to get online devices", err)
		}
	}()
}
