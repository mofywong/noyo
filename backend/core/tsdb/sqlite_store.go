package tsdb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// SQLiteStore implements TimeSeriesStore using rolling SQLite databases
type SQLiteStore struct {
	baseDir string
	logger  *zap.Logger
	dbs     map[string]*gorm.DB // Cache: "202310" -> *gorm.DB
	mu      sync.RWMutex
}

// NewSQLiteStore creates a new SQLiteStore
func NewSQLiteStore(baseDir string, log *zap.Logger) *SQLiteStore {
	return &SQLiteStore{
		baseDir: baseDir,
		logger:  log,
		dbs:     make(map[string]*gorm.DB),
	}
}

// Init ensures the data directory exists
func (s *SQLiteStore) Init() error {
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create tsdb directory: %w", err)
	}
	return nil
}

// Close closes all open database connections
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var firstErr error
	for k, db := range s.dbs {
		sqlDB, err := db.DB()
		if err == nil {
			if e := sqlDB.Close(); e != nil && firstErr == nil {
				firstErr = e
			}
		}
		delete(s.dbs, k)
	}
	return firstErr
}

// getDB returns a DB instance for the specific month (e.g. "202310")
func (s *SQLiteStore) getDB(monthStr string) (*gorm.DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.getDBContext(ctx, monthStr)
}

func (s *SQLiteStore) getDBContext(ctx context.Context, monthStr string) (*gorm.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	if db, ok := s.dbs[monthStr]; ok {
		s.mu.RUnlock()
		return db, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Double check
	if db, ok := s.dbs[monthStr]; ok {
		return db, nil
	}

	// Open new DB
	dbPath := filepath.Join(s.baseDir, fmt.Sprintf("history_%s.db", monthStr))

	// GORM Config: Silent logger to avoid noise
	gormConfig := &gorm.Config{
		Logger:                logger.Default.LogMode(logger.Silent),
		DefaultContextTimeout: 5 * time.Second,
	}

	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open tsdb file %s: %w", dbPath, err)
	}

	// WAL Mode for concurrency
	if err := db.WithContext(ctx).Exec("PRAGMA journal_mode=WAL;").Error; err != nil {
		if conn, closeErr := db.DB(); closeErr == nil {
			_ = conn.Close()
		}
		return nil, fmt.Errorf("failed to enable history WAL: %w", err)
	}

	// Auto Migrate
	if err := db.WithContext(ctx).AutoMigrate(&Record{}); err != nil {
		if conn, closeErr := db.DB(); closeErr == nil {
			_ = conn.Close()
		}
		return nil, fmt.Errorf("failed to migrate tsdb table: %w", err)
	}

	s.dbs[monthStr] = db
	return db, nil
}

// getMonthStr returns "YYYYMM" from timestamp (ms)
func (s *SQLiteStore) getMonthStr(ts int64) string {
	t := time.UnixMilli(ts)
	return t.Format("200601")
}

// WriteBatch writes a batch of records
func (s *SQLiteStore) WriteBatch(records []*Record) error {
	return s.WriteBatchContext(context.Background(), records)
}

// WriteBatchContext allows shutdown to share a deadline across every month.
func (s *SQLiteStore) WriteBatchContext(parent context.Context, records []*Record) error {
	// Group records by month to minimize DB switching/transactions
	groups := make(map[string][]*Record)
	for _, r := range records {
		if r == nil {
			return errors.New("nil history record")
		}
		if r.IngestID == nil || *r.IngestID == "" {
			key, err := uuid.NewRandom()
			if err != nil {
				return err
			}
			keyString := key.String()
			r.IngestID = &keyString
		}
		month := s.getMonthStr(r.Ts)
		groups[month] = append(groups[month], r)
	}

	var writeErrors []error
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()
	for month, recs := range groups {
		if err := ctx.Err(); err != nil {
			writeErrors = append(writeErrors, err)
			break
		}
		db, err := s.getDBContext(ctx, month)
		if err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("open history month %s: %w", month, err))
			continue
		}
		// SQLite can commit one month before another fails. Replay the same
		// ingestion identities, never month-local row IDs, after such a failure.
		copies := make([]*Record, len(recs))
		for i, record := range recs {
			copyRecord := *record
			copyRecord.ID = 0
			copies[i] = &copyRecord
		}
		err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "ingest_id"}}, DoNothing: true}).CreateInBatches(copies, 100).Error
		})
		if err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("insert history month %s: %w", month, err))
			continue
		}
		// RETURNING rows omit conflicts, so a mixed replay/new batch cannot
		// safely map returned IDs by slice position.
		keys := make([]string, len(recs))
		for i, record := range recs {
			keys[i] = *record.IngestID
		}
		var identities []Record
		if err := db.WithContext(ctx).Select("id,ingest_id").Where("ingest_id IN ?", keys).Find(&identities).Error; err != nil {
			writeErrors = append(writeErrors, fmt.Errorf("read committed history identities %s: %w", month, err))
			continue
		}
		ids := make(map[string]int64, len(identities))
		for _, record := range identities {
			if record.IngestID != nil {
				ids[*record.IngestID] = record.ID
			}
		}
		for _, record := range recs {
			record.ID = ids[*record.IngestID]
		}
	}
	return errors.Join(writeErrors...)
}

// Prune removes old files
func (s *SQLiteStore) Prune(retentionDays int) {
	if retentionDays <= 0 {
		return
	}

	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		s.logger.Error("Prune: failed to read dir", zap.Error(err))
		return
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		// Basic check
		if len(name) < 15 || name[:8] != "history_" {
			continue
		}

		info, err := f.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			s.logger.Info("Pruning old TSDB file", zap.String("file", name))
			fullPath := filepath.Join(s.baseDir, name)
			os.Remove(fullPath)
		}
	}
}

func (s *SQLiteStore) getMonthsInRange(start, end int64) []string {
	if start > end {
		return []string{}
	}
	startT := time.UnixMilli(start)
	endT := time.UnixMilli(end)
	months := []string{}
	curr := time.Date(startT.Year(), startT.Month(), 1, 0, 0, 0, 0, startT.Location())
	for !curr.After(endT) {
		m := curr.Format("200601")
		if len(months) == 0 || months[len(months)-1] != m {
			months = append(months, m)
		}
		curr = curr.AddDate(0, 1, 0)
		curr = time.Date(curr.Year(), curr.Month(), 1, 0, 0, 0, 0, curr.Location())
	}
	endM := endT.Format("200601")
	if len(months) == 0 || months[len(months)-1] != endM {
		months = append(months, endM)
	}
	return months
}

// DeleteRecord deletes a record by its ID. The ts is used to locate the correct monthly database.
func (s *SQLiteStore) DeleteRecord(id int64, ts int64) error {
	monthStr := s.getMonthStr(ts)
	db, err := s.getDB(monthStr)
	if err != nil {
		return fmt.Errorf("failed to get DB for month %s: %w", monthStr, err)
	}
	result := db.Delete(&Record{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("record not found: id=%d", id)
	}
	s.logger.Info("Deleted TSDB record", zap.Int64("id", id), zap.String("month", monthStr))
	return nil
}
