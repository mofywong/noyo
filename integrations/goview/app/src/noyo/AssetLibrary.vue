<template>
  <n-modal :show="show" preset="card" :title="t('图片素材', 'Image library')" class="asset-library" @update:show="emit('update:show', $event)">
    <p class="asset-help">{{ t('选择当前大屏中的图片，或关闭后上传新图片。', 'Choose an image from this screen, or close to upload a new image.') }}</p>
    <p v-if="error" role="alert">{{ t('无法加载图片，请重试。', 'Images could not be loaded. Please retry.') }}</p>
    <p v-if="!loading && !items.length && !error" class="asset-empty">{{ t('暂无图片素材', 'No images yet') }}</p>
    <div class="asset-grid">
      <button v-for="item in items" :key="item.id" type="button" class="asset-choice" :aria-pressed="String(item.id) === String(selectedId)" @click="emit('choose', String(item.id)); emit('update:show', false)">
        <img v-if="item.url" :src="item.url" alt="" loading="lazy" />
        <span v-else class="asset-placeholder">{{ t('预览不可用', 'Preview unavailable') }}</span>
        <strong>{{ item.name || `${t('图片', 'Image')} ${item.id}` }}</strong>
        <span>{{ item.width }} × {{ item.height }} · {{ Math.ceil(item.size / 1024) }} KiB</span>
      </button>
    </div>
    <p v-if="loading" role="status">{{ t('正在加载图片…', 'Loading images…') }}</p>
    <footer><button v-if="nextCursor || error" type="button" :disabled="loading" @click="load">{{ t(error ? '重试' : '加载更多', error ? 'Retry' : 'Load more') }}</button><button type="button" @click="emit('update:show', false)">{{ t('关闭', 'Close') }}</button></footer>
  </n-modal>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue'
import { rpc, state } from './bridge'
const props = defineProps<{ show: boolean; selectedId?: string }>()
const emit = defineEmits(['update:show', 'choose'])
const items = ref<any[]>([]), loading = ref(false), error = ref(false), nextCursor = ref(0)
let generation = 0
const t = (zh: string, en: string) => state.locale?.startsWith('en') ? en : zh
async function load() {
  if (loading.value) return
  const current = generation
  loading.value = true; error.value = false
  try {
    const result = await rpc('asset.list', { cursor: nextCursor.value })
    if (current !== generation) return
    const page = (result.items || []).map((item: any) => ({ ...item, url: '' }))
    items.value.push(...page); nextCursor.value = result.nextCursor || 0
    for (const item of page) {
      if (current !== generation || !props.show) break
      try { const preview = await rpc('asset.resolve', { assetId: String(item.id), thumbnail: true }); if (current === generation && /^blob:/.test(preview.url)) { const target = items.value.find(entry => entry.id === item.id); if (target) target.url = preview.url } } catch { /* Each missing preview remains selectable and labelled. */ }
    }
  } catch { if (current === generation) error.value = true }
  finally { if (current === generation) loading.value = false }
}
watch(() => props.show, show => { generation++; loading.value = false; if (show) { items.value = []; nextCursor.value = 0; load() } })
</script>
<style>
.asset-library.n-card{width:min(960px,calc(100vw - 48px));max-height:calc(100vh - 48px);overflow:auto;background:var(--bg-surface);color:var(--text-primary)}.asset-library .n-card__content{padding:24px}.asset-help,.asset-choice span{color:var(--text-secondary)}.asset-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(160px,1fr));gap:16px}.asset-choice{display:flex;flex-direction:column;gap:8px;padding:16px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-primary);text-align:left;cursor:pointer}.asset-choice[aria-pressed=true]{border-color:var(--primary-color)}.asset-choice:focus-visible,.asset-library footer button:focus-visible{outline:2px solid var(--primary-color);outline-offset:2px}.asset-choice img,.asset-placeholder{width:100%;height:128px;object-fit:contain;background:var(--noyo-canvas-bg)}.asset-placeholder{display:grid;place-items:center}.asset-choice strong{max-width:100%;overflow-wrap:anywhere}.asset-empty{padding:96px 24px;text-align:center;color:var(--text-secondary)}.asset-library footer{display:flex;justify-content:flex-end;gap:8px;margin-top:24px}.asset-library footer button{padding:8px 16px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-primary);cursor:pointer}.asset-library [role=alert]{color:var(--danger-color)}
</style>
