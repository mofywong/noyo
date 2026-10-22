// Reproducible Noyo patch over the pinned, checksum-verified Gitee source.
const fs = require('node:fs')
const path = require('node:path')
const app = path.resolve(__dirname, '../app')
const write = (name, value) => fs.writeFileSync(path.join(app, name), value)
write('index.html', '<!doctype html><html><head><meta charset="UTF-8"/><meta name="viewport" content="width=device-width, initial-scale=1"/><title>Noyo \u00b7 GoView</title></head><body><div id="app"></div><script type="module" src="/src/noyo/main.ts"></script></body></html>\n')
write('.env', '# The embedded build does not connect to an upstream API.\n')
write('vite.config.ts', `import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
export default defineConfig({
  base: process.env.NOYO_SCREEN_BASE || '/screen-editor/goview-5ab05b4-noyo14/',
  plugins: [vue({ template: { compilerOptions: { isCustomElement: tag => tag.startsWith('iconify-icon') } } })],
  resolve: { alias: { '@': resolve(__dirname, 'src'), 'vue-i18n': 'vue-i18n/dist/vue-i18n.cjs.js' }, dedupe: ['vue'] },
  css: { preprocessorOptions: { scss: { additionalData: '@import "src/styles/common/style.scss";' } } },
  define: { __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false },
  server: { host: '127.0.0.1', port: 3002, open: false },
  build: { outDir: 'dist', target: 'es2020', chunkSizeWarningLimit: 3000 }
})\n`)
write('src/router/index.ts', `import { createRouter, createWebHashHistory } from 'vue-router'
import Editor from '@/noyo/Editor.vue'
const router = createRouter({ history: createWebHashHistory(''), routes: [
 { path: '/chart/home/:id(.*)*', name: 'ChartHome', component: Editor },
 { path: '/chart/preview/:id(.*)*', name: 'ChartPreview', component: Editor },
 { path: '/:pathMatch(.*)*', redirect: '/chart/home/embedded' }
] })
export function setupRouter(app: any) { app.use(router) }
export default router\n`)
write('src/packages/index.ts', `import { createMaterial, materials, packagesList as catalogue, thumbnailFor } from '@/noyo/materials'
import { state } from '@/noyo/bridge'
export const packagesList = catalogue
export const createComponent = async (data: any) => createMaterial(data.key, state.locale)
export const fetchChartComponent = (data: any) => materials.find(m => m.key === data.key)?.view
export const fetchConfigComponent = (data: any) =>
  materials.find(m => m.key === data.key)?.panel || { render: () => null }
export const fetchImages = async (data: any) => thumbnailFor(data)
`)
write('src/api/axios.ts', `// Network access belongs to the Noyo host; no localStorage token is read here.
export default function disabledRequest<T = any>(..._args: any[]): Promise<T> { return Promise.reject(new Error('Direct upstream HTTP is disabled')) }\n`)
write('src/api/http.ts', `const denied = (..._args: any[]): Promise<any> => Promise.reject(new Error('Direct HTTP and executable request parameters are disabled'))
export const get = denied, post = denied, put = denied, patch = denied, del = denied
export const http = (..._args: any[]) => denied
export const customizeHttp = denied\n`)
write('src/hooks/useSystemInit.hook.ts', 'export const useSystemInit = () => {}\n')
write('src/hooks/useLifeHandler.hook.ts', 'export const useLifeHandler = (..._args: any[]) => ({})\nexport const generateBaseFunc = (..._args: any[]) => undefined\n')
write('src/hooks/useChartDataPondFetch.hook.ts', `export const useChartDataPondFetch = () => ({ initDataPond: (..._args: any[]) => {}, clearMittDataPondMap: () => {}, addGlobalDataInterface: (..._args: any[]) => {} })\n`)
write('src/hooks/useChartDataFetch.hook.ts', `import { ref, watch, nextTick, onMounted } from 'vue'
import { subscribeWidget } from '@/noyo/runtime'
export const useChartDataFetch = (component: any, _store: any, update?: (value: any) => void) => {
 const vChartRef = ref<any>(null)
 let latest: any
 const apply = () => {
  if (latest === undefined) return
  if (component.chartConfig?.chartFrame === 'echarts') {
   if (!vChartRef.value) return
   const option: any = { dataset: latest }
   const first = component.option?.series?.[0]
   if (first && first.type !== 'pie' && Array.isArray(latest?.dimensions)) option.series = Array.from({ length: Math.max(1, latest.dimensions.length - 1) }, () => JSON.parse(JSON.stringify(first)))
   vChartRef.value.setOption(option, option.series ? { replaceMerge: ['series'] } : undefined)
   return
  }
  if (update) update(latest)
 }
 subscribeWidget(component.id, value => { latest = value; apply() })
 watch(vChartRef, () => nextTick(apply)); onMounted(() => nextTick(apply))
 return { vChartRef }
}\n`)
let source = fs.readFileSync(path.join(app, 'src/utils/utils.ts'), 'utf8')
source = source.replace("const fn = new Function('data', 'res', funcStr)", "const fn = (data: any, _res: any) => data // executable filters disabled")
const evalStart = source.indexOf('export const evalFn =')
const titleStart = source.indexOf('export const setTitle =', evalStart)
if (evalStart < 0 || titleStart < 0) throw new Error('Unexpected upstream utils layout')
source = source.slice(0, evalStart) + "export const evalFn = (_fn: string) => undefined\nexport const JSONParse = (data: string) => data.trim() ? JSON.parse(data) : undefined\n\n" + source.slice(titleStart)
write('src/utils/utils.ts', source)
source = fs.readFileSync(path.join(app, 'src/views/chart/ContentEdit/index.vue'), 'utf8')
source = source.replace(/\s*<!-- 工具栏 -->[\s\S]*?(?=\s*<\/content-box>)/, '')
source = source.replace("import { useAddKeyboard } from '../hooks/useKeyboard.hook'", '')
source = source.replace("import { useSync } from '../hooks/useSync.hook'", '')
source = source.replace("import { EditTools } from './components/EditTools'", '')
// The bottom bar (zoom, history, shortcut hints) is kept: it is the canvas's
// only affordance for scale, and Noyo drives save state through the host.
source = source.replace('const { dataSyncFetch, intervalDataSyncUpdate } = useSync()', '')
source = source.replace('addWindowUnload()', '')
source = source.replace(/onMounted\(\(\) => \{[\s\S]*?\n\}\)/, '')
source = source.replace('const editRuleMethods = {', 'const editRuleMethods = {')
source = source.replace('</script>', 'defineExpose(editRuleMethods)\n</script>')
source = source.replace('  </content-box>', '    <template #bottom><edit-bottom /></template>\n  </content-box>')
write('src/views/chart/ContentEdit/index.vue', source)
source = fs.readFileSync(path.join(app, 'src/views/chart/ContentEdit/components/EditBottom/index.vue'), 'utf8')
// The scheduled cloud sync belongs to the upstream demo backend; Noyo saves
// through the host bridge, so only its control is removed.
source = source.replace('      <edit-data-sync></edit-data-sync>\n', '')
source = source.replace("import { EditDataSync } from '../EditDataSync/index'\n", '')
write('src/views/chart/ContentEdit/components/EditBottom/index.vue', source)
source = fs.readFileSync(path.join(app, 'src/views/chart/ContentLayers/index.vue'), 'utf8')
// The layer panel is mounted on demand by the Noyo shell rather than kept
// resident, so the first snapshot must be applied immediately.
source = source.replace(`  newValue => {
    layerList.value = newValue
  }
)`, `  newValue => {
    layerList.value = newValue
  },
  { immediate: true }
)`)
write('src/views/chart/ContentLayers/index.vue', source)
source = fs.readFileSync(path.join(app, 'src/packages/components/Tables/Tables/TableList/index.vue'), 'utf8')
source = source.replace('No.{{ item.ranking }}', '{{ item.ranking }}')
source = source.replace('v-html="item.name"', 'v-text="item.name"')
source = source.replace('<div class="ranking-column"', '<div v-if="typeof item.value === \'number\'" class="ranking-column"')
source = source.replace('mergedConfig: props.chartConfig.option,', 'mergedConfig: JSON.parse(JSON.stringify(props.chartConfig.option)),')
source = source.replace('let { dataset, rowNum, sort } = status.mergedConfig\n', 'let { dataset, rowNum, sort } = status.mergedConfig\n  dataset = dataset.map((row: any) => ({ ...row }))\n')
source = source.replace('percent: ((row.value + minAbs) / total) * 100', "percent: typeof row.value === 'number' && total ? ((row.value + minAbs) / total) * 100 : 0")
source = source.replace('if (!status.mergedConfig) return\n', 'if (!status.mergedConfig) return\n    status.mergedConfig = { ...props.chartConfig.option, dataset: status.mergedConfig.dataset }\n')
source = source.replace('() => props.chartConfig.option.dataset,\n  () => {\n', '() => props.chartConfig.option.dataset,\n  () => {\n    status.mergedConfig.dataset = JSON.parse(JSON.stringify(props.chartConfig.option.dataset))\n')
source = source.replace('props.chartConfig.option.dataset = resData', 'status.mergedConfig.dataset = resData')
write('src/packages/components/Tables/Tables/TableList/index.vue', source)
const packageFile = path.join(app, 'package.json')
const pkg = JSON.parse(fs.readFileSync(packageFile, 'utf8'))
pkg.name = '@noyo/goview-editor'; pkg.private = true
pkg.scripts.build = 'vite build && node ../tooling/stamp-build.cjs'
delete pkg.scripts.postinstall
write('package.json', JSON.stringify(pkg, null, 2) + '\n')
console.log('Applied Noyo embedded runtime patch')
