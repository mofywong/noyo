<template>
  <figure class="assistant-preview">
    <figcaption>{{ t('方案预览', 'Proposal preview') }}<small>{{ t('使用相同样本对比当前画布与方案 · 未验证数据会单独标注', 'Compares the canvas and proposal with the same samples · unchecked data is marked separately') }}</small></figcaption>
    <div ref="viewport" class="assistant-preview-viewport" :style="{ height: `${height * scale}px` }">
      <div ref="canvas" class="assistant-preview-canvas" :style="canvasStyle(document)">
        <div v-for="node in nodes" :key="node.id" class="assistant-preview-node" :class="{ 'is-highlighted': highlighted === node.previewSourceId }" :data-component-id="node.previewSourceId" :style="nodeStyle(node)">
          <span v-if="node.previewScalar" class="assistant-preview-value">—</span>
          <component v-else :is="node.chartConfig.chartKey" :chart-config="node" :theme-setting="document.content?.editCanvasConfig?.chartThemeSetting" :style="widgetStyle(node)" />
          <small v-if="node.previewBound && node.previewQuality !== 'good'" class="assistant-preview-data-state" data-quality-ignore>{{ sampleLabel(node.previewQuality) }}</small>
        </div>
      </div>
    </div>
    <div v-if="baseline" ref="baselineCanvas" class="assistant-preview-baseline" aria-hidden="true" :inert="true" :style="canvasStyle(baseline)">
      <div v-for="node in baselineNodes" :key="node.id" class="assistant-preview-node" :data-component-id="node.previewSourceId" :style="nodeStyle(node)">
        <span v-if="node.previewScalar" class="assistant-preview-value">—</span>
        <component v-else :is="node.chartConfig.chartKey" :chart-config="node" :theme-setting="baseline.content?.editCanvasConfig?.chartThemeSetting" :style="widgetStyle(node)" />
      </div>
    </div>
  </figure>
</template>
<script setup lang="ts">
import { computed, nextTick, onErrorCaptured, onMounted, onUnmounted, ref, watch } from 'vue'
import { sanitizeComponent } from './materials'
import { shapeOf } from './widgets'
import { shapeData } from './shapes'
import { rpc } from './bridge'
import { getInstanceByDom } from 'echarts/core'
import { inspectRenderedComponents, waitForRenderStability, type ChartRenderProbe, type RenderQualityReport } from './renderQuality'
import { compareValidation, previewRequestFailure, type ProposalIssue, type ProposalValidation, type ValidationIdentity } from './proposalSession'
const props = defineProps<{ document: any; baseline?: any; locale: string; identity?: ValidationIdentity; highlighted?: string; recheck?: number }>()
const emit = defineEmits<{ (event: 'report', result: ProposalValidation): void; (event: 'invalidated', identity: ValidationIdentity): void }>()
const t = (zh: string, en: string) => props.locale.startsWith('en') ? en : zh
const identity = () => props.identity || { candidateRevision: 0, candidateDigest: '', environmentDigest: props.locale }
const viewport = ref<HTMLElement>(), canvas = ref<HTMLElement>(), baselineCanvas = ref<HTMLElement>(), available = ref(320)
const size = (doc: any, axis: 'width' | 'height') => Number(doc.canvas?.[axis] ?? doc.content?.editCanvasConfig?.[axis]) || (axis === 'width' ? 1920 : 1080)
const width = computed(() => size(props.document, 'width')), height = computed(() => size(props.document, 'height')), scale = computed(() => Math.min(1, available.value / width.value))
const canvasStyle = (doc: any) => ({ width: `${size(doc, 'width')}px`, height: `${size(doc, 'height')}px`, transform: `scale(${scale.value})`, background: doc.content?.editCanvasConfig?.background || 'var(--bg-body)' })
let sequence = 0, measurement = 0, disposed = false, loading = false, observer: ResizeObserver | undefined
const renderErrors = new Map<string, string>()
const requestErrors = new Map<any, ProposalIssue>()
type Sample = { value?: any; quality: string; timezone?: string }
const samples = ref<Record<string, Sample>>({}), frozen = new Map<string, Sample>()
const keyOf = (binding: any) => JSON.stringify(binding, (_key, item) => item && !Array.isArray(item) && typeof item === 'object' ? Object.keys(item).sort().reduce((all: any, key) => { all[key] = item[key]; return all }, {}) : item)
function buildNodes(doc: any, prefix: string) {
  if (!doc) return []
  const targets = new Map((doc.targets || []).map((target: any) => [target.componentId, doc.bindings?.[target.bindingId]]))
  const flatten = (list: any[], x = 0, y = 0): any[] => list.flatMap(item => item.isGroup ? flatten(item.groupList || [], x + item.attr.x, y + item.attr.y) : [{ ...item, attr: { ...item.attr, x: x + item.attr.x, y: y + item.attr.y } }])
  return flatten(doc.content?.componentList || []).map(item => {
    const node = sanitizeComponent(JSON.parse(JSON.stringify(item))), binding: any = targets.get(item.id), shape = shapeOf(item.chartConfig.key)
    node.id = `${prefix}-${item.id}`; node.previewSourceId = item.id; node.previewBound = !!binding
    const sample = binding ? samples.value[keyOf(binding)] : undefined
    node.previewQuality = sample?.quality || 'unchecked'
    node.previewScalar = !!binding && node.previewQuality !== 'good' && ['number', 'percent', 'progress', 'text'].includes(shape)
    if (binding) {
      if (binding.decimals != null) node.option.precision = binding.decimals
      if (binding.unit != null) node.option.suffixText = binding.unit ? ` ${binding.unit}` : ''
      try {
        node.option.dataset = shapeData(sample?.value ?? null, { ...binding, resultTimezone: sample?.timezone }, shape, props.locale)
        if (node.previewQuality === 'good' && ['number', 'percent', 'progress', 'text'].includes(shape) && Array.isArray(sample?.value) && sample!.value[0]?.[binding.mapping?.value || 'value'] == null) { node.previewQuality = 'no_data'; node.previewScalar = true }
      } catch (error) { node.previewError = (error as Error).message; node.previewScalar = true; node.option.dataset = null }
    }
    return node
  })
}
const nodes = computed(() => buildNodes(props.document, 'proposal-preview')), baselineNodes = computed(() => buildNodes(props.baseline, 'baseline-preview'))
const nodeStyle = (node: any) => ({ left: `${node.attr.x}px`, top: `${node.attr.y}px`, width: `${node.attr.w}px`, height: `${node.attr.h}px` })
const widgetStyle = (node: any) => ({ width: `${node.attr.w}px`, height: `${node.attr.h}px` })
const sampleLabel = (quality: string) => ['error', 'query_error', 'access_denied'].includes(quality) ? t('查询失败或无权访问', 'Query failed or access denied') : quality === 'stale' ? t('数据已过期', 'Stale data') : ['no_data', 'offline', 'waiting'].includes(quality) ? t('已绑定 · 等待数据', 'Bound · awaiting data') : t('数据未验证', 'Data not checked')
function chartProbe(element: HTMLElement): ChartRenderProbe | undefined {
  const target = element.querySelector<HTMLElement>('[_echarts_instance_]'), chart = target && getInstanceByDom(target)
  if (!chart || chart.isDisposed()) return
  const rendered = chart.getZr().storage.getDisplayList()
  return { width: chart.getWidth(), height: chart.getHeight(), seriesCount: (chart.getOption().series as any[] || []).length, renderedElementCount: rendered.length, textBounds: rendered.filter((item: any) => item.type === 'tspan' && !item.ignore && !item.invisible).map((item: any) => { const rect = item.getBoundingRect().clone(); rect.applyTransform(item.getComputedTransform()); return { x: rect.x, y: rect.y, width: rect.width, height: rect.height, text: String(item.style?.text || '').slice(0, 160) } }) }
}
function inspect(list: any[], root: HTMLElement | undefined, doc: any): RenderQualityReport {
  const report = inspectRenderedComponents(list.map(node => {
    const element = [...root?.querySelectorAll<HTMLElement>('[data-component-id]') || []].find(element => element.dataset.componentId === node.previewSourceId) || null
    const shape = shapeOf(node.chartConfig.key), bindingId = (doc.targets || []).find((target: any) => target.componentId === node.previewSourceId)?.bindingId
    return { componentId: node.previewSourceId, element, bound: node.previewBound, quality: node.previewQuality, value: bindingId ? samples.value[keyOf(doc.bindings[bindingId])]?.value : undefined, renderError: node.previewError || renderErrors.get(node.id) || renderErrors.get('preview'), chartExpected: node.chartConfig.key === 'PieCircle' || ['chart', 'pie', 'radar', 'heatmap', 'treemap', 'words'].includes(shape), chart: element ? chartProbe(element) : undefined }
  }))
  for (const node of list) { const { x, y, w, h } = node.attr; if (x < 0 || y < 0 || x + w > size(doc, 'width') || y + h > size(doc, 'height')) report.issues.push({ code: 'bounds', severity: 'error', componentId: node.previewSourceId, expected: { width: size(doc, 'width'), height: size(doc, 'height') }, actual: { x, y, width: w, height: h } }) }
  const failure = requestErrors.get(doc)
  if (failure) report.issues.push(failure)
  return report
}
async function validate(version = sequence) {
  if (loading || disposed) return
  const current = ++measurement, token = { ...identity() }; emit('invalidated', token)
  await nextTick()
  if (canvas.value) await waitForRenderStability(canvas.value)
  if (disposed || version !== sequence || current !== measurement) return
  const report = inspect(nodes.value, canvas.value, props.document), baseline = props.baseline && inspect(baselineNodes.value, baselineCanvas.value, props.baseline)
  emit('report', { ...compareValidation(report, baseline, props.baseline, props.document), ...token })
}
async function previewSamples(version: number) {
  const documents = [props.document, props.baseline].filter(Boolean), next: Record<string, Sample> = {}, deadline = Date.now() + 3500
  for (const doc of documents) {
    const bindings = Object.entries<any>(doc.bindings || {}).filter(([, binding]) => !frozen.has(keyOf(binding)))
    if (!bindings.length) continue
    let timer: ReturnType<typeof setTimeout> | undefined
    try {
      const result: any = await Promise.race([rpc('binding.preview', { document: doc }), new Promise<null>(resolve => { timer = setTimeout(() => resolve(null), Math.max(0, deadline - Date.now())) })])
      if (disposed || version !== sequence) return
      for (const [id, binding] of bindings) frozen.set(keyOf(binding), result?.bindings?.[id] || { quality: result?.deniedBindings?.includes(id) ? 'access_denied' : 'unchecked' })
    } catch (error) {
      if (disposed || version !== sequence) return
      const failure = previewRequestFailure(error)
      if (failure) requestErrors.set(doc, failure)
      else for (const [, binding] of bindings) frozen.set(keyOf(binding), { quality: 'unchecked' })
    }
    finally { clearTimeout(timer) }
  }
  for (const doc of documents) for (const binding of Object.values(doc.bindings || {})) next[keyOf(binding)] = frozen.get(keyOf(binding)) || { quality: 'unchecked' }
  if (!disposed && version === sequence) samples.value = next
}
onErrorCaptured((error, instance) => { renderErrors.set(String((instance?.$props as any)?.chartConfig?.id || 'preview'), error.message); if (!loading) validate(); return false })
watch(() => [props.document, props.baseline, props.recheck], async (_next, previous) => {
  const version = ++sequence; ++measurement; loading = true; renderErrors.clear(); requestErrors.clear(); emit('invalidated', { ...identity() })
  if (!previous || previous[1] !== props.baseline || previous[2] !== props.recheck) frozen.clear()
  await previewSamples(version)
  if (!disposed && version === sequence) { loading = false; await validate(version) }
}, { immediate: true })
watch(() => props.locale, () => validate())
function fontsChanged() { validate() }
onMounted(() => {
  observer = new ResizeObserver(entries => { available.value = entries[0].contentRect.width; validate() })
  if (viewport.value) observer.observe(viewport.value)
  document.fonts?.ready.then(() => { if (!disposed) validate() })
  document.fonts?.addEventListener('loadingdone', fontsChanged)
})
onUnmounted(() => { disposed = true; ++sequence; ++measurement; observer?.disconnect(); document.fonts?.removeEventListener('loadingdone', fontsChanged) })
</script>
<style scoped>
.assistant-preview{margin:0;display:flex;flex-direction:column;gap:8px}.assistant-preview figcaption{font-weight:600}.assistant-preview figcaption small{display:block;font-weight:400;color:var(--text-secondary);margin-top:8px}.assistant-preview-viewport{position:relative;width:100%;overflow:hidden;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body)}.assistant-preview-canvas{position:absolute;transform-origin:top left;overflow:hidden;pointer-events:none}.assistant-preview-node{position:absolute}.assistant-preview-node.is-highlighted{outline:3px solid var(--primary-color);outline-offset:2px}.assistant-preview-baseline{position:fixed;left:-100000px;top:0;transform-origin:top left;pointer-events:none}.assistant-preview-value{display:flex;align-items:center;justify-content:center;width:100%;height:100%;font-size:40px;color:var(--text-secondary)}.assistant-preview-data-state{position:absolute;right:8px;bottom:8px;padding:8px;background:var(--bg-body);color:var(--text-secondary);font-size:12px;border-radius:4px}
</style>
