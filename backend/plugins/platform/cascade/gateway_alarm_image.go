package cascade

import (
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"noyo/core/types"
)

// Local reporting has already persisted snapshot_base64 and replaced it with a
// gateway-local URL. Restore the bytes only in the uplink copy so the platform's
// normal ReportDeviceEvent path persists its own image before recording the alarm.
func attachGatewaySnapshot(event types.Event) (types.Event, error) {
	if event.Type != types.EventEventReported {
		return event, nil
	}
	payload, _ := event.Payload.(map[string]interface{})
	params, _ := payload["params"].(map[string]interface{})
	if data, _ := params["snapshot_base64"].(string); data != "" {
		return event, nil
	}
	url, _ := params["snapshot_url"].(string)
	if !strings.HasPrefix(url, "/data/images/") {
		return event, nil
	}
	name := strings.TrimPrefix(url, "/data/images/")
	if name == "" || strings.ContainsAny(name, "/\\:") || name == "." || name == ".." {
		return event, fmt.Errorf("invalid local snapshot filename")
	}
	mime := "image/jpeg"
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		mime = "image/png"
	case ".webp":
		mime = "image/webp"
	case ".jpg", ".jpeg":
	default:
		return event, fmt.Errorf("unsupported local snapshot extension")
	}
	// Root confines reads even if an image path is replaced by a symlink.
	root, err := os.OpenRoot("./data/images")
	if err != nil {
		return event, err
	}
	defer root.Close()
	data, err := root.ReadFile(name)
	if err != nil {
		return event, err
	}
	if len(data) == 0 {
		return event, fmt.Errorf("local snapshot is empty")
	}
	params = maps.Clone(params)
	params["snapshot_base64"] = "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	delete(params, "snapshot_url")
	payload = maps.Clone(payload)
	payload["params"] = params
	event.Payload = payload
	return event, nil
}
