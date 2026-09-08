<template>
  <Teleport to="body">
    <Transition name="noyo-confirm-fade">
      <div
        v-if="confirmState.visible"
        class="noyo-confirm-overlay"
        @click.self="handleCancel"
        @keydown.esc="handleCancel"
      >
        <div
          class="noyo-confirm"
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="noyo-confirm-title"
          tabindex="-1"
        >
          <div class="noyo-confirm-icon" :class="iconClass">
            <i class="bi" :class="iconName"></i>
          </div>
          <h5 id="noyo-confirm-title" class="noyo-confirm-title">{{ confirmState.title }}</h5>
          <p class="noyo-confirm-message">{{ confirmState.message }}</p>
          <div class="noyo-confirm-actions">
            <LiquidGlassButton variant="outline-secondary" @click="handleCancel">
              {{ confirmState.cancelText || $t('common_cancel', '取消 Cancel') }}
            </LiquidGlassButton>
            <LiquidGlassButton :variant="confirmState.variant === 'neutral' ? 'primary' : 'danger'" @click="handleConfirm">
              {{ confirmState.confirmText || $t('common_confirm', '确认 Confirm') }}
            </LiquidGlassButton>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { computed, watch, nextTick, ref } from 'vue';
import { useConfirm } from '../composables/useConfirm';

const { confirmState, resolveConfirm } = useConfirm();
const cancelRef = ref(null);

const iconName = computed(() => {
  if (confirmState.value.icon) return confirmState.value.icon;
  return confirmState.value.variant === 'neutral'
    ? 'bi-question-circle'
    : 'bi-exclamation-triangle-fill';
});

const iconClass = computed(() => ({
  'noyo-confirm-icon--danger': confirmState.value.variant === 'danger',
  'noyo-confirm-icon--neutral': confirmState.value.variant === 'neutral',
}));

const confirmButtonClass = computed(() =>
  confirmState.value.variant === 'neutral'
    ? 'btn-primary'
    : 'btn-danger'
);

const handleConfirm = () => resolveConfirm(true);
const handleCancel = () => resolveConfirm(false);

// 打开时聚焦对话框（键盘可达性，§12）；关闭时释放焦点
watch(
  () => confirmState.value.visible,
  async (visible) => {
    if (visible) {
      await nextTick();
      document.activeElement?.blur?.();
      cancelRef.value?.focus?.();
    }
  }
);
</script>

<style scoped>
.noyo-confirm-overlay {
  position: fixed;
  inset: 0;
  z-index: var(--lg-layer-tooltip);
  background: rgba(15, 23, 42, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

[data-bs-theme='dark'] .noyo-confirm-overlay {
  background: rgba(0, 0, 0, 0.6);
}

.noyo-confirm {
  width: min(480px, 100%);
  background: var(--bg-elevated);
  border-radius: var(--radius-modal);
  box-shadow: var(--shadow-floating);
  padding: 24px;
  text-align: center;
  outline: none;
}

[data-bs-theme='dark'] .noyo-confirm {
  box-shadow: var(--shadow-floating), var(--surface-highlight);
}

.noyo-confirm-icon {
  width: 48px;
  height: 48px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.5rem;
  margin: 0 auto 16px;
}

.noyo-confirm-icon--danger {
  background: color-mix(in srgb, var(--color-danger) 12%, transparent);
  color: var(--color-danger);
}

.noyo-confirm-icon--neutral {
  background: color-mix(in srgb, var(--color-info) 12%, transparent);
  color: var(--color-info);
}

.noyo-confirm-title {
  font-size: 1rem;
  font-weight: 600;
  margin-bottom: 8px;
  color: var(--text-primary);
}

.noyo-confirm-message {
  font-size: 0.875rem;
  line-height: 1.6;
  color: var(--text-secondary);
  margin-bottom: 24px;
  word-break: break-word;
}

.noyo-confirm-actions {
  display: flex;
  justify-content: center;
  gap: 12px;
}

.noyo-confirm-actions .btn {
  min-width: 96px;
}

/* 动效（§10.2）：200ms fade + 轻微上移 8px */
.noyo-confirm-fade-enter-active,
.noyo-confirm-fade-leave-active {
  transition: opacity 0.2s cubic-bezier(0.2, 0, 0, 1);
}

.noyo-confirm-fade-enter-active .noyo-confirm,
.noyo-confirm-fade-leave-active .noyo-confirm {
  transition: transform 0.2s cubic-bezier(0.2, 0, 0, 1), opacity 0.2s cubic-bezier(0.2, 0, 0, 1);
}

.noyo-confirm-fade-enter-from,
.noyo-confirm-fade-leave-to {
  opacity: 0;
}

.noyo-confirm-fade-enter-from .noyo-confirm {
  transform: translateY(8px);
}

.noyo-confirm-fade-leave-to .noyo-confirm {
  transform: translateY(-4px);
}
</style>
