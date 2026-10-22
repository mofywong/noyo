<template>
  <main class="noyo-editor" :class="{ viewer: readonly }" :style="readonly ? viewerShell : null">
    <p v-if="!state.initialized" class="empty" role="status">{{ t('正在连接 Noyo…', 'Connecting to Noyo…') }}</p>
    <template v-else>
      <header v-if="!readonly" class="editor-toolbar">
        <strong>{{ t('画布编辑器', 'Canvas editor') }}</strong>
        <div class="toolbar-group" role="group" :aria-label="t('侧栏', 'Side panel')">
          <button type="button" :aria-pressed="showPalette && !showLayers" @click="togglePalette(false)">{{ t('组件', 'Components') }}</button>
          <button type="button" :aria-pressed="showPalette && showLayers" @click="togglePalette(true)">{{ t('图层', 'Layers') }}</button>
        </div>
        <div class="toolbar-group" role="group" :aria-label="t('历史', 'History')">
          <button type="button" @click="history.backAction()">{{ t('撤销', 'Undo') }}</button>
          <button type="button" @click="history.forwardAction()">{{ t('重做', 'Redo') }}</button>
        </div>
        <div class="toolbar-group" role="group" :aria-label="t('缩放', 'Zoom')">
          <button type="button" @click="canvas?.zoomOut()" :aria-label="t('缩小', 'Zoom out')">−</button>
          <button type="button" @click="canvas?.reset()">{{ t('适应', 'Fit') }}</button>
          <button type="button" @click="canvas?.zoomIn()" :aria-label="t('放大', 'Zoom in')">+</button>
        </div>
        <button type="button" :aria-pressed="showInspector && !showAssistant" @click="showAssistant = false; showInspector = !showInspector">{{ t('属性', 'Properties') }}</button>
        <button type="button" :aria-pressed="!showPalette && !showInspector && !showAssistant" @click="focusCanvas">{{ t('专注画布', 'Focus canvas') }}</button>
        <span class="save-state">{{ state.dirty ? t('有未保存更改', 'Unsaved changes') : t('已同步', 'Synced') }}</span>
        <button v-if="state.capabilities.save && !embedded" ref="assistantTrigger" class="assistant-trigger" type="button" :aria-pressed="showAssistant" @click="showAssistant = !showAssistant">{{ t('IOC AI 助手', 'IOC AI assistant') }}</button>
      </header>
      <p v-if="state.error" class="editor-error" role="alert">{{ state.error }}</p>
      <div v-if="!readonly && degradedBindings.length" class="editor-data-status" role="status">
        <details><summary>{{ t('数据状态', 'Data status') }} · {{ degradedBindings.length }}</summary><ul><li v-for="item in degradedBindings" :key="item.componentId">{{ screenComponents(store.componentList).find(component => component.id === item.componentId)?.chartConfig.title || item.componentId }} · {{ statusText(item) }}</li></ul></details>
        <button type="button" @click="refreshHistories">{{ t('重试数据', 'Retry data') }}</button>
      </div>
      <details v-if="readonly && hasDatasets" class="dataset-filters">
        <summary>{{ t('数据筛选', 'Data filters') }}</summary>
        <label>{{ t('开始时间（浏览器时区，可选）', 'From (browser timezone, optional)') }}<input v-model="queryRange.from" type="datetime-local" /></label>
        <label>{{ t('结束时间（浏览器时区，可选）', 'To (browser timezone, optional)') }}<input v-model="queryRange.to" type="datetime-local" /></label>
        <label v-for="control in queryControls" :key="`${control.id}:${control.name}`">{{ t('参数', 'Parameter') }}: {{ control.name }} · {{ control.id }}
          <select v-if="control.definition.options?.length" :value="JSON.stringify(parameterValue(control))" @change="setParameter(control, $event)"><option v-for="option in control.definition.options" :key="JSON.stringify(option)" :value="JSON.stringify(option)">{{ option }}</option></select>
          <input v-else-if="control.definition.type === 'boolean'" type="checkbox" :checked="parameterValue(control) === true" @change="setParameter(control, $event)" />
          <input v-else :type="control.definition.type === 'number' ? 'number' : 'text'" :value="parameterValue(control)" :required="control.definition.required" @change="setParameter(control, $event)" />
        </label>
        <p v-if="filterError" role="alert">{{ filterError }}</p>
        <button type="button" @click="applyQueryFilters">{{ t('应用筛选', 'Apply filters') }}</button>
      </details>
      <section v-if="readonly" class="viewer-stage" :style="viewerSize">
        <preview-render-list />
        <div v-if="qualityComponents.length" class="canvas-quality-layer viewer-quality-layer" aria-hidden="true">
          <span v-for="item in qualityComponents" :key="item.id" class="canvas-quality-badge" :class="`is-${item.quality}`" :style="{ left: `${item.attr.x + item.attr.w - 8}px`, top: `${item.attr.y + 8}px` }">{{ item.label }}</span>
        </div>
      </section>
      <ul v-if="readonly && degradedBindings.length" class="viewer-status" role="status">
        <li v-for="item in degradedBindings" :key="item.componentId">{{ screenComponents(store.componentList).find(component => component.id === item.componentId)?.chartConfig.title || item.componentId }} · {{ statusText(item) }}</li>
      </ul>
      <div v-if="!readonly" class="editor-workspace" :class="{ 'with-palette': showPalette, 'with-inspector': showInspector && !showAssistant, 'with-assistant': showAssistant, 'assistant-expanded': showAssistant && assistantExpanded }">
        <aside v-show="showPalette" class="editor-left" :aria-label="showLayers ? t('图层', 'Layers') : t('组件', 'Components')">
          <content-charts v-if="!showLayers" />
          <content-layers v-else @back="showLayers = false" />
        </aside>
        <section ref="canvasStage" class="canvas-stage" :class="{ 'is-context-picking': contextSelection }" tabindex="-1" :aria-label="t('拖拽画布', 'Drag and resize canvas')" @pointerdown.capture="startContextSelection" @pointermove="moveContextSelection" @pointerup="finishContextSelection" @pointercancel="cancelContextSelection" @mousedown.capture="suppressContextMouse" @click.capture="handleCanvasClick" @keydown.esc="stopContextSelection">
          <content-edit :key="canvasLayoutKey" ref="canvas" />
          <div v-if="emptyBadges.length" class="canvas-quality-layer" aria-hidden="true">
            <span v-for="badge in emptyBadges" :key="badge.id" class="canvas-quality-badge" :class="`is-${badge.quality}`" :style="badge.style">{{ badge.label }}</span>
          </div>
          <div v-if="contextDrag" class="canvas-context-region" :style="contextRegionStyle" aria-hidden="true" />
        </section>
        <ioc-assistant v-show="showAssistant" :active="showAssistant" :expanded="assistantExpanded" :snapshot="snapshot" :apply-document="applyAssistantDocument" :selection-ids="store.targetChart.selectId" :selection-area="selectionArea" :selection-mode="contextSelection" :catalog="catalog" :products="catalogProducts" :catalog-meta="catalogMeta" :catalog-loading="catalogLoading" :catalog-error="catalogError" @scope-change="setAssistantScope" @select-context="beginContextSelection" @clear-selection="clearContextSelection" @remove-selection="removeContextSelection" @refresh-catalog="loadCatalog" @expand="assistantExpanded = !assistantExpanded" @close="closeAssistant" />
        <aside v-show="showInspector && !showAssistant" class="inspector">
          <n-tabs v-if="!selected" class="inspector-tabs" size="small" type="segment" :value="'page'">
            <n-tab-pane name="page" :tab="t('页面配置', 'Page')"><canvas-page /></n-tab-pane>
          </n-tabs>
          <template v-else>
            <div v-if="selected.chartConfig.key === 'Image' || selected.chartConfig.key === 'ImageCarousel'" class="asset-block">
              <label>{{ t('图片资源', 'Image asset') }}<input type="file" accept="image/png,image/jpeg,image/webp" @change="uploadImage" /></label>
              <button type="button" @click="showAssets = true">{{ t('从素材库选择', 'Choose from library') }}</button>
              <span v-if="selected.option.assetId" class="hint">{{ t('已关联素材', 'Asset attached') }}</span>
            </div>
            <n-tabs v-model:value="tab" class="inspector-tabs" size="small" type="segment" display-directive="show:lazy">
              <n-tab-pane name="style" :tab="t('定制', 'Style')"><chart-setting /></n-tab-pane>
              <n-tab-pane name="data" :tab="t('数据', 'Data')">
                <template v-if="bindable">
                  <label>{{ t('数据类型', 'Data kind') }}<select v-model="bindingForm.kind" @change="bindingKindChanged"><option v-for="option in visibleKinds" :key="option.value" :value="option.value">{{ t(option.zh, option.en) }}</option></select></label>
                  <DataQueryEditor v-if="bindingForm.kind === 'dataset'" :key="selected.id" :binding="state.document?.bindings?.[boundTarget?.bindingId]" :catalog="catalog" :shape="shape" @apply="applyDataset" />
                  <template v-else>
                  <template v-if="shape === 'map'">
                    <label>{{ t('坐标系', 'Coordinate system') }}<select :value="selected.option.coordinateSystem" @change="changeCRS"><option value="wgs84">WGS84</option><option value="gcj02">GCJ-02</option></select></label>
                    <label>{{ t('离线轮廓（GeoJSON）', 'Offline outline (GeoJSON)') }}<input type="file" accept=".json,.geojson,application/json" @change="uploadOutline" /></label>
                    <p class="hint">{{ t('轮廓与设备须使用同一坐标系，不进行坐标转换。最多 1 MiB、10000 个坐标。', 'Outline and devices must share a CRS; coordinates are not converted. Maximum 1 MiB and 10000 positions.') }}</p>
                  </template>
                  <label>{{ t('设备', 'Device') }}<input v-model="bindingForm.deviceCode" :placeholder="['aggregate', 'deviceSummary', 'alarmList', 'eventList', 'location'].includes(bindingForm.kind) ? t('设备编码，逗号分隔', 'Device codes, comma separated') : t('设备编码', 'Device code')" /></label>
                  <p v-if="catalog.length" class="quick-pick">
                    <select :value="''" :aria-label="t('从目录选择设备', 'Pick a device from the catalog')" @change="pickDevice($event)"><option value="">{{ t('从目录选择设备…', 'Pick a device…') }}</option><option v-for="item in catalog" :key="item.code" :value="item.code">{{ item.online ? '●' : '○' }} {{ item.name }}（{{ item.code }}）</option></select>
                    <select v-if="bindingForm.kind !== 'deviceSummary'" :value="''" :aria-label="t('从目录选择属性', 'Pick a property from the catalog')" @change="pickKey($event)"><option value="">{{ t('选择属性…', 'Pick a property…') }}</option><option v-for="prop in catalogProperties(bindingForm.deviceCode.split(',')[0].trim())" :key="prop.key" :value="prop.key">{{ prop.name }}（{{ prop.key }}）</option></select>
                  </p>
                  <template v-if="bindingForm.kind === 'location'"><label>{{ t('纬度属性', 'Latitude property') }}<input v-model="bindingForm.latitudeKey" /></label><label>{{ t('经度属性', 'Longitude property') }}<input v-model="bindingForm.longitudeKey" /></label></template>
                  <label v-else-if="!['deviceStatus', 'deviceSummary', 'alarmList', 'eventList'].includes(bindingForm.kind)">{{ t('属性', 'Property') }}<input v-model="bindingForm.key" :placeholder="shape === 'table-basic' ? t('属性键，逗号分隔可多列', 'Property keys, comma separated for extra columns') : t('属性键', 'Property key')" /></label>
                  <label v-if="bindingForm.kind === 'deviceSummary'">{{ t('设备统计', 'Device summary') }}<select v-model="bindingForm.aggregate"><option v-for="option in summaryChoices" :key="option.value" :value="option.value">{{ t(option.zh, option.en) }}</option></select></label>
                  <label v-if="bindingForm.kind === 'aggregate'">{{ t('聚合方法', 'Aggregate') }}<select v-model="bindingForm.aggregate"><option value="avg">{{ t('平均', 'Average') }}</option><option value="min">{{ t('最小值', 'Minimum') }}</option><option value="max">{{ t('最大值', 'Maximum') }}</option><option value="sum">{{ t('求和', 'Sum') }}</option><option value="count">{{ t('计数', 'Count') }}</option><option value="median">{{ t('中位数', 'Median') }}</option></select></label>
                  <button type="button" :disabled="!canBind" @click="bind">{{ t('应用数据绑定', 'Apply binding') }}</button>
                  <p v-if="deviceError" class="hint" role="alert">{{ t('最新值、历史趋势与在线状态只能绑定单台设备，请只填一个设备编码。', 'Latest value, history and online status bind exactly one device; enter a single device code.') }}</p>
                  </template>
                  <button v-if="boundTarget" type="button" @click="unbind">{{ t('移除绑定', 'Remove binding') }}</button>
                  <p v-if="boundTarget && statuses[selected.id]" class="binding-status" :class="`quality-${statuses[selected.id].quality}`" role="status">{{ statusText(statuses[selected.id]) }}</p>
                  <small>{{ t('运行数据不写入画布文档', 'Live values are not saved in the document') }}</small>
                </template>
                <p v-else class="hint">{{ t('该组件为静态展示或媒体组件，不接收设备数据。', 'This component is decorative or media-only and does not take device data.') }}</p>
              </n-tab-pane>
              <n-tab-pane name="animation" :tab="t('动画', 'Animation')"><chart-animation /></n-tab-pane>
            </n-tabs>
            <button type="button" class="danger" @click="confirmDelete = true">{{ t('删除组件', 'Delete component') }}</button>
          </template>
        </aside>
      </div>
      <n-modal v-model:show="confirmDelete" preset="dialog" :title="t('删除组件', 'Delete component')" :content="t('确认删除选中的组件？', 'Delete the selected component?')" :positive-text="t('删除', 'Delete')" :negative-text="t('取消', 'Cancel')" @positive-click="remove" />
      <asset-library v-model:show="showAssets" :selected-id="selected?.option.assetId" @choose="chooseAsset" />
      <n-modal v-model:show="confirmCRS" preset="dialog" :title="t('更改坐标系', 'Change coordinate system')" :content="t('将清除现有轮廓和位置绑定，请使用新坐标系重新导入和绑定。', 'This clears the existing outline and location binding. Import and bind again using the new CRS.')" :positive-text="t('更改', 'Change')" :negative-text="t('取消', 'Cancel')" @positive-click="applyCRS" />
    </template>
  </main>
</template>
<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartHistoryStore } from '@/store/modules/chartHistoryStore/chartHistoryStore'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { useLangStore } from '@/store/modules/langStore/langStore'
import ContentEdit from '@/views/chart/ContentEdit/index.vue'
import ContentCharts from '@/views/chart/ContentCharts/index.vue'
import ContentLayers from '@/views/chart/ContentLayers/index.vue'
import CanvasPage from '@/views/chart/ContentConfigurations/components/CanvasPage/index.vue'
import ChartSetting from '@/views/chart/ContentConfigurations/components/ChartSetting/index.vue'
import ChartAnimation from '@/views/chart/ContentConfigurations/components/ChartAnimation/index.vue'
import PreviewRenderList from '@/views/preview/components/PreviewRenderList/index.vue'
import { sanitizeComponent } from './materials'
import { kindsOf, shapeOf, targetOf } from './widgets'
import AssetLibrary from './AssetLibrary.vue'
import IocAssistant from './Assistant.vue'
import { areaBetween, canvasPoint, componentAtPoint, componentsInArea } from './assistantContext.js'
import DataQueryEditor from './DataQueryEditor.vue'
import { importOutline } from './mapGeometry'
import { connect, documentSignature, restoreDatasetDefaults, embedded, markDirty, on, rpc, send, state } from './bridge'
import { deliverBinding, deliverWidget, qualityLabel, screenComponents, subscribeWidgetStatus } from './runtime'
const store = useChartEditStore(), history = useChartHistoryStore()
const canvas = ref<any>(), confirmDelete = ref(false)
const english = computed(() => state.locale.startsWith('en'))
const t = (zh: string, en: string) => english.value ? en : zh
const readonly = computed(() => state.mode !== 'edit')
const selected = computed<any>(() => store.componentList.find(item => item.id === store.targetChart.selectId[0]))
const boundTarget = computed(() => state.document?.targets?.find((target: any) => target.componentId === selected.value?.id))
const bindingForm = reactive({ kind: 'point', deviceCode: '', key: '', aggregate: 'avg', latitudeKey: '', longitudeKey: '' })
const showAssets = ref(false)
const showAssistant = ref(false)
type ContextArea = { x: number; y: number; w: number; h: number }
const canvasStage = ref<HTMLElement>(), contextSelection = ref('')
const selectionArea = ref<ContextArea | null>(null)
const contextDrag = ref<{ start: { x: number; y: number }; area: ContextArea; rect: DOMRect; stage: DOMRect } | null>(null)
let contextPointer = false
const contextRegionStyle = computed(() => {
  const drag = contextDrag.value
  if (!drag) return {}
  const scaleX = drag.rect.width / store.editCanvasConfig.width, scaleY = drag.rect.height / store.editCanvasConfig.height
  return { left: `${drag.rect.left - drag.stage.left + drag.area.x * scaleX}px`, top: `${drag.rect.top - drag.stage.top + drag.area.y * scaleY}px`, width: `${drag.area.w * scaleX}px`, height: `${drag.area.h * scaleY}px` }
})
function beginContextSelection(mode: string) { if (!mode) { stopContextSelection(); return }; contextSelection.value = mode; assistantExpanded.value = false }
function stopContextSelection(event?: KeyboardEvent) {
  if (!contextSelection.value) return
  event?.preventDefault(); event?.stopPropagation()
  cancelContextSelection(); contextSelection.value = ''
}
function clearContextSelection() { store.setTargetSelectChart(); selectionArea.value = null }
function removeContextSelection(id: string) { store.setTargetSelectChart(store.targetChart.selectId.filter(item => item !== id)) }
function contextSurface() { return canvasStage.value?.querySelector('.go-edit-range') as HTMLElement | null }
function startContextSelection(event: PointerEvent) {
  if (!contextSelection.value || event.button !== 0) return
  const surface = contextSurface(), stage = canvasStage.value
  if (!surface || !stage || !surface.contains(event.target as Node)) return
  event.preventDefault(); event.stopPropagation(); contextPointer = true
  stage.focus({ preventScroll: true }); stage.setPointerCapture(event.pointerId)
  const rect = surface.getBoundingClientRect(), point = canvasPoint(event.clientX, event.clientY, rect, store.editCanvasConfig)!
  if (contextSelection.value === 'components') {
    const item = componentAtPoint(store.componentList, point)
    if (item) { const ids = store.targetChart.selectId; store.setTargetSelectChart(ids.includes(item.id) ? ids.filter(id => id !== item.id) : [...ids, item.id]); selectionArea.value = null }
  } else {
    contextDrag.value = { start: point, area: areaBetween(point, point), rect, stage: stage.getBoundingClientRect() }
  }
}
function moveContextSelection(event: PointerEvent) {
  const drag = contextDrag.value
  if (!drag) return
  drag.area = areaBetween(drag.start, canvasPoint(event.clientX, event.clientY, drag.rect, store.editCanvasConfig)!)
}
function finishContextSelection(event: PointerEvent) {
  const drag = contextDrag.value
  if (drag) {
    moveContextSelection(event)
    if (drag.area.w > 0 && drag.area.h > 0) { selectionArea.value = { ...drag.area }; store.setTargetSelectChart(componentsInArea(store.componentList, drag.area)); contextSelection.value = '' }
    contextDrag.value = null
  }
  if (canvasStage.value?.hasPointerCapture(event.pointerId)) canvasStage.value.releasePointerCapture(event.pointerId)
  setTimeout(() => { contextPointer = false }, 0)
}
function cancelContextSelection() { contextDrag.value = null; contextPointer = false }
function suppressContextMouse(event: MouseEvent) { if (contextSelection.value || contextPointer) { event.preventDefault(); event.stopPropagation() } }
function handleCanvasClick(event: MouseEvent) {
  if (contextSelection.value || contextPointer) { event.preventDefault(); event.stopPropagation(); return }
  const surface = contextSurface()
  if (!surface?.contains(event.target as Node)) return
  const point = canvasPoint(event.clientX, event.clientY, surface.getBoundingClientRect(), store.editCanvasConfig)
  if (point && componentAtPoint(store.componentList, point)) { selectionArea.value = null; showAssistant.value = false; showInspector.value = true }
}
const assistantExpanded = ref(false), assistantTrigger = ref<HTMLButtonElement>()
const showPalette = ref(innerWidth >= 1440), showInspector = ref(false)
function togglePalette(layers: boolean) { showPalette.value = !showPalette.value || showLayers.value !== layers; showLayers.value = layers }
function focusCanvas() { showPalette.value = false; showInspector.value = false; closeAssistant() }
async function closeAssistant() { showAssistant.value = false; await nextTick(); if (embedded) send('assistant.focus'); else assistantTrigger.value?.focus() }
watch(showAssistant, open => { if (open) showPalette.value = false; else stopContextSelection(); send('assistant.state', { open }) })
const confirmCRS = ref(false), pendingCRS = ref('wgs84')
const catalog = ref<any[]>([]), catalogProducts = ref<any[]>([]), catalogMeta = ref({ deviceCount: 0, truncated: false })
const catalogLoading = ref(false), catalogError = ref('')
const viewport = reactive({ width: innerWidth, height: innerHeight })
const resize = () => { viewport.width = innerWidth; viewport.height = innerHeight }
// Recreate the ruler when the usable stage changes so its auto-fit uses the
// actual canvas area, instead of retaining the old tiny scale and pan offset.
const canvasLayoutKey = computed(() => [viewport.width, viewport.height, showPalette.value, showInspector.value && !showAssistant.value, showAssistant.value && !assistantExpanded.value].join(':'))
// The published canvas paints the configured background; without it a dark-themed
// screen sits on the host page colour instead of its own.
// Playback letterboxes the canvas around its centre: pinning it to the top-left
// left an ultrawide wall with the whole screen in one corner. The shell takes the
// canvas colour so the bands read as part of the screen, not as a page margin.
const viewerShell = computed(() => ({ background: store.editCanvasConfig.background || 'var(--noyo-canvas-bg)' }))
const viewerSize = computed(() => {
  const width = store.editCanvasConfig.width || 1920, height = store.editCanvasConfig.height || 1080
  const fit = Math.min(viewport.width / width, viewport.height / height)
  const scale = Number.isFinite(fit) && fit > 0 ? fit : 1
  return { width: `${width}px`, height: `${height}px`, background: store.editCanvasConfig.background || 'var(--noyo-canvas-bg)', transform: `translate(-50%, -50%) scale(${scale})`, transformOrigin: 'center center' }
})
const statuses = reactive<Record<string, any>>({})
const queryDefinitions = reactive<Record<string, any>>({}), queryParameters = reactive<Record<string, any>>({})
const queryRange = reactive({from:'',to:''}), appliedParameters = ref<Record<string,any>>({}), appliedRange = ref<any>({})
const filterError = ref('')
let queryGeneration = 0
const hasDatasets = computed(() => Object.values(state.document?.bindings || {}).some((binding:any) => binding.kind === 'dataset'))
const queryControls = computed(() => Object.entries(state.document?.bindings || {}).flatMap(([id,binding]:[string,any]) => binding.kind === 'dataset' ? Object.entries(binding.query?.parameters || queryDefinitions[id] || {}).map(([name,definition]:[string,any]) => ({id,name,definition,binding})) : []))
function parameterValue(control:any) { return queryParameters[control.id]?.[control.name] ?? control.binding.parameters?.[control.name] ?? control.definition.default ?? '' }
function setParameter(control:any,event:Event) {
  const input=event.target as HTMLInputElement
  let value:any=control.definition.options?.length ? JSON.parse(input.value) : control.definition.type==='boolean' ? input.checked : control.definition.type==='number' ? Number(input.value) : input.value
  queryParameters[control.id] ||= {};queryParameters[control.id][control.name]=value
}
function applyQueryFilters() {
  const from=queryRange.from?new Date(queryRange.from).getTime():undefined,to=queryRange.to?new Date(queryRange.to).getTime():undefined
  if ((from!=null&&!Number.isFinite(from))||(to!=null&&!Number.isFinite(to))||(from!=null&&to!=null&&from>=to)) { filterError.value=t('时间范围无效', 'Invalid time range');return }
  filterError.value='';appliedParameters.value=clone(queryParameters);appliedRange.value={from,to};queryGeneration++;refreshDatasets()
}
const degradedBindings = computed(() => Object.entries(statuses).filter(([, entry]: any) => ['stale', 'offline', 'no_data', 'access_denied', 'error'].includes(entry.quality)).map(([componentId, entry]: any) => ({ componentId, ...entry })))
const kindOptions = [{ value: 'point', zh: '最新值', en: 'Latest value' }, { value: 'history', zh: '历史趋势', en: 'History' }, { value: 'deviceStatus', zh: '在线状态', en: 'Online status' }, { value: 'aggregate', zh: '聚合统计', en: 'Aggregate' }, { value: 'alarmList', zh: '未关闭告警', en: 'Open alarms' }, { value: 'location', zh: '设备位置', en: 'Device location' }]
kindOptions.push({ value: 'eventList', zh: '物模型事件', en: 'Product events' })
kindOptions.push({ value: 'deviceSummary', zh: '设备汇总', en: 'Device summary' })
const summaryOptions = [{ value: 'total', zh: '范围内设备总数', en: 'Scoped device total' }, { value: 'online', zh: '已知在线设备', en: 'Known online devices' }, { value: 'offline', zh: '已知离线设备', en: 'Known offline devices' }, { value: 'unknown', zh: '状态未知设备', en: 'Unknown device states' }, { value: 'onlineRate', zh: '在线率（含未知设备分母）', en: 'Online rate (all scoped devices)' }]
const kindLabels = new Map(kindOptions.map(item => [item.value, item]))
function bindingKindChanged() {
  if (bindingForm.kind === 'deviceSummary' && !summaryChoices.value.some(option => option.value === bindingForm.aggregate)) bindingForm.aggregate = summaryChoices.value[0]?.value || 'total'
  if (bindingForm.kind === 'aggregate' && summaryOptions.some(option => option.value === bindingForm.aggregate)) bindingForm.aggregate = 'avg'
}
// The palette is upstream's: it reads the same catalogue through
// `packages/index.ts`, so no second list of widgets is kept here.
const showLayers = ref(false)
const tab = ref<'style' | 'data' | 'animation'>('style')
const shape = computed(() => shapeOf(selected.value?.chartConfig.key))
const summaryChoices = computed(() => summaryOptions.filter(option => !['percent', 'progress'].includes(shape.value) || option.value === 'onlineRate'))
const bindable = computed(() => !!targetOf(selected.value?.chartConfig.key))
// Only the kinds a family can actually render are offered; anything else would
// store a binding that never shows up on screen.
const visibleKinds = computed(() => [...kindsOf(selected.value?.chartConfig.key).map(value => kindLabels.get(value)!), ...(bindable.value && shape.value !== 'map' ? [{value:'dataset',zh:'数据集 / 自定义统计',en:'Dataset / custom query'}] : [])])
// The host only accepts a single device for point / history / deviceStatus
// bindings (backend validateBinding); reject it here instead of failing the save.
const singleDeviceKinds = ['point', 'history', 'deviceStatus']
const deviceCount = computed(() => bindingForm.deviceCode.split(',').map((code: string) => code.trim()).filter(Boolean).length)
const deviceError = computed(() => singleDeviceKinds.includes(bindingForm.kind) && deviceCount.value > 1)
const canBind = computed(() => {
  if (!selected.value || !bindable.value || !bindingForm.deviceCode) return false
  if (deviceError.value) return false
  if (bindingForm.kind === 'location') return !!(bindingForm.latitudeKey && bindingForm.longitudeKey)
  return bindingForm.kind === 'deviceSummary' ? summaryChoices.value.some(option => option.value === bindingForm.aggregate) : !!bindingForm.key || ['deviceStatus', 'alarmList', 'eventList'].includes(bindingForm.kind)
})
let loading = true, initialContent = '', stopConnect: (() => void) | undefined, stopStatus: (() => void) | undefined
let autosaveTimer: ReturnType<typeof setTimeout> | undefined
function scheduleAutosave() {
  clearTimeout(autosaveTimer)
  if (state.dirty && state.capabilities.save && !readonly.value) autosaveTimer = setTimeout(save, 2000)
}
const blobUrls = new Set<string>()
const clone = (value: any) => JSON.parse(JSON.stringify(value))
function statusText(entry: any) {
  if (!entry) return ''
  const [label] = qualityLabel(entry.quality, entry.bindingId)
  const time = entry.receivedAt ? new Date(entry.receivedAt).toLocaleTimeString() : ''
  const parts=String(entry.message || '').split(' / '), message=parts.length>1 ? parts[english.value?0:1] : parts[0]
  return [label,time,message].filter(Boolean).join(' · ')
}
function watchStatuses() {
  stopStatus?.()
  const ids = Array.from(new Set((state.document?.targets || []).map((target: any) => target.componentId)))
  const disposers = ids.map(id => subscribeWidgetStatus(id, (status: any) => { statuses[id] = { ...status, at: Date.now() } }))
  stopStatus = () => disposers.forEach(dispose => dispose())
}
// A degraded binding used to be reported only in the toolbar strip, which is easy
// to miss while looking at the canvas. The badge sits on the widget it belongs to,
// so "this chart is empty" is visible where the emptiness is.
const QUALITY_BADGES = new Set(['no_data', 'offline', 'stale', 'error', 'access_denied'])
const canvasBox = ref<{ left: number; top: number; scale: number } | null>(null)
function measureCanvasBox() {
  const stage = canvasStage.value
  const surface = contextSurface()
  if (!stage || !surface) { canvasBox.value = null; return }
  const stageRect = stage.getBoundingClientRect()
  const rect = surface.getBoundingClientRect()
  const width = Number(store.editCanvasConfig.width) || 1920
  canvasBox.value = rect.width && width ? { left: rect.left - stageRect.left, top: rect.top - stageRect.top, scale: rect.width / width } : null
}
const qualityComponents = computed(() => {
  const index = english.value ? 1 : 0
  return screenComponents(store.componentList)
    .filter((component: any) => component?.attr && component?.id && statuses[component.id] && QUALITY_BADGES.has(statuses[component.id].quality))
    .slice(0, 40)
    .map((component: any) => ({
      id: component.id,
      quality: statuses[component.id].quality,
      label: qualityLabel(statuses[component.id].quality, statuses[component.id].bindingId)[index],
      attr: component.attr
    }))
})
const emptyBadges = computed(() => {
  const box = canvasBox.value
  if (!box) return []
  return qualityComponents.value.map(component => ({ ...component,
    style: { left: `${Math.round(box.left + (component.attr.x + component.attr.w) * box.scale - 4)}px`, top: `${Math.round(box.top + component.attr.y * box.scale + 4)}px` }
  }))
})
let canvasObserver: ResizeObserver | null = null
let observedSurface: HTMLElement | null = null
function observeCanvasSurface() {
  const surface = contextSurface()
  if (!surface || typeof ResizeObserver === 'undefined' || observedSurface === surface) return
  canvasObserver?.disconnect()
  observedSurface = surface
  canvasObserver = new ResizeObserver(() => measureCanvasBox())
  canvasObserver.observe(surface)
}
// The canvas surface is created by the upstream editor after the document loads
// and after its scale is applied, so a single mount-time measurement lands before
// it exists and leaves the badges hidden forever.
function scheduleMeasure() {
  measureCanvasBox()
  nextTick(() => { measureCanvasBox(); observeCanvasSurface() })
  window.setTimeout(() => { measureCanvasBox(); observeCanvasSurface() }, 150)
  window.setTimeout(() => { measureCanvasBox(); observeCanvasSurface() }, 700)
}
onMounted(() => {
  scheduleMeasure()
  if (typeof ResizeObserver !== 'undefined' && canvasStage.value) {
    canvasObserver = new ResizeObserver(() => measureCanvasBox())
    canvasObserver.observe(canvasStage.value)
  }
})
onUnmounted(() => { canvasObserver?.disconnect(); canvasObserver = null; observedSurface = null })
watch(() => [state.initialized, store.editCanvasConfig.width, store.editCanvasConfig.height, store.componentList.length], scheduleMeasure)
function catalogDevice(code: string) { return catalog.value.find((item: any) => item.code === code) }
function catalogProperties(code: string) { return (catalogDevice(code)?.fields || []).map((field: any) => ({ ...field, name: english.value ? field.nameEn || field.name : field.nameZh || field.name })) }
// The text inputs stay the source of truth; the catalog only fills them so a
// device code is never typed blind.
function pickDevice(event: Event) { const value = (event.target as HTMLSelectElement).value; if (value) bindingForm.deviceCode = value }
function pickKey(event: Event) { const value = (event.target as HTMLSelectElement).value; if (value) bindingForm.key = value }
async function loadCatalog() {
  if (catalogLoading.value) return
  catalogLoading.value = true; catalogError.value = ''
  try { const result = await rpc('binding.catalog'); catalog.value = result?.devices || []; catalogProducts.value = result?.products || []; catalogMeta.value = { deviceCount: result?.deviceCount ?? catalog.value.length, truncated: result?.truncated === true } }
  catch (error) { catalogError.value = (error as Error).message }
  finally { catalogLoading.value = false }
}
function appearance(payload: any) {
  if (payload.locale) { state.locale = payload.locale; useLangStore().$patch({ lang: payload.locale.startsWith('en') ? 'EN' : 'ZH' } as any) }
  if (payload.theme) { state.theme = payload.theme; useDesignStore().$patch({ darkTheme: payload.theme === 'dark', themeName: payload.theme } as any); document.documentElement.dataset.theme = payload.theme }
  for (const [key, value] of Object.entries(payload.tokens || {})) if (/^--[a-z0-9-]+$/.test(key) && typeof value === 'string') document.documentElement.style.setProperty(key, value)
}
async function load(documentValue: any, revision: number) {
  loading = true
  queryGeneration++;appliedParameters.value={};appliedRange.value={};queryRange.from='';queryRange.to=''
  for (const key of Object.keys(queryDefinitions)) delete queryDefinitions[key]
  for (const key of Object.keys(queryParameters)) delete queryParameters[key]
  state.document = clone(documentValue); state.document.bindings ||= {}; state.document.targets ||= []
  if (!readonly.value && state.capabilities.editorBuildId) state.document.engine.buildId = state.capabilities.editorBuildId
  state.revision = revision
  const content = documentValue.content || {}
  store.editCanvasConfig = { ...store.editCanvasConfig, ...clone(content.editCanvasConfig || {}) }
  store.componentList = clone(content.componentList || []).map(sanitizeComponent)
  resetNativeHistory()
  store.requestGlobalConfig = { ...store.requestGlobalConfig, requestOriginUrl: '', requestDataPond: [], requestParams: { Header: {}, Params: {}, Body: {} } } as any
  store.projectInfo.projectId = String(documentValue.id || '')
  await nextTick(); initialContent = documentSignature(snapshot()); loading = false; markDirty(false)
  watchStatuses()
  for (const item of screenComponents(store.componentList)) if (item.option?.assetId) resolveAsset(item)
}
function snapshot() {
  const content = clone(store.getStorageInfo())
  if (state.document?.content?.noyoAssistantContext) content.noyoAssistantContext = clone(state.document.content.noyoAssistantContext)
  content.componentList = content.componentList.map(sanitizeComponent)
  return { ...clone(state.document), content:restoreDatasetDefaults(state.document,content) }
}
function setAssistantScope(scope: any) {
  if (readonly.value || !state.capabilities.save) return
  state.document.content.noyoAssistantContext = clone(scope)
  markDirty(documentSignature(snapshot()) !== initialContent)
  scheduleAutosave()
}
function resetNativeHistory() {
  // The embedded route skips GoView's normal project bootstrap. Seed a real
  // canvas baseline; clearBackStack() on an empty stack retains [undefined].
  history.$patch({ backStack: [], forwardStack: [] })
  history.canvasInit(clone(store.editCanvasConfig))
}
async function applyAssistantDocument(documentValue: any) {
  if (readonly.value || !state.capabilities.save || state.saving) throw new Error(t('当前无法编辑大屏。', 'The screen cannot be edited right now.'))
  // Prepare every component before mutating live state so a failed factory or
  // sanitizer cannot leave a partially applied proposal behind.
  const document = clone(documentValue)
  const components = document.content.componentList.map(sanitizeComponent)
  clearTimeout(autosaveTimer)
  loading = true
  try {
    state.document = document
    store.editCanvasConfig = { ...store.editCanvasConfig, ...document.content.editCanvasConfig }
    store.componentList = components
    store.targetChart.selectId = store.targetChart.selectId.filter(id => components.some((item: any) => item.id === id))
    // Native history tracks component-only deltas. AI changes include bindings
    // and use the assistant's complete-document undo instead.
    resetNativeHistory()
    await nextTick()
  } finally { loading = false }
  markDirty(documentSignature(snapshot()) !== initialContent)
  watchStatuses()
  for (const item of screenComponents(components)) if (item.option?.assetId) resolveAsset(item)
  scheduleAutosave()
}
async function save() {
  if (readonly.value || state.saving) return
  clearTimeout(autosaveTimer)
  state.error = ''; state.saving = true
  const wiring = wiringSignature()
  try {
    const submitted = snapshot()
    const result = await rpc('screen.save', { document: submitted, baseDraftVersion: state.revision })
    const current = snapshot()
    state.revision = result.draftVersion
    state.runtimeRevision = `draft-${result.draftVersion}`
    const accepted = result.document || submitted
    // The API canonicalizes optional binding fields. Retain only edits made
    // while the save was in flight; otherwise empty optional fields would
    // differ forever and trigger another autosave every two seconds.
    state.document = { ...accepted,
      bindings: documentSignature(current.bindings) === documentSignature(submitted.bindings) ? accepted.bindings : current.bindings,
      targets: documentSignature(current.targets) === documentSignature(submitted.targets) ? accepted.targets : current.targets }
    initialContent = documentSignature(result.document || submitted)
    markDirty(documentSignature(snapshot()) !== initialContent)
    refreshRuntime(wiring !== wiringSignature())
  }
  catch (error) { state.error = (error as Error).message }
  finally { state.saving = false; if (state.dirty && !state.error) scheduleAutosave() }
}
function remove() { if (!selected.value) return; const id = selected.value.id; unbind(); store.removeComponentList(id); confirmDelete.value = false }
async function bind() {
  if (!selected.value || deviceError.value) return
  const target = targetOf(selected.value.chartConfig.key)
  if (!target) return
  const id = boundTarget.value?.bindingId || `binding-${selected.value.id}`
  const devices = bindingForm.deviceCode.split(',').map((code: string) => code.trim()).filter(Boolean)
  const keys = bindingForm.key.split(',').map((key: string) => key.trim()).filter(Boolean)
  const binding: any = { ...bindingForm, ...(bindingForm.kind === 'history' && keys.length ? { keys } : {}) }
  if (['aggregate', 'deviceSummary', 'alarmList', 'eventList', 'location'].includes(bindingForm.kind)) { binding.deviceCodes = devices; delete binding.deviceCode }
  if (!['aggregate', 'deviceSummary'].includes(bindingForm.kind)) delete binding.aggregate
  if (['deviceStatus', 'deviceSummary', 'alarmList', 'eventList', 'eventList', 'location'].includes(bindingForm.kind)) delete binding.key
  if (bindingForm.kind === 'deviceSummary' && binding.aggregate === 'onlineRate') binding.unit = '%'
  if (bindingForm.kind === 'location') binding.coordinateSystem = selected.value.option.coordinateSystem
  else { delete binding.latitudeKey; delete binding.longitudeKey }
  if (binding.kind === 'eventList' && Array.isArray(selected.value.option.header)) {
    selected.value.option.header = [t('时间', 'Time'), t('设备', 'Device'), t('事件', 'Event'), t('类型', 'Type'), t('参数', 'Parameters')]
    selected.value.option.align = ['left', 'left', 'left', 'center', 'left']
  }
  state.document.bindings[id] = binding
  state.document.targets = state.document.targets.filter((target: any) => target.componentId !== selected.value.id)
  state.document.targets.push({ componentId: selected.value.id, bindingId: id, target })
  markDirty(); watchStatuses(); await save()
}
async function applyDataset(binding: any) {
  if (!selected.value || readonly.value) return
  const target = targetOf(selected.value.chartConfig.key)
  if (!target) return
  const id = boundTarget.value?.bindingId || `binding-${selected.value.id}`
  state.document.bindings[id] = binding
  state.document.targets = state.document.targets.filter((target: any) => target.componentId !== selected.value.id)
  state.document.targets.push({componentId:selected.value.id,bindingId:id,target})
  markDirty(); watchStatuses(); await save()
}
// The stream is pinned to a revision, so every save needs a new subscription.
// The series queries behind it (history windows, datasets, alarms) only depend on
// the wiring, and re-running them on each 2s autosave was the bulk of the traffic.
function wiringSignature() {
  return documentSignature({ bindings: state.document?.bindings || {}, targets: state.document?.targets || [] })
}
async function refreshRuntime(reloadSeries = true) {
  const revision = state.runtimeRevision
  try { await rpc('runtime.subscribe') } catch (error) { if (revision === state.runtimeRevision) state.error = (error as Error).message; return }
  if (reloadSeries) await refreshHistories()
  watchStatuses()
}
async function refreshHistories() {
  if (!state.initialized) return
  await refreshDatasets()
  const revision = state.runtimeRevision
  const bindings = Object.entries(state.document?.bindings || {}).filter(([, binding]: any) => ['history', 'alarmList', 'eventList'].includes(binding.kind))
  for (const [bindingId, binding] of bindings as [string, any][]) {
    try {
      const result = await rpc(binding.kind === 'alarmList' ? 'runtime.alarms' : 'runtime.history', { bindingId, maxPoints: 120 })
      if (state.initialized && revision === state.runtimeRevision) deliverBinding(bindingId, result.value, result.quality)
    } catch (error) {
      if (state.initialized && revision === state.runtimeRevision) deliverBinding(bindingId, null, 'error', { message: (error as Error).message })
    }
  }
}
let queryBusy = false, queryQueued = false
async function refreshDatasets() {
  if (queryBusy) { queryQueued = true; return }
  queryBusy = true
  const revision = state.runtimeRevision
  const ids = Object.entries(state.document?.bindings || {}).filter(([,binding]: any) => binding.kind === 'dataset').map(([id]) => id)
  const generation = queryGeneration
  try {
    for (let at = 0; at < ids.length; at += 24) {
      const batchIds=ids.slice(at,at+24), parameters=Object.fromEntries(batchIds.filter(id=>appliedParameters.value[id]).map(id=>[id,appliedParameters.value[id]]))
      const result = await rpc('runtime.data', {bindingIds:batchIds,parameters,...appliedRange.value})
      if (!state.initialized || revision !== state.runtimeRevision || generation !== queryGeneration) return
      for (const [id,sample] of Object.entries(result.bindings || {}) as [string,any][]) {queryDefinitions[id]=sample.parameters || {};deliverBinding(id,sample.value,sample.quality,{receivedAt:sample.receivedAt,meta:sample.meta,message:sample.message})}
    }
  } catch(error) {
    if (revision === state.runtimeRevision && generation === queryGeneration) for (const id of ids) deliverBinding(id,null,'error',{message:(error as Error).message})
  } finally {
    queryBusy = false
    if (queryQueued) { queryQueued = false; refreshDatasets() }
  }
}
function unbind() { if (!selected.value) return; const target = boundTarget.value; if (target) { state.document.targets = state.document.targets.filter((item: any) => item !== target); if (!state.document.targets.some((item: any) => item.bindingId === target.bindingId)) delete state.document.bindings[target.bindingId]; deliverWidget(selected.value.id, selected.value.option.dataset); delete statuses[selected.value.id]; watchStatuses(); markDirty() } }
async function resolveAsset(item: any) { try { const result = await rpc('asset.resolve', { assetId: item.option.assetId }); if (result.url && /^blob:/.test(result.url)) { blobUrls.add(result.url); deliverWidget(item.id, result.url) } } catch(error) { state.error = (error as Error).message } }
async function uploadImage(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0], item = selected.value
  if (!file || !item) return
  try { const result = await rpc('asset.upload', { file, name: file.name }); item.option.assetId = result.assetId || result.asset?.id; await resolveAsset(item); markDirty() }
  catch(error) { state.error = (error as Error).message }
}
async function chooseAsset(assetId: string) { const item = selected.value; if (!item || item.chartConfig.key !== 'Image') return; item.option.assetId = assetId; await resolveAsset(item); markDirty() }
function changeCRS(event: Event) { pendingCRS.value = (event.target as HTMLSelectElement).value; if (boundTarget.value || selected.value?.option.geoLines?.length) confirmCRS.value = true; else applyCRS() }
function applyCRS() { if (!selected.value || selected.value.chartConfig.key !== 'LocationMap') return; selected.value.option.coordinateSystem = pendingCRS.value; selected.value.option.geoLines = []; unbind(); markDirty(); confirmCRS.value = false }
async function uploadOutline(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0], item = selected.value
  if (!file || !item) return
  try { if (file.size > 1024 * 1024) throw new Error('OUTLINE_LIMIT'); const lines = importOutline(JSON.parse(await file.text())); item.option.geoLines = lines; markDirty() }
  catch { state.error = t('轮廓无效或超过限制。请使用不带 CRS 元数据的 Polygon、LineString 或 FeatureCollection。', 'Invalid or oversized outline. Use Polygon, LineString or FeatureCollection without CRS metadata.') }
  finally { (event.target as HTMLInputElement).value = '' }
}
watch(selected, () => {
  showAssets.value = false
  tab.value = 'style'
  const binding = state.document?.bindings?.[boundTarget.value?.bindingId]
  const kinds = kindsOf(selected.value?.chartConfig.key)
  const kind = binding?.kind === 'dataset' ? 'dataset' : binding?.kind && kinds.includes(binding.kind) ? binding.kind : kinds[0] || 'point'
  Object.assign(bindingForm, { kind, deviceCode: (binding?.deviceCodes || [binding?.deviceCode]).filter(Boolean).join(',') || '', key: binding?.key || binding?.keys?.join(',') || '', aggregate: binding?.aggregate || 'avg', latitudeKey: binding?.latitudeKey || '', longitudeKey: binding?.longitudeKey || '' })
})
watch(() => [store.editCanvasConfig, store.componentList, state.document?.bindings, state.document?.targets], scheduleDirtyCheck, { deep: true })
const disposers = [
  on('host.init', async payload => { try { appearance(payload); state.mode = payload.mode || 'edit'; state.capabilities = { ...payload.capabilities, editorBuildId: payload.editorBuildId }; showAssistant.value = state.mode === 'edit' && state.capabilities.save && payload.assistantOpen === true; state.runtimeRevision = payload.runtimeRevision || (state.mode === 'edit' ? `draft-${payload.revision || 0}` : ''); await load(payload.document, payload.revision || 0); state.initialized = true; send('child.initialized'); if (state.mode === 'edit') loadCatalog(); refreshRuntime() } catch(error) { state.error = (error as Error).message } }),
  on('host.theme', appearance), on('host.locale', appearance), on('host.save', save),
  on('host.assistant', () => { if (!readonly.value && state.capabilities.save) showAssistant.value = !showAssistant.value }),
  on('host.dispose', () => { clearTimeout(autosaveTimer); for (const url of blobUrls) URL.revokeObjectURL(url); blobUrls.clear() })
]
let historyTimer: ReturnType<typeof setInterval> | undefined
let dirtyTimer: ReturnType<typeof setTimeout> | undefined
// Dirty detection serializes the whole document. Doing that on every mutation
// (a drag emits one per frame) is the single most expensive thing the editor
// does, so it runs on a short trailing edge instead.
function scheduleDirtyCheck() {
  clearTimeout(dirtyTimer)
  dirtyTimer = setTimeout(() => {
    dirtyTimer = undefined
    if (loading || !state.initialized || readonly.value || state.saving) return
    markDirty(documentSignature(snapshot()) !== initialContent)
    scheduleAutosave()
  }, 200)
}
// Dataset queries and history windows are re-read on a timer. A hidden tab has
// nobody to show them to, so it must not keep the backend busy.
function refreshHistoriesIfVisible() { if (document.visibilityState === 'visible') refreshHistories() }
function onVisibilityChange() { if (document.visibilityState === 'visible') refreshHistories() }
// Ctrl/Cmd+S is what an operator actually presses; without it the browser save
// dialog appears and the autosave looks like it did nothing.
function onKeydown(event: KeyboardEvent) {
  if (!(event.ctrlKey || event.metaKey) || event.altKey || event.key.toLowerCase() !== 's') return
  event.preventDefault()
  if (!readonly.value) save()
}
onMounted(() => { historyTimer = setInterval(refreshHistoriesIfVisible, 30000); document.addEventListener('visibilitychange', onVisibilityChange); window.addEventListener('keydown', onKeydown); window.addEventListener('resize', resize); if (!embedded) state.error = t('请从 Noyo 大屏页面打开编辑器。', 'Open this editor from the Noyo screens page.'); stopConnect = connect() })
onUnmounted(() => { clearTimeout(autosaveTimer); clearTimeout(dirtyTimer); clearInterval(historyTimer); document.removeEventListener('visibilitychange', onVisibilityChange); window.removeEventListener('keydown', onKeydown); window.removeEventListener('resize', resize); stopConnect?.(); stopStatus?.(); disposers.forEach(dispose => dispose()); for (const url of blobUrls) URL.revokeObjectURL(url) })
</script>
