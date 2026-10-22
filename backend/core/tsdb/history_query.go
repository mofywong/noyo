package tsdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const maxHistoryRows = 100000
const maxHistoryBytes = 16 << 20

// Apply the per-response byte cap before materializing a legacy oversized JSON
// value in Go. The original length is checked even when payload is omitted.
const boundedHistoryColumns = "id, ts, device_code, type, CASE WHEN length(CAST(payload AS BLOB)) <= 16777216 THEN payload ELSE NULL END AS payload, length(CAST(payload AS BLOB)) AS payload_bytes"

type boundedHistoryRecord struct {
	Record
	PayloadBytes int64 `gorm:"column:payload_bytes"`
}

// Existing shards are opened read-only and released after each use. Only the
// write path owns cached connections and schema migrations.
func (s *SQLiteStore) readDB(month string) (*gorm.DB, func(), error) {
	path := filepath.Join(s.baseDir, "history_"+month+".db")
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("invalid history shard %s", month)
	}
	db, err := gorm.Open(sqlite.Open("file:"+filepath.ToSlash(path)+"?mode=ro"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, nil, fmt.Errorf("open history %s: %w", month, err)
	}
	conn, err := db.DB()
	if err != nil {
		return nil, nil, err
	}
	conn.SetMaxOpenConns(1)
	return db, func() { _ = conn.Close() }, nil
}

func (s *SQLiteStore) existingMonths(start, end int64) ([]string, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}
	first, last := s.getMonthStr(start), s.getMonthStr(end)
	months := make([]string, 0)
	for _, entry := range entries {
		name := entry.Name()
		if len(name) != 17 || !strings.HasPrefix(name, "history_") || !strings.HasSuffix(name, ".db") {
			continue
		}
		month := name[8:14]
		if _, err := time.Parse("200601", month); err != nil {
			continue
		}
		if month >= first && month <= last {
			months = append(months, month)
		}
	}
	sort.Strings(months)
	if len(months) > 120 {
		return nil, errors.New("History range exceeds 120 stored months; narrow the time range / 历史范围超过120个月，请缩小时间范围")
	}
	return months, nil
}

func (s *SQLiteStore) Query(req QueryRequest) (*QueryResponse, error) {
	if req.EndTime == 0 {
		req.EndTime = time.Now().UnixMilli()
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.MaxPoints <= 0 {
		req.MaxPoints = 2000
	}
	if req.StartTime < 0 || req.EndTime < req.StartTime || req.PageSize > 500 || req.Page > 10000 || int64(req.Page-1)*int64(req.PageSize) > maxHistoryRows || req.MaxPoints > 10000 || len(req.Keys) > 128 || len(req.Owners) > 5000 {
		return nil, errors.New("History query exceeds parameter limits / 历史查询参数超出限制")
	}
	if req.Aggregate && req.EndTime-req.StartTime > 366*86400000 {
		return nil, errors.New("Aggregation is limited to 366 days / 聚合时间范围不能超过366天")
	}
	if req.Aggregate && req.AggMethod != "" && req.AggMethod != "avg" && req.AggMethod != "min" && req.AggMethod != "max" && req.AggMethod != "median" {
		return nil, errors.New("Invalid aggregation method / 无效的聚合方法")
	}
	if req.Context == nil {
		req.Context = context.Background()
	}
	ctx, cancel := context.WithTimeout(req.Context, 10*time.Second)
	defer cancel()
	req.Context = ctx
	if req.Owners != nil && len(req.Owners) == 0 {
		return &QueryResponse{List: []interface{}{}}, nil
	}
	months, err := s.existingMonths(req.StartTime, req.EndTime)
	if err != nil {
		return nil, err
	}
	if req.Aggregate {
		return s.historyAggregated(req, months)
	}
	return s.historyList(req, months)
}

func historyFilter(db *gorm.DB, req QueryRequest) (*gorm.DB, bool, error) {
	tx := db.WithContext(req.Context).Model(&Record{}).Where("ts BETWEEN ? AND ?", req.StartTime, req.EndTime)
	if req.DeviceCode != "" {
		tx = tx.Where("device_code = ?", req.DeviceCode)
	}
	if req.Type > 0 {
		tx = tx.Where("type = ?", req.Type)
	}
	legacy := false
	if req.Owners != nil {
		// Old schemas cannot prove ownership. Keep these rows unavailable until an
		// operator supplies a trusted mapping; never infer ownership from a code.
		var columns []struct{ Name string }
		if err := db.WithContext(req.Context).Raw("PRAGMA table_info(records)").Scan(&columns).Error; err != nil {
			return nil, false, err
		}
		found := map[string]bool{}
		for _, column := range columns {
			found[column.Name] = true
		}
		if len(found) == 0 {
			return nil, false, errors.New("history records table missing")
		}
		if !found["device_id"] || !found["tenant_id"] || !found["project_id"] {
			return tx.Where("1=0"), true, nil
		}
		var unowned int
		if err := db.WithContext(req.Context).Raw("SELECT EXISTS(SELECT 1 FROM records WHERE device_id IS NULL OR device_id = 0 LIMIT 1)").Scan(&unowned).Error; err != nil {
			return nil, false, err
		}
		legacy = unowned != 0
		owners := db.Where("1=0")
		groups := map[[2]uint][]uint{}
		for _, owner := range req.Owners {
			if owner.DeviceID > 0 && owner.TenantID > 0 {
				key := [2]uint{owner.TenantID, owner.ProjectID}
				groups[key] = append(groups[key], owner.DeviceID)
			}
		}
		if len(groups) > 500 {
			return nil, false, errors.New("Narrow history to fewer projects / 请缩小历史查询项目范围")
		}
		for group, ids := range groups {
			owners = owners.Or("(tenant_id = ? AND project_id = ? AND device_id IN ?)", group[0], group[1], ids)
		}
		tx = tx.Where(owners)
	}
	return tx, legacy, nil
}

func (s *SQLiteStore) historyList(req QueryRequest, months []string) (*QueryResponse, error) {
	result := &QueryResponse{List: make([]interface{}, 0, req.PageSize)}
	counts := make(map[string]int64, len(months))
	if !req.SkipTotal {
		for _, month := range months {
			db, closeDB, err := s.readDB(month)
			if err != nil {
				return nil, err
			}
			tx, legacy, err := historyFilter(db, req)
			var count int64
			if err == nil {
				err = tx.Count(&count).Error
			}
			closeDB()
			if err != nil {
				return nil, fmt.Errorf("count history %s: %w", month, err)
			}
			result.LegacyUnscoped = result.LegacyUnscoped || legacy
			counts[month] = count
			result.Total += count
		}
	}
	offset := (req.Page - 1) * req.PageSize
	bytesRead := 0
	for i := len(months) - 1; i >= 0; i-- {
		month := months[i]
		if !req.SkipTotal && int64(offset) >= counts[month] {
			offset -= int(counts[month])
			continue
		}
		db, closeDB, err := s.readDB(month)
		if err != nil {
			return nil, err
		}
		tx, legacy, err := historyFilter(db, req)
		// The recent-events request (page one) performs no COUNT. For later
		// pages, consume the offset before loading any payloads into memory.
		wanted := req.PageSize + 1 - len(result.List)
		if err == nil && req.SkipTotal && offset > 0 {
			var count int64
			err = tx.Count(&count).Error
			if err == nil && int64(offset) >= count {
				offset -= int(count)
				closeDB()
				continue
			}
		}
		if err == nil {
			err = func() error {
				rows, err := tx.Select(boundedHistoryColumns).Order("ts DESC, id DESC").Offset(offset).Limit(wanted).Rows()
				if err != nil {
					return err
				}
				defer rows.Close()
				for rows.Next() {
					if err := req.Context.Err(); err != nil {
						return err
					}
					var record boundedHistoryRecord
					if err := db.ScanRows(rows, &record); err != nil {
						return err
					}
					bytesRead += int(record.PayloadBytes)
					if bytesRead > maxHistoryBytes {
						return errors.New("History response exceeds byte budget / 历史响应超出数据量限制")
					}
					var payload map[string]interface{}
					if err := json.Unmarshal(record.Payload, &payload); err != nil || payload == nil {
						return errors.New("Invalid history payload / 历史记录内容无效")
					}
					if req.Type == TypeTelemetry && len(req.Keys) > 0 {
						payload = filterHistoryKeys(payload, req.Keys)
					}
					payload["ts"] = record.Ts
					payload["device_code"] = record.DeviceCode
					payload["_record_id"] = record.ID
					payload["_type"] = record.Type
					result.List = append(result.List, payload)
				}
				return rows.Err()
			}()
		}
		closeDB()
		if err != nil {
			return nil, fmt.Errorf("read history %s: %w", month, err)
		}
		result.LegacyUnscoped = result.LegacyUnscoped || legacy
		offset = 0
		if len(result.List) > req.PageSize {
			result.HasMore = true
			result.List = result.List[:req.PageSize]
			break
		}
	}
	if !req.SkipTotal {
		result.HasMore = int64((req.Page-1)*req.PageSize+len(result.List)) < result.Total
	}
	return result, nil
}

func filterHistoryKeys(payload map[string]interface{}, keys []string) map[string]interface{} {
	filtered := make(map[string]interface{}, len(keys))
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			filtered[key] = value
		}
	}
	return filtered
}

func (s *SQLiteStore) visitHistory(req QueryRequest, months []string, visit func(Record) error) (bool, error) {
	count, bytesRead := 0, 0
	legacy := false
	for _, month := range months {
		err := func() error {
			db, closeDB, err := s.readDB(month)
			if err != nil {
				return err
			}
			defer closeDB()
			tx, old, err := historyFilter(db, req)
			legacy = legacy || old
			if err != nil {
				return err
			}
			rows, err := tx.Select(boundedHistoryColumns).Order("ts ASC, id ASC").Limit(maxHistoryRows - count + 1).Rows()
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				if err := req.Context.Err(); err != nil {
					return err
				}
				var record boundedHistoryRecord
				if err := db.ScanRows(rows, &record); err != nil {
					return err
				}
				count++
				bytesRead += int(record.PayloadBytes)
				if count > maxHistoryRows || bytesRead > maxHistoryBytes {
					return errors.New("History aggregation exceeds scan budget; narrow the time range / 历史聚合超出扫描限制，请缩小时间范围")
				}
				if err := visit(record.Record); err != nil {
					return err
				}
			}
			return rows.Err()
		}()
		if err != nil {
			return false, fmt.Errorf("aggregate history %s: %w", month, err)
		}
	}
	return legacy, nil
}

type historyBucket struct {
	last   map[string]interface{}
	sum    map[string]float64
	count  map[string]int
	values map[string][]float64
}

func (s *SQLiteStore) historyAggregated(req QueryRequest, months []string) (*QueryResponse, error) {
	result := &QueryResponse{List: []interface{}{}}
	interval := (req.EndTime-req.StartTime)/int64(req.MaxPoints) + 1
	if interval < 1000 {
		interval = 1000
	}
	buckets := map[int64]*historyBucket{}
	events := map[string]int{}
	cells := 0
	raw := make([]interface{}, 0, req.MaxPoints)
	useRaw := true
	legacy, err := s.visitHistory(req, months, func(record Record) error {
		var payload map[string]interface{}
		if err := json.Unmarshal(record.Payload, &payload); err != nil || payload == nil {
			return errors.New("Invalid history payload / 历史记录内容无效")
		}
		if req.Type == TypeEvent {
			id, ok := payload["event_id"].(string)
			if !ok {
				id = "Unknown"
			}
			events[id]++
			if len(events) > req.MaxPoints {
				return errors.New("History event groups exceed output budget / 历史事件分组超出输出限制")
			}
			return nil
		}
		if len(req.Keys) > 0 {
			payload = filterHistoryKeys(payload, req.Keys)
		}
		if len(payload) > 128 {
			return errors.New("History payload exceeds field budget / 历史记录字段超出限制")
		}
		idx := (record.Ts - req.StartTime) / interval
		bucket := buckets[idx]
		if bucket == nil {
			bucket = &historyBucket{last: map[string]interface{}{}, sum: map[string]float64{}, count: map[string]int{}, values: map[string][]float64{}}
			buckets[idx] = bucket
		}
		for key, value := range payload {
			if _, exists := bucket.last[key]; !exists {
				cells++
				if cells > 100000 {
					return errors.New("History aggregation exceeds cell budget / 历史聚合超出字段单元限制")
				}
			}
			if number, ok := value.(float64); ok {
				bucket.count[key]++
				switch req.AggMethod {
				case "min":
					if previous, found := bucket.sum[key]; !found || number < previous {
						bucket.sum[key] = number
					}
				case "max":
					if previous, found := bucket.sum[key]; !found || number > previous {
						bucket.sum[key] = number
					}
				case "median":
					bucket.values[key] = append(bucket.values[key], number)
				default:
					bucket.sum[key] += number
				}
			}
			bucket.last[key] = value
		}
		if useRaw {
			if len(raw) == req.MaxPoints {
				useRaw = false
				raw = nil
			} else {
				item := map[string]interface{}{}
				for k, v := range payload {
					item[k] = v
				}
				item["ts"] = record.Ts
				raw = append(raw, item)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result.LegacyUnscoped = legacy
	if req.Type == TypeEvent {
		names := make([]string, 0, len(events))
		for id := range events {
			names = append(names, id)
		}
		sort.Strings(names)
		for _, id := range names {
			result.List = append(result.List, map[string]interface{}{"event_id": id, "count": events[id], "ts": req.StartTime})
		}
	} else if useRaw {
		result.List = raw
	} else {
		indexes := make([]int64, 0, len(buckets))
		for idx := range buckets {
			indexes = append(indexes, idx)
		}
		sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
		for _, idx := range indexes {
			bucket := buckets[idx]
			item := bucket.last
			for key, count := range bucket.count {
				switch req.AggMethod {
				case "median":
					values := bucket.values[key]
					sort.Float64s(values)
					item[key] = values[len(values)/2]
				case "min", "max":
					item[key] = bucket.sum[key]
				default:
					item[key] = bucket.sum[key] / float64(count)
				}
			}
			item["ts"] = req.StartTime + idx*interval
			result.List = append(result.List, item)
		}
		result.Interval = interval
	}
	result.Total = int64(len(result.List))
	return result, nil
}
