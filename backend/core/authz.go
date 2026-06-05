package core

import (
	"fmt"
	"sort"

	"noyo/core/store"
	"noyo/core/utils"
)

const authContextKey = "auth_context"

const (
	RoleCodeSuperAdmin   = "super_admin"
	RoleCodeTenantAdmin  = "tenant_admin"
	RoleCodeProjectAdmin = "project_admin"
	RoleCodeViewer       = "viewer"
)

type AuthContext struct {
	SubjectType        string
	UserID             uint
	AppID              string
	AppDBID            uint
	Username           string
	TenantID           uint
	ProjectID          uint
	Role               string
	IsSystemAdmin      bool
	IsTenantAdmin      bool
	IsProjectAdmin     bool
	RoleIDs            []uint
	RoleProjectIDs     map[uint]map[uint]bool
	PermissionCodes    map[string]bool
	ProjectPermissions map[uint]map[string]bool
	DeviceTagAccess    map[uint]map[uint]string
	AllowedProjectIDs  []uint
	UsesProjectLimits  bool
	MustChangePassword bool
	AppRateLimit       int
}

func (ctx *AuthContext) HasPermission(code string) bool {
	if ctx == nil {
		return false
	}
	if ctx.SubjectType == "app" {
		if ctx.ProjectID > 0 {
			return ctx.HasProjectPermission(code, ctx.ProjectID)
		}
		for _, permissions := range ctx.ProjectPermissions {
			if permissions[code] {
				return true
			}
		}
		return false
	}
	return ctx.PermissionCodes[code]
}

func (ctx *AuthContext) HasProjectPermission(code string, projectID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.SubjectType != "app" {
		return ctx.HasPermission(code)
	}
	return projectID > 0 && ctx.ProjectPermissions[projectID] != nil && ctx.ProjectPermissions[projectID][code]
}

func (ctx *AuthContext) AppTagPermission(projectID, tagID uint) string {
	if ctx == nil || ctx.SubjectType != "app" || projectID == 0 || tagID == 0 {
		return ""
	}
	if ctx.DeviceTagAccess[projectID] == nil {
		return ""
	}
	return ctx.DeviceTagAccess[projectID][tagID]
}

func (ctx *AuthContext) HasAppTagPermission(projectID, tagID uint, required string) bool {
	permission := ctx.AppTagPermission(projectID, tagID)
	if permission == "" {
		return false
	}
	if required == "read" {
		return permission == "read" || permission == "write"
	}
	return permission == "write"
}

func (ctx *AuthContext) CanAccessProject(projectID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin {
		return false
	}
	if projectID == 0 {
		return ctx.IsTenantAdmin
	}
	for _, id := range ctx.AllowedProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

func (ctx *AuthContext) ProjectIDsForTenantQuery() ([]uint, bool) {
	if ctx == nil || ctx.IsSystemAdmin {
		return []uint{}, true
	}
	if ctx.IsTenantAdmin {
		if ctx.ProjectID > 0 {
			return []uint{ctx.ProjectID}, true
		}
		return nil, false
	}
	if ctx.ProjectID > 0 {
		return []uint{ctx.ProjectID}, true
	}
	return ctx.AllowedProjectIDs, true
}

func (ctx *AuthContext) IsRoleAllowed(allowedRoles ...string) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin {
		return true
	}
	for _, role := range allowedRoles {
		if ctx.Role == role {
			return true
		}
		if role == RoleCodeTenantAdmin && ctx.IsTenantAdmin {
			return true
		}
		if role == RoleCodeProjectAdmin && ctx.IsProjectAdmin {
			return true
		}
	}
	return false
}

func (ctx *AuthContext) CanManageTenantResource(tenantID uint) bool {
	if ctx == nil {
		return false
	}
	return tenantID > 0 && tenantID == ctx.TenantID && ctx.IsTenantAdmin
}

func (ctx *AuthContext) CanUseTenantScopedResource(tenantID uint) bool {
	if ctx == nil {
		return false
	}
	if tenantID == 0 || tenantID != ctx.TenantID {
		return false
	}
	return ctx.IsTenantAdmin || len(ctx.AllowedProjectIDs) > 0
}

func (ctx *AuthContext) CanManageProject(projectID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsTenantAdmin {
		return true
	}
	return projectID > 0 && ctx.CanAccessProject(projectID)
}

func (ctx *AuthContext) CanManageRole(role store.Role) bool {
	if ctx == nil {
		return false
	}
	if role.IsBuiltin {
		return false
	}
	if role.TenantID != ctx.TenantID {
		return false
	}
	if ctx.IsTenantAdmin {
		return true
	}
	return role.ProjectID > 0 && ctx.CanAccessProject(role.ProjectID)
}

func (ctx *AuthContext) CanViewRole(role store.Role) bool {
	if ctx == nil {
		return false
	}
	if role.TenantID != 0 && role.TenantID != ctx.TenantID {
		return false
	}
	if ctx.IsTenantAdmin {
		return role.TenantID == ctx.TenantID
	}
	if role.ProjectID > 0 {
		return ctx.CanAccessProject(role.ProjectID)
	}
	return role.IsInherited && len(ctx.AllowedProjectIDs) > 0
}

func (ctx *AuthContext) CanAssignRole(role store.Role, targetProjectID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin || role.Code == RoleCodeSuperAdmin {
		return false
	}
	if role.Code == RoleCodeTenantAdmin && !ctx.IsTenantAdmin {
		return false
	}
	if role.TenantID != 0 && role.TenantID != ctx.TenantID {
		return false
	}
	if targetProjectID == 0 {
		return ctx.IsTenantAdmin && role.ProjectID == 0 && !role.IsInherited && role.Code != RoleCodeProjectAdmin
	}
	if !ctx.CanAccessProject(targetProjectID) {
		return false
	}
	if role.IsBuiltin && role.Code == RoleCodeProjectAdmin {
		return ctx.CanManageProject(targetProjectID)
	}
	if ctx.IsTenantAdmin {
		return role.Code != RoleCodeTenantAdmin && (role.ProjectID == 0 || role.ProjectID == targetProjectID)
	}
	if role.IsBuiltin && role.Code != RoleCodeProjectAdmin {
		return false
	}
	return role.ProjectID == targetProjectID || (role.ProjectID == 0 && role.IsInherited)
}

func ResolveUserAuthContext(userID, requestedTenantID, requestedProjectID uint) (*AuthContext, error) {
	user, err := store.GetUserByID(userID)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if user.Status != 1 {
		return nil, fmt.Errorf("user disabled")
	}

	tenantID := user.TenantID
	projectID := requestedProjectID
	if user.TenantID == 0 {
		// System users stay in global scope; stale tenant/project headers must
		// not block global account flows such as required password changes.
		tenantID = 0
		projectID = 0
	} else if requestedTenantID > 0 && requestedTenantID != user.TenantID {
		return nil, fmt.Errorf("tenant is outside allowed scope")
	}

	ctx := &AuthContext{
		SubjectType:        "user",
		UserID:             user.ID,
		Username:           user.Username,
		TenantID:           tenantID,
		ProjectID:          projectID,
		Role:               user.Role,
		PermissionCodes:    make(map[string]bool),
		MustChangePassword: user.MustChangePassword,
	}

	roleBindings, err := resolveUserRoleBindings(user.ID, tenantID)
	if err != nil {
		return nil, err
	}
	applyRoleBindings(ctx, roleBindings, user.Role)

	if projectID > 0 && !ctx.CanAccessProject(projectID) {
		return nil, fmt.Errorf("project is outside allowed scope")
	}
	loadPermissions(ctx)
	return ctx, nil
}

func ResolveAppAuthContext(app store.App, requestedProjectID uint) (*AuthContext, error) {
	ctx := &AuthContext{
		SubjectType:        "app",
		AppID:              app.AppID,
		AppDBID:            app.ID,
		Username:           "app:" + app.Name,
		TenantID:           app.TenantID,
		ProjectID:          requestedProjectID,
		Role:               "app",
		PermissionCodes:    make(map[string]bool),
		ProjectPermissions: make(map[uint]map[string]bool),
		DeviceTagAccess:    make(map[uint]map[uint]string),
		UsesProjectLimits:  true,
		AppRateLimit:       app.RateLimit,
	}

	if app.Status != 1 {
		return nil, fmt.Errorf("app disabled")
	}

	var projectAccess []store.AppProjectAccess
	if err := store.DB.Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID).Find(&projectAccess).Error; err != nil {
		return nil, err
	}
	allowedProjectSet := make(map[uint]bool, len(projectAccess))
	for _, access := range projectAccess {
		if access.ProjectID == 0 {
			continue
		}
		var count int64
		if err := store.DB.Model(&store.Project{}).Where("id = ? AND tenant_id = ?", access.ProjectID, app.TenantID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			continue
		}
		allowedProjectSet[access.ProjectID] = true
		ctx.AllowedProjectIDs = append(ctx.AllowedProjectIDs, access.ProjectID)
		ctx.ProjectPermissions[access.ProjectID] = make(map[string]bool)
	}
	if requestedProjectID > 0 && !ctx.CanAccessProject(requestedProjectID) {
		return nil, fmt.Errorf("project is outside app allowed scope")
	}

	var appPermissions []store.AppPermission
	if err := store.DB.Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID).Find(&appPermissions).Error; err != nil {
		return nil, err
	}
	tenantLimit := permissionLimitCodeSet(permissionLimitScopeTenant, app.TenantID, 0)
	for _, appPermission := range appPermissions {
		if !allowedProjectSet[appPermission.ProjectID] {
			continue
		}
		var permission store.Permission
		if err := store.DB.First(&permission, appPermission.PermissionID).Error; err != nil {
			continue
		}
		projectLimit := permissionLimitCodeSet(permissionLimitScopeProject, app.TenantID, appPermission.ProjectID)
		if !tenantLimit[permission.Code] || !projectLimit[permission.Code] {
			continue
		}
		ctx.ProjectPermissions[appPermission.ProjectID][permission.Code] = true
		ctx.PermissionCodes[permission.Code] = true
	}

	var tagPermissions []store.AppDeviceTagPermission
	if err := store.DB.Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID).Find(&tagPermissions).Error; err != nil {
		return nil, err
	}
	for _, tagPermission := range tagPermissions {
		if !allowedProjectSet[tagPermission.ProjectID] || tagPermission.TagID == 0 {
			continue
		}
		if tagPermission.Permission != "read" && tagPermission.Permission != "write" {
			continue
		}
		if ctx.DeviceTagAccess[tagPermission.ProjectID] == nil {
			ctx.DeviceTagAccess[tagPermission.ProjectID] = make(map[uint]string)
		}
		ctx.DeviceTagAccess[tagPermission.ProjectID][tagPermission.TagID] = tagPermission.Permission
	}

	return ctx, nil
}

func resolveAppAuthContextFromClaims(claims *utils.JWTClaims, requestedProjectID uint) (*AuthContext, error) {
	if claims == nil || claims.SubjectType != "app" || claims.TokenUse != "access" || claims.AppDBID == 0 || claims.AppID == "" {
		return nil, fmt.Errorf("invalid app access token")
	}

	var app store.App
	if err := store.DB.Where("id = ? AND app_id = ?", claims.AppDBID, claims.AppID).First(&app).Error; err != nil {
		return nil, fmt.Errorf("invalid app access token")
	}
	return ResolveAppAuthContext(app, requestedProjectID)
}

type resolvedRoleBinding struct {
	Role      store.Role
	ProjectID uint
}

func resolveUserRoleBindings(userID, tenantID uint) ([]resolvedRoleBinding, error) {
	var bindings []store.UserRoleBinding
	if err := store.DB.Where("user_id = ? AND (tenant_id = ? OR tenant_id = 0)", userID, tenantID).Find(&bindings).Error; err != nil {
		return nil, err
	}

	resolved := make([]resolvedRoleBinding, 0, len(bindings))
	for _, binding := range bindings {
		var role store.Role
		err := store.DB.Where("id = ? AND (tenant_id = ? OR tenant_id = 0)", binding.RoleID, tenantID).First(&role).Error
		if err == nil {
			resolved = append(resolved, resolvedRoleBinding{Role: role, ProjectID: binding.ProjectID})
		}
	}

	return resolved, nil
}

func applyRoleBindings(ctx *AuthContext, bindings []resolvedRoleBinding, legacyRole string) {
	roleIDMap := make(map[uint]bool)
	projectIDMap := make(map[uint]bool)
	if ctx.RoleProjectIDs == nil {
		ctx.RoleProjectIDs = make(map[uint]map[uint]bool)
	}
	for _, binding := range bindings {
		role := binding.Role
		if !roleIDMap[role.ID] {
			roleIDMap[role.ID] = true
			ctx.RoleIDs = append(ctx.RoleIDs, role.ID)

			switch role.Code {
			case RoleCodeSuperAdmin:
				ctx.IsSystemAdmin = true
				ctx.Role = RoleCodeSuperAdmin
			case RoleCodeTenantAdmin:
				ctx.IsTenantAdmin = true
				if ctx.Role != RoleCodeSuperAdmin {
					ctx.Role = RoleCodeTenantAdmin
				}
			case RoleCodeProjectAdmin:
				ctx.IsProjectAdmin = true
				if ctx.Role != RoleCodeSuperAdmin && ctx.Role != RoleCodeTenantAdmin {
					ctx.Role = RoleCodeProjectAdmin
				}
			}
		}
		roleProjectID := uint(0)
		if binding.ProjectID > 0 {
			roleProjectID = binding.ProjectID
		} else if role.ProjectID > 0 {
			roleProjectID = role.ProjectID
		}
		if ctx.RoleProjectIDs[role.ID] == nil {
			ctx.RoleProjectIDs[role.ID] = make(map[uint]bool)
		}
		ctx.RoleProjectIDs[role.ID][roleProjectID] = true

		if role.DataScope == 1 {
			for _, id := range allProjectIDs(ctx.TenantID) {
				projectIDMap[id] = true
			}
		}
		if binding.ProjectID > 0 {
			projectIDMap[binding.ProjectID] = true
			ctx.UsesProjectLimits = true
		}
		if role.ProjectID > 0 || role.IsInherited || role.Code == RoleCodeProjectAdmin {
			ctx.UsesProjectLimits = true
		}
		if role.ProjectID > 0 {
			projectIDMap[role.ProjectID] = true
		}
	}

	// Legacy hardcoded roles are deprecated in favor of DB roles
	if ctx.IsProjectAdmin && !ctx.IsTenantAdmin && ctx.Role != "admin" {
		ctx.Role = "project_admin"
	}
	if ctx.Role == "" {
		ctx.Role = "viewer"
	}

	for id := range projectIDMap {
		ctx.AllowedProjectIDs = append(ctx.AllowedProjectIDs, id)
	}
	sort.Slice(ctx.AllowedProjectIDs, func(i, j int) bool {
		return ctx.AllowedProjectIDs[i] < ctx.AllowedProjectIDs[j]
	})
}

func loadPermissions(ctx *AuthContext) {
	if ctx == nil || len(ctx.RoleIDs) == 0 {
		return
	}
	var permissions []store.Permission
	store.DB.Model(&store.Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id IN ?", ctx.RoleIDs).
		Find(&permissions)
	baseCodes := make(map[string]bool)
	for _, permission := range permissions {
		baseCodes[permission.Code] = true
	}
	ctx.PermissionCodes = applyPermissionLimits(ctx, baseCodes)
}

func applyPermissionLimits(ctx *AuthContext, baseCodes map[string]bool) map[string]bool {
	if ctx == nil {
		return map[string]bool{}
	}
	if ctx.TenantID == 0 {
		return baseCodes
	}

	tenantLimit := permissionLimitCodeSet("tenant", ctx.TenantID, 0)
	filteredCodes := intersectPermissionCodes(baseCodes, tenantLimit)
	if ctx.IsTenantAdmin && ctx.ProjectID == 0 {
		return filteredCodes
	}
	if !ctx.UsesProjectLimits && ctx.ProjectID == 0 {
		return filteredCodes
	}

	var projectLimit map[string]bool
	if ctx.ProjectID > 0 {
		projectLimit = permissionLimitCodeSet("project", ctx.TenantID, ctx.ProjectID)
	} else {
		projectLimit = projectPermissionLimitUnionCodeSet(ctx.TenantID, ctx.AllowedProjectIDs)
	}
	return intersectPermissionCodes(filteredCodes, projectLimit)
}

func permissionLimitCodeSet(scopeType string, tenantID, projectID uint) map[string]bool {
	var codes []string
	store.DB.Model(&store.Permission{}).
		Select("permissions.code").
		Joins("JOIN scope_permission_limits ON scope_permission_limits.permission_id = permissions.id").
		Where("scope_permission_limits.scope_type = ? AND scope_permission_limits.tenant_id = ? AND scope_permission_limits.project_id = ?", scopeType, tenantID, projectID).
		Scan(&codes)
	return permissionCodeSet(codes)
}

func projectPermissionLimitUnionCodeSet(tenantID uint, projectIDs []uint) map[string]bool {
	if len(projectIDs) == 0 {
		return map[string]bool{}
	}
	var codes []string
	store.DB.Model(&store.Permission{}).
		Select("permissions.code").
		Joins("JOIN scope_permission_limits ON scope_permission_limits.permission_id = permissions.id").
		Where("scope_permission_limits.scope_type = ? AND scope_permission_limits.tenant_id = ? AND scope_permission_limits.project_id IN ?", "project", tenantID, projectIDs).
		Scan(&codes)
	return permissionCodeSet(codes)
}

func permissionCodeSet(codes []string) map[string]bool {
	result := make(map[string]bool, len(codes))
	for _, code := range codes {
		result[code] = true
	}
	return result
}

func intersectPermissionCodes(left, right map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for code := range left {
		if right[code] {
			result[code] = true
		}
	}
	return result
}

func allProjectIDs(tenantID uint) []uint {
	if tenantID == 0 {
		return []uint{}
	}
	var projects []store.Project
	store.DB.Where("tenant_id = ?", tenantID).Find(&projects)
	ids := make([]uint, 0, len(projects))
	for _, project := range projects {
		ids = append(ids, project.ID)
	}
	return ids
}

func projectBelongsToTenant(projectID, tenantID uint) bool {
	var count int64
	store.DB.Model(&store.Project{}).Where("id = ? AND tenant_id = ?", projectID, tenantID).Count(&count)
	return count > 0
}
