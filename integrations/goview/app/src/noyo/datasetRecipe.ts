// Guided authoring for the `dataset` binding kind.
//
// The screen editor used to expose the data-query DSL directly, which meant an
// operator had to know the selector shape, the step pipeline and the output
// mapping before seeing a single number. Everything in this module turns an
// intent ("latest reading per device", "trend of this measure") into a plan the
// backend already accepts, so the UI can ask five plain questions instead of
// asking for JSON.
//
// Kept free of Vue imports so the mapping can be unit tested directly.

export const SCALAR_SHAPES = ['number', 'percent', 'progress', 'text', 'flip']
export const TABULAR_SHAPES = ['table', 'table-basic', 'table-grid']
const ALL_SHAPES = [
  'number', 'percent', 'progress', 'text', 'flip',
  'chart', 'pie', 'radar', 'vchart', 'heatmap', 'treemap', 'words',
  'table', 'table-basic', 'table-grid'
]

export type DatasetSource = 'telemetry' | 'latest' | 'devices' | 'alarms' | 'workorders' | 'static'

export interface RecipeForm {
  source: DatasetSource
  /** Product scope. Only used when no explicit device is picked. */
  product: string
  /** Comma separated device codes; empty means the whole readable scope. */
  devices: string
  field: string
  method: string
  /** `sample`: every valid sample weighs the same. `device`: average per device first. */
  weight: 'sample' | 'device'
  group: string
  hours: number
  /** 0 disables bucketing. */
  minutes: number
  timezone: string
}

export interface Recipe {
  id: string
  zh: string
  en: string
  descZh: string
  descEn: string
  shapes: string[]
  source: DatasetSource
  /** `false` hides the aggregation question because the recipe fixes it. */
  method: string | false
  field: boolean
  hours: number
  group: string
}

// `group` sentinels resolved against the widget shape: a single-value widget can
// only render one row, so the same recipe collapses its grouping for it.
export const AUTO_DEVICE = 'AUTO:device'
export const AUTO_ALARM = 'AUTO:alarm'
export const AUTO_STATE = 'AUTO:state'

export const RECIPES: Recipe[] = [
  {
    id: 'latest', zh: '最新读数', en: 'Latest reading',
    descZh: '每台设备最后一个上报值', descEn: 'Last reported value per device',
    shapes: ALL_SHAPES, source: 'latest', method: false, field: true, hours: 24, group: AUTO_DEVICE
  },
  {
    id: 'trend', zh: '变化趋势', en: 'Trend over time',
    descZh: '一个指标随时间怎么变化', descEn: 'How one measure changes over time',
    shapes: ['chart', 'pie', 'vchart', 'radar', 'heatmap', 'table', 'table-basic', 'table-grid'],
    source: 'telemetry', method: 'avg', field: true, hours: 24, group: ''
  },
  {
    id: 'summary', zh: '汇总成一个数', en: 'One summary number',
    descZh: '所选设备整段时间合成一个数值', descEn: 'Selected devices and samples combined into one number',
    shapes: ['number', 'percent', 'progress', 'text', 'chart', 'pie', 'vchart'],
    source: 'telemetry', method: 'avg', field: true, hours: 24, group: ''
  },
  {
    id: 'compare', zh: '设备之间对比', en: 'Compare devices',
    descZh: '每台设备一个点，看谁高谁低', descEn: 'One point per device so values can be compared',
    shapes: ['chart', 'pie', 'radar', 'vchart', 'heatmap', 'treemap', 'words', 'table', 'table-basic', 'table-grid'],
    source: 'telemetry', method: 'avg', field: true, hours: 24, group: 'device_code'
  },
  {
    id: 'alarms', zh: '告警数量', en: 'Alarm counts',
    descZh: '按级别统计告警条数', descEn: 'Counts alarm records by severity',
    shapes: ALL_SHAPES, source: 'alarms', method: false, field: false, hours: 24, group: AUTO_ALARM
  },
  {
    id: 'devices', zh: '设备数量', en: 'Device counts',
    descZh: '在线、离线、未知各多少台', descEn: 'How many devices are online, offline or unknown',
    shapes: ALL_SHAPES, source: 'devices', method: false, field: false, hours: 24, group: AUTO_STATE
  }
]

export const AGGREGATIONS = [
  { value: 'avg', zh: '平均值', en: 'Average', hintZh: '把范围内的采样点平均，适合温度、压力这类连续读数', hintEn: 'Averages every sample in the window; suits continuous readings such as temperature or pressure' },
  { value: 'max', zh: '最大值', en: 'Maximum', hintZh: '范围内最大的那次采样，适合找峰值', hintEn: 'The single highest sample in the window; use it to find peaks' },
  { value: 'min', zh: '最小值', en: 'Minimum', hintZh: '范围内最小的那次采样，适合找谷值', hintEn: 'The single lowest sample in the window; use it to find troughs' },
  { value: 'sum', zh: '合计', en: 'Total', hintZh: '所有采样点相加，适合累计量（能耗、产量）', hintEn: 'Adds every sample; suits cumulative readings such as energy or output' },
  { value: 'count', zh: '条数', en: 'Record count', hintZh: '只数多少条记录，不看具体数值', hintEn: 'Counts records without reading any value' },
  { value: 'count_distinct', zh: '去重条数', en: 'Distinct count', hintZh: '按指标值去重后计数', hintEn: 'Counts distinct measure values' },
  { value: 'median', zh: '中位数', en: 'Median', hintZh: '排序后取中间值，能避开极端值干扰', hintEn: 'Middle value after sorting; resistant to outliers' }
]

export const WINDOWS = [
  { hours: 1, zh: '近 1 小时', en: 'Last hour' },
  { hours: 6, zh: '近 6 小时', en: 'Last 6 hours' },
  { hours: 24, zh: '近 24 小时', en: 'Last 24 hours' },
  { hours: 168, zh: '近 7 天', en: 'Last 7 days' },
  { hours: 720, zh: '近 30 天', en: 'Last 30 days' }
]

export const BUCKETS = [
  { minutes: 5, zh: '5 分钟', en: '5 minutes' },
  { minutes: 60, zh: '1 小时', en: '1 hour' },
  { minutes: 1440, zh: '1 天', en: '1 day' }
]

export function isScalarShape(shape: string) { return SCALAR_SHAPES.includes(shape) }
export function isTabularShape(shape: string) { return TABULAR_SHAPES.includes(shape) }

export function recipesFor(shape: string) {
  return RECIPES.filter(recipe => recipe.shapes.includes(shape))
}

export function recipeById(id: string) { return RECIPES.find(recipe => recipe.id === id) }

/** Device codes are stored as a comma separated string, matching the selector. */
export function splitCodes(text: string): string[] {
  return [...new Set(String(text || '').split(',').map(value => value.trim()).filter(Boolean))]
}

export function toggleCode(text: string, code: string): string {
  const codes = splitCodes(text)
  const index = codes.indexOf(code)
  if (index >= 0) codes.splice(index, 1)
  else codes.push(code)
  return codes.join(',')
}

export function defaultForm(shape: string, timezone = 'Asia/Shanghai'): RecipeForm {
  const recipe = recipesFor(shape)[0]
  return applyRecipe(defaultRawForm(shape, timezone), recipe, shape)
}

export function defaultRawForm(shape: string, timezone = 'Asia/Shanghai'): RecipeForm {
  return {
    source: 'telemetry', product: '', devices: '', field: '',
    method: 'avg', weight: 'sample', group: '', hours: 24,
    minutes: isScalarShape(shape) ? 0 : 60, timezone
  }
}

function resolveGroup(recipe: Recipe, shape: string): string {
  // A single-value widget renders exactly one row, so every recipe collapses
  // its grouping for those shapes instead of producing an invalid binding.
  const scalar = isScalarShape(shape)
  if (recipe.group === AUTO_DEVICE) return scalar ? '' : 'device_code'
  if (recipe.group === AUTO_ALARM) return scalar ? '' : 'severity'
  if (recipe.group === AUTO_STATE) return scalar ? '' : 'online'
  return recipe.group
}

/**
 * Applies a recipe to an existing form. Device and measure selections are kept
 * so switching intent does not throw away what the user already picked.
 */
export function applyRecipe(form: RecipeForm, recipe: Recipe | undefined, shape: string): RecipeForm {
  const next: RecipeForm = { ...form }
  if (!recipe) return next
  next.source = recipe.source
  next.hours = recipe.hours
  next.method = typeof recipe.method === 'string' ? recipe.method : (next.method || 'avg')
  if (recipe.method === false) next.method = recipe.source === 'alarms' || recipe.source === 'devices' ? 'count' : 'avg'
  next.weight = 'sample'
  next.group = resolveGroup(recipe, shape)
  // Bucketing only means something for a time series source.
  next.minutes = recipe.id === 'trend' ? (form.minutes || 60) : 0
  if (!recipe.field) next.field = ''
  return next
}

export function buildPlan(form: RecipeForm) {
  const source: any = { id: 'raw', kind: form.source }
  if (form.source === 'static') {
    source.rows = [{ name: 'A', value: 10 }, { name: 'B', value: 20 }]
  } else if (form.source === 'workorders') {
    // Work orders reject every selector field; filtering happens on result fields.
  } else {
    const selector: any = {}
    const codes = splitCodes(form.devices)
    if (codes.length) selector.deviceCodes = codes
    else if (form.product) selector.productCode = form.product
    source.selector = selector
  }
  if (form.source === 'telemetry' || form.source === 'latest') source.fields = form.field ? [form.field] : []

  const steps: any[] = []
  let input = 'raw'
  const group = splitCodes(form.group)

  if (form.minutes > 0 && form.source === 'telemetry') {
    steps.push({
      id: 'bucketed', op: 'bucket', input, field: 'ts',
      ...(form.minutes === 60 ? { calendar: 'hour' } : form.minutes === 1440 ? { calendar: 'day' } : { intervalMs: form.minutes * 60000 })
    })
    input = 'bucketed'
    group.unshift('ts')
  }
  if (form.weight === 'device' && splitCodes(form.devices).length > 1 && ['avg', 'sum'].includes(form.method)) {
    steps.push({ id: 'per_device', op: 'aggregate', input, groupBy: [...new Set([...group, 'device_code'])], measures: [{ id: 'device_avg', function: 'avg', field: form.field }] })
    input = 'per_device'
  }
  const measure: any = { id: 'value', function: form.method }
  if (form.method !== 'count') measure.field = form.weight === 'device' && splitCodes(form.devices).length > 1 ? 'device_avg' : form.field || 'value'
  steps.push({ id: 'stats', op: 'aggregate', input, groupBy: group, measures: [measure] })
  input = 'stats'
  if (group.length) {
    steps.push({ id: 'ordered', op: 'sort', input, orders: group.map(field => ({ field })) })
    input = 'ordered'
  }
  return { schemaVersion: 'noyo.dataset/1', sources: [source], steps, output: input, windowMs: form.hours * 3600000, timezone: form.timezone, maxRows: 1000 }
}

/**
 * Recovers the guided form from a stored plan. Returns null when the plan uses
 * anything the guide cannot express (filter/derive/join/union/project/limit),
 * so the caller keeps showing the JSON editor instead of silently dropping
 * steps on the next apply.
 */
export function adoptPlan(plan: any, timezone = 'Asia/Shanghai'): { form: RecipeForm; recipeId: string } | null {
  if (!plan || typeof plan !== 'object' || !Array.isArray(plan.sources) || plan.sources.length !== 1) return null
  const source = plan.sources[0]
  if (!source || !['telemetry', 'latest', 'devices', 'alarms'].includes(source.kind)) return null
  const steps = Array.isArray(plan.steps) ? plan.steps : []
  if (steps.some((step: any) => !['bucket', 'aggregate', 'sort'].includes(step?.op))) return null
  const aggregates = steps.filter((step: any) => step?.op === 'aggregate')
  if (aggregates.length === 0 || aggregates.length > 2) return null
  const last = aggregates[aggregates.length - 1]
  const measures = Array.isArray(last?.measures) ? last.measures : []
  if (measures.length !== 1) return null
  const measure = measures[0]
  if (measures[0]?.id !== 'value') return null
  if (Array.isArray(last.groupBy) && last.groupBy.some((field: string) => field !== 'ts' && field !== 'device_code' && field !== 'severity' && field !== 'online')) return null
  const bucket = steps.find((step: any) => step?.op === 'bucket')
  const minutes = bucket ? (bucket.calendar === 'hour' ? 60 : bucket.calendar === 'day' ? 1440 : Math.round((bucket.intervalMs || 0) / 60000)) : 0
  const groupBy = (Array.isArray(last.groupBy) ? last.groupBy : []).filter((field: string) => field !== 'ts')
  const selector = source.selector || {}
  const form: RecipeForm = {
    source: source.kind,
    product: selector.productCode || '',
    devices: Array.isArray(selector.deviceCodes) ? selector.deviceCodes.join(',') : '',
    field: Array.isArray(source.fields) && source.fields.length ? String(source.fields[0]) : (measure.field && measure.field !== 'value' && measure.field !== 'device_avg' ? String(measure.field) : ''),
    method: String(measure.function || 'avg'),
    weight: aggregates.length > 1 ? 'device' : 'sample',
    group: groupBy.join(','),
    hours: Math.max(1, Math.round((plan.windowMs || 86400000) / 3600000)),
    minutes,
    timezone: plan.timezone || timezone
  }
  let recipeId = ''
  if (source.kind === 'alarms') recipeId = 'alarms'
  else if (source.kind === 'devices') recipeId = 'devices'
  else if (source.kind === 'latest') recipeId = 'latest'
  else if (minutes > 0) recipeId = 'trend'
  else if (groupBy.includes('device_code')) recipeId = 'compare'
  else recipeId = 'summary'
  return { form, recipeId }
}

export interface FieldMeta { id: string; type?: string }

const TIME_FIELD = /^(ts|time|date)$|(_at|_ts|_time)$/i

/**
 * Picks sensible value/category/series fields from a query result so the user
 * never has to type a column name. Explicit picks are preserved when the new
 * result still contains them.
 *
 * Timestamps arrive as `number`, so the type alone cannot separate the axis from
 * the measure; the name plus a constant-value probe settle it. A column that
 * never changes (the snapshot timestamp of a per-device listing) makes a
 * useless axis, which is why constant columns are only used as a last resort.
 */
export function inferMapping(fields: FieldMeta[], rows: Record<string, any>[], current: { value?: string; category?: string; series?: string } = {}) {
  const list = Array.isArray(fields) ? fields : []
  if (!list.length) return { value: current.value || 'value', category: current.category || '', series: current.series || '' }
  const header = Array.isArray(rows) && rows.length ? rows : []
  const isTime = (field: FieldMeta) => TIME_FIELD.test(field.id)
  const isNumber = (field: FieldMeta) => {
    if (isTime(field)) return false
    if (field.type === 'number') return true
    if (field.type && field.type !== 'number') return false
    const sample = header.find(row => row?.[field.id] != null)
    return !!sample && typeof sample[field.id] === 'number'
  }
  const varies = (field: FieldMeta) => header.length < 2 || header.some(row => row?.[field.id] !== header[0]?.[field.id])
  const numeric = list.filter(isNumber)
  const textual = list.filter(field => !isNumber(field))
  const ordered = (pool: FieldMeta[], prefer?: (field: FieldMeta) => boolean) => {
    const matching = pool.filter(field => field && prefer && prefer(field))
    const varying = pool.filter(field => varies(field))
    return [...new Set([...matching, ...varying, ...pool])]
  }
  const keep = (id: string | undefined, pool: FieldMeta[], fallback = '') => {
    if (id && list.some(field => field.id === id)) return id
    return (pool.length ? pool : list)[0]?.id || fallback
  }
  const value = keep(current.value, ordered(numeric.length ? numeric : list))
  const category = keep(current.category, ordered(textual.length ? textual : list, isTime))
  const seriesPool = ordered(textual.filter(field => field.id !== category && field.id !== value))
  // An explicit empty series means the author cleared it; only infer when the
  // caller never expressed a preference, otherwise a refresh keeps re-adding it.
  const series = current.series !== undefined
    ? (list.some(field => field.id === current.series) ? current.series : '')
    : (seriesPool.length === 1 ? seriesPool[0].id : '')
  return { value, category, series }
}

/** Whether the widget can only render a single result row. */
export function needsSingleRow(shape: string) { return isScalarShape(shape) }
