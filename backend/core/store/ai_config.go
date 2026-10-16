package store

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var aiConfigKeys = map[string]bool{
	"enable_ai": true, "enable_yolo": true, "enable_yolo_webrtc": true,
	"enable_pose_annotation": true,
	"yolo_confidence":        true, "yolo_classes": true,
	"ai_basic_detection_groups": true, "ai_basic_detections": true,
	"ai_scene_rules": true,
}

// DeviceAIConfigView keeps independent AI readers away from device credentials
// and connection settings. Protocol selectors are needed to render AI controls.
func DeviceAIConfigView(raw string) string {
	var config map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &config) != nil {
		return "{}"
	}
	view := make(map[string]json.RawMessage)
	for key, value := range config {
		if aiConfigKeys[key] || key == "_protocol" || key == "protocol" {
			view[key] = value
		}
	}
	data, _ := json.Marshal(view)
	return string(data)
}

// DeviceAIConfigChanged also covers inventory/import writes, so callers cannot
// bypass AI authorization by choosing the general device endpoint.
func DeviceAIConfigChanged(before, after string) bool {
	decode := func(raw string) (map[string]interface{}, bool) {
		config := map[string]interface{}{}
		if raw == "" {
			return config, true
		}
		if json.Unmarshal([]byte(raw), &config) != nil || config == nil {
			return nil, false
		}
		return config, true
	}
	oldConfig, oldOK := decode(before)
	newConfig, newOK := decode(after)
	if !oldOK || !newOK {
		return true
	}
	// Disabled/empty defaults added by a protocol do not enable an AI capability.
	normalize := func(value interface{}) interface{} {
		switch v := value.(type) {
		case bool:
			if !v {
				return nil
			}
		case float64:
			if v == 0 {
				return nil
			}
		case []interface{}:
			if len(v) == 0 {
				return nil
			}
		}
		return value
	}
	for key := range aiConfigKeys {
		if !reflect.DeepEqual(normalize(oldConfig[key]), normalize(newConfig[key])) {
			return true
		}
	}
	return false
}

// MergeDeviceAIConfig applies only AI analysis fields. It cannot change device
// identity, connection settings, protocol parameters or point configuration.
func MergeDeviceAIConfig(current string, patch map[string]json.RawMessage) (string, error) {
	if len(patch) == 0 {
		return "", fmt.Errorf("AI configuration patch is empty")
	}
	config := make(map[string]json.RawMessage)
	if current != "" {
		if err := json.Unmarshal([]byte(current), &config); err != nil || config == nil {
			return "", fmt.Errorf("existing device configuration is not a JSON object")
		}
	}
	for key, value := range patch {
		if !aiConfigKeys[key] {
			return "", fmt.Errorf("%s is not an AI configuration field", key)
		}
		if len(value) == 0 || !json.Valid(value) || string(value) == "null" {
			return "", fmt.Errorf("invalid AI configuration field %s", key)
		}
		if err := validateAIConfigValue(key, value); err != nil {
			return "", err
		}
		config[key] = value
	}
	merged, err := json.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(merged), nil
}

func validateAIConfigValue(key string, value json.RawMessage) error {
	switch key {
	case "enable_ai", "enable_yolo", "enable_yolo_webrtc", "enable_pose_annotation":
		var enabled bool
		if err := json.Unmarshal(value, &enabled); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
	case "yolo_confidence":
		var confidence float64
		if err := json.Unmarshal(value, &confidence); err != nil || math.IsNaN(confidence) || confidence <= 0 || confidence > 1 {
			return fmt.Errorf("yolo_confidence must be between 0 and 1")
		}
	case "yolo_classes":
		var classes []string
		if err := json.Unmarshal(value, &classes); err != nil || classes == nil {
			return fmt.Errorf("yolo_classes must be an array of strings")
		}
	case "ai_basic_detection_groups", "ai_basic_detections", "ai_scene_rules":
		var entries []map[string]json.RawMessage
		if err := json.Unmarshal(value, &entries); err != nil || entries == nil {
			return fmt.Errorf("%s must be an array of objects", key)
		}
	}
	return nil
}

// UpdateDeviceAIConfig is a scoped maintenance exemption for the independent
// AI analysis module. Normal device edits continue through the write guard.
func UpdateDeviceAIConfig(code string, patch map[string]json.RawMessage) (*Device, error) {
	var updated Device
	err := DB.Transaction(func(tx *gorm.DB) error {
		var device Device
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("code = ?", code).First(&device).Error; err != nil {
			return err
		}
		config, err := MergeDeviceAIConfig(device.Config, patch)
		if err != nil {
			return err
		}
		if config != device.Config {
			if err := WithAutomaticConfigWrite(tx).Model(&device).Update("config", config).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("code = ?", code).First(&updated).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &updated, nil
}
