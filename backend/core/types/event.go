package types

// EventType defines the type of event
type EventType string

const (
	EventDeviceStatusChanged EventType = "device.status.changed"
	EventPropertyReported    EventType = "property.reported"
	EventEventReported       EventType = "event.reported"
)

// Event represents a generic event in the system
type Event struct {
	Type      EventType
	Topic     string // e.g., device code or specific topic
	Payload   interface{}
	Timestamp int64
}
