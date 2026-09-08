package core

import (
	"testing"
	"time"

	"noyo/core/platform"
)

func TestPlaybackSnapshotBoundToUserDeviceAndExpiry(t *testing.T) {
	m := NewMediaSessionManager()
	now := time.Unix(1700000000, 0)
	m.now = func() time.Time { return now }
	servers := []platform.ICEServerConfig{{URLs: []string{"stun:original.example"}}}
	s, err := m.CreateWithICE("camera", "", 1, servers)
	if err != nil {
		t.Fatal(err)
	}
	servers[0].URLs[0] = "stun:changed.example"
	if s.ICEServers[0].URLs[0] != "stun:original.example" {
		t.Fatal("snapshot mutated")
	}
	if _, err := m.GetPlayback(s.ID, 2, "camera"); err == nil {
		t.Fatal("foreign snapshot accepted")
	}
	if _, err := m.GetPlayback(s.ID, 1, "other"); err == nil {
		t.Fatal("wrong device snapshot accepted")
	}
	if _, err := m.GetPlayback(s.ID, 1, "camera"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(3 * time.Minute)
	if _, err := m.GetPlayback(s.ID, 1, "camera"); err == nil {
		t.Fatal("expired snapshot accepted")
	}
}
