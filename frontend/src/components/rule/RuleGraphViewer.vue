<template>
  <div class="rule-graph-viewer" v-if="rule" ref="graphViewerRef">
    <!-- 顶部工具栏 -->
    <div class="rg-toolbar text-end mb-3">
      <button class="btn btn-sm btn-outline-info me-2" @click="exportToImage">
        <i class="bi bi-image"></i> {{ $t('export_image', '导出图片') }}
      </button>
      <button class="btn btn-sm btn-outline-danger" @click="exportToPdf">
        <i class="bi bi-file-pdf"></i> {{ $t('export_pdf', '导出PDF') }}
      </button>
    </div>

    <!-- 规则概览面板 -->
    <div class="rg-summary">
      <div class="rg-summary__item">
        <div class="rg-summary__icon rg-summary__icon--primary">
          <i class="bi bi-diagram-3"></i>
        </div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('rule_name') }}</span>
          <strong>{{ rule.name }}</strong>
        </div>
      </div>
      <div class="rg-summary__item">
        <div class="rg-summary__icon rg-summary__icon--scope">
          <i class="bi bi-hdd-rack"></i>
        </div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('rule_scope') }}</span>
          <strong>{{ rule.scope === 'gateway' ? $t('rule_scope_gateway') : $t('rule_scope_platform') }}</strong>
        </div>
      </div>
      <div class="rg-summary__item">
        <div class="rg-summary__icon rg-summary__icon--status">
          <i class="bi bi-activity"></i>
        </div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('status') }}</span>
          <span class="rg-status" :class="`rg-status--${rule.status}`">
            {{ statusLabel(rule.status) }}
          </span>
        </div>
      </div>
      <!-- 防护配置 -->
      <div class="rg-summary__item" v-if="rule.throttleSec || rule.maxPerHour">
        <div class="rg-summary__icon rg-summary__icon--shield">
          <i class="bi bi-shield-check"></i>
        </div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('rule_graph_protection') }}</span>
          <span class="rg-summary__detail" v-if="rule.throttleSec">{{ $t('rule_graph_throttle', { sec: rule.throttleSec }) }}</span>
          <span class="rg-summary__detail" v-if="rule.maxPerHour">{{ $t('rule_graph_max_per_hour', { count: rule.maxPerHour }) }}</span>
        </div>
      </div>
    </div>

    <!-- 新增：时间展示独立板块 -->
    <div class="rg-effective-time-panel" v-if="effectiveNodes.length">
      <div class="rg-effective-time-panel__header">
        <i class="bi bi-calendar-check-fill"></i>
        <span>{{ $t('rule_effective_time') }}</span>
      </div>
      <div class="rg-effective-time-panel__body">
        <RgCard v-for="node in effectiveNodes" :key="node.id" :node="node" />
      </div>
    </div>

    <!-- 流程图画布 -->
    <div class="rg-canvas">
      <div class="rg-canvas__scan"></div>

      <!-- === WHEN 触发条件 === -->
      <section class="rg-section rg-section--trigger">
        <div class="rg-section__header">
          <span class="rg-section__pill">
            <i class="bi bi-lightning-charge-fill"></i>
            <span class="rg-section__kicker">{{ $t('rule_when') }}</span>
            <span class="rg-section__title">{{ $t('rule_triggers') }}</span>
          </span>
          <span class="rg-section__hint">{{ $t('rule_graph_trigger_or') }}</span>
        </div>

        <div class="rg-triggers">
          <template v-for="(node, i) in triggerNodes" :key="node.id">
            <RgCard :node="node" />
            <div v-if="i < triggerNodes.length - 1" class="rg-logic-diamond rg-logic-diamond--or">
              <span>{{ $t('rule_graph_logic_or') }}</span>
            </div>
          </template>
        </div>
      </section>

      <!-- 连接线 WHEN → IF -->
      <div class="rg-connector">
        <div class="rg-connector__line"></div>
        <div class="rg-connector__dot"></div>
      </div>

      <!-- === IF 判断条件 === -->
      <section class="rg-section rg-section--condition">
        <div class="rg-section__header">
          <span class="rg-section__pill">
            <i class="bi bi-funnel-fill"></i>
            <span class="rg-section__kicker">{{ $t('rule_if') }}</span>
            <span class="rg-section__title">{{ $t('rule_conditions') }}</span>
          </span>
          <span class="rg-section__hint" v-if="!hasConditions">{{ $t('rule_graph_condition_skip') }}</span>
        </div>

        <div class="rg-conditions" v-if="hasConditions">
          <RgConditionGroup :group="conditionTree" :devices="devices" :depth="0" />
        </div>
        <div v-else class="rg-empty-hint">
          <i class="bi bi-skip-forward-circle"></i>
          {{ $t('rule_no_conditions') }}
        </div>
      </section>

      <!-- 连接线 IF → THEN (直接连接) -->
      <div class="rg-connector">
        <div class="rg-connector__line"></div>
        <div class="rg-connector__dot"></div>
      </div>

      <!-- === THEN 执行动作 === -->
      <section class="rg-section rg-section--action">
        <div class="rg-section__header">
          <span class="rg-section__pill">
            <i class="bi bi-play-circle-fill"></i>
            <span class="rg-section__kicker">{{ $t('rule_then') }}</span>
            <span class="rg-section__title">{{ $t('rule_actions') }}</span>
          </span>
        </div>

        <div class="rg-actions">
          <template v-for="(node, i) in actionNodes" :key="node.id">
            <RgActionNode :node="node" :colorIndex="i" />

            <!-- 串行步骤间连线 -->
            <div v-if="i < actionNodes.length - 1 && !node.isDelay" class="rg-step-connector">
              <div class="rg-step-connector__line"></div>
              <div class="rg-step-connector__arrow">▼</div>
            </div>
          </template>
        </div>
      </section>
    </div>
  </div>
</template>

<script>
import { defineComponent, computed, h, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import html2canvas from 'html2canvas'
import { jsPDF } from 'jspdf'

/* ============================================
 *  递归卡片节点组件 — 展示单个条件/动作的详情
 * ============================================ */
const RgCard = defineComponent({
  name: 'RgCard',
  props: {
    node: { type: Object, required: true }
  },
  setup(props) {
    return () => {
      const n = props.node
      const iconEl = h('div', { class: ['rg-card__icon', `rg-card__icon--${n.tone}`] }, [
        h('i', { class: ['bi', n.icon || 'bi-gear'] })
      ])
      const titleEl = h('div', { class: 'rg-card__title' }, n.title)
      const detailEl = n.detail
        ? h('div', { class: 'rg-card__detail' }, n.detail)
        : null
      const badgesEl = n.badges?.length
        ? h('div', { class: 'rg-card__badges' },
            n.badges.map(b => h('span', { class: ['rg-card__badge', `rg-card__badge--${n.tone}`], key: b }, b))
          )
        : null
      const textEl = h('div', { class: 'rg-card__text' }, [titleEl, detailEl])

      return h('div', {
        class: [
          'rg-card',
          `rg-card--${n.tone}`,
          { 'rg-card--empty': n.empty }
        ]
      }, [iconEl, textEl, badgesEl])
    }
  }
})

/* ============================================
 *  递归条件组组件 — 展示 AND/OR 嵌套条件
 * ============================================ */
const RgConditionGroup = defineComponent({
  name: 'RgConditionGroup',
  props: {
    group: { type: Object, required: true },
    devices: { type: Array, default: () => [] },
    depth: { type: Number, default: 0 },
    colorIndex: { type: Number, default: 0 }
  },
  setup(props) {
    const { t } = useI18n()
    const collapsed = ref(false)

    function findDevice(code) {
      return props.devices.find(d => d.code === code)
    }
    function deviceName(code) {
      if (!code) return '-'
      const d = findDevice(code)
      return d ? (d.name || d.code) : code
    }
    function optKey(item) { return item?.key || item?.identifier || '' }
    function propName(devCode, propKey) {
      if (!propKey) return '-'
      const dev = findDevice(devCode)
      const p = (dev?.properties || []).find(item => optKey(item) === propKey)
      return p ? (p.name || optKey(p)) : propKey
    }
    function opLabel(op) {
      const map = {
        changed: t('rule_op_changed'), contains: t('rule_op_contains'),
        eq: '=', neq: '≠', gt: '>', gte: '≥', lt: '<', lte: '≤'
      }
      return map[op] || op || '-'
    }

    return () => {
      const g = props.group
      const isOr = (g.logic || 'and').toLowerCase() === 'or'
      const logicLabel = isOr ? t('rule_graph_logic_or') : t('rule_graph_logic_and')
      const hintLabel = isOr ? t('rule_graph_any_match') : t('rule_graph_all_match')
      const leafConditions = g.conditions || []
      const subGroups = g.groups || []
      const allItems = []

      // 叶子条件转成卡片
      leafConditions.forEach((c, i) => {
        let icon = 'bi-wrench-adjustable'
        let title = t('rule_condition_property')
        let detail = ''
        if (c.type === 'device_status') {
          icon = 'bi-toggle-on'
          title = t('rule_condition_status')
          detail = `${deviceName(c.deviceCode)} = ${c.statusValue === 'offline' ? t('dev_offline') : t('dev_online')}`
        } else if (c.type === 'time_range') {
          icon = 'bi-stopwatch'
          title = t('rule_condition_time')
          detail = `${c.startTime || '00:00'} ~ ${c.endTime || '24:00'}`
          if (c.weekdays?.length) detail += ` [${c.weekdays.join(',')}]`
        } else {
          detail = `${deviceName(c.deviceCode)} / ${propName(c.deviceCode, c.propertyKey)} ${opLabel(c.operator)} ${c.value ?? ''}`
        }
        allItems.push({
          type: 'leaf',
          key: `cond-${props.depth}-${i}`,
          node: { id: c.id || `c${i}`, title, detail, icon, tone: 'condition', badges: [] }
        })
      })

      // 嵌套子组
      subGroups.forEach((sg, i) => {
        allItems.push({
          type: 'group',
          key: `group-${props.depth}-${i}`,
          group: sg
        })
      })

      if (!allItems.length) return null

      // 构建逻辑标签 header
      const headerEl = h('div', { class: 'rg-cond-group__header' }, [
        h('div', { class: ['rg-logic-pill', isOr ? 'rg-logic-pill--or' : 'rg-logic-pill--and'] }, [
          h('span', { class: 'rg-logic-pill__label' }, logicLabel),
          h('span', { class: 'rg-logic-pill__hint' }, hintLabel)
        ]),
        allItems.length > 2
          ? h('button', {
              class: 'rg-cond-group__toggle',
              onClick: (e) => { e.stopPropagation(); collapsed.value = !collapsed.value }
            }, [
              h('i', { class: ['bi', collapsed.value ? 'bi-chevron-down' : 'bi-chevron-up'] })
            ])
          : null
      ])

      // 折叠时只显示摘要
      if (collapsed.value) {
        return h('div', { class: ['rg-cond-group', `rg-cond-group--depth-${props.depth}`] }, [
          headerEl,
          h('div', {
            class: 'rg-cond-group__collapsed',
            onClick: () => { collapsed.value = false }
          }, [
            h('i', { class: 'bi bi-plus-circle-dotted' }),
            h('span', t('rule_graph_expand', { count: allItems.length }))
          ])
        ])
      }

      // 渲染子项
      const itemEls = []
      let childGroupCount = 0
      allItems.forEach((item, idx) => {
        if (item.type === 'leaf') {
          itemEls.push(h(RgCard, { node: item.node, key: item.key }))
        } else {
          itemEls.push(h(RgConditionGroup, {
            group: item.group,
            devices: props.devices,
            depth: props.depth + 1,
            colorIndex: (props.colorIndex + childGroupCount + 1) % 6,
            key: item.key
          }))
          childGroupCount++
        }
      })

      return h('div', { class: ['rg-cond-group', `rg-group-color-${props.colorIndex}`] }, [
        headerEl,
        h('div', { class: 'rg-cond-group__items' }, itemEls)
      ])
    }
  }
})

/* ============================================
 *  递归动作节点组件 — 支持串行、并行互相嵌套
 * ============================================ */
const RgActionNode = defineComponent({
  name: 'RgActionNode',
  props: {
    node: { type: Object, required: true },
    colorIndex: { type: Number, default: 0 }
  },
  setup(props) {
    const { t } = useI18n()
    return () => {
      const node = props.node
      if (node.isParallel) {
        return h('div', { class: ['rg-parallel-group', `rg-group-color-${props.colorIndex % 6}`] }, [
          h('div', { class: 'rg-parallel-group__label' }, [
            h('i', { class: 'bi bi-cpu' }),
            t('rule_graph_parallel_run'),
            h('span', { class: 'rg-parallel-group__count' }, `(${node.children.length})`)
          ]),
          h('div', { class: 'rg-parallel-fork' }, [
            h('div', { class: 'rg-parallel-fork__stem' }),
            h('div', { class: 'rg-parallel-fork__rail' })
          ]),
          h('div', { class: 'rg-parallel-branches' }, node.children.map((child, i) => {
            return h('div', { class: 'rg-parallel-branch', key: child.id }, [
              h('div', { class: 'rg-parallel-branch__line' }),
              h(RgActionNode, { node: child, colorIndex: props.colorIndex + i + 1 })
            ])
          })),
          h('div', { class: 'rg-parallel-join' }, [
            h('div', { class: 'rg-parallel-join__rail' }),
            h('div', { class: 'rg-parallel-join__stem' })
          ])
        ])
      } else if (node.isSerial) {
        return h('div', { class: ['rg-serial-group', `rg-group-color-${props.colorIndex % 6}`] }, [
          h('div', { class: 'rg-serial-group__label' }, [
            h('i', { class: 'bi bi-list-ol' }),
            t('rule_graph_serial_run'),
            h('span', { class: 'rg-serial-group__count' }, `(${node.children.length})`)
          ]),
          h('div', { class: 'rg-serial-steps' }, node.children.map((child, ci) => {
            const arr = [ h(RgActionNode, { node: child, colorIndex: props.colorIndex + ci + 1, key: child.id }) ]
            if (ci < node.children.length - 1) {
              arr.push(h('div', { class: 'rg-serial-arrow', key: child.id + '-arrow' }, [
                h('div', { class: 'rg-serial-arrow__line' }),
                h('div', { class: 'rg-serial-arrow__pulse' })
              ]))
            }
            return arr
          }).flat())
        ])
      } else if (node.isDelay) {
        return h('div', { class: 'rg-delay-node' }, [
          h('div', { class: 'rg-delay-node__icon' }, [ h('i', { class: 'bi bi-hourglass-split' }) ]),
          h('span', { class: 'rg-delay-node__text' }, t('rule_graph_delay_wait', { sec: node.delaySec }))
        ])
      } else {
        return h(RgCard, { node })
      }
    }
  }
})

export default {
  name: 'RuleGraphViewer',
  components: { RgCard, RgConditionGroup, RgActionNode },
  props: {
    rule: { type: Object, default: () => null },
    devices: { type: Array, default: () => [] }
  },
  setup(props) {
    const { t } = useI18n()

    // === 工具函数 ===
    function safeParse(text, fallback) {
      try { return typeof text === 'string' ? JSON.parse(text || '[]') : (text || fallback) } catch (_) { return fallback }
    }
    function findDevice(code) { return props.devices.find(d => d.code === code) }
    function deviceName(code) {
      if (!code) return '-'
      const d = findDevice(code)
      return d ? `${d.name || d.code}` : code
    }
    function optKey(item) { return item?.key || item?.identifier || '' }
    function propName(devCode, propKey) {
      if (!propKey) return propKey
      const dev = findDevice(devCode)
      const p = (dev?.properties || []).find(item => optKey(item) === propKey)
      return p ? (p.name || optKey(p)) : propKey
    }
    function eventName(devCode, eventId) {
      if (!eventId) return eventId
      const dev = findDevice(devCode)
      const e = (dev?.events || []).find(item => optKey(item) === eventId)
      return e ? (e.name || optKey(e)) : eventId
    }
    function serviceName(devCode, serviceCode) {
      if (!serviceCode) return serviceCode
      const dev = findDevice(devCode)
      const s = (dev?.services || []).find(item => optKey(item) === serviceCode)
      return s ? (s.name || optKey(s)) : serviceCode
    }
    function opLabel(op) {
      const map = {
        changed: t('rule_op_changed'), contains: t('rule_op_contains'),
        increased_by: '↑', decreased_by: '↓',
        eq: '=', neq: '≠', gt: '>', gte: '≥', lt: '<', lte: '≤'
      }
      return map[op] || op || '-'
    }
    function statusLabel(status) {
      const map = {
        enabled: t('rule_status_enabled'), disabled: t('rule_status_disabled'),
        draft: t('rule_status_draft'), error: t('rule_status_error')
      }
      return map[status] || status
    }
    function alarmLevelLabel(level) {
      const map = {
        info: t('rule_alarm_info'), warning: t('rule_alarm_warning'),
        critical: t('rule_alarm_critical'), danger: t('rule_alarm_danger')
      }
      return map[level] || level
    }

    // === 触发条件节点 ===
    const triggerNodes = computed(() => {
      const triggers = safeParse(props.rule?.triggers, [])
      if (!triggers.length) {
        return [{ id: 'trig-empty', title: t('rule_trigger_required'), detail: '', icon: 'bi-exclamation-circle', tone: 'trigger', badges: [], empty: true }]
      }
      return triggers.map((trig, i) => {
        let icon = 'bi-activity', title = '', detail = '', badges = []
        switch (trig.type) {
          case 'cron':
            icon = 'bi-clock-history'
            title = t('rule_trigger_cron')
            detail = trig.cronExpr || trig.cronDesc || '-'
            badges = [trig.cronMode === 'advanced' ? 'Cron' : t('rule_cron_mode_visual')]
            break
          case 'event':
            icon = 'bi-broadcast-pin'
            title = t('rule_trigger_event')
            detail = `${deviceName(trig.deviceCode)} → ${eventName(trig.deviceCode, trig.eventId)}`
            break
          case 'device_status':
            icon = 'bi-power'
            title = t('rule_trigger_status')
            detail = `${deviceName(trig.deviceCode)} → ${trig.statusValue === 'offline' ? t('dev_offline') : t('dev_online')}`
            break
          default: // property_change
            icon = 'bi-graph-up-arrow'
            title = t('rule_trigger_property')
            detail = `${deviceName(trig.deviceCode)} / ${propName(trig.deviceCode, trig.propertyKey)}`
            if (trig.operator && trig.operator !== 'changed') {
              detail += ` ${opLabel(trig.operator)} ${trig.value ?? ''}`
            } else if (trig.operator === 'changed') {
              detail += ` [${t('rule_op_changed')}]`
            }
        }
        return { id: trig.id || `trig-${i}`, title, detail, icon, tone: 'trigger', badges }
      })
    })

    // === 判断条件树 ===
    function hasConditionContent(group) {
      if (!group) return false
      if ((group.conditions || []).length > 0) return true
      return (group.groups || []).some(hasConditionContent)
    }
    const conditionTree = computed(() => safeParse(props.rule?.conditions, null))
    const hasConditions = computed(() => hasConditionContent(conditionTree.value))

    // === 生效时间节点 ===
    const effectiveNodes = computed(() => {
      const et = safeParse(props.rule?.effective_time, null)
      const mode = et?.mode || 'always'
      const labels = {
        always: t('rule_effective_always'), daily: t('rule_effective_daily'),
        weekly: t('rule_effective_weekly'), monthly: t('rule_effective_monthly'),
        workday: t('rule_effective_workday'), holiday: t('rule_effective_holiday'),
        custom: t('rule_effective_custom')
      }
      if (mode === 'always' || !et) {
        return [{ id: 'eff-always', title: labels.always, detail: '00:00 ~ 24:00', icon: 'bi-infinity', tone: 'time', badges: [] }]
      }
      const windows = Array.isArray(et.windows) && et.windows.length
        ? et.windows
        : [{ startTime: et.startTime || '00:00', endTime: et.endTime || '24:00' }]
      return windows.map((w, i) => {
        const badges = []
        if (mode === 'weekly' && et.weekdays?.length) badges.push(`${t('rule_effective_weekdays')}: ${et.weekdays.join(',')}`)
        if (mode === 'monthly' && w.monthDays?.length) badges.push(`${t('rule_effective_month_days')}: ${w.monthDays.join(',')}`)
        return { id: `eff-${i}`, title: labels[mode] || mode, detail: `${w.startTime || '00:00'} ~ ${w.endTime || '24:00'}`, icon: 'bi-calendar3-range', tone: 'time', badges }
      })
    })

    // === 执行动作节点 ===
    function buildActionNode(action, prefix) {
      const base = { id: action.id || prefix, tone: 'action', badges: [] }
      if (action.type === 'parallel_group') {
        return {
          ...base,
          isParallel: true,
          children: (action.subActions || []).map((sa, j) => buildActionNode(sa, `${prefix}-p${j}`))
        }
      }
      if (action.type === 'sequence_group') {
        return {
          ...base,
          isSerial: true,
          children: (action.subActions || []).map((sa, j) => buildActionNode(sa, `${prefix}-s${j}`))
        }
      }
      if (action.type === 'delay') {
        return { ...base, isDelay: true, delaySec: action.delaySec || 0, icon: 'bi-hourglass-split', title: t('rule_action_delay') }
      }
      if (action.type === 'set_property') {
        return {
          ...base,
          icon: 'bi-pencil-square',
          title: t('rule_action_set_property'),
          detail: `${deviceName(action.deviceCode)} / ${propName(action.deviceCode, action.propertyKey)} = ${action.value ?? ''}`
        }
      }
      if (action.type === 'call_service') {
        return {
          ...base,
          icon: 'bi-gear-wide-connected',
          title: t('rule_action_call_service'),
          detail: `${deviceName(action.deviceCode)} → ${serviceName(action.deviceCode, action.serviceCode)}`
        }
      }
      if (action.type === 'notification') {
        return {
          ...base,
          icon: 'bi-chat-left-dots',
          title: t('rule_action_notification'),
          detail: action.notifyTitle || action.notifyContent || '-'
        }
      }
      if (action.type === 'alarm') {
        return {
          ...base,
          icon: 'bi-exclamation-triangle-fill',
          title: t('rule_action_alarm'),
          detail: action.alarmTitle || action.alarmContent || '-',
          badges: [alarmLevelLabel(action.alarmLevel)]
        }
      }
      // 未知类型回退
      return { ...base, icon: 'bi-question-circle', title: action.type, detail: JSON.stringify(action).substring(0, 80) }
    }

    const actionNodes = computed(() => {
      const actions = safeParse(props.rule?.actions, [])
      if (!actions.length) {
        return [{ id: 'act-empty', title: t('rule_action_required'), detail: '', icon: 'bi-exclamation-circle', tone: 'action', badges: [], empty: true }]
      }
      return actions.map((a, i) => buildActionNode(a, `act-${i}`))
    })

    const graphViewerRef = ref(null)

    const exportToImage = async () => {
      if (!graphViewerRef.value) return
      const canvas = await html2canvas(graphViewerRef.value, { scale: 2, useCORS: true })
      const link = document.createElement('a')
      link.download = `rule-${props.rule?.name || 'graph'}.png`
      link.href = canvas.toDataURL()
      link.click()
    }

    const exportToPdf = async () => {
      if (!graphViewerRef.value) return
      const canvas = await html2canvas(graphViewerRef.value, { scale: 2, useCORS: true })
      const imgData = canvas.toDataURL('image/png')
      const pdf = new jsPDF('p', 'mm', 'a4')
      const pdfWidth = pdf.internal.pageSize.getWidth()
      let pdfHeight = (canvas.height * pdfWidth) / canvas.width
      
      if (pdfHeight > pdf.internal.pageSize.getHeight()) {
        let heightLeft = pdfHeight
        let position = 0
        pdf.addImage(imgData, 'PNG', 0, position, pdfWidth, pdfHeight)
        heightLeft -= pdf.internal.pageSize.getHeight()
        while (heightLeft >= 0) {
          position = heightLeft - pdfHeight
          pdf.addPage()
          pdf.addImage(imgData, 'PNG', 0, position, pdfWidth, pdfHeight)
          heightLeft -= pdf.internal.pageSize.getHeight()
        }
      } else {
        pdf.addImage(imgData, 'PNG', 0, 0, pdfWidth, pdfHeight)
      }
      pdf.save(`rule-${props.rule?.name || 'graph'}.pdf`)
    }

    return {
      triggerNodes, conditionTree, hasConditions,
      effectiveNodes, actionNodes, statusLabel,
      graphViewerRef, exportToImage, exportToPdf
    }
  }
}
</script>

<style>
/* ======================================================================
 *  CSS 变量 — 明亮/暗黑双主题
 * ====================================================================== */
.rule-graph-viewer {
  /* 明亮主题默认值 */
  --rg-bg: #f1f5f9;
  --rg-surface: rgba(255, 255, 255, 0.88);
  --rg-surface-solid: #ffffff;
  --rg-border: rgba(226, 232, 240, 0.85);
  --rg-text-primary: #0f172a;
  --rg-text-secondary: #475569;
  --rg-text-tertiary: #94a3b8;
  --rg-dot-color: #cbd5e1;
  --rg-connector-color: #94a3b8;
  --rg-shadow: rgba(0, 0, 0, 0.04);
  --rg-shadow-hover: rgba(0, 0, 0, 0.08);

  /* 主题色 */
  --rg-trigger: #8b5cf6;
  --rg-trigger-bg: rgba(139, 92, 246, 0.06);
  --rg-trigger-border: rgba(139, 92, 246, 0.2);
  --rg-condition: #f59e0b;
  --rg-condition-bg: rgba(245, 158, 11, 0.06);
  --rg-condition-border: rgba(245, 158, 11, 0.2);
  --rg-time: #06b6d4;
  --rg-time-bg: rgba(6, 182, 212, 0.06);
  --rg-time-border: rgba(6, 182, 212, 0.2);
  --rg-action: #10b981;
  --rg-action-bg: rgba(16, 185, 129, 0.06);
  --rg-action-border: rgba(16, 185, 129, 0.2);

  display: flex;
  flex-direction: column;
  gap: 1.25rem;
  font-family: 'Inter', system-ui, -apple-system, sans-serif;
  position: relative;
}

/* === 暗黑主题 === */
:root[data-bs-theme="dark"] .rule-graph-viewer,
[data-bs-theme="dark"] .rule-graph-viewer {
  --rg-bg: #0f172a;
  --rg-surface: rgba(30, 41, 59, 0.85);
  --rg-surface-solid: #1e293b;
  --rg-border: rgba(51, 65, 85, 0.8);
  --rg-text-primary: #f1f5f9;
  --rg-text-secondary: #94a3b8;
  --rg-text-tertiary: #64748b;
  --rg-dot-color: #334155;
  --rg-connector-color: #475569;
  --rg-shadow: rgba(0, 0, 0, 0.2);
  --rg-shadow-hover: rgba(0, 0, 0, 0.35);

  --rg-trigger: #a78bfa;
  --rg-trigger-bg: rgba(167, 139, 250, 0.08);
  --rg-trigger-border: rgba(167, 139, 250, 0.25);
  --rg-condition: #fbbf24;
  --rg-condition-bg: rgba(251, 191, 36, 0.08);
  --rg-condition-border: rgba(251, 191, 36, 0.25);
  --rg-time: #22d3ee;
  --rg-time-bg: rgba(34, 211, 238, 0.08);
  --rg-time-border: rgba(34, 211, 238, 0.25);
  --rg-action: #34d399;
  --rg-action-bg: rgba(52, 211, 153, 0.08);
  --rg-action-border: rgba(52, 211, 153, 0.25);
}

/* ======================================================================
 *  概览面板
 * ====================================================================== */
.rg-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 0.75rem;
  padding: 1rem 1.25rem;
  background: var(--rg-surface);
  backdrop-filter: blur(12px);
  border: 1px solid var(--rg-border);
  border-radius: 0.875rem;
  box-shadow: 0 4px 16px var(--rg-shadow);
}

.rg-summary__item {
  flex: 1;
  min-width: 180px;
  display: flex;
  align-items: center;
  gap: 0.65rem;
  padding: 0.6rem 0.85rem;
  background: var(--rg-surface-solid);
  border: 1px solid var(--rg-border);
  border-radius: 0.625rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.rg-summary__item:hover {
  border-color: var(--rg-trigger);
  box-shadow: 0 0 0 1px var(--rg-trigger-border);
}

.rg-summary__icon {
  width: 2rem;
  height: 2rem;
  border-radius: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  flex-shrink: 0;
}
.rg-summary__icon--primary { background: rgba(99, 102, 241, 0.1); color: #6366f1; }
.rg-summary__icon--scope   { background: rgba(139, 92, 246, 0.1); color: #8b5cf6; }
.rg-summary__icon--status  { background: rgba(245, 158, 11, 0.1); color: #f59e0b; }
.rg-summary__icon--shield  { background: rgba(16, 185, 129, 0.1); color: #10b981; }

[data-bs-theme="dark"] .rg-summary__icon--primary { background: rgba(99, 102, 241, 0.15); }
[data-bs-theme="dark"] .rg-summary__icon--scope   { background: rgba(139, 92, 246, 0.15); }
[data-bs-theme="dark"] .rg-summary__icon--status  { background: rgba(245, 158, 11, 0.15); }
[data-bs-theme="dark"] .rg-summary__icon--shield  { background: rgba(16, 185, 129, 0.15); }

.rg-summary__text {
  display: flex;
  flex-direction: column;
}
.rg-summary__label {
  font-size: 0.68rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--rg-text-tertiary);
  margin-bottom: 0.1rem;
}
.rg-summary__text strong {
  font-size: 0.88rem;
  color: var(--rg-text-primary);
}
.rg-summary__detail {
  font-size: 0.75rem;
  color: var(--rg-text-secondary);
}

.rg-status {
  font-size: 0.78rem;
  font-weight: 600;
  padding: 0.1rem 0.45rem;
  border-radius: 0.25rem;
  display: inline-block;
}
.rg-status--enabled  { background: rgba(16, 185, 129, 0.12); color: #059669; }
.rg-status--disabled { background: rgba(100, 116, 139, 0.12); color: #475569; }
.rg-status--draft    { background: rgba(59, 130, 246, 0.12); color: #2563eb; }
.rg-status--error    { background: rgba(239, 68, 68, 0.12); color: #dc2626; }

[data-bs-theme="dark"] .rg-status--enabled  { background: rgba(52, 211, 153, 0.15); color: #34d399; }
[data-bs-theme="dark"] .rg-status--disabled { background: rgba(148, 163, 184, 0.15); color: #94a3b8; }
[data-bs-theme="dark"] .rg-status--draft    { background: rgba(96, 165, 250, 0.15); color: #60a5fa; }
[data-bs-theme="dark"] .rg-status--error    { background: rgba(248, 113, 113, 0.15); color: #f87171; }

/* ======================================================================
 *  流程画布
 * ====================================================================== */
.rg-canvas {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 2.5rem 2rem;
  background-color: var(--rg-bg);
  background-image: radial-gradient(var(--rg-dot-color) 1px, transparent 1px);
  background-size: 18px 18px;
  border: 1px solid var(--rg-border);
  border-radius: 1rem;
  overflow: hidden;
}

.rg-canvas__scan {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1.5px;
  background: linear-gradient(90deg, transparent, var(--rg-trigger), transparent);
  animation: rg-scan 5s linear infinite;
  opacity: 0.4;
}
@keyframes rg-scan {
  0%   { transform: translateY(0); opacity: 0; }
  10%  { opacity: 0.5; }
  90%  { opacity: 0.5; }
  100% { transform: translateY(800px); opacity: 0; }
}

/* ======================================================================
 *  Section 头部
 * ====================================================================== */
.rg-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
  max-width: 860px;
  position: relative;
  z-index: 2;
}

.rg-section__header {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.35rem;
  margin-bottom: 1.25rem;
}

.rg-section__pill {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.45rem 1.1rem;
  border-radius: 2rem;
  font-weight: 700;
  font-size: 0.92rem;
  color: white;
  box-shadow: 0 4px 14px var(--rg-shadow);
}
.rg-section--trigger .rg-section__pill  { background: linear-gradient(135deg, #a78bfa, #7c3aed); box-shadow: 0 4px 14px rgba(124, 58, 237, 0.2); }
.rg-section--condition .rg-section__pill { background: linear-gradient(135deg, #fbbf24, #d97706); box-shadow: 0 4px 14px rgba(217, 119, 6, 0.2); }
.rg-section--time .rg-section__pill      { background: linear-gradient(135deg, #22d3ee, #0284c7); box-shadow: 0 4px 14px rgba(2, 132, 199, 0.2); }
.rg-section--action .rg-section__pill    { background: linear-gradient(135deg, #34d399, #059669); box-shadow: 0 4px 14px rgba(5, 150, 105, 0.2); }

.rg-section__pill i {
  font-size: 1rem;
}
.rg-section__kicker {
  font-size: 0.65rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  opacity: 0.8;
}
.rg-section__title {
  font-size: 0.92rem;
}

.rg-section__hint {
  font-size: 0.72rem;
  color: var(--rg-text-tertiary);
  font-style: italic;
}

/* ======================================================================
 *  段间连接器
 * ====================================================================== */
.rg-connector {
  position: relative;
  height: 2.5rem;
  width: 2px;
  margin: 0.25rem 0;
  z-index: 2;
}
.rg-connector__line {
  position: absolute;
  inset: 0;
  width: 100%;
  background: linear-gradient(180deg, var(--rg-connector-color), var(--rg-connector-color));
}
.rg-connector__dot {
  position: absolute;
  left: -3px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--rg-trigger);
  box-shadow: 0 0 6px var(--rg-trigger);
  animation: rg-pulse-down 2.5s infinite linear;
}
@keyframes rg-pulse-down {
  0%   { top: 0; opacity: 0; }
  15%  { opacity: 1; }
  85%  { opacity: 1; }
  100% { top: 100%; opacity: 0; }
}

/* ======================================================================
 *  节点卡片 (RgCard)
 * ====================================================================== */
.rg-card {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.8rem 1.1rem;
  background: var(--rg-surface);
  backdrop-filter: blur(10px);
  border: 1px solid var(--rg-border);
  border-radius: 0.75rem;
  box-shadow: 0 3px 12px var(--rg-shadow);
  transition: all 0.25s cubic-bezier(0.25, 0.8, 0.25, 1);
  position: relative;
  max-width: 480px;
  width: 100%;
}
.rg-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px var(--rg-shadow-hover);
}

/* 左侧色条 */
.rg-card::before {
  content: '';
  position: absolute;
  left: 0;
  top: 12%;
  bottom: 12%;
  width: 3.5px;
  border-radius: 0 3px 3px 0;
  transition: all 0.3s;
}
.rg-card--trigger::before  { background: linear-gradient(to bottom, #a78bfa, #7c3aed); }
.rg-card--condition::before { background: linear-gradient(to bottom, #fbbf24, #d97706); }
.rg-card--time::before     { background: linear-gradient(to bottom, #22d3ee, #0284c7); }
.rg-card--action::before   { background: linear-gradient(to bottom, #34d399, #059669); }

.rg-card:hover::before { top: 5%; bottom: 5%; }

.rg-card--trigger:hover  { border-color: var(--rg-trigger-border); }
.rg-card--condition:hover { border-color: var(--rg-condition-border); }
.rg-card--time:hover     { border-color: var(--rg-time-border); }
.rg-card--action:hover   { border-color: var(--rg-action-border); }

/* 空卡片 */
.rg-card--empty {
  border-style: dashed;
  opacity: 0.6;
  cursor: default;
}
.rg-card--empty::before { display: none; }
.rg-card--empty:hover { transform: none; box-shadow: 0 3px 12px var(--rg-shadow); }

/* 图标 */
.rg-card__icon {
  width: 2.1rem;
  height: 2.1rem;
  border-radius: 0.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  flex-shrink: 0;
  transition: transform 0.25s;
}
.rg-card:hover .rg-card__icon { transform: scale(1.1); }

.rg-card__icon--trigger   { background: var(--rg-trigger-bg); color: var(--rg-trigger); }
.rg-card__icon--condition { background: var(--rg-condition-bg); color: var(--rg-condition); }
.rg-card__icon--time      { background: var(--rg-time-bg); color: var(--rg-time); }
.rg-card__icon--action    { background: var(--rg-action-bg); color: var(--rg-action); }

/* 文本 */
.rg-card__text {
  flex: 1;
  min-width: 0;
}
.rg-card__title {
  font-weight: 600;
  font-size: 0.84rem;
  color: var(--rg-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.rg-card__detail {
  font-size: 0.75rem;
  color: var(--rg-text-secondary);
  margin-top: 0.15rem;
  word-break: break-all;
  line-height: 1.4;
}

/* 徽章 */
.rg-card__badges {
  display: flex;
  gap: 0.25rem;
  flex-shrink: 0;
}
.rg-card__badge {
  font-size: 0.62rem;
  font-weight: 700;
  padding: 0.1rem 0.4rem;
  border-radius: 0.25rem;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}
.rg-card__badge--trigger   { background: var(--rg-trigger-bg); color: var(--rg-trigger); }
.rg-card__badge--condition { background: var(--rg-condition-bg); color: var(--rg-condition); }
.rg-card__badge--time      { background: var(--rg-time-bg); color: var(--rg-time); }
.rg-card__badge--action    { background: var(--rg-action-bg); color: var(--rg-action); }

/* ======================================================================
 *  逻辑菱形标注 (OR / AND)
 * ====================================================================== */
.rg-logic-diamond {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2.2rem;
  height: 2.2rem;
  transform: rotate(45deg);
  border-radius: 0.35rem;
  flex-shrink: 0;
}
.rg-logic-diamond span {
  transform: rotate(-45deg);
  font-size: 0.6rem;
  font-weight: 800;
  letter-spacing: 0.04em;
}
.rg-logic-diamond--or {
  background: rgba(245, 158, 11, 0.12);
  border: 1.5px solid rgba(245, 158, 11, 0.35);
  color: #d97706;
}
.rg-logic-diamond--and {
  background: rgba(59, 130, 246, 0.12);
  border: 1.5px solid rgba(59, 130, 246, 0.35);
  color: #2563eb;
}

[data-bs-theme="dark"] .rg-logic-diamond--or {
  background: rgba(251, 191, 36, 0.12);
  border-color: rgba(251, 191, 36, 0.35);
  color: #fbbf24;
}
[data-bs-theme="dark"] .rg-logic-diamond--and {
  background: rgba(96, 165, 250, 0.12);
  border-color: rgba(96, 165, 250, 0.35);
  color: #60a5fa;
}

/* ======================================================================
 *  触发条件区 — 水平排列 + OR 菱形
 * ====================================================================== */
.rg-triggers {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.75rem;
  width: 100%;
}
.rg-triggers .rg-card {
  flex: 0 1 auto;
  max-width: 380px;
  min-width: 240px;
}

/* ======================================================================
 *  判断条件区 — 递归条件组
 * ====================================================================== */
.rg-conditions {
  width: 100%;
  display: flex;
  justify-content: center;
}

.rg-group-color-0 { --group-color: #f59e0b; --group-bg: rgba(245, 158, 11, 0.06); --group-border: rgba(245, 158, 11, 0.3); }
.rg-group-color-1 { --group-color: #3b82f6; --group-bg: rgba(59, 130, 246, 0.06); --group-border: rgba(59, 130, 246, 0.3); }
.rg-group-color-2 { --group-color: #8b5cf6; --group-bg: rgba(139, 92, 246, 0.06); --group-border: rgba(139, 92, 246, 0.3); }
.rg-group-color-3 { --group-color: #10b981; --group-bg: rgba(16, 185, 129, 0.06); --group-border: rgba(16, 185, 129, 0.3); }
.rg-group-color-4 { --group-color: #f43f5e; --group-bg: rgba(244, 63, 94, 0.06); --group-border: rgba(244, 63, 94, 0.3); }
.rg-group-color-5 { --group-color: #06b6d4; --group-bg: rgba(6, 182, 212, 0.06); --group-border: rgba(6, 182, 212, 0.3); }

[data-bs-theme="dark"] .rg-group-color-0 { --group-color: #fbbf24; --group-bg: rgba(251, 191, 36, 0.1); --group-border: rgba(251, 191, 36, 0.35); }
[data-bs-theme="dark"] .rg-group-color-1 { --group-color: #60a5fa; --group-bg: rgba(96, 165, 250, 0.1); --group-border: rgba(96, 165, 250, 0.35); }
[data-bs-theme="dark"] .rg-group-color-2 { --group-color: #a78bfa; --group-bg: rgba(167, 139, 250, 0.1); --group-border: rgba(167, 139, 250, 0.35); }
[data-bs-theme="dark"] .rg-group-color-3 { --group-color: #34d399; --group-bg: rgba(52, 211, 153, 0.1); --group-border: rgba(52, 211, 153, 0.35); }
[data-bs-theme="dark"] .rg-group-color-4 { --group-color: #fb7185; --group-bg: rgba(251, 113, 133, 0.1); --group-border: rgba(251, 113, 133, 0.35); }
[data-bs-theme="dark"] .rg-group-color-5 { --group-color: #22d3ee; --group-bg: rgba(34, 211, 238, 0.1); --group-border: rgba(34, 211, 238, 0.35); }

.rg-cond-group {
  position: relative;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.25rem 1rem 1rem;
  border: 1.5px dashed var(--group-border, var(--rg-condition-border));
  border-radius: 0.875rem;
  background: var(--group-bg, var(--rg-condition-bg));
  margin: 1rem 0 0.5rem;
}

.rg-cond-group__header {
  position: absolute;
  top: -0.85rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 0.5rem;
  z-index: 5;
}

.rg-logic-pill {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.25rem 0.75rem;
  border-radius: 1rem;
  font-size: 0.72rem;
  font-weight: 700;
  background: var(--group-bg, rgba(245, 158, 11, 0.1));
  border: 1px solid var(--group-border, rgba(245, 158, 11, 0.2));
  color: var(--group-color, #d97706);
}

.rg-logic-pill__label {
  font-weight: 800;
  letter-spacing: 0.04em;
}
.rg-logic-pill__hint {
  font-weight: 500;
  opacity: 0.75;
}

.rg-cond-group__toggle {
  background: none;
  border: 1px solid var(--rg-border);
  border-radius: 50%;
  width: 1.5rem;
  height: 1.5rem;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--rg-text-tertiary);
  transition: all 0.2s;
  font-size: 0.7rem;
}
.rg-cond-group__toggle:hover {
  border-color: var(--rg-condition);
  color: var(--rg-condition);
}

.rg-cond-group__items {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
}

.rg-cond-group__collapsed {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 0.5rem 1rem;
  background: var(--rg-surface);
  border: 1px dashed var(--rg-border);
  border-radius: 0.5rem;
  font-size: 0.78rem;
  color: var(--rg-text-tertiary);
  cursor: pointer;
  transition: all 0.2s;
}
.rg-cond-group__collapsed:hover {
  border-color: var(--rg-condition);
  color: var(--rg-condition);
}

.rg-empty-hint {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 0.75rem 1.25rem;
  background: var(--rg-surface);
  border: 1px dashed var(--rg-border);
  border-radius: 0.75rem;
  font-size: 0.82rem;
  color: var(--rg-text-tertiary);
  font-style: italic;
}

/* ======================================================================
 *  生效时间区
 * ====================================================================== */
.rg-time-cards {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.75rem;
  width: 100%;
}
.rg-time-cards .rg-card {
  max-width: 340px;
}

/* ======================================================================
 *  执行动作区
 * ====================================================================== */
.rg-actions {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  width: 100%;
}
.rg-actions > .rg-card {
  max-width: 480px;
}

/* 串行步骤连线 */
.rg-step-connector {
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 2rem;
  position: relative;
}
.rg-step-connector__line {
  width: 2px;
  height: 100%;
  background: var(--rg-connector-color);
}
.rg-step-connector__arrow {
  font-size: 0.65rem;
  color: var(--rg-action);
  margin-top: -0.1rem;
  line-height: 1;
}

/* ======================================================================
 *  延迟节点
 * ====================================================================== */
.rg-delay-node {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.5rem;
  padding: 0.5rem 1.25rem;
  background: var(--rg-surface);
  border: 1.5px dashed var(--rg-connector-color);
  border-radius: 2rem;
  margin: 0.25rem 0;
}
.rg-delay-node__icon {
  font-size: 0.9rem;
  color: var(--rg-text-tertiary);
  animation: rg-hourglass 2s ease-in-out infinite;
}
@keyframes rg-hourglass {
  0%, 100% { transform: rotate(0deg); }
  50%      { transform: rotate(180deg); }
}
.rg-delay-node__text {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--rg-text-secondary);
}

/* ======================================================================
 *  并行组 — 铁路图风格
 * ====================================================================== */
.rg-parallel-group {
  position: relative;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.25rem;
  border: 1.5px dashed var(--group-border, rgba(99, 102, 241, 0.3));
  border-radius: 1rem;
  background: var(--group-bg, rgba(99, 102, 241, 0.02));
  margin: 0.25rem 0;
}
[data-bs-theme="dark"] .rg-parallel-group {
  border-color: var(--group-border, rgba(129, 140, 248, 0.3));
  background: var(--group-bg, rgba(129, 140, 248, 0.04));
}

.rg-parallel-group__label {
  position: absolute;
  top: -0.6rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.15rem 0.65rem;
  border-radius: 1rem;
  font-size: 0.7rem;
  font-weight: 700;
  background: var(--group-bg, rgba(99, 102, 241, 0.12));
  color: var(--group-color, #6366f1);
  border: 1px solid var(--group-border, rgba(99, 102, 241, 0.2));
  z-index: 5;
}
[data-bs-theme="dark"] .rg-parallel-group__label {
  background: var(--group-bg, rgba(129, 140, 248, 0.15));
  color: var(--group-color, #818cf8);
  border-color: var(--group-border, rgba(129, 140, 248, 0.25));
}
.rg-parallel-group__count {
  font-weight: 500;
  opacity: 0.7;
}

/* 并行分叉/汇聚 */
.rg-parallel-fork,
.rg-parallel-join {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}
.rg-parallel-fork__stem,
.rg-parallel-join__stem {
  width: 2px;
  height: 12px;
  background: var(--rg-connector-color);
}
.rg-parallel-fork__rail,
.rg-parallel-join__rail {
  width: 80%;
  max-width: 600px;
  height: 2px;
  background: var(--rg-connector-color);
}

.rg-parallel-branches {
  display: flex;
  justify-content: center;
  gap: 0.75rem;
  width: 100%;
  flex-wrap: wrap;
  padding: 0.25rem 0;
}

.rg-parallel-branch {
  flex: 1;
  min-width: 200px;
  max-width: 320px;
  display: flex;
  flex-direction: column;
  align-items: center;
}
.rg-parallel-branch__line {
  width: 2px;
  height: 14px;
  background: var(--rg-connector-color);
}
.rg-parallel-branch .rg-card {
  max-width: 100%;
}

/* ======================================================================
 *  串行组
 * ====================================================================== */
.rg-serial-group {
  position: relative;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 1.25rem;
  border: 1.5px dashed var(--group-border, var(--rg-action-border));
  border-radius: 1rem;
  background: var(--group-bg, var(--rg-action-bg));
  margin: 0.25rem 0;
}

.rg-serial-group__label {
  position: absolute;
  top: -0.6rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.15rem 0.65rem;
  border-radius: 1rem;
  font-size: 0.7rem;
  font-weight: 700;
  background: var(--group-bg, var(--rg-action-bg));
  color: var(--group-color, var(--rg-action));
  border: 1px solid var(--group-border, var(--rg-action-border));
  z-index: 5;
}
.rg-serial-group__count {
  font-weight: 500;
  opacity: 0.7;
}

.rg-serial-steps {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0;
  width: 100%;
}

.rg-serial-arrow {
  position: relative;
  height: 1.5rem;
  width: 2px;
  margin: 0.15rem 0;
}
.rg-serial-arrow__line {
  position: absolute;
  inset: 0;
  background: var(--rg-connector-color);
}
.rg-serial-arrow__pulse {
  position: absolute;
  left: -2.5px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--rg-action);
  box-shadow: 0 0 6px var(--rg-action);
  animation: rg-pulse-down 1.8s infinite linear;
}

/* ======================================================================
 *  响应式
 * ====================================================================== */
@media (max-width: 768px) {
  .rg-canvas {
    padding: 1.5rem 1rem;
  }
  .rg-triggers {
    flex-direction: column;
  }
  .rg-triggers .rg-card {
    max-width: 100%;
  }
  .rg-parallel-branches {
    flex-direction: column;
    align-items: center;
  }
  .rg-parallel-branch {
    max-width: 100%;
  }
  .rg-summary {
    flex-direction: column;
  }
  .rg-summary__item {
    min-width: auto;
  }
}
/* ======================================================================
 *  新增：生效时间独立板块及导出工具栏
 * ====================================================================== */
.rg-toolbar {
  position: absolute;
  top: -45px;
  right: 0;
  z-index: 10;
}

.rg-effective-time-panel {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 1rem;
  padding: 0.75rem 1.25rem;
  background: var(--rg-surface);
  border: 1px solid var(--rg-border);
  border-radius: 12px;
  margin: 1rem auto;
  width: 100%;
  max-width: 800px;
  box-shadow: 0 4px 15px var(--rg-shadow);
}
.rg-effective-time-panel__header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 600;
  color: #0284c7;
  white-space: nowrap;
}
[data-bs-theme="dark"] .rg-effective-time-panel__header {
  color: #38bdf8;
}
.rg-effective-time-panel__body {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.rg-effective-time-panel__body .rg-card {
  margin: 0;
  padding: 0.5rem 1rem;
  min-width: unset;
  max-width: unset;
}
</style>
