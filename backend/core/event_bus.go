package core

import (
	"context"
	"noyo/core/types"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// EventHandler is a function that handles an event
type EventHandler func(event types.Event)

type job struct {
	handler EventHandler
	event   types.Event
}

// subscription holds a handler with its unique ID
type subscription struct {
	id      uint64
	handler EventHandler
}

// EventBus manages event subscriptions and publishing
type EventBus struct {
	mu             sync.RWMutex
	subscribers    map[types.EventType][]subscription
	nextID         atomic.Uint64
	jobQueue       chan job
	workerPoolSize int
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	publishWG      sync.WaitGroup
	closed         bool
	closeOnce      sync.Once
	closeDone      chan struct{}
	logger         *zap.Logger
}

// NewEventBus creates a new EventBus
func NewEventBus(logger *zap.Logger) *EventBus {
	// Standard configuration
	poolSize := 100 // Limit concurrent goroutines to 100
	queueSize := 5000

	ctx, cancel := context.WithCancel(context.Background())

	eb := &EventBus{
		subscribers:    make(map[types.EventType][]subscription),
		jobQueue:       make(chan job, queueSize),
		workerPoolSize: poolSize,
		ctx:            ctx,
		cancel:         cancel,
		logger:         logger,
		closeDone:      make(chan struct{}),
	}

	// Start Workers
	eb.startWorkers()

	return eb
}

func (eb *EventBus) startWorkers() {
	for i := 0; i < eb.workerPoolSize; i++ {
		eb.wg.Add(1)
		go eb.worker()
	}
}

func (eb *EventBus) worker() {
	defer eb.wg.Done()
	for {
		select {
		case <-eb.ctx.Done():
			return
		case j, ok := <-eb.jobQueue:
			if !ok {
				return
			}
			// Execute handler safe
			func() {
				defer func() {
					if r := recover(); r != nil {
						eb.logger.Error("EventBus worker panic", zap.Any("recover", r))
					}
				}()
				j.handler(j.event)
			}()
		}
	}
}

// Subscribe subscribes a handler to a specific event type (legacy, no unsubscribe support)
func (eb *EventBus) Subscribe(eventType types.EventType, handler EventHandler) {
	eb.SubscribeWithID(eventType, handler)
}

// SubscribeWithID subscribes a handler and returns a unique subscription ID for later unsubscription
func (eb *EventBus) SubscribeWithID(eventType types.EventType, handler EventHandler) uint64 {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if eb.closed || handler == nil {
		return 0
	}
	id := eb.nextID.Add(1)
	eb.subscribers[eventType] = append(eb.subscribers[eventType], subscription{id: id, handler: handler})
	return id
}

// HasSubscribers reports whether a running bus has an installed capability
// consumer. Delivery may still fail or time out after this availability check.
func (eb *EventBus) HasSubscribers(eventType types.EventType) bool {
	if eb == nil {
		return false
	}
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return !eb.closed && len(eb.subscribers[eventType]) > 0
}

// Unsubscribe removes a subscription by its ID
func (eb *EventBus) Unsubscribe(eventType types.EventType, id uint64) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	subs := eb.subscribers[eventType]
	for i, sub := range subs {
		if sub.id == id {
			eb.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
			return
		}
	}
}

// Publish publishes an event to all subscribers asynchronously via the worker pool
func (eb *EventBus) Publish(event types.Event) {
	eb.mu.RLock()
	if eb.closed {
		eb.mu.RUnlock()
		return
	}
	eb.publishWG.Add(1)
	defer eb.publishWG.Done()
	subs := eb.subscribers[event.Type]
	// Copy handlers to avoid holding lock during enqueue
	handlers := make([]EventHandler, len(subs))
	for i, s := range subs {
		handlers[i] = s.handler
	}
	eb.mu.RUnlock()

	for _, handler := range handlers {
		select {
		case eb.jobQueue <- job{handler: handler, event: event}:
		case <-time.After(100 * time.Millisecond):
			// Log dropped event
			eb.logger.Warn("EventBus dropped event due to full queue",
				zap.String("type", string(event.Type)),
				zap.String("topic", event.Topic))
		case <-eb.ctx.Done():
			return
		}
	}
}

// PublishSync publishes an event to all subscribers synchronously
func (eb *EventBus) PublishSync(event types.Event) {
	eb.mu.RLock()
	if eb.closed {
		eb.mu.RUnlock()
		return
	}
	eb.publishWG.Add(1)
	defer eb.publishWG.Done()
	subs := eb.subscribers[event.Type]
	handlers := make([]EventHandler, len(subs))
	for i, s := range subs {
		handlers[i] = s.handler
	}
	eb.mu.RUnlock()

	for _, handler := range handlers {
		handler(event)
	}
}

// Close shuts down the event bus
func (eb *EventBus) Close() {
	_ = eb.CloseWithContext(context.Background())
}

// CloseWithContext stops new publications and drains accepted jobs. A timeout
// cancels workers without pretending that queued work was delivered. A handler
// that ignores cancellation may continue until it returns.
func (eb *EventBus) CloseWithContext(ctx context.Context) error {
	eb.closeOnce.Do(func() {
		eb.mu.Lock()
		eb.closed = true
		eb.mu.Unlock()
		go func() {
			eb.publishWG.Wait()
			close(eb.jobQueue)
			eb.wg.Wait()
			eb.cancel()
			close(eb.closeDone)
		}()
	})
	select {
	case <-eb.closeDone:
		return nil
	case <-ctx.Done():
		eb.cancel()
		return ctx.Err()
	}
}
