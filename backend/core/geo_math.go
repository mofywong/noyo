package core

import (
	"encoding/json"
	"fmt"
	"math"
)

// 地理计算工具。noyo 用 SQLite，没有 PostGIS，所以空间判定在 Go 侧完成。
//
// 精度说明：围栏尺度通常在数十米到数公里，在这个尺度上把经纬度当平面处理
// （纬度方向 1° ≈ 111320 m，经度方向按 cos(lat) 收缩）带来的误差远小于
// GPS 自身的精度半径，因此不必引入完整的大地测量库。

const (
	metersPerDegreeLat = 111320.0
	earthRadiusMeters  = 6371000.0
)

// haversineMeters 返回两点间的大圆距离（米）。
func haversineMeters(lng1, lat1, lng2, lat2 float64) float64 {
	radLat1 := lat1 * math.Pi / 180
	radLat2 := lat2 * math.Pi / 180
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180

	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)
	a := sinLat*sinLat + math.Cos(radLat1)*math.Cos(radLat2)*sinLng*sinLng
	return 2 * earthRadiusMeters * math.Asin(math.Min(1, math.Sqrt(a)))
}

// lngLatToLocalMeters 把经纬度差值换算成以 (originLng, originLat) 为原点的平面米坐标。
func lngLatToLocalMeters(lng, lat, originLng, originLat float64) (x, y float64) {
	x = (lng - originLng) * metersPerDegreeLat * math.Cos(originLat*math.Pi/180)
	y = (lat - originLat) * metersPerDegreeLat
	return x, y
}

// GeoJSONRing 是多边形外环：[[lng, lat], ...]。
type GeoJSONRing [][2]float64

// ParseGeoJSONPolygonRing 解析 GeoJSON Polygon 的外环。
// 同时兼容：
//   - 完整 GeoJSON 几何对象 {"type":"Polygon","coordinates":[[[...]]]}
//   - 直接给坐标数组 [[[lng,lat],...]]
func ParseGeoJSONPolygonRing(geometry string) (GeoJSONRing, error) {
	if geometry == "" {
		return nil, fmt.Errorf("empty geometry")
	}

	var raw any
	if err := json.Unmarshal([]byte(geometry), &raw); err != nil {
		return nil, fmt.Errorf("invalid geometry json: %w", err)
	}

	coordinates := raw
	if obj, ok := raw.(map[string]any); ok {
		geometryType, _ := obj["type"].(string)
		if geometryType != "" && geometryType != "Polygon" {
			return nil, fmt.Errorf("unsupported geometry type %q (only Polygon)", geometryType)
		}
		coordinates = obj["coordinates"]
	}

	rings, ok := coordinates.([]any)
	if !ok || len(rings) == 0 {
		return nil, fmt.Errorf("polygon coordinates must be a non-empty array")
	}
	outer, ok := rings[0].([]any)
	if !ok || len(outer) < 3 {
		return nil, fmt.Errorf("polygon outer ring needs at least 3 points")
	}

	ring := make(GeoJSONRing, 0, len(outer))
	for _, item := range outer {
		pair, ok := item.([]any)
		if !ok || len(pair) < 2 {
			return nil, fmt.Errorf("polygon point must be [lng, lat]")
		}
		lng, okLng := numericValue(pair[0])
		lat, okLat := numericValue(pair[1])
		if !okLng || !okLat {
			return nil, fmt.Errorf("polygon point must be numeric")
		}
		ring = append(ring, [2]float64{lng, lat})
	}
	return ring, nil
}

// PointInRing 用射线法判断点是否落在多边形内（经纬度按平面处理）。
func PointInRing(lng, lat float64, ring GeoJSONRing) bool {
	inside := false
	n := len(ring)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		xi, yi := ring[i][0], ring[i][1]
		xj, yj := ring[j][0], ring[j][1]
		intersect := (yi > lat) != (yj > lat)
		if intersect {
			xCross := (xj-xi)*(lat-yi)/(yj-yi) + xi
			if lng < xCross {
				inside = !inside
			}
		}
	}
	return inside
}

// DistanceToRingBoundaryMeters 返回点到多边形边界的最短距离（米）。
// 用于构造"靠近边界 ±accuracy 时视为不确定"的不确定带。
func DistanceToRingBoundaryMeters(lng, lat float64, ring GeoJSONRing) float64 {
	if len(ring) == 0 {
		return math.Inf(1)
	}
	minDistance := math.Inf(1)
	n := len(ring)
	for i := 0; i < n; i++ {
		x1, y1 := ring[i][0], ring[i][1]
		x2, y2 := ring[(i+1)%n][0], ring[(i+1)%n][1]

		ax, ay := lngLatToLocalMeters(x1, y1, lng, lat)
		bx, by := lngLatToLocalMeters(x2, y2, lng, lat)
		distance := pointToSegmentDistanceMeters(0, 0, ax, ay, bx, by)
		if distance < minDistance {
			minDistance = distance
		}
	}
	return minDistance
}

// pointToSegmentDistanceMeters 计算点 (px,py) 到线段 (ax,ay)-(bx,by) 的距离（米，平面坐标）。
func pointToSegmentDistanceMeters(px, py, ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// RingBBox 返回环的包围盒（minLng, minLat, maxLng, maxLat）。
func RingBBox(ring GeoJSONRing) (float64, float64, float64, float64) {
	if len(ring) == 0 {
		return 0, 0, 0, 0
	}
	minLng, minLat := ring[0][0], ring[0][1]
	maxLng, maxLat := minLng, minLat
	for _, point := range ring {
		minLng = math.Min(minLng, point[0])
		maxLng = math.Max(maxLng, point[0])
		minLat = math.Min(minLat, point[1])
		maxLat = math.Max(maxLat, point[1])
	}
	return minLng, minLat, maxLng, maxLat
}

// CircleBBox 返回圆的包围盒（用半径换算成经纬度跨度，供预筛使用）。
func CircleBBox(centerLng, centerLat, radiusM float64) (float64, float64, float64, float64) {
	latSpan := radiusM / metersPerDegreeLat
	cosLat := math.Cos(centerLat * math.Pi / 180)
	if math.Abs(cosLat) < 1e-6 {
		cosLat = 1e-6
	}
	lngSpan := radiusM / (metersPerDegreeLat * math.Abs(cosLat))
	return centerLng - lngSpan, centerLat - latSpan, centerLng + lngSpan, centerLat + latSpan
}

// ===================== 多边形围栏几何 =====================
//
// 只支持「单外环」的 Polygon 是不够的：按行政区名称生成的边界普遍是
// MultiPolygon（海岛、飞地、被河流切开的区块），而且带内环（洞）。
// 这一层把 GeoJSON 统一解析成 FenceShape，判定只面对这一种结构。
//
// 性能约束：市级边界动辄上万个顶点，而判定发生在**每台设备每次位置落库**时。
// 所以这里做两件事：
//   1. 解析只在围栏缓存装载时做一次，绝不在每次判定时重新 Unmarshal；
//   2. 预先算出「简化环」，只用于 ±accuracy 不确定带的距离计算 ——
//      包含判定仍用完整顶点，精度不受影响。

// FencePolygon 是一个多边形的外环与其内环（洞）。
type FencePolygon struct {
	Outer GeoJSONRing
	Holes []GeoJSONRing
}

// FenceShape 是围栏几何的判定视图。
type FenceShape struct {
	Polygons []FencePolygon

	// BBox 是全部多边形的联合包围盒，用于 O(1) 排除。
	minLng, minLat, maxLng, maxLat float64

	// 简化环：与 Polygons 一一对应，只用于不确定带距离计算。
	bandPolygons []FencePolygon
}

// 不确定带距离计算的顶点上限。400 个顶点在 ±accuracy（通常 10~50 m）尺度上
// 的边距误差远小于 GPS 自身误差。
const fenceBandMaxPoints = 400

// ParseFenceShape 解析 GeoJSON 几何为判定视图。
// 兼容 Polygon / MultiPolygon / Feature 包装；坐标一律 WGS-84。
func ParseFenceShape(geometry string) (*FenceShape, error) {
	if geometry == "" {
		return nil, fmt.Errorf("empty geometry")
	}
	var raw any
	if err := json.Unmarshal([]byte(geometry), &raw); err != nil {
		return nil, fmt.Errorf("invalid geometry json: %w", err)
	}

	obj, ok := raw.(map[string]any)
	if !ok {
		// 直接给坐标数组时按 Polygon 处理，兼容旧数据。
		obj = map[string]any{"type": "Polygon", "coordinates": raw}
	}
	geomType, _ := obj["type"].(string)
	if geomType == "Feature" {
		inner, _ := obj["geometry"].(map[string]any)
		if inner == nil {
			return nil, fmt.Errorf("feature has no geometry")
		}
		obj = inner
		geomType, _ = obj["type"].(string)
	}

	var polygons [][][][]float64
	switch geomType {
	case "Polygon":
		poly, err := parsePolygonCoordinates(obj["coordinates"])
		if err != nil {
			return nil, err
		}
		polygons = [][][][]float64{poly}
	case "MultiPolygon":
		list, ok := obj["coordinates"].([]any)
		if !ok || len(list) == 0 {
			return nil, fmt.Errorf("multipolygon coordinates must be a non-empty array")
		}
		for _, item := range list {
			poly, err := parsePolygonCoordinates(item)
			if err != nil {
				return nil, err
			}
			polygons = append(polygons, poly)
		}
	default:
		return nil, fmt.Errorf("unsupported geometry type %q (Polygon or MultiPolygon)", geomType)
	}

	shape := &FenceShape{}
	for _, poly := range polygons {
		if len(poly) == 0 {
			continue
		}
		entry := FencePolygon{Outer: ringFromRaw(poly[0])}
		if len(entry.Outer) < 3 {
			continue
		}
		for _, hole := range poly[1:] {
			ring := ringFromRaw(hole)
			if len(ring) >= 3 {
				entry.Holes = append(entry.Holes, ring)
			}
		}
		shape.Polygons = append(shape.Polygons, entry)
	}
	if len(shape.Polygons) == 0 {
		return nil, fmt.Errorf("geometry has no usable polygon ring")
	}

	shape.minLng, shape.minLat, shape.maxLng, shape.maxLat = shape.Polygons[0].Outer[0][0], shape.Polygons[0].Outer[0][1], shape.Polygons[0].Outer[0][0], shape.Polygons[0].Outer[0][1]
	for _, poly := range shape.Polygons {
		minLng, minLat, maxLng, maxLat := RingBBox(poly.Outer)
		shape.minLng = math.Min(shape.minLng, minLng)
		shape.minLat = math.Min(shape.minLat, minLat)
		shape.maxLng = math.Max(shape.maxLng, maxLng)
		shape.maxLat = math.Max(shape.maxLat, maxLat)

		band := FencePolygon{Outer: decimateRing(poly.Outer, fenceBandMaxPoints)}
		for _, hole := range poly.Holes {
			band.Holes = append(band.Holes, decimateRing(hole, fenceBandMaxPoints))
		}
		shape.bandPolygons = append(shape.bandPolygons, band)
	}
	return shape, nil
}

// parsePolygonCoordinates 把 [[ring],[hole]...] 解析成 [][][]float64。
func parsePolygonCoordinates(raw any) ([][][]float64, error) {
	rings, ok := raw.([]any)
	if !ok || len(rings) == 0 {
		return nil, fmt.Errorf("polygon coordinates must be a non-empty array")
	}
	out := make([][][]float64, 0, len(rings))
	for _, item := range rings {
		points, ok := item.([]any)
		if !ok || len(points) < 3 {
			return nil, fmt.Errorf("polygon ring needs at least 3 points")
		}
		ring := make([][]float64, 0, len(points))
		for _, point := range points {
			pair, ok := point.([]any)
			if !ok || len(pair) < 2 {
				return nil, fmt.Errorf("polygon point must be [lng, lat]")
			}
			lng, okLng := numericValue(pair[0])
			lat, okLat := numericValue(pair[1])
			if !okLng || !okLat {
				return nil, fmt.Errorf("polygon point must be numeric")
			}
			ring = append(ring, []float64{lng, lat})
		}
		out = append(out, ring)
	}
	return out, nil
}

func ringFromRaw(raw [][]float64) GeoJSONRing {
	ring := make(GeoJSONRing, 0, len(raw))
	for _, point := range raw {
		if len(point) < 2 {
			continue
		}
		ring = append(ring, [2]float64{point[0], point[1]})
	}
	return ring
}

// decimateRing 等距抽稀到 maxPoints 个顶点（首尾保持）。
// 只用于不确定带距离计算，不参与包含判定。
func decimateRing(ring GeoJSONRing, maxPoints int) GeoJSONRing {
	if len(ring) <= maxPoints || maxPoints < 4 {
		return ring
	}
	step := int(math.Ceil(float64(len(ring)) / float64(maxPoints)))
	out := make(GeoJSONRing, 0, maxPoints+1)
	for i := 0; i < len(ring); i += step {
		out = append(out, ring[i])
	}
	last := ring[len(ring)-1]
	if tail := out[len(out)-1]; tail[0] != last[0] || tail[1] != last[1] {
		out = append(out, last)
	}
	return out
}

// IsEmpty 表示没有任何可用多边形。
func (s *FenceShape) IsEmpty() bool {
	return s == nil || len(s.Polygons) == 0
}

// BBox 返回联合包围盒。
func (s *FenceShape) BBox() (float64, float64, float64, float64) {
	if s == nil {
		return 0, 0, 0, 0
	}
	return s.minLng, s.minLat, s.maxLng, s.maxLat
}

// PolygonCount 返回独立多边形个数（地区围栏可能由多个区块组成）。
func (s *FenceShape) PolygonCount() int {
	if s == nil {
		return 0
	}
	return len(s.Polygons)
}

// Contains 判断点是否落在围栏内：在任一外环内且不在该外环的任何洞内。
func (s *FenceShape) Contains(lng, lat float64) bool {
	if s == nil {
		return false
	}
	if lng < s.minLng || lng > s.maxLng || lat < s.minLat || lat > s.maxLat {
		return false
	}
	for _, poly := range s.Polygons {
		if !PointInRing(lng, lat, poly.Outer) {
			continue
		}
		inHole := false
		for _, hole := range poly.Holes {
			if PointInRing(lng, lat, hole) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

// DistanceToBoundaryMeters 返回点到围栏边界的近似最短距离（米），用简化环计算。
func (s *FenceShape) DistanceToBoundaryMeters(lng, lat float64) float64 {
	if s == nil || len(s.bandPolygons) == 0 {
		return math.Inf(1)
	}
	minDistance := math.Inf(1)
	for _, poly := range s.bandPolygons {
		if d := DistanceToRingBoundaryMeters(lng, lat, poly.Outer); d < minDistance {
			minDistance = d
		}
		for _, hole := range poly.Holes {
			if d := DistanceToRingBoundaryMeters(lng, lat, hole); d < minDistance {
				minDistance = d
			}
		}
	}
	return minDistance
}
