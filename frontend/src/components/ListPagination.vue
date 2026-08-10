<template>
  <footer v-if="total > 0" class="list-pagination">
    <div class="list-pagination__summary">
      <span class="text-body-secondary small">{{ copy.total.replace('{total}', String(total)) }}</span>
      <label v-if="showPageSize" class="list-pagination__page-size">
        <span>{{ copy.rowsPerPage }}</span>
        <select :value="pageSize" class="form-select form-select-sm" :disabled="disabled" @change="changePageSize">
          <option v-for="size in pageSizeOptions" :key="size" :value="size">{{ size }}</option>
        </select>
      </label>
    </div>

    <div class="list-pagination__controls">
      <nav class="list-pagination__pages" :aria-label="copy.navigation">
        <button type="button" class="btn btn-sm btn-outline-secondary" :disabled="disabled || page <= 1" :aria-label="copy.previous" @click="emitPage(page - 1)"><i class="bi bi-chevron-left"></i></button>
        <template v-for="(token, index) in pageItems" :key="`${token}-${index}`">
          <span v-if="token === 'ellipsis'" class="list-pagination__ellipsis" aria-hidden="true">&hellip;</span>
          <button v-else type="button" class="btn btn-sm" :class="token === page ? 'btn-primary' : 'btn-outline-secondary'" :disabled="disabled" :aria-current="token === page ? 'page' : undefined" :aria-label="copy.goTo.replace('{page}', String(token))" @click="emitPage(token)">{{ token }}</button>
        </template>
        <button type="button" class="btn btn-sm btn-outline-secondary" :disabled="disabled || page >= pageCount" :aria-label="copy.next" @click="emitPage(page + 1)"><i class="bi bi-chevron-right"></i></button>
      </nav>

      <form class="list-pagination__jump" @submit.prevent="submitJump">
        <label :for="jumpInputID" class="small text-body-secondary">{{ copy.jumpTo }}</label>
        <input :id="jumpInputID" v-model="jumpPage" type="number" min="1" :max="pageCount" inputmode="numeric" class="form-control form-control-sm" :disabled="disabled" :aria-label="copy.jumpTo">
        <span class="small text-body-secondary">{{ copy.pageUnit }}</span>
        <button type="submit" class="btn btn-sm btn-outline-secondary" :disabled="disabled">{{ copy.confirm }}</button>
      </form>
    </div>
  </footer>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  page: { type: Number, default: 1 },
  pageSize: { type: Number, default: 10 },
  total: { type: Number, default: 0 },
  pageSizeOptions: { type: Array, default: () => [10, 20, 50, 100] },
  disabled: { type: Boolean, default: false },
  showPageSize: { type: Boolean, default: true },
  idPrefix: { type: String, default: 'list' }
})
const emit = defineEmits(['update:page', 'update:pageSize'])
const { locale } = useI18n()
const jumpPage = ref(props.page)
const pageCount = computed(() => Math.max(1, Math.ceil(Number(props.total || 0) / Math.max(1, Number(props.pageSize || 1)))))
const jumpInputID = computed(() => `${props.idPrefix}-page-jump`)
const copy = computed(() => String(locale.value || 'zh').toLowerCase().startsWith('en') ? {
  total: '{total} records', rowsPerPage: 'Rows per page', navigation: 'Pagination', previous: 'Previous page', next: 'Next page', goTo: 'Go to page {page}', jumpTo: 'Go to', pageUnit: 'page', confirm: 'Go'
} : {
  total: '\u5171 {total} \u6761\u8bb0\u5f55', rowsPerPage: '\u6bcf\u9875\u663e\u793a', navigation: '\u5217\u8868\u5206\u9875', previous: '\u4e0a\u4e00\u9875', next: '\u4e0b\u4e00\u9875', goTo: '\u524d\u5f80\u7b2c {page} \u9875', jumpTo: '\u524d\u5f80', pageUnit: '\u9875', confirm: '\u786e\u5b9a'
})
const pageItems = computed(() => paginationItems(props.page, pageCount.value))

watch(() => props.page, value => { jumpPage.value = value })

function paginationItems(currentPage, totalPages) {
  if (totalPages <= 7) return Array.from({ length: totalPages }, (_, index) => index + 1)
  const pages = new Set([1, totalPages, currentPage - 1, currentPage, currentPage + 1])
  const sorted = [...pages].filter(value => value >= 1 && value <= totalPages).sort((left, right) => left - right)
  const result = []
  for (const value of sorted) {
    if (result.length && value - result[result.length - 1] > 1) result.push('ellipsis')
    result.push(value)
  }
  return result
}

function emitPage(target) {
  const next = Math.min(pageCount.value, Math.max(1, Number(target) || 1))
  jumpPage.value = next
  if (next !== props.page) emit('update:page', next)
}

function changePageSize(event) {
  const next = Math.max(1, Number(event.target.value) || props.pageSize)
  emit('update:pageSize', next)
}

function submitJump() {
  emitPage(jumpPage.value)
}
</script>

<style scoped>
.list-pagination { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: .75rem 1rem; padding: .5rem 1rem; border-top: 1px solid var(--bs-border-color); background: var(--bs-body-bg); }
.list-pagination__summary, .list-pagination__controls, .list-pagination__pages, .list-pagination__jump, .list-pagination__page-size { display: flex; align-items: center; gap: .4rem; }
.list-pagination__summary { flex-wrap: wrap; gap: .55rem 1rem; }
.list-pagination__controls { flex-wrap: wrap; justify-content: flex-end; }
.list-pagination__page-size { color: var(--bs-secondary-color); font-size: .875rem; white-space: nowrap; }
.list-pagination__page-size .form-select { width: 4.75rem; }
.list-pagination__pages .btn { min-width: 2rem; }
.list-pagination__ellipsis { min-width: 1.25rem; text-align: center; color: var(--bs-secondary-color); }
.list-pagination__jump { white-space: nowrap; }
.list-pagination__jump .form-control { width: 4.25rem; text-align: center; }
@media (max-width: 767.98px) {
  .list-pagination, .list-pagination__controls { align-items: stretch; flex-direction: column; }
  .list-pagination__controls { width: 100%; }
  .list-pagination__pages { flex-wrap: wrap; }
  .list-pagination__jump { justify-content: flex-end; }
}
</style>
