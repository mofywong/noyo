import { ref, watch, nextTick, onMounted } from 'vue'
import { subscribeWidget } from '@/noyo/runtime'
import { shapeOf } from '@/noyo/widgets'

// Upstream components receive host data through this single hook. It replaces
// the upstream request/pond pipeline: no component-level HTTP, no timers.
//
// Two different contracts meet here:
//  - echarts components take a `dataset` (cartesian families) or mutate their
//    own option through the `update` callback (radar / heatmap / treemap / word
//    cloud). Upstream only repaints those inside its preview route, so the
//    repaint is issued here as well — otherwise a published Noyo screen would
//    keep showing placeholder data.
//  - VChart keeps its dataset as `{ values: [...] }` with per-chart field
//    names, so the generic host rows are remapped onto the component's own
//    category / value / series fields.
const clone = (value: any) => JSON.parse(JSON.stringify(value))

function fieldOf(candidates: any[], fallback: string) {
  for (const candidate of candidates) {
    if (Array.isArray(candidate) && typeof candidate[0] === 'string') return candidate[0]
    if (typeof candidate === 'string' && candidate) return candidate
  }
  return fallback
}

export const useChartDataFetch = (component: any, _store: any, update?: (value: any) => void) => {
  const vChartRef = ref<any>(null)
  const shape = shapeOf(component?.chartConfig?.key)
  let latest: any

  const echarts = () => component.chartConfig?.chartFrame === 'echarts'

  const applyVChart = () => {
    const option = component.option || {}
    const source = Array.isArray(latest?.source) ? latest.source : []
    const valueField = fieldOf([option.valueField, option.yField], 'value')
    const categoryField = fieldOf([option.categoryField, option.xField], 'category')
    const seriesField = typeof option.seriesField === 'string' ? option.seriesField : ''
    const keys = (latest?.dimensions || []).slice(1)
    const values = keys.length > 1
      ? source.flatMap((row: any) => keys.map((key: string) => ({ [categoryField]: row.ts, [valueField]: row[key] == null ? null : Number(row[key]) || 0, ...(seriesField ? { [seriesField]: key } : {}) })))
      : source.map((row: any) => ({ [categoryField]: row.ts, [valueField]: row[keys[0]] == null ? null : Number(row[keys[0]]) || 0, ...(seriesField ? { [seriesField]: keys[0] } : {}) }))
    option.dataset = { ...(option.dataset || {}), values }
  }

  const apply = () => {
    if (latest === undefined) return
    if (shape === 'vchart') { applyVChart(); return }
    if (!echarts()) { update?.(latest); return }
    // A component that consumes an `update` callback owns its option and does
    // not expose a chart ref (the ring gauge is one). Gating on the ref would
    // starve those families of data entirely.
    if (!vChartRef.value) { update?.(latest); return }
    if (shape === 'chart' || shape === 'pie') {
      const first = component.option?.series?.[0]
      const payload: any = { dataset: latest }
      if (shape === 'pie') {
        if (first) payload.series = [{ ...clone(first), encode: first.encode || { itemName: 0, value: 1 } }]
      } else if (first && first.type !== 'pie' && Array.isArray(latest?.dimensions)) {
        payload.series = Array.from({ length: Math.max(1, latest.dimensions.length - 1) }, () => clone(first))
      }
      vChartRef.value.setOption(payload, payload.series ? { replaceMerge: ['series'] } : undefined)
      // Only the pie family has a callback here (carousel bookkeeping); the
      // cartesian families must never mutate their saved option from live data.
      if (shape === 'pie') update?.(latest)
      return
    }
    // Families that own their option shape: let the component write the value
    // into its option, then repaint with the merged result.
    update?.(latest)
    try { vChartRef.value.setOption(component.option) } catch { /* component already repainted */ }
  }

  subscribeWidget(component.id, value => { latest = value; apply() })
  watch(vChartRef, () => nextTick(apply))
  onMounted(() => nextTick(apply))
  return { vChartRef }
}
