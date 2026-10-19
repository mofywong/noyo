package core

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// RegisterLocationRoutes 注册设备地图与地理围栏相关接口。
func (s *Server) RegisterLocationRoutes(group *ghttp.RouterGroup) {
	mapGroup := group.Group("/device-map")

	// 地图页一次拉全量设备（含位置），由前端做视口与聚合运算。
	permissionGET(mapGroup, "/devices", "device_map:view", s.handleListMapDevices)
	permissionGET(mapGroup, "/devices/:code", "device_map:view", s.handleGetDeviceLocationView)
	permissionPUT(mapGroup, "/devices/:code/location", "device_location:edit", s.handleSetDeviceLocation)
	permissionDELETE(mapGroup, "/devices/:code/location", "device_location:edit", s.handleClearDeviceLocation)

	permissionGET(mapGroup, "/fences", "geofence:list", s.handleListGeofences)
	permissionPOST(mapGroup, "/fences", "geofence:create", s.handleCreateGeofence)
	permissionPUT(mapGroup, "/fences/:id", "geofence:edit", s.handleUpdateGeofence)
	permissionDELETE(mapGroup, "/fences/:id", "geofence:delete", s.handleDeleteGeofence)
	permissionGET(mapGroup, "/fences/states", "device_map:view", s.handleListFenceStates)
	permissionPUT(mapGroup, "/fences/:id/devices", "geofence:edit", s.handleSetFenceDevices)

	// 地点搜索与行政区边界。用 view 权限即可：这两条只读取公共地理数据，
	// 真正的写操作仍由围栏的 create/edit 权限把关。
	permissionGET(mapGroup, "/geocode", "device_map:view", s.handleGeocodeSearch)
	permissionGET(mapGroup, "/region", "device_map:view", s.handleRegionLookup)

	// 无设备时的默认视野：按公网出口 IP 的城市级粗定位。
	// 只读、无副作用，同样用 view 权限；拿不到时前端退回中国全境。
	permissionGET(mapGroup, "/ip-location", "device_map:view", s.handleIPLocation)
}

// currentScopeFromRequest 读取请求上下文的租户 / 项目范围与项目白名单。
func currentScopeFromRequest(r *ghttp.Request) (uint, uint, []uint) {
	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()

	var projectIDs []uint
	if authCtx := requestAuthContext(r); authCtx != nil {
		if ids, restricted := authCtx.ProjectIDsForTenantQuery(); restricted && projectID == 0 {
			projectIDs = ids
		}
	}
	return tenantID, projectID, projectIDs
}

func (s *Server) handleListMapDevices(r *ghttp.Request) {
	if s.Location == nil {
		r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"devices": []any{}}})
		return
	}

	tenantID, projectID, projectIDs := currentScopeFromRequest(r)

	filter := store.DeviceQueryFilter{
		TenantID:          tenantID,
		ProjectID:         projectID,
		AllowedProjectIDs: projectIDs,
		Search:            r.Get("search").String(),
		ProductCode:       r.Get("product_code").String(),
	}

	if tagID := r.Get("tag_id").Uint(); tagID > 0 {
		filter.TagID = tagID
	}

	// 与设备列表保持一致：网关自身不在地图上作为业务设备展示。
	if localGatewayCode := store.LocalConfigGatewayCode(); localGatewayCode != "" {
		filter.ExcludeCodes = []string{localGatewayCode}
	}

	items, err := s.Location.ListMapDevices(filter)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to load device locations: " + err.Error()})
		return
	}

	located := 0
	for _, item := range items {
		if item.HasLocation {
			located++
		}
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"devices":         items,
		"total":           len(items),
		"located_total":   located,
		"unlocated_total": len(items) - located,
	}})
}

func (s *Server) handleGetDeviceLocationView(r *ghttp.Request) {
	deviceCode := r.Get("code").String()
	if deviceCode == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device code is required"})
		return
	}
	device, err := store.GetDevice(deviceCode)
	if err != nil || device == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	view, err := s.Location.DeviceLocationView(deviceCode)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	// 附上语义解析细节（显式标记 / 自动识别 / 映射点 precision 告警）
	semantics, _ := s.Location.SemanticsForDeviceDetailed(deviceCode)

	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"device":    view,
		"semantics": semantics,
	}})
}

type setDeviceLocationRequest struct {
	Longitude *float64 `json:"longitude"`
	Latitude  *float64 `json:"latitude"`
}

func (s *Server) handleSetDeviceLocation(r *ghttp.Request) {
	deviceCode := r.Get("code").String()
	if deviceCode == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device code is required"})
		return
	}
	device, err := store.GetDevice(deviceCode)
	if err != nil || device == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var payload setDeviceLocationRequest
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	if payload.Longitude == nil || payload.Latitude == nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Longitude and latitude are required"})
		return
	}

	operator := ""
	if authCtx := requestAuthContext(r); authCtx != nil {
		operator = authCtx.Username
		if operator == "" {
			operator = strconv.FormatUint(uint64(authCtx.UserID), 10)
		}
	}

	err = s.Location.SetManualLocation(deviceCode, *payload.Longitude, *payload.Latitude, operator)
	switch {
	case err == nil:
	case isLocationConflictError(err):
		// 来源互斥：遥测设备的位置由物模型点位管理，人工写入会被下一次上报静默覆盖。
		r.Response.WriteJson(g.Map{
			"code":    409,
			"message": "该设备的位置由物模型遥测点位上报，已禁用手动编辑 / This device reports its own location through telemetry points; manual editing is disabled",
		})
		return
	default:
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}

	view, _ := s.Location.DeviceLocationView(deviceCode)
	r.Response.WriteJson(g.Map{"code": 0, "data": view, "message": "Device location saved"})
}

func (s *Server) handleClearDeviceLocation(r *ghttp.Request) {
	deviceCode := r.Get("code").String()
	device, err := store.GetDevice(deviceCode)
	if err != nil || device == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if err := s.Location.ClearLocation(deviceCode); err != nil {
		if isLocationConflictError(err) {
			r.Response.WriteJson(g.Map{"code": 409, "message": err.Error()})
			return
		}
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Device location cleared"})
}

func isLocationConflictError(err error) bool {
	if err == nil {
		return false
	}
	if err == ErrLocationManagedByTelemetry {
		return true
	}
	return strings.Contains(err.Error(), ErrLocationManagedByTelemetry.Error())
}

// ===================== 围栏 =====================

func (s *Server) handleListGeofences(r *ghttp.Request) {
	tenantID, projectID, _ := currentScopeFromRequest(r)
	fences, err := store.ListGeofences(tenantID, projectID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	if fences == nil {
		fences = []store.Geofence{}
	}
	// 绑定设备一次装配，避免前端为每条围栏各发一次请求。
	if assigned, assignErr := store.ListGeofenceDeviceMap(); assignErr == nil {
		for i := range fences {
			if set, ok := assigned[fences[i].ID]; ok {
				codes := make([]string, 0, len(set))
				for code := range set {
					codes = append(codes, code)
				}
				sort.Strings(codes)
				fences[i].AssignedDevices = codes
			}
		}
	}
	states, _ := store.ListDeviceFenceStates("")

	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"fences": fences,
		"states": states,
	}})
}

type geofenceRequest struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	CenterLng   *float64 `json:"center_lng"`
	CenterLat   *float64 `json:"center_lat"`
	RadiusM     *float64 `json:"radius_m"`
	Geometry    string   `json:"geometry"`
	RuleType    string   `json:"rule_type"`
	EnterDwellS *int     `json:"enter_dwell_s"`
	ExitDwellS  *int     `json:"exit_dwell_s"`
	Enabled     *bool    `json:"enabled"`
	Color       string   `json:"color"`

	Source      string `json:"source"`
	RegionName  string `json:"region_name"`
	RegionLevel string `json:"region_level"`

	// DeviceCodes 为空数组表示"不绑定 → 对全部设备生效"；
	// 为 nil 表示这次请求不改动绑定关系（编辑名称时不该把绑定清空）。
	DeviceCodes []string `json:"device_codes"`
}

func (req geofenceRequest) apply(fence *store.Geofence) {
	fence.Name = strings.TrimSpace(req.Name)
	fence.Kind = req.Kind
	fence.CenterLng = req.CenterLng
	fence.CenterLat = req.CenterLat
	fence.RadiusM = req.RadiusM
	fence.Geometry = req.Geometry
	fence.RuleType = req.RuleType
	fence.Color = req.Color
	fence.Source = req.Source
	fence.RegionName = req.RegionName
	fence.RegionLevel = req.RegionLevel
	if req.EnterDwellS != nil {
		fence.EnterDwellS = *req.EnterDwellS
	}
	if req.ExitDwellS != nil {
		fence.ExitDwellS = *req.ExitDwellS
	}
	if req.Enabled != nil {
		fence.Enabled = *req.Enabled
	}
}

func (s *Server) handleCreateGeofence(r *ghttp.Request) {
	var req geofenceRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	// 圆形围栏已从前端移除：新建只接受多边形（手绘或按地区生成）。
	// 历史圆形围栏仍然可读、可判定、可编辑，只是不能再新建。
	if req.Kind == store.FenceKindCircle {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Circle geofences are no longer supported; draw a polygon instead"})
		return
	}
	tenantID, projectID, _ := currentScopeFromRequest(r)

	fence := &store.Geofence{TenantID: tenantID, ProjectID: projectID, Enabled: true}
	req.apply(fence)
	if authCtx := requestAuthContext(r); authCtx != nil {
		fence.CreatedBy = authCtx.Username
	}

	if err := s.Geofence.CreateGeofence(fence); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	if req.DeviceCodes != nil {
		if err := store.ReplaceGeofenceDevices(fence.ID, tenantID, projectID, req.DeviceCodes); err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
			return
		}
		s.Geofence.InvalidateCache()
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": fence, "message": "Geofence created"})
}

func (s *Server) handleUpdateGeofence(r *ghttp.Request) {
	id := r.Get("id").Uint()
	if id == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Geofence id is required"})
		return
	}
	existing, err := store.GetGeofence(id)
	if err != nil || existing == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Geofence not found"})
		return
	}
	if !canAccessGeofence(r, existing) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var req geofenceRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	req.apply(existing)

	if err := s.Geofence.UpdateGeofence(existing); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	if req.DeviceCodes != nil {
		if err := store.ReplaceGeofenceDevices(existing.ID, existing.TenantID, existing.ProjectID, req.DeviceCodes); err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
			return
		}
		s.Geofence.InvalidateCache()
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": existing, "message": "Geofence updated"})
}

// handleSetFenceDevices 整体替换某围栏绑定的设备集合。
//
// 单独开一个接口而不是塞进 PUT /fences/:id：绑定关系经常单独调整
// （新设备上线就加到围栏里），不该逼前端把整条围栏定义重发一遍 ——
// 那样任何一处校验失败都会连带改不了绑定。
func (s *Server) handleSetFenceDevices(r *ghttp.Request) {
	id := r.Get("id").Uint()
	if id == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Geofence id is required"})
		return
	}
	existing, err := store.GetGeofence(id)
	if err != nil || existing == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Geofence not found"})
		return
	}
	if !canAccessGeofence(r, existing) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var req struct {
		DeviceCodes []string `json:"device_codes"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	if req.DeviceCodes == nil {
		req.DeviceCodes = []string{}
	}
	// 去重 + 过滤空串：前端多选组件在极端情况下会带空项。
	seen := make(map[string]bool, len(req.DeviceCodes))
	codes := make([]string, 0, len(req.DeviceCodes))
	for _, code := range req.DeviceCodes {
		trimmed := strings.TrimSpace(code)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		codes = append(codes, trimmed)
	}
	if err := store.ReplaceGeofenceDevices(id, existing.TenantID, existing.ProjectID, codes); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.Geofence.InvalidateCache()
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"device_codes": codes}, "message": "Geofence devices updated"})
}

// writeGeoError 把地理服务的错误翻成「前端能自己本地化」的形状。
//
// 只回分类后的 reason 和一句短说明：上游 URL、EOF 之类的细节只写日志。
// 前端据 reason 选文案，不再直接显示后端字符串（线上出现过把
// `Get "https://nominatim…": EOF` 原样展示给用户的情况）。
func (s *Server) writeGeoError(r *ghttp.Request, scope string, query string, err error) {
	var userErr *geoUserError
	if errors.As(err, &userErr) {
		r.Response.WriteJson(g.Map{
			"code":    userErr.Code,
			"message": userErr.Detail,
			"data":    g.Map{"reason": userErr.Reason},
		})
		return
	}

	code, reason, message := 502, "failed", "geocoding request failed"
	switch {
	case errors.Is(err, ErrGeoUnavailable):
		// 5xx / 429 或网络层失败：过一会可能就好，提示用户重试。
		code, reason, message = 503, "unreachable", "geocoding service is temporarily unreachable"
	case errors.Is(err, ErrGeoRejected):
		code, reason, message = 502, "rejected", "geocoding service rejected the request"
	case errors.Is(err, ErrGeoNoMatch):
		code, reason, message = 404, "no_match", "no place matched the keyword"
	case errors.Is(err, ErrGeoNoBoundary):
		code, reason, message = 404, "no_boundary", "matched place has no administrative boundary"
	}

	if s != nil && s.Logger != nil {
		// 详情只进日志：这里的 err 可能带着上游地址与底层错误。
		s.Logger.Warn("Geocoding request failed",
			zap.String("scope", scope), zap.String("query", query),
			zap.String("reason", reason), zap.Error(err))
	}
	r.Response.WriteJson(g.Map{
		"code":    code,
		"message": message,
		"data":    g.Map{"reason": reason},
	})
}

// handleGeocodeSearch 按名称搜索地点（地图上的搜索框）。
func (s *Server) handleGeocodeSearch(r *ghttp.Request) {
	query := strings.TrimSpace(r.Get("q").String())
	if query == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "q is required", "data": g.Map{"reason": "empty_query"}})
		return
	}
	limit := r.Get("limit").Int()
	places, err := s.SearchPlaces(query, limit)
	if err != nil {
		s.writeGeoError(r, "search", query, err)
		return
	}
	if places == nil {
		places = []GeocodePlace{}
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"places": places, "source": "osm"}})
}

// handleRegionLookup 按地区名称取行政区边界，供「把某个市/街道设为围栏」使用。
func (s *Server) handleRegionLookup(r *ghttp.Request) {
	query := strings.TrimSpace(r.Get("q").String())
	if query == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "q is required", "data": g.Map{"reason": "empty_query"}})
		return
	}
	boundary, err := s.LookupRegionBoundary(query)
	if err != nil {
		s.writeGeoError(r, "region", query, err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": boundary})
}

func (s *Server) handleDeleteGeofence(r *ghttp.Request) {
	id := r.Get("id").Uint()
	if id == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Geofence id is required"})
		return
	}
	existing, err := store.GetGeofence(id)
	if err != nil || existing == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Geofence not found"})
		return
	}
	if !canAccessGeofence(r, existing) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if err := s.Geofence.DeleteGeofence(id); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Geofence deleted"})
}

func (s *Server) handleListFenceStates(r *ghttp.Request) {
	deviceCode := r.Get("device_code").String()
	states, err := store.ListDeviceFenceStates(deviceCode)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	if states == nil {
		states = []store.DeviceFenceState{}
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": states})
}

// cleanupDeviceLocationData 清理设备的位置快照、围栏绑定与判定状态。
// 设备删除时调用；失败只记录，不阻断删除主流程。
func (s *Server) cleanupDeviceLocationData(deviceCode string) {
	if deviceCode == "" {
		return
	}
	if err := store.DeleteDeviceLocation(deviceCode); err != nil && s.Logger != nil {
		s.Logger.Warn("Failed to clean up device location", zap.String("device", deviceCode), zap.Error(err))
	}
	// 绑定必须一起清：留下的孤儿绑定会让围栏看起来"还绑着一台不存在的设备"，
	// 并且让围栏从"对全部设备生效"悄悄变成"只对这台幽灵设备生效"。
	if store.DB != nil {
		if err := store.DB.Transaction(func(tx *gorm.DB) error {
			return store.DeleteGeofenceDevicesForDevice(tx, deviceCode)
		}); err != nil && s.Logger != nil {
			s.Logger.Warn("Failed to clean up geofence assignments", zap.String("device", deviceCode), zap.Error(err))
		}
	}
	if s.Geofence != nil {
		s.Geofence.ClearDevice(deviceCode)
		s.Geofence.InvalidateCache()
	}
}

// canAccessGeofence 判断当前请求是否有权操作该围栏。
func canAccessGeofence(r *ghttp.Request, fence *store.Geofence) bool {
	if fence == nil {
		return false
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil {
		return false
	}
	if authCtx.IsSystemAdmin {
		return authCtx.TenantID == 0 || fence.TenantID == authCtx.TenantID
	}
	if authCtx.TenantID != fence.TenantID {
		return false
	}
	if authCtx.ProjectID > 0 {
		return fence.ProjectID == authCtx.ProjectID
	}
	return true
}
