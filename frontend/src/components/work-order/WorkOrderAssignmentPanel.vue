<template>
  <section v-if="blocks.length || error || loading" class="mb-4 border rounded p-3 bg-body-tertiary" :aria-busy="loading || busy">
    <div class="fw-semibold mb-2">{{ copy.title }}</div>
    <p v-if="loading" class="small text-body-secondary mb-0" role="status">{{ copy.loading }}</p>
    <div v-if="error" class="alert alert-danger" role="alert">
      {{ error }} <button type="button" class="btn btn-sm btn-outline-danger ms-2" :disabled="busy || loading" @click="refresh">{{ copy.refresh }}</button>
    </div>
    <p v-if="blocks.length" class="small text-body-secondary">{{ copy.hint }}</p>
    <div v-for="block in blocks" :key="block.ID" class="border-top pt-3 mt-3">
      <div class="fw-semibold">{{ workOrderNodeLabel(workflow, block.node_id, locale) }}</div>
      <p class="small text-body-secondary mt-2">{{ copy[block.dispatch_error] || copy.unavailable }}</p>
      <template v-if="canManage">
        <button type="button" class="btn btn-sm btn-outline-primary" :disabled="busy || loading" @click="dispatch(block)">{{ copy.retry }}</button>
        <form class="mt-3" @submit.prevent="dispatch(block, true)">
          <label :for="`dispatch-member-${block.ID}`" class="form-label small">{{ copy.member }}</label>
          <select :id="`dispatch-member-${block.ID}`" v-model.number="draft(block.ID).userID" class="form-select form-select-sm" :disabled="busy || loading" required>
            <option :value="0" disabled>{{ copy.choose }}</option>
            <option v-for="member in participants" :key="member.id" :value="member.id">{{ member.display_name || member.username }}</option>
          </select>
          <label :for="`dispatch-reason-${block.ID}`" class="form-label small mt-3">{{ copy.reason }}</label>
          <textarea :id="`dispatch-reason-${block.ID}`" v-model.trim="draft(block.ID).reason" class="form-control form-control-sm" rows="2" maxlength="2000" :disabled="busy || loading" required></textarea>
          <button type="submit" class="btn btn-sm btn-primary mt-3" :disabled="busy || loading || !draft(block.ID).userID || !draft(block.ID).reason">{{ copy.assign }}</button>
        </form>
      </template>
    </div>
  </section>
</template>

<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { workOrderApi } from '../../utils/workOrderApi.js'
import { workOrderAssignmentCopy } from '../../utils/workOrderAssignment.js'
import { workOrderNodeLabel } from '../../utils/workOrderUi.js'

const props = defineProps({
  orderId: { type: Number, required: true }, version: { type: Number, required: true },
  workflow: { type: Object, required: true }, locale: { type: String, default: 'zh' },
  canManage: { type: Boolean, default: false }, participants: { type: Array, default: () => [] },
})
const emit = defineEmits(['refresh'])
const copy = computed(() => workOrderAssignmentCopy(props.locale))
const blocks = ref([])
const drafts = reactive({})
const loading = ref(false)
const busy = ref(false)
const error = ref('')
let generation = 0

function draft(id) {
  return drafts[id] || (drafts[id] = { userID: 0, reason: '' })
}

function refresh() {
  emit('refresh', props.orderId)
  loadBlocks()
}

async function loadBlocks() {
  const request = ++generation
  loading.value = true
  error.value = ''
  try {
    const response = await workOrderApi.assignmentBlocks(props.orderId)
    if (request !== generation) return
    if (response.data.code !== 0) throw new Error('load failed')
    blocks.value = response.data.data || []
  } catch {
    if (request === generation) error.value = copy.value.loadFailed
  } finally {
    if (request === generation) loading.value = false
  }
}

async function dispatch(block, manual = false) {
  if (busy.value || !props.canManage) return
  const id = props.orderId
  busy.value = true
  error.value = ''
  try {
    const response = await workOrderApi.dispatchNode(id, block.ID, {
      expected_version: props.version,
      user_ids: manual ? [draft(block.ID).userID] : [],
      comment: manual ? draft(block.ID).reason : '',
    })
    if (response.data.code !== 0) throw new Error('dispatch failed')
    if (id === props.orderId) emit('refresh', id)
  } catch {
    if (id === props.orderId) error.value = copy.value.failed
  } finally {
    busy.value = false
  }
}

watch(() => [props.orderId, props.version], () => { blocks.value = []; loadBlocks() }, { immediate: true })
</script>
