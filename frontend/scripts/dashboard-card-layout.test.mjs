import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const dashboard = readFileSync(path.join(frontendRoot, 'src/views/Dashboard.vue'), 'utf8')
const liquidGlass = readFileSync(path.join(frontendRoot, 'src/styles/liquid-glass.css'), 'utf8')

function readTag(source, start) {
  assert.equal(source[start], '<', `Expected a tag at ${start}`)

  if (source.startsWith('<!--', start)) {
    const end = source.indexOf('-->', start + 4)
    assert.notEqual(end, -1, 'Unterminated template comment')
    return { kind: 'comment', start, end: end + 3 }
  }

  let cursor = start + 1
  let closing = false
  if (source[cursor] === '/') {
    closing = true
    cursor += 1
  }

  while (/\s/.test(source[cursor] || '')) cursor += 1
  const nameStart = cursor
  while (/[A-Za-z0-9:_-]/.test(source[cursor] || '')) cursor += 1
  const name = source.slice(nameStart, cursor)
  assert.ok(name, `Expected a tag name at ${start}`)

  let quote = ''
  for (; cursor < source.length; cursor += 1) {
    const character = source[cursor]
    if (quote) {
      if (character === quote && source[cursor - 1] !== '\\') quote = ''
      continue
    }
    if (character === '"' || character === "'" || character === '`') {
      quote = character
      continue
    }
    if (character === '>') break
  }
  assert.notEqual(cursor, source.length, `Unterminated <${name}> tag`)

  const end = cursor + 1
  const raw = source.slice(start, end)
  return {
    kind: 'tag',
    start,
    end,
    nameStart,
    name,
    closing,
    selfClosing: !closing && /\/\s*>$/.test(raw),
    raw,
  }
}

function nextTag(source, start, end = source.length) {
  const tagStart = source.indexOf('<', start)
  if (tagStart === -1 || tagStart >= end) return null
  const tag = readTag(source, tagStart)
  assert.ok(tag.end <= end, 'Tag crosses the enclosing template boundary')
  return tag
}

function findMatchingElement(source, openingTag) {
  assert.equal(openingTag.kind, 'tag')
  assert.equal(openingTag.closing, false)
  if (openingTag.selfClosing) {
    return {
      open: openingTag,
      close: null,
      contentStart: openingTag.end,
      contentEnd: openingTag.end,
      end: openingTag.end,
    }
  }

  let depth = 1
  let cursor = openingTag.end
  while (cursor < source.length) {
    const tag = nextTag(source, cursor)
    assert.ok(tag, `Missing closing </${openingTag.name}> tag`)
    cursor = tag.end
    if (tag.kind === 'comment' || tag.name !== openingTag.name) continue
    if (!tag.closing && !tag.selfClosing) depth += 1
    if (tag.closing) depth -= 1
    if (depth === 0) {
      return {
        open: openingTag,
        close: tag,
        contentStart: openingTag.end,
        contentEnd: tag.start,
        end: tag.end,
      }
    }
  }
  assert.fail(`Missing closing </${openingTag.name}> tag`)
}

function directChildElements(source, element) {
  const children = []
  let cursor = element.contentStart
  while (cursor < element.contentEnd) {
    const tag = nextTag(source, cursor, element.contentEnd)
    if (!tag) break
    cursor = tag.end
    if (tag.kind === 'comment' || tag.closing) continue
    const child = findMatchingElement(source, tag)
    assert.ok(child.end <= element.contentEnd, `${tag.name} escapes its parent element`)
    children.push(child)
    cursor = child.end
  }
  return children
}

function hasClass(tag, className) {
  const classAttribute = tag.raw.match(/\bclass\s*=\s*(["'])([\s\S]*?)\1/)
  return classAttribute?.[2].split(/\s+/).includes(className) || false
}

function findElementsByClass(source, element, className) {
  const matches = []
  let cursor = element.contentStart
  while (cursor < element.contentEnd) {
    const tag = nextTag(source, cursor, element.contentEnd)
    if (!tag) break
    cursor = tag.end
    if (tag.kind === 'tag' && !tag.closing && hasClass(tag, className)) {
      matches.push(findMatchingElement(source, tag))
    }
  }
  return matches
}

function findSingleGrid(source, branch, gridClass, branchName) {
  const grids = findElementsByClass(source, branch, gridClass)
  assert.equal(grids.length, 1, `${branchName} branch must contain one ${gridClass} grid`)
  return grids[0]
}

function repeatCount(element) {
  const vFor = element.open.raw.match(/\bv-for\s*=\s*(["'])([\s\S]*?)\1/)
  if (!vFor) return 1
  const staticRange = vFor[2].match(/\bin\s+(\d+)\s*$/)
  assert.ok(staticRange, 'Card v-for must use a static numeric range for this layout test')
  return Number(staticRange[1])
}

function countOpeningTags(source, element, name) {
  let count = 0
  let cursor = element.contentStart
  while (cursor < element.contentEnd) {
    const tag = nextTag(source, cursor, element.contentEnd)
    if (!tag) break
    cursor = tag.end
    if (tag.kind === 'tag' && !tag.closing && tag.name === name) count += 1
  }
  return count
}

function cardRoots(source, grid, cardClass, expectedCount, branchName, markerRequired) {
  const children = directChildElements(source, grid)
  const roots = children.filter((child) => child.open.name === 'GlassGroup')
  assert.equal(children.length, roots.length, `${branchName} ${cardClass} grid must contain only GlassGroup root cards`)
  assert.equal(
    roots.reduce((total, root) => total + repeatCount(root), 0),
    expectedCount,
    `${branchName} ${cardClass} grid must render exactly ${expectedCount} independent GlassGroup cards`,
  )

  for (const [index, root] of roots.entries()) {
    assert.ok(hasClass(root.open, 'dashboard-card'), `${branchName} ${cardClass} card ${index + 1} must be an outer dashboard-card`)
    assert.ok(hasClass(root.open, cardClass), `${branchName} card ${index + 1} must be associated with its ${cardClass} grid`)
    assert.match(root.open.raw, /\bprofile\s*=\s*["']regular["']/)
    assert.match(root.open.raw, /\bcomposition\s*=\s*["']island["']/)
    assert.match(root.open.raw, /\bshape\s*=\s*["']panel["']/)
    assert.match(root.open.raw, /\binteractive\b/)
    if (markerRequired) {
      assert.match(root.open.raw, /\bdata-dashboard-card\b/, `${branchName} ${cardClass} card ${index + 1} must expose data-dashboard-card on its GlassGroup root`)
    }
  }
  return roots
}

function extractDashboardTemplate(source) {
  const start = source.indexOf('<template>')
  const script = source.indexOf('<script setup')
  const end = source.lastIndexOf('</template>', script)
  assert.ok(start >= 0 && script > start && end > start, 'Dashboard must contain a complete root template')
  return source.slice(start, end + '</template>'.length)
}

function findBranch(source, attribute, name) {
  let cursor = 0
  while (cursor < source.length) {
    const tag = nextTag(source, cursor)
    assert.ok(tag, `Missing ${name} template branch`)
    cursor = tag.end
    if (tag.kind === 'tag' && !tag.closing && tag.name === 'template' && tag.raw.includes(attribute)) {
      return findMatchingElement(source, tag)
    }
  }
  assert.fail(`Missing ${name} template branch`)
}

function assertBranchCards(source, branch, branchName, markerRequired) {
  const kpiGrid = findSingleGrid(source, branch, 'dashboard-kpi-grid', branchName)
  const contentGrid = findSingleGrid(source, branch, 'dashboard-content-grid', branchName)
  const kpiRoots = cardRoots(source, kpiGrid, 'dashboard-kpi-card', 4, branchName, markerRequired)
  const contentRoots = cardRoots(source, contentGrid, 'dashboard-content-card', 3, branchName, markerRequired)

  const branchGlassGroups = countOpeningTags(source, branch, 'GlassGroup')
  assert.equal(
    branchGlassGroups,
    kpiRoots.length + contentRoots.length,
    `${branchName} branch must contain only its direct grid card GlassGroups, without nested or shared GlassGroup islands`,
  )

  const contentModifiers = contentRoots.map((root) => (
    ['dashboard-content-card--guardian', 'dashboard-content-card--resources', 'dashboard-content-card--service']
      .find((className) => hasClass(root.open, className))
  )).sort()
  assert.deepEqual(
    contentModifiers,
    ['dashboard-content-card--guardian', 'dashboard-content-card--resources', 'dashboard-content-card--service'],
    `${branchName} content grid must keep guardian, resources, and service as independent cards`,
  )
}

function assertDashboardCardStructure(template) {
  const loading = findBranch(template, 'v-if="initialLoading"', 'loading')
  const loaded = findBranch(template, 'v-else', 'loaded')
  assertBranchCards(template, loading, 'loading', false)
  assertBranchCards(template, loaded, 'loaded', true)
}

function extractDashboardStyle(source) {
  const style = source.match(/<style[^>]*>([\s\S]*?)<\/style>/)
  assert.ok(style, 'Dashboard must contain a style block')
  return style[1]
}

function selectorRuleBodies(css, selector) {
  const rules = []
  for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selectorList = match[1].replace(/\/\*[\s\S]*?\*\//g, '').split(',').map((item) => item.trim())
    if (selectorList.includes(selector)) rules.push(match[2])
  }
  return rules
}

function assertSelectorHasNoTextShadow(css, selector) {
  for (const body of selectorRuleBodies(css, selector)) {
    assert.doesNotMatch(body, /\btext-shadow\s*:/, `${selector} must not use text-shadow in a card text rule`)
  }
}

function assertCardTextRules(source) {
  const css = extractDashboardStyle(source)
  const cardTextSelectors = [
    '.kpi-label',
    '.kpi-value',
    '.dashboard-content-card .card-header',
    '.dashboard-content-card .card-body',
    '.dashboard-container .metric-block',
    '.dashboard-container .metric-label',
  ]
  for (const selector of cardTextSelectors) assertSelectorHasNoTextShadow(css, selector)
  assert.match(
    css,
    /\.dashboard-page-heading \.page-title,\s*\.dashboard-page-heading \.page-subtitle\s*\{[^{}]*\btext-shadow\s*:/,
    'The page heading may retain its text shadow',
  )
}

function replaceRange(source, start, end, replacement) {
  return `${source.slice(0, start)}${replacement}${source.slice(end)}`
}

function firstRootCard(template, branchAttribute, gridClass) {
  const branch = findBranch(template, branchAttribute, branchAttribute)
  const grid = findSingleGrid(template, branch, gridClass, branchAttribute)
  return cardRoots(template, grid, gridClass === 'dashboard-kpi-grid' ? 'dashboard-kpi-card' : 'dashboard-content-card', gridClass === 'dashboard-kpi-grid' ? 4 : 3, branchAttribute, false)[0]
}

function replaceElementTagName(template, element, replacement) {
  assert.ok(element.close, 'Expected a non-self-closing card root')
  const replacements = [element.open, element.close]
    .map((tag) => ({ start: tag.nameStart, end: tag.nameStart + tag.name.length }))
    .sort((left, right) => right.start - left.start)
  return replacements.reduce((result, range) => replaceRange(result, range.start, range.end, replacement), template)
}

function moveLoadedMarkerInsideCard(template) {
  const root = firstRootCard(template, 'v-else', 'dashboard-kpi-grid')
  const marker = root.open.raw.match(/\s+data-dashboard-card\b/)
  assert.ok(marker, 'Test mutation requires a loaded root marker')
  const firstChild = directChildElements(template, root)[0]
  const withInnerMarker = replaceRange(
    template,
    firstChild.open.nameStart + firstChild.open.name.length,
    firstChild.open.nameStart + firstChild.open.name.length,
    ' data-dashboard-card',
  )
  return replaceRange(
    withInnerMarker,
    root.open.start + marker.index,
    root.open.start + marker.index + marker[0].length,
    '',
  )
}

function nestGlassGroupInLoadingCard(template) {
  const root = firstRootCard(template, 'v-if="initialLoading"', 'dashboard-kpi-grid')
  return replaceRange(template, root.contentStart, root.contentStart, '<GlassGroup as="section"></GlassGroup>')
}

const template = extractDashboardTemplate(dashboard)

assert.match(
  dashboard,
  /import DashboardLiquidGlassFilters from '\.\.\/components\/liquid-glass\/DashboardLiquidGlassFilters\.vue';/,
  'Dashboard must import its page-scoped refractive filter definitions',
)
assert.match(
  template,
  /<DashboardLiquidGlassFilters\s+@ready="handleRefractionReady"\s*\/>/,
  'Dashboard must mount one page-scoped refractive filter registry',
)
assert.match(
  dashboard,
  /class="dashboard-container noyo-liquid-page noyo-liquid-v4-page"\s+:class="\{ 'dashboard-refraction-ready': refractionReady \}"/,
  'Dashboard must enable refraction only after the SVG maps are ready',
)
assert.match(dashboard, /const refractionReady = ref\(false\);/)
assert.match(
  dashboard,
  /const handleRefractionReady = \(\{ supported \}\) => \{\s*refractionReady\.value = supported === true;\s*\};/,
  'Dashboard readiness must be driven by the filter capability result',
)
assert.equal(
  dashboard.includes('dashboard-kpi-island') || dashboard.includes('dashboard-content-island'),
  false,
  'Dashboard cards must not be enclosed in a continuous glass island',
)
assert.equal(
  (dashboard.match(/data-dashboard-card/g) || []).length,
  7,
  'Dashboard must expose four KPI cards and three content cards as independent outer cards',
)
assertDashboardCardStructure(template)
assert.match(dashboard, /\.dashboard-kpi-grid\s*\{[\s\S]*?gap:\s*16px;/)
assert.match(dashboard, /\.dashboard-content-grid\s*\{[\s\S]*?gap:\s*16px;/)
assert.match(liquidGlass, /--noyo-duration-fast:\s*160ms;/)
assert.match(liquidGlass, /--noyo-ease-standard:\s*cubic-bezier\(0\.2,\s*0,\s*0,\s*1\);/)
assert.match(
  dashboard,
  /\.dashboard-card\s*\{[\s\S]*?transition:\s*transform\s+var\(--noyo-duration-fast\)\s+var\(--noyo-ease-standard\),\s*box-shadow\s+var\(--noyo-duration-fast\)\s+var\(--noyo-ease-standard\);[\s\S]*?\}[\s\S]*?@media \(hover: hover\) and \(pointer: fine\)[\s\S]*?\.dashboard-card:hover/,
  'Fine-pointer card hover must use the global fast-duration and standard-ease tokens',
)
assert.match(dashboard, /@media \(prefers-reduced-motion: reduce\)[\s\S]*?\.dashboard-card/)
assert.match(
  dashboard,
  /@media \(prefers-reduced-motion: reduce\)\s*\{[\s\S]*?\.dashboard-card:hover,\s*\.dashboard-card:focus-within\s*\{[\s\S]*?transform:\s*none;[\s\S]*?box-shadow:\s*none;/,
  'Reduced motion must disable card hover and focus movement and shadow',
)
assertCardTextRules(dashboard)

assert.throws(
  () => assertDashboardCardStructure(replaceElementTagName(template, firstRootCard(template, 'v-if="initialLoading"', 'dashboard-kpi-grid'), 'div')),
  /GlassGroup root cards/,
  'A loading card replaced with an ordinary element must fail the structure check',
)
assert.throws(
  () => assertDashboardCardStructure(moveLoadedMarkerInsideCard(template)),
  /must expose data-dashboard-card on its GlassGroup root/,
  'A marker moved below the loaded card root must fail the structure check',
)
assert.throws(
  () => assertDashboardCardStructure(nestGlassGroupInLoadingCard(template)),
  /without nested or shared GlassGroup islands/,
  'A nested GlassGroup in loading state must fail the structure check',
)
assert.throws(
  () => assertCardTextRules(dashboard.replace('</style>', '\n.dashboard-container .metric-block { text-shadow: 0 1px 1px currentColor; }\n</style>')),
  /metric-block must not use text-shadow/,
  'A metric text shadow must fail the selector-specific check',
)

console.log('dashboard card layout tests passed with template structure and mutation coverage')
