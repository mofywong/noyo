package tsdb

// TimeSeriesStore defines the interface for time-series storage
type TimeSeriesStore interface {
	// Init initializes the storage
	Init() error

	// Close closes the storage connections
	Close() error

	// WriteBatch writes a batch of records
	WriteBatch(records []*Record) error

	// Query executes a query
	Query(req QueryRequest) (*QueryResponse, error)

	// Prune removes old data exceeding retention period
	Prune(retentionDays int)
}
