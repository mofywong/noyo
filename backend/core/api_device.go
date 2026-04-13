package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/protocol"
	"noyo/core/store"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

func (s *Server) RegisterDeviceRoutes(group *ghttp.RouterGroup) {
	group.GET("/products", s.handleListProducts)
	group.GET("/products/:code", s.handleGetProduct)
	group.POST("/products", s.handleCreateProduct)
	group.PUT("/products/:code", s.handleUpdateProduct)
	group.DELETE("/products/:code", s.handleDeleteProduct)

	group.GET("/devices", s.handleListDevices)
	group.GET("/devices/import/template", s.handleDownloadTemplate)
	group.POST("/devices/import", s.handleImportDevices)
	group.POST("/devices", s.handleCreateDevice)
	group.GET("/devices/:code", s.handleGetDevice)
	group.PUT("/devices/:code", s.handleUpdateDevice)
	group.DELETE("/devices/:code", s.handleDeleteDevice)

	group.POST("/devices/:code/start", s.handleStartDevice)
	group.POST("/devices/:code/stop", s.handleStopDevice)
	group.POST("/devices/:code/write", s.handleWritePoint)
	group.POST("/devices/:code/invoke", s.handleInvokeService)
	group.GET("/devices/:code/data", s.handleGetDeviceData)
	group.GET("/devices/:code/events", s.handleListDeviceEvents)
	group.GET("/stats", s.handleGetStats)

	// 设备配置 Schema API（子设备从父设备获取协议的 SubDeviceConfigSchema）
	group.GET("/devices/config-schema", s.handleGetDeviceConfigSchema)
}

// --- Product Handlers ---

func (s *Server) handleListProducts(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 10).Int()

	products, total, err := store.ListProducts(page, pageSize)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}
	r.Response.WriteJson(g.Map{
		"code":     0,
		"data":     products,
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
	// ProtocolName 现在为可选字段，为空表示该产品只能作为子设备使用
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
	r.Response.WriteJson(g.Map{"code": 0, "data": device})
}

func (s *Server) handleListDevices(r *ghttp.Request) {
	page := r.Get("page", 1).Int()
	pageSize := r.Get("pageSize", 10).Int()

	devices, total, err := store.ListDevices(page, pageSize)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
		return
	}

	// Enrich with status
	type DeviceWithStatus struct {
		store.Device
		Status          string             `json:"status"` // "running", "stopped"
		Online          bool               `json:"online"`
		LastActive      time.Time          `json:"last_active"`
		AIHealthScore   *float64           `json:"ai_health_score,omitempty"`
		AIHealthDetails map[string]float64 `json:"ai_health_details,omitempty"`
		AILatched       bool               `json:"ai_latched,omitempty"`
		AIHealthTrigger *float64           `json:"ai_health_trigger,omitempty"`
	}

	result := make([]DeviceWithStatus, 0)

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

	// 验证协议配置
	product, err := store.GetProduct(d.ProductCode)
	if err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Product not found"})
		return
	}

	if d.ParentCode == "" {
		// 直连设备：产品必须绑定协议
		if product.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "该产品没有绑定协议，只能作为子设备使用"})
			return
		}
	} else {
		// 子设备：验证父设备存在且其产品有协议
		parentDevice, err := store.GetDevice(d.ParentCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备不存在"})
			return
		}
		parentProduct, err := store.GetProduct(parentDevice.ProductCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备的产品不存在"})
			return
		}
		if parentProduct.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备的产品没有绑定协议"})
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
	if err := validatePollingGroupsPreserved(oldDevice.Config, d.Config); err != nil {
		r.Response.WriteJson(g.Map{"code": 400, "message": err.Error()})
		return
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

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device updated"})
}

func (s *Server) handleDeleteDevice(r *ghttp.Request) {
	code := r.Get("code").String()

	// Get device info before deletion to check ParentCode
	device, _ := store.GetDevice(code)
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

	r.Response.WriteJson(g.Map{"code": 0, "message": "Device stopped"})
}

func (s *Server) handleGetDeviceData(r *ghttp.Request) {
	code := r.Get("code").String()
	data := s.DeviceManager.GetLatestData(code)
	if data == nil {
		data = make(map[string]interface{})
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": data})
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
	if !deviceModel.Enabled {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device is disabled"})
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
	protocolPlugin, ok := plugin.(protocol.IProtocolPlugin)
	if !ok {
		r.Response.WriteJson(g.Map{"code": 500, "message": "Plugin is not a protocol plugin"})
		return
	}

	// 4. Execute Write
	// Use WriteProperty instead of WritePoint
	if err := protocolPlugin.WriteProperty(deviceMeta, req.PointID, req.Value); err != nil {
		r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
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
	if !deviceModel.Enabled {
		r.Response.WriteJson(g.Map{"code": 400, "message": "Device is disabled"})
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
	// 1. Products Count
	var totalProducts int64
	store.DB.Model(&store.Product{}).Count(&totalProducts)

	// 2. Devices Stats - Use Registry cache instead of DB query
	devices := s.DeviceManager.Registry.GetAllDevices()
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
// 子设备：返回父设备协议的 SubDeviceConfigSchema
func (s *Server) handleGetDeviceConfigSchema(r *ghttp.Request) {
	productCode := r.Get("productCode").String()
	parentCode := r.Get("parentCode").String()
	schemaType := r.Get("type", "device").String()

	if productCode == "" {
		r.Response.WriteJson(g.Map{"code": 400, "message": "productCode is required"})
		return
	}

	var protocolName string
	var isSubDevice bool
	var schema []byte
	var err error

	if parentCode == "" {
		// 直连设备：使用产品自己的协议
		var product *store.Product
		product, err = store.GetProduct(productCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 404, "message": "Product not found"})
			return
		}
		if product.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "该产品没有绑定协议，只能作为子设备使用"})
			return
		}
		protocolName = product.ProtocolName
		isSubDevice = false

		// 获取协议插件的设备配置 Schema
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

		if schemaType == "point" {
			// Get Point Config Schema
			schema, err = protocolPlugin.GetPointConfigSchema()
		} else {
			// Get Device Config Schema
			meta := DeviceMeta{ParentCode: "", ProductCode: productCode}
			schema, err = protocolPlugin.GetDeviceConfigSchema(meta)
		}

		if err != nil {
			r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
			return
		}
	} else {
		// 子设备：优先从父设备产品配置获取 sub_device_config_schema
		parentDevice, err := store.GetDevice(parentCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 404, "message": "父设备不存在"})
			return
		}
		parentProduct, err := store.GetProduct(parentDevice.ProductCode)
		if err != nil {
			r.Response.WriteJson(g.Map{"code": 404, "message": "父设备的产品不存在"})
			return
		}
		if parentProduct.ProtocolName == "" {
			r.Response.WriteJson(g.Map{"code": 400, "message": "父设备的产品没有绑定协议"})
			return
		}
		protocolName = parentProduct.ProtocolName
		isSubDevice = true

		// 尝试从父产品配置获取 sub_device_config_schema
		var customSchema []byte
		if parentProduct.Config != "" {
			var prodConfig map[string]interface{}
			if json.Unmarshal([]byte(parentProduct.Config), &prodConfig) == nil {
				if subSchema, ok := prodConfig["sub_device_config_schema"]; ok {
					customSchema, _ = json.Marshal(subSchema)
				}
			}
		}

		if len(customSchema) > 0 {
			// 使用父产品自定义的子设备配置 Schema
			schema = customSchema
		} else {
			// 回退到协议插件的 Schema
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

			if schemaType == "point" {
				schema, err = protocolPlugin.GetPointConfigSchema()
			} else {
				meta := DeviceMeta{ParentCode: parentCode, ProductCode: productCode}
				schema, err = protocolPlugin.GetDeviceConfigSchema(meta)
			}

			if err != nil {
				r.Response.WriteJson(g.Map{"code": 500, "message": err.Error()})
				return
			}
		}
	}

	r.Response.WriteJson(g.Map{
		"code": 0,
		"data": g.Map{
			"schema":       json.RawMessage(schema),
			"protocolName": protocolName,
			"isSubDevice":  isSubDevice,
		},
	})
}
