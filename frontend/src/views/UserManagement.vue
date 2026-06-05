<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('user_management') }}</h2>
      <div class="d-flex align-items-center gap-2">
        <select v-model="filterProjectId" class="form-select form-select-sm" @change="changePage(1)" style="width: 150px;">
          <option value="">{{ $t('project_all', '全部项目') }}</option>
          <option v-for="p in allProjects" :key="p.ID" :value="p.ID">{{ p.name }}</option>
        </select>
        <select v-model="filterRoleId" class="form-select form-select-sm" @change="changePage(1)" style="width: 150px;">
          <option value="">{{ $t('role_all', '全部角色') }}</option>
          <option v-for="r in allRoles" :key="r.ID" :value="r.ID">{{ r.name }}</option>
        </select>
        <button class="btn btn-sm btn-outline-secondary" @click="resetFilters">{{ $t('common_reset', '重置') }}</button>
        <button class="btn btn-primary btn-sm ms-2" @click="openCreateModal" v-permission="'user:create'">
          <i class="bi bi-person-plus me-1"></i> {{ $t('user_add') }}
        </button>
      </div>
    </div>

    <!-- Users Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ $t('auth_username') }}</th>
                <th>{{ $t('user_name', '姓名') }}</th>
                <th>{{ $t('user_permissions_assign', '权限分配 (项目与角色)') }}</th>
                <th>{{ $t('user_last_login') }}</th>
                <th>{{ $t('user_created_at') }}</th>
                <th class="text-end">{{ $t('user_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="6" class="text-center py-4">
                  <div class="spinner-border text-primary" role="status">
                    <span class="visually-hidden">Loading...</span>
                  </div>
                </td>
              </tr>
              <tr v-else-if="users.length === 0">
                <td colspan="6" class="text-center py-4 text-muted">{{ $t('user_no_users') }}</td>
              </tr>
              <tr v-for="user in users" :key="user.id" v-else>
                <td><strong>{{ user.username }}</strong></td>
                <td>{{ user.display_name }}</td>
                <td>
                  <span class="d-flex gap-1 flex-wrap">
                    <span v-for="(item, idx) in getPermissionsSummary(user).slice(0, 2)" :key="idx" 
                          class="badge" :class="item.type === 'primary' ? 'text-bg-primary' : 'text-bg-info'">
                      {{ item.text }}
                    </span>
                    <span v-if="getPermissionsSummary(user).length > 2" 
                          class="badge text-bg-secondary" style="cursor: pointer;" @click="openDetailsModal(user)">
                      +{{ getPermissionsSummary(user).length - 2 }} {{ $t('common_more', '更多') }}
                    </span>
                    <span v-if="getPermissionsSummary(user).length === 0" class="text-muted small">{{ $t('user_no_permission', '无权限') }}</span>
                  </span>
                </td>
                <td>{{ user.last_login_at || $t('user_never') }}</td>
                <td>{{ user.created_at }}</td>
                <td class="text-end">
                  <div class="d-inline-flex align-items-center justify-content-end gap-2">
                    <button class="btn btn-sm btn-outline-secondary" @click="openDetailsModal(user)" :title="$t('common_view_details', '查看详情')">
                      <i class="bi bi-eye"></i>
                    </button>
                    <button class="btn btn-sm" :class="hasAssignedRoles(user) ? 'btn-outline-info' : 'btn-outline-secondary'" @click="openRolesModal(user)" :title="$t('user_assign_roles', '分配角色')" :disabled="isRoleModificationDisabled(user)" v-permission="'user:edit'">
                      <i class="bi bi-shield-check"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-warning" @click="openResetPasswordModal(user)" :title="$t('reset_password', '重置密码')" v-permission="'user:edit'">
                      <i class="bi bi-key"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-primary" @click="openEditModal(user)" :title="$t('user_edit', '编辑')" v-permission="'user:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="btn btn-sm btn-outline-danger" @click="deleteUser(user)" :disabled="isUserDeletionDisabled(user)" :title="$t('user_delete', '删除')" v-permission="'user:delete'">
                      <i class="bi bi-trash"></i>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
      <div class="card-footer d-flex justify-content-between align-items-center">
        <span class="text-muted small">Total: {{ total }}</span>
        <nav v-if="total > pageSize">
          <ul class="pagination pagination-sm mb-0">
            <li class="page-item" :class="{ disabled: page === 1 }">
              <button class="page-link" @click="changePage(page - 1)">Previous</button>
            </li>
            <li class="page-item" :class="{ disabled: page * pageSize >= total }">
              <button class="page-link" @click="changePage(page + 1)">Next</button>
            </li>
          </ul>
        </nav>
      </div>
    </div>

    <!-- User Modal -->
    <div class="modal fade" id="userModal" tabindex="-1" ref="userModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('user_edit') : $t('user_add') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveUser">
              <div class="row">
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ getFormNameLabel() }} <span class="text-danger">*</span></label>
                  <input v-model="form.display_name" type="text" class="form-control" required>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('auth_username', '账号') }} <span class="text-danger">*</span></label>
                  <input v-model="form.username" type="text" class="form-control" :disabled="isEditing" required>
                </div>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('user_phone', '电话') }}</label>
                <input v-model="form.email" type="text" class="form-control">
              </div>
              <div class="row" v-if="!isEditing">
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('auth_password', '密码') }} <span class="text-danger">*</span></label>
                  <input v-model="form.password" type="password" class="form-control">
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('auth_password_confirm', '确认密码') }} <span class="text-danger">*</span></label>
                  <input v-model="form.confirm_password" type="password" class="form-control">
                </div>
              </div>

            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('user_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveUser">{{ $t('user_save') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Reset Password Modal -->
    <div class="modal fade" id="resetPasswordModal" tabindex="-1" ref="resetPasswordModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('user_reset_password_for', { username: resetUser.username }) }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ $t('user_new_password') }}</label>
              <input v-model="newPassword" type="text" class="form-control" required>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('user_cancel') }}</button>
            <button type="button" class="btn btn-danger" @click="resetPassword">{{ $t('user_reset_password') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Assign Roles Modal -->
    <div class="modal fade" id="rolesModal" tabindex="-1" ref="rolesModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('user_assign_roles_for', '为 {username} 分配角色').replace('{username}', currentUserToAssign?.username) }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
             <div v-if="allRoles.length === 0" class="text-muted">{{ $t('user_no_available_roles', '暂无可用的角色，请先在角色管理中添加。') }}</div>
             
             <div>
               <div class="d-flex justify-content-between align-items-center border-bottom pb-2 mb-2">
                 <h6 class="fw-bold mb-0">{{ $t('user_unified_role_config', '用户权限配置') }}</h6>
                 <button type="button" class="btn btn-sm btn-outline-primary py-0" @click="addUnifiedRoleRow">
                   <i class="bi bi-plus"></i>
                 </button>
               </div>
               <div v-if="unifiedRoleAssignments.length === 0" class="text-muted small">
                 {{ $t('user_no_unified_roles', '未分配任何权限。') }}
               </div>
               <div v-for="(pr, index) in unifiedRoleAssignments" :key="index" class="row mb-2 align-items-end">
                 <div class="col-md-5">
                   <label v-if="index === 0" class="form-label small text-muted mb-1">{{ $t('user_select_scope', '授权范围') }}</label>
                   <select v-model="pr.project_id" class="form-select form-select-sm" required>
                     <option :value="null" disabled>{{ $t('user_please_select_scope', '请选择范围') }}</option>
                     <option :value="0">{{ $t('scope_tenant', '租户') }}</option>
                     <option v-for="p in allProjects" :key="p.ID" :value="p.ID">{{ p.name }}</option>
                   </select>
                 </div>
                 <div class="col-md-5">
                   <label v-if="index === 0" class="form-label small text-muted mb-1">{{ $t('user_project_role', '角色') }}</label>
                   <select v-model="pr.role_id" class="form-select form-select-sm" required>
                     <option :value="0" disabled>{{ $t('user_please_select_role', '请选择角色') }}</option>
                     <option v-for="r in getRolesForScope(pr.project_id)" :key="r.ID" :value="r.ID">{{ r.name }}</option>
                   </select>
                 </div>
                 <div class="col-md-2">
                   <button type="button" class="btn btn-sm btn-outline-danger w-100" @click="removeUnifiedRoleRow(index)">
                     <i class="bi bi-trash"></i>
                   </button>
                 </div>
               </div>
             </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('user_cancel') }}</button>
            <button type="button" class="btn btn-primary" @click="saveRoles">{{ $t('user_save_assign', '保存分配') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- User Details Modal -->
    <div class="modal fade" id="userDetailsModal" tabindex="-1" ref="userDetailsModalRef" data-bs-backdrop="static" data-bs-keyboard="false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('user_details', '用户详情') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body p-0">
            <div v-if="currentUserDetails" class="bg-light">
              <div class="p-4 text-center border-bottom bg-white">
                <div class="display-4 text-primary mb-2">
                  <i class="bi bi-person-circle"></i>
                </div>
                <h5 class="mb-1">{{ currentUserDetails.display_name }}</h5>
                <p class="text-muted mb-0">@{{ currentUserDetails.username }}</p>
              </div>
              <div class="p-4">
                <div class="row g-3 mb-4">
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('user_phone', '电话/联系方式') }}</label>
                    <div class="fw-medium">{{ currentUserDetails.email || $t('user_not_provided', '未提供') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('user_last_login') }}</label>
                    <div class="fw-medium">{{ currentUserDetails.last_login_at || $t('user_never') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('user_created_at') }}</label>
                    <div class="fw-medium">{{ currentUserDetails.created_at }}</div>
                  </div>
                </div>
                
                <h6 class="border-bottom pb-2 mb-3"><i class="bi bi-shield-check text-primary me-2"></i>{{ $t('user_permission_details', '权限分配明细') }}</h6>
                
                <div class="card border-0 shadow-sm mb-3">
                  <div class="card-body p-3">
                    <div class="text-muted small mb-2 fw-bold">{{ $t('user_global_roles', '全局角色') }}</div>
                    <div v-if="currentUserDetails.tenant_roles && currentUserDetails.tenant_roles.length > 0" class="d-flex flex-wrap gap-2">
                      <span v-for="r in currentUserDetails.tenant_roles" :key="r.role_id" class="badge text-bg-primary px-3 py-2">
                        <i class="bi bi-globe me-1"></i> {{ r.role_name }}
                      </span>
                    </div>
                    <div v-else class="text-muted small">{{ $t('user_no_global_roles', '无全局角色') }}</div>
                  </div>
                </div>

                <div class="card border-0 shadow-sm">
                  <div class="card-body p-3">
                    <div class="text-muted small mb-2 fw-bold">{{ $t('user_project_permissions', '项目权限') }}</div>
                    <div v-if="currentUserDetails.projects && currentUserDetails.projects.length > 0">
                      <ul class="list-group list-group-flush">
                        <li v-for="p in getGroupedProjects(currentUserDetails.projects)" :key="p.name" class="list-group-item px-0 d-flex justify-content-between align-items-center bg-transparent">
                          <span><i class="bi bi-folder text-info me-2"></i>{{ p.name }}</span>
                          <span class="text-end">
                            <span v-for="role in p.roles" :key="role" class="badge text-bg-light text-dark border ms-1">{{ role }}</span>
                          </span>
                        </li>
                      </ul>
                    </div>
                    <div v-else class="text-muted small">{{ $t('user_no_project_permissions', '无项目权限') }}</div>
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

  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { Modal } from 'bootstrap'
import { useAuthStore } from '../stores/auth'
import { useI18n } from 'vue-i18n'

const authStore = useAuthStore()
const currentUser = authStore.user
const { t } = useI18n()

const currentProjectId = ref(Number(localStorage.getItem('current_project_id') || 0));

const users = ref([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(10)
const loading = ref(false)

const userModalRef = ref(null)
let userModal = null
const resetPasswordModalRef = ref(null)
let resetPasswordModal = null
const rolesModalRef = ref(null)
let rolesModal = null
const userDetailsModalRef = ref(null)
let userDetailsModal = null
const currentUserDetails = ref(null)

const filterProjectId = ref('')
const filterRoleId = ref('')

const allRoles = ref([])
const unifiedRoleAssignments = ref([])
const allProjects = ref([])
const currentUserToAssign = ref(null)

const addUnifiedRoleRow = () => {
  unifiedRoleAssignments.value.push({ project_id: null, role_id: 0 })
}
const removeUnifiedRoleRow = (index) => {
  unifiedRoleAssignments.value.splice(index, 1)
}

const getRolesForScope = (projectId) => {
  if (projectId === null || projectId === undefined) return [];
  if (projectId === 0) {
    return allRoles.value.filter(isTenantAssignableRole);
  }
  return allRoles.value.filter(role => isProjectAssignableRole(role, projectId));
}

const isTenantAssignableRole = (role) => {
  return role.project_id === 0 && role.is_inherited !== true && role.code !== 'project_admin' && role.code !== 'super_admin';
}

const isProjectAssignableRole = (role, projectId) => {
  if (role.code === 'tenant_admin' || role.code === 'super_admin') return false;
  if (role.code === 'project_admin') return true;
  return (role.project_id === 0 && role.is_inherited === true) || role.project_id === projectId;
}

const isEditing = ref(false)
const form = ref({
  id: 0,
  username: '',
  password: '',
  confirm_password: '',
  display_name: '',
  email: '',
  role: ''
})

const getFormNameLabel = () => {
  const user = isEditing.value ? users.value.find(u => u.id === form.value.id) : null;
  if (user) {
    if (user.is_system_admin || user.is_tenant_admin || user.is_project_admin) {
      return t('user_admin_name', '管理员姓名');
    }
  }
  return t('user_display_name', '姓名');
};

const getRolesForProject = (projectId) => {
  if (!projectId) return [];
  return allRoles.value.filter(role => isProjectAssignableRole(role, projectId));
}

const getGroupedProjects = (projects) => {
  if (!projects || projects.length === 0) return [];
  const map = new Map();
  projects.forEach(p => {
    if (!p.role_id || !p.role_name) {
      return;
    }
    if (!map.has(p.project_name)) {
      map.set(p.project_name, new Set());
    }
    if (p.role_name) {
      map.get(p.project_name).add(p.role_name);
    }
  });
  return Array.from(map.entries()).map(([name, roles]) => {
    return { name, roles: Array.from(roles) };
  });
}

const hasAssignedRoles = (user) => {
  const hasTenantRole = user.tenant_roles?.some(r => r.role_id && r.role_name);
  const hasProjectRole = user.projects?.some(p => p.role_id && p.role_name);
  return !!(hasTenantRole || hasProjectRole);
}

const getPermissionsSummary = (user) => {
  const items = [];
  if (user.tenant_roles) {
    const tenantName = authStore.user?.tenant_name || localStorage.getItem('tenant_name') || '租户';
    user.tenant_roles.forEach(r => items.push({ text: `${tenantName} - ${r.role_name}`, type: 'primary' }));
  }
  const grouped = getGroupedProjects(user.projects);
  grouped.forEach(p => items.push({ text: `${p.name} - ${p.roles.join('、')}`, type: 'info' }));
  return items;
}

const openDetailsModal = (user) => {
  currentUserDetails.value = user;
  userDetailsModal.show();
}

const isRoleModificationDisabled = (user) => {
  if (user.is_system_admin) {
    return true;
  }
  if (user.tenant_roles && user.tenant_roles.some(r => r.role_code === 'super_admin')) {
    return true;
  }
  if (user.projects && user.projects.some(p => p.role_code === 'tenant_admin')) {
    return true;
  }
  return false;
}

const isUserDeletionDisabled = (user) => {
  if (user.username === 'admin' || user.id === currentUser.id) return true;
  if (user.is_system_admin) return true;
  if (user.projects && user.projects.some(p => p.role_code === 'tenant_admin')) return true;
  return false;
}

const resetUser = ref({})
const newPassword = ref('')

const resetFilters = () => {
  filterProjectId.value = ''
  filterRoleId.value = ''
  changePage(1)
}

const loadUsers = async () => {
  loading.value = true
  try {
    const params = { page: page.value, pageSize: pageSize.value }
    if (filterProjectId.value) params.project_id = filterProjectId.value
    if (filterRoleId.value) params.role_id = filterRoleId.value

    const res = await axios.get('/api/users', { params })
    if (res.data.code === 0) {
      users.value = res.data.data || []
      total.value = res.data.total || 0
    }
  } catch (error) {
    console.error("Failed to load users:", error)
  } finally {
    loading.value = false
  }
}

const loadAllRoles = async () => {
  try {
    const res = await axios.get('/api/roles', { params: { include_builtin: 1 } })
    if (res.data.code === 0) {
      allRoles.value = res.data.data || []
    }
  } catch (e) {
    console.error("Failed to load roles:", e)
  }
}

const loadAllProjects = async () => {
  try {
    const res = await axios.get('/api/auth/projects')
    if (res.data.code === 0) {
      allProjects.value = res.data.data || []
    }
  } catch (e) {
    console.error("Failed to load projects:", e)
  }
}

const changePage = (p) => {
  page.value = p
  loadUsers()
}

onMounted(() => {
  userModal = new Modal(userModalRef.value)
  resetPasswordModal = new Modal(resetPasswordModalRef.value)
  rolesModal = new Modal(rolesModalRef.value)
  userDetailsModal = new Modal(userDetailsModalRef.value)

  loadUsers()
  loadAllRoles()
  loadAllProjects()
})

const openCreateModal = () => {
  isEditing.value = false
  form.value = {
    id: 0,
    username: '',
    password: '',
    confirm_password: '',
    display_name: '',
    email: '',
    role: ''
  }
  userModal.show()
}

const openEditModal = async (user) => {
  isEditing.value = true
  form.value = {
    id: user.id,
    username: user.username,
    display_name: user.display_name,
    email: user.email,
    role: user.role
  }
  userModal.show()
}

const saveUser = async () => {
  if (!isEditing.value && form.value.password !== form.value.confirm_password) {
    alert(t('auth_password_mismatch', '两次输入的密码不一致！'))
    return
  }
  try {
    let res
    if (isEditing.value) {
      res = await axios.put(`/api/users/${form.value.id}`, form.value)
    } else {
      res = await axios.post('/api/users', form.value)
      if (res.data.code === 0 && res.data.data?.existing) {
         // User already exists in tenant. We can still let them proceed or just ignore.
         // Actually, binding logic is moved to list.
      }
    }

    if (res.data.code === 0) {
      userModal.hide()
      loadUsers()
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(t('common_save_failed', '保存失败'))
  }
}

const deleteUser = async (user) => {
  if (confirm(t('user_delete_confirm', { username: user.username }))) {
    try {
      const res = await axios.delete(`/api/users/${user.id}`)
      if (res.data.code === 0) {
        loadUsers()
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert(t('user_delete_failed'))
    }
  }
}

const openResetPasswordModal = (user) => {
  resetUser.value = user
  newPassword.value = ''
  resetPasswordModal.show()
}

const resetPassword = async () => {
  if (!newPassword.value) return
  
  try {
    const res = await axios.post(`/api/users/${resetUser.value.id}/reset-password`, { new_password: newPassword.value })
    if (res.data.code === 0) {
      resetPasswordModal.hide()
      alert(t('user_reset_success'))
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(t('user_reset_failed'))
  }
}

const openRolesModal = async (user) => {
  currentUserToAssign.value = user
  unifiedRoleAssignments.value = []
  try {
    const resRoles = await axios.get(`/api/users/${user.id}/roles`)
    if (resRoles.data.code === 0) {
      const roleIds = resRoles.data.data || []
      roleIds.forEach(rId => {
        unifiedRoleAssignments.value.push({ project_id: 0, role_id: rId })
      })
    }
    const resProj = await axios.get(`/api/users/${user.id}/projects`)
    if (resProj.data.code === 0) {
      const projs = resProj.data.data || []
      projs.forEach(p => {
        unifiedRoleAssignments.value.push({ project_id: p.project_id, role_id: p.role_id })
      })
    }
    rolesModal.show()
  } catch (e) {
    alert(t('user_load_roles_failed', '加载角色信息失败'))
  }
}

const saveRoles = async () => {
  try {
    for (let i = 0; i < unifiedRoleAssignments.value.length; i++) {
      const pr = unifiedRoleAssignments.value[i]
      if (pr.project_id === null || !pr.role_id) {
        alert(t('user_project_role_required', '请完整选择项目和对应的角色。'))
        return
      }
    }

    const tenantRoleIds = unifiedRoleAssignments.value
      .filter(x => x.project_id === 0)
      .map(x => x.role_id)
    
    const specificProjects = unifiedRoleAssignments.value
      .filter(x => x.project_id !== 0)
      .map(x => ({ project_id: x.project_id, role_id: x.role_id }))

    const resRoles = authStore.user?.is_tenant_admin === true
      ? await axios.put(`/api/users/${currentUserToAssign.value.id}/roles`, {
          role_ids: tenantRoleIds
        })
      : { data: { code: 0 } }
    const resProj = await axios.put(`/api/users/${currentUserToAssign.value.id}/projects`, {
      projects: specificProjects
    })
    
    if (resRoles.data.code === 0 && resProj.data.code === 0) {
      rolesModal.hide()
      loadUsers()
    } else {
      alert("保存失败")
    }
  } catch (e) {
    alert(t('user_save_roles_failed', '保存角色分配失败'))
  }
}


</script>
