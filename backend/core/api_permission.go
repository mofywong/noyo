package core

import (
	"encoding/json"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) handleGetSystemPermissions(r *ghttp.Request) {
	var perms []store.Permission
	if err := store.DB.Find(&perms).Error; err != nil {
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

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 && targetRole.TenantID != tenantID && !targetRole.IsBuiltin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || (!authCtx.CanManageRole(targetRole) && !targetRole.IsBuiltin) {
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

func (s *Server) handleSetRolePermissions(r *ghttp.Request) {
	roleID := r.Get("id").Uint()
	if roleID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Role ID is required"})
		return
	}

	role := r.GetCtxVar("role").String()
	isProjectAdmin := role == "project_admin"

	var targetRole store.Role
	if err := store.DB.First(&targetRole, roleID).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Role not found"})
		return
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 && targetRole.TenantID != tenantID {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageRole(targetRole) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	if targetRole.IsBuiltin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Cannot modify builtin roles"})
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
		if isProjectAdmin {
			var perms []store.Permission
			if err := tx.Where("id IN ?", req.PermissionIDs).Find(&perms).Error; err == nil {
				var validIDs []uint
				for _, p := range perms {
					if p.Module != "project" && p.Module != "tenant" {
						validIDs = append(validIDs, p.ID)
					}
				}
				req.PermissionIDs = validIDs
			}
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
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update role permissions: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Permissions updated successfully"})
}
