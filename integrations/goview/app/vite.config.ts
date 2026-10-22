import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'
// The widget harness is development-only tooling, but the bundle it has to check
// is the production one: dev mode compiles each widget on demand, so a slow
// first transform looks exactly like a widget that renders nothing. Building it
// behind a flag lets the acceptance run inspect real chunks without shipping the
// harness (it fabricates device traffic) to operators.
const withHarness = process.env.NOYO_HARNESS === '1'
export default defineConfig({
  base: process.env.NOYO_SCREEN_BASE || '/screen-editor/goview-5ab05b4-noyo14/',
  plugins: [vue({ template: { compilerOptions: { isCustomElement: tag => tag.startsWith('iconify-icon') } } })],
  resolve: { alias: { '@': resolve(__dirname, 'src'), 'vue-i18n': 'vue-i18n/dist/vue-i18n.cjs.js' }, dedupe: ['vue'] },
  css: { preprocessorOptions: { scss: { additionalData: '@import "src/styles/common/style.scss";' } } },
  define: { __VUE_PROD_HYDRATION_MISMATCH_DETAILS__: false },
  server: { host: '127.0.0.1', port: 3002, open: false },
  build: {
    outDir: 'dist',
    // The harness awaits its module imports at the top level, which the shipping
    // target rejects; verification builds therefore use the newer target. Both
    // produce the same module graph and chunk layout.
    target: withHarness ? 'esnext' : 'es2020',
    chunkSizeWarningLimit: 3000,
    // Only the verification build overrides the entry list; spreading keeps
    // Vite's own default single-page input otherwise.
    ...(withHarness
      ? { rollupOptions: { input: { index: resolve(__dirname, 'index.html'), harness: resolve(__dirname, 'harness.html') } } }
      : {})
  }
})
