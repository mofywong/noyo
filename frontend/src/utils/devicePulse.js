/**
 * 设备圆点的「状态脉冲高亮」。
 *
 * 解决的问题：密集点位里，圆点只有 12–14px，状态色在大面积底图上会被稀释，
 * 用户扫视一眼分辨不出「哪些是告警」。脉冲动画是**前注意（pre-attentive）**通道，
 * 不需要聚焦就能把视线拉过去 —— 这是颜色本身做不到的。
 *
 * 三条硬约束（都来自既有实现，改动前先读）：
 *
 * 1. **不动圆点本身**。`deviceDotCanvas.js` 用「颜色 + 填充方式 + 尺寸」三重编码状态，
 *    其中尺寸（告警 +28%）是纯色觉之外的第二通道（WCAG 1.4.1）。脉冲若去缩放 icon-size，
 *    这层语义就跟动画混在一起，"告警更大"变成"告警正在放大"，不可读。
 *    所以脉冲只画在**圆点下方的光晕**上，核心圆点保持稳定。
 *
 * 2. **不用 icon-size 做动画**。symbol 图层的 icon-size 是 **layout** 属性，
 *    逐帧改它会让 MapLibre 每帧重新做碰撞检测与布局，几十个点就掉帧；
 *    circle 图层的 radius / opacity 是 **paint** 属性，逐帧更新只重算 uniform，
 *    是官方示例里做脉冲的常规做法。
 *
 * 3. **相位统一**。MapLibre 没有「全局动画变量」这种输入，paint 表达式里也做不了
 *    三角函数，逐 feature 错相只能靠每帧 setFeatureState —— 几千个设备时是不可接受的开销。
 *    统一相位反而更像"心跳节拍"，视觉上更整齐。
 *
 * ---- 2026-10-19 改版：从"呼吸"改为"脉冲" ----
 *
 * 旧实现在真机上被判定为「几乎看不出来」，复盘出三个原因，都在这里修掉：
 *
 * a) **太淡**：峰值不透明度只有 0.34，而 circle-blur 0.45 又把边缘糊开，
 *    实际上落到像素上的色量不足 0.2 —— 在天地图的浅色路网上等同不存在。
 *    现在峰值提到 0.62、blur 收到 0.18，并且**颜色本身提亮加饱和**（glowColor）。
 * b) **只有一层**：单层实心圆扩散时只是"变淡"，没有方向感。现在用双层：
 *    内层柔光负责"亮"，外层描边环负责"扩"，两者叠在一起才是"闪一下"。
 * c) **曲线太软**：对称余弦的端点速度趋零，看起来像"缓慢起伏"。
 *    现在改成非对称包络：16% 周期内冲到峰值（easeOutCubic 起手极快），
 *    剩下 84% 指数式回落，节拍更像心跳/雷达扫描。
 */

import { DEVICE_STATUS_ORDER } from './deviceMap.js'

const STORAGE_KEY = 'noyo.map.pulse'

/**
 * 一次完整脉冲的时长。
 * 1800ms 是"呼吸"的节拍，对"闪"来说太慢；1500ms 在密集点位里仍不显焦躁。
 */
export const PULSE_PERIOD_MS = 1500

/** 逐帧重绘的最小间隔：30fps。半径变化只有几像素，60fps 是纯浪费。 */
export const PULSE_FRAME_INTERVAL_MS = 33

/**
 * 起手爆发占周期的比例。前 16% 冲到峰值，剩下 84% 回落 ——
 * 这个不对称比值就是"闪"与"呼吸"的分界：越接近 0.5 越像呼吸。
 */
const PULSE_ATTACK_RATIO = 0.16

/**
 * 包络最低点。刻意不归零：归零会在每个周期边界出现一次"硬灭"，
 * 看似更闪，实则变成刺眼的频闪（EC 2.3.1 闪烁风险）。
 */
const PULSE_INTENSITY_FLOOR = 0.1

/**
 * 内层柔光：紧贴圆点向外出，负责"亮"。
 *
 * 下限必须**大于圆点本体**：`device-point` 的 icon-size 在 zoom 3 时是 0.8，
 * 核心半径 7.2 的圆点实际只画到 5.76px；如果光晕下限也取 9，乘以 0.8 后
 * 恰好被圆点整个盖住 —— 峰值那一瞬等于什么都没发生。取 12 留出 3px 以上余量。
 */
const CORE_RADIUS_MIN = 12
const CORE_RADIUS_MAX = 18
const CORE_OPACITY_MAX = 0.62
const CORE_OPACITY_MIN = 0.12

/** 外层扩散波：从圆点推出去的一圈描边，负责"扩"。 */
const WAVE_RADIUS_MIN = 15
const WAVE_RADIUS_MAX = 32
const WAVE_OPACITY_MAX = 0.9
const WAVE_OPACITY_MIN = 0

/** 静止态（减少动效 / 关掉动画时）用的固定强度：取偏亮的一档，不是"半亮"。 */
export const PULSE_STILL_INTENSITY = 0.55

/**
 * 与 `device-point` 的 `icon-size` 曲线保持一致：
 * `['interpolate', ['linear'], ['zoom'], 3, 0.8, 18, 1]`。
 * 不一致的话，低缩放下光晕会明显大于/小于圆点，看起来像两个不相关的东西。
 */
function zoomIconScale(zoom) {
  const minZoom = 3
  const maxZoom = 18
  const minScale = 0.8
  const maxScale = 1
  if (!Number.isFinite(zoom)) return maxScale
  const ratio = Math.min(1, Math.max(0, (zoom - minZoom) / (maxZoom - minZoom)))
  return minScale + (maxScale - minScale) * ratio
}

/**
 * 脉冲相位 → 强度 [FLOOR, 1]。1 = 最亮最紧，FLOOR = 最淡最扩。
 *
 * 两端都取 FLOOR，循环无缝；峰值只在每个周期开头出现一次。
 */
export function pulseIntensity(elapsedMs) {
  const phase = ((elapsedMs % PULSE_PERIOD_MS) + PULSE_PERIOD_MS) % PULSE_PERIOD_MS
  const progress = phase / PULSE_PERIOD_MS
  const floor = PULSE_INTENSITY_FLOOR
  if (progress < PULSE_ATTACK_RATIO) {
    const t = progress / PULSE_ATTACK_RATIO
    // easeOutCubic：起手第一帧就冲上去，视觉上是"啪"地亮起来。
    return floor + (1 - floor) * (1 - Math.pow(1 - t, 3))
  }
  const t = (progress - PULSE_ATTACK_RATIO) / (1 - PULSE_ATTACK_RATIO)
  // 指数式衰减：前段掉得快，后段几乎停住 —— 余辉感来自这一段。
  return floor + (1 - floor) * Math.pow(1 - t, 2.4)
}

/**
 * @param {number} intensity 1 = 最亮最紧，0 = 最淡最扩
 * @param {number} zoom 当前地图缩放级
 * @returns {{coreRadius:number, coreOpacity:number, waveRadius:number, waveOpacity:number}}
 */
export function pulseGeometry(intensity, zoom) {
  const clamped = Math.min(1, Math.max(0, intensity))
  const scale = zoomIconScale(zoom)
  // span：0 = 贴着圆点，1 = 扩到底。
  const span = 1 - clamped
  return {
    coreRadius: (CORE_RADIUS_MIN + (CORE_RADIUS_MAX - CORE_RADIUS_MIN) * span) * scale,
    coreOpacity: CORE_OPACITY_MAX - (CORE_OPACITY_MAX - CORE_OPACITY_MIN) * span,
    waveRadius: (WAVE_RADIUS_MIN + (WAVE_RADIUS_MAX - WAVE_RADIUS_MIN) * span) * scale,
    // 波的可见度与强度非线性挂钩：峰值时最实，扩散到一半就基本消失，
    // 否则会出现"一圈灰环停在原地"的观感。
    waveOpacity: WAVE_OPACITY_MAX * Math.pow(clamped, 1.6) + WAVE_OPACITY_MIN,
  }
}

function clampChannel(value) {
  return Math.min(255, Math.max(0, Math.round(value)))
}

function parseColor(value) {
  const text = String(value || '').trim()
  if (!text) return null
  if (text.startsWith('#')) {
    const hex = text.slice(1)
    const full = hex.length === 3 ? hex.split('').map((char) => char + char).join('') : hex
    if (full.length !== 6) return null
    const r = parseInt(full.slice(0, 2), 16)
    const g = parseInt(full.slice(2, 4), 16)
    const b = parseInt(full.slice(4, 6), 16)
    if ([r, g, b].some((channel) => Number.isNaN(channel))) return null
    return { r, g, b }
  }
  const match = text.match(/rgba?\(([^)]+)\)/i)
  if (!match) return null
  const parts = match[1].split(',').map((part) => parseFloat(part))
  if (parts.length < 3 || parts.slice(0, 3).some((channel) => Number.isNaN(channel))) return null
  return { r: parts[0], g: parts[1], b: parts[2] }
}

/**
 * 光晕色 = 状态色「提亮 + 加饱和」。
 *
 * 为什么不直接用 `deviceStatusColor()`：那是**圆点本体**的填充色，取值偏沉稳，
 * 目的是让圆点看起来是"实体"。光晕是纯发光，需要更高的亮度与饱和度才能在
 * 浅色路网 / 深色影像上同样扎眼。
 *
 * 这两者刻意分开，不复用 `deviceDotCanvas.js` 里的 `lighten()`：
 * 那里的混白幅度只有 18%，是为了做顶部受光渐变，强度不足以做光晕。
 */
export function glowColor(color) {
  const rgb = parseColor(color)
  if (!rgb) return color

  const r = rgb.r / 255
  const g = rgb.g / 255
  const b = rgb.b / 255
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  let h = 0
  let s = 0
  const l = (max + min) / 2

  if (max !== min) {
    const delta = max - min
    s = l > 0.5 ? delta / (2 - max - min) : delta / (max + min)
    if (max === r) h = ((g - b) / delta + (g < b ? 6 : 0)) / 6
    else if (max === g) h = ((b - r) / delta + 2) / 6
    else h = ((r - g) / delta + 4) / 6
  }

  // 饱和度 ×1.10（上限 100%），亮度 +6 个百分点（上限 68%）。
  //
  // 这两个幅度是量出来的，不是拍的：再往上（试过 +12 / ×1.18）在线绿会变成
  // rgb(12,234,94) 那种荧光色，一是跟圆点本体的沉稳绿差异过大、看起来像另一个
  // 状态，二是在浅色路网底图上亮度逼近底色，对比度反而下降。
  // 光晕的"亮"主要靠不透明度（0.34 → 0.62），颜色只做小幅提亮。
  const nextS = Math.min(1, s * 1.1)
  const nextL = Math.min(0.68, l + 0.06)

  // HSL -> RGB（标准公式，s/l 已归一化）
  const hueToRgb = (p, q, t) => {
    if (t < 0) t += 1
    if (t > 1) t -= 1
    if (t < 1 / 6) return p + (q - p) * 6 * t
    if (t < 1 / 2) return q
    if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6
    return p
  }
  const q = nextL < 0.5 ? nextL * (1 + nextS) : nextL + nextS - nextL * nextS
  const p = 2 * nextL - q
  const outR = clampChannel(255 * hueToRgb(p, q, h + 1 / 3))
  const outG = clampChannel(255 * hueToRgb(p, q, h))
  const outB = clampChannel(255 * hueToRgb(p, q, h - 1 / 3))
  return `rgb(${outR}, ${outG}, ${outB})`
}

/** 把任意输入归一成「按 DEVICE_STATUS_ORDER 排序、去重、只含合法状态」的数组。 */
export function normalizePulseStatuses(value) {
  const list = Array.isArray(value) ? value : []
  return DEVICE_STATUS_ORDER.filter((status) => list.includes(status))
}

export function loadPulseStatuses() {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    // 无存储 = 首次进入 ⇒ **默认四种状态全选**（产品决定：进页面就有脉冲提示，
    // 不必先找到按钮才知道有这个能力）。
    // 注意必须区分「没有存储」与「存储为空数组」：后者是用户主动清空的选择，
    // 不能被默认值覆盖，否则清空后刷新又全都亮起来。
    if (raw == null || raw === '') return [...DEVICE_STATUS_ORDER]
    return normalizePulseStatuses(JSON.parse(raw))
  } catch (error) {
    return [...DEVICE_STATUS_ORDER]
  }
}

export function savePulseStatuses(statuses) {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(normalizePulseStatuses(statuses)))
  } catch (error) {
    /* localStorage 不可用时静默降级：本次会话仍然生效 */
  }
}

/**
 * 构造脉冲图层的 filter。
 *
 * 空数组故意返回一个**永假**条件而不是 `['boolean', true]`：
 * 图层本身留在样式里（避免反复 addLayer/removeLayer 引发 styledata 抖动），
 * 只是没有任何 feature 命中，渲染开销为零。
 */
export function buildPulseFilter(statuses) {
  const list = normalizePulseStatuses(statuses)
  const base = ['!', ['has', 'point_count']]
  if (!list.length) return ['all', base, ['==', ['get', 'status'], '__none__']]
  const anyStatus = ['any', ...list.map((status) => ['==', ['get', 'status'], status])]
  return ['all', base, anyStatus]
}

/** 系统级「减少动态效果」。为 true 时退化为静态光环，不启动逐帧循环。 */
export function prefersReducedMotion() {
  try {
    return typeof window !== 'undefined' && typeof window.matchMedia === 'function'
      ? window.matchMedia('(prefers-reduced-motion: reduce)').matches
      : false
  } catch (error) {
    return false
  }
}
