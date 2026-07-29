const FIELD_TYPES = Object.freeze([
  { type: 'text', group: 'basic', icon: 'bi-input-cursor-text', zh: '单行文本', en: 'Single-line text' },
  { type: 'textarea', group: 'basic', icon: 'bi-text-paragraph', zh: '多行文本', en: 'Long text' },
  { type: 'number', group: 'basic', icon: 'bi-123', zh: '数字', en: 'Number' },
  { type: 'integer', group: 'basic', icon: 'bi-hash', zh: '整数', en: 'Integer' },
  { type: 'boolean', group: 'basic', icon: 'bi-toggle-on', zh: '开关', en: 'Switch' },
  { type: 'select', group: 'basic', icon: 'bi-record-circle', zh: '单选', en: 'Single choice' },
  { type: 'multi_select', group: 'basic', icon: 'bi-check2-square', zh: '多选', en: 'Multiple choice' },
  { type: 'date', group: 'basic', icon: 'bi-calendar3', zh: '日期', en: 'Date' },
  { type: 'datetime', group: 'basic', icon: 'bi-calendar2-week', zh: '日期时间', en: 'Date and time' },
  { type: 'device', group: 'business', icon: 'bi-hdd-network', zh: '选择设备', en: 'Device selector' },
  { type: 'user', group: 'business', icon: 'bi-person', zh: '选择用户', en: 'User selector' },
  { type: 'images', group: 'business', icon: 'bi-images', zh: '上传图片', en: 'Image upload' },
  { type: 'attachment', group: 'business', icon: 'bi-paperclip', zh: '附件引用', en: 'Attachment' }
])

function language(locale = 'zh') {
  return String(locale || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh'
}

function fieldType(type) {
  const value = FIELD_TYPES.find(item => item.type === type)
  if (!value) throw new Error(`unsupported work-order form field type: ${type}`)
  return value
}

export function workOrderFieldPalette(locale = 'zh') {
  const lang = language(locale)
  return [
    {
      key: 'basic',
      label: lang === 'en' ? 'Basic fields' : '基础字段',
      items: FIELD_TYPES.filter(item => item.group === 'basic').map(item => ({ ...item, label: item[lang] }))
    },
    {
      key: 'business',
      label: lang === 'en' ? 'Business fields' : '业务字段',
      items: FIELD_TYPES.filter(item => item.group === 'business').map(item => ({ ...item, label: item[lang] }))
    }
  ]
}

export function createWorkOrderDesignerField(type = 'text', key = '', locale = 'zh') {
  const meta = fieldType(type)
  const lang = language(locale)
  const field = {
    key: key || `field_${Math.random().toString(36).slice(2, 8)}`,
    label: meta[lang],
    type,
    required: false,
    options: []
  }
  if (type === 'select' || type === 'multi_select') {
    field.options = lang === 'en' ? ['Option 1', 'Option 2'] : ['选项 1', '选项 2']
    field.optionsText = field.options.join('\n')
  }
  return field
}

export function insertWorkOrderDesignerField(fields = [], field, index = fields.length) {
  const next = Array.isArray(fields) ? [...fields] : []
  const target = Math.max(0, Math.min(Number(index) || 0, next.length))
  next.splice(target, 0, field)
  return next
}

export function reorderWorkOrderDesignerFields(fields = [], fromIndex, toIndex) {
  const next = Array.isArray(fields) ? [...fields] : []
  const from = Number(fromIndex)
  const to = Math.max(0, Math.min(Number(toIndex), next.length - 1))
  if (!Number.isInteger(from) || !Number.isInteger(to) || from < 0 || from >= next.length || from === to) return next
  const [field] = next.splice(from, 1)
  next.splice(to, 0, field)
  return next
}

export function workOrderDesignerDropTarget(fromIndex, dropIndex, fieldCount) {
  const from = Number(fromIndex)
  const drop = Math.max(0, Math.min(Number(dropIndex), Number(fieldCount)))
  if (!Number.isInteger(from) || !Number.isInteger(drop) || from < 0 || from >= Number(fieldCount)) return -1
  return from < drop ? drop - 1 : drop
}

export function normalizeWorkOrderDesignerOptions(value) {
  return [...new Set(String(value || '').split(/\r?\n/).map(item => item.trim()).filter(Boolean))]
}

export function changeWorkOrderDesignerFieldType(field = {}, type = 'text', locale = 'zh') {
  fieldType(type)
  const next = { ...field, type }
  if (type === 'select' || type === 'multi_select') {
    next.options = Array.isArray(next.options) && next.options.length
      ? [...next.options]
      : (language(locale) === 'en' ? ['Option 1', 'Option 2'] : ['选项 1', '选项 2'])
    next.optionsText = next.options.join('\n')
  } else {
    next.options = []
    next.optionsText = ''
  }

  const value = next.default_value
  const keepDefault = value === undefined
    || (type === 'select' && next.options.includes(value))
    || (type === 'boolean' && typeof value === 'boolean')
    || (['number', 'integer'].includes(type) && typeof value === 'number' && Number.isFinite(value) && (type !== 'integer' || Number.isInteger(value)))
    || (!['select', 'multi_select', 'boolean', 'number', 'integer', 'device', 'user', 'images'].includes(type) && typeof value === 'string')
  if (!keepDefault) delete next.default_value
  return next
}

export function workOrderDesignerFieldType(type, locale = 'zh') {
  const meta = fieldType(type)
  return { ...meta, label: meta[language(locale)] }
}
