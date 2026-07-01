<template>
  <aside class="rule-graph-inspector">
    <div class="inspector-head">
      <div>
        <div class="inspector-kicker">{{ title }}</div>
        <h6 class="mb-0">{{ heading }}</h6>
      </div>
      <button v-if="canDelete" type="button" class="btn btn-sm btn-outline-danger" @click="$emit('delete', selection)">
        <i class="bi bi-trash"></i>
      </button>
    </div>

    <div v-if="kind === 'meta'" class="inspector-form">
      <FieldInput :label="labels.name" :value="node.name" @input="patchMeta({ name: $event })" />
      <FieldInput :label="labels.description" :value="node.description" @input="patchMeta({ description: $event })" />
      <div class="row g-2">
        <div class="col-6">
          <FieldInput type="number" :label="labels.priority" :value="node.priority" @input="patchMeta({ priority: Number($event) || 1 })" />
        </div>
        <div class="col-6">
          <FieldInput type="number" :label="labels.throttle" :value="node.throttle_sec" @input="patchMeta({ throttle_sec: Number($event) || 0 })" />
        </div>
      </div>
      <div class="row g-2">
        <div class="col-6">
          <FieldInput type="number" :label="labels.maxPerHour" :value="node.max_per_hour" @input="patchMeta({ max_per_hour: Number($event) || 0 })" />
        </div>
        <div class="col-6">
          <FieldInput type="number" :label="labels.retryCount" :value="node.retry_count" @input="patchMeta({ retry_count: Number($event) || 0 })" />
        </div>
      </div>
    </div>

    <div v-else-if="kind === 'trigger'" class="inspector-form">
      <SelectField :label="labels.type" :value="node.type" :options="triggerTypes" @change="patch({ type: $event })" />
      <template v-if="node.type !== 'cron'">
        <SelectField :label="labels.device" :value="node.deviceCode" :options="deviceOptions" @change="patch({ deviceCode: $event, propertyKey: '', eventId: '', serviceCode: '' })" />
        <SelectField v-if="node.type === 'property_change'" :label="labels.property" :value="node.propertyKey" :options="propertyOptions(node.deviceCode)" @change="patch({ propertyKey: $event })" />
        <SelectField v-if="node.type === 'property_change'" :label="labels.operator" :value="node.operator" :options="triggerOperators" @change="patch({ operator: $event })" />
        <FieldInput v-if="node.type === 'property_change' && node.operator !== 'changed'" :label="labels.value" :value="node.value" @input="patch({ value: $event })" />
        <SelectField v-if="node.type === 'event'" :label="labels.event" :value="node.eventId" :options="eventOptions(node.deviceCode)" @change="patch({ eventId: $event })" />
        <SelectField v-if="node.type === 'device_status'" :label="labels.status" :value="node.statusValue" :options="statusOptions" @change="patch({ statusValue: $event })" />
      </template>
      <FieldInput v-else :label="labels.cron" :value="node.cronExpr" @input="patch({ cronExpr: $event })" />
    </div>

    <div v-else-if="kind === 'condition_group'" class="inspector-form">
      <FieldInput :label="labels.name" :value="node.name" @input="patch({ name: $event })" />
      <SelectField :label="labels.logic" :value="node.logic" :options="logicOptions" @change="patch({ logic: $event })" />
    </div>

    <div v-else-if="kind === 'condition'" class="inspector-form">
      <SelectField :label="labels.type" :value="node.type" :options="conditionTypes" @change="patch({ type: $event })" />
      <SelectField :label="labels.device" :value="node.deviceCode" :options="deviceOptions" @change="patch({ deviceCode: $event, propertyKey: '' })" />
      <SelectField v-if="node.type === 'property'" :label="labels.property" :value="node.propertyKey" :options="propertyOptions(node.deviceCode)" @change="patch({ propertyKey: $event })" />
      <SelectField v-if="node.type === 'property'" :label="labels.operator" :value="node.operator" :options="conditionOperators" @change="patch({ operator: $event })" />
      <FieldInput v-if="node.type === 'property'" :label="labels.value" :value="node.value" @input="patch({ value: $event })" />
      <SelectField v-if="node.type === 'device_status'" :label="labels.status" :value="node.statusValue" :options="statusOptions" @change="patch({ statusValue: $event })" />
    </div>

    <div v-else-if="kind === 'effective_time'" class="inspector-form">
      <SelectField :label="labels.mode" :value="node.mode" :options="effectiveModes" @change="patchEffectiveMode($event)" />
      <template v-if="node.mode !== 'always'">
        <div v-for="(window, index) in node.windows || []" :key="index" class="window-row">
          <FieldInput :label="labels.startTime" :value="window.startTime" placeholder="00:00:00" @input="patchWindow(index, { startTime: $event })" />
          <FieldInput :label="labels.endTime" :value="window.endTime" placeholder="24:00:00" @input="patchWindow(index, { endTime: $event })" />
        </div>
        <button type="button" class="btn btn-sm btn-outline-primary" @click="addWindow">
          <i class="bi bi-plus-lg me-1"></i>{{ labels.addWindow }}
        </button>
      </template>
    </div>

    <div v-else-if="kind === 'action'" class="inspector-form">
      <FieldInput v-if="isActionGroup" :label="labels.name" :value="node.name" @input="patch({ name: $event })" />
      <SelectField :label="labels.type" :value="node.type" :options="actionTypes" @change="patch({ type: $event })" />
      <template v-if="node.type === 'set_property'">
        <SelectField :label="labels.device" :value="node.deviceCode" :options="deviceOptions" @change="patch({ deviceCode: $event, propertyKey: '' })" />
        <SelectField :label="labels.property" :value="node.propertyKey" :options="propertyOptions(node.deviceCode)" @change="patch({ propertyKey: $event })" />
        <FieldInput :label="labels.value" :value="node.value" @input="patch({ value: $event })" />
      </template>
      <template v-else-if="node.type === 'call_service'">
        <SelectField :label="labels.device" :value="node.deviceCode" :options="deviceOptions" @change="patch({ deviceCode: $event, serviceCode: '' })" />
        <SelectField :label="labels.service" :value="node.serviceCode" :options="serviceOptions(node.deviceCode)" @change="patch({ serviceCode: $event })" />
        <label class="form-label">{{ labels.params }}</label>
        <textarea class="form-control form-control-sm font-monospace" rows="4" :value="paramsText" @input="patchParams($event.target.value)"></textarea>
      </template>
      <template v-else-if="node.type === 'notification'">
        <FieldInput :label="labels.title" :value="node.notifyTitle" @input="patch({ notifyTitle: $event })" />
        <TextAreaField :label="labels.content" :value="node.notifyContent" @input="patch({ notifyContent: $event })" />
      </template>
      <template v-else-if="node.type === 'alarm'">
        <SelectField :label="labels.level" :value="node.alarmLevel" :options="alarmLevels" @change="patch({ alarmLevel: $event })" />
        <FieldInput :label="labels.title" :value="node.alarmTitle" @input="patch({ alarmTitle: $event })" />
        <TextAreaField :label="labels.content" :value="node.alarmContent" @input="patch({ alarmContent: $event })" />
      </template>
      <template v-else-if="node.type === 'delay'">
        <FieldInput type="number" :label="labels.delaySec" :value="node.delaySec" @input="patch({ delaySec: Number($event) || 1 })" />
      </template>
      <template v-else-if="node.type === 'llm'">
        <TextAreaField :label="labels.llmPrompt" :value="node.llmPrompt" @input="patch({ llmPrompt: $event })" />
      </template>
      <button v-if="isActionGroup" type="button" class="btn btn-sm btn-outline-primary" @click="$emit('add-child', node.id)">
        <i class="bi bi-node-plus me-1"></i>{{ labels.addChildAction }}
      </button>
    </div>

    <div v-else class="text-muted small">{{ labels.selectNode }}</div>
  </aside>
</template>

<script>
import VarInputWrapper from './rule/VarInputWrapper.vue'
const FieldInput = {
  props: ['label', 'value', 'type', 'placeholder'],
  emits: ['input'],
  template: `
    <div>
      <label class="form-label">{{ label }}</label>
      <VarInputWrapper :type="type || \'text\'" :modelValue="value" :placeholder="placeholder" @update:modelValue="$emit(\'input\', $event)" />
    </div>
  `
}

const TextAreaField = {
  props: ['label', 'value'],
  emits: ['input'],
  template: `
    <div>
      <label class="form-label">{{ label }}</label>
      <VarInputWrapper :textarea="true" :rows="3" :modelValue="value" @update:modelValue="$emit(\'input\', $event)" />
    </div>
  `
}

const SelectField = {
  props: ['label', 'value', 'options'],
  emits: ['change'],
  template: `
    <div>
      <label class="form-label">{{ label }}</label>
      <select class="form-select form-select-sm" :value="value" @change="$emit('change', $event.target.value)">
        <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
      </select>
    </div>
  `
}

export default {
  name: 'RuleGraphInspector',
  components: { FieldInput, TextAreaField, SelectField , VarInputWrapper},
  props: {
    title: {
      type: String,
      required: true
    },
    selection: {
      type: Object,
      default: null
    },
    selectedNode: {
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
  emits: ['update', 'update-meta', 'delete', 'add-child'],
  computed: {
    kind() {
      return this.selection?.kind || 'meta'
    },
    node() {
      return this.selectedNode || {}
    },
    heading() {
      return this.labels.kind?.[this.kind] || this.labels.selectNode
    },
    canDelete() {
      return ['trigger', 'condition', 'condition_group', 'action'].includes(this.kind)
    },
    isActionGroup() {
      return this.kind === 'action' && (this.node.type === 'sequence_group' || this.node.type === 'parallel_group')
    },
    deviceOptions() {
      return [{ value: '', label: this.labels.selectDevice }].concat(this.devices.map(device => ({
        value: device.code,
        label: `${device.name || device.code} (${device.code})`
      })))
    },
    triggerTypes() {
      return [
        { value: 'property_change', label: this.labels.triggerType.property_change },
        { value: 'event', label: this.labels.triggerType.event },
        { value: 'device_status', label: this.labels.triggerType.device_status },
        { value: 'cron', label: this.labels.triggerType.cron }
      ]
    },
    conditionTypes() {
      return [
        { value: 'property', label: this.labels.conditionType.property },
        { value: 'device_status', label: this.labels.conditionType.device_status }
      ]
    },
    actionTypes() {
      return [
        { value: 'set_property', label: this.labels.actionType.set_property },
        { value: 'call_service', label: this.labels.actionType.call_service },
        { value: 'notification', label: this.labels.actionType.notification },
        { value: 'alarm', label: this.labels.actionType.alarm },
        { value: 'delay', label: this.labels.actionType.delay },
        { value: 'text', label: this.labels.actionType.text || 'Text' },
        { value: 'llm', label: this.labels.actionType.llm || 'LLM' },
        { value: 'voice_playback', label: this.labels.actionType.voice_playback || 'Voice playback' },
        { value: 'sequence_group', label: this.labels.actionType.sequence_group },
        { value: 'parallel_group', label: this.labels.actionType.parallel_group }
      ]
    },
    triggerOperators() {
      return [
        { value: 'changed', label: this.labels.changed },
        { value: 'eq', label: '=' },
        { value: 'gt', label: '>' },
        { value: 'gte', label: '>=' },
        { value: 'lt', label: '<' },
        { value: 'lte', label: '<=' }
      ]
    },
    conditionOperators() {
      return [
        { value: 'eq', label: '=' },
        { value: 'neq', label: '!=' },
        { value: 'gt', label: '>' },
        { value: 'gte', label: '>=' },
        { value: 'lt', label: '<' },
        { value: 'lte', label: '<=' },
        { value: 'contains', label: this.labels.contains }
      ]
    },
    statusOptions() {
      return [
        { value: 'online', label: this.labels.online },
        { value: 'offline', label: this.labels.offline }
      ]
    },
    logicOptions() {
      return [
        { value: 'and', label: 'AND' },
        { value: 'or', label: 'OR' }
      ]
    },
    effectiveModes() {
      return [
        { value: 'always', label: this.labels.effectiveMode.always },
        { value: 'daily', label: this.labels.effectiveMode.daily },
        { value: 'weekly', label: this.labels.effectiveMode.weekly },
        { value: 'monthly', label: this.labels.effectiveMode.monthly },
        { value: 'workday', label: this.labels.effectiveMode.workday },
        { value: 'holiday', label: this.labels.effectiveMode.holiday },
        { value: 'custom', label: this.labels.effectiveMode.custom }
      ]
    },
    alarmLevels() {
      return [
        { value: 'info', label: this.labels.info },
        { value: 'warning', label: this.labels.warning },
        { value: 'critical', label: this.labels.critical }
      ]
    },
    paramsText() {
      return JSON.stringify(this.node.serviceParams || {}, null, 2)
    }
  },
  methods: {
    optionKey(item) {
      return item?.key || item?.identifier || item?.code || item?.id || item?.name || ''
    },
    optionLabel(item) {
      const key = this.optionKey(item)
      return item?.name ? `${item.name} (${key})` : key
    },
    findDevice(code) {
      return this.devices.find(device => device.code === code)
    },
    propertyOptions(deviceCode) {
      const items = this.findDevice(deviceCode)?.properties || []
      return [{ value: '', label: this.labels.selectProperty }].concat(items.map(item => ({
        value: this.optionKey(item),
        label: this.optionLabel(item)
      })))
    },
    eventOptions(deviceCode) {
      const items = this.findDevice(deviceCode)?.events || []
      return [{ value: '', label: this.labels.event }].concat(items.map(item => ({
        value: this.optionKey(item),
        label: this.optionLabel(item)
      })))
    },
    serviceOptions(deviceCode) {
      const items = this.findDevice(deviceCode)?.services || []
      return [{ value: '', label: this.labels.selectService }].concat(items.map(item => ({
        value: this.optionKey(item),
        label: this.optionLabel(item)
      })))
    },
    patch(patch) {
      this.$emit('update', { kind: this.kind, id: this.selection?.id, patch })
    },
    patchMeta(patch) {
      this.$emit('update-meta', patch)
    },
    patchEffectiveMode(mode) {
      const windows = this.node.windows?.length ? this.node.windows : [{ startTime: '00:00:00', endTime: '24:00:00' }]
      this.patch({ mode, windows })
    },
    patchWindow(index, patch) {
      const windows = [...(this.node.windows || [])]
      windows[index] = { ...windows[index], ...patch }
      this.patch({ windows })
    },
    addWindow() {
      this.patch({ windows: [...(this.node.windows || []), { startTime: '00:00:00', endTime: '24:00:00' }] })
    },
    patchParams(text) {
      try {
        this.patch({ serviceParams: JSON.parse(text || '{}') })
      } catch (_) {
        this.patch({ serviceParamsText: text })
      }
    }
  }
}
</script>

<style scoped>
.rule-graph-inspector {
  display: grid;
  gap: 1rem;
  position: sticky;
  top: 1rem;
  min-width: 0;
}

.inspector-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.inspector-kicker {
  color: var(--bs-secondary-color);
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
}

.inspector-form {
  display: grid;
  gap: 0.8rem;
}

.window-row {
  display: grid;
  gap: 0.5rem;
  padding: 0.65rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  background: var(--bs-tertiary-bg);
}
</style>
