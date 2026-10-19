package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"noyo/core/store"
)

// 物模型属性上的经纬度语义标记。
//
// 设计依据（方案文档 §8.1）：位置是物模型的语义，不是平行概念。
// 用语义标记表达，可以跟着 Product.Config 既有链路走，零迁移、零新增同步实体；
// 而新增一张绑定表等于新增一个配置类实体，要把 cascade 同步整条链路都接上。
const (
	SemanticLongitude = "longitude"
	SemanticLatitude  = "latitude"
	SemanticAltitude  = "altitude"
	SemanticAccuracy  = "accuracy"
	SemanticSpeed     = "speed"
	SemanticHeading   = "heading"
	SemanticFixTime   = "fixTime"
)

// 已知语义取值集合，用于校验与前端下拉。
var KnownLocationSemantics = []string{
	SemanticLongitude,
	SemanticLatitude,
	SemanticAltitude,
	SemanticAccuracy,
	SemanticSpeed,
	SemanticHeading,
	SemanticFixTime,
}

// LocationSemantics 把物模型里的语义标记解析成「属性 identifier → 角色」的映射。
type LocationSemantics struct {
	Longitude string `json:"longitude,omitempty"`
	Latitude  string `json:"latitude,omitempty"`
	Altitude  string `json:"altitude,omitempty"`
	Accuracy  string `json:"accuracy,omitempty"`
	Speed     string `json:"speed,omitempty"`
	Heading   string `json:"heading,omitempty"`
	FixTime   string `json:"fixTime,omitempty"`

	// AutoDetected 为 true 表示经/纬度来自 identifier 命名回退而非显式 semantic 标记。
	AutoDetected bool `json:"auto_detected,omitempty"`

	// PrecisionWarnings 记录跨层校验发现的问题（映射点 precision 落在 (0,6) 等）。
	PrecisionWarnings []string `json:"precision_warnings,omitempty"`
}

// Resolved 表示经纬度都识别到了。
func (s LocationSemantics) Resolved() bool {
	return s.Longitude != "" && s.Latitude != ""
}

// Mode 是定位来源模式。
//
// 模式判定零额外配置：物模型里有经纬度语义属性 ⇒ 动态设备（遥测），
// 没有 ⇒ 静态设备（手动标注）。一条规则同时完成"模式判定"与"编辑权限判定"。
func (s LocationSemantics) Mode() string {
	if s.Resolved() {
		return store.LocationSourceTelemetry
	}
	return store.LocationSourceManual
}

// Empty 表示物模型里没有任何定位语义。
func (s LocationSemantics) Empty() bool {
	return s.Longitude == "" && s.Latitude == "" &&
		s.Altitude == "" && s.Accuracy == "" &&
		s.Speed == "" && s.Heading == "" && s.FixTime == ""
}

// 语义归一化：去掉分隔符与大小写差异，便于比较（"fix_time" / "fixTime" / "FIXTIME" 等价）。
func normalizeSemanticName(raw string) string {
	raw = strings.TrimSpace(raw)
	var b strings.Builder
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
		}
	}
	return b.String()
}

// 显式语义标记 → 角色。
func semanticRole(normalized string) string {
	switch normalized {
	case "longitude", "lng", "lon", "long":
		return SemanticLongitude
	case "latitude", "lat":
		return SemanticLatitude
	case "altitude", "alt", "elevation", "height":
		return SemanticAltitude
	case "accuracy", "acc", "hdop", "precision":
		return SemanticAccuracy
	case "speed", "velocity":
		return SemanticSpeed
	case "heading", "course", "bearing", "direction":
		return SemanticHeading
	case "fixtime", "fixtimestamp", "gpstime", "timestamp":
		return SemanticFixTime
	default:
		return ""
	}
}

// 命名回退：物模型里没有 semantic 标记时，按 identifier 约定识别，让既有物模型免配置。
func fallbackRoleFromIdentifier(normalized string) string {
	if len(normalized) < 3 {
		return ""
	}
	hasSuffix := func(suffixes ...string) bool {
		for _, sfx := range suffixes {
			if strings.HasSuffix(normalized, sfx) {
				return true
			}
		}
		return false
	}
	switch {
	case hasSuffix("longitude", "lng", "lon"):
		return SemanticLongitude
	case hasSuffix("latitude", "lat"):
		return SemanticLatitude
	case hasSuffix("altitude", "elevation"):
		return SemanticAltitude
	case hasSuffix("accuracy"):
		return SemanticAccuracy
	case hasSuffix("speed", "velocity"):
		return SemanticSpeed
	case hasSuffix("heading"):
		return SemanticHeading
	default:
		return ""
	}
}

func assignSemanticRole(sem *LocationSemantics, role, identifier string) {
	if role == "" || identifier == "" {
		return
	}
	switch role {
	case SemanticLongitude:
		if sem.Longitude == "" {
			sem.Longitude = identifier
		}
	case SemanticLatitude:
		if sem.Latitude == "" {
			sem.Latitude = identifier
		}
	case SemanticAltitude:
		if sem.Altitude == "" {
			sem.Altitude = identifier
		}
	case SemanticAccuracy:
		if sem.Accuracy == "" {
			sem.Accuracy = identifier
		}
	case SemanticSpeed:
		if sem.Speed == "" {
			sem.Speed = identifier
		}
	case SemanticHeading:
		if sem.Heading == "" {
			sem.Heading = identifier
		}
	case SemanticFixTime:
		if sem.FixTime == "" {
			sem.FixTime = identifier
		}
	}
}

// ResolveLocationSemantics 从物模型配置 JSON 里解析定位语义。
//
// 两级识别：先看显式 `dataType.specs.semantic`，没有则按 identifier 命名回退。
// 显式标记始终优先 —— 回退只是为了让既有/导入的物模型免配置可用。
func ResolveLocationSemantics(productConfig string) LocationSemantics {
	if strings.TrimSpace(productConfig) == "" {
		return LocationSemantics{}
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(productConfig), &config); err != nil {
		return LocationSemantics{}
	}
	return ResolveLocationSemanticsFromConfig(config)
}

// ResolveLocationSemanticsFromConfig 与 ResolveLocationSemantics 等价，
// 但直接接受已解析的配置对象（运行时内存里的 ProductMeta.Config）。
func ResolveLocationSemanticsFromConfig(config map[string]any) LocationSemantics {
	if config == nil {
		return LocationSemantics{}
	}
	tsl := config
	if rawTSL, ok := config["tsl"].(map[string]any); ok {
		tsl = rawTSL
	}
	return resolveLocationSemanticsFromProperties(extractRuleProperties(tsl["properties"]))
}

func resolveLocationSemanticsFromProperties(properties []RulePropertyOption) LocationSemantics {
	var sem LocationSemantics
	var pending [][2]string // [role, identifier]，显式标记的待写入项（优先级更高）

	for _, prop := range properties {
		identifier := strings.TrimSpace(prop.Key)
		if identifier == "" {
			continue
		}

		explicit := ""
		if specs, ok := prop.Specs.(map[string]any); ok {
			if raw, ok := specs["semantic"].(string); ok {
				explicit = semanticRole(normalizeSemanticName(raw))
			}
		}
		if explicit != "" {
			pending = append(pending, [2]string{explicit, identifier})
			continue
		}

		if role := fallbackRoleFromIdentifier(normalizeSemanticName(identifier)); role != "" {
			// 回退命中先记下，但不覆盖显式标记
			if !sem.AutoDetected {
				sem.AutoDetected = true
			}
			assignSemanticRole(&sem, role, identifier)
		}
	}

	// 显式标记覆盖同名角色（assignSemanticRole 只在空位写入，所以先清空被显式声明的角色）
	for _, item := range pending {
		clearSemanticRole(&sem, item[0])
	}
	for _, item := range pending {
		assignSemanticRole(&sem, item[0], item[1])
	}
	return sem
}

func clearSemanticRole(sem *LocationSemantics, role string) {
	switch role {
	case SemanticLongitude:
		sem.Longitude = ""
	case SemanticLatitude:
		sem.Latitude = ""
	case SemanticAltitude:
		sem.Altitude = ""
	case SemanticAccuracy:
		sem.Accuracy = ""
	case SemanticSpeed:
		sem.Speed = ""
	case SemanticHeading:
		sem.Heading = ""
	case SemanticFixTime:
		sem.FixTime = ""
	}
}

// ===================== 写入护栏 =====================
//
// 以下三项校验是**必须自己实现的**：平台的属性写入路径不做 property 级
// min/max 校验（specs.min/max 只是 UI 提示）。见方案文档 §8.1 与 §12-6。

// NullIslandEpsilon 是 (0, 0) 哨兵判定的容差（度）。约 11 m。
const NullIslandEpsilon = 0.0001

// IsNullIsland 判断坐标是否为 GPS 未定位时常见的 (0, 0) 哨兵值。
// 若不识别，所有未定位设备会扎堆在几内亚湾。
func IsNullIsland(longitude, latitude float64) bool {
	return absFloat(longitude) <= NullIslandEpsilon && absFloat(latitude) <= NullIslandEpsilon
}

func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// ErrNullIsland 表示坐标是无有效定位的哨兵值。
var ErrNullIsland = fmt.Errorf("coordinate is the (0,0) null-island sentinel: no valid fix")

// ValidateLocationCoordinates 校验坐标合法性与哨兵值。
//
// 范围校验同时是"缩放系数配错"的探针：如果映射点漏写 read_expr，
// 上报的放大整数（如 116397428）会立刻被拦下，而不是安静地画到地图外面。
func ValidateLocationCoordinates(longitude, latitude float64) error {
	if IsNullIsland(longitude, latitude) {
		return ErrNullIsland
	}
	if longitude < -180 || longitude > 180 {
		return fmt.Errorf("longitude out of range: %v (expected -180..180)", longitude)
	}
	if latitude < -90 || latitude > 90 {
		return fmt.Errorf("latitude out of range: %v (expected -90..90)", latitude)
	}
	return nil
}

// MinCoordinatePrecision 是经纬度语义属性的映射点所允许的最小四舍五入位数。
//
// precision 落在 (0, 6) 区间会静默毁掉坐标：
// 2 ≈ 1.1 km、4 ≈ 11 m、5 ≈ 1.1 m、6 ≈ 0.11 m（推荐下限），0 = 不取整。
const MinCoordinatePrecision = 6

// ValidateMappingPrecision 校验「被标记为经纬度语义的属性」其映射点的 precision。
// 返回非 nil 表示配置有问题，应当阻止保存或强告警。
func ValidateMappingPrecision(semantic string, precision int) error {
	switch semantic {
	case SemanticLongitude, SemanticLatitude:
	default:
		return nil
	}
	if precision > 0 && precision < MinCoordinatePrecision {
		return fmt.Errorf(
			"precision %d is too coarse for %s: about %.0f m of error; use 0 (no rounding) or >= %d",
			precision, semantic, coordinateErrorMeters(precision), MinCoordinatePrecision)
	}
	return nil
}

// coordinateErrorMeters 把小数位数量换算成赤道附近的近似误差（米）。
func coordinateErrorMeters(precision int) float64 {
	const metersPerDegree = 111320.0
	err := metersPerDegree
	for i := 0; i < precision; i++ {
		err /= 10
	}
	return err
}

// CollectMappingPrecisions 从协议映射配置 JSON 里递归收集「点位 key → precision」。
//
// 映射配置的结构因协议而异（Modbus 有子设备 points、BACnet 有对象列表），
// 所以这里不假设具体层级，只按特征字段识别：同时出现 precision 与 key/name/identifier 的对象。
func CollectMappingPrecisions(configJSON string) map[string]int {
	out := make(map[string]int)
	if strings.TrimSpace(configJSON) == "" {
		return out
	}
	var root any
	if err := json.Unmarshal([]byte(configJSON), &root); err != nil {
		return out
	}
	collectMappingPrecisions(root, out)
	return out
}

func collectMappingPrecisions(node any, out map[string]int) {
	switch typed := node.(type) {
	case map[string]any:
		if precision, ok := typed["precision"]; ok {
			key := firstStringValue(typed, "key", "identifier", "name", "property")
			if key != "" {
				if p, ok := numericValue(precision); ok {
					out[key] = int(p)
				}
			}
		}
		for _, v := range typed {
			collectMappingPrecisions(v, out)
		}
	case []any:
		for _, item := range typed {
			collectMappingPrecisions(item, out)
		}
	}
}

func firstStringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := m[key].(string); ok && strings.TrimSpace(raw) != "" {
			return strings.TrimSpace(raw)
		}
	}
	return ""
}

// ValidateLocationMapping 做跨层校验：物模型的经纬度语义属性 ↔ 协议映射点的 precision。
// 返回人类可读的告警列表（空表示干净）。
func ValidateLocationMapping(sem LocationSemantics, mappingConfigs ...string) []string {
	if len(mappingConfigs) == 0 {
		return nil
	}
	precisions := make(map[string]int)
	for _, cfg := range mappingConfigs {
		for key, precision := range CollectMappingPrecisions(cfg) {
			precisions[key] = precision
		}
	}

	var warnings []string
	check := func(semantic, identifier string) {
		if identifier == "" {
			return
		}
		precision, ok := precisions[identifier]
		if !ok {
			return
		}
		if err := ValidateMappingPrecision(semantic, precision); err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", identifier, err))
		}
	}
	check(SemanticLongitude, sem.Longitude)
	check(SemanticLatitude, sem.Latitude)
	return warnings
}

// ===================== 从属性值抽取定位样本 =====================

// LocationSample 是一次属性上报里抽出的定位样本。
type LocationSample struct {
	Longitude  float64
	Latitude   float64
	Altitude   *float64
	AccuracyM  *float64
	SpeedMPS   *float64
	HeadingDeg *float64
	FixAt      int64
}

// ExtractLocationSample 按语义映射从属性集合里抽位置。
// 返回 ok=false 表示这次上报不构成一个可用的定位（缺经纬度、哨兵值或超范围）。
func ExtractLocationSample(sem LocationSemantics, properties map[string]interface{}) (LocationSample, bool) {
	var sample LocationSample
	if properties == nil || !sem.Resolved() {
		return sample, false
	}

	lng, okLng := numericValue(properties[sem.Longitude])
	lat, okLat := numericValue(properties[sem.Latitude])
	if !okLng || !okLat {
		return sample, false
	}
	if err := ValidateLocationCoordinates(lng, lat); err != nil {
		return sample, false
	}
	sample.Longitude = lng
	sample.Latitude = lat

	if v, ok := optionalNumeric(properties[sem.Altitude]); ok {
		sample.Altitude = &v
	}
	if v, ok := optionalNumeric(properties[sem.Accuracy]); ok {
		sample.AccuracyM = &v
	}
	if v, ok := optionalNumeric(properties[sem.Speed]); ok {
		sample.SpeedMPS = &v
	}
	if v, ok := optionalNumeric(properties[sem.Heading]); ok {
		sample.HeadingDeg = &v
	}
	if v, ok := optionalNumeric(properties[sem.FixTime]); ok {
		sample.FixAt = normalizeEpochMillis(v)
	}
	return sample, true
}

func optionalNumeric(raw interface{}) (float64, bool) {
	if raw == nil {
		return 0, false
	}
	return numericValue(raw)
}

// normalizeEpochMillis 把可能是秒、毫秒或字符串的时间值统一成毫秒。
func normalizeEpochMillis(v float64) int64 {
	switch {
	case v <= 0:
		return 0
	case v < 1e11: // 秒
		return int64(v * 1000)
	default: // 毫秒
		return int64(v)
	}
}
