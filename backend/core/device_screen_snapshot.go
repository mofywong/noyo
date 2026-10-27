package core

import (
	"noyo/core/types"
	"time"
)

const screenRuntimeDeviceUpdated types.EventType = "device.telemetry.updated"

// ScreenSample uses a cache sequence, never a device clock, to order values.
type ScreenSample struct {
	Value         interface{} `json:"value"`
	SampleVersion uint64      `json:"sampleVersion"`
	ReceivedAt    int64       `json:"receivedAt"`
}

type ScreenDeviceSnapshot struct {
	Samples          map[string]ScreenSample `json:"samples"`
	Status           DeviceStatus            `json:"status"`
	StatusVersion    uint64                  `json:"statusVersion"`
	StatusReceivedAt int64                   `json:"statusReceivedAt"`
}

// recordScreenSamplesLocked is called under the same device lock as Properties.
// Repeated values advance metadata as well, so a stable sensor is not stale.
func recordScreenSamplesLocked(rt *DeviceRuntime, data map[string]interface{}) map[string]ScreenSample {
	if rt.ScreenSamples == nil {
		rt.ScreenSamples = make(map[string]ScreenSample)
	}
	rt.ScreenVersion++
	now := time.Now().UnixMilli()
	updates := make(map[string]ScreenSample, len(data))
	for key, value := range data {
		sample := ScreenSample{Value: value, SampleVersion: rt.ScreenVersion, ReceivedAt: now}
		rt.ScreenSamples[key] = sample
		updates[key] = sample
	}
	return updates
}

// SeedScreenSamples restores the last known values after a restart so published
// screens are not blank until every device happens to report again. The stored
// timestamp is the record's own, never "now": an old value must be reported as
// stale, not offered as current. A value already in memory always wins, because
// it arrived from a live report after the warm-up query was issued.
func (dm *DeviceManager) SeedScreenSamples(deviceCode string, values map[string]interface{}, receivedAt int64) bool {
	if len(values) == 0 || receivedAt <= 0 {
		return false
	}
	value, ok := dm.runtimes.Load(deviceCode)
	if !ok {
		return false
	}
	rt, ok := value.(*DeviceRuntime)
	if !ok || rt == nil {
		return false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.ScreenSamples == nil {
		rt.ScreenSamples = make(map[string]ScreenSample, len(values))
	}
	pending := make(map[string]interface{}, len(values))
	for key, item := range values {
		if _, exists := rt.ScreenSamples[key]; !exists {
			pending[key] = item
		}
	}
	if len(pending) == 0 {
		return false
	}
	rt.ScreenVersion++
	for key, item := range pending {
		rt.ScreenSamples[key] = ScreenSample{Value: item, SampleVersion: rt.ScreenVersion, ReceivedAt: receivedAt}
	}
	return true
}

func (dm *DeviceManager) GetLatestSnapshot(deviceCode string) ScreenDeviceSnapshot {
	result := ScreenDeviceSnapshot{Samples: map[string]ScreenSample{}}
	value, ok := dm.runtimes.Load(deviceCode)
	if !ok {
		return result
	}
	rt := value.(*DeviceRuntime)
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	for key, sample := range rt.ScreenSamples {
		result.Samples[key] = sample
	}
	result.Status, result.StatusVersion, result.StatusReceivedAt = rt.Status, rt.ScreenStatusVersion, rt.ScreenStatusReceived
	return result
}

func (dm *DeviceManager) publishScreenRuntimeChange(deviceCode string, samples map[string]ScreenSample) {
	if dm.EventBus == nil || !dm.EventBus.HasSubscribers(screenRuntimeDeviceUpdated) {
		return
	}
	// This dedicated channel has only nonblocking screen wake-up consumers. Sync
	// delivery avoids the general-purpose worker queue silently dropping hints.
	dm.EventBus.PublishSync(types.Event{Type: screenRuntimeDeviceUpdated, Topic: deviceCode, Payload: samples, Timestamp: time.Now().UnixMilli()})
}
