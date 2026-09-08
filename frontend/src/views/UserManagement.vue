<template>
  <div class="user-management-page page-fixed-height">
    <div class="page-header list-page-header">
      <div>
        <h1>{{ $t('user_management', '用户管理') }}</h1>
        <p class="page-subtitle">{{ $t('user_management_subtitle', '管理系统用户、项目归属与权限角色分配') }}</p>
      </div>
      <LiquidGlassButton
        variant="primary"
        icon="bi bi-person-plus"
        @click="openCreateModal"
        v-permission="'user:create'"
      >
        {{ $t('user_add') }}
      </LiquidGlassButton>
    </div>

    <div class="page-toolbar device-list-control-bar list-page-controls">
      <select v-model="filterProjectId" class="form-select form-select-sm list-toolbar-filters" @change="changePage(1)" style="min-width: 160px; max-width: 220px;">
        <option value="">{{ $t('project_all', '全部项目') }}</option>
        <option v-for="p in allProjects" :key="p.ID" :value="p.ID">{{ p.name }}</option>
      </select>
      <select v-model="filterRoleId" class="form-select form-select-sm list-toolbar-filters" @change="changePage(1)" style="min-width: 160px; max-width: 220px;">
        <option value="">{{ $t('role_all', '全部角色') }}</option>
        <option v-for="r in allRoles" :key="r.ID" :value="r.ID">{{ r.name }}</option>
      </select>
      <CompactListMetrics v-model="metricFilters" :metrics="metricCards" class="list-toolbar-metrics" :aria-label="$t('stat_total', '用户统计')" />
      <div class="list-toolbar-actions">
        <LiquidGlassButton
          variant="outline-secondary"
          size="sm"
          @click="resetFilters"
        >
          {{ $t('common_reset', '重置') }}
        </LiquidGlassButton>
      </div>
    </div>

    <!-- Users Table -->
    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4">{{ $t('auth_username') }}</th>
                <th>{{ $t('user_name', '姓名') }}</th>
                <th>{{ $t('user_permissions_assign', '权限分配 (项目与角色)') }}</th>
                <th>{{ $t('user_last_login') }}</th>
                <th class="text-end pe-4">{{ $t('user_actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <!-- 骨架屏 -->
              <tr v-if="loading" v-for="n in 5" :key="'sk-' + n">
                <td class="ps-4">
                  <div class="d-flex align-items-center gap-2">
                    <div class="skeleton rounded-circle" style="width: 32px; height: 32px;"></div>
                    <div>
                      <div class="skeleton" style="height: 16px; width: 100px;"></div>
                      <div class="skeleton mt-1" style="height: 12px; width: 140px;"></div>
                    </div>
                  </div>
                </td>
                <td><div class="skeleton" style="height: 16px; width: 80px;"></div></td>
                <td><div class="skeleton" style="height: 22px; width: 160px; border-radius: 999px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 130px;"></div></td>
                <td class="text-end pe-4"><div class="skeleton ms-auto" style="height: 28px; width: 80px;"></div></td>
              </tr>
              <!-- 空状态 -->
              <tr v-else-if="filteredUsers.length === 0">
                <td colspan="5" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-people"></i>
                    <p>{{ $t('user_no_users_found', '未找到匹配的用户数据') }}</p>
                    <LiquidGlassButton variant="primary" icon="bi bi-person-plus" @click="openCreateModal" v-permission="'user:create'">
                      {{ $t('user_add', '添加用户') }}
                    </LiquidGlassButton>
                  </div>
                </td>
              </tr>
              <tr v-for="user in filteredUsers" :key="user.id">
                <td class="ps-4">
                  <div class="d-flex align-items-center">
                    <div class="avatar-sm rounded-circle text-primary me-2 d-flex align-items-center justify-content-center fw-bold" style="width: 32px; height: 32px; background: var(--bg-surface); border: 1px solid var(--border-color);">
                      {{ (user.username || 'U').substring(0, 2).toUpperCase() }}
                    </div>
                    <div>
                      <div class="fw-medium">{{ user.username }}</div>
                      <small class="text-muted" v-if="user.email">{{ user.email }}</small>
                    </div>
                  </div>
                </td>
                <td>{{ user.display_name || '-' }}</td>
                <td>
                  <div class="d-flex flex-wrap gap-1 align-items-center">
                    <template v-if="user.tenant_roles && user.tenant_roles.length > 0">
                      <span v-for="role in user.tenant_roles" :key="'tr-'+role.role_id" class="spec-badge spec-badge--primary">
                        <i class="bi bi-globe me-1"></i>{{ isGatewayMode && (role.role_code === 'tenant_admin' || role.role_code === 'super_admin' || role.role_code === 'gateway_admin') ? $t('role_super_admin', '超级管理员') : role.role_name }}
                      </span>
                    </template>
                    <template v-if="!(isGatewayMode && user.tenant_roles?.some(r => ['tenant_admin', 'super_admin', 'gateway_admin'].includes(r.role_code)))">
                      <template v-if="user.projects && user.projects.length > 0">
                        <span v-for="proj in getGroupedProjects(user.projects)" :key="'p-'+proj.name" class="spec-badge spec-badge--info">
                          <i class="bi bi-folder me-1"></i>{{ proj.name }}: {{ proj.roles.join(', ') }}
                        </span>
                      </template>
                    </template>
                    <span v-if="!hasAssignedRoles(user)" class="text-muted small">
                      {{ $t('user_no_permissions', '未分配权限') }}
                    </span>
                  </div>
                </td>
                <td>
                  <span v-if="user.last_login_at" class="text-muted small">{{ formatDateTime(user.last_login_at) }}</span>
                  <span v-else class="spec-badge spec-badge--neutral">{{ $t('user_never') }}</span>
                </td>
                <td class="text-end pe-4">
                  <div class="table-actions">
                    <button class="table-action-btn" @click="openDetailsModal(user)" :title="$t('common_view_details', '查看详情')">
                      <i class="bi bi-eye"></i>
                    </button>
                    <button class="table-action-btn" :class="hasAssignedRoles(user) ? 'table-action-btn--info' : ''" @click="openRolesModal(user)" :title="$t('user_assign_roles', '分配角色')" :disabled="isRoleModificationDisabled(user)" v-permission="'user:edit'">
                      <i class="bi bi-shield-lock"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--warning" @click="openResetPasswordModal(user)" :title="$t('reset_password', '重置密码')" v-permission="'user:edit'">
                      <i class="bi bi-key"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--primary" @click="openEditModal(user)" :title="$t('user_edit', '编辑')" v-permission="'user:edit'">
                      <i class="bi bi-pencil"></i>
                    </button>
                    <button class="table-action-btn table-action-btn--danger" @click="deleteUser(user)" :disabled="isUserDeletionDisabled(user)" :title="$t('user_delete', '删除')" v-permission="'user:delete'">
                      <i class="bi bi-trash"></i>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="p-3 border-top d-flex justify-content-between align-items-center" v-if="total > pageSize">
          <span class="text-muted small">{{ $t('user_total_records', { total }) }}</span>
          <ListPagination
            :page="page"
            :page-size="pageSize"
            :total="total"
            :disabled="loading"
            id-prefix="user-management"
            @update:page="changePage"
            @update:page-size="changePageSize"
          />
        </div>
      </div>
    </div>

    <Teleport to="body">
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
              <div class="mb-3">
                <label class="form-label">{{ $t('auth_username') }} <span class="text-danger">*</span></label>
                <input v-model="form.username" type="text" class="form-control" :disabled="isEditing" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('user_name', '姓名') }} <span class="text-danger">*</span></label>
                <input v-model="form.display_name" type="text" class="form-control" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('user_phone', '电话/联系方式') }}</label>
                <input v-model="form.email" type="text" class="form-control">
              </div>
              <div class="row" v-if="!isEditing">
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('auth_password') }} <span class="text-danger">*</span></label>
                  <input v-model="form.password" type="password" class="form-control" required>
                </div>
                <div class="col-md-6 mb-3">
                  <label class="form-label">{{ $t('auth_password_confirm', '确认密码') }} <span class="text-danger">*</span></label>
                  <input v-model="form.confirm_password" type="password" class="form-control" required>
                </div>
              </div>

            </form>
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ $t('user_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveUser">{{ $t('user_save') }}</LiquidGlassButton>
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
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ $t('user_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="danger" @click="resetPassword">{{ $t('user_reset_password') }}</LiquidGlassButton>
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
            <LiquidGlassButton variant="secondary" data-bs-dismiss="modal">{{ $t('user_cancel') }}</LiquidGlassButton>
            <LiquidGlassButton variant="primary" @click="saveRoles">{{ $t('user_save_assign', '保存分配') }}</LiquidGlassButton>
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
            <div v-if="currentUserDetails" class="user-details-card">
              <div class="p-4 text-center border-bottom user-details-header">
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
                    <div class="fw-medium">{{ currentUserDetails.last_login_at ? formatDateTime(currentUserDetails.last_login_at) : $t('user_never') }}</div>
                  </div>
                  <div class="col-6">
                    <label class="text-muted small mb-1">{{ $t('user_created_at') }}</label>
                    <div class="fw-medium">{{ formatDateTime(currentUserDetails.created_at) }}</div>
                  </div>
                </div>
                
                <h6 class="border-bottom pb-2 mb-3"><i class="bi bi-shield-check text-primary me-2"></i>{{ $t('user_permission_details', '权限分配明细') }}</h6>
                
                <div class="card border-0 shadow-sm mb-3">
                  <div class="card-body p-3">
                    <div class="text-muted small mb-2 fw-bold">{{ $t('user_global_roles', '全局角色') }}</div>
                    <div v-if="currentUserDetails.tenant_roles && currentUserDetails.tenant_roles.length > 0" class="d-flex flex-wrap gap-2">
                      <span v-for="roleName in Array.from(new Set(currentUserDetails.tenant_roles.map(r => isGatewayMode && (r.role_code === 'tenant_admin' || r.role_code === 'super_admin' || r.role_code === 'gateway_admin') ? $t('role_super_admin', '超级管理员') : r.role_name)))" :key="roleName" class="spec-badge spec-badge--primary px-3 py-2">
                        <i class="bi bi-globe me-1"></i> {{ roleName }}
                      </span>
                    </div>
                    <div v-else class="text-muted small">{{ $t('user_no_global_roles', '无全局角色') }}</div>
                  </div>
                </div>

                <template v-if="!(isGatewayMode && currentUserDetails.tenant_roles?.some(r => ['tenant_admin', 'super_admin', 'gateway_admin'].includes(r.role_code)))">
                  <div class="card border-0 shadow-sm">
                    <div class="card-body p-3">
                      <div class="text-muted small mb-2 fw-bold">{{ $t('user_project_permissions', '项目权限') }}</div>
                      <div v-if="currentUserDetails.projects && currentUserDetails.projects.length > 0">
                        <ul class="list-group list-group-flush">
                          <li v-for="p in getGroupedProjects(currentUserDetails.projects)" :key="p.name" class="list-group-item px-0 d-flex justify-content-between align-items-center bg-transparent">
                            <span><i class="bi bi-folder text-info me-2"></i>{{ p.name }}</span>
                            <span class="text-end">
                              <span v-for="role in p.roles" :key="role" class="spec-badge spec-badge--info ms-1">{{ role }}</span>
                            </span>
                          </li>
                        </ul>
                      </div>
                      <div v-else class="text-muted small">{{ $t('user_no_project_permissions', '无项目权限') }}</div>
                    </div>
                  </div>
                </template>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <LiquidGlassButton variant="danger" data-bs-dismiss="modal">{{ $t('common_close', '关闭') }}</LiquidGlassButton>
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
import { useAuthStore } from '../stores/auth'
import { useI18n } from 'vue-i18n'
import { isSingleProjectMode } from '../utils/systemMode'
import ListPagination from '../components/ListPagination.vue'
import { formatDateTime } from '../utils/dateTime.js'
import CompactListMetrics from '../components/CompactListMetrics.vue'

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

const adminCount = computed(() => {
  return users.value.filter(u => u.tenant_roles?.some(r => ['tenant_admin', 'super_admin', 'gateway_admin', 'admin'].includes(r.role_code))).length
})
const projectMembersCount = computed(() => {
  return users.value.filter(u => u.projects && u.projects.length > 0).length
})
const metricCards = computed(() => [
  { key: 'total', label: t('stat_total', '总用户数'), value: total.value || users.value.length, icon: 'bi-people-fill', tone: 'brand' },
  { key: 'admin', label: t('stat_admin_users', '管理权限'), value: adminCount.value, icon: 'bi-shield-check', tone: 'success' },
  { key: 'projectMember', label: t('stat_project_members', '项目成员'), value: projectMembersCount.value, icon: 'bi-person-badge', tone: 'neutral' }
])
const metricFilters = ref([])
const filteredUsers = computed(() => users.value.filter((user) => {
  if (!metricFilters.value.length) return true
  const isAdmin = user.tenant_roles?.some((role) => ['tenant_admin', 'super_admin', 'gateway_admin', 'admin'].includes(role.role_code))
  const isProjectMember = Boolean(user.projects?.length)
  return (metricFilters.value.includes('admin') && isAdmin)
    || (metricFilters.value.includes('projectMember') && isProjectMember)
}))
const toggleMetricFilter = (key) => {
  metricFilters.value = metricFilters.value.includes(key)
    ? metricFilters.value.filter((item) => item !== key)
    : [...metricFilters.value, key]
}

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

const isGatewayMode = computed(() => {
  return isSingleProjectMode(localStorage.getItem('system_mode') || '');
});

const getPermissionsSummary = (user) => {
  const items = [];
  const addedTexts = new Set();
  let isSuperAdmin = false;
  if (user.tenant_roles) {
    const tenantName = authStore.user?.tenant_name || localStorage.getItem('tenant_name') || '租户';
    user.tenant_roles.forEach(r => {
      let roleName = r.role_name;
      if (isGatewayMode.value && (r.role_code === 'tenant_admin' || r.role_code === 'super_admin' || r.role_code === 'gateway_admin')) {
        roleName = t('role_super_admin', '超级管理员');
        isSuperAdmin = true;
      }
      const text = isGatewayMode.value ? roleName : `${tenantName} - ${roleName}`;
      if (!addedTexts.has(text)) {
        addedTexts.add(text);
        items.push({ text, type: 'primary' });
      }
    });
  }
  
  if (isGatewayMode.value && isSuperAdmin) {
    return items;
  }
  const grouped = getGroupedProjects(user.projects);
  grouped.forEach(p => {
    const text = isGatewayMode.value ? p.roles.join('、') : `${p.name} - ${p.roles.join('、')}`;
    if (!addedTexts.has(text)) {
      addedTexts.add(text);
      items.push({ text, type: 'info' });
    }
  });
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
  if (user.tenant_roles && user.tenant_roles.some(r => r.role_code === 'super_admin' || r.role_code === 'tenant_admin')) {
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
  if (p < 1 || p > Math.max(1, Math.ceil(total.value / pageSize.value))) return
  page.value = p
  loadUsers()
}

const changePageSize = (size) => {
  pageSize.value = Number(size) || 10
  page.value = 1
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

<style scoped>
.user-details-card {
  background: var(--bg-surface);
  color: var(--text-main);
}
.user-details-header {
  background: var(--bg-drawer-header);
  border-color: var(--border-color) !important;
}
</style>

