package core

import (
	"context"
	"noyo/core/platform"
	"noyo/core/pool"
	"noyo/core/types"
	"sync"
	"time"

	"go.uber.org/zap"
)

// dispatchJob represents a task to send data to a plugin
type dispatchJob struct {
	plugin      platform.IPlatformPlugin
	deviceCode  string
	productCode string
	dataType    string // "status", "property", "event"
	timestamp   int64
	payload     interface{} // raw payload from event
	uniqueId    string      // for event type
}

// DispatchService handles data dispatching to platform plugins
type DispatchService struct {
	Manager       *PluginManager
	Registry      *DeviceRegistry
	EventBus      *EventBus
	Logger        *zap.Logger
	dispatchQueue chan dispatchJob
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// NewDispatchService creates a new DispatchService
func NewDispatchService(manager *PluginManager, registry *DeviceRegistry, eventBus *EventBus, logger *zap.Logger) *DispatchService {
	ctx, cancel := context.WithCancel(context.Background())
	return &DispatchService{
		Manager:       manager,
		Registry:      registry,
		EventBus:      eventBus,
		Logger:        logger,
		dispatchQueue: make(chan dispatchJob, 10000), // Large buffer for dispatch
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start subscribes to events and starts managing dispatch
func (s *DispatchService) Start() {
	// Start Worker Pool
	workers := 50 // Configurable?
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	s.EventBus.Subscribe(types.EventDeviceStatusChanged, s.handleDeviceStatusChanged)
	s.EventBus.Subscribe(types.EventPropertyReported, s.handlePropertyReported)
	s.EventBus.Subscribe(types.EventEventReported, s.handleEventReported)
	s.Logger.Info("DispatchService started listening to events", zap.Int("workers", workers))
}

// Stop shuts down the dispatch service
func (s *DispatchService) Stop() {
	s.cancel()
	s.wg.Wait()
	s.Logger.Info("DispatchService stopped")
}

func (s *DispatchService) worker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case job := <-s.dispatchQueue:
			s.processJob(job)
		}
	}
}

func (s *DispatchService) processJob(job dispatchJob) {
	// Prepare DataModel
	data := pool.DataModelPool.Get()
	defer pool.DataModelPool.Put(data)

	data.DeviceCode = job.deviceCode
	data.ProductCode = job.productCode
	data.Type = job.dataType
	data.Timestamp = job.timestamp
	data.UniqueId = job.uniqueId // Only for event

	// Map Payload
	switch job.dataType {
	case types.DataTypeStatus:
		if status, ok := job.payload.(string); ok {
			data.Payload["status"] = status
		}
	case types.DataTypeProperty:
		if props, ok := job.payload.(map[string]interface{}); ok {
			for k, v := range props {
				data.Payload[k] = v
			}
		}
	case types.DataTypeEvent:
		if payloadMap, ok := job.payload.(map[string]interface{}); ok {
			if params, ok := payloadMap["params"].(map[string]interface{}); ok {
				for k, v := range params {
					data.Payload[k] = v
				}
			}
		}
	}

	// Push to plugin
	// Recover to be safe? Plugin might panic.
	// But PushData should be safe.
	if err := job.plugin.PushData(data); err != nil {
		s.Logger.Error("Plugin PushData failed",
			zap.String("plugin", job.plugin.GetMeta().Name),
			zap.Error(err),
		)
	}
}

func (s *DispatchService) getPlatformPlugins() []platform.IPlatformPlugin {
	return s.Manager.GetPlatformPlugins()
}

func (s *DispatchService) getProductCode(deviceCode string) string {
	if device, ok := s.Registry.GetDevice(deviceCode); ok {
		return device.ProductCode
	}
	return ""
}

// pushJob enqueues a job with non-blocking logic (or timeout)
func (s *DispatchService) pushJob(job dispatchJob) {
	select {
	case s.dispatchQueue <- job:
	case <-time.After(100 * time.Millisecond): // Unified timeout with EventBus
		s.Logger.Warn("DispatchQueue full, dropping event",
			zap.String("device", job.deviceCode),
			zap.String("type", job.dataType))
	}
}

func (s *DispatchService) handleDeviceStatusChanged(event types.Event) {
	status, ok := event.Payload.(string)
	if !ok {
		return
	}

	productCode := s.getProductCode(event.Topic)
	plugins := s.getPlatformPlugins()

	for _, p := range plugins {
		if !p.IsEnabled() {
			continue
		}
		s.pushJob(dispatchJob{
			plugin:      p,
			deviceCode:  event.Topic,
			productCode: productCode,
			dataType:    types.DataTypeStatus,
			timestamp:   event.Timestamp,
			payload:     status,
		})
	}
}

func (s *DispatchService) handlePropertyReported(event types.Event) {
	props, ok := event.Payload.(map[string]interface{})
	if !ok {
		return
	}

	productCode := s.getProductCode(event.Topic)
	plugins := s.getPlatformPlugins()

	for _, p := range plugins {
		if !p.IsEnabled() {
			continue
		}
		s.pushJob(dispatchJob{
			plugin:      p,
			deviceCode:  event.Topic,
			productCode: productCode,
			dataType:    types.DataTypeProperty,
			timestamp:   event.Timestamp,
			payload:     props,
		})
	}
}

func (s *DispatchService) handleEventReported(event types.Event) {
	payloadMap, ok := event.Payload.(map[string]interface{})
	if !ok {
		return
	}
	eventId, _ := payloadMap["eventId"].(string)
	// params is inside payloadMap, we pass the whole payloadMap to processJob to extract params
	// Or we extract params here?
	// The original code passed params to PushData.
	// Let's pass payloadMap to processJob and let it extract params.

	productCode := s.getProductCode(event.Topic)
	plugins := s.getPlatformPlugins()

	for _, p := range plugins {
		if !p.IsEnabled() {
			continue
		}
		s.pushJob(dispatchJob{
			plugin:      p,
			deviceCode:  event.Topic,
			productCode: productCode,
			dataType:    types.DataTypeEvent,
			timestamp:   event.Timestamp,
			uniqueId:    eventId,
			payload:     payloadMap,
		})
	}
}
