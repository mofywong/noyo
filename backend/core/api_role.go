package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) handleGetRoles(r *ghttp.Request) {
	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()
	role := r.GetCtxVar("role").String()
	userID := r.GetCtxVar("user_id").Uint()

	isProjectAdmin := role == "project_admin"

	db := store.DB.Model(&store.Role{})
	if tenantID > 0 {
		if isProjectAdmin {
			allowedProjectIDs := s.getManagedProjectIDs(userID)
			if len(allowedProjectIDs) == 0 {
				r.Response.WriteJson(g.Map{"code": 0, "data": []store.Role{}, "total": 0})
				return
			}
			if projectID > 0 {
				allowed := false
				for _, pid := range allowedProjectIDs {
					if pid == projectID {
						allowed = true
						break
					}
				}
				if !allowed {
					r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
					return
				}
				db = db.Where("is_builtin = ? OR (tenant_id = ? AND (project_id = ? OR (project_id = 0 AND is_inherited = ?)))", true, tenantID, projectID, true)
			} else {
				db = db.Where("is_builtin = ? OR (tenant_id = ? AND (project_id IN ? OR (project_id = 0 AND is_inherited = ?)))", true, tenantID, allowedProjectIDs, true)
			}

			// 过滤掉租户管理员等高级角色
			db = db.Where("code NOT IN ?", []string{"tenant_admin", "super_admin"})
		} else {
			if projectID > 0 {
				db = db.Where("is_builtin = ? OR (tenant_id = ? AND (project_id = ? OR (project_id = 0 AND is_inherited = ?)))", true, tenantID, projectID, true)
			} else {
				db = db.Where("tenant_id = ? OR is_builtin = ?", tenantID, true)
			}
			// 租户管理员不应该看到超级管理员角色
			db = db.Where("code != ?", "super_admin")
		}
	}

	var roles []store.Role
	if err := db.Find(&roles).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch roles"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": roles, "total": len(roles)})
}

func (s *Server) handleCreateRole(r *ghttp.Request) {
	var role store.Role
	if err := json.Unmarshal(r.GetBody(), &role); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 {
		role.TenantID = tenantID
		role.IsBuiltin = false
	}

	userRole := r.GetCtxVar("role").String()
	authCtx := requestAuthContext(r)
	if userRole == "project_admin" {
		userID := r.GetCtxVar("user_id").Uint()
		allowedProjectIDs := s.getManagedProjectIDs(userID)
		if len(allowedProjectIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 403, "message": "No project access"})
			return
		}

		if role.ProjectID > 0 {
			allowed := false
			for _, pid := range allowedProjectIDs {
				if pid == role.ProjectID {
					allowed = true
					break
				}
			}
			if !allowed {
				r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
				return
			}
		} else {
			role.ProjectID = allowedProjectIDs[0]
		}
	}

	if authCtx != nil && role.ProjectID > 0 && !authCtx.CanManageProject(role.ProjectID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied to this project"})
		return
	}

	// 约束：如果角色属于某个项目，则不能被继承
	if role.ProjectID > 0 {
		role.IsInherited = false
	}
	// 如果 ProjectID == 0，保留由前端传来的 role.IsInherited 状态

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

	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageRole(role) {
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
	role.Status = update.Status

	// 更新继承属性：仅当非项目级角色时允许继承
	if role.ProjectID == 0 {
		role.IsInherited = update.IsInherited
	} else {
		role.IsInherited = false
	}

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

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 && role.TenantID != tenantID {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageRole(role) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	if role.IsBuiltin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Built-in roles cannot be deleted"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		tx.Model(&store.UserRoleBinding{}).Where("role_id = ?", id).Count(&count)
		if count > 0 {
			return fmt.Errorf("cannot delete role: %d users are still bound to it", count)
		}

		tx.Where("role_id = ?", id).Delete(&store.RolePermission{})
		tx.Where("role_id = ?", id).Delete(&store.RoleDeviceTagPermission{})
		tx.Where("role_id = ?", id).Delete(&store.PositionRole{})

		return tx.Delete(&role).Error
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}
