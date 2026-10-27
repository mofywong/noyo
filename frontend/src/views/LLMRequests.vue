<template>
  <div class="llm-requests-page">
    <header class="page-header">
      <div><h1>{{ t('llm_requests') }}</h1><p class="page-subtitle">{{ t('llm_requests_subtitle') }}</p></div>
      <LiquidGlassButton type="button" variant="outline-secondary" :disabled="loading" @click="reloadAll">{{ t('llm_refresh') }}</LiquidGlassButton>
    </header>

    <section class="llm-overview" :aria-label="t('llm_stats_overview')" :aria-busy="statsLoading">
      <article v-for="period in periods" :key="period.key" class="llm-panel llm-stat">
        <span class="llm-stat-label">{{ t(period.label) }}</span>
        <strong class="llm-stat-value">{{ stats ? number(stats[period.key]?.requests) : '—' }}</strong>
        <p class="llm-stat-meta">
          <span><i class="bi bi-check-circle" aria-hidden="true"></i>{{ t('llm_stats_success') }} {{ stats ? number(stats[period.key]?.successes) : '—' }}</span>
          <span><i class="bi bi-x-circle" aria-hidden="true"></i>{{ t('llm_stats_failed') }} {{ stats ? number(stats[period.key]?.errors) : '—' }}</span>
        </p>
        <p class="llm-stat-meta"><span><i class="bi bi-cpu" aria-hidden="true"></i>{{ t('llm_stats_tokens') }} {{ stats ? number(stats[period.key]?.total_tokens) : '—' }}</span></p>
      </article>
    </section>
    <p v-if="statsError" class="llm-note llm-error" role="alert">{{ t('llm_stats_unavailable') }}</p>

    <form class="llm-panel llm-filters" @submit.prevent="applyFilters">
      <label><span>{{ t('llm_model') }}</span><input v-model="filters.model" class="form-control form-control-sm" type="text"></label>
      <label><span>{{ t('llm_source') }}</span><select v-model="filters.source" class="form-select form-select-sm"><option value="">{{ t('llm_all') }}</option><option v-for="option in sourceOptions" :key="option" :value="option">{{ llmSourceLabel(option, t) }}</option></select></label>
      <label><span>{{ t('llm_session') }}</span><select v-model="filters.session_id" class="form-select form-select-sm"><option value="">{{ t('llm_all') }}</option><option v-for="option in sessionOptions" :key="option.value" :value="option.value">{{ llmSessionLabel(option.value, option.title, t) }}</option></select></label>
      <label><span>{{ t('llm_status') }}</span><select v-model="filters.status" class="form-select form-select-sm"><option value="">{{ t('llm_all') }}</option><option v-for="status in statuses" :key="status" :value="status">{{ statusLabel(status) }}</option></select></label>
      <label class="llm-search"><span>{{ t('llm_search') }}</span><input v-model="filters.search" class="form-control form-control-sm" type="search"></label>
      <label><span>{{ t('llm_from') }}</span><input v-model="filters.started_after" class="form-control form-control-sm" type="datetime-local" step="1"></label>
      <label><span>{{ t('llm_until') }}</span><input v-model="filters.started_before" class="form-control form-control-sm" type="datetime-local" step="1"></label>
      <div class="llm-filter-actions"><LiquidGlassButton type="submit" variant="primary" :disabled="loading">{{ t('llm_query') }}</LiquidGlassButton><LiquidGlassButton type="button" variant="outline-secondary" :disabled="loading" @click="resetFilters">{{ t('llm_reset') }}</LiquidGlassButton></div>
    </form>
    <p v-if="validationError" class="llm-error" role="alert">{{ t(`llm_${validationError}`) }}</p>
    <div v-if="loadError" class="llm-error llm-retry" role="alert"><span>{{ t('llm_load_failed') }}</span><LiquidGlassButton type="button" variant="outline-secondary" @click="loadRequests">{{ t('llm_retry') }}</LiquidGlassButton></div>

    <section class="llm-panel llm-list" :aria-label="t('llm_requests')" :aria-busy="loading">
      <div v-if="loading" class="llm-empty" role="status">{{ t('llm_loading') }}</div>
      <div v-else-if="!requests.length" class="llm-empty"><i class="bi bi-journal-code" aria-hidden="true"></i><strong>{{ t('llm_no_records') }}</strong><span>{{ t('llm_no_records_hint') }}</span></div>
      <div v-else class="table-responsive"><table class="table table-hover table-compact align-middle mb-0"><thead><tr><th>{{ t('llm_started') }}</th><th>{{ t('llm_session') }}</th><th>{{ t('llm_source') }}</th><th>{{ t('llm_model') }}</th><th>{{ t('llm_status') }}</th><th>{{ t('llm_duration') }}</th><th>{{ t('llm_prompt_tokens') }}</th><th>{{ t('llm_completion_tokens') }}</th><th>{{ t('llm_total_tokens') }}</th><th>{{ t('llm_details') }}</th></tr></thead><tbody><tr v-for="row in rows" :key="row.id"><td class="llm-nowrap">{{ formatDateTime(row.started_at) }}</td><td><button v-if="row.session_id" type="button" class="llm-link llm-entity" :title="`${t('llm_filter_session')} ${row.sessionName}`" @click="filterSession(row.session_id, row.session_title)"><span class="llm-entity-name">{{ row.sessionName }}</span><span class="llm-entity-id">（{{ row.session_id }}）</span></button><span v-else class="llm-note">{{ t('llm_no_session') }}</span></td><td><span v-if="row.source" class="llm-entity"><span class="llm-entity-name">{{ row.sourceName || row.source }}</span><span v-if="row.sourceName" class="llm-entity-id">（{{ row.source }}）</span></span><span v-else class="llm-note">{{ t('llm_no_source') }}</span></td><td><strong>{{ row.model || t('llm_no_value') }}</strong><small class="llm-provider">{{ row.provider || t('llm_no_value') }}</small></td><td><span class="spec-badge" :class="statusClass(row.status)">{{ statusLabel(row.status) }}</span><small class="llm-provider">{{ row.stream ? t('llm_streaming') : t('llm_standard') }}</small></td><td class="llm-nowrap">{{ duration(row.duration_ms) }}</td><td>{{ requestToken(row, 'prompt_tokens') }}</td><td>{{ requestToken(row, 'completion_tokens') }}</td><td>{{ requestToken(row, 'total_tokens') }}</td><td><LiquidGlassButton type="button" variant="outline-secondary" size="sm" :aria-label="`${t('llm_details')} ${row.id}`" @click="openDetails(row)">{{ t('llm_details') }}</LiquidGlassButton></td></tr></tbody></table></div>
      <ListPagination :page="page" :page-size="pageSize" :total="total" :disabled="loading" id-prefix="llm-requests" @update:page="changePage" @update:page-size="changePageSize" />
    </section>

    <dialog ref="detailDialog" class="llm-dialog" aria-labelledby="llm-detail-title" @close="onDetailClosed">
      <div class="llm-dialog-header"><h2 id="llm-detail-title">{{ t('llm_request_number', { id: selectedID }) }}</h2><LiquidGlassButton type="button" variant="outline-secondary" autofocus @click="closeDetails">{{ t('llm_close') }}</LiquidGlassButton></div>
      <div v-if="detailLoading" class="llm-empty" role="status">{{ t('llm_loading') }}</div>
      <div v-else-if="detailError" class="llm-error llm-retry" role="alert"><span>{{ t('llm_detail_failed') }}</span><LiquidGlassButton type="button" variant="outline-secondary" @click="loadDetail(selectedID)">{{ t('llm_retry') }}</LiquidGlassButton></div>
      <template v-else-if="detail">
        <dl class="llm-detail-grid"><template v-for="field in detailFields" :key="field.key"><div><dt>{{ t(field.label) }}</dt><dd>{{ detailValue(field.key) }}</dd></div></template></dl>
        <section v-if="detail.error" class="llm-error" role="alert"><h3>{{ t('llm_failure') }}</h3><pre>{{ detail.error }}</pre></section>
        <div class="llm-detail-tokens"><div v-for="field in tokenFields" :key="field.key"><span>{{ t(field.label) }}</span><strong>{{ detail.usage_available ? number(detail[field.key]) : t('llm_usage_missing') }}</strong></div></div>
        <p v-if="!detail.usage_available" class="llm-note" role="status">{{ t('llm_usage_missing_note') }}</p>
        <p class="llm-note">{{ t('llm_token_note') }}</p>
        <section v-for="field in payloadFields" :key="field.key" class="llm-payload"><h3>{{ t(field.label) }}</h3><p v-if="field.key === 'response_body' && detail.stream" class="llm-note">{{ t('llm_raw_stream_note') }}</p><pre tabindex="0" :aria-label="t(field.label)">{{ formatLLMPayload(detail[field.key]) || t('llm_empty_payload') }}</pre></section>
      </template>
    </dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import ListPagination from '../components/ListPagination.vue'
import { formatDateTime } from '../utils/dateTime.js'
import {
  emptyLLMFilters, buildLLMRequestParams, formatLLMPayload,
  llmSourceLabel, llmSessionLabel, llmSourceName, llmSessionName
} from '../utils/llmRequests.js'

const { t, locale } = useI18n()
const filters = ref(emptyLLMFilters())
const appliedFilters = ref({})
const validationError = ref('')
const statuses = ['pending', 'success', 'error', 'cancelled']
const periods = [
  { key: 'today', label: 'llm_stats_today' }, { key: 'week', label: 'llm_stats_week' },
  { key: 'month', label: 'llm_stats_month' }, { key: 'all', label: 'llm_stats_all' }
]
const stats = ref(null)
const statsLoading = ref(false)
const statsError = ref(false)
const facets = ref({ sources: [], sessions: [] })
const extraSessions = ref([])
const requests = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const loadError = ref(false)
const detailDialog = ref(null)
const selectedID = ref(null)
const detail = ref(null)
const detailLoading = ref(false)
const detailError = ref(false)
let listController
let detailController
let statsController
let facetsController
let listVersion = 0
let detailVersion = 0
let statsVersion = 0
let facetsVersion = 0

const sourceOptions = computed(() => {
  const values = facets.value.sources.map(item => item.value).filter(Boolean)
  if (filters.value.source && !values.includes(filters.value.source)) values.push(filters.value.source)
  return values
})// The active conversation must stay selectable even when it falls outside the
// facet cap or its conversation row has been removed.
const sessionOptions = computed(() => {
  const options = [...facets.value.sessions, ...extraSessions.value]
  const seen = new Set()
  const result = []
  for (const option of options) {
    if (!option.value || seen.has(option.value)) continue
    seen.add(option.value)
    result.push(option)
  }
  if (filters.value.session_id && !seen.has(filters.value.session_id)) result.push({ value: filters.value.session_id, title: '' })
  return result
})

// Rows carry resolved labels so the template never formats text twice and the
// labels follow the active language.
const rows = computed(() => requests.value.map(row => ({
  ...row,
  sessionName: row.session_id ? llmSessionName(row.session_title, t) : '',
  sourceName: row.source ? llmSourceName(row.source, t) : ''
})))

const tokenFields = [
  { key: 'prompt_tokens', label: 'llm_prompt_tokens' }, { key: 'completion_tokens', label: 'llm_completion_tokens' },
  { key: 'total_tokens', label: 'llm_total_tokens' }, { key: 'cached_tokens', label: 'llm_cached_tokens' },
  { key: 'reasoning_tokens', label: 'llm_reasoning_tokens' }
]
const detailFields = [
  { key: 'session_id', label: 'llm_session' }, { key: 'source', label: 'llm_source' },
  { key: 'provider', label: 'llm_provider' }, { key: 'model', label: 'llm_model' },
  { key: 'status', label: 'llm_status' }, { key: 'stream', label: 'llm_stream' },
  { key: 'started_at', label: 'llm_started' }, { key: 'finished_at', label: 'llm_finished' },
  { key: 'duration_ms', label: 'llm_duration' }, { key: 'status_code', label: 'llm_http_status' },
  { key: 'first_byte_ms', label: 'llm_first_byte' }, { key: 'usage_available', label: 'llm_usage_available' },
  { key: 'request_id', label: 'llm_request_id' }, { key: 'method', label: 'llm_method' },
  { key: 'provider_request_id', label: 'llm_provider_request_id' }, { key: 'response_id', label: 'llm_response_id' },
  { key: 'finish_reason', label: 'llm_finish_reason' },
  { key: 'endpoint', label: 'llm_endpoint' }, { key: 'tenant_id', label: 'llm_tenant' },
  { key: 'project_id', label: 'llm_project' }, { key: 'user_id', label: 'llm_user' }
]
const payloadFields = [
  { key: 'request_body', label: 'llm_request_body' }, { key: 'response_body', label: 'llm_response_body' },
  { key: 'usage_json', label: 'llm_raw_usage' }, { key: 'response_headers', label: 'llm_response_headers' }
]
const number = value => Number(value || 0).toLocaleString(locale.value)
const requestToken = (row, key) => row.usage_available ? number(row[key]) : t('llm_usage_missing')
const duration = value => `${Number(value || 0).toLocaleString(locale.value, { maximumFractionDigits: 1 })} ms`
const statusLabel = status => statuses.includes(status) ? t(`llm_${status}`) : status || t('llm_no_value')
const statusClass = status => status === 'success' ? 'spec-badge--success' : (status === 'error' || status === 'cancelled' ? 'spec-badge--danger' : 'spec-badge--neutral')

function detailValue(key) {
  const value = detail.value?.[key]
  if (key === 'status') return statusLabel(value)
  if (key === 'stream') return t(value ? 'llm_streaming' : 'llm_standard')
  if (key === 'usage_available') return t(value ? 'llm_usage_reported' : 'llm_usage_missing')
  if (key === 'session_id') return value ? llmSessionLabel(value, detail.value?.session_title, t) : t('llm_no_session')
  if (key === 'source') return value ? llmSourceLabel(value, t) : t('llm_no_source')
  if (key === 'started_at' || key === 'finished_at') return value ? formatDateTime(value) : t('llm_no_value')
  if (key === 'duration_ms' || key === 'first_byte_ms') return duration(value)
  return value === '' || value === null || value === undefined ? t('llm_no_value') : value
}

async function loadRequests() {
  const version = ++listVersion
  listController?.abort()
  listController = new AbortController()
  loading.value = true
  loadError.value = false
  requests.value = []
  total.value = 0
  try {
    const params = { ...appliedFilters.value }
    const result = await axios.get('/api/llm-requests', { params: { ...params, page: page.value, page_size: pageSize.value }, signal: listController.signal })
    if (version !== listVersion) return
    if (result.data.code !== 0) throw new Error('request_failed')
    requests.value = result.data.data || []
    total.value = Number(result.data.total || 0)
  } catch (error) {
    if (version === listVersion && !axios.isCancel(error)) loadError.value = true
  } finally {
    if (version === listVersion) loading.value = false
  }
}

async function loadStats() {
  const version = ++statsVersion
  statsController?.abort()
  statsController = new AbortController()
  statsLoading.value = true
  statsError.value = false
  try {
    const result = await axios.get('/api/llm-requests/stats', { signal: statsController.signal })
    if (version !== statsVersion) return
    if (result.data.code !== 0 || !result.data.data) throw new Error('stats_failed')
    stats.value = result.data.data
  } catch (error) {
    if (version === statsVersion && !axios.isCancel(error)) { statsError.value = true; stats.value = null }
  } finally {
    if (version === statsVersion) statsLoading.value = false
  }
}

async function loadFacets() {
  const version = ++facetsVersion
  facetsController?.abort()
  facetsController = new AbortController()
  try {
    const result = await axios.get('/api/llm-requests/facets', { signal: facetsController.signal })
    if (version !== facetsVersion) return
    if (result.data.code !== 0 || !result.data.data) return
    facets.value = {
      sources: Array.isArray(result.data.data.sources) ? result.data.data.sources : [],
      sessions: Array.isArray(result.data.data.sessions) ? result.data.data.sessions : []
    }
  } catch (error) {
    // Filters stay usable with the "all" option; the next refresh retries.
  }
}

function reloadAll() { loadStats(); loadFacets(); loadRequests() }

function applyFilters() {
  try {
    appliedFilters.value = buildLLMRequestParams(filters.value)
    validationError.value = ''
    page.value = 1
    loadRequests()
  } catch (error) { validationError.value = error.message }
}
function resetFilters() { filters.value = emptyLLMFilters(); applyFilters() }
function filterSession(session, title) {
  filters.value.session_id = session
  if (session && !extraSessions.value.some(item => item.value === session)) extraSessions.value.push({ value: session, title: title || '' })
  applyFilters()
}
function changePage(value) { page.value = value; loadRequests() }
function changePageSize(value) { pageSize.value = value; page.value = 1; loadRequests() }

function openDetails(row) {
  selectedID.value = row.id
  detail.value = null
  detailDialog.value.showModal()
  loadDetail(row.id)
}
async function loadDetail(id) {
  const version = ++detailVersion
  detailController?.abort()
  detailController = new AbortController()
  detailLoading.value = true
  detailError.value = false
  try {
    const result = await axios.get(`/api/llm-requests/${id}`, { signal: detailController.signal })
    if (version !== detailVersion) return
    if (result.data.code !== 0 || !result.data.data) throw new Error('detail_failed')
    detail.value = result.data.data
  } catch (error) {
    if (version === detailVersion && !axios.isCancel(error)) detailError.value = true
  } finally { if (version === detailVersion) detailLoading.value = false }
}
function closeDetails() { detailDialog.value?.close() }
function onDetailClosed() { detailVersion++; detailController?.abort(); detail.value = null; selectedID.value = null }
onMounted(() => { loadStats(); loadFacets(); loadRequests() })
onBeforeUnmount(() => {
  listVersion++; detailVersion++; statsVersion++; facetsVersion++
  listController?.abort(); detailController?.abort(); statsController?.abort(); facetsController?.abort()
  detailDialog.value?.close()
})
</script>

<style scoped>
.llm-requests-page { padding: 24px; display: flex; flex-direction: column; gap: 24px; color: var(--text-primary); }
.llm-requests-page .page-header { margin-bottom: 0; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; }
.llm-panel { background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-card); }
.llm-overview { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
.llm-stat { padding: 16px 24px 24px; display: flex; flex-direction: column; gap: 8px; overflow-wrap: anywhere; }
.llm-stat-label { color: var(--text-secondary); font-size: .8125rem; }
.llm-stat-value { font-size: 1.75rem; line-height: 1.2; font-variant-numeric: tabular-nums; }
.llm-stat-meta { margin: 0; display: flex; flex-wrap: wrap; gap: 8px 16px; color: var(--text-secondary); font-size: .8125rem; }
.llm-stat-meta span { display: inline-flex; align-items: center; gap: 8px; }
.llm-stat-meta .bi-check-circle { color: var(--color-success); }
.llm-stat-meta .bi-x-circle { color: var(--color-danger); }
.llm-stat-meta .bi-cpu { color: var(--text-secondary); }
.llm-filters { padding: 24px; display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px; }
.llm-filters label { display: flex; flex-direction: column; gap: 8px; font-size: .875rem; color: var(--text-secondary); }
.llm-search { grid-column: span 2; }
.llm-filter-actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: end; grid-column: span 2; }
.llm-note, .llm-provider { color: var(--text-secondary); font-size: .8125rem; }
.llm-note { margin: 8px 0; }
.llm-list { overflow: hidden; }
.llm-list th, .llm-list td { padding: 16px 12px; }
.llm-list thead { white-space: nowrap; }
.llm-nowrap { white-space: nowrap; }
.llm-provider { display: block; margin-top: 8px; }
.llm-link { padding: 0; border: 0; background: none; color: var(--color-brand); text-align: start; text-decoration: underline; cursor: pointer; overflow-wrap: anywhere; }
.llm-entity { display: flex; flex-direction: column; gap: 2px; max-width: 250px; text-align: start; }
.llm-entity-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.llm-entity-id { color: var(--text-secondary); font-size: .75rem; font-family: var(--font-mono); line-height: 1.5; overflow-wrap: anywhere; }
.llm-link .llm-entity-id { text-decoration: none; }
/* The name and its identifier must stay readable; the table scrolls instead. */
.llm-list th:nth-child(2), .llm-list td:nth-child(2) { min-width: 260px; }
.llm-list th:nth-child(3), .llm-list td:nth-child(3) { min-width: 200px; }
.llm-list th:last-child, .llm-list td:last-child { white-space: nowrap; }
.llm-empty { padding: 96px 24px; display: flex; flex-direction: column; align-items: center; gap: 16px; text-align: center; color: var(--text-secondary); }
.llm-empty i { font-size: 2rem; }
.llm-error { color: var(--color-danger); margin: 0; }
.llm-retry { display: flex; align-items: center; flex-wrap: wrap; gap: 16px; }
.llm-dialog { color: var(--text-primary); background: var(--bg-body); border: 1px solid var(--border-color); border-radius: var(--radius-modal); padding: 24px; width: min(1120px, calc(100vw - 48px)); max-height: calc(100dvh - 48px); overflow-y: auto; }
.llm-dialog::backdrop { background: color-mix(in srgb, var(--bg-body) 80%, transparent); }
.llm-dialog-header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 16px; margin-bottom: 16px; position: sticky; top: -24px; padding: 16px 0; background: var(--bg-body); z-index: 1; }
.llm-dialog h2 { margin: 0; font-size: 1.125rem; }
.llm-detail-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; padding: 16px 0; }
.llm-detail-grid dt { font-size: .8125rem; color: var(--text-secondary); font-weight: 400; margin-bottom: 8px; }
.llm-detail-grid dd { margin: 0; overflow-wrap: anywhere; }
.llm-detail-tokens { padding: 16px; background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-card); display: flex; flex-wrap: wrap; gap: 24px; }
.llm-detail-tokens div { display: flex; flex-direction: column; gap: 8px; }
.llm-detail-tokens span { font-size: .8125rem; color: var(--text-secondary); }
.llm-payload { margin-top: 24px; }
.llm-dialog h3 { font-size: 1rem; margin: 16px 0; }
.llm-dialog pre { white-space: pre-wrap; overflow-wrap: anywhere; padding: 16px; background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-card); max-height: 480px; overflow-y: auto; color: var(--text-primary); font-size: .8125rem; }
.llm-link:focus-visible, .llm-dialog pre:focus-visible { outline: 2px solid var(--color-brand); outline-offset: 4px; }
@media (max-width: 1200px) { .llm-overview { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 768px) { .llm-filters { grid-template-columns: repeat(2, minmax(0, 1fr)); } .llm-detail-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 480px) { .llm-filters, .llm-overview, .llm-detail-grid { grid-template-columns: minmax(0, 1fr); } .llm-search, .llm-filter-actions { grid-column: auto; } }
</style>
