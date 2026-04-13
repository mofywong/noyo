package core

import (
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// StatusUpdater is a callback to report task execution status
type StatusUpdater func(deviceCode string, err error)

// TaskScheduler manages device polling tasks using a centralized cron scheduler
type TaskScheduler struct {
	mu            sync.RWMutex
	cron          *cron.Cron
	deviceEntries map[string][]cron.EntryID // Track cron entries per device
	Logger        *zap.Logger
	StatusUpdater StatusUpdater
	workerPool    chan struct{} // Global semaphore for concurrency limiting
}

// NewTaskScheduler creates a new TaskScheduler
func NewTaskScheduler(logger *zap.Logger, updater StatusUpdater) *TaskScheduler {
	// Use cron with seconds support
	c := cron.New(cron.WithSeconds())
	return &TaskScheduler{
		cron:          c,
		deviceEntries: make(map[string][]cron.EntryID),
		Logger:        logger,
		StatusUpdater: updater,
		workerPool:    make(chan struct{}, 1000), // Limit to 1000 concurrent task executions
	}
}

// Init initializes the task manager
func (ts *TaskScheduler) Init() {
	ts.cron.Start()
	ts.Logger.Info("TaskScheduler initialized and started")
}

// Shutdown shuts down the task manager
func (ts *TaskScheduler) Shutdown() {
	ts.Logger.Info("Stopping TaskScheduler...")
	ctx := ts.cron.Stop()
	// Wait for running jobs to complete
	select {
	case <-ctx.Done():
		ts.Logger.Info("TaskScheduler stopped gracefully")
	case <-time.After(5 * time.Second):
		ts.Logger.Warn("TaskScheduler stop timed out")
	}
}

// StartDeviceTasks starts tasks for a device
func (ts *TaskScheduler) StartDeviceTasks(deviceCode string, defs []TaskDefinition) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if _, exists := ts.deviceEntries[deviceCode]; exists {
		return fmt.Errorf("device %s tasks already running", deviceCode)
	}

	var entryIDs []cron.EntryID

	for _, def := range defs {
		// Capture variable for closure
		taskDef := def

		// Wrap handler for concurrency control and error reporting
		job := func() {
			// Try to acquire semaphore without blocking too long?
			// Cron runs in its own goroutine per job trigger.
			// Blocking here means the cron worker waits.
			// If we want to skip if busy:
			select {
			case ts.workerPool <- struct{}{}:
				// Acquired
				defer func() { <-ts.workerPool }()

				// Execute
				err := taskDef.Handler()

				// Report Status (仅对非 SkipStatusUpdate 的任务上报)
				if ts.StatusUpdater != nil && !taskDef.SkipStatusUpdate {
					// StatusUpdater is usually fast (event bus publish).
					// But to be safe properly:
					go ts.StatusUpdater(deviceCode, err)
				}

				if err != nil {
					ts.Logger.Error("Task execution failed",
						zap.String("device", deviceCode),
						zap.String("task", taskDef.Name),
						zap.Error(err))
				}

			default:
				// Busy
				ts.Logger.Warn("Task missed schedule due to high load",
					zap.String("device", deviceCode),
					zap.String("task", taskDef.Name))
			}
		}

		id, err := ts.cron.AddFunc(taskDef.Interval, job)
		if err != nil {
			// Rollback installed entries for this device
			for _, installedID := range entryIDs {
				ts.cron.Remove(installedID)
			}
			return fmt.Errorf("failed to schedule task %s: %w", taskDef.Name, err)
		}
		entryIDs = append(entryIDs, id)
	}

	ts.deviceEntries[deviceCode] = entryIDs
	ts.Logger.Info("Started tasks for device", zap.String("code", deviceCode), zap.Int("count", len(defs)))
	return nil
}

// StopDeviceTasks stops tasks for a device
func (ts *TaskScheduler) StopDeviceTasks(deviceCode string) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.stopDeviceTasksLocked(deviceCode)
}

func (ts *TaskScheduler) stopDeviceTasksLocked(deviceCode string) error {
	entryIDs, exists := ts.deviceEntries[deviceCode]
	if !exists {
		return nil
	}

	for _, id := range entryIDs {
		ts.cron.Remove(id)
	}
	delete(ts.deviceEntries, deviceCode)
	ts.Logger.Info("Stopped tasks for device", zap.String("code", deviceCode))
	return nil
}

// IsRunning checks if tasks are running for a device
func (ts *TaskScheduler) IsRunning(deviceCode string) bool {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	_, exists := ts.deviceEntries[deviceCode]
	return exists
}
