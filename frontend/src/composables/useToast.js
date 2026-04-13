import { ref } from 'vue';

const toasts = ref([]);

export function useToast() {
  const showToast = (type, message) => {
    const id = Date.now();
    toasts.value.push({ id, type, message });
    
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
