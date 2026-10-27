<template>
  <div class="ioc-assistant-launcher" :class="{ 'is-dragging': dragging }" :style="{ right: `${position.right}px`, bottom: `${position.bottom}px` }" @pointerdown="startDrag" @lostpointercapture="cancelDrag">
    <AiAssistantFab ref="button" :label="label" :aria-expanded="open" aria-controls="ioc-assistant-panel" @click="activate" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import AiAssistantFab from '../../../../../frontend/src/components/liquid-glass/AiAssistantFab.vue'

defineProps<{ label: string; open: boolean }>()
const emit = defineEmits<{ (event: 'activate'): void }>()
const button = ref<InstanceType<typeof AiAssistantFab>>()
const KEY = 'noyo.assistant.launcher.position.v1', SIZE = 56, EDGE = 8
function fit(value: { right: number; bottom: number }) {
  return { right: Math.max(EDGE, Math.min(value.right, window.innerWidth - SIZE - EDGE)), bottom: Math.max(EDGE, Math.min(value.bottom, window.innerHeight - SIZE - EDGE)) }
}
function restore() {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY) || 'null')
    if (saved && Number.isFinite(saved.right) && Number.isFinite(saved.bottom)) return fit(saved)
  } catch { /* Storage may be unavailable. */ }
  return fit({ right: 24, bottom: 24 })
}
const position = ref(restore()), dragging = ref(false)
let gesture: { id: number; x: number; y: number; right: number; bottom: number; target: HTMLElement } | null = null
let dragged = false
function startDrag(event: PointerEvent) {
  if (!event.isPrimary || event.button !== 0) return
  dragged = false
  const target = event.currentTarget as HTMLElement
  gesture = { id: event.pointerId, x: event.clientX, y: event.clientY, ...position.value, target }
  window.addEventListener('pointermove', moveDrag)
  window.addEventListener('pointerup', endDrag)
  window.addEventListener('pointercancel', cancelDrag)
}
function moveDrag(event: PointerEvent) {
  if (!gesture || event.pointerId !== gesture.id) return
  const dx = event.clientX - gesture.x, dy = event.clientY - gesture.y
  if (!dragged && Math.hypot(dx, dy) <= 5) return
  if (!dragged) gesture.target.setPointerCapture(event.pointerId)
  dragged = true; dragging.value = true
  position.value = fit({ right: gesture.right - dx, bottom: gesture.bottom - dy })
  event.preventDefault()
}
function persist() { try { localStorage.setItem(KEY, JSON.stringify(position.value)) } catch { /* Optional preference. */ } }
function endDrag(event: PointerEvent) {
  if (!gesture || event.pointerId !== gesture.id) return
  const { target, id } = gesture
  gesture = null; dragging.value = false
  removeListeners()
  if (target.hasPointerCapture(id)) target.releasePointerCapture(id)
  if (dragged) persist()
}
function removeListeners() { window.removeEventListener('pointermove', moveDrag); window.removeEventListener('pointerup', endDrag); window.removeEventListener('pointercancel', cancelDrag) }
function cancelDrag() {
  const previous = gesture
  gesture = null; dragging.value = false; removeListeners()
  if (previous?.target.hasPointerCapture(previous.id)) previous.target.releasePointerCapture(previous.id)
}
function activate(event: MouseEvent) {
  if (event.detail > 0 && dragged) { event.preventDefault(); return }
  emit('activate')
}
function resize() { position.value = fit(position.value) }
onMounted(() => window.addEventListener('resize', resize))
onUnmounted(() => { window.removeEventListener('resize', resize); cancelDrag() })
defineExpose({ focus: () => button.value?.focus() })
</script>

<style scoped>
.ioc-assistant-launcher { position: fixed; z-index: 61; width: 56px; height: 56px; touch-action: none; user-select: none; }
.ioc-assistant-launcher :deep(.noyo-glass-btn) { cursor: grab; }
.ioc-assistant-launcher.is-dragging :deep(.noyo-glass-btn) { cursor: grabbing; animation: none; transition: none; transform: none; }
</style>
