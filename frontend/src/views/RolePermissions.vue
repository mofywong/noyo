<template>
  <div class="modal fade" id="permissionModal" tabindex="-1" ref="modalRef" data-bs-backdrop="static" data-bs-keyboard="false">
    <div class="modal-dialog modal-xl modal-dialog-scrollable">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">{{ $t('role_config_perm', '配置权限') }} - {{ role?.name }}</h5>
          <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
        </div>
        <div class="modal-body bg-light">
          <div v-if="loading" class="text-center py-5">
            <div class="spinner-border text-primary" role="status"></div>
            <div class="mt-2 text-muted">{{ $t('common_loading', '加载中...') }}</div>
          </div>
          <div v-else>
            <!-- 权限双模组件 -->
            <PermissionDualMode 
              :allPermissions="allPermissions"
              v-model="formPermissionIds"
              :isReadOnly="isFunctionPermissionReadOnly"
              :title="$t('role_menu_perm', '菜单权限')"
            />

            <!-- ================= 设备标签权限 ================= -->
            <div v-if="showDeviceTagPermissions" class="mt-4 pt-3 border-top">
              <div class="fw-bold fs-5 text-primary mb-3">
                <i class="bi bi-tags-fill me-2"></i>{{ $t('role_device_tag_perm', '设备标签权限') }}
              </div>
              <div v-if="requiresProjectContext" class="alert alert-warning d-flex align-items-center gap-2 mb-3">
                <i class="bi bi-exclamation-triangle"></i>
                <span>{{ $t('role_select_project_for_tag_perm', 'Select a project before configuring inherited-role device data permissions.') }}</span>
              </div>
              <div v-if="loadingTags && !requiresProjectContext" class="text-center py-3">{{ $t('common_loading', '加载中...') }}</div>
              <div class="card shadow-sm border-0" v-else-if="!requiresProjectContext">
                <div class="table-responsive">
                  <table class="table table-hover mb-0 align-middle">
                    <thead class="table-light">
                      <tr>
                        <th>{{ $t('device_tag_name', '标签名称') }}</th>
                        <th>{{ $t('role_scope', '作用域') }}</th>
                        <th style="width: 200px">{{ $t('role_permission_assign', '权限分配') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-if="tags.length === 0">
                        <td colspan="3" class="text-center text-muted py-4">{{ $t('role_no_device_tags', '暂无设备标签') }}</td>
                      </tr>
                      <tr v-for="t in tags" :key="t.ID" v-else>
                        <td>
                          <i :class="['bi', t.icon, 'me-2']" :style="{color: t.color}"></i>
                          {{ t.name }}
                        </td>
                        <td>
                          <span class="badge bg-secondary bg-opacity-10 text-secondary">{{ t.scope_type === 'global' ? $t('common_global', '全局') : t.scope_type }}</span>
                        </td>
                        <td>
                          <select class="form-select form-select-sm" v-model="tagPermissions[t.ID]" :disabled="isDeviceTagPermissionReadOnly || requiresProjectContext">
                            <option value="">{{ $t('role_perm_none', '无权限') }}</option>
                            <option value="read">{{ $t('role_perm_read', '只读') }}</option>
                            <option value="write">{{ $t('role_perm_write', '读写 (控制)') }}</option>
                          </select>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </div>
              <div class="text-muted small mt-2">
                <i class="bi bi-info-circle me-1"></i> {{ $t('role_tag_perm_tip', '提示：未配置标签权限时，默认无权限访问该标签下属设备；若设备本身无标签，则按系统默认规则处理。') }}
              </div>
            </div>
          </div>
        </div>
        
        <div class="modal-footer bg-light border-top-0">
          <button type="button" class="btn btn-secondary px-4" data-bs-dismiss="modal">{{ $t('common_cancel', '取消') }}</button>
          <button type="button" class="btn btn-primary px-4" @click="save" :disabled="saving || !canSave">
            <span v-if="saving" class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>
            <i v-else class="bi bi-check2 me-1"></i>
            {{ $t('common_save', '保存') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import axios from 'axios'
import { Modal } from 'bootstrap'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { isInheritedRoleReadOnlyForUser } from '../utils/authIdentity'
import PermissionDualMode from '../components/PermissionDualMode.vue'

const emit = defineEmits(['saved'])
const { t } = useI18n()
const authStore = useAuthStore()

const modalRef = ref(null)
let modalInstance = null

const role = ref(null)
const loading = ref(false)
const loadingTags = ref(false)
const saving = ref(false)

const allPermissions = ref([])
const formPermissionIds = ref([])

const tags = ref([])
const tagPermissions = ref({})
const activeProjectId = ref(0)
const isInheritedProjectRole = computed(() => role.value?.project_id === 0 && role.value?.is_inherited === true)
const deviceTagProjectId = computed(() => {
  if (role.value?.project_id > 0) return Number(role.value.project_id)
  if (isInheritedProjectRole.value) return Number(activeProjectId.value || 0)
  return 0
})
const requiresProjectContext = computed(() => isInheritedProjectRole.value && deviceTagProjectId.value <= 0)
const showDeviceTagPermissions = computed(() => authStore.user?.is_tenant_admin !== true)
const isFunctionPermissionReadOnly = computed(() => {
  if (isInheritedProjectRole.value && deviceTagProjectId.value > 0) return true
  return isInheritedRoleReadOnlyForUser(authStore.user, role.value)
})
const isDeviceTagPermissionReadOnly = computed(() => {
  if (!showDeviceTagPermissions.value) return true
  return !role.value
})
const canSave = computed(() => {
  if (!role.value) return false
  if (!isFunctionPermissionReadOnly.value) return true
  return showDeviceTagPermissions.value && !isDeviceTagPermissionReadOnly.value && !requiresProjectContext.value
})

const initModal = () => {
  if (!modalInstance && modalRef.value) {
    modalInstance = new Modal(modalRef.value)
  }
}

const open = async (roleItem) => {
  initModal()
  role.value = roleItem
  activeProjectId.value = Number(localStorage.getItem('current_project_id') || 0)
  formPermissionIds.value = []
  tagPermissions.value = {}
  
  modalInstance.show()
  
  const loaders = [loadSystemPermissions()]
  if (showDeviceTagPermissions.value) {
    loaders.push(loadDeviceTags())
  }
  await Promise.all(loaders)
  
  await loadRolePermissions(roleItem.ID)
}

const loadSystemPermissions = async () => {
  loading.value = true
  try {
    const params = {}
    if (role.value?.project_id > 0) {
      params.project_id = role.value.project_id
    }
    const res = await axios.get('/api/permissions', { params })
    if (res.data.code === 0) {
      const excludedCodes = ['tenant:transfer'] 
      allPermissions.value = (res.data.data || []).filter(p => !excludedCodes.includes(p.code))
    }
  } catch(e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

const loadDeviceTags = async () => {
  loadingTags.value = true
  try {
    const res = await axios.get('/api/device-tags')
    if (res.data.code === 0) {
      tags.value = res.data.data || []
      tags.value.forEach(tag => {
        tagPermissions.value[tag.ID] = ''
      })
    }
  } catch(e) {
    console.error(e)
  } finally {
    loadingTags.value = false
  }
}

const loadRolePermissions = async (roleId) => {
  try {
    const params = {}
    if (deviceTagProjectId.value > 0) {
      params.project_id = deviceTagProjectId.value
    }
    const res = await axios.get(`/api/roles/${roleId}/permissions`, { params })
    if (res.data.code === 0) {
      const data = res.data.data
      if (data.permissions) {
        formPermissionIds.value = data.permissions.map(p => p.permission_id)
      }
      if (data.device_tags) {
        data.device_tags.forEach(dt => {
          tagPermissions.value[dt.tag_id] = dt.permission
        })
      }
    }
  } catch(e) {
    console.error(e)
  }
}

const save = async () => {
  if (!role.value || !canSave.value) return
  saving.value = true
  
  const deviceTagsPayload = []
  if (showDeviceTagPermissions.value) {
    for (const [tagId, perm] of Object.entries(tagPermissions.value)) {
      if (perm) {
        deviceTagsPayload.push({ tag_id: parseInt(tagId), permission: perm })
      }
    }
  }
  
  try {
    const payload = {
      project_id: deviceTagProjectId.value
    }
    if (!isFunctionPermissionReadOnly.value) {
      payload.permission_ids = formPermissionIds.value
    }
    if (showDeviceTagPermissions.value) {
      payload.device_tags = deviceTagsPayload
    }
    const res = await axios.put(`/api/roles/${role.value.ID}/permissions`, payload)
    
    if (res.data.code === 0) {
      modalInstance.hide()
      emit('saved')
    } else {
      alert(res.data.message)
    }
  } catch(e) {
    alert(t('common_save_failed', '保存失败'))
  } finally {
    saving.value = false
  }
}

defineExpose({ open })
</script>

<style scoped>
</style>
