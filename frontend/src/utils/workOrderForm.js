const CONTROLLED_TYPES = new Set(['string', 'number', 'integer', 'boolean', 'enum', 'date-time', 'text', 'textarea', 'select', 'multi_select', 'date', 'datetime', 'device', 'user', 'images', 'attachment'])
const RESOURCE_TYPES = new Set(['device', 'user', 'space', 'alarm'])

export const controlledFormTypes = Object.freeze([...CONTROLLED_TYPES])

function propertyMap(definition = {}) {
  if (definition.schema?.properties && typeof definition.schema.properties === 'object') return definition.schema.properties
  if (definition.properties && typeof definition.properties === 'object') return definition.properties
  const result = {}
  for (const field of Array.isArray(definition.fields) ? definition.fields : []) if (field?.key) result[field.key] = field
  return result
}

function normalizeType(field = {}) {
  if (field.type === 'text' || field.type === 'textarea') return 'string'
  if (field.type === 'select') return 'enum'
  if (field.type === 'multi_select') return 'multi_select'
  if (field.type === 'date' || field.type === 'datetime') return field.type === 'datetime' ? 'date-time' : 'string'
  if (field.format === 'date-time') return 'date-time'
  if (Array.isArray(field.enum)) return 'enum'
  if (field['x-noyo-images']) return 'images'
  if (field['x-noyo-resource']) return field['x-noyo-resource']
  if (field['x-noyo-attachment']) return 'attachment'
  return field.type
}

function validateType(field, key) {
  const type = normalizeType(field)
  if (!CONTROLLED_TYPES.has(type)) throw new Error(`unsupported form field type: ${field?.type || type} (${key})`)
  if (field['x-noyo-resource'] && !RESOURCE_TYPES.has(field['x-noyo-resource'])) throw new Error(`unsupported resource type: ${field['x-noyo-resource']} (${key})`)
  if (field['x-noyo-attachment'] && field.type !== 'string') throw new Error(`attachment field must be string: ${key}`)
  if (field['x-noyo-images'] && (field.type !== 'array' || field.items?.type !== 'string')) throw new Error(`image field must be an array of strings: ${key}`)
  if ((type === 'enum' || type === 'multi_select' || field.type === 'select') && !Array.isArray(field.enum || field.options)) throw new Error(`enum options are required: ${key}`)
  return type
}

export function createFieldModel(field = {}, key = field.key || field.name) {
  if (!key) throw new Error('form field key is required')
  const type = validateType(field, key)
  const options = Array.isArray(field.enum) ? [...field.enum] : (Array.isArray(field.options) ? [...field.options] : [])
  return Object.freeze({ key, type, originalType: field.type || type, label: typeof field.title === 'string' ? field.title : (field.label || key), required: Boolean(field.required), defaultValue: field.default !== undefined ? field.default : field.default_value, options, resource: field['x-noyo-resource'] || field.resourceType || null, resourceType: field['x-noyo-resource'] || field.resourceType || null, images: field['x-noyo-images'] === true, attachment: field['x-noyo-attachment'] === true })
}

export function createFieldModels(definition = {}, uiSchemaOverride = undefined) {
  const properties = propertyMap(definition)
  const required = new Set(definition.required || definition.schema?.required || [])
  const order = definition.field_order || definition.schema?.['x-noyo-field-order'] || Object.keys(properties)
  const keys = [...order, ...Object.keys(properties).filter(key => !order.includes(key))]
  const models = keys.map(key => createFieldModel({ ...properties[key], required: required.has(key) || properties[key]?.required }, key))
  const uiOrder = (uiSchemaOverride || definition.ui_schema || definition.uiSchema)?.order
  if (Array.isArray(uiOrder)) {
    const positions = new Map(uiOrder.map((key, index) => [key, index]))
    return models.slice().sort((a, b) => (positions.get(a.key) ?? 9999) - (positions.get(b.key) ?? 9999))
  }
  if (uiSchemaOverride && !Array.isArray(uiOrder)) {
    const positions = new Map(Object.entries(uiSchemaOverride).filter(([, value]) => Number.isFinite(value?.['ui:order'])).map(([key, value]) => [key, value['ui:order']]))
    if (positions.size) return models.slice().sort((a, b) => (positions.get(a.key) ?? 9999) - (positions.get(b.key) ?? 9999))
  }
  return models
}

export const generateFieldModels = createFieldModels
export const buildFieldModels = createFieldModels

export function defaultFormValues(definition = {}) {
  const values = {}
  for (const field of createFieldModels(definition)) {
    if (field.defaultValue !== undefined) values[field.key] = field.defaultValue
    else if (field.type === 'boolean') values[field.key] = false
    else if (field.type === 'multi_select' || field.type === 'images') values[field.key] = []
  }
  return values
}
export const getDefaultFormValues = defaultFormValues

export function initialFormValues(modelsOrDefinition, source = {}) {
  const models = Array.isArray(modelsOrDefinition) ? modelsOrDefinition : createFieldModels(modelsOrDefinition || {})
  const values = { ...(Array.isArray(modelsOrDefinition) ? {} : defaultFormValues(modelsOrDefinition || {})), ...source }
  for (const field of models) if (values[field.key] === undefined && field.defaultValue !== undefined) values[field.key] = field.defaultValue
  for (const field of models) if (values[field.key] === undefined && field.type === 'boolean') values[field.key] = false
  for (const field of models) if (values[field.key] === undefined && (field.type === 'multi_select' || field.type === 'images')) values[field.key] = []
  return values
}

export function validateFormDefinition(definition = {}) {
  if (definition.schema && definition.schema.type !== 'object') throw new Error('schema root type must be object')
  const fields = createFieldModels(definition)
  const known = new Set(fields.map(field => field.key))
  const uiOrder = definition.ui_schema?.order || definition.uiSchema?.order || []
  for (const key of uiOrder) if (!known.has(key)) throw new Error(`UI schema references unknown field: ${key}`)
  return fields
}

export function formValidationMessages(definitionOrModels = {}, values = {}) {
  const fields = Array.isArray(definitionOrModels) ? definitionOrModels : createFieldModels(definitionOrModels)
  const messages = {}
  for (const field of fields) {
    const value = values[field.key]
    const empty = value === undefined || value === null || value === '' || (Array.isArray(value) && value.length === 0)
    if (field.required && empty) messages[field.key] = 'This field is required / 此字段为必填项'
    if (empty) continue
    if ((field.type === 'number' || field.type === 'integer') && (typeof value !== 'number' || Number.isNaN(value))) messages[field.key] = 'Invalid number / 数值无效'
    if (field.type === 'integer' && !Number.isInteger(value)) messages[field.key] = 'Invalid integer / 整数无效'
    if (field.type === 'boolean' && typeof value !== 'boolean') messages[field.key] = 'Invalid boolean / 布尔值无效'
    if (field.type === 'enum' && !field.options.includes(value)) messages[field.key] = 'Invalid option / 选项无效'
    if (field.type === 'multi_select' && (!Array.isArray(value) || value.some(option => !field.options.includes(option)))) messages[field.key] = 'Invalid options / 选项无效'
    if (field.type === 'images' && (!Array.isArray(value) || value.some(image => typeof image !== 'string' || !image.trim()) || new Set(value).size !== value.length)) messages[field.key] = 'Invalid images / 图片无效'
  }
  return messages
}

export function validateFormValues(definitionOrModels = {}, values = {}) {
  const messages = formValidationMessages(definitionOrModels, values)
  if (!Array.isArray(definitionOrModels)) return messages
  const translated = {}
  for (const [key, message] of Object.entries(messages)) {
    if (message.includes('required')) translated[key] = '此字段为必填项'
    else if (message.includes('option')) translated[key] = '选项无效'
    else if (message.includes('integer')) translated[key] = '请输入整数'
    else if (message.includes('number')) translated[key] = '请输入数字'
    else if (message.includes('boolean')) translated[key] = '请输入布尔值'
    else if (message.includes('images')) translated[key] = '图片无效'
    else translated[key] = message
  }
  return translated
}

export function resourceFieldMetadata(definition = {}) {
  return createFieldModels(definition).filter(field => field.resource).map(field => Object.freeze({ key: field.key, type: field.resource, resourceType: field.resource, label: field.label }))
}
export const getResourceFieldMetadata = resourceFieldMetadata

export function fieldModels(schema = {}, uiSchema = {}) { return createFieldModels({ schema }, uiSchema) }
export function inputTypeForField(field) { if (field.type === 'number' || field.type === 'integer') return 'number'; if (field.type === 'date-time') return 'datetime-local'; if (field.type === 'boolean') return 'checkbox'; return 'text' }
