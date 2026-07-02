import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import fs from 'node:fs'

const hasPro = fs.existsSync(fileURLToPath(new URL('./src/plugins/pro', import.meta.url)))
const escapeHtmlAttr = (value) => String(value)
  .replaceAll('&', '&amp;')
  .replaceAll('"', '&quot;')
  .replaceAll('<', '&lt;')
  .replaceAll('>', '&gt;')

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const brandLogo = env.VITE_BRAND_LOGO || '/Noyo.svg'
  const brandFavicon = env.VITE_BRAND_FAVICON || brandLogo
  const brandTitle = env.VITE_BRAND_TITLE || '诺优Noyo'

  return {
    plugins: [
      vue(),
      {
        name: 'noyo-brand-html',
        transformIndexHtml(html) {
          return html
            .replaceAll('__NOYO_BRAND_FAVICON__', escapeHtmlAttr(brandFavicon))
            .replaceAll('__NOYO_BRAND_TITLE__', escapeHtmlAttr(brandTitle))
        },
      },
    ],
    resolve: {
      alias: [
        ...(hasPro ? [] : [
          { find: '@/plugins/pro/protocol/gb28181/GB28181PlayerWidget.vue', replacement: fileURLToPath(new URL('./src/components/EmptyWidget.vue', import.meta.url)) }
        ]),
        { find: '@', replacement: fileURLToPath(new URL('./src', import.meta.url)) }
      ],
      preserveSymlinks: true
    },
    server: {
      host: true,
      proxy: {
        '/api': {
          target: 'http://localhost:8989',
          changeOrigin: true,
          ws: true,
        },
        '/data': {
          target: 'http://localhost:8989',
          changeOrigin: true,
        }
      }
    },
    build: {
      outDir: '../backend/dist',
      emptyOutDir: true
    }
  }
})
