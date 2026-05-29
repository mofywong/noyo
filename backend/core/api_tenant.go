package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"
	"noyo/core/utils"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) handleGetTenants(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 20).Int()
	if pageSize > 100 {
		pageSize = 100
	}
	if page < 1 {
		page = 1
	}

	var total int64
	store.DB.Model(&store.Tenant{}).Count(&total)

	var tenants []store.Tenant
	offset := (page - 1) * pageSize
	if err := store.DB.Offset(offset).Limit(pageSize).Find(&tenants).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch tenants"})
		return
	}

	type TenantResponse struct {
		store.Tenant
		PermissionIDs []uint `json:"permission_ids"`
	}
	res := make([]TenantResponse, 0, len(tenants))
	for _, tenant := range tenants {
		permissionIDs, err := loadScopePermissionLimitIDs(store.DB, permissionLimitScopeTenant, tenant.ID, 0)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch tenant permission limits"})
			return
		}
		res = append(res, TenantResponse{Tenant: tenant, PermissionIDs: permissionIDs})
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": res, "total": total, "page": page, "pageSize": pageSize})
}

func (s *Server) handleCreateTenant(r *ghttp.Request) {
	var req struct {
		store.Tenant
		AdminUsername string `json:"admin_username"`
		AdminPassword string `json:"admin_password"`
		PermissionIDs []uint `json:"permission_ids"`
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

		var tenantAdminRole store.Role
		if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error; err != nil {
			return err
		}

		userRole := store.UserRoleBinding{
			UserID:    adminUser.ID,
			RoleID:    tenantAdminRole.ID,
			TenantID:  req.Tenant.ID,
			ProjectID: 0,
		}
		if err := tx.Create(&userRole).Error; err != nil {
			return err
		}

		return replaceTenantPermissionLimit(tx, req.Tenant.ID, req.PermissionIDs)
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

	var update struct {
		store.Tenant
		PermissionIDs []uint `json:"permission_ids"`
	}
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tenant.Name = update.Tenant.Name
	tenant.Contact = update.Tenant.Contact
	tenant.Phone = update.Tenant.Phone
	tenant.Email = update.Tenant.Email
	tenant.Description = update.Tenant.Description
	tenant.Logo = update.Tenant.Logo
	tenant.LoginSuffix = update.Tenant.LoginSuffix
	tenant.Status = update.Tenant.Status
	tenant.MaxUsers = update.Tenant.MaxUsers
	tenant.MaxDevices = update.Tenant.MaxDevices
	tenant.ExpiresAt = update.Tenant.ExpiresAt

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&tenant).Error; err != nil {
			return err
		}
		if update.PermissionIDs != nil {
			return replaceTenantPermissionLimit(tx, tenant.ID, update.PermissionIDs)
		}
		return nil
	})
	if err != nil {
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
		tx.Unscoped().Where("tenant_id = ?", id).Delete(&store.ScopePermissionLimit{})
		tx.Where("tenant_id = ?", id).Delete(&store.AuditLog{})
		return tx.Delete(&store.Tenant{}, id).Error
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete tenant: " + err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}

func (s *Server) handleGetTenantPermissionOptions(r *ghttp.Request) {
	var permissions []store.Permission
	if err := tenantPermissionOptionQuery(store.DB).Order("module asc, sort_order asc, code asc").Find(&permissions).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch permissions"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": permissions, "total": len(permissions)})
}

func (s *Server) handleGetTenantUsers(r *ghttp.Request) {
	id := r.Get("id").Uint()
	if id == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid tenant ID"})
		return
	}

	var users []store.User
	if err := store.DB.Where("tenant_id = ?", id).Order("created_at desc").Find(&users).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch users"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": users})
}

func (s *Server) handleChangeTenantAdmin(r *ghttp.Request) {
	id := r.Get("id").Uint()
	if id == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid tenant ID"})
		return
	}

	var req struct {
		UserID uint `json:"user_id"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil || req.UserID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid request: user_id is required"})
		return
	}

	// Verify target user exists and belongs to this tenant
	var targetUser store.User
	if err := store.DB.Where("id = ? AND tenant_id = ?", req.UserID, id).First(&targetUser).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found in this tenant"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		// Tenant admin is a system builtin role; the binding carries tenant scope.
		var tenantAdminRole store.Role
		if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error; err != nil {
			return err
		}

		// Find current admin binding
		var currentBinding store.UserRoleBinding
		currentErr := tx.Where("role_id = ? AND tenant_id = ? AND project_id = ?", tenantAdminRole.ID, id, 0).First(&currentBinding).Error

		// If target user is already the admin, return error
		if currentErr == nil && currentBinding.UserID == req.UserID {
			return fmt.Errorf("user is already the tenant admin")
		}

		// Remove old admin binding
		if currentErr == nil {
			if err := tx.Delete(&currentBinding).Error; err != nil {
				return err
			}
			// Downgrade old admin's user role
			tx.Model(&store.User{}).Where("id = ?", currentBinding.UserID).Update("role", "viewer")
		}

		// Create new admin binding
		newBinding := store.UserRoleBinding{
			UserID:    req.UserID,
			RoleID:    tenantAdminRole.ID,
			TenantID:  id,
			ProjectID: 0,
		}
		if err := tx.Create(&newBinding).Error; err != nil {
			return err
		}

		// Update new admin's user role
		tx.Model(&store.User{}).Where("id = ?", req.UserID).Update("role", "admin")

		return nil
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Tenant admin changed successfully"})
}

func (s *Server) handleTransferTenantAdmin(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.IsTenantAdmin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Only tenant admin can transfer admin role"})
		return
	}

	var req struct {
		TargetUserID uint `json:"target_user_id"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil || req.TargetUserID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid request: target_user_id is required"})
		return
	}

	tenantID := authCtx.TenantID

	// Verify target user exists and belongs to the same tenant
	var targetUser store.User
	if err := store.DB.Where("id = ? AND tenant_id = ?", req.TargetUserID, tenantID).First(&targetUser).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found in your tenant"})
		return
	}

	// Cannot transfer to yourself
	if req.TargetUserID == authCtx.UserID {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Cannot transfer admin to yourself"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		// Tenant admin is a system builtin role; the binding carries tenant scope.
		var tenantAdminRole store.Role
		if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error; err != nil {
			return err
		}

		// Remove current admin binding
		if err := tx.Where("user_id = ? AND role_id = ? AND tenant_id = ?", authCtx.UserID, tenantAdminRole.ID, tenantID).Delete(&store.UserRoleBinding{}).Error; err != nil {
			return err
		}

		// Create new admin binding for target user
		newBinding := store.UserRoleBinding{
			UserID:    req.TargetUserID,
			RoleID:    tenantAdminRole.ID,
			TenantID:  tenantID,
			ProjectID: 0,
		}
		if err := tx.Create(&newBinding).Error; err != nil {
			return err
		}

		// Update user roles
		tx.Model(&store.User{}).Where("id = ?", authCtx.UserID).Update("role", "operator")
		tx.Model(&store.User{}).Where("id = ?", req.TargetUserID).Update("role", "admin")

		return nil
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Admin role transferred successfully"})
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

	var tenantAdminRole store.Role
	if err := store.DB.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Tenant admin role not found"})
		return
	}

	var adminUser store.User
	if err := store.DB.
		Joins("JOIN user_role_bindings ON user_role_bindings.user_id = users.id").
		Where("users.tenant_id = ? AND user_role_bindings.role_id = ? AND user_role_bindings.tenant_id = ? AND user_role_bindings.project_id = ?", id, tenantAdminRole.ID, id, 0).
		First(&adminUser).Error; err != nil {
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
