<template>
  <figure class="assistant-preview">
    <figcaption>{{ t('方案预览', 'Proposal preview') }} <small>{{ t('布局与绑定预览 · 实时数据在应用后加载', 'Layout and binding preview · live data loads after applying') }}</small></figcaption>
    <div ref="viewport" class="assistant-preview-viewport" :style="{ height: `${height * scale}px` }">
      <div ref="canvas" class="assistant-preview-canvas" :style="{ width: `${width}px`, height: `${height}px`, transform: `scale(${scale})`, background: document.content?.editCanvasConfig?.background || 'var(--bg-body)' }">
        <div v-for="node in nodes" :key="node.id" class="assistant-preview-node" :data-node="node.id" :style="{ left: `${node.attr.x}px`, top: `${node.attr.y}px`, width: `${node.attr.w}px`, height: `${node.attr.h}px` }">
          <span v-if="node.previewScalar" class="assistant-preview-value">—</span>
          <component v-else :is="node.chartConfig.chartKey" :chart-config="node" :theme-setting="document.content?.editCanvasConfig?.chartThemeSetting" :style="{ width: `${node.attr.w}px`, height: `${node.attr.h}px` }" />
          <small v-if="node.previewBound" class="assistant-preview-data-state">{{ t('已绑定 · 待加载', 'Bound · awaiting data') }}</small>
        </div>
      </div>
    </div>
  </figure>
</template>
<script setup lang="ts">
import { computed, nextTick, onErrorCaptured, onMounted, onUnmounted, ref, watch } from 'vue'
import { sanitizeComponent } from './materials'
import { shapeOf } from './widgets'
import { shapeData } from './shapes'
const props = defineProps<{ document: any; locale: string }>()
const emit = defineEmits<{ (event: 'validated', issues: string[]): void }>()
const t = (zh: string, en: string) => props.locale.startsWith('en') ? en : zh
const viewport = ref<HTMLElement>(), canvas = ref<HTMLElement>(), available = ref(320)
// Older screens store their dimensions only in the GoView canvas config.
const dimension = (axis: 'width' | 'height', fallback: number) => {
  const value = Number(props.document.canvas?.[axis] ?? props.document.content?.editCanvasConfig?.[axis])
  return Number.isFinite(value) && value > 0 ? value : fallback
}
const width = computed(() => dimension('width', 1920)), height = computed(() => dimension('height', 1080))
const scale = computed(() => Math.min(1, available.value / width.value))
let sequence = 0, disposed = false, observer: ResizeObserver | undefined
const renderErrors = new Set<string>()
const nodes = computed(() => {
  const targets = new Map((props.document.targets || []).map((target: any) => [target.componentId, props.document.bindings?.[target.bindingId]]))
  const flatten = (list: any[], x = 0, y = 0): any[] => list.flatMap(item => item.isGroup ? flatten(item.groupList || [], x + item.attr.x, y + item.attr.y) : [{ ...item, attr: { ...item.attr, x: x + item.attr.x, y: y + item.attr.y } }])
  return flatten(props.document.content?.componentList || []).map(item => {
    const node = sanitizeComponent(JSON.parse(JSON.stringify(item)))
    const binding: any = targets.get(item.id), shape = shapeOf(item.chartConfig.key)
    // Separate IDs prevent preview components subscribing to live canvas values.
    node.id = `proposal-preview-${item.id}`
    node.previewBound = !!binding
    node.previewScalar = !!binding && ['number', 'percent', 'progress', 'text'].includes(shape)
    if (binding) node.option.dataset = shapeData(null, binding, shape, props.locale)
    return node
  })
})
async function validate() {
  const version = ++sequence
  await nextTick()
  await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
  if (disposed || version !== sequence) return
  const issues = [...renderErrors]
  for (const node of nodes.value) {
    const { x, y, w, h } = node.attr
    if (x < 0 || y < 0 || x + w > width.value || y + h > height.value) issues.push(`bounds:${node.id}`)
  }
  // Measure text in the actual renderer, after scale and font styles are applied.
  for (const element of canvas.value?.querySelectorAll<HTMLElement>('.assistant-preview-node') || []) {
    const node = nodes.value.find(item => item.id === element.dataset.node)
    if (node && !node.previewBound && shapeOf(node.chartConfig.key) === 'text') {
      const text = element.firstElementChild as HTMLElement
      if (text && (text.scrollWidth > text.clientWidth + 2 || text.scrollHeight > text.clientHeight + 2)) issues.push(`text:${node.id}`)
    }
  }
  emit('validated', [...new Set(issues)])
}
onErrorCaptured(() => { renderErrors.add('render'); emit('validated', [...renderErrors]); return false })
watch(() => props.document, () => { renderErrors.clear(); validate() }, { immediate: true })
onMounted(() => {
  observer = new ResizeObserver(entries => { available.value = entries[0].contentRect.width; validate() })
  if (viewport.value) observer.observe(viewport.value)
  document.fonts?.ready.then(() => { if (!disposed) validate() })
})
onUnmounted(() => { disposed = true; observer?.disconnect() })
</script>
<style scoped>
.assistant-preview{margin:0;display:flex;flex-direction:column;gap:8px}.assistant-preview figcaption{font-weight:600}.assistant-preview figcaption small{display:block;font-weight:400;color:var(--text-secondary);margin-top:8px}.assistant-preview-viewport{position:relative;width:100%;overflow:hidden;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body)}.assistant-preview-canvas{position:absolute;transform-origin:top left;overflow:hidden;pointer-events:none}.assistant-preview-node{position:absolute}.assistant-preview-value{display:flex;align-items:center;justify-content:center;width:100%;height:100%;font-size:40px;color:var(--text-secondary)}.assistant-preview-data-state{position:absolute;right:8px;bottom:8px;padding:4px 8px;background:var(--bg-body);color:var(--text-secondary);font-size:12px;border-radius:4px}
</style>
