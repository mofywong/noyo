package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"noyo/core/aiassets"
	"noyo/core/alarmmedia"
	"noyo/core/config"
	"noyo/core/protocol"
	"noyo/core/screenassets"
	"noyo/core/store"
	"noyo/core/system"
	"noyo/core/tsdb"
	"noyo/core/types"
	"noyo/core/workorder"

	// "runtime"

	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	// ort "github.com/yalue/onnxruntime_go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Server represents the gateway core server
type Server struct {
	Config            *config.GlobalConfig
	Logger            *zap.Logger
	Manager           *PluginManager
	DeviceManager     *DeviceManager
	DispatchService   *DispatchService
	RuleEngine        *RuleEngine
	WorkOrderService  *WorkOrderService
	WorkOrderCommands workorder.CommandPort
	AlarmInstances    workorder.AlarmInstanceService
	AlarmCenter       *AlarmCenterService
	Integrations      *workorder.GormIntegrationStore
	TSDB              *tsdb.TSDBManager
	MediaSessions     *MediaSessionManager
	// Location 统一收口设备位置写入（遥测与人工标注），Geofence 负责空间判定。
	Location        *LocationService
	Geofence        *GeofenceService
	WebServer       *ghttp.Server
	uiFS            fs.FS
	setupMu         sync.Mutex
	aiAssetsMu      sync.Mutex
	aiAssets        *aiassets.Service
	aiAssetsRoot    string
	aiModelConfigMu sync.RWMutex
}

func (s *Server) SetUI(uiFS fs.FS) {
	s.uiFS = uiFS
}

// wrapPluginHandler wraps a plugin handler with authentication and audit logging.
// This ensures all plugin-registered routes under /api/extension/* are authenticated.
func (s *Server) wrapPluginHandler(handler interface{}) func(*ghttp.Request) {
	originalHandler, ok := handler.(func(*ghttp.Request))
	if !ok {
		s.Logger.Error("Plugin registered handler with unsupported type")
		return func(r *ghttp.Request) {
			r.Response.WriteJson(map[string]interface{}{
				"code":    500,
				"message": "Internal server error: invalid handler type",
			})
		}
	}

	secret := s.Config.Auth.JWTSecret

	return func(r *ghttp.Request) {
		authCtx := authenticateRequest(r, secret)
		if authCtx == nil {
			return // auth failed, response already written
		}
		defer logAuditRecord(r)
		if !s.authorizePluginHTTP(r, authCtx) {
			return
		}
		originalHandler(r)
	}
}

// NewServer creates a new server instance
func NewServer() (*Server, error) {
	// 0. Init Database
	// Default to sqlite with file noyo.db
	if err := store.InitDB("./data/db/noyo.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"); err != nil {
		return nil, err
	}
	if err := cleanupSingleProjectRuntimePluginRows(); err != nil {
		return nil, err
	}

	// 1. Load Config from Database
	cfg, err := store.LoadGlobalConfig()
	newConfig := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !newConfig {
		return nil, fmt.Errorf("load global config: %w", err)
	}
	if newConfig {
		cfg = config.DefaultConfig()
	}
	cfg.AlarmMedia.Normalize()
	if err := alarmmedia.Configure(cfg.AlarmMedia); err != nil {
		return nil, fmt.Errorf("configure alarm recording storage: %w", err)
	}
	changed, err := cfg.NormalizeAuth()
	if err != nil {
		return nil, fmt.Errorf("normalize authentication config: %w", err)
	}
	if newConfig || changed {
		if err := store.SaveGlobalConfig(cfg); err != nil {
			return nil, fmt.Errorf("persist authentication config: %w", err)
		}
	}

	logger := InitLogger(cfg.Log)
	store.SetDBLogLevel(cfg.Log.Level)

	s := &Server{
		Config: cfg,
		Logger: logger,
	}
	s.Manager = NewPluginManager(s)
	s.DeviceManager = NewDeviceManager(s)
	s.DispatchService = NewDispatchService(s.Manager, s.DeviceManager.Registry, s.DeviceManager.EventBus, logger)
	s.TSDB = tsdb.NewManager(cfg.TSDB, logger)
	s.MediaSessions = NewMediaSessionManager()
	s.DeviceManager.TSDB = s.TSDB // Inject TSDB into DeviceManager
	s.WorkOrderService = NewWorkOrderService(store.DB)
	s.WorkOrderCommands = NewLocalCommandAdapter(s.WorkOrderService)
	s.AlarmInstances = workorder.NewGormAlarmInstanceStore(store.DB)
	s.AlarmCenter = NewAlarmCenterService(store.DB, s.AlarmInstances, s.WorkOrderService, s.WorkOrderCommands)
	s.Integrations = workorder.NewGormIntegrationStore(store.DB)
	s.RuleEngine = NewRuleEngine(s)
	s.Location = NewLocationService(s)
	s.Geofence = NewGeofenceService(s)
	go s.AlarmCenter.RunAutoCloseLoop()
	// 围栏迟滞必须由时间驱动：人工标注只写一次位置，没有后续上报来推进状态机。
	go s.Geofence.RunDwellLoop()

	SetDefaultServer(s)
	return s, nil
}

// Run starts the server and blocks until an interrupt signal is received
func (s *Server) Run() error {
	s.Logger.Info("Starting Gateway Server...")
	historyStopped := false
	stopHistory := func() {
		if historyStopped {
			return
		}
		historyStopped = true
		if err := s.TSDB.Stop(); err != nil {
			s.Logger.Error("History shutdown left pending records for recovery", zap.Error(err))
		}
	}

	// Start TSDB
	if err := s.TSDB.Init(); err != nil {
		s.Logger.Error("Failed to init TSDB", zap.Error(err))
	} else {
		if err := s.TSDB.Start(); err != nil {
			return fmt.Errorf("start history: %w", err)
		}
		defer stopHistory()
	}

	// Global ONNX Runtime Initialization
	// if !ort.IsInitialized() {
	// 	var initErr error
	// 	initialized := false
	// 	var dlls []string
	// 	switch runtime.GOOS {
	// 	case "windows":
	// 		dlls = []string{"lib/onnxruntime.dll", "lib/onnxruntime-cuda.dll", "lib/onnxruntime-gpu.dll", "lib/onnxruntime-cpu.dll"}
	// 	case "darwin":
	// 		dlls = []string{"lib/libonnxruntime.dylib"}
	// 	case "linux":
	// 		dlls = []string{"lib/libonnxruntime.so", "lib/libonnxruntime-cuda.so", "lib/libonnxruntime-gpu.so", "lib/libonnxruntime-cpu.so"}
	// 	default:
	// 		dlls = []string{"lib/libonnxruntime.so"}
	// 	}
	// 	for _, libPath := range dlls {
	// 		ort.SetSharedLibraryPath(libPath)
	// 		if err := ort.InitializeEnvironment(); err == nil {
	// 			initialized = true
	// 			s.Logger.Info("ONNX environment globally initialized", zap.String("library", libPath))
	// 			break
	// 		} else {
	// 			initErr = err
	// 			s.Logger.Debug("Failed to globally initialize ONNX environment", zap.String("library", libPath), zap.Error(err))
	// 		}
	// 	}
	// 	if !initialized {
	// 		s.Logger.Warn("Failed to globally initialize ONNX environment with any library", zap.Error(initErr))
	// 	}
	// }

	// Initialize Web Server (so plugins can register routes)
	s.WebServer = g.Server()
	configureAIModelUploads(s.WebServer)
	s.WebServer.SetRouteOverWrite(true) // Enable route overwrite for plugin reload
	s.WebServer.SetPort(s.Config.Server.Port)
	s.WebServer.SetDumpRouterMap(false) // Disable printing the route table on startup

	s.registerAPIRoutes()

	// Serve UI
	if s.uiFS != nil {
		screenEditor := screenassets.Handler(s.uiFS)
		s.WebServer.BindHandler("/screen-editor/*", func(r *ghttp.Request) {
			screenEditor.ServeHTTP(r.Response.Writer, r.Request)
		})
		// Serve static images from /data/images and attachments from /data/attachments
		s.WebServer.BindHandler("/data/images/*", func(r *ghttp.Request) {
			s.serveAuthorizedAsset(r, "./data/images", "/data/images/", false)
		})
		s.WebServer.BindHandler("/data/attachments/*", func(r *ghttp.Request) {
			s.serveAuthorizedAsset(r, "./data/attachments", "/data/attachments/", true)
		})

		s.WebServer.BindHandler("/*", func(r *ghttp.Request) {
			// Skip API
			if strings.HasPrefix(r.Request.URL.Path, "/api/") || strings.HasPrefix(r.Request.URL.Path, "/data/") || strings.HasPrefix(r.Request.URL.Path, "/screen-editor") {
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
	defer s.closeAIModelAssets()

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
	// Published screens read live values from memory, which is empty on a cold
	// start. Refill the devices they bind in the background; the web server must
	// not wait for it.
	go s.warmScreenSamples()
	if s.RuleEngine != nil {
		if err := s.RuleEngine.Start(); err != nil {
			s.Logger.Error("Failed to start rule engine", zap.Error(err))
		}
	}

	// 3. Start Web Server
	s.WebServer.Start() // Non-blocking start
	s.Logger.Info("Web Server started on port", zap.Int("port", s.Config.Server.Port))

	// 4. Wait for signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	s.Logger.Info("Shutting down server...")
	if s.WebServer != nil {
		s.WebServer.Shutdown()
	}
	if s.RuleEngine != nil {
		s.RuleEngine.Stop()
	}
	s.DeviceManager.StopAll()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelShutdown()
	if s.DeviceManager != nil && s.DeviceManager.EventBus != nil {
		if err := s.DeviceManager.EventBus.CloseWithContext(shutdownCtx); err != nil {
			s.Logger.Warn("Event bus shutdown did not drain", zap.Error(err))
		}
	}
	if s.DispatchService != nil {
		if err := s.DispatchService.CloseWithContext(shutdownCtx); err != nil {
			s.Logger.Warn("Dispatch shutdown did not drain", zap.Error(err))
		}
	}
	s.Manager.StopPlugins()
	stopHistory()
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
	owners, err := historyOwners(r, req.DeviceCode)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "History access denied / 无权访问历史数据"})
		return
	}
	req.Owners = owners
	req.Context = r.Context()

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

func (s *Server) handleUploadImage(r *ghttp.Request) {
	file := r.GetUploadFile("file")
	if file == nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "No file uploaded"})
		return
	}

	filename, err := saveUploadedImage(file, "./data/images")
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid image or image storage unavailable"})
		return
	}
	if err := recordUploadedAsset(r, "/data/images/"+filename); err != nil {
		_ = os.Remove("./data/images/" + filename)
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to record image ownership"})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"url": "/data/images/" + filename,
		},
	})
}

func (s *Server) handleListPlugins(r *ghttp.Request) {
	r.Response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	type PluginSummary struct {
		Name                    string              `json:"name"`
		Title                   map[string]string   `json:"title"`       // Display Name (i18n)
		Description             map[string]string   `json:"description"` // Description (i18n)
		Status                  string              `json:"status"`      // "running", "stopped"
		Category                string              `json:"category"`
		Icon                    string              `json:"icon"` // Base64 encoded icon
		Schema                  *PluginConfigSchema `json:"schema"`
		ProtocolMappingRequired *bool               `json:"protocolMappingRequired,omitempty"` // 协议映射是否需�?
		IsPro                   bool                `json:"isPro"`
		IsUnauthorized          bool                `json:"isUnauthorized"`
	}

	requestedType := r.GetQuery("type").String()
	enabledParam := strings.ToLower(r.GetQuery("enabled").String())
	enabledOnly := enabledParam == "1" || enabledParam == "true" || enabledParam == "yes"
	authCtx := requestAuthContext(r)

	summary := make([]PluginSummary, 0)

	for _, meta := range pluginMetas {
		if meta.Name == "license_auth" {
			continue // Hide license_auth plugin from the marketplace
		}
		if requestedType != "" && meta.Category != requestedType {
			continue
		}

		isPro := meta.Name == "ai_brain" || meta.Name == "ai_predict" || meta.Name == "ai_copilot" || strings.EqualFold(meta.Name, "script") || meta.Name == "gb28181" || meta.Name == "webrtc" || meta.Name == "voice_assistant" || meta.Name == "yolo_pro"
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
			status := pluginStatusForRequest(p, meta.Name, authCtx)

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
			status := pluginStatusForRequest(nil, meta.Name, authCtx)
			ps = PluginSummary{
				Name:           meta.Name,
				Title:          meta.Title,
				Description:    meta.Description,
				Status:         status,
				Category:       meta.Category,
				Icon:           iconStr,
				Schema:         nil,
				IsPro:          isPro,
				IsUnauthorized: isUnauthorized,
			}
		}

		if enabledOnly && ps.Status != "running" {
			continue
		}
		summary = append(summary, ps)
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": summary,
	})
}

func pluginStatusForRequest(plugin IManagedPlugin, name string, authCtx *AuthContext) string {
	if plugin == nil {
		return "stopped"
	}
	enabled := plugin.IsEnabled()
	if authCtx != nil && authCtx.TenantID > 0 && authCtx.ProjectID > 0 {
		enabled = false
		if scoped, err := store.GetPluginForScope(name, authCtx.TenantID, authCtx.ProjectID); err == nil && scoped != nil {
			enabled = scoped.Enabled
		}
	}
	if enabled {
		return "running"
	}
	return "stopped"
}

func (s *Server) handleGetPlugin(r *ghttp.Request) {
	name := r.Get("name").String()
	p := s.Manager.GetPlugin(name)
	if p != nil {
		schema := GetPluginConfigSchema(p)
		if resolver, ok := p.(IConfigRequestScopeResolver); ok {
			tenantID, projectID, err := resolver.ConfigRequestScope(r)
			if err != nil {
				r.Response.WriteStatus(403, g.Map{"code": 403, "message": err.Error()})
				return
			}
			if provider, ok := p.(IScopedConfigSchemaProvider); ok {
				schema, err = provider.GetConfigSchemaForScope(tenantID, projectID)
				if err != nil {
					r.Response.WriteJson(g.Map{"code": 500, "message": "Configuration unavailable"})
					return
				}
			}
		}
		r.Response.WriteJson(g.Map{
			"code": 0,
			"data": schema,
		})
		return
	}
	r.Response.WriteJson(g.Map{"code": 404, "message": "Plugin not found"})
}

func (s *Server) handleGetPluginSchemas(r *ghttp.Request) {
	name := NormalizeProtocolName(r.Get("name").String())
	if authCtx := requestAuthContext(r); authCtx != nil && authCtx.TenantID > 0 {
		tenantID, projectID, scopeErr := currentTenantProjectScope(r)
		if scopeErr != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": scopeErr.Error()})
			return
		}
		canonical, err := s.EnsureProtocolPluginEnabled(name, tenantID, projectID)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
			return
		}
		name = canonical
	}

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

	// Update Config (Persist to DB)
	var err error
	if resolver, ok := p.(IConfigRequestScopeResolver); ok {
		tenantID, projectID, scopeErr := resolver.ConfigRequestScope(r)
		if scopeErr != nil {
			r.Response.WriteStatus(403, g.Map{"code": 403, "message": scopeErr.Error()})
			return
		}
		err = UpdatePluginConfigForScope(p, newConfig, tenantID, projectID)
	} else if authCtx := requestAuthContext(r); authCtx != nil && authCtx.TenantID > 0 {
		tenantID, projectID, scopeErr := currentTenantProjectScope(r)
		if scopeErr != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": scopeErr.Error()})
			return
		}
		err = UpdatePluginConfigForScope(p, newConfig, tenantID, projectID)
	} else {
		err = UpdatePluginConfig(p, newConfig)
	}
	if err != nil {
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

func (s *Server) handleDeleteRecord(r *ghttp.Request) {
	var req struct {
		ID int64 `json:"id"`
		Ts int64 `json:"ts"`
	}
	if err := r.Parse(&req); err != nil || req.ID <= 0 || req.Ts <= 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid parameters: id and ts are required"})
		return
	}

	record, err := s.TSDB.GetRecord(r.Context(), req.ID, req.Ts)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "History record unavailable / 历史记录不可用"})
		return
	}
	if !canDeleteHistoryRecord(r, record) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "History access denied / 无权访问历史数据"})
		return
	}
	owner := tsdb.RecordOwner{DeviceID: record.DeviceID, TenantID: record.TenantID, ProjectID: record.ProjectID}
	if err := s.TSDB.DeleteOwnedRecord(r.Context(), req.ID, req.Ts, owner); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Record deleted"})
}

// Injecting debug route
func init() {
	// We cannot easily inject a route without modifying server.go setup
}
