<template>
  <div class="project-management-page page-fixed-height">
    <div class="page-header list-page-header">
      <div>
        <h1>{{ $t('project_management') }}</h1>
        <p class="page-subtitle">{{ $t('project_management_subtitle', '管理系统业务项目、管理员分配与权限模式') }}</p>
      </div>
      <LiquidGlassButton
        variant="primary"
        icon="bi bi-folder-plus"
        @click="openCreateModal"
        v-permission="'project:create'"
      >
        {{ $t('project_add') }}
      </LiquidGlassButton>
    </div>

    <!-- Toolbar -->
    <div class="page-toolbar device-list-control-bar list-page-controls">
      <div class="input-group list-toolbar-query" style="max-width: 320px;">
        <span class="input-group-text bg-transparent"><i class="bi bi-search"></i></span>
        <input type="text" class="form-control" :placeholder="$t('project_search_placeholder', '搜索项目名称或编码...')" v-model="filterKeyword" @keyup.enter="loadProjects">
      </div>
      <CompactListMetrics v-model="metricFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="$t('stat_total', '项目统计')" />
    </div>

    <!-- Projects Table -->
    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4">{{ $t('project_code') }}</th>
                <th>{{ $t('project_name') }}</th>
                <th>{{ $t('project_admin', '管理员') }}</th>
                <th>{{ $t('project_description') }}</th>
                <th>{{ $t('scope_permission_policy') }}</th>
                <th>{{ $t('user_created_at') }}</th>
                <th class="text-end pe-4">{{ $t('project_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <!-- 骨架屏 -->
              <tr v-if="loading" v-for="n in 5" :key="'sk-' + n">
                <td><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 140px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 160px;"></div></td>
                <td><div class="skeleton" style="height: 20px; width: 80px; border-radius: 999px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 130px;"></div></td>
                <td class="text-end pe-4"><div class="skeleton ms-auto" style="height: 28px; width: 120px;"></div></td>
              </tr>
              <!-- 空状态 -->
              <tr v-else-if="filteredProjects.length === 0">
                <td colspan="7" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-folder"></i>
                    <p>{{ $t('project_no_data', '暂无项目数据') }}</p>
                    <LiquidGlassButton variant="primary" icon="bi bi-plus-lg" @click="openCreateModal" v-permission="'project:create'">
                      {{ $t('project_add', '添加项目') }}
                    </LiquidGlassButton>
                  </div>
                </td>
              </tr>
              <tr v-for="p in filteredProjects" :key="p.ID" v-else>
                <td><strong>{{ p.code }}</strong></td>
                <td>{{ p.name }}</td>
                <td>{{ p.admins || $t('common_none', '暂无') }}</td>
                <td>{{ p.description }}</td>
                <td>
                  <span class="spec-badge" :class="p.permission_mode === 'custom' ? 'spec-badge--info' : (p.permission_mode === 'all' ? 'spec-badge--primary' : 'spec-badge--neutral')">
                    {{ $t(`scope_permission_mode_${p.permission_mode || 'custom'}`) }}
                  </span>
                </td>
                <td>{{ formatDateTime(p.CreatedAt) }}</td>
                <td class="text-end pe-4">
                  <div class="table-actions">
                    <button class="table-action-btn" @click="openDetailsModal(p)" :title="$t('common_view_details', '查看详情')">
                      <i class="bi bi-eye"></i>
                    </button>
                    <button class="table-action-btn" :class="p.permission_mode === 'custom' ? 'table-action-btn--info' : (p.permission_mode === 'all' ? 'table-action-btn--success' : 'table-action-btn--primary')" @click="openPermissionModal(p)" :title="$t('project_permission_config', '权限配置')" v-permission="'project:edit'">
                      <i class="bi bi-shield-check"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--primary" @click="openEditModal(p)" :title="$t('project_edit', '编辑')" v-permission="'project:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--danger" @click="deleteProject(p)" :disabled="p.code === 'default'" :title="$t('project_delete', '删除')" v-permission="'project:delete'">
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
      <!-- Project Permission Modal -->
      <div class="modal fade" id="projectPermissionModal" tabindex="-1" ref="projectPermissionModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('project_permission_config', '权限配置') }} - {{ form.name }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div v-if="projectPermissionOptions.length === 0" class="text-muted small mb-3">
              {{ $t('project_permission_empty', '暂无可分配权限') }}
            </div>
            <ScopePermissionPolicyEditor
              scope="project"
              :allPermissions="projectPermissionOptions"
              :title="$t('project_permission_limit', '项目最大权限集')"
              v-model:mode="form.permission_mode"
              v-model:permission-ids="form.permission_ids"
            />
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ $t('common_cancel', '取消') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveProjectPermission">{{ $t('project_save', '保存') }}</LiquidGlassButton>
          </div>
        </div>
      </div>
    </div>

    <!-- Project Details Modal -->
    <div class="modal fade" id="projectDetailsModal" tabindex="-1" ref="projectDetailsModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('project_details', '项目详情') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body p-0">
            <div v-if="currentProjectDetails" class="project-details-card">
              <div class="p-4 text-center border-bottom project-details-header">
                <div class="display-4 text-info mb-2">
                  <i class="bi bi-folder-fill"></i>
                </div>
                <h5 class="mb-1">{{ currentProjectDetails.name }}</h5>
                <p class="text-muted mb-0">{{ $t('project_code', '编码') }}: {{ currentProjectDetails.code }}</p>
              </div>
              <div class="p-4">
                <div class="row g-3">
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('project_admin', '管理员') }}</label>
                    <div class="fw-medium text-primary">{{ currentProjectDetails.admins || $t('common_none', '暂无') }}</div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('project_description', '项目描述') }}</label>
                    <div class="fw-medium">{{ currentProjectDetails.description || $t('common_none', '暂无描述') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('user_created_at', '创建时间') }}</label>
                    <div class="fw-medium">{{ formatDateTime(currentProjectDetails.CreatedAt) }}</div>
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

    <!-- Project Modal -->
    <div class="modal fade" id="projectModal" tabindex="-1" ref="projectModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('project_edit') : $t('project_add') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveProject">
              <div class="mb-3">
                <label class="form-label">{{ $t('project_code') }}</label>
                <input v-model="form.code" type="text" class="form-control" :disabled="isEditing" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('project_name') }}</label>
                <input v-model="form.name" type="text" class="form-control" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('project_admin') }}</label>
                <select v-model="form.admin_user_id" class="form-select" required>
                  <option value="" disabled>{{ $t('project_select_admin') }}</option>
                  <option v-for="user in users" :key="user.id" :value="user.id">
                    {{ user.display_name || user.username }}
                  </option>
                </select>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('project_description') }}</label>
                <textarea v-model="form.description" class="form-control" rows="3"></textarea>
              </div>


            </form>
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ $t('project_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveProject">{{ $t('project_save') }}</LiquidGlassButton>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import axios from 'axios'
import ScopePermissionPolicyEditor from '../components/ScopePermissionPolicyEditor.vue'
import { Modal } from 'bootstrap'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '../utils/dateTime.js'
import CompactListMetrics from '../components/CompactListMetrics.vue'

const { t } = useI18n()

const projects = ref([])
const loading = ref(false)

const assignedAdminCount = computed(() => projects.value.filter(p => p.admins && p.admins.trim() !== '').length)
const unassignedAdminCount = computed(() => projects.value.filter(p => !p.admins || p.admins.trim() === '').length)
const metricCards = computed(() => [
  { key: 'total', label: t('stat_total', '总项目数'), value: projects.value.length, icon: 'bi-folder-fill', tone: 'brand' },
  { key: 'assigned', label: t('stat_assigned_admins', '已配管理员'), value: assignedAdminCount.value, icon: 'bi-person-check-fill', tone: 'success' },
  { key: 'unassigned', label: t('stat_unassigned_admins', '未配管理员'), value: unassignedAdminCount.value, icon: 'bi-person-x', tone: 'neutral' }
])
const metricFilters = ref([])
const filteredProjects = computed(() => projects.value.filter((project) => {
  if (!metricFilters.value.length) return true
  const assigned = Boolean(project.admins?.trim())
  return (metricFilters.value.includes('assigned') && assigned)
    || (metricFilters.value.includes('unassigned') && !assigned)
}))
const toggleMetricFilter = (key) => {
  metricFilters.value = metricFilters.value.includes(key)
    ? metricFilters.value.filter((item) => item !== key)
    : [...metricFilters.value, key]
}
const projectModalRef = ref(null)
let projectModal = null
const projectDetailsModalRef = ref(null)
const projectPermissionModalRef = ref(null)
let projectPermissionModal = null

let projectDetailsModal = null

const filterKeyword = ref('')
const currentProjectDetails = ref(null)

const isEditing = ref(false)
const users = ref([])
const projectPermissionOptions = ref([])
const form = ref({
  id: 0,
  code: '',
  name: '',
  description: '',
  admin_user_id: '',
  permission_mode: 'inherit',
  permission_ids: []
})

const openPermissionModal = (item) => {
    form.value = {
      id: item.ID,
      name: item.name,
      permission_mode: item.permission_mode || 'custom',
      permission_ids: item.permission_ids || []
    }
    projectPermissionModal.show()
  }

  const saveProjectPermission = async () => {
    try {
      const res = await axios.put(`/api/projects/${form.value.id}`, {
        permission_mode: form.value.permission_mode,
        permission_ids: form.value.permission_ids
      })
      if (res.data.code === 0) {
        projectPermissionModal.hide()
        loadProjects()
      } else {
        alert(res.data.message || '保存失败')
      }
    } catch (error) {
      console.error("Failed to save project permissions:", error)
      alert('保存失败，请检查网络或联系管理员')
    }
  }

  const loadProjects = async () => {
  loading.value = true
  try {
    const res = await axios.get('/api/projects', { params: { keyword: filterKeyword.value } })
    if (res.data.code === 0) {
      projects.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load projects:", error)
  } finally {
    loading.value = false
  }
}

const loadUsers = async () => {
  try {
    const res = await axios.get('/api/users?pageSize=1000')
    if (res.data.code === 0) {
      users.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load users:", error)
  }
}

const loadProjectPermissionOptions = async () => {
  try {
    const res = await axios.get('/api/projects/permission-options')
    if (res.data.code === 0) {
      projectPermissionOptions.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load project permission options:", error)
  }
}

onMounted(() => {
  projectModal = new Modal(projectModalRef.value)
  projectDetailsModal = new Modal(projectDetailsModalRef.value)
  projectPermissionModal = new Modal(projectPermissionModalRef.value)
  loadProjects()
  loadUsers()
  loadProjectPermissionOptions()
})

const openDetailsModal = (item) => {
  currentProjectDetails.value = item
  projectDetailsModal.show()
}

const openCreateModal = () => {
  isEditing.value = false
  form.value = {
    id: 0,
    code: '',
    name: '',
    description: '',
    admin_user_id: '',
    permission_mode: 'inherit',
    permission_ids: []
  }
  projectModal.show()
}

const openEditModal = (item) => {
  isEditing.value = true
  form.value = {
    id: item.ID,
    code: item.code,
    name: item.name,
    description: item.description,
    admin_user_id: item.admin_user_id || '',
    permission_mode: item.permission_mode || 'custom',
    permission_ids: item.permission_ids || []
  }
  projectModal.show()
}

const saveProject = async () => {

  try {
    let res
    if (isEditing.value) {
      res = await axios.put(`/api/projects/${form.value.id}`, form.value)
    } else {
      res = await axios.post('/api/projects', form.value)
    }
    
    if (res.data.code === 0) {
      projectModal.hide()
      loadProjects()
      window.dispatchEvent(new Event('project-updated'))
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(t('common_save_failed', '保存失败'))
  }
}

const deleteProject = async (item) => {
  if (confirm(t('project_delete_confirm', { name: item.name }))) {
    try {
      const res = await axios.delete(`/api/projects/${item.ID}`)
      if (res.data.code === 0) {
        loadProjects()
        window.dispatchEvent(new Event('project-updated'))
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert(t('common_delete_failed', '删除失败'))
    }
  }
}
</script>

<style scoped>
.project-details-card {
  background: var(--bg-surface);
  color: var(--text-main);
}
.project-details-header {
  background: var(--bg-drawer-header);
  border-color: var(--border-color) !important;
}
</style>

