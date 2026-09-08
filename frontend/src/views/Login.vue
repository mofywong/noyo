<template>
  <div
    ref="pageRoot"
    class="login-page"
    :data-bs-theme="resolvedTheme"
    @pointermove="handlePagePointerMove"
    @pointerleave="handlePagePointerLeave"
  >
    <!-- 背景典雅极光光场层 (Ambient Aurora Field) -->
    <div class="login-ambient" aria-hidden="true">
      <div class="login-ambient__grid"></div>
      <div class="login-ambient__caustics"></div>
      <div class="login-ambient__orb login-ambient__orb--one"></div>
      <div class="login-ambient__orb login-ambient__orb--two"></div>
      <div class="login-ambient__orb login-ambient__orb--three"></div>
      <div class="login-ambient__orb login-ambient__orb--four"></div>
      <!-- 飘动光晕：纯渐变的弥散光团，随机游走 + 呼吸 + 指针感应
           (Drifting halos: diffuse gradient glows with random walk,
           breathing and pointer awareness) -->
      <div class="login-ambient__halo login-ambient__halo--one"><span class="login-ambient__halo-core"></span></div>
      <div class="login-ambient__halo login-ambient__halo--two"><span class="login-ambient__halo-core"></span></div>
      <div class="login-ambient__halo login-ambient__halo--three"><span class="login-ambient__halo-core"></span></div>
      <!-- 暗色模式 = 同一场景蒙加透灰幕，折射目标保持一致
           (Dark mode = the same scene under a translucent dim veil) -->
      <div class="login-ambient__dim"></div>
      <div class="login-ambient__veil"></div>
    </div>

    <!-- 卡片透镜折射滤镜 (Card lens refraction filter, Apple Liquid Glass optics) -->
    <svg
      class="login-card-filters"
      aria-hidden="true"
      focusable="false"
      width="0"
      height="0"
    >
      <defs>
        <filter
          v-if="filterAsset"
          :id="refractionFilterId"
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
            result="login-map"
          />
          <!-- RGB 三通道差异化位移 = 边缘色散 (Per-channel dispersion) -->
          <feDisplacementMap
            in="SourceGraphic"
            in2="login-map"
            :scale="filterAsset.redScale"
            xChannelSelector="R"
            yChannelSelector="G"
            result="login-warp-r"
          />
          <feColorMatrix
            in="login-warp-r"
            type="matrix"
            values="1 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1 0"
            result="login-channel-r"
          />
          <feDisplacementMap
            in="SourceGraphic"
            in2="login-map"
            :scale="filterAsset.greenScale"
            xChannelSelector="R"
            yChannelSelector="G"
            result="login-warp-g"
          />
          <feColorMatrix
            in="login-warp-g"
            type="matrix"
            values="0 0 0 0 0  0 1 0 0 0  0 0 0 0 0  0 0 0 1 0"
            result="login-channel-g"
          />
          <feDisplacementMap
            in="SourceGraphic"
            in2="login-map"
            :scale="filterAsset.blueScale"
            xChannelSelector="R"
            yChannelSelector="G"
            result="login-warp-b"
          />
          <feColorMatrix
            in="login-warp-b"
            type="matrix"
            values="0 0 0 0 0  0 0 0 0 0  0 0 1 0 0  0 0 0 1 0"
            result="login-channel-b"
          />
          <feBlend in="login-channel-r" in2="login-channel-g" mode="screen" result="login-rg" />
          <feBlend in="login-rg" in2="login-channel-b" mode="screen" result="login-rgb" />
          <!-- 折射结果裁剪到卡片轮廓 (Clip to card silhouette) -->
          <feComposite in="login-rgb" in2="login-map" operator="in" result="login-inside" />
          <feComposite in="SourceGraphic" in2="login-map" operator="out" result="login-outside" />
          <feMerge>
            <feMergeNode in="login-outside" />
            <feMergeNode in="login-inside" />
          </feMerge>
        </filter>
      </defs>
    </svg>

    <!-- 右上角主题切换组件 (Theme Switcher) -->
    <div
      class="login-theme-switcher"
      role="group"
      :aria-label="`${$t('theme_light')} / ${$t('theme_dark')} / ${$t('theme_system')}`"
    >
      <button
        type="button"
        class="login-theme-switcher__option"
        :class="{ 'is-active': activeThemeConfig === 'light' }"
        :aria-label="$t('theme_light')"
        :aria-pressed="activeThemeConfig === 'light'"
        :title="$t('theme_light')"
        @click="setLoginTheme('light')"
      >
        <i class="bi bi-sun-fill" aria-hidden="true"></i>
      </button>
      <button
        type="button"
        class="login-theme-switcher__option"
        :class="{ 'is-active': activeThemeConfig === 'dark' }"
        :aria-label="$t('theme_dark')"
        :aria-pressed="activeThemeConfig === 'dark'"
        :title="$t('theme_dark')"
        @click="setLoginTheme('dark')"
      >
        <i class="bi bi-moon-stars-fill" aria-hidden="true"></i>
      </button>
      <button
        type="button"
        class="login-theme-switcher__option"
        :class="{ 'is-active': activeThemeConfig === 'system' }"
        :aria-label="$t('theme_system')"
        :aria-pressed="activeThemeConfig === 'system'"
        :title="$t('theme_system')"
        @click="setLoginTheme('system')"
      >
        <i class="bi bi-circle-half" aria-hidden="true"></i>
      </button>
    </div>

    <!-- 居中液态玻璃登录卡片 -->
    <main class="login-shell">
      <LiquidGlassCard
        ref="cardEl"
        class="login-card"
        :class="{ 'login-card--refractive': refractionReady }"
        variant="surface"
        elevation="floating"
        body-class="login-card__body"
        :custom-style="cardGlassStyle"
        @pointermove="handleCardPointerMove"
        @pointerleave="handleCardPointerLeave"
      >
        <!-- 玻璃流光：缓慢掠过表面的柔和高光带 (Slow sheen sweeping the surface) -->
        <span class="login-card__sheen" aria-hidden="true"></span>
        <!-- 指针跟随焦散高光 (Pointer-tracked caustic glare) -->
        <span
          class="login-card__glare"
          aria-hidden="true"
          :style="cardGlareStyle"
        ></span>
        <!-- Logo 区域 -->
        <div class="logo-section">
          <div v-if="tenantLogo" class="tenant-logo-wrap">
            <div
              v-if="tenantLogo.trim().startsWith('<svg') || tenantLogo.trim().startsWith('<?xml')"
              v-html="DOMPurify.sanitize(tenantLogo, { USE_PROFILES: { svg: true } })"
              class="svg-container"
            ></div>
            <img v-else :src="tenantLogo" alt="Logo" class="tenant-logo-img">
          </div>
          <div v-else class="noyo-logo-wrap">
            <img :src="appBrand.logoUrl" :alt="`${brandNameForLocale(locale)} Logo`" class="noyo-logo-img">
          </div>
          <h1 class="brand-title">{{ tenantName || brandNameForLocale(locale) }}</h1>
          <p class="brand-subtitle">{{ $t('auth_login_subtitle') }}</p>
        </div>

        <div class="divider"></div>

        <!-- 错误提示 -->
        <div v-if="errorMsg" class="error-alert" role="alert">
          <i class="bi bi-exclamation-triangle-fill" aria-hidden="true"></i>
          <span>{{ errorMsg }}</span>
        </div>

        <!-- 登录表单 -->
        <form @submit.prevent="handleLogin" class="login-form">
          <div class="input-group-custom">
            <label class="input-label" for="login-username">{{ $t('auth_username') }}</label>
            <div class="input-wrap">
              <i class="bi bi-person input-icon" aria-hidden="true"></i>
              <input
                id="login-username"
                v-model="username"
                type="text"
                class="input-field"
                :placeholder="$t('auth_username_placeholder')"
                required
                autofocus
                autocomplete="username"
              >
            </div>
          </div>

          <div class="input-group-custom">
            <label class="input-label" for="login-password">{{ $t('auth_password') }}</label>
            <div class="input-wrap">
              <i class="bi bi-lock input-icon" aria-hidden="true"></i>
              <input
                id="login-password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                class="input-field"
                :placeholder="$t('auth_password_placeholder')"
                required
                autocomplete="current-password"
              >
              <button
                type="button"
                class="toggle-password"
                :aria-pressed="showPassword"
                :aria-label="$t('auth_password')"
                @click="showPassword = !showPassword"
              >
                <i :class="showPassword ? 'bi bi-eye-slash' : 'bi bi-eye'" aria-hidden="true"></i>
              </button>
            </div>
          </div>

          <button
            id="login-submit"
            type="submit"
            class="submit-btn"
            :disabled="loading"
          >
            <span v-if="loading" class="spinner" aria-hidden="true"></span>
            <span>{{ loading ? $t('loading', 'Loading...') : $t('auth_sign_in') }}</span>
          </button>
        </form>

        <div class="card-footer-tag">
          <span class="aiot-badge">AIoT Platform</span>
        </div>
      </LiquidGlassCard>
    </main>

    <!-- 底部版权 -->
    <footer class="page-footer">
      <span>© {{ new Date().getFullYear() }} {{ appBrand.footerText }}</span>
    </footer>
  </div>
</template>

<script setup>
import { ref, computed, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import DOMPurify from 'dompurify'
import { useAuthStore } from '../stores/auth'
import { useI18n } from 'vue-i18n'
import axios from 'axios'
import { appBrand, brandNameForLocale } from '../config/brand.js'
import LiquidGlassCard from '../components/liquid-glass/LiquidGlassCard.vue'
import {
  computeRoundedRectDisplacementMap,
  computeSvgFilterRegion,
  encodeDisplacementMapDataUrl,
  supportsBackdropSvgFilter,
} from '../utils/liquidGlassRefraction.js'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t, locale } = useI18n()

const username = ref('')
const password = ref('')
const showPassword = ref(false)
const errorMsg = ref('')
const loading = ref(false)
const tenantName = ref('')
const tenantLogo = ref('')

// 视口微视差 (Pointer micro-parallax)：仅精指针设备启用，尊重系统减弱动效偏好
const pageRoot = ref(null)
let parallaxEnabled = false
let parallaxRafId = null
const parallaxTarget = { x: 0, y: 0 }
const parallaxCurrent = { x: 0, y: 0 }

const supportsLoginParallax = () => (
  typeof window !== 'undefined'
  && window.matchMedia('(hover: hover) and (pointer: fine)').matches
  && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
)

const stepLoginParallax = () => {
  const root = pageRoot.value
  if (!root) {
    parallaxRafId = null
    return
  }
  const dx = parallaxTarget.x - parallaxCurrent.x
  const dy = parallaxTarget.y - parallaxCurrent.y
  parallaxCurrent.x += dx * 0.08
  parallaxCurrent.y += dy * 0.08
  root.style.setProperty('--login-parallax-x', `${(parallaxCurrent.x * 14).toFixed(2)}px`)
  root.style.setProperty('--login-parallax-y', `${(parallaxCurrent.y * 10).toFixed(2)}px`)
  if (Math.abs(dx) + Math.abs(dy) > 0.001) {
    parallaxRafId = requestAnimationFrame(stepLoginParallax)
  } else {
    parallaxRafId = null
  }
}

const queueLoginParallax = () => {
  if (parallaxRafId === null) {
    parallaxRafId = requestAnimationFrame(stepLoginParallax)
  }
}

const handlePagePointerMove = (event) => {
  updateHaloAttraction(event)
  if (!parallaxEnabled) return
  const width = window.innerWidth || 1
  const height = window.innerHeight || 1
  parallaxTarget.x = (event.clientX / width) - 0.5
  parallaxTarget.y = (event.clientY / height) - 0.5
  queueLoginParallax()
}

const handlePagePointerLeave = () => {
  releaseHalos()
  if (!parallaxEnabled) return
  parallaxTarget.x = 0
  parallaxTarget.y = 0
  queueLoginParallax()
}

// —— 光晕随机游走 + 呼吸 + 指针感应 ——
// 外层元素由 JS 驱动位移（连续随机游走 / 吸引跟随 / 摆动悬停），
// 内层核心承载恒定 CSS 呼吸动画，两者互不干扰。
let haloTimers = []
let haloWanderFns = []
let haloStates = []
let haloDriftEnabled = false
let haloSwayRaf = null

const HALO_PULL = 0.42
const HALO_MAX_PULL_X = 170
const HALO_MAX_PULL_Y = 130
const clampHalo = (value, min, max) => Math.max(min, Math.min(max, value))

const applyHaloTransform = (el, state) => {
  el.style.transform = `translate3d(${state.x.toFixed(0)}px, ${state.y.toFixed(0)}px, 0) scale(${state.scale.toFixed(2)})`
}

const startHaloDrift = () => {
  const root = pageRoot.value
  if (!root) return
  const halos = root.querySelectorAll('.login-ambient__halo')
  halos.forEach((el, index) => {
    haloStates[index] = { x: 0, y: 0, scale: 1 }
    const wander = () => {
      const st = haloStates[index]
      // 连续随机游走：以当前位置累加随机位移并夹紧边界；
      // 节奏短于过渡时长，过渡中不断重定目标，几乎从不停歇。
      st.x = clampHalo(st.x + (Math.random() * 2 - 1) * 130, -260, 260)
      st.y = clampHalo(st.y + (Math.random() * 2 - 1) * 100, -200, 200)
      st.scale = 0.88 + Math.random() * 0.34
      applyHaloTransform(el, st)
      haloTimers[index] = window.setTimeout(wander, 3200 + Math.random() * 3000)
    }
    haloWanderFns[index] = wander
    haloTimers[index] = window.setTimeout(wander, index * 900 + Math.random() * 1100)
  })
}

const stopHaloDrift = () => {
  haloTimers.forEach((timer) => window.clearTimeout(timer))
  haloTimers = []
  haloWanderFns = []
  if (haloSwayRaf !== null) {
    cancelAnimationFrame(haloSwayRaf)
    haloSwayRaf = null
  }
}

const resumeHaloWander = (index) => {
  haloTimers[index] = window.setTimeout(() => haloWanderFns[index]?.(), 700 + Math.random() * 1100)
}

// 点亮态悬停：rAF 驱动绕指针目标的轻柔正弦摆动，像悬停的萤火虫
const applyHaloSway = () => {
  const root = pageRoot.value
  if (!root) return
  const now = performance.now()
  let anyLit = false
  root.querySelectorAll('.login-ambient__halo').forEach((el, index) => {
    if (!el.classList.contains('is-lit')) return
    anyLit = true
    const st = haloStates[index]
    if (!st) return
    const swayX = Math.sin(now / 640 + index * 2.1) * 12
    const swayY = Math.cos(now / 810 + index * 1.7) * 9
    el.style.transform = `translate3d(${(st.x + swayX).toFixed(1)}px, ${(st.y + swayY).toFixed(1)}px, 0) scale(${st.scale.toFixed(2)})`
  })
  haloSwayRaf = anyLit ? requestAnimationFrame(applyHaloSway) : null
}

const ensureHaloSway = () => {
  if (haloSwayRaf === null) haloSwayRaf = requestAnimationFrame(applyHaloSway)
}

// 指针感应：进入范围点亮 + 吸引跟随；超出 1.35 倍范围（迟滞）熄灭并恢复漫游
const updateHaloAttraction = (event) => {
  const root = pageRoot.value
  if (!root || !haloDriftEnabled || haloWanderFns.length === 0) return
  const halos = root.querySelectorAll('.login-ambient__halo')
  const reads = []
  halos.forEach((el) => reads.push(el.getBoundingClientRect()))
  halos.forEach((el, index) => {
    const rect = reads[index]
    if (rect.width <= 0) return
    const dx = event.clientX - (rect.left + rect.width / 2)
    const dy = event.clientY - (rect.top + rect.height / 2)
    const dist = Math.hypot(dx, dy)
    const radius = rect.width * 0.6
    const lit = el.classList.contains('is-lit')
    if (!lit && dist <= radius) {
      el.classList.add('is-lit')
      el.style.transition = 'transform 1.4s cubic-bezier(0.22, 1, 0.36, 1), filter 900ms ease'
      if (haloTimers[index]) {
        window.clearTimeout(haloTimers[index])
        haloTimers[index] = null
      }
    } else if (lit && dist > radius * 1.35) {
      el.classList.remove('is-lit')
      el.style.transition = 'transform 7s ease-in-out, filter 900ms ease'
      resumeHaloWander(index)
      return
    }
    if (!el.classList.contains('is-lit')) return
    const st = haloStates[index]
    if (!st) return
    st.x = clampHalo(dx * HALO_PULL, -HALO_MAX_PULL_X, HALO_MAX_PULL_X)
    st.y = clampHalo(dy * HALO_PULL, -HALO_MAX_PULL_Y, HALO_MAX_PULL_Y)
    const proximity = Math.max(0, 1 - dist / (radius * 1.35))
    st.scale = 1.06 + proximity * 0.22
    ensureHaloSway()
  })
}

const releaseHalos = () => {
  const root = pageRoot.value
  if (!root) return
  root.querySelectorAll('.login-ambient__halo').forEach((el, index) => {
    if (el.classList.contains('is-lit')) {
      el.classList.remove('is-lit')
      el.style.transition = 'transform 7s ease-in-out, filter 900ms ease'
      resumeHaloWander(index)
    }
  })
}

// —— 卡片透镜折射管线 (Card lens refraction pipeline) ——
// 与 LiquidGlassButton 同源的 Apple Liquid Glass 光学实现：
// SDF 圆角矩形位移图 → RGB 三通道差异化位移（色散）→ 裁剪到卡片轮廓。
// 不支持 backdrop-filter + SVG url() 组合时降级为纯模糊玻璃。
const refractionFilterId = 'login-card-refraction'
const CARD_RADIUS = 28
const DISPERSION = 0.22
const cardEl = ref(null)
const refractionReady = ref(false)
const filterAsset = ref(null)

const buildLoginFilterAsset = (width, height) => {
  const w = Math.round(width)
  const h = Math.round(height)
  if (w < 64 || h < 64) return null
  const shortSide = Math.min(w, h)
  const depth = Math.max(18, Math.min(48, shortSide * 0.11))
  const strength = Math.max(12, Math.min(20, shortSide * 0.055))
  const map = computeRoundedRectDisplacementMap({
    width: w,
    height: h,
    radius: CARD_RADIUS,
    depth,
    curvature: 0.30,
    bend: 0.95,
    bendWidth: 0.13,
    specularAngle: 315,
    strength,
  })
  return {
    mapUrl: encodeDisplacementMapDataUrl(map, document),
    region: computeSvgFilterRegion({ width: w, height: h, strength, blur: 0 }),
    redScale: Number((strength * (1 + DISPERSION)).toFixed(2)),
    greenScale: Number(strength.toFixed(2)),
    blueScale: Number((strength * (1 - DISPERSION)).toFixed(2)),
  }
}

let cardResizeObserver = null
let cardRebuildTimer = null

const rebuildCardFilterAsset = () => {
  const component = cardEl.value
  const el = component?.$el || component
  if (!el || !refractionReady.value) return
  try {
    const rect = el.getBoundingClientRect()
    if (rect.width > 0 && rect.height > 0) {
      filterAsset.value = buildLoginFilterAsset(rect.width, rect.height)
    }
  } catch (error) {
    console.warn('Login card refraction disabled:', error)
    refractionReady.value = false
    filterAsset.value = null
  }
}

const scheduleCardRebuild = () => {
  if (cardRebuildTimer !== null) window.clearTimeout(cardRebuildTimer)
  cardRebuildTimer = window.setTimeout(() => {
    cardRebuildTimer = null
    rebuildCardFilterAsset()
  }, 150)
}

// 折射生效时的 backdrop-filter：先位移锐利背景，再做轻度模糊（通透中心 + 边缘透镜）。
const cardGlassStyle = computed(() => {
  if (!refractionReady.value || !filterAsset.value) return {}
  return {
    '--login-card-backdrop-filter': `url(#${refractionFilterId}) blur(var(--login-card-blur)) saturate(var(--login-card-saturate)) brightness(var(--login-card-brightness))`,
  }
})

// —— 指针跟随焦散高光 (Pointer-tracked caustic glare) ——
const glareX = ref(50)
const glareY = ref(32)
const cardGlareStyle = computed(() => ({
  '--login-glare-x': `${glareX.value.toFixed(1)}%`,
  '--login-glare-y': `${glareY.value.toFixed(1)}%`,
}))

const handleCardPointerMove = (event) => {
  const component = cardEl.value
  const el = component?.$el || component
  if (!el || !parallaxEnabled) return
  const rect = el.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) return
  glareX.value = Math.max(0, Math.min(100, ((event.clientX - rect.left) / rect.width) * 100))
  glareY.value = Math.max(0, Math.min(100, ((event.clientY - rect.top) / rect.height) * 100))
}

const handleCardPointerLeave = () => {
  glareX.value = 50
  glareY.value = 32
}

const activeThemeConfig = ref(localStorage.getItem('theme') || 'dark')
const systemPrefersDark = ref(typeof window !== 'undefined' ? window.matchMedia('(prefers-color-scheme: dark)').matches : false)

const resolvedTheme = computed(() => {
  if (activeThemeConfig.value === 'system') {
    return systemPrefersDark.value ? 'dark' : 'light'
  }
  return activeThemeConfig.value
})

const applyThemeToDom = (theme) => {
  if (typeof document === 'undefined') return
  document.documentElement.setAttribute('data-bs-theme', theme)
  if (document.body) {
    document.body.setAttribute('data-bs-theme', theme)
  }
}

const setLoginTheme = (theme) => {
  activeThemeConfig.value = theme
  localStorage.setItem('theme', theme)
  const targetTheme = theme === 'system' ? (systemPrefersDark.value ? 'dark' : 'light') : theme
  applyThemeToDom(targetTheme)
  window.dispatchEvent(new CustomEvent('noyo-theme-changed', { detail: { theme } }))
}

let mediaQuery = null
const handleMediaChange = (e) => {
  systemPrefersDark.value = e.matches
  if (activeThemeConfig.value === 'system') {
    applyThemeToDom(resolvedTheme.value)
  }
}

const handleGlobalThemeChange = (event) => {
  const theme = event?.detail?.theme
  if (!['light', 'dark', 'system'].includes(theme)) return
  activeThemeConfig.value = theme
  applyThemeToDom(resolvedTheme.value)
}

onMounted(async () => {
  if (typeof window !== 'undefined') {
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', handleMediaChange)
    window.addEventListener('noyo-theme-changed', handleGlobalThemeChange)
  }
  applyThemeToDom(resolvedTheme.value)
  parallaxEnabled = supportsLoginParallax()
  haloDriftEnabled = typeof window !== 'undefined'
    && !window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (haloDriftEnabled) {
    nextTick(startHaloDrift)
  }
  try {
    if (supportsBackdropSvgFilter(window.CSS, window, document, navigator)) {
      refractionReady.value = true
      nextTick(rebuildCardFilterAsset)
    }
  } catch (error) {
    console.warn('Login card refraction probe failed:', error)
  }
  const cardComponent = cardEl.value
  const cardHost = cardComponent?.$el || cardComponent
  if (cardHost && typeof ResizeObserver !== 'undefined') {
    cardResizeObserver = new ResizeObserver(scheduleCardRebuild)
    cardResizeObserver.observe(cardHost)
  }

  if (!route.params.suffix) return

  try {
    const res = await axios.get('/api/auth/tenant-info', { params: { suffix: route.params.suffix } })
    if (res.data.code === 0 && res.data.data) {
      tenantName.value = res.data.data.name
      tenantLogo.value = res.data.data.logo
    }
  } catch (error) {
    console.warn('Failed to fetch tenant info for suffix:', route.params.suffix)
  }
})

onBeforeUnmount(() => {
  parallaxEnabled = false
  haloDriftEnabled = false
  stopHaloDrift()
  if (parallaxRafId !== null) {
    cancelAnimationFrame(parallaxRafId)
    parallaxRafId = null
  }
  if (cardRebuildTimer !== null) {
    window.clearTimeout(cardRebuildTimer)
    cardRebuildTimer = null
  }
  if (cardResizeObserver) {
    cardResizeObserver.disconnect()
    cardResizeObserver = null
  }
  if (typeof window !== 'undefined') {
    if (mediaQuery) {
      mediaQuery.removeEventListener('change', handleMediaChange)
    }
    window.removeEventListener('noyo-theme-changed', handleGlobalThemeChange)
  }
})

const handleLogin = async () => {
  if (!username.value || !password.value) return

  errorMsg.value = ''
  loading.value = true

  try {
    const res = await authStore.login(username.value, password.value, route.params.suffix || '')
    if (res.code === 0) {
      router.push('/')
    } else {
      errorMsg.value = res.message || t('auth_login_failed')
    }
  } catch (err) {
    if (err.response && err.response.data && err.response.data.message) {
      errorMsg.value = err.response.data.message
    } else {
      errorMsg.value = t('auth_network_error')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
/* ============================================================
   1. 页面级设计令牌 · 亮色 (Page-scoped Design Tokens · Light)
   登录页专属视觉色值仅允许定义在本令牌区内，
   其余规则一律通过 CSS 变量引用；文本/品牌色复用全局语义令牌。
   Login-specific raw color values live only inside the marked
   token blocks below; all other rules consume these variables,
   and shared text/brand colors reuse the global semantic tokens.
   ============================================================ */
.login-page {
/* <login-design-tokens> */
  /* 环境光场 (Ambient aurora field) */
  --login-ambient-opacity: 0.88;
  --login-ambient-blur: 54px;
  --login-page-bg:
    radial-gradient(circle at 10% 12%, rgba(59, 130, 246, 0.13) 0%, transparent 45%),
    radial-gradient(circle at 90% 18%, rgba(99, 102, 241, 0.10) 0%, transparent 45%),
    radial-gradient(circle at 85% 85%, rgba(14, 165, 233, 0.10) 0%, transparent 45%),
    radial-gradient(circle at 15% 88%, rgba(45, 212, 191, 0.08) 0%, transparent 40%),
    linear-gradient(180deg, #f4f6fc 0%, #edf3fb 50%, #f1f5f9 100%);
  --login-grid-dot: rgba(69, 91, 122, 0.16);
  --login-grid-opacity: 0.40;
  --login-grid-mask: radial-gradient(circle at 50% 50%, rgba(0, 0, 0, 0.9) 30%, rgba(0, 0, 0, 0.1) 75%);
  --login-caustics-a: rgba(14, 165, 233, 0.14);
  --login-caustics-b: rgba(99, 102, 241, 0.12);
  --login-caustics-opacity: 0.55;
  --login-orb-one: radial-gradient(circle, rgba(59, 130, 246, 0.26) 0%, rgba(147, 197, 253, 0.12) 50%, transparent 72%);
  --login-orb-two: radial-gradient(circle, rgba(6, 182, 212, 0.22) 0%, rgba(103, 232, 249, 0.10) 50%, transparent 70%);
  --login-orb-three: radial-gradient(circle, rgba(99, 102, 241, 0.18) 0%, rgba(165, 180, 252, 0.08) 50%, transparent 72%);
  --login-orb-four: radial-gradient(circle, rgba(255, 255, 255, 0.55) 0%, rgba(219, 234, 254, 0.24) 45%, transparent 70%);
  --login-veil: linear-gradient(112deg, transparent 16%, rgba(255, 255, 255, 0.30) 49%, transparent 82%);
  --login-veil-opacity: 0.45;

  /* 主题切换胶囊 (Theme switcher capsule) */
  --login-switcher-border: rgba(255, 255, 255, 0.60);
  --login-switcher-bg: rgba(255, 255, 255, 0.65);
  --login-switcher-shadow: 0 8px 24px rgba(15, 23, 42, 0.08), inset 0 1px 0 rgba(255, 255, 255, 0.80);
  --login-option-color: #475569;
  --login-option-active-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.40);

  /* 液态玻璃卡片 (Liquid glass card) */
  --login-card-border: rgba(255, 255, 255, 0.75);
  --login-card-bg: rgba(255, 255, 255, 0.38);
  --login-card-blur: 12px;
  --login-card-saturate: 168%;
  --login-card-brightness: 1.08;
  --login-card-fallback-blur: 28px;
  --login-card-shadow:
    0 30px 60px -12px rgba(30, 41, 59, 0.18),
    0 12px 28px -8px rgba(30, 41, 59, 0.10),
    inset 0 1px 1px rgba(255, 255, 255, 0.92),
    inset 0 -1px 1px rgba(255, 255, 255, 0.38),
    inset 0 0 26px -10px rgba(255, 255, 255, 0.60),
    inset 0 0 44px -20px rgba(59, 130, 246, 0.22);
  --login-card-specular: linear-gradient(90deg, transparent 10%, rgba(255, 255, 255, 0.95) 46%, rgba(255, 255, 255, 0.72) 54%, transparent 90%);
  --login-card-specular-opacity: 0.95;
  --login-card-bloom: radial-gradient(120% 90% at 18% 0%, rgba(59, 130, 246, 0.10), transparent 42%);
  --login-card-bloom-opacity: 0.45;
  --login-card-hover-border: rgba(59, 130, 246, 0.45);
  --login-card-hover-shadow:
    0 34px 70px -12px rgba(30, 41, 59, 0.20),
    0 18px 44px rgba(59, 130, 246, 0.22),
    inset 0 1px 1px rgba(255, 255, 255, 0.95),
    inset 0 -1px 1px rgba(255, 255, 255, 0.45),
    inset 0 0 30px -10px rgba(255, 255, 255, 0.70),
    inset 0 0 52px -20px rgba(59, 130, 246, 0.28);
  --login-card-fallback-bg: #ffffff;

  /* 飘动光晕：纯渐变弥散光 (Drifting halos: pure gradient glows) */
  --login-halo-one: radial-gradient(circle, rgba(147, 197, 253, 0.48) 0%, rgba(96, 165, 250, 0.20) 38%, transparent 68%);
  --login-halo-two: radial-gradient(circle, rgba(165, 180, 252, 0.42) 0%, rgba(129, 140, 248, 0.16) 38%, transparent 68%);
  --login-halo-three: radial-gradient(circle, rgba(103, 232, 249, 0.40) 0%, rgba(34, 211, 238, 0.15) 38%, transparent 68%);
  --login-dim-color: rgba(7, 10, 18, 1);
  --login-dim-opacity: 0;
  --login-sheen-color: rgba(255, 255, 255, 0.35);
  --login-sheen-opacity: 0.50;
  --login-glare-color: rgba(255, 255, 255, 0.55);

  /* 表单控件 (Form controls) */
  --login-input-border: rgba(255, 255, 255, 0.85);
  --login-input-bg: rgba(255, 255, 255, 0.55);
  --login-input-placeholder: #94a3b8;
  --login-input-focus-bg: #ffffff;
  --login-divider: linear-gradient(90deg, transparent, rgba(15, 23, 42, 0.08), transparent);

  /* 错误提示 (Error alert) */
  --login-error-border: rgba(239, 68, 68, 0.35);
  --login-error-bg: rgba(239, 68, 68, 0.12);
  --login-error-color: #dc2626;

  /* 提交按钮 (Submit button) */
  --login-btn-border: #1d4ed8;
  --login-btn-bg: linear-gradient(135deg, #0284c7 0%, #2563eb 50%, #1d4ed8 100%);
  --login-btn-shadow: 0 10px 24px -4px rgba(37, 99, 235, 0.45), inset 0 1px 1px rgba(255, 255, 255, 0.40);
  --login-btn-hover-bg: linear-gradient(135deg, #0369a1 0%, #1d4ed8 50%, #1e40af 100%);
  --login-btn-hover-shadow: 0 14px 28px -4px rgba(37, 99, 235, 0.60), inset 0 1px 1px rgba(255, 255, 255, 0.60);
  --login-btn-text: #ffffff;
  --login-spinner-track: rgba(255, 255, 255, 0.36);
  --login-spinner-fill: #ffffff;

  /* 徽标与页脚 (Badge and footer) */
  --login-badge-border: rgba(37, 99, 235, 0.28);
  --login-badge-bg: rgba(37, 99, 235, 0.08);
  --login-badge-color: #1d4ed8;
  --login-logo-filter: drop-shadow(0 8px 14px rgba(59, 130, 246, 0.28));
/* </login-design-tokens> */
  position: relative;
  isolation: isolate;
  display: grid;
  min-height: 100dvh;
  width: 100%;
  overflow: hidden;
  place-items: center;
  padding: 24px;
  background: var(--login-page-bg);
  color: var(--text-main);
  font-family: var(--font-sans);
}

/* ============================================================
   2. 设计令牌 · 暗黑 (Design Tokens · Dark)
   场景与亮色完全一致，仅蒙加透灰幕（Apple 同场景材质自适应模式）；
   玻璃、控件等"材质"令牌按暗色重新取值。
   ============================================================ */
.login-page[data-bs-theme='dark'] {
/* <login-design-tokens> */
  --login-dim-opacity: 0.58;

  --login-switcher-border: rgba(255, 255, 255, 0.16);
  --login-switcher-bg: rgba(18, 24, 38, 0.65);
  --login-switcher-shadow: 0 8px 24px rgba(0, 0, 0, 0.40), inset 0 1px 0 rgba(255, 255, 255, 0.20);
  --login-option-color: #94a3b8;
  --login-option-active-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.25);

  --login-card-border: rgba(255, 255, 255, 0.22);
  --login-card-bg: rgba(14, 21, 35, 0.36);
  --login-card-blur: 14px;
  --login-card-saturate: 180%;
  --login-card-brightness: 1.12;
  --login-card-fallback-blur: 28px;
  --login-card-shadow:
    0 34px 70px -14px rgba(0, 0, 0, 0.90),
    0 0 44px -6px rgba(59, 130, 246, 0.28),
    inset 0 1px 1px rgba(255, 255, 255, 0.38),
    inset 0 -1px 1px rgba(255, 255, 255, 0.14),
    inset 0 0 30px -12px rgba(147, 197, 253, 0.22),
    inset 0 0 52px -22px rgba(59, 130, 246, 0.30);
  --login-card-specular: linear-gradient(90deg, transparent 10%, rgba(255, 255, 255, 0.55) 46%, rgba(255, 255, 255, 0.40) 54%, transparent 90%);
  --login-card-specular-opacity: 0.80;
  --login-card-bloom: radial-gradient(120% 90% at 18% 0%, rgba(37, 99, 235, 0.16), transparent 42%);
  --login-card-bloom-opacity: 0.55;
  --login-card-hover-border: rgba(96, 165, 250, 0.50);
  --login-card-hover-shadow:
    0 38px 76px -14px rgba(0, 0, 0, 0.92),
    0 0 56px rgba(59, 130, 246, 0.38),
    inset 0 1px 1px rgba(255, 255, 255, 0.45),
    inset 0 -1px 1px rgba(255, 255, 255, 0.18),
    inset 0 0 36px -12px rgba(147, 197, 253, 0.28),
    inset 0 0 60px -22px rgba(59, 130, 246, 0.38);
  --login-card-fallback-bg: #0f172a;

  --login-sheen-color: rgba(147, 197, 253, 0.20);
  --login-sheen-opacity: 0.70;
  --login-glare-color: rgba(147, 197, 253, 0.35);

  --login-input-border: rgba(255, 255, 255, 0.20);
  --login-input-bg: rgba(255, 255, 255, 0.08);
  --login-input-placeholder: #64748b;
  --login-input-focus-bg: rgba(255, 255, 255, 0.12);
  --login-divider: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.16), transparent);

  --login-error-border: rgba(239, 68, 68, 0.45);
  --login-error-bg: rgba(239, 68, 68, 0.18);
  --login-error-color: #f87171;

  --login-btn-border: #2563eb;
  --login-btn-bg: linear-gradient(135deg, #0ea5e9 0%, #2563eb 55%, #1d4ed8 100%);
  --login-btn-shadow: 0 10px 26px -4px rgba(59, 130, 246, 0.50), inset 0 1px 1px rgba(255, 255, 255, 0.35);
  --login-btn-hover-bg: linear-gradient(135deg, #38bdf8 0%, #3b82f6 55%, #2563eb 100%);
  --login-btn-hover-shadow: 0 14px 30px -4px rgba(59, 130, 246, 0.65), inset 0 1px 1px rgba(255, 255, 255, 0.55);
  --login-btn-text: #ffffff;
  --login-spinner-track: rgba(255, 255, 255, 0.36);
  --login-spinner-fill: #ffffff;

  --login-badge-border: rgba(96, 165, 250, 0.35);
  --login-badge-bg: rgba(96, 165, 250, 0.14);
  --login-badge-color: #60a5fa;
  --login-logo-filter: brightness(1.3) saturate(1.15) drop-shadow(0 8px 16px rgba(59, 130, 246, 0.45));
/* </login-design-tokens> */
}

/* ============================================================
   3. 右上角三态主题切换器 (Theme Switcher)
   ============================================================ */
.login-theme-switcher {
  position: absolute;
  z-index: 2;
  top: 24px;
  right: 24px;
  display: inline-flex;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--login-switcher-border);
  border-radius: var(--radius-pill, 9999px);
  background: var(--login-switcher-bg);
  box-shadow: var(--login-switcher-shadow);
  backdrop-filter: blur(20px) saturate(132%);
  -webkit-backdrop-filter: blur(20px) saturate(132%);
}

.login-theme-switcher__option {
  display: inline-grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--login-option-color);
  cursor: pointer;
  transition: background 160ms ease, color 160ms ease, box-shadow 160ms ease;
}

.login-theme-switcher__option:hover,
.login-theme-switcher__option:focus-visible,
.login-theme-switcher__option.is-active {
  background: var(--color-brand-subtle);
  color: var(--color-brand-hover);
}

.login-page[data-bs-theme='dark'] .login-theme-switcher__option:hover,
.login-page[data-bs-theme='dark'] .login-theme-switcher__option:focus-visible,
.login-page[data-bs-theme='dark'] .login-theme-switcher__option.is-active {
  color: var(--color-brand);
}

.login-theme-switcher__option.is-active {
  box-shadow: var(--login-option-active-shadow);
}

.login-theme-switcher__option:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}

/* ============================================================
   4. 极光流体环境光场 (Ambient Aurora Field)
   ============================================================ */
.login-ambient,
.login-ambient__grid,
.login-ambient__caustics,
.login-ambient__orb,
.login-ambient__veil,
.login-ambient__dim {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.login-ambient {
  z-index: 0;
  overflow: hidden;
  transform: translate3d(var(--login-parallax-x, 0px), var(--login-parallax-y, 0px), 0);
  will-change: transform;
}

/* 微点阵微经纬网格 */
.login-ambient__grid {
  background-image: radial-gradient(var(--login-grid-dot) 1px, transparent 1px);
  background-size: 28px 28px;
  opacity: var(--login-grid-opacity);
  mask-image: var(--login-grid-mask);
  -webkit-mask-image: var(--login-grid-mask);
}

/* 光学焦散纹理 */
.login-ambient__caustics {
  background:
    radial-gradient(ellipse at 42% 38%, var(--login-caustics-a), transparent 55%),
    radial-gradient(ellipse at 58% 62%, var(--login-caustics-b), transparent 50%);
  filter: blur(48px);
  opacity: var(--login-caustics-opacity);
  transform: scale(1.1);
  animation: login-liquid-caustics 20s ease infinite alternate;
  will-change: transform;
}

.login-ambient__orb {
  inset: auto;
  width: min(56vw, 760px);
  aspect-ratio: 1;
  border-radius: 50%;
  opacity: var(--login-ambient-opacity);
  filter: blur(var(--login-ambient-blur)) saturate(160%);
  will-change: transform;
}

/* Orb 1: 电光蓝 */
.login-ambient__orb--one {
  top: -24vmin;
  left: -14vmin;
  background: var(--login-orb-one);
  animation: login-liquid-one 18s ease infinite alternate;
}

/* Orb 2: 极光青 */
.login-ambient__orb--two {
  right: -22vmin;
  bottom: -32vmin;
  width: min(60vw, 800px);
  background: var(--login-orb-two);
  animation: login-liquid-two 24s ease infinite alternate;
}

/* Orb 3: 幽邃紫 */
.login-ambient__orb--three {
  top: 20%;
  left: 30%;
  width: min(44vw, 580px);
  background: var(--login-orb-three);
  animation: login-liquid-three 28s ease infinite alternate;
}

/* Orb 4: 中心背光核心 */
.login-ambient__orb--four {
  top: 36%;
  left: 46%;
  width: min(34vw, 440px);
  background: var(--login-orb-four);
  animation: login-liquid-four 32s ease infinite alternate;
}

.login-ambient__veil {
  background: var(--login-veil);
  opacity: var(--login-veil-opacity);
  transform: translateX(-18%);
  animation: login-liquid-veil 22s ease infinite alternate;
  will-change: transform;
}

/* 飘动光晕：外层承载位移（JS 随机游走/吸引），内层承载恒定呼吸，
   两者互不干扰；点亮时呼吸加快，像被唤醒的萤火虫。 */
.login-ambient__halo {
  position: absolute;
  inset: auto;
  transition: transform 7s ease-in-out;
  will-change: transform;
}

.login-ambient__halo--one {
  width: 460px;
  height: 460px;
  left: calc(50% - 520px);
  top: 14%;
}

.login-ambient__halo--two {
  width: 360px;
  height: 360px;
  right: 5%;
  top: 9%;
}

.login-ambient__halo--three {
  width: 430px;
  height: 430px;
  left: calc(50% + 240px);
  bottom: 11%;
}

.login-ambient__halo-core {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  filter: blur(12px);
  animation: login-halo-breathe 4.6s ease-in-out infinite alternate;
}

.login-ambient__halo--one .login-ambient__halo-core {
  background: var(--login-halo-one);
  animation-delay: -1.2s;
}

.login-ambient__halo--two .login-ambient__halo-core {
  background: var(--login-halo-two);
  animation-delay: -2.6s;
}

.login-ambient__halo--three .login-ambient__halo-core {
  background: var(--login-halo-three);
  animation-delay: -3.8s;
}

/* 指针点亮：萤火虫被唤醒，增亮提饱和并加快呼吸 */
.login-ambient__halo.is-lit .login-ambient__halo-core {
  filter: blur(9px) brightness(1.5) saturate(1.3);
  animation-duration: 2.6s;
}

@keyframes login-halo-breathe {
  from { transform: scale(0.94); opacity: 0.82; }
  to { transform: scale(1.08); opacity: 1; }
}

/* 暗色透灰幕：同一场景之上蒙灰，折射目标与亮色完全一致
   (Dark dim veil: the same scene under a translucent gray layer) */
.login-ambient__dim {
  background: var(--login-dim-color);
  opacity: var(--login-dim-opacity);
  transition: opacity 400ms ease;
}

/* ============================================================
   5. 登录卡片主体 (Liquid Glass Card)
   ============================================================ */
.login-shell {
  position: relative;
  z-index: 1;
  width: min(100%, 432px);
  transform: translate3d(
    calc(var(--login-parallax-x, 0px) * -0.35),
    calc(var(--login-parallax-y, 0px) * -0.35),
    0
  );
}

.login-card {
  width: 100%;
  overflow: hidden;
  border: 1px solid var(--login-card-border);
  border-radius: 28px;
  background: var(--login-card-bg);
  box-shadow: var(--login-card-shadow);
  /* 默认降级为磨砂玻璃；折射就绪时由 JS 注入
     "url(#透镜位移) → 轻模糊" 的真液态玻璃管线。 */
  backdrop-filter: var(--login-card-backdrop-filter, blur(var(--login-card-fallback-blur)) saturate(var(--login-card-saturate)) brightness(var(--login-card-brightness)));
  -webkit-backdrop-filter: var(--login-card-backdrop-filter, blur(var(--login-card-fallback-blur)) saturate(var(--login-card-saturate)) brightness(var(--login-card-brightness)));
  transform-origin: center;
  transition: transform 200ms ease, border-color 200ms ease, box-shadow 200ms ease;
  animation: login-card-enter 480ms cubic-bezier(0.16, 1, 0.3, 1) backwards;
}

@keyframes login-card-enter {
  from {
    opacity: 0;
    transform: translateY(14px) scale(0.985);
  }
}

/* 玻璃流光：一道柔和光带缓慢掠过表面 (Slow diagonal sheen sweep) */
.login-card__sheen {
  position: absolute;
  inset: -12%;
  z-index: 0;
  border-radius: inherit;
  background: linear-gradient(
    105deg,
    transparent 42%,
    var(--login-sheen-color) 50%,
    transparent 58%
  );
  mix-blend-mode: screen;
  opacity: var(--login-sheen-opacity);
  pointer-events: none;
  transform: translateX(-72%) rotate(6deg);
  animation: login-card-sheen 9s ease-in-out infinite alternate;
  will-change: transform;
}

@keyframes login-card-sheen {
  from { transform: translateX(-72%) rotate(6deg); }
  to { transform: translateX(72%) rotate(6deg); }
}

/* 卡片透镜滤镜定义容器 (hidden filter defs host) */
.login-card-filters {
  position: absolute;
  width: 0;
  height: 0;
  overflow: hidden;
  pointer-events: none;
}

/* 指针跟随焦散高光 (Pointer-tracked caustic glare) */
.login-card__glare {
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: inherit;
  background: radial-gradient(
    46% 40% at var(--login-glare-x, 50%) var(--login-glare-y, 32%),
    var(--login-glare-color),
    transparent 72%
  );
  mix-blend-mode: screen;
  opacity: 0;
  pointer-events: none;
  transition: opacity 300ms ease;
}

.login-card:hover .login-card__glare {
  opacity: 1;
}

/* 顶部白金镜面反光导光条 (Platinum specular light bar) */
.login-card::before {
  background: var(--login-card-specular);
  opacity: var(--login-card-specular-opacity);
}

/* 内部折射光晕 (Internal refraction bloom) */
.login-card::after {
  position: absolute;
  inset: 0;
  z-index: 0;
  border-radius: inherit;
  background: var(--login-card-bloom);
  content: '';
  opacity: var(--login-card-bloom-opacity);
  pointer-events: none;
}

@media (hover: hover) and (pointer: fine) {
  .login-card:hover {
    transform: translateY(-6px) scale(1.02);
    border-color: var(--login-card-hover-border);
    box-shadow: var(--login-card-hover-shadow);
  }
}

.login-card :deep(.login-card__body) {
  position: relative;
  z-index: 1;
  padding: 40px 36px 32px;
}

/* ============================================================
   6. 品牌标题与 Logo
   ============================================================ */
.logo-section {
  margin-bottom: 8px;
  text-align: center;
}

.noyo-logo-wrap,
.tenant-logo-wrap {
  display: flex;
  height: 64px;
  margin-bottom: 16px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.noyo-logo-img,
.tenant-logo-img {
  max-width: 180px;
  max-height: 100%;
  object-fit: contain;
}

.noyo-logo-img {
  height: 60px;
  filter: var(--login-logo-filter);
}

:deep(.svg-container) {
  display: flex;
  height: 100%;
  align-items: center;
  justify-content: center;
}

:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
}

.brand-title {
  margin: 0 0 4px;
  color: var(--text-main);
  font-size: 1.75rem;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.brand-subtitle {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.75rem;
  font-weight: 500;
  letter-spacing: 0.16em;
  text-transform: uppercase;
}

.divider {
  height: 1px;
  margin: 24px 0;
  background: var(--login-divider);
}

/* ============================================================
   7. 表单与输入框
   ============================================================ */
.error-alert {
  display: flex;
  margin-bottom: 20px;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border: 1px solid var(--login-error-border);
  border-radius: 12px;
  background: var(--login-error-bg);
  color: var(--login-error-color);
  font-size: 0.85rem;
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.input-group-custom {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.input-label {
  color: var(--text-secondary);
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.08em;
}

.input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  z-index: 1;
  left: 14px;
  color: var(--text-secondary);
  transition: color 160ms ease;
}

.input-field {
  width: 100%;
  padding: 12px 44px 12px 42px;
  border: 1px solid var(--login-input-border);
  border-radius: 12px;
  outline: none;
  background: var(--login-input-bg);
  color: var(--text-main);
  font: inherit;
  transition: border-color 160ms ease, box-shadow 160ms ease, background 160ms ease;
}

.input-field::placeholder {
  color: var(--login-input-placeholder);
}

.input-field:focus {
  border-color: var(--color-brand);
  background: var(--login-input-focus-bg);
  box-shadow: 0 0 0 3px var(--color-brand-subtle);
}

.input-wrap:focus-within .input-icon {
  color: var(--color-brand);
}

.toggle-password {
  position: absolute;
  right: 8px;
  display: inline-grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: color 160ms ease, background 160ms ease;
}

.toggle-password:hover,
.toggle-password:focus-visible {
  background: var(--color-brand-subtle);
  color: var(--color-brand-hover);
}

.login-page[data-bs-theme='dark'] .toggle-password:hover,
.login-page[data-bs-theme='dark'] .toggle-password:focus-visible {
  color: var(--color-brand);
}

.toggle-password:focus-visible,
.submit-btn:focus-visible {
  outline: 2px solid var(--color-brand);
  outline-offset: 2px;
}

/* ============================================================
   8. 登录提交按钮 (Submit Button)
   ============================================================ */
.submit-btn {
  display: inline-flex;
  min-height: 48px;
  width: 100%;
  margin-top: 4px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 1px solid var(--login-btn-border);
  border-radius: 12px;
  background: var(--login-btn-bg);
  box-shadow: var(--login-btn-shadow);
  color: var(--login-btn-text) !important;
  cursor: pointer;
  font: inherit;
  font-weight: 650;
  letter-spacing: 0.04em;
  transition: transform 160ms ease, background 160ms ease, box-shadow 160ms ease;
}

.submit-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  background: var(--login-btn-hover-bg);
  box-shadow: var(--login-btn-hover-shadow);
}

.submit-btn:active:not(:disabled) {
  transform: translateY(0);
}

.submit-btn:disabled {
  cursor: not-allowed;
  opacity: 0.6;
}

.spinner {
  width: 18px;
  height: 18px;
  border: 2px solid var(--login-spinner-track);
  border-top-color: var(--login-spinner-fill);
  border-radius: 50%;
  animation: login-spinner 0.7s linear infinite;
}

.card-footer-tag {
  display: flex;
  margin-top: 28px;
  justify-content: center;
}

.aiot-badge {
  display: inline-flex;
  padding: 4px 14px;
  border: 1px solid var(--login-badge-border);
  border-radius: 9999px;
  background: var(--login-badge-bg);
  color: var(--login-badge-color);
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}

.page-footer {
  position: absolute;
  z-index: 1;
  right: 24px;
  bottom: 20px;
  left: 24px;
  color: var(--text-secondary);
  font-size: 0.72rem;
  letter-spacing: 0.04em;
  text-align: center;
}

/* ============================================================
   9. 关键帧动画与无障碍降级
   ============================================================ */
@keyframes login-liquid-one {
  0% { transform: translate3d(0, 0, 0) scale(1); }
  50% { transform: translate3d(12vmin, 8vmin, 0) scale(1.08); }
  100% { transform: translate3d(18vmin, 14vmin, 0) scale(1.14); }
}

@keyframes login-liquid-two {
  0% { transform: translate3d(0, 0, 0) scale(1); }
  50% { transform: translate3d(-10vmin, -8vmin, 0) scale(1.06); }
  100% { transform: translate3d(-16vmin, -16vmin, 0) scale(1.15); }
}

@keyframes login-liquid-three {
  0% { transform: translate3d(-6vmin, 4vmin, 0) scale(0.92); }
  50% { transform: translate3d(2vmin, -3vmin, 0) scale(1.05); }
  100% { transform: translate3d(8vmin, -10vmin, 0) scale(1.16); }
}

@keyframes login-liquid-four {
  0% { transform: translate3d(-4vmin, -4vmin, 0) scale(0.9); }
  50% { transform: translate3d(6vmin, 5vmin, 0) scale(1.1); }
  100% { transform: translate3d(-2vmin, 8vmin, 0) scale(1.02); }
}

@keyframes login-liquid-caustics {
  0% { transform: scale(1.05) rotate(0deg); opacity: 0.55; }
  50% { transform: scale(1.18) rotate(3deg); opacity: 0.72; }
  100% { transform: scale(1.1) rotate(-3deg); opacity: 0.58; }
}

@keyframes login-liquid-veil {
  from { transform: translateX(-18%) rotate(-4deg) scale(1.08); }
  to { transform: translateX(18%) rotate(4deg) scale(1.14); }
}

@keyframes login-spinner {
  to { transform: rotate(360deg); }
}

@media (max-width: 480px) {
  .login-theme-switcher {
    top: 16px;
    right: 16px;
  }

  .login-card :deep(.login-card__body) {
    padding: 32px 24px 24px;
  }

  .brand-title {
    font-size: 1.4rem;
  }
}

@media (max-width: 360px) {
  .login-card :deep(.login-card__body) {
    padding: 24px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .login-ambient__orb,
  .login-ambient__caustics,
  .login-ambient__veil,
  .spinner {
    animation: none;
  }

  .login-ambient__halo-core {
    animation: none;
  }

  .login-ambient,
  .login-shell {
    transform: none;
    will-change: auto;
  }

  .login-card {
    animation: none;
    transform: none;
    transition: none;
  }

  .login-card__sheen {
    animation: none;
    opacity: 0;
  }

  .submit-btn {
    transition: none;
  }
}

@media (prefers-reduced-transparency: reduce) {
  .login-card {
    background: var(--login-card-fallback-bg);
  }
}

@media (forced-colors: active) {
  .login-card,
  .input-field,
  .submit-btn {
    border-color: CanvasText;
  }
}

@supports not (backdrop-filter: blur(1px)) {
  .login-card {
    background: var(--login-card-fallback-bg);
  }
}
</style>
