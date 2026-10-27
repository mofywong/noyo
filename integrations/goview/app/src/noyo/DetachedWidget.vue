<template>
  <component
    :is="resolveDynamicComponent(widget)"
    :chartConfig="renderModel"
    :themeSetting="renderTheme"
    :themeColor="renderColors"
  />
</template>

<script setup lang="ts">
import { nextTick, onUnmounted, reactive, ref, resolveDynamicComponent, watch } from 'vue'
import cloneDeep from 'lodash/cloneDeep'
import { replayWidget } from './runtime'

const props = defineProps<{
  widget: string
  chartConfig: Record<string, any>
  themeSetting: Record<string, any>
  themeColor: any
}>()

// The chart store is the authored document. Upstream renderers and their data
// hooks intentionally mutate chartConfig; they receive only this detached
// model, including in groups and in the published viewer. Preserve the root
// identity because useChartDataFetch captures it once during widget setup.
const renderModel = reactive(cloneDeep(props.chartConfig))
const renderTheme = reactive(cloneDeep(props.themeSetting))
const renderColors = ref(cloneDeep(props.themeColor))
const container = (value: any) => value !== null && typeof value === 'object'
function reconcile(target: any, next: any) {
  // Several upstream widgets call toRefs(option) during setup. Replacing even
  // the nested option object would strand those refs on the previous style.
  // Keep shared container identities while removing runtime-only properties.
  for (const key of Object.keys(target)) if (!(key in next)) delete target[key]
  if (Array.isArray(target)) target.length = next.length
  for (const key of Object.keys(next)) {
    if (container(target[key]) && container(next[key]) && Array.isArray(target[key]) === Array.isArray(next[key])) reconcile(target[key], next[key])
    else target[key] = next[key]
  }
}
let alive = true, queued = false
function queueReplay() {
  if (queued) return
  queued = true
  nextTick(() => { queued = false; if (alive) replayWidget(renderModel.id) })
}
watch(() => props.chartConfig, authored => {
  reconcile(renderModel, cloneDeep(authored))
  // A style/geometry edit replaces authored defaults in the render model.
  // Restore its cached live sample after child config watchers have settled.
  queueReplay()
}, { deep: true })
watch(() => props.themeSetting, authored => { reconcile(renderTheme, cloneDeep(authored)); queueReplay() }, { deep: true })
watch(() => props.themeColor, authored => {
  const next = cloneDeep(authored)
  if (container(renderColors.value) && container(next) && Array.isArray(renderColors.value) === Array.isArray(next)) reconcile(renderColors.value, next)
  else renderColors.value = next
  queueReplay()
}, { deep: true })
onUnmounted(() => { alive = false })
</script>
