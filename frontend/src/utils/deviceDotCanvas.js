/**
 * 设备地图圆点标记的纯绘制层。
 *
 * 只接受「调色板 + 状态」两个输入，不读 DOM、不读主题 —— 同一份几何既能被地图
 * （deviceMapMarkers.js）消费，也能在设计预览页里逐像素核对。
 *
 * 为什么用圆点而不是图钉：
 * - 圆点**以坐标为中心**（icon-anchor: center），不像图钉要靠针尖偏移去对位，
 *   少一层几何耦合，密集点位之间也不会被长尾巴切割；
 * - 设备地图看的是"分布 + 状态"，不是"某一个具体位置"，圆点在小尺寸下更稳，
 *   缩到 12px 仍能分辨，图钉在这个尺寸会糊成一团；
 * - 业内（Grafana Geomap、Datadog、ThingsBoard、Google Maps POI）在密集点位场景
 *   一律收成点，把形状留给需要指位的 POI。
 *
 * 状态同时用**颜色 + 填充方式 + 尺寸**三重编码：
 * 在线/预警＝实心常规尺寸，告警＝实心放大（+17%），离线＝空心圆环。
 * 只靠颜色区分状态对色觉障碍用户不可用（WCAG 1.4.1），给三档差异成本几乎为零。
 */

/** 画布尺寸（逻辑像素）。@2x 超采样后以 pixelRatio: 2 注册给 MapLibre。 */
export const DEVICE_DOT_SIZE = { width: 48, height: 48 }

/** 圆点中心 = 图片中心；symbol 图层用 icon-anchor: center 直接对到经纬度。 */
export const DEVICE_DOT_CENTER = { x: 24, y: 24 }

export const DEVICE_DOT_PIXEL_RATIO = 2

const CX = DEVICE_DOT_CENTER.x
const CY = DEVICE_DOT_CENTER.y

/** 核心半径：告警刻意放大一档（+28%），作为"严重程度"的非颜色通道。 */
const CORE_RADIUS = { online: 7.2, warning: 7.2, alarm: 9.2, offline: 7.2 }

/** 把任意 CSS 颜色改成指定透明度（令牌可能是 hex 或 rgba）。 */
function withAlpha(color, alpha) {
  const value = String(color || '').trim()
  if (value.startsWith('#')) {
    const hex = value.slice(1)
    const full = hex.length === 3 ? hex.split('').map((char) => char + char).join('') : hex
    const r = parseInt(full.slice(0, 2), 16)
    const g = parseInt(full.slice(2, 4), 16)
    const b = parseInt(full.slice(4, 6), 16)
    if ([r, g, b].some((channel) => Number.isNaN(channel))) return value
    return `rgba(${r}, ${g}, ${b}, ${alpha})`
  }
  const match = value.match(/rgba?\(([^)]+)\)/i)
  if (match) {
    const [r, g, b] = match[1].split(',').map((part) => parseFloat(part))
    return `rgba(${r}, ${g}, ${b}, ${alpha})`
  }
  return value
}

/** 把 hex 向白色混合，用于圆点顶部的微弱受光渐变（非 hex 原样返回）。 */
function lighten(color, amount) {
  const value = String(color || '').trim()
  if (!value.startsWith('#')) return value
  const hex = value.slice(1)
  const full = hex.length === 3 ? hex.split('').map((char) => char + char).join('') : hex
  const channels = [0, 1, 2].map((index) => parseInt(full.slice(index * 2, index * 2 + 2), 16))
  if (channels.some((channel) => Number.isNaN(channel))) return value
  const mixed = channels.map((channel) => Math.round(channel + (255 - channel) * amount))
  return `rgb(${mixed[0]}, ${mixed[1]}, ${mixed[2]})`
}

/**
 * @param {CanvasRenderingContext2D} ctx 逻辑坐标系（调用方负责 pixelRatio 缩放）
 * @param {object} palette
 * @param {string} palette.statusColor 状态主色
 * @param {string} palette.surface 咬边/空心填充色（浅色下为白，深色下为深灰）
 * @param {string} palette.highlight 顶部受光色（深色主题下几乎不可见，正好）
 * @param {string} palette.shadowColor 阴影基色（带透明度）
 * @param {string} palette.brand 选中态品牌色
 * @param {object} state
 * @param {string} state.status online | warning | alarm | offline
 * @param {boolean} state.selected
 */
export function drawDeviceDot(ctx, palette, state = {}) {
  const { status = 'online', selected = false } = state
  const { statusColor, surface, highlight, shadowColor, brand } = palette
  const radius = CORE_RADIUS[status] || CORE_RADIUS.online

  ctx.save()
  ctx.lineJoin = 'round'
  ctx.lineCap = 'round'

  // 1) 落地阴影：压扁的柔和投影，让圆点贴在地图平面上而不是浮着。
  //    刻意用中性阴影而非状态色 —— 几十个状态色光晕会把底图染花。
  ctx.save()
  ctx.translate(CX, CY + 1.2)
  ctx.scale(1, 0.62)
  const shadow = ctx.createRadialGradient(0, 0, radius * 0.5, 0, 0, radius + 4.6)
  shadow.addColorStop(0, withAlpha(shadowColor, selected ? 0.4 : 0.28))
  shadow.addColorStop(1, withAlpha(shadowColor, 0))
  ctx.fillStyle = shadow
  ctx.beginPath()
  ctx.arc(0, 0, radius + 4.6, 0, Math.PI * 2)
  ctx.fill()
  ctx.restore()

  // 2) 选中态：品牌色细环 + 极淡外发光。环放在核心之外，不改变圆点本身的读数。
  if (selected) {
    const ringRadius = radius + 4.2
    const glow = ctx.createRadialGradient(CX, CY, ringRadius, CX, CY, ringRadius + 6)
    glow.addColorStop(0, withAlpha(brand, 0.22))
    glow.addColorStop(1, withAlpha(brand, 0))
    ctx.fillStyle = glow
    ctx.beginPath()
    ctx.arc(CX, CY, ringRadius + 6, 0, Math.PI * 2)
    ctx.fill()

    ctx.beginPath()
    ctx.arc(CX, CY, ringRadius, 0, Math.PI * 2)
    ctx.strokeStyle = brand
    ctx.lineWidth = 2
    ctx.stroke()
  }

  // 3) 核心。
  if (status === 'offline') {
    // 空心：离线 = 没有数据流，用"空"这个形状本身表达，不只靠灰色。
    ctx.beginPath()
    ctx.arc(CX, CY, radius - 1.1, 0, Math.PI * 2)
    ctx.fillStyle = surface
    ctx.fill()
    ctx.strokeStyle = statusColor
    ctx.lineWidth = 2.2
    ctx.stroke()
  } else {
    const body = ctx.createLinearGradient(CX, CY - radius, CX, CY + radius)
    body.addColorStop(0, lighten(statusColor, 0.18))
    body.addColorStop(1, statusColor)
    ctx.beginPath()
    ctx.arc(CX, CY, radius, 0, Math.PI * 2)
    ctx.fillStyle = body
    ctx.fill()
    // 咬边：2px 表面色，任何底图上都能把圆点从背景里切出来（天地图高速是紫色粗线）。
    ctx.strokeStyle = surface
    ctx.lineWidth = 2
    ctx.stroke()
  }

  // 4) 顶部受光弧：1px，只做一点点"实体感"，不制造高光噪点。
  ctx.beginPath()
  ctx.arc(CX, CY, radius - 2.6, Math.PI * 1.16, Math.PI * 1.62, false)
  ctx.strokeStyle = status === 'offline' ? surface : highlight
  ctx.globalAlpha = status === 'offline' ? 0.9 : 0.55
  ctx.lineWidth = 1
  ctx.stroke()
  ctx.globalAlpha = 1

  ctx.restore()
}
