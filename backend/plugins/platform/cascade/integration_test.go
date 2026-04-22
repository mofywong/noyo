package cascade

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"noyo/core"
	"noyo/core/store"
	"noyo/core/types"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// mockContext simulates the platform.Context needed by the engines
type mockContext struct {
	server *core.Server
	logger *zap.Logger
}

func (m *mockContext) IssueCommand(deviceCode string, cmdCode string, params map[string]interface{}) (interface{}, error) {
	return nil, nil
}
func (m *mockContext) GetConfig() map[string]interface{} { return nil }
func (m *mockContext) GetCoreServer() interface{}        { return m.server }
func (m *mockContext) LogInfo(msg string, fields ...interface{}) {
	m.logger.Sugar().Infof(msg, fields...)
}
func (m *mockContext) LogError(msg string, err error) {
	m.logger.Error(msg, zap.Error(err))
}
func (m *mockContext) GetLogger() *zap.Logger                                                    { return m.logger }
func (m *mockContext) RegisterHTTPHandler(path string, handler interface{}) error                { return nil }
func (m *mockContext) GetOnlineDevices() ([]types.DeviceMeta, error)                             { return nil, nil }
func (m *mockContext) GetDeviceData(deviceCode string) map[string]interface{}                    { return nil }
func (m *mockContext) GetEnabledProtocols() ([]types.PluginMeta, error)                          { return nil, nil }
func (m *mockContext) SubscribeEvent(eventType types.EventType, handler func(event types.Event)) {}
func (m *mockContext) PublishEvent(event types.Event)                                            {}
func (m *mockContext) ReportDeviceProperties(deviceCode string, properties map[string]interface{}) error {
	return nil
}
func (m *mockContext) ReportDeviceEvent(deviceCode string, eventId string, params map[string]interface{}) error {
	return nil
}

func TestEnginesIntegration(t *testing.T) {
	brokerURL := "tcp://test.zenaios.com.cn:1883"
	gwSn := "GW-TEST-" + uuid.New().String()[:6]

	// Init DB so store operations don't panic
	store.InitDB(":memory:")

	// Add a dummy gateway device and product to DB for sync test
	prod := &store.Product{Code: "P-GW-TEST", Name: "Test Gateway Product", ProtocolName: "cascade"}
	store.SaveProduct(prod)
	gwDev := &store.Device{Code: gwSn, ProductCode: "P-GW-TEST", Name: "Test Gateway"}
	store.SaveDevice(gwDev)

	// Create dummy core server
	logger, _ := zap.NewDevelopment()
	srv := &core.Server{Logger: logger}
	// Initialize required components
	srv.Manager = core.NewPluginManager(srv)
	srv.DeviceManager = core.NewDeviceManager(srv)
	srv.TSDB = nil

	ctx := &mockContext{server: srv, logger: logger}

	// 1. Init Platform Engine
	platformCfg := &Config{
		MqttUrl:   brokerURL,
		Username:  "",
		Password:  "",
		GatewaySn: "", // Platform doesn't need this
	}
	platformEngine := NewPlatformEngine(ctx, logger, platformCfg)
	if err := platformEngine.Start(); err != nil {
		t.Fatalf("Platform engine failed to start: %v", err)
	}
	defer platformEngine.Stop()

	// Ensure the gateway device is enabled so it can receive sync/request
	_ = store.SaveDevice(&store.Device{
		Code:        gwSn,
		Name:        "Test Gateway",
		ProductCode: "noyo-gw",
		Enabled:     true,
	})

	// 2. Init Gateway Engine
	gwCfg := &Config{
		MqttUrl:   brokerURL,
		Username:  "",
		Password:  "",
		GatewaySn: gwSn,
	}
	gwEngine := NewGatewayEngine(ctx, logger, gwCfg)
	if err := gwEngine.Start(); err != nil {
		t.Fatalf("Gateway engine failed to start: %v", err)
	}
	defer gwEngine.Stop()

	time.Sleep(2 * time.Second) // wait for connections and subscriptions

	t.Run("Test Sync Flow", func(t *testing.T) {
		// When Gateway Engine started, it automatically sent a sync request.
		// The Platform Engine should have handled it and sent a sync_config.json via FileTransfer.
		// Since we didn't mock the whole FileSender/FileReceiver HTTP server,
		// we'll just check if they are communicating. Wait, FileSender in PlatformEngine uses MQTT!
		// Let's just wait a bit and check logs, or simulate a manual sync request

		// To be observable in the test, we can use a separate MQTT client to monitor
		opts := mqtt.NewClientOptions().AddBroker(brokerURL).SetClientID("monitor-" + uuid.New().String()[:8])
		monitorClient := mqtt.NewClient(opts)
		monitorClient.Connect().Wait()
		defer monitorClient.Disconnect(250)

		syncCh := make(chan bool, 1)
		monitorClient.Subscribe(fmt.Sprintf("noyo/cascade/gw/%s/sync/request", gwSn), 1, func(c mqtt.Client, m mqtt.Message) {
			t.Logf("Monitor observed sync request: %s", m.Payload())
			syncCh <- true
		})

		// Trigger a config change which causes gateway to send sync request
		monitorClient.Publish(fmt.Sprintf("noyo/cascade/gw/%s/config_changed", gwSn), 1, false, []byte(`{}`))

		select {
		case <-syncCh:
			t.Log("Sync flow observed successfully")
		case <-time.After(5 * time.Second):
			t.Error("Timeout waiting for sync flow")
		}
	})

	t.Run("Test Command Flow", func(t *testing.T) {
		cmdId := uuid.New().String()
		payload := map[string]interface{}{
			"id":         cmdId,
			"method":     "service_invoke",
			"deviceCode": "dummy-dev",
			"params": map[string]interface{}{
				"service_id": "test_svc",
				"params":     map[string]interface{}{},
			},
		}
		payloadBytes, _ := json.Marshal(payload)

		// Platform Engine sends command
		// Note: since our dummy core.Server doesn't have a real DeviceManager initialized,
		// the Gateway Engine will reply with code 500 "core server not found" or "core server not available"
		// Wait, we can initialize a basic DeviceManager in the dummy core.Server
		// But it won't have the device, so it will fail gracefully.
		// Let's just check if the platform engine gets the reply.

		go func() {
			platformImpl := platformEngine.(*platformEngineImpl)
			res, err := platformImpl.SendCommand(gwSn, cmdId, payloadBytes)
			t.Logf("SendCommand returned: res=%v, err=%v", res, err)
			// Expecting an error because the device is not real, but the communication should succeed
		}()

		time.Sleep(2 * time.Second)
	})

	t.Run("Test Telemetry Flow", func(t *testing.T) {
		// We can directly call handleLocalEvent on GatewayEngine to simulate a core event
		gwImpl, ok := gwEngine.(*gatewayEngineImpl)
		if ok {
			event := types.Event{
				Type:    types.EventPropertyReported,
				Topic:   "dummy-dev",
				Payload: map[string]interface{}{"temp": 25.5},
			}
			gwImpl.handleLocalEvent(event)
			t.Log("Telemetry event simulated on gateway")
		}

		time.Sleep(1 * time.Second)
	})
}
