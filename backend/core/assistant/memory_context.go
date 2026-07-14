package assistant

// EntityRef binds assistant memory retrieval to concrete runtime objects.
type EntityRef struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type MemoryContextRequest struct {
	Query         string      `json:"query"`
	Limit         int         `json:"limit"`
	EntityContext []EntityRef `json:"entity_context,omitempty"`
}

// MemoryContext contains data-only, policy-filtered memory claims.
type MemoryContext struct {
	Items         []MemoryContextItem `json:"items"`
	ReturnedCount int                 `json:"returned_count"`
}

type MemoryContextItem struct {
	Code       string `json:"code"`
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	TrustLevel string `json:"trust_level"`
	ScopeType  string `json:"scope_type"`
	EntityType string `json:"entity_type,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
}
