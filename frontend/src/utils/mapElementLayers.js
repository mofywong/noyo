/**
 * 地图元素可见性。
 *
 * 能力边界（必须先说清，否则 UI 会许下做不到的承诺）：
 * - **矢量底图**（开发期 OpenFreeMap/Liberty）的样式是 100 多个独立图层，可以按
 *   类别逐项开关 —— 道路、水系、绿地、建筑、注记、POI、行政边界都在这里实现。
 * - **天地图是 WMTS 栅格瓦片**，道路/水系/建筑已经烘进 PNG，**物理上不能单独关掉**；
 *   唯一可分离的只有注记服务（`cva`，样式里是 `tdt-label` 图层）。
 *   所以栅格底图下只开放"注记"一项，其余置灰并给出说明。
 *
 * 另一个铁律：**关掉用 'none'，打开要还原图层原本的 visibility**。
 * 直接写 'visible' 会把样式里刻意隐藏的图层（例如只在 z16 以上出现的 3D 建筑）
 * 强行点亮，造成"恢复默认反而更乱"。
 */

const STORAGE_KEY = 'noyo.map.elements'

/** 业务图层分组（我们自己的图层，与底图无关）。 */
export const BUSINESS_LAYER_IDS = {
  // device-pulse / device-pulse-wave 是状态脉冲的两层光晕（柔光 + 扩散环）：
  // 它们描述的是设备点位，必须跟点位同生共死，
  // 否则会出现"点位关掉了、光晕还在地图上闪动"。
  devices: ['device-pulse-wave', 'device-pulse', 'device-cluster', 'device-point', 'device-selected'],
  fences: ['fence-fill', 'fence-line'],
}

export const BASE_ELEMENT_GROUPS = [
  { id: 'labels', key: 'device_map_layer_labels' },
  { id: 'poi', key: 'device_map_layer_poi' },
  { id: 'roads', key: 'device_map_layer_roads' },
  { id: 'water', key: 'device_map_layer_water' },
  { id: 'land', key: 'device_map_layer_land' },
  { id: 'buildings', key: 'device_map_layer_buildings' },
  { id: 'boundary', key: 'device_map_layer_boundary' },
]

export const BUSINESS_LAYER_GROUPS = [
  { id: 'devices', key: 'device_map_layer_devices' },
  { id: 'fences', key: 'device_map_layer_fences' },
]

export const DEFAULT_ELEMENT_PREFS = {
  labels: true,
  poi: true,
  roads: true,
  water: true,
  land: true,
  buildings: true,
  boundary: true,
  devices: true,
  fences: true,
}

const isTextLayer = (layer) => layer.type === 'symbol' && Boolean(layer.layout && layer.layout['text-field'])

/**
 * 图层归类。**顺序即优先级**：`poi_*` 也是带 text-field 的 symbol 图层，
 * 必须排在 labels 之前，否则兴趣点会被并进"注记"。
 * 分类规则按 OpenFreeMap Liberty 的实际图层 id 写死（已核对 111 个图层），
 * 换底图样式时需要重新核对一遍。
 */
const CLASSIFIERS = [
  { id: 'poi', match: (layer) => /^poi_/.test(layer.id) },
  { id: 'labels', match: (layer) => isTextLayer(layer) },
  { id: 'roads', match: (layer) => /^(road_|bridge_|tunnel_|aeroway_)/.test(layer.id) },
  { id: 'water', match: (layer) => layer.id === 'water' || /^waterway_/.test(layer.id) },
  { id: 'land', match: (layer) => /^(park|landuse_|landcover_)/.test(layer.id) },
  { id: 'buildings', match: (layer) => /^building/.test(layer.id) },
  { id: 'boundary', match: (layer) => /^boundary_/.test(layer.id) },
]

export function classifyLayer(layer) {
  const hit = CLASSIFIERS.find((rule) => rule.match(layer))
  return hit ? hit.id : null
}

export function loadElementPrefs() {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) return { ...DEFAULT_ELEMENT_PREFS }
    const parsed = JSON.parse(raw)
    const merged = { ...DEFAULT_ELEMENT_PREFS }
    for (const key of Object.keys(DEFAULT_ELEMENT_PREFS)) {
      if (typeof parsed?.[key] === 'boolean') merged[key] = parsed[key]
    }
    return merged
  } catch (error) {
    return { ...DEFAULT_ELEMENT_PREFS }
  }
}

export function saveElementPrefs(prefs) {
  try {
    const payload = {}
    for (const key of Object.keys(DEFAULT_ELEMENT_PREFS)) payload[key] = prefs[key] !== false
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(payload))
  } catch (error) {
    /* localStorage 不可用时静默降级 */
  }
}

function applyVisibility(map, id, visible, original) {
  if (!map.getLayer(id)) return
  const next = visible ? (original || 'visible') : 'none'
  let current
  try {
    current = map.getLayoutProperty(id, 'visibility')
  } catch (error) {
    current = undefined
  }
  if (current === next) return
  map.setLayoutProperty(id, 'visibility', next)
}

/**
 * 把偏好落到当前样式上。切换底图（setStyle）会重建所有图层，
 * 所以每次 style.load 之后都必须重跑一遍。
 */
export function applyElementVisibility(map, prefs) {
  if (!map || typeof map.getStyle !== 'function') return
  const style = map.getStyle()
  const rasterBasemap = Boolean(map.getLayer('tdt-label'))

  if (rasterBasemap) {
    // 栅格底图：只有注记层是独立图层，其余已烘进瓦片。
    applyVisibility(map, 'tdt-label', prefs.labels !== false, 'visible')
  } else {
    for (const layer of style.layers || []) {
      const group = classifyLayer(layer)
      if (!group) continue
      applyVisibility(map, layer.id, prefs[group] !== false, layer.layout?.visibility || 'visible')
    }
  }

  for (const [group, ids] of Object.entries(BUSINESS_LAYER_IDS)) {
    for (const id of ids) applyVisibility(map, id, prefs[group] !== false, 'visible')
  }
}
