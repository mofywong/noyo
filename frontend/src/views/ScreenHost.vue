<template>
  <section ref="hostElement" class="screen-host" :class="{ 'screen-host-full': fullscreen }" :aria-label="t('screens.title')">
    <header class="screen-host-toolbar">
      <div class="screen-host-identity">
        <RouterLink class="btn btn-outline-secondary btn-sm" to="/screens" :aria-label="t('screens.back')"><i class="bi bi-arrow-left" aria-hidden="true"></i></RouterLink>
        <div><h1>{{ screen?.name || t('screens.title') }}</h1><p>{{ t(`screens.${mode}Mode`) }} <span v-if="version"> · {{ t('screens.draftLabel', { version }) }}</span></p></div>
      </div>
      <div class="screen-host-actions">
        <span v-if="mode === 'edit'" class="screen-host-save-state" aria-live="polite">{{ t(saving ? 'screens.saving' : dirty ? 'screens.unsaved' : 'screens.saved') }}</span>
        <LiquidGlassButton v-if="mode === 'edit' && auth.hasPermission('screen:edit')" ref="assistantButton" class="screen-host-assistant" :variant="assistantOpen ? 'outline-primary' : 'outline-secondary'" size="sm" icon="bi bi-stars" :aria-label="t(assistantOpen ? 'screens.aiAssistantClose' : 'screens.aiAssistantOpen')" :aria-pressed="assistantOpen" :disabled="!ready || conflict" @click="send('host.assistant')">{{ t('screens.aiAssistant') }}</LiquidGlassButton>
        <LiquidGlassButton v-if="mode === 'edit'" variant="outline-secondary" size="sm" :disabled="!ready || saving || conflict" @click="send('host.save')">{{ t('screens.save') }}</LiquidGlassButton>
        <LiquidGlassButton v-if="mode === 'edit' && auth.hasPermission('screen:view')" variant="outline-secondary" size="sm" @click="router.push(`/screens/${id}/preview`)">{{ t('screens.preview') }}</LiquidGlassButton>
        <LiquidGlassButton v-if="mode === 'edit' && auth.hasPermission('screen:publish')" variant="primary" size="sm" :disabled="!ready || dirty || saving || conflict" @click="publishCurrent">{{ t('screens.publish') }}</LiquidGlassButton>
        <LiquidGlassButton variant="outline-secondary" size="sm" @click="toggleFullscreen">{{ t(fullscreen ? 'screens.exitFullscreen' : 'screens.fullscreen') }}</LiquidGlassButton>
      </div>
    </header>
    <div v-if="error" class="screen-host-alert" role="alert"><span>{{ error }}</span><button type="button" class="btn btn-outline-secondary btn-sm" @click="reload">{{ t('screens.reload') }}</button></div>
    <div v-if="runtimeError && !error" class="screen-host-runtime" role="status">{{ runtimeError }}</div>
    <div v-if="loading" class="screen-host-empty" role="status">{{ t('screens.starting') }}</div>
    <div v-else-if="!src" class="screen-host-empty"><i class="bi bi-window-x" aria-hidden="true"></i><h2>{{ t('screens.unavailable') }}</h2><p>{{ t('screens.unavailableHint') }}</p><LiquidGlassButton variant="outline-secondary" @click="reload">{{ t('screens.retry') }}</LiquidGlassButton></div>
    <iframe v-if="src" ref="frame" :key="channelId" :src="src" :title="`${t('screens.title')} · ${screen?.name || ''}`" class="screen-host-frame" referrerpolicy="no-referrer" allow="fullscreen" @load="frameLoaded" />
    <div v-if="src && !ready && !error" class="screen-host-connecting" role="status">{{ t('screens.starting') }}</div>
  </section>
</template>

<script setup>
import axios from 'axios'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { useConfirm } from '../composables/useConfirm.js'
import { useScreenRuntime } from '../composables/useScreenRuntime.js'
import { assistantTaskRequest, createAssistantTask, pollAssistantTask } from '../screens/assistantStream.js'
import { bridgeError, engineUrl, isScreenMessage, screenMessage, SCREEN_RPC_TYPES } from '../screens/bridge.js'
import { SCREEN_ENGINE, SCREEN_PROTOCOL, screensApi, screenRequest, draftVersion, errorKey, requestId, screenId } from '../screens/api.js'

const route = useRoute(), router = useRouter(), auth = useAuthStore(), { t, locale } = useI18n(), { confirmDialog } = useConfirm()
const id = computed(() => String(route.params.id)), mode = computed(() => route.meta.screenMode || 'edit')
const frame = ref(null), hostElement = ref(null), src = ref(''), channelId = ref(requestId()), loading = ref(true), ready = ref(false)
const screen = ref(null), documentState = ref(null), release = ref(null), version = ref(0)
const editorBuildId = ref('')
const assistantOpen = ref(false), assistantButton = ref(null)
const saving = ref(false), dirty = ref(false), conflict = ref(false), error = ref(''), runtimeError = ref(''), fullscreen = ref(false)
const pending = new Set(), assetUrls = new Map(), runtime = useScreenRuntime()
const runtimeWanted = ref(false)
let handshakeTimer, observer, loadController, disposed = false, bootSequence = 0

// The revision the child compares against; it must match the server's runtime
// revision exactly (draft-N for a draft mount, the release id for playback).
const runtimeRevision = computed(() => release.value ? screenId(release.value) : (version.value ? `draft-${version.value}` : ''))

function send(type, payload, correlation) {
  frame.value?.contentWindow?.postMessage(screenMessage(channelId.value, type, payload, correlation), window.location.origin)
}
function environment() {
  const style = getComputedStyle(document.documentElement)
  const tokens = {}
  for (const name of ['--bg-body', '--bg-surface', '--text-primary', '--text-secondary', '--border-color', '--accent-color', '--color-success', '--color-warning', '--color-danger', '--chart-1', '--chart-2', '--chart-3', '--chart-4', '--chart-5', '--chart-6']) tokens[name] = style.getPropertyValue(name).trim()
  return { locale: locale.value.startsWith('zh') ? 'zh' : 'en', theme: document.documentElement.dataset.bsTheme === 'light' ? 'light' : 'dark', tokens }
}
function init() {
  send('host.init', { screenId: id.value, mode: mode.value, editorBuildId: editorBuildId.value, assistantOpen: route.query.assistant === '1', ...environment(), capabilities: { save: mode.value === 'edit' && auth.hasPermission('screen:edit'), publish: mode.value === 'edit' && auth.hasPermission('screen:publish'), runtime: true, upload: mode.value === 'edit' && auth.hasPermission('screen:edit'), unsafeCode: false }, document: documentState.value, revision: version.value, runtimeRevision: runtimeRevision.value, releaseId: release.value ? screenId(release.value) : null })
}
function stopRuntime() { runtime.stop(); runtimeError.value = '' }
function cleanup() {
  assistantController?.abort(); stopRuntime(); clearTimeout(handshakeTimer); loadController?.abort(); pending.clear(); send('host.dispose')
  for (const url of assetUrls.values()) URL.revokeObjectURL(url)
  assetUrls.clear()
}
async function boot() {
  const sequence = ++bootSequence
  runtimeWanted.value = false
  cleanup(); src.value = ''; ready.value = false; assistantOpen.value = false; loading.value = true; saving.value = false; error.value = ''; conflict.value = false; dirty.value = false; channelId.value = requestId()
  loadController = new AbortController()
  try {
    const [manifestResponse, loaded] = await Promise.all([
      axios.get('/screen-editor/manifest.json', { signal: loadController.signal }),
      screensApi.load(id.value, mode.value, mode.value === 'view' ? route.query.releaseId : undefined),
    ])
    if (disposed || sequence !== bootSequence) return
    const manifest = manifestResponse.data
    if (manifest.protocol !== SCREEN_PROTOCOL || !manifest.buildId || !manifest.entry) throw new Error('Invalid editor manifest')
    const requiredEngine = loaded.release?.engineBuildId || loaded.document?.engine?.buildId || SCREEN_ENGINE
    // The current adapter understands older schema-1 drafts. Editing uses its
    // new tools; release playback keeps the adapter pinned to that release.
    editorBuildId.value = mode.value === 'edit' && loaded.document?.schemaVersion === SCREEN_PROTOCOL && manifest.buildId === SCREEN_ENGINE ? manifest.buildId : requiredEngine
    const entry = manifest.buildId === editorBuildId.value ? manifest.entry : `/screen-editor/${encodeURIComponent(editorBuildId.value)}/`
    screen.value = loaded.screen; documentState.value = loaded.document; release.value = loaded.release || null
    version.value = loaded.draftVersion ?? loaded.release?.sourceDraftVersion ?? draftVersion(loaded.screen)
    src.value = engineUrl(entry, id.value, mode.value, channelId.value, window.location.origin)
    await nextTick()
    handshakeTimer = setTimeout(() => { if (!ready.value) error.value = t('screens.handshakeFailed') }, 30000)
  } catch (err) {
    if (disposed || sequence !== bootSequence) return
    error.value = err?.response?.status === 404 && mode.value === 'view' ? t('screens.publishedUnavailable') : t(`screens.${errorKey(err)}`)
  } finally { if (sequence === bootSequence) loading.value = false }
}
async function reload() { if (await canLeave()) await boot() }
function frameLoaded() { if (!ready.value) send('host.ping') }

function ensureEdit(permission = 'edit') {
  if (mode.value !== 'edit' || !auth.hasPermission(`screen:${permission}`)) {
    const err = new Error('Access denied'); err.response = { status: 403 }; throw err
  }
}
function currentRevision() { return release.value ? { revision: screenId(release.value) } : { revision: mode.value === 'view' ? 'published' : 'draft', draftVersion: version.value } }
async function publishCurrent() {
  ensureEdit('publish')
  if (dirty.value || saving.value) { error.value = t('screens.saveFirst'); return }
  if (!(await confirmDialog({ title: t('screens.publishTitle'), message: t('screens.publishMessage'), variant: 'neutral', confirmText: t('screens.publish'), cancelText: t('screens.cancel') }))) return
  try { const result = await screensApi.publish(id.value, version.value); send('host.published', { release: result }); error.value = '' }
  catch (err) { error.value = t(`screens.${errorKey(err)}`) }
}
async function saveDocument(payload) {
  ensureEdit()
  if (saving.value || conflict.value) { const err = new Error('Draft conflict'); err.response = { status: 412 }; throw err }
  if (payload.baseDraftVersion != null && Number(payload.baseDraftVersion) !== version.value) { const err = new Error('Draft conflict'); err.response = { status: 412 }; throw err }
  if (!payload.document || typeof payload.document !== 'object') throw new Error('Invalid document')
  saving.value = true
  const mountChannel = channelId.value
  try {
    const result = await screensApi.save(id.value, payload.document, version.value)
    if (disposed || mountChannel !== channelId.value) return result
    documentState.value = result.document; version.value = result.draftVersion; screen.value = result.screen; dirty.value = false; error.value = ''
    if (runtimeWanted.value) startRuntime()
    return result
  } catch (err) {
    if (mountChannel === channelId.value) {
      if (errorKey(err) === 'conflict') { conflict.value = true; stopRuntime() }
      error.value = t(`screens.${errorKey(err)}`)
    }
    throw err
  } finally { if (mountChannel === channelId.value) saving.value = false }
}
async function uploadAsset(payload) {
  ensureEdit()
  const file = payload.file || payload.blob
  if (!(file instanceof Blob) || !['image/png', 'image/jpeg', 'image/webp'].includes(file.type) || file.size > 10 * 1024 * 1024) throw new Error('Invalid asset')
  const form = new FormData(); form.append('file', file, payload.name || 'screen-image')
  return screenRequest('post', `/${encodeURIComponent(id.value)}/assets`, form)
}
async function resolveAsset(payload) {
  const assetId = String(payload.assetId || '')
  if (!assetId || !/^[a-zA-Z0-9_-]+$/.test(assetId)) throw new Error('Invalid asset')
  const thumbnail = payload.thumbnail === true, cacheKey = `${assetId}:${thumbnail ? 'thumbnail' : 'full'}`
  if (!assetUrls.has(cacheKey)) {
    const response = await axios.get(`/api/screens/${encodeURIComponent(id.value)}/assets/${encodeURIComponent(assetId)}`, { params: { ...currentRevision(), ...(thumbnail ? { thumbnail: true } : {}) }, responseType: 'blob' })
    assetUrls.set(cacheKey, URL.createObjectURL(response.data))
  }
  return { assetId, url: assetUrls.get(cacheKey) }
}
async function rpc(type, payload) {
  switch (type) {
    case 'screen.load': return { document: documentState.value, draftVersion: version.value, screen: screen.value, release: release.value }
    case 'screen.save': return saveDocument(payload)
    case 'screen.validate': ensureEdit(); return screensApi.validate(id.value, payload.document || documentState.value, version.value)
    case 'assistant.plan': return planWithProgress(payload)
    case 'assistant.task.current': ensureEdit(); return screenRequest('get', `/${encodeURIComponent(id.value)}/assistant/tasks`, null, { params: { conversationId: payload.conversationId || '' } })
    case 'assistant.task.resume': return planWithProgress(payload, true)
    case 'assistant.task.cancel': {
      ensureEdit()
      const status = await screenRequest('post', `/${encodeURIComponent(id.value)}/assistant/tasks/${encodeURIComponent(payload.taskId)}/cancel`, {})
      if (status.state === 'canceled') assistantController?.abort()
      return status
    }
    case 'assistant.conversations': ensureEdit(); return screenRequest('get', `/${encodeURIComponent(id.value)}/assistant/conversations`, null, { params: { before: payload.before || undefined } })
    case 'assistant.conversation.create': ensureEdit(); return screenRequest('post', `/${encodeURIComponent(id.value)}/assistant/conversations`, {})
    case 'assistant.history': ensureEdit(); return screenRequest('get', `/${encodeURIComponent(id.value)}/assistant/messages`, null, { params: { conversationId: payload.conversationId, before: payload.before || undefined } })
    case 'assistant.message': ensureEdit(); return screenRequest('post', `/${encodeURIComponent(id.value)}/assistant/messages`, payload.message)
    case 'screen.publish': ensureEdit('publish'); if (dirty.value || saving.value) throw new Error('Save first'); return screensApi.publish(id.value, version.value)
    case 'asset.upload': return uploadAsset(payload)
    case 'asset.resolve': return resolveAsset(payload)
    case 'asset.list': ensureEdit(); return screenRequest('get', `/${encodeURIComponent(id.value)}/assets`, null, { params: { cursor: Number(payload.cursor) || undefined } })
    case 'binding.catalog': return screenRequest('get', `/${encodeURIComponent(id.value)}/binding-catalog`)
    case 'dataset.preview': ensureEdit(); return screenRequest('post', `/${encodeURIComponent(id.value)}/data-preview`, {plan:payload.plan,parameters:payload.parameters,from:payload.from,to:payload.to})
    case 'dataset.list': ensureEdit(); return screenRequest('get', `/${encodeURIComponent(id.value)}/datasets`)
    case 'dataset.load': ensureEdit(); return screenRequest('get', `/${encodeURIComponent(id.value)}/datasets/${encodeURIComponent(payload.id)}/versions/${encodeURIComponent(payload.version)}`)
    case 'dataset.save': ensureEdit(); return screenRequest('post', `/${encodeURIComponent(id.value)}/datasets`, {id:payload.id,baseVersion:payload.baseVersion,name:payload.name,plan:payload.plan,parameters:payload.parameters})
    case 'runtime.data': return screenRequest('post', `/${encodeURIComponent(id.value)}/runtime/data`, {bindingIds:payload.bindingIds,parameters:payload.parameters,from:payload.from,to:payload.to}, { params: currentRevision(), timeout:15000 })
    case 'runtime.snapshot': return screenRequest('post', `/${encodeURIComponent(id.value)}/runtime/snapshot`, {}, { params: currentRevision() })
    case 'runtime.history': return screenRequest('post', `/${encodeURIComponent(id.value)}/runtime/history`, { bindingId: payload.bindingId, startTime: payload.startTime, endTime: payload.endTime, maxPoints: payload.maxPoints }, { params: currentRevision() })
    case 'runtime.alarms': return screenRequest('post', `/${encodeURIComponent(id.value)}/runtime/alarms`, { bindingId: payload.bindingId }, { params: currentRevision() })
    case 'runtime.subscribe': runtimeWanted.value = true; startRuntime(); return { subscribed: true }
    case 'runtime.unsubscribe': runtimeWanted.value = false; stopRuntime(); return { subscribed: false }
    default: { const err = new Error('Unsupported RPC'); err.code = 'UNSUPPORTED'; throw err }
  }
}
let assistantController = null
async function planWithProgress(payload, resume = false) {
  ensureEdit()
  if (assistantController) throw new Error('Assistant is busy')
  const controller = new AbortController(), mountChannel = channelId.value
  assistantController = controller
  const timer = setTimeout(() => controller.abort(), 7 * 60 * 1000)
  try {
    const baseUrl = `/api/screens/${encodeURIComponent(id.value)}/assistant`
    const options = { headers: streamHeaders(), signal: controller.signal, onProgress: progress => {
      if (!disposed && mountChannel === channelId.value) send('assistant.progress', { runId: payload.runId, ...progress })
    } }
    if (resume) {
      const status = await assistantTaskRequest(`${baseUrl}/tasks/${encodeURIComponent(payload.taskId)}/resume`, { ...options, method: 'POST', payload: {} })
      return await pollAssistantTask(`${baseUrl}/tasks`, status, options)
    }
    return await createAssistantTask(`${baseUrl}/plan`, {
      prompt: payload.prompt, conversationId: payload.conversationId, document: payload.document, selectionIds: payload.selectionIds,
      selectionArea: payload.selectionArea, deviceCodes: payload.deviceCodes, contextScope: payload.contextScope,
      productCodes: payload.productCodes, scopeMode: payload.scopeMode, requestId: payload.requestId || payload.runId || requestId(),
      widgets: payload.widgets, history: payload.history, locale: locale.value.startsWith('zh') ? 'zh' : 'en',
    }, options)
  } finally { clearTimeout(timer); if (assistantController === controller) assistantController = null }
}
async function receive(event) {
  if (!isScreenMessage(event, frame.value?.contentWindow, window.location.origin, channelId.value)) return
  const message = event.data
  if (message.type === 'child.ready') { init(); return }
  if (message.type === 'child.initialized') { ready.value = true; clearTimeout(handshakeTimer); return }
  if (message.type === 'assistant.state') { assistantOpen.value = message.payload?.open === true; return }
  if (message.type === 'assistant.focus') { assistantButton.value?.focus(); return }
  if (message.type === 'screen.dirty') { if (mode.value === 'edit') dirty.value = message.payload?.dirty !== false; return }
  if (!SCREEN_RPC_TYPES.has(message.type) || typeof message.requestId !== 'string' || message.requestId.length > 128 || pending.has(message.requestId) || pending.size >= 64) return
  const mountChannel = channelId.value
  pending.add(message.requestId)
  try {
    const data = await rpc(message.type, message.payload || {})
    if (!disposed && mountChannel === channelId.value) send('rpc.result', { ok: true, data }, message.requestId)
  } catch (err) {
    if (!disposed && mountChannel === channelId.value) {
      const aiError = (message.type === 'assistant.plan' || message.type === 'assistant.task.resume') && ['aiUnavailable', 'aiInvalid', 'aiTimeout', 'aiFailed', 'aiOutOfScope'].includes(err?.response?.data?.errorKey) ? err.response.data.errorKey : null
      const queryMessage = message.type.startsWith('dataset.') && err?.response?.status === 400 && typeof err?.response?.data?.message === 'string' ? err.response.data.message.slice(0,512) : null
      // The plan endpoint returns only the machine reason ("unsupported section
      // role \"chart\""). Appending it keeps the localized sentence while making
      // the rejection diagnosable instead of a dead end. Validation rejections
      // carry the same kind of reason ("binding ai-x: a bounded device set is
      // required"), which otherwise surfaced as a bare "invalid request".
      const reason = (aiError === 'aiInvalid' || message.type === 'screen.validate') && typeof err?.response?.data?.message === 'string' ? err.response.data.message.slice(0, 240).trim() : ''
      const localized = queryMessage || t(`screens.${aiError || errorKey(err)}`)
      send('rpc.result', bridgeError(err, reason ? `${localized}（${reason}）` : localized), message.requestId)
    }
  }
  finally { pending.delete(message.requestId) }
}

function streamHeaders() {
  const headers = { Authorization: `Bearer ${auth.token}`, Accept: 'text/event-stream' }
  const tenantId = auth.user?.tenant_id || localStorage.getItem('current_tenant_id')
  const projectId = localStorage.getItem('current_project_id')
  if (tenantId) headers['X-Current-Tenant-ID'] = String(tenantId)
  if (projectId) headers['X-Current-Project-ID'] = projectId
  return headers
}
// A live value never becomes an empty dataset: terminal events stop the stream
// and ask the user to reload instead of silently showing stale numbers.
function handleRuntimeEvent(name, payload) {
  if (name === 'snapshot' || name === 'patch') { send('runtime.batch', payload); runtimeError.value = ''; return false }
  if (name === 'resync_required') { runtimeError.value = t('screens.revisionChanged'); boot(); return true }
  if (name === 'release_changed') {
    if (mode.value === 'view') { boot(); return true }
    // In edit mode this means another window moved the draft on. Re-subscribing
    // with our own (now stale) version makes the server answer 412, which turns a
    // silent freeze into the explicit "edited in another window" message.
    if (runtimeWanted.value) startRuntime(); else runtimeError.value = t('screens.revisionChanged')
    return true
  }
  if (name === 'access_revoked') { runtimeWanted.value = false; send('runtime.revoked', {}); runtimeError.value = t('screens.unauthorized'); return true }
  return false
}
function startRuntime() {
  if (disposed || !ready.value) return
  runtimeError.value = ''
  const params = new URLSearchParams(currentRevision())
  runtime.start({
    url: `/api/screens/${encodeURIComponent(id.value)}/runtime/stream?${params}`,
    headers: streamHeaders(),
    onEvent: handleRuntimeEvent,
    onTerminal: (reason) => {
      runtimeWanted.value = false
      send('runtime.revoked', {})
      runtimeError.value = t(reason === 'conflict' ? 'screens.conflict' : 'screens.unauthorized')
    },
    onError: () => { runtimeError.value = t('screens.runtimeFailed') },
  })
}
async function canLeave() { return !dirty.value || confirmDialog({ title: t('screens.leaveTitle'), message: t('screens.leaveMessage'), confirmText: t('screens.confirm'), cancelText: t('screens.cancel') }) }
function beforeUnload(event) { if (dirty.value) { event.preventDefault(); event.returnValue = '' } }
async function toggleFullscreen() { try { if (document.fullscreenElement) await document.exitFullscreen(); else await hostElement.value?.requestFullscreen() } catch { error.value = t('screens.failed') } }
function syncFullscreen() { fullscreen.value = document.fullscreenElement === hostElement.value }
watch(locale, () => { if (ready.value) send('host.locale', environment()) })
watch(() => [route.params.id, route.meta.screenMode, route.query.releaseId], boot)
watch(() => auth.token, (token) => { if (!token) { cleanup(); src.value = ''; error.value = t('screens.unauthorized') } })
onBeforeRouteLeave(canLeave)
onBeforeRouteUpdate(canLeave)
onMounted(() => {
  window.addEventListener('message', receive); window.addEventListener('beforeunload', beforeUnload); document.addEventListener('fullscreenchange', syncFullscreen)
  observer = new MutationObserver(() => { if (ready.value) send('host.theme', environment()) }); observer.observe(document.documentElement, { attributes: true, attributeFilter: ['data-bs-theme'] })
  boot()
})
onBeforeUnmount(() => { disposed = true; cleanup(); observer?.disconnect(); window.removeEventListener('message', receive); window.removeEventListener('beforeunload', beforeUnload); document.removeEventListener('fullscreenchange', syncFullscreen) })
</script>

<style scoped>
.screen-host { padding: 24px; color: var(--text-primary); background: var(--bg-body); display: flex; flex-direction: column; height: 100dvh; min-height: 0; overflow: hidden; position: relative; gap: 16px; }.screen-host-full { background: var(--bg-body); height: 100dvh; min-height: 0; }
.screen-host-toolbar { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px; }.screen-host-identity { display: flex; gap: 16px; align-items: center; min-width: 0; }.screen-host-identity h1 { font-size: 1.125rem; margin: 0; overflow-wrap: anywhere; }.screen-host-identity p { font-size: .75rem; color: var(--text-secondary); margin: 8px 0 0; }.screen-host-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }.screen-host-save-state { color: var(--text-secondary); font-size: .75rem; margin-inline-end: 8px; }
.screen-host-alert { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 16px; border: 1px solid var(--color-danger); border-radius: var(--radius-control); color: var(--color-danger); background: var(--bg-surface); }.screen-host-runtime { color: var(--color-warning); font-size: .875rem; }
.screen-host-frame { width: 100%; min-height: 0; height: 0; flex: 1; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-body); }
.screen-host-empty { padding: 96px 24px; text-align: center; display: flex; align-items: center; flex-direction: column; gap: 16px; color: var(--text-secondary); }.screen-host-empty h2 { font-size: 1.25rem; color: var(--text-primary); margin: 0; }.screen-host-empty p { margin: 0; }.screen-host-empty i { font-size: 3rem; }.screen-host-connecting { color: var(--text-secondary); padding: 16px; text-align: center; }
@media (max-width: 768px) { .screen-host-actions { width: 100%; }.screen-host-save-state { flex-basis: 100%; } }
</style>
