import { ref } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useConfirm } from './useConfirm.js'
import { useToast } from './useToast.js'
import { resolveApiErrorMessage } from '../utils/apiMessages.js'

export async function deleteItems(codes, remove, onError = () => {}) {
  const succeeded = []
  const failed = []
  for (const code of codes) {
    try {
      const response = await remove(code)
      if (response?.data?.code != null && response.data.code !== 0) {
        throw Object.assign(new Error(response.data.message), { response })
      }
      succeeded.push(code)
    } catch (error) {
      failed.push(code)
      onError(error, code)
    }
  }
  return { succeeded, failed }
}

export function useBatchDelete({ selected, endpoint, refresh, onDeleted = () => {} }) {
  const deleting = ref(false)
  const { t, locale } = useI18n()
  const { confirmDialog } = useConfirm()
  const { showToast } = useToast()
  const batchDelete = async () => {
    if (deleting.value || !selected.value.length) return
    const codes = [...selected.value]
    deleting.value = true
    try {
      const confirmed = await confirmDialog({
        title: t('batch_delete_title', { count: codes.length }),
        message: t('batch_delete_message', { count: codes.length }),
        variant: 'danger', confirmText: t('common_delete'), cancelText: t('common_cancel'),
      })
      if (!confirmed) return
      // A page/filter change while the confirmation was open invalidates it.
      if (codes.some(code => !selected.value.includes(code))) return
      const reasons = new Set()
      const { succeeded, failed } = await deleteItems(codes, code => axios.delete(`${endpoint}/${encodeURIComponent(code)}`), error => {
        const reason = resolveApiErrorMessage(error, locale.value)
        if (reason) reasons.add(reason)
      })
      selected.value = selected.value.filter(code => !succeeded.includes(code))
      onDeleted(succeeded)
      const summary = t(failed.length ? 'batch_delete_partial' : 'batch_delete_success', {
        count: succeeded.length, failed: failed.length,
      })
      showToast(failed.length ? 'danger' : 'success', [summary, ...reasons].join(' · '))
      await refresh()
    } finally {
      deleting.value = false
    }
  }
  return { deleting, batchDelete }
}
