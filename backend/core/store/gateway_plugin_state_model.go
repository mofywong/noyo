package store

import "time"

// GatewayPluginStateModel stores the platform-side desired state for one
// remotely managed gateway plugin.
type GatewayPluginStateModel struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	GatewaySN       string    `gorm:"index:idx_gateway_plugin_state,unique" json:"gw_sn"`
	PluginName      string    `gorm:"index:idx_gateway_plugin_state,unique" json:"plugin_name"`
	DesiredConfig   string    `json:"desired_config"`
	DesiredEnabled  bool      `json:"desired_enabled"`
	SchemaSnapshot  string    `json:"schema_snapshot"`
	SummarySnapshot string    `json:"summary_snapshot"`
	BaseVersion     int64     `json:"base_version"`
	GatewayVersion  int64     `json:"gateway_version"`
	SyncState       string    `json:"sync_state"`
	EnabledAt       int64     `json:"enabled_at"`
	LastSyncedAt    int64     `json:"last_synced_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (GatewayPluginStateModel) TableName() string {
	return "gateway_plugin_states"
}
