<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('position_management', '岗位管理') }}</h2>
      <button class="btn btn-primary btn-sm" @click="openCreateModal">
        <i class="bi bi-person-badge me-1"></i> {{ $t('position_add', '新增岗位') }}
      </button>
    </div>

    <!-- Positions Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ $t('role_code', '标识码') }}</th>
                <th>{{ $t('role_name', '名称') }}</th>
                <th>{{ $t('role_description', '描述') }}</th>
                <th>{{ $t('user_created_at', '创建时间') }}</th>
                <th class="text-end">{{ $t('role_actions', '操作') }}</th>
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
              <tr v-else-if="positions.length === 0">
                <td colspan="5" class="text-center py-4 text-muted">{{ $t('position_no_data', '暂无岗位数据') }}</td>
              </tr>
              <tr v-for="p in positions" :key="p.ID" v-else>
                <td><strong>{{ p.code }}</strong></td>
                <td>{{ p.name }}</td>
                <td>{{ p.description }}</td>
                <td>{{ new Date(p.CreatedAt).toLocaleString() }}</td>
                <td class="text-end">
                  <button class="btn btn-sm btn-outline-info me-2" @click="openRolesModal(p)" :title="$t('position_assign_roles', '分配角色')">
                    <i class="bi bi-shield-lock"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-primary me-2" @click="openEditModal(p)">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-danger" @click="deletePosition(p)">
                    <i class="bi bi-trash"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Position Modal -->
    <div class="modal fade" id="positionModal" tabindex="-1" ref="positionModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? $t('position_edit', '编辑岗位') : $t('position_add', '新增岗位') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="savePosition">
              <div class="mb-3">
                <label class="form-label">{{ $t('role_code', '标识码') }}</label>
                <input v-model="form.code" type="text" class="form-control" :disabled="isEditing" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('role_name', '名称') }}</label>
                <input v-model="form.name" type="text" class="form-control" required>
              </div>
              <div class="mb-3">
                <label class="form-label">{{ $t('role_description', '描述') }}</label>
                <textarea v-model="form.description" class="form-control" rows="2"></textarea>
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('role_cancel', '取消') }}</button>
            <button type="button" class="btn btn-primary" @click="savePosition">{{ $t('role_save', '保存') }}</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Assign Roles Modal -->
    <div class="modal fade" id="rolesModal" tabindex="-1" ref="rolesModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('position_assign_roles', '分配角色') }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
             <div v-for="r in allRoles" :key="r.ID" class="form-check">
               <input class="form-check-input" type="checkbox" :value="r.ID" v-model="selectedRoleIds" :id="'role'+r.ID">
               <label class="form-check-label" :for="'role'+r.ID">
                 {{ r.name }} ({{ r.code }})
               </label>
             </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">{{ $t('role_cancel', '取消') }}</button>
            <button type="button" class="btn btn-primary" @click="saveRoles">{{ $t('position_save_assign', '保存分配') }}</button>
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
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const positions = ref([])
const allRoles = ref([])
const loading = ref(false)

const positionModalRef = ref(null)
const rolesModalRef = ref(null)
let positionModal = null
let rolesModal = null

const isEditing = ref(false)
const currentPosId = ref(0)
const selectedRoleIds = ref([])

const form = ref({
  id: 0,
  code: '',
  name: '',
  description: ''
})

const loadPositions = async () => {
  loading.value = true
  try {
    const res = await axios.get('/api/positions')
    if (res.data.code === 0) {
      positions.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load positions:", error)
  } finally {
    loading.value = false
  }
}

const loadRoles = async () => {
  try {
    const res = await axios.get('/api/roles')
    if (res.data.code === 0) {
      allRoles.value = res.data.data || []
    }
  } catch (error) {
    console.error("Failed to load roles:", error)
  }
}

onMounted(() => {
  positionModal = new Modal(positionModalRef.value)
  rolesModal = new Modal(rolesModalRef.value)
  loadPositions()
  loadRoles()
})

const openCreateModal = () => {
  isEditing.value = false
  form.value = { id: 0, code: '', name: '', description: '' }
  positionModal.show()
}

const openEditModal = (item) => {
  isEditing.value = true
  form.value = { id: item.ID, code: item.code, name: item.name, description: item.description }
  positionModal.show()
}

const savePosition = async () => {
  try {
    let res
    if (isEditing.value) {
      res = await axios.put(`/api/positions/${form.value.id}`, form.value)
    } else {
      res = await axios.post('/api/positions', form.value)
    }
    
    if (res.data.code === 0) {
      positionModal.hide()
      loadPositions()
    } else {
      alert($t('common_save_failed', '保存失败'))
    }
  } catch (error) {
    alert($t('common_save_failed', '保存失败'))
  }
}

const deletePosition = async (item) => {
  if (confirm($t('position_delete_confirm', '确定要删除岗位 {name} 吗？').replace('{name}', item.name))) {
    try {
      const res = await axios.delete(`/api/positions/${item.ID}`)
      if (res.data.code === 0) {
        loadPositions()
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert($t('common_delete_failed', '删除失败'))
    }
  }
}

const openRolesModal = async (item) => {
  currentPosId.value = item.ID
  try {
    const res = await axios.get(`/api/positions/${item.ID}/roles`)
    if (res.data.code === 0) {
      selectedRoleIds.value = res.data.data || []
      rolesModal.show()
    }
  } catch (e) {
    alert($t('position_load_roles_failed', '加载角色分配失败'))
  }
}

const saveRoles = async () => {
  try {
    const res = await axios.put(`/api/positions/${currentPosId.value}/roles`, {
      role_ids: selectedRoleIds.value
    })
    if (res.data.code === 0) {
      rolesModal.hide()
    } else {
      alert(res.data.message)
    }
  } catch (e) {
    alert($t('position_save_roles_failed', '保存角色分配失败'))
  }
}
</script>
