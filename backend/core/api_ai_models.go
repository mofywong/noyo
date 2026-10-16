package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"noyo/core/aiassets"
	"noyo/core/store"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

const maxModelUploadBytes = int64(2 << 30)

// GoFrame applies its body limit before route handlers. Permit model archives
// there, and retain the framework's 8 MiB default for every other request.
func configureAIModelUploads(web *ghttp.Server) {
	handler := web.GetHandler()
	web.SetClientMaxBodySize(maxModelUploadBytes)
	web.SetHandler(func(w http.ResponseWriter, r *http.Request) {
		limit := int64(8 << 20)
		modelUpload := r.Method == http.MethodPost && strings.TrimSuffix(r.URL.Path, "/") == "/api/ai/models/import"
		if modelUpload {
			limit = maxModelUploadBytes
		} else if r.ContentLength > limit {
			http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, limit)
		handler(w, r)
	})
}

// AIModelAssets is independent of plugin startup, so an empty installation can
// be repaired through the core API before inference is enabled.
func (s *Server) AIModelAssets() (*aiassets.Service, error) {
	s.aiAssetsMu.Lock()
	defer s.aiAssetsMu.Unlock()
	root, err := configuredAIModelRoot()
	if err != nil {
		return nil, err
	}
	if s.aiAssets != nil {
		if s.aiAssetsRoot != root {
			return nil, &aiassets.Error{Code: "model_root_changed", Message: "Restart after changing the model directory"}
		}
		return s.aiAssets, nil
	}
	options := aiassets.Options{Providers: modelAssetProviders()}
	if store.DB != nil {
		options.RecordHistory = func(event aiassets.HistoryEvent) error {
			return persistModelHistory(root, event)
		}
	}
	service, err := aiassets.NewService(root, options)
	if err != nil {
		return nil, err
	}
	s.aiAssets, s.aiAssetsRoot = service, root
	return service, nil
}

func persistModelHistory(root string, event aiassets.HistoryEvent) error {
	return store.CreateModelHistory(&store.ModelHistory{
		EventID: event.ID, ModelRoot: root, PackageID: event.PackageID, Version: event.Version,
		PreviousVersion: event.PreviousVersion, Action: event.Action,
		OperatorUserID: event.Actor.UserID, OperatorUsername: event.Actor.Username,
		OperatorName: event.Actor.Name, TenantID: event.Actor.TenantID, IP: event.Actor.IP,
		CreatedAt: event.CreatedAt,
	})
}

func configuredAIModelRoot() (string, error) {
	root := strings.TrimSpace(os.Getenv("NOYO_MODEL_DIR"))
	if root == "" {
		root = "./models"
	}
	return filepath.Abs(filepath.Clean(root))
}

func (s *Server) closeAIModelAssets() {
	s.aiAssetsMu.Lock()
	defer s.aiAssetsMu.Unlock()
	if s.aiAssets != nil {
		_ = s.aiAssets.Close()
		s.aiAssets = nil
	}
}

func modelAssetProviders() []aiassets.Provider {
	providers := aiassets.Providers()
	hasYOLO := false
	for i := range providers {
		p := &providers[i]
		if p.ID == "yolo_pro" {
			hasYOLO = true
		}
		required, existing := p.RequiredKinds, p.References
		p.References = func(root, kind string) []string {
			var references []string
			if existing != nil {
				references = append(references, existing(root, kind)...)
			}
			if required == nil || store.DB == nil {
				return references
			}
			// Include offline devices and disabled plugins: saved enabled AI rules
			// still protect their dependencies from uninstall.
			var devices []store.Device
			if err := store.DB.Select("code", "config").Find(&devices).Error; err != nil {
				return append(references, "configuration_database_unavailable")
			}
			for _, device := range devices {
				if slices.Contains(required(device.Config), kind) {
					references = append(references, device.Code)
				}
			}
			slices.Sort(references)
			return slices.Compact(references)
		}
	}
	if !hasYOLO {
		providers = append(providers, unsupportedYOLOProvider())
	}
	return providers
}

func unsupportedYOLOProvider() aiassets.Provider {
	p := aiassets.Provider{ID: "yolo_pro"}
	p.Capabilities = aiassets.YOLORecognitionCapabilities(nil, "algorithm_not_built")
	p.RequiredKinds = func(configJSON string) []string {
		var config struct {
			EnableAI bool `json:"enable_ai"`
		}
		_ = json.Unmarshal([]byte(configJSON), &config)
		if config.EnableAI {
			return []string{"general"}
		}
		return nil
	}
	return p
}

// Existing enabled configurations remain editable when resources are missing.
// Only newly required dependencies (or provider-specific contract changes) gate a save.
func (s *Server) validateAIModelConfigChange(before, after string) error {
	if before == after {
		return nil
	}
	for _, provider := range modelAssetProviders() {
		if provider.RequiredKinds == nil {
			continue
		}
		previous, next := provider.RequiredKinds(before), provider.RequiredKinds(after)
		if len(next) == 0 {
			continue
		}
		var added []string
		for _, kind := range next {
			if !slices.Contains(previous, kind) {
				added = append(added, kind)
			}
		}
		if len(added) == 0 && provider.ValidateConfig == nil {
			continue
		}
		service, err := s.AIModelAssets()
		if err != nil {
			return err
		}
		for _, kind := range added {
			if _, ready := service.Resolve(kind); ready {
				continue
			}
			if provider.Legacy != nil && provider.Legacy(service.Root(), kind) {
				continue
			}
			code := "model_not_ready"
			for _, capability := range provider.Capabilities {
				if capability.UnsupportedReason != "" {
					code = "algorithm_not_built"
					break
				}
			}
			return &aiassets.Error{Code: code, Message: fmt.Sprintf("Required model is unavailable: %s", kind)}
		}
		if provider.ValidateConfig != nil {
			if err := provider.ValidateConfig(service.Root(), before, after); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) registerAIModelRoutes(protected *ghttp.RouterGroup) {
	models := protected.Group("/ai/models")
	permissionGET(models, "/", "ai_model:list", s.handleModelCatalog)
	permissionGET(models, "/catalog", "ai_model:list", s.handleModelCatalog)
	permissionGET(models, "/export", "ai_model:export", s.handleModelExport)
	permissionGET(models, "/history", "ai_model:list", s.handleModelHistory)
	permissionGET(models, "/jobs", "ai_model:list", s.handleModelJobs)
	permissionGET(models, "/jobs/:id", "ai_model:list", s.handleModelJob)
	admin := models.Group("/")
	admin.Middleware(func(r *ghttp.Request) {
		auth := RequestAuthContext(r)
		if auth == nil || !auth.IsSystemAdmin {
			s.writeAIModelError(r, &aiassets.Error{Code: "admin_required", Message: "System administrator required"})
			return
		}
		r.Middleware.Next()
	})
	admin.Middleware(PermissionMiddleware("ai_model:manage"))
	admin.POST("/plan", s.handleModelPlan)
	admin.GET("/delivery-catalog", s.handleModelDeliveryCatalog)
	admin.POST("/jobs", s.handleModelInstall)
	admin.POST("/jobs/:id/cancel", s.handleModelCancel)
	admin.POST("/jobs/:id/retry", s.handleModelRetry)
	admin.POST("/import", s.handleModelImport)
	admin.DELETE("/imports/:id", s.handleModelDiscardImport)
	admin.POST("/activate", s.handleModelActivate)
	admin.POST("/packages/:id/:version/verify", s.handleModelVerify)
	admin.DELETE("/packages/:id/:version", s.handleModelDelete)
}

func (s *Server) handleModelDeliveryCatalog(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		r.Response.WriteJson(g.Map{"code": 0, "data": service.DeliveryCatalog()})
	})
}

func (s *Server) handleModelExport(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		r.Response.Header().Set("Cache-Control", "no-store")
		r.Response.WriteJson(g.Map{"code": 0, "data": service.Inventory()})
	})
}

func (s *Server) withModelAssets(r *ghttp.Request, action func(*aiassets.Service)) {
	service, err := s.AIModelAssets()
	if err != nil {
		s.writeAIModelError(r, err)
		return
	}
	action(service)
}

func modelJSON(r *ghttp.Request, target interface{}) error {
	r.Request.Body = http.MaxBytesReader(nil, r.Request.Body, 1<<20)
	decoder := json.NewDecoder(r.Request.Body)
	if err := decoder.Decode(target); err != nil {
		return &aiassets.Error{Code: "invalid_request", Message: "Invalid JSON request"}
	}
	if err := decoder.Decode(new(interface{})); err != io.EOF {
		return &aiassets.Error{Code: "invalid_request", Message: "Request must contain a single JSON object"}
	}
	return nil
}

func (s *Server) handleModelCatalog(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		catalog := service.Catalog()
		auth := RequestAuthContext(r)
		canManage := auth != nil && auth.IsSystemAdmin
		root := ""
		if canManage {
			root = service.Root()
		} else {
			for i := range catalog.Packages {
				catalog.Packages[i].References = nil
			}
		}
		r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"capabilities": catalog.Capabilities, "packages": catalog.Packages,
			"jobs": publicModelJobs(catalog.Jobs), "generation": catalog.Generation, "can_manage": canManage, "model_root": root}})
	})
}

func publicModelJob(job aiassets.Job) aiassets.Job {
	// File-system and network errors belong in server logs. The UI translates
	// the structured error code without exposing paths or source credentials.
	job.Error, job.IdempotencyKey = "", ""
	job.Actor = aiassets.HistoryActor{}
	return job
}
func publicModelJobs(jobs []aiassets.Job) []aiassets.Job {
	for i := range jobs {
		jobs[i] = publicModelJob(jobs[i])
	}
	return jobs
}
func (s *Server) handleModelPlan(r *ghttp.Request) {
	var req struct {
		aiassets.PlanRequest
		CapabilityIDs []string `json:"capability_ids"`
	}
	if err := modelJSON(r, &req); err != nil {
		s.writeAIModelError(r, err)
		return
	}
	if len(req.Capabilities) == 0 {
		req.Capabilities = req.CapabilityIDs
	}
	s.withModelAssets(r, func(service *aiassets.Service) {
		plan, err := service.Plan(req.PlanRequest)
		if err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0, "data": plan})
	})
}
func (s *Server) handleModelInstall(r *ghttp.Request) {
	var req struct {
		PlanID     string `json:"planId"`
		Key        string `json:"idempotencyKey"`
		LegacyPlan string `json:"plan_id"`
		LegacyKey  string `json:"idempotency_key"`
	}
	if err := modelJSON(r, &req); err != nil {
		s.writeAIModelError(r, err)
		return
	}
	if req.PlanID == "" {
		req.PlanID = req.LegacyPlan
	}
	if req.Key == "" {
		req.Key = req.LegacyKey
	}
	if req.Key == "" {
		req.Key = r.Request.Header.Get("Idempotency-Key")
	}
	s.withModelAssets(r, func(service *aiassets.Service) {
		job, err := service.Install(req.PlanID, req.Key, modelHistoryActor(r))
		if err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteHeader(http.StatusAccepted)
		r.Response.WriteJson(g.Map{"code": 0, "data": publicModelJob(job)})
	})
}
func (s *Server) handleModelJobs(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		r.Response.WriteJson(g.Map{"code": 0, "data": publicModelJobs(service.Jobs())})
	})
}
func (s *Server) handleModelJob(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		job, err := service.Job(r.Get("id").String())
		if err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0, "data": publicModelJob(job)})
	})
}
func (s *Server) handleModelCancel(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		job, err := service.Cancel(r.Get("id").String())
		if err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0, "data": publicModelJob(job)})
	})
}
func (s *Server) handleModelRetry(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		job, err := service.Retry(r.Get("id").String(), modelHistoryActor(r))
		if err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteHeader(http.StatusAccepted)
		r.Response.WriteJson(g.Map{"code": 0, "data": publicModelJob(job)})
	})
}
func (s *Server) handleModelDiscardImport(r *ghttp.Request) {
	s.withModelAssets(r, func(service *aiassets.Service) {
		if err := service.DiscardImport(r.Get("id").String()); err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0})
	})
}

func (s *Server) handleModelImport(r *ghttp.Request) {
	if r.Request.ContentLength > maxModelUploadBytes {
		s.writeAIModelError(r, &aiassets.Error{Code: "archive_too_large"})
		return
	}
	r.Request.Body = http.MaxBytesReader(nil, r.Request.Body, maxModelUploadBytes)
	if err := r.Request.ParseMultipartForm(16 << 20); err != nil {
		code := "invalid_archive"
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			code = "archive_too_large"
		}
		s.writeAIModelError(r, &aiassets.Error{Code: code})
		return
	}
	if r.Request.MultipartForm != nil {
		defer r.Request.MultipartForm.RemoveAll()
	}
	file, _, err := r.Request.FormFile("file")
	if err != nil {
		s.writeAIModelError(r, &aiassets.Error{Code: "invalid_archive"})
		return
	}
	defer file.Close()
	s.withModelAssets(r, func(service *aiassets.Service) {
		var plan aiassets.Plan
		var err error
		if r.GetQuery("single").Bool() {
			plan, err = service.ImportModel(r.Context(), file)
		} else {
			plan, err = service.Import(r.Context(), file)
		}
		if err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0, "data": plan})
	})
}
func (s *Server) handleModelActivate(r *ghttp.Request) {
	s.aiModelConfigMu.Lock()
	defer s.aiModelConfigMu.Unlock()
	var req aiassets.ActivateRequest
	if err := modelJSON(r, &req); err != nil {
		s.writeAIModelError(r, err)
		return
	}
	s.withModelAssets(r, func(service *aiassets.Service) {
		if err := service.Activate(req, modelHistoryActor(r)); err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0})
	})
}
func (s *Server) handleModelVerify(r *ghttp.Request) {
	s.aiModelConfigMu.Lock()
	defer s.aiModelConfigMu.Unlock()
	s.withModelAssets(r, func(service *aiassets.Service) {
		if err := service.Verify(r.Get("id").String(), r.Get("version").String()); err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0})
	})
}
func (s *Server) handleModelDelete(r *ghttp.Request) {
	s.aiModelConfigMu.Lock()
	defer s.aiModelConfigMu.Unlock()
	s.withModelAssets(r, func(service *aiassets.Service) {
		if err := service.Delete(r.Get("id").String(), r.Get("version").String(), modelHistoryActor(r)); err != nil {
			s.writeAIModelError(r, err)
			return
		}
		r.Response.WriteJson(g.Map{"code": 0})
	})
}

func modelHistoryActor(r *ghttp.Request) aiassets.HistoryActor {
	auth := RequestAuthContext(r)
	if auth == nil {
		return aiassets.HistoryActor{Name: "system"}
	}
	username := auth.Username
	name := username
	if auth.UserID != 0 && store.DB != nil {
		if user, err := store.GetUserByID(auth.UserID); err == nil {
			username = user.Username
			name = strings.TrimSpace(user.DisplayName)
			if name == "" {
				name = username
			}
		}
	}
	if name == "" && auth.UserID != 0 {
		name = fmt.Sprintf("%d", auth.UserID)
	}
	if name == "" {
		name = auth.AppID
	}
	return aiassets.HistoryActor{UserID: auth.UserID, TenantID: auth.TenantID, Username: username, Name: name, IP: r.GetClientIp()}
}

func (s *Server) handleModelHistory(r *ghttp.Request) {
	packageID := strings.TrimSpace(r.GetQuery("packageId").String())
	if packageID == "" || len(packageID) > 128 {
		s.writeAIModelError(r, &aiassets.Error{Code: "invalid_request", Message: "Select a model"})
		return
	}
	s.withModelAssets(r, func(service *aiassets.Service) {
		if err := service.FlushHistory(); err != nil {
			s.writeAIModelError(r, err)
			return
		}
		page, err := store.ListModelHistory(service.Root(), packageID, r.GetQuery("page").Int(), r.GetQuery("pageSize").Int())
		if err != nil {
			s.writeAIModelError(r, &aiassets.Error{Code: "history_unavailable", Message: "Cannot read model operation history"})
			return
		}
		items := make([]g.Map, 0, len(page.Items))
		for _, item := range page.Items {
			operator := item.OperatorName
			if item.OperatorUserID != 0 {
				operator = fmt.Sprintf("%s (%d)", operator, item.OperatorUserID)
			}
			items = append(items, g.Map{"id": item.ID, "action": item.Action, "version": item.Version,
				"previousVersion": item.PreviousVersion, "operatorName": operator, "createdAt": item.CreatedAt})
		}
		r.Response.Header().Set("Cache-Control", "no-store")
		r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"items": items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize}})
	})
}

func (s *Server) writeAIModelError(r *ghttp.Request, err error) {
	code, status := "model_operation_failed", http.StatusInternalServerError
	var modelErr *aiassets.Error
	if errors.As(err, &modelErr) {
		code = modelErr.Code
	}
	switch code {
	case "admin_required":
		status = http.StatusForbidden
	case "not_found", "job_not_found", "plan_not_found", "package_not_found", "version_not_found":
		status = http.StatusNotFound
	case "model_in_use", "in_use", "model_referenced", "referenced", "dependency_conflict", "busy", "store_locked", "model_root_changed", "model_not_ready", "algorithm_not_built", "conflict", "unsupported", "missing_dependency", "missing_dependencies", "plan_stale", "runtime_missing":
		status = http.StatusConflict
	case "archive_too_large", "package_too_large":
		status = http.StatusRequestEntityTooLarge
	case "disk_insufficient":
		status = http.StatusInsufficientStorage
	case "storage_error":
		status = http.StatusInternalServerError
	default:
		if modelErr != nil {
			status = http.StatusBadRequest
		}
	}
	message := "模型操作失败，请检查资源及任务状态 / Model operation failed; check resources and job status"
	switch code {
	case "admin_required":
		message = "仅系统管理员可管理模型 / Only system administrators can manage models"
	case "model_not_ready":
		message = "所需模型尚未就绪，请先到模型管理安装 / Required models are not ready; install them in Model Management"
	case "algorithm_not_built", "unsupported_build", "unsupported":
		message = "当前程序未包含此算法，请升级程序版本 / This build does not include the algorithm; upgrade the program"
	case "model_root_changed":
		message = "模型目录已变更，请重启服务 / The model directory changed; restart the service"
	case "model_in_use", "in_use":
		message = "模型正在使用，请先停止相关分析任务 / Model is in use; stop related analysis tasks first"
	case "model_referenced", "referenced":
		message = "已有启用规则引用该模型，请先调整规则 / Enabled rules reference this model; update the rules first"
	case "archive_too_large", "package_too_large":
		message = "资源包超过大小限制 / Resource package exceeds the size limit"
	case "disk_insufficient":
		message = "磁盘空间不足，请释放空间后重试 / Insufficient disk space; free space and retry"
	case "invalid_request":
		message = "请求格式无效 / Invalid request format"
	case "manifest_root_missing":
		message = "ZIP 根目录缺少 manifest.json，请移除外层文件夹后重新打包 / manifest.json is missing at the ZIP root; remove the wrapper folder and repack"
	case "invalid_archive", "archive_invalid":
		message = "离线模型包无效 / Invalid offline model archive"
	case "integrity":
		message = "模型文件校验失败，请检查安装任务或重新安装 / Model integrity check failed; check the job or reinstall"
	case "missing_dependency", "missing_dependencies":
		message = "所需模型依赖尚未就绪，请安装完整依赖集合 / Required model dependencies are not ready; install the complete dependency set"
	case "plan_stale":
		message = "活动模型已变更，请刷新后重试 / Active models changed; refresh and retry"
	case "runtime_missing":
		message = "推理运行库不可用，请修复部署环境 / Inference runtime is unavailable; repair the deployment environment"
	case "store_locked":
		message = "模型目录由其他进程管理 / Another process manages the model directory"
	}
	if s.Logger != nil {
		s.Logger.Warn("Model asset operation rejected", zap.String("reason", code), zap.Error(err))
	}
	r.Response.WriteHeader(status)
	r.Response.WriteJson(g.Map{"code": status, "reason_code": code, "message": message})
}
