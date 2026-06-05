const normalizeLocale = (locale) => (String(locale || 'en').toLowerCase().startsWith('zh') ? 'zh' : 'en')

const exactMessages = {
  'read-only access to this device due to tag restrictions': {
    en: 'Read-only access to this device due to tag restrictions',
    zh: '由于设备标签权限限制，您只有该设备的只读权限'
  },
  'Access denied': {
    en: 'Access denied',
    zh: '访问被拒绝'
  },
  'Access denied to this project': {
    en: 'Access denied to this project',
    zh: '无权访问该项目'
  },
  'Invalid JSON': {
    en: 'Invalid JSON',
    zh: '请求数据格式无效'
  },
  'Invalid JSON payload': {
    en: 'Invalid JSON payload',
    zh: '请求数据格式无效'
  },
  'Invalid App Credentials': {
    en: 'Invalid App credentials',
    zh: '应用凭证无效'
  },
  'Invalid App credentials': {
    en: 'Invalid App credentials',
    zh: '应用凭证无效'
  },
  'AppID and AppKey are required': {
    en: 'AppID and AppKey are required',
    zh: 'AppID 和 AppKey 必填'
  },
  'Missing Authorization header or App Credentials': {
    en: 'Missing Authorization header or App credentials',
    zh: '缺少登录令牌或应用凭证'
  },
  'Missing Authorization header': {
    en: 'Missing Authorization header',
    zh: '缺少 Authorization 请求头'
  },
  'Invalid Authorization header format': {
    en: 'Invalid Authorization header format',
    zh: '认证请求头格式无效'
  },
  'Invalid or expired token': {
    en: 'Invalid or expired token',
    zh: '登录令牌无效或已过期'
  },
  'Token has been revoked': {
    en: 'Token has been revoked',
    zh: '登录令牌已失效'
  },
  'Unauthorized': {
    en: 'Unauthorized',
    zh: '未登录或登录已失效'
  },
  'Password change required': {
    en: 'Password change required',
    zh: '需要修改密码'
  },
  'Invalid username or password': {
    en: 'Invalid username or password',
    zh: '用户名或密码错误'
  },
  'Invalid or expired refresh token': {
    en: 'Invalid or expired refresh token',
    zh: '刷新令牌无效或已过期'
  },
  'Invalid or expired app refresh token': {
    en: 'Invalid or expired app refresh token',
    zh: '应用刷新令牌无效或已过期'
  },
  'Refresh token cannot be used for API access': {
    en: 'Refresh token cannot be used for API access',
    zh: '刷新令牌不能用于调用业务 API'
  },
  'invalid app access token': {
    en: 'Invalid app access token',
    zh: '应用访问令牌无效'
  },
  'App no longer active': {
    en: 'App no longer active',
    zh: '应用已停用或不存在'
  },
  'Failed to generate app tokens': {
    en: 'Failed to generate app tokens',
    zh: '生成应用令牌失败'
  },
  'User no longer active': {
    en: 'User no longer active',
    zh: '用户已停用'
  },
  'User not found': {
    en: 'User not found',
    zh: '用户不存在'
  },
  'Role not found': {
    en: 'Role not found',
    zh: '角色不存在'
  },
  'Project not found': {
    en: 'Project not found',
    zh: '项目不存在'
  },
  'Tenant not found': {
    en: 'Tenant not found',
    zh: '租户不存在'
  },
  'App not found': {
    en: 'App not found',
    zh: '应用不存在'
  },
  'Device not found': {
    en: 'Device not found',
    zh: '设备不存在'
  },
  'Product not found': {
    en: 'Product not found',
    zh: '产品不存在'
  },
  'Plugin not found': {
    en: 'Plugin not found',
    zh: '插件不存在'
  },
  'Product is outside current project': {
    en: 'Product is outside current project',
    zh: '产品不属于当前项目'
  },
  'Parent device is outside current project': {
    en: 'Parent device is outside current project',
    zh: '父设备不属于当前项目'
  },
  'Parent product is outside current project': {
    en: 'Parent product is outside current project',
    zh: '父设备产品不属于当前项目'
  },
  'Project context is required': {
    en: 'Project context is required',
    zh: '需要选择项目上下文'
  },
  'Tenant context is required': {
    en: 'Tenant context is required',
    zh: '需要租户上下文'
  },
  'Role ID is required': {
    en: 'Role ID is required',
    zh: '角色 ID 必填'
  },
  'Username is required': {
    en: 'Username is required',
    zh: '用户名必填'
  },
  'Password is required for new users': {
    en: 'Password is required for new users',
    zh: '新用户密码必填'
  },
  'New password is required': {
    en: 'New password is required',
    zh: '新密码必填'
  },
  'Incorrect old password': {
    en: 'Incorrect old password',
    zh: '旧密码不正确'
  },
  'Password must be at least 8 characters': {
    en: 'Password must be at least 8 characters',
    zh: '密码至少需要 8 个字符'
  },
  'Password must contain uppercase, lowercase and numbers': {
    en: 'Password must contain uppercase, lowercase and numbers',
    zh: '密码必须包含大写字母、小写字母和数字'
  },
  'Deleted successfully': {
    en: 'Deleted successfully',
    zh: '删除成功'
  },
  'Permissions updated successfully': {
    en: 'Permissions updated successfully',
    zh: '权限保存成功'
  },
  'App roles updated successfully': {
    en: 'App roles updated successfully',
    zh: '应用角色保存成功'
  },
  'Key reset successfully': {
    en: 'Key reset successfully',
    zh: '密钥重置成功'
  },
  'role cannot be assigned to this app scope': {
    en: 'Role cannot be assigned to this app scope',
    zh: '角色不能分配到当前应用范围'
  },
  'function permissions cannot be modified in the current role context': {
    en: 'Function permissions cannot be modified in the current role context',
    zh: '当前角色上下文中不能修改功能权限'
  },
  'device tag permissions cannot be modified in the current role context': {
    en: 'Device tag permissions cannot be modified in the current role context',
    zh: '当前角色上下文中不能修改设备标签权限'
  },
  'project context is required for inherited role device tag permissions': {
    en: 'Project context is required for inherited role device tag permissions',
    zh: '配置继承角色的设备标签权限时必须选择项目'
  }
}

const prefixMessages = {
  'Failed to fetch apps: ': {
    en: 'Failed to fetch apps: ',
    zh: '获取应用列表失败：'
  },
  'Failed to create app: ': {
    en: 'Failed to create app: ',
    zh: '创建应用失败：'
  },
  'Failed to update app: ': {
    en: 'Failed to update app: ',
    zh: '更新应用失败：'
  },
  'Failed to update app roles: ': {
    en: 'Failed to update app roles: ',
    zh: '更新应用角色失败：'
  },
  'Failed to create role: ': {
    en: 'Failed to create role: ',
    zh: '创建角色失败：'
  },
  'Failed to update role: ': {
    en: 'Failed to update role: ',
    zh: '更新角色失败：'
  },
  'Failed to create project: ': {
    en: 'Failed to create project: ',
    zh: '创建项目失败：'
  },
  'Failed to update project: ': {
    en: 'Failed to update project: ',
    zh: '更新项目失败：'
  },
  'Failed to create tenant: ': {
    en: 'Failed to create tenant: ',
    zh: '创建租户失败：'
  },
  'Failed to update tenant: ': {
    en: 'Failed to update tenant: ',
    zh: '更新租户失败：'
  },
  'Failed to delete tenant: ': {
    en: 'Failed to delete tenant: ',
    zh: '删除租户失败：'
  },
  'Failed to reload plugin: ': {
    en: 'Failed to reload plugin: ',
    zh: '重载插件失败：'
  },
  'Protocol plugin not found: ': {
    en: 'Protocol plugin not found: ',
    zh: '协议插件不存在：'
  },
  'Plugin not found: ': {
    en: 'Plugin not found: ',
    zh: '插件不存在：'
  }
}

export const translateApiMessage = (message, locale = 'en') => {
  if (typeof message !== 'string' || message.length === 0) return message

  const lang = normalizeLocale(locale)
  const exact = exactMessages[message]
  if (exact) return exact[lang]

  for (const [prefix, translations] of Object.entries(prefixMessages)) {
    if (message.startsWith(prefix)) {
      const detail = message.slice(prefix.length)
      return translations[lang] + translateApiMessage(detail, lang)
    }
  }

  return message
}

export const translateApiResponseMessages = (payload, locale = 'en') => {
  if (!payload || typeof payload !== 'object') return payload
  if (typeof payload.message === 'string') {
    payload.message = translateApiMessage(payload.message, locale)
  }
  return payload
}
