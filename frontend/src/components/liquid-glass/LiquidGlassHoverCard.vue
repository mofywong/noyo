<template>
  <Teleport to="body">
    <Transition name="lg-hover-card">
      <GlassGroup
        v-if="open"
        ref="panelGroup"
        class="lg-hover-card"
        :class="[panelClass, { 'lg-hover-card--interactive': interactive }]"
        profile="floating"
        shape="panel"
        :style="positionStyle"
        role="tooltip"
        :aria-label="label || undefined"
        data-liquid-glass-hover-card
        @mouseenter="$emit('mouseenter', $event)"
        @mouseleave="$emit('mouseleave', $event)"
      >
        <slot />
      </GlassGroup>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'
import GlassGroup from './GlassGroup.vue'

/**
 * 悬浮信息卡片原语（Liquid Glass Hover Card）。
 *
 * 交互设计：
 * 1. 支持 interactive 交互模式（默认开启）：鼠标可平滑移入卡片查看长属性或滚动查看。
 * 2. 内部滚动防护：卡片内部的滚动操作不会触发全局视口 scroll dismiss。
 * 3. 层级固定低于 popover（`--noyo-layer-popover - 1`），保证行内操作弹层始终浮于卡片之上。
 */
interface LiquidGlassHoverCardProps {
  /** 是否显示 */
  open: boolean
  /** 锚点视口坐标（光标进入触发元素瞬间的 clientX / clientY） */
  anchorX?: number
  anchorY?: number
  /** 相对锚点的偏移量 */
  offsetX?: number
  offsetY?: number
  /** 追加的面板类名 */
  panelClass?: string
  /** 无障碍标签 */
  label?: string
  /** 是否允许鼠标移入卡片进行交互/滚动 */
  interactive?: boolean
}

interface GlassGroupExpose {
  getElement: () => HTMLElement | null
}

const props = withDefaults(defineProps<LiquidGlassHoverCardProps>(), {
  anchorX: 0,
  anchorY: 0,
  offsetX: 10,
  offsetY: 10,
  panelClass: '',
  label: '',
  interactive: true,
})

const emit = defineEmits<{
  (e: 'dismiss'): void
  (e: 'mouseenter', ev: MouseEvent): void
  (e: 'mouseleave', ev: MouseEvent): void
}>()

const panelGroup = ref<GlassGroupExpose | null>(null)
const positionStyle = ref<Record<string, string>>({ visibility: 'hidden' })
let frameId: number | null = null

const getPanelElement = () => panelGroup.value?.getElement() ?? null

/**
 * 计算最终落点：优先落在锚点右下，空间不足则水平/垂直翻转，最后钳制在视口安全边距内。
 */
const updatePosition = () => {
  frameId = null
  const panel = getPanelElement()
  if (!panel) return

  const rect = panel.getBoundingClientRect()
  const inset = 12
  const viewportWidth = window.innerWidth
  const viewportHeight = window.innerHeight

  let left = props.anchorX + props.offsetX
  let top = props.anchorY + props.offsetY
  let flippedX = false
  let flippedY = false

  if (left + rect.width > viewportWidth - inset) {
    left = props.anchorX - props.offsetX - rect.width
    flippedX = true
  }
  if (top + rect.height > viewportHeight - inset) {
    top = props.anchorY - props.offsetY - rect.height
    flippedY = true
  }

  left = Math.max(inset, Math.min(left, viewportWidth - rect.width - inset))
  top = Math.max(inset, Math.min(top, viewportHeight - rect.height - inset))

  positionStyle.value = {
    top: Math.round(top) + 'px',
    left: Math.round(left) + 'px',
    // 变换原点跟随翻转方向，让入场缩放从贴近光标的一角展开
    transformOrigin: `${flippedX ? 'right' : 'left'} ${flippedY ? 'bottom' : 'top'}`,
    visibility: 'visible',
  }
}

const schedulePosition = () => {
  if (!props.open || frameId !== null) return
  frameId = requestAnimationFrame(updatePosition)
}

const onViewportScroll = (e?: Event) => {
  if (!props.open) return
  // 如果滚动事件来自卡片内部自身的滚动容器，绝不能关闭卡片
  const target = e?.target as Node | null
  const panel = getPanelElement()
  if (panel && target && panel.contains(target)) {
    return
  }
  emit('dismiss')
}

const bindListeners = () => {
  window.addEventListener('resize', schedulePosition, { passive: true })
  window.addEventListener('scroll', onViewportScroll, { passive: true, capture: true })
}

const unbindListeners = () => {
  window.removeEventListener('resize', schedulePosition)
  window.removeEventListener('scroll', onViewportScroll, true)
  if (frameId !== null) {
    cancelAnimationFrame(frameId)
    frameId = null
  }
}

watch(
  () => props.open,
  async (nextOpen) => {
    if (!nextOpen) {
      unbindListeners()
      positionStyle.value = { visibility: 'hidden' }
      return
    }
    positionStyle.value = { visibility: 'hidden' }
    bindListeners()
    await nextTick()
    schedulePosition()
  },
)

// 保持打开状态下更换锚点（例如从一行移到另一行）时重新定位
watch(
  () => [props.anchorX, props.anchorY],
  () => {
    if (!props.open) return
    positionStyle.value = { visibility: 'hidden' }
    schedulePosition()
  },
)

onBeforeUnmount(unbindListeners)
</script>
