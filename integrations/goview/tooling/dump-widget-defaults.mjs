// Exports the reviewed default document for every catalogued widget straight
// from the running catalogue, so screen authoring never hand-writes chartKey,
// conKey or package (the backend rejects a mismatch).
import fs from 'node:fs'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire('C:/Users/Administrator/.workbuddy/binaries/node/workspace/')
const puppeteer = require('puppeteer-core')

const here = path.dirname(fileURLToPath(import.meta.url))
const out = path.resolve(here, '../tmp-verify/fixtures/widget-defaults.json')
const base = process.argv[2] || 'http://127.0.0.1:5199/screen-editor/goview-5ab05b4-noyo4/harness.html'

const browser = await puppeteer.launch({
  executablePath: 'C:/Program Files/Google/Chrome/Application/chrome.exe',
  headless: true,
  args: ['--no-sandbox', '--disable-gpu']
})
try {
  const page = await browser.newPage()
  await page.goto(`${base}?dump=1`, { waitUntil: 'domcontentloaded', timeout: 60000 })
  await page.waitForFunction('window.__HARNESS_READY__ === true', { timeout: 60000 })
  const dump = await page.evaluate(() => window.__HARNESS_DUMP__)
  fs.mkdirSync(path.dirname(out), { recursive: true })
  fs.writeFileSync(out, JSON.stringify(dump, null, 1))
  console.log(`widgets=${dump.length} -> ${out}`)
} finally {
  await browser.close()
}
