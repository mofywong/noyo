<template>
  <span
    class="noyo-glass-btn-shell"
    :data-refraction="refractionReady && filterAsset ? 'active' : 'fallback'"
  >
    <!--
      物理折射滤镜定义（Apple Liquid Glass 光学管线）
      Physical refraction filter definitions (Apple Liquid Glass optics pipeline)

      管线 / Pipeline:
        feImage 位移图 → RGB 三通道不同 scale 位移（色散 chromatic aberration）
        → screen 混合 → 形状遮罩裁剪 → 定向镜面高光叠加
      位移图 R/G 通道编码折射方向，B 通道编码光源方向的镜面高光强度，
      A 通道为按钮轮廓遮罩；由 liquidGlassRefraction.js 统一生成。
    -->
    <svg
      class="noyo-glass-btn-filters"
      aria-hidden="true"
      focusable="false"
      width="0"
      height="0"
    >
      <defs>
        <filter
          v-if="filterAsset"
          :id="filterId"
          :x="filterAsset.region.x"
          :y="filterAsset.region.y"
          :width="filterAsset.region.width"
          :height="filterAsset.region.height"
          color-interpolation-filters="sRGB"
        >
          <feImage
            :href="filterAsset.mapUrl"
            x="0"
            y="0"
            width="100%"
            height="100%"
            preserveAspectRatio="none"
            result="btn-map"
          />

          <!-- 红色通道：最强位移（色散外圈） / Red channel: strongest displacement -->
          <feDisplacementMap
            in="SourceGraphic"
            in2="btn-map"
            :scale="filterAsset.redScale"
            xChannelSelector="R"
            yChannelSelector="G"
            result="btn-warp-r"
          />
          <feColorMatrix
            in="btn-warp-r"
            type="matrix"
            :values="RED_CHANNEL_MATRIX"
            result="btn-channel-r"
          />

          <!-- 绿色通道：基准位移 / Green channel: base displacement -->
          <feDisplacementMap
            in="SourceGraphic"
            in2="btn-map"
            :scale="filterAsset.greenScale"
            xChannelSelector="R"
            yChannelSelector="G"
            result="btn-warp-g"
          />
          <feColorMatrix
            in="btn-warp-g"
            type="matrix"
            :values="GREEN_CHANNEL_MATRIX"
            result="btn-channel-g"
          />

          <!-- 蓝色通道：最弱位移（色散内圈） / Blue channel: weakest displacement -->
          <feDisplacementMap
            in="SourceGraphic"
            in2="btn-map"
            :scale="filterAsset.blueScale"
            xChannelSelector="R"
            yChannelSelector="G"
            result="btn-warp-b"
          />
          <feColorMatrix
            in="btn-warp-b"
            type="matrix"
            :values="BLUE_CHANNEL_MATRIX"
            result="btn-channel-b"
          />

          <feBlend
            in="btn-channel-r"
            in2="btn-channel-g"
            mode="screen"
            result="btn-channels-rg"
          />
          <feBlend
            in="btn-channels-rg"
            in2="btn-channel-b"
            mode="screen"
            result="btn-channels-rgb"
          />

          <!-- 定向镜面高光（源自位移图 B 通道） / Directional specular from map blue channel -->
          <feColorMatrix
            in="btn-map"
            type="matrix"
            :values="SPECULAR_ALPHA_MATRIX"
            result="btn-specular"
          />
          <feComposite
            in="btn-specular"
            in2="btn-map"
            operator="in"
            result="btn-specular-clipped"
          />

          <!-- 将折射结果裁剪到按钮轮廓 / Clip refraction to the button silhouette -->
          <feComposite
            in="btn-channels-rgb"
            in2="btn-map"
            operator="in"
            result="btn-inside"
          />
          <feComposite
            in="SourceGraphic"
            in2="btn-map"
            operator="out"
            result="btn-outside"
          />
          <feMerge result="btn-shape">
            <feMergeNode in="btn-outside" />
            <feMergeNode in="btn-inside" />
          </feMerge>
          <feBlend
            in="btn-shape"
            in2="btn-specular-clipped"
            mode="screen"
            result="btn-output"
          />
        </filter>
      </defs>
    </svg>

    <!-- 苹果液态纯透玻璃按钮 / Apple Liquid Glass crystal button -->
    <button
      ref="buttonEl"
      :type="type"
      class="noyo-glass-btn"
      :class="[
        `noyo-glass-btn--${variant}`,
        `noyo-glass-btn--${size}`,
        {
          'noyo-glass-btn--loading': loading,
          'noyo-glass-btn--pressed': pressed
        }
      ]"
      :style="computedStyle"
      :disabled="disabled || loading"
      @pointermove="onPointerMove"
      @pointerleave="onPointerLeave"
      @pointerdown="onPointerDown"
      @pointerup="onPointerUp"
      @pointercancel="onPointerUp"
      @click="onClick"
    >
      <!-- 玻璃体：背景折射采样 + 玻璃 tint / Glass body: backdrop refraction sampling + tint -->
      <span class="noyo-glass-btn__glass" :style="glassBackdropStyle" aria-hidden="true"></span>

      <!-- 镜面高光与玻璃厚度 / Specular highlight and glass thickness -->
      <span class="noyo-glass-btn__specular" aria-hidden="true"></span>

      <!-- 边缘色散微光 / Prismatic edge dispersion -->
      <span class="noyo-glass-btn__prism" aria-hidden="true"></span>

      <!-- 动态焦散光斑（跟随指针的折射焦点） / Dynamic caustic spotlight following the pointer -->
      <span class="noyo-glass-btn__caustic" aria-hidden="true"></span>

      <!-- 按压波纹（玻璃受力波动） / Press ripple (glass pressure wave) -->
      <span
        v-if="ripple.seq > 0"
        :key="ripple.seq"
        class="noyo-glass-btn__ripple"
        :style="{ '--lg-btn-ripple-x': ripple.x + 'px', '--lg-btn-ripple-y': ripple.y + 'px' }"
        aria-hidden="true"
      ></span>

      <!-- 按钮内容 / Button content -->
      <span class="noyo-glass-btn__content">
        <span v-if="loading" class="spinner-border spinner-border-sm" role="status" aria-hidden="true"></span>
        <slot v-else name="icon">
          <i v-if="icon" :class="[icon, 'noyo-glass-btn__icon']" aria-hidden="true"></i>
        </slot>
        <span class="noyo-glass-btn__text">
          <slot />
        </span>
      </span>
    </button>
  </span>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  computeRoundedRectDisplacementMap,
  computeSvgFilterRegion,
  encodeDisplacementMapDataUrl,
  supportsBackdropSvgFilter,
} from '../../utils/liquidGlassRefraction.js'

interface LiquidGlassButtonProps {
  variant?:
    | 'primary'
    | 'secondary'
    | 'crystal'
    | 'success'
    | 'danger'
    | 'warning'
    | 'info'
    | 'neutral'
    | 'outline-primary'
    | 'outline-secondary'
    | 'outline-info'
  size?: 'sm' | 'md' | 'lg'
  type?: 'button' | 'submit' | 'reset'
  icon?: string
  disabled?: boolean
  loading?: boolean
  /** 是否启用指针 3D 弯曲（可弯曲玻璃） / enable pointer 3D bending */
  tilt?: boolean
  /** 是否启用背景折射管线 / enable backdrop refraction pipeline */
  refract?: boolean
}

const props = withDefaults(defineProps<LiquidGlassButtonProps>(), {
  variant: 'primary',
  size: 'md',
  type: 'button',
  disabled: false,
  loading: false,
  tilt: true,
  refract: true,
})

const emit = defineEmits<{
  (e: 'click', event: MouseEvent): void
}>()

const RED_CHANNEL_MATRIX = '1 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1 0'
const GREEN_CHANNEL_MATRIX = '0 0 0 0 0  0 1 0 0 0  0 0 0 0 0  0 0 0 1 0'
const BLUE_CHANNEL_MATRIX = '0 0 0 0 0  0 0 0 0 0  0 0 1 0 0  0 0 0 1 0'
const SPECULAR_ALPHA_MATRIX = '0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0.3 0 0'

/** 色散系数：RGB 三通道位移差异，产生边缘彩虹折射 / dispersion factor */
const DISPERSION = 0.16

interface FilterAsset {
  mapUrl: string
  redScale: number
  greenScale: number
  blueScale: number
  region: { x: string; y: string; width: string; height: string }
}

const instanceId = Math.random().toString(36).substring(2, 9)
const filterId = `noyo-lg-btn-refract-${instanceId}`

const buttonEl = ref<HTMLButtonElement | null>(null)
const refractionReady = ref(false)
const filterAsset = ref<FilterAsset | null>(null)
const pointerX = ref(50)
const pointerY = ref(50)
const tiltX = ref(0)
const tiltY = ref(0)
const pressed = ref(false)
const ripple = ref({ x: 0, y: 0, seq: 0 })

// 能力与偏好检测 / capability and preference detection
const prefersReducedMotion =
  typeof window !== 'undefined' &&
  window.matchMedia('(prefers-reduced-motion: reduce)').matches
const hasFinePointer =
  typeof window !== 'undefined' &&
  window.matchMedia('(hover: hover)').matches &&
  window.matchMedia('(pointer: fine)').matches
const canTilt = computed(() => props.tilt && hasFinePointer && !prefersReducedMotion)

/** backdrop-filter + SVG url() 能力检测，模块级缓存，避免每个实例重复 DOM probe */
let cachedBackdropSvgSupport: boolean | null = null
function backdropSvgSupport(): boolean {
  if (cachedBackdropSvgSupport === null) {
    cachedBackdropSvgSupport = supportsBackdropSvgFilter(
      window.CSS,
      window,
      document,
      navigator,
    )
  }
  return cachedBackdropSvgSupport
}

/**
 * 依据按钮实测尺寸生成 pill 透镜位移图与 filter 资产。
 * Build a pill-shaped lens displacement map and filter asset from measured size.
 */
function buildFilterAsset(width: number, height: number): FilterAsset | null {
  const w = Math.round(width)
  const h = Math.round(height)
  if (w < 16 || h < 8) return null
  const radius = h / 2
  const depth = Math.max(3, Math.min(10, h * 0.2))
  const strength = Math.max(6, Math.min(12, h * 0.24))
  const map = computeRoundedRectDisplacementMap({
    width: w,
    height: h,
    radius,
    depth,
    curvature: 0.4,
    bend: 0.95,
    bendWidth: 0.16,
    specularAngle: 315,
    strength,
  })
  const mapUrl = encodeDisplacementMapDataUrl(map, document)
  const region = computeSvgFilterRegion({ width: w, height: h, strength, blur: 0 })
  return {
    mapUrl,
    redScale: Number((strength * (1 + DISPERSION)).toFixed(2)),
    greenScale: Number(strength.toFixed(2)),
    blueScale: Number((strength * (1 - DISPERSION)).toFixed(2)),
    region,
  }
}

let resizeObserver: ResizeObserver | null = null
let rebuildTimer: number | null = null

function rebuildAsset() {
  const el = buttonEl.value
  if (!el || !refractionReady.value) return
  try {
    const rect = el.getBoundingClientRect()
    if (rect.width > 0 && rect.height > 0) {
      filterAsset.value = buildFilterAsset(rect.width, rect.height)
    }
  } catch (error) {
    console.warn('Liquid Glass button refraction disabled:', error)
    refractionReady.value = false
    filterAsset.value = null
  }
}

function scheduleRebuild() {
  if (rebuildTimer !== null) window.clearTimeout(rebuildTimer)
  rebuildTimer = window.setTimeout(() => {
    rebuildTimer = null
    rebuildAsset()
  }, 150)
}

const clamp = (value: number, min: number, max: number) =>
  Math.max(min, Math.min(max, value))

const MAX_TILT_DEG = 3.2

function onPointerMove(event: PointerEvent) {
  const el = buttonEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return
  const px = clamp(((event.clientX - rect.left) / rect.width) * 100, 0, 100)
  const py = clamp(((event.clientY - rect.top) / rect.height) * 100, 0, 100)
  pointerX.value = px
  pointerY.value = py
  if (canTilt.value && !pressed.value) {
    // 玻璃向指针位置凸起弯曲 / the glass bends toward the pointer
    tiltX.value = ((py - 50) / 50) * MAX_TILT_DEG
    tiltY.value = ((px - 50) / 50) * MAX_TILT_DEG
  }
}

function onPointerLeave() {
  pointerX.value = 50
  pointerY.value = 50
  tiltX.value = 0
  tiltY.value = 0
  pressed.value = false
}

function onPointerDown(event: PointerEvent) {
  const el = buttonEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  ripple.value = {
    x: clamp(event.clientX - rect.left, 0, rect.width),
    y: clamp(event.clientY - rect.top, 0, rect.height),
    seq: ripple.value.seq + 1,
  }
  pressed.value = true
  // 按压缩平玻璃 / flatten the glass while pressed
  tiltX.value = 0
  tiltY.value = 0
}

function onPointerUp() {
  pressed.value = false
}

function onClick(event: MouseEvent) {
  emit('click', event)
}

onMounted(() => {
  if (props.refract && backdropSvgSupport()) {
    refractionReady.value = true
    nextTick(rebuildAsset)
  }
  if (buttonEl.value && typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(scheduleRebuild)
    resizeObserver.observe(buttonEl.value)
  }
})

onBeforeUnmount(() => {
  if (rebuildTimer !== null) window.clearTimeout(rebuildTimer)
  if (resizeObserver) {
    resizeObserver.disconnect()
    resizeObserver = null
  }
})

const computedStyle = computed(() => ({
  '--lg-btn-px': pointerX.value.toFixed(1) + '%',
  '--lg-btn-py': pointerY.value.toFixed(1) + '%',
  '--lg-btn-tilt-x': tiltX.value.toFixed(2) + 'deg',
  '--lg-btn-tilt-y': tiltY.value.toFixed(2) + 'deg',
}))

/**
 * 玻璃体 backdrop-filter：
 * 折射管线 = 背景模糊 → 透镜位移（三通道色散）→ 提饱和与亮度；
 * 不支持 SVG url() 时降级为纯模糊玻璃。
 * Glass body backdrop-filter: refraction pipeline or blur-only fallback.
 */
const glassBackdropStyle = computed(() => {
  const saturation = 'var(--lg-btn-saturate)'
  const brightness = 'var(--lg-btn-brightness)'
  if (refractionReady.value && filterAsset.value) {
    const refraction = `blur(var(--lg-btn-blur)) url(#${filterId}) saturate(${saturation}) brightness(${brightness})`
    return {
      'backdrop-filter': refraction,
      '-webkit-backdrop-filter': refraction,
    }
  }
  const fallback = `blur(var(--lg-btn-blur-fallback)) saturate(${saturation}) brightness(${brightness})`
  return {
    'backdrop-filter': fallback,
    '-webkit-backdrop-filter': fallback,
  }
})
</script>

<style scoped>
/* ── 外壳：提供 3D 透视 / Shell: provides 3D perspective ── */
.noyo-glass-btn-shell {
  display: inline-flex;
  position: relative;
  vertical-align: middle;
  perspective: 640px;
}

.noyo-glass-btn-filters {
  position: absolute;
  width: 0;
  height: 0;
  overflow: hidden;
  pointer-events: none;
}

/* ── 按钮主体 / Button body ── */
.noyo-glass-btn {
  --lg-btn-px: 50%;
  --lg-btn-py: 50%;
  --lg-btn-tilt-x: 0deg;
  --lg-btn-tilt-y: 0deg;
  --lg-btn-caustic-opacity: 0;
  --lg-btn-text: var(--lg-btn-text-primary);
  --lg-btn-spring: cubic-bezier(0.34, 1.56, 0.64, 1);

  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: var(--noyo-radius-pill);
  font: inherit;
  font-size: 0.875rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  line-height: 1.25;
  color: var(--lg-btn-text);
  background: var(--noyo-color-transparent);
  cursor: pointer;
  user-select: none;
  -webkit-tap-highlight-color: transparent;
  isolation: isolate;

  /* 流体弯曲：玻璃向指针凸起，弹簧回弹 / fluid bending with spring rebound */
  transform:
    translateY(0)
    rotateX(var(--lg-btn-tilt-x))
    rotateY(var(--lg-btn-tilt-y))
    scale(var(--lg-btn-squeeze));
  transform-style: preserve-3d;
  box-shadow: var(--lg-btn-shadow);

  transition:
    transform 0.5s var(--lg-btn-spring),
    box-shadow 0.35s ease;
}

/* ── 尺寸 / Sizes ── */
.noyo-glass-btn--sm {
  min-height: 32px;
  padding: 4px 14px;
  font-size: 0.8125rem;
  gap: 6px;
}

.noyo-glass-btn--md {
  min-height: 38px;
  padding: 6px 18px;
  font-size: 0.875rem;
  gap: 7px;
}

.noyo-glass-btn--lg {
  min-height: 46px;
  padding: 10px 26px;
  font-size: 1rem;
  gap: 9px;
}

/* ── 变体：文字颜色与描边 / Variants: label color & stroke ── */
.noyo-glass-btn--primary {
  --lg-btn-text: var(--lg-btn-text-primary);
}

.noyo-glass-btn--secondary,
.noyo-glass-btn--crystal,
.noyo-glass-btn--neutral {
  --lg-btn-text: var(--lg-btn-text-secondary);
}

.noyo-glass-btn--success {
  --lg-btn-text: var(--lg-btn-text-success);
}

.noyo-glass-btn--danger {
  --lg-btn-text: var(--lg-btn-text-danger);
}

.noyo-glass-btn--warning {
  --lg-btn-text: var(--color-warning, #d97706);
}

.noyo-glass-btn--info {
  --lg-btn-text: var(--color-info, #0284c7);
}

.noyo-glass-btn--outline-primary {
  --lg-btn-text: var(--lg-btn-text-primary);
  box-shadow: var(--lg-btn-shadow), inset 0 0 0 1px var(--color-brand);
}

.noyo-glass-btn--outline-secondary {
  --lg-btn-text: var(--lg-btn-text-secondary);
  box-shadow: var(--lg-btn-shadow), inset 0 0 0 1px var(--border-color);
}

.noyo-glass-btn--outline-info {
  --lg-btn-text: var(--color-info, #0284c7);
  box-shadow: var(--lg-btn-shadow), inset 0 0 0 1px var(--color-info, #0284c7);
}

/* ── 玻璃体：背景折射采样 + 玻璃 tint / Glass body: backdrop refraction + tint ── */
.noyo-glass-btn__glass {
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: inherit;
  background: var(--lg-btn-glass-tint);
  transition: background 0.3s ease;
}

/* 玻璃内部折射光轴（静态焦散斜带） / internal refraction light axis (static caustic band) */
.noyo-glass-btn__glass::after {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(
    118deg,
    var(--noyo-color-transparent) 30%,
    var(--lg-btn-specular-sheen) 45%,
    var(--noyo-color-transparent) 60%
  );
  opacity: 0.35;
  mix-blend-mode: screen;
}

/* ── 镜面高光与玻璃厚度 / Specular highlight and glass thickness ── */
.noyo-glass-btn__specular {
  position: absolute;
  inset: 0;
  z-index: 1;
  border-radius: inherit;
  pointer-events: none;
  background: linear-gradient(
    180deg,
    var(--lg-btn-specular-top) 0%,
    var(--lg-btn-specular-sheen) 32%,
    var(--noyo-color-transparent) 58%
  );
  box-shadow:
    inset 0 1px 0 var(--lg-btn-rim),
    inset 0 -1px 0 var(--lg-btn-edge-dark),
    inset 0 0 14px var(--lg-btn-rim-soft);
}

/* ── 边缘色散微光（镜头彩虹 rim） / Prismatic edge dispersion ── */
.noyo-glass-btn__prism {
  position: absolute;
  inset: 0;
  z-index: 2;
  border-radius: inherit;
  padding: 1px;
  pointer-events: none;
  background: linear-gradient(
    135deg,
    var(--lg-btn-prism-warm) 0%,
    var(--noyo-color-transparent) 32%,
    var(--lg-btn-prism-cool) 66%,
    var(--noyo-color-transparent) 100%
  );
  -webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
  mask-composite: exclude;
  opacity: 0.5;
  transition: opacity 0.35s ease;
}

.noyo-glass-btn:hover:not(:disabled) .noyo-glass-btn__prism {
  opacity: 0.9;
}

/* ── 动态焦散光斑（指针折射焦点） / Dynamic caustic spotlight ── */
.noyo-glass-btn__caustic {
  position: absolute;
  inset: 0;
  z-index: 3;
  border-radius: inherit;
  pointer-events: none;
  background: radial-gradient(
    120px circle at var(--lg-btn-px) var(--lg-btn-py),
    var(--lg-btn-caustic) 0%,
    var(--lg-btn-caustic-halo) 28%,
    var(--noyo-color-transparent) 62%
  );
  mix-blend-mode: overlay;
  opacity: var(--lg-btn-caustic-opacity);
  transition: opacity 0.3s ease;
}

[data-bs-theme="dark"] .noyo-glass-btn__caustic {
  mix-blend-mode: screen;
}

.noyo-glass-btn:hover:not(:disabled) {
  --lg-btn-caustic-opacity: 1;
}

/* ── 按压波纹 / Press ripple ── */
.noyo-glass-btn__ripple {
  position: absolute;
  z-index: 4;
  left: var(--lg-btn-ripple-x);
  top: var(--lg-btn-ripple-y);
  width: 10px;
  height: 10px;
  margin: -5px 0 0 -5px;
  border-radius: 50%;
  pointer-events: none;
  background: radial-gradient(
    circle,
    var(--lg-btn-caustic) 0%,
    var(--noyo-color-transparent) 70%
  );
  animation: noyo-glass-btn-ripple 0.55s ease-out forwards;
}

@keyframes noyo-glass-btn-ripple {
  from {
    transform: scale(1);
    opacity: 0.9;
  }
  to {
    transform: scale(18);
    opacity: 0;
  }
}

/* ── 内容层 / Content layer ── */
.noyo-glass-btn__content {
  position: relative;
  z-index: 5;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: inherit;
  text-shadow: 0 1px 1px var(--lg-btn-rim-soft);
}

[data-bs-theme="dark"] .noyo-glass-btn__content {
  text-shadow: 0 1px 2px var(--noyo-glass-on-media-shadow);
}

.noyo-glass-btn__icon {
  font-size: 1.1em;
  display: inline-flex;
  align-items: center;
}

/* ── 交互状态：悬浮上浮、按压压缩 / Hover lift, press squeeze ── */
.noyo-glass-btn:hover:not(:disabled) {
  transform:
    translateY(-2px)
    rotateX(var(--lg-btn-tilt-x))
    rotateY(var(--lg-btn-tilt-y))
    scale(1.03);
  box-shadow: var(--lg-btn-shadow-hover);
}

.noyo-glass-btn:hover:not(:disabled) .noyo-glass-btn__glass {
  background: var(--lg-btn-glass-tint-hover);
}

.noyo-glass-btn--pressed,
.noyo-glass-btn--pressed:hover {
  transform: translateY(1px) scale(0.97);
  --lg-btn-caustic-opacity: 1;
  transition-duration: 0.12s;
}

.noyo-glass-btn--pressed .noyo-glass-btn__glass {
  background: var(--lg-btn-glass-tint-active);
}

/* ── 禁用与加载 / Disabled and loading ── */
.noyo-glass-btn:disabled,
.noyo-glass-btn--loading {
  cursor: not-allowed;
  opacity: 0.45;
  transform: none;
}

.noyo-glass-btn:focus-visible {
  outline: 2px solid var(--noyo-focus-ring);
  outline-offset: 3px;
}

/* ── 动效偏好：减少动态 / Reduced motion preference ── */
@media (prefers-reduced-motion: reduce) {
  .noyo-glass-btn {
    transition-duration: 0.01ms;
  }
  .noyo-glass-btn__ripple {
    display: none;
  }
}
</style>
