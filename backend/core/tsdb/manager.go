package tsdb

import (
	"context"
	"errors"
	"fmt"
	"noyo/core/config"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TSDBManager handles the time-series data lifecycle
type TSDBManager struct {
	cfg       config.TSDBConfig
	store     TimeSeriesStore // Abstract Interface
	logger    *zap.Logger
	spool     *pendingSpool
	wakeCh    chan struct{}
	stopCh    chan struct{}
	stopDone  chan struct{}
	wg        sync.WaitGroup
	mu        sync.Mutex
	running   bool
	stopping  bool
	stopErr   error
	writerErr error
}

// NewManager creates a new TSDB manager
func NewManager(cfg config.TSDBConfig, logger *zap.Logger) *TSDBManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg.Dir == "" {
		cfg.Dir = "./data/db/history"
	}
	// Default to SQLiteStore
	store := NewSQLiteStore(cfg.Dir, logger)

	return &TSDBManager{
		cfg:    cfg,
		store:  store,
		logger: logger,
		wakeCh: make(chan struct{}, 1),
		stopCh: make(chan struct{}),
	}
}

// Init initializes the store
func (m *TSDBManager) Init() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		return nil
	}
	if m.spool != nil {
		return nil
	}
	if err := m.store.Init(); err != nil {
		return err
	}
	spool, err := openPendingSpool(m.cfg.Dir)
	if err != nil {
		_ = m.store.Close()
		return err
	}
	m.spool = spool
	return nil
}

// Start begins the background workers
func (m *TSDBManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled || m.running {
		return nil
	}
	if m.stopping {
		return ErrManagerStopped
	}
	if m.spool == nil {
		return errors.New("history manager must be initialized before starting")
	}
	m.stopCh = make(chan struct{})
	m.stopDone = make(chan struct{})
	m.wakeCh = make(chan struct{}, 1)
	m.stopErr, m.writerErr = nil, nil
	m.running = true
	m.wg.Add(2)

	// 1. Writer Worker
	go m.writerLoop()

	// 2. Cleaner Worker
	go m.cleanerLoop()

	m.logger.Info("TSDB Manager started", zap.String("dir", m.cfg.Dir))
	m.wakeCh <- struct{}{} // Replay previously accepted records before waiting for new input.
	return nil
}

// Stop gracefully shuts down the manager
func (m *TSDBManager) Stop() error {
	m.mu.Lock()
	if m.stopping {
		done := m.stopDone
		m.mu.Unlock()
		<-done
		m.mu.Lock()
		err := m.stopErr
		m.mu.Unlock()
		return err
	}
	if !m.running {
		if m.spool != nil {
			m.stopErr = errors.Join(m.spool.close(), m.store.Close())
			m.spool = nil
		}
		err := m.stopErr
		m.mu.Unlock()
		return err
	}
	m.running = false // Push stops accepting before the writer's final drain.
	m.stopping = true
	close(m.stopCh)
	m.mu.Unlock()
	m.wg.Wait()
	closeErr := errors.Join(m.store.Close(), m.spool.close())
	m.mu.Lock()
	m.stopErr = errors.Join(m.writerErr, closeErr)
	m.spool = nil
	m.stopping = false
	close(m.stopDone)
	err := m.stopErr
	m.mu.Unlock()
	m.logger.Info("TSDB Manager stopped")
	return err
}

// Push adds a record to the write queue
func (m *TSDBManager) Push(r *Record) error {
	if !m.cfg.Enabled {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running || m.stopping {
		return ErrManagerStopped
	}
	if err := m.spool.append(r); err != nil {
		return err
	}
	select {
	case m.wakeCh <- struct{}{}:
	default:
	}
	return nil
}

// writerLoop handles batch writing
func (m *TSDBManager) writerLoop() {
	defer m.wg.Done()

	batchSize := m.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	if batchSize > 1000 {
		batchSize = 1000
	}
	flushInterval := time.Duration(m.cfg.FlushIntervalMs) * time.Millisecond
	if flushInterval <= 0 {
		flushInterval = 1 * time.Second
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	flush := func(shutdown bool) error {
		deadline := time.Now().Add(10 * time.Second)
		ctx, cancel := context.WithDeadline(context.Background(), deadline)
		defer cancel()
		for {
			if shutdown {
				if time.Now().After(deadline) && m.spool.size() > 0 {
					return errors.New("history drain timed out; pending records retained for restart")
				}
			} else {
				select {
				case <-m.stopCh:
					return nil
				default:
				}
			}
			pending, records, err := m.spool.next(batchSize)
			if err != nil {
				return err
			}
			if len(records) == 0 {
				return nil
			}
			var writeErr error
			if writer, ok := m.store.(interface {
				WriteBatchContext(context.Context, []*Record) error
			}); ok {
				writeErr = writer.WriteBatchContext(ctx, records)
			} else {
				writeErr = m.store.WriteBatch(records)
			}
			if writeErr != nil {
				return fmt.Errorf("history batch retained for retry: %w", writeErr)
			}
			if err := m.spool.ack(pending); err != nil {
				return fmt.Errorf("ack history batch (safe to replay): %w", err)
			}
		}
	}
	var retryAfter time.Time
	for {
		select {
		case <-m.wakeCh:
			if time.Now().Before(retryAfter) {
				continue
			}
			if m.spool.size() < int64(batchSize) {
				continue
			}
			if err := flush(false); err != nil {
				m.logger.Error("Failed to write TSDB batch", zap.Error(err))
				retryAfter = time.Now().Add(flushInterval)
			}
		case <-ticker.C:
			if err := flush(false); err != nil {
				m.logger.Error("Failed to write TSDB batch", zap.Error(err))
				retryAfter = time.Now().Add(flushInterval)
			}
		case <-m.stopCh:
			m.writerErr = flush(true)
			return
		}
	}
}

// cleanerLoop handles periodic cleanup
func (m *TSDBManager) cleanerLoop() {
	defer m.wg.Done()

	// Check once a day
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.store.Prune(m.cfg.RetentionDays)
		case <-m.stopCh:
			return
		}
	}
}

// Query executes a query
func (m *TSDBManager) Query(req QueryRequest) (*QueryResponse, error) {
	if !m.cfg.Enabled {
		return &QueryResponse{List: []interface{}{}}, nil
	}
	return m.store.Query(req)
}

// DeleteRecord deletes a record by its ID
func (m *TSDBManager) DeleteRecord(id int64, ts int64) error {
	if !m.cfg.Enabled {
		return fmt.Errorf("TSDB is disabled")
	}
	return m.store.DeleteRecord(id, ts)
}
