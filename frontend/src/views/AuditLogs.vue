<template>
  <div class="audit-logs-page page-fixed-height">
    <div class="page-header">
      <div>
        <h1>{{ $t('audit_logs', '审计日志') }}</h1>
        <p class="page-subtitle">{{ $t('audit_logs_subtitle', '记录关键操作审计跟踪、安全访问与系统变更日志') }}</p>
      </div>
    </div>

    <!-- Toolbar -->
    <div class="page-toolbar mb-3">
      <form class="d-flex align-items-center flex-wrap gap-2" @submit.prevent="applyFilters">
        <input type="text" class="form-control form-control-sm" v-model="filters.username" :placeholder="$t('auth_username', '用户名')" style="width: 150px;">
        <input type="text" class="form-control form-control-sm" v-model="filters.module" :placeholder="$t('audit_module_placeholder', '模块(例如:/api/users)')" style="width: 200px;">
        <select class="form-select form-select-sm" v-model="filters.action" style="width: 150px;">
          <option value="">{{ $t('audit_all_actions', '全部操作') }}</option>
          <option value="CREATE">{{ $t('audit_action_create', '新增 (CREATE)') }}</option>
          <option value="UPDATE">{{ $t('audit_action_update', '更新 (UPDATE)') }}</option>
          <option value="DELETE">{{ $t('audit_action_delete', '删除 (DELETE)') }}</option>
        </select>
        <LiquidGlassButton type="submit" variant="primary" size="sm" class="ms-1">{{ $t('query', '查询') }}</LiquidGlassButton>
        <LiquidGlassButton type="button" variant="outline-secondary" size="sm" @click="resetFilters">{{ $t('common_reset', '重置') }}</LiquidGlassButton>
      </form>
    </div>

    <!-- Logs Table -->
    <div class="card border-0 shadow-sm table-glass-card">
      <div class="card-body p-0 d-flex flex-column h-100 overflow-hidden">
        <div class="table-responsive flex-grow-1">
          <table class="table table-hover align-middle mb-0 table-compact">
            <thead>
              <tr>
                <th class="ps-4">{{ $t('time', '时间') }}</th>
                <th>{{ $t('auth_username', '用户名') }}</th>
                <th>AppID</th>
                <th>{{ $t('module', '模块') }}</th>
                <th>{{ $t('action', '操作') }}</th>
                <th>{{ $t('resource', '资源') }}</th>
                <th>IP</th>
                <th class="pe-4">{{ $t('details', '详情') }}</th>
              </tr>
            </thead>
            <tbody>
              <!-- 骨架屏 -->
              <tr v-if="loading" v-for="n in 5" :key="'sk-' + n">
                <td class="ps-4"><div class="skeleton" style="height: 16px; width: 130px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 80px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 80px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 90px;"></div></td>
                <td><div class="skeleton" style="height: 20px; width: 70px; border-radius: 999px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td><div class="skeleton" style="height: 16px; width: 100px;"></div></td>
                <td class="pe-4"><div class="skeleton" style="height: 16px; width: 150px;"></div></td>
              </tr>
              <!-- 空状态 -->
              <tr v-else-if="logs.length === 0">
                <td colspan="8" class="p-0">
                  <div class="empty-state">
                    <i class="bi bi-clock-history"></i>
                    <p>{{ $t('audit_no_logs', '暂无审计日志记录') }}</p>
                  </div>
                </td>
              </tr>
              <tr v-for="log in logs" :key="log.id" v-else>
                <td class="ps-4 font-mono small text-secondary">{{ formatDateTime(log.created_at) }}</td>
                <td class="fw-semibold">{{ log.username || '-' }}</td>
                <td><code class="text-primary">{{ log.app_id || '-' }}</code></td>
                <td>{{ log.module }}</td>
                <td>
                  <span class="spec-badge" :class="getActionBadgeClass(log.action)">
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
        
        <ListPagination
          :page="page"
          :page-size="pageSize"
          :total="total"
          :disabled="loading"
          id-prefix="audit-logs"
          @update:page="changePage"
          @update:page-size="changePageSize"
        />

      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import axios from 'axios'
import ListPagination from '../components/ListPagination.vue'
import { formatDateTime } from '../utils/dateTime.js'

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
  if (newPage >= 1 && newPage <= Math.max(1, Math.ceil(total.value / pageSize.value))) {
    page.value = newPage
    loadLogs()
  }
}

const changePageSize = (size) => {
  pageSize.value = Number(size) || 20
  page.value = 1
  loadLogs()
}

const getActionBadgeClass = (action) => {
  switch (action) {
    case 'CREATE': return 'spec-badge--primary'
    case 'UPDATE': return 'spec-badge--info'
    case 'DELETE': return 'spec-badge--danger'
    default: return 'spec-badge--neutral'
  }
}
</script>
