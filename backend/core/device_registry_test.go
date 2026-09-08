package core

import (
	"noyo/core/store"
	"testing"

	"go.uber.org/zap"
)

func TestDeviceRegistry_IndexMaintenance(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	dr := NewDeviceRegistry(logger)

	// Mock Devices
	parent := &store.Device{Code: "Gateway1", ProductCode: "P1"}
	child1 := &store.Device{Code: "Sensor1", ProductCode: "P2", ParentCode: "Gateway1"}

	// 1. Add Parent
	dr.UpdateDevice(parent)

	// 2. Add Child
	dr.UpdateDevice(child1)

	// Verify Index
	dr.mu.RLock()
	children := dr.childrenIndex["Gateway1"]
	dr.mu.RUnlock()

	if len(children) != 1 || children[0] != "Sensor1" {
		t.Errorf("Index mismatch after add, expected [Sensor1], got %v", children)
	}

	// 3. Move Child to new Parent
	newParent := &store.Device{Code: "Gateway2", ProductCode: "P1"}
	dr.UpdateDevice(newParent)

	child1Mod := &store.Device{Code: "Sensor1", ProductCode: "P2", ParentCode: "Gateway2"}
	dr.UpdateDevice(child1Mod)

	// Verify Old Parent Index
	dr.mu.RLock()
	oldChildren := dr.childrenIndex["Gateway1"]
	newChildren := dr.childrenIndex["Gateway2"]
	dr.mu.RUnlock()

	if len(oldChildren) != 0 {
		t.Errorf("Old parent index should be empty, got %v", oldChildren)
	}
	if len(newChildren) != 1 || newChildren[0] != "Sensor1" {
		t.Errorf("New parent index mismatch, expected [Sensor1], got %v", newChildren)
	}

	// 4. Remove Child from Parent (Make independent)
	child1Indep := &store.Device{Code: "Sensor1", ProductCode: "P2", ParentCode: ""}
	dr.UpdateDevice(child1Indep)

	dr.mu.RLock()
	finalChildren := dr.childrenIndex["Gateway2"]
	dr.mu.RUnlock()

	if len(finalChildren) != 0 {
		t.Errorf("New parent index should be empty after removal, got %v", finalChildren)
	}
}

func TestDeviceRegistry_UsesCascadeForGB28181CameraBehindGateway(t *testing.T) {
	logger := zap.NewNop()
	registry := NewDeviceRegistry(logger)
	registry.UpdateDevice(&store.Device{Code: "gateway-001", ProtocolName: "cascade"})
	registry.UpdateDevice(&store.Device{
		Code:         "34020009132401000002",
		ParentCode:   "gateway-001",
		ProtocolName: "gb28181",
	})

	protocol, err := registry.GetEffectiveProtocol("34020009132401000002")
	if err != nil {
		t.Fatalf("GetEffectiveProtocol() error = %v", err)
	}
	if protocol != "cascade" {
		t.Fatalf("GetEffectiveProtocol() = %q, want %q so playback is forwarded to the gateway", protocol, "cascade")
	}
}
