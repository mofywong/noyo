<template>
  <span v-if="presentation" class="config-sync-status" :class="{ 'config-sync-status--compact': compact }">
    <component
      :is="interactive ? 'button' : 'span'"
      class="dash-pill"
      :class="[presentation.tone, { 'dash-pill--actionable': interactive }]"
      :type="interactive ? 'button' : null"
      :title="compact ? `${label}: ${hint}` : hint"
      :aria-label="compact ? `${label}: ${hint}` : hint"
      :tabindex="compact && !interactive ? 0 : null"
      @click="handleClick"
    >
      <span class="dash-pill-dot" aria-hidden="true"></span><span class="dash-pill-label">{{ label }}</span>
    </component>
    <span v-if="failureReason" class="config-sync-error">{{ failureReason }}</span>
  </span>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { useAuthStore } from '../../stores/auth.js';
import { configurationNeedsAttention } from '../../utils/configurationState.js';
import { resolveServerText } from '../../utils/serverText.js';

const props = defineProps({
  state: { type: String, default: '' },
  reason: { type: [String, Object], default: '' },
  compact: { type: Boolean, default: false },
});
const { locale } = useI18n();
const router = useRouter();
const authStore = useAuthStore();
const isZh = computed(() => String(locale.value).toLowerCase().startsWith('zh'));
const STATES = {
  pending: { tone: 'dash-pill--warning', zh: '已保存，待应用', en: 'Saved, pending apply', zhHint: '修改意图已保存，尚未收到现场提交确认', enHint: 'The intent is saved; gateway commitment has not been confirmed' },
  pending_sync: { tone: 'dash-pill--warning', zh: '待同步到平台', en: 'Pending sync to platform', zhHint: '本地配置已保存，平台尚未确认接收；平台连接恢复后会自动同步。此提示不表示本地配置尚未生效。', enHint: 'The configuration is saved locally; platform receipt is not yet confirmed. It will synchronize automatically when the platform connection recovers. This does not indicate pending local activation.' },
  pending_create: { tone: 'dash-pill--warning', zh: '已保存，待创建', en: 'Saved, pending creation', zhHint: '创建意图已保存，现场尚未确认创建资源', enHint: 'The creation intent is saved; the gateway has not confirmed creating the resource' },
  pending_delete: { tone: 'dash-pill--warning', zh: '已保存，待删除', en: 'Saved, pending deletion', zhHint: '删除意图已保存，现场仍保留最后确认配置', enHint: 'The deletion intent is saved; the last confirmed gateway configuration remains' },
  sync_conflict: { tone: 'dash-pill--danger', zh: '同步冲突', en: 'Sync conflict', zhHint: '两侧版本均已保留，系统未自动覆盖；请前往网关管理页决定保留哪一侧', enHint: 'Both versions were preserved and nothing was overwritten; decide which side to keep on the gateway management page' },
  conflict: { tone: 'dash-pill--danger', zh: '配置冲突', en: 'Config conflict', zhHint: '现场配置已保留，平台修改未应用；打开详情可查看现场值并重新编辑', enHint: 'Gateway configuration was kept; the platform edit was not applied. Open the detail panel to review the gateway value and edit again' },
  committed: { tone: 'dash-pill--neutral', zh: '已提交', en: 'Committed', zhHint: '配置已由现场确认，运行生效尚未确认', enHint: 'Gateway commitment is confirmed; runtime activation is not yet confirmed' },
  applying: { tone: 'dash-pill--warning', zh: '运行待生效', en: 'Runtime pending', zhHint: '配置已提交，运行进程正在应用或等待重试', enHint: 'Configuration is committed; the runtime is applying it or awaiting retry' },
  active: { tone: 'dash-pill--success', zh: '运行已生效', en: 'Runtime active', zhHint: '现场运行进程已确认加载当前配置', enHint: 'The gateway runtime confirmed loading the current configuration' },
  failed: { tone: 'dash-pill--danger', zh: '应用失败', en: 'Apply failed', zhHint: '配置提交或运行应用失败，请查看详情中的原因与最后可用版本', enHint: 'Configuration commitment or runtime apply failed; see details for the cause and last working version' },
};
// "committed" and "active" stay in the table for reference but are suppressed by
// the shared classifier: a settled resource renders no badge at all, so the
// lists are not littered with "已提交" on every healthy row.
const presentation = computed(() => {
  const state = String(props.state || '').toLowerCase();
  if (!configurationNeedsAttention(state)) return null;
  return STATES[state];
});
const label = computed(() => isZh.value ? presentation.value?.zh : presentation.value?.en);
const failureReason = computed(() => String(props.state).toLowerCase() === 'failed'
  ? resolveServerText(props.reason, locale.value) : '');
const hint = computed(() => failureReason.value ? `${label.value}: ${failureReason.value}`
  : isZh.value ? presentation.value?.zhHint : presentation.value?.enHint);
// A replication conflict is arbitrated on the gateway page. The gateway
// runtime has no such page, so the badge stays informational there.
const interactive = computed(() => String(props.state).toLowerCase() === 'sync_conflict'
  && authStore.hasPermission('gateway:list'));

function handleClick(event) {
  if (!interactive.value) return;
  // The badge sits inside clickable table rows; never let it open the row.
  event.stopPropagation();
  router.push({ name: 'GatewayManagement' });
}
</script>

<style scoped>
.config-sync-status {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  max-width: 100%;
}
.config-sync-error {
  color: var(--color-danger);
  font-size: var(--font-size-sm, 0.875rem);
  overflow-wrap: anywhere;
}
.config-sync-status--compact {
  width: 100%;
  min-width: 0;
  flex-wrap: nowrap;
}
.config-sync-status--compact .dash-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  max-width: 100%;
  min-width: 0;
}
.config-sync-status--compact .dash-pill-label {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.config-sync-status--compact .config-sync-error { display: none; }
.config-sync-status--compact .dash-pill:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}
.dash-pill--actionable {
  cursor: pointer;
  font-family: inherit;
  appearance: none;
  -webkit-appearance: none;
}
.dash-pill--actionable:hover {
  border-color: currentColor;
}
.dash-pill--actionable:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}
</style>
