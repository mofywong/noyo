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

func (s *Server) handleGetSystemPermissions(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	db := store.DB.Model(&store.Permission{})
	switch {
	case authCtx.IsSystemAdmin:
		db = tenantPermissionOptionQuery(store.DB)
	case authCtx.IsTenantAdmin:
		projectID := r.Get("project_id").Uint()
		if projectID > 0 {
			if !projectBelongsToTenant(projectID, authCtx.TenantID) {
				r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
				return
			}
			db = db.Where(
				"id IN (?)",
				store.DB.Model(&store.ScopePermissionLimit{}).
					Select("permission_id").
					Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeProject, authCtx.TenantID, projectID),
			)
		} else {
			db = db.Where(
				"id IN (?)",
				store.DB.Model(&store.ScopePermissionLimit{}).
					Select("permission_id").
					Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeTenant, authCtx.TenantID, 0),
			)
		}
	default:
		projectID := r.Get("project_id").Uint()
		if projectID == 0 {
			projectID = authCtx.ProjectID
		}
		if projectID == 0 && len(authCtx.AllowedProjectIDs) == 1 {
			projectID = authCtx.AllowedProjectIDs[0]
		}
		if projectID == 0 || !authCtx.CanAccessProject(projectID) {
			r.Response.WriteJson(g.Map{"code": 0, "data": []store.Permission{}, "total": 0})
			return
		}
		db = db.Where(
			"id IN (?)",
			store.DB.Model(&store.ScopePermissionLimit{}).
				Select("permission_id").
				Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeProject, authCtx.TenantID, projectID),
		)
	}

	var perms []store.Permission
	if err := db.Order("module asc, sort_order asc, code asc").Find(&perms).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch permissions"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": perms, "total": len(perms)})
}

func (s *Server) handleGetRolePermissions(r *ghttp.Request) {
	roleID := r.Get("id").Uint()
	if roleID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Role ID is required"})
		return
	}

	var targetRole store.Role
	if err := store.DB.First(&targetRole, roleID).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Role not found"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if !targetRole.IsBuiltin && !authCtx.CanViewRole(targetRole) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if targetRole.IsBuiltin && authCtx.TenantID == 0 && !authCtx.IsSystemAdmin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var rps []store.RolePermission
	if err := store.DB.Where("role_id = ?", roleID).Find(&rps).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch role permissions"})
		return
	}

	var dtps []store.RoleDeviceTagPermission
	if err := store.DB.Where("role_id = ?", roleID).Find(&dtps).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch role device tag permissions"})
		return
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"permissions": rps,
			"device_tags": dtps,
		},
	})
}

func permissionAssignableToRole(permission store.Permission, targetRole store.Role, authCtx *AuthContext) bool {
	if authCtx == nil {
		return false
	}
	if authCtx.IsSystemAdmin {
		return true
	}
	if permission.Module == "system" {
		return false
	}
	if targetRole.ProjectID > 0 && permission.Module != "project" {
		return false
	}
	if authCtx.IsTenantAdmin {
		return true
	}
	if permission.Module == "project" {
		return true
	}
	return false
}

func validateAssignablePermissionIDs(tx *gorm.DB, permissionIDs []uint, targetRole store.Role, authCtx *AuthContext) error {
	if len(permissionIDs) == 0 {
		return nil
	}

	var perms []store.Permission
	if err := tx.Where("id IN ?", permissionIDs).Find(&perms).Error; err != nil {
		return err
	}
	if len(perms) != len(permissionIDs) {
		return fmt.Errorf("invalid permission assignment")
	}
	for _, permission := range perms {
		if !permissionAssignableToRole(permission, targetRole, authCtx) {
			return fmt.Errorf("所选权限 [%s] 超出了可分配的模块范围，请检查是否跨越了项目或租户层级限制", permission.Name)
		}
		if !permissionWithinAssignmentLimit(tx, permission.ID, targetRole, authCtx) {
			return fmt.Errorf("所选权限 [%s] 超过了当前系统授予的最大权限边界限制", permission.Name)
		}
	}
	return nil
}

type roleDeviceTagAssignmentInput struct {
	TagID      uint   `json:"tag_id"`
	Permission string `json:"permission"`
}

func validateRoleDeviceTagAssignments(tx *gorm.DB, authCtx *AuthContext, targetRole store.Role, assignments []roleDeviceTagAssignmentInput) error {
	if len(assignments) == 0 {
		return nil
	}
	if authCtx == nil || authCtx.TenantID == 0 {
		return fmt.Errorf("tenant context is required")
	}
	if targetRole.TenantID != authCtx.TenantID {
		return fmt.Errorf("role is outside tenant scope")
	}
	if targetRole.ProjectID > 0 && !authCtx.CanManageProject(targetRole.ProjectID) {
		return fmt.Errorf("role project is outside allowed scope")
	}

	tagIDs := make([]uint, 0, len(assignments))
	seenTags := make(map[uint]bool, len(assignments))
	for _, assignment := range assignments {
		if assignment.TagID == 0 {
			return fmt.Errorf("device tag is required")
		}
		if assignment.Permission != "read" && assignment.Permission != "write" {
			return fmt.Errorf("invalid device tag permission")
		}
		if seenTags[assignment.TagID] {
			return fmt.Errorf("duplicate device tag assignment")
		}
		seenTags[assignment.TagID] = true
		tagIDs = append(tagIDs, assignment.TagID)
	}

	scopeID := strconv.FormatUint(uint64(authCtx.TenantID), 10)
	var count int64
	if err := tx.Model(&store.DeviceTag{}).
		Where("scope_type = ? AND scope_id = ? AND id IN ?", "tenant", scopeID, tagIDs).
		Count(&count).Error; err != nil {
		return err
	}
	if count != int64(len(tagIDs)) {
		return fmt.Errorf("one or more device tags are outside tenant scope")
	}
	return nil
}

func (s *Server) handleSetRolePermissions(r *ghttp.Request) {
	roleID := r.Get("id").Uint()
	if roleID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Role ID is required"})
		return
	}

	var targetRole store.Role
	if err := store.DB.First(&targetRole, roleID).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Role not found"})
		return
	}

	if targetRole.IsBuiltin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Cannot modify builtin roles"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanManageRole(targetRole) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var req struct {
		PermissionIDs []uint                         `json:"permission_ids"`
		DeviceTags    []roleDeviceTagAssignmentInput `json:"device_tags"`
	}

	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateAssignablePermissionIDs(tx, req.PermissionIDs, targetRole, authCtx); err != nil {
			return err
		}
		if err := validateRoleDeviceTagAssignments(tx, authCtx, targetRole, req.DeviceTags); err != nil {
			return err
		}

		if err := tx.Where("role_id = ?", roleID).Delete(&store.RolePermission{}).Error; err != nil {
			return err
		}

		for _, pID := range req.PermissionIDs {
			rp := store.RolePermission{RoleID: roleID, PermissionID: pID}
			if err := tx.Create(&rp).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("role_id = ?", roleID).Delete(&store.RoleDeviceTagPermission{}).Error; err != nil {
			return err
		}

		for _, dt := range req.DeviceTags {
			dtp := store.RoleDeviceTagPermission{RoleID: roleID, TagID: dt.TagID, Permission: dt.Permission}
			if err := tx.Create(&dtp).Error; err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "保存角色权限失败: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Permissions updated successfully"})
}
