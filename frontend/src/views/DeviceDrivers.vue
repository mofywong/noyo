<template>
  <div class="device-drivers-page page-fixed-height">
    <!-- Page Header（§7.2） -->
    <div class="page-header list-page-header">
      <div>
        <h1>{{ $t('sidebar_device_drivers', '设备驱动') }}</h1>
        <p class="page-subtitle">{{ $t('driver_subtitle', '管理设备驱动配置文件与协议转换规则') }}</p>
      </div>
      <LiquidGlassButton
        variant="primary"
        icon="bi bi-plus-lg"
        @click="openCreateModal"
        v-permission="'product:create'"
      >
        {{ $t('driver_create', '新建驱动') }}
      </LiquidGlassButton>
    </div>

    <!-- Toolbar（§7.2） -->
    <div class="page-toolbar device-list-control-bar list-page-controls">
      <div class="input-group list-toolbar-query" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input
          v-model="search"
          type="search"
          class="form-control"
          :placeholder="$t('driver_search_placeholder', '搜索驱动名称或编码...')"
          :aria-label="$t('driver_search_placeholder', '搜索驱动名称或编码...')"
        >
      </div>
      <div class="list-toolbar-filters" v-if="protocols.length > 0">
        <select v-model="filterProtocol" class="form-select form-select-sm" style="min-width: 180px;">
          <option value="">{{ $t('driver_all_protocols', '全部协议插件') }}</option>
          <option v-for="p in protocols" :key="p.name" :value="p.name">{{ getPluginTitle(p.name) }}</option>
        </select>
      </div>
      <CompactListMetrics v-model="metricFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="$t('driver_stat_total', '驱动统计')" />
    </div>

    <!-- Table Glass Card（单屏填充 + 内部纵向滚动） -->
    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4 d-none d-md-table-cell">{{ $t('driver_code', '驱动编码') }}</th>
                <th>{{ $t('driver_name', '驱动名称') }}</th>
                <th v-if="showProjectColumn" class="d-none d-lg-table-cell">{{ $t('project_name', '所属项目') }}</th>
                <th class="d-none d-lg-table-cell">{{ $t('driver_protocol_plugin', '协议插件') }}</th>
                <th class="d-none d-xl-table-cell">{{ $t('dev_created', '创建时间') }}</th>
                <th class="text-end pe-4">{{ $t('actions', '操作') }}</th>
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
              <!-- 空状态：无驱动 -->
              <tr v-else-if="drivers.length === 0">
                <td :colspan="showProjectColumn ? 6 : 5" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-cpu"></i>
                    <p>{{ $t('driver_no_data', '暂无驱动数据') }}</p>
                    <button v-permission="'product:create'" type="button" class="btn btn-primary" @click="openCreateModal">
                      {{ $t('driver_create', '新建驱动') }}
                    </button>
                  </div>
                </td>
              </tr>
              <!-- 空状态：搜索无结果 -->
              <tr v-else-if="filteredDrivers.length === 0">
                <td :colspan="showProjectColumn ? 6 : 5" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-search"></i>
                    <p>{{ $t('prod_search_no_match', '未找到匹配结果') }}</p>
                  </div>
                </td>
              </tr>
              <!-- 数据行 -->
              <tr v-for="driver in filteredDrivers" :key="driver.code" class="driver-row" @click="openEditModal(driver)">
                <td class="ps-4 d-none d-md-table-cell font-mono small text-truncate" style="max-width: 200px;">
                  {{ formatNamedReference(driver.name, driver.code) }}
                </td>
                <td class="fw-bold text-truncate" style="max-width: 200px;">
                  <span>{{ driver.name }}</span>
                  <span v-if="isSystemDriver(driver)" class="badge text-bg-secondary ms-2" style="font-size: 0.75rem;">
                    {{ $t('driver_system_builtin', '系统内置') }}
                  </span>
                </td>
                <td v-if="showProjectColumn" class="d-none d-lg-table-cell">
                  <span class="badge text-bg-light border">{{ driver.project_name || '-' }}</span>
                </td>
                <td class="d-none d-lg-table-cell">
                  <span class="dash-pill" :class="driver.protocol_name === 'script' ? 'dash-pill--success' : 'dash-pill--primary'">
                    <span class="dash-pill-dot"></span>
                    {{ getPluginTitle(driver.protocol_name) }}
                  </span>
                </td>
                <td class="d-none d-xl-table-cell">
                  <div class="font-mono small text-secondary">{{ formatDateTime(driver.CreatedAt) }}</div>
                </td>
                <td class="text-end pe-4" @click.stop>
                  <div class="table-actions">
                    <button 
                      class="table-action-btn table-action-btn--primary" 
                      :title="isSystemDriver(driver) ? $t('driver_view', '查看') : $t('edit', '编辑')" 
                      @click="openEditModal(driver)" 
                      v-permission="'product:list'"
                    >
                      <i :class="isSystemDriver(driver) ? 'bi bi-eye' : 'bi bi-pencil'"></i>
                    </button>
                    <button 
                      v-if="!isSystemDriver(driver)"
                      class="table-action-btn table-action-btn--danger" 
                      :title="$t('delete', '删除')" 
                      @click="deleteDriver(driver.code)" 
                      v-permission="'product:delete'"
                    >
                      <i class="bi bi-trash"></i>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      
      <ListPagination
        :page="page"
        :page-size="pageSize"
        :total="total"
        :disabled="loading"
        id-prefix="device-drivers"
        @update:page="changePage"
        @update:page-size="changePageSize"
      />
    </div>

    <Teleport to="body">
      <!-- Create/Edit/View Modal -->
      <div v-if="showModal" class="modal fade show d-block modal-overlay-theme">
        <div class="modal-dialog modal-lg modal-dialog-centered">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title">
                {{ isSystemDriver(currentDriver) ? $t('driver_view', '查看驱动') : (isEditing ? $t('driver_edit', '编辑驱动') : $t('driver_create', '新建驱动')) }}
                <span v-if="isSystemDriver(currentDriver)" class="badge text-bg-secondary ms-2" style="font-size: 0.8rem;">
                  {{ $t('driver_system_builtin', '系统内置') }}
                </span>
              </h5>
              <button type="button" class="btn-close" @click="closeModal"></button>
            </div>
            <div class="modal-body">
              <div v-if="isSystemDriver(currentDriver)" class="alert alert-info d-flex align-items-center py-2 px-3 mb-3 small">
                <i class="bi bi-info-circle me-2 flex-shrink-0"></i>
                <span>{{ $t('driver_system_readonly_hint', '系统内置驱动由平台自动生成与维护，禁止编辑。') }}</span>
              </div>

              <div class="mb-3">
                <label class="form-label">{{ $t('driver_code', '驱动编码') }}</label>
                <input v-model="currentDriver.code" type="text" class="form-control" :disabled="isEditing || isSystemDriver(currentDriver)">
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('driver_name', '驱动名称') }}</label>
                <input v-model="currentDriver.name" type="text" class="form-control" :disabled="isSystemDriver(currentDriver)">
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('driver_protocol_plugin', '协议插件') }}</label>
                <select v-model="currentDriver.protocol_name" class="form-select" :disabled="isEditing || isSystemDriver(currentDriver)" @change="fetchSchema(currentDriver.protocol_name)">
                  <option value="">{{ $t('driver_select_protocol_plugin', '请选择协议插件') }}</option>
                  <option v-for="p in protocols" :key="p.name" :value="p.name">{{ getPluginTitle(p.name) }}</option>
                </select>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('description', '描述') }}</label>
                <textarea v-model="currentDriver.description" class="form-control" rows="2" :disabled="isSystemDriver(currentDriver)"></textarea>
              </div>

              <!-- 脚本驱动配置 -->
              <div class="mb-3" v-if="isScriptProtocol">
                 <ScriptProductConfig
                        v-model="currentDriver.config"
                        :product-code="currentDriver.code"
                    />
              </div>

              <!-- 协议插件驱动参数定义列表展示 -->
              <div class="mb-3" v-else-if="currentSchema && schemaParams.length > 0">
                 <label class="form-label fw-bold mb-2">{{ $t('driver_params_definition', '驱动参数定义') }}</label>
                 <div class="table-responsive border rounded bg-white">
                   <table class="table table-hover align-middle mb-0">
                     <thead class="table-light">
                       <tr>
                         <th style="width: 25%">{{ $t('script_param_key', '参数标识') }}</th>
                         <th style="width: 30%">{{ $t('script_param_name', '显示名称') }}</th>
                         <th style="width: 20%">{{ $t('script_param_type', '类型') }}</th>
                         <th style="width: 25%">{{ $t('description', '说明') }}</th>
                       </tr>
                     </thead>
                     <tbody>
                       <tr v-for="param in schemaParams" :key="param.key">
                         <td>
                           <span class="font-mono fw-bold text-primary">{{ param.key }}</span>
                         </td>
                         <td>
                           <span>{{ param.name }}</span>
                         </td>
                         <td>
                           <span class="badge text-bg-light border font-mono">{{ param.typeLabel }}</span>
                         </td>
                         <td class="text-secondary small">
                           {{ param.description || '-' }}
                         </td>
                       </tr>
                     </tbody>
                   </table>
                 </div>
                 <div class="form-text mt-1 text-muted">
                   <i class="bi bi-info-circle me-1"></i>{{ $t('driver_params_hint', '驱动定义了该协议在设备实例化时需要配置的参数项。') }}
                 </div>
              </div>

              <!-- 协议无配置项时的提示 -->
              <div class="mb-3" v-else-if="currentSchema && schemaParams.length === 0">
                 <label class="form-label fw-bold mb-2">{{ $t('driver_params_definition', '驱动参数定义') }}</label>
                 <div class="text-muted p-3 border rounded bg-light small">
                   <i class="bi bi-check-circle me-1"></i>{{ $t('driver_no_params', '该协议无需额外配置驱动参数。') }}
                 </div>
              </div>
            </div>
            <div class="modal-footer">
              <LiquidGlassButton variant="secondary" @click="closeModal">
                {{ isSystemDriver(currentDriver) ? $t('close', '关闭') : $t('cancel', '取消') }}
              </LiquidGlassButton>
              <LiquidGlassButton v-if="!isSystemDriver(currentDriver)" variant="primary" @click="saveDriver">
                {{ $t('save', '保存') }}
              </LiquidGlassButton>
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue';
import axios from 'axios';
import { useToast } from '../composables/useToast';
import { useI18n } from 'vue-i18n';
import ScriptProductConfig from '../components/script/ScriptProductConfig.vue';
import { useAuthStore } from '../stores/auth';
import { isSingleProjectMode } from '../utils/systemMode.js';
import { formatNamedReference } from '../utils/entityDisplay.js';
import ListPagination from '../components/ListPagination.vue';
import { formatDateTime } from '../utils/dateTime.js';
import CompactListMetrics from '../components/CompactListMetrics.vue';

const { t, locale } = useI18n();
const { showToast } = useToast();
const authStore = useAuthStore();

const drivers = ref([]);
const protocols = ref([]);
const loading = ref(false);
const total = ref(0);
const page = ref(1);
const pageSize = ref(10);
const search = ref('');
const filterProtocol = ref('');

const showModal = ref(false);
const isEditing = ref(false);
const currentDriver = ref({});
const currentSchema = ref(null);

const isSystemDriver = (driver) => {
  if (!driver) return false;
  return Boolean(
    driver.is_system ||
    driver.code === 'bacnet_default_driver' ||
    driver.code === 'modbus_tcp_default_driver' ||
    (Number(driver.tenant_id || 0) === 0 && Number(driver.project_id || 0) === 0)
  );
};

const isScriptProtocol = computed(() => {
  return (currentDriver.value.protocol_name || '').toLowerCase() === 'script';
});

const scriptDriverCount = computed(() => {
  return drivers.value.filter(d => (d.protocol_name || '').toLowerCase() === 'script').length;
});

const pluginDriverCount = computed(() => {
  return drivers.value.filter(d => (d.protocol_name || '').toLowerCase() !== 'script').length;
});

const metricCards = computed(() => [
  { key: 'total', label: t('driver_stat_total', '驱动总数'), value: total.value, icon: 'bi-cpu-fill', tone: 'brand' },
  { key: 'script', label: t('driver_stat_script', '脚本驱动'), value: scriptDriverCount.value, icon: 'bi-code-slash', tone: 'success' },
  { key: 'plugin', label: t('driver_stat_plugin', '插件驱动'), value: pluginDriverCount.value, icon: 'bi-puzzle-fill', tone: 'neutral' }
]);

const metricFilters = ref([]);
const toggleMetricFilter = (key) => {
  metricFilters.value = metricFilters.value.includes(key)
    ? metricFilters.value.filter((item) => item !== key)
    : [...metricFilters.value, key];
};

const filteredDrivers = computed(() => {
  let list = drivers.value || [];
  if (filterProtocol.value) {
    list = list.filter(d => d.protocol_name === filterProtocol.value);
  }
  if (metricFilters.value.length) {
    list = list.filter((driver) =>
      (metricFilters.value.includes('script') && (driver.protocol_name || '').toLowerCase() === 'script') ||
      (metricFilters.value.includes('plugin') && (driver.protocol_name || '').toLowerCase() !== 'script')
    );
  }
  const q = search.value.trim().toLowerCase();
  if (q) {
    list = list.filter(d => (d.name && d.name.toLowerCase().includes(q)) || (d.code && d.code.toLowerCase().includes(q)));
  }
  return list;
});

const schemaParams = computed(() => {
  if (!currentSchema.value || !currentSchema.value.properties) return [];
  const props = currentSchema.value.properties;
  const isZh = locale.value === 'zh';
  return Object.keys(props).map(key => {
    const item = props[key] || {};
    let name = item.title;
    if (isZh && item.title_zh) {
      name = item.title_zh;
    } else if (!isZh && item.title_en) {
      name = item.title_en;
    } else if (!name) {
      name = key;
    }

    let desc = item.description;
    if (isZh && item.description_zh) {
      desc = item.description_zh;
    } else if (!isZh && item.description_en) {
      desc = item.description_en;
    }

    let typeLabel = item.type || 'string';
    if (typeLabel === 'string') {
      typeLabel = isZh ? '字符串' : 'string';
    } else if (typeLabel === 'number' || typeLabel === 'integer' || typeLabel === 'int') {
      typeLabel = isZh ? '数字' : 'number';
    } else if (typeLabel === 'boolean' || typeLabel === 'bool') {
      typeLabel = isZh ? '布尔' : 'boolean';
    } else if (typeLabel === 'array') {
      typeLabel = isZh ? '数组' : 'array';
    } else if (typeLabel === 'object') {
      typeLabel = isZh ? '对象' : 'object';
    }

    return {
      key,
      name,
      type: item.type || 'string',
      typeLabel,
      description: desc
    };
  });
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
    console.error('Failed to load protocols', err);
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

const changePageSize = (size) => {
  pageSize.value = Number(size) || 10;
  page.value = 1;
  loadDrivers();
};

onMounted(async () => {
  loading.value = true;
  await loadProtocols();
  await loadDrivers();
});
</script>

<style scoped>
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

.dash-pill--primary {
  color: #1d4ed8 !important;
  background: rgba(59, 130, 246, 0.12);
  border: 1px solid rgba(59, 130, 246, 0.28);
}

[data-bs-theme="dark"] .dash-pill--primary {
  color: #60a5fa !important;
  background: rgba(96, 165, 250, 0.15);
  border: 1px solid rgba(96, 165, 250, 0.35);
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

.driver-row {
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
