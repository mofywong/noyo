import { RECOGNITION_CATALOG, RECOGNITION_CATEGORIES } from './recognitionCatalog.js';

// The model center translates stable reason codes, never arbitrary server errors.
const messages = {
  zh: {
    title: '模型中心', subtitle: '选择要使用的识别能力，准备所需模型资源。',
    refresh: '刷新', search: '搜索识别能力', all: '全部', missing: '待补齐资源', installed: '资源就绪', blocked: '不可用',
    capabilities: '识别能力', resources: '模型资源', views: '模型中心视图', categories: '业务分类', otherCategory: '其他能力', packages: '模型包与版本', jobs: '安装任务', availableCount: '资源就绪的能力', taskCount: '进行中的任务',
    resourcesReady: '资源就绪', resourcesPending: '待启用', resourcesMissing: '待补齐资源', buildUnsupported: '当前程序不支持', runtimeMissing: '推理环境缺失', configure: '去 AI 识别配置',
    readOnly: '只有系统管理员可以安装、切换或卸载模型。',
    empty: '当前程序没有提供模型目录，请检查部署版本。', noMatch: '没有匹配的模型能力。', clearSearch: '清除筛选',
    dependencies: '依赖', version: '版本', source: '来源', license: '许可', contract: '兼容契约', files: '资源文件',
    unspecified: '未提供', none: '无', offlineOnly: '仅支持离线导入', customHint: '散列校验只确认文件与清单一致；此来源的发布者未经官方验证。',
    import: '导入离线包', fileLabel: '离线 ZIP 包', importHelp: '选择包含 manifest.json 的 ZIP 包；上传后先检查计划，再开始安装。',
    selectedFile: '已选择文件', importProgress: '上传中',
    jobsHelp: '离开页面后任务继续执行；重新进入可查看进度。', noJobs: '暂无安装任务。',
    progress: '安装进度', transferred: '已处理', package: '模型包', attempt: '尝试次数', cancel: '取消', retry: '重试',
    cancelTitle: '取消安装任务', cancelMessage: '取消当前任务？已发布的模型包会保留，尚未完成的资源不会激活。', keepTask: '保留任务',
    activate: '切换为活动版本', verify: '重新验证', uninstall: '卸载', activeVersion: '活动版本', candidateVersion: '候选版本',
    activateTitle: '切换模型版本', activateMessage: '切换到所选版本？请先暂停使用此模型的摄像头任务。',
    uninstallTitle: '卸载模型版本', uninstallMessage: '永久删除所选版本的模型文件？后续使用时需重新安装。',
    inUse: '正在使用，请先暂停相关任务。', referenced: '存在共享依赖或规则引用，暂时不能卸载。',
    notInstalled: '尚未安装', verifySuccess: '模型文件和推理兼容性验证通过。', activateSuccess: '活动版本已切换。',
    uninstallSuccess: '模型版本已卸载。', taskCreated: '安装任务已提交。', taskRetry: '任务已重新提交。', taskCancelled: '任务已取消。',
    fetchError: '无法获取模型目录，请重试。', operationError: '操作失败，请刷新后重试。', networkError: '连接失败，请检查网络后重试。',
    invalidFile: '请选择 ZIP 格式的离线包。', interruptedHint: '任务在服务中断后停止，可手动重试。',
    states: { missing: '缺失', incomplete: '不完整', installed: '已安装', ready: '可用', active: '活动版本', candidate: '候选版本', loaded: '已加载', unsupported: '当前程序不支持', runtime_missing: '推理环境缺失', offline_only: '待离线导入', queued: '等待中', downloading: '下载中', verifying: '校验中', publishing: '发布中', succeeded: '已完成', failed: '失败', cancelled: '已取消', interrupted: '已中断', unknown: '状态待确认' },
    reasons: { invalid_request: '请求无效，请刷新后重新选择。', not_found: '资源不存在或计划已失效，请重新预览。', unsupported: '当前程序未包含此算法或不支持该模型契约，请升级程序。', unsupported_build: '当前程序未包含此算法，请升级程序。', missing_dependency: '缺少必需的模型依赖，请补齐安装计划。', in_use: '模型正在使用，请先暂停相关任务。', referenced: '模型存在共享依赖或规则引用，请先解除引用。', integrity: '文件大小或 SHA256 校验失败，请重新下载或检查离线包。', archive_invalid: '离线包无效，请检查清单、文件和路径。', download_failed: '下载失败，请检查网络或使用离线导入。', runtime_missing: '推理环境缺失，请补齐运行库后重新验证。', validation_failed: '模型兼容性验证失败，请检查版本与模型契约。', cancelled: '安装任务已取消。', conflict: '存在冲突操作或未完成的安装，请稍后重试。', plan_stale: '安装计划或版本选择已过期，请刷新目录后重新预览。', storage_error: '无法写入模型目录，请检查空间和文件权限。', pending_activation: '模型已安装到本机，启用后即可使用。', forbidden: '需要系统管理员权限。', unauthorized: '登录已失效，请重新登录。' },
  },
  en: {
    title: 'Model center', subtitle: 'Choose recognition capabilities and prepare their required model resources.',
    refresh: 'Refresh', search: 'Search recognition capabilities', all: 'All', missing: 'Resources needed', installed: 'Resources ready', blocked: 'Unavailable',
    capabilities: 'Recognition capabilities', resources: 'Model resources', views: 'Model center views', categories: 'Business categories', otherCategory: 'Other capabilities', packages: 'Packages and versions', jobs: 'Installation tasks', availableCount: 'Capabilities with ready resources', taskCount: 'Active tasks',
    resourcesReady: 'Resources ready', resourcesPending: 'Awaiting activation', resourcesMissing: 'Resources needed', buildUnsupported: 'Unsupported in this build', runtimeMissing: 'Inference runtime missing', configure: 'Configure AI recognition',
    readOnly: 'Only system administrators can install, activate or uninstall models.',
    empty: 'This build has no model catalog. Check the deployed application version.', noMatch: 'No matching capabilities.', clearSearch: 'Clear filters',
    dependencies: 'Dependencies', version: 'Version', source: 'Source', license: 'License', contract: 'Compatibility contract', files: 'Resource files',
    unspecified: 'Not provided', none: 'None', offlineOnly: 'Offline import only', customHint: 'Hashes confirm that files match the manifest. The publisher of this source has not been verified.',
    import: 'Import offline package', fileLabel: 'Offline ZIP package', importHelp: 'Choose a ZIP containing manifest.json. Review its plan after uploading, then start installation.',
    selectedFile: 'Selected file', importProgress: 'Uploading',
    jobsHelp: 'Tasks continue after you leave this page. Return to see their progress.', noJobs: 'No installation tasks yet.',
    progress: 'Installation progress', transferred: 'Processed', package: 'Package', attempt: 'Attempts', cancel: 'Cancel', retry: 'Retry',
    cancelTitle: 'Cancel installation', cancelMessage: 'Cancel this task? Published packages will be kept. Incomplete resources will not be activated.', keepTask: 'Keep task',
    activate: 'Activate version', verify: 'Verify again', uninstall: 'Uninstall', activeVersion: 'Active version', candidateVersion: 'Candidate version',
    activateTitle: 'Activate model version', activateMessage: 'Switch to the selected version? Pause camera tasks using this model first.',
    uninstallTitle: 'Uninstall model version', uninstallMessage: 'Permanently delete this version’s files? They will need to be installed again for future use.',
    inUse: 'In use. Pause related tasks first.', referenced: 'Shared dependencies or rules still reference this model; it cannot be uninstalled.',
    notInstalled: 'Not installed', verifySuccess: 'Model files and inference compatibility verified.', activateSuccess: 'Active version changed.',
    uninstallSuccess: 'Model version uninstalled.', taskCreated: 'Installation task submitted.', taskRetry: 'Task resubmitted.', taskCancelled: 'Task cancelled.',
    fetchError: 'Cannot retrieve the model catalog. Try again.', operationError: 'Operation failed. Refresh and try again.', networkError: 'Connection failed. Check your network and try again.',
    invalidFile: 'Choose an offline package in ZIP format.', interruptedHint: 'This task stopped after a service interruption. Retry it manually.',
    states: { missing: 'Missing', incomplete: 'Incomplete', installed: 'Installed', ready: 'Available', active: 'Active version', candidate: 'Candidate version', loaded: 'Loaded', unsupported: 'Unsupported by this build', runtime_missing: 'Runtime missing', offline_only: 'Awaiting offline import', queued: 'Queued', downloading: 'Downloading', verifying: 'Verifying', publishing: 'Publishing', succeeded: 'Completed', failed: 'Failed', cancelled: 'Cancelled', interrupted: 'Interrupted', unknown: 'Status unknown' },
    reasons: { invalid_request: 'Invalid request. Refresh and select again.', not_found: 'The resource or plan is no longer available. Create a new preview.', unsupported: 'This build lacks the algorithm or model contract. Upgrade the application.', unsupported_build: 'This build lacks the algorithm. Upgrade the application.', missing_dependency: 'Required model dependencies are missing. Complete the installation plan.', in_use: 'The model is in use. Pause related tasks first.', referenced: 'Shared dependencies or rules reference this model. Remove those references first.', integrity: 'File size or SHA256 verification failed. Download again or check the offline package.', archive_invalid: 'Invalid offline package. Check its manifest, files and paths.', download_failed: 'Download failed. Check your network or use an offline import.', runtime_missing: 'Inference runtime is missing. Install the required runtime, then verify again.', validation_failed: 'Model compatibility verification failed. Check the version and model contract.', cancelled: 'Installation cancelled.', conflict: 'A conflicting operation or installation is pending. Try again later.', plan_stale: 'The installation plan or version selection is stale. Refresh the catalog and preview again.', storage_error: 'Cannot write model files. Check disk space and directory permissions.', pending_activation: 'The model is installed on this machine and only needs activation.', forbidden: 'System administrator permission is required.', unauthorized: 'Your session has expired. Sign in again.' },
  },
};

Object.assign(messages.zh.reasons, {
  disk_insufficient: '磁盘空间不足，请释放空间后重试。',
  model_not_ready: '所需模型尚未就绪，请先安装完整依赖。',
  model_root_changed: '模型目录已变更，请重启服务。',
  archive_too_large: '资源包超过大小限制，请检查离线包。',
  manifest_root_missing: 'ZIP 根目录缺少 manifest.json；请移除外层文件夹后重新打包。',
  archive_file_type: '请选择或拖放 ZIP 格式的模型安装包。',
  archive_multiple_files: '一次只能选择一个 ZIP 模型安装包。',
  history_unavailable: '无法获取历史记录，请重试。',
  store_locked: '模型目录由其他进程管理，请检查服务实例。',
});
Object.assign(messages.zh, {
  version: '包发布版本', catalogVersion: '目录发布版本', packageId: '包 ID', provider: '算法提供方', kind: '资源类型',
  resourceKinds: { general: '通用检测模型', helmet: '安全帽与安全背心模型', pose: '人体姿态模型', smoking: '香烟检测模型', chef_hat: '厨师帽模型', chef_hat_hairnet: '发网增强模型', kitchen: '厨房卫生检测模型', fighting: '打架分析模型' },
  algorithms: '直接支持的识别能力', requiredBy: '作为依赖支持', currentResources: '所需模型',
  legacyHint: '此类资源使用旧目录文件，未纳入版本管理；其实际版本未知，不能视为此目录发布版本已安装。',
  legacyResource: '旧目录资源（版本未知）', versionHelp: '包版本由发布者在 manifest.json 中定义；内置发布清单随程序提供。兼容契约版本独立定义输入、输出和类别映射。',
  usageHelp: '模型资源在多个识别项目之间共享；安装模型不会自动启用摄像头规则。',
});
Object.assign(messages.en, {
  version: 'Package release', catalogVersion: 'Catalog release', packageId: 'Package ID', provider: 'Algorithm provider', kind: 'Resource kind',
  resourceKinds: { general: 'General detection model', helmet: 'Safety helmet and vest model', pose: 'Person pose model', smoking: 'Cigarette detection model', chef_hat: 'Chef hat model', chef_hat_hairnet: 'Hairnet enhancement model', kitchen: 'Kitchen hygiene detection model', fighting: 'Fighting analysis model' },
  algorithms: 'Directly supported recognition capabilities', requiredBy: 'Supports as a dependency', currentResources: 'Required models',
  legacyHint: 'This resource kind uses unmanaged files from the legacy directory. Their version is unknown; this catalog release is not confirmed installed.',
  legacyResource: 'Legacy resources (version unknown)', versionHelp: 'Publishers define package releases in manifest.json; built-in release descriptors ship with the application. Contract versions separately define inputs, outputs and class mappings.',
  usageHelp: 'Model resources are shared across recognition items. Installing a model does not enable camera rules.',
});
messages.zh.states.legacy = '使用旧目录资源';
messages.en.states.legacy = 'Legacy resources in use';

// ── 2026-10-12 重构：面向使用者的措辞 ──────────────────────────────
// 设计依据 docs/plans/2026-10-12-model-center-redesign-design.md §7。
// 首屏（AI 能力视图）不得出现「契约 / 散列 / 包 ID / 发布版本 / 安装计划」。
Object.assign(messages.zh, {
  subtitle: '让摄像头看懂现场：挑能力、装模型、配规则。',
  tabCapabilities: 'AI 能力', tabLibrary: '模型库', tabDistribution: '导入离线包', tabs: '模型中心标签页',
  adminOnlyTabs: '后两项仅系统管理员可见',
  statusAvailable: '可用', statusPending: '已装待启用', statusNeedsInstall: '待安装', statusUnsupported: '当前程序不支持',
  statusSummary: '能力状态总览', statusShowAll: '显示全部能力',
  cardReconfigure: '去配置规则', cardDetails: '查看详情',
  cardActivate: '立即启用',
  cardPendingActivation: '模型已装到本机，启用后即可使用；正在进行的识别任务结束后会自动启用。',
  cardReady: '已就绪，可直接在摄像头中配置规则',
  cardNeedsPackages: '需要导入厂商提供的离线包：{count} 个模型包 · {size}',
  cardNeedsOffline: '需要导入厂商提供的离线包',
  cardUnsupported: '当前程序版本不支持，需升级程序',
  cardRuntimeMissing: '缺少运行组件，请补齐运行库',
  cardClassMissing: '当前模型不含所需类别，需安装支持该类的模型',
  cardNoop: '资源已完整安装，无需操作',
  installing: '安装中', cancelInstall: '取消',
  installingFile: '{name} · {done} / {total}',
  details: '详情', technicalDetails: '技术信息', closeDetails: '关闭',
  activityStrip: '进行中的安装任务', activityExpand: '展开', activityCollapse: '收起',
  goDistribution: '前往离线导入',
  // ── P1 模型库 / Model library ──
  libSearch: '搜索模型包或被服务的 AI 能力',
  colPackage: '模型包', colKind: '类型', colVersion: '当前版本', colState: '状态',
  colUsedBy: '服务的 AI 能力', colActions: '操作',
  libEmptyFiltered: '没有匹配的模型包。',
  libTotal: '模型包', libInstalled: '已安装', libActive: '使用中', libReleases: '共 {count} 个版本',
  moreActions: '更多操作', viewDetail: '查看详情',
  versionHistory: '版本历史', versionCurrent: '当前', versionCount: '{count} 个版本',
  versionRollback: '回退到该版本', versionSwitch: '切换到该版本',
  installedReleases: '已安装版本', unknownVersion: '版本未知',
  copyChecksum: '复制校验码', checksumCopied: '校验码已复制',
  drawerIdentity: '模型包身份', drawerUsage: '使用关系', drawerResources: '资源文件',
  usedByDirect: '直接支持', usedByDependency: '作为依赖支持', usedByOptional: '可选增强',
  noUsage: '暂无 AI 能力使用此模型包。',
  reserved: '预留',
  legacyTitle: '旧目录资源',
  legacyNotice: '此类资源使用旧目录文件，未纳入版本管理，实际版本未知。导入替代包后会创建候选版本，需管理员明确切换。',
  // ── P2 离线导入 / Offline import ──
  distImportTitle: '导入离线包',
  distImportHelp: '选择厂商提供的 ZIP 离线包，点击一次即可完成上传、解析、安装与启用。',
  distImportChoose: '选择离线 ZIP 文件',
  distImportAction: '导入并安装',
  distImportAgain: '重新导入',
  // 从能力卡跳转过来时的上下文提示
  importContextHint: '「{name}」需要：{packages}。请导入厂商提供的离线包，导入后自动安装并启用。',
  // 四阶段 / Four phases
  phaseUpload: '上传离线包',
  phaseParse: '解析包内容',
  phaseInstall: '安装到本机',
  phaseActivate: '启用新模型',
  importRunning: '正在导入，请勿关闭页面。',
  importSucceeded: '导入完成',
  importFailed: '导入失败',
  importActivatedCount: '已启用 {count} 项',
  importPendingCount: '待启用 {count} 项',
  importPendingHint: '以下模型已安装到本机，但尚未接管对应的资源类型。',
  importPendingInUse: '正在使用中，运行中的会话结束后会自动启用。',
  importPendingDependency: '缺少前置模型包，请连同依赖一起重新导入。',
  importActivateNow: '立即启用',
  importViewCapabilities: '查看能力',
  importRowWait: '等待安装',
  importRowInstalling: '安装中',
  importRowActivating: '启用中',
  importRowActivated: '已启用',
  importRowPending: '待启用',
  importRowInstalled: '已安装',
  importRowFailed: '失败',
  distPlanTitle: '程序识别出的内容',
  distPlanPackages: '离线包内共 {count} 个模型包',
  distPlanNew: '新增', distPlanReplace: '替换待激活', distPlanReuse: '已有可用，跳过',
  distPlanCapabilities: '服务的能力',
  distPlanOffline: '此计划为离线安装，不需要联网下载。',
  distPlanBlocked: '此离线包当前无法安装，请检查上表。',
  distPlanMissingDependency: '离线包缺少内置依赖：{names}。请先安装这些内置包，或在清单中包含其固定版本。',
  // ── P3 打磨 / Polish ──
  // 骨架屏
  loadingCatalog: '正在读取模型目录',
  loadingLibrary: '正在读取模型库',
  // 空状态
  emptySearchHint: '试试{action}。',
  emptySearchAction: '清除搜索或筛选条件',
  emptyLibrary: '模型目录中没有可管理的模型包。',
  emptyLibraryFiltered: '没有符合当前筛选的模型包。',
  emptyFiltered: '没有符合当前筛选的 AI 能力。',
  // Toast / 操作结果
  toastJobCancelled: '安装任务已取消。',
  toastJobRetried: '安装任务已重新排队。',
  toastActivated: '{name} 已切换为使用中版本。',
  toastUninstalled: '{name} 已卸载。',
  toastVerified: '{name} 校验通过。',
  // 无障碍
  a11yCapabilityList: 'AI 能力列表',
  a11yCapabilityCard: '{name}，状态：{status}',
  a11yLibraryTable: '模型包列表',
  a11yImportTable: '离线包内容列表',
  a11yJobProgress: '{name} 安装进度',
  a11yJobRegion: '进行中的安装任务',
});

Object.assign(messages.en, {
  subtitle: 'Teach cameras to understand your site: pick a capability, install its model, configure rules.',
  tabCapabilities: 'Capabilities', tabLibrary: 'Model library', tabDistribution: 'Import package', tabs: 'Model center tabs',
  adminOnlyTabs: 'The last two tabs are for system administrators',
  statusAvailable: 'Available', statusPending: 'Installed — awaiting activation', statusNeedsInstall: 'Needs install', statusUnsupported: 'Unsupported by this build',
  statusSummary: 'Capability status summary', statusShowAll: 'Show all capabilities',
  cardReconfigure: 'Configure rules', cardDetails: 'View details',
  cardActivate: 'Activate now',
  cardPendingActivation: 'The model is already on this machine and only needs activation; it activates automatically once running recognition tasks end.',
  cardReady: 'Ready — configure camera rules',
  cardNeedsPackages: 'Import the offline package supplied by the vendor: {count} packages · {size}',
  cardNeedsOffline: 'Import the offline package supplied by the vendor',
  cardUnsupported: 'Not supported by this build — upgrade the app',
  cardRuntimeMissing: 'Missing runtime — install the required runtime',
  cardClassMissing: 'The active model lacks this class — install one that provides it',
  cardNoop: 'Resources are already complete; nothing to install',
  installing: 'Installing', cancelInstall: 'Cancel',
  installingFile: '{name} · {done} / {total}',
  details: 'Details', technicalDetails: 'Technical details', closeDetails: 'Close',
  activityStrip: 'Installation tasks in progress', activityExpand: 'Expand', activityCollapse: 'Collapse',
  goDistribution: 'Go to offline import',
  // ── P1 Model library ──
  libSearch: 'Search model packages or the AI capabilities they serve',
  colPackage: 'Package', colKind: 'Kind', colVersion: 'Current version', colState: 'Status',
  colUsedBy: 'Serves these AI capabilities', colActions: 'Actions',
  libEmptyFiltered: 'No matching model packages.',
  libTotal: 'Packages', libInstalled: 'Installed', libActive: 'In use', libReleases: '{count} releases',
  moreActions: 'More actions', viewDetail: 'View details',
  versionHistory: 'Version history', versionCurrent: 'Current', versionCount: '{count} releases',
  versionRollback: 'Roll back to this version', versionSwitch: 'Switch to this version',
  installedReleases: 'Installed releases', unknownVersion: 'Version unknown',
  copyChecksum: 'Copy checksum', checksumCopied: 'Checksum copied',
  drawerIdentity: 'Package identity', drawerUsage: 'Usage', drawerResources: 'Resource files',
  usedByDirect: 'Directly supports', usedByDependency: 'Supports as a dependency', usedByOptional: 'Optionally enhances',
  noUsage: 'No AI capability uses this package yet.',
  reserved: 'reserved',
  legacyTitle: 'Legacy directory resources',
  legacyNotice: 'This kind uses unmanaged files from the legacy directory; its actual version is unknown. Importing a replacement creates a candidate release that an administrator must activate explicitly.',
  // ── P2 Offline import ──
  distImportTitle: 'Import an offline package',
  distImportHelp: 'Choose the ZIP archive supplied by the vendor. One click covers upload, parsing, installation and activation.',
  distImportChoose: 'Choose an offline ZIP file',
  distImportAction: 'Import and install',
  distImportAgain: 'Import another package',
  // Context hint shown when arriving from a capability card
  importContextHint: '“{name}” needs: {packages}. Import the offline package supplied by the vendor; it installs and activates automatically.',
  // Four phases
  phaseUpload: 'Uploading package',
  phaseParse: 'Parsing contents',
  phaseInstall: 'Installing locally',
  phaseActivate: 'Activating models',
  importRunning: 'Import in progress — keep this page open.',
  importSucceeded: 'Import complete',
  importFailed: 'Import failed',
  importActivatedCount: '{count} activated',
  importPendingCount: '{count} awaiting activation',
  importPendingHint: 'These packages are installed but do not own their resource kind yet.',
  importPendingInUse: 'In use — activates automatically once running sessions end.',
  importPendingDependency: 'A prerequisite package is missing. Re-import it together with its dependencies.',
  importActivateNow: 'Activate now',
  importViewCapabilities: 'View capabilities',
  importRowWait: 'Waiting',
  importRowInstalling: 'Installing',
  importRowActivating: 'Activating',
  importRowActivated: 'Activated',
  importRowPending: 'Awaiting activation',
  importRowInstalled: 'Installed',
  importRowFailed: 'Failed',
  distPlanTitle: 'What the application identified',
  distPlanPackages: '{count} model package(s) inside this offline archive',
  distPlanNew: 'New', distPlanReplace: 'Replacement to activate', distPlanReuse: 'Already available — skipped',
  distPlanCapabilities: 'Capabilities served',
  distPlanOffline: 'This plan installs offline and needs no network download.',
  distPlanBlocked: 'This offline package cannot be installed right now. Review the table above.',
  distPlanMissingDependency: 'The offline archive lacks built-in dependencies: {names}. Install those built-in packages first, or include their pinned releases in the manifest.',
  // ── P3 Polish ──
  // Skeletons
  loadingCatalog: 'Loading the model catalog',
  loadingLibrary: 'Loading the model library',
  // Empty states
  emptySearchHint: 'Try to {action}.',
  emptySearchAction: 'clear the search or filters',
  emptyLibrary: 'No manageable model package in the catalog.',
  emptyLibraryFiltered: 'No model package matches the current filters.',
  emptyFiltered: 'No AI capability matches the current filters.',
  // Toasts / operation results
  toastJobCancelled: 'Installation task cancelled.',
  toastJobRetried: 'Installation task queued again.',
  toastActivated: '{name} is now the active release.',
  toastUninstalled: '{name} uninstalled.',
  toastVerified: '{name} passed validation.',
  // Accessibility
  a11yCapabilityList: 'AI capability list',
  a11yCapabilityCard: '{name}, status: {status}',
  a11yLibraryTable: 'Model package list',
  a11yImportTable: 'Offline package contents list',
  a11yJobProgress: '{name} installation progress',
  a11yJobRegion: 'Installation tasks in progress',
});
Object.assign(messages.en.reasons, {
  disk_insufficient: 'Insufficient disk space. Free space and retry.',
  model_not_ready: 'Required models are not ready. Install the complete dependency set.',
  model_root_changed: 'The model directory changed. Restart the service.',
  archive_too_large: 'The resource package exceeds the size limit. Check the offline archive.',
  manifest_root_missing: 'manifest.json is missing at the ZIP root. Remove the wrapper folder and repack.',
  archive_file_type: 'Choose or drop a ZIP model installation package.',
  archive_multiple_files: 'Choose only one ZIP model installation package at a time.',
  history_unavailable: 'Cannot load history. Try again.',
  store_locked: 'Another process manages the model directory. Check service instances.',
});
const reasonAliases = {
  admin_required: 'forbidden', algorithm_not_built: 'unsupported_build',
  model_in_use: 'in_use', model_referenced: 'referenced', invalid_archive: 'archive_invalid',
  missing_dependencies: 'missing_dependency', dependency_conflict: 'missing_dependency',
  busy: 'conflict', unsupported_contract: 'validation_failed', package_too_large: 'archive_too_large',
  job_not_found: 'not_found', plan_not_found: 'not_found', package_not_found: 'not_found', version_not_found: 'not_found',
};

messages.zh.reasons.model_class_missing = '当前活动模型不支持此识别项目，请在模型资源中准备并切换支持该类别的模型。';
messages.en.reasons.model_class_missing = 'The active model does not support this recognition item. Prepare and activate a model supporting this class in Model resources.';

Object.assign(messages.zh, {
  demandTitle: '算法需求清单', demandCount: '已选择 {count} 个算法', demandHelp: '勾选需要的算法，导出清单交给供应方打包。清单包含完整依赖和已有模型。',
  selectAlgorithm: '选择此算法', selectVisible: '全选当前结果', clearSelection: '清空选择', exportDemandJSON: '导出需求 JSON', exportDemandCSV: '导出需求 CSV', exportDeliveryCatalog: '导出发布目录',
  demandPreview: '预览清单 · 共需 {count} 个模型', optionalDemand: '可选增强（默认不包含）',
  confirmImport: '确认安装并启用', submitImportRetry: '重试提交安装', previewImportHelp: '请核对版本与影响算法。确认后启用新版本，旧版本保留供回退。',
  distImportAction: '上传并预览', distPlanReplace: '替换活动版本', importPreviousVersion: '当前 {version}',
  importedSource: '离线导入', builtinSource: '内置', customSource: '自定义',
});
Object.assign(messages.en, {
  demandTitle: 'Algorithm requirements', demandCount: '{count} algorithms selected', demandHelp: 'Select algorithms and send the exported request to your supplier. It includes all dependencies and existing models.',
  selectAlgorithm: 'Select this algorithm', selectVisible: 'Select current results', clearSelection: 'Clear selection', exportDemandJSON: 'Export request JSON', exportDemandCSV: 'Export request CSV', exportDeliveryCatalog: 'Export release catalog',
  demandPreview: 'Preview request · {count} models required', optionalDemand: 'Optional enhancements (excluded by default)',
  confirmImport: 'Confirm installation and activation', submitImportRetry: 'Retry installation submission', previewImportHelp: 'Review versions and affected algorithms. Confirmation activates the new releases and retains older releases for rollback.',
  distImportAction: 'Upload and preview', distPlanReplace: 'Replace active version', importPreviousVersion: 'Current {version}',
  importedSource: 'Offline import', builtinSource: 'Built-in', customSource: 'Custom',
});

Object.assign(messages.zh, {
  subtitle: '查看场景能力，管理本机模型及版本。', tabCapabilities: '场景能力',
  search: '搜索场景能力', libSearch: '搜索模型名称或版本',
  sceneAvailable: '可用', sceneUnavailable: '不可用', emptyScenes: '当前系统暂无场景能力。',
  updateModels: '更新模型', closeUpdate: '关闭更新', exportModels: '导出模型信息',
  exportingModels: '导出中', installed: '已安装', notInstalled: '未安装', version: '版本',
  colVersion: '当前版本', colUsedBy: '支持的场景', a11yCapabilityList: '场景能力列表',
  emptyInstalledModels: '当前没有已安装的模型。',
  distImportHelp: '点击选择或拖放一个根目录含 manifest.json 的单模型 ZIP 安装包，核对模型和版本后确认安装，后端将自动安装并生效。',
  distImportTitle: '更新模型', distImportChoose: '选择模型安装包',
  importViewCapabilities: '查看场景能力',
  chooseVersion: '选择活动版本', selectModelVersion: '选择 {name} 的活动版本',
});
Object.assign(messages.en, {
  subtitle: 'View scenario capabilities and manage installed models and versions.', tabCapabilities: 'Scenario capabilities',
  search: 'Search scenario capabilities', libSearch: 'Search model names or versions',
  sceneAvailable: 'Available', sceneUnavailable: 'Unavailable', emptyScenes: 'No scenario capabilities in this system.',
  updateModels: 'Update models', closeUpdate: 'Close update', exportModels: 'Export model information',
  exportingModels: 'Exporting', installed: 'Installed', notInstalled: 'Not installed', version: 'Version',
  colVersion: 'Current version', colUsedBy: 'Supported scenarios', a11yCapabilityList: 'Scenario capability list',
  emptyInstalledModels: 'No models are installed.',
  distImportHelp: 'Click to choose or drop one single-model ZIP package with manifest.json at its root. Review its model and version, then confirm; the backend installs and activates it automatically.',
  distImportTitle: 'Update models', distImportChoose: 'Choose a model installation package',
  importViewCapabilities: 'View scenario capabilities',
  chooseVersion: 'Choose an active version', selectModelVersion: 'Choose the active version of {name}',
});

export function modelCenterCopy(locale = 'zh') {
  return messages[String(locale).startsWith('zh') ? 'zh' : 'en'];
}

Object.assign(messages.zh, {
  viewVersions: '查看版本', viewVersionsFor: '查看 {name} 的版本', historyRecords: '历史记录', viewHistoryFor: '查看 {name} 的历史记录',
  noActiveVersion: '未启用', versionNumber: '版本号', versionDescription: '版本描述', installedAt: '安装时间', noVersionDescription: '未提供版本描述。',
  noVersions: '暂无可查看的模型版本。', deleteVersion: '删除此版本', historyEmpty: '暂无历史记录。', historyLoading: '正在加载历史记录…',
  historyLoadError: '无法获取历史记录，请重试。', historyTotal: '共 {count} 条记录', historyPage: '第 {page} 页，共 {pages} 页',
  historyPrevious: '上一页', historyNext: '下一页', historyAction: '操作', historyAdded: '安装', historyDeleted: '删除', historySwitched: '切换',
  historyOperator: '操作人', historyTime: '操作时间', historyPreviousVersion: '原版本', systemOperator: '系统',
});
Object.assign(messages.en, {
  viewVersions: 'View versions', viewVersionsFor: 'View versions of {name}', historyRecords: 'History', viewHistoryFor: 'View history of {name}',
  noActiveVersion: 'Inactive', versionNumber: 'Version', versionDescription: 'Version description', installedAt: 'Installed at', noVersionDescription: 'No version description provided.',
  noVersions: 'No model versions to display.', deleteVersion: 'Delete this version', historyEmpty: 'No history yet.', historyLoading: 'Loading history…',
  historyLoadError: 'Cannot load history. Try again.', historyTotal: '{count} records', historyPage: 'Page {page} of {pages}',
  historyPrevious: 'Previous', historyNext: 'Next', historyAction: 'Action', historyAdded: 'Installed', historyDeleted: 'Deleted', historySwitched: 'Switched',
  historyOperator: 'Operator', historyTime: 'Time', historyPreviousVersion: 'Previous version', systemOperator: 'System',
});

export function modelTimestamp(value, locale) {
  if (!value) return modelCenterCopy(locale).unspecified;
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return modelCenterCopy(locale).unspecified;
  return new Intl.DateTimeFormat(String(locale).startsWith('zh') ? 'zh-CN' : 'en-US', {
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).format(date);
}

export function modelText(value, locale, fallback = '') {
  if (!value || typeof value !== 'object') return fallback;
  const language = String(locale).startsWith('zh') ? 'zh' : 'en';
  return value[language] || fallback;
}

export function unwrapModelResponse(response) {
  const body = response && Object.hasOwn(response, 'code') ? response : response?.data ?? response;
  if (body && Object.hasOwn(body, 'code') && Number(body.code) !== 0) {
    throw { response: { data: body, status: Number(body.code) } };
  }
  return body && Object.hasOwn(body, 'code') ? body.data : body;
}

const list = value => Array.isArray(value) ? value : [];
const finite = value => Math.max(0, Number.isFinite(Number(value)) ? Number(value) : 0);

export function normalizeModelPackage(row = {}) {
  const pkg = row.package || row;
  return {
    ...pkg, descriptor: pkg, usage: row.usage || null, id: pkg.id || pkg.package_id || '', version: String(pkg.version || ''),
    description: row.description || pkg.description || {}, installedAt: row.installedAt || row.installed_at || pkg.installedAt || pkg.installed_at || '',
    files: list(pkg.files), dependencies: list(pkg.dependencies),
    offlineOnly: pkg.offlineOnly === true || pkg.offline_only === true,
    installed: row.installed === true, active: row.active === true,
    inUse: row.inUse === true || row.in_use === true || finite(row.refCount ?? row.ref_count) > 0,
    references: list(row.references), state: row.state || 'missing',
    reasonCode: row.reasonCode || row.reason_code || '',
  };
}

export function normalizeModelCatalog(data = {}) {
  return {
    // The backend owns this decision. Missing metadata always means read-only.
    canManage: data.can_manage === true,
    generation: data.generation ?? null,
    capabilities: list(data.capabilities).map(row => {
      const capability = row.capability || row;
      const definition = RECOGNITION_CATALOG.find(item => item.capabilityId === capability.id || item.id === capability.sceneType || item.id === capability.id);
      const reasonCode = row.reasonCode || row.reason_code || capability.unsupportedReason || '';
      return { ...capability, id: capability.id || '', category: capability.category || definition?.category || 'other', icon: capability.icon || definition?.icon || 'bi-cpu', sceneType: capability.sceneType || definition?.id || '', packages: list(capability.packages), optionalPackages: list(capability.optionalPackages), dependencies: list(capability.dependencies), missing: list(row.missing), unactivated: list(row.unactivated), state: row.loaded === true ? 'loaded' : row.state || 'missing', reasonCode, installable: row.state !== 'unsupported' && !capability.unsupportedReason && reasonCode !== 'model_class_missing' && row.installable !== false };
    }),
    packages: list(data.packages).map(normalizeModelPackage),
    jobs: list(data.jobs).map(normalizeModelJob),
  };
}

export function normalizeModelPlan(data = {}) {
  const plan = data.plan || data;
  const usage = list(plan.usage);
  const packages = list(plan.packages).map(pkg => normalizeModelPackage({ package: pkg, usage: usage.find(row => row.packageId === pkg.id && row.version === pkg.version) }));
  return { id: plan.id || plan.plan_id || '', capabilities: list(plan.capabilities || plan.capability_ids), usage, packages, offline: plan.offline === true, bytes: finite(plan.downloadBytes ?? plan.download_bytes), requiredDiskBytes: finite(plan.requiredDiskBytes ?? plan.required_disk_bytes), blocked: plan.can_install === false || (!plan.offline && packages.some(pkg => pkg.offlineOnly || pkg.files.some(file => !file.url && !file.inline))), blockers: list(plan.blockers) };
}

export function modelRecognitionCategories(catalog) {
  const categories = RECOGNITION_CATEGORIES.filter(category => catalog.capabilities.some(item => item.category === category.id));
  const unknown = [...new Set(catalog.capabilities.map(item => item.category))].filter(id => !RECOGNITION_CATEGORIES.some(category => category.id === id));
  return [...categories, ...unknown.map(id => ({ id, icon: 'bi-cpu', name: { zh: modelCenterCopy('zh').otherCategory, en: modelCenterCopy('en').otherCategory } }))];
}

export function modelCapabilityStateText(capability, locale) {
  const copy = modelCenterCopy(locale);
  const state = capability.state;
  if (state === 'unsupported') return copy.buildUnsupported;
  if (state === 'runtime_missing') return copy.runtimeMissing;
  if (state === 'candidate') return copy.resourcesPending;
  return ['ready', 'installed', 'loaded'].includes(state) ? copy.resourcesReady : copy.resourcesMissing;
}

const matchesModelQuery = (values, query) => String(query).toLowerCase().trim().split(/\s+/).filter(Boolean).every(term => values.join(' ').toLowerCase().includes(term));

export function filterModelCapabilities(catalog, query = '', category = 'all', status = 'all') {
  // 遗留状态词汇表（missing/installed/blocked）。`candidate` 已装但尚未接管，
  // 对使用者而言仍不可用，因此归入 missing，避免被状态筛选整类丢弃。
  const states = { missing: ['missing', 'incomplete', 'offline_only', 'candidate'], installed: ['ready', 'installed', 'loaded'], blocked: ['unsupported', 'runtime_missing'] };
  return catalog.capabilities.filter(item => {
    if (category !== 'all' && item.category !== category) return false;
    if (status !== 'all' && !states[status]?.includes(item.state)) return false;
    const group = RECOGNITION_CATEGORIES.find(group => group.id === item.category);
    const packages = item.packages.flatMap(id => catalog.packages.filter(pkg => pkg.id === id));
    return matchesModelQuery([item.id, item.sceneType, ...Object.values(item.title || {}), ...Object.values(item.description || {}), ...Object.values(group?.name || {}), ...packages.flatMap(pkg => [pkg.id, ...Object.values(pkg.title || {})])], query);
  }).sort((a, b) => {
    const order = item => { const index = RECOGNITION_CATALOG.findIndex(definition => definition.capabilityId === item.id || definition.id === item.sceneType); return index < 0 ? RECOGNITION_CATALOG.length : index; };
    return order(a) - order(b);
  });
}

export function filterModelPackages(catalog, query = '') {
  return catalog.packages.filter(pkg => {
    const usage = modelPackageUsage(pkg, catalog);
    const capabilities = catalog.capabilities.filter(item => [...usage.capabilities, ...usage.requiredBy, ...modelOptionalCapabilities(pkg, catalog)].includes(item.id));
    return matchesModelQuery([pkg.id, pkg.kind, ...Object.values(pkg.title || {}), ...Object.values(pkg.description || {}), ...capabilities.flatMap(item => Object.values(item.title || {}))], query);
  });
}

export function modelOptionalCapabilities(pkg, catalog) {
  return catalog.capabilities.filter(item => item.state !== 'unsupported' && item.optionalPackages.some(id => catalog.packages.some(expected => expected.id === id && compatibleModel(expected, pkg)))).map(item => item.id);
}

const compatibleModel = (a, b) => !!a.provider && !!a.kind && !!a.contract && a.provider === b.provider && a.kind === b.kind && a.contract === b.contract;

// Compatibility fallback for an older running server. Uploaded capability IDs
// are never used; only the application's catalog establishes ownership.
export function modelPackageUsage(pkg, catalog) {
  if (pkg.usage) return { ...pkg.usage, capabilities: list(pkg.usage.capabilities), requiredBy: list(pkg.usage.requiredBy) };
  const usage = { capabilities: [], requiredBy: [] };
  for (const capability of catalog.capabilities) {
    if (capability.provider !== pkg.provider || capability.state === 'unsupported') continue;
    let direct = false, dependency = false;
    const seen = new Set();
    const visit = (id, isDirect) => {
      const releases = catalog.packages.filter(item => item.id === id && item.source !== 'custom');
      if (releases.some(item => compatibleModel(pkg, item))) { if (isDirect) direct = true; else dependency = true; }
      if (seen.has(id)) return;
      seen.add(id);
      releases.forEach(item => item.dependencies.forEach(dep => visit(dep, false)));
    };
    const seenCapabilities = new Set();
    const visitCapability = (id, isDirect) => {
      if (seenCapabilities.has(id)) return;
      seenCapabilities.add(id);
      const item = catalog.capabilities.find(cap => cap.id === id && cap.provider === pkg.provider && cap.state !== 'unsupported');
      if (!item) return;
      item.packages.forEach(pid => visit(pid, isDirect));
      item.dependencies.forEach(cid => visitCapability(cid, false));
    };
    visitCapability(capability.id, true);
    if (direct) usage.capabilities.push(capability.id);
    else if (dependency) usage.requiredBy.push(capability.id);
  }
  return usage;
}

export function modelCapabilityResources(capability, catalog) {
  const result = { direct: [], dependencies: [] }, seen = new Set(), seenCapabilities = new Set();
  const visit = (id, isDirect) => {
    if (seen.has(id)) return;
    seen.add(id);
    const expected = catalog.packages.find(pkg => pkg.id === id);
    if (!expected) return;
    const selected = catalog.packages.find(pkg => pkg.active && compatibleModel(expected, pkg)) || expected;
    result[isDirect ? 'direct' : 'dependencies'].push(selected);
    selected.dependencies.forEach(dep => visit(dep, false));
  };
  const visitCapability = (item, isDirect) => {
    if (seenCapabilities.has(item.id)) return;
    seenCapabilities.add(item.id);
    item.packages.forEach(id => visit(id, isDirect));
    item.dependencies.forEach(id => { const dep = catalog.capabilities.find(cap => cap.id === id); if (dep) visitCapability(dep, false); });
  };
  visitCapability(capability, true);
  return result;
}

export const newerModelVersion = (a, b) => {
  const aa = a.replace(/^v/, '').split('.'), bb = b.replace(/^v/, '').split('.');
  for (let i = 0; i < Math.min(aa.length, bb.length); i++) {
    if (/^\d+$/.test(aa[i]) && /^\d+$/.test(bb[i]) && Number(aa[i]) !== Number(bb[i])) return Number(aa[i]) > Number(bb[i]);
    if (aa[i] !== bb[i]) return aa[i] > bb[i];
  }
  return a > b;
};

const terminalStates = new Set(['succeeded', 'failed', 'cancelled', 'interrupted']);
export function normalizeModelJob(row = {}) {
  const job = row.job || row;
  const state = ({ completed: 'succeeded', canceled: 'cancelled' })[job.state] || job.state || 'queued';
  const completed = finite(job.completedBytes ?? job.completed_bytes);
  const total = finite(job.totalBytes ?? job.total_bytes);
  // `activated` / `pending` are the offline-import outcome reported by the
  // installer: which packages took over their resource kind, and which ones
  // installed but could not (missing dependency or a kind still in use).
  const activated = Array.isArray(job.activated) ? job.activated.map(entry => String(entry)) : [];
  const pending = Array.isArray(job.pending)
    ? job.pending.map(entry => ({
      packageId: String(entry?.packageId ?? entry?.package_id ?? ''),
      version: String(entry?.version ?? ''),
      kind: String(entry?.kind ?? ''),
      reason: String(entry?.reason ?? entry?.reasonCode ?? entry?.reason_code ?? ''),
    })).filter(entry => entry.packageId)
    : [];
  return { ...job, id: job.id || job.job_id || '', state, packageId: job.packageId || job.package_id || '', completedBytes: completed, totalBytes: total, percent: state === 'succeeded' ? 100 : total > 0 ? Math.min(100, Math.floor(completed / total * 100)) : 0, errorCode: job.errorCode || job.reason_code || job.error_code || '', activated, pending, terminal: terminalStates.has(state), retryable: ['failed', 'cancelled', 'interrupted'].includes(state) };
}

export function modelErrorText(error, locale, fallback = 'operationError') {
  const copy = modelCenterCopy(locale);
  if (error?.response?.status === 413) return copy.reasons.archive_too_large;
  const data = error?.response?.data || error || {};
  const rawCode = data.reason_code || data.reasonCode || data.errorCode || (typeof data.code === 'string' ? data.code : '');
  const code = reasonAliases[rawCode] || rawCode;
  if (copy.reasons[code]) return copy.reasons[code];
  if (error?.response?.status === 403) return copy.reasons.forbidden;
  if (error?.response?.status === 401) return copy.reasons.unauthorized;
  if (error?.code === 'ERR_NETWORK' || error?.code === 'ECONNABORTED') return copy.networkError;
  return copy[fallback] || copy.operationError;
}

export function modelStateText(state, locale) {
  const copy = modelCenterCopy(locale);
  return copy.states[state] || copy.states.unknown;
}

// ── 2026-10-12 重构新增：面向使用者的状态归纳与行动文案 ──────────────
// 设计依据 §4.1 / §4.2。仅改前端措辞，不改后端状态枚举。
/** 页面级状态分组：能力状态 → 摘要条分组。 */
export function modelStatusGroups() {
  return ['available', 'pending', 'needsInstall', 'unsupported'];
}

/**
 * 能力状态 → 摘要条分组 id。
 * `candidate` 表示所需模型都已装到本机、只是还没接管资源类型（离线升级时
 * 旧版本的会话仍在跑，或资源被占用），因此单列一组「已装待启用」。
 */
export function modelCapabilityGroup(state) {
  if (['ready', 'installed', 'loaded'].includes(state)) return 'available';
  if (['unsupported', 'runtime_missing'].includes(state)) return 'unsupported';
  if (state === 'candidate') return 'pending';
  return 'needsInstall';
}

/** 能力状态 → 摘要条展示文案（可用 / 已装待启用 / 待安装 / 当前程序不支持）。 */
export function modelCapabilityStatusText(capability, locale) {
  const copy = modelCenterCopy(locale);
  const key = { available: 'statusAvailable', pending: 'statusPending', needsInstall: 'statusNeedsInstall', unsupported: 'statusUnsupported' }[modelCapabilityGroup(capability.state)];
  return copy[key];
}

/**
 * 能力卡「唯一行动障碍」文案（§4.2 第 ② 段）。
 * 返回 { text, tone, action }：
 *   tone   — 'ready' | 'warning' | 'muted'
 *   action — 'install' | 'offline' | 'activate' | 'none'
 */
export function modelCapabilityActionText(capability, catalog, locale) {
  const copy = modelCenterCopy(locale);
  const state = capability.state;
  if (['ready', 'installed', 'loaded'].includes(state)) return { text: copy.cardReady, tone: 'ready', action: 'none' };
  if (state === 'unsupported') return { text: copy.cardUnsupported, tone: 'muted', action: 'none' };
  if (state === 'runtime_missing') return { text: copy.cardRuntimeMissing, tone: 'muted', action: 'none' };
  if (state === 'offline_only') return { text: copy.cardNeedsOffline, tone: 'warning', action: 'offline' };
  if (state === 'candidate') return { text: copy.cardPendingActivation, tone: 'warning', action: 'activate' };
  if (capability.reasonCode === 'model_class_missing') return { text: copy.cardClassMissing, tone: 'warning', action: 'none' };
  // missing / incomplete：模型只能通过离线包获得，因此不再提供在线安装动作。
  //
  // 内置包里 general / smoking / chef / hairnet / kitchen 都是 offlineOnly（清单里没有
  // 下载地址），而几乎所有能力都依赖 general，于是「在线计划」在本产品中必然 blocked
  // ——原来那个「安装所需模型」按钮是一次注定失败的死路。这里统一改为引导导入离线包。
  const pending = modelPendingPackages(capability, catalog);
  const bytes = pending.reduce((sum, pkg) => sum + finite(pkg.downloadBytes ?? pkg.bytes), 0);
  return { text: copy.cardNeedsPackages.replace('{count}', pending.length).replace('{size}', modelBytes(bytes)), tone: 'warning', action: 'offline' };
}

/** 能力尚未安装、且本次安装将新增的模型包（去重）。 */
export function modelPendingPackages(capability, catalog) {
  const seen = new Set(), pending = [];
  const visit = (id) => {
    if (seen.has(id)) return;
    seen.add(id);
    const releases = catalog.packages.filter(pkg => pkg.id === id && pkg.source !== 'custom');
    if (!releases.length) return;
    const active = releases.find(pkg => pkg.active);
    if (active) return;
    const target = releases.reduce((latest, pkg) => !latest || newerModelVersion(pkg.version, latest.version) ? pkg : latest, null);
    pending.push(target);
    target.dependencies.forEach(visit);
  };
  capability.packages.forEach(visit);
  return pending;
}

// ── P1 模型库 / Model library ──────────────────────────────────────
// 后端 Catalog() 对每个包 ID 的每个版本都产出一条 PackageStatus，
// 因此版本历史可由前端按 id 分组直接得出，无需后端改动。

const installedPackageStates = new Set(['active', 'candidate']);
const pendingPackageStates = new Set(['missing', 'offline_only']);

/** 包级状态 → 语义色分组。 */
export function modelPackageGroup(state) {
  if (['active', 'candidate'].includes(state)) return 'available';
  if (['unsupported', 'runtime_missing'].includes(state)) return 'unsupported';
  return 'needsInstall';
}

/** 包状态 → 使用者文案。active 优先展示「使用中」。 */
export function modelPackageStatusText(pkg, locale) {
  const copy = modelCenterCopy(locale);
  if (pkg.state === 'legacy') return copy.legacyTitle;
  if (pkg.state === 'active') return copy.libActive;
  if (pkg.state === 'candidate') return copy.installedReleases;
  return modelStateText(pkg.state, locale);
}

/**
 * 按包 ID 聚合出「一行一个模型包」的表格数据。
 * 返回 [{ id, title, kind, activeRelease, releases, installedCount, state, usage }]
 */
export function modelPackageRows(catalog) {
  const byId = new Map();
  for (const pkg of catalog.packages) {
    if (!byId.has(pkg.id)) byId.set(pkg.id, []);
    byId.get(pkg.id).push(pkg);
  }
  return [...byId.entries()].map(([id, releases]) => {
    // 同版本多来源时优先内置；确保 id@version 唯一
    const unique = new Map();
    for (const pkg of releases) {
      const key = pkg.version;
      if (!unique.has(key) || (unique.get(key).source === 'custom' && pkg.source !== 'custom')) unique.set(key, pkg);
    }
    const versions = [...unique.values()].sort((a, b) => newerModelVersion(a.version, b.version) ? -1 : 1);
    const activeRelease = versions.find(pkg => pkg.active) || null;
    const representative = activeRelease || versions.find(pkg => pkg.installed) || versions[0];
    const installedCount = versions.filter(pkg => installedPackageStates.has(pkg.state)).length;
    const state = activeRelease ? 'active'
      : installedCount > 0 ? 'candidate'
      : versions.every(pkg => pkg.state === 'legacy') ? 'legacy'
      : versions.every(pkg => pendingPackageStates.has(pkg.state)) ? 'missing'
      : representative.state;
    return {
      id,
      title: representative.title,
      kind: representative.kind,
      provider: representative.provider,
      releases: versions,
      activeRelease,
      installedCount,
      state,
      inUse: versions.some(pkg => pkg.inUse),
      usage: representative.usage || modelPackageUsage(representative, catalog),
      representative,
    };
  }).sort((a, b) => String(a.id).localeCompare(String(b.id)));
}

/** Only scenarios implemented by this build belong in the scenario view. */
export function modelSystemCapabilities(catalog) {
  const unsupported = new Set(['unsupported', 'unsupported_build', 'algorithm_not_built', 'algorithm_not_supported', 'not_implemented']);
  return catalog.capabilities.filter(item => {
    if (unsupported.has(item.reasonCode) || unsupported.has(item.unsupportedReason)) return false;
    return item.state !== 'unsupported' || item.reasonCode === 'model_class_missing';
  });
}

export function modelSceneAvailable(capability) {
  return ['ready', 'loaded', 'installed', 'legacy'].includes(capability.state);
}

/** Unmanaged files are installed, but have no verifiable release version. */
export function modelLibraryRows(catalog) {
  return modelPackageRows(catalog).map(row => {
    const installedReleases = row.releases.filter(pkg => pkg.installed || ['active', 'candidate', 'installed'].includes(pkg.state));
    const legacy = row.releases.some(pkg => pkg.state === 'legacy');
    const current = row.activeRelease;
    const installed = installedReleases.length > 0 || legacy;
    return { ...row, installed, installedVersions: installedReleases.map(pkg => pkg.version), version: current?.version || '', state: installed ? 'installed' : 'missing' };
  });
}

/** 模型包服务的 AI 能力（直接支持 / 作为依赖 / 可选增强）。 */
export function modelPackageCapabilities(pkg, catalog) {
  const usage = pkg.usage || modelPackageUsage(pkg, catalog);
  return {
    direct: list(usage.capabilities),
    dependency: list(usage.requiredBy),
    optional: modelOptionalCapabilities(pkg.representative || pkg, catalog),
  };
}

/** 模型包服务的 AI 能力总数（去重）。 */
export function modelPackageCapabilityCount(pkg, catalog) {
  const caps = modelPackageCapabilities(pkg, catalog);
  return new Set([...caps.direct, ...caps.dependency, ...caps.optional]).size;
}

/**
 * 包级动作是否可发起。
 * 注意两处与占用（inUse）相关、但方向不同的判定：
 *   verify   — 只读校验，不改变激活状态，占用中也允许；
 *   activate — 允许占用中发起：运行中的会话继续用旧版本，最后一个租约释放后才
 *              装载新版本（yolo_pro 的指纹切换），因此不需要等占用解除；
 *   uninstall— 占用中必须阻止，否则会在推理中被抽掉文件。
 */
export function modelPackageActionAllowed(pkg, action) {
  if (!pkg.installed) return false;
  if (action === 'verify') return true;
  if (action === 'activate') return !pkg.active;
  if (pkg.inUse) return false;
  if (action === 'uninstall') return pkg.references.length === 0;
  return false;
}

export function modelBytes(value) {
  const bytes = finite(value);
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 ** 2) return `${(bytes / 1024).toFixed(1)} KiB`;
  if (bytes < 1024 ** 3) return `${(bytes / 1024 ** 2).toFixed(1)} MiB`;
  return `${(bytes / 1024 ** 3).toFixed(1)} GiB`;
}

/**
 * 导入计划的逐包分类，供「程序识别出的内容」表格使用。
 * 后端 `usage.effect` 是权威（计划创建时的预测），本地 catalog 只在 effect 缺失时兜底。
 * `reuse` = 目标上已有可用版本，安装时跳过；`replace` = 会成为候选、需管理员显式切换。
 */
export function modelPlanRows(plan, catalog) {
  const existing = new Map(catalog.packages.map(pkg => [`${pkg.id}@${pkg.version}`, pkg]));
  return list(plan?.packages).map(pkg => {
    const current = existing.get(`${pkg.id}@${pkg.version}`);
    // usage 同时挂在顶层与包上；包上优先，避免依赖 normalizeModelPlan 的中间层。
    const usage = pkg.usage || list(plan?.usage).find(row => row.packageId === pkg.id && row.version === pkg.version) || {};
    const effect = usage.effect || '';
    const previous = catalog.packages.find(item => item.active && compatibleModel(item, pkg));
    const kind = effect === 'reuse_active' ? 'reuse'
      : effect === 'activate_if_missing' ? (previous || catalog.packages.some(item => item.state === 'legacy' && item.kind === pkg.kind) ? 'replace' : 'new')
      : effect === 'candidate' ? 'replace'
      : current?.active ? 'reuse'
      : current?.installed ? 'replace'
      : 'new';
    return { pkg, current, previous, effect, kind };
  });
}

/** 导入计划缺失的内置依赖包 ID（离线包不含其固定版本时返回，供工具栏提示）。 */
export function modelPlanMissingDependencies(plan, catalog) {
  const inside = new Set(list(plan?.packages).map(pkg => pkg.id));
  for (const pkg of list(catalog?.packages)) {
    if (pkg.installed || ['active', 'legacy'].includes(pkg.state)) inside.add(pkg.id);
  }
  return [...new Set(list(plan?.packages).flatMap(pkg =>
    (pkg.source === 'custom' ? [] : list(pkg.dependencies)).filter(id => !inside.has(id))
  ))];
}

/** 计划里的能力 → 双语名称列表。 */
export function modelPlanCapabilityNames(plan, catalog, locale) {
  return [...new Set(list(plan?.capabilities))].map(id => {
    const capability = catalog.capabilities.find(item => item.id === id);
    return capability ? modelText(capability.title, locale, id) : id;
  });
}

// ── 2026-10-12 离线导入单动作流程 / One-click offline import ────────────
// 一次点击贯通「上传 → 解析 → 安装 → 启用」。阶段与总进度由本组函数推导，
// 组件只负责渲染，保证进度语义与 Job 状态机一一对应。

/** 四阶段顺序（与 UI 步骤条一致）。 */
export const modelImportPhases = ['upload', 'parse', 'install', 'activate'];

/**
 * 导入流程阶段与总进度。
 * 入参：
 *   uploading / uploadPercent — 浏览器上传进度（0-100）
 *   parsed                    — 解析是否已返回计划
 *   job                       — normalizeModelJob 归一化后的安装任务（可为 null）
 * 返回 { phase, percent, indeterminate }：
 *   phase         — 'upload' | 'parse' | 'install' | 'activate' | 'done'
 *   indeterminate — 该阶段无确定进度，UI 应显示活动条而非百分比
 * 权重：上传 0-30、解析 30-45、安装 45-90、启用 90-100。
 */
export function modelImportProgress({ uploading = false, uploadPercent = 0, parsed = false, job = null } = {}) {
  const pct = Math.max(0, Math.min(100, finite(uploadPercent)));
  if (!parsed && !job) {
    if (uploading) return { phase: 'upload', percent: Math.round(pct * 0.3), indeterminate: false };
    return { phase: 'parse', percent: 30, indeterminate: true };
  }
  if (!job) return { phase: 'parse', percent: 30, indeterminate: true };
  if (job.state === 'succeeded') return { phase: 'done', percent: 100, indeterminate: false };
  // 自行判定终态：调用方可能传入未经过 normalizeModelJob 的原始任务。
  if (terminalStates.has(job.state)) return { phase: 'install', percent: 45 + Math.round(finite(job.percent) * 0.45), indeterminate: false };
  if (job.state === 'publishing') return { phase: 'activate', percent: 90, indeterminate: true };
  // queued / downloading / verifying 均属于「安装到本机」
  if (job.state === 'queued') return { phase: 'install', percent: 45, indeterminate: true };
  return { phase: 'install', percent: 45 + Math.round(finite(job.percent) * 0.45), indeterminate: false };
}

/**
 * 表格中单个包在导入过程中的行状态。
 * 返回：wait | installing | activating | activated | pending | installed | failed。
 * 一个 Job 覆盖整个计划，`job.packageId` 指明当前正在处理的包，其余包按此推导。
 */
export function modelImportRowState(pkg, job) {
  if (!pkg || !job) return 'wait';
  // 终态自行判定，避免依赖 normalizeModelJob 注入的 `terminal` 字段。
  if (terminalStates.has(job.state)) {
    if (job.state !== 'succeeded') return 'failed';
    if (list(job.activated).includes(pkg.id)) return 'activated';
    if (list(job.pending).some(entry => entry.packageId === pkg.id)) return 'pending';
    return 'installed';
  }
  if (job.state === 'publishing') return 'activating';
  return job.packageId === pkg.id ? 'installing' : 'wait';
}

/** 待启用原因 → 双语说明文案。 */
export function modelPendingReasonText(reason, locale) {
  const copy = modelCenterCopy(locale);
  return { in_use: copy.importPendingInUse, missing_dependency: copy.importPendingDependency }[reason] || copy.importPendingHint;
}

/**
 * 能力「已装待启用」时，需要被接管的具体包 → [{ packageId, version }]。
 * 后端只给出包 ID（`capability.unactivated`），版本取本机已安装里最新的一个。
 */
export function modelUnactivatedPackages(capability, catalog) {
  return list(capability?.unactivated).flatMap(id => {
    const installed = list(catalog?.packages).filter(pkg => pkg.id === id && pkg.installed);
    if (!installed.length) return [];
    const best = installed.reduce((latest, pkg) => (!latest || newerModelVersion(pkg.version, latest.version) ? pkg : latest), null);
    return [{ packageId: id, version: best.version }];
  });
}

/**
 * 该版本相对当前活动版本是「回退」还是「切换」。
 * activeVersion 更新时返回 true（回退），否则为向前切换。
 */
export function modelIsRollback(release, activeVersion) {
  if (!release || !activeVersion) return false;
  return newerModelVersion(activeVersion, release.version);
}
