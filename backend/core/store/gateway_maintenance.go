package store

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	GatewayModeNormal      = "normal"
	GatewayModeMaintenance = "maintenance"
	GatewayModeReconciling = "reconciling"
)

var ErrGatewayModeChanged = errors.New("gateway maintenance state changed")
var ErrRuntimeApplyPending = errors.New("accepted runtime configuration is still pending")

// GatewayMaintenance is addressed by the existing gateway code. Epoch fences
// messages from an earlier maintenance session; StateVersion fences reordering
// within the same session. Neither value is a resource identifier.
type GatewayMaintenance struct {
	GatewayCode  string `gorm:"primaryKey;size:128" json:"gateway_code"`
	Mode         string `gorm:"size:32;not null" json:"mode"`
	Epoch        int64  `gorm:"not null" json:"epoch"`
	StateVersion int64  `gorm:"not null" json:"state_version"`
	EndSeq       int64  `gorm:"not null" json:"end_seq"`
	UpdatedAt    int64  `gorm:"not null" json:"updated_at"`
}

func GetGatewayMaintenance(code string) (*GatewayMaintenance, error) {
	return getGatewayMaintenance(DB, code)
}

// GetGatewayMaintenanceTx reads the mode in the caller's write transaction so
// replay cannot commit across a concurrent maintenance transition.
func GetGatewayMaintenanceTx(tx *gorm.DB, code string) (*GatewayMaintenance, error) {
	return getGatewayMaintenance(tx, code)
}

func getGatewayMaintenance(db *gorm.DB, code string) (*GatewayMaintenance, error) {
	if code == "" {
		return nil, fmt.Errorf("gateway code is required")
	}
	var state GatewayMaintenance
	// Writes read this row inside their business transaction. Locking the same
	// row as the mode transition prevents a local commit from crossing its gate.
	err := db.Session(&gorm.Session{NewDB: true}).Clauses(clause.Locking{Strength: "UPDATE"}).Where("gateway_code = ?", code).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &GatewayMaintenance{GatewayCode: code, Mode: GatewayModeNormal}, nil
	}
	return &state, err
}

// TransitionGatewayMaintenance is used only by the gateway coordinator. The
// transition and its sequence barrier are durable before a status is sent.
func TransitionGatewayMaintenance(code, from, to string) (*GatewayMaintenance, error) {
	return transitionGatewayMaintenance(code, from, to, false)
}

// TransitionGatewayMaintenanceAfterRuntime closes the write gate only when all
// admitted tasks are terminal in the same transaction as the mode transition.
func TransitionGatewayMaintenanceAfterRuntime(code, from, to string) (*GatewayMaintenance, error) {
	return transitionGatewayMaintenance(code, from, to, true)
}

func transitionGatewayMaintenance(code, from, to string, requireSettled bool) (*GatewayMaintenance, error) {
	if code == "" {
		return nil, fmt.Errorf("gateway code is required")
	}
	if !validMaintenanceTransition(from, to) {
		return nil, fmt.Errorf("invalid gateway mode transition %s -> %s", from, to)
	}
	var result GatewayMaintenance
	err := DB.Transaction(func(tx *gorm.DB) error {
		state := GatewayMaintenance{}
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("gateway_code = ?", code).First(&state).Error
		exists := err == nil
		if errors.Is(err, gorm.ErrRecordNotFound) {
			state = GatewayMaintenance{GatewayCode: code, Mode: GatewayModeNormal}
		} else if err != nil {
			return err
		}
		if state.Mode != from {
			return ErrGatewayModeChanged
		}
		if requireSettled {
			var pending int64
			if err := tx.Model(&RuntimeApply{}).Where("status = ?", RuntimeApplyPending).Count(&pending).Error; err != nil {
				return err
			}
			if pending > 0 {
				return ErrRuntimeApplyPending
			}
		}
		previousVersion := state.StateVersion
		state.Mode = to
		state.StateVersion++
		state.UpdatedAt = time.Now().UnixMilli()
		if to == GatewayModeMaintenance {
			state.Epoch++
			state.EndSeq = 0
		}
		if to == GatewayModeReconciling {
			// Capture the handover barrier in the transition transaction. A write
			// cannot slip between a separate MAX(seq) read and the mode change.
			if err := tx.Model(&SyncChange{}).Select("COALESCE(MAX(seq), 0)").Row().Scan(&state.EndSeq); err != nil {
				return err
			}
		}
		if !exists {
			if err := tx.Create(&state).Error; err != nil {
				return err
			}
		} else {
			updated := tx.Model(&GatewayMaintenance{}).
				Where("gateway_code = ? AND state_version = ?", code, previousVersion).
				Updates(map[string]interface{}{
					"mode": state.Mode, "epoch": state.Epoch,
					"state_version": state.StateVersion, "end_seq": state.EndSeq,
					"updated_at": state.UpdatedAt,
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return ErrGatewayModeChanged
			}
		}
		result = state
		return nil
	})
	return &result, err
}

func validMaintenanceTransition(from, to string) bool {
	return from == GatewayModeNormal && to == GatewayModeMaintenance ||
		from == GatewayModeReconciling && to == GatewayModeMaintenance ||
		from == GatewayModeMaintenance && to == GatewayModeReconciling ||
		from == GatewayModeReconciling && to == GatewayModeNormal
}

// ObserveGatewayMaintenance persists a gateway-authored state at the platform.
// Old MQTT deliveries must never reopen a gateway after a later maintenance
// session has already been observed.
func ObserveGatewayMaintenance(in GatewayMaintenance) (bool, error) {
	if in.GatewayCode == "" || in.StateVersion <= 0 || in.Epoch <= 0 {
		return false, fmt.Errorf("invalid gateway maintenance state")
	}
	if in.Mode != GatewayModeNormal && in.Mode != GatewayModeMaintenance && in.Mode != GatewayModeReconciling {
		return false, fmt.Errorf("invalid gateway mode %q", in.Mode)
	}
	changed := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current GatewayMaintenance
		err := tx.Where("gateway_code = ?", in.GatewayCode).First(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			changed = true
			return tx.Create(&in).Error
		}
		if err != nil {
			return err
		}
		if in.Epoch < current.Epoch || in.Epoch == current.Epoch && in.StateVersion <= current.StateVersion {
			return nil
		}
		updated := tx.Model(&GatewayMaintenance{}).
			Where("gateway_code = ? AND state_version = ?", in.GatewayCode, current.StateVersion).
			Updates(map[string]interface{}{
				"mode": in.Mode, "epoch": in.Epoch, "state_version": in.StateVersion,
				"end_seq": in.EndSeq, "updated_at": in.UpdatedAt,
			})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrGatewayModeChanged
		}
		changed = true
		return nil
	})
	return changed, err
}
