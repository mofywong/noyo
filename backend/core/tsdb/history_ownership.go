package tsdb

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type ownedRecordStore interface {
	GetRecord(context.Context, int64, int64) (*Record, error)
	DeleteOwnedRecord(context.Context, int64, int64, RecordOwner) error
}

func (m *TSDBManager) GetRecord(ctx context.Context, id, ts int64) (*Record, error) {
	store, ok := m.store.(ownedRecordStore)
	if !m.cfg.Enabled || !ok {
		return nil, errors.New("History is unavailable / 历史存储不可用")
	}
	return store.GetRecord(ctx, id, ts)
}

func (m *TSDBManager) DeleteOwnedRecord(ctx context.Context, id, ts int64, owner RecordOwner) error {
	store, ok := m.store.(ownedRecordStore)
	if !m.cfg.Enabled || !ok {
		return errors.New("History is unavailable / 历史存储不可用")
	}
	return store.DeleteOwnedRecord(ctx, id, ts, owner)
}

func (s *SQLiteStore) GetRecord(ctx context.Context, id, ts int64) (*Record, error) {
	db, closeDB, err := s.readDB(s.getMonthStr(ts))
	if err != nil {
		return nil, err
	}
	defer closeDB()
	var record Record
	if err := db.WithContext(ctx).Omit("Payload").Where("id = ? AND ts = ?", id, ts).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *SQLiteStore) DeleteOwnedRecord(ctx context.Context, id, ts int64, owner RecordOwner) error {
	if owner.DeviceID == 0 || owner.TenantID == 0 {
		return errors.New("History ownership is unavailable / 历史记录归属不明")
	}
	// Verify that the shard exists first: a delete request must never create it.
	if _, closeDB, err := s.readDB(s.getMonthStr(ts)); err != nil {
		return err
	} else {
		closeDB()
	}
	db, err := s.getDB(s.getMonthStr(ts))
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Where("id = ? AND ts = ? AND device_id = ? AND tenant_id = ? AND project_id = ?", id, ts, owner.DeviceID, owner.TenantID, owner.ProjectID).Delete(&Record{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// LegacyOwnershipMapping must come from a trusted historical inventory, not a
// request or the current device with the same code. The time bounds are inclusive.
type LegacyOwnershipMapping struct {
	DeviceCode string      `json:"device_code"`
	Owner      RecordOwner `json:"owner"`
	From       int64       `json:"from"`
	To         int64       `json:"to"`
}

func validateLegacyMapping(mapping LegacyOwnershipMapping) error {
	if mapping.DeviceCode == "" || mapping.Owner.DeviceID == 0 || mapping.Owner.TenantID == 0 || mapping.From < 0 || mapping.To < mapping.From {
		return errors.New("invalid legacy ownership mapping")
	}
	return nil
}

// PreviewLegacyOwnership counts candidates through read-only connections.
func (s *SQLiteStore) PreviewLegacyOwnership(ctx context.Context, mapping LegacyOwnershipMapping) (int64, error) {
	if err := validateLegacyMapping(mapping); err != nil {
		return 0, err
	}
	months, err := s.existingMonths(mapping.From, mapping.To)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, month := range months {
		db, closeDB, err := s.readDB(month)
		if err != nil {
			return 0, err
		}
		var columns []struct{ Name string }
		err = db.WithContext(ctx).Raw("PRAGMA table_info(records)").Scan(&columns).Error
		owned := false
		for _, column := range columns {
			if column.Name == "device_id" {
				owned = true
			}
		}
		tx := db.WithContext(ctx).Model(&Record{}).Where("device_code = ? AND ts BETWEEN ? AND ?", mapping.DeviceCode, mapping.From, mapping.To)
		if owned {
			tx = tx.Where("device_id IS NULL OR device_id = 0")
		}
		var count int64
		if err == nil {
			err = tx.Count(&count).Error
		}
		closeDB()
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// BackfillLegacyOwnership is an explicit maintenance operation. It never changes
// an already-owned row. Operators must validate and back up their mapping first.
func (s *SQLiteStore) BackfillLegacyOwnership(ctx context.Context, mapping LegacyOwnershipMapping) (int64, error) {
	if err := validateLegacyMapping(mapping); err != nil {
		return 0, err
	}
	months, err := s.existingMonths(mapping.From, mapping.To)
	if err != nil {
		return 0, err
	}
	var changed int64
	for _, month := range months {
		if err := ctx.Err(); err != nil {
			return changed, err
		}
		db, err := s.getDB(month)
		if err != nil {
			return changed, err
		}
		result := db.WithContext(ctx).Model(&Record{}).Where("device_code = ? AND ts BETWEEN ? AND ? AND (device_id = 0 OR device_id IS NULL)", mapping.DeviceCode, mapping.From, mapping.To).Updates(map[string]interface{}{"device_id": mapping.Owner.DeviceID, "tenant_id": mapping.Owner.TenantID, "project_id": mapping.Owner.ProjectID})
		if result.Error != nil {
			return changed, fmt.Errorf("backfill %s: %w", month, result.Error)
		}
		changed += result.RowsAffected
	}
	return changed, nil
}
