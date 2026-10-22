package tsdb

import (
	"context"
	"encoding/json"
)

// DataType enum
const (
	TypeTelemetry = 1
	TypeEvent     = 2
)

// Record represents a single row in the history table
type Record struct {
	ID int64 `gorm:"primaryKey;autoIncrement" json:"id"`
	// NULL for legacy rows; stable across spool replays and partial batch retries.
	IngestID   *string         `gorm:"uniqueIndex:idx_ingest_id" json:"ingest_id,omitempty"`
	Ts         int64           `gorm:"index:idx_device_ts;index:idx_owner_ts,priority:4;index:idx_history_type_ts,priority:2" json:"ts"` // Unix timestamp in milliseconds
	DeviceCode string          `gorm:"index:idx_device_ts;type:text" json:"device_code"`
	DeviceID   uint            `gorm:"index:idx_owner_ts,priority:3;index:idx_device_identity" json:"device_id"`
	TenantID   uint            `gorm:"index:idx_owner_ts,priority:1" json:"tenant_id"`
	ProjectID  uint            `gorm:"index:idx_owner_ts,priority:2" json:"project_id"`
	Type       int             `gorm:"index:idx_history_type_ts,priority:1" json:"type"` // 1=Telemetry, 2=Event
	Payload    json.RawMessage `gorm:"type:text;serializer:json" json:"payload"`         // Accept legacy SQLite TEXT and BLOB JSON.
}

// TableName overrides the table name
func (Record) TableName() string {
	return "records"
}

// QueryRequest defines the query parameters
type QueryRequest struct {
	// Server-only authorization snapshot. A non-nil empty set matches nothing.
	Owners     []RecordOwner   `json:"-" form:"-"`
	Context    context.Context `json:"-" form:"-"`
	SkipTotal  bool            `json:"skip_total"`
	DeviceCode string          `json:"device_code"`
	StartTime  int64           `json:"start_time"` // ms
	EndTime    int64           `json:"end_time"`   // ms
	Type       int             `json:"type"`       // 1 or 2
	Keys       []string        `json:"keys"`       // Optional, for filtering specific keys in telemetry
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	Aggregate  bool            `json:"aggregate"`  // Whether to perform aggregation
	MaxPoints  int             `json:"max_points"` // Max points to return (controls aggregation resolution)
	AggMethod  string          `json:"agg_method"` // avg, min, max, median
}

// QueryResponse defines the query result
type QueryResponse struct {
	HasMore        bool          `json:"has_more"`
	LegacyUnscoped bool          `json:"legacy_unscoped,omitempty"`
	Total          int64         `json:"total"`
	List           []interface{} `json:"list"`     // List of parsed JSON maps
	Interval       int64         `json:"interval"` // Aggregation interval in ms (0 if raw)
}

// RecordOwner binds history to an immutable device identity and its ingestion scope.
type RecordOwner struct {
	DeviceID  uint `json:"device_id"`
	TenantID  uint `json:"tenant_id"`
	ProjectID uint `json:"project_id"`
}
