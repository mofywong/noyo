<template>
  <div class="card border-0 shadow-sm h-100">
    <div class="card-header bg-transparent border-0 d-flex justify-content-between align-items-center py-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('sidebar_device_drivers', '设备驱动') }}</h2>
      <button class="btn btn-primary btn-sm" @click="openCreateModal" v-permission="'product:create'">
        <i class="bi bi-plus-lg me-1"></i> {{ $t('driver_create', '新建驱动') }}
      </button>
    </div>
    <div class="card-body p-0">
      <div class="table-responsive">
        <table class="table table-hover align-middle mb-0">
          <thead class="bg-light">
            <tr>
              <th class="ps-4 d-none d-md-table-cell">{{ $t('driver_code', '驱动编码') }}</th>
              <th>{{ $t('driver_name', '驱动名称') }}</th>
              <th v-if="showProjectColumn" class="d-none d-lg-table-cell">{{ $t('project_name', '所属项目') }}</th>
              <th class="d-none d-lg-table-cell">{{ $t('prod_protocol', '通信协议') }}</th>
              <th class="d-none d-xl-table-cell" style="font-size: 0.8rem; color: #6c757d;">
                <div style="line-height: 1.2;">{{ $t('dev_created', '创建时间') }}</div>
                <div style="line-height: 1.2;">{{ $t('dev_updated', '更新时间') }}</div>
              </th>
              <th class="text-end pe-4">{{ $t('actions', '操作') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading" class="text-center">
              <td :colspan="showProjectColumn ? 6 : 5" class="py-4 text-muted">{{ $t('loading', '加载中...') }}</td>
            </tr>
            <tr v-else-if="drivers.length === 0" class="text-center">
              <td :colspan="showProjectColumn ? 6 : 5" class="py-4 text-muted">{{ $t('driver_no_data', '暂无驱动数据') }}</td>
            </tr>
            <tr v-for="driver in drivers" :key="driver.code">
              <td class="ps-4 font-monospace d-none d-md-table-cell">{{ driver.code }}</td>
              <td class="fw-bold">{{ driver.name }}</td>
              <td v-if="showProjectColumn" class="d-none d-lg-table-cell">
                <span class="badge text-bg-light border">{{ driver.project_name || '-' }}</span>
              </td>
              <td class="d-none d-lg-table-cell">
                <span class="badge bg-info bg-opacity-10 text-info">{{ getPluginTitle(driver.protocol_name) }}</span>
              </td>
              <td class="small text-muted d-none d-xl-table-cell" style="font-size: 0.75rem;">
                <div>{{ driver.CreatedAt ? new Date(driver.CreatedAt).toLocaleString() : '-' }}</div>
                <div>{{ driver.UpdatedAt ? new Date(driver.UpdatedAt).toLocaleString() : '-' }}</div>
              </td>
              <td class="text-end pe-4">
                <div class="btn-group btn-group-sm me-2">
                  <button class="btn btn-outline-secondary" :title="$t('edit', '编辑')" @click="openEditModal(driver)" v-permission="'product:edit'">
                    <i class="bi bi-pencil"></i>
                  </button>
                </div>
                <button class="btn btn-sm btn-outline-danger" :title="$t('delete', '删除')" @click="deleteDriver(driver.code)" v-permission="'product:delete'">
                  <i class="bi bi-trash"></i>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      
      <!-- Pagination -->
      <div v-if="total > 0" class="d-flex justify-content-between align-items-center p-3 border-top">
        <div class="text-muted small">
          {{ $t('pagination_total', { total: total }) }}
        </div>
        <nav :aria-label="$t('pagination_navigation')">
          <ul class="pagination pagination-sm mb-0">
            <li class="page-item" :class="{ disabled: page === 1 }">
              <button class="page-link" @click="changePage(page - 1)"><i class="bi bi-chevron-left"></i></button>
            </li>
            <li class="page-item disabled">
              <span class="page-link">{{ page }} / {{ Math.ceil(total / pageSize) }}</span>
            </li>
            <li class="page-item" :class="{ disabled: page >= Math.ceil(total / pageSize) }">
              <button class="page-link" @click="changePage(page + 1)"><i class="bi bi-chevron-right"></i></button>
            </li>
          </ul>
        </nav>
      </div>
    </div>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal fade show d-block" style="background: rgba(0,0,0,0.5)">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('driver_edit', '编辑驱动') : $t('driver_create', '新建驱动') }}</h5>
            <button type="button" class="btn-close" @click="closeModal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('driver_code', '驱动编码') }}</label>
              <input v-model="currentDriver.code" type="text" class="form-control" :disabled="isEditing">
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('driver_name', '驱动名称') }}</label>
              <input v-model="currentDriver.name" type="text" class="form-control">
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('prod_protocol', '通信协议') }}</label>
              <select v-model="currentDriver.protocol_name" class="form-select" :disabled="isEditing" @change="fetchSchema(currentDriver.protocol_name)">
                <option value="">{{ $t('driver_select_protocol', '请选择协议') }}</option>
                <option v-for="p in protocols" :key="p.name" :value="p.name">{{ getPluginTitle(p.name) }}</option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-label">{{ $t('description', '描述') }}</label>
              <textarea v-model="currentDriver.description" class="form-control" rows="2"></textarea>
            </div>
            <div class="mb-3" v-if="isScriptProtocol">
               <ScriptProductConfig
                      v-model="currentDriver.config"
                      :product-code="currentDriver.code"
                  />
            </div>
            <div class="mb-3" v-else-if="currentSchema">
               <label class="form-label">{{ $t('driver_config', '驱动配置') }}</label>
               <div class="border rounded p-3 bg-light">
                  <SchemaForm 
                      :schema="currentSchema" 
                      v-model="currentDriver.config" 
                  />
               </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="closeModal">{{ $t('cancel', '取消') }}</button>
            <button type="button" class="btn btn-primary" @click="saveDriver">{{ $t('save', '保存') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onBeforeUnmount, computed } from 'vue';
import axios from 'axios';
import { useToast } from '../composables/useToast';
import { useI18n } from 'vue-i18n';
import SchemaForm from '../components/SchemaForm.vue';
import ScriptProductConfig from '../components/script/ScriptProductConfig.vue';
import { useAuthStore } from '../stores/auth';
import { isSingleProjectMode } from '../utils/systemMode.js';

const { t, locale } = useI18n();
const { showToast } = useToast();
const authStore = useAuthStore();

const drivers = ref([]);
const protocols = ref([]);
const loading = ref(false);
const total = ref(0);
const page = ref(1);
const pageSize = ref(10);

const showModal = ref(false);
const isEditing = ref(false);
const currentDriver = ref({});
const currentSchema = ref(null);

const isScriptProtocol = computed(() => {
  return (currentDriver.value.protocol_name || '').toLowerCase() === 'script';
});

const showProjectColumn = computed(() => {
    const mode = localStorage.getItem('system_mode') || '';
    if (isSingleProjectMode(mode)) {
        return false;
    }
    return authStore.user?.role === 'tenant_admin' || authStore.user?.role === 'admin' || authStore.isGlobalAdmin;
  });

const loadDrivers = async () => {
  loading.value = true;
  try {
    const res = await axios.get('/api/protocol-profiles', {
      params: { page: page.value, pageSize: pageSize.value }
    });
    drivers.value = res.data.data || [];
    total.value = res.data.total || 0;
  } catch (err) {
    showToast(err.response?.data?.error || t('sys_error', 'System error'), 'danger');
  } finally {
    loading.value = false;
  }
};

const loadProtocols = async () => {
  try {
    const res = await axios.get('/api/plugins', {
      params: { category: 'protocol' }
    });
    protocols.value = (res.data.data || []).filter(p => p.status === 'running' && p.category === 'protocol');
  } catch (err) {
    console.danger('Failed to load protocols', err);
  }
};

const getPluginTitle = (name) => {
  const p = protocols.value.find(p => p.name === name);
  if (!p) return name;
  if (p.title) {
    return p.title[locale.value] || p.title['en'] || name;
  }
  return name;
};

const normalizeConfig = (config) => {
  if (!config) return {};
  if (typeof config === 'object') return { ...config };
  try {
    const parsed = JSON.parse(config);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : {};
  } catch (err) {
    console.error('Failed to parse driver config', err);
    return {};
  }
};

const fetchSchema = async (protocolName) => {
  if (!protocolName) {
    currentSchema.value = null;
    return;
  }
  currentDriver.value.config = normalizeConfig(currentDriver.value.config);
  if (protocolName.toLowerCase() === 'script') {
    currentSchema.value = null;
    return;
  }
  try {
    const res = await axios.get(`/api/devices/config-schema`, {
      params: { protocolName: protocolName, type: 'profile' }
    });
    if (res.data.code === 0) {
      currentSchema.value = res.data.data;
    } else {
      currentSchema.value = null;
    }
  } catch (err) {
    console.error('Failed to load schema', err);
    currentSchema.value = null;
  }
};

const openCreateModal = () => {
  const mode = localStorage.getItem('system_mode') || '';
  if (!isSingleProjectMode(mode) && Number(localStorage.getItem('current_project_id') || 0) === 0) {
    showToast(t('project_required', '请先选择项目再创建驱动 / Please select a project first'), 'danger');
    return;
  }
  isEditing.value = false;
  currentDriver.value = {
    code: '',
    name: '',
    protocol_name: '',
    description: '',
    config: {}
  };
  currentSchema.value = null;
  showModal.value = true;
};

const openEditModal = async (driver) => {
  isEditing.value = true;
  currentDriver.value = { ...driver, config: normalizeConfig(driver.config) };
  if (driver.protocol_name) {
    await fetchSchema(driver.protocol_name);
  }
  showModal.value = true;
};

const closeModal = () => {
  showModal.value = false;
  currentDriver.value = {};
  currentSchema.value = null;
};

const saveDriver = async () => {
  if (!currentDriver.value.code || !currentDriver.value.name || !currentDriver.value.protocol_name) {
    showToast(t('invalid_input', 'Invalid input'), 'warning');
    return;
  }

  try {
    const payload = {
      code: currentDriver.value.code,
      name: currentDriver.value.name,
      protocol_name: currentDriver.value.protocol_name,
      description: currentDriver.value.description,
      config: currentDriver.value.config
    };

    if (isEditing.value) {
      await axios.put(`/api/protocol-profiles/${currentDriver.value.code}`, payload);
    } else {
      await axios.post('/api/protocol-profiles', payload);
    }
    
    showToast(isEditing.value ? t('update_success', 'Update successful') : t('create_success', 'Create successful'), 'success');
    closeModal();
    loadDrivers();
  } catch (err) {
    showToast(err.response?.data?.error || t('sys_error', 'System error'), 'danger');
  }
};

const deleteDriver = async (code) => {
  if (!confirm(t('confirm_delete', 'Are you sure you want to delete this?'))) {
    return;
  }
  try {
    await axios.delete(`/api/protocol-profiles/${code}`);
    showToast(t('delete_success', 'Delete successful'), 'success');
    loadDrivers();
  } catch (err) {
    showToast(err.response?.data?.error || t('sys_error', 'System error'), 'danger');
  }
};

const changePage = (p) => {
  if (p < 1 || p > Math.ceil(total.value / pageSize.value)) return;
  page.value = p;
  loadDrivers();
};

onMounted(() => {
  loadProtocols();
  loadDrivers();
});

</script>
