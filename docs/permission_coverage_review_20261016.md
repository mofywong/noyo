# 最近三个月权限覆盖审查 / Permission coverage review

审查日期 / Date：2026-10-16

## 范围与依据 / Scope and evidence

- 时间范围：2026-07-16 至 2026-10-16。社区版以 `772a7a0c259215c4707493ec4b94c73781c8e810` 为迭代前基线，当前 HEAD 为 `09fe51259d160de41783f65edddd1c5ccb4de655`。
- 同时检查当前接入的 Pro 插件，Pro HEAD 为 `926723dea78b948ca79c55d9218952ea08787321`；包含工作区当前源码，不重置既有未提交改动。
- 对照权限目录、租户/项目权限上限、内置/自定义角色、应用授权、路由、侧栏、插件菜单、快捷授权、按钮和后台主动请求；继续核对接口权限、设备归属与标签权限。
- 图谱用于代码发现，并执行关键文件覆盖检查。索引对本次文件返回 `metadata_changed` 或 `coverage_unavailable`，结论以当前源码、Git 差异和下列针对性测试为依据，不将图谱视为完整性证明。

## 缺口与修复 / Findings and changes

| 功能 / Feature | 原缺口 / Gap | 补齐方式 / Change |
| --- | --- | --- |
| AI 预测 / AI prediction | 7 个接口直接向 HTTP server 注册，绕过插件统一认证；任务读写、锁存清除、重新校准缺少设备范围授权 | 接入 `RegisterHTTPHandler`，统一要求 `plugin:config`，读取按实际设备过滤，写入按设备与标签写权限预检；批量先整体授权；禁止跨设备接管 TaskID |
| 级联网关 / Cascade gateways | 扩展接口主要依赖登录，远程配置、日志、授权管理及冲突解决缺少功能和对象权限 | 19 个路由建立方法/权限表；查看用 `gateway:list`，远程管理用 `gateway:config`；校验实际租户、项目和成员范围 |
| 同步冲突 / Sync conflicts | 缺少资源类型权限和整批预授权 | 按资源检查查看/编辑权限；接受对端删除还需删除权限；检查对端资源 scope，整批授权通过后执行 |
| 模型中心 / Model center | 路由复用 `plugin:list`，侧栏入口未独立控制，导出没有单独权限 | 增加模型查看、导出、管理权限；菜单、路由、按钮和 API 同步接入；全局模型修改继续只允许系统管理员 |
| AI 场景 / AI scenes | 菜单依赖插件配置，场景修改复用设备编辑；普通设备编辑或导入可更改同一 AI 字段 | 独立场景查看/编辑权限；提供只返回 AI 字段的设备列表/详情入口；设备创建、修改、导入均检查 AI 字段变化；专用保存响应也脱敏 |
| 语义复核 / Semantic reviews | 查看记录和证据复用 `device:list`，页面无权时仍加载组件/请求 | 独立查看权限，后端继续核对设备范围与标签；前端无权时不挂载、不请求 |
| AI 助手 / AI assistant | 全局组件及页面触发入口缺少独立授权，扩展 API 仅要求登录 | 独立使用权限控制入口、组件、主动请求及聊天/会话/文件解析等接口；保留助手内部动作级权限校验；完整配置仍需 `plugin:config` |
| 通用扩展 / Plugin extensions | 语音启停、脚本调试等扩展仅认证；语音启停可以 GET 触发 | 统一 wrapper 增加功能授权与 HTTP 方法约束；脚本调试要求 `device:control` 并检查实际设备/标签写权限；授权状态、媒体网络等沿用相应系统权限 |
| 告警中心 / Alarm center | 处置判定只看告警状态，策略/导出按钮缺少权限，页面无条件请求关联资源 | 对齐 `alarm:handle`、`alarm:manage`、`work_order:create`，新增 `alarm:export` 控制客户端报告导出；关联目录与历史请求按其权限触发 |
| 设备详情 / Device details | 新版详情抽屉内新增子设备、映射、编辑、删除等按钮未与列表权限对齐 | 分别对齐 `device:create`、`device:edit`、`device:delete`；AI 预测配置使用 `plugin:config`，与摄像头 AI 场景区分 |
| 快捷授权 / Quick permissions | 缺少规则、工单与 AI 模块，网关“完全权限”包含不存在的 `gateway:delete` | 补齐模块及告警策略权限，移除不存在的 `gateway:delete` 和空“完全访问”选项；网关删除仍由设备模块的 `device:delete` 控制；新增权限文案支持中文和英文 |
| 全局状态 / Global status | 启动时无条件查询授权/MQTT 状态，级联状态对所有登录用户暴露连接信息 | 前端按权限请求；级联 status/SSE 无 `gateway:list` 时仅返回必要模式/插件信息，隐藏连接明细 |
| 设备对象授权 / Device scope | 标签鉴权在指定项目时直接使用请求项目，未读取设备实际项目；导入可按同编码覆盖其他 scope 的设备 | 标签鉴权以实际设备项目为准并核对选中项目；导入覆盖要求设备编辑权限且真实租户/项目一致 |

AI Brain、规则与工单等已有服务端独立权限继续保留，本次补齐其遗漏的快捷授权或 UI 入口，不重复新增同义权限。

## 新增权限 / New permissions

| 权限码 / Code | 用途 / Purpose | 可授权范围 / Scope |
| --- | --- | --- |
| `ai_scene:list` | 查看 AI 场景 / View AI scenes | 租户、项目、角色、应用 |
| `ai_scene:edit` | 配置 AI 场景 / Configure AI scenes | 租户、项目、角色、应用；受设备标签写权限约束 |
| `semantic_review:list` | 查看语义复核 / View semantic reviews | 租户、项目、角色、应用；受设备数据范围约束 |
| `ai_model:list` | 查看模型中心 / View model center | 租户、项目、角色、应用 |
| `ai_model:export` | 导出模型清单 / Export model inventory | 租户、项目、角色、应用 |
| `ai_model:manage` | 管理全局模型资产 / Manage global model assets | `system` 模块，仅系统管理员 |
| `ai_assistant:use` | 使用 AI 助手 / Use AI assistant | 租户、项目、角色、应用；内部操作仍需对应业务权限 |
| `alarm:export` | 导出告警报告 / Export alarm reports | 租户、项目、角色、应用；报告在客户端生成 |

## 升级行为 / Upgrade behavior

- 启动时幂等补齐权限目录和内置管理员角色，继续受租户/项目最大权限限制。
- 现有自定义角色以及显式自定义的租户/项目上限不会自动扩权。需要由管理员在相应上限、角色或应用授权中勾选新增权限。
- 模型资产为全局资源，管理操作保留系统管理员边界，不能通过租户/项目权限授予全局模型写权限。
- AI 场景专用接口不返回设备连接地址、密码等非 AI 配置。播放器、截图和媒体操作仍要求已有 `device:control` 权限。

## 验证 / Validation

- 社区版：`rtk go test ./core ./core/store -run '^(TestPermissionCoverage.*|TestModelAPIEmptyDeploymentAndAdminBoundary)$' -count=1`，41 项通过（含子用例）。
- AI 保存响应脱敏与独立应用授权：`rtk go test ./core -run '^(TestPermissionCoverageAIAppWrite|TestAIConfigAPIOutsideMaintenance)$' -count=1`，2 项通过。
- 设备真实项目边界修正后：`rtk go test ./core -run '^(TestPermissionCoverageTagProjectBoundary|TestPermissionCoverageAIAppWrite)$' -count=1`，2 项通过。
- 级联：`rtk go test ./plugins/platform/cascade -run '^TestCascade(HTTPPermissionBoundaries|StatusRedactsConnectionDetails|SyncConflictAuthorization)$' -count=1`，25 项通过（含子用例）。
- Pro 预测：`rtk go test noyo/plugins/pro/platform/ai_predict -run '^TestPredictHTTP(Registration|DeviceReadPermissions|DeviceWritePermissions)$' -count=1`，16 项通过（含子用例），覆盖项目间权限拼接、标签只读、批量写入先授权与任务身份隔离。
- 前端（社区版 `frontend` 目录）：`rtk node --test scripts/permission-coverage.test.mjs ../../noyo-pro/frontend/src/plugins/platform/yolo_pro/AiConfigCenter.semantic.test.mjs ../../noyo-pro/frontend/src/plugins/platform/yolo_pro/AiConfigCenter.asyncSave.test.mjs`，17/17 通过；同一专项脚本完成 13 个改动 Vue SFC 的 script/template 编译。
- 网关快捷授权选项调整后，仅重跑直接相关的 `rtk node --test scripts/permission-coverage.test.mjs`，6/6 通过（包含上述 13 个 Vue SFC 编译检查）。
- 两仓 `rtk git diff --check` 通过。

仅执行与改动直接相关的最小检查；未运行全量测试、生产构建或浏览器端人工验收。
