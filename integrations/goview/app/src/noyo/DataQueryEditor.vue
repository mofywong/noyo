<template>
  <section class="data-query" :aria-label="t('数据集查询', 'Dataset query')">
    <fieldset :disabled="busy" style="display:contents">
    <p class="hint">{{ t('先配置统计口径并预览，再将结果字段映射到组件。', 'Configure and preview the query, then map its output fields to the widget.') }}</p>
    <details :open="!binding">
      <summary>{{ t('快速配置', 'Quick configuration') }}</summary>
      <label>{{ t('数据源', 'Source') }}<select v-model="form.source"><option v-for="item in sources" :key="item[0]" :value="item[0]">{{ t(item[1], item[2]) }}</option></select></label>
      <label v-if="!['static','workorders'].includes(form.source)">{{ t('产品范围', 'Product scope') }}<input v-model="form.product" list="query-products" :placeholder="t('留空则按设备或当前可读范围', 'Leave empty for devices or the readable scope')" /><datalist id="query-products"><option v-for="product in products" :key="product" :value="product" /></datalist></label>
      <label v-if="!['static','workorders'].includes(form.source)">{{ t('设备编码（可选，逗号分隔）', 'Device codes (optional, comma separated)') }}<textarea v-model="form.devices" rows="2" /></label>
      <label>{{ t('统计字段', 'Measure field') }}<input v-model="form.field" list="query-fields" /><datalist id="query-fields"><option v-for="field in fields" :key="field.key" :value="field.key">{{ english ? field.nameEn : field.nameZh }}</option></datalist></label>
      <label>{{ t('统计方法', 'Aggregation') }}<select v-model="form.method"><option v-for="item in methods" :key="item[0]" :value="item[0]">{{ t(item[1], item[2]) }}</option></select></label>
      <label>{{ t('统计权重', 'Weighting') }}<select v-model="form.weight"><option value="sample">{{ t('每个有效样本等权', 'Equal weight per valid sample') }}</option><option value="device">{{ t('先算各设备均值，再等权汇总', 'Average per device, then aggregate equally') }}</option></select></label>
      <label>{{ t('分组字段（可选，逗号分隔）', 'Group fields (optional, comma separated)') }}<input v-model="form.group" :placeholder="t('例如 device_code、severity', 'For example device_code, severity')" /></label>
      <div class="field-pair"><label>{{ t('最近小时数', 'Lookback hours') }}<input v-model.number="form.hours" type="number" min="1" max="8784" /></label><label>{{ t('时间粒度（分钟，0为不分桶）', 'Bucket minutes (0 for none)') }}<input v-model.number="form.minutes" type="number" min="0" /></label></div>
      <label>{{ t('时区', 'Timezone') }}<input v-model="form.timezone" /></label>
      <button type="button" :disabled="busy" @click="generate">{{ t('生成查询配置', 'Generate query configuration') }}</button>
    </details>
    <label>{{ t('查询配置 JSON', 'Query configuration JSON') }}<textarea v-model="planText" rows="12" spellcheck="false" @input="detach" /></label>
    <p class="hint">{{ t('支持 filter、derive、bucket、aggregate、sort、limit、project、join、union。公式可用 row["字段"]、param["参数"]、abs、round、coalesce、ifelse；除零和缺失值保留为空。时间范围左闭右开。', 'Supports filter, derive, bucket, aggregate, sort, limit, project, join and union. Expressions support row["field"], param["parameter"], abs, round, coalesce and ifelse. Missing values and division by zero remain null. Time ranges are [from, to).') }}</p>
    <label>{{ t('参数值 JSON', 'Parameter values JSON') }}<textarea v-model="parametersText" rows="2" spellcheck="false" @input="detach" /></label>
    <button type="button" :disabled="busy" @click="preview">{{ busy ? t('查询中…', 'Querying…') : t('预览查询', 'Preview query') }}</button>
    <p v-if="error" role="alert" class="query-error">{{ error }}</p>
    <template v-if="result">
      <p role="status">{{ t('结果', 'Result') }}: {{ result.rows.length }} · {{ t('扫描行数', 'Rows scanned') }}: {{ result.meta.scannedRows }} · {{ result.meta.cacheHit ? t('缓存命中', 'Cache hit') : t('实时计算', 'Computed') }}</p>
      <p class="hint">{{ new Date(result.meta.from).toLocaleString() }} → {{ new Date(result.meta.to).toLocaleString() }} · {{ result.meta.timezone }}</p>
      <div v-if="result.rows.length" class="query-preview" tabindex="0" :aria-label="t('查询结果，最多预览20行', 'Query results, first 20 rows')"><table><thead><tr><th v-for="field in result.fields" :key="field.id">{{ field.id }}</th></tr></thead><tbody><tr v-for="(row, index) in result.rows.slice(0,20)" :key="index"><td v-for="field in result.fields" :key="field.id">{{ row[field.id] ?? '—' }}</td></tr></tbody></table></div>
      <p v-else class="query-empty">{{ t('此范围内暂无数据', 'No data in this range') }}</p>
    </template>
    <datalist id="query-output-fields"><option v-for="field in result?.fields || []" :key="field.id" :value="field.id" /></datalist>
    <label>{{ t('数值字段', 'Value field') }}<input v-model="mapping.value" list="query-output-fields" /></label>
    <label>{{ t('分类或时间字段', 'Category or time field') }}<input v-model="mapping.category" list="query-output-fields" /></label>
    <label>{{ t('系列字段（可选）', 'Series field (optional)') }}<input v-model="mapping.series" list="query-output-fields" /></label>
    <label>{{ t('输出列（可选，逗号分隔）', 'Output columns (optional, comma separated)') }}<input v-model="columns" /></label>
    <button type="button" :disabled="busy" @click="apply">{{ t('应用数据集绑定', 'Apply dataset binding') }}</button>
    <details><summary>{{ t('复用数据集', 'Reusable datasets') }}</summary>
      <label>{{ t('项目数据集', 'Project dataset') }}<select v-model="chosen" @focus="loadDatasets" @change="chooseDataset"><option value="">{{ t('选择数据集…', 'Choose a dataset…') }}</option><option v-for="item in datasets" :key="item.id" :value="item.id">{{ item.name }} · v{{ item.version }}</option></select></label>
      <p v-if="pinned" class="hint">{{ t('固定版本', 'Pinned version') }}: {{ pinned.id }} / v{{ pinned.version }}</p>
      <label>{{ t('数据集名称', 'Dataset name') }}<input v-model="name" maxlength="128" /></label>
      <button type="button" :disabled="busy || !name.trim()" @click="saveDataset">{{ t('另存为项目数据集', 'Save as project dataset') }}</button>
      <button v-if="pinned" type="button" :disabled="busy || !name.trim()" @click="saveDataset(true)">{{ t('保存新版本', 'Save a new version') }}</button>
    </details>
    </fieldset>
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { rpc, state } from './bridge'
const props = defineProps<{ binding?: any; catalog: any[]; shape: string }>()
const emit = defineEmits<{ (event: 'apply', binding: any): void }>()
const english = computed(() => state.locale.startsWith('en'))
const t = (zh: string, en: string) => english.value ? en : zh
const sources = [['telemetry','历史遥测','Historical telemetry'],['latest','最新遥测','Latest telemetry'],['devices','设备目录与状态','Device inventory and status'],['alarms','告警记录','Alarm records'],['workorders','工单记录','Work orders'],['static','静态数据','Static data']]
const methods = [['avg','平均值','Average'],['sum','求和','Sum'],['min','最小值','Minimum'],['max','最大值','Maximum'],['count','行数','Row count'],['count_distinct','去重计数','Distinct count'],['median','中位数','Median']]
const form = reactive({ source:'telemetry',product:'',devices:'',field:'',method:'avg',weight:'sample',group:'',hours:24,minutes:['number','percent','progress','text'].includes(props.shape)?0:60,timezone:'Asia/Shanghai' })
const products = computed(() => [...new Set(props.catalog.map(item => item.productCode).filter(Boolean))])
const fields = computed(() => props.catalog.find(item => !form.product || item.productCode === form.product)?.fields || [])
const planText = ref(''), parametersText = ref('{}'), columns = ref(''), name = ref(''), chosen = ref('')
const mapping = reactive({ value:'value',category:'ts',series:'' })
const pinned = ref<any>(null), datasets = ref<any[]>([]), result = ref<any>(null), error = ref(''), busy = ref(false)
const pinnedPlan = ref('')
const split = (text: string) => [...new Set(text.split(',').map(value => value.trim()).filter(Boolean))]
let sequence = 0
function detach() { result.value = null; sequence++ }
function generate() {
  if (!form.field) form.field=fields.value[0]?.key||''
  const source: any = { id:'raw',kind:form.source,selector:{ ...(form.product ? {productCode:form.product}:{}), ...(form.devices ? {deviceCodes:split(form.devices)}:{}) } }
  if (['telemetry','latest'].includes(form.source)) source.fields = form.field ? [form.field] : []
  if (form.source === 'static') { source.rows = [{ name:'A', value:10 },{ name:'B', value:20 }]; delete source.selector }
  if (form.source === 'workorders') delete source.selector
  const steps: any[] = []; let input = 'raw'; const group = split(form.group)
  if (form.minutes > 0 && form.source !== 'static') { steps.push({id:'bucketed',op:'bucket',input,field:'ts',...(form.minutes===60?{calendar:'hour'}:form.minutes===1440?{calendar:'day'}:{intervalMs:form.minutes*60000})});input='bucketed';group.unshift('ts') }
  if (form.weight === 'device') { steps.push({id:'per_device',op:'aggregate',input,groupBy:[...new Set([...group,'device_code'])],measures:[{id:'device_avg',function:'avg',field:form.field}]});input='per_device' }
  const measure: any = { id:'value',function:form.method }
  if (form.method !== 'count') measure.field = form.weight === 'device' ? 'device_avg' : form.field || 'value'
  steps.push({id:'stats',op:'aggregate',input,groupBy:group,measures:[measure]});input='stats'
  if(group.length){steps.push({id:'ordered',op:'sort',input,orders:group.map(field => ({field}))});input='ordered'}
  planText.value=JSON.stringify({schemaVersion:'noyo.dataset/1',sources:[source],steps,output:input,windowMs:form.hours*3600000,timezone:form.timezone,maxRows:1000},null,2)
  mapping.value='value';mapping.category=group[0]||'';mapping.series=group.find(field => field!=='ts')||'';pinned.value=null;detach()
}
function config() { const plan=JSON.parse(planText.value), parameters=JSON.parse(parametersText.value);if(!parameters||Array.isArray(parameters)||typeof parameters!=='object')throw new Error(t('参数值必须是 JSON 对象', 'Parameter values must be a JSON object'));return {plan,parameters} }
async function preview() {
  const request=++sequence;busy.value=true;error.value='';result.value=null
  try {const value=await rpc('dataset.preview',config());if(request===sequence)result.value=value;return request===sequence ? value : null}
  catch(err){if(request===sequence)error.value=(err as Error).message;return null}
  finally{busy.value=false}
}
async function apply() {
  const data=await preview();if(!data)return
  try {
    const map={...mapping,columns:split(columns.value)}
    const scalar=['number','text','percent','progress','flip'].includes(props.shape)
    if(scalar&&data.rows.length>1)throw new Error(t('单值组件需要一行结果，请移除分组或增加汇总步骤。', 'A scalar widget needs one result row. Remove grouping or add a final aggregation.'))
    if(data.rows.length&&map.value&&!data.fields.some((field:any)=>field.id===map.value))throw new Error(t('数值字段不在查询结果中', 'Value field is missing from the result'))
    const {plan,parameters}=config();emit('apply',{kind:'dataset',...(pinned.value && pinnedPlan.value===JSON.stringify(plan)?{dataset:{...pinned.value}}:{query:plan}),parameters,mapping:map})
  }catch(err){error.value=(err as Error).message}
}
async function loadDatasets(){try{datasets.value=(await rpc('dataset.list')).datasets||[]}catch(err){error.value=(err as Error).message}}
async function chooseDataset(){
  const item=datasets.value.find(item=>String(item.id)===String(chosen.value));if(!item)return
  busy.value=true;error.value=''
  try{const value=await rpc('dataset.load',{id:item.id,version:item.version});planText.value=JSON.stringify(value.plan,null,2);pinnedPlan.value=JSON.stringify(value.plan);pinned.value=value.dataset;name.value=item.name;detach()}
  catch(err){error.value=(err as Error).message}finally{busy.value=false}
}
async function saveDataset(update: any = false){
  if(update!==true)update=false
  const data=await preview();if(!data)return;busy.value=true
  try{const {plan,parameters}=config();const saved=await rpc('dataset.save',{...(update&&pinned.value?{id:pinned.value.id,baseVersion:pinned.value.version}:{}),name:name.value,plan,parameters});pinned.value={id:saved.id,version:saved.version};pinnedPlan.value=JSON.stringify(plan);await loadDatasets()}
  catch(err){error.value=(err as Error).message}finally{busy.value=false}
}
watch(()=>props.binding,async binding=>{
  sequence++;result.value=null;error.value='';pinned.value=binding?.dataset?{...binding.dataset}:null;parametersText.value=JSON.stringify(binding?.parameters||{},null,2)
  Object.assign(mapping,{value:'value',category:'ts',series:'',...binding?.mapping});columns.value=(binding?.mapping?.columns||[]).join(',')
  if(binding?.query)planText.value=JSON.stringify(binding.query,null,2)
  else if(binding?.dataset){const request=sequence;try{const value=await rpc('dataset.load',binding.dataset);if(request===sequence){planText.value=JSON.stringify(value.plan,null,2);pinnedPlan.value=JSON.stringify(value.plan)}}catch(err){error.value=(err as Error).message}}
  else generate()
},{immediate:true})
watch(()=>form.source,source=>{if(['devices','alarms','workorders','static'].includes(source)){form.minutes=0;form.weight='sample';form.method=source==='static'?'avg':'count';form.field=source==='static'?'value':'';form.group=source==='alarms'?'severity':source==='workorders'?'status':source==='static'?'name':''}})
</script>

<style scoped>
.data-query{display:grid;gap:8px;padding:24px 0}.data-query details{padding:8px 0}.data-query summary{cursor:pointer;margin-bottom:16px}.data-query label{margin-bottom:8px}.data-query textarea{font:12px/1.5 ui-monospace,monospace;resize:vertical}.query-error{color:var(--danger-color);overflow-wrap:anywhere}.query-preview{max-height:240px;overflow:auto;border:1px solid var(--border-color);border-radius:8px}.query-preview table{border-collapse:collapse;font-size:12px;white-space:nowrap}.query-preview th,.query-preview td{padding:8px;border-bottom:1px solid var(--border-color);text-align:left}.query-preview th{background:var(--bg-body)}.query-empty{padding:96px 24px;text-align:center;color:var(--text-secondary)}
</style>
