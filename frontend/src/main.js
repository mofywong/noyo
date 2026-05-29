import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'bootstrap/dist/css/bootstrap.min.css'
import 'bootstrap/dist/js/bootstrap.bundle.min.js'
import 'bootstrap-icons/font/bootstrap-icons.css'
import './style.css'
import App from './App.vue'
import i18n from './i18n'
import router from './router'
import { loader } from '@guolao/vue-monaco-editor'
import axios from 'axios'
import { useAuthStore } from './stores/auth'

import * as monaco from 'monaco-editor'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'
import cssWorker from 'monaco-editor/esm/vs/language/css/css.worker?worker'
import htmlWorker from 'monaco-editor/esm/vs/language/html/html.worker?worker'
import tsWorker from 'monaco-editor/esm/vs/language/typescript/ts.worker?worker'

self.MonacoEnvironment = {
  getWorker(_, label) {
    if (label === 'json') {
      return new jsonWorker()
    }
    if (label === 'css' || label === 'scss' || label === 'less') {
      return new cssWorker()
    }
    if (label === 'html' || label === 'handlebars' || label === 'razor') {
      return new htmlWorker()
    }
    if (label === 'typescript' || label === 'javascript') {
      return new tsWorker()
    }
    return new editorWorker()
  }
}

loader.config({ monaco })

const app = createApp(App)
const pinia = createPinia()
app.use(pinia)
app.use(i18n)
app.use(router)

app.directive('permission', {
  mounted(el, binding) {
    const authStore = useAuthStore()
    if (!authStore.hasPermission(binding.value)) {
      el.parentNode && el.parentNode.removeChild(el)
    }
  }
})

// Configure global Axios interceptors
axios.interceptors.request.use((config) => {
  const authStore = useAuthStore()
  if (authStore.token) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }
  
  const projectId = localStorage.getItem('current_project_id')
  if (projectId) {
    config.headers['X-Current-Project-ID'] = projectId
  }

  if (authStore.user && authStore.user.tenant_id > 0) {
    config.headers['X-Current-Tenant-ID'] = authStore.user.tenant_id
  } else {
    // Fallback for system admin managing a specific tenant
    const tenantId = localStorage.getItem('current_tenant_id')
    if (tenantId) {
      config.headers['X-Current-Tenant-ID'] = tenantId
    }
  }
  
  return config
})

let isRefreshing = false
let failedQueue = []

const processQueue = (error, token = null) => {
  failedQueue.forEach(prom => {
    if (error) {
      prom.reject(error);
    } else {
      prom.resolve(token);
    }
  })
  failedQueue = [];
}

const handle401Error = (originalRequest) => {
  const authStore = useAuthStore()
  if (originalRequest.url === '/api/auth/login' || originalRequest.url === '/api/auth/refresh' || originalRequest._retry) {
    authStore.logout()
    router.push('/login')
    return Promise.reject('Unauthorized')
  }

  if (isRefreshing) {
    return new Promise(function(resolve, reject) {
      failedQueue.push({resolve, reject})
    }).then(token => {
      originalRequest.headers.Authorization = 'Bearer ' + token;
      return axios(originalRequest);
    }).catch(err => Promise.reject(err))
  }

  originalRequest._retry = true;
  isRefreshing = true;

  return new Promise(function (resolve, reject) {
    axios.post('/api/auth/refresh', { refresh_token: authStore.refreshToken })
      .then(({data}) => {
        if (data.code === 0) {
          authStore.token = data.data.access_token;
          authStore.refreshToken = data.data.refresh_token;
          localStorage.setItem('access_token', data.data.access_token);
          localStorage.setItem('refresh_token', data.data.refresh_token);
          
          processQueue(null, data.data.access_token);
          originalRequest.headers.Authorization = 'Bearer ' + data.data.access_token;
          resolve(axios(originalRequest));
        } else {
          processQueue(new Error('Refresh failed'), null);
          authStore.logout()
          router.push('/login')
          reject('Refresh failed')
        }
      })
      .catch((err) => {
        processQueue(err, null);
        authStore.logout()
        router.push('/login')
        reject(err)
      })
      .finally(() => { isRefreshing = false })
  })
}

axios.interceptors.response.use(
  (response) => {
    if (response.data && response.data.code === 401) {
      const url = response.config.url || ''
      if (url === '/api/auth/login' || url === '/api/auth/refresh') {
        return response
      }
      return handle401Error(response.config)
    }
    return response
  },
  (error) => {
    if (error.response && error.response.status === 401) {
      const url = error.config?.url || ''
      if (url === '/api/auth/login' || url === '/api/auth/refresh') {
        return Promise.reject(error)
      }
      return handle401Error(error.config)
    }
    return Promise.reject(error)
  }
)

app.mount('#app')
