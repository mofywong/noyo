package core

import (
	"noyo/core/store"
	"noyo/core/types"
)

// DevicePluginAdapter is the protocol-owned seam for device identity,
// defaults, and gateway translation. Core and platform plugins invoke this
// contract without embedding any protocol product or driver identifiers.
type DevicePluginAdapter interface {
	OwnsDevice(*store.Device) bool
	OwnsDeviceMeta(types.DeviceMeta) bool
	ApplyDeviceDefaults(*store.Device)
	PrepareGatewayDevice(gateway, device *store.Device) *store.Device
	PrepareGatewayTelemetry(gatewayCode string, event types.Event, device *store.Device) types.Event
	GatewayCommandDeviceCode(types.DeviceMeta) string
	SkipRestartForDeviceUpdate(oldDevice, newDevice *store.Device) bool
	HandleGatewayDeviceEvent(server *Server, gatewayCode, eventID string, params map[string]interface{}) (bool, error)
}

func (pm *PluginManager) devicePluginAdapter(device *store.Device) DevicePluginAdapter {
	for _, plugin := range pm.GetPlugins() {
		if !plugin.IsEnabled() {
			continue
		}
		if adapter, ok := plugin.(DevicePluginAdapter); ok && adapter.OwnsDevice(device) {
			return adapter
		}
	}
	return nil
}

func (pm *PluginManager) ApplyDeviceDefaults(device *store.Device) {
	if adapter := pm.devicePluginAdapter(device); adapter != nil {
		adapter.ApplyDeviceDefaults(device)
	}
}

func (pm *PluginManager) PrepareGatewayDevice(gateway, device *store.Device) *store.Device {
	if adapter := pm.devicePluginAdapter(device); adapter != nil {
		return adapter.PrepareGatewayDevice(gateway, device)
	}
	return nil
}

func (pm *PluginManager) PrepareGatewayTelemetry(gatewayCode string, event types.Event, device *store.Device) types.Event {
	if adapter := pm.devicePluginAdapter(device); adapter != nil {
		return adapter.PrepareGatewayTelemetry(gatewayCode, event, device)
	}
	return event
}

func (pm *PluginManager) IsGatewayOwnedDevice(device *store.Device) bool {
	return pm.devicePluginAdapter(device) != nil
}

func (pm *PluginManager) GatewayCommandDeviceCode(device types.DeviceMeta) string {
	for _, plugin := range pm.GetPlugins() {
		if !plugin.IsEnabled() {
			continue
		}
		if adapter, ok := plugin.(DevicePluginAdapter); ok && adapter.OwnsDeviceMeta(device) {
			return adapter.GatewayCommandDeviceCode(device)
		}
	}
	return device.DeviceCode
}

func (pm *PluginManager) SkipRestartForDeviceUpdate(oldDevice, newDevice *store.Device) bool {
	if adapter := pm.devicePluginAdapter(newDevice); adapter != nil {
		return adapter.SkipRestartForDeviceUpdate(oldDevice, newDevice)
	}
	return false
}

func (pm *PluginManager) HandleGatewayDeviceEvent(gatewayCode, eventID string, params map[string]interface{}) (bool, error) {
	for _, plugin := range pm.GetPlugins() {
		if !plugin.IsEnabled() {
			continue
		}
		if adapter, ok := plugin.(DevicePluginAdapter); ok {
			if handled, err := adapter.HandleGatewayDeviceEvent(pm.Server, gatewayCode, eventID, params); handled || err != nil {
				return handled, err
			}
		}
	}
	return false, nil
}
