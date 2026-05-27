package core

import (
	"fmt"
	"sort"

	"noyo/core/store"
)

const authContextKey = "auth_context"

type AuthContext struct {
	SubjectType       string
	UserID            uint
	AppID             string
	AppDBID           uint
	Username          string
	TenantID          uint
	ProjectID         uint
	Role              string
	IsSystemAdmin     bool
	IsTenantAdmin     bool
	IsProjectAdmin    bool
	RoleIDs           []uint
	PermissionCodes   map[string]bool
	AllowedProjectIDs []uint
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
	if projectID == 0 {
		return ctx.IsSystemAdmin || ctx.IsTenantAdmin
	}
	for _, id := range ctx.AllowedProjectIDs {
		if id == projectID {
			return true
		}
	}
	return false
}

func (ctx *AuthContext) ProjectIDsForTenantQuery() ([]uint, bool) {
	if ctx == nil || ctx.IsSystemAdmin || ctx.IsTenantAdmin {
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
		if role == "admin" && ctx.IsTenantAdmin {
			return true
		}
		if role == "project_admin" && ctx.IsProjectAdmin {
			return true
		}
	}
	return false
}

func (ctx *AuthContext) CanManageTenantResource(tenantID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin {
		return true
	}
	return tenantID > 0 && tenantID == ctx.TenantID && ctx.IsTenantAdmin
}

func (ctx *AuthContext) CanManageProject(projectID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin || ctx.IsTenantAdmin {
		return true
	}
	return projectID > 0 && ctx.CanAccessProject(projectID)
}

func (ctx *AuthContext) CanManageRole(role store.Role) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin {
		return true
	}
	if role.TenantID != ctx.TenantID {
		return false
	}
	if ctx.IsTenantAdmin {
		return true
	}
	return role.ProjectID > 0 && ctx.CanAccessProject(role.ProjectID)
}

func (ctx *AuthContext) CanAssignRole(role store.Role, targetProjectID uint) bool {
	if ctx == nil {
		return false
	}
	if ctx.IsSystemAdmin {
		return true
	}
	if role.Code == "tenant_admin" && !ctx.IsTenantAdmin {
		return false
	}
	if role.TenantID != 0 && role.TenantID != ctx.TenantID {
		return false
	}
	if targetProjectID == 0 {
		return ctx.IsTenantAdmin
	}
	if !ctx.CanAccessProject(targetProjectID) {
		return false
	}
	if ctx.IsTenantAdmin {
		return role.ProjectID == 0 || role.ProjectID == targetProjectID
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
	if user.TenantID == 0 && requestedTenantID > 0 {
		tenantID = requestedTenantID
	} else if requestedTenantID > 0 && requestedTenantID != user.TenantID {
		return nil, fmt.Errorf("tenant is outside allowed scope")
	}

	ctx := &AuthContext{
		SubjectType:     "user",
		UserID:          user.ID,
		Username:        user.Username,
		TenantID:        tenantID,
		ProjectID:       requestedProjectID,
		Role:            user.Role,
		PermissionCodes: make(map[string]bool),
	}

	roleBindings, err := resolveUserRoleBindings(user.ID, tenantID)
	if err != nil {
		return nil, err
	}
	applyRoleBindings(ctx, roleBindings, user.Role)

	if requestedProjectID > 0 && !ctx.CanAccessProject(requestedProjectID) {
		return nil, fmt.Errorf("project is outside allowed scope")
	}
	loadPermissions(ctx)
	return ctx, nil
}

func ResolveAppAuthContext(app store.App, requestedProjectID uint) (*AuthContext, error) {
	if app.Status != 1 {
		return nil, fmt.Errorf("app disabled")
	}
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

	var appRoles []store.AppRole
	if err := store.DB.Where("app_id = ?", app.ID).Find(&appRoles).Error; err != nil {
		return nil, err
	}

	roleBindings := make([]resolvedRoleBinding, 0, len(appRoles))
	for _, appRole := range appRoles {
		var role store.Role
		if err := store.DB.Where("id = ? AND status = ?", appRole.RoleID, 1).First(&role).Error; err == nil {
			roleBindings = append(roleBindings, resolvedRoleBinding{Role: role, ProjectID: role.ProjectID})
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
		err := store.DB.Where("id = ? AND status = ? AND (tenant_id = ? OR tenant_id = 0)", binding.RoleID, 1, tenantID).First(&role).Error
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
			err := store.DB.Where("id = ? AND status = ? AND (tenant_id = ? OR tenant_id = 0)", positionRole.RoleID, 1, tenantID).First(&role).Error
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
		if roleIDMap[role.ID] {
			continue
		}
		roleIDMap[role.ID] = true
		ctx.RoleIDs = append(ctx.RoleIDs, role.ID)

		switch role.Code {
		case "super_admin":
			ctx.IsSystemAdmin = true
			if ctx.Role == "" || ctx.Role == "viewer" || ctx.Role == "admin" || ctx.Role == "project_admin" {
				ctx.Role = "superadmin"
			}
		case "tenant_admin":
			ctx.IsTenantAdmin = true
			if ctx.Role == "" || ctx.Role == "viewer" || ctx.Role == "project_admin" {
				ctx.Role = "admin"
			}
		case "project_admin":
			ctx.IsProjectAdmin = true
			if ctx.Role == "" || ctx.Role == "viewer" {
				ctx.Role = "project_admin"
			}
		}

		if role.DataScope == 1 {
			for _, id := range allProjectIDs(ctx.TenantID) {
				projectIDMap[id] = true
			}
			continue
		}
		if binding.ProjectID > 0 {
			projectIDMap[binding.ProjectID] = true
			continue
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
	for _, permission := range permissions {
		ctx.PermissionCodes[permission.Code] = true
	}
}

func allProjectIDs(tenantID uint) []uint {
	if tenantID == 0 {
		return []uint{}
	}
	var projects []store.Project
	store.DB.Where("tenant_id = ? AND status = ?", tenantID, 1).Find(&projects)
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
