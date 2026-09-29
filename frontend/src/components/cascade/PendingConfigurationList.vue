<template>
  <section v-if="intents.length" class="pending-configuration-list" aria-live="polite">
    <h6 class="mb-0">{{ isZh ? '待创建配置' : 'Configuration awaiting creation' }}</h6>
    <p class="small text-secondary mb-0">{{ isZh ? '以下创建意图已保存，尚未成为现场确认资源。' : 'These creation intents are saved and have not become gateway-confirmed resources.' }}</p>
    <button v-for="intent in intents" :key="intent.operation_code" type="button" class="pending-configuration-list__item" @click="selected = intent">
      <span>{{ intentName(intent) }} <span class="font-mono">({{ intent.entity_code }})</span></span>
      <ConfigSyncBadge :state="intent.status === 'pending' ? 'pending_create' : intent.status === 'rejected' ? 'failed' : intent.status" :reason="intent.last_error" />
    </button>
    <DetailDrawer :visible="!!selected" :title="selected ? intentName(selected) : ''" @close="selected = null">
      <ConfigurationStatePanel v-if="selected" :entity-type="entityType" :entity-code="selected.entity_code" :gateway-code="selected.gateway_code || ''" />
    </DetailDrawer>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import ConfigSyncBadge from './ConfigSyncBadge.vue';
import ConfigurationStatePanel from './ConfigurationStatePanel.vue';
import DetailDrawer from '../DetailDrawer.vue';

defineProps({ entityType: { type: String, required: true }, intents: { type: Array, default: () => [] } });
const { locale } = useI18n();
const isZh = computed(() => String(locale.value).toLowerCase().startsWith('zh'));
const selected = ref(null);
function intentName(intent) {
  try {
    const payload = typeof intent.payload === 'string' ? JSON.parse(intent.payload) : intent.payload;
    return payload?.name || payload?.device?.name || payload?.product?.name || intent.entity_code;
  } catch { return intent.entity_code; }
}
</script>

<style scoped>
.pending-configuration-list { display: flex; flex-direction: column; flex-shrink: 0; gap: 8px; padding: 16px; margin-bottom: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-surface); color: var(--text-main); max-height: 240px; overflow: auto; }
.pending-configuration-list__item { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px; padding: 8px 16px; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-elevated); color: var(--text-main); text-align: start; }
.pending-configuration-list__item:hover { background: var(--color-brand-subtle); }
.pending-configuration-list__item:focus-visible { outline: 2px solid var(--color-brand); outline-offset: 2px; }
</style>
