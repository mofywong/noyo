<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('role_management') }}</h2>
      <div class="d-flex align-items-center gap-2">
        <select v-model="filterProjectId" class="form-select form-select-sm" style="width: 220px;">
          <option :value="-1">{{ $t('role_all', '全部角色') }}</option>
          <option :value="0">{{ $t('role_tenant_public', '租户级公共角色') }}</option>
          <option v-for="p in projects" :key="p.ID" :value="p.ID">
            {{ $t('role_project_exclusive', '项目专属') }}: {{ p.name }}
          </option>
        </select>
        <button class="btn btn-primary btn-sm ms-2" @click="openCreateModal" v-permission="'role:create'">
          <i class="bi bi-shield-lock me-1"></i> {{ $t('role_add') }}
        </button>
      </div>
    </div>

    <!-- Roles Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ $t('role_code') }}</th>
                <th>{{ $t('role_name') }}</th>
                <th>{{ $t('role_description') }}</th>
                <th>{{ $t('role_scope', '作用域') }}</th>
                <th>{{ $t('role_status') }}</th>
                <th>{{ $t('user_created_at') }}</th>
                <th class="text-end">{{ $t('role_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="8" class="text-center py-4">
                  <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Loading...</span>
                  </div>
                </td>
              </tr>
              <tr v-else-if="filteredRoles.length === 0">
                <td colspan="8" class="text-center py-4 text-muted">{{ $t('role_no_data') }}</td>
              </tr>
              <tr v-for="r in filteredRoles" :key="r.ID" v-else>
                <td><strong>{{ r.code }}</strong></td>
                <td>{{ r.name }}</td>
                <td>{{ r.description }}</td>
                <td>
                  <span v-if="r.project_id === 0 && !r.is_inherited" class="badge text-bg-success">
                    {{ $t('role_tenant_level', '租户级') }}
                  </span>
                  <span v-else-if="r.project_id === 0 && r.is_inherited" class="badge text-bg-info">
                    {{ $t('role_project_level', '项目级') }}
                  </span>
                  <span v-else class="badge text-bg-primary">
                    {{ $t('role_project_exclusive', '项目专属') }} ({{ projectMap[r.project_id] || 'ID: ' + r.project_id }})
                  </span>
                  <span v-if="r.is_builtin" class="badge text-bg-secondary ms-1">{{ $t('role_system_builtin', '系统内置') }}</span>
                </td>
                <td>
                  <span class="badge" :class="r.status === 1 ? 'text-bg-success' : 'text-bg-danger'">
                    {{ r.status === 1 ? $t('user_active') : $t('user_disabled') }}
                  </span>
                </td>
                <td>{{ new Date(r.CreatedAt).toLocaleString() }}</td>
                <td class="text-end">
                  <button class="btn btn-sm btn-outline-success me-2" @click="openDetailsModal(r)" :title="$t('common_view_details', '查看详情')">
                    <i class="bi bi-eye"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-info me-2" @click="openPermModal(r)" :disabled="r.is_builtin" :title="$t('role_config_perm', '配置权限')" v-permission="'role:edit'">
                    <i class="bi bi-shield-check"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-primary me-2" @click="openEditModal(r)" :disabled="r.is_builtin || isRoleReadOnly(r)" :title="$t('role_edit', '编辑')" v-permission="'role:edit'">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-danger" @click="deleteRole(r)" :disabled="r.is_builtin || isRoleReadOnly(r)" :title="$t('role_delete', '删除')" v-permission="'role:delete'">
                    <i class="bi bi-trash"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

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
              

              <!-- 仅租户管理员可选择角色归属项目 -->
              <div class="mb-3" v-if="isTenantAdmin">
                <label class="form-label">{{ $t('role_scope_label', '角色作用域') }}</label>
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
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('role_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveRole">{{ $t('role_save') }}</button>
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
            <div v-if="currentRoleDetails" class="bg-light">
              <div class="p-4 text-center border-bottom bg-white">
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
                      <span v-if="currentRoleDetails.project_id === 0 && !currentRoleDetails.is_inherited" class="badge text-bg-success">{{ $t('role_tenant_level', '租户级') }}</span>
                      <span v-else-if="currentRoleDetails.project_id === 0 && currentRoleDetails.is_inherited" class="badge text-bg-info">{{ $t('role_project_level', '项目级') }}</span>
                      <span v-else class="badge text-bg-primary">{{ $t('role_project_exclusive', '项目专属') }} ({{ projectMap[currentRoleDetails.project_id] || currentRoleDetails.project_id }})</span>
                      <span v-if="currentRoleDetails.is_builtin" class="badge text-bg-secondary ms-1">{{ $t('role_system_builtin', '系统内置') }}</span>
                    </div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('role_status') }}</label>
                    <div>
                      <span class="badge" :class="currentRoleDetails.status === 1 ? 'text-bg-success' : 'text-bg-danger'">
                        {{ currentRoleDetails.status === 1 ? $t('user_active') : $t('user_disabled') }}
                      </span>
                    </div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('role_data_permission', '数据权限') }}</label>
                    <div>
                      <span class="badge text-bg-info">
                        {{ currentRoleDetails.device_tags ? currentRoleDetails.device_tags : (currentRoleDetails.data_scope === 1 ? 'All' : (currentRoleDetails.data_scope === 2 ? 'Project' : 'Personal')) }}
                      </span>
                    </div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('user_created_at') }}</label>
                    <div class="fw-medium">{{ new Date(currentRoleDetails.CreatedAt).toLocaleString() }}</div>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('common_close', '关闭') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Permissions Modal -->
    <RolePermissions ref="permModalRef" @saved="loadRoles" />
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

const { t } = useI18n()
const authStore = useAuthStore()
const isTenantAdmin = computed(() => authStore.user?.is_tenant_admin === true)

const roles = ref([])
const projects = ref([])
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
  status: 1,
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
  if (filterProjectId.value === -1) {
    return roles.value
  }
  return roles.value.filter(r => r.project_id === filterProjectId.value)
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
    status: 1,
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
    status: item.status,
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
