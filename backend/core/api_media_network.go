package core

import (
	"encoding/json"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func requireMediaNetworkAdmin(r *ghttp.Request) bool {
	ctx := requestAuthContext(r)
	if ctx == nil || !ctx.IsSystemAdmin || ctx.SubjectType == "app" {
		r.Response.WriteJson(g.Map{"code": 403, "message": "System administrator access required"})
		return false
	}
	r.Response.Header().Set("Cache-Control", "no-store")
	return true
}

func (s *Server) handleGetMediaNetwork(r *ghttp.Request) {
	if !requireMediaNetworkAdmin(r) {
		return
	}
	cfg, source, err := loadMediaNetworkConfig()
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 503, "message": "Media network configuration unavailable"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": cfg.View(source)})
}

func (s *Server) handleUpdateMediaNetwork(r *ghttp.Request) {
	if !requireMediaNetworkAdmin(r) {
		return
	}
	var req mediaNetworkUpdate
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	cfg, err := saveMediaNetworkConfig(req)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": cfg.View("database")})
}
