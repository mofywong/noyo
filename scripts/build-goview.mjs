import { createHash } from 'node:crypto'
import { spawn } from 'node:child_process'
import fs from 'node:fs/promises'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = fileURLToPath(new URL('../', import.meta.url))
const sourceRoot = path.join(root, 'integrations', 'goview')
const app = path.join(sourceRoot, 'app')
const sourceDist = path.join(app, 'dist')
const cache = path.join(root, 'frontend', 'node_modules', '.cache', 'noyo-screen-engines')
const args = process.argv.slice(2)
const options = new Set(['--copy-only', '--preserve-only', '--verify-only'])
let outDir = process.env.NOYO_FRONTEND_OUT_DIR || path.join(root, 'backend', 'dist')
for (let i = 0; i < args.length; i++) {
  if (args[i] === '--out-dir' && args[i + 1]) { outDir = path.resolve(process.cwd(), args[++i]); continue }
  if (!options.has(args[i])) throw new Error(`Unknown build-goview argument: ${args[i]}`)
}
outDir = path.isAbsolute(outDir) ? path.resolve(outDir) : path.resolve(root, 'frontend', outDir)
const output = path.join(outDir, 'screen-editor')
const upstream = JSON.parse(await fs.readFile(path.join(sourceRoot, 'UPSTREAM.json'), 'utf8'))
const buildId = upstream.buildId
if (!/^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(buildId) || !/^[a-f0-9]{40}$/.test(upstream.commit)) throw new Error('Invalid GoView upstream identity')
const base = `/screen-editor/${buildId}/`
const manifest = { protocol: 'noyo.screen/1', buildId, upstreamSha: upstream.commit, entry: base }
const assetsManifestName = '.noyo-assets.json'
const assetsProtocol = 'noyo.screen.assets/1'

async function exists(filename) { try { await fs.access(filename); return true } catch { return false } }
function inside(parent, child) {
  const relative = path.relative(path.resolve(parent), path.resolve(child))
  if (!relative || relative.startsWith(`..${path.sep}`) || relative === '..' || path.isAbsolute(relative)) throw new Error('Unsafe editor output path')
}
async function assertDirectory(directory) {
  const stats = await fs.lstat(directory)
  if (!stats.isDirectory() || stats.isSymbolicLink()) throw new Error(`Editor directory must not be a link: ${directory}`)
}
async function assertTree(directory) {
  await assertDirectory(directory)
  for (const entry of await fs.readdir(directory, { withFileTypes: true })) {
    const filename = path.join(directory, entry.name)
    if (entry.isSymbolicLink()) throw new Error(`Editor assets must not contain links: ${filename}`)
    if (entry.isDirectory()) await assertTree(filename)
    else if (!entry.isFile()) throw new Error(`Unsupported editor asset: ${filename}`)
  }
}
async function copyTree(from, to, parent) {
  inside(parent, to)
  await assertTree(from)
  if (await exists(to)) await assertTree(to)
  await fs.mkdir(to, { recursive: true })
  await fs.cp(from, to, { recursive: true, force: true, dereference: false })
}
async function preserve() {
  if (!(await exists(output))) return
  await assertDirectory(output)
  await fs.mkdir(cache, { recursive: true })
  await assertDirectory(cache)
  for (const entry of await fs.readdir(output, { withFileTypes: true })) {
    if (entry.isSymbolicLink()) throw new Error('The editor output must not contain symlinks')
    if (entry.isDirectory() && /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(entry.name)) {
      await copyTree(path.join(output, entry.name), path.join(cache, entry.name), cache)
    }
  }
}
async function fingerprint() {
  const hash = createHash('sha256')
  async function visit(directory, prefix = '') {
    const entries = (await fs.readdir(directory, { withFileTypes: true })).sort((a, b) => a.name < b.name ? -1 : a.name > b.name ? 1 : 0)
    for (const entry of entries) {
      if (['node_modules', 'dist', '.git', '.cache'].includes(entry.name)) continue
      const filename = path.join(directory, entry.name), relative = `${prefix}${entry.name}`
      if (entry.isSymbolicLink()) throw new Error('GoView source must not contain symlinks')
      if (entry.isDirectory()) await visit(filename, `${relative}/`)
      else if (entry.isFile()) { hash.update(relative); hash.update('\0'); hash.update(await fs.readFile(filename)); hash.update('\0') }
    }
  }
  await visit(app)
  hash.update(await fs.readFile(path.join(sourceRoot, 'UPSTREAM.json')))
  if (await exists(path.join(sourceRoot, 'tooling'))) await visit(path.join(sourceRoot, 'tooling'), 'tooling/')
  hash.update(base)
  return hash.digest('hex')
}
async function stampAt(directory) {
  const stamp = JSON.parse(await fs.readFile(path.join(directory, '.noyo-build.json'), 'utf8'))
  if (stamp.buildId !== buildId || stamp.upstreamSha !== upstream.commit || stamp.base !== base) throw new Error('GoView build stamp does not match the pinned upstream')
  const index = await fs.readFile(path.join(directory, 'index.html'), 'utf8')
  if (!index.includes(base)) throw new Error('GoView index.html has an incorrect deployment base')
  return stamp
}
async function assetDigests(directory) {
  await assertTree(directory)
  const files = {}
  async function visit(current, prefix = '') {
    const entries = (await fs.readdir(current, { withFileTypes: true })).sort((a, b) => a.name.localeCompare(b.name, 'en'))
    for (const entry of entries) {
      const relative = `${prefix}${entry.name}`
      if (relative === assetsManifestName) continue
      const filename = path.join(current, entry.name)
      if (entry.isDirectory()) await visit(filename, `${relative}/`)
      else files[relative] = createHash('sha256').update(await fs.readFile(filename)).digest('hex')
    }
  }
  await visit(directory)
  return files
}
async function writeAssetManifest(directory) {
  const files = await assetDigests(directory)
  const digestManifest = { protocol: assetsProtocol, buildId, upstreamSha: upstream.commit, files }
  await fs.writeFile(path.join(directory, assetsManifestName), `${JSON.stringify(digestManifest, null, 2)}\n`, { flag: 'wx' })
}
async function verifyAssetManifest(directory) {
  let installed
  try { installed = JSON.parse(await fs.readFile(path.join(directory, assetsManifestName), 'utf8')) }
  catch { throw new Error(`Missing or invalid ${assetsManifestName}: ${directory}. Restore a trusted release package or rebuild this exact engine; historical output is never automatically attested.`) }
  if (installed.protocol !== assetsProtocol || installed.buildId !== buildId || installed.upstreamSha !== upstream.commit || !installed.files || typeof installed.files !== 'object' || Array.isArray(installed.files)) throw new Error('Editor asset manifest identity is invalid')
  const actual = await assetDigests(directory)
  const wanted = Object.keys(installed.files).sort()
  const found = Object.keys(actual).sort()
  if (!actual['index.html'] || !actual['.noyo-build.json'] || wanted.length !== found.length || wanted.some((key, index) => key !== found[index] || installed.files[key] !== actual[key])) throw new Error(`Editor asset SHA256 verification failed: ${directory}`)
}
async function runNpm(arguments_) {
  const npmCli = process.env.npm_execpath
  const executable = npmCli ? process.execPath : process.platform === 'win32' ? 'npm.cmd' : 'npm'
  const cliArgs = npmCli ? [npmCli, ...arguments_] : arguments_
  console.log(`[GoView] ${arguments_.join(' ')} (${buildId})`)
  await new Promise((resolve, reject) => {
    const child = spawn(executable, cliArgs, { cwd: app, env: { ...process.env, NOYO_SCREEN_BASE: base }, shell: !npmCli && process.platform === 'win32', stdio: ['ignore', 'pipe', 'pipe'] })
    let tail = ''
    const capture = (chunk) => { tail = (tail + chunk.toString()).slice(-3600) }
    child.stdout.on('data', capture); child.stderr.on('data', capture)
    child.on('error', reject)
    child.on('close', (code) => code === 0 ? resolve() : reject(new Error(`GoView npm failed (${code}):\n${tail}`)))
  })
}
async function dependenciesCurrent() {
  try {
    const wanted = JSON.parse(await fs.readFile(path.join(app, 'package-lock.json'), 'utf8')).packages
    const installed = JSON.parse(await fs.readFile(path.join(app, 'node_modules', '.package-lock.json'), 'utf8')).packages
    if (!wanted || !installed) return false
    for (const [name, item] of Object.entries(installed)) if (name && item.version !== wanted[name]?.version) return false
    for (const [name, item] of Object.entries(wanted)) if (name && !item.optional && installed[name]?.version !== item.version) return false
    return true
  } catch { return false }
}
async function verify() {
  const installed = JSON.parse(await fs.readFile(path.join(output, 'manifest.json'), 'utf8'))
  for (const key of Object.keys(manifest)) if (installed[key] !== manifest[key]) throw new Error(`Editor manifest mismatch: ${key}`)
  await stampAt(path.join(output, buildId))
  await verifyAssetManifest(path.join(output, buildId))
  console.log(`[GoView] Verified ${path.join(output, 'manifest.json')}`)
}

if (args.includes('--verify-only')) { await verify() }
else if (args.includes('--preserve-only')) { await preserve() }
else {
  inside(outDir, output)
  const sourceFingerprint = await fingerprint()
  let stamp
  try { stamp = await stampAt(sourceDist) } catch { stamp = null }
  let fresh = stamp?.sourceFingerprint === sourceFingerprint
  if (fresh) {
    try { await verifyAssetManifest(sourceDist) }
    catch { fresh = false }
  }
  if (!fresh && args.includes('--copy-only')) throw new Error('GoView output is stale. Run the build without --copy-only first.')
  if (!fresh) {
    if (!(await dependenciesCurrent())) await runNpm(['ci', '--no-audit', '--no-fund'])
    await runNpm(['run', 'build'])
    stamp = await stampAt(sourceDist)
    if (await fingerprint() !== sourceFingerprint) throw new Error('GoView sources changed while building. Retry after source edits are complete.')
    await fs.writeFile(path.join(sourceDist, '.noyo-build.json'), `${JSON.stringify({ ...stamp, sourceFingerprint }, null, 2)}\n`)
    await writeAssetManifest(sourceDist)
  } else console.log(`[GoView] Reusing source-verified ${buildId}`)
  await preserve()
  await fs.mkdir(output, { recursive: true })
  await assertDirectory(output)
  if (await exists(cache)) {
    await assertDirectory(cache)
    for (const entry of await fs.readdir(cache, { withFileTypes: true })) {
      if (entry.isDirectory() && /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/.test(entry.name)) await copyTree(path.join(cache, entry.name), path.join(output, entry.name), output)
    }
  }
  const currentOutput = path.join(output, buildId)
  inside(output, currentOutput)
  if (await exists(currentOutput)) {
    await assertTree(currentOutput)
    await fs.rm(currentOutput, { recursive: true })
  }
  await copyTree(sourceDist, currentOutput, output)
  const temporaryManifest = path.join(output, `.manifest-${process.pid}.json`)
  await fs.writeFile(temporaryManifest, `${JSON.stringify(manifest, null, 2)}\n`, { flag: 'wx' })
  await fs.rename(temporaryManifest, path.join(output, 'manifest.json'))
  await verify()
}
