/**
 * Liquid Glass Button browser contract.
 * 液态玻璃按钮浏览器契约测试。
 *
 * Verifies on the real Edge renderer:
 *  - refraction pipeline: backdrop-filter carries url(#noyo-lg-btn-refract-*)
 *  - SVG displacement filter exists with a PNG displacement map
 *  - aurora backdrop provides color content behind the button
 *  - pointer tilt writes CSS variables (fluid bending)
 *  - press state toggles, ripple sequence advances
 *  - light/dark text contrast >= 4.5:1
 *  - light/dark screenshots of the create-product button
 */
import assert from 'node:assert/strict'
import { spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { contrastRatioOverLayers } from '../src/utils/liquidGlassDemoMetrics.js'

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const vitePort = Number(process.env.LIQUID_GLASS_BUTTON_TEST_PORT || 5190)
const cdpPort = Number(process.env.LIQUID_GLASS_BUTTON_CDP_PORT || 9340)
const screenshotDir = path.resolve(
  frontendRoot,
  process.env.LIQUID_GLASS_BUTTON_SCREENSHOT_DIR || '../docs/design-assets/liquid-glass-v5/2026-08-18',
)
mkdirSync(screenshotDir, { recursive: true })

const edgeCandidates = process.platform === 'win32'
  ? [
      process.env.LIQUID_GLASS_BROWSER,
      'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
      'C:\\Program Files\\Microsoft\\Edge\\Application\\msedge.exe',
    ]
  : [process.env.LIQUID_GLASS_BROWSER]
const edgeExecutable = edgeCandidates.find((candidate) => candidate && existsSync(candidate))

const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function waitFor(label, getValue, predicate, timeoutMs = 15000) {
  const deadline = Date.now() + timeoutMs
  let lastValue
  while (Date.now() < deadline) {
    try {
      lastValue = await getValue()
      if (predicate(lastValue)) return lastValue
    } catch {
      // dev-server / browser races
    }
    await delay(120)
  }
  throw new Error('Timed out waiting for ' + label + '; last: ' + JSON.stringify(lastValue))
}

function stopProcess(childProcess) {
  if (!childProcess) return
  if (process.platform === 'win32' && childProcess.pid) {
    spawnSync('taskkill', ['/pid', String(childProcess.pid), '/t', '/f'], {
      stdio: 'ignore',
      windowsHide: true,
    })
    return
  }
  childProcess.kill()
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
  const evaluate = async (expression) => {
    const result = await send('Runtime.evaluate', {
      expression,
      returnByValue: true,
      awaitPromise: true,
    })
    if (result.exceptionDetails) throw new Error(JSON.stringify(result.exceptionDetails))
    return result.result.value
  }
  const on = (method, callback) => {
    const callbacks = listeners.get(method) || []
    callbacks.push(callback)
    listeners.set(method, callbacks)
  }
  const run = (fn, ...args) => {
    const serializedArgs = args.map((value) => JSON.stringify(value)).join(',')
    return evaluate('(' + fn.toString() + ')(' + serializedArgs + ')')
  }
  const navigate = (url) => send('Page.navigate', { url })
  const screenshot = async (clip) => {
    const result = await send('Page.captureScreenshot', clip ? {
      format: 'png',
      clip: { x: clip.x, y: clip.y, width: clip.width, height: clip.height, scale: 1 },
    } : { format: 'png' })
    return Buffer.from(result.data, 'base64')
  }
  return { socket, send, evaluate, run, navigate, screenshot, on }
}

let viteProcess
let edgeProcess
let browserProfile

try {
  assert.ok(edgeExecutable, 'A local Edge executable is required')

  viteProcess = spawn(
    process.execPath,
    [
      path.join(frontendRoot, 'node_modules', 'vite', 'bin', 'vite.js'),
      '--host', '127.0.0.1',
      '--port', String(vitePort),
      '--strictPort',
    ],
    { cwd: frontendRoot, stdio: 'ignore', windowsHide: true },
  )
  await waitFor('Vite dev server', async () => {
    const response = await fetch('http://127.0.0.1:' + vitePort + '/')
    return response.ok
  }, Boolean)

  browserProfile = mkdtempSync(path.join(os.tmpdir(), 'noyo-lg-btn-test-'))
  edgeProcess = spawn(
    edgeExecutable,
    [
      '--headless=new',
      '--remote-debugging-port=' + cdpPort,
      '--user-data-dir=' + browserProfile,
      '--no-first-run',
      '--disable-extensions',
      '--disable-gpu',
      '--window-size=1600,1000',
      'about:blank',
    ],
    { stdio: 'ignore', windowsHide: true },
  )
  const cdp = await createCdpClient(cdpPort)

  const fixtureUser = {
    id: 1,
    username: 'liquid-glass-button-test',
    role: 'admin',
    tenant_id: 0,
    is_system_admin: true,
    must_change_password: false,
  }
  const fixtureProducts = [
    {
      code: 'LG-BTN-001',
      name: 'Liquid Glass QA Product',
      project_code: '',
      project_name: '',
      config: '{}',
      CreatedAt: '2026-08-18T10:00:00+08:00',
      UpdatedAt: '2026-08-18T10:00:00+08:00',
    },
  ]
  // Fetch mock for /api/*
  const originalSend = cdp.send
  const onFetch = ({ requestId, request }) => {
    const url = new URL(request.url)
    let data = []
    if (url.pathname === '/api/products' && request.method === 'GET') {
      data = fixtureProducts
    } else if (url.pathname === '/api/auth/profile') {
      data = fixtureUser
    } else if (url.pathname === '/api/setup/status') {
      data = { initialized: true, mode: 'enterprise' }
    } else if (url.pathname === '/api/projects' && request.method === 'GET') {
      data = []
    }
    const body = Buffer.from(JSON.stringify({ code: 0, data, total: data.length }))
      .toString('base64')
    originalSend('Fetch.fulfillRequest', {
      requestId,
      responseCode: 200,
      responseHeaders: [{ name: 'Content-Type', value: 'application/json; charset=utf-8' }],
      body,
    }).catch(() => {})
  }
  // 注册 Fetch mock / register fetch mock
  const originalOn = cdp.on
  originalOn('Fetch.requestPaused', onFetch)
  await originalSend('Fetch.enable', {
    patterns: [{ urlPattern: '*://127.0.0.1:' + vitePort + '/api/*' }],
  })
  await originalSend('Emulation.setDeviceMetricsOverride', {
    width: 1600,
    height: 1000,
    deviceScaleFactor: 1,
    mobile: false,
  })

  const navigateAsAdmin = async (theme) => {
    await cdp.navigate('http://127.0.0.1:' + vitePort + '/login')
    const storage = {
      access_token: 'liquid-glass-button-test',
      refresh_token: 'liquid-glass-button-test-refresh',
      user_info: JSON.stringify(fixtureUser),
      system_mode: 'enterprise',
      lang: 'zh',
      theme,
    }
    await cdp.run((entries) => {
      Object.entries(entries).forEach(([key, value]) => localStorage.setItem(key, value))
      return true
    }, storage)
    await waitFor('app shell mount', () => cdp.run(() => {
      const app = document.querySelector('#app')?.__vue_app__
      const pinia = app?.config.globalProperties.$pinia
      return {
        app: Boolean(app),
        auth: Boolean(pinia?._s?.get('auth')),
        route: location.pathname,
      }
    }), (state) => state.app && state.auth)
    const pushResult = await cdp.run(async (user) => {
      const app = document.querySelector('#app')?.__vue_app__
      const router = app?.config.globalProperties.$router
      const pinia = app?.config.globalProperties.$pinia
      const auth = pinia?._s?.get('auth')
      if (!router || !auth) return { ready: false }
      const forcedUser = { ...user, must_change_password: false }
      auth.token = 'liquid-glass-button-test'
      auth.refreshToken = 'liquid-glass-button-test-refresh'
      auth.user = forcedUser
      localStorage.setItem('user_info', JSON.stringify(forcedUser))
      try {
        const nav = await router.push({ name: 'Products' })
        return {
          ready: true,
          route: router.currentRoute.value.name,
          navName: nav?.name || String(nav),
          isLoggedIn: auth.isLoggedIn,
          hasPerm: auth.hasPermission('product:list'),
        }
      } catch (err) {
        return { ready: false, error: String(err) }
      }
    }, fixtureUser)
    return pushResult
  }

  const collectButtonState = () => cdp.run(() => {
    const shell = document.querySelector('.noyo-glass-btn-shell')
    const button = document.querySelector('.noyo-glass-btn')
    const glass = document.querySelector('.noyo-glass-btn__glass')
    const filterSvg = document.getElementById(
      Array.from(document.querySelectorAll('.noyo-glass-btn-filters filter'))
        .map((node) => node.id)[0],
    )
    const svgFilter = document.querySelector('.noyo-glass-btn-filters filter')
    const auroraSpots = document.querySelectorAll('.product-aurora__spot').length
    const text = document.querySelector('.noyo-glass-btn__text')
    const prism = document.querySelector('.noyo-glass-btn__prism')
    if (!shell || !button || !glass || !text) return null
    const btnStyle = getComputedStyle(button)
    const glassStyle = getComputedStyle(glass)
    const contentStyle = getComputedStyle(text)
    return {
      shellRefraction: shell.getAttribute('data-refraction'),
      backdrop: glassStyle.backdropFilter || glassStyle.webkitBackdropFilter,
      filterCount: document.querySelectorAll('.noyo-glass-btn-filters filter').length,
      feImageHref: svgFilter?.querySelector('feImage')?.getAttribute('href')?.slice(0, 22) || '',
      filterScale: svgFilter?.querySelector('feDisplacementMap')?.getAttribute('scale') || '',
      auroraSpots,
      label: text.textContent.trim(),
      textColor: contentStyle.color,
      tint: glassStyle.backgroundColor,
      borderTopWidth: btnStyle.borderTopWidth,
      borderRadius: btnStyle.borderRadius,
      pageBg: getComputedStyle(document.body).backgroundColor,
      pageBgToken: getComputedStyle(
        document.querySelector('[data-bs-theme]') || document.documentElement,
      ).getPropertyValue('--bg-page').trim(),
    }
  })

  // ── Light 主题 / Light theme ──
  const lightNav = await navigateAsAdmin('light')
  assert.equal(lightNav.ready, true, 'auth injection must succeed')
  assert.equal(lightNav.route, 'Products', 'must navigate to the products page')
  const lightState = await waitFor(
    'ProductList light glass button',
    collectButtonState,
    (state) => state && state.shellRefraction === 'active',
  )
  assert.equal(lightState.shellRefraction, 'active', 'Refraction pipeline must be active')
  assert.match(
    lightState.backdrop,
    /url\(["']?#noyo-lg-btn-refract-/,
    'backdrop-filter must carry the SVG refraction filter url',
  )
  assert.ok(lightState.backdrop.includes('blur('), 'backdrop-filter must blur before refracting')
  assert.ok(lightState.backdrop.includes('saturate('), 'backdrop-filter must saturate')
  assert.equal(lightState.filterCount, 1)
  assert.equal(lightState.feImageHref, 'data:image/png;base64,')
  assert.ok(Number(lightState.filterScale) > 0, 'displacement scale must be positive')
  assert.equal(lightState.auroraSpots, 3, 'aurora spots must provide refraction content')
  assert.ok(lightState.label.includes('创建产品') || lightState.label.includes('Create'), 'button label must be the localized create-product label')

  // 对比度 / contrast
  const lightContrast = contrastRatioOverLayers(
    lightState.textColor,
    [lightState.tint],
    lightState.pageBg,
  )
  assert.ok(
    lightContrast >= 4.5,
    'Light text contrast must be >= 4.5:1, got ' + lightContrast.toFixed(2),
  )

  // 指针 3D 弯曲 / pointer tilt
  const tiltState = await cdp.run(async () => {
    const button = document.querySelector('.noyo-glass-btn')
    if (!button) return null
    const rect = button.getBoundingClientRect()
    const event = new PointerEvent('pointermove', {
      clientX: rect.left + rect.width * 0.15,
      clientY: rect.top + rect.height * 0.15,
      bubbles: true,
    })
    button.dispatchEvent(event)
    await new Promise((resolve) => setTimeout(resolve, 80))
    const style = button.style
    return {
      rect: { w: rect.width, h: rect.height },
      tiltX: style.getPropertyValue('--lg-btn-tilt-x'),
      tiltY: style.getPropertyValue('--lg-btn-tilt-y'),
      px: style.getPropertyValue('--lg-btn-px'),
    }
  })
  assert.ok(tiltState, 'button must exist for tilt assertions')
  assert.notEqual(tiltState.tiltX, '0.00deg', 'pointer must bend the glass on X')
  assert.notEqual(tiltState.tiltY, '0.00deg', 'pointer must bend the glass on Y')

  // 按压状态 / press state
  const pressState = await cdp.run(async () => {
    const button = document.querySelector('.noyo-glass-btn')
    if (!button) return null
    const rect = button.getBoundingClientRect()
    button.dispatchEvent(new PointerEvent('pointerdown', {
      clientX: rect.left + rect.width / 2,
      clientY: rect.top + rect.height / 2,
      bubbles: true,
    }))
    await new Promise((resolve) => setTimeout(resolve, 80))
    const pressedClass = button.classList.contains('noyo-glass-btn--pressed')
    const rippleCount = document.querySelectorAll('.noyo-glass-btn__ripple').length
    const transform = getComputedStyle(button).transform
    button.dispatchEvent(new PointerEvent('pointerup', { bubbles: true }))
    await new Promise((resolve) => setTimeout(resolve, 80))
    return { pressedClass, rippleCount, transform, released: !button.classList.contains('noyo-glass-btn--pressed') }
  })
  assert.equal(pressState.pressedClass, true, 'pointerdown must mark the button pressed')
  assert.ok(pressState.rippleCount >= 1, 'pointerdown must spawn a ripple')
  assert.match(pressState.transform, /matrix/, 'pressed transform must be computed')
  assert.equal(pressState.released, true, 'pointerup must release the press state')

  // 截图（全页：backdrop-filter 区域在 clip 截图下会丢失 backdrop 内容）
  const lightShot = await cdp.screenshot()
  writeFileSync(path.join(screenshotDir, '01-product-create-button-light.png'), lightShot)
  assert.ok(lightShot.length > 1000, 'light screenshot must have content')

  // ── Dark 主题 / Dark theme ──
  // auth store 与 App 主题均从 localStorage 初始化恢复；
  // 直接整页加载 /products，避免 SPA 导航与守卫重定向竞争。
  await cdp.run(() => {
    localStorage.setItem('theme', 'dark')
    return true
  })
  await cdp.navigate('http://127.0.0.1:' + vitePort + '/products')
  const darkState = await waitFor(
    'ProductList dark glass button',
    collectButtonState,
    (state) => state
      && state.shellRefraction === 'active'
      && state.auroraSpots === 3
      && state.textColor !== lightState.textColor,
  )
  assert.match(darkState.backdrop, /url\(["']?#noyo-lg-btn-refract-/, 'dark backdrop must refract')
  const darkContrast = contrastRatioOverLayers(
    darkState.textColor,
    [darkState.tint],
    darkState.pageBgToken,
  )
  assert.ok(
    darkContrast >= 4.5,
    'Dark text contrast must be >= 4.5:1, got ' + darkContrast.toFixed(2),
  )
  const darkShot = await cdp.screenshot()
  writeFileSync(path.join(screenshotDir, '02-product-create-button-dark.png'), darkShot)
  assert.ok(darkShot.length > 1000, 'dark screenshot must have content')

  console.log('liquid glass button browser contract passed')
  console.log('  light contrast:', lightContrast.toFixed(2), '| dark contrast:', darkContrast.toFixed(2))
  console.log('  light backdrop:', lightState.backdrop.slice(0, 90))
  console.log('  screenshots:', screenshotDir)
} finally {
  stopProcess(edgeProcess)
  stopProcess(viteProcess)
  if (browserProfile) {
    try {
      rmSync(browserProfile, { recursive: true, force: true })
    } catch {
      // profile cleanup is best-effort
    }
  }
}
