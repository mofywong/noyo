package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"net/http"
	"noyo/core/dataquery"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
	"noyo/core/screen"
	"noyo/core/store"
)

type screenAPIError struct {
	status  int
	message string
}

// Screen images use the existing validated image-storage functions, with a
// dedicated directory so the generic /data/images owner route cannot bypass
// the screen's current revision visibility after permissions change.
const screenAssetDirectory = "./data/screen-assets"

var screenAuditRequestID = regexp.MustCompile(`^[A-Za-z0-9_-]{8,128}$`)

// screenAuditSummary accepts metadata only: neither request bodies nor arbitrary
// URLs/query strings can enter an audit record through this function.
func screenAuditSummary(method, path string, screenID uint, revision, requestID string, status int) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	action := "screen_request"
	if len(parts) == 2 && method == http.MethodPost {
		action = "create"
	}
	if len(parts) >= 3 {
		if len(parts) == 3 && method == http.MethodDelete {
			action = "delete"
		}
		if len(parts) == 4 {
			switch parts[3] {
			case "draft":
				if method == http.MethodPut {
					action = "save_draft"
				}
			case "metadata":
				if method == http.MethodPut {
					action = "rename"
				}
			case "publish", "unpublish", "copy", "validate", "apply":
				if method == http.MethodPost {
					action = parts[3]
				}
			case "assets":
				if method == http.MethodPost {
					action = "upload_asset"
				}
			}
		}
		if len(parts) == 5 && parts[3] == "runtime" && method == http.MethodPost {
			if parts[4] == "snapshot" || parts[4] == "history" {
				action = "runtime_" + parts[4]
			}
		}
		if len(parts) == 6 && parts[3] == "releases" && parts[5] == "restore" && method == http.MethodPost {
			action = "restore"
		}
	}
	if len(revision) > 32 {
		revision = ""
	} else if revision != "published" && revision != "unpublished" {
		candidate := strings.TrimPrefix(revision, "draft-")
		if n, err := strconv.ParseUint(candidate, 10, 64); err != nil || n == 0 {
			revision = ""
		}
	}
	if !screenAuditRequestID.MatchString(requestID) {
		requestID = ""
	}
	if status < 100 {
		status = http.StatusOK
	}
	data, _ := json.Marshal(struct {
		Action    string `json:"action"`
		ScreenID  uint   `json:"screenId"`
		Revision  string `json:"revision,omitempty"`
		RequestID string `json:"requestId,omitempty"`
		Result    int    `json:"result"`
	}{action, screenID, revision, requestID, status})
	return string(data)
}

func screenAuditDetail(r *ghttp.Request) string {
	id := r.GetCtxVar("screen_audit_screen_id").Uint()
	if id == 0 {
		id = r.GetRouter("id").Uint()
	}
	revision := r.GetCtxVar("screen_audit_revision").String()
	if revision == "" {
		revision = r.GetQuery("revision").String()
	}
	requestID := r.GetCtxVar("screen_audit_request_id").String()
	if requestID == "" {
		requestID = r.Header.Get("X-Request-ID")
	}
	return screenAuditSummary(r.Method, r.URL.Path, id, revision, requestID, r.Response.Status)
}

func (e screenAPIError) Error() string { return e.message }
func screenFail(status int, message string) error {
	return screenAPIError{status: status, message: message}
}
func screenWriteError(r *ghttp.Request, err error) {
	status := http.StatusBadRequest
	var apiErr screenAPIError
	if errors.As(err, &apiErr) {
		status = apiErr.status
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		status = http.StatusNotFound
		err = errors.New("Screen or resource not found / 大屏或资源不存在")
	} else if errors.Is(err, store.ErrScreenConflict) {
		status = http.StatusPreconditionFailed
	} else if errors.Is(err, store.ErrScreenRequestConflict) {
		status = http.StatusConflict
	}
	r.Response.Header().Set("Cache-Control", "private, no-store")
	r.Response.WriteHeader(status)
	r.Response.WriteJson(map[string]interface{}{"code": status, "message": err.Error()})
}
func screenWriteData(r *ghttp.Request, data interface{}) {
	r.Response.Header().Set("Cache-Control", "private, no-store")
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": data})
}

func (s *Server) RegisterScreenRoutes(group *ghttp.RouterGroup) {
	g := group.Group("/screens")
	g.GET("/", s.handleScreenList)
	g.POST("/", s.handleScreenCreate)
	g.GET("/:id", s.handleScreenGet)
	g.PUT("/:id/metadata", s.handleScreenMetadata)
	g.GET("/:id/draft", s.handleScreenDraft)
	g.PUT("/:id/draft", s.handleScreenSaveDraft)
	g.POST("/:id/apply", s.handleScreenApply)
	g.GET("/:id/apply/:requestId", s.handleScreenApplyStatus)
	g.POST("/:id/validate", s.handleScreenValidate)
	g.POST("/:id/publish", s.handleScreenPublish)
	g.POST("/:id/unpublish", s.handleScreenUnpublish)
	g.GET("/:id/published", s.handleScreenPublished)
	g.GET("/:id/releases", s.handleScreenReleases)
	g.GET("/:id/releases/:releaseId", s.handleScreenRelease)
	g.POST("/:id/releases/:releaseId/restore", s.handleScreenRestore)
	g.POST("/:id/copy", s.handleScreenCopy)
	g.DELETE("/:id", s.handleScreenDelete)
	g.GET("/:id/assets", s.handleScreenAssets)
	g.POST("/:id/assets", s.handleScreenUploadAsset)
	g.GET("/:id/assets/:assetId", s.handleScreenAsset)
	g.POST("/:id/runtime/snapshot", s.handleScreenRuntimeSnapshot)
	g.POST("/:id/runtime/history", s.handleScreenRuntimeHistory)
	g.POST("/:id/runtime/alarms", s.handleScreenRuntimeAlarms)
	g.POST("/:id/runtime/data", s.handleScreenRuntimeData)
	g.POST("/:id/data-preview", s.handleScreenDataPreview)
	g.POST("/:id/binding-preview", s.handleScreenBindingPreview)
	g.GET("/:id/datasets", s.handleScreenDatasets)
	g.POST("/:id/datasets", s.handleScreenDatasetSave)
	g.GET("/:id/datasets/:datasetId/versions/:version", s.handleScreenDatasetVersion)
	g.GET("/:id/runtime/stream", s.handleScreenRuntimeStream)
	g.GET("/:id/binding-catalog", s.handleScreenBindingCatalog)
	g.POST("/:id/assistant/plan", s.handleScreenAssistantPlan)
	g.GET("/:id/assistant/tasks", s.handleScreenAssistantTaskCurrent)
	g.GET("/:id/assistant/tasks/:taskId", s.handleScreenAssistantTaskGet)
	g.POST("/:id/assistant/tasks/:taskId/resume", s.handleScreenAssistantTaskResume)
	g.POST("/:id/assistant/tasks/:taskId/cancel", s.handleScreenAssistantTaskCancel)
	g.GET("/:id/assistant/conversations", s.handleScreenAssistantConversations)
	g.POST("/:id/assistant/conversations", s.handleScreenAssistantConversationCreate)
	g.GET("/:id/assistant/messages", s.handleScreenAssistantHistory)
	g.POST("/:id/assistant/messages", s.handleScreenAssistantMessage)
}

// screenCurrentScope resolves one selected project. Super administrators have
// every screen permission, but still read/write only that project's resources.
func screenCurrentScope(r *ghttp.Request) (uint, uint, error) {
	auth := requestAuthContext(r)
	if auth == nil {
		return 0, 0, screenFail(401, "Authentication required / 请先登录")
	}
	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID == 0 {
		tenantID = auth.TenantID
	}
	projectID := r.GetCtxVar("project_id").Uint()
	if projectID == 0 {
		projectID = auth.ProjectID
	}
	if auth.IsSystemAdmin && auth.TenantID == 0 {
		// Global account resolution intentionally discards tenant headers. Use
		// the selector only as a consistency check against the selected project.
		if selected := r.Header.Get("X-Current-Tenant-ID"); selected != "" {
			id, err := strconv.ParseUint(selected, 10, 64)
			if err != nil {
				return 0, 0, screenFail(403, "Invalid tenant selection / 租户选择无效")
			}
			tenantID = uint(id)
		}
	}
	tenantID, projectID, err := screenResolveScope(auth, tenantID, projectID)
	if err != nil {
		return 0, 0, err
	}
	// Audit records must use the selected project's tenant, including for a
	// global administrator. Keep the authenticated account identity unchanged.
	r.SetCtxVar("tenant_id", tenantID)
	r.SetCtxVar("project_id", projectID)
	return tenantID, projectID, nil
}

func screenResolveScope(auth *AuthContext, tenantID, projectID uint) (uint, uint, error) {
	if auth == nil {
		return 0, 0, screenFail(401, "Authentication required / 请先登录")
	}
	if projectID == 0 {
		return 0, 0, screenFail(403, "Select an authorized tenant and project / 请选择已授权租户和项目")
	}
	if auth.IsSystemAdmin {
		var project store.Project
		if err := store.DB.Select("id", "tenant_id").First(&project, projectID).Error; err != nil || project.TenantID == 0 || (tenantID != 0 && tenantID != project.TenantID) {
			return 0, 0, screenFail(403, "Project is outside selected tenant / 项目不属于所选租户")
		}
		return project.TenantID, projectID, nil
	}
	if tenantID == 0 || tenantID != auth.TenantID {
		return 0, 0, screenFail(403, "Select an authorized tenant and project / 请选择已授权租户和项目")
	}
	if !projectBelongsToTenant(projectID, tenantID) || (!auth.IsTenantAdmin && !auth.CanAccessProject(projectID)) {
		return 0, 0, screenFail(403, "Project is outside allowed scope / 无权访问此项目")
	}
	return tenantID, projectID, nil
}

func screenHasPermission(auth *AuthContext, projectID uint, permission string) bool {
	if auth == nil {
		return false
	}
	if permission == "screen:view" {
		return auth.HasProjectPermission("screen:view", projectID) || auth.HasProjectPermission("screen:edit", projectID)
	}
	if permission == "screen:publish" {
		return auth.HasProjectPermission(permission, projectID) && auth.HasProjectPermission("screen:edit", projectID)
	}
	if permission == "screen:create" {
		return auth.HasProjectPermission(permission, projectID) && auth.HasProjectPermission("screen:edit", projectID)
	}
	return auth.HasProjectPermission(permission, projectID)
}

func screenRequire(r *ghttp.Request, permission string) (uint, uint, error) {
	tenantID, projectID, err := screenCurrentScope(r)
	if err != nil {
		return 0, 0, err
	}
	if !screenHasPermission(requestAuthContext(r), projectID, permission) {
		return 0, 0, screenFail(403, "Screen permission denied / 无大屏操作权限: "+permission)
	}
	r.SetCtxVar("required_permission", permission)
	return tenantID, projectID, nil
}
func screenLoad(r *ghttp.Request, permission string) (*store.IOCScreen, error) {
	tenantID, projectID, err := screenRequire(r, permission)
	if err != nil {
		return nil, err
	}
	row, err := store.ScreenFind(store.DB, tenantID, projectID, r.GetRouter("id").Uint())
	if err == nil {
		r.SetCtxVar("screen_audit_screen_id", row.ID)
		r.SetCtxVar("screen_audit_revision", fmt.Sprintf("draft-%d", row.DraftVersion))
	}
	return row, err
}

// screenAuthorizedDocument is used by every runtime and asset read. A viewer
// may read only the active release; editors must identify an exact draft.
func screenAuthorizedDocument(r *ghttp.Request) (*store.IOCScreen, screen.Envelope, string, error) {
	var doc screen.Envelope
	row, err := screenLoad(r, "screen:view")
	if err != nil {
		return nil, doc, "", err
	}
	revision := r.GetQuery("revision").String()
	if revision == "" {
		revision = "published"
	}
	content := ""
	if revision == "draft" {
		if !screenHasPermission(requestAuthContext(r), row.ProjectID, "screen:edit") {
			return nil, doc, "", screenFail(403, "Draft requires edit permission / 草稿需要编辑权限")
		}
		if version := r.GetQuery("draftVersion").Uint64(); version == 0 || version != row.DraftVersion {
			return nil, doc, "", store.ErrScreenConflict
		}
		content = row.DraftJSON
		revision = fmt.Sprintf("draft-%d", row.DraftVersion)
	} else {
		var releaseID uint
		if revision == "published" {
			if row.PublishedReleaseID == nil {
				return nil, doc, "", gorm.ErrRecordNotFound
			}
			releaseID = *row.PublishedReleaseID
		} else {
			n, e := strconv.ParseUint(revision, 10, 64)
			if e != nil || n == 0 {
				return nil, doc, "", gorm.ErrRecordNotFound
			}
			releaseID = uint(n)
		}
		if (row.PublishedReleaseID == nil || releaseID != *row.PublishedReleaseID) && !screenHasPermission(requestAuthContext(r), row.ProjectID, "screen:edit") {
			return nil, doc, "", gorm.ErrRecordNotFound
		}
		var release store.IOCScreenRelease
		if err = store.DB.Where("screen_id = ? AND id = ?", row.ID, releaseID).First(&release).Error; err != nil {
			return nil, doc, "", err
		}
		content = release.ContentJSON
		revision = strconv.FormatUint(uint64(releaseID), 10)
	}
	r.SetCtxVar("screen_audit_revision", revision)
	doc, _, err = screen.Validate([]byte(content))
	return row, doc, revision, err
}

func screenReadJSON(r *ghttp.Request, dest interface{}, limits ...int64) error {
	limit := int64(screen.MaxDocumentBytes + 65536)
	if len(limits) > 0 {
		limit = limits[0]
	}
	if r.ContentLength > limit {
		return screenFail(413, "Screen request exceeds limit / 大屏请求超过大小限制")
	}
	reader := io.LimitReader(r.Body, limit+1)
	dec := json.NewDecoder(reader)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dest); err != nil {
		return screenFail(400, "Invalid request JSON / 请求 JSON 无效")
	}
	if dec.Decode(new(interface{})) != io.EOF {
		return screenFail(400, "Request must contain one JSON value / 请求只能包含一份 JSON")
	}
	return nil
}
func screenReadDocument(r *ghttp.Request) (screen.Envelope, []screen.ResourceRef, error) {
	var body struct {
		Document json.RawMessage `json:"document"`
	}
	if err := screenReadJSON(r, &body); err != nil {
		return screen.Envelope{}, nil, err
	}
	return screen.Validate(body.Document)
}

func screenAuthorizeRefs(r *ghttp.Request, row *store.IOCScreen, refs []screen.ResourceRef, docs ...screen.Envelope) error {
	checked := map[string]bool{}
	for _, ref := range refs {
		if ref.Type == "device" && !checked[ref.ID] {
			if !requestAuthContext(r).HasProjectPermission("device:list", row.ProjectID) {
				return screenFail(403, "Device read permission required / 需要设备读取权限")
			}
			device, err := store.GetDevice(ref.ID)
			if err != nil || device.TenantID != row.TenantID || device.ProjectID != row.ProjectID || !canAccessDevice(r, device) {
				return screenFail(403, "Binding device is outside readable scope / 绑定设备不在可读范围")
			}
			checked[ref.ID] = true
		}
		if ref.Type == "asset" {
			var asset store.IOCScreenAsset
			id, err := strconv.ParseUint(ref.ID, 10, 64)
			if err != nil || id == 0 {
				return screenFail(400, "Invalid asset id / 资产编号无效")
			}
			if store.DB.Where("id = ? AND tenant_id = ? AND project_id = ?", id, row.TenantID, row.ProjectID).First(&asset).Error != nil {
				return screenFail(403, "Asset is outside project scope / 资产不在项目范围")
			}
		}
	}
	var doc screen.Envelope
	if len(docs) > 0 {
		doc = docs[0]
	} else {
		var err error
		doc, _, err = screen.Validate([]byte(row.DraftJSON))
		if err != nil {
			return err
		}
	}
	for _, binding := range doc.Bindings {
		if binding.Kind == "dataset" {
			plan, err := screenDatasetPlan(row, binding)
			if err != nil {
				return err
			}
			input, err := dataquery.Resolve(plan, binding.Parameters, 0, 0, time.Now().UnixMilli())
			if err != nil {
				return err
			}
			if _, _, err = screenPrepareData(r, row, plan, input); err != nil {
				return err
			}
		}
	}
	return nil
}

func screenDraftResponse(r *ghttp.Request, row *store.IOCScreen) {
	doc, _, err := screen.Validate([]byte(row.DraftJSON))
	if err != nil {
		screenWriteError(r, err)
		return
	}
	r.Response.Header().Set("ETag", fmt.Sprintf("\"draft-%d\"", row.DraftVersion))
	r.SetCtxVar("screen_audit_revision", fmt.Sprintf("draft-%d", row.DraftVersion))
	screenWriteData(r, map[string]interface{}{"screen": row, "document": doc, "draftVersion": row.DraftVersion})
}
func screenBaseVersion(r *ghttp.Request) (uint64, error) {
	return screenMatchVersion(r, "draft")
}

func screenMatchVersion(r *ghttp.Request, kind string) (uint64, error) {
	value := r.Header.Get("If-Match")
	if value == "" {
		return 0, screenFail(428, "If-Match version is required / 必须提供版本")
	}
	prefix := "\"" + kind + "-"
	if !strings.HasPrefix(value, prefix) || !strings.HasSuffix(value, "\"") {
		return 0, screenFail(400, "Invalid If-Match version / 版本格式无效")
	}
	version, err := strconv.ParseUint(strings.TrimSuffix(strings.TrimPrefix(value, prefix), "\""), 10, 64)
	if err != nil || version == 0 {
		return 0, screenFail(400, "Invalid version / 版本无效")
	}
	return version, nil
}

func (s *Server) handleScreenList(r *ghttp.Request) {
	tenantID, projectID, err := screenRequire(r, "screen:list")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	page := r.GetQuery("page", 1).Int()
	if page < 1 {
		page = 1
	}
	pageSize := r.GetQuery("pageSize", 20).Int()
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	q := strings.TrimSpace(r.GetQuery("q").String())
	if q == "" {
		q = strings.TrimSpace(r.GetQuery("search").String())
	}
	if len(q) > 256 {
		q = q[:256]
	}
	query := store.DB.Model(&store.IOCScreen{}).Where("tenant_id = ? AND project_id = ?", tenantID, projectID)
	if q != "" {
		query = query.Where("name LIKE ?", "%"+q+"%")
	}
	var total int64
	var rows []store.IOCScreen
	if err = query.Count(&total).Error; err == nil {
		err = query.Select("id", "tenant_id", "project_id", "name", "draft_version", "published_release_id", "created_by", "updated_by", "created_at", "updated_at").Order("updated_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if rows == nil {
		rows = []store.IOCScreen{}
	}
	screenWriteData(r, map[string]interface{}{"items": rows, "total": total})
}
func (s *Server) handleScreenCreate(r *ghttp.Request) {
	tenantID, projectID, err := screenRequire(r, "screen:create")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	row, err := store.ScreenCreate(store.DB, tenantID, projectID, requestAuthContext(r).UserID, body.Name, screen.DefaultDocument(), nil)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	r.SetCtxVar("screen_audit_screen_id", row.ID)
	r.SetCtxVar("screen_audit_revision", fmt.Sprintf("draft-%d", row.DraftVersion))
	screenWriteData(r, row)
}
func (s *Server) handleScreenGet(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:view")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, row)
}
func (s *Server) handleScreenMetadata(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	version, err := screenMatchVersion(r, "metadata")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || len([]rune(body.Name)) > 128 {
		screenWriteError(r, errors.New("Name must contain 1–128 characters / 名称需要 1–128 个字符"))
		return
	}
	row, err = store.ScreenRename(store.DB, row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID, version, body.Name)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	r.Response.Header().Set("ETag", fmt.Sprintf("\"metadata-%d\"", row.MetadataVersion))
	screenWriteData(r, row)
}
func (s *Server) handleScreenDraft(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenDraftResponse(r, row)
}
func (s *Server) handleScreenSaveDraft(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	version, err := screenBaseVersion(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, refs, err := screenReadDocument(r)
	if err == nil {
		err = screenAuthorizeRefs(r, row, refs, doc)
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	row, err = store.ScreenSaveDraft(store.DB, row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID, version, doc, refs)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenDraftResponse(r, row)
}
func (s *Server) handleScreenValidate(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, refs, err := screenReadDocument(r)
	if err == nil {
		err = screenAuthorizeRefs(r, row, refs, doc)
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"valid": true, "resourceCount": len(refs)})
}
func (s *Server) handleScreenPublish(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:publish")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		DraftVersion uint64 `json:"draftVersion"`
		RequestID    string `json:"requestId"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	r.SetCtxVar("screen_audit_request_id", body.RequestID)
	release, err := store.ScreenPublish(store.DB, row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID, body.DraftVersion, body.RequestID, func(candidate *store.IOCScreen, refs []screen.ResourceRef) error {
		return screenAuthorizeRefs(r, candidate, refs)
	})
	if err != nil {
		screenWriteError(r, err)
		return
	}
	r.SetCtxVar("screen_audit_revision", strconv.FormatUint(uint64(release.ID), 10))
	screenWriteData(r, release)
}
func (s *Server) handleScreenUnpublish(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:publish")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	err = store.DB.Model(row).Updates(map[string]interface{}{"published_release_id": nil, "updated_by": requestAuthContext(r).UserID}).Error
	if err != nil {
		screenWriteError(r, err)
		return
	}
	row.PublishedReleaseID = nil
	r.SetCtxVar("screen_audit_revision", "unpublished")
	screenWriteData(r, row)
}
func (s *Server) handleScreenPublished(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:view")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if row.PublishedReleaseID == nil {
		screenWriteError(r, gorm.ErrRecordNotFound)
		return
	}
	screenReleaseResponse(r, row, *row.PublishedReleaseID)
}
func screenReleaseResponse(r *ghttp.Request, row *store.IOCScreen, id uint) {
	var release store.IOCScreenRelease
	if err := store.DB.Where("screen_id = ? AND id = ?", row.ID, id).First(&release).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	doc, _, err := screen.Validate([]byte(release.ContentJSON))
	if err != nil {
		screenWriteError(r, err)
		return
	}
	doc, redacted := screenVisibleDocument(r, row, doc)
	screenWriteData(r, map[string]interface{}{"screen": row, "release": release, "document": doc, "redactedBindings": redacted})
}

// screenVisibleDocument strips device identities and field definitions only at
// the viewer HTTP boundary. Internal runtime authorization always receives the
// original validated document, and stable binding/target IDs remain intact.
func screenVisibleDocument(r *ghttp.Request, row *store.IOCScreen, doc screen.Envelope) (screen.Envelope, []string) {
	redacted := []string{}
	auth := requestAuthContext(r)
	if screenHasPermission(auth, row.ProjectID, "screen:edit") {
		return doc, redacted
	}
	bindings := make(map[string]screen.Binding, len(doc.Bindings))
	readable := map[string]bool{}
	checked := map[string]bool{}
	for id, binding := range doc.Bindings {
		if binding.Kind == "dataset" {
			plan, err := screenDatasetPlan(row, binding)
			if err == nil {
				input, resolveErr := dataquery.Resolve(plan, binding.Parameters, 0, 0, time.Now().UnixMilli())
				err = resolveErr
				if err == nil {
					_, _, err = screenPrepareData(r, row, plan, input)
				}
			}
			if err == nil {
				bindings[id] = binding
			} else {
				bindings[id] = screen.Binding{Kind: binding.Kind}
				redacted = append(redacted, id)
			}
			continue
		}
		allowed := auth != nil && auth.HasProjectPermission("device:list", row.ProjectID)
		devices := append([]string{}, binding.DeviceCodes...)
		if binding.DeviceCode != "" {
			devices = append(devices, binding.DeviceCode)
		}
		for _, code := range devices {
			if !checked[code] {
				device, err := store.GetDevice(code)
				readable[code] = err == nil && device.TenantID == row.TenantID && device.ProjectID == row.ProjectID && canAccessDevice(r, device)
				checked[code] = true
			}
			allowed = allowed && readable[code]
		}
		if allowed {
			bindings[id] = binding
		} else {
			bindings[id] = screen.Binding{Kind: binding.Kind}
			redacted = append(redacted, id)
		}
	}
	sort.Strings(redacted)
	doc.Bindings = bindings
	return doc, redacted
}
func (s *Server) handleScreenReleases(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var items []store.IOCScreenRelease
	if err = store.DB.Where("screen_id = ?", row.ID).Order("release_no DESC").Limit(100).Find(&items).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	if items == nil {
		items = []store.IOCScreenRelease{}
	}
	screenWriteData(r, map[string]interface{}{"items": items})
}
func (s *Server) handleScreenRelease(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenReleaseResponse(r, row, r.Get("releaseId").Uint())
}
func (s *Server) handleScreenRestore(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var body struct {
		DraftVersion uint64 `json:"draftVersion"`
		RequestID    string `json:"requestId"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	r.SetCtxVar("screen_audit_request_id", body.RequestID)
	var release store.IOCScreenRelease
	if err = store.DB.Where("id = ? AND screen_id = ?", r.Get("releaseId").Uint(), row.ID).First(&release).Error; err != nil {
		screenWriteError(r, err)
		return
	}
	doc, refs, err := screen.Validate([]byte(release.ContentJSON))
	if err == nil {
		err = screenAuthorizeRefs(r, row, refs, doc)
	}
	if err != nil {
		screenWriteError(r, err)
		return
	}
	row, err = store.ScreenRestore(store.DB, row.TenantID, row.ProjectID, row.ID, release.ID, requestAuthContext(r).UserID, body.DraftVersion, body.RequestID)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenDraftResponse(r, row)
}
func (s *Server) handleScreenCopy(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if !screenHasPermission(requestAuthContext(r), row.ProjectID, "screen:create") {
		screenWriteError(r, screenFail(403, "Create permission required / 需要创建权限"))
		return
	}
	var body struct {
		Name         string `json:"name"`
		DraftVersion uint64 `json:"draftVersion"`
		RequestID    string `json:"requestId"`
	}
	if err = screenReadJSON(r, &body); err != nil {
		screenWriteError(r, err)
		return
	}
	r.SetCtxVar("screen_audit_request_id", body.RequestID)
	copy, err := store.ScreenCopy(store.DB, row.TenantID, row.ProjectID, row.ID, requestAuthContext(r).UserID, body.DraftVersion, body.Name, body.RequestID, func(candidate *store.IOCScreen, refs []screen.ResourceRef) error {
		return screenAuthorizeRefs(r, candidate, refs)
	})
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, copy)
}
func (s *Server) handleScreenDelete(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:delete")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	err = store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(row).Update("published_release_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(row).Error
	})
	if err != nil {
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"deleted": true})
}

func (s *Server) handleScreenAssets(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	var items []store.IOCScreenAsset
	refs := store.DB.Model(&store.IOCScreenResourceRef{}).Select("resource_id").Where("tenant_id = ? AND project_id = ? AND owner_kind = ? AND owner_id = ? AND resource_type = ?", row.TenantID, row.ProjectID, "draft", row.ID, "asset")
	query := store.DB.Where("tenant_id = ? AND project_id = ? AND (screen_id = ? OR id IN (?))", row.TenantID, row.ProjectID, row.ID, refs)
	if cursor := r.GetQuery("cursor").Uint(); cursor != 0 {
		query = query.Where("id < ?", cursor)
	}
	err = query.Order("id DESC").Limit(33).Find(&items).Error
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if items == nil {
		items = []store.IOCScreenAsset{}
	}
	var nextCursor uint
	if len(items) > 32 {
		items = items[:32]
		nextCursor = items[len(items)-1].ID
	}
	screenWriteData(r, map[string]interface{}{"items": items, "nextCursor": nextCursor})
}
func (s *Server) handleScreenUploadAsset(r *ghttp.Request) {
	row, err := screenLoad(r, "screen:edit")
	if err != nil {
		screenWriteError(r, err)
		return
	}
	if r.ContentLength > maxImageUploadBytes+(1<<20) {
		screenWriteError(r, screenFail(413, "Image exceeds 10 MiB / 图片超过 10 MiB"))
		return
	}
	r.Request.Body = http.MaxBytesReader(r.Response.Writer, r.Request.Body, maxImageUploadBytes+(1<<20))
	upload := r.GetUploadFile("file")
	if upload == nil {
		screenWriteError(r, errors.New("Image file required / 请选择图片"))
		return
	}
	src, err := upload.Open()
	if err != nil {
		screenWriteError(r, err)
		return
	}
	cfg, format, err := image.DecodeConfig(io.LimitReader(src, maxImageUploadBytes+1))
	src.Close()
	if err != nil || (format != "png" && format != "jpeg" && format != "webp") {
		screenWriteError(r, errors.New("Only PNG, JPEG and WebP are supported / 仅支持 PNG、JPEG 和 WebP"))
		return
	}
	name, err := saveUploadedImage(upload, screenAssetDirectory)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	f, err := openStoredFile(screenAssetDirectory, name)
	if err != nil {
		os.Remove(screenAssetDirectory + "/" + name)
		screenWriteError(r, err)
		return
	}
	hash := sha256.New()
	size, err := io.Copy(hash, f)
	f.Close()
	if err != nil {
		os.Remove(screenAssetDirectory + "/" + name)
		screenWriteError(r, err)
		return
	}
	displayName := strings.ReplaceAll(upload.Filename, "\\", "/")
	if slash := strings.LastIndex(displayName, "/"); slash >= 0 {
		displayName = displayName[slash+1:]
	}
	displayRunes := []rune(displayName)
	if len(displayRunes) > 128 {
		displayName = string(displayRunes[:128])
	}
	asset := store.IOCScreenAsset{Name: displayName, TenantID: row.TenantID, ProjectID: row.ProjectID, ScreenID: row.ID, StorageKey: name, SHA256: hex.EncodeToString(hash.Sum(nil)), MIME: "image/png", Size: size, Width: cfg.Width, Height: cfg.Height, Uploader: requestAuthContext(r).UserID}
	err = store.DB.Create(&asset).Error
	if err != nil {
		os.Remove(screenAssetDirectory + "/" + name)
		screenWriteError(r, err)
		return
	}
	screenWriteData(r, map[string]interface{}{"assetId": strconv.FormatUint(uint64(asset.ID), 10), "asset": asset})
}
func (s *Server) handleScreenAsset(r *ghttp.Request) {
	row, doc, revision, err := screenAuthorizedDocument(r)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	documentJSON, _ := json.Marshal(doc)
	_, refs, err := screen.Validate(documentJSON)
	if err != nil {
		screenWriteError(r, err)
		return
	}
	id := r.Get("assetId").Uint()
	var asset store.IOCScreenAsset
	if err = store.DB.Where("id = ? AND tenant_id = ? AND project_id = ?", id, row.TenantID, row.ProjectID).First(&asset).Error; err != nil {
		screenWriteError(r, gorm.ErrRecordNotFound)
		return
	}
	allowed := false
	for _, ref := range refs {
		if ref.Type == "asset" && ref.ID == strconv.FormatUint(uint64(id), 10) {
			allowed = true
			break
		}
	}
	if !allowed && strings.HasPrefix(revision, "draft-") && asset.ScreenID == row.ID {
		allowed = true
	}
	if !allowed {
		screenWriteError(r, gorm.ErrRecordNotFound)
		return
	}
	f, err := openStoredFile(screenAssetDirectory, asset.StorageKey)
	if err != nil {
		screenWriteError(r, gorm.ErrRecordNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		screenWriteError(r, gorm.ErrRecordNotFound)
		return
	}
	r.Response.Header().Set("Content-Type", asset.MIME)
	r.Response.Header().Set("X-Content-Type-Options", "nosniff")
	r.Response.Header().Set("Cache-Control", "private, no-store")
	r.Response.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	if r.GetQuery("thumbnail").Bool() {
		if err := screenWriteThumbnail(r.Response.Writer, f); err != nil {
			screenWriteError(r, err)
		}
		return
	}
	http.ServeContent(r.Response.Writer, r.Request, asset.StorageKey, info.ModTime(), f)
}
