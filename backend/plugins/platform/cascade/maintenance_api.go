package cascade

import (
	"encoding/json"
	"errors"

	"noyo/core"
	"noyo/core/store"

	"github.com/gogf/gf/v2/net/ghttp"
)

// /maintenance is the local gateway status-bar entry. The gateway code is
// implicit only here, because the server is bound to exactly one gateway.
func (p *CascadePlugin) handleLocalMaintenance(r *ghttp.Request) {
	active := p.getActivePlugin()
	engine, ok := active.GatewayEngine.(*gatewayEngineImpl)
	if !ok || active.Config.Mode != "gateway" || engine.gatewayCode == "" {
		r.Response.WriteJson(maintenanceErrorPayload(503, errors.New("gateway mode is unavailable")))
		return
	}
	if r.Method == "GET" {
		state, err := store.GetGatewayMaintenance(engine.gatewayCode)
		writeMaintenanceResponse(r, state, err)
		return
	}
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	if principal := core.RequestAuthContext(r); principal == nil || !principal.HasPermission("gateway:config") {
		r.Response.WriteStatus(403, map[string]interface{}{"code": 403, "message": "Access denied"})
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(r.GetBody(), &body); err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 400, "message": "Invalid JSON"})
		return
	}
	var state *store.GatewayMaintenance
	var err error
	switch body.Action {
	case "enter":
		state, err = engine.enterMaintenance()
	case "exit":
		state, err = engine.exitMaintenance()
	default:
		r.Response.WriteJson(maintenanceErrorPayload(400, errors.New("action must be enter or exit")))
		return
	}
	writeMaintenanceResponse(r, state, err)
}

// The platform can request entry while the gateway is online. The gateway
// reports the authoritative mode; a published request is never presented as a
// completed mode switch.
func (p *CascadePlugin) handleGatewayMaintenance(r *ghttp.Request) {
	active := p.getActivePlugin()
	engine, ok := active.PlatformEngine.(*platformEngineImpl)
	if !ok || active.Config.Mode != "platform" {
		r.Response.WriteJson(maintenanceErrorPayload(503, errors.New("platform mode is unavailable")))
		return
	}
	code := p.getRouteParam(r, "gwSn")
	device, err := store.GetDevice(code)
	if err != nil || device == nil || !active.IsGatewayDevice(code) {
		r.Response.WriteJson(map[string]interface{}{"code": 404, "message": "Gateway not found"})
		return
	}
	if tenant := r.GetCtxVar("tenant_id").Uint(); tenant != 0 && device.TenantID != tenant {
		r.Response.WriteJson(map[string]interface{}{"code": 403, "message": "Access denied"})
		return
	}
	if project := r.GetCtxVar("project_id").Uint(); project != 0 && device.ProjectID != project {
		r.Response.WriteJson(map[string]interface{}{"code": 403, "message": "Access denied"})
		return
	}
	if r.Method == "GET" {
		state, err := store.GetGatewayMaintenance(code)
		writeMaintenanceResponse(r, state, err)
		return
	}
	if r.Method != "POST" {
		r.Response.WriteStatus(405)
		return
	}
	if principal := core.RequestAuthContext(r); principal == nil || !principal.HasProjectPermission("gateway:config", device.ProjectID) {
		r.Response.WriteStatus(403, map[string]interface{}{"code": 403, "message": "Access denied"})
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := json.Unmarshal(r.GetBody(), &body); err != nil || (body.Action != "enter" && body.Action != "exit") {
		r.Response.WriteJson(maintenanceErrorPayload(400, errors.New("action must be enter or exit")))
		return
	}
	if err := engine.requestMaintenanceAction(code, body.Action); err != nil {
		r.Response.WriteJson(maintenanceErrorPayload(503, err))
		return
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": map[string]string{"gateway_code": code, "status": "requested"}})
}

func (p *CascadePlugin) handleGatewayRuntimeResults(r *ghttp.Request) {
	active := p.getActivePlugin()
	if active.Config.Mode != "platform" || r.Method != "GET" {
		r.Response.WriteStatus(405)
		return
	}
	principal := core.RequestAuthContext(r)
	if principal == nil || !principal.HasPermission("gateway:config") {
		r.Response.WriteStatus(403)
		return
	}
	code := p.getRouteParam(r, "gwSn")
	device, err := store.GetDevice(code)
	if err != nil || device == nil || !active.IsGatewayDevice(code) {
		r.Response.WriteJson(map[string]interface{}{"code": 404, "message": "Gateway not found"})
		return
	}
	if tenant := r.GetCtxVar("tenant_id").Uint(); tenant != 0 && device.TenantID != tenant {
		r.Response.WriteStatus(403)
		return
	}
	if project := r.GetCtxVar("project_id").Uint(); project != 0 && device.ProjectID != project {
		r.Response.WriteStatus(403)
		return
	}
	var authorized int64
	if err := configurationScope(store.DB.Model(&store.Device{}), principal, r.GetCtxVar("tenant_id").Uint(), r.GetCtxVar("project_id").Uint()).
		Where("code = ?", code).Count(&authorized).Error; err != nil {
		r.Response.WriteStatus(500)
		return
	}
	if authorized == 0 || !principal.HasProjectPermission("gateway:config", device.ProjectID) {
		r.Response.WriteStatus(403)
		return
	}
	status := r.GetQuery("status").String()
	if status != "" && status != store.RuntimeApplyFailed {
		r.Response.WriteStatus(400, map[string]interface{}{"code": 400, "message": "Unsupported runtime status"})
		return
	}
	if status == store.RuntimeApplyFailed {
		page, pageSize := r.GetQuery("page", 1).Int(), r.GetQuery("page_size", 100).Int()
		if page < 1 || pageSize < 1 || pageSize > 500 || page-1 > int(^uint(0)>>1)/pageSize {
			r.Response.WriteStatus(400, map[string]interface{}{"code": 400, "message": "Invalid runtime result pagination"})
			return
		}
		results, total, err := listGatewayRuntimeFailures(store.DB, code, page, pageSize)
		if err != nil {
			r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
			return
		}
		for i := range results {
			results[i].LastError = redactConfigurationError(results[i].LastError)
			results[i].CompensationError = redactConfigurationError(results[i].CompensationError)
		}
		r.Response.WriteJson(map[string]interface{}{
			"code": 0, "data": results, "total": total,
			"has_more": int64((page-1)*pageSize)+int64(len(results)) < total,
			"page":     page, "page_size": pageSize,
		})
		return
	}
	var results []store.GatewayRuntimeState
	if err := store.DB.Where("gateway_code = ?", code).Order("updated_at desc").Limit(500).Find(&results).Error; err != nil {
		r.Response.WriteJson(map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	for i := range results {
		results[i].LastError = redactConfigurationError(results[i].LastError)
		results[i].CompensationError = redactConfigurationError(results[i].CompensationError)
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": results})
}

func writeMaintenanceResponse(r *ghttp.Request, state *store.GatewayMaintenance, err error) {
	if err != nil {
		r.Response.WriteJson(maintenanceErrorPayload(409, err))
		return
	}
	r.Response.WriteJson(map[string]interface{}{"code": 0, "data": state})
}

// Mode transitions have their own stable error contract; callers can show a
// recovery step in either console language without parsing an internal error.
func maintenanceErrorPayload(code int, err error) map[string]interface{} {
	errorCode := "GATEWAY_MAINTENANCE_OPERATION_FAILED"
	zh := "维护状态操作失败，请检查配置的应用结果后重试。"
	en := "Could not change maintenance mode. Check the configuration apply results and retry."
	detail := redactConfigurationError(err.Error())
	switch {
	case errors.Is(err, store.ErrGatewayModeChanged):
		errorCode = "GATEWAY_MODE_CHANGED"
		zh = "网关维护状态已发生变化，请刷新状态后重试。"
		en = "The gateway maintenance state changed. Refresh the status and retry."
		detail = ""
	case errors.Is(err, store.ErrRuntimeApplyPending):
		errorCode = "GATEWAY_RUNTIME_PENDING"
		zh = "网关仍有配置等待运行生效，请等待应用完成后重试。"
		en = "Gateway configuration is still awaiting runtime activation. Wait for application to finish and retry."
		detail = ""
	case err.Error() == "sync engine is not running":
		errorCode = "GATEWAY_SYNC_UNAVAILABLE"
		zh = "网关同步服务尚未就绪，请等待服务恢复后重试。"
		en = "The gateway synchronization service is not ready. Wait for it to recover and retry."
		detail = ""
	case err.Error() == "gateway mode is unavailable" || err.Error() == "platform mode is unavailable":
		errorCode = "GATEWAY_MAINTENANCE_UNAVAILABLE"
		zh = "当前服务不支持此维护操作，请刷新页面并检查网关连接。"
		en = "This maintenance operation is unavailable. Refresh the page and check the gateway connection."
		detail = ""
	case err.Error() == "action must be enter" || err.Error() == "action must be enter or exit":
		errorCode = "GATEWAY_MAINTENANCE_ACTION_INVALID"
		zh = "维护操作无效，请刷新页面后重新选择操作。"
		en = "Invalid maintenance action. Refresh the page and choose the action again."
		detail = ""
	}
	result := map[string]interface{}{"code": code, "error_code": errorCode, "message": en, "message_zh": zh, "message_en": en}
	if detail != "" {
		result["error_detail"] = detail
	}
	return result
}
