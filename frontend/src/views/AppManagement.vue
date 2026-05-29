<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('app_management', '应用接入') }}</h2>
      <div>
        <button class="btn btn-outline-info me-2" @click="goToGuide">
          <i class="bi bi-book me-1"></i> 接入指导
        </button>
        <button class="btn btn-primary" @click="openCreateModal" v-permission="'app:create'">
          <i class="bi bi-window-sidebar me-1"></i> 新增应用
        </button>
      </div>
    </div>

    <!-- Apps Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>AppID</th>
                <th>名称</th>
                <th>描述</th>
                <th>限流 (次/秒)</th>
                <th>状态</th>
                <th>创建时间</th>
                <th class="text-end">操作</th>
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
                <td colspan="7" class="text-center py-4 text-muted">暂无应用数据</td>
              </tr>
              <tr v-for="a in apps" :key="a.ID" v-else>
                <td><code class="text-primary">{{ a.app_id }}</code></td>
                <td>{{ a.name }}</td>
                <td>{{ a.description }}</td>
                <td>{{ a.rate_limit || '无限制' }}</td>
                <td>
                  <span class="badge" :class="a.status === 1 ? 'bg-success' : 'bg-danger'">
                    {{ a.status === 1 ? '正常' : '停用' }}
                  </span>
                </td>
                <td>{{ new Date(a.CreatedAt).toLocaleString() }}</td>
                <td class="text-end">
                  <button class="btn btn-sm btn-outline-warning me-2" @click="resetAppKey(a)" title="重置密钥" v-permission="'app:reset-key'">
                    <i class="bi bi-key"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-primary me-2" @click="openEditModal(a)" v-permission="'app:edit'">
                    <i class="bi bi-pencil"></i>
                  </button>
                  <button class="btn btn-sm btn-outline-danger" @click="deleteApp(a)" v-permission="'app:delete'">
                    <i class="bi bi-trash"></i>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- App Modal -->
    <div class="modal fade" id="appModal" tabindex="-1" ref="appModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ isEditing ? '编辑应用' : '新增应用' }}</h5>
            <button type="button" class="btn-close" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <form @submit.prevent="saveApp">
              <div class="mb-3">
                <label class="form-label">名称</label>
                <input v-model="form.name" type="text" class="form-control" required>
              </div>
              <div class="mb-3">
                <label class="form-label">描述</label>
                <textarea v-model="form.description" class="form-control" rows="2"></textarea>
              </div>
              <div class="mb-3">
                <label class="form-label">限流速率 (次/秒, 0代表无限制)</label>
                <input v-model.number="form.rate_limit" type="number" class="form-control" min="0">
              </div>
              <div class="mb-3 form-check" v-if="isEditing">
                <input v-model="form.status" type="checkbox" class="form-check-input" id="appStatus" :true-value="1" :false-value="0">
                <label class="form-check-label" for="appStatus">正常状态</label>
              </div>
            </form>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">取消</button>
            <button type="button" class="btn btn-primary" @click="saveApp">保存</button>
          </div>
        </div>
      </div>
    </div>

    <!-- Success Modal -->
    <div class="modal fade" id="appSuccessModal" tabindex="-1" ref="appSuccessModalRef">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header bg-success text-white">
            <h5 class="modal-title">应用创建成功</h5>
            <button type="button" class="btn-close btn-close-white" data-bs-dismiss="modal"></button>
          </div>
          <div class="modal-body">
            <div class="alert alert-warning">
              <i class="bi bi-exclamation-triangle-fill me-1"></i>
              请妥善保管您的 AppKey，它仅在创建时显示一次！
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold">AppID</label>
              <div class="input-group">
                <input type="text" class="form-control" readonly :value="newAppInfo.app_id">
                <button class="btn btn-outline-secondary" @click="copyToClipboard(newAppInfo.app_id)">
                  <i class="bi bi-clipboard"></i> 复制
                </button>
              </div>
            </div>
            <div class="mb-3">
              <label class="form-label fw-bold">AppKey</label>
              <div class="input-group">
                <input type="text" class="form-control" readonly :value="newAppInfo.AppKey">
                <button class="btn btn-outline-secondary" @click="copyToClipboard(newAppInfo.AppKey)">
                  <i class="bi bi-clipboard"></i> 复制
                </button>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-primary" data-bs-dismiss="modal">我知道了</button>
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

const router = useRouter()

const apps = ref([])
const loading = ref(false)

const appModalRef = ref(null)
let appModal = null

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

const goToGuide = () => {
  router.push('/settings/apps/guide')
}

const copyToClipboard = (text) => {
  navigator.clipboard.writeText(text).then(() => {
    alert('复制成功！')
  }).catch(() => {
    alert('复制失败，请手动复制。')
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
    console.error("Failed to load apps:", error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  appModal = new Modal(appModalRef.value)
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
  form.value = { id: item.ID, name: item.name, description: item.description, rate_limit: item.rate_limit, status: item.status }
  appModal.show()
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
    alert("保存失败")
  }
}

const deleteApp = async (item) => {
  if (confirm(`确定要删除应用 ${item.name} 吗？`)) {
    try {
      const res = await axios.delete(`/api/apps/${item.ID}`)
      if (res.data.code === 0) {
        loadApps()
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert("删除失败")
    }
  }
}

const resetAppKey = async (item) => {
  if (confirm(`确定要重置应用 ${item.name} 的密钥吗？旧密钥将立即失效！`)) {
    try {
      const res = await axios.post(`/api/apps/${item.ID}/reset-key`)
      if (res.data.code === 0) {
        alert(`重置成功！请保存新密钥，它不会再次显示：\n\n${res.data.data.AppKey}`)
      } else {
        alert(res.data.message)
      }
    } catch (error) {
      alert("重置密钥失败")
    }
  }
}
</script>
