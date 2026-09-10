<template>
  <section v-if="loading || error || recordings.length" class="alarm-video-evidence" :aria-label="text.title">
    <h3 class="h6">{{ text.title }}</h3>
    <p v-if="error" role="alert" class="text-danger small">
      {{ text.loadFailed }} <button type="button" class="btn btn-link btn-sm" @click="load">{{ text.retry }}</button>
    </p>
    <p v-else-if="loading" role="status" class="text-body-secondary small">{{ text.loading }}</p>
    <template v-if="recordings.length">
      <label v-if="recordings.length > 1" class="form-label small" :for="`alarm-video-${alarmId}`">{{ text.select }}</label>
      <select v-if="recordings.length > 1" :id="`alarm-video-${alarmId}`" v-model="selectedId" class="form-select mb-3">
        <option v-for="record in recordings" :key="record.id" :value="record.id">{{ eventTime(record) }} · {{ statusText(record.status) }}</option>
      </select>
      <template v-if="selected">
        <p class="small text-body-secondary" role="status">{{ statusText(selected.status) }}</p>
        <p v-if="selected.partial" class="small text-warning">{{ text.partialHint }}</p>
        <template v-if="playable">
          <video :key="selected.id" :src="previewUrl" controls playsinline preload="metadata" tabindex="0" :aria-label="text.title" @error="playbackError = true" @loadeddata="playbackError = false">{{ text.unsupported }}</video>
          <p v-if="playbackError" role="alert" class="small text-danger">{{ text.playFailed }}</p>
          <p class="small text-body-secondary mt-2 mb-2">{{ text.range }}: {{ formatTime(selected.start_at) }} – {{ formatTime(selected.end_at) }}</p>
          <a :href="previewUrl" class="btn btn-outline-primary btn-sm" download rel="noreferrer">{{ language === 'en' ? 'Download annotated recording' : '下载标注录像' }}</a>
        </template>
        <p v-else-if="selected.status === 'failed'" class="small text-body-secondary">{{ text.failedHint }}</p>
      </template>
    </template>
  </section>
</template>

<script setup>
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import axios from 'axios'
import { useAuthStore } from '../stores/auth.js'

const props = defineProps({ alarmId: { type: String, required: true }, recordingId: { type: String, default: '' }, language: { type: String, default: 'zh' } })
const auth = useAuthStore()
const recordings = ref([])
const selectedId = ref('')
const loading = ref(true)
const error = ref(false)
const playbackError = ref(false)
let timer
let controller
let generation = 0
const copy = {
  zh: { title: '告警录像', select: '选择告警录像', loading: '正在加载录像…', capturing: '正在录制告警后 5 秒视频…', rendering: '正在生成标注录像…', uploading: '等待网关上传录像…', ready: '录像已就绪', partial: '部分录像已就绪', failed: '录像生成失败', partialHint: '视频流启动、断流或资源限制导致前后片段不完整，实际录像时间如下。', failedHint: '本次告警未能生成可播放录像，截图和告警记录仍可查看。', source: '下载原始视频流', range: '实际录像时间', loadFailed: '录像状态加载失败。', retry: '重试', unsupported: '浏览器不支持视频播放。', playFailed: '视频加载失败，请刷新详情后重试。', unknown: '等待录像状态', unknownTime: '告警录像' },
  en: { title: 'Alarm recording', select: 'Select recording', loading: 'Loading recordings…', capturing: 'Recording 5 seconds after the alarm…', rendering: 'Preparing annotated recording…', uploading: 'Waiting for gateway upload…', ready: 'Recording ready', partial: 'Partial recording ready', failed: 'Recording failed', partialHint: 'Stream startup, interruption or resource limits shortened this clip. The available time range is shown below.', failedHint: 'A playable recording could not be generated. The snapshot and alarm record remain available.', source: 'Download source stream', range: 'Recorded time range', loadFailed: 'Could not load recording status.', retry: 'Retry', unsupported: 'Your browser does not support video playback.', playFailed: 'Could not load the video. Refresh the details and try again.', unknown: 'Waiting for recording status', unknownTime: 'Alarm recording' }
}
const text = computed(() => copy[props.language] || copy.zh)
const selected = computed(() => recordings.value.find(record => record.id === selectedId.value))
const playable = computed(() => ['ready', 'partial'].includes(selected.value?.status))
const fileUrl = kind => {
  if (!selected.value) return ''
  const query = new URLSearchParams({ token: auth.token, media_project_id: String(selected.value.project_id), media_tenant_id: String(selected.value.tenant_id) })
  return `/api/alarm-instances/${encodeURIComponent(props.alarmId)}/media/${encodeURIComponent(selected.value.id)}/${kind}?${query}`
}
const previewUrl = computed(() => fileUrl('preview.mp4'))
const statusText = status => text.value[status] || text.value.unknown
const formatTime = time => time ? new Date(time).toLocaleString(props.language === 'en' ? 'en-US' : 'zh-CN') : text.value.unknownTime
const eventTime = record => formatTime(record.occurred_at)

async function load() {
  clearTimeout(timer)
  controller?.abort()
  controller = new AbortController()
  const current = ++generation
  try {
    const response = await axios.get(`/api/alarm-instances/${encodeURIComponent(props.alarmId)}/media`, { signal: controller.signal })
    if (current !== generation) return
    if (response.data?.code !== 0 || !Array.isArray(response.data.data)) throw new Error('Invalid recording response')
    recordings.value = props.recordingId ? response.data.data.filter(record => record.id === props.recordingId) : response.data.data
    if (!recordings.value.some(record => record.id === selectedId.value)) selectedId.value = recordings.value[0]?.id || ''
    error.value = false
  } catch (failure) {
    if (current !== generation || axios.isCancel(failure)) return
    error.value = true
  } finally {
    if (current === generation) {
      loading.value = false
      if (error.value || recordings.value.some(record => !['ready', 'partial', 'failed'].includes(record.status))) timer = setTimeout(load, error.value ? 10000 : 3000)
    }
  }
}
watch(() => [props.alarmId, props.recordingId], () => {
  generation++
  recordings.value = []
  selectedId.value = ''
  loading.value = true
  error.value = false
  load()
}, { immediate: true })
watch(selectedId, () => { playbackError.value = false })
onBeforeUnmount(() => { generation++; clearTimeout(timer); controller?.abort() })
</script>

<style scoped>
.alarm-video-evidence { margin-block: 24px; padding: 16px; border: 1px solid var(--bs-border-color); border-radius: var(--bs-border-radius); background: var(--bs-body-bg); color: var(--bs-body-color); }
.alarm-video-evidence video { display: block; width: 100%; max-height: 480px; border-radius: var(--bs-border-radius); background: var(--bs-tertiary-bg); }
</style>
