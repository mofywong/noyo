<template>
  <div class="work-order-detail">
    <div v-if="order" class="card-body">
      <div class="text-muted small">{{ order.work_order?.code }}</div>
      <h5 class="mb-1">{{ order.work_order?.title }}</h5>
      <p v-if="order.work_order?.summary" class="text-body-secondary">{{ order.work_order.summary }}</p>
      <dl class="row small mb-3">
        <template v-for="field in order.form_definition?.fields || []" :key="field.key">
          <dt class="col-sm-4 text-muted">{{ field.label }}</dt><dd class="col-sm-8 text-break">{{ displayValue(order.form_data?.[field.key]) }}</dd>
        </template>
      </dl>
      <WorkOrderDetailActions v-bind="actions" @claim="$emit('claim')" @transition="$emit('transition', $event)" @approve="$emit('approve')" @reject="$emit('reject')" />
    </div>
    <div v-else class="card-body text-center text-muted py-5">选择一张工单查看详情</div>
  </div>
</template>

<script setup>
import WorkOrderDetailActions from './WorkOrderDetailActions.vue'
defineProps({
  order: { type: Object, default: null },
  actions: { type: Object, default: () => ({}) }
})
defineEmits(['claim', 'transition', 'approve', 'reject'])
function displayValue(value) { return Array.isArray(value) ? value.join(', ') : value === true ? '是' : value === false ? '否' : value ?? '-' }
</script>
