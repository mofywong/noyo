package sagoo

import (
	"encoding/json"
	"fmt"
	"noyo/core"
	"time"
	// "go.uber.org/zap"
	// "go.uber.org/zap"
)

// Common Payload Structure
type Payload struct {
	ID        string      `json:"id"`
	Version   string      `json:"version"`
	Method    string      `json:"method"`
	Params    interface{} `json:"params"`
	Timestamp int64       `json:"timestamp"`
}

func (p *SagooPlugin) publish(topic string, method string, params interface{}) error {
	payload := Payload{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Version:   "1.0",
		Method:    method,
		Params:    params,
		Timestamp: time.Now().UnixMilli(),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// p.Logger.Debug("Publishing", zap.String("topic", topic), zap.String("payload", string(data)))

	// p.Logger.Debug("Publishing", zap.String("topic", topic), zap.String("payload", string(data)))

	// Async Push with dropping
	select {
	case p.msgChan <- PublishJob{
		Topic:    topic,
		QoS:      0,
		Retained: false,
		Payload:  data,
	}:
	default:
		// Drop message if buffer full
		if p.Logger != nil {
			// p.Logger.Warn("Sagoo MQTT Buffer Full, dropping message", zap.String("topic", topic))
		}
	}
	return nil
}

// /sys/{ProductCode}/{GatewayCode}/child/{DeviceCode}/thing/...
func (p *SagooPlugin) getTopicPrefix(meta core.DeviceMeta) string {
	return fmt.Sprintf("/sys/%s/%s/child/%s/thing", meta.ProductCode, p.Config.GatewayCode, meta.DeviceCode)
}

func (p *SagooPlugin) ReportProperty(meta core.DeviceMeta, properties map[string]interface{}) error {
	topic := fmt.Sprintf("%s/event/property/post", p.getTopicPrefix(meta))
	return p.publish(topic, "thing.event.property.post", properties)
}

func (p *SagooPlugin) ReportEvent(meta core.DeviceMeta, eventId string, params map[string]interface{}) error {
	topic := fmt.Sprintf("%s/event/%s/post", p.getTopicPrefix(meta), eventId)
	// Wrap params in a "value" object
	wrappedParams := map[string]interface{}{
		"value": params,
	}
	return p.publish(topic, fmt.Sprintf("thing.event.%s.post", eventId), wrappedParams)
}

func (p *SagooPlugin) ReportBatchProperties(meta core.DeviceMeta, properties map[string]interface{}) error {
	topic := fmt.Sprintf("%s/event/property/pack/post", p.getTopicPrefix(meta))
	// Sagoo pack format: params: { "properties": { "temp": { "value": 25, "time": 123 } } }
	// The interface input 'properties' is simple map[string]interface{}.
	// We need to wrap it if strictly following manual.
	// Manual says: params: { properties: { key: { value: v, time: t } } }

	// Construct the complex structure
	complexProps := make(map[string]interface{})
	now := time.Now().UnixMilli()
	for k, v := range properties {
		complexProps[k] = map[string]interface{}{
			"value": v,
			"time":  now,
		}
	}

	params := map[string]interface{}{
		"properties": complexProps,
	}

	return p.publish(topic, "thing.event.property.pack.post", params)
}

func (p *SagooPlugin) ReportStatus(meta core.DeviceMeta, status string) error {
	// status: "online" or "offline"
	topic := fmt.Sprintf("%s/status/%s", p.getTopicPrefix(meta), status)
	return p.publish(topic, fmt.Sprintf("thing.status.%s", status), map[string]interface{}{})
}
