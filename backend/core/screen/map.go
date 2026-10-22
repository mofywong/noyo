package screen

import (
	"errors"
	"math"
)

func validateMapOption(value interface{}) error {
	option, ok := value.(map[string]interface{})
	if !ok {
		return errors.New("map options required")
	}
	if option["coordinateSystem"] != "wgs84" && option["coordinateSystem"] != "gcj02" {
		return errors.New("map coordinate system must be declared")
	}
	lines, ok := option["geoLines"].([]interface{})
	if !ok || len(lines) > 256 {
		return errors.New("invalid offline map outline")
	}
	count := 0
	for _, raw := range lines {
		line, ok := raw.([]interface{})
		if !ok || len(line) < 2 {
			return errors.New("invalid map line")
		}
		for _, rawPoint := range line {
			point, ok := rawPoint.([]interface{})
			if !ok || len(point) != 2 {
				return errors.New("invalid map position")
			}
			lon, lonOK := point[0].(float64)
			lat, latOK := point[1].(float64)
			if !lonOK || !latOK || math.IsNaN(lon) || math.IsNaN(lat) || math.IsInf(lon, 0) || math.IsInf(lat, 0) || lon < -180 || lon > 180 || lat < -90 || lat > 90 {
				return errors.New("map position out of bounds")
			}
			count++
			if count > 10000 {
				return errors.New("offline map outline exceeds 10000 positions")
			}
		}
	}
	return nil
}

func validateMapTargets(doc Envelope) error {
	maps := map[string]map[string]interface{}{}
	var collect func([]interface{})
	collect = func(items []interface{}) {
		for _, raw := range items {
			component, _ := raw.(map[string]interface{})
			config, _ := component["chartConfig"].(map[string]interface{})
			if config["key"] == "LocationMap" {
				id, _ := component["id"].(string)
				maps[id], _ = component["option"].(map[string]interface{})
			}
			if children, ok := component["groupList"].([]interface{}); ok {
				collect(children)
			}
		}
	}
	items, _ := doc.Content["componentList"].([]interface{})
	collect(items)
	for _, target := range doc.Targets {
		option, isMap := maps[target.ComponentID]
		if !isMap && target.Target != "map.points" {
			continue
		}
		binding := doc.Bindings[target.BindingID]
		if !isMap || target.Target != "map.points" || binding.Kind != "location" || binding.CoordinateSystem != option["coordinateSystem"] {
			return errors.New("map binding and outline must use the same declared coordinate system")
		}
	}
	return nil
}
