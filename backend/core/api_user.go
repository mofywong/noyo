package core

import (
	"encoding/json"
	"fmt"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"

	"noyo/core/store"
	"noyo/core/utils"
)

// getManagedProjectIDs returns a list of project IDs where the user has the project_admin role
func (s *Server) getManagedProjectIDs(userID uint) []uint {
	var bindings []store.UserRoleBinding
	store.DB.Where("user_id = ?", userID).Find(&bindings)

	var managed []uint
	for _, b := range bindings {
		var role store.Role
		if err := store.DB.First(&role, b.RoleID).Error; err == nil {
			if role.Code == "project_admin" {
				if b.ProjectID > 0 {
					managed = append(managed, b.ProjectID)
				} else {
					var projects []store.Project
					store.DB.Where("tenant_id = ?", b.TenantID).Find(&projects)
					for _, p := range projects {
						managed = append(managed, p.ID)
					}
				}
			}
		}
	}
	return managed
}

func getTenantAdminRole(tx *gorm.DB) (store.Role, error) {
	var tenantAdminRole store.Role
	err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeTenantAdmin, 0, 0).First(&tenantAdminRole).Error
	return tenantAdminRole, err
}

func userHasTenantAdminBinding(tx *gorm.DB, userID, tenantID uint) (bool, error) {
	tenantAdminRole, err := getTenantAdminRole(tx)
	if err != nil {
		return false, err
	}

	var count int64
	err = tx.Model(&store.UserRoleBinding{}).
		Joins("JOIN users ON users.id = user_role_bindings.user_id").
		Where("user_role_bindings.user_id = ? AND user_role_bindings.tenant_id = ? AND user_role_bindings.project_id = ? AND user_role_bindings.role_id = ? AND users.deleted_at IS NULL",
			userID, tenantID, 0, tenantAdminRole.ID).
		Count(&count).Error
	return count > 0, err
}

func countTenantAdminBindings(tx *gorm.DB, tenantID uint) (int64, error) {
	tenantAdminRole, err := getTenantAdminRole(tx)
	if err != nil {
		return 0, err
	}

	var count int64
	err = tx.Model(&store.UserRoleBinding{}).
		Joins("JOIN users ON users.id = user_role_bindings.user_id").
		Where("user_role_bindings.tenant_id = ? AND user_role_bindings.project_id = ? AND user_role_bindings.role_id = ? AND users.deleted_at IS NULL",
			tenantID, 0, tenantAdminRole.ID).
		Distinct("user_role_bindings.user_id").
		Count(&count).Error
	return count, err
}

func replaceUserTenantRoleBindings(tx *gorm.DB, authCtx *AuthContext, user store.User, roleIDs []uint) error {
	if authCtx == nil || !authCtx.IsTenantAdmin || user.TenantID != authCtx.TenantID {
		return fmt.Errorf("access denied")
	}

	tenantAdminRole, err := getTenantAdminRole(tx)
	if err != nil {
		return err
	}

	nextHasTenantAdmin := false
	validRoleIDs := make([]uint, 0, len(roleIDs))
	seenRoleIDs := make(map[uint]bool)
	for _, roleID := range roleIDs {
		if roleID == 0 || seenRoleIDs[roleID] {
			continue
		}
		seenRoleIDs[roleID] = true

		var roleToAssign store.Role
		if err := tx.First(&roleToAssign, roleID).Error; err != nil || !authCtx.CanAssignRole(roleToAssign, 0) {
			return fmt.Errorf("invalid role assignment")
		}
		if roleToAssign.ID == tenantAdminRole.ID {
			nextHasTenantAdmin = true
		}
		validRoleIDs = append(validRoleIDs, roleID)
	}

	currentHasTenantAdmin, err := userHasTenantAdminBinding(tx, user.ID, user.TenantID)
	if err != nil {
		return err
	}
	if currentHasTenantAdmin && !nextHasTenantAdmin {
		adminCount, err := countTenantAdminBindings(tx, user.TenantID)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return fmt.Errorf("at least one tenant administrator is required")
		}
	}

	if err := tx.Unscoped().Where("user_id = ? AND project_id = ?", user.ID, 0).Delete(&store.UserRoleBinding{}).Error; err != nil {
		return err
	}
	for _, roleID := range validRoleIDs {
		if err := tx.Create(&store.UserRoleBinding{UserID: user.ID, RoleID: roleID, TenantID: user.TenantID, ProjectID: 0}).Error; err != nil {
			return err
		}
	}

	nextRole := "viewer"
	if nextHasTenantAdmin {
		nextRole = "admin"
	} else if user.Role != "admin" {
		nextRole = user.Role
	}
	return tx.Model(&store.User{}).Where("id = ?", user.ID).Update("role", nextRole).Error
}

func validateUserDeletion(tx *gorm.DB, authCtx *AuthContext, user store.User, currentUserID uint) error {
	if currentUserID == user.ID {
		return fmt.Errorf("cannot delete your own account")
	}
	if authCtx == nil || user.TenantID != authCtx.TenantID || !authCtx.CanManageTenantResource(user.TenantID) {
		return fmt.Errorf("access denied")
	}

	hasTenantAdmin, err := userHasTenantAdminBinding(tx, user.ID, user.TenantID)
	if err != nil {
		return err
	}
	if hasTenantAdmin {
		adminCount, err := countTenantAdminBindings(tx, user.TenantID)
		if err != nil {
			return err
		}
		if adminCount <= 1 {
			return fmt.Errorf("at least one tenant administrator is required")
		}
	}
	return nil
}

// handleListUsers returns a paginated list of users
func (s *Server) handleListUsers(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 10).Int()

	tenantID := r.GetCtxVar("tenant_id").Uint()
	roleID := r.Get("role_id").Uint()

	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID != tenantID {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	isProjectScoped := !authCtx.IsTenantAdmin
	allowedProjectIDs := authCtx.AllowedProjectIDs

	projectID := r.Get("project_id").Uint()
	if projectID == 0 {
		projectID = r.GetCtxVar("project_id").Uint()
	}

	users, total, err := store.ListUsers(page, pageSize, tenantID, projectID, roleID, isProjectScoped, allowedProjectIDs)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	type UserProjectInfo struct {
		ProjectID   uint   `json:"project_id"`
		ProjectName string `json:"project_name"`
		RoleID      uint   `json:"role_id"`
		RoleName    string `json:"role_name"`
		RoleCode    string `json:"role_code"`
	}

	type UserRoleInfo struct {
		RoleID   uint   `json:"role_id"`
		RoleName string `json:"role_name"`
		RoleCode string `json:"role_code"`
	}

	// Remove passwords before returning
	type UserResponse struct {
		ID             uint              `json:"id"`
		Username       string            `json:"username"`
		DisplayName    string            `json:"display_name"`
		Email          string            `json:"email"`
		Role           string            `json:"role"`
		Status         int               `json:"status"`
		IsSystemAdmin  bool              `json:"is_system_admin"`
		IsTenantAdmin  bool              `json:"is_tenant_admin"`
		IsProjectAdmin bool              `json:"is_project_admin"`
		CreatedAt      string            `json:"created_at"`
		LastLoginAt    string            `json:"last_login_at,omitempty"`
		Projects       []UserProjectInfo `json:"projects"`
		TenantRoles    []UserRoleInfo    `json:"tenant_roles"`
	}

	var res []UserResponse
	var userIDs []uint
	for _, u := range users {
		userIDs = append(userIDs, u.ID)
	}

	type BindingResult struct {
		UserID      uint
		RoleID      uint
		RoleName    string
		RoleCode    string
		ProjectID   uint
		ProjectName string
	}
	var allBindings []BindingResult

	if len(userIDs) > 0 {
		store.DB.Model(&store.UserRoleBinding{}).
			Select("user_role_bindings.user_id, user_role_bindings.role_id, roles.name as role_name, roles.code as role_code, user_role_bindings.project_id, projects.name as project_name").
			Joins("LEFT JOIN roles ON roles.id = user_role_bindings.role_id").
			Joins("LEFT JOIN projects ON projects.id = user_role_bindings.project_id").
			Where("user_role_bindings.user_id IN ?", userIDs).
			Scan(&allBindings)
	}

	userProjsMap := make(map[uint][]UserProjectInfo)
	userRolesMap := make(map[uint][]UserRoleInfo)
	userIsSystemAdmin := make(map[uint]bool)
	userIsTenantAdmin := make(map[uint]bool)
	userIsProjectAdmin := make(map[uint]bool)

	for _, b := range allBindings {
		if b.ProjectID > 0 {
			if b.ProjectName == "" {
				continue
			}
			userProjsMap[b.UserID] = append(userProjsMap[b.UserID], UserProjectInfo{
				ProjectID:   b.ProjectID,
				ProjectName: b.ProjectName,
				RoleID:      b.RoleID,
				RoleName:    b.RoleName,
				RoleCode:    b.RoleCode,
			})
			if b.RoleCode == "project_admin" {
				userIsProjectAdmin[b.UserID] = true
			}
		} else {
			userRolesMap[b.UserID] = append(userRolesMap[b.UserID], UserRoleInfo{
				RoleID:   b.RoleID,
				RoleName: b.RoleName,
				RoleCode: b.RoleCode,
			})
			if b.RoleCode == "super_admin" {
				userIsSystemAdmin[b.UserID] = true
			}
			if b.RoleCode == "tenant_admin" {
				userIsTenantAdmin[b.UserID] = true
			}
		}
	}

	for _, u := range users {
		var lastLogin string
		if u.LastLoginAt != nil {
			lastLogin = u.LastLoginAt.Format("2006-01-02 15:04:05")
		}

		userProjs := userProjsMap[u.ID]
		if userProjs == nil {
			userProjs = []UserProjectInfo{}
		}

		userRoles := userRolesMap[u.ID]
		if userRoles == nil {
			userRoles = []UserRoleInfo{}
		}

		res = append(res, UserResponse{
			ID:             u.ID,
			Username:       u.Username,
			DisplayName:    u.DisplayName,
			Email:          u.Email,
			Role:           u.Role,
			Status:         u.Status,
			IsSystemAdmin:  userIsSystemAdmin[u.ID],
			IsTenantAdmin:  userIsTenantAdmin[u.ID],
			IsProjectAdmin: userIsProjectAdmin[u.ID],
			CreatedAt:      u.CreatedAt.Format("2006-01-02 15:04:05"),
			LastLoginAt:    lastLogin,
			Projects:       userProjs,
			TenantRoles:    userRoles,
		})
	}

	if res == nil {
		res = []UserResponse{}
	}

	r.Response.WriteJson(g.Map{
		"code":     0,
		"data":     res,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// handleCreateUser creates a new user
func (s *Server) handleCreateUser(r *ghttp.Request) {
	type CreateRequest struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Status      *int   `json:"status"`
	}

	var req CreateRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	if req.Username == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Username is required"})
		return
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID != tenantID || (!authCtx.CanManageTenantResource(tenantID) && len(authCtx.AllowedProjectIDs) == 0) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	existingUser, _ := store.GetUserByTenantAndUsername(tenantID, req.Username)
	if existingUser != nil {
		r.Response.WriteJson(g.Map{"code": 0, "message": "User exists in tenant", "data": g.Map{"id": existingUser.ID, "existing": true}})
		return
	}

	if req.Password == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Password is required for new users"})
		return
	}

	if err := validatePasswordStrength(req.Password); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}

	hashed, err := utils.HashPassword(req.Password)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to hash password"})
		return
	}

	if req.Role == "" {
		req.Role = "viewer"
	}
	if req.Role == "admin" && (authCtx == nil || !authCtx.IsTenantAdmin) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Only tenant admins can create admin users"})
		return
	}

	status := 1
	if req.Status != nil {
		status = *req.Status
	}

	newUser := store.User{
		TenantID:    tenantID,
		Username:    req.Username,
		Password:    hashed,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Role:        req.Role,
		Status:      status,
	}

	if err := store.SaveUser(&newUser); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "User created", "data": g.Map{"id": newUser.ID}})
}

// handleUpdateUser updates an existing user
func (s *Server) handleUpdateUser(r *ghttp.Request) {
	id := r.Get("id").Uint()

	type UpdateRequest struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Status      *int   `json:"status"`
	}

	var req UpdateRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	user, err := store.GetUserByID(id)
	if err != nil || user == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil || user.TenantID != authCtx.TenantID {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if !authCtx.IsTenantAdmin {
		allowedProjectIDs := authCtx.AllowedProjectIDs
		var userCount int64
		store.DB.Model(&store.UserRoleBinding{}).Where("user_id = ? AND project_id IN ?", id, allowedProjectIDs).Count(&userCount)
		if userCount == 0 {
			r.Response.WriteJson(g.Map{"code": 403, "message": "You can only update users who belong to your managed projects"})
			return
		}
	}

	user.DisplayName = req.DisplayName
	user.Email = req.Email
	if req.Role != "" {
		if req.Role == "admin" && !authCtx.IsTenantAdmin {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Only tenant admins can promote users to admin"})
			return
		}
		user.Role = req.Role
	}
	if req.Status != nil {
		user.Status = *req.Status
	}

	if err := store.SaveUser(user); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "User updated"})
}

// handleDeleteUser hard deletes a user
func (s *Server) handleDeleteUser(r *ghttp.Request) {
	id := r.Get("id").Uint()

	user, err := store.GetUserByID(id)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}

	authCtx := requestAuthContext(r)
	if err := validateUserDeletion(store.DB, authCtx, *user, r.GetCtxVar("user_id").Uint()); err != nil {
		statusCode := 403
		if err.Error() == "cannot delete your own account" {
			statusCode = 400
		}
		r.Response.WriteJson(g.Map{"code": statusCode, "message": err.Error()})
		return
	}

	if err := store.DeleteUser(id); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "User deleted"})
}

// handleResetPassword admin action to reset a user's password
func (s *Server) handleResetPassword(r *ghttp.Request) {
	id := r.Get("id").Uint()

	type ResetRequest struct {
		NewPassword string `json:"new_password"`
	}

	var req ResetRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	if req.NewPassword == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "New password is required"})
		return
	}

	user, err := store.GetUserByID(id)
	if err != nil || user == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || user.TenantID != authCtx.TenantID {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	authCtx := requestAuthContext(r)
	if !authCtx.IsTenantAdmin {
		allowedProjectIDs := authCtx.AllowedProjectIDs
		var userCount int64
		store.DB.Model(&store.UserRoleBinding{}).Where("user_id = ? AND project_id IN ?", id, allowedProjectIDs).Count(&userCount)
		if userCount == 0 {
			r.Response.WriteJson(g.Map{"code": 403, "message": "You can only reset password for users in your managed projects"})
			return
		}
	}

	hashed, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to hash password"})
		return
	}

	user.Password = hashed
	if err := store.SaveUser(user); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "User password reset successfully"})
}

func (s *Server) handleGetUserPositions(r *ghttp.Request) {
	userID := r.Get("id").Uint()
	user, err := store.GetUserByID(userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || user.TenantID != authCtx.TenantID || !authCtx.CanManageTenantResource(user.TenantID) && !authCtx.IsProjectAdmin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	var mappings []store.UserPosition
	if err := store.DB.Where("user_id = ?", userID).Find(&mappings).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to get user positions"})
		return
	}
	positionIDs := make([]uint, 0)
	for _, m := range mappings {
		positionIDs = append(positionIDs, m.PositionID)
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": positionIDs})
}

func (s *Server) handleSetUserPositions(r *ghttp.Request) {
	userID := r.Get("id").Uint()
	user, err := store.GetUserByID(userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil || user.TenantID != authCtx.TenantID || !authCtx.CanManageTenantResource(user.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	var req struct {
		PositionIDs []uint `json:"position_ids"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tx := store.DB.Begin()
	tx.Where("user_id = ?", userID).Delete(&store.UserPosition{})
	for _, pid := range req.PositionIDs {
		var position store.Position
		if err := tx.Where("id = ? AND tenant_id = ?", pid, user.TenantID).First(&position).Error; err != nil {
			tx.Rollback()
			r.Response.WriteJson(g.Map{"code": 403, "message": "Invalid position"})
			return
		}
		tx.Create(&store.UserPosition{UserID: userID, PositionID: pid})
	}
	if err := tx.Commit().Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update positions"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Updated successfully"})
}

func (s *Server) handleGetUserRoles(r *ghttp.Request) {
	userID := r.Get("id").Uint()
	user, err := store.GetUserByID(userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || user.TenantID != authCtx.TenantID || !authCtx.CanManageTenantResource(user.TenantID) && !authCtx.IsProjectAdmin {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx != nil && !authCtx.IsTenantAdmin {
		r.Response.WriteJson(g.Map{"code": 0, "data": []uint{}})
		return
	}
	var mappings []store.UserRoleBinding
	if err := store.DB.Where("user_id = ? AND project_id = 0", userID).Find(&mappings).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to get user roles"})
		return
	}
	roleIDs := make([]uint, 0)
	for _, m := range mappings {
		roleIDs = append(roleIDs, m.RoleID)
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": roleIDs})
}

func (s *Server) handleSetUserRoles(r *ghttp.Request) {
	userID := r.Get("id").Uint()
	var req struct {
		RoleIDs []uint `json:"role_ids"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if !authCtx.IsTenantAdmin {
		if len(req.RoleIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 0, "message": "Updated successfully"})
			return
		}
		r.Response.WriteJson(g.Map{"code": 403, "message": "Only tenant admins can assign tenant roles"})
		return
	}

	user, err := store.GetUserByID(userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	if user.TenantID != authCtx.TenantID || !authCtx.CanManageTenantResource(user.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	if err := store.DB.Transaction(func(tx *gorm.DB) error {
		return replaceUserTenantRoleBindings(tx, authCtx, *user, req.RoleIDs)
	}); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Failed to update roles: " + err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Updated successfully"})
}

func (s *Server) handleGetUserProjects(r *ghttp.Request) {
	userID := r.Get("id").Uint()
	user, err := store.GetUserByID(userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil || user.TenantID != authCtx.TenantID || (!authCtx.CanManageTenantResource(user.TenantID) && len(authCtx.AllowedProjectIDs) == 0) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	var mappings []store.UserRoleBinding
	db := store.DB.Where("user_id = ? AND project_id > 0", userID)
	if !authCtx.IsTenantAdmin {
		db = db.Where("project_id IN ?", authCtx.AllowedProjectIDs)
	}
	if err := db.Find(&mappings).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to get user projects"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": mappings})
}

func (s *Server) handleSetUserProjects(r *ghttp.Request) {
	userID := r.Get("id").Uint()
	type ProjectRoleReq struct {
		ProjectID uint `json:"project_id"`
		RoleID    uint `json:"role_id"`
	}
	var req struct {
		Projects []ProjectRoleReq `json:"projects"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	user, err := store.GetUserByID(userID)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "User not found"})
		return
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil || user.TenantID != authCtx.TenantID {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if !authCtx.IsTenantAdmin && len(authCtx.AllowedProjectIDs) == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied: admin role required"})
		return
	}

	tx := store.DB.Begin()

	if !authCtx.IsTenantAdmin {
		allowedProjectIDs := authCtx.AllowedProjectIDs
		allowedMap := make(map[uint]bool)
		for _, pid := range allowedProjectIDs {
			allowedMap[pid] = true
		}
		for _, p := range req.Projects {
			if !allowedMap[p.ProjectID] {
				tx.Rollback()
				r.Response.WriteJson(g.Map{"code": 403, "message": "You can only assign users to projects you manage"})
				return
			}
		}

		if len(allowedProjectIDs) > 0 {
			tx.Unscoped().Where("user_id = ? AND project_id IN ?", userID, allowedProjectIDs).Delete(&store.UserRoleBinding{})
		}
	} else {
		tx.Unscoped().Where("user_id = ? AND project_id > 0", userID).Delete(&store.UserRoleBinding{})
	}

	for _, p := range req.Projects {
		var project store.Project
		if err := tx.Where("id = ? AND tenant_id = ?", p.ProjectID, user.TenantID).First(&project).Error; err != nil || !authCtx.CanManageProject(p.ProjectID) {
			tx.Rollback()
			r.Response.WriteJson(g.Map{"code": 403, "message": "Invalid project assignment"})
			return
		}
		var roleToAssign store.Role
		if err := tx.First(&roleToAssign, p.RoleID).Error; err != nil || !authCtx.CanAssignRole(roleToAssign, p.ProjectID) {
			tx.Rollback()
			r.Response.WriteJson(g.Map{"code": 403, "message": "Invalid role assignment"})
			return
		}
		tx.Create(&store.UserRoleBinding{UserID: userID, ProjectID: p.ProjectID, RoleID: p.RoleID, TenantID: user.TenantID})
	}
	if err := tx.Commit().Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update projects"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Updated successfully"})
}
