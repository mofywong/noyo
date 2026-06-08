import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import Marketplace from '../views/Marketplace.vue'
import ProductList from '../views/ProductList.vue'
import DeviceList from '../views/DeviceList.vue'
import DeviceTags from '../views/DeviceTags.vue'
import DeviceTopology from '../views/DeviceTopology.vue'
import GatewayManagement from '../views/GatewayManagement.vue'
import GatewayPlugins from '../views/GatewayPlugins.vue'
import GatewayPluginConfig from '../views/GatewayPluginConfig.vue'
import PluginConfig from '../views/PluginConfig.vue'
import UserManagement from '../views/UserManagement.vue'
import TenantManagement from '../views/TenantManagement.vue'
import ProjectManagement from '../views/ProjectManagement.vue'
import RoleManagement from '../views/RoleManagement.vue'
import AppManagement from '../views/AppManagement.vue'
import AppIntegrationGuide from '../views/AppIntegrationGuide.vue'
import AuditLogs from '../views/AuditLogs.vue'
import Settings from '../views/Settings.vue'
import Logs from '../views/Logs.vue'
import License from '../views/License.vue'
import AlarmCenter from '../views/AlarmCenter.vue'
import Login from '../views/Login.vue'
import { loadPlugins, usePlugins } from '../plugins/registry.js'
import { useAuthStore } from '../stores/auth.js'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: Dashboard,
    meta: { requiresAuth: true, permission: 'dashboard:view' }
  },
  {
    path: '/marketplace',
    name: 'Marketplace',
    component: Marketplace,
    meta: { requiresAuth: true, permission: 'plugin:list' }
  },
  {
    path: '/products',
    name: 'Products',
    component: ProductList,
    meta: { requiresAuth: true, permission: 'product:list' }
  },
  {
    path: '/devices',
    name: 'Devices',
    component: DeviceList,
    meta: { requiresAuth: true, permission: 'device:list' }
  },
  {
    path: '/device-tags',
    name: 'DeviceTags',
    component: DeviceTags,
    meta: { requiresAuth: true, permission: 'device_tag:list' }
  },
  {
    path: '/topology',
    name: 'DeviceTopology',
    component: DeviceTopology,
    meta: { requiresAuth: true, permission: 'device:topology' }
  },
  {
    path: '/gateways',
    name: 'GatewayManagement',
    component: GatewayManagement,
    meta: { requiresAuth: true, permission: 'gateway:list' }
  },
  {
    path: '/gateways/:gwSn/plugins',
    name: 'GatewayPlugins',
    component: GatewayPlugins,
    props: true,
    meta: { requiresAuth: true, permission: 'gateway:config' }
  },
  {
    path: '/gateways/:gwSn/plugins/:name',
    name: 'GatewayPluginConfig',
    component: GatewayPluginConfig,
    props: true,
    meta: { requiresAuth: true, permission: 'gateway:config' }
  },
  {
    path: '/plugins/:name',
    name: 'PluginConfig',
    component: PluginConfig,
    props: true,
    meta: { requiresAuth: true, permission: 'plugin:config' }
  },
  {
    path: '/settings',
    name: 'Settings',
    component: Settings
  },
  {
    path: '/settings/users',
    name: 'UserManagement',
    component: UserManagement,
    meta: { requiresAuth: true, permission: 'user:list' }
  },
  {
    path: '/settings/tenants',
    name: 'TenantManagement',
    component: TenantManagement,
    meta: { requiresAuth: true, permission: 'tenant:list' }
  },
  {
    path: '/settings/projects',
    name: 'ProjectManagement',
    component: ProjectManagement,
    meta: { requiresAuth: true, permission: 'project:list' }
  },
  {
    path: '/settings/roles',
    name: 'RoleManagement',
    component: RoleManagement,
    meta: { requiresAuth: true, permission: 'role:list' }
  },
  {
    path: '/settings/apps',
    name: 'AppManagement',
    component: AppManagement,
    meta: { requiresAuth: true, permission: 'app:list' }
  },
  {
    path: '/settings/apps/guide',
    name: 'AppIntegrationGuide',
    component: AppIntegrationGuide,
    meta: { requiresAuth: true, permission: 'app:list' }
  },
  {
    path: '/settings/audit-logs',
    name: 'AuditLogs',
    component: AuditLogs,
    meta: { requiresAuth: true, permission: 'audit:list' }
  },
  {
    path: '/license',
    name: 'License',
    component: License,
    meta: { requiresAuth: true, permission: 'system:license' }
  },
  {
    path: '/logs',
    name: 'Logs',
    component: Logs,
    meta: { requiresAuth: true, permission: 'system:logs' }
  },
  {
    path: '/alarms',
    name: 'AlarmCenter',
    component: AlarmCenter,
    meta: { requiresAuth: true, permission: 'alarm:list' }
  },
  {
    path: '/login/:suffix?',
    name: 'Login',
    component: Login
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

function getFallbackRoute(authStore) {
  for (const r of routes) {
    if (r.meta?.requiresAuth && r.meta?.permission) {
      if (authStore.hasPermission(r.meta.permission)) {
        return { name: r.name }
      }
    }
  }
  return false // No accessible routes
}

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  if (to.name === 'Login') {
    if (authStore.isLoggedIn) {
      const fallback = getFallbackRoute(authStore)
      return next(fallback || '/')
    }
    return next()
  }

  if (!authStore.isLoggedIn) {
    return next('/login')
  }

  if (to.meta?.permission && !authStore.hasPermission(to.meta.permission)) {
    const fallback = getFallbackRoute(authStore)
    if (fallback && fallback.name !== to.name) {
      return next(fallback)
    }
    return next(false)
  }

  next()
})

// Load plugins and register dynamic routes before starting
loadPlugins().then(() => {
  const { extensions } = usePlugins()
  if (extensions.value.routes) {
    extensions.value.routes.forEach(route => {
      if (!route.meta) route.meta = {}
      if (!route.meta.requiresAuth) route.meta.requiresAuth = true
      if (!route.meta.permission) route.meta.permission = 'plugin:config'
      router.addRoute(route)
    })
  }
})

export default router
