package types

// Device Status
const (
	DeviceStatusOnline  = "online"
	DeviceStatusOffline = "offline"
)

// Plugin Categories
const (
	PluginCategoryPlatform = "platform"
	PluginCategoryProtocol = "protocol"
)

// Data Types (used in DataModel and Events)
const (
	DataTypeProperty      = "property"
	DataTypeEvent         = "event"
	DataTypeStatus        = "status"
	DataTypeServiceResult = "service_result"
)

// Common Metadata Keys
const (
	MetaKeyProductCode = "product_code"
	MetaKeyDeviceCode  = "device_code"
)
