package cascade

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	"noyo/core"
	"noyo/core/config"
	"noyo/core/platform"
	"noyo/core/store"

	"gorm.io/gorm"
)

// Node settings that change listener/storage construction use the same daemon
// restart contract as the existing local system API. A restarted process loads
// the committed configuration and completes its persisted runtime task.
var restartGatewayForConfig = func() {
	go func() { time.Sleep(2 * time.Second); os.Exit(0) }()
}

func (e *gatewayEngineImpl) applyProtectedGatewayRuntimeChange(ch store.SyncChange) error {
	if e.server == nil {
		return fmt.Errorf("gateway server is unavailable")
	}
	value, err := store.GetProtectedConfigTx(store.DB, ch.EntityCode)
	if err != nil {
		return err
	}
	if value.GatewaySN != e.gatewayCode {
		return fmt.Errorf("protected configuration belongs to another gateway")
	}
	if value.Revision != ch.NewRev {
		return fmt.Errorf("protected configuration runtime task was superseded")
	}
	switch value.Kind {
	case "plugin":
		if e.server.Manager == nil {
			return fmt.Errorf("plugin manager is unavailable")
		}
		if value.PluginName == "cascade" && e.config != nil {
			// A replacement Cascade starts with its committed settings while the
			// previous instance is still completing this same durable task. Avoid
			// recursively replacing the already-correct synchronization engine.
			next := *e.config
			data, err := json.Marshal(value.Config)
			if err != nil {
				return err
			}
			if err = json.Unmarshal(data, &next); err != nil {
				return err
			}
			enabled, known := value.Config["enabled"].(bool)
			if (!known || enabled) && reflect.DeepEqual(next, *e.config) {
				return nil
			}
		}
		return e.server.Manager.ReloadPlugin(value.PluginName)
	case "media_network":
		if e.server.Manager == nil {
			return fmt.Errorf("plugin manager is unavailable")
		}
		provider := e.server.Manager.MediaNetworkProvider()
		if provider == nil {
			return fmt.Errorf("media network provider is unavailable")
		}
		data, err := json.Marshal(value.Config)
		if err != nil {
			return err
		}
		var settings SyncMediaNetwork
		if err = json.Unmarshal(data, &settings); err != nil {
			return err
		}
		provider.SetGatewayMediaNetwork(platform.GatewayMediaNetwork{StunURLs: settings.StunURLs, TurnURLs: settings.TurnURLs, TurnUsername: settings.TurnUsername, TurnPassword: settings.TurnPassword})
		return nil
	case "system":
		if e.server.Config == nil {
			return fmt.Errorf("system configuration is unavailable")
		}
		data, err := json.Marshal(value.Config)
		if err != nil {
			return err
		}
		next := *e.server.Config
		if err = json.Unmarshal(data, &next); err != nil {
			return err
		}
		if next.Server.Port <= 0 || next.Server.Port > 65535 {
			return fmt.Errorf("invalid HTTP port")
		}
		if !reflect.DeepEqual(next.Server, e.server.Config.Server) || !reflect.DeepEqual(next.TSDB, e.server.Config.TSDB) || !sameLogConstruction(next.Log, e.server.Config.Log) {
			// Keep the old runtime marked active until the new process has started.
			if e.server.WebServer != nil {
				restartGatewayForConfig()
			}
			return &core.RuntimeActivationError{Cause: errors.New("committed system configuration is awaiting automatic daemon restart"), CompensationStatus: store.RuntimeCompensated}
		}
		e.server.Config.Log = next.Log
		core.SetLoggerLevel(next.Log.Level)
		store.SetDBLogLevel(next.Log.Level)
		return nil
	default:
		return fmt.Errorf("unsupported protected configuration kind %q", value.Kind)
	}
}

func sameLogConstruction(a, b config.LogConfig) bool {
	a.Level = ""
	b.Level = ""
	return reflect.DeepEqual(a, b)
}

func (e *gatewayEngineImpl) restoreProtectedMediaNetwork() error {
	code := store.ProtectedConfigCode(e.gatewayCode, "media_network", "")
	value, err := store.GetProtectedConfigTx(store.DB, code)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return e.applyProtectedGatewayRuntimeChange(store.SyncChange{EntityType: store.SyncEntityNodeConfig, EntityCode: code, NewRev: value.Revision})
}
