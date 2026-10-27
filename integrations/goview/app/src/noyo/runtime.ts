import { onUnmounted } from 'vue'
import { assistantDocumentSignature, on, state } from './bridge'
import { shapeOf, type WidgetShape } from './widgets'
import { shapeData } from './shapes'

const callbacks = new Map<string, Set<(value: any) => void>>()
const statusCallbacks = new Map<string, Set<(status: any) => void>>()
const latest = new Map<string, any>()
const latestContracts = new Map<string, string>()
const targetIndex = new Map<string, any[]>()
let targetIndexSource: any = null, targetIndexLength = -1
const versions = new Map<string, number>()
// componentId -> shape, built once per document instead of once per target.
const shapeIndex = new Map<string, WidgetShape>()
let shapeIndexSource: any = null
let currentEpoch: any = null
export function subscribeWidget(id: string, callback: (value: any) => void) {
  const set = callbacks.get(id) || new Set()
  set.add(callback); callbacks.set(id, set)
  if (latest.has(id) && latestContracts.get(id) === widgetContract(id)) callback(latest.get(id))
  onUnmounted(() => { set.delete(callback); if (!set.size) callbacks.delete(id) })
}
// Quality is delivered on a separate channel: a widget must be able to render
// "stale" or "access denied" without inventing a numeric placeholder.
export function subscribeWidgetStatus(id: string, callback: (status: any) => void) {
  const set = statusCallbacks.get(id) || new Set()
  set.add(callback); statusCallbacks.set(id, set)
  return () => { set.delete(callback); if (!set.size) statusCallbacks.delete(id) }
}
const qualityLabels: Record<string, [string, string]> = {
  good: ['正常', 'Good'],
  stale: ['数据过期', 'Stale'],
  offline: ['设备离线', 'Offline'],
  no_data: ['暂无数据', 'No data'],
  access_denied: ['无权限', 'Access denied'],
  error: ['数据异常', 'Error']
}
export function qualityLabel(quality: string, bindingId?: string) {
  const binding = state.document?.bindings?.[bindingId || '']
  const label = quality === 'no_data' && binding?.kind === 'deviceSummary' && ['online', 'offline', 'onlineRate'].includes(binding.aggregate)
    ? ['含未知状态', 'Includes unknown states']
    : qualityLabels[quality]
  if (!label) return ['—', '—']
  return (state.locale || '').startsWith('en') ? [label[1], label[1]] : label
}

const english = () => (state.locale || '').startsWith('en')

function dataFor(value: any, binding: any, shape: WidgetShape) {
  return shapeData(value, binding, shape, state.locale)
}

export function screenComponents(list: any[] = []): any[] {
  return list.flatMap(item => [item, ...(item.isGroup && Array.isArray(item.groupList) ? screenComponents(item.groupList) : [])])
}

function shapeFor(componentId: string): WidgetShape {
  const list = state.document?.content?.componentList
  // A batch can carry dozens of targets; rebuilding the flattened component list
  // for each of them made every tick quadratic in component count.
  if (list !== shapeIndexSource) {
    shapeIndexSource = list
    shapeIndex.clear()
    for (const item of screenComponents(list || [])) if (item?.id) shapeIndex.set(item.id, shapeOf(item?.chartConfig?.key))
  }
  return shapeIndex.get(componentId) || 'static'
}

function widgetContract(id: string) {
  const all = state.document?.targets || []
  if (all !== targetIndexSource || all.length !== targetIndexLength) {
    targetIndexSource = all; targetIndexLength = all.length; targetIndex.clear()
    for (const target of all) { const list = targetIndex.get(target.componentId) || []; list.push(target); targetIndex.set(target.componentId, list) }
  }
  const targets = targetIndex.get(id) || []
  const bindings = Object.fromEntries(targets.map(target => [target.bindingId, state.document?.bindings?.[target.bindingId]]))
  return assistantDocumentSignature({ shape: shapeFor(id), bindings, targets })
}
export function deliverWidget(id: string, value: any) { latest.set(id, value); latestContracts.set(id, widgetContract(id)); for (const callback of callbacks.get(id) || []) callback(value) }
// Repaint a detached render model after its authored configuration changes.
export function replayWidget(id: string) { if (latest.has(id) && latestContracts.get(id) === widgetContract(id)) for (const callback of callbacks.get(id) || []) callback(latest.get(id)) }
function deliverStatus(id: string, status: any) { for (const callback of statusCallbacks.get(id) || []) callback(status) }
function sampleQuality(quality = 'good', shaped: any, binding: any) {
  if (quality !== 'good') return quality
  if (shaped == null || shaped === '—') return 'no_data'
  // Empty alarm feeds are a valid "no incidents" result. Historical/query
  // collections instead need an explicit empty state beside the blank chart.
  if (binding.kind === 'history' || binding.kind === 'eventList' || binding.kind === 'dataset') {
    if (Array.isArray(shaped) && !shaped.length) return 'no_data'
    if (Array.isArray(shaped.source) && !shaped.source.length) return 'no_data'
  }
  return quality
}
export function deliverBinding(bindingId: string, value: any, quality = 'good', metadata: any = {}) {
  const binding = state.document?.bindings?.[bindingId]
  if (!binding) return
  for (const target of state.document?.targets || []) {
    if (target.bindingId !== bindingId) continue
    let currentQuality = quality, message = metadata.message
    try {
      const shaped = dataFor(value, {...binding,resultTimezone:metadata.meta?.timezone}, shapeFor(target.componentId))
      deliverWidget(target.componentId, shaped)
      currentQuality = sampleQuality(quality, shaped, binding)
    } catch(error) { currentQuality = 'error'; message = (error as Error).message; deliverWidget(target.componentId, null) }
    deliverStatus(target.componentId, { ...metadata, quality:currentQuality, message, bindingId })
  }
}
on('runtime.batch', batch => {
  if (!state.initialized || batch?.revision !== state.runtimeRevision) return
  state.transportLost = false
  state.lastFrameAt = Number(batch.serverTime) || Date.now()
  if (batch.runtimeEpoch !== currentEpoch) { versions.clear(); currentEpoch = batch.runtimeEpoch }
  for (const target of state.document?.targets || []) {
    const binding = state.document.bindings?.[target.bindingId]
    const sample = batch.bindings?.[target.bindingId]
    if (!binding || !sample) continue
    const key = `${target.componentId}:${target.bindingId}`
    const version = Number(sample.sampleVersion || 0)
    if (version && version < (versions.get(key) || 0)) continue
    // An identical sample would only make the chart redraw the same picture.
    // The status is delivered anyway: a device can go offline without its
    // telemetry counter moving.
    const unchanged = version > 0 && version === versions.get(key) && latestContracts.get(target.componentId) === widgetContract(target.componentId)
    if (version) versions.set(key, version)
    if (!unchanged) deliverWidget(target.componentId, dataFor(sample.value, binding, shapeFor(target.componentId)))
    const quality = sampleQuality(sample.quality, latest.get(target.componentId), binding)
    deliverStatus(target.componentId, {
      quality,
      receivedAt: sample.receivedAt,
      bindingId: target.bindingId,
      label: qualityLabel(quality, target.bindingId)[0]
    })
  }
  // Bindings the viewer may not read never arrive as samples; the host reports
  // them once per snapshot so the component can say "access denied" instead of
  // pretending there is no data.
  for (const bindingId of batch.deniedBindings || []) for (const target of state.document?.targets || []) if (target.bindingId === bindingId) deliverStatus(target.componentId, { quality: 'access_denied', receivedAt: batch.serverTime, bindingId, label: english() ? qualityLabels.access_denied[1] : qualityLabels.access_denied[0] })
})
on('host.dispose', () => { callbacks.clear(); statusCallbacks.clear(); latest.clear(); latestContracts.clear(); versions.clear(); shapeIndex.clear(); shapeIndexSource = null; targetIndex.clear(); targetIndexSource = null; targetIndexLength = -1; state.lastFrameAt = 0; state.transportLost = false })
// The socket lives in the host page, so this is the only way the canvas can learn
// that the numbers on it stopped arriving. Without it a dead link renders as a
// perfectly healthy screen that simply never changes.
on('host.runtimeState', payload => {
  // Only "lost" means the feed stopped. "idle" is a deliberate stop (autosave
  // restarts the stream, unmount, revocation) and "connecting" is the recovery
  // path itself, so neither may repaint a healthy canvas as stale.
  const lost = payload?.state === 'lost'
  if (lost === state.transportLost) return
  state.transportLost = lost
  // No frame has ever arrived: the widgets have nothing to be stale about yet.
  if (!lost || !state.lastFrameAt) return
  // Values are kept on purpose — an operator needs the last known reading, but
  // every widget must say that it is no longer current.
  for (const target of state.document?.targets || []) {
    deliverStatus(target.componentId, {
      quality: 'stale',
      bindingId: target.bindingId,
      receivedAt: state.lastFrameAt,
      message: english() ? 'Live connection lost' : '实时连接中断'
    })
  }
})
on('runtime.revoked', () => {
  for (const bindingId of Object.keys(state.document?.bindings || {})) deliverBinding(bindingId, null, 'access_denied')
  versions.clear()
})
