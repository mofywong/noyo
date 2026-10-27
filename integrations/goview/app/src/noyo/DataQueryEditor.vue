<template>
  <section class="dq" :aria-label="t('自定义数据', 'Custom data')">
    <fieldset :disabled="busy" class="dq-fieldset">

      <p v-if="!catalog.length" class="dq-hint">{{ t('正在读取设备目录，稍候即可选择设备与指标。', 'Reading the device catalogue; the device and measure pickers appear shortly.') }}</p>

      <div v-if="showTip" class="dq-tip">
        <span>{{ t('只要一台设备的一个值？用上面的「最新值」即可。这里用于跨设备统计、分组汇总与多步计算。', 'One value from one device? The "Latest value" kind above is simpler. Use this for cross-device totals, grouped summaries and multi-step maths.') }}</span>
        <button type="button" :aria-label="t('知道了', 'Got it')" @click="dismissTip">×</button>
      </div>

      <div v-if="jsonOnly && jsonReason === 'adopted'" class="dq-notice">
        <p>{{ t('这个组件已在用高级查询配置（含筛选、派生或多数据源），下面的引导不会改动它。可以用「预览查询」验证，或在高级区继续编辑。', 'This widget already uses an advanced query (filter, derive or several sources), so the guide leaves it untouched. Use "Preview query" to verify it, or keep editing under Advanced.') }}</p>
        <button type="button" @click="restartGuide">{{ t('改用引导重新配置', 'Reconfigure from scratch') }}</button>
      </div>

      <template v-else>
        <div class="dq-step">
          <p class="dq-q"><span class="dq-n">1</span>{{ t('这块显示什么', 'What should it show') }}</p>
          <div class="dq-grid">
            <button v-for="item in recipes" :key="item.id" type="button" class="dq-card" :class="{ 'is-on': recipeId === item.id }" @click="chooseRecipe(item)">
              <b>{{ t(item.zh, item.en) }}</b>
              <small>{{ t(item.descZh, item.descEn) }}</small>
            </button>
          </div>
        </div>

        <template v-if="recipeId">
          <div class="dq-step">
            <p class="dq-q"><span class="dq-n">{{ stepNo.devices }}</span>{{ t('统计哪些设备', 'Which devices') }}</p>
            <div v-if="picked.length" class="dq-tags">
              <span v-for="code in picked" :key="code" class="dq-tag">{{ deviceName(code) }}<button type="button" :aria-label="t('移除该设备', 'Remove this device')" @click="toggleDevice(code)">×</button></span>
            </div>
            <input v-model="deviceQuery" :placeholder="picked.length ? t('继续添加设备…', 'Add another device…') : t('搜索设备名称或编码；不选则统计全部可读设备', 'Search by name or code; empty means every readable device')" @focus="showDevices = true" />
            <ul v-if="showDevices && deviceMatches.length" class="dq-list">
              <li v-for="item in deviceMatches" :key="item.code">
                <button type="button" :aria-pressed="picked.includes(item.code)" @click="toggleDevice(item.code)">
                  <i class="dq-dot" :class="item.online ? 'is-online' : 'is-offline'" />{{ item.name }}<small>{{ item.code }} · {{ item.productName }}</small>
                </button>
              </li>
            </ul>
            <p v-else-if="showDevices && deviceQuery" class="dq-hint">{{ t('没有匹配的设备', 'No matching device') }}</p>
            <p v-else-if="!showDevices && picked.length" class="dq-hint">{{ t('点输入框可以继续添加设备', 'Tap the field to add more devices') }}</p>
          </div>

          <div v-if="needField" class="dq-step">
            <p class="dq-q"><span class="dq-n">{{ stepNo.field }}</span>{{ t('看哪个指标', 'Which measure') }}</p>
            <select v-if="availableFields.length" v-model="form.field">
              <option v-for="field in availableFields" :key="field.key" :value="field.key">{{ fieldLabel(field) }}</option>
            </select>
            <p v-else class="dq-hint">{{ t('先选设备，才能列出它上报的指标', 'Pick a device to list the measures it reports') }}</p>
          </div>

          <div v-if="needMethod" class="dq-step">
            <p class="dq-q"><span class="dq-n">{{ stepNo.method }}</span>{{ activeRecipe && activeRecipe.id === 'trend' ? t('每个时间点怎么取数', 'How to sample each bucket') : t('怎么算', 'How to aggregate') }}</p>
            <select v-model="form.method">
              <option v-for="item in AGGREGATIONS" :key="item.value" :value="item.value">{{ t(item.zh, item.en) }}</option>
            </select>
            <p class="dq-hint">{{ aggregationHint }}</p>
            <template v-if="activeRecipe && activeRecipe.id === 'summary' && picked.length > 1 && ['avg', 'sum'].includes(form.method)">
              <select v-model="form.weight" class="dq-sub">
                <option value="sample">{{ t('所有采样点一起算', 'Every sample counts equally') }}</option>
                <option value="device">{{ t('先算每台设备的均值，再平均', 'Average per device first, then average those') }}</option>
              </select>
              <p class="dq-hint">{{ t('各设备上报频率不一致时，「先算每台」能避免话痨设备主导结果。', 'When devices report at different rates, averaging per device stops a chatty device from dominating.') }}</p>
            </template>
          </div>

          <div class="dq-step">
            <p class="dq-q"><span class="dq-n">{{ stepNo.window }}</span>{{ t('时间范围', 'Time range') }}</p>
            <div class="dq-chips">
              <button v-for="item in WINDOWS" :key="item.hours" type="button" :class="{ 'is-on': form.hours === item.hours }" @click="form.hours = item.hours">{{ t(item.zh, item.en) }}</button>
            </div>
          </div>

          <div v-if="activeRecipe && activeRecipe.id === 'trend'" class="dq-step">
            <p class="dq-q"><span class="dq-n">{{ stepNo.bucket }}</span>{{ t('每个点代表多久', 'Span of each point') }}</p>
            <div class="dq-chips">
              <button v-for="item in BUCKETS" :key="item.minutes" type="button" :class="{ 'is-on': form.minutes === item.minutes }" @click="form.minutes = item.minutes">{{ t(item.zh, item.en) }}</button>
            </div>
          </div>
        </template>

        <div class="dq-actions">
          <button type="button" :class="result ? 'dq-secondary' : 'dq-primary'" :disabled="!canPreview" @click="preview">{{ busy ? t('查询中…', 'Querying…') : result ? t('重新查询', 'Run again') : t('查看数据', 'Show the data') }}</button>
          <p v-if="!canPreview" class="dq-hint">{{ blockReason }}</p>
        </div>
      </template>

      <p v-if="error" role="alert" class="dq-error">{{ error }}</p>

      <template v-if="result">
      <div ref="resultPanel">
        <p class="dq-meta" role="status">{{ t('共', 'Rows') }} {{ result.rows.length }} · {{ t('扫描', 'Scanned') }} {{ result.meta.scannedRows }} · {{ result.meta.cacheHit ? t('缓存命中', 'Cache hit') : t('实时计算', 'Computed') }}</p>
        <p class="dq-hint">{{ new Date(result.meta.from).toLocaleString() }} → {{ new Date(result.meta.to).toLocaleString() }} · {{ result.meta.timezone }}</p>
        <div v-if="result.rows.length" class="dq-table" tabindex="0" :aria-label="t('查询结果，最多预览 20 行', 'Query results, first 20 rows')">
          <table>
            <thead><tr><th v-for="field in result.fields" :key="field.id">{{ outputLabel(field.id) }}</th></tr></thead>
            <tbody><tr v-for="(row, index) in result.rows.slice(0, 20)" :key="index"><td v-for="field in result.fields" :key="field.id">{{ display(field.id, row[field.id]) }}</td></tr></tbody>
          </table>
        </div>
        <p v-else class="dq-empty">{{ t('这个范围内没有数据，试试放宽时间范围或换一台设备。', 'No data in this range. Widen the time range or pick another device.') }}</p>

        <template v-if="result.rows.length">
          <div class="dq-step">
            <p class="dq-q"><span class="dq-n">✓</span>{{ t('组件用哪一列', 'Which column feeds the widget') }}</p>
            <label>{{ t('数值', 'Value') }}<select v-model="mapping.value"><option v-for="field in result.fields" :key="field.id" :value="field.id">{{ outputLabel(field.id) }}</option></select></label>
            <label v-if="!isScalar && result.fields.length > 1">{{ t('横轴 / 分类', 'Axis or category') }}<select v-model="mapping.category"><option value="">{{ t('不指定', 'None') }}</option><option v-for="field in result.fields" :key="field.id" :value="field.id">{{ outputLabel(field.id) }}</option></select></label>
            <label v-if="!isScalar && result.fields.length > 2">{{ t('系列（可选）', 'Series (optional)') }}<select v-model="mapping.series" @change="seriesTouched = true"><option value="">{{ t('不区分', 'None') }}</option><option v-for="field in result.fields" :key="field.id" :value="field.id">{{ outputLabel(field.id) }}</option></select></label>
          </div>
          <div class="dq-actions">
            <button type="button" class="dq-primary" :disabled="busy" @click="apply">{{ t('用这份数据', 'Use this data') }}</button>
          </div>
        </template>
      </div>
      </template>

      <details class="dq-advanced">
        <summary>{{ t('高级：直接编辑查询配置', 'Advanced: edit the query directly') }}</summary>
        <p class="dq-hint">{{ t('支持 filter、derive、bucket、aggregate、sort、limit、project、join、union。公式可用 row["字段"]、param["参数"]、abs、round、coalesce、ifelse；除零和缺失值保留为空。时间范围左闭右开。', 'Supports filter, derive, bucket, aggregate, sort, limit, project, join and union. Expressions accept row["field"], param["parameter"], abs, round, coalesce and ifelse; missing values and division by zero stay null. Time ranges are [from, to).') }}</p>
        <button v-if="jsonOnly" type="button" @click="backToGuide">{{ t('回到引导配置', 'Back to the guided form') }}</button>
        <label>{{ t('查询配置 JSON', 'Query configuration JSON') }}<textarea v-model="planText" rows="12" spellcheck="false" @input="enterJsonMode" /></label>
        <label>{{ t('参数值 JSON', 'Parameter values JSON') }}<textarea v-model="parametersText" rows="2" spellcheck="false" @input="detach" /></label>
        <label>{{ t('输出列（可选，逗号分隔）', 'Output columns (optional, comma separated)') }}<input v-model="columns" /></label>
        <button type="button" :disabled="busy" @click="preview">{{ busy ? t('查询中…', 'Querying…') : t('预览查询', 'Preview query') }}</button>

        <details class="dq-datasets">
          <summary>{{ t('复用项目数据集', 'Reuse a project dataset') }}</summary>
          <label>{{ t('项目数据集', 'Project dataset') }}<select v-model="chosen" @focus="loadDatasets" @change="chooseDataset"><option value="">{{ t('选择数据集…', 'Choose a dataset…') }}</option><option v-for="item in datasets" :key="item.id" :value="item.id">{{ item.name }} · v{{ item.version }}</option></select></label>
          <p v-if="pinned" class="dq-hint">{{ t('固定版本', 'Pinned version') }}: {{ pinned.id }} / v{{ pinned.version }}</p>
          <label>{{ t('数据集名称', 'Dataset name') }}<input v-model="name" maxlength="128" /></label>
          <button type="button" :disabled="busy || !name.trim()" @click="saveDataset">{{ t('另存为项目数据集', 'Save as project dataset') }}</button>
          <button v-if="pinned" type="button" :disabled="busy || !name.trim()" @click="saveDataset(true)">{{ t('保存新版本', 'Save a new version') }}</button>
        </details>
      </details>
    </fieldset>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, reactive, ref, watch } from 'vue'
import { rpc, state } from './bridge'
import {
  AGGREGATIONS, BUCKETS, WINDOWS,
  adoptPlan, applyRecipe, buildPlan, defaultForm, defaultRawForm,
  inferMapping, isScalarShape, recipeById, recipesFor, splitCodes, toggleCode
} from './datasetRecipe'

const props = defineProps<{ binding?: any; catalog: any[]; shape: string }>()
const emit = defineEmits<{ (event: 'apply', binding: any): void }>()

const english = computed(() => state.locale.startsWith('en'))
const t = (zh: string, en: string) => english.value ? en : zh

const isScalar = computed(() => isScalarShape(props.shape))
const recipes = computed(() => recipesFor(props.shape))
const activeRecipe = computed(() => recipeById(recipeId.value))
const needField = computed(() => activeRecipe.value?.field === true)
const needMethod = computed(() => !!activeRecipe.value && activeRecipe.value.method !== false)
// Questions disappear depending on the purpose, so the numbers are derived
// instead of hardcoded or an alarms recipe would read 1, 2, 5.
const stepNo = computed(() => {
  const map: Record<string, number> = { devices: 2 }
  let next = 3
  if (needField.value) map.field = next++
  if (needMethod.value) map.method = next++
  map.window = next++
  map.bucket = next
  return map
})

const form = reactive(defaultForm(props.shape))
const recipeId = ref('')
/** `guide` builds the plan from the answers; `json` uses the textarea verbatim. */
const mode = ref<'guide' | 'json'>('guide')
const deviceQuery = ref('')
const showDevices = ref(false)
const TIP_KEY = 'noyo-dataset-tip'
const showTip = ref(localStorage.getItem(TIP_KEY) !== 'off')
function dismissTip() { showTip.value = false; localStorage.setItem(TIP_KEY, 'off') }
const resultPanel = ref<HTMLElement | null>(null)
const planText = ref(''), parametersText = ref('{}'), columns = ref(''), name = ref(''), chosen = ref('')
const mapping = reactive({ value: 'value', category: '', series: '' })
/** Set once the author edits the series picker, so inference stops overriding it. */
const seriesTouched = ref(false)
const pinned = ref<any>(null), datasets = ref<any[]>([]), result = ref<any>(null), error = ref(''), busy = ref(false)
const pinnedPlan = ref('')
let sequence = 0

const jsonOnly = computed(() => mode.value === 'json')
/** Why the panel is in JSON mode: an unrepresentable stored plan, or the author typing. */
const jsonReason = ref<'adopted' | 'typed'>('typed')
const picked = computed(() => splitCodes(form.devices))

const catalogIndex = computed(() => {
  const index = new Map<string, any>()
  for (const device of props.catalog || []) if (device?.code) index.set(device.code, device)
  return index
})
function deviceName(code: string) { return catalogIndex.value.get(code)?.name || code }

const deviceMatches = computed(() => {
  const keyword = deviceQuery.value.trim().toLowerCase()
  const list = (props.catalog || []).filter(item => {
    if (!keyword) return true
    return [item.name, item.code, item.productName].some(value => String(value || '').toLowerCase().includes(keyword))
  })
  return list.slice(0, 30)
})

// Measures come from the devices actually picked, so a pump never offers a
// meter's fields. With nothing picked the whole catalogue is offered.
const availableFields = computed(() => {
  const devices = picked.value.length ? picked.value.map(code => catalogIndex.value.get(code)).filter(Boolean) : (props.catalog || [])
  const seen = new Map<string, any>()
  for (const device of devices) for (const field of device?.fields || []) if (field?.key && !seen.has(field.key)) seen.set(field.key, field)
  return [...seen.values()]
})

function fieldLabel(field: any) {
  if (!field) return ''
  const name = english.value ? (field.nameEn || field.nameZh || field.name) : (field.nameZh || field.name || field.nameEn)
  if (!name || name === field.key) return field.key
  const unit = field.unit ? ` (${field.unit})` : ''
  return `${name}${unit} · ${field.key}`
}

// Result columns carry no display name, so well-known ones get a label and the
// rest fall back to the catalogue entry for the measure that produced them.
const OUTPUT_LABELS: Record<string, [string, string]> = {
  ts: ['时间', 'Time'], value: ['数值', 'Value'], device_code: ['设备编码', 'Device code'],
  device_name: ['设备名称', 'Device name'], product_code: ['产品编码', 'Product code'],
  online: ['在线', 'Online'], severity: ['告警级别', 'Severity'], title: ['告警标题', 'Alarm title'],
  condition_status: ['触发状态', 'Condition'], handling_status: ['处理状态', 'Handling'],
  device_avg: ['设备均值', 'Device average']
}
function outputLabel(id: string) {
  const label = OUTPUT_LABELS[id]
  if (label) return t(label[0], label[1])
  const known = availableFields.value.find(field => field.key === id)
  return known ? fieldLabel(known) : id
}

const aggregationHint = computed(() => {
  const item = AGGREGATIONS.find(entry => entry.value === form.method)
  return item ? t(item.hintZh, item.hintEn) : ''
})

const blockReason = computed(() => {
  if (!recipeId.value) return t('先选一个用途', 'Pick a purpose first')
  if (needField.value && !form.field) return t('请选择一个指标', 'Pick a measure')
  return ''
})
const canPreview = computed(() => mode.value === 'json' || (!!recipeId.value && (!needField.value || !!form.field)))

function display(id: string, value: any) {
  if (value === null || value === undefined) return '—'
  if (typeof value === 'boolean') return value ? t('是', 'Yes') : t('否', 'No')
  if (id === 'ts' || /_at$|_ts$/.test(id)) {
    const stamp = Number(value)
    if (Number.isFinite(stamp) && stamp > 1e11) return new Date(stamp).toLocaleString()
  }
  return String(value)
}

function detach() { result.value = null; sequence++ }

function chooseRecipe(item: any, run = true) {
  if (!item || jsonOnly.value) return
  recipeId.value = item.id
  Object.assign(form, applyRecipe(form, item, props.shape))
  ensureField()
  deviceQuery.value = ''
  showDevices.value = false
  detach()
  // Answering the question should show the answer. Auto-running here turns the
  // flow into "pick a purpose, see the data" instead of "pick, scroll, submit".
  if (run && canPreview.value) nextTick(() => preview())
}

function ensureField() {
  if (!needField.value) { form.field = ''; return }
  if (form.field && availableFields.value.some(field => field.key === form.field)) return
  form.field = availableFields.value[0]?.key || ''
}

// The backend rejects a selector above 200 devices, so stop the selection here
// instead of letting the save fail with a budget error.
const MAX_DEVICES = 200
function toggleDevice(code: string) {
  const next = toggleCode(form.devices, code)
  if (splitCodes(next).length > MAX_DEVICES) {
    error.value = t(`最多选择 ${MAX_DEVICES} 台设备，请改用产品范围或在时间上缩小。`, `At most ${MAX_DEVICES} devices can be selected; narrow by product or time instead.`)
    return
  }
  error.value = ''
  form.devices = next
  ensureField()
  detach()
}

// The catalogue arrives after the panel mounts, so the very first render has no
// measures. Fill the measure as soon as a device reports fields rather than
// leaving an empty selector that only fails once the operator hits preview.
watch(availableFields, () => { if (mode.value === 'guide') ensureField() })

function activePlan() {
  if (mode.value === 'json') {
    if (!planText.value.trim()) throw new Error(t('请填写查询配置 JSON', 'Enter the query configuration JSON'))
    return JSON.parse(planText.value)
  }
  return buildPlan(form)
}

function enterJsonMode() {
  // Typing in the textarea takes over; the guide must not overwrite it silently.
  mode.value = 'json'
  jsonReason.value = 'typed'
  detach()
}

function backToGuide() {
  if (planText.value.trim() && !window.confirm(t('回到引导会放弃当前手写的查询配置，确定吗？', 'Going back to the guided form discards the query you typed. Continue?'))) return
  restartGuide()
}

function restartGuide() {
  mode.value = 'guide'
  Object.assign(form, defaultRawForm(props.shape))
  recipeId.value = ''
  planText.value = ''
  result.value = null
  error.value = ''
  chooseRecipe(recipes.value[0])
}

async function preview() {
  const request = ++sequence
  busy.value = true; error.value = ''; result.value = null
  try {
    const plan = activePlan()
    const parameters = JSON.parse(parametersText.value || '{}')
    if (!parameters || Array.isArray(parameters) || typeof parameters !== 'object') throw new Error(t('参数值必须是 JSON 对象', 'Parameter values must be a JSON object'))
    const value = await rpc('dataset.preview', { plan, parameters })
    if (request === sequence) {
      result.value = value
      nextTick(() => resultPanel.value?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }))
      Object.assign(mapping, inferMapping(value?.fields || [], value?.rows || [], {
        value: mapping.value,
        category: mapping.category,
        series: seriesTouched.value ? mapping.series : undefined
      }))
    }
    return request === sequence ? value : null
  } catch (err) { if (request === sequence) error.value = (err as Error).message; return null }
  finally { busy.value = false }
}

async function apply() {
  const data = await preview()
  if (!data) return
  try {
    if (isScalar.value && data.rows.length > 1) throw new Error(t('单值组件只能显示一行结果，请把用途改成「汇总成一个数」。', 'A single-value widget shows exactly one row; switch the purpose to "One summary number".'))
    if (data.rows.length && mapping.value && !data.fields.some((field: any) => field.id === mapping.value)) throw new Error(t('数值列不在查询结果中', 'The value column is missing from the result'))
    const plan = activePlan()
    const parameters = JSON.parse(parametersText.value || '{}')
    const map: any = { ...mapping }
    if (columns.value.trim()) map.columns = splitCodes(columns.value)
    emit('apply', {
      kind: 'dataset',
      ...(pinned.value && pinnedPlan.value === JSON.stringify(plan) ? { dataset: { ...pinned.value } } : { query: plan }),
      parameters,
      mapping: map
    })
  } catch (err) { error.value = (err as Error).message }
}

async function loadDatasets() { try { datasets.value = (await rpc('dataset.list')).datasets || [] } catch (err) { error.value = (err as Error).message } }
async function chooseDataset() {
  const item = datasets.value.find(entry => String(entry.id) === String(chosen.value)); if (!item) return
  busy.value = true; error.value = ''
  try {
    const value = await rpc('dataset.load', { id: item.id, version: item.version })
    pinnedPlan.value = JSON.stringify(value.plan)
    pinned.value = value.dataset
    name.value = item.name
    const recovered = adoptPlan(value.plan)
    if (recovered) { mode.value = 'guide'; recipeId.value = recovered.recipeId; Object.assign(form, recovered.form); ensureField(); planText.value = '' }
    else { mode.value = 'json'; jsonReason.value = 'adopted'; planText.value = JSON.stringify(value.plan, null, 2) }
    detach()
  } catch (err) { error.value = (err as Error).message } finally { busy.value = false }
}

async function saveDataset(update: any = false) {
  if (update !== true) update = false
  const data = await preview(); if (!data) return
  busy.value = true
  try {
    const plan = activePlan()
    const parameters = JSON.parse(parametersText.value || '{}')
    const saved = await rpc('dataset.save', { ...(update && pinned.value ? { id: pinned.value.id, baseVersion: pinned.value.version } : {}), name: name.value, plan, parameters })
    pinned.value = { id: saved.id, version: saved.version }
    pinnedPlan.value = JSON.stringify(plan)
    await loadDatasets()
  } catch (err) { error.value = (err as Error).message } finally { busy.value = false }
}

watch(() => props.binding, async binding => {
  sequence++; result.value = null; error.value = ''; deviceQuery.value = ''
  pinned.value = binding?.dataset ? { ...binding.dataset } : null
  parametersText.value = JSON.stringify(binding?.parameters || {}, null, 2)
  Object.assign(mapping, { value: 'value', category: '', series: '', ...binding?.mapping })
  seriesTouched.value = false
  columns.value = (binding?.mapping?.columns || []).join(',')

  const adopt = (plan: any) => {
    const recovered = adoptPlan(plan)
    if (!recovered) return false
    mode.value = 'guide'; recipeId.value = recovered.recipeId
    Object.assign(form, recovered.form); ensureField(); planText.value = ''
    return true
  }
  if (!binding) { restartGuide(); return }
  if (binding.query && adopt(binding.query)) return
  if (binding.query) { mode.value = 'json'; jsonReason.value = 'adopted'; planText.value = JSON.stringify(binding.query, null, 2); return }
  if (binding.dataset) {
    const request = sequence
    try {
      const value = await rpc('dataset.load', binding.dataset)
      if (request !== sequence) return
      pinnedPlan.value = JSON.stringify(value.plan)
      if (!adopt(value.plan)) { mode.value = 'json'; jsonReason.value = 'adopted'; planText.value = JSON.stringify(value.plan, null, 2) }
    } catch (err) { if (request === sequence) error.value = (err as Error).message }
    return
  }
  restartGuide()
}, { immediate: true })

// Mirror the guided answers into the JSON view so opening "Advanced" never shows
// a stale plan the author would then mistake for the live one.
watch(() => mode.value === 'guide' ? JSON.stringify(buildPlan(form)) : '', text => {
  if (!text) return
  planText.value = JSON.stringify(JSON.parse(text), null, 2)
})
</script>

<style scoped>
.dq-fieldset{display:block;border:0;margin:0;padding:0;min-width:0}
.dq-step{margin-bottom:20px}
.dq-q{display:flex;align-items:center;gap:8px;margin:0 0 8px;font-size:13px;font-weight:600;color:var(--text-primary)}
.dq-n{display:inline-flex;align-items:center;justify-content:center;width:18px;height:18px;flex-shrink:0;border-radius:50%;background:var(--primary-color);color:#fff;font-size:11px;font-weight:600}
.dq-grid{display:grid;grid-template-columns:1fr 1fr;gap:8px}
.dq .dq-card{display:flex;margin-bottom:0;flex-direction:column;gap:4px;align-items:flex-start;text-align:left;padding:10px 12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-body);color:var(--text-primary);cursor:pointer;transition:border-color var(--transition-fast),background var(--transition-fast)}
.dq .dq-card:hover{border-color:var(--primary-color)}
.dq .dq-card.is-on{border-color:var(--primary-color);background:color-mix(in srgb,var(--primary-color) 10%,var(--bg-body))}
.dq-card b{font-size:13px;font-weight:600}
.dq-card small{font-size:11px;line-height:1.4;color:var(--text-secondary)}
.dq-chips{display:flex;flex-wrap:wrap;gap:8px}
.dq .dq-chips button{width:auto;margin-bottom:0;padding:6px 12px;border:1px solid var(--border-color);border-radius:999px;background:var(--bg-body);color:var(--text-primary);font-size:12px;cursor:pointer}
.dq .dq-chips button.is-on{border-color:var(--primary-color);color:var(--primary-color);background:color-mix(in srgb,var(--primary-color) 10%,var(--bg-body))}
.dq-tags{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:8px}
.dq-tag{display:inline-flex;align-items:center;gap:6px;padding:4px 6px 4px 10px;border-radius:999px;background:color-mix(in srgb,var(--primary-color) 12%,var(--bg-body));color:var(--text-primary);font-size:12px}
.dq .dq-tag button{width:auto;margin-bottom:0;border:0;background:transparent;color:var(--text-secondary);cursor:pointer;font-size:14px;line-height:1;padding:0 2px}
.dq-list{list-style:none;margin:8px 0 0;padding:0;max-height:132px;overflow:auto;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-body)}
.dq-list button{display:flex;align-items:center;gap:8px;width:100%;padding:8px 10px;border:0;border-bottom:1px solid var(--border-color);background:transparent;color:var(--text-primary);font-size:12px;text-align:left;cursor:pointer}
.dq-list li:last-child button{border-bottom:0}
.dq-list button:hover{background:var(--bg-surface)}
.dq-list button[aria-pressed=true]{background:color-mix(in srgb,var(--primary-color) 12%,transparent);color:var(--primary-color)}
.dq-list small{margin-left:auto;color:var(--text-secondary);font-size:11px}
.dq-dot{width:7px;height:7px;border-radius:50%;flex-shrink:0;background:var(--text-secondary)}
.dq-dot.is-online{background:#22c55e}
.dq-dot.is-offline{background:#94a3b8}
.dq-sub{margin-top:8px}
.dq-tip{display:flex;gap:8px;align-items:flex-start;margin:0 0 16px;padding:10px 12px;border:1px dashed var(--border-color);border-radius:10px;font-size:12px;line-height:1.5;color:var(--text-secondary)}
.dq-tip span{flex:1 1 auto;min-width:0;overflow-wrap:anywhere}
.dq .dq-tip button{flex:0 0 auto;width:auto;margin:0;padding:0 4px;border:0;background:transparent;color:var(--text-secondary);font-size:14px;line-height:1;cursor:pointer}
.dq-secondary{width:100%;padding:10px 12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-body);color:var(--text-primary);font-size:13px;cursor:pointer}
.dq-actions{position:sticky;bottom:-1px;z-index:3;padding:8px 0 4px;background:linear-gradient(to bottom,color-mix(in srgb,var(--bg-surface) 0%,transparent),var(--bg-surface) 24%)}
.dq-primary{width:100%;padding:10px 12px;border:1px solid var(--primary-color);border-radius:10px;background:var(--primary-color);color:#fff;font-size:13px;font-weight:600;cursor:pointer}
.dq-primary:disabled{opacity:.5;cursor:default}
.dq-hint{margin:6px 0 0;font-size:12px;line-height:1.5;color:var(--text-secondary)}
.dq-error{margin:8px 0 0;font-size:12px;color:var(--danger-color);overflow-wrap:anywhere}
.dq-meta{margin:12px 0 0;font-size:12px;color:var(--text-primary)}
.dq-notice{margin-bottom:16px;padding:12px;border:1px solid var(--border-color);border-radius:10px;background:var(--bg-body)}
.dq-notice p{margin:0 0 8px;font-size:12px;line-height:1.5;color:var(--text-secondary)}
.dq .dq-notice button{margin-bottom:0;width:100%;padding:8px 12px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-surface);color:var(--text-primary);font-size:12px;cursor:pointer}
.dq-table{max-height:240px;overflow:auto;margin-top:8px;border:1px solid var(--border-color);border-radius:10px}
.dq-table table{border-collapse:collapse;font-size:12px;white-space:nowrap}
.dq-table th,.dq-table td{padding:8px;border-bottom:1px solid var(--border-color);text-align:left}
.dq-table th{position:sticky;top:0;background:var(--bg-body);font-weight:600}
.dq-empty{margin:12px 0;padding:32px 12px;text-align:center;font-size:12px;color:var(--text-secondary);border:1px dashed var(--border-color);border-radius:10px}
.dq-advanced{margin-top:16px;padding-top:12px;border-top:1px solid var(--border-color)}
.dq-advanced summary{cursor:pointer;font-size:12px;color:var(--text-secondary)}
.dq-advanced textarea{font:12px/1.5 ui-monospace,monospace;resize:vertical}
.dq-datasets{margin-top:12px}
</style>
