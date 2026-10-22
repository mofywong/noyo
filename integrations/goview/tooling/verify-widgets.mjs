// Drives the development harness in a real Chromium and reports, per widget,
// whether anything actually reached the screen and whether a device sample
// changed what is drawn. This is the only check that can tell a blank chart
// from a working one: build, type checks and contract tests cannot.
//
//   node tooling/verify-widgets.mjs [--mode edit|view] [--batch N] [--keep]
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire('C:/Users/Administrator/.workbuddy/binaries/node/workspace/')
const puppeteer = require('puppeteer-core')
const { PNG } = require('pngjs')

const here = path.dirname(fileURLToPath(import.meta.url))
const outDir = path.resolve(here, '../tmp-verify')
fs.mkdirSync(outDir, { recursive: true })

const args = process.argv.slice(2)
const option = (name, fallback) => {
  const index = args.indexOf(`--${name}`)
  return index >= 0 && args[index + 1] ? args[index + 1] : fallback
}
const mode = option('mode', 'edit')
const onlyBatch = option('batch', '')
const base = option('url', 'http://127.0.0.1:5199/screen-editor/goview-5ab05b4-noyo4/harness.html')
const CHROME = 'C:/Program Files/Google/Chrome/Application/chrome.exe'

function inkRatio(png, rect) {
  const { x, y, w, h } = rect
  if (w < 4 || h < 4) return { ratio: 0, samples: 0 }
  const left = Math.max(0, Math.min(png.width - 1, x))
  const top = Math.max(0, Math.min(png.height - 1, y))
  const right = Math.max(left + 1, Math.min(png.width, x + w))
  const bottom = Math.max(top + 1, Math.min(png.height, y + h))
  // Background = the most common colour along the region border. Anything that
  // differs from it is content, regardless of which widget painted it.
  const tally = new Map()
  for (let px = left; px < right; px++) {
    for (const py of [top, bottom - 1]) {
      const index = (py * png.width + px) * 4
      const key = `${png.data[index]},${png.data[index + 1]},${png.data[index + 2]}`
      tally.set(key, (tally.get(key) || 0) + 1)
    }
  }
  let background = [17, 17, 17]
  let best = -1
  for (const [key, count] of tally) if (count > best) { best = count; background = key.split(',').map(Number) }
  let inked = 0, total = 0
  for (let py = top; py < bottom; py += 2) {
    for (let px = left; px < right; px += 2) {
      const index = (py * png.width + px) * 4
      const distance = Math.abs(png.data[index] - background[0]) + Math.abs(png.data[index + 1] - background[1]) + Math.abs(png.data[index + 2] - background[2])
      total++
      if (distance > 24) inked++
    }
  }
  return { ratio: total ? inked / total : 0, samples: total, background: background.join(',') }
}

const browser = await puppeteer.launch({
  executablePath: CHROME,
  headless: true,
  args: ['--no-sandbox', '--disable-gpu', '--force-device-scale-factor=1', '--allow-file-access-from-files']
})

const report = { mode, startedAt: new Date().toISOString(), pages: [], widgets: [], failures: [] }

try {
  const page = await browser.newPage()
  await page.setViewport({ width: 1700, height: 1000, deviceScaleFactor: 1 })
  const consoleErrors = []
  page.on('console', message => { if (message.type() === 'error') consoleErrors.push(message.text().slice(0, 300)) })
  page.on('pageerror', error => consoleErrors.push(`pageerror: ${String(error).slice(0, 300)}`))

  let batch = 0
  while (true) {
    if (onlyBatch !== '' && Number(onlyBatch) !== batch) { batch++; if (batch > 20) break; continue }
    const url = `${base}?batch=${batch}&mode=${mode}`
    await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 60000 })
    try {
      await page.waitForFunction('window.__HARNESS_READY__ === true', { timeout: 90000 })
    } catch {
      report.failures.push(`batch ${batch}: harness never settled`)
      break
    }
    const result = await page.evaluate(() => window.__HARNESS_RESULT__)
    if (!result || result.timeout) { report.failures.push(`batch ${batch}: ${JSON.stringify(result)}`); break }
    if (result.failures?.length) report.failures.push(...result.failures.map(item => `batch ${batch}: ${item}`))

    const shotPath = path.join(outDir, `${mode}-batch-${batch}.png`)
    const buffer = await page.screenshot({ type: 'png', clip: { x: 0, y: 0, width: 1700, height: 1000 } })
    fs.writeFileSync(shotPath, buffer)
    const png = PNG.sync.read(buffer)

    for (const widget of result.widgets || []) {
      const ink = widget.missing ? { ratio: 0 } : inkRatio(png, widget.rect)
      report.widgets.push({ ...widget, ink: Math.round(ink.ratio * 10000) / 100, batch })
    }
    report.pages.push({ batch, items: result.items, paletteCount: result.paletteCount, paletteGroups: result.paletteGroups, panelHosts: result.panelHosts, panelsAvailable: result.panelsAvailable, rpcCalls: result.rpcCalls, errors: result.errors, screenshot: path.basename(shotPath) })
    if (!result.widgets?.length) break
    batch++
    if (batch > 20) break
  }
  report.consoleErrors = consoleErrors.slice(0, 40)
} finally {
  await browser.close()
}

report.finishedAt = new Date().toISOString()
fs.writeFileSync(path.join(outDir, `report-${mode}.json`), JSON.stringify(report, null, 2))

const blank = report.widgets.filter(widget => widget.missing || (widget.ink < 0.05 && !widget.canvasInked && !widget.svg && !widget.textLength))
console.log(`mode=${mode} widgets=${report.widgets.length} blank=${blank.length}`)
console.log('palette:', report.pages.map(page => page.paletteCount).join('/'), 'groups:', report.pages.map(page => page.paletteGroups).join('/'), 'panels:', report.pages.map(page => page.panelHosts).join('/'))
if (blank.length) console.log('blank:', blank.map(widget => `${widget.key} ink=${widget.ink}${widget.missing ? ' MISSING' : ''}`).join(', '))
if (report.failures.length) console.log('failures:', report.failures.join(' | '))
if (report.consoleErrors?.length) console.log('console:', report.consoleErrors.slice(0, 8).join(' | '))
console.log('report:', path.join(outDir, `report-${mode}.json`))
