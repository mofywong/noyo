<template>
  <div v-if="count" class="d-flex align-items-center gap-2 flex-wrap" role="group" :aria-label="$t('batch_delete')">
    <span class="fw-bold small">{{ $t('batch_selected_count', { count }) }}</span>
    <slot />
    <button type="button" class="btn btn-outline-danger btn-sm" :disabled="busy" @click="$emit('delete')" v-permission="permission">
      <i class="bi bi-trash me-1" aria-hidden="true"></i>{{ $t('batch_delete') }}
    </button>
    <button type="button" class="btn btn-link btn-sm text-muted" :disabled="busy" @click="$emit('clear')">
      {{ $t('deselect_all') }}
    </button>
  </div>
</template>

<script setup>
defineProps({ count: Number, busy: Boolean, permission: String })
defineEmits(['delete', 'clear'])
</script>
