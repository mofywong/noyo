package core

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"

	"noyo/core/store"
	"noyo/core/utils"
)

var (
	TokenBlacklistCache sync.Map // key: token hash, value: expiry time (read-through cache)
	loginTracker        = &LoginAttemptTracker{attempts: make(map[string]*AttemptInfo)}
)

type LoginAttemptTracker struct {
	mu       sync.Mutex
	attempts map[string]*AttemptInfo
}

type AttemptInfo struct {
	FailCount   int
	LockedUntil time.Time
}

func (t *LoginAttemptTracker) CheckAndRecord(key string, success bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	info, exists := t.attempts[key]
	if exists && time.Now().Before(info.LockedUntil) {
		return fmt.Errorf("Account locked, try again after %v", info.LockedUntil.Format("15:04:05"))
	}

	if success {
		delete(t.attempts, key)
		return nil
	}

	if !exists {
		info = &AttemptInfo{}
		t.attempts[key] = info
	}
	info.FailCount++
	if info.FailCount >= 5 {
		info.LockedUntil = time.Now().Add(15 * time.Minute)
	}
	return nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func isTokenBlacklisted(token string) bool {
	tokenHash := hashToken(token)
	// Check in-memory cache first
	if exp, ok := TokenBlacklistCache.Load(tokenHash); ok {
		if time.Now().Before(exp.(time.Time)) {
			return true
		}
		TokenBlacklistCache.Delete(tokenHash)
		return false
	}
	// Check database
	var entry store.TokenBlacklist
	if err := store.DB.Where("token_hash = ?", tokenHash).First(&entry).Error; err == nil {
		if time.Now().Before(entry.ExpiresAt) {
			TokenBlacklistCache.Store(tokenHash, entry.ExpiresAt)
			return true
		}
		// Expired, clean up
		store.DB.Delete(&entry)
	}
	return false
}

// blacklistToken persists a token revocation to the database and in-memory cache.
func blacklistToken(tokenString string, expiry time.Time) {
	tokenHash := hashToken(tokenString)
	store.DB.Create(&store.TokenBlacklist{
		TokenHash: tokenHash,
		ExpiresAt: expiry,
	})
	TokenBlacklistCache.Store(tokenHash, expiry)
}

func validatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("Password must be at least 8 characters")
	}
	hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
	hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
	hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
	if !hasUpper || !hasLower || !hasDigit {
		return errors.New("Password must contain uppercase, lowercase and numbers")
	}
	return nil
}

type LoginRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	LoginSuffix string `json:"login_suffix"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AppTokenRequest struct {
	AppID  string `json:"app_id"`
	AppKey string `json:"app_key"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func getEffectiveRole(user *store.User) string {
	authCtx, err := ResolveUserAuthContext(user.ID, 0, 0)
	if err == nil && authCtx != nil {
		return authCtx.Role
	}
	return user.Role
}

func getAllowedScopes(user *store.User) ([]uint, []uint) {
	authCtx, err := ResolveUserAuthContext(user.ID, 0, 0)
	if err != nil || authCtx == nil {
		return []uint{}, []uint{}
	}
	allowedTenants := []uint{}
	if authCtx.TenantID > 0 {
		allowedTenants = append(allowedTenants, authCtx.TenantID)
	}
	return allowedTenants, authCtx.AllowedProjectIDs
}

func permissionCodesFromAuthContext(authCtx *AuthContext) []string {
	if authCtx == nil {
		return []string{}
	}
	permissions := make([]string, 0, len(authCtx.PermissionCodes))
	for code := range authCtx.PermissionCodes {
		permissions = append(permissions, code)
	}
	return permissions
}

func normalizedTokenExpiries(accessExpiry, refreshExpiry int) (int, int) {
	if accessExpiry == 0 {
		accessExpiry = 120
	}
	if refreshExpiry == 0 {
		refreshExpiry = 10080
	}
	return accessExpiry, refreshExpiry
}

func issueAppTokens(app store.App, secret string, accessExpiryMin, refreshExpiryMin int) (string, string, error) {
	accessExpiryMin, refreshExpiryMin = normalizedTokenExpiries(accessExpiryMin, refreshExpiryMin)
	return utils.GenerateAppTokens(
		app.ID,
		app.TenantID,
		app.AppID,
		"app:"+app.Name,
		secret,
		accessExpiryMin,
		refreshExpiryMin,
	)
}

func buildAppTokenResponse(app store.App, authCtx *AuthContext, accessToken, refreshToken string, accessExpiryMin, refreshExpiryMin int) g.Map {
	accessExpiryMin, refreshExpiryMin = normalizedTokenExpiries(accessExpiryMin, refreshExpiryMin)
	return g.Map{
		"access_token":        accessToken,
		"refresh_token":       refreshToken,
		"token_type":          "Bearer",
		"expires_in":          accessExpiryMin * 60,
		"refresh_expires_in":  refreshExpiryMin * 60,
		"app_id":              app.AppID,
		"tenant_id":           app.TenantID,
		"allowed_project_ids": authCtx.AllowedProjectIDs,
		"permissions":         permissionCodesFromAuthContext(authCtx),
	}
}

func buildAuthUserInfo(user *store.User, authCtx *AuthContext, tenantName, tenantLogo string) g.Map {
	effectiveRole := user.Role
	isSystemAdmin := false
	isTenantAdmin := false
	isProjectAdmin := false
	allowedProjectIDs := []uint{}
	permissions := []string{}
	if authCtx != nil {
		effectiveRole = authCtx.Role
		isSystemAdmin = authCtx.IsSystemAdmin
		isTenantAdmin = authCtx.IsTenantAdmin
		isProjectAdmin = authCtx.IsProjectAdmin
		allowedProjectIDs = authCtx.AllowedProjectIDs
		permissions = permissionCodesFromAuthContext(authCtx)
	}

	return g.Map{
		"id":                   user.ID,
		"tenant_id":            user.TenantID,
		"tenant_name":          tenantName,
		"tenant_logo":          tenantLogo,
		"username":             user.Username,
		"display_name":         user.DisplayName,
		"email":                user.Email,
		"role":                 effectiveRole,
		"is_system_admin":      isSystemAdmin,
		"is_tenant_admin":      isTenantAdmin,
		"is_project_admin":     isProjectAdmin,
		"permissions":          permissions,
		"allowed_project_ids":  allowedProjectIDs,
		"must_change_password": user.MustChangePassword,
	}
}

// handleLogin authenticates a user and returns JWT tokens
func (s *Server) handleLogin(r *ghttp.Request) {
	var req LoginRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON payload"})
		return
	}

	// Rate Limiting based on IP and Username
	clientIP := r.GetClientIp()
	trackerKey := fmt.Sprintf("%s:%s", clientIP, req.Username)
	if err := loginTracker.CheckAndRecord(trackerKey, false); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}

	users, err := store.GetUsersByUsername(req.Username)
	if err != nil || len(users) == 0 {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid username or password"})
		return
	}

	var matchedUser *store.User
	for i := range users {
		if utils.CheckPasswordHash(req.Password, users[i].Password) {
			if users[i].Status == 1 {
				matchedUser = &users[i]
				break
			}
		}
	}

	if matchedUser == nil {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid username or password"})
		return
	}

	// Strictly verify login suffix
	if req.LoginSuffix != "" {
		var expectedTenant store.Tenant
		if err := store.DB.Where("login_suffix = ?", req.LoginSuffix).First(&expectedTenant).Error; err == nil {
			if matchedUser.TenantID != expectedTenant.ID {
				r.Response.WriteJson(g.Map{"code": 403, "message": "账户不属于该租户，无法在此页面登录"})
				return
			}
		}
	} else {
		if len(users) > 1 {
			r.Response.WriteJson(g.Map{"code": 403, "message": "该用户名存在于多个租户中，请指定租户后缀"})
			return
		}
	}

	// Login successful, clear attempts
	loginTracker.CheckAndRecord(trackerKey, true)

	// Update last login time
	now := time.Now()
	matchedUser.LastLoginAt = &now
	store.SaveUser(matchedUser)

	// Generate tokens
	accessExpiry := s.Config.Auth.AccessTokenExpiry
	refreshExpiry := s.Config.Auth.RefreshTokenExpiry
	if accessExpiry == 0 {
		accessExpiry = 120
	}
	if refreshExpiry == 0 {
		refreshExpiry = 10080
	}

	authCtx, err := ResolveUserAuthContext(matchedUser.ID, 0, 0)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}
	effectiveRole := authCtx.Role
	allowedTenants, allowedProjects := getAllowedScopes(matchedUser)

	accessToken, refreshToken, err := utils.GenerateTokens(
		matchedUser.ID,
		matchedUser.TenantID,
		matchedUser.Username,
		effectiveRole,
		allowedTenants,
		allowedProjects,
		s.Config.Auth.JWTSecret,
		accessExpiry,
		refreshExpiry,
	)

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate tokens"})
		return
	}

	var tenantName string
	var tenantLogo string
	if matchedUser.TenantID > 0 {
		var tenant store.Tenant
		if err := store.DB.First(&tenant, matchedUser.TenantID).Error; err == nil {
			tenantName = tenant.Name
			tenantLogo = tenant.Logo
		}
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"expires_in":    accessExpiry * 60,
			"user_info":     buildAuthUserInfo(matchedUser, authCtx, tenantName, tenantLogo),
		},
	})
}

func (s *Server) handleGetTenantBySuffix(r *ghttp.Request) {
	suffix := r.Get("suffix").String()
	if suffix == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Suffix is required"})
		return
	}

	var tenant store.Tenant
	if err := store.DB.Where("login_suffix = ?", suffix).First(&tenant).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Tenant not found"})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"name": tenant.Name,
			"logo": tenant.Logo,
		},
	})
}

// handleIssueAppToken exchanges AppID/AppKey for short-lived app bearer tokens.
func (s *Server) handleIssueAppToken(r *ghttp.Request) {
	var req AppTokenRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON payload"})
		return
	}
	req.AppID = strings.TrimSpace(req.AppID)
	if req.AppID == "" || req.AppKey == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "AppID and AppKey are required"})
		return
	}

	appTrackerKey := fmt.Sprintf("app:%s", req.AppID)
	if err := loginTracker.CheckAndRecord(appTrackerKey, false); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}

	var app store.App
	if err := store.DB.Where("app_id = ? AND status = ?", req.AppID, 1).First(&app).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid App credentials"})
		return
	}
	if !utils.CheckPasswordHash(req.AppKey, app.AppKey) {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid App credentials"})
		return
	}
	loginTracker.CheckAndRecord(appTrackerKey, true)

	authCtx, err := ResolveAppAuthContext(app, 0)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}

	accessExpiry, refreshExpiry := normalizedTokenExpiries(s.Config.Auth.AccessTokenExpiry, s.Config.Auth.RefreshTokenExpiry)
	accessToken, refreshToken, err := issueAppTokens(app, s.Config.Auth.JWTSecret, accessExpiry, refreshExpiry)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate app tokens"})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": buildAppTokenResponse(app, authCtx, accessToken, refreshToken, accessExpiry, refreshExpiry),
	})
}

// handleRefreshAppToken rotates app bearer tokens using an app refresh token.
func (s *Server) handleRefreshAppToken(r *ghttp.Request) {
	var req RefreshRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON payload"})
		return
	}

	claims, err := utils.ParseToken(req.RefreshToken, s.Config.Auth.JWTSecret)
	if err != nil || claims.SubjectType != "app" || claims.TokenUse != "refresh" || claims.AppDBID == 0 || claims.AppID == "" {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid or expired app refresh token"})
		return
	}

	var app store.App
	if err := store.DB.Where("id = ? AND app_id = ? AND status = ?", claims.AppDBID, claims.AppID, 1).First(&app).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 401, "message": "App no longer active"})
		return
	}

	authCtx, err := ResolveAppAuthContext(app, 0)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}

	accessExpiry, refreshExpiry := normalizedTokenExpiries(s.Config.Auth.AccessTokenExpiry, s.Config.Auth.RefreshTokenExpiry)
	accessToken, refreshToken, err := issueAppTokens(app, s.Config.Auth.JWTSecret, accessExpiry, refreshExpiry)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate app tokens"})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": buildAppTokenResponse(app, authCtx, accessToken, refreshToken, accessExpiry, refreshExpiry),
	})
}

// handleRefreshToken issues a new access token given a valid refresh token
func (s *Server) handleRefreshToken(r *ghttp.Request) {
	var req RefreshRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON payload"})
		return
	}

	claims, err := utils.ParseToken(req.RefreshToken, s.Config.Auth.JWTSecret)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid or expired refresh token"})
		return
	}
	if claims.SubjectType == "app" {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Invalid or expired refresh token"})
		return
	}

	// Make sure the user still exists and is active
	var user *store.User
	userID, parseErr := strconv.ParseUint(claims.Subject, 10, 64)
	if parseErr == nil {
		user, err = store.GetUserByID(uint(userID))
	} else {
		// Fallback to username lookup for backward compatibility
		user, err = store.GetUserByUsername(claims.Subject)
	}

	if err != nil || user == nil || user.Status != 1 {
		r.Response.WriteJson(g.Map{"code": 401, "message": "User no longer active"})
		return
	}

	accessExpiry := s.Config.Auth.AccessTokenExpiry
	if accessExpiry == 0 {
		accessExpiry = 120
	}

	authCtx, err := ResolveUserAuthContext(user.ID, 0, 0)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}
	effectiveRole := authCtx.Role
	allowedTenants, allowedProjects := getAllowedScopes(user)

	// Generate only new access token (or could generate both, but we'll stick to rotating access)
	accessToken, newRefreshToken, err := utils.GenerateTokens(
		user.ID,
		user.TenantID,
		user.Username,
		effectiveRole,
		allowedTenants,
		allowedProjects,
		s.Config.Auth.JWTSecret,
		accessExpiry,
		s.Config.Auth.RefreshTokenExpiry,
	)

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate access token"})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"access_token":  accessToken,
			"refresh_token": newRefreshToken,
			"expires_in":    accessExpiry * 60,
		},
	})
}

// handleGetProfile returns the current logged-in user's profile
func (s *Server) handleGetProfile(r *ghttp.Request) {
	userIDVal := r.GetCtxVar("user_id")
	if userIDVal == nil {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Unauthorized"})
		return
	}
	userID := userIDVal.Uint()

	user, err := store.GetUserByID(userID)
	if err != nil || user == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	var tenantName string
	if user.TenantID > 0 {
		var tenant store.Tenant
		if err := store.DB.First(&tenant, user.TenantID).Error; err == nil {
			tenantName = tenant.Name
		}
	}

	userInfo := buildAuthUserInfo(user, authCtx, tenantName, "")
	userInfo["status"] = user.Status

	r.Response.WriteJson(g.Map{"code": 0, "data": userInfo})
}

// handleChangePassword allows users to change their own password
func (s *Server) handleChangePassword(r *ghttp.Request) {
	userIDVal := r.GetCtxVar("user_id")
	if userIDVal == nil {
		r.Response.WriteJson(g.Map{"code": 401, "message": "Unauthorized"})
		return
	}
	userID := userIDVal.Uint()

	var req ChangePasswordRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	user, err := store.GetUserByID(userID)
	if err != nil || user == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}

	if !utils.CheckPasswordHash(req.OldPassword, user.Password) {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Incorrect old password"})
		return
	}

	if err := validatePasswordStrength(req.NewPassword); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}

	newHash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to hash new password"})
		return
	}

	user.Password = newHash
	user.MustChangePassword = false
	if err := store.SaveUser(user); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update password"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Password updated successfully"})
}

func (s *Server) handleLogout(r *ghttp.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
			tokenString := parts[1]
			claims, _ := utils.ParseToken(tokenString, s.Config.Auth.JWTSecret)
			if claims != nil {
				blacklistToken(tokenString, claims.ExpiresAt.Time)
			}
		}
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Logged out successfully"})
}
