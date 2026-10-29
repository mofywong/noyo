<template>
  <section class="operations-panel" :aria-busy="busy || loading">
    <div class="toolbar"><h2 class="h5">{{ t('operations') }}</h2><button class="btn btn-outline-secondary" :disabled="busy || loading" @click="load">{{ t('refresh') }}</button></div>
    <p v-if="loading" role="status">{{ t('loading') }}</p><p v-if="error" role="alert">{{ t('error') }}</p>
    <div class="summary">
      <button v-for="item in indicators" :key="item.key" class="indicator" @click="$emit('filter',item.filter)"><span>{{ t(item.key) }}</span><strong>{{ summary[item.field] || 0 }}</strong></button>
    </div>
    <p v-if="summary.oldest_assignment_at">{{ t('oldest') }}: {{ date(summary.oldest_assignment_at) }}</p>
    <template v-if="canManage">
      <h3 class="h6">{{ t('failedDelivery') }}</h3>
      <div v-if="!failures.length && !loading" class="empty">{{ t('empty') }}</div>
      <article v-for="failure in failures" :key="id(failure)" class="failure">
        <strong>{{ failure.event_id }}</strong><p>{{ t('status') }}: {{ t(failure.status) }} · {{ t('attempts') }}: {{ failure.attempt_count }}</p><p>{{ t('lastError') }}: {{ failure.last_error }}</p>
        <label>{{ t('reason') }}<input v-model.trim="reasons[id(failure)]" class="form-control" maxlength="2000"></label>
        <div class="toolbar"><button class="btn btn-outline-primary" :disabled="busy || !reasons[id(failure)]" @click="recover(failure,false)">{{ t('retry') }}</button><button v-if="failure.status !== 'stopped'" class="btn btn-outline-danger" :disabled="busy || !reasons[id(failure)]" @click="recover(failure,true)">{{ t('stop') }}</button></div>
      </article>
    </template>
  </section>
</template>
<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { workOrderApi } from '../../utils/workOrderApi.js'
import { operationID as id, operationText } from './workOrderOperations.js'
const props = defineProps({ locale: String, canManage: Boolean })
defineEmits(['filter'])
const summary=ref({}),failures=ref([]),reasons=ref({}),busy=ref(false),loading=ref(false),error=ref(false)
const t=key=>operationText(props.locale,key),date=value=>new Date(value).toLocaleString(props.locale)
const indicators=[{key:'blocked',field:'pending_assignment',filter:'blocked'},{key:'claimable',field:'claimable',filter:'claimable'},{key:'breached',field:'sla_breached',filter:'sla_breached'},{key:'failedDelivery',field:'failed_delivery',filter:'delivery_failed'}]
let generation=0
async function load(){const stamp=++generation;loading.value=true;error.value=false;try{const [a,b]=await Promise.all([workOrderApi.operationsSummary(),props.canManage?workOrderApi.failedDeliveries():Promise.resolve({data:{data:[]}})]);if(stamp!==generation)return;summary.value=a.data.data||{};failures.value=b.data.data||[]}catch{if(stamp===generation)error.value=true}finally{if(stamp===generation)loading.value=false}}
async function recover(event,stop){if(!props.canManage||busy.value||!reasons.value[id(event)])return;if(stop&&!window.confirm(t('stopConfirm')))return;busy.value=true;error.value=false;try{await workOrderApi.recoverDelivery(id(event),{expected_attempts:event.attempt_count,stop,reason:reasons.value[id(event)]});await load()}catch{error.value=true}finally{busy.value=false}}
onMounted(load);onBeforeUnmount(()=>generation++)
</script>
<style scoped>
.operations-panel{flex:1;min-height:0;padding:24px;overflow:auto;color:var(--text-primary);background:var(--bg-surface)}.toolbar{display:flex;gap:16px;align-items:center;flex-wrap:wrap;margin-bottom:16px}.summary{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:16px;margin-bottom:24px}.indicator{display:grid;gap:8px;padding:24px;border:1px solid var(--border-color);border-radius:8px;color:var(--text-primary);background:var(--bg-surface);text-align:start}.indicator strong{font-size:2rem}.indicator:focus-visible{outline:2px solid var(--text-primary);outline-offset:2px}.failure{border:1px solid var(--border-color);border-radius:8px;padding:24px;margin-bottom:16px;overflow-wrap:anywhere}.failure label{display:grid;gap:8px;margin-bottom:16px}.empty{padding:96px 24px;text-align:center;color:var(--text-secondary)}
</style>
