import fs from 'fs'
import path from 'path'

function normalizePluginPath(pluginPath) {
  return pluginPath.split(path.sep).join('/')
}

export function parseBuildYoloEnabled(rawValue) {
  if (rawValue === undefined || rawValue === null || rawValue === '') {
    return true
  }
  return rawValue !== '0' && rawValue.toLowerCase() !== 'false'
}

export function collectGoPluginPaths(rootDir, options = {}) {
  const {
    skipTopLevelNames = [],
    skipHidden = false,
  } = options

  const pluginPaths = []

  function walk(currentDir, relativePrefix = '') {
    if (!fs.existsSync(currentDir)) {
      return
    }

    const items = fs.readdirSync(currentDir).sort((a, b) => a.localeCompare(b))
    for (const item of items) {
      if (relativePrefix === '' && skipTopLevelNames.includes(item)) {
        continue
      }
      if (skipHidden && item.startsWith('.')) {
        continue
      }

      const itemPath = path.join(currentDir, item)
      let stats
      try {
        stats = fs.statSync(itemPath)
      } catch (error) {
        console.warn(`[Sync-Pro] Warning: skipping ${itemPath}: ${error.message}`)
        continue
      }

      if (!stats.isDirectory()) {
        continue
      }

      const pluginPath = relativePrefix ? path.join(relativePrefix, item) : item
      const pluginGoPath = path.join(itemPath, 'plugin.go')
      if (fs.existsSync(pluginGoPath)) {
        pluginPaths.push(normalizePluginPath(pluginPath))
        continue
      }

      walk(itemPath, pluginPath)
    }
  }

  walk(rootDir)
  return pluginPaths
}

export function collectProPluginImports(pluginPaths, buildYoloEnabled = true) {
  return pluginPaths
    .filter((pluginPath) => buildYoloEnabled || pluginPath !== 'platform/yolo_pro')
    .map((pluginPath) => `_ "noyo/plugins/pro/${pluginPath}"`)
}

export function collectCommunityPluginImports(pluginPaths) {
  return pluginPaths.map((pluginPath) => `_ "noyo/plugins/${pluginPath}"`)
}
