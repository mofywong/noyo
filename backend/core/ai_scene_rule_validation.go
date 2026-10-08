package core

import (
	"encoding/json"
	"fmt"
	"math"
)

// validateEnabledAISceneRulesConfig rejects enabled intrusion regions that the
// scene analyzer would silently discard. Disabled rules remain valid drafts.
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
	}
	if err := json.Unmarshal(config.AISceneRules, &rules); err != nil {
		return fmt.Errorf("invalid ai_scene_rules: %w", err)
	}
	for i, rule := range rules {
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
