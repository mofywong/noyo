package tsdb

import "encoding/json"

// DataType enum
const (
	TypeTelemetry = 1
	TypeEvent     = 2
)

// Record represents a single row in the history table
type Record struct {
	ID         int64           `gorm:"primaryKey;autoIncrement" json:"id"`
	Ts         int64           `gorm:"index:idx_device_ts" json:"ts"` // Unix timestamp in milliseconds
	DeviceCode string          `gorm:"index:idx_device_ts;type:text" json:"device_code"`
	Type       int             `json:"type"`                     // 1=Telemetry, 2=Event
	Payload    json.RawMessage `gorm:"type:text" json:"payload"` // JSON content
}

// TableName overrides the table name
func (Record) TableName() string {
	return "records"
}

// QueryRequest defines the query parameters
type QueryRequest struct {
	DeviceCode string   `json:"device_code"`
	StartTime  int64    `json:"start_time"` // ms
	EndTime    int64    `json:"end_time"`   // ms
	Type       int      `json:"type"`       // 1 or 2
	Keys       []string `json:"keys"`       // Optional, for filtering specific keys in telemetry
	Page       int      `json:"page"`
	PageSize   int      `json:"page_size"`
	Aggregate  bool     `json:"aggregate"`  // Whether to perform aggregation
	MaxPoints  int      `json:"max_points"` // Max points to return (controls aggregation resolution)
	AggMethod  string   `json:"agg_method"` // avg, min, max, median
}

// QueryResponse defines the query result
type QueryResponse struct {
	Total    int64         `json:"total"`
	List     []interface{} `json:"list"`     // List of parsed JSON maps
	Interval int64         `json:"interval"` // Aggregation interval in ms (0 if raw)
}
