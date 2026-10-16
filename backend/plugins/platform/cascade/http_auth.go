package cascade

import (
	"encoding/json"
	"net/http"
	"strings"

	"noyo/core"
	"noyo/core/store"

	"github.com/gogf/gf/v2/net/ghttp"
)

// Every extension route has an explicit method and permission boundary. The
// core plugin wrapper authenticates the caller; this layer authorizes the
// gateway operations without changing internal MQTT synchronization.
var cascadeHTTPPermissions = map[string]map[string]string{
	"/api/extension/cascade/status":                                    {http.MethodGet: ""},
	"/api/extension/cascade/stream":                                    {http.MethodGet: ""},
	"/api/extension/cascade/maintenance":                               {http.MethodGet: "gateway:list", http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/maintenance":                {http.MethodGet: "gateway:list", http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/runtime-results":            {http.MethodGet: "gateway:config"},
	"/api/extension/cascade/configuration/resources":                   {http.MethodGet: ""}, // Checked per resource kind by the handler.
	"/api/extension/cascade/sync/conflicts":                            {http.MethodGet: "gateway:list"},
	"/api/extension/cascade/sync/conflicts/resolve":                    {http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways":                                  {http.MethodGet: "gateway:list", http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/plugins":                    {http.MethodGet: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/plugins/:pluginName":        {http.MethodGet: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/plugins/:pluginName/config": {http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/plugins/:pluginName/status": {http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/system/config":              {http.MethodGet: "gateway:config", http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/license/status":             {http.MethodGet: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/license/upload":             {http.MethodPost: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/system/log/files":           {http.MethodGet: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/system/log/file":            {http.MethodGet: "gateway:config"},
	"/api/extension/cascade/gateways/:gwSn/system/log/tail":            {http.MethodGet: "gateway:config"},
}

func (p *CascadePlugin) authorizeHTTP(path string, handler func(*ghttp.Request)) func(*ghttp.Request) {
	return func(r *ghttp.Request) {
		permission, allowedMethod := cascadeHTTPPermissions[path][r.Method]
		if !allowedMethod {
			r.Response.WriteStatus(http.StatusMethodNotAllowed)
			return
		}
		principal := core.RequestAuthContext(r)
		if principal == nil || (permission != "" && !principal.HasPermission(permission)) {
			writeCascadeAccessDenied(r)
			return
		}
		if strings.Contains(path, "/:gwSn/") {
			if gateway := p.getRouteParam(r, "gwSn"); gateway == "" || !p.ensureGatewayInRequestScope(r, gateway, permission) {
				return
			}
		}
		handler(r)
	}
}

func writeCascadeAccessDenied(r *ghttp.Request) {
	r.Response.WriteStatus(http.StatusForbidden, map[string]interface{}{"code": http.StatusForbidden, "message": "Access denied / 无访问权限"})
}

func cascadeResourceAllowed(r *ghttp.Request, tenantID, projectID uint, permission string) bool {
	principal := core.RequestAuthContext(r)
	if principal == nil || (permission != "" && !principal.HasProjectPermission(permission, projectID)) {
		return false
	}
	if tenant := r.GetCtxVar("tenant_id").Uint(); tenant != 0 && tenant != tenantID {
		return false
	}
	if project := r.GetCtxVar("project_id").Uint(); project != 0 && project != projectID {
		return false
	}
	if principal.IsSystemAdmin {
		return true
	}
	if principal.TenantID != tenantID {
		return false
	}
	if projects, restricted := principal.ProjectIDsForTenantQuery(); restricted {
		for _, id := range projects {
			if id == projectID {
				return true
			}
		}
		return false
	}
	return true
}

func syncConflictVisible(r *ghttp.Request, item syncConflictView) bool {
	switch item.EntityType {
	case store.SyncEntityDevice:
		return item.Local != nil && cascadeResourceAllowed(r, item.Local.TenantID, item.Local.ProjectID, "device:list")
	case store.SyncEntityProduct:
		return item.LocalProd != nil && cascadeResourceAllowed(r, item.LocalProd.TenantID, item.LocalProd.ProjectID, "product:list")
	}
	return false
}

// Reauthorize persisted ownership for every requested decision. Scope headers
// only narrow this boundary, and a bulk request cannot change earlier items
// before discovering an unauthorized item later in the same request.
func syncResolutionAllowed(r *ghttp.Request, item syncResolveRequestItem) bool {
	var tenantID, projectID uint
	var conflictPayload, module string
	switch item.EntityType {
	case store.SyncEntityDevice:
		current, err := store.GetDevice(item.EntityCode)
		if err != nil || current == nil {
			return false
		}
		tenantID, projectID, conflictPayload, module = current.TenantID, current.ProjectID, current.ConflictPayload, "device"
	case store.SyncEntityProduct:
		current, err := store.GetProduct(item.EntityCode)
		if err != nil || current == nil {
			return false
		}
		tenantID, projectID, conflictPayload, module = current.TenantID, current.ProjectID, current.ConflictPayload, "product"
	default:
		return false
	}
	if !cascadeResourceAllowed(r, tenantID, projectID, module+":edit") {
		return false
	}
	if item.Resolution == syncResolveTakePeer {
		record, err := decodeConflictRecord(conflictPayload)
		if err != nil || record == nil {
			return false
		}
		if len(record.Payload) == 0 {
			return cascadeResourceAllowed(r, tenantID, projectID, module+":delete")
		}
		// A peer revision can carry ownership fields too. Accepting it must not
		// move a resource into a tenant or project the caller cannot manage.
		var peer struct {
			TenantID  uint `json:"tenant_id"`
			ProjectID uint `json:"project_id"`
		}
		if json.Unmarshal(record.Payload, &peer) != nil || !cascadeResourceAllowed(r, peer.TenantID, peer.ProjectID, module+":edit") {
			return false
		}
	}
	return true
}

func (p *CascadePlugin) statusPayload(r *ghttp.Request, connected bool, mode, broker, gateway string) map[string]interface{} {
	data := map[string]interface{}{"plugin": "cascade", "mode": mode}
	if principal := core.RequestAuthContext(r); principal != nil && principal.HasPermission("gateway:list") {
		data["status"] = map[bool]string{true: "connected", false: "disconnected"}[connected]
		data["connected"] = connected
		data["broker"] = broker
		data["gateway_code"] = gateway
	}
	return data
}
