package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"noyo/core/protocol"
	"noyo/core/store"
	"noyo/core/types"

	"github.com/robfig/cron/v3"
)

// RuntimeActivationError distinguishes a rejected desired configuration from
// failure to restore the last usable execution configuration.
type RuntimeActivationError struct {
	Cause              error
	CompensationStatus string
	CompensationError  error
}

func (e *RuntimeActivationError) Error() string { return e.Cause.Error() }
func (e *RuntimeActivationError) Unwrap() error { return e.Cause }

type deviceRuntimePlan struct {
	meta    types.DeviceMeta
	product types.ProductMeta
	plugin  IManagedPlugin
	defs    []TaskDefinition
}

func (dm *DeviceManager) prepareRuntimePlan(registry *DeviceRegistry, code string) (*deviceRuntimePlan, error) {
	device, ok := registry.GetDevice(code)
	if !ok || !device.Enabled {
		return nil, nil
	}
	meta, err := registry.GetDeviceMeta(code)
	if err != nil {
		return nil, err
	}
	if device.ProtocolProfileCode != "" {
		if _, err := registry.runtimeProfile(device.ProtocolProfileCode); err != nil {
			return nil, fmt.Errorf("driver %s: %w", device.ProtocolProfileCode, err)
		}
	}
	product, err := registry.GetProductMeta(device.ProductCode)
	if err != nil {
		return nil, err
	}
	name, err := registry.GetEffectiveProtocol(code)
	if err != nil {
		return nil, err
	}
	if dm.Server == nil || dm.Server.Manager == nil {
		return nil, fmt.Errorf("plugin manager is unavailable")
	}
	plugin := dm.Server.Manager.GetPlugin(name)
	if plugin == nil {
		return nil, fmt.Errorf("protocol plugin %s is unavailable", name)
	}
	plan := &deviceRuntimePlan{meta: *meta, product: product, plugin: plugin}
	if _, modern := plugin.(protocol.IProtocolPlugin); !modern {
		if legacy, ok := plugin.(LegacyTaskPlugin); ok {
			plan.defs, err = legacy.CreateTasks(*meta, ProductMeta(product))
			if err != nil {
				return nil, err
			}
		}
	}
	if plan.defs == nil {
		plan.defs = []TaskDefinition{{Name: "KeepAlive", Interval: "@every 60s", Handler: func() error { return nil }, SkipStatusUpdate: true}}
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	for _, def := range plan.defs {
		if def.Handler == nil {
			return nil, fmt.Errorf("task %s has no handler", def.Name)
		}
		if _, err := parser.Parse(def.Interval); err != nil {
			return nil, fmt.Errorf("task %s: %w", def.Name, err)
		}
	}
	return plan, nil
}

func (dm *DeviceManager) installRuntimePlan(code string, previous, next *deviceRuntimePlan) error {
	if previous != nil {
		if plugin, ok := previous.plugin.(protocol.IProtocolPlugin); ok {
			if err := plugin.RemoveDevice(code); err != nil {
				return err
			}
		}
	}
	if next == nil {
		return dm.Scheduler.StopDeviceTasks(code)
	}
	if plugin, ok := next.plugin.(protocol.IProtocolPlugin); ok {
		if err := plugin.BatchAddDevice([]types.DeviceMeta{next.meta}); err != nil {
			return err
		}
	}
	return dm.Scheduler.ReplaceDeviceTasks(code, next.defs)
}

// ApplyRuntimeConfig prepares the whole affected set before touching running
// tasks. If a protocol rejects activation, every changed device is compensated
// using its retained metadata and task closures, without changing store rows.
func (dm *DeviceManager) ApplyRuntimeConfig(entityType, code, payload string) error {
	dm.runtimeConfigMu.Lock()
	defer dm.runtimeConfigMu.Unlock()
	if dm.Scheduler == nil {
		return fmt.Errorf("device scheduler is unavailable")
	}
	next := dm.Registry.cloneRuntime()
	if err := next.stageRuntimeConfig(entityType, code, payload); err != nil {
		return &RuntimeActivationError{Cause: err, CompensationStatus: store.RuntimeCompensated}
	}
	var codes []string
	for _, device := range dm.Registry.GetAllDevices() {
		oldMeta, oldErr := dm.Registry.GetDeviceMeta(device.Code)
		newMeta, newErr := next.GetDeviceMeta(device.Code)
		oldJSON, _ := json.Marshal(oldMeta)
		newJSON, _ := json.Marshal(newMeta)
		if (entityType == store.SyncEntityDevice && device.Code == code) || (entityType == store.SyncEntityProduct && device.ProductCode == code) || oldErr != nil || newErr != nil || string(oldJSON) != string(newJSON) {
			codes = append(codes, device.Code)
		}
	}
	if entityType == store.SyncEntityDevice {
		found := false
		for _, c := range codes {
			if c == code {
				found = true
			}
		}
		if !found {
			codes = append(codes, code)
		}
	}
	plans := make(map[string]*deviceRuntimePlan, len(codes))
	oldPlans := make(map[string]*deviceRuntimePlan, len(codes))
	for _, c := range codes {
		if old, ok := dm.runtimePlans.Load(c); ok {
			oldPlans[c] = old.(*deviceRuntimePlan)
		}
		plan, err := dm.prepareRuntimePlan(next, c)
		if err != nil {
			return &RuntimeActivationError{Cause: err, CompensationStatus: store.RuntimeCompensated}
		}
		plans[c] = plan
	}
	for i, c := range codes {
		if err := dm.installRuntimePlan(c, oldPlans[c], plans[c]); err != nil {
			var compensationErrors []error
			for j := i; j >= 0; j-- {
				changed := codes[j]
				if restoreErr := dm.installRuntimePlan(changed, plans[changed], oldPlans[changed]); restoreErr != nil {
					compensationErrors = append(compensationErrors, fmt.Errorf("restore device %s: %w", changed, restoreErr))
				}
			}
			compensationErr := errors.Join(compensationErrors...)
			status := store.RuntimeCompensated
			if compensationErr != nil {
				status = store.RuntimeCompensationFailed
			}
			return &RuntimeActivationError{Cause: fmt.Errorf("activate device %s: %w", c, err), CompensationStatus: status, CompensationError: compensationErr}
		}
	}
	dm.Registry.installRuntime(next)
	for _, c := range codes {
		if plans[c] == nil {
			dm.runtimePlans.Delete(c)
		} else {
			dm.runtimePlans.Store(c, plans[c])
		}
	}
	return nil
}

func validateRuntimeJSON(config string) error {
	if config == "" {
		return nil
	}
	var object map[string]interface{}
	return json.Unmarshal([]byte(config), &object)
}
