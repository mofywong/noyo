package core

import (
	"encoding/json"
	"errors"
	"noyo/core/store"
	"noyo/core/types"
	"slices"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// handleUpdateDeviceAIConfig changes only the AI analysis portion of a camera's
// stored configuration. It does not make a manual device inventory edit.
func (s *Server) handleUpdateDeviceAIConfig(r *ghttp.Request) {
	s.aiModelConfigMu.RLock()
	defer s.aiModelConfigMu.RUnlock()
	code := r.Get("code").String()
	device, err := store.GetDevice(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if err := s.checkDeviceTagPermission(r, code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}
	var patch map[string]json.RawMessage
	if err := json.Unmarshal(r.GetBody(), &patch); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid AI configuration JSON"})
		return
	}
	merged, err := store.MergeDeviceAIConfig(device.Config, patch)
	if err == nil {
		err = validateEnabledAISceneRulesConfig(merged)
	}
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}
	if err := s.validateAIModelConfigChange(device.Config, merged); err != nil {
		s.writeAIModelError(r, err)
		return
	}
	updated, err := store.UpdateDeviceAIConfig(code, patch)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		} else {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		}
		return
	}
	s.DeviceManager.Registry.UpdateDevice(updated)
	s.DeviceManager.EventBus.Publish(types.Event{
		Type: types.EventDeviceListChanged, Timestamp: time.Now().UnixMilli(),
	})
	s.Logger.Info("AI analysis configuration updated",
		zap.String("deviceCode", code), zap.String("protocol", updated.ProtocolName),
		zap.Bool("online", slices.Contains(s.DeviceManager.GetOnlineDeviceCodes(), code)))
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{"config": updated.Config}})
}
