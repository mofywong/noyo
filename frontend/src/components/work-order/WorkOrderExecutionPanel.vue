<template>
  <section v-if="claimable.length || sla || error" class="execution-panel" :aria-label="t('sla')">
    <p v-if="error" role="alert">{{ t('error') }} <button type="button" class="btn btn-outline-secondary btn-sm" @click="load">{{ t('refresh') }}</button></p>
    <button v-for="task in claimable" :key="operationID(task)" type="button" class="btn btn-primary" :disabled="busy || !canProcess" @click="claim(task)">{{ t('claimTask') }} · {{ nodeName(task.node_id) }}</button>
    <div v-if="sla">
      <h3 class="h6">{{ t('sla') }} · {{ t(sla.completed_at ? 'completed' : sla.paused_at ? 'paused' : 'running') }}</h3>
      <p>{{ t('responseDue') }}: {{ date(sla.response_due_at) }} · {{ t('resolutionDue') }}: {{ date(sla.resolution_due_at) }}</p>
      <p v-if="sla.response_breached || sla.resolution_breached" role="status">{{ t('breached') }}</p>
      <p v-if="sla.responded_at">{{ t('responded') }}: {{ date(sla.responded_at) }}</p>
      <div v-if="canManage && !sla.completed_at && (sla.paused_at || pauseReasons.length)" class="controls">
        <label>{{ t('reason') }}<select v-if="!sla.paused_at" v-model="reason" class="form-select"><option value="">{{ t('select') }}</option><option v-for="value in pauseReasons" :key="value">{{ value }}</option></select><input v-else v-model.trim="reason" class="form-control" maxlength="2000"></label>
        <button type="button" class="btn btn-outline-secondary" :disabled="busy || !reason" @click="pause">{{ t(sla.paused_at ? 'resume' : 'pause') }}</button>
      </div>
    </div>
  </section>
</template>
<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { workOrderApi, checkedWorkOrderResponse } from '../../utils/workOrderApi.js'
import { candidateTasks, operationText, operationID } from './workOrderOperations.js'
const props = defineProps({ orderId: [Number, String], version: Number, cycle: Number, actorId: [Number, String], workflow: Object, locale: String, canProcess: Boolean, canManage: Boolean })
const emit = defineEmits(['refresh'])
const sla = ref(null), tasks = ref([]), error = ref(false), busy = ref(false), reason = ref('')
let generation = 0
const t = key => operationText(props.locale, key)
const claimable = computed(() => candidateTasks(tasks.value, props.actorId, props.cycle))
const pauseReasons = computed(() => { try { return JSON.parse(sla.value?.policy_snapshot || '{}').pause_reasons || [] } catch { return [] } })
const date = value => new Date(value).toLocaleString(props.locale)
const nodeName = id => props.workflow?.nodes?.find(node => node.id === id)?.name || id
async function load() { const stamp = ++generation; error.value = false; try { const [a,b] = await Promise.all([workOrderApi.tasks(props.orderId), workOrderApi.sla(props.orderId)]); if (stamp !== generation) return; checkedWorkOrderResponse(a); tasks.value = a.data.data || []; sla.value = b.data.data; reason.value = '' } catch { if (stamp === generation) error.value = true } }
async function mutate(action) { if (busy.value) return; busy.value = true; error.value = false; try { await action(); emit('refresh'); await load() } catch { error.value = true; emit('refresh') } finally { busy.value = false } }
function claim(task) { if (!props.canProcess || !claimable.value.some(item => operationID(item) === operationID(task))) return; return mutate(() => workOrderApi.claimTask(props.orderId, operationID(task), props.version)) }
function pause() { if (!props.canManage || !reason.value || !sla.value || sla.value.completed_at) return; return mutate(() => workOrderApi.pauseSLA(props.orderId, { expected_version: props.version, pause: !sla.value.paused_at, reason: reason.value })) }
watch(() => [props.orderId, props.version], load, { immediate: true })
onBeforeUnmount(() => generation++)
</script>
<style scoped>
.execution-panel{padding:24px;margin-bottom:24px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-surface);color:var(--text-primary);display:grid;gap:16px}.controls{display:flex;align-items:end;gap:16px;flex-wrap:wrap}.controls label{display:grid;gap:8px}p{margin:8px 0;color:var(--text-secondary)}
</style>
