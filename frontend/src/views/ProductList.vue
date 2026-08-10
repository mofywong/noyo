<template>
  <div>
    <!-- Page Header（§7.2） -->
    <div class="page-header">
      <div>
        <h1>{{ $t('sidebar_products') }}</h1>
        <p class="page-subtitle">{{ $t('prod_subtitle') }}</p>
      </div>
      <button class="btn btn-primary" @click="openCreateModal" v-permission="'product:create'">
        <i class="bi bi-plus-lg me-1"></i> {{ $t('prod_create') }}
      </button>
    </div>

    <!-- KPI 统计行（§7.3） -->
    <div class="row g-3 mb-3">
      <div class="col-6 col-md-4 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">{{ $t('prod_stat_total') }}</div>
          <div class="kpi-value">{{ stats.total }}</div>
        </div>
      </div>
      <div class="col-6 col-md-4 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">
            <i class="bi bi-check-circle me-1" style="color: var(--color-success);"></i>{{ $t('prod_stat_tsl_configured') }}
          </div>
          <div class="kpi-value">{{ stats.tslConfigured }}</div>
        </div>
      </div>
      <div class="col-6 col-md-4 col-xl-2">
        <div class="kpi-card">
          <div class="kpi-label">
            <i class="bi bi-dash-circle me-1" style="color: var(--text-tertiary);"></i>{{ $t('prod_stat_tsl_not') }}
          </div>
          <div class="kpi-value">{{ stats.tslNotConfigured }}</div>
        </div>
      </div>
    </div>

    <!-- Toolbar（§7.2） -->
    <div class="page-toolbar">
      <div class="input-group" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input
          v-model="search"
          type="search"
          class="form-control"
          :placeholder="$t('prod_search_placeholder')"
          :aria-label="$t('prod_search_placeholder')"
        >
      </div>
    </div>

    <div class="card border-0 shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
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
                    <button v-permission="'product:create'" type="button" class="btn btn-primary" @click="openCreateModal">
                      {{ $t('prod_create') }}
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
                  <span v-if="hasTSL(product)" class="spec-badge spec-badge--success">
                    <i class="bi bi-check-circle me-1"></i>{{ $t('prod_tsl_configured') }}
                  </span>
                  <span v-else class="spec-badge spec-badge--neutral">
                    <i class="bi bi-dash-circle me-1"></i>{{ $t('prod_tsl_not_configured') }}
                  </span>
                </td>
                <td class="d-none d-xl-table-cell">
                  <div class="font-mono small text-secondary">{{ formatDateTime(product.CreatedAt) }}</div>
                </td>
                <td class="text-end pe-4" @click.stop>
                  <div class="btn-group btn-group-sm me-2">
                    <button class="btn btn-outline-secondary" :title="$t('prod_edit_info')" @click="openInfoEditModal(product)" v-permission="'product:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="btn btn-outline-primary" :title="$t('prod_edit_tsl')" @click="openTSLEditModal(product)" v-permission="'product:edit'">
                      <i class="bi bi-diagram-3"></i>
                    </button>
                  </div>
                  <button class="btn btn-sm btn-outline-danger" :title="$t('tsl_delete')" @click="deleteProduct(product.code)" v-permission="'product:delete'">
                    <i class="bi bi-trash"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <ListPagination :page="page" :page-size="pageSize" :total="total" :disabled="loading" id-prefix="products" @update:page="changePage" @update:page-size="changePageSize" />
    </div>

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
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import TSLEditor from '../components/tsl/TSLEditor.vue';
import ListPagination from '../components/ListPagination.vue';
import DetailDrawer from '../components/DetailDrawer.vue';
import { useConfirm } from '../composables/useConfirm';
import { useToast } from '../composables/useToast';
import { isSingleProjectMode } from '../utils/systemMode.js';
import { formatDateTime } from '../utils/dateTime.js';

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
  if (!q) return products.value;
  return products.value.filter((p) =>
    String(p.code || '').toLowerCase().includes(q)
    || String(p.name || '').toLowerCase().includes(q)
  );
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
