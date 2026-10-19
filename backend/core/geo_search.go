package core

// 地点搜索与行政区边界。
//
// 选型（全免费、无需 Key、输出 WGS-84）：
//   - 地点搜索：Photon（komoot），OSM 数据，中文环境返回中文地名；
//   - 行政区边界：Nominatim，`polygon_geojson=1` 可取到省 / 市 / 区县 / 街道
//     的多边形（含 MultiPolygon 与洞）。
//
// 两条都是公共服务，有明确的使用约束：必须带可识别的 User-Agent、
// Nominatim 限速 1 次/秒。因此这里统一做三件事：**串行限速 + 结果缓存 +
// 边界简化**。判定与存储完全不依赖具体服务商 —— 换成天地图官方服务
// （带审图号、需服务端 Key）只需要替换下面两个 fetch 函数。
//
// 合规说明：这里取到的边界只作为「用户定义的围栏几何」，不参与底图渲染；
// 生产环境面向大陆客户时，底图仍应使用天地图（官方审图号）。

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	geoUserAgent = "noyo-platform/1.0 (device-map geofence; contact: platform-admin)"

	photonSearchEndpoint   = "https://photon.komoot.io/api/"
	nominatimSearchEndpoint = "https://nominatim.openstreetmap.org/search"

	// Nominatim 的公开使用策略要求 ≤1 次/秒，留一点余量。
	nominatimMinInterval = 1100 * time.Millisecond

	// 单次尝试的超时。整条链路的预算另由 geoTotalBudget 兜底，
	// 不能靠「单次超时 × 次数」相乘 —— 那会算出分钟级的接口耗时。
	geoHTTPTimeout = 12 * time.Second
	// 一次逻辑请求的总预算（含重试与退避）。超过就认输，交给用户重试，
	// 而不是让前端转圈到用户以为页面卡死。
	geoTotalBudget = 26 * time.Second
	// 重试次数。实测该链路的失败几乎都是"连接被中途掐断"，
	// 换一条新连接基本立刻成功，因此宁可重试也不要一次失败就报错。
	geoAttempts     = 3
	geoRetryBackoff = 400 * time.Millisecond

	geocodeCacheTTL = 30 * time.Minute
	regionCacheTTL  = 12 * time.Hour
	geoCacheMaxSize = 256

	// 单个环简化后的顶点上限。行政边界原始数据常有数万点，
	// 判定与渲染都用不着这个精度；3000 点已经能保留道路级轮廓。
	regionRingMaxPoints = 3000
	// 多区块边界保留的最大区块数（按面积降序）。海岛城市会有几百个碎块。
	regionMaxPolygons = 200
	// 面积小于最大区块该比例的碎块直接丢弃，避免几万个珊瑚礁小岛撑爆存储。
	regionMinAreaRatio = 5e-6
)

// GeocodePlace 是一条地点搜索结果。坐标一律 WGS-84。
type GeocodePlace struct {
	Name     string  `json:"name"`
	Address  string  `json:"address"`
	Lng      float64 `json:"lng"`
	Lat      float64 `json:"lat"`
	Kind     string  `json:"kind"`     // country/province/city/district/town/village/street/poi
	Boundary bool    `json:"boundary"` // 是否可能取到面状边界（可"设为围栏"）
}

// RegionBoundary 是行政区边界查询结果。
type RegionBoundary struct {
	Name        string     `json:"name"`
	DisplayName string     `json:"display_name"`
	Level       string     `json:"level"`
	Geometry    string     `json:"geometry"` // GeoJSON Polygon / MultiPolygon（WGS-84）
	BBox        [4]float64 `json:"bbox"`     // minLng, minLat, maxLng, maxLat
	Points      int        `json:"points"`
	Polygons    int        `json:"polygons"`
	Source      string     `json:"source"`
	Simplified  bool       `json:"simplified"`
}

// ===================== 带限速与缓存的 HTTP =====================

// 分类错误。前端要靠它决定"提示重试"还是"提示换个名字/改用手绘"，
// 所以不能只扔一个拼好的字符串出去 —— 更不能把上游 URL 原样抛给用户。
var (
	// ErrGeoUnavailable 表示上游服务当前够不着（连接被断、超时、5xx、限流）。
	// 语义是"这次没成，但过一会可能成"。
	ErrGeoUnavailable = errors.New("geocoding service is temporarily unreachable")
	// ErrGeoRejected 表示上游明确拒绝了这个请求（4xx），重试没有意义。
	ErrGeoRejected = errors.New("geocoding service rejected the request")
	// ErrGeoNoMatch 表示服务连通，但没有匹配到任何地点。
	ErrGeoNoMatch = errors.New("no place matched the keyword")
	// ErrGeoNoBoundary 表示匹配到了 POI，但它没有面状边界（商场、学校）。
	ErrGeoNoBoundary = errors.New("matched place has no administrative boundary")
)

// geoUserError 是一种"可以安全回给前端"的错误：已分类的原因码 + 一句不含
// 上游细节的说明。上游 URL、底层 EOF 之类的东西只进日志，不进响应体。
type geoUserError struct {
	Code   int
	Reason string
	Detail string
}

func (e *geoUserError) Error() string {
	return e.Detail
}

var (
	// geoHTTPClient 刻意关掉 keep-alive。
	//
	// 线上报过 `Get "https://nominatim…": EOF`：Go 默认复用空闲连接，而这两种
	// 中间设备很常见 —— 企业准入/零信任客户端、以及各种 TLS 隧道 —— 会静默掐断
	// 空闲连接，下一个请求复用时就立刻拿到 EOF。实测同一台机器上"每次新建连接"
	// 连续 12/12 成功，而复用会偶发失败。
	//
	// 代价可以忽略：一条围栏一辈子只查几次边界，不复用连接省下的握手成本
	// 远小于一次失败重试。
	geoHTTPClient = &http.Client{
		Timeout: geoHTTPTimeout,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DisableKeepAlives:   true,
			ForceAttemptHTTP2:   true,
			MaxIdleConnsPerHost: 1,
		},
	}

	geoRateMu        sync.Mutex
	geoLastNominatim time.Time

	geoCacheMu sync.Mutex
	geoCache   = map[string]geoCacheEntry{}
)

type geoCacheEntry struct {
	value  any
	expire time.Time
}

func geoCacheGet(key string) (any, bool) {
	geoCacheMu.Lock()
	defer geoCacheMu.Unlock()
	entry, ok := geoCache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expire) {
		delete(geoCache, key)
		return nil, false
	}
	return entry.value, true
}

func geoCachePut(key string, value any, ttl time.Duration) {
	geoCacheMu.Lock()
	defer geoCacheMu.Unlock()
	if len(geoCache) >= geoCacheMaxSize {
		// 简单淘汰：先清过期项，仍然超限就整体清空。缓存丢了只是慢一点。
		now := time.Now()
		for k, entry := range geoCache {
			if now.After(entry.expire) {
				delete(geoCache, k)
			}
		}
		if len(geoCache) >= geoCacheMaxSize {
			geoCache = map[string]geoCacheEntry{}
		}
	}
	geoCache[key] = geoCacheEntry{value: value, expire: time.Now().Add(ttl)}
}

// waitNominatimSlot 串行化 Nominatim 调用，满足其 1 次/秒的使用约束。
func waitNominatimSlot() {
	geoRateMu.Lock()
	elapsed := time.Since(geoLastNominatim)
	if elapsed < nominatimMinInterval {
		time.Sleep(nominatimMinInterval - elapsed)
	}
	geoLastNominatim = time.Now()
	geoRateMu.Unlock()
}

// geoStatusError 携带上游返回的非 200 状态码，供重试策略判断是否值得再试。
type geoStatusError struct {
	code int
}

func (e *geoStatusError) Error() string {
	return fmt.Sprintf("upstream returned HTTP %d", e.code)
}

// retryable 判定这个状态码重新发一次有没有意义。
// 429 与 5xx 是"服务端临时状态"，4xx 是"你这个请求本身有问题"，重试只会白等。
func (e *geoStatusError) retryable() bool {
	return e.code == http.StatusTooManyRequests || e.code >= 500
}

// geoGetJSON 带重试地拉取一个 JSON。
//
// beforeAttempt 用来在每次尝试前做限速把门（Nominatim 的 1 次/秒）。
func geoGetJSON(rawURL string, out any, beforeAttempt func()) error {
	var lastErr error
	deadline := time.Now().Add(geoTotalBudget)
	for attempt := 1; attempt <= geoAttempts; attempt++ {
		if attempt > 1 {
			// 退避：给中间设备换一条干净连接留出时间。
			time.Sleep(geoRetryBackoff * time.Duration(attempt-1))
		}
		if time.Now().After(deadline) {
			break
		}
		if beforeAttempt != nil {
			beforeAttempt()
		}

		err := geoGetJSONOnce(rawURL, out)
		if err == nil {
			return nil
		}
		lastErr = err

		var statusErr *geoStatusError
		if errors.As(err, &statusErr) && !statusErr.retryable() {
			return fmt.Errorf("%w: %v", ErrGeoRejected, err)
		}
	}
	// 走到这里说明重试也没成。对外只暴露"够不着服务"，不暴露上游 URL 与
	// 底层错误详情 —— 用户看不懂，也不该看到我们请求了哪个地址。
	return fmt.Errorf("%w: %v", ErrGeoUnavailable, lastErr)
}

func geoGetJSONOnce(rawURL string, out any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", geoUserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")

	resp, err := geoHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 24<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return &geoStatusError{code: resp.StatusCode}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("upstream returned invalid json: %w", err)
	}

	return nil
}

// ===================== 地点搜索 =====================

type photonResponse struct {
	Features []struct {
		Geometry struct {
			Coordinates []float64 `json:"coordinates"`
		} `json:"geometry"`
		Properties struct {
			Name    string `json:"name"`
			Street  string `json:"street"`
			City    string `json:"city"`
			County  string `json:"county"`
			State   string `json:"state"`
			Country string `json:"country"`
			OSMKey  string `json:"osm_key"`
			OSMValue string `json:"osm_value"`
		} `json:"properties"`
	} `json:"features"`
}

// SearchPlaces 按名称搜索地点。返回 WGS-84 坐标。
func (s *Server) SearchPlaces(query string, limit int) ([]GeocodePlace, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if limit <= 0 || limit > 20 {
		limit = 8
	}
	key := fmt.Sprintf("search:%s:%d", strings.ToLower(query), limit)
	if cached, ok := geoCacheGet(key); ok {
		if places, okCast := cached.([]GeocodePlace); okCast {
			return places, nil
		}
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", strconv.Itoa(limit))
	places, err := fetchPhotonPlaces(photonSearchEndpoint + "?" + params.Encode())
	if err != nil {
		// Photon 挂了就退到 Nominatim —— 只是没有结构化地址，坐标一样可用。
		places, err = fetchNominatimPlaces(query, limit)
		if err != nil {
			if s != nil && s.Logger != nil {
				s.Logger.Warn("Place search failed", zap.String("query", query), zap.Error(err))
			}
			return nil, err
		}
	}
	geoCachePut(key, places, geocodeCacheTTL)
	return places, nil
}

func fetchPhotonPlaces(rawURL string) ([]GeocodePlace, error) {
	var resp photonResponse
	if err := geoGetJSON(rawURL, &resp, nil); err != nil {
		return nil, err
	}
	places := make([]GeocodePlace, 0, len(resp.Features))
	for _, feature := range resp.Features {
		coords := feature.Geometry.Coordinates
		if len(coords) < 2 {
			continue
		}
		props := feature.Properties
		kind := photonKind(props.OSMKey, props.OSMValue)
		name := strings.TrimSpace(props.Name)
		if name == "" {
			name = strings.TrimSpace(props.Street)
		}
		places = append(places, GeocodePlace{
			Name:     name,
			Address:  joinAddressParts(props.State, props.City, props.County, props.Country),
			Lng:      coords[0],
			Lat:      coords[1],
			Kind:     kind,
			Boundary: isAdministrativeKind(kind),
		})
	}
	return places, nil
}

func fetchNominatimPlaces(query string, limit int) ([]GeocodePlace, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "jsonv2")
	params.Set("limit", strconv.Itoa(limit))
	params.Set("accept-language", "zh-CN,zh")
	// 复用 nominatimBoundaryRow：同名行结构，且它已经处理了
	// jsonv2 的 `category` 字段名差异，不要在这里另造一份匿名结构。
	var rows []nominatimBoundaryRow
	if err := geoGetJSON(nominatimSearchEndpoint+"?"+params.Encode(), &rows, waitNominatimSlot); err != nil {
		return nil, err
	}
	places := make([]GeocodePlace, 0, len(rows))
	for _, row := range rows {
		lng, errLng := strconv.ParseFloat(row.Lon, 64)
		lat, errLat := strconv.ParseFloat(row.Lat, 64)
		if errLng != nil || errLat != nil {
			continue
		}
		kind := nominatimKind(row.osmCategory(), row.Type, row.Addresstype, row.Name)
		places = append(places, GeocodePlace{
			Name:     firstNonEmpty(row.Name, row.DisplayName),
			Address:  row.DisplayName,
			Lng:      lng,
			Lat:      lat,
			Kind:     kind,
			Boundary: isAdministrativeKind(kind),
		})
	}
	return places, nil
}

func photonKind(osmKey, osmValue string) string {
	value := strings.ToLower(strings.TrimSpace(osmValue))
	if strings.EqualFold(osmKey, "place") {
		switch value {
		case "country":
			return "country"
		case "state", "province", "region":
			return "province"
		case "county":
			return "city"
		case "city", "town":
			return "city"
		case "district", "suburb", "borough":
			return "district"
		case "village", "hamlet", "locality", "quarter", "neighbourhood":
			return "town"
		}
	}
	if strings.EqualFold(osmKey, "highway") || strings.EqualFold(osmKey, "waterway") {
		return "street"
	}
	return "poi"
}

// nominatimKind 把 OSM 的 class/type/addresstype 映射成本产品的前端分类。
//
// 中国的行政层级必须先看名字后缀：OSM 把**市辖区一律标成 addresstype=city**
// （福田区、西湖区都是 city），直接照搬会把"福田区"显示成"市"。后缀判定只对
// 行政对象生效，因此不会把"XX 开发区"这类 POI 误判成区县。
func nominatimKind(category, typ, addresstype, name string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	addresstype = strings.ToLower(strings.TrimSpace(addresstype))

	if administrative := func() bool {
		switch typ {
		case "administrative", "boundary":
			return true
		}
		switch addresstype {
		case "state", "province", "region", "county", "city", "municipality",
			"district", "suburb", "borough", "town", "village", "hamlet":
			return true
		}
		return false
	}(); administrative {
		if kind := chineseAdminKind(name); kind != "" {
			return kind
		}
	}

	// Nominatim 把行政区归到 class=boundary/place，具体层级在 type 上。
	switch typ {
	case "country":
		return "country"
	case "state", "province", "region":
		return "province"
	case "city", "county", "municipality":
		return "city"
	case "district", "borough":
		return "district"
	case "suburb":
		return "street"
	case "town", "village", "hamlet":
		return "town"
	case "administrative":
		switch addresstype {
		case "state":
			return "province"
		case "county", "city":
			return "city"
		case "district":
			return "district"
		case "suburb":
			return "street"
		case "town", "village":
			return "town"
		}
		return "district"
	}
	if strings.EqualFold(category, "highway") {
		return "street"
	}
	return "poi"
}

// chineseAdminKind 按中文行政区名后缀判定层级。判不出返回空串。
func chineseAdminKind(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	switch {
	case strings.HasSuffix(name, "特别行政区"),
		strings.HasSuffix(name, "自治区"),
		strings.HasSuffix(name, "省"):
		return "province"
	case strings.HasSuffix(name, "自治州"),
		strings.HasSuffix(name, "地区"),
		strings.HasSuffix(name, "盟"),
		strings.HasSuffix(name, "市"):
		return "city"
	case strings.HasSuffix(name, "自治县"),
		strings.HasSuffix(name, "县"),
		strings.HasSuffix(name, "旗"),
		strings.HasSuffix(name, "区"):
		return "district"
	case strings.HasSuffix(name, "街道"):
		return "street"
	case strings.HasSuffix(name, "镇"),
		strings.HasSuffix(name, "乡"):
		return "town"
	}
	return ""
}

func isAdministrativeKind(kind string) bool {
	switch kind {
	case "country", "province", "city", "district", "town":
		return true
	default:
		return false
	}
}

func joinAddressParts(parts ...string) string {
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return strings.Join(out, " · ")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// ===================== 行政区边界 =====================

type nominatimBoundaryRow struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	Lat         string          `json:"lat"`
	Lon         string          `json:"lon"`
	Class       string          `json:"class"`
	Category    string          `json:"category"`
	Type        string          `json:"type"`
	Addresstype string          `json:"addresstype"`
	BoundingBox []string        `json:"boundingbox"`
	GeoJSON     json.RawMessage `json:"geojson"`
}

// osmCategory 取 OSM 的一级分类。
//
// ⚠️ Nominatim 的 `format=jsonv2` 把 `class` 改名成了 `category`，只有
// `format=json` 才叫 `class`。我们用的是 jsonv2，所以 `Class` 一直是空串 ——
// 之前依赖 `class=boundary` 的排序其实从未生效过。两个字段都读，谁有值用谁。
func (row nominatimBoundaryRow) osmCategory() string {
	if category := strings.TrimSpace(row.Category); category != "" {
		return category
	}
	return strings.TrimSpace(row.Class)
}

// looksLikeAdministrative 判断这条结果是否属于行政层级。
// 只有行政对象才用名字后缀去纠层级 —— POI（"XX 园区""XX 开发区"）不算。
func (row nominatimBoundaryRow) looksLikeAdministrative() bool {
	switch strings.ToLower(strings.TrimSpace(row.Type)) {
	case "administrative", "boundary":
		return true
	}
	switch strings.ToLower(strings.TrimSpace(row.Addresstype)) {
	case "state", "province", "region", "county", "city", "municipality",
		"district", "suburb", "borough", "town", "village", "hamlet":
		return true
	}
	return false
}

// LookupRegionBoundary 按地区名称取面状边界，返回可直接入库的 GeoJSON。
//
// 只接受 Polygon / MultiPolygon 结果：搜到的是 POI（学校、商场）时没有面，
// 明确报错而不是拿一个点当成围栏让用户困惑。
func (s *Server) LookupRegionBoundary(query string) (*RegionBoundary, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, &geoUserError{Code: 400, Reason: "empty_query", Detail: "region name is required"}
	}
	key := "region:" + strings.ToLower(query)
	if cached, ok := geoCacheGet(key); ok {
		if boundary, okCast := cached.(*RegionBoundary); okCast {
			return boundary, nil
		}
	}

	params := url.Values{}
	params.Set("q", query)
	params.Set("format", "jsonv2")
	params.Set("polygon_geojson", "1")
	params.Set("limit", "6")
	params.Set("accept-language", "zh-CN,zh")
	var rows []nominatimBoundaryRow
	if err := geoGetJSON(nominatimSearchEndpoint+"?"+params.Encode(), &rows, waitNominatimSlot); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrGeoNoMatch, query)
	}

	// 优先 class=boundary 的面：同一名称可能同时命中"西湖"景点与"西湖区"。
	sort.SliceStable(rows, func(i, j int) bool {
		return boundaryRank(rows[i]) > boundaryRank(rows[j])
	})

	var chosen *nominatimBoundaryRow
	for i := range rows {
		if geometryTypeOf(rows[i].GeoJSON) == "Polygon" || geometryTypeOf(rows[i].GeoJSON) == "MultiPolygon" {
			chosen = &rows[i]
			break
		}
	}
	if chosen == nil {
		return nil, fmt.Errorf("%w: %s", ErrGeoNoBoundary, query)
	}

	shape, err := ParseFenceShape(string(chosen.GeoJSON))
	if err != nil {
		return nil, fmt.Errorf("boundary geometry is unusable: %w", err)
	}
	shape, dropped := trimShape(shape)
	simplified := simplifyShape(shape, regionRingMaxPoints)

	if s != nil && s.Logger != nil && dropped > 0 {
		s.Logger.Info("Region boundary trimmed",
			zap.String("query", query), zap.Int("dropped_polygons", dropped))
	}

	geometry, points, polygons, err := shapeToGeoJSON(simplified)
	if err != nil {
		return nil, err
	}
	minLng, minLat, maxLng, maxLat := simplified.BBox()

	boundary := &RegionBoundary{
		Name:        firstNonEmpty(chosen.Name, chosen.DisplayName, query),
		DisplayName: firstNonEmpty(chosen.DisplayName, chosen.Name, query),
		Level:       nominatimKind(chosen.osmCategory(), chosen.Type, chosen.Addresstype, chosen.Name),
		Geometry:    geometry,
		BBox:        [4]float64{minLng, minLat, maxLng, maxLat},
		Points:      points,
		Polygons:    polygons,
		Source:      "osm",
		Simplified:  points < shapePointCount(shape),
	}
	geoCachePut(key, boundary, regionCacheTTL)
	return boundary, nil
}

func boundaryRank(row nominatimBoundaryRow) int {
	score := 0
	// 行政边界优先于同名 POI：搜"西湖"会同时命中景点与西湖区。
	if strings.EqualFold(row.osmCategory(), "boundary") {
		score += 10
	}
	switch nominatimKind(row.osmCategory(), row.Type, row.Addresstype, row.Name) {
	case "country":
		score += 5
	case "province":
		score += 4
	case "city":
		score += 3
	case "district":
		score += 2
	case "town":
		score += 1
	}
	return score
}

func geometryTypeOf(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return ""
	}
	return probe.Type
}

// ===================== 几何裁剪与简化 =====================

// trimShape 丢弃过小的碎块（远洋小岛），并把区块数压到上限内。
func trimShape(shape *FenceShape) (*FenceShape, int) {
	if shape == nil || len(shape.Polygons) == 0 {
		return shape, 0
	}
	type item struct {
		poly FencePolygon
		area float64
	}
	items := make([]item, 0, len(shape.Polygons))
	for _, poly := range shape.Polygons {
		minLng, minLat, maxLng, maxLat := RingBBox(poly.Outer)
		items = append(items, item{poly: poly, area: math.Abs((maxLng - minLng) * (maxLat - minLat))})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].area > items[j].area })

	maxArea := items[0].area
	kept := make([]FencePolygon, 0, len(items))
	for index, entry := range items {
		if entry.area <= 0 {
			continue
		}
		if maxArea > 0 && entry.area < maxArea*regionMinAreaRatio {
			continue
		}
		if index >= regionMaxPolygons {
			continue
		}
		kept = append(kept, entry.poly)
	}
	if len(kept) == 0 {
		return shape, 0
	}
	trimmed, err := rebuildShape(kept)
	if err != nil {
		return shape, 0
	}
	return trimmed, len(shape.Polygons) - len(kept)
}

// rebuildShape 用保留的多边形重建 FenceShape（重算包围盒与简化环）。
func rebuildShape(polygons []FencePolygon) (*FenceShape, error) {
	raw := make([]any, 0, len(polygons))
	for _, poly := range polygons {
		rings := make([]any, 0, len(poly.Holes)+1)
		rings = append(rings, ringToAny(poly.Outer))
		for _, hole := range poly.Holes {
			rings = append(rings, ringToAny(hole))
		}
		raw = append(raw, rings)
	}
	final := raw
	if len(polygons) == 1 {
		final = raw[0].([]any)
	}
	typeName := "MultiPolygon"
	if len(polygons) == 1 {
		typeName = "Polygon"
	}
	payload, err := json.Marshal(map[string]any{"type": typeName, "coordinates": final})
	if err != nil {
		return nil, err
	}
	return ParseFenceShape(string(payload))
}

func ringToAny(ring GeoJSONRing) []any {
	out := make([]any, 0, len(ring))
	for _, point := range ring {
		out = append(out, []float64{point[0], point[1]})
	}
	return out
}

// simplifyShape 用 Douglas-Peucker 把每个环压到 maxPoints 以内。
func simplifyShape(shape *FenceShape, maxPoints int) *FenceShape {
	if shape == nil {
		return nil
	}
	out := &FenceShape{Polygons: make([]FencePolygon, 0, len(shape.Polygons))}
	for _, poly := range shape.Polygons {
		entry := FencePolygon{Outer: simplifyRing(poly.Outer, maxPoints)}
		for _, hole := range poly.Holes {
			simplifiedHole := simplifyRing(hole, maxPoints)
			if len(simplifiedHole) >= 4 {
				entry.Holes = append(entry.Holes, simplifiedHole)
			}
		}
		out.Polygons = append(out.Polygons, entry)
	}
	rebuilt, err := rebuildShape(out.Polygons)
	if err != nil {
		return shape
	}
	return rebuilt
}

// simplifyRing 以环的包围盒对角线为尺度，逐步放大容差直到顶点数达标。
func simplifyRing(ring GeoJSONRing, maxPoints int) GeoJSONRing {
	if len(ring) <= maxPoints || maxPoints < 8 {
		return ring
	}
	minLng, minLat, maxLng, maxLat := RingBBox(ring)
	diagonal := math.Hypot(maxLng-minLng, maxLat-minLat)
	if diagonal <= 0 {
		return ring
	}
	tolerance := diagonal / 4096
	for attempt := 0; attempt < 24; attempt += 1 {
		candidate := douglasPeucker(ring, tolerance)
		if len(candidate) <= maxPoints {
			return candidate
		}
		tolerance *= 1.8
	}
	return douglasPeucker(ring, diagonal/16)
}

// douglasPeucker 对闭合环做迭代式抽稀（避免深递归爆栈）。
func douglasPeucker(ring GeoJSONRing, tolerance float64) GeoJSONRing {
	if len(ring) <= 3 {
		return ring
	}
	// GeoJSON 环首尾相同；抽稀时按"开口折线"处理，最后补回闭合点。
	points := ring
	closed := points[0][0] == points[len(points)-1][0] && points[0][1] == points[len(points)-1][1]
	if closed && len(points) > 1 {
		points = points[:len(points)-1]
	}

	keep := make([]bool, len(points))
	keep[0] = true
	keep[len(points)-1] = true

	type span struct{ first, last int }
	stack := []span{{0, len(points) - 1}}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.last-current.first < 2 {
			continue
		}
		maxDistance := -1.0
		maxIndex := -1
		ax, ay := points[current.first][0], points[current.first][1]
		bx, by := points[current.last][0], points[current.last][1]
		for i := current.first + 1; i < current.last; i += 1 {
			distance := perpendicularDistance(points[i][0], points[i][1], ax, ay, bx, by)
			if distance > maxDistance {
				maxDistance = distance
				maxIndex = i
			}
		}
		if maxDistance > tolerance && maxIndex > 0 {
			keep[maxIndex] = true
			stack = append(stack, span{current.first, maxIndex}, span{maxIndex, current.last})
		}
	}

	out := make(GeoJSONRing, 0, len(points))
	for i, point := range points {
		if keep[i] {
			out = append(out, point)
		}
	}
	if len(out) < 3 {
		out = points
	}
	if closed {
		out = append(out, out[0])
	}
	return out
}

func perpendicularDistance(px, py, ax, ay, bx, by float64) float64 {
	dx := bx - ax
	dy := by - ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	return math.Abs(dy*px-dx*py+bx*ay-by*ax) / math.Hypot(dx, dy)
}

// shapeToGeoJSON 把判定视图序列化回 GeoJSON，并返回顶点数与区块数。
func shapeToGeoJSON(shape *FenceShape) (string, int, int, error) {
	if shape == nil || len(shape.Polygons) == 0 {
		return "", 0, 0, fmt.Errorf("empty shape")
	}
	points := 0
	for _, poly := range shape.Polygons {
		points += len(poly.Outer)
		for _, hole := range poly.Holes {
			points += len(hole)
		}
	}
	raw := make([]any, 0, len(shape.Polygons))
	for _, poly := range shape.Polygons {
		rings := make([]any, 0, len(poly.Holes)+1)
		rings = append(rings, ringToAny(poly.Outer))
		for _, hole := range poly.Holes {
			rings = append(rings, ringToAny(hole))
		}
		raw = append(raw, rings)
	}
	payload := map[string]any{"type": "MultiPolygon", "coordinates": raw}
	if len(shape.Polygons) == 1 {
		payload = map[string]any{"type": "Polygon", "coordinates": raw[0]}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", 0, 0, err
	}
	return string(encoded), points, len(shape.Polygons), nil
}

func shapePointCount(shape *FenceShape) int {
	if shape == nil {
		return 0
	}
	total := 0
	for _, poly := range shape.Polygons {
		total += len(poly.Outer)
		for _, hole := range poly.Holes {
			total += len(hole)
		}
	}
	return total
}
