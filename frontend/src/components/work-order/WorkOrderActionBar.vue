<template>
  <div class="work-order-action-bar d-flex flex-wrap gap-2">
    <button
      v-for="action in visibleActions"
      :key="action"
      type="button"
      class="btn btn-sm"
      :class="action === 'cancel' || action === 'reject' ? 'btn-outline-danger' : 'btn-outline-primary'"
      :disabled="disabled"
      @click="$emit('action', action)"
    >{{ labels[action] || action }}</button>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  actions: { type: Array, default: () => [] },
  pendingApproval: { type: Boolean, default: false },
  isApprovalCandidate: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false }
})
defineEmits(['action'])
const labels = { assign: '分派', reassign: '转派', start: '开始', pause: '暂停', resume: '恢复', resolve: '完成', accept: '验收', close: '关闭', reopen: '重开', cancel: '取消', withdraw: '撤回', approve: '批准', reject: '驳回' }
const visibleActions = computed(() => {
  if (props.pendingApproval && !props.isApprovalCandidate) return props.actions.filter(action => action === 'withdraw')
  return props.actions.filter(action => !['approve', 'reject'].includes(action) || props.isApprovalCandidate)
})
</script>
