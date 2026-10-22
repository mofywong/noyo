<template>
  <div class="location-map" :style="box" :data-map-state="mapState" :data-map-provider="provider" :data-map-idle="idle ? '1' : '0'" :data-map-errors="tileErrors">
    <div v-show="mapState === 'loading' || mapState === 'ready'" ref="canvasRef" class="location-map-canvas" :style="canvasStyle" />
    <!--
      示意网格是**保底**而不是"另一种底图"：没有底图能力时（无 Key / 内网 / 离线）
      让大屏还能正确表达"设备之间相对位置"，同时明确标注它没有地理底图。
    -->
    <svg v-if="mapState === 'schematic' || mapState === 'fallback'" :viewBox="`0 0 ${width} ${height}`" role="img" :aria-label="t('设备位置示意图', 'Schematic device locations')">
      <path v-for="n in 5" :key="`v${n}`" class="map-grid" :d="`M ${width * n / 6} 32 V ${height - 32}`" />
      <path v-for="n in 4" :key="`h${n}`" class="map-grid" :d="`M 32 ${height * n / 5} H ${width - 32}`" />
      <path v-for="(line, i) in outlinePaths" :key="`outline${i}`" class="map-outline" :d="line" />
      <g
        v-for="point in points"
        :key="point.deviceCode"
        class="map-marker"
        :data-device="point.deviceCode"
        :class="{ degraded: point.quality !== 'good' }"
        :transform="`translate(${projection.project([point.longitude, point.latitude]).join(',')})`"
      >
        <circle r="6" />
        <title>{{ labelOf(point) }} · {{ point.longitude.toFixed(5) }}, {{ point.latitude.toFixed(5) }} · {{ qualityLabel(point.quality || 'no_data')[0] }}</title>
        <text x="12" y="-8">{{ labelOf(point) }}</text>
      </g>
      <text x="16" y="24" class="map-caption">{{ t('示意网格 · 非地理底图', 'Schematic grid · not a geographic basemap') }} · {{ crs === 'gcj02' ? 'GCJ-02' : 'WGS84' }}</text>
      <text x="16" :y="height - 16" class="map-caption">{{ points.length }} {{ t('台设备', 'devices') }} · {{ t('经度 / 纬度', 'Longitude / latitude') }}</text>
    </svg>
    <p v-if="mapState === 'loading'" class="map-loading">{{ t('底图加载中…', 'Loading basemap…') }}</p>
    <p v-if="mapState === 'fallback'" class="map-fallback">{{ fallbackHint }}</p>
    <p v-if="mapState === 'ready' && !points.length" class="map-empty" role="status">{{ t('暂无可显示的位置', 'No locations to display') }}</p>
    <div class="map-caption-band">
      <span class="map-caption-title">{{ t('设备分布', 'Device locations') }}</span>
      <span class="map-caption-meta">{{ points.length }} {{ t('台', 'devices') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import 'maplibre-gl/dist/maplibre-gl.css'
// MapLibre 默认按 `new URL('maplibre-gl-worker.mjs', import.meta.url)` 就地找 worker；
// 打进产物后该地址不存在，后端静态服务会回退成 index.html ⇒ worker 拿到一段 HTML
// 静默启动失败，表现为"底图一片空白、控制台只有一句 worker 报错"。
// 用 `?url` 让 vite 明确产出该文件，再把地址交给 maplibre。
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'
import { subscribeWidget, qualityLabel } from './runtime'
import { state } from './bridge'
import { resolveBasemap, type BasemapMode } from './basemap'
import { mapProjection, safePoints, type MapPoint, type Position } from './mapGeometry'

const props = defineProps<{ chartConfig: any }>()
// Keep legible SVG coordinates while the viewport follows the assigned box.
const width = computed(() => Math.max(240, props.chartConfig.attr.w))
const height = computed(() => Math.max(192, props.chartConfig.attr.h))
const box = computed(() => ({ width: `${props.chartConfig.attr.w}px`, height: `${props.chartConfig.attr.h}px` }))
const option = computed(() => props.chartConfig.option || {})
const crs = computed(() => option.value.coordinateSystem || 'wgs84')

const canvasRef = ref<HTMLElement | null>(null)
const mapState = ref<'loading' | 'ready' | 'fallback' | 'schematic'>('loading')
// `load` 只代表样式解析完，瓦片还没画出来。把 idle 单独暴露出去，
// 截图/巡检脚本才能等到"真的有图"而不是等到"容器建好了"。
const idle = ref(false)
const tileErrors = ref(0)
const provider = ref<string>('')
const fallbackHint = ref('')
const map = ref<any>(null)
let maplibre: any = null
let resizeObserver: ResizeObserver | null = null
let fitted = false

const value = ref<any>([])
subscribeWidget(props.chartConfig.id, data => { value.value = data })
const points = computed<MapPoint[]>(() => safePoints(value.value, crs.value))
const lines = computed<Position[][]>(() => option.value.geoLines || [])
const projection = computed(() => mapProjection(points.value, lines.value, width.value, height.value))
const outlinePaths = computed(() => lines.value.map(line => line.map((position, index) => `${index ? 'L' : 'M'} ${projection.value.project(position).join(' ')}`).join(' ')))

/**
 * 暗色屏适配：对**瓦片画布**做亮度衰减，DOM 标注不受影响。
 *
 * 为什么不做全局半透明暗层：标注是 DOM Marker（栅格底图没有 glyphs，文字只能走 DOM），
 * 盖一层暗层会把标注一起压灰。为什么不用 `raster-opacity`：那是对底图颜色的再加工，
 * 已明确定论不要（见 mapProviders 注释）。
 */
const canvasStyle = computed(() => {
  const dim = Number(option.value.dim ?? 0.55)
  if (!dim) return {}
  return { filter: `brightness(${(1 - Math.min(Math.max(dim, 0), 0.8)).toFixed(2)}) saturate(0.9)` }
})

const t = (zh: string, en: string) => (state.locale?.startsWith('en') ? en : zh)
function labelOf(point: MapPoint) {
  const name = String(point.deviceName || '').trim()
  return name || point.deviceCode
}

const markers = new Map<string, { marker: any; el: HTMLElement; label: HTMLElement; link: SVGLineElement }>()

/**
 * 标注排在点位哪一侧。
 *
 * 大屏地图面板是宽扁的（约 3:1），厂区设备在南北向铺开 ⇒ 全览后点位会挤成一条竖线，
 * 统一把标注放右侧必然互相压字。按经度相对簇中心分成左右两侧，等于把可用横向空间
 * 翻了一倍，且规则稳定（同一台设备不会随数据刷新来回跳边）。
 */
const clusterCenter = computed(() => {
  const list = points.value
  if (!list.length) return null
  return {
    longitude: list.reduce((sum, point) => sum + point.longitude, 0) / list.length,
    latitude: list.reduce((sum, point) => sum + point.latitude, 0) / list.length,
  }
})

const SVG_NS = 'http://www.w3.org/2000/svg'

function createMarker(point: MapPoint) {
  const el = document.createElement('div')
  el.className = 'device-marker'
  // 标注被错开时，用一根细引线连回点位 —— 否则六七台设备挤在一起时，
  // 谁也说不清哪个名字属于哪个点。
  const svg = document.createElementNS(SVG_NS, 'svg')
  svg.setAttribute('class', 'device-marker-link')
  svg.setAttribute('width', '12')
  svg.setAttribute('height', '12')
  const link = document.createElementNS(SVG_NS, 'line')
  svg.append(link)
  const dot = document.createElement('span')
  dot.className = 'device-marker-dot'
  const label = document.createElement('span')
  label.className = 'device-marker-label'
  el.append(svg, dot, label)
  const marker = new maplibre.Marker({ element: el, anchor: 'center' })
    .setLngLat([point.longitude, point.latitude])
    .addTo(map.value)
  const entry = { marker, el, label, link }
  markers.set(point.deviceCode, entry)
  return entry
}

function syncMarkers() {
  if (!map.value || mapState.value !== 'ready') return
  const alive = new Set<string>()
  const center = clusterCenter.value
  for (const point of points.value) {
    alive.add(point.deviceCode)
    const entry = markers.get(point.deviceCode) || createMarker(point)
    entry.marker.setLngLat([point.longitude, point.latitude])
    const text = labelOf(point)
    if (entry.label.textContent !== text) entry.label.textContent = text
    const side = center && point.longitude < center.longitude ? 'left' : 'right'
    if (entry.el.dataset.side !== side) entry.el.dataset.side = side
    const quality = point.quality || 'no_data'
    if (entry.el.dataset.quality !== quality) entry.el.dataset.quality = quality
    const title = `${text}${text === point.deviceCode ? '' : `（${point.deviceCode}）`} · ${point.longitude.toFixed(5)}, ${point.latitude.toFixed(5)} · ${qualityLabel(quality)[0]}`
    if (entry.el.title !== title) entry.el.title = title
  }
  for (const [code, entry] of markers) {
    if (alive.has(code)) continue
    entry.marker.remove()
    markers.delete(code)
  }
  layoutLabels()
}

/**
 * 标注避让。
 *
 * 厂区设备常常几百米一簇（同一栋楼的探头），全览后必然压字。这里按"上面优先"
 * 逐个放置：先试默认位置，冲突就上下错开一档，仍冲突再换另一侧 —— 只挪标注、
 * 不动点位，读图的人看到的仍是真实坐标。
 */
function layoutLabels() {
  if (!map.value || mapState.value !== 'ready') return
  const taken: Array<{ x1: number; y1: number; x2: number; y2: number }> = []
  const order = [...points.value].sort((a, b) => b.latitude - a.latitude)
  for (const point of order) {
    const entry = markers.get(point.deviceCode)
    if (!entry) continue
    const screen = map.value.project([point.longitude, point.latitude])
    const width = entry.label.offsetWidth || 90
    const height = entry.label.offsetHeight || 18
    const preferred = entry.el.dataset.side === 'left' ? 'left' : 'right'
    const sides = preferred === 'left' ? ['left', 'right'] : ['right', 'left']
    let chosen: { left: number; top: number; side: string; box: any } | null = null
    outer: for (const side of sides) {
      const left = side === 'right' ? 16 : 6 - 10 - width
      for (const shift of [0, -1, 1, -2, 2, -3, 3]) {
        // 6 = 点位圆心在 12px 标记盒里的偏移；shift 以 20px 为一档。
        const top = 6 - height / 2 + shift * 20
        const box = {
          x1: screen.x - 6 + left,
          y1: screen.y - 6 + top,
          x2: screen.x - 6 + left + width,
          y2: screen.y - 6 + top + height,
        }
        if (taken.some(other => box.x1 < other.x2 && box.x2 > other.x1 && box.y1 < other.y2 && box.y2 > other.y1)) continue
        chosen = { left, top, side, box }
        break outer
      }
    }
    if (!chosen) continue
    taken.push(chosen.box)
    entry.label.style.left = `${chosen.left}px`
    entry.label.style.right = 'auto'
    entry.label.style.top = `${chosen.top}px`
    entry.label.style.transform = 'none'
    entry.el.dataset.side = chosen.side
    const anchorX = chosen.side === 'right' ? chosen.left : chosen.left + width
    const anchorY = chosen.top + height / 2
    entry.link.setAttribute('x1', '6')
    entry.link.setAttribute('y1', '6')
    entry.link.setAttribute('x2', String(anchorX))
    entry.link.setAttribute('y2', String(anchorY))
    // 只有真的错开时才画线；正对时那 4px 间距不需要引线。
    entry.link.style.display = Math.abs(anchorY - 6) < 2 ? 'none' : ''
  }
}

/** 只做一次全览：每秒一批数据也在动，反复 fitBounds 会让地图自己抖。 */
function fitOnce() {
  if (fitted || !map.value || !points.value.length) return
  const bounds = new maplibre.LngLatBounds()
  for (const point of points.value) bounds.extend([point.longitude, point.latitude])
  map.value.fitBounds(bounds, { padding: 64, maxZoom: Number(option.value.maxZoom || 15), duration: 0 })
  fitted = true
}

function outlineGeoJSON() {
  return {
    type: 'Feature',
    properties: {},
    geometry: {
      type: 'MultiLineString',
      coordinates: lines.value.map(line => line.map(([lng, lat]) => [lng, lat])),
    },
  }
}

function syncOutline() {
  if (!map.value) return
  const source = map.value.getSource?.('noyo-outline')
  if (!source) {
    if (!lines.value.length) return
    map.value.addSource('noyo-outline', { type: 'geojson', data: outlineGeoJSON() })
    map.value.addLayer({
      id: 'noyo-outline',
      type: 'line',
      source: 'noyo-outline',
      paint: { 'line-color': option.value.outlineColor || '#ffd166', 'line-width': 1.6, 'line-dasharray': [2, 2] },
    })
    return
  }
  source.setData(outlineGeoJSON())
}

async function boot() {
  const mode: BasemapMode = (option.value.basemap || 'auto') as BasemapMode
  const resolved = resolveBasemap(mode)
  provider.value = resolved.provider
  if (!resolved.ok) {
    fallbackHint.value = t(resolved.hint[0], resolved.hint[1]) + (resolved.detail ? ` · ${resolved.detail}` : '')
    mapState.value = mode === 'schematic' ? 'schematic' : 'fallback'
    return
  }
  try {
    maplibre = await import('maplibre-gl')
    maplibre.setWorkerUrl(maplibreWorkerUrl)
    await nextTick()
    if (!canvasRef.value) return
    map.value = new maplibre.Map({
      container: canvasRef.value,
      style: resolved.style,
      center: option.value.center || [104.2, 35.9],
      zoom: Number(option.value.zoom ?? 3.5),
      maxZoom: resolved.maxZoom,
      attributionControl: false,
      // 大屏墙面不装鼠标，但编辑器里要能拖动核对，交互保持开启。
      interactive: option.value.interactive !== false,
      fadeDuration: 0,
    })
    map.value.on('load', () => {
      mapState.value = 'ready'
      syncOutline()
      syncMarkers()
      fitOnce()
      map.value?.resize()
    })
    // 平移/缩放后点位相对位置变了，避让结果要重算。
    map.value.on('moveend', () => layoutLabels())
    map.value.on('idle', () => { idle.value = true; layoutLabels() })
    map.value.on('error', (event: any) => {
      if (!String(event?.error?.message || '').match(/Failed to fetch|NetworkError|401|403/)) return
      tileErrors.value += 1
      console.warn('[noyo-screen] basemap tile request failed', event?.error?.message)
    })
    resizeObserver = new ResizeObserver(() => map.value?.resize())
    resizeObserver.observe(canvasRef.value)
  } catch (error) {
    console.warn('[noyo-screen] basemap unavailable', error)
    fallbackHint.value = t('底图初始化失败，已改为示意网格', 'Basemap failed to start; showing schematic grid')
    mapState.value = 'fallback'
  }
}

watch(points, () => {
  syncMarkers()
  fitOnce()
})
watch(lines, () => syncOutline())

onMounted(boot)
onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
  for (const entry of markers.values()) entry.marker.remove()
  markers.clear()
  map.value?.remove()
  map.value = null
})
</script>

<style scoped>
.location-map{position:relative;background:var(--bg-surface);color:var(--text-primary);overflow:hidden;border:1px solid var(--border-color);border-radius:8px}
.location-map-canvas{position:absolute;inset:0}
.location-map-canvas :deep(.maplibregl-canvas){outline:none}
.location-map svg{display:block;width:100%;height:100%}
.map-grid{stroke:var(--border-color);stroke-width:1;fill:none}
.map-outline{stroke:var(--text-secondary);stroke-width:1.5;fill:none}
.map-marker{fill:var(--primary-color)}
.map-marker.degraded{fill:var(--warning-color)}
.map-marker text,.map-caption{fill:var(--text-primary);font:12px system-ui,sans-serif}
.map-caption{fill:var(--text-secondary)}
.map-empty{position:absolute;inset:32px 0;display:grid;place-items:center;margin:0;padding:96px 24px;color:var(--text-secondary);pointer-events:none;font:14px system-ui,sans-serif}
.map-loading,.map-fallback{position:absolute;left:0;right:0;bottom:10px;margin:0;text-align:center;font:12px system-ui,sans-serif;color:var(--text-secondary);pointer-events:none}
.map-fallback{color:var(--warning-color)}
/* 底图没有标题栏，大屏上需要一行说明它是什么；这条带子同时压住瓦片顶部的亮区。 */
.map-caption-band{position:absolute;top:0;left:0;right:0;height:26px;display:flex;align-items:center;gap:8px;padding:0 10px;background:linear-gradient(180deg,rgba(3,10,20,.82),rgba(3,10,20,0));pointer-events:none;font:12px/1 system-ui,sans-serif}
.map-caption-title{color:#cfe3f5;letter-spacing:.12em}
.map-caption-meta{color:#7f9bb3}
</style>

<style>
/*
  Marker 由 MapLibre 挂到 map container 上，作用域样式够不到它，因此这段是全局的。
  类名带 noyo- 前缀，避免与上游组件样式撞车。
*/
.noyo-screen-map .device-marker,
.device-marker{position:relative;width:12px;height:12px;cursor:default}
.device-marker-link{position:absolute;left:0;top:0;overflow:visible;pointer-events:none}
.device-marker-link line{stroke:rgba(190,220,245,.5);stroke-width:1}
.device-marker-dot{position:absolute;inset:0;border-radius:50%;background:#38bdf8;border:2px solid rgba(255,255,255,.85);box-shadow:0 0 8px rgba(56,189,248,.8)}
.device-marker[data-quality="good"] .device-marker-dot{background:#34d399;box-shadow:0 0 10px rgba(52,211,153,.85)}
.device-marker[data-quality="stale"] .device-marker-dot{background:#fbbf24;box-shadow:0 0 10px rgba(251,191,36,.75)}
.device-marker[data-quality="no_data"] .device-marker-dot,
.device-marker[data-quality="error"] .device-marker-dot{background:#94a3b8;box-shadow:0 0 6px rgba(148,163,184,.6)}
.device-marker-label{position:absolute;left:16px;top:50%;transform:translateY(-50%);max-width:9em;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;padding:1px 5px;border-radius:3px;background:rgba(4,12,22,.86);border:1px solid rgba(120,170,210,.28);font:12px/1.35 system-ui,sans-serif;color:#f8fafc;text-shadow:0 1px 2px rgba(0,0,0,.95);pointer-events:none}
.device-marker[data-side="left"] .device-marker-label{left:auto;right:16px;text-align:right}
.device-marker[data-quality="no_data"] .device-marker-label,
.device-marker[data-quality="error"] .device-marker-label{color:#cbd5e1}
</style>
