package core

import (
	"encoding/json"
	"noyo/core/store"
	"noyo/core/types"
	"strconv"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"gorm.io/gorm"
)

type deviceTagPayload struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Icon        string `json:"icon"`
	Description string `json:"description"`
}

type deviceTagAssignmentPayload struct {
	TagIDs      []uint   `json:"tag_ids"`
	DeviceCodes []string `json:"device_codes"`
}

func currentDeviceTagScope(_ *ghttp.Request) store.AccessScope {
	// Future RBAC integration should resolve this from authenticated user context.
	return store.GlobalAccessScope()
}

func parseUintRouteParam(r *ghttp.Request, name string) (uint, error) {
	value := r.Get(name).String()
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

func (s *Server) publishDeviceListChanged() {
	if s.DeviceManager == nil || s.DeviceManager.EventBus == nil {
		return
	}
	s.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceListChanged,
		Timestamp: time.Now().UnixMilli(),
	})
}

func (s *Server) handleListDeviceTags(r *ghttp.Request) {
	tags, err := store.ListDeviceTags(currentDeviceTagScope(r))
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": tags})
}

func (s *Server) handleCreateDeviceTag(r *ghttp.Request) {
	var payload deviceTagPayload
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tag, err := store.CreateDeviceTag(currentDeviceTagScope(r), &store.DeviceTag{
		Name:        payload.Name,
		Color:       payload.Color,
		Icon:        payload.Icon,
		Description: payload.Description,
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.publishDeviceListChanged()
	r.Response.WriteJson(g.Map{"code": 0, "data": tag, "message": "Device tag created"})
}

func (s *Server) handleUpdateDeviceTag(r *ghttp.Request) {
	id, err := parseUintRouteParam(r, "id")
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid tag id"})
		return
	}

	var payload deviceTagPayload
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	tag, err := store.UpdateDeviceTag(currentDeviceTagScope(r), &store.DeviceTag{
		Model:       gorm.Model{ID: id},
		Name:        payload.Name,
		Color:       payload.Color,
		Icon:        payload.Icon,
		Description: payload.Description,
	})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.publishDeviceListChanged()
	r.Response.WriteJson(g.Map{"code": 0, "data": tag, "message": "Device tag updated"})
}

func (s *Server) handleDeleteDeviceTag(r *ghttp.Request) {
	id, err := parseUintRouteParam(r, "id")
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid tag id"})
		return
	}

	if err := store.DeleteDeviceTag(currentDeviceTagScope(r), id); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.publishDeviceListChanged()
	r.Response.WriteJson(g.Map{"code": 0, "message": "Device tag deleted"})
}

func (s *Server) handleListDeviceTagDevices(r *ghttp.Request) {
	id, err := parseUintRouteParam(r, "id")
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid tag id"})
		return
	}

	deviceCodes, err := store.ListDeviceCodesForTag(currentDeviceTagScope(r), id)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"device_codes": deviceCodes}})
}

func (s *Server) handleReplaceDeviceTagDevices(r *ghttp.Request) {
	id, err := parseUintRouteParam(r, "id")
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid tag id"})
		return
	}

	var payload deviceTagAssignmentPayload
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	if err := store.ReplaceDevicesForTag(currentDeviceTagScope(r), id, payload.DeviceCodes); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.publishDeviceListChanged()
	r.Response.WriteJson(g.Map{"code": 0, "message": "Device tag assignments updated"})
}

func (s *Server) handleReplaceDeviceTags(r *ghttp.Request) {
	code := r.Get("code").String()

	var payload deviceTagAssignmentPayload
	if err := json.Unmarshal(r.GetBody(), &payload); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	scope := currentDeviceTagScope(r)
	if err := store.ReplaceDeviceTags(scope, code, payload.TagIDs); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	tagsByDevice, err := store.ListTagsForDevices(scope, []string{code})
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.publishDeviceListChanged()
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"tags": tagsByDevice[code]}, "message": "Device tags updated"})
}
