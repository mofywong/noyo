<template>
  <div class="form-designer" :class="{ 'form-designer--readonly': readonly, 'form-designer--dragging': dragging }">
    <aside class="form-designer__palette" :aria-label="text('palette')">
      <div class="form-designer__panel-head"><strong>{{ text('palette') }}</strong><span>{{ text('dragHint') }}</span></div>
      <section v-for="group in palette" :key="group.key" class="form-designer__palette-group">
        <h6>{{ group.label }}</h6>
        <div class="form-designer__palette-grid">
          <button v-for="item in group.items" :key="item.type" type="button" class="form-designer__palette-item" :disabled="readonly" :title="text('dragHint')" @pointerdown="startPointerDrag($event, { kind: 'palette', type: item.type })" @click="onPaletteClick($event, item.type)">
            <i class="bi" :class="item.icon"></i><span>{{ item.label }}</span><i class="bi bi-grip-vertical form-designer__grip"></i>
          </button>
        </div>
      </section>
    </aside>

    <main class="form-designer__canvas" @dragover.prevent @drop="dropAt($event, fields.length)">
      <div class="form-designer__canvas-head"><div><strong>{{ text('preview') }}</strong><span>{{ text('previewHint') }}</span></div><span class="form-designer__count">{{ fieldCountLabel }}</span></div>
      <div class="form-designer__paper">
        <div v-if="!fields.length" class="form-designer__empty" data-form-drop-index="0" :class="{ 'form-designer__drop-target--active': activeDropIndex === 0 }" @dragover.prevent="markDropTarget(0)" @drop.stop="dropAt($event, 0)"><i class="bi bi-ui-checks-grid"></i><strong>{{ text('emptyTitle') }}</strong><span>{{ text('emptyHint') }}</span></div>
        <template v-for="(field, index) in fields" :key="field.key">
          <div class="form-designer__drop-zone" :data-form-drop-index="index" :class="{ 'form-designer__drop-target--active': activeDropIndex === index }" @dragover.prevent="markDropTarget(index)" @drop.stop="dropAt($event, index)"><span>{{ text('dropHere') }}</span></div>
          <article class="form-designer__field" :class="{ 'form-designer__field--selected': selectedIndex === index, 'form-designer__field--drag-source': dragging && dragPayload?.kind === 'field' && dragPayload.index === index }" :tabindex="readonly ? -1 : 0" role="group" :aria-label="field.label" @focus="selectedIndex = index" @click="selectedIndex = index" @keydown.enter.prevent="selectedIndex = index">
            <button v-if="!readonly" type="button" class="form-designer__field-delete" :aria-label="text('delete')" :title="text('delete')" @click.stop="removeField(index)"><i class="bi bi-x-lg"></i></button>
            <button v-if="!readonly" type="button" class="form-designer__field-handle" :aria-label="text('dragField')" :title="text('dragField')" @pointerdown.stop="startPointerDrag($event, { kind: 'field', index })"><i class="bi bi-grip-vertical"></i></button>
            <WorkOrderFormFields v-model="previewValues" :definition="{ fields: [field] }" :id-prefix="`form-preview-${index}`" :locale="lang" preview />
            <button v-if="!readonly" type="button" class="form-designer__insert" :title="text('insertAfter')" @click.stop="addField('text', index + 1)"><i class="bi bi-plus-lg"></i><span>{{ text('insertAfter') }}</span></button>
          </article>
        </template>
        <div v-if="fields.length" class="form-designer__drop-zone form-designer__drop-zone--last" :data-form-drop-index="fields.length" :class="{ 'form-designer__drop-target--active': activeDropIndex === fields.length }" @dragover.prevent="markDropTarget(fields.length)" @drop.stop="dropAt($event, fields.length)"><span>{{ text('dropEnd') }}</span></div>
      </div>
    </main>

    <aside class="form-designer__properties" :aria-label="text('properties')">
      <div class="form-designer__panel-head"><strong>{{ selectedField ? fieldTypeLabel : text('properties') }}</strong><span>{{ selectedField ? text('selectedHint') : text('selectHint') }}</span></div>
      <div v-if="selectedField" class="form-designer__property-body">
        <label class="form-label" :for="propertyInputId('label')">{{ text('label') }} *</label>
        <input :id="propertyInputId('label')" :value="selectedField.label" class="form-control" :class="{ 'is-invalid': labelError }" :aria-invalid="labelError ? 'true' : 'false'" :placeholder="text('labelPlaceholder')" :disabled="readonly" @input="updateSelected({ label: $event.target.value })">
        <div v-if="labelError" class="invalid-feedback d-block">{{ text('labelError') }}</div>
        <div class="mt-3"><label class="form-label" :for="propertyInputId('type')">{{ text('type') }}</label><select :id="propertyInputId('type')" :value="selectedField.type" class="form-select" :disabled="readonly" @change="changeSelectedType($event.target.value)"><option v-for="item in allPaletteItems" :key="item.type" :value="item.type">{{ item.label }}</option></select></div>
        <div class="mt-3"><label class="form-label" :for="propertyInputId('key')">{{ text('identifier') }}</label><input :id="propertyInputId('key')" :value="selectedField.key" class="form-control font-monospace" disabled><div class="form-text">{{ text('identifierHint') }}</div></div>
        <div v-if="choiceField" class="mt-3"><label class="form-label" :for="propertyInputId('options')">{{ text('options') }} *</label><textarea :id="propertyInputId('options')" :value="selectedField.optionsText" class="form-control" :class="{ 'is-invalid': optionsError }" rows="6" :aria-invalid="optionsError ? 'true' : 'false'" :disabled="readonly" :placeholder="text('optionsHint')" @input="updateOptions($event.target.value)"></textarea><div v-if="optionsError" class="invalid-feedback d-block">{{ text('optionsError') }}</div></div>
        <div v-if="selectedField.type === 'select'" class="mt-3"><label class="form-label" :for="propertyInputId('default')">{{ text('defaultValue') }}</label><select :id="propertyInputId('default')" :value="selectedField.default_value ?? ''" class="form-select" :disabled="readonly" @change="updateDefault($event.target.value)"><option value="">{{ text('notSet') }}</option><option v-for="option in selectedField.options || []" :key="option" :value="option">{{ option }}</option></select></div>
        <div v-else-if="selectedField.type === 'boolean'" class="mt-3"><label class="form-label" :for="propertyInputId('default')">{{ text('defaultValue') }}</label><select :id="propertyInputId('default')" :value="selectedField.default_value ?? ''" class="form-select" :disabled="readonly" @change="updateDefault($event.target.value)"><option value="">{{ text('notSet') }}</option><option value="true">{{ text('yes') }}</option><option value="false">{{ text('no') }}</option></select></div>
        <div v-else-if="!['device', 'user', 'multi_select', 'images'].includes(selectedField.type)" class="mt-3"><label class="form-label" :for="propertyInputId('default')">{{ text('defaultValue') }}</label><input :id="propertyInputId('default')" :value="defaultInputValue" class="form-control" :class="{ 'is-invalid': defaultError }" :type="defaultInputType" :step="selectedField.type === 'integer' ? '1' : undefined" :aria-invalid="defaultError ? 'true' : 'false'" :disabled="readonly" @input="updateDefault($event.target.value)"><div v-if="defaultError" class="invalid-feedback d-block">{{ text('integerDefaultError') }}</div></div>
        <div class="form-designer__switch-row mt-4"><label :for="`designer-required-${selectedField.key}`"><strong>{{ text('required') }}</strong><span>{{ text('requiredHint') }}</span></label><div class="form-check form-switch"><input :id="`designer-required-${selectedField.key}`" :checked="selectedField.required" class="form-check-input" type="checkbox" :disabled="readonly" @change="updateSelected({ required: $event.target.checked })"></div></div>
        <div v-if="!readonly" class="d-flex gap-2 mt-4"><button class="btn btn-outline-secondary flex-fill" :disabled="selectedIndex === 0" @click="moveSelected(-1)"><i class="bi bi-arrow-up me-1"></i>{{ text('moveUp') }}</button><button class="btn btn-outline-secondary flex-fill" :disabled="selectedIndex === fields.length - 1" @click="moveSelected(1)"><i class="bi bi-arrow-down me-1"></i>{{ text('moveDown') }}</button></div>
        <button v-if="!readonly" class="btn btn-outline-danger w-100 mt-2" @click="removeField(selectedIndex)"><i class="bi bi-trash me-1"></i>{{ text('deleteField') }}</button>
      </div>
      <div v-else class="form-designer__property-empty"><i class="bi bi-cursor"></i><span>{{ text('selectHint') }}</span></div>
    </aside>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import WorkOrderFormFields from './WorkOrderFormFields.vue'
import { changeWorkOrderDesignerFieldType, createWorkOrderDesignerField, insertWorkOrderDesignerField, normalizeWorkOrderDesignerOptions, reorderWorkOrderDesignerFields, workOrderDesignerDropTarget, workOrderDesignerFieldType, workOrderFieldPalette } from '../../utils/workOrderFormDesigner.js'

const props = defineProps({ modelValue: { type: Object, default: () => ({ fields: [] }) }, readonly: { type: Boolean, default: false }, locale: { type: String, default: 'zh' } })
const emit = defineEmits(['update:modelValue'])
const selectedIndex = ref(-1)
const dragPayload = ref(null)
const dragging = ref(false)
const activeDropIndex = ref(-1)
const pointerDrag = ref(null)
const previewValues = ref({})
let suppressPaletteClick = false
const lang = computed(() => String(props.locale || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh')
const fields = computed(() => Array.isArray(props.modelValue?.fields) ? props.modelValue.fields : [])
const palette = computed(() => workOrderFieldPalette(lang.value))
const allPaletteItems = computed(() => palette.value.flatMap(group => group.items))
const selectedField = computed(() => fields.value[selectedIndex.value] || null)
const choiceField = computed(() => ['select', 'multi_select'].includes(selectedField.value?.type))
const fieldTypeLabel = computed(() => selectedField.value ? workOrderDesignerFieldType(selectedField.value.type, lang.value).label : '')
const fieldCountLabel = computed(() => lang.value === 'en' ? `${fields.value.length} ${fields.value.length === 1 ? 'field' : 'fields'}` : `${fields.value.length} 个字段`)
const labelError = computed(() => selectedField.value ? !String(selectedField.value.label || '').trim() || [...String(selectedField.value.label || '')].length > 128 : false)
const optionsError = computed(() => choiceField.value && normalizeWorkOrderDesignerOptions(selectedField.value?.optionsText ?? (selectedField.value?.options || []).join('\n')).length === 0)
const defaultError = computed(() => selectedField.value?.type === 'integer' && selectedField.value.default_value !== undefined && !Number.isInteger(selectedField.value.default_value))
const defaultInputType = computed(() => ({ number: 'number', integer: 'number', date: 'date', datetime: 'datetime-local' })[selectedField.value?.type] || 'text')
const defaultInputValue = computed(() => selectedField.value?.type === 'datetime' ? dateTimeLocalValue(selectedField.value.default_value) : (selectedField.value?.default_value ?? ''))

const copy = {
  zh: { palette: '字段组件', dragHint: '点击添加，或拖拽到画布', preview: '表单实时预览', previewHint: '字段顺序和样式与实际填单保持一致', fields: '个字段', properties: '字段属性', selectedHint: '修改后立即更新预览', selectHint: '选择画布中的字段后配置属性', emptyTitle: '从左侧添加第一个字段', emptyHint: '点击字段组件即可添加，也可以直接拖拽到这里', dropHere: '放到这里', dropEnd: '拖到此处追加字段', delete: '删除字段', dragField: '拖动调整字段顺序', insertAfter: '在后面添加字段', label: '字段名称', labelPlaceholder: '请输入字段名称', labelError: '字段名称必填，且不能超过 128 个字符。', type: '字段类型', identifier: '字段标识', identifierHint: '发布后作为数据键使用，为保证兼容性不可直接修改。', options: '选项', optionsHint: '每行一个选项，按 Enter 换行', optionsError: '至少填写一个有效选项。', defaultValue: '默认值（可选）', integerDefaultError: '整数默认值不能包含小数。', notSet: '不设置', yes: '是', no: '否', required: '必填', requiredHint: '创建工单时必须填写', moveUp: '上移', moveDown: '下移', deleteField: '删除此字段' },
  en: { palette: 'Field components', dragHint: 'Click to add, or drag onto the canvas', preview: 'Live form preview', previewHint: 'Field order and appearance match the actual form', fields: 'fields', properties: 'Field properties', selectedHint: 'Changes update the preview immediately', selectHint: 'Select a field on the canvas to edit its properties', emptyTitle: 'Add the first field from the left', emptyHint: 'Click a field component or drag it here', dropHere: 'Drop here', dropEnd: 'Drop here to append', delete: 'Delete field', dragField: 'Drag to reorder this field', insertAfter: 'Add field after', label: 'Field label', labelPlaceholder: 'Enter a field label', labelError: 'A field label is required and must not exceed 128 characters.', type: 'Field type', identifier: 'Field key', identifierHint: 'Used as the persisted data key and kept stable for compatibility.', options: 'Options', optionsHint: 'One option per line', optionsError: 'Enter at least one valid option.', defaultValue: 'Default value (optional)', integerDefaultError: 'An integer default cannot contain a decimal value.', notSet: 'Not set', yes: 'Yes', no: 'No', required: 'Required', requiredHint: 'Must be completed when the work order is created', moveUp: 'Move up', moveDown: 'Move down', deleteField: 'Delete this field' }
}
const text = key => copy[lang.value][key] || key

watch(fields, value => { if (!value.length) selectedIndex.value = -1; else if (selectedIndex.value < 0 || selectedIndex.value >= value.length) selectedIndex.value = 0 }, { immediate: true })

function emitFields(next) { emit('update:modelValue', { ...(props.modelValue || {}), fields: next }) }
function propertyInputId(suffix) { return `designer-${suffix}-${selectedField.value?.key || 'field'}` }
function uniqueKey(type) { const used = new Set(fields.value.map(field => field.key)); let key; do { key = `${type}_${Math.random().toString(36).slice(2, 8)}` } while (used.has(key)); return key }
function addField(type = 'text', index = fields.value.length) { if (props.readonly) return; const field = createWorkOrderDesignerField(type, uniqueKey(type), lang.value); emitFields(insertWorkOrderDesignerField(fields.value, field, index)); selectedIndex.value = index }
function removeField(index) { if (props.readonly || index < 0) return; const next = [...fields.value]; next.splice(index, 1); emitFields(next); selectedIndex.value = Math.min(index, next.length - 1) }
function replaceSelected(field) { if (!selectedField.value || props.readonly) return; const next = [...fields.value]; next[selectedIndex.value] = field; emitFields(next) }
function updateSelected(patch) { replaceSelected({ ...selectedField.value, ...patch }) }
function changeSelectedType(type) { replaceSelected(changeWorkOrderDesignerFieldType(selectedField.value, type, lang.value)) }
function updateOptions(value) { const options = normalizeWorkOrderDesignerOptions(value); const next = { ...selectedField.value, optionsText: value, options }; if (next.default_value !== undefined && !options.includes(next.default_value)) delete next.default_value; replaceSelected(next) }
function updateDefault(value) {
  if (value === '') { const next = { ...selectedField.value }; delete next.default_value; replaceSelected(next); return }
  let nextValue = value
  if (['number', 'integer'].includes(selectedField.value.type)) nextValue = Number(value)
  else if (selectedField.value.type === 'boolean') nextValue = value === 'true'
  else if (selectedField.value.type === 'datetime') nextValue = new Date(value).toISOString()
  updateSelected({ default_value: nextValue })
}
function moveSelected(direction) { const target = selectedIndex.value + direction; emitFields(reorderWorkOrderDesignerFields(fields.value, selectedIndex.value, target)); selectedIndex.value = target }
function dateTimeLocalValue(value) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 16)
}
function readDragPayload(event) {
  if (dragPayload.value) return dragPayload.value
  const encoded = event.dataTransfer?.getData('application/x-noyo-work-order-field')
  if (encoded) { try { return JSON.parse(encoded) } catch { return null } }
  const plain = event.dataTransfer?.getData('text/plain') || ''
  if (plain.startsWith('palette:')) return { kind: 'palette', type: plain.slice(8) }
  if (plain.startsWith('field:')) return { kind: 'field', index: Number(plain.slice(6)) }
  return null
}
function applyDrop(payload, index) {
  if (!payload) return
  if (payload.kind === 'palette') { addField(payload.type, index); return }
  const target = workOrderDesignerDropTarget(payload.index, index, fields.value.length)
  if (target < 0) return
  emitFields(reorderWorkOrderDesignerFields(fields.value, payload.index, target))
  selectedIndex.value = target
}
function resetDragState() { dragPayload.value = null; pointerDrag.value = null; dragging.value = false; activeDropIndex.value = -1 }
function dropAt(event, index) { if (props.readonly) return; event.preventDefault(); applyDrop(readDragPayload(event), index); resetDragState() }
function markDropTarget(index) { if (dragging.value) activeDropIndex.value = index }
function onPaletteClick(event, type) { if (suppressPaletteClick) { event.preventDefault(); suppressPaletteClick = false; return } addField(type) }
function startPointerDrag(event, payload) {
  if (props.readonly || (event.button !== undefined && event.button !== 0)) return
  pointerDrag.value = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, payload, moved: false }
}
function pointerDropIndex(clientX, clientY) {
  const target = document.elementFromPoint(clientX, clientY)?.closest?.('[data-form-drop-index]')
  if (!target) return -1
  const index = Number(target.dataset.formDropIndex)
  return Number.isInteger(index) ? index : -1
}
function onPointerMove(event) {
  const state = pointerDrag.value
  if (!state || state.pointerId !== event.pointerId) return
  if (!state.moved && Math.hypot(event.clientX - state.startX, event.clientY - state.startY) < 6) return
  state.moved = true
  dragging.value = true
  dragPayload.value = state.payload
  activeDropIndex.value = pointerDropIndex(event.clientX, event.clientY)
  if (event.cancelable) event.preventDefault()
}
function onPointerUp(event) {
  const state = pointerDrag.value
  if (!state || state.pointerId !== event.pointerId) return
  const dropIndex = pointerDropIndex(event.clientX, event.clientY)
  if (state.moved && dropIndex >= 0) applyDrop(state.payload, dropIndex)
  if (state.moved && state.payload.kind === 'palette') { suppressPaletteClick = true; setTimeout(() => { suppressPaletteClick = false }, 0) }
  resetDragState()
}
function onPointerCancel(event) { if (pointerDrag.value?.pointerId === event.pointerId) resetDragState() }

onMounted(() => {
  window.addEventListener('pointermove', onPointerMove, { passive: false })
  window.addEventListener('pointerup', onPointerUp)
  window.addEventListener('pointercancel', onPointerCancel)
})
onBeforeUnmount(() => {
  window.removeEventListener('pointermove', onPointerMove)
  window.removeEventListener('pointerup', onPointerUp)
  window.removeEventListener('pointercancel', onPointerCancel)
})
</script>

<style scoped>
.form-designer { --designer-panel-bg: var(--bs-body-bg); --designer-raised-bg: var(--bs-tertiary-bg); --designer-canvas-bg: color-mix(in srgb, var(--bs-secondary) 8%, var(--bs-body-bg)); --designer-paper-bg: var(--bs-body-bg); display: grid; grid-template-columns: 16rem minmax(30rem, 1fr) 21rem; min-height: 0; height: 100%; background: var(--designer-canvas-bg); color: var(--bs-body-color); }
.form-designer__palette, .form-designer__properties, .form-designer__canvas { scrollbar-color: color-mix(in srgb, var(--bs-secondary) 45%, transparent) transparent; scrollbar-width: thin; }
.form-designer__palette, .form-designer__properties { min-width: 0; overflow: auto; background: var(--designer-panel-bg); }
.form-designer__palette { border-right: 1px solid var(--bs-border-color); padding: 1rem; }
.form-designer__properties { border-left: 1px solid var(--bs-border-color); }
.form-designer__panel-head { display: flex; flex-direction: column; gap: .3rem; padding-bottom: .9rem; border-bottom: 1px solid var(--bs-border-color); }
.form-designer__panel-head strong { font-size: .95rem; }
.form-designer__panel-head span { color: var(--bs-secondary-color); font-size: .8rem; line-height: 1.4; }
.form-designer__palette-group { margin-top: 1rem; }
.form-designer__palette-group h6 { margin-bottom: .6rem; color: var(--bs-secondary-color); font-size: .75rem; font-weight: 700; letter-spacing: .04em; text-transform: uppercase; }
.form-designer__palette-grid { display: grid; grid-template-columns: 1fr; gap: .45rem; }
.form-designer__palette-item { display: grid; grid-template-columns: 1.4rem minmax(0, 1fr) auto; align-items: center; gap: .5rem; min-height: 2.8rem; padding: .6rem .7rem; border: 1px solid var(--bs-border-color); border-radius: .5rem; background: var(--designer-raised-bg); color: var(--bs-body-color); text-align: left; touch-action: none; transition: border-color .15s ease, background-color .15s ease, transform .15s ease; }
.form-designer__palette-item span { min-width: 0; font-size: .82rem; font-weight: 550; line-height: 1.3; white-space: normal; }
.form-designer__palette-item > .bi:first-child { color: var(--bs-primary); font-size: 1rem; }
.form-designer__palette-item:hover:not(:disabled) { border-color: var(--bs-primary); background: color-mix(in srgb, var(--bs-primary) 6%, var(--bs-body-bg)); transform: translateY(-1px); }
.form-designer__grip { color: var(--bs-secondary-color); opacity: .55; }
.form-designer__canvas { min-width: 0; overflow: auto; padding: 1rem clamp(1rem, 3vw, 3rem) 2rem; }
.form-designer__canvas-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; margin: 0 auto .8rem; max-width: 48rem; }
.form-designer__canvas-head div { display: flex; flex-direction: column; }
.form-designer__canvas-head div span { color: var(--bs-secondary-color); font-size: .75rem; }
.form-designer__count { flex: 0 0 auto; padding: .22rem .55rem; border: 1px solid var(--bs-border-color); border-radius: 999px; background: var(--designer-panel-bg); color: var(--bs-body-color); font-size: .72rem; font-weight: 600; }
.form-designer__paper { min-height: calc(100% - 3rem); max-width: 48rem; margin: auto; padding: 1.25rem clamp(1rem, 3vw, 2.25rem) 2rem; border: 1px solid var(--bs-border-color); border-radius: .85rem; background: var(--designer-paper-bg); box-shadow: 0 .8rem 2rem rgba(15, 23, 42, .08); }
.form-designer__empty { display: flex; min-height: 20rem; flex-direction: column; align-items: center; justify-content: center; gap: .55rem; border: 2px dashed var(--bs-border-color); border-radius: .7rem; color: var(--bs-secondary-color); text-align: center; }
.form-designer__empty i { color: var(--bs-primary); font-size: 2rem; }
.form-designer__empty span { max-width: 18rem; font-size: .8rem; }
.form-designer__drop-zone { position: relative; display: grid; min-height: 1.35rem; place-items: center; border: 1px dashed transparent; border-radius: .45rem; color: var(--bs-secondary-color); transition: min-height .15s ease, border-color .15s ease, background-color .15s ease; }
.form-designer__drop-zone::before { position: absolute; right: .75rem; left: .75rem; height: 1px; background: var(--bs-border-color); content: ''; opacity: .45; }
.form-designer__drop-zone span { position: relative; z-index: 1; padding: .12rem .5rem; border: 1px solid var(--bs-border-color); border-radius: 1rem; background: var(--designer-paper-bg); color: var(--bs-secondary-color); font-size: .68rem; line-height: 1.2; opacity: 0; }
.form-designer--dragging .form-designer__drop-zone { min-height: 2rem; }
.form-designer--dragging .form-designer__drop-zone span { opacity: 1; }
.form-designer__drop-target--active { border-color: var(--bs-primary) !important; background: color-mix(in srgb, var(--bs-primary) 10%, transparent) !important; }
.form-designer__drop-target--active::before { opacity: 0; }
.form-designer__drop-target--active span { border-color: var(--bs-primary); background: var(--bs-primary); color: white; opacity: 1; }
.form-designer__drop-zone--last { min-height: 2rem; }
.form-designer__field { position: relative; padding: 1rem 2.35rem 1rem 1.8rem; border: 1px solid var(--bs-border-color-translucent); border-radius: .65rem; background: color-mix(in srgb, var(--designer-raised-bg) 45%, transparent); cursor: pointer; transition: border-color .15s ease, background-color .15s ease, box-shadow .15s ease; }
.form-designer__field:hover, .form-designer__field:focus-within { border-color: var(--bs-border-color); background: var(--bs-tertiary-bg); }
.form-designer__field:focus-visible { outline: 2px solid var(--bs-primary); outline-offset: 2px; }
.form-designer__field--selected { border-color: var(--bs-primary); background: color-mix(in srgb, var(--bs-primary) 5%, var(--bs-body-bg)); box-shadow: 0 0 0 2px color-mix(in srgb, var(--bs-primary) 12%, transparent); }
.form-designer__field--drag-source { opacity: .55; }
.form-designer__field-handle { position: absolute; z-index: 2; top: 50%; left: .12rem; display: grid; width: 1.75rem; height: 2.25rem; place-items: center; border: 0; border-radius: .35rem; background: transparent; color: var(--bs-secondary-color); cursor: grab; opacity: .55; touch-action: none; transform: translateY(-50%); transition: opacity .15s ease, background-color .15s ease; }
.form-designer__field-handle:active { cursor: grabbing; }
.form-designer__field:hover .form-designer__field-handle, .form-designer__field:focus-within .form-designer__field-handle, .form-designer__field--selected .form-designer__field-handle { opacity: .9; }
.form-designer__field-handle:hover, .form-designer__field-handle:focus-visible { background: color-mix(in srgb, var(--bs-primary) 12%, transparent); color: var(--bs-primary); }
.form-designer__field-delete { position: absolute; z-index: 2; top: .3rem; right: .3rem; display: grid; width: 2rem; height: 2rem; place-items: center; border: 0; border-radius: 50%; background: transparent; color: var(--bs-danger); opacity: .55; transition: opacity .15s ease, background-color .15s ease, color .15s ease; }
.form-designer__field:hover .form-designer__field-delete, .form-designer__field:focus-within .form-designer__field-delete, .form-designer__field--selected .form-designer__field-delete { opacity: 1; }
.form-designer__field-delete:hover, .form-designer__field-delete:focus-visible { background: var(--bs-danger); color: white; }
.form-designer__insert { display: flex; align-items: center; gap: .3rem; margin: .65rem auto -.35rem; border: 0; background: transparent; color: var(--bs-primary); font-size: .75rem; opacity: 0; transition: opacity .15s ease; }
.form-designer__field:hover .form-designer__insert, .form-designer__field--selected .form-designer__insert { opacity: 1; }
.form-designer__property-body { padding: 1rem; }
.form-designer__property-body .form-label { margin-bottom: .45rem; font-size: .82rem; font-weight: 650; }
.form-designer__property-body .form-text { font-size: .78rem; line-height: 1.45; }
.form-designer__properties > .form-designer__panel-head { padding: 1rem; }
.form-designer__switch-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding: .8rem 0; border-block: 1px solid var(--bs-border-color); }
.form-designer__switch-row > label { display: flex; flex-direction: column; cursor: pointer; }
.form-designer__switch-row span { color: var(--bs-secondary-color); font-size: .75rem; }
.form-designer__properties .form-control:disabled { background-color: var(--bs-tertiary-bg); color: var(--bs-secondary-color); opacity: 1; }
.form-designer__property-empty { display: flex; min-height: 18rem; flex-direction: column; align-items: center; justify-content: center; gap: .6rem; padding: 1rem; color: var(--bs-secondary-color); text-align: center; }
.form-designer__property-empty i { font-size: 1.75rem; }
.form-designer--readonly .form-designer__palette-item, .form-designer--readonly .form-designer__field { cursor: default; }
:global([data-bs-theme='dark']) .form-designer { --designer-panel-bg: #181c24; --designer-raised-bg: #202631; --designer-canvas-bg: #262c36; --designer-paper-bg: #171b23; }
:global([data-bs-theme='dark']) .form-designer__paper { box-shadow: 0 .8rem 2rem rgba(0, 0, 0, .32); }
@media (max-width: 1199px) { .form-designer { grid-template-columns: 13rem minmax(25rem, 1fr) 18rem; } }
@media (max-width: 991px) { .form-designer { grid-template-columns: 1fr; height: auto; overflow: auto; } .form-designer__palette, .form-designer__properties { overflow: visible; border: 0; border-bottom: 1px solid var(--bs-border-color); } .form-designer__palette-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); } .form-designer__canvas { overflow: visible; } }
</style>
