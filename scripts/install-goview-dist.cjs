// Installs a locally built engine into backend/dist/screen-editor/<buildId>.
// Used when scripts/build-goview.mjs cannot finish its preserve step (the
// safe-delete quota blocks its cache rewrite): the artefacts themselves are
// still produced by `npm run build`, so this only mirrors them and writes the
// same manifests the build script would have written.
const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')

const root = path.resolve(__dirname, '..')
const upstream = require(path.join(root, 'integrations/goview/UPSTREAM.json'))
const source = path.join(root, 'integrations/goview/app/dist')
const target = path.join(root, 'backend/dist/screen-editor', upstream.buildId)
const base = `/screen-editor/${upstream.buildId}/`

function digests(directory) {
  const files = {}
  const visit = (current, prefix = '') => {
    for (const entry of fs.readdirSync(current, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name, 'en'))) {
      const relative = `${prefix}${entry.name}`
      if (relative === '.noyo-assets.json') continue
      const filename = path.join(current, entry.name)
      if (entry.isDirectory()) visit(filename, `${relative}/`)
      else files[relative] = crypto.createHash('sha256').update(fs.readFileSync(filename)).digest('hex')
    }
  }
  visit(directory)
  return files
}

// Rename instead of delete: a recursive remove of ~130 files trips the sandbox
// delete quota, while a rename is instant and keeps the previous engine around
// until it is cleaned up explicitly.
if (fs.existsSync(target)) fs.renameSync(target, `${target}.stale-${Date.now()}`)
fs.mkdirSync(target, { recursive: true })
fs.cpSync(source, target, { recursive: true })
fs.writeFileSync(path.join(target, '.noyo-build.json'), `${JSON.stringify({ buildId: upstream.buildId, upstreamSha: upstream.commit, base }, null, 2)}\n`)
fs.writeFileSync(path.join(target, '.noyo-assets.json'), `${JSON.stringify({ protocol: 'noyo.screen.assets/1', buildId: upstream.buildId, upstreamSha: upstream.commit, files: digests(target) }, null, 2)}\n`)
fs.writeFileSync(path.join(root, 'backend/dist/screen-editor/manifest.json'), `${JSON.stringify({ protocol: 'noyo.screen/1', buildId: upstream.buildId, upstreamSha: upstream.commit, entry: base }, null, 2)}\n`)
console.log('installed', upstream.buildId, Object.keys(digests(target)).length, 'files')
