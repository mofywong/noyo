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
import PositionManagement from '../views/PositionManagement.vue'
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
    component: Dashboard
  },
  {
    path: '/marketplace',
    name: 'Marketplace',
    component: Marketplace
  },
  {
    path: '/products',
    name: 'Products',
    component: ProductList
  },
  {
    path: '/devices',
    name: 'Devices',
    component: DeviceList
  },
  {
    path: '/device-tags',
    name: 'DeviceTags',
    component: DeviceTags
  },
  {
    path: '/topology',
    name: 'DeviceTopology',
    component: DeviceTopology
  },
  {
    path: '/gateways',
    name: 'GatewayManagement',
    component: GatewayManagement
  },
  {
    path: '/gateways/:gwSn/plugins',
    name: 'GatewayPlugins',
    component: GatewayPlugins,
    props: true
  },
  {
    path: '/gateways/:gwSn/plugins/:name',
    name: 'GatewayPluginConfig',
    component: GatewayPluginConfig,
    props: true
  },
  {
    path: '/plugins/:name',
    name: 'PluginConfig',
    component: PluginConfig,
    props: true
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
    meta: { requiresAuth: true, systemAdmin: true }
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
    path: '/settings/positions',
    name: 'PositionManagement',
    component: PositionManagement,
    meta: { requiresAuth: true, permission: 'position:list' }
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
    component: License
  },
  {
    path: '/logs',
    name: 'Logs',
    component: Logs
  },
  {
    path: '/alarms',
    name: 'AlarmCenter',
    component: AlarmCenter
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

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()
  
  if (to.name === 'Login') {
    if (authStore.isLoggedIn) {
      return next('/')
    }
    return next()
  }

  if (!authStore.isLoggedIn) {
    return next('/login')
  }

  if (to.meta?.systemAdmin && !authStore.isSystemAdmin) {
    return next('/')
  }

  if (to.meta?.permission && !authStore.hasPermission(to.meta.permission)) {
    return next('/')
  }

  if (to.meta?.roles && to.meta.roles.length > 0) {
    const userRole = authStore.user?.role
    const allowed = to.meta.roles.includes(userRole)
    if (!allowed && !authStore.isSystemAdmin) {
      return next('/')
    }
  }

  next()
})

// Load plugins and register dynamic routes before starting
loadPlugins().then(() => {
  const { extensions } = usePlugins()
  if (extensions.value.routes) {
    extensions.value.routes.forEach(route => {
      if (!route.meta) route.meta = {}
      if (!route.meta.roles) route.meta.roles = ['admin']
      router.addRoute(route)
    })
  }
})

export default router
