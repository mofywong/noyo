export function gatewayMaintenanceClass(mode) {
  return ['maintenance', 'reconciling'].includes(mode) ? `gateway-state--${mode}` : '';
}

export function gatewayMaintenanceLabel(mode, locale) {
  const labels = String(locale).toLowerCase().startsWith('zh')
    ? { maintenance: '本地维护中', reconciling: '正在交接' }
    : { maintenance: 'Local maintenance', reconciling: 'Reconciling' };
  return labels[mode] || '';
}

export function gatewayState(mode, online, enabled) {
  if (online === false) return 'offline';
  if (enabled === false) return 'disabled';
  if (['maintenance', 'reconciling'].includes(mode)) return mode;
  return online === true ? 'online' : '';
}

export function gatewayStateClass(mode, online, enabled) {
  const state = gatewayState(mode, online, enabled);
  return state ? `gateway-state--${state}` : '';
}

export function gatewayStateLabel(mode, online, enabled, locale) {
  const labels = String(locale).toLowerCase().startsWith('zh')
    ? { online: '在线', offline: '离线', disabled: '已禁用', maintenance: '本地维护中', reconciling: '正在交接' }
    : { online: 'Online', offline: 'Offline', disabled: 'Disabled', maintenance: 'Local maintenance', reconciling: 'Reconciling' };
  return labels[gatewayState(mode, online, enabled)] || '';
}

export function updateParentGatewayOnline(devices, gatewayCode, online) {
  for (const device of devices) {
    if (device?.parent_code === gatewayCode && typeof device.gateway_online === 'boolean') {
      device.gateway_online = online;
    }
  }
}
