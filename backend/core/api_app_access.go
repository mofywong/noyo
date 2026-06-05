package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"
	"strconv"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

type appDeviceTagAccessInput struct {
	TagID      uint   `json:"tag_id"`
	Permission string `json:"permission"`
}

type appProjectAccessInput struct {
	ProjectID            uint                      `json:"project_id"`
	PermissionIDs        []uint                    `json:"permission_ids"`
	DeviceTagPermissions []appDeviceTagAccessInput `json:"device_tag_permissions"`
}

func permissionAssignableToApp(permission store.Permission) bool {
	switch permission.Module {
	case "tenant", "user", "role", "app", "audit":
		return false
	case "system":
		return permission.Code == "dashboard:view"
	default:
		return true
	}
}

func permissionCodesAssignableByAuthForProject(tx *gorm.DB, authCtx *AuthContext, projectID uint) (map[string]bool, error) {
	if authCtx == nil || authCtx.TenantID == 0 || projectID == 0 {
		return map[string]bool{}, fmt.Errorf("tenant and project context are required")
	}
	if !projectBelongsToTenantInTx(tx, projectID, authCtx.TenantID) {
		return map[string]bool{}, fmt.Errorf("project is outside tenant scope")
	}
	if !authCtx.IsTenantAdmin && !authCtx.CanAccessProject(projectID) {
		return map[string]bool{}, fmt.Errorf("project is outside allowed scope")
	}

	tenantLimit := permissionLimitCodeSet(permissionLimitScopeTenant, authCtx.TenantID, 0)
	projectLimit := permissionLimitCodeSet(permissionLimitScopeProject, authCtx.TenantID, projectID)
	if authCtx.IsTenantAdmin {
		return intersectPermissionCodes(tenantLimit, projectLimit), nil
	}

	roleIDs := roleIDsEffectiveForProject(authCtx, projectID)
	if len(roleIDs) == 0 {
		return map[string]bool{}, nil
	}
	var permissions []store.Permission
	if err := tx.Model(&store.Permission{}).
		Joins("JOIN role_permissions ON role_permissions.permission_id = permissions.id").
		Where("role_permissions.role_id IN ?", roleIDs).
		Find(&permissions).Error; err != nil {
		return nil, err
	}
	baseCodes := make(map[string]bool, len(permissions))
	for _, permission := range permissions {
		baseCodes[permission.Code] = true
	}
	projectCtx := *authCtx
	projectCtx.ProjectID = projectID
	projectCtx.RoleIDs = roleIDs
	projectCtx.AllowedProjectIDs = []uint{projectID}
	projectCtx.UsesProjectLimits = true
	return applyPermissionLimits(&projectCtx, baseCodes), nil
}

func assignableAppPermissionIDsForProject(tx *gorm.DB, authCtx *AuthContext, projectID uint) (map[uint]bool, []store.Permission, error) {
	allowedCodes, err := permissionCodesAssignableByAuthForProject(tx, authCtx, projectID)
	if err != nil {
		return nil, nil, err
	}
	if len(allowedCodes) == 0 {
		return map[uint]bool{}, []store.Permission{}, nil
	}

	var permissions []store.Permission
	if err := tx.Model(&store.Permission{}).Order("module asc, sort_order asc, code asc").Find(&permissions).Error; err != nil {
		return nil, nil, err
	}
	allowedIDs := make(map[uint]bool)
	filtered := make([]store.Permission, 0, len(permissions))
	for _, permission := range permissions {
		if allowedCodes[permission.Code] && permissionAssignableToApp(permission) {
			allowedIDs[permission.ID] = true
			filtered = append(filtered, permission)
		}
	}
	return allowedIDs, filtered, nil
}

func canGrantAppDeviceTagPermission(tx *gorm.DB, authCtx *AuthContext, projectID, tagID uint, permission string) (bool, error) {
	if permission != "read" && permission != "write" {
		return false, fmt.Errorf("invalid device tag permission")
	}
	if authCtx == nil || authCtx.TenantID == 0 || projectID == 0 || tagID == 0 {
		return false, fmt.Errorf("tenant, project and device tag are required")
	}
	if !projectBelongsToTenantInTx(tx, projectID, authCtx.TenantID) {
		return false, fmt.Errorf("project is outside tenant scope")
	}
	scopeID := strconv.FormatUint(uint64(authCtx.TenantID), 10)
	var tagCount int64
	if err := tx.Model(&store.DeviceTag{}).Where("scope_type = ? AND scope_id = ? AND id = ?", "tenant", scopeID, tagID).Count(&tagCount).Error; err != nil {
		return false, err
	}
	if tagCount == 0 {
		return false, fmt.Errorf("device tag is outside tenant scope")
	}
	if authCtx.IsTenantAdmin || (authCtx.IsProjectAdmin && authCtx.CanManageProject(projectID)) {
		return true, nil
	}
	if !authCtx.CanAccessProject(projectID) {
		return false, fmt.Errorf("project is outside allowed scope")
	}
	query, ok := scopedDeviceTagPermissionQuery(authCtx, projectID)
	if !ok {
		return false, nil
	}
	var count int64
	allowedPermissions := []string{"write"}
	if permission == "read" {
		allowedPermissions = []string{"read", "write"}
	}
	if err := query.Where("role_device_tag_permissions.tag_id = ? AND role_device_tag_permissions.permission IN ?", tagID, allowedPermissions).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func validateAppAccessPayload(tx *gorm.DB, app store.App, authCtx *AuthContext, projects []appProjectAccessInput) error {
	if authCtx == nil || app.TenantID != authCtx.TenantID {
		return fmt.Errorf("Access denied")
	}
	seenProjects := make(map[uint]bool, len(projects))
	for _, project := range projects {
		if project.ProjectID == 0 {
			return fmt.Errorf("project is required")
		}
		if seenProjects[project.ProjectID] {
			return fmt.Errorf("duplicate project access")
		}
		seenProjects[project.ProjectID] = true

		assignablePermissionIDs, _, err := assignableAppPermissionIDsForProject(tx, authCtx, project.ProjectID)
		if err != nil {
			return err
		}
		for _, permissionID := range uniquePermissionIDs(project.PermissionIDs) {
			if !assignablePermissionIDs[permissionID] {
				return fmt.Errorf("permission is outside allowed scope")
			}
		}

		seenTags := make(map[uint]bool, len(project.DeviceTagPermissions))
		for _, tagPermission := range project.DeviceTagPermissions {
			if seenTags[tagPermission.TagID] {
				return fmt.Errorf("duplicate device tag assignment")
			}
			seenTags[tagPermission.TagID] = true
			allowed, err := canGrantAppDeviceTagPermission(tx, authCtx, project.ProjectID, tagPermission.TagID, tagPermission.Permission)
			if err != nil {
				return err
			}
			if !allowed {
				return fmt.Errorf("device tag permission is outside allowed scope")
			}
		}
	}
	return nil
}

func appAccessProjectFilter(authCtx *AuthContext) ([]uint, bool) {
	if authCtx == nil {
		return []uint{}, true
	}
	projectIDs, restricted := authCtx.ProjectIDsForTenantQuery()
	if !restricted {
		return nil, false
	}
	return uniquePositiveUintIDs(projectIDs), true
}

func uniquePositiveUintIDs(ids []uint) []uint {
	seen := make(map[uint]bool, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
}

func replaceAppAccess(tx *gorm.DB, app store.App, authCtx *AuthContext, projects []appProjectAccessInput) error {
	projectIDs, restricted := appAccessProjectFilter(authCtx)
	if restricted && len(projectIDs) == 0 {
		return nil
	}

	deleteProjectAccess := tx.Unscoped().Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID)
	deletePermissions := tx.Unscoped().Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID)
	deleteTagPermissions := tx.Unscoped().Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID)
	if restricted {
		deleteProjectAccess = deleteProjectAccess.Where("project_id IN ?", projectIDs)
		deletePermissions = deletePermissions.Where("project_id IN ?", projectIDs)
		deleteTagPermissions = deleteTagPermissions.Where("project_id IN ?", projectIDs)
	}

	if err := deleteProjectAccess.Delete(&store.AppProjectAccess{}).Error; err != nil {
		return err
	}
	if err := deletePermissions.Delete(&store.AppPermission{}).Error; err != nil {
		return err
	}
	if err := deleteTagPermissions.Delete(&store.AppDeviceTagPermission{}).Error; err != nil {
		return err
	}
	for _, project := range projects {
		if err := tx.Create(&store.AppProjectAccess{AppID: app.ID, TenantID: app.TenantID, ProjectID: project.ProjectID}).Error; err != nil {
			return err
		}
		for _, permissionID := range uniquePermissionIDs(project.PermissionIDs) {
			if err := tx.Create(&store.AppPermission{AppID: app.ID, TenantID: app.TenantID, ProjectID: project.ProjectID, PermissionID: permissionID}).Error; err != nil {
				return err
			}
		}
		for _, tagPermission := range project.DeviceTagPermissions {
			if err := tx.Create(&store.AppDeviceTagPermission{
				AppID:      app.ID,
				TenantID:   app.TenantID,
				ProjectID:  project.ProjectID,
				TagID:      tagPermission.TagID,
				Permission: tagPermission.Permission,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) handleGetAppAccessOptions(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var projects []store.Project
	projectQuery := store.DB.Model(&store.Project{}).Where("tenant_id = ?", authCtx.TenantID)
	if projectIDs, restricted := authCtx.ProjectIDsForTenantQuery(); restricted {
		if len(projectIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"projects": []store.Project{}, "permissions_by_project": []g.Map{}, "device_tags": []store.DeviceTagWithCount{}}})
			return
		}
		projectQuery = projectQuery.Where("id IN ?", projectIDs)
	}
	if err := projectQuery.Order("created_at desc").Find(&projects).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch projects"})
		return
	}

	permissionsByProject := make([]g.Map, 0, len(projects))
	for _, project := range projects {
		_, permissions, err := assignableAppPermissionIDsForProject(store.DB, authCtx, project.ID)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
			return
		}
		permissionsByProject = append(permissionsByProject, g.Map{"project_id": project.ID, "permissions": permissions})
	}

	tags, err := store.ListDeviceTags(currentDeviceTagScope(r))
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"projects":               projects,
		"permissions_by_project": permissionsByProject,
		"device_tags":            tags,
	}})
}

func (s *Server) handleGetAppAccess(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanUseTenantScopedResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var accesses []store.AppProjectAccess
	accessQuery := store.DB.Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID)
	if projectIDs, restricted := appAccessProjectFilter(authCtx); restricted {
		if len(projectIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"projects": []appProjectAccessInput{}}})
			return
		}
		accessQuery = accessQuery.Where("project_id IN ?", projectIDs)
	}
	if err := accessQuery.Order("project_id asc").Find(&accesses).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch app access"})
		return
	}
	result := make([]appProjectAccessInput, 0, len(accesses))
	for _, access := range accesses {
		var permissionIDs []uint
		if err := store.DB.Model(&store.AppPermission{}).Where("app_id = ? AND tenant_id = ? AND project_id = ?", app.ID, app.TenantID, access.ProjectID).Order("permission_id asc").Pluck("permission_id", &permissionIDs).Error; err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch app permissions"})
			return
		}
		var tagPermissions []store.AppDeviceTagPermission
		if err := store.DB.Where("app_id = ? AND tenant_id = ? AND project_id = ?", app.ID, app.TenantID, access.ProjectID).Order("tag_id asc").Find(&tagPermissions).Error; err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch app device tag permissions"})
			return
		}
		deviceTags := make([]appDeviceTagAccessInput, 0, len(tagPermissions))
		for _, tagPermission := range tagPermissions {
			deviceTags = append(deviceTags, appDeviceTagAccessInput{TagID: tagPermission.TagID, Permission: tagPermission.Permission})
		}
		result = append(result, appProjectAccessInput{ProjectID: access.ProjectID, PermissionIDs: permissionIDs, DeviceTagPermissions: deviceTags})
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"projects": result}})
}

func (s *Server) handleSetAppAccess(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanUseTenantScopedResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var req struct {
		Projects []appProjectAccessInput `json:"projects"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateAppAccessPayload(tx, app, authCtx, req.Projects); err != nil {
			return err
		}
		return replaceAppAccess(tx, app, authCtx, req.Projects)
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Failed to update app access: " + err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "App access updated successfully"})
}
