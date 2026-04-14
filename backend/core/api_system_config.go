package core

import (
	"encoding/json"
	"noyo/core/config"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// handleGetSystemConfig returns the current system configuration
func (s *Server) handleGetSystemConfig(r *ghttp.Request) {
	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": s.Config,
	})
}

// handleUpdateSystemConfig updates the system configuration
func (s *Server) handleUpdateSystemConfig(r *ghttp.Request) {
	var req config.GlobalConfig
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	// Update config in memory
	s.Config.Server = req.Server
	s.Config.TSDB = req.TSDB
	s.Config.Log = req.Log

	// Save to database
	if err := store.SaveGlobalConfig(s.Config); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to save config: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{
		"code":    0,
		"message": "System configuration updated. A restart may be required for some changes to take effect.",
	})
}
