package store

import (
	"encoding/json"
	"errors"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RuntimeApply records whether the gateway execution process has loaded a
// durably synchronized configuration. It is distinct from the replication ACK.
// One row per resource retains only the latest desired revision for retries.
type RuntimeApply struct {
	EntityType             string `gorm:"primaryKey;size:32" json:"entity_type"`
	EntityCode             string `gorm:"primaryKey;size:128" json:"entity_code"`
	ChangeID               string `gorm:"size:160;not null" json:"change_id"`
	Revision               int64  `gorm:"not null" json:"revision"`
	Payload                string `gorm:"type:text" json:"-"`
	Operation              string `gorm:"size:16" json:"-"`
	ActiveRevision         int64  `gorm:"not null;default:0" json:"active_revision"`
	ActiveLifecycleVersion int64  `gorm:"not null;default:0" json:"active_lifecycle_version"`
	ActivePayload          string `gorm:"type:text" json:"-"`
	CompensationStatus     string `gorm:"size:16" json:"compensation_status,omitempty"`
	CompensationError      string `gorm:"size:1024" json:"compensation_error,omitempty"`
	LifecycleVersion       int64  `gorm:"not null;default:1" json:"lifecycle_version"`
	Status                 string `gorm:"size:16;not null" json:"status"`
	LastError              string `gorm:"size:1024" json:"last_error,omitempty"`
	Attempts               int    `gorm:"not null;default:0" json:"attempts"`
	ReportedAttempts       int    `gorm:"not null;default:0" json:"reported_attempts"`
	UpdatedAt              int64  `gorm:"not null" json:"updated_at"`
}

// GatewayRuntimeState is the platform's last verified runtime result for a
// resource on one gateway. The desired configuration remains separate.
type GatewayRuntimeState struct {
	GatewayCode            string `gorm:"primaryKey;size:128" json:"gateway_code"`
	EntityType             string `gorm:"primaryKey;size:32" json:"entity_type"`
	EntityCode             string `gorm:"primaryKey;size:128" json:"entity_code"`
	ChangeID               string `gorm:"size:160;not null" json:"change_id"`
	ChangeSeq              int64  `gorm:"not null" json:"change_seq"`
	Revision               int64  `gorm:"not null" json:"revision"`
	ActiveRevision         int64  `gorm:"not null;default:0" json:"active_revision"`
	ActiveLifecycleVersion int64  `gorm:"not null;default:0" json:"active_lifecycle_version"`
	CompensationStatus     string `gorm:"size:16" json:"compensation_status,omitempty"`
	CompensationError      string `gorm:"size:1024" json:"compensation_error,omitempty"`
	LifecycleVersion       int64  `gorm:"not null;default:1" json:"lifecycle_version"`
	Status                 string `gorm:"size:16;not null" json:"status"`
	LastError              string `gorm:"size:1024" json:"last_error,omitempty"`
	Attempts               int    `gorm:"not null" json:"attempts"`
	UpdatedAt              int64  `gorm:"not null" json:"updated_at"`
}

const (
	RuntimeApplyPending       = "pending"
	RuntimeApplyFailed        = "failed"
	RuntimeApplyActive        = "active"
	RuntimeCompensated        = "restored"
	RuntimeCompensationFailed = "failed"
)

func QueueRuntimeApplyTx(tx *gorm.DB, change SyncChange) error {
	var existing RuntimeApply
	err := tx.Session(&gorm.Session{NewDB: true}).Where("entity_type = ? AND entity_code = ?", change.EntityType, change.EntityCode).Take(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil && existing.Status == RuntimeApplyActive && existing.Revision == change.NewRev && existing.LifecycleVersion == change.LifecycleVersion && existing.Operation == change.Op && runtimePayloadEqual(existing.Payload, change.Payload) {
		// A satisfied intent still receives a canonical ACK, without restarting
		// an execution process which already has this exact configuration.
		return nil
	}
	entry := RuntimeApply{
		EntityType: change.EntityType, EntityCode: change.EntityCode,
		ChangeID: change.ChangeID(), Revision: change.NewRev, LifecycleVersion: change.LifecycleVersion,
		Payload: change.Payload, Operation: change.Op, ActivePayload: change.BasePayload, ActiveRevision: change.BaseRev,
		Status: RuntimeApplyPending, UpdatedAt: time.Now().UnixMilli(),
	}
	if change.BasePayload != "" || change.BaseRev > 0 {
		entry.ActiveLifecycleVersion = change.LifecycleVersion
	}
	return tx.Session(&gorm.Session{NewDB: true}).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "entity_type"}, {Name: "entity_code"}},
		DoUpdates: clause.AssignmentColumns([]string{"change_id", "revision", "payload", "operation", "lifecycle_version", "status", "last_error", "compensation_status", "compensation_error", "attempts", "reported_attempts", "updated_at"}),
	}).Create(&entry).Error
}

func runtimePayloadEqual(left, right string) bool {
	if left == right {
		return true
	}
	var a, b interface{}
	return json.Unmarshal([]byte(left), &a) == nil && json.Unmarshal([]byte(right), &b) == nil && reflect.DeepEqual(a, b)
}

// CompleteRuntimeApply only updates the revision that was actually attempted.
// A newer config may already have replaced it while a slow driver was loading.
func CompleteRuntimeApply(changeID string, err error) error {
	return CompleteRuntimeApplyWithCompensation(changeID, err, "", nil)
}

func CompleteRuntimeApplyWithCompensation(changeID string, err error, compensationStatus string, compensationErr error) error {
	status, detail := RuntimeApplyActive, ""
	if err != nil {
		status, detail = RuntimeApplyFailed, err.Error()
	}
	updates := map[string]interface{}{
		"status": status, "last_error": detail,
		"compensation_status": compensationStatus, "compensation_error": "",
		"attempts": gorm.Expr("attempts + 1"), "updated_at": time.Now().UnixMilli(),
	}
	if compensationErr != nil {
		updates["compensation_error"] = compensationErr.Error()
	}
	if err == nil {
		updates["active_revision"] = gorm.Expr("revision")
		updates["active_lifecycle_version"] = gorm.Expr("lifecycle_version")
		updates["active_payload"] = gorm.Expr("payload")
	}
	return DB.Model(&RuntimeApply{}).Where("change_id = ?", changeID).Updates(updates).Error
}

// CompleteRuntimeApplyAttempt also retains a successfully loaded snapshot when
// a newer desired change arrived during activation. Its pending/failed status
// and revision remain untouched, so the newer configuration is still retried.
func CompleteRuntimeApplyAttempt(attempt RuntimeApply, applyErr error, compensationStatus string, compensationErr error) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status": RuntimeApplyActive, "last_error": "", "compensation_status": compensationStatus, "compensation_error": "",
			"attempts": gorm.Expr("attempts + 1"), "updated_at": time.Now().UnixMilli(),
		}
		if applyErr != nil {
			updates["status"], updates["last_error"] = RuntimeApplyFailed, applyErr.Error()
		}
		if compensationErr != nil {
			updates["compensation_error"] = compensationErr.Error()
		}
		if applyErr == nil {
			updates["active_revision"], updates["active_lifecycle_version"], updates["active_payload"] = attempt.Revision, attempt.LifecycleVersion, attempt.Payload
		}
		result := tx.Model(&RuntimeApply{}).Where("change_id = ?", attempt.ChangeID).Updates(updates)
		if result.Error != nil || result.RowsAffected > 0 || applyErr != nil {
			return result.Error
		}
		return tx.Model(&RuntimeApply{}).Where("entity_type = ? AND entity_code = ? AND lifecycle_version = ? AND active_revision <= ?", attempt.EntityType, attempt.EntityCode, attempt.LifecycleVersion, attempt.Revision).
			Updates(map[string]interface{}{"active_revision": attempt.Revision, "active_lifecycle_version": attempt.LifecycleVersion, "active_payload": attempt.Payload}).Error
	})
}

// LastActiveRuntimeConfigs supplies execution snapshots without reverting the
// newer, durably committed desired configuration.
func LastActiveRuntimeConfigs() ([]RuntimeApply, error) {
	if !DB.Migrator().HasTable(&RuntimeApply{}) {
		return nil, nil
	}
	var rows []RuntimeApply
	err := DB.Where("status IN ?", []string{RuntimeApplyPending, RuntimeApplyFailed}).Find(&rows).Error
	return rows, err
}

func GetRuntimeApply(entityType, entityCode string) (*RuntimeApply, error) {
	var row RuntimeApply
	err := DB.Where("entity_type = ? AND entity_code = ?", entityType, entityCode).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func ListUnappliedRuntime(limit int) ([]RuntimeApply, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []RuntimeApply
	err := DB.Where("status IN ?", []string{RuntimeApplyPending, RuntimeApplyFailed}).Order("updated_at asc").Limit(limit).Find(&rows).Error
	return rows, err
}

// ListPendingRuntime excludes terminal failures. Maintenance drains accepted
// work once; a previous failed activation must not prevent handing over mode.
func ListPendingRuntime(limit int) ([]RuntimeApply, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []RuntimeApply
	err := DB.Where("status = ?", RuntimeApplyPending).Order("updated_at asc").Limit(limit).Find(&rows).Error
	return rows, err
}

func ListUnreportedRuntime(limit int) ([]RuntimeApply, error) {
	if limit <= 0 {
		limit = 100
	}
	var rows []RuntimeApply
	err := DB.Where("status IN ? AND attempts > reported_attempts", []string{RuntimeApplyActive, RuntimeApplyFailed}).Order("updated_at asc").Limit(limit).Find(&rows).Error
	return rows, err
}

func AckRuntimeApply(changeID string, attempts int) error {
	return DB.Model(&RuntimeApply{}).Where("change_id = ? AND attempts = ?", changeID, attempts).
		Update("reported_attempts", attempts).Error
}

// RecordGatewayRuntimeState ignores stale retries from an older change or an
// earlier attempt of the same change. The caller authenticates the change and
// its gateway routing before recording it.
func RecordGatewayRuntimeState(state GatewayRuntimeState) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var previous GatewayRuntimeState
		err := tx.Where("gateway_code = ? AND entity_type = ? AND entity_code = ?", state.GatewayCode, state.EntityType, state.EntityCode).Take(&previous).Error
		if err == nil {
			if previous.LifecycleVersion > state.LifecycleVersion ||
				(previous.LifecycleVersion == state.LifecycleVersion && previous.Revision > state.Revision) ||
				(previous.LifecycleVersion == state.LifecycleVersion && previous.Revision == state.Revision &&
					(previous.ChangeID != state.ChangeID || previous.Attempts >= state.Attempts)) {
				return nil
			}
		} else if err != gorm.ErrRecordNotFound {
			return err
		}
		return tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "gateway_code"}, {Name: "entity_type"}, {Name: "entity_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"change_id", "change_seq", "revision", "active_revision", "active_lifecycle_version", "compensation_status", "compensation_error", "lifecycle_version", "status", "last_error", "attempts", "updated_at"}),
		}).Create(&state).Error
	})
}
