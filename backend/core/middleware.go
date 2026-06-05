package core

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"

	"noyo/core/store"
	"noyo/core/utils"
)

type appRateBucket struct {
	WindowStart time.Time
	Count       int
}

var appRateLimiter = struct {
	sync.Mutex
	Buckets map[string]appRateBucket
}{Buckets: make(map[string]appRateBucket)}

func allowAppRequest(appID string, limit int) bool {
	if limit <= 0 || appID == "" {
		return true
	}
	now := time.Now()
	appRateLimiter.Lock()
	defer appRateLimiter.Unlock()

	bucket := appRateLimiter.Buckets[appID]
	if bucket.WindowStart.IsZero() || now.Sub(bucket.WindowStart) >= time.Second {
		bucket = appRateBucket{WindowStart: now, Count: 0}
	}
	if bucket.Count >= limit {
		appRateLimiter.Buckets[appID] = bucket
		return false
	}
	bucket.Count++
	appRateLimiter.Buckets[appID] = bucket
	return true
}

func resetAppRateLimiterForTest() {
	appRateLimiter.Lock()
	defer appRateLimiter.Unlock()
	appRateLimiter.Buckets = make(map[string]appRateBucket)
}

// authenticateRequest performs JWT/App authentication and sets context vars.
// Returns *AuthContext on success, nil on failure (response already written).
func authenticateRequest(r *ghttp.Request, secret string) *AuthContext {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		r.Response.WriteJson(map[string]interface{}{
			"code":    401,
			"message": "Missing Authorization header",
		})
		return nil
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		r.Response.WriteJson(map[string]interface{}{
			"code":    401,
			"message": "Invalid Authorization header format",
		})
		return nil
	}

	tokenString := parts[1]
	claims, err := utils.ParseToken(tokenString, secret)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{
			"code":    401,
			"message": "Invalid or expired token",
		})
		return nil
	}

	if isTokenBlacklisted(tokenString) {
		r.Response.WriteJson(map[string]interface{}{
			"code":    401,
			"message": "Token has been revoked",
		})
		return nil
	}

	// Extract current context from headers
	headerTenantID, _ := strconv.ParseUint(r.Header.Get("X-Current-Tenant-ID"), 10, 64)
	headerProjectID, _ := strconv.ParseUint(r.Header.Get("X-Current-Project-ID"), 10, 64)

	currentTenantID := uint(headerTenantID)
	currentProjectID := uint(headerProjectID)

	if claims.SubjectType == "app" {
		authCtx, err := resolveAppAuthContextFromClaims(claims, currentProjectID)
		if err != nil {
			r.Response.WriteJson(map[string]interface{}{
				"code":    401,
				"message": err.Error(),
			})
			return nil
		}
		if !allowAppRequest(authCtx.AppID, authCtx.AppRateLimit) {
			r.Response.WriteStatusExit(429, map[string]interface{}{
				"code":    429,
				"message": "Rate limit exceeded",
			})
			return nil
		}
		r.SetCtxVar(authContextKey, authCtx)
		r.SetCtxVar("user_id", uint(0))
		r.SetCtxVar("tenant_id", authCtx.TenantID)
		r.SetCtxVar("project_id", authCtx.ProjectID)
		r.SetCtxVar("username", authCtx.Username)
		r.SetCtxVar("role", authCtx.Role)
		r.SetCtxVar("app_id", authCtx.AppID)
		return authCtx
	}

	if claims.TokenUse == "refresh" {
		r.Response.WriteJson(map[string]interface{}{
			"code":    401,
			"message": "Refresh token cannot be used for API access",
		})
		return nil
	}

	// M5: Validate requested tenant against AllowedTenants from JWT claims
	// Empty AllowedTenants means system admin with unrestricted access
	if currentTenantID > 0 && claims.TenantID == 0 && len(claims.AllowedTenants) > 0 {
		allowed := false
		for _, t := range claims.AllowedTenants {
			if t == currentTenantID {
				allowed = true
				break
			}
		}
		if !allowed {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Tenant is outside allowed scope",
			})
			return nil
		}
	}

	authCtx, err := ResolveUserAuthContext(claims.UserID, currentTenantID, currentProjectID)
	if err != nil {
		r.Response.WriteJson(map[string]interface{}{
			"code":    403,
			"message": err.Error(),
		})
		return nil
	}

	// Inject user info into context
	r.SetCtxVar(authContextKey, authCtx)
	r.SetCtxVar("user_id", authCtx.UserID)
	r.SetCtxVar("tenant_id", authCtx.TenantID)
	r.SetCtxVar("project_id", authCtx.ProjectID)
	r.SetCtxVar("username", authCtx.Username)
	r.SetCtxVar("role", authCtx.Role)
	r.SetCtxVar("app_id", "")

	// M2: Enforce MustChangePassword — only allow password change endpoint
	if authCtx.MustChangePassword && r.URL.Path != "/api/auth/password" {
		r.Response.WriteJson(map[string]interface{}{
			"code":    403,
			"message": "Password change required",
			"data":    map[string]interface{}{"must_change_password": true},
		})
		return nil
	}

	return authCtx
}

// logAuditRecord writes an audit log entry for mutating requests.
func logAuditRecord(r *ghttp.Request) {
	method := r.Method
	if method != "POST" && method != "PUT" && method != "DELETE" {
		return
	}

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
		Detail:    "Body: " + redactSensitiveBody(r.GetBody(), r.URL.Path),
		IP:        r.GetClientIp(),
		UserAgent: r.Header.Get("User-Agent"),
	}
	store.DB.Create(&log)
}

// redactSensitiveBody removes sensitive fields from request body before logging.
func redactSensitiveBody(body []byte, uri string) string {
	if len(body) == 0 {
		return ""
	}
	// For auth endpoints, redact entirely
	if strings.Contains(uri, "/auth/") || strings.Contains(uri, "/password") {
		return "[REDACTED]"
	}
	// For other endpoints, redact known sensitive fields
	var m map[string]interface{}
	if err := json.Unmarshal(body, &m); err != nil {
		return "[UNPARSEABLE]"
	}
	sensitiveKeys := []string{"password", "app_key", "app_secret", "token", "secret", "old_password", "new_password"}
	for _, key := range sensitiveKeys {
		if _, ok := m[key]; ok {
			m[key] = "[REDACTED]"
		}
	}
	redacted, _ := json.Marshal(m)
	return string(redacted)
}

// AuthMiddleware creates a middleware for JWT authentication
func AuthMiddleware(secret string) ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		authCtx := authenticateRequest(r, secret)
		if authCtx == nil {
			return // response already written
		}
		r.Middleware.Next()
	}
}

// AuditMiddleware records actions for specific methods (POST/PUT/DELETE)
func AuditMiddleware() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		r.Middleware.Next()
		logAuditRecord(r)
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
		r.SetCtxVar("required_permission", permissionCode)
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

// TenantMiddleware ensures tenant-scoped APIs are never reached without a
// concrete tenant context. System admins manage tenants through dedicated
// system routes and do not receive tenant business-data scope.
func TenantMiddleware() ghttp.HandlerFunc {
	return func(r *ghttp.Request) {
		authCtx := requestAuthContext(r)
		if authCtx == nil || authCtx.IsSystemAdmin {
			r.Response.WriteJson(map[string]interface{}{
				"code":    403,
				"message": "Access denied: Tenant context required",
			})
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
