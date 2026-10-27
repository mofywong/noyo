import { reactive } from 'vue'

export const protocol = 'noyo.screen/1'
const query = new URLSearchParams(location.hash.split('?')[1] || '')
const channelId = query.get('channelId') || ''
let hostOrigin = location.origin
try { if (document.referrer) hostOrigin = new URL(document.referrer).origin } catch {}
const pending = new Map<string, { resolve: (value: any) => void; reject: (error: Error) => void; timer: ReturnType<typeof setTimeout> }>()
const listeners = new Map<string, Set<(value: any) => void>>()
let counter = 0
export const embedded = query.get('embed') === '1' && !!channelId && window.parent !== window
export const state = reactive({ initialized: false, mode: 'edit', locale: 'zh-CN', theme: 'dark', revision: 0, runtimeRevision: '', document: null as any, error: '', dirty: false, saving: false, lastFrameAt: 0, transportLost: false, capabilities: {} as Record<string, any> })
// Go serializes object keys in a different order from the canvas store. Dirty
// detection compares JSON values rather than incidental key insertion order.
export function documentSignature(value: any) {
  return JSON.stringify(value, (_key, item) => item && typeof item === 'object' && !Array.isArray(item) ? Object.fromEntries(Object.keys(item).sort().map(key => [key, item[key]])) : item)
}
// Author identity excludes adapter metadata and selection preferences. Go's
// omitempty also removes empty optional binding fields without an author edit.
// Component options remain exact: empty text and numeric zero can be intent.
export function canonicalAuthorDocument(value: any) {
  const bindings = Object.fromEntries(Object.entries(value?.bindings || {}).map(([id, source]) => {
    const binding = { ...(source as Record<string, any>) }
    for (const key of ['deviceCode', 'key', 'valueType', 'aggregate', 'latitudeKey', 'longitudeKey', 'locationSource', 'coordinateSystem', 'unit']) {
      if (binding[key] == null || binding[key] === '') delete binding[key]
    }
    for (const key of ['staleAfterMs', 'maxPoints', 'windowMs']) {
      if (binding[key] == null || binding[key] === 0) delete binding[key]
    }
    for (const key of ['deviceCodes', 'keys']) {
      if (binding[key] == null || Array.isArray(binding[key]) && !binding[key].length) delete binding[key]
    }
    if (binding.parameters == null || !Object.keys(binding.parameters).length) delete binding.parameters
    // Pointer values preserve meaningful zeroes, including scale and decimals.
    for (const key of ['query', 'dataset', 'mapping', 'scale', 'offset', 'decimals']) {
      if (binding[key] == null) delete binding[key]
    }
    if (binding.mapping) {
      binding.mapping = { ...binding.mapping }
      for (const key of ['value', 'category', 'series']) {
        if (binding.mapping[key] == null || binding.mapping[key] === '') delete binding.mapping[key]
      }
      if (binding.mapping.columns == null || Array.isArray(binding.mapping.columns) && !binding.mapping.columns.length) delete binding.mapping.columns
    }
    return [id, binding]
  }))
  const result = { ...value, bindings }
  if (value?.engine) {
    result.engine = { ...value.engine }
    delete result.engine.buildId
    delete result.engine.upstreamSha
  }
  if (value?.content) {
    result.content = { ...value.content }
    delete result.content.noyoAssistantContext
  }
  return result
}
export function assistantDocumentSignature(value: any) { return documentSignature(canonicalAuthorDocument(value)) }
// Some upstream renderers write live values into their option. Persist the
// saved placeholder for dataset bindings while retaining the author's styles.
export function restoreDatasetDefaults(document: any, content: any) {
  const flatten = (list: any[] = []): any[] => list.flatMap(item=>[item,...(item.isGroup?flatten(item.groupList):[])])
  const baseline = new Map(flatten(document?.content?.componentList).map(item=>[item.id,item]))
  const bound = new Set((document?.targets || []).filter((target:any)=>document.bindings?.[target.bindingId]?.kind==='dataset').map((target:any)=>target.componentId))
  for (const item of flatten(content.componentList)) {
    if (!bound.has(item.id) || !baseline.has(item.id) || !item.option) continue
    const original = baseline.get(item.id)?.option
    if (original && Object.hasOwn(original,'dataset')) item.option.dataset=JSON.parse(JSON.stringify(original.dataset))
    else delete item.option.dataset
  }
  return content
}
export function send(type: string, payload: any = {}, requestId = '') {
  if (embedded) parent.postMessage({ protocol, channelId, requestId, type, payload: clonePayload(payload) }, hostOrigin)
}
function clonePayload(value: any): any {
  if (Array.isArray(value)) return value.map(clonePayload)
  // Compiled proposals contain GoView class instances wrapped by Vue refs.
  // Copy their enumerable data too; keep upload Blobs as structured-clone data.
  if (value && Object.prototype.toString.call(value) === '[object Object]') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clonePayload(item)]))
  return value
}
export function rpc(type: string, payload: any = {}): Promise<any> {
  if (!embedded) return Promise.reject(new Error('The editor must be opened by Noyo'))
  const requestId = `${Date.now()}-${++counter}`
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => { pending.delete(requestId); reject(new Error(state.locale.startsWith('en') ? 'Noyo request timed out. Please retry.' : 'Noyo 请求超时，请重试。')) }, ['assistant.plan', 'assistant.task.resume'].includes(type) ? 780000 : 20000)
    pending.set(requestId, { resolve, reject, timer })
    try { send(type, payload, requestId) }
    catch (error) { clearTimeout(timer); pending.delete(requestId); reject(error) }
  })
}
export function on(type: string, callback: (value: any) => void) {
  const callbacks = listeners.get(type) || new Set()
  callbacks.add(callback); listeners.set(type, callbacks)
  return () => callbacks.delete(callback)
}
export function markDirty(dirty = true) { state.dirty = dirty; send('screen.dirty', { dirty }) }
function receive(event: MessageEvent) {
  const message = event.data
  if (event.source !== parent || event.origin !== hostOrigin || !message || message.protocol !== protocol || message.channelId !== channelId || typeof message.type !== 'string') return
  if (message.type === 'rpc.result') {
    const request = pending.get(message.requestId)
    if (!request) return
    clearTimeout(request.timer); pending.delete(message.requestId)
    message.payload?.ok ? request.resolve(message.payload.data) : request.reject(Object.assign(new Error(message.payload?.error?.message || 'Noyo request failed'), { code: message.payload?.error?.code }))
    return
  }
  if (message.type === 'host.dispose') {
    for (const request of pending.values()) { clearTimeout(request.timer); request.reject(new Error('Editor closed')) }
    pending.clear()
  }
  for (const callback of listeners.get(message.type) || []) callback(message.payload)
}
export function connect() { window.addEventListener('message', receive); send('child.ready'); return () => window.removeEventListener('message', receive) }
