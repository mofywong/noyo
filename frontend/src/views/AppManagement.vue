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
                    <span class="visually-hidden">{{ t('loading') }}</span>
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
                    <button class="btn btn-sm btn-outline-secondary" @click="openAccessModal(a)" :title="t('app_access')" v-permission="'app:edit'">
                      <i class="bi bi-shield-lock"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-warning" @click="resetAppKey(a)" :title="t('app_reset_key')" v-permission="'app:reset-key'">
                      <i class="bi bi-key"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-primary" @click="openEditModal(a)" :title="t('app_edit')" v-permission="'app:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-danger" @click="deleteApp(a)" :title="t('common_delete')" v-permission="'app:delete'">
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

    <div class="modal fade" id="appAccessModal" tabindex="-1" ref="appAccessModalRef">
      <div class="modal-dialog modal-xl modal-dialog-scrollable">
        <div class="modal-content">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">{{ t('app_access_title', { name: currentAppForAccess?.name || '' }) }}</h5>
              <div class="text-muted small mt-1">{{ t('app_access_hint') }}</div>
            </div>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body bg-light">
            <div v-if="accessLoading" class="text-center py-5">
              <div class="spinner-border text-primary" role="status"></div>
            </div>
            <div v-else-if="availableProjects.length === 0" class="text-center text-muted py-5 bg-white border rounded">
              {{ t('app_access_no_projects') }}
            </div>
            <div v-else class="app-access-grid">
              <section v-for="project in availableProjects" :key="project.ID" class="app-project-card">
                <div class="app-project-header">
                  <div>
                    <div class="fw-semibold">{{ project.name || project.code || project.ID }}</div>
                    <div class="text-muted small">{{ t('project_code') }}: {{ project.code || project.ID }}</div>
                  </div>
                  <div class="form-check form-switch m-0">
                    <input class="form-check-input" type="checkbox" role="switch" :id="`app-project-${project.ID}`" :checked="isProjectEnabled(project.ID)" @change="toggleProject(project.ID, $event.target.checked)">
                    <label class="form-check-label small" :for="`app-project-${project.ID}`">{{ t('app_project_enabled') }}</label>
                  </div>
                </div>

                <div v-if="isProjectEnabled(project.ID)" class="app-project-body">
                  <div class="access-panel">
                    <div class="access-panel-title">
                      <i class="bi bi-grid-3x3-gap text-primary"></i>
                      {{ t('app_function_permissions') }}
                    </div>
                    <div v-if="permissionsForProject(project.ID).length === 0" class="text-muted small py-3">
                      {{ t('app_no_permissions_available') }}
                    </div>
                    <div v-else class="permission-groups">
                      <div v-for="group in groupedPermissions(project.ID)" :key="group.module" class="permission-group">
                        <div class="permission-group-title">{{ moduleLabel(group.module) }}</div>
                        <label v-for="permission in group.permissions" :key="permission.ID" class="permission-check">
                          <input type="checkbox" class="form-check-input" :checked="projectAccess(project.ID).permission_ids.includes(permission.ID)" @change="togglePermission(project.ID, permission.ID, $event.target.checked)">
                          <span>{{ permission.name || permission.code }}</span>
                        </label>
                      </div>
                    </div>
                  </div>

                  <div class="access-panel">
                    <div class="access-panel-title">
                      <i class="bi bi-tags text-primary"></i>
                      {{ t('app_device_tag_permissions') }}
                    </div>
                    <div v-if="deviceTags.length === 0" class="text-muted small py-3">
                      {{ t('app_no_device_tags') }}
                    </div>
                    <div v-else class="tag-permission-list">
                      <div v-for="tag in deviceTags" :key="tag.ID" class="tag-permission-row">
                        <div class="tag-name">
                          <span class="tag-swatch" :style="{ backgroundColor: tag.color || tag.Color || '#0d6efd' }"></span>
                          <span>{{ tag.name || tag.Name }}</span>
                        </div>
                        <select class="form-select form-select-sm tag-select" :value="tagPermission(project.ID, tag.ID)" @change="setTagPermission(project.ID, tag.ID, $event.target.value)">
                          <option value="">{{ t('app_permission_none') }}</option>
                          <option value="read">{{ t('app_permission_read') }}</option>
                          <option value="write">{{ t('app_permission_write') }}</option>
                        </select>
                      </div>
                    </div>
                  </div>
                </div>
              </section>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ t('role_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveAppAccess">{{ t('app_save_access') }}</button>
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
                  <i class="bi bi-clipboard"></i> {{ t('common_copy') }}
                </button>
              </div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold">AppKey</label>
              <div class="input-group">
                <input type="text" class="form-control" readonly :value="newAppInfo.AppKey">
                <button class="btn btn-outline-secondary" @click="copyToClipboard(newAppInfo.AppKey)">
                  <i class="bi bi-clipboard"></i> {{ t('common_copy') }}
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
const accessLoading = ref(false)

const appModalRef = ref(null)
let appModal = null
const appAccessModalRef = ref(null)
let appAccessModal = null
const appSuccessModalRef = ref(null)
let appSuccessModal = null

const isEditing = ref(false)
const form = ref({ id: 0, name: '', description: '', rate_limit: 0, status: 1 })
const newAppInfo = ref({ app_id: '', AppKey: '' })
const currentAppForAccess = ref(null)
const appAccessProjects = ref([])
const availableProjects = ref([])
const permissionsByProject = ref({})
const deviceTags = ref([])

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
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  appModal = new Modal(appModalRef.value)
  appAccessModal = new Modal(appAccessModalRef.value)
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

const normalizeProjectAccess = (item) => ({
  project_id: item.project_id || 0,
  permission_ids: [...new Set(item.permission_ids || [])],
  device_tag_permissions: (item.device_tag_permissions || []).filter((tag) => tag.tag_id && tag.permission)
})

const loadAppAccessOptions = async () => {
  const res = await axios.get('/api/apps/access-options')
  if (res.data.code !== 0) throw new Error(res.data.message)
  const data = res.data.data || {}
  availableProjects.value = data.projects || []
  deviceTags.value = data.device_tags || []
  permissionsByProject.value = {}
  for (const row of data.permissions_by_project || []) {
    permissionsByProject.value[row.project_id] = row.permissions || []
  }
}

const loadAppAccess = async () => {
  const res = await axios.get(`/api/apps/${currentAppForAccess.value.ID}/access`)
  if (res.data.code !== 0) throw new Error(res.data.message)
  appAccessProjects.value = ((res.data.data || {}).projects || []).map(normalizeProjectAccess)
}

const openAccessModal = async (item) => {
  currentAppForAccess.value = item
  appAccessProjects.value = []
  accessLoading.value = true
  appAccessModal.show()
  try {
    await loadAppAccessOptions()
    await loadAppAccess()
  } catch (error) {
    alert(error.response?.data?.message || error.message || t('app_access_load_failed'))
  } finally {
    accessLoading.value = false
  }
}

const projectAccess = (projectId) => {
  let access = appAccessProjects.value.find((item) => item.project_id === projectId)
  if (!access) {
    access = { project_id: projectId, permission_ids: [], device_tag_permissions: [] }
    appAccessProjects.value.push(access)
  }
  return access
}

const isProjectEnabled = (projectId) => appAccessProjects.value.some((item) => item.project_id === projectId)

const toggleProject = (projectId, enabled) => {
  if (enabled) {
    projectAccess(projectId)
  } else {
    appAccessProjects.value = appAccessProjects.value.filter((item) => item.project_id !== projectId)
  }
}

const permissionsForProject = (projectId) => permissionsByProject.value[projectId] || []

const groupedPermissions = (projectId) => {
  const groups = new Map()
  for (const permission of permissionsForProject(projectId)) {
    const module = permission.module || permission.Module || 'other'
    if (!groups.has(module)) groups.set(module, [])
    groups.get(module).push(permission)
  }
  return Array.from(groups.entries()).map(([module, permissions]) => ({ module, permissions }))
}

const moduleLabel = (module) => t(`perm_mod_${module}`, module)

const togglePermission = (projectId, permissionId, checked) => {
  const access = projectAccess(projectId)
  if (checked && !access.permission_ids.includes(permissionId)) {
    access.permission_ids.push(permissionId)
  }
  if (!checked) {
    access.permission_ids = access.permission_ids.filter((id) => id !== permissionId)
  }
}

const tagPermission = (projectId, tagId) => {
  const access = projectAccess(projectId)
  return access.device_tag_permissions.find((item) => item.tag_id === tagId)?.permission || ''
}

const setTagPermission = (projectId, tagId, permission) => {
  const access = projectAccess(projectId)
  access.device_tag_permissions = access.device_tag_permissions.filter((item) => item.tag_id !== tagId)
  if (permission) {
    access.device_tag_permissions.push({ tag_id: tagId, permission })
  }
}

const saveAppAccess = async () => {
  if (!currentAppForAccess.value) return
  const projects = appAccessProjects.value
    .filter((item) => item.project_id > 0)
    .map((item) => normalizeProjectAccess(item))
  try {
    const res = await axios.put(`/api/apps/${currentAppForAccess.value.ID}/access`, { projects })
    if (res.data.code === 0) {
      appAccessModal.hide()
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(error.response?.data?.message || t('app_access_save_failed'))
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

<style scoped>
.app-access-grid {
  display: grid;
  gap: 1rem;
}

.app-project-card {
  background: #fff;
  border: 1px solid #dfe3ea;
  border-radius: 8px;
  overflow: hidden;
}

.app-project-header {
  align-items: center;
  background: #f8fafc;
  border-bottom: 1px solid #e5e7eb;
  display: flex;
  justify-content: space-between;
  padding: 1rem;
}

.app-project-body {
  display: grid;
  gap: 1rem;
  grid-template-columns: minmax(0, 1.4fr) minmax(280px, 0.8fr);
  padding: 1rem;
}

.access-panel {
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  padding: 1rem;
}

.access-panel-title {
  align-items: center;
  display: flex;
  font-weight: 600;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.permission-groups {
  display: grid;
  gap: 0.75rem;
}

.permission-group {
  background: #fbfcfe;
  border: 1px solid #edf0f5;
  border-radius: 6px;
  padding: 0.75rem;
}

.permission-group-title {
  color: #6b7280;
  font-size: 0.82rem;
  font-weight: 600;
  margin-bottom: 0.5rem;
}

.permission-check {
  align-items: center;
  display: inline-flex;
  gap: 0.4rem;
  margin: 0 1rem 0.5rem 0;
}

.tag-permission-list {
  display: grid;
  gap: 0.5rem;
}

.tag-permission-row {
  align-items: center;
  display: flex;
  justify-content: space-between;
}

.tag-name {
  align-items: center;
  display: inline-flex;
  gap: 0.5rem;
  min-width: 0;
}

.tag-swatch {
  border-radius: 50%;
  display: inline-block;
  height: 0.7rem;
  width: 0.7rem;
}

.tag-select {
  width: 8.5rem;
}

@media (max-width: 992px) {
  .app-project-body {
    grid-template-columns: 1fr;
  }
}
</style>
