<template>
  <section class="screen-workspace" aria-labelledby="screens-title">
    <header class="screen-heading">
      <div><h1 id="screens-title">{{ t('screens.title') }}</h1><p>{{ t('screens.subtitle') }}</p></div>
      <div class="screen-card-actions">
        <LiquidGlassButton v-if="can('create') && can('edit')" variant="outline-secondary" @click="openName(null, true)"><i class="bi bi-stars" aria-hidden="true"></i> {{ t('screens.aiCreate') }}</LiquidGlassButton>
        <LiquidGlassButton v-if="can('create')" variant="primary" @click="openName()"><i class="bi bi-plus-lg" aria-hidden="true"></i> {{ t('screens.create') }}</LiquidGlassButton>
      </div>
    </header>
    <div v-if="error" class="screen-alert" role="alert">{{ error }} <button type="button" class="btn btn-sm btn-outline-secondary" @click="load">{{ t('screens.retry') }}</button></div>
    <div class="screen-toolbar">
      <label class="screen-search"><i class="bi bi-search" aria-hidden="true"></i><input v-model="search" type="search" class="form-control" :aria-label="t('screens.search')" :placeholder="t('screens.search')" /></label>
      <span class="screen-muted" aria-live="polite">{{ t('screens.items', { count: total }) }}</span>
    </div>
    <div v-if="loading" class="screen-empty" role="status">{{ t('screens.loading') }}</div>
    <div v-else-if="!items.length" class="screen-empty">
      <i class="bi bi-window-stack screen-empty-icon" aria-hidden="true"></i>
      <h2>{{ t(search ? 'screens.noResults' : 'screens.empty') }}</h2>
      <p>{{ t(search ? 'screens.noResultsHint' : 'screens.emptyHint') }}</p>
      <LiquidGlassButton v-if="can('create') && !search" variant="primary" @click="openName()">{{ t('screens.create') }}</LiquidGlassButton>
    </div>
    <div v-else class="screen-cards">
      <article v-for="screen in items" :key="screenId(screen)" class="screen-card">
        <div class="screen-card-visual" aria-hidden="true"><i class="bi bi-grid-1x2"></i><span>{{ screenId(screen).padStart(3, '0') }}</span></div>
        <div class="screen-card-body">
          <div class="screen-card-title"><h2>{{ screen.name }}</h2><span class="screen-status" :class="{ published: publishedReleaseId(screen) }">{{ t(publishedReleaseId(screen) ? 'screens.published' : 'screens.draft') }}</span></div>
          <div class="screen-card-meta"><span>{{ t('screens.draftLabel', { version: draftVersion(screen) }) }}</span><time>{{ formatDate(screen.updatedAt || screen.updated_at || screen.UpdatedAt) }}</time></div>
          <div class="screen-card-actions">
            <RouterLink v-if="can('edit')" class="btn btn-primary btn-sm" :to="`/screens/${screenId(screen)}/edit`">{{ t('screens.edit') }}</RouterLink>
            <RouterLink v-if="can('view')" class="btn btn-outline-secondary btn-sm" :to="`/screens/${screenId(screen)}/preview`">{{ t('screens.preview') }}</RouterLink>
            <RouterLink v-if="can('view') && publishedReleaseId(screen)" class="btn btn-outline-secondary btn-sm" :to="`/screens/${screenId(screen)}/view`">{{ t('screens.play') }}</RouterLink>
            <div v-if="hasMenu()" class="screen-card-more">
              <button
                type="button" class="btn btn-outline-secondary btn-sm screen-more-toggle"
                :data-menu-toggle="screenId(screen)" :aria-label="t('screens.more')" :title="t('screens.more')"
                aria-haspopup="menu" :aria-expanded="openMenu === screenId(screen)"
                @click="toggleMenu(screen, $event)" @keydown.down.prevent="openMenuAt(screen, $event, 'first')" @keydown.up.prevent="openMenuAt(screen, $event, 'last')"
              ><i class="bi bi-three-dots" aria-hidden="true"></i></button>
              <div v-if="openMenu === screenId(screen)" class="screen-menu" role="menu" :aria-label="t('screens.actions')" @keydown="onMenuKeydown" @keydown.esc.prevent="closeMenu()">
                <button v-if="can('edit')" type="button" role="menuitem" :disabled="busy" @click="menuAction(() => openName(screen))">{{ t('screens.rename') }}</button>
                <button v-if="can('create')" type="button" role="menuitem" :disabled="busy" @click="menuAction(() => duplicate(screen))">{{ t('screens.copy') }}</button>
                <button v-if="can('edit')" type="button" role="menuitem" :disabled="busy" @click="menuAction(() => openHistory(screen))">{{ t('screens.history') }}</button>
                <div class="screen-menu-sep" role="separator"></div>
                <button v-if="can('publish')" type="button" role="menuitem" :disabled="busy" @click="menuAction(() => publish(screen))">{{ t('screens.publish') }}</button>
                <button v-if="can('publish') && publishedReleaseId(screen)" type="button" role="menuitem" :disabled="busy" @click="menuAction(() => unpublish(screen))">{{ t('screens.unpublish') }}</button>
                <button v-if="can('delete')" type="button" role="menuitem" class="screen-danger" :disabled="busy" @click="menuAction(() => remove(screen))">{{ t('screens.delete') }}</button>
              </div>
            </div>
          </div>
        </div>
      </article>
    </div>
    <nav v-if="total > pageSize" class="screen-pagination" :aria-label="t('screens.title')"><button type="button" class="btn btn-outline-secondary" :disabled="page <= 1 || loading" @click="page--">{{ t('screens.previous') }}</button><span>{{ page }} / {{ Math.ceil(total / pageSize) }}</span><button type="button" class="btn btn-outline-secondary" :disabled="page * pageSize >= total || loading" @click="page++">{{ t('screens.next') }}</button></nav>

    <Teleport to="body">
      <dialog v-if="nameDialog" ref="nameDialogElement" class="screen-dialog" aria-labelledby="screen-name-title" @cancel.prevent="closeName" @keydown="trapDialogFocus">
        <form @submit.prevent="saveName">
          <h2 id="screen-name-title">{{ t(editing ? 'screens.rename' : createWithAI ? 'screens.aiCreate' : 'screens.create') }}</h2>
          <label for="screen-name">{{ t('screens.name') }}</label><input id="screen-name" ref="nameInput" v-model="name" class="form-control" maxlength="120" required :placeholder="t('screens.namePlaceholder')" />
          <p v-if="nameError" class="screen-danger" role="alert">{{ nameError }}</p>
          <div class="screen-dialog-actions"><LiquidGlassButton type="button" variant="outline-secondary" :disabled="busy" @click="closeName">{{ t('screens.cancel') }}</LiquidGlassButton><LiquidGlassButton type="submit" variant="primary" :disabled="busy || !name.trim()">{{ t('screens.save') }}</LiquidGlassButton></div>
        </form>
      </dialog>
      <dialog v-if="historyScreen" ref="historyDialog" class="screen-dialog screen-history" aria-labelledby="screen-history-title" @cancel.prevent="closeHistory" @keydown="trapDialogFocus">
        <section>
          <div class="screen-heading"><h2 id="screen-history-title">{{ t('screens.history') }} · {{ historyScreen.name }}</h2><button type="button" class="btn btn-outline-secondary" @click="closeHistory">{{ t('screens.close') }}</button></div>
          <p v-if="historyError" class="screen-danger" role="alert">{{ historyError }}</p>
          <div v-if="historyLoading" class="screen-history-empty" role="status">{{ t('screens.loading') }}</div>
          <div v-else-if="!releases.length" class="screen-history-empty">{{ t('screens.noReleases') }}</div>
          <article v-for="release in releases" :key="screenId(release)" class="screen-release">
            <div><strong>{{ t('screens.releaseNumber', { number: releaseNumber(release) }) }}</strong><p>{{ formatDate(release.createdAt || release.created_at || release.CreatedAt) }}</p></div>
            <div class="screen-card-actions"><RouterLink class="btn btn-sm btn-outline-secondary" :to="{ path: `/screens/${screenId(historyScreen)}/view`, query: { releaseId: screenId(release) } }">{{ t('screens.play') }}</RouterLink><button v-if="can('edit')" type="button" class="btn btn-sm btn-outline-secondary" :disabled="busy" @click="restore(release)">{{ t('screens.restore') }}</button></div>
          </article>
        </section>
      </dialog>
    </Teleport>
  </section>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { useConfirm } from '../composables/useConfirm.js'
import { screensApi, screenId, draftVersion, metadataVersion, publishedReleaseId, releaseNumber, errorKey, requestId } from '../screens/api.js'

const { t, locale } = useI18n()
const router = useRouter()
const auth = useAuthStore()
const { confirmDialog } = useConfirm()
const can = (action) => auth.hasPermission(`screen:${action}`)
const items = ref([]), total = ref(0), search = ref(''), page = ref(1), pageSize = 20
const loading = ref(false), busy = ref(false), error = ref('')
const nameDialog = ref(false), nameDialogElement = ref(null), editing = ref(null), name = ref(''), nameError = ref(''), nameInput = ref(null)
const createWithAI = ref(false)
const historyScreen = ref(null), releases = ref([]), historyLoading = ref(false), historyError = ref(''), historyDialog = ref(null)
let timer, controller, opener
const pendingCopies = new Map()
// Nine equal-weight buttons made every card read like a toolbar, and the two
// destructive ones sat right next to "Edit". The three actions an operator
// actually uses stay visible; everything else lives behind one menu in a fixed
// order (rename → duplicate → history → publish → unpublish → delete).
const openMenu = ref('')
function hasMenu() { return ['edit', 'create', 'publish', 'delete'].some((action) => can(action)) }
function focusMenuItem(wrapper, position = 'first') {
  const items = wrapper?.querySelectorAll('[role="menuitem"]:not([disabled])')
  if (!items?.length) return
  items[position === 'last' ? items.length - 1 : 0].focus()
}
function toggleMenu(screen, event) {
  const id = screenId(screen)
  if (openMenu.value === id) { closeMenu(); return }
  openMenu.value = id
  const wrapper = event.currentTarget.closest('.screen-card-more')
  nextTick(() => focusMenuItem(wrapper))
}
function openMenuAt(screen, event, position) {
  openMenu.value = screenId(screen)
  const wrapper = event.currentTarget.closest('.screen-card-more')
  nextTick(() => focusMenuItem(wrapper, position))
}
function closeMenu(refocus = true) {
  const id = openMenu.value
  if (!id) return
  openMenu.value = ''
  if (refocus) nextTick(() => document.querySelector(`[data-menu-toggle="${CSS.escape(id)}"]`)?.focus())
}
// Closing restores focus to the toggle first, so a dialog opened from the menu
// still has somewhere sensible to return to when it closes.
function menuAction(action) {
  const toggle = document.querySelector(`[data-menu-toggle="${CSS.escape(openMenu.value)}"]`)
  closeMenu(false)
  toggle?.focus()
  action()
}
function onMenuKeydown(event) {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End', 'Tab'].includes(event.key)) return
  // Tabbing out of the menu keeps the toggle focused; the menu itself is about
  // to be removed, and focus must not fall back to the document body.
  if (event.key === 'Tab') {
    document.querySelector(`[data-menu-toggle="${CSS.escape(openMenu.value)}"]`)?.focus()
    closeMenu(false)
    return
  }
  const menu = event.currentTarget
  const items = [...menu.querySelectorAll('[role="menuitem"]:not([disabled])')]
  if (!items.length) return
  event.preventDefault()
  const index = items.indexOf(document.activeElement)
  const next = event.key === 'Home' ? 0
    : event.key === 'End' ? items.length - 1
      : event.key === 'ArrowDown' ? (index + 1) % items.length
        : index <= 0 ? items.length - 1 : index - 1
  items[next].focus()
}
function onDocumentPointerDown(event) {
  if (openMenu.value && !event.target.closest?.('.screen-card-more')) closeMenu(false)
}
function trapDialogFocus(event) {
  if (event.key !== 'Tab' || event.ctrlKey || event.altKey || event.metaKey) return
  const dialog = event.currentTarget
  const controls = Array.from(dialog.querySelectorAll('button, [href], input, select, textarea, [tabindex]'))
    .filter(element => element.tabIndex >= 0 && !element.matches(':disabled') && element.getClientRects().length > 0)
  const first = controls[0], last = controls[controls.length - 1]
  if (!first) { event.preventDefault(); dialog.focus(); return }
  const active = document.activeElement
  if (!dialog.contains(active) || (event.shiftKey && active === first) || (!event.shiftKey && active === last)) {
    event.preventDefault()
    const target = event.shiftKey ? last : first
    target.focus()
  }
}
const formatDate = (value) => {
  const date = value ? new Date(value) : null
  return date && Number.isFinite(date.getTime()) ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(date) : '—'
}

async function load() {
  controller?.abort(); controller = new AbortController()
  const signal = controller.signal
  loading.value = true; error.value = ''
  try {
    const result = await screensApi.list({ search: search.value.trim(), page: page.value, pageSize }, signal)
    if (!signal.aborted) { items.value = result?.items || []; total.value = result?.total || 0 }
  } catch (err) { if (!signal.aborted) error.value = t(`screens.${errorKey(err)}`) }
  finally { if (!signal.aborted) loading.value = false }
}
watch(search, () => { clearTimeout(timer); timer = setTimeout(() => { if (page.value === 1) load(); else page.value = 1 }, 250) })
watch(page, load)
onMounted(() => { load(); document.addEventListener('pointerdown', onDocumentPointerDown) })
onBeforeUnmount(() => { clearTimeout(timer); controller?.abort(); document.removeEventListener('pointerdown', onDocumentPointerDown) })

async function run(action) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await action(); await load() }
  catch (err) { error.value = t(`screens.${errorKey(err)}`) }
  finally { busy.value = false }
}
async function openName(screen = null, assistant = false) { opener = document.activeElement; editing.value = screen; createWithAI.value = !screen && assistant; name.value = screen?.name || ''; nameError.value = ''; nameDialog.value = true; await nextTick(); nameDialogElement.value?.showModal(); nameInput.value?.focus() }
function finishName() { nameDialogElement.value?.close(); nameDialog.value = false; opener?.focus?.() }
function closeName() { if (!busy.value) finishName() }
async function saveName() {
  const clean = name.value.trim()
  if (!clean || clean.length > 120) { nameError.value = t(clean ? 'screens.invalidName' : 'screens.nameRequired'); return }
  busy.value = true; nameError.value = ''
  try {
    if (editing.value) { await screensApi.rename(screenId(editing.value), clean, metadataVersion(editing.value)); finishName(); await load() }
    else { const result = await screensApi.create(clean); finishName(); router.push({ path: `/screens/${screenId(result?.screen || result)}/edit`, query: createWithAI.value ? { assistant: '1' } : {} }) }
  } catch (err) { nameError.value = t(`screens.${errorKey(err)}`); if (err?.response?.status === 412) await load() }
  finally { busy.value = false }
}
async function duplicate(screen) {
  const id = screenId(screen)
  // Keep the intent across an uncertain response so retry cannot create twice.
  const intent = pendingCopies.get(id) || { name: `${screen.name} ${t('screens.copySuffix')}`.slice(0, 120), version: draftVersion(screen), requestId: requestId() }
  pendingCopies.set(id, intent)
  await run(async () => {
    try { await screensApi.copy(id, intent.name, intent.version, intent.requestId); pendingCopies.delete(id) }
    catch (err) {
      if ([409, 412].includes(err?.response?.status)) { pendingCopies.delete(id); await load() }
      throw err
    }
  })
}
async function ask(key, variant = 'danger') { return confirmDialog({ title: t(`screens.${key}Title`), message: t(`screens.${key}Message`), variant, confirmText: t('screens.confirm'), cancelText: t('screens.cancel') }) }
async function publish(screen) { if (await ask('publish', 'neutral')) await run(() => screensApi.publish(screenId(screen), draftVersion(screen))) }
async function unpublish(screen) { if (await ask('unpublish')) await run(() => screensApi.unpublish(screenId(screen))) }
async function remove(screen) { if (await ask('delete')) await run(() => screensApi.remove(screenId(screen))) }
async function openHistory(screen) {
  opener = document.activeElement; historyScreen.value = screen; releases.value = []; historyError.value = ''; historyLoading.value = true
  await nextTick(); historyDialog.value?.showModal()
  try { const result = await screensApi.releases(screenId(screen)); if (historyScreen.value === screen) releases.value = result?.items || [] }
  catch (err) { historyError.value = t(`screens.${errorKey(err)}`) }
  finally { historyLoading.value = false }
}
function finishHistory() { historyDialog.value?.close(); historyScreen.value = null; opener?.focus?.() }
function closeHistory() { if (!busy.value) finishHistory() }
async function restore(release) {
  // The shared confirmation lives outside the native dialog's top layer.
  const restoreButton = document.activeElement
  historyDialog.value?.close()
  if (await ask('restore')) await run(async () => { const id = screenId(historyScreen.value); const current = await screensApi.load(id); await screensApi.restore(id, screenId(release), current.draftVersion); finishHistory() })
  if (historyScreen.value) { await nextTick(); historyDialog.value?.showModal(); restoreButton?.focus?.() }
}
</script>

<style scoped>
.screen-workspace { padding: 24px; color: var(--text-primary); }
.screen-heading { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px; margin-bottom: 24px; }
.screen-heading h1 { font-size: 1.5rem; margin: 0 0 8px; }.screen-heading p, .screen-card-meta, .screen-muted { color: var(--text-secondary); margin: 0; }
.screen-toolbar { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; margin-bottom: 24px; }
.screen-search { display: flex; align-items: center; gap: 8px; flex: 1; max-width: 400px; }
.screen-alert { border: 1px solid var(--color-danger); color: var(--color-danger); background: var(--bg-surface); padding: 16px; border-radius: var(--radius-control); margin-bottom: 24px; display: flex; justify-content: space-between; gap: 16px; }
.screen-empty { padding: 96px 24px; display: flex; align-items: center; flex-direction: column; gap: 16px; text-align: center; color: var(--text-secondary); }.screen-empty h2 { font-size: 1.25rem; color: var(--text-primary); margin: 0; }.screen-empty p { margin: 0; }.screen-empty-icon { font-size: 3rem; color: var(--accent-color); }
.screen-cards { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 360px), 1fr)); gap: 24px; }
.screen-card { background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-card); overflow: visible; }
.screen-card-visual { background: var(--bg-hover); color: var(--accent-color); height: 128px; display: flex; align-items: center; justify-content: center; gap: 24px; border-bottom: 1px solid var(--border-color); border-radius: calc(var(--radius-card) - 1px) calc(var(--radius-card) - 1px) 0 0; }.screen-card-visual i { font-size: 3rem; }.screen-card-visual span { font-variant-numeric: tabular-nums; font-size: 2rem; opacity: .48; }
.screen-card-body { padding: 24px; }.screen-card-title { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }.screen-card-title h2 { margin: 0; font-size: 1.125rem; overflow-wrap: anywhere; }.screen-status { color: var(--text-secondary); font-size: .75rem; white-space: nowrap; }.screen-status.published { color: var(--color-success); }
.screen-card-meta { display: flex; flex-wrap: wrap; justify-content: space-between; gap: 8px; font-size: .75rem; margin-top: 16px; }.screen-card-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 24px; }
.screen-card-more { position: relative; margin-left: auto; }.screen-more-toggle { display: inline-flex; align-items: center; justify-content: center; width: 32px; padding: 0; }
.screen-menu { position: absolute; top: calc(100% + 6px); right: 0; z-index: 30; display: flex; flex-direction: column; gap: 2px; min-width: 176px; padding: 6px; background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-control); box-shadow: var(--shadow-floating); }
.screen-menu button { display: flex; align-items: center; min-height: 34px; padding: 0 10px; border: 0; border-radius: calc(var(--radius-control) - 4px); background: none; color: var(--text-primary); font-size: .8125rem; text-align: left; cursor: pointer; }
.screen-menu button:hover, .screen-menu button:focus-visible { background: var(--bg-hover); outline: none; }.screen-menu button:disabled { opacity: .5; cursor: default; }
.screen-menu-sep { height: 1px; margin: 4px 6px; background: var(--border-color); }.screen-menu .screen-danger { color: var(--color-danger); }
.screen-danger { color: var(--color-danger); }.screen-pagination { display: flex; align-items: center; justify-content: center; gap: 16px; margin-top: 24px; }
.screen-history-empty { padding: 96px 24px; text-align: center; color: var(--text-secondary); }.screen-release { display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 16px; border-top: 1px solid var(--border-color); padding: 16px 0; }.screen-release p { color: var(--text-secondary); margin: 8px 0 0; }.screen-release .screen-card-actions { margin: 0; }
@media (max-width: 600px) { .screen-heading { align-items: flex-start; }.screen-toolbar .screen-search { max-width: none; width: 100%; flex-basis: 100%; } }
</style>
<style>
.screen-dialog::backdrop { background: var(--bg-sidebar); backdrop-filter: blur(16px); }
.screen-dialog { width: min(480px, calc(100% - 48px)); max-height: calc(100dvh - 48px); overflow: auto; padding: 24px; background: var(--bg-body); border: 1px solid var(--border-color); color: var(--text-primary); border-radius: var(--radius-modal); }.screen-dialog h2 { font-size: 1.25rem; margin: 0 0 24px; }.screen-dialog label { margin-bottom: 8px; }.screen-dialog-actions { display: flex; justify-content: flex-end; flex-wrap: wrap; gap: 8px; margin-top: 24px; }.screen-dialog p[role='alert'] { margin-top: 16px; }.screen-history { width: min(800px, calc(100% - 48px)); }
</style>
