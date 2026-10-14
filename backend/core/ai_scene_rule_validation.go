package core

import (
	"encoding/json"
	"fmt"
	"math"
)

// validateEnabledAISceneRulesConfig rejects duplicate scene types and enabled
// rules the scene analyzer would silently discard. Disabled rules remain valid drafts.
func validateEnabledAISceneRulesConfig(configJSON string) error {
	if configJSON == "" {
		return nil
	}
	var config struct {
		AISceneRules json.RawMessage `json:"ai_scene_rules"`
		EnableAI     *bool           `json:"enable_ai"`
	}
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		// Other device plugins own their config syntax.
		return nil
	}
	if len(config.AISceneRules) == 0 {
		return nil
	}
	if config.EnableAI != nil && !*config.EnableAI {
		// A global stop must remain possible for legacy invalid rules. Enabling
		// AI again requires the rule to be repaired or disabled first.
		return nil
	}
	var rules []struct {
		SceneType  string         `json:"scene_type"`
		Enabled    bool           `json:"enabled"`
		ROI        []aiScenePoint `json:"roi_points"`
		ROIRegions []struct {
			ROI []aiScenePoint `json:"roi_points"`
		} `json:"roi_regions"`
		SemanticVerification struct {
			Enabled         bool   `json:"enabled"`
			Mode            string `json:"mode"`
			CooldownSeconds int    `json:"cooldown_seconds"`
			Execution       string `json:"execution"`
		} `json:"semantic_verification"`
	}
	if err := json.Unmarshal(config.AISceneRules, &rules); err != nil {
		return fmt.Errorf("invalid ai_scene_rules: %w", err)
	}
	sceneTypes := make(map[string]bool, len(rules))
	for i, rule := range rules {
		if rule.SceneType != "" {
			if sceneTypes[rule.SceneType] {
				return fmt.Errorf("duplicate AI scene rule %s / 同一摄像机不能重复配置同一种AI规则", rule.SceneType)
			}
			sceneTypes[rule.SceneType] = true
		}
		if rule.Enabled && rule.SemanticVerification.Enabled {
			v := rule.SemanticVerification
			if rule.SceneType != "sleep_on_duty" && rule.SceneType != "person_fall" && rule.SceneType != "smoking" && rule.SceneType != "fighting" {
				return fmt.Errorf("rule %d does not support semantic review / 该规则不支持语义复核", i+1)
			}
			if v.Mode != "" && v.Mode != "verify_uncertain" && v.Mode != "review_local_alerts" {
				return fmt.Errorf("rule %d has invalid semantic review mode / 语义复核模式无效", i+1)
			}
			if v.Execution != "" && v.Execution != "local_first" && v.Execution != "platform" {
				return fmt.Errorf("rule %d has invalid semantic execution policy / 语义复核执行策略无效", i+1)
			}
			if v.CooldownSeconds != 0 && v.CooldownSeconds < 30 {
				return fmt.Errorf("rule %d semantic cooldown must be 0 (unlimited) or at least 30 seconds / 语义复核冷却为0表示不限制，或至少30秒", i+1)
			}
		}
		if !rule.Enabled || rule.SceneType != "area_intrusion" {
			continue
		}
		if len(rule.ROI) == 0 {
			allRegionsUndrawn := true
			for _, region := range rule.ROIRegions {
				if len(region.ROI) != 0 {
					allRegionsUndrawn = false
					break
				}
			}
			if allRegionsUndrawn {
				continue
			}
		}
		if len(rule.ROIRegions) == 0 {
			if !validAIScenePolygon(rule.ROI) {
				return fmt.Errorf("area_intrusion rule %d requires a valid ROI / 入侵规则需要有效区域", i+1)
			}
			continue
		}
		for j, region := range rule.ROIRegions {
			if !validAIScenePolygon(region.ROI) {
				return fmt.Errorf("area_intrusion rule %d region %d requires a valid ROI / 入侵规则区域需要有效范围", i+1, j+1)
			}
		}
	}
	return nil
}

type aiScenePoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func validAIScenePolygon(points []aiScenePoint) bool {
	if len(points) < 3 {
		return false
	}
	// Scene ROI coordinates use the plugin's 0..10000 source-frame scale.
	const sceneCoordScale = 10000.0
	var doubleArea float64
	for i, p := range points {
		if math.IsNaN(p.X) || math.IsNaN(p.Y) || p.X < 0 || p.X > sceneCoordScale || p.Y < 0 || p.Y > sceneCoordScale {
			return false
		}
		next := points[(i+1)%len(points)]
		doubleArea += p.X*next.Y - next.X*p.Y
	}
	return math.Abs(doubleArea) > sceneCoordScale*sceneCoordScale*1e-6
}
