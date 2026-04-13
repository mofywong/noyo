package mqtt_api

import (
	"fmt"
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
func (p *MQTTAPIPlugin) subscribeToCommands() {
	// All commands (API): /sys/{GatewayCode}/api/#
	topicGateway := fmt.Sprintf("/sys/%s/api/+", p.Config.GatewayCode)
	p.client.Subscribe(topicGateway, 0, p.handleGatewayCommand)
}

// Note: IPlugin.SetProperty and CallService are Downstream methods (Core -> Plugin).
// Since MQTTAPIPlugin is Northbound (Gateway -> Platform), it receives commands via MQTT (Upstream from Device perspective, but Downlink from Platform).
// The actual execution would involve calling a Southbound Plugin.
// Thus, we do not override SetProperty/CallService here as this plugin doesn't control a device directly.
