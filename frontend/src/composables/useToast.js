import { ref } from 'vue';

/**
 * 全局 Toast（Noyo UX Guidelines §8.9 / §10.1 / §10.3）。
 * showToast(typeOrMessage, messageOrType, options?)
 *   options: { duration?: number (默认 4000ms), actionLabel?: string, onAction?: fn }
 * 可逆操作可传入 actionLabel/onAction 实现「即时执行 + Toast 撤销」。
 */
const toasts = ref([]);
const toastTypes = new Set(['success', 'warning', 'danger', 'error']);

// id -> { timeout, remaining, startedAt }
const timers = new Map();

export const normalizeToastArgs = (first, second) => {
  const firstValue = first == null ? '' : String(first);
  const secondValue = second == null ? '' : String(second);
  const firstType = firstValue.toLowerCase();
  const secondType = secondValue.toLowerCase();

  if (toastTypes.has(firstType)) {
    return {
      type: firstType === 'error' ? 'danger' : firstType,
      message: secondValue,
    };
  }
  if (toastTypes.has(secondType)) {
    return {
      type: secondType === 'error' ? 'danger' : secondType,
      message: firstValue,
    };
  }
  return {
    type: 'danger',
    message: firstValue || secondValue,
  };
};

const removeToast = (id) => {
  const entry = timers.get(id);
  if (entry) {
    clearTimeout(entry.timeout);
    timers.delete(id);
  }
  const index = toasts.value.findIndex((t) => t.id === id);
  if (index !== -1) {
    toasts.value.splice(index, 1);
  }
};

const scheduleRemove = (id, duration) => {
  timers.set(id, { timeout: null, remaining: duration, startedAt: Date.now() });
  const entry = timers.get(id);
  entry.timeout = setTimeout(() => {
    timers.delete(id);
    removeToast(id);
  }, duration);
};

export function useToast() {
  const showToast = (typeOrMessage, messageOrType, options = {}) => {
    const id = Date.now() + Math.random().toString(36).slice(2, 6);
    const toast = normalizeToastArgs(typeOrMessage, messageOrType);
    const item = {
      id,
      ...toast,
      duration: options.duration || 4000,
      actionLabel: options.actionLabel || '',
      onAction: options.onAction || null,
    };
    toasts.value.push(item);
    scheduleRemove(id, item.duration);
  };

  const pauseToast = (id) => {
    const entry = timers.get(id);
    if (!entry) return;
    clearTimeout(entry.timeout);
    entry.remaining -= Date.now() - entry.startedAt;
  };

  const resumeToast = (id) => {
    const toast = toasts.value.find((t) => t.id === id);
    if (!toast || !timers.has(id)) return;
    const entry = timers.get(id);
    entry.startedAt = Date.now();
    entry.timeout = setTimeout(() => {
      timers.delete(id);
      removeToast(id);
    }, Math.max(entry.remaining, 300));
  };

  const triggerAction = (id) => {
    const toast = toasts.value.find((t) => t.id === id);
    if (toast && typeof toast.onAction === 'function') {
      try {
        toast.onAction();
      } catch (e) {
        console.error('toast action failed', e);
      }
    }
    removeToast(id);
  };

  return {
    toasts,
    showToast,
    removeToast,
    pauseToast,
    resumeToast,
    triggerAction,
  };
}
