<template>
  <div class="marketplace-page page-fixed-height">
    <!-- Page Header -->
    <div class="page-header list-page-header">
      <div>
        <h1>{{ $t('page_marketplace') }}</h1>
        <p class="page-subtitle">{{ $t('market_subtitle') }}</p>
      </div>
    </div>

    <!-- Toolbar（§7.2：36px 控件） -->
    <div class="page-toolbar device-list-control-bar list-page-controls">
      <div class="input-group list-toolbar-query" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input
          v-model="search"
          type="search"
          class="form-control"
          :placeholder="$t('market_search_placeholder')"
          :aria-label="$t('market_search_placeholder')"
        >
      </div>
      <div class="btn-group list-toolbar-filter-group" role="group" :aria-label="$t('market_filter_all')">
        <button
          v-for="opt in categoryOptions"
          :key="opt.value"
          type="button"
          class="btn btn-sm"
          :class="categoryFilter === opt.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="categoryFilter = opt.value"
        >
          {{ $t(opt.labelKey) }}
        </button>
      </div>
      <div class="btn-group list-toolbar-filter-group" role="group" :aria-label="$t('status_running')">
        <button
          v-for="opt in statusOptions"
          :key="opt.value"
          type="button"
          class="btn btn-sm"
          :class="statusFilter === opt.value ? 'btn-primary' : 'btn-outline-secondary'"
          @click="statusFilter = opt.value"
        >
          {{ $t(opt.labelKey) }}
        </button>
      </div>
      <CompactListMetrics v-model="metricStatusFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="$t('market_stat_total')" @toggle="statusFilter = 'all'" />
    </div>

    <!-- 列表容器 -->
    <div class="marketplace-grid-container flex-grow-1 overflow-y-auto pe-1 pb-3">
      <!-- 骨架屏 -->
      <div v-if="loading" class="row g-4" aria-hidden="true">
        <div v-for="n in 8" :key="n" class="col-md-6 col-lg-4 col-xl-3">
          <div class="card h-100">
            <div class="card-body">
              <div class="d-flex justify-content-between align-items-start mb-3">
                <div class="skeleton" style="width: 64px; height: 64px; border-radius: 10px;"></div>
                <div class="skeleton" style="width: 72px; height: 20px;"></div>
              </div>
              <div class="skeleton mb-2" style="width: 60%; height: 18px;"></div>
              <div class="skeleton" style="width: 100%; height: 14px;"></div>
              <div class="skeleton mt-2" style="width: 80%; height: 14px;"></div>
              <div class="d-flex justify-content-between align-items-center mt-3 pt-3 border-top">
                <div class="skeleton" style="width: 88px; height: 30px;"></div>
                <div class="skeleton" style="width: 40px; height: 20px;"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 空状态：无插件 -->
      <div v-else-if="plugins.length === 0" class="empty-state">
        <i class="bi bi-box"></i>
        <p>{{ $t('market_empty_no_plugins') }}</p>
      </div>

      <!-- 空状态：筛选无结果 -->
      <div v-else-if="filteredPlugins.length === 0" class="empty-state">
        <i class="bi bi-funnel"></i>
        <p>{{ $t('market_empty_no_match') }}</p>
        <button type="button" class="btn btn-primary" @click="clearFilters">
          {{ $t('market_clear_filters') }}
        </button>
      </div>

      <!-- 卡片网格 -->
      <template v-else>
        <template v-if="categoryFilter === 'all'">
          <div v-if="platforms.length > 0" class="mb-4">
            <h5 class="mb-3 text-secondary text-uppercase fs-6 fw-semibold letter-spacing-sm">
              {{ $t('cat_platform_plugins') }}
            </h5>
            <div class="row g-4">
              <div v-for="plugin in platforms" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
                <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
              </div>
            </div>
          </div>
          <div v-if="protocols.length > 0" class="mb-4">
            <h5 class="mb-3 text-secondary text-uppercase fs-6 fw-semibold letter-spacing-sm">
              {{ $t('cat_protocol_plugins') }}
            </h5>
            <div class="row g-4">
              <div v-for="plugin in protocols" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
                <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
              </div>
            </div>
          </div>
          <div v-if="others.length > 0" class="mb-4">
            <h5 class="mb-3 text-secondary text-uppercase fs-6 fw-semibold letter-spacing-sm">
              {{ $t('cat_other_plugins') }}
            </h5>
            <div class="row g-4">
              <div v-for="plugin in others" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
                <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
              </div>
            </div>
          </div>
        </template>
        <div v-else class="row g-4">
          <div v-for="plugin in filteredPlugins" :key="plugin.name" class="col-md-6 col-lg-4 col-xl-3">
            <PluginCard :plugin="plugin" @configure="$emit('configure', plugin.name)" @update-status="handleStatusUpdate" />
          </div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import PluginCard from '../components/PluginCard.vue';
import CompactListMetrics from '../components/CompactListMetrics.vue';

const props = defineProps({
  plugins: Array,
  loading: Boolean
});

const emit = defineEmits(['configure', 'update-status']);

const { locale, t } = useI18n();

const search = ref('');
const categoryFilter = ref('all');
const statusFilter = ref('all');
const metricStatusFilters = ref([]);

const categoryOptions = [
  { value: 'all', labelKey: 'market_filter_all' },
  { value: 'platform', labelKey: 'cat_platform' },
  { value: 'protocol', labelKey: 'cat_protocol' },
  { value: 'other', labelKey: 'cat_other' }
];

const statusOptions = [
  { value: 'all', labelKey: 'market_filter_all' },
  { value: 'running', labelKey: 'status_running' },
  { value: 'stopped', labelKey: 'status_stopped' }
];

const pluginTime = (plugin) => Number(plugin.enabledAt || plugin.updatedAt || plugin.lastSyncedAt || 0);

const stats = computed(() => {
  const list = props.plugins || [];
  return {
    total: list.length,
    running: list.filter((p) => p.status === 'running').length,
    stopped: list.filter((p) => p.status !== 'running').length
  };
});

const metricCards = computed(() => [
  { key: 'total', label: t('market_stat_total'), value: stats.value.total, icon: 'bi-grid-fill', tone: 'brand' },
  { key: 'running', label: t('market_stat_running'), value: stats.value.running, icon: 'bi-play-circle-fill', tone: 'success' },
  { key: 'stopped', label: t('market_stat_stopped'), value: stats.value.stopped, icon: 'bi-pause-circle', tone: 'neutral' }
]);

const sortedPlugins = computed(() => {
  const list = props.plugins || [];
  return [...list].sort((a, b) => {
    const aEnabled = a.status === 'running' ? 1 : 0;
    const bEnabled = b.status === 'running' ? 1 : 0;
    if (aEnabled !== bEnabled) return bEnabled - aEnabled;
    if (aEnabled && bEnabled) return pluginTime(b) - pluginTime(a);
    return String(a.name).localeCompare(String(b.name));
  });
});

const filteredPlugins = computed(() => {
  let list = sortedPlugins.value;

  if (categoryFilter.value !== 'all') {
    list = list.filter((p) => (categoryFilter.value === 'other')
      ? (p.category !== 'platform' && p.category !== 'protocol')
      : p.category === categoryFilter.value);
  }

  if (statusFilter.value !== 'all') {
    list = list.filter((p) => (statusFilter.value === 'running') === (p.status === 'running'));
  }

  if (metricStatusFilters.value.length) {
    list = list.filter((plugin) => metricStatusFilters.value.includes(plugin.status === 'running' ? 'running' : 'stopped'));
  }

  const q = search.value.trim().toLowerCase();
  if (q) {
    list = list.filter((p) => {
      const title = p.title ? (p.title[locale.value] || p.title.en || p.name) : p.name;
      const desc = p.description ? (p.description[locale.value] || p.description.en || '') : '';
      return String(p.name).toLowerCase().includes(q)
        || String(title).toLowerCase().includes(q)
        || String(desc).toLowerCase().includes(q);
    });
  }
  return list;
});

const platforms = computed(() => filteredPlugins.value.filter((p) => p.category === 'platform'));
const protocols = computed(() => filteredPlugins.value.filter((p) => p.category === 'protocol'));
const others = computed(() => filteredPlugins.value.filter((p) => p.category !== 'platform' && p.category !== 'protocol'));

const clearFilters = () => {
  search.value = '';
  categoryFilter.value = 'all';
  statusFilter.value = 'all';
  metricStatusFilters.value = [];
};

const clearMetricStatusFilters = () => {
  metricStatusFilters.value = [];
  statusFilter.value = 'all';
};

const toggleMetricStatus = (status) => {
  statusFilter.value = 'all';
  metricStatusFilters.value = metricStatusFilters.value.includes(status)
    ? metricStatusFilters.value.filter((item) => item !== status)
    : [...metricStatusFilters.value, status];
};

const handleStatusUpdate = (name, enabled) => {
  emit('update-status', name, enabled);
};
</script>

<style scoped>
.letter-spacing-sm {
  letter-spacing: 0.05em;
}

.kpi-card,
.kpi-card--compact {
  background: var(--bg-surface);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-card, 14px);
  box-shadow: var(--card-shadow);
  transition: transform 0.4s cubic-bezier(0.34, 1.56, 0.64, 1), box-shadow 0.35s ease, border-color 0.35s ease;
}

.kpi-card--compact {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.45rem 0.85rem;
  min-height: 48px;
}

@media (hover: hover) and (pointer: fine) {
  .kpi-card:hover,
  .kpi-card--compact:hover {
    transform: translateY(-4px) scale(1.012);
    border-color: rgba(147, 197, 253, 0.75);
    box-shadow: var(--shadow-floating);
  }
}

.kpi-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.kpi-label {
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  line-height: 1.1;
  white-space: nowrap;
}

.kpi-value {
  font-size: 1.25rem;
  font-weight: 800;
  line-height: 1.15;
  color: var(--text-main);
  letter-spacing: -0.02em;
}

.kpi-icon-box {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  font-size: 0.95rem;
  flex-shrink: 0;
}

.icon-brand {
  color: var(--color-brand);
  background: rgba(59, 130, 246, 0.15);
}

.icon-chart-2 {
  color: var(--color-success, #16a34a);
  background: rgba(22, 163, 74, 0.15);
}

.icon-neutral {
  color: var(--text-secondary);
  background: rgba(100, 116, 139, 0.15);
}
</style>
