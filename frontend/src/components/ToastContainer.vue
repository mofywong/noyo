<template>
  <div class="toast-container position-fixed bottom-0 end-0 p-3">
    <div 
      v-for="toast in toasts" 
      :key="toast.id" 
      class="toast show align-items-center border-0" 
      :class="`text-bg-${toast.type}`"
      role="alert" 
      aria-live="assertive" 
      aria-atomic="true"
    >
      <div class="d-flex">
        <div class="toast-body d-flex align-items-center">
          <i class="bi me-2 fs-5" :class="toast.type === 'success' ? 'bi-check-circle-fill' : 'bi-exclamation-octagon-fill'"></i>
          {{ toast.message }}
        </div>
        <button type="button" class="btn-close btn-close-white me-2 m-auto" @click="removeToast(toast.id)" aria-label="Close"></button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useToast } from '../composables/useToast';

const { toasts, removeToast } = useToast();
</script>

<style scoped>
.toast {
  transition: all 0.3s ease;
}
</style>
