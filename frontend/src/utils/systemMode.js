export const SYSTEM_MODES = {
  MULTI_TENANT_PLATFORM: 'multi_tenant_platform',
  MULTI_PROJECT_PLATFORM: 'multi_project_platform',
  PLATFORM_GATEWAY: 'platform_gateway',
  LOCAL_PROJECT: 'local_project',
}

const legacyModeMap = {
  platform: SYSTEM_MODES.MULTI_TENANT_PLATFORM,
  gateway_managed: SYSTEM_MODES.PLATFORM_GATEWAY,
  gateway_standalone: SYSTEM_MODES.LOCAL_PROJECT,
}

export function normalizeSystemMode(mode) {
  const value = String(mode || '').trim()
  return legacyModeMap[value] || value || SYSTEM_MODES.MULTI_TENANT_PLATFORM
}

export function isSingleProjectMode(mode) {
  const value = normalizeSystemMode(mode)
  return value === SYSTEM_MODES.PLATFORM_GATEWAY || value === SYSTEM_MODES.LOCAL_PROJECT
}

export function hidesTenantManagement(mode) {
  const value = normalizeSystemMode(mode)
  return value === SYSTEM_MODES.MULTI_PROJECT_PLATFORM || isSingleProjectMode(value)
}

export function systemModeLabel(mode) {
  const value = normalizeSystemMode(mode)
  return {
    [SYSTEM_MODES.MULTI_TENANT_PLATFORM]: '多租户运营平台',
    [SYSTEM_MODES.MULTI_PROJECT_PLATFORM]: '多项目管理平台',
    [SYSTEM_MODES.PLATFORM_GATEWAY]: '平台接入网关',
    [SYSTEM_MODES.LOCAL_PROJECT]: '本地单项目管理',
  }[value] || value
}
