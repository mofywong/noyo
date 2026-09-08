import type { Ref } from 'vue'
import { resolveGlassPointerState } from '../utils/glassPointer.js'

export function useGlassPointer(el: Ref<HTMLElement | null>) {
  const canFollow =
    typeof window !== 'undefined' &&
    window.matchMedia('(hover: hover)').matches &&
    !window.matchMedia('(pointer: coarse)').matches &&
    !window.matchMedia('(prefers-reduced-motion: reduce)').matches

  let target: HTMLElement | null = null
  let rect: DOMRect | null = null
  let latestPoint: { clientX: number; clientY: number } | null = null
  let frameId: number | null = null
  let rectFrameId: number | null = null
  let resizeObserver: ResizeObserver | null = null

  const setRestState = () => {
    if (!target) return
    target.classList.remove('noyo-glass-group--energized')
    target.style.setProperty('--noyo-pointer-x', '50%')
    target.style.setProperty('--noyo-pointer-y', '50%')
    target.style.setProperty('--noyo-glass-energy', '0')
    target.style.setProperty('--noyo-edge-energy', '0')
  }

  const updateRect = () => {
    if (target) rect = target.getBoundingClientRect()
  }

  const scheduleRectUpdate = () => {
    if (rectFrameId !== null) return
    rectFrameId = requestAnimationFrame(() => {
      rectFrameId = null
      updateRect()
    })
  }

  const flushPointer = () => {
    frameId = null
    if (!target || !rect || !latestPoint) return

    const state = resolveGlassPointerState(latestPoint, rect)
    target.style.setProperty('--noyo-pointer-x', state.x.toFixed(2) + '%')
    target.style.setProperty('--noyo-pointer-y', state.y.toFixed(2) + '%')
    target.style.setProperty('--noyo-glass-energy', state.energy.toFixed(3))
    target.style.setProperty('--noyo-edge-energy', state.edgeEnergy.toFixed(3))
  }

  const schedulePointer = () => {
    if (frameId === null) frameId = requestAnimationFrame(flushPointer)
  }

  const onPointerEnter = (event: PointerEvent) => {
    if (document.documentElement.classList.contains('noyo-liquid-popover-open')) return
    updateRect()
    latestPoint = event
    target?.classList.add('noyo-glass-group--energized')
    schedulePointer()
  }

  const onPointerMove = (event: PointerEvent) => {
    if (document.documentElement.classList.contains('noyo-liquid-popover-open')) return
    latestPoint = event
    schedulePointer()
  }

  const onPointerLeave = () => {
    latestPoint = null
    setRestState()
  }

  const onWindowBlur = () => {
    latestPoint = null
    setRestState()
  }

  const onPopoverState = () => {
    latestPoint = null
    setRestState()
  }

  const bind = (nextTarget: HTMLElement | null) => {
    if (target === nextTarget) return
    unbind()
    target = nextTarget || el.value
    if (!target) return

    setRestState()
    updateRect()
    if (!canFollow) return

    target.addEventListener('pointerenter', onPointerEnter, { passive: true })
    target.addEventListener('pointermove', onPointerMove, { passive: true })
    target.addEventListener('pointerleave', onPointerLeave, { passive: true })
    window.addEventListener('resize', scheduleRectUpdate, { passive: true })
    window.addEventListener('scroll', scheduleRectUpdate, {
      passive: true,
      capture: true,
    })
    window.addEventListener('blur', onWindowBlur)
    document.addEventListener('noyo-liquid-popover-state', onPopoverState)

    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(scheduleRectUpdate)
      resizeObserver.observe(target)
    }
  }

  const unbind = () => {
    if (frameId !== null) cancelAnimationFrame(frameId)
    if (rectFrameId !== null) cancelAnimationFrame(rectFrameId)
    frameId = null
    rectFrameId = null
    latestPoint = null

    if (target) {
      target.removeEventListener('pointerenter', onPointerEnter)
      target.removeEventListener('pointermove', onPointerMove)
      target.removeEventListener('pointerleave', onPointerLeave)
      setRestState()
    }

    window.removeEventListener('resize', scheduleRectUpdate)
    window.removeEventListener('scroll', scheduleRectUpdate, true)
    window.removeEventListener('blur', onWindowBlur)
    document.removeEventListener('noyo-liquid-popover-state', onPopoverState)
    resizeObserver?.disconnect()
    resizeObserver = null
    target = null
    rect = null
  }

  return { bind, unbind }
}
