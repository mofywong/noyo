export function defaultNodeIdentity(node) {
  return node?._id || node?.id || ''
}

export function insertActionAt(targetList, action, insertIndex) {
  if (!Array.isArray(targetList) || !action) return false
  const boundedIndex = Number.isInteger(insertIndex)
    ? Math.max(0, Math.min(insertIndex, targetList.length))
    : targetList.length
  targetList.splice(boundedIndex, 0, action)
  return true
}

export function canActionNodeHandleDrop(node) {
  return !!node && !node.empty && Array.isArray(node._list)
}

export function findActionById(actions, sourceId, nodeIdentity = defaultNodeIdentity) {
  for (const action of actions || []) {
    if (nodeIdentity(action) === sourceId) return action
    const found = findActionById(action.subActions || [], sourceId, nodeIdentity)
    if (found) return found
  }
  return null
}

export function actionContainsId(action, targetId, nodeIdentity = defaultNodeIdentity) {
  if (!action || !targetId) return false
  if (nodeIdentity(action) === targetId) return true
  return (action.subActions || []).some(child => actionContainsId(child, targetId, nodeIdentity))
}

export function removeActionById(actions, sourceId, nodeIdentity = defaultNodeIdentity) {
  return removeActionFromList(actions || [], sourceId, nodeIdentity)
}

function removeActionFromList(list, sourceId, nodeIdentity) {
  const idx = (list || []).findIndex(item => nodeIdentity(item) === sourceId)
  if (idx >= 0) {
    const [node] = list.splice(idx, 1)
    return { node, list, index: idx }
  }

  for (const item of list || []) {
    const found = removeActionFromList(item.subActions || [], sourceId, nodeIdentity)
    if (found) return found
  }

  return null
}

export function moveActionToList(actions, sourceId, targetList, insertIndex, targetGroup = null, nodeIdentity = defaultNodeIdentity) {
  if (!sourceId || !Array.isArray(targetList)) return false
  const movingAction = findActionById(actions, sourceId, nodeIdentity)
  const targetId = nodeIdentity(targetGroup)
  if (targetId && actionContainsId(movingAction, targetId, nodeIdentity)) return false

  const removed = removeActionById(actions, sourceId, nodeIdentity)
  if (!removed) return false

  let adjustedIndex = insertIndex
  if (removed.list === targetList && Number.isInteger(adjustedIndex) && removed.index < adjustedIndex) {
    adjustedIndex -= 1
  }
  return insertActionAt(targetList, removed.node, adjustedIndex)
}
