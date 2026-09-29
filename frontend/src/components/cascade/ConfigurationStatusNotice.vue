<template>
  <div v-if="failed" class="configuration-status-notice" role="alert">
    <span>{{ cached ? copy.stale : copy.unavailable }}</span>
    <button type="button" class="btn btn-outline-secondary btn-sm" @click="$emit('refresh')">{{ copy.refresh }}</button>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

defineProps({ failed: Boolean, cached: Boolean });
defineEmits(['refresh']);
const { locale } = useI18n();
const copy = computed(() => String(locale.value).toLowerCase().startsWith('zh') ? {
  stale: '配置状态更新失败，正在显示上次结果，可能已过期。',
  unavailable: '配置状态暂不可用，无法确认当前应用结果。',
  refresh: '重试',
} : {
  stale: 'Configuration status could not be refreshed. The last results are shown and may be out of date.',
  unavailable: 'Configuration status is unavailable. The current apply result cannot be confirmed.',
  refresh: 'Retry',
});
</script>

<style scoped>
.configuration-status-notice { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 8px 16px; margin-bottom: 16px; color: var(--color-warning); background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: var(--radius-control); }
</style>
