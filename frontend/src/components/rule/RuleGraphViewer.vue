<template>
  <div class="rule-graph-viewer" :class="{ 'rg-editing': isEditing }" v-if="rule || editableRule" ref="graphViewerRef">
    <!-- 顶部工具栏 -->
    <div class="rg-toolbar text-end mb-3">
      <button v-if="!isEditing" class="btn btn-sm btn-primary me-2" @click="startEditing">
        <i class="bi bi-pencil"></i> {{ $t('rule_edit', '编辑规则') }}
      </button>
      <button v-if="isEditing" class="btn btn-sm btn-secondary me-2" @click="cancelEditing">
        {{ $t('tsl_cancel', '取消') }}
      </button>
      <button v-if="isEditing" class="btn btn-sm btn-success me-2" :disabled="savingEditing" @click="saveEditing">
        <span v-if="savingEditing" class="spinner-border spinner-border-sm me-1"></span>
        <i class="bi bi-save"></i> {{ $t('save', '保存') }}
      </button>
      <button v-if="!isEditing" class="btn btn-sm btn-outline-info me-2" @click="exportToImage">
        <i class="bi bi-image"></i> {{ $t('export_image', '导出图片') }}
      </button>
      <button class="btn btn-sm btn-outline-danger" @click="exportToPdf">
        <i class="bi bi-file-pdf"></i> {{ $t('export_pdf', '导出PDF') }}
      </button>
    </div>
    <div v-if="saveMessage" class="alert py-2" :class="saveMessageType === 'success' ? 'alert-success' : 'alert-danger'">
      {{ saveMessage }}
    </div>

    <!-- 规则概览面板 (非编辑时显示) -->
    <div class="rg-summary" v-if="!isEditing">
      <div class="rg-summary__item">
        <div class="rg-summary__icon rg-summary__icon--primary"><i class="bi bi-diagram-3"></i></div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('rule_name') }}</span>
          <strong>{{ rule.name }}</strong>
        </div>
      </div>
      <div class="rg-summary__item">
        <div class="rg-summary__icon rg-summary__icon--scope"><i class="bi bi-hdd-rack"></i></div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('rule_scope') }}</span>
          <strong>{{ rule.scope === 'gateway' ? $t('rule_scope_gateway') : $t('rule_scope_platform') }}</strong>
        </div>
      </div>
      <div class="rg-summary__item">
        <div class="rg-summary__icon rg-summary__icon--status"><i class="bi bi-activity"></i></div>
        <div class="rg-summary__text">
          <span class="rg-summary__label">{{ $t('status') }}</span>
          <span class="rg-status" :class="`rg-status--${rule.status}`">{{ statusLabel(rule.status) }}</span>
        </div>
      </div>
    </div>

    <div class="rg-workspace d-flex align-items-stretch">
      <!-- 左侧组件库 -->
      <div v-if="isEditing" class="rg-palette border-end p-3" style="width: 250px; background: var(--rg-surface); overflow-y: auto; position: sticky; top: 1rem; max-height: calc(100vh - 4rem); align-self: flex-start;">
        <h6 class="mb-3"><i class="bi bi-puzzle"></i> {{ $t('rule_components', '组件库') }}</h6>
        <div class="rg-palette-section mb-3">
          <div class="text-muted small mb-2">{{ $t('rule_triggers', '触发条件') }}</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'property_change' }, 'trigger')"><i class="bi bi-graph-up-arrow text-primary"></i> 属性变更触发</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'event' }, 'trigger')"><i class="bi bi-broadcast-pin text-primary"></i> 事件触发</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'device_status' }, 'trigger')"><i class="bi bi-power text-primary"></i> 状态触发</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'cron', cronMode: 'visual' }, 'trigger')"><i class="bi bi-clock-history text-primary"></i> 定时触发</div>
        </div>
        <div class="rg-palette-section mb-3">
          <div class="text-muted small mb-2">{{ $t('rule_conditions', '判断条件') }}</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'condition_leaf', detailType: 'property' }, 'condition')"><i class="bi bi-wrench-adjustable text-warning"></i> 属性判断</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'condition_leaf', detailType: 'device_status' }, 'condition')"><i class="bi bi-toggle-on text-warning"></i> 状态判断</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'condition_group_and', logic: 'and' }, 'condition_group')"><i class="bi bi-intersect text-warning"></i> 满足所有(AND)</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'condition_group_or', logic: 'or' }, 'condition_group')"><i class="bi bi-union text-warning"></i> 满足任一(OR)</div>
        </div>
        <div class="rg-palette-section rg-palette-section--ai mb-3">
          <div class="text-muted small mb-2">{{ $t('rule_ai_capability', 'AI 能力') }}</div>
          <div class="rg-palette-item rg-palette-item--ai border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'llm' }, 'action')"><i class="bi bi-stars text-info"></i> {{ $t('rule_action_ai_reasoning', 'AI 推理') }}</div>
        </div>
        <div class="rg-palette-section mb-3">
          <div class="text-muted small mb-2">{{ $t('rule_actions', '执行动作') }}</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'set_property' }, 'action')"><i class="bi bi-pencil-square text-success"></i> 设置属性</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'call_service' }, 'action')"><i class="bi bi-gear-wide-connected text-success"></i> 调用服务</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'notification' }, 'action')"><i class="bi bi-chat-left-dots text-success"></i> 消息通知</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'alarm' }, 'action')"><i class="bi bi-exclamation-triangle-fill text-success"></i> 触发告警</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'delay', delaySec: 1 }, 'action')"><i class="bi bi-hourglass-split text-success"></i> 延迟执行</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'text' }, 'action')"><i class="bi bi-text-paragraph text-success"></i> {{ $t('rule_action_text', '文本组件') }}</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'voice_playback' }, 'action')"><i class="bi bi-volume-up-fill text-success"></i> 语音播放</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'action_group', mode: 'parallel' }, 'action_group')"><i class="bi bi-cpu text-success"></i> 并行执行组</div>
          <div class="rg-palette-item border rounded p-2 mb-2 cursor-pointer bg-white shadow-sm" draggable="true" @dragstart="onDragStart($event, { type: 'action_group', mode: 'sequence' }, 'action_group')"><i class="bi bi-list-ol text-success"></i> 串行执行组</div>
        </div>
      </div>
      <!-- 流程图画布 -->
      <div class="rg-canvas flex-grow-1" @click="selectedNode = null">
        <div class="rg-canvas__scan"></div>

        <!-- === 生效时间 === -->
        <section class="rg-section rg-section--time" v-if="effectiveNodes.length">
          <div class="rg-section__header cursor-pointer" @click.stop="toggleSection('time')">
            <span class="rg-section__pill">
              <i class="bi bi-calendar-check-fill"></i>
              <span class="rg-section__kicker">{{ $t('rule_schedule', '调度') }}</span>
              <span class="rg-section__title">{{ $t('rule_effective_time') }} <i :class="['bi', collapsed.time ? 'bi-chevron-down' : 'bi-chevron-up']"></i></span>
            </span>
          </div>
          <div class="rg-time-cards" v-show="!collapsed.time">
            <RgCard v-for="node in effectiveNodes" :key="node.id" :node="node" />
          </div>
        </section>

        <!-- 连接线 -->
        <div class="rg-connector" v-if="effectiveNodes.length && !collapsed.time">
          <div class="rg-connector__line"></div>
          <div class="rg-connector__dot"></div>
        </div>

        <!-- === WHEN 触发条件 === -->
        <section class="rg-section rg-section--trigger">
          <div class="rg-section__header cursor-pointer" @click.stop="toggleSection('trigger')">
            <span class="rg-section__pill">
              <i class="bi bi-lightning-charge-fill"></i>
              <span class="rg-section__kicker">{{ $t('rule_when') }}</span>
              <span class="rg-section__title">{{ $t('rule_triggers') }} <i :class="['bi', collapsed.trigger ? 'bi-chevron-down' : 'bi-chevron-up']"></i></span>
            </span>
            <span class="rg-section__hint" v-show="!collapsed.trigger">{{ $t('rule_graph_trigger_or') }}</span>
          </div>

          <div class="rg-triggers" v-show="!collapsed.trigger" @dragover.prevent @drop="onDropTrigger">
            <template v-for="(node, i) in triggerNodes" :key="node.id">
              <RgCard :node="node" />
            </template>
          </div>
        </section>

        <div class="rg-connector" v-show="!collapsed.trigger && !collapsed.condition">
          <div class="rg-connector__line"></div>
          <div class="rg-connector__dot"></div>
        </div>

        <!-- === IF 判断条件 === -->
        <section class="rg-section rg-section--condition">
          <div class="rg-section__header cursor-pointer" @click.stop="toggleSection('condition')">
            <span class="rg-section__pill">
              <i class="bi bi-funnel-fill"></i>
              <span class="rg-section__kicker">{{ $t('rule_if') }}</span>
              <span class="rg-section__title">{{ $t('rule_conditions') }} <i :class="['bi', collapsed.condition ? 'bi-chevron-down' : 'bi-chevron-up']"></i></span>
            </span>
            <span class="rg-section__hint" v-if="!hasConditions && !collapsed.condition">{{ $t('rule_graph_condition_skip') }}</span>
          </div>

          <div v-show="!collapsed.condition" class="w-100 d-flex justify-content-center">
            <div class="rg-conditions" v-if="hasConditions" @dragover.prevent @drop.stop="onDropConditionRoot">
              <RgConditionGroup :group="conditionTree" :devices="devices" :depth="0" />
            </div>
            <div v-else class="rg-empty-hint" @dragover.prevent @drop.stop="onDropConditionRoot">
              <i class="bi bi-skip-forward-circle"></i>
              {{ $t('rule_no_conditions') }}
            </div>
          </div>
        </section>

        <div class="rg-connector" v-show="!collapsed.condition && !collapsed.action">
          <div class="rg-connector__line"></div>
          <div class="rg-connector__dot"></div>
        </div>

        <!-- === THEN 执行动作 === -->
        <section class="rg-section rg-section--action">
          <div class="rg-section__header cursor-pointer" @click.stop="toggleSection('action')">
            <span class="rg-section__pill">
              <i class="bi bi-play-circle-fill"></i>
              <span class="rg-section__kicker">{{ $t('rule_then') }}</span>
              <span class="rg-section__title">{{ $t('rule_actions') }} <i :class="['bi', collapsed.action ? 'bi-chevron-down' : 'bi-chevron-up']"></i></span>
            </span>
          </div>

          <div class="rg-actions" v-show="!collapsed.action" @dragover.prevent @drop.stop="onDropActionRoot">
            <template v-for="(node, i) in actionNodes" :key="node.id">
              <RgActionNode :node="node" :depth="0" />

              <div v-if="i < actionNodes.length - 1 && !node.isDelay" class="rg-step-connector">
                <div class="rg-step-connector__line"></div>
                <div class="rg-step-connector__arrow">▼</div>
              </div>
            </template>
          </div>
        </section>
      </div>
      
      <!-- 右侧属性面板 -->
      <div v-if="isEditing" class="rg-properties border-start p-3" style="width: 320px; background: var(--rg-surface); overflow-y: auto; position: sticky; top: 1rem; max-height: calc(100vh - 4rem); align-self: flex-start;" @click.stop>
        <!-- 基础信息面板 (无选中节点时) -->
        <div v-if="!selectedNode">
          <h6 class="mb-3"><i class="bi bi-info-circle"></i> {{ $t('rule_basic_info', '规则基础信息') }}</h6>
          <div class="mb-3">
            <label class="form-label">规则名称</label>
            <VarInputWrapper v-model="editableRule.name" />
          </div>
          <div class="mb-3">
            <label class="form-label">描述</label>
            <VarInputWrapper :textarea="true" v-model="editableRule.description" :rows="4" :maxRows="10" />
          </div>
          <div class="mb-3">
            <label class="form-label">{{ $t('rule_group') }}</label>
            <select class="form-select form-select-sm" v-model="editableRule.group_id">
              <option :value="null">{{ $t('rule_no_group') }}</option>
              <option v-for="group in groups" :key="group.id || group.ID" :value="group.id || group.ID">{{ group.name }}</option>
            </select>
          </div>
          <div class="mb-3">
            <label class="form-label">{{ $t('rule_priority') }}</label>
            <input type="number" min="1" max="100" class="form-control form-control-sm" v-model.number="editableRule.priority">
          </div>
          <div class="mb-3">
            <label class="form-label">{{ $t('rule_throttle_sec') }}</label>
            <input type="number" min="1" class="form-control form-control-sm" v-model.number="editableRule.throttle_sec">
          </div>
          <div class="mb-3">
            <label class="form-label">{{ $t('rule_max_per_hour') }}</label>
            <input type="number" min="1" class="form-control form-control-sm" v-model.number="editableRule.max_per_hour">
          </div>
          <div class="mb-3">
            <label class="form-label">{{ $t('rule_retry_count') }}</label>
            <input type="number" min="0" max="3" class="form-control form-control-sm" v-model.number="editableRule.retry_count">
          </div>
        </div>

        <!-- 节点属性面板 -->
        <div v-else class="rg-properties-form">
          <div class="d-flex align-items-center justify-content-between mb-3">
            <h6 class="mb-0"><i class="bi bi-sliders"></i> {{ $t('properties', '属性配置') }}</h6>
            <button v-if="canDeleteSelectedNode" type="button" class="btn btn-sm btn-outline-danger" @click="deleteSelectedNode">
              <i class="bi bi-trash"></i>
            </button>
          </div>
          <div class="mb-3" v-if="selectedNode._graphKind === 'condition_group'">
            <label class="form-label">{{ $t('rule_logic', '逻辑') }}</label>
            <select class="form-select form-select-sm" v-model="selectedNode.logic">
              <option value="and">{{ $t('rule_graph_logic_and', '满足所有(AND)') }}</option>
              <option value="or">{{ $t('rule_graph_logic_or', '满足任一(OR)') }}</option>
            </select>
          </div>
          <div class="mb-3" v-else-if="selectedNode.type !== 'effective_time'">
            <label class="form-label">{{ $t('rule_type', '类型') }}</label>
            <select class="form-select form-select-sm" :value="selectedNode.type" @change="changeSelectedNodeType($event.target.value)">
              <option v-for="option in selectedNodeTypeOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
            </select>
          </div>

          <!-- 生效时间编辑 (全局) -->
          <template v-if="selectedNode.type === 'effective_time'">
            <div class="mb-3">
              <label class="form-label">生效模式</label>
              <select class="form-select form-select-sm" v-model="selectedNode.mode">
                <option value="always">始终生效</option>
                <option value="daily">每天 (daily)</option>
                <option value="weekly">每周 (weekly)</option>
                <option value="monthly">每月 (monthly)</option>
                <option value="workday">工作日 (workday)</option>
                <option value="holiday">节假日 (holiday)</option>
                <option value="custom">自定义 (custom)</option>
              </select>
            </div>
            <div class="mb-3" v-if="selectedNode.mode !== 'always'">
              <label class="form-label">开始时间</label>
              <input type="text" inputmode="numeric" class="form-control form-control-sm" placeholder="00:00" v-model="computedStartTime">
            </div>
            <div class="mb-3" v-if="selectedNode.mode !== 'always'">
              <label class="form-label">结束时间</label>
              <input type="text" inputmode="numeric" class="form-control form-control-sm" placeholder="24:00" v-model="computedEndTime">
            </div>
            <div class="mb-3" v-if="['weekly', 'custom'].includes(selectedNode.mode)">
              <label class="form-label">星期配置 (逗号分隔1-7)</label>
              <VarInputWrapper v-model="computedEffectiveWeekdays" placeholder="例如: 1,2,3,4,5" />
            </div>
            <div class="mb-3" v-if="['monthly', 'custom'].includes(selectedNode.mode)">
              <label class="form-label">月份日期 (逗号分隔1-31)</label>
              <VarInputWrapper v-model="computedEffectiveMonthDays" placeholder="例如: 1,15,30" />
            </div>
            <div class="mb-3" v-if="['custom'].includes(selectedNode.mode)">
              <label class="form-label">月份 (逗号分隔1-12)</label>
              <VarInputWrapper v-model="computedEffectiveMonths" placeholder="例如: 1,6,12" />
            </div>
            <div class="mb-3" v-if="selectedNode.mode !== 'always'">
              <label class="form-label">时区</label>
              <VarInputWrapper v-model="selectedNode.timezone" placeholder="Asia/Shanghai" />
            </div>
          </template>

          <!-- 针对具有 deviceCode 的节点 -->
          <div class="mb-3" v-if="['property_change', 'event', 'device_status', 'property', 'set_property', 'call_service'].includes(selectedNode.type)">
            <label class="form-label">设备</label>
            <select class="form-select form-select-sm" v-model="selectedNode.deviceCode">
              <option value="">请选择设备</option>
              <option v-for="dev in devices" :key="dev.code" :value="dev.code">{{ dev.name || dev.code }}</option>
            </select>
          </div>

          <!-- 针对 propertyKey -->
          <div class="mb-3" v-if="['property_change', 'property', 'set_property'].includes(selectedNode.type)">
            <label class="form-label">属性名称</label>
            <select class="form-select form-select-sm" v-model="selectedNode.propertyKey" :disabled="!selectedNode.deviceCode">
              <option value="">请选择属性</option>
              <option v-for="p in getDeviceProperties(selectedNode.deviceCode)" :key="p.key || p.identifier" :value="p.key || p.identifier">
                {{ p.name || p.key || p.identifier }}
              </option>
            </select>
            <label class="form-label mt-2">属性标识</label>
            <input class="form-control form-control-sm" :value="selectedNode.propertyKey || ''" readonly placeholder="选择属性后自动带出">
          </div>
          
          <!-- 针对 eventId -->
          <div class="mb-3" v-if="['event'].includes(selectedNode.type)">
            <label class="form-label">事件名称</label>
            <select class="form-select form-select-sm" v-model="selectedNode.eventId" :disabled="!selectedNode.deviceCode">
              <option value="">请选择事件</option>
              <option v-for="e in getDeviceEvents(selectedNode.deviceCode)" :key="e.key || e.identifier" :value="e.key || e.identifier">
                {{ e.name || e.key || e.identifier }}
              </option>
            </select>
            <label class="form-label mt-2">事件标识</label>
            <input class="form-control form-control-sm" :value="selectedNode.eventId || ''" readonly placeholder="选择事件后自动带出">
          </div>

          <!-- Operator (条件/触发器) -->
          <div class="mb-3" v-if="['property_change', 'property'].includes(selectedNode.type)">
            <label class="form-label">操作符</label>
            <select class="form-select form-select-sm" v-model="selectedNode.operator">
              <option value="eq">=</option>
              <option value="neq">≠</option>
              <option value="gt">&gt;</option>
              <option value="gte">&ge;</option>
              <option value="lt">&lt;</option>
              <option value="lte">&le;</option>
              <option value="changed">改变时</option>
              <option value="contains">包含</option>
            </select>
          </div>

          <!-- 针对 value -->
          <div class="mb-3" v-if="['property_change', 'property', 'set_property'].includes(selectedNode.type)">
            <label class="form-label">值</label>
            <VarInputWrapper v-model="selectedNode.value" />
          </div>
          
          <!-- 针对 statusValue -->
          <div class="mb-3" v-if="['device_status'].includes(selectedNode.type)">
            <label class="form-label">状态</label>
            <select class="form-select form-select-sm" v-model="selectedNode.statusValue">
              <option value="online">在线</option>
              <option value="offline">离线</option>
            </select>
          </div>
          
          <!-- 定时触发 可视化配置 -->
          <template v-if="selectedNode.type === 'cron'">
            <div class="mb-3">
              <label class="form-label">配置方式</label>
              <select class="form-select form-select-sm" v-model="selectedNode.cronMode" @change="syncSelectedCron">
                <option value="visual">常用配置</option>
                <option value="fields">Cron 字段</option>
                <option value="advanced">高级表达式</option>
              </select>
            </div>
            <template v-if="selectedNode.cronMode !== 'advanced'">
              <div class="mb-3" v-if="selectedNode.cronMode === 'visual'">
                <label class="form-label">定时类型</label>
                <select class="form-select form-select-sm" v-model="selectedNode.schedule.mode" @change="syncSelectedCron">
                  <option value="every_minutes">每隔 N 分钟</option>
                  <option value="hourly">每小时</option>
                  <option value="daily">每天</option>
                  <option value="weekly">每周</option>
                  <option value="monthly">每月</option>
                </select>
              </div>
              <div class="mb-3" v-if="selectedNode.cronMode === 'visual' && selectedNode.schedule.mode === 'every_minutes'">
                <label class="form-label">间隔分钟</label>
                <input class="form-control form-control-sm" type="number" min="1" max="59" v-model.number="selectedNode.schedule.intervalMinutes" @input="syncSelectedCron">
              </div>
              <div class="row g-2" v-if="selectedNode.cronMode === 'visual' && selectedNode.schedule.mode !== 'every_minutes'">
                <div class="col-6">
                  <label class="form-label">时</label>
                  <input class="form-control form-control-sm" type="number" min="0" max="23" v-model.number="selectedNode.schedule.hour" @input="syncSelectedCron">
                </div>
                <div class="col-6">
                  <label class="form-label">分</label>
                  <input class="form-control form-control-sm" type="number" min="0" max="59" v-model.number="selectedNode.schedule.minute" @input="syncSelectedCron">
                </div>
              </div>
              <div class="mb-3 mt-2" v-if="selectedNode.cronMode === 'visual' && selectedNode.schedule.mode === 'weekly'">
                <label class="form-label">星期</label>
                <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.weekdaysText" placeholder="1,3,5" @input="syncSelectedCron">
              </div>
              <div class="mb-3 mt-2" v-if="selectedNode.cronMode === 'visual' && selectedNode.schedule.mode === 'monthly'">
                <label class="form-label">月份日期</label>
                <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.monthDaysText" placeholder="1,15,28" @input="syncSelectedCron">
              </div>
              <div class="row g-2" v-if="selectedNode.cronMode === 'fields'">
                <div class="col-6">
                  <label class="form-label">分</label>
                  <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.minuteExpr" placeholder="*/5" @input="syncSelectedCron">
                </div>
                <div class="col-6">
                  <label class="form-label">时</label>
                  <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.hourExpr" placeholder="*" @input="syncSelectedCron">
                </div>
                <div class="col-6">
                  <label class="form-label">日</label>
                  <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.dayOfMonthExpr" placeholder="*" @input="syncSelectedCron">
                </div>
                <div class="col-6">
                  <label class="form-label">月</label>
                  <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.monthExpr" placeholder="*" @input="syncSelectedCron">
                </div>
                <div class="col-12">
                  <label class="form-label">周</label>
                  <input class="form-control form-control-sm" v-model.trim="selectedNode.schedule.weekdayExpr" placeholder="*" @input="syncSelectedCron">
                </div>
              </div>
            </template>
            <div class="mb-3 mt-2">
              <label class="form-label">Cron 表达式</label>
              <input class="form-control form-control-sm" v-model.trim="selectedNode.cronExpr" :readonly="selectedNode.cronMode !== 'advanced'" placeholder="*/5 * * * *">
            </div>
            <div class="mb-3">
              <label class="form-label">任务描述</label>
              <VarInputWrapper v-model="selectedNode.cronDesc" placeholder="例如：每天8点执行" />
            </div>
          </template>

          <!-- 服务调用 -->
          <template v-if="selectedNode.type === 'call_service'">
            <div class="mb-3">
              <label class="form-label">服务</label>
              <select class="form-select form-select-sm" v-model="selectedNode.serviceCode" v-if="selectedNode.deviceCode">
                <option value="">请选择服务</option>
                <option v-for="s in getDeviceServices(selectedNode.deviceCode)" :key="s.key || s.identifier" :value="s.key || s.identifier">
                  {{ s.name || s.key || s.identifier }}
                </option>
              </select>
              <VarInputWrapper v-model="selectedNode.serviceCode" placeholder="输入服务标识" />
            </div>
            <div class="mb-3">
              <label class="form-label">服务参数 (JSON格式)</label>
              <VarInputWrapper :textarea="true" v-model="computedServiceParams" :rows="5" :maxRows="14" placeholder='{"param1": "value"}' />
            </div>
          </template>

          <!-- 消息通知 -->
          <template v-if="selectedNode.type === 'notification'">
            <div class="mb-3">
              <label class="form-label">通知标题</label>
              <VarInputWrapper v-model="selectedNode.notifyTitle" />
            </div>
            <div class="mb-3">
              <label class="form-label">通知内容</label>
              <VarInputWrapper :textarea="true" v-model="selectedNode.notifyContent" :rows="5" :maxRows="12" />
            </div>
          </template>

          <!-- 触发告警 -->
          <template v-if="selectedNode.type === 'alarm'">
            <div class="mb-3">
              <label class="form-label">告警设备</label>
              <select class="form-select form-select-sm" v-model="selectedNode.alarmDevice">
                <option value="">全局或请选择设备</option>
                <option v-for="dev in devices" :key="dev.code" :value="dev.code">{{ dev.name || dev.code }}</option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-label">告警级别</label>
              <select class="form-select form-select-sm" v-model="selectedNode.alarmLevel">
                <option value="info">提示</option>
                <option value="warning">警告</option>
                <option value="critical">严重</option>
                <option value="danger">危险</option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-label">告警名称</label>
              <VarInputWrapper v-model="selectedNode.alarmTitle" />
            </div>
            <div class="mb-3">
              <label class="form-label">告警内容</label>
              <VarInputWrapper :textarea="true" v-model="selectedNode.alarmContent" :rows="5" :maxRows="12" />
            </div>
          </template>

          <!-- 延迟 -->
          <div class="mb-3" v-if="selectedNode.type === 'delay'">
            <label class="form-label">延迟(秒)</label>
            <input type="number" class="form-control form-control-sm" v-model.number="selectedNode.delaySec" min="0" max="300">
          </div>

          <!-- 文本 -->
          <template v-if="selectedNode.type === 'text'">
            <div class="mb-3">
              <label class="form-label">{{ $t('rule_action_text_content', '文本内容') }}</label>
              <VarInputWrapper :textarea="true" v-model="selectedNode.textContent" :rows="5" :maxRows="12" />
            </div>
          </template>

          <!-- LLM -->
          <template v-if="selectedNode.type === 'llm'">
            <div class="mb-3">
              <label class="form-label">附加描述词</label>
              <VarInputWrapper :textarea="true" v-model="selectedNode.llmPrompt" :rows="7" :maxRows="18" />
            </div>
          </template>

          <!-- 语音播放 -->
          <template v-if="selectedNode.type === 'voice_playback'">
            <div class="mb-3">
              <label class="form-label">{{ $t('rule_action_voice_text', '播放文本') }}</label>
              <VarInputWrapper :textarea="true" v-model="selectedNode.voiceText" :rows="5" :maxRows="12" />
            </div>
          </template>

        </div>
      </div>
    </div>
  </div>
</template>

<script>
import VarInputWrapper from './VarInputWrapper.vue'
import { defineComponent, computed, h, ref, reactive, provide, inject, watch, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import html2canvas from 'html2canvas'
import { jsPDF } from 'jspdf'
import { insertActionAt, moveActionToList } from '../../utils/ruleGraphDnd.js'

/* ============================================
 *  递归卡片节点组件
 * ============================================ */
const RgCard = defineComponent({
  name: 'RgCard',
  props: {
    node: { type: Object, required: true }
  },
  setup(props) {
    const ctxData = inject('rgContext', {})
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

      const graphNodeId = n._ref?._id || n._ref?.id || n.id || n._id
      const isSel = ctxData.selectedNode?.value && ctxData.selectedNode.value._id === graphNodeId
      const isReferenced = ctxData.referenceNodeId?.value && ctxData.referenceNodeId.value === graphNodeId
      
      const children = [iconEl, textEl, badgesEl]
      
      if (ctxData.isEditing?.value && !n.empty) {
        children.push(
          h('button', {
            class: 'rg-card__delete btn btn-sm btn-danger rounded-circle shadow-sm',
            type: 'button',
            draggable: false,
            onMousedown: ctxData.stopDeletePointerEvent,
            onClick: (e) => {
              e.stopPropagation()
              ctxData.deleteNode(n._ref || n)
            }
          }, [ h('i', { class: 'bi bi-x' }) ])
        )
      }

      return h('div', {
        class: [
          'rg-card',
          `rg-card--${n.tone}`,
          { 'rg-card--empty': n.empty },
          { 'rg-node-selected': isSel && !n.empty },
          { 'rg-node-referenced': isReferenced && !n.empty }
        ],
        'data-rg-node-id': graphNodeId || undefined,
        onClick: (e) => {
          if (ctxData.isEditing?.value && !n.empty) {
            e.stopPropagation()
            ctxData.selectNode(n._ref || n, ctxData.dragGroupForNode?.(n) || n.tone)
          }
        },
        draggable: ctxData.isEditing?.value && !n.empty && !!ctxData.dragGroupForNode?.(n),
        onDragstart: (e) => {
          const group = ctxData.dragGroupForNode?.(n)
          if (ctxData.isEditing?.value && group && n._ref) {
            e.dataTransfer.setData('text/plain', JSON.stringify({ item: n._ref, group, isExisting: true, sourceId: n._ref._id || n._ref.id }))
            e.stopPropagation()
          }
        }
      }, children)
    }
  }
})

/* ============================================
 *  递归条件组组件
 * ============================================ */
const RgConditionGroup = defineComponent({
  name: 'RgConditionGroup',
  props: {
    group: { type: Object, required: true },
    devices: { type: Array, default: () => [] },
    depth: { type: Number, default: 0 }
  },
  setup(props) {
    const { t } = useI18n()
    const ctxData = inject('rgContext', {})
    const collapsed = ref(false)

    function findDevice(code) { return props.devices.find(d => d.code === code) }
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

      leafConditions.forEach((c, i) => {
        let icon = 'bi-wrench-adjustable'
        let title = t('rule_condition_property')
        let detail = ''
        if (c.type === 'device_status') {
          icon = 'bi-toggle-on'
          title = t('rule_condition_status')
          detail = `${deviceName(c.deviceCode)} = ${c.statusValue === 'offline' ? t('dev_offline') : t('dev_online')}`
        } else {
          detail = `${deviceName(c.deviceCode)} / ${propName(c.deviceCode, c.propertyKey)} ${opLabel(c.operator)} ${c.value ?? ''}`
        }
        allItems.push({
          type: 'leaf',
          key: `cond-${props.depth}-${i}-${c._id}`,
          node: { _ref: c, id: c._id, title, detail, icon, tone: 'condition', badges: [] }
        })
      })

      subGroups.forEach((sg, i) => {
        allItems.push({ type: 'group', key: `group-${props.depth}-${i}-${sg._id}`, group: sg })
      })

      const isSel = ctxData.selectedNode?.value && ctxData.selectedNode.value._id === g._id
      const depthColorIndex = props.depth % 6

      const headerEl = h('div', { class: 'rg-cond-group__header cursor-pointer', onClick: (e) => { e.stopPropagation(); collapsed.value = !collapsed.value } }, [
        h('div', { class: ['rg-logic-pill', isOr ? 'rg-logic-pill--or' : 'rg-logic-pill--and'] }, [
          h('span', { class: 'rg-logic-pill__label' }, logicLabel),
          h('span', { class: 'rg-logic-pill__hint' }, hintLabel)
        ]),
        h('button', {
            class: 'rg-cond-group__toggle',
            style: 'pointer-events: none;'
          }, [ h('i', { class: ['bi', collapsed.value ? 'bi-chevron-down' : 'bi-chevron-up'] }) ])
      ])

      const deleteBtn = ctxData.isEditing?.value && props.depth > 0
        ? h('button', {
            class: 'rg-group__delete btn btn-sm btn-danger rounded-circle shadow-sm',
            type: 'button',
            draggable: false,
            onMousedown: ctxData.stopDeletePointerEvent,
            onClick: (e) => { e.stopPropagation(); ctxData.deleteNode(g) }
          }, [ h('i', { class: 'bi bi-x' }) ])
        : null

      if (collapsed.value) {
        return h('div', { class: ['rg-cond-group', `rg-group-color-${depthColorIndex}`] }, [
          deleteBtn,
          headerEl,
          h('div', { class: 'rg-cond-group__collapsed', onClick: () => { collapsed.value = false } }, [
            h('i', { class: 'bi bi-plus-circle-dotted' }),
            h('span', t('rule_graph_expand', { count: allItems.length }))
          ])
        ])
      }

      const itemEls = allItems.map((item) => {
        if (item.type === 'leaf') return h(RgCard, { node: item.node, key: item.key })
        return h(RgConditionGroup, { group: item.group, devices: props.devices, depth: props.depth + 1, key: item.key })
      })
      if (!itemEls.length) {
        itemEls.push(
          h('div', { class: 'rg-cond-group__empty' }, [
            h('i', { class: 'bi bi-node-plus' }),
            h('span', t('rule_graph_drop_condition_here', '拖入判断条件或条件组'))
          ])
        )
      }

      const isDragOver = ctxData.dragOverGroup?.value === g
      return h('div', { 
        class: [
          'rg-cond-group', 
          `rg-group-color-${depthColorIndex}`, 
          { 'rg-drag-over': isDragOver },
          { 'rg-node-selected': isSel }
        ],
        onClick: (e) => {
          if (ctxData.isEditing?.value) {
            e.stopPropagation()
            ctxData.selectNode(g, 'condition_group')
          }
        },
        draggable: ctxData.isEditing?.value && props.depth > 0,
        onDragstart: (e) => {
          if (ctxData.isEditing?.value && props.depth > 0) {
            e.dataTransfer.setData('text/plain', JSON.stringify({ item: g, group: 'condition_group', isExisting: true, sourceId: g._id || g.id }))
            e.stopPropagation()
          }
        },
        onDragover: (e) => {
          if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(g) }
        },
        onDragleave: () => { if (ctxData.isEditing?.value) ctxData.setDragOverGroup(null) },
        onDrop: (e) => {
          if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(null); ctxData.handleDropCondition(e, g) }
        }
      }, [
        deleteBtn,
        headerEl,
        h('div', { class: 'rg-cond-group__items' }, itemEls)
      ])
    }
  }
})

/* ============================================
 *  递归动作节点组件
 * ============================================ */
const RgActionNode = defineComponent({
  name: 'RgActionNode',
  props: {
    node: { type: Object, required: true },
    depth: { type: Number, default: 0 }
  },
  setup(props) {
    const { t } = useI18n()
    const ctxData = inject('rgContext', {})
    return () => {
      const node = props.node
      const g = node._ref
      const groupSubActions = () => {
        if (!g) return []
        g.subActions = g.subActions || []
        return g.subActions
      }
      const isDragOver = ctxData.dragOverGroup?.value === g
      const depthColorIndex = props.depth % 6
      const isSel = ctxData.selectedNode?.value && ctxData.selectedNode.value._id === g?._id
      const dropIndexForNode = (e) => {
        if (!Array.isArray(node._list) || !Number.isInteger(node._index)) return undefined
        const rect = e.currentTarget.getBoundingClientRect()
        return e.clientY < rect.top + rect.height / 2 ? node._index : node._index + 1
      }

      const deleteBtn = ctxData.isEditing?.value
        ? h('button', {
            class: 'rg-group__delete btn btn-sm btn-danger rounded-circle shadow-sm',
            type: 'button',
            draggable: false,
            onMousedown: ctxData.stopDeletePointerEvent,
            onClick: (e) => { e.stopPropagation(); ctxData.deleteNode(g) }
          }, [ h('i', { class: 'bi bi-x' }) ])
        : null

      const commonProps = {
        class: [
          `rg-group-color-${depthColorIndex}`,
          { 'rg-drag-over': isDragOver },
          { 'rg-node-selected': isSel }
        ],
        onClick: (e) => {
          if (ctxData.isEditing?.value) { e.stopPropagation(); ctxData.selectNode(g, 'action') }
        },
        draggable: ctxData.isEditing?.value,
        onDragstart: (e) => {
          if (ctxData.isEditing?.value) {
            const dragGroup = ctxData.dragGroupForNode?.(node) || 'action'
            e.dataTransfer.setData('text/plain', JSON.stringify({ item: g, group: dragGroup, isExisting: true, sourceId: g._id || g.id }))
            e.stopPropagation()
          }
        },
        onDragover: (e) => {
          if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(g); }
        },
        onDragleave: () => { if (ctxData.isEditing?.value) ctxData.setDragOverGroup(null) },
        onDrop: (e) => {
          if (ctxData.isEditing?.value) {
            e.preventDefault()
            e.stopPropagation()
            ctxData.setDragOverGroup(null)
            if (Array.isArray(node._list)) ctxData.handleDropAction(e, node._list, node._parentGroup || null, dropIndexForNode(e))
          }
        }
      }

      if (node.isParallel) {
        commonProps.class.unshift('rg-parallel-group')
        return h('div', commonProps, [
          deleteBtn,
          h('div', { class: 'rg-parallel-group__label' }, [
            h('i', { class: 'bi bi-cpu' }),
            t('rule_graph_parallel_run'),
            h('span', { class: 'rg-parallel-group__count' }, `(${node.children.length})`)
          ]),
          h('div', { class: 'rg-parallel-fork' }, [
            h('div', { class: 'rg-parallel-fork__stem' }),
            h('div', { class: 'rg-parallel-fork__rail' })
          ]),
          h('div', {
            class: 'rg-parallel-branches',
            onDragover: (e) => { if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(g) } },
            onDrop: (e) => { if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(null); ctxData.handleDropAction(e, groupSubActions(), g) } }
          }, node.children.map((child, i) => {
            return h('div', { class: 'rg-parallel-branch', key: child.id }, [
              h('div', { class: 'rg-parallel-branch__line' }),
              h(RgActionNode, { node: child, depth: props.depth + 1 })
            ])
          })),
          h('div', { class: 'rg-parallel-join' }, [
            h('div', { class: 'rg-parallel-join__rail' }),
            h('div', { class: 'rg-parallel-join__stem' })
          ])
        ])
      } else if (node.isSerial) {
        commonProps.class.unshift('rg-serial-group')
        return h('div', commonProps, [
          deleteBtn,
          h('div', { class: 'rg-serial-group__label' }, [
            h('i', { class: 'bi bi-list-ol' }),
            t('rule_graph_serial_run'),
            h('span', { class: 'rg-serial-group__count' }, `(${node.children.length})`)
          ]),
          h('div', {
            class: 'rg-serial-steps',
            onDragover: (e) => { if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(g) } },
            onDrop: (e) => { if (ctxData.isEditing?.value) { e.preventDefault(); e.stopPropagation(); ctxData.setDragOverGroup(null); ctxData.handleDropAction(e, groupSubActions(), g) } }
          }, node.children.map((child, ci) => {
            const arr = [ h(RgActionNode, { node: child, depth: props.depth + 1, key: child.id }) ]
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
        return h('div', { ...commonProps, class: ['rg-delay-node', { 'rg-node-selected': isSel }, { 'rg-drag-over': isDragOver }] }, [
          h('div', { class: 'rg-delay-node__icon' }, [ h('i', { class: 'bi bi-hourglass-split' }) ]),
          h('span', { class: 'rg-delay-node__text' }, t('rule_graph_delay_wait', { sec: node.delaySec })),
          deleteBtn
        ])
      } else {
        return h('div', { ...commonProps, class: ['rg-action-node-shell', { 'rg-node-selected': isSel }, { 'rg-drag-over': isDragOver }] }, [
          h(RgCard, { node })
        ])
      }
    }
  }
})

function injectId(obj) {
  if (obj && typeof obj === 'object') {
    if (!obj._id) obj._id = obj.id || `node_${Math.random().toString(36).substr(2, 9)}`
    if (!obj.id) obj.id = obj._id
    for (let k in obj) {
      if (Array.isArray(obj[k])) obj[k].forEach(injectId)
      else if (typeof obj[k] === 'object') injectId(obj[k])
    }
  }
}

export default {
  name: 'RuleGraphViewer',
  components: { VarInputWrapper, RgCard, RgConditionGroup, RgActionNode },
  props: {
    rule: { type: Object, default: () => null },
    devices: { type: Array, default: () => [] },
    groups: { type: Array, default: () => [] }
  },
  setup(props, ctx) {
    const { t } = useI18n()
    const isEditing = ref(false)
    const editableRule = ref(null)
    const selectedNode = ref(null)
    const referenceNodeId = ref(null)
    const graphViewerRef = ref(null)
    const dragOverGroup = ref(null)
    const savingEditing = ref(false)
    const saveMessage = ref('')
    const saveMessageType = ref('success')

    const collapsed = reactive({
      time: false,
      trigger: false,
      condition: false,
      action: false
    })

    const toggleSection = (sec) => { collapsed[sec] = !collapsed[sec] }

    watch(isEditing, (val) => {
      const dialog = document.querySelector('.rule-graph-dialog')
      if (dialog) {
        if (val) {
          dialog.style.maxWidth = '1750px'
          dialog.style.transition = 'max-width 0.3s ease'
        } else {
          dialog.style.maxWidth = ''
        }
      }
    })

    onUnmounted(() => {
      const dialog = document.querySelector('.rule-graph-dialog')
      if (dialog) {
        dialog.style.maxWidth = ''
      }
    })

    const nodeIdentity = (node) => node?._id || node?.id || null
    const stopDeletePointerEvent = (e) => {
      e.stopPropagation()
    }
    const preserveNodeIdentity = (node, next) => {
      const id = nodeIdentity(node) || nodeIdentity(next)
      const kind = node?._graphKind
      Object.keys(node).forEach(key => {
        if (!['id', '_id', '_graphKind'].includes(key)) delete node[key]
      })
      Object.assign(node, next)
      if (id) {
        node.id = id
        node._id = id
      }
      if (kind) node._graphKind = kind
    }
    const findAndDelete = (obj, targetId) => {
      if (!obj) return false
      if (Array.isArray(obj)) {
        const idx = obj.findIndex(x => x && nodeIdentity(x) === targetId)
        if (idx >= 0) { obj.splice(idx, 1); return true }
        for (const item of obj) if (findAndDelete(item, targetId)) return true
      } else if (typeof obj === 'object') {
        for (const key in obj) if (findAndDelete(obj[key], targetId)) return true
      }
      return false
    }
    const isConditionNodeRef = (node) => {
      if (!node || typeof node !== 'object') return false
      if (['property', 'device_status'].includes(node.type)) return true
      return Array.isArray(node.conditions) && Array.isArray(node.groups)
    }
    const deleteConditionFromGroup = (group, targetId) => {
      if (!group || !targetId) return false
      const conditionIndex = (group.conditions || []).findIndex(condition => nodeIdentity(condition) === targetId)
      if (conditionIndex >= 0) {
        group.conditions.splice(conditionIndex, 1)
        return true
      }

      const groupIndex = (group.groups || []).findIndex(child => nodeIdentity(child) === targetId)
      if (groupIndex >= 0) {
        group.groups.splice(groupIndex, 1)
        return true
      }

      return (group.groups || []).some(child => deleteConditionFromGroup(child, targetId))
    }
    const deleteConditionNode = (nodeRef) => {
      const targetId = nodeIdentity(nodeRef)
      const rootGroup = editableRule.value?.conditions
      if (!targetId || !rootGroup || nodeIdentity(rootGroup) === targetId) return false
      return deleteConditionFromGroup(rootGroup, targetId)
    }

    const deleteNode = (nodeRef) => {
      const targetId = nodeIdentity(nodeRef)
      if (!targetId) return
      const deleted = isConditionNodeRef(nodeRef)
        ? deleteConditionNode(nodeRef)
        : findAndDelete(editableRule.value, targetId)
      if (deleted && selectedNode.value && nodeIdentity(selectedNode.value) === targetId) selectedNode.value = null
    }
    const deleteSelectedNode = () => {
      if (canDeleteSelectedNode.value) deleteNode(selectedNode.value)
    }

    const dragGroupForNode = (node) => {
      if (!node || node.empty) return null
      if (node.tone === 'trigger') return 'trigger'
      if (node.tone === 'condition') return 'condition'
      if (node.tone === 'action') return 'action'
      return null
    }

    const makeNodeId = (prefix) => `${prefix}_${Date.now()}_${Math.random().toString(16).slice(2)}`
    const defaultCronSchedule = () => ({
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
    })
    const defaultActionFields = (type = 'set_property') => ({
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
      textContent: '',
      voiceText: '',
      llmPrompt: ''
    })
    const createTriggerNode = (item = {}) => {
      const type = item.type || 'property_change'
      const id = makeNodeId('trg')
      return {
        id,
        _id: id,
        type,
        deviceCode: '',
        propertyKey: '',
        operator: type === 'property_change' ? 'changed' : '',
        value: '',
        eventId: '',
        statusValue: 'online',
        cronMode: type === 'cron' ? 'visual' : '',
        cronExpr: '*/5 * * * *',
        cronDesc: '',
        schedule: defaultCronSchedule(),
        ...item
      }
    }
    const createConditionNode = (type = 'property') => {
      const id = makeNodeId('cond')
      return {
        id,
        _id: id,
        type,
        deviceCode: '',
        propertyKey: '',
        operator: 'eq',
        value: '',
        statusValue: 'online',
        startTime: '00:00:00',
        endTime: '24:00:00',
        weekdays: [],
        timezone: ''
      }
    }
    const createConditionGroupNode = (logic = 'and') => {
      const id = makeNodeId('cond_grp')
      return { id, _id: id, logic, conditions: [], groups: [] }
    }
    const createActionNode = (item = {}) => {
      const type = item.type || 'set_property'
      const id = makeNodeId(type === 'parallel_group' || type === 'sequence_group' ? 'act_grp' : 'act')
      const node = { id, _id: id, ...defaultActionFields(type), ...item }
      if ((type === 'parallel_group' || type === 'sequence_group') && !Array.isArray(node.subActions)) {
        node.subActions = [createActionNode({ type: 'set_property' })]
      }
      return node
    }
    const ensureBackendIds = (value) => {
      if (!value || typeof value !== 'object') return
      if (value._id && !value.id) value.id = value._id
      if (value._graphKind) delete value._graphKind
      if (value.type === 'event') delete value.eventFilter
      if (value.type === 'llm') {
        delete value.llmPlayAudio
        delete value.llmIncludeContext
      }
      Object.keys(value).forEach(key => {
        const child = value[key]
        if (Array.isArray(child)) child.forEach(ensureBackendIds)
        else if (child && typeof child === 'object') ensureBackendIds(child)
      })
    }

    const startEditing = () => {
      let cloned = JSON.parse(JSON.stringify(props.rule || {}))
      cloned.triggers = safeParse(cloned.triggers, [])
      cloned.conditions = safeParse(cloned.conditions, { logic: 'and', conditions: [], groups: [] })
      if (!cloned.conditions) cloned.conditions = { logic: 'and', conditions: [], groups: [] }
      cloned.actions = safeParse(cloned.actions, [])
      cloned.effective_time = safeParse(cloned.effective_time, { mode: 'always', windows: [] })
      if (!cloned.effective_time) cloned.effective_time = { mode: 'always', windows: [] }
      cloned.group_id = cloned.group_id || cloned.GroupID || null
      cloned.priority = cloned.priority || 50
      cloned.throttle_sec = cloned.throttle_sec || cloned.throttleSec || 60
      cloned.max_per_hour = cloned.max_per_hour || 60
      cloned.retry_count = cloned.retry_count || 0
      
      injectId(cloned)
      
      editableRule.value = cloned
      isEditing.value = true
      saveMessage.value = ''
    }

    const saveEditing = () => {
      if (savingEditing.value) return
      const ruleToSave = JSON.parse(JSON.stringify(editableRule.value))
      ensureBackendIds(ruleToSave)
      savingEditing.value = true
      saveMessage.value = ''
      ctx.emit('update-rule', ruleToSave, {
        done: (ok, message, savedRule) => {
          savingEditing.value = false
          if (ok && savedRule && editableRule.value) {
            Object.assign(editableRule.value, savedRule)
          }
          saveMessageType.value = ok ? 'success' : 'danger'
          saveMessage.value = message || (ok ? t('rule_save_success', '保存成功') : t('common_save_fail', '保存失败'))
        }
      })
    }

    watch(() => props.rule, (value) => {
      if (value && !value.code) startEditing()
    }, { immediate: true })

    const cancelEditing = () => {
      isEditing.value = false
      editableRule.value = null
      selectedNode.value = null
      savingEditing.value = false
      saveMessage.value = ''
    }

    const getDeviceProperties = (code) => {
      const dev = props.devices.find(d => d.code === code)
      return dev ? dev.properties || [] : []
    }
    const getDeviceEvents = (code) => {
      const dev = props.devices.find(d => d.code === code)
      return dev ? dev.events || [] : []
    }
    const boundedNumber = (value, min, max, fallback) => {
      const n = Number(value)
      if (!Number.isFinite(n)) return fallback
      return Math.min(max, Math.max(min, Math.trunc(n)))
    }
    const parseNumberList = (text, min, max, fallback) => {
      const values = String(text || '')
        .split(',')
        .map(item => Number.parseInt(item.trim(), 10))
        .filter(item => Number.isInteger(item) && item >= min && item <= max)
      return values.length ? values : fallback
    }
    const cronWeekdays = (text) => parseNumberList(text, 0, 7, [1, 2, 3, 4, 5]).map(day => day === 7 ? 0 : day).join(',')
    const cronMonthDays = (text) => parseNumberList(text, 1, 31, [1]).join(',')
    const cronExprPart = (value, fallback = '*') => String(value || '').trim() || fallback
    const buildCronExpression = (schedule = {}) => {
      const minute = boundedNumber(schedule.minute, 0, 59, 0)
      const hour = boundedNumber(schedule.hour, 0, 23, 0)
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
        case 'fields':
          return [
            cronExprPart(schedule.minuteExpr),
            cronExprPart(schedule.hourExpr),
            cronExprPart(schedule.dayOfMonthExpr),
            cronExprPart(schedule.monthExpr),
            cronExprPart(schedule.weekdayExpr)
          ].join(' ')
        case 'every_minutes':
        default:
          return `*/${boundedNumber(schedule.intervalMinutes, 1, 59, 5)} * * * *`
      }
    }
    const ensureCronNode = (node) => {
      if (!node || node.type !== 'cron') return
      node.cronMode = node.cronMode || 'visual'
      node.schedule = { ...defaultCronSchedule(), ...(node.schedule || {}) }
      if (node.cronMode === 'fields') node.schedule.mode = 'fields'
      if (!node.cronExpr) node.cronExpr = buildCronExpression(node.schedule)
    }
    const syncSelectedCron = () => {
      if (!selectedNode.value || selectedNode.value.type !== 'cron') return
      ensureCronNode(selectedNode.value)
      if (selectedNode.value.cronMode === 'fields') selectedNode.value.schedule.mode = 'fields'
      if (selectedNode.value.cronMode !== 'advanced') {
        selectedNode.value.cronExpr = buildCronExpression(selectedNode.value.schedule)
      }
    }
    const getDeviceServices = (code) => {
      const dev = props.devices.find(d => d.code === code)
      return dev ? dev.services || [] : []
    }

    const createIntArrayComputed = (key) => computed({
      get() {
        if (!selectedNode.value || !selectedNode.value[key]) return ''
        return selectedNode.value[key].join(',')
      },
      set(val) {
        if (!selectedNode.value) return
        if (!val) { selectedNode.value[key] = []; return }
        selectedNode.value[key] = val.split(',').map(s => parseInt(s.trim())).filter(n => !isNaN(n))
      }
    })
    const createTimeComputed = (key) => computed({
      get() {
        if (!selectedNode.value) return ''
        if (selectedNode.value.type === 'effective_time' && selectedNode.value.windows?.length) {
          const windowValue = selectedNode.value.windows[0]?.[key] || ''
          return windowValue.length >= 5 ? windowValue.substring(0, 5) : windowValue
        }
        if (!selectedNode.value[key]) return ''
        const val = selectedNode.value[key]
        return val.length >= 5 ? val.substring(0, 5) : val
      },
      set(val) {
        if (!selectedNode.value) return
        if (selectedNode.value.type === 'effective_time' && selectedNode.value.windows?.length) {
          selectedNode.value.windows[0] = { ...selectedNode.value.windows[0], [key]: val }
        }
        selectedNode.value[key] = val
      }
    })
    const computedStartTime = createTimeComputed('startTime')
    const computedEndTime = createTimeComputed('endTime')

    const computedEffectiveWeekdays = createIntArrayComputed('weekdays')
    const computedEffectiveMonthDays = createIntArrayComputed('monthDays')
    const computedEffectiveMonths = createIntArrayComputed('months')

    const computedServiceParams = computed({
      get() {
        if (!selectedNode.value || selectedNode.value.type !== 'call_service') return ''
        return selectedNode.value.serviceParams ? JSON.stringify(selectedNode.value.serviceParams, null, 2) : ''
      },
      set(val) {
        if (!selectedNode.value || selectedNode.value.type !== 'call_service') return
        try {
          selectedNode.value.serviceParams = JSON.parse(val || '{}')
        } catch(e) {
          // ignore parsing error while typing
        }
      }
    })

    const onDragStart = (e, item, group) => {
      e.dataTransfer.setData('text/plain', JSON.stringify({ item, group, isExisting: false }))
    }

    const onDropTrigger = (e) => {
      const data = JSON.parse(e.dataTransfer.getData('text/plain') || '{}')
      if (data.group === 'trigger' && data.isExisting) {
        moveExistingTrigger(data.sourceId)
      } else if (data.group === 'trigger' && !data.isExisting) {
        const triggers = editableRule.value.triggers || []
        triggers.push(createTriggerNode(data.item))
        editableRule.value.triggers = triggers
      }
    }
    const moveExistingTrigger = (sourceId) => {
      const triggers = editableRule.value.triggers || []
      const index = triggers.findIndex(trigger => nodeIdentity(trigger) === sourceId)
      if (index < 0) return
      const [node] = triggers.splice(index, 1)
      triggers.push(node)
      editableRule.value.triggers = triggers
    }
    const removeConditionFromGroup = (group, sourceId) => {
      if (!group) return null
      const conditionIndex = (group.conditions || []).findIndex(condition => nodeIdentity(condition) === sourceId)
      if (conditionIndex >= 0) {
        const [node] = group.conditions.splice(conditionIndex, 1)
        return { group: 'condition', node }
      }
      const groupIndex = (group.groups || []).findIndex(child => nodeIdentity(child) === sourceId)
      if (groupIndex >= 0) {
        const [node] = group.groups.splice(groupIndex, 1)
        return { group: 'condition_group', node }
      }
      for (const child of group.groups || []) {
        const found = removeConditionFromGroup(child, sourceId)
        if (found) return found
      }
      return null
    }
    const conditionGroupContains = (group, sourceId) => {
      if (!group) return false
      if (nodeIdentity(group) === sourceId) return true
      return (group.groups || []).some(child => conditionGroupContains(child, sourceId))
    }
    const findConditionGroupById = (group, sourceId) => {
      if (!group) return null
      if (nodeIdentity(group) === sourceId) return group
      for (const child of group.groups || []) {
        const found = findConditionGroupById(child, sourceId)
        if (found) return found
      }
      return null
    }
    const moveExistingCondition = (sourceId, targetGroup) => {
      if (!sourceId || !targetGroup) return
      if (nodeIdentity(targetGroup) === sourceId) return
      const movingGroup = findConditionGroupById(editableRule.value.conditions, sourceId)
      if (movingGroup && conditionGroupContains(movingGroup, nodeIdentity(targetGroup))) return
      const moved = removeConditionFromGroup(editableRule.value.conditions, sourceId)
      if (!moved) return
      if (moved.group === 'condition_group') {
        targetGroup.groups = targetGroup.groups || []
        targetGroup.groups.push(moved.node)
      } else {
        targetGroup.conditions = targetGroup.conditions || []
        targetGroup.conditions.push(moved.node)
      }
    }
    const moveExistingAction = (sourceId, targetList, targetGroup = null, insertIndex) => {
      moveActionToList(editableRule.value.actions || [], sourceId, targetList, insertIndex, targetGroup, nodeIdentity)
    }
    const selectedNodeKind = computed(() => selectedNode.value?._graphKind || '')
    const canDeleteSelectedNode = computed(() => {
      if (!selectedNode.value || selectedNode.value.type === 'effective_time') return false
      if (selectedNodeKind.value === 'condition_group' && nodeIdentity(selectedNode.value) === nodeIdentity(editableRule.value?.conditions)) return false
      return ['trigger', 'condition', 'condition_group', 'action'].includes(selectedNodeKind.value)
    })
    const selectedNodeTypeOptions = computed(() => {
      const kind = selectedNodeKind.value
      if (kind === 'trigger') {
        return ['property_change', 'event', 'device_status', 'cron'].map(value => ({ value, label: typeLabel(value) }))
      }
      if (kind === 'condition') {
        return ['property', 'device_status'].map(value => ({ value, label: typeLabel(value) }))
      }
      if (kind === 'action') {
        return ['set_property', 'call_service', 'notification', 'alarm', 'delay', 'text', 'llm', 'voice_playback', 'parallel_group', 'sequence_group'].map(value => ({ value, label: typeLabel(value) }))
      }
      return selectedNode.value?.type ? [{ value: selectedNode.value.type, label: typeLabel(selectedNode.value.type) }] : []
    })
    const changeSelectedNodeType = (type) => {
      if (!selectedNode.value || !type || selectedNode.value.type === type) return
      const kind = selectedNodeKind.value
      const id = nodeIdentity(selectedNode.value)
      if (kind === 'trigger') preserveNodeIdentity(selectedNode.value, createTriggerNode({ type, id, _id: id }))
      if (kind === 'condition') preserveNodeIdentity(selectedNode.value, createConditionNode(type))
      if (kind === 'action') preserveNodeIdentity(selectedNode.value, createActionNode({ type, id, _id: id }))
      selectedNode.value._graphKind = kind
    }

    const onDropConditionRoot = (e) => {
      const data = JSON.parse(e.dataTransfer.getData('text/plain') || '{}')
      if ((data.group === 'condition' || data.group === 'condition_group') && data.isExisting) {
        moveExistingCondition(data.sourceId, editableRule.value.conditions)
      } else if (data.group === 'condition' && !data.isExisting) {
        const conds = editableRule.value.conditions.conditions || []
        conds.push(createConditionNode(data.item.detailType || 'property'))
        editableRule.value.conditions.conditions = conds
      } else if (data.group === 'condition_group' && !data.isExisting) {
        const groups = editableRule.value.conditions.groups || []
        groups.push(createConditionGroupNode(data.item.logic))
        editableRule.value.conditions.groups = groups
      }
    }

    const handleDropCondition = (e, targetGroup) => {
      const data = JSON.parse(e.dataTransfer.getData('text/plain') || '{}')
      if ((data.group === 'condition' || data.group === 'condition_group') && data.isExisting) {
        moveExistingCondition(data.sourceId, targetGroup)
      } else if (data.group === 'condition' && !data.isExisting) {
        targetGroup.conditions = targetGroup.conditions || []
        targetGroup.conditions.push(createConditionNode(data.item.detailType || 'property'))
      } else if (data.group === 'condition_group' && !data.isExisting) {
        targetGroup.groups = targetGroup.groups || []
        targetGroup.groups.push(createConditionGroupNode(data.item.logic))
      }
    }

    const createDroppedAction = (data) => {
      if (data.group === 'action') return createActionNode(data.item)
      if (data.group === 'action_group') return createActionNode({ type: data.item.mode === 'parallel' ? 'parallel_group' : 'sequence_group' })
      return null
    }

    const handleActionDrop = (targetList, data, targetGroup = null, insertIndex) => {
      if (data.isExisting && data.sourceId) {
        moveExistingAction(data.sourceId, targetList, targetGroup, insertIndex)
      } else {
        insertActionAt(targetList, createDroppedAction(data), insertIndex)
      }
    }

    const onDropActionRoot = (e) => {
      e.preventDefault()
      e.stopPropagation()
      const data = JSON.parse(e.dataTransfer.getData('text/plain') || '{}')
      handleActionDrop(editableRule.value.actions, data)
    }

    const handleDropAction = (e, targetList, targetGroup = null, insertIndex) => {
      const data = JSON.parse(e.dataTransfer.getData('text/plain') || '{}')
      handleActionDrop(targetList, data, targetGroup, insertIndex)
    }

    const highlightReferenceNode = (id, scroll = false) => {
      referenceNodeId.value = id || null
      if (!id || !scroll) return
      nextTick(() => {
        const nodes = graphViewerRef.value?.querySelectorAll?.('[data-rg-node-id]') || []
        const target = Array.from(nodes).find(el => el.dataset.rgNodeId === id)
        target?.scrollIntoView?.({ behavior: 'smooth', block: 'center', inline: 'center' })
      })
    }

    const clearReferenceHighlight = () => {
      referenceNodeId.value = null
    }

    provide('rgContext', {
      isEditing,
      selectedNode,
      referenceNodeId,
      dragOverGroup,
      selectNode: (node, kind) => {
        if (node && typeof node === 'object' && kind) node._graphKind = kind
        ensureCronNode(node)
        selectedNode.value = node
      },
      deleteNode,
      dragGroupForNode,
      stopDeletePointerEvent,
      setDragOverGroup: (g) => { dragOverGroup.value = g },
      handleDropCondition,
      handleDropAction
    })


    // === 工具函数 ===
    function typeLabel(type) {
      const map = {
        'property_change': t('rule_trigger_property', '属性变更触发'),
        'event': t('rule_trigger_event', '事件触发'),
        'device_status': t('rule_trigger_status', '状态触发/判断'),
        'cron': t('rule_trigger_cron', '定时触发'),
        'property': t('rule_condition_property', '属性判断'),
        'set_property': t('rule_action_set_property', '设置属性'),
        'call_service': t('rule_action_call_service', '调用服务'),
        'notification': t('rule_action_notification', '消息通知'),
        'alarm': t('rule_action_alarm', '告警'),
        'delay': t('rule_action_delay', '延迟执行'),
        'text': t('rule_action_text', '文本组件'),
        'llm': t('rule_action_ai_reasoning', 'AI 推理'),
        'voice_playback': t('rule_action_voice_playback', '语音播放'),
        'parallel_group': t('rule_graph_parallel_run', '并行执行'),
        'sequence_group': t('rule_graph_serial_run', '串行执行组'),
        'condition_group_and': t('rule_graph_logic_and', '满足所有(AND)'),
        'condition_group_or': t('rule_graph_logic_or', '满足任一(OR)')
      }
      return map[type] || type
    }

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

    const triggerNodes = computed(() => {
      const triggers = safeParse(isEditing.value ? editableRule.value?.triggers : props.rule?.triggers, [])
      if (!triggers.length) {
        return [{ id: 'trig-empty', title: t('rule_trigger_required'), detail: '', icon: 'bi-exclamation-circle', tone: 'trigger', badges: [], empty: true }]
      }
      return triggers.map((trig, i) => {
        let icon = 'bi-activity', title = '', detail = '', badges = []
        switch (trig.type) {
          case 'cron':
            icon = 'bi-clock-history'
            title = t('rule_trigger_cron')
            detail = trig.cronDesc || trig.cronExpr || '-'
            badges = []
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
        return { _ref: trig, id: trig._id || `trig-${i}`, title, detail, icon, tone: 'trigger', badges }
      })
    })

    function hasConditionContent(group) {
      if (!group) return false
      if ((group.conditions || []).length > 0) return true
      if ((group.groups || []).length > 0) return true
      return false
    }
    const conditionTree = computed(() => safeParse(isEditing.value ? editableRule.value?.conditions : props.rule?.conditions, null))
    const hasConditions = computed(() => hasConditionContent(conditionTree.value))

    const effectiveNodes = computed(() => {
      const et = safeParse(isEditing.value ? editableRule.value?.effective_time : props.rule?.effective_time, null)
      const mode = et?.mode || 'always'
      const labels = {
        always: t('rule_effective_always'), daily: t('rule_effective_daily'),
        weekly: t('rule_effective_weekly'), monthly: t('rule_effective_monthly'),
        workday: t('rule_effective_workday'), holiday: t('rule_effective_holiday'),
        custom: t('rule_effective_custom')
      }
      
      if (!et) return []

      const windows = Array.isArray(et.windows) && et.windows.length
        ? et.windows
        : [{ startTime: et.startTime || '00:00', endTime: et.endTime || '24:00' }]
      
      et.type = 'effective_time' // to differentiate in property panel
      
      return windows.map((w, i) => {
        const badges = []
        if (mode === 'weekly' && et.weekdays?.length) badges.push(`${t('rule_effective_weekdays')}: ${et.weekdays.join(',')}`)
        if (mode === 'monthly' && w.monthDays?.length) badges.push(`${t('rule_effective_month_days')}: ${w.monthDays.join(',')}`)
        
          const fmtTime = (t) => t && t.length >= 5 ? t.substring(0, 5) : (t || '00:00')
          return { _ref: et, id: `eff-${i}`, title: labels[mode] || mode, detail: `${fmtTime(w.startTime || et.startTime)} ~ ${fmtTime(w.endTime || et.endTime || '24:00')}`, icon: 'bi-calendar3-range', tone: 'time', badges }
      })
    })

    function buildActionNode(action, prefix, list = null, index = -1, parentGroup = null) {
      const base = { _ref: action, _list: list, _index: index, _parentGroup: parentGroup, id: action._id || prefix, tone: 'action', badges: [] }
      if (action.type === 'parallel_group') {
        return {
          ...base,
          isParallel: true,
          children: (action.subActions || []).map((sa, j) => buildActionNode(sa, `${prefix}-p${j}`, action.subActions || [], j, action))
        }
      }
      if (action.type === 'sequence_group') {
        return {
          ...base,
          isSerial: true,
          children: (action.subActions || []).map((sa, j) => buildActionNode(sa, `${prefix}-s${j}`, action.subActions || [], j, action))
        }
      }
      if (action.type === 'delay') {
        return { ...base, isDelay: true, delaySec: action.delaySec || 0, icon: 'bi-hourglass-split', title: t('rule_action_delay') }
      }
      if (action.type === 'text') {
        return { ...base, icon: 'bi-text-paragraph', title: t('rule_action_text', '文本组件'), detail: action.textContent || t('rule_action_text_content', '文本内容') }
      }
      if (action.type === 'llm') {
        return { ...base, icon: 'bi-stars', title: t('rule_action_ai_reasoning', 'AI 推理'), detail: action.llmPrompt || t('rule_action_ai_reasoning_detail', '根据规则上下文执行智能判断') }
      }
      if (action.type === 'voice_playback') {
        return { ...base, icon: 'bi-volume-up-fill', title: t('rule_action_voice_playback', '语音播放'), detail: action.voiceText || t('rule_action_voice_playback', '语音播放') }
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
          detail: `${deviceName(action.deviceCode)} → ${serviceName(action.deviceCode, action.serviceCode)}` + (action.serviceParams ? ' {...}' : '')
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
      return { ...base, icon: 'bi-question-circle', title: action.type, detail: JSON.stringify(action).substring(0, 80) }
    }

    const actionNodes = computed(() => {
      const actions = safeParse(isEditing.value ? editableRule.value?.actions : props.rule?.actions, [])
      if (!actions.length) {
        return [{ id: 'act-empty', title: t('rule_action_required'), detail: '', icon: 'bi-exclamation-circle', tone: 'action', badges: [], empty: true }]
      }
      return actions.map((a, i) => buildActionNode(a, `act-${i}`, actions, i))
    })

    function flattenConditionNodes(group, result = []) {
      if (!group) return result
      ;(group.conditions || []).forEach(condition => result.push(condition))
      ;(group.groups || []).forEach(child => flattenConditionNodes(child, result))
      return result
    }

    function conditionDisplayNode(condition) {
      let title = t('rule_condition_property')
      let detail = ''
      if (condition.type === 'device_status') {
        title = t('rule_condition_status')
        detail = `${deviceName(condition.deviceCode)} = ${condition.statusValue === 'offline' ? t('dev_offline') : t('dev_online')}`
      } else {
        detail = `${deviceName(condition.deviceCode)} / ${propName(condition.deviceCode, condition.propertyKey)} ${opLabel(condition.operator)} ${condition.value ?? ''}`
      }
      return { label: title, detail }
    }

    function addDisplayNode(map, id, node) {
      if (!id || !node) return
      map[id] = { label: node.title || node.label || '', detail: node.detail || '' }
    }

    function collectActionDisplayNodes(nodes, map) {
      ;(nodes || []).forEach(node => {
        if (!node || node.empty) return
        addDisplayNode(map, node._ref?._id || node._ref?.id || node.id, node)
        if (node.children?.length) collectActionDisplayNodes(node.children, map)
      })
    }

    const nodeDisplay = computed(() => {
      const map = {}
      triggerNodes.value.forEach(node => {
        if (!node.empty) addDisplayNode(map, node._ref?._id || node._ref?.id || node.id, node)
      })
      flattenConditionNodes(conditionTree.value).forEach(condition => {
        const id = condition?._id || condition?.id
        const display = conditionDisplayNode(condition)
        if (id) map[id] = display
      })
      collectActionDisplayNodes(actionNodes.value, map)
      return map
    })
    const hasReferenceContent = () => {
      const kind = selectedNode.value?._graphKind || ''
      const type = selectedNode.value?.type || ''
      const actionTypes = ['set_property', 'call_service', 'notification', 'alarm', 'delay', 'text', 'llm', 'voice_playback', 'parallel_group', 'sequence_group']
      return kind === 'condition' || kind === 'condition_group' || kind === 'action' || type === 'property' || actionTypes.includes(type)
    }

    provide('ruleContext', {
      getContext: () => {
        const activeRule = isEditing.value ? editableRule.value : props.rule
        return {
          effectiveTime: safeParse(activeRule?.effective_time, null),
          triggers: safeParse(activeRule?.triggers, []),
          conditions: flattenConditionNodes(safeParse(activeRule?.conditions, null)),
          actions: safeParse(activeRule?.actions, []),
          rule: activeRule || {},
          currentNode: selectedNode.value,
          devices: props.devices || [],
          nodeDisplay: nodeDisplay.value
        }
      },
      highlightNode: highlightReferenceNode,
      clearHighlight: clearReferenceHighlight,
      hasReferenceContent
    })

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
      effectiveNodes, actionNodes, statusLabel, getDeviceProperties, getDeviceEvents, getDeviceServices,
      computedServiceParams, computedStartTime, computedEndTime, computedEffectiveWeekdays, computedEffectiveMonthDays, computedEffectiveMonths,
      typeLabel, syncSelectedCron, graphViewerRef, exportToImage, exportToPdf, selectedNodeTypeOptions, changeSelectedNodeType, canDeleteSelectedNode, deleteSelectedNode,
      isEditing, editableRule, startEditing, saveEditing, cancelEditing, selectedNode, onDragStart,
      onDropTrigger, onDropConditionRoot, onDropActionRoot,
      collapsed, toggleSection, savingEditing, saveMessage, saveMessageType
    }
  }
}
</script>

<style>
/* ======================================================================
 *  CSS 变量 — 明亮/暗黑双主题
 * ====================================================================== */
.rule-graph-viewer {
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
.rg-summary__text strong { font-size: 0.88rem; color: var(--rg-text-primary); }
.rg-summary__detail { font-size: 0.75rem; color: var(--rg-text-secondary); }

.rg-status { font-size: 0.78rem; font-weight: 600; padding: 0.1rem 0.45rem; border-radius: 0.25rem; display: inline-block; }
.rg-status--enabled  { background: rgba(16, 185, 129, 0.12); color: #059669; }
.rg-status--disabled { background: rgba(100, 116, 139, 0.12); color: #475569; }
.rg-status--draft    { background: rgba(59, 130, 246, 0.12); color: #2563eb; }
.rg-status--error    { background: rgba(239, 68, 68, 0.12); color: #dc2626; }

[data-bs-theme="dark"] .rg-status--enabled  { background: rgba(52, 211, 153, 0.15); color: #34d399; }
[data-bs-theme="dark"] .rg-status--disabled { background: rgba(148, 163, 184, 0.15); color: #94a3b8; }
[data-bs-theme="dark"] .rg-status--draft    { background: rgba(96, 165, 250, 0.15); color: #60a5fa; }
[data-bs-theme="dark"] .rg-status--error    { background: rgba(248, 113, 113, 0.15); color: #f87171; }

.rg-palette-section--ai {
  padding: 0.65rem;
  background: rgba(13, 202, 240, 0.06);
  border: 1px solid rgba(13, 202, 240, 0.22);
  border-radius: 0.5rem;
}

.rg-palette-item--ai {
  border-color: rgba(13, 202, 240, 0.32) !important;
}

.rg-palette-item--ai:hover {
  box-shadow: 0 0 0 2px rgba(13, 202, 240, 0.14) !important;
}

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
  top: 0; left: 0; right: 0; height: 1.5px;
  background: linear-gradient(90deg, transparent, var(--rg-trigger), transparent);
  animation: rg-scan 5s linear infinite;
  opacity: 0.4;
}
@keyframes rg-scan {
  0% { transform: translateY(0); opacity: 0; }
  10% { opacity: 0.5; }
  90% { opacity: 0.5; }
  100% { transform: translateY(800px); opacity: 0; }
}

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
.rg-section__pill i { font-size: 1rem; }
.rg-section__kicker { font-size: 0.65rem; text-transform: uppercase; letter-spacing: 0.06em; opacity: 0.8; }
.rg-section__title { font-size: 0.92rem; }
.rg-section__hint { font-size: 0.72rem; color: var(--rg-text-tertiary); font-style: italic; }

.rg-connector { position: relative; height: 2.5rem; width: 2px; margin: 0.25rem 0; z-index: 2; }
.rg-connector__line { position: absolute; inset: 0; width: 100%; background: linear-gradient(180deg, var(--rg-connector-color), var(--rg-connector-color)); }
.rg-connector__dot {
  position: absolute; left: -3px; width: 8px; height: 8px; border-radius: 50%;
  background: var(--rg-trigger); box-shadow: 0 0 6px var(--rg-trigger);
  animation: rg-pulse-down 2.5s infinite linear;
}
@keyframes rg-pulse-down {
  0% { top: 0; opacity: 0; }
  15% { opacity: 1; }
  85% { opacity: 1; }
  100% { top: 100%; opacity: 0; }
}

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
.rg-card:hover { transform: translateY(-2px); box-shadow: 0 8px 24px var(--rg-shadow-hover); }

/* 左侧色条 */
.rg-card::before {
  content: ''; position: absolute; left: 0; top: 12%; bottom: 12%; width: 3.5px; border-radius: 0 3px 3px 0; transition: all 0.3s;
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
.rg-card--empty { border-style: dashed; opacity: 0.6; cursor: default; }
.rg-card--empty::before { display: none; }
.rg-card--empty:hover { transform: none; box-shadow: 0 3px 12px var(--rg-shadow); }

/* 删除按钮 */
.rg-card__delete, .rg-group__delete {
  position: absolute;
  top: -8px;
  right: -8px;
  width: 24px;
  height: 24px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.2s, transform 0.2s;
  z-index: 10;
}
.rg-card:hover .rg-card__delete, .rg-cond-group:hover > .rg-group__delete, 
.rg-serial-group:hover > .rg-group__delete, .rg-parallel-group:hover > .rg-group__delete,
.rg-delay-node:hover > .rg-group__delete {
  opacity: 1;
}

/* 图标 */
.rg-card__icon {
  width: 2.1rem; height: 2.1rem; border-radius: 0.5rem; display: flex; align-items: center; justify-content: center; font-size: 1rem; flex-shrink: 0; transition: transform 0.25s;
}
.rg-card:hover .rg-card__icon { transform: scale(1.1); }
.rg-card__icon--trigger   { background: var(--rg-trigger-bg); color: var(--rg-trigger); }
.rg-card__icon--condition { background: var(--rg-condition-bg); color: var(--rg-condition); }
.rg-card__icon--time      { background: var(--rg-time-bg); color: var(--rg-time); }
.rg-card__icon--action    { background: var(--rg-action-bg); color: var(--rg-action); }

.rg-card__text { flex: 1; min-width: 0; }
.rg-card__title { font-weight: 600; font-size: 0.84rem; color: var(--rg-text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rg-card__detail { font-size: 0.75rem; color: var(--rg-text-secondary); margin-top: 0.15rem; word-break: break-all; line-height: 1.4; }

.rg-card__badges { display: flex; gap: 0.25rem; flex-shrink: 0; }
.rg-card__badge { font-size: 0.62rem; font-weight: 700; padding: 0.1rem 0.4rem; border-radius: 0.25rem; text-transform: uppercase; letter-spacing: 0.02em; }
.rg-card__badge--trigger   { background: var(--rg-trigger-bg); color: var(--rg-trigger); }
.rg-card__badge--condition { background: var(--rg-condition-bg); color: var(--rg-condition); }
.rg-card__badge--time      { background: var(--rg-time-bg); color: var(--rg-time); }
.rg-card__badge--action    { background: var(--rg-action-bg); color: var(--rg-action); }

.rg-logic-diamond { display: flex; align-items: center; justify-content: center; width: 2.2rem; height: 2.2rem; transform: rotate(45deg); border-radius: 0.35rem; flex-shrink: 0; }
.rg-logic-diamond span { transform: rotate(-45deg); font-size: 0.6rem; font-weight: 800; letter-spacing: 0.04em; }
.rg-logic-diamond--or { background: rgba(245, 158, 11, 0.12); border: 1.5px solid rgba(245, 158, 11, 0.35); color: #d97706; }
.rg-logic-diamond--and { background: rgba(59, 130, 246, 0.12); border: 1.5px solid rgba(59, 130, 246, 0.35); color: #2563eb; }

[data-bs-theme="dark"] .rg-logic-diamond--or { background: rgba(251, 191, 36, 0.12); border-color: rgba(251, 191, 36, 0.35); color: #fbbf24; }
[data-bs-theme="dark"] .rg-logic-diamond--and { background: rgba(96, 165, 250, 0.12); border-color: rgba(96, 165, 250, 0.35); color: #60a5fa; }

.rg-triggers { display: flex; flex-wrap: wrap; align-items: center; justify-content: center; gap: 0.75rem; width: 100%; }
.rg-triggers .rg-card { flex: 0 1 auto; max-width: 380px; min-width: 240px; }

.rg-conditions { width: 100%; display: flex; justify-content: center; }

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
  position: relative; width: 100%; display: flex; flex-direction: column; align-items: center;
  padding: 1.25rem 1rem 1rem; border: 1.5px dashed var(--group-border, var(--rg-condition-border));
  border-radius: 0.875rem; background: var(--group-bg, var(--rg-condition-bg)); margin: 1rem 0 0.5rem;
}

.rg-cond-group__header { position: absolute; top: -0.85rem; left: 50%; transform: translateX(-50%); display: flex; align-items: center; gap: 0.5rem; z-index: 5; }

.rg-logic-pill { display: inline-flex; align-items: center; gap: 0.35rem; padding: 0.25rem 0.75rem; border-radius: 1rem; font-size: 0.72rem; font-weight: 700; background: var(--group-bg, rgba(245, 158, 11, 0.1)); border: 1px solid var(--group-border, rgba(245, 158, 11, 0.2)); color: var(--group-color, #d97706); }
.rg-logic-pill__label { font-weight: 800; letter-spacing: 0.04em; }
.rg-logic-pill__hint { font-weight: 500; opacity: 0.75; }

.rg-cond-group__toggle { background: none; border: 1px solid var(--rg-border); border-radius: 50%; width: 1.5rem; height: 1.5rem; display: flex; align-items: center; justify-content: center; cursor: pointer; color: var(--rg-text-tertiary); transition: all 0.2s; font-size: 0.7rem; }
.rg-cond-group__toggle:hover { border-color: var(--rg-condition); color: var(--rg-condition); }

.rg-cond-group__items { display: flex; flex-direction: column; align-items: center; gap: 0.5rem; width: 100%; }
.rg-cond-group__empty { display: flex; align-items: center; justify-content: center; gap: 0.4rem; width: 100%; min-height: 2.75rem; padding: 0.6rem 1rem; background: var(--rg-surface); border: 1px dashed var(--rg-border); border-radius: 0.6rem; font-size: 0.78rem; color: var(--rg-text-tertiary); }
.rg-cond-group__collapsed { display: flex; align-items: center; justify-content: center; gap: 0.4rem; padding: 0.5rem 1rem; background: var(--rg-surface); border: 1px dashed var(--rg-border); border-radius: 0.5rem; font-size: 0.78rem; color: var(--rg-text-tertiary); cursor: pointer; transition: all 0.2s; }
.rg-cond-group__collapsed:hover { border-color: var(--rg-condition); color: var(--rg-condition); }

.rg-empty-hint { display: flex; align-items: center; justify-content: center; gap: 0.4rem; padding: 0.75rem 1.25rem; background: var(--rg-surface); border: 1px dashed var(--rg-border); border-radius: 0.75rem; font-size: 0.82rem; color: var(--rg-text-tertiary); font-style: italic; }

.rg-time-cards { display: flex; flex-wrap: wrap; justify-content: center; gap: 0.75rem; width: 100%; }
.rg-time-cards .rg-card { max-width: 340px; }

.rg-actions { display: flex; flex-direction: column; align-items: center; gap: 0; width: 100%; }
.rg-actions > .rg-card { max-width: 480px; }

.rg-step-connector { display: flex; flex-direction: column; align-items: center; height: 2rem; position: relative; }
.rg-step-connector__line { width: 2px; height: 100%; background: var(--rg-connector-color); }
.rg-step-connector__arrow { font-size: 0.65rem; color: var(--rg-action); margin-top: -0.1rem; line-height: 1; }

.rg-delay-node { position: relative; display: flex; align-items: center; justify-content: center; gap: 0.5rem; padding: 0.5rem 1.25rem; background: var(--rg-surface); border: 1.5px dashed var(--rg-connector-color); border-radius: 2rem; margin: 0.25rem 0; cursor: pointer; }
.rg-delay-node__icon { font-size: 0.9rem; color: var(--rg-text-tertiary); animation: rg-hourglass 2s ease-in-out infinite; }
@keyframes rg-hourglass { 0%, 100% { transform: rotate(0deg); } 50% { transform: rotate(180deg); } }
.rg-delay-node__text { font-size: 0.78rem; font-weight: 600; color: var(--rg-text-secondary); }

.rg-parallel-group { position: relative; width: 100%; display: flex; flex-direction: column; align-items: center; padding: 1.25rem; border: 1.5px dashed var(--group-border, rgba(99, 102, 241, 0.3)); border-radius: 1rem; background: var(--group-bg, rgba(99, 102, 241, 0.02)); margin: 0.25rem 0; }
[data-bs-theme="dark"] .rg-parallel-group { border-color: var(--group-border, rgba(129, 140, 248, 0.3)); background: var(--group-bg, rgba(129, 140, 248, 0.04)); }
.rg-parallel-group__label { position: absolute; top: -0.6rem; left: 50%; transform: translateX(-50%); display: flex; align-items: center; gap: 0.3rem; padding: 0.15rem 0.65rem; border-radius: 1rem; font-size: 0.7rem; font-weight: 700; background: var(--group-bg, rgba(99, 102, 241, 0.12)); color: var(--group-color, #6366f1); border: 1px solid var(--group-border, rgba(99, 102, 241, 0.2)); z-index: 5; }
[data-bs-theme="dark"] .rg-parallel-group__label { background: var(--group-bg, rgba(129, 140, 248, 0.15)); color: var(--group-color, #818cf8); border-color: var(--group-border, rgba(129, 140, 248, 0.25)); }
.rg-parallel-group__count { font-weight: 500; opacity: 0.7; }

.rg-parallel-fork, .rg-parallel-join { display: flex; flex-direction: column; align-items: center; width: 100%; }
.rg-parallel-fork__stem, .rg-parallel-join__stem { width: 2px; height: 12px; background: var(--rg-connector-color); }
.rg-parallel-fork__rail, .rg-parallel-join__rail { width: 80%; max-width: 600px; height: 2px; background: var(--rg-connector-color); }

.rg-parallel-branches { display: flex; justify-content: center; gap: 0.75rem; width: 100%; flex-wrap: wrap; padding: 0.25rem 0; }
.rg-parallel-branch { flex: 1; min-width: 200px; max-width: 320px; display: flex; flex-direction: column; align-items: center; }
.rg-parallel-branch__line { width: 2px; height: 14px; background: var(--rg-connector-color); }
.rg-parallel-branch .rg-card { max-width: 100%; }

.rg-serial-group { position: relative; width: 100%; display: flex; flex-direction: column; align-items: center; padding: 1.25rem; border: 1.5px dashed var(--group-border, var(--rg-action-border)); border-radius: 1rem; background: var(--group-bg, var(--rg-action-bg)); margin: 0.25rem 0; }
.rg-serial-group__label { position: absolute; top: -0.6rem; left: 50%; transform: translateX(-50%); display: flex; align-items: center; gap: 0.3rem; padding: 0.15rem 0.65rem; border-radius: 1rem; font-size: 0.7rem; font-weight: 700; background: var(--group-bg, var(--rg-action-bg)); color: var(--group-color, var(--rg-action)); border: 1px solid var(--group-border, var(--rg-action-border)); z-index: 5; }
.rg-serial-group__count { font-weight: 500; opacity: 0.7; }
.rg-serial-steps { display: flex; flex-direction: column; align-items: center; gap: 0; width: 100%; }
.rg-serial-arrow { position: relative; height: 1.5rem; width: 2px; margin: 0.15rem 0; }
.rg-serial-arrow__line { position: absolute; inset: 0; background: var(--rg-connector-color); }
.rg-serial-arrow__pulse { position: absolute; left: -2.5px; width: 7px; height: 7px; border-radius: 50%; background: var(--rg-action); box-shadow: 0 0 6px var(--rg-action); animation: rg-pulse-down 1.8s infinite linear; }

@media (max-width: 768px) {
  .rg-canvas { padding: 1.5rem 1rem; }
  .rg-triggers { flex-direction: column; }
  .rg-triggers .rg-card { max-width: 100%; }
  .rg-parallel-branches { flex-direction: column; align-items: center; }
  .rg-parallel-branch { max-width: 100%; }
  .rg-summary { flex-direction: column; }
  .rg-summary__item { min-width: auto; }
}
.rg-toolbar { position: absolute; top: -45px; right: 0; z-index: 10; }

.rg-drag-over { outline: 2px dashed var(--rg-trigger) !important; outline-offset: 2px; background-color: rgba(139, 92, 246, 0.05); }
.rg-node-selected { outline: 2px solid #0d6efd !important; outline-offset: -2px; box-shadow: 0 0.5rem 1rem rgba(0, 0, 0, 0.15) !important; }
.rg-node-referenced {
  outline: 2px solid #8b5cf6 !important;
  outline-offset: 3px;
  box-shadow: 0 0 0 4px rgba(139, 92, 246, 0.16), 0 0.65rem 1.35rem rgba(15, 23, 42, 0.18) !important;
}
</style>
