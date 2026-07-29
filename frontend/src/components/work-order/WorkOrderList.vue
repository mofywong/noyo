<template>
  <div class="work-order-list card border-0 shadow-sm h-100">
    <div class="table-responsive">
      <table class="table table-hover align-middle mb-0">
        <thead class="table-light"><tr><th>工单</th><th>来源</th><th>优先级</th><th>状态</th><th>负责人</th><th>创建时间</th></tr></thead>
        <tbody>
          <tr v-if="loading && entries.length === 0"><td colspan="6" class="text-center py-5 text-muted">加载中…</td></tr>
          <tr v-else-if="entries.length === 0"><td colspan="6" class="text-center py-5 text-muted">暂无匹配工单</td></tr>
          <tr v-for="entry in entries" :key="orderId(entry.work_order)" role="button" :class="{ 'table-primary': selectedId === orderId(entry.work_order) }" @click="$emit('select', orderId(entry.work_order))">
            <td><div class="fw-semibold">{{ entry.work_order.title }}</div><div class="small text-muted">{{ entry.work_order.code }}</div></td>
            <td>{{ entry.work_order.source_type || '-' }}</td>
            <td>{{ entry.work_order.priority || '-' }}</td>
            <td>{{ statusLabel(entry.work_order.status) }}</td>
            <td>{{ entry.work_order.assignee_user_id || '未领取' }}</td>
            <td class="small text-muted">{{ formatTime(entry.work_order.CreatedAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-if="total > entries.length" class="card-footer bg-transparent text-end">
      <button class="btn btn-sm btn-outline-primary" :disabled="loading" @click="$emit('load-more')">加载更多（{{ entries.length }}/{{ total }}）</button>
    </div>
  </div>
</template>

<script setup>
import { formatDateTime } from '../../utils/dateTime.js'

defineProps({
  entries: { type: Array, default: () => [] },
  selectedId: { type: [Number, String], default: 0 },
  statuses: { type: Array, default: () => [] },
  total: { type: Number, default: 0 },
  loading: { type: Boolean, default: false }
})
defineEmits(['select', 'load-more'])
function orderId(order) { return order?.ID || order?.id || 0 }
function statusLabel(value) { return value || '-' }
function formatTime(value) { return formatDateTime(value) }
</script>
