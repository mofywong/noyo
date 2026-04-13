package aiot

import (
	"encoding/json"
	// "fmt"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	// "go.uber.org/zap"
)

// Reply Structure for Command Response
type CommandReply struct {
	ID        string      `json:"id"`
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data"`
	Timestamp int64       `json:"timestamp"`
}

// subscribeToCommands subscribes to platform command topics
// Topic: /sys/+/GatewayCode/child/+/thing/service/#
// We use wildcards for ProductCode (+) and DeviceCode (+)
func (p *AiotPlugin) subscribeToCommands() {
	// topic pattern: /sys/{ChildProductKey}/{ChildDeviceKey}/thing/service/property/set
	// We want to match ANY product and ANY device for this gateway
	topic := "/sys/+/+/thing/service/property/set"

	// p.Logger.Info("Subscribing to commands",
	// zap.String("topic", topic),
	// zap.String("gateway_code", p.Config.GatewayCode),
	// )

	token := p.client.Subscribe(topic, 0, p.handleCommand)
	if token.Wait() && token.Error() != nil {
		// p.Logger.Error("Failed to subscribe to commands",
		// zap.String("topic", topic),
		// zap.Error(token.Error()),
		// )
	}
}

// handleCommand processes incoming MQTT messages from the platform
func (p *AiotPlugin) handleCommand(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := msg.Payload()

	// 打印完整的接收信息，包括 Topic 和 Payload
	// p.Logger.Info("@@@ Received MQTT Message",
	// zap.String("topic", topic),
	// zap.String("payload_hex", fmt.Sprintf("%x", payload)), // 打印 Hex 方便调试二进制问题
	// zap.String("payload_str", string(payload)),            // 打印字符串内容
	// )

	// Avoid infinite loop: ignore own replies
	if strings.HasSuffix(topic, "_reply") {
		return
	}

	// Parse Topic to extract metadata
	// Format: /sys/{productCode}/{deviceCode}/thing/service/property/set
	parts := strings.Split(topic, "/")
	// ""/sys/prod/dev/thing/service/property/set
	// 0 / 1 / 2  / 3 / 4   / 5     / 6      / 7
	if len(parts) < 8 {
		// p.Logger.Warn("Invalid command topic format", zap.String("topic", topic))
		return
	}

	// productCode := parts[2]
	deviceCode := parts[3]

	// Determine serviceId from topic suffix
	var serviceId string
	if strings.HasSuffix(topic, "/thing/service/property/set") {
		serviceId = "property/set"
	} else {
		// Fallback/Generic parsing if needed, but for now we focus on property/set
		// serviceId = ...
	}

	// Parse Payload
	var cmd Payload
	if err := json.Unmarshal(payload, &cmd); err != nil {
		// p.Logger.Error("Failed to parse command payload", zap.Error(err))
		return
	}

	// Logic to handle command would go here.
	// p.Logger.Info("Processing Command",
	// zap.String("deviceCode", deviceCode),
	// zap.String("serviceId", serviceId),
	// zap.String("cmdId", cmd.ID),
	// )

	reply := CommandReply{
		ID:        cmd.ID,
		Timestamp: time.Now().UnixMilli(),
	}

	// Handle Property Set
	if serviceId == "property/set" {
		// p.Logger.Info("Handling Property Set Command", zap.String("device", deviceCode))

		// Cast Params
		paramsMap, ok := cmd.Params.(map[string]interface{})
		if !ok {
			// p.Logger.Error("Invalid params format for property set", zap.Any("params", cmd.Params))
			reply.Code = 400
			reply.Message = "Invalid params"
		} else {
			// Dispatch via Context IssueCommand
			_, err := p.Ctx.IssueCommand(deviceCode, "set_properties", paramsMap)
			if err != nil {
				// p.Logger.Error("Failed to set properties", zap.Error(err))
				reply.Code = 500
				reply.Message = err.Error()
			} else {
				reply.Code = 200
				reply.Message = "success"
				reply.Data = map[string]interface{}{}
			}
		}
	} else {
		// Default handler for other commands (mock success for now)
		reply.Code = 200
		reply.Message = "success"
		reply.Data = map[string]interface{}{}
	}

	// Send Reply (Ack)
	// Reply Topic: {original_topic}_reply
	replyTopic := topic + "_reply"

	if p.client == nil || !p.client.IsConnected() {
		// p.Logger.Warn("MQTT not connected, cannot send command reply", zap.String("topic", replyTopic))
		return
	}

	replyData, _ := json.Marshal(reply)

	token := p.client.Publish(replyTopic, 0, false, replyData)
	if token.Wait() && token.Error() != nil {
		// p.Logger.Error("Failed to send command reply", zap.Error(token.Error()))
	} else {
		// p.Logger.Info("Sent Command Reply", zap.String("topic", replyTopic))
	}
}

// Note: IPlugin.SetProperty and CallService are Downstream methods (Core -> Plugin).
// Since AiotPlugin is Northbound (Gateway -> Platform), it receives commands via MQTT (Upstream from Device perspective, but Downlink from Platform).
// The actual execution would involve calling a Southbound Plugin.
// Thus, we do not override SetProperty/CallService here as this plugin doesn't control a device directly.
