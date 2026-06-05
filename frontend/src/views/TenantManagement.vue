<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('tenant_management') }}</h2>
      <button class="btn btn-primary" @click="openCreateModal" v-permission="'tenant:create'">
        <i class="bi bi-building-add me-1"></i> {{ $t('tenant_add') }}
      </button>
    </div>

    <!-- Tenants Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ $t('tenant_code') }}</th>
                <th>{{ $t('tenant_name') }}</th>
                <th>{{ $t('admin_name', '管理员姓名') }}</th>
                <th>{{ $t('tenant_phone') }}</th>
                <th>{{ $t('user_created_at') }}</th>
                <th class="text-end">{{ $t('tenant_actions') }}</th>
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
              <tr v-else-if="tenants.length === 0">
                <td colspan="8" class="text-center py-4 text-muted">{{ $t('tenant_no_data') }}</td>
              </tr>
              <tr v-for="t in tenants" :key="t.ID" v-else>
                <td><strong>{{ t.code }}</strong></td>
                <td>{{ t.name }}</td>
                <td>{{ t.contact }}</td>
                <td>{{ t.phone }}</td>
                <td>{{ new Date(t.CreatedAt).toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-') }}</td>
                <td class="text-end">
                  <div class="d-inline-flex align-items-center justify-content-end gap-2">
                    <button class="btn btn-sm btn-outline-secondary" @click="openDetailsModal(t)" :title="$t('common_view_details', '查看详情')">
                      <i class="bi bi-eye"></i>
                    </button>
                    <button class="btn btn-sm" :class="(t.permission_ids && t.permission_ids.length > 0) ? 'btn-outline-info' : 'btn-outline-secondary'" @click="openPermissionModal(t)" :title="$t('tenant_permission_config', '权限配置')" v-permission="'tenant:edit'">
                      <i class="bi bi-shield-check"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-warning" @click="openResetPasswordModal(t)" :title="$t('reset_password', '重置密码')" v-permission="'tenant:edit'">
                      <i class="bi bi-key"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-secondary" @click="openChangeAdminModal(t)" :title="$t('tenant_change_admin', '更换管理员')" v-permission="'tenant:edit'">
                      <i class="bi bi-person-gear"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-primary" @click="openEditModal(t)" :title="$t('tenant_edit', '编辑')" v-permission="'tenant:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-danger" @click="deleteTenant(t)" :disabled="t.code === 'default'" :title="$t('tenant_delete', '删除')" v-permission="'tenant:delete'">
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

    <!-- Tenant Permission Modal -->
    <div class="modal fade" id="tenantPermissionModal" tabindex="-1" ref="tenantPermissionModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('tenant_permission_config', '权限配置') }} - {{ form.name }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div v-if="tenantPermissionOptions.length === 0" class="text-muted small mb-3">
              {{ $t('tenant_permission_empty', '暂无可分配权限') }}
            </div>
            <PermissionDualMode
              :allPermissions="tenantPermissionOptions"
              v-model="form.permission_ids"
              :title="$t('tenant_permission_limit', '租户最大权限集')"
            />
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('common_cancel', '取消') }}</button>
            <button type="button" class="btn btn-primary" @click="saveTenantPermission">{{ $t('tenant_save', '保存') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Tenant Details Modal -->
    <div class="modal fade" id="tenantDetailsModal" tabindex="-1" ref="tenantDetailsModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('tenant_details', '租户详情') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body p-0">
            <div v-if="currentTenantDetails" class="bg-light">
              <div class="p-4 text-center border-bottom bg-white">
                <div class="display-4 text-primary mb-2">
                  <div v-if="currentTenantDetails.logo" class="svg-container mx-auto" style="height: 60px; max-width: 200px; display: flex; align-items: center; justify-content: center;">
                    <div v-if="currentTenantDetails.logo.trim().startsWith('<svg') || currentTenantDetails.logo.trim().startsWith('<?xml')" v-html="DOMPurify.sanitize(currentTenantDetails.logo, { USE_PROFILES: { svg: true } })" style="max-height: 100%; display: flex; align-items: center; justify-content: center;"></div>
                    <img v-else :src="currentTenantDetails.logo" style="max-height: 100%; max-width: 100%; object-fit: contain;">
                  </div>
                  <i v-else class="bi bi-building"></i>
                </div>
                <h5 class="mb-1">{{ currentTenantDetails.name }}</h5>
                <p class="text-muted mb-0">{{ $t('tenant_code', '编码') }}: {{ currentTenantDetails.code }}</p>
              </div>
              <div class="p-4">
                <div class="row g-3">
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('admin_name', '管理员') }}</label>
                    <div class="fw-medium text-primary">{{ currentTenantDetails.contact }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('tenant_phone', '联系电话') }}</label>
                    <div class="fw-medium">{{ currentTenantDetails.phone || $t('common_none', '无') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('tenant_email', '邮箱') }}</label>
                    <div class="fw-medium">{{ currentTenantDetails.email || $t('common_none', '无') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('login_suffix', '专属登录后缀') }}</label>
                    <div class="fw-medium">{{ currentTenantDetails.login_suffix ? '/login/' + currentTenantDetails.login_suffix : $t('common_none', '无') }}</div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('tenant_description', '描述') }}</label>
                    <div class="fw-medium">{{ currentTenantDetails.description || $t('common_none', '无') }}</div>
                  </div>
                  <div class="col-12">
                    <label class="text-muted small mb-1">{{ $t('user_created_at', '创建时间') }}</label>
                    <div class="fw-medium">{{ new Date(currentTenantDetails.CreatedAt).toLocaleString('zh-CN', { hour12: false }).replace(/\//g, '-') }}</div>
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

    <!-- Tenant Modal -->
    <div class="modal fade" id="tenantModal" tabindex="-1" ref="tenantModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog modal-lg">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('tenant_edit') : $t('tenant_add') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveTenant">
              <!-- Basic Info -->
              <h6 class="text-primary mb-3"><i class="bi bi-building me-2"></i>{{ $t('tenant_basic_info', '基本信息') }}</h6>
              <div class="row">
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('tenant_code') }} <span class="text-danger">*</span></label>
                  <input v-model="form.code" type="text" class="form-control" :disabled="isEditing" required>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('tenant_name') }} <span class="text-danger">*</span></label>
                  <input v-model="form.name" type="text" class="form-control" required>
                </div>
              </div>
              <div class="row">
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('login_suffix', '专属登录后缀') }}</label>
                  <div class="input-group">
                    <span class="input-group-text">/login/</span>
                    <input v-model="form.login_suffix" type="text" class="form-control" placeholder="如: my-company">
                  </div>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('tenant_logo', '品牌Logo (SVG, PNG, JPG)') }}</label>
                  <input type="file" class="form-control" accept="image/*" @change="handleLogoUpload">
                  <div v-if="form.logo" class="mt-2 p-2 border rounded bg-light" style="width: fit-content; max-width: 150px; overflow: hidden; display: flex; align-items: center; justify-content: center;">
                    <div v-if="form.logo.trim().startsWith('<svg') || form.logo.trim().startsWith('<?xml')" v-html="DOMPurify.sanitize(form.logo, { USE_PROFILES: { svg: true } })" class="svg-container" style="max-height: 40px; display: flex; align-items: center; justify-content: center;"></div>
                    <img v-else :src="form.logo" style="max-height: 40px; max-width: 100%; object-fit: contain;">
                  </div>
                </div>
              </div>
              <div class="row">
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('tenant_phone') }} <span class="text-danger">*</span></label>
                  <input v-model="form.phone" type="text" class="form-control" required>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('tenant_email') }}</label>
                  <input v-model="form.email" type="email" class="form-control">
                </div>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('tenant_description') }}</label>
                <textarea v-model="form.description" class="form-control" rows="2"></textarea>
              </div>

              <!-- Admin Info (create only) -->
              <template v-if="!isEditing">
                <hr class="my-3">
                <h6 class="text-primary mb-3"><i class="bi bi-person-gear me-2"></i>{{ $t('tenant_admin_info', '管理员信息') }}</h6>
                <div class="row">
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('admin_name', '管理员姓名') }} <span class="text-danger">*</span></label>
                    <input v-model="form.contact" type="text" class="form-control" required>
                  </div>
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('admin_username', '管理员账号') }} <span class="text-danger">*</span></label>
                    <input v-model="form.admin_username" type="text" class="form-control" required>
                  </div>
                </div>
                <div class="row">
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('admin_password', '管理员密码') }} <span class="text-danger">*</span></label>
                    <div class="input-group">
                      <input v-model="form.admin_password" :type="showAdminPassword ? 'text' : 'password'" class="form-control" required>
                      <button class="btn btn-outline-secondary" type="button" @click="showAdminPassword = !showAdminPassword">
                        <i class="bi" :class="showAdminPassword ? 'bi-eye-slash' : 'bi-eye'"></i>
                      </button>
                    </div>
                  </div>
                  <div class="col-md-6 mb-3">
                    <label class="form-label">{{ $t('admin_password_confirm', '确认密码') }} <span class="text-danger">*</span></label>
                    <div class="input-group">
                      <input v-model="form.admin_password_confirm" :type="showConfirmPassword ? 'text' : 'password'" class="form-control" required>
                      <button class="btn btn-outline-secondary" type="button" @click="showConfirmPassword = !showConfirmPassword">
                        <i class="bi" :class="showConfirmPassword ? 'bi-eye-slash' : 'bi-eye'"></i>
                      </button>
                    </div>
                  </div>
                </div>
              </template>
              <template v-else>
                <hr class="my-3">
                <h6 class="text-primary mb-3"><i class="bi bi-person me-2"></i>{{ $t('tenant_contact_info', '联系人信息') }}</h6>
                <div class="mb-3">
                  <label class="form-label">{{ $t('admin_name', '管理员姓名') }} <span class="text-danger">*</span></label>
                  <input v-model="form.contact" type="text" class="form-control" required>
                </div>
              </template>



            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('tenant_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveTenant">{{ $t('tenant_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Reset Password Modal -->
    <div class="modal fade" id="tenantResetPasswordModal" tabindex="-1" ref="tenantResetPasswordModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('user_reset_password_for', { username: resetTenant?.name }) }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('user_new_password', '新密码') }}</label>
              <input v-model="newPassword" type="text" class="form-control" required>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('user_cancel', '取消') }}</button>
            <button type="button" class="btn btn-danger" @click="resetPassword">{{ $t('user_reset_password', '重置密码') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Change Admin Modal -->
    <div class="modal fade" id="changeAdminModal" tabindex="-1" ref="changeAdminModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('tenant_change_admin') }} - {{ changeAdminTenant?.name }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('tenant_select_admin') }} <span class="text-danger">*</span></label>
              <select v-model="selectedAdminUserId" class="form-select">
                <option value="" disabled>{{ $t('tenant_select_admin') }}</option>
                <option v-for="u in tenantUsers" :key="u.ID" :value="u.ID">
                  {{ u.display_name || u.username }} ({{ u.username }})
                </option>
              </select>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('tenant_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="changeAdmin" :disabled="!selectedAdminUserId">{{ $t('tenant_save') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, onMounted } from 'vue'
import axios from 'axios'
import DOMPurify from 'dompurify'
import { Modal } from 'bootstrap'
import { useI18n } from 'vue-i18n'
import PermissionDualMode from '../components/PermissionDualMode.vue'

const { t } = useI18n()

const tenants = ref([])
const loading = ref(false)
const tenantPermissionOptions = ref([])
const tenantModalRef = ref(null)
let tenantModal = null

const tenantDetailsModalRef = ref(null)
let tenantDetailsModal = null
const currentTenantDetails = ref(null)

const tenantPermissionModalRef = ref(null)
let tenantPermissionModal = null

const tenantResetPasswordModalRef = ref(null)
let tenantResetPasswordModal = null
const resetTenant = ref(null)
const newPassword = ref('')

const changeAdminModalRef = ref(null)
let changeAdminModal = null
const changeAdminTenant = ref(null)
const tenantUsers = ref([])
const selectedAdminUserId = ref('')

const showAdminPassword = ref(false)
const showConfirmPassword = ref(false)

const isEditing = ref(false)
const form = ref({
  id: 0,
  code: '',
  name: '',
  contact: '',
  phone: '',
  email: '',
  description: '',
  logo: '',
  login_suffix: '',
  max_users: 0,
  max_devices: 0,
    admin_username: '',
  admin_password: '',
  admin_password_confirm: '',
  permission_ids: []
})



const handleLogoUpload = (event) => {
  const file = event.target.files[0];
  if (!file) return;
  
  if (file.size > 2 * 1024 * 1024) {
    alert(t('logo_too_large', '图片大小不能超过2MB'));
    event.target.value = '';
    return;
  }

  const reader = new FileReader();
  reader.onload = (e) => {
    form.value.logo = e.target.result;
  };

  if (file.type === "image/svg+xml") {
    reader.readAsText(file);
  } else if (file.type.startsWith("image/")) {
    reader.readAsDataURL(file);
  } else {
    alert(t('logo_must_be_image', 'Logo 必须是图片格式文件'));
    event.target.value = '';
  }
}

const loadTenants = async () => {
  loading.value = true
  try {
    const res = await axios.get('/api/tenants')
    if (res.data.code === 0) {
      tenants.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load tenants:", error)
  } finally {
    loading.value = false
  }
}

const loadTenantPermissionOptions = async () => {
  try {
    const res = await axios.get('/api/tenants/permission-options')
    if (res.data.code === 0) {
      tenantPermissionOptions.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load tenant permission options:", error)
  }
}

onMounted(() => {
  tenantModal = new Modal(tenantModalRef.value)
  tenantDetailsModal = new Modal(tenantDetailsModalRef.value)
  tenantPermissionModal = new Modal(tenantPermissionModalRef.value)
  tenantResetPasswordModal = new Modal(tenantResetPasswordModalRef.value)
  changeAdminModal = new Modal(changeAdminModalRef.value)
  loadTenants()
  loadTenantPermissionOptions()
})

const openDetailsModal = (item) => {
  currentTenantDetails.value = item
  tenantDetailsModal.show()
}

const openCreateModal = () => {
  isEditing.value = false
  form.value = {
    id: 0,
    code: '',
    name: '',
    contact: '',
    phone: '',
    email: '',
    description: '',
    logo: '',
    login_suffix: '',
    max_users: 0,
    max_devices: 0,
        admin_username: '',
    admin_password: '',
    admin_password_confirm: '',
    permission_ids: []
  }
  showAdminPassword.value = false
  showConfirmPassword.value = false
  tenantModal.show()
}

const openEditModal = (item) => {
  isEditing.value = true
  form.value = {
    id: item.ID,
    code: item.code,
    name: item.name,
    contact: item.contact,
    phone: item.phone,
    email: item.email,
    description: item.description,
    logo: item.logo,
    login_suffix: item.login_suffix,
    max_users: item.max_users,
    max_devices: item.max_devices,
        permission_ids: item.permission_ids || []
  }
  tenantModal.show()
}

const openPermissionModal = (item) => {
  form.value = {
    id: item.ID,
    name: item.name,
    permission_ids: item.permission_ids || []
  }
  tenantPermissionModal.show()
}

const saveTenantPermission = async () => {
  try {
    const res = await axios.put(`/api/tenants/${form.value.id}`, {
      permission_ids: form.value.permission_ids
    })
    if (res.data.code === 0) {
      tenantPermissionModal.hide()
      loadTenants()
    } else {
      alert(res.data.message || '保存失败')
    }
  } catch (error) {
    console.error("Failed to save tenant permissions:", error)
    alert('保存失败，请检查网络或联系管理员')
  }
}

const saveTenant = async () => {
  if (!isEditing.value && form.value.admin_password !== form.value.admin_password_confirm) {
    alert(t('auth_password_mismatch', '两次输入的密码不一致！'))
    return
  }
  try {
    let res
    if (isEditing.value) {
      res = await axios.put(`/api/tenants/${form.value.id}`, form.value)
    } else {
      res = await axios.post('/api/tenants', form.value)
    }
    
    if (res.data.code === 0) {
      tenantModal.hide()
      loadTenants()
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(t('common_save_failed', '保存失败'))
  }
}

const deleteTenant = async (item) => {
  if (confirm(t('tenant_delete_confirm', { name: item.name }))) {
    try {
      const res = await axios.delete(`/api/tenants/${item.ID}`)
      if (res.data.code === 0) {
        loadTenants()
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert(t('common_delete_failed', '删除失败'))
    }
  }
}

const openResetPasswordModal = (item) => {
  resetTenant.value = item
  newPassword.value = ''
  tenantResetPasswordModal.show()
}

const resetPassword = async () => {
  if (!newPassword.value) return

  try {
    const res = await axios.post(`/api/tenants/${resetTenant.value.ID}/reset-password`, { new_password: newPassword.value })
    if (res.data.code === 0) {
      tenantResetPasswordModal.hide()
      alert(t('user_reset_success', '密码重置成功'))
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(t('user_reset_failed', '密码重置失败'))
  }
}

const openChangeAdminModal = async (item) => {
  changeAdminTenant.value = item
  selectedAdminUserId.value = ''
  tenantUsers.value = []
  try {
    const res = await axios.get(`/api/tenants/${item.ID}/users`)
    if (res.data.code === 0) {
      tenantUsers.value = (res.data.data || [])
    }
  } catch (error) {
    console.error("Failed to load tenant users:", error)
  }
  changeAdminModal.show()
}

const changeAdmin = async () => {
  if (!selectedAdminUserId.value) return

  if (!confirm(t('tenant_change_admin_confirm'))) return

  try {
    const res = await axios.post(`/api/tenants/${changeAdminTenant.value.ID}/change-admin`, { user_id: selectedAdminUserId.value })
    if (res.data.code === 0) {
      changeAdminModal.hide()
      alert(t('tenant_change_admin_success'))
      loadTenants()
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(error.response?.data?.message || t('common_save_failed', '操作失败'))
  }
}
</script>

<style scoped>
:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
}
</style>
