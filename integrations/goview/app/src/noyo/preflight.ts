// Compile-time preflight for a generated document.
//
// Everything here is a defect the compiler or an upstream widget can introduce
// without anyone noticing until a screen is projected in an operations room: two
// widgets stacked on the same rectangle, a chart still holding its sample data,
// a CSS variable written into an ECharts option (which silently renders as an
// invalid colour), a caption wider than its own box.
//
// Errors block the proposal. Warnings are appended to the assistant reply so the
// reviewer knows what was downgraded instead of discovering it later.
import { shapeOf, targetOf, type WidgetShape } from './widgets'
import { paletteForCanvas } from './screenTheme'

export interface PreflightReport {
  errors: string[]
  warnings: string[]
}

export interface PreflightOptions {
  locale?: string
  /** Narrower canvases legitimately hold fewer components. */
  maxComponents?: number
}

const english = (locale?: string) => String(locale || '').startsWith('en')
const LABELS = {
  overflow: ['组件超出画布', 'widget exceeds the canvas'],
  overlap: ['组件重叠', 'widgets overlap'],
  bindingMissing: ['绑定缺失', 'binding is missing'],
  orphanTarget: ['绑定指向不存在的组件', 'binding targets a missing widget'],
  cssVar: ['图表颜色使用了 CSS 变量（ECharts 不解析）', 'chart colour uses a CSS variable, which ECharts cannot resolve'],
  demoData: ['图表未绑定数据，仍显示示例数据', 'chart is unbound and still shows sample data'],
  textOverflow: ['文字超出组件宽度', 'caption is wider than its widget'],
  density: ['组件数量偏多', 'component count is high'],
  tiny: ['组件尺寸过小', 'widget is too small to read'],
  emptyText: ['文字组件内容为空', 'text widget is empty'],
  contrast: ['文字与画布背景对比不足', 'text contrast against the canvas is too low']
}
function pick(key: keyof typeof LABELS, locale?: string) {
  return LABELS[key][english(locale) ? 1 : 0]
}

const CHART_SHAPES: WidgetShape[] = ['chart', 'pie', 'radar', 'heatmap', 'treemap', 'words', 'vchart']

function flatten(list: any[] = []): any[] {
  return list.flatMap(item => [item, ...(item?.isGroup && Array.isArray(item.groupList) ? flatten(item.groupList) : [])])
}

function optionHasCssVar(value: any, depth = 0): boolean {
  if (depth > 8 || value == null) return false
  if (typeof value === 'string') return value.includes('var(--')
  if (Array.isArray(value)) return value.some(item => optionHasCssVar(item, depth + 1))
  if (typeof value === 'object') return Object.values(value).some(item => optionHasCssVar(item, depth + 1))
  return false
}

// A dataset binding keeps the saved option's placeholder for the editor, so the
// sample-data check only applies to charts with no binding at all.
function hasSampleData(option: any): boolean {
  const dataset = option?.dataset
  if (dataset && Array.isArray(dataset.source) && dataset.source.length > 0) return true
  if (Array.isArray(option?.series)) {
    return option.series.some((item: any) => Array.isArray(item?.data) && item.data.length > 0)
  }
  return false
}

// CJK glyphs are roughly one em wide, Latin about 0.55em. The estimate only needs
// to catch gross overflows, not to measure precisely.
function estimatedWidth(text: string, fontSize: number) {
  let width = 0
  for (const char of String(text)) {
    width += /[\u3000-\u9fff\uff00-\uffef]/.test(char) ? fontSize : fontSize * 0.56
  }
  return width
}

function luminance(color: unknown): number | null {
  const match = /^#([0-9a-f]{6})$/i.exec(String(color || ''))
  if (!match) return null
  return [0, 2, 4].map(at => parseInt(match[1].slice(at, at + 2), 16) / 255)
    .map(value => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4)
    .reduce((sum, value, index) => sum + value * [0.2126, 0.7152, 0.0722][index], 0)
}

export function preflightDocument(document: any, options: PreflightOptions = {}): PreflightReport {
  const locale = options.locale
  const errors: string[] = []
  const warnings: string[] = []
  const content = document?.content || {}
  const components = Array.isArray(content.componentList) ? content.componentList : []
  const flat = flatten(components)
  const width = Number(content.editCanvasConfig?.width || document?.canvas?.width || 1920)
  const height = Number(content.editCanvasConfig?.height || document?.canvas?.height || 1080)
  const bindings = document?.bindings || {}
  const targets = Array.isArray(document?.targets) ? document.targets : []
  const bound = new Set(targets.map((target: any) => target.componentId))
  const ids = new Set(flat.map((item: any) => item?.id).filter(Boolean))

  for (const target of targets) {
    if (!ids.has(target.componentId)) errors.push(`${pick('orphanTarget', locale)}: ${target.componentId}`)
    if (!bindings[target.bindingId]) errors.push(`${pick('bindingMissing', locale)}: ${target.bindingId}`)
  }

  const boxes: { id: string; x: number; y: number; w: number; h: number }[] = []
  for (const component of flat) {
    if (component?.isGroup) continue
    const attr = component?.attr || {}
    const label = component?.chartConfig?.title || component?.id
    if (![attr.x, attr.y, attr.w, attr.h].every((value: any) => typeof value === 'number' && Number.isFinite(value))) {
      errors.push(`${pick('overflow', locale)}: ${label}`)
      continue
    }
    if (attr.x < 0 || attr.y < 0 || attr.w <= 0 || attr.h <= 0 || attr.x + attr.w > width || attr.y + attr.h > height) {
      errors.push(`${pick('overflow', locale)}: ${label}`)
    }
    // Only the known panel material is a background layer. Data widgets still
    // undergo the full overlap check; arbitrary static components are not exempt.
    if (component?.chartConfig?.key !== 'Border06') boxes.push({ id: String(component.id), x: attr.x, y: attr.y, w: attr.w, h: attr.h })

    const key = component?.chartConfig?.key
    const shape = shapeOf(key)
    const option = component?.option || {}

    if ((CHART_SHAPES.includes(shape) || shape === 'number' || shape === 'progress') && optionHasCssVar(option)) {
      errors.push(`${pick('cssVar', locale)}: ${label}`)
    }
    if (CHART_SHAPES.includes(shape) && !bound.has(component.id) && hasSampleData(option)) {
      warnings.push(`${pick('demoData', locale)}: ${label}`)
    }
    if (shape === 'text') {
      const foreground = luminance(option.fontColor)
      const background = luminance(paletteForCanvas(content.editCanvasConfig?.background).canvas)
      if (foreground !== null && background !== null && (Math.max(foreground, background) + 0.05) / (Math.min(foreground, background) + 0.05) < 4.5) errors.push(`${pick('contrast', locale)}: ${label}`)
      const text = String(option.dataset ?? '')
      if (!text.trim()) warnings.push(`${pick('emptyText', locale)}: ${label}`)
      else {
        const size = Number(option.fontSize) || 14
        if (estimatedWidth(text, size) > attr.w + 8) warnings.push(`${pick('textOverflow', locale)}: ${label}`)
      }
    }
    // Decorates02 renders a horizontal SVG rule, not readable content. Its 3px
    // line can use a thin strip; keep all geometry checks and content minima.
    const cramped = key === 'Decorates02' && !bound.has(component.id)
      ? attr.w < 80 || attr.h < 3
      : CHART_SHAPES.includes(shape)
        ? attr.w < 180 || attr.h < 120
        : shape === 'text'
          ? attr.w < 80 || attr.h < 20
          : attr.w < 80 || attr.h < 40
    if (cramped) warnings.push(`${pick('tiny', locale)}: ${label}`)
  }

  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i], b = boxes[j]
      const overlapW = Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x)
      const overlapH = Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y)
      if (overlapW > 2 && overlapH > 2) errors.push(`${pick('overlap', locale)}: ${a.id} / ${b.id}`)
    }
  }

  const limit = Number(options.maxComponents) || Math.max(24, Math.round(96 * width * height / (1920 * 1080)))
  if (flat.length > limit) warnings.push(`${pick('density', locale)}: ${flat.length}/${limit}`)

  return { errors, warnings }
}

// Exported for the tests: the same reference the compiler uses to decide whether a
// widget may even carry a binding.
export function bindingTargetOf(key: string | undefined) {
  return targetOf(key)
}
