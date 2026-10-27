package core

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func llmRequestScope(auth *AuthContext) store.LLMRequestScope {
	if auth == nil {
		return store.LLMRequestScope{RestrictProjects: true}
	}
	scope := store.LLMRequestScope{TenantID: auth.TenantID, AllTenants: auth.IsSystemAdmin && auth.TenantID == 0}
	if auth.SubjectType == "app" {
		// An app can hold this permission in one project while accessing other
		// projects for unrelated capabilities. Scope each project explicitly.
		scope.RestrictProjects = true
		projects, _ := auth.ProjectIDsForTenantQuery()
		for _, projectID := range projects {
			if auth.HasProjectPermission("llm_request:list", projectID) {
				scope.ProjectIDs = append(scope.ProjectIDs, projectID)
			}
		}
		return scope
	}
	if auth.ProjectID > 0 {
		scope.RestrictProjects, scope.ProjectIDs = true, []uint{auth.ProjectID}
	} else if !auth.IsSystemAdmin {
		scope.ProjectIDs, scope.RestrictProjects = auth.ProjectIDsForTenantQuery()
	}
	return scope
}

func parseLLMRequestFilter(get func(string) string) (store.LLMRequestFilter, error) {
	filter := store.LLMRequestFilter{
		Model: strings.TrimSpace(get("model")), Provider: strings.TrimSpace(get("provider")),
		Source: strings.TrimSpace(get("source")), SessionID: strings.TrimSpace(get("session_id")),
		Status: get("status"), Search: get("search"),
	}
	for _, value := range []string{filter.Model, filter.Provider, filter.Source, filter.SessionID, filter.Search} {
		if len(value) > 4096 {
			return filter, errors.New("Filter is too long / 筛选条件过长")
		}
	}
	switch filter.Status {
	case "", "pending", "success", "error", "cancelled":
	default:
		return filter, errors.New("Invalid request status / 无效的请求状态")
	}
	if raw := get("stream"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return filter, errors.New("Invalid stream filter / 无效的流式筛选条件")
		}
		filter.Stream = &value
	}
	for _, field := range []struct {
		name   string
		target **time.Time
	}{
		{"started_after", &filter.StartedAfter}, {"started_before", &filter.StartedBefore},
	} {
		if raw := get(field.name); raw != "" {
			value, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				return filter, errors.New("Dates must use ISO 8601 with a time zone / 时间须包含时区并使用 ISO 8601 格式")
			}
			value = value.UTC()
			*field.target = &value
		}
	}
	for _, field := range []struct {
		name   string
		target **int64
	}{
		{"min_duration_ms", &filter.MinDurationMS}, {"max_duration_ms", &filter.MaxDurationMS},
	} {
		if raw := get(field.name); raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || value < 0 {
				return filter, errors.New("Duration must be a non-negative integer / 耗时须为非负整数")
			}
			*field.target = &value
		}
	}
	if filter.StartedAfter != nil && filter.StartedBefore != nil && filter.StartedAfter.After(*filter.StartedBefore) {
		return filter, errors.New("Start must precede end / 开始时间不能晚于结束时间")
	}
	if filter.MinDurationMS != nil && filter.MaxDurationMS != nil && *filter.MinDurationMS > *filter.MaxDurationMS {
		return filter, errors.New("Minimum duration exceeds maximum / 最小耗时不能大于最大耗时")
	}
	return filter, nil
}

func llmRequestError(r *ghttp.Request, status int, message string) {
	r.Response.WriteStatus(status, g.Map{"code": status, "message": message})
}

// llmRequestView adds the resolved conversation title so the client can present
// "name (id)" instead of a bare identifier.
type llmRequestView struct {
	store.LLMRequest
	SessionTitle string `json:"session_title"`
}

func llmRequestViews(rows []store.LLMRequest) []llmRequestView {
	views := make([]llmRequestView, 0, len(rows))
	refs := make([]store.LLMSessionRef, 0, len(rows))
	for _, row := range rows {
		views = append(views, llmRequestView{LLMRequest: row})
		if row.SessionID != "" {
			refs = append(refs, store.LLMSessionRef{TenantID: row.TenantID, ProjectID: row.ProjectID, SessionID: row.SessionID})
		}
	}
	titles, err := store.LLMSessionTitles(store.DB, refs)
	if err == nil {
		for index, view := range views {
			views[index].SessionTitle = titles[store.LLMSessionRef{TenantID: view.TenantID, ProjectID: view.ProjectID, SessionID: view.SessionID}]
		}
	}
	return views
}

func (s *Server) handleListLLMRequests(r *ghttp.Request) {
	filter, err := parseLLMRequestFilter(func(key string) string { return r.GetQuery(key).String() })
	if err != nil {
		llmRequestError(r, http.StatusBadRequest, err.Error())
		return
	}
	page, errPage := strconv.Atoi(r.GetQuery("page", "1").String())
	size, errSize := strconv.Atoi(r.GetQuery("page_size", "20").String())
	if errPage != nil || errSize != nil || page < 1 || page > 1000000 || size < 1 || size > 100 {
		llmRequestError(r, http.StatusBadRequest, "Invalid pagination / 无效的分页参数")
		return
	}
	rows, total, err := store.ListLLMRequests(store.DB, llmRequestScope(requestAuthContext(r)), filter, page, size)
	if err != nil {
		llmRequestError(r, http.StatusInternalServerError, "Could not load LLM requests / 无法加载 LLM 请求")
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": llmRequestViews(rows), "total": total, "page": page, "page_size": size})
}

// handleLLMRequestStats reports today, this week, this month and all time. It
// deliberately ignores the active filter: the cards answer "how much has the
// system been used", not "what does this query contain".
func (s *Server) handleLLMRequestStats(r *ghttp.Request) {
	result, err := store.SummarizeLLMRequestPeriods(store.DB, llmRequestScope(requestAuthContext(r)), time.Now(), time.Local)
	if err != nil {
		llmRequestError(r, http.StatusInternalServerError, "Could not aggregate LLM requests / 无法聚合 LLM 请求")
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": result})
}

// handleLLMRequestFacets supplies the option lists for the source and session
// filters, including conversation titles for "name (id)" labels.
func (s *Server) handleLLMRequestFacets(r *ghttp.Request) {
	result, err := store.ListLLMRequestFacets(store.DB, llmRequestScope(requestAuthContext(r)))
	if err != nil {
		llmRequestError(r, http.StatusInternalServerError, "Could not load LLM request filters / 无法加载 LLM 请求筛选项")
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": result})
}

func (s *Server) handleLLMRequestSummary(r *ghttp.Request) {
	filter, err := parseLLMRequestFilter(func(key string) string { return r.GetQuery(key).String() })
	if err != nil {
		llmRequestError(r, http.StatusBadRequest, err.Error())
		return
	}
	group := r.GetQuery("group_by", "model").String()
	switch group {
	case "model", "provider", "source", "session", "day", "status":
	default:
		llmRequestError(r, http.StatusBadRequest, "Invalid aggregation group / 无效的聚合维度")
		return
	}
	result, err := store.SummarizeLLMRequests(store.DB, llmRequestScope(requestAuthContext(r)), filter, group)
	if err != nil {
		llmRequestError(r, http.StatusInternalServerError, "Could not aggregate LLM requests / 无法聚合 LLM 请求")
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": result})
}

func (s *Server) handleGetLLMRequest(r *ghttp.Request) {
	id, err := strconv.ParseUint(r.GetRouter("id").String(), 10, strconv.IntSize)
	if err != nil || id == 0 {
		llmRequestError(r, http.StatusBadRequest, "Invalid request ID / 无效的请求编号")
		return
	}
	row, err := store.FindLLMRequest(store.DB, llmRequestScope(requestAuthContext(r)), uint(id))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		llmRequestError(r, http.StatusNotFound, "Request not found / 请求不存在")
		return
	}
	if err != nil {
		llmRequestError(r, http.StatusInternalServerError, "Could not load request details / 无法加载请求详情")
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": llmRequestViews([]store.LLMRequest{*row})[0]})
}
