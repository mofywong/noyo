package core

import (
	"noyo/core/types"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestEventBus_WorkerPool(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	eb := NewEventBus(logger)
	defer eb.Close()

	if eb.workerPoolSize != 100 {
		t.Errorf("Expected pool size 100, got %d", eb.workerPoolSize)
	}

	receivedCount := 0
	var mu sync.Mutex
	wg := sync.WaitGroup{}

	totalEvents := 200
	wg.Add(totalEvents)

	handler := func(event types.Event) {
		mu.Lock()
		receivedCount++
		mu.Unlock()
		wg.Done()
	}

	eb.Subscribe("test_event", handler)

	for i := 0; i < totalEvents; i++ {
		eb.Publish(types.Event{
			Type:    "test_event",
			Payload: i,
		})
	}

	// Wait for all events to be processed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for events")
	}

	mu.Lock()
	if receivedCount != totalEvents {
		t.Errorf("Expected %d events, got %d", totalEvents, receivedCount)
	}
	mu.Unlock()
}
