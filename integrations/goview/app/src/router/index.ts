import { createRouter, createWebHashHistory } from 'vue-router'
import Editor from '@/noyo/Editor.vue'
const router = createRouter({ history: createWebHashHistory(''), routes: [
 { path: '/chart/home/:id(.*)*', name: 'ChartHome', component: Editor },
 { path: '/chart/preview/:id(.*)*', name: 'ChartPreview', component: Editor },
 { path: '/:pathMatch(.*)*', redirect: '/chart/home/embedded' }
] })
export function setupRouter(app: any) { app.use(router) }
export default router
