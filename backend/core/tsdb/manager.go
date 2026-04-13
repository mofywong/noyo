package tsdb

import (
	"noyo/core/config"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TSDBManager handles the time-series data lifecycle
type TSDBManager struct {
	cfg     config.TSDBConfig
	store   TimeSeriesStore // Abstract Interface
	logger  *zap.Logger
	dataCh  chan *Record
	stopCh  chan struct{}
	wg      sync.WaitGroup
	running bool
}

// NewManager creates a new TSDB manager
func NewManager(cfg config.TSDBConfig, logger *zap.Logger) *TSDBManager {
	if cfg.Dir == "" {
		cfg.Dir = "./data/db/history"
	}
	// Default to SQLiteStore
	store := NewSQLiteStore(cfg.Dir, logger)

	return &TSDBManager{
		cfg:    cfg,
		store:  store,
		logger: logger,
		dataCh: make(chan *Record, 5000), // Buffer
		stopCh: make(chan struct{}),
	}
}

// Init initializes the store
func (m *TSDBManager) Init() error {
	if !m.cfg.Enabled {
		return nil
	}
	return m.store.Init()
}

// Start begins the background workers
func (m *TSDBManager) Start() {
	if !m.cfg.Enabled || m.running {
		return
	}
	m.running = true
	m.wg.Add(2)

	// 1. Writer Worker
	go m.writerLoop()

	// 2. Cleaner Worker
	go m.cleanerLoop()

	m.logger.Info("TSDB Manager started", zap.String("dir", m.cfg.Dir))
}

// Stop gracefully shuts down the manager
func (m *TSDBManager) Stop() {
	if !m.running {
		return
	}
	close(m.stopCh)
	m.wg.Wait()
	m.store.Close()
	m.running = false
	m.logger.Info("TSDB Manager stopped")
}

// Push adds a record to the write queue
func (m *TSDBManager) Push(r *Record) {
	if !m.cfg.Enabled {
		return
	}
	select {
	case m.dataCh <- r:
	default:
		m.logger.Warn("TSDB Buffer full, dropping record", zap.String("device", r.DeviceCode))
	}
}

// writerLoop handles batch writing
func (m *TSDBManager) writerLoop() {
	defer m.wg.Done()

	batchSize := m.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}
	flushInterval := time.Duration(m.cfg.FlushIntervalMs) * time.Millisecond
	if flushInterval <= 0 {
		flushInterval = 1 * time.Second
	}

	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	var batch []*Record

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := m.store.WriteBatch(batch); err != nil {
			m.logger.Error("Failed to write TSDB batch", zap.Error(err))
		}
		// Reset batch
		batch = make([]*Record, 0, batchSize)
	}

	for {
		select {
		case r := <-m.dataCh:
			batch = append(batch, r)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-m.stopCh:
			flush() // Final flush
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
