<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-4">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('tenant_transfer_admin') }}</h2>
    </div>

    <div class="card shadow-sm">
      <div class="card-body">
        <div class="alert alert-warning d-flex align-items-center mb-4" role="alert">
          <i class="bi bi-exclamation-triangle-fill me-2"></i>
          <div>{{ $t('tenant_transfer_admin_confirm') }}</div>
        </div>

        <div class="mb-4">
          <label class="form-label fw-semibold">{{ $t('tenant_select_admin') }} <span class="text-danger">*</span></label>
          <select v-model="selectedUserId" class="form-select">
            <option value="" disabled>{{ $t('tenant_select_admin') }}</option>
            <option v-for="u in users" :key="u.ID" :value="u.ID">
              {{ u.display_name || u.username }} ({{ u.username }})
            </option>
          </select>
        </div>

        <button class="btn btn-danger" @click="transferAdmin" :disabled="!selectedUserId || transferring">
          <span v-if="transferring" class="spinner-border spinner-border-sm me-1"></span>
          <i v-else class="bi bi-arrow-left-right me-1"></i>
          {{ $t('tenant_transfer_admin') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth.js'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const users = ref([])
const selectedUserId = ref('')
const transferring = ref(false)

const loadUsers = async () => {
  try {
    const res = await axios.get('/api/users')
    if (res.data.code === 0) {
      // Exclude current user from the list
      users.value = (res.data.data || []).filter(u => u.ID !== authStore.user.user_id && u.status === 1)
    }
  } catch (error) {
    console.error("Failed to load users:", error)
  }
}

const transferAdmin = async () => {
  if (!selectedUserId.value) return

  if (!confirm(t('tenant_transfer_admin_confirm'))) return

  transferring.value = true
  try {
    const res = await axios.post('/api/tenant-transfer/admin', { target_user_id: selectedUserId.value })
    if (res.data.code === 0) {
      alert(t('tenant_transfer_admin_success'))
      authStore.logout()
      router.push('/login')
    } else {
      alert(res.data.message)
    }
  } catch (error) {
    alert(error.response?.data?.message || t('common_save_failed', '操作失败'))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  loadUsers()
})
</script>
