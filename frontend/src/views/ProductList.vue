<template>
  <div class="card border-0 shadow-sm h-100">
    <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center py-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('sidebar_products') }}</h2>
      <button class="btn btn-primary btn-sm" @click="openCreateModal" v-permission="'product:create'">
        <i class="bi bi-plus-lg me-1"></i> {{ $t('prod_create') }}
      </button>
    </div>
    <div class="card-body p-0">
      <div class="table-responsive">
        <table class="table table-hover align-middle mb-0">
          <thead class="bg-light">
            <tr>
              <th class="ps-4 d-none d-md-table-cell">{{ $t('prod_code') }}</th>
              <th>{{ $t('prod_name') }}</th>
              <th v-if="showProjectColumn" class="d-none d-lg-table-cell">{{ $t('project_name') }}</th>
              <th>{{ $t('prod_tsl_status') }}</th>
              <th class="d-none d-xl-table-cell" style="font-size: 0.8rem; color: #6c757d;">
                <div style="line-height: 1.2;">{{ $t('dev_created') }}</div>
                <div style="line-height: 1.2;">{{ $t('dev_updated') }}</div>
              </th>
              <th class="text-end pe-4">{{ $t('prod_actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading" class="text-center">
              <td :colspan="showProjectColumn ? 7 : 6" class="py-4 text-muted">{{ $t('loading') }}</td>
            </tr>
            <tr v-else-if="products.length === 0" class="text-center">
              <td :colspan="showProjectColumn ? 7 : 6" class="py-4 text-muted">{{ $t('prod_no_products') }}</td>
            </tr>
            <tr v-for="product in products" :key="product.code">
              <td class="ps-4 font-monospace d-none d-md-table-cell">{{ product.code }}</td>
              <td class="fw-bold">{{ product.name }}</td>
              <td v-if="showProjectColumn" class="d-none d-lg-table-cell">
                <span class="badge text-bg-light border">{{ product.project_name || '-' }}</span>
              </td>
              <td>
                <span v-if="hasTSL(product)" class="badge bg-success bg-opacity-10 text-success">
                  <i class="bi bi-check-circle me-1"></i>{{ $t('prod_tsl_configured') }}
                </span>
                <span v-else class="badge bg-secondary bg-opacity-10 text-secondary">
                  <i class="bi bi-dash-circle me-1"></i>{{ $t('prod_tsl_not_configured') }}
                </span>
              </td>
              <td class="small text-muted d-none d-xl-table-cell" style="font-size: 0.75rem;">
                <div>{{ product.CreatedAt ? new Date(product.CreatedAt).toLocaleString() : '-' }}</div>
                <div>{{ product.UpdatedAt ? new Date(product.UpdatedAt).toLocaleString() : '-' }}</div>
              </td>
              <td class="text-end pe-4">
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

    <!-- Create/Edit Info Modal -->
    <div v-if="showCreateModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-lg">
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
            <button type="button" class="btn btn-secondary" @click="closeCreateModal">{{ $t('tsl_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveProduct">{{ $t('tsl_confirm') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- TSL Editor Modal -->
    <div v-if="showTSLModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-xl">
        <div class="modal-content h-100">
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
    <div class="card-footer bg-transparent border-0 d-flex justify-content-end align-items-center py-3" v-if="total > 0">
      <div class="d-flex align-items-center gap-2">
        <select class="form-select form-select-sm" style="width: auto" v-model="pageSize" @change="changePageSize">
          <option :value="10">10 / {{ $t('page') }}</option>
          <option :value="20">20 / {{ $t('page') }}</option>
          <option :value="50">50 / {{ $t('page') }}</option>
        </select>
        <nav>
          <ul class="pagination pagination-sm mb-0">
            <li class="page-item disabled me-2 d-flex align-items-center">
              <span class="text-muted small border-0 bg-transparent">共 {{ total }} 条</span>
            </li>
            <li class="page-item" :class="{ disabled: page === 1 }">
              <button class="page-link" @click="changePage(page - 1)">
                <i class="bi bi-chevron-left"></i>
              </button>
            </li>
            <li class="page-item disabled">
              <span class="page-link">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
            </li>
            <li class="page-item" :class="{ disabled: page * pageSize >= total }">
              <button class="page-link" @click="changePage(page + 1)">
                <i class="bi bi-chevron-right"></i>
              </button>
            </li>
          </ul>
        </nav>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue';
import axios from 'axios';
import { useI18n } from 'vue-i18n';
import TSLEditor from '../components/tsl/TSLEditor.vue';
import { isSingleProjectMode } from '../utils/systemMode.js';

const { t, locale } = useI18n();

const products = ref([]);
const loading = ref(false);
const showCreateModal = ref(false);
const showTSLModal = ref(false);
const isEditing = ref(false);
const showProjectColumn = computed(() => {
  const mode = localStorage.getItem('system_mode') || '';
  if (isSingleProjectMode(mode)) return false;
  return Number(localStorage.getItem('current_project_id') || 0) === 0;
});

// 分页状?
const page = ref(1);
const pageSize = ref(10);
const total = ref(0);

// 产品信息状?
const currentProduct = ref({ code: '', name: '', config: {} });

// TSL 编辑器状?
const editingProduct = ref({});
const currentTSL = ref({ properties: [], events: [], services: [] });
const currentProtocolSchema = ref(null);
const currentMapping = ref({ points: [] });

const hasTSL = (product) => {
  try {
    const config = typeof product.config === 'string' 
      ? JSON.parse(product.config || '{}') 
      : product.config || {};
      
    if (!config.tsl) return false;
    
    // Check if any property, event or service is defined
    const { properties, events, services } = config.tsl;
    return (properties && properties.length > 0) || 
           (events && events.length > 0) || 
           (services && services.length > 0);
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
  } finally {
    loading.value = false;
  }
};

const changePage = (newPage) => {
  if (newPage < 1 || newPage > Math.ceil(total.value / pageSize.value)) return;
  page.value = newPage;
  fetchProducts();
};

const changePageSize = () => {
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
      fetchProducts();
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_save_fail'));
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

const closeTSLModal = () => {
  if (tslDirty.value) {
    if (!confirm('您有未保存的修改，确定要关闭吗？')) {
      return;
    }
  }
  showTSLModal.value = false;
  tslDirty.value = false;
};

// 监听 TSL 变化以标记未保存状?
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
      fetchProducts(); // Refresh list
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_save_fail'));
  }
};

const deleteProduct = async (code) => {
  if (!confirm(t('common_delete_confirm'))) return;
  try {
    const res = await axios.delete(`/api/products/${code}`);
    if (res.data.code === 0) {
      fetchProducts();
    } else {
      alert(res.data.message);
    }
  } catch (e) {
    console.error(e);
    alert(t('common_delete_fail'));
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