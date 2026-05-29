<template>
  <div class="login-container d-flex align-items-center justify-content-center vh-100 bg-light">
    <div class="card shadow-sm" style="width: 100%; max-width: 400px;">
      <div class="card-body p-4">
        <div class="text-center mb-4">
          <div v-if="tenantLogo" class="mb-3 mx-auto" style="height: 80px; width: 100%; display: flex; justify-content: center; align-items: center; overflow: hidden;">
            <div v-if="tenantLogo.trim().startsWith('<svg') || tenantLogo.trim().startsWith('<?xml')" v-html="DOMPurify.sanitize(tenantLogo, { USE_PROFILES: { svg: true } })" class="svg-container" style="height: 100%; display: flex; align-items: center; justify-content: center;"></div>
            <img v-else :src="tenantLogo" style="max-height: 100%; max-width: 100%; object-fit: contain;">
          </div>
          <h2 class="fw-bold text-primary">{{ tenantName || $t('auth_login_title') }}</h2>
          <p class="text-muted">{{ $t('auth_login_subtitle') }}</p>
        </div>

        <div v-if="errorMsg" class="alert alert-danger" role="alert">
          {{ errorMsg }}
        </div>

        <form @submit.prevent="handleLogin">
          <div class="mb-3">
            <label class="form-label">{{ $t('auth_username') }}</label>
            <input 
              v-model="username" 
              type="text" 
              class="form-control" 
              :placeholder="$t('auth_username_placeholder')"
              required 
              autofocus
            />
          </div>
          <div class="mb-4">
            <label class="form-label">{{ $t('auth_password') }}</label>
            <div class="position-relative">
              <input
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                class="form-control pe-5"
                :placeholder="$t('auth_password_placeholder')"
                required
              />
              <button
                type="button"
                class="btn btn-link position-absolute end-0 top-50 translate-middle-y text-muted p-0 me-2"
                style="z-index: 5;"
                @click="showPassword = !showPassword"
                tabindex="-1"
              >
                <i :class="showPassword ? 'bi bi-eye-slash' : 'bi bi-eye'"></i>
              </button>
            </div>
          </div>
          <button 
            type="submit" 
            class="btn btn-primary w-100" 
            :disabled="loading"
          >
            <span v-if="loading" class="spinner-border spinner-border-sm me-2" role="status" aria-hidden="true"></span>
            {{ $t('auth_sign_in') }}
          </button>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import DOMPurify from 'dompurify'
import { useAuthStore } from '../stores/auth'
import { useI18n } from 'vue-i18n'
import axios from 'axios'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t } = useI18n()

const username = ref('')
const password = ref('')
const showPassword = ref(false)
const errorMsg = ref('')
const loading = ref(false)

const tenantName = ref('')
const tenantLogo = ref('')

onMounted(async () => {
  if (route.params.suffix) {
    try {
      const res = await axios.get('/api/auth/tenant-info', { params: { suffix: route.params.suffix } })
      if (res.data.code === 0 && res.data.data) {
        tenantName.value = res.data.data.name
        tenantLogo.value = res.data.data.logo
      }
    } catch (e) {
      console.warn('Failed to fetch tenant info for suffix:', route.params.suffix)
    }
  }
})

const handleLogin = async () => {
  if (!username.value || !password.value) return
  
  errorMsg.value = ''
  loading.value = true
  
  try {
    const res = await authStore.login(username.value, password.value, route.params.suffix || '')
    if (res.code === 0) {
      router.push('/')
    } else {
      errorMsg.value = res.message || t('auth_login_failed')
    }
  } catch (err) {
    if (err.response && err.response.data && err.response.data.message) {
      errorMsg.value = err.response.data.message
    } else {
      errorMsg.value = t('auth_network_error')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  background-color: #f8f9fa;
}
:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
}
</style>
