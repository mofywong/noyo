# Noyo patch register — GoView embedded editor

Pinned upstream: Gitee `dromara/go-view`, branch `master-fetch`, commit
`5ab05b439703ffda6d050e5626969459b3e3c1cf` (version 2.2.9), retrieved 2026-10-19
and verified blob-by-blob (1002 SHA-1 checksums, `tooling/fetch-upstream.cjs`).

This register lists every deviation from that upstream tree. `patches/` holds no
binary diffs: `tooling/patch-upstream.cjs` re-applies the whole set
deterministically over a clean checkout, and `src/noyo/` is maintained directly
as Noyo code. Upgrading upstream means re-reading the pinned tree, diffing
against this register, re-applying, and bumping `adapterVersion` in
`UPSTREAM.json` (never editing the pinned SHA silently).

## A. Whole-file replacements (`tooling/patch-upstream.cjs`)

| Upstream path | Change | Reason |
| --- | --- | --- |
| `index.html` | Entry points at `/src/noyo/main.ts`; title "Noyo · GoView" | The embedded entry is the Noyo adapter, not the upstream demo shell |
| `.env` | Emptied except a comment | Remove the upstream demo API origin; the build connects to nothing upstream |
| `vite.config.ts` | Base comes from `NOYO_SCREEN_BASE`; keeps the Vue plugin with `iconify-icon` custom elements; no upstream proxy/dev server conveniences | Assets must resolve under the versioned `/screen-editor/<engineBuildId>/` prefix |
| `src/router/index.ts` | Only `/chart/home/:id` and `/chart/preview/:id` exist, both rendered by the Noyo Editor | Login, workbench, template market and other upstream flows are not part of the Noyo product |
| `src/packages/index.ts` | The component registry is backed by the Noyo widget catalogue (81 declarations), the grouped `packagesList` the upstream palette iterates, and the per-widget thumbnails resolved by `thumbnailFor`; `fetchConfigComponent` returns the component's own `config.vue` so the upstream "customise" tab mounts it | Only declared materials may be constructed; one declaration table drives palette, thumbnails, drag payloads and appearance panels. `packagesList` is re-exported rather than re-bound, because `packagesStore` reads it while `materials.ts` is still resolving the globbed configs (a local `const` hits the TDZ) |
| `src/api/axios.ts` | Every call rejects | No upstream base URL, no `localStorage` token, no second identity |
| `src/api/http.ts` | `get/post/put/patch/del/customizeHttp` reject | Closes the arbitrary remote request branch entirely |
| `src/hooks/useSystemInit.hook.ts` | No-op | Upstream init fetched user/project state that must not exist here |
| `src/hooks/useLifeHandler.hook.ts`, `generateBaseFunc` | No-ops | Lifecycle and base event script execution is an execution point; disabled, not hidden |
| `src/hooks/useChartDataPondFetch.hook.ts` | No-op data pond | Polling/pond data sources are replaced by the host runtime subscription |
| `src/hooks/useChartDataFetch.hook.ts` | Subscribes to `@/noyo/runtime`; echarts cartesian families are driven by the delivered `dataset`, pie gains an explicit `encode`, families that own their option shape (radar / heatmap / treemap / word cloud / map) are repainted after their `update` callback, and VChart rows are remapped onto each chart's own category/value/series fields. A component that supplies an `update` callback but exposes no chart ref now takes the callback path instead of being skipped | One host stream per screen; no component-level HTTP or timers. Upstream only repaints those families inside its `preview` route, which the Noyo host never uses, so the repaint is issued here — otherwise a published screen would keep placeholder data. The ring gauge is exactly such a component: gating on a ref it never binds left it frozen on its seeded percentage |
| `package.json` | Renamed `@noyo/goview-editor`, marked private, build appends the stamp step, `postinstall` removed | Deterministic, non-published build without upstream install side effects |

## B. In-place source edits (`tooling/patch-upstream.cjs`)

| Upstream path | Change | Reason |
| --- | --- | --- |
| `src/utils/utils.ts` | `new Function('data','res',funcStr)` replaced by a pass-through; `evalFn` returns `undefined`; `JSONParse` kept strict | User data filters/formatters could execute arbitrary code; P0 disables the branch itself |
| `src/views/chart/ContentEdit/index.vue` | Upstream toolbar (login/save/publish/demo actions), keyboard and sync hooks and the edit-tools cluster removed; `EditBottom` kept and mounted through the bottom slot; `editRuleMethods` exposed via `defineExpose` | The Noyo toolbar drives undo/redo/save; upstream actions must not reach a second backend. The bottom bar is retained because it is the canvas's only scale and history affordance |
| `src/views/chart/ContentEdit/components/EditBottom/index.vue` | The scheduled `EditDataSync` control is removed | It wrote documents to the upstream demo backend on a timer; Noyo saves through the host bridge |
| `src/views/chart/ContentLayers/index.vue` | The layer list watch is `immediate` | The Noyo shell mounts the panel on demand, so the first snapshot never produced a change event and the tree rendered empty |

## C. Removed upstream content

| `src/packages/components/Charts/Pies/PieCircle/index.vue` | The preview callback runs through `dataHandle` and republishes `option.value` as a deep copy | The upstream callback wrote into `option.value`'s inner fields. `v-chart` watches the option reference, so the ring and its centre label never repainted; reassigning the same object is a no-op for the watcher |
| `src/noyo/echarts.ts` (new) | Registers `GaugeChart`, `GraphChart`, `SankeyChart` and `LinesChart` once, from `materials.ts` | Four upstream components render a series they never register themselves (Dial renders `gauge` but registers `PieChart`; Graph and Sankey register `RadarChart`; MapBase's flow overlay is `lines`). An unregistered series is dropped silently: the widget keeps its frame and draws nothing, which no build or type check can catch |

`src/packages/components/Tables/Tables/TableList/index.vue` also renders row names
as plain text, isolates runtime data from persisted options, copies datasets
before sorting, and hides numeric progress bars for text alarm severities. This
prevents alarm titles from becoming HTML and live updates from becoming drafts.

- Production mock (`prodEnabled: true`): not present in this tree, and nothing
  re-adds it; a mock response must never mask a failed save.
- Demo login/user-info flows, template market, remote iframe/material and the
  unrestricted request/script editors: unreachable through the embedded routes
  and registry.
- Monaco editor code is not reachable from the embedded entry, so no editor
  worker is shipped in the production assets.

## D. Noyo-maintained files (not patches, tracked directly)

`src/noyo/main.ts`, `App.vue`, `Editor.vue`, `bridge.ts`, `runtime.ts`,
`materials.ts`, `widgets.ts`, `shapes.ts`, `style.css`, `AssetLibrary.vue`, `Assistant.vue`, `assistantPlan.js`, `LocationMap.vue`,
`LocationMapConfig.ts`, `mapGeometry.ts`, `basemap.ts`.

`LocationMap` renders a **real basemap** (MapLibre GL + the same provider set as the
console's device-map page) instead of the old hand-drawn SVG grid, and labels points
with the device **name**. MapLibre is loaded through a dynamic `import()` so the
~900 kB of GL code is only fetched when a screen actually contains a map. Two details
are load-bearing:

- `import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url'` +
  `setWorkerUrl(...)`: without the explicit `?url` the worker is never emitted, the
  backend's SPA fallback answers with `index.html`, and the map silently renders no
  tiles at all (same failure that hit the console map in 2026-10-17).
- The basemap provider and the Tianditu key are read from the **same localStorage keys**
  as the console (`noyo.map.basemap`, `noyo.map.tianditu.token`). The screen runs in a
  same-origin iframe, so an operator who configured the key once sees a working map in
  the IOC screen with no second configuration step. Missing key / offline ⇒ the widget
  falls back to the schematic grid and says why; it never draws a fake map. These implement the `noyo.screen/1` handshake, the
document envelope round-trip, the runtime transport (values + quality), the
bilingual restricted inspector, the reviewed 81-component catalogue and its
per-family shaping contract.

The catalogue is the only place a component is declared: `widgets.ts` holds one
row per component with its `shape`, `materials.ts` assembles constructor / view /
appearance panel through `import.meta.glob`, and `tooling/catalogue.test.cjs`
fails when a declaration, an upstream file, a backend whitelist entry or a
backend binding target drifts apart.

## E. Consumer-side pins

- `backend/core/screen/document.go` — `EngineBuildID` (current) and
  `LegacyEngineBuildIDs` (still loadable). New documents stamp the current id;
  old drafts and releases keep validating against their own.
- `frontend/src/screens/api.js` — `SCREEN_ENGINE` must match the manifest the
  backend serves; `UPSTREAM.json` — `buildId` is the same value.
- Build retention: `scripts/build-goview.mjs` preserves existing engine directories
  and refuses to copy stale output. It generates and verifies a SHA256 asset
  manifest for newly built output. `screen-preflight` checks the candidate
  package against actual stored draft/release references before deployment;
  it never invents a missing digest manifest for historical output.

## F. Dedicated IOC assistant / 独立 IOC 助手

`Assistant.vue` 提供大屏专用侧栏、双语提示、变更预览、过期方案保护、破坏性操作确认及完整文档撤回。`assistantPlan.js` 在分离的文档副本中合并组件、布局、静态数据和绑定；`Editor.vue` 通过现有校验、自动保存及运行时绑定流程应用方案。`runtime.ts` 和组件清理支持组合内子组件，保留其实际数据形态。

`Assistant.vue` provides the screen sidebar, bilingual prompts, proposal preview, stale proposal guards, destructive confirmations and whole-document undo. `assistantPlan.js` merges widgets, layout, static data and bindings into a detached document. `Editor.vue` applies proposals through existing validation, autosave and runtime bindings. Runtime shaping and component sanitization preserve children inside groups.

宿主通过受限的 `assistant.plan` RPC 调用独立接口。当前构建为 `goview-5ab05b4-noyo13`；编辑旧草稿时使用新适配器，预览和已发布版本保持固定资源版本。

The host calls a dedicated endpoint through the restricted `assistant.plan` RPC. The current build is `goview-5ab05b4-noyo13`; historical drafts use the new adapter for edits, while preview and published releases keep their pinned assets.

## G. Canvas workspace and assistant / 画布工作区与助手

编辑器移除运行时版本标签；MIT 版权及许可通过 `public/third-party/GoView-LICENSE.txt` 随构建分发，独立资源声明保存在同目录。`Editor.vue`、`style.css` 使用可收起的组件/属性区和自适应画布。助手采用宽侧栏及页面内非模态浮窗，保留对话，并支持用户开启可撤回的已有组件自动微调；首次方案、新增、删除和画布尺寸变更仍预览。宿主工作区隐藏全局助手入口，避免两个入口叠加。

The runtime version label is removed. The MIT copyright and license ship in `public/third-party/GoView-LICENSE.txt`, beside separate asset notices. Collapsible palette/properties and an adaptive canvas maximize workspace. The assistant uses a wide dock or a local non-modal window, preserves conversation, and offers opt-in undoable refinements to existing widgets. Initial plans, additions, removals and canvas resizing still require preview. The host workspace hides the global assistant launcher.

双向桥接递归复制 Vue 包装的 GoView 类实例，保留上传 Blob；发送失败立即释放待处理 RPC。嵌入式加载和 AI 应用后初始化有效撤销基线。历史/告警轮询失败仅影响对应绑定，支持重试及恢复，不再覆盖编辑操作的错误状态。设备上下文补充产品、单位、枚举，并优先保留当前画布已绑定设备；仍执行租户/项目权限和目录数量上限。

Both bridge directions unwrap reactive GoView class data while preserving upload Blobs; failed sends release pending RPCs. Embedded loads and AI applications seed valid undo history. History/alarm polling failures affect only the binding, with retry and recovery, leaving editing error state intact. Device context includes product, units and enums, prioritizing existing bindings within tenant/project permissions and catalog bounds.

交互设计及验证记录见 `docs/ioc-workspace-assistant.md`。 / See `docs/ioc-workspace-assistant.md` for interaction decisions and verification.

## H. Inspector layout and IOC composition / 属性布局与 IOC 构图

`src/noyo/style.css` 隔离原生表单样式与 Naive UI 内部控件，属性栏采用可收缩网格。上游 `SettingItemBox.vue` 使用 `minmax(0,1fr)`；`CanvasPage/index.vue` 的尺寸、背景上传、清除操作及缩放设置改为流式布局，并为新增文案提供中英切换。`TimeCommon/index.vue` 首次渲染即读取当前时间；生成布局显式设置单行时钟字号和行高。

`src/noyo/style.css` isolates native form styles from Naive UI internals and keeps inspector grids shrinkable. Upstream `SettingItemBox.vue` uses `minmax(0,1fr)`. `CanvasPage/index.vue` has fluid size, upload, clear-action and scaling controls with bilingual new copy. `TimeCommon/index.vue` reads the current time on its first render; generated clocks receive explicit single-line typography.

`layout.ts`、`screenTheme.ts` 复用真实 `Border06` / `Decorates02` 配置，均分业务行剩余高度，并按完整设备名称预留多行空间。`preflight.ts` 对细装饰线采用专用尺寸门槛，保持越界、重叠及业务控件尺寸校验。`runtime.ts` 和 `Editor.vue` 在编辑与预览中标出空历史数据。业务方案与参考见 `docs/ioc-screen-quality.md`；浏览器回归入口为 `tooling/inspector.browser.cjs` 和 `tooling/quality-preview.browser.cjs`。

`layout.ts` and `screenTheme.ts` reuse the real `Border06` / `Decorates02` schema, share remaining height across business rows, and reserve multiline space for full device names. `preflight.ts` has a dedicated size threshold for thin decorative rules while retaining bounds, collision and business-control checks. `runtime.ts` and `Editor.vue` identify empty history data in edit and preview modes. See `docs/ioc-screen-quality.md` for decisions/references and the two browser regressions in `tooling/inspector.browser.cjs` and `tooling/quality-preview.browser.cjs`.
