package core

import (
	"encoding/json"
	"noyo/core/store"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

func (s *Server) handleGetProjects(r *ghttp.Request) {
	tenantID := r.GetCtxVar("tenant_id").Uint()
	userID := r.GetCtxVar("user_id").Uint()
	userRole := r.GetCtxVar("role").String()

	db := store.DB.Model(&store.Project{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}

	keyword := r.Get("keyword").String()
	if keyword != "" {
		db = db.Where("name LIKE ? OR code LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}

	if tenantID > 0 && userRole != "admin" {
		var bindings []store.UserRoleBinding
		if err := store.DB.Where("user_id = ?", userID).Find(&bindings).Error; err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to query user projects"})
			return
		}

		projectIDs := make([]uint, 0)
		hasGlobal := false
		for _, b := range bindings {
			if b.ProjectID == 0 {
				hasGlobal = true
				break
			}
			projectIDs = append(projectIDs, b.ProjectID)
		}

		if !hasGlobal {
			if len(projectIDs) == 0 {
				r.Response.WriteJson(g.Map{
					"code":  0,
					"data":  []store.Project{},
					"total": 0,
				})
				return
			}
			db = db.Where("id IN (?)", projectIDs)
		}
	}

	var projects []store.Project
	if err := db.Find(&projects).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch projects"})
		return
	}

	type ProjectResponse struct {
		store.Project
		Admins string `json:"admins"`
	}

	var res []ProjectResponse
	for _, p := range projects {
		var adminNames []string
		store.DB.Table("users").
			Select("CASE WHEN users.display_name != '' AND users.display_name IS NOT NULL THEN users.display_name ELSE users.username END as name").
			Joins("JOIN user_role_bindings ON users.id = user_role_bindings.user_id").
			Joins("JOIN roles ON user_role_bindings.role_id = roles.id").
			Where("user_role_bindings.project_id = ? AND roles.code = ?", p.ID, "project_admin").
			Where("users.id NOT IN (SELECT user_id FROM user_role_bindings JOIN roles r2 ON user_role_bindings.role_id = r2.id WHERE r2.code = 'tenant_admin')").
			Pluck("name", &adminNames)

		adminsStr := ""
		if len(adminNames) > 0 {
			adminsStr = strings.Join(adminNames, "、")
		}
		res = append(res, ProjectResponse{Project: p, Admins: adminsStr})
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": res, "total": len(res)})
}

func (s *Server) handleCreateProject(r *ghttp.Request) {
	var req struct {
		store.Project
		AdminUserID uint `json:"admin_user_id"`
	}

	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	if req.AdminUserID == 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Project administrator is required"})
		return
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 {
		req.Project.TenantID = tenantID
	}
	if authCtx := requestAuthContext(r); authCtx != nil && !authCtx.CanManageTenantResource(req.Project.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		var adminUser store.User
		if err := tx.Where("id = ? AND tenant_id = ?", req.AdminUserID, req.Project.TenantID).First(&adminUser).Error; err != nil {
			return err
		}
		if err := tx.Create(&req.Project).Error; err != nil {
			return err
		}

		// 1. Find tenant-specific project_admin Role
		var projectRole store.Role
		if err := tx.Where("code = ? AND tenant_id = ?", "project_admin", req.Project.TenantID).First(&projectRole).Error; err != nil {
			return err
		}

		// 2. Map user to project and role
		userProject := store.UserRoleBinding{
			UserID:    req.AdminUserID,
			ProjectID: req.Project.ID,
			RoleID:    projectRole.ID,
			TenantID:  req.Project.TenantID,
		}
		if err := tx.Create(&userProject).Error; err != nil {
			return err
		}

		return nil
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

	var update store.Project
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	project.Name = update.Name
	project.Description = update.Description
	project.Status = update.Status

	if err := store.DB.Save(&project).Error; err != nil {
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
	if err := store.DB.Delete(&store.Project{}, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete project"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}
