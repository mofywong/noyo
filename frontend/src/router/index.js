import { createRouter, createWebHistory } from 'vue-router'
import Dashboard from '../views/Dashboard.vue'
import Marketplace from '../views/Marketplace.vue'
import ProductList from '../views/ProductList.vue'
import DeviceList from '../views/DeviceList.vue'
import DeviceTopology from '../views/DeviceTopology.vue'
import PluginConfig from '../views/PluginConfig.vue'
import Settings from '../views/Settings.vue'
import Logs from '../views/Logs.vue'
import License from '../views/License.vue'
import { loadPlugins, usePlugins } from '../plugins/registry.js'

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
    path: '/topology',
    name: 'DeviceTopology',
    component: DeviceTopology
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
    path: '/license',
    name: 'License',
    component: License
  },
  {
    path: '/logs',
    name: 'Logs',
    component: Logs
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

// Load plugins and register dynamic routes before starting
loadPlugins().then(() => {
  const { extensions } = usePlugins()
  if (extensions.value.routes) {
    extensions.value.routes.forEach(route => {
      router.addRoute(route)
    })
  }
})

export default router
