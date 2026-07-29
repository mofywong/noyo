<template>
  <div ref="editorRoot" class="work-order-workflow-editor" :class="{ 'is-fullscreen': isFullscreen }">
    <div class="workflow-editor-content" :class="{ 'workflow-editor-content--canvas-only': !showProperties }">
      <div class="workflow-canvas-shell editable-rule-graph" @click="activeEdgeDropdown = null">
        <VueFlow
          v-model:nodes="nodes"
          v-model:edges="edges"
          :default-zoom="1"
          :min-zoom="0.2"
          :max-zoom="4"
          :nodes-draggable="!readonly"
          :nodes-connectable="!readonly"
          @node-click="onNodeClick"
          @pane-click="onPaneClick"
          @connect="onConnect"
        >
          <template #node-custom="props">
            <article v-if="!isParallelGateway(props.data)" class="graph-node" :class="[`graph-node--${props.data.type}`, { 'is-selected': selectedNodeId === props.id }]">
              <div class="graph-node__top">
                <div class="graph-node__title d-flex align-items-center gap-2">
                  <i :class="nodeIcon(props.data)"></i> {{ props.data.name || nodeTypeLabel(props.data.type) }}
                </div>
                <button v-if="!readonly && !['start','end'].includes(props.data.type)" class="graph-node__delete" type="button" title="删除节点" aria-label="删除节点" @click.stop="removeNodeById(props.id)">
                  <i class="bi bi-trash"></i>
                </button>
              </div>
              <div class="graph-node__detail">{{ nodeMeta(props.data) || '-' }}</div>
              <div class="graph-node__badges">
                <span class="graph-badge">{{ nodeTypeLabel(props.data.type) }}</span>
                <span v-if="props.data.type === 'user_task'" class="graph-badge">{{ props.data.completion_mode === 'all' ? '会签' : '或签' }}</span>
              </div>
              
              <Handle v-if="props.data.type !== 'start'" type="target" :position="Position.Top" />
              <Handle v-if="props.data.type !== 'end'" type="source" :position="Position.Bottom" />
            </article>
            <div v-else class="parallel-gateway" aria-hidden="true">
              <Handle type="target" :position="Position.Top" />
              <Handle type="source" :position="Position.Bottom" />
            </div>
          </template>

          <template #edge-custom="props">
            <BaseEdge
              :id="props.id"
              :style="workflowEdgeStyle"
              :path="getSmoothStepPath(props)[0]"
              :marker-end="props.markerEnd"
            />
            <EdgeLabelRenderer v-if="!readonly">
              <div
                :style="{
                  position: 'absolute',
                  transform: `translate(-50%, -50%) translate(${getSmoothStepPath(props)[1]}px, ${getSmoothStepPath(props)[2]}px)`,
                  pointerEvents: 'all'
                }"
                class="edge-insert-wrapper"
              >
                <button class="btn btn-sm btn-light rounded-circle shadow-sm edge-insert-btn" @click.stop="toggleEdgeDropdown(props.id)">
                  <i class="bi bi-plus"></i>
                </button>
                <ul class="dropdown-menu shadow-sm" :class="{ show: activeEdgeDropdown === props.id }" style="position:absolute; top: 100%; left: 50%; transform: translateX(-50%); z-index: 1000;">
                  <li><a class="dropdown-item" href="#" @click.prevent="insertNodeOnEdge(props, 'user_task')"><i class="bi bi-person-gear me-2 text-primary"></i>添加处理任务</a></li>
                  <li><a class="dropdown-item" href="#" @click.prevent="insertNodeOnEdge(props, 'cc_task')"><i class="bi bi-send me-2 text-info"></i>添加抄送节点</a></li>
                  <li v-if="canInsertParallelOnEdge(props.id)"><a class="dropdown-item" href="#" @click.prevent="insertNodeOnEdge(props, 'parallel_split')"><i class="bi bi-diagram-3 me-2 text-purple"></i>添加并行分支</a></li>
                </ul>
              </div>
            </EdgeLabelRenderer>
          </template>

          
          
        </VueFlow>
      </div>

      <aside v-if="showProperties" class="workflow-properties" :class="{ 'is-empty': !selectedNode }">
        <template v-if="selectedNode">
          <div class="d-flex justify-content-between align-items-center mb-3">
            <h6 class="mb-0">节点设置</h6>
            <span class="badge text-bg-light">{{ nodeTypeLabel(selectedNode.type) }}</span>
          </div>
          
          <div v-if="!['start', 'end', 'parallel_split', 'parallel_join'].includes(selectedNode.type)" class="mb-3">
            <label class="form-label small">节点名称</label>
            <input v-model.trim="selectedNode.name" class="form-control form-control-sm" :disabled="readonly">
          </div>

          <template v-if="selectedNode.type === 'user_task' || selectedNode.type === 'cc_task'">
            <template v-if="selectedNode.type === 'user_task'">
              <div class="mb-3">
                <label class="form-label small">任务类型</label>
                <select v-model="selectedNode.task_kind" class="form-select form-select-sm" :disabled="readonly" @change="ensureRejectTarget">
                  <option value="handle">处理任务</option>
                  <option value="approve">审批任务</option>
                </select>
              </div>
              <div class="mb-3">
                <label class="form-label small">完成方式</label>
                <select v-model="selectedNode.completion_mode" class="form-select form-select-sm" :disabled="readonly">
                  <option value="any">任一人完成（或签）</option>
                  <option value="all">全部完成（会签）</option>
                </select>
              </div>
              <div v-if="selectedNode.task_kind === 'approve'" class="mb-3">
                <label class="form-label small">驳回后返回节点</label>
                <select v-model="selectedNode.reject_target_node_id" class="form-select form-select-sm" :disabled="readonly">
                  <option value="">无可返回的前置任务</option>
                  <option v-for="node in rejectTargetNodes" :key="node.id" :value="node.id">{{ node.name || node.id }}</option>
                </select>
              </div>
            </template>
            <div>
              <label class="form-label small d-flex justify-content-between">
                <span>{{ selectedNode.type === 'cc_task' ? '抄送给' : '项目成员' }}</span>
                <span v-if="participantsLoading" class="spinner-border spinner-border-sm"></span>
              </label>
              <div class="participant-picker">
                <div class="p-2 border-bottom sticky-top bg-body" style="z-index: 10;">
                  <input v-model.trim="participantSearchKeyword" type="text" class="form-control form-control-sm" placeholder="搜索成员名称或账号...">
                </div>
                <label v-for="participant in filteredParticipants" :key="participant.id" class="participant-option">
                  <input type="checkbox" :checked="participantSelected(participant.id)" :disabled="readonly" @change="toggleParticipant(participant.id)">
                  <span>{{ participant.display_name || participant.username }}</span>
                  <small>{{ participant.username }}</small>
                </label>
                <div v-if="!participantsLoading && filteredParticipants.length === 0" class="text-muted small py-3 text-center">没有找到匹配的成员。</div>
              </div>
            </div>
          </template>

          <template v-else-if="selectedNode.type === 'end'">
            <div class="mb-3">
              <label class="form-label small">结束结果</label>
              <select v-model="selectedNode.result" class="form-select form-select-sm" :disabled="readonly">
                <option value="completed">已解决</option>
                <option value="closed">已关闭</option>
                <option value="cancelled">已取消</option>
              </select>
            </div>
          </template>
        </template>
        <div v-else class="text-muted small text-center py-5">
          <i class="bi bi-cursor fs-4 d-block mb-2"></i>选择画布中的节点以配置属性。
        </div>
      </aside>
    </div>

    <div v-if="error" class="alert alert-danger py-2 mt-3 mb-0">{{ error }}</div>
  </div>
</template>

<script setup>
import { computed, nextTick, ref, watch, onMounted, onUnmounted } from 'vue'
import { VueFlow, useVueFlow, Position, Handle, BaseEdge, EdgeLabelRenderer, getSmoothStepPath, addEdge } from '@vue-flow/core'
import { canInsertWorkOrderParallelOnEdge, normalizeWorkOrderWorkflowGraph, removeWorkOrderWorkflowNode, workOrderWorkflowRejectTargetNodes } from '../../utils/workOrderWorkflow.js'


import '@vue-flow/core/dist/style.css'
import '@vue-flow/core/dist/theme-default.css'


const props = defineProps({
  modelValue: { type: Object, default: () => ({ schema_version: 2, start_node_id: 'start', nodes: [], edges: [] }) },
  participants: { type: Array, default: () => [] },
  participantsLoading: { type: Boolean, default: false },
  readonly: { type: Boolean, default: false },
  showProperties: { type: Boolean, default: true },
  saving: { type: Boolean, default: false },
  error: { type: String, default: '' }
})
const emit = defineEmits(['update:modelValue'])
const { fitView } = useVueFlow()

const nodes = ref([])
const edges = ref([])
const selectedNodeId = ref('')
const activeEdgeDropdown = ref(null)
const syncing = ref(false)
const lastEmittedSignature = ref('')
const participantSearchKeyword = ref('')
const editorRoot = ref(null)
const isFullscreen = ref(false)
const workflowEdgeStyle = computed(() => ({ stroke: 'var(--workflow-edge-color)', strokeWidth: 2, strokeDasharray: '5,5' }))

const toggleEdgeDropdown = (id) => { activeEdgeDropdown.value = activeEdgeDropdown.value === id ? null : id }

const filteredParticipants = computed(() => {
  if (!participantSearchKeyword.value) return props.participants
  const kw = participantSearchKeyword.value.toLowerCase()
  return props.participants.filter(p => (p.display_name || '').toLowerCase().includes(kw) || (p.username || '').toLowerCase().includes(kw))
})

const selectedNode = computed(() => {
  if (!selectedNodeId.value) return null
  const n = nodes.value.find(n => n.id === selectedNodeId.value)
  return n ? n.data : null
})

const rejectTargetNodes = computed(() => {
  const selectedID = selectedNode.value?.id
  if (!selectedID) return []
  return workOrderWorkflowRejectTargetNodes(selectedID, nodes.value.map(node => node.data), edges.value)
})

watch(() => props.modelValue, (value) => {
  const sig = JSON.stringify(value)
  if (sig === lastEmittedSignature.value) return
  loadFromFlatGraph(value)
}, { deep: true, immediate: true })

watch(nodes, () => { if (!syncing.value) emitGraph() }, { deep: true, flush: 'post' })
watch(edges, () => { if (!syncing.value) emitGraph() }, { deep: true, flush: 'post' })

function getNextId(prefix) {
  return `${prefix}_${Math.random().toString(36).substr(2, 9)}`
}

function loadFromFlatGraph(value) {
  syncing.value = true
  const sourceGraph = value?.schema_version === 2 && Array.isArray(value.nodes) && value.nodes.length ? value : defaultWorkflowGraph()
  const graph = normalizeWorkOrderWorkflowGraph(sourceGraph)
  
  const layout = graph.layout?.nodes || []
  const layoutMap = new Map(layout.map(l => [l.id, l]))

  nodes.value = graph.nodes.map((n, idx) => {
    const l = layoutMap.get(n.id) || { x: 250, y: 50 + idx * 150 }
    return {
      id: n.id,
      type: 'custom',
      position: { x: l.x, y: l.y },
      data: { ...n }
    }
  })
  
  edges.value = graph.edges.map(e => ({
    id: e.id || `edge_${e.source}_${e.target}`,
    source: e.source,
    target: e.target,
    branch_id: e.branch_id || '',
    type: 'custom'
  }))
  
  queueMicrotask(() => { syncing.value = false })
}

function flattenToGraph() {
  ensureAllRejectTargets()
  const flatNodes = nodes.value.map(n => ({ ...n.data }))
  const flatEdges = edges.value.map((e, idx) => ({
    id: e.id || `edge_${idx}`,
    source: e.source,
    target: e.target,
    ...(e.branch_id ? { branch_id: e.branch_id } : {})
  }))
  const layoutNodes = nodes.value.map(n => ({ id: n.id, x: Math.round(n.position.x), y: Math.round(n.position.y) }))
  
  return normalizeWorkOrderWorkflowGraph({
    schema_version: 2,
    start_node_id: nodes.value.find(n => n.data.type === 'start')?.id || '',
    nodes: flatNodes,
    edges: flatEdges,
    layout: { nodes: layoutNodes }
  })
}

function emitGraph() {
  if (syncing.value) return
  const graph = flattenToGraph()
  lastEmittedSignature.value = JSON.stringify(graph)
  emit('update:modelValue', graph)
}

function getWorkflowDefinition() {
  return flattenToGraph()
}
defineExpose({ getWorkflowDefinition, toggleFullscreen })

function syncFullscreen() {
  isFullscreen.value = document.fullscreenElement === editorRoot.value
}

async function toggleFullscreen() {
  if (!editorRoot.value) return
  if (document.fullscreenElement === editorRoot.value) {
    await document.exitFullscreen()
    return
  }
  await editorRoot.value.requestFullscreen()
}

onMounted(() => document.addEventListener('fullscreenchange', syncFullscreen))
onUnmounted(() => document.removeEventListener('fullscreenchange', syncFullscreen))

function defaultWorkflowGraph() {
  return {
    schema_version: 2,
    start_node_id: 'start',
    nodes: [
      { id: 'start', type: 'start', name: '开始' },
      { id: 'handle', type: 'user_task', name: '处理任务', task_kind: 'handle', completion_mode: 'all', participant_user_ids: [] },
      { id: 'end', type: 'end', name: '结束', result: 'completed' }
    ],
    edges: [{ id: 'edge-start-handle', source: 'start', target: 'handle' }, { id: 'edge-handle-end', source: 'handle', target: 'end' }],
    layout: { nodes: [
      { id: 'start', x: 250, y: 50 },
      { id: 'handle', x: 250, y: 200 },
      { id: 'end', x: 250, y: 350 }
    ]}
  }
}


function onConnect(params) {
  if (props.readonly) return
  params.type = 'custom'
  const sourceNode = nodes.value.find(node => node.id === params.source)?.data
  if (sourceNode?.type === 'parallel_split') {
    const usedBranchIDs = new Set(edges.value.filter(edge => edge.source === params.source).map(edge => edge.branch_id).filter(Boolean))
    let branchNumber = 1
    while (usedBranchIDs.has(`branch_${branchNumber}`)) branchNumber += 1
    params.branch_id = `branch_${branchNumber}`
  }
  edges.value = addEdge(params, edges.value)
}

function onNodeClick({ node }) { selectedNodeId.value = node.id; ensureRejectTarget() }
function onPaneClick() { selectedNodeId.value = ''; activeEdgeDropdown.value = null }

function ensureRejectTarget() {
  const node = selectedNode.value
  ensureRejectTargetForNode(node)
}

function ensureAllRejectTargets() {
  nodes.value.forEach(node => ensureRejectTargetForNode(node.data))
}

function ensureRejectTargetForNode(node) {
  if (!node || node.type !== 'user_task' || node.task_kind !== 'approve') return
  const targets = workOrderWorkflowRejectTargetNodes(node.id, nodes.value.map(item => item.data), edges.value)
  if (targets.some(target => target.id === node.reject_target_node_id)) return
  node.reject_target_node_id = targets[0]?.id || ''
}

function canInsertParallelOnEdge(edgeID) {
  return canInsertWorkOrderParallelOnEdge(edgeID, nodes.value.map(item => item.data), edges.value)
}

function isParallelGateway(node) {
  return node?.type === 'parallel_split' || node?.type === 'parallel_join'
}

function removeNodeById(id) {
  const node = nodes.value.find(n => n.id === id)
  if (!node || props.readonly || ['start', 'end'].includes(node.data.type)) return
  const graph = removeWorkOrderWorkflowNode(flattenToGraph(), id)
  loadFromFlatGraph(graph)
  selectedNodeId.value = ''
  queueMicrotask(() => {
    emitGraph()
    nextTick(() => fitView({ padding: 0.2, duration: 200 }))
  })
}

function removeSelectedNode() {
  if (!selectedNode.value) return
  removeNodeById(selectedNode.value.id)
}

function insertNodeOnEdge(edgeProps, type) {
  activeEdgeDropdown.value = null
  const sourceNode = nodes.value.find(n => n.id === edgeProps.source)
  const targetNode = nodes.value.find(n => n.id === edgeProps.target)
  const originalEdge = edges.value.find(edge => edge.id === edgeProps.id)
  const inheritedBranchID = originalEdge?.branch_id || ''
  
  let newY = 200
  let newX = 250
  
  if (sourceNode) {
    newX = sourceNode.position.x
  }

  let newNodes = []
  let newEdges = []

  if (type === 'user_task' || type === 'cc_task') {
    const yShift = 150
    if (sourceNode) {
      newY = sourceNode.position.y + yShift
      nodes.value.forEach(n => {
        if (n.position.y > sourceNode.position.y) {
          n.position.y += yShift
        }
      })
    }
    
    const taskId = getNextId(type === 'cc_task' ? 'cc' : 'task')
    if (type === 'cc_task') {
      newNodes.push({
        id: taskId, type: 'custom', position: { x: newX, y: newY },
        data: { id: taskId, type: 'cc_task', name: '抄送节点', participant_user_ids: [] }
      })
    } else {
      newNodes.push({
        id: taskId, type: 'custom', position: { x: newX, y: newY },
        data: { id: taskId, type: 'user_task', name: '新任务', task_kind: 'handle', completion_mode: 'all', participant_user_ids: [] }
      })
    }
    newEdges.push(
      { id: `edge_${edgeProps.source}_${taskId}`, source: edgeProps.source, target: taskId, branch_id: inheritedBranchID, type: 'custom' },
      { id: `edge_${taskId}_${edgeProps.target}`, source: taskId, target: edgeProps.target, type: 'custom' }
    )
  } else if (type === 'parallel_split') {
    const yShift = 450
    if (sourceNode) {
      newY = sourceNode.position.y + 150
      nodes.value.forEach(n => {
        if (n.position.y > sourceNode.position.y) {
          n.position.y += yShift
        }
      })
    }
    
    const splitId = getNextId('split')
    const joinId = getNextId('join')
    
    newNodes.push(
      { id: splitId, type: 'custom', position: { x: newX, y: newY }, data: { id: splitId, type: 'parallel_split', name: '并行分支', pair_id: splitId } },
      { id: joinId, type: 'custom', position: { x: newX, y: newY + 300 }, data: { id: joinId, type: 'parallel_join', name: '并行汇聚', pair_id: splitId } }
    )

    const branchA = getNextId('task')
    const branchB = getNextId('task')
    newNodes.push(
      { id: branchA, type: 'custom', position: { x: newX - 150, y: newY + 150 }, data: { id: branchA, type: 'user_task', name: '分支A任务', task_kind: 'handle', completion_mode: 'all', participant_user_ids: [] } },
      { id: branchB, type: 'custom', position: { x: newX + 150, y: newY + 150 }, data: { id: branchB, type: 'user_task', name: '分支B任务', task_kind: 'handle', completion_mode: 'all', participant_user_ids: [] } }
    )

    newEdges.push(
      { id: `edge_${edgeProps.source}_${splitId}`, source: edgeProps.source, target: splitId, branch_id: inheritedBranchID, type: 'custom' },
      { id: `edge_${joinId}_${edgeProps.target}`, source: joinId, target: edgeProps.target, type: 'custom' },
      { id: `edge_${splitId}_${branchA}`, source: splitId, target: branchA, branch_id: 'branch_1', type: 'custom' },
      { id: `edge_${splitId}_${branchB}`, source: splitId, target: branchB, branch_id: 'branch_2', type: 'custom' },
      { id: `edge_${branchA}_${joinId}`, source: branchA, target: joinId, type: 'custom' },
      { id: `edge_${branchB}_${joinId}`, source: branchB, target: joinId, type: 'custom' }
    )
  }
  
  nodes.value.push(...newNodes)
  
  // Remove original edge
  edges.value = edges.value.filter(e => e.id !== edgeProps.id)
  edges.value.push(...newEdges)
}

function nodeTypeLabel(type) {
  return ({ start: '开始', user_task: '用户任务', cc_task: '抄送节点', parallel_split: '并行分流', parallel_join: '并行合流', end: '结束' })[type] || type
}

function nodeIcon(node) {
  if (node.type === 'start') return 'bi-play-fill'
  if (node.type === 'end') return 'bi-stop-fill'
  if (node.type === 'cc_task') return 'bi-send'
  if (node.type === 'user_task') return node.task_kind === 'approve' ? 'bi-check2-square' : 'bi-person-gear'
  if (node.type === 'parallel_split' || node.type === 'parallel_join') return 'bi-diagram-3'
  return 'bi-square'
}

function nodeMeta(node) {
  if (node.type === 'user_task') {
    return `${node.task_kind === 'approve' ? '审批' : '处理'} · ${node.completion_mode === 'all' ? '会签' : '或签'}`
  }
  if (node.type === 'cc_task') {
    return `抄送 ${(node.participant_user_ids || []).length} 人`
  }
  if (node.type === 'start') return '流程起点'
  if (node.type === 'end') return node.result === 'completed' ? '已解决' : node.result === 'closed' ? '已关闭' : '已取消'
  return ''
}

function participantSelected(userID) { return (selectedNode.value?.participant_user_ids || []).includes(userID) }
function toggleParticipant(userID) {
  if (!selectedNode.value || props.readonly) return
  const current = new Set(selectedNode.value.participant_user_ids || [])
  if (current.has(userID)) current.delete(userID); else current.add(userID)
  selectedNode.value.participant_user_ids = [...current]
}
</script>

<style scoped>
.work-order-workflow-editor {
  --workflow-canvas-bg: var(--bs-body-bg);
  --workflow-grid-line: rgba(var(--bs-primary-rgb), 0.06);
  --workflow-grid-line-strong: rgba(var(--bs-primary-rgb), 0.1);
  --workflow-node-bg: var(--bs-tertiary-bg);
  --workflow-node-hover-bg: var(--bs-body-bg);
  --workflow-edge-color: color-mix(in srgb, var(--bs-secondary-color) 72%, transparent);
  --workflow-delete-bg: var(--bs-body-bg);
  display: flex;
  flex-direction: column;
  min-width: 0;
  height: 100%;
  min-height: 0;
}

:global([data-bs-theme='dark']) .work-order-workflow-editor {
  --workflow-canvas-bg: #111827;
  --workflow-grid-line: rgba(148, 163, 184, 0.08);
  --workflow-grid-line-strong: rgba(148, 163, 184, 0.12);
  --workflow-node-bg: #1f2937;
  --workflow-node-hover-bg: #243044;
  --workflow-edge-color: rgba(203, 213, 225, 0.74);
  --workflow-delete-bg: #111827;
}

.workflow-editor-content { display: grid; grid-template-columns: minmax(0, 1fr) minmax(232px, 264px); flex: 1 1 auto; gap: 0.75rem; height: min(78vh, 900px); min-height: 620px; }
.workflow-editor-content--canvas-only { grid-template-columns: minmax(0, 1fr); }
.workflow-properties { border: 1px solid var(--bs-border-color); border-radius: 0.5rem; height: 100%; min-height: 0; overflow: auto; padding: 1rem; background: var(--bs-tertiary-bg); }
.workflow-properties.is-empty { display: flex; align-items: center; justify-content: center; }

.workflow-canvas-shell { border: 1px solid var(--bs-border-color); border-radius: 0.5rem; height: 100%; min-height: 0; overflow: hidden; position: relative; }
:deep(.vue-flow) { height: 100%; }

/* Rule Engine Canvas Styles */
:deep(.vue-flow__edge-labels) {
  z-index: 100 !important;
}

.editable-rule-graph {
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  background:
    linear-gradient(90deg, var(--workflow-grid-line-strong) 0 1px, transparent 1px 100%),
    linear-gradient(0deg, var(--workflow-grid-line) 0 1px, transparent 1px 100%),
    var(--workflow-canvas-bg);
  background-size: 2.5rem 2.5rem;
}

.graph-node {
  border: 1px solid var(--bs-border-color);
  border-left: 4px solid var(--bs-primary);
  border-radius: 8px;
  padding: 0.7rem;
  background: var(--workflow-node-bg);
  min-width: 220px;
  width: auto;
  cursor: pointer;
  box-shadow: 0 2px 4px rgba(0,0,0,0.02);
  transition: border-color 0.16s ease, box-shadow 0.16s ease, background-color 0.16s ease;
}

.graph-node:hover,
.graph-node.is-selected {
  border-color: var(--bs-primary);
  box-shadow: 0 0.35rem 1rem rgba(var(--bs-primary-rgb), 0.1);
  background: var(--workflow-node-hover-bg);
}

.graph-node--start, .graph-node--end { border-left-color: #198754; }
.graph-node--user_task { border-left-color: #fd7e14; }
.graph-node--parallel_split, .graph-node--parallel_join { border-left-color: #6f42c1; }
.parallel-gateway { width: 220px; height: 1px; pointer-events: none; }
.parallel-gateway :deep(.vue-flow__handle) { opacity: 0; pointer-events: none; }

.graph-node__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.4rem;
}

.graph-node__title {
  min-width: 0;
  font-weight: 700;
  line-height: 1.3;
  word-break: break-word;
}

.graph-node__delete {
  width: 1.7rem;
  height: 1.7rem;
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  border: 1px solid rgba(var(--bs-danger-rgb), 0.22);
  border-radius: 0.35rem;
  background: var(--workflow-delete-bg);
  color: var(--bs-danger);
  opacity: 0;
  pointer-events: none;
  transform: translateY(-2px);
  transition: opacity 0.14s ease, transform 0.14s ease, background-color 0.14s ease, border-color 0.14s ease;
}

.graph-node:hover .graph-node__delete,
.graph-node:focus-within .graph-node__delete,
.graph-node__delete:focus-visible {
  opacity: 1;
  pointer-events: auto;
  transform: translateY(0);
}

.graph-node__delete:hover,
.graph-node__delete:focus-visible {
  border-color: rgba(var(--bs-danger-rgb), 0.5);
  background: rgba(var(--bs-danger-rgb), 0.1);
}

.graph-node__detail {
  color: var(--bs-secondary-color);
  font-size: 0.82rem;
  line-height: 1.45;
  margin-top: 0.25rem;
  overflow-wrap: anywhere;
}

.graph-node__badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.45rem;
}

.graph-badge {
  display: inline-flex;
  align-items: center;
  min-height: 1.35rem;
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
  background: rgba(var(--bs-primary-rgb), 0.1);
  color: var(--bs-primary);
  font-size: 0.72rem;
  font-weight: 700;
}

.edge-insert-btn {
  width: 24px;
  height: 24px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--bs-border-color);
  color: var(--bs-primary);
  background: var(--bs-body-bg);
  transition: all 0.2s ease;
}

.edge-insert-btn:hover {
  background-color: var(--bs-primary);
  color: white;
  border-color: var(--bs-primary);
}

.text-purple { color: #6f42c1; }

.participant-picker { border: 1px solid var(--bs-border-color); border-radius: 0.375rem; max-height: 300px; overflow: auto; background: var(--bs-body-bg); }
.participant-option { display: grid; grid-template-columns: auto 1fr; gap: 0.15rem 0.5rem; padding: 0.5rem 0.75rem; border-bottom: 1px solid var(--bs-border-color); cursor: pointer; margin: 0; align-items: center; }
.participant-option:last-child { border-bottom: 0; }
.participant-option small { grid-column: 2; color: var(--bs-secondary-color); }

.work-order-workflow-editor:fullscreen { width: 100vw; height: 100vh; box-sizing: border-box; padding: 1rem; overflow: hidden; background: var(--bs-body-bg); }
.work-order-workflow-editor:fullscreen .workflow-editor-content { height: 100%; min-height: 0; }

@media (max-width: 991px) {
  .workflow-editor-content { grid-template-columns: 1fr; height: auto; min-height: 0; }
  .workflow-canvas-shell { height: 560px; min-height: 560px; }
  .workflow-properties { height: auto; min-height: auto; }
}
</style>
