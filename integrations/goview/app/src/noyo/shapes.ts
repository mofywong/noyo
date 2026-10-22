// Pure shaping layer: one host sample in, one upstream-consumable payload out.
// No Vue and no bridge imports so the contract can be unit tested directly
// (see tooling/shapes.test.cjs).
import type { WidgetShape } from './widgets'

export interface ShapedBinding {
  kind?: string
  key?: string
  keys?: string[]
  aggregate?: string
  [field: string]: any
}

const english = (locale: string) => (locale || '').startsWith('en')

function num(value: any) {
  const parsed = typeof value === 'number' ? value : Number.parseFloat(String(value ?? ''))
  return Number.isFinite(parsed) ? parsed : null
}
function rowsOf(value: any) {
  return Array.isArray(value) ? value.filter(row => row && typeof row === 'object') : []
}
function columnsOf(rows: any[], binding: ShapedBinding) {
  if (binding.keys?.length) return binding.keys
  const keys = Object.keys(rows[0] || {}).filter(key => key !== 'ts')
  return keys.length ? keys : [binding.key || 'value']
}
const pad2 = (value: number) => String(value).padStart(2, '0')
// History rows carry epoch milliseconds. Rendering them raw turns every axis
// into thirteen unreadable digits, so any plausible timestamp is formatted.
function labelOf(row: any, index: number, compact = false) {
  const label = row?.ts ?? row?.time ?? row?.name
  if (typeof label === 'number' || (typeof label === 'string' && /^\d{11,13}$/.test(label))) {
    const stamp = new Date(Number(label))
    if (!Number.isNaN(stamp.getTime())) {
      const time = `${pad2(stamp.getHours())}:${pad2(stamp.getMinutes())}`
      // A 24h window repeats the same date on every tick; only the clock matters.
      return compact ? time : `${pad2(stamp.getMonth() + 1)}-${pad2(stamp.getDate())} ${time}`
    }
  }
  return label == null || label === '' ? String(index + 1) : String(label)
}
// Category axes are dense: 145 points over 24h would render "10-20 12:00" on
// every second tick and crowd the plot. A span under about a day drops the date.
function compactSpan(rows: any[]) {
  const stamps = rows.map(row => Number(row?.ts)).filter(value => Number.isFinite(value))
  if (stamps.length < 2) return false
  return Math.max(...stamps) - Math.min(...stamps) <= 27 * 3_600_000
}
// A single latest value is described as a one-point series so charts keep their
// axis instead of switching between two incompatible payload shapes.
function asSeries(value: any, binding: ShapedBinding) {
  const rows = rowsOf(value)
  if (Array.isArray(value) || value == null) return { rows, keys: columnsOf(rows, binding) }
  return { rows: [{ ts: '', [binding.key || 'value']: value }], keys: [binding.key || 'value'] }
}
// Legends read the model keys verbatim, so AI_L1_RESIDUAL_CURRENT and its three
// siblings collide into a scroll pager. Tokens every key shares describe the
// group rather than the series, so they are dropped from the display alias.
function compactLabels(keys: string[]) {
  if (keys.length < 2) return keys
  const split = keys.map(key => key.split('_'))
  const shared = split[0].filter(token => split.every(parts => parts.includes(token)))
  const stripped = split.map(parts => {
    const rest = [...parts]
    for (const token of shared) {
      const at = rest.indexOf(token)
      if (at >= 0) rest.splice(at, 1)
    }
    return rest.join('_')
  })
  const usable = stripped.every(label => label) && new Set(stripped).size === stripped.length
  return usable ? stripped : keys
}

// Alarms are objects, not samples. Every table family needs its own projection
// of the same record, so the mapping lives here instead of each component
// guessing column names from Object.keys().
const alarmSeverity: Record<string, [string, string]> = { critical: ['紧急', 'Critical'], major: ['严重', 'Major'], warning: ['警告', 'Warning'], minor: ['次要', 'Minor'], info: ['提示', 'Info'], firing: ['触发中', 'Firing'], recovered: ['已恢复', 'Recovered'], unknown: ['未知', 'Unknown'] }

// Fixed-width cells do not ellipsis, so long copy is trimmed here instead of
// spilling over the neighbouring column.
const clip = (value: any, limit: number) => {
  const text = String(value ?? '—')
  return text.length > limit ? `${text.slice(0, limit - 1)}…` : text
}

function alarmColumns(row: any, index: number, en: boolean) {
  const severity = row.severity || row.conditionStatus
  const occurred = row.occurredAt ?? row.lastOccurredAt ?? row.ts
  const stamp = occurred ? new Date(occurred) : null
  return {
    time: stamp && !Number.isNaN(stamp.getTime()) ? stamp.toLocaleTimeString() : labelOf(row, index),
    // The plant name says more than the device code, but the cell is narrow, so
    // whatever the host sends is trimmed to a readable stub.
    device: clip(row.deviceName || row.deviceCode, 8),
    title: clip(row.title, 14),
    severity: alarmSeverity[severity]?.[en ? 1 : 0] || severity || '—'
  }
}

// Scroll-board cells render HTML upstream. Event names and values remain text.
const eventEntities: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }
const eventCell = (value: any, limit: number) => clip(value, limit).replace(/[&<>"']/g, char => eventEntities[char])
function eventColumns(row: any, index: number, en: boolean) {
  const types: Record<string, [string, string]> = { info: ['信息', 'Info'], alert: ['告警', 'Alert'], error: ['故障', 'Error'] }
  const parameters = (Array.isArray(row.parameters) ? row.parameters : []).map((field: any) => {
    const label = (en ? field.en : field.zh) || field.key
    const value = typeof field.value === 'boolean' ? (en ? (field.value ? 'Yes' : 'No') : (field.value ? '是' : '否')) : field.value ?? '—'
    return `${label}: ${value}${field.unit ? ` ${field.unit}` : ''}`
  }).join('; ')
  return {
    time: eventCell(labelOf(row, index), 32),
    device: eventCell(row.deviceName || row.deviceCode, 16),
    title: eventCell((en ? row.eventEn : row.eventZh) || row.eventKey, 24),
    type: eventCell(types[row.eventType]?.[en ? 1 : 0] || row.eventType, 16),
    parameters: eventCell(parameters || '—', 160)
  }
}

export function shapeData(value: any, binding: ShapedBinding, shape: WidgetShape, locale: string) {
  if (binding.kind === 'dataset') return shapeDataset(value, binding, shape, locale)
  const en = english(locale)
  if (binding.kind === 'deviceStatus') {
    // A bare word carries no state at a glance and every status renders in the
    // same colour, so the glyph is the only channel that distinguishes them
    // without a per-component colour callback.
    const labels: Record<string, [string, string]> = { online: ['● 在线', '● Online'], offline: ['○ 离线', '○ Offline'], unknown: ['◌ 未知', '◌ Unknown'] }
    value = labels[value]?.[en ? 1 : 0] || value
  }
  if (shape === 'static') return value

  if (shape === 'chart' || shape === 'vchart') {
    const { rows, keys } = asSeries(value, binding)
    const labels = compactLabels(keys)
    // A 24h window ends on the same clock time it starts, so dropping the date
    // can produce a duplicate category. ECharts merges duplicate categories and
    // draws the series against the wrong points, so the short form is only used
    // when it stays unique.
    const short = rows.map((row, index) => labelOf(row, index, true))
    const compact = compactSpan(rows) && new Set(short).size === short.length
    // The category axis reads the `ts` column verbatim, so epoch milliseconds
    // are formatted here rather than rendering thirteen digits per tick.
    return {
      dimensions: ['ts', ...labels],
      source: rows.map((row, index) => {
        const item: any = { ts: labelOf(row, index, compact) }
        keys.forEach((key, at) => { item[labels[at]] = row[key] })
        return item
      })
    }
  }
  if (shape === 'pie') {
    const { rows, keys } = asSeries(value, binding)
    return { dimensions: ['name', 'value'], source: rows.map((row, index) => ({ name: labelOf(row, index), value: num(row[keys[0]]) })) }
  }
  if (shape === 'radar') {
    const { rows, keys } = asSeries(value, binding)
    const last = rows[rows.length - 1] || {}
    const indicator = keys.map(key => {
      const peak = Math.max(...rows.map(row => Math.abs(num(row[key]))), 0)
      return { name: key, max: peak > 0 ? peak * 1.2 : 1 }
    })
    return { radarIndicator: indicator, seriesData: [{ name: binding.key || 'value', value: keys.map(key => num(last[key])) }] }
  }
  if (shape === 'heatmap') {
    const { rows, keys } = asSeries(value, binding)
    return {
      xAxis: rows.map((row, index) => labelOf(row, index)),
      yAxis: keys,
      seriesData: rows.flatMap((row, index) => keys.map((key, column) => [index, column, num(row[key])]))
    }
  }
  if (shape === 'treemap' || shape === 'words') {
    const { rows, keys } = asSeries(value, binding)
    return rows.map((row, index) => ({ name: labelOf(row, index), value: num(row[keys[0]]) }))
  }
  if (shape === 'number') return num(value)
  // PieCircle multiplies by 100 itself; accept either a ratio or a percentage.
  if (shape === 'percent') { const ratio = num(value); return ratio == null ? null : Math.min(Math.max(binding.kind === 'deviceSummary' || binding.unit === '%' || ratio > 1 ? ratio / 100 : ratio, 0), 1) }
  if (shape === 'progress') { const amount = num(value); return amount == null ? null : Math.min(Math.max(binding.kind !== 'deviceSummary' && binding.unit !== '%' && amount > 0 && amount <= 1 ? amount * 100 : amount, 0), 100) }
  if (shape === 'table') {
    if (binding.kind === 'eventList' && Array.isArray(value)) {
      return value.map((row, index) => { const columns = eventColumns(row, index, en); return { name: `${columns.device} · ${columns.title}`, value: columns.parameters } })
    }
    if (binding.kind === 'alarmList' && Array.isArray(value)) {
      return value.map((row: any, index: number) => {
        const columns = alarmColumns(row, index, en)
        return { name: `${columns.device} · ${columns.title}`, value: columns.severity }
      })
    }
    const { rows, keys } = asSeries(value, binding)
    return rows.map((row, index) => ({ name: labelOf(row, index), value: row[keys[0]] ?? 0 }))
  }
  if (shape === 'table-basic') {
    if (binding.kind === 'eventList' && Array.isArray(value)) {
      return {
        dimensions: [
          { title: en ? 'Time' : '时间', key: 'time' },
          { title: en ? 'Device' : '设备', key: 'device' },
          { title: en ? 'Event' : '事件', key: 'title' },
          { title: en ? 'Type' : '类型', key: 'type' },
          { title: en ? 'Parameters' : '参数', key: 'parameters' }
        ],
        source: value.map((row, index) => eventColumns(row, index, en))
      }
    }
    if (binding.kind === 'alarmList' && Array.isArray(value)) {
      return {
        dimensions: [
          { title: en ? 'Time' : '时间', key: 'time' },
          { title: en ? 'Device' : '设备', key: 'device' },
          { title: en ? 'Alarm' : '告警', key: 'title' },
          { title: en ? 'Severity' : '级别', key: 'severity' }
        ],
        source: value.map((row: any, index: number) => alarmColumns(row, index, en))
      }
    }
    const { rows, keys } = asSeries(value, binding)
    const columns = ['ts', ...keys.filter(key => key !== 'ts')]
    return {
      dimensions: columns.map(key => ({ title: key === 'ts' ? (en ? 'Time' : '时间') : key, key })),
      source: rows.map((row, index) => {
        const item: any = {}
        for (const key of columns) item[key] = key === 'ts' ? labelOf(row, index) : row[key]
        return item
      })
    }
  }
  if (shape === 'table-grid') {
    if (binding.kind === 'eventList' && Array.isArray(value)) {
      return value.map((row, index) => Object.values(eventColumns(row, index, en)))
    }
    if (binding.kind === 'alarmList' && Array.isArray(value)) {
      return value.map((row: any, index: number) => {
        const columns = alarmColumns(row, index, en)
        return [columns.time, columns.device, columns.title, columns.severity]
      })
    }
    const { rows, keys } = asSeries(value, binding)
    return rows.map((row, index) => [labelOf(row, index), ...keys.map(key => String(row[key] ?? ''))])
  }
  if (shape === 'map') return value
  if (shape === 'text') return value == null ? '—' : value
  return value == null ? '—' : value
}

function shapeDataset(value: any, binding: ShapedBinding, shape: WidgetShape, locale: string): any {
  const rows = rowsOf(value), map = binding.mapping || {}, valueKey = map.value || 'value'
  const requested = [map.value,map.category,map.series,...(map.columns||[])].filter(Boolean)
  if (rows.length && requested.some(key=>!rows.some(row=>Object.hasOwn(row,key)))) throw new Error(english(locale) ? 'Mapped field is missing from the result' : '映射字段不在查询结果中')
  if (['number','percent','progress','text'].includes(shape)) {
    if (rows.length > 1) throw new Error(english(locale) ? 'Scalar widgets require one result row' : '单值组件需要一行查询结果')
    const scalar = rows[0]?.[valueKey]
    if (scalar == null) return shape === 'text' ? '—' : null
    if (shape === 'text') return String(scalar)
    if (typeof scalar !== 'number' || !Number.isFinite(scalar)) throw new Error(english(locale) ? 'Mapped value must be numeric' : '映射的数值字段必须为数字')
    return shapeData(scalar, {kind:'point'}, shape, locale)
  }
  const category = (row: any, index: number) => {
    if (!map.category) return String(index+1)
    if (map.category === 'ts' && typeof row.ts === 'number' && binding.resultTimezone) return new Intl.DateTimeFormat(english(locale)?'en-GB':'zh-CN',{timeZone:binding.resultTimezone,month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hourCycle:'h23'}).format(new Date(row.ts))
    return map.category === 'ts' ? labelOf({ts:row.ts},index) : String(row[map.category] ?? '—')
  }
  const columns: string[] = map.columns?.length ? map.columns : [valueKey]
  if (['chart','vchart','pie','treemap','words','radar','heatmap'].includes(shape) && rows.some(row=>(['pie','treemap','words'].includes(shape)?[valueKey]:columns).some(key=>row[key]!=null&&(typeof row[key]!=='number'||!Number.isFinite(row[key]))))) throw new Error(english(locale) ? 'Chart values must be numeric' : '图表数值字段必须为数字')
  if (shape === 'table-basic') {
    const keys = [...new Set<string>([...(map.category ? [map.category] : []),...columns])]
    return {dimensions:keys.map(key=>({title:key,key})),source:rows.map((row,index)=>Object.fromEntries(keys.map(key=>[key,key===map.category?category(row,index):row[key]??null])))}
  }
  if (shape === 'table-grid') return rows.map((row,index)=>[category(row,index),...columns.map(key=>row[key] == null ? '—' : String(row[key]))])
  if (['pie','treemap','words','table'].includes(shape)) {
    const items = rows.map((row,index)=>({name:category(row,index),value:row[valueKey]??null}))
    return shape === 'pie' ? {dimensions:['name','value'],source:items} : items
  }
  let data: any[] = rows.map((row,index)=>({ts:category(row,index),...Object.fromEntries(columns.map(key=>[key,row[key]??null]))})), keys=columns
  if (map.series) {
    const series = [...new Set<string>(rows.map(row=>String(row[map.series]??'—')))]
    if (series.includes('ts')) throw new Error(english(locale) ? 'Series name ts is reserved; rename it with a derive step' : '系列名 ts 为保留名称，请通过计算步骤重命名')
    if (series.length>64 || rows.length*series.length>100000) throw new Error(english(locale) ? 'Too many chart series' : '图表系列过多，请缩小范围')
    const groups = new Map<string,any>(), seen = new Set<string>()
    rows.forEach((row,index)=>{
      const label=category(row,index), groupKey=JSON.stringify(map.category?row[map.category]:index), name=String(row[map.series]??'—'), pair=JSON.stringify([groupKey,name])
      if (seen.has(pair)) throw new Error(english(locale) ? 'Aggregate duplicate category/series rows before mapping' : '同分类同系列存在多行，请先聚合')
      seen.add(pair)
      if (!groups.has(groupKey)) groups.set(groupKey,Object.fromEntries([['ts',label],...series.map(key=>[key,null])]))
      Object.defineProperty(groups.get(groupKey),name,{value:row[valueKey]??null,writable:true,enumerable:true,configurable:true})
    });data=[...groups.values()];keys=series
  }
  if (shape === 'chart' || shape === 'vchart') return {dimensions:['ts',...keys],source:data}
  if (shape === 'radar') {
    const last=data[data.length-1]||{}
    return {radarIndicator:keys.map(key=>({name:key,max:Math.max(1,...data.map(row=>Math.abs(typeof row[key]==='number'?row[key]:0)))*1.2})),seriesData:[{name:valueKey,value:keys.map(key=>last[key]??null)}]}
  }
  if (shape === 'heatmap') return {xAxis:data.map(row=>row.ts),yAxis:keys,seriesData:data.flatMap((row,index)=>keys.flatMap((key,column)=>row[key]==null?[]:[[index,column,row[key]]]))}
  return value
}
