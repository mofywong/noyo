package core

import (
	"encoding/json"
	"noyo/core/store"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

type ProjectAdminSummary struct {
	Names   string
	UserIDs []uint
	UserID  uint
}

func loadProjectAdminSummary(db *gorm.DB, projectID uint) (ProjectAdminSummary, error) {
	type adminRow struct {
		UserID uint
		Name   string
	}

	var rows []adminRow
	if err := db.Table("users").
		Select("users.id AS user_id, CASE WHEN users.display_name != '' AND users.display_name IS NOT NULL THEN users.display_name ELSE users.username END AS name").
		Joins("JOIN user_role_bindings ON users.id = user_role_bindings.user_id").
		Joins("JOIN roles ON user_role_bindings.role_id = roles.id").
		Where("user_role_bindings.project_id = ? AND roles.code = ?", projectID, RoleCodeProjectAdmin).
		Group("users.id, users.display_name, users.username").
		Order("users.id ASC").
		Scan(&rows).Error; err != nil {
		return ProjectAdminSummary{}, err
	}

	names := make([]string, 0, len(rows))
	userIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		names = append(names, row.Name)
		userIDs = append(userIDs, row.UserID)
	}

	summary := ProjectAdminSummary{
		Names:   strings.Join(names, ", "),
		UserIDs: userIDs,
	}
	if len(userIDs) > 0 {
		summary.UserID = userIDs[0]
	}
	return summary, nil
}

func replaceProjectAdmin(tx *gorm.DB, tenantID, projectID, adminUserID uint) error {
	var adminUser store.User
	if err := tx.Where("id = ? AND tenant_id = ?", adminUserID, tenantID).First(&adminUser).Error; err != nil {
		return err
	}

	var projectAdminRole store.Role
	if err := tx.Where("code = ? AND tenant_id = ? AND project_id = ?", RoleCodeProjectAdmin, 0, 0).First(&projectAdminRole).Error; err != nil {
		return err
	}

	var projectAdminRoleIDs []uint
	if err := tx.Model(&store.Role{}).Where("code = ?", RoleCodeProjectAdmin).Pluck("id", &projectAdminRoleIDs).Error; err != nil {
		return err
	}
	if len(projectAdminRoleIDs) > 0 {
		if err := tx.Unscoped().Where("project_id = ? AND role_id IN ?", projectID, projectAdminRoleIDs).Delete(&store.UserRoleBinding{}).Error; err != nil {
			return err
		}
	}

	return tx.Create(&store.UserRoleBinding{
		UserID:    adminUserID,
		ProjectID: projectID,
		RoleID:    projectAdminRole.ID,
		TenantID:  tenantID,
	}).Error
}

func (s *Server) handleGetProjects(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	db := store.DB.Model(&store.Project{}).Where("tenant_id = ?", authCtx.TenantID)
	if projectIDs, restricted := authCtx.ProjectIDsForTenantQuery(); restricted {
		if len(projectIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 0, "data": []store.Project{}, "total": 0})
			return
		}
		db = db.Where("id IN ?", projectIDs)
	}

	keyword := r.Get("keyword").String()
	if keyword != "" {
		db = db.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	var projects []store.Project
	if err := db.Find(&projects).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch projects"})
		return
	}

	type ProjectResponse struct {
		store.Project
		Admins        string `json:"admins"`
		AdminUserIDs  []uint `json:"admin_user_ids"`
		AdminUserID   uint   `json:"admin_user_id"`
		PermissionIDs []uint `json:"permission_ids"`
	}

	res := make([]ProjectResponse, 0, len(projects))
	for _, p := range projects {
		adminSummary, err := loadProjectAdminSummary(store.DB, p.ID)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch project administrators"})
			return
		}
		permissionIDs, err := loadScopePermissionLimitIDs(store.DB, permissionLimitScopeProject, p.TenantID, p.ID)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch project permission limits"})
			return
		}
		res = append(res, ProjectResponse{
			Project:       p,
			Admins:        adminSummary.Names,
			AdminUserIDs:  adminSummary.UserIDs,
			AdminUserID:   adminSummary.UserID,
			PermissionIDs: permissionIDs,
		})
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": res, "total": len(res)})
}

func (s *Server) handleGetAccessibleProjects(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil || authCtx.TenantID == 0 || authCtx.IsSystemAdmin {
		r.Response.WriteJson(g.Map{"code": 0, "data": []store.Project{}, "total": 0})
		return
	}

	db := store.DB.Model(&store.Project{}).Where("tenant_id = ?", authCtx.TenantID)
	if projectIDs, restricted := authCtx.ProjectIDsForTenantQuery(); restricted {
		if len(projectIDs) == 0 {
			r.Response.WriteJson(g.Map{"code": 0, "data": []store.Project{}, "total": 0})
			return
		}
		db = db.Where("id IN ?", projectIDs)
	}

	var projects []store.Project
	if err := db.Order("created_at desc").Find(&projects).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch projects"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": projects, "total": len(projects)})
}

func (s *Server) handleGetProjectPermissionOptions(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.IsTenantAdmin || (!authCtx.HasPermission("project:create") && !authCtx.HasPermission("project:edit")) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var permissions []store.Permission
	if err := store.DB.Model(&store.Permission{}).
		Where(
			"id IN (?)",
			store.DB.Model(&store.ScopePermissionLimit{}).
				Select("permission_id").
				Where("scope_type = ? AND tenant_id = ? AND project_id = ?", permissionLimitScopeTenant, authCtx.TenantID, 0),
		).
		Order("module asc, sort_order asc, code asc").
		Find(&permissions).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch permissions"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": permissions, "total": len(permissions)})
}

func (s *Server) handleCreateProject(r *ghttp.Request) {
	var req struct {
		store.Project
		AdminUserID   uint   `json:"admin_user_id"`
		PermissionIDs []uint `json:"permission_ids"`
	}

	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	if req.AdminUserID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Project administrator is required"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanManageTenantResource(authCtx.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	req.Project.TenantID = authCtx.TenantID

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&req.Project).Error; err != nil {
			return err
		}

		if err := replaceProjectPermissionLimit(tx, req.Project.TenantID, req.Project.ID, req.PermissionIDs); err != nil {
			return err
		}
		return replaceProjectAdmin(tx, req.Project.TenantID, req.Project.ID, req.AdminUserID)
	})

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to create project: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": req.Project})
}

func (s *Server) handleUpdateProject(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var project store.Project
	if err := store.DB.First(&project, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Project not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || project.TenantID != authCtx.TenantID || !authCtx.CanManageProject(project.ID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var update struct {
		store.Project
		AdminUserID   uint   `json:"admin_user_id"`
		PermissionIDs []uint `json:"permission_ids"`
	}
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	var rawUpdate map[string]json.RawMessage
	if err := json.Unmarshal(r.GetBody(), &rawUpdate); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	hasProjectField := func(field string) bool {
		_, ok := rawUpdate[field]
		return ok
	}
	if hasProjectField("name") {
		project.Name = update.Name
	}
	if hasProjectField("description") {
		project.Description = update.Description
	}

	authCtx := requestAuthContext(r)
	if update.AdminUserID > 0 && (authCtx == nil || !authCtx.IsTenantAdmin) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Only tenant admins can change project administrators"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&project).Error; err != nil {
			return err
		}
		if update.AdminUserID > 0 {
			if err := replaceProjectAdmin(tx, project.TenantID, project.ID, update.AdminUserID); err != nil {
				return err
			}
		}
		if update.PermissionIDs != nil {
			return replaceProjectPermissionLimit(tx, project.TenantID, project.ID, update.PermissionIDs)
		}
		return nil
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update project: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": project})
}

func (s *Server) handleDeleteProject(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var project store.Project
	if err := store.DB.First(&project, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Project not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || project.TenantID != authCtx.TenantID || !authCtx.CanManageProject(project.ID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	err := store.DB.Transaction(func(tx *gorm.DB) error {
		return deleteProjectCascade(tx, project)
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete project"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}
