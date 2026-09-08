<template>
  <div class="device-stat-strip compact-list-metrics" role="group" :aria-label="ariaLabel">
    <button
      v-for="metric in metrics"
      :key="metric.key"
      type="button"
      class="device-stat-item device-stat-item--interactive compact-list-metric"
      :class="[
        `compact-list-metric--${metric.tone || 'neutral'}`,
        { 'compact-list-metric--active': isSelected(metric.key) }
      ]"
      :aria-pressed="isSelected(metric.key)"
      @click="toggle(metric.key)"
    >
      <i v-if="metric.icon" class="compact-list-metric__icon bi" :class="metric.icon" aria-hidden="true"></i>
      <span class="compact-list-metric__content">
        <span class="compact-list-metric__label">{{ metric.label }}</span>
        <strong class="compact-list-metric__value">{{ metric.value }}</strong>
      </span>
    </button>
  </div>
</template>

<script setup>
const selected = defineModel({ default: () => [] })
const emit = defineEmits(['toggle'])

defineProps({
  metrics: { type: Array, required: true },
  ariaLabel: { type: String, default: 'List metrics' }
})

function isSelected(key) {
  return key === 'total'
    ? selected.value.length === 0
    : selected.value.includes(key)
}

function toggle(key) {
  if (key === 'total') {
    selected.value = []
    emit('toggle', key)
    return
  }

  const next = [...selected.value]
  const index = next.indexOf(key)
  if (index >= 0) {
    next.splice(index, 1)
  } else {
    next.push(key)
  }
  selected.value = next
  emit('toggle', key)
}
</script>
