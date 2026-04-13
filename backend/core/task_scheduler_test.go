package core

import (
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestTaskScheduler_Cron(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	var mu sync.Mutex
	callCount := 0

	updater := func(deviceCode string, err error) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}

	ts := NewTaskScheduler(logger, updater)
	ts.Init()
	defer ts.Shutdown()

	defs := []TaskDefinition{
		{
			Name:     "TestTask",
			Interval: "@every 1s",
			Handler: func() error {
				return nil
			},
		},
	}

	err := ts.StartDeviceTasks("dev1", defs)
	if err != nil {
		t.Fatalf("Failed to start tasks: %v", err)
	}

	// Wait for 2 seconds (should run at least once, maybe twice)
	time.Sleep(2500 * time.Millisecond)

	mu.Lock()
	count := callCount
	mu.Unlock()

	if count < 2 {
		t.Errorf("Expected at least 2 executions, got %d", count)
	}

	// Stop
	ts.StopDeviceTasks("dev1")

	// Wait more
	time.Sleep(2000 * time.Millisecond)

	mu.Lock()
	countAfterStop := callCount
	mu.Unlock()

	if countAfterStop > count+1 { // Allow 1 race if it was just starting
		t.Errorf("Tasks continued after stop: %d -> %d", count, countAfterStop)
	}
}

func TestTaskScheduler_ConcurrencyLimit(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	var mu sync.Mutex
	executedCount := 0

	updater := func(deviceCode string, err error) {
		mu.Lock()
		executedCount++
		mu.Unlock()
	}

	ts := NewTaskScheduler(logger, updater)
	ts.Init()
	defer ts.Shutdown()

	// Fill the worker pool to max
	// Max is 1000.
	// We'll manually consume 1000 slots to simulate busy
	for i := 0; i < 1000; i++ {
		ts.workerPool <- struct{}{}
	}

	// Now try to run a task
	// It should skip immediately
	defs := []TaskDefinition{
		{
			Name:     "SkipTask",
			Interval: "@every 1s",
			Handler: func() error {
				return nil
			},
		},
	}

	ts.StartDeviceTasks("busy_dev", defs)

	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	count := executedCount
	mu.Unlock()

	if count > 0 {
		t.Errorf("Expected 0 executions because pool is full, got %d", count)
	}

	// Drain pool
	for i := 0; i < 1000; i++ {
		<-ts.workerPool
	}
}
