# Noyo GoView embedded editor

Source: Gitee `dromara/go-view`, `master-fetch`, commit `5ab05b439703ffda6d050e5626969459b3e3c1cf`, upstream version 2.2.9. `UPSTREAM.json` records the pinned build. `app/LICENSE` retains upstream MIT terms. `THIRD_PARTY_NOTICES.md` lists the third-party assets that are **not** covered by that MIT grant.

All 1002 upstream blobs were retrieved from the official Gitee tree/raw endpoints and checked against Git blob SHA-1. `tooling/fetch-upstream.cjs` reproduces retrieval; it is intended only for an empty source checkout because it restores upstream files. `tooling/patch-upstream.cjs` records the Noyo runtime patch; the new `src/noyo/` files are maintained directly. `patches/PATCHES.md` is the human-readable patch register: what was changed, why, and which upstream path it touches.

Build independently (Node 22 tested):

```powershell
cd integrations/goview/app
npm ci --ignore-scripts --no-audit --no-fund
$env:NOYO_SCREEN_BASE = '/screen-editor/goview-5ab05b4-noyo13/'
npm run build
node --test ../tooling/bridge.test.cjs
```

Output: `app/dist`, including `.noyo-build.json`. The Noyo root build copies these assets; this integration does not alter the host's Vue/ECharts dependency graph. `package-lock.json` is the active independent lock; upstream `pnpm-lock.yaml` is retained as source provenance.

`engineBuildId = upstream short SHA + adapter version` (see `UPSTREAM.json`). The source fingerprint is checked separately in `.noyo-build.json`. The current generation is `goview-5ab05b4-noyo9`; generations 1 through 8 remain accepted for existing documents. The root build preserves available previous directories under `backend/dist/screen-editor/`. It generates `.noyo-assets.json` with SHA256 for each current-engine file and verifies the copied output. Run the root build (`frontend/npm run build`) for release packaging; the standalone command above only builds the child application.

Before deployment, use the candidate binary's `screen-preflight --database <existing SQLite> [--dist <candidate dist>]`. It reads the database without migrations and checks all retained release and active draft engine references against embedded or external assets. Missing versions, missing digest manifests, unsafe links and altered/missing/unrecorded files block deployment. Old output without a digest manifest is never automatically attested: recover the exact trusted release package or rebuild its original source and explicitly migrate it. The backend stamps only the current build ID into new documents.

Open the editor only through the Noyo host, with `#/chart/home/:id?embed=1&channelId=...` or `#/chart/preview/:id?embed=1&channelId=...`. The `noyo.screen/1` bridge verifies parent window, parent origin, protocol and channel. The host owns API authentication, draft revisions, permissions, immutable releases, resources and shared realtime transport.

## IOC AI assistant / IOC AI 助手

从大屏列表的“AI 创建大屏”或编辑器的“IOC AI 助手”打开独立侧栏。描述组件、位置、尺寸、静态数据和设备绑定，检查方案后应用到画布；删除、解绑和撤回会再次确认。生成期间继续编辑会使旧方案失效。修改沿用现有草稿自动保存，发布仍由现有发布入口完成。

Open the dedicated sidebar from **AI create screen** in the screen list or **IOC AI assistant** in the editor. Describe widgets, positions, sizes, static data and device bindings, then review and apply the proposal. Removing widgets, unbinding and undo require confirmation. Further canvas edits invalidate an older proposal. Changes use the existing draft autosave and publication controls.

`POST /api/screens/:id/assistant/plan` 复用已启用的 AI 助手插件中首个配置完整的模型服务，但使用独立会话和受限的大屏方案协议。需要 `screen:edit`；设备目录按项目和设备权限过滤，告警绑定另需 `alarm:list`。上下文包含当前未保存画布、选中组件、组件能力、实际设备属性及有限的本地对话。服务端校验组件、几何范围、数据与绑定；模型不会直接写入草稿或调用全局助手工具。无模型配置时侧栏提示管理员配置服务。

`POST /api/screens/:id/assistant/plan` reuses the first fully configured model provider from the enabled AI assistant plugin, with a separate conversation and a restricted screen proposal protocol. It requires `screen:edit`; device visibility follows project and device permissions, and alarm bindings require `alarm:list`. Context includes the unsaved canvas, selected widgets, widget capabilities, real device properties and bounded local history. The server validates widgets, geometry, data and bindings. The model does not write drafts or use global assistant tools. The sidebar explains when an administrator must configure a provider.

历史草稿在编辑时使用当前适配器，并在实际保存编辑后升级引擎标识；已发布版本继续使用其固定引擎资源。外部请求、脚本和媒体 URL 不属于 AI 方案协议。

Historical drafts open in the current adapter for editing and upgrade their engine identity when edits are saved. Published releases retain their pinned engine assets. External requests, scripts and media URLs are outside the AI proposal protocol.

The embedded UI reuses GoView's actual `ContentEdit` canvas (drag, resize, selection, undo/redo, ruler/zoom) and the complete pinned component set: all 80 upstream definitions plus a Noyo-maintained `LocationMap` material, i.e. 81 palette entries in six groups with name search. Every component keeps its upstream appearance panel (`config.vue`) inside the Noyo inspector. The palette and inspector are restricted and bilingual; data bindings live in the document envelope. Runtime updates use material callbacks and are never saved as document content. Image URLs are transient host-resolved blob URLs; only `option.assetId` is persisted. The image library supports cursor pagination, original filenames, dimensions and authorized 256px previews; screen editors can reuse images uploaded by another editor of that screen.

Which component can carry device data, and in what payload, is declared once in `app/src/noyo/widgets.ts` (`shape`) and executed by `app/src/noyo/shapes.ts`: 39 components are data-bindable (charts, tables, numbers, text, map) and 42 stay static by design (borders, decorations, media URLs, form controls, and the two topology graphs that cannot be derived from telemetry). `tooling/catalogue.test.cjs` and `tooling/shapes.test.cjs` guard that contract.

Runtime bindings cover `point`, `history`, `deviceStatus`, `aggregate`, `location` and read-only `alarmList`. History and alarms refresh every 30 seconds through bounded host RPCs. Alarm reads require both device and alarm permissions, scan at most the latest 1000 open alarms in the current tenant/project and return at most 100 authorized rows without evidence. `truncated` reports that bound. The offline device map plots declared WGS84 or GCJ-02 positions and optional GeoJSON lines/polygons (1 MiB, 256 lines, 10000 positions). Imports retain only numeric geometry; outline and binding CRS must match. There is no coordinate conversion or external basemap dependency. Per-component quality (`good`, `stale`, `offline`, `no_data`, `access_denied`, `error`) is delivered separately from values. Video and advanced GIS/3D remain follow-up work.

Draft changes autosave after two seconds of idle time. JSON signatures ignore object-key ordering to avoid save loops after the Go API round trip. Manual save remains available. Concurrent edits use draft versions and fail with a conflict; the UI keeps unsaved changes for recovery instead of overwriting another draft.

The embedded entry has no upstream login, demo API, production mock, unrestricted request/data/script editor, original JSON editor or remote iframe material. Direct upstream axios/custom HTTP, filter/formatter revival and lifecycle/base event script execution are disabled. The original source is retained for traceability, but unsupported routes and the upstream request/data-source tabs are absent from the embedded build registry. `MapAmap` renders statically because it requires a third-party AMap key and network access.

Tests cover handshake spoof rejection, RPC matching/error handling/disposal and runtime revision/sample ordering with no document mutation. Browser integration and server validation remain the final authority for end-to-end behavior; this module does not treat a client-side component whitelist as a permission boundary.
