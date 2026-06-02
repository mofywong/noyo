package core

import (
	"encoding/json"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *Server) handleListPositions(r *ghttp.Request) {
	tenantID := r.GetCtxVar("tenant_id").Uint()

	db := store.DB.Model(&store.Position{})
	if tenantID > 0 {
		db = db.Where("tenant_id = ?", tenantID)
	}

	var positions []store.Position
	if err := db.Find(&positions).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch positions"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": positions, "total": len(positions)})
}

func (s *Server) handleCreatePosition(r *ghttp.Request) {
	var pos store.Position
	if err := json.Unmarshal(r.GetBody(), &pos); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID > 0 {
		pos.TenantID = tenantID
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(pos.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var count int64
	store.DB.Model(&store.Position{}).Where("tenant_id = ? AND code = ?", pos.TenantID, pos.Code).Count(&count)
	if count > 0 {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Position code already exists"})
		return
	}

	if err := store.DB.Create(&pos).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to create position: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": pos})
}

func (s *Server) handleUpdatePosition(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var pos store.Position
	if err := store.DB.First(&pos, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Position not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(pos.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	var update store.Position
	if err := json.Unmarshal(r.GetBody(), &update); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	pos.Name = update.Name
	pos.Description = update.Description
	pos.Status = update.Status

	if err := store.DB.Save(&pos).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update position: " + err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "data": pos})
}

func (s *Server) handleDeletePosition(r *ghttp.Request) {
	id := r.Get("id").Uint()
	var pos store.Position
	if err := store.DB.First(&pos, id).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Position not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(pos.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	if err := store.DB.Unscoped().Delete(&pos).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to delete position"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Deleted successfully"})
}

func (s *Server) handleGetPositionRoles(r *ghttp.Request) {
	posID := r.Get("id").Uint()
	var pos store.Position
	if err := store.DB.First(&pos, posID).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Position not found"})
		return
	}
	if authCtx := requestAuthContext(r); authCtx == nil || !authCtx.CanManageTenantResource(pos.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	var mappings []store.PositionRole
	if err := store.DB.Where("position_id = ?", posID).Find(&mappings).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to get position roles"})
		return
	}
	roleIDs := make([]uint, 0)
	for _, m := range mappings {
		roleIDs = append(roleIDs, m.RoleID)
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": roleIDs})
}

func (s *Server) handleSetPositionRoles(r *ghttp.Request) {
	posID := r.Get("id").Uint()
	var pos store.Position
	if err := store.DB.First(&pos, posID).Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Position not found"})
		return
	}
	authCtx := requestAuthContext(r)
	if authCtx == nil || !authCtx.CanManageTenantResource(pos.TenantID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	var req struct {
		RoleIDs []uint `json:"role_ids"`
	}
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tx := store.DB.Begin()
	tx.Where("position_id = ?", posID).Delete(&store.PositionRole{})
	for _, rid := range req.RoleIDs {
		var role store.Role
		if err := tx.First(&role, rid).Error; err != nil || !authCtx.CanAssignRole(role, 0) {
			tx.Rollback()
			r.Response.WriteJson(g.Map{"code": 403, "message": "Invalid role assignment"})
			return
		}
		tx.Create(&store.PositionRole{PositionID: posID, RoleID: rid})
	}
	if err := tx.Commit().Error; err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to update roles"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Updated successfully"})
}
