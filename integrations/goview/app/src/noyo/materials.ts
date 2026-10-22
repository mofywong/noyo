// Must run before any component mounts: four upstream widgets render a series
// they never register themselves.
import './echarts'
import { widgetByKey, widgets, type WidgetSpec } from './widgets'
import LocationMap from './LocationMapConfig'
import LocationMapView from './LocationMap.vue'

// Every upstream component is imported eagerly through Vite's glob so the
// palette, the renderer and the inspector stay in sync with the pinned tree
// without a hand-maintained import list. `widgets.ts` stays the allowlist: a
// glob hit that is not declared there is ignored, never constructed.
//
// Deliberately eager: splitting each `index.vue` into its own chunk is the
// obvious way to shrink the 6.8 MB entry, but the upstream component tree
// re-exports itself in cycles (VChart core, ChartItemSetting). Rollup cannot
// split a cycle across chunks, and the resulting order leaves bindings
// uninitialised — widgets that render fine in one bundle go blank when split.
// Any future attempt must first remove those cycles upstream.
const configModules = import.meta.glob<any>('../packages/components/**/config.ts', { eager: true })
const viewModules = import.meta.glob<any>('../packages/components/**/index.vue', { eager: true })
const panelModules = import.meta.glob<any>('../packages/components/**/config.vue', { eager: true })

const prefix = '../packages/components/'
function byPath(modules: Record<string, any>, suffix: string) {
  const map = new Map<string, any>()
  for (const [file, module] of Object.entries(modules)) {
    if (!file.startsWith(prefix) || !file.endsWith(suffix)) continue
    map.set(file.slice(prefix.length, file.length - suffix.length), module?.default ?? null)
  }
  return map
}
const configs = byPath(configModules, '/config.ts')
const views = byPath(viewModules, '/index.vue')
const panels = byPath(panelModules, '/config.vue')

export interface Material extends WidgetSpec {
  ctor: any
  view: any
  panel: any
}

function build(): Material[] {
  const list: Material[] = []
  for (const spec of widgets) {
    if (spec.key === 'LocationMap') {
      list.push({ ...spec, ctor: LocationMap, view: LocationMapView, panel: null })
      continue
    }
    const ctor = configs.get(spec.path)
    const view = views.get(spec.path)
    // A declared widget missing either half would render an empty box, so it is
    // dropped here and reported by the build-time catalogue check instead.
    if (!ctor || !view) continue
    list.push({ ...spec, ctor, view, panel: panels.get(spec.path) || null })
  }
  return list
}

export const materials: Material[] = build()

export function materialsOfGroup(group: string) {
  return materials.filter(item => item.group === group)
}

// Upstream ships one thumbnail per widget under `assets/images/chart/<package>/`.
// Resolving them through a glob keeps the palette images in the build only once
// and lets `fetchImages` stay a single call in `packages/index.ts`.
const imageModules = import.meta.glob<string>('../assets/images/chart/**/*.{png,jpg,jpeg,gif,svg}', {
  eager: true,
  query: '?url',
  import: 'default'
})
const imageUrls = new Map<string, string>()
for (const [file, url] of Object.entries(imageModules)) {
  imageUrls.set(file.replace('../assets/images/chart/', ''), url)
}

export function thumbnailFor(chartConfig: any) {
  if (!chartConfig?.image) return ''
  return imageUrls.get(`${String(chartConfig.package || '').toLowerCase()}/${chartConfig.image}`) || ''
}

// The upstream palette iterates `packagesList` by package name. Deriving it from
// the catalogue keeps the allowlist, the thumbnails and the drag payload in sync
// without a second hand-maintained table.
export const packagesList: Record<string, any[]> = {}
for (const material of materials) {
  const chartConfig = new material.ctor().chartConfig
  if (chartConfig.key === 'LocationMap') chartConfig.image = ''
  ;(packagesList[chartConfig.package] ||= []).push(chartConfig)
}

// Every appearance panel is registered up front: `ChartSetting` resolves it by
// `conKey` from the document, not by a palette drag, so an existing screen that
// was authored elsewhere must find it already installed.
export function registerMaterials(app: any) {
  for (const item of materials) {
    const instance = new item.ctor()
    app.component(instance.chartConfig.chartKey, item.view)
    if (item.panel) app.component(instance.chartConfig.conKey, item.panel)
  }
}

export function panelFor(key: string | undefined) {
  if (!key) return null
  return materials.find(item => item.key === key)?.panel || null
}

// Both palette clicks and native drops use the same safe, localized defaults.
export function createMaterial(key: string, locale: string) {
  const material = materials.find(item => item.key === key)
  if (!material) throw new Error('Unsupported GoView material')
  const component = sanitizeComponent(new material.ctor())
  const english = locale.startsWith('en')
  component.chartConfig.title = english ? material.en : material.zh
  if (material.shape === 'text') {
    component.option.dataset = english ? 'Text' : '文本'
    component.option.fontColor = 'var(--text-primary)'
    component.attr.w = 240
    component.attr.h = 80
  }
  return component
}

export function sanitizeComponent(component: any) {
  if (component.isGroup && Array.isArray(component.groupList)) {
    component.groupList = component.groupList.map(sanitizeComponent)
    component.filter = ''
    component.events = { baseEvent: {}, advancedEvents: {}, interactEvents: [] }
    component.request = { requestDataType: 0, requestUrl: '', requestParams: { Header: {}, Params: {}, Body: {} } }
    return component
  }
  const key = component.chartConfig?.key || component?.key
  const material = materials.find(m => m.key === key)
  if (!material) throw new Error('Unsupported GoView material')
  const trusted = new material.ctor()
  component.key = material.key
  component.chartConfig = { ...trusted.chartConfig, title: component.chartConfig.title }
  component.filter = ''
  component.events = { baseEvent: {}, advancedEvents: {}, interactEvents: [] }
  component.request = { requestDataType: 0, requestUrl: '', requestParams: { Header: {}, Params: {}, Body: {} } }
  if (material.key === 'Image' || material.key === 'ImageCarousel') component.option.dataset = ''
  if (material.shape === 'text') { component.option.link = ''; component.option.linkHead = '' }
  delete component.option?.valueFormatter
  return component
}

export { widgetByKey }
