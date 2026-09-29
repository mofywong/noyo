<template>
  <div v-if="gatewayMode && state" class="gateway-maintenance-status" aria-live="polite">
    <span class="gateway-maintenance-status__label" :class="gatewayMaintenanceClass(state.mode)" :title="state.mode === 'reconciling' ? copy.handoverHint : copy.hint">
      <i :class="state.mode === 'normal' ? 'bi bi-lock' : 'bi bi-tools'" aria-hidden="true"></i>
      {{ modeLabel }}
    </span>
    <button
      v-if="state.mode !== 'reconciling'"
      v-permission="'gateway:config'"
      type="button"
      class="gateway-maintenance-status__action"
      :disabled="changing"
      @click="changeMode"
    >
      {{ changing ? copy.working : (state.mode === 'maintenance' ? copy.exit : copy.enter) }}
    </button>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import { useToast } from '../../composables/useToast';
import { useConfirm } from '../../composables/useConfirm';
import { gatewayMaintenanceClass } from '../../utils/gatewayMaintenance.js';

const props = defineProps({ mode: { type: String, default: '' } });
const { locale } = useI18n();
const { showToast } = useToast();
const { confirmDialog } = useConfirm();
const gatewayMode = computed(() => props.mode === 'gateway');
const state = ref(null);
const changing = ref(false);
const copy = computed(() => String(locale.value).toLowerCase().startsWith('zh')
  ? {
      normal: '正常模式', maintenance: '本地维护中', reconciling: '正在交接',
      enter: '进入维护', exit: '结束维护', working: '处理中…',
      hint: '只有在页面新增、修改或删除产品、驱动和设备时需要维护模式；维护期间平台对这些资源的修改暂停。',
      handoverHint: '正在同步本地改动，交接期间产品、驱动和设备暂时只读。系统会自动完成交接并恢复正常状态，无需重复操作。',
      enterTitle: '准备进入维护模式',
      enterMessage: '进入后，你可以在网关页面新增、修改或删除产品、驱动和设备，平台对这些资源的修改会暂停。请完成并保存改动后点击「结束维护」，系统会把改动同步给平台。',
      exitTitle: '准备结束维护模式',
      exitMessage: '请先保存本地改动。结束后，产品、驱动和设备将恢复本地只读，系统会把已保存的改动同步给平台。请等待「正在交接」结束；确认和重试由系统自动完成，无需重复操作。',
      exitConfirm: '结束维护并同步', enterCancel: '暂不进入', exitCancel: '继续维护',
      entered: '已进入维护模式。请在页面完成并保存产品、驱动或设备的改动，完成后点击「结束维护」。',
      failed: '维护状态操作失败',
    }
  : {
      normal: 'Normal mode', maintenance: 'Local maintenance', reconciling: 'Reconciling',
      enter: 'Start maintenance', exit: 'Finish maintenance', working: 'Working…',
      hint: 'Maintenance mode is required only to create, edit or delete products, drivers and devices on the page. Platform edits to these resources pause during maintenance.',
      handoverHint: 'Syncing your local changes. Products, drivers and devices are read-only during handover. The system will finish the handover and return to normal automatically; no further action is needed.',
      enterTitle: 'Start maintenance mode',
      enterMessage: 'You will be able to create, edit or delete products, drivers and devices on the gateway page. Platform edits to these resources will pause. Save your changes, then click “Finish maintenance” to sync them to the platform.',
      exitTitle: 'Finish maintenance mode',
      exitMessage: 'Please save your local changes first. Products, drivers and devices will become locally read-only, and your saved changes will sync to the platform. Wait for “Reconciling” to finish. The system handles confirmation and retries automatically; no further action is needed.',
      exitConfirm: 'Finish and sync', enterCancel: 'Not now', exitCancel: 'Keep maintaining',
      entered: 'Maintenance mode is on. Make and save your product, driver or device changes on the page, then click “Finish maintenance”.',
      failed: 'Could not change maintenance mode',
    });
const modeLabel = computed(() => copy.value[state.value?.mode] || copy.value.normal);
let pollTimer;

async function fetchState() {
  if (!gatewayMode.value) { state.value = null; return; }
  try {
    const response = await axios.get('/api/extension/cascade/maintenance');
    if (response.data?.code === 0) state.value = response.data.data;
  } catch (_) {
    // The cascade plugin may not have initialized yet; the next poll retries.
  }
}

async function changeMode() {
  if (!gatewayMode.value || !['normal', 'maintenance'].includes(state.value?.mode) || changing.value) return;
  const mode = state.value.mode;
  const action = mode === 'maintenance' ? 'exit' : 'enter';
  changing.value = true;
  try {
    const confirmed = await confirmDialog({
      title: action === 'exit' ? copy.value.exitTitle : copy.value.enterTitle,
      message: action === 'exit' ? copy.value.exitMessage : copy.value.enterMessage,
      confirmText: action === 'exit' ? copy.value.exitConfirm : copy.value.enter,
      cancelText: action === 'exit' ? copy.value.exitCancel : copy.value.enterCancel,
      variant: 'neutral',
      icon: action === 'exit' ? 'bi-arrow-repeat' : 'bi-tools',
    });
    if (!confirmed || !gatewayMode.value || state.value?.mode !== mode) return;
    const response = await axios.post('/api/extension/cascade/maintenance', { action });
    if (response.data?.code !== 0) throw new Error(response.data?.message || copy.value.failed);
    state.value = response.data.data;
    if (state.value.mode === 'reconciling') showToast('success', copy.value.handoverHint);
    else if (action === 'enter' && state.value.mode === 'maintenance') showToast('success', copy.value.entered);
  } catch (error) {
    showToast('danger', error.response?.data?.message || error.message || copy.value.failed);
  } finally {
    changing.value = false;
  }
}

watch(gatewayMode, fetchState);
onMounted(() => { fetchState(); pollTimer = setInterval(fetchState, 5000); });
onUnmounted(() => clearInterval(pollTimer));
</script>

<style scoped>
.gateway-maintenance-status { display: inline-flex; align-items: center; gap: 8px; min-height: 32px; }
.gateway-maintenance-status__label { display: inline-flex; align-items: center; gap: 8px; padding: 4px 8px; border-radius: var(--noyo-radius-pill, 999px); color: var(--text-main); background: var(--bg-surface); border: 1px solid var(--border-color); font-size: 12px; white-space: nowrap; }
.gateway-maintenance-status__label.gateway-state--maintenance,
.gateway-maintenance-status__label.gateway-state--reconciling {
  color: var(--gateway-state-color);
  background: color-mix(in srgb, var(--gateway-state-color) 12%, var(--bg-surface));
  border-color: color-mix(in srgb, var(--gateway-state-color) 40%, var(--border-color));
  box-shadow: 0 0 12px color-mix(in srgb, var(--gateway-state-color) 16%, transparent);
}
.gateway-maintenance-status__action { min-height: 32px; padding: 4px 8px; border-radius: var(--noyo-radius-pill, 999px); color: var(--text-main); background: var(--bg-surface); border: 1px solid var(--border-color); font-size: 12px; cursor: pointer; }
.gateway-maintenance-status__action:hover { background: var(--bg-body); }
.gateway-maintenance-status__action:focus-visible { outline: 2px solid var(--bs-primary); outline-offset: 2px; }
.gateway-maintenance-status__action:disabled { opacity: .6; cursor: wait; }
</style>
