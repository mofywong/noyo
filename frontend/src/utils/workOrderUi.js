const PERMISSIONS = Object.freeze({
  create: 'work_order:create',
  process: 'work_order:process',
  approve: 'work_order:approve',
  manage: 'work_order:manage',
  integration: 'work_order:integration'
})

function permissionSet(permissions = {}) {
  if (Array.isArray(permissions)) return new Set(permissions.map(String))
  if (permissions && typeof permissions === 'object') {
    return new Set(Object.entries(permissions).filter(([, allowed]) => Boolean(allowed)).map(([code]) => code))
  }
  return new Set()
}

export function hasWorkOrderPermission(permissions, permission) {
  return permissionSet(permissions).has(permission)
}

function candidateValues(task = {}) {
  return [task.candidate_refs, task.candidate_user_ids, task.candidate_ids, task.approver_user_ids]
    .find(value => Array.isArray(value)) || []
}

export function isApprovalCandidate(task = {}, currentUser = '') {
  if (task.status !== 'pending') return false
  const expected = String(currentUser)
  return candidateValues(task).some(candidate => String(candidate) === expected)
}

export function canShowWorkOrderAction(action, permissions = {}, task = {}, currentUser = '') {
  const allowed = permissionSet(permissions)
  if (action === 'approve' || action === 'reject') {
    return allowed.has(PERMISSIONS.approve) && isApprovalCandidate(task, currentUser)
  }
  if (action === 'create') return allowed.has(PERMISSIONS.create)
  if (action === 'integrate' || action === 'retry' || action === 'replay') return allowed.has(PERMISSIONS.integration)
  if (action === 'manage' || action === 'publish') return allowed.has(PERMISSIONS.manage)
  return allowed.has(PERMISSIONS.process)
}

export function applyVersionConflict(current, incoming, errorCode) {
  if (String(errorCode || '').toUpperCase() !== 'VERSION_CONFLICT') {
    const result = { value: incoming, conflict: false }
    Object.defineProperty(result, 'refreshRequired', { value: false, enumerable: false })
    return result
  }
  const result = { value: current, conflict: true, message: '工单已被其他用户更新，请刷新后重试' }
  Object.defineProperty(result, 'refreshRequired', { value: true, enumerable: false })
  return result
}

export function errorCodeOf(error) {
  return error?.response?.data?.error_code || error?.response?.data?.code_name || error?.response?.data?.details?.code || error?.code || ''
}

export function publishErrorLocation(error) {
  const data = error?.response?.data || error || {}
  const raw = String(data.message || data.error || error?.message || '')
  const path = data.path || data.field || data.node || data.details?.path || ''
  const match = raw.match(/(?:field|字段|node|节点|path|路径)\s*[=:：]?\s*[`'\"]?([\w.[\]-]+)[`'\"]?/i)
  const location = String(path || match?.[1] || '').trim()
  return {
    message: raw || '发布失败，请检查定义后重试',
    location,
    field: location.startsWith('fields.') ? location.slice(7) : location,
    node: location.startsWith('nodes.') ? location.slice(6) : ''
  }
}

const FEEDBACK = Object.freeze({
  dead_letter: { level: 'danger', message: '消息已进入死信队列', requiresAction: true, action: 'retry' },
  conflict: { level: 'warning', message: '同步发生版本冲突', requiresAction: true, action: 'refresh' },
  replayed: { level: 'info', message: '消息已重放', requiresAction: false, action: '' },
  delivered: { level: 'success', message: '操作已完成', requiresAction: false, action: '' }
})

export function integrationFeedback(result = {}) {
  const status = String(result.status || result.state || result.code || '').toLowerCase()
  const value = FEEDBACK[status] || FEEDBACK.delivered
  return { ...value, status: status || 'delivered', message: result.message || value.message }
}

export function filterWorkOrders(entries = [], filters = {}) {
  const keyword = String(filters.keyword || '').trim().toLowerCase()
  return entries.filter(entry => {
    const order = entry?.work_order || entry || {}
    if (filters.status && order.status !== filters.status) return false
    if (filters.sourceType && order.source_type !== filters.sourceType) return false
    if (!keyword) return true
    return [order.code, order.title, order.summary].some(value => String(value || '').toLowerCase().includes(keyword))
  })
}

export function workOrderId(order) {
  return order?.ID || order?.id || 0
}

export const workOrderRelationFilters = Object.freeze([
  { key: '', label: '全部工单' },
  { key: 'pending', label: '待我处理' },
  { key: 'pending_approval', label: '待我审批' },
  { key: 'processed', label: '我已处理' },
  { key: 'created', label: '我创建的' },
  { key: 'cc', label: '抄送我的' }
])

const WORK_ORDER_RELATION_LABELS = Object.freeze({
  '': { zh: '全部工单', en: 'All work orders' },
  pending: { zh: '待我处理', en: 'Pending for me' },
  pending_approval: { zh: '待我审批', en: 'Pending approval' },
  processed: { zh: '我已处理', en: 'Processed by me' },
  created: { zh: '我创建的', en: 'Created by me' },
  cc: { zh: '抄送我的', en: 'Copied to me' }
})

export function workOrderRelationFiltersForLocale(locale = 'zh') {
  const lang = language(locale)
  return workOrderRelationFilters.map(filter => ({
    ...filter,
    label: WORK_ORDER_RELATION_LABELS[filter.key]?.[lang] || filter.label
  }))
}

const WORK_ORDER_EVENT_LABELS = Object.freeze({
  created: { zh: '创建工单', en: 'Created' },
  updated: { zh: '更新工单', en: 'Updated' },
  claimed: { zh: '领取工单', en: 'Claimed' },
  transferred: { zh: '转交工单', en: 'Transferred' },
  commented: { zh: '添加备注', en: 'Commented' },
  cc: { zh: '抄送工单', en: 'Copied' },
  transitioned: { zh: '状态流转', en: 'Status changed' },
  'approval.requested': { zh: '发起审批', en: 'Approval requested' },
  'approval.approved': { zh: '审批通过', en: 'Approved' },
  'approval.completed': { zh: '审批完成', en: 'Approval completed' },
  'approval.rejected': { zh: '审批驳回', en: 'Rejected' },
  'workflow.task.assigned': { zh: '分配流程任务', en: 'Task assigned' },
  'workflow.task.transferred': { zh: '转交流程任务', en: 'Task transferred' },
  'workflow.task.completed': { zh: '完成流程任务', en: 'Task completed' },
  'workflow.task.rejected': { zh: '驳回流程任务', en: 'Task rejected' },
  'workflow.node.completed': { zh: '完成流程节点', en: 'Step completed' },
  'workflow.completed': { zh: '流程完成', en: 'Workflow completed' }
})

function language(locale = 'zh') {
  return String(locale || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh'
}

function localized(locale, zh, en) {
  return language(locale) === 'en' ? en : zh
}

function dateValue(value) {
  if (!value) return 0
  const timestamp = new Date(value).getTime()
  return Number.isNaN(timestamp) ? 0 : timestamp
}

function fieldValue(value = {}, ...keys) {
  for (const key of keys) if (value?.[key] !== undefined && value?.[key] !== null) return value[key]
  return undefined
}

export function workOrderEventLabel(type, locale = 'zh') {
  const label = WORK_ORDER_EVENT_LABELS[type]
  return label?.[language(locale)] || localized(locale, '工单事件', 'Work order event')
}

export function workOrderActorLabel(item = {}, locale = 'zh') {
  const actorUserID = fieldValue(item.event, 'actor_user_id', 'ActorUserID')
  if (!actorUserID) return localized(locale, '系统', 'System')
  const resolved = String(item.actor_name || '').trim()
  const unavailableLabels = new Set(['已删除或不可用用户', 'Deleted or unavailable user', '已删除或不可用用户 / Deleted or unavailable user'])
  if (resolved && !unavailableLabels.has(resolved)) return resolved
  return localized(locale, '已删除或不可用用户', 'Deleted or unavailable user')
}

export function workOrderNodeLabel(workflow = {}, nodeID = '', locale = 'zh') {
  const node = (workflow.nodes || []).find(candidate => String(candidate?.id) === String(nodeID))
  if (node?.type === 'start') return localized(locale, '开始', 'Start')
  if (node?.type === 'end') return localized(locale, '结束', 'End')
  return String(node?.name || '').trim() || localized(locale, '未命名流程节点', 'Unnamed workflow step')
}

function taskStatusForNode(node, tasks, events) {
  if (node.type === 'start') return events.length > 0 ? 'completed' : 'waiting'
  if (node.type === 'end') return events.some(item => fieldValue(item.event, 'type', 'Type') === 'workflow.completed') ? 'completed' : 'waiting'
  const matchingTasks = tasks.filter(task => String(fieldValue(task, 'node_id', 'NodeID')) === String(node.id))
  const matchingEvents = events.filter(item => String(item.payload?.node_id || '') === String(node.id))
  if (!matchingTasks.length) {
    if (matchingEvents.some(item => ['workflow.node.completed', 'workflow.task.completed', 'cc'].includes(fieldValue(item.event, 'type', 'Type')))) return 'completed'
    return 'waiting'
  }
  const latestCycle = Math.max(...matchingTasks.map(task => Number(fieldValue(task, 'workflow_cycle', 'WorkflowCycle') || 0)))
  const latest = matchingTasks.filter(task => Number(fieldValue(task, 'workflow_cycle', 'WorkflowCycle') || 0) === latestCycle)
  const statuses = latest.map(task => String(fieldValue(task, 'status', 'Status') || ''))
  if (statuses.includes('pending')) return 'active'
  if (statuses.includes('rejected')) return 'rejected'
  if (node.completion_mode === 'any' && statuses.includes('completed')) return 'completed'
  if (statuses.length && statuses.every(status => status === 'completed' || status === 'cancelled')) return 'completed'
  return 'waiting'
}

function traceNode(node, tasks, events, locale) {
  const matchingTasks = tasks.filter(task => String(fieldValue(task, 'node_id', 'NodeID')) === String(node.id))
  const latestCycle = matchingTasks.length ? Math.max(...matchingTasks.map(task => Number(fieldValue(task, 'workflow_cycle', 'WorkflowCycle') || 0))) : 0
  const latestTasks = matchingTasks.filter(task => Number(fieldValue(task, 'workflow_cycle', 'WorkflowCycle') || 0) === latestCycle)
  const people = [...new Set(latestTasks.map(task => String(fieldValue(task, 'assignee_display_name', 'AssigneeDisplayName') || '').trim()).filter(Boolean))]
  const records = [
    ...latestTasks.map(task => ({
      time: fieldValue(task, 'CompletedAt', 'completed_at', 'UpdatedAt', 'updated_at', 'CreatedAt', 'created_at'),
      comment: String(fieldValue(task, 'comment', 'Comment') || '').trim()
    })),
    ...events.filter(item => String(item.payload?.node_id || '') === String(node.id)).map(item => ({
      time: fieldValue(item.event, 'CreatedAt', 'created_at'),
      comment: String(item.payload?.comment || '').trim()
    }))
  ].sort((a, b) => dateValue(b.time) - dateValue(a.time))
  return { type: 'node', id: node.id, node, label: workOrderNodeLabel({ nodes: [node] }, node.id, locale), status: taskStatusForNode(node, tasks, events), people, comment: records.find(record => record.comment)?.comment || '', time: records[0]?.time || '' }
}

function aggregateStatus(steps) {
  const statuses = steps.flatMap(step => step.type === 'parallel' ? step.branches.flatMap(branch => branch.steps.map(child => child.status)) : [step.status])
  if (statuses.includes('active')) return 'active'
  if (statuses.includes('rejected')) return 'rejected'
  if (statuses.length && statuses.every(status => status === 'completed')) return 'completed'
  return 'waiting'
}

export function buildWorkOrderFlowTrace(workflow = {}, tasks = [], events = [], locale = 'zh') {
  const nodes = Array.isArray(workflow.nodes) ? workflow.nodes : []
  const edges = Array.isArray(workflow.edges) ? workflow.edges : []
  const nodesByID = new Map(nodes.map(node => [String(node.id), node]))
  const outgoing = new Map()
  for (const edge of edges) {
    const source = String(edge.source || '')
    if (!outgoing.has(source)) outgoing.set(source, [])
    outgoing.get(source).push(edge)
  }
  const layoutByID = new Map((workflow.layout?.nodes || []).map(node => [String(node.id), node]))
  const sortEdges = list => [...(list || [])].sort((a, b) => {
    const ax = Number(layoutByID.get(String(a.target))?.x || 0)
    const bx = Number(layoutByID.get(String(b.target))?.x || 0)
    return ax - bx || String(a.branch_id || a.target).localeCompare(String(b.branch_id || b.target))
  })
  const joinsByPairID = new Map(nodes.filter(node => node.type === 'parallel_join').map(node => [String(node.pair_id || ''), node]))
  const branchByNodeID = new Map()
  const branchByID = new Map()

  for (const split of nodes.filter(node => node.type === 'parallel_split')) {
    const join = joinsByPairID.get(String(split.pair_id || ''))
    if (!join) continue
    sortEdges(outgoing.get(String(split.id))).forEach((edge, index) => {
      const meta = { id: String(edge.branch_id || `${split.id}:${index + 1}`), index: index + 1, label: localized(locale, `分支 ${index + 1}`, `Branch ${index + 1}`) }
      branchByID.set(meta.id, meta)
      const queue = [String(edge.target)]
      const seen = new Set()
      while (queue.length) {
        const nodeID = queue.shift()
        if (!nodeID || nodeID === String(join.id) || seen.has(nodeID)) continue
        seen.add(nodeID)
        branchByNodeID.set(nodeID, meta)
        for (const next of outgoing.get(nodeID) || []) queue.push(String(next.target))
      }
    })
  }

  const walk = (startID, stopID = '', ancestors = new Set()) => {
    const steps = []
    let currentID = String(startID || '')
    const seen = new Set(ancestors)
    while (currentID && currentID !== String(stopID || '') && !seen.has(currentID)) {
      seen.add(currentID)
      const node = nodesByID.get(currentID)
      if (!node) break
      if (node.type === 'parallel_split') {
        const join = joinsByPairID.get(String(node.pair_id || ''))
        if (!join) break
        const branches = sortEdges(outgoing.get(currentID)).map((edge, index) => {
          const meta = branchByID.get(String(edge.branch_id || '')) || { id: String(edge.branch_id || `${node.id}:${index + 1}`), index: index + 1, label: localized(locale, `分支 ${index + 1}`, `Branch ${index + 1}`) }
          return { ...meta, steps: walk(edge.target, join.id, seen) }
        })
        const parallel = { type: 'parallel', id: node.id, branches }
        parallel.status = aggregateStatus(branches.flatMap(branch => branch.steps))
        steps.push(parallel)
        currentID = String(sortEdges(outgoing.get(String(join.id)))[0]?.target || '')
        continue
      }
      if (node.type === 'parallel_join') break
      steps.push(traceNode(node, tasks, events, locale))
      currentID = String(sortEdges(outgoing.get(currentID))[0]?.target || '')
    }
    return steps
  }

  const audit = [...events].sort((a, b) => Number(fieldValue(a.event, 'sequence', 'Sequence') || 0) - Number(fieldValue(b.event, 'sequence', 'Sequence') || 0)).map(item => {
    const nodeID = String(item.payload?.node_id || item.payload?.end_node_id || '')
    const branch = branchByID.get(String(item.payload?.branch_id || '')) || branchByNodeID.get(nodeID)
    return {
      ...item,
      label: workOrderEventLabel(fieldValue(item.event, 'type', 'Type'), locale),
      actor: workOrderActorLabel(item, locale),
      nodeID,
      nodeName: nodeID ? workOrderNodeLabel(workflow, nodeID, locale) : '',
      branchLabel: branch?.label || '',
      transferRecipient: String(item.payload?.to_user_name || '').trim()
    }
  })
  return { steps: walk(workflow.start_node_id || nodes.find(node => node.type === 'start')?.id), audit }
}

export const workOrderPermissions = PERMISSIONS
