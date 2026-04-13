package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"noyo/core/store"
	"reflect"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

// Plugin is the base struct that plugins should embed
type Plugin struct {
	Meta    PluginMeta
	Server  *Server
	Logger  *zap.Logger
	Config  interface{}
	Enabled bool        // Status: enabled or disabled
	Handler interface{} // The interface implementation (legacy, kept for backwards compatibility)
}

// LoadConfig loads configuration from database
func (p *Plugin) LoadConfig() error {
	// Default to disabled
	p.Enabled = false

	// 1. Try to load from Database
	model, err := store.GetPlugin(p.Meta.Name)
	if err != nil {
		return fmt.Errorf("failed to load plugin config from db: %w", err)
	}

	if model != nil {
		// Found in DB
		p.Enabled = model.Enabled

		if p.Config != nil && model.Config != "" {
			if err := json.Unmarshal([]byte(model.Config), p.Config); err != nil {
				return fmt.Errorf("failed to unmarshal config from db: %w", err)
			}
			p.Logger.Info("Loaded config from database", zap.Bool("enabled", p.Enabled))
		}
	} else {
		// Not found in DB.
		// Use default values from struct (already initialized by go)
		// Or if DefaultYaml provided, use it for initial structure (though user said no default needed)
		// We still respect DefaultYaml if present for initial setup in memory
		// But we don't save to DB yet? Or should we?
		// User said: "web上每个插件需要配置哪些参数，由插件在代码中写死即可，不需要默认值"
		// "用户配置的参数，需要写入到数据库中"
		// So if no DB record, we just leave it as is (defaults from struct definition).
		p.Logger.Info("No config found in database, using code defaults", zap.Bool("enabled", p.Enabled))
	}

	return nil
}

// --- Default Implementations for IPlugin interface ---

func (p *Plugin) Init(server *Server, meta *PluginMeta) error {
	p.Server = server
	p.Meta = *meta
	// Logger will be injected by the manager

	// Auto register routes
	p.RegisterAutoRoutes()

	return nil
}

// RegisterAutoRoutes scans for methods starting with "API_" and registers them
func (p *Plugin) RegisterAutoRoutes() {
	if p.Handler == nil {
		return
	}

	t := reflect.TypeOf(p.Handler)
	v := reflect.ValueOf(p.Handler)

	for i := 0; i < t.NumMethod(); i++ {
		method := t.Method(i)
		methodName := method.Name

		if strings.HasPrefix(methodName, "API_") {
			// API_Hello -> hello
			routePath := strings.ToLower(strings.TrimPrefix(methodName, "API_"))
			// Construct full path: /api/extension/{plugin_name}/{routePath}
			// Use plugin name from meta
			fullPath := fmt.Sprintf("/api/extension/%s/%s", strings.ToLower(p.Meta.Name), routePath)

			// Check signature: func(r *ghttp.Request)
			// In(0) is receiver, In(1) should be *ghttp.Request
			if method.Type.NumIn() == 2 && method.Type.In(1) == reflect.TypeOf((*ghttp.Request)(nil)) {
				p.Logger.Info("Registering auto route", zap.String("path", fullPath), zap.String("method", methodName))

				// Create a wrapper function to call the method
				handlerFunc := func(r *ghttp.Request) {
					method.Func.Call([]reflect.Value{v, reflect.ValueOf(r)})
				}

				p.Server.WebServer.BindHandler(fullPath, handlerFunc)
			}
		}
	}
}

func (p *Plugin) Start() error {
	return nil
}

func (p *Plugin) Stop() error {
	return nil
}

func (p *Plugin) GetMeta() *PluginMeta {
	return &p.Meta
}

func (p *Plugin) SetMeta(meta *PluginMeta) {
	p.Meta = *meta
}

func (p *Plugin) IsEnabled() bool {
	return p.Enabled
}

func (p *Plugin) SetEnabled(enabled bool) {
	p.Enabled = enabled
}

// OnConnect is a hook for connection events (override in specific plugins)
func (p *Plugin) OnConnect() {
}

// OnDisconnect is a hook for disconnection events (override in specific plugins)
func (p *Plugin) OnDisconnect() {
}

func (p *Plugin) ReportProperty(meta DeviceMeta, properties map[string]interface{}) error {
	return ErrNotImplemented
}

func (p *Plugin) ReportEvent(meta DeviceMeta, eventId string, params map[string]interface{}) error {
	return ErrNotImplemented
}

func (p *Plugin) ReportBatchProperties(meta DeviceMeta, properties map[string]interface{}) error {
	return ErrNotImplemented
}

func (p *Plugin) ReportStatus(meta DeviceMeta, status string) error {
	if p.Server == nil || p.Server.DeviceManager == nil {
		return errors.New("server not initialized")
	}
	var err error
	if status != "online" {
		err = fmt.Errorf("device reported status: %s", status)
	}
	p.Server.DeviceManager.UpdateStatus(meta.DeviceCode, err)
	return nil
}

func (p *Plugin) SetProperty(meta DeviceMeta, properties map[string]interface{}) error {
	return ErrNotImplemented
}

func (p *Plugin) CallService(meta DeviceMeta, serviceId string, params map[string]interface{}) (interface{}, error) {
	return nil, ErrNotImplemented
}
