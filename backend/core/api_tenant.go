package core

import (
	"encoding/json"
	"noyo/core/store"
	"noyo/core/utils"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) handleGetTenants(r *ghttp.Request) {
	var tenants []store.Tenant
	if err := store.DB.Find(&tenants).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch tenants"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": tenants, "total": len(tenants)})
}

func (s *Server) handleCreateTenant(r *ghttp.Request) {
	var req struct {
		store.Tenant
		AdminUsername string `json:"admin_username"`
		AdminPassword string `json:"admin_password"`
	}

	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	if req.AdminUsername == "" || req.AdminPassword == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Admin username and password are required"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&req.Tenant).Error; err != nil {
			return err
		}

		hashedPassword, _ := utils.HashPassword(req.AdminPassword)
		adminUser := store.User{
			TenantID:    req.Tenant.ID,
			Username:    req.AdminUsername,
			Password:    hashedPassword,
			DisplayName: req.Tenant.Contact,
			Role:        "admin",
			Status:      1,
		}

		if err := tx.Create(&adminUser).Error; err != nil {
			return err
		}

		// 1. Find global tenant_admin Role
		var globalTenantAdmin store.Role
		if err := tx.Where("code = ? AND tenant_id = 0", "tenant_admin").First(&globalTenantAdmin).Error; err != nil {
			return err
		}
		
		// Find global project_admin Role
		var globalProjectAdmin store.Role
		if err := tx.Where("code = ? AND tenant_id = 0", "project_admin").First(&globalProjectAdmin).Error; err != nil {
			return err
		}

		// 2. Clone tenant_admin
		tenantAdminRole := globalTenantAdmin
		tenantAdminRole.ID = 0
		tenantAdminRole.TenantID = req.Tenant.ID
		tenantAdminRole.IsBuiltin = true
		if err := tx.Create(&tenantAdminRole).Error; err != nil {
			return err
		}

		// Clone project_admin
		projectAdminRole := globalProjectAdmin
		projectAdminRole.ID = 0
		projectAdminRole.TenantID = req.Tenant.ID
		projectAdminRole.IsBuiltin = true
		if err := tx.Create(&projectAdminRole).Error; err != nil {
			return err
		}

		// 3. Clone permissions for tenant_admin
		var tenantAdminPerms []store.RolePermission
		tx.Where("role_id = ?", globalTenantAdmin.ID).Find(&tenantAdminPerms)
		for _, rp := range tenantAdminPerms {
			newRp := store.RolePermission{
				RoleID:       tenantAdminRole.ID,
				PermissionID: rp.PermissionID,
			}
			tx.Create(&newRp)
		}

		// Clone permissions for project_admin
		var projectAdminPerms []store.RolePermission
		tx.Where("role_id = ?", globalProjectAdmin.ID).Find(&projectAdminPerms)
		for _, rp := range projectAdminPerms {
			newRp := store.RolePermission{
				RoleID:       projectAdminRole.ID,
				PermissionID: rp.PermissionID,
			}
			tx.Create(&newRp)
		}

		// 4. Map user to the newly cloned tenant_admin role
		userRole := store.UserRoleBinding{
			UserID: adminUser.ID,
			RoleID: tenantAdminRole.ID,
			TenantID: req.Tenant.ID,
			ProjectID: 0,
		}
		if err := tx.Create(&userRole).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to create tenant: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": req.Tenant})
}

func (s *Server) handleUpdateTenant(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var tenant store.Tenant
	if err := store.DB.First(&tenant, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Tenant not found"})
		return
	}

	var update store.Tenant
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tenant.Name = update.Name
	tenant.Contact = update.Contact
	tenant.Phone = update.Phone
	tenant.Email = update.Email
	tenant.Description = update.Description
	tenant.Logo = update.Logo
	tenant.LoginSuffix = update.LoginSuffix
	tenant.Status = update.Status
	tenant.MaxUsers = update.MaxUsers
	tenant.MaxDevices = update.MaxDevices
	tenant.ExpiresAt = update.ExpiresAt

	if err := store.DB.Save(&tenant).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update tenant: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": tenant})
}

func (s *Server) handleDeleteTenant(r *ghttp.Request) {
	id := r.Get("id").Uint()
	
	err := store.DB.Transaction(func(tx *gorm.DB) error {
		tx.Where("tenant_id = ?", id).Delete(&store.User{})
		tx.Where("tenant_id = ?", id).Delete(&store.Role{})
		tx.Where("tenant_id = ?", id).Delete(&store.Project{})
		tx.Where("tenant_id = ?", id).Delete(&store.App{})
		tx.Where("tenant_id = ?", id).Delete(&store.UserRoleBinding{})
		tx.Where("tenant_id = ?", id).Delete(&store.AuditLog{})
		return tx.Delete(&store.Tenant{}, id).Error
	})
	
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete tenant: " + err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}

func (s *Server) handleResetTenantPassword(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var req struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	if req.NewPassword == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "New password is required"})
		return
	}
	
	// Ensure the caller is a system admin
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.IsSystemAdmin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Only System Admin can reset tenant passwords"})
		return
	}

	var adminUser store.User
	if err := store.DB.Where("tenant_id = ? AND role = ?", id, "admin").First(&adminUser).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Tenant admin user not found"})
		return
	}

	hashed, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to hash password"})
		return
	}

	adminUser.Password = hashed
	if err := store.DB.Save(&adminUser).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to save password"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Password reset successfully"})
}

