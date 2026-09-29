<template>
  <section v-if="visible" class="configuration-state-panel noyo-drawer-section" aria-live="polite">
    <div class="configuration-state-panel__heading">
      <h6 class="noyo-drawer-section-title mb-0">{{ copy.title }}</h6>
      <button type="button" class="btn btn-outline-secondary btn-sm" :disabled="loading" @click="load">{{ copy.refresh }}</button>
    </div>
    <p class="small text-secondary mb-0">{{ copy.explanation }}</p>
    <p v-if="error" class="text-danger mb-0" role="alert">{{ data ? copy.stale : copy.failed }}</p>
    <p v-if="loading && !data" class="text-secondary mb-0" role="status">{{ copy.loading }}</p>
    <template v-if="data">
      <div class="configuration-state-panel__group">
        <h6>{{ copy.confirmed }}</h6>
        <p v-if="!resource.projections.length" class="small text-secondary mb-0">{{ copy.noConfirmed }}</p>
        <article v-for="entry in resource.projections" :key="entry.gateway_code" class="configuration-state-panel__entry">
          <div class="configuration-state-panel__heading">
            <span class="font-mono">{{ entry.gateway_code || entityCode }}</span>
          </div>
          <dl class="configuration-state-panel__metadata">
            <dt>{{ copy.revision }}</dt><dd>{{ entry.revision }}</dd>
            <dt>{{ copy.confirmedAt }}</dt><dd>{{ formatDateTime(entry.confirmed_at) }}</dd>
          </dl>
          <p v-if="entry.deleted" class="small mb-0">{{ copy.deleted }}</p>
          <details v-if="entry.payload"><summary>{{ copy.configuration }}</summary><pre>{{ formatConfigurationPayload(entry.payload, activeLocale) }}</pre></details>
        </article>
      </div>
      <div class="configuration-state-panel__group">
        <h6>{{ copy.intents }}</h6>
        <p v-if="!resource.intents.length" class="small text-secondary mb-0">{{ copy.noIntents }}</p>
        <article v-for="entry in resource.intents" :key="`${entry.operation_code}:${entry.gateway_code || entry.target_node}`" class="configuration-state-panel__entry">
          <div class="configuration-state-panel__heading"><span class="font-mono">{{ entry.gateway_code || entry.target_node || entityCode }}</span><span>{{ stateLabel(entry.status) }}</span></div>
          <p class="small mb-0">{{ entry.kind === 'delete' ? copy.deleteIntent : copy.upsertIntent }}</p>
          <p v-if="entry.last_error" class="small text-danger mb-0">{{ backendText(entry.last_error) }}</p>
          <p class="small text-secondary mb-0">{{ copy.savedAt }}: {{ formatDateTime(entry.created_at) }}</p>
          <details v-if="entry.payload"><summary>{{ copy.pendingConfiguration }}</summary><pre>{{ formatConfigurationPayload(entry.payload, activeLocale) }}</pre></details>
          <template v-if="needsReedit(entry)">
            <p class="small text-danger mb-0">{{ copy.reeditHint }}</p>
            <button type="button" class="btn btn-outline-primary btn-sm configuration-state-panel__reedit" @click="requestReedit(entry)">{{ copy.reedit }}</button>
          </template>
        </article>
      </div>
      <div class="configuration-state-panel__group">
        <h6>{{ copy.runtime }}</h6>
        <p v-if="!resource.runtime.length" class="small text-secondary mb-0">{{ copy.noRuntime }}</p>
        <article v-for="entry in resource.runtime" :key="`${entry.gateway_code || 'local'}:${entry.entity_code}`" class="configuration-state-panel__entry">
          <div class="configuration-state-panel__heading"><span class="font-mono">{{ entry.gateway_code || entityCode }}</span><ConfigSyncBadge :state="entry.status === 'pending' ? 'applying' : entry.status" :reason="entry.last_error" /></div>
          <dl class="configuration-state-panel__metadata">
            <dt>{{ copy.desiredRevision }}</dt><dd>{{ entry.revision }}</dd>
            <dt>{{ copy.activeRevision }}</dt><dd>{{ entry.active_revision || (entry.status === 'active' ? entry.revision : copy.unconfirmed) }}</dd>
            <dt>{{ copy.runtimeAt }}</dt><dd>{{ formatDateTime(entry.updated_at) }}</dd>
            <template v-if="entry.compensation_status"><dt>{{ copy.compensation }}</dt><dd>{{ stateLabel(entry.compensation_status) }}</dd></template>
          </dl>
          <p v-if="entry.compensation_error" class="small text-danger mb-0">{{ backendText(entry.compensation_error) }}</p>
          <details v-if="entry.active_payload"><summary>{{ copy.activeConfiguration }}</summary><pre>{{ formatConfigurationPayload(entry.active_payload, activeLocale) }}</pre></details>
        </article>
      </div>
    </template>
  </section>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import ConfigSyncBadge from './ConfigSyncBadge.vue';
import { configurationNeedsAttention, configurationStatus, formatConfigurationPayload, resourceConfiguration } from '../../utils/configurationState.js';
import { formatDateTime } from '../../utils/dateTime.js';
import { resolveServerText } from '../../utils/serverText.js';

const props = defineProps({ entityType: { type: String, required: true }, entityCode: { type: String, required: true }, gatewayCode: { type: String, default: '' } });
const emit = defineEmits(['reedit']);
const { locale } = useI18n();
// Snapshots and backend error copy are localized with the same value the panel
// body switches on, so a console never shows two languages at once.
const activeLocale = computed(() => String(locale.value || ''));
const isZh = computed(() => activeLocale.value.toLowerCase().startsWith('zh'));
const copy = computed(() => isZh.value ? {
  title: '配置提交与运行状态', refresh: '刷新', explanation: '最后确认配置、平台修改意图和现场运行状态分别展示；已提交不代表运行已生效。',
  failed: '配置状态加载失败，请重试。', stale: '配置状态更新失败，正在显示上次结果，可能已过期。', loading: '正在加载配置状态…', confirmed: '最后现场确认配置', noConfirmed: '尚无现场确认记录；待应用修改不能作为已确认配置。',
  revision: '提交版本', confirmedAt: '最后确认时间', deleted: '现场已确认删除此资源。', configuration: '查看确认配置',
  intents: '平台修改意图', noIntents: '没有待应用或失败的修改意图。', pendingConfiguration: '查看修改内容', savedAt: '保存时间',
  deleteIntent: '删除意图：收到现场确认前保留最后确认配置。', upsertIntent: '创建或修改意图：收到现场确认前不作为已提交配置。',
  runtime: '现场运行状态', noRuntime: '尚未收到运行结果；不能确认配置已生效。', desiredRevision: '期望运行版本', activeRevision: '最后可用运行版本',
  runtimeAt: '运行结果时间', compensation: '自动补偿', activeConfiguration: '查看最后可用运行配置', unconfirmed: '尚未确认', unknown: '未知状态',
  reedit: '按现场值重新编辑', reeditHint: '现场配置在本平台修改之后又发生过变更，本次修改未被采用。编辑表单会载入现场当前值，保存后即以此为基线重新提交。',
} : {
  title: 'Configuration commitment and runtime', refresh: 'Refresh', explanation: 'Confirmed configuration, platform intents and gateway runtime are shown separately. Commitment does not confirm runtime activation.',
  failed: 'Could not load configuration state. Please retry.', stale: 'Configuration status could not be refreshed. The last results are shown and may be out of date.', loading: 'Loading configuration state…', confirmed: 'Last gateway-confirmed configuration', noConfirmed: 'No gateway confirmation yet. Pending edits are not confirmed configuration.',
  revision: 'Committed revision', confirmedAt: 'Last confirmed', deleted: 'The gateway confirmed deletion of this resource.', configuration: 'View confirmed configuration',
  intents: 'Platform change intents', noIntents: 'No pending or failed change intents.', pendingConfiguration: 'View proposed changes', savedAt: 'Saved at',
  deleteIntent: 'Deletion intent: the last confirmed configuration remains until gateway confirmation.', upsertIntent: 'Create or edit intent: not committed until gateway confirmation.',
  runtime: 'Gateway runtime', noRuntime: 'No runtime result yet. Activation is not confirmed.', desiredRevision: 'Desired runtime revision', activeRevision: 'Last working runtime revision',
  runtimeAt: 'Runtime result time', compensation: 'Automatic compensation', activeConfiguration: 'View last working runtime configuration', unconfirmed: 'Not confirmed', unknown: 'Unknown state',
  reedit: 'Re-edit from gateway value', reeditHint: 'The gateway configuration changed after this platform edit, so this change was not applied. The form loads the current gateway value and resubmits from that baseline.',
});
const data = ref(null);
const loading = ref(false);
const error = ref(false);
const resource = computed(() => resourceConfiguration(data.value, props.entityType, props.entityCode, props.gatewayCode));
// The panel explains a difference between what the platform asked for and what
// the gateway did. While the two agree the whole section is noise for an
// operator — three groups of "nothing to report" — so it renders only while a
// change is still in flight, conflicted or failed.
const status = computed(() => configurationStatus(resource.value));
const visible = computed(() => error.value || (loading.value && !data.value) || configurationNeedsAttention(status.value));
let generation = 0;
let controller;
let timer;

function stateLabel(status) {
  const labels = {
    pending: ['已保存，待应用', 'Saved, pending apply'], queued: ['已保存，待应用', 'Saved, pending apply'], submitted: ['已发送，待提交', 'Sent, awaiting commitment'],
    committed: ['已提交', 'Committed'], applied: ['已提交', 'Committed'], active: ['运行已生效', 'Runtime active'], failed: ['应用失败', 'Apply failed'],
    rejected: ['已拒绝', 'Rejected'], conflict: ['现场配置已保留，修改未应用', 'Gateway configuration kept; edit not applied'],
    restored: ['已恢复最后可用配置', 'Last working configuration restored'], succeeded: ['补偿成功', 'Compensation succeeded'],
    retrying: ['正在自动重试', 'Retrying automatically'], none: ['无需补偿', 'No compensation needed'],
  };
  return labels[status]?.[isZh.value ? 0 : 1] || `${copy.value.unknown}: ${status}`;
}

// Failure copy is produced by the backend, which cannot know the console
// language. New writers emit a bilingual payload and legacy rows hold a single
// language, so both go through the shared resolver rather than being printed
// raw inside an otherwise localized panel.
function backendText(value) {
  return resolveServerText(value, activeLocale.value);
}

// Rebasing needs a confirmed value to rebase onto, so the offer only appears
// for resources the gateway has already committed. A conflicted deletion is
// re-issued by deleting again, not by editing fields, and a creation that never
// reached the gateway is re-submitted from the pending list instead.
function needsReedit(entry) {
  return resource.value.projections.length > 0
    && entry.kind !== 'delete'
    && ['conflict', 'failed', 'rejected'].includes(entry.status);
}

// The host loads the resource's current gateway-confirmed value into its edit
// form, so saving it again rebases the proposal onto the live revision.
function requestReedit(entry) {
  emit('reedit', {
    entityType: props.entityType,
    entityCode: props.entityCode,
    gatewayCode: entry.gateway_code || props.gatewayCode || '',
  });
}

async function load() {
  const current = ++generation;
  controller?.abort();
  if (!props.entityCode) { data.value = null; return; }
  controller = new AbortController();
  loading.value = true;
  error.value = false;
  try {
    const response = await axios.get('/api/extension/cascade/configuration/resources', {
      params: { entity_type: props.entityType, entity_code: props.entityCode, ...(props.gatewayCode ? { gateway_code: props.gatewayCode } : {}) },
      signal: controller.signal,
    });
    if (response.data?.code && response.data.code !== 200) throw new Error('configuration state unavailable');
    if (current === generation) data.value = response.data?.data || null;
  } catch {
    if (current === generation) error.value = true;
  } finally {
    if (current === generation) loading.value = false;
  }
}

watch(() => [props.entityType, props.entityCode, props.gatewayCode], () => {
  data.value = null;
  window.clearInterval(timer);
  load();
  timer = window.setInterval(load, 15000);
}, { immediate: true });
onUnmounted(() => { ++generation; controller?.abort(); window.clearInterval(timer); });
</script>

<style scoped>
.configuration-state-panel { display: flex; flex-direction: column; gap: 16px; color: var(--text-main); }
.configuration-state-panel__heading { display: flex; align-items: center; justify-content: space-between; gap: 16px; flex-wrap: wrap; }
.configuration-state-panel__group { display: flex; flex-direction: column; gap: 8px; }
.configuration-state-panel__group h6 { margin: 0; }
.configuration-state-panel__entry { padding: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-surface); display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.configuration-state-panel__metadata { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 8px 16px; margin: 0; font-size: 0.875rem; }
.configuration-state-panel__metadata dt { font-weight: normal; color: var(--text-secondary); }
.configuration-state-panel__metadata dd { margin: 0; overflow-wrap: anywhere; }
.configuration-state-panel summary { cursor: pointer; color: var(--text-main); }
.configuration-state-panel pre { max-height: 320px; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; padding: 16px; margin: 8px 0 0; background: var(--bg-elevated); color: var(--text-main); border-radius: var(--radius-control); }
.configuration-state-panel__reedit { align-self: flex-start; }
</style>
