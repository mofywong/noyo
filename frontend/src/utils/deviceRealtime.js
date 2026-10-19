/**
 * 设备实时属性快照：地图悬停卡与设备列表共用同一套取数与装配规则。
 *
 * 为什么不复用列表页里那段内联代码：那里的实现和 `products` 列表、`hoveredDevice`
 * 这些列表页专有状态缠在一起，直接搬过来会让地图页被迫加载整张产品表。
 * 这里把「取什么 + 怎么拼」抽干净，页面只负责什么时候取、取到之后摆哪儿。
 *
 * 两条硬约束：
 *
 * 1. **物模型是产品级的，必须按 product_code 缓存**。地图上一屏几十台设备往往
 *    只属于三五个产品，每台都拉一次 `/products/:code` 是纯浪费。
 * 2. **数值缺失 ≠ 值为 0**。`hasValue` 为假的属性照样要列出来（浅色显示 `--`），
 *    否则用户会误以为"这个点位不存在"，而实际是"设备没上报过"。
 */
import axios from 'axios'

/** 产品物模型缓存：产品定义几乎不变，5 分钟足够，也避免切页面回来又拉一遍。 */
const PRODUCT_TTL_MS = 5 * 60 * 1000
const productCache = new Map() // product_code -> { properties, timestamp }
const productInflight = new Map() // product_code -> Promise

/** 实时数据缓存：与列表页保持一致的 5 秒节流窗口。 */
const DATA_TTL_MS = 5 * 1000
const dataCache = new Map() // device_code -> { data, updatedAt, timestamp }

/** 从产品 config（JSON 字符串）里取 properties 定义。解析失败按"没有物模型"处理。 */
function extractTslProperties(product) {
  const raw = product?.config
  if (!raw) return []
  try {
    const config = typeof raw === 'string' ? JSON.parse(raw) : raw
    const properties = config?.tsl?.properties
    return Array.isArray(properties) ? properties : []
  } catch (error) {
    return []
  }
}

/**
 * 取产品的属性定义。失败返回空数组 —— 调用方会退化成"只显示上报过的键"。
 */
export async function loadProductProperties(productCode) {
  if (!productCode) return []
  const cached = productCache.get(productCode)
  if (cached && Date.now() - cached.timestamp < PRODUCT_TTL_MS) return cached.properties

  if (productInflight.has(productCode)) return productInflight.get(productCode)

  const task = (async () => {
    try {
      const response = await axios.get(`/api/products/${encodeURIComponent(productCode)}`)
      if (response?.data?.code !== 0) return []
      const properties = extractTslProperties(response.data.data)
      productCache.set(productCode, { properties, timestamp: Date.now() })
      return properties
    } catch (error) {
      // 无 product:list 权限、产品已删除、网络失败 —— 一律降级为"没有物模型"。
      // 这里绝不能抛：属性显示不出来不该让悬停卡整个失效。
      return []
    } finally {
      productInflight.delete(productCode)
    }
  })()

  productInflight.set(productCode, task)
  return task
}

/**
 * 取设备最新上报值。
 *
 * @returns {Promise<{data: object, updatedAt: Date|null, error: string|null}>}
 *   error 只区分两类：`forbidden` 与 `failed`，界面据此选文案（别把后端 message 甩给用户）。
 */
export async function loadDeviceSnapshot(deviceCode) {
  if (!deviceCode) return { data: {}, updatedAt: null, error: null }
  const cached = dataCache.get(deviceCode)
  if (cached && Date.now() - cached.timestamp < DATA_TTL_MS) {
    return { data: cached.data, updatedAt: cached.updatedAt, error: null }
  }

  try {
    const response = await axios.get(`/api/devices/${encodeURIComponent(deviceCode)}/data`)
    const code = response?.data?.code
    if (code !== 0) {
      return { data: {}, updatedAt: null, error: code === 403 ? 'forbidden' : 'failed' }
    }
    const data = response.data.data || {}
    const updatedAt = new Date()
    dataCache.set(deviceCode, { data, updatedAt, timestamp: Date.now() })
    return { data, updatedAt, error: null }
  } catch (error) {
    const status = error?.response?.status
    return { data: {}, updatedAt: null, error: status === 403 ? 'forbidden' : 'failed' }
  }
}

/** 物模型里不参与展示的元字段（与后端 `handleGetDeviceData` 过滤的一致）。 */
const META_KEYS = new Set(['ts', '_type', 'raw', 'error'])

/**
 * 把「物模型定义」和「最新值」拼成展示列表。
 *
 * 顺序规则：有值的在前，同组按 key 字典序 —— 悬停卡空间有限，
 * 用户最想看的是"现在是什么值"，而不是"这个产品定义了哪些点位"。
 *
 * @param {Array} tslProperties 物模型 properties（可空）
 * @param {object|null} data 最新上报值
 * @param {object} [options]
 * @param {boolean} [options.onlyReported] 为真时只返回有值的项（地图卡默认不看空点位）
 */
export function assembleDeviceProperties(tslProperties, data, { onlyReported = false } = {}) {
  const list = []
  const reported = new Set()
  if (data && typeof data === 'object') {
    Object.keys(data).forEach((key) => {
      if (!META_KEYS.has(key)) reported.add(key)
    })
  }

  for (const property of tslProperties || []) {
    const key = property?.identifier
    if (!key) continue
    const hasValue = reported.has(key)
    reported.delete(key)
    if (onlyReported && !hasValue) continue
    list.push({
      key,
      name: property.name || key,
      value: hasValue ? data[key] : undefined,
      unit: property.dataType?.specs?.unit || '',
      type: property.dataType?.type || '',
      hasValue,
    })
  }

  // 物模型没定义、但设备确实上报了的点位：照实列出，名称直接用 key。
  reported.forEach((key) => {
    list.push({ key, name: key, value: data[key], unit: '', type: typeof data[key], hasValue: true })
  })

  list.sort((a, b) => {
    if (a.hasValue !== b.hasValue) return a.hasValue ? -1 : 1
    return a.key.localeCompare(b.key)
  })

  return list
}

/** 属性值格式化：布尔转 TRUE/FALSE，小数保留两位，对象退化为 JSON。 */
export function formatPropertyValue(value) {
  if (value === undefined || value === null || value === '') return '--'
  if (typeof value === 'boolean') return value ? 'TRUE' : 'FALSE'
  if (typeof value === 'number') {
    if (Number.isInteger(value)) return String(value)
    return String(Number(value.toFixed(2)))
  }
  if (typeof value === 'object') {
    try {
      return JSON.stringify(value)
    } catch (error) {
      return String(value)
    }
  }
  return String(value)
}

/** 测试/调试用：清空两级缓存。 */
export function clearDeviceRealtimeCache() {
  productCache.clear()
  dataCache.clear()
}
