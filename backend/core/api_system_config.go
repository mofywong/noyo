package core

import (
	"encoding/json"
	"os"
	"time"

	"noyo/core/alarmmedia"
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
	req := *s.Config
	req.AlarmMedia.Normalize()
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	if err := req.AlarmMedia.Validate(); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}

	portChanged := req.Server.Port > 0 && s.Config.Server.Port != req.Server.Port

	next := *s.Config
	next.Server = req.Server
	next.TSDB = req.TSDB
	next.Log = req.Log
	next.AlarmMedia = req.AlarmMedia

	// Save to database
	if err := store.SaveGlobalConfig(&next); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to save config: " + err.Error()})
		return
	}
	if store.LocalConfigGatewayCode() == "" || !store.HasProtectedConfigSink() {
		*s.Config = next
		_ = alarmmedia.Configure(next.AlarmMedia)
		SetLoggerLevel(req.Log.Level)
		store.SetDBLogLevel(req.Log.Level)
	}

	r.Response.WriteJson(g.Map{
		"code":    0,
		"message": "System configuration updated. A restart may be required for some changes to take effect.",
	})

	if portChanged {
		go func() {
			s.Logger.Info("HTTP port changed, exiting process to allow daemon restart...")
			time.Sleep(2 * time.Second)
			os.Exit(0)
		}()
	}
}
