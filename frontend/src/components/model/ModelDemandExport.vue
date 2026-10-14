<template>
  <section class="model-demand" :aria-label="copy.demandTitle">
    <div class="model-demand__bar">
      <div>
        <strong>{{ copy.demandCount.replace('{count}', selected.length) }}</strong>
        <p>{{ copy.demandHelp }}</p>
      </div>
      <div class="model-demand__actions">
        <LiquidGlassButton size="sm" variant="outline-secondary" :disabled="!visibleCapabilities.length" @click="$emit('update:selected', [...new Set([...selected, ...visibleCapabilities.map(c => c.id)])])">{{ copy.selectVisible }}</LiquidGlassButton>
        <LiquidGlassButton size="sm" variant="outline-secondary" :disabled="!selected.length" @click="$emit('update:selected', [])">{{ copy.clearSelection }}</LiquidGlassButton>
        <LiquidGlassButton size="sm" :disabled="!selected.length || !demand" @click="exportJSON">{{ copy.exportDemandJSON }}</LiquidGlassButton>
        <LiquidGlassButton size="sm" variant="outline-secondary" :disabled="!selected.length || !demand" @click="exportCSV">{{ copy.exportDemandCSV }}</LiquidGlassButton>
        <LiquidGlassButton v-if="catalog.canManage" size="sm" variant="outline-secondary" :loading="exporting" @click="exportCatalog">{{ copy.exportDeliveryCatalog }}</LiquidGlassButton>
      </div>
    </div>
    <p v-if="error" role="alert">{{ modelErrorText(error, locale) }}</p>
    <details v-if="selected.length && demand">
      <summary>{{ copy.demandPreview.replace('{count}', demand.packageIds.length) }}</summary>
      <ul>
        <li v-for="row in demand.rows" :key="row.capability.id">
          {{ modelText(row.capability.title, locale, row.capability.id) }}
          <span v-if="row.capability.state === 'unsupported'"> · {{ copy.buildUnsupported }}</span>
          <code>{{ row.packages.join(' · ') }}</code>
        </li>
      </ul>
      <fieldset v-if="demand.optionalIds.length">
        <legend>{{ copy.optionalDemand }}</legend>
        <label v-for="id in demand.optionalIds" :key="id">
          <input v-model="enhancements" type="checkbox" :value="id">
          {{ modelText(catalog.packages.find(p => p.id === id)?.title, locale, id) }}
        </label>
      </fieldset>
    </details>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import axios from 'axios';
import LiquidGlassButton from '../liquid-glass/LiquidGlassButton.vue';
import { modelCenterCopy, modelErrorText, modelText, unwrapModelResponse } from '../../utils/modelCenter.js';
import { downloadModelFile, modelDemand, modelDemandCSV } from '../../utils/modelDemand.js';

const props = defineProps({
  catalog: { type: Object, required: true },
  selected: { type: Array, required: true },
  visibleCapabilities: { type: Array, required: true },
});
defineEmits(['update:selected']);
const { locale } = useI18n();
const copy = computed(() => modelCenterCopy(locale.value));
const enhancements = ref([]);
const error = ref(null);
const exporting = ref(false);
const demand = computed(() => {
  try { return modelDemand(props.catalog, props.selected, enhancements.value); }
  catch { return null; }
});
function exportJSON() { downloadModelFile('model-requirements.json', JSON.stringify(demand.value.request, null, 2)); }
function exportCSV() { downloadModelFile('model-requirements.csv', modelDemandCSV(demand.value), 'text/csv;charset=utf-8'); }
async function exportCatalog() {
  if (exporting.value || !props.catalog.canManage) return;
  exporting.value = true;
  error.value = null;
  try {
    const catalog = unwrapModelResponse(await axios.get('/api/ai/models/delivery-catalog'));
    downloadModelFile('model-release-catalog.json', JSON.stringify(catalog, null, 2));
  } catch (failure) { error.value = failure; }
  finally { exporting.value = false; }
}
</script>

<style scoped>
.model-demand { margin: 16px 0; padding: 16px; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); }
.model-demand__bar, .model-demand__actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; }
.model-demand__bar { justify-content: space-between; gap: 16px; }
.model-demand p { margin: 8px 0 0; color: var(--text-secondary); }
.model-demand details { margin-top: 16px; }
.model-demand summary { cursor: pointer; }
.model-demand ul { margin: 16px 0; padding-left: 24px; }
.model-demand li { margin: 8px 0; }
.model-demand code { display: block; overflow-wrap: anywhere; color: var(--text-secondary); }
.model-demand fieldset { display: flex; flex-wrap: wrap; gap: 16px; }
.model-demand legend { font-size: inherit; margin-bottom: 8px; }
.model-demand label { display: flex; align-items: center; gap: 8px; }
</style>
