package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/platform"
	"noyo/core/protocol"
	"noyo/core/store"
	"noyo/core/types"
	"reflect"
	"strings"
	"sync"

	"github.com/gogf/gf/v2/util/gconv"
	"go.uber.org/zap"
)

var pluginMetas []PluginMeta

// PluginMeta alias is already defined in plugin.go

// InstallPlugin registers a plugin
func InstallPlugin[C any](meta PluginMeta) {
	var c *C
	meta.Type = reflect.TypeOf(c).Elem()
	if meta.Name == "" {
		meta.Name = strings.TrimSuffix(meta.Type.Name(), "Plugin")
	}
	pluginMetas = append(pluginMetas, meta)
}

// IManagedPlugin defines the common interface for all managed plugins
type IManagedPlugin interface {
	GetMeta() *types.PluginMeta
	IsEnabled() bool
	Start() error
	Stop() error
}

// PluginManager manages the lifecycle of plugins
type PluginManager struct {
	Server          *Server
	ProtocolPlugins map[string]protocol.IProtocolPlugin
	PlatformPlugins map[string]platform.IPlatformPlugin
	mu              sync.RWMutex
}

func NewPluginManager(s *Server) *PluginManager {
	return &PluginManager{
		Server:          s,
		ProtocolPlugins: make(map[string]protocol.IProtocolPlugin),
		PlatformPlugins: make(map[string]platform.IPlatformPlugin),
	}
}

// createInstance creates and initializes a plugin instance
func (pm *PluginManager) createInstance(meta PluginMeta) (interface{}, error) {
	// 1. Create instance via reflection
	instance := reflect.New(meta.Type).Interface()

	// 2. Check Interface and Init

	// A. Protocol Plugin
	if p, ok := instance.(protocol.IProtocolPlugin); ok {
		cfgData, err := pm.configurePlugin(p, instance, meta)
		if err != nil {
			return nil, err
		}
		ctx := NewProtocolContext(meta.Name, pm.Server, cfgData)
		if err := p.Init(ctx); err != nil {
			return nil, fmt.Errorf("%w: failed to init protocol plugin %s: %v", types.ErrInternal, meta.Name, err)
		}
		return p, nil
	}

	// B. Platform Plugin
	if p, ok := instance.(platform.IPlatformPlugin); ok {
		cfgData, err := pm.configurePlugin(p, instance, meta)
		if err != nil {
			return nil, err
		}
		ctx := NewPlatformContext(meta.Name, pm.Server, cfgData)
		if err := p.Init(ctx); err != nil {
			return nil, fmt.Errorf("%w: failed to init platform plugin %s: %v", types.ErrInternal, meta.Name, err)
		}
		return p, nil
	}

	return nil, fmt.Errorf("plugin %s does not implement any known interface (IProtocolPlugin or IPlatformPlugin)", meta.Name)
}

// pluginConfigResult holds both config and enabled state
type pluginConfigResult struct {
	config  map[string]interface{}
	enabled bool
}

func (pm *PluginManager) loadPluginConfigData(meta PluginMeta) (*pluginConfigResult, error) {
	// Try to load from Database
	model, err := store.GetPlugin(meta.Name)
	if err != nil {
		return nil, err
	}

	if model != nil {
		var cfgMap map[string]interface{}
		if model.Config != "" {
			if err := json.Unmarshal([]byte(model.Config), &cfgMap); err != nil {
				return nil, err
			}
		}
		return &pluginConfigResult{config: cfgMap, enabled: model.Enabled}, nil
	}

	return &pluginConfigResult{config: nil, enabled: false}, nil
}

// IConfigurablePlugin defines plugins that can be configured programmatically
type IConfigurablePlugin interface {
	SetMeta(meta *PluginMeta)
	SetEnabled(enabled bool)
}

// configurePlugin handles common configuration logic (Meta, Enabled, Config)
func (pm *PluginManager) configurePlugin(p interface{}, instance interface{}, meta PluginMeta) (map[string]interface{}, error) {
	// 1. Load Config
	configResult, err := pm.loadPluginConfigData(meta)
	if err != nil {
		return nil, err
	}

	// 2. Set Meta and Enabled via Interface (Validation)
	if cp, ok := p.(IConfigurablePlugin); ok {
		cp.SetMeta(&meta)
		if configResult != nil {
			cp.SetEnabled(configResult.enabled)
		}
	} else {
		// Fallback or Error?
		// Since we control BasePlugin, all plugins SHOULD implement this if they embed BasePlugin.
		// If they implement the interface manually, they should also implement these methods.
		// For safety/strictness, we could log a warning if not implemented.
		pm.Server.Logger.Warn("Plugin does not implement IConfigurablePlugin, skipping Meta/Enabled setup via interface", zap.String("name", meta.Name))
	}

	// 3. Map Config via Reflection (because Config struct field is in the concrete struct)
	if configResult != nil && configResult.config != nil {
		val := reflect.ValueOf(instance).Elem()
		pm.mapConfigToStruct(val, configResult.config)
		return configResult.config, nil
	}

	return nil, nil
}

// mapConfigToStruct maps a config map to the plugin's Config struct field
func (pm *PluginManager) mapConfigToStruct(pluginVal reflect.Value, cfgMap map[string]interface{}) {
	configField := pluginVal.FieldByName("Config")
	if !configField.IsValid() {
		return
	}

	// Use gconv for robust mapping (supports json tags, types conversion, etc.)
	if configField.Kind() == reflect.Struct && configField.CanAddr() {
		// Pass pointer to the struct
		if err := gconv.Struct(cfgMap, configField.Addr().Interface()); err != nil {
			pm.Server.Logger.Error("Failed to map config to struct", zap.Error(err))
		}
	}
}

// InitPlugins initializes all registered plugins
func (pm *PluginManager) InitPlugins() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, meta := range pluginMetas {
		pm.Server.Logger.Info("Initializing plugin", zap.String("name", meta.Name))

		instance, err := pm.createInstance(meta)
		if err != nil {
			pm.Server.Logger.Error("Failed to create plugin instance", zap.String("name", meta.Name), zap.Error(err))
			continue
		}

		if p, ok := instance.(protocol.IProtocolPlugin); ok {
			pm.ProtocolPlugins[meta.Name] = p
		} else if p, ok := instance.(platform.IPlatformPlugin); ok {
			pm.PlatformPlugins[meta.Name] = p
		} else {
			pm.Server.Logger.Error("Plugin does not implement known interface", zap.String("name", meta.Name))
		}
	}
	return nil
}

// ReloadPlugin reloads a specific plugin by name
func (pm *PluginManager) ReloadPlugin(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var oldInstance IManagedPlugin

	// Search in maps
	if p, ok := pm.ProtocolPlugins[name]; ok {
		oldInstance = p
	} else if p, ok := pm.PlatformPlugins[name]; ok {
		oldInstance = p
	} else {
		return fmt.Errorf("plugin %s not found", name)
	}

	pm.Server.Logger.Info("Reloading plugin", zap.String("name", name))

	// Find registered meta to get the correct Type
	var registeredMeta PluginMeta
	found := false
	for _, m := range pluginMetas {
		if m.Name == name {
			registeredMeta = m
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("plugin %s is not registered", name)
	}

	// Stop old instance
	if err := oldInstance.Stop(); err != nil {
		pm.Server.Logger.Error("Failed to stop plugin during reload", zap.String("plugin", name), zap.Error(err))
	}

	// Re-create using registered meta (which has Type)
	newInstance, err := pm.createInstance(registeredMeta)
	if err != nil {
		return fmt.Errorf("failed to create new instance for plugin %s: %w", name, err)
	}

	// Start new if enabled
	var newManaged IManagedPlugin
	if p, ok := newInstance.(protocol.IProtocolPlugin); ok {
		pm.ProtocolPlugins[name] = p
		newManaged = p
	} else if p, ok := newInstance.(platform.IPlatformPlugin); ok {
		pm.PlatformPlugins[name] = p
		newManaged = p
	} else {
		return fmt.Errorf("reloaded plugin %s is not valid type", name)
	}

	if newManaged.IsEnabled() {
		if err := newManaged.Start(); err != nil {
			return fmt.Errorf("failed to start reloaded plugin %s: %w", name, err)
		}
	} else {
		pm.Server.Logger.Info("Plugin reloaded but disabled, skipping start", zap.String("name", name))
	}

	pm.Server.Logger.Info("Plugin reloaded successfully", zap.String("name", name))
	return nil
}

// GetPlugins returns a copy of the current plugin instances (thread-safe)
func (pm *PluginManager) GetPlugins() []IManagedPlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var instances []IManagedPlugin
	for _, p := range pm.ProtocolPlugins {
		instances = append(instances, p)
	}
	for _, p := range pm.PlatformPlugins {
		instances = append(instances, p)
	}
	return instances
}

// GetPlatformPlugins returns a copy of the current platform plugin instances (thread-safe)
func (pm *PluginManager) GetPlatformPlugins() []platform.IPlatformPlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var instances []platform.IPlatformPlugin
	for _, p := range pm.PlatformPlugins {
		instances = append(instances, p)
	}
	return instances
}

// GetPlugin returns a specific plugin by name (thread-safe)
func (pm *PluginManager) GetPlugin(name string) IManagedPlugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if p, ok := pm.ProtocolPlugins[name]; ok {
		return p
	}
	if p, ok := pm.PlatformPlugins[name]; ok {
		return p
	}
	return nil
}

// StartPlugins starts all plugins
func (pm *PluginManager) StartPlugins() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var wg sync.WaitGroup

	start := func(p IManagedPlugin) {
		defer wg.Done()
		if !p.IsEnabled() {
			pm.Server.Logger.Info("Plugin disabled, skipping start", zap.String("plugin", p.GetMeta().Name))
			return
		}
		if err := p.Start(); err != nil {
			pm.Server.Logger.Error("Plugin start failed", zap.String("plugin", p.GetMeta().Name), zap.Error(err))
		}
	}

	for _, p := range pm.ProtocolPlugins {
		wg.Add(1)
		go start(p)
	}
	for _, p := range pm.PlatformPlugins {
		wg.Add(1)
		go start(p)
	}

	wg.Wait()
}

// StopPlugins stops all plugins
func (pm *PluginManager) StopPlugins() {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stop := func(p IManagedPlugin) {
		if err := p.Stop(); err != nil {
			pm.Server.Logger.Error("Plugin stop failed", zap.String("plugin", p.GetMeta().Name), zap.Error(err))
		}
	}

	for _, p := range pm.ProtocolPlugins {
		stop(p)
	}
	for _, p := range pm.PlatformPlugins {
		stop(p)
	}
}

// Broadcast methods removed - moved to DispatchService
