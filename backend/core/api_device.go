package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/protocol"
	"noyo/core/store"
	"noyo/core/tsdb"
	"noyo/core/types"
	"reflect"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func (s *Server) RegisterDeviceRoutes(group *ghttp.RouterGroup) {
	permissionGET(group, "/products", "product:list", s.handleListProducts)
	permissionGET(group, "/products/:code", "product:list", s.handleGetProduct)
	permissionPOST(group, "/products", "product:create", s.handleCreateProduct)
	permissionPUT(group, "/products/:code", "product:edit", s.handleUpdateProduct)
	permissionDELETE(group, "/products/:code", "product:delete", s.handleDeleteProduct)

	permissionGET(group, "/devices", "device:list", s.handleListDevices)
	permissionGET(group, "/devices/stream", "device:list", s.handleDeviceStream)
	permissionGET(group, "/devices/import/template", "device:create", s.handleDownloadTemplate)
	permissionPOST(group, "/devices/import", "device:create", s.handleImportDevices)
	permissionGET(group, "/devices/config-schema", "device:list", s.handleGetDeviceConfigSchema)
	permissionPOST(group, "/devices", "device:create", s.handleCreateDevice)
	permissionPUT(group, "/devices/:code/tags", "device:edit", s.handleReplaceDeviceTags)
	permissionGET(group, "/devices/:code", "device:list", s.handleGetDevice)
	permissionPUT(group, "/devices/:code", "device:edit", s.handleUpdateDevice)
	permissionDELETE(group, "/devices/:code", "device:delete", s.handleDeleteDevice)

	permissionGET(group, "/device-tags", "device_tag:list", s.handleListDeviceTags)
	permissionPOST(group, "/device-tags", "device_tag:create", s.handleCreateDeviceTag)
	permissionPUT(group, "/device-tags/:id", "device_tag:edit", s.handleUpdateDeviceTag)
	permissionDELETE(group, "/device-tags/:id", "device_tag:delete", s.handleDeleteDeviceTag)
	permissionGET(group, "/device-tags/:id/devices", "device_tag:list", s.handleListDeviceTagDevices)
	permissionPUT(group, "/device-tags/:id/devices", "device_tag:edit", s.handleReplaceDeviceTagDevices)

	permissionPOST(group, "/devices/:code/start", "device:control", s.handleStartDevice)
	permissionPOST(group, "/devices/:code/stop", "device:control", s.handleStopDevice)
	permissionPOST(group, "/devices/:code/write", "device:control", s.handleWritePoint)
	permissionPOST(group, "/devices/:code/invoke", "device:control", s.handleInvokeService)
	permissionGET(group, "/devices/:code/data", "device:list", s.handleGetDeviceData)
	permissionGET(group, "/devices/:code/events", "device:list", s.handleListDeviceEvents)
	permissionGET(group, "/stats", "device:list", s.handleGetStats)

}

// --- Product Handlers ---

func (s *Server) handleListProducts(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 10).Int()

	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()

	var projectIDs []uint
	restrictProjects := false
	if authCtx := requestAuthContext(r); authCtx != nil {
		if ids, restricted := authCtx.ProjectIDsForTenantQuery(); restricted && projectID == 0 {
			projectIDs = ids
			restrictProjects = true
		}
	}

	var products []store.Product
	var total int64
	var err error
	if restrictProjects {
		products, total, err = store.ListProducts(page, pageSize, tenantID, projectID, projectIDs)
	} else {
		products, total, err = store.ListProducts(page, pageSize, tenantID, projectID)
	}
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	type ProductResponse struct {
		store.Product
		ProjectName string `json:"project_name"`
	}
	projectNames := projectNameMap(tenantID)
	productResponses := make([]ProductResponse, 0, len(products))
	for _, product := range products {
		productResponses = append(productResponses, ProductResponse{
			Product:     product,
			ProjectName: projectNames[product.ProjectID],
		})
	}
	r.Response.WriteJson(g.Map{
		"code":     0,
		"data":     productResponses,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (s *Server) handleGetProduct(r *ghttp.Request) {
	code := r.Get("code").String()
	product, err := store.GetProduct(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Product not found"})
		return
	}
	if !canAccessProduct(r, product) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": product})
}

func (s *Server) handleCreateProduct(r *ghttp.Request) {
	var p store.Product
	if err := json.Unmarshal(r.GetBody(), &p); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	if p.Code == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Code is required"})
		return
	}

	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": scopeErr.Error()})
		return
	}
	p.TenantID = tenantID
	p.ProjectID = projectID
	if !canAccessProduct(r, &p) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}


	if err := store.SaveProduct(&p); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	// Fetch the saved product and update cache
	updatedProduct, _ := store.GetProduct(p.Code)
	if updatedProduct != nil {
		s.DeviceManager.Registry.UpdateProduct(updatedProduct)
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Product created"})
}

func (s *Server) handleUpdateProduct(r *ghttp.Request) {
	code := r.Get("code").String()
	var p store.Product
	if err := json.Unmarshal(r.GetBody(), &p); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	// Ensure code matches
	p.Code = code
	existing, err := store.GetProduct(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Product not found"})
		return
	}
	if !canAccessProduct(r, existing) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if existing.TenantID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Cannot modify global product"})
		return
	}
	p.TenantID = existing.TenantID
	p.ProjectID = existing.ProjectID

	if err := store.UpdateProduct(&p); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	// Fetch the full updated product from DB to ensure cache is not corrupted by partial updates
	updatedProduct, err := store.GetProduct(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch updated product"})
		return
	}
	// Update Registry Cache
	s.DeviceManager.Registry.UpdateProduct(updatedProduct)

	// Hot Reload: Restart all enabled devices using this product
	devices, err := store.ListDevicesByProduct(updatedProduct.Code)
	if err == nil {
		count := 0
		for _, d := range devices {
			if d.Enabled {
				// Use RestartDevice to stop and start (reloading config)
				if err := s.DeviceManager.RestartDevice(d.Code); err != nil {
					s.Logger.Error("Failed to hot-reload device", zap.String("code", d.Code), zap.Error(err))
				} else {
					count++
				}
			}
		}
		if count > 0 {
			s.Logger.Info("Hot-reloaded devices for product update", zap.String("product", p.Code), zap.Int("count", count))
		}
	} else {
		s.Logger.Error("Failed to list devices for hot-reload", zap.String("product", p.Code), zap.Error(err))
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Product updated"})
}

func (s *Server) handleDeleteProduct(r *ghttp.Request) {
	code := r.Get("code").String()
	product, err := store.GetProduct(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Product not found"})
		return
	}
	if !canAccessProduct(r, product) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if product.TenantID == 0 {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Cannot delete global product"})
		return
	}
	if err := store.DeleteProduct(code); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "message": "Product deleted"})
}

// --- Device Handlers ---

func (s *Server) handleGetDevice(r *ghttp.Request) {
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
	r.Response.WriteJson(g.Map{"code": 0, "data": device})
}

func (s *Server) handleListDevices(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 10).Int()

	r.Response.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	r.Response.Header().Set("Pragma", "no-cache")
	r.Response.Header().Set("Expires", "0")

	tenantID := r.GetCtxVar("tenant_id").Uint()
	projectID := r.GetCtxVar("project_id").Uint()

	var projectIDs []uint
	restrictProjects := false
	if authCtx := requestAuthContext(r); authCtx != nil {
		if ids, restricted := authCtx.ProjectIDsForTenantQuery(); restricted && projectID == 0 {
			projectIDs = ids
			restrictProjects = true
		}
	}

	var devices []store.Device
	var total int64
	var err error
	if restrictProjects {
		devices, total, err = store.ListDevices(page, pageSize, tenantID, projectID, projectIDs)
	} else {
		devices, total, err = store.ListDevices(page, pageSize, tenantID, projectID)
	}
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	s.Logger.Info("ListDevices fetched from DB", zap.Int("count", len(devices)), zap.Uint("reqTenantID", tenantID), zap.Uint("reqProjectID", projectID))

	if authCtx := requestAuthContext(r); authCtx != nil {
		filtered := make([]store.Device, 0, len(devices))
		scope := currentDeviceTagScope(r)
		for _, device := range devices {
			allowed, err := canReadDeviceByTagPermission(authCtx, scope, device.Code)
			canAccess := canAccessDevice(r, &device)
			if err == nil && allowed && canAccess {
				filtered = append(filtered, device)
			} else {
				s.Logger.Info("Device filtered out", zap.String("code", device.Code), zap.Bool("allowed", allowed), zap.Bool("canAccess", canAccess), zap.Error(err))
			}
		}
		devices = filtered
		total = int64(len(filtered))
	}

	// Enrich with status
	type DeviceWithStatus struct {
		store.Device
		Status          string             `json:"status"` // "running", "stopped"
		Online          bool               `json:"online"`
		LastActive      time.Time          `json:"last_active"`
		ProjectName     string             `json:"project_name"`
		Tags            []store.DeviceTag  `json:"tags"`
		AIHealthScore   *float64           `json:"ai_health_score,omitempty"`
		AIHealthDetails map[string]float64 `json:"ai_health_details,omitempty"`
		AILatched       bool               `json:"ai_latched,omitempty"`
		AIHealthTrigger *float64           `json:"ai_health_trigger,omitempty"`
	}

	result := make([]DeviceWithStatus, 0)
	deviceCodes := make([]string, 0, len(devices))
	for _, d := range devices {
		deviceCodes = append(deviceCodes, d.Code)
	}
	tagsByDevice, err := store.ListTagsForDevices(currentDeviceTagScope(r), deviceCodes)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	projectNames := projectNameMap(tenantID)

	for _, d := range devices {
		status := "stopped"
		if s.DeviceManager.IsRunning(d.Code) {
			status = "running"
		}

		online := false
		var lastActive time.Time
		if sVal, ok := s.DeviceManager.GetStatus(d.Code); ok {
			online = sVal.Online
			lastActive = sVal.LastActive
		}

		var healthScore *float64
		var healthDetails map[string]float64
		var isLatched bool = false
		var healthTrigger *float64

		// Only compute health score when device is online
		if online {
			latestData := s.DeviceManager.GetLatestData(d.Code)
			if latestData != nil {
				var minHealth float64 = 101 // Initialize > 100
				foundHealth := false
				details := make(map[string]float64)
				for k, v := range latestData {
					// Check for latched status
					if len(k) > 11 && k[len(k)-11:] == "_ai_latched" {
						if val, ok := v.(bool); ok && val {
							isLatched = true
						}
					}

					// Check for trigger score (health score at moment of lock)
					if len(k) > 18 && k[len(k)-18:] == "_ai_health_trigger" {
						switch val := v.(type) {
						case float64:
							if val > 0 {
								healthTrigger = &val
							}
						case int:
							f := float64(val)
							if f > 0 {
								healthTrigger = &f
							}
						case float32:
							f := float64(val)
							if f > 0 {
								healthTrigger = &f
							}
						}
					}

					// Assuming standard metric naming like `temp_ai_health`
					if len(k) > 10 && k[len(k)-10:] == "_ai_health" {
						var score float64
						isValid := false
						switch val := v.(type) {
						case float64:
							score = val
							isValid = true
						case int:
							score = float64(val)
							isValid = true
						case int64:
							score = float64(val)
							isValid = true
						case float32:
							score = float64(val)
							isValid = true
						}

						if isValid {
							// Extract property name: e.g. "temp_ai_health" -> "temp"
							propName := k[:len(k)-10]
							details[propName] = score
							if score < minHealth {
								minHealth = score
								foundHealth = true
							}
						}
					}
				}
				if foundHealth {
					healthScore = &minHealth
					healthDetails = details
				}
			}
		}

		result = append(result, DeviceWithStatus{
			Device:          d,
			Status:          status,
			Online:          online,
			LastActive:      lastActive,
			ProjectName:     projectNames[d.ProjectID],
			Tags:            tagsByDevice[d.Code],
			AIHealthScore:   healthScore,
			AIHealthDetails: healthDetails,
			AILatched:       isLatched,
			AIHealthTrigger: healthTrigger,
		})
	}

	r.Response.WriteJson(g.Map{
		"code":     0,
		"data":     result,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (s *Server) handleCreateDevice(r *ghttp.Request) {
	var d store.Device
	if err := json.Unmarshal(r.GetBody(), &d); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	if d.Code == "" || d.ProductCode == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Code and ProductCode are required"})
		return
	}

	tenantID, projectID, scopeErr := currentTenantProjectScope(r)
	if scopeErr != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": scopeErr.Error()})
		return
	}
	d.TenantID = tenantID
	d.ProjectID = projectID
	if !canAccessDevice(r, &d) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}

	// 验证协议配置
	product, err := store.GetProduct(d.ProductCode)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Product not found"})
		return
	}
	if (product.TenantID != 0 && product.TenantID != d.TenantID) || (product.ProjectID != 0 && product.ProjectID != d.ProjectID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Product is outside current project"})
		return
	}

	if d.ProtocolName != "" {
		if err := validateProtocolEnabledForProject(d.ProtocolName, d.TenantID, d.ProjectID); err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
			return
		}
	} else if d.ParentCode == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "直连设备必须绑定通信协议"})
		return
	}

	if d.ParentCode != "" {
		// 子设备：验证父设备存在
		parentDevice, err := store.GetDevice(d.ParentCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备不存在"})
			return
		}
		if !canAccessDevice(r, parentDevice) {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
			return
		}
		if parentDevice.TenantID != d.TenantID || parentDevice.ProjectID != d.ProjectID {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Parent device is outside current project"})
			return
		}

		if parentDevice.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备没有绑定协议"})
			return
		}

		// 如果父设备是级联网关，则该设备视作直连设备，自身必须绑定协议
		if parentDevice.ProtocolName == "cascade" && d.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "级联网关下的设备（视作直连设备）必须绑定协议"})
			return
		}
	}

	if err := store.SaveDevice(&d); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Fetch the saved device and update cache
	savedDevice, _ := store.GetDevice(d.Code)
	if savedDevice != nil {
		s.DeviceManager.Registry.UpdateDevice(savedDevice)
	}

	// Auto-start if enabled
	if d.Enabled {
		if err := s.DeviceManager.StartDevice(d.Code); err != nil {
			s.Logger.Error("Failed to auto-start device", zap.String("code", d.Code), zap.Error(err))
			// Don't fail the request, but maybe warn?
		}
	}

	// Restart Parent if this is a sub-device
	if d.ParentCode != "" {
		s.restartParent(d.ParentCode)
	}

	s.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceListChanged,
		Timestamp: time.Now().UnixMilli(),
	})

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device created"})
}

func (s *Server) handleUpdateDevice(r *ghttp.Request) {
	code := r.Get("code").String()
	var d store.Device
	if err := json.Unmarshal(r.GetBody(), &d); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}
	d.Code = code

	// Validate Polling Groups Integrity
	oldDevice, err := store.GetDevice(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, oldDevice) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if err := s.checkDeviceTagPermission(r, code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}
	d.TenantID = oldDevice.TenantID
	d.ProjectID = oldDevice.ProjectID
	if err := validatePollingGroupsPreserved(oldDevice.Config, d.Config); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
	}

	// 验证协议配置
	product, err := store.GetProduct(d.ProductCode)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Product not found"})
		return
	}
	if (product.TenantID != 0 && product.TenantID != d.TenantID) || (product.ProjectID != 0 && product.ProjectID != d.ProjectID) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Product is outside current project"})
		return
	}

	if d.ProtocolName != "" {
		if err := validateProtocolEnabledForProject(d.ProtocolName, d.TenantID, d.ProjectID); err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
			return
		}
	} else if d.ParentCode == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "直连设备必须绑定通信协议"})
		return
	}

	if d.ParentCode != "" {
		parentDevice, err := store.GetDevice(d.ParentCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备不存在"})
			return
		}
		if !canAccessDevice(r, parentDevice) {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
			return
		}
		if parentDevice.TenantID != d.TenantID || parentDevice.ProjectID != d.ProjectID {
			r.Response.WriteJson(g.Map{"code": 403, "message": "Parent device is outside current project"})
			return
		}

		if parentDevice.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备没有绑定协议"})
			return
		}

		if parentDevice.ProtocolName == "cascade" && d.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "级联网关下的设备（视作直连设备）必须绑定协议"})
			return
		}
	}

	if err := store.UpdateDevice(&d); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Fetch the full updated device from DB to ensure cache is not corrupted by partial updates
	updatedDevice, err := store.GetDevice(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Failed to fetch updated device"})
		return
	}

	// Update Registry Cache
	s.DeviceManager.Registry.UpdateDevice(updatedDevice)

	// Restart logic
	noRestart := r.Get("no_restart", false).Bool() || shouldSkipRestartForDeviceUpdate(oldDevice, updatedDevice)
	if !noRestart {
		// 1. Stop if running
		if s.DeviceManager.IsRunning(code) {
			s.DeviceManager.StopDevice(code)
		}

		// 2. Start if enabled
		if updatedDevice.Enabled {
			if err := s.DeviceManager.StartDevice(code); err != nil {
				s.Logger.Error("Failed to restart device", zap.String("code", code), zap.Error(err))
			}
		}

		// 3. Restart Parent if this is a sub-device
		if updatedDevice.ParentCode != "" {
			s.restartParent(updatedDevice.ParentCode)
		}
	}

	s.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceListChanged,
		Timestamp: time.Now().UnixMilli(),
	})

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device updated"})
}

func shouldSkipRestartForDeviceUpdate(oldDevice, newDevice *store.Device) bool {
	if oldDevice == nil || newDevice == nil {
		return false
	}
	if oldDevice.ProductCode != "gb28181_camera" || newDevice.ProductCode != "gb28181_camera" {
		return false
	}
	if oldDevice.Code != newDevice.Code ||
		oldDevice.Name != newDevice.Name ||
		oldDevice.ProductCode != newDevice.ProductCode ||
		oldDevice.ParentCode != newDevice.ParentCode ||
		oldDevice.Enabled != newDevice.Enabled {
		return false
	}

	var oldConfig map[string]interface{}
	var newConfig map[string]interface{}
	if err := json.Unmarshal([]byte(oldDevice.Config), &oldConfig); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(newDevice.Config), &newConfig); err != nil {
		return false
	}

	yoloConfigKeys := map[string]bool{
		"enable_yolo":               true,
		"yolo_classes":              true,
		"enable_yolo_webrtc":        true,
		"yolo_confidence":           true,
		"ai_basic_detections":       true,
		"ai_basic_detection_groups": true,
		"ai_scene_rules":            true,
	}

	changed := false
	seen := make(map[string]bool, len(oldConfig)+len(newConfig))
	for key, oldValue := range oldConfig {
		seen[key] = true
		newValue, ok := newConfig[key]
		if !ok || !reflect.DeepEqual(oldValue, newValue) {
			if !yoloConfigKeys[key] {
				return false
			}
			changed = true
		}
	}
	for key, newValue := range newConfig {
		if seen[key] {
			continue
		}
		if !yoloConfigKeys[key] {
			return false
		}
		if _, exists := oldConfig[key]; !exists || !reflect.DeepEqual(oldConfig[key], newValue) {
			changed = true
		}
	}

	return changed
}

func (s *Server) handleDeleteDevice(r *ghttp.Request) {
	code := r.Get("code").String()

	// Get device info before deletion to check ParentCode
	device, err := store.GetDevice(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, device) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	parentCode := ""
	if device != nil {
		parentCode = device.ParentCode
	}

	// Stop if running
	s.DeviceManager.StopDevice(code)

	if err := store.DeleteDevice(code); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Restart Parent if this was a sub-device
	if parentCode != "" {
		s.restartParent(parentCode)
	}

	s.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceListChanged,
		Timestamp: time.Now().UnixMilli(),
	})

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device deleted"})
}

func (s *Server) handleStartDevice(r *ghttp.Request) {
	code := r.Get("code").String()

	// Update DB enabled status
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
	device.Enabled = true
	if err := store.SaveDevice(device); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	// Update Registry Cache
	s.DeviceManager.Registry.UpdateDevice(device)

	if err := s.DeviceManager.StartDevice(code); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Restart Parent if this is a sub-device
	if device.ParentCode != "" {
		s.restartParent(device.ParentCode)
	}

	s.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceListChanged,
		Timestamp: time.Now().UnixMilli(),
	})

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device started"})
}

func (s *Server) handleStopDevice(r *ghttp.Request) {
	code := r.Get("code").String()

	// Update DB enabled status
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
	device.Enabled = false
	if err := store.SaveDevice(device); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	// Update Registry Cache
	s.DeviceManager.Registry.UpdateDevice(device)

	if err := s.DeviceManager.StopDevice(code); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Recursively disable children if this is a parent
	// Note: ListDevices(0,0) returns all for now if pagination not enforced, or we need specific method
	// But ListDevices now requires pagination args. Let's use ListDevicesByParent instead which is safer.
	// Or just use ListDevices(0,0) if we update it to handle 0 as "all".
	// Let's update store.ListDevices to handle 0,0 as all.

	// Actually, better to use ListDevicesByParent which we need to expose or use ListDevices with large limit?
	// Let's look at store.ListDevicesByParent implementation. It's already there.

	childDevices, _ := store.ListDevicesByParent(code)
	for _, child := range childDevices {
		if child.ParentCode == code && child.Enabled {
			child.Enabled = false
			store.SaveDevice(&child)
			// Update Registry Cache
			s.DeviceManager.Registry.UpdateDevice(&child)
			s.DeviceManager.StopDevice(child.Code)
			s.Logger.Info("Auto-disabled child device", zap.String("parent", code), zap.String("child", child.Code))
		}
	}

	// Restart Parent if this is a sub-device
	if device.ParentCode != "" {
		s.restartParent(device.ParentCode)
	}

	s.DeviceManager.EventBus.Publish(types.Event{
		Type:      types.EventDeviceListChanged,
		Timestamp: time.Now().UnixMilli(),
	})

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device stopped"})
}

func (s *Server) handleGetDeviceData(r *ghttp.Request) {
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
	data := s.DeviceManager.GetLatestData(code)

	// If no runtime data (e.g., offline or just restarted), try to fetch the last reported data from TSDB
	if (data == nil || len(data) == 0) && s.DeviceManager.TSDB != nil {
		req := tsdb.QueryRequest{
			DeviceCode: code,
			StartTime:  0,
			EndTime:    time.Now().UnixMilli(),
			Type:       tsdb.TypeTelemetry,
			Page:       1,
			PageSize:   1,
		}
		res, err := s.DeviceManager.TSDB.Query(req)
		if err == nil && len(res.List) > 0 {
			if latestRec, ok := res.List[0].(map[string]interface{}); ok {
				data = make(map[string]interface{})
				for k, v := range latestRec {
					if k != "ts" && k != "_type" && k != "raw" && k != "error" {
						data[k] = v
					}
				}
			}
		}
	}

	if data == nil {
		data = make(map[string]interface{})
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": data})
}

func (s *Server) handleDeviceStream(r *ghttp.Request) {
	r.Response.Header().Set("Content-Type", "text/event-stream")
	r.Response.Header().Set("Cache-Control", "no-cache")
	r.Response.Header().Set("Connection", "keep-alive")
	r.Response.Header().Set("Access-Control-Allow-Origin", "*")
	r.Response.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering

	// Send initial connection event so client knows SSE is working
	r.Response.Write("event: connected\ndata: {}\n\n")
	r.Response.Flush()

	eventChan := make(chan types.Event, 100)
	handler := func(e types.Event) {
		select {
		case eventChan <- e:
		default:
			s.Logger.Warn("SSE client event buffer full, dropping event", zap.String("type", string(e.Type)))
		}
	}

	// Subscribe with IDs so we can unsubscribe on disconnect
	id1 := s.DeviceManager.EventBus.SubscribeWithID(types.EventDeviceStatusChanged, handler)
	id2 := s.DeviceManager.EventBus.SubscribeWithID(types.EventDeviceListChanged, handler)
	id3 := s.DeviceManager.EventBus.SubscribeWithID(types.EventEventReported, handler)
	id4 := s.DeviceManager.EventBus.SubscribeWithID(types.EventPropertyReported, handler)

	// Cleanup subscriptions when this SSE connection closes
	defer func() {
		s.DeviceManager.EventBus.Unsubscribe(types.EventDeviceStatusChanged, id1)
		s.DeviceManager.EventBus.Unsubscribe(types.EventDeviceListChanged, id2)
		s.DeviceManager.EventBus.Unsubscribe(types.EventEventReported, id3)
		s.DeviceManager.EventBus.Unsubscribe(types.EventPropertyReported, id4)
		s.Logger.Debug("SSE client disconnected, subscriptions cleaned up")
	}()

	ctx := r.Context()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case e := <-eventChan:
			if !canAccessDeviceEvent(r, e) {
				continue
			}
			data, err := json.Marshal(e)
			if err == nil {
				msg := fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, string(data))
				_, writeErr := r.Response.Writer.Write([]byte(msg))
				if writeErr != nil {
					s.Logger.Info("SSE client disconnected (write error)", zap.Error(writeErr))
					return
				}
				r.Response.Flush()
			}
		case <-heartbeat.C:
			// Send SSE heartbeat event to keep connection alive and let frontend detect it
			_, writeErr := r.Response.Writer.Write([]byte("event: heartbeat\ndata: {}\n\n"))
			if writeErr != nil {
				s.Logger.Info("SSE client disconnected (heartbeat write error)", zap.Error(writeErr))
				return
			}
			r.Response.Flush()
		}
	}
}

func (s *Server) handleWritePoint(r *ghttp.Request) {
	code := r.Get("code").String()

	type WriteRequest struct {
		PointID string      `json:"point_id"`
		Value   interface{} `json:"value"`
	}
	var req WriteRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	// 1. Load Device
	deviceModel, err := store.GetDevice(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, deviceModel) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if !deviceModel.Enabled {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device is disabled"})
		return
	}

	// Permission Check
	if err := s.checkDeviceTagPermission(r, code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}

	// 2. Prepare Meta & Point Config
	deviceMeta := DeviceMeta{
		ProductCode: deviceModel.ProductCode,
		DeviceCode:  deviceModel.Code,
		ParentCode:  deviceModel.ParentCode,
		Extras:      make(map[string]interface{}),
	}
	if deviceModel.Config != "" {
		json.Unmarshal([]byte(deviceModel.Config), &deviceMeta.Extras)
	}

	// Find Point Config
	var pointConfig map[string]interface{}

	// points can be a map or a list depending on how it was saved (legacy vs new)
	// Current frontend saves it as a list of objects with "name" property
	if points, ok := deviceMeta.Extras["points"].([]interface{}); ok {
		for _, p := range points {
			if cfg, ok := p.(map[string]interface{}); ok {
				if name, ok := cfg["name"].(string); ok && name == req.PointID {
					pointConfig = cfg
					break
				}
			}
		}
	} else if points, ok := deviceMeta.Extras["points"].(map[string]interface{}); ok {
		// Legacy support if it was a map
		if cfg, exists := points[req.PointID]; exists {
			if cfgMap, ok := cfg.(map[string]interface{}); ok {
				pointConfig = cfgMap
			}
		}
	}

	if pointConfig == nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Point config not found"})
		return
	}

	// 3. Get Plugin (子设备从父设备获取协议)
	effectiveProtocol, err := s.DeviceManager.Registry.GetEffectiveProtocol(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	plugin := s.Manager.GetPlugin(effectiveProtocol)
	if plugin == nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Protocol plugin not found"})
		return
	}

	// 4. Execute Write
	type propertyWriter interface {
		WriteProperty(device DeviceMeta, propName string, value interface{}) error
	}
	type pointWriter interface {
		WritePoint(device DeviceMeta, pointCode string, value interface{}) error
	}

	if writer, ok := plugin.(propertyWriter); ok {
		if err := writer.WriteProperty(deviceMeta, req.PointID, req.Value); err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
			return
		}
	} else if writer, ok := plugin.(pointWriter); ok {
		if err := writer.WritePoint(deviceMeta, req.PointID, req.Value); err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
			return
		}
	} else {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Plugin does not support WriteProperty/WritePoint"})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Write successful"})
}

func (s *Server) handleInvokeService(r *ghttp.Request) {
	code := r.Get("code").String()

	type InvokeRequest struct {
		ServiceID string                 `json:"service_id"`
		Params    map[string]interface{} `json:"params"`
	}
	var req InvokeRequest
	if err := json.Unmarshal(r.GetBody(), &req); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Invalid JSON"})
		return
	}

	// 1. Load Device
	deviceModel, err := store.GetDevice(code)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 404, "message": "Device not found"})
		return
	}
	if !canAccessDevice(r, deviceModel) {
		r.Response.WriteJson(g.Map{"code": 403, "message": "Access denied"})
		return
	}
	if !deviceModel.Enabled {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device is disabled"})
		return
	}

	// Permission Check
	if err := s.checkDeviceTagPermission(r, code); err != nil {
		r.Response.WriteJson(g.Map{"code": 403, "message": err.Error()})
		return
	}

	result, err := s.DeviceManager.CallDeviceService(code, req.ServiceID, req.Params)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	r.Response.WriteJson(g.Map{"code": 0, "message": "Service invoked", "data": result})
}

// Helper: Restart parent device if running
func (s *Server) restartParent(parentCode string) {
	if parentCode == "" {
		return
	}
	// Check if parent is running
	if s.DeviceManager.IsRunning(parentCode) {
		s.Logger.Info("Restarting parent device due to sub-device change", zap.String("parent", parentCode))
		// Use RestartDevice to avoid offline status
		if err := s.DeviceManager.RestartDevice(parentCode); err != nil {
			s.Logger.Error("Failed to restart parent device", zap.String("parent", parentCode), zap.Error(err))
		}
	}
}

// Helper: Validate that no polling groups are deleted or renamed
func validatePollingGroupsPreserved(oldConfigJSON, newConfigJSON string) error {
	if oldConfigJSON == "" {
		return nil
	}

	// Parse old config
	var oldConfig map[string]interface{}
	if err := json.Unmarshal([]byte(oldConfigJSON), &oldConfig); err != nil {
		return nil // Old config invalid, skip check
	}

	// Parse new config
	var newConfig map[string]interface{}
	if err := json.Unmarshal([]byte(newConfigJSON), &newConfig); err != nil {
		return nil // New config invalid, will be caught later
	}

	// Extract old group names
	oldGroups := extractGroupNames(oldConfig)
	if len(oldGroups) == 0 {
		return nil
	}

	// Extract new group names
	newGroups := extractGroupNames(newConfig)
	newGroupSet := make(map[string]bool)
	for _, name := range newGroups {
		newGroupSet[name] = true
	}

	// Check if all old groups exist in new groups
	for _, name := range oldGroups {
		if !newGroupSet[name] {
			return fmt.Errorf("polling group '%s' cannot be deleted or renamed (it might be used by sub-devices)", name)
		}
	}

	return nil
}

func extractGroupNames(config map[string]interface{}) []string {
	var names []string
	if groups, ok := config["polling_groups"].([]interface{}); ok {
		for _, g := range groups {
			if groupMap, ok := g.(map[string]interface{}); ok {
				if name, ok := groupMap["name"].(string); ok {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

func (s *Server) handleGetStats(r *ghttp.Request) {
	authCtx := requestAuthContext(r)
	// 1. Products Count
	var totalProducts int64
	productQuery := store.DB.Model(&store.Product{})
	if authCtx != nil && authCtx.TenantID > 0 {
		productQuery = productQuery.Where("tenant_id = ?", authCtx.TenantID)
		if !authCtx.IsTenantAdmin && !authCtx.IsSystemAdmin {
			if authCtx.ProjectID > 0 {
				productQuery = productQuery.Where("project_id = ?", authCtx.ProjectID)
			} else if len(authCtx.AllowedProjectIDs) > 0 {
				productQuery = productQuery.Where("project_id IN ?", authCtx.AllowedProjectIDs)
			} else {
				productQuery = productQuery.Where("1 = 0")
			}
		}
	}
	productQuery.Count(&totalProducts)

	// 2. Devices Stats - Use Registry cache instead of DB query
	devices := s.DeviceManager.Registry.GetAllDevices()
	devices = filterDevicesForAuthContextStats(authCtx, devices)
	totalDevices := len(devices)
	runningDevices := 0
	onlineDevices := 0

	for _, d := range devices {
		if s.DeviceManager.IsRunning(d.Code) {
			runningDevices++
		}
		if status, ok := s.DeviceManager.GetStatus(d.Code); ok && status.Online {
			onlineDevices++
		}
	}

	// 3. Plugin Stats
	plugins := s.Manager.GetPlugins()
	totalPlugins := len(plugins)
	runningPlugins := 0
	for _, p := range plugins {
		if p.IsEnabled() {
			runningPlugins++
		}
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"products": g.Map{
				"total": totalProducts,
			},
			"devices": g.Map{
				"total":   totalDevices,
				"running": runningDevices,
				"online":  onlineDevices,
				"offline": totalDevices - onlineDevices,
			},
			"plugins": g.Map{
				"total":   totalPlugins,
				"running": runningPlugins,
			},
		},
	})
}

// handleGetDeviceConfigSchema 获取设备配置 Schema
// 参数：productCode（必填）, parentCode（可选，子设备时传入父设备编码）
// 直连设备：返回产品协议的 DeviceConfigSchema
// handleGetDeviceConfigSchema returns the device or point config schema for a given protocol.
func (s *Server) handleGetDeviceConfigSchema(r *ghttp.Request) {
	protocolName := r.Get("protocolName").String()
	productCode := r.Get("productCode").String()
	schemaType := r.Get("type", "device").String()
	isSubDevice := r.Get("isSubDevice", false).Bool()
	profileCode := r.Get("profileCode").String()

	if protocolName == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "protocolName is required"})
		return
	}

	plugin := s.Manager.GetPlugin(protocolName)
	if plugin == nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Protocol plugin not found: " + protocolName})
		return
	}
	protocolPlugin, ok := plugin.(protocol.IProtocolPlugin)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Plugin is not a protocol plugin"})
		return
	}

	var schema []byte
	var err error

	if schemaType == "point" {
		schema, err = protocolPlugin.GetPointConfigSchema()
	} else if schemaType == "profile" {
		schema, err = protocolPlugin.GetProfileConfigSchema()
	} else {
		// Device Config Schema
		meta := types.DeviceMeta{
			ParentCode:  "",
			ProductCode: productCode,
			Extras: map[string]interface{}{
				"profileCode": profileCode,
			},
		}
		if isSubDevice {
			meta.ParentCode = "dummy" // 标记为子设备，部分插件可能需要此上下文
		}
		schema, err = protocolPlugin.GetDeviceConfigSchema(meta)
	}

	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	if len(schema) > 0 {
		var schemaMap interface{}
		if err := json.Unmarshal(schema, &schemaMap); err == nil {
			r.Response.WriteJson(g.Map{"code": 0, "data": schemaMap})
			return
		}
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": g.Map{}})

}

func (s *Server) checkDeviceTagPermission(r *ghttp.Request, deviceCode string) error {
	authCtx := requestAuthContext(r)
	if authCtx == nil {
		return fmt.Errorf("auth context not found")
	}
	if authCtx.SubjectType == "app" {
		projectID, err := deviceTagPermissionProjectID(authCtx, deviceCode)
		if err != nil {
			return err
		}
		requiredPermission := r.GetCtxVar("required_permission").String()
		if requiredPermission != "" && !authCtx.HasProjectPermission(requiredPermission, projectID) {
			return fmt.Errorf("permission denied: %s", requiredPermission)
		}
		allowed, err := canWriteDeviceByTagPermission(authCtx, currentDeviceTagScope(r), deviceCode)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
		return fmt.Errorf("read-only access to this device due to tag restrictions")
	}
	if authCtx.IsSystemAdmin || authCtx.IsTenantAdmin || authCtx.IsProjectAdmin {
		return nil
	}

	projectID, err := deviceTagPermissionProjectID(authCtx, deviceCode)
	if err != nil {
		return err
	}
	hasScopedPermissions, err := hasAnyDeviceTagPermission(authCtx, projectID)
	if err != nil {
		return err
	}
	if !hasScopedPermissions {
		allowed, err := hasNonInheritedDevicePermissionFallback(authCtx, projectID)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
		return fmt.Errorf("no device tag write permission for this project")
	}

	scope := currentDeviceTagScope(r)
	var bindings []store.DeviceTagBinding
	if err := store.DB.Where("scope_type = ? AND scope_id = ? AND device_code = ?", scope.Type, scope.ID, deviceCode).Find(&bindings).Error; err != nil {
		return err
	}
	if len(bindings) == 0 {
		return fmt.Errorf("no device tag write permission for this device")
	}

	tagIDs := make([]uint, 0, len(bindings))
	for _, b := range bindings {
		tagIDs = append(tagIDs, b.TagID)
	}
	var count int64
	query, ok := scopedDeviceTagPermissionQuery(authCtx, projectID)
	if !ok {
		return fmt.Errorf("no device tag write permission for this project")
	}
	if err := query.
		Where("role_device_tag_permissions.tag_id IN ? AND role_device_tag_permissions.permission = ?", tagIDs, "write").
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	return fmt.Errorf("read-only access to this device due to tag restrictions")
}

func filterDevicesForAuthContextStats(authCtx *AuthContext, devices []*store.Device) []*store.Device {
	if authCtx == nil {
		return []*store.Device{}
	}
	if authCtx.IsSystemAdmin && authCtx.TenantID == 0 {
		return devices
	}
	filtered := make([]*store.Device, 0, len(devices))
	for _, device := range devices {
		if device == nil {
			continue
		}
		if authCtx.IsSystemAdmin {
			if device.TenantID == authCtx.TenantID {
				filtered = append(filtered, device)
			}
			continue
		}
		if device.TenantID != authCtx.TenantID {
			continue
		}
		if authCtx.IsTenantAdmin || (device.ProjectID > 0 && authCtx.CanAccessProject(device.ProjectID)) {
			filtered = append(filtered, device)
		}
	}
	return filtered
}

func canReadDeviceByTagPermission(authCtx *AuthContext, scope store.AccessScope, deviceCode string) (bool, error) {
	if authCtx == nil {
		return false, nil
	}
	if authCtx.SubjectType == "app" {
		projectID, err := deviceTagPermissionProjectID(authCtx, deviceCode)
		if err != nil {
			return false, err
		}
		if !authCtx.HasProjectPermission("device:list", projectID) {
			return false, nil
		}
		return appCanAccessDeviceByTagPermission(authCtx, scope, deviceCode, "read")
	}
	if authCtx.IsSystemAdmin || authCtx.IsTenantAdmin || authCtx.IsProjectAdmin {
		return true, nil
	}
	if len(authCtx.RoleIDs) == 0 {
		return false, nil
	}
	projectID, err := deviceTagPermissionProjectID(authCtx, deviceCode)
	if err != nil {
		return false, err
	}
	hasScopedPermissions, err := hasAnyDeviceTagPermission(authCtx, projectID)
	if err != nil {
		return false, err
	}
	if !hasScopedPermissions {
		return hasNonInheritedDevicePermissionFallback(authCtx, projectID)
	}

	var bindings []store.DeviceTagBinding
	if err := store.DB.Where("scope_type = ? AND scope_id = ? AND device_code = ?", scope.Type, scope.ID, deviceCode).Find(&bindings).Error; err != nil {
		return false, err
	}
	if len(bindings) == 0 {
		return false, nil
	}
	tagIDs := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		tagIDs = append(tagIDs, binding.TagID)
	}
	var count int64
	query, ok := scopedDeviceTagPermissionQuery(authCtx, projectID)
	if !ok {
		return false, nil
	}
	if err := query.
		Where("role_device_tag_permissions.tag_id IN ? AND role_device_tag_permissions.permission IN ?", tagIDs, []string{"read", "write"}).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func canWriteDeviceByTagPermission(authCtx *AuthContext, scope store.AccessScope, deviceCode string) (bool, error) {
	if authCtx == nil {
		return false, nil
	}
	if authCtx.SubjectType == "app" {
		return appCanAccessDeviceByTagPermission(authCtx, scope, deviceCode, "write")
	}
	if authCtx.IsSystemAdmin || authCtx.IsTenantAdmin || authCtx.IsProjectAdmin {
		return true, nil
	}
	projectID, err := deviceTagPermissionProjectID(authCtx, deviceCode)
	if err != nil {
		return false, err
	}
	hasScopedPermissions, err := hasAnyDeviceTagPermission(authCtx, projectID)
	if err != nil {
		return false, err
	}
	if !hasScopedPermissions {
		return hasNonInheritedDevicePermissionFallback(authCtx, projectID)
	}

	var bindings []store.DeviceTagBinding
	if err := store.DB.Where("scope_type = ? AND scope_id = ? AND device_code = ?", scope.Type, scope.ID, deviceCode).Find(&bindings).Error; err != nil {
		return false, err
	}
	if len(bindings) == 0 {
		return false, nil
	}
	tagIDs := make([]uint, 0, len(bindings))
	for _, binding := range bindings {
		tagIDs = append(tagIDs, binding.TagID)
	}
	var count int64
	query, ok := scopedDeviceTagPermissionQuery(authCtx, projectID)
	if !ok {
		return false, nil
	}
	if err := query.
		Where("role_device_tag_permissions.tag_id IN ? AND role_device_tag_permissions.permission = ?", tagIDs, "write").
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func appCanAccessDeviceByTagPermission(authCtx *AuthContext, scope store.AccessScope, deviceCode, required string) (bool, error) {
	projectID, err := deviceTagPermissionProjectID(authCtx, deviceCode)
	if err != nil {
		return false, err
	}
	var bindings []store.DeviceTagBinding
	if err := store.DB.Where("scope_type = ? AND scope_id = ? AND device_code = ?", scope.Type, scope.ID, deviceCode).Find(&bindings).Error; err != nil {
		return false, err
	}
	if len(bindings) == 0 {
		return false, nil
	}
	for _, binding := range bindings {
		if authCtx.HasAppTagPermission(projectID, binding.TagID, required) {
			return true, nil
		}
	}
	return false, nil
}

func deviceTagPermissionProjectID(authCtx *AuthContext, deviceCode string) (uint, error) {
	if authCtx == nil {
		return 0, fmt.Errorf("auth context not found")
	}
	if authCtx.ProjectID > 0 {
		return authCtx.ProjectID, nil
	}
	var device store.Device
	if err := store.DB.Select("project_id").Where("code = ?", deviceCode).First(&device).Error; err != nil {
		return 0, err
	}
	if device.ProjectID > 0 && !authCtx.CanAccessProject(device.ProjectID) {
		return 0, fmt.Errorf("device project is outside allowed scope")
	}
	return device.ProjectID, nil
}

func roleIDsEffectiveForProject(authCtx *AuthContext, projectID uint) []uint {
	if authCtx == nil || len(authCtx.RoleIDs) == 0 {
		return []uint{}
	}
	if len(authCtx.RoleProjectIDs) == 0 {
		return authCtx.RoleIDs
	}
	roleIDs := make([]uint, 0, len(authCtx.RoleIDs))
	for _, roleID := range authCtx.RoleIDs {
		projectMap := authCtx.RoleProjectIDs[roleID]
		if projectMap[0] || projectMap[projectID] || projectID == 0 {
			roleIDs = append(roleIDs, roleID)
		}
	}
	return roleIDs
}

func scopedDeviceTagPermissionQuery(authCtx *AuthContext, projectID uint) (*gorm.DB, bool) {
	roleIDs := roleIDsEffectiveForProject(authCtx, projectID)
	if len(roleIDs) == 0 {
		return nil, false
	}
	query := store.DB.Model(&store.RoleDeviceTagPermission{}).
		Joins("JOIN roles ON roles.id = role_device_tag_permissions.role_id").
		Where("role_device_tag_permissions.role_id IN ?", roleIDs)
	if projectID > 0 {
		query = query.Where(
			"(role_device_tag_permissions.project_id = ? OR (role_device_tag_permissions.project_id = ? AND roles.is_inherited = ?))",
			projectID,
			0,
			false,
		)
	} else {
		query = query.Where("role_device_tag_permissions.project_id = ?", 0)
	}
	return query, true
}

func hasAnyDeviceTagPermission(authCtx *AuthContext, projectID uint) (bool, error) {
	query, ok := scopedDeviceTagPermissionQuery(authCtx, projectID)
	if !ok {
		return false, nil
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func hasNonInheritedDevicePermissionFallback(authCtx *AuthContext, projectID uint) (bool, error) {
	roleIDs := roleIDsEffectiveForProject(authCtx, projectID)
	if len(roleIDs) == 0 {
		return false, nil
	}
	var count int64
	query := store.DB.Model(&store.Role{}).
		Where("id IN ? AND is_inherited = ?", roleIDs, false)
	if projectID > 0 {
		query = query.Where("(project_id = ? OR project_id = ?)", 0, projectID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func canAccessDeviceEvent(r *ghttp.Request, event types.Event) bool {
	if event.Type == types.EventDeviceListChanged {
		authCtx := requestAuthContext(r)
		if authCtx == nil {
			return false
		}
		if authCtx.IsSystemAdmin {
			return true
		}
		// Check if event carries tenant info matching user's tenant
		if payload, ok := event.Payload.(map[string]interface{}); ok && payload != nil {
			if eventTenantID, ok := payload["tenant_id"].(uint); ok {
				return eventTenantID == authCtx.TenantID
			}
		}
		return false
	}
	if event.Topic == "" {
		return false
	}
	device, err := store.GetDevice(event.Topic)
	if err != nil {
		return false
	}
	return canAccessDevice(r, device)
}

func currentTenantProjectScope(r *ghttp.Request) (uint, uint, error) {
	authCtx := requestAuthContext(r)
	if authCtx == nil {
		return 0, 0, fmt.Errorf("Tenant context is required")
	}

	isGateway := false
	var gatewayTenantID, gatewayProjectID uint
	if state, err := store.LoadSetupState(); err == nil {
		isGateway = IsSingleProjectSetupMode(state.Mode)
		gatewayTenantID = state.TenantID
		gatewayProjectID = state.ProjectID
	}

	if !isGateway && (authCtx.IsSystemAdmin || authCtx.TenantID == 0) {
		return 0, 0, fmt.Errorf("Tenant context is required")
	}

	tenantID := r.GetCtxVar("tenant_id").Uint()
	if tenantID == 0 {
		tenantID = authCtx.TenantID
	}
	if isGateway && tenantID == 0 {
		tenantID = gatewayTenantID
	}

	if !isGateway && tenantID != authCtx.TenantID {
		return 0, 0, fmt.Errorf("Tenant is outside allowed scope")
	}

	projectID := r.GetCtxVar("project_id").Uint()
	if projectID == 0 {
		projectID = authCtx.ProjectID
	}
	if isGateway && projectID == 0 {
		projectID = gatewayProjectID
	}

	if projectID == 0 {
		return tenantID, 0, fmt.Errorf("Project context is required")
	}

	if !isGateway && (!projectBelongsToTenant(projectID, tenantID) || !authCtx.CanManageProject(projectID)) {
		return tenantID, projectID, fmt.Errorf("Project is outside allowed scope")
	}

	return tenantID, projectID, nil
}

func validateProtocolEnabledForProject(protocolName string, tenantID, projectID uint) error {
	if protocolName == "" {
		return nil
	}
	if !isProtocolPluginMeta(protocolName) {
		return fmt.Errorf("Protocol plugin is not available")
	}
	plugin, err := store.GetPluginForScope(protocolName, tenantID, projectID)
	if err != nil {
		return err
	}
	if plugin == nil || !plugin.Enabled {
		return fmt.Errorf("Protocol plugin is not enabled for current project")
	}
	return nil
}

func isProtocolPluginMeta(protocolName string) bool {
	for _, meta := range pluginMetas {
		if meta.Name == protocolName && meta.Category == types.PluginCategoryProtocol {
			return true
		}
	}
	return false
}

func projectNameMap(tenantID uint) map[uint]string {
	names := map[uint]string{}
	if tenantID == 0 {
		return names
	}
	var projects []store.Project
	store.DB.Model(&store.Project{}).Where("tenant_id = ?", tenantID).Find(&projects)
	for _, project := range projects {
		names[project.ID] = project.Name
	}
	return names
}

func canAccessProduct(r *ghttp.Request, product *store.Product) bool {
	authCtx := requestAuthContext(r)
	if authCtx == nil || product == nil {
		return false
	}
	if authCtx.IsSystemAdmin {
		return authCtx.TenantID == 0 || product.TenantID == authCtx.TenantID
	}
	// Allow read access to global products
	if product.TenantID == 0 {
		return true
	}
	if product.TenantID != authCtx.TenantID {
		return false
	}
	if authCtx.IsTenantAdmin {
		return true
	}
	return product.ProjectID > 0 && authCtx.CanAccessProject(product.ProjectID)
}

func canAccessDevice(r *ghttp.Request, device *store.Device) bool {
	authCtx := requestAuthContext(r)
	if authCtx == nil || device == nil {
		return false
	}
	if authCtx.IsSystemAdmin {
		return authCtx.TenantID == 0 || device.TenantID == authCtx.TenantID
	}
	if device.TenantID != authCtx.TenantID {
		return false
	}
	if authCtx.IsTenantAdmin {
		return true
	}
	if !(device.ProjectID > 0 && authCtx.CanAccessProject(device.ProjectID)) {
		return false
	}
	allowed, err := canReadDeviceByTagPermission(authCtx, currentDeviceTagScope(r), device.Code)
	return err == nil && allowed
}
