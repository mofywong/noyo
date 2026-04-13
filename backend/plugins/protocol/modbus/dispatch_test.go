package modbus

import (
	"encoding/binary"
	"noyo/core"
	"noyo/plugins/protocol/modbus/codec"
	"testing"

	"go.uber.org/zap"
)

func TestDispatchData(t *testing.T) {
	// Setup
	logger, _ := zap.NewDevelopment()
	p := &ModbusPlugin{
		Logger: logger,
	}

	// 1. Define Polling Group
	// Group: Start 0, Length 10 (20 bytes)
	groupName := "Group1"

	// 2. Define Gateway Self-Point (Offset 0, uint16)
	gatewayCode := "GW001"
	gatewayPoint := struct {
		Name         string `json:"name"`
		PollingGroup string `json:"polling_group"`
		codec.PointConfig
	}{
		Name:         "Voltage",
		PollingGroup: groupName,
		PointConfig: codec.PointConfig{
			Offset:    0, // Bytes 0-1
			DataType:  "uint16",
			ByteOrder: "ABCD",
		},
	}

	// 3. Define Sub-Device Point (Offset 2, uint16)
	subDeviceCode := "SUB001"
	subDevicePoint := struct {
		Name         string `json:"name"`
		PollingGroup string `json:"polling_group"`
		codec.PointConfig
	}{
		Name:         "Current",
		PollingGroup: groupName,
		PointConfig: codec.PointConfig{
			Offset:    2, // Register 2
			DataType:  "uint16",
			ByteOrder: "ABCD",
		},
	}

	// 4. Construct ChildrenMetas
	children := []ChildMeta{
		{
			Meta: core.DeviceMeta{DeviceCode: gatewayCode},
			ParsedPoints: []ChildPoint{
				{
					Name:         gatewayPoint.Name,
					PollingGroup: gatewayPoint.PollingGroup,
					PointConfig:  gatewayPoint.PointConfig,
				},
			},
		},
		{
			Meta: core.DeviceMeta{DeviceCode: subDeviceCode},
			ParsedPoints: []ChildPoint{
				{
					Name:         subDevicePoint.Name,
					PollingGroup: subDevicePoint.PollingGroup,
					PointConfig:  subDevicePoint.PointConfig,
				},
			},
		},
	}

	// 5. Mock Data (20 bytes)
	data := make([]byte, 20)
	// Voltage = 220 (Reg 0) -> 0x00DC
	binary.BigEndian.PutUint16(data[0:], 220)
	// Current = 50 (Reg 2) -> 0x0032
	binary.BigEndian.PutUint16(data[4:], 50)

	// 6. Execute Dispatch
	results, _ := p.dispatchData(data, children, PollingGroupConfig{Name: groupName, FunctionCode: 3})

	// 7. Verify Results

	// Check Gateway Result
	if gwRes, ok := results[gatewayCode]; !ok {
		t.Errorf("Expected result for gateway %s", gatewayCode)
	} else {
		if val, ok := gwRes["Voltage"]; !ok {
			t.Errorf("Expected point Voltage for gateway")
		} else {
			if v, ok := val.(float64); !ok || v != 220 {
				t.Errorf("Expected Voltage 220, got %v", val)
			}
		}
	}

	// Check Sub-Device Result
	if subRes, ok := results[subDeviceCode]; !ok {
		t.Errorf("Expected result for sub-device %s", subDeviceCode)
	} else {
		if val, ok := subRes["Current"]; !ok {
			t.Errorf("Expected point Current for sub-device")
		} else {
			expected := float64(50)
			if val.(float64) != expected {
				t.Errorf("Expected Current %v, got %v", expected, val)
			}
		}
	}
}
