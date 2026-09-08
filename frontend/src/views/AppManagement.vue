<template>
  <div class="app-management-page page-fixed-height">
    <div class="page-header list-page-header">
      <div>
        <h1>{{ t('app_management') }}</h1>
        <p class="page-subtitle">{{ t('app_management_subtitle', '管理系统 Open API 应用凭证、流控与集成配置') }}</p>
      </div>
      <div class="d-flex align-items-center gap-2">
        <LiquidGlassButton variant="outline-info" size="sm" icon="bi bi-book" @click="goToGuide">
          {{ t('app_access_guide') }}
        </LiquidGlassButton>
        <LiquidGlassButton variant="primary" size="sm" icon="bi bi-window-sidebar" @click="openCreateModal" v-permission="'app:create'">
          {{ t('app_add') }}
        </LiquidGlassButton>
      </div>
    </div>

    <div class="page-toolbar device-list-control-bar list-page-controls">
      <CompactListMetrics v-model="metricFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="t('stat_total', '应用统计')" />
    </div>

    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4">AppID</th>
                <th>{{ t('app_name') }}</th>
                <th>{{ t('app_description') }}</th>
                <th>{{ t('app_rate_limit') }}</th>
                <th>{{ t('app_status') }}</th>
                <th>{{ t('app_created_at') }}</th>
                <th class="text-end pe-4">{{ t('app_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <!-- 骨架屏 -->
              <tr v-if="loading" v-for="n in 5" :key="'sk-' + n">
                <td><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 140px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 160px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 80px;"></div></td>
                <td><div class="skeleton" style="height: 20px; width: 80px; border-radius: 999px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 130px;"></div></td>
                <td class="text-end pe-4"><div class="skeleton ms-auto" style="height: 28px; width: 100px;"></div></td>
              </tr>
              <!-- 空状态 -->
              <tr v-else-if="filteredApps.length === 0">
                <td colspan="7" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-grid-fill"></i>
                    <p>{{ t('app_no_data', '暂无应用接入数据') }}</p>
                    <LiquidGlassButton variant="primary" icon="bi bi-plus-lg" @click="openAppModal" v-permission="'app:create'">
                      {{ t('app_create') }}
                    </LiquidGlassButton>
                  </div>
                </td>
              </tr>
              <tr v-for="a in filteredApps" :key="a.ID" v-else>
                <td><code class="text-primary">{{ a.app_id }}</code></td>
                <td>{{ a.name }}</td>
                <td>{{ a.description }}</td>
                <td>{{ a.rate_limit || t('app_unlimited') }}</td>
                <td>
                  <span class="dash-pill" :class="a.status === 1 ? 'dash-pill--success' : 'dash-pill--neutral'">
                    <span class="dash-pill-dot"></span>
                    {{ a.status === 1 ? t('app_status_active') : t('app_status_disabled') }}
                  </span>
                </td>
                <td>{{ formatDateTime(a.CreatedAt) }}</td>
                <td class="text-end pe-4">
                  <div class="table-actions">
                    <button class="table-action-btn table-action-btn--info" @click="openAccessModal(a)" :title="t('app_access')" v-permission="'app:edit'">
                      <i class="bi bi-shield-lock"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--warning" @click="resetAppKey(a)" :title="t('app_reset_key')" v-permission="'app:reset-key'">
                      <i class="bi bi-key"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--primary" @click="openEditModal(a)" :title="t('app_edit')" v-permission="'app:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--danger" @click="deleteApp(a)" :title="t('common_delete')" v-permission="'app:delete'">
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
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ t('role_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveApp">{{ t('role_save') }}</LiquidGlassButton>
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
          <div class="modal-body app-access-modal-body">
            <div v-if="accessLoading" class="text-center py-5">
              <div class="spinner-border text-primary" role="status"></div>
            </div>
            <div v-else-if="availableProjects.length === 0" class="text-center text-muted py-5 border rounded app-access-empty">
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
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ t('role_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveAppAccess">{{ t('app_save_access') }}</LiquidGlassButton>
          </div>
        </div>
      </div>
    </div>

    <div class="modal fade" id="appSuccessModal" tabindex="-1" ref="appSuccessModalRef" data-bs-backdrop="static">
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
            <LiquidGlassButton variant="primary" data-bs-dismiss="modal">{{ t('app_i_know') }}</LiquidGlassButton>
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
import { Modal } from 'bootstrap'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '../utils/dateTime.js'
import CompactListMetrics from '../components/CompactListMetrics.vue'

const router = useRouter()
const { t } = useI18n()

const apps = ref([])
const loading = ref(false)
const accessLoading = ref(false)

const activeAppCount = computed(() => apps.value.filter(a => a.status === 1).length)
const disabledAppCount = computed(() => apps.value.filter(a => a.status !== 1).length)
const metricCards = computed(() => [
  { key: 'total', label: t('stat_total', '总接入应用'), value: apps.value.length, icon: 'bi-grid-fill', tone: 'brand' },
  { key: 'active', label: t('stat_active_apps', '正常运行'), value: activeAppCount.value, icon: 'bi-check-circle-fill', tone: 'success' },
  { key: 'disabled', label: t('stat_disabled_apps', '限制/停用'), value: disabledAppCount.value, icon: 'bi-slash-circle', tone: 'neutral' }
])
const metricFilters = ref([])
const filteredApps = computed(() => {
  if (!metricFilters.value.length) return apps.value
  return apps.value.filter((app) =>
    (metricFilters.value.includes('active') && app.status === 1) ||
    (metricFilters.value.includes('disabled') && app.status !== 1)
  )
})

const toggleMetricFilter = (key) => {
  metricFilters.value = metricFilters.value.includes(key)
    ? metricFilters.value.filter((item) => item !== key)
    : [...metricFilters.value, key]
}

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

.app-access-modal-body {
  background: var(--bg-surface);
  color: var(--text-main);
}

.app-access-empty {
  background: var(--bg-elevated);
  border-color: var(--border-color) !important;
}
</style>
