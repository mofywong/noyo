package core

import (
	"fmt"
	"sort"

	"noyo/core/store"
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
	PermissionCodes    map[string]bool
	AllowedProjectIDs  []uint
	UsesProjectLimits  bool
	MustChangePassword bool
}

func (ctx *AuthContext) HasPermission(code string) bool {
	if ctx == nil {
		return false
	}
	return ctx.PermissionCodes[code]
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
		SubjectType:     "app",
		AppID:           app.AppID,
		AppDBID:         app.ID,
		Username:        "app:" + app.Name,
		TenantID:        app.TenantID,
		ProjectID:       requestedProjectID,
		Role:            "viewer",
		PermissionCodes: make(map[string]bool),
	}

	if app.Status != 1 {
		return nil, fmt.Errorf("app disabled")
	}

	var appRoles []store.AppRole
	if err := store.DB.Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID).Find(&appRoles).Error; err != nil {
		return nil, err
	}

	roleBindings := make([]resolvedRoleBinding, 0, len(appRoles))
	for _, appRole := range appRoles {
		var role store.Role
		if err := store.DB.Where("id = ? AND (tenant_id = ? OR tenant_id = 0)", appRole.RoleID, app.TenantID).First(&role).Error; err == nil {
			if appRole.ProjectID > 0 {
				var count int64
				store.DB.Model(&store.Project{}).Where("id = ? AND tenant_id = ?", appRole.ProjectID, app.TenantID).Count(&count)
				if count == 0 {
					continue
				}
			}
			roleBindings = append(roleBindings, resolvedRoleBinding{Role: role, ProjectID: appRole.ProjectID})
		}
	}
	applyRoleBindings(ctx, roleBindings, "")
	if requestedProjectID > 0 && !ctx.CanAccessProject(requestedProjectID) {
		return nil, fmt.Errorf("project is outside app allowed scope")
	}
	loadPermissions(ctx)
	return ctx, nil
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

	var userPositions []store.UserPosition
	if err := store.DB.Where("user_id = ?", userID).Find(&userPositions).Error; err != nil {
		return nil, err
	}
	for _, userPosition := range userPositions {
		var position store.Position
		if err := store.DB.Where("id = ? AND tenant_id = ? AND status = ?", userPosition.PositionID, tenantID, 1).First(&position).Error; err != nil {
			continue
		}
		var positionRoles []store.PositionRole
		if err := store.DB.Where("position_id = ?", position.ID).Find(&positionRoles).Error; err != nil {
			return nil, err
		}
		for _, positionRole := range positionRoles {
			var role store.Role
			err := store.DB.Where("id = ? AND (tenant_id = ? OR tenant_id = 0)", positionRole.RoleID, tenantID).First(&role).Error
			if err == nil {
				resolved = append(resolved, resolvedRoleBinding{Role: role, ProjectID: role.ProjectID})
			}
		}
	}

	return resolved, nil
}

func applyRoleBindings(ctx *AuthContext, bindings []resolvedRoleBinding, legacyRole string) {
	roleIDMap := make(map[uint]bool)
	projectIDMap := make(map[uint]bool)
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
	if ctx.IsTenantAdmin || !ctx.UsesProjectLimits {
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
