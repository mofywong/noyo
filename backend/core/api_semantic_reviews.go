package core

import (
	"errors"
	"net/http"
	"noyo/core/semanticreview"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) handleSemanticReviewRecords(r *ghttp.Request) {
	r.Response.Header().Set("Cache-Control", "no-store")
	query, code := semanticReviewRecordQuery(r)
	if code != 0 {
		writeSemanticReviewRecordError(r, code)
		return
	}
	writeSemanticReviewRecords(r, query)
}

func semanticReviewRecordQuery(r *ghttp.Request) (semanticreview.RecordQuery, int) {
	query := semanticreview.RecordQuery{DeviceCode: r.Get("device_code").String(), Status: r.Get("status").String(), ID: r.Get("id").String(), Page: r.Get("page", 1).Int(), PageSize: r.Get("page_size", 20).Int()}
	auth := requestAuthContext(r)
	if auth == nil {
		return query, http.StatusForbidden
	}
	filter := store.DeviceQueryFilter{TenantID: r.GetCtxVar("tenant_id").Uint(), ProjectID: r.GetCtxVar("project_id").Uint()}
	if ids, restricted := auth.ProjectIDsForTenantQuery(); restricted && filter.ProjectID == 0 {
		if len(ids) == 0 {
			return query, 0
		}
		filter.AllowedProjectIDs = ids
	}
	if code := r.Get("device_code").String(); code != "" {
		filter.IncludeCodes = []string{code}
	}
	devices, _, err := store.ListDevicesWithFilter(0, 0, filter)
	if err != nil {
		return query, http.StatusInternalServerError
	}
	codes := make([]string, 0, len(devices))
	for _, d := range devices {
		if canAccessDevice(r, &d) {
			codes = append(codes, d.Code)
		}
	}
	query.DeviceCodes = codes
	return query, 0
}

func (s *Server) handleSemanticReviewEvidence(r *ghttp.Request) {
	r.Response.Header().Set("Cache-Control", "no-store")
	r.Response.Header().Set("X-Content-Type-Options", "nosniff")
	query, code := semanticReviewRecordQuery(r)
	if code != 0 {
		r.Response.WriteHeader(code)
		writeSemanticReviewRecordError(r, code)
		return
	}
	frame, err := semanticreview.RecordEvidence(store.DB, query, r.Get("frame_id").String())
	if err != nil || (frame.MIME != "image/jpeg" && frame.MIME != "image/png") {
		code = http.StatusNotFound
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			code = http.StatusInternalServerError
		}
		r.Response.WriteHeader(code)
		writeSemanticReviewRecordError(r, code)
		return
	}
	r.Response.Header().Set("Content-Type", frame.MIME)
	r.Response.Write(frame.Data)
}

func writeSemanticReviewRecordError(r *ghttp.Request, code int) {
	message := "Cannot load review records / 无法加载复核记录"
	if code == http.StatusForbidden {
		message = "Device access denied / 无设备访问权限"
	}
	if code == http.StatusNotFound {
		message = "Review evidence unavailable / 复核证据不可用"
	}
	r.Response.WriteJson(g.Map{"code": code, "message": message})
}

func writeSemanticReviewRecords(r *ghttp.Request, filter semanticreview.RecordQuery) {
	records, total, err := semanticreview.ListRecords(store.DB, filter)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Cannot load review records / 无法加载复核记录"})
		return
	}
	if filter.ID != "" && len(records) == 0 {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Review record unavailable / 复核记录不可用"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"items": records, "total": total}})
}
