package core

// 无设备时的默认视野 —— 按公网出口 IP 做城市级粗定位。
//
// 为什么放在服务端而不是浏览器直连第三方 IP 库：
//  1. 桌面 / 网关形态下浏览器与服务平台同机，前端根本拿不到「我的公网出口 IP」，
//     只有服务端能去问「我是谁」；反过来，平台远程部署时 `GetClientIp()`
//     拿到的是真实用户 IP，比服务端自己的出口更准 —— 两种形态只有服务端能兼顾。
//  2. 第三方 IP 库在大陆的可达性很不稳定（实测同一天内 ipwho.is / ip.sb
//     全部超时）。跨源重试 + 结果缓存必须收敛在服务端，不能让每个浏览器各试一遍。
//  3. 客户端 IP 属于个人数据：不进 localStorage、不回显到界面，只用来算坐标。
//
// 三条数据源同时问，任一成功即返回（见 ResolveApproximateLocation 里的并发说明）：
//  1. ip-api.com    —— 明文 HTTP，一次往返直接给经纬度（免费额度 45 次/分钟）；
//  2. ipwhois.app   —— HTTPS，一次往返给经纬度；
//  3. myip.ipip.net —— 国内服务，只给中文城市名，再用 Photon 反查坐标（两跳）。
//
// 三者都是免 Key 公共服务。IP 定位本来就只有城市级精度（数公里到数十公里），
// 所以失败没有任何损失 —— 调用方退回中国全境视野即可。

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"go.uber.org/zap"
)

// GeoIPLocation 是 IP 粗定位结果。坐标一律 WGS-84，与全站坐标真源一致。
//
// 刻意*不*包含国家 / 省 / 城市 / ISP 文本：第三方库对地名的表述五花八门且不可控，
// 一旦渲染到界面上就会绕过项目的 i18n 与地名口径；界面只需要一个中心点。
type GeoIPLocation struct {
	Longitude float64 `json:"longitude"`
	Latitude  float64 `json:"latitude"`
	// Source 只用于排障（哪个数据源给的），界面不显示。
	Source string `json:"source"`
}

const (
	// 单源超时。IP 定位是「锦上添花」，用户不该为它等超过 3 秒。
	ipGeoProviderTimeout = 3 * time.Second
	// 整条链的总预算。三源并发后它就是「最慢的一条」的上限（含 ipip 的两跳），
	// 用完就认输，让前端停在已经铺好的中国全境视野上。
	ipGeoTotalBudget = 6 * time.Second
	// 结果缓存：同一个出口 IP 的位置一天内基本不变。
	ipGeoCacheTTL = 12 * time.Hour
	// 缓存条目上限。按 IP 分布，正常部署远达不到。
	ipGeoCacheMaxSize = 512
)

var (
	ipGeoMu     sync.Mutex
	ipGeoCache  = map[string]ipGeoEntry{}
	ipGeoClient = &http.Client{
		Timeout: ipGeoProviderTimeout,
		// 与 geoHTTPClient 同样的理由：企业准入 / 零信任客户端会静默掐断空闲连接，
		// 复用连接的下一个请求直接拿到 EOF。这里请求量极小，不复用毫无成本。
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DisableKeepAlives:   true,
			ForceAttemptHTTP2:   true,
			MaxIdleConnsPerHost: 1,
		},
	}
)

type ipGeoEntry struct {
	location *GeoIPLocation
	expire   time.Time
}

// ===================== 数据源 =====================

// ipGeoProvider 描述一条数据源。fetch 拿到 targetIP（空串表示「按服务端自己的出口来定位」）。
type ipGeoProvider struct {
	name  string
	fetch func(s *Server, targetIP string) (*GeoIPLocation, error)
}

var ipGeoProviders = []ipGeoProvider{
	{name: "ip-api", fetch: fetchGeoIPFromIPApi},
	{name: "ipwhois", fetch: fetchGeoIPFromIPWhois},
	{name: "ipip", fetch: fetchGeoIPFromIPIP},
}

// fetchGeoIPFromIPApi 走 ip-api.com。
//
// 免费额度 45 次/分钟、只支持明文 HTTP，因此结果缓存是必须的（见 ResolveApproximateLocation）。
func fetchGeoIPFromIPApi(_ *Server, targetIP string) (*GeoIPLocation, error) {
	endpoint := "http://ip-api.com/json/"
	if targetIP != "" {
		endpoint += url.PathEscape(targetIP)
	}
	params := url.Values{}
	params.Set("fields", "status,message,lat,lon")
	endpoint += "?" + params.Encode()

	var payload struct {
		Status  string   `json:"status"`
		Message string   `json:"message"`
		Lat     looseNum `json:"lat"`
		Lon     looseNum `json:"lon"`
	}
	if err := ipGeoGetJSON(endpoint, &payload); err != nil {
		return nil, err
	}
	if payload.Status != "success" {
		return nil, fmt.Errorf("ip-api reported %q: %s", payload.Status, payload.Message)
	}
	return newGeoIPLocation(payload.Lon.value(), payload.Lat.value(), "ip-api")
}

// fetchGeoIPFromIPWhois 走 ipwhois.app。
func fetchGeoIPFromIPWhois(_ *Server, targetIP string) (*GeoIPLocation, error) {
	endpoint := "https://ipwhois.app/json/"
	if targetIP != "" {
		endpoint += url.PathEscape(targetIP)
	}

	var payload struct {
		Success   bool     `json:"success"`
		Message   string   `json:"message"`
		Latitude  looseNum `json:"latitude"`
		Longitude looseNum `json:"longitude"`
	}
	if err := ipGeoGetJSON(endpoint, &payload); err != nil {
		return nil, err
	}
	if !payload.Success {
		return nil, fmt.Errorf("ipwhois responded unsuccessfully: %s", payload.Message)
	}
	return newGeoIPLocation(payload.Longitude.value(), payload.Latitude.value(), "ipwhois")
}

// fetchGeoIPFromIPIP 走 myip.ipip.net：只要得到城市名，再借 Photon 换算坐标。
//
// 它是大陆网络下最稳的一条，但多一次往返，所以排在最后 —— 前两条都通了就不必付这个成本。
func fetchGeoIPFromIPIP(s *Server, targetIP string) (*GeoIPLocation, error) {
	endpoint := "https://myip.ipip.net"
	if targetIP != "" {
		endpoint += "/" + url.PathEscape(targetIP)
	}
	body, err := ipGeoGetText(endpoint)
	if err != nil {
		return nil, err
	}

	place := parseIPIPRegion(body)
	if place == "" {
		return nil, fmt.Errorf("ipip returned an unparsable response")
	}
	if s == nil {
		return nil, fmt.Errorf("server is not ready for geocoding")
	}
	// 用「省 / 市 / 区」里最具体的一级去查，Photon 对中文行政区名的召回足够。
	places, err := s.SearchPlaces(place, 1)
	if err != nil || len(places) == 0 {
		return nil, fmt.Errorf("geocoding %q failed: %w", place, err)
	}
	return newGeoIPLocation(places[0].Lng, places[0].Lat, "ipip")
}

// parseIPIPRegion 从 `当前 IP：x.x.x.x  来自于：中国 广东 深圳  电信` 里取出最具体的地名。
//
// 格式不固定（省级结果只有两段、境外结果可能带着洲名），因此按「去掉末尾 ISP」
// 的规则取，而不是按固定下标取。
func parseIPIPRegion(body string) string {
	const marker = "来自于："
	index := strings.Index(body, marker)
	if index < 0 {
		// 少数情况下分隔符是全角冒号的变体，退一步按 "来自于" 之后找。
		markerAlt := "来自于"
		index = strings.Index(body, markerAlt)
		if index < 0 {
			return ""
		}
		body = body[index+len(markerAlt):]
	} else {
		body = body[index+len(marker):]
	}

	fields := strings.Fields(body)
	if len(fields) < 2 {
		return ""
	}
	// 末尾是运营商（电信 / 联通 / 移动 / 阿里云 …），不是地名。
	candidates := fields[:len(fields)-1]
	return strings.TrimSpace(candidates[len(candidates)-1])
}

// ===================== 解析 & 取数 =====================

// looseNum 兼容第三方把同一字段写成数字或字符串（"25.05" / 25.05）的情况。
type looseNum struct {
	raw   json.RawMessage
	valid bool
}

func (n *looseNum) UnmarshalJSON(data []byte) error {
	n.raw = append(n.raw[:0], data...)
	n.valid = true
	return nil
}

func (n looseNum) value() float64 {
	if !n.valid {
		return 0
	}
	text := strings.Trim(strings.TrimSpace(string(n.raw)), `"`)
	if text == "" || text == "null" {
		return 0
	}
	number, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return 0
	}
	return number
}

// newGeoIPLocation 做一次落库前的合法性校验。
//
// (0,0) 与越界值一律当失败：IP 库对外海 / 无数据 IP 经常返回 0，照单全收会把
// 视野拉到几内亚湾 —— 与设备坐标那条链路的判定保持同一套规则。
func newGeoIPLocation(longitude, latitude float64, source string) (*GeoIPLocation, error) {
	if !isValidGeoCoordinate(longitude, latitude) {
		return nil, fmt.Errorf("provider returned an unusable coordinate (%v, %v)", longitude, latitude)
	}
	return &GeoIPLocation{Longitude: longitude, Latitude: latitude, Source: source}, nil
}

func isValidGeoCoordinate(longitude, latitude float64) bool {
	if math.IsNaN(longitude) || math.IsNaN(latitude) || math.IsInf(longitude, 0) || math.IsInf(latitude, 0) {
		return false
	}
	if longitude < -180 || longitude > 180 || latitude < -90 || latitude > 90 {
		return false
	}
	return !(math.Abs(longitude) < 1e-6 && math.Abs(latitude) < 1e-6)
}

func ipGeoGetJSON(endpoint string, out any) error {
	body, err := ipGeoGet(endpoint, "application/json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("upstream returned invalid json: %w", err)
	}
	return nil
}

func ipGeoGetText(endpoint string) (string, error) {
	body, err := ipGeoGet(endpoint, "text/plain")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func ipGeoGet(endpoint, accept string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", geoUserAgent)
	request.Header.Set("Accept", accept)
	// 部分 IP 库按 Accept-Language 决定城市名的语言。
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.5")

	response, err := ipGeoClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 256<<10))
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		return nil, &geoStatusError{code: response.StatusCode}
	}
	return body, nil
}

// ===================== 对外能力 =====================

// ResolveApproximateLocation 返回某个客户端 IP 的城市级粗定位。
//
// clientIP 为私网 / 环回 / 非法地址时自动降级成「按服务端自己的出口 IP 定位」——
// 桌面与网关形态下浏览器就长在同一台机器上，客户端地址对第三方毫无意义。
//
// 全链路失败返回 ErrGeoUnavailable，调用方据此退回中国全境视野；这里不制造
// 「宁可用一个瞎猜的坐标」的兜底，错的位置比没有位置更糟。
func (s *Server) ResolveApproximateLocation(clientIP string) (*GeoIPLocation, error) {
	targetIP := selectGeoIPTarget(clientIP)
	cacheKey := targetIP
	if cacheKey == "" {
		cacheKey = "self"
	}

	if location, ok := ipGeoCacheGet(cacheKey); ok {
		return location, nil
	}

	// 三条数据源**并发**跑，谁先给出可用坐标就用谁。
	//
	// 串行尝试的最坏耗时是三个超时之和（9 秒以上），而底图在等它的时候已经先
	// 铺好中国全境视野 —— 用户会看到"过一会儿地图自己跳走了"，且跳之前的地图
	// 看起来完全正常，这种延迟跳变比不定跳更糟。并发把最坏耗时压到「最慢的一条」，
	// 代价只是缓存缺失时多发两个请求（结果缓存 12 小时，正常一天也就几条）。
	//
	// 第三方 IP 库在大陆的可达性确实会整片失败（实测同一天内多个源全部超时），
	// 所以「任一成功即可」比「按优先级串行」更符合这里的收益结构。
	results := make(chan ipGeoAttempt, len(ipGeoProviders))
	for _, provider := range ipGeoProviders {
		go func(provider ipGeoProvider) {
			location, err := provider.fetch(s, targetIP)
			if err != nil && s != nil && s.Logger != nil {
				// 详情只进日志：这里可能带着上游地址与底层网络错误。
				s.Logger.Warn("IP location provider failed",
					zap.String("provider", provider.name), zap.Error(err))
			}
			results <- ipGeoAttempt{provider: provider.name, location: location, err: err}
		}(provider)
	}

	deadline := time.After(ipGeoTotalBudget)
	failures := make([]string, 0, len(ipGeoProviders))
	for attempt := 0; attempt < len(ipGeoProviders); attempt++ {
		select {
		case result := <-results:
			if result.err == nil && result.location != nil {
				ipGeoCachePut(cacheKey, result.location)
				if s != nil && s.Logger != nil {
					s.Logger.Debug("Resolved approximate location by IP",
						zap.String("source", result.location.Source),
						zap.Float64("longitude", result.location.Longitude),
						zap.Float64("latitude", result.location.Latitude))
				}
				return result.location, nil
			}
			failures = append(failures, fmt.Sprintf("%s: %v", result.provider, result.err))
		case <-deadline:
			// 预算用完就认输，不再等剩下的源：调用方此刻已经停在中国全境视野上。
			failures = append(failures, "budget exhausted")
			attempt = len(ipGeoProviders)
		}
	}

	if len(failures) == 0 {
		failures = append(failures, "no provider answered")
	}
	return nil, fmt.Errorf("%w: %s", ErrGeoUnavailable, strings.Join(failures, "; "))
}

type ipGeoAttempt struct {
	provider string
	location *GeoIPLocation
	err      error
}

func ipGeoCacheGet(key string) (*GeoIPLocation, bool) {
	ipGeoMu.Lock()
	defer ipGeoMu.Unlock()
	entry, ok := ipGeoCache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expire) {
		delete(ipGeoCache, key)
		return nil, false
	}
	return entry.location, true
}

func ipGeoCachePut(key string, location *GeoIPLocation) {
	ipGeoMu.Lock()
	defer ipGeoMu.Unlock()
	if len(ipGeoCache) >= ipGeoCacheMaxSize {
		now := time.Now()
		for existingKey, entry := range ipGeoCache {
			if now.After(entry.expire) {
				delete(ipGeoCache, existingKey)
			}
		}
		if len(ipGeoCache) >= ipGeoCacheMaxSize {
			ipGeoCache = map[string]ipGeoEntry{}
		}
	}
	ipGeoCache[key] = ipGeoEntry{location: location, expire: time.Now().Add(ipGeoCacheTTL)}
}

// selectGeoIPTarget 判定这个客户端地址值不值得拿去问第三方。
//
// 私网 / 环回 / 链路本地 / CGNAT / TUN 虚拟网段（198.18.0.0/15）都返回空串，
// 表示「改为定位服务端自己的出口 IP」。本机实测出口走 TUN 的 198.18.0.1，
// 而 `GetClientIp()` 在这种部署下拿到的是 ::1 —— 不加这层判定就永远定不准。
func selectGeoIPTarget(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return ""
	}
	if v4 := ip.To4(); v4 != nil {
		// CGNAT 100.64.0.0/10：运营商 / 企业出口常见，第三方查不到东西。
		if v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
			return ""
		}
		// 198.18.0.0/15：基准测试保留段，被多数代理 / TUN 虚拟网卡占用。
		if v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
			return ""
		}
		return v4.String()
	}
	return ip.String()
}

// handleIPLocation 提供「无设备时该看哪里」的默认视野。
func (s *Server) handleIPLocation(r *ghttp.Request) {
	location, err := s.ResolveApproximateLocation(r.GetClientIp())
	if err != nil {
		s.writeGeoError(r, "ip-location", "", err)
		return
	}
	r.Response.WriteJson(g.Map{"code": 0, "data": location})
}
