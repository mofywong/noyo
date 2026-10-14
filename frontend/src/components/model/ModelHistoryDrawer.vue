<template>
  <DetailDrawer :visible="visible" :title="drawerTitle" @close="$emit('close')">
    <template #header-badge><i class="bi bi-clock-history model-history__badge" aria-hidden="true"></i></template>
    <p class="model-history__total">{{ copy.historyTotal.replace('{count}', total) }}</p>
    <p v-if="loading" class="model-history__empty" role="status">{{ copy.historyLoading }}</p>
    <div v-else-if="error" class="model-history__error" role="alert">
      <p>{{ error }}</p><LiquidGlassButton variant="outline-secondary" size="sm" icon="bi bi-arrow-clockwise" @click="$emit('retry')">{{ copy.retry }}</LiquidGlassButton>
    </div>
    <ul v-else-if="items.length" class="model-history__list">
      <li v-for="item in items" :key="item.id" class="model-history__item">
        <div class="model-history__head"><strong>{{ actionText(item.action) }}</strong><time :datetime="item.createdAt">{{ modelTimestamp(item.createdAt, locale) }}</time></div>
        <dl class="model-history__meta">
          <dt>{{ copy.versionNumber }}</dt><dd>{{ item.version || copy.unspecified }}</dd>
          <template v-if="item.previousVersion"><dt>{{ copy.historyPreviousVersion }}</dt><dd>{{ item.previousVersion }}</dd></template>
          <dt>{{ copy.historyOperator }}</dt><dd>{{ item.operatorName || copy.systemOperator }}</dd>
        </dl>
      </li>
    </ul>
    <p v-else class="model-history__empty">{{ copy.historyEmpty }}</p>
    <template #footer>
      <div class="model-history__pagination">
        <LiquidGlassButton variant="outline-secondary" size="sm" icon="bi bi-chevron-left" :disabled="loading || page <= 1" @click="changePage(page - 1)">{{ copy.historyPrevious }}</LiquidGlassButton>
        <span>{{ copy.historyPage.replace('{page}', page).replace('{pages}', pages) }}</span>
        <LiquidGlassButton variant="outline-secondary" size="sm" icon="bi bi-chevron-right" :disabled="loading || page >= pages" @click="changePage(page + 1)">{{ copy.historyNext }}</LiquidGlassButton>
      </div>
    </template>
  </DetailDrawer>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import DetailDrawer from '../DetailDrawer.vue';
import LiquidGlassButton from '../liquid-glass/LiquidGlassButton.vue';
import { modelCenterCopy, modelText, modelTimestamp } from '../../utils/modelCenter.js';

const props = defineProps({
  visible: { type: Boolean, default: false },
  row: { type: Object, default: null },
  items: { type: Array, default: () => [] },
  total: { type: Number, default: 0 },
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 50 },
  loading: { type: Boolean, default: false },
  error: { type: String, default: '' },
});
const emit = defineEmits(['close', 'retry', 'page']);
const { locale } = useI18n();
const copy = computed(() => modelCenterCopy(locale.value));
const drawerTitle = computed(() => `${modelText(props.row?.title, locale.value, props.row?.id || '')} · ${copy.value.historyRecords}`);
const pages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)));
const actionText = action => ({ add: copy.value.historyAdded, delete: copy.value.historyDeleted, switch: copy.value.historySwitched }[action] || copy.value.unspecified);
function changePage(page) {
  if (!props.loading && page >= 1 && page <= pages.value && page !== props.page) emit('page', page);
}
</script>

<style scoped>
.model-history__badge { color: var(--color-brand); }
.model-history__total { margin: 0 0 16px; color: var(--text-secondary); font-size: 14px; }
.model-history__list { display: grid; gap: 16px; margin: 0; padding: 0; list-style: none; }
.model-history__item { padding: 24px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); }
.model-history__head { display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; gap: 8px; margin-bottom: 16px; color: var(--text-primary); }
.model-history__head time { color: var(--text-secondary); font-size: 12px; }
.model-history__meta { display: grid; grid-template-columns: minmax(96px, 1fr) minmax(0, 3fr); gap: 8px 16px; margin: 0; font-size: 14px; }
.model-history__meta dt { color: var(--text-secondary); }
.model-history__meta dd { margin: 0; color: var(--text-primary); overflow-wrap: anywhere; }
.model-history__empty { padding: 96px 24px; margin: 0; color: var(--text-secondary); text-align: center; }
.model-history__error { padding: 24px; border-radius: var(--radius-control); background: color-mix(in srgb, var(--color-danger) 8%, var(--bg-surface)); color: var(--color-danger); }
.model-history__error p { margin: 0 0 16px; }
.model-history__pagination { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px; }
.model-history__pagination span { color: var(--text-secondary); font-size: 14px; }
@media (max-width: 480px) { .model-history__meta { grid-template-columns: 1fr; } }
</style>
