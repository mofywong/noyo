package core

import (
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/net/ghttp"

	"noyo/core/store"
	"noyo/core/utils"
)

// AuthMiddleware creates a middleware for JWT authentication
func AuthMiddleware(secret string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		// 1. Try App Authentication First
		appID := r.Header.Get("X-App-ID")
		appKey := r.Header.Get("X-App-Key")
		if appID != "" && appKey != "" {
			var app store.App
			if err := store.DB.Where("app_id = ? AND app_key = ?", appID, appKey).First(&app).Error; err == nil && app.Status == 1 {
				headerProjectID, _ := strconv.ParseUint(r.Header.Get("X-Current-Project-ID"), 10, 64)
				authCtx, err := ResolveAppAuthContext(app, uint(headerProjectID))
				if err != nil {
					r.Response.WriteJson(map[string]interface{}{
						"code":    403,
						"message": err.Error(),
					})
					return
				}
				r.SetCtxVar(authContextKey, authCtx)
				r.SetCtxVar("user_id", uint(0))
				r.SetCtxVar("tenant_id", authCtx.TenantID)
				r.SetCtxVar("project_id", authCtx.ProjectID)
				r.SetCtxVar("username", authCtx.Username)
				r.SetCtxVar("app_id", app.AppID)
				r.SetCtxVar("role", authCtx.Role)
				r.Middleware.Next()
				return
			} else {
				r.Response.WriteJson(map[string]interface{}{
					"code":    401,
					"message": "Invalid App Credentials",
				})
				return
			}
		}

		// 2. Try JWT Authentication
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			r.Response.WriteJson(map[string]interface{}{
				"code":    401,
				"message": "Missing Authorization header or App Credentials",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			r.Response.WriteJson(map[string]interface{}{
				"code":    401,
				"message": "Invalid Authorization header format",
			})
			return
		}

		tokenString := parts[1]
		claims, err := utils.ParseToken(tokenString, secret)
		if err != nil {
			r.Response.WriteJson(map[string]interface{}{
				"code":    401,
				"message": "Invalid or expired token",
			})
			return
		}

		if isTokenBlacklisted(tokenString) {
			r.Response.WriteJson(map[string]interface{}{
				"code":    401,
				"message": "Token has been revoked",
			})
			return
		}

		// Extract current context from headers
		headerTenantID, _ := strconv.ParseUint(r.Header.Get("X-Current-Tenant-ID"), 10, 64)
		headerProjectID, _ := strconv.ParseUint(r.Header.Get("X-Current-Project-ID"), 10, 64)

		currentTenantID := uint(headerTenantID)
		currentProjectID := uint(headerProjectID)

		authCtx, err := ResolveUserAuthContext(claims.UserID, currentTenantID, currentProjectID)
		if err != nil {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": err.Error(),
			})
			return
		}

		// Inject user info into context
		r.SetCtxVar(authContextKey, authCtx)
		r.SetCtxVar("user_id", authCtx.UserID)
		r.SetCtxVar("tenant_id", authCtx.TenantID)
		r.SetCtxVar("project_id", authCtx.ProjectID)
		r.SetCtxVar("username", authCtx.Username)
		r.SetCtxVar("role", authCtx.Role)
		r.SetCtxVar("app_id", "")

		r.Middleware.Next()
	}
}

// AuditMiddleware records actions for specific methods (POST/PUT/DELETE)
func AuditMiddleware() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		r.Middleware.Next()

		method := r.Method
		if method == "POST" || method == "PUT" || method == "DELETE" {
			tenantID := r.GetCtxVar("tenant_id").Uint()
			userID := r.GetCtxVar("user_id").Uint()
			username := r.GetCtxVar("username").String()
			appID := r.GetCtxVar("app_id").String()

			action := "UNKNOWN"
			switch method {
			case "POST":
				action = "CREATE"
			case "PUT":
				action = "UPDATE"
			case "DELETE":
				action = "DELETE"
			}

			log := store.AuditLog{
				TenantID:  tenantID,
				UserID:    userID,
				Username:  username,
				AppID:     appID,
				Module:    r.Router.Uri,
				Action:    action,
				Resource:  r.URL.Path,
				Detail:    "Body: " + string(r.GetBody()),
				IP:        r.GetClientIp(),
				UserAgent: r.Header.Get("User-Agent"),
			}
			store.DB.Create(&log)
		}
	}
}

// RoleMiddleware creates a middleware that restricts access to certain roles
func RoleMiddleware(allowedRoles ...string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		authCtx := requestAuthContext(r)
		if authCtx == nil {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Role not found in context",
			})
			return
		}

		if !authCtx.IsRoleAllowed(allowedRoles...) {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Access denied: insufficient role permissions",
			})
			return
		}

		r.Middleware.Next()
	}
}

// PermissionMiddleware checks if the user has a specific permission
func PermissionMiddleware(permissionCode string) func(*ghttp.Request) {
	return func(r *ghttp.Request) {
		authCtx := requestAuthContext(r)
		if authCtx == nil {
			r.Response.WriteJson(map[string]interface{}{
				"code":    401,
				"message": "Unauthorized",
			})
			return
		}
		if !authCtx.HasPermission(permissionCode) {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Permission denied: " + permissionCode,
			})
			return
		}

		r.Middleware.Next()
	}
}

// SystemAdminMiddleware ensures the user is a system admin (TenantID == 0 and Role == "admin")
func SystemAdminMiddleware() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		authCtx := requestAuthContext(r)
		if authCtx == nil || !authCtx.IsSystemAdmin {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Access denied: System Admin only",
			})
			return
		}
		r.Middleware.Next()
	}
}

func requestAuthContext(r *ghttp.Request) *AuthContext {
	v := r.GetCtxVar(authContextKey)
	if v == nil {
		return nil
	}
	if ctx, ok := v.Interface().(*AuthContext); ok {
		return ctx
	}
	return nil
}

// TenantMiddleware ensures tenant-scoped users have a tenant context while
// preserving the system-admin global context.
func TenantMiddleware() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		if authCtx := requestAuthContext(r); authCtx != nil && authCtx.IsSystemAdmin {
			r.Middleware.Next()
			return
		}

		tenantID := r.GetCtxVar("tenant_id").Uint()
		if tenantID == 0 {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Access denied: Tenant context required",
			})
			return
		}
		r.Middleware.Next()
	}
}
