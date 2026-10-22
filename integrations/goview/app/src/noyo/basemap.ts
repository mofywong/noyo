/**
 * IOC 大屏底图解析。
 *
 * 与「设备地图」页共用同一套 provider 与同一组 localStorage 键，原因很直接：
 * 天地图 Key 是**浏览器级**配置（域名白名单 + 每日配额），控制台里粘贴过一次，
 * 大屏就应该直接可用 —— 大屏跑在同源 iframe 里，localStorage 天然共享，
 * 不应再让用户为大屏单独配一次 Key。
 *
 * 合规口径沿用 2026-10-16 定案：生产底图＝天地图（官方审图号，CGCS2000≈WGS-84）；
 * OpenFreeMap 仅作开发/演示底图，且**只在用户自己于控制台选中过它时才使用**。
 */

const TOKEN_KEY = 'noyo.map.tianditu.token'
const PREF_KEY = 'noyo.map.basemap'
const SUBDOMAINS = ['t0', 't1', 't2', 't3', 't4', 't5', 't6', 't7']

export type BasemapMode = 'auto' | 'tianditu' | 'openfreemap' | 'schematic'

export interface BasemapReady {
  ok: true
  provider: 'tianditu' | 'openfreemap'
  /** maplibre 可直接消费的 style：天地图给对象，OpenFreeMap 给 URL。 */
  style: any
  attribution: string
  maxZoom: number
}

export interface BasemapMissing {
  ok: false
  provider: BasemapMode
  /** 回退示意网格时展示的原因，双语与组件内其它文案同构。 */
  hint: [string, string]
  detail?: string
}

export type BasemapResolution = BasemapReady | BasemapMissing

function readStorage(key: string): string {
  try {
    return window.localStorage.getItem(key) || ''
  } catch {
    // 隐私模式/沙箱下 localStorage 会抛异常，按未配置处理。
    return ''
  }
}

export function loadTiandituToken(): string {
  return readStorage(TOKEN_KEY).trim()
}

/** 用户上次在控制台选中的底图；默认 openfreemap 与控制台保持一致。 */
export function loadProviderPreference(): string {
  return readStorage(PREF_KEY) || 'openfreemap'
}

/**
 * 天地图 WMTS 瓦片地址。
 *
 * `_w` 后缀＝球面墨卡托（EPSG:3857），MapLibre 只认这个；`_c` 是经纬度投影，
 * 挂上去整体错位。子域显式展开而不是用 `{s}`，避免不同渲染器的解析差异。
 */
function tiandituTiles(service: string, layer: string, token: string): string[] {
  return SUBDOMAINS.map(sub =>
    `https://${sub}.tianditu.gov.cn/${service}_w/wmts` +
    `?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0` +
    `&LAYER=${layer}&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles` +
    `&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}` +
    `&tk=${encodeURIComponent(token)}`
  )
}

/**
 * 栅格底图样式：底图 `vec` + 注记 `cva` 两层。
 *
 * 刻意不加任何 `raster-*` 着色参数 —— 天地图高速公路的粗紫线是烘进瓦片的
 * 制图规范，WMTS 没有样式入口，任何滤镜补偿都只会引入新的色彩失真
 * （2026-10-19 已定论，不要再试）。
 */
function tiandituStyle(token: string) {
  return {
    version: 8,
    sources: {
      'tdt-base': { type: 'raster', tiles: tiandituTiles('vec', 'vec', token), tileSize: 256, minzoom: 1, maxzoom: 18, attribution: '© 天地图' },
      'tdt-label': { type: 'raster', tiles: tiandituTiles('cva', 'cva', token), tileSize: 256, minzoom: 1, maxzoom: 18 },
    },
    layers: [
      { id: 'tdt-base', type: 'raster', source: 'tdt-base' },
      { id: 'tdt-label', type: 'raster', source: 'tdt-label' },
    ],
  }
}

/**
 * 解析出可用的底图。
 *
 * `schematic` 是显式的"只要示意网格"，用于没有外网的内网墙面；
 * `auto` 跟随控制台配置，任何一方不可用都退回示意网格并说明原因 ——
 * 大屏宁可显示"没有底图"，也不能显示一张假的底图。
 */
export function resolveBasemap(mode: BasemapMode = 'auto'): BasemapResolution {
  if (mode === 'schematic') {
    return { ok: false, provider: 'schematic', hint: ['示意网格', 'Schematic grid'] }
  }
  const token = loadTiandituToken()
  const preference = mode === 'auto' ? loadProviderPreference() : mode

  if (preference !== 'openfreemap') {
    if (!token) {
      return {
        ok: false,
        provider: 'tianditu',
        hint: ['未配置天地图 Key，无法加载底图', 'No Tianditu key: basemap unavailable'],
        detail: '设备地图 › 底图设置',
      }
    }
    return { ok: true, provider: 'tianditu', style: tiandituStyle(token), attribution: '© 天地图', maxZoom: 18 }
  }
  return {
    ok: true,
    provider: 'openfreemap',
    style: 'https://tiles.openfreemap.org/styles/liberty',
    attribution: '© OpenFreeMap © OpenMapTiles © OpenStreetMap contributors',
    maxZoom: 20,
  }
}
