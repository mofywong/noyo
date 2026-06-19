<template>
  <section class="editable-rule-graph">
    <div class="graph-column graph-column--trigger">
      <GraphSectionHeader :icon="'bi-lightning-charge'" :kicker="labels.when" :title="labels.triggers" />
      <div class="graph-node-list">
        <GraphNode
          v-for="trigger in draft.triggers"
          :key="trigger.id"
          :node="trigger"
          kind="trigger"
          :selected="isSelected('trigger', trigger.id)"
          :title="triggerTitle(trigger)"
          :detail="triggerDetail(trigger)"
          :badges="[trigger.type]"
          @select="$emit('select', { kind: 'trigger', id: trigger.id })"
          @delete="$emit('delete', { kind: 'trigger', id: trigger.id })"
        />
        <EmptyNode v-if="!draft.triggers.length" :label="labels.emptyTrigger" />
        <DropInsert
          :label="labels.insertHere"
          kind="trigger"
          :after-id="lastId(draft.triggers)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
        />
      </div>
      <FlowArrow :label="labels.conditions" />
    </div>

    <div class="graph-column graph-column--condition">
      <GraphSectionHeader :icon="'bi-filter-circle'" :kicker="labels.ifText" :title="labels.conditions" />
      <ConditionGroup
        v-if="rootConditionGroup"
        :group="rootConditionGroup"
        :selected="selected"
        :labels="labels"
        :describe-condition="conditionDetail"
        @select="$emit('select', $event)"
        @delete="$emit('delete', $event)"
        @insert="$emit('insert', $event)"
        @drop-node="$emit('drop-node', $event)"
        @toggle-logic="$emit('toggle-logic', $event)"
      />
      <FlowArrow :label="labels.time" />
    </div>

    <div class="graph-column graph-column--time">
      <GraphSectionHeader :icon="'bi-calendar2-check'" :kicker="labels.valid" :title="labels.time" />
      <GraphNode
        :node="draft.effectiveTime"
        kind="effective_time"
        :selected="isSelected('effective_time', 'effective_time')"
        :title="effectiveTitle"
        :detail="effectiveDetail"
        :badges="[draft.effectiveTime.mode || 'always']"
        :deletable="false"
        @select="$emit('select', { kind: 'effective_time', id: 'effective_time' })"
      />
      <FlowArrow :label="labels.actions" />
    </div>

    <div class="graph-column graph-column--action">
      <GraphSectionHeader :icon="'bi-play-circle'" :kicker="labels.then" :title="labels.actions" />
      <div class="graph-node-list">
        <ActionNode
          v-for="action in draft.actions"
          :key="action.id"
          :action="action"
          :selected="selected"
          :labels="labels"
          :describe-action="actionDetail"
          @select="$emit('select', $event)"
          @delete="$emit('delete', $event)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
          @add-child="$emit('add-child', $event)"
        />
        <EmptyNode v-if="!draft.actions.length" :label="labels.emptyAction" />
        <DropInsert
          :label="labels.insertHere"
          kind="action"
          :after-id="lastId(draft.actions)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
        />
      </div>
    </div>
  </section>
</template>

<script>
const GraphSectionHeader = {
  props: ['icon', 'kicker', 'title'],
  template: `
    <header class="graph-section-header">
      <span class="graph-section-icon"><i class="bi" :class="icon"></i></span>
      <span>
        <span class="graph-kicker">{{ kicker }}</span>
        <strong>{{ title }}</strong>
      </span>
    </header>
  `
}

const EmptyNode = {
  props: ['label'],
  template: '<div class="graph-node graph-node--empty">{{ label }}</div>'
}

const GraphNode = {
  props: {
    node: Object,
    kind: String,
    selected: Boolean,
    title: String,
    detail: String,
    badges: Array,
    deletable: {
      type: Boolean,
      default: true
    }
  },
  emits: ['select', 'delete'],
  template: `
    <article
      class="graph-node"
      :class="['graph-node--' + kind, { 'is-selected': selected }]"
      :data-graph-kind="kind"
      :data-graph-id="node?.id || kind"
      @click="$emit('select')"
    >
      <div class="graph-node__top">
        <div class="graph-node__title">{{ title }}</div>
        <button v-if="deletable" type="button" class="graph-icon-btn" @click.stop="$emit('delete')" :title="'Delete'">
          <i class="bi bi-x-lg"></i>
        </button>
      </div>
      <div class="graph-node__detail">{{ detail || '-' }}</div>
      <div class="graph-node__badges" v-if="badges && badges.length">
        <span v-for="badge in badges" :key="badge" class="graph-badge">{{ badge }}</span>
      </div>
    </article>
  `
}

const DropInsert = {
  props: ['label', 'kind', 'afterId'],
  emits: ['insert', 'drop-node'],
  methods: {
    onDrop(event) {
      event.preventDefault()
      const text = event.dataTransfer.getData('application/json')
      if (!text) return
      try {
        const item = JSON.parse(text)
        this.$emit('drop-node', { targetKind: this.kind, afterId: this.afterId, item })
      } catch (_) {
        // Ignore malformed external drops.
      }
    }
  },
  template: `
    <button
      type="button"
      class="graph-insert"
      @click="$emit('insert', { kind, afterId })"
      @dragover.prevent
      @drop="onDrop"
    >
      <i class="bi bi-plus-lg"></i>
      <span>{{ label }}</span>
    </button>
  `
}

const FlowArrow = {
  props: ['label'],
  template: `
    <div class="graph-flow-arrow" aria-hidden="true">
      <span>{{ label }}</span>
      <i class="bi bi-arrow-right"></i>
    </div>
  `
}

const ConditionGroup = {
  name: 'ConditionGroup',
  components: { GraphNode, DropInsert },
  props: ['group', 'selected', 'labels', 'describeCondition'],
  emits: ['select', 'delete', 'insert', 'drop-node', 'toggle-logic'],
  methods: {
    isSelected(kind, id) {
      return this.selected?.kind === kind && this.selected?.id === id
    },
    conditionTitle(condition) {
      return this.labels.conditionType?.[condition.type] || condition.type || this.labels.condition
    },
    lastId(list) {
      return list?.length ? list[list.length - 1].id : this.group.id
    }
  },
  template: `
    <article
      class="condition-group"
      :class="{ 'is-selected': isSelected('condition_group', group.id) }"
      data-graph-kind="condition_group"
      :data-graph-id="group.id"
      @click.stop="$emit('select', { kind: 'condition_group', id: group.id })"
    >
      <div class="condition-group__head">
        <div>
          <strong>{{ group.name || labels.conditionGroup }}</strong>
          <span class="graph-badge">{{ String(group.logic || 'and').toUpperCase() }}</span>
        </div>
        <button type="button" class="btn btn-sm btn-outline-secondary" @click.stop="$emit('toggle-logic', group.id)">
          {{ labels.toggleLogic }}
        </button>
      </div>
      <div class="graph-node-list">
        <GraphNode
          v-for="condition in group.conditions"
          :key="condition.id"
          :node="condition"
          kind="condition"
          :selected="isSelected('condition', condition.id)"
          :title="conditionTitle(condition)"
          :detail="describeCondition(condition)"
          :badges="[condition.operator || condition.type]"
          @select="$emit('select', { kind: 'condition', id: condition.id })"
          @delete="$emit('delete', { kind: 'condition', id: condition.id })"
        />
        <ConditionGroup
          v-for="child in group.groups"
          :key="child.id"
          :group="child"
          :selected="selected"
          :labels="labels"
          :describe-condition="describeCondition"
          @select="$emit('select', $event)"
          @delete="$emit('delete', $event)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
          @toggle-logic="$emit('toggle-logic', $event)"
        />
        <DropInsert
          :label="labels.insertHere"
          kind="condition"
          :after-id="lastId(group.conditions)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
        />
      </div>
    </article>
  `
}

const ActionNode = {
  name: 'ActionNode',
  components: { GraphNode, DropInsert },
  props: ['action', 'selected', 'labels', 'describeAction'],
  emits: ['select', 'delete', 'insert', 'drop-node', 'add-child'],
  methods: {
    isSelected(kind, id) {
      return this.selected?.kind === kind && this.selected?.id === id
    },
    actionTitle(action) {
      return this.labels.actionType?.[action.type] || action.type || this.labels.action
    },
    canHaveChildren(action) {
      return action.type === 'sequence_group' || action.type === 'parallel_group'
    },
    lastId(list) {
      return list?.length ? list[list.length - 1].id : this.action.id
    }
  },
  template: `
    <div class="action-branch">
      <GraphNode
        :node="action"
        kind="action"
        :selected="isSelected('action', action.id)"
        :title="action.name || actionTitle(action)"
        :detail="describeAction(action)"
        :badges="[action.type]"
        @select="$emit('select', { kind: 'action', id: action.id })"
        @delete="$emit('delete', { kind: 'action', id: action.id })"
      />
      <div v-if="canHaveChildren(action)" class="action-children">
        <button type="button" class="btn btn-sm btn-outline-primary mb-2" @click="$emit('add-child', action.id)">
          <i class="bi bi-node-plus me-1"></i>{{ labels.addChildAction }}
        </button>
        <ActionNode
          v-for="child in action.children"
          :key="child.id"
          :action="child"
          :selected="selected"
          :labels="labels"
          :describe-action="describeAction"
          @select="$emit('select', $event)"
          @delete="$emit('delete', $event)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
          @add-child="$emit('add-child', $event)"
        />
        <DropInsert
          :label="labels.insertHere"
          kind="action"
          :after-id="lastId(action.children)"
          @insert="$emit('insert', $event)"
          @drop-node="$emit('drop-node', $event)"
        />
      </div>
      <DropInsert
        :label="labels.insertHere"
        kind="action"
        :after-id="action.id"
        @insert="$emit('insert', $event)"
        @drop-node="$emit('drop-node', $event)"
      />
    </div>
  `
}

export default {
  name: 'RuleGraphCanvas',
  components: { GraphSectionHeader, GraphNode, EmptyNode, DropInsert, FlowArrow, ConditionGroup, ActionNode },
  props: {
    draft: {
      type: Object,
      required: true
    },
    selected: {
      type: Object,
      default: null
    },
    labels: {
      type: Object,
      required: true
    },
    devices: {
      type: Array,
      default: () => []
    }
  },
  emits: ['select', 'insert', 'drop-node', 'delete', 'toggle-logic', 'add-child'],
  computed: {
    rootConditionGroup() {
      return this.draft.conditionGroups?.[0] || null
    },
    effectiveTitle() {
      return this.labels.effectiveMode?.[this.draft.effectiveTime?.mode] || this.labels.time
    },
    effectiveDetail() {
      const time = this.draft.effectiveTime || {}
      if (time.mode === 'always') return this.labels.effectiveAlways
      const windows = time.windows || []
      return windows.map(window => `${window.startTime || '00:00:00'} - ${window.endTime || '24:00:00'}`).join(', ') || this.labels.effectiveCustom
    }
  },
  methods: {
    isSelected(kind, id) {
      return this.selected?.kind === kind && this.selected?.id === id
    },
    lastId(list) {
      return list?.length ? list[list.length - 1].id : null
    },
    findDevice(code) {
      return this.devices.find(device => device.code === code)
    },
    optionKey(item) {
      return item?.identifier || item?.code || item?.id || item?.name || ''
    },
    optionLabel(item) {
      const key = this.optionKey(item)
      return item?.name ? `${item.name} (${key})` : key
    },
    deviceName(code) {
      if (!code) return this.labels.selectDevice
      const device = this.findDevice(code)
      return device ? `${device.name || device.code} (${device.code})` : code
    },
    propertyName(deviceCode, propertyKey) {
      if (!propertyKey) return this.labels.selectProperty
      const prop = this.findDevice(deviceCode)?.properties?.find(item => this.optionKey(item) === propertyKey)
      return prop ? this.optionLabel(prop) : propertyKey
    },
    serviceName(deviceCode, serviceCode) {
      if (!serviceCode) return this.labels.selectService
      const service = this.findDevice(deviceCode)?.services?.find(item => this.optionKey(item) === serviceCode)
      return service ? this.optionLabel(service) : serviceCode
    },
    triggerTitle(trigger) {
      return this.labels.triggerType?.[trigger.type] || trigger.type || this.labels.trigger
    },
    triggerDetail(trigger) {
      if (trigger.type === 'cron') return trigger.cronExpr || '-'
      if (trigger.type === 'event') return `${this.deviceName(trigger.deviceCode)} / ${trigger.eventId || this.labels.event}`
      if (trigger.type === 'device_status') return `${this.deviceName(trigger.deviceCode)} ${trigger.statusValue || ''}`
      return `${this.deviceName(trigger.deviceCode)} / ${this.propertyName(trigger.deviceCode, trigger.propertyKey)} ${trigger.operator || ''} ${trigger.value || ''}`.trim()
    },
    conditionDetail(condition) {
      if (condition.type === 'time') return `${condition.startTime || '00:00:00'} - ${condition.endTime || '24:00:00'}`
      if (condition.type === 'device_status') return `${this.deviceName(condition.deviceCode)} ${condition.statusValue || ''}`
      return `${this.deviceName(condition.deviceCode)} / ${this.propertyName(condition.deviceCode, condition.propertyKey)} ${condition.operator || ''} ${condition.value || ''}`.trim()
    },
    actionDetail(action) {
      if (action.type === 'call_service') return `${this.deviceName(action.deviceCode)} / ${this.serviceName(action.deviceCode, action.serviceCode)}`
      if (action.type === 'notification') return action.notifyTitle || action.notifyContent || this.labels.notification
      if (action.type === 'alarm') return `${action.alarmLevel || ''} ${action.alarmTitle || action.alarmContent || ''}`.trim()
      if (action.type === 'delay') return `${action.delaySec || 1}s`
      if (action.type === 'sequence_group' || action.type === 'parallel_group') return `${(action.children || []).length} ${this.labels.childActions}`
      return `${this.deviceName(action.deviceCode)} / ${this.propertyName(action.deviceCode, action.propertyKey)} = ${action.value || ''}`.trim()
    }
  }
}
</script>

<style scoped>
.editable-rule-graph {
  display: grid;
  grid-template-columns: repeat(4, minmax(13rem, 1fr));
  gap: 0.9rem;
  align-items: stretch;
  min-width: 980px;
  padding: 1rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  background:
    linear-gradient(90deg, rgba(var(--bs-primary-rgb), 0.06) 0 1px, transparent 1px 100%),
    linear-gradient(0deg, rgba(var(--bs-primary-rgb), 0.04) 0 1px, transparent 1px 100%),
    var(--bs-body-bg);
  background-size: 2.5rem 2.5rem;
}

.graph-column {
  position: relative;
  display: grid;
  align-content: start;
  gap: 0.75rem;
  min-width: 0;
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  padding: 0.85rem;
  background: rgba(var(--bs-body-bg-rgb), 0.96);
}

.graph-section-header {
  display: flex;
  align-items: center;
  gap: 0.65rem;
}

.graph-section-icon {
  flex: 0 0 2rem;
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #fff;
  background: var(--bs-primary);
}

.graph-column--condition .graph-section-icon {
  background: #6f42c1;
}

.graph-column--time .graph-section-icon {
  background: #198754;
}

.graph-column--action .graph-section-icon {
  background: #fd7e14;
}

.graph-kicker {
  display: block;
  color: var(--bs-secondary-color);
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
}

.graph-node-list {
  display: grid;
  gap: 0.65rem;
}

.graph-node {
  border: 1px solid var(--bs-border-color);
  border-left: 4px solid var(--bs-primary);
  border-radius: 8px;
  padding: 0.7rem;
  background: var(--bs-tertiary-bg);
  min-width: 0;
  cursor: pointer;
  transition: border-color 0.16s ease, box-shadow 0.16s ease, background-color 0.16s ease;
}

.graph-node:hover,
.graph-node.is-selected {
  border-color: var(--bs-primary);
  box-shadow: 0 0.35rem 1rem rgba(var(--bs-primary-rgb), 0.1);
  background: var(--bs-body-bg);
}

.graph-node--condition {
  border-left-color: #6f42c1;
}

.graph-node--effective_time {
  border-left-color: #198754;
}

.graph-node--action {
  border-left-color: #fd7e14;
}

.graph-node--empty {
  border-style: dashed;
  border-left-color: var(--bs-secondary-color);
  color: var(--bs-secondary-color);
  background: transparent;
  cursor: default;
}

.graph-node__top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.4rem;
}

.graph-node__title {
  font-weight: 700;
  line-height: 1.3;
  word-break: break-word;
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

.graph-icon-btn {
  width: 1.8rem;
  height: 1.8rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 8px;
  color: var(--bs-secondary-color);
  background: transparent;
}

.graph-icon-btn:hover {
  color: var(--bs-danger);
  background: rgba(var(--bs-danger-rgb), 0.08);
}

.graph-insert {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  width: 100%;
  min-height: 2.2rem;
  border: 1px dashed var(--bs-border-color);
  border-radius: 8px;
  color: var(--bs-secondary-color);
  background: rgba(var(--bs-body-bg-rgb), 0.78);
}

.graph-insert:hover,
.graph-insert:focus {
  border-color: var(--bs-primary);
  color: var(--bs-primary);
  background: rgba(var(--bs-primary-rgb), 0.06);
}

.graph-flow-arrow {
  position: absolute;
  top: 50%;
  right: -1.45rem;
  z-index: 3;
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--bs-border-color);
  border-radius: 999px;
  color: var(--bs-primary);
  background: var(--bs-body-bg);
  transform: translateY(-50%);
}

.graph-flow-arrow span {
  position: absolute;
  bottom: calc(100% + 0.25rem);
  left: 50%;
  transform: translateX(-50%);
  white-space: nowrap;
  color: var(--bs-secondary-color);
  font-size: 0.68rem;
  font-weight: 700;
}

.condition-group {
  display: grid;
  gap: 0.65rem;
  border: 1px solid rgba(111, 66, 193, 0.28);
  border-left: 4px solid #6f42c1;
  border-radius: 8px;
  padding: 0.7rem;
  background: rgba(111, 66, 193, 0.04);
}

.condition-group.is-selected {
  border-color: #6f42c1;
}

.condition-group__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.action-branch {
  display: grid;
  gap: 0.5rem;
}

.action-children {
  position: relative;
  display: grid;
  gap: 0.5rem;
  margin-left: 0.8rem;
  padding-left: 0.75rem;
  border-left: 2px solid var(--bs-border-color);
}

@media (max-width: 1199.98px) {
  .editable-rule-graph {
    grid-template-columns: 1fr;
    min-width: 0;
  }

  .graph-flow-arrow {
    position: static;
    margin: 0 auto -0.4rem;
    transform: rotate(90deg);
  }

  .graph-flow-arrow span {
    display: none;
  }
}
</style>
