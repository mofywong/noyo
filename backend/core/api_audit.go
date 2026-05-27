package core

import (
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *Server) handleListAuditLogs(r *ghttp.Request) {
	tenantID := r.GetCtxVar("tenant_id").Uint()
	
	db := store.DB.Model(&store.AuditLog{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}

	// Pagination
	page := r.Get("page", 1).Int()
	pageSize := r.Get("page_size", 20).Int()

	// Filters
	if module := r.Get("module").String(); module != "" {
		db = db.Where("module = ?", module)
	}
	if action := r.Get("action").String(); action != "" {
		db = db.Where("action = ?", action)
	}
	if username := r.Get("username").String(); username != "" {
		db = db.Where("username LIKE ?", "%"+username+"%")
	}

	var total int64
	db.Count(&total)

	var logs []store.AuditLog
	if err := db.Order("created_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&logs).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch audit logs"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": logs, "total": total})
}
