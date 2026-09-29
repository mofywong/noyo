<template>
  <!-- Rendered only when there is something to arbitrate, so the gateway page is
       visually unchanged on a healthy deployment. -->
  <div v-if="total > 0" class="card border-0 shadow-sm mb-3 sync-conflict-panel">
    <div class="card-header bg-warning-subtle d-flex flex-wrap align-items-center justify-content-between gap-2">
      <div class="d-flex align-items-center gap-2">
        <i class="bi bi-exclamation-triangle-fill text-warning-emphasis"></i>
        <span class="fw-bold text-warning-emphasis">{{ L.title }}</span>
        <span class="badge text-bg-warning">{{ total }}</span>
      </div>
      <div class="d-flex flex-wrap align-items-center gap-2">
        <button type="button" class="btn btn-sm btn-outline-secondary" :disabled="busy" @click="load">
          <i class="bi bi-arrow-clockwise me-1"></i>{{ L.refresh }}
        </button>
        <button type="button" class="btn btn-sm btn-outline-primary" :disabled="busy" @click="resolveAll('local')">
          {{ L.keepLocalAll }}
        </button>
        <button type="button" class="btn btn-sm btn-outline-primary" :disabled="busy" @click="resolveAll('peer')">
          {{ L.takePeerAll }}
        </button>
      </div>
    </div>

    <div class="card-body">
      <p class="text-body-secondary small mb-3">{{ L.hint }}</p>

      <div v-if="error" class="alert alert-danger py-2 mb-3">{{ error }}</div>

      <div class="table-responsive">
        <table class="table table-sm align-middle mb-0">
          <thead>
            <tr class="text-body-secondary small">
              <th scope="col">{{ L.colEntity }}</th>
              <th scope="col">{{ L.colLocal }}</th>
              <th scope="col">{{ L.colPeer }}</th>
              <th scope="col" class="text-end">{{ L.colDecision }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in items" :key="item.entity_type + ':' + item.entity_code">
              <td>
                <div class="fw-semibold">{{ item.name || item.entity_code }}</div>
                <div class="text-body-secondary small font-monospace">{{ item.entity_code }}</div>
                <div class="text-body-tertiary small">{{ entityLabel(item) }}</div>
              </td>
              <td class="small">{{ localSummary(item) }}</td>
              <td class="small">
                <span v-if="item.peer_delete" class="text-danger-emphasis">{{ L.peerDeleted }}</span>
                <span v-else>{{ peerSummary(item) }}</span>
                <div class="text-body-tertiary small">
                  {{ L.fromPeer }}:
                  <span v-if="peerNodeLabel(item)" :title="item.peer_node">{{ peerNodeLabel(item) }}</span>
                  <span v-else class="font-monospace">{{ item.peer_node || '-' }}</span>
                  <span v-if="item.peer_rev"> · rev {{ item.peer_rev }}</span>
                </div>
              </td>
              <td class="text-end">
                <div class="btn-group btn-group-sm" role="group">
                  <button
                    type="button"
                    class="btn btn-outline-secondary"
                    :disabled="busy"
                    @click="resolveOne(item, 'local')"
                  >
                    {{ L.keepLocal }}
                  </button>
                  <button
                    type="button"
                    class="btn btn-outline-primary"
                    :disabled="busy"
                    @click="resolveOne(item, 'peer')"
                  >
                    {{ L.takePeer }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import { useConfirm } from '../../composables/useConfirm';
import { useToast } from '../../composables/useToast';

// Copy switches with the console language instead of printing both languages on
// one line ("同步冲突 (Sync Conflicts)", "启用 (enabled)"), which made the table
// read as mixed-language in either locale. The sibling configuration panel uses
// the same locale-driven shape, so both stay recognisable next to each other.
// The `name` column renders stored business data and is intentionally untouched.
const { locale } = useI18n();
const isZh = computed(() => String(locale.value || '').toLowerCase().startsWith('zh'));

const L = computed(() => (isZh.value ? {
  title: '同步冲突',
  refresh: '刷新',
  keepLocalAll: '全部保留本机',
  takePeerAll: '全部采用对端',
  hint: '两侧版本均已完整保留，系统未做任何自动覆盖。请选择需要保留的一侧，决定会立即同步给对端。',
  colEntity: '对象',
  colLocal: '本机版本',
  colPeer: '对端版本',
  colDecision: '处理',
  keepLocal: '保留本机',
  takePeer: '采用对端',
  peerDeleted: '对端已删除',
  fromPeer: '来自',
  platformPeer: '平台',
  deviceType: '设备',
  productType: '产品',
  enabled: '启用',
  disabled: '停用',
  confirmBulkLocalTitle: '确认保留本机版本',
  confirmBulkPeerTitle: '确认采用对端版本',
  confirmBulkLocalMessage: '本机版本将被保留，对端对应记录会被覆盖或移除。',
  confirmBulkPeerMessage: '对端版本将覆盖本机，其中包含的删除决定会移除本机设备。',
  confirmDeleteTitle: '确认删除设备',
  confirmDeleteMessage: '对端已删除该设备，采用后将同时删除本机记录。',
  confirmText: '确认',
  cancelText: '取消',
  resolved: '冲突已处理',
  refreshFailed: '读取冲突列表失败',
  resolveFailed: '处理冲突失败'
} : {
  title: 'Sync conflicts',
  refresh: 'Refresh',
  keepLocalAll: 'Keep local for all',
  takePeerAll: 'Take peer for all',
  hint: 'Both versions are preserved and nothing was overwritten automatically. Choose the side to keep; the decision is propagated to the peer immediately.',
  colEntity: 'Entity',
  colLocal: 'Local',
  colPeer: 'Peer',
  colDecision: 'Decision',
  keepLocal: 'Keep local',
  takePeer: 'Take peer',
  peerDeleted: 'Peer deleted',
  fromPeer: 'from',
  platformPeer: 'Platform',
  deviceType: 'Device',
  productType: 'Product',
  enabled: 'enabled',
  disabled: 'disabled',
  confirmBulkLocalTitle: 'Confirm keeping local',
  confirmBulkPeerTitle: 'Confirm taking the peer version',
  confirmBulkLocalMessage: 'The local version is kept; the matching peer record is overwritten or removed.',
  confirmBulkPeerMessage: 'The peer version overwrites the local one; deletion decisions it carries remove local devices.',
  confirmDeleteTitle: 'Confirm device deletion',
  confirmDeleteMessage: 'The peer deleted this device. Taking the peer version also deletes the local record.',
  confirmText: 'Confirm',
  cancelText: 'Cancel',
  resolved: 'Conflict resolved',
  refreshFailed: 'Failed to load conflicts',
  resolveFailed: 'Failed to resolve conflict'
}));

const items = ref([]);
const total = ref(0);
const busy = ref(false);
const error = ref('');
const { confirmDialog } = useConfirm();
const { showToast } = useToast();

// A conflict record identifies the peer by sync node id, which is the gateway
// serial number. Operators recognise a gateway by its name, so the host passes
// the gateway list and the raw id degrades to a tooltip for support.
const props = defineProps({
  gateways: { type: Array, default: () => [] },
});

const gatewayNamesByNode = computed(() => {
  const names = new Map();
  for (const gateway of props.gateways || []) {
    const name = String(gateway?.name || '').trim();
    if (!name) continue;
    for (const identifier of [gateway?.serialNumber, gateway?.sn]) {
      const node = String(identifier || '').trim();
      if (node) names.set(node, name);
    }
  }
  return names;
});

// The platform itself is a sync peer too and must not surface as the literal
// node id "cloud" next to real gateway names.
const peerNodeLabel = (item) => {
  const node = String(item?.peer_node || '').trim();
  if (!node) return '';
  if (node === 'cloud') return L.value.platformPeer;
  return gatewayNamesByNode.value.get(node) || '';
};

const endpoint = '/api/extension/cascade/sync/conflicts';
const resolveEndpoint = `${endpoint}/resolve`;

const load = async () => {
  busy.value = true;
  error.value = '';
  try {
    const res = await axios.get(endpoint);
    const data = res.data?.data || {};
    items.value = data.items || [];
    total.value = Number(data.total || items.value.length);
  } catch (err) {
    // A missing endpoint means the plugin is not running in this deployment;
    // the panel stays hidden rather than surfacing a permanent error banner.
    items.value = [];
    total.value = 0;
    error.value = err?.response?.status === 404 ? '' : L.value.refreshFailed;
  } finally {
    busy.value = false;
  }
};

const entityLabel = (item) => (item.entity_type === 'product' ? L.value.productType : L.value.deviceType);

const localSummary = (item) => {
  const local = item.local || item.local_product;
  if (!local) {
    return '-';
  }
  const parts = [];
  if (local.name) {
    parts.push(local.name);
  }
  if (item.entity_type === 'device') {
    parts.push(local.enabled ? L.value.enabled : L.value.disabled);
  }
  return parts.join(' · ') || '-';
};

const peerSummary = (item) => {
  const peer = item.peer || item.peer_product;
  if (!peer) {
    return '-';
  }
  const parts = [];
  if (peer.name) {
    parts.push(peer.name);
  }
  if (item.entity_type === 'device') {
    parts.push(peer.enabled ? L.value.enabled : L.value.disabled);
  }
  return parts.join(' · ') || '-';
};

const peerDeleteCount = computed(() => items.value.filter((item) => item.peer_delete).length);

const resolveOne = async (item, resolution) => {
  if (resolution === 'peer' && item.peer_delete) {
    const confirmed = await confirmDialog({
      title: L.value.confirmDeleteTitle,
      message: L.value.confirmDeleteMessage,
      confirmText: L.value.confirmText,
      cancelText: L.value.cancelText
    });
    if (!confirmed) {
      return;
    }
  }
  await submit([{ entity_type: item.entity_type, entity_code: item.entity_code, resolution }]);
};

const resolveAll = async (resolution) => {
  const confirmed = await confirmDialog({
    title: resolution === 'local' ? L.value.confirmBulkLocalTitle : L.value.confirmBulkPeerTitle,
    message: resolution === 'local' ? L.value.confirmBulkLocalMessage : L.value.confirmBulkPeerMessage,
    confirmText: L.value.confirmText,
    cancelText: L.value.cancelText
  });
  if (!confirmed) {
    return;
  }
  await submit(items.value.map((item) => ({
    entity_type: item.entity_type,
    entity_code: item.entity_code,
    resolution
  })));
};

const submit = async (payload) => {
  if (!payload.length) {
    return;
  }
  busy.value = true;
  error.value = '';
  try {
    const res = await axios.post(resolveEndpoint, { items: payload });
    const data = res.data?.data || {};
    if (Number(data.failed || 0) > 0) {
      const firstFailure = (data.results || []).find((entry) => entry.ok === false);
      error.value = firstFailure?.message || L.value.resolveFailed;
    } else {
      showToast('success', L.value.resolved);
    }
    await load();
  } catch (err) {
    error.value = err?.response?.data?.message || L.value.resolveFailed;
  } finally {
    busy.value = false;
  }
};

onMounted(load);

defineExpose({ load, peerDeleteCount });
</script>

<style scoped>
/* Spacing follows the 8pt grid via Bootstrap's spacing scale; the surface itself
   inherits theme tokens so both light and dark themes are covered. */
.sync-conflict-panel .table > :not(caption) > * > * {
  padding-top: 0.5rem;
  padding-bottom: 0.5rem;
}
</style>
