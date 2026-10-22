import './bootstrap'
import { createApp } from 'vue'
import App from './App.vue'
import router from '@/router'
import i18n from '@/i18n'
import { setupStore } from '@/store'
import { setupNaive, setupDirectives, setupCustomComponents } from '@/plugins'
import { registerMaterials } from './materials'
import '@/styles/pages/index.scss'
import 'vue3-sketch-ruler/lib/style.css'
import './style.css'

const app = createApp(App)
setupNaive(app)
setupDirectives(app)
setupCustomComponents(app)
setupStore(app)
registerMaterials(app)
app.use(i18n)
window['$t'] = i18n.global.t
app.use(router)
window['$vue'] = app
router.isReady().then(() => app.mount('#app'))
