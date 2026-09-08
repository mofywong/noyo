import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  resolveGlassGroupPolicy,
  resolveLiquidGlassPolicy,
  resolveSolidSurfacePolicy,
} from '../src/utils/liquidGlassPolicy.js'
import {
  normalizeGlassPointer,
  resolveGlassPointerState,
} from '../src/utils/glassPointer.js'
import {
  LIQUID_GLASS_TARGET_FPS,
  contrastRatio,
  contrastRatioOverLayers,
  parseCssColor,
  summarizeFrameTimes,
} from '../src/utils/liquidGlassDemoMetrics.js'
import * as liquidGlassDemoMetrics from '../src/utils/liquidGlassDemoMetrics.js'

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const readFrontendSource = (relativePath) => readFileSync(
  path.join(frontendRoot, relativePath),
  'utf8',
)

const regular = resolveGlassGroupPolicy()
assert.equal(LIQUID_GLASS_TARGET_FPS, 55)

const resolveHardwareGpuRendererPolicy = liquidGlassDemoMetrics.resolveHardwareGpuRendererPolicy
assert.equal(
  typeof resolveHardwareGpuRendererPolicy,
  'function',
  'The GPU gate must expose a testable fail-closed renderer policy',
)

if (typeof resolveHardwareGpuRendererPolicy === 'function') {
  const hardwareRenderers = [
    {
      vendor: 'NVIDIA Corporation',
      renderer: 'ANGLE (NVIDIA, NVIDIA GeForce RTX 4060 Laptop GPU, D3D11)',
    },
    {
      vendor: 'Intel Inc.',
      renderer: 'ANGLE (Intel, Intel Iris Xe Graphics, D3D11)',
    },
  ]
  for (const renderer of hardwareRenderers) {
    assert.equal(
      resolveHardwareGpuRendererPolicy({
        debugInfoAvailable: true,
        ...renderer,
      }).hardware,
      true,
      renderer.renderer,
    )
  }

  assert.equal(
    resolveHardwareGpuRendererPolicy({
      debugInfoAvailable: false,
      vendor: 'NVIDIA Corporation',
      renderer: 'NVIDIA GeForce RTX 4060',
    }).hardware,
    false,
    'The gate must fail closed when WEBGL_debug_renderer_info is unavailable',
  )
  assert.equal(
    resolveHardwareGpuRendererPolicy({
      debugInfoAvailable: true,
      vendor: 'WebKit',
      renderer: 'WebKit WebGL',
    }).hardware,
    false,
    'A generic WebGL renderer is not positive hardware evidence',
  )

  const softwareRenderers = [
    'Google SwiftShader',
    'ANGLE (Microsoft, Microsoft Basic Render Driver, D3D11)',
    'ANGLE (Microsoft, WARP, D3D11)',
    'llvmpipe (LLVM 18.1.8)',
    'Mesa lavapipe',
    'Mesa softpipe',
    'Software Rasterizer',
    'GDI Generic',
  ]
  for (const renderer of softwareRenderers) {
    assert.equal(
      resolveHardwareGpuRendererPolicy({
        debugInfoAvailable: true,
        vendor: 'Software Adapter',
        renderer,
      }).hardware,
      false,
      renderer,
    )
  }
}

const meetsLiquidGlassGpuPerformanceGate = liquidGlassDemoMetrics.meetsLiquidGlassGpuPerformanceGate
assert.equal(
  typeof meetsLiquidGlassGpuPerformanceGate,
  'function',
  'The GPU performance boundary must be testable independently of one hardware run',
)
if (typeof meetsLiquidGlassGpuPerformanceGate === 'function') {
  assert.equal(meetsLiquidGlassGpuPerformanceGate({ fps: 55, p95: 19 }), true)
  assert.equal(
    meetsLiquidGlassGpuPerformanceGate({ fps: 55, p95: 20 }),
    false,
    'The p95 boundary is strictly below 20ms',
  )
  assert.equal(meetsLiquidGlassGpuPerformanceGate({ fps: 54, p95: 17 }), false)
}
assert.equal(regular.profile, 'regular')
assert.equal(regular.shape, 'rounded')
assert.deepEqual(regular.classes, [
  'noyo-glass-group',
  'noyo-glass-group--regular',
  'noyo-glass-group--shape-rounded',
  'noyo-glass-group--control',
  'noyo-glass-group--static',
])

const floating = resolveGlassGroupPolicy({
  profile: 'floating',
  shape: 'pill',
  interactive: true,
})
assert.deepEqual(floating.classes, [
  'noyo-glass-group',
  'noyo-glass-group--floating',
  'noyo-glass-group--shape-pill',
  'noyo-glass-group--control',
  'noyo-glass-group--interactive',
])

const island = resolveGlassGroupPolicy({
  composition: 'island',
  profile: 'regular',
  shape: 'panel',
  interactive: true,
})
assert.equal(island.composition, 'island')
assert.ok(island.classes.includes('noyo-glass-group--island'))

const invalidComposition = resolveGlassGroupPolicy({ composition: 'card' })
assert.equal(invalidComposition.composition, 'control')
assert.ok(invalidComposition.classes.includes('noyo-glass-group--control'))

const clearMedia = resolveGlassGroupPolicy({
  profile: 'clear-media',
  shape: 'panel',
})
assert.equal(clearMedia.profile, 'clear-media')
assert.ok(clearMedia.classes.includes('noyo-glass-group--clear-media'))

const unsupportedGlass = resolveGlassGroupPolicy({
  profile: 'content',
  shape: 'unbounded-custom-shape',
})
assert.equal(unsupportedGlass.profile, 'regular')
assert.equal(unsupportedGlass.shape, 'rounded')
assert.ok(!unsupportedGlass.classes.some((name) => name.includes('content')))
assert.ok(!unsupportedGlass.classes.some((name) => name.includes('lensed')))

const legacyContent = resolveLiquidGlassPolicy({ profile: 'content' })
assert.equal(legacyContent.kind, 'solid')
assert.equal(legacyContent.profile, 'solid-content')
assert.ok(legacyContent.classes.includes('noyo-solid-surface'))

const legacySolid = resolveLiquidGlassPolicy({ profile: 'solid', opaque: true })
assert.equal(legacySolid.kind, 'solid')
assert.equal(legacySolid.profile, 'solid-content')

const legacyChrome = resolveLiquidGlassPolicy({ profile: 'chrome' })
assert.equal(legacyChrome.kind, 'glass')
assert.equal(legacyChrome.profile, 'regular')
assert.ok(legacyChrome.classes.includes('noyo-glass-group--regular'))

const solid = resolveSolidSurfacePolicy({
  density: 'compact',
  elevation: 'flat',
  interactive: true,
})
assert.deepEqual(solid.classes, [
  'noyo-solid-surface',
  'noyo-solid-surface--compact',
  'noyo-solid-surface--flat',
  'noyo-solid-surface--interactive',
])

const solidFallback = resolveSolidSurfacePolicy({
  density: 'unbounded-density',
  elevation: 'unbounded-elevation',
})
assert.equal(solidFallback.density, 'comfortable')
assert.equal(solidFallback.elevation, 'raised')

assert.deepEqual(
  normalizeGlassPointer(
    { clientX: 20, clientY: 110 },
    { left: 10, top: 10, width: 200, height: 100 },
  ),
  { x: 5, y: 100 },
)
assert.deepEqual(
  normalizeGlassPointer(
    { clientX: -500, clientY: 500 },
    { left: 0, top: 0, width: 100, height: 100 },
  ),
  { x: 0, y: 100 },
)
assert.deepEqual(
  normalizeGlassPointer(
    { clientX: 20, clientY: 20 },
    { left: 0, top: 0, width: 0, height: 0 },
  ),
  { x: 50, y: 50 },
)

const centerPointer = resolveGlassPointerState(
  { clientX: 110, clientY: 60 },
  { left: 10, top: 10, width: 200, height: 100 },
)
assert.deepEqual(centerPointer, {
  x: 50,
  y: 50,
  directionX: 0,
  directionY: 0,
  energy: 0,
  edgeEnergy: 0,
})

const edgePointer = resolveGlassPointerState(
  { clientX: 210, clientY: 110 },
  { left: 10, top: 10, width: 200, height: 100 },
)
assert.equal(edgePointer.energy, 1)
assert.equal(edgePointer.edgeEnergy, 1)

assert.deepEqual(parseCssColor('rgb(15, 23, 42)'), {
  r: 15,
  g: 23,
  b: 42,
  a: 1,
})
assert.deepEqual(parseCssColor('rgba(255, 255, 255, 0.75)'), {
  r: 255,
  g: 255,
  b: 255,
  a: 0.75,
})
assert.deepEqual(parseCssColor('#0f172a'), {
  r: 15,
  g: 23,
  b: 42,
  a: 1,
})
assert.equal(parseCssColor('transparent'), null)
assert.equal(Number(contrastRatio('#ffffff', '#000000').toFixed(2)), 21)
assert.equal(Number(contrastRatio('#0f172a', '#ffffff').toFixed(2)), 17.85)
assert.equal(
  Number(contrastRatioOverLayers(
    '#9aa8bc',
    ['rgba(0, 0, 0, 0)', '#151e2a'],
    '#ffffff',
  ).toFixed(2)),
  Number(contrastRatio('#9aa8bc', '#151e2a').toFixed(2)),
)
assert.ok(
  contrastRatioOverLayers(
    '#9aa8bc',
    ['rgba(0, 0, 0, 0)', 'rgba(13, 21, 33, 0.18)'],
    '#151e2a',
  ) >= 4.5,
)
assert.deepEqual(summarizeFrameTimes([16, 17, 18, 20]), {
  averageFps: 56,
  p95FrameTime: 20,
})
assert.deepEqual(summarizeFrameTimes([]), {
  averageFps: 0,
  p95FrameTime: 0,
})

const deviceListSource = readFrontendSource('src/views/DeviceList.vue')
assert.match(deviceListSource, /import LiquidGlassPopover from/)
assert.equal(
  (deviceListSource.match(/<LiquidGlassPopover\b/g) || []).length,
  2,
  'Device filters and row actions must use the shared floating popover primitive while secondary actions remain inline',
)
assert.doesNotMatch(deviceListSource, /noyo-glass-popover/)
assert.doesNotMatch(deviceListSource, /activeDeviceActionMenu|deviceActionMenuPosition|showFilters/)

const popoverSource = readFrontendSource('src/components/liquid-glass/LiquidGlassPopover.vue')
assert.match(popoverSource, /<Teleport to="body">/)
assert.match(popoverSource, /<slot :close="close"\s*\/>/)
assert.match(popoverSource, /noyo-liquid-popover-open/)

const appSource = readFrontendSource('src/App.vue')
assert.match(appSource, /const modalElement = document\.getElementById\('forceChangePasswordModal'\)/)
assert.match(appSource, /if \(!modalElement\) return/)
assert.match(appSource, /watch\(shouldLoadShellData,[\s\S]*?flush:\s*'post'/)

const dashboardSource = readFrontendSource('src/views/Dashboard.vue')
const dashboardFilterSource = readFrontendSource('src/components/liquid-glass/DashboardLiquidGlassFilters.vue')
assert.equal(
  (dashboardFilterSource.match(/<feDisplacementMap\b/g) || []).length,
  3,
  'Dashboard filter template must keep exactly three RGB displacement taps',
)
assert.equal(
  (dashboardFilterSource.match(/id: 'noyo-dashboard-(?:kpi|panel)-refraction'/g) || []).length,
  2,
  'Dashboard must keep exactly one bounded filter preset per card geometry',
)
assert.doesNotMatch(
  dashboardFilterSource,
  /<feTurbulence\b/,
  'Dashboard refraction must use deterministic SDF maps rather than animated noise',
)
assert.match(
  dashboardSource,
  /\.dashboard-refraction-ready \.dashboard-kpi-card :deep\(\.noyo-glass-group__backdrop\)\s*\{[\s\S]*?filter:\s*url\(#noyo-dashboard-kpi-refraction\);[\s\S]*?backdrop-filter:\s*blur\(0\.5px\)/,
  'Dashboard KPI cards must sample the backdrop before applying SVG refraction',
)
assert.match(
  dashboardSource,
  /\.dashboard-refraction-ready \.dashboard-content-card :deep\(\.noyo-glass-group__backdrop\)\s*\{[\s\S]*?filter:\s*url\(#noyo-dashboard-panel-refraction\);[\s\S]*?backdrop-filter:\s*blur\(0\.75px\)/,
  'Dashboard content cards must sample the backdrop before applying SVG refraction',
)
assert.match(
  dashboardSource,
  /\.dashboard-card :deep\(\.noyo-glass-group__content\)\s*\{[\s\S]*?filter:\s*none;[\s\S]*?transform:\s*none;/,
  'Dashboard text and controls must stay outside the optical distortion layer',
)
assert.match(
  dashboardSource,
  /scaleX\(calc\(1 \+ var\(--noyo-glass-energy\) \* 0\.014\)\)[\s\S]*?scaleY\(calc\(1 - var\(--noyo-glass-energy\) \* 0\.008\)\)/,
  'Dashboard optical layers must stay within the documented 1.4% deformation envelope',
)
assert.match(
  dashboardSource,
  /\.resource-ring\s*\{[\s\S]*?transition:\s*stroke-dasharray\s+var\(--noyo-duration-standard\)/,
)
const liquidGlassStyles = readFrontendSource('src/styles/liquid-glass.css')
for (const token of [
  '--noyo-dashboard-liquid-tint',
  '--noyo-dashboard-liquid-tint-min',
  '--noyo-dashboard-liquid-tint-max',
  '--noyo-dashboard-liquid-content-text',
  '--noyo-dashboard-liquid-backdrop-brightness',
  '--noyo-dashboard-liquid-specular',
  '--noyo-dashboard-liquid-edge',
  '--noyo-dashboard-liquid-caustic',
]) {
  assert.equal(
    (liquidGlassStyles.match(new RegExp(`${token}:`, 'g')) || []).length,
    2,
    `${token} must define paired light and dark theme values`,
  )
}
assert.equal(
  (liquidGlassStyles.match(/--noyo-dashboard-liquid-edge-band:\s*5px/g) || []).length,
  1,
  'Dashboard Liquid Glass must keep the article-inspired five-pixel edge band',
)
const standardDuration = liquidGlassStyles.match(/--noyo-duration-standard:\s*(\d+)ms/)
assert.ok(standardDuration, 'The standard motion token must resolve to milliseconds')
assert.ok(Number(standardDuration[1]) <= 300, 'Standard motion must stay within 300ms')
assert.match(
  dashboardSource,
  /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.resource-ring\s*\{[\s\S]*?transition:\s*none/,
)
assert.match(
  dashboardSource,
  /@media \(prefers-reduced-transparency: reduce\)[\s\S]*?\.dashboard-refraction-ready[\s\S]*?backdrop-filter:\s*none/,
  'Dashboard must provide an opaque non-refractive accessibility fallback',
)

console.log('liquid glass policy tests passed')
