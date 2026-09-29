<template>
  <DetailDrawer :visible="!!gateway" :title="`${copy.title} · ${gateway?.name || gateway?.sn || ''}`" @close="$emit('close')">
    <section class="gateway-runtime-failures" aria-live="polite">
      <div class="gateway-runtime-failures__heading">
        <p class="text-secondary mb-0">{{ copy.explanation }}</p>
        <button type="button" class="btn btn-outline-secondary btn-sm" :disabled="loading" @click="load()">{{ copy.refresh }}</button>
      </div>
      <p v-if="error" class="text-danger mb-0" role="alert">{{ failures.length ? copy.stale : copy.failed }}</p>
      <p v-if="loading && !failures.length" class="text-secondary mb-0" role="status">{{ copy.loading }}</p>
      <p v-else-if="!error && !failures.length" class="gateway-runtime-failures__empty text-secondary mb-0">{{ copy.empty }}</p>
      <article v-for="entry in failures" :key="`${entry.entity_type}:${entry.entity_code}`" class="gateway-runtime-failures__entry">
        <h6 class="mb-0">{{ entityLabel(entry.entity_type) }} · <code>{{ entry.entity_code }}</code></h6>
        <p v-if="entry.deleted" class="text-danger mb-0">{{ copy.deleted }}</p>
        <p v-else-if="entry.has_projection === false" class="text-secondary mb-0">{{ copy.noProjection }}</p>
        <dl class="gateway-runtime-failures__metadata">
          <dt>{{ copy.reason }}</dt><dd class="text-danger">{{ backendText(entry.last_error) || copy.noReason }}</dd>
          <dt>{{ copy.desiredRevision }}</dt><dd>{{ entry.revision }}</dd>
          <dt>{{ copy.activeRevision }}</dt><dd>{{ entry.active_revision || copy.unconfirmed }}</dd>
          <dt>{{ copy.updatedAt }}</dt><dd>{{ formatDateTime(entry.updated_at) }}</dd>
        </dl>
        <button v-if="supportsDetails(entry)" type="button" class="btn btn-outline-primary btn-sm" @click="selectedFailure = entry">{{ copy.details }}</button>
        <button v-else-if="['plugin_config', 'node_config'].includes(entry.entity_type) && gateway?.online" v-permission="'gateway:config'" type="button" class="btn btn-outline-primary btn-sm" @click="$emit('configure', { gateway, entry })">{{ copy.openSettings }}</button>
      </article>
      <button v-if="hasMore" type="button" class="btn btn-outline-secondary" :disabled="loading" @click="load(page + 1)">{{ copy.more }}</button>
      <ConfigurationStatePanel v-if="selectedFailure" :entity-type="selectedFailure.entity_type" :entity-code="selectedFailure.entity_code" :gateway-code="gateway.sn" />
    </section>
  </DetailDrawer>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import DetailDrawer from '../DetailDrawer.vue';
import ConfigurationStatePanel from './ConfigurationStatePanel.vue';
import { resolveServerText } from '../../utils/serverText.js';
import { formatDateTime } from '../../utils/dateTime.js';

const props = defineProps({ gateway: { type: Object, default: null } });
defineEmits(['close', 'configure']);
const { locale } = useI18n();
const isZh = computed(() => String(locale.value).toLowerCase().startsWith('zh'));
const copy = computed(() => isZh.value ? {
  title: '未生效配置', explanation: '配置已保存，但网关尚未成功运行以下改动。请查看具体资源和失败原因；系统会自动重试。',
  refresh: '刷新', loading: '正在加载失败明细…', failed: '失败明细暂时无法加载，请刷新重试。', stale: '明细更新失败，以下为上次结果，请刷新重试。',
  empty: '当前没有未生效配置。状态可能刚刚更新，请刷新网关列表确认。', reason: '失败原因', noReason: '网关尚未提供失败原因。',
  desiredRevision: '目标版本', activeRevision: '当前运行版本', unconfirmed: '尚未确认', updatedAt: '更新时间',
  deleted: '删除尚未生效：配置已提交删除，但网关还没有完成移除。', noProjection: '尚无已确认配置记录，请根据资源代码和失败原因核对。',
  details: '查看配置详情', openSettings: '打开对应配置', more: '加载更多', unknownType: '其他配置',
} : {
  title: 'Configurations not active', explanation: 'These changes were saved, but the gateway could not activate them. Check the resource and error below. The system will retry automatically.',
  refresh: 'Refresh', loading: 'Loading failure details…', failed: 'Could not load failure details. Please refresh to retry.', stale: 'Could not update details. Showing the previous result; please refresh to retry.',
  empty: 'No configurations are currently failing. The status may have just changed; refresh the gateway list to confirm.', reason: 'Failure reason', noReason: 'The gateway has not provided an error yet.',
  desiredRevision: 'Target revision', activeRevision: 'Active revision', unconfirmed: 'Not confirmed', updatedAt: 'Updated at',
  deleted: 'Removal not active: deletion was submitted, but the gateway has not finished removing this resource.', noProjection: 'No confirmed configuration record is available. Check the resource code and error.',
  details: 'View configuration details', openSettings: 'Open configuration', more: 'Load more', unknownType: 'Other configuration',
});
const failures = ref([]);
const selectedFailure = ref(null);
const loading = ref(false);
const error = ref(false);
const page = ref(1);
const hasMore = ref(false);
let controller;
let generation = 0;

const entityLabel = (type) => {
  const labels = isZh.value
    ? { device: '设备', product: '产品', driver: '驱动', rule: '规则', rule_group: '规则组', plugin_config: '网关插件', node_config: '系统参数' }
    : { device: 'Device', product: 'Product', driver: 'Driver', rule: 'Rule', rule_group: 'Rule group', plugin_config: 'Gateway plugin', node_config: 'System settings' };
  return labels[type] || `${copy.value.unknownType} (${type})`;
};
const backendText = (text) => resolveServerText(text, locale.value);
const supportsDetails = (entry) => ['device', 'product', 'driver', 'rule', 'rule_group'].includes(entry.entity_type);

async function load(nextPage = 1) {
  if (!props.gateway?.sn) return;
  const current = ++generation;
  controller?.abort();
  controller = new AbortController();
  loading.value = true;
  error.value = false;
  try {
    const response = await axios.get(`/api/extension/cascade/gateways/${encodeURIComponent(props.gateway.sn)}/runtime-results`, {
      params: { status: 'failed', page: nextPage, page_size: 100 }, signal: controller.signal,
    });
    if (response.data?.code !== 0 || !Array.isArray(response.data.data)) throw new Error('runtime details unavailable');
    if (current !== generation) return;
    const rows = response.data.data.filter((entry) => entry.status === 'failed');
    failures.value = nextPage === 1 ? rows : [...failures.value, ...rows];
    page.value = nextPage;
    hasMore.value = response.data.has_more === true;
    if (selectedFailure.value) selectedFailure.value = failures.value.find((entry) => entry.entity_type === selectedFailure.value.entity_type && entry.entity_code === selectedFailure.value.entity_code) || null;
  } catch {
    if (current === generation) error.value = true;
  } finally {
    if (current === generation) loading.value = false;
  }
}

watch(() => props.gateway?.sn, () => {
  ++generation;
  controller?.abort();
  failures.value = [];
  selectedFailure.value = null;
  hasMore.value = false;
  error.value = false;
  load();
}, { immediate: true });
onUnmounted(() => { ++generation; controller?.abort(); });
</script>

<style scoped>
.gateway-runtime-failures { display: grid; gap: 16px; }
.gateway-runtime-failures__heading { display: flex; align-items: flex-start; gap: 16px; }
.gateway-runtime-failures__heading .btn { flex-shrink: 0; }
.gateway-runtime-failures__entry { display: grid; gap: 16px; padding: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); overflow-wrap: anywhere; }
.gateway-runtime-failures__entry code { color: var(--text-main); }
.gateway-runtime-failures__metadata { display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 8px 16px; margin: 0; font-size: .875rem; }
.gateway-runtime-failures__metadata dt { color: var(--text-secondary); font-weight: 500; }
.gateway-runtime-failures__metadata dd { margin: 0; color: var(--text-main); }
.gateway-runtime-failures__empty { padding: 48px 0; }
</style>
