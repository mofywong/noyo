package store

import (
	"errors"
	"time"

	"noyo/core/llmlog"

	"gorm.io/gorm"
)

// LLMRequest is the durable wire-level record shared by all model providers.
type LLMRequest = llmlog.Record

func init() {
	llmlog.SetSink(func(record *llmlog.Record) error {
		if DB == nil {
			return errors.New("LLM request database is unavailable")
		}
		// A cancelled inference context must not cancel its audit write.
		return DB.Save(record).Error
	})
}

// Only called during startup. Preserve unknown completion time and usage when
// the previous process stopped before it could finalize an in-flight record.
func recoverInterruptedLLMRequests(db *gorm.DB) error {
	return db.Model(&LLMRequest{}).Where("status = ?", "pending").Updates(map[string]interface{}{
		"status": "error", "error": "Process restarted before the full response was recorded / 进程重启，请求结果未能完整记录",
	}).Error
}

type LLMRequestScope struct {
	TenantID         uint
	AllTenants       bool
	RestrictProjects bool
	ProjectIDs       []uint
}

type LLMRequestFilter struct {
	Model, Provider, Source, SessionID, Status, Search string
	Stream                                             *bool
	StartedAfter, StartedBefore                        *time.Time
	MinDurationMS, MaxDurationMS                       *int64
}

func llmRequestQuery(db *gorm.DB, scope LLMRequestScope, filter LLMRequestFilter) *gorm.DB {
	query := db.Model(&LLMRequest{})
	if !scope.AllTenants {
		if scope.TenantID == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("tenant_id = ?", scope.TenantID)
		}
	}
	if scope.RestrictProjects {
		if len(scope.ProjectIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("project_id IN ?", scope.ProjectIDs)
		}
	}
	for _, field := range []struct{ column, value string }{
		{"model", filter.Model}, {"provider", filter.Provider}, {"source", filter.Source},
		{"session_id", filter.SessionID}, {"status", filter.Status},
	} {
		if field.value != "" {
			query = query.Where(field.column+" = ?", field.value)
		}
	}
	if filter.Stream != nil {
		query = query.Where("stream = ?", *filter.Stream)
	}
	if filter.StartedAfter != nil {
		query = query.Where("started_at >= ?", *filter.StartedAfter)
	}
	if filter.StartedBefore != nil {
		query = query.Where("started_at <= ?", *filter.StartedBefore)
	}
	if filter.MinDurationMS != nil {
		query = query.Where("duration_ms >= ?", *filter.MinDurationMS)
	}
	if filter.MaxDurationMS != nil {
		query = query.Where("duration_ms <= ?", *filter.MaxDurationMS)
	}
	if filter.Search != "" {
		query = query.Where("(instr(request_body, ?) > 0 OR instr(response_body, ?) > 0 OR instr(error, ?) > 0 OR instr(request_id, ?) > 0)", filter.Search, filter.Search, filter.Search, filter.Search)
	}
	return query
}

func ListLLMRequests(db *gorm.DB, scope LLMRequestScope, filter LLMRequestFilter, page, pageSize int) ([]LLMRequest, int64, error) {
	if page < 1 || pageSize < 1 || pageSize > 100 {
		return nil, 0, errors.New("invalid pagination")
	}
	var total int64
	query := llmRequestQuery(db, scope, filter)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	rows := make([]LLMRequest, 0)
	// Large payloads are only loaded when opening the scoped detail endpoint.
	err := query.Omit("request_body", "response_body", "usage_json", "response_headers").
		Order("started_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	return rows, total, err
}

func FindLLMRequest(db *gorm.DB, scope LLMRequestScope, id uint) (*LLMRequest, error) {
	var row LLMRequest
	err := llmRequestQuery(db, scope, LLMRequestFilter{}).Where("id = ?", id).First(&row).Error
	return &row, err
}

type LLMRequestStats struct {
	Requests              int64   `json:"requests"`
	UsageReportedRequests int64   `json:"usage_reported_requests"`
	UsageMissingRequests  int64   `json:"usage_missing_requests"`
	Successes             int64   `json:"successes"`
	Errors                int64   `json:"errors"`
	PromptTokens          int64   `json:"prompt_tokens"`
	CompletionTokens      int64   `json:"completion_tokens"`
	TotalTokens           int64   `json:"total_tokens"`
	CachedTokens          int64   `json:"cached_tokens"`
	ReasoningTokens       int64   `json:"reasoning_tokens"`
	AvgDurationMS         float64 `json:"avg_duration_ms"`
	MaxDurationMS         int64   `json:"max_duration_ms"`
}

type LLMRequestGroup struct {
	Key             string `json:"key"`
	LLMRequestStats `gorm:"embedded"`
}

type LLMRequestSummary struct {
	Totals LLMRequestStats   `json:"totals"`
	Groups []LLMRequestGroup `json:"groups"`
}

const llmStatsSelect = `COUNT(*) AS requests,
	COALESCE(SUM(CASE WHEN usage_available THEN 1 ELSE 0 END), 0) AS usage_reported_requests,
	COALESCE(SUM(CASE WHEN NOT usage_available THEN 1 ELSE 0 END), 0) AS usage_missing_requests,
	COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS successes,
	COALESCE(SUM(CASE WHEN status IN ('error', 'cancelled') THEN 1 ELSE 0 END), 0) AS errors,
	COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens,
	COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
	COALESCE(SUM(total_tokens), 0) AS total_tokens,
	COALESCE(SUM(cached_tokens), 0) AS cached_tokens,
	COALESCE(SUM(reasoning_tokens), 0) AS reasoning_tokens,
	COALESCE(AVG(CASE WHEN finished_at IS NOT NULL THEN duration_ms END), 0) AS avg_duration_ms,
	COALESCE(MAX(duration_ms), 0) AS max_duration_ms`

// LLMRequestPeriods holds the same totals for four fixed windows so the
// overview cards never depend on the active filter or the current page.
type LLMRequestPeriods struct {
	Today LLMRequestStats `json:"today"`
	Week  LLMRequestStats `json:"week"`
	Month LLMRequestStats `json:"month"`
	All   LLMRequestStats `json:"all"`
}

func llmRequestStatsSince(db *gorm.DB, scope LLMRequestScope, since *time.Time) (LLMRequestStats, error) {
	var stats LLMRequestStats
	query := llmRequestQuery(db, scope, LLMRequestFilter{})
	if since != nil {
		// Stored timestamps are UTC text; comparing a local-time string would
		// silently shift the window by the zone offset.
		utc := since.UTC()
		query = query.Where("started_at >= ?", utc)
	}
	err := query.Select(llmStatsSelect).Scan(&stats).Error
	return stats, err
}

// SummarizeLLMRequestPeriods measures today, this week (Monday based), this
// month and all time in the supplied location.
func SummarizeLLMRequestPeriods(db *gorm.DB, scope LLMRequestScope, now time.Time, loc *time.Location) (*LLMRequestPeriods, error) {
	if loc == nil {
		loc = time.Local
	}
	local := now.In(loc)
	startOfDay := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	startOfWeek := startOfDay.AddDate(0, 0, -((int(local.Weekday()) + 6) % 7))
	startOfMonth := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, loc)
	result := &LLMRequestPeriods{}
	for _, window := range []struct {
		target *LLMRequestStats
		since  *time.Time
	}{
		{&result.Today, &startOfDay}, {&result.Week, &startOfWeek},
		{&result.Month, &startOfMonth}, {&result.All, nil},
	} {
		stats, err := llmRequestStatsSince(db, scope, window.since)
		if err != nil {
			return nil, err
		}
		*window.target = stats
	}
	return result, nil
}

type LLMRequestFacet struct {
	Value string `json:"value"`
	Title string `json:"title,omitempty"`
	Count int64  `json:"count"`
}

type LLMRequestFacets struct {
	Sources  []LLMRequestFacet `json:"sources"`
	Sessions []LLMRequestFacet `json:"sessions"`
}

// LLMSessionRef keeps the tenant and project so a reused session identifier never
// borrows another scope's conversation title.
type LLMSessionRef struct {
	TenantID, ProjectID uint
	SessionID           string
}

// LLMRequestFacetLimit caps how many distinct values reach the filter lists.
const LLMRequestFacetLimit = 200

func ListLLMRequestFacets(db *gorm.DB, scope LLMRequestScope) (*LLMRequestFacets, error) {
	result := &LLMRequestFacets{Sources: make([]LLMRequestFacet, 0), Sessions: make([]LLMRequestFacet, 0)}
	err := llmRequestQuery(db, scope, LLMRequestFilter{}).
		Select("source AS value, COUNT(*) AS count").Group("source").
		Order("count DESC, value ASC").Limit(LLMRequestFacetLimit).Scan(&result.Sources).Error
	if err != nil {
		return nil, err
	}
	var sessions []struct {
		TenantID  uint
		ProjectID uint
		Value     string
		Count     int64
	}
	err = llmRequestQuery(db, scope, LLMRequestFilter{}).
		Select("tenant_id, project_id, session_id AS value, COUNT(*) AS count").
		Where("session_id <> ''").Group("tenant_id, project_id, session_id").
		Order("count DESC, value ASC").Limit(LLMRequestFacetLimit).Scan(&sessions).Error
	if err != nil {
		return nil, err
	}
	refs := make([]LLMSessionRef, 0, len(sessions))
	for _, row := range sessions {
		refs = append(refs, LLMSessionRef{row.TenantID, row.ProjectID, row.Value})
	}
	titles, err := LLMSessionTitles(db, refs)
	if err != nil {
		return nil, err
	}
	for _, row := range sessions {
		result.Sessions = append(result.Sessions, LLMRequestFacet{
			Value: row.Value, Count: row.Count,
			Title: titles[LLMSessionRef{row.TenantID, row.ProjectID, row.Value}],
		})
	}
	return result, nil
}

// LLMSessionTitles resolves conversation titles for the given requests. Missing
// or untitled conversations simply return no entry.
func LLMSessionTitles(db *gorm.DB, refs []LLMSessionRef) (map[LLMSessionRef]string, error) {
	titles := make(map[LLMSessionRef]string, len(refs))
	if len(refs) == 0 {
		return titles, nil
	}
	tenants := make([]uint, 0, len(refs))
	projects := make([]uint, 0, len(refs))
	ids := make([]string, 0, len(refs))
	seenTenant, seenProject, seenID := map[uint]bool{}, map[uint]bool{}, map[string]bool{}
	for _, ref := range refs {
		if ref.SessionID == "" {
			continue
		}
		if !seenTenant[ref.TenantID] {
			seenTenant[ref.TenantID] = true
			tenants = append(tenants, ref.TenantID)
		}
		if !seenProject[ref.ProjectID] {
			seenProject[ref.ProjectID] = true
			projects = append(projects, ref.ProjectID)
		}
		if !seenID[ref.SessionID] {
			seenID[ref.SessionID] = true
			ids = append(ids, ref.SessionID)
		}
	}
	if len(ids) == 0 {
		return titles, nil
	}
	var rows []struct {
		TenantID       uint
		ProjectID      uint
		ConversationID string
		Title          string
	}
	err := db.Model(&IOCScreenConversation{}).
		Select("tenant_id, project_id, conversation_id, title").
		Where("tenant_id IN ? AND project_id IN ? AND conversation_id IN ?", tenants, projects, ids).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Title != "" {
			titles[LLMSessionRef{row.TenantID, row.ProjectID, row.ConversationID}] = row.Title
		}
	}
	return titles, nil
}

func SummarizeLLMRequests(db *gorm.DB, scope LLMRequestScope, filter LLMRequestFilter, groupBy string) (*LLMRequestSummary, error) {
	// Expressions are an allowlist; no client-supplied SQL enters GROUP BY.
	column, ok := map[string]string{
		"model": "model", "provider": "provider", "source": "source", "session": "session_id",
		"status": "status", "day": "strftime('%Y-%m-%d', started_at)",
	}[groupBy]
	if !ok {
		return nil, errors.New("invalid aggregation group")
	}
	result := &LLMRequestSummary{Groups: make([]LLMRequestGroup, 0)}
	if err := llmRequestQuery(db, scope, filter).Select(llmStatsSelect).Scan(&result.Totals).Error; err != nil {
		return nil, err
	}
	err := llmRequestQuery(db, scope, filter).Select(column + " AS key, " + llmStatsSelect).
		Group(column).Order("requests DESC, key ASC").Scan(&result.Groups).Error
	return result, err
}
