package tsdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const maxPendingRecords = 100000
const maxPendingBytes = 256 << 20
const maxRecordBytes = 1 << 20

var ErrPendingFull = errors.New("history pending storage is full")
var ErrManagerStopped = errors.New("history manager is not accepting records")

type pendingRecord struct {
	ID   int64  `gorm:"primaryKey;autoIncrement"`
	Data []byte `gorm:"type:blob"`
}

// pendingSpool acknowledges ingestion only after a synchronous durable commit.
// A crash between the month commit and ack is safe because IngestID is unique.
type pendingSpool struct {
	mu    sync.Mutex
	db    *gorm.DB
	count int64
	bytes int64
}

func openPendingSpool(dir string) (*pendingSpool, error) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "pending.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), DefaultContextTimeout: 5 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open history pending storage: %w", err)
	}
	conn, err := db.DB()
	if err != nil {
		return nil, err
	}
	conn.SetMaxOpenConns(1)
	closeOnError := func(err error) (*pendingSpool, error) { _ = conn.Close(); return nil, err }
	for _, pragma := range []string{"PRAGMA journal_mode=WAL", "PRAGMA synchronous=FULL", "PRAGMA busy_timeout=5000"} {
		if err := db.Exec(pragma).Error; err != nil {
			return closeOnError(err)
		}
	}
	if err := db.AutoMigrate(&pendingRecord{}); err != nil {
		return closeOnError(err)
	}
	s := &pendingSpool{db: db}
	var totals struct {
		Count int64
		Bytes int64
	}
	if err := db.Model(&pendingRecord{}).Select("COUNT(*) AS count, COALESCE(SUM(LENGTH(data)),0) AS bytes").Scan(&totals).Error; err != nil {
		return closeOnError(err)
	}
	s.count, s.bytes = totals.Count, totals.Bytes
	return s, nil
}

func (s *pendingSpool) append(record *Record) error {
	if record == nil {
		return errors.New("nil history record")
	}
	if len(record.Payload) > maxRecordBytes {
		return errors.New("history record exceeds 1 MiB")
	}
	// The snapshot isolates asynchronous writes from caller mutations.
	copyRecord := *record
	copyRecord.ID = 0
	key, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	keyString := key.String()
	copyRecord.IngestID = &keyString
	data, err := json.Marshal(copyRecord)
	if err != nil {
		return fmt.Errorf("encode history record: %w", err)
	}
	if len(data) > maxRecordBytes {
		return errors.New("history record exceeds 1 MiB")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.count >= maxPendingRecords || s.bytes+int64(len(data)) > maxPendingBytes {
		return ErrPendingFull
	}
	if err := s.db.Create(&pendingRecord{Data: data}).Error; err != nil {
		return fmt.Errorf("persist pending history: %w", err)
	}
	s.count++
	s.bytes += int64(len(data))
	return nil
}

func (s *pendingSpool) next(limit int) ([]pendingRecord, []*Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var pending []pendingRecord
	if err := s.db.Order("id ASC").Limit(limit).Find(&pending).Error; err != nil {
		return nil, nil, err
	}
	records := make([]*Record, 0, len(pending))
	for _, entry := range pending {
		var record Record
		if err := json.Unmarshal(entry.Data, &record); err != nil {
			return nil, nil, fmt.Errorf("decode pending history %d: %w", entry.ID, err)
		}
		if record.IngestID == nil || *record.IngestID == "" {
			return nil, nil, fmt.Errorf("pending history %d has no ingestion identity", entry.ID)
		}
		records = append(records, &record)
	}
	return pending, records, nil
}

func (s *pendingSpool) ack(entries []pendingRecord) error {
	if len(entries) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]int64, len(entries))
	var bytes int64
	for i, entry := range entries {
		ids[i] = entry.ID
		bytes += int64(len(entry.Data))
	}
	if err := s.db.Where("id IN ?", ids).Delete(&pendingRecord{}).Error; err != nil {
		return err
	}
	s.count -= int64(len(entries))
	s.bytes -= bytes
	return nil
}

func (s *pendingSpool) size() int64 { s.mu.Lock(); defer s.mu.Unlock(); return s.count }
func (s *pendingSpool) close() error {
	db, err := s.db.DB()
	if err != nil {
		return err
	}
	return db.Close()
}
