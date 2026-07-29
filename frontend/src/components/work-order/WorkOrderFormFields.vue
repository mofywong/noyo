<template>
  <div class="row g-3 work-order-form-fields" :class="{ 'work-order-form-fields--preview': preview }">
    <div v-for="field in fields" :key="field.key" :class="field.originalType === 'textarea' || field.type === 'multi_select' || field.type === 'images' ? 'col-12' : 'col-md-6'">
      <label
        :id="`${inputId(field)}-label`"
        :for="field.type === 'multi_select' ? undefined : inputId(field)"
        class="form-label"
      >
        {{ field.label }}
        <span v-if="field.required" class="text-danger">*</span>
      </label>

      <textarea
        v-if="field.originalType === 'textarea'"
        :id="inputId(field)"
        v-model="values[field.key]"
        class="form-control"
        rows="3"
      ></textarea>
      <select
        v-else-if="field.type === 'enum'"
        :id="inputId(field)"
        v-model="values[field.key]"
        class="form-select"
      >
        <option value="">{{ text('select') }}</option>
        <option v-for="option in field.options || []" :key="option" :value="option">{{ optionLabel(field, option) }}</option>
      </select>
      <div
        v-else-if="field.type === 'multi_select'"
        :id="inputId(field)"
        class="work-order-multi-choice"
        role="group"
        :aria-labelledby="`${inputId(field)}-label`"
      >
        <div v-for="(option, optionIndex) in field.options || []" :key="option" class="form-check">
          <input :id="`${inputId(field)}-${optionIndex}`" v-model="values[field.key]" class="form-check-input" type="checkbox" :value="option">
          <label class="form-check-label" :for="`${inputId(field)}-${optionIndex}`">{{ optionLabel(field, option) }}</label>
        </div>
      </div>
      <select
        v-else-if="field.type === 'device'"
        :id="inputId(field)"
        v-model="values[field.key]"
        class="form-select"
      >
        <option value="">{{ text('selectDevice') }}</option>
        <option v-if="deviceLoading" disabled value="">{{ text('loadingDevices') }}</option>
        <option v-else-if="deviceLoadFailed" disabled value="">{{ text('deviceLoadFailed') }}</option>
        <option v-if="allowRuntimeValues" value="${trigger.deviceCode}">{{ text('triggerDevice') }}</option>
        <option v-for="device in devices" :key="device.code" :value="device.code">{{ device.name ? `${device.name} (${device.code})` : device.code }}</option>
      </select>
      <select
        v-else-if="field.type === 'user'"
        :id="inputId(field)"
        v-model="values[field.key]"
        class="form-select"
      >
        <option value="">{{ text('selectUser') }}</option>
        <option v-if="participantLoading" disabled value="">{{ text('loadingUsers') }}</option>
        <option v-else-if="participantLoadFailed" disabled value="">{{ text('userLoadFailed') }}</option>
        <option v-if="unresolvedParticipantValue(field)" disabled :value="unresolvedParticipantValue(field)">{{ text('unavailableUser') }}</option>
        <option v-for="participant in participants" :key="participant.id" :value="String(participant.id)">{{ participantLabel(participant) }}</option>
      </select>
      <div v-else-if="field.type === 'images'" class="work-order-image-upload">
        <input
          :id="inputId(field)"
          class="visually-hidden"
          type="file"
          accept="image/jpeg,image/png,image/gif,image/webp,image/bmp"
          multiple
          :disabled="preview || isImageUploading(field.key)"
          @change="uploadImages(field, $event)"
        >
        <label class="work-order-image-dropzone" :class="{ 'work-order-image-dropzone--disabled': preview || isImageUploading(field.key) }" :for="inputId(field)" :aria-disabled="preview || isImageUploading(field.key) ? 'true' : 'false'">
          <span class="work-order-image-dropzone__icon"><i class="bi" :class="isImageUploading(field.key) ? 'bi-arrow-repeat' : 'bi-cloud-arrow-up'"></i></span>
          <span><strong>{{ isImageUploading(field.key) ? text('uploadingImages') : text('uploadImages') }}</strong><small>{{ text('imageHint') }}</small></span>
        </label>
        <div v-if="imageUploadErrors[field.key]" class="text-danger small mt-2" role="alert">{{ imageUploadErrors[field.key] }}</div>
        <div v-if="imageValues(field).length" class="work-order-image-grid" :aria-label="text('previewImages')">
          <figure v-for="(imageUrl, imageIndex) in imageValues(field)" :key="`${imageUrl}-${imageIndex}`" class="work-order-image-card">
            <a :href="imageUrl" target="_blank" rel="noopener noreferrer"><img :src="imageUrl" :alt="`${field.label} ${imageIndex + 1}`"></a>
            <button v-if="!preview" type="button" class="work-order-image-remove" :aria-label="`${text('removeImage')} ${imageIndex + 1}`" :title="text('removeImage')" @click="removeImage(field, imageIndex)"><i class="bi bi-x-lg"></i></button>
          </figure>
        </div>
      </div>
      <div v-else-if="field.type === 'boolean'" class="form-check form-switch pt-2">
        <input :id="inputId(field)" v-model="values[field.key]" class="form-check-input" type="checkbox">
        <label :for="inputId(field)" class="form-check-label">{{ text('yes') }}</label>
      </div>
      <input
        v-else-if="field.type === 'number' || field.type === 'integer'"
        :id="inputId(field)"
        v-model.number="values[field.key]"
        type="number"
        :step="field.type === 'integer' ? '1' : 'any'"
        class="form-control"
      >
      <input
        v-else-if="field.type === 'date-time' || field.originalType === 'datetime' || field.originalType === 'date-time'"
        :id="inputId(field)"
        :value="dateTimeInputValue(values[field.key])"
        type="datetime-local"
        class="form-control"
        @input="updateDateTime(field, $event.target.value)"
      >
      <input
        v-else
        :id="inputId(field)"
        v-model="values[field.key]"
        :type="inputType(field.originalType)"
        class="form-control"
        :placeholder="placeholder(field.originalType)"
      >
      <div v-if="field.originalType === 'attachment'" class="form-text">{{ text('attachmentHint') }}</div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import axios from 'axios'
import { createFieldModels, defaultFormValues } from '../../utils/workOrderForm.js'
import { loadAllWorkOrderParticipants } from '../../utils/workOrderApi.js'

const props = defineProps({
  definition: {
    type: Object,
    default: () => ({ fields: [] })
  },
  modelValue: {
    type: Object,
    default: () => ({})
  },
  idPrefix: {
    type: String,
    default: 'work-order-field'
  },
  allowRuntimeValues: {
    type: Boolean,
    default: false
  },
  locale: {
    type: String,
    default: 'zh'
  },
  preview: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['update:modelValue'])
const values = ref({})
const devices = ref([])
const participants = ref([])
const deviceLoading = ref(false)
const participantLoading = ref(false)
const deviceLoadFailed = ref(false)
const participantLoadFailed = ref(false)
const imageUploading = ref({})
const imageUploadErrors = ref({})
let devicesLoaded = false
let participantsLoaded = false

const fields = computed(() => createFieldModels(props.definition || {}))
const lang = computed(() => String(props.locale || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh')
const copy = {
  zh: { select: '请选择', selectDevice: '请选择设备', triggerDevice: '触发设备（规则运行时）', selectUser: '请选择用户', loadingDevices: '正在加载设备…', deviceLoadFailed: '设备加载失败，请稍后重试', loadingUsers: '正在加载用户…', userLoadFailed: '用户加载失败，请稍后重试', unavailableUser: '已删除或不可用用户', yes: '是', uploadImages: '选择图片（支持多张）', uploadingImages: '正在上传图片…', imageHint: '支持 JPG、PNG、GIF、WebP、BMP，单张不超过 10 MB', imageUploadFailed: '图片上传失败，请稍后重试', invalidImageType: '仅支持 JPG、PNG、GIF、WebP 或 BMP 图片', imageTooLarge: '单张图片不能超过 10 MB', removeImage: '移除图片', previewImages: '已上传图片', attachmentHint: '填写已上传附件的地址或存储引用', devicePlaceholder: '设备编码', userPlaceholder: '用户', attachmentPlaceholder: '附件地址或存储引用' },
  en: { select: 'Select', selectDevice: 'Select a device', triggerDevice: 'Triggering device (at rule runtime)', selectUser: 'Select a user', loadingDevices: 'Loading devices…', deviceLoadFailed: 'Unable to load devices. Try again later.', loadingUsers: 'Loading users…', userLoadFailed: 'Unable to load users. Try again later.', unavailableUser: 'Deleted or unavailable user', yes: 'Yes', uploadImages: 'Choose images (multiple allowed)', uploadingImages: 'Uploading images…', imageHint: 'JPG, PNG, GIF, WebP, or BMP; up to 10 MB each', imageUploadFailed: 'Unable to upload the image. Try again later.', invalidImageType: 'Use a JPG, PNG, GIF, WebP, or BMP image', imageTooLarge: 'Each image must be 10 MB or smaller', removeImage: 'Remove image', previewImages: 'Uploaded images', attachmentHint: 'Enter an uploaded attachment URL or storage reference', devicePlaceholder: 'Device code', userPlaceholder: 'User', attachmentPlaceholder: 'Attachment URL or storage reference' }
}
const text = key => copy[lang.value][key] || key

function optionLabel(field, option) {
  if (field?.key !== 'severity') return option
  const labels = {
    zh: { low: '低', normal: '普通', high: '高', urgent: '紧急' },
    en: { low: 'Low', normal: 'Normal', high: 'High', urgent: 'Urgent' }
  }
  return labels[lang.value][String(option)] || option
}

function normalizeValues(source = {}) {
  return { ...defaultFormValues(props.definition || {}), ...source }
}

watch(
  () => [props.definition, props.modelValue],
  () => {
    values.value = normalizeValues(props.modelValue || {})
  },
  { immediate: true, deep: true }
)

watch(values, (next) => {
  if (JSON.stringify(next) !== JSON.stringify(props.modelValue || {})) {
    emit('update:modelValue', { ...next })
  }
}, { deep: true })

const resourceTypeSignature = computed(() => [...new Set(fields.value.map(field => field.type).filter(type => type === 'device' || type === 'user'))].sort().join(','))

watch(resourceTypeSignature, async () => {
  if (props.preview) return
  const resourceTypes = new Set(resourceTypeSignature.value.split(',').filter(Boolean))
  const requests = []
  if (resourceTypes.has('device') && !devicesLoaded && !deviceLoading.value) {
    deviceLoading.value = true
    deviceLoadFailed.value = false
    requests.push(axios.get('/api/devices').then(response => {
      if (response.data?.code !== 0) throw new Error(response.data?.message || 'load devices failed')
      const data = response.data?.data
      devices.value = Array.isArray(data) ? data : (Array.isArray(data?.list) ? data.list : [])
      devicesLoaded = true
    }).catch(() => { deviceLoadFailed.value = true }).finally(() => { deviceLoading.value = false }))
  }
  if (resourceTypes.has('user') && !participantsLoaded && !participantLoading.value) {
    participantLoading.value = true
    participantLoadFailed.value = false
    requests.push(loadAllWorkOrderParticipants().then(items => {
      participants.value = items
      participantsLoaded = true
    }).catch(() => { participantLoadFailed.value = true }).finally(() => { participantLoading.value = false }))
  }
  await Promise.all(requests)
}, { immediate: true })

function inputId(field) {
  return `${props.idPrefix}-${field.key}`
}

function inputType(type) {
  if (type === 'number' || type === 'integer') return 'number'
  if (type === 'date') return 'date'
  if (type === 'datetime' || type === 'date-time') return 'datetime-local'
  return 'text'
}

function placeholder(type) {
  const labels = {
    device: text('devicePlaceholder'),
    user: text('userPlaceholder'),
    attachment: text('attachmentPlaceholder')
  }
  return labels[type] || ''
}

function dateTimeInputValue(value) {
  if (!value) return ''
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return ''
  return new Date(parsed.getTime() - parsed.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}

function updateDateTime(field, value) {
  if (!value) { values.value[field.key] = ''; return }
  const parsed = new Date(value)
  values.value[field.key] = Number.isNaN(parsed.getTime()) ? '' : parsed.toISOString()
}

function participantLabel(participant = {}) {
  const displayName = String(participant.display_name || '').trim()
  const username = String(participant.username || '').trim()
  if (displayName && username && displayName !== username) return `${displayName} (${username})`
  return displayName || username || text('unavailableUser')
}

function unresolvedParticipantValue(field) {
  const value = String(values.value[field.key] || '')
  if (!value || participants.value.some(participant => String(participant.id) === value)) return ''
  return value
}

function imageValues(field) {
  const value = values.value[field.key]
  return Array.isArray(value) ? value.filter(item => typeof item === 'string' && item.trim()) : []
}

function isImageUploading(fieldKey) {
  return Boolean(imageUploading.value[fieldKey])
}

async function uploadImages(field, event) {
  const input = event.target
  const files = [...(input.files || [])]
  input.value = ''
  if (!files.length || props.preview) return
  const allowedTypes = new Set(['image/jpeg', 'image/png', 'image/gif', 'image/webp', 'image/bmp'])
  if (files.some(file => !allowedTypes.has(file.type))) {
    imageUploadErrors.value = { ...imageUploadErrors.value, [field.key]: text('invalidImageType') }
    return
  }
  if (files.some(file => file.size > 10 * 1024 * 1024)) {
    imageUploadErrors.value = { ...imageUploadErrors.value, [field.key]: text('imageTooLarge') }
    return
  }
  imageUploading.value = { ...imageUploading.value, [field.key]: true }
  imageUploadErrors.value = { ...imageUploadErrors.value, [field.key]: '' }
  try {
    const existing = imageValues(field)
    const uploaded = []
    for (const file of files) {
      const formData = new FormData()
      formData.append('file', file)
      const response = await axios.post('/api/work-orders/images', formData)
      if (response.data?.code !== 0 || !response.data?.data?.url) throw new Error(response.data?.message || 'upload image failed')
      uploaded.push(response.data.data.url)
      values.value[field.key] = [...existing, ...uploaded]
    }
  } catch {
    imageUploadErrors.value = { ...imageUploadErrors.value, [field.key]: text('imageUploadFailed') }
  } finally {
    imageUploading.value = { ...imageUploading.value, [field.key]: false }
  }
}

function removeImage(field, imageIndex) {
  values.value[field.key] = imageValues(field).filter((_, index) => index !== imageIndex)
}
</script>

<style scoped>
.work-order-form-fields--preview > div { width: 100%; }
.work-order-multi-choice { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .55rem 1rem; min-height: 2.5rem; padding: .7rem .8rem; border: 1px solid var(--bs-border-color); border-radius: var(--bs-border-radius); background: var(--bs-body-bg); }
.work-order-multi-choice .form-check { min-width: 0; margin: 0; }
.work-order-multi-choice .form-check-label { overflow-wrap: anywhere; }
.work-order-image-upload { min-width: 0; }
.work-order-image-dropzone { display: flex; min-height: 5.5rem; align-items: center; justify-content: center; gap: .8rem; padding: 1rem; border: 1px dashed color-mix(in srgb, var(--bs-primary) 48%, var(--bs-border-color)); border-radius: .65rem; background: color-mix(in srgb, var(--bs-primary) 5%, var(--bs-body-bg)); color: var(--bs-body-color); cursor: pointer; text-align: left; transition: border-color .15s ease, background-color .15s ease; }
.work-order-image-dropzone:hover { border-color: var(--bs-primary); background: color-mix(in srgb, var(--bs-primary) 9%, var(--bs-body-bg)); }
.work-order-image-dropzone--disabled { cursor: default; opacity: .72; }
.work-order-image-dropzone__icon { display: grid; width: 2.5rem; height: 2.5rem; flex: 0 0 auto; place-items: center; border-radius: .65rem; background: color-mix(in srgb, var(--bs-primary) 15%, transparent); color: var(--bs-primary); font-size: 1.15rem; }
.work-order-image-dropzone__icon .bi-arrow-repeat { animation: work-order-image-spin 1s linear infinite; }
.work-order-image-dropzone strong, .work-order-image-dropzone small { display: block; }
.work-order-image-dropzone small { margin-top: .2rem; color: var(--bs-secondary-color); line-height: 1.35; }
.work-order-image-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(6.5rem, 1fr)); gap: .65rem; margin-top: .75rem; }
.work-order-image-card { position: relative; min-width: 0; margin: 0; overflow: hidden; border: 1px solid var(--bs-border-color); border-radius: .6rem; background: var(--bs-tertiary-bg); aspect-ratio: 4 / 3; }
.work-order-image-card a, .work-order-image-card img { display: block; width: 100%; height: 100%; }
.work-order-image-card img { object-fit: cover; }
.work-order-image-remove { position: absolute; top: .3rem; right: .3rem; display: grid; width: 2rem; height: 2rem; place-items: center; border: 1px solid rgba(255, 255, 255, .35); border-radius: 50%; background: rgba(15, 23, 42, .78); color: white; }
.work-order-image-remove:hover, .work-order-image-remove:focus-visible { background: var(--bs-danger); }
@keyframes work-order-image-spin { to { transform: rotate(360deg); } }
@media (max-width: 575px) { .work-order-multi-choice { grid-template-columns: 1fr; } }
</style>
