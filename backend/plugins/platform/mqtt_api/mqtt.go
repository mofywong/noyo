package mqtt_api

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	// "go.uber.org/zap"
)

func (p *MQTTAPIPlugin) Start() error {
	cfg := p.Ctx.GetConfig()
	if v, ok := cfg["enable_tls"].(bool); ok {
		p.Config.EnableTLS = v
	}
	// Add support for insecure_skip_verify (default to true for backward compatibility if not present)
	// Or better, default false for security?
	// The original code hardcoded true. Let's keep true as default if not specified to avoid breaking existing setups,
	// checking if user provided it.
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

	// p.Logger.Info("Starting MQTT API Plugin",
	// zap.String("broker", p.Config.Broker),
	// zap.String("gateway", p.Config.GatewayCode),
	// )

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
		p.OnConnect()
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		// p.Logger.Error("MQTT Connection Lost", zap.Error(err))
		p.OnDisconnect()
	})
	opts.SetAutoReconnect(true)

	p.client = mqtt.NewClient(opts)

	// Move connection to background with exponential backoff retry support
	go func() {
		// Base delay 1s, max 60s
		delay := 1 * time.Second
		maxDelay := 60 * time.Second

		for {
			// Check if plugin is stopping
			select {
			case <-p.ctx.Done():
				return
			default:
			}

			if p.client.IsConnected() {
				break
			}

			// Try to connect
			if token := p.client.Connect(); token.Wait() && token.Error() != nil {
				// p.Logger.Error("Failed to connect to MQTT",
				// zap.Error(token.Error()),
				// zap.Duration("retry_in", delay))

				// Use select for interruptible sleep
				select {
				case <-p.ctx.Done():
					return
				case <-time.After(delay):
				}

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

	// Start Message Sender Worker
	p.wg.Add(1)
	go p.messageSenderLoop()

	return nil
}

func (p *MQTTAPIPlugin) messageSenderLoop() {
	defer p.wg.Done()
	for {
		select {
		case <-p.ctx.Done():
			return
		case job := <-p.msgChan:
			if p.client != nil && p.client.IsConnected() {
				token := p.client.Publish(job.Topic, job.QoS, job.Retained, job.Payload)
				// We wait here in worker, so it doesn't block the dispatcher
				if token.Wait() && token.Error() != nil {
					// p.Logger.Error("Failed to publish mqtt msg", zap.String("topic", job.Topic), zap.Error(token.Error()))
				}
			}
		}
	}
}

func (p *MQTTAPIPlugin) Stop() error {
	// Cancel workers
	if p.cancel != nil {
		p.cancel()
	}
	p.wg.Wait()

	if p.client != nil && p.client.IsConnected() {
		p.client.Disconnect(250)
	}
	return nil
}

func (p *MQTTAPIPlugin) OnDisconnect() {
	// p.Logger.Info("Disconnected from MQTT API Platform")
}

func (p *MQTTAPIPlugin) OnConnect() {
	// p.Logger.Info("Connected to MQTT API Platform",
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

			ctx := context.Background() // Or plugin context if available

			for _, meta := range onlineDevices {
				// Wait for ticker or context cancellation
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
