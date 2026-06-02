package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

var assignableBuiltinRoleCodes = []string{RoleCodeTenantAdmin, RoleCodeProjectAdmin}

func isReservedAdminRoleCode(code string) bool {
	return code == RoleCodeSuperAdmin || code == RoleCodeTenantAdmin || code == RoleCodeProjectAdmin
}

func (s *Server) handleGetRoles(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	includeBuiltin := r.Get("include_builtin").Bool()
	projectID := r.Get("project_id").Uint()
	if projectID == 0 {
		projectID = r.GetCtxVar("project_id").Uint()
	}

	db := store.DB.Model(&store.Role{})
	if authCtx.IsTenantAdmin {
		if projectID > 0 {
			if !projectBelongsToTenant(projectID, authCtx.TenantID) {
				r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
				return
			}
			db = db.Where("tenant_id = ? AND is_builtin = ? AND (project_id = ? OR project_id = 0)", authCtx.TenantID, false, projectID)
		} else {
			db = db.Where("tenant_id = ? AND is_builtin = ?", authCtx.TenantID, false)
		}
		if includeBuiltin {
			db = db.Or("tenant_id = ? AND project_id = ? AND is_builtin = ? AND code IN ?", 0, 0, true, assignableBuiltinRoleCodes)
		}
	} else {
		allowedProjectIDs := authCtx.AllowedProjectIDs
		if projectID > 0 {
			if !authCtx.CanAccessProject(projectID) {
				r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
				return
			}
			allowedProjectIDs = []uint{projectID}
		}
		if len(allowedProjectIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 0, "data": []store.Role{}, "total": 0})
			return
		}
		db = db.Where(
			"tenant_id = ? AND is_builtin = ? AND (project_id IN ? OR (project_id = 0 AND is_inherited = ?))",
			authCtx.TenantID,
			false,
			allowedProjectIDs,
			true,
		)
	}

	var roles []store.Role
	if err := db.Order("project_id asc, created_at desc").Find(&roles).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch roles"})
		return
	}

	type RoleResponse struct {
		store.Role
		HasPermissions bool `json:"has_permissions"`
	}
	res := make([]RoleResponse, 0, len(roles))
	if len(roles) > 0 {
		var roleIDs []uint
		for _, r := range roles {
			roleIDs = append(roleIDs, r.ID)
		}

		var rpCounts []struct {
			RoleID uint
		}
		store.DB.Model(&store.RolePermission{}).Select("role_id").Where("role_id IN ?", roleIDs).Group("role_id").Find(&rpCounts)

		var dtpCounts []struct {
			RoleID uint
		}
		store.DB.Model(&store.RoleDeviceTagPermission{}).Select("role_id").Where("role_id IN ?", roleIDs).Group("role_id").Find(&dtpCounts)

		hasPermMap := make(map[uint]bool)
		for _, c := range rpCounts {
			hasPermMap[c.RoleID] = true
		}
		for _, c := range dtpCounts {
			hasPermMap[c.RoleID] = true
		}

		for _, r := range roles {
			res = append(res, RoleResponse{Role: r, HasPermissions: hasPermMap[r.ID]})
		}
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": res, "total": len(roles)})
}

func (s *Server) handleCreateRole(r *ghttp.Request) {
	var role store.Role
	if err := json.Unmarshal(r.GetBody(), &role); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if isReservedAdminRoleCode(role.Code) {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Reserved admin roles are managed by the system"})
		return
	}

	role.TenantID = authCtx.TenantID
	role.IsBuiltin = false

	if role.ProjectID == 0 {
		if !authCtx.IsTenantAdmin {
			if authCtx.ProjectID > 0 {
				role.ProjectID = authCtx.ProjectID
			} else if len(authCtx.AllowedProjectIDs) > 0 {
				role.ProjectID = authCtx.AllowedProjectIDs[0]
			} else {
				r.Response.WriteJson(g.Map{"code": 403, "message": "No project access"})
				return
			}
		}
	}

	if role.ProjectID == 0 {
		if !authCtx.IsTenantAdmin {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Only tenant admins can create tenant common roles"})
			return
		}
		} else {
		if !projectBelongsToTenant(role.ProjectID, authCtx.TenantID) || !authCtx.CanManageProject(role.ProjectID) {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
			return
		}
		role.IsInherited = false
	}

	var count int64
	store.DB.Model(&store.Role{}).Where("tenant_id = ? AND project_id = ? AND code = ?", role.TenantID, role.ProjectID, role.Code).Count(&count)
	if count > 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Role code already exists in this project/tenant"})
		return
	}

	if err := store.DB.Create(&role).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to create role: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": role})
}

func (s *Server) handleUpdateRole(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var role store.Role
	if err := store.DB.First(&role, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Role not found"})
		return
	}

	if role.IsBuiltin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Built-in roles cannot be modified"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanManageRole(role) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var update store.Role
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	role.Name = update.Name
	role.Description = update.Description
	role.DataScope = update.DataScope
	role.IsInherited = role.ProjectID == 0

	if err := store.DB.Save(&role).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update role: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": role})
}

func (s *Server) handleDeleteRole(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var role store.Role
	if err := store.DB.First(&role, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Role not found"})
		return
	}

	if role.IsBuiltin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Built-in roles cannot be deleted"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageRole(role) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&store.UserRoleBinding{}).Where("role_id = ?", id).Count(&count)
		if count > 0 {
			return fmt.Errorf("cannot delete role: %d users are still bound to it", count)
		}

		tx.Unscoped().Where("role_id = ?", id).Delete(&store.RolePermission{})
		tx.Unscoped().Where("role_id = ?", id).Delete(&store.RoleDeviceTagPermission{})
		tx.Unscoped().Where("role_id = ?", id).Delete(&store.PositionRole{})

		return tx.Unscoped().Delete(&role).Error
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}
