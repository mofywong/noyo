import { computed, ref, watch } from 'vue'

// Selection is limited to the visible page; hidden rows are never deleted.
export function useListSelection(rows, context = []) {
  const selected = ref([])
  const codes = computed(() => rows.value.map(row => row.code))
  const allSelected = computed(() => codes.value.length > 0 && codes.value.every(code => selected.value.includes(code)))
  const someSelected = computed(() => selected.value.length > 0 && !allSelected.value)
  const clearSelection = () => { selected.value = [] }
  watch(codes, current => {
    selected.value = selected.value.filter(code => current.includes(code))
  }, { flush: 'sync' })
  if (context.length) watch(context, clearSelection, { flush: 'sync' })
  const toggleSelection = code => {
    if (!codes.value.includes(code)) return
    selected.value = selected.value.includes(code)
      ? selected.value.filter(item => item !== code)
      : [...selected.value, code]
  }
  const toggleAll = () => { selected.value = allSelected.value ? [] : [...codes.value] }
  return { selected, allSelected, someSelected, clearSelection, toggleSelection, toggleAll }
}
