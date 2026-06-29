package core

import (
	"encoding/json"
	"noyo/core/store"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func (s *Server) RegisterProtocolProfileRoutes(group *ghttp.RouterGroup) {
	// Let's reuse product list/edit permissions for protocol profiles for now
	permissionGET(group, "/protocol-profiles", "product:list", s.handleListProtocolProfiles)
	permissionGET(group, "/protocol-profiles/:code", "product:list", s.handleGetProtocolProfile)
	permissionPOST(group, "/protocol-profiles", "product:create", s.handleCreateProtocolProfile)
	permissionPUT(group, "/protocol-profiles/:code", "product:edit", s.handleUpdateProtocolProfile)
	permissionDELETE(group, "/protocol-profiles/:code", "product:delete", s.handleDeleteProtocolProfile)
}

func (s *Server) handleListProtocolProfiles(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 10).Int()

	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteStatus(400, g.Map{"error": scopeErr.Error()})
		r.ExitAll()
	}

	// We simply rely on tenantID and projectID here for profiles for now
	profiles, total, err := store.ListProtocolProfiles(page, pageSize, tenantID, projectID)
	if err != nil {
		r.Response.WriteJson(g.Map{"error": "Failed to list protocol profiles: " + err.Error()})
		r.ExitAll()
	}

	r.Response.WriteJson(g.Map{
		"data":  profiles,
		"total": total,
	})
}

func (s *Server) handleGetProtocolProfile(r *ghttp.Request) {
	code := r.Get("code").String()

	profile, err := store.GetProtocolProfile(code)
	if err != nil {
		r.Response.WriteStatus(404, g.Map{"error": "Protocol profile not found"})
		r.ExitAll()
	}

	// Basic tenant check
	tenantID, projectID, _ := currentTenantProjectScope(r)
	if tenantID != 0 && profile.TenantID != 0 && profile.TenantID != tenantID {
		r.Response.WriteStatus(403, g.Map{"error": "Forbidden"})
		r.ExitAll()
	}
	if projectID != 0 && profile.ProjectID != 0 && profile.ProjectID != projectID {
		r.Response.WriteStatus(403, g.Map{"error": "Forbidden"})
		r.ExitAll()
	}

	r.Response.WriteJson(profile)
}

func (s *Server) handleCreateProtocolProfile(r *ghttp.Request) {
	var req struct {
		Code         string      `json:"code" v:"required#Code is required"`
		Name         string      `json:"name" v:"required#Name is required"`
		ProtocolName string      `json:"protocol_name" v:"required#ProtocolName is required"`
		ProductCode  string      `json:"product_code"`
		Description  string      `json:"description"`
		Config       interface{} `json:"config"`
	}

	if err := r.Parse(&req); err != nil {
		r.Response.WriteStatus(400, g.Map{"error": err.Error()})
		r.ExitAll()
	}

	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteStatus(400, g.Map{"error": scopeErr.Error()})
		r.ExitAll()
	}

	configStr := "{}"
	if req.Config != nil {
		switch v := req.Config.(type) {
		case string:
			configStr = v
		default:
			if bytes, err := json.Marshal(req.Config); err == nil {
				configStr = string(bytes)
			}
		}
	}

	pp := &store.ProtocolProfile{
		TenantID:     tenantID,
		ProjectID:    projectID,
		Code:         req.Code,
		Name:         req.Name,
		ProtocolName: req.ProtocolName,
		ProductCode:  req.ProductCode,
		Description:  req.Description,
		Config:       configStr,
	}

	if err := store.SaveProtocolProfile(pp); err != nil {
		r.Response.WriteStatus(500, g.Map{"error": "Failed to save protocol profile: " + err.Error()})
		r.ExitAll()
	}



	r.Response.WriteJson(pp)
}

func (s *Server) handleUpdateProtocolProfile(r *ghttp.Request) {
	code := r.Get("code").String()

	pp, err := store.GetProtocolProfile(code)
	if err != nil {
		r.Response.WriteStatus(404, g.Map{"error": "Protocol profile not found"})
		r.ExitAll()
	}

	tenantID, projectID, _ := currentTenantProjectScope(r)
	if tenantID != 0 && pp.TenantID != 0 && pp.TenantID != tenantID {
		r.Response.WriteStatus(403, g.Map{"error": "Forbidden"})
		r.ExitAll()
	}
	if projectID != 0 && pp.ProjectID != 0 && pp.ProjectID != projectID {
		r.Response.WriteStatus(403, g.Map{"error": "Forbidden"})
		r.ExitAll()
	}

	var req struct {
		Name         string      `json:"name"`
		ProductCode  string      `json:"product_code"`
		Description  string      `json:"description"`
		Config       interface{} `json:"config"`
	}

	if err := r.Parse(&req); err != nil {
		r.Response.WriteStatus(400, g.Map{"error": err.Error()})
		r.ExitAll()
	}

	if req.Name != "" {
		pp.Name = req.Name
	}
	if req.ProductCode != "" {
		pp.ProductCode = req.ProductCode
	}
	if req.Description != "" {
		pp.Description = req.Description
	}
	if req.Config != nil {
		switch v := req.Config.(type) {
		case string:
			pp.Config = v
		default:
			if bytes, err := json.Marshal(req.Config); err == nil {
				pp.Config = string(bytes)
			}
		}
	}

	if err := store.SaveProtocolProfile(pp); err != nil {
		r.Response.WriteStatus(500, g.Map{"error": "Failed to update protocol profile: " + err.Error()})
		r.ExitAll()
	}



	r.Response.WriteJson(pp)
}

func (s *Server) handleDeleteProtocolProfile(r *ghttp.Request) {
	code := r.Get("code").String()

	pp, err := store.GetProtocolProfile(code)
	if err != nil {
		r.Response.WriteStatus(404, g.Map{"error": "Protocol profile not found"})
		r.ExitAll()
	}

	tenantID, projectID, _ := currentTenantProjectScope(r)
	if tenantID != 0 && pp.TenantID != 0 && pp.TenantID != tenantID {
		r.Response.WriteStatus(403, g.Map{"error": "Forbidden"})
		r.ExitAll()
	}
	if projectID != 0 && pp.ProjectID != 0 && pp.ProjectID != projectID {
		r.Response.WriteStatus(403, g.Map{"error": "Forbidden"})
		r.ExitAll()
	}

	if err := store.DeleteProtocolProfile(code); err != nil {
		r.Response.WriteStatus(500, g.Map{"error": "Failed to delete protocol profile: " + err.Error()})
		r.ExitAll()
	}



	r.Response.WriteJson(g.Map{"message": "success"})
}
