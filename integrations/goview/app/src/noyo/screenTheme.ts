// Screen design tokens. CSS variables are resolved before entering ECharts,
// whose canvas renderer cannot interpret var(). Both themes use the same keys.
import { shapeOf, type WidgetShape } from './widgets'
import tokenDefaults from './screenTokens.json'

export type ScreenTone = 'primary' | 'danger' | 'warning' | 'success' | 'info' | 'neutral'

type ScreenMode = 'light' | 'dark'
type TokenName = keyof typeof tokenDefaults
const paletteKeys = ['canvas', 'panel', 'panelBorder', 'grid', 'axis', 'axisLabel', 'text1', 'text2', 'text3'] as const
export type ScreenPalette = Record<typeof paletteKeys[number], string> & { series: string[] }

function resolveToken(name: TokenName, style?: CSSStyleDeclaration | null): string {
  return style?.getPropertyValue(name).trim() || tokenDefaults[name]
}

function paletteFromTokens(mode: ScreenMode, style?: CSSStyleDeclaration | null): ScreenPalette {
  const result = {} as ScreenPalette
  for (const key of paletteKeys) result[key] = resolveToken(`--noyo-screen-${mode}-${key}` as TokenName, style)
  result.series = Array.from({ length: 8 }, (_, index) => resolveToken(`--noyo-screen-${mode}-series-${index + 1}` as TokenName, style))
  return result
}

// Non-browser compilation uses the same registry that generates the CSS variables.
export const screenPalette = paletteFromTokens('dark')

/** Resolve the token family against the actual saved canvas, not editor chrome. */
export function paletteForCanvas(background?: string): ScreenPalette {
  let color = String(background || screenPalette.canvas)
  const style = typeof document === 'undefined' ? null : getComputedStyle(document.documentElement)
  const variable = /^var\((--[\w-]+)\)$/.exec(color)
  if (variable && style) color = style.getPropertyValue(variable[1]).trim() || color
  const hex = /^#([0-9a-f]{6})$/i.exec(color)
  const rgb = hex ? [0, 2, 4].map(at => parseInt(hex[1].slice(at, at + 2), 16)) : null
  const mode = rgb && rgb[0] * 0.2126 + rgb[1] * 0.7152 + rgb[2] * 0.0722 > 145 ? 'light' : 'dark'
  const resolved = paletteFromTokens(mode, style)
  if (hex) resolved.canvas = color
  return resolved
}

const toneColors: Record<ScreenTone, string> = {
  primary: resolveToken('--noyo-screen-dark-tone-primary'),
  danger: resolveToken('--noyo-screen-dark-tone-danger'),
  warning: resolveToken('--noyo-screen-dark-tone-warning'),
  success: resolveToken('--noyo-screen-dark-tone-success'),
  info: resolveToken('--noyo-screen-dark-tone-info'),
  neutral: resolveToken('--noyo-screen-dark-tone-neutral')
}

export function toneColor(tone: unknown, fallback: ScreenTone = 'primary', mode: ScreenMode = 'dark'): string {
  const key = String(tone || '') as ScreenTone
  const name = toneColors[key] ? key : fallback
  const style = typeof document === 'undefined' ? null : getComputedStyle(document.documentElement)
  return resolveToken(`--noyo-screen-${mode}-tone-${name}` as TokenName, style)
}

export type TextVariant = 'title' | 'label' | 'value' | 'metricLabel'

export interface ThemeOptions {
  tone?: unknown
  variant?: TextVariant
  /** Trend window in hours, used only to pick a sensible tooltip unit. */
  window?: string
  canvasBackground?: string
}

const isObject = (value: any): boolean => value !== null && typeof value === 'object' && !Array.isArray(value)

function axisList(axis: any): any[] {
  if (Array.isArray(axis)) return axis
  return isObject(axis) ? [axis] : []
}

// Grid lines belong on the value axis only; a category axis with dashed
// verticals turns a 24-point time series into a hatch pattern.
function themeAxis(axis: any, splitLine: boolean, screenPalette: ScreenPalette) {
  for (const item of axisList(axis)) {
    item.axisLabel = { ...(item.axisLabel || {}), color: screenPalette.axisLabel, fontSize: 11 }
    item.axisLine = { ...(item.axisLine || {}), show: true, lineStyle: { ...(item.axisLine?.lineStyle || {}), color: screenPalette.grid } }
    item.axisTick = { ...(item.axisTick || {}), show: false }
    if (item.splitLine || splitLine) {
      item.splitLine = { ...(item.splitLine || {}), show: splitLine, lineStyle: { ...(item.splitLine?.lineStyle || {}), color: screenPalette.grid, type: 'dashed', width: 1 } }
    }
    if (isObject(item.nameTextStyle)) item.nameTextStyle.color = screenPalette.text3
  }
}

// Series styling keeps the upstream `type` untouched and only neutralises the
// fixed colours (white labels and per-series fills). Dropping
// the literal lets the top-level palette own the colour, so a screen never
// mixes white labels with a dark panel.
function themeSeries(series: any[], screenPalette: ScreenPalette) {
  for (const item of series) {
    if (!isObject(item)) continue
    const type = String(item.type || '')
    if (type === 'line') {
      item.smooth = true
      item.symbol = 'none'
      item.lineStyle = { ...(item.lineStyle || {}), width: 2, color: null }
      // The upstream gradient stops are hard-coded per widget, which breaks a
      // shared palette. A plain translucent fill inherits the series colour.
      if (isObject(item.areaStyle)) item.areaStyle = { opacity: 0.14 }
      item.label = { ...(item.label || {}), show: false }
      item.itemStyle = { ...(item.itemStyle || {}), color: null }
    } else if (type === 'bar') {
      item.barWidth = item.barWidth && item.barWidth < 28 ? item.barWidth : 14
      item.itemStyle = { ...(item.itemStyle || {}), borderRadius: [3, 3, 0, 0], color: null }
      item.label = { ...(item.label || {}), show: true, position: 'top', color: screenPalette.text2, fontSize: 11 }
    } else if (type === 'pie') {
      item.label = { ...(item.label || {}), color: screenPalette.text2, fontSize: 11 }
      item.itemStyle = { ...(item.itemStyle || {}), borderColor: screenPalette.canvas, borderWidth: 2 }
      if (isObject(item.labelLine)) {
        item.labelLine = { ...item.labelLine, lineStyle: { ...(item.labelLine?.lineStyle || {}), color: screenPalette.axis } }
      }
    } else {
      item.label = { ...(item.label || {}), color: screenPalette.text2, fontSize: 11 }
    }
  }
}

function themeCartesian(option: any, screenPalette: ScreenPalette) {
  option.backgroundColor = 'transparent'
  option.color = [...screenPalette.series]
  option.textStyle = { ...(option.textStyle || {}), fontFamily: 'system-ui, sans-serif', color: screenPalette.text2 }
  option.grid = { ...(option.grid || {}), left: 56, right: 28, top: 44, bottom: 32, containLabel: true }
  option.legend = {
    ...(option.legend || {}),
    show: true,
    top: 6,
    right: 12,
    itemWidth: 8,
    itemHeight: 8,
    itemGap: 14,
    icon: 'circle',
    textStyle: { ...(option.legend?.textStyle || {}), color: screenPalette.text2, fontSize: 12 }
  }
  option.tooltip = {
    ...(option.tooltip || {}),
    backgroundColor: screenPalette.panel,
    borderColor: screenPalette.panelBorder,
    borderWidth: 1,
    padding: [8, 12],
    textStyle: { ...(option.tooltip?.textStyle || {}), color: screenPalette.text1, fontSize: 12 }
  }
  themeAxis(option.xAxis, false, screenPalette)
  themeAxis(option.yAxis, true, screenPalette)
  if (Array.isArray(option.series)) themeSeries(option.series, screenPalette)
  // Trend lines must not start at zero: a level that moves between 3.9 and 4.4
  // renders as a flat line when the axis is pinned to the origin. Bars keep the
  // zero baseline because their height is the measurement.
  const hasBar = Array.isArray(option.series) && option.series.some((item: any) => item?.type === 'bar')
  if (!hasBar) {
    for (const item of axisList(option.yAxis)) {
      if (!item.type || item.type === 'value') item.scale = true
    }
  }
}

function themePie(option: any, accent: string, screenPalette: ScreenPalette) {
  option.backgroundColor = 'transparent'
  option.color = [...screenPalette.series]
  option.tooltip = {
    ...(option.tooltip || {}),
    backgroundColor: screenPalette.panel,
    borderColor: screenPalette.panelBorder,
    borderWidth: 1,
    textStyle: { ...(option.tooltip?.textStyle || {}), color: screenPalette.text1, fontSize: 12 }
  }
  option.legend = {
    ...(option.legend || {}),
    show: true,
    bottom: 4,
    itemWidth: 8,
    itemHeight: 8,
    icon: 'circle',
    textStyle: { ...(option.legend?.textStyle || {}), color: screenPalette.text2, fontSize: 12 }
  }
  if (Array.isArray(option.series)) themeSeries(option.series, screenPalette)
  option.series?.[0] && (option.series[0].radius = option.series[0].radius || ['46%', '68%'])
  option.series?.[0] && (option.series[0].center = ['50%', '46%'])
  option.series?.[0] && (option.series[0].itemStyle = { ...(option.series[0].itemStyle || {}), borderColor: screenPalette.canvas })
  if (isObject(option.title?.textStyle)) option.title.textStyle.color = accent
}

const textVariants = (screenPalette: ScreenPalette): Record<TextVariant, { size: number; color: string; weight: string; spacing: number }> => ({
  title: { size: 15, color: screenPalette.text1, weight: '600', spacing: 0.5 },
  label: { size: 12, color: screenPalette.text3, weight: 'normal', spacing: 1 },
  value: { size: 13, color: screenPalette.text2, weight: 'normal', spacing: 0 },
  metricLabel: { size: 13, color: screenPalette.text2, weight: 'normal', spacing: 0 }
})

// Mutates a freshly constructed material in place. `option` already carries the
// upstream defaults, so series types and component-specific keys survive.
export function applyScreenTheme(component: any, locale: string, options: ThemeOptions = {}) {
  const key = component?.chartConfig?.key
  const shape: WidgetShape = shapeOf(key)
  const option = component?.option
  if (!option) return component
  const screenPalette = paletteForCanvas(options.canvasBackground)
  const dark = screenPalette.text1 === paletteForCanvas().text1
  const accent = toneColor(options.tone, 'primary', dark ? 'dark' : 'light')
  const english = String(locale || '').startsWith('en')

  if (key === 'Border06') {
    option.colors = [screenPalette.panelBorder, accent]
    option.backgroundColor = screenPalette.panel
  } else if (key === 'Decorates02') {
    option.colors = [screenPalette.panelBorder, accent]
    option.lineHeight = 2
  } else if (key === 'TimeCommon') {
    option.timeColor = screenPalette.text2
    option.colorShadow = accent
  } else if (shape === 'chart') themeCartesian(option, screenPalette)
  else if (shape === 'pie') themePie(option, accent, screenPalette)
  else if (shape === 'percent') {
    themePie(option, accent, screenPalette)
    const series = option.series?.[0]
    if (series?.data?.[0]?.itemStyle) series.data[0].itemStyle.color = accent
    if (Array.isArray(series?.color)) series.color = [`${accent}33`, 'transparent']
    if (isObject(option.title?.textStyle)) option.title.textStyle.color = accent
  } else if (shape === 'number') {
    option.numberColor = accent
    option.numberSize = 44
    option.prefixColor = screenPalette.text3
    option.suffixColor = screenPalette.text2
    option.dur = 0.8
    if (typeof option.precision !== 'number') option.precision = 0
  } else if (shape === 'progress') {
    option.color = accent
    option.railColor = screenPalette.grid
    option.indicatorTextColor = screenPalette.text1
    option.indicatorTextSize = 20
    option.type = option.type || 'line'
  } else if (shape === 'table-grid') {
    option.headerBGC = screenPalette.panelBorder
    option.oddRowBGC = 'transparent'
    option.evenRowBGC = screenPalette.panel
    option.header = english ? ['Time', 'Device', 'Alarm', 'Severity'] : ['时间', '设备', '告警', '级别']
    option.index = false
    option.color = screenPalette.text1
    option.headerHeight = 32
    option.rowNum = option.rowNum || 6
    option.align = ['left', 'left', 'left', 'center']
    option.columnWidth = []
  } else if (shape === 'table' || shape === 'table-basic') {
    option.headerBGC = screenPalette.panelBorder
    option.oddRowBGC = 'transparent'
    option.evenRowBGC = screenPalette.panel
  } else if (shape === 'text') {
    const variant = textVariants(screenPalette)[options.variant || 'value']
    option.fontColor = variant.color
    option.fontSize = variant.size
    option.fontWeight = variant.weight
    option.letterSpacing = variant.spacing
    option.backgroundColor = 'transparent'
    option.borderWidth = 0
    option.paddingX = 0
    option.paddingY = 0
    option.textAlign = options.variant === 'title' ? 'left' : 'center'
    if (options.variant === 'title') option.dataset = english ? option.dataset : option.dataset
  }

  return component
}

export { toneColors }
