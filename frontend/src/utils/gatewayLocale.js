import { formatDateTime } from './dateTime.js';

const gatewayMessages = {
  en: {
    gateway_management: 'Gateway Management',
    gateway_management_hint: 'Manage gateway-side plugins from the platform.',
    add_gateway: 'Add Edge Gateway',
    register_gateway: 'Register Gateway',
    gateway_display_name: 'Gateway Display Name',
    gateway_physical_sn: 'Gateway Physical SN',
    gateway_physical_sn_hint: 'Use the physical SN printed on the edge gateway.',
    gateway_physical_sn_required: 'Gateway physical SN is required.',
    platform_tenant_id: 'Platform Tenant ID',
    platform_project_id: 'Platform Project ID',
    gateway_registration_created: 'Gateway pre-registration is complete.',
    gateway_registration_hint: 'Enter these tenant and project IDs together with this physical SN in the edge gateway setup wizard.',
    gateway_created: 'Gateway pre-registered successfully.',
    gateway_create_failed: 'Failed to pre-register gateway.',
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
    add_gateway: '\u6dfb\u52a0\u8fb9\u7f18\u7f51\u5173',
    register_gateway: '\u767b\u8bb0\u7f51\u5173',
    gateway_display_name: '\u7f51\u5173\u663e\u793a\u540d\u79f0',
    gateway_physical_sn: '\u7f51\u5173\u7269\u7406 SN',
    gateway_physical_sn_hint: '\u8bf7\u586b\u5199\u8fb9\u7f18\u7f51\u5173\u8bbe\u5907\u5f20\u8d34\u6216\u786c\u4ef6\u6807\u6ce8\u4e0a\u7684\u7269\u7406 SN\u3002',
    gateway_physical_sn_required: '\u7f51\u5173\u7269\u7406 SN \u4e0d\u80fd\u4e3a\u7a7a\u3002',
    platform_tenant_id: '\u5e73\u53f0\u79df\u6237 ID',
    platform_project_id: '\u5e73\u53f0\u9879\u76ee ID',
    gateway_registration_created: '\u7f51\u5173\u9884\u767b\u8bb0\u5df2\u5b8c\u6210\u3002',
    gateway_registration_hint: '\u8bf7\u5728\u8fb9\u7f18\u7f51\u5173\u521d\u59cb\u5316\u5411\u5bfc\u4e2d\u540c\u65f6\u586b\u5199\u8fd9\u4e9b\u79df\u6237\u3001\u9879\u76ee ID \u548c\u8be5\u7269\u7406 SN\u3002',
    gateway_created: '\u7f51\u5173\u9884\u767b\u8bb0\u6210\u529f\u3002',
    gateway_create_failed: '\u7f51\u5173\u9884\u767b\u8bb0\u5931\u8d25\u3002',
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
  return formatDateTime(value);
}

export { gatewayMessages };
