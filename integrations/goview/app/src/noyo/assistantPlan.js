// Compile a declarative AI proposal against a detached editor snapshot. Nothing
// reaches the live canvas until the host has authorized the complete candidate.
//
// Two proposal shapes are accepted:
//  - `sections`: a semantic blueprint. A deterministic layout compiler turns it
//    into exact rectangles, picks widgets and derives bindings, so the model
//    cannot produce overlapping or off-canvas geometry.
//  - `operations`: direct component edits, used for incremental adjustments to
//    an existing screen.
import { preflightDocument } from './preflight'
import { screenPalette } from './screenTheme'

const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key)
const clone = value => JSON.parse(JSON.stringify(value))
const object = value => value !== null && typeof value === 'object' && !Array.isArray(value)
const fail = () => { throw new Error('INVALID_PLAN') }

function checkJSON(value, depth = 0) {
  if (depth > 32) fail()
  if (typeof value === 'number' && !Number.isFinite(value)) fail()
  if (value !== null && typeof value === 'object') {
    for (const [key, item] of Object.entries(value)) {
      if (['__proto__', 'prototype', 'constructor'].includes(key)) fail()
      checkJSON(item, depth + 1)
    }
  } else if (!['string', 'number', 'boolean'].includes(typeof value) && value !== null) fail()
}

function merge(base, patch) {
  const result = object(base) ? clone(base) : {}
  for (const [key, value] of Object.entries(patch)) result[key] = object(value) ? merge(result[key], value) : clone(value)
  return result
}

function unbind(document, componentId) {
  const removed = document.targets.filter(target => target.componentId === componentId)
  document.targets = document.targets.filter(target => target.componentId !== componentId)
  for (const target of removed) {
    if (!document.targets.some(item => item.bindingId === target.bindingId)) delete document.bindings[target.bindingId]
  }
}

function geometry(attr, width, height) {
  if (!attr || !['x', 'y', 'w', 'h'].every(key => typeof attr[key] === 'number' && Number.isFinite(attr[key]))) fail()
  if (attr.x < 0 || attr.y < 0 || attr.w <= 0 || attr.h <= 0 || attr.x + attr.w > width || attr.y + attr.h > height) fail()
}

function locate(items, id) {
  for (const component of items) {
    if (component.id === id) return { component, items }
    if (component.isGroup && Array.isArray(component.groupList)) {
      const result = locate(component.groupList, id)
      if (result) return result
    }
  }
  return null
}

function descendants(component) {
  return [component, ...(component.isGroup ? component.groupList.flatMap(descendants) : [])]
}

function flatten(items = []) {
  return items.flatMap(item => descendants(item))
}

// The server rejects any binding without a device set: the runtime filters rows by
// that set, so an empty one is meaningless — and a 400 on validate discards the
// whole proposal. Dropping only the broken binding keeps the rest of the screen
// applicable, and the note tells the reviewer which widget lost its data.
// Intent narrows the compiler's write scope. It never grants authorization to
// query data; that remains the host's job. Ambiguous requests stay mixed.
export function inferAssistantIntent(prompt = '', plan = {}) {
  const text = String(prompt).toLowerCase()
    .replace(/(?:保持|保留|不修改|不改变|不更改)[^，。；\n]*(?:绑定|数据源|指标)[^，。；\n]*/g, '')
    .replace(/(?:keep|preserve|retain)[^.;\n]*(?:binding|data source|metric)[^.;\n]*/g, '')
  if (/(?:新增|创建|重新生成|删除|重做|绑定|换数据源|接入数据|统计口径)|\b(?:create|rebuild|regenerate|delete|bind|rebind|query)\b/.test(text)) return 'mixed'
  if (/(?:布局|排版|位置|间距|对齐)|\b(?:layout|position|spacing|alignment)\b/.test(text)) return 'layout'
  if (/(?:文案|文字|说明|标注|颜色|字体|样式)|\b(?:copy|wording|note|label|colou?r|font|style)\b/.test(text)) return 'copy_style'
  return !text.trim() && ['copy', 'style', 'copy_style', 'layout', 'binding', 'mixed'].includes(plan?.intent) ? plan.intent : 'mixed'
}

// Compiled blueprints must not silently overlap. Incremental edits are exempt:
// decorations and grouped layers are legitimately stacked.
function verifyLayout(componentList) {
  const boxes = flatten(componentList).filter(item => !item.isGroup && item.chartConfig?.key !== 'Border06').map(item => ({ id: item.id, ...item.attr }))
  for (let i = 0; i < boxes.length; i++) {
    for (let j = i + 1; j < boxes.length; j++) {
      const a = boxes[i], b = boxes[j]
      const overlapW = Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x)
      const overlapH = Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y)
      if (overlapW > 2 && overlapH > 2) throw new Error('LAYOUT_OVERLAP')
    }
  }
}

export function compileAssistantPlan(source, plan, { createComponent, getWidget, locale, applyTheme, expandBlueprint, intent = 'mixed' }) {
  checkJSON(plan)
  if (!object(plan) || plan.clarification) fail()
  const preservesBindings = ['copy', 'style', 'copy_style', 'layout'].includes(intent)
  const copyStyleOnly = ['copy', 'style', 'copy_style'].includes(intent)
  const scopeViolation = () => { throw new Error('INTENT_SCOPE_VIOLATION') }
  if (preservesBindings && plan.sections != null || copyStyleOnly && plan.canvas != null) scopeViolation()
  const document = clone(source)
  document.bindings ||= {}
  document.targets ||= []
  const content = document.content
  if (!object(content) || !Array.isArray(content.componentList)) fail()
  const config = content.editCanvasConfig ||= {}
  const width = plan.canvas?.width ?? config.width ?? document.canvas?.width ?? 1920
  const height = plan.canvas?.height ?? config.height ?? document.canvas?.height ?? 1080
  if (![width, height].every(value => Number.isInteger(value) && value >= 64 && value <= 16384)) fail()
  if (plan.canvas) {
    if (!object(plan.canvas) || Object.keys(plan.canvas).some(key => !['width', 'height'].includes(key))) fail()
    document.canvas = { ...document.canvas, width, height }
    Object.assign(config, { width, height })
  }

  let operations = plan.operations
  let notes = []
  let compiled = false
  if (plan.sections != null) {
    if (!Array.isArray(plan.sections) || plan.sections.length > 40 || !expandBlueprint) fail()
    if (Array.isArray(plan.operations) && plan.operations.length) fail()
    const taken = new Set(flatten(content.componentList).map(item => item.id).filter(Boolean))
    const compiledPlan = expandBlueprint(plan.sections, { width, height }, { taken, locale, composition: plan.composition })
    // A blueprint describes a complete replacement. Remove the old top-level
    // layers in this detached candidate so the new grid cannot collide with
    // the previous screen. Normal remove handling also clears child bindings
    // and marks the proposal destructive, requiring confirmation before apply.
    operations = [
      ...content.componentList.map(component => ({ type: 'remove', id: component.id })),
      ...compiledPlan.operations
    ]
    notes = Array.isArray(compiledPlan.notes) ? compiledPlan.notes : []
    compiled = true
    // Preserve an explicitly chosen canvas and derive every widget's theme from
    // it. A new canvas gets a coherent screen palette rather than shell defaults.
    config.background ||= screenPalette.canvas
    config.selectColor = true
    config.backgroundImage = ''
  }
  if (!Array.isArray(operations) || !operations.length || operations.length > (compiled ? 400 : 200)) fail()

  const ids = new Set(), changes = []
  for (const operation of operations) {
    if (!object(operation) || !/^[A-Za-z0-9][A-Za-z0-9_.:\-]{0,127}$/.test(operation.id) || ids.has(operation.id)) fail()
    if (Object.keys(operation).some(key => !['type', 'id', 'key', 'x', 'y', 'w', 'h', 'title', 'option', 'binding', 'theme'].includes(key))) fail()
    if (preservesBindings && (operation.type !== 'update' || own(operation, 'binding'))) scopeViolation()
    if (copyStyleOnly && ['x', 'y', 'w', 'h'].some(key => own(operation, key))) scopeViolation()
    if (preservesBindings && object(operation.option) && own(operation.option, 'dataset') && document.targets.some(target => target.componentId === operation.id)) scopeViolation()
    ids.add(operation.id)
    const location = locate(content.componentList, operation.id)
    let component = location?.component
    const before = component ? clone(component) : null
    if (operation.type === 'add') {
      if (component || !getWidget(operation.key)) fail()
      component = createComponent(operation.key, locale)
      component.id = operation.id
      Object.assign(component.attr, { x: 24, y: 24, w: 320, h: 200 })
      if (operation.theme && applyTheme) applyTheme(component, locale, { ...operation.theme, canvasBackground: config.background })
      if (operation.theme?.option) {
        if (!object(operation.theme.option)) fail()
        component.option = merge(component.option, operation.theme.option)
      }
      content.componentList.push(component)
    } else if (!component || !['update', 'remove'].includes(operation.type) || own(operation, 'key')) fail()
    if (operation.type === 'remove') {
      if (Object.keys(operation).some(key => !['type', 'id'].includes(key))) fail()
      location.items.splice(location.items.indexOf(component), 1)
      for (const child of descendants(component)) unbind(document, child.id)
    } else {
      const spec = getWidget(component.chartConfig.key)
      if (!spec) fail()
      for (const key of ['x', 'y', 'w', 'h']) if (own(operation, key)) component.attr[key] = operation[key]
      geometry(component.attr, width, height)
      if (own(operation, 'title')) {
        if (typeof operation.title !== 'string' || operation.title.length > 200) fail()
        component.chartConfig.title = operation.title
      }
      if (own(operation, 'option')) {
        if (!object(operation.option)) fail()
        component.option = merge(component.option, operation.option)
      }
      if (own(operation, 'binding')) {
        if (operation.binding !== null && (!object(operation.binding) || !spec.target || (operation.binding.kind !== 'dataset' && !spec.kinds.includes(operation.binding.kind)))) fail()
        unbind(document, component.id)
        if (operation.binding !== null) {
          const stem = `ai-${component.id}`.slice(0, 112)
          let bindingId = stem, suffix = 0
          while (own(document.bindings, bindingId)) bindingId = `${stem}-${++suffix}`
          document.bindings[bindingId] = clone(operation.binding)
          document.targets.push({ componentId: component.id, bindingId, target: spec.target })
          if (operation.binding.kind === 'eventList' && Array.isArray(component.option.header)) {
            component.option.header = locale.startsWith('en') ? ['Time', 'Device', 'Event', 'Type', 'Parameters'] : ['时间', '设备', '事件', '类型', '参数']
            component.option.align = ['left', 'left', 'left', 'center', 'left']
          }
          if (['metric.value', 'progress.value'].includes(spec.target)) component.option.dataset = null
        }
      }
    }
    const { theme: _theme, ...rest } = operation
    changes.push({ ...clone(rest), key: component.chartConfig.key, title: component.chartConfig.title, before: before?.attr, after: operation.type === 'remove' ? null : clone(component.attr) })
  }
  if (flatten(content.componentList).length > 200) fail()
  if (plan.canvas) for (const component of flatten(content.componentList)) geometry(component.attr, width, height)
  if (compiled) {
    // Binding errors must reach structured validation. Silently deleting a
    // contract here hides an invalid proposal and can discard existing wiring.
    verifyLayout(content.componentList)
    // A compiled screen is fully machine-authored, so it gets the full check. A
    // hand-edited screen is exempt: stacked decorations are legitimate there.
    const report = preflightDocument(document, { locale })
    if (report.errors.length) throw new Error(report.errors[0])
    notes.push(...report.warnings)
  }
  if (preservesBindings && (JSON.stringify(document.bindings) !== JSON.stringify(source.bindings || {}) || JSON.stringify(document.targets) !== JSON.stringify(source.targets || []))) scopeViolation()
  return { document, changes, notes, destructive: changes.some(item => item.type === 'remove' || own(item, 'binding') && item.binding === null) }
}
