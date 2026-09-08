package cascade

import (
	"testing"
	"testing/synctest"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type commandTestToken struct{ mqtt.Token }

func (commandTestToken) Wait() bool   { return true }
func (commandTestToken) Error() error { return nil }

type commandTestClient struct {
	mqtt.Client
	reply func()
}

func (commandTestClient) IsConnected() bool { return true }
func (c commandTestClient) Publish(string, byte, bool, interface{}) mqtt.Token {
	go c.reply()
	return commandTestToken{}
}

func TestVideoCommandWaitsForSIPAndICE(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		engine := &platformEngineImpl{}
		replied := make(chan struct{})
		engine.client = commandTestClient{reply: func() {
			defer close(replied)
			time.Sleep(12 * time.Second)
			if ch, ok := engine.pendingCmds.Load("play"); ok {
				ch.(chan map[string]interface{}) <- map[string]interface{}{"code": float64(200), "data": "answer"}
			}
		}}
		result, err := engine.SendCommand("gateway", "play", []byte(`{"method":"service_invoke","params":{"service_id":"PlayRealTimeStream","params":{"sdp_offer":"offer"}}}`))
		<-replied
		if err != nil || result != "answer" {
			t.Fatalf("video reply after SIP/ICE preparation was lost: result=%v err=%v", result, err)
		}
	})
}

func TestOrdinaryCommandStillTimesOutPromptly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		engine := &platformEngineImpl{client: commandTestClient{reply: func() {}}}
		started := time.Now()
		_, err := engine.SendCommand("gateway", "control", []byte(`{"method":"service_invoke","params":{"service_id":"PTZControl"}}`))
		if err == nil || time.Since(started) > 10*time.Second {
			t.Fatalf("ordinary command timeout changed: elapsed=%v err=%v", time.Since(started), err)
		}
	})
}
