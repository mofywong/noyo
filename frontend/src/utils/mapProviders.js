/**
 * 底图 provider 抽象层。
 *
 * 目的：让「开发期 OpenFreeMap / 生产天地图 / 未来离线 PMTiles」三者共存，
 * 业务层只持有 WGS-84 坐标，底图差异全部收敛在这里。
 *
 * 关键约定（方案文档 §7 / §10 / §11）：
 * - **坐标单一真源是 WGS-84**。设备坐标入库即为 WGS-84；只有 provider 声明的
 *   crs 不是 WGS84 时，才在渲染前转换，且**绝不回写数据库**。
 * - `attribution` 由 provider 自带 ⇒ 换底图不会漏掉版权与审图号。
 * - `darkMode` 声明式表达暗色处理方式 ⇒ 切底图时 UI 自动跟随。
 */

export const CRS_WGS84 = 'WGS84'
export const CRS_GCJ02 = 'GCJ02'

/** 方案甲：底图之上叠半透明暗层。收敛为单一常量，便于后续调优。 */
export const DARK_OVERLAY = 'rgba(13, 15, 20, 0.55)'
/** 注记层在暗色下的不透明度 —— 保证中文标注始终可读。 */
export const DARK_LABEL_OPACITY = 0.92

export const TIANDITU_SUBDOMAINS = ['t0', 't1', 't2', 't3', 't4', 't5', 't6', 't7']

/**
 * 关于天地图高速公路的粗紫线 —— 已核实：**前端无法从根本上去掉，故不作任何着色补偿**。
 *
 * 三条已确认的事实：
 * 1. 紫线是官方制图规范的一部分，**已经烘进 `vec` 瓦片像素**，不是独立矢量要素；
 * 2. WMTS 请求只接受 `STYLE=default`，官方未开放 SLD/自定义样式
 *    （OGC 的 `STYLES` 参数在这里没有第二个可选值，服务端也不解析外部样式 URL）；
 * 3. 官方服务清单里没有"无道路 / 无高速"的图层变体（`vec`/`cva`/`img`/`ter`/`ibo`）。
 *
 * 因此可选项只有三个，全部被否决或代价过高：
 * - 全局 `raster-saturation` 降饱和：能压成灰紫，但会连带把水系、绿地一起洗掉，
 *   而且紫线仍然在，只是变灰 —— 主人明确要求"不要减弱高饱和度"，已移除；
 * - 换 `img` 影像底图：不含道路配色，但主人已否决影像底图；
 * - 换矢量底图（可图层级删 `motorway`）：不满足大陆生产合规要求。
 *
 * 结论：接受现状。`vec` 里出现紫线属于底图固有表现，不是前端缺陷，
 * 不要再为此加着色/滤镜类补丁（会引入新的色彩失真且收益为零）。
 */

const TIANDITU_TOKEN_STORAGE_KEY = 'noyo.map.tianditu.token'
const BASEMAP_STORAGE_KEY = 'noyo.map.basemap'

/**
 * 天地图 WMTS 瓦片地址。
 *
 * 投影后缀必须是 `_w`（球面墨卡托 EPSG:3857）—— MapLibre 用的是 3857，
 * `_c` 是经纬度投影，挂上去会错位。
 *
 * 注意子域展开：不使用 `{s}` 占位符，而是显式展开成多个 URL，
 * 避免不同渲染器对 subdomains 的解析差异。
 */
export function tiandituTileUrls(service, layer, token) {
  const query = (sub) =>
    `https://${sub}.tianditu.gov.cn/${service}_w/wmts` +
    `?SERVICE=WMTS&REQUEST=GetTile&VERSION=1.0.0` +
    `&LAYER=${layer}&STYLE=default&TILEMATRIXSET=w&FORMAT=tiles` +
    `&TILECOL={x}&TILEROW={y}&TILEMATRIX={z}` +
    `&tk=${encodeURIComponent(token || '')}`
  return TIANDITU_SUBDOMAINS.map(query)
}

/**
 * 天地图底图样式：底图 + 注记两层独立挂载。
 *
 * 两层分开挂的原因不是"美观"，而是**栅格底图唯一能做的元素级开关就是注记**
 * （`cva` 是独立服务）。底图 `vec` 里的道路/水系/建筑已经烘进 PNG，
 * 按官方制图规范高速公路还是粗紫线，WMTS 没有样式入口，前端关不掉。
 * 要素级开关请走矢量底图，见 utils/mapElementLayers.js 的能力边界说明。
 *
 * 此处**刻意不加任何 `raster-*` 着色参数**：底色就是官方原色。
 */
function composeTiandituStyle(token, base, label) {
  return {
    version: 8,
    sources: {
      'tdt-base': {
        type: 'raster',
        tiles: tiandituTileUrls(base, base, token),
        tileSize: 256,
        minzoom: 1,
        maxzoom: 18,
        attribution: '© 天地图',
      },
      'tdt-label': {
        type: 'raster',
        tiles: tiandituTileUrls(label, label, token),
        tileSize: 256,
        minzoom: 1,
        maxzoom: 18,
      },
    },
    layers: [
      { id: 'tdt-base', type: 'raster', source: 'tdt-base' },
      { id: 'tdt-label', type: 'raster', source: 'tdt-label' },
    ],
  }
}

/** 默认矢量底图（含路网，高速为紫色粗线）。 */
export function buildTiandituStyle(token) {
  return composeTiandituStyle(token, 'vec', 'cva')
}

const TIANDITU_MISSING_TOKEN = {
  zh: '需要先配置天地图浏览器端 Key（并设置域名白名单）',
  en: 'Configure a browser-side Tianditu key (with a domain whitelist) first',
}

/**
 * provider 定义。`id` 是稳定标识，写入 localStorage。
 */
export const BASEMAP_PROVIDERS = [
  {
    id: 'tianditu',
    label: { zh: '天地图', en: 'Tianditu' },
    hint: {
      zh: '生产底图 · 官方审图号 · CGCS2000 与 WGS-84 差异 <1m',
      en: 'Production basemap · official approval · CGCS2000 within 1m of WGS-84',
    },
    crs: CRS_WGS84,
    // 栅格瓦片：元素级开关只到"注记"这一层（见 mapElementLayers.js）。
    kind: 'raster',
    requiresToken: true,
    minZoom: 1,
    maxZoom: 18,
    darkMode: 'overlay',
    attribution: '© 天地图',
    // 提供 buildStyle 而非直接给 style URL —— 它需要 token。
    buildStyle: (options = {}) => buildTiandituStyle(options.token),
    missingTokenMessage: TIANDITU_MISSING_TOKEN,
  },
  {
    id: 'openfreemap',
    label: { zh: 'OpenFreeMap（开发预览）', en: 'OpenFreeMap (development)' },
    hint: {
      zh: '免 Key、无配额，仅用于开发调试；不可作为面向大陆客户的生产底图',
      en: 'No key, no quota, for development only; not a compliant production basemap for mainland China',
    },
    crs: CRS_WGS84,
    // 矢量瓦片：道路/水系/绿地/建筑/注记可以逐项开关。
    kind: 'vector',
    requiresToken: false,
    minZoom: 0,
    maxZoom: 20,
    darkMode: 'overlay',
    attribution: '© OpenFreeMap © OpenMapTiles © OpenStreetMap contributors',
    styleUrl: 'https://tiles.openfreemap.org/styles/liberty',
  },
]

export function getProvider(id) {
  return BASEMAP_PROVIDERS.find((item) => item.id === id) || null
}

/** 记住上次选择的底图；默认开发期用 OpenFreeMap，避免烧穿天地图配额。 */
export function loadBasemapPreference() {
  try {
    const stored = window.localStorage.getItem(BASEMAP_STORAGE_KEY)
    if (stored && getProvider(stored)) return stored
  } catch (error) {
    /* localStorage 不可用时静默降级 */
  }
  return 'openfreemap'
}

export function saveBasemapPreference(id) {
  try {
    window.localStorage.setItem(BASEMAP_STORAGE_KEY, id)
  } catch (error) {
    /* ignore */
  }
}

export function loadTiandituToken() {
  try {
    return window.localStorage.getItem(TIANDITU_TOKEN_STORAGE_KEY) || ''
  } catch (error) {
    return ''
  }
}

export function saveTiandituToken(token) {
  try {
    window.localStorage.setItem(TIANDITU_TOKEN_STORAGE_KEY, token || '')
  } catch (error) {
    /* ignore */
  }
}

/**
 * 解析出一个可直接喂给 maplibre 的样式描述。
 * 返回值：{ style: string | object, attribution, crs, darkMode, minZoom, maxZoom }
 */
export function resolveBasemap(providerId, options = {}) {
  const provider = getProvider(providerId)
  if (!provider) return null

  if (provider.requiresToken && !options.token) {
    return { ...provider, style: null, unavailable: true }
  }
  if (provider.buildStyle) {
    return { ...provider, style: provider.buildStyle(options) }
  }
  return { ...provider, style: provider.styleUrl }
}

// ===================== 坐标转换 =====================
//
// 设备坐标入库为 WGS-84，天地图是 CGCS2000（实务等同 WGS-84），因此**当前不需要转换**。
// 下面的 GCJ-02 转换是为「将来接入高德/腾讯底图」预留的 —— 一旦接入，
// 转换只发生在**渲染前**，绝不回写数据库（跨模块铁律，见方案文档 §11）。

const GCJ02_A = 6378245.0
const GCJ02_EE = 0.00669342162296594323

function outOfChina(lng, lat) {
  return lng < 72.004 || lng > 137.8347 || lat < 0.8293 || lat > 55.8271
}

function transformLat(x, y) {
  let ret = -100.0 + 2.0 * x + 3.0 * y + 0.2 * y * y + 0.1 * x * y + 0.2 * Math.sqrt(Math.abs(x))
  ret += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0
  ret += ((20.0 * Math.sin(y * Math.PI) + 40.0 * Math.sin((y / 3.0) * Math.PI)) * 2.0) / 3.0
  ret += ((160.0 * Math.sin((y / 12.0) * Math.PI) + 320 * Math.sin((y * Math.PI) / 30.0)) * 2.0) / 3.0
  return ret
}

function transformLng(x, y) {
  let ret = 300.0 + x + 2.0 * y + 0.1 * x * x + 0.1 * x * y + 0.1 * Math.sqrt(Math.abs(x))
  ret += ((20.0 * Math.sin(6.0 * x * Math.PI) + 20.0 * Math.sin(2.0 * x * Math.PI)) * 2.0) / 3.0
  ret += ((20.0 * Math.sin(x * Math.PI) + 40.0 * Math.sin((x / 3.0) * Math.PI)) * 2.0) / 3.0
  ret += ((150.0 * Math.sin((x / 12.0) * Math.PI) + 300.0 * Math.sin((x / 30.0) * Math.PI)) * 2.0) / 3.0
  return ret
}

/** WGS-84 → GCJ-02。境外坐标原样返回。 */
export function wgs84ToGcj02(lng, lat) {
  if (outOfChina(lng, lat)) return [lng, lat]
  const dLatRaw = transformLat(lng - 105.0, lat - 35.0)
  const dLngRaw = transformLng(lng - 105.0, lat - 35.0)
  const radLat = (lat / 180.0) * Math.PI
  let magic = Math.sin(radLat)
  magic = 1 - GCJ02_EE * magic * magic
  const sqrtMagic = Math.sqrt(magic)
  const dLat = (dLatRaw * 180.0) / (((GCJ02_A * (1 - GCJ02_EE)) / (magic * sqrtMagic)) * Math.PI)
  const dLng = (dLngRaw * 180.0) / ((GCJ02_A / sqrtMagic) * Math.cos(radLat) * Math.PI)
  return [lng + dLng, lat + dLat]
}

/** GCJ-02 → WGS-84（迭代逼近，足够日常精度）。 */
export function gcj02ToWgs84(lng, lat) {
  if (outOfChina(lng, lat)) return [lng, lat]
  const [gcjLng, gcjLat] = [lng, lat]
  let wgsLng = lng
  let wgsLat = lat
  for (let i = 0; i < 6; i += 1) {
    const [testLng, testLat] = wgs84ToGcj02(wgsLng, wgsLat)
    wgsLng += gcjLng - testLng
    wgsLat += gcjLat - testLat
  }
  return [wgsLng, wgsLat]
}

/**
 * 按目标底图的坐标系转换一个 [lng, lat]。
 * crs 为 WGS84 时是**零成本直通**（当前两个 provider 都走这条路）。
 */
export function projectForCrs(lng, lat, crs) {
  if (crs === CRS_GCJ02) return wgs84ToGcj02(lng, lat)
  return [lng, lat]
}
