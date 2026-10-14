<template>
  <div class="model-lib" role="table" :aria-label="copy.a11yLibraryTable" :aria-rowcount="rows.length + 1">
    <!-- 表头 / Table header -->
    <div class="model-lib__head" role="row">
      <span class="model-lib__th model-lib__th--pkg" role="columnheader">{{ copy.colPackage }}</span>
      <span class="model-lib__th model-lib__th--kind" role="columnheader">{{ copy.colKind }}</span>
      <span class="model-lib__th model-lib__th--version" role="columnheader">{{ copy.colVersion }}</span>
      <span class="model-lib__th model-lib__th--state" role="columnheader">{{ copy.colState }}</span>
      <span class="model-lib__th model-lib__th--used" role="columnheader">{{ copy.colUsedBy }}</span>
      <span class="model-lib__th model-lib__th--actions" role="columnheader">{{ copy.colActions }}</span>
    </div>

    <!-- 表体 / Table body -->
    <div class="model-lib__body">
      <div
        v-for="row in rows"
        :key="row.id"
        class="model-lib__row"
        role="row"
        :aria-label="rowLabel(row)"
      >
        <!-- 模型包 / Package -->
        <div class="model-lib__td model-lib__td--pkg" role="cell">
          <i :class="['bi', kindIcon(row.kind), 'model-lib__icon']" aria-hidden="true"></i>
          <div class="model-lib__name">
            <strong>{{ rowTitle(row) }}</strong>
            <code>{{ row.id }}</code>
          </div>
        </div>

        <!-- 类型 / Kind -->
        <div class="model-lib__td model-lib__td--kind" role="cell">
          <span class="model-lib__kind">{{ kindText(row.kind) }}</span>
        </div>

        <!-- 当前版本 / Current version -->
        <div class="model-lib__td model-lib__td--version" role="cell">
          <span class="model-lib__version">{{ row.activeRelease?.version || copy.noActiveVersion }}</span>
        </div>

        <!-- 状态 / Status -->
        <div class="model-lib__td model-lib__td--state" role="cell">
          <span class="model-lib__pill" :class="`model-lib__pill--${stateGroup(row.state)}`">
            {{ statusText(row) }}
          </span>
        </div>

        <!-- 服务的 AI 能力 / Served capabilities -->
        <div class="model-lib__td model-lib__td--used" role="cell">
          <span v-if="rowCapabilities(row).length" class="model-lib__caps">
            <span v-for="cap in rowCapabilities(row).slice(0, 3)" :key="cap.id" class="model-lib__chip">{{ cap.name }}</span>
            <span v-if="rowCapabilities(row).length > 3" class="model-lib__more">+{{ rowCapabilities(row).length - 3 }}</span>
          </span>
          <span v-else class="model-lib__sub">{{ copy.none }}</span>
        </div>

        <!-- 操作 / Actions -->
        <div class="model-lib__td model-lib__td--actions" role="cell">
          <button type="button" class="model-lib__action" :title="copy.viewVersions" :aria-label="copy.viewVersionsFor.replace('{name}', rowTitle(row))" @click="openVersions(row)">
            <i class="bi bi-layers" aria-hidden="true"></i>
          </button>
          <button type="button" class="model-lib__action" :title="copy.historyRecords" :aria-label="copy.viewHistoryFor.replace('{name}', rowTitle(row))" @click="openHistory(row)">
            <i class="bi bi-clock-history" aria-hidden="true"></i>
          </button>
        </div>
      </div>

      <div v-if="!rows.length" class="model-lib__empty">
        <p>{{ copy.libEmptyFiltered }}</p>
        <LiquidGlassButton variant="outline-primary" size="sm" @click="$emit('clearSearch')">{{ copy.clearSearch }}</LiquidGlassButton>
      </div>
    </div>
  </div>
</template>
<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import LiquidGlassButton from '../liquid-glass/LiquidGlassButton.vue';
import {
  modelCenterCopy,
  modelPackageCapabilities,
  modelText,
} from '../../utils/modelCenter.js';

const props = defineProps({
  rows: { type: Array, required: true },
  catalog: { type: Object, required: true },
  canManage: { type: Boolean, default: false },
});

const emit = defineEmits(['viewVersions', 'viewHistory', 'clearSearch']);

const { locale } = useI18n();
const copy = computed(() => modelCenterCopy(locale.value));

const rowTitle = row => modelText(row.title, locale.value, row.id);
const kindText = kind => copy.value.resourceKinds[kind] || kind || copy.value.unspecified;
const stateGroup = state => state === 'installed' ? 'available' : 'needsInstall';
const statusText = row => row.installed ? copy.value.installed : copy.value.notInstalled;
/** 读屏器播报「名称 + 状态」，复用能力卡的双语模板。 */
const rowLabel = row => copy.value.a11yCapabilityCard.replace('{name}', rowTitle(row)).replace('{status}', statusText(row));

const kindIcons = {
  general: 'bi-cpu', helmet: 'bi-shield-check', pose: 'bi-person-walking',
  smoking: 'bi-fire', chef_hat: 'bi-person-exclamation', chef_hat_hairnet: 'bi-person-badge',
  kitchen: 'bi-cup-hot', fighting: 'bi-people',
};
const kindIcon = kind => kindIcons[kind] || 'bi-box';

/** 行内展示的 AI 能力名称（直接支持优先，其次依赖）。 */
function rowCapabilities(row) {
  const caps = modelPackageCapabilities(row, props.catalog);
  const ids = [...new Set([...caps.direct, ...caps.dependency])];
  return ids.map(id => {
    const capability = props.catalog.capabilities.find(item => item.id === id);
    return { id, name: capability ? modelText(capability.title, locale.value, id) : id };
  });
}

const openVersions = row => emit('viewVersions', row);
const openHistory = row => emit('viewHistory', row);
</script>

<style scoped>
.model-lib { display: flex; flex-direction: column; min-width: 0; border: 1px solid var(--border-color); border-radius: var(--radius-card); background: var(--bg-surface); overflow: hidden; }

.model-lib__head,
.model-lib__row {
  display: grid;
  grid-template-columns: minmax(0, 2.4fr) minmax(0, 1.2fr) minmax(0, 1.1fr) minmax(0, 1.2fr) minmax(0, 2fr) minmax(0, 1.5fr);
  gap: 16px;
  align-items: center;
  padding: 16px 24px;
  min-width: 0;
}

.model-lib__head { border-bottom: 1px solid var(--border-color); background: var(--bg-elevated); }
.model-lib__th { color: var(--text-secondary); font-size: 0.75rem; font-weight: 600; letter-spacing: 0.04em; }
.model-lib__body { overflow-y: auto; flex: 1; min-height: 0; }
.model-lib__row { border-bottom: 1px solid var(--border-color); transition: background var(--noyo-duration-fast, 0.15s) var(--noyo-ease-standard, ease); }
.model-lib__row:last-child { border-bottom: 0; }
.model-lib__row:hover { background: var(--bg-hover); }

.model-lib__td { display: flex; flex-direction: column; gap: 8px; min-width: 0; }
.model-lib__td--pkg { flex-direction: row; align-items: flex-start; gap: 8px; }
.model-lib__td--actions { flex-direction: row; align-items: center; gap: 8px; justify-content: flex-end; }

.model-lib__icon { color: var(--color-brand); font-size: 1rem; line-height: 1.5; flex-shrink: 0; }
.model-lib__name { min-width: 0; }
.model-lib__name strong { display: block; color: var(--text-primary); font-size: 0.875rem; line-height: 1.4; overflow-wrap: anywhere; }
.model-lib__name code { display: block; color: var(--text-muted); font-family: var(--font-mono); font-size: 0.6875rem; overflow-wrap: anywhere; }

.model-lib__kind { color: var(--text-primary); font-size: 0.8125rem; overflow-wrap: anywhere; }
.model-lib__version { color: var(--text-primary); font-family: var(--font-mono); font-size: 0.8125rem; }
.model-lib__releases { color: var(--text-muted); font-size: 0.6875rem; }
.model-lib__sub { color: var(--text-muted); font-size: 0.6875rem; line-height: 1.4; }

.model-lib__pill { align-self: flex-start; padding: 0 8px; border-radius: var(--noyo-radius-pill, 999px); font-size: 0.75rem; white-space: nowrap; }
.model-lib__pill--available { color: var(--color-success); background: color-mix(in srgb, var(--color-success) 12%, var(--bg-surface)); }
.model-lib__pill--needsInstall { color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 12%, var(--bg-surface)); }
.model-lib__pill--unsupported { color: var(--text-muted); background: var(--bg-elevated); }

.model-lib__caps { display: flex; flex-wrap: wrap; gap: 8px; }
.model-lib__chip { padding: 0 8px; border-radius: var(--noyo-radius-pill, 999px); background: var(--bg-elevated); color: var(--text-secondary); font-size: 0.6875rem; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 128px; }
.model-lib__more { color: var(--text-muted); font-size: 0.6875rem; align-self: center; }

.model-lib__action { display: inline-flex; align-items: center; justify-content: center; width: 40px; height: 40px; border: 1px solid var(--border-color); border-radius: var(--radius-control); background: var(--bg-surface); color: var(--text-secondary); cursor: pointer; }
.model-lib__action:hover { background: var(--bg-hover); color: var(--text-primary); }
.model-lib__action:focus-visible { outline: 2px solid var(--color-brand); outline-offset: 8px; }

.model-lib__empty { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 16px; padding: 96px 24px; color: var(--text-secondary); text-align: center; }
.model-lib__empty p { margin: 0; }

@media (max-width: 1100px) {
  .model-lib__head { display: none; }
  .model-lib__row { grid-template-columns: minmax(0, 1fr); gap: 8px; }
  .model-lib__td--actions { justify-content: flex-start; }
}
</style>
