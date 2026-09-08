<template>
  <div class="role-management-page page-fixed-height">
    <div class="page-header list-page-header">
      <div>
        <h1>{{ $t('role_management') }}</h1>
        <p class="page-subtitle">{{ $t('role_management_subtitle', '定义角色权限模型与功能访问控制策略') }}</p>
      </div>
      <LiquidGlassButton
        variant="primary"
        icon="bi bi-shield-lock"
        @click="openCreateModal"
        v-permission="'role:create'"
      >
        {{ $t('role_add') }}
      </LiquidGlassButton>
    </div>

    <div class="page-toolbar device-list-control-bar list-page-controls">
      <select v-if="!isGatewayMode" v-model="filterProjectId" class="form-select form-select-sm list-toolbar-filters" style="min-width: 200px; max-width: 280px;">
        <option :value="-1">{{ $t('role_all', '全部角色') }}</option>
        <option :value="0">{{ $t('role_tenant_public', '租户级公共角色') }}</option>
        <option v-for="p in projects" :key="p.ID" :value="p.ID">
          {{ $t('role_project_exclusive', '项目专属') }}: {{ p.name }}
        </option>
      </select>
      <CompactListMetrics v-model="metricFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="$t('stat_total', '角色统计')" />
    </div>

    <!-- Roles Table -->
    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4">{{ $t('role_code') }}</th>
                <th>{{ $t('role_name') }}</th>
                <th>{{ $t('role_description') }}</th>
                <th v-if="!isGatewayMode">{{ $t('role_scope', '作用域') }}</th>
                <th>{{ $t('user_created_at') }}</th>
                <th class="text-end pe-4">{{ $t('role_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <!-- 骨架屏 -->
              <tr v-if="loading" v-for="n in 5" :key="'sk-' + n">
                <td><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 120px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 160px;"></div></td>
                <td v-if="!isGatewayMode"><div class="skeleton" style="height: 20px; width: 80px; border-radius: 999px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 130px;"></div></td>
                <td class="text-end pe-4"><div class="skeleton ms-auto" style="height: 28px; width: 80px;"></div></td>
              </tr>
              <!-- 空状态 -->
              <tr v-else-if="filteredRoles.length === 0">
                <td :colspan="isGatewayMode ? 5 : 6" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-shield-lock"></i>
                    <p>{{ $t('role_no_data', '暂无角色数据') }}</p>
                    <LiquidGlassButton variant="primary" icon="bi bi-plus-lg" @click="openRoleModal" v-permission="'role:create'">
                      {{ $t('role_add', '添加角色') }}
                    </LiquidGlassButton>
                  </div>
                </td>
              </tr>
              <tr v-for="r in filteredRoles" :key="r.ID" v-else>
                <td><strong>{{ r.code }}</strong></td>
                <td>
                  {{ r.name }}
                  <span v-if="r.is_builtin" class="spec-badge spec-badge--neutral ms-1">{{ $t('role_system_builtin', '系统内置') }}</span>
                </td>
                <td>{{ r.description }}</td>
                <td v-if="!isGatewayMode">
                  <template v-if="!isGatewayMode">
                    <span v-if="r.project_id === 0 && !r.is_inherited" class="spec-badge spec-badge--primary">
                      {{ $t('role_tenant_level', '租户级') }}
                    </span>
                    <span v-else-if="r.project_id === 0 && r.is_inherited" class="spec-badge spec-badge--info">
                      {{ $t('role_project_level', '项目级') }}
                    </span>
                    <span v-else class="spec-badge spec-badge--warning">
                      {{ $t('role_project_exclusive', '项目专属') }} ({{ projectMap[r.project_id] || 'ID: ' + r.project_id }})
                    </span>
                  </template>
                </td>

                <td>{{ formatDateTime(r.CreatedAt) }}</td>
                <td class="text-end pe-4">
                  <div class="table-actions">
                    <button class="table-action-btn" @click="openDetailsModal(r)" :title="$t('common_view_details', '查看详情')">
                      <i class="bi bi-eye"></i>
                    </button>
                    <button class="table-action-btn" :class="r.has_permissions ? 'table-action-btn--info' : ''" @click="openPermModal(r)" :disabled="r.is_builtin" :title="$t('role_config_perm', '配置权限')" v-permission="'role:edit'">
                      <i class="bi bi-shield-check"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--primary" @click="openEditModal(r)" :disabled="r.is_builtin || isRoleReadOnly(r)" :title="$t('role_edit', '编辑')" v-permission="'role:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--danger" @click="deleteRole(r)" :disabled="r.is_builtin || isRoleReadOnly(r)" :title="$t('role_delete', '删除')" v-permission="'role:delete'">
                      <i class="bi bi-trash"></i>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <!-- Role Modal -->
      <div class="modal fade" id="roleModal" tabindex="-1" ref="roleModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('role_edit') : $t('role_add') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveRole">
              <div class="mb-3">
                <label class="form-label">{{ $t('role_code') }}</label>
                <input v-model="form.code" type="text" class="form-control" :disabled="isEditing" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('role_name') }}</label>
                <input v-model="form.name" type="text" class="form-control" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('role_description') }}</label>
                <textarea v-model="form.description" class="form-control" rows="2"></textarea>
              </div>
              

              <!-- 租户管理员选择角色范围 -->
              <div class="mb-3" v-if="!isGatewayMode && isTenantAdmin && !isProjectAdmin && !form.id">
                <label class="form-label">{{ $t('role_scope', '角色范围') }}</label>
                <select v-model="form.is_inherited" class="form-select" :disabled="isEditing">
                  <option :value="false">{{ $t('role_tenant_level', '租户级') }}</option>
                  <option :value="true">{{ $t('role_project_level', '项目级') }}</option>
                </select>
              </div>
              <div class="mb-3" v-else>
                <label class="form-label">{{ $t('role_scope_label', '角色作用域') }}</label>
                <input type="text" class="form-control" :value="$t('role_current_project', '当前项目')" disabled>
              </div>

            </form>
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ $t('role_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveRole">{{ $t('role_save') }}</LiquidGlassButton>
          </div>
        </div>
      </div>
    </div>

    <!-- Role Details Modal -->
    <div class="modal fade" id="roleDetailsModal" tabindex="-1" ref="roleDetailsModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('role_details', '角色详情') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body p-0">
            <div v-if="currentRoleDetails" class="role-details-card">
              <div class="p-4 text-center border-bottom role-details-header">
                <div class="display-4 text-primary mb-2">
                  <i class="bi bi-shield-lock-fill"></i>
                </div>
                <h5 class="mb-1">{{ currentRoleDetails.name }}</h5>
                <p class="text-muted mb-0">Code: {{ currentRoleDetails.code }}</p>
              </div>
              <div class="p-4">
                <div class="row g-3">
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('role_description') }}</label>
                    <div class="fw-medium">{{ currentRoleDetails.description || $t('common_none', '无') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('role_scope', '作用域') }}</label>
                    <div>
                      <span v-if="currentRoleDetails.project_id === 0 && !currentRoleDetails.is_inherited" class="spec-badge spec-badge--primary">{{ $t('role_tenant_level', '租户级') }}</span>
                      <span v-else-if="currentRoleDetails.project_id === 0 && currentRoleDetails.is_inherited" class="spec-badge spec-badge--info">{{ $t('role_project_level', '项目级') }}</span>
                      <span v-else class="spec-badge spec-badge--warning">{{ $t('role_project_exclusive', '项目专属') }} ({{ projectMap[currentRoleDetails.project_id] || currentRoleDetails.project_id }})</span>
                      <span v-if="currentRoleDetails.is_builtin" class="spec-badge spec-badge--neutral ms-1">{{ $t('role_system_builtin', '系统内置') }}</span>
                    </div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('role_data_permission', '数据权限') }}</label>
                    <div>
                      <span class="spec-badge spec-badge--info">
                        {{ currentRoleDetails.device_tags ? currentRoleDetails.device_tags : (currentRoleDetails.data_scope === 1 ? 'All' : (currentRoleDetails.data_scope === 2 ? 'Project' : 'Personal')) }}
                      </span>
                    </div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('user_created_at') }}</label>
                    <div class="fw-medium">{{ formatDateTime(currentRoleDetails.CreatedAt) }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="danger" data-bs-dismiss="modal">{{ $t('common_close', '关闭') }}</LiquidGlassButton>
          </div>
        </div>
      </div>
    </div>

      <!-- Permissions Modal -->
      <RolePermissions ref="permModalRef" @saved="loadRoles" />
    </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import { Modal } from 'bootstrap'
import { useI18n } from 'vue-i18n'
import RolePermissions from './RolePermissions.vue'
import { useAuthStore } from '../stores/auth'
import { isInheritedRoleReadOnlyForUser } from '../utils/authIdentity'
import { isSingleProjectMode } from '../utils/systemMode'
import { formatDateTime } from '../utils/dateTime.js'
import CompactListMetrics from '../components/CompactListMetrics.vue'

const { t } = useI18n()
const authStore = useAuthStore()
const isTenantAdmin = computed(() => authStore.user?.is_tenant_admin === true)

const isGatewayMode = computed(() => {
  return isSingleProjectMode(localStorage.getItem('system_mode') || '');
});

const roles = ref([])
const projects = ref([])

const builtinRoleCount = computed(() => roles.value.filter(r => r.is_builtin).length)
const customRoleCount = computed(() => roles.value.filter(r => !r.is_builtin).length)
const metricCards = computed(() => [
  { key: 'total', label: t('stat_total', '总角色数'), value: roles.value.length, icon: 'bi-shield-lock-fill', tone: 'brand' },
  { key: 'builtin', label: t('stat_builtin_roles', '系统内置'), value: builtinRoleCount.value, icon: 'bi-award-fill', tone: 'success' },
  { key: 'custom', label: t('stat_custom_roles', '自定义角色'), value: customRoleCount.value, icon: 'bi-sliders', tone: 'neutral' }
])
const metricFilters = ref([])
const toggleMetricFilter = (key) => {
  metricFilters.value = metricFilters.value.includes(key)
    ? metricFilters.value.filter((item) => item !== key)
    : [...metricFilters.value, key]
}

const filterProjectId = ref(-1)
const loading = ref(false)
const roleModalRef = ref(null)
let roleModal = null
const permModalRef = ref(null)
const roleDetailsModalRef = ref(null)
let roleDetailsModal = null
const currentRoleDetails = ref(null)

const isEditing = ref(false)
const roleScope = ref('tenant')
const form = ref({
  id: 0,
  code: '',
  name: '',
  description: '',
  data_scope: 5,
    project_id: 0,
  is_inherited: false
})

const loadRoles = async () => {
  loading.value = true
  try {
    const res = await axios.get('/api/roles')
    if (res.data.code === 0) {
      roles.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load roles:", error)
  } finally {
    loading.value = false
  }
}

const loadProjects = async () => {
  try {
    const res = await axios.get('/api/auth/projects')
    if (res.data.code === 0) {
      projects.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load projects:", error)
  }
}

const projectMap = computed(() => {
  const map = {}
  projects.value.forEach(p => {
    map[p.ID] = p.name
  })
  return map
})

const filteredRoles = computed(() => {
  let result = roles.value
  if (filterProjectId.value === -1) {
    result = roles.value
  } else if (filterProjectId.value === 0) {
    result = roles.value.filter(r => r.project_id === 0 && r.is_inherited !== true)
  } else {
    result = roles.value.filter(r => r.project_id === filterProjectId.value || (r.project_id === 0 && r.is_inherited === true))
  }
  if (!metricFilters.value.length) return result
  return result.filter((role) =>
    (metricFilters.value.includes('builtin') && role.is_builtin)
    || (metricFilters.value.includes('custom') && !role.is_builtin)
  )
})

const isRoleReadOnly = (item) => isInheritedRoleReadOnlyForUser(authStore.user, item)

onMounted(() => {
  roleModal = new Modal(roleModalRef.value)
  roleDetailsModal = new Modal(roleDetailsModalRef.value)
  loadRoles()
  loadProjects()
})

const openCreateModal = () => {
  isEditing.value = false
  roleScope.value = 'tenant'
  form.value = {
    id: 0,
    code: '',
    name: '',
    description: '',
    data_scope: 5,
        project_id: 0,
    is_inherited: false
  }
  roleModal.show()
}

const openEditModal = (item) => {
  isEditing.value = true
  roleScope.value = item.project_id > 0 ? 'project' : 'tenant'
  form.value = {
    id: item.ID,
    code: item.code,
    name: item.name,
    description: item.description,
    data_scope: item.data_scope,
        project_id: item.project_id || 0,
    is_inherited: item.is_inherited || false
  }
  roleModal.show()
}

const openPermModal = (item) => {
  if (permModalRef.value) {
    permModalRef.value.open(item)
  }
}

const openDetailsModal = (item) => {
  currentRoleDetails.value = item
  roleDetailsModal.show()
}

const saveRole = async () => {
  // 如果是项目管理员，强制绑定到当前项目
  if (!isTenantAdmin.value) {
    const pId = Number(localStorage.getItem('current_project_id') || 0)
    if (pId > 0) {
      form.value.project_id = pId
    }
    form.value.is_inherited = false
  } else {
    // Tenant Admins always create templates (project_id = 0)
    form.value.project_id = 0
  }

  try {
    let res
    if (isEditing.value) {
      res = await axios.put(`/api/roles/${form.value.id}`, form.value)
    } else {
      res = await axios.post('/api/roles', form.value)
    }
    
    if (res.data.code === 0) {
      roleModal.hide()
      loadRoles()
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    console.error("Failed to save role:", error)
    if (error.response && error.response.data && error.response.data.message) {
      alert(t('common_save_failed', '保存失败: ') + error.response.data.message)
    } else {
      alert(t('common_save_failed', '保存失败'))
    }
  }
}

const deleteRole = async (item) => {
  if (confirm(t('role_delete_confirm', { name: item.name }))) {
    try {
      const res = await axios.delete(`/api/roles/${item.ID}`)
      if (res.data.code === 0) {
        loadRoles()
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert("Failed to delete role")
    }
  }
}
</script>

<style scoped>
.role-details-card {
  background: var(--bg-surface);
  color: var(--text-main);
}
.role-details-header {
  background: var(--bg-drawer-header);
  border-color: var(--border-color) !important;
}
</style>
