// Development-only harness. It plays the part of the Noyo host so widgets can
// be mounted, fed a sample and inspected without a backend. `harness.html` is
// not a build input, so nothing here reaches the shipped bundle.
//
// Three modes:
//   ?batch=N&mode=edit|view   synthesized grids of every catalogued widget
//   ?dump=1                   export the reviewed default for every widget
//   ?assistant=1              isolated assistant UI with deterministic mock plans
//   ?assistant=1&context=1    two widgets for component/area context selection
//   window.__FIXTURE_*        render a real screen document with real samples
import { createPinia, setActivePinia } from 'pinia'

// Upstream config constructors reach into the edit store, so the harness needs
// an active Pinia before the material registry is imported. The registry is
// built at module scope, hence the dynamic import: a static import would run
// before this line.
setActivePinia(createPinia())
const { createMaterial, materials } = await import('./materials')
const { shapeOf, targetOf } = await import('./widgets')
type Material = (typeof materials)[number]

const PROTOCOL = 'noyo.screen/1'
const CHANNEL = 'verify'
const COLUMNS = 4
const ROWS = 3
const PER_PAGE = COLUMNS * ROWS

const params = new URLSearchParams(location.search)
const batch = Number(params.get('batch') || 0)
const dump = params.get('dump') === '1'
const assistant = params.get('assistant') === '1'
const locale = params.get('locale') === 'en' ? 'en' : 'zh-CN'
const theme = params.get('theme') === 'light' ? 'light' : 'dark'
const mode = assistant || params.get('mode') === 'edit' ? 'edit' : 'view'
const settleMs = Number(params.get('settle') || 2500)

const host = window as any
const fixtureDocument = host.__FIXTURE_DOCUMENT__ || null
const fixtureSamples = host.__FIXTURE_SAMPLES__ || null
const fixtureHistory = host.__FIXTURE_HISTORY__ || null

if (dump) {
  host.__HARNESS_DUMP__ = materials.map(material => {
    const component = createMaterial(material.key, 'zh-CN')
    component.id = `dump-${material.key}`
    return component
  })
  host.__HARNESS_READY__ = true
}

// ---------------------------------------------------------------- document
const pages: Material[][] = []
for (let index = 0; index < materials.length; index += PER_PAGE) pages.push(materials.slice(index, index + PER_PAGE))
const items: Material[] = pages[batch] || []

const kindForShape: Record<string, string> = {
  chart: 'history', pie: 'history', radar: 'history', heatmap: 'history',
  treemap: 'history', words: 'history', vchart: 'history',
  number: 'point', percent: 'point', progress: 'point',
  table: 'history', 'table-basic': 'history', 'table-grid': 'history',
  text: 'point', map: 'location'
}

const historyRows = Array.from({ length: 24 }, (_, index) => ({
  ts: `${String(index).padStart(2, '0')}:00`,
  temperature: Math.round((20 + Math.sin(index / 3) * 6) * 10) / 10
}))
const alarmRows = [
  { deviceCode: 'DEV-001', title: '高温告警', severity: 'critical', conditionStatus: 'firing' },
  { deviceCode: 'DEV-002', title: '通信中断', severity: 'major', conditionStatus: 'firing' },
  { deviceCode: 'DEV-003', title: '压力偏高', severity: 'warning', conditionStatus: 'recovered' }
]
const locationRows = [
  { deviceCode: 'DEV-001', longitude: 120.15, latitude: 30.28, coordinateSystem: 'wgs84', quality: 'good' },
  { deviceCode: 'DEV-002', longitude: 120.28, latitude: 30.2, coordinateSystem: 'wgs84', quality: 'good' },
  { deviceCode: 'DEV-003', longitude: 120.05, latitude: 30.35, coordinateSystem: 'wgs84', quality: 'stale' }
]

function sampleValue(shape: string, kind: string) {
  if (kind === 'history') return historyRows
  if (kind === 'alarmList') return alarmRows
  if (kind === 'location') return locationRows
  if (kind === 'deviceStatus') return 'online'
  switch (shape) {
    case 'percent': return 0.42
    case 'progress': return 68
    case 'text': return '24.6 °C'
    default: return 68
  }
}

function synthetic() {
  const canvasWidth = 1600, canvasHeight = 900
  const cellWidth = canvasWidth / COLUMNS, cellHeight = canvasHeight / ROWS
  const componentList: any[] = []
  const bindings: Record<string, any> = {}
  const targets: any[] = []
  const failures: string[] = []
  for (const [index, material] of items.entries()) {
    const id = `verify-${material.key}`
    let component: any
    try {
      component = createMaterial(material.key, 'zh-CN')
    } catch (error) {
      failures.push(`${material.key}: construct failed — ${(error as Error).message}`)
      continue
    }
    component.id = id
    component.attr = { ...component.attr, x: (index % COLUMNS) * cellWidth + 8, y: Math.floor(index / COLUMNS) * cellHeight + 8, w: cellWidth - 16, h: cellHeight - 16 }
    component.chartConfig.title = material.key
    componentList.push(component)
    const target = targetOf(material.key)
    if (!target) continue
    const shape = shapeOf(material.key)
    const kind = kindForShape[shape] || 'point'
    const bindingId = `bind-${material.key}`
    const binding: any = { kind, deviceCode: 'DEV-001' }
    if (kind === 'history') binding.keys = ['temperature']
    else if (kind === 'location') { binding.latitudeKey = 'latitude'; binding.longitudeKey = 'longitude'; binding.coordinateSystem = 'wgs84' }
    else binding.key = 'temperature'
    bindings[bindingId] = binding
    targets.push({ componentId: id, bindingId, target })
  }
  const samples: Record<string, any> = {}
  for (const [bindingId, binding] of Object.entries(bindings)) {
    if (binding.kind === 'history' || binding.kind === 'alarmList') continue
    samples[bindingId] = {
      value: sampleValue(shapeOf(bindingId.replace('bind-', '')), binding.kind),
      quality: 'good', receivedAt: Date.now(), sampleVersion: 1
    }
  }
  return {
    document: {
      id: 'verify', title: 'Widget harness', bindings, targets,
      content: {
        editCanvasConfig: { width: canvasWidth, height: canvasHeight, background: '', backgroundImage: '', selectColor: '#4a86ff' },
        componentList
      }
    },
    samples,
    failures,
    canvas: { width: canvasWidth, height: canvasHeight }
  }
}

const source = fixtureDocument
  ? {
      document: fixtureDocument,
      samples: fixtureSamples?.bindings || {},
      failures: [] as string[],
      canvas: { width: fixtureDocument.canvas?.width || 1920, height: fixtureDocument.canvas?.height || 1080 }
    }
  : synthetic()

let screenDocument = JSON.parse(JSON.stringify(source.document)), draftVersion = 1
const applyReceipts = new Map<string, any>()
if (assistant) { screenDocument.content.componentList = []; screenDocument.bindings = {}; screenDocument.targets = [] }
if (assistant && params.get('context') === '1') {
  screenDocument.content.componentList = [0, 1].map(index => {
    const item = createMaterial('TextCommon', locale)
    item.id = `context-${index + 1}`
    item.attr = { ...item.attr, x: 48 + index * 400, y: 48, w: 320, h: 120 }
    item.chartConfig.title = locale === 'en' ? `Context widget ${index + 1}` : `上下文组件 ${index + 1}`
    item.option.dataset = item.chartConfig.title
    return item
  })
}
if (assistant && host.__LIFECYCLE_FIXTURE__) {
  const note = createMaterial('TextCommon', locale); note.id = 'id_note'; Object.assign(note.attr, { x: 24, y: 24, w: 600, h: 180 }); Object.assign(note.option, { dataset: '原始说明', fontSize: 18, paddingX: 0, paddingY: 0 }); note.chartConfig.title = locale === 'en' ? 'Data notes' : '数据说明'
  const dial = createMaterial('Dial', locale); dial.id = 'id_dial'; Object.assign(dial.attr, { x: 664, y: 24, w: 280, h: 240 })
  const ring = createMaterial('PieCircle', locale); ring.id = 'id_ring'; Object.assign(ring.attr, { x: 24, y: 256, w: 280, h: 200 })
  screenDocument.content.componentList = [note, dial, ring]
  screenDocument.bindings = { dial: { kind: 'point', deviceCode: 'DEV-001', key: 'temperature' }, ring: { kind: 'point', deviceCode: 'DEV-001', key: 'temperature' } }
  screenDocument.targets = [{ componentId: 'id_dial', bindingId: 'dial' }, { componentId: 'id_ring', bindingId: 'ring' }]
  host.__ASSISTANT_RESULT__ = { summary: locale === 'en' ? 'Update data notes.' : '完善数据说明。', resultStatus: 'ready', intent: 'copy_style', operations: [{ type: 'update', id: 'id_note', option: { dataset: Array.from({ length: 9 }, (_, index) => `${index + 1}、${locale === 'en' ? 'Preserve the existing binding.' : '保留现有数据绑定。'}`).join('\n') } }] }
  host.__ASSISTANT_PROGRESS__ = [{ event: 'task', taskId: 'lifecycle-fixture', state: 'running', stage: 'reviewing', steps: ['intention', 'metrics', 'checking', 'generating', 'reviewing'].map(id => ({ id, state: 'succeeded' })) }]
}
const liveSamples = source.samples
const failures = source.failures
const tracked = (screenDocument.content?.componentList || []).map((component: any) => ({
  id: component.id,
  key: component.chartConfig?.key || '',
  group: component.chartConfig?.package || '',
  bound: (screenDocument.targets || []).some((target: any) => target.componentId === component.id)
}))

// ------------------------------------------------------------------ bridge
const stage = window.document.getElementById('stage') as HTMLElement
const frame = window.document.createElement('iframe')
frame.width = String(source.canvas.width)
frame.height = String(source.canvas.height + (mode === 'edit' ? 40 : 0))
frame.src = `./#/chart/${mode === 'edit' ? 'home' : 'preview'}/verify?embed=1&channelId=${CHANNEL}`
if (assistant) { frame.style.width = '100vw'; frame.style.height = '100vh' }
if (!dump) stage.appendChild(frame)

const status = { ready: false, initialized: false, rpcCalls: {} as Record<string, number> }
host.__HARNESS_ASSISTANT_REQUESTS__ = []
host.__HARNESS_RPC_CALLS__ = status.rpcCalls
host.__HARNESS_APPLIED__ = []
host.__HARNESS_SEND_RUNTIME__ = (value: number) => post({ protocol: PROTOCOL, channelId: CHANNEL, type: 'runtime.batch', payload: { revision: `draft-${draftVersion}`, runtimeEpoch: 'verify-epoch-1', serverTime: Date.now(), bindings: { dial: { value, quality: 'good', sampleVersion: Date.now() }, ring: { value, quality: 'good', sampleVersion: Date.now() } } } })

function post(message: any) {
  frame.contentWindow?.postMessage(message, location.origin)
}
function reply(requestId: string, payload: any) {
  post({ protocol: PROTOCOL, channelId: CHANNEL, requestId, type: 'rpc.result', payload })
}
function handleRequest(message: any) {
  const type = message.type
  status.rpcCalls[type] = (status.rpcCalls[type] || 0) + 1
  const requestId = message.requestId
  const conversationKey = `noyo-assistant-harness:${params.get('conversation') || 'verify'}`
  const sessionsKey = `${conversationKey}:sessions`
  const sessions = JSON.parse(localStorage.getItem(sessionsKey) || '[]')
  const messagesKey = (id: string) => id === 'legacy' ? conversationKey : `${conversationKey}:${id}`
  if (assistant && type === 'assistant.conversations') {
    const legacy = JSON.parse(localStorage.getItem(conversationKey) || '[]')
    const list = [...sessions]
    if (legacy.length) list.push({ id: 'legacy', title: legacy.find((item: any) => item.role === 'user')?.content || '', createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z' })
    const before = Number(message.payload.before || 0), items = list.slice(before, before + 2)
    return reply(requestId, { ok: true, data: { items, before: before + items.length, hasMore: before + items.length < list.length } })
  }
  if (assistant && type === 'assistant.conversation.create') {
    const now = new Date().toISOString()
    const session = { id: `session-${Date.now()}-${Math.random().toString(36).slice(2)}`, title: '', createdAt: now, updatedAt: now }
    sessions.unshift(session); localStorage.setItem(sessionsKey, JSON.stringify(sessions))
    return reply(requestId, { ok: true, data: session })
  }
  if (assistant && type === 'assistant.history') {
    if (host.__ASSISTANT_FAIL_HISTORY__ === message.payload.conversationId) return reply(requestId, { ok: false, error: { message: 'History unavailable' } })
    return reply(requestId, { ok: true, data: { items: JSON.parse(localStorage.getItem(messagesKey(message.payload.conversationId || 'legacy')) || '[]'), hasMore: false } })
  }
  if (assistant && type === 'assistant.task.current') return reply(requestId, { ok: true, data: null })
  if (assistant && type === 'assistant.message') {
    if (host.__ASSISTANT_FAIL_SAVE__) return reply(requestId, { ok: false, error: { message: 'Save unavailable' } })
    const id = message.payload.message.conversationId || 'legacy'
    const saved = JSON.parse(localStorage.getItem(messagesKey(id)) || '[]')
    const index = saved.findIndex((item: any) => item.id === message.payload.message.id)
    if (index < 0) saved.push(message.payload.message)
    else saved[index] = message.payload.message
    localStorage.setItem(messagesKey(id), JSON.stringify(saved))
    const session = sessions.find((item: any) => item.id === id)
    if (session && message.payload.message.role === 'user' && !session.title) {
      session.title = message.payload.message.content; localStorage.setItem(sessionsKey, JSON.stringify(sessions))
    }
    return reply(requestId, { ok: true, data: { saved: true } })
  }
  if (assistant && type === 'assistant.plan') {
    host.__HARNESS_ASSISTANT_REQUESTS__.push(message.payload)
    if (host.__ASSISTANT_RESULT__) {
      const packets = host.__ASSISTANT_PROGRESS__ || []
      packets.forEach((payload: any, index: number) => setTimeout(() => post({ protocol: PROTOCOL, channelId: CHANNEL, type: 'assistant.progress', payload: { runId: message.payload.runId, ...payload } }), index * 350))
      return setTimeout(() => reply(requestId, { ok: true, data: host.__ASSISTANT_RESULT__ }), packets.length * 350)
    }
    if (host.__ASSISTANT_REFUSE__) return reply(requestId, { ok: false, error: { message: 'aiOutOfScope' } })
    const exists = message.payload.document.content.componentList.some((item: any) => item.id === 'assistant-fixture')
    const title = locale === 'en' ? 'Cooling temperature' : '冷机温度'
    const progress = (payload: any) => post({ protocol: PROTOCOL, channelId: CHANNEL, type: 'assistant.progress', payload: { runId: message.payload.runId, ...payload } })
    progress({ event: 'stage', stage: 'planning' })
    setTimeout(() => progress({ event: 'summary', content: locale === 'en' ? 'Adding a temperature metric…' : '正在生成温度指标…' }), 200)
    setTimeout(() => reply(requestId, { ok: true, data: { summary: locale === 'en' ? 'Temperature metric created.' : '已生成温度指标。', operations: [{ type: exists ? 'update' : 'add', id: 'assistant-fixture', ...(exists ? {} : { key: 'TextCommon' }), x: 24, y: 24, w: 320, h: 120, title, option: { dataset: title, fontColor: 'var(--text-primary)' }, ...(message.payload.deviceCodes?.includes('DEV-001') ? { binding: { kind: 'point', deviceCode: 'DEV-001', key: 'temperature' } } : {}) }] } }), host.__ASSISTANT_PLAN_DELAY__ || 700)
    return
  }
  if (type === 'binding.catalog') {
    if (host.__FIXTURE_CATALOG__) return reply(requestId, { ok: true, data: host.__FIXTURE_CATALOG__ })
    return reply(requestId, { ok: true, data: { devices: [
      { code: 'DEV-001', name: '一号冷水机组', online: true, fields: [{ key: 'temperature', name: '温度', nameZh: '温度', nameEn: 'Temperature' }] },
      { code: 'DEV-002', name: '二号冷却塔', online: true, fields: [{ key: 'temperature', name: '温度', nameZh: '温度', nameEn: 'Temperature' }] }
    ] } })
  }
  if (type === 'binding.preview') {
    const bindings: Record<string, any> = {}
    for (const [id, binding] of Object.entries<any>(message.payload.document.bindings || {})) bindings[id] = binding.kind === 'dataset' ? { quality: 'good', value: [{ value: 24.6, state: 'Running', ts: Date.now() }], timezone: 'UTC' } : { quality: 'good', value: 24.6 }
    return reply(requestId, { ok: true, data: { bindings, serverTime: Date.now() } })
  }
  if (type === 'screen.apply') {
    const payload = message.payload, previous = applyReceipts.get(payload.idempotencyKey)
    if (previous) return reply(requestId, { ok: true, data: { ...previous, replayed: true } })
    if (payload.baseDraftVersion !== draftVersion) return reply(requestId, { ok: false, error: { code: 'conflict', message: 'Draft changed / 草稿已变化' } })
    screenDocument = JSON.parse(JSON.stringify(payload.document)); ++draftVersion
    const receipt = { document: screenDocument, draftVersion, currentDraftVersion: draftVersion, idempotencyKey: payload.idempotencyKey, committed: true }
    applyReceipts.set(payload.idempotencyKey, receipt); host.__HARNESS_APPLIED__.push({ ...payload, accepted: receipt })
    return reply(requestId, { ok: true, data: receipt })
  }
  if (type === 'screen.apply.status') return reply(requestId, { ok: true, data: applyReceipts.get(message.payload.idempotencyKey) || { committed: false } })
  // The guided dataset panel is only exercised end to end if preview answers,
  // and the shape of the answer drives the field mapping, so the mock honours
  // the grouping the caller asked for instead of returning a fixed table.
  if (type === 'dataset.preview') {
    const plan = message.payload?.plan || {}
    const aggregate = [...(plan.steps || [])].reverse().find((step: any) => step.op === 'aggregate') || {}
    const groupBy: string[] = aggregate.groupBy || []
    const to = Date.now(), from = to - (plan.windowMs || 86400000)
    const rows: any[] = []
    if (groupBy.includes('ts')) {
      for (let index = 0; index < 12; index += 1) rows.push({ ts: from + index * Math.round((to - from) / 12), value: Math.round((20 + Math.sin(index / 2) * 6) * 10) / 10 })
    } else if (groupBy.includes('device_code')) {
      rows.push({ device_code: 'DEV-001', value: 23.4 }, { device_code: 'DEV-002', value: 25.1 })
    } else if (groupBy.includes('severity')) {
      rows.push({ severity: 'critical', value: 3 }, { severity: 'warning', value: 12 }, { severity: 'info', value: 27 })
    } else if (groupBy.includes('online')) {
      rows.push({ online: true, value: 18 }, { online: false, value: 3 }, { online: null, value: 1 })
    } else {
      rows.push({ value: 24.6 })
    }
    const typeOf = (value: any) => typeof value === 'number' ? 'number' : typeof value === 'boolean' ? 'boolean' : 'string'
    const fields = Object.keys(rows[0] || { value: 0 }).map(id => ({ id, type: typeOf(rows[0][id]) }))
    return reply(requestId, { ok: true, data: {
      schemaVersion: 'noyo.dataset/1', fields, rows,
      meta: { quality: 'good', completeness: 'complete', generatedAt: to, from, to, timezone: plan.timezone || 'Asia/Shanghai', scannedRows: 1480, cacheHit: false, warnings: [] }
    } })
  }
  if (type === 'runtime.subscribe') return reply(requestId, { ok: true, data: { revision: 'draft-1' } })
  if (type === 'screen.save') { screenDocument = JSON.parse(JSON.stringify(message.payload?.document || screenDocument)); ++draftVersion; return reply(requestId, { ok: true, data: { document: screenDocument, draftVersion } }) }
  if (type === 'runtime.history' || type === 'runtime.alarms') {
    const bindingId = message.payload?.bindingId
    if (fixtureHistory?.[bindingId]) return reply(requestId, { ok: true, data: { bindingId, ...fixtureHistory[bindingId] } })
    const fallback = type === 'runtime.alarms' ? alarmRows : historyRows
    return reply(requestId, { ok: true, data: { bindingId, value: fallback, quality: 'good' } })
  }
  if (type === 'asset.resolve') return reply(requestId, { ok: true, data: { url: '' } })
  return reply(requestId, { ok: true, data: {} })
}

window.addEventListener('message', event => {
  if (event.source !== frame.contentWindow) return
  const message = event.data
  if (!message || message.protocol !== PROTOCOL || message.channelId !== CHANNEL) return
  if (message.type === 'child.ready') {
    status.ready = true
    post({ protocol: PROTOCOL, channelId: CHANNEL, type: 'host.init', payload: {
      mode,
      locale,
      theme,
      revision: 1,
      runtimeRevision: 'draft-1',
      capabilities: assistant ? { save: true, atomicApply: true, bindingPreview: true, proposalValidationV2: true } : {},
      assistantOpen: assistant,
      // The host speaks JSON only: upstream option defaults contain formatter
      // functions, which postMessage cannot clone.
      document: JSON.parse(JSON.stringify(screenDocument))
    } })
    return
  }
  if (message.type === 'child.initialized') {
    status.initialized = true
    setTimeout(() => post({ protocol: PROTOCOL, channelId: CHANNEL, type: 'runtime.batch', payload: {
      revision: 'draft-1',
      runtimeEpoch: 'verify-epoch-1',
      serverTime: Date.now(),
      bindings: liveSamples
    } }), 400)
    return
  }
  if (message.type === 'rpc.result') return
  if (message.requestId) return handleRequest(message)
})

// ----------------------------------------------------------------- collect
function canvasInk(element: Element) {
  const canvases = Array.from(element.querySelectorAll('canvas'))
  if (!canvases.length) return { count: 0, inked: false }
  let inked = false
  for (const canvas of canvases) {
    if (canvas.width < 2 || canvas.height < 2) continue
    const context = canvas.getContext('2d')
    if (!context) { inked = true; continue }
    try {
      const { data } = context.getImageData(0, 0, canvas.width, canvas.height)
      for (let index = 3; index < data.length; index += 4 * 53) if (data[index] > 10) { inked = true; break }
    } catch { inked = true }
    if (inked) break
  }
  return { count: canvases.length, inked }
}

function collect() {
  const frameDocument = frame.contentDocument
  if (!frameDocument) return { error: 'no frame document' }
  const origin = frame.getBoundingClientRect()
  const widgets = tracked.map((entry: any) => {
    // Edit mode tags each item with data-id; the preview list uses the id
    // attribute. Both are the same component identity.
    const element = frameDocument.getElementById(entry.id) || frameDocument.querySelector(`[data-id="${entry.id}"]`)
    if (!element) return { ...entry, missing: true }
    // Edit mode wraps the widget in a zero-sized positioning box, so the box
    // that actually paints is the largest descendant.
    let rect = element.getBoundingClientRect()
    if (rect.width < 4 || rect.height < 4) {
      let area = 0
      for (const node of element.querySelectorAll('*')) {
        const candidate = node.getBoundingClientRect()
        if (candidate.width * candidate.height > area) { area = candidate.width * candidate.height; rect = candidate }
      }
    }
    const ink = canvasInk(element)
    const images = Array.from(element.querySelectorAll('img'))
    const text = (element.textContent || '').replace(/\s+/g, ' ').trim()
    return {
      ...entry,
      rect: { x: Math.round(origin.left + rect.left), y: Math.round(origin.top + rect.top), w: Math.round(rect.width), h: Math.round(rect.height) },
      canvas: ink.count,
      canvasInked: ink.inked,
      svg: element.querySelectorAll('svg').length,
      img: images.length,
      imgLoaded: images.filter(image => image.complete && image.naturalWidth > 0).length,
      nodes: element.querySelectorAll('*').length,
      textLength: text.length,
      text: text.slice(0, 80)
    }
  })
  const syntheticPalette = mode === 'edit' && !fixtureDocument
  const palette = syntheticPalette ? Array.from(frameDocument.querySelectorAll('.palette button')).map(button => (button.textContent || '').trim()) : []
  const paletteGroups = syntheticPalette ? frameDocument.querySelectorAll('.palette h3').length : 0
  return {
    widgets, paletteCount: palette.length, paletteGroups,
    panelHosts: frameDocument.querySelectorAll('.panel-host').length,
    panelsAvailable: materials.filter(material => !!material.panel).length,
    paletteSample: palette.slice(0, 6)
  }
}

const errors: string[] = []
window.addEventListener('error', event => errors.push(String(event.message)))
frame.addEventListener('load', () => {
  frame.contentWindow?.addEventListener('error', event => errors.push(`widget: ${event.message}`))
  frame.contentWindow?.addEventListener('unhandledrejection', (event: any) => errors.push(`reject: ${event.reason}`))
})

if (dump) throw new Error('__dump_done__')

const started = Date.now()
const timer = setInterval(() => {
  if (!status.initialized) return
  clearInterval(timer)
  setTimeout(() => {
    host.__HARNESS_RESULT__ = { batch, mode, fixture: !!fixtureDocument, total: materials.length, pages: pages.length, items: tracked.length, failures, errors, rpcCalls: status.rpcCalls, elapsed: Date.now() - started, ...collect() }
    host.__HARNESS_READY__ = true
  }, settleMs)
}, 100)
setTimeout(() => {
  if (!host.__HARNESS_READY__) {
    host.__HARNESS_RESULT__ = { batch, mode, fixture: !!fixtureDocument, total: materials.length, items: tracked.length, failures, errors, ready: status.ready, initialized: status.initialized, timeout: true }
    host.__HARNESS_READY__ = true
  }
}, settleMs + 20000)
