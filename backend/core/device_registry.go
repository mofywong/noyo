package core

import (
	"encoding/json"
	"fmt"
	"noyo/core/store"
	"noyo/core/types"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// DeviceRegistry manages device metadata and persistence
type DeviceRegistry struct {
	mu            sync.RWMutex
	devices       map[string]*store.Device
	products      map[string]*store.Product
	profiles      map[string]*store.ProtocolProfile
	childrenIndex map[string][]string // Optimization: ParentDeviceCode -> []ChildDeviceCode
	Logger        *zap.Logger
}

// NewDeviceRegistry creates a new DeviceRegistry
func NewDeviceRegistry(logger *zap.Logger) *DeviceRegistry {
	return &DeviceRegistry{
		devices:       make(map[string]*store.Device),
		products:      make(map[string]*store.Product),
		childrenIndex: make(map[string][]string),
		Logger:        logger,
	}
}

// Init loads all devices and products from the database
func (dr *DeviceRegistry) Init() error {
	// 1. Load data without holding lock
	var products []store.Product
	if err := store.DB.Find(&products).Error; err != nil {
		return fmt.Errorf("failed to load products: %w", err)
	}

	devices, _, err := store.ListDevices(0, 0, 0, 0)
	if err != nil {
		return fmt.Errorf("failed to load devices: %w", err)
	}

	// 2. Build temporary maps
	newProducts := make(map[string]*store.Product, len(products))
	for _, p := range products {
		val := p // copy
		newProducts[p.Code] = &val
	}

	newDevices := make(map[string]*store.Device, len(devices))
	newChildrenIndex := make(map[string][]string)
	for _, d := range devices {
		val := d // copy
		newDevices[d.Code] = &val
		if d.ParentCode != "" {
			newChildrenIndex[d.ParentCode] = append(newChildrenIndex[d.ParentCode], d.Code)
		}
	}

	// 3. Short lock to swap caches
	dr.mu.Lock()
	dr.products = newProducts
	dr.devices = newDevices
	dr.childrenIndex = newChildrenIndex
	dr.mu.Unlock()
	if err := dr.restoreActiveRuntime(); err != nil {
		return err
	}

	dr.Logger.Info("DeviceRegistry initialized",
		zap.Int("devices", len(newDevices)),
		zap.Int("products", len(newProducts)),
		zap.Int("parents_indexed", len(newChildrenIndex)))
	return nil
}

// GetDevice returns the raw device model from cache
func (dr *DeviceRegistry) GetDevice(code string) (*store.Device, bool) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	d, ok := dr.devices[code]
	if !ok {
		return nil, false
	}
	copy := *d
	return &copy, true
}

// GetDeviceName returns the name of a device by code, or empty string if not found
func (dr *DeviceRegistry) GetDeviceName(code string) string {
	if code == "" {
		return ""
	}
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	if d, ok := dr.devices[code]; ok && d != nil {
		return d.Name
	}
	return ""
}

// GetAllDevices returns all devices
func (dr *DeviceRegistry) GetAllDevices() []*store.Device {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	list := make([]*store.Device, 0, len(dr.devices))
	for _, d := range dr.devices {
		copy := *d
		list = append(list, &copy)
	}
	return list
}

// GetProduct returns the raw product model from cache
func (dr *DeviceRegistry) GetProduct(code string) (*store.Product, bool) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	p, ok := dr.products[code]
	if !ok {
		return nil, false
	}
	copy := *p
	return &copy, true
}

// GetDeviceMeta constructs the DeviceMeta (including context) for a device
func (dr *DeviceRegistry) GetDeviceMeta(deviceCode string) (*types.DeviceMeta, error) {
	dr.mu.RLock()
	device, ok := dr.devices[deviceCode]
	if !ok {
		dr.mu.RUnlock()
		return nil, fmt.Errorf("%w: device %s", types.ErrNotFound, deviceCode)
	}
	// Copy necessary data to avoid holding lock during JSON unmarshal
	devCopy := *device

	// Pre-fetch relation meta to avoid re-locking or long holding
	var parentDevice *store.Device
	if devCopy.ParentCode != "" {
		parentDevice = dr.devices[devCopy.ParentCode]
	}

	// 忽略 cascade 网关这一层：如果父设备是 cascade，则本设备视作顶级直连设备
	if parentDevice != nil {
		if parentDevice.ProtocolName == "cascade" {
			parentDevice = nil
			devCopy.ParentCode = "" // 抹去 ParentCode，使其在 Meta 中表现为顶级设备
		}
	}

	var childDevices []*store.Device
	if childrenCodes, hasChildren := dr.childrenIndex[devCopy.Code]; hasChildren {
		for _, code := range childrenCodes {
			if child, exists := dr.devices[code]; exists && child.Enabled {
				childDevices = append(childDevices, child)
			}
		}
	}
	dr.mu.RUnlock()

	// 1. Construct Base Meta
	//fmt.Printf("[DEBUG] GetDeviceMeta: Code=%s ConfigRaw='%s'\n", devCopy.Code, devCopy.Config)
	deviceMeta := &types.DeviceMeta{
		ProductCode: devCopy.ProductCode,
		DeviceCode:  devCopy.Code,
		ParentCode:  devCopy.ParentCode,
		Extras:      make(map[string]interface{}),
	}
	if devCopy.ProtocolProfileCode != "" {
		if pp, err := dr.runtimeProfile(devCopy.ProtocolProfileCode); err == nil && pp.Config != "" {
			_ = json.Unmarshal([]byte(pp.Config), &deviceMeta.Extras)
		}
	}
	if devCopy.Config != "" {
		var devConfig map[string]interface{}
		if err := json.Unmarshal([]byte(devCopy.Config), &devConfig); err == nil {
			for k, v := range devConfig {
				deviceMeta.Extras[k] = v
			}
		}
	}

	// 2. Populate Context
	if parentDevice != nil {
		// SubDevice: Find Parent
		parentMeta := types.DeviceMeta{
			ProductCode: parentDevice.ProductCode,
			DeviceCode:  parentDevice.Code,
			ParentCode:  parentDevice.ParentCode,
			Extras:      make(map[string]interface{}),
		}
		if parentDevice.ProtocolProfileCode != "" {
			if pp, err := dr.runtimeProfile(parentDevice.ProtocolProfileCode); err == nil && pp.Config != "" {
				_ = json.Unmarshal([]byte(pp.Config), &parentMeta.Extras)
			}
		}
		if parentDevice.Config != "" {
			var devConfig map[string]interface{}
			if err := json.Unmarshal([]byte(parentDevice.Config), &devConfig); err == nil {
				for k, v := range devConfig {
					parentMeta.Extras[k] = v
				}
			}
		}
		deviceMeta.Parent = &parentMeta
	} else if len(childDevices) > 0 {
		// Gateway: Find SubDevices using index
		for _, sub := range childDevices {
			subMeta := types.DeviceMeta{
				ProductCode: sub.ProductCode,
				DeviceCode:  sub.Code,
				ParentCode:  sub.ParentCode,
				Extras:      make(map[string]interface{}),
			}
			if sub.ProtocolProfileCode != "" {
				if pp, err := dr.runtimeProfile(sub.ProtocolProfileCode); err == nil && pp.Config != "" {
					_ = json.Unmarshal([]byte(pp.Config), &subMeta.Extras)
				}
			}
			if sub.Config != "" {
				var devConfig map[string]interface{}
				if err := json.Unmarshal([]byte(sub.Config), &devConfig); err == nil {
					for k, v := range devConfig {
						subMeta.Extras[k] = v
					}
				}
			}
			deviceMeta.SubDevices = append(deviceMeta.SubDevices, subMeta)
		}
	}

	return deviceMeta, nil
}

// GetProductMeta returns the ProductMeta
func (dr *DeviceRegistry) GetProductMeta(productCode string) (types.ProductMeta, error) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	p, ok := dr.products[productCode]
	if !ok {
		return types.ProductMeta{}, fmt.Errorf("%w: product %s", types.ErrNotFound, productCode)
	}

	meta := types.ProductMeta{
		Name:   p.Name,
		Code:   p.Code,
		Config: make(map[string]interface{}),
	}
	if p.Config != "" {
		_ = json.Unmarshal([]byte(p.Config), &meta.Config)
	}
	return meta, nil
}

// GetEffectiveProtocol 获取设备实际使用的协议名称
// 直连设备：使用自己产品的协议
// 子设备：使用父设备产品的协议
func (dr *DeviceRegistry) GetEffectiveProtocol(deviceCode string) (string, error) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	device, ok := dr.devices[deviceCode]
	if !ok {
		return "", fmt.Errorf("%w: device %s", types.ErrNotFound, deviceCode)
	}
	// A registered cascade gateway is remote unless it is this node's own
	// gateway identity. Locally owned devices must use their protocol drivers.
	localGatewayCode := store.LocalConfigGatewayCode()
	if device.OwnerNode != "" && device.OwnerNode != device.Code && device.OwnerNode != localGatewayCode {
		if owner, exists := dr.devices[device.OwnerNode]; exists && owner.ProtocolName == "cascade" {
			return "cascade", nil
		}
	}

	// Forward devices below a remote gateway through cascade, but stop at the
	// local gateway boundary so local service calls reach the protocol plugin.
	current := device
	for current.ParentCode != "" {
		parentDevice, ok := dr.devices[current.ParentCode]
		if !ok {
			break
		}
		if parentDevice.ProtocolName == "cascade" {
			if parentDevice.Code != localGatewayCode {
				return "cascade", nil
			}
			break
		}
		current = parentDevice
	}

	if device.ProtocolName != "" {
		return device.ProtocolName, nil
	}

	if device.ParentCode == "" {
		return "", fmt.Errorf("直连设备必须绑定协议")
	}

	// 子设备：使用父设备的协议
	parentDevice, ok := dr.devices[device.ParentCode]
	if !ok {
		return "", fmt.Errorf("%w: parent device %s", types.ErrNotFound, device.ParentCode)
	}
	if parentDevice.ProtocolName == "" {
		return "", fmt.Errorf("父设备没有绑定协议")
	}
	return parentDevice.ProtocolName, nil
}

// UpdateDeviceStatus updates status (last active time) in DB (optional/async)
// For now Registry focuses on metadata. Status is handled by StatusMonitor/Manager.

// SetDeviceEnabled updates the enabled state of a device
func (dr *DeviceRegistry) SetDeviceEnabled(code string, enabled bool) error {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	d, ok := dr.devices[code]
	if !ok {
		return fmt.Errorf("%w: device not found", types.ErrNotFound)
	}

	// Update DB
	next := *d
	next.Enabled = enabled
	if err := store.UpdateDevice(&next); err != nil {
		return err
	}
	if store.IsPlatformConfigProjectionEnabled() && d.OwnerNode != "" {
		confirmed, err := store.GetDevice(code)
		if err != nil {
			return err
		}
		next = *confirmed
	}
	dr.devices[code] = &next

	// Cache is already a pointer to the struct in map?
	// In Init: dr.devices[d.Code] = &val (val is a copy of d)
	// So updating d.Enabled here updates the cache if d is the pointer from map.
	// Let's verify: d, ok := dr.devices[code] -> returns *store.Device
	// Yes, d is the pointer.

	return nil
}

// UpdateDevice updates a device in the registry and maintains the index
func (dr *DeviceRegistry) UpdateDevice(device *store.Device) {
	if store.IsPlatformConfigProjectionEnabled() && device.OwnerNode != "" {
		confirmed, err := store.GetDevice(device.Code)
		if err != nil {
			return
		}
		device = confirmed
	}
	dr.updateRuntimeDevice(device)
}

func (dr *DeviceRegistry) updateRuntimeDevice(device *store.Device) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// 1. Handle Index Maintenance
	if old, ok := dr.devices[device.Code]; ok {
		// If Parent Changed
		if old.ParentCode != device.ParentCode {
			// Remove from old parent
			if old.ParentCode != "" {
				dr.removeFromIndexLocked(old.ParentCode, old.Code)
			}
			// Add to new parent
			if device.ParentCode != "" {
				dr.addToIndexLocked(device.ParentCode, device.Code)
			}
		}
	} else {
		// New Device
		if device.ParentCode != "" {
			dr.addToIndexLocked(device.ParentCode, device.Code)
		}
	}

	// 2. Update Cache
	val := *device // Copy to avoid external mutation affecting cache
	dr.devices[device.Code] = &val
}

// UpdateProduct updates a product in the registry
func (dr *DeviceRegistry) UpdateProduct(product *store.Product) {
	if store.IsPlatformConfigProjectionEnabled() && product.OwnerNode != "" {
		confirmed, err := store.GetProduct(product.Code)
		if err != nil {
			return
		}
		product = confirmed
	}
	dr.mu.Lock()
	defer dr.mu.Unlock()

	val := *product // Copy to avoid external mutation affecting cache
	dr.products[product.Code] = &val
}

func (dr *DeviceRegistry) RemoveProduct(code string) {
	if store.IsPlatformConfigProjectionEnabled() {
		if current, ok := dr.GetProduct(code); ok && current.OwnerNode != "" {
			if _, err := store.GetProduct(code); err != gorm.ErrRecordNotFound {
				return
			}
		}
	}
	dr.removeRuntimeProduct(code)
}

func (dr *DeviceRegistry) removeRuntimeProduct(code string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	delete(dr.products, code)
}

// RemoveDevice removes a device from registry
func (dr *DeviceRegistry) RemoveDevice(code string) {
	if store.IsPlatformConfigProjectionEnabled() {
		if current, ok := dr.GetDevice(code); ok && current.OwnerNode != "" {
			if _, err := store.GetDevice(code); err != gorm.ErrRecordNotFound {
				return
			}
		}
	}
	dr.removeRuntimeDevice(code)
}

func (dr *DeviceRegistry) removeRuntimeDevice(code string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	if d, ok := dr.devices[code]; ok {
		if d.ParentCode != "" {
			dr.removeFromIndexLocked(d.ParentCode, code)
		}
		delete(dr.devices, code)
	}
}

// addToIndexLocked adds a child code to the parent's index
func (dr *DeviceRegistry) addToIndexLocked(parentCode, childCode string) {
	// Optimization: Check for duplicates?
	// Assuming callers don't add duplicates or it's acceptable.
	// To be safe:
	list := dr.childrenIndex[parentCode]
	for _, c := range list {
		if c == childCode {
			return // Already exists
		}
	}
	dr.childrenIndex[parentCode] = append(list, childCode)
}

// removeFromIndexLocked removes a child code from the parent's index
func (dr *DeviceRegistry) removeFromIndexLocked(parentCode, childCode string) {
	list := dr.childrenIndex[parentCode]
	for i, c := range list {
		if c == childCode {
			// Remove
			dr.childrenIndex[parentCode] = append(list[:i], list[i+1:]...)
			return
		}
	}
}

// Reload reloads the registry from DB
func (dr *DeviceRegistry) Reload() error {
	return dr.Init()
}
