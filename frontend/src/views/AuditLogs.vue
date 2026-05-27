<template>
  <div class="container-fluid py-4">
    <div class="d-flex justify-content-between align-items-center mb-3">
      <h2 class="h4 mb-0 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('audit_logs', '审计日志') }}</h2>
      <form class="d-flex align-items-center gap-2" @submit.prevent="applyFilters">
        <input type="text" class="form-control form-control-sm" v-model="filters.username" placeholder="用户名" style="width: 150px;">
        <input type="text" class="form-control form-control-sm" v-model="filters.module" placeholder="模块(例如:/api/users)" style="width: 200px;">
        <select class="form-select form-select-sm" v-model="filters.action" style="width: 150px;">
          <option value="">全部操作</option>
          <option value="CREATE">新增 (CREATE)</option>
          <option value="UPDATE">更新 (UPDATE)</option>
          <option value="DELETE">删除 (DELETE)</option>
        </select>
        <button type="submit" class="btn btn-primary btn-sm ms-1">查询</button>
        <button type="button" class="btn btn-outline-secondary btn-sm" @click="resetFilters">重置</button>
      </form>
    </div>

    <!-- Logs Table -->
    <div class="card shadow-sm">
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>时间</th>
                <th>用户名</th>
                <th>AppID</th>
                <th>模块</th>
                <th>操作</th>
                <th>资源</th>
                <th>IP</th>
                <th>详情</th>
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
              <tr v-else-if="logs.length === 0">
                <td colspan="8" class="text-center py-4 text-muted">暂无日志记录</td>
              </tr>
              <tr v-for="log in logs" :key="log.id" v-else>
                <td>{{ new Date(log.created_at).toLocaleString() }}</td>
                <td>{{ log.username || '-' }}</td>
                <td>{{ log.app_id || '-' }}</td>
                <td>{{ log.module }}</td>
                <td>
                  <span class="badge" :class="getActionBadgeClass(log.action)">
                    {{ log.action }}
                  </span>
                </td>
                <td>{{ log.resource }}</td>
                <td>{{ log.ip }}</td>
                <td>{{ log.detail }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        
        <!-- Pagination -->
        <div class="d-flex justify-content-between align-items-center p-3 border-top">
          <div class="text-muted small">共 {{ total }} 条记录</div>
          <ul class="pagination pagination-sm mb-0">
            <li class="page-item" :class="{ disabled: page === 1 }">
              <button class="page-link" @click="changePage(page - 1)">上一页</button>
            </li>
            <li class="page-item active"><span class="page-link">{{ page }}</span></li>
            <li class="page-item" :class="{ disabled: page * pageSize >= total }">
              <button class="page-link" @click="changePage(page + 1)">下一页</button>
            </li>
          </ul>
        </div>

      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'

const logs = ref([])
const loading = ref(false)
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)

const filters = ref({
  username: '',
  module: '',
  action: ''
})

const loadLogs = async () => {
  loading.value = true
  try {
    const params = {
      page: page.value,
      page_size: pageSize.value,
      ...filters.value
    }
    const res = await axios.get('/api/audit-logs', { params })
    if (res.data.code === 0) {
      logs.value = res.data.data || []
      total.value = res.data.total || 0
    }
  } catch (error) {
    console.error("Failed to load audit logs:", error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadLogs()
})

const applyFilters = () => {
  page.value = 1
  loadLogs()
}

const resetFilters = () => {
  filters.value = { username: '', module: '', action: '' }
  applyFilters()
}

const changePage = (newPage) => {
  if (newPage >= 1) {
    page.value = newPage
    loadLogs()
  }
}

const getActionBadgeClass = (action) => {
  switch (action) {
    case 'CREATE': return 'bg-success'
    case 'UPDATE': return 'bg-warning text-dark'
    case 'DELETE': return 'bg-danger'
    default: return 'bg-secondary'
  }
}
</script>
