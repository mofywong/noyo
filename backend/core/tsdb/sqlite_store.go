package tsdb

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
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
	s.mu.RLock()
	if db, ok := s.dbs[monthStr]; ok {
		s.mu.RUnlock()
		return db, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check
	if db, ok := s.dbs[monthStr]; ok {
		return db, nil
	}

	// Open new DB
	dbPath := filepath.Join(s.baseDir, fmt.Sprintf("history_%s.db", monthStr))

	// GORM Config: Silent logger to avoid noise
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open tsdb file %s: %w", dbPath, err)
	}

	// WAL Mode for concurrency
	db.Exec("PRAGMA journal_mode=WAL;")

	// Auto Migrate
	if err := db.AutoMigrate(&Record{}); err != nil {
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
	// Group records by month to minimize DB switching/transactions
	groups := make(map[string][]*Record)
	for _, r := range records {
		month := s.getMonthStr(r.Ts)
		groups[month] = append(groups[month], r)
	}

	for month, recs := range groups {
		db, err := s.getDB(month)
		if err != nil {
			s.logger.Error("Failed to get DB for batch", zap.String("month", month), zap.Error(err))
			continue
		}

		// Transaction
		err = db.Transaction(func(tx *gorm.DB) error {
			if err := tx.CreateInBatches(recs, 100).Error; err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			s.logger.Error("Batch insert failed", zap.String("month", month), zap.Error(err))
		}
	}
	return nil
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

// Query executes a query
func (s *SQLiteStore) Query(req QueryRequest) (*QueryResponse, error) {
	//s.logger.Info("SQLiteStore Query",
	//	zap.String("device", req.DeviceCode),
	//	zap.Int("type", req.Type))

	if req.Aggregate {
		return s.queryAggregated(req)
	}
	return s.queryList(req)
}

func (s *SQLiteStore) queryList(req QueryRequest) (*QueryResponse, error) {
	start := req.StartTime
	end := req.EndTime
	if end == 0 {
		end = time.Now().UnixMilli()
	}

	months := s.getMonthsInRange(start, end)

	// 1. Calculate Total Count
	var total int64
	for _, month := range months {
		db, err := s.getDB(month)
		if err != nil {
			continue
		}
		var count int64
		tx := db.Model(&Record{}).Where("device_code = ? AND ts BETWEEN ? AND ?", req.DeviceCode, start, end)
		if req.Type > 0 {
			tx = tx.Where("type = ?", req.Type)
		}
		tx.Count(&count)
		total += count
	}

	// 2. Pagination (Newest to Oldest)
	offset := int64((req.Page - 1) * req.PageSize)
	if offset < 0 {
		offset = 0
	}
	limit := int(req.PageSize)
	if limit <= 0 {
		limit = 20
	}

	resultList := make([]interface{}, 0)

	// Iterate backwards
	for i := len(months) - 1; i >= 0; i-- {
		if limit <= 0 {
			break
		}
		month := months[i]
		db, err := s.getDB(month)
		if err != nil {
			continue
		}

		var count int64
		tx := db.Model(&Record{}).Where("device_code = ? AND ts BETWEEN ? AND ?", req.DeviceCode, start, end)
		if req.Type > 0 {
			tx = tx.Where("type = ?", req.Type)
		}
		tx.Count(&count)

		if count == 0 {
			continue
		}

		if offset >= count {
			offset -= count
			continue
		}

		var recs []Record
		tx2 := db.Where("device_code = ? AND ts BETWEEN ? AND ?", req.DeviceCode, start, end)
		if req.Type > 0 {
			tx2 = tx2.Where("type = ?", req.Type)
		}

		if err := tx2.Order("ts desc").Offset(int(offset)).Limit(limit).Find(&recs).Error; err != nil {
			s.logger.Error("Query failed", zap.String("month", month), zap.Error(err))
			continue
		}

		for _, r := range recs {
			var payload map[string]interface{}
			if err := json.Unmarshal(r.Payload, &payload); err != nil {
				payload = map[string]interface{}{"raw": string(r.Payload), "error": "parse_failed"}
			}

			if req.Type == TypeTelemetry && len(req.Keys) > 0 {
				filtered := make(map[string]interface{})
				for _, k := range req.Keys {
					if v, ok := payload[k]; ok {
						filtered[k] = v
					}
				}
				filtered["ts"] = r.Ts
				resultList = append(resultList, filtered)
			} else {
				payload["ts"] = r.Ts
				payload["_type"] = r.Type
				resultList = append(resultList, payload)
			}
		}

		fetched := len(recs)
		limit -= fetched
		offset = 0 // Offset consumed
	}

	return &QueryResponse{
		Total: total,
		List:  resultList,
	}, nil
}

func (s *SQLiteStore) queryAggregated(req QueryRequest) (*QueryResponse, error) {
	start := req.StartTime
	end := req.EndTime
	if end == 0 {
		end = time.Now().UnixMilli()
	}

	months := s.getMonthsInRange(start, end)
	var interval int64
	var resultList []interface{}

	if req.Type == TypeEvent {
		eventCounts := make(map[string]int)
		for _, month := range months {
			db, err := s.getDB(month)
			if err != nil {
				continue
			}
			rows, err := db.Model(&Record{}).
				Where("device_code = ? AND type = ? AND ts >= ? AND ts <= ?", req.DeviceCode, req.Type, start, end).
				Rows()
			if err != nil {
				continue
			}
			defer rows.Close()

			for rows.Next() {
				var r Record
				db.ScanRows(rows, &r)
				var payload map[string]interface{}
				if err := json.Unmarshal(r.Payload, &payload); err == nil {
					if eid, ok := payload["event_id"].(string); ok {
						eventCounts[eid]++
					} else {
						eventCounts["Unknown"]++
					}
				}
			}
		}
		for k, v := range eventCounts {
			resultList = append(resultList, map[string]interface{}{
				"event_id": k,
				"count":    v,
				"ts":       start,
			})
		}
	} else {
		// Telemetry Aggregation
		targetPoints := req.MaxPoints
		if targetPoints <= 0 {
			targetPoints = 2000
		}
		interval = (end - start) / int64(targetPoints)
		if interval < 1000 {
			interval = 1000
		}

		type Bucket struct {
			Sum    map[string]float64
			Count  map[string]int
			Min    map[string]float64
			Max    map[string]float64
			Values map[string][]float64
			Last   map[string]interface{}
		}
		buckets := make(map[int64]*Bucket)

		rawLimit := targetPoints
		rawList := make([]interface{}, 0, rawLimit)
		useRaw := true

		for _, month := range months {
			db, err := s.getDB(month)
			if err != nil {
				continue
			}

			rows, err := db.Model(&Record{}).
				Where("device_code = ? AND type = ? AND ts >= ? AND ts <= ?", req.DeviceCode, req.Type, start, end).
				Rows()
			if err != nil {
				continue
			}
			defer rows.Close()

			for rows.Next() {
				var r Record
				db.ScanRows(rows, &r)
				var payload map[string]interface{}
				if err := json.Unmarshal(r.Payload, &payload); err == nil {
					// Aggregation
					binIdx := (r.Ts - start) / interval
					b, ok := buckets[binIdx]
					if !ok {
						b = &Bucket{
							Sum:    make(map[string]float64),
							Count:  make(map[string]int),
							Min:    make(map[string]float64),
							Max:    make(map[string]float64),
							Values: make(map[string][]float64),
							Last:   make(map[string]interface{}),
						}
						buckets[binIdx] = b
					}

					for k, v := range payload {
						if len(req.Keys) > 0 {
							found := false
							for _, rk := range req.Keys {
								if rk == k {
									found = true
									break
								}
							}
							if !found {
								continue
							}
						}

						if num, ok := v.(float64); ok {
							b.Count[k]++
							switch req.AggMethod {
							case "min":
								if val, exists := b.Min[k]; !exists || num < val {
									b.Min[k] = num
								}
							case "max":
								if val, exists := b.Max[k]; !exists || num > val {
									b.Max[k] = num
								}
							case "median":
								b.Values[k] = append(b.Values[k], num)
							default: // avg
								b.Sum[k] += num
							}
						}
						b.Last[k] = v
					}

					// Raw
					if useRaw {
						item := make(map[string]interface{})
						item["ts"] = r.Ts
						for k, v := range payload {
							if len(req.Keys) > 0 {
								found := false
								for _, rk := range req.Keys {
									if rk == k {
										found = true
										break
									}
								}
								if !found {
									continue
								}
							}
							item[k] = v
						}
						rawList = append(rawList, item)
						if len(rawList) > rawLimit {
							useRaw = false
							rawList = nil
						}
					}
				}
			}
		}

		if useRaw {
			sort.Slice(rawList, func(i, j int) bool {
				t1 := rawList[i].(map[string]interface{})["ts"].(int64)
				t2 := rawList[j].(map[string]interface{})["ts"].(int64)
				return t1 < t2
			})
			return &QueryResponse{
				Total: int64(len(rawList)),
				List:  rawList,
			}, nil
		}

		// Fallback to Aggregated
		var keys []int64
		for k := range buckets {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

		for _, k := range keys {
			b := buckets[k]
			item := make(map[string]interface{})
			item["ts"] = start + k*interval
			for pk, pv := range b.Last {
				item[pk] = pv
			}
			switch req.AggMethod {
			case "min":
				for pk, val := range b.Min {
					item[pk] = val
				}
			case "max":
				for pk, val := range b.Max {
					item[pk] = val
				}
			case "median":
				for pk, vals := range b.Values {
					if len(vals) > 0 {
						sort.Float64s(vals)
						item[pk] = vals[len(vals)/2]
					}
				}
			default:
				for pk, sum := range b.Sum {
					if cnt := b.Count[pk]; cnt > 0 {
						item[pk] = sum / float64(cnt)
					}
				}
			}
			resultList = append(resultList, item)
		}
	}

	return &QueryResponse{
		Total:    int64(len(resultList)),
		List:     resultList,
		Interval: interval,
	}, nil
}

func (s *SQLiteStore) getMonthsInRange(start, end int64) []string {
	startT := time.UnixMilli(start)
	endT := time.UnixMilli(end)
	months := []string{}
	curr := startT
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
