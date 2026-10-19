/**
 * 设备地图的数据访问与展示规则。
 *
 * 这里是「地图怎么画」的唯一事实来源：状态色、离线判定、围栏违规判定都在此收口，
 * 组件只负责渲染，不自行推断语义。
 */
import axios from 'axios'

// ===================== 设备点状态色 =====================
//
// 与视觉稿（Ardot file/737044812304464）第 ①② 板定型一致，取自 style.css 的语义色。
// offline 用**虚线描边**表达"非实时"，而不是让设备点消失 —— 设备看不见
// 比设备置灰更容易被误读为"数据丢了"。
export const DEVICE_POINT_STATUS = {
  online: { token: '--color-success', key: 'device_map_status_online' },
  warning: { token: '--color-warning', key: 'device_map_status_warning' },
  alarm: { token: '--color-danger', key: 'device_map_status_alarm' },
  offline: { token: '--map-device-offline', key: 'device_map_status_offline' },
}

export const DEVICE_STATUS_ORDER = ['online', 'warning', 'alarm', 'offline']

/** 位置超过该时长即视为"不新鲜"，描边转为虚线段以示区分。 */
export const STALE_LOCATION_MS = 10 * 60 * 1000

export function mapTokenColor(token) {
  const root = document.querySelector('[data-bs-theme]') || document.documentElement
  return getComputedStyle(root).getPropertyValue(token).trim()
}

export function deviceStatusColor(status) {
  const entry = DEVICE_POINT_STATUS[status] || DEVICE_POINT_STATUS.offline
  return mapTokenColor(entry.token)
}

export function deviceStatusLabelKey(status) {
  const entry = DEVICE_POINT_STATUS[status] || DEVICE_POINT_STATUS.offline
  return entry.key
}

/**
 * 计算设备在地图上的展示状态。
 *
 * 优先级：离线 > 违规告警 > 围栏预警 > 在线。
 * 离线优先，因为一个不上报的设备的围栏判定结果本身不可信。
 */
export function resolveDeviceStatus(device, fenceStates, fences) {
  if (!device || !device.online) return 'offline'

  const severities = collectFenceSeverities(device.device_code, fenceStates, fences)
  if (severities.includes('alarm')) return 'alarm'
  if (severities.includes('warning')) return 'warning'
  return 'online'
}

/** 收集某设备在所有围栏上的严重度。 */
export function collectFenceSeverities(deviceCode, fenceStates, fences) {
  if (!deviceCode || !Array.isArray(fenceStates) || !Array.isArray(fences)) return []

  const byId = new Map()
  fences.forEach((fence) => byId.set(Number(fence.ID ?? fence.id), fence))

  const out = []
  fenceStates.forEach((state) => {
    if (state.device_code !== deviceCode) return
    const fence = byId.get(Number(state.fence_id))
    if (!fence) return
    const severity = fenceSeverity(fence, state)
    if (severity) out.push(severity)
  })
  return out
}

/**
 * 一个设备的某条围栏状态对应的严重度。
 *
 * - `forbid`（禁入）：在内部 ⇒ 违规
 * - `keep_in`（必须在内）：在外部 ⇒ 违规
 * - `notify`（通知）：在内部 ⇒ 提示
 */
export function fenceSeverity(fence, state) {
  if (!fence || !state) return null
  const ruleType = fence.rule_type || fence.ruleType
  const inside = state.state === 'inside'
  const outside = state.state === 'outside'
  if (!inside && !outside) return null

  if (ruleType === 'forbid' && inside) return 'alarm'
  if (ruleType === 'keep_in' && outside) return 'alarm'
  if (ruleType === 'notify' && inside) return 'warning'
  return null
}

/** 位置是否陈旧（超过阈值未更新）。 */
export function isStaleLocation(fixAt, now = Date.now()) {
  if (!fixAt) return true
  return now - Number(fixAt) > STALE_LOCATION_MS
}

// ===================== 数据访问 =====================

const API = '/api/device-map'

function unwrap(response) {
  if (response?.data?.code !== 0) {
    const message = response?.data?.message || 'Request failed'
    const error = new Error(message)
    error.code = response?.data?.code
    // reason 是后端给机器读的原因码（unreachable / no_match / no_boundary …）。
    // 界面按它选本地化文案，绝不直接把后端 message 甩给用户 ——
    // 之前就把 `Get "https://nominatim…": EOF` 原样展示过。
    error.reason = response?.data?.data?.reason || ''
    throw error
  }
  return response.data.data
}

export async function fetchMapDevices(params = {}) {
  const data = await unwrap(await axios.get(`${API}/devices`, { params }))
  return {
    devices: data?.devices || [],
    total: data?.total || 0,
    locatedTotal: data?.located_total || 0,
    unlocatedTotal: data?.unlocated_total || 0,
  }
}

/**
 * 「浏览器所在网络的公网 IP」粗定位（城市级）。
 *
 * 只用于「一台有位置的设备都没有」时的默认视野，因此**失败一律返回 null**，
 * 不弹提示、不重试：拿不到就退到中国全境，这是已约定的降级路径，
 * 让它变成一个错误提示只会打扰人。
 *
 * 结果按会话缓存 30 分钟：IP 粗定位在城市级，短时间内不会变。有缓存时首帧就能
 * 直接落到城市上，不会先铺一次中国全境再收窄 —— 两次跳变是可见的体验缺陷。
 */
const IP_LOCATION_STORAGE_KEY = 'noyo.map.ipLocation'
const IP_LOCATION_STORAGE_TTL = 30 * 60 * 1000

/** 坐标可用性判定：(0,0) 与越界值一律视为无数据。 */
function isUsableLngLat(longitude, latitude) {
  if (!Number.isFinite(longitude) || !Number.isFinite(latitude)) return false
  if (Math.abs(longitude) > 180 || Math.abs(latitude) > 90) return false
  return !(Math.abs(longitude) < 1e-6 && Math.abs(latitude) < 1e-6)
}

/** 同步读缓存的粗定位结果（首帧直接可用）。没有 / 过期 / 损坏都返回 null。 */
export function readIpLocationCache() {
  let parsed
  try {
    parsed = JSON.parse(sessionStorage.getItem(IP_LOCATION_STORAGE_KEY) || 'null')
  } catch (error) {
    return null
  }
  if (!parsed) return null
  const age = Date.now() - Number(parsed.at || 0)
  if (!(age >= 0 && age < IP_LOCATION_STORAGE_TTL)) return null
  const longitude = Number(parsed.longitude)
  const latitude = Number(parsed.latitude)
  return isUsableLngLat(longitude, latitude) ? { longitude, latitude } : null
}

export async function fetchIpLocation() {
  const cached = readIpLocationCache()
  if (cached) return cached

  let data
  try {
    data = unwrap(await axios.get(`${API}/ip-location`, { timeout: 9000 }))
  } catch (error) {
    return null
  }

  const longitude = Number(data?.longitude)
  const latitude = Number(data?.latitude)
  // 后端已经校验过，这里再挡一次：脏坐标混进来会把视野硬拉到几内亚湾。
  if (!isUsableLngLat(longitude, latitude)) return null

  try {
    sessionStorage.setItem(IP_LOCATION_STORAGE_KEY, JSON.stringify({ longitude, latitude, at: Date.now() }))
  } catch (error) {
    /* 隐私模式下写不进去：只是少了缓存，不影响本次结果 */
  }
  return { longitude, latitude }
}

export async function fetchDeviceLocation(deviceCode) {
  return unwrap(await axios.get(`${API}/devices/${encodeURIComponent(deviceCode)}`))
}

export async function saveDeviceLocation(deviceCode, longitude, latitude) {
  return unwrap(
    await axios.put(`${API}/devices/${encodeURIComponent(deviceCode)}/location`, {
      longitude,
      latitude,
    }),
  )
}

export async function removeDeviceLocation(deviceCode) {
  return unwrap(await axios.delete(`${API}/devices/${encodeURIComponent(deviceCode)}/location`))
}

export async function fetchGeofences() {
  const data = await unwrap(await axios.get(`${API}/fences`))
  return {
    fences: data?.fences || [],
    states: data?.states || [],
  }
}

export async function createGeofence(payload) {
  return unwrap(await axios.post(`${API}/fences`, payload))
}

export async function updateGeofence(id, payload) {
  return unwrap(await axios.put(`${API}/fences/${id}`, payload))
}

export async function deleteGeofence(id) {
  return unwrap(await axios.delete(`${API}/fences/${id}`))
}

/** 整体替换某围栏绑定的设备（空数组 = 不绑定，对全部设备生效）。 */
export async function setFenceDevices(id, deviceCodes) {
  return unwrap(await axios.put(`${API}/fences/${id}/devices`, { device_codes: deviceCodes || [] }))
}

/**
 * 地点搜索。
 *
 * 走本端后端而不是直连公共地理编码服务：一来避免浏览器 CORS 与跨域限制，
 * 二来服务商的限速与 User-Agent 要求只能在服务端统一遵守。
 */
export async function searchPlaces(query, limit = 8) {
  const data = await unwrap(await axios.get(`${API}/geocode`, { params: { q: query, limit } }))
  return data?.places || []
}

/** 按地区名称取行政区边界（省/市/区县/街道），返回可直接入库的 GeoJSON。 */
export async function lookupRegionBoundary(query) {
  return unwrap(await axios.get(`${API}/region`, { params: { q: query } }))
}

export async function fetchFenceStates(deviceCode = '') {
  const data = await unwrap(
    await axios.get(`${API}/fences/states`, { params: deviceCode ? { device_code: deviceCode } : {} }),
  )
  return data || []
}

// ===================== GeoJSON 装配 =====================

/** 把设备列表装配成 GeoJSON FeatureCollection（供 maplibre 的 cluster 源使用）。 */
export function devicesToFeatureCollection(devices, { fenceStates = [], fences = [], includeUnlocated = false } = {}) {
  const features = []
  devices.forEach((device) => {
    if (!device.has_location || device.longitude == null || device.latitude == null) {
      if (!includeUnlocated) return
      return
    }
    const status = resolveDeviceStatus(device, fenceStates, fences)
    features.push({
      type: 'Feature',
      id: device.device_code,
      geometry: { type: 'Point', coordinates: [Number(device.longitude), Number(device.latitude)] },
      properties: {
        device_code: device.device_code,
        device_name: device.device_name,
        product_code: device.product_code,
        status,
        source: device.source,
        accuracy_m: device.accuracy_m ?? null,
        fix_at: device.fix_at ?? null,
        received_at: device.received_at ?? null,
        stale: isStaleLocation(device.fix_at) ? 1 : 0,
      },
    })
  })
  return { type: 'FeatureCollection', features }
}

/** 把围栏装配成 GeoJSON（圆转成 64 边多边形，避免额外的符号层）。 */
export function fencesToFeatureCollection(fences, selectedFenceId = null) {
  const features = []
  const selected = selectedFenceId == null ? null : Number(selectedFenceId)
  ;(fences || []).forEach((fence) => {
    const geometry = fenceGeometry(fence)
    if (!geometry) return
    const id = Number(fence.ID ?? fence.id)
    features.push({
      type: 'Feature',
      id,
      geometry,
      properties: {
        fence_id: id,
        name: fence.name,
        kind: fence.kind,
        rule_type: fence.rule_type,
        enabled: fence.enabled ? 1 : 0,
        // 选中态写进数据而不是用 feature-state：feature-state 需要 promoteId，
        // 而围栏源每次 syncSources 都会整份 setData，用属性更直白。
        selected: selected !== null && selected === id ? 1 : 0,
      },
    })
  })
  return { type: 'FeatureCollection', features }
}

export function fenceGeometry(fence) {
  if (!fence) return null
  const kind = fence.kind
  if (kind === 'circle') {
    // 圆形围栏已不再支持新建，这里保留渲染转换只为让历史数据还能显示。
    const center = fenceCircleCenter(fence)
    const radius = Number(fence.radius_m ?? fence.radiusM ?? 0)
    if (!center || !(radius > 0)) return null
    return { type: 'Polygon', coordinates: [circleRing(center[0], center[1], radius)] }
  }
  if (kind === 'polygon' && fence.geometry) {
    try {
      const parsed = typeof fence.geometry === 'string' ? JSON.parse(fence.geometry) : fence.geometry
      const geometry = parsed?.type === 'Feature' ? parsed.geometry : parsed
      // 地区围栏是 MultiPolygon（海岛、飞地、被河道切开的区块），
      // 只放行 Polygon 会让"按地区名称建的围栏"在地图上完全消失。
      if (geometry?.type === 'Polygon' || geometry?.type === 'MultiPolygon') return geometry
    } catch (error) {
      return null
    }
  }
  return null
}

/** 统计围栏几何的顶点数与区块数（用于表单摘要与列表说明）。 */
export function fenceGeometryStats(fence) {
  const geometry = fenceGeometry(fence)
  if (!geometry) return { vertices: 0, polygons: 0 }
  const polygons = geometry.type === 'MultiPolygon' ? geometry.coordinates : [geometry.coordinates]
  let vertices = 0
  polygons.forEach((rings) => {
    ;(rings || []).forEach((ring) => {
      vertices += Math.max(0, (ring || []).length - 1) // 首尾闭合点不重复计
    })
  })
  return { vertices, polygons: polygons.length }
}

export function fenceCircleCenter(fence) {
  const lng = fence?.center_lng ?? fence?.centerLng
  const lat = fence?.center_lat ?? fence?.centerLat
  if (lng == null || lat == null) return null
  return [Number(lng), Number(lat)]
}

/** 把圆离散成多边形环（64 段足够平滑，且比自定义符号层更好做点击命中）。 */
export function circleRing(centerLng, centerLat, radiusM, steps = 64) {
  const metersPerDegreeLat = 111320
  const cosLat = Math.cos((centerLat * Math.PI) / 180) || 1e-6
  const ring = []
  for (let i = 0; i <= steps; i += 1) {
    const theta = (i / steps) * 2 * Math.PI
    const dLat = (radiusM * Math.sin(theta)) / metersPerDegreeLat
    const dLng = (radiusM * Math.cos(theta)) / (metersPerDegreeLat * cosLat)
    ring.push([centerLng + dLng, centerLat + dLat])
  }
  return ring
}

export const FENCE_RULE_TYPES = ['forbid', 'keep_in', 'notify']

export const FENCE_RULE_ACCENT = {
  forbid: { light: '#DC2626', dark: '#FF453A' },
  keep_in: { light: '#3B82F6', dark: '#409CFF' },
  notify: { light: '#D97706', dark: '#FF9F0A' },
}

export function fenceAccent(ruleType, isDark) {
  const entry = FENCE_RULE_ACCENT[ruleType] || FENCE_RULE_ACCENT.notify
  return isDark ? entry.dark : entry.light
}

/**
 * 与后端 `fallbackRoleFromIdentifier` 镜像的命名回退识别。
 *
 * 物模型里显式标记了 semantic 时以标记为准；没有标记时按 identifier 命名自动识别，
 * 让既有/导入的物模型免配置即可定位。这个函数只用于在编辑器里给出提示
 * ——「已自动识别，可手动指定」，实际判定仍然以后端为准。
 */
export function autoDetectLocationRole(identifier) {
  if (!identifier) return ''
  const normalized = String(identifier)
    .toLowerCase()
    .replace(/[^a-z0-9]/g, '')
  if (normalized.length < 3) return ''
  const hasSuffix = (...suffixes) => suffixes.some((suffix) => normalized.endsWith(suffix))
  if (hasSuffix('longitude', 'lng', 'lon')) return 'longitude'
  if (hasSuffix('latitude', 'lat')) return 'latitude'
  return ''
}
