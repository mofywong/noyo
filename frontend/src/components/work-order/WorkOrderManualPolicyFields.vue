<template>
  <fieldset class="manual-policy">
    <label>{{ t('policies') }}<select :value="modelValue || 0" class="form-select" :disabled="loading" @change="choose(Number($event.target.value))"><option :value="0">{{ text('直接使用流程模板','Use workflow template directly') }}</option><option v-for="version in versions" :key="version.id" :value="version.id">{{ version.name }}</option></select></label>
    <div v-if="modelValue" class="context-fields"><label>{{ t('location') }}<input :value="routingContext?.location" class="form-control" maxlength="128" @input="patch('location',$event.target.value)" /></label><label>{{ t('device') }}<input :value="routingContext?.device_code" class="form-control" maxlength="128" @input="patch('device_code',$event.target.value)" /></label><label>{{ t('issue') }}<input :value="routingContext?.issue_code" class="form-control" maxlength="128" @input="patch('issue_code',$event.target.value)" /></label></div>
    <p v-if="error" role="alert">{{ t('error') }} <button type="button" class="btn btn-sm btn-outline-secondary" @click="load">{{ t('refresh') }}</button></p>
  </fieldset>
</template>
<script setup>
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { workOrderApi } from '../../utils/workOrderApi.js'
import { operationID, operationText } from './workOrderOperations.js'
const props=defineProps({modelValue:Number,routingContext:Object,locale:String})
const emit=defineEmits(['update:modelValue','update:routingContext','resolved','loading'])
const policies=ref([]),loading=ref(false),error=ref(false)
const t=key=>operationText(props.locale,key),text=(zh,en)=>String(props.locale).startsWith('en')?en:zh
const versions=computed(()=>policies.value.filter(entry=>entry.policy.enabled).flatMap(entry=>entry.versions.map(version=>({id:operationID(version),name:`${entry.policy.name} · ${t('version')} ${version.version}`}))))
let generation=0
const patch=(key,value)=>emit('update:routingContext',{...props.routingContext,[key]:value})
async function load(){const stamp=++generation;error.value=false;try{const response=await workOrderApi.policies();if(stamp===generation)policies.value=response.data.data||[]}catch{if(stamp===generation)error.value=true}}
async function choose(version){const stamp=++generation;emit('update:modelValue',version);emit('resolved',null);if(!version){loading.value=false;emit('loading',false);return;}loading.value=true;emit('loading',true);error.value=false;try{const response=await workOrderApi.policyVersion(version);if(stamp===generation)emit('resolved',response.data.data)}catch{if(stamp===generation)error.value=true}finally{if(stamp===generation){loading.value=false;emit('loading',false)}}}
onMounted(load);onBeforeUnmount(()=>generation++)
</script>
<style scoped>
.manual-policy{display:grid;gap:16px;margin-bottom:24px;color:var(--text-primary)}label{display:grid;gap:8px}.context-fields{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px}p{color:var(--text-secondary)}
</style>
