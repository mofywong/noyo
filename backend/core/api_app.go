package core

import (
	"encoding/json"
	"noyo/core/store"
	"noyo/core/utils"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
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

	var update store.App
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	app.Name = update.Name
	app.Description = update.Description
	app.RateLimit = update.RateLimit

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

	if err := store.DB.Unscoped().Delete(&app).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete app"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
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
