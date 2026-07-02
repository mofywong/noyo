const env = import.meta.env || process.env || {}

const readEnv = (key, fallback) => {
  const value = env[key]
  return typeof value === 'string' && value.trim() ? value.trim() : fallback
}

export const appBrand = Object.freeze({
  name: readEnv('VITE_BRAND_NAME', 'Noyo'),
  nameZh: readEnv('VITE_BRAND_NAME_ZH', '诺优 Noyo'),
  logoUrl: readEnv('VITE_BRAND_LOGO', '/Noyo.svg'),
  faviconUrl: readEnv('VITE_BRAND_FAVICON', readEnv('VITE_BRAND_LOGO', '/Noyo.svg')),
  documentTitle: readEnv('VITE_BRAND_TITLE', '诺优Noyo'),
  footerText: readEnv('VITE_BRAND_FOOTER', 'Noyo · Intelligent IoT'),
})

export const brandNameForLocale = (locale) => {
  return String(locale || '').toLowerCase().startsWith('zh') ? appBrand.nameZh : appBrand.name
}

export const brandMessages = Object.freeze({
  en: {
    brand_name: appBrand.name,
    ai_copilot: `${appBrand.name} Copilot`,
    ai_welcome_msg: `Hello! I am ${appBrand.name} Copilot.<br/>I can help you configure protocols, parse point lists, monitor status, or answer technical questions.`,
    auth_login_title: `${appBrand.name} IoT`,
  },
  zh: {
    brand_name: appBrand.nameZh,
    ai_copilot: 'AI 助手',
    ai_welcome_msg: `你好！我是 ${appBrand.nameZh} Copilot。<br/>我可以帮你快速配置协议、解析复杂点表、监控系统状态或解答技术疑问。`,
    auth_login_title: `${appBrand.name} IoT`,
  },
})
