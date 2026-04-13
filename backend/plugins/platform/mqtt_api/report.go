package mqtt_api

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
	ID          string      `json:"id"`
	Version     string      `json:"version"`
	ProductCode string      `json:"productCode,omitempty"`
	DeviceCode  string      `json:"deviceCode,omitempty"`
	Method      string      `json:"method"`
	Params      interface{} `json:"params"`
	Timestamp   int64       `json:"timestamp"`
}

func (p *MQTTAPIPlugin) publish(topic string, method string, productCode string, deviceCode string, params interface{}) error {
	payload := Payload{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Version:     "1.0",
		ProductCode: productCode,
		DeviceCode:  deviceCode,
		Method:      method,
		Params:      params,
		Timestamp:   time.Now().UnixMilli(),
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
			// p.Logger.Warn("MQTT API Buffer Full, dropping message", zap.String("topic", topic))
		}
	}
	return nil
}

func (p *MQTTAPIPlugin) getTopicPrefix() string {
	return fmt.Sprintf("/sys/%s/api", p.Config.GatewayCode)
}

func (p *MQTTAPIPlugin) ReportProperty(meta core.DeviceMeta, properties map[string]interface{}) error {
	topic := fmt.Sprintf("%s/property_post", p.getTopicPrefix())
	return p.publish(topic, "property_post", meta.ProductCode, meta.DeviceCode, properties)
}

func (p *MQTTAPIPlugin) ReportEvent(meta core.DeviceMeta, eventId string, params map[string]interface{}) error {
	topic := fmt.Sprintf("%s/event_post", p.getTopicPrefix())
	// Wrap params in a "value" object and include eventId
	wrappedParams := map[string]interface{}{
		"eventId": eventId,
		"value":   params,
	}
	return p.publish(topic, "event_post", meta.ProductCode, meta.DeviceCode, wrappedParams)
}

func (p *MQTTAPIPlugin) ReportBatchProperties(meta core.DeviceMeta, properties map[string]interface{}) error {
	topic := fmt.Sprintf("%s/property_pack_post", p.getTopicPrefix())
	// MQTT API pack format: params: { "properties": { "temp": { "value": 25, "time": 123 } } }
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

	return p.publish(topic, "property_pack_post", meta.ProductCode, meta.DeviceCode, params)
}

func (p *MQTTAPIPlugin) ReportStatus(meta core.DeviceMeta, status string) error {
	// status: "online" or "offline"
	topic := fmt.Sprintf("%s/status_post", p.getTopicPrefix())
	return p.publish(topic, "status_post", meta.ProductCode, meta.DeviceCode, map[string]interface{}{
		"status": status,
	})
}
