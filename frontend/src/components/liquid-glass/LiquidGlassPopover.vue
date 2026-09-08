<template>
  <div ref="rootEl" class="lg-popover-root" :class="{ 'lg-popover-root--open': open }">
    <button
      ref="triggerEl"
      type="button"
      class="lg-popover-trigger"
      :aria-controls="popoverId"
      :aria-expanded="open"
      :aria-label="triggerLabel || label || undefined"
      aria-haspopup="dialog"
      @click="toggle"
    >
      <slot name="trigger" />
    </button>

    <Teleport to="body">
      <Transition name="lg-popover">
        <GlassGroup
          v-if="open"
          :id="popoverId"
          ref="popoverGroup"
          profile="floating"
          shape="panel"
          class="lg-popover"
          :class="[`lg-popover--${placement}`, panelClass]"
          :style="positionStyle"
          role="dialog"
          :aria-label="label || undefined"
          data-liquid-glass-popover
        >
          <slot :close="close" />
        </GlassGroup>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import {
  getCurrentInstance,
  nextTick,
  onBeforeUnmount,
  ref,
} from 'vue'
import GlassGroup from './GlassGroup.vue'

type Placement = 'bottom-start' | 'bottom-end' | 'top-start' | 'top-end'

interface LiquidGlassPopoverProps {
  placement?: Placement
  label?: string
  triggerLabel?: string
  panelClass?: string
}

interface GlassGroupExpose {
  getElement: () => HTMLElement | null
}

const props = withDefaults(defineProps<LiquidGlassPopoverProps>(), {
  placement: 'bottom-start',
  label: '',
  triggerLabel: '',
  panelClass: '',
})

const publishPopoverState = () => {
  if (typeof document === 'undefined') return
  const isOpen = Number(document.documentElement.dataset.noyoLiquidPopoverCount || 0) > 0
  document.documentElement.classList.toggle('noyo-liquid-popover-open', isOpen)
  document.dispatchEvent(new CustomEvent('noyo-liquid-popover-state', {
    detail: { open: isOpen },
  }))
}

const instance = getCurrentInstance()
const popoverId = 'noyo-liquid-popover-' + (instance?.uid ?? 'default')
const rootEl = ref<HTMLElement | null>(null)
const triggerEl = ref<HTMLButtonElement | null>(null)
const popoverGroup = ref<GlassGroupExpose | null>(null)
const open = ref(false)
const positionStyle = ref<Record<string, string>>({ visibility: 'hidden' })
let frameId: number | null = null
let registeredOpen = false

const setRegisteredOpen = (nextOpen: boolean) => {
  if (registeredOpen === nextOpen) return
  registeredOpen = nextOpen
  const root = document.documentElement
  const currentCount = Number(root.dataset.noyoLiquidPopoverCount || 0)
  root.dataset.noyoLiquidPopoverCount = String(
    Math.max(0, currentCount + (nextOpen ? 1 : -1)),
  )
  publishPopoverState()
}

const getPopoverElement = () => popoverGroup.value?.getElement() ?? null

const updatePosition = () => {
  frameId = null
  const trigger = triggerEl.value
  const popover = getPopoverElement()
  if (!trigger || !popover) return

  const triggerRect = trigger.getBoundingClientRect()
  const popoverRect = popover.getBoundingClientRect()
  const gap = 8
  const viewportInset = 16
  const preferredTop = props.placement.startsWith('top')
  const preferredEnd = props.placement.endsWith('end')
  const spaceBelow = window.innerHeight - triggerRect.bottom - viewportInset
  const spaceAbove = triggerRect.top - viewportInset
  const placeAbove = preferredTop
    ? spaceAbove >= popoverRect.height + gap || spaceAbove > spaceBelow
    : !(spaceBelow >= popoverRect.height + gap || spaceBelow >= spaceAbove)

  let top = placeAbove
    ? triggerRect.top - popoverRect.height - gap
    : triggerRect.bottom + gap
  let left = preferredEnd
    ? triggerRect.right - popoverRect.width
    : triggerRect.left

  top = Math.max(
    viewportInset,
    Math.min(top, window.innerHeight - popoverRect.height - viewportInset),
  )
  left = Math.max(
    viewportInset,
    Math.min(left, window.innerWidth - popoverRect.width - viewportInset),
  )

  positionStyle.value = {
    top: Math.round(top) + 'px',
    left: Math.round(left) + 'px',
    visibility: 'visible',
  }
}

const schedulePosition = () => {
  if (!open.value || frameId !== null) return
  frameId = requestAnimationFrame(updatePosition)
}

const close = (restoreFocus = false) => {
  if (!open.value) return
  open.value = false
  setRegisteredOpen(false)
  unbindOpenListeners()
  if (frameId !== null) {
    cancelAnimationFrame(frameId)
    frameId = null
  }
  if (restoreFocus) nextTick(() => triggerEl.value?.focus())
}

const toggle = async () => {
  if (open.value) {
    close()
    return
  }
  positionStyle.value = { visibility: 'hidden' }
  open.value = true
  setRegisteredOpen(true)
  bindOpenListeners()
  await nextTick()
  schedulePosition()
}

const onDocumentPointerDown = (event: PointerEvent) => {
  if (!open.value) return
  const target = event.target as Node
  if (rootEl.value?.contains(target) || getPopoverElement()?.contains(target)) return
  close()
}

const onKeydown = (event: KeyboardEvent) => {
  if (event.key !== 'Escape' || !open.value) return
  event.preventDefault()
  close(true)
}

const bindOpenListeners = () => {
  document.addEventListener('pointerdown', onDocumentPointerDown, { passive: true })
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('resize', schedulePosition, { passive: true })
  window.addEventListener('scroll', schedulePosition, { passive: true, capture: true })
}

const unbindOpenListeners = () => {
  document.removeEventListener('pointerdown', onDocumentPointerDown)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('resize', schedulePosition)
  window.removeEventListener('scroll', schedulePosition, true)
}

onBeforeUnmount(() => {
  setRegisteredOpen(false)
  unbindOpenListeners()
  if (frameId !== null) cancelAnimationFrame(frameId)
})
</script>

<style scoped>
.lg-popover-root {
  position: relative;
  display: inline-flex;
}

.lg-popover-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 0;
  border-radius: var(--noyo-radius-control);
  background: var(--noyo-color-transparent);
  color: inherit;
  cursor: pointer;
  font: inherit;
}
</style>
