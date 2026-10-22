package tsdb

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"
)

// ScanRequest reads exact raw samples, never chart-downsampled values. Codes
// must already be authorized. Empty codes are deliberately not a wildcard.
type ScanRequest struct {
	Owners             []RecordOwner
	DeviceCodes        []string
	Keys               []string
	StartTime, EndTime int64
	Limit              int
}
type Scanner interface {
	Scan(context.Context, ScanRequest, func(Record) error) error
}

var ErrScanLimit = errors.New("history scan budget exceeded")

func (m *TSDBManager) Scan(ctx context.Context, req ScanRequest, visit func(Record) error) error {
	if !m.cfg.Enabled {
		return errors.New("History storage is disabled / 历史存储未启用")
	}
	scanner, ok := m.store.(Scanner)
	if !ok {
		return errors.New("Storage does not support exact batch queries / 存储不支持精确批量查询")
	}
	return scanner.Scan(ctx, req, visit)
}
func (s *SQLiteStore) Scan(ctx context.Context, req ScanRequest, visit func(Record) error) error {
	if len(req.DeviceCodes) == 0 || len(req.DeviceCodes) > 200 || req.StartTime < 0 || req.EndTime <= req.StartTime || req.EndTime-req.StartTime > 366*86400000 || req.Limit < 1 || req.Limit > 100000 {
		return errors.New("invalid history scan")
	}
	count, bytesRead := 0, 0
	// getDB uses the same local calendar for shard selection as writes do.
	start := time.UnixMilli(req.StartTime)
	firstOfMonth := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location()).UnixMilli()
	for _, month := range s.getMonthsInRange(firstOfMonth, req.EndTime-1) {
		if err := ctx.Err(); err != nil {
			return err
		}
		db, closeDB, err := s.readDB(month)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		tx, _, err := historyFilter(db, QueryRequest{Context: ctx, Owners: req.Owners, StartTime: req.StartTime, EndTime: req.EndTime})
		if err != nil {
			closeDB()
			return err
		}
		rows, err := tx.Select("id,ts,device_code,type,payload").Where("device_code IN ? AND type = ? AND ts >= ? AND ts < ?", req.DeviceCodes, TypeTelemetry, req.StartTime, req.EndTime).Limit(req.Limit - count + 1).Rows()
		if err != nil {
			closeDB()
			return err
		}
		err = func() error {
			defer rows.Close()
			for rows.Next() {
				if err := ctx.Err(); err != nil {
					return err
				}
				var record Record
				if err := db.ScanRows(rows, &record); err != nil {
					return err
				}
				count++
				bytesRead += len(record.Payload)
				if count > req.Limit || bytesRead > 16<<20 {
					return ErrScanLimit
				}
				if !json.Valid(record.Payload) {
					return errors.New("corrupt telemetry payload")
				}
				if err := visit(record); err != nil {
					return err
				}
			}
			return rows.Err()
		}()
		closeDB()
		if err != nil {
			return err
		}
	}
	return nil
}
