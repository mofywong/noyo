import { ref } from 'vue';

/**
 * 全局确认对话框状态（Noyo UX Guidelines §8.4 / §10.3）。
 * 由 components/ConfirmDialog.vue 消费渲染；业务方调用 confirmDialog(options)
 * 获得 Promise<boolean>，替代原生 confirm()。
 *
 * options: {
 *   title: string,        // 动作名，如「删除设备 Delete Device」
 *   message: string,      // 后果说明（12-14px 次级色）
 *   variant: 'danger' | 'neutral',  // 默认 'danger'
 *   confirmText?: string, // 默认「确认 Confirm」
 *   cancelText?: string,  // 默认「取消 Cancel」
 *   icon?: string         // bootstrap-icons 类名，缺省按 variant 选择
 * }
 */
const state = ref({
  visible: false,
  title: '',
  message: '',
  variant: 'danger',
  confirmText: '',
  cancelText: '',
  icon: '',
});

let resolver = null;

export function useConfirm() {
  const confirmDialog = (options = {}) => {
    // 并发守卫：已有确认框打开时忽略新请求（§8.4，避免 Promise 永久挂起）
    if (resolver) {
      return Promise.resolve(false);
    }
    return new Promise((resolve) => {
      resolver = resolve;
      state.value = {
        visible: true,
        title: options.title || '',
        message: options.message || '',
        variant: options.variant === 'neutral' ? 'neutral' : 'danger',
        confirmText: options.confirmText || '',
        cancelText: options.cancelText || '',
        icon: options.icon || '',
      };
    });
  };

  const resolveConfirm = (result) => {
    state.value.visible = false;
    if (resolver) {
      resolver(result);
      resolver = null;
    }
  };

  return {
    confirmDialog,
    confirmState: state,
    resolveConfirm,
  };
}
