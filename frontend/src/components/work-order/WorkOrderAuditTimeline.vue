<template>
  <section class="flow-trace" aria-live="polite">
    <div class="flow-trace__section-head">
      <div>
        <h6 class="mb-1">{{ text('trajectory') }}</h6>
        <p class="mb-0 small text-body-secondary">{{ text('trajectoryHint') }}</p>
      </div>
      <span class="badge rounded-pill text-bg-light border text-body-secondary">{{ trace.steps.length }} {{ text('stages') }}</span>
    </div>

    <div v-if="trace.steps.length" class="flow-trace__path" role="list">
      <template v-for="(step, index) in trace.steps" :key="step.id">
        <div v-if="index" class="flow-trace__connector" aria-hidden="true"><i class="bi bi-arrow-down"></i></div>
        <div v-if="step.type === 'parallel'" class="flow-trace__parallel" role="listitem">
          <div class="flow-trace__parallel-head">
            <span class="flow-trace__parallel-icon"><i class="bi bi-diagram-3"></i></span>
            <div><strong>{{ text('parallelStage') }}</strong><div class="small text-body-secondary">{{ text('parallelHint') }}</div></div>
            <span class="ms-auto badge rounded-pill" :class="statusClass(step.status)">{{ statusText(step.status) }}</span>
          </div>
          <div class="flow-trace__branches" :style="{ '--branch-count': Math.max(step.branches.length, 1) }">
            <article v-for="branch in step.branches" :key="branch.id" class="flow-trace__branch">
              <div class="flow-trace__branch-label"><span>{{ branch.label }}</span></div>
              <template v-for="(branchStep, branchIndex) in branch.steps" :key="branchStep.id">
                <div v-if="branchIndex" class="flow-trace__branch-connector" aria-hidden="true"><i class="bi bi-arrow-down-short"></i></div>
                <FlowNodeCard :step="branchStep" :locale="locale" />
              </template>
              <div v-if="!branch.steps.length" class="flow-trace__empty-branch small text-body-secondary">{{ text('emptyBranch') }}</div>
            </article>
          </div>
          <div class="flow-trace__join" aria-hidden="true"><span><i class="bi bi-chevron-double-down"></i></span></div>
        </div>
        <FlowNodeCard v-else :step="step" :locale="locale" role="listitem" />
      </template>
    </div>
    <div v-else class="flow-trace__empty text-body-secondary small">
      <i class="bi bi-signpost-split me-2"></i>{{ text('noTrajectory') }}
    </div>

    <div class="flow-trace__audit">
      <div class="flow-trace__section-head mb-3">
        <div><h6 class="mb-1">{{ text('activity') }}</h6><p class="mb-0 small text-body-secondary">{{ text('activityHint') }}</p></div>
        <span class="small text-body-secondary">{{ trace.audit.length }} {{ text('records') }}</span>
      </div>
      <div v-if="!trace.audit.length" class="text-body-secondary small">{{ text('noActivity') }}</div>
      <ol v-else class="flow-trace__audit-list">
        <li v-for="item in trace.audit" :key="item.event?.ID || item.event?.id || item.event?.sequence" class="flow-trace__audit-item">
          <span class="flow-trace__audit-dot" aria-hidden="true"></span>
          <div class="flow-trace__audit-card">
            <div class="d-flex flex-wrap align-items-center gap-2">
              <strong class="small">{{ item.label }}</strong>
              <span v-if="item.branchLabel" class="badge rounded-pill flow-trace__branch-badge">{{ item.branchLabel }}</span>
              <span v-if="item.nodeName" class="badge rounded-pill text-bg-light border text-body-secondary">{{ item.nodeName }}</span>
            </div>
            <div class="small text-body-secondary mt-1"><i class="bi bi-person me-1"></i>{{ item.actor }}<span class="mx-2">·</span>{{ formatTime(item.event?.CreatedAt || item.event?.created_at) }}</div>
            <div v-if="item.transferRecipient" class="small text-body-secondary mt-1"><i class="bi bi-arrow-right me-1"></i>{{ item.transferRecipient }}</div>
            <p v-if="item.payload?.comment" class="flow-trace__comment small mb-0 mt-2">{{ item.payload.comment }}</p>
          </div>
        </li>
      </ol>
    </div>
  </section>
</template>

<script setup>
import { computed, defineComponent, h } from 'vue'
import { buildWorkOrderFlowTrace } from '../../utils/workOrderUi.js'
import { formatDateTime } from '../../utils/dateTime.js'

const props = defineProps({
  workflow: { type: Object, default: () => ({}) },
  tasks: { type: Array, default: () => [] },
  events: { type: Array, default: () => [] },
  locale: { type: String, default: 'zh' }
})

const copy = {
  zh: { trajectory: '流程轨迹', trajectoryHint: '按流程结构展示当前进度，并行任务会展开为独立分支。', stages: '个阶段', parallelStage: '并行处理', parallelHint: '所有分支在此同时推进，完成后继续汇聚。', emptyBranch: '此分支没有业务节点', noTrajectory: '此工单没有可展示的图形化流程', activity: '操作记录', activityHint: '按发生顺序保留完整流转证据。', records: '条记录', noActivity: '暂无操作记录', completed: '已完成', active: '处理中', rejected: '已驳回', waiting: '未开始', assignee: '处理人' },
  en: { trajectory: 'Flow trace', trajectoryHint: 'Progress follows the workflow structure; parallel tasks are expanded into branches.', stages: 'stages', parallelStage: 'Parallel stage', parallelHint: 'All branches progress together and merge before the next stage.', emptyBranch: 'No business step in this branch', noTrajectory: 'No visual workflow is available for this work order', activity: 'Activity log', activityHint: 'A complete chronological record of every transition.', records: 'records', noActivity: 'No activity yet', completed: 'Completed', active: 'In progress', rejected: 'Rejected', waiting: 'Not started', assignee: 'Assignee' }
}

const lang = computed(() => String(props.locale || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh')
const text = key => copy[lang.value][key] || key
const trace = computed(() => buildWorkOrderFlowTrace(props.workflow, props.tasks, props.events, lang.value))
const statusText = status => text(status || 'waiting')
const statusClass = status => ({ completed: 'text-bg-success', active: 'text-bg-primary', rejected: 'text-bg-danger', waiting: 'text-bg-secondary' })[status] || 'text-bg-secondary'
const formatTime = value => formatDateTime(value)

const FlowNodeCard = defineComponent({
  props: { step: { type: Object, required: true }, locale: { type: String, default: 'zh' } },
  setup(nodeProps) {
    return () => {
      const step = nodeProps.step
      const node = step.node || {}
      const icon = node.type === 'start' ? 'bi-play-fill' : node.type === 'end' ? 'bi-flag-fill' : node.task_kind === 'approve' ? 'bi-patch-check' : node.type === 'cc_task' ? 'bi-send' : 'bi-person-workspace'
      const metadata = []
      if (step.people?.length) metadata.push(h('span', { class: 'flow-trace__node-meta' }, [h('i', { class: 'bi bi-people me-1' }), step.people.join('、')]))
      if (step.time) metadata.push(h('span', { class: 'flow-trace__node-meta' }, [h('i', { class: 'bi bi-clock me-1' }), formatTime(step.time)]))
      return h('article', { class: ['flow-trace__node', `flow-trace__node--${step.status || 'waiting'}`] }, [
        h('span', { class: 'flow-trace__node-icon' }, [h('i', { class: `bi ${icon}` })]),
        h('div', { class: 'flow-trace__node-body' }, [
          h('div', { class: 'd-flex align-items-start gap-2' }, [h('strong', { class: 'flow-trace__node-title' }, step.label || node.name || (lang.value === 'en' ? 'Unnamed step' : '未命名节点')), h('span', { class: ['badge rounded-pill ms-auto', statusClass(step.status)] }, statusText(step.status))]),
          metadata.length ? h('div', { class: 'flow-trace__node-metadata' }, metadata) : null,
          step.comment ? h('p', { class: 'flow-trace__comment small mb-0 mt-2' }, step.comment) : null
        ])
      ])
    }
  }
})
</script>

<style scoped>
.flow-trace {
  --trace-line: color-mix(in srgb, var(--bs-primary) 34%, var(--bs-border-color));
  --trace-surface: color-mix(in srgb, var(--bs-body-bg) 94%, var(--bs-primary) 6%);
  --trace-muted: color-mix(in srgb, var(--bs-body-bg) 96%, var(--bs-secondary) 4%);
}
.flow-trace__section-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; }
.flow-trace__path { margin-top: 1rem; }
.flow-trace__connector { display: grid; height: 1.9rem; place-items: center; color: var(--trace-line); }
.flow-trace__node { display: flex; gap: .75rem; width: min(100%, 30rem); margin-inline: auto; padding: .8rem; border: 1px solid var(--bs-border-color); border-left: 3px solid var(--bs-secondary); border-radius: .7rem; background: var(--bs-body-bg); box-shadow: 0 .25rem .8rem rgba(15, 23, 42, .045); }
.flow-trace__node--completed { border-left-color: var(--bs-success); }
.flow-trace__node--active { border-left-color: var(--bs-primary); box-shadow: 0 0 0 2px color-mix(in srgb, var(--bs-primary) 13%, transparent); }
.flow-trace__node--rejected { border-left-color: var(--bs-danger); }
.flow-trace__node-icon { display: grid; flex: 0 0 2rem; height: 2rem; place-items: center; border-radius: .55rem; background: var(--trace-surface); color: var(--bs-primary); }
.flow-trace__node-body { min-width: 0; flex: 1; }
.flow-trace__node-title { line-height: 1.35; }
.flow-trace__node-metadata { display: flex; flex-wrap: wrap; gap: .3rem .75rem; margin-top: .35rem; color: var(--bs-secondary-color); font-size: .75rem; }
.flow-trace__parallel { overflow: hidden; border: 1px solid var(--trace-line); border-radius: .85rem; background: var(--trace-muted); }
.flow-trace__parallel-head { display: flex; align-items: center; gap: .65rem; padding: .75rem .9rem; border-bottom: 1px solid var(--trace-line); background: var(--trace-surface); }
.flow-trace__parallel-icon { display: grid; width: 2rem; height: 2rem; place-items: center; border-radius: 50%; background: var(--bs-primary); color: white; }
.flow-trace__branches { display: grid; grid-template-columns: repeat(var(--branch-count), minmax(12rem, 1fr)); gap: .75rem; overflow-x: auto; padding: 1rem .75rem .75rem; }
.flow-trace__branch { position: relative; min-width: 12rem; padding: 0 .55rem .75rem; border-top: 2px solid var(--trace-line); }
.flow-trace__branch::before { position: absolute; top: -2px; left: 50%; width: 1px; height: 1rem; background: var(--trace-line); content: ''; }
.flow-trace__branch-label { display: flex; justify-content: center; padding: .55rem 0 .65rem; }
.flow-trace__branch-label span, .flow-trace__branch-badge { border: 1px solid var(--trace-line); background: var(--trace-surface); color: var(--bs-primary-text-emphasis); font-size: .72rem; }
.flow-trace__branch .flow-trace__node { height: calc(100% - 2.3rem); min-height: 6rem; }
.flow-trace__branch-connector { display: grid; height: 1.3rem; place-items: center; color: var(--trace-line); }
.flow-trace__empty-branch { padding: 1rem; border: 1px dashed var(--bs-border-color); border-radius: .6rem; text-align: center; }
.flow-trace__join { display: flex; justify-content: center; height: 1.8rem; border-top: 1px solid var(--trace-line); color: var(--bs-primary); }
.flow-trace__join span { display: grid; width: 2.2rem; place-items: center; background: var(--trace-surface); }
.flow-trace__empty { margin-top: 1rem; padding: 1rem; border: 1px dashed var(--bs-border-color); border-radius: .7rem; text-align: center; }
.flow-trace__audit { margin-top: 1.5rem; padding-top: 1.25rem; border-top: 1px solid var(--bs-border-color); }
.flow-trace__audit-list { margin: 0; padding: 0; list-style: none; }
.flow-trace__audit-item { position: relative; margin-left: .35rem; padding: 0 0 1rem 1.4rem; border-left: 1px solid var(--trace-line); }
.flow-trace__audit-item:last-child { padding-bottom: 0; border-left-color: transparent; }
.flow-trace__audit-dot { position: absolute; top: .85rem; left: -.32rem; width: .62rem; height: .62rem; border: 2px solid var(--bs-body-bg); border-radius: 50%; background: var(--bs-primary); box-shadow: 0 0 0 1px var(--bs-primary); }
.flow-trace__audit-card { padding: .65rem .75rem; border: 1px solid var(--bs-border-color); border-radius: .65rem; background: var(--bs-body-bg); }
.flow-trace__comment { padding: .5rem .65rem; border-radius: .45rem; background: var(--trace-muted); overflow-wrap: anywhere; }
:global([data-bs-theme='dark']) .flow-trace { --trace-surface: #202631; --trace-muted: #1b2028; }
@media (max-width: 767px) { .flow-trace__section-head { align-items: flex-start; } .flow-trace__branches { grid-template-columns: 1fr; } .flow-trace__branch { min-width: 0; } }
</style>
