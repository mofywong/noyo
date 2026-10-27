// Deterministic layout compiler.
//
// The AI returns a semantic blueprint (what to show, how it relates) and never
// pixel geometry. This module turns that blueprint into exact rectangles on a
// 12-column grid, picks the widget for every slot, and derives the data binding
// from the item shape instead of trusting the model to get `kind`/`target` right.
//
// Invariants enforced here (and covered by tooling/layout.test.cjs):
//   - no two emitted rectangles overlap
//   - every rectangle stays inside the canvas with 24px page padding
//   - rows always fill the width, so no unpainted gutter column appears
//   - every binding kind is legal for the chosen widget shape
import { kindsOf } from './widgets'
import { selectPresentation, type PresentationTask } from './presentationMatcher'
import { applyScreenTheme, type ThemeOptions } from './screenTheme'

export const GRID = { columns: 12, padding: 24, gutter: 24 }

export type SectionRole = 'header' | 'kpi' | 'trend' | 'distribution' | 'statusWall' | 'alarmTable' | 'eventTable' | 'list' | 'map'
export type Composition = 'overview' | 'equipment' | 'compact'

export interface BlueprintItem {
  task?: PresentationTask
  priority?: number
  categoryCount?: number
  binding?: Record<string, any>
  placeholder?: boolean
  label?: string
  device?: string
  devices?: string[]
  key?: string
  keys?: string[]
  unit?: string
  precision?: number
  tone?: string
  form?: string
  viz?: string
  kind?: string
  aggregate?: 'sum' | 'avg' | 'min' | 'max' | 'count' | 'total' | 'online' | 'offline' | 'unknown' | 'onlineRate'
}

export interface BlueprintSection {
  task?: PresentationTask
  binding?: Record<string, any>
  role?: string
  title?: string
  subtitle?: string
  span?: number
  viz?: string
  window?: string
  items?: BlueprintItem[]
  devices?: string[]
  question?: string
  priority?: number
}

export interface LayoutOperation {
  type: 'add'
  id: string
  key: string
  x: number
  y: number
  w: number
  h: number
  title?: string
  option?: Record<string, any>
  binding?: Record<string, any> | null
  theme?: ThemeOptions & { option?: Record<string, any> }
}

export interface LayoutResult {
  operations: LayoutOperation[]
  notes: string[]
  composition: Composition
}

const HEADER_H = 92
const KPI_H = 136
const TITLE_H = 32
/** Preferred height for a chart/table band. */
const MIN_BODY_H = 96
/** Hard floor: below this a band cannot hold even a caption plus a chart. */
const MIN_BODY_FLOOR = 64
/** One readable "device name / state" line inside a status wall. */
const MIN_STATUS_ROW = 28
/** One readable device card in a grid-shaped status wall. */
const MIN_STATUS_CELL = 56
/** A stacked trend/distribution slice is unreadable below this height. */
const MIN_SLICE_H = 64

const ROLE_WEIGHT: Record<SectionRole, number> = {
  header: 0,
  kpi: 0,
  trend: 3,
  map: 3,
  distribution: 2,
  alarmTable: 2,
  eventTable: 2,
  list: 2,
  statusWall: 1
}

const ROLE_ALIASES: Record<string, SectionRole> = {
  header: 'header', title: 'header', banner: 'header',
  kpi: 'kpi', metric: 'kpi', metrics: 'kpi', stat: 'kpi', stats: 'kpi', summary: 'kpi',
  trend: 'trend', chart: 'trend', series: 'trend', timeseries: 'trend', line: 'trend',
  distribution: 'distribution', pie: 'distribution', breakdown: 'distribution', composition: 'distribution',
  status: 'statusWall', statuswall: 'statusWall', connectivity: 'statusWall', health: 'statusWall',
  alarmtable: 'alarmTable', alarms: 'alarmTable', alarm: 'alarmTable', table: 'alarmTable',
  eventtable: 'eventTable', event: 'eventTable', events: 'eventTable',
  list: 'list', ranking: 'list', top: 'list',
  map: 'map', geo: 'map', location: 'map'
}

// The AI writes role names in whatever case it likes; normalise before lookup.
function roleOf(section: BlueprintSection): SectionRole | null {
  const raw = String(section?.role || '').toLowerCase().replace(/[^a-z]/g, '')
  return ROLE_ALIASES[raw] || null
}

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

// TextCommon preserves newlines. Wrap unpredictable device names explicitly so
// the layout can reserve real line height instead of clipping a fixed prefix.
function statusLabel(text: string, width: number) {
  const lines: string[] = []
  for (const paragraph of text.split(/\r?\n/)) {
    let line = ''
    let used = 0
    for (const char of paragraph) {
      const size = /[\u3000-\u9fff\uff00-\uffef]/.test(char) ? 13 : 13 * 0.6
      if (line && used + size > width - 8) {
        lines.push(line)
        line = ''
        used = 0
      }
      line += char
      used += size
    }
    lines.push(line)
  }
  return { text: lines.join('\n'), height: lines.length * 24 }
}

// Budget static copy before mounting the renderer. Explicit wrapping is stable
// across CJK/Latin labels; excess copy gets an ellipsis and its full title stays
// available in the component metadata/tooltip. Bound readings are never cut.
export function fitStaticText(text: string, width: number, height: number, fontSize: number, spacing = 0, paddingX = 0, paddingY = 0) {
  const available = Math.max(1, width - paddingX * 2 - 8)
  const maxLines = Math.max(1, Math.floor((height - paddingY * 2) / (fontSize * 1.6)))
  const lines: string[] = []
  let line = '', used = 0
  for (const char of String(text)) {
    const size = (/[\x20-\x7e]/.test(char) ? fontSize * 0.7 : fontSize * 1.1) + spacing
    if (char === '\n' || line && used + size > available) {
      lines.push(line); line = ''; used = 0
      if (lines.length === maxLines) {
        lines[maxLines - 1] = [...lines[maxLines - 1]].slice(0, -2).join('') + '…'
        return lines.join('\n')
      }
      if (char === '\n') continue
    }
    line += char; used += size
  }
  lines.push(line)
  return lines.join('\n')
}

function spanOf(section: BlueprintSection) {
  const span = Number(section?.span)
  return Number.isFinite(span) ? clamp(Math.round(span), 1, GRID.columns) : GRID.columns
}

export function windowMs(window: string | undefined) {
  const text = String(window || '').trim().toLowerCase()
  const match = /^(\d+)\s*(m|h|d)$/.exec(text)
  if (!match) return 86400000
  const amount = Number(match[1])
  const unit = match[2]
  const value = unit === 'm' ? amount * 60_000 : unit === 'h' ? amount * 3_600_000 : amount * 86_400_000
  return clamp(value, 60_000, 2_678_400_000)
}

function uniqBy(list: string[]) {
  return [...new Set(list.filter(Boolean))]
}

function itemDevices(item: BlueprintItem): string[] {
  const explicit = Array.isArray(item?.devices) ? item.devices : []
  const single = typeof item?.device === 'string' ? [item.device] : []
  return uniqBy([...single, ...explicit])
}

function itemKeys(item: BlueprintItem): string[] {
  const explicit = Array.isArray(item?.keys) ? item.keys : []
  const single = typeof item?.key === 'string' ? [item.key] : []
  return uniqBy([...single, ...explicit])
}

function decimalsOf(item: BlueprintItem) {
  const declared = item?.binding?.decimals ?? item?.precision
  const value = declared == null ? NaN : Number(declared)
  if (Number.isFinite(value)) return clamp(Math.round(value), 0, 12)
  const form = String(item?.form || '').toLowerCase()
  // Counts and percentages read wrong with a forced decimal: "2.0 alarms".
  if (form === 'count' || form === 'percent' || form === 'progress' || form === 'text') return 0
  return (item?.binding?.unit ?? item?.unit) ? 1 : 0
}

function maxPointsFor(width: number) {
  return clamp(Math.round(width / 7), 48, 360)
}

function bindingFor(kind: string, device: string, keys: string[], section: BlueprintSection, item: BlueprintItem, width: number) {
  if (item.placeholder) return null
  if (item.binding) return { ...JSON.parse(JSON.stringify(item.binding)), decimals: decimalsOf(item), unit: item.binding.unit ?? item.unit }
  kind = item.kind || kind
  const devices = itemDevices(item)
  if (devices.length > 1 && kind !== 'aggregate' && kind !== 'deviceSummary') throw new Error('INVALID_METRIC_SCOPE')
  if (kind === 'deviceSummary') {
    if (!devices.length || keys.length || !['total', 'online', 'offline', 'unknown', 'onlineRate'].includes(item.aggregate || '')) throw new Error('INVALID_METRIC_SCOPE')
    return { kind, deviceCodes: devices, aggregate: item.aggregate, decimals: decimalsOf(item), unit: item.aggregate === 'onlineRate' ? '%' : item.unit }
  }
  if (kind === 'deviceStatus') return { kind, deviceCode: device, staleAfterMs: 300000 }
  if (kind === 'history') {
    return {
      kind,
      deviceCode: device,
      keys,
      aggregate: item.aggregate || 'avg',
      maxPoints: maxPointsFor(width),
      windowMs: windowMs(section.window),
      staleAfterMs: 300000,
      decimals: decimalsOf(item),
      unit: item?.unit
    }
  }
  if (kind === 'aggregate') {
    if (keys.length !== 1) throw new Error('INVALID_METRIC_SCOPE')
    return {
      kind,
      deviceCodes: devices.length ? devices : [device],
      keys,
      aggregate: item.aggregate || 'sum',
      maxPoints: 1,
      windowMs: windowMs(section.window),
      staleAfterMs: 300000,
      decimals: decimalsOf(item),
      unit: item?.unit
    }
  }
  return { kind: 'point', deviceCode: device, key: keys[0], decimals: decimalsOf(item), staleAfterMs: 300000, unit: item?.unit }
}

// Group items that can share one widget: same device for history, one widget per
// metric for scalars. Anything ungroupable is dropped with a note rather than
// producing a binding the runtime cannot serve.
function groupByDevice(items: BlueprintItem[]) {
  const groups = new Map<string, BlueprintItem[]>()
  for (const item of items) {
    const device = itemDevices(item)[0]
    if (!device && !item.binding && !item.placeholder) continue
    const scope = item.binding || item.placeholder ? `compiled:${groups.size}` : JSON.stringify([itemDevices(item), item.kind || '', item.aggregate || '', item.unit || ''])
    const list = groups.get(scope) || []
    list.push(item)
    groups.set(scope, list)
  }
  return [...groups.values()].map(list => [itemDevices(list[0])[0], list] as [string, BlueprintItem[]])
}


interface Rect { x: number; y: number; w: number; h: number }

function packRows(sections: BlueprintSection[]) {
  const rows: { sections: BlueprintSection[]; span: number }[] = []
  let current: BlueprintSection[] = []
  let span = 0
  for (const section of sections) {
    const width = spanOf(section)
    if (span + width > GRID.columns && current.length) {
      rows.push({ sections: current, span })
      current = []
      span = 0
    }
    current.push(section)
    span += width
    if (span >= GRID.columns) {
      rows.push({ sections: current, span })
      current = []
      span = 0
    }
  }
  if (current.length) rows.push({ sections: current, span })
  return rows
}

// Widths are distributed from a single unit so the row right edge is exact:
// rounding each column independently leaves 1-2px slivers over 12 columns.
function rowRects(row: { sections: BlueprintSection[]; span: number }, canvasWidth: number): Rect[] {
  const usable = canvasWidth - GRID.padding * 2
  const total = row.span || GRID.columns
  const scale = GRID.columns / total
  const rects: Rect[] = []
  let x = GRID.padding
  let consumed = 0
  for (let index = 0; index < row.sections.length; index++) {
    const section = row.sections[index]
    const last = index === row.sections.length - 1
    const own = spanOf(section) * scale
    const width = last ? canvasWidth - GRID.padding - x : Math.round((usable - GRID.gutter * (row.sections.length - 1)) * own / GRID.columns)
    rects.push({ x, y: 0, w: Math.max(1, width), h: 0 })
    x += width + GRID.gutter
    consumed += width
  }
  return rects
}

function sortSections(sections: BlueprintSection[]) {
  // The header must be the first band and the KPI strip the second; everything
  // else keeps the order the model proposed.
  const header = sections.find(section => roleOf(section) === 'header')
  const kpis = sections.filter(section => roleOf(section) === 'kpi')
  const rest = sections.filter(section => {
    const role = roleOf(section)
    return role && role !== 'header' && role !== 'kpi'
  })
  return { header, kpis, rest }
}

// The families describe content hierarchy, independent of an industry name.
// Authored spans remain authoritative; generated unpositioned sections use the
// curated slots without changing any metric or dropping any section.
export function chooseComposition(sections: BlueprintSection[], requested?: string): Composition {
  if (requested != null && !['overview', 'equipment', 'compact'].includes(requested)) throw new Error('INVALID_COMPOSITION')
  if (requested) return requested as Composition
  const body = sections.filter(section => !['header', 'kpi'].includes(roleOf(section) || ''))
  const analytical = body.filter(section => ['trend', 'distribution', 'map'].includes(roleOf(section) || ''))
  if (body.length <= 2 && !analytical.length) return 'compact'
  if (body.some(section => roleOf(section) === 'statusWall') && !body.some(section => ['alarmTable', 'eventTable', 'map'].includes(roleOf(section) || ''))) return 'equipment'
  return 'overview'
}

function compositionSections(sections: BlueprintSection[], composition: Composition): BlueprintSection[] {
  if (sections.some(section => section.span != null)) return sections
  const priorityOf = (section: BlueprintSection) => clamp(Number(section.priority) || 0, 0, 3)
  if (composition === 'equipment') {
    // Comparable measurements share a row before status furniture. A high
    // priority status wall must not split two peer charts into different sizes.
    const analytical = sections.filter(section => ['trend', 'distribution', 'map'].includes(roleOf(section) || ''))
    const status = sections.filter(section => roleOf(section) === 'statusWall')
    const remaining = sections.filter(section => !analytical.includes(section) && !status.includes(section))
    const byPriority = (a: BlueprintSection, b: BlueprintSection) => priorityOf(b) - priorityOf(a)
    const paired = (group: BlueprintSection[]) => group.sort(byPriority).map((section, index) => ({ ...section, span: index === group.length - 1 && index % 2 === 0 ? 12 : 6 }))
    return [...paired(analytical), ...paired(remaining), ...status.sort(byPriority).map(section => ({ ...section, span: 12 }))]
  }
  const rank = (section: BlueprintSection) => {
    const role = roleOf(section)
    const fit = role === 'trend' || role === 'map' ? 5 : ['alarmTable', 'eventTable'].includes(role) ? 4 : 2
    return priorityOf(section) * 10 + fit
  }
  const ordered = [...sections].sort((a, b) => composition === 'compact' ? priorityOf(b) - priorityOf(a) : rank(b) - rank(a))
  return ordered.map((section, index) => {
    const remaining = ordered.length - index
    const span = composition === 'overview'
      ? ordered.length === 1 ? 12 : index === 0 ? 8 : index === 1 ? 4 : remaining === 1 && index % 2 === 0 ? 12 : 6
      : remaining === 1 && index % 2 === 0 ? 12 : 6
    return { ...section, span }
  })
}

function statusBandHeight(section: BlueprintSection, panelWidth: number) {
  const devices = (section.items || []).flatMap(item => itemDevices(item).map(device => ({ label: itemDevices(item).length > 1 ? device : String(item.label || device) })))
  const width = panelWidth - 32
  let bodyHeight = 80
  if (devices.length && width >= 720 && devices.length <= 6) {
    const columns = Math.min(4, devices.length)
    const rows = Math.ceil(devices.length / columns)
    const cellWidth = Math.floor((width - GRID.gutter * (columns - 1)) / columns)
    const labelHeight = Math.max(24, ...devices.map(item => statusLabel(item.label, cellWidth).height))
    bodyHeight = rows * (Math.max(MIN_STATUS_CELL, labelHeight + 32) + 16) + GRID.gutter * (rows - 1)
  } else if (devices.length) {
    const maxColumns = Math.max(1, Math.floor((width + GRID.gutter) / (224 + GRID.gutter)))
    const columns = Math.min(maxColumns, Math.max(1, Math.ceil(devices.length / 6)))
    const nameWidth = Math.round(Math.floor((width - GRID.gutter * (columns - 1)) / columns) * 0.62)
    const rows = Math.ceil(devices.length / columns)
    const labels = devices.map(item => statusLabel(item.label, nameWidth))
    bodyHeight = Math.max(...Array.from({ length: columns }, (_, column) => labels.slice(column * rows, (column + 1) * rows).reduce((sum, label) => sum + Math.max(MIN_STATUS_ROW, label.height), 0))) + 16
  }
  const titleHeight = section.title || section.question ? TITLE_H : 0
  return Math.ceil((32 + titleHeight + Math.max(80, bodyHeight)) / 8) * 8
}

let sequence = 0
function nextId(prefix: string, taken: Set<string>) {
  let candidate = `ai-${prefix}-${++sequence}`
  while (taken.has(candidate)) candidate = `ai-${prefix}-${++sequence}`
  taken.add(candidate)
  return candidate
}

export function expandBlueprint(sections: BlueprintSection[], canvas: { width: number; height: number }, options: { taken?: Set<string>; locale?: string; composition?: string } = {}): LayoutResult {
  const english = (options.locale || '').startsWith('en')
  const width = Math.max(640, Math.floor(canvas?.width || 1920))
  const height = Math.max(480, Math.floor(canvas?.height || 1080))
  const taken = options.taken || new Set<string>()
  const notes: string[] = []
  const operations: LayoutOperation[] = []

  const usable = Array.isArray(sections) ? sections : []
  const { header, kpis, rest } = sortSections(usable)
  const composition = chooseComposition(usable, options.composition)
  const authored = rest.some(section => section.span != null)
  const rows = packRows(compositionSections(rest, composition))
  const kpiBandHeight = !authored && composition === 'compact' ? 160 : !authored && composition === 'equipment' ? 128 : KPI_H

  // ── Vertical allocation ──────────────────────────────────────────────────
  // Bands are stacked top-down from a single budget. The previous version let a
  // row keep a fixed minimum height even when the canvas could not hold it, and
  // doubled the gutters when it "compressed" rows; either way the last band
  // landed past the canvas edge, so the whole proposal was rejected as
  // overlapping or off-canvas. Everything below is derived from one number, so
  // the sum of band heights plus gutters never exceeds the usable height.
  // Sparse status bands leave intentional whitespace outside their borders.
  const kpiColumns = Math.min(6, Math.max(1, Math.floor((width - GRID.padding * 2 + GRID.gutter) / 224)))
  const kpiGroups = kpis.map(section => {
    const items = (Array.isArray(section.items) ? section.items : []).map(item => ({ ...item, priority: item.priority ?? section.priority ?? 0 }))
    if (!authored) items.sort((a, b) => (b.priority || 0) - (a.priority || 0))
    const rowCount = Math.ceil(items.length / kpiColumns)
    const title = String(section.title || section.question || '')
    const titleHeight = title ? 32 : 0
    return { items, title, titleHeight, height: titleHeight + rowCount * kpiBandHeight + GRID.gutter * Math.max(0, rowCount - 1), priority: section.priority ?? Math.max(0, ...items.map(item => item.priority || 0)) }
  }).filter(group => group.items.length)
  if (!authored) kpiGroups.sort((a, b) => b.priority - a.priority)
  const rankedKpis = kpiGroups.flatMap(group => group.items).slice().sort((a, b) => (b.priority || 0) - (a.priority || 0))
  const focalKpi = !authored && rankedKpis.length > 1 && (rankedKpis[0].priority || 0) > (rankedKpis[1].priority || 0) ? rankedKpis[0] : undefined
  type Band = { kind: 'header' } | { kind: 'kpi'; group: (typeof kpiGroups)[number] } | { kind: 'row'; row: (typeof rows)[number] }
  const bands: Band[] = []
  if (header) bands.push({ kind: 'header' })
  for (const group of kpiGroups) bands.push({ kind: 'kpi', group })
  for (const row of rows) bands.push({ kind: 'row', row })

  const fixedOf = (list: Band[]) => list.reduce((sum, band) => sum + (band.kind === 'header' ? HEADER_H : band.kind === 'kpi' ? band.group.height : 0), 0)
  const flexOf = (list: Band[]) => height - GRID.padding * 2 - GRID.gutter * Math.max(0, list.length - 1) - fixedOf(list)
  const bodyRows = (list: Band[]) => list.filter(band => band.kind === 'row').length

  const planned = bands
  const flexAvail = flexOf(planned)
  // Missing business sections cannot be repaired by silently deleting them.
  if (flexAvail < bodyRows(planned) * MIN_BODY_FLOOR) throw new Error('LAYOUT_CAPACITY')

  const plannedRows = planned.flatMap(band => (band.kind === 'row' ? [band.row] : []))
  const weights = plannedRows.map((row, index) => {
    const weight = Math.max(...row.sections.map(section => ROLE_WEIGHT[roleOf(section) || 'trend'] + clamp(Number(section.priority) || 0, 0, 3) * 2), 1)
    if (authored) return weight
    if (composition === 'compact') return 1 + Math.max(...row.sections.map(section => clamp(Number(section.priority) || 0, 0, 3)))
    return index === 0 ? weight * (composition === 'overview' ? 1.5 : 1.25) : weight
  })
  const heights: number[] = []
  if (plannedRows.length) {
    const base = Math.max(MIN_BODY_FLOOR, Math.min(MIN_BODY_H, Math.floor(flexAvail / plannedRows.length)))
    const caps = plannedRows.map(row => !authored && composition !== 'overview' && row.sections.length === 1 && roleOf(row.sections[0]) === 'statusWall'
      ? statusBandHeight(row.sections[0], width - GRID.padding * 2)
      : Infinity)
    plannedRows.forEach(() => heights.push(base))
    let spare = Math.max(0, flexAvail - base * plannedRows.length)
    let active = plannedRows.map((_row, index) => index)
    while (spare > 0 && active.length) {
      const totalWeight = active.reduce((sum, index) => sum + weights[index], 0) || 1
      const bounded = active.filter(index => heights[index] + spare * weights[index] / totalWeight >= caps[index])
      if (bounded.length) {
        bounded.forEach(index => {
          spare -= caps[index] - heights[index]
          heights[index] = caps[index]
        })
        active = active.filter(index => !bounded.includes(index))
        continue
      }
      let allocated = 0
      let cumulativeWeight = 0
      active.forEach(index => {
        cumulativeWeight += weights[index]
        const edge = Math.round(spare * cumulativeWeight / totalWeight)
        heights[index] += edge - allocated
        allocated = edge
      })
      spare = 0
    }
  }

  let y = GRID.padding
  const fullWidth = width - GRID.padding * 2

  // Every device the blueprint mentions, in order of first appearance. Sections
  // that list events (alarms) or plot positions (map) are about the whole screen,
  // and the server rejects any binding with an empty device set, so they scope
  // themselves to this set instead of emitting an unbindable component.
  // KPI devices come first: they are the metrics the screen leads with, so an
  // alarm feed or map that covers "the whole screen" lists them in that order.
  const planDevices = uniqBy(
    kpis.flatMap(section => (Array.isArray(section?.items) ? section.items : [])).flatMap(itemDevices)
      .concat(plannedRows.flatMap(row => row.sections.flatMap(section => (Array.isArray(section?.items) ? section.items : []).flatMap(itemDevices))))
  ).slice(0, 200)

  if (header) {
    const titleText = String(header.title || '').slice(0, 64)
    const subtitle = String(header.subtitle || '').slice(0, 120)
    // The clock is furniture on the right edge; the title gets what is left, and
    // it never claims the clock's rectangle on a narrow canvas.
    const timeW = clamp(Math.round(width * 0.14), 160, 260)
    const copyW = Math.max(160, fullWidth - timeW - GRID.gutter)
    if (titleText) {
      operations.push({
        type: 'add',
        id: nextId('header', taken),
        key: 'TextCommon',
        x: GRID.padding,
        y: y + 10,
        w: copyW,
        h: 52,
        title: titleText,
        theme: { variant: 'title', option: { dataset: titleText, fontSize: 32, fontWeight: '700', letterSpacing: 2, textAlign: 'left', paddingX: 0 } }
      })
    }
    if (subtitle) {
      operations.push({
        type: 'add',
        id: nextId('subtitle', taken),
        key: 'TextCommon',
        x: GRID.padding,
        y: y + 62,
        w: copyW,
        h: 22,
        title: subtitle,
        theme: { variant: 'label', option: { dataset: subtitle, fontSize: 12, letterSpacing: 1, textAlign: 'left' } }
      })
    }
    operations.push({
      type: 'add',
      id: nextId('clock', taken),
      key: 'TimeCommon',
      x: width - GRID.padding - timeW,
      y: y + 18,
      w: timeW,
      h: 44,
      title: '',
      theme: { option: { timeSize: timeW < 200 ? 14 : 16, timeLineHeight: 40, timeTextIndent: 0, showShadow: false } }
    })
    // Decorative rules occupy their own strip, so the existing collision check
    // still protects all content rather than exempting arbitrary decorations.
    operations.push({ type: 'add', id: nextId('headrule', taken), key: 'Decorates02', x: GRID.padding, y: y + 88, w: fullWidth, h: 4, title: '', theme: {} })
    y += HEADER_H + GRID.gutter
  }

  for (const group of kpiGroups) {
    const visible = group.items
    // Business groups retain their captions and row boundaries. A high-priority
    // item may move within its group, never into another question's strip.
    const stripTitle = group.title
    const bodyY = y + group.titleHeight
    const bodyH = kpiBandHeight
    if (stripTitle) {
      operations.push({
        type: 'add',
        id: nextId('kpihdr', taken),
        key: 'TextCommon',
        x: GRID.padding,
        y,
        w: fullWidth,
        h: 24,
        title: stripTitle,
        theme: { variant: 'label', option: { dataset: stripTitle, fontSize: 12, letterSpacing: 1, textAlign: 'left', paddingX: 0 } }
      })
    }
    const count = Math.min(kpiColumns, visible.length) || 1
    const cells: { x: number; w: number }[] = []
    for (let start = 0; start < visible.length; start += count) {
      const rowItems = visible.slice(start, start + count)
      const available = fullWidth - GRID.gutter * (rowItems.length - 1)
      // Enlarge a unique primary metric only as far as peer labels stay readable.
      const focalWeight = Math.max(1, Math.min(2, available / 208 - (rowItems.length - 1)))
      const weights = rowItems.map(item => item === focalKpi ? focalWeight : 1)
      const total = weights.reduce((sum, weight) => sum + weight, 0)
      let consumed = 0
      rowItems.forEach((_, at) => {
        const left = Math.round(available * consumed / total / 8) * 8
        consumed += weights[at]
        const right = at === rowItems.length - 1 ? available : Math.round(available * consumed / total / 8) * 8
        cells.push({ x: GRID.padding + left + GRID.gutter * at, w: right - left })
      })
    }
    const labelH = 48
    visible.forEach((item, index) => {
      const device = itemDevices(item)[0]
      const keys = itemKeys(item)
      const title = String(item?.label || '')
      const caption = title.includes(' · ') ? title.split(' · ').slice(1).join(' · ') : title
      const unit = String(item?.binding?.unit ?? item?.unit ?? '')
      if (!item.binding && !item.placeholder && (!device || !keys.length && item.kind !== 'deviceSummary')) {
        notes.push(`KPI "${title}" has no device/key and was skipped`)
        return
      }
      const cellX = cells[index].x
      const cellWidth = cells[index].w
      const valueH = bodyH - labelH - 32
      const kind = item.binding?.kind || item.kind || (itemDevices(item).length > 1 || item.aggregate ? 'aggregate' : 'point')
      const key = item.placeholder ? 'TextCommon' : selectPresentation({ role: 'kpi', task: item.task, hint: item.form, width: cellWidth - 32, height: valueH, kind })
      if (item.form === 'percent' && key !== 'PieCircle') notes.push(english ? `"${title}" uses a number because the ring would be too small.` : `“${title}”改用数字显示，当前区域不足以清晰显示环图。`)
      if (!item.placeholder && kind !== 'dataset' && !kindsOf(key).includes(kind)) throw new Error('INVALID_METRIC_KIND')
      const cellY = bodyY + Math.floor(index / count) * (kpiBandHeight + GRID.gutter)
      operations.push({ type: 'add', id: nextId('kpicard', taken), key: 'Border06', x: cellX, y: cellY, w: cellWidth, h: bodyH, title, theme: { tone: item?.tone } })
      // The caption is its own element: folding it into the widget's prefixText
      // renders it at the value's size, which turns a KPI strip into a wall of
      // equal-weight text.
      operations.push({
        type: 'add',
        id: nextId('kpilabel', taken),
        key: 'TextCommon',
        x: cellX + 16,
        y: cellY + 16,
        w: cellWidth - 32,
        h: labelH,
        title,
        theme: { variant: 'metricLabel', option: { dataset: caption, fontSize: 13, letterSpacing: 1, textAlign: 'center', paddingX: 0 } }
      })
      const emphasis = !focalKpi || item === focalKpi ? 1 : 0.8
      operations.push({
        type: 'add',
        id: nextId('kpi', taken),
        key,
        x: cellX + 16,
        y: cellY + 16 + labelH,
        w: cellWidth - 32,
        h: valueH,
        title,
        option: key === 'Number'
          ? { prefixText: '', suffixText: unit ? ` ${unit}` : '', precision: decimalsOf(item), numberSize: Math.round(clamp(Math.round(valueH * 0.62), 36, 72) * emphasis) }
          : key === 'Process' ? { type: 'line', indicatorPlacement: 'inside' }
          : { dataset: item.placeholder ? '—' : title },
        binding: bindingFor(kind, device, keys, item as any, item, cellWidth),
        theme: { tone: item?.tone, variant: key === 'TextCommon' ? 'value' : 'metricLabel', ...(key === 'TextCommon' ? { option: { fontSize: Math.round(clamp(Math.round(valueH * 0.6), 28, 48) * emphasis), fontWeight: '600', textAlign: 'center', paddingX: 0 } } : {}) }
      })
    })
    y += group.height + GRID.gutter
  }

  plannedRows.forEach((row, index) => {
    const rects = rowRects(row, width)
    const rowH = heights[index]
    row.sections.forEach((section, sectionIndex) => {
      const role = roleOf(section) || 'trend'
      const panel: Rect = { ...rects[sectionIndex], y, h: rowH }
      operations.push({ type: 'add', id: nextId('panel', taken), key: 'Border06', ...panel, title: section.title, theme: { tone: role === 'alarmTable' ? 'warning' : role === 'statusWall' ? 'success' : 'primary' } })
      const rect: Rect = { x: panel.x + 16, y: panel.y + 16, w: panel.w - 32, h: panel.h - 32 }
      const sectionTitle = String(section?.title || section?.question || '').slice(0, 80)
      let bodyY = rect.y
      let bodyH = rect.h
      if (sectionTitle && role !== 'kpi') {
        operations.push({
          type: 'add',
          id: nextId('title', taken),
          key: 'TextCommon',
          x: rect.x,
          y: rect.y,
          w: rect.w,
          h: TITLE_H - 8,
          title: sectionTitle,
          theme: { variant: 'title', option: { dataset: sectionTitle, fontSize: (section.priority || 0) >= 3 ? 20 : 16, letterSpacing: 0.5, textAlign: 'left', paddingX: 0 } }
        })
        operations.push({ type: 'add', id: nextId('sectionrule', taken), key: 'Decorates02', x: rect.x, y: rect.y + TITLE_H - 4, w: rect.w, h: 4, title: '', theme: { tone: role === 'alarmTable' ? 'warning' : role === 'statusWall' ? 'success' : 'primary' } })
        bodyY += TITLE_H
        bodyH -= TITLE_H
      }
      if (bodyH < 40) throw new Error('LAYOUT_CAPACITY')
      const items = Array.isArray(section?.items) ? section.items : []

      if (role === 'statusWall') {
        // A narrow column reads as a status list (name left, state right); only a
        // wide band is worth splitting into a grid. Spreading eight devices over
        // equal quarter-cells leaves the column visually empty.
        const devices = items.flatMap(item => itemDevices(item).map(device => ({ ...item, label: itemDevices(item).length > 1 ? device : item.label, device, devices: undefined })))
        if (!devices.length) {
          notes.push(`section "${sectionTitle || role}" has no usable device`)
          return
        }
        const perRow = Math.min(4, devices.length)
        const gridRows = Math.ceil(devices.length / perRow)
        const gridCellW = Math.floor((rect.w - GRID.gutter * (perRow - 1)) / perRow)
        const gridLabelH = Math.max(24, ...devices.map(item => statusLabel(String(item.label || item.device), gridCellW).height))
        // A grid cell needs real height; when it does not fit, fall back to the
        // dense list instead of forcing cells past the band (which used to land
        // on the next row and fail the whole proposal as overlapping).
        const gridFits = rect.w >= 720 && devices.length <= 6 && gridRows * Math.max(MIN_STATUS_CELL, gridLabelH + 32) + GRID.gutter * (gridRows - 1) <= bodyH
        if (gridFits) {
          const cellW = gridCellW
          const cellH = Math.floor((bodyH - GRID.gutter * (gridRows - 1)) / gridRows)
          devices.forEach((item, at) => {
            const device = itemDevices(item)[0]
            const cx = rect.x + (at % perRow) * (cellW + GRID.gutter)
            const cy = bodyY + Math.floor(at / perRow) * (cellH + GRID.gutter)
            const label = String(item?.label || device)
            operations.push({
              type: 'add',
              id: nextId('devname', taken),
              key: 'TextCommon',
              x: cx,
              y: cy,
              w: cellW,
              h: gridLabelH,
              title: label,
              theme: { variant: 'value', option: { dataset: statusLabel(label, cellW).text, fontSize: 13, letterSpacing: 0, textAlign: 'center', paddingX: 0 } }
            })
            operations.push({
              type: 'add',
              id: nextId('devstate', taken),
              key: 'TextCommon',
              x: cx,
              y: cy + gridLabelH + 8,
              w: cellW,
              h: Math.max(24, Math.min(36, cellH - gridLabelH - 8)),
              title: label,
              theme: { variant: 'value', option: { fontSize: 14, fontWeight: '600', letterSpacing: 0.5, textAlign: 'center', paddingX: 0 } },
              binding: bindingFor('deviceStatus', device, [], section, item, cellW)
            })
          })
          return
        }
        // Choose the fewest readable columns that hold every full device name.
        // Long names get multiple lines; impossible content remains a capacity
        // error instead of disappearing behind TextCommon's overflow clipping.
        let list: { columnW: number; nameW: number; rowCount: number; rows: ReturnType<typeof statusLabel>[] } | undefined
        const maxColumns = Math.floor((rect.w + GRID.gutter) / (224 + GRID.gutter))
        for (let columns = 1; columns <= maxColumns; columns++) {
          const columnW = Math.floor((rect.w - GRID.gutter * (columns - 1)) / columns)
          const nameW = Math.round(columnW * 0.62)
          const rowCount = Math.ceil(devices.length / columns)
          const rows = devices.map(item => statusLabel(String(item.label || item.device), nameW))
          const heights = Array.from({ length: columns }, (_, column) => rows.slice(column * rowCount, (column + 1) * rowCount).reduce((sum, row) => sum + Math.max(MIN_STATUS_ROW, row.height), 0))
          if (heights.every(height => height <= bodyH)) {
            list = { columnW, nameW, rowCount, rows }
            break
          }
        }
        if (!list) throw new Error('LAYOUT_CAPACITY')
        const { columnW, nameW, rowCount, rows } = list
        const cursors: number[] = []
        devices.forEach((item, at) => {
          const device = itemDevices(item)[0]
          const column = Math.floor(at / rowCount)
          const columnRows = rows.slice(column * rowCount, (column + 1) * rowCount)
          const minimumH = columnRows.reduce((sum, row) => sum + Math.max(MIN_STATUS_ROW, row.height), 0)
          const extraH = Math.floor((bodyH - minimumH) / columnRows.length)
          const cellH = Math.max(MIN_STATUS_ROW, rows[at].height) + extraH
          const cx = rect.x + column * (columnW + GRID.gutter)
          const cy = cursors[column] ?? bodyY
          cursors[column] = cy + cellH
          const label = String(item?.label || device)
          operations.push({
            type: 'add',
            id: nextId('devname', taken),
            key: 'TextCommon',
            x: cx,
            y: cy,
            w: nameW,
            h: cellH,
            title: label,
            theme: { variant: 'value', option: { dataset: rows[at].text, fontSize: 13, letterSpacing: 0, textAlign: 'left', paddingX: 0 } }
          })
          operations.push({
            type: 'add',
            id: nextId('devstate', taken),
            key: 'TextCommon',
            x: cx + nameW,
            y: cy,
            w: columnW - nameW,
            h: cellH,
            title: label,
            theme: { variant: 'value', option: { fontSize: 14, fontWeight: '600', letterSpacing: 0.5, textAlign: 'right', paddingX: 0 } },
            binding: bindingFor('deviceStatus', device, [], section, item, rect.w)
          })
        })
        return
      }

      if (role === 'alarmTable' || role === 'eventTable' || role === 'list') {
        const key = selectPresentation({ role, task: section.task, width: rect.w, height: bodyH, kind: section.binding?.kind })
        const supportsAlarms = kindsOf(key).includes('alarmList')
        const sectionDevices = uniqBy([...(section.devices || []), ...items.flatMap(itemDevices)])
        const keys = uniqBy(items.flatMap(itemKeys))
        // Alarm-center incidents and raw product events use distinct feeds. A
        // ranking list defaults to alarms only without a metric. Runtime scopes the
        // bound device set, and the server rejects a binding that has none, so an
        // unscoped section covers every device the blueprint mentions.
        const wantEvents = role === 'eventTable'
        const wantAlarms = !wantEvents && (role === 'alarmTable' || !keys.length)
        const devices = sectionDevices.length ? sectionDevices : planDevices
        let binding: Record<string, any> | null
        if (section.binding) {
          binding = JSON.parse(JSON.stringify(section.binding))
          if (binding!.kind !== 'dataset' && !kindsOf(key).includes(binding!.kind)) throw new Error('INVALID_METRIC_KIND')
        } else if (wantEvents) {
          if (!kindsOf(key).includes('eventList') || !devices.length) {
            notes.push(`section "${sectionTitle || role}" has no compatible event feed`)
            return
          }
          binding = { kind: 'eventList', deviceCodes: devices, windowMs: windowMs(section.window), maxPoints: 120 }
        } else if (wantAlarms) {
          if (!supportsAlarms) {
            notes.push(`section "${sectionTitle || role}" cannot list alarms and was skipped`)
            return
          }
          if (!devices.length) {
            notes.push(`section "${sectionTitle || role}" has no device to list alarms for and was skipped`)
            return
          }
          binding = devices.length === 1 ? { kind: 'alarmList', deviceCode: devices[0] } : { kind: 'alarmList', deviceCodes: devices }
        } else {
          if (!sectionDevices.length) {
            notes.push(`section "${sectionTitle || role}" has no usable device and was skipped`)
            return
          }
          binding = bindingFor('history', sectionDevices[0], keys, section, items[0] || {}, rect.w)
        }
        operations.push({
          type: 'add',
          id: nextId(wantEvents ? 'events' : role === 'list' ? 'list' : 'alarms', taken),
          key,
          x: rect.x,
          y: bodyY,
          w: rect.w,
          h: bodyH,
          title: sectionTitle,
          theme: { option: wantEvents ? {
            header: english ? ['Time', 'Device', 'Event', 'Type', 'Parameters'] : ['时间', '设备', '事件', '类型', '参数'],
            align: ['left', 'left', 'left', 'center', 'left']
          } : {} },
          binding
        })
        return
      }

      if (role === 'map') {
        // Positions are read per bound device, so an empty set is rejected by the
        // server; a map section legitimately covers the whole screen.
        const sectionDevices = uniqBy([...(section.devices || []), ...items.flatMap(itemDevices)])
        const devices = sectionDevices.length ? sectionDevices : planDevices
        if (!section.binding && !devices.length) {
          notes.push('map section has no device with a location and was skipped')
          return
        }
        operations.push({
          type: 'add',
          id: nextId('map', taken),
          key: 'LocationMap',
          x: rect.x,
          y: bodyY,
          w: rect.w,
          h: bodyH,
          title: sectionTitle,
          theme: { option: {} },
          binding: section.binding ? JSON.parse(JSON.stringify(section.binding)) : { kind: 'location', locationSource: 'device', coordinateSystem: 'wgs84', ...(devices.length === 1 ? { deviceCode: devices[0] } : { deviceCodes: devices }) }
        })
        return
      }

      // trend / distribution: one widget per device group. A second device in the
      // same section is rendered as a separate stacked widget so no binding has to
      // combine unrelated series.
      const groups = groupByDevice(items)
      if (!groups.length) {
        notes.push(`section "${sectionTitle || role}" has no usable device`)
        return
      }
      const usableGroups = groups
      if (bodyH < MIN_SLICE_H * usableGroups.length + GRID.gutter * (usableGroups.length - 1)) throw new Error('LAYOUT_CAPACITY')
      const sliceH = Math.floor((bodyH - GRID.gutter * (usableGroups.length - 1)) / usableGroups.length)
      usableGroups.forEach(([device, group], at) => {
        const keys = uniqBy(group.flatMap(itemKeys))
        if (!keys.length && !group[0]?.binding && !group[0]?.placeholder) {
          notes.push(`section "${sectionTitle || role}" has no property key`)
          return
        }
        const viz = String(group[0]?.viz || section.viz || '').toLowerCase()
        const key = group[0]?.placeholder ? 'TextCommon' : selectPresentation({ role, task: group[0]?.task || section.task, hint: viz, seriesCount: keys.length, categoryCount: group[0]?.categoryCount, width: rect.w, height: sliceH, kind: group[0]?.binding?.kind || group[0]?.kind })
        const kind = group[0]?.binding?.kind || group[0]?.kind || (group[0]?.aggregate ? 'aggregate' : kindsOf(key).includes('history') ? 'history' : kindsOf(key)[0])
        if (!group[0]?.placeholder && kind !== 'dataset' && !kindsOf(key).includes(kind)) throw new Error('INVALID_METRIC_KIND')
        if (kind === 'aggregate' && keys.length !== 1) throw new Error('INVALID_METRIC_SCOPE')
        const sliceY = bodyY + at * (sliceH + GRID.gutter)
        operations.push({
          type: 'add',
          id: nextId(role === 'distribution' ? 'dist' : 'trend', taken),
          key,
          x: rect.x,
          y: sliceY,
          w: rect.w,
          h: at === usableGroups.length - 1 ? bodyY + bodyH - sliceY : sliceH,
          title: usableGroups.length > 1 && sectionTitle ? `${sectionTitle} · ${group[0]?.label || device}` : sectionTitle,
          theme: { tone: group[0]?.tone, ...(group[0]?.placeholder ? { option: { dataset: english ? 'Data capability pending' : '数据能力待接通', fontSize: 16 } } : {}) },
          binding: bindingFor(kind, device, keys, section, group[0], rect.w)
        })
      })
    })
    y += rowH + GRID.gutter
  })

  // ── Last line of defence ─────────────────────────────────────────────────
  // The allocation above is meant to be collision-free by construction, and the
  // tests assert exactly that. This pass exists so a future layout change can
  // never turn a rounding slip into "the whole proposal failed": the later of two
  // colliding elements is dropped and reported instead of rejecting everything.
  const kept: LayoutOperation[] = []
  for (const operation of operations) {
    if (operation.key === 'TextCommon' && !operation.binding) {
      const option = operation.theme?.option
      if (option?.dataset != null) option.dataset = fitStaticText(option.dataset, operation.w, operation.h, option.fontSize || 14, option.letterSpacing || 0, option.paddingX || 0, option.paddingY || 0)
    }
    if (operation.key !== 'Border06' && kept.some(other => other.key !== 'Border06' && Math.min(other.x + other.w, operation.x + operation.w) - Math.max(other.x, operation.x) > 2 && Math.min(other.y + other.h, operation.y + operation.h) - Math.max(other.y, operation.y) > 2)) throw new Error('LAYOUT_OVERLAP')
    kept.push(operation)
  }

  return { operations: kept, notes, composition }
}

export { applyScreenTheme }
