package store

import (
	"errors"
	"sync/atomic"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ConfigProjection is a gateway-confirmed configuration, never a platform edit.
// Pending creates and deletes therefore cannot enter the executable registry.
type ConfigProjection struct {
	GatewayCode      string `gorm:"primaryKey;size:128" json:"gateway_code"`
	EntityType       string `gorm:"primaryKey;size:32" json:"entity_type"`
	EntityCode       string `gorm:"primaryKey;size:128" json:"entity_code"`
	TenantID         uint   `gorm:"index" json:"tenant_id"`
	ProjectID        uint   `gorm:"index" json:"project_id"`
	Payload          string `gorm:"type:text" json:"payload"`
	Revision         int64  `json:"revision"`
	LifecycleVersion int64  `json:"lifecycle_version"`
	Deleted          bool   `json:"deleted"`
	ConfirmedAt      int64  `json:"confirmed_at"`
}

type ConfigIntent struct {
	OperationCode   string `gorm:"primaryKey;size:160" json:"operation_code"`
	PredecessorCode string `gorm:"size:160" json:"predecessor_code,omitempty"`
	Sequence        int64  `gorm:"index" json:"-"`
	EntityType      string `gorm:"index;size:32" json:"entity_type"`
	EntityCode      string `gorm:"index;size:128" json:"entity_code"`
	GatewayCode     string `gorm:"index;size:128" json:"gateway_code"`
	TargetNode      string `gorm:"type:text" json:"target_node"`
	TenantID        uint   `gorm:"index" json:"tenant_id"`
	ProjectID       uint   `gorm:"index" json:"project_id"`
	Kind            string `json:"kind"`
	Payload         string `gorm:"type:text" json:"payload"`
	BasePayload     string `gorm:"type:text" json:"base_payload"`
	Status          string `gorm:"index;size:32" json:"status"`
	LastError       string `json:"last_error,omitempty"`
	CreatedAt       int64  `gorm:"index" json:"created_at"`
}

var platformConfigProjectionEnabled atomic.Bool

func EnablePlatformConfigProjection(enabled bool) { platformConfigProjectionEnabled.Store(enabled) }
func IsPlatformConfigProjectionEnabled() bool     { return platformConfigProjectionEnabled.Load() }

func SaveConfigIntentTx(tx *gorm.DB, ch SyncChange) error {
	query := tx.Session(&gorm.Session{NewDB: true})
	var previous ConfigIntent
	err := query.Where("entity_type = ? AND entity_code = ? AND target_node = ? AND tenant_id = ? AND project_id = ? AND operation_code <> ?", ch.EntityType, ch.EntityCode, ch.TargetNode, ch.TenantID, ch.ProjectID, ch.ChangeID()).Order("sequence desc").Order("created_at desc").Take(&previous).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil && (previous.Status == "pending" || previous.Status == "committed") {
		var prior SyncChange
		if err = query.Where("seq = ?", previous.Sequence).Take(&prior).Error; err != nil {
			return err
		}
		// A reused code starts a new intent chain. Results from a deleted
		// incarnation cannot be used as the editing baseline of its replacement.
		if prior.OriginInstance == ch.OriginInstance && prior.LifecycleVersion == ch.LifecycleVersion && prior.Seq < ch.Seq {
			ch.PredecessorCode = previous.OperationCode
			if err = query.Model(&SyncChange{}).Where("seq = ?", ch.Seq).Update("predecessor_code", ch.PredecessorCode).Error; err != nil {
				return err
			}
		}
	}
	row := ConfigIntent{OperationCode: ch.ChangeID(), EntityType: ch.EntityType, EntityCode: ch.EntityCode,
		PredecessorCode: ch.PredecessorCode, Sequence: ch.Seq,
		TargetNode: ch.TargetNode, TenantID: ch.TenantID, ProjectID: ch.ProjectID, Kind: ch.Op,
		Payload: ch.Payload, BasePayload: ch.BasePayload, Status: "pending", CreatedAt: time.Now().UnixMilli()}
	if targets := ch.TargetNodes(); len(targets) == 1 {
		row.GatewayCode = targets[0]
	}
	return tx.Session(&gorm.Session{NewDB: true}).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
}

func SaveConfigProjectionTx(tx *gorm.DB, gatewayCode string, ch SyncChange) error {
	row := ConfigProjection{GatewayCode: gatewayCode, EntityType: ch.EntityType, EntityCode: ch.EntityCode,
		TenantID: ch.TenantID, ProjectID: ch.ProjectID, Payload: ch.Payload, Revision: ch.NewRev,
		LifecycleVersion: ch.LifecycleVersion, Deleted: ch.Op == SyncOpDelete, ConfirmedAt: time.Now().UnixMilli()}
	return tx.Session(&gorm.Session{NewDB: true}).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "gateway_code"}, {Name: "entity_type"}, {Name: "entity_code"}}, UpdateAll: true,
	}).Create(&row).Error
}

// DeferredConfigDelete is checked inside the original deletion transaction.
func DeferredConfigDelete(tx *gorm.DB, ch *SyncChange) (bool, error) {
	if ch == nil || !IsPlatformConfigProjectionEnabled() {
		return false, nil
	}
	var count int64
	err := tx.Session(&gorm.Session{NewDB: true}).Model(&ConfigIntent{}).Where("operation_code = ?", ch.ChangeID()).Count(&count).Error
	return count != 0, err
}

// Transport confirmation settles an intent, but never changes the confirmed
// projection. Only the gateway's canonical journal or snapshot can do that.
func UpdateConfigIntentAckTx(tx *gorm.DB, ch SyncChange, peer, outcome string, failures ...SyncFailure) error {
	if !tx.Migrator().HasTable(&ConfigIntent{}) {
		return nil
	}
	status := "pending"
	switch outcome {
	case "failed":
		status = "failed"
	case "conflict":
		status = "conflict"
	case "skipped":
		status = "rejected"
	case "applied":
		targets := ch.TargetNodes()
		if ch.AckTargets != "" {
			targets = (&SyncChange{TargetNode: ch.AckTargets}).TargetNodes()
		}
		if len(targets) == 0 {
			return nil
		}
		var settled int64
		if err := tx.Model(&SyncDeliveryResult{}).Where("change_id = ? AND peer_code IN ? AND status = ?", ch.ChangeID(), targets, "applied").Count(&settled).Error; err != nil {
			return err
		}
		if settled == int64(len(targets)) {
			status = "committed"
		}
	}
	if status == "pending" {
		return nil
	}
	updates := map[string]interface{}{"status": status}
	if status == "failed" && len(failures) != 0 {
		updates["last_error"] = failures[0].LastError
	}
	return tx.Model(&ConfigIntent{}).Where("operation_code = ? AND status = ?", ch.ChangeID(), "pending").Updates(updates).Error
}
