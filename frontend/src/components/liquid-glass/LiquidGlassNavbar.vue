<template>
  <GlassGroup
    ref="glassGroup"
    as="nav"
    profile="regular"
    shape="rounded"
    class="lg-navbar"
  >
    <slot />
  </GlassGroup>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import GlassGroup from './GlassGroup.vue'

interface LiquidGlassNavbarProps {
  scrollTarget?: string | HTMLElement | null
  threshold?: number
}

interface GlassGroupExpose {
  getElement: () => HTMLElement | null
}

const props = withDefaults(defineProps<LiquidGlassNavbarProps>(), {
  scrollTarget: null,
  threshold: 24,
})

const glassGroup = ref<GlassGroupExpose | null>(null)
let scrollEl: Window | HTMLElement | null = null
let frameId: number | null = null

const resolveScrollTarget = () => {
  if (props.scrollTarget == null) return window
  if (typeof props.scrollTarget === 'string') {
    return document.querySelector<HTMLElement>(props.scrollTarget)
  }
  return props.scrollTarget
}

const readScrollTop = () => {
  if (!scrollEl) return 0
  return scrollEl === window
    ? window.scrollY || document.documentElement.scrollTop || 0
    : (scrollEl as HTMLElement).scrollTop
}

const schedule = () => {
  if (frameId !== null) return
  frameId = requestAnimationFrame(() => {
    frameId = null
    const edge = Math.min(1, readScrollTop() / Math.max(1, props.threshold))
    glassGroup.value
      ?.getElement()
      ?.style.setProperty('--noyo-scroll-edge', edge.toFixed(3))
  })
}

onMounted(() => {
  scrollEl = resolveScrollTarget()
  scrollEl?.addEventListener('scroll', schedule, { passive: true })
  schedule()
})

onBeforeUnmount(() => {
  if (frameId !== null) cancelAnimationFrame(frameId)
  scrollEl?.removeEventListener('scroll', schedule)
  scrollEl = null
})
</script>
