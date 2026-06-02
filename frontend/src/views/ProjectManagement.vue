<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('project_management') }}</h2>
      <div class="d-flex align-items-center gap-2">
        <div class="input-group input-group-sm" style="width: 250px;">
          <input type="text" class="form-control" :placeholder="$t('project_search_placeholder', '搜索项目名称或编码...')" v-model="filterKeyword" @keyup.enter="loadProjects">
          <button class="btn btn-outline-secondary" @click="loadProjects"><i class="bi bi-search"></i></button>
        </div>
        <button class="btn btn-primary btn-sm ms-2" @click="openCreateModal" v-permission="'project:create'">
          <i class="bi bi-folder-plus me-1"></i> {{ $t('project_add') }}
        </button>
      </div>
    </div>

    <!-- Projects Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ $t('project_code') }}</th>
                <th>{{ $t('project_name') }}</th>
                <th>{{ $t('project_admin', '管理员') }}</th>
                <th>{{ $t('project_description') }}</th>
                                <th>{{ $t('user_created_at') }}</th>
                <th class="text-end">{{ $t('project_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="7" class="text-center py-4">
                  <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Loading...</span>
                  </div>
                </td>
              </tr>
              <tr v-else-if="projects.length === 0">
                <td colspan="7" class="text-center py-4 text-muted">{{ $t('project_no_data') }}</td>
              </tr>
              <tr v-for="p in projects" :key="p.ID" v-else>
                <td><strong>{{ p.code }}</strong></td>
                <td>{{ p.name }}</td>
                <td>{{ p.admins || $t('common_none', '暂无') }}</td>
                <td>{{ p.description }}</td>

                <td>{{ new Date(p.CreatedAt).toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-') }}</td>
                <td class="text-end">
                  <button class="btn btn-sm btn-outline-success me-2" @click="openDetailsModal(p)" :title="$t('common_view_details', '查看详情')">
                    <i class="bi bi-eye"></i>
                  </button>
                  <button class="btn btn-sm me-2" :class="(p.permission_ids && p.permission_ids.length > 0) ? 'btn-outline-info' : 'btn-outline-secondary'" @click="openPermissionModal(p)" :title="$t('project_permission_config', '权限配置')" v-permission="'project:edit'">
                    <i class="bi bi-shield-check"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-primary me-2" @click="openEditModal(p)" :title="$t('project_edit', '编辑')" v-permission="'project:edit'">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-danger" @click="deleteProject(p)" :disabled="p.code === 'default'" :title="$t('project_delete', '删除')" v-permission="'project:delete'">
                    <i class="bi bi-trash"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

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
            <PermissionDualMode
              :allPermissions="projectPermissionOptions"
              v-model="form.permission_ids"
              :title="$t('project_permission_limit', '项目最大权限集')"
            />
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('common_cancel', '取消') }}</button>
            <button type="button" class="btn btn-primary" @click="saveProjectPermission">{{ $t('project_save', '保存') }}</button>
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
            <div v-if="currentProjectDetails" class="bg-light">
              <div class="p-4 text-center border-bottom bg-white">
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
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('project_status', '状态') }}</label>
                    <div><span class="badge" :class="currentProjectDetails.status === 1 ? 'text-bg-success' : 'text-bg-danger'">{{ currentProjectDetails.status === 1 ? $t('user_enabled', '启用') : $t('user_disabled', '禁用') }}</span></div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('project_description', '项目描述') }}</label>
                    <div class="fw-medium">{{ currentProjectDetails.description || $t('common_none', '暂无描述') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('user_created_at', '创建时间') }}</label>
                    <div class="fw-medium">{{ new Date(currentProjectDetails.CreatedAt).toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-') }}</div>
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
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('project_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveProject">{{ $t('project_save') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import axios from 'axios'
import PermissionDualMode from '../components/PermissionDualMode.vue'
import { Modal } from 'bootstrap'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const projects = ref([])
const loading = ref(false)
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
  permission_ids: []
})

const groupedProjectPermissionOptions = computed(() => {
  const groups = {}
  projectPermissionOptions.value.forEach(permission => {
    if (!groups[permission.module]) groups[permission.module] = []
    groups[permission.module].push(permission)
  })
  return groups
})



const openPermissionModal = (item) => {
    form.value = {
      id: item.ID,
      name: item.name,
      permission_ids: item.permission_ids || []
    }
    projectPermissionModal.show()
  }

  const saveProjectPermission = async () => {
    try {
      // Use the project update endpoint with just the permission_ids, or a specific permission endpoint
      // based on original code, it was probably calling PUT /api/projects/:id
      const res = await axios.put(`/api/projects/${form.value.id}`, {
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
      alert($t('common_delete_failed', '删除失败'))
    }
  }
}
</script>
