<template>
  <div class="work-order-filters row g-2">
    <div class="col-md-4">
      <input v-model.trim="draft.keyword" class="form-control" placeholder="搜索工单编号、标题或摘要" @keyup.enter="submit">
    </div>
    <div class="col-md-3">
      <select v-model="draft.status" class="form-select" @change="submit">
        <option value="">全部状态</option>
        <option v-for="status in statuses" :key="status.key" :value="status.key">{{ status.name || status.key }}</option>
      </select>
    </div>
    <div class="col-md-3">
      <select v-model="draft.sourceType" class="form-select" @change="submit">
        <option value="">全部来源</option>
        <option value="manual">人工创建</option>
        <option value="rule">规则引擎</option>
        <option value="alarm">告警</option>
        <option value="ai">AI</option>
        <option value="external">外部业务</option>
      </select>
    </div>
    <div class="col-md-2 d-grid">
      <button class="btn btn-outline-primary" :disabled="loading" @click="submit">查询</button>
    </div>
  </div>
</template>

<script setup>
import { reactive, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({ keyword: '', status: '', sourceType: '' }) },
  statuses: { type: Array, default: () => [] },
  loading: { type: Boolean, default: false }
})
const emit = defineEmits(['update:modelValue', 'submit'])
const draft = reactive({ keyword: '', status: '', sourceType: '' })

watch(() => props.modelValue, value => Object.assign(draft, value || {}), { immediate: true, deep: true })
function submit() {
  const value = { ...draft }
  emit('update:modelValue', value)
  emit('submit', value)
}
</script>
