package core

import (
	"fmt"
	"noyo/core/store"
	"noyo/core/types"
	"strings"
	"sync"
)

var (
	defaultServerMu sync.RWMutex
	defaultServer   *Server
)

// SetDefaultServer sets the globally active server instance.
func SetDefaultServer(s *Server) {
	defaultServerMu.Lock()
	defer defaultServerMu.Unlock()
	defaultServer = s
}

// GetDefaultServer returns the globally active server instance if available.
func GetDefaultServer() *Server {
	defaultServerMu.RLock()
	defer defaultServerMu.RUnlock()
	return defaultServer
}

// NormalizeProtocolName standardizes protocol names and aliases into canonical registered plugin names.
// e.g. "modbus_tcp" -> "Modbus", "bacnet_ip" -> "BACnet", "lua" -> "Script", "GB28181" -> "gb28181".
func NormalizeProtocolName(name string) string {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return ""
	}

	// 1. Exact match against registered protocol plugins
	for _, meta := range pluginMetas {
		if meta.Category == types.PluginCategoryProtocol && meta.Name == raw {
			return meta.Name
		}
	}

	// 2. Case-insensitive match against registered protocol plugins
	for _, meta := range pluginMetas {
		if meta.Category == types.PluginCategoryProtocol && strings.EqualFold(meta.Name, raw) {
			return meta.Name
		}
	}

	// 3. Known alias mapping
	normalizedLower := strings.ToLower(raw)
	normalizedLower = strings.ReplaceAll(normalizedLower, "-", "_")
	normalizedLower = strings.ReplaceAll(normalizedLower, " ", "_")

	switch normalizedLower {
	case "modbus", "modbus_tcp", "modbus_rtu", "modbus_ascii", "modbustcp", "modbusrtu":
		return resolveRegisteredProtocolName("Modbus")
	case "bacnet", "bacnet_ip", "bacnet_mstp", "bacnetip", "bacnet/ip":
		return resolveRegisteredProtocolName("BACnet")
	case "script", "lua", "lua_script", "luascript", "script_driver":
		return resolveRegisteredProtocolName("Script")
	case "gb28181", "gb_28181", "gb-28181", "gb/t28181", "gb_t28181", "video", "sip":
		return resolveRegisteredProtocolName("gb28181")
	}

	// 4. Prefix match fallback (e.g. "modbus_*" -> "Modbus")
	for _, meta := range pluginMetas {
		if meta.Category == types.PluginCategoryProtocol {
			lowerMeta := strings.ToLower(meta.Name)
			if strings.HasPrefix(normalizedLower, lowerMeta+"_") || strings.HasPrefix(normalizedLower, lowerMeta+"-") {
				return meta.Name
			}
		}
	}

	return raw
}

func resolveRegisteredProtocolName(preferred string) string {
	for _, meta := range pluginMetas {
		if meta.Category == types.PluginCategoryProtocol {
			if strings.EqualFold(meta.Name, preferred) {
				return meta.Name
			}
		}
	}
	return preferred
}

// FindProtocolPluginMeta searches for a registered protocol plugin matching the name or alias.
// Returns canonical name and true if found.
func FindProtocolPluginMeta(name string) (string, bool) {
	if strings.TrimSpace(name) == "" {
		return "", false
	}
	canonical := NormalizeProtocolName(name)
	for _, meta := range pluginMetas {
		if meta.Category == types.PluginCategoryProtocol && meta.Name == canonical {
			return meta.Name, true
		}
	}
	return canonical, false
}

// EnsureProtocolPluginEnabled ensures that the given protocol plugin is valid and enabled for the tenant/project scope.
// If the plugin has not yet been configured in the database (built-in default), it automatically enables and starts it.
// Returns canonical protocol name and nil on success.
func (s *Server) EnsureProtocolPluginEnabled(protocolName string, tenantID, projectID uint) (string, error) {
	if strings.TrimSpace(protocolName) == "" {
		return "", nil
	}

	canonicalName, found := FindProtocolPluginMeta(protocolName)
	if !found {
		return "", fmt.Errorf("Protocol plugin '%s' is not available", protocolName)
	}

	// Check DB state for current scope
	pluginModel, err := store.GetPluginForScope(canonicalName, tenantID, projectID)
	if err != nil {
		return canonicalName, fmt.Errorf("failed to query protocol plugin status: %w", err)
	}

	if pluginModel == nil {
		// Auto-heal: built-in standard protocol plugin not yet explicitly configured in DB.
		// Enable it for the scope and start it.
		var p IManagedPlugin
		if s != nil && s.Manager != nil {
			p = s.Manager.GetPlugin(canonicalName)
		}
		if p != nil {
			if err := UpdatePluginConfigForScope(p, map[string]interface{}{"enabled": true}, tenantID, projectID); err != nil {
				return canonicalName, fmt.Errorf("failed to auto-enable protocol plugin %s: %w", canonicalName, err)
			}
			if s.Manager != nil {
				_ = s.Manager.ReloadPlugin(canonicalName)
			}
		} else {
			if err := store.SavePluginForScope(canonicalName, tenantID, projectID, true, nil); err != nil {
				return canonicalName, fmt.Errorf("failed to auto-enable protocol plugin %s in db: %w", canonicalName, err)
			}
			if s != nil && s.Manager != nil {
				_ = s.Manager.LoadPlugin(canonicalName)
			}
		}
		return canonicalName, nil
	}

	if !pluginModel.Enabled {
		return canonicalName, fmt.Errorf("Protocol plugin '%s' is disabled for current project", canonicalName)
	}

	// Plugin is enabled in DB; make sure it is running in memory
	if s != nil && s.Manager != nil {
		p := s.Manager.GetPlugin(canonicalName)
		if p == nil {
			_ = s.Manager.LoadPlugin(canonicalName)
		} else if !p.IsEnabled() {
			_ = s.Manager.ReloadPlugin(canonicalName)
		}
	}

	return canonicalName, nil
}

// EnsureProtocolPluginEnabled is a package-level helper using the default server instance if available.
func EnsureProtocolPluginEnabled(protocolName string, tenantID, projectID uint) (string, error) {
	s := GetDefaultServer()
	if s != nil {
		return s.EnsureProtocolPluginEnabled(protocolName, tenantID, projectID)
	}
	canonicalName, found := FindProtocolPluginMeta(protocolName)
	if !found {
		if strings.TrimSpace(protocolName) == "" {
			return "", nil
		}
		return "", fmt.Errorf("Protocol plugin '%s' is not available", protocolName)
	}
	pluginModel, err := store.GetPluginForScope(canonicalName, tenantID, projectID)
	if err != nil {
		return canonicalName, err
	}
	if pluginModel == nil {
		if err := store.SavePluginForScope(canonicalName, tenantID, projectID, true, nil); err != nil {
			return canonicalName, err
		}
		return canonicalName, nil
	}
	if !pluginModel.Enabled {
		return canonicalName, fmt.Errorf("Protocol plugin '%s' is disabled for current project", canonicalName)
	}
	return canonicalName, nil
}
