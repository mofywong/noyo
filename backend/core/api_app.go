package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"
	"noyo/core/utils"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *Server) handleListApps(r *ghttp.Request) {
	tenantID := r.GetCtxVar("tenant_id").Uint()

	db := store.DB.Model(&store.App{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}

	var apps []store.App
	if err := db.Find(&apps).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch apps"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": apps, "total": len(apps)})
}

func (s *Server) handleCreateApp(r *ghttp.Request) {
	var app store.App
	if err := json.Unmarshal(r.GetBody(), &app); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 {
		app.TenantID = tenantID
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	// Auto generate AppID and AppKey (key is hashed before storage)
	app.AppID = strings.ReplaceAll(uuid.New().String(), "-", "")
	if app.Status == 0 {
		app.Status = 1
	}
	rawAppKey := strings.ReplaceAll(uuid.New().String(), "-", "")
	hashedKey, err := utils.HashPassword(rawAppKey)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate app key"})
		return
	}
	app.AppKey = hashedKey

	if err := store.DB.Create(&app).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to create app: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"ID":          app.ID,
		"app_id":      app.AppID,
		"AppKey":      rawAppKey, // Return the raw key 鈥?this is the only time it's visible
		"name":        app.Name,
		"description": app.Description,
		"status":      app.Status,
		"rate_limit":  app.RateLimit,
		"CreatedAt":   app.CreatedAt,
	}})
}

func (s *Server) handleUpdateApp(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var update struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Status      *int   `json:"status"`
		RateLimit   int    `json:"rate_limit"`
	}
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	app.Name = update.Name
	app.Description = update.Description
	app.RateLimit = update.RateLimit
	if update.Status != nil {
		app.Status = *update.Status
	}

	if err := store.DB.Save(&app).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update app: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": app})
}

func (s *Server) handleDeleteApp(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("app_id = ?", app.ID).Delete(&store.AppRole{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&app).Error
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete app"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}

type appRoleAssignmentInput struct {
	ProjectID uint `json:"project_id"`
	RoleID    uint `json:"role_id"`
}

func (s *Server) handleGetAppRoles(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var appRoles []store.AppRole
	if err := store.DB.Where("app_id = ? AND tenant_id = ?", app.ID, app.TenantID).Find(&appRoles).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch app roles"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": appRoles, "total": len(appRoles)})
}

func (s *Server) handleSetAppRoles(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}

	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanManageTenantResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var req struct {
		Roles []appRoleAssignmentInput `json:"roles"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	err := store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("app_id = ?", app.ID).Delete(&store.AppRole{}).Error; err != nil {
			return err
		}
		seen := make(map[string]bool, len(req.Roles))
		for _, assignment := range req.Roles {
			if assignment.RoleID == 0 {
				continue
			}
			if assignment.ProjectID > 0 && !projectBelongsToTenant(assignment.ProjectID, app.TenantID) {
				return fmt.Errorf("project is outside tenant scope")
			}
			var role store.Role
			if err := tx.Where("id = ? AND (tenant_id = ? OR tenant_id = 0)", assignment.RoleID, app.TenantID).First(&role).Error; err != nil {
				return fmt.Errorf("role is outside tenant scope")
			}
			if !authCtx.CanAssignRole(role, assignment.ProjectID) {
				return fmt.Errorf("role cannot be assigned to this app scope")
			}
			key := fmt.Sprintf("%d:%d", assignment.ProjectID, assignment.RoleID)
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := tx.Create(&store.AppRole{
				AppID:     app.ID,
				RoleID:    role.ID,
				TenantID:  app.TenantID,
				ProjectID: assignment.ProjectID,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Failed to update app roles: " + err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "App roles updated successfully"})
}

func (s *Server) handleResetAppKey(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var app store.App
	if err := store.DB.First(&app, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "App not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(app.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	rawAppKey := strings.ReplaceAll(uuid.New().String(), "-", "")
	hashedKey, err := utils.HashPassword(rawAppKey)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to generate app key"})
		return
	}
	app.AppKey = hashedKey
	if err := store.DB.Save(&app).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to reset app key"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{
		"ID":     app.ID,
		"app_id": app.AppID,
		"AppKey": rawAppKey, // Return the raw key 鈥?this is the only time it's visible
	}, "message": "Key reset successfully"})
}
