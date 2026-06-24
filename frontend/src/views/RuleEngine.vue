<template>
  <div class="rule-engine container-fluid py-4">
    <div class="d-flex flex-wrap justify-content-between align-items-center gap-3 mb-4">
      <div>
        <h2 class="h4 mb-1 fw-bold text-primary border-start border-primary border-4 ps-2">{{ $t('rule_engine') }}</h2>
        <div class="text-muted small">{{ $t('rule_engine_subtitle') }}</div>
      </div>
      <div class="d-flex gap-2">
        <button class="btn btn-outline-secondary btn-sm" @click="fetchAll" :disabled="loading">
          <i class="bi me-1" :class="loading ? 'bi-arrow-repeat spin' : 'bi-arrow-clockwise'"></i>{{ $t('refresh') }}
        </button>
        <button class="btn btn-primary btn-sm" @click="openNewRuleGraph" v-permission="'rule:create'">
          <i class="bi bi-plus-lg me-1"></i>{{ $t('rule_create') }}
        </button>
      </div>
    </div>

    <div class="row g-3 mb-4">
      <div class="col-md-3 col-sm-6" v-for="card in summaryCards" :key="card.key">
        <div class="card border-0 shadow-sm h-100">
          <div class="card-body py-3">
            <div class="d-flex align-items-center justify-content-between">
              <div>
                <div class="text-muted small">{{ card.label }}</div>
                <div class="fs-4 fw-semibold">{{ card.value }}</div>
              </div>
              <i class="bi fs-3 text-primary opacity-75" :class="card.icon"></i>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="card border-0 shadow-sm mb-4">
      <div class="card-header bg-transparent border-0 py-3">
        <div class="row g-2 align-items-center">
          <div class="col-lg-5">
            <div class="input-group input-group-sm">
              <span class="input-group-text"><i class="bi bi-search"></i></span>
              <input class="form-control" v-model.trim="search" :placeholder="$t('rule_search_placeholder')">
            </div>
          </div>
          <div class="col-sm-4 col-lg-2">
            <select class="form-select form-select-sm" v-model="statusFilter">
              <option value="">{{ $t('all') }}</option>
              <option value="enabled">{{ $t('rule_status_enabled') }}</option>
              <option value="disabled">{{ $t('rule_status_disabled') }}</option>
              <option value="draft">{{ $t('rule_status_draft') }}</option>
              <option value="error">{{ $t('rule_status_error') }}</option>
            </select>
          </div>
          <div class="col-sm-4 col-lg-2">
            <select class="form-select form-select-sm" v-model="groupFilter">
              <option value="">{{ $t('rule_all_groups') }}</option>
              <option value="__none__">{{ $t('rule_no_group') }}</option>
              <option v-for="group in groups" :key="group.id || group.ID" :value="String(group.id || group.ID)">
                {{ group.name }}
              </option>
            </select>
          </div>
          <div class="col-sm-4 col-lg-3 text-lg-end">
            <button class="btn btn-outline-primary btn-sm" @click="openGroupModal" v-permission="'rule_group:manage'">
              <i class="bi bi-folder-plus me-1"></i>{{ $t('rule_manage_groups') }}
            </button>
          </div>
        </div>
      </div>
      <div class="card-body p-0">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ $t('rule_name') }}</th>
                <th>{{ $t('rule_trigger') }}</th>
                <th>{{ $t('rule_scope') }}</th>
                <th>{{ $t('status') }}</th>
                <th class="text-end">{{ $t('rule_trigger_count') }}</th>
                <th class="text-end">{{ $t('actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && rules.length === 0">
                <td colspan="6" class="text-center py-5 text-muted">
                  <span class="spinner-border spinner-border-sm me-2"></span>{{ $t('loading') }}
                </td>
              </tr>
              <tr v-else-if="filteredRules.length === 0">
                <td colspan="6" class="text-center py-5 text-muted">
                  <i class="bi bi-diagram-3 fs-1 d-block mb-2"></i>{{ $t('rule_empty') }}
                </td>
              </tr>
              <tr v-for="rule in filteredRules" :key="rule.code">
                <td>
                  <div class="fw-semibold">{{ rule.name }}</div>
                  <div class="small text-muted text-truncate rule-desc">{{ rule.description || rule.code }}</div>
                  <span v-if="groupName(rule.group_id)" class="badge bg-secondary-subtle text-secondary-emphasis mt-1">
                    {{ groupName(rule.group_id) }}
                  </span>
                </td>
                <td>
                  <div class="small">{{ describeTriggers(rule) }}</div>
                  <div class="small text-muted">{{ describeActions(rule) }}</div>
                </td>
                <td>
                  <span class="badge rounded-pill" :class="rule.scope === 'gateway' ? 'bg-info-subtle text-info-emphasis' : 'bg-primary-subtle text-primary-emphasis'">
                    {{ rule.scope === 'gateway' ? $t('rule_scope_gateway') : $t('rule_scope_platform') }}
                  </span>
                  <div v-if="rule.gateway_sn" class="small text-muted mt-1">{{ rule.gateway_sn }}</div>
                  <div class="small text-muted">{{ syncStateLabel(rule.sync_state) }}</div>
                </td>
                <td>
                  <span class="badge rounded-pill" :class="statusBadge(rule.status)">
                    {{ statusLabel(rule.status) }}
                  </span>
                  <div v-if="rule.error_message" class="small text-danger mt-1">{{ localizedRuleError(rule.error_message) }}</div>
                </td>
                <td class="text-end">
                  <div>{{ rule.trigger_count || 0 }}</div>
                  <div class="small text-muted">{{ formatTime(rule.last_triggered_at) }}</div>
                </td>
                <td class="text-end">
                  <div class="btn-group btn-group-sm">
                    <button class="btn btn-outline-secondary" @click="openLogs(rule)" :title="$t('rule_logs')" v-permission="'rule:log'">
                      <i class="bi bi-clock-history"></i>
                    </button>
                    <button class="btn btn-outline-info" @click="openRuleGraph(rule)" :title="$t('rule_graph_view')">
                      <i class="bi bi-diagram-3"></i>
                    </button>
                    <button v-if="rule.enabled" class="btn btn-outline-warning" @click="toggleRule(rule, false)" :title="$t('disable')" v-permission="'rule:enable'">
                      <i class="bi bi-pause-fill"></i>
                    </button>
                    <button v-else class="btn btn-outline-success" @click="toggleRule(rule, true)" :title="$t('enable')" v-permission="'rule:enable'">
                      <i class="bi bi-play-fill"></i>
                    </button>
                    <button class="btn btn-outline-danger" :disabled="rule.enabled" @click="deleteRule(rule)" :title="$t('tsl_delete')" v-permission="'rule:delete'">
                      <i class="bi bi-trash"></i>
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <div v-if="showGraph" class="modal fade show d-block rule-modal" tabindex="-1">
      <div class="modal-dialog modal-xl rule-graph-dialog">
        <div class="modal-content border-0 shadow-lg">
          <div class="modal-header">
            <div>
              <h5 class="modal-title">{{ graphRule?.code ? $t('rule_graph_view') : $t('rule_create') }} - {{ graphRule?.name || $t('rule_new_default_name') }}</h5>
              <div class="small text-muted">{{ graphRule?.code ? $t('rule_graph_readonly') : $t('rule_graph_editing') }}</div>
            </div>
            <button type="button" class="btn-close" @click="closeRuleGraph"></button>
          </div>
          <div class="modal-body bg-light">
            <RuleGraphViewer :rule="graphRule" :devices="devices" :groups="groups" @update-rule="handleGraphUpdate" />
          </div>
        </div>
      </div>
    </div>

    <div v-if="showLogs" class="modal fade show d-block rule-modal" tabindex="-1">
      <div class="modal-dialog modal-lg">
        <div class="modal-content border-0 shadow-lg">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('rule_logs') }} - {{ logRule?.name }}</h5>
            <button type="button" class="btn-close" @click="showLogs = false"></button>
          </div>
          <div class="modal-body p-0">
            <table class="table table-hover align-middle mb-0">
              <thead class="table-light">
                <tr>
                  <th>{{ $t('time') }}</th>
                  <th>{{ $t('rule_trigger') }}</th>
                  <th>{{ $t('status') }}</th>
                  <th class="text-end">{{ $t('rule_duration') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="logs.length === 0">
                  <td colspan="4" class="text-center py-4 text-muted">{{ $t('rule_no_logs') }}</td>
                </tr>
                <tr v-for="log in logs" :key="log.id || log.ID">
                  <td>{{ formatTime(log.executed_at) }}</td>
                  <td>{{ triggerTypeLabel(log.trigger_type) }}</td>
                  <td>
                    <span class="badge rounded-pill" :class="log.success ? 'bg-success' : 'bg-danger'">
                      {{ log.success ? $t('success') : $t('failed') }}
                    </span>
                    <div v-if="log.error_message" class="small text-danger">{{ localizedRuleError(log.error_message) }}</div>
                  </td>
                  <td class="text-end">{{ log.duration_ms }} ms</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showGroupModal" class="modal fade show d-block rule-modal" tabindex="-1">
      <div class="modal-dialog">
        <div class="modal-content border-0 shadow-lg">
          <div class="modal-header">
            <h5 class="modal-title">{{ $t('rule_manage_groups') }}</h5>
            <button type="button" class="btn-close" @click="showGroupModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="input-group mb-3">
              <input class="form-control" v-model.trim="groupForm.name" :placeholder="$t('rule_group_name')">
              <button class="btn btn-primary" @click="saveGroup" :disabled="!groupForm.name">
                <i class="bi bi-plus-lg"></i>
              </button>
            </div>
            <div class="list-group">
              <div v-for="group in groups" :key="group.id || group.ID" class="list-group-item d-flex justify-content-between align-items-center">
                <span>{{ group.name }}</span>
                <button class="btn btn-sm btn-outline-danger" @click="deleteGroup(group)">
                  <i class="bi bi-trash"></i>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import VarInputWrapper from '../components/rule/VarInputWrapper.vue'
import { computed, defineComponent, h, onMounted, reactive, ref, watch, provide, inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToast } from '../composables/useToast'
import axios from 'axios'
import RuleGraphViewer from '@/components/rule/RuleGraphViewer.vue'

function uid(prefix) {
  return `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`
}

function optionKey(item) {
  return item?.key || item?.identifier || ''
}

function baseAction() {
  return {
    id: uid('act'),
    type: 'set_property',
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
    voiceText: '',
    subActions: []
  }
}

function baseCondition() {
  return { id: uid('cond'), type: 'property', deviceCode: '', propertyKey: '', operator: 'eq', value: '', statusValue: 'online', startTime: '', endTime: '' }
}

function baseTrigger(type = 'property_change') {
  return {
    id: uid('trg'),
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
      monthDaysText: '1',
      minuteExpr: '*/5',
      hourExpr: '*',
      dayOfMonthExpr: '*',
      monthExpr: '*',
      weekdayExpr: '*'
    }
  }
}

const RuleConditionGroupEditor = defineComponent({
  name: 'RuleConditionGroupEditor',
  props: {
    group: { type: Object, required: true },
    devices: { type: Array, required: true },
    labels: { type: Object, required: true },
    level: { type: Number, default: 0 }
  },
  setup(props) {
    const isExpanded = ref(true)
    const generateUniqueName = inject('generateUniqueName')
    const getAllGroupNames = inject('getAllGroupNames')
    const condColors = ['#6f42c1', '#0dcaf0', '#20c997', '#d63384', '#0d6efd']
    const groupColor = computed(() => condColors[props.level % condColors.length])
    const groupBg = computed(() => {
      const hex = groupColor.value
      const r = parseInt(hex.slice(1, 3), 16), g = parseInt(hex.slice(3, 5), 16), b = parseInt(hex.slice(5, 7), 16)
      return `rgba(${r}, ${g}, ${b}, 0.03)`
    })
    const deviceLabel = (device) => `${device.name || device.code} (${device.code})`
    const optionLabel = (item) => item?.name ? `${item.name} (${optionKey(item)})` : optionKey(item)
    const deviceFor = (code) => props.devices.find(device => device.code === code)
    const ensureArrays = () => {
      if (!props.group.conditions) props.group.conditions = []
      if (!props.group.groups) props.group.groups = []
      if (!props.group.logic) props.group.logic = 'and'
    }
    const addCondition = () => {
      ensureArrays()
      props.group.conditions.push(baseCondition())
    }
    const addGroup = () => {
      ensureArrays()
      const name = generateUniqueName ? generateUniqueName(props.labels.conditionGroup) : `${props.labels.conditionGroup} ${Date.now()}`
      props.group.groups.push({ logic: 'and', conditions: [baseCondition()], groups: [], name })
    }
    const row = (children) => h('div', { class: 'builder-line' }, children)
    const field = (label, child, cls = 'col-md-3') => h('div', { class: cls }, [h('label', { class: 'form-label' }, label), child])
    const bind = (target, key) => ({ value: target[key] || '', onInput: e => { target[key] = e.target.value } })
    const renderCondition = (condition, index) => {
      const device = deviceFor(condition.deviceCode)
      const properties = device?.properties || []
      return h('article', { class: 'builder-item' }, [
        row([
          field(props.labels.type, h('select', {
            class: 'form-select',
            value: condition.type,
            onChange: e => { condition.type = e.target.value }
          }, [
            h('option', { value: 'property' }, props.labels.propertyCondition),
            h('option', { value: 'device_status' }, props.labels.statusCondition),
            h('option', { value: 'time_range' }, props.labels.timeCondition)
          ])),
          condition.type !== 'time_range'
            ? field(props.labels.device, h('select', {
                class: 'form-select',
                value: condition.deviceCode,
                onChange: e => { condition.deviceCode = e.target.value }
              }, [h('option', { value: '' }, props.labels.selectDevice), ...props.devices.map(device => h('option', { value: device.code }, deviceLabel(device)))]), 'col-md-4')
            : null,
          condition.type === 'property'
            ? field(props.labels.property, h('select', {
                class: 'form-select',
                value: condition.propertyKey,
                onChange: e => { condition.propertyKey = e.target.value }
              }, [h('option', { value: '' }, props.labels.selectProperty), ...properties.map(prop => h('option', { value: optionKey(prop) }, optionLabel(prop)))]), 'col-md-3')
            : null,
          condition.type === 'property'
            ? field(props.labels.operator, h('select', {
                class: 'form-select',
                value: condition.operator,
                onChange: e => { condition.operator = e.target.value }
              }, [
                h('option', { value: 'eq' }, '='),
                h('option', { value: 'neq' }, '!='),
                h('option', { value: 'gt' }, '>'),
                h('option', { value: 'gte' }, '>='),
                h('option', { value: 'lt' }, '<'),
                h('option', { value: 'lte' }, '<='),
                h('option', { value: 'contains' }, props.labels.contains)
              ]), 'col-md-2')
            : null,
          condition.type === 'property'
            ? field(props.labels.value, h('input', { class: 'form-control', ...bind(condition, 'value') }), 'col-md-3')
            : null,
          condition.type === 'device_status'
            ? field(props.labels.status, h('select', {
                class: 'form-select',
                value: condition.statusValue,
                onChange: e => { condition.statusValue = e.target.value }
              }, [h('option', { value: 'online' }, props.labels.online), h('option', { value: 'offline' }, props.labels.offline)]), 'col-md-3')
            : null,
          condition.type === 'time_range'
            ? field(props.labels.start, h('input', { class: 'form-control', type: 'time', ...bind(condition, 'startTime') }), 'col-md-3')
            : null,
          condition.type === 'time_range'
            ? field(props.labels.end, h('input', { class: 'form-control', type: 'time', ...bind(condition, 'endTime') }), 'col-md-3')
            : null,
          h('div', { class: 'col text-end' }, [
            h('button', { class: 'btn btn-outline-danger btn-sm', onClick: () => props.group.conditions.splice(index, 1) }, [h('i', { class: 'bi bi-trash' })])
          ])
        ])
      ])
    }
    return () => {
      ensureArrays()
      const empty = props.group.conditions.length === 0 && props.group.groups.length === 0
      return h('div', { class: ['condition-group-card', props.level > 0 ? 'condition-group-card--nested' : ''], style: { '--group-color': groupColor.value, '--group-bg': groupBg.value } }, [
        h('div', { class: 'condition-group-card__header d-flex flex-wrap justify-content-between align-items-center gap-2 mb-3 pb-3' }, [
          h('div', { class: 'd-flex align-items-center gap-2' }, [
            h('button', { class: 'btn btn-sm btn-link text-decoration-none px-0 toggle-btn', onClick: () => { isExpanded.value = !isExpanded.value } }, [
              h('i', { class: isExpanded.value ? 'bi bi-chevron-down' : 'bi bi-chevron-right', style: { color: groupColor.value } })
            ]),
            h('div', { class: 'd-flex align-items-center group-title', style: { color: groupColor.value } }, [
              h('i', { class: 'bi bi-filter-square me-2 fw-bold' }),
              h('input', {
                class: 'form-control form-control-sm border-0 bg-transparent fw-bold p-0',
                style: { color: groupColor.value, width: '150px', boxShadow: 'none' },
                value: props.group.name || props.labels.conditionGroup,
                onFocus: () => { props.group._oldName = props.group.name },
                onChange: e => {
                  const desired = e.target.value.trim()
                  if (!desired) {
                    e.target.value = props.group._oldName || props.labels.conditionGroup
                    props.group.name = props.group._oldName || props.labels.conditionGroup
                    return
                  }
                  if (desired !== props.group._oldName) {
                    const existing = getAllGroupNames ? getAllGroupNames() : new Set()
                    existing.delete(props.group._oldName)
                    let finalName = desired
                    let count = 1
                    while(existing.has(finalName)) {
                      finalName = `${desired} ${count}`
                      count++
                    }
                    props.group.name = finalName
                    e.target.value = finalName
                  }
                },
                placeholder: props.labels.conditionGroup
              })
            ]),
            h('div', { class: 'btn-group btn-group-sm ms-2 bg-body rounded shadow-sm logic-switch' }, [
              h('button', { class: props.group.logic === 'and' ? 'btn btn-primary' : 'btn btn-outline-secondary', onClick: () => { props.group.logic = 'and' } }, props.labels.and),
              h('button', { class: props.group.logic === 'or' ? 'btn btn-primary' : 'btn btn-outline-secondary', onClick: () => { props.group.logic = 'or' } }, props.labels.or)
            ])
          ]),
          h('div', { class: 'btn-group btn-group-sm shadow-sm' }, [
            h('button', { class: 'btn btn-outline-primary bg-body', onClick: addCondition }, [h('i', { class: 'bi bi-plus-lg me-1' }), props.labels.addCondition]),
            h('button', { class: 'btn btn-outline-primary bg-body', onClick: addGroup }, [h('i', { class: 'bi bi-diagram-3 me-1' }), props.labels.addGroup])
          ])
        ]),
        h('div', { style: { display: isExpanded.value ? 'block' : 'none' }, class: 'condition-group-body' }, [
          empty ? h('div', { class: 'builder-empty' }, props.labels.noConditions) : null,
          ...props.group.conditions.map((cond, idx) => h('div', { class: 'condition-item-wrapper position-relative' }, [
            renderCondition(cond, idx)
          ])),
          ...props.group.groups.map((group, index) => h('div', { class: 'builder-item-wrapper position-relative mt-3' }, [
            h('div', { class: 'd-flex justify-content-end align-items-center mb-2' }, [
              h('button', { class: 'btn btn-outline-danger btn-sm shadow-sm bg-body', onClick: () => props.group.groups.splice(index, 1) }, [h('i', { class: 'bi bi-trash' })])
            ]),
            h(RuleConditionGroupEditor, { group, devices: props.devices, labels: props.labels, level: props.level + 1 })
          ]))
        ])
      ])
    }
  }
})

const ActionEditor = defineComponent({
  name: 'ActionEditor',
  props: {
    action: { type: Object, required: true },
    devices: { type: Array, required: true },
    level: { type: Number, default: 0 },
    labels: { type: Object, required: true }
  },
  emits: ['remove', 'add-sub', 'remove-sub'],
  setup(props, { emit }) {
    const isExpanded = ref(true)
    const generateUniqueName = inject('generateUniqueName')
    const getAllGroupNames = inject('getAllGroupNames')
    const actionColors = ['#fd7e14', '#e83e8c', '#6610f2', '#0dcaf0', '#20c997']
    const groupColor = computed(() => actionColors[props.level % actionColors.length])
    const groupBg = computed(() => {
      const hex = groupColor.value
      const r = parseInt(hex.slice(1, 3), 16), g = parseInt(hex.slice(3, 5), 16), b = parseInt(hex.slice(5, 7), 16)
      return `rgba(${r}, ${g}, ${b}, 0.03)`
    })
    const deviceLabel = (device) => `${device.name || device.code} (${device.code})`
    const optionLabel = (item) => item?.name ? `${item.name} (${optionKey(item)})` : optionKey(item)
    const device = computed(() => props.devices.find(d => d.code === props.action.deviceCode))
    const properties = computed(() => device.value?.properties || [])
    const services = computed(() => device.value?.services || [])
    const row = (children) => h('div', { class: 'builder-line' }, children)
    const field = (label, child, cls = 'col-md-3') => h('div', { class: cls }, [h('label', { class: 'form-label' }, label), child])
    const inputModel = (key) => ({
      modelValue: props.action[key] || '',
      'onUpdate:modelValue': val => { props.action[key] = val }
    })

    return () => {
      const isGroup = props.action.type === 'parallel_group' || props.action.type === 'sequence_group'
      
      if (isGroup) {
        return h('div', { class: ['condition-group-card', 'action-group-card', props.level > 0 ? 'condition-group-card--nested' : ''], style: { '--group-color': groupColor.value, '--group-bg': groupBg.value } }, [
          h('div', { class: 'condition-group-card__header d-flex flex-wrap justify-content-between align-items-center gap-2 mb-3 pb-3' }, [
            h('div', { class: 'd-flex align-items-center gap-2' }, [
              h('button', { class: 'btn btn-sm btn-link text-decoration-none px-0 toggle-btn', onClick: () => { isExpanded.value = !isExpanded.value } }, [
                h('i', { class: isExpanded.value ? 'bi bi-chevron-down' : 'bi bi-chevron-right', style: { color: groupColor.value } })
              ]),
              h('div', { class: 'd-flex align-items-center group-title', style: { color: groupColor.value } }, [
                h('i', { class: props.action.type === 'sequence_group' ? 'bi bi-list-nested me-2 fw-bold' : 'bi bi-diagram-3 me-2 fw-bold' }),
                h('input', {
                  class: 'form-control form-control-sm border-0 bg-transparent fw-bold p-0',
                  style: { color: groupColor.value, width: '150px', boxShadow: 'none' },
                  value: props.action.name,
                  onFocus: () => { props.action._oldName = props.action.name },
                  onChange: e => {
                    const desired = e.target.value.trim()
                    if (!desired) {
                      const fallback = props.action.type === 'sequence_group' ? props.labels.sequenceGroup : props.labels.parallelGroup
                      e.target.value = props.action._oldName || fallback
                      props.action.name = props.action._oldName || fallback
                      return
                    }
                    if (desired !== props.action._oldName) {
                      const existing = getAllGroupNames ? getAllGroupNames() : new Set()
                      existing.delete(props.action._oldName)
                      let finalName = desired
                      let count = 1
                      while(existing.has(finalName)) {
                        finalName = `${desired} ${count}`
                        count++
                      }
                      props.action.name = finalName
                      e.target.value = finalName
                    }
                  },
                  placeholder: props.action.type === 'sequence_group' ? props.labels.sequenceGroup : props.labels.parallelGroup
                })
              ]),
              h('div', { class: 'btn-group btn-group-sm ms-2 bg-body rounded shadow-sm logic-switch' }, [
                h('button', { class: props.action.type === 'sequence_group' ? 'btn btn-primary' : 'btn btn-outline-secondary', onClick: () => { props.action.type = 'sequence_group' } }, props.labels.sequenceGroup),
                h('button', { class: props.action.type === 'parallel_group' ? 'btn btn-primary' : 'btn btn-outline-secondary', onClick: () => { props.action.type = 'parallel_group' } }, props.labels.parallelGroup)
              ])
            ]),
            h('div', { class: 'd-flex align-items-center gap-2' }, [
              h('div', { class: 'btn-group btn-group-sm shadow-sm' }, [
                h('button', { class: 'btn btn-outline-primary bg-body', onClick: () => emit('add-sub') }, [h('i', { class: 'bi bi-plus-lg me-1' }), props.labels.addAction]),
                h('button', { class: 'btn btn-outline-primary bg-body', onClick: () => {
                  if (!props.action.subActions) props.action.subActions = [];
                  const base = props.labels.sequenceGroup || '串行动作组';
                  const name = generateUniqueName ? generateUniqueName(base) : `${base} ${Date.now()}`;
                  props.action.subActions.push({
                    id: `act_${Date.now()}_${Math.random().toString(16).slice(2)}`,
                    type: 'sequence_group',
                    name,
                    subActions: []
                  });
                } }, [h('i', { class: 'bi bi-diagram-3 me-1' }), props.labels.addGroup || '添加组'])
              ]),
              h('button', { class: 'btn btn-outline-danger btn-sm shadow-sm bg-body', onClick: () => emit('remove') }, [h('i', { class: 'bi bi-trash' })])
            ])
          ]),
          h('div', { style: { display: isExpanded.value ? 'block' : 'none' }, class: 'condition-group-body action-group-body' }, [
            (!props.action.subActions || props.action.subActions.length === 0) ? h('div', { class: 'builder-empty' }, props.labels.noActions || '无执行动作') : null,
            ...(props.action.subActions || []).map((sub, idx) => h('div', { class: 'action-sub-wrapper position-relative' }, [
              h(ActionEditor, {
                action: sub,
                devices: props.devices,
                labels: props.labels,
                level: props.level + 1,
                onRemove: () => emit('remove-sub', idx),
                onAddSub: () => {
                  if (!sub.subActions) sub.subActions = [];
                  sub.subActions.push({
                    id: `act_${Date.now()}_${Math.random().toString(16).slice(2)}`,
                    type: 'set_property',
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
                    subActions: []
                  });
                },
                onRemoveSub: subIndex => sub.subActions.splice(subIndex, 1)
              })
            ]))
          ])
        ])
      }

      return h('article', { class: ['builder-item', props.level > 0 ? 'builder-item--nested' : ''] }, [
        row([
          field(props.labels.type, h('select', {
            class: 'form-select',
            value: props.action.type,
            onChange: e => { 
              props.action.type = e.target.value 
            }
          }, [
            h('option', { value: 'set_property' }, props.labels.setProperty),
            h('option', { value: 'call_service' }, props.labels.callService),
            h('option', { value: 'notification' }, props.labels.notification),
            h('option', { value: 'alarm' }, props.labels.alarm),
            h('option', { value: 'delay' }, props.labels.delay),
            h('option', { value: 'llm' }, props.labels.llm || 'LLM 组件'),
            h('option', { value: 'voice_playback' }, props.labels.voicePlayback || '语音播放')
          ])),
          props.action.type === 'set_property' || props.action.type === 'call_service'
            ? field(props.labels.device, h('select', {
                class: 'form-select',
                value: props.action.deviceCode,
                onChange: e => { props.action.deviceCode = e.target.value }
              }, [h('option', { value: '' }, props.labels.selectDevice), ...props.devices.map(device => h('option', { value: device.code }, deviceLabel(device)))]), 'col-md-4')
            : null,
          props.action.type === 'set_property'
            ? field(props.labels.property, h('select', {
                class: 'form-select',
                value: props.action.propertyKey,
                onChange: e => { props.action.propertyKey = e.target.value }
              }, [h('option', { value: '' }, props.labels.selectProperty), ...properties.value.map(prop => h('option', { value: optionKey(prop) }, optionLabel(prop)))]), 'col-md-3')
            : null,
          props.action.type === 'set_property'
            ? field(props.labels.value, h('input', { class: 'form-control', value: props.action.value || '', onInput: e => { props.action.value = e.target.value } }), 'col-md-2')
            : null,
          props.action.type === 'call_service'
            ? field(props.labels.service, h('select', {
                class: 'form-select',
                value: props.action.serviceCode,
                onChange: e => { props.action.serviceCode = e.target.value }
              }, [h('option', { value: '' }, props.labels.selectService), ...services.value.map(svc => h('option', { value: optionKey(svc) }, optionLabel(svc)))]), 'col-md-4')
            : null,
          props.action.type === 'call_service'
            ? field(props.labels.params, h('textarea', {
                class: 'form-control',
                rows: 1,
                value: JSON.stringify(props.action.serviceParams || {}),
                onInput: e => {
                  try { props.action.serviceParams = JSON.parse(e.target.value || '{}') } catch (_) {}
                }
              }), 'col-md-4')
            : null,
          props.action.type === 'notification'
            ? field(props.labels.title, h(VarInputWrapper, { ...inputModel('notifyTitle') }), 'col-md-4')
            : null,
          props.action.type === 'notification'
            ? field(props.labels.content, h(VarInputWrapper, { ...inputModel('notifyContent') }), 'col-md-5')
            : null,
          props.action.type === 'alarm'
            ? field(props.labels.level, h('select', {
                class: 'form-select',
                value: props.action.alarmLevel || 'warning',
                onChange: e => { props.action.alarmLevel = e.target.value }
              }, [
                h('option', { value: 'info' }, props.labels.info),
                h('option', { value: 'warning' }, props.labels.warning),
                h('option', { value: 'critical' }, props.labels.critical)
              ]), 'col-md-2')
            : null,
          props.action.type === 'alarm'
            ? field(props.labels.title, h(VarInputWrapper, { ...inputModel('alarmTitle') }), 'col-md-4')
            : null,
          props.action.type === 'alarm'
            ? field(props.labels.content, h(VarInputWrapper, { ...inputModel('alarmContent') }), 'col-md-4')
            : null,
          props.action.type === 'delay'
            ? field(props.labels.delaySec, h('input', { class: 'form-control', type: 'number', min: 0, max: 300, value: props.action.delaySec, onInput: e => { props.action.delaySec = Number(e.target.value) } }), 'col-md-3')
            : null,
          props.action.type === 'llm'
            ? field(props.labels.llmPrompt || '描述词', h(VarInputWrapper, { textarea: true, rows: 1, ...inputModel('llmPrompt') }), 'col-md-5')
            : null,
          props.action.type === 'voice_playback'
            ? field(props.labels.voiceText || '播放文本', h(VarInputWrapper, { textarea: true, rows: 1, ...inputModel('voiceText') }), 'col-md-6')
            : null,
          h('div', { class: 'col text-end' }, [
            h('button', { class: 'btn btn-outline-danger btn-sm shadow-sm bg-body', onClick: () => emit('remove') }, [h('i', { class: 'bi bi-trash' })])
          ])
        ])
      ])
    }
  }
})

export default {
  name: 'RuleEngine',
  components: { ActionEditor, RuleConditionGroupEditor, RuleGraphViewer , VarInputWrapper },
  setup() {
    const { t } = useI18n()
    const { showToast } = useToast()
    const rules = ref([])
    const groups = ref([])
    const devices = ref([])
    const logs = ref([])
    const loading = ref(false)
    const saving = ref(false)
    const search = ref('')
    const statusFilter = ref('')
    const groupFilter = ref('')
    const showEditor = ref(false)
    const showLogs = ref(false)
    const showGraph = ref(false)
    const showGroupModal = ref(false)
    const editingCode = ref('')
    const logRule = ref(null)
    const graphRule = ref(null)
    const groupForm = reactive({ name: '' })
    const analysis = ref(null)
    const form = reactive(defaultForm())
    const expandedSections = reactive({
      basicInfo: true,
      triggers: true,
      conditions: true,
      actions: true
    })

    const getAllGroupNames = () => {
      const set = new Set()
      if (form.conditions) {
        const traverseCond = (g) => {
          if (g.name) set.add(g.name)
          if (g.groups) g.groups.forEach(traverseCond)
        }
        traverseCond(form.conditions)
      }
      
      const traverseAction = (actions) => {
        if (!actions) return
        actions.forEach(a => {
          if (['sequence_group', 'parallel_group'].includes(a.type)) {
            if (a.name) set.add(a.name)
            if (a.subActions) traverseAction(a.subActions)
          }
        })
      }
      traverseAction(form.actions)
      return set
    }

    const generateUniqueName = (base) => {
      const existing = getAllGroupNames()
      let count = 1
      let name = `${base} ${count}`
      while(existing.has(name)) { count++; name = `${base} ${count}` }
      return name
    }
    
    provide('getAllGroupNames', getAllGroupNames)
    provide('generateUniqueName', generateUniqueName)

    function defaultForm() {
      return {
        name: '',
        description: '',
        group_id: null,
        priority: 50,
        throttle_sec: 60,
        max_per_hour: 60,
        retry_count: 0,
        effective_time: defaultEffectiveTime(),
        triggers: [baseTrigger()],
        conditions: { logic: 'and', conditions: [], groups: [], name: t('rule_condition_group_root', '主条件组') },
        actions: [baseAction()]
      }
    }

    function defaultGraphRule() {
      return {
        code: '',
        name: t('rule_new_default_name'),
        description: '',
        group_id: null,
        priority: 50,
        throttle_sec: 60,
        max_per_hour: 60,
        retry_count: 0,
        status: 'draft',
        enabled: false,
        effective_time: defaultEffectiveTime(),
        triggers: [],
        conditions: { logic: 'and', conditions: [], groups: [] },
        actions: []
      }
    }

    function defaultEffectiveTime() {
      return { mode: 'always', windows: [{ startTime: '00:00:00', endTime: '24:00:00', monthDays: [], monthDaysText: '1' }], weekdays: [], monthDays: [], months: [] }
    }

    function assignForm(data) {
      Object.assign(form, defaultForm(), data)
      if (!form.conditions) form.conditions = { logic: 'and', conditions: [], groups: [], name: t('rule_condition_group_root', '主条件组') }
      if (!form.conditions.name) form.conditions.name = t('rule_condition_group_root', '主条件组')
      if (!form.conditions.conditions) form.conditions.conditions = []
      if (!form.conditions.groups) form.conditions.groups = []
      if (!form.effective_time) form.effective_time = defaultEffectiveTime()
      if (!form.effective_time.weekdays) form.effective_time.weekdays = []
      if (!form.effective_time.monthDays) form.effective_time.monthDays = []
      if (!form.effective_time.months) form.effective_time.months = []
      if (!form.effective_time.windows) {
        form.effective_time.windows = form.effective_time.startTime || form.effective_time.endTime
          ? [{ startTime: form.effective_time.startTime || '00:00:00', endTime: form.effective_time.endTime || '24:00:00' }]
          : [{ startTime: '00:00:00', endTime: '24:00:00' }]
      }
      form.effective_time.windows = form.effective_time.windows.map(normalizeEffectiveWindow)
      form.triggers = form.triggers.map(normalizeTrigger)
      if (!form.actions.length) form.actions = [baseAction()]
      if (!form.triggers.length) form.triggers = defaultForm().triggers
    }

    async function fetchAll() {
      loading.value = true
      try {
        const [ruleRes, groupRes, deviceRes] = await Promise.all([
          axios.get('/api/rules', { params: { page: 1, pageSize: 200 } }),
          axios.get('/api/rule-groups'),
          axios.get('/api/rules/device-options')
        ])
        rules.value = (ruleRes.data.data || []).map(normalizeRule)
        groups.value = groupRes.data.data || []
        devices.value = deviceRes.data.data || []
      } finally {
        loading.value = false
      }
    }

    function normalizeRule(rule) {
      return {
        ...rule,
        id: rule.id || rule.ID,
        group_id: rule.group_id || rule.GroupID || null,
        trigger_count: rule.trigger_count || 0,
        last_triggered_at: rule.last_triggered_at || null,
        enabled: !!rule.enabled
      }
    }

    const filteredRules = computed(() => rules.value.filter(rule => {
      const q = search.value.toLowerCase()
      if (q && !`${rule.name} ${rule.description} ${rule.code}`.toLowerCase().includes(q)) return false
      if (statusFilter.value && rule.status !== statusFilter.value) return false
      if (groupFilter.value === '__none__' && rule.group_id) return false
      if (groupFilter.value && groupFilter.value !== '__none__' && String(rule.group_id) !== groupFilter.value) return false
      return true
    }))

    const summaryCards = computed(() => [
      { key: 'total', label: t('rule_total'), value: rules.value.length, icon: 'bi-diagram-3' },
      { key: 'enabled', label: t('rule_status_enabled'), value: rules.value.filter(r => r.enabled).length, icon: 'bi-play-circle' },
      { key: 'gateway', label: t('rule_scope_gateway'), value: rules.value.filter(r => r.scope === 'gateway').length, icon: 'bi-hdd-network' },
      { key: 'error', label: t('rule_status_error'), value: rules.value.filter(r => r.status === 'error').length, icon: 'bi-exclamation-triangle' }
    ])

    const validationMessage = computed(() => {
      if (!form.name) return t('rule_name_required')
      if (!form.triggers.length) return t('rule_trigger_required')
      if (!form.actions.length) return t('rule_action_required')
      return ''
    })

    const executionPreview = computed(() => {
      if (!analysis.value) return ''
      if (analysis.value.scope === 'gateway') return `${t('rule_scope_gateway')}: ${analysis.value.gateway_sn || '-'}`
      return t('rule_scope_platform')
    })

    const weekdayOptions = computed(() => [
      { value: 1, label: t('weekday_mon') },
      { value: 2, label: t('weekday_tue') },
      { value: 3, label: t('weekday_wed') },
      { value: 4, label: t('weekday_thu') },
      { value: 5, label: t('weekday_fri') },
      { value: 6, label: t('weekday_sat') },
      { value: 7, label: t('weekday_sun') }
    ])

    const effectiveMonthDaysText = computed({
      get: () => (form.effective_time.monthDays || []).join(','),
      set: value => { form.effective_time.monthDays = parseNumberList(value, 1, 31) }
    })

    const effectiveMonthsText = computed({
      get: () => (form.effective_time.months || []).join(','),
      set: value => { form.effective_time.months = parseNumberList(value, 1, 12) }
    })

    const showsEffectiveWeekdays = computed(() => form.effective_time.mode === 'weekly' || form.effective_time.mode === 'custom')
    const showsEffectiveMonthDays = computed(() => form.effective_time.mode === 'custom')
    const showsEffectiveMonths = computed(() => form.effective_time.mode === 'custom')
    function groupName(id) {
      if (!id) return ''
      const group = groups.value.find(g => String(g.id || g.ID) === String(id))
      return group?.name || ''
    }

    function statusBadge(status) {
      return {
        enabled: 'bg-success',
        disabled: 'bg-secondary',
        draft: 'bg-info',
        error: 'bg-danger'
      }[status] || 'bg-secondary'
    }

    function statusLabel(status) {
      return {
        enabled: t('rule_status_enabled'),
        disabled: t('rule_status_disabled'),
        draft: t('rule_status_draft'),
        error: t('rule_status_error')
      }[status] || status
    }

    function syncStateLabel(state) {
      if (!state) return ''
      const map = {
        local: t('rule_sync_local'),
        pending: t('rule_sync_pending'),
        synced: t('rule_sync_synced'),
        disabled: t('rule_sync_disabled'),
        error: t('rule_sync_error')
      }
      return `${t('rule_sync')}: ${map[state] || state}`
    }

    function triggerTypeLabel(type) {
      return {
        property_change: t('rule_trigger_property'),
        event: t('rule_trigger_event'),
        device_status: t('rule_trigger_status'),
        cron: t('rule_trigger_cron')
      }[type] || type || '-'
    }

    function actionTypeLabel(type) {
      return {
        set_property: t('rule_action_set_property'),
        call_service: t('rule_action_call_service'),
        notification: t('rule_action_notification'),
        alarm: t('rule_action_alarm'),
        delay: t('rule_action_delay'),
        llm: t('rule_action_llm'),
        voice_playback: t('rule_action_voice_playback'),
        parallel_group: t('rule_parallel_group'),
        sequence_group: t('rule_sequence_group')
      }[type] || type || '-'
    }

    function localizedRuleError(message) {
      if (!message) return ''
      const normalized = String(message).trim().toLowerCase()
      const map = {
        'rule is outside effective time': t('rule_error_outside_effective_time'),
        'rule is throttled': t('rule_error_throttled'),
        'rule exceeds hourly execution limit': t('rule_error_hourly_limit')
      }
      return map[normalized] || message
    }

    function describeTriggers(rule) {
      return safeParse(rule.triggers, []).map(item => triggerTypeLabel(item.type)).join(', ') || '-'
    }

    function describeActions(rule) {
      return safeParse(rule.actions, []).map(item => actionTypeLabel(item.type)).join(', ') || '-'
    }

    function safeParse(text, fallback) {
      try { return typeof text === 'string' ? JSON.parse(text || '[]') : (text || fallback) } catch (_) { return fallback }
    }

    function parseNumberList(value, min, max) {
      return String(value || '')
        .split(',')
        .map(item => Number(item.trim()))
        .filter(item => Number.isInteger(item) && item >= min && item <= max)
    }

    function normalizeEffectiveWindow(window = {}) {
      const monthDays = Array.isArray(window.monthDays) ? window.monthDays : parseNumberList(window.monthDaysText, 1, 31)
      return {
        ...window,
        startTime: window.startTime || '00:00:00',
        endTime: window.endTime || '24:00:00',
        monthDays,
        monthDaysText: monthDays.join(',') || '1'
      }
    }

    function normalizeTrigger(trigger = {}) {
      const normalized = { ...baseTrigger(trigger.type || 'property_change'), ...trigger }
      delete normalized.eventFilter
      normalized.schedule = { ...baseTrigger('cron').schedule, ...(trigger.schedule || {}) }
      if (normalized.type === 'cron' && !normalized.cronMode) {
        normalized.cronMode = normalized.cronExpr ? 'advanced' : 'visual'
      }
      if (normalized.type === 'cron' && normalized.cronMode !== 'advanced') {
        normalized.cronExpr = buildCronExpression(normalized.schedule)
      }
      return normalized
    }

    function boundedNumber(value, min, max, fallback) {
      const n = Number(value)
      if (!Number.isFinite(n)) return fallback
      return Math.min(max, Math.max(min, Math.trunc(n)))
    }

    function cronWeekdays(text) {
      const days = parseNumberList(text, 1, 7)
      return (days.length ? days : [1, 2, 3, 4, 5]).map(day => day === 7 ? 0 : day).join(',')
    }

    function cronMonthDays(text) {
      const days = parseNumberList(text, 1, 31)
      return (days.length ? days : [1]).join(',')
    }

    function buildCronExpression(schedule = {}) {
      const minute = boundedNumber(schedule.minute, 0, 59, 0)
      const hour = boundedNumber(schedule.hour, 0, 23, 0)
      const exprPart = (value, fallback = '*') => String(value || '').trim() || fallback
      switch (schedule.mode) {
        case 'hourly':
          return `${minute} * * * *`
        case 'daily':
          return `${minute} ${hour} * * *`
        case 'weekly':
          return `${minute} ${hour} * * ${cronWeekdays(schedule.weekdaysText)}`
        case 'monthly':
          return `${minute} ${hour} ${cronMonthDays(schedule.monthDaysText)} * *`
        case 'custom_fields':
          return [
            exprPart(schedule.minuteExpr),
            exprPart(schedule.hourExpr),
            exprPart(schedule.dayOfMonthExpr),
            exprPart(schedule.monthExpr),
            exprPart(schedule.weekdayExpr)
          ].join(' ')
        case 'every_minutes':
        default:
          return `*/${boundedNumber(schedule.intervalMinutes, 1, 59, 5)} * * * *`
      }
    }

    function syncCronExpression(trigger) {
      if (!trigger.schedule) trigger.schedule = baseTrigger('cron').schedule
      if (trigger.cronMode !== 'advanced') {
        trigger.cronExpr = buildCronExpression(trigger.schedule)
      }
    }

    function hasConditionNodes(group) {
      return !!group && ((group.conditions || []).length > 0 || (group.groups || []).some(hasConditionNodes))
    }

    function formatTime(value) {
      if (!value) return '-'
      return new Date(Number(value)).toLocaleString()
    }

    function deviceLabel(device) {
      return `${device.name || device.code} (${device.code})`
    }

    function optionLabel(item) {
      return item?.name ? `${item.name} (${optionKey(item)})` : optionKey(item)
    }

    function findDevice(code) {
      return devices.value.find(device => device.code === code)
    }

    function propertiesFor(code) {
      return findDevice(code)?.properties || []
    }

    function eventsFor(code) {
      return findDevice(code)?.events || []
    }

    function servicesFor(code) {
      return findDevice(code)?.services || []
    }

    function deviceName(code) {
      if (!code) return '-'
      const device = findDevice(code)
      return device ? deviceLabel(device) : code
    }

    function propertyName(deviceCode, propertyKey) {
      if (!propertyKey) return '-'
      const prop = propertiesFor(deviceCode).find(item => optionKey(item) === propertyKey)
      return prop ? optionLabel(prop) : propertyKey
    }

    function eventName(deviceCode, eventId) {
      if (!eventId) return '-'
      const evt = eventsFor(deviceCode).find(item => optionKey(item) === eventId)
      return evt ? optionLabel(evt) : eventId
    }

    function serviceName(deviceCode, serviceCode) {
      if (!serviceCode) return '-'
      const service = servicesFor(deviceCode).find(item => optionKey(item) === serviceCode)
      return service ? optionLabel(service) : serviceCode
    }

    function operatorLabel(operator) {
      return {
        changed: t('rule_op_changed'),
        contains: t('rule_op_contains'),
        eq: '=',
        neq: '!=',
        gt: '>',
        gte: '>=',
        lt: '<',
        lte: '<='
      }[operator] || operator || '-'
    }

    function logicLabel(logic) {
      return String(logic || 'and').toLowerCase() === 'or' ? 'OR' : 'AND'
    }

    function openEditor(rule) {
      editingCode.value = rule?.code || ''
      analysis.value = null
      if (rule) {
        assignForm({
          name: rule.name,
          description: rule.description,
          group_id: rule.group_id || null,
          priority: rule.priority || 50,
          throttle_sec: rule.throttle_sec || 60,
          max_per_hour: rule.max_per_hour || 60,
          retry_count: rule.retry_count || 0,
          effective_time: safeParse(rule.effective_time, defaultEffectiveTime()),
          triggers: safeParse(rule.triggers, []),
          conditions: safeParse(rule.conditions, { logic: 'and', conditions: [], groups: [], name: t('rule_condition_group_root', '主条件组') }),
          actions: safeParse(rule.actions, [])
        })
      } else {
        assignForm(defaultForm())
      }
      showEditor.value = true
      analyzeRule()
    }

    function closeEditor() {
      showEditor.value = false
    }

    function addTrigger() {
      form.triggers.push(baseTrigger())
    }

    function removeTrigger(index) {
      form.triggers.splice(index, 1)
    }

    function addCondition() {
      form.conditions.conditions.push(baseCondition())
    }

    function removeCondition(index) {
      form.conditions.conditions.splice(index, 1)
    }

    function addAction() {
      form.actions.push(baseAction())
    }

    function addSequenceGroup() {
      form.actions.push({ ...baseAction(), id: uid('act'), type: 'sequence_group', name: generateUniqueName(t('rule_sequence_group')), subActions: [baseAction(), baseAction()] })
    }

    function addParallelGroup() {
      form.actions.push({ ...baseAction(), id: uid('act'), type: 'parallel_group', name: generateUniqueName(t('rule_parallel_group')), subActions: [baseAction(), baseAction()] })
    }

    function applyEffectiveModeDefaults() {
      if (!form.effective_time.windows || form.effective_time.windows.length === 0) {
        form.effective_time.windows = [{ startTime: '00:00:00', endTime: '24:00:00', monthDays: [], monthDaysText: '1' }]
      }
      form.effective_time.windows = form.effective_time.windows.map(normalizeEffectiveWindow)
      if (form.effective_time.mode === 'weekly' && (!form.effective_time.weekdays || form.effective_time.weekdays.length === 0)) {
        form.effective_time.weekdays = [1, 2, 3, 4, 5, 6, 7]
      }
      if (form.effective_time.mode === 'monthly' && (!form.effective_time.monthDays || form.effective_time.monthDays.length === 0)) {
        form.effective_time.monthDays = [1]
      }
    }

    function addEffectiveWindow() {
      if (!form.effective_time.windows) form.effective_time.windows = []
      form.effective_time.windows.push({ startTime: '00:00:00', endTime: '24:00:00', monthDays: form.effective_time.mode === 'monthly' ? [1] : [], monthDaysText: '1' })
    }

    function removeEffectiveWindow(index) {
      if ((form.effective_time.windows || []).length <= 1) return
      form.effective_time.windows.splice(index, 1)
    }

    function removeAction(index) {
      form.actions.splice(index, 1)
    }

    function addSubAction(action) {
      action.subActions.push(baseAction())
    }

    function removeSubAction(action, index) {
      action.subActions.splice(index, 1)
    }

    function payload(enable = false) {
      const normalizeActionPayload = (action) => {
        const { llmPlayAudio, llmIncludeContext, ...rest } = action
        if (rest.subActions) {
          rest.subActions = rest.subActions.map(normalizeActionPayload)
        }
        return rest
      }
      return {
        name: form.name,
        description: form.description,
        group_id: form.group_id || null,
        priority: form.priority,
        throttle_sec: form.throttle_sec,
        max_per_hour: form.max_per_hour,
        retry_count: form.retry_count,
        effective_time: {
          ...form.effective_time,
          windows: (form.effective_time.windows || []).map(window => {
            const normalized = normalizeEffectiveWindow(window)
            const { monthDaysText, ...payloadWindow } = normalized
            if (form.effective_time.mode !== 'monthly') delete payloadWindow.monthDays
            return payloadWindow
          })
        },
        triggers: form.triggers.map(trigger => normalizeTrigger(trigger)),
        conditions: hasConditionNodes(form.conditions) ? form.conditions : null,
        actions: form.actions.map(normalizeActionPayload),
        enable
      }
    }

    async function analyzeRule() {
      try {
        const res = await axios.post('/api/rules/analyze', payload(false))
        analysis.value = res.data.data
      } catch (_) {
        analysis.value = null
      }
    }

    async function saveRule(enable) {
      saving.value = true
      try {
        if (editingCode.value) {
          await axios.put(`/api/rules/${editingCode.value}`, payload(enable))
        } else {
          await axios.post('/api/rules', payload(enable))
        }
        showEditor.value = false
        await fetchAll()
        showToast('success', t('rule_save_success', '保存成功'))
      } finally {
        saving.value = false
      }
    }

    async function toggleRule(rule, enable) {
      await axios.put(`/api/rules/${rule.code}/${enable ? 'enable' : 'disable'}`)
      await fetchAll()
    }

    async function deleteRule(rule) {
      if (!confirm(t('confirm_delete_rule', { name: rule.name }))) return
      await axios.delete(`/api/rules/${rule.code}`)
      await fetchAll()
    }

    function openNewRuleGraph() {
      graphRule.value = defaultGraphRule()
      showGraph.value = true
    }

    function openRuleGraph(rule) {
      graphRule.value = rule
      showGraph.value = true
    }

    function closeRuleGraph() {
      showGraph.value = false
      graphRule.value = null
    }

    function mapIds(arr) {
      if (!arr) return arr;
      return arr.map(item => {
        if (item._id && !item.id) item.id = item._id.replace(/^node_/, 'act_'); // ensure string format
        if (item.subActions) item.subActions = mapIds(item.subActions);
        return item;
      });
    }

    async function handleGraphUpdate(updatedRule, feedback) {
      saving.value = true
      try {
        const isCreate = !updatedRule.code
        const payload = {
          name: updatedRule.name,
          description: updatedRule.description,
          group_id: updatedRule.group_id || null,
          priority: updatedRule.priority || 0,
          throttle_sec: updatedRule.throttle_sec || updatedRule.throttleSec || 0,
          max_per_hour: updatedRule.max_per_hour || 0,
          retry_count: updatedRule.retry_count || 0,
          effective_time: updatedRule.effective_time,
          triggers: (updatedRule.triggers || []).map(t => {
            if (t._id && !t.id) t.id = t._id;
            return normalizeTrigger(t);
          }),
          conditions: hasConditionNodes(updatedRule.conditions) ? updatedRule.conditions : null,
          actions: mapIds(updatedRule.actions || []),
          enable: updatedRule.status === 'enabled'
        }
        const res = isCreate
          ? await axios.post('/api/rules', payload)
          : await axios.put(`/api/rules/${updatedRule.code}`, payload)
        const savedRule = normalizeRule(res.data.data || {})
        graphRule.value = {
          ...updatedRule,
          ...payload,
          ...savedRule,
          code: savedRule.code || updatedRule.code,
          status: savedRule.status || updatedRule.status || 'draft'
        }
        await fetchAll()
        const message = t('rule_save_success', '保存成功')
        showToast('success', message)
        feedback?.done?.(true, message, {
          id: savedRule.id,
          code: savedRule.code,
          status: savedRule.status,
          enabled: savedRule.enabled
        })
      } catch (err) {
        console.error('Failed to save rule from graph:', err)
        const message = err.response?.data?.message || t('common_save_fail', '保存失败')
        showToast('danger', message)
        feedback?.done?.(false, message)
      } finally {
        saving.value = false
      }
    }

    async function openLogs(rule) {
      logRule.value = rule
      const res = await axios.get(`/api/rules/${rule.code}/logs`, { params: { page: 1, pageSize: 50 } })
      logs.value = res.data.data || []
      showLogs.value = true
    }

    function openGroupModal() {
      groupForm.name = ''
      showGroupModal.value = true
    }

    async function saveGroup() {
      await axios.post('/api/rule-groups', { name: groupForm.name })
      groupForm.name = ''
      const res = await axios.get('/api/rule-groups')
      groups.value = res.data.data || []
    }

    async function deleteGroup(group) {
      if (!confirm(t('confirm_delete_rule_group', { name: group.name }))) return
      await axios.delete(`/api/rule-groups/${group.id || group.ID}`)
      const res = await axios.get('/api/rule-groups')
      groups.value = res.data.data || []
    }

    const actionLabels = computed(() => ({
      type: t('type'),
      device: t('device'),
      property: t('property'),
      value: t('value'),
      service: t('service'),
      params: t('params'),
      title: t('title'),
      content: t('content'),
      level: t('level'),
      info: t('rule_alarm_info'),
      warning: t('rule_alarm_warning'),
      critical: t('rule_alarm_critical'),
      delaySec: t('rule_delay_sec'),
      selectDevice: t('select_device'),
      selectProperty: t('select_properties'),
      selectService: t('rule_select_service'),
      setProperty: t('rule_action_set_property'),
      callService: t('rule_action_call_service'),
      notification: t('rule_action_notification'),
      alarm: t('rule_action_alarm'),
      delay: t('rule_action_delay'),
      llm: t('rule_action_llm'),
      voicePlayback: t('rule_action_voice_playback'),
      llmPrompt: t('rule_action_llm_prompt'),
      voiceText: t('rule_action_voice_text'),
      yes: t('yes'),
      sequenceGroup: t('rule_sequence_group'),
      parallelGroup: t('rule_parallel_group'),
      addAction: t('rule_add_action')
    }))

    const conditionLabels = computed(() => ({
      type: t('type'),
      device: t('device'),
      property: t('property'),
      operator: t('operator'),
      value: t('value'),
      status: t('status'),
      start: t('rule_time_start'),
      end: t('rule_time_end'),
      selectDevice: t('select_device'),
      selectProperty: t('select_properties'),
      propertyCondition: t('rule_condition_property'),
      statusCondition: t('rule_condition_status'),
      timeCondition: t('rule_condition_time'),
      contains: t('rule_op_contains'),
      online: t('dev_online'),
      offline: t('dev_offline'),
      and: t('logic_and'),
      or: t('logic_or'),
      addCondition: t('rule_add_condition'),
      addGroup: t('rule_add_condition_group'),
      conditionGroup: t('rule_condition_group'),
      noConditions: t('rule_no_conditions')
    }))

    watch(() => JSON.stringify(payload(false)), () => {
      if (showEditor.value) analyzeRule()
    })

    onMounted(fetchAll)

    return {
      expandedSections,
      rules, groups, devices, logs, loading, saving, search, statusFilter, groupFilter, showEditor, showLogs, showGraph,
      showGroupModal, editingCode, logRule, graphRule, groupForm, form, analysis, filteredRules, summaryCards,
      validationMessage, executionPreview, weekdayOptions, effectiveMonthDaysText, effectiveMonthsText,
      showsEffectiveWeekdays, showsEffectiveMonthDays, showsEffectiveMonths,
      actionLabels, conditionLabels, fetchAll, groupName, statusBadge, statusLabel,
      syncStateLabel, describeTriggers, describeActions, localizedRuleError, formatTime, deviceLabel, optionLabel, optionKey, propertiesFor,
      eventsFor, openNewRuleGraph, openEditor, closeEditor, addTrigger, removeTrigger, addCondition, removeCondition, addAction,
      addSequenceGroup, addParallelGroup, addEffectiveWindow, removeEffectiveWindow, applyEffectiveModeDefaults,
      parseNumberList, syncCronExpression,
      removeAction, addSubAction, removeSubAction, saveRule, toggleRule, deleteRule,
      openRuleGraph, closeRuleGraph, handleGraphUpdate, triggerTypeLabel, openLogs, openGroupModal, saveGroup, deleteGroup
    }
  }
}
</script>

<style>
.cursor-pointer { cursor: pointer; }
.rule-desc {
  max-width: 280px;
}

.rule-modal {
  background: rgba(0, 0, 0, 0.55);
}

.rule-editor-dialog {
  max-width: min(1140px, calc(100vw - 2rem));
}

.rule-editor-dialog .modal-content,
.rule-editor-dialog .modal-body {
  overflow-x: hidden;
}

.rule-graph-dialog {
  max-width: min(1240px, calc(100vw - 2rem));
}

.rule-graph-summary {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.rule-graph-summary > div {
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  padding: 0.75rem;
  background: var(--bs-tertiary-bg);
  min-width: 0;
}

.graph-meta-label {
  display: block;
  color: var(--bs-secondary-color);
  font-size: 0.72rem;
  margin-bottom: 0.2rem;
}

.rule-graph-canvas {
  display: grid;
  grid-template-columns: repeat(4, minmax(13.5rem, 1fr));
  gap: 0.9rem;
  align-items: stretch;
  padding: 1rem;
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  background:
    linear-gradient(90deg, rgba(var(--bs-primary-rgb), 0.06) 0 1px, transparent 1px 100%),
    var(--bs-body-bg);
  background-size: 2.5rem 2.5rem;
}

.rule-graph-section {
  position: relative;
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  padding: 0.85rem;
  background: rgba(var(--bs-body-bg-rgb), 0.95);
  min-width: 0;
}

.rule-graph-section__header {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  margin-bottom: 0.75rem;
}

.rule-graph-section__header h6 {
  margin: 0;
  font-weight: 700;
}

.rule-graph-section__icon {
  width: 2rem;
  height: 2rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  color: #fff;
  background: var(--bs-primary);
}

.rule-graph-kicker {
  color: var(--bs-secondary-color);
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
}

.rule-graph-section--trigger .rule-graph-section__icon {
  background: #0d6efd;
}

.rule-graph-section--condition .rule-graph-section__icon {
  background: #6f42c1;
}

.rule-graph-section--time .rule-graph-section__icon {
  background: #198754;
}

.rule-graph-section--action .rule-graph-section__icon {
  background: #fd7e14;
}

.rule-graph-node-list,
.rule-graph-children {
  display: grid;
  gap: 0.65rem;
}

.rule-graph-node {
  border: 1px solid var(--bs-border-color);
  border-left: 4px solid var(--bs-primary);
  border-radius: 8px;
  padding: 0.7rem;
  background: var(--bs-tertiary-bg);
  min-width: 0;
}

.rule-graph-node--child {
  background: var(--bs-body-bg);
}

.rule-graph-node--empty {
  border-style: dashed;
  border-left-color: var(--bs-secondary-color);
  color: var(--bs-secondary-color);
}

.rule-graph-node__title {
  font-weight: 700;
  line-height: 1.3;
  word-break: break-word;
}

.rule-graph-node__detail {
  color: var(--bs-secondary-color);
  font-size: 0.82rem;
  line-height: 1.45;
  margin-top: 0.25rem;
  overflow-wrap: anywhere;
}

.rule-graph-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-top: 0.45rem;
}

.rule-graph-badge {
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

.rule-graph-children {
  position: relative;
  margin-top: 0.65rem;
  padding-left: 0.7rem;
}

.rule-graph-children::before {
  content: '';
  position: absolute;
  left: 0.1rem;
  top: 0.2rem;
  bottom: 0.2rem;
  width: 2px;
  background: var(--bs-border-color);
}

.rule-graph-arrow {
  position: absolute;
  top: 50%;
  right: -1.35rem;
  z-index: 2;
  width: 1.75rem;
  height: 1.75rem;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--bs-border-color);
  border-radius: 999px;
  color: var(--bs-primary);
  background: var(--bs-body-bg);
  transform: translateY(-50%);
}

.rule-panel,
.rule-step {
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  padding: 1rem;
  background: var(--bs-body-bg);
}

.rule-builder {
  display: grid;
  gap: 1rem;
}

.rule-step__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.step-kicker {
  display: block;
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--bs-primary);
  text-transform: uppercase;
}

.builder-item {
  border: 1px solid var(--bs-border-color);
  border-radius: 8px;
  padding: 0.85rem;
  margin-bottom: 0.85rem;
  background: var(--bs-body-bg);
  box-shadow: 0 1px 3px rgba(0,0,0,0.02);
  transition: all 0.2s;
}

.builder-item:hover {
  border-color: var(--bs-primary);
  box-shadow: 0 2px 8px rgba(var(--bs-primary-rgb), 0.08);
}

.builder-item--nested {
  background: var(--bs-tertiary-bg);
}

.builder-line {
  display: flex;
  flex-wrap: nowrap;
  align-items: flex-end;
  gap: 0.5rem;
  padding-bottom: 0.15rem;
  max-width: 100%;
}

.builder-line > [class*="col"] {
  flex: 1 1 8.75rem;
  min-width: 0;
  max-width: none;
}

.builder-line > .col-md-2 {
  flex-basis: 7rem;
}

.builder-line > .col-md-3 {
  flex-basis: 9.5rem;
}

.builder-line > .col-md-4 {
  flex-basis: 13rem;
}

.builder-line > .col-md-5 {
  flex-basis: 16rem;
}

.builder-line > .col-md-7 {
  flex-basis: 22rem;
}

.builder-line > .col {
  flex: 0 0 2.75rem;
}

/* === 新增：树状结构条件组与动作组强化 UI === */
.condition-group-card, .action-group-container {
  border: 1px solid var(--bs-border-color);
  border-left: 4px solid var(--group-color, #6f42c1);
  border-radius: 8px;
  padding: 1rem 1rem 1rem 1.5rem;
  background: var(--group-bg, rgba(0,0,0,0.02));
  margin-bottom: 1rem;
  box-shadow: 0 2px 4px rgba(0,0,0,0.02);
  position: relative;
  transition: all 0.2s ease;
}

.condition-group-card--nested, .action-group-container--nested {
  margin-top: 1rem;
  margin-left: 1.5rem;
}

.condition-group-card__header, .action-group-container__header {
  border-bottom: 1px dashed var(--bs-border-color);
  margin-bottom: 1rem !important;
  padding-bottom: 0.75rem !important;
  position: relative;
  min-height: 28px;
  opacity: 1;
}

.toggle-btn {
  position: absolute;
  left: -1.5rem;
  top: 50%;
  transform: translate(calc(-50% - 2px), -50%);
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--bs-body-bg);
  border: 1px solid var(--group-color);
  z-index: 3;
  padding: 0;
  transition: all 0.2s;
  cursor: pointer;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
}

.toggle-btn:hover {
  background: var(--bs-tertiary-bg);
}

.toggle-btn i {
  font-size: 0.75rem;
}

.group-title {
  font-size: 0.9rem;
  letter-spacing: 0.02em;
}

.logic-switch button {
  padding: 0.25rem 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
}

/* Horizontal branches */
.condition-item-wrapper::before,
.builder-item-wrapper::before,
.action-sub-wrapper::before {
  content: "";
  position: absolute;
  left: -1.5rem;
  top: 29px;
  width: 1.5rem;
  height: 2px;
  background: var(--group-color);
  z-index: 0;
}

.logic-badge {
  position: absolute;
  top: 19px;
  left: -0.75rem;
  transform: translateX(-50%);
  z-index: 2;
  background: var(--group-color);
  color: #fff;
  font-size: 0.65rem;
  font-weight: 700;
  padding: 0.15rem 0.35rem;
  border-radius: 4px;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  text-transform: uppercase;
}


.builder-item--group {
  padding-bottom: 1rem;
  border-color: rgba(253, 126, 20, 0.3);
  border-left: 4px solid #fd7e14;
  background: var(--bs-body-bg);
}

.builder-item-wrapper {
  margin-bottom: 0.75rem;
}

.builder-empty {
  border: 1px dashed var(--bs-border-color);
  border-radius: 8px;
  padding: 1rem;
  color: var(--bs-secondary-color);
  text-align: center;
}

.form-label {
  font-size: 0.78rem;
  color: var(--bs-secondary-color);
  margin-bottom: 0.25rem;
}

.config-help {
  color: var(--bs-secondary-color);
  font-size: 0.72rem;
  line-height: 1.35;
  margin-top: 0.25rem;
}

@media (max-width: 991.98px) {
  .rule-graph-summary,
  .rule-graph-canvas {
    grid-template-columns: 1fr;
  }

  .rule-graph-arrow {
    position: static;
    margin: 0.5rem auto -0.2rem;
    transform: rotate(90deg);
  }
}

.spin {
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>

