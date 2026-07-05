<template>
  <div class="toast-container position-fixed bottom-0 end-0 p-3">
    <div 
      v-for="toast in toasts" 
      :key="toast.id" 
      class="toast show align-items-stretch border-0 shadow-lg noyo-toast-item"
      :class="toastVariant(toast.type)"
      role="alert" 
      aria-live="assertive" 
      aria-atomic="true"
    >
      <div class="d-flex w-100">
        <div class="noyo-toast-accent"></div>
        <div class="toast-body flex-grow-1">
          <div class="noyo-toast-title">
            <i class="bi me-2 fs-5" :class="toastIcon(toast.type)"></i>
            <span>{{ toastTitle(toast.type) }}</span>
          </div>
          <div class="noyo-toast-message">{{ toast.message }}</div>
        </div>
        <button type="button" class="btn-close noyo-toast-close me-2 mt-3" @click="removeToast(toast.id)" aria-label="Close"></button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useToast } from '../composables/useToast';

const { toasts, removeToast } = useToast();

const toastVariant = (type) => {
  if (type === 'success') return 'toast-item--success';
  if (type === 'warning') return 'toast-item--warning';
  return 'toast-item--danger';
};

const toastIcon = (type) => {
  if (type === 'success') return 'bi-check-circle-fill';
  if (type === 'warning') return 'bi-exclamation-triangle-fill';
  return 'bi-exclamation-octagon-fill';
};

const toastTitle = (type) => {
  if (type === 'success') return '操作成功 / Success';
  if (type === 'warning') return '操作提醒 / Notice';
  return '操作失败 / Error';
};
</script>

<style scoped>
.noyo-toast-item {
  border-radius: 8px;
  color: #111827;
  min-width: min(360px, calc(100vw - 2rem));
  max-width: min(420px, calc(100vw - 2rem));
  overflow: hidden;
  transition: all 0.3s ease;
}

.noyo-toast-accent {
  flex: 0 0 6px;
}

.noyo-toast-title {
  align-items: center;
  display: flex;
  font-weight: 700;
  line-height: 1.2;
  margin-bottom: 0.25rem;
}

.noyo-toast-message {
  color: inherit;
  font-size: 0.9rem;
  line-height: 1.45;
  word-break: break-word;
}

.noyo-toast-close {
  opacity: 0.75;
}

.toast-item--success {
  background: #ecfdf5;
  border: 1px solid #10b981 !important;
  box-shadow: 0 12px 30px rgba(6, 95, 70, 0.22) !important;
  color: #064e3b;
}

.toast-item--success .noyo-toast-accent {
  background: #059669;
}

.toast-item--warning {
  background: #fffbeb;
  border: 1px solid #f59e0b !important;
  box-shadow: 0 12px 30px rgba(146, 64, 14, 0.22) !important;
  color: #78350f;
}

.toast-item--warning .noyo-toast-accent {
  background: #d97706;
}

.toast-item--danger {
  background: #fef2f2;
  border: 1px solid #ef4444 !important;
  box-shadow: 0 12px 30px rgba(127, 29, 29, 0.24) !important;
  color: #7f1d1d;
}

.toast-item--danger .noyo-toast-accent {
  background: #dc2626;
}
</style>
