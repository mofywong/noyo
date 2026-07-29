const restartableNodeTypes = new Set(['start', 'user_task', 'parallel_split'])

function isWorkflowIdentifier(value) {
  return typeof value === 'string' && /^[a-z0-9_-]{2,64}$/.test(value.trim())
}

function workflowGraphMaps(nodes, edges) {
  const nodeByID = new Map(nodes.map(node => [node.id, node]))
  const incomingByTarget = new Map()
  const outgoingBySource = new Map()
  for (const edge of edges) {
    const incoming = incomingByTarget.get(edge.target) || []
    incoming.push(edge.source)
    incomingByTarget.set(edge.target, incoming)
    const outgoing = outgoingBySource.get(edge.source) || []
    outgoing.push(edge)
    outgoingBySource.set(edge.source, outgoing)
  }
  return { nodeByID, incomingByTarget, outgoingBySource }
}

function parallelSplitByBranchNode(nodeByID, outgoingBySource) {
  const joinByPairID = new Map()
  for (const node of nodeByID.values()) {
    if (node.type === 'parallel_join') joinByPairID.set(node.pair_id, node.id)
  }
  const result = new Map()
  for (const split of nodeByID.values()) {
    if (split.type !== 'parallel_split') continue
    const joinID = joinByPairID.get(split.pair_id)
    const queue = (outgoingBySource.get(split.id) || []).map(edge => edge.target)
    const visited = new Set([split.id])
    while (queue.length > 0) {
      const currentID = queue.shift()
      if (currentID === joinID || visited.has(currentID)) continue
      visited.add(currentID)
      result.set(currentID, split.id)
      queue.push(...(outgoingBySource.get(currentID) || []).map(edge => edge.target))
    }
  }
  return result
}

export function workOrderWorkflowRejectTargetNodes(nodeID, nodes, edges) {
  const { nodeByID, incomingByTarget, outgoingBySource } = workflowGraphMaps(nodes, edges)
  const containingSplitByNode = parallelSplitByBranchNode(nodeByID, outgoingBySource)
  const candidates = []
  const candidateIDs = new Set()
  const visited = new Set([nodeID])
  let restartFromID = nodeID
  const containingSplitID = containingSplitByNode.get(nodeID)
  if (containingSplitID && nodeByID.has(containingSplitID)) {
    candidates.push(nodeByID.get(containingSplitID))
    candidateIDs.add(containingSplitID)
    visited.add(containingSplitID)
    restartFromID = containingSplitID
  }
  const splitByPairID = new Map()
  for (const node of nodeByID.values()) {
    if (node.type === 'parallel_split') splitByPairID.set(node.pair_id, node.id)
  }
  const queue = [...(incomingByTarget.get(restartFromID) || [])]
  while (queue.length > 0) {
    const currentID = queue.shift()
    if (visited.has(currentID)) continue
    visited.add(currentID)
    const node = nodeByID.get(currentID)
    if (!node) continue
    if (node.type === 'parallel_join') {
      const splitID = splitByPairID.get(node.pair_id)
      if (splitID && !candidateIDs.has(splitID) && nodeByID.has(splitID)) {
        candidates.push(nodeByID.get(splitID))
        candidateIDs.add(splitID)
        queue.push(...(incomingByTarget.get(splitID) || []))
      }
      continue
    }
    if (restartableNodeTypes.has(node.type) && !candidateIDs.has(node.id)) {
      candidates.push(node)
      candidateIDs.add(node.id)
    }
    queue.push(...(incomingByTarget.get(currentID) || []))
  }
  return candidates
}

export function canInsertWorkOrderParallelOnEdge(edgeID, nodes, edges) {
  const { nodeByID, outgoingBySource } = workflowGraphMaps(nodes, edges)
  const containingSplitByNode = parallelSplitByBranchNode(nodeByID, outgoingBySource)
  const edge = edges.find(candidate => candidate.id === edgeID)
  return !!edge && !containingSplitByNode.has(edge.source) && !containingSplitByNode.has(edge.target)
}

function nextWorkflowEdgeID(edges, source, target) {
  const base = `edge_${source.slice(0, 24)}_${target.slice(0, 24)}`
  const usedIDs = new Set(edges.map(edge => edge.id))
  if (!usedIDs.has(base)) return base
  let suffix = 2
  while (usedIDs.has(`${base}_${suffix}`)) suffix += 1
  return `${base}_${suffix}`
}

function appendWorkflowEdge(edges, source, target, branchID = '') {
  if (!source || !target || source === target) return
  const existing = edges.find(edge => edge.source === source && edge.target === target)
  if (existing) {
    if (!existing.branch_id && branchID) existing.branch_id = branchID
    return
  }
  const edge = { id: nextWorkflowEdgeID(edges, source, target), source, target }
  if (branchID) edge.branch_id = branchID
  edges.push(edge)
}

function collapseDegenerateParallelGateways(nodes, edges) {
  let currentNodes = nodes
  let currentEdges = edges
  let changed = true

  while (changed) {
    changed = false
    const split = currentNodes.find(node => node.type === 'parallel_split' && currentEdges.filter(edge => edge.source === node.id).length < 2)
    if (!split) break

    const join = currentNodes.find(node => node.type === 'parallel_join' && node.pair_id === split.pair_id)
    const incomingToSplit = currentEdges.filter(edge => edge.target === split.id)
    const outgoingFromSplit = currentEdges.filter(edge => edge.source === split.id)

    if (!join) {
      currentNodes = currentNodes.filter(node => node.id !== split.id)
      currentEdges = currentEdges.filter(edge => edge.source !== split.id && edge.target !== split.id)
      for (const incoming of incomingToSplit) {
        for (const outgoing of outgoingFromSplit) {
          appendWorkflowEdge(currentEdges, incoming.source, outgoing.target, incoming.branch_id || '')
        }
      }
      changed = true
      continue
    }

    const incomingToJoin = currentEdges.filter(edge => edge.target === join.id)
    const outgoingFromJoin = currentEdges.filter(edge => edge.source === join.id)
    const remainingBranchStart = outgoingFromSplit[0]?.target
    currentNodes = currentNodes.filter(node => node.id !== split.id && node.id !== join.id)
    currentEdges = currentEdges.filter(edge => ![split.id, join.id].includes(edge.source) && ![split.id, join.id].includes(edge.target))

    if (remainingBranchStart && remainingBranchStart !== join.id) {
      for (const incoming of incomingToSplit) {
        appendWorkflowEdge(currentEdges, incoming.source, remainingBranchStart, incoming.branch_id || '')
      }
      for (const branchEnd of incomingToJoin.filter(edge => edge.source !== split.id)) {
        for (const outgoing of outgoingFromJoin) {
          appendWorkflowEdge(currentEdges, branchEnd.source, outgoing.target, outgoing.branch_id || '')
        }
      }
    } else {
      for (const incoming of incomingToSplit) {
        for (const outgoing of outgoingFromJoin) {
          appendWorkflowEdge(currentEdges, incoming.source, outgoing.target, incoming.branch_id || outgoing.branch_id || '')
        }
      }
    }
    changed = true
  }

  const splitIDs = new Set(currentNodes.filter(node => node.type === 'parallel_split').map(node => node.id))
  for (const edge of currentEdges) {
    if (!splitIDs.has(edge.source)) delete edge.branch_id
  }
  return { nodes: currentNodes, edges: currentEdges }
}

function workflowNodeLevels(nodes, edges, startNodeID) {
  const nodeIDs = new Set(nodes.map(node => node.id))
  const incomingCounts = new Map(nodes.map(node => [node.id, 0]))
  const outgoingBySource = new Map()
  for (const edge of edges) {
    if (!nodeIDs.has(edge.source) || !nodeIDs.has(edge.target)) continue
    incomingCounts.set(edge.target, (incomingCounts.get(edge.target) || 0) + 1)
    const outgoing = outgoingBySource.get(edge.source) || []
    outgoing.push(edge.target)
    outgoingBySource.set(edge.source, outgoing)
  }

  const levels = new Map()
  const queue = nodes
    .filter(node => incomingCounts.get(node.id) === 0)
    .map(node => node.id)
    .sort((left, right) => left === startNodeID ? -1 : right === startNodeID ? 1 : left.localeCompare(right))
  for (const nodeID of queue) levels.set(nodeID, 0)

  while (queue.length > 0) {
    const source = queue.shift()
    const nextLevel = (levels.get(source) || 0) + 1
    for (const target of outgoingBySource.get(source) || []) {
      levels.set(target, Math.max(levels.get(target) || 0, nextLevel))
      incomingCounts.set(target, (incomingCounts.get(target) || 0) - 1)
      if (incomingCounts.get(target) === 0) queue.push(target)
    }
  }

  let fallbackLevel = Math.max(0, ...levels.values()) + 1
  for (const node of nodes) {
    if (levels.has(node.id)) continue
    levels.set(node.id, fallbackLevel)
    fallbackLevel += 1
  }
  return levels
}

function workflowParallelLanes(nodes, edges, existingLayout) {
  const nodeByID = new Map(nodes.map(node => [node.id, node]))
  const joinByPairID = new Map(nodes.filter(node => node.type === 'parallel_join').map(node => [node.pair_id, node.id]))
  const outgoingBySource = new Map()
  for (const edge of edges) {
    const outgoing = outgoingBySource.get(edge.source) || []
    outgoing.push(edge)
    outgoingBySource.set(edge.source, outgoing)
  }

  const lanes = new Map()
  for (const split of nodes.filter(node => node.type === 'parallel_split')) {
    const joinID = joinByPairID.get(split.pair_id)
    const branchEdges = [...(outgoingBySource.get(split.id) || [])].sort((left, right) => {
      const leftX = existingLayout.get(left.target)?.x
      const rightX = existingLayout.get(right.target)?.x
      if (Number.isFinite(leftX) && Number.isFinite(rightX) && leftX !== rightX) return leftX - rightX
      return String(left.branch_id || left.target).localeCompare(String(right.branch_id || right.target))
    })
    branchEdges.forEach((branchEdge, index) => {
      const queue = [branchEdge.target]
      const visited = new Set([split.id])
      while (queue.length > 0) {
        const nodeID = queue.shift()
        if (!nodeByID.has(nodeID) || nodeID === joinID || visited.has(nodeID)) continue
        visited.add(nodeID)
        if (!lanes.has(nodeID)) lanes.set(nodeID, { index, count: branchEdges.length })
        queue.push(...(outgoingBySource.get(nodeID) || []).map(edge => edge.target))
      }
    })
  }
  return lanes
}

function layoutWorkOrderWorkflowGraph(graph) {
  const nodes = Array.isArray(graph.nodes) ? graph.nodes : []
  const edges = Array.isArray(graph.edges) ? graph.edges : []
  const existingLayout = new Map((graph.layout?.nodes || []).map(node => [node.id, node]))
  const levels = workflowNodeLevels(nodes, edges, graph.start_node_id)
  const lanes = workflowParallelLanes(nodes, edges, existingLayout)
  const nodesByLevel = new Map()
  for (const node of nodes) {
    const level = levels.get(node.id) || 0
    const levelNodes = nodesByLevel.get(level) || []
    levelNodes.push(node)
    nodesByLevel.set(level, levelNodes)
  }

  const centerX = 350
  const horizontalGap = 300
  const topY = 50
  const verticalGap = 150
  const layoutNodes = []
  for (const [level, levelNodes] of [...nodesByLevel.entries()].sort((left, right) => left[0] - right[0])) {
    levelNodes.sort((left, right) => {
      const leftX = existingLayout.get(left.id)?.x
      const rightX = existingLayout.get(right.id)?.x
      if (Number.isFinite(leftX) && Number.isFinite(rightX) && leftX !== rightX) return leftX - rightX
      return left.id.localeCompare(right.id)
    })
    levelNodes.forEach((node, index) => {
      const lane = lanes.get(node.id)
      const count = lane?.count || levelNodes.length
      const positionIndex = lane?.index ?? index
      layoutNodes.push({
        id: node.id,
        x: Math.round(centerX + (positionIndex - (count - 1) / 2) * horizontalGap),
        y: topY + level * verticalGap
      })
    })
  }
  return { ...graph, layout: { ...(graph.layout || {}), nodes: layoutNodes } }
}

export function removeWorkOrderWorkflowNode(graph, nodeID) {
  if (graph?.schema_version !== 2) return graph
  let nodes = Array.isArray(graph.nodes) ? graph.nodes.map(node => ({ ...node })) : []
  let edges = Array.isArray(graph.edges) ? graph.edges.map(edge => ({ ...edge })) : []
  const targetNode = nodes.find(node => node.id === nodeID)
  if (!targetNode || targetNode.type === 'start' || targetNode.type === 'end') return layoutWorkOrderWorkflowGraph(normalizeWorkOrderWorkflowGraph({ ...graph, nodes, edges }))

  const incoming = edges.filter(edge => edge.target === nodeID)
  const outgoing = edges.filter(edge => edge.source === nodeID)
  nodes = nodes.filter(node => node.id !== nodeID)
  edges = edges.filter(edge => edge.source !== nodeID && edge.target !== nodeID)
  if (incoming.length === 1 && outgoing.length === 1) {
    const incomingNode = nodes.find(node => node.id === incoming[0].source)
    const outgoingNode = nodes.find(node => node.id === outgoing[0].target)
    const removesWholeParallelBranch = incomingNode?.type === 'parallel_split' && outgoingNode?.type === 'parallel_join' && incomingNode.pair_id === outgoingNode.pair_id
    if (!removesWholeParallelBranch) {
      appendWorkflowEdge(edges, incoming[0].source, outgoing[0].target, incoming[0].branch_id || '')
    }
  }

  const collapsed = collapseDegenerateParallelGateways(nodes, edges)
  const normalized = normalizeWorkOrderWorkflowGraph({ ...graph, nodes: collapsed.nodes, edges: collapsed.edges })
  return layoutWorkOrderWorkflowGraph(normalized)
}

export function normalizeWorkOrderWorkflowGraph(graph) {
  if (graph?.schema_version !== 2) return graph
  const nodes = Array.isArray(graph.nodes) ? graph.nodes.map(node => ({ ...node })) : []
  const edges = Array.isArray(graph.edges) ? graph.edges.map(edge => ({ ...edge })) : []

  for (const split of nodes.filter(node => node.type === 'parallel_split')) {
    const usedBranchIDs = new Set()
    const preservedEdges = new Set()
    for (const edge of edges.filter(edge => edge.source === split.id)) {
      const branchID = typeof edge.branch_id === 'string' ? edge.branch_id.trim() : ''
      if (isWorkflowIdentifier(branchID) && !usedBranchIDs.has(branchID)) {
        edge.branch_id = branchID
        usedBranchIDs.add(branchID)
        preservedEdges.add(edge)
      }
    }
    let nextBranchNumber = 1
    for (const edge of edges.filter(edge => edge.source === split.id)) {
      if (preservedEdges.has(edge)) continue
      let branchID
      do {
        branchID = `branch_${nextBranchNumber}`
        nextBranchNumber += 1
      } while (usedBranchIDs.has(branchID))
      edge.branch_id = branchID
      usedBranchIDs.add(branchID)
    }
  }

  for (const node of nodes) {
    if (node.type !== 'user_task' || node.task_kind !== 'approve') continue
    const candidates = workOrderWorkflowRejectTargetNodes(node.id, nodes, edges)
    if (!candidates.some(candidate => candidate.id === node.reject_target_node_id)) {
      node.reject_target_node_id = candidates[0]?.id || ''
    }
  }
  return { ...graph, nodes, edges }
}
