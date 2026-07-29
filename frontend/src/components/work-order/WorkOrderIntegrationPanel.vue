<template>
  <section class="work-order-integration-panel">
    <div v-if="feedback" class="alert" :class="`alert-${feedback.level}`" role="status">
      {{ feedback.message }}
      <button v-if="feedback.action === 'retry'" class="btn btn-sm btn-outline-danger ms-2" :disabled="disabled" @click="$emit('retry')">重试</button>
      <button v-if="feedback.action === 'refresh'" class="btn btn-sm btn-outline-warning ms-2" :disabled="disabled" @click="$emit('refresh')">刷新</button>
    </div>
    <button v-if="canIntegrate" class="btn btn-sm btn-outline-primary" :disabled="disabled" @click="$emit('integrate')">提交集成</button>
  </section>
</template>

<script setup>
defineProps({
  feedback: { type: Object, default: null },
  canIntegrate: { type: Boolean, default: false },
  disabled: { type: Boolean, default: false }
})
defineEmits(['retry', 'refresh', 'integrate'])
</script>
