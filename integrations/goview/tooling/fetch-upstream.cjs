// Fetch the pinned Gitee tree without interactive Git credential helpers.
const { mkdir, readFile, writeFile } = require('node:fs/promises')
const { resolve, dirname, sep } = require('node:path')
const { createHash } = require('node:crypto')
const commit = '5ab05b439703ffda6d050e5626969459b3e3c1cf'
const target = resolve(__dirname, '../app')
const treeFile = process.argv[2]
const digest = b => createHash('sha1').update(`blob ${b.length}\0`).update(b).digest('hex')
async function main() {
  const tree = treeFile ? JSON.parse((await readFile(treeFile, 'utf8')).replace(/^\uFEFF/, '')) : await (await fetch(`https://gitee.com/api/v5/repos/dromara/go-view/git/trees/${commit}?recursive=1`)).json()
  if (tree.truncated || !Array.isArray(tree.tree)) throw new Error('Incomplete upstream tree')
  const blobs = tree.tree.filter(e => e.type === 'blob')
  let next = 0, done = 0
  const failures = []
  async function worker() {
    while (next < blobs.length) {
      const entry = blobs[next++]
      const file = resolve(target, entry.path)
      if (!file.startsWith(target + sep)) throw new Error('Unsafe upstream path')
      try {
        let existing
        try { existing = await readFile(file) } catch {}
        if (!existing || digest(existing) !== entry.sha) {
          let bytes
          for (let attempt = 0; attempt < 4; attempt++) {
            try {
              const response = await fetch(`https://gitee.com/dromara/go-view/raw/${commit}/${entry.path}`, { signal: AbortSignal.timeout(30000) })
              if (!response.ok) throw new Error(`HTTP ${response.status}`)
              bytes = Buffer.from(await response.arrayBuffer())
              if (digest(bytes) !== entry.sha) throw new Error('Git blob checksum mismatch')
              break
            } catch (error) {
              if (attempt === 3) throw error
              await new Promise(r => setTimeout(r, 500 * (attempt + 1)))
            }
          }
          await mkdir(dirname(file), { recursive: true })
          await writeFile(file, bytes)
        }
        done++
        if (done % 100 === 0) process.stdout.write(`Verified ${done}/${blobs.length}\n`)
      } catch (error) { failures.push(`${entry.path}: ${error.message}`) }
    }
  }
  await Promise.all(Array.from({ length: 8 }, worker))
  console.log(JSON.stringify({ commit, verified: done, total: blobs.length, failures }))
  if (failures.length) process.exitCode = 1
}
main().catch(error => { console.error(error.message); process.exitCode = 1 })
