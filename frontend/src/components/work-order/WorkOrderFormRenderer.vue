<template>
  <form class="work-order-form-renderer" @submit.prevent="submit">
    <div v-for="field in models" :key="field.key" class="mb-3">
      <label class="form-label" :for="fieldId(field)">
        {{ field.label }} <span v-if="field.required" class="text-danger">*</span>
      </label>
      <textarea v-if="field.type === 'textarea'" :id="fieldId(field)" v-model="values[field.key]" class="form-control" rows="3" />
      <select v-else-if="field.type === 'enum'" :id="fieldId(field)" v-model="values[field.key]" class="form-select">
        <option value="">请选择</option>
        <option v-for="option in field.options" :key="String(option)" :value="option">{{ option }}</option>
      </select>
      <input v-else :id="fieldId(field)" v-model="values[field.key]" class="form-control" :type="inputType(field)" />
      <div v-if="errors[field.key]" class="invalid-feedback d-block">{{ errors[field.key] }}</div>
    </div>
    <button v-if="showSubmit" type="submit" class="btn btn-primary">提交</button>
  </form>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { fieldModels, initialFormValues, validateFormValues } from '../../utils/workOrderForm'

const props = defineProps({
  schema: { type: Object, default: () => ({ type: 'object', properties: {} }) },
  uiSchema: { type: Object, default: () => ({}) },
  modelValue: { type: Object, default: () => ({}) },
  showSubmit: { type: Boolean, default: true },
  idPrefix: { type: String, default: 'work-order-form' }
})
const emit = defineEmits(['update:modelValue', 'submit', 'validation-error'])
const values = ref({})
const errors = ref({})
const models = computed(() => fieldModels(props.schema, props.uiSchema))

watch(() => [props.schema, props.uiSchema, props.modelValue], () => {
  values.value = initialFormValues(models.value, props.modelValue)
}, { immediate: true, deep: true })
watch(values, next => emit('update:modelValue', { ...next }), { deep: true })

function fieldId(field) { return `${props.idPrefix}-${field.key}` }
function inputType(field) {
  if (field.type === 'number' || field.type === 'integer') return 'number'
  if (field.type === 'date') return 'date'
  if (field.type === 'date-time' || field.type === 'datetime') return 'datetime-local'
  if (field.type === 'attachment') return 'file'
  return 'text'
}
function submit() {
  errors.value = validateFormValues(models.value, values.value)
  if (Object.keys(errors.value).length) { emit('validation-error', errors.value); return }
  emit('submit', { ...values.value })
}
</script>
