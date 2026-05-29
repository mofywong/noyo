export const isSystemAdminUser = (user) => {
  if (!user) return false
  if (user.is_system_admin === true) return true
  return user.tenant_id === 0 && ['admin', 'superadmin', 'super_admin'].includes(user.role)
}

export const isTenantAdminUser = (user) => {
  if (!user) return false
  return user.is_tenant_admin === true
}

export const isProjectAdminUser = (user) => {
  if (!user) return false
  return user.is_project_admin === true
}

export const hasUserPermission = (user, code) => {
  if (!user) return false
  return Array.isArray(user.permissions) && user.permissions.includes(code)
}

export const isInheritedRoleReadOnlyForUser = (user, role) => {
  if (!user || !role) return false
  if (isTenantAdminUser(user)) return false
  return isProjectAdminUser(user) && role.project_id === 0 && role.is_inherited === true
}
