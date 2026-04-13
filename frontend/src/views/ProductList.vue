<template>
  <div class="card border-0 shadow-sm h-100">
    <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center py-3">
      <h5 class="mb-0">{{ $t('sidebar_products') }}</h5>
      <button class="btn btn-primary btn-sm" @click="openCreateModal">
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
              <th class="d-none d-lg-table-cell">{{ $t('prod_protocol') }}</th>
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
              <td colspan="6" class="py-4 text-muted">{{ $t('loading') }}</td>
            </tr>
            <tr v-else-if="products.length === 0" class="text-center">
              <td colspan="6" class="py-4 text-muted">{{ $t('prod_no_products') }}</td>
            </tr>
            <tr v-for="product in products" :key="product.code">
              <td class="ps-4 font-monospace d-none d-md-table-cell">{{ product.code }}</td>
              <td class="fw-bold">{{ product.name }}</td>
              <td class="d-none d-lg-table-cell">
                <span v-if="product.protocol_name" class="badge bg-info bg-opacity-10 text-info">{{ getPluginTitle(product.protocol_name) }}</span>
                <span v-else class="badge bg-secondary bg-opacity-10 text-secondary">{{ $t('prod_subdevice_only') }}</span>
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
                  <button class="btn btn-outline-secondary" :title="$t('prod_edit_info')" @click="openInfoEditModal(product)">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-outline-primary" :title="$t('prod_edit_tsl')" @click="openTSLEditModal(product)">
                    <i class="bi bi-diagram-3"></i>
                  </button>
                </div>
                <button class="btn btn-sm btn-outline-danger" :title="$t('tsl_delete')" @click="deleteProduct(product.code)">
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
                <div class="mb-3">
                  <label class="form-label">{{ $t('prod_protocol') }}</label>
                  <select v-model="currentProduct.protocol_name" class="form-select" :disabled="isEditing">
                    <option value="">{{ $t('prod_no_protocol') }}</option>
                    <option v-for="p in protocols" :key="p.name" :value="p.name">{{ getPluginTitle(p.name) }}</option>
                  </select>
                  <div v-if="!currentProduct.protocol_name" class="form-text text-warning">
                    <i class="bi bi-exclamation-triangle me-1"></i>{{ $t('prod_no_protocol_hint') }}
                  </div>
                </div>
                <div class="mb-3" v-if="currentSchema || currentProduct.protocol_name === 'Script' || currentProduct.protocol_name === 'script'">
                   <label class="form-label">{{ $t('prod_proto_config') }}</label>
                   <div class="border rounded p-3 bg-light">
                      <ScriptProductConfig 
                          v-if="currentProduct.protocol_name === 'Script' || currentProduct.protocol_name === 'script'"
                          :modelValue="currentProduct.config"
                          @update:modelValue="handleScriptConfigUpdate"
                          :product-code="currentProduct.code"
                      />
                      <SchemaForm 
                          v-else
                          :schema="currentSchema" 
                          v-model="currentProduct.config" 
                      />
                   </div>
                </div>
                <!-- 子设备配置参数定义（仅当协议允许自定义时显示） -->
                <div class="mb-3" v-if="currentProduct.protocol_name && subDeviceConfigCustomizable">
                   <label class="form-label">
                     {{ $t('prod_sub_device_params') }}
                     <span class="text-muted small ms-1">({{ $t('optional') }})</span>
                   </label>
                   <div class="form-text text-muted mb-2">{{ $t('prod_sub_device_params_hint') }}</div>
                   <div class="border rounded p-3 bg-light">
                      <div v-for="(param, index) in subDeviceParams" :key="index" class="row g-2 mb-2 align-items-center">
                        <div class="col-3">
                          <input type="text" class="form-control form-control-sm" v-model="param.key" :placeholder="$t('script_param_key')">
                        </div>
                        <div class="col-3">
                          <input type="text" class="form-control form-control-sm" v-model="param.title" :placeholder="$t('script_param_name')">
                        </div>
                        <div class="col-2">
                          <select class="form-select form-select-sm" v-model="param.type">
                            <option value="string">String</option>
                            <option value="integer">Integer</option>
                            <option value="number">Number</option>
                            <option value="boolean">Boolean</option>
                          </select>
                        </div>
                        <div class="col-2">
                          <input type="text" class="form-control form-control-sm" v-model="param.default" :placeholder="$t('default')">
                        </div>
                        <div class="col-1">
                          <div class="form-check">
                            <input class="form-check-input" type="checkbox" v-model="param.required">
                          </div>
                        </div>
                        <div class="col-1">
                          <button type="button" class="btn btn-sm btn-outline-danger" @click="removeSubDeviceParam(index)">
                            <i class="bi bi-trash"></i>
                          </button>
                        </div>
                      </div>
                      <div v-if="subDeviceParams.length > 0" class="row g-2 mb-2 text-muted small">
                        <div class="col-3">{{ $t('script_param_key') }}</div>
                        <div class="col-3">{{ $t('script_param_name') }}</div>
                        <div class="col-2">{{ $t('script_param_type') }}</div>
                        <div class="col-2">{{ $t('default') }}</div>
                        <div class="col-1">{{ $t('script_param_required') }}</div>
                        <div class="col-1"></div>
                      </div>
                      <button type="button" class="btn btn-sm btn-outline-primary" @click="addSubDeviceParam">
                        <i class="bi bi-plus me-1"></i>{{ $t('script_add_param') }}
                      </button>
                   </div>
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
            <button type="button" class="btn-close" @click="showTSLModal = false"></button>
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
import SchemaForm from '../components/SchemaForm.vue';
import ScriptProductConfig from '../components/script/ScriptProductConfig.vue';
import TSLEditor from '../components/tsl/TSLEditor.vue';

const { t, locale } = useI18n();

const getPluginTitle = (name) => {
  const p = protocols.value.find(plugin => plugin.name === name);
  if (p && p.title) {
    if (typeof p.title === 'string') return p.title;
    return p.title[locale.value] || p.title['en'] || name;
  }
  return name;
};

const products = ref([]);
const loading = ref(false);
const showCreateModal = ref(false);
const showTSLModal = ref(false);
const isEditing = ref(false);
const protocols = ref([]);

// 分页状态
const page = ref(1);
const pageSize = ref(10);
const total = ref(0);

// 产品信息状态
const currentProduct = ref({ code: '', name: '', protocol_name: '', config: {} });
const currentSchema = ref(null);

// 子设备配置参数
const subDeviceParams = ref([]);
const subDeviceConfigCustomizable = ref(true);

const addSubDeviceParam = () => {
  subDeviceParams.value.push({ key: '', title: '', type: 'string', default: '', required: false });
};

const removeSubDeviceParam = (index) => {
  subDeviceParams.value.splice(index, 1);
};

// 将 subDeviceParams 转换为 JSON Schema 格式
const buildSubDeviceConfigSchema = () => {
  if (subDeviceParams.value.length === 0) return null;
  const properties = {};
  const required = [];
  for (const param of subDeviceParams.value) {
    if (!param.key) continue;
    const prop = { type: param.type, title: param.title || param.key };
    if (param.default !== '') {
      if (param.type === 'integer') prop.default = parseInt(param.default) || 0;
      else if (param.type === 'number') prop.default = parseFloat(param.default) || 0;
      else if (param.type === 'boolean') prop.default = param.default === 'true';
      else prop.default = param.default;
    }
    properties[param.key] = prop;
    if (param.required) required.push(param.key);
  }
  if (Object.keys(properties).length === 0) return null;
  const schema = { type: 'object', properties };
  if (required.length > 0) schema.required = required;
  return schema;
};

// 从产品配置中加载 subDeviceParams
const loadSubDeviceParams = (config) => {
  const schema = config?.sub_device_config_schema;
  if (!schema || !schema.properties) {
    subDeviceParams.value = [];
    return;
  }
  const params = [];
  const requiredList = schema.required || [];
  for (const [key, prop] of Object.entries(schema.properties)) {
    params.push({
      key,
      title: prop.title || '',
      type: prop.type || 'string',
      default: prop.default !== undefined ? String(prop.default) : '',
      required: requiredList.includes(key)
    });
  }
  subDeviceParams.value = params;
};

// TSL 编辑器状态
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

const fetchProtocols = async () => {
  try {
    const res = await axios.get('/api/plugins?type=protocol');
    if (res.data.code === 0) {
      protocols.value = res.data.data.filter(p => p.category === 'protocol');
    }
  } catch (e) {
    console.error(e);
  }
};

const fetchProtocolSchema = async (protocolName) => {
  if (!protocolName) {
    subDeviceConfigCustomizable.value = true;
    return null;
  }
  try {
    const res = await axios.get(`/api/plugins/${protocolName}/schemas`);
    if (res.data.code === 0) {
      // 更新子设备配置可自定义标志
      subDeviceConfigCustomizable.value = res.data.data.subDeviceConfigCustomizable !== false;
      if (res.data.data.product) {
        return res.data.data.product;
      }
    }
  } catch (e) {
    console.error(e);
  }
  return null;
};

// Watch protocol change in Create/Edit Modal
watch(() => currentProduct.value.protocol_name, async (newVal) => {
  if (newVal) {
    currentSchema.value = await fetchProtocolSchema(newVal);
  } else {
    currentSchema.value = null;
  }
});

// Open Create Modal
const openCreateModal = () => {
  isEditing.value = false;
  currentProduct.value = { code: '', name: '', protocol_name: '', config: {} };
  currentSchema.value = null;
  subDeviceParams.value = [];
  subDeviceConfigCustomizable.value = true;
  showCreateModal.value = true;
};

const handleScriptConfigUpdate = (newConfig) => {
  console.log('ProductList: Received config update from ScriptProductConfig', newConfig);
  // Ensure reactivity by assigning a new object via deep clone
  currentProduct.value.config = JSON.parse(JSON.stringify(newConfig));
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
  // 加载子设备配置参数
  loadSubDeviceParams(currentProduct.value.config);
  // Fetch Schema
  if (currentProduct.value.protocol_name) {
    currentSchema.value = await fetchProtocolSchema(currentProduct.value.protocol_name);
  }
  showCreateModal.value = true;
};

const closeCreateModal = () => {
  showCreateModal.value = false;
};

const saveProduct = async () => {
  try {
    // 将子设备配置参数合并到产品配置中
    const configToSave = { ...(currentProduct.value.config || {}) };
    const subSchema = buildSubDeviceConfigSchema();
    if (subSchema) {
      configToSave.sub_device_config_schema = subSchema;
    } else {
      delete configToSave.sub_device_config_schema;
    }

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
    currentTSL.value = configObj.tsl;
  } else {
    currentTSL.value = { properties: [], events: [], services: [] };
  }

  // Extract Mapping (Points)
  if (configObj.points) {
    currentMapping.value = { points: configObj.points };
  } else {
    currentMapping.value = { points: [] };
  }

  // Fetch Protocol Schema (for mapping UI)
  if (product.protocol_name) {
    currentProtocolSchema.value = await fetchProtocolSchema(product.protocol_name);
  }

  showTSLModal.value = true;
};

const updateMapping = (newMapping) => {
  currentMapping.value = newMapping;
};

const saveTSL = async () => {
  try {
    // 1. Get current config from product (or reload it to be safe, but here we use what we have)
    let configObj = {};
    try {
       // Re-parse original config to preserve other fields (like polling_groups)
       configObj = JSON.parse(editingProduct.value.config || '{}');
    } catch (e) {
       configObj = {};
    }

    // 2. Update TSL and Points
    configObj.tsl = currentTSL.value;
    configObj.points = currentMapping.value.points;

    // 3. Save
    const payload = {
      ...editingProduct.value,
      config: JSON.stringify(configObj)
    };

    const res = await axios.put(`/api/products/${editingProduct.value.code}`, payload);
    if (res.data.code === 0) {
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
  fetchProtocols();
  window.addEventListener('noyo-data-updated', fetchProducts);
});

onUnmounted(() => {
  window.removeEventListener('noyo-data-updated', fetchProducts);
});
</script>
