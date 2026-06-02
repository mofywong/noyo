export const QUICK_MODE_MODULES = [
  {
    module: 'dashboard',
    group: 'business',
    icon: 'bi-speedometer2',
    nameKey: 'perm_mod_dashboard',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['dashboard:view'],
    edit: ['dashboard:edit'],
    full: ['dashboard:delete']
  },
  {
    module: 'plugin',
    group: 'business',
    icon: 'bi-shop',
    nameKey: 'perm_mod_plugin',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['plugin:list'],
    edit: ['plugin:config'],
    full: ['plugin:delete']
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
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['gateway:list'],
    edit: ['gateway:config'],
    full: ['gateway:delete']
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
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['device:topology'],
    edit: ['topology:edit'],
    full: ['topology:delete']
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
    module: 'alarm',
    group: 'business',
    icon: 'bi-bell-fill',
    nameKey: 'perm_mod_alarm',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['alarm:list'],
    edit: ['alarm:handle'],
    full: ['alarm:delete']
  },
  {
    module: 'audit',
    group: 'business',
    icon: 'bi-journal-text',
    nameKey: 'perm_mod_audit',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['audit:list'],
    edit: ['audit:edit'],
    full: ['audit:delete']
  },
  {
    module: 'system_logs',
    group: 'business',
    icon: 'bi-journal-code',
    nameKey: 'system_logs',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['system:logs'],
    edit: ['system:logs_edit'],
    full: ['system:logs_delete']
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
    full: ['tenant:delete']
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
    module: 'position',
    group: 'business',
    icon: 'bi-person-badge',
    nameKey: 'perm_mod_position',
    levels: ['none', 'readonly', 'edit', 'full'],
    readonly: ['position:list'],
    edit: ['position:create', 'position:edit'],
    full: ['position:delete']
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
    full: ['history:delete']
  }
];

export function inferQuickModeLevel(modConfig, selectedCodes, currentLevel) {
  const { readonly, edit, full } = modConfig;
  
  let allModCodes = [...readonly, ...edit, ...full];
  let selectedModCodes = selectedCodes.filter(c => allModCodes.includes(c));
  
  if (selectedModCodes.length === 0) return 'none';
  
  if (currentLevel && currentLevel !== 'custom' && currentLevel !== 'none') {
    const expectedCodes = getCodesForLevel(modConfig, currentLevel);
    const isExactMatch = selectedModCodes.length === expectedCodes.length && 
                         expectedCodes.every(c => selectedModCodes.includes(c));
    if (isExactMatch) {
      return currentLevel;
    }
  }
  
  const hasAllRead = readonly.length > 0 && readonly.every(c => selectedCodes.includes(c));
  const hasAllEdit = edit.length > 0 && edit.every(c => selectedCodes.includes(c));
  const hasAllFull = full.length > 0 && full.every(c => selectedCodes.includes(c));
  
  if (modConfig.module === 'history') {
    if (full.length > 0 && hasAllFull && selectedModCodes.length === full.length) return 'full';
    return 'custom';
  }
  
  if (readonly.length > 0 && hasAllRead) {
    if (edit.length > 0 && hasAllEdit) {
      if (full.length > 0 && hasAllFull && selectedModCodes.length === readonly.length + edit.length + full.length) {
        return 'full';
      }
      if (selectedModCodes.length === readonly.length + edit.length && (!full.length || !hasAllFull)) {
        return 'edit';
      }
    }
    if (selectedModCodes.length === readonly.length && (!edit.length || !hasAllEdit)) {
      return 'readonly';
    }
  }
  
  return 'custom';
}

export function getCodesForLevel(modConfig, level) {
  if (level === 'none' || level === 'custom') return [];
  if (level === 'readonly') return [...modConfig.readonly];
  if (level === 'edit') return [...modConfig.readonly, ...modConfig.edit];
  if (level === 'full') return [...modConfig.readonly, ...modConfig.edit, ...modConfig.full];
  return [];
}
