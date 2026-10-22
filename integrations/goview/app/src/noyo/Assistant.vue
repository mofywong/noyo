<template>
  <aside class="ioc-assistant" :class="{ 'is-expanded': expanded }" :aria-label="t('IOC AI 助手', 'IOC AI assistant')" @keydown="onPanelKeydown">
    <header class="ioc-assistant-heading">
      <strong>{{ t('IOC AI 助手', 'IOC AI assistant') }}</strong>
      <div class="ioc-assistant-actions">
        <button type="button" :aria-pressed="!!expanded" @click="$emit('expand')">{{ expanded ? t('恢复宽度', 'Restore width') : t('展开', 'Expand') }}</button>
        <button type="button" :aria-label="t('关闭助手', 'Close assistant')" @click="$emit('close')">×</button>
      </div>
    </header>
    <div class="ioc-assistant-sessions" :aria-busy="sessionsBusy">
      <label class="ioc-assistant-session-select">
        <span>{{ t('历史对话', 'Conversation history') }}</span>
        <select :value="activeConversation" :aria-label="t('历史对话', 'Conversation history')" :disabled="sessionLocked || !sessionsLoaded" @change="onConversationChange">
          <option v-if="!sessions.length" value="">{{ t('新对话', 'New conversation') }}</option>
          <option v-for="session in sessions" :key="session.id" :value="session.id">{{ session.title || (session.id === 'legacy' ? t('以前的对话', 'Earlier conversation') : t('新对话', 'New conversation')) }}</option>
        </select>
      </label>
      <button type="button" :disabled="sessionLocked || !sessionsLoaded" @click="newConversation">{{ t('新开对话', 'New conversation') }}</button>
      <button v-if="sessionsHasMore" type="button" :disabled="sessionLocked" @click="loadSessions(true)">{{ t('更多对话', 'More conversations') }}</button>
      <span v-if="sessionsBusy" role="status">{{ t('正在读取对话…', 'Loading conversations…') }}</span>
      <p v-if="sessionsError" class="ioc-assistant-warning" role="alert">{{ t('对话操作失败，原有记录已保留。', 'Conversation action failed. Existing history is preserved.') }} <button type="button" :disabled="sessionLocked" @click="loadSessions()">{{ t('重试', 'Retry') }}</button></p>
    </div>
    <p class="ioc-assistant-purpose">{{ t('仅用于 IOC 大屏的设计、生成和编辑；对话自动保存。', 'For IOC screen design, generation and editing only. Conversations save automatically.') }}</p>
    <div ref="conversation" class="ioc-assistant-conversation" :class="{ 'is-empty': !messages.length && !historyBusy }" role="log" :aria-label="t('大屏设计对话', 'Screen design conversation')" aria-live="polite" :aria-busy="busy || historyBusy">
      <button v-if="hasMore" type="button" :disabled="historyBusy" @click="loadHistory(true)">{{ t('加载更早的对话', 'Load earlier messages') }}</button>
      <p v-if="historyBusy" class="ioc-assistant-status" role="status">{{ t('正在读取对话…', 'Loading conversation…') }}</p>
      <p v-if="historyError" class="ioc-assistant-warning" role="alert">{{ t('对话记录读取失败', 'Could not load conversation') }} <button type="button" @click="loadHistory(historyLoaded && hasMore)">{{ t('重试', 'Retry') }}</button></p>
      <section v-if="!messages.length && !historyBusy" class="ioc-assistant-welcome">
        <div class="ioc-assistant-welcome-intro">
          <span class="ioc-assistant-welcome-mark" aria-hidden="true">
            <svg viewBox="0 0 24 24"><path d="M12 3.2l1.9 4.6 4.6 1.9-4.6 1.9L12 16.2l-1.9-4.6L5.5 9.7l4.6-1.9z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" /><path d="M17.6 15.4l.9 2.1 2.1.9-2.1.9-.9 2.1-.9-2.1-2.1-.9 2.1-.9z" fill="currentColor" /></svg>
          </span>
          <div>
            <h2>{{ t('你想设计怎样的 IOC 大屏？', 'What IOC screen would you like to design?') }}</h2>
            <p>{{ t('描述业务目标即可。AI 会根据产品物模型选择指标、组件与布局，自动绑定数据；尚未上报的设备也可参与设计。', 'Describe your business goal. AI selects metrics, widgets and layout from product models and binds their data, including devices that have not reported yet.') }}</p>
          </div>
        </div>
        <div class="ioc-assistant-examples" role="group" :aria-label="t('示例需求', 'Example requests')">
          <button v-for="(example, index) in examples" :key="index" type="button" @click="prompt = example">
            <span>{{ example }}</span>
            <svg class="ioc-assistant-example-arrow" viewBox="0 0 24 24" aria-hidden="true"><path d="M9 6l6 6-6 6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" /></svg>
          </button>
        </div>
      </section>
      <article v-for="message in messages" :key="message.id" class="ioc-assistant-message" :class="message.role" :aria-label="message.role === 'user' ? t('你说', 'You said') : t('助手回复', 'Assistant reply')">
        <svg v-if="message.role === 'user'" class="ioc-assistant-avatar" viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="8.5" r="3.6" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M4.8 20c0-3.9 3.2-6.4 7.2-6.4s7.2 2.5 7.2 6.4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>
        <svg v-else class="ioc-assistant-avatar" viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3.2l1.9 4.6 4.6 1.9-4.6 1.9L12 16.2l-1.9-4.6L5.5 9.7l4.6-1.9z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/><path d="M17.6 15.4l.9 2.1 2.1.9-2.1.9-.9 2.1-.9-2.1-2.1-.9 2.1-.9z" fill="currentColor"/></svg>
        <div class="ioc-assistant-bubble">
          <div v-if="message.context" class="ioc-assistant-message-context" :aria-label="t('本次上下文', 'Context for this request')">
            <span v-if="message.context.area">{{ t('画布选区', 'Canvas selection') }} · {{ areaLabel(message.context.area) }}</span>
            <span v-if="message.context.scope?.mode === 'project'">{{ t('项目产品定义', 'Project product definitions') }}</span>
            <details v-if="message.context.products?.length"><summary>{{ t('产品', 'Products') }} · {{ message.context.products.length }}</summary><p v-for="product in message.context.products" :key="product.code">{{ product.name || product.code }}</p></details>
            <span v-for="item in message.context.items" :key="item.id">{{ contextLabel(item) }}</span>
            <details v-if="message.context.devices?.length"><summary>{{ t('指定设备', 'Specific devices') }} · {{ message.context.devices.length }}</summary><p v-for="device in message.context.devices" :key="device.code">{{ device.name || device.code }} · {{ device.code }}</p></details>
          </div>
          <details v-if="message.stages?.length" class="ioc-assistant-progress" :open="message.stagesOpen !== false" @toggle="rememberStages(message, $event)">
            <summary>
              <span class="ioc-assistant-stage-current">{{ stageLabel(message.stages[message.stages.length - 1]) }}<span v-if="message.status === 'generating'" class="ioc-assistant-dots" role="status">…</span></span>
              <span class="ioc-assistant-stage-count">{{ message.stages.length }} {{ t('步', 'steps') }}</span>
            </summary>
            <ol>
              <li v-for="(stage, index) in message.stages" :key="stage" :class="{ 'is-done': index < message.stages.length - 1 || message.status !== 'generating' }">{{ stageLabel(stage) }}</li>
            </ol>
          </details>
          <div v-if="message.content" class="ioc-assistant-richtext" :class="{ 'is-collapsed': isLong(message.content) && !expandedMessages.has(message.id) }">
            <template v-for="(block, index) in renderBlocks(message.content)" :key="index">
              <p v-if="block.type === 'p'"><template v-for="(span, at) in block.spans" :key="at"><strong v-if="span.type === 'strong'">{{ span.text }}</strong><code v-else-if="span.type === 'code'">{{ span.text }}</code><template v-else>{{ span.text }}</template></template></p>
              <ul v-else-if="block.type === 'ul'"><li v-for="(item, at) in block.items" :key="at"><template v-for="(span, i2) in item" :key="i2"><strong v-if="span.type === 'strong'">{{ span.text }}</strong><code v-else-if="span.type === 'code'">{{ span.text }}</code><template v-else>{{ span.text }}</template></template></li></ul>
            </template>
          </div>
          <button v-if="isLong(message.content)" type="button" class="ioc-assistant-more" @click="toggleExpanded(message.id)">{{ expandedMessages.has(message.id) ? t('收起', 'Show less') : t('展开全文', 'Show more') }}</button>
          <small v-if="message.status && message.status !== 'generating' && message.role === 'assistant'" class="ioc-assistant-status">{{ statusLabel(message.status) }}<template v-if="message.changeCount"> · {{ message.changeCount }} {{ t('项变更', 'changes') }}</template></small>
        </div>
      </article>
      <section v-if="proposal && candidate" class="ioc-assistant-proposal" :aria-label="t('待应用方案', 'Pending proposal')">
        <AssistantPreview :document="candidate.document" :locale="state.locale" @validated="onPreviewValidated" />
        <p v-if="proposal.resultStatus === 'structure_only'" class="ioc-assistant-warning" role="status">{{ t('布局草稿已生成，部分数据能力尚未接通。完善数据能力后可应用。', 'The layout draft is ready. Some required data capabilities need to be connected before applying.') }}</p>
        <ul v-if="proposal.limitations?.length" class="ioc-assistant-limitations"><li v-for="limitation in proposal.limitations" :key="limitation">{{ limitation }}</li></ul>
        <p v-if="previewIssues.length" class="ioc-assistant-warning" role="alert">{{ t('预览发现组件越界或文字溢出，请调整后再应用。', 'Preview detected content outside its bounds or overflowing text. Adjust the proposal before applying.') }}</p>
        <details><summary>{{ t('查看变更', 'Review changes') }} · {{ candidate.changes.length }}</summary>
          <ol><li v-for="change in candidate.changes" :key="change.id">{{ actionLabel(change.type) }} · {{ change.title || widgetLabel(change.key) }}</li></ol>
        </details>
        <p v-if="stale" class="ioc-assistant-warning" role="status">{{ t('画布已变化，请重新生成。', 'The canvas changed. Generate again.') }}</p>
        <button type="button" :disabled="busy || applying || stale || !editable || !proposalApplicable || !previewValidated" @click="requestApply">{{ t('应用方案', 'Apply proposal') }}</button>
        <button type="button" :disabled="applying" @click="discardProposal">{{ t('放弃方案', 'Discard proposal') }}</button>
      </section>
    </div>
    <div v-if="task" class="ioc-assistant-task" role="status">
      <span>{{ taskStateLabel(task.state) }}<template v-if="task.stage"> · {{ stageLabel(task.stage) }}</template></span>
      <button v-if="['queued', 'running'].includes(task.state)" type="button" :disabled="canceling" @click="cancelTask">{{ t('取消生成', 'Cancel generation') }}</button>
      <button v-if="task.state === 'failed'" type="button" :disabled="busy || applying || !editable || task.attempt >= 3" @click="resumeTask">{{ t('从此步骤继续', 'Continue from this step') }}</button>
      <button v-if="task.state === 'disconnected'" type="button" :disabled="busy" @click="recoverTask">{{ t('重新连接任务', 'Reconnect task') }}</button>
    </div>
    <div v-if="undoEntry" class="ioc-assistant-undo">
      <span>{{ undoStale ? t('画布已变化', 'Canvas changed') : t('AI 修改已应用', 'AI changes applied') }}</span>
      <button type="button" :disabled="busy || applying || undoStale || !editable" @click="confirmation = 'undo'">{{ t('回退上次修改', 'Undo last change') }}</button>
    </div>
    <div v-if="syncError" class="ioc-assistant-warning ioc-assistant-sync" role="alert">{{ t('对话尚未保存', 'Conversation not saved') }} <button type="button" @click="retrySaves">{{ t('重试保存', 'Retry saving') }}</button></div>
    <div v-if="error" ref="errorElement" class="ioc-assistant-error" role="alert" tabindex="-1">{{ error }}</div>
    <form class="ioc-assistant-compose" @submit.prevent="generate">
      <div class="ioc-assistant-selection-chips" role="list" :aria-label="t('已选上下文', 'Selected context')">
        <span role="listitem">{{ scope.mode === 'project' ? t('项目产品定义', 'Project product definitions') : t('已选上下文', 'Selected context') }}<button v-if="scope.mode === 'project' || selectedProducts.length || selectedCodes.length" type="button" :disabled="locked" :aria-label="t('清空产品与设备上下文', 'Clear product and device context')" @click="clearScope">×</button></span>
        <span v-if="scope.mode === 'project' || selectedProducts.length" role="listitem">{{ t('产品定义', 'Product definitions') }} · {{ scope.mode === 'project' ? (products || []).length : selectedProducts.length }}</span>
        <span v-if="selectedCodes.length" role="listitem">{{ t('指定设备', 'Specific devices') }} · {{ selectedCodes.length }}</span>
        <span v-if="selectionArea" role="listitem">{{ t('画布选区', 'Canvas selection') }} · {{ areaLabel(selectionArea) }}<button type="button" :disabled="locked" :aria-label="t('清空画布上下文', 'Clear canvas context')" @click="$emit('clear-selection')">×</button></span>
        <span v-for="item in contextItems" :key="item.id" role="listitem">{{ contextLabel(item) }}<button type="button" :disabled="locked" :aria-label="t('移除上下文组件：', 'Remove context widget: ') + contextLabel(item)" @click="$emit('remove-selection', item.id)">×</button></span>
        <button v-if="contextItems.length || selectionArea" type="button" :disabled="locked" @click="$emit('clear-selection')">{{ t('清空', 'Clear') }}</button>
      </div>
      <p v-if="selectionMode" class="ioc-assistant-selection-hint" role="status">{{ selectionMode === 'area' ? t('在画布上拖出区域', 'Drag an area on the canvas') : t('点击组件以添加或移除', 'Click widgets to add or remove them') }}<button type="button" @click="$emit('select-context', '')">{{ t('完成', 'Done') }}</button></p>
      <label for="ioc-assistant-prompt" class="ioc-assistant-input-label">{{ t('你的需求', 'Your request') }}</label>
      <textarea id="ioc-assistant-prompt" ref="promptElement" v-model="prompt" rows="3" maxlength="4000" :disabled="locked || !sessionsLoaded || historyBusy" :placeholder="t('描述 IOC 大屏的设计、生成或编辑需求…', 'Describe an IOC screen design, generation or editing request…')" @keydown.ctrl.enter.prevent="generate" @keydown.meta.enter.prevent="generate" />
      <div class="ioc-assistant-compose-footer">
        <div class="ioc-assistant-tools">
          <div ref="deviceControl" class="ioc-assistant-device-control">
            <button ref="productTrigger" type="button" aria-haspopup="dialog" :aria-expanded="scopePicker === 'products'" aria-controls="ioc-assistant-scope-picker" :disabled="locked" @click="toggleScopePicker('products')">{{ t('产品', 'Products') }}<template v-if="selectedProducts.length"> · {{ selectedProducts.length }}</template></button>
            <button ref="deviceTrigger" type="button" aria-haspopup="dialog" :aria-expanded="scopePicker === 'devices'" aria-controls="ioc-assistant-scope-picker" :disabled="locked" @click="toggleScopePicker('devices')">{{ t('设备', 'Devices') }}<template v-if="selectedCodes.length"> · {{ selectedCodes.length }}</template></button>
            <section v-if="scopePicker" id="ioc-assistant-scope-picker" ref="scopeDialog" class="ioc-assistant-device-picker" role="dialog" :aria-label="scopePicker === 'products' ? t('选择产品', 'Select products') : t('选择具体设备', 'Select specific devices')" @keydown="onPickerKeydown">
              <div class="ioc-assistant-picker-heading"><strong>{{ scopePicker === 'products' ? t('选择产品', 'Select products') : t('选择具体设备', 'Select specific devices') }}</strong><button type="button" :aria-label="t('关闭范围选择', 'Close scope selector')" @click="closeScopePicker">×</button></div>
              <p class="ioc-assistant-picker-description">{{ scopePicker === 'products' ? t('仅提供产品信息与物模型定义，不会勾选关联设备。AI 选定指标后，再解析所需的产品设备范围。', 'Provide product information and thing-model definitions only. Related devices stay unselected; device scope is resolved after AI selects metrics.') : t('仅将明确勾选的设备作为设备上下文，与产品定义独立选择。', 'Only explicitly checked devices become device context, independently of product definitions.') }}</p>
              <input ref="scopeSearchElement" v-model="scopeSearch" type="search" :aria-label="scopePicker === 'products' ? t('搜索产品', 'Search products') : t('搜索设备', 'Search devices')" :placeholder="t('搜索名称或编码', 'Search name or code')" />
              <p v-if="scope.mode === 'project'" class="ioc-assistant-picker-description">{{ t('当前自动发现项目产品定义；具体设备需另行勾选。', 'Product definitions are discovered automatically; specific devices require an explicit selection.') }}</p>
              <div class="ioc-assistant-picker-actions"><button v-if="scopePicker === 'products'" type="button" :disabled="catalogLoading" @click="useProjectScope">{{ t('项目产品定义', 'Project product definitions') }}</button><button type="button" :disabled="catalogLoading || !(scopePicker === 'products' ? filteredProducts.length : filteredDevices.length)" @click="selectVisibleScope">{{ t('选择搜索结果', 'Select search results') }}</button><button type="button" @click="clearPickerScope">{{ t('清空当前选择', 'Clear this selection') }}</button><button type="button" :disabled="catalogLoading" @click="$emit('refresh-catalog')">{{ t('刷新', 'Refresh') }}</button></div>
              <p class="ioc-assistant-scope" role="status"><span>{{ t('指定设备', 'Specific devices') }} · {{ scopeCount }}</span><span>{{ t('物模型属性', 'Model properties') }} · {{ catalogSummary.fields }}</span><span>{{ t('物模型事件', 'Model events') }} · {{ catalogSummary.events }}</span></p>
              <p v-if="catalogLoading" role="status">{{ t('正在读取上下文目录…', 'Loading context catalog…') }}</p>
              <p v-else-if="catalogError" class="ioc-assistant-warning" role="alert">{{ catalogError }}</p>
              <p v-else-if="!(scopePicker === 'products' ? filteredProducts.length : filteredDevices.length)" class="ioc-assistant-picker-empty">{{ t('没有匹配的结果', 'No matching results') }}</p>
              <p v-if="scopePicker === 'devices' && catalogMeta?.truncated" class="ioc-assistant-picker-description">{{ t('设备列表仅展示部分目录。需要分类聚合时可提供产品定义，AI 选定指标后再解析设备范围。', 'Only part of the device catalog is shown. For category aggregates, provide product definitions; device scope is resolved after metric selection.') }}</p>
              <div class="ioc-assistant-device-options">
                <template v-if="scopePicker === 'products'"><label v-for="product in filteredProducts" :key="product.code"><input type="checkbox" :checked="selectedProductCodes.includes(product.code)" @change="toggleProduct(product.code, ($event.target as HTMLInputElement).checked)" /><span>{{ product.name || product.code }}<small>{{ product.code }} · {{ product.fieldCount ?? (product.fields || []).length }} {{ t('项属性', 'properties') }} · {{ (product.events || []).length }} {{ t('项事件', 'events') }}</small></span></label></template>
                <template v-else><label v-for="device in filteredDevices" :key="device.code"><input type="checkbox" :checked="selectedCodes.includes(device.code)" @change="toggleDevice(device.code, ($event.target as HTMLInputElement).checked)" /><span>{{ device.name || device.code }}<small>{{ device.code }}</small></span></label></template>
              </div>
            </section>
          </div>
          <button type="button" :aria-pressed="selectionMode === 'components'" :aria-label="t('选择多个组件', 'Select multiple widgets')" :disabled="locked" @click="$emit('select-context', selectionMode === 'components' ? '' : 'components')">{{ t('组件', 'Widgets') }}</button>
          <button type="button" :aria-pressed="selectionMode === 'area'" :aria-label="t('框选画布区域', 'Select canvas area')" :disabled="locked" @click="$emit('select-context', selectionMode === 'area' ? '' : 'area')">{{ t('画布选区', 'Canvas selection') }}</button>
        </div>
        <button type="submit" class="ioc-assistant-primary" :disabled="!prompt.trim() || locked || historyBusy || sessionsBusy || !sessionsLoaded || historyError">{{ busy ? t('生成中…', 'Generating…') : applying ? t('应用中…', 'Applying…') : t('生成方案', 'Generate proposal') }}</button>
      </div>
    </form>
    <n-modal :show="!!confirmation" :closable="false" preset="dialog" :title="confirmation === 'undo' ? t('回退 AI 修改', 'Undo AI change') : t('确认移除内容', 'Confirm removal')" :content="confirmation === 'undo' ? t('恢复到上次 AI 应用前的画布及数据绑定？', 'Restore the canvas and bindings from before the last AI change?') : t('此方案会删除组件或解除数据绑定。确认应用？', 'This proposal removes widgets or data bindings. Apply it?')" :positive-text="t('确认', 'Confirm')" :negative-text="t('取消', 'Cancel')" @update:show="value => { if (!value) confirmation = '' }" @positive-click="confirmAction" @negative-click="confirmation = ''" />
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { createMaterial, materials } from './materials'
import { kindsOf, targetOf, widgetByKey } from './widgets'
import { documentSignature, on, rpc, state } from './bridge'
import { compileAssistantPlan } from './assistantPlan.js'
import { applyScreenTheme } from './screenTheme'
import { expandBlueprint } from './layout'
import { selectedContext } from './assistantContext.js'
import { assistantScope, assistantScopeCount, assistantScopeRequest } from './assistantScope.js'
import AssistantPreview from './AssistantPreview.vue'

type ContextArea = { x: number; y: number; w: number; h: number }
type ContextItem = { id: string; key: string; title: string }
type Message = { id: string; conversationId?: string; role: 'user' | 'assistant'; content: string; status?: string; stages?: string[]; stagesOpen?: boolean; changeCount?: number; taskId?: string; context?: { items: ContextItem[]; area: ContextArea | null; devices: { code: string; name: string }[]; products?: { code: string; name: string }[]; scope?: any } }
type Conversation = { id: string; title: string; createdAt: string; updatedAt: string }
const props = defineProps<{ active: boolean; expanded?: boolean; catalogLoading?: boolean; catalogError?: string; snapshot: () => any; applyDocument: (document: any) => Promise<void>; selectionIds: string[]; selectionArea?: ContextArea | null; selectionMode?: string; catalog: any[]; products?: any[]; catalogMeta?: { deviceCount?: number; truncated?: boolean } }>()
const emit = defineEmits<{ (event: 'close'): void; (event: 'expand'): void; (event: 'refresh-catalog'): void; (event: 'scope-change', scope: any): void; (event: 'select-context', mode: string): void; (event: 'clear-selection'): void; (event: 'remove-selection', id: string): void }>()
const english = computed(() => state.locale.startsWith('en'))
const t = (zh: string, en: string) => english.value ? en : zh
const prompt = ref(''), busy = ref(false), applying = ref(false), error = ref(''), confirmation = ref('')
const proposal = ref<any>(null), candidate = ref<any>(null), baseSignature = ref('')
const previewIssues = ref<string[]>([]), previewValidated = ref(false)
const proposalApplicable = computed(() => !['structure_only', 'blocked'].includes(proposal.value?.resultStatus) && !previewIssues.value.length)
const task = ref<any>(null), canceling = ref(false)
const taskDocument = ref<any>(null)
const messages = ref<Message[]>([]), currentMessage = ref<Message | null>(null)
const historyBusy = ref(false), historyError = ref(false), hasMore = ref(false), historyLoaded = ref(false)
const sessions = ref<Conversation[]>([]), activeConversation = ref(''), sessionsBusy = ref(false), sessionsError = ref(false), sessionsLoaded = ref(false), sessionsHasMore = ref(false)
let historyBefore = 0, sessionsBefore = 0, disposed = false, runId = '', historyVersion = 0
const scopePicker = ref<'' | 'products' | 'devices'>(''), scopeSearch = ref('')
const scope = ref(assistantScope(null))
const selectedCodes = computed<string[]>(() => scope.value.deviceCodes)
const selectedProductCodes = computed<string[]>(() => scope.value.productCodes)
const selectedProducts = computed(() => (props.products || []).filter(product => selectedProductCodes.value.includes(product.code)))
const selectedDevices = computed(() => props.catalog.filter(device => selectedCodes.value.includes(device.code)))
const scopeCount = computed(() => assistantScopeCount(scope.value))
const catalogSummary = computed(() => {
  const products = new Map((scope.value.mode === 'project' ? props.products || [] : selectedProducts.value).map(product => [product.code, product]))
  for (const device of selectedDevices.value) if (!products.has(device.productCode)) products.set(device.productCode || device.code, device)
  return { fields: [...products.values()].reduce((sum, product) => sum + (product.fieldCount ?? (product.fields || []).filter((field: any) => field.role !== 'ignore').length), 0), events: [...products.values()].reduce((sum, product) => sum + (product.events || []).length, 0) }
})
const filteredDevices = computed(() => props.catalog.filter(device => `${device.name} ${device.code}`.toLowerCase().includes(scopeSearch.value.trim().toLowerCase())))
const filteredProducts = computed(() => (props.products || []).filter(product => `${product.name} ${product.code}`.toLowerCase().includes(scopeSearch.value.trim().toLowerCase())))
const contextItems = computed<ContextItem[]>(() => selectedContext(props.snapshot().content?.componentList || [], props.selectionIds))
const conversation = ref<HTMLElement>(), errorElement = ref<HTMLElement>(), promptElement = ref<HTMLTextAreaElement>(), deviceControl = ref<HTMLElement>()
const productTrigger = ref<HTMLButtonElement>(), deviceTrigger = ref<HTMLButtonElement>(), scopeDialog = ref<HTMLElement>(), scopeSearchElement = ref<HTMLInputElement>()
const editable = computed(() => state.mode === 'edit' && state.capabilities.save && !state.saving)
const locked = computed(() => busy.value || applying.value || !editable.value)
const sessionLocked = computed(() => locked.value || historyBusy.value || sessionsBusy.value)
const currentSignature = computed(() => documentSignature(props.snapshot()))
const stale = computed(() => !!proposal.value && baseSignature.value !== currentSignature.value)
const undoStack = ref<{ document: any; signature: string; messageId: string }[]>([])
const undoEntry = computed(() => undoStack.value[undoStack.value.length - 1])
const undoStale = computed(() => undoEntry.value?.signature !== currentSignature.value)
const examples = computed(() => [t('生成设备运维大屏', 'Create a device operations screen'), t('优化选中组件的布局', 'Improve the selected widgets’ layout')])
const areaLabel = (area: ContextArea) => `${area.x}, ${area.y} · ${area.w} × ${area.h}`
const widgetLabel = (key: string) => { const spec = widgetByKey.get(key); return spec ? english.value ? spec.en : spec.zh : key === 'group' ? t('分组', 'Group') : key }
const contextLabel = (item: ContextItem) => { const spec = widgetByKey.get(item.key); return !item.title || item.title === spec?.zh || item.title === spec?.en ? widgetLabel(item.key) : item.title }
const actionLabel = (type: string) => type === 'add' ? t('新增', 'Add') : type === 'update' ? t('修改', 'Update') : t('删除', 'Remove')
const stageLabel = (stage: string) => {
  const labels: Record<string, [string, string]> = { context: ['已读取所选上下文', 'Selected context read'], discovering: ['正在整理产品能力', 'Discovering product capabilities'], planning: ['正在分析需求', 'Analyzing request'], intention: ['正在分析关注目标', 'Analyzing monitoring priorities'], metrics: ['正在选择有价值的指标', 'Selecting useful metrics'], generating: ['正在匹配组件与布局', 'Matching widgets and layout'], reviewing: ['正在检查数据与视觉方案', 'Checking data and visual design'], validating: ['正在校验指标与绑定', 'Validating metrics and bindings'], completed: ['方案已生成', 'Proposal generated'], applying: ['正在应用修改', 'Applying changes'], applied: ['已应用到画布', 'Applied to canvas'], undone: ['已回退', 'Undone'] }
  return labels[stage] ? t(...labels[stage]) : ''
}
const statusLabel = (status: string) => {
  const labels: Record<string, [string, string]> = { done: ['已完成', 'Completed'], ready: ['方案待预览和应用', 'Proposal ready to review and apply'], waiting: ['等待确认', 'Awaiting confirmation'], applied: ['已应用到画布', 'Applied to canvas'], undone: ['已回退', 'Undone'], failed: ['此步骤未完成，可继续生成', 'This step is incomplete; generation can continue'], interrupted: ['正在检查上次任务状态', 'Checking previous task status'], canceled: ['生成已取消', 'Generation canceled'], discarded: ['方案已放弃', 'Proposal discarded'] }
  return labels[status] ? t(...labels[status]) : ''
}
const getWidget = (key: string) => { const spec = widgetByKey.get(key); return spec && { ...spec, target: targetOf(key), kinds: kindsOf(key) } }
async function scroll() { await nextTick(); conversation.value?.scrollTo({ top: conversation.value.scrollHeight }) }
async function showError(message: string) { error.value = message; await nextTick(); errorElement.value?.focus() }
function newMessage(role: Message['role'], content: string, extra: Partial<Message> = {}) {
  const message = reactive<Message>({ id: `m-${Date.now()}-${Math.random().toString(36).slice(2)}`, conversationId: activeConversation.value, role, content, ...extra })
  messages.value.push(message); return message
}

// Rendering: the assistant writes short paragraphs with bullet lists, so a full
// Markdown engine would be dead weight -- but a plain <p> loses the structure and
// makes a plan summary read as one blob. Blocks are parsed into data and rendered
// through the template, never as HTML, so model output cannot inject markup.
interface InlineSpan { type: 'text' | 'strong' | 'code'; text: string }
interface MessageBlock { type: 'p' | 'ul'; spans?: InlineSpan[]; items?: InlineSpan[][] }
const expandedMessages = ref(new Set<string>())
const blockCache = new Map<string, MessageBlock[]>()
const INLINE = /(\*\*[^*]+\*\*|`[^`]+`)/
const BULLET = /^\s*[-*·•]\s+(.*)$/
const isLong = (text: string) => { const value = String(text || ''); return value.length > 260 || value.split('\n').length > 9 }
function toggleExpanded(id: string) {
  const next = new Set(expandedMessages.value)
  if (next.has(id)) next.delete(id); else next.add(id)
  expandedMessages.value = next
}
// The stages panel opens while a run is in flight and collapses once it finishes,
// unless the reader toggled it themselves.
function rememberStages(message: Message, event: Event) {
  if (!message.stages?.length) return
  const open = (event.target as HTMLDetailsElement).open
  if (open !== (message.stagesOpen !== false)) message.stagesOpen = open
}
function parseInline(text: string): InlineSpan[] {
  const spans: InlineSpan[] = []
  let index = 0
  for (const match of text.matchAll(new RegExp(INLINE, 'g'))) {
    if (match.index > index) spans.push({ type: 'text', text: text.slice(index, match.index) })
    const token = match[0]
    spans.push(token.startsWith('**') ? { type: 'strong', text: token.slice(2, -2) } : { type: 'code', text: token.slice(1, -1) })
    index = match.index + token.length
  }
  if (index < text.length) spans.push({ type: 'text', text: text.slice(index) })
  return spans.length ? spans : [{ type: 'text', text }]
}
function renderBlocks(text: string): MessageBlock[] {
  const source = String(text || '')
  const cached = blockCache.get(source)
  if (cached) return cached
  const blocks: MessageBlock[] = []
  let list: InlineSpan[][] | null = null
  for (const raw of source.split(/\r?\n/)) {
    const line = raw.replace(/\s+$/, '')
    const bullet = BULLET.exec(line)
    if (bullet) { (list ||= []).push(parseInline(bullet[1])); continue }
    if (list) { blocks.push({ type: 'ul', items: list }); list = null }
    if (!line.trim()) continue
    blocks.push({ type: 'p', spans: parseInline(line.trim()) })
  }
  if (list) blocks.push({ type: 'ul', items: list })
  if (blockCache.size > 240) blockCache.clear()
  blockCache.set(source, blocks)
  return blocks
}

// Serialize writes per message, so a slow partial save cannot replace its final result.
const saveQueues = new Map<string, Promise<boolean>>(), failedSaves = new Map<string, Message>(), syncError = ref(false)
function saveMessage(message: Message) {
  const snapshot = JSON.parse(JSON.stringify(message)) as Message
  const pending = (saveQueues.get(message.id) || Promise.resolve()).then(() => rpc('assistant.message', { message: snapshot })).then(() => {
    failedSaves.delete(message.id); syncError.value = failedSaves.size > 0
    const session = sessions.value.find(item => item.id === snapshot.conversationId)
    if (session && snapshot.role === 'user' && !session.title) session.title = [...snapshot.content].slice(0, 80).join('')
    return true
  }).catch(() => { failedSaves.set(message.id, snapshot); syncError.value = true; return false })
  saveQueues.set(message.id, pending)
  pending.finally(() => { if (saveQueues.get(message.id) === pending) saveQueues.delete(message.id) })
  return pending
}
async function retrySaves() { await Promise.all([...failedSaves.values()].map(saveMessage)) }
async function flushSaves() {
  await Promise.all([...saveQueues.values()])
  if (failedSaves.size) await retrySaves()
  return failedSaves.size === 0
}
let partialSave: ReturnType<typeof setTimeout> | undefined
function schedulePartialSave() { if (!partialSave) partialSave = setTimeout(() => { partialSave = undefined; if (currentMessage.value) saveMessage(currentMessage.value) }, 1000) }
function resetConversation(id: string) {
  historyVersion++; activeConversation.value = id; messages.value = []; currentMessage.value = null
  historyBefore = 0; hasMore.value = false; historyLoaded.value = false; historyError.value = false
  proposal.value = null; candidate.value = null; undoStack.value = []; baseSignature.value = ''; runId = ''
  task.value = null; taskDocument.value = null; previewIssues.value = []; previewValidated.value = false
  prompt.value = ''; error.value = ''; confirmation.value = ''; expandedMessages.value = new Set(); blockCache.clear()
  emit('select-context', '')
}
async function createConversation() {
  const session = await rpc('assistant.conversation.create', {}) as Conversation
  if (!session || typeof session.id !== 'string' || !session.id) throw new Error('INVALID_CONVERSATION')
  sessions.value = [session, ...sessions.value.filter(item => item.id !== session.id)]
  resetConversation(session.id); historyLoaded.value = true
  return session
}
async function loadSessions(older = false) {
  if (sessionsBusy.value) return
  sessionsBusy.value = true; sessionsError.value = false
  let resume = ''
  try {
    const result = await rpc('assistant.conversations', { before: older ? sessionsBefore : undefined })
    if (disposed) return
    const items = (result.items || []) as Conversation[]
    const existing = new Set(sessions.value.map(item => item.id))
    sessions.value = older ? [...sessions.value, ...items.filter(item => !existing.has(item.id))] : [...items, ...sessions.value.filter(item => item.id === activeConversation.value && !items.some(loaded => loaded.id === item.id))]
    sessionsBefore = result.before || 0; sessionsHasMore.value = result.hasMore === true; sessionsLoaded.value = true
    if (!activeConversation.value && sessions.value.length) resume = sessions.value[0].id
    else if (!activeConversation.value) historyLoaded.value = true
  } catch { if (!disposed) sessionsError.value = true }
  finally { if (!disposed) sessionsBusy.value = false }
  if (resume && !disposed) { resetConversation(resume); await loadHistory() }
}
async function selectConversation(id: string) {
  if (!id || id === activeConversation.value || sessionLocked.value) return
  sessionsBusy.value = true
  try {
    if (!await flushSaves()) return
    resetConversation(id); await loadHistory()
  } finally { if (!disposed) sessionsBusy.value = false }
}
function onConversationChange(event: Event) {
  const target = event.target as HTMLSelectElement, id = target.value
  target.value = activeConversation.value
  selectConversation(id)
}
async function newConversation() {
  if (sessionLocked.value || !sessionsLoaded.value) return
  sessionsBusy.value = true; sessionsError.value = false
  try {
    if (!await flushSaves()) return
    await createConversation(); await nextTick(); promptElement.value?.focus()
  } catch { if (!disposed) sessionsError.value = true }
  finally { if (!disposed) sessionsBusy.value = false }
}
async function loadHistory(older = false) {
  if (historyBusy.value || !activeConversation.value) return
  const id = activeConversation.value, version = ++historyVersion
  historyBusy.value = true; historyError.value = false
  const oldHeight = conversation.value?.scrollHeight || 0
  try {
    const result = await rpc('assistant.history', { conversationId: id, before: older ? historyBefore : undefined })
    if (disposed || version !== historyVersion || id !== activeConversation.value) return
    const existing = new Set(messages.value.map(message => message.id))
    const loaded = (result.items || []).filter((message: Message) => !existing.has(message.id)).map((message: Message) => ({ ...message, conversationId: id, status: message.status === 'generating' ? 'interrupted' : message.status }))
    messages.value = [...loaded, ...messages.value]
    hasMore.value = result.hasMore === true; historyBefore = result.before || 0; historyLoaded.value = true
    await nextTick()
    if (older && conversation.value) conversation.value.scrollTop += conversation.value.scrollHeight - oldHeight
    else scroll()
  } catch { if (!disposed && version === historyVersion) historyError.value = true }
  finally { if (!disposed && version === historyVersion) historyBusy.value = false }
  if (!older && !disposed && version === historyVersion) await recoverTask()
}
function addStage(message: Message, stage: string) { if (!message.stages?.includes(stage)) (message.stages ||= []).push(stage) }
const stopProgress = on('assistant.progress', payload => {
  const message = currentMessage.value
  if (!message || payload.runId !== runId || !busy.value) return
  if (payload.event === 'task' && payload.taskId) { task.value = { ...task.value, ...payload }; message.taskId = payload.taskId }
  if (payload.event === 'stage' && ['discovering', 'planning', 'intention', 'metrics', 'generating', 'reviewing', 'validating', 'completed'].includes(payload.stage)) addStage(message, payload.stage)
  if (payload.event === 'summary' && typeof payload.content === 'string') { addStage(message, 'generating'); message.content = payload.content.slice(0, 16000) }
  schedulePartialSave(); scroll()
})
watch(() => [props.active, state.initialized], async () => {
  if (!props.active || !state.initialized) return
  if (!sessionsLoaded.value) await loadSessions()
  await nextTick(); promptElement.value?.focus()
}, { immediate: true })
watch(() => props.snapshot().content?.noyoAssistantContext, value => { scope.value = assistantScope(value) }, { immediate: true, deep: true })
function setScope(value: any) { scope.value = assistantScope(value); emit('scope-change', scope.value) }
function clearScope() { setScope({ version: 1, mode: 'selected', productCodes: [], deviceCodes: [] }) }
function useProjectScope() { setScope({ version: 1, mode: 'selected', productCodes: (props.products || []).map(product => product.code), deviceCodes: selectedCodes.value }) }
function clearPickerScope() {
  setScope({ version: 1, mode: 'selected', productCodes: scopePicker.value === 'products' ? [] : selectedProductCodes.value, deviceCodes: scopePicker.value === 'devices' ? [] : selectedCodes.value })
}
function toggleProduct(code: string, selected: boolean) {
  setScope({ version: 1, mode: 'selected', productCodes: selected ? [...selectedProductCodes.value, code] : selectedProductCodes.value.filter(value => value !== code), deviceCodes: selectedCodes.value })
}
function toggleDevice(code: string, selected: boolean) {
  setScope({ version: 1, mode: 'selected', productCodes: selectedProductCodes.value, deviceCodes: selected ? [...selectedCodes.value, code] : selectedCodes.value.filter(value => value !== code) })
}
function selectVisibleScope() {
  setScope({ version: 1, mode: 'selected', productCodes: scopePicker.value === 'products' ? [...selectedProductCodes.value, ...filteredProducts.value.map(product => product.code)] : selectedProductCodes.value, deviceCodes: scopePicker.value === 'devices' ? [...selectedCodes.value, ...filteredDevices.value.map(device => device.code)] : selectedCodes.value })
}
async function toggleScopePicker(kind: 'products' | 'devices') {
  if (scopePicker.value === kind) { closeScopePicker(); return }
  scopePicker.value = kind; scopeSearch.value = ''; await nextTick(); scopeSearchElement.value?.focus()
}
function closeScopePicker() {
  const trigger = scopePicker.value === 'products' ? productTrigger.value : deviceTrigger.value
  scopePicker.value = ''; trigger?.focus()
}
function onPickerKeydown(event: KeyboardEvent) {
  if (event.key !== 'Tab') return
  const controls = [...(scopeDialog.value?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled)') || [])]
  if (!controls.length) return
  if (event.shiftKey && document.activeElement === controls[0]) { event.preventDefault(); controls[controls.length - 1].focus() }
  else if (!event.shiftKey && document.activeElement === controls[controls.length - 1]) { event.preventDefault(); controls[0].focus() }
}
function outsideDevices(event: PointerEvent) { if (scopePicker.value && !deviceControl.value?.contains(event.target as Node)) scopePicker.value = '' }
onMounted(() => document.addEventListener('pointerdown', outsideDevices))
onUnmounted(() => { disposed = true; stopProgress(); clearTimeout(partialSave); document.removeEventListener('pointerdown', outsideDevices) })
function onPanelKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || confirmation.value) return
  event.preventDefault(); event.stopPropagation()
  if (scopePicker.value) closeScopePicker()
  else if (props.selectionMode) emit('select-context', '')
  else emit('close')
}

function taskStateLabel(value: string) {
  const labels: Record<string, [string, string]> = { queued: ['等待生成', 'Queued'], running: ['正在生成', 'Generating'], completed: ['方案已生成', 'Proposal generated'], failed: ['此步骤未完成', 'Step incomplete'], canceled: ['生成已取消', 'Generation canceled'], disconnected: ['连接中断，任务仍保留', 'Disconnected; task retained'] }
  return labels[value] ? t(...labels[value]) : ''
}
function taskErrorLabel(code: string) {
  const labels: Record<string, [string, string]> = {
    aiTimeout: ['此步骤生成超时，已完成的步骤已保存，可继续生成。', 'This step timed out. Completed steps are saved; generation can continue.'],
    aiUnavailable: ['AI 服务暂不可用，请检查模型配置后继续。', 'AI is unavailable. Check the model configuration before continuing.'],
    aiInvalid: ['此步骤的方案未通过校验，可继续生成。', 'The proposal did not pass validation. Generation can continue.'],
    aiFailed: ['此步骤生成失败，已完成的步骤已保存。', 'This step failed. Completed steps are saved.'],
    aiOutOfScope: ['请描述希望创建或修改的 IOC 大屏。', 'Describe the IOC screen you want to create or edit.']
  }
  return labels[code] ? t(...labels[code]) : t('连接或生成未完成，可重新连接检查任务状态。', 'The connection or generation is incomplete. Reconnect to check the task.')
}
function recordTaskFailure(answer: Message, code: string) {
  if (['queued', 'running'].includes(task.value?.state) || (!task.value && !['CONVERSATION_SAVE_FAILED', 'aiUnavailable', 'aiInvalid', 'aiTimeout', 'aiFailed', 'aiOutOfScope'].includes(code))) {
    task.value = { ...task.value, state: 'disconnected' }
  }
  answer.status = task.value?.state === 'canceled' ? 'canceled' : 'failed'
  answer.content = answer.status === 'canceled' ? taskStateLabel('canceled') : taskErrorLabel(code)
}
function onPreviewValidated(issues: string[]) { previewIssues.value = issues; previewValidated.value = true }
function acceptResult(result: any, document: any, answer: Message) {
  previewValidated.value = false; previewIssues.value = []
  answer.content = String(result.summary || result.clarification || '')
  if (result.resultStatus === 'blocked' || result.clarification) { answer.status = 'done'; proposal.value = null; candidate.value = null; return }
  const compiled = compileAssistantPlan(document, result, { getWidget, createComponent: createMaterial, applyTheme: applyScreenTheme, expandBlueprint, locale: state.locale })
  baseSignature.value = documentSignature(document)
  proposal.value = result; candidate.value = compiled
  answer.changeCount = compiled.changes.length; answer.status = 'ready'; addStage(answer, 'completed')
}
async function cancelTask() {
  if (!task.value?.taskId || canceling.value) return
  canceling.value = true
  try {
    task.value = await rpc('assistant.task.cancel', { taskId: task.value.taskId })
    if (task.value.state === 'canceled' && currentMessage.value) {
      currentMessage.value.status = 'canceled'; currentMessage.value.content = taskStateLabel('canceled'); await saveMessage(currentMessage.value)
    }
  }
  catch { await showError(t('取消未确认，请重新连接检查任务状态。', 'Cancellation is unconfirmed. Reconnect to check the task.')) }
  finally { canceling.value = false }
}
async function resumeTask() {
  if (!task.value?.taskId || busy.value || !taskDocument.value) return
  const document = taskDocument.value
  if (documentSignature(document) !== documentSignature(props.snapshot())) { await showError(t('画布已变化，请根据当前画布重新生成。', 'The canvas changed. Generate a new proposal for the current canvas.')); return }
  const answer = currentMessage.value || newMessage('assistant', '', { status: 'generating', taskId: task.value.taskId })
  currentMessage.value = answer; runId = answer.id; answer.status = 'generating'; busy.value = true; error.value = ''
  try { const result = await rpc('assistant.task.resume', { runId, taskId: task.value.taskId }); if (!disposed) acceptResult(result, document, answer) }
  catch (err) { if (!disposed) recordTaskFailure(answer, (err as any).code || (err as Error).message) }
  finally { if (!disposed) { busy.value = false; await saveMessage(answer); scroll() } }
}
async function recoverTask() {
  if (!activeConversation.value || busy.value || disposed) return
  const conversationId = activeConversation.value
  try {
    const retained = await rpc('assistant.task.current', { conversationId })
    if (disposed || conversationId !== activeConversation.value) return
    if (!retained?.taskId) { task.value = null; return }
    task.value = retained; taskDocument.value = retained.baseDocument
    let answer = messages.value.find(message => message.role === 'assistant' && message.taskId === retained.taskId)
    // Interrupted saves can precede the first task progress packet.
    answer ||= [...messages.value].reverse().find(message => message.role === 'assistant' && message.status === 'interrupted')
    if (answer?.status === 'applied' || answer?.status === 'undone' || answer?.status === 'discarded') return
    answer ||= newMessage('assistant', '', { status: 'generating', taskId: retained.taskId })
    answer.taskId = retained.taskId; currentMessage.value = answer; runId = answer.id
    if (retained.state === 'completed') { acceptResult(retained.result, retained.baseDocument || props.snapshot(), answer); await saveMessage(answer); return }
    if (['failed', 'canceled'].includes(retained.state)) { answer.status = retained.state; answer.content = retained.state === 'canceled' ? taskStateLabel('canceled') : taskErrorLabel(retained.errorKey); await saveMessage(answer); return }
    await resumeTask()
  } catch { if (!disposed) { if (task.value) task.value.state = 'disconnected'; error.value = t('暂时无法读取任务，可重新连接。', 'Task status is unavailable. Reconnect to try again.') } }
}

async function generate() {
  if (!prompt.value.trim() || locked.value || historyBusy.value || sessionsBusy.value || !sessionsLoaded.value || historyError.value) return
  const input = prompt.value.trim(), document = props.snapshot()
  const context = { items: contextItems.value.map(item => ({ ...item })), area: props.selectionArea ? { ...props.selectionArea } : null, devices: selectedDevices.value.map(device => ({ code: device.code, name: device.name || '' })), products: selectedProducts.value.map(product => ({ code: product.code, name: product.name || '' })), scope: assistantScope(scope.value) }
  busy.value = true; error.value = ''; scopePicker.value = ''; proposal.value = null; candidate.value = null; task.value = null; previewValidated.value = false; previewIssues.value = []; baseSignature.value = documentSignature(document)
  try {
    if (!await flushSaves()) throw new Error('CONVERSATION_SAVE_FAILED')
    if (!activeConversation.value) await createConversation()
  } catch {
    busy.value = false
    await showError(t('对话尚未保存，请重试保存后再生成。', 'The conversation could not be saved. Retry saving before generating.'))
    return
  }
  baseSignature.value = documentSignature(document)
  taskDocument.value = document
  const user = newMessage('user', input, { context, status: 'done' })
  const answer = newMessage('assistant', '', { status: 'generating', stages: ['context'] })
  currentMessage.value = answer; runId = answer.id
  prompt.value = ''; emit('select-context', ''); scroll()
  try {
    if (!await saveMessage(user) || !await saveMessage(answer)) throw new Error('CONVERSATION_SAVE_FAILED')
    const result = await rpc('assistant.plan', {
      runId, requestId: runId, conversationId: activeConversation.value, prompt: input, document, selectionIds: context.items.map(item => item.id), selectionArea: context.area, ...assistantScopeRequest(context.scope),
      widgets: materials.map(spec => ({ key: spec.key, zh: spec.zh, en: spec.en, shape: spec.shape, target: targetOf(spec.key) || '', kinds: kindsOf(spec.key), optionFields: Object.keys(new spec.ctor().option || {}) }))
    })
    if (disposed) return
    acceptResult(result, document, answer)
  } catch (err) {
    if (!disposed) {
      prompt.value = input; candidate.value = null; proposal.value = null; recordTaskFailure(answer, (err as any).code || (err as Error).message)
      // A long request was restored into a textarea that had been scrolled to the
      // end by the user; showing the head of it is more useful than the tail.
      await nextTick()
      if (promptElement.value) promptElement.value.scrollTop = 0
      const code = (err as any).code || (err as Error).message
      answer.content = answer.status === 'canceled' ? taskStateLabel('canceled') : code === 'CONVERSATION_SAVE_FAILED'
        ? t('对话尚未保存，请重试保存后再生成。', 'The conversation could not be saved. Retry saving before generating.')
        : code === 'aiOutOfScope'
          ? t('IOC AI 助手仅处理 IOC 大屏的设计、生成和编辑。请描述大屏布局、组件、样式或设备数据展示需求。', 'The IOC AI assistant only handles IOC screen design, generation and editing. Describe the screen layout, widgets, styling or device data to display.')
        : code === 'INVALID_PLAN'
        ? t('方案校验未通过，可从此步骤继续生成。', 'Proposal validation failed. Continue generation from this step.')
        : code === 'LAYOUT_CAPACITY'
          ? t('自动编排未能适配当前画布，可重新生成以调整指标密度与布局。', 'The automatic layout could not fit the canvas. Regenerate to adjust metric density and layout.')
        : ['INVALID_METRIC_SCOPE', 'INVALID_METRIC_KIND'].includes(code)
          ? t('指标范围与组件能力不匹配，请重新生成方案。', 'The metric scope does not match the widget capability. Generate a new proposal.')
        : code === 'LAYOUT_OVERLAP'
          ? t('自动编排检测到组件重叠，可重新生成以调整布局。', 'The automatic layout detected overlapping components. Regenerate to adjust the layout.')
          : taskErrorLabel(code)
    }
  } finally {
    clearTimeout(partialSave); partialSave = undefined
    if (!disposed) { await saveMessage(answer); busy.value = false; scroll() }
  }
  // A new proposal remains detached for visual review before it changes the canvas.
}
function requestApply() {
  if (!candidate.value || !previewValidated.value || !proposalApplicable.value || !editable.value || busy.value || applying.value || sessionsBusy.value || historyBusy.value) return
  if (stale.value) { showError(t('画布已变化，请重新生成方案。', 'The canvas changed. Generate a new proposal.')); return }
  if (candidate.value.destructive) { confirmation.value = 'apply'; if (currentMessage.value) { currentMessage.value.status = 'waiting'; saveMessage(currentMessage.value) } }
  else apply()
}
function discardProposal() { proposal.value = null; candidate.value = null; if (currentMessage.value) { currentMessage.value.status = 'discarded'; saveMessage(currentMessage.value) } }
async function confirmAction() { const action = confirmation.value; confirmation.value = ''; if (action === 'undo') await undo(); else if (action === 'apply') await apply() }
async function apply() {
  if (!candidate.value || !previewValidated.value || !proposalApplicable.value || stale.value || !editable.value || busy.value || applying.value || sessionsBusy.value || historyBusy.value) return
  applying.value = true; error.value = ''
  const previous = props.snapshot(), expected = currentSignature.value, next = candidate.value.document, answer = currentMessage.value
  try {
    if (answer) addStage(answer, 'validating')
    await rpc('screen.validate', { document: next })
    if (disposed) return
    if (expected !== documentSignature(props.snapshot())) throw new Error('STALE_PLAN')
    if (answer) addStage(answer, 'applying')
    await props.applyDocument(next)
    undoStack.value.push({ document: previous, signature: documentSignature(props.snapshot()), messageId: answer?.id || '' })
    if (undoStack.value.length > 10) undoStack.value.shift()
    proposal.value = null; candidate.value = null
    if (answer) { answer.status = 'applied'; addStage(answer, 'applied'); await saveMessage(answer) }
  } catch (err) {
    if (!disposed) { if (answer) { answer.status = 'failed'; await saveMessage(answer) }; await showError((err as Error).message === 'STALE_PLAN' ? t('画布已变化，请重新生成方案。', 'The canvas changed. Generate a new proposal.') : (err as Error).message) }
  } finally { if (!disposed) { applying.value = false; scroll() } }
}
async function undo() {
  if (!undoEntry.value || undoStale.value || !editable.value || busy.value || applying.value || sessionsBusy.value || historyBusy.value) return
  applying.value = true; error.value = ''
  const entry = undoEntry.value
  try {
    await rpc('screen.validate', { document: entry.document })
    if (disposed) return
    if (entry.signature !== documentSignature(props.snapshot())) throw new Error('STALE_PLAN')
    await props.applyDocument(entry.document)
    undoStack.value.pop(); proposal.value = null; candidate.value = null
    const answer = messages.value.find(message => message.id === entry.messageId)
    if (answer) { answer.status = 'undone'; addStage(answer, 'undone'); await saveMessage(answer) }
  } catch (err) { if (!disposed) await showError((err as Error).message === 'STALE_PLAN' ? t('画布已变化，无法回退。', 'The canvas changed; undo is unavailable.') : (err as Error).message) }
  finally { if (!disposed) applying.value = false }
}
</script>

<style scoped>
.ioc-assistant{display:flex;flex-direction:column;min-width:0;min-height:0;height:100%;background:var(--bg-surface);border-left:1px solid var(--border-color);overflow:hidden}
.ioc-assistant-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:16px 24px;flex-shrink:0;border-bottom:1px solid var(--border-color)}.ioc-assistant-heading strong{font-size:16px}.ioc-assistant-actions{display:flex;gap:8px;flex-shrink:0}
.ioc-assistant-sessions{display:flex;align-items:flex-end;flex-wrap:wrap;gap:8px;padding:16px 24px 8px;flex-shrink:0}.ioc-assistant-session-select{display:flex;flex:1;flex-direction:column;gap:8px;min-width:160px;color:var(--text-secondary);font-size:12px}.ioc-assistant-session-select select{width:100%;min-height:32px;padding:4px 8px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-primary);font:inherit;text-overflow:ellipsis}.ioc-assistant-session-select select:focus-visible{outline:2px solid var(--primary-color);outline-offset:2px}.ioc-assistant-sessions>p{flex-basis:100%;margin:0}.ioc-assistant-purpose{margin:0;padding:8px 24px 16px;border-bottom:1px solid var(--border-color);font-size:12px;line-height:1.5;color:var(--text-secondary)}
.ioc-assistant-conversation{flex:1;min-height:0;overflow:auto;padding:24px;overflow-wrap:anywhere;overscroll-behavior:contain}
/* The empty state is centred in the panel instead of sitting under a 96px void:
   the old top padding read as a rendering gap above "what would you like to
   create". Centring keeps the same breathing room on both sides. */
.ioc-assistant-conversation.is-empty{display:flex;flex-direction:column}
/* Top-aligned on purpose: centring it left a 250px void above the heading. */
.ioc-assistant-welcome{display:flex;flex-direction:column;gap:24px;width:100%;max-width:520px;margin:0;padding:0 0 16px}
.ioc-assistant-welcome-intro{display:flex;align-items:flex-start;gap:16px}
.ioc-assistant-welcome-mark{flex:none;display:inline-flex;align-items:center;justify-content:center;width:36px;height:36px;border-radius:12px;background:color-mix(in srgb,var(--primary-color) 12%,var(--bg-body));border:1px solid color-mix(in srgb,var(--primary-color) 28%,var(--border-color));color:var(--primary-color)}
.ioc-assistant-welcome-mark svg{width:20px;height:20px}
.ioc-assistant-welcome h2{font-size:17px;line-height:1.4;margin:0}
.ioc-assistant-welcome-intro p{margin:8px 0 0;font-size:13px;line-height:1.7;color:var(--text-secondary)}
.ioc-assistant-examples{display:flex;flex-direction:column;gap:8px}
.ioc-assistant .ioc-assistant-examples button{display:flex;align-items:center;justify-content:space-between;gap:12px;width:100%;margin:0;padding:12px 16px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-body);color:var(--text-primary);font-size:13px;text-align:left;white-space:normal}
.ioc-assistant .ioc-assistant-examples button:hover{border-color:var(--primary-color);background:color-mix(in srgb,var(--primary-color) 6%,var(--bg-body))}
.ioc-assistant-example-arrow{flex:none;width:16px;height:16px;color:var(--text-secondary)}
.ioc-assistant .ioc-assistant-examples button:hover .ioc-assistant-example-arrow{color:var(--primary-color)}
.ioc-assistant-message{display:flex;align-items:flex-start;gap:8px;margin-bottom:16px}.ioc-assistant-message.user{flex-direction:row-reverse}.ioc-assistant-avatar{flex:none;width:28px;height:28px;padding:5px;border-radius:50%;background:var(--bg-body);border:1px solid var(--border-color);color:var(--text-secondary)}.ioc-assistant-message.user .ioc-assistant-avatar{background:color-mix(in srgb,var(--primary-color) 14%,var(--bg-body));border-color:color-mix(in srgb,var(--primary-color) 40%,var(--border-color));color:var(--primary-color)}.ioc-assistant-bubble{display:flex;flex-direction:column;gap:8px;max-width:calc(100% - 44px);min-width:0;padding:12px 16px;border:1px solid var(--border-color);border-radius:12px;background:color-mix(in srgb,var(--text-primary) 4%,var(--bg-surface))}.ioc-assistant-message.user .ioc-assistant-bubble{background:color-mix(in srgb,var(--primary-color) 10%,var(--bg-surface));border-color:color-mix(in srgb,var(--primary-color) 30%,var(--border-color))}.ioc-assistant-richtext{font-size:13px;line-height:1.7;color:var(--text-primary);overflow-wrap:anywhere}.ioc-assistant-richtext p{margin:0 0 8px}.ioc-assistant-richtext p:last-child,.ioc-assistant-richtext ul:last-child{margin-bottom:0}.ioc-assistant-richtext ul{margin:0 0 8px;padding-left:20px}.ioc-assistant-richtext li{margin:2px 0}.ioc-assistant-richtext code{padding:1px 4px;border-radius:4px;background:var(--bg-body);font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}.ioc-assistant-richtext.is-collapsed{max-height:168px;overflow:hidden;-webkit-mask-image:linear-gradient(180deg,#000 80%,transparent);mask-image:linear-gradient(180deg,#000 80%,transparent)}.ioc-assistant-more{align-self:flex-start;margin:0!important;padding:4px 8px;font-size:12px}.ioc-assistant-message-context{display:flex;flex-wrap:wrap;gap:8px;font-size:12px;color:var(--text-secondary)}.ioc-assistant-message-context>span{padding:8px;background:var(--bg-body);border-radius:8px}.ioc-assistant-message-context details{flex-basis:100%;max-height:160px;overflow:auto}
.ioc-assistant-status{color:var(--text-secondary);font-size:12px}.ioc-assistant-progress{padding:8px 12px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-secondary);font-size:12px}.ioc-assistant-progress summary{display:flex;align-items:center;gap:8px;cursor:pointer;list-style:none;color:var(--text-secondary)}.ioc-assistant-progress summary::-webkit-details-marker{display:none}.ioc-assistant-progress summary::before{content:"";flex:none;width:6px;height:6px;border-radius:50%;background:var(--primary-color)}.ioc-assistant-stage-current{color:var(--text-primary)}.ioc-assistant-stage-count{margin-left:auto;font-variant-numeric:tabular-nums;opacity:.75}.ioc-assistant-dots{color:var(--primary-color)}.ioc-assistant-progress ol{margin:8px 0 0;padding-left:20px}.ioc-assistant-progress li{margin-top:4px}.ioc-assistant-progress li.is-done{color:var(--text-primary)}summary{cursor:pointer}
.ioc-assistant-proposal{padding:16px;border:1px solid var(--primary-color);border-radius:8px}.ioc-assistant-proposal ol{padding-left:24px}.ioc-assistant-proposal button{margin:8px 8px 0 0}.ioc-assistant-warning{color:var(--warning-color)}.ioc-assistant-error,.ioc-assistant-sync{padding:8px 24px;max-height:128px;overflow:auto;font-size:12px}.ioc-assistant-error{color:var(--danger-color)}.ioc-assistant-undo{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;padding:8px 24px;font-size:12px;color:var(--text-secondary)}
.ioc-assistant-compose{position:relative;flex-shrink:0;padding:16px 24px 24px;border-top:1px solid var(--border-color)}.ioc-assistant-input-label{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}.ioc-assistant-compose textarea,.ioc-assistant-device-picker input[type=search]{box-sizing:border-box;width:100%;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-primary);font:inherit;padding:8px}.ioc-assistant-compose textarea{display:block;resize:vertical;min-height:96px;max-height:192px}.ioc-assistant-compose-footer,.ioc-assistant-tools{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.ioc-assistant-compose-footer{justify-content:space-between;margin-top:8px}.ioc-assistant-selection-chips{display:flex;gap:8px;flex-wrap:wrap;max-height:96px;overflow:auto;margin-bottom:8px;font-size:12px}.ioc-assistant-selection-chips>span{display:inline-flex;align-items:center;gap:8px;padding:8px;background:var(--bg-body);border:1px solid var(--border-color);border-radius:8px;max-width:100%;overflow-wrap:anywhere}.ioc-assistant-selection-chips button{padding:0 8px}.ioc-assistant-selection-hint{font-size:12px;color:var(--text-secondary);margin:0 0 8px}.ioc-assistant-selection-hint button{margin-left:8px}
.ioc-assistant-device-picker{position:absolute;bottom:72px;left:24px;right:24px;z-index:10;padding:16px;max-height:min(480px,65vh);overflow:auto;background:var(--bg-body);isolation:isolate;border:1px solid var(--border-color);border-radius:8px;box-shadow:0 16px 48px color-mix(in srgb,var(--text-primary) 16%,transparent)}.ioc-assistant-scope{display:flex;flex-wrap:wrap;gap:8px 16px;margin:8px 0;padding:8px 16px;border-radius:8px;background:var(--bg-body);color:var(--text-secondary);font-size:12px;line-height:1.6}.ioc-assistant-scope strong{color:var(--text-primary);font-weight:500}.ioc-assistant-scope.is-thin{color:var(--warning-color)}.ioc-assistant-hint{margin:0 0 8px;font-size:12px;line-height:1.6;color:var(--warning-color)}.ioc-assistant-picker-heading,.ioc-assistant-picker-actions{display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;margin-bottom:8px}.ioc-assistant-picker-actions{margin-top:8px}.ioc-assistant-device-options{max-height:240px;overflow:auto}.ioc-assistant-device-options label{display:flex;align-items:center;gap:8px;padding:8px;cursor:pointer;border-radius:8px}.ioc-assistant-device-options label:hover{background:var(--bg-body)}.ioc-assistant-device-options small{display:block;color:var(--text-secondary);margin-top:8px}.ioc-assistant-device-options input{accent-color:var(--primary-color)}
.ioc-assistant button{border:1px solid var(--border-color);background:var(--bg-body);color:var(--text-primary);border-radius:8px;padding:8px;font:inherit;font-size:12px;cursor:pointer;transition:background var(--transition-fast),border-color var(--transition-fast)}.ioc-assistant button:hover,.ioc-assistant button[aria-pressed=true]{border-color:var(--primary-color)}.ioc-assistant button:disabled{opacity:.5;cursor:not-allowed}.ioc-assistant button.ioc-assistant-primary{background:var(--primary-color);border-color:var(--primary-color);color:var(--bg-body)}.ioc-assistant :is(button,input,textarea,summary):focus-visible{outline:2px solid var(--primary-color);outline-offset:2px}
@media(prefers-reduced-motion:reduce){.ioc-assistant button{transition:none}}
</style>
