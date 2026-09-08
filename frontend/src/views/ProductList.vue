<template>
  <div class="product-list-page page-fixed-height">
    <!-- Page Header（§7.2） -->
    <div class="page-header list-page-header">
      <div>
        <h1>{{ $t('sidebar_products') }}</h1>
        <p class="page-subtitle">{{ $t('prod_subtitle') }}</p>
      </div>
      <div class="d-flex align-items-center gap-2">
        <LiquidGlassButton
          variant="secondary"
          size="sm"
          :icon="loading ? 'bi bi-arrow-repeat spin' : 'bi bi-arrow-clockwise'"
          :disabled="loading"
          @click="fetchProducts"
        >
          {{ $t('refresh') }}
        </LiquidGlassButton>
        <LiquidGlassButton
          variant="primary"
          size="sm"
          icon="bi bi-plus-lg"
          @click="openCreateModal"
          v-permission="'product:create'"
        >
          {{ $t('prod_create') }}
        </LiquidGlassButton>
      </div>
    </div>

    <!-- Toolbar（§7.2） -->
    <div class="page-toolbar device-list-control-bar list-page-controls">
      <div class="input-group list-toolbar-query" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input
          v-model="search"
          type="search"
          class="form-control"
          :placeholder="$t('prod_search_placeholder')"
          :aria-label="$t('prod_search_placeholder')"
        >
      </div>
      <CompactListMetrics v-model="metricFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="$t('prod_stat_total')" />
    </div>

    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4 d-none d-md-table-cell">{{ $t('prod_code') }}</th>
                <th>{{ $t('prod_name') }}</th>
                <th v-if="showProjectColumn" class="d-none d-lg-table-cell">{{ $t('project_name') }}</th>
                <th>{{ $t('prod_tsl_status') }}</th>
                <th class="d-none d-xl-table-cell">{{ $t('dev_created') }}</th>
                <th class="text-end pe-4">{{ $t('prod_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <!-- 骨架屏（§8.7） -->
              <tr v-if="loading" v-for="n in 5" :key="'sk-' + n">
                <td class="ps-4 d-none d-md-table-cell"><div class="skeleton" style="height: 16px; width: 120px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 160px;"></div></td>
                <td v-if="showProjectColumn" class="d-none d-lg-table-cell"><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td><div class="skeleton" style="height: 20px; width: 90px;"></div></td>
                <td class="d-none d-xl-table-cell"><div class="skeleton" style="height: 16px; width: 130px;"></div></td>
                <td class="text-end pe-4"><div class="skeleton ms-auto" style="height: 24px; width: 96px;"></div></td>
              </tr>
              <!-- 空状态：无产品（§8.6） -->
              <tr v-else-if="products.length === 0">
                <td :colspan="showProjectColumn ? 7 : 6" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-box-seam"></i>
                    <p>{{ $t('prod_empty_message') }}</p>
                    <button
                      type="button"
                      class="btn btn-primary"
                      @click="openCreateModal"
                      v-permission="'product:create'"
                    >
                      <i class="bi bi-plus-lg me-1"></i>{{ $t('prod_create') }}
                    </button>
                  </div>
                </td>
              </tr>
              <!-- 空状态：搜索无结果（§10.3） -->
              <tr v-else-if="filteredProducts.length === 0">
                <td :colspan="showProjectColumn ? 7 : 6" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-search"></i>
                    <p>{{ $t('prod_search_no_match') }}</p>
                    <button type="button" class="btn btn-primary" @click="clearSearch">
                      {{ $t('prod_clear_search') }}
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-for="product in filteredProducts" :key="product.code" class="device-row" @click="openProductDrawer(product)">
                <td class="ps-4 d-none d-md-table-cell font-mono small text-truncate" style="max-width: 200px;" :title="product.code">{{ product.code }}</td>
                <td class="fw-bold text-truncate" style="max-width: 200px;" :title="product.name">{{ product.name }}</td>
                <td v-if="showProjectColumn" class="d-none d-lg-table-cell">
                  <span class="badge text-bg-light border">{{ product.project_name || '-' }}</span>
                </td>
                <td>
                  <span v-if="hasTSL(product)" class="dash-pill dash-pill--success">
                    <span class="dash-pill-dot"></span>{{ $t('prod_tsl_configured') }}
                  </span>
                  <span v-else class="dash-pill dash-pill--neutral">
                    <span class="dash-pill-dot"></span>{{ $t('prod_tsl_not_configured') }}
                  </span>
                </td>
                <td class="d-none d-xl-table-cell">
                  <div class="font-mono small text-secondary">{{ formatDateTime(product.CreatedAt) }}</div>
                </td>
                <td class="text-end pe-4" @click.stop>
                  <div class="table-actions">
                    <button class="table-action-btn" :title="$t('prod_edit_info')" @click="openInfoEditModal(product)" v-permission="'product:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--primary" :title="$t('prod_edit_tsl')" @click="openTSLEditModal(product)" v-permission="'product:edit'">
                      <i class="bi bi-diagram-3"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--danger" :title="$t('tsl_delete')" @click="deleteProduct(product.code)" v-permission="'product:delete'">
                      <i class="bi bi-trash"></i>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <ListPagination :page="page" :page-size="pageSize" :total="total" :disabled="loading" id-prefix="products" @update:page="changePage" @update:page-size="changePageSize" />
    </div>

  <Teleport to="body">
    <!-- Create/Edit Info Modal -->
    <div v-if="showCreateModal" class="modal fade show d-block modal-overlay-theme">
      <div class="modal-dialog modal-lg modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('prod_edit_info') : $t('prod_create') }}</h5>
            <button type="button" class="btn-close" @click="closeCreateModal"></button>
          </div>
          <div class="modal-body">
            <div class="row">
              <div class="col-md-12">
                <div class="mb-3">
                  <label class="form-label">{{ $t('prod_code') }}</label>
                  <input v-model="currentProduct.code" type="text" class="form-control" :disabled="isEditing">
                </div>
                <div class="mb-3">
                  <label class="form-label">{{ $t('prod_name') }}</label>
                  <input v-model="currentProduct.name" type="text" class="form-control">
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-outline-secondary" @click="closeCreateModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveProduct">{{ $t('tsl_confirm') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- TSL Editor Modal -->
    <div v-if="showTSLModal" class="modal fade show d-block modal-overlay-theme">
      <div class="modal-dialog modal-xl modal-dialog-centered">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('prod_edit_tsl') }}: {{ editingProduct.name }}</h5>
            <button type="button" class="btn-close" @click="closeTSLModal"></button>
          </div>
          <div class="modal-body d-flex flex-column" style="height: 80vh;">
            <TSLEditor
              v-model="currentTSL"
              :protocolSchema="currentProtocolSchema"
              :mapping="currentMapping"
              @update:mapping="updateMapping"
              @save="saveTSL"
            />
          </div>
        </div>
      </div>
    </div>
    <!-- 产品详情抽屉（§8.4：与设备列表共用 DetailDrawer） -->
    <DetailDrawer :visible="drawerVisible" :title="drawerProduct?.name || drawerProduct?.code || ''" @close="closeProductDrawer">
      <template #header-badge>
        <span v-if="hasTSL(drawerProduct)" class="spec-badge spec-badge--success">
          <i class="bi bi-check-circle me-1"></i>{{ $t('prod_tsl_configured') }}
        </span>
        <span v-else class="spec-badge spec-badge--neutral">
          <i class="bi bi-dash-circle me-1"></i>{{ $t('prod_tsl_not_configured') }}
        </span>
      </template>
      <!-- 基本信息 -->
      <section class="noyo-drawer-section">
        <h6 class="noyo-drawer-section-title">{{ $t('drawer_basic_info') }}</h6>
        <dl class="row mb-0 fs-6">
          <dt class="col-4 text-secondary fw-normal">{{ $t('prod_code') }}</dt>
          <dd class="col-8 font-mono">{{ drawerProduct?.code || '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('prod_name') }}</dt>
          <dd class="col-8">{{ drawerProduct?.name || '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('project_name') }}</dt>
          <dd class="col-8">{{ drawerProduct?.project_name || '-' }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_created') }}</dt>
          <dd class="col-8 font-mono">{{ formatDateTime(drawerProduct?.CreatedAt) }}</dd>
          <dt class="col-4 text-secondary fw-normal">{{ $t('dev_updated') }}</dt>
          <dd class="col-8 font-mono">{{ formatDateTime(drawerProduct?.UpdatedAt) }}</dd>
        </dl>
      </section>
      <!-- 物模型配置摘要 -->
      <section v-if="drawerProduct" class="noyo-drawer-section">
        <h6 class="noyo-drawer-section-title">{{ $t('prod_edit_tsl') }}</h6>
        <div class="d-flex gap-4">
          <div>
            <div class="text-secondary small">{{ $t('tsl_properties') }}</div>
            <div class="font-mono fs-5 fw-bold">{{ drawerTSLStats.properties }}</div>
          </div>
          <div>
            <div class="text-secondary small">{{ $t('tsl_events') }}</div>
            <div class="font-mono fs-5 fw-bold">{{ drawerTSLStats.events }}</div>
          </div>
          <div>
            <div class="text-secondary small">{{ $t('tsl_services') }}</div>
            <div class="font-mono fs-5 fw-bold">{{ drawerTSLStats.services }}</div>
          </div>
          <div>
            <div class="text-secondary small">{{ $t('tsl_points') }}</div>
            <div class="font-mono fs-5 fw-bold">{{ drawerTSLStats.points }}</div>
          </div>
        </div>
      </section>
      <template #footer>
        <button class="btn btn-outline-secondary btn-sm" @click="openInfoEditModal(drawerProduct)" v-permission="'product:edit'">
          <i class="bi bi-pencil me-1"></i>{{ $t('prod_edit_info') }}
        </button>
        <button class="btn btn-outline-primary btn-sm" @click="openTSLEditModal(drawerProduct)" v-permission="'product:edit'">
          <i class="bi bi-diagram-3 me-1"></i>{{ $t('prod_edit_tsl') }}
        </button>
        <button class="btn btn-outline-danger btn-sm" @click="deleteProduct(drawerProduct.code)" v-permission="'product:delete'">
          <i class="bi bi-trash me-1"></i>{{ $t('tsl_delete') }}
        </button>
      </template>
    </DetailDrawer>
  </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import TSLEditor from '../components/tsl/TSLEditor.vue';
import ListPagination from '../components/ListPagination.vue';
import DetailDrawer from '../components/DetailDrawer.vue';
import LiquidGlassButton from '../components/liquid-glass/LiquidGlassButton.vue';
import { useConfirm } from '../composables/useConfirm';
import { useToast } from '../composables/useToast';
import { isSingleProjectMode } from '../utils/systemMode.js';
import { formatDateTime } from '../utils/dateTime.js';
import CompactListMetrics from '../components/CompactListMetrics.vue';

const { t } = useI18n();
const { confirmDialog } = useConfirm();
const { showToast } = useToast();

const products = ref([]);
const loading = ref(false);
const showCreateModal = ref(false);
const showTSLModal = ref(false);
const isEditing = ref(false);
const search = ref('');
const showProjectColumn = computed(() => {
  const mode = localStorage.getItem('system_mode') || '';
  if (isSingleProjectMode(mode)) return false;
  return Number(localStorage.getItem('current_project_id') || 0) === 0;
});

// 分页状态
const page = ref(1);
const pageSize = ref(10);
const total = ref(0);

// 产品信息状态
const currentProduct = ref({ code: '', name: '', config: {} });

// TSL 编辑器状态
const editingProduct = ref({});
const currentTSL = ref({ properties: [], events: [], services: [] });
const currentProtocolSchema = ref(null);
const currentMapping = ref({ points: [] });

const stats = computed(() => ({
  total: products.value.length,
  tslConfigured: products.value.filter(hasTSL).length,
  tslNotConfigured: products.value.filter((p) => !hasTSL(p)).length
}));

const metricCards = computed(() => [
  { key: 'total', label: t('prod_stat_total'), value: stats.value.total, icon: 'bi-box-seam-fill', tone: 'brand' },
  { key: 'configured', label: t('prod_stat_tsl_configured'), value: stats.value.tslConfigured, icon: 'bi-check-circle-fill', tone: 'success' },
  { key: 'notConfigured', label: t('prod_stat_tsl_not'), value: stats.value.tslNotConfigured, icon: 'bi-dash-circle', tone: 'neutral' }
]);

const metricFilters = ref([]);
const toggleMetricFilter = (key) => {
  metricFilters.value = metricFilters.value.includes(key)
    ? metricFilters.value.filter((item) => item !== key)
    : [...metricFilters.value, key];
};

// 产品详情抽屉（§8.4：与设备列表共用 DetailDrawer）
const drawerVisible = ref(false);
const drawerProduct = ref(null);

const drawerTSLStats = computed(() => {
  const p = drawerProduct.value;
  if (!p) return { properties: 0, events: 0, services: 0, points: 0 };
  let cfg = {};
  try {
    cfg = typeof p.config === 'string' ? JSON.parse(p.config || '{}') : p.config || {};
  } catch (e) {
    cfg = {};
  }
  return {
    properties: cfg.tsl?.properties?.length || 0,
    events: cfg.tsl?.events?.length || 0,
    services: cfg.tsl?.services?.length || 0,
    points: Array.isArray(cfg.points) ? cfg.points.length : 0
  };
});

const openProductDrawer = (product) => {
  drawerProduct.value = product;
  drawerVisible.value = true;
};

const closeProductDrawer = () => {
  drawerVisible.value = false;
  drawerProduct.value = null;
};

const filteredProducts = computed(() => {
  const q = search.value.trim().toLowerCase();
  return products.value.filter((product) => {
    const searchable = String(product.code || '').toLowerCase().includes(q)
      || String(product.name || '').toLowerCase().includes(q);
    if (q && !searchable) return false;
    if (!metricFilters.value.length) return true;
    return (metricFilters.value.includes('configured') && hasTSL(product))
      || (metricFilters.value.includes('notConfigured') && !hasTSL(product));
  });
});

const clearSearch = () => {
  search.value = '';
};

const hasTSL = (product) => {
  try {
    const config = typeof product.config === 'string'
      ? JSON.parse(product.config || '{}')
      : product.config || {};

    if (!config.tsl) return false;

    // Check if any property, event or service is defined
    const { properties, events, services } = config.tsl;
    return (properties && properties.length > 0)
      || (events && events.length > 0)
      || (services && services.length > 0);
  } catch (e) {
    return false;
  }
};

const fetchProducts = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/products', {
      params: {
        page: page.value,
        pageSize: pageSize.value
      }
    });
    if (res.data.code === 0) {
      products.value = res.data.data || [];
      total.value = res.data.total || 0;
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('prod_load_fail'));
  } finally {
    loading.value = false;
  }
};

const changePage = (newPage) => {
  if (newPage < 1 || newPage > Math.ceil(total.value / pageSize.value)) return;
  page.value = newPage;
  fetchProducts();
};

const changePageSize = (size) => {
  pageSize.value = Number(size) || 10;
  page.value = 1;
  fetchProducts();
};

// Open Create Modal
const openCreateModal = () => {
  isEditing.value = false;
  currentProduct.value = { code: '', name: '', config: {} };
  showCreateModal.value = true;
};

// Open Info Edit Modal
const openInfoEditModal = async (product) => {
  isEditing.value = true;
  // Deep copy
  currentProduct.value = { ...product };
  // Parse config if string
  if (typeof currentProduct.value.config === 'string') {
    try {
      currentProduct.value.config = JSON.parse(currentProduct.value.config);
    } catch (e) {
      currentProduct.value.config = {};
    }
  }
  showCreateModal.value = true;
};

const closeCreateModal = () => {
  showCreateModal.value = false;
};

const saveProduct = async () => {
  try {
    const configToSave = { ...(currentProduct.value.config || {}) };

    const payload = {
      ...currentProduct.value,
      config: JSON.stringify(configToSave)
    };

    let res;
    if (isEditing.value) {
      res = await axios.put(`/api/products/${currentProduct.value.code}`, payload);
    } else {
      res = await axios.post('/api/products', payload);
    }

    if (res.data.code === 0) {
      closeCreateModal();
      showToast('success', t('prod_save_success'));
      fetchProducts();
    } else {
      showToast('danger', res.data.message || t('common_save_fail'));
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('common_save_fail'));
  }
};

// TSL Logic
const openTSLEditModal = async (product) => {
  editingProduct.value = { ...product };

  // Load Config
  let configObj = {};
  try {
    configObj = JSON.parse(product.config || '{}');
  } catch (e) {
    configObj = {};
  }

  // Extract TSL
  if (configObj.tsl) {
    // Deep clone to prevent mutating original config and avoid immediate watch triggers on mount
    currentTSL.value = JSON.parse(JSON.stringify(configObj.tsl));
  } else {
    currentTSL.value = { properties: [], events: [], services: [] };
  }

  // Extract Mapping (Points)
  if (configObj.points) {
    currentMapping.value = JSON.parse(JSON.stringify({ points: configObj.points }));
  } else {
    currentMapping.value = { points: [] };
  }

  currentProtocolSchema.value = null; // Removed protocol schema dependency from Product

  showTSLModal.value = true;
  setTimeout(() => { tslDirty.value = false; }, 0);
};

const closeTSLModal = async () => {
  if (tslDirty.value) {
    // 未保存修改确认（§10.3 非破坏性，neutral 变体）
    const ok = await confirmDialog({
      title: t('prod_unsaved_title'),
      message: t('prod_unsaved_message'),
      variant: 'neutral',
      confirmText: t('common_confirm'),
      cancelText: t('common_cancel')
    });
    if (!ok) return;
  }
  showTSLModal.value = false;
  tslDirty.value = false;
};

// 监听 TSL 变化以标记未保存状态
const tslDirty = ref(false);
watch([currentTSL, currentMapping], () => {
  if (showTSLModal.value) {
    tslDirty.value = true;
  }
}, { deep: true });

const updateMapping = (newMapping) => {
  currentMapping.value = newMapping;
};

const saveTSL = async () => {
  try {
    let configObj = {};
    try {
      configObj = JSON.parse(editingProduct.value.config || '{}');
    } catch (e) {
      configObj = {};
    }

    configObj.tsl = currentTSL.value;
    configObj.points = currentMapping.value.points;

    const payload = {
      ...editingProduct.value,
      config: JSON.stringify(configObj)
    };

    const res = await axios.put(`/api/products/${editingProduct.value.code}`, payload);
    if (res.data.code === 0) {
      tslDirty.value = false;
      showTSLModal.value = false;
      showToast('success', t('prod_save_success'));
      fetchProducts(); // Refresh list
    } else {
      showToast('danger', res.data.message || t('common_save_fail'));
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('common_save_fail'));
  }
};

const deleteProduct = async (code) => {
  // 破坏性操作确认（§8.4 / §10.3）
  const ok = await confirmDialog({
    title: t('prod_delete_title'),
    message: t('prod_delete_message', { code }),
    variant: 'danger',
    confirmText: t('common_delete'),
    cancelText: t('common_cancel')
  });
  if (!ok) return;
  try {
    const res = await axios.delete(`/api/products/${code}`);
    if (res.data.code === 0) {
      if (drawerProduct.value?.code === code) closeProductDrawer();
      showToast('success', t('prod_delete_success'));
      fetchProducts();
    } else {
      showToast('danger', res.data.message || t('common_delete_fail'));
    }
  } catch (e) {
    console.error(e);
    showToast('danger', t('common_delete_fail'));
  }
};

onMounted(() => {
  fetchProducts();
  window.addEventListener('noyo-data-updated', fetchProducts);
});

onUnmounted(() => {
  window.removeEventListener('noyo-data-updated', fetchProducts);
});
</script>

<style scoped>
/* ── 液态玻璃环境光底景 / Liquid Glass ambient aurora backdrop ── */
.product-list-page {
  position: relative;
}

/* 内容浮于环境光之上（positioned siblings 按 DOM 顺序绘制在 aurora 之上）；
   仅提升页面内容容器，避免误伤 fixed 定位的 modal / drawer */
.product-list-page > .page-header,
.product-list-page > .page-toolbar,
.product-list-page > .row,
.product-list-page > .card {
  position: relative;
}



.kpi-card,
.kpi-card--compact {
  background: var(--noyo-dashboard-liquid-tint, var(--bg-surface));
  backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  -webkit-backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  border: 1px solid var(--noyo-dashboard-liquid-edge, var(--border-color));
  border-radius: var(--radius-card, 14px);
  box-shadow:
    var(--noyo-dashboard-liquid-shadow, var(--card-shadow)),
    inset 0 1.5px 0.5px var(--noyo-dashboard-liquid-specular, rgba(255, 255, 255, 0.85)),
    inset 0 -1.5px 1px rgba(0, 0, 0, 0.05);
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

/* 晶体发光胶囊 */
.dash-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.72rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  line-height: 1.2;
  transition: all var(--noyo-duration-fast) var(--noyo-ease-standard);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
}

.dash-pill-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: currentColor;
  box-shadow: 0 0 6px currentColor;
  flex-shrink: 0;
  display: inline-block;
}

.dash-pill--success {
  color: #15803d !important;
  background: rgba(22, 163, 74, 0.12);
  border: 1px solid rgba(22, 163, 74, 0.28);
}

[data-bs-theme="dark"] .dash-pill--success {
  color: #4ade80 !important;
  background: rgba(74, 222, 128, 0.15);
  border: 1px solid rgba(74, 222, 128, 0.35);
}

.dash-pill--neutral {
  color: var(--text-secondary) !important;
  background: rgba(100, 116, 139, 0.12);
  border: 1px solid rgba(100, 116, 139, 0.24);
}

.device-row {
  cursor: pointer;
  transition: background-color var(--noyo-duration-fast) var(--noyo-ease-standard);
}

.table-glass-card {
  border-radius: var(--radius-card, 16px);
  background: var(--noyo-dashboard-liquid-tint, var(--bg-surface));
  backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  -webkit-backdrop-filter: blur(var(--noyo-dashboard-liquid-blur, 10px)) saturate(175%) brightness(var(--noyo-dashboard-liquid-backdrop-brightness, 1.05));
  border: 1px solid var(--noyo-dashboard-liquid-edge, var(--border-color));
}
</style>
