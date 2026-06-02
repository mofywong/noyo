<template>
  <div
    class="login-page"
    ref="pageRef"
    @mousemove="onMouseMove"
    @touchmove.passive="onTouchMove"
  >
    <!-- Canvas 鼠标跟随光影层 -->
    <canvas ref="canvasRef" class="light-canvas"></canvas>

    <!-- 背景网格层 -->
    <div class="grid-overlay"></div>

    <!-- 浮动粒子 -->
    <div class="particles">
      <span v-for="n in 8" :key="n" class="particle" :style="particleStyle(n)"></span>
    </div>

    <!-- 品牌水印（大型半透明 Logo） -->
    <div class="brand-watermark">
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 654 500" width="100%" height="100%">
        <defs>
          <linearGradient id="wm-grad" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="rgba(56, 189, 248, 0.04)" />
            <stop offset="100%" stop-color="rgba(6, 114, 192, 0.02)" />
          </linearGradient>
        </defs>
        <path d="M 167.99 19.40 Q 156.00 19.00, 155.74 31.00 L 148.26 372.00 Q 148.00 384.00, 159.55 387.24 L 193.45 396.76 Q 205.00 400.00, 213.10 391.15 L 239.90 361.85 Q 248.00 353.00, 247.89 341.00 L 247.11 256.00 Q 247.00 244.00, 241.30 233.44 L 191.70 141.56 Q 186.00 131.00, 193.84 140.09 L 398.16 376.91 Q 406.00 386.00, 418.00 386.13 L 484.00 386.87 Q 496.00 387.00, 496.30 375.00 L 504.70 40.00 Q 505.00 28.00, 493.76 23.81 L 457.24 10.19 Q 446.00 6.00, 437.70 14.66 L 408.30 45.34 Q 400.00 54.00, 400.22 66.00 L 401.78 152.00 Q 402.00 164.00, 407.90 174.45 L 461.10 268.55 Q 467.00 279.00, 459.18 269.90 L 253.82 31.10 Q 246.00 22.00, 234.01 21.60 Z" fill="url(#wm-grad)" />
      </svg>
    </div>

    <!-- 登录卡片 -->
    <div class="login-card" :class="{ 'card-glow': isNearCard }">
      <!-- Logo 区域 -->
      <div class="logo-section">
        <div v-if="tenantLogo" class="tenant-logo-wrap">
          <div v-if="tenantLogo.trim().startsWith('<svg') || tenantLogo.trim().startsWith('<?xml')" v-html="DOMPurify.sanitize(tenantLogo, { USE_PROFILES: { svg: true } })" class="svg-container"></div>
          <img v-else :src="tenantLogo" alt="Logo" class="tenant-logo-img">
        </div>
        <div v-else class="noyo-logo-wrap">
          <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 654 500" class="noyo-logo-svg">
            <defs>
              <linearGradient id="logo-grad" x1="0%" y1="0%" x2="100%" y2="100%">
                <stop offset="0%" stop-color="#38bdf8" />
                <stop offset="50%" stop-color="#3b82f6" />
                <stop offset="100%" stop-color="#0672C0" />
              </linearGradient>
              <filter id="logo-glow">
                <feGaussianBlur stdDeviation="3" result="blur" />
                <feMerge>
                  <feMergeNode in="blur" />
                  <feMergeNode in="SourceGraphic" />
                </feMerge>
              </filter>
            </defs>
            <path d="M 167.99 19.40 Q 156.00 19.00, 155.74 31.00 L 148.26 372.00 Q 148.00 384.00, 159.55 387.24 L 193.45 396.76 Q 205.00 400.00, 213.10 391.15 L 239.90 361.85 Q 248.00 353.00, 247.89 341.00 L 247.11 256.00 Q 247.00 244.00, 241.30 233.44 L 191.70 141.56 Q 186.00 131.00, 193.84 140.09 L 398.16 376.91 Q 406.00 386.00, 418.00 386.13 L 484.00 386.87 Q 496.00 387.00, 496.30 375.00 L 504.70 40.00 Q 505.00 28.00, 493.76 23.81 L 457.24 10.19 Q 446.00 6.00, 437.70 14.66 L 408.30 45.34 Q 400.00 54.00, 400.22 66.00 L 401.78 152.00 Q 402.00 164.00, 407.90 174.45 L 461.10 268.55 Q 467.00 279.00, 459.18 269.90 L 253.82 31.10 Q 246.00 22.00, 234.01 21.60 Z" fill="url(#logo-grad)" filter="url(#logo-glow)" />
          </svg>
        </div>
        <h1 class="brand-title">{{ tenantName || 'Noyo' }}</h1>
        <p class="brand-subtitle">{{ $t('auth_login_subtitle') }}</p>
      </div>

      <!-- 分隔线 -->
      <div class="divider"></div>

      <!-- 错误提示 -->
      <div v-if="errorMsg" class="error-alert">
        <i class="bi bi-exclamation-triangle-fill"></i>
        <span>{{ errorMsg }}</span>
      </div>

      <!-- 登录表单 -->
      <form @submit.prevent="handleLogin" class="login-form">
        <div class="input-group-custom">
          <label class="input-label">{{ $t('auth_username') }}</label>
          <div class="input-wrap">
            <i class="bi bi-person input-icon"></i>
            <input
              v-model="username"
              type="text"
              class="input-field"
              :placeholder="$t('auth_username_placeholder')"
              required
              autofocus
              id="login-username"
            />
          </div>
        </div>

        <div class="input-group-custom">
          <label class="input-label">{{ $t('auth_password') }}</label>
          <div class="input-wrap">
            <i class="bi bi-lock input-icon"></i>
            <input
              v-model="password"
              :type="showPassword ? 'text' : 'password'"
              class="input-field"
              :placeholder="$t('auth_password_placeholder')"
              required
              id="login-password"
            />
            <button
              type="button"
              class="toggle-password"
              @click="showPassword = !showPassword"
              tabindex="-1"
            >
              <i :class="showPassword ? 'bi bi-eye-slash' : 'bi bi-eye'"></i>
            </button>
          </div>
        </div>

        <button
          type="submit"
          class="submit-btn"
          :disabled="loading"
          id="login-submit"
        >
          <span v-if="loading" class="spinner"></span>
          <span>{{ $t('auth_sign_in') }}</span>
        </button>
      </form>

      <!-- 底部标语 -->
      <div class="card-footer-tag">
        <span class="aiot-badge">AIoT Platform</span>
      </div>
    </div>

    <!-- 页面底部版权 -->
    <div class="page-footer">
      <span>© {{ new Date().getFullYear() }} Noyo · Intelligent IoT</span>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import DOMPurify from 'dompurify'
import { useAuthStore } from '../stores/auth'
import { useI18n } from 'vue-i18n'
import axios from 'axios'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t } = useI18n()

const username = ref('')
const password = ref('')
const showPassword = ref(false)
const errorMsg = ref('')
const loading = ref(false)

const tenantName = ref('')
const tenantLogo = ref('')

// Canvas 相关
const canvasRef = ref(null)
const pageRef = ref(null)
let ctx = null
let animationId = null
let mouseX = 0
let mouseY = 0
let currentX = 0
let currentY = 0
let isNearCard = ref(false)

// 线性插值
const lerp = (a, b, t) => a + (b - a) * t

// 粒子样式生成
const particleStyle = (n) => {
  const seed = n * 137.508
  const size = 2 + (seed % 4)
  const left = (seed * 7) % 100
  const top = (seed * 13) % 100
  const duration = 15 + (seed % 20)
  const delay = -(seed % 15)
  const opacity = 0.15 + (seed % 30) / 100

  return {
    width: `${size}px`,
    height: `${size}px`,
    left: `${left}%`,
    top: `${top}%`,
    animationDuration: `${duration}s`,
    animationDelay: `${delay}s`,
    opacity: opacity,
  }
}

// 鼠标移动处理
const onMouseMove = (e) => {
  mouseX = e.clientX
  mouseY = e.clientY
  checkNearCard(e.clientX, e.clientY)
}

const onTouchMove = (e) => {
  if (e.touches.length > 0) {
    mouseX = e.touches[0].clientX
    mouseY = e.touches[0].clientY
    checkNearCard(mouseX, mouseY)
  }
}

// 检测鼠标是否靠近卡片
const checkNearCard = (x, y) => {
  const card = document.querySelector('.login-card')
  if (!card) return
  const rect = card.getBoundingClientRect()
  const cx = rect.left + rect.width / 2
  const cy = rect.top + rect.height / 2
  const dist = Math.sqrt((x - cx) ** 2 + (y - cy) ** 2)
  isNearCard.value = dist < 300
}

// Canvas 动画循环
const animate = () => {
  if (!ctx || !canvasRef.value) return

  const canvas = canvasRef.value
  const w = canvas.width
  const h = canvas.height

  // 平滑跟随
  currentX = lerp(currentX, mouseX, 0.06)
  currentY = lerp(currentY, mouseY, 0.06)

  // 清除画布
  ctx.clearRect(0, 0, w, h)

  // 主光圈 — 品牌蓝色
  const mainGrad = ctx.createRadialGradient(
    currentX, currentY, 0,
    currentX, currentY, 350
  )
  mainGrad.addColorStop(0, 'rgba(56, 189, 248, 0.12)')
  mainGrad.addColorStop(0.4, 'rgba(59, 130, 246, 0.06)')
  mainGrad.addColorStop(1, 'rgba(59, 130, 246, 0)')
  ctx.fillStyle = mainGrad
  ctx.fillRect(0, 0, w, h)

  // 副光圈 — 紫色偏移
  const offsetX = currentX + 120
  const offsetY = currentY - 80
  const subGrad = ctx.createRadialGradient(
    offsetX, offsetY, 0,
    offsetX, offsetY, 200
  )
  subGrad.addColorStop(0, 'rgba(139, 92, 246, 0.08)')
  subGrad.addColorStop(0.5, 'rgba(139, 92, 246, 0.03)')
  subGrad.addColorStop(1, 'rgba(139, 92, 246, 0)')
  ctx.fillStyle = subGrad
  ctx.fillRect(0, 0, w, h)

  // 第三光圈 — 青色微弱
  const t3X = currentX - 80
  const t3Y = currentY + 100
  const thirdGrad = ctx.createRadialGradient(
    t3X, t3Y, 0,
    t3X, t3Y, 180
  )
  thirdGrad.addColorStop(0, 'rgba(6, 182, 212, 0.06)')
  thirdGrad.addColorStop(1, 'rgba(6, 182, 212, 0)')
  ctx.fillStyle = thirdGrad
  ctx.fillRect(0, 0, w, h)

  animationId = requestAnimationFrame(animate)
}

// Canvas 尺寸自适应
const resizeCanvas = () => {
  if (!canvasRef.value) return
  const dpr = window.devicePixelRatio || 1
  canvasRef.value.width = window.innerWidth * dpr
  canvasRef.value.height = window.innerHeight * dpr
  canvasRef.value.style.width = window.innerWidth + 'px'
  canvasRef.value.style.height = window.innerHeight + 'px'
  ctx.scale(dpr, dpr)
}

onMounted(async () => {
  // 初始化 Canvas
  if (canvasRef.value) {
    ctx = canvasRef.value.getContext('2d')
    resizeCanvas()
    // 初始位置设到中间
    mouseX = window.innerWidth / 2
    mouseY = window.innerHeight / 2
    currentX = mouseX
    currentY = mouseY
    animate()
  }

  window.addEventListener('resize', resizeCanvas)

  // 加载租户信息
  if (route.params.suffix) {
    try {
      const res = await axios.get('/api/auth/tenant-info', { params: { suffix: route.params.suffix } })
      if (res.data.code === 0 && res.data.data) {
        tenantName.value = res.data.data.name
        tenantLogo.value = res.data.data.logo
      }
    } catch (e) {
      console.warn('Failed to fetch tenant info for suffix:', route.params.suffix)
    }
  }
})

onUnmounted(() => {
  if (animationId) cancelAnimationFrame(animationId)
  window.removeEventListener('resize', resizeCanvas)
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
   Noyo Login — AIOT 科技感暗色主题
   ============================================================ */

@import url('https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap');

.login-page {
  position: relative;
  width: 100vw;
  height: 100vh;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #060b18;
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
}

/* ---- Canvas 光影层 ---- */
.light-canvas {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 1;
  pointer-events: none;
}

/* ---- 科技网格背景 ---- */
.grid-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 2;
  pointer-events: none;
  background-image:
    linear-gradient(rgba(56, 189, 248, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(56, 189, 248, 0.03) 1px, transparent 1px);
  background-size: 60px 60px;
  mask-image: radial-gradient(ellipse 70% 70% at 50% 50%, black 30%, transparent 100%);
  -webkit-mask-image: radial-gradient(ellipse 70% 70% at 50% 50%, black 30%, transparent 100%);
}

/* ---- 浮动粒子 ---- */
.particles {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 3;
  pointer-events: none;
}

.particle {
  position: absolute;
  background: #38bdf8;
  border-radius: 50%;
  animation: particle-float linear infinite;
  box-shadow: 0 0 6px 2px rgba(56, 189, 248, 0.3);
}

@keyframes particle-float {
  0% {
    transform: translate(0, 0) scale(1);
    opacity: 0;
  }
  10% {
    opacity: 1;
  }
  50% {
    transform: translate(40px, -60px) scale(1.2);
  }
  90% {
    opacity: 1;
  }
  100% {
    transform: translate(-30px, 50px) scale(0.8);
    opacity: 0;
  }
}

/* ---- 品牌水印 ---- */
.brand-watermark {
  position: absolute;
  width: 600px;
  height: 500px;
  right: -80px;
  bottom: -60px;
  z-index: 2;
  pointer-events: none;
  opacity: 0.4;
}

/* ---- 登录卡片 ---- */
.login-card {
  position: relative;
  z-index: 10;
  width: 100%;
  max-width: 420px;
  padding: 40px 36px 32px;
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(24px) saturate(1.2);
  -webkit-backdrop-filter: blur(24px) saturate(1.2);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 20px;
  box-shadow:
    0 32px 64px -16px rgba(0, 0, 0, 0.6),
    inset 0 1px 0 0 rgba(255, 255, 255, 0.05);
  transition: border-color 0.6s ease, box-shadow 0.6s ease;
}

.login-card.card-glow {
  border-color: rgba(56, 189, 248, 0.15);
  box-shadow:
    0 32px 64px -16px rgba(0, 0, 0, 0.6),
    0 0 60px -10px rgba(56, 189, 248, 0.08),
    inset 0 1px 0 0 rgba(255, 255, 255, 0.08);
}

/* ---- Logo 区域 ---- */
.logo-section {
  text-align: center;
  margin-bottom: 8px;
}

.noyo-logo-wrap {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 64px;
  margin-bottom: 16px;
}

.noyo-logo-svg {
  height: 60px;
  width: auto;
  animation: logo-breathe 4s ease-in-out infinite;
}

@keyframes logo-breathe {
  0%, 100% {
    filter: drop-shadow(0 0 8px rgba(56, 189, 248, 0.3));
  }
  50% {
    filter: drop-shadow(0 0 16px rgba(56, 189, 248, 0.6));
  }
}

.tenant-logo-wrap {
  display: flex;
  justify-content: center;
  align-items: center;
  height: 64px;
  margin-bottom: 16px;
  overflow: hidden;
}

.tenant-logo-img {
  max-height: 100%;
  max-width: 100%;
  object-fit: contain;
}

:deep(.svg-container) {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

:deep(.svg-container svg) {
  max-width: 100%;
  max-height: 100%;
  filter: drop-shadow(0 0 8px rgba(255,255,255,0.15));
}

.brand-title {
  font-size: 1.75rem;
  font-weight: 700;
  color: #f0f6ff;
  letter-spacing: 2px;
  margin: 0 0 4px 0;
  background: linear-gradient(135deg, #e0f2fe, #38bdf8, #818cf8);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.brand-subtitle {
  font-size: 0.75rem;
  color: #64748b;
  letter-spacing: 3px;
  text-transform: uppercase;
  margin: 0;
  font-weight: 400;
}

/* ---- 分隔线 ---- */
.divider {
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(56, 189, 248, 0.2), transparent);
  margin: 20px 0 24px;
}

/* ---- 错误提示 ---- */
.error-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  background: rgba(239, 68, 68, 0.08);
  border: 1px solid rgba(239, 68, 68, 0.15);
  border-radius: 10px;
  color: #fca5a5;
  font-size: 0.85rem;
  margin-bottom: 20px;
  backdrop-filter: blur(4px);
}

.error-alert i {
  flex-shrink: 0;
  color: #ef4444;
}

/* ---- 表单样式 ---- */
.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.input-group-custom {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.input-label {
  font-size: 0.72rem;
  font-weight: 500;
  color: #94a3b8;
  letter-spacing: 1px;
  text-transform: uppercase;
}

.input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  left: 14px;
  color: #475569;
  font-size: 1rem;
  z-index: 1;
  transition: color 0.3s ease;
}

.input-field {
  width: 100%;
  padding: 12px 14px 12px 42px;
  background: rgba(15, 23, 42, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 12px;
  color: #f1f5f9;
  font-size: 0.95rem;
  font-family: inherit;
  transition: all 0.3s ease;
  outline: none;
}

.input-field::placeholder {
  color: #475569;
}

.input-field:focus {
  background: rgba(15, 23, 42, 0.8);
  border-color: rgba(56, 189, 248, 0.4);
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.08), 0 0 20px -4px rgba(56, 189, 248, 0.15);
}

.input-field:focus ~ .input-icon,
.input-wrap:focus-within .input-icon {
  color: #38bdf8;
}

.toggle-password {
  position: absolute;
  right: 12px;
  background: none;
  border: none;
  color: #475569;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  transition: color 0.2s;
}

.toggle-password:hover {
  color: #94a3b8;
}

/* ---- 登录按钮 ---- */
.submit-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 13px 24px;
  margin-top: 4px;
  background: linear-gradient(135deg, #0369a1, #3b82f6);
  border: none;
  border-radius: 12px;
  color: #fff;
  font-size: 0.95rem;
  font-weight: 600;
  font-family: inherit;
  letter-spacing: 1px;
  cursor: pointer;
  transition: all 0.3s ease;
  position: relative;
  overflow: hidden;
}

.submit-btn::before {
  content: '';
  position: absolute;
  top: 0;
  left: -100%;
  width: 100%;
  height: 100%;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.1), transparent);
  transition: left 0.5s ease;
}

.submit-btn:hover:not(:disabled) {
  transform: translateY(-2px);
  box-shadow: 0 12px 28px -6px rgba(59, 130, 246, 0.45);
  background: linear-gradient(135deg, #0284c7, #60a5fa);
}

.submit-btn:hover:not(:disabled)::before {
  left: 100%;
}

.submit-btn:active:not(:disabled) {
  transform: translateY(0);
  box-shadow: 0 4px 12px -2px rgba(59, 130, 246, 0.3);
}

.submit-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

/* ---- 加载旋转器 ---- */
.spinner {
  width: 18px;
  height: 18px;
  border: 2px solid rgba(255, 255, 255, 0.3);
  border-top-color: #fff;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* ---- 底部标签 ---- */
.card-footer-tag {
  display: flex;
  justify-content: center;
  margin-top: 28px;
}

.aiot-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 14px;
  background: rgba(56, 189, 248, 0.06);
  border: 1px solid rgba(56, 189, 248, 0.1);
  border-radius: 20px;
  color: #38bdf8;
  font-size: 0.68rem;
  font-weight: 500;
  letter-spacing: 2px;
  text-transform: uppercase;
}

/* ---- 页面底部 ---- */
.page-footer {
  position: absolute;
  bottom: 20px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 10;
  color: #334155;
  font-size: 0.72rem;
  letter-spacing: 0.5px;
}

/* ---- 响应式 ---- */
@media (max-width: 480px) {
  .login-card {
    margin: 16px;
    padding: 32px 24px 24px;
    max-width: calc(100% - 32px);
  }

  .brand-title {
    font-size: 1.4rem;
  }

  .brand-watermark {
    width: 300px;
    height: 250px;
    right: -40px;
    bottom: -30px;
  }

  .noyo-logo-svg {
    height: 48px;
  }
}

@media (max-width: 360px) {
  .login-card {
    padding: 24px 20px 20px;
  }
}
</style>
