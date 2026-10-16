<template>
  <div class="model-center-page page-fixed-height">
    <div class="page-header">
      <div><h1>{{ copy.title }}</h1><p class="page-subtitle">{{ copy.subtitle }}</p></div>
      <LiquidGlassButton variant="outline-secondary" size="sm" icon="bi bi-arrow-clockwise" :loading="loading" :disabled="loading || !!busy" @click="refreshCatalog">{{ copy.refresh }}</LiquidGlassButton>
    </div>
    <div class="model-content">
      <nav class="model-tabs" :aria-label="copy.tabs">
        <button v-for="tab in visibleTabs" :key="tab.id" type="button" class="model-tab" :class="{ 'model-tab--active': activeTab === tab.id }" :aria-pressed="activeTab === tab.id" @click="switchTab(tab.id)">{{ copy[tab.key] }}</button>
      </nav>
      <div v-if="error" class="model-feedback model-feedback--danger" role="alert">{{ errorText }}</div>

      <template v-if="activeTab === 'capabilities'">
        <ModelSkeleton v-if="loading && !catalog.capabilities.length" variant="cards" :count="6" :label="copy.loadingCatalog" />
        <ModelEmptyState v-else-if="!capabilities.length" icon="bi-cpu" :message="copy.emptyScenes" />
        <div v-else class="model-capability-grid" role="list" :aria-label="copy.a11yCapabilityList">
          <CapabilityCard v-for="capability in capabilities" :key="capability.id" :capability="capability" />
        </div>
      </template>

      <template v-else>
        <div class="model-library-tools">
          <div class="model-filters" role="group" :aria-label="copy.colState">
            <button v-for="filter in libraryFilters" :key="filter" type="button" class="model-tab" :class="{ 'model-tab--active': libraryFilter === filter }" :aria-pressed="libraryFilter === filter" @click="libraryFilter = filter">{{ copy[filter] }}</button>
          </div>
          <input v-model.trim="search" class="form-control form-control-sm model-search" type="search" :placeholder="copy.libSearch" :aria-label="copy.libSearch">
          <LiquidGlassButton v-if="catalog.canManage" variant="primary" size="sm" icon="bi bi-box-arrow-in-down" :disabled="!!busy" :aria-expanded="updateOpen" aria-controls="model-update" @click="updateOpen = !updateOpen">{{ updateOpen ? copy.closeUpdate : copy.updateModels }}</LiquidGlassButton>
          <LiquidGlassButton variant="outline-secondary" size="sm" icon="bi bi-download" :loading="exporting" :disabled="loading || exporting || !!busy" @click="exportModels">{{ copy.exportModels }}</LiquidGlassButton>
        </div>
        <section v-show="updateOpen && catalog.canManage" id="model-update" class="model-section" :aria-label="copy.updateModels">
          <h2>{{ copy.updateModels }}</h2>
          <ModelImportStep :catalog="catalog" :job="importJob" :busy="!!busy" :visible="updateOpen && catalog.canManage" :submit="importOffline" @start="startImport" @activate="activatePending" @retry="retryJob" @view-capabilities="switchTab('capabilities')" />
        </section>
        <ModelSkeleton v-if="loading && !catalog.packages.length" variant="table" :count="6" :label="copy.loadingLibrary" />
        <ModelEmptyState v-else-if="!catalog.packages.length" icon="bi-box" :message="copy.emptyInstalledModels" />
        <ModelLibraryTable v-else :rows="filteredPackageRows" :catalog="catalog" @view-versions="openVersions" @view-history="openHistory" @clear-search="search = ''; libraryFilter = 'all'" />
      </template>
    </div>

    <div v-if="activeTab === 'library' && activeJobs.length" class="model-activity" role="region" :aria-label="copy.a11yJobRegion">
      <div v-for="job in activeJobs" :key="job.id" class="model-activity__row">
        <span>{{ packageNames(job.packageId ? [job.packageId] : []) }}</span>
        <progress :value="job.percent" max="100" :aria-label="copy.a11yJobProgress.replace('{name}', packageNames(job.packageId ? [job.packageId] : []) || job.id)"></progress>
        <span>{{ job.percent }}%</span>
        <LiquidGlassButton v-if="catalog.canManage" variant="outline-danger" size="sm" :disabled="!!busy" @click="cancelJob(job)">{{ copy.cancel }}</LiquidGlassButton>
      </div>
    </div>
    <ModelPackageDrawer :visible="drawerView === 'versions'" :row="selectedPackageRow" :catalog="catalog" :busy="!!busy" :error="error ? errorText : ''" @close="closeDrawer" @activate="activatePackage" @uninstall="uninstallPackage" @verify="verifyPackage" />
    <ModelHistoryDrawer :visible="drawerView === 'history'" :row="selectedPackageRow" :items="historyItems" :total="historyTotal" :page="historyPage" :page-size="historyPageSize" :loading="historyLoading" :error="historyErrorText" @close="closeDrawer" @retry="loadHistory(historyPage)" @page="loadHistory" />
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import LiquidGlassButton from '../components/liquid-glass/LiquidGlassButton.vue';
import CapabilityCard from '../components/model/CapabilityCard.vue';
import ModelLibraryTable from '../components/model/ModelLibraryTable.vue';
import ModelPackageDrawer from '../components/model/ModelPackageDrawer.vue';
import ModelHistoryDrawer from '../components/model/ModelHistoryDrawer.vue';
import ModelImportStep from '../components/model/ModelImportStep.vue';
import ModelSkeleton from '../components/model/ModelSkeleton.vue';
import ModelEmptyState from '../components/model/ModelEmptyState.vue';
import { useConfirm } from '../composables/useConfirm.js';
import { useToast } from '../composables/useToast.js';
import { modelCenterCopy, modelErrorText, modelPackageActionAllowed, modelLibraryRows, modelSystemCapabilities, modelText, normalizeModelCatalog, normalizeModelJob, unwrapModelResponse } from '../utils/modelCenter.js';

const API = '/api/ai/models';
const { locale } = useI18n();
const { confirmDialog } = useConfirm();
const { showToast } = useToast();
const copy = computed(() => modelCenterCopy(locale.value));
const catalog = ref(normalizeModelCatalog());
const jobs = ref([]);
const search = ref('');
const activeTab = ref('capabilities');
const libraryFilters = ['all', 'installed', 'notInstalled'];
const libraryFilter = ref('all');
const updateOpen = ref(false);
const exporting = ref(false);
const loading = ref(false);
const busy = ref('');
const error = ref(null);
const drawerView = ref('');
const selectedPackageId = ref('');
const selectedPackageTitle = ref({});
const historyItems = ref([]);
const historyTotal = ref(0);
const historyPage = ref(1);
const historyPageSize = ref(50);
const historyLoading = ref(false);
const historyError = ref(null);
const historyErrorText = computed(() => historyError.value ? modelErrorText(historyError.value, locale.value, 'historyLoadError') : '');
/** 离线导入任务 id：导入组件完成上传/解析后创建任务，随后由轮询维护。 */
const importJobId = ref('');
const importJob = computed(() => jobs.value.find(job => job.id === importJobId.value) || null);
const errorText = computed(() => modelErrorText(error.value, locale.value));

/** The library supports read-only inventory access. */
const visibleTabs = [{ id: 'capabilities', key: 'tabCapabilities' }, { id: 'library', key: 'tabLibrary' }];

const activeJobs = computed(() => jobs.value.filter(job => !job.terminal));
const capabilities = computed(() => modelSystemCapabilities(catalog.value));
const packageRows = computed(() => modelLibraryRows(catalog.value));
const selectedPackageRow = computed(() => packageRows.value.find(row => row.id === selectedPackageId.value) || { id: selectedPackageId.value, title: selectedPackageTitle.value, releases: [] });
const filteredPackageRows = computed(() => {
  const terms = String(search.value).toLowerCase().trim().split(/\s+/).filter(Boolean);
  return packageRows.value.filter(row => {
    if (libraryFilter.value === 'installed' && !row.installed) return false;
    if (libraryFilter.value === 'notInstalled' && row.installed) return false;
    const haystack = [row.id, row.kind, row.version, ...row.installedVersions, ...Object.values(row.title || {})].join(' ').toLowerCase();
    return terms.every(term => haystack.includes(term));
  });
});

let disposed = false;
let pollTimer;
let jobRevision = 0;
let historyRevision = 0;
const controllers = new Set();
const title = item => modelText(item.title, locale.value, item.id);
const packageKey = pkg => `${pkg.id}@${pkg.version}`;
const packageNames = ids => ids.length ? ids.map(id => title(catalog.value.packages.find(pkg => pkg.id === id) || { id })).join(' · ') : copy.value.none;
const actionAllowed = modelPackageActionAllowed;

function selectPackage(row) {
  selectedPackageId.value = row.id;
  selectedPackageTitle.value = row.title;
}

function closeDrawer() {
  drawerView.value = '';
  ++historyRevision;
  historyLoading.value = false;
}

function openVersions(row) {
  closeDrawer();
  selectPackage(row);
  error.value = null;
  drawerView.value = 'versions';
}

async function openHistory(row) {
  closeDrawer();
  selectPackage(row);
  historyItems.value = [];
  historyTotal.value = 0;
  historyPage.value = 1;
  historyPageSize.value = 50;
  drawerView.value = 'history';
  await loadHistory(1);
}

async function loadHistory(page = 1) {
  if (!selectedPackageId.value || drawerView.value !== 'history') return;
  const revision = ++historyRevision;
  const packageId = selectedPackageId.value;
  historyLoading.value = true;
  historyError.value = null;
  try {
    const data = await request('get', '/history', undefined, { params: { packageId, page, pageSize: 50 } });
    if (disposed || revision !== historyRevision) return;
    historyItems.value = Array.isArray(data?.items) ? data.items : [];
    historyTotal.value = Math.max(0, Number(data?.total) || 0);
    historyPage.value = Math.max(1, Number(data?.page) || page);
    historyPageSize.value = Math.max(1, Number(data?.pageSize) || 50);
  } catch (failure) {
    if (!disposed && revision === historyRevision && !axios.isCancel(failure)) historyError.value = failure;
  } finally { if (!disposed && revision === historyRevision) historyLoading.value = false; }
}

function switchTab(id) {
  if (!visibleTabs.some(tab => tab.id === id)) return;
  activeTab.value = id;
  search.value = '';
}

async function exportModels() {
  if (exporting.value) return;
  exporting.value = true;
  error.value = null;
  let url;
  try {
    const snapshot = await request('get', '/export');
    if (disposed) return;
    url = URL.createObjectURL(new Blob([JSON.stringify(snapshot, null, 2)], { type: 'application/json;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = `noyo-models-${new Date().toISOString().slice(0, 10)}.json`;
    document.body.appendChild(link);
    link.click();
    link.remove();
  } catch (failure) {
    if (!disposed && !axios.isCancel(failure)) error.value = failure;
  } finally {
    if (url) window.setTimeout(() => URL.revokeObjectURL(url), 1000);
    if (!disposed) exporting.value = false;
  }
}

async function request(method, path, data, config = {}) {
  const controller = new AbortController();
  controllers.add(controller);
  try {
    const response = await axios.request({ method, url: `${API}${path}`, data, timeout: method === 'get' ? 15000 : 0, ...config, signal: controller.signal });
    return unwrapModelResponse(response);
  } finally { controllers.delete(controller); }
}

function setJobs(data) {
  const rows = Array.isArray(data) ? data : data?.jobs || data?.installations || [];
  jobs.value = rows.map(normalizeModelJob).sort((a, b) => String(b.createdAt || '').localeCompare(String(a.createdAt || '')));
}

function schedulePoll() {
  window.clearTimeout(pollTimer);
  if (!disposed && activeJobs.value.length) pollTimer = window.setTimeout(pollJobs, 2000);
}

async function pollJobs() {
  const revision = jobRevision;
  const previous = new Set(activeJobs.value.map(job => job.id));
  try {
    const data = await request('get', '/jobs');
    if (disposed || revision !== jobRevision) return;
    setJobs(data);
    if (jobs.value.some(job => previous.has(job.id) && job.terminal)) await refreshCatalog(false);
  } catch (failure) {
    if (!disposed && !axios.isCancel(failure)) error.value = failure;
  } finally { schedulePoll(); }
}

async function refreshCatalog(showLoading = true) {
  if (showLoading) { loading.value = true; error.value = null; }
  try {
    const data = await request('get', '/catalog');
    if (disposed) return;
    catalog.value = normalizeModelCatalog(data);
    setJobs(data.jobs || []);
    if (!visibleTabs.some(tab => tab.id === activeTab.value)) activeTab.value = 'capabilities';
    schedulePoll();
  } catch (failure) {
    if (!disposed && !axios.isCancel(failure)) error.value = failure;
  } finally { if (!disposed) loading.value = false; }
}

async function mutate(key, action) {
  if (busy.value || !catalog.value.canManage) return;
  ++jobRevision;
  window.clearTimeout(pollTimer);
  busy.value = key; error.value = null;
  // 包名必须在操作前解析：卸载后该包已不在新目录中。
  const label = announceLabel(key);
  try {
    await action();
    if (!disposed) {
      await refreshCatalog(false);
      announce(key, label);
    }
  } catch (failure) {
    if (!disposed && !axios.isCancel(failure)) {
      error.value = failure;
      showToast(modelErrorText(failure, locale.value), 'danger');
    }
  } finally { if (!disposed) { busy.value = ''; schedulePoll(); } }
}

/** 操作目标的双语名称（在目录刷新前解析）；包与能力两种目标都支持。 */
function announceLabel(key) {
  const target = key.split(':')[1] || '';
  const pkg = catalog.value.packages.find(row => packageKey(row) === target);
  if (pkg) return title(pkg);
  const capability = catalog.value.capabilities.find(row => row.id === target);
  return capability ? title(capability) : target;
}

/**
 * 操作成功提示（§8.9）。`key` 形如 `activate:<id>` / `verify:<id>@<version>`；
 * 只在需要让用户确认「动作已生效」时提示。
 * 离线导入不在此列：它的进度与结果都在导入视图就地呈现。
 */
function announce(key, label) {
  const action = key.split(':')[0];
  const message = {
    cancel: () => copy.value.toastJobCancelled,
    retry: () => copy.value.toastJobRetried,
    activate: () => copy.value.toastActivated.replace('{name}', label),
    uninstall: () => copy.value.toastUninstalled.replace('{name}', label),
    verify: () => copy.value.toastVerified.replace('{name}', label),
  }[action];
  if (message) showToast(message(), 'success');
}

/**
 * 用户确认解析计划后创建安装任务。Promise 回传错误给导入组件，
 * 任务随后由既有轮询维护，并经 importJob 回流。
 */
async function importOffline({ plan: importedPlan, idempotencyKey: key }) {
  if (busy.value || !catalog.value.canManage || importedPlan.blocked || !importedPlan.packages.length || !importedPlan.id) throw { reason_code: 'conflict' };
  ++jobRevision;
  window.clearTimeout(pollTimer);
  busy.value = 'import';
  error.value = null;
  try {
    const job = normalizeModelJob(await request('post', '/jobs', { planId: importedPlan.id, idempotencyKey: key }));
    if (disposed) throw { reason_code: 'interrupted' };
    importJobId.value = job.id;
    jobs.value = [job, ...jobs.value.filter(row => row.id !== job.id)];
    return job;
  } catch (failure) {
    if (!disposed && !axios.isCancel(failure)) {
      error.value = failure;
      showToast(modelErrorText(failure, locale.value), 'danger');
    }
    throw failure;
  } finally {
    if (!disposed) { busy.value = ''; schedulePoll(); }
  }
}

/**
 * 启用「已装待启用」的包：离线导入时该资源类型仍被占用（或依赖缺失），
 * 用户可在结果区手动催一次。使用目录当前代次做乐观并发保护。
 */
async function activatePending(item) {
  if (busy.value || !catalog.value.canManage) return;
  const expectedGeneration = catalog.value.generation;
  await mutate(`activate:${item.packageId}@${item.version}`, () => request('post', '/activate', {
    packages: { [item.packageId]: item.version },
    expectedGeneration,
  }));
}

/** 用户开始导入：丢弃上一轮的安装任务 id 与上下文提示，避免旧状态污染新进度。 */
function startImport() {
  importJobId.value = '';
}

async function cancelJob(job) {
  if (busy.value || !catalog.value.canManage || job.terminal) return;
  const confirmed = await confirmDialog({ title: copy.value.cancelTitle, message: copy.value.cancelMessage, confirmText: copy.value.cancel, cancelText: copy.value.keepTask, variant: 'danger' });
  if (confirmed) await mutate(`cancel:${job.id}`, () => request('post', `/jobs/${encodeURIComponent(job.id)}/cancel`));
}

async function retryJob(job) {
  if (!job.retryable) return;
  await mutate(`retry:${job.id}`, () => request('post', `/jobs/${encodeURIComponent(job.id)}/retry`));
}

const packagePath = pkg => `/packages/${encodeURIComponent(pkg.id)}/${encodeURIComponent(pkg.version)}`;
/** 校验只读，不改变激活状态，因此使用中的模型也允许校验。 */
async function verifyPackage(pkg) {
  if (busy.value || !catalog.value.canManage || !pkg.installed) return;
  await mutate(`verify:${packageKey(pkg)}`, () => request('post', `${packagePath(pkg)}/verify`));
}

async function activatePackage(pkg) {
  if (busy.value || !catalog.value.canManage || !actionAllowed(pkg, 'activate')) return;
  const expectedGeneration = catalog.value.generation;
  const confirmed = await confirmDialog({ title: copy.value.activateTitle, message: `${title(pkg)} · ${copy.value.version} ${pkg.version}\n${copy.value.activateMessage}`, confirmText: copy.value.activate, cancelText: copy.value.cancel, variant: 'neutral' });
  if (confirmed) await mutate(`activate:${packageKey(pkg)}`, () => request('post', '/activate', { packages: { [pkg.id]: pkg.version }, expectedGeneration }));
}

async function uninstallPackage(pkg) {
  if (busy.value || !catalog.value.canManage || !actionAllowed(pkg, 'uninstall')) return;
  const confirmed = await confirmDialog({ title: copy.value.uninstallTitle, message: `${title(pkg)} · ${copy.value.version} ${pkg.version}\n${copy.value.uninstallMessage}`, confirmText: copy.value.uninstall, cancelText: copy.value.cancel, variant: 'danger' });
  if (confirmed) await mutate(`uninstall:${packageKey(pkg)}`, () => request('delete', packagePath(pkg)));
}

onMounted(() => refreshCatalog());
onUnmounted(() => {
  disposed = true;
  window.clearTimeout(pollTimer);
  for (const controller of controllers) controller.abort();
});
</script>

<style scoped>
.model-center-page { padding: 24px; min-width: 0; }
.page-header { display: flex; align-items: flex-start; justify-content: space-between; flex-wrap: wrap; gap: 16px; }
.model-content { flex: 1; min-height: 0; min-width: 0; overflow-y: auto; padding-bottom: 24px; }
.model-tabs, .model-filters { display: flex; flex-wrap: wrap; gap: 8px; }
.model-tabs { margin-bottom: 24px; }
.model-tab { padding: 8px 16px; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-surface); color: var(--text-secondary); cursor: pointer; }
.model-tab:hover { background: var(--bg-hover); color: var(--text-primary); }
.model-tab--active { background: var(--color-brand-subtle); color: var(--color-brand); border-color: var(--color-brand); }
.model-tab:focus-visible { outline: 2px solid var(--color-brand); outline-offset: 2px; }
.model-capability-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(320px, 100%), 1fr)); gap: 16px; }
.model-library-tools { display: flex; align-items: center; flex-wrap: wrap; gap: 16px; margin-bottom: 24px; }
.model-search { flex: 1 1 240px; width: auto; min-width: 0; }
.model-section { padding: 24px; margin-bottom: 24px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); }
.model-section h2 { margin: 0 0 16px; font-size: 1rem; color: var(--text-primary); }
.model-feedback { padding: 16px; margin-bottom: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-control); }
.model-feedback--danger { color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 8%, var(--bg-surface)); }
.model-activity { flex-shrink: 0; padding: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-surface); }
.model-activity__row { display: flex; align-items: center; flex-wrap: wrap; gap: 16px; color: var(--text-secondary); }
progress { flex: 1 1 160px; min-width: 0; height: 8px; accent-color: var(--color-brand); }
@media (max-width: 768px) { .model-capability-grid { grid-template-columns: minmax(0, 1fr); } .model-search { flex-basis: 100%; } }
</style>
