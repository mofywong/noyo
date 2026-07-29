<template>
  <div class="work-order-form-panel">
    <WorkOrderFormFields :definition="definition" :model-value="modelValue" :id-prefix="idPrefix" @update:model-value="$emit('update:modelValue', $event)" />
  </div>
</template>

<script setup>
import WorkOrderFormFields from './WorkOrderFormFields.vue'
defineProps({
  definition: { type: Object, default: () => ({ fields: [] }) },
  modelValue: { type: Object, default: () => ({}) },
  idPrefix: { type: String, default: 'work-order-form' }
})
defineEmits(['update:modelValue'])
</script>
