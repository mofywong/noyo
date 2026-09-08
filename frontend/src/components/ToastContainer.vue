<template>
  <div class="toast-container position-fixed bottom-0 end-0 p-3 noyo-toast-stack">
    <div
      v-for="toast in toasts"
      :key="toast.id"
      class="toast show align-items-stretch border-0 noyo-toast-item"
      :class="toastVariant(toast.type)"
      role="status"
      aria-live="polite"
      aria-atomic="true"
      @mouseenter="pauseToast(toast.id)"
      @mouseleave="resumeToast(toast.id)"
    >
      <div class="d-flex w-100">
        <div class="noyo-toast-accent"></div>
        <div class="toast-body flex-grow-1">
          <div class="noyo-toast-title">
            <i class="bi me-2 fs-5" :class="toastIcon(toast.type)"></i>
            <span>{{ toastTitle(toast.type) }}</span>
          </div>
          <div class="noyo-toast-message">{{ toast.message }}</div>
          <button
            v-if="toast.actionLabel"
            type="button"
            class="btn btn-sm noyo-toast-action"
            :class="toastActionClass(toast.type)"
            @click="triggerAction(toast.id)"
          >
            {{ toast.actionLabel }}
          </button>
        </div>
        <button type="button" class="btn-close noyo-toast-close me-2 mt-3" @click="removeToast(toast.id)" :aria-label="$t('common_close', '关闭 Close')"></button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { useToast } from '../composables/useToast';

const { toasts, removeToast, pauseToast, resumeToast, triggerAction } = useToast();

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

const toastActionClass = (type) => {
  if (type === 'success') return 'noyo-toast-action--success';
  if (type === 'warning') return 'noyo-toast-action--warning';
  return 'noyo-toast-action--danger';
};
</script>

<style scoped>
/* Noyo UX Guidelines §8.9：右下角堆叠、宽 360px、语义色左条 + 10-15% 底（双主题） */
.noyo-toast-stack {
  z-index: var(--lg-layer-toast);
}

.noyo-toast-item {
  border-radius: 8px;
  color: var(--text-primary);
  min-width: min(360px, calc(100vw - 2rem));
  max-width: min(420px, calc(100vw - 2rem));
  overflow: hidden;
  transition: opacity 0.2s ease;
  box-shadow: var(--shadow-floating) !important;
}

[data-bs-theme='dark'] .noyo-toast-item {
  box-shadow: var(--shadow-floating), var(--surface-highlight) !important;
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
  background: color-mix(in srgb, var(--color-success) 10%, var(--bg-elevated));
  border: 1px solid color-mix(in srgb, var(--color-success) 35%, transparent) !important;
  color: var(--text-primary);
}

.toast-item--success .noyo-toast-accent {
  background: var(--color-success);
}

.toast-item--success .noyo-toast-title {
  color: var(--color-success);
}

.toast-item--warning {
  background: color-mix(in srgb, var(--color-warning) 10%, var(--bg-elevated));
  border: 1px solid color-mix(in srgb, var(--color-warning) 35%, transparent) !important;
  color: var(--text-primary);
}

.toast-item--warning .noyo-toast-accent {
  background: var(--color-warning);
}

.toast-item--warning .noyo-toast-title {
  color: var(--color-warning);
}

.toast-item--danger {
  background: color-mix(in srgb, var(--color-danger) 10%, var(--bg-elevated));
  border: 1px solid color-mix(in srgb, var(--color-danger) 35%, transparent) !important;
  color: var(--text-primary);
}

.toast-item--danger .noyo-toast-accent {
  background: var(--color-danger);
}

.toast-item--danger .noyo-toast-title {
  color: var(--color-danger);
}

.noyo-toast-action {
  margin-top: 8px;
  padding: 2px 12px;
  font-weight: 500;
}

.noyo-toast-action--success {
  color: var(--color-success);
  border: 1px solid color-mix(in srgb, var(--color-success) 40%, transparent);
}

.noyo-toast-action--warning {
  color: var(--color-warning);
  border: 1px solid color-mix(in srgb, var(--color-warning) 40%, transparent);
}

.noyo-toast-action--danger {
  color: var(--color-danger);
  border: 1px solid color-mix(in srgb, var(--color-danger) 40%, transparent);
}
</style>
