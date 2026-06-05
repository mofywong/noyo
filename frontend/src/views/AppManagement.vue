<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ t('app_management') }}</h2>
      <div>
        <button class="btn btn-outline-info me-2" @click="goToGuide">
          <i class="bi bi-book me-1"></i> {{ t('app_access_guide') }}
        </button>
        <button class="btn btn-primary" @click="openCreateModal" v-permission="'app:create'">
          <i class="bi bi-window-sidebar me-1"></i> {{ t('app_add') }}
        </button>
      </div>
    </div>

    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>AppID</th>
                <th>{{ t('app_name') }}</th>
                <th>{{ t('app_description') }}</th>
                <th>{{ t('app_rate_limit') }}</th>
                <th>{{ t('app_status') }}</th>
                <th>{{ t('app_created_at') }}</th>
                <th class="text-end">{{ t('app_actions') }}</th>
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
              <tr v-else-if="apps.length === 0">
                <td colspan="7" class="text-center py-4 text-muted">{{ t('app_no_data') }}</td>
              </tr>
              <tr v-for="a in apps" :key="a.ID" v-else>
                <td><code class="text-primary">{{ a.app_id }}</code></td>
                <td>{{ a.name }}</td>
                <td>{{ a.description }}</td>
                <td>{{ a.rate_limit || t('app_unlimited') }}</td>
                <td>
                  <span class="badge" :class="a.status === 1 ? 'bg-success' : 'bg-danger'">
                    {{ a.status === 1 ? t('app_status_active') : t('app_status_disabled') }}
                  </span>
                </td>
                <td>{{ new Date(a.CreatedAt).toLocaleString() }}</td>
                <td class="text-end">
                  <div class="d-inline-flex align-items-center justify-content-end gap-2">
                    <button class="btn btn-sm btn-outline-secondary" @click="openRolesModal(a)" :title="t('app_roles')" v-permission="'app:edit'">
                      <i class="bi bi-shield-lock"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-warning" @click="resetAppKey(a)" :title="t('app_reset_key')" v-permission="'app:reset-key'">
                      <i class="bi bi-key"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-primary" @click="openEditModal(a)" :title="t('app_edit')" v-permission="'app:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-danger" @click="deleteApp(a)" :title="t('common_delete', 'Delete')" v-permission="'app:delete'">
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

    <div class="modal fade" id="appModal" tabindex="-1" ref="appModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? t('app_edit') : t('app_add') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveApp">
              <div class="mb-3">
                <label class="form-label">{{ t('app_name') }}</label>
                <input v-model="form.name" type="text" class="form-control" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ t('app_description') }}</label>
                <textarea v-model="form.description" class="form-control" rows="2"></textarea>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ t('app_rate_limit_hint') }}</label>
                <input v-model.number="form.rate_limit" type="number" class="form-control" min="0">
              </div>
              <div class="mb-3 form-check" v-if="isEditing">
                <input v-model="form.status" type="checkbox" class="form-check-input" id="appStatus" :true-value="1" :false-value="0">
                <label class="form-check-label" for="appStatus">{{ t('app_status_active') }}</label>
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ t('role_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveApp">{{ t('role_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="modal fade" id="appRoleModal" tabindex="-1" ref="appRoleModalRef">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ t('app_roles_title', { name: currentAppForRoles?.name || '' }) }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="d-flex justify-content-between align-items-center mb-3">
              <div class="text-muted small">{{ t('app_roles_hint') }}</div>
              <button type="button" class="btn btn-sm btn-outline-primary" @click="addAppRoleRow">
                <i class="bi bi-plus-lg me-1"></i>{{ t('app_add_role') }}
              </button>
            </div>
            <div v-if="appRoleAssignments.length === 0" class="text-center text-muted py-4 border rounded">
              {{ t('app_no_role_assignments') }}
            </div>
            <div v-for="(assignment, index) in appRoleAssignments" :key="index" class="row g-2 align-items-center mb-2">
              <div class="col-md-5">
                <select class="form-select" v-model.number="assignment.project_id">
                  <option :value="0">{{ t('scope_tenant') }}</option>
                  <option v-for="project in availableProjects" :key="project.ID" :value="project.ID">
                    {{ project.name || project.code || project.ID }}
                  </option>
                </select>
              </div>
              <div class="col-md-6">
                <select class="form-select" v-model.number="assignment.role_id">
                  <option :value="0">{{ t('app_select_role') }}</option>
                  <option v-for="role in availableRoles" :key="role.ID" :value="role.ID">
                    {{ role.name || role.code }}
                  </option>
                </select>
              </div>
              <div class="col-md-1 text-end">
                <button type="button" class="btn btn-outline-danger" @click="removeAppRoleRow(index)">
                  <i class="bi bi-x-lg"></i>
                </button>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ t('role_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveAppRoles">{{ t('app_save_roles') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div class="modal fade" id="appSuccessModal" tabindex="-1" ref="appSuccessModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header bg-success text-white">
            <h5 class="modal-title">{{ t('app_created_success') }}</h5>
            <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="alert alert-warning">
              <i class="bi bi-exclamation-triangle-fill me-1"></i>
              {{ t('app_key_once_warning') }}
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold">AppID</label>
              <div class="input-group">
                <input type="text" class="form-control" readonly :value="newAppInfo.app_id">
                <button class="btn btn-outline-secondary" @click="copyToClipboard(newAppInfo.app_id)">
                  <i class="bi bi-clipboard"></i> {{ t('common_copy', 'Copy') }}
                </button>
              </div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold">AppKey</label>
              <div class="input-group">
                <input type="text" class="form-control" readonly :value="newAppInfo.AppKey">
                <button class="btn btn-outline-secondary" @click="copyToClipboard(newAppInfo.AppKey)">
                  <i class="bi bi-clipboard"></i> {{ t('common_copy', 'Copy') }}
                </button>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-primary" data-bs-dismiss="modal">{{ t('app_i_know') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { Modal } from 'bootstrap'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const { t } = useI18n()

const apps = ref([])
const loading = ref(false)

const appModalRef = ref(null)
let appModal = null
const appRoleModalRef = ref(null)
let appRoleModal = null

const isEditing = ref(false)
const form = ref({
  id: 0,
  name: '',
  description: '',
  rate_limit: 0,
  status: 1
})

const newAppInfo = ref({ app_id: '', AppKey: '' })
const appSuccessModalRef = ref(null)
let appSuccessModal = null
const currentAppForRoles = ref(null)
const appRoleAssignments = ref([])
const availableProjects = ref([])
const availableRoles = ref([])

const goToGuide = () => {
  router.push('/settings/apps/guide')
}

const copyToClipboard = (text) => {
  navigator.clipboard.writeText(text).then(() => {
    alert(t('app_copy_success'))
  }).catch(() => {
    alert(t('app_copy_failed'))
  })
}

const loadApps = async () => {
  loading.value = true
  try {
    const res = await axios.get('/api/apps')
    if (res.data.code === 0) {
      apps.value = res.data.data || []
    }
  } catch (error) {
    console.error('Failed to load apps:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  appModal = new Modal(appModalRef.value)
  appRoleModal = new Modal(appRoleModalRef.value)
  appSuccessModal = new Modal(appSuccessModalRef.value)
  loadApps()
})

const openCreateModal = () => {
  isEditing.value = false
  form.value = { id: 0, name: '', description: '', rate_limit: 0, status: 1 }
  appModal.show()
}

const openEditModal = (item) => {
  isEditing.value = true
  form.value = { id: item.ID, name: item.name, description: item.description, rate_limit: item.rate_limit, status: item.status ?? 1 }
  appModal.show()
}

const normalizeRole = (role) => {
  return role.Role || role
}

const loadAppRoleOptions = async () => {
  const [projectsRes, rolesRes] = await Promise.all([
    axios.get('/api/auth/projects'),
    axios.get('/api/roles', { params: { include_builtin: 1 } })
  ])
  if (projectsRes.data.code === 0) {
    availableProjects.value = projectsRes.data.data || []
  }
  if (rolesRes.data.code === 0) {
    availableRoles.value = (rolesRes.data.data || []).map(normalizeRole)
  }
}

const loadAppRoles = async () => {
  if (!currentAppForRoles.value) return
  const res = await axios.get(`/api/apps/${currentAppForRoles.value.ID}/roles`)
  if (res.data.code === 0) {
    appRoleAssignments.value = (res.data.data || []).map((item) => ({
      project_id: item.project_id || 0,
      role_id: item.role_id || 0
    }))
  } else {
    appRoleAssignments.value = []
    alert(res.data.message)
  }
}

const openRolesModal = async (item) => {
  currentAppForRoles.value = item
  appRoleAssignments.value = []
  await loadAppRoleOptions()
  await loadAppRoles()
  appRoleModal.show()
}

const addAppRoleRow = () => {
  appRoleAssignments.value.push({ project_id: 0, role_id: 0 })
}

const removeAppRoleRow = (index) => {
  appRoleAssignments.value.splice(index, 1)
}

const saveAppRoles = async () => {
  if (!currentAppForRoles.value) return
  const roles = appRoleAssignments.value
    .filter((item) => item.role_id > 0)
    .map((item) => ({
      project_id: item.project_id || 0,
      role_id: item.role_id
    }))
  const res = await axios.put(`/api/apps/${currentAppForRoles.value.ID}/roles`, { roles })
  if (res.data.code === 0) {
    appRoleModal.hide()
  } else {
    alert(res.data.message)
  }
}

const saveApp = async () => {
  try {
    let res
    if (isEditing.value) {
      res = await axios.put(`/api/apps/${form.value.id}`, form.value)
    } else {
      res = await axios.post('/api/apps', form.value)
    }

    if (res.data.code === 0) {
      appModal.hide()
      loadApps()
      if (!isEditing.value) {
        newAppInfo.value = res.data.data
        appSuccessModal.show()
      }
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(error.response?.data?.message || t('app_save_failed'))
  }
}

const deleteApp = async (item) => {
  if (confirm(t('app_delete_confirm', { name: item.name }))) {
    try {
      const res = await axios.delete(`/api/apps/${item.ID}`)
      if (res.data.code === 0) {
        loadApps()
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert(error.response?.data?.message || t('app_delete_failed'))
    }
  }
}

const resetAppKey = async (item) => {
  if (confirm(t('app_reset_key_confirm', { name: item.name }))) {
    try {
      const res = await axios.post(`/api/apps/${item.ID}/reset-key`)
      if (res.data.code === 0) {
        alert(t('app_reset_success_with_key', { key: res.data.data.AppKey }))
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert(error.response?.data?.message || t('app_reset_failed'))
    }
  }
}
</script>
