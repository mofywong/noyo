<template>
  <div class="modal fade" id="permissionModal" tabindex="-1" ref="modalRef" data-bs-backdrop="static" data-bs-keyboard="false">
    <div class="modal-dialog modal-lg">
      <div class="modal-content">
        <div class="modal-header">
          <h5 class="modal-title">{{ $t('role_config_perm', '配置权限') }} - {{ role?.name }}</h5>
          <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
        </div>
        <div class="modal-body">
          <ul class="nav nav-tabs mb-3" id="permissionTab" role="tablist">
            <li class="nav-item" role="presentation">
              <button class="nav-link active" id="menu-tab" data-bs-toggle="tab" data-bs-target="#menu-pane" type="button" role="tab">{{ $t('role_menu_perm', '菜单权限') }}</button>
            </li>
            <li class="nav-item" role="presentation">
              <button class="nav-link" id="device-tab" data-bs-toggle="tab" data-bs-target="#device-pane" type="button" role="tab">{{ $t('role_device_tag_perm', '设备标签权限') }}</button>
            </li>
          </ul>
          
          <div class="tab-content" id="permissionTabContent">
            <!-- 菜单权限 -->
            <div class="tab-pane fade show active" id="menu-pane" role="tabpanel">
              <div v-if="loading" class="text-center py-3">{{ $t('common_loading', '加载中...') }}</div>
              <div v-else>
                <div class="d-flex justify-content-end mb-3 gap-2">
                  <button class="btn btn-sm btn-outline-primary" @click="selectAllPerms" :disabled="isReadOnly">{{ $t('common_select_all', '全选') }}</button>
                  <button class="btn btn-sm btn-outline-secondary" @click="invertAllPerms" :disabled="isReadOnly">{{ $t('common_invert_selection', '反选') }}</button>
                </div>
                <div class="row">
                  <div class="col-md-4 mb-3" v-for="(perms, module) in groupedPermissions" :key="module">
                    <div class="card h-100 shadow-sm border-0">
                      <div class="card-header bg-light fw-bold text-uppercase d-flex justify-content-between align-items-center">
                        <span>{{ translateModule(module) }}</span>
                        <div>
                          <button class="btn btn-sm btn-link text-primary p-0 text-decoration-none me-2" @click="selectAllModule(perms)" :disabled="isReadOnly">{{ $t('common_select_all', '全选') }}</button>
                          <button class="btn btn-sm btn-link text-secondary p-0 text-decoration-none" @click="invertModule(perms)" :disabled="isReadOnly">{{ $t('common_invert_selection', '反选') }}</button>
                        </div>
                      </div>
                    <ul class="list-group list-group-flush">
                      <li class="list-group-item" v-for="p in perms" :key="p.ID">
                        <div class="form-check">
                          <input class="form-check-input" type="checkbox" :value="p.ID" :id="'perm'+p.ID" v-model="selectedPerms" :disabled="isReadOnly">
                          <label class="form-check-label" :for="'perm'+p.ID">
                            {{ p.name }} <span class="badge bg-secondary ms-1" style="font-size: 0.7em">{{ translateType(p.type) }}</span>
                          </label>
                        </div>
                      </li>
                    </ul>
                  </div>
                </div>
              </div>
              </div>
            </div>
            
            <!-- 设备标签权限 -->
            <div class="tab-pane fade" id="device-pane" role="tabpanel">
              <div v-if="loadingTags" class="text-center py-3">{{ $t('common_loading', '加载中...') }}</div>
              <table class="table table-bordered table-hover mt-2" v-else>
                <thead class="table-light">
                  <tr>
                    <th>{{ $t('device_tag_name', '标签名称') }}</th>
                    <th>{{ $t('role_scope', '作用域') }}</th>
                    <th style="width: 200px">{{ $t('role_permission_assign', '权限分配') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="tags.length === 0">
                    <td colspan="3" class="text-center text-muted">{{ $t('role_no_device_tags', '暂无设备标签') }}</td>
                  </tr>
                  <tr v-for="t in tags" :key="t.ID" v-else>
                    <td>
                      <i :class="['bi', t.icon, 'me-2']"></i>
                      {{ t.name }}
                    </td>
                    <td>{{ t.scope_type === 'global' ? $t('common_global', '全局') : t.scope_type }}</td>
                    <td>
                      <select class="form-select form-select-sm" v-model="tagPermissions[t.ID]" :disabled="isReadOnly">
                        <option value="">{{ $t('role_perm_none', '无权限') }}</option>
                        <option value="read">{{ $t('role_perm_read', '只读') }}</option>
                        <option value="write">{{ $t('role_perm_write', '读写 (控制)') }}</option>
                      </select>
                    </td>
                  </tr>
                </tbody>
              </table>
              <div class="alert alert-info mt-3" style="font-size: 0.9em">
                <i class="bi bi-info-circle me-1"></i> {{ $t('role_tag_perm_tip', '提示：未配置标签权限时，默认无权限访问该标签下属设备；若设备本身无标签，则按系统默认规则处理。') }}
              </div>
            </div>
          </div>
        </div>
        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('common_cancel', '取消') }}</button>
          <button type="button" class="btn btn-primary" @click="save" :disabled="saving || isReadOnly">
            <span v-if="saving" class="spinner-border spinner-border-sm me-1" role="status" aria-hidden="true"></span>
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
const selectedPerms = ref([])

const tags = ref([])
const tagPermissions = ref({}) // tag_id -> permission string
const isReadOnly = computed(() => isInheritedRoleReadOnlyForUser(authStore.user, role.value))

const initModal = () => {
  if (!modalInstance && modalRef.value) {
    modalInstance = new Modal(modalRef.value)
  }
}

const open = async (roleItem) => {
  initModal()
  role.value = roleItem
  selectedPerms.value = []
  tagPermissions.value = {}
  modalInstance.show()
  
  await Promise.all([
    loadSystemPermissions(),
    loadDeviceTags()
  ])
  
  await loadRolePermissions(roleItem.ID)
}

const loadSystemPermissions = async () => {
  loading.value = true
  try {
    const res = await axios.get('/api/permissions')
    if (res.data.code === 0) {
      allPermissions.value = res.data.data || []
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
    }
  } catch(e) {
    console.error(e)
  } finally {
    loadingTags.value = false
  }
}

const loadRolePermissions = async (roleId) => {
  try {
    const res = await axios.get(`/api/roles/${roleId}/permissions`)
    if (res.data.code === 0) {
      const data = res.data.data
      if (data.permissions) {
        selectedPerms.value = data.permissions.map(p => p.permission_id)
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
  if (!role.value) return
  if (isReadOnly.value) return
  saving.value = true
  
  const deviceTagsPayload = []
  for (const [tagId, perm] of Object.entries(tagPermissions.value)) {
    if (perm) {
      deviceTagsPayload.push({
        tag_id: parseInt(tagId),
        permission: perm
      })
    }
  }
  
  try {
    const res = await axios.put(`/api/roles/${role.value.ID}/permissions`, {
      permission_ids: selectedPerms.value,
      device_tags: deviceTagsPayload
    })
    
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

const groupedPermissions = computed(() => {
  const groups = {}
  allPermissions.value.forEach(p => {
    if (!groups[p.module]) {
      groups[p.module] = []
    }
    groups[p.module].push(p)
  })
  return groups
})

const selectAllPerms = () => {
  selectedPerms.value = allPermissions.value.map(p => p.ID)
}

const invertAllPerms = () => {
  const allIds = allPermissions.value.map(p => p.ID)
  selectedPerms.value = allIds.filter(id => !selectedPerms.value.includes(id))
}

const selectAllModule = (perms) => {
  perms.forEach(p => {
    if (!selectedPerms.value.includes(p.ID)) {
      selectedPerms.value.push(p.ID)
    }
  })
}

const invertModule = (perms) => {
  perms.forEach(p => {
    const idx = selectedPerms.value.indexOf(p.ID)
    if (idx > -1) {
      selectedPerms.value.splice(idx, 1)
    } else {
      selectedPerms.value.push(p.ID)
    }
  })
}

const translateModule = (module) => {
  const m = module ? module.toLowerCase() : ''
  const keyMap = {
    'tenant': '租户管理',
    'project': '项目管理',
    'system': '系统管理',
    'device': '设备管理',
    'device_tag': '设备标签',
    'gateway': '网关管理',
    'history': '历史记录',
    'rule': '规则引擎',
    'alarm': '告警中心',
    'auth': '认证授权',
    'product': '产品管理',
    'user': '用户管理',
    'role': '角色管理',
    'plugin': '插件管理',
    'audit': '审计日志',
    'app': '应用集成',
    'position': '岗位管理'
  }
  return t('perm_module_' + m, keyMap[m] || module)
}

const translateType = (type) => {
  const t_key = type ? type.toLowerCase() : ''
  const keyMap = {
    'menu': '菜单',
    'button': '按钮',
    'api': '接口',
    'data': '数据'
  }
  return t('perm_type_' + t_key, keyMap[t_key] || type)
}

defineExpose({
  open
})
</script>
