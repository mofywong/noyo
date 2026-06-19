function uid(prefix) {
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`
}

export function safeParse(value, fallback) {
  if (Array.isArray(value) || (value && typeof value === 'object')) return value
  try {
    const parsed = JSON.parse(value || '')
    return parsed || fallback
  } catch (_) {
    return fallback
  }
}

export function defaultEffectiveTime() {
  return {
    mode: 'always',
    windows: [{ startTime: '00:00:00', endTime: '24:00:00' }],
    weekdays: [],
    monthDays: [],
    months: []
  }
}

export function baseTrigger(type = 'property_change') {
  return {
    id: uid('trg'),
    kind: 'trigger',
    type,
    deviceCode: '',
    propertyKey: '',
    operator: 'changed',
    value: '',
    eventId: '',
    statusValue: 'online',
    cronMode: 'visual',
    cronExpr: '*/5 * * * *',
    schedule: {
      mode: 'every_minutes',
      intervalMinutes: 5,
      hour: 0,
      minute: 0,
      weekdaysText: '1,2,3,4,5',
      monthDaysText: '1'
    }
  }
}

export function baseCondition(type = 'property') {
  return {
    id: uid('cond'),
    kind: 'condition',
    type,
    deviceCode: '',
    propertyKey: '',
    operator: 'eq',
    value: '',
    statusValue: 'online',
    startTime: '',
    endTime: ''
  }
}

export function baseAction(type = 'set_property') {
  const action = {
    id: uid('act'),
    kind: 'action',
    type,
    deviceCode: '',
    propertyKey: '',
    value: '',
    serviceCode: '',
    serviceParams: {},
    notifyTitle: '',
    notifyContent: '',
    alarmLevel: 'warning',
    alarmTitle: '',
    alarmContent: '',
    alarmDevice: 'trigger',
    delaySec: 1,
    llmPrompt: '',
    llmPlayAudio: false,
    llmIncludeContext: false,
    children: []
  }
  if (type === 'sequence_group' || type === 'parallel_group') {
    action.children = [baseAction('set_property')]
  }
  return action
}

export function actionToNode(action = {}) {
  const children = (action.subActions || []).map(actionToNode)
  const node = {
    ...baseAction(action.type || 'set_property'),
    ...action,
    kind: 'action',
    children
  }
  delete node.subActions
  return node
}

export function actionNodeToAction(node = {}) {
  const { kind, children, subActions, ...rest } = node
  if (rest.type === 'voice_playback') delete rest.voiceText
  return {
    ...rest,
    subActions: (children || []).map(actionNodeToAction)
  }
}

export function createGraphNode(kind, type) {
  if (kind === 'trigger') return baseTrigger(type || 'property_change')
  if (kind === 'condition') return baseCondition(type || 'property')
  if (kind === 'condition_group') {
    return {
      id: uid('cond_group'),
      kind: 'condition_group',
      name: 'Condition group',
      logic: 'and',
      conditions: [],
      groups: []
    }
  }
  if (kind === 'action') return baseAction(type || 'set_property')
  return null
}

function normalizeConditionGroup(group) {
  const source = group || {}
  return {
    id: source.id || uid('cond_group'),
    kind: 'condition_group',
    name: source.name || '主条件组',
    logic: String(source.logic || 'and').toLowerCase() === 'or' ? 'or' : 'and',
    conditions: (source.conditions || []).map(condition => ({
      ...baseCondition(condition.type || 'property'),
      ...condition,
      kind: 'condition'
    })),
    groups: (source.groups || []).map(normalizeConditionGroup)
  }
}

function conditionGroupToPayload(group) {
  return {
    logic: String(group.logic || 'and').toLowerCase() === 'or' ? 'or' : 'and',
    name: group.name || '主条件组',
    conditions: (group.conditions || []).map(({ kind, ...condition }) => condition),
    groups: (group.groups || []).map(conditionGroupToPayload)
  }
}

export function buildRuleGraphDraft(rule = {}) {
  const conditions = safeParse(rule.conditions, { logic: 'and', conditions: [], groups: [], name: '主条件组' })
  return {
    meta: {
      code: rule.code || '',
      name: rule.name || '',
      description: rule.description || '',
      group_id: rule.group_id || rule.GroupID || null,
      priority: rule.priority || 50,
      throttle_sec: rule.throttle_sec || 60,
      max_per_hour: rule.max_per_hour || 60,
      retry_count: rule.retry_count || 0
    },
    triggers: safeParse(rule.triggers, []).map(trigger => ({
      ...baseTrigger(trigger.type || 'property_change'),
      ...trigger,
      kind: 'trigger'
    })),
    conditionGroups: [normalizeConditionGroup(conditions)],
    effectiveTime: { ...defaultEffectiveTime(), ...safeParse(rule.effective_time, defaultEffectiveTime()) },
    actions: safeParse(rule.actions, []).map(actionToNode),
    selected: null
  }
}

function updateListItem(list, id, patch) {
  return list.map(item => (item.id === id ? { ...item, ...patch } : item))
}

function updateActionTree(actions, id, patch) {
  return actions.map(action => {
    if (action.id === id) return { ...action, ...patch }
    return { ...action, children: updateActionTree(action.children || [], id, patch) }
  })
}

function updateConditionGroup(group, id, patch) {
  if (group.id === id) return { ...group, ...patch }
  return {
    ...group,
    conditions: updateListItem(group.conditions || [], id, patch),
    groups: (group.groups || []).map(child => (child.id === id ? { ...child, ...patch } : updateConditionGroup(child, id, patch)))
  }
}

export function updateDraftNode(draft, kind, id, patch) {
  if (kind === 'trigger') return { ...draft, triggers: updateListItem(draft.triggers, id, patch) }
  if (kind === 'condition') return { ...draft, conditionGroups: draft.conditionGroups.map(group => updateConditionGroup(group, id, patch)) }
  if (kind === 'condition_group') return { ...draft, conditionGroups: draft.conditionGroups.map(group => updateConditionGroup(group, id, patch)) }
  if (kind === 'effective_time') return { ...draft, effectiveTime: { ...draft.effectiveTime, ...patch } }
  if (kind === 'action') return { ...draft, actions: updateActionTree(draft.actions, id, patch) }
  return draft
}

function insertActionInTree(actions, afterId, node) {
  const result = []
  for (const action of actions) {
    result.push({ ...action, children: insertActionInTree(action.children || [], afterId, node) })
    if (action.id === afterId) result.push(node)
  }
  return result
}

export function insertActionAfter(draft, afterId, node = baseAction()) {
  return {
    ...draft,
    actions: insertActionInTree(draft.actions, afterId, { ...node, kind: 'action', children: node.children || [] })
  }
}

function listLastId(list) {
  return list && list.length ? list[list.length - 1].id : null
}

function insertListAfter(list, afterId, node) {
  if (!afterId) return [...list, node]
  const result = []
  let inserted = false
  for (const item of list) {
    result.push(item)
    if (item.id === afterId) {
      result.push(node)
      inserted = true
    }
  }
  return inserted ? result : [...list, node]
}

export function insertTriggerAfter(draft, afterId, node = baseTrigger()) {
  return {
    ...draft,
    triggers: insertListAfter(draft.triggers || [], afterId, { ...node, kind: 'trigger' })
  }
}

function insertConditionInGroup(group, afterId, node) {
  const nextNode = { ...node, kind: 'condition' }
  if (!afterId || group.id === afterId) {
    return { group: { ...group, conditions: [...(group.conditions || []), nextNode] }, inserted: true }
  }

  const conditions = group.conditions || []
  const index = conditions.findIndex(condition => condition.id === afterId)
  if (index >= 0) {
    return {
      group: {
        ...group,
        conditions: [
          ...conditions.slice(0, index + 1),
          nextNode,
          ...conditions.slice(index + 1)
        ]
      },
      inserted: true
    }
  }

  let inserted = false
  const groups = (group.groups || []).map(child => {
    if (inserted) return child
    const result = insertConditionInGroup(child, afterId, node)
    inserted = result.inserted
    return result.group
  })
  return { group: { ...group, groups }, inserted }
}

export function insertConditionAfter(draft, afterId, node = baseCondition()) {
  const rootId = draft.conditionGroups?.[0]?.id
  const targetId = afterId && afterId !== rootId ? afterId : null
  let inserted = false
  const conditionGroups = (draft.conditionGroups || []).map(group => {
    if (inserted) return group
    const result = insertConditionInGroup(group, targetId, node)
    inserted = result.inserted
    return result.group
  })
  return {
    ...draft,
    conditionGroups
  }
}

export function insertActionChild(draft, parentId, node = baseAction()) {
  const nextNode = { ...node, kind: 'action', children: node.children || [] }
  function visit(actions) {
    return (actions || []).map(action => {
      if (action.id === parentId) return { ...action, children: [...(action.children || []), nextNode] }
      return { ...action, children: visit(action.children || []) }
    })
  }
  return { ...draft, actions: visit(draft.actions || []) }
}

export function addPaletteItemToDraft(draft, item = {}) {
  const node = createGraphNode(item.kind, item.type)
  if (!node) return { draft, node: null }

  if (item.kind === 'trigger') {
    return {
      draft: insertTriggerAfter(draft, listLastId(draft.triggers || []), node),
      node
    }
  }

  if (item.kind === 'condition') {
    return {
      draft: insertConditionAfter(draft, draft.conditionGroups?.[0]?.id, node),
      node
    }
  }

  if (item.kind === 'action') {
    const afterId = listLastId(draft.actions || [])
    return {
      draft: afterId
        ? insertActionAfter(draft, afterId, node)
        : { ...draft, actions: [...(draft.actions || []), node] },
      node
    }
  }

  return { draft, node: null }
}

function removeFromActionTree(actions, id) {
  return (actions || [])
    .filter(action => action.id !== id)
    .map(action => ({ ...action, children: removeFromActionTree(action.children || [], id) }))
}

function removeFromConditionGroup(group, id) {
  return {
    ...group,
    conditions: (group.conditions || []).filter(condition => condition.id !== id),
    groups: (group.groups || [])
      .filter(child => child.id !== id)
      .map(child => removeFromConditionGroup(child, id))
  }
}

export function removeDraftNode(draft, kind, id) {
  if (!id || kind === 'meta' || kind === 'effective_time') return draft
  if (kind === 'trigger') return { ...draft, triggers: (draft.triggers || []).filter(trigger => trigger.id !== id) }
  if (kind === 'condition' || kind === 'condition_group') {
    return {
      ...draft,
      conditionGroups: (draft.conditionGroups || []).map(group => removeFromConditionGroup(group, id))
    }
  }
  if (kind === 'action') return { ...draft, actions: removeFromActionTree(draft.actions || [], id) }
  return draft
}

function findInConditionGroup(group, kind, id) {
  if (!group) return null
  if (kind === 'condition_group' && group.id === id) return group
  if (kind === 'condition') {
    const condition = (group.conditions || []).find(item => item.id === id)
    if (condition) return condition
  }
  for (const child of group.groups || []) {
    const found = findInConditionGroup(child, kind, id)
    if (found) return found
  }
  return null
}

function findInActions(actions, id) {
  for (const action of actions || []) {
    if (action.id === id) return action
    const found = findInActions(action.children || [], id)
    if (found) return found
  }
  return null
}

export function findDraftNode(draft, selection) {
  if (!selection) return { kind: 'meta', id: 'meta', node: draft.meta }
  if (selection.kind === 'meta') return { kind: 'meta', id: 'meta', node: draft.meta }
  if (selection.kind === 'effective_time') return { kind: 'effective_time', id: 'effective_time', node: draft.effectiveTime }
  if (selection.kind === 'trigger') return { ...selection, node: (draft.triggers || []).find(item => item.id === selection.id) || null }
  if (selection.kind === 'condition' || selection.kind === 'condition_group') {
    return { ...selection, node: findInConditionGroup(draft.conditionGroups?.[0], selection.kind, selection.id) }
  }
  if (selection.kind === 'action') return { ...selection, node: findInActions(draft.actions || [], selection.id) }
  return { ...selection, node: null }
}

function hasConditionContent(group) {
  return !!group && ((group.conditions || []).length > 0 || (group.groups || []).some(hasConditionContent))
}

export function validateRuleGraphDraft(draft) {
  const issues = []
  if (!draft.meta.name) issues.push({ code: 'name_required', message: 'Rule name is required' })
  if (!draft.triggers.length) issues.push({ code: 'trigger_required', message: 'At least one trigger is required' })
  if (!draft.actions.length) issues.push({ code: 'action_required', message: 'At least one action is required' })
  return issues
}

export function graphDraftToPayload(draft, enable = false) {
  const rootConditions = draft.conditionGroups[0]
  return {
    ...draft.meta,
    group_id: draft.meta.group_id || null,
    effective_time: draft.effectiveTime || defaultEffectiveTime(),
    triggers: draft.triggers.map(({ kind, ...trigger }) => trigger),
    conditions: hasConditionContent(rootConditions) ? conditionGroupToPayload(rootConditions) : null,
    actions: draft.actions.map(actionNodeToAction),
    enable
  }
}
