<template>
  <div>
    <!-- Page Header（§7.2） -->
    <div class="page-header">
      <div>
        <h1>{{ $t('page_marketplace') }}</h1>
        <p class="page-subtitle">{{ $t('market_subtitle') }}</p>
      </div>
    </div>

    <!-- KPI 统计行（§7.3） -->
    <div class="row g-3 mb-4">
      <div class="col-6 col-md-4 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">{{ $t('market_stat_total') }}</div>
          <div class="kpi-value">{{ stats.total }}</div>
        </div>
      </div>
      <div class="col-6 col-md-4 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">
            <span class="status-dot status-dot--online me-1 align-middle"></span>{{ $t('market_stat_running') }}
          </div>
          <div class="kpi-value">{{ stats.running }}</div>
        </div>
      </div>
      <div class="col-6 col-md-4 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">
            <span class="status-dot status-dot--offline me-1 align-middle"></span>{{ $t('market_stat_stopped') }}
          </div>
          <div class="kpi-value">{{ stats.stopped }}</div>
        </div>
      </div>
    </div>

    <!-- Toolbar（§7.2：36px 控件） -->
    <div class="page-toolbar">
      <div class="input-group" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input
          v-model="search"
          type="search"
          class="form-control"
          :placeholder="$t('market_search_placeholder')"
          :aria-label="$t('market_search_placeholder')"
        >
      </div>
      <div class="btn-group" role="group" :aria-label="$t('market_filter_all')">
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
      <div class="btn-group" role="group" :aria-label="$t('status_running')">
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
    </div>

    <!-- 骨架屏（§8.7，>300ms 加载） -->
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

    <!-- 空状态：无插件（§8.6） -->
    <div v-else-if="plugins.length === 0" class="empty-state">
      <i class="bi bi-box"></i>
      <p>{{ $t('market_empty_no_plugins') }}</p>
    </div>

    <!-- 空状态：筛选无结果（§10.3 空结果规则） -->
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
</template>

<script setup>
import { ref, computed } from 'vue';
import { useI18n } from 'vue-i18n';
import PluginCard from '../components/PluginCard.vue';

const props = defineProps({
  plugins: Array,
  loading: Boolean
});

const emit = defineEmits(['configure', 'update-status']);

const { locale } = useI18n();

const search = ref('');
const categoryFilter = ref('all');
const statusFilter = ref('all');

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
};

const handleStatusUpdate = (name, enabled) => {
  emit('update-status', name, enabled);
};
</script>

<style scoped>
.letter-spacing-sm {
  letter-spacing: 0.05em;
}
</style>
