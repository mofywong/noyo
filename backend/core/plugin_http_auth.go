package core

import (
	"net/http"
	"path"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"
)

// Plugin routes bypass registerAPIRoutes, so authentication alone is not enough.
// Cascade and AI Brain perform their resource-specific checks in their handlers.
func pluginHTTPPermissions(route, method string) ([]string, []string) {
	route = path.Clean(route)
	switch {
	case strings.HasPrefix(route, "/api/extension/cascade/") || strings.HasPrefix(route, "/api/plugins/ai_brain/"):
		return nil, nil
	case strings.HasPrefix(route, "/api/plugins/yolo_pro/"):
		return []string{"ai_scene:list", "semantic_review:list", "plugin:list"}, []string{http.MethodGet}
	case strings.HasPrefix(route, "/api/plugins/ai_predict/"):
		return []string{"plugin:config"}, []string{http.MethodGet, http.MethodPost}
	case route == "/api/extension/ai/config":
		return []string{"plugin:config"}, []string{http.MethodGet}
	case route == "/api/extension/ai/providers":
		return []string{"ai_assistant:use", "plugin:config"}, []string{http.MethodGet}
	case route == "/api/extension/ai/sessions":
		return []string{"ai_assistant:use"}, []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}
	case strings.HasPrefix(route, "/api/extension/ai/sessions/"):
		return []string{"ai_assistant:use"}, []string{http.MethodGet, http.MethodPut, http.MethodDelete}
	case strings.HasPrefix(route, "/api/extension/ai/"):
		return []string{"ai_assistant:use"}, []string{http.MethodPost}
	case route == "/api/extension/voice/enable" || route == "/api/extension/voice/disable" || route == "/api/extension/voice/test/tts":
		return []string{"plugin:config"}, []string{http.MethodPost}
	case route == "/api/extension/voice/config" || route == "/api/extension/voice/devices":
		return []string{"plugin:config"}, []string{http.MethodGet, http.MethodPost}
	case route == "/api/extension/license/status":
		return []string{"system:license"}, []string{http.MethodGet}
	case route == "/api/extension/license/upload":
		return []string{"system:config"}, []string{http.MethodPost}
	case route == "/api/system/media-network":
		return []string{"system:config"}, []string{http.MethodGet, http.MethodPut}
	case strings.HasPrefix(route, "/api/extension/gb28181/"):
		return []string{"plugin:config"}, []string{http.MethodGet}
	case route == "/api/plugins/script/templates":
		return []string{"product:list"}, []string{http.MethodGet}
	case route == "/api/plugins/script/debug/attach":
		return []string{"device:control"}, []string{http.MethodGet}
	case route == "/api/plugins/script/debug/eval":
		return []string{"device:control"}, []string{http.MethodPost}
	case route == "/api/extension/mqtt_api/status":
		return []string{"plugin:list", "gateway:list"}, []string{http.MethodGet}
	default:
		if method == http.MethodGet || method == http.MethodHead {
			return []string{"plugin:list"}, nil
		}
		return []string{"plugin:config"}, nil
	}
}

func (s *Server) authorizePluginHTTP(r *ghttp.Request, auth *AuthContext) bool {
	if auth == nil {
		r.Response.WriteStatus(http.StatusUnauthorized, map[string]interface{}{"code": 401, "message": "Unauthorized"})
		return false
	}
	permissions, methods := pluginHTTPPermissions(r.URL.Path, r.Method)
	if len(methods) > 0 {
		allowed := false
		for _, method := range methods {
			allowed = allowed || r.Method == method
		}
		if !allowed {
			r.Response.Header().Set("Allow", strings.Join(methods, ", "))
			r.Response.WriteStatus(http.StatusMethodNotAllowed, map[string]interface{}{"code": 405, "message": "Method Not Allowed"})
			return false
		}
	}
	if len(permissions) == 0 {
		return true
	}
	for _, permission := range permissions {
		if !auth.HasPermission(permission) {
			continue
		}
		r.SetCtxVar("required_permission", permission)
		if strings.HasPrefix(path.Clean(r.URL.Path), "/api/plugins/script/debug/") {
			code := r.Get("device_code").String()
			if !canAccessDeviceCode(r, code) || s.checkDeviceTagPermission(r, code) != nil {
				r.Response.WriteStatus(http.StatusForbidden, map[string]interface{}{"code": 403, "message": "Device access denied"})
				return false
			}
		}
		return true
	}
	r.Response.WriteStatus(http.StatusForbidden, map[string]interface{}{"code": 403, "message": "Permission denied: " + strings.Join(permissions, " / ")})
	return false
}
