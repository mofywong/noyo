<template>
  <div class="var-picker-popover shadow border rounded bg-white" :style="popoverStyle" @click.stop>
    <div class="var-picker-header border-bottom" @pointerdown="startDrag">
      <div class="var-picker-drag-title">
        <div class="fw-bold">插入引用</div>
        <div class="text-muted small">选择常用运行时变量</div>
      </div>
      <div class="var-picker-header-actions">
        <div class="var-picker-search">
          <i class="bi bi-search"></i>
          <input v-model.trim="keyword" type="search" placeholder="搜索字段">
        </div>
        <button type="button" class="btn btn-sm var-picker-pin" :class="pinned ? 'btn-primary' : 'btn-outline-secondary'" :title="pinned ? '取消顶置' : '顶置'" @click.stop="togglePinned">
          <i :class="['bi', pinned ? 'bi-pin-angle-fill' : 'bi-pin-angle']"></i>
        </button>
        <button type="button" class="btn btn-sm btn-outline-secondary var-picker-close" title="关闭" @click.stop="close">
          <i class="bi bi-x-lg"></i>
        </button>
      </div>
    </div>
    <div class="var-picker-layout">
      <div class="picker-pane border-end p-2 overflow-auto">
        <div class="fw-bold text-muted small mb-2 px-1">来源</div>
        <button
          v-for="group in groups"
          :key="group.key"
          type="button"
          class="picker-item"
          :class="{ active: activeGroupKey === group.key }"
          @click="selectGroup(group.key)"
        >
          <span><i :class="['bi', group.icon, 'me-1']"></i>{{ group.label }}</span>
          <small>{{ group.nodes.length }} 个节点</small>
        </button>
      </div>

      <div class="picker-pane border-end p-2 overflow-auto">
        <div class="fw-bold text-muted small mb-2 px-1">节点</div>
        <button
          v-for="node in activeNodes"
          :key="node.key"
          type="button"
          class="picker-item"
          :class="{ active: activeNodeKey === node.key }"
          :title="node.detail || node.label"
          @mouseenter="highlightNode(node)"
          @mouseleave="clearHighlight"
          @click="selectNode(node)"
        >
          <span class="text-truncate">{{ node.label }}</span>
          <small v-if="node.detail" class="text-muted text-truncate">{{ node.detail }}</small>
        </button>
        <div v-if="!activeNodes.length" class="small text-muted px-1">暂无可用节点</div>
      </div>

      <div class="variable-pane p-2 overflow-auto">
        <template v-for="section in visibleSections" :key="section.title">
          <div class="var-section-title">{{ section.title }}</div>
          <button
            v-for="item in section.items"
            :key="`${section.title}-${item.value}-${item.label}`"
            type="button"
            class="var-item"
            @click="insert(item.value)"
          >
            <span class="var-item-label">
              <span>{{ item.label }}</span>
              <small v-if="item.description" class="text-muted">{{ item.description }}</small>
            </span>
            <code class="small text-muted">{{ item.value }}</code>
          </button>
        </template>
        <div v-if="!visibleSections.length" class="small text-muted px-1">没有匹配的可插入项</div>
        <div class="mt-3 small text-muted px-1">
          <i class="bi bi-info-circle me-1"></i>只显示常用且后端会填充的运行时变量。
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, defineComponent, inject, onUnmounted, ref, watch } from 'vue'

export default defineComponent({
  name: 'VarPickerPopover',
  props: {
    positionStyle: { type: Object, default: () => ({}) }
  },
  emits: ['select', 'close'],
  setup(props, { emit }) {
    const activeGroupKey = ref('trigger')
    const activeNodeKey = ref('')
    const keyword = ref('')
    const popoverStyle = ref({})
    const dragging = ref(false)
    const dragOffset = ref({ x: 0, y: 0 })
    const hasDragged = ref(false)
    const pinned = ref(false)
    const ruleContext = inject('ruleContext', null)

    const context = ref(ruleContext?.getContext
      ? ruleContext.getContext()
      : { triggers: [], conditions: [], actions: [], devices: [], currentNode: null, rule: null, effectiveTime: null })

    const groups = computed(() => buildGroups(context.value))

    const activeGroup = computed(() => groups.value.find(group => group.key === activeGroupKey.value) || groups.value[0])
    const activeNodes = computed(() => activeGroup.value?.nodes || [])
    const activeNode = computed(() => activeNodes.value.find(node => node.key === activeNodeKey.value) || activeNodes.value[0])
    const activeSections = computed(() => activeNode.value?.sections || [])
    const visibleSections = computed(() => filterSections(activeSections.value, keyword.value))

    const selectGroup = (key) => {
      activeGroupKey.value = key
      activeNodeKey.value = ''
      keyword.value = ''
      clearHighlight()
    }
    const selectNode = (node) => {
      activeNodeKey.value = node.key
      highlightNode(node, true)
    }
    const highlightNode = (node, scroll = false) => {
      if (node?.graphNodeId && ruleContext?.highlightNode) ruleContext.highlightNode(node.graphNodeId, scroll)
    }
    const clearHighlight = () => {
      if (ruleContext?.clearHighlight) ruleContext.clearHighlight()
    }
    const insert = (val) => emit('select', val)
    const close = () => {
      clearHighlight()
      emit('close')
    }
    const togglePinned = () => {
      pinned.value = !pinned.value
      if (pinned.value) {
        hasDragged.value = true
      } else {
        hasDragged.value = false
        popoverStyle.value = { ...(props.positionStyle || {}) }
      }
    }
    const clampPosition = (left, top, width, height) => {
      const margin = 8
      return {
        left: `${Math.min(Math.max(left, margin), Math.max(margin, window.innerWidth - width - margin))}px`,
        top: `${Math.min(Math.max(top, margin), Math.max(margin, window.innerHeight - height - margin))}px`,
        width: `${width}px`,
        height: `${height}px`
      }
    }
    const startDrag = (event) => {
      if (event.target?.closest?.('input, button')) return
      const rect = event.currentTarget.closest('.var-picker-popover')?.getBoundingClientRect()
      if (!rect) return
      dragging.value = true
      hasDragged.value = true
      dragOffset.value = { x: event.clientX - rect.left, y: event.clientY - rect.top }
      document.addEventListener('pointermove', onDragMove)
      document.addEventListener('pointerup', stopDrag, { once: true })
      event.preventDefault()
    }
    const onDragMove = (event) => {
      if (!dragging.value) return
      const width = Number.parseFloat(popoverStyle.value.width) || 760
      const height = Number.parseFloat(popoverStyle.value.height) || 360
      popoverStyle.value = clampPosition(event.clientX - dragOffset.value.x, event.clientY - dragOffset.value.y, width, height)
    }
    const stopDrag = () => {
      dragging.value = false
      document.removeEventListener('pointermove', onDragMove)
    }

    watch(activeNodes, (nodes) => {
      if (!nodes.some(node => node.key === activeNodeKey.value)) {
        activeNodeKey.value = nodes[0]?.key || ''
      }
    }, { immediate: true })
    watch(() => props.positionStyle, (style) => {
      if (!hasDragged.value && !pinned.value) popoverStyle.value = { ...(style || {}) }
    }, { immediate: true, deep: true })
    onUnmounted(() => {
      stopDrag()
      clearHighlight()
    })

    return {
      groups,
      activeGroupKey,
      activeNodeKey,
      activeNodes,
      keyword,
      visibleSections,
      popoverStyle,
      selectGroup,
      selectNode,
      highlightNode,
      clearHighlight,
      startDrag,
      insert,
      close,
      pinned,
      togglePinned
    }
  }
})

function buildTriggerNodes(triggers, devices, nodeDisplay) {
  const configuredTriggers = triggers || []
  const nodes = [buildRuntimeTriggerNode(configuredTriggers, devices)]
  nodes.push(...configuredTriggers.map((trigger, index) => {
    const id = trigger.id || trigger._id || `trigger_${index}`
    const display = displayFor(nodeDisplay, id, triggerLabel(trigger, index), '')
    const device = findDevice(devices, trigger.deviceCode)
    const property = findByKey(device?.properties, trigger.propertyKey)
    const event = findByKey(device?.events, trigger.eventId)
    const triggerPath = `rule.triggers.${index}`
    const sections = [section('触发器配置', compact([
      variableItem('触发类型', `\${${triggerPath}.type}`),
      trigger.deviceCode ? variableItem('配置设备编码', `\${${triggerPath}.deviceCode}`) : null
    ]))]

    if (trigger.type === 'property_change') {
      const propKey = trigger.propertyKey || '<属性标识>'
      sections.push(section('属性触发配置', compact([
        variableItem('属性名称', '${trigger.propertyName}'),
        variableItem('属性标识', `\${${triggerPath}.propertyKey}`),
        variableItem('比较规则', `\${${triggerPath}.operator}`),
        variableItem('配置比较值', `\${${triggerPath}.value}`),
        variableItem(`${property?.name || propKey} 触发值`, '${trigger.triggerValue}')
      ])))
    }

    if (trigger.type === 'event') {
      sections.push(section('事件触发配置', compact([
        variableItem('事件名称', '${trigger.eventName}'),
        variableItem('事件标识', `\${${triggerPath}.eventId}`)
      ])))
    }

    if (trigger.type === 'device_status') {
      sections.push(section('状态触发配置', [
        variableItem('目标状态', `\${${triggerPath}.statusValue}`)
      ]))
    }

    if (trigger.type === 'cron') {
      sections.push(section('定时触发配置', compact([
        variableItem('cron 表达式', `\${${triggerPath}.cronExpr}`),
        variableItem('定时描述', `\${${triggerPath}.cronDesc}`)
      ])))
    }

    return {
      key: id,
      graphNodeId: id,
      label: display.label,
      detail: display.detail || device?.name || trigger.deviceCode || property?.name || event?.name || '',
      sections: normalizeSections(sections)
    }
  }))

  return nodes
}

function buildRuleNodes(rule, effectiveTime) {
  const et = effectiveTime || rule?.effective_time || rule?.effectiveTime || {}
  return [
    {
      key: 'rule_basic',
      label: '规则基础信息',
      detail: rule?.name || rule?.code || '当前规则',
      sections: normalizeSections([
        section('规则基础信息', compact([
          variableItem('规则名称', '${rule.name}'),
          variableItem('规则编码', '${rule.code}'),
          variableItem('规则描述', '${rule.description}'),
          variableItem('规则版本', '${rule.version}')
        ]))
      ])
    },
    {
      key: 'rule_effective_time',
      label: '规则生效时间',
      detail: effectiveTimeDetail(et),
      sections: normalizeSections([
        section('生效时间配置', compact([
          variableItem('生效模式', '${rule.effectiveTime.mode}'),
          variableItem('开始时间', '${rule.effectiveTime.windows.0.startTime}'),
          variableItem('结束时间', '${rule.effectiveTime.windows.0.endTime}'),
          variableItem('星期配置', '${rule.effectiveTime.weekdays}'),
          variableItem('月份日期', '${rule.effectiveTime.monthDays}'),
          variableItem('月份配置', '${rule.effectiveTime.months}'),
          variableItem('时区', '${rule.effectiveTime.timezone}')
        ]))
      ])
    }
  ]
}

function buildRuntimeTriggerNode(triggers, devices) {
  const hasDeviceTrigger = (triggers || []).some(trigger => trigger.deviceCode)
  const hasPropertyTrigger = (triggers || []).some(trigger => trigger.type === 'property_change')
  const hasEventTrigger = (triggers || []).some(trigger => trigger.type === 'event')
  const hasStatusTrigger = (triggers || []).some(trigger => trigger.type === 'device_status')
  const hasCronTrigger = (triggers || []).some(trigger => trigger.type === 'cron')
  const eventParamItems = collectEventParamItems(triggers, devices)
  return {
    key: 'runtime_trigger',
    label: '触发节点',
    detail: '本次触发规则时产生的上下文',
    sections: normalizeSections([
      section('触发时间', [
        variableItem('触发时间', '${trigger.triggerTimeText}')
      ]),
      section('触发设备', compact([
        hasDeviceTrigger ? variableItem('设备名称', '${trigger.deviceName}') : null,
        hasDeviceTrigger ? variableItem('设备编码', '${trigger.deviceCode}') : null,
        (hasDeviceTrigger || hasStatusTrigger) ? variableItem('设备状态', '${trigger.deviceStatus}') : null
      ])),
      section('属性触发', compact([
        hasPropertyTrigger ? variableItem('属性名称', '${trigger.propertyName}') : null,
        hasPropertyTrigger ? variableItem('属性标识', '${trigger.propertyKey}') : null,
        hasPropertyTrigger ? variableItem('触发值', '${trigger.triggerValue}') : null
      ])),
      section('事件触发', compact([
        hasEventTrigger ? variableItem('事件时间', '${trigger.triggerTimeText}') : null,
        hasEventTrigger ? variableItem('事件名称', '${trigger.eventName}') : null,
        hasEventTrigger ? variableItem('事件标识', '${trigger.eventId}') : null,
        ...eventParamItems,
        hasEventTrigger && !eventParamItems.length ? variableItem('指定事件参数值', '${trigger.eventParams.<参数名>}') : null
      ])),
      section('定时触发', compact([
        hasCronTrigger ? variableItem('cron 表达式', '${trigger.cronExpr}') : null,
        hasCronTrigger ? variableItem('定时描述', '${trigger.cronDesc}') : null
      ]))
    ])
  }
}

function buildConditionNodes(conditions, devices, nodeDisplay) {
  return (conditions || []).map((condition, index) => {
    const id = condition.id || condition._id || `condition_${index}`
    const display = displayFor(nodeDisplay, id, conditionLabel(condition, index), '')
    const device = findDevice(devices, condition.deviceCode)
    const property = findByKey(device?.properties, condition.propertyKey)
    const conditionPath = `condition_values.${index}`
    const sections = [
      section('判断条件配置', compact([
        variableItem('条件类型', `\${${conditionPath}.type}`),
        variableItem('设备名称', `\${${conditionPath}.deviceName}`),
        variableItem('设备编码', `\${${conditionPath}.deviceCode}`),
        condition.type === 'property' ? variableItem('属性名称', `\${${conditionPath}.propertyName}`) : null,
        condition.type === 'property' ? variableItem('属性标识', `\${${conditionPath}.propertyKey}`) : null,
        condition.type === 'property' ? variableItem('比较规则', `\${${conditionPath}.operator}`) : null,
        condition.type === 'property' ? variableItem('配置比较值', `\${${conditionPath}.expected}`) : null,
        condition.type === 'device_status' ? variableItem('目标状态', `\${${conditionPath}.statusValue}`) : null
      ])),
      section('运行时设备', [
        variableItem('设备名称', `\${${conditionPath}.deviceName}`),
        variableItem('设备编码', `\${${conditionPath}.deviceCode}`),
        variableItem('设备状态', `\${${conditionPath}.deviceStatus}`)
      ]),
      section('判断结果', [
        variableItem('实际值', `\${${conditionPath}.actualValue}`),
        variableItem('是否匹配', `\${${conditionPath}.matched}`)
      ])
    ]

    if (condition.type === 'property') {
      const propKey = condition.propertyKey || '<属性标识>'
      sections.push(section('运行时属性', [
        variableItem(`${property?.name || propKey} 实时值`, `\${${conditionPath}.properties.${propKey}}`),
        variableItem(`${property?.name || propKey} 实际值`, `\${${conditionPath}.propertyValue}`),
        variableItem('属性名称', `\${${conditionPath}.propertyName}`),
        variableItem('属性标识', `\${${conditionPath}.propertyKey}`)
      ]))
    }

    return {
      key: id,
      graphNodeId: id,
      label: display.label,
      detail: display.detail || device?.name || condition.deviceCode || property?.name || '',
      sections: normalizeSections(sections)
    }
  })
}

function buildActionNodes(actions, currentNode, nodeDisplay) {
  const flat = flattenActions(actions)
  const availableIds = collectPreviousActionIds(actions, currentNode)
  return flat.filter(action => availableIds.has(actionId(action)) && actionCanExposeOutput(action)).map((action, index) => {
    const id = action.id || action._id || `action_${index}`
    const display = displayFor(nodeDisplay, id, actionLabel(action, index), action.type)
    return {
      key: id,
      graphNodeId: id,
      label: display.label,
      detail: display.detail || action.type,
      sections: [
        section('动作输出', compact([
          variableItem('动作输出', `\${node.${id}.data}`),
          action.type === 'alarm' ? variableItem('告警标题', `\${node.${id}.title}`) : null,
          action.type === 'alarm' ? variableItem('告警内容', `\${node.${id}.content}`) : null,
          action.type === 'alarm' ? variableItem('告警级别', `\${node.${id}.level}`) : null,
          action.type === 'alarm' ? variableItem('告警设备编码', `\${node.${id}.deviceCode}`) : null,
          variableItem('执行状态', `\${node.${id}.status}`),
          variableItem('动作错误信息', `\${node.${id}.error}`)
        ]))
      ]
    }
  })
}

function buildGroups(ctx) {
  const scope = referenceScope(ctx.currentNode)
  const groupMap = {
    rule: { key: 'rule', label: '规则信息', icon: 'bi-info-circle', nodes: buildRuleNodes(ctx.rule, ctx.effectiveTime) },
    trigger: { key: 'trigger', label: '触发节点', icon: 'bi-lightning-charge', nodes: buildTriggerNodes(ctx.triggers, ctx.devices, ctx.nodeDisplay) },
    condition: { key: 'condition', label: '判断条件', icon: 'bi-funnel', nodes: buildConditionNodes(ctx.conditions, ctx.devices, ctx.nodeDisplay) },
    action: { key: 'action', label: '上文动作', icon: 'bi-play-circle', nodes: buildActionNodes(ctx.actions, ctx.currentNode, ctx.nodeDisplay) }
  }
  return scope.map(key => groupMap[key]).filter(group => group && group.nodes.length)
}

function displayFor(nodeDisplay, id, fallbackLabel, fallbackDetail = '') {
  const display = nodeDisplay?.[id] || {}
  return {
    label: display.label || fallbackLabel,
    detail: display.detail || fallbackDetail
  }
}

function referenceScope(currentNode) {
  const kind = currentNode?._graphKind || ''
  const type = currentNode?.type || ''
  if (kind === 'condition' || kind === 'condition_group' || ['property'].includes(type)) {
    return ['rule', 'trigger']
  }
  if (kind === 'action' || isActionType(type)) {
    return ['rule', 'trigger', 'condition', 'action']
  }
  return []
}

function isActionType(type) {
  return [
    'set_property',
    'call_service',
    'notification',
    'alarm',
    'delay',
    'llm',
    'voice_playback',
    'parallel_group',
    'sequence_group'
  ].includes(type)
}

function actionCanExposeOutput(action) {
  return ['llm', 'alarm'].includes(action?.type)
}

function effectiveTimeDetail(et) {
  const mode = et?.mode || 'always'
  const firstWindow = Array.isArray(et?.windows) ? et.windows[0] : null
  const start = firstWindow?.startTime || et?.startTime || ''
  const end = firstWindow?.endTime || et?.endTime || ''
  return start || end ? `${mode} ${start || '-'} ~ ${end || '-'}` : mode
}

function collectEventParamItems(triggers, devices) {
  const seen = new Set()
  const items = []
  ;(triggers || []).forEach(trigger => {
    if (trigger.type !== 'event') return
    const event = findByKey(findDevice(devices, trigger.deviceCode)?.events, trigger.eventId)
    buildEventParamItems(event, 'trigger.eventParams').forEach(item => {
      if (!seen.has(item.value)) {
        seen.add(item.value)
        items.push(item)
      }
    })
  })
  return items
}

function buildEventParamItems(event, basePath) {
  return eventParams(event).map(param => {
    const key = optionKey(param)
    return key ? variableItem(`${param.name || key} 参数值`, `\${${basePath}.${key}}`, `参数标识: ${key}`) : null
  }).filter(Boolean)
}

function eventParams(event) {
  return event?.params || event?.parameters || event?.properties || []
}

function section(title, items) {
  return { title, items: compact(items) }
}

function variableItem(label, value, description = '') {
  if (!value) return null
  return { label, value, description }
}

function compact(items) {
  return (items || []).filter(Boolean)
}

function normalizeSections(sections) {
  return (sections || []).map(s => section(s.title, s.items)).filter(s => s.items.length)
}

function filterSections(sections, keyword) {
  if (!keyword) return sections
  const lower = keyword.toLowerCase()
  return sections.map(sec => ({
    ...sec,
    items: sec.items.filter(item => [
      item.label,
      item.value,
      item.description,
      sec.title
    ].some(text => String(text || '').toLowerCase().includes(lower)))
  })).filter(sec => sec.items.length)
}

function flattenActions(actions) {
  const result = []
  ;(actions || []).forEach(action => {
    if (['sequence_group', 'parallel_group'].includes(action.type)) {
      result.push(...flattenActions(action.subActions || []))
    } else {
      result.push(action)
    }
  })
  return result
}

function collectPreviousActionIds(actions, currentNode) {
  const targetId = actionId(currentNode)
  const previous = new Set()
  if (!targetId) return previous
  return collectPreviousFromList(actions || [], targetId, previous) ? previous : new Set()
}

function collectPreviousFromList(actions, targetId, previous) {
  for (const action of actions || []) {
    if (actionId(action) === targetId) return true
    if (isActionGroup(action)) {
      const beforeGroup = new Set(previous)
      if (action.type === 'sequence_group') {
        if (collectPreviousFromList(action.subActions || [], targetId, previous)) return true
      } else {
        for (const child of action.subActions || []) {
          const branchPrevious = new Set(beforeGroup)
          if (collectPreviousFromList([child], targetId, branchPrevious)) {
            previous.clear()
            branchPrevious.forEach(id => previous.add(id))
            return true
          }
        }
      }
      collectLeafActionIds(action).forEach(id => previous.add(id))
    } else {
      previous.add(actionId(action))
    }
  }
  return false
}

function collectLeafActionIds(action) {
  if (!action) return []
  if (!isActionGroup(action)) return [actionId(action)].filter(Boolean)
  return (action.subActions || []).flatMap(child => collectLeafActionIds(child))
}

function isActionGroup(action) {
  return ['sequence_group', 'parallel_group'].includes(action?.type)
}

function actionId(action) {
  return action?.id || action?._id || ''
}

function findDevice(devices, code) {
  return (devices || []).find(device => device.code === code)
}

function optionKey(item) {
  return item?.key || item?.identifier || item?.code || item?.id || item?.name || ''
}

function findByKey(options, key) {
  return (options || []).find(item => optionKey(item) === key)
}

function triggerLabel(trigger, index) {
  if (trigger.type === 'event') return `事件触发 ${index + 1}`
  if (trigger.type === 'device_status') return `状态触发 ${index + 1}`
  if (trigger.type === 'cron') return `定时触发 ${index + 1}`
  return `属性触发 ${index + 1}`
}

function conditionLabel(condition, index) {
  if (condition.type === 'device_status') return `状态判断 ${index + 1}`
  return `属性判断 ${index + 1}`
}

function actionLabel(action, index) {
  const labels = {
    set_property: '设置属性',
    call_service: '调用服务',
    notification: '消息通知',
    alarm: '触发告警',
    delay: '延迟执行',
    llm: '大模型',
    voice_playback: '语音播放'
  }
  return `${labels[action.type] || action.type || '动作'} ${index + 1}`
}
</script>

<style scoped>
.var-picker-popover {
  position: fixed;
  z-index: 10880;
  width: min(760px, calc(100vw - 24px));
  height: 360px;
  font-size: 0.85rem;
  box-shadow: 0 18px 40px rgba(15, 23, 42, 0.18), 0 8px 16px rgba(15, 23, 42, 0.12) !important;
}
.var-picker-header {
  height: 58px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.65rem 0.8rem;
  cursor: move;
  user-select: none;
}
.var-picker-drag-title {
  min-width: 0;
}
.var-picker-header-actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  flex-shrink: 0;
}
.var-picker-search {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  width: 200px;
  padding: 0.25rem 0.5rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 0.4rem;
  background: var(--bs-body-bg);
  cursor: default;
}
.var-picker-search input {
  width: 100%;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--bs-body-color);
  font-size: 0.82rem;
}
.var-picker-close,
.var-picker-pin {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.var-picker-layout {
  display: grid;
  grid-template-columns: 140px 200px 1fr;
  height: calc(100% - 58px);
}
.picker-item,
.var-item {
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--bs-body-color);
  text-align: left;
  border-radius: 0.35rem;
  padding: 0.38rem 0.45rem;
  margin-bottom: 0.25rem;
  cursor: pointer;
  transition: background-color 0.16s, color 0.16s;
}
.picker-item {
  display: flex;
  flex-direction: column;
}
.picker-item.active {
  background: var(--bs-primary);
  color: #fff;
}
.picker-item.active small {
  color: rgba(255, 255, 255, 0.75) !important;
}
.picker-item:hover:not(.active),
.var-item:hover {
  background-color: rgba(139, 92, 246, 0.1);
  color: #8b5cf6;
}
.var-section-title {
  position: sticky;
  top: -0.5rem;
  z-index: 1;
  padding: 0.45rem 0.35rem 0.3rem;
  margin: 0.2rem 0 0.15rem;
  background: var(--bs-body-bg);
  color: var(--bs-secondary-color);
  font-size: 0.72rem;
  font-weight: 700;
}
.var-item {
  display: grid;
  grid-template-columns: minmax(150px, 1fr) auto;
  align-items: center;
  gap: 0.75rem;
}
.var-item-label {
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.var-item-label span,
.var-item-label small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.var-item code {
  max-width: 300px;
  color: #8b5cf6;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
[data-bs-theme="dark"] .var-picker-popover {
  background-color: #1e293b !important;
  border-color: #334155 !important;
}
[data-bs-theme="dark"] .picker-item,
[data-bs-theme="dark"] .var-item {
  color: #cbd5e1;
}
[data-bs-theme="dark"] .picker-item:hover:not(.active),
[data-bs-theme="dark"] .var-item:hover {
  background-color: rgba(139, 92, 246, 0.2);
}
[data-bs-theme="dark"] .var-section-title {
  background: #1e293b;
}
@media (max-width: 720px) {
  .var-picker-popover {
    height: min(480px, calc(100vh - 24px));
  }
  .var-picker-header {
    height: auto;
    flex-direction: column;
    align-items: stretch;
  }
  .var-picker-header-actions {
    width: 100%;
  }
  .var-picker-search {
    flex: 1;
    width: 100%;
  }
  .var-picker-layout {
    grid-template-columns: 1fr;
    height: calc(100% - 104px);
  }
  .picker-pane {
    max-height: 120px;
    border-right: 0 !important;
    border-bottom: 1px solid var(--bs-border-color);
  }
  .var-item {
    grid-template-columns: 1fr;
    gap: 0.25rem;
  }
  .var-item code {
    max-width: 100%;
  }
}
</style>
