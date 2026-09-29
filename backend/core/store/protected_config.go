package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"noyo/core/config"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ProtectedConfig stores node and plugin intent separately from running state.
// The key uses the existing gateway code and plugin name; no new resource ID is
// introduced. SystemConfig already participates in database backups/snapshots.
type ProtectedConfig struct {
	Code       string                 `json:"code"`
	GatewaySN  string                 `json:"gateway_sn"`
	PluginName string                 `json:"plugin_name,omitempty"`
	Kind       string                 `json:"kind"`
	TenantID   uint                   `json:"tenant_id"`
	ProjectID  uint                   `json:"project_id"`
	Revision   int64                  `json:"revision"`
	Config     map[string]interface{} `json:"config"`
	SyncState  string                 `json:"sync_state"`
}

const ProtectedConfigPrefix = "cascade:protected_config:"

func ProtectedConfigCode(gateway, kind, plugin string) string {
	if kind == "plugin" {
		return gateway + "/" + plugin
	}
	return gateway + "/" + kind
}

func (p ProtectedConfig) Validate() error {
	if p.GatewaySN == "" || p.TenantID == 0 || p.ProjectID == 0 || p.Revision < 0 || p.Config == nil {
		return fmt.Errorf("protected configuration requires gateway, scope, version and configuration")
	}
	if p.Kind != "plugin" && p.Kind != "system" && p.Kind != "media_network" {
		return fmt.Errorf("unsupported protected configuration kind %q", p.Kind)
	}
	if p.Kind == "plugin" && (p.PluginName == "" || strings.Contains(p.PluginName, "/")) {
		return fmt.Errorf("invalid protected plugin name")
	}
	if p.Kind != "plugin" && p.PluginName != "" || p.Code != ProtectedConfigCode(p.GatewaySN, p.Kind, p.PluginName) {
		return fmt.Errorf("protected configuration code does not match its gateway")
	}
	return nil
}

func GetProtectedConfigTx(tx *gorm.DB, code string) (*ProtectedConfig, error) {
	tx = tx.Session(&gorm.Session{NewDB: true})
	var row SystemConfig
	if err := tx.Where("key = ?", ProtectedConfigPrefix+code).Take(&row).Error; err != nil {
		return nil, err
	}
	var result ProtectedConfig
	if err := json.Unmarshal([]byte(row.Value), &result); err != nil {
		return nil, err
	}
	return &result, result.Validate()
}

func SaveProtectedConfigTx(tx *gorm.DB, value ProtectedConfig) error {
	tx = tx.Session(&gorm.Session{NewDB: true})
	if err := value.Validate(); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return tx.Set(SyncApplyingKey, true).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&SystemConfig{Key: ProtectedConfigPrefix + value.Code, Value: string(data)}).Error
}

func ListProtectedConfigsTx(tx *gorm.DB) ([]ProtectedConfig, error) {
	tx = tx.Session(&gorm.Session{NewDB: true})
	var rows []SystemConfig
	if err := tx.Where("key LIKE ?", ProtectedConfigPrefix+"%").Order("key").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]ProtectedConfig, 0, len(rows))
	for _, row := range rows {
		var value ProtectedConfig
		if err := json.Unmarshal([]byte(row.Value), &value); err != nil {
			return nil, err
		}
		if err := value.Validate(); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, nil
}

// ProtectedConfigSink records local writes inside their existing transaction.
// Replay uses SyncApplyingKey, so runtime retries never create an echo event.
type ProtectedConfigSink interface {
	RecordLocalPluginTx(*gorm.DB, PluginModel) error
	RecordLocalGlobalConfigTx(*gorm.DB, *config.GlobalConfig) error
	RecordProtectedIntentTx(*gorm.DB, ProtectedConfig, *ProtectedConfig) error
}

var protectedConfigSink ProtectedConfigSink

func SetProtectedConfigSink(sink ProtectedConfigSink) { protectedConfigSink = sink }

// Wake propagation/execution only after the caller's database transaction has
// committed. Replay and bootstrap never invoke this local-write notification.
func NotifyProtectedConfigCommitted(code string) {
	if notifier, ok := protectedConfigSink.(interface{ ProtectedConfigCommitted(string) }); ok {
		notifier.ProtectedConfigCommitted(code)
	}
}

func HasProtectedConfigSink() bool { return protectedConfigSink != nil }

func RecordProtectedIntentTx(tx *gorm.DB, desired ProtectedConfig, base *ProtectedConfig) error {
	if protectedConfigSink == nil {
		return nil
	}
	return protectedConfigSink.RecordProtectedIntentTx(tx, desired, base)
}

func recordLocalGlobalConfigTx(tx *gorm.DB, cfg *config.GlobalConfig) error {
	if _, applying := tx.Get(SyncApplyingKey); applying || protectedConfigSink == nil {
		return nil
	}
	return protectedConfigSink.RecordLocalGlobalConfigTx(tx, cfg)
}

func ProtectedConfigMissing(err error) bool { return errors.Is(err, gorm.ErrRecordNotFound) }

func settleProtectedConfigAckTx(tx *gorm.DB, ch SyncChange, status string) error {
	value, err := GetProtectedConfigTx(tx, ch.EntityCode)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if value.Revision != ch.NewRev || value.SyncState != SyncStatePending {
		return nil
	}
	if status == "conflict" {
		value.SyncState = SyncStateConflict
	} else if status == "applied" {
		value.SyncState = SyncStateSynced
	} else {
		return nil
	}
	return SaveProtectedConfigTx(tx, *value)
}
