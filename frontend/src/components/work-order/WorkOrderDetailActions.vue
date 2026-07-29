<template>
  <div class="work-order-detail-actions d-flex flex-wrap gap-2">
    <button v-if="availableActions.claim && can('process')" class="btn btn-sm btn-outline-primary" :disabled="disabled" @click="$emit('claim')">领取</button>
    <button v-for="transition in visibleTransitions" :key="transition.key" class="btn btn-sm btn-primary" :disabled="disabled" @click="$emit('transition', transition.key)">{{ transition.name || transition.key }}</button>
    <button v-if="can('approve')" class="btn btn-sm btn-success" :disabled="disabled" @click="$emit('approve')">通过审批</button>
    <button v-if="can('reject')" class="btn btn-sm btn-outline-danger" :disabled="disabled" @click="$emit('reject')">驳回审批</button>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { canShowWorkOrderAction } from '../../utils/workOrderUi.js'

const props = defineProps({
  transitions: { type: Array, default: () => [] },
  availableActions: { type: Object, default: () => ({ claim: false, transitions: [] }) },
  permissions: { type: [Object, Array], default: () => ({}) },
  approvalTask: { type: Object, default: () => ({}) },
  currentUser: { type: [String, Number], default: '' },
  disabled: { type: Boolean, default: false }
})
defineEmits(['claim', 'transition', 'approve', 'reject'])
const visibleTransitions = computed(() => (props.availableActions.transitions?.length ? props.availableActions.transitions : props.transitions)
  .filter(item => canShowWorkOrderAction('process', props.permissions, item)))
function can(action) { return canShowWorkOrderAction(action, props.permissions, props.approvalTask, props.currentUser) }
</script>
