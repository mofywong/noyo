// Package llmlog records each outbound model API exchange independently of the
// SDK and of the application workflow consuming its response.
package llmlog

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sync"
	"time"
)

type Metadata struct {
	TenantID, ProjectID, UserID uint
	SessionID, Source, Provider string
}

type Record struct {
	ID                uint       `gorm:"primaryKey" json:"id"`
	TenantID          uint       `gorm:"index:idx_llm_scope_time,priority:1;index" json:"tenant_id"`
	ProjectID         uint       `gorm:"index:idx_llm_scope_time,priority:2;index" json:"project_id"`
	UserID            uint       `gorm:"index" json:"user_id"`
	SessionID         string     `gorm:"size:256;index" json:"session_id"`
	Source            string     `gorm:"size:128;index" json:"source"`
	Provider          string     `gorm:"size:128;index" json:"provider"`
	Model             string     `gorm:"size:256;index" json:"model"`
	Endpoint          string     `gorm:"type:text" json:"endpoint"`
	Method            string     `gorm:"size:16" json:"method"`
	StartedAt         time.Time  `gorm:"index:idx_llm_scope_time,priority:3;index" json:"started_at"`
	FinishedAt        *time.Time `json:"finished_at"`
	DurationMS        int64      `json:"duration_ms"`
	FirstByteMS       int64      `json:"first_byte_ms"`
	Status            string     `gorm:"size:24;index" json:"status"`
	StatusCode        int        `json:"status_code"`
	Stream            bool       `json:"stream"`
	RequestBody       string     `gorm:"type:text" json:"request_body"`
	ResponseBody      string     `gorm:"type:text" json:"response_body"`
	ResponseHeaders   string     `gorm:"type:text" json:"response_headers"`
	UsageJSON         string     `gorm:"type:text" json:"usage_json"`
	UsageAvailable    bool       `json:"usage_available"`
	PromptTokens      int64      `json:"prompt_tokens"`
	CompletionTokens  int64      `json:"completion_tokens"`
	TotalTokens       int64      `json:"total_tokens"`
	CachedTokens      int64      `json:"cached_tokens"`
	ReasoningTokens   int64      `json:"reasoning_tokens"`
	Error             string     `gorm:"type:text" json:"error"`
	RequestID         string     `gorm:"size:128;uniqueIndex" json:"request_id"`
	ProviderRequestID string     `gorm:"size:256" json:"provider_request_id"`
	ResponseID        string     `gorm:"size:256" json:"response_id"`
	FinishReason      string     `gorm:"size:128" json:"finish_reason"`
}

func (Record) TableName() string { return "llm_requests" }

type metadataKey struct{}

// Merge only replaces fields explicitly supplied by the caller.
func Merge(base, override Metadata) Metadata {
	if override.TenantID != 0 {
		base.TenantID = override.TenantID
	}
	if override.ProjectID != 0 {
		base.ProjectID = override.ProjectID
	}
	if override.UserID != 0 {
		base.UserID = override.UserID
	}
	if override.SessionID != "" {
		base.SessionID = override.SessionID
	}
	if override.Source != "" {
		base.Source = override.Source
	}
	if override.Provider != "" {
		base.Provider = override.Provider
	}
	return base
}

func FromContext(ctx context.Context) Metadata {
	if ctx == nil {
		return Metadata{}
	}
	value, _ := ctx.Value(metadataKey{}).(Metadata)
	return value
}

func WithMetadata(ctx context.Context, metadata Metadata) context.Context {
	return context.WithValue(ctx, metadataKey{}, Merge(FromContext(ctx), metadata))
}

func NewSessionID(prefix string) string {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return prefix + time.Now().UTC().Format("20060102T150405.000000000")
	}
	return prefix + hex.EncodeToString(random[:])
}

var sinkState struct {
	sync.RWMutex
	fn func(*Record) error
}

// SetSink installs the database writer. Each exchange is saved before network
// dispatch and updated when its body completes, fails, or is closed.
func SetSink(fn func(*Record) error) {
	sinkState.Lock()
	sinkState.fn = fn
	sinkState.Unlock()
}

func persist(record *Record) error {
	sinkState.RLock()
	fn := sinkState.fn
	sinkState.RUnlock()
	if fn != nil {
		if err := fn(record); err != nil {
			log.Printf("LLM request log persistence failed request_id=%s: %v", record.RequestID, err)
			return err
		}
	}
	return nil
}
