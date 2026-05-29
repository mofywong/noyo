package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"

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
		db = db.Where("module IN ?", []string{"tenant", "system"})
	case authCtx.IsTenantAdmin:
		db = db.Where("module NOT IN ?", []string{"tenant", "system"})
	default:
		db = db.Where("module NOT IN ?", []string{"tenant", "system", "project"})
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
	if authCtx == nil || authCtx.IsSystemAdmin {
		return false
	}
	if permission.Module == "tenant" || permission.Module == "system" {
		return false
	}
	if authCtx.IsProjectAdmin && permission.Module == "project" {
		return false
	}
	if targetRole.ProjectID > 0 && authCtx.IsProjectAdmin && permission.Module == "project" {
		return false
	}
	return true
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
			return fmt.Errorf("permission %s is outside assignable scope", permission.Code)
		}
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
		PermissionIDs []uint `json:"permission_ids"`
		DeviceTags    []struct {
			TagID      uint   `json:"tag_id"`
			Permission string `json:"permission"`
		} `json:"device_tags"`
	}

	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := validateAssignablePermissionIDs(tx, req.PermissionIDs, targetRole, authCtx); err != nil {
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
		r.Response.WriteJson(g.Map{"code": 403, "message": "Failed to update role permissions: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Permissions updated successfully"})
}
