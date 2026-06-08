import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import fs from 'node:fs'

const hasPro = fs.existsSync(fileURLToPath(new URL('./src/plugins/pro', import.meta.url)))

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue()],
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
})
