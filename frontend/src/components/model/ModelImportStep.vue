<template>
  <div class="model-import">
    <!-- ① 选择文件 / Choose the archive -->
    <div v-if="!started" class="model-import__pick">
      <label
        class="model-import__drop"
        :class="{ 'model-import__drop--dragging': dragging, 'model-import__drop--disabled': selectionLocked }"
        :aria-disabled="selectionLocked"
        @dragenter.prevent="onDragEnter"
        @dragover.prevent="onDragOver"
        @dragleave.prevent="onDragLeave"
        @drop.prevent="onDrop"
      >
        <i class="bi bi-file-earmark-zip" aria-hidden="true"></i>
        <span class="model-import__drop-title">{{ copy.distImportChoose }}</span>
        <span class="model-import__drop-hint">{{ copy.distImportHelp }}</span>
        <input
          ref="fileInput"
          class="model-import__input"
          type="file"
          accept=".zip,application/zip"
          :disabled="selectionLocked"
          :aria-label="copy.distImportChoose"
          @change="onSelect"
        >
        <span v-if="archive" class="model-import__drop-cta">{{ archive.name }}</span>
      </label>

      <p v-if="archive" class="model-import__selected">
        {{ copy.selectedFile }}: {{ archive.name }} · {{ modelBytes(archive.size) }}
      </p>

      <div class="model-import__actions">
        <LiquidGlassButton
          variant="primary"
          icon="bi bi-box-arrow-in-down"
          :disabled="!archive || selectionLocked"
          :loading="working"
          @click="start"
        >{{ copy.distImportAction }}</LiquidGlassButton>
        <LiquidGlassButton
          v-if="archive"
          variant="outline-secondary"
          :disabled="selectionLocked"
          @click="reset"
        >{{ copy.cancel }}</LiquidGlassButton>
      </div>

      <!-- 上传/解析失败：就地反馈，避免用户找不到原因 -->
      <p v-if="failure" class="model-import__warn" role="alert">
        <i class="bi bi-exclamation-triangle" aria-hidden="true"></i>
        {{ modelErrorText(failure, locale) }}
      </p>
    </div>

    <!-- ② 导入过程与结果 / Progress and outcome -->
    <template v-else>
      <!-- 四阶段步骤条与总进度：仅在真正跟踪任务时出现 -->
      <template v-if="tracking">
        <ol class="model-import__steps" :aria-label="copy.distImportTitle">
          <li
            v-for="(step, index) in steps"
            :key="step.id"
            class="model-import__step"
            :class="{
              'model-import__step--active': step.id === progress.phase,
              'model-import__step--done': step.done,
              'model-import__step--failed': step.failed,
            }"
            :aria-current="step.id === progress.phase ? 'step' : undefined"
          >
            <span class="model-import__step-mark" aria-hidden="true">
              <i :class="['bi', step.failed ? 'bi-x-lg' : step.done ? 'bi-check-lg' : step.icon]"></i>
            </span>
            <span class="model-import__step-label">{{ copy[step.key] }}</span>
            <span v-if="index < steps.length - 1" class="model-import__step-line" aria-hidden="true"></span>
          </li>
        </ol>

        <div class="model-import__bar" role="status" aria-live="polite">
          <div class="model-import__track">
            <div
              class="model-import__fill"
              :class="{ 'model-import__fill--live': progress.indeterminate && !finished }"
              :style="{ width: `${progress.percent}%` }"
            ></div>
          </div>
          <span class="model-import__bar-text">
            {{ running ? copy.importRunning : succeeded ? copy.importSucceeded : copy.importFailed }}
            <strong v-if="!progress.indeterminate">{{ progress.percent }}%</strong>
          </span>
        </div>
      </template>

      <!-- 结果汇总 / Outcome summary -->
      <section v-if="finished" class="model-import__result" :class="succeeded ? 'model-import__result--ok' : 'model-import__result--bad'" aria-live="polite">
        <p class="model-import__result-head">
          <i :class="['bi', succeeded ? 'bi-check-circle-fill' : 'bi-exclamation-triangle']" aria-hidden="true"></i>
          <strong>{{ succeeded ? copy.importSucceeded : copy.importFailed }}</strong>
          <span v-if="succeeded" class="model-import__result-counts">
            <span class="model-import__pill model-import__pill--ok">{{ copy.importActivatedCount.replace('{count}', activatedRows.length) }}</span>
            <span v-if="pendingRows.length" class="model-import__pill model-import__pill--warn">{{ copy.importPendingCount.replace('{count}', pendingRows.length) }}</span>
          </span>
        </p>

        <!-- 失败原因 / Failure reason -->
        <p v-if="!succeeded" class="model-import__hint">{{ modelErrorText({ reason_code: job.errorCode }, locale) }}</p>

        <!-- 待启用清单：给出原因与就地启用入口 -->
        <template v-if="pendingRows.length">
          <p class="model-import__hint">{{ copy.importPendingHint }}</p>
          <ul class="model-import__pending">
            <li v-for="item in pendingRows" :key="`${item.packageId}@${item.version}`" class="model-import__pending-row">
              <span class="model-import__pending-name">
                <strong>{{ rowTitleFor(item.packageId) }}</strong>
                <code>{{ item.packageId }} · {{ item.version }}</code>
              </span>
              <span class="model-import__pending-reason">{{ modelPendingReasonText(item.reason, locale) }}</span>
              <LiquidGlassButton
                v-if="item.reason !== 'missing_dependency'"
                variant="outline-primary"
                size="sm"
                :disabled="busy"
                @click="$emit('activate', item)"
              >{{ copy.importActivateNow }}</LiquidGlassButton>
            </li>
          </ul>
        </template>

        <div class="model-import__actions">
          <LiquidGlassButton v-if="succeeded" variant="primary" icon="bi bi-grid-3x3-gap" @click="$emit('view-capabilities')">{{ copy.importViewCapabilities }}</LiquidGlassButton>
          <LiquidGlassButton v-if="job.retryable" variant="outline-primary" icon="bi bi-arrow-clockwise" :disabled="busy" @click="$emit('retry', job)">{{ copy.retry }}</LiquidGlassButton>
          <LiquidGlassButton variant="outline-secondary" :disabled="selectionLocked" @click="reset">{{ copy.distImportAgain }}</LiquidGlassButton>
        </div>
      </section>

      <!-- 解析结果：程序识别出的内容（安装中实时更新状态） / Parsed contents -->
      <section v-if="plan" class="model-import__section" aria-live="polite">
        <header class="model-import__plan-head">
          <h3>{{ copy.distPlanTitle }}</h3>
          <span class="model-import__plan-count">{{ copy.distPlanPackages.replace('{count}', planRows.length) }}</span>
        </header>

        <!-- 缺依赖提示 / Missing built-in dependencies -->
        <p v-if="missingDependencies.length" class="model-import__warn">
          <i class="bi bi-exclamation-triangle" aria-hidden="true"></i>
          {{ copy.distPlanMissingDependency.replace('{names}', missingDependencies.join(', ')) }}
        </p>

        <div class="model-import__table" role="table" :aria-label="copy.a11yImportTable">
          <div class="model-import__thead" role="row">
            <span role="columnheader">{{ copy.colPackage }}</span>
            <span role="columnheader">{{ copy.version }}</span>
            <span role="columnheader">{{ copy.colState }}</span>
            <span role="columnheader">{{ copy.distPlanCapabilities }}</span>
          </div>
          <div
            v-for="row in planRows"
            :key="`${row.pkg.id}@${row.pkg.version}`"
            class="model-import__trow"
            role="row"
          >
            <span class="model-import__cell model-import__cell--pkg" role="cell">
              <i :class="['bi', kindIcon(row.pkg.kind), 'model-import__icon']" aria-hidden="true"></i>
              <span class="model-import__name">
                <strong>{{ rowTitle(row.pkg) }}</strong>
                <code>{{ row.pkg.id }}</code>
              </span>
            </span>
            <span class="model-import__cell model-import__cell--version" role="cell">
              {{ row.pkg.version }}
              <small v-if="row.previous && row.kind === 'replace'">{{ copy.importPreviousVersion.replace('{version}', row.previous.version) }}</small>
            </span>
            <span class="model-import__cell" role="cell">
              <span class="model-import__pill" :class="`model-import__pill--${statusClass(row)}`">
                <i v-if="statusClass(row) === 'installing'" class="bi bi-arrow-repeat model-import__spin" aria-hidden="true"></i>
                {{ statusText(row) }}
              </span>
            </span>
            <span class="model-import__cell model-import__cell--caps" role="cell">
              <span v-for="name in rowCapabilities(row.pkg)" :key="name" class="model-import__chip">{{ name }}</span>
              <span v-if="!rowCapabilities(row.pkg).length" class="model-import__muted">{{ copy.none }}</span>
            </span>
          </div>
        </div>

        <p v-if="plan.offline" class="model-import__hint">{{ copy.distPlanOffline }}</p>
        <p v-if="plan.blocked" class="model-import__warn">
          <i class="bi bi-slash-circle" aria-hidden="true"></i>
          {{ copy.distPlanBlocked }}
        </p>

        <!-- 计划不可安装：停在解析结果，让用户看清原因后再决定 -->
        <div v-if="!tracking" class="model-import__actions">
          <p v-if="!plan.blocked" class="model-import__hint">{{ copy.previewImportHelp }}</p>
          <p v-if="failure" class="model-import__warn" role="alert">{{ modelErrorText(failure, locale) }}</p>
          <LiquidGlassButton v-if="!plan.blocked && plan.packages.length" variant="primary" :disabled="busy || working" @click="submitPlan">{{ failure ? copy.submitImportRetry : copy.confirmImport }}</LiquidGlassButton>
          <LiquidGlassButton variant="outline-secondary" icon="bi bi-arrow-counterclockwise" :disabled="selectionLocked" @click="reset">{{ copy.distImportAgain }}</LiquidGlassButton>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import LiquidGlassButton from '../liquid-glass/LiquidGlassButton.vue';
import {
  modelBytes,
  modelCenterCopy,
  modelErrorText,
  modelImportPhases,
  modelImportProgress,
  modelImportRowState,
  modelPendingReasonText,
  modelPlanMissingDependencies,
  modelPlanRows,
  modelText,
  normalizeModelJob,
  normalizeModelPlan,
  unwrapModelResponse,
} from '../../utils/modelCenter.js';

const props = defineProps({
  catalog: { type: Object, required: true },
  /** 由页面轮询维护的安装任务；组件只读取，不创建。 */
  job: { type: Object, default: null },
  /** 页面正在执行其它变更操作（如启用待启用项）时为 true。 */
  busy: { type: Boolean, default: false },
  submit: { type: Function, required: true },
});

const emit = defineEmits(['start', 'activate', 'retry', 'view-capabilities']);

const { locale } = useI18n();
const copy = computed(() => modelCenterCopy(locale.value));

const fileInput = ref(null);
const archive = ref(null);
const plan = ref(null);
const uploading = ref(false);
const submitting = ref(false);
const uploadPercent = ref(0);
const failure = ref(null);
const idempotencyKey = ref('');
const dragging = ref(false);
let dragDepth = 0;
/**
 * 本次导入会话标记。`job` 是页面轮询维护的任务对象，会跨越多次导入留在 props 里，
 * 因此不能单独用它判断「是否已开始」，否则重置后再选文件会直接跳到结果区。
 */
const sessionActive = ref(false);

const working = computed(() => uploading.value || submitting.value);
const selectionLocked = computed(() => working.value || props.busy || (sessionActive.value && Boolean(props.job) && !props.job.terminal));
const started = computed(() => Boolean(archive.value) && sessionActive.value);
/** 是否处于「有进度可跟踪」的状态：上传/解析中，或已经创建了安装任务。 */
const tracking = computed(() => working.value || Boolean(props.job));
const finished = computed(() => Boolean(props.job?.terminal) && sessionActive.value);
const succeeded = computed(() => finished.value && props.job?.state === 'succeeded');
const running = computed(() => tracking.value && !finished.value);

/** 逐包分类（新增 / 替换待激活 / 已有可用），由 plan + catalog 推导。 */
const planRows = computed(() => modelPlanRows(plan.value, props.catalog));
const missingDependencies = computed(() => modelPlanMissingDependencies(plan.value, props.catalog));

const progress = computed(() => modelImportProgress({
  uploading: uploading.value,
  uploadPercent: uploadPercent.value,
  parsed: Boolean(plan.value),
  job: props.job,
}));

/** 阶段条：done 由 phase 顺序推导，失败时把当前阶段标红。 */
const steps = computed(() => {
  const order = modelImportPhases;
  const current = order.indexOf(progress.value.phase);
  const state = { upload: 'bi-cloud-arrow-up', parse: 'bi-file-earmark-text', install: 'bi-box-arrow-in-down', activate: 'bi-lightning-charge' };
  const label = { upload: 'phaseUpload', parse: 'phaseParse', install: 'phaseInstall', activate: 'phaseActivate' };
  return order.map((id, index) => ({
    id,
    key: label[id],
    icon: state[id],
    // phase 'done' 时全部完成；失败时停在安装阶段并标记
    done: progress.value.phase === 'done' || index < current,
    failed: index === current && finished.value && !succeeded.value,
  }));
});

/** 结果汇总：已启用 / 待启用。 */
const activatedRows = computed(() => Array.isArray(props.job?.activated) ? props.job.activated : []);
const pendingRows = computed(() => Array.isArray(props.job?.pending) ? props.job.pending : []);

const rowTitle = pkg => modelText(pkg.title, locale.value, pkg.id);
const rowTitleFor = id => {
  const pkg = props.catalog.packages.find(item => item.id === id);
  return pkg ? rowTitle(pkg) : id;
};
const kindIcons = {
  general: 'bi-cpu', helmet: 'bi-shield-check', pose: 'bi-person-walking',
  smoking: 'bi-fire', chef_hat: 'bi-person-exclamation', chef_hat_hairnet: 'bi-person-badge',
  kitchen: 'bi-cup-hot', fighting: 'bi-people',
};
const kindIcon = kind => kindIcons[kind] || 'bi-box';

/** 状态列：任务已创建后展示安装状态，否则展示计划预测的安装结果。 */
const rowStatus = row => modelImportRowState(row.pkg, props.job);
const statusClass = row => {
  if (!props.job) return row.kind;
  const state = rowStatus(row);
  return { activated: 'ok', installed: 'reuse', pending: 'warn', failed: 'replace' }[state] || state;
};
const statusText = row => {
  if (!props.job) return ({ new: copy.value.distPlanNew, replace: copy.value.distPlanReplace, reuse: copy.value.distPlanReuse })[row.kind] || row.kind;
  const state = rowStatus(row);
  return {
    wait: copy.value.importRowWait,
    installing: copy.value.importRowInstalling,
    activating: copy.value.importRowActivating,
    activated: copy.value.importRowActivated,
    pending: copy.value.importRowPending,
    installed: copy.value.importRowInstalled,
    failed: copy.value.importRowFailed,
  }[state] || state;
};

/** 该包在离线包内服务的能力名称（用计划里的 usage，避免依赖目标机 catalog）。 */
function rowCapabilities(pkg) {
  const ids = [...new Set([
    ...(pkg.usage?.capabilities || []),
    ...(pkg.usage?.requiredBy || []),
  ])];
  return ids.map(id => {
    const capability = props.catalog.capabilities.find(item => item.id === id);
    return capability ? modelText(capability.title, locale.value, id) : id;
  });
}

function selectArchive(files) {
  if (selectionLocked.value) return;
  const selected = Array.from(files || []);
  if (!selected.length) return;
  plan.value = null;
  failure.value = null;
  idempotencyKey.value = '';
  sessionActive.value = false;
  archive.value = selected.length === 1 ? selected[0] : null;
  if (selected.length !== 1 || !selected[0].name.toLowerCase().endsWith('.zip')) {
    archive.value = null;
    failure.value = { reason_code: selected.length !== 1 ? 'archive_multiple_files' : 'archive_file_type' };
  }
  if (fileInput.value) fileInput.value.value = '';
}

function onSelect(event) {
  selectArchive(event.target.files);
}

function onDragEnter(event) {
  if (selectionLocked.value || !Array.from(event.dataTransfer?.types || []).includes('Files')) return;
  dragDepth += 1;
  dragging.value = true;
}

function onDragOver(event) {
  if (event.dataTransfer) event.dataTransfer.dropEffect = selectionLocked.value ? 'none' : 'copy';
}

function onDragLeave() {
  dragDepth = Math.max(0, dragDepth - 1);
  dragging.value = dragDepth > 0;
}

function onDrop(event) {
  dragDepth = 0;
  dragging.value = false;
  selectArchive(event.dataTransfer?.files);
}

/**
 * 上传与解析在此完成，用户核对后确认安装。
 * 页面通过 submit Promise 创建任务并轮询，成功/失败显式回流。
 */
async function start() {
  if (!archive.value || selectionLocked.value) return;
  const form = new FormData();
  form.append('file', archive.value);
  uploadPercent.value = 0;
  failure.value = null;
  uploading.value = true;
  sessionActive.value = true;
  // 通知页面丢弃上一轮的安装任务，避免旧结果污染本轮进度显示。
  emit('start');
  try {
    const response = await axios.request({
      method: 'post',
      url: '/api/ai/models/import?single=true',
      data: form,
      timeout: 0,
      onUploadProgress: event => { if (event.total) uploadPercent.value = Math.floor(event.loaded / event.total * 100); },
    });
    plan.value = normalizeModelPlan(unwrapModelResponse(response));
    // 计划不可安装时停在解析结果，让用户看清原因而不是发一个注定失败的任务。
    if (!plan.value.packages.length || plan.value.blocked || !plan.value.id) return;
    idempotencyKey.value = globalThis.crypto?.randomUUID?.() || `model-${Date.now()}-${Math.random().toString(36).slice(2)}`;
  } catch (error) {
    failure.value = error;
    archive.value = null;
    plan.value = null;
    sessionActive.value = false;
  } finally {
    uploading.value = false;
  }
}

async function submitPlan() {
  if (!plan.value?.id || plan.value.blocked || working.value || props.busy) return;
  submitting.value = true;
  failure.value = null;
  try { await props.submit({ plan: plan.value, idempotencyKey: idempotencyKey.value }); }
  catch (error) { failure.value = error; }
  finally { submitting.value = false; }
}

// 已创建的任务回流后也可解除提交进度。
watch(() => props.job?.id, id => { if (id) submitting.value = false; });

function reset() {
  if (selectionLocked.value) return;
  archive.value = null;
  plan.value = null;
  failure.value = null;
  idempotencyKey.value = '';
  uploading.value = false;
  submitting.value = false;
  uploadPercent.value = 0;
  sessionActive.value = false;
  dragDepth = 0;
  dragging.value = false;
  if (fileInput.value) fileInput.value.value = '';
}
</script>

<style scoped>
.model-import { display: flex; flex-direction: column; gap: 20px; min-width: 0; }

/* 选择文件 / File picker */
.model-import__pick { display: flex; flex-direction: column; gap: 16px; }
.model-import__drop {
  display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 8px;
  padding: 32px 24px;
  border: 1px dashed var(--border-color);
  border-radius: var(--radius-card);
  background: var(--bg-surface);
  color: var(--text-secondary);
  text-align: center;
  cursor: pointer;
  transition: border-color var(--noyo-duration-fast, 0.15s) var(--noyo-ease-standard, ease), background var(--noyo-duration-fast, 0.15s) var(--noyo-ease-standard, ease);
}
.model-import__drop:focus-within,
.model-import__drop:not(.model-import__drop--disabled):hover,
.model-import__drop--dragging { border-color: var(--color-brand); background: var(--bg-hover); }
.model-import__drop:focus-within { outline: 2px solid var(--color-brand); outline-offset: 8px; }
.model-import__drop--disabled { cursor: default; opacity: 0.6; }
.model-import__drop i { font-size: 1.75rem; color: var(--color-brand); }
.model-import__drop-title { color: var(--text-primary); font-size: 0.875rem; font-weight: 600; }
.model-import__drop-hint { font-size: 0.75rem; line-height: 1.5; max-width: 48ch; }
.model-import__drop-cta { margin-top: 8px; font-family: var(--font-mono); font-size: 0.75rem; color: var(--color-brand); overflow-wrap: anywhere; }
.model-import__input { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }

.model-import__selected { margin: 0; color: var(--text-secondary); font-size: 0.75rem; overflow-wrap: anywhere; }
.model-import__actions { display: flex; flex-wrap: wrap; gap: 8px; }

/* 四阶段步骤条 / Four-phase stepper */
.model-import__steps { display: flex; flex-wrap: wrap; gap: 8px; margin: 0; padding: 0; list-style: none; }
.model-import__step { position: relative; display: flex; align-items: center; gap: 8px; color: var(--text-muted); font-size: 0.8125rem; }
.model-import__step:not(:last-child) { margin-right: 20px; }
.model-import__step-line { position: absolute; left: 100%; top: 50%; width: 20px; height: 1px; margin-left: 2px; background: var(--border-color); }
.model-import__step-mark {
  display: inline-flex; align-items: center; justify-content: center;
  width: 22px; height: 22px;
  border: 1px solid var(--border-color);
  border-radius: var(--noyo-radius-pill, 999px);
  background: var(--bg-surface);
  font-size: 0.6875rem;
}
.model-import__step--active { color: var(--color-brand); }
.model-import__step--active .model-import__step-mark { border-color: var(--color-brand); color: var(--color-brand); box-shadow: 0 0 0 3px var(--color-brand-subtle); }
.model-import__step--done { color: var(--text-secondary); }
.model-import__step--done .model-import__step-mark { border-color: var(--color-brand); background: var(--color-brand); color: var(--bg-surface); }
.model-import__step--failed { color: var(--color-danger); }
.model-import__step--failed .model-import__step-mark { border-color: var(--color-danger); color: var(--color-danger); }

/* 总进度 / Overall progress */
.model-import__bar { display: flex; flex-direction: column; gap: 8px; }
.model-import__track { height: 6px; border-radius: var(--noyo-radius-pill, 999px); background: var(--bg-elevated); overflow: hidden; }
.model-import__fill { height: 100%; border-radius: inherit; background: var(--color-brand); transition: width var(--noyo-duration-standard, 0.25s) var(--noyo-ease-standard, ease); }
.model-import__fill--live { animation: model-import-pulse 1.4s var(--noyo-ease-standard, ease-in-out) infinite; }
@keyframes model-import-pulse { 0%, 100% { opacity: 1; } 50% { opacity: 0.45; } }
.model-import__bar-text { display: flex; align-items: baseline; gap: 8px; color: var(--text-secondary); font-size: 0.75rem; }
.model-import__bar-text strong { color: var(--text-primary); font-variant-numeric: tabular-nums; }

/* 结果汇总 / Outcome */
.model-import__result { display: flex; flex-direction: column; gap: 16px; padding: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); }
.model-import__result--ok { border-color: color-mix(in srgb, var(--color-brand) 40%, var(--border-color)); }
.model-import__result--bad { border-color: color-mix(in srgb, var(--color-danger) 40%, var(--border-color)); }
.model-import__result-head { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin: 0; color: var(--text-primary); font-size: 0.875rem; }
.model-import__result--ok .model-import__result-head > i { color: var(--color-brand); }
.model-import__result--bad .model-import__result-head > i { color: var(--color-danger); }
.model-import__result-counts { display: flex; flex-wrap: wrap; gap: 8px; }

.model-import__pending { display: flex; flex-direction: column; gap: 8px; margin: 0; padding: 0; list-style: none; }
.model-import__pending-row { display: flex; flex-wrap: wrap; align-items: center; gap: 16px; padding: 8px 16px; border-radius: var(--radius-control); background: var(--bg-elevated); }
.model-import__pending-name { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.model-import__pending-name strong { color: var(--text-primary); font-size: 0.8125rem; }
.model-import__pending-name code { color: var(--text-muted); font-family: var(--font-mono); font-size: 0.6875rem; }
.model-import__pending-reason { flex: 1 1 18ch; min-width: 0; color: var(--text-secondary); font-size: 0.75rem; }

/* 解析结果 / Parsed plan */
.model-import__section { display: flex; flex-direction: column; gap: 16px; }
.model-import__plan-head { display: flex; align-items: baseline; justify-content: space-between; gap: 16px; }
.model-import__plan-head h3 { margin: 0; color: var(--text-primary); font-size: 0.9375rem; font-weight: 600; }
.model-import__plan-count { color: var(--text-muted); font-size: 0.75rem; }

.model-import__warn { display: flex; align-items: flex-start; gap: 8px; margin: 0; padding: 10px 12px; border-radius: var(--radius-control); background: color-mix(in srgb, var(--color-warning) 12%, var(--bg-surface)); color: var(--color-warning); font-size: 0.75rem; line-height: 1.55; }
.model-import__hint { margin: 0; color: var(--text-muted); font-size: 0.75rem; line-height: 1.55; }

.model-import__table { display: flex; flex-direction: column; min-width: 0; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); overflow: hidden; }
.model-import__thead,
.model-import__trow {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 0.6fr) minmax(0, 1.1fr) minmax(0, 1.6fr);
  gap: 16px;
  align-items: center;
  padding: 16px;
  min-width: 0;
}
.model-import__thead { border-bottom: 1px solid var(--border-color); background: var(--bg-elevated); color: var(--text-secondary); font-size: 0.75rem; font-weight: 600; letter-spacing: 0.04em; }
.model-import__trow { border-bottom: 1px solid var(--border-color); }
.model-import__trow:last-child { border-bottom: 0; }
.model-import__cell { min-width: 0; font-size: 0.8125rem; color: var(--text-primary); }
.model-import__cell--pkg { display: flex; align-items: flex-start; gap: 8px; }
.model-import__cell--version { font-family: var(--font-mono); font-size: 0.8125rem; }
.model-import__cell--caps { display: flex; flex-wrap: wrap; gap: 8px; }
.model-import__icon { color: var(--color-brand); font-size: 1rem; line-height: 1.5; flex-shrink: 0; }
.model-import__name { min-width: 0; }
.model-import__name strong { display: block; color: var(--text-primary); font-size: 0.875rem; line-height: 1.4; overflow-wrap: anywhere; }
.model-import__name code { display: block; color: var(--text-muted); font-family: var(--font-mono); font-size: 0.6875rem; overflow-wrap: anywhere; }

.model-import__pill { display: inline-flex; align-items: center; gap: 8px; padding: 0 8px; border-radius: var(--noyo-radius-pill, 999px); font-size: 0.75rem; white-space: nowrap; }
.model-import__pill--new,
.model-import__pill--installing { color: var(--color-brand); background: var(--color-brand-subtle); }
.model-import__pill--ok,
.model-import__pill--activated { color: var(--color-brand); background: var(--color-brand-subtle); }
.model-import__pill--replace,
.model-import__pill--pending,
.model-import__pill--warn { color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 12%, var(--bg-surface)); }
.model-import__pill--reuse,
.model-import__pill--wait,
.model-import__pill--installed { color: var(--text-muted); background: var(--bg-elevated); }
.model-import__pill--activating { color: var(--color-brand); background: var(--color-brand-subtle); }
.model-import__pill--failed { color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 12%, var(--bg-surface)); }
.model-import__spin { animation: model-import-spin 1s linear infinite; }
@keyframes model-import-spin { to { transform: rotate(360deg); } }

.model-import__chip { padding: 0 8px; border-radius: var(--noyo-radius-pill, 999px); background: var(--bg-elevated); color: var(--text-secondary); font-size: 0.6875rem; white-space: nowrap; max-width: 140px; overflow: hidden; text-overflow: ellipsis; }
.model-import__muted { color: var(--text-muted); font-size: 0.75rem; }

@media (max-width: 1100px) {
  .model-import__thead { display: none; }
  .model-import__trow { grid-template-columns: minmax(0, 1fr); gap: 8px; }
}
</style>
