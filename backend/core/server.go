package core

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"noyo/core/config"
	"noyo/core/protocol"
	"noyo/core/store"
	"noyo/core/system"
	"noyo/core/tsdb"
	"noyo/core/types"

	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

// Server represents the gateway core server
type Server struct {
	Config          *config.GlobalConfig
	Logger          *zap.Logger
	Manager         *PluginManager
	DeviceManager   *DeviceManager
	DispatchService *DispatchService
	TSDB            *tsdb.TSDBManager
	WebServer       *ghttp.Server
	uiFS            fs.FS
}

func (s *Server) SetUI(uiFS fs.FS) {
	s.uiFS = uiFS
}

// NewServer creates a new server instance
func NewServer() (*Server, error) {
	// 0. Init Database
	// Default to sqlite with file noyo.db
	if err := store.InitDB("./data/db/noyo.db"); err != nil {
		return nil, err
	}

	// 1. Load Config from Database
	cfg, err := store.LoadGlobalConfig()
	if err != nil {
		// If not found or error, use default and save it
		cfg = config.DefaultConfig()
		if saveErr := store.SaveGlobalConfig(cfg); saveErr != nil {
			// ignore save error, just use default
		}
	}

	logger := InitLogger(cfg.Log)

	if err != nil && err.Error() != "record not found" {
		logger.Warn("Failed to load global config from db, using defaults", zap.Error(err))
	}

	s := &Server{
		Config: cfg,
		Logger: logger,
	}
	s.Manager = NewPluginManager(s)
	s.DeviceManager = NewDeviceManager(s)
	s.DispatchService = NewDispatchService(s.Manager, s.DeviceManager.Registry, s.DeviceManager.EventBus, logger)
	s.TSDB = tsdb.NewManager(cfg.TSDB, logger)
	s.DeviceManager.TSDB = s.TSDB // Inject TSDB into DeviceManager

	return s, nil
}

// Run starts the server and blocks until an interrupt signal is received
func (s *Server) Run() error {
	s.Logger.Info("Starting Gateway Server...")

	// Start TSDB
	if err := s.TSDB.Init(); err != nil {
		s.Logger.Error("Failed to init TSDB", zap.Error(err))
	} else {
		s.TSDB.Start()
		defer s.TSDB.Stop()
	}

	// Initialize Web Server (so plugins can register routes)
	s.WebServer = g.Server()
	s.WebServer.SetRouteOverWrite(true) // Enable route overwrite for plugin reload
	s.WebServer.SetPort(s.Config.Server.Port)

	// API Routes
	s.WebServer.Group("/api", func(group *ghttp.RouterGroup) {
		group.GET("/plugins", s.handleListPlugins)
		group.GET("/plugins/:name", s.handleGetPlugin)
		group.GET("/plugins/:name/schemas", s.handleGetPluginSchemas)
		group.POST("/plugins/:name/config", s.handleUpdatePluginConfig)
		group.POST("/plugins/:name/discover", s.handlePluginDiscover)
		group.POST("/history/query", s.handleQueryHistory) // Add History Query API
		group.GET("/system/stats", s.handleSystemStats)
		s.RegisterDeviceRoutes(group)
	})

	// System Settings / Logs
	s.WebServer.BindHandler("GET:/api/system/config", s.handleGetSystemConfig)
	s.WebServer.BindHandler("POST:/api/system/config", s.handleUpdateSystemConfig)
	s.WebServer.BindHandler("GET:/api/system/log/config", s.handleGetLogConfig)
	s.WebServer.BindHandler("POST:/api/system/log/config", s.handleUpdateLogConfig)
	s.WebServer.BindHandler("GET:/api/system/log/files", s.handleListLogFiles)
	s.WebServer.BindHandler("GET:/api/system/log/file", s.handleReadLogFile)
	s.WebServer.BindHandler("GET:/api/system/log/tail", s.handleTailLog)
	s.WebServer.BindHandler("GET:/api/system/log/download", s.handleDownloadLogFile)
	s.WebServer.BindHandler("/api/system/log/stream", s.handleRealtimeLogs)

	// Serve UI
	if s.uiFS != nil {
		s.WebServer.BindHandler("/*", func(r *ghttp.Request) {
			// Skip API
			if strings.HasPrefix(r.Request.URL.Path, "/api/") {
				r.Response.WriteStatus(404)
				return
			}

			path := strings.TrimPrefix(r.Request.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			// Try to open file
			f, err := s.uiFS.Open(path)
			if err != nil {
				// If file not found, serve index.html (SPA fallback)
				f, err = s.uiFS.Open("index.html")
				if err != nil {
					r.Response.WriteStatus(404)
					r.Response.Write("404 Not Found")
					return
				}
			}
			defer f.Close()

			stat, _ := f.Stat()
			// http.ServeContent requires ReadSeeker
			if rs, ok := f.(io.ReadSeeker); ok {
				http.ServeContent(r.Response.Writer, r.Request, path, stat.ModTime(), rs)
			} else {
				// Should be ReadSeeker for embed.FS
				r.Response.WriteStatus(500)
			}
		})
	}

	defer store.CloseDB()

	// 1. Init Plugins
	if err := s.Manager.InitPlugins(); err != nil {
		return err
	}

	// 2. Start Plugins
	s.Manager.StartPlugins()

	// 2.1 Start DispatchService
	s.DispatchService.Start()

	// 2.5 Init and Start Device Manager
	if err := s.DeviceManager.Init(); err != nil {
		s.Logger.Error("Failed to init device manager", zap.Error(err))
		// Don't fail server, just log
	}
	if err := s.DeviceManager.StartAll(); err != nil {
		s.Logger.Error("Failed to start devices", zap.Error(err))
	}

	// 3. Start Web Server
	s.WebServer.Start() // Non-blocking start
	s.Logger.Info("Web Server started on port", zap.Int("port", s.Config.Server.Port))

	// 4. Wait for signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	s.Logger.Info("Shutting down server...")
	s.Manager.StopPlugins()
	if s.WebServer != nil {
		s.WebServer.Shutdown()
	}
	s.Logger.Sync()
	return nil
}

// --- API Handlers ---

func (s *Server) handleQueryHistory(r *ghttp.Request) {
	var req tsdb.QueryRequest
	if err := r.Parse(&req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid parameters"})
		return
	}

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	res, err := s.TSDB.Query(req)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": res,
	})
}

func (s *Server) handleListPlugins(r *ghttp.Request) {
	type PluginSummary struct {
		Name                    string              `json:"name"`
		Title                   map[string]string   `json:"title"`       // Display Name (i18n)
		Description             map[string]string   `json:"description"` // Description (i18n)
		Status                  string              `json:"status"`      // "running", "stopped"
		Category                string              `json:"category"`
		Icon                    string              `json:"icon"` // Base64 encoded icon
		Schema                  *PluginConfigSchema `json:"schema"`
		ProtocolMappingRequired *bool               `json:"protocolMappingRequired,omitempty"` // 协议映射是否需要
		IsPro                   bool                `json:"isPro"`
		IsUnauthorized          bool                `json:"isUnauthorized"`
	}

	summary := make([]PluginSummary, 0)

	for _, meta := range pluginMetas {
		if meta.Name == "license_auth" {
			continue // Hide license_auth plugin from the marketplace
		}

		isPro := meta.Name == "ai_predict" || meta.Name == "ai_copilot" || strings.EqualFold(meta.Name, "script") || meta.Name == "cascade" || meta.Name == "gb28181" || meta.Name == "webrtc"
		isAllowed := s.Manager.IsAllowed(meta)
		isUnauthorized := isPro && !isAllowed

		// Prepare icon string
		iconStr := ""
		if len(meta.Icon) > 0 {
			mimeType := "image/svg+xml"
			base64Icon := base64.StdEncoding.EncodeToString(meta.Icon)
			iconStr = "data:" + mimeType + ";base64," + base64Icon
		}

		var ps PluginSummary
		p := s.Manager.GetPlugin(meta.Name)

		if p != nil {
			schema := GetPluginConfigSchema(p)
			status := "stopped"
			if p.IsEnabled() {
				status = "running"
			}

			ps = PluginSummary{
				Name:           meta.Name,
				Title:          meta.Title,
				Description:    meta.Description,
				Status:         status,
				Category:       meta.Category,
				Icon:           iconStr,
				Schema:         schema,
				IsPro:          isPro,
				IsUnauthorized: isUnauthorized,
			}

			if pp, ok := p.(protocol.IProtocolPlugin); ok {
				v := pp.ProtocolMappingRequired()
				ps.ProtocolMappingRequired = &v
			}
		} else {
			// Plugin is not instantiated (could be unauthorized pro plugin)
			ps = PluginSummary{
				Name:           meta.Name,
				Title:          meta.Title,
				Description:    meta.Description,
				Status:         "stopped",
				Category:       meta.Category,
				Icon:           iconStr,
				Schema:         nil,
				IsPro:          isPro,
				IsUnauthorized: isUnauthorized,
			}
		}

		summary = append(summary, ps)
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": summary,
	})
}

func (s *Server) handleGetPlugin(r *ghttp.Request) {
	name := r.Get("name").String()
	p := s.Manager.GetPlugin(name)
	if p != nil {
		schema := GetPluginConfigSchema(p)
		r.Response.WriteJson(g.Map{
			"code": 0,
			"data": schema,
		})
		return
	}
	r.Response.WriteJson(g.Map{"code": 404, "message": "Plugin not found"})
}

func (s *Server) handleGetPluginSchemas(r *ghttp.Request) {
	name := r.Get("name").String()
	plugin := s.Manager.GetPlugin(name)
	if plugin == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Plugin not found"})
		return
	}

	// Check if it is a protocol plugin
	protocolPlugin, ok := plugin.(protocol.IProtocolPlugin)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Plugin is not a protocol plugin"})
		return
	}

	// Get Schemas
	parentCode := r.Get("parent_code").String()
	productCode := r.Get("product_code").String()
	deviceMeta := types.DeviceMeta{
		ParentCode:  parentCode,
		ProductCode: productCode,
	}

	productSchema, _ := protocolPlugin.GetProductConfigSchema()
	deviceSchema, _ := protocolPlugin.GetDeviceConfigSchema(deviceMeta)
	pointSchema, _ := protocolPlugin.GetPointConfigSchema()

	// We need to parse them to raw JSON object to embed in response,
	// otherwise they will be double-encoded strings if we just cast to string.
	// Or we can return them as RawMessage if we use encoding/json.
	// g.Map handles interface{}.

	var prodObj, devObj, pointObj interface{}
	if len(productSchema) > 0 {
		json.Unmarshal(productSchema, &prodObj)
	}
	if len(deviceSchema) > 0 {
		json.Unmarshal(deviceSchema, &devObj)
	}
	if len(pointSchema) > 0 {
		json.Unmarshal(pointSchema, &pointObj)
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"product":                     prodObj,
			"device":                      devObj,
			"point":                       pointObj,
			"subDeviceConfigCustomizable": protocolPlugin.SubDeviceConfigCustomizable(),
			"protocolMappingRequired":     protocolPlugin.ProtocolMappingRequired(),
		},
	})
}

func (s *Server) handleUpdatePluginConfig(r *ghttp.Request) {
	name := r.Get("name").String()
	var newConfig map[string]interface{}
	if err := json.Unmarshal(r.GetBody(), &newConfig); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	p := s.Manager.GetPlugin(name)
	if p == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Plugin not found"})
		return
	}

	// Update Config (Persist to file)
	if err := UpdatePluginConfig(p, newConfig); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Reload Plugin (Stop -> Create New Instance -> Load Config -> Init -> Start -> Replace)
	if err := s.Manager.ReloadPlugin(name); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to reload plugin: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Config updated and plugin reloaded"})

}

func (s *Server) handleSystemStats(r *ghttp.Request) {
	stats, err := system.GetStats()
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": stats})
}

func (s *Server) handlePluginDiscover(r *ghttp.Request) {
	name := r.Get("name").String()
	plugin := s.Manager.GetPlugin(name)
	if plugin == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Plugin not found"})
		return
	}

	protocolPlugin, ok := plugin.(protocol.IProtocolPlugin)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Plugin is not a protocol plugin"})
		return
	}

	// Parse Params
	var params map[string]interface{}
	// GetBody return []byte, unmarshal it
	body := r.GetBody()
	if len(body) > 0 {
		if err := json.Unmarshal(body, &params); err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
			return
		}
	} else {
		params = make(map[string]interface{})
	}

	devices, err := protocolPlugin.Discover(params)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": devices})
}
