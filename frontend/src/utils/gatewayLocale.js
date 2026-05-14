const gatewayMessages = {
  en: {
    gateway_management: 'Gateway Management',
    gateway_management_hint: 'Manage gateway-side plugins from the platform.',
    gateway: 'Gateway',
    status: 'Status',
    enabled: 'Enabled',
    disabled: 'Disabled',
    updated_at: 'Updated At',
    operation: 'Operation',
    enter: 'Enter',
    no_gateways: 'No managed gateways found.',
    sync: 'Sync',
    gateway_plugins_hint: 'Configure plugins installed on this gateway remotely.',
    remote_gateway_config: 'Remote Gateway Config',
    version: 'Version',
    pull_gateway_config: 'Pull',
    gateway_load_failed: 'Failed to load gateways',
    gateway_plugins_load_failed: 'Failed to load gateway plugins',
    gateway_plugin_config_load_failed: 'Failed to fetch plugin config',
    gateway_plugin_status_update_failed: 'Failed to update plugin status',
    gateway_plugin_status_updated: 'Plugin {action} successfully.',
    gateway_plugin_config_saved: 'Saved and applied on gateway.',
    gateway_plugin_config_save_failed: 'Failed to save config',
    gateway_plugin_marketplace_title: 'Gateway Plugin Marketplace',
    gateway_plugin_config_title: 'Gateway Plugin Configuration',
    enabled_plugins: 'Enabled Plugins',
    gateway_sync_synced: 'Synced',
    gateway_sync_pending: 'Pending sync',
    gateway_sync_syncing: 'Syncing',
    gateway_sync_conflict: 'Config conflict',
    gateway_sync_failed: 'Sync failed',
    gateway_offline_editable: 'Gateway is offline. Changes will be saved on the platform and synced when the gateway reconnects.',
    gateway_conflict_hint: 'Gateway-side config changed after this platform edit. Choose how to resolve it.',
    override_gateway_config: 'Override Gateway',
    pull_gateway_config_full: 'Pull Gateway Config',
    close: 'Close',
    language_english: 'English',
    language_chinese: '\u4e2d\u6587',
    action_enabled: 'enabled',
    action_disabled: 'disabled'
  },
  zh: {
    gateway_management: '\u7f51\u5173\u7ba1\u7406',
    gateway_management_hint: '\u5728\u5e73\u53f0\u4fa7\u7edf\u4e00\u7ba1\u7406\u7f51\u5173\u63d2\u4ef6\u3002',
    gateway: '\u7f51\u5173',
    status: '\u72b6\u6001',
    enabled: '\u5df2\u542f\u7528',
    disabled: '\u5df2\u505c\u7528',
    updated_at: '\u66f4\u65b0\u65f6\u95f4',
    operation: '\u64cd\u4f5c',
    enter: '\u8fdb\u5165',
    no_gateways: '\u6682\u672a\u627e\u5230\u53d7\u7ba1\u7f51\u5173\u3002',
    sync: '\u540c\u6b65',
    gateway_plugins_hint: '\u8fdc\u7a0b\u914d\u7f6e\u8be5\u7f51\u5173\u5df2\u5b89\u88c5\u7684\u63d2\u4ef6\u3002',
    remote_gateway_config: '\u8fdc\u7a0b\u7f51\u5173\u914d\u7f6e',
    version: '\u7248\u672c',
    pull_gateway_config: '\u62c9\u53d6',
    gateway_load_failed: '\u52a0\u8f7d\u7f51\u5173\u5217\u8868\u5931\u8d25',
    gateway_plugins_load_failed: '\u52a0\u8f7d\u7f51\u5173\u63d2\u4ef6\u5931\u8d25',
    gateway_plugin_config_load_failed: '\u52a0\u8f7d\u63d2\u4ef6\u914d\u7f6e\u5931\u8d25',
    gateway_plugin_status_update_failed: '\u66f4\u65b0\u63d2\u4ef6\u72b6\u6001\u5931\u8d25',
    gateway_plugin_status_updated: '\u63d2\u4ef6\u5df2{action}\u3002',
    gateway_plugin_config_saved: '\u914d\u7f6e\u5df2\u4fdd\u5b58\u5e76\u4e0b\u53d1\u5230\u7f51\u5173\u3002',
    gateway_plugin_config_save_failed: '\u4fdd\u5b58\u63d2\u4ef6\u914d\u7f6e\u5931\u8d25',
    gateway_plugin_marketplace_title: '\u7f51\u5173\u63d2\u4ef6\u5e02\u573a',
    gateway_plugin_config_title: '\u7f51\u5173\u63d2\u4ef6\u914d\u7f6e',
    enabled_plugins: '\u5df2\u542f\u7528\u63d2\u4ef6',
    gateway_sync_synced: '\u5df2\u540c\u6b65',
    gateway_sync_pending: '\u5f85\u540c\u6b65',
    gateway_sync_syncing: '\u540c\u6b65\u4e2d',
    gateway_sync_conflict: '\u914d\u7f6e\u51b2\u7a81',
    gateway_sync_failed: '\u540c\u6b65\u5931\u8d25',
    gateway_offline_editable: '\u7f51\u5173\u79bb\u7ebf\uff0c\u4fee\u6539\u5c06\u5148\u4fdd\u5b58\u5728\u5e73\u53f0\u4fa7\uff0c\u7f51\u5173\u6062\u590d\u8fde\u63a5\u540e\u518d\u540c\u6b65\u3002',
    gateway_conflict_hint: '\u8be5\u5e73\u53f0\u4fee\u6539\u4e4b\u540e\uff0c\u7f51\u5173\u4fa7\u914d\u7f6e\u4e5f\u53d1\u751f\u8fc7\u53d8\u66f4\uff0c\u8bf7\u9009\u62e9\u5904\u7406\u65b9\u5f0f\u3002',
    override_gateway_config: '\u8986\u76d6\u7f51\u5173',
    pull_gateway_config_full: '\u62c9\u53d6\u7f51\u5173\u914d\u7f6e',
    close: '\u5173\u95ed',
    language_english: '\u82f1\u6587',
    language_chinese: '\u4e2d\u6587',
    action_enabled: '\u542f\u7528',
    action_disabled: '\u505c\u7528'
  }
};

export function resolveGatewayLocale(locale) {
  return locale === 'zh' ? 'zh' : 'en';
}

export function gatewayText(locale, key, params = {}) {
  const lang = resolveGatewayLocale(locale);
  const template = gatewayMessages[lang][key] ?? gatewayMessages.en[key] ?? key;

  return template.replace(/\{(\w+)\}/g, (_, name) => {
    return params[name] ?? `{${name}}`;
  });
}

export function gatewayActionText(locale, enabled) {
  return gatewayText(locale, enabled ? 'action_enabled' : 'action_disabled');
}

export function gatewayDateTime(locale, value) {
  if (!value) {
    return '-';
  }

  const lang = resolveGatewayLocale(locale);
  const normalizedLocale = lang === 'zh' ? 'zh-CN' : 'en-US';

  return new Date(value).toLocaleString(normalizedLocale);
}

export { gatewayMessages };
