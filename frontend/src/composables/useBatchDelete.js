import { ref } from 'vue'
import axios from 'axios'
import { useI18n } from 'vue-i18n'
import { useConfirm } from './useConfirm.js'
import { useToast } from './useToast.js'

export async function deleteItems(codes, remove) {
  const succeeded = []
  const failed = []
  for (const code of codes) {
    try {
      const response = await remove(code)
      if (response?.data?.code != null && response.data.code !== 0) throw new Error(response.data.message)
      succeeded.push(code)
    } catch {
      failed.push(code)
    }
  }
  return { succeeded, failed }
}

export function useBatchDelete({ selected, endpoint, refresh, onDeleted = () => {} }) {
  const deleting = ref(false)
  const { t } = useI18n()
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
      const { succeeded, failed } = await deleteItems(codes, code => axios.delete(`${endpoint}/${encodeURIComponent(code)}`))
      selected.value = selected.value.filter(code => !succeeded.includes(code))
      onDeleted(succeeded)
      showToast(failed.length ? 'danger' : 'success', t(failed.length ? 'batch_delete_partial' : 'batch_delete_success', {
        count: succeeded.length, failed: failed.length,
      }))
      await refresh()
    } finally {
      deleting.value = false
    }
  }
  return { deleting, batchDelete }
}
