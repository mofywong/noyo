package types

// EventType defines the type of event
type EventType string

const (
	EventDeviceStatusChanged EventType = "device.status.changed"
	EventPropertyReported    EventType = "property.reported"
	EventEventReported       EventType = "event.reported"
	EventDeviceServiceResult EventType = "device.service.result"
	EventDeviceListChanged   EventType = "device.list.changed"
	// EventDeviceLocationChanged 是设备地理位置的落库变更（遥测解析出的坐标、
	// 人工标注都算）。地图页据此实时移动点位 —— 没有这条事件，点位会一直停在
	// 进入页面那一刻的位置，越看越偏离现实。
	EventDeviceLocationChanged EventType = "device.location.changed"
	// EventGeofenceTriggered 是设备相对地理围栏的状态跃迁（进入 / 离开）。
	// 它作为规则引擎的一等触发源发布，避免另起一条并行的告警通道。
	EventGeofenceTriggered EventType = "geofence.triggered"
)

// Event represents a generic event in the system
type Event struct {
	Type      EventType
	Topic     string // e.g., device code or specific topic
	Payload   interface{}
	Metadata  map[string]interface{}
	Timestamp int64
}
