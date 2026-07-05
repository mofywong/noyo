import { ref } from 'vue';

const toasts = ref([]);
const toastTypes = new Set(['success', 'warning', 'danger', 'error']);

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

export function useToast() {
  const showToast = (typeOrMessage, messageOrType) => {
    const id = Date.now();
    const toast = normalizeToastArgs(typeOrMessage, messageOrType);
    toasts.value.push({ id, ...toast });
    
    // Auto remove after 3 seconds
    setTimeout(() => {
      removeToast(id);
    }, 3000);
  };

  const removeToast = (id) => {
    const index = toasts.value.findIndex(t => t.id === id);
    if (index !== -1) {
      toasts.value.splice(index, 1);
    }
  };

  return {
    toasts,
    showToast,
    removeToast
  };
}
