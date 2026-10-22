// Renders an authored screen document with real sample data and captures it.
//   node tooling/render-screen.mjs [--mode view|edit] [--settle 4000]
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire('C:/Users/Administrator/.workbuddy/binaries/node/workspace/')
const puppeteer = require('puppeteer-core')

const here = path.dirname(fileURLToPath(import.meta.url))
const fixtures = path.resolve(here, '../tmp-verify/fixtures')
const outDir = path.resolve(here, '../tmp-verify')

const args = process.argv.slice(2)
const option = (name, fallback) => {
  const index = args.indexOf(`--${name}`)
  return index >= 0 && args[index + 1] ? args[index + 1] : fallback
}
const mode = option('mode', 'view')
const settle = option('settle', '6000')
const base = option('url', 'http://127.0.0.1:5199/screen-editor/goview-5ab05b4-noyo4/harness.html')
// MapLibre needs a real WebGL context. Headless Chrome without SwiftShader
// silently fails to create one, which would look exactly like "the map is broken".
const gpu = process.argv.includes('--gpu')

const document = JSON.parse(fs.readFileSync(path.join(fixtures, option('doc', 'fire-screen.json')), 'utf8'))
const samples = JSON.parse(fs.readFileSync(path.join(fixtures, option('samples', 'screen-samples.json')), 'utf8'))
const history = JSON.parse(fs.readFileSync(path.join(fixtures, option('history', 'screen-history.json')), 'utf8'))
const width = document.canvas?.width || 1920
const height = document.canvas?.height || 1080

const browser = await puppeteer.launch({
  executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
  headless: true,
  args: gpu
    ? ['--no-sandbox', '--force-device-scale-factor=1', '--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader']
    : ['--no-sandbox', '--disable-gpu', '--force-device-scale-factor=1']
})
try {
  const page = await browser.newPage()
  await page.setViewport({ width: width + 40, height: height + 60, deviceScaleFactor: 1 })
  const consoleErrors = []
  page.on('console', message => { if (message.type() === 'error') consoleErrors.push(message.text().slice(0, 240)) })
  page.on('pageerror', error => consoleErrors.push(`pageerror: ${String(error).slice(0, 240)}`))
  await page.evaluateOnNewDocument((doc, sample, hist) => {
    window.__FIXTURE_DOCUMENT__ = doc
    window.__FIXTURE_SAMPLES__ = sample
    window.__FIXTURE_HISTORY__ = hist
  }, document, samples, history)
  await page.goto(`${base}?mode=${mode}&settle=${settle}`, { waitUntil: 'domcontentloaded', timeout: 60000 })
  await page.waitForFunction('window.__HARNESS_READY__ === true', { timeout: 90000 })
  const result = await page.evaluate(() => window.__HARNESS_RESULT__)
  // The harness mounts the published viewer inside its own iframe, so the probe
  // must look there rather than at the harness document.
  const readMap = async () => {
    for (const frame of page.frames()) {
      if (frame === page.mainFrame()) continue
      const probe = await frame.evaluate(() => {
        const root = document.querySelector('[data-map-state]')
        if (!root) return null
        const labels = [...root.querySelectorAll('.device-marker-label')].map(node => node.textContent)
        const canvas = root.querySelector('.maplibregl-canvas')
        return {
          state: root.dataset.mapState,
          provider: root.dataset.mapProvider,
          idle: root.dataset.mapIdle === '1',
          tileErrors: Number(root.dataset.mapErrors || 0),
          markers: labels.length,
          labels: labels.slice(0, 12),
          canvas: canvas ? `${canvas.width}x${canvas.height}` : null,
        }
      }).catch(() => null)
      if (probe) return probe
    }
    return null
  }
  // Tiles arrive over the network: poll for the widget's own ready+idle flags instead
  // of guessing a sleep, otherwise a slow tile server looks like a broken map.
  let mapProbe = await readMap()
  for (let attempt = 0; attempt < 40 && mapProbe && mapProbe.state === 'loading'; attempt += 1) {
    await new Promise(resolve => setTimeout(resolve, 1000))
    mapProbe = await readMap()
  }
  for (let attempt = 0; attempt < 25 && mapProbe && !mapProbe.idle; attempt += 1) {
    await new Promise(resolve => setTimeout(resolve, 1000))
    mapProbe = await readMap()
  }
  const shot = path.join(outDir, `screen-${mode}${option('tag', '') ? `-${option('tag', '')}` : ''}.png`)
  await page.screenshot({ path: shot, clip: { x: 0, y: 0, width, height } })
  fs.writeFileSync(path.join(outDir, `screen-${mode}-result.json`), JSON.stringify({ result, mapProbe, consoleErrors }, null, 1))
  console.log(`widgets=${result.widgets?.length} missing=${(result.widgets || []).filter(w => w.missing).length} console=${consoleErrors.length} -> ${shot}`)
  if (mapProbe) console.log(`  map: state=${mapProbe.state} provider=${mapProbe.provider} idle=${mapProbe.idle} tileErrors=${mapProbe.tileErrors} markers=${mapProbe.markers} canvas=${mapProbe.canvas} labels=${JSON.stringify(mapProbe.labels)}`)
  for (const widget of result.widgets || []) {
    console.log(`  ${String(widget.key).padEnd(18)} ${widget.missing ? 'MISSING' : `ink-canvas=${widget.canvas}/${widget.canvasInked} svg=${widget.svg} nodes=${widget.nodes} text="${String(widget.text || '').slice(0, 40)}"`}`)
  }
  if (result.errors?.length) console.log('runtime errors:', result.errors.slice(0, 6).join(' | '))
  if (consoleErrors.length) console.log('console:', consoleErrors.slice(0, 6).join(' | '))
} finally {
  await browser.close()
}
