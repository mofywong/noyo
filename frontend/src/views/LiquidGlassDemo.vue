<template>
  <div
    ref="demoRoot"
    class="liquid-glass-demo noyo-liquid-page noyo-liquid-v4-page"
    :class="demoClasses"
    :data-bs-theme="resolvedTheme"
    :style="qualityStyle"
  >
    <header class="demo-header">
      <div>
        <p class="demo-eyebrow">Noyo Design Lab</p>
        <h1>{{ text.title }}</h1>
        <p>{{ text.subtitle }}</p>
      </div>
      <span class="demo-dev-badge">
        <i class="bi bi-tools" aria-hidden="true"></i>
        {{ text.developmentOnly }}
      </span>
    </header>

    <GlassGroup
      as="section"
      profile="regular"
      shape="panel"
      class="demo-console"
      labelled-by="liquid-glass-console-title"
    >
      <div class="demo-section-heading">
        <div>
          <p class="demo-kicker">01</p>
          <h2 id="liquid-glass-console-title">{{ text.consoleTitle }}</h2>
        </div>
        <p>{{ text.consoleHint }}</p>
      </div>

      <div class="demo-control-grid">
        <label v-for="control in controls" :key="control.key" class="demo-control">
          <span>{{ control.label }}</span>
          <select
            v-model="settings[control.key]"
            class="form-select form-select-sm"
            :aria-label="control.label"
            :data-demo-control="control.key"
          >
            <option
              v-for="option in control.options"
              :key="option.value"
              :value="option.value"
            >
              {{ option.label }}
            </option>
          </select>
        </label>
      </div>
    </GlassGroup>

    <section class="demo-section" aria-labelledby="liquid-glass-compare-title">
      <div class="demo-section-heading demo-section-heading--outside">
        <div>
          <p class="demo-kicker">02</p>
          <h2 id="liquid-glass-compare-title">{{ text.compareTitle }}</h2>
        </div>
        <p>{{ text.compareHint }}</p>
      </div>

      <div class="demo-comparison-grid">
        <article data-demo-current class="demo-comparison-card">
          <div class="demo-comparison-label demo-comparison-label--current">
            <span><i class="bi bi-layers" aria-hidden="true"></i>{{ text.current }}</span>
            <small>{{ text.currentHint }}</small>
          </div>
          <div class="demo-scene" :class="backgroundClass">
            <div class="demo-current-toolbar demo-frosted-surface">
              <strong>{{ scenarioTitle }}</strong>
              <span class="demo-current-pill demo-frosted-surface">
                <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
                {{ text.refresh }}
              </span>
            </div>

            <div class="demo-kpi-grid">
              <div
                v-for="item in fixtureKpis"
                :key="'current-' + item.key"
                class="demo-current-kpi demo-frosted-surface"
              >
                <span>{{ item.label }}</span>
                <strong :data-fixture-total="item.key === 'total' ? '' : undefined">
                  {{ item.value }}
                </strong>
              </div>
            </div>

            <div v-if="settings.scenario === 'dashboard'" class="demo-current-panel demo-frosted-surface">
              <div class="demo-panel-title">{{ text.resourceOverview }}</div>
              <div class="demo-bar-list">
                <div v-for="bar in resourceBars" :key="bar.label">
                  <span>{{ bar.label }}</span>
                  <div><i :style="{ width: bar.value + '%' }"></i></div>
                </div>
              </div>
            </div>

            <div v-else class="demo-current-table demo-frosted-surface">
              <table>
                <thead>
                  <tr>
                    <th>{{ text.device }}</th>
                    <th>{{ text.product }}</th>
                    <th>{{ text.status }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="device in demoDevices.slice(0, 5)" :key="'current-' + device.code">
                    <td>{{ device.name }}</td>
                    <td>{{ device.product }}</td>
                    <td>{{ device.status }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </article>

        <article data-demo-target class="demo-comparison-card">
          <div class="demo-comparison-label demo-comparison-label--target">
            <span><i class="bi bi-check2-circle" aria-hidden="true"></i>{{ text.target }}</span>
            <small>{{ text.targetHint }}</small>
          </div>
          <div ref="targetStage" class="demo-scene demo-target-stage" :class="backgroundClass">
            <GlassGroup
              as="header"
              :profile="materialProfile"
              shape="rounded"
              class="demo-target-toolbar"
            >
              <strong>{{ scenarioTitle }}</strong>
              <div class="demo-target-actions">
                <button type="button" class="btn btn-sm btn-outline-primary">
                  <i class="bi bi-arrow-clockwise" aria-hidden="true"></i>
                  {{ text.refresh }}
                </button>
                <LiquidGlassPopover placement="bottom-end" :label="text.materialDetails">
                  <template #trigger>
                    <span class="demo-info-trigger">
                      <i class="bi bi-info-circle" aria-hidden="true"></i>
                    </span>
                  </template>
                  <div class="demo-popover-copy">
                    <strong>{{ text.materialDetails }}</strong>
                    <span>{{ text.materialDetailsBody }}</span>
                  </div>
                </LiquidGlassPopover>
              </div>
            </GlassGroup>

            <GlassGroup
              as="section"
              :profile="materialProfile"
              composition="island"
              shape="panel"
              interactive
              class="demo-target-kpi-island"
            >
              <div class="demo-kpi-grid">
                <div
                  v-for="item in fixtureKpis"
                  :key="'target-' + item.key"
                  class="noyo-glass-zone demo-target-kpi"
                  data-contrast-sample
                >
                  <span data-contrast-text>{{ item.label }}</span>
                  <strong :data-fixture-total="item.key === 'total' ? '' : undefined">
                    {{ item.value }}
                  </strong>
                </div>
              </div>
            </GlassGroup>

            <template v-if="settings.scenario === 'dashboard'">
              <GlassGroup
                as="section"
                :profile="materialProfile"
                composition="island"
                shape="panel"
                interactive
                class="demo-dashboard-panels demo-target-content-island"
              >
                <div class="noyo-glass-zone demo-target-panel">
                  <div class="demo-panel-title">{{ text.resourceOverview }}</div>
                  <div class="demo-bar-list">
                    <div v-for="bar in resourceBars" :key="'target-' + bar.label">
                      <span>{{ bar.label }}</span>
                      <div><i :style="{ width: bar.value + '%' }"></i></div>
                    </div>
                  </div>
                </div>
                <button
                  type="button"
                  class="noyo-glass-zone noyo-glass-action demo-ai-trigger"
                >
                  <i class="bi bi-robot" aria-hidden="true"></i>
                  <span><strong>{{ text.aiCopilot }}</strong><small>{{ text.running }}</small></span>
                  <i class="bi bi-arrow-right" aria-hidden="true"></i>
                </button>
              </GlassGroup>
            </template>

            <section
              v-else
              class="demo-target-table noyo-content-surface"
              data-demo-target-solid-table
            >
              <div class="demo-table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>{{ text.device }}</th>
                      <th>{{ text.product }}</th>
                      <th>{{ text.protocol }}</th>
                      <th>{{ text.status }}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="device in demoDevices" :key="device.code">
                      <td><strong>{{ device.name }}</strong><small>{{ device.code }}</small></td>
                      <td>{{ device.product }}</td>
                      <td><span class="demo-protocol-badge">{{ device.protocol }}</span></td>
                      <td>
                        <span class="demo-status" :class="device.online ? 'demo-status--online' : 'demo-status--offline'">
                          <i class="bi" :class="device.online ? 'bi-check-circle-fill' : 'bi-dash-circle-fill'" aria-hidden="true"></i>
                          {{ device.status }}
                        </span>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </section>
          </div>
        </article>
      </div>
    </section>

    <section class="demo-section" aria-labelledby="liquid-glass-components-title">
      <div class="demo-section-heading demo-section-heading--outside">
        <div>
          <p class="demo-kicker">03</p>
          <h2 id="liquid-glass-components-title">{{ text.componentsTitle }}</h2>
        </div>
        <p>{{ text.componentsHint }}</p>
      </div>
      <div class="demo-component-grid">
        <SolidSurface class="demo-component-card">
          <i class="bi bi-window-stack" aria-hidden="true"></i>
          <div><strong>{{ text.solidContent }}</strong><span>{{ text.solidContentHint }}</span></div>
        </SolidSurface>
        <GlassGroup profile="regular" class="demo-component-card">
          <i class="bi bi-toggles" aria-hidden="true"></i>
          <div><strong>{{ text.regularGlass }}</strong><span>{{ text.regularGlassHint }}</span></div>
        </GlassGroup>
        <div class="demo-media-swatch" :class="backgroundClass">
          <GlassGroup profile="clear-media" shape="pill" class="demo-media-control">
            <i class="bi bi-play-fill" aria-hidden="true"></i>
            <span>{{ text.clearMedia }}</span>
          </GlassGroup>
        </div>
      </div>
    </section>

    <SolidSurface
      as="section"
      density="compact"
      elevation="flat"
      class="demo-diagnostics"
      labelled-by="liquid-glass-diagnostics-title"
    >
      <div class="demo-section-heading">
        <div>
          <p class="demo-kicker">04</p>
          <h2 id="liquid-glass-diagnostics-title">{{ text.diagnosticsTitle }}</h2>
        </div>
        <p>{{ text.diagnosticsHint }}</p>
      </div>

      <div class="demo-diagnostic-grid">
        <div v-for="metric in diagnosticMetrics" :key="metric.key" class="demo-diagnostic">
          <span>{{ metric.label }}</span>
          <strong :data-diagnostic="metric.key">{{ metric.value }}</strong>
          <small>{{ metric.unit }}</small>
        </div>
      </div>

      <div class="demo-warning-list" aria-live="polite">
        <div v-for="warning in warnings" :key="warning.key" :class="'demo-warning--' + warning.level">
          <i class="bi" :class="warning.icon" aria-hidden="true"></i>
          <span>{{ warning.message }}</span>
        </div>
      </div>
    </SolidSurface>
  </div>
</template>

<script setup>
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from 'vue'
import { useI18n } from 'vue-i18n'
import GlassGroup from '../components/liquid-glass/GlassGroup.vue'
import LiquidGlassPopover from '../components/liquid-glass/LiquidGlassPopover.vue'
import SolidSurface from '../components/liquid-glass/SolidSurface.vue'
import {
  LIQUID_GLASS_TARGET_FPS,
  contrastRatioOverLayers,
  summarizeFrameTimes,
} from '../utils/liquidGlassDemoMetrics.js'

const copy = {
  zh: {
    title: 'Noyo 液态玻璃 v4.4 实验台',
    subtitle: '用同一组数据、同一背景和实时指标验证：功能层用玻璃、概览区用连续玻璃岛、密集数据用实体阅读表面。',
    developmentOnly: '仅开发环境',
    consoleTitle: '环境控制台',
    consoleHint: '切换环境与无障碍偏好，所有方案共享相同 fixture。',
    compareTitle: '同场景对照',
    compareHint: '左侧保留旧磨砂反例，右侧执行 v4.4 分层材质方案。',
    current: '当前磨砂方案',
    currentHint: '内容 blur、多层采样、信息随背景漂移',
    target: 'Noyo v4.4 分层目标方案',
    targetHint: '分层材质：功能层玻璃、连续概览岛、实体数据面、无嵌套采样',
    componentsTitle: '材质角色展台',
    componentsHint: 'Regular、Clear media 与 Solid 各自承担单一职责。',
    diagnosticsTitle: '实时验收面板',
    diagnosticsHint: '只统计右侧目标场景与当前活动浮层。',
    theme: '主题',
    background: '背景',
    material: '材质',
    transparency: '透明度',
    contrast: '对比度',
    motion: '动效',
    input: '输入方式',
    quality: '质量',
    scenario: '场景',
    light: '明亮',
    dark: '黑暗',
    system: '跟随系统',
    neutralLight: '中性明亮',
    neutralDark: '中性黑暗',
    spectrum: '彩色光谱',
    highContrast: '高对比背景',
    dense: '密集数据背景',
    videoFrame: '视频帧背景',
    regular: 'Regular',
    floating: 'Floating',
    clearMedia: 'Clear media',
    solidFallback: '实体降级',
    default: '默认',
    reduced: '降低透明度',
    increased: '增强对比度',
    forced: '强制色模拟',
    mouse: '鼠标',
    touch: '触摸',
    keyboard: '键盘',
    high: '高',
    balanced: '均衡',
    low: '低',
    dashboard: '仪表盘',
    devices: '设备管理',
    total: '设备总数',
    online: '在线',
    offline: '离线',
    alarms: '今日告警',
    refresh: '刷新',
    resourceOverview: '资源概览',
    cpu: 'CPU',
    memory: '内存',
    services: '服务',
    aiCopilot: 'AI 助手',
    running: '运行中',
    device: '设备',
    product: '产品',
    protocol: '协议',
    status: '状态',
    solidContent: 'Reading protection',
    solidContentHint: 'KPI、表格、图表和表单的局部可读层',
    regularGlass: 'Glass Regular',
    regularGlassHint: '导航、工具栏与组合控制',
    clearMediaHint: '仅用于受控媒体上方小控件',
    materialDetails: '材质说明',
    materialDetailsBody: '该浮层已 Teleport 到 body，并使用 Floating 外壳与稳定正文。',
    groups: '活动玻璃组',
    backdrops: 'Backdrop 采样层',
    nested: '玻璃嵌套',
    contentBackdrops: '内容滤镜命中',
    fps: '平均帧率',
    p95: 'P95 帧时间',
    pointerP95: '指针处理 P95',
    contrastMetric: '关键文字对比度',
    countUnit: '个',
    fpsUnit: 'FPS',
    msUnit: 'ms',
    ratioUnit: ':1',
    passStructure: '结构通过：目标场景没有玻璃嵌套或内容滤镜。',
    failStructure: '结构警告：检测到玻璃嵌套或内容滤镜。',
    passContrast: '可读性通过：关键文字对比度不低于 4.5:1。',
    failContrast: '可读性警告：关键文字对比度低于 4.5:1。',
    measuring: '正在采集最近 5 秒的帧数据。',
    passPerformance: '性能通过：平均帧率达到 55 FPS。',
    failPerformance: '性能警告：平均帧率低于 55 FPS。',
  },
  en: {
    title: 'Noyo Liquid Glass v4.4 Lab',
    subtitle: 'Validate one rule with identical data, backgrounds, and live metrics: functional glass for controls, continuous glass islands for overview, and solid reading surfaces for dense data.',
    developmentOnly: 'Development only',
    consoleTitle: 'Environment console',
    consoleHint: 'Change environment and accessibility preferences while every option keeps the same fixture.',
    compareTitle: 'Same-scene comparison',
    compareHint: 'The legacy frosted anti-pattern stays left; the v4.4 layered material system stays right.',
    current: 'Current frosted approach',
    currentHint: 'Blurred content, repeated sampling, background-dependent reading',
    target: 'Noyo v4.4 layered target',
    targetHint: 'Functional glass, continuous overview islands, solid data surface, no nested sampling',
    componentsTitle: 'Material role showcase',
    componentsHint: 'Regular, Clear media, and Solid each serve one role.',
    diagnosticsTitle: 'Live acceptance panel',
    diagnosticsHint: 'Counts only the target scene and its active floating layer.',
    theme: 'Theme',
    background: 'Background',
    material: 'Material',
    transparency: 'Transparency',
    contrast: 'Contrast',
    motion: 'Motion',
    input: 'Input',
    quality: 'Quality',
    scenario: 'Scenario',
    light: 'Light',
    dark: 'Dark',
    system: 'System',
    neutralLight: 'Neutral light',
    neutralDark: 'Neutral dark',
    spectrum: 'Color spectrum',
    highContrast: 'High contrast',
    dense: 'Dense data',
    videoFrame: 'Video frame',
    regular: 'Regular',
    floating: 'Floating',
    clearMedia: 'Clear media',
    solidFallback: 'Solid fallback',
    default: 'Default',
    reduced: 'Reduced transparency',
    increased: 'Increased',
    forced: 'Forced-colors simulation',
    mouse: 'Mouse',
    touch: 'Touch',
    keyboard: 'Keyboard',
    high: 'High',
    balanced: 'Balanced',
    low: 'Low',
    dashboard: 'Dashboard',
    devices: 'Device Management',
    total: 'Total devices',
    online: 'Online',
    offline: 'Offline',
    alarms: 'Alarms today',
    refresh: 'Refresh',
    resourceOverview: 'Resource overview',
    cpu: 'CPU',
    memory: 'Memory',
    services: 'Services',
    aiCopilot: 'AI Copilot',
    running: 'Running',
    device: 'Device',
    product: 'Product',
    protocol: 'Protocol',
    status: 'Status',
    solidContent: 'Reading protection',
    solidContentHint: 'Local readable zones for KPI, table, chart, and form content',
    regularGlass: 'Glass Regular',
    regularGlassHint: 'Navigation, toolbar, and grouped controls',
    clearMediaHint: 'Small controls over controlled media only',
    materialDetails: 'Material details',
    materialDetailsBody: 'This layer is teleported to body and combines a Floating shell with stable content.',
    groups: 'Active Glass Groups',
    backdrops: 'Backdrop sampling layers',
    nested: 'Nested glass groups',
    contentBackdrops: 'Content filter hits',
    fps: 'Average frame rate',
    p95: 'P95 frame time',
    pointerP95: 'Pointer handler P95',
    contrastMetric: 'Key text contrast',
    countUnit: 'count',
    fpsUnit: 'FPS',
    msUnit: 'ms',
    ratioUnit: ':1',
    passStructure: 'Structure passes: the target has no nested glass or content filters.',
    failStructure: 'Structure warning: nested glass or content filters detected.',
    passContrast: 'Readability passes: key text contrast is at least 4.5:1.',
    failContrast: 'Readability warning: key text contrast is below 4.5:1.',
    measuring: 'Collecting the latest five seconds of frame data.',
    passPerformance: 'Performance passes: average frame rate is at least 55 FPS.',
    failPerformance: 'Performance warning: average frame rate is below 55 FPS.',
  },
}

const { locale } = useI18n()
const text = computed(() => copy[String(locale.value).startsWith('zh') ? 'zh' : 'en'])
const systemDark = ref(
  typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches,
)
const settings = reactive({
  theme: 'light',
  background: 'spectrum',
  material: 'regular',
  transparency: 'default',
  contrast: 'default',
  motion: 'default',
  input: 'mouse',
  quality: 'balanced',
  scenario: 'dashboard',
})

const option = (value, label) => ({ value, label })
const controls = computed(() => [
  { key: 'theme', label: text.value.theme, options: [option('light', text.value.light), option('dark', text.value.dark), option('system', text.value.system)] },
  { key: 'background', label: text.value.background, options: [option('neutral-light', text.value.neutralLight), option('neutral-dark', text.value.neutralDark), option('spectrum', text.value.spectrum), option('high-contrast', text.value.highContrast), option('dense', text.value.dense), option('video-frame', text.value.videoFrame)] },
  { key: 'material', label: text.value.material, options: [option('regular', text.value.regular), option('floating', text.value.floating), option('clear-media', text.value.clearMedia), option('solid-fallback', text.value.solidFallback)] },
  { key: 'transparency', label: text.value.transparency, options: [option('default', text.value.default), option('reduced', text.value.reduced)] },
  { key: 'contrast', label: text.value.contrast, options: [option('default', text.value.default), option('increased', text.value.increased), option('forced', text.value.forced)] },
  { key: 'motion', label: text.value.motion, options: [option('default', text.value.default), option('reduced', text.value.reduced)] },
  { key: 'input', label: text.value.input, options: [option('mouse', text.value.mouse), option('touch', text.value.touch), option('keyboard', text.value.keyboard)] },
  { key: 'quality', label: text.value.quality, options: [option('high', text.value.high), option('balanced', text.value.balanced), option('low', text.value.low)] },
  { key: 'scenario', label: text.value.scenario, options: [option('dashboard', text.value.dashboard), option('devices', text.value.devices)] },
])

const resolvedTheme = computed(() => (
  settings.theme === 'system' ? (systemDark.value ? 'dark' : 'light') : settings.theme
))
const backgroundClass = computed(() => 'demo-background--' + settings.background)
const materialProfile = computed(() => (
  settings.material === 'solid-fallback' ? 'regular' : settings.material
))
const demoClasses = computed(() => ({
  'demo-reduced-transparency': settings.transparency === 'reduced',
  'demo-reduced-motion': settings.motion === 'reduced',
  'demo-increased-contrast': settings.contrast === 'increased',
  'demo-forced-colors': settings.contrast === 'forced',
  'demo-solid-fallback': settings.material === 'solid-fallback',
}))
const qualityStyle = computed(() => ({
  '--noyo-glass-regular-blur': settings.quality === 'high' ? '22px' : settings.quality === 'low' ? '10px' : '18px',
  '--noyo-glass-floating-blur': settings.quality === 'high' ? '28px' : settings.quality === 'low' ? '14px' : '24px',
  '--noyo-glass-clear-blur': settings.quality === 'high' ? '14px' : settings.quality === 'low' ? '8px' : '12px',
}))
const scenarioTitle = computed(() => (
  settings.scenario === 'dashboard' ? text.value.dashboard : text.value.devices
))
const fixtureKpis = computed(() => [
  { key: 'total', label: text.value.total, value: 128 },
  { key: 'online', label: text.value.online, value: 126 },
  { key: 'offline', label: text.value.offline, value: 2 },
  { key: 'alarms', label: text.value.alarms, value: 3 },
])
const resourceBars = computed(() => [
  { label: text.value.cpu, value: 42 },
  { label: text.value.memory, value: 63 },
  { label: text.value.services, value: 100 },
])
const demoDevices = computed(() => Array.from({ length: 20 }, (_, index) => {
  const online = index !== 6 && index !== 15
  return {
    code: 'NOYO-' + String(index + 1).padStart(3, '0'),
    name: (String(locale.value).startsWith('zh') ? '环境设备 ' : 'Environment device ') + String(index + 1).padStart(2, '0'),
    product: index % 2 === 0
      ? (String(locale.value).startsWith('zh') ? '温湿度传感器' : 'Temperature sensor')
      : (String(locale.value).startsWith('zh') ? '工业网关' : 'Industrial gateway'),
    protocol: index % 3 === 0 ? 'MQTT' : index % 3 === 1 ? 'Modbus' : 'BACnet',
    online,
    status: online ? text.value.online : text.value.offline,
  }
}))

const demoRoot = ref(null)
const targetStage = ref(null)
const activeGroups = ref(0)
const activeBackdrops = ref(0)
const nestedGroups = ref(0)
const contentBackdrops = ref(0)
const averageFps = ref(0)
const p95FrameTime = ref(0)
const pointerP95 = ref(0)
const keyContrast = ref(0)
const frameSamples = []
const pointerSamples = []
let animationFrame = 0
let lastFrame = 0
let metricsTimer = 0
let mediaQuery

const isVisible = (element) => {
  const style = getComputedStyle(element)
  return style.display !== 'none' && style.visibility !== 'hidden'
}

const hasBackdrop = (element) => {
  const style = getComputedStyle(element)
  const value = style.backdropFilter || style.webkitBackdropFilter || 'none'
  return value !== 'none'
}

const percentile95 = (values) => {
  if (!values.length) return 0
  const sorted = [...values].sort((left, right) => left - right)
  return sorted[Math.max(0, Math.ceil(sorted.length * 0.95) - 1)]
}

const updateDiagnostics = () => {
  const stage = targetStage.value
  if (!stage) return
  const stageGroups = [...stage.querySelectorAll('.noyo-glass-group')].filter(isVisible)
  const floatingGroups = [...document.querySelectorAll('body > [data-liquid-glass-popover]')].filter(isVisible)
  const groups = [...stageGroups, ...floatingGroups]
  const backdropLayers = groups.flatMap((group) => (
    [...group.querySelectorAll(':scope > .noyo-glass-group__backdrop')]
  ))
  const contentTargets = [...stage.querySelectorAll(
    '.noyo-solid-surface, table, th, input, select, .pagination, .page-link',
  )]
  const sample = stage.querySelector('[data-contrast-sample]')
  const sampleText = sample?.querySelector('[data-contrast-text]')

  activeGroups.value = groups.length
  activeBackdrops.value = backdropLayers.filter(hasBackdrop).length
  nestedGroups.value = stageGroups.filter((group) => {
    const parentGroup = group.parentElement?.closest('.noyo-glass-group')
    return parentGroup && stage.contains(parentGroup)
  }).length
  contentBackdrops.value = contentTargets.filter(hasBackdrop).length

  if (sample && sampleText) {
    const sampleGroup = sample.closest('.noyo-glass-group')
    const sampleBackdrop = sampleGroup?.querySelector(':scope > .noyo-glass-group__backdrop')
    const materialFallback = getComputedStyle(demoRoot.value)
      .getPropertyValue('--noyo-glass-island-fallback')
      .trim()
    keyContrast.value = contrastRatioOverLayers(
      getComputedStyle(sampleText).color,
      [
        getComputedStyle(sample).backgroundColor,
        sampleBackdrop ? getComputedStyle(sampleBackdrop).backgroundColor : '',
      ],
      materialFallback,
    )
  }

  const pointerWindow = pointerSamples.filter((item) => performance.now() - item.time <= 5000)
  pointerSamples.splice(0, pointerSamples.length, ...pointerWindow)
  pointerP95.value = percentile95(pointerWindow.map((item) => item.duration))
}

const onTargetPointerMove = (event) => {
  const started = performance.now()
  event.target.closest?.('.noyo-glass-group')
  pointerSamples.push({ time: performance.now(), duration: performance.now() - started })
}

const collectFrame = (timestamp) => {
  if (lastFrame > 0) {
    const frameTime = timestamp - lastFrame
    if (frameTime > 0 && frameTime < 250) {
      frameSamples.push({ time: timestamp, value: frameTime })
    }
  }
  lastFrame = timestamp
  const recent = frameSamples.filter((item) => timestamp - item.time <= 5000)
  frameSamples.splice(0, frameSamples.length, ...recent)
  const summary = summarizeFrameTimes(recent.map((item) => item.value))
  averageFps.value = summary.averageFps
  p95FrameTime.value = summary.p95FrameTime
  animationFrame = requestAnimationFrame(collectFrame)
}

const diagnosticMetrics = computed(() => [
  { key: 'groups', label: text.value.groups, value: activeGroups.value, unit: text.value.countUnit },
  { key: 'backdrops', label: text.value.backdrops, value: activeBackdrops.value, unit: text.value.countUnit },
  { key: 'nested', label: text.value.nested, value: nestedGroups.value, unit: text.value.countUnit },
  { key: 'contentBackdrops', label: text.value.contentBackdrops, value: contentBackdrops.value, unit: text.value.countUnit },
  { key: 'fps', label: text.value.fps, value: averageFps.value, unit: text.value.fpsUnit },
  { key: 'p95', label: text.value.p95, value: p95FrameTime.value, unit: text.value.msUnit },
  { key: 'pointerP95', label: text.value.pointerP95, value: pointerP95.value.toFixed(2), unit: text.value.msUnit },
  { key: 'contrast', label: text.value.contrastMetric, value: keyContrast.value.toFixed(2), unit: text.value.ratioUnit },
])

const warnings = computed(() => {
  const structurePass = nestedGroups.value === 0 && contentBackdrops.value === 0
  const contrastPass = keyContrast.value >= 4.5
  const performancePending = averageFps.value === 0
  const performancePass = averageFps.value >= LIQUID_GLASS_TARGET_FPS
  return [
    {
      key: 'structure',
      level: structurePass ? 'success' : 'danger',
      icon: structurePass ? 'bi-check-circle-fill' : 'bi-exclamation-triangle-fill',
      message: structurePass ? text.value.passStructure : text.value.failStructure,
    },
    {
      key: 'contrast',
      level: contrastPass ? 'success' : 'danger',
      icon: contrastPass ? 'bi-check-circle-fill' : 'bi-exclamation-triangle-fill',
      message: contrastPass ? text.value.passContrast : text.value.failContrast,
    },
    {
      key: 'performance',
      level: performancePending ? 'info' : performancePass ? 'success' : 'danger',
      icon: performancePending ? 'bi-hourglass-split' : performancePass ? 'bi-check-circle-fill' : 'bi-exclamation-triangle-fill',
      message: performancePending ? text.value.measuring : performancePass ? text.value.passPerformance : text.value.failPerformance,
    },
  ]
})

watch(settings, async () => {
  await nextTick()
  updateDiagnostics()
}, { deep: true })

onMounted(() => {
  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  const onSystemThemeChange = (event) => { systemDark.value = event.matches }
  mediaQuery.addEventListener('change', onSystemThemeChange)
  mediaQuery.onNoyoDemoChange = onSystemThemeChange
  targetStage.value?.addEventListener('pointermove', onTargetPointerMove, { passive: true })
  metricsTimer = window.setInterval(updateDiagnostics, 500)
  animationFrame = requestAnimationFrame(collectFrame)
  nextTick(updateDiagnostics)
})

onBeforeUnmount(() => {
  cancelAnimationFrame(animationFrame)
  window.clearInterval(metricsTimer)
  targetStage.value?.removeEventListener('pointermove', onTargetPointerMove)
  if (mediaQuery?.onNoyoDemoChange) {
    mediaQuery.removeEventListener('change', mediaQuery.onNoyoDemoChange)
  }
})
</script>

<style scoped>
.liquid-glass-demo {
  --demo-page-bg: #eef3f9;
  --demo-heading: #172033;
  --demo-muted: #637087;
  --demo-border: rgba(72, 94, 126, 0.18);
  --demo-success: #137a4b;
  --demo-success-bg: rgba(19, 122, 75, 0.10);
  --demo-danger: #b42318;
  --demo-danger-bg: rgba(180, 35, 24, 0.10);
  --demo-info: #2563a7;
  --demo-info-bg: rgba(37, 99, 167, 0.10);
  --demo-media-foreground: #ffffff;
  --demo-frosted-tint: rgba(255, 255, 255, 0.28);
  --demo-frosted-border: rgba(255, 255, 255, 0.64);
  --demo-frosted-shadow: 0 22px 48px -24px rgba(23, 32, 51, 0.46);
  --demo-neutral-light: linear-gradient(135deg, #e7edf5, #f8fafc);
  --demo-neutral-dark: linear-gradient(135deg, #111827, #2d3748);
  --demo-spectrum: radial-gradient(circle at 18% 16%, #4f7cff, transparent 34%), radial-gradient(circle at 82% 74%, #17a398, transparent 32%), linear-gradient(135deg, #3a245f, #d98b3d);
  --demo-high-contrast: linear-gradient(112deg, #ffffff 0 34%, #101828 34% 66%, #ffd447 66%);
  --demo-dense: repeating-linear-gradient(90deg, rgba(45, 75, 115, 0.42) 0 2px, transparent 2px 24px), repeating-linear-gradient(0deg, rgba(45, 75, 115, 0.32) 0 2px, transparent 2px 20px), #cdd9e8;
  --demo-video-frame: linear-gradient(180deg, rgba(8, 16, 28, 0.12), rgba(8, 16, 28, 0.76)), radial-gradient(circle at 72% 28%, #f0ab55, transparent 24%), linear-gradient(135deg, #1c5f77, #17314f);
  --demo-background: var(--demo-spectrum);
  --text-main: var(--demo-heading);
  --text-secondary: var(--demo-muted);
  min-height: 100%;
  padding: 24px;
  color: var(--demo-heading);
  background: var(--demo-page-bg) !important;
}

.liquid-glass-demo[data-bs-theme='dark'] {
  --demo-page-bg: #0d121b;
  --demo-heading: #edf3fb;
  --demo-muted: #9aa8bc;
  --demo-border: rgba(255, 255, 255, 0.12);
  --demo-success: #6ee7a8;
  --demo-success-bg: rgba(110, 231, 168, 0.10);
  --demo-danger: #fda29b;
  --demo-danger-bg: rgba(253, 162, 155, 0.10);
  --demo-info: #8ec5ff;
  --demo-info-bg: rgba(142, 197, 255, 0.10);
  --demo-media-foreground: #ffffff;
  --demo-frosted-tint: rgba(13, 20, 31, 0.34);
  --demo-frosted-border: rgba(255, 255, 255, 0.18);
  --demo-frosted-shadow: 0 26px 56px -24px rgba(0, 0, 0, 0.88);
  --demo-neutral-light: linear-gradient(135deg, #cbd5e1, #f8fafc);
  --demo-neutral-dark: linear-gradient(135deg, #070b12, #1f2937);
  --demo-spectrum: radial-gradient(circle at 18% 16%, #3159b8, transparent 34%), radial-gradient(circle at 82% 74%, #0d766f, transparent 32%), linear-gradient(135deg, #25183d, #704621);
  --demo-high-contrast: linear-gradient(112deg, #f8fafc 0 34%, #020617 34% 66%, #d5a800 66%);
  --demo-dense: repeating-linear-gradient(90deg, rgba(125, 161, 210, 0.30) 0 2px, transparent 2px 24px), repeating-linear-gradient(0deg, rgba(125, 161, 210, 0.24) 0 2px, transparent 2px 20px), #172033;
  --demo-video-frame: linear-gradient(180deg, rgba(2, 6, 14, 0.16), rgba(2, 6, 14, 0.82)), radial-gradient(circle at 72% 28%, #b87132, transparent 24%), linear-gradient(135deg, #16465a, #111f36);
}

.demo-background--neutral-light { --demo-background: var(--demo-neutral-light); }
.demo-background--neutral-dark { --demo-background: var(--demo-neutral-dark); }
.demo-background--spectrum { --demo-background: var(--demo-spectrum); }
.demo-background--high-contrast { --demo-background: var(--demo-high-contrast); }
.demo-background--dense { --demo-background: var(--demo-dense); }
.demo-background--video-frame { --demo-background: var(--demo-video-frame); }

.demo-header,
.demo-section-heading,
.demo-comparison-label,
.demo-target-toolbar :deep(.noyo-glass-group__content),
.demo-target-actions,
.demo-component-card,
.demo-ai-trigger :deep(.noyo-glass-group__content),
.demo-dev-badge {
  display: flex;
  align-items: center;
}

.demo-header {
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}

.demo-header h1 {
  margin: 0 0 8px;
  color: var(--demo-heading);
  font-size: clamp(1.75rem, 3vw, 2.5rem);
  letter-spacing: -0.035em;
}

.demo-header p,
.demo-section-heading p,
.demo-comparison-label small,
.demo-component-card span,
.demo-diagnostic span,
.demo-diagnostic small {
  color: var(--demo-muted);
}

.demo-header > div > p:last-child {
  max-width: 760px;
  margin: 0;
}

.demo-eyebrow,
.demo-kicker {
  margin: 0 0 4px;
  color: var(--color-brand) !important;
  font-size: 0.72rem;
  font-weight: 800;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.demo-dev-badge {
  flex: 0 0 auto;
  gap: 8px;
  padding: 8px 16px;
  border: 1px solid var(--demo-border);
  border-radius: var(--noyo-radius-pill);
  background: var(--noyo-solid-surface);
  color: var(--demo-muted);
  font-size: 0.78rem;
  font-weight: 700;
}

.demo-console {
  margin-bottom: 32px;
}

.demo-console :deep(.noyo-glass-group__content) {
  padding: 24px;
}

.demo-section-heading {
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 16px;
}

.demo-section-heading h2 {
  margin: 0;
  color: var(--demo-heading);
  font-size: 1.05rem;
}

.demo-section-heading > p {
  max-width: 520px;
  margin: 0;
  font-size: 0.85rem;
  text-align: right;
}

.demo-section-heading--outside {
  padding-inline: 4px;
}

.demo-control-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.demo-control {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.demo-control > span {
  color: var(--demo-muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.demo-control .form-select {
  min-height: 40px;
  color: var(--demo-heading);
}

.demo-section {
  margin-bottom: 32px;
}

.demo-comparison-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.demo-comparison-card {
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--demo-border);
  border-radius: var(--noyo-radius-panel);
  background: var(--noyo-solid-surface-muted);
}

.demo-comparison-label {
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border-bottom: 1px solid var(--demo-border);
}

.demo-comparison-label span {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-weight: 800;
}

.demo-comparison-label--current span { color: var(--demo-danger); }
.demo-comparison-label--target span { color: var(--demo-success); }

.demo-scene {
  min-height: 500px;
  padding: 16px;
  background: var(--demo-background);
  background-position: center;
  background-size: cover;
}

.demo-frosted-surface {
  border: 1px solid var(--demo-frosted-border);
  background: var(--demo-frosted-tint);
  box-shadow: var(--demo-frosted-shadow);
  -webkit-backdrop-filter: blur(24px) saturate(1.42);
  backdrop-filter: blur(24px) saturate(1.42);
}

.demo-current-toolbar,
.demo-target-toolbar {
  margin-bottom: 16px;
  border-radius: var(--noyo-radius-glass);
}

.demo-current-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
}

.demo-current-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: var(--noyo-radius-pill);
}

.demo-target-toolbar :deep(.noyo-glass-group__content) {
  justify-content: space-between;
  gap: 16px;
  padding: 8px 16px;
}

.demo-target-actions { gap: 8px; }
.demo-info-trigger { display: inline-flex; padding: 8px; color: var(--demo-heading); }
.demo-popover-copy { display: grid; gap: 8px; }
.demo-popover-copy span { max-width: 280px; color: var(--text-secondary); font-size: 0.82rem; }

.demo-kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
}

.demo-target-kpi-island { margin-bottom: 16px; }

.demo-target-kpi-island,
.demo-target-content-island {
}

.demo-current-kpi,
.demo-target-kpi {
  min-height: 104px;
  padding: 16px;
}

.demo-current-kpi {
  border-radius: var(--noyo-radius-glass);
  transition: transform var(--noyo-duration-standard) var(--noyo-ease-standard);
}

.demo-current-kpi:hover { transform: scale(1.035); }

.demo-current-kpi span,
.demo-target-kpi span {
  display: block;
  min-height: 36px;
  color: var(--demo-muted);
  font-size: 0.72rem;
}

.demo-current-kpi strong,
.demo-target-kpi strong {
  color: var(--demo-heading);
  font-family: var(--font-mono, ui-monospace);
  font-size: 1.55rem;
}

.demo-current-panel,
.demo-current-table {
  min-height: 238px;
  padding: 24px;
  border-radius: var(--noyo-radius-glass);
}

.demo-panel-title {
  margin-bottom: 24px;
  color: var(--demo-heading);
  font-weight: 800;
}

.demo-bar-list { display: grid; gap: 16px; }
.demo-bar-list > div { display: grid; grid-template-columns: 72px 1fr; align-items: center; gap: 16px; }
.demo-bar-list span { color: var(--demo-muted); font-size: 0.78rem; font-weight: 700; }
.demo-bar-list div > div { height: 8px; overflow: hidden; border-radius: var(--noyo-radius-pill); background: var(--noyo-solid-surface-muted); }
.demo-bar-list i { display: block; height: 100%; border-radius: inherit; background: var(--color-brand); }

.demo-dashboard-panels {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 184px;
}

.demo-dashboard-panels :deep(.noyo-glass-group__content) {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 184px;
}

.demo-target-panel { min-height: 238px; padding: 24px; }

.demo-ai-trigger {
  min-height: 238px;
  border-width: 0;
  border-radius: 0;
}

.demo-ai-trigger {
  display: grid;
  grid-template-columns: auto 1fr auto;
  align-items: center;
  gap: 16px;
  padding: 24px;
}

.demo-ai-trigger > i:first-child {
  color: var(--color-brand);
  font-size: 1.5rem;
}

.demo-ai-trigger span { display: grid; gap: 4px; }
.demo-ai-trigger small { color: var(--demo-success); }

.demo-current-table table,
.demo-target-table table {
  width: 100%;
  border-collapse: collapse;
}

.demo-current-table th,
.demo-current-table td,
.demo-target-table th,
.demo-target-table td {
  padding: 8px 16px;
  border-bottom: 1px solid var(--demo-border);
  text-align: left;
  font-size: 0.75rem;
}

.demo-target-table { height: 300px; }
.demo-table-scroll {
  height: 100%;
  margin: 0;
  overflow: auto;
  border: 0;
  border-radius: 0;
  background: var(--noyo-color-transparent);
}
.demo-target-table th { position: sticky; top: 0; z-index: 1; background: var(--noyo-solid-surface-muted); }
.demo-target-table td strong,
.demo-target-table td small { display: block; }
.demo-target-table td small { color: var(--demo-muted); }

.demo-protocol-badge,
.demo-status {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 8px;
  border-radius: var(--noyo-radius-pill);
  font-size: 0.7rem;
  font-weight: 700;
}

.demo-protocol-badge { color: var(--demo-info); background: var(--demo-info-bg); }
.demo-status--online { color: var(--demo-success); background: var(--demo-success-bg); }
.demo-status--offline { color: var(--demo-muted); background: var(--noyo-solid-surface-muted); }

.demo-component-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.demo-component-card {
  min-height: 136px;
  gap: 16px;
  padding: 24px;
}

.demo-component-card > i,
.demo-component-card :deep(.noyo-glass-group__content) > i {
  color: var(--color-brand);
  font-size: 1.5rem;
}

.demo-component-card :deep(.noyo-glass-group__content) {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px;
}

.demo-component-card div { display: grid; gap: 8px; }

.demo-media-swatch {
  display: flex;
  align-items: flex-end;
  min-height: 136px;
  padding: 16px;
  border-radius: var(--noyo-radius-glass);
  background: var(--demo-background);
}

.demo-media-control :deep(.noyo-glass-group__content) {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
}

.demo-media-control { color: var(--demo-media-foreground); }

.demo-diagnostics {
  height: auto;
  padding: 24px;
}

.demo-diagnostic-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.demo-diagnostic {
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--demo-border);
  border-radius: var(--noyo-radius-control);
  background: var(--noyo-solid-surface-muted);
}

.demo-diagnostic span { display: block; min-height: 34px; font-size: 0.72rem; }
.demo-diagnostic strong { color: var(--demo-heading); font-family: var(--font-mono, ui-monospace); font-size: 1.35rem; }
.demo-diagnostic small { margin-left: 5px; font-size: 0.66rem; }

.demo-warning-list {
  display: grid;
  gap: 8px;
  margin-top: 16px;
}

.demo-warning-list > div {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: var(--noyo-radius-control);
  font-size: 0.78rem;
  font-weight: 650;
}

.demo-warning--success { color: var(--demo-success); background: var(--demo-success-bg); }
.demo-warning--danger { color: var(--demo-danger); background: var(--demo-danger-bg); }
.demo-warning--info { color: var(--demo-info); background: var(--demo-info-bg); }

.demo-reduced-transparency :deep(.noyo-glass-group__backdrop),
.demo-solid-fallback :deep(.noyo-glass-group__backdrop),
.demo-forced-colors :deep(.noyo-glass-group__backdrop) {
  background: var(--noyo-glass-fallback);
  -webkit-backdrop-filter: none !important;
  backdrop-filter: none !important;
}

.demo-reduced-transparency :deep(.noyo-glass-group__tint),
.demo-solid-fallback :deep(.noyo-glass-group__tint),
.demo-forced-colors :deep(.noyo-glass-group__tint) {
  display: none;
}

.demo-increased-contrast :deep(.noyo-glass-group__rim),
.demo-increased-contrast :deep(.noyo-solid-surface),
.demo-forced-colors :deep(.noyo-glass-group__rim),
.demo-forced-colors :deep(.noyo-solid-surface) {
  border-width: 2px !important;
  border-color: var(--demo-heading) !important;
}

.demo-reduced-motion *,
.demo-reduced-motion *::before,
.demo-reduced-motion *::after {
  scroll-behavior: auto !important;
  transition: none !important;
  animation: none !important;
}

@media (max-width: 1199.98px) {
  .demo-control-grid,
  .demo-diagnostic-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .demo-comparison-grid { grid-template-columns: 1fr; }
}

@media (max-width: 767.98px) {
  .liquid-glass-demo { padding: 24px; }
  .demo-header,
  .demo-section-heading { align-items: flex-start; flex-direction: column; }
  .demo-section-heading > p { text-align: left; }
  .demo-control-grid,
  .demo-component-grid { grid-template-columns: 1fr; }
  .demo-kpi-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .demo-dashboard-panels,
  .demo-dashboard-panels :deep(.noyo-glass-group__content) { grid-template-columns: 1fr; }
  .demo-ai-trigger { border-width: 1px 0 0; }
  .demo-ai-trigger { min-height: 112px; }
}

@media (max-width: 479.98px) {
  .demo-diagnostic-grid { grid-template-columns: 1fr; }
  .demo-target-toolbar :deep(.noyo-glass-group__content) { align-items: flex-start; flex-direction: column; }
}
</style>
