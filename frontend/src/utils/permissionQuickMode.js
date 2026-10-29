export const QUICK_MODE_MODULES = [
  {
    module: 'dashboard',
    group: 'business',
    icon: 'bi-speedometer2',
    nameKey: 'perm_mod_dashboard',
    levels: ['none', 'readonly'],
    readonly: ['dashboard:view'],
    edit: [],
    full: []
  },
  {
    module: 'plugin',
    group: 'business',
    icon: 'bi-shop',
    nameKey: 'perm_mod_plugin',
    levels: ['none', 'readonly', 'edit'],
    readonly: ['plugin:list'],
    edit: ['plugin:config'],
    full: []
  },
  {
    module: 'product',
    group: 'business',
    icon: 'bi-box-seam',
    nameKey: 'perm_mod_product',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['product:list'],
    edit: ['product:create', 'product:edit'],
    full: ['product:delete']
  },
  {
    module: 'gateway',
    group: 'business',
    icon: 'bi-hdd-network',
    nameKey: 'perm_mod_gateway',
    levels: ['none', 'readonly', 'edit'],
    readonly: ['gateway:list'],
    edit: ['gateway:config'],
    full: []
  },
  {
    module: 'device',
    group: 'business',
    icon: 'bi-cpu',
    nameKey: 'perm_mod_device',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['device:list'],
    edit: ['device:create', 'device:edit', 'device:control', 'device:upload'],
    full: ['device:delete']
  },
  {
    module: 'topology',
    group: 'business',
    icon: 'bi-diagram-2',
    nameKey: 'perm_mod_topology',
    levels: ['none', 'readonly'],
    readonly: ['device:topology'],
    edit: [],
    full: []
  },
  {
    module: 'device_map', group: 'business', icon: 'bi-geo-alt', nameKey: 'perm_mod_device_map',
    levels: ['none', 'readonly', 'edit'],
    readonly: ['device_map:view'], edit: ['device_location:edit'], full: []
  },
  {
    module: 'geofence', group: 'business', icon: 'bi-bounding-box', nameKey: 'perm_mod_geofence',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['geofence:list'], edit: ['geofence:create', 'geofence:edit'], full: ['geofence:delete']
  },
  {
    module: 'device_tag',
    group: 'business',
    icon: 'bi-tags',
    nameKey: 'perm_mod_device_tag',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['device_tag:list'],
    edit: ['device_tag:create', 'device_tag:edit'],
    full: ['device_tag:delete']
  },
  {
    module: 'rule',
    group: 'business',
    icon: 'bi-diagram-3',
    nameKey: 'perm_mod_rule',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['rule:list', 'rule:detail', 'rule:log'],
    edit: ['rule:create', 'rule:edit', 'rule:enable'],
    full: ['rule:delete', 'rule_group:manage']
  },
  {
    module: 'work_order',
    group: 'business',
    icon: 'bi-ticket-detailed',
    nameKey: 'perm_mod_work_order',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['work_order:list'],
    edit: ['work_order:create', 'work_order:process'],
    full: ['work_order:approve', 'work_order:manage', 'work_order:integration']
  },
  {
    module: 'ai_scene',
    group: 'business',
    icon: 'bi-bounding-box-circles',
    nameKey: 'perm_mod_ai_scene',
    levels: ['none', 'readonly', 'edit'],
    readonly: ['ai_scene:list'],
    edit: ['ai_scene:edit'],
    full: []
  },
  {
    module: 'inspection', group: 'business', icon: 'bi-camera-video', nameKey: 'perm_mod_inspection',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['inspection:list'],
    edit: ['inspection:execute', 'inspection:order', 'inspection:export', 'inspection:review'],
    full: ['inspection:manage', 'inspection:archive']
  },
  {
    module: 'semantic_review',
    group: 'business',
    icon: 'bi-journal-text',
    nameKey: 'perm_mod_semantic_review',
    levels: ['none', 'readonly'],
    dependencies: ['ai_scene:list'],
    readonly: ['semantic_review:list'],
    edit: [],
    full: []
  },
  {
    module: 'ai_model',
    group: 'business',
    icon: 'bi-box-arrow-in-down',
    nameKey: 'perm_mod_ai_model',
    levels: ['none', 'readonly', 'edit'],
    readonly: ['ai_model:list'],
    edit: ['ai_model:export'],
    full: []
  },
  {
    module: 'ai_assistant',
    group: 'business',
    icon: 'bi-stars',
    nameKey: 'perm_mod_ai_assistant',
    levels: ['none', 'readonly'],
    readonly: ['ai_assistant:use'],
    edit: [],
    full: []
  },
  {
    module: 'ai_brain', group: 'business', icon: 'bi-cpu', nameKey: 'perm_mod_ai_brain',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['ai_brain:view'], edit: ['ai_brain:edit', 'ai_brain:review', 'ai_brain:suggestion'],
    full: ['ai_brain:delete', 'ai_brain:manage']
  },
  {
    module: 'screen', group: 'business', icon: 'bi-display', nameKey: 'perm_mod_screen',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['screen:list', 'screen:view'], edit: ['screen:create', 'screen:edit'],
    full: ['screen:publish', 'screen:delete']
  },
  {
    module: 'alarm',
    group: 'business',
    icon: 'bi-bell-fill',
    nameKey: 'perm_mod_alarm',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['alarm:list'],
    edit: ['alarm:handle'],
    full: ['alarm:manage', 'alarm:export']
  },
  {
    module: 'audit',
    group: 'business',
    icon: 'bi-journal-text',
    nameKey: 'perm_mod_audit',
    levels: ['none', 'readonly'],
    readonly: ['audit:list'],
    edit: [],
    full: []
  },
  {
    module: 'system_logs',
    group: 'business',
    icon: 'bi-journal-code',
    nameKey: 'system_logs',
    levels: ['none', 'readonly'],
    readonly: ['system:logs'],
    edit: [],
    full: []
  },
  {
    module: 'llm_request',
    group: 'business',
    icon: 'bi-journal-code',
    nameKey: 'llm_requests',
    levels: ['none', 'readonly'],
    readonly: ['llm_request:list'],
    edit: [],
    full: []
  },
  {
    module: 'user',
    group: 'business',
    icon: 'bi-people',
    nameKey: 'perm_mod_user',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['user:list'],
    edit: ['user:create', 'user:edit'],
    full: ['user:delete']
  },
  {
    module: 'tenant',
    group: 'business',
    icon: 'bi-building',
    nameKey: 'perm_mod_tenant',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['tenant:list'],
    edit: ['tenant:create', 'tenant:edit'],
    full: ['tenant:delete', 'tenant:transfer']
  },
  {
    module: 'history', group: 'business', icon: 'bi-clock-history', nameKey: 'perm_mod_history',
    levels: ['none', 'full'], readonly: [], edit: [], full: ['history:delete']
  },
  {
    module: 'project',
    group: 'business',
    icon: 'bi-folder',
    nameKey: 'perm_mod_project',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['project:list'],
    edit: ['project:create', 'project:edit'],
    full: ['project:delete']
  },
  {
    module: 'role',
    group: 'business',
    icon: 'bi-shield-lock',
    nameKey: 'perm_mod_role',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['role:list'],
    edit: ['role:create', 'role:edit'],
    full: ['role:delete']
  },
  {
    module: 'app',
    group: 'business',
    icon: 'bi-window-sidebar',
    nameKey: 'perm_mod_app',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['app:list'],
    edit: ['app:create', 'app:edit', 'app:reset-key'],
    full: ['app:delete']
  },
  {
    module: 'system',
    group: 'business',
    icon: 'bi-gear',
    nameKey: 'perm_mod_system',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['system:license'],
    edit: ['system:config'],
    full: ['ai_model:manage']
  }
];

export function getPermissionModule(permission) {
  if (permission.code === 'dashboard:view') return 'dashboard';
  if (permission.code === 'device:topology') return 'topology';
  if (permission.code === 'system:logs') return 'system_logs';
  return permission.module || permission.code.split(':')[0];
}

export function getAvailableQuickModeModules(allPermissions, hiddenModules = []) {
  const permissions = allPermissions || [];
  const allowedCodes = new Set(permissions.map(p => p.code));
  const knownCodes = new Set(QUICK_MODE_MODULES.flatMap(mod => [...mod.readonly, ...mod.edit, ...mod.full]));
  const modules = QUICK_MODE_MODULES.map(mod => ({
    ...mod,
    dependencies: (mod.dependencies || []).filter(code => allowedCodes.has(code)),
    readonly: mod.readonly.filter(code => allowedCodes.has(code)),
    edit: mod.edit.filter(code => allowedCodes.has(code)),
    full: mod.full.filter(code => allowedCodes.has(code))
  }));

  // New backend capabilities remain configurable without silently treating a
  // mutation as read-only. Unknown actions require explicit full control.
  for (const permission of permissions) {
    if (knownCodes.has(permission.code)) continue;
    const module = getPermissionModule(permission);
    let config = modules.find(mod => mod.module === module);
    if (!config) {
      config = { module, group: 'business', icon: 'bi-shield-lock', nameKey: `perm_mod_${module}`, readonly: [], edit: [], full: [], dependencies: [] };
      modules.push(config);
    }
    const level = /:(list|view|read|get|detail|log)$/.test(permission.code) ? 'readonly' : 'full';
    if (!config[level].includes(permission.code)) config[level].push(permission.code);
  }

  return modules
    .filter(mod => !hiddenModules.includes(mod.module))
    .map(mod => ({
      ...mod,
      // Do not show equivalent levels after the server filters permissions.
      levels: ['none', ...['readonly', 'edit', 'full'].filter(level => mod[level].length > 0)]
    }))
    .filter(mod => mod.levels.length > 1);
}

export function inferQuickModeLevel(modConfig, selectedCodes, currentLevel) {
  const allModCodes = new Set([...modConfig.readonly, ...modConfig.edit, ...modConfig.full]);
  const selected = new Set(selectedCodes);
  const selectedModCodes = [...allModCodes].filter(code => selected.has(code));
  if (selectedModCodes.length === 0) return 'none';
  const levels = modConfig.levels || ['none', 'readonly', 'edit', 'full'];
  const candidates = [currentLevel, ...levels].filter((level, index, values) =>
    level && level !== 'none' && level !== 'custom' && levels.includes(level) && values.indexOf(level) === index);
  for (const level of candidates) {
    const expectedCodes = getCodesForLevel(modConfig, level);
    const ownCodes = expectedCodes.filter(code => allModCodes.has(code));
    if (ownCodes.length === selectedModCodes.length && expectedCodes.every(code => selected.has(code))) return level;
  }
  return 'custom';
}

export function getCodesForLevel(modConfig, level) {
  if (!['readonly', 'edit', 'full'].includes(level)) return [];
  const codes = [...modConfig.readonly];
  if (level === 'edit' || level === 'full') codes.push(...modConfig.edit);
  if (level === 'full') codes.push(...modConfig.full);
  return [...new Set([...codes, ...(modConfig.dependencies || [])])];
}
