import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { inflateSync } from 'node:zlib'
import {
  contrastRatio,
  meetsLiquidGlassGpuPerformanceGate,
  parseCssColor,
  resolveHardwareGpuRendererPolicy,
} from '../src/utils/liquidGlassDemoMetrics.js'

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const vitePort = Number(process.env.LIQUID_GLASS_TEST_PORT || 5189)
const cdpPort = Number(process.env.LIQUID_GLASS_CDP_PORT || 9337)
const readPathArgument = (name) => process.argv
  .find((argument) => argument.startsWith(name + '='))
  ?.slice(name.length + 1)
const screenshotPath = readPathArgument('--screenshot')
const dashboardScreenshotPath = readPathArgument('--dashboard-screenshot')
const dashboardDarkScreenshotPath = readPathArgument('--dashboard-dark-screenshot')
const deviceScreenshotPath = readPathArgument('--device-screenshot')
const deviceDarkScreenshotPath = readPathArgument('--device-dark-screenshot')
const gpuPerformanceGate = process.argv.includes('--gpu-performance')
const islandTokenNames = [
  '--noyo-glass-island-tint',
  '--noyo-glass-island-border',
  '--noyo-glass-island-highlight',
  '--noyo-glass-island-edge',
  '--noyo-glass-island-caustic',
  '--noyo-glass-island-reading',
  '--noyo-glass-title-shadow',
]
const edgeCandidates = process.platform === 'win32'
  ? [
      process.env.LIQUID_GLASS_BROWSER,
      'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
      'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    ]
  : [process.env.LIQUID_GLASS_BROWSER]
const edgeExecutable = edgeCandidates.find((candidate) => candidate && existsSync(candidate))

const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function waitFor(label, getValue, predicate, timeoutMs = 10000) {
  const deadline = Date.now() + timeoutMs
  let lastValue
  while (Date.now() < deadline) {
    try {
      lastValue = await getValue()
      if (predicate(lastValue)) return lastValue
    } catch {
      // Vite, route, and browser startup races are expected here.
    }
    await delay(100)
  }
  throw new Error(
    'Timed out waiting for ' + label + '; last value: ' + JSON.stringify(lastValue),
  )
}

async function createCdpClient(port) {
  const targets = await waitFor(
    'Edge CDP page target',
    async () => {
      const response = await fetch('http://127.0.0.1:' + port + '/json')
      return response.ok ? response.json() : []
    },
    (items) => Array.isArray(items) && items.some((item) => item.type === 'page'),
  )
  const target = targets.find((item) => item.type === 'page')
  const socket = new WebSocket(target.webSocketDebuggerUrl)
  await new Promise((resolve, reject) => {
    socket.addEventListener('open', resolve, { once: true })
    socket.addEventListener('error', reject, { once: true })
  })

  let sequence = 0
  const pending = new Map()
  const listeners = new Map()
  socket.addEventListener('message', (event) => {
    const message = JSON.parse(event.data)
    if (!message.id) {
      const callbacks = listeners.get(message.method) || []
      callbacks.forEach((callback) => callback(message.params))
      return
    }
    const request = pending.get(message.id)
    if (!request) return
    pending.delete(message.id)
    if (message.error) request.reject(new Error(JSON.stringify(message.error)))
    else request.resolve(message.result)
  })

  const send = (method, params = {}) => new Promise((resolve, reject) => {
    const id = ++sequence
    pending.set(id, { resolve, reject })
    socket.send(JSON.stringify({ id, method, params }))
  })
  const on = (method, callback) => {
    const callbacks = listeners.get(method) || []
    callbacks.push(callback)
    listeners.set(method, callbacks)
  }
  const evaluate = async (expression) => {
    const result = await send('Runtime.evaluate', {
      expression,
      returnByValue: true,
      awaitPromise: true,
    })
    if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails))
    return result.result.value
  }
  const run = (fn, ...args) => {
    const serializedArgs = args.map((value) => JSON.stringify(value)).join(',')
    return evaluate('(' + fn.toString() + ')(' + serializedArgs + ')')
  }
  const navigate = async (url) => {
    await send('Page.navigate', { url })
    await waitFor(
      'document readiness',
      () => evaluate('document.readyState'),
      (state) => state === 'complete',
    )
    await delay(300)
  }

  await send('Page.enable')
  await send('Runtime.enable')
  await send('Network.enable')
  return { socket, send, evaluate, run, navigate, on }
}

function stopProcess(childProcess) {
  if (!childProcess || childProcess.exitCode !== null) return
  if (process.platform === 'win32') {
    spawnSync(
      'taskkill',
      ['/pid', String(childProcess.pid), '/t', '/f'],
      { stdio: 'ignore', windowsHide: true },
    )
    return
  }
  childProcess.kill()
}

function readBackdrop(value) {
  return value && value !== 'none' ? value : 'none'
}

function normalizeBrowserColor(value) {
  const srgb = /^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\)$/i.exec(value)
  if (!srgb) return value
  return {
    r: Number(srgb[1]) * 255,
    g: Number(srgb[2]) * 255,
    b: Number(srgb[3]) * 255,
    a: srgb[4] === undefined ? 1 : Number(srgb[4]),
  }
}

function browserColorAlpha(value) {
  const normalized = normalizeBrowserColor(value)
  const parsed = typeof normalized === 'string' ? parseCssColor(normalized) : normalized
  return parsed?.a
}

function resolveTransformScale(value) {
  const matrix2d = /^matrix\(([^,]+)/.exec(value)
  if (matrix2d) return Number.parseFloat(matrix2d[1])
  const matrix3d = /^matrix3d\(([^,]+)/.exec(value)
  return matrix3d ? Number.parseFloat(matrix3d[1]) : Number.NaN
}

function paethPredictor(left, up, upperLeft) {
  const prediction = left + up - upperLeft
  const leftDistance = Math.abs(prediction - left)
  const upDistance = Math.abs(prediction - up)
  const upperLeftDistance = Math.abs(prediction - upperLeft)
  if (leftDistance <= upDistance && leftDistance <= upperLeftDistance) return left
  if (upDistance <= upperLeftDistance) return up
  return upperLeft
}

function decodePng(base64) {
  const png = Buffer.from(base64, 'base64')
  assert.deepEqual(Array.from(png.subarray(0, 8)), [137, 80, 78, 71, 13, 10, 26, 10])
  let offset = 8
  let width = 0
  let height = 0
  let bitDepth = 0
  let colorType = 0
  let interlace = 0
  const compressed = []
  while (offset < png.length) {
    const length = png.readUInt32BE(offset)
    const type = png.toString('ascii', offset + 4, offset + 8)
    const dataStart = offset + 8
    const dataEnd = dataStart + length
    if (type === 'IHDR') {
      width = png.readUInt32BE(dataStart)
      height = png.readUInt32BE(dataStart + 4)
      bitDepth = png[dataStart + 8]
      colorType = png[dataStart + 9]
      interlace = png[dataStart + 12]
    } else if (type === 'IDAT') {
      compressed.push(png.subarray(dataStart, dataEnd))
    } else if (type === 'IEND') {
      break
    }
    offset = dataEnd + 4
  }
  assert.equal(bitDepth, 8, 'CDP screenshots must use 8-bit PNG channels')
  assert.equal(interlace, 0, 'CDP screenshots must be non-interlaced')
  const channelCount = colorType === 6 ? 4 : colorType === 2 ? 3 : colorType === 0 ? 1 : 0
  assert.ok(channelCount > 0, `Unsupported CDP PNG color type ${colorType}`)
  const inflated = inflateSync(Buffer.concat(compressed))
  const rowStride = width * channelCount
  const rgba = new Uint8Array(width * height * 4)
  let sourceOffset = 0
  let previous = new Uint8Array(rowStride)
  for (let y = 0; y < height; y += 1) {
    const filter = inflated[sourceOffset]
    sourceOffset += 1
    const row = new Uint8Array(rowStride)
    for (let x = 0; x < rowStride; x += 1) {
      const encoded = inflated[sourceOffset]
      sourceOffset += 1
      const left = x >= channelCount ? row[x - channelCount] : 0
      const up = previous[x]
      const upperLeft = x >= channelCount ? previous[x - channelCount] : 0
      let predictor = 0
      if (filter === 1) predictor = left
      else if (filter === 2) predictor = up
      else if (filter === 3) predictor = Math.floor((left + up) / 2)
      else if (filter === 4) predictor = paethPredictor(left, up, upperLeft)
      else assert.equal(filter, 0, `Unsupported PNG row filter ${filter}`)
      row[x] = (encoded + predictor) & 255
    }
    for (let x = 0; x < width; x += 1) {
      const source = x * channelCount
      const target = (y * width + x) * 4
      if (colorType === 0) {
        rgba[target] = row[source]
        rgba[target + 1] = row[source]
        rgba[target + 2] = row[source]
        rgba[target + 3] = 255
      } else {
        rgba[target] = row[source]
        rgba[target + 1] = row[source + 1]
        rgba[target + 2] = row[source + 2]
        rgba[target + 3] = colorType === 6 ? row[source + 3] : 255
      }
    }
    previous = row
  }
  return { width, height, data: rgba }
}

function pixelLuma(image, x, y) {
  const offset = (y * image.width + x) * 4
  return image.data[offset] * 0.2126 + image.data[offset + 1] * 0.7152 + image.data[offset + 2] * 0.0722
}

function sobelEnergy(image, region) {
  let total = 0
  let count = 0
  const left = Math.max(1, Math.floor(region.x))
  const top = Math.max(1, Math.floor(region.y))
  const right = Math.min(image.width - 1, Math.ceil(region.x + region.width))
  const bottom = Math.min(image.height - 1, Math.ceil(region.y + region.height))
  for (let y = top; y < bottom; y += 1) {
    for (let x = left; x < right; x += 1) {
      const gx =
        -pixelLuma(image, x - 1, y - 1) + pixelLuma(image, x + 1, y - 1) +
        -2 * pixelLuma(image, x - 1, y) + 2 * pixelLuma(image, x + 1, y) +
        -pixelLuma(image, x - 1, y + 1) + pixelLuma(image, x + 1, y + 1)
      const gy =
        -pixelLuma(image, x - 1, y - 1) - 2 * pixelLuma(image, x, y - 1) - pixelLuma(image, x + 1, y - 1) +
        pixelLuma(image, x - 1, y + 1) + 2 * pixelLuma(image, x, y + 1) + pixelLuma(image, x + 1, y + 1)
      total += Math.hypot(gx, gy)
      count += 1
    }
  }
  return count ? total / count : 0
}

function meanRgbEdgeBandDifference(first, second, edgeBand = 12) {
  assert.equal(first.width, second.width)
  assert.equal(first.height, second.height)
  let total = 0
  let pixelCount = 0
  for (let y = 0; y < first.height; y += 1) {
    for (let x = 0; x < first.width; x += 1) {
      if (x >= edgeBand && x < first.width - edgeBand && y >= edgeBand && y < first.height - edgeBand) continue
      const offset = (y * first.width + x) * 4
      total += (
        Math.abs(first.data[offset] - second.data[offset]) +
        Math.abs(first.data[offset + 1] - second.data[offset + 1]) +
        Math.abs(first.data[offset + 2] - second.data[offset + 2])
      ) / 3
      pixelCount += 1
    }
  }
  return pixelCount ? total / pixelCount : 0
}

function opticalPixelMetrics(baseline, glass) {
  assert.equal(glass.width, baseline.width)
  assert.equal(glass.height, baseline.height)
  const insetX = Math.max(24, glass.width * 0.24)
  const insetY = Math.max(24, glass.height * 0.24)
  const center = {
    x: insetX,
    y: insetY,
    width: glass.width - insetX * 2,
    height: glass.height - insetY * 2,
  }
  const baselineEnergy = sobelEnergy(baseline, center)
  const glassEnergy = sobelEnergy(glass, center)
  let hasContinuousClipLine = false
  const edgeLines = [
    Array.from({ length: glass.width }, (_, x) => [x, 0]),
    Array.from({ length: glass.width }, (_, x) => [x, glass.height - 1]),
    Array.from({ length: glass.height }, (_, y) => [0, y]),
    Array.from({ length: glass.height }, (_, y) => [glass.width - 1, y]),
  ]
  for (const line of edgeLines) {
    const clipped = line.filter(([x, y]) => {
      const offset = (y * glass.width + x) * 4
      return glass.data[offset + 3] < 250 || (
        glass.data[offset] <= 1 && glass.data[offset + 1] <= 1 && glass.data[offset + 2] <= 1
      )
    }).length
    if (clipped / line.length >= 0.9) hasContinuousClipLine = true
  }
  return {
    centerSobelRetention: baselineEnergy > 0 ? glassEnergy / baselineEnergy : 0,
    hasContinuousClipLine,
  }
}

let viteProcess
let edgeProcess
let cdp
let browserProfile

async function capturePageScreenshot(filePath, fullPage = false) {
  if (!filePath) return
  await cdp.run(() => {
    document.querySelectorAll('.toast-container').forEach((node) => {
      node.style.display = 'none'
    })
    window.scrollTo({ top: 0, behavior: 'instant' })
    return true
  })
  await delay(300)
  const options = {
    format: 'png',
    fromSurface: true,
    captureBeyondViewport: fullPage,
  }
  if (fullPage) {
    const metrics = await cdp.send('Page.getLayoutMetrics')
    const content = metrics.cssContentSize
    options.clip = {
      x: 0,
      y: 0,
      width: Math.ceil(content.width),
      height: Math.ceil(content.height),
      scale: 1,
    }
  }
  const screenshot = await cdp.send('Page.captureScreenshot', options)
  writeFileSync(filePath, Buffer.from(screenshot.data, 'base64'))
}

function minimumContrastAgainstActualBackdrop(image, samples) {
  let minimum = {
    ratio: Number.POSITIVE_INFINITY,
    foreground: null,
    outline: null,
    background: null,
    rect: null,
  }
  for (const sample of samples) {
    const foreground = normalizeBrowserColor(sample.color)
    const outline = sample.outlineWidth >= 0.5
      ? normalizeBrowserColor(sample.outlineColor)
      : null
    const left = Math.max(0, Math.floor(sample.rect.left))
    const top = Math.max(0, Math.floor(sample.rect.top))
    const right = Math.min(image.width, Math.ceil(sample.rect.right))
    const bottom = Math.min(image.height, Math.ceil(sample.rect.bottom))
    for (let y = top; y < bottom; y += 1) {
      for (let x = left; x < right; x += 1) {
        const offset = (y * image.width + x) * 4
        const background = {
          r: image.data[offset],
          g: image.data[offset + 1],
          b: image.data[offset + 2],
          a: image.data[offset + 3] / 255,
        }
        const fillRatio = contrastRatio(foreground, background)
        const outlineRatio = outline ? contrastRatio(outline, background) : 0
        const ratio = Math.max(fillRatio, outlineRatio)
        if (ratio < minimum.ratio) {
          minimum = {
            ratio,
            fillRatio,
            outlineRatio,
            foreground,
            outline,
            background,
            rect: sample.rect,
          }
        }
      }
    }
  }
  return minimum
}

async function captureDashboardBackdropContrast() {
  const sampleState = await cdp.run(() => {
    const hiddenTextElements = new Set()
    const collectTextRuns = (selector) => [...document.querySelectorAll(selector)].flatMap((element) => {
      const walker = document.createTreeWalker(element, NodeFilter.SHOW_TEXT)
      const samples = []
      let textNode = walker.nextNode()
      while (textNode) {
        if (textNode.textContent?.trim()) {
          const range = document.createRange()
          range.selectNodeContents(textNode)
          const rect = range.getBoundingClientRect()
          const textElement = textNode.parentElement || element
          const textStyle = getComputedStyle(textElement)
          const color = textStyle.color
          if (
            rect.width > 0 && rect.height > 0 &&
            rect.left >= 0 && rect.top >= 0 &&
            rect.right <= window.innerWidth && rect.bottom <= window.innerHeight
          ) {
            samples.push({
              color,
              outlineColor: textStyle.webkitTextStrokeColor,
              outlineWidth: Number.parseFloat(textStyle.webkitTextStrokeWidth) || 0,
              rect: {
                left: rect.left,
                top: rect.top,
                right: rect.right,
                bottom: rect.bottom,
              },
            })
            hiddenTextElements.add(textElement)
          }
        }
        textNode = walker.nextNode()
      }
      return samples
    })
    const reading = collectTextRuns(
      '.dashboard-kpi-card .kpi-label, .dashboard-content-card .metric-label, .dashboard-content-card .empty-state, .dashboard-content-card .text-secondary, .dashboard-content-card .card-header, .dashboard-card .text-danger, .dashboard-card .text-success, .dashboard-card .text-warning, .dashboard-card .text-info, .dashboard-card .text-primary',
    )
    const kpi = collectTextRuns('.dashboard-kpi-card .kpi-value')
    const hideStyle = document.createElement('style')
    hideStyle.id = 'dashboard-contrast-text-hide'
    hideStyle.textContent = '.dashboard-contrast-text-hidden{color:transparent!important;-webkit-text-fill-color:transparent!important;-webkit-text-stroke-color:transparent!important;text-decoration-color:transparent!important;text-shadow:none!important}'
    document.head.appendChild(hideStyle)
    hiddenTextElements.forEach((node) => node.classList.add('dashboard-contrast-text-hidden'))
    window.__dashboardContrastHiddenText = [...hiddenTextElements]
    return {
      reading,
      kpi,
      width: window.innerWidth,
      height: window.innerHeight,
    }
  })
  assert.ok(sampleState.reading.length > 0, 'Dashboard must expose visible reading text runs')
  assert.ok(sampleState.kpi.length > 0, 'Dashboard must expose visible KPI text runs')
  try {
    await cdp.run(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    const screenshot = await cdp.send('Page.captureScreenshot', {
      format: 'png',
      fromSurface: true,
      clip: {
        x: 0,
        y: 0,
        width: sampleState.width,
        height: sampleState.height,
        scale: 1,
      },
    })
    const backdrop = decodePng(screenshot.data)
    return {
      reading: minimumContrastAgainstActualBackdrop(backdrop, sampleState.reading),
      kpi: minimumContrastAgainstActualBackdrop(backdrop, sampleState.kpi),
    }
  } finally {
    await cdp.run(() => {
      for (const node of window.__dashboardContrastHiddenText || []) {
        node.classList.remove('dashboard-contrast-text-hidden')
      }
      document.getElementById('dashboard-contrast-text-hide')?.remove()
      delete window.__dashboardContrastHiddenText
      return true
    })
  }
}

try {
  assert.ok(
    edgeExecutable,
    'A local Edge executable is required for the Liquid Glass browser test',
  )

  viteProcess = spawn(
    process.execPath,
    [
      path.join(frontendRoot, 'node_modules', 'vite', 'bin', 'vite.js'),
      '--host',
      '127.0.0.1',
      '--port',
      String(vitePort),
      '--strictPort',
    ],
    { cwd: frontendRoot, stdio: 'ignore', windowsHide: true },
  )
  await waitFor(
    'Vite development server',
    async () => {
      const response = await fetch('http://127.0.0.1:' + vitePort + '/')
      return response.ok
    },
    Boolean,
  )

  browserProfile = mkdtempSync(path.join(os.tmpdir(), 'noyo-liquid-glass-test-'))
  const browserArguments = [
    '--headless=new',
    '--remote-debugging-port=' + cdpPort,
    '--user-data-dir=' + browserProfile,
    '--no-first-run',
    '--disable-extensions',
    '--window-size=1600,1000',
  ]
  if (!gpuPerformanceGate) browserArguments.push('--disable-gpu')
  browserArguments.push('about:blank')
  edgeProcess = spawn(
    edgeExecutable,
    browserArguments,
    { stdio: 'ignore', windowsHide: true },
  )
  cdp = await createCdpClient(cdpPort)
  const fixtureUser = {
    id: 1,
    username: 'liquid-glass-test',
    role: 'admin',
    tenant_id: 0,
    is_system_admin: true,
    must_change_password: false,
  }
  const fixtureDevice = {
    code: 'liquid-glass-device-1',
    name: 'Liquid Glass QA Device',
    product_code: '',
    protocol_name: '',
    enabled: true,
    online: true,
    tags: [],
    config: '{}',
  }
  cdp.on('Fetch.requestPaused', ({ requestId, request }) => {
    const url = new URL(request.url)
    let data = []
    if (url.pathname === '/api/devices' && request.method === 'GET') data = [fixtureDevice]
    else if (url.pathname === '/api/auth/profile') data = fixtureUser
    else if (url.pathname === '/api/setup/status') data = { initialized: true, mode: 'enterprise' }
    else if (url.pathname === '/api/system/stats') {
      data = {
        cpu: 19,
        memory_total: 34091302912,
        memory_used: 25710231552,
        memory_percent: 75,
        disk_total: 1099511627776,
        disk_used: 336167190528,
        disk_percent: 31,
        service_cpu: 28,
        service_memory: 259522560,
        uptime: 6240,
        ip: '198.18.0.1',
        os: 'windows',
        arch: 'amd64',
        version: 'v1.0.0-dev',
        pid: 30788,
        num_goroutine: 879,
        num_gc: 299,
        go_version: 'go1.26.5',
      }
    } else if (url.pathname === '/api/plugins/ai_predict/stats') {
      data = { active_tasks: 0, avg_health: 0, anomaly_count: 0, anomalies: [] }
    }
    const body = Buffer.from(JSON.stringify({ code: 0, data })).toString('base64')
    cdp.send('Fetch.fulfillRequest', {
      requestId,
      responseCode: 200,
      responseHeaders: [{ name: 'Content-Type', value: 'application/json; charset=utf-8' }],
      body,
    }).catch(() => {})
  })
  await cdp.send('Fetch.enable', {
    patterns: [{ urlPattern: '*://127.0.0.1:' + vitePort + '/api/*' }],
  })
  await cdp.send('Emulation.setDeviceMetricsOverride', {
    width: 1728,
    height: 911,
    deviceScaleFactor: 1,
    mobile: false,
    screenWidth: 1728,
    screenHeight: 911,
  })

  await cdp.navigate('http://127.0.0.1:' + vitePort + '/design/liquid-glass')
  const standaloneDemo = await waitFor(
    'unauthenticated standalone Liquid Glass demo',
    () => cdp.run(() => ({
      demo: Boolean(document.querySelector('.liquid-glass-demo')),
      shell: Boolean(document.querySelector('.sidebar, .top-header')),
      currentTotal: document.querySelector(
        '[data-demo-current] [data-fixture-total]',
      )?.textContent?.trim() || '',
      targetTotal: document.querySelector(
        '[data-demo-target] [data-fixture-total]',
      )?.textContent?.trim() || '',
    })),
    (state) => state.demo,
  )
  assert.equal(standaloneDemo.shell, false, 'Development demo must not load the app shell')
  assert.equal(standaloneDemo.currentTotal, standaloneDemo.targetTotal)

  await cdp.navigate('http://127.0.0.1:' + vitePort + '/login')
  const backgroundSvg = [
    '<svg xmlns="http://www.w3.org/2000/svg" width="1600" height="1000">',
    '<rect width="1600" height="1000" fill="#10233f"/>',
    '<circle cx="390" cy="260" r="230" fill="#7c3aed" opacity=".92"/>',
    '<circle cx="1180" cy="680" r="300" fill="#0d9488" opacity=".88"/>',
    '<rect x="610" y="120" width="460" height="760" fill="#f59e0b" opacity=".72"/>',
    '</svg>',
  ].join('')
  const storage = {
    access_token: 'liquid-glass-browser-test',
    refresh_token: 'liquid-glass-browser-test-refresh',
    user_info: JSON.stringify(fixtureUser),
    system_mode: 'enterprise',
    lang: 'zh',
    theme: 'light',
    noyo_custom_bg: 'data:image/svg+xml;base64,' + Buffer.from(backgroundSvg).toString('base64'),
  }
  await cdp.run((entries) => {
    Object.entries(entries).forEach(([key, value]) => localStorage.setItem(key, value))
    return true
  }, storage)

  const forcedPasswordNavigation = await cdp.run(async (user) => {
    const app = document.querySelector('#app')?.__vue_app__
    const router = app?.config.globalProperties.$router
    const pinia = app?.config.globalProperties.$pinia
    const auth = pinia?._s?.get('auth')
    if (!router || !auth) return { ready: false }
    const forcedUser = { ...user, must_change_password: true }
    auth.token = 'liquid-glass-browser-test'
    auth.refreshToken = 'liquid-glass-browser-test-refresh'
    auth.user = forcedUser
    localStorage.setItem('user_info', JSON.stringify(forcedUser))
    await router.push({ name: 'Dashboard' })
    return {
      ready: true,
      route: router.currentRoute.value.name,
    }
  }, fixtureUser)
  assert.equal(forcedPasswordNavigation.ready, true)
  assert.equal(forcedPasswordNavigation.route, 'Dashboard')
  const forcedPasswordModalState = await waitFor(
    'forced-password modal after Login-to-app SPA navigation',
    () => cdp.run(() => {
      const modal = document.getElementById('forceChangePasswordModal')
      return {
        mounted: Boolean(modal),
        visible: modal?.classList.contains('show') || false,
      }
    }),
    (state) => state.mounted && state.visible,
  )
  assert.equal(forcedPasswordModalState.visible, true)

  await cdp.run(() => {
    localStorage.clear()
    return true
  })
  await cdp.navigate('http://127.0.0.1:' + vitePort + '/login')
  await cdp.run((entries) => {
    Object.entries(entries).forEach(([key, value]) => localStorage.setItem(key, value))
    return true
  }, storage)

  await cdp.navigate('http://127.0.0.1:' + vitePort + '/')
  const lightMaterialContract = await waitFor(
    'Light Liquid Glass island material contract',
    () => cdp.run((tokenNames) => {
      const page = document.querySelector('.dashboard-container')
      if (!page) return null
      const probe = document.createElement('div')
      probe.className = 'noyo-glass-group--island'
      probe.hidden = true
      probe.innerHTML = [
        '<div class="noyo-glass-zone" style="backdrop-filter:blur(2px)"></div>',
        '<div class="noyo-glass-reading-zone" style="backdrop-filter:blur(2px)"></div>',
        '<div class="noyo-glass-divider" style="backdrop-filter:blur(2px)"></div>',
        '<button class="noyo-glass-action" style="backdrop-filter:blur(2px)"></button>',
        '<table style="backdrop-filter:blur(2px)"><tbody><tr><td style="backdrop-filter:blur(2px)"></td></tr></tbody></table>',
        '<input style="backdrop-filter:blur(2px)">',
        '<div class="progress" style="backdrop-filter:blur(2px)"></div>',
        '<div class="noyo-glass-chart" style="backdrop-filter:blur(2px)"></div>',
      ].join('')
      page.appendChild(probe)
      const filtered = [...probe.querySelectorAll('*')]
        .map((node) => getComputedStyle(node).backdropFilter)
        .filter((value) => value && value !== 'none')
      const style = getComputedStyle(page)
      const tokens = Object.fromEntries(tokenNames.map((name) => [
        name,
        style.getPropertyValue(name).trim(),
      ]))
      probe.remove()
      return { tokens, filtered }
    }, islandTokenNames),
    (state) => state && Object.values(state.tokens).every(Boolean),
  )
  assert.deepEqual(
    lightMaterialContract.filtered,
    [],
    'Island reading and action descendants must never sample the backdrop',
  )
  const densityTriggerExists = await cdp.run(() => Boolean(
    document.querySelector('[data-liquid-glass-density-trigger]'),
  ))
  assert.equal(
    densityTriggerExists,
    true,
    'The top status bar must expose the Liquid Glass density control',
  )
  await cdp.run(() => {
    document.querySelector('[data-liquid-glass-density-trigger]')?.click()
    return true
  })
  const densityControl = await waitFor(
    'Liquid Glass density range control',
    () => cdp.run(() => {
      const control = document.querySelector('[data-liquid-glass-density-range]')
      return control
        ? {
            value: Number(control.value),
            min: Number(control.min),
            max: Number(control.max),
            ariaValue: Number(control.getAttribute('aria-valuenow')),
            ariaControls: document.querySelector('[data-liquid-glass-density-trigger]')?.getAttribute('aria-controls') || '',
            menuId: control.closest('.liquid-glass-density-menu')?.id || '',
            menuParent: control.closest('.liquid-glass-density-menu')?.parentElement?.tagName || '',
          }
        : null
    }),
    (state) => state?.value === 58,
  )
  assert.deepEqual(densityControl, {
    value: 58,
    min: 0,
    max: 100,
    ariaValue: 58,
    ariaControls: 'liquid-glass-density-menu',
    menuId: 'liquid-glass-density-menu',
    menuParent: 'BODY',
  })
  const densityOpenState = await cdp.run(() => {
    const trigger = document.querySelector('[data-liquid-glass-density-trigger]')
    const control = document.querySelector('[data-liquid-glass-density-range]')
    control?.focus()
    return {
      expanded: trigger?.getAttribute('aria-expanded') || '',
      rangeFocused: document.activeElement === control,
    }
  })
  assert.deepEqual(densityOpenState, { expanded: 'true', rangeFocused: true })
  await cdp.send('Input.dispatchKeyEvent', {
    type: 'keyDown',
    key: 'Escape',
    code: 'Escape',
    windowsVirtualKeyCode: 27,
  })
  await cdp.send('Input.dispatchKeyEvent', {
    type: 'keyUp',
    key: 'Escape',
    code: 'Escape',
    windowsVirtualKeyCode: 27,
  })
  const densityEscapeState = await waitFor(
    'Liquid Glass density Escape close and focus restore',
    () => cdp.run(() => {
      const trigger = document.querySelector('[data-liquid-glass-density-trigger]')
      const menu = document.getElementById('liquid-glass-density-menu')
      return {
        expanded: trigger?.getAttribute('aria-expanded') || '',
        focused: document.activeElement === trigger,
        shown: menu?.classList.contains('show') || false,
      }
    }),
    (state) => state?.expanded === 'false' && state.focused && !state.shown,
  )
  assert.deepEqual(densityEscapeState, { expanded: 'false', focused: true, shown: false })
  await cdp.run(() => {
    document.querySelector('[data-liquid-glass-density-trigger]')?.click()
    return true
  })

  for (const density of [0, 58, 100]) {
    await cdp.run((value) => {
      const control = document.querySelector('[data-liquid-glass-density-range]')
      control.value = String(value)
      control.dispatchEvent(new Event('input', { bubbles: true }))
      return true
    }, density)
    const densityState = await waitFor(
      `Liquid Glass density ${density}`,
      () => cdp.run(() => {
        const control = document.querySelector('[data-liquid-glass-density-range]')
      const root = document.querySelector('[data-liquid-glass-density-root]')
      const style = root ? getComputedStyle(root) : null
      return {
        value: Number(control.value),
        stored: localStorage.getItem('noyo_liquid_glass_density'),
        density: style?.getPropertyValue('--noyo-liquid-glass-density').trim() || '',
        densityPercent: style?.getPropertyValue('--noyo-liquid-glass-density-percent').trim() || '',
      }
      }),
      (state) => state.density === String(density / 100),
    )
    assert.deepEqual(densityState, {
      value: density,
      stored: String(density),
      density: String(density / 100),
      densityPercent: density + '%',
    })
  }

  await cdp.run(() => {
    location.reload()
    return true
  })
  const restoredDensity = await waitFor(
    'persisted Liquid Glass density',
    () => cdp.run(() => {
      const root = document.querySelector('[data-liquid-glass-density-root]')
      const style = root ? getComputedStyle(root) : null
      return {
        dashboard: Boolean(document.querySelector('.dashboard-container')),
        stored: localStorage.getItem('noyo_liquid_glass_density'),
        density: style?.getPropertyValue('--noyo-liquid-glass-density').trim() || '',
      }
    }),
    (state) => state.dashboard && state.density === '1',
  )
  assert.deepEqual(restoredDensity, { dashboard: true, stored: '100', density: '1' })

  const captureDensityMaterial = async (density) => {
    await cdp.run((value) => {
      const control = document.querySelector('[data-liquid-glass-density-range]')
      control.value = String(value)
      control.dispatchEvent(new Event('input', { bubbles: true }))
      return true
    }, density)
    return waitFor(
      `Dashboard density material ${density}`,
      () => cdp.run(() => {
        const root = document.querySelector('[data-liquid-glass-density-root]')
        const card = document.querySelector('[data-dashboard-card]')
        const tint = card?.querySelector(':scope > .noyo-glass-group__tint')
        const backdrop = card?.querySelector(':scope > .noyo-glass-group__backdrop')
        const content = card?.querySelector(':scope > .noyo-glass-group__content')
        const heading = document.querySelector('.dashboard-page-heading .page-title')
        const subtitle = document.querySelector('.dashboard-page-heading .page-subtitle')
        const rim = card?.querySelector(':scope > .noyo-glass-group__rim')
        const rootStyle = root ? getComputedStyle(root) : null
        const tintStyle = tint ? getComputedStyle(tint) : null
        const backdropStyle = backdrop ? getComputedStyle(backdrop) : null
        const contentStyle = content ? getComputedStyle(content) : null
        const headingStyle = heading ? getComputedStyle(heading) : null
        const copyColors = [heading, subtitle, ...document.querySelectorAll('[data-dashboard-card] *')]
          .filter((node) => node && !node.matches('.bi') && [...node.childNodes].some((child) => (
            child.nodeType === Node.TEXT_NODE && child.textContent.trim()
          )))
          .map((node) => getComputedStyle(node).color)
        const rimBandStyle = rim ? getComputedStyle(rim, '::before') : null
        return {
          density: rootStyle?.getPropertyValue('--noyo-liquid-glass-density').trim() || '',
          tint: tintStyle?.backgroundColor || '',
          filter: backdropStyle?.filter || '',
          backdrop: backdropStyle?.backdropFilter || '',
          textShadow: contentStyle?.textShadow || '',
          textStrokeWidth: contentStyle?.webkitTextStrokeWidth || '',
          headingShadow: headingStyle?.textShadow || '',
          copyColors,
          rimBandDisplay: rimBandStyle?.display || '',
          rimBandMask: rimBandStyle?.maskImage || rimBandStyle?.webkitMaskImage || '',
          rimBandWidth: rimBandStyle?.paddingTop || '',
        }
      }),
      (state) => state.density === String(density / 100) && Boolean(state.tint),
    )
  }

  const captureDensityOpticalMetrics = async (density) => {
    await captureDensityMaterial(density)
    const fixture = await cdp.run(() => {
      document.getElementById('dashboard-density-optical-fixture')?.remove()
      document.getElementById('dashboard-density-optical-background')?.remove()
      document.getElementById('dashboard-density-optical-freeze')?.remove()
      const page = document.querySelector('.dashboard-container')
      const source = page?.querySelector('.dashboard-kpi-card[data-dashboard-card]')
      if (!page || !source) return null
      const freeze = document.createElement('style')
      freeze.id = 'dashboard-density-optical-freeze'
      freeze.textContent = '#dashboard-density-optical-fixture,#dashboard-density-optical-fixture *{animation:none!important;transition:none!important}'
      document.head.appendChild(freeze)
      const background = document.createElement('div')
      background.id = 'dashboard-density-optical-background'
      Object.assign(background.style, {
        position: 'fixed',
        left: '420px',
        top: '240px',
        width: '320px',
        height: '180px',
        zIndex: '2147483000',
        backgroundImage: [
          'repeating-linear-gradient(90deg,rgba(248,250,252,.72) 0 4px,rgba(17,24,39,.72) 4px 8px)',
          'linear-gradient(90deg,#6d28d9 0 33.333%,#d97706 33.333% 66.666%,#0f766e 66.666% 100%)',
        ].join(','),
      })
      const glass = source.cloneNode(true)
      glass.id = 'dashboard-density-optical-fixture'
      glass.removeAttribute('data-dashboard-card')
      glass.querySelectorAll('[id]').forEach((node) => node.removeAttribute('id'))
      const content = glass.querySelector(':scope > .noyo-glass-group__content')
      if (content) content.style.visibility = 'hidden'
      Object.assign(glass.style, {
        position: 'fixed',
        left: '420px',
        top: '240px',
        width: '320px',
        height: '180px',
        minHeight: '0',
        margin: '0',
        zIndex: '2147483001',
        pointerEvents: 'none',
        visibility: 'hidden',
      })
      page.append(background, glass)
      const backdrop = glass.querySelector(':scope > .noyo-glass-group__backdrop')
      const rect = glass.getBoundingClientRect()
      return {
        clip: {
          x: rect.left,
          y: rect.top,
          width: rect.width,
          height: rect.height,
          scale: 1,
        },
        filter: backdrop ? getComputedStyle(backdrop).filter : '',
      }
    })
    assert.ok(fixture, `Density ${density} requires an optical fixture`)
    assert.match(
      fixture.filter,
      /url\(["']?#noyo-dashboard-kpi-refraction["']?\)/,
      `Density ${density} must keep the production displacement filter`,
    )
    await cdp.run(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))))
    const baseline = await cdp.send('Page.captureScreenshot', {
      format: 'png',
      fromSurface: true,
      clip: fixture.clip,
    })
    await cdp.run(() => {
      const glass = document.getElementById('dashboard-density-optical-fixture')
      if (glass) glass.style.visibility = 'visible'
      return new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
    })
    const refracted = await cdp.send('Page.captureScreenshot', {
      format: 'png',
      fromSurface: true,
      clip: fixture.clip,
    })
    await cdp.run(() => {
      const backdrop = document.querySelector('#dashboard-density-optical-fixture > .noyo-glass-group__backdrop')
      backdrop?.style.setProperty('-webkit-filter', 'none')
      backdrop?.style.setProperty('filter', 'none')
      return new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
    })
    const plain = await cdp.send('Page.captureScreenshot', {
      format: 'png',
      fromSurface: true,
      clip: fixture.clip,
    })
    await cdp.run(() => {
      document.getElementById('dashboard-density-optical-fixture')?.remove()
      document.getElementById('dashboard-density-optical-background')?.remove()
      document.getElementById('dashboard-density-optical-freeze')?.remove()
      return true
    })
    return {
      density,
      ...opticalPixelMetrics(decodePng(baseline.data), decodePng(refracted.data)),
      edgeRefractionDifference: meanRgbEdgeBandDifference(
        decodePng(refracted.data),
        decodePng(plain.data),
        12,
      ),
    }
  }

  const assertDensityOptics = (theme, metrics) => {
    const minimumCenterRetention = new Map([[0, 0.50], [58, 0.40], [100, 0.30]])
    for (const state of metrics) {
      assert.ok(
        state.edgeRefractionDifference >= 4,
        `${theme} density ${state.density} edge SVG difference ${state.edgeRefractionDifference.toFixed(3)} must be at least 4`,
      )
      assert.ok(
        state.centerSobelRetention >= minimumCenterRetention.get(state.density),
        `${theme} density ${state.density} center retention ${state.centerSobelRetention.toFixed(3)} is below its calibrated floor`,
      )
      assert.equal(
        state.hasContinuousClipLine,
        false,
        `${theme} density ${state.density} must not create a continuous clip line`,
      )
    }
  }

  const lightDensityMaterials = []
  for (const density of [0, 58, 100]) {
    lightDensityMaterials.push(await captureDensityMaterial(density))
  }
  assert.ok(
    browserColorAlpha(lightDensityMaterials[0].tint) < browserColorAlpha(lightDensityMaterials[1].tint) &&
      browserColorAlpha(lightDensityMaterials[1].tint) < browserColorAlpha(lightDensityMaterials[2].tint),
    `Light Dashboard cover alpha must increase with density: ${JSON.stringify(lightDensityMaterials)}`,
  )
  assert.ok(
    lightDensityMaterials.every((state) => (
      /url\(["']?#noyo-dashboard-(?:kpi|panel)-refraction["']?\)/.test(state.filter) &&
      /blur\((?:0\.5|0\.75)px\)/.test(state.backdrop)
    )),
    'The density slider must not disable Dashboard refraction',
  )
  assert.ok(
    lightDensityMaterials.every((state) => (
      state.textShadow === 'none' &&
      Number.parseFloat(state.textStrokeWidth) === 0 &&
      state.headingShadow === 'none' &&
      state.copyColors.length > 0 &&
      state.copyColors.every((color) => color === 'rgb(0, 0, 0)')
    )),
    `Dashboard text must stay shadow-free and stroke-free: ${JSON.stringify(lightDensityMaterials)}`,
  )
  assert.ok(
    lightDensityMaterials.every((state) => (
      state.rimBandDisplay !== 'none' &&
      state.rimBandMask !== 'none' &&
      state.rimBandWidth === '5px'
    )),
    'Dashboard cards must expose the masked liquid edge band at every density',
  )
  const lightDensityOptics = []
  for (const density of [0, 58, 100]) {
    lightDensityOptics.push(await captureDensityOpticalMetrics(density))
  }
  assertDensityOptics('Light Dashboard', lightDensityOptics)
  await captureDensityMaterial(0)
  const lightClearContrast = await captureDashboardBackdropContrast()
  assert.ok(
    lightClearContrast.reading.ratio >= 4.5 && lightClearContrast.kpi.ratio >= 3,
    `Light Dashboard must remain readable at maximum transparency: ${JSON.stringify(lightClearContrast)}`,
  )
  await captureDensityMaterial(58)
  const dashboard = await waitFor(
    'Dashboard v4 Liquid Glass islands',
    () => cdp.run(() => {
      const page = document.querySelector('.dashboard-container')
      if (!page) return null
      const solids = [...page.querySelectorAll('.noyo-solid-surface')]
      const groups = [...page.querySelectorAll('.noyo-glass-group')]
      const islands = [...page.querySelectorAll('.noyo-glass-group--island')]
      const aiTrigger = page.querySelector('[data-dashboard-ai-trigger]')
      const aiRect = aiTrigger?.getBoundingClientRect()
      const aiContent = aiTrigger?.querySelector(':scope > .noyo-glass-group__content')
      const aiContentRect = aiContent?.getBoundingClientRect()
      const title = page.querySelector('.page-title')
      const filteredTargets = [...page.querySelectorAll(
        '.noyo-glass-reading-zone, .noyo-glass-divider, .noyo-glass-action, table, th, td, input, .progress, .noyo-glass-chart',
      )]
      const metricBlocks = [...page.querySelectorAll('.dashboard-content-card--guardian .metric-block')]
      const islandBackdropFilters = islands
        .map((node) => getComputedStyle(node.querySelector(':scope > .noyo-glass-group__backdrop')).backdropFilter)
      const kpiBackdropFilters = [...page.querySelectorAll('.dashboard-kpi-card')]
        .map((node) => {
          const style = getComputedStyle(node.querySelector(':scope > .noyo-glass-group__backdrop'))
          return { backdrop: style.backdropFilter, filter: style.filter }
        })
      const contentBackdropFilters = [...page.querySelectorAll('.dashboard-content-card')]
        .map((node) => {
          const style = getComputedStyle(node.querySelector(':scope > .noyo-glass-group__backdrop'))
          return { backdrop: style.backdropFilter, filter: style.filter }
        })
      const contentLayers = groups.map((node) => {
        const content = node.querySelector(':scope > .noyo-glass-group__content')
        const style = content ? getComputedStyle(content) : null
        return {
          filter: style?.filter || '',
          transform: style?.transform || '',
        }
      })
      const refractionFilters = [...page.querySelectorAll('[data-dashboard-liquid-glass-filters] filter')]
        .map((filter) => ({
          id: filter.id,
          region: {
            x: filter.getAttribute('x'),
            y: filter.getAttribute('y'),
            width: filter.getAttribute('width'),
            height: filter.getAttribute('height'),
          },
          displacements: [...filter.querySelectorAll('feDisplacementMap')].map((node) => ({
            scale: Number(node.getAttribute('scale')),
            x: node.getAttribute('xChannelSelector'),
            y: node.getAttribute('yChannelSelector'),
            result: node.getAttribute('result') || '',
          })),
          matrices: [...filter.querySelectorAll('feColorMatrix')].map((node) => (
            (node.getAttribute('values') || '').replace(/\s+/g, ' ').trim()
          )),
          blendModes: [...filter.querySelectorAll('feBlend')].map((node) => node.getAttribute('mode')),
          composites: [...filter.querySelectorAll('feComposite')].map((node) => ({
            operator: node.getAttribute('operator'),
            result: node.getAttribute('result') || '',
          })),
          mergeInputs: [...filter.querySelectorAll('feMergeNode')].map((node) => node.getAttribute('in') || ''),
        }))
      const islandVisuals = islands.map((node) => {
        const rim = node.querySelector(':scope > .noyo-glass-group__rim')
        const rimStyle = rim ? getComputedStyle(rim) : null
        const rimBandStyle = rim ? getComputedStyle(rim, '::before') : null
        return {
          edgeWidth: Number.parseFloat(getComputedStyle(node).getPropertyValue('--noyo-glass-island-edge-width')),
          rimBorderWidth: Number.parseFloat(rimStyle?.borderTopWidth || ''),
          rimBandDisplay: rimBandStyle?.display || '',
        }
      })
      const dashboardZoneBorders = [...page.querySelectorAll('[data-dashboard-card]')]
        .map((node) => {
          const style = getComputedStyle(node)
          return [style.borderTopWidth, style.borderRightWidth, style.borderBottomWidth, style.borderLeftWidth]
            .map((value) => Number.parseFloat(value))
        })
      const dashboardZoneBackgrounds = [...page.querySelectorAll('[data-dashboard-card]')]
        .map((node) => getComputedStyle(node).backgroundColor)
      const dashboardContentShadows = [...page.querySelectorAll(
        '.dashboard-kpi-card .kpi-label, .dashboard-kpi-card .kpi-value, .dashboard-content-card .card-body',
      )].map((node) => getComputedStyle(node).textShadow)
      const islandMaterial = islands.map((node) => {
        const style = getComputedStyle(node)
        const metricValue = node.querySelector('.kpi-value')
        return {
          tint: style.getPropertyValue('--noyo-glass-tint').trim(),
          textMain: style.getPropertyValue('--text-main').trim(),
          metricValueColor: metricValue ? getComputedStyle(metricValue).color : '',
        }
      })
      const readingTextColors = [...page.querySelectorAll(
        '.dashboard-kpi-card .kpi-label, .dashboard-content-card .metric-label, .dashboard-content-card .empty-state, .dashboard-content-card .text-secondary, .dashboard-content-card .card-header',
      )].map((node) => getComputedStyle(node).color)
      const kpiTextColors = [...page.querySelectorAll('.dashboard-kpi-card .kpi-value')]
        .map((node) => getComputedStyle(node).color)
      return {
        solidCount: solids.length,
        groupCount: groups.length,
        islandCount: islands.length,
        hasKpiIsland: Boolean(page.querySelector('[data-dashboard-kpi-island]')),
        hasContentIsland: Boolean(page.querySelector('[data-dashboard-content-island]')),
        aiTriggerIsAction: aiTrigger?.classList.contains('noyo-glass-action') || false,
        headingCardCount: page.querySelectorAll('.noyo-page-heading-surface').length,
        contentGlassCount: page.querySelectorAll('.lg-glass--content').length,
        dashboardReadingZoneCount: page.querySelectorAll('.noyo-glass-reading-zone').length,
        lensCount: page.querySelectorAll(
          'feDisplacementMap, .lg-glass__refract, .lg-glass__filter',
        ).length,
        globalLensCount: document.querySelectorAll('feDisplacementMap').length,
        filterRegistryCount: page.querySelectorAll('[data-dashboard-liquid-glass-filters]').length,
        filterIds: [...page.querySelectorAll('[data-dashboard-liquid-glass-filters] filter')]
          .map((node) => node.id),
        mapsReady: [...page.querySelectorAll('[data-dashboard-liquid-glass-filters] feImage')]
          .every((node) => (node.getAttribute('href') || '').startsWith('data:image/png')),
        refractionReady: page.classList.contains('dashboard-refraction-ready'),
        dashboardTint: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-tint').trim(),
        dashboardSpecular: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-specular').trim(),
        dashboardCaustic: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-caustic').trim(),
        dashboardEdge: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-edge').trim(),
        pageBackdrop: getComputedStyle(page).backdropFilter,
        headingColor: title ? getComputedStyle(title).color : '',
        headingShadow: title ? getComputedStyle(title).textShadow : '',
        solidBackdrops: solids.map((node) => getComputedStyle(node).backdropFilter),
        groupRootBackdrops: groups.map((node) => getComputedStyle(node).backdropFilter),
        backdropLayerCounts: groups.map((node) => (
          node.querySelectorAll(':scope > .noyo-glass-group__backdrop').length
        )),
        islandBackdropFilters,
        kpiBackdropFilters,
        contentBackdropFilters,
        contentLayers,
        refractionFilters,
        islandVisuals,
        dashboardZoneBorders,
        dashboardZoneBackgrounds,
        dashboardContentShadows,
        islandMaterial,
        readingTextColors,
        kpiTextColors,
        nestedGroupCount: groups.filter((node) => (
          node.parentElement?.closest('.noyo-glass-group')
        )).length,
        filteredDescendants: filteredTargets
          .map((node) => getComputedStyle(node).backdropFilter)
          .filter((value) => value && value !== 'none'),
        metricBlocks: metricBlocks.map((node) => {
          const style = getComputedStyle(node)
          return {
            hasReadingZone: node.classList.contains('noyo-glass-reading-zone'),
            background: style.backgroundColor,
            borderTopWidth: style.borderTopWidth,
            textShadow: style.textShadow,
            visibleValue: node.querySelector('.metric-value')?.textContent?.trim() || '',
            visibleLabel: node.querySelector('.metric-label')?.textContent?.trim() || '',
          }
        }),
        aiPointer: aiRect
          ? {
              x: Math.round(aiRect.right - 12),
              y: Math.round(aiRect.top + 12),
            }
          : null,
        aiContentSize: aiContentRect
          ? { width: aiContentRect.width, height: aiContentRect.height }
          : null,
      }
    }),
    (state) => (
      state?.groupCount === 8 &&
      state?.islandCount === 7 &&
      state?.nestedGroupCount === 0 &&
      state?.backdropLayerCounts.every((count) => count === 1) &&
      state?.headingCardCount === 0 &&
      state?.refractionReady
    ),
  )
  assert.equal(dashboard.contentGlassCount, 0, 'Dashboard content must not use a glass profile')
  assert.equal(
    dashboard.dashboardReadingZoneCount,
    0,
    'Dashboard islands must keep one visible glass layer without inner reading panels',
  )
  assert.equal(dashboard.lensCount, 6, 'Dashboard must mount three displacement taps for each geometry')
  assert.equal(dashboard.globalLensCount, 6, 'The application must mount only the six Dashboard displacement taps')
  assert.equal(dashboard.filterRegistryCount, 1, 'Dashboard must mount one page-scoped filter registry')
  assert.deepEqual(
    dashboard.filterIds.sort(),
    ['noyo-dashboard-kpi-refraction', 'noyo-dashboard-panel-refraction'],
  )
  assert.equal(dashboard.mapsReady, true, 'Both SDF displacement maps must be encoded before enabling refraction')
  assert.equal(dashboard.refractionReady, true)
  assert.equal(dashboard.refractionFilters.length, 2)
  assert.ok(
    dashboard.refractionFilters.every((filter) => (
      filter.displacements.length === 3 &&
      filter.displacements.every((tap) => tap.x === 'R' && tap.y === 'G')
    )),
    'Each Dashboard filter must use three RG displacement taps',
  )
  const kpiFilterGraph = dashboard.refractionFilters.find((filter) => filter.id === 'noyo-dashboard-kpi-refraction')
  const panelFilterGraph = dashboard.refractionFilters.find((filter) => filter.id === 'noyo-dashboard-panel-refraction')
  assert.ok(kpiFilterGraph && panelFilterGraph)
  assert.ok(Math.abs(kpiFilterGraph.displacements[0].scale - 18.88) < 0.01)
  assert.ok(Math.abs(kpiFilterGraph.displacements[1].scale - 16) < 0.01)
  assert.ok(Math.abs(kpiFilterGraph.displacements[2].scale - 13.12) < 0.01)
  assert.ok(Math.abs(panelFilterGraph.displacements[0].scale - 25.96) < 0.01)
  assert.ok(Math.abs(panelFilterGraph.displacements[1].scale - 22) < 0.01)
  assert.ok(Math.abs(panelFilterGraph.displacements[2].scale - 18.04) < 0.01)
  assert.deepEqual(kpiFilterGraph.region, { x: '-14%', y: '-24%', width: '128%', height: '148%' })
  assert.deepEqual(panelFilterGraph.region, { x: '-22%', y: '-22%', width: '144%', height: '144%' })
  assert.ok(
    dashboard.refractionFilters.every((filter) => (
      filter.matrices.length >= 4 &&
      filter.blendModes.length >= 3 &&
      filter.blendModes.every((mode) => mode === 'screen') &&
      filter.composites.some((node) => node.operator === 'in' && /inside/.test(node.result)) &&
      filter.composites.some((node) => node.operator === 'out' && /outside/.test(node.result)) &&
      filter.mergeInputs.some((input) => /inside/.test(input)) &&
      filter.mergeInputs.some((input) => /outside/.test(input))
    )),
    'Each Dashboard filter must isolate RGB, merge shape-inside/outside, and screen the specular pass',
  )
  assert.ok(
    dashboard.dashboardTint && dashboard.dashboardSpecular &&
      dashboard.dashboardCaustic && dashboard.dashboardEdge,
    'Light Dashboard Clear Lens tokens must resolve',
  )
  assert.ok(dashboard.readingTextColors.length > 0, 'Light Dashboard must expose normal reading text samples')
  assert.ok(dashboard.kpiTextColors.length > 0, 'Light Dashboard must expose KPI text samples')
  const lightBackdropContrast = await captureDashboardBackdropContrast()
  assert.ok(
    lightBackdropContrast.reading.ratio >= 4.5,
    `Light Dashboard actual-backdrop reading contrast ${JSON.stringify(lightBackdropContrast.reading)} must be at least 4.5:1`,
  )
  assert.ok(
    lightBackdropContrast.kpi.ratio >= 3,
    `Light Dashboard actual-backdrop KPI contrast ${JSON.stringify(lightBackdropContrast.kpi)} must be at least 3:1`,
  )
  assert.equal(readBackdrop(dashboard.pageBackdrop), 'none')
  assert.equal(dashboard.solidCount, 0, 'Dashboard v4 must not fall back to opaque cards')
  assert.equal(dashboard.hasKpiIsland, false)
  assert.equal(dashboard.hasContentIsland, false)
  assert.equal(dashboard.aiTriggerIsAction, false)
  assert.deepEqual(dashboard.filteredDescendants, [])
  assert.ok(
    dashboard.islandMaterial.every((material) => !/rgba\(10,\s*18,\s*30,\s*0\.22\)/.test(material.tint)),
    'Light Dashboard must not apply the dark clear-media tint to a Regular island',
  )
  assert.ok(
    dashboard.islandMaterial.every((material) => !/248,\s*250,\s*252/.test(material.metricValueColor)),
    'Light Dashboard Regular content must resolve to the light-theme foreground instead of media-white text',
  )
  assert.equal(dashboard.metricBlocks.length, 3, 'Dashboard AI Guardian keeps three metric blocks')
  assert.ok(
    dashboard.metricBlocks.every((block) => !block.hasReadingZone),
    'Dashboard metric blocks must be inlays inside the island, not another glass reading layer',
  )
  assert.ok(
    dashboard.metricBlocks.every((block) => block.borderTopWidth === '0px'),
    'Dashboard metric blocks must not draw a second rounded card edge inside the glass island',
  )
  assert.ok(
    dashboard.metricBlocks.every((block) => block.background === 'rgba(0, 0, 0, 0)' || block.background === 'transparent'),
    'Dashboard metric blocks must not add a second surface fill inside the glass island',
  )
  assert.ok(
    dashboard.metricBlocks.every((block) => block.visibleValue && block.visibleLabel),
    'Dashboard metric block text must remain visible after removing the inner surface',
  )
  assert.ok(
    dashboard.metricBlocks.every((block) => block.textShadow === 'none'),
    'Dashboard metric copy must not use text shadows',
  )
  assert.ok(dashboard.headingColor, 'Dashboard title must keep a resolved theme color')
  assert.equal(dashboard.headingShadow, 'none', 'Dashboard title must not use text shadows')
  assert.ok(
    dashboard.solidBackdrops.every((value) => readBackdrop(value) === 'none'),
    'Dashboard solid content must not sample the backdrop',
  )
  assert.ok(
    dashboard.groupRootBackdrops.every((value) => readBackdrop(value) === 'none'),
    'GlassGroup roots must not sample the backdrop',
  )
  assert.ok(
    dashboard.backdropLayerCounts.every((count) => count === 1),
    'Every GlassGroup must own exactly one backdrop sampling layer',
  )
  assert.ok(
    dashboard.kpiBackdropFilters.length === 4 &&
      dashboard.kpiBackdropFilters.every((value) => (
        /url\(["']?#noyo-dashboard-kpi-refraction["']?\)/.test(value.filter) &&
        /blur\(0\.5px\)/.test(value.backdrop)
      )),
    'Every KPI card must use the bounded KPI refraction map',
  )
  assert.ok(
    dashboard.contentBackdropFilters.length === 3 &&
      dashboard.contentBackdropFilters.every((value) => (
        /url\(["']?#noyo-dashboard-panel-refraction["']?\)/.test(value.filter) &&
        /blur\(0\.75px\)/.test(value.backdrop)
      )),
    'Every content card must use the bounded panel refraction map',
  )
  assert.ok(
    dashboard.contentLayers.every((layer) => layer.filter === 'none' && layer.transform === 'none'),
    'Dashboard copy and controls must remain outside all distortion transforms',
  )
  assert.ok(
    dashboard.islandVisuals.every((visual) => (
      Number.isFinite(visual.edgeWidth) && visual.edgeWidth === 1 &&
      visual.rimBorderWidth === 1 &&
      visual.rimBandDisplay !== 'none'
    )),
    'Dashboard islands must keep the hairline edge and expose the inner liquid band',
  )
  assert.ok(
    dashboard.dashboardZoneBorders.every((widths) => widths.every((width) => width === 0)),
    'Dashboard content zones must float on one continuous glass plane without card-like dividers',
  )
  assert.ok(
    dashboard.dashboardZoneBackgrounds.every((value) => value === 'rgba(0, 0, 0, 0)' || value === 'transparent'),
    'Dashboard zones and actions must not tint a second surface inside the continuous glass plane',
  )
  assert.ok(
    dashboard.dashboardContentShadows.length > 0 &&
      dashboard.dashboardContentShadows.every((value) => value === 'none'),
    'Dashboard card copy must remain shadow-free above the optical layers',
  )
  assert.equal(dashboard.nestedGroupCount, 0, 'Dashboard must not nest GlassGroup roots')

  const refractionClip = await cdp.run(() => {
    const freezeStyle = document.createElement('style')
    freezeStyle.id = 'liquid-glass-pixel-freeze'
    freezeStyle.textContent = '*,*::before,*::after{animation-play-state:paused!important;transition:none!important;caret-color:transparent!important}'
    document.head.appendChild(freezeStyle)
    const card = document.querySelector('.dashboard-kpi-card[data-dashboard-card]')
    const rect = card?.getBoundingClientRect()
    return rect
      ? {
          x: Math.max(0, rect.left + window.scrollX),
          y: Math.max(0, rect.top + window.scrollY),
          width: Math.max(1, rect.width),
          height: Math.max(1, rect.height),
          scale: 1,
        }
      : null
  })
  assert.ok(refractionClip, 'A KPI card clip is required for painted refraction verification')
  await delay(100)
  const refractedPixels = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    fromSurface: true,
    clip: refractionClip,
  })
  await cdp.run(() => {
    const card = document.querySelector('.dashboard-kpi-card[data-dashboard-card]')
    const backdrop = card?.querySelector(':scope > .noyo-glass-group__backdrop')
    if (!backdrop) return false
    backdrop.style.setProperty('-webkit-filter', 'none')
    backdrop.style.setProperty('filter', 'none')
    return true
  })
  await cdp.run(() => new Promise((resolve) => {
    requestAnimationFrame(() => requestAnimationFrame(resolve))
  }))
  const plainBlurPixels = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    fromSurface: true,
    clip: refractionClip,
  })
  assert.notEqual(
    refractedPixels.data,
    plainBlurPixels.data,
    'The SVG displacement URL must materially change the painted card pixels',
  )
  const productionRefractionDifference = meanRgbEdgeBandDifference(
    decodePng(refractedPixels.data),
    decodePng(plainBlurPixels.data),
    12,
  )
  assert.ok(
    productionRefractionDifference >= 4,
    `Production card 12px edge-band SVG-only RGB difference ${productionRefractionDifference.toFixed(3)} must be at least 4`,
  )
  await cdp.run(() => {
    const card = document.querySelector('.dashboard-kpi-card[data-dashboard-card]')
    const backdrop = card?.querySelector(':scope > .noyo-glass-group__backdrop')
    backdrop?.style.removeProperty('-webkit-filter')
    backdrop?.style.removeProperty('filter')
    document.getElementById('liquid-glass-pixel-freeze')?.remove()
    return true
  })

  const opticalFixture = await cdp.run(() => {
    const page = document.querySelector('.dashboard-container')
    const source = page?.querySelector('.dashboard-kpi-card[data-dashboard-card]')
    if (!page || !source) return null
    const freeze = document.createElement('style')
    freeze.id = 'dashboard-clear-lens-fixture-freeze'
    freeze.textContent = '#dashboard-clear-lens-fixture,#dashboard-clear-lens-fixture *{animation:none!important;transition:none!important}'
    document.head.appendChild(freeze)

    const background = document.createElement('div')
    background.id = 'dashboard-clear-lens-fixture-background'
    Object.assign(background.style, {
      position: 'fixed',
      left: '420px',
      top: '240px',
      width: '320px',
      height: '180px',
      zIndex: '2147483000',
      backgroundImage: [
        'repeating-linear-gradient(90deg,rgba(248,250,252,.72) 0 4px,rgba(17,24,39,.72) 4px 8px)',
        'linear-gradient(90deg,#6d28d9 0 33.333%,#d97706 33.333% 66.666%,#0f766e 66.666% 100%)',
      ].join(','),
    })

    const glass = source.cloneNode(true)
    glass.id = 'dashboard-clear-lens-fixture'
    glass.removeAttribute('data-dashboard-card')
    glass.querySelectorAll('[id]').forEach((node) => node.removeAttribute('id'))
    const content = glass.querySelector(':scope > .noyo-glass-group__content')
    if (content) content.style.visibility = 'hidden'
    Object.assign(glass.style, {
      position: 'fixed',
      left: '420px',
      top: '240px',
      width: '320px',
      height: '180px',
      minHeight: '0',
      margin: '0',
      zIndex: '2147483001',
      pointerEvents: 'none',
      visibility: 'hidden',
    })
    page.append(background, glass)
    const rect = glass.getBoundingClientRect()
    return {
      clip: {
        x: rect.left,
        y: rect.top,
        width: rect.width,
        height: rect.height,
        scale: 1,
      },
      filter: getComputedStyle(
        glass.querySelector(':scope > .noyo-glass-group__backdrop'),
      ).filter,
    }
  })
  assert.ok(opticalFixture)
  assert.match(opticalFixture.filter, /url\(["']?#noyo-dashboard-kpi-refraction["']?\)/)
  await cdp.run(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))))
  const fixtureBaseline = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    fromSurface: true,
    clip: opticalFixture.clip,
  })
  await cdp.run(() => {
    const fixture = document.getElementById('dashboard-clear-lens-fixture')
    if (fixture) fixture.style.visibility = 'visible'
    return new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)))
  })
  const fixtureGlass = await cdp.send('Page.captureScreenshot', {
    format: 'png',
    fromSurface: true,
    clip: opticalFixture.clip,
  })
  const opticalMetrics = opticalPixelMetrics(
    decodePng(fixtureBaseline.data),
    decodePng(fixtureGlass.data),
  )
  assert.ok(
    opticalMetrics.centerSobelRetention >= 0.40,
    `Adjustable Liquid Glass center Sobel retention ${opticalMetrics.centerSobelRetention.toFixed(3)} must be at least 0.40`,
  )
  assert.equal(opticalMetrics.hasContinuousClipLine, false, 'Clear Lens filter region must not create black or transparent crop lines')
  await cdp.run(() => {
    document.getElementById('dashboard-clear-lens-fixture')?.remove()
    document.getElementById('dashboard-clear-lens-fixture-background')?.remove()
    document.getElementById('dashboard-clear-lens-fixture-freeze')?.remove()
    return true
  })

  await cdp.send('Input.dispatchMouseEvent', {
    type: 'mouseMoved',
    x: dashboard.aiPointer.x,
    y: dashboard.aiPointer.y,
    buttons: 0,
  })
  const pointerState = await waitFor(
    'Dashboard AI GlassGroup pointer response',
    () => cdp.run(() => {
      const group = document.querySelector('[data-dashboard-ai-trigger]')
      const style = group ? getComputedStyle(group) : null
      const content = group?.querySelector(':scope > .noyo-glass-group__content')
      const contentStyle = content ? getComputedStyle(content) : null
      const contentRect = content?.getBoundingClientRect()
      return {
        energized: group?.classList.contains('noyo-glass-group--energized') || false,
        x: Number.parseFloat(style?.getPropertyValue('--noyo-pointer-x') || ''),
        y: Number.parseFloat(style?.getPropertyValue('--noyo-pointer-y') || ''),
        energy: Number.parseFloat(style?.getPropertyValue('--noyo-glass-energy') || ''),
        displacementCount: group?.querySelectorAll('feDisplacementMap').length || 0,
        backdropTransform: group
          ? getComputedStyle(group.querySelector(':scope > .noyo-glass-group__backdrop')).transform
          : '',
        contentTransform: contentStyle?.transform || '',
        contentFilter: contentStyle?.filter || '',
        contentSize: contentRect ? { width: contentRect.width, height: contentRect.height } : null,
      }
    }),
    (state) => (
      state.energized &&
      Number.isFinite(state.energy) &&
      resolveTransformScale(state.backdropTransform) > 1
    ),
  )
  assert.equal(pointerState.displacementCount, 0)
  assert.ok(pointerState.energy > 0, 'Pointer interaction must raise optical energy above zero')
  const pointerScale = resolveTransformScale(pointerState.backdropTransform)
  assert.ok(pointerScale > 1, 'Pointer energy must dynamically scale only the optical layer')
  assert.equal(pointerState.contentTransform, 'none')
  assert.equal(pointerState.contentFilter, 'none')
  assert.ok(Math.abs(pointerState.contentSize.width - dashboard.aiContentSize.width) < 0.5)
  assert.ok(Math.abs(pointerState.contentSize.height - dashboard.aiContentSize.height) < 0.5)

  await cdp.run(() => {
    const trigger = document.querySelector('.lg-popover-trigger')
    trigger?.focus()
    trigger?.click()
    return true
  })
  const floatingPopover = await waitFor(
    'teleported Dashboard floating popover',
    () => cdp.run(() => {
      const popover = document.querySelector('[data-liquid-glass-popover]')
      const page = document.querySelector('.dashboard-container')
      return {
        open: Boolean(popover),
        parentIsBody: popover?.parentElement === document.body,
        nested: Boolean(popover?.parentElement?.closest('.noyo-glass-group')),
        pageGroups: page?.querySelectorAll('.noyo-glass-group').length || 0,
        floatingGroups: document.body.querySelectorAll(
          ':scope > [data-liquid-glass-popover].noyo-glass-group--floating',
        ).length,
        triggerLabel: document.querySelector('.lg-popover-trigger')?.getAttribute('aria-label') || '',
      }
    }),
    (state) => state.open,
  )
  assert.equal(floatingPopover.parentIsBody, true, 'Popover must be teleported to body')
  assert.equal(floatingPopover.nested, false, 'Popover must not create glass-on-glass')
  assert.ok(floatingPopover.triggerLabel, 'Popover icon triggers require an accessible label')
  assert.equal(floatingPopover.pageGroups, 8, 'Dashboard must keep seven card groups plus the page toolbar')
  assert.equal(floatingPopover.floatingGroups, 1, 'An open popover is one independent transient group')

  await cdp.send('Input.dispatchKeyEvent', {
    type: 'keyDown',
    key: 'Escape',
    code: 'Escape',
    windowsVirtualKeyCode: 27,
  })
  const popoverClosed = await waitFor(
    'Dashboard popover focus restoration',
    () => cdp.run(() => ({
      open: Boolean(document.querySelector('[data-liquid-glass-popover]')),
      triggerFocused: document.activeElement?.classList.contains('lg-popover-trigger') || false,
    })),
    (state) => !state.open && state.triggerFocused,
  )
  assert.equal(popoverClosed.triggerFocused, true)
  await capturePageScreenshot(dashboardScreenshotPath)

  await cdp.navigate('http://127.0.0.1:' + vitePort + '/devices')
  const devices = await waitFor(
    'Device Management v4 Liquid Glass islands',
    () => cdp.run(() => {
      const page = document.querySelector('.device-management-page')
      if (!page) return null
      const groups = [...page.querySelectorAll('.noyo-glass-group')]
      const islands = [...page.querySelectorAll('.noyo-glass-group--island')]
      const kpis = [...page.querySelectorAll('.device-kpi-zone')]
      const table = page.querySelector('.device-table-surface')
      const toolbar = page.querySelector('.device-functional-toolbar')
      const title = page.querySelector('.page-header h1')
      const filterTargets = table
        ? [...table.querySelectorAll('table, thead, tbody, tr, th, td, input, select, .list-pagination, .pagination, .page-link, .noyo-glass-reading-zone')]
        : []
      const islandVisuals = islands.map((node) => {
        const backdrop = node.querySelector(':scope > .noyo-glass-group__backdrop')
        const rim = node.querySelector(':scope > .noyo-glass-group__rim')
        const nodeStyle = getComputedStyle(node)
        const rimStyle = rim ? getComputedStyle(rim) : null
        const rimBandStyle = rim ? getComputedStyle(rim, '::before') : null
        return {
          backdrop: backdrop ? getComputedStyle(backdrop).backdropFilter : '',
          edgeWidth: Number.parseFloat(nodeStyle.getPropertyValue('--noyo-glass-island-edge-width')),
          rimBorderWidth: Number.parseFloat(rimStyle?.borderTopWidth || ''),
          rimBandDisplay: rimBandStyle?.display || '',
        }
      })
      const readableText = table?.querySelector('tbody td') || table?.querySelector('thead th')
      return {
        groupCount: groups.length,
        islandCount: islands.length,
        kpiCount: kpis.length,
        kpiBackdrops: kpis.map((node) => getComputedStyle(node).backdropFilter),
        kpiIsland: Boolean(page.querySelector('[data-device-kpi-island]')),
        toolbarIsGlass: toolbar?.classList.contains('noyo-glass-group') || false,
        toolbarBackdropLayers: toolbar?.querySelectorAll(
          ':scope > .noyo-glass-group__backdrop',
        ).length || 0,
        tableIsIsland: table?.classList.contains('noyo-glass-group--island') || false,
        tableIsSolid: table?.classList.contains('noyo-content-surface') || false,
        paginationInsideIsland: Boolean(table?.querySelector('.list-pagination')),
        tableBackdrop: table ? getComputedStyle(table).backdropFilter : '',
        backdropLayerCounts: groups.map((node) => (
          node.querySelectorAll(':scope > .noyo-glass-group__backdrop').length
        )),
        nestedGroupCount: groups.filter((node) => (
          node.parentElement?.closest('.noyo-glass-group')
        )).length,
        headingCardCount: page.querySelectorAll('.noyo-page-heading-surface').length,
        filteredDescendants: filterTargets
          .map((node) => getComputedStyle(node).backdropFilter)
          .filter((value) => value && value !== 'none'),
        lensCount: page.querySelectorAll('feDisplacementMap, .lg-glass__refract').length,
        globalLensCount: document.querySelectorAll('feDisplacementMap').length,
        dashboardUrlEffects: [...document.querySelectorAll('.noyo-glass-group__backdrop')]
          .flatMap((node) => {
            const style = getComputedStyle(node)
            return [style.backdropFilter, style.filter]
          })
          .filter((value) => /noyo-dashboard-/.test(value)),
        pageBackdrop: getComputedStyle(page).backdropFilter,
        headingColor: title ? getComputedStyle(title).color : '',
        headingShadow: title ? getComputedStyle(title).textShadow : '',
        islandVisuals,
        readableTextColor: readableText ? getComputedStyle(readableText).color : '',
      }
    }),
    (state) => (
      state?.groupCount === 2 &&
      state?.islandCount === 1 &&
      state?.kpiCount === 4 &&
      state?.toolbarIsGlass &&
      state?.tableIsSolid &&
      state?.nestedGroupCount === 0 &&
      state?.headingCardCount === 0
    ),
  )
  assert.equal(devices.kpiIsland, true)
  assert.equal(devices.tableIsIsland, false)
  assert.equal(devices.tableIsSolid, true)
  assert.equal(devices.paginationInsideIsland, true, 'Pagination must remain inside the table island')
  assert.ok(devices.kpiBackdrops.every((value) => readBackdrop(value) === 'none'))
  assert.equal(devices.toolbarBackdropLayers, 1)
  assert.equal(readBackdrop(devices.tableBackdrop), 'none')
  assert.ok(devices.backdropLayerCounts.every((count) => count === 1))
  assert.equal(devices.nestedGroupCount, 0)
  assert.deepEqual(devices.filteredDescendants, [])
  assert.equal(devices.lensCount, 0)
  assert.equal(devices.globalLensCount, 0, 'Dashboard filter definitions must unmount after route navigation')
  assert.deepEqual(devices.dashboardUrlEffects, [], 'Non-Dashboard pages must not retain Dashboard filter URLs')
  assert.equal(readBackdrop(devices.pageBackdrop), 'none')
  assert.ok(devices.headingColor, 'Device Management title must keep a resolved theme color')
  assert.notEqual(devices.headingShadow, 'none', 'Device title must keep local reading protection')
  assert.ok(
    devices.islandVisuals.every((visual) => (
      /blur\([\d.]+px\)/.test(visual.backdrop) &&
      Number.isFinite(visual.edgeWidth) && visual.edgeWidth === 1 &&
      visual.rimBorderWidth === 1 &&
      visual.rimBandDisplay === 'none'
    )),
    'Device Light islands must preserve the shared v4.4 backdrop and one-hairline-rim contract',
  )
  assert.ok(devices.readableTextColor, 'Device Light table text must resolve to a readable theme color')

  const exerciseDevicePopover = async (rootSelector, label) => {
    const triggerState = await cdp.run((selector) => {
      const trigger = document.querySelector(selector + ' .lg-popover-trigger')
      trigger?.focus()
      trigger?.click()
      return {
        found: Boolean(trigger),
        controls: trigger?.getAttribute('aria-controls') || '',
        expanded: trigger?.getAttribute('aria-expanded') || '',
      }
    }, rootSelector)
    assert.equal(triggerState.found, true, label + ' trigger must exist')
    assert.ok(triggerState.controls, label + ' trigger requires aria-controls')
    const openState = await waitFor(
      label + ' floating popover',
      () => cdp.run((controls) => {
        const popover = document.getElementById(controls)
        return {
          open: Boolean(popover),
          parentIsBody: popover?.parentElement === document.body,
          floating: popover?.classList.contains('noyo-glass-group--floating') || false,
          nested: Boolean(popover?.parentElement?.closest('.noyo-glass-group')),
          backgroundPaused: document.documentElement.classList.contains('noyo-liquid-popover-open'),
          expanded: document.querySelector('[aria-controls="' + controls + '"]')
            ?.getAttribute('aria-expanded') || '',
        }
      }, triggerState.controls),
      (state) => state.open && state.expanded === 'true',
    )
    assert.equal(openState.parentIsBody, true)
    assert.equal(openState.floating, true)
    assert.equal(openState.nested, false)
    assert.equal(openState.backgroundPaused, true)
    assert.equal(openState.expanded, 'true')
    await cdp.send('Input.dispatchKeyEvent', {
      type: 'keyDown',
      key: 'Escape',
      code: 'Escape',
      windowsVirtualKeyCode: 27,
    })
    const closedState = await waitFor(
      label + ' focus restoration',
      () => cdp.run((selector) => ({
        open: Boolean(document.querySelector('[data-liquid-glass-popover]')),
        triggerFocused: document.activeElement === document.querySelector(
          selector + ' .lg-popover-trigger',
        ),
        backgroundPaused: document.documentElement.classList.contains('noyo-liquid-popover-open'),
      }), rootSelector),
      (state) => !state.open && state.triggerFocused,
    )
    assert.equal(closedState.backgroundPaused, false)
  }
  await exerciseDevicePopover('[data-device-filter-popover]', 'Device filter')
  await exerciseDevicePopover('[data-device-action-popover]', 'Device row action')
  await capturePageScreenshot(deviceScreenshotPath)

  await cdp.run(() => {
    localStorage.setItem('theme', 'dark')
    location.reload()
    return true
  })
  const darkDevices = await waitFor(
    'Dark Device Management Liquid Glass islands',
    () => cdp.run(() => {
      const surface = document.querySelector('[data-device-kpi-island]')
      const style = surface ? getComputedStyle(surface) : null
      const title = document.querySelector('.device-management-page .page-header h1')
      const islands = [...document.querySelectorAll('.device-management-page .noyo-glass-group--island')]
      const tableText = document.querySelector('.device-management-page .device-table-surface tbody td') ||
        document.querySelector('.device-management-page .device-table-surface thead th')
      return {
        surface: style?.backgroundColor || '',
        backdrop: style?.backdropFilter || '',
        headingColor: title ? getComputedStyle(title).color : '',
        islandVisuals: islands.map((node) => {
          const backdrop = node.querySelector(':scope > .noyo-glass-group__backdrop')
          const rim = node.querySelector(':scope > .noyo-glass-group__rim')
          const nodeStyle = getComputedStyle(node)
          const rimStyle = rim ? getComputedStyle(rim) : null
          const rimBandStyle = rim ? getComputedStyle(rim, '::before') : null
          return {
            backdrop: backdrop ? getComputedStyle(backdrop).backdropFilter : '',
            edgeWidth: Number.parseFloat(nodeStyle.getPropertyValue('--noyo-glass-island-edge-width')),
            rimBorderWidth: Number.parseFloat(rimStyle?.borderTopWidth || ''),
            rimBandDisplay: rimBandStyle?.display || '',
          }
        }),
        readableTextColor: tableText ? getComputedStyle(tableText).color : '',
      }
    }),
    (state) => state.surface && state.backdrop === 'none',
  )
  assert.ok(darkDevices.headingColor, 'Dark Device Management title must keep a resolved theme color')
  assert.ok(
    darkDevices.islandVisuals.every((visual) => (
      /blur\([\d.]+px\)/.test(visual.backdrop) &&
      Number.isFinite(visual.edgeWidth) && visual.edgeWidth === 1 &&
      visual.rimBorderWidth === 1 &&
      visual.rimBandDisplay === 'none'
    )),
    'Device Dark islands must preserve the shared v4.4 backdrop and one-hairline-rim contract',
  )
  assert.ok(darkDevices.readableTextColor, 'Device Dark table text must resolve to a readable theme color')
  const darkIslandTokens = await cdp.run((tokenNames) => {
    const page = document.querySelector('.device-management-page')
    const style = page ? getComputedStyle(page) : null
    return Object.fromEntries(tokenNames.map((name) => [
      name,
      style?.getPropertyValue(name).trim() || '',
    ]))
  }, islandTokenNames)
  assert.ok(
    Object.values(darkIslandTokens).every(Boolean),
    'Dark theme must define every Liquid Glass island material token',
  )
  await capturePageScreenshot(deviceDarkScreenshotPath)

  await cdp.navigate('http://127.0.0.1:' + vitePort + '/')
  const darkDashboard = await waitFor(
    'Dark Dashboard v4.4 layered materials',
    () => cdp.run(() => {
      const page = document.querySelector('.dashboard-container')
      if (!page) return null
      const islands = [...page.querySelectorAll('.noyo-glass-group--island')]
      const metricValues = [...page.querySelectorAll('.dashboard-kpi-card .kpi-value')]
      const title = page.querySelector('.page-title')
      const backdropFilters = [...page.querySelectorAll('[data-dashboard-card]')]
        .map((node) => {
          const style = getComputedStyle(node.querySelector(':scope > .noyo-glass-group__backdrop'))
          return { backdrop: style.backdropFilter, filter: style.filter }
        })
      const contentLayers = [...page.querySelectorAll('[data-dashboard-card]')]
        .map((node) => {
          const style = getComputedStyle(node.querySelector(':scope > .noyo-glass-group__content'))
          return { filter: style.filter, transform: style.transform }
        })
      const readingTextColors = [...page.querySelectorAll(
        '.dashboard-kpi-card .kpi-label, .dashboard-content-card .metric-label, .dashboard-content-card .empty-state, .dashboard-content-card .text-secondary, .dashboard-content-card .card-header',
      )].map((node) => getComputedStyle(node).color)
      const kpiTextColors = [...page.querySelectorAll('.dashboard-kpi-card .kpi-value')]
        .map((node) => getComputedStyle(node).color)
      return {
        groupCount: page.querySelectorAll('.noyo-glass-group').length,
        islandCount: islands.length,
        islandTints: islands.map((node) => getComputedStyle(node).getPropertyValue('--noyo-glass-tint').trim()),
        metricColors: metricValues.map((node) => getComputedStyle(node).color),
        headingColor: title ? getComputedStyle(title).color : '',
        headingShadow: title ? getComputedStyle(title).textShadow : '',
        refractionReady: page.classList.contains('dashboard-refraction-ready'),
        dashboardTint: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-tint').trim(),
        dashboardSpecular: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-specular').trim(),
        dashboardCaustic: getComputedStyle(page).getPropertyValue('--noyo-dashboard-liquid-caustic').trim(),
        backdropFilters,
        contentLayers,
        readingTextColors,
        kpiTextColors,
      }
    }),
    (state) => state?.groupCount === 8 && state?.islandCount === 7 && state.metricColors.length === 3,
  )
  const darkDensityMaterials = []
  for (const density of [0, 58, 100]) {
    darkDensityMaterials.push(await captureDensityMaterial(density))
  }
  assert.ok(
    browserColorAlpha(darkDensityMaterials[0].tint) < browserColorAlpha(darkDensityMaterials[1].tint) &&
      browserColorAlpha(darkDensityMaterials[1].tint) < browserColorAlpha(darkDensityMaterials[2].tint),
    `Dark Dashboard cover alpha must increase with density: ${JSON.stringify(darkDensityMaterials)}`,
  )
  assert.ok(
    darkDensityMaterials.every((state) => (
      /url\(["']?#noyo-dashboard-(?:kpi|panel)-refraction["']?\)/.test(state.filter) &&
      state.textShadow === 'none' &&
      Number.parseFloat(state.textStrokeWidth) === 0 &&
      state.headingShadow === 'none' &&
      state.copyColors.length > 0 &&
      state.copyColors.every((color) => color === 'rgb(255, 255, 255)') &&
      state.rimBandDisplay !== 'none' &&
      state.rimBandMask !== 'none' &&
      state.rimBandWidth === '5px'
    )),
    `Dark Dashboard density states must preserve optics and natural text: ${JSON.stringify(darkDensityMaterials)}`,
  )
  const darkDensityOptics = []
  for (const density of [0, 58, 100]) {
    darkDensityOptics.push(await captureDensityOpticalMetrics(density))
  }
  assertDensityOptics('Dark Dashboard', darkDensityOptics)
  await captureDensityMaterial(0)
  const darkClearContrast = await captureDashboardBackdropContrast()
  assert.ok(
    darkClearContrast.reading.ratio >= 4.5 && darkClearContrast.kpi.ratio >= 3,
    `Dark Dashboard must remain readable at maximum transparency: ${JSON.stringify(darkClearContrast)}`,
  )
  await captureDensityMaterial(58)
  assert.ok(
    darkDashboard.islandTints.every((tint) => /rgba\(13,\s*21,\s*33,\s*0\.24\)/.test(tint)),
    'Dark Dashboard Regular islands must resolve to the dark theme material rather than the light or media tint',
  )
  assert.ok(
    darkDashboard.metricColors.every(Boolean),
    'Dark Dashboard metric text must keep a resolved readable foreground',
  )
  assert.ok(darkDashboard.headingColor, 'Dark Dashboard title must keep a resolved theme color')
  assert.equal(darkDashboard.headingShadow, 'none', 'Dark Dashboard title must not use text shadows')
  assert.equal(darkDashboard.refractionReady, true)
  assert.ok(
    darkDashboard.dashboardTint && darkDashboard.dashboardSpecular &&
      darkDashboard.dashboardCaustic,
    'Dark Dashboard Clear Lens tokens must resolve',
  )
  assert.ok(darkDashboard.readingTextColors.length > 0, 'Dark Dashboard must expose normal reading text samples')
  assert.ok(darkDashboard.kpiTextColors.length > 0, 'Dark Dashboard must expose KPI text samples')
  const darkBackdropContrast = await captureDashboardBackdropContrast()
  assert.ok(
    darkBackdropContrast.reading.ratio >= 4.5,
    `Dark Dashboard actual-backdrop reading contrast ${JSON.stringify(darkBackdropContrast.reading)} must be at least 4.5:1`,
  )
  assert.ok(
    darkBackdropContrast.kpi.ratio >= 3,
    `Dark Dashboard actual-backdrop KPI contrast ${JSON.stringify(darkBackdropContrast.kpi)} must be at least 3:1`,
  )
  assert.notEqual(darkDashboard.dashboardTint, dashboard.dashboardTint, 'Dashboard tint must adapt between themes')
  assert.ok(
    darkDashboard.backdropFilters.every((value) => (
      /url\(["']?#noyo-dashboard-(?:kpi|panel)-refraction["']?\)/.test(value.filter) &&
      /blur\((?:0\.5|0\.75)px\)/.test(value.backdrop)
    )),
    'Dark Dashboard must preserve refraction instead of reverting to plain blur',
  )
  assert.ok(
    darkDashboard.contentLayers.every((layer) => layer.filter === 'none' && layer.transform === 'none'),
    'Dark Dashboard copy must remain outside distortion transforms',
  )
  await capturePageScreenshot(dashboardDarkScreenshotPath)

  const unsupportedFallback = await cdp.run(() => {
    const page = document.querySelector('.dashboard-container')
    page?.classList.remove('dashboard-refraction-ready')
    const values = [...(page?.querySelectorAll('[data-dashboard-card]') || [])].map((card) => {
      const style = getComputedStyle(card.querySelector(':scope > .noyo-glass-group__backdrop'))
      return { filter: style.filter, backdrop: style.backdropFilter }
    })
    page?.classList.add('dashboard-refraction-ready')
    return values
  })
  assert.ok(
    unsupportedFallback.every((value) => value.filter === 'none' && /blur\(4px\)/.test(value.backdrop)),
    'Unsupported Dashboard optics must fail closed to the static 4px clear fallback',
  )

  await cdp.send('Emulation.setEmulatedMedia', {
    features: [{ name: 'prefers-reduced-motion', value: 'reduce' }],
  })
  const reducedMotion = await cdp.run(() => [...document.querySelectorAll(
    '[data-dashboard-card] > .noyo-glass-group__backdrop, [data-dashboard-card] > .noyo-glass-group__tint, [data-dashboard-card] > .noyo-glass-group__rim',
  )].map((node) => ({
    transform: getComputedStyle(node).transform,
    transition: getComputedStyle(node).transitionDuration,
  })))
  assert.ok(
    reducedMotion.length > 0 && reducedMotion.every((value) => (
      value.transform === 'none' && Number.parseFloat(value.transition) <= 0.0001
    )),
    `Reduced Motion must disable Dashboard optical deformation and transitions: ${JSON.stringify(reducedMotion)}`,
  )

  await cdp.send('Emulation.setEmulatedMedia', {
    features: [{ name: 'prefers-reduced-transparency', value: 'reduce' }],
  })
  const reducedTransparency = await cdp.run(() => [...document.querySelectorAll(
    '[data-dashboard-card] > .noyo-glass-group__backdrop',
  )].map((node) => {
    const style = getComputedStyle(node)
    return {
      filter: style.filter,
      backdrop: style.backdropFilter,
      background: style.backgroundColor,
    }
  }))
  assert.ok(
    reducedTransparency.length === 7 && reducedTransparency.every((value) => (
      value.filter === 'none' && value.backdrop === 'none' && value.background !== 'rgba(0, 0, 0, 0)'
    )),
    'Reduced Transparency must replace Dashboard refraction with a readable solid surface',
  )

  await cdp.send('Emulation.setEmulatedMedia', {
    features: [{ name: 'forced-colors', value: 'active' }],
  })
  const forcedColors = await cdp.run(() => [...document.querySelectorAll(
    '[data-dashboard-card] > .noyo-glass-group__backdrop, [data-dashboard-card] > .noyo-glass-group__tint',
  )].map((node) => getComputedStyle(node).display))
  assert.ok(
    forcedColors.length > 0 && forcedColors.every((display) => display === 'none'),
    'Forced Colors must remove Dashboard optical backdrop and tint layers',
  )
  await cdp.send('Emulation.setEmulatedMedia', { features: [] })

  await cdp.navigate('http://127.0.0.1:' + vitePort + '/design/liquid-glass')
  const demo = await waitFor(
    'Liquid Glass v4 development demo',
    () => cdp.run(() => {
      const page = document.querySelector('.liquid-glass-demo')
      const title = page?.querySelector('.demo-header h1')
      const targetHint = page?.querySelector('[data-demo-target] .demo-comparison-label small')
      const current = page?.querySelector('[data-demo-current] [data-fixture-total]')
      const target = page?.querySelector('[data-demo-target] [data-fixture-total]')
      const stage = page?.querySelector('.demo-target-stage')
      const groups = [...(stage?.querySelectorAll('.noyo-glass-group') || [])]
      const diagnostics = [...(page?.querySelectorAll('[data-diagnostic]') || [])]
      const values = Object.fromEntries(diagnostics.map((node) => [
        node.getAttribute('data-diagnostic'),
        node.textContent?.trim() || '',
      ]))
      return {
        page: Boolean(page),
        title: title?.textContent?.trim() || '',
        targetHint: targetHint?.textContent?.trim() || '',
        controlCount: page?.querySelectorAll('[data-demo-control]').length || 0,
        currentTotal: current?.textContent?.trim() || '',
        targetTotal: target?.textContent?.trim() || '',
        groupCount: groups.length,
        islandCount: stage?.querySelectorAll('.noyo-glass-group--island').length || 0,
        nestedGroupCount: groups.filter((node) => node.parentElement?.closest('.noyo-glass-group')).length,
        backdropLayerCounts: groups.map((node) => node.querySelectorAll(':scope > .noyo-glass-group__backdrop').length),
        filteredDescendants: stage
          ? [...stage.querySelectorAll('.noyo-glass-zone, .noyo-glass-reading-zone, .noyo-glass-action, table, th, td, input')]
            .map((node) => getComputedStyle(node).backdropFilter)
            .filter((value) => value && value !== 'none')
          : [],
        diagnostics: values,
      }
    }),
    (state) => (
      state.page &&
      state.controlCount >= 8 &&
      state.currentTotal === state.targetTotal &&
      Number(state.diagnostics.fps) > 0
    ),
    15000,
  )
  assert.ok(demo.diagnostics.groups)
  assert.ok(demo.diagnostics.backdrops)
  assert.match(demo.title, /v4/i, 'Development demo must identify the approved v4 direction')
  assert.match(
    demo.targetHint,
    /(分层|layered)/i,
    'Target description must explain the layered material composition',
  )
  assert.equal(demo.groupCount, 3)
  assert.equal(demo.islandCount, 2)
  assert.equal(demo.nestedGroupCount, 0)
  assert.ok(demo.backdropLayerCounts.every((count) => count === 1))
  assert.deepEqual(demo.filteredDescendants, [])
  assert.equal(Number(demo.diagnostics.groups), 3)
  assert.equal(Number(demo.diagnostics.backdrops), 3)
  assert.equal(Number(demo.diagnostics.nested), 0)
  assert.equal(Number(demo.diagnostics.contentBackdrops), 0)
  assert.ok(Number(demo.diagnostics.p95) > 0)

  await cdp.run(() => {
    const select = (key, value) => {
      const control = document.querySelector('[data-demo-control="' + key + '"]')
      if (!control) throw new Error('Missing demo control: ' + key)
      control.value = value
      control.dispatchEvent(new Event('change', { bubbles: true }))
    }
    select('theme', 'dark')
    select('background', 'dense')
    select('material', 'solid-fallback')
    select('transparency', 'reduced')
    select('scenario', 'devices')
    return true
  })
  const changedDemo = await waitFor(
    'Liquid Glass demo control state',
    () => cdp.run(() => {
      const page = document.querySelector('.liquid-glass-demo')
      const target = page?.querySelector('.demo-target-stage')
      const filteredBackdrops = target
        ? [...target.querySelectorAll('.noyo-glass-group__backdrop')]
          .map((node) => getComputedStyle(node).backdropFilter)
          .filter((value) => value && value !== 'none')
        : []
      return {
        theme: page?.getAttribute('data-bs-theme') || '',
        reduced: page?.classList.contains('demo-reduced-transparency') || false,
        fallback: page?.classList.contains('demo-solid-fallback') || false,
        dense: target?.classList.contains('demo-background--dense') || false,
        deviceRows: target?.querySelectorAll('.demo-target-table tbody tr').length || 0,
        groupCount: target?.querySelectorAll('.noyo-glass-group').length || 0,
        islandCount: target?.querySelectorAll('.noyo-glass-group--island').length || 0,
        solidTable: target?.querySelector('[data-demo-target-solid-table]')?.classList.contains('noyo-content-surface') || false,
        contrast: page?.querySelector('[data-diagnostic="contrast"]')?.textContent?.trim() || '',
        filteredBackdrops,
      }
    }),
    (state) => (
      state.theme === 'dark' &&
      state.reduced &&
      state.fallback &&
      state.dense &&
      state.deviceRows === 20 &&
      state.groupCount === 2 &&
      state.islandCount === 1 &&
      state.solidTable
    ),
  )
  assert.deepEqual(changedDemo.filteredBackdrops, [])
  assert.equal(changedDemo.solidTable, true, 'Demo Device Management must use a solid reading surface')
  assert.ok(
    Number(changedDemo.contrast) >= 4.5,
    'Dark solid fallback must preserve at least 4.5:1 key-text contrast',
  )

  await cdp.run(() => {
    const select = (key, value) => {
      const control = document.querySelector('[data-demo-control="' + key + '"]')
      control.value = value
      control.dispatchEvent(new Event('change', { bubbles: true }))
    }
    select('theme', 'light')
    select('background', 'spectrum')
    select('material', 'regular')
    select('transparency', 'default')
    select('scenario', 'dashboard')
    return true
  })
  await waitFor(
    'Liquid Glass demo reset state',
    () => cdp.run(() => ({
      theme: document.querySelector('.liquid-glass-demo')?.getAttribute('data-bs-theme'),
      dashboard: Boolean(document.querySelector('.demo-dashboard-panels')),
    })),
    (state) => state.theme === 'light' && state.dashboard,
  )

  let stableDemoDiagnostics = demo.diagnostics
  let gpuRenderer = 'disabled structural browser'
  let performanceSource = 'demo-structural'
  if (screenshotPath || gpuPerformanceGate) {
    await delay(5200)
    stableDemoDiagnostics = await cdp.run(() => Object.fromEntries(
      [...document.querySelectorAll('[data-diagnostic]')].map((node) => [
        node.getAttribute('data-diagnostic'),
        node.textContent?.trim() || '',
      ]),
    ))
    if (gpuPerformanceGate) {
      await cdp.navigate('http://127.0.0.1:' + vitePort + '/')
      await waitFor(
        'Dashboard real-GPU fixture',
        () => cdp.run(() => ({
          page: Boolean(document.querySelector('.dashboard-container.dashboard-refraction-ready')),
          cards: document.querySelectorAll('[data-dashboard-card]').length,
          taps: document.querySelectorAll('[data-dashboard-liquid-glass-filters] feDisplacementMap').length,
        })),
        (state) => state.page && state.cards === 7 && state.taps === 6,
        15000,
      )
      const dashboardPerformance = await cdp.run(() => new Promise((resolve, reject) => {
        const cards = [...document.querySelectorAll('[data-dashboard-card]')]
        if (cards.length !== 7) {
          reject(new Error(`Expected seven Dashboard cards, received ${cards.length}`))
          return
        }
        const cardRects = cards.map((card) => card.getBoundingClientRect())
        const allCardsVisible = cardRects.every((rect) => (
          rect.left >= 0 && rect.top >= 0 &&
          rect.right <= window.innerWidth && rect.bottom <= window.innerHeight
        ))
        if (!allCardsVisible || window.innerWidth !== 1728 || window.innerHeight !== 911) {
          reject(new Error(
            `Dashboard GPU gate requires seven visible cards at 1728x911; viewport=${window.innerWidth}x${window.innerHeight}`,
          ))
          return
        }
        const warmupMs = 2000
        const sampleMs = 10000
        const started = performance.now()
        const sampleStart = started + warmupMs
        const sampleEnd = sampleStart + sampleMs
        const deltas = []
        const sampledCards = new Set()
        let lastSample = 0
        let activeCard = null
        const animate = (now) => {
          const sampleElapsed = Math.max(0, now - sampleStart)
          const progress = Math.max(0, Math.min(0.999999, sampleElapsed / sampleMs))
          const cardIndex = Math.min(cards.length - 1, Math.floor(progress * cards.length))
          const card = cards[cardIndex]
          if (activeCard !== card) {
            activeCard?.dispatchEvent(new PointerEvent('pointerleave'))
            activeCard = card
            const rect = card.getBoundingClientRect()
            card.dispatchEvent(new PointerEvent('pointerenter', {
              clientX: rect.left + 12,
              clientY: rect.top + 12,
            }))
          }
          const rect = card.getBoundingClientRect()
          const localProgress = (progress * cards.length) - cardIndex
          card.dispatchEvent(new PointerEvent('pointermove', {
            clientX: rect.left + 12 + Math.max(0, rect.width - 24) * localProgress,
            clientY: rect.top + rect.height * (0.5 + Math.sin(localProgress * Math.PI * 2) * 0.32),
          }))
          if (now >= sampleStart) {
            sampledCards.add(cardIndex)
            if (lastSample > 0) deltas.push(now - lastSample)
            lastSample = now
          }
          if (now < sampleEnd) {
            requestAnimationFrame(animate)
            return
          }
          activeCard?.dispatchEvent(new PointerEvent('pointerleave'))
          const sorted = [...deltas].sort((a, b) => a - b)
          const average = deltas.reduce((sum, value) => sum + value, 0) / Math.max(1, deltas.length)
          resolve({
            source: 'dashboard',
            cards: cards.length,
            visibleCards: cardRects.length,
            sampledCards: sampledCards.size,
            viewport: `${window.innerWidth}x${window.innerHeight}`,
            sampleCount: deltas.length,
            fps: 1000 / average,
            p95: sorted[Math.floor((sorted.length - 1) * 0.95)] || 0,
          })
        }
        requestAnimationFrame(animate)
      }))
      assert.equal(
        dashboardPerformance.source,
        'dashboard',
        'The real-GPU gate must measure the seven production Dashboard lenses',
      )
      assert.equal(dashboardPerformance.cards, 7)
      assert.equal(dashboardPerformance.visibleCards, 7)
      assert.equal(dashboardPerformance.sampledCards, 7, 'The ten-second GPU sample must traverse every Dashboard card')
      assert.equal(dashboardPerformance.viewport, '1728x911')
      assert.ok(dashboardPerformance.sampleCount > 300, 'Dashboard GPU sampling must span the full ten-second interaction path')
      performanceSource = dashboardPerformance.source
      stableDemoDiagnostics = {
        ...stableDemoDiagnostics,
        fps: dashboardPerformance.fps.toFixed(2),
        p95: dashboardPerformance.p95.toFixed(2),
      }
      const gpuRendererInfo = await cdp.run(() => {
        const canvas = document.createElement('canvas')
        const gl = canvas.getContext('webgl') || canvas.getContext('experimental-webgl')
        if (!gl) {
          return {
            debugInfoAvailable: false,
            vendor: '',
            renderer: '',
          }
        }
        const extension = gl.getExtension('WEBGL_debug_renderer_info')
        return {
          debugInfoAvailable: Boolean(extension),
          vendor: extension ? gl.getParameter(extension.UNMASKED_VENDOR_WEBGL) : '',
          renderer: extension ? gl.getParameter(extension.UNMASKED_RENDERER_WEBGL) : '',
        }
      })
      const gpuRendererPolicy = resolveHardwareGpuRendererPolicy(gpuRendererInfo)
      assert.equal(
        gpuRendererPolicy.hardware,
        true,
        `The real-GPU gate requires positively identified hardware (${gpuRendererPolicy.reason})`,
      )
      gpuRenderer = gpuRendererPolicy.renderer
      assert.equal(
        meetsLiquidGlassGpuPerformanceGate({
          fps: dashboardPerformance.fps,
          p95: dashboardPerformance.p95,
        }),
        true,
        `Real-GPU steady state must remain at or above 55 FPS with p95 below 20ms: ${JSON.stringify(dashboardPerformance)}`,
      )
    } else {
      assert.ok(
        Number(stableDemoDiagnostics.fps) > 0 && Number(stableDemoDiagnostics.p95) > 0,
        'GPU-disabled headless capture must still produce diagnostic frame metrics',
      )
    }
  }
  await capturePageScreenshot(screenshotPath, true)

  console.log(
    'liquid glass browser test passed ' + JSON.stringify({
      groups: stableDemoDiagnostics.groups,
      backdrops: stableDemoDiagnostics.backdrops,
      nested: stableDemoDiagnostics.nested,
      contentBackdrops: stableDemoDiagnostics.contentBackdrops,
      fps: stableDemoDiagnostics.fps,
      p95: stableDemoDiagnostics.p95,
      contrast: stableDemoDiagnostics.contrast,
      dashboardReadingContrast: Math.min(
        lightBackdropContrast.reading.ratio,
        darkBackdropContrast.reading.ratio,
      ).toFixed(2),
      dashboardKpiContrast: Math.min(
        lightBackdropContrast.kpi.ratio,
        darkBackdropContrast.kpi.ratio,
      ).toFixed(2),
      centerSobelRetention: opticalMetrics.centerSobelRetention.toFixed(3),
      edgeMeanAbsoluteDifference: productionRefractionDifference.toFixed(3),
      densityCenterRetentionFloor: Math.min(
        ...lightDensityOptics.map((state) => state.centerSobelRetention),
        ...darkDensityOptics.map((state) => state.centerSobelRetention),
      ).toFixed(3),
      densityEdgeDifferenceFloor: Math.min(
        ...lightDensityOptics.map((state) => state.edgeRefractionDifference),
        ...darkDensityOptics.map((state) => state.edgeRefractionDifference),
      ).toFixed(3),
      performanceGate: gpuPerformanceGate ? 'real-gpu' : 'structural-only',
      performanceSource,
      gpuRenderer,
    }),
  )
} finally {
  try {
    cdp?.socket.close()
  } catch {
    // The browser process cleanup below is authoritative.
  }
  stopProcess(edgeProcess)
  stopProcess(viteProcess)
  await delay(500)
  if (browserProfile) {
    try {
      rmSync(browserProfile, {
        recursive: true,
        force: true,
        maxRetries: 10,
        retryDelay: 250,
      })
    } catch (error) {
      console.warn(
        'Unable to remove temporary browser profile ' +
        browserProfile +
        ': ' +
        error.message,
      )
    }
  }
}
