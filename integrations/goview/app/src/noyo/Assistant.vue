<template>
  <aside ref="root" class="ioc-assistant" :class="{ 'is-expanded': expanded }" :style="panelStyle" :aria-label="t('IOC AI 助手', 'IOC AI assistant')" @keydown="onPanelKeydown">
    <div class="ioc-assistant-resize is-left" role="separator" aria-orientation="vertical" :aria-label="t('调整宽度', 'Resize width')" :aria-valuenow="rect?.w" :aria-valuemin="MIN_WIDTH" tabindex="0" @pointerdown="startResize($event, 'left')" @keydown="onResizeKeydown" />
    <div class="ioc-assistant-resize is-right" role="separator" aria-orientation="vertical" :aria-label="t('调整宽度', 'Resize width')" :aria-valuenow="rect?.w" :aria-valuemin="MIN_WIDTH" tabindex="0" @pointerdown="startResize($event, 'right')" @keydown="onResizeKeydown" />
    <div class="ioc-assistant-resize is-bottom" role="separator" aria-orientation="horizontal" :aria-label="t('调整高度', 'Resize height')" :aria-valuenow="rect?.h" :aria-valuemin="MIN_HEIGHT" tabindex="0" @pointerdown="startResize($event, 'bottom')" @keydown="onResizeKeydown" />
    <div class="ioc-assistant-resize is-corner" role="button" :aria-label="t('调整大小', 'Resize panel')" tabindex="0" @pointerdown="startResize($event, 'corner')" @keydown="onResizeKeydown" />
    <header class="ioc-assistant-heading" :title="t('拖动移动，双击最大化', 'Drag to move, double-click to maximise')" @pointerdown="startDrag" @dblclick="onHeadingDoubleClick">
      <svg class="ioc-assistant-grip" viewBox="0 0 10 16" aria-hidden="true"><g fill="currentColor"><circle cx="3" cy="4" r="1.1" /><circle cx="7" cy="4" r="1.1" /><circle cx="3" cy="8" r="1.1" /><circle cx="7" cy="8" r="1.1" /><circle cx="3" cy="12" r="1.1" /><circle cx="7" cy="12" r="1.1" /></g></svg>
      <span class="ioc-assistant-heading-avatar" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="m12 3 2.2 6.8L21 12l-6.8 2.2L12 21l-2.2-6.8L3 12l6.8-2.2Z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round" /></svg></span>
      <div class="ioc-assistant-heading-title"><strong>{{ t('IOC AI 助手', 'IOC AI assistant') }}</strong><span><i aria-hidden="true" />{{ t('大屏设计与编辑', 'Screen design and editing') }}</span></div>
      <div class="ioc-assistant-actions">
        <button type="button" :disabled="sessionLocked || !sessionsLoaded" :aria-label="t('新开对话', 'New conversation')" :title="t('新开对话', 'New conversation')" @click="newConversation"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5v14M5 12h14" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" /></svg></button>
        <button ref="historyToggle" type="button" :aria-label="historyOpen ? t('隐藏历史对话', 'Hide conversation history') : t('显示历史对话', 'Show conversation history')" :title="historyOpen ? t('隐藏历史对话', 'Hide conversation history') : t('显示历史对话', 'Show conversation history')" :aria-expanded="historyOpen" :aria-pressed="historyOpen" aria-controls="ioc-assistant-history" @click="historyOpen = !historyOpen"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 10a8 8 0 1 1 2 8M4 4v6h6m2-2v5l3 2" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg></button>
        <button type="button" :aria-label="t('复位', 'Reset')" :title="t('放回默认位置', 'Back to the default corner')" @click="resetPosition"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 10a8 8 0 1 1 2 8M4 4v6h6" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg></button>
        <button type="button" :aria-pressed="!!expanded" :aria-label="expanded ? t('还原大小', 'Restore size') : t('最大化', 'Maximise')" :title="expanded ? t('还原大小', 'Restore size') : t('最大化', 'Maximise')" @click="$emit('expand')"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="expanded ? 'M4 9h5V4m11 11h-5v5M9 9 3 3m12 12 6 6' : 'M9 4H4v5m11 11h5v-5M4 4l6 6m10 10-6-6'" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" /></svg></button>
        <button type="button" class="ioc-assistant-close" :aria-label="t('关闭助手', 'Close assistant')" :title="t('关闭助手', 'Close assistant')" @click="$emit('close')"><svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" /></svg></button>
      </div>
    </header>
    <div class="ioc-assistant-layout">
      <aside v-show="historyOpen" id="ioc-assistant-history" class="ioc-assistant-history" :aria-label="t('历史对话', 'Conversation history')" :aria-busy="sessionsBusy" @keydown.esc.stop="hideHistory">
        <label class="ioc-assistant-history-search"><svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="10.5" cy="10.5" r="6.5" fill="none" stroke="currentColor" stroke-width="1.6" /><path d="m16 16 4 4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" /></svg><input v-model.trim="historySearch" type="search" :aria-label="t('搜索历史对话', 'Search conversation history')" :placeholder="t('搜索历史对话', 'Search conversations')" /></label>
        <nav class="ioc-assistant-history-list" :aria-label="t('历史对话列表', 'Conversation list')">
          <button v-for="session in filteredSessions" :key="session.id" type="button" class="ioc-assistant-history-session" :class="{ 'is-active': session.id === activeConversation }" :aria-current="session.id === activeConversation ? 'page' : undefined" :disabled="sessionLocked || !sessionsLoaded" @click="selectConversation(session.id)">
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 4h14a1 1 0 0 1 1 1v12H9l-5 4V5a1 1 0 0 1 1-1Z" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round" /><path d="M8 8h8M8 12h5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" /></svg>
            <span><strong :title="conversationTitle(session)">{{ conversationTitle(session) }}</strong><time v-if="conversationTime(session)" :datetime="session.updatedAt || session.createdAt">{{ conversationTime(session) }}</time></span>
          </button>
          <p v-if="!filteredSessions.length && sessionsLoaded && !sessionsBusy" class="ioc-assistant-history-empty">{{ historySearch ? t('没有匹配的历史对话', 'No matching conversations') : t('暂无历史对话', 'No conversation history yet') }}</p>
          <button v-if="sessionsHasMore" type="button" class="ioc-assistant-history-more" :disabled="sessionLocked" @click="loadSessions(true)">{{ t('更多对话', 'More conversations') }}</button>
        </nav>
      </aside>
      <div class="ioc-assistant-main">
        <div v-if="sessionsBusy || sessionsError" class="ioc-assistant-sessions" :aria-busy="sessionsBusy">
          <span v-if="sessionsBusy" class="ioc-assistant-spinner" role="status" :aria-label="t('正在读取对话…', 'Loading conversations…')" />
          <p v-if="sessionsError" class="ioc-assistant-warning" role="alert">{{ t('对话操作失败，原有记录已保留。', 'Conversation action failed. Existing history is preserved.') }} <button type="button" :disabled="sessionLocked" @click="loadSessions()">{{ t('重试', 'Retry') }}</button></p>
        </div>
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
            <p class="ioc-assistant-welcome-note">{{ t('仅用于 IOC 大屏的设计、生成和编辑；对话自动保存。', 'For IOC screen design, generation and editing only. Conversations save automatically.') }}</p>
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
          <AssistantProgress v-if="message.stages?.length || message.steps?.length" :steps="progressFor(message)" :locale="state.locale" :waiting="message.status === 'waiting'" />
          <div v-if="message.content" class="ioc-assistant-richtext" :class="{ 'is-collapsed': isLong(message.content) && !expandedMessages.has(message.id) }">
            <template v-for="(block, index) in renderBlocks(message.content)" :key="index">
              <p v-if="block.type === 'p'"><template v-for="(span, at) in block.spans" :key="at"><strong v-if="span.type === 'strong'">{{ span.text }}</strong><code v-else-if="span.type === 'code'">{{ span.text }}</code><template v-else>{{ span.text }}</template></template></p>
              <ul v-else-if="block.type === 'ul'"><li v-for="(item, at) in block.items" :key="at"><template v-for="(span, i2) in item" :key="i2"><strong v-if="span.type === 'strong'">{{ span.text }}</strong><code v-else-if="span.type === 'code'">{{ span.text }}</code><template v-else>{{ span.text }}</template></template></li></ul>
            </template>
          </div>
          <button v-if="isLong(message.content)" type="button" class="ioc-assistant-more" @click="toggleExpanded(message.id)">{{ expandedMessages.has(message.id) ? t('收起', 'Show less') : t('展开全文', 'Show more') }}</button>
          <small v-if="message.status && message.status !== 'generating' && message.role === 'assistant'" class="ioc-assistant-status">{{ message === currentMessage && candidate ? lifecycleLabel : statusLabel(message.status) }}<template v-if="message.changeCount"> · {{ message.changeCount }} {{ t('项变更', 'changes') }}</template></small>
        </div>
      </article>
      <section v-if="proposal && candidate" class="ioc-assistant-proposal" :aria-label="t('待应用方案', 'Pending proposal')">
        <AssistantPreview :document="candidate.document" :baseline="baseDocument" :identity="validationIdentity" :recheck="recheckVersion" :highlighted="highlightedComponent" :locale="state.locale" @report="onPreviewValidated" @invalidated="onPreviewInvalidated" />
        <ul v-if="proposal.limitations?.length" class="ioc-assistant-limitations"><li v-for="limitation in proposal.limitations" :key="limitation">{{ limitation }}</li></ul>
        <p v-if="!previewIssues.length && previewQuality?.coverage === 'partial'" class="ioc-assistant-warning" role="status">{{ t('部分组件暂无可验证数据，应用后会显示对应的数据状态。仅缺少当前数据不会阻止应用。', 'Some widgets have no data available to validate and will show their data status after applying. Missing current data alone does not prevent applying.') }}</p>
        <p v-if="repairNotice" role="status">{{ repairNotice }}</p>
        <ol v-if="previewQuality?.issues.length" class="ioc-assistant-validation" :aria-label="t('校验问题', 'Validation issues')">
          <li v-for="issue in previewQuality.issues" :key="`${issue.code}:${issue.componentId}`" :class="{ 'is-blocking': issue.severity === 'error' }">
            <strong>{{ componentTitle(issue.componentId) }} · {{ issueLabel(issue) }}</strong>
            <span>{{ issueEvidence(issue) }}</span>
            <small v-if="issue.origin === 'baseline'">{{ t('当前画布已有，方案未改变此组件；不阻止应用。', 'Already present on the canvas; this component is unchanged and does not block applying.') }}</small>
            <div class="ioc-assistant-issue-actions">
              <button type="button" @click="highlightedComponent = issue.componentId">{{ t('定位预览', 'Locate in preview') }}</button>
              <template v-if="issue.severity === 'error'">
                <button v-if="issue.code === 'text_overflow'" type="button" :disabled="repairing || applying || stale" @click="repairProposal(false)">{{ t('修复换行并校验', 'Repair line breaks and check') }}</button>
                <button v-if="issue.code === 'text_overflow'" type="button" :disabled="repairing || applying || stale" @click="repairProposal(true)">{{ t('允许增高并校验', 'Allow more height and check') }}</button>
                <button type="button" :disabled="repairing || applying || stale" @click="revertIssueComponent(issue.componentId)">{{ t('撤销此组件的方案变更', 'Revert this component’s proposal changes') }}</button>
              </template>
            </div>
          </li>
        </ol>
        <button v-if="previewValidated && !applying" type="button" :disabled="repairing || stale" @click="recheckProposal">{{ t('重新取样并校验', 'Refresh samples and check') }}</button>
        <button v-if="repairing" type="button" @click="cancelRepair">{{ t('停止自动修复', 'Stop automatic repair') }}</button>
        <details><summary>{{ t('查看变更', 'Review changes') }} · {{ candidate.changes.length }}</summary>
          <ol><li v-for="change in candidate.changes" :key="change.id">{{ actionLabel(change.type) }} · {{ change.title || widgetLabel(change.key) }}</li></ol>
        </details>
        <p v-if="applyDisabledReason" id="ioc-assistant-apply-reason" class="ioc-assistant-warning" :role="previewIssues.length ? 'alert' : 'status'">{{ applyDisabledReason }}</p>
        <button type="button" :disabled="!!applyDisabledReason" :aria-describedby="applyDisabledReason ? 'ioc-assistant-apply-reason' : undefined" @click="requestApply">{{ t('应用方案', 'Apply proposal') }}</button>
        <button type="button" :disabled="applying" @click="discardProposal">{{ t('放弃方案', 'Discard proposal') }}</button>
      </section>
    </div>
    <div v-if="task" class="ioc-assistant-task" role="status">
      <span>{{ taskStateLabel(task.state) }}<template v-if="task.stage && !['completed', 'canceled'].includes(task.state)"> · {{ stageLabel(task.stage) }}</template></span>
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
      <p v-if="selectionMode" class="ioc-assistant-selection-hint" role="status">{{ selectionMode === 'area' ? t('在画布上拖出区域', 'Drag an area on the canvas') : t('点击组件以添加或移除', 'Click widgets to add or remove them') }}</p>
      <label for="ioc-assistant-prompt" class="ioc-assistant-input-label">{{ t('你的需求', 'Your request') }}</label>
      <textarea id="ioc-assistant-prompt" ref="promptElement" v-model="prompt" rows="2" maxlength="4000" :disabled="locked || !sessionsLoaded || historyBusy" :placeholder="currentMessage?.status === 'waiting' ? t('补充上述信息，继续生成方案…', 'Reply with the requested information to continue…') : t('描述 IOC 大屏的设计、生成或编辑需求…', 'Describe an IOC screen design, generation or editing request…')" @keydown.ctrl.enter.prevent="generate" @keydown.meta.enter.prevent="generate" />
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
        <button type="submit" class="ioc-assistant-primary" :disabled="!prompt.trim() || locked || historyBusy || sessionsBusy || !sessionsLoaded || historyError">{{ busy ? t('生成中…', 'Generating…') : applying ? t('应用中…', 'Applying…') : currentMessage?.status === 'waiting' ? t('补充并继续', 'Reply and continue') : t('生成方案', 'Generate proposal') }}</button>
      </div>
    </form>
      </div>
    </div>
    <n-modal :show="!!confirmation" :closable="false" preset="dialog" :title="confirmation === 'undo' ? t('回退 AI 修改', 'Undo AI change') : t('确认移除内容', 'Confirm removal')" :content="confirmation === 'undo' ? t('恢复到上次 AI 应用前的画布及数据绑定？', 'Restore the canvas and bindings from before the last AI change?') : t('此方案会删除组件或解除数据绑定。确认应用？', 'This proposal removes widgets or data bindings. Apply it?')" :positive-text="t('确认', 'Confirm')" :negative-text="t('取消', 'Cancel')" @update:show="value => { if (!value) confirmation = '' }" @positive-click="confirmAction" @negative-click="confirmation = ''" />
  </aside>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { createMaterial, materials } from './materials'
import { kindsOf, targetOf, widgetByKey } from './widgets'
import { assistantDocumentSignature as documentSignature, on, rpc, state } from './bridge'
import { compileAssistantPlan, inferAssistantIntent } from './assistantPlan.js'
import { applyScreenTheme } from './screenTheme'
import { expandBlueprint } from './layout'
import { selectedContext } from './assistantContext.js'
import { assistantScope, assistantScopeCount, assistantScopeRequest } from './assistantScope.js'
import AssistantPreview from './AssistantPreview.vue'
import { proposalDisposition, reportMatches, repairTextOverflow, revertComponent, flattenComponents, type ProposalIssue, type ProposalValidation, type ValidationIdentity } from './proposalSession'
import AssistantProgress from './AssistantProgress.vue'
import { assistantProgress, type AssistantStep } from './assistantProgress'

type ContextArea = { x: number; y: number; w: number; h: number }
type ContextItem = { id: string; key: string; title: string }
type Message = { id: string; conversationId?: string; role: 'user' | 'assistant'; content: string; status?: string; steps?: AssistantStep[]; stages?: string[]; stagesOpen?: boolean; changeCount?: number; taskId?: string; context?: { items: ContextItem[]; area: ContextArea | null; devices: { code: string; name: string }[]; products?: { code: string; name: string }[]; scope?: any } }
type Conversation = { id: string; title: string; createdAt: string; updatedAt: string }
const props = defineProps<{ active: boolean; expanded?: boolean; inspectorOpen?: boolean; catalogLoading?: boolean; catalogError?: string; snapshot: () => any; applyDocument: (document: any, acceptedVersion?: number) => Promise<void>; settleSaves?: () => Promise<void>; beginApply?: () => () => void; selectionIds: string[]; selectionArea?: ContextArea | null; selectionMode?: string; catalog: any[]; products?: any[]; catalogMeta?: { deviceCount?: number; truncated?: boolean } }>()
const emit = defineEmits<{ (event: 'close'): void; (event: 'expand'): void; (event: 'refresh-catalog'): void; (event: 'scope-change', scope: any): void; (event: 'select-context', mode: string): void; (event: 'clear-selection'): void; (event: 'remove-selection', id: string): void }>()
const english = computed(() => state.locale.startsWith('en'))
const t = (zh: string, en: string) => english.value ? en : zh
const prompt = ref(''), busy = ref(false), applying = ref(false), error = ref(''), confirmation = ref('')
const proposal = ref<any>(null), candidate = ref<any>(null), baseSignature = ref('')
const previewIssues = ref<string[]>([]), previewValidated = ref(false), previewQuality = ref<ProposalValidation>()
const baseDocument = ref<any>(null), candidateRevision = ref(0), recheckVersion = ref(0), highlightedComponent = ref('')
const repairing = ref(false), repairNotice = ref(''), repairAttempts = ref(0), autoRepairStopped = ref(false)
const validationIdentity = computed<ValidationIdentity>(() => ({ candidateRevision: candidateRevision.value, candidateDigest: candidate.value ? documentSignature(candidate.value.document) : '', environmentDigest: state.locale }))
let repairDeadline = 0, applyKey = ''
const proposalApplicable = computed(() => !['structure_only', 'blocked'].includes(proposal.value?.resultStatus) && !previewIssues.value.length)
const task = ref<any>(null), canceling = ref(false)
const taskDocument = ref<any>(null)
const messages = ref<Message[]>([]), currentMessage = ref<Message | null>(null)
const historyBusy = ref(false), historyError = ref(false), hasMore = ref(false), historyLoaded = ref(false)
const sessions = ref<Conversation[]>([]), activeConversation = ref(''), sessionsBusy = ref(false), sessionsError = ref(false), sessionsLoaded = ref(false), sessionsHasMore = ref(false)
const HISTORY_VISIBILITY_KEY = 'noyo.assistant.history.visible'
function savedHistoryVisibility() { try { return localStorage.getItem(HISTORY_VISIBILITY_KEY) !== 'false' } catch { return true } }
const historyOpen = ref(savedHistoryVisibility()), historySearch = ref(''), historyToggle = ref<HTMLButtonElement>()
watch(historyOpen, open => { try { localStorage.setItem(HISTORY_VISIBILITY_KEY, String(open)) } catch { /* Optional preference. */ } })
const conversationTitle = (session: Conversation) => session.title || (session.id === 'legacy' ? t('以前的对话', 'Earlier conversation') : t('新对话', 'New conversation'))
const filteredSessions = computed(() => {
  const search = historySearch.value.toLocaleLowerCase()
  return search ? sessions.value.filter(session => conversationTitle(session).toLocaleLowerCase().includes(search)) : sessions.value
})
function conversationTime(session: Conversation) {
  const date = new Date(session.updatedAt || session.createdAt)
  return Number.isFinite(date.getTime()) ? new Intl.DateTimeFormat(english.value ? 'en' : 'zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date) : ''
}
async function hideHistory() { historyOpen.value = false; await nextTick(); historyToggle.value?.focus() }
let historyBefore = 0, sessionsBefore = 0, disposed = false, runId = '', historyVersion = 0
let titlePoll: ReturnType<typeof setTimeout> | undefined
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
const conversation = ref<HTMLElement>(), errorElement = ref<HTMLElement>(), promptElement = ref<HTMLTextAreaElement>(), deviceControl = ref<HTMLElement>(), root = ref<HTMLElement>()
const productTrigger = ref<HTMLButtonElement>(), deviceTrigger = ref<HTMLButtonElement>(), scopeDialog = ref<HTMLElement>(), scopeSearchElement = ref<HTMLInputElement>()
// Panel geometry. The assistant is a free-floating window over the canvas: drag
// the title bar to move it, drag an edge or the bottom-right corner to resize.
// A docked column used to cost the canvas 448-560px permanently; more importantly
// the panel and the inspector no longer exclude each other, because a floating
// panel can simply be dragged aside instead of being closed.
const MIN_WIDTH = 360, MIN_HEIGHT = 300, DEFAULT_WIDTH = 744
const EDGE = 8
// v2: the coordinate space changed from workspace-relative to viewport-relative
// when the panel became position:fixed, so the old value would be misread.
const RECT_STORAGE_KEY = 'noyo.assistant.rect.v2'
type PanelRect = { x: number; y: number; w: number; h: number }
type ResizeEdge = 'left' | 'right' | 'bottom' | 'corner'
// Seeded synchronously, not in onMounted: the panel is position:fixed with
// inset:auto, so a null rect on the first paint would park it at its static
// position inside the workspace for one frame.
const restoredRect = savedRect()
const rect = ref<PanelRect | null>(restoredRect || defaultRect())
// Once the user positions the panel we stop moving it for them -- an automatic
// dodge that overrides a deliberate placement is worse than an overlap.
let positionChosen = !!restoredRect
function savedRect(): PanelRect | null {
  try {
    const parsed = JSON.parse(localStorage.getItem(RECT_STORAGE_KEY) || 'null')
    if (!parsed || ![parsed.x, parsed.y, parsed.w, parsed.h].every((value: unknown) => Number.isFinite(value))) return null
    return { x: Math.round(parsed.x), y: Math.round(parsed.y), w: Math.round(parsed.w), h: Math.round(parsed.h) }
  } catch { return null }
}
function saveRect() {
  try { if (rect.value) localStorage.setItem(RECT_STORAGE_KEY, JSON.stringify(rect.value)) } catch { /* storage can be unavailable */ }
}
function workspace(): HTMLElement | null { return (root.value?.parentElement as HTMLElement) || null }
// The panel is fixed to the editor viewport, not to the workspace grid: the user
// must be able to park it over the toolbar, the component palette or the status
// strip, i.e. anywhere on the page. Workspace-relative bounds capped it inside
// the canvas area, which is exactly the region the screenshot flagged.
function available() { return { width: window.innerWidth, height: window.innerHeight } }
function between(value: number, min: number, max: number) { return Math.min(Math.max(value, min), Math.max(min, max)) }
function fitSize(w: number, h: number) {
  const area = available()
  return {
    w: Math.round(between(w, MIN_WIDTH, area.width ? area.width - EDGE * 2 : 880)),
    h: Math.round(between(h, MIN_HEIGHT, area.height ? area.height - EDGE * 2 : 900)),
  }
}
// No clamping while moving: the panel can be parked over any part of the page,
// including flush against an edge or hanging past it. The only guard is on load
// and on window resize -- if the stored rect ended up completely outside the
// viewport (smaller window, different monitor) it is pulled back to the default
// corner, so the panel is never unreachable after a reload.
function placeRect(x: number, y: number, w: number, h: number): PanelRect {
  return { x: Math.round(x), y: Math.round(y), w, h }
}
// Overlap of the rect with the viewport, in pixels, on each axis.
function visibleSpan(r: PanelRect) {
  const area = available()
  return {
    w: Math.min(r.x + r.w, area.width) - Math.max(r.x, 0),
    h: Math.min(r.y + r.h, area.height) - Math.max(r.y, 0),
  }
}
const RECOVER_MIN = 48
function recoverable(r: PanelRect) { const span = visibleSpan(r); return span.w >= RECOVER_MIN && span.h >= RECOVER_MIN }
function defaultRect(): PanelRect {
  const area = available()
  const size = fitSize(DEFAULT_WIDTH, area.height ? area.height - EDGE * 2 : 640)
  return placeRect(area.width ? area.width - size.w - EDGE : EDGE, EDGE, size.w, size.h)
}
function currentRect(): PanelRect { return rect.value || defaultRect() }
function applyRect(next: { x: number; y: number; w: number; h: number }, persist = true) {
  const size = fitSize(next.w, next.h)
  rect.value = placeRect(next.x, next.y, size.w, size.h)
  if (persist) saveRect()
}
const panelStyle = computed(() => {
  if (props.expanded || !rect.value) return {}
  return { left: `${rect.value.x}px`, top: `${rect.value.y}px`, width: `${rect.value.w}px`, height: `${rect.value.h}px` }
})
let dragOrigin: { px: number; py: number; rect: PanelRect } | null = null
let dragMoved = false
function startDrag(event: PointerEvent) {
  if (props.expanded || event.button !== 0) return
  if ((event.target as HTMLElement).closest('button, a, input, select, textarea, [role="separator"]')) return
  event.preventDefault()
  dragOrigin = { px: event.clientX, py: event.clientY, rect: currentRect() }
  dragMoved = false
  document.body.classList.add('ioc-assistant-moving')
  document.addEventListener('pointermove', moveDrag)
  document.addEventListener('pointerup', stopDrag, { once: true })
  document.addEventListener('pointercancel', stopDrag, { once: true })
}
function moveDrag(event: PointerEvent) {
  if (!dragOrigin) return
  const dx = event.clientX - dragOrigin.px, dy = event.clientY - dragOrigin.py
  if (Math.abs(dx) + Math.abs(dy) > 2) dragMoved = true
  applyRect({ ...dragOrigin.rect, x: dragOrigin.rect.x + dx, y: dragOrigin.rect.y + dy }, false)
}
function stopDrag() {
  if (!dragOrigin) return
  dragOrigin = null
  document.removeEventListener('pointermove', moveDrag)
  document.body.classList.remove('ioc-assistant-moving')
  if (!dragMoved) return
  positionChosen = true
  saveRect()
}
let resizeOrigin: { px: number; py: number; rect: PanelRect; edge: ResizeEdge } | null = null
function startResize(event: PointerEvent, edge: ResizeEdge) {
  if (props.expanded || event.button !== 0) return
  event.preventDefault(); event.stopPropagation()
  resizeOrigin = { px: event.clientX, py: event.clientY, rect: currentRect(), edge }
  document.body.classList.add('ioc-assistant-resizing')
  document.body.style.cursor = edge === 'bottom' ? 'row-resize' : edge === 'corner' ? 'nwse-resize' : 'col-resize'
  document.addEventListener('pointermove', moveResize)
  document.addEventListener('pointerup', stopResize, { once: true })
  document.addEventListener('pointercancel', stopResize, { once: true })
}
function moveResize(event: PointerEvent) {
  const origin = resizeOrigin
  if (!origin) return
  const dx = event.clientX - origin.px, dy = event.clientY - origin.py
  const horizontal = origin.edge === 'left' || origin.edge === 'right' || origin.edge === 'corner'
  // Resizing never pushes against a page edge: the size is only bounded so the
  // panel cannot become larger than the viewport itself.
  const size = fitSize(horizontal ? origin.rect.w + (origin.edge === 'left' ? -dx : dx) : origin.rect.w, origin.edge === 'bottom' || origin.edge === 'corner' ? origin.rect.h + dy : origin.rect.h)
  // The left edge keeps the right edge anchored, so the origin follows the width.
  applyRect({ x: origin.edge === 'left' ? origin.rect.x + origin.rect.w - size.w : origin.rect.x, y: origin.rect.y, ...size }, false)
}
function stopResize() {
  if (!resizeOrigin) return
  resizeOrigin = null
  document.removeEventListener('pointermove', moveResize)
  document.body.classList.remove('ioc-assistant-resizing')
  document.body.style.cursor = ''
  positionChosen = true
  saveRect()
}
function onResizeKeydown(event: KeyboardEvent) {
  const deltas: Record<string, [number, number]> = { ArrowLeft: [-40, 0], ArrowRight: [40, 0], ArrowUp: [0, -40], ArrowDown: [0, 40] }
  const delta = deltas[event.key]
  if (!delta) return
  event.preventDefault(); event.stopPropagation()
  const current = currentRect()
  const size = fitSize(current.w + delta[0], current.h + delta[1])
  applyRect({ x: delta[0] < 0 ? current.x + current.w - size.w : current.x, y: current.y, ...size })
  positionChosen = true
}
// Escape hatch for a panel dragged out of reach: back to the default corner, and
// the inspector dodge is re-armed because the placement is no longer the user's.
function resetPosition() {
  rect.value = defaultRect()
  positionChosen = false
  saveRect()
  nextTick(avoidInspector)
}
function onHeadingDoubleClick(event: MouseEvent) { if ((event.target as HTMLElement).closest('button, [role="separator"]')) return; emit('expand') }
function onWindowResize() {
  if (!rect.value) return
  if (recoverable(rect.value)) { applyRect({ ...rect.value }, false); return }
  resetPosition()
}
// The inspector opens on the right when a widget is selected. Until the user has
// chosen a position, slide the panel just clear of it; afterwards the overlap is
// theirs to resolve by dragging.
function avoidInspector() {
  if (!props.inspectorOpen || props.expanded || positionChosen || !rect.value || !root.value) return
  const parent = workspace()
  const inspector = parent?.querySelector('.inspector') as HTMLElement | null
  if (!parent || !inspector || !inspector.getClientRects().length) return
  const panel = root.value.getBoundingClientRect(), pane = inspector.getBoundingClientRect()
  const overlaps = panel.left < pane.right && panel.right > pane.left && panel.top < pane.bottom && panel.bottom > pane.top
  if (!overlaps) return
  applyRect({ ...rect.value, x: rect.value.x - (panel.right - pane.left) - EDGE }, false)
}

const editable = computed(() => state.mode === 'edit' && state.capabilities.save && !state.saving)
const locked = computed(() => busy.value || applying.value || !editable.value)
const sessionLocked = computed(() => locked.value || historyBusy.value || sessionsBusy.value)
const currentSignature = computed(() => documentSignature(props.snapshot()))
const stale = computed(() => !!proposal.value && baseSignature.value !== currentSignature.value)
const disposition = computed(() => proposalDisposition({ candidate: !!candidate.value, awaitingInput: currentMessage.value?.status === 'waiting', generating: busy.value, applying: applying.value, repairing: repairing.value, conflict: stale.value, writable: state.mode === 'edit' && state.capabilities.save, blocked: ['structure_only', 'blocked'].includes(proposal.value?.resultStatus), validated: previewValidated.value && !!previewQuality.value && reportMatches(previewQuality.value, validationIdentity.value), issues: previewQuality.value?.issues, occupied: historyBusy.value || sessionsBusy.value, saving: state.saving }))
const lifecycleLabel = computed(() => {
  const labels: Record<string, [string, string]> = { idle: ['本轮处理已结束', 'Request finished'], generating: ['正在生成方案', 'Generating proposal'], awaiting_input: ['等待补充信息', 'Awaiting information'], validating: ['正在校验方案', 'Checking proposal'], repairing: ['正在修复并重新校验', 'Repairing and checking again'], ready: ['方案可应用', 'Proposal ready to apply'], ready_with_warnings: ['方案可应用，有提示项', 'Ready to apply with advisories'], needs_fix: ['方案待修复', 'Proposal needs fixes'], conflict: ['画布已修改，需重新生成', 'Canvas edited; generate again'], applying: ['正在应用方案', 'Applying proposal'] }
  return t(...(labels[disposition.value.state] || labels.idle))
})
const applyDisabledReason = computed(() => {
  const labels: Record<string, [string, string]> = {
    conflict: ['方案依据的画布或数据绑定已变化，请重新生成。', 'The canvas or data bindings used by this proposal have changed. Generate again.'],
    saving: ['正在自动保存画布，完成后可应用。', 'The canvas is being saved. Apply after saving finishes.'],
    permission: ['当前大屏不可编辑，请检查编辑权限。', 'This screen cannot be edited. Check your edit permissions.'],
    occupied: ['正在处理当前操作，完成后可应用。', 'An operation is in progress. Apply after it finishes.'],
    validating: ['正在校验方案预览，校验完成后可应用。', 'The proposal preview is being checked. Apply after validation finishes.'],
    repairing: ['正在修复并重新校验，完成后可应用。', 'Repairing and checking again. Apply when checks finish.'],
    applying: ['正在提交方案。', 'Committing proposal.'], generating: ['正在生成方案。', 'Generating proposal.']
  }
  if (disposition.value.reason === 'needs_fix') return proposal.value?.resultStatus === 'structure_only' ? t('当前是布局草稿，所需数据能力尚未接通，完善后可应用。', 'This is a layout draft. Connect the required data capabilities before applying.') : t(`还有 ${previewIssues.value.length} 项阻止应用的问题，请处理上方具体原因。`, `${previewIssues.value.length} blocking issues remain. Resolve the specific causes above.`)
  return labels[disposition.value.reason] ? t(...labels[disposition.value.reason]) : ''
})
const undoStack = ref<{ document: any; signature: string; messageId: string }[]>([])
const undoEntry = computed(() => undoStack.value[undoStack.value.length - 1])
const undoStale = computed(() => undoEntry.value?.signature !== currentSignature.value)
const examples = computed(() => [t('生成设备运维大屏', 'Create a device operations screen'), t('优化选中组件的布局', 'Improve the selected widgets’ layout')])
const areaLabel = (area: ContextArea) => `${area.x}, ${area.y} · ${area.w} × ${area.h}`
const widgetLabel = (key: string) => { const spec = widgetByKey.get(key); return spec ? english.value ? spec.en : spec.zh : key === 'group' ? t('分组', 'Group') : key }
const contextLabel = (item: ContextItem) => { const spec = widgetByKey.get(item.key); return !item.title || item.title === spec?.zh || item.title === spec?.en ? widgetLabel(item.key) : item.title }
const actionLabel = (type: string) => type === 'add' ? t('新增', 'Add') : type === 'update' ? t('修改', 'Update') : t('删除', 'Remove')
const stageLabel = (stage: string) => {
  const labels: Record<string, [string, string]> = { context: ['已读取所选上下文', 'Selected context read'], discovering: ['正在整理产品能力', 'Discovering product capabilities'], planning: ['正在分析需求', 'Analyzing request'], intention: ['正在分析关注目标', 'Analyzing monitoring priorities'], metrics: ['正在选择有价值的指标', 'Selecting useful metrics'], generating: ['正在匹配组件与布局', 'Matching widgets and layout'], reviewing: ['正在检查数据与视觉方案', 'Checking data and visual design'], checking: ['正在检查数据能力', 'Checking data capabilities'], validating: ['正在校验指标与绑定', 'Validating metrics and bindings'], completed: ['方案已生成', 'Proposal generated'], applying: ['正在应用修改', 'Applying changes'], applied: ['已应用到画布', 'Applied to canvas'], undone: ['已回退', 'Undone'] }
  return labels[stage] ? t(...labels[stage]) : ''
}
const statusLabel = (status: string) => {
  const labels: Record<string, [string, string]> = { done: ['已完成', 'Completed'], ready: ['方案待预览和应用', 'Proposal ready to review and apply'], waiting: ['等待补充信息', 'Awaiting information'], applied: ['已应用到画布', 'Applied to canvas'], undone: ['已回退', 'Undone'], failed: ['此步骤未完成，可继续生成', 'This step is incomplete; generation can continue'], interrupted: ['正在检查上次任务状态', 'Checking previous task status'], canceled: ['生成已取消', 'Generation canceled'], discarded: ['方案已放弃', 'Proposal discarded'] }
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
// The server titles a conversation with the model right after the first proposal
// lands (see api_screen_assistant_title.go). Until then the list shows a
// truncated placeholder, so poll briefly instead of opening a second channel:
// the title is cosmetic and must never delay or fail the transcript.
function watchTitle(previous: string) {
  clearTimeout(titlePoll)
  const id = activeConversation.value
  if (!id) return
  let attempt = 0
  const tick = async () => {
    if (disposed || id !== activeConversation.value) return
    try {
      const result = await rpc('assistant.conversations', {})
      if (disposed || id !== activeConversation.value) return
      const items = (result.items || []) as Conversation[]
      const known = new Set(items.map(item => item.id))
      sessions.value = [...items.map(item => ({ ...(sessions.value.find(cached => cached.id === item.id) || item), title: item.title })), ...sessions.value.filter(item => !known.has(item.id))]
      if (items.find(item => item.id === id)?.title !== previous) return
    } catch { /* a title refresh failure is not worth reporting */ }
    if (++attempt < 5) titlePoll = setTimeout(tick, 1500)
  }
  titlePoll = setTimeout(tick, 1500)
}
async function selectConversation(id: string) {
  if (!id || id === activeConversation.value || sessionLocked.value) return
  sessionsBusy.value = true
  try {
    if (!await flushSaves()) return
    resetConversation(id); await loadHistory()
    if (!historyError.value && root.value && root.value.clientWidth < 600) historyOpen.value = false
  } finally { if (!disposed) sessionsBusy.value = false }
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
  if (payload.event === 'task' && payload.taskId) { task.value = { ...task.value, ...payload }; message.taskId = payload.taskId; if (Array.isArray(payload.steps)) message.steps = payload.steps }
  if (payload.event === 'stage' && ['discovering', 'planning', 'intention', 'metrics', 'checking', 'generating', 'reviewing', 'validating', 'completed'].includes(payload.stage)) addStage(message, payload.stage)
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
onMounted(() => {
  // Seeded in setup, so this only handles a rect restored from a larger window:
  // if it is now entirely out of reach the panel goes back to the default corner
  // (not persisted -- a first-visit placement must still be free to dodge).
  if (!rect.value || !recoverable(rect.value)) { rect.value = defaultRect(); positionChosen = false }
  else applyRect({ ...rect.value }, false)
  nextTick(avoidInspector)
  document.addEventListener('pointerdown', outsideDevices)
  window.addEventListener('resize', onWindowResize)
})
// Opening the panel after the inspector is already up must also dodge it, and so
// must a late inspector (the layout settles a frame later).
watch(() => props.active, open => { if (open) nextTick(avoidInspector) })
watch(() => props.inspectorOpen, () => nextTick(avoidInspector))
onUnmounted(() => { disposed = true; stopProgress(); clearTimeout(partialSave); clearTimeout(titlePoll); document.removeEventListener('pointerdown', outsideDevices); document.removeEventListener('pointermove', moveResize); document.removeEventListener('pointermove', moveDrag); window.removeEventListener('resize', onWindowResize) })
function onPanelKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape' || confirmation.value) return
  event.preventDefault(); event.stopPropagation()
  if (scopePicker.value) closeScopePicker()
  else if (props.selectionMode) emit('select-context', '')
  else emit('close')
}

function taskStateLabel(value: string) {
  if (value === 'completed') {
    const status = currentMessage.value?.status
    if (['waiting', 'applied', 'undone', 'discarded'].includes(status || '')) return statusLabel(status!)
    if (task.value?.result?.clarification || task.value?.result?.resultStatus === 'blocked') return statusLabel('waiting')
    if (proposal.value && candidate.value) return lifecycleLabel.value
    return t('本轮处理已结束', 'Request finished')
  }
  const labels: Record<string, [string, string]> = { queued: ['等待生成', 'Queued'], running: ['正在生成', 'Generating'], failed: ['此步骤未完成', 'Step incomplete'], canceled: ['生成已取消', 'Generation canceled'], disconnected: ['连接中断，任务仍保留', 'Disconnected; task retained'] }
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
function progressFor(message: Message) {
  return assistantProgress(message, candidate.value && message === currentMessage.value ? { validated: previewValidated.value, issues: previewIssues.value } : undefined)
}
function onPreviewInvalidated(identity: ValidationIdentity) {
  if (reportMatches(identity, validationIdentity.value)) previewValidated.value = false
}
function onPreviewValidated(report: ProposalValidation) {
  if (!candidate.value || !reportMatches(report, validationIdentity.value)) return
  if (repairing.value) repairNotice.value = report.issues.some(issue => issue.severity === 'error')
    ? t('修复已完成，仍有以下问题需要处理。', 'Repair finished; the following issues still need attention.')
    : t('已完成修复与校验，方案可应用。', 'Repair and validation finished. The proposal is ready to apply.')
  previewQuality.value = report; previewIssues.value = report.issues.filter(issue => issue.severity === 'error').map(issue => `${issue.code}:${issue.componentId}`); previewValidated.value = true; repairing.value = false
  if (currentMessage.value) { currentMessage.value.steps = progressFor(currentMessage.value); schedulePartialSave() }
  if (previewIssues.value.length && !autoRepairStopped.value && repairAttempts.value < 2 && Date.now() < repairDeadline) repairProposal(false, true)
}
function replaceCandidate(document: any) {
  candidate.value = { ...candidate.value, document }; ++candidateRevision.value; previewValidated.value = false; previewQuality.value = undefined; previewIssues.value = []; applyKey = ''
  if (currentMessage.value) currentMessage.value.changeCount = candidate.value.changes.length
}
function cancelRepair() { autoRepairStopped.value = true; repairing.value = false; repairNotice.value = t('自动修复已停止；仍可逐项处理问题。', 'Automatic repair stopped. You can still resolve individual issues.') }
function repairProposal(allowLayout = false, automatic = false) {
  if (!candidate.value || applying.value || stale.value || !previewQuality.value) return
  if (automatic && (repairAttempts.value >= 2 || Date.now() >= repairDeadline || autoRepairStopped.value)) return
  if (!automatic) { autoRepairStopped.value = true; repairDeadline = Date.now() + 10000 }
  ++repairAttempts.value
  const repaired = repairTextOverflow(candidate.value.document, previewQuality.value.issues, { allowLayout })
  if (!repaired.changed) { repairing.value = false; repairNotice.value = t('在当前约束内无法安全自动修复。可撤销此组件的方案变更，或调整内容后重新生成。', 'No safe automatic repair is available within these constraints. Revert this component’s proposal changes or revise the content and generate again.'); return }
  repairing.value = true
  repairNotice.value = allowLayout ? t('已按允许范围修复说明文本或高度，正在重新校验。', 'Repaired the note text or height within the allowed scope; checking again.') : t('已移除不必要的硬换行，文字、字号、绑定和布局保持原意，正在重新校验。', 'Removed unnecessary hard line breaks without changing wording, font size, bindings or layout; checking again.')
  replaceCandidate(repaired.document)
}
function recheckProposal() { if (candidate.value && !applying.value && !stale.value) { previewValidated.value = false; ++recheckVersion.value } }
function revertIssueComponent(id: string) {
  if (!candidate.value || applying.value || stale.value) return
  const document = revertComponent(candidate.value.document, baseDocument.value, id)
  if (!document) { repairNotice.value = t('此组件是本次新增内容，请调整需求后重新生成。', 'This component was added by this proposal. Revise the request and generate again.'); return }
  candidate.value = { ...candidate.value, changes: candidate.value.changes.filter((change: any) => change.id !== id) }
  autoRepairStopped.value = true; repairing.value = true; replaceCandidate(document)
  repairNotice.value = t('已撤销此组件的方案变更，正在重新校验其余变更。', 'Reverted this component’s proposal changes; checking the remaining changes.')
}
function componentTitle(id: string) { if (id === 'proposal') return t('方案', 'Proposal'); const node = flattenComponents(candidate.value?.document).find(item => item.id === id); return node?.chartConfig?.title || node?.title || id }
function issueLabel(issue: ProposalIssue) {
  const labels: Record<string, [string, string]> = { text_overflow: ['文字超出组件边界', 'Text exceeds component bounds'], chart_text_overflow: ['图表文字超出边界', 'Chart text exceeds bounds'], render_failed: ['组件渲染失败', 'Widget failed to render'], render_missing: ['组件未正常显示', 'Widget is not visible'], chart_empty: ['图表没有渲染内容', 'Chart has no rendered content'], bounds: ['组件超出画布', 'Widget exceeds canvas bounds'], data_error: ['数据查询失败或无权访问', 'Data query failed or access denied'], data_waiting: ['等待可用数据', 'Awaiting available data'], data_unchecked: ['数据尚未验证', 'Data is not checked'] }
  return t(...(labels[issue.code] || ['校验问题', 'Validation issue']))
}
function issueEvidence(issue: ProposalIssue) {
  const expected: any = issue.expected, actual: any = issue.actual
  if (issue.code === 'text_overflow') return t(`可用 ${expected.width} × ${expected.height}px；内容至少需要 ${actual.requiredWidth ?? actual.width} × ${actual.requiredHeight ?? actual.height}px。`, `Available ${expected.width} × ${expected.height}px; content needs at least ${actual.requiredWidth ?? actual.width} × ${actual.requiredHeight ?? actual.height}px.`)
  if (issue.code === 'data_unchecked') return t('本次预览未能验证此数据源；应用后仍会显示真实数据状态。', 'This preview could not check the source; the applied widget will show its actual data status.')
  if (issue.code === 'data_waiting') return t('绑定保留，缺少当前样本不会阻止应用。', 'The binding is preserved. Missing current samples do not block applying.')
  return issue.code === 'data_error' ? t('请检查数据源、字段和读取权限，再重新取样校验。', 'Check the source, fields and read permissions, then refresh samples.') : t('请定位组件，调整或撤销此组件的方案变更后重新校验。', 'Locate the widget, adjust or revert its proposal changes, then check again.')
}
function acceptResult(result: any, document: any, answer: Message) {
  previewValidated.value = false; previewIssues.value = []; previewQuality.value = undefined
  baseDocument.value = JSON.parse(JSON.stringify(document)); repairAttempts.value = 0; autoRepairStopped.value = false; repairing.value = false; repairNotice.value = ''; highlightedComponent.value = ''; repairDeadline = Date.now() + 15000
  if (task.value) task.value = { ...task.value, state: 'completed', stage: 'completed', result }
  answer.content = String(result.summary || result.clarification || '')
  if (messages.value.filter(message => message.role === 'user').length <= 1) watchTitle(sessions.value.find(item => item.id === activeConversation.value)?.title || '')
  if (result.resultStatus === 'blocked' || result.clarification) { answer.status = 'waiting'; answer.steps = assistantProgress(answer); proposal.value = null; candidate.value = null; return }
  const input = [...messages.value].reverse().find(message => message.role === 'user')?.content || ''
  const compiled = compileAssistantPlan(document, result, { getWidget, createComponent: createMaterial, applyTheme: applyScreenTheme, expandBlueprint, locale: state.locale, intent: inferAssistantIntent(input, result) })
  baseSignature.value = documentSignature(document)
  proposal.value = result; candidate.value = compiled; ++candidateRevision.value; applyKey = ''
  answer.changeCount = compiled.changes.length; answer.status = 'ready'; addStage(answer, 'completed')
  answer.steps = assistantProgress({ ...answer, steps: undefined }).filter(step => step.id !== 'preview').map(step => ({ ...answer.steps?.find(existing => existing.id === step.id), ...step }))
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
    answer.taskId = retained.taskId; answer.steps = retained.steps; currentMessage.value = answer; runId = answer.id
    if (retained.state === 'completed') { baseDocument.value = retained.baseDocument; if (await recoverCommitted()) return; acceptResult(retained.result, retained.baseDocument || props.snapshot(), answer); await saveMessage(answer); return }
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
  if (!disposition.value.canApply) return
  if (candidate.value.destructive) confirmation.value = 'apply'
  else apply()
}
function discardProposal() { proposal.value = null; candidate.value = null; if (currentMessage.value) { currentMessage.value.status = 'discarded'; saveMessage(currentMessage.value) } }
const commitStorageKey = () => `noyo.assistant.commit.${task.value?.taskId || currentMessage.value?.id || ''}`
function rememberCommit(key: string) { try { if (key) localStorage.setItem(commitStorageKey(), key); else localStorage.removeItem(commitStorageKey()) } catch {} }
async function recoverCommitted(heldBarrier = false) {
  let key = applyKey
  try { key ||= localStorage.getItem(commitStorageKey()) || '' } catch {}
  if (!key) return false
  try {
    const receipt = await rpc('screen.apply.status', { idempotencyKey: key })
    if (!receipt.committed || disposed) return false
    if (receipt.currentDraftVersion === receipt.draftVersion) {
      const previous = baseDocument.value || taskDocument.value
      const release = heldBarrier ? undefined : props.beginApply?.()
      try {
        const current = documentSignature(props.snapshot()), accepted = documentSignature(receipt.document)
        if (heldBarrier || current === accepted || previous && current === documentSignature(previous)) {
          await props.applyDocument(receipt.document, receipt.draftVersion)
          if (previous && !undoStack.value.some(entry => entry.messageId === currentMessage.value?.id)) undoStack.value.push({ document: previous, signature: documentSignature(props.snapshot()), messageId: currentMessage.value?.id || '' })
        } else await showError(t('方案此前已应用；保留了当前尚未保存的画布修改。', 'The proposal was applied previously. Current unsaved canvas edits have been preserved.'))
      } finally { release?.() }
    }
    if (currentMessage.value) { currentMessage.value.status = 'applied'; addStage(currentMessage.value, 'applied'); await saveMessage(currentMessage.value) }
    candidate.value = null; proposal.value = null; rememberCommit('')
    if (receipt.currentDraftVersion !== receipt.draftVersion) repairNotice.value = t('方案此前已应用，之后画布又有修改。', 'The proposal was applied previously and the canvas was edited afterwards.')
    return true
  } catch { return false }
}
async function confirmAction() { const action = confirmation.value; confirmation.value = ''; if (action === 'undo') await undo(); else if (action === 'apply') await apply() }
async function apply() {
  if (!disposition.value.canApply) return
  applying.value = true; error.value = ''
  const previous = props.snapshot(), expected = currentSignature.value, next = candidate.value.document, answer = currentMessage.value
  let release: (() => void) | undefined
  try {
    release = props.beginApply?.()
    if (answer) addStage(answer, 'validating')
    await props.settleSaves?.()
    if (disposed) return
    if (expected !== documentSignature(props.snapshot())) throw new Error('STALE_PLAN')
    if (!state.capabilities.atomicApply) throw new Error(t('编辑器版本不匹配，请刷新页面后重新校验。', 'Editor versions do not match. Refresh the page and check again.'))
    if (answer) addStage(answer, 'applying')
    if (!applyKey) applyKey = `apply-${answer?.id || Date.now()}-${candidateRevision.value}-${Math.random().toString(36).slice(2)}`
    rememberCommit(applyKey)
    const accepted = await rpc('screen.apply', { document: next, baseDocument: previous, baseDraftVersion: state.revision, idempotencyKey: applyKey })
    if (disposed) return
    await props.applyDocument(accepted.document, accepted.draftVersion)
    undoStack.value.push({ document: previous, signature: documentSignature(props.snapshot()), messageId: answer?.id || '' })
    if (undoStack.value.length > 10) undoStack.value.shift()
    proposal.value = null; candidate.value = null
    if (answer) { answer.status = 'applied'; addStage(answer, 'applied'); await saveMessage(answer) }
    rememberCommit('')
  } catch (err) {
    if (!disposed) {
      if (applyKey && await recoverCommitted(true)) return
      await showError((err as Error).message === 'STALE_PLAN' ? t('画布已变化，请重新生成方案。', 'The canvas changed. Generate a new proposal.') : (err as Error).message)
    }
  } finally { release?.(); if (!disposed) { applying.value = false; scroll() } }
}
async function undo() {
  if (!undoEntry.value || undoStale.value || !editable.value || busy.value || applying.value || sessionsBusy.value || historyBusy.value) return
  applying.value = true; error.value = ''
  const entry = undoEntry.value
  let release: (() => void) | undefined
  try {
    release = props.beginApply?.()
    await props.settleSaves?.()
    if (disposed) return
    if (entry.signature !== documentSignature(props.snapshot())) throw new Error('STALE_PLAN')
    const accepted = await rpc('screen.apply', { document: entry.document, baseDocument: props.snapshot(), baseDraftVersion: state.revision, idempotencyKey: `undo-${entry.messageId}-${entry.signature.length}` })
    await props.applyDocument(accepted.document, accepted.draftVersion)
    undoStack.value.pop(); proposal.value = null; candidate.value = null
    const answer = messages.value.find(message => message.id === entry.messageId)
    if (answer) { answer.status = 'undone'; addStage(answer, 'undone'); await saveMessage(answer) }
  } catch (err) { if (!disposed) await showError((err as Error).message === 'STALE_PLAN' ? t('画布已变化，无法回退。', 'The canvas changed; undo is unavailable.') : (err as Error).message) }
  finally { release?.(); if (!disposed) applying.value = false }
}
</script>

<style scoped>
/* Geometry (position:fixed, left/top/width/height) is owned by style.css plus the
   inline style, so the panel can be parked anywhere in the editor viewport. This
   block only owns the internal column layout. */
.ioc-assistant{position:relative;display:flex;flex-direction:column;min-width:0;min-height:0;height:100%;background:var(--noyo-float-bg,var(--bg-surface));overflow:hidden;border-radius:12px}
.ioc-assistant-validation{display:flex;flex-direction:column;gap:16px;padding:16px 24px;border:1px solid var(--border-color);border-radius:8px}.ioc-assistant-validation li{padding:8px}.ioc-assistant-validation strong,.ioc-assistant-validation span,.ioc-assistant-validation small{display:block;margin-bottom:8px}.ioc-assistant-validation .is-blocking{border-inline-start:3px solid var(--warning-color,var(--primary-color))}.ioc-assistant-issue-actions{display:flex;flex-wrap:wrap;gap:8px}
/* Four affordances: both vertical edges, the bottom edge, and the corner. A free
   floating window needs them; a docked column only ever needed its inner edge. */
.ioc-assistant-resize{position:absolute;z-index:6;touch-action:none;background:transparent;transition:background-color var(--transition-fast)}
.ioc-assistant-resize.is-left,.ioc-assistant-resize.is-right{inset-block:12px;width:8px;cursor:col-resize}
.ioc-assistant-resize.is-left{inset-inline-start:0}
.ioc-assistant-resize.is-right{inset-inline-end:0}
.ioc-assistant-resize.is-bottom{inset-block-end:0;inset-inline:12px;height:8px;cursor:row-resize}
.ioc-assistant-resize.is-corner{inset-block-end:0;inset-inline-end:0;width:16px;height:16px;cursor:nwse-resize;border-end-end-radius:12px;background-image:linear-gradient(135deg,transparent 52%,var(--border-color) 52%,var(--border-color) 59%,transparent 59%),linear-gradient(135deg,transparent 71%,var(--border-color) 71%,var(--border-color) 78%,transparent 78%)}
.ioc-assistant-resize:hover,.ioc-assistant-resize:focus-visible{background-color:color-mix(in srgb,var(--primary-color) 20%,transparent)}
.ioc-assistant-heading{display:flex;align-items:center;gap:10px;padding:0 16px;min-height:48px;flex-shrink:0;border-bottom:1px solid var(--border-color);cursor:move;touch-action:none;user-select:none}
.ioc-assistant-grip{flex:none;width:10px;height:16px;color:var(--text-secondary);opacity:.6}
.ioc-assistant-heading strong{flex:1;min-width:0;font-size:15px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.ioc-assistant-actions{display:flex;gap:8px;flex-shrink:0}
.ioc-assistant-heading button{cursor:pointer}
body.ioc-assistant-moving,body.ioc-assistant-moving *{cursor:move!important;user-select:none!important}
body.ioc-assistant-resizing{user-select:none}
/* One 32px row: the current title, a new-conversation icon, and (only when there
   are more than 50 threads) a paging icon. The old label + select + button stack
   cost 84px of a 740px panel, which is transcript space. */
.ioc-assistant-sessions{display:flex;align-items:center;flex-wrap:wrap;gap:8px;padding:8px 24px;flex-shrink:0;border-bottom:1px solid var(--border-color)}
.ioc-assistant-sessions>select{flex:1;min-width:0;min-height:32px;padding:4px 8px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-primary);font:inherit;font-size:12px;text-overflow:ellipsis}
.ioc-assistant-sessions>select:focus-visible{outline:2px solid var(--primary-color);outline-offset:2px}
.ioc-assistant-sessions>p{flex-basis:100%;margin:0}
.ioc-assistant .ioc-assistant-icon{display:inline-flex;flex:none;align-items:center;justify-content:center;width:32px;height:32px;padding:0}
.ioc-assistant-icon svg{width:16px;height:16px}
.ioc-assistant-spinner{flex:none;width:14px;height:14px;border-radius:50%;border:2px solid var(--border-color);border-top-color:var(--primary-color);animation:ioc-assistant-spin 800ms linear infinite}
@keyframes ioc-assistant-spin{to{transform:rotate(360deg)}}
.ioc-assistant-conversation{flex:1;min-height:0;overflow:auto;padding:16px 24px;overflow-wrap:anywhere;overscroll-behavior:contain}
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
.ioc-assistant-welcome-intro p{margin:8px 0 0;font-size:13px;line-height:1.7;color:var(--text-secondary)}.ioc-assistant-welcome-intro .ioc-assistant-welcome-note{font-size:12px;opacity:.85}
.ioc-assistant-examples{display:flex;flex-direction:column;gap:8px}
.ioc-assistant .ioc-assistant-examples button{display:flex;align-items:center;justify-content:space-between;gap:12px;width:100%;margin:0;padding:12px 16px;border:1px solid var(--border-color);border-radius:12px;background:var(--bg-body);color:var(--text-primary);font-size:13px;text-align:left;white-space:normal}
.ioc-assistant .ioc-assistant-examples button:hover{border-color:var(--primary-color);background:color-mix(in srgb,var(--primary-color) 6%,var(--bg-body))}
.ioc-assistant-example-arrow{flex:none;width:16px;height:16px;color:var(--text-secondary)}
.ioc-assistant .ioc-assistant-examples button:hover .ioc-assistant-example-arrow{color:var(--primary-color)}
.ioc-assistant-message{display:flex;align-items:flex-start;gap:8px;margin-bottom:16px}.ioc-assistant-message.user{flex-direction:row-reverse}.ioc-assistant-avatar{flex:none;width:28px;height:28px;padding:5px;border-radius:50%;background:var(--bg-body);border:1px solid var(--border-color);color:var(--text-secondary)}.ioc-assistant-message.user .ioc-assistant-avatar{background:color-mix(in srgb,var(--primary-color) 14%,var(--bg-body));border-color:color-mix(in srgb,var(--primary-color) 40%,var(--border-color));color:var(--primary-color)}.ioc-assistant-bubble{display:flex;flex-direction:column;gap:8px;max-width:calc(100% - 44px);min-width:0;padding:12px 16px;border:1px solid var(--border-color);border-radius:12px;background:color-mix(in srgb,var(--text-primary) 4%,var(--bg-surface))}.ioc-assistant-message.user .ioc-assistant-bubble{background:color-mix(in srgb,var(--primary-color) 10%,var(--bg-surface));border-color:color-mix(in srgb,var(--primary-color) 30%,var(--border-color))}.ioc-assistant-richtext{font-size:13px;line-height:1.7;color:var(--text-primary);overflow-wrap:anywhere}.ioc-assistant-richtext p{margin:0 0 8px}.ioc-assistant-richtext p:last-child,.ioc-assistant-richtext ul:last-child{margin-bottom:0}.ioc-assistant-richtext ul{margin:0 0 8px;padding-left:20px}.ioc-assistant-richtext li{margin:2px 0}.ioc-assistant-richtext code{padding:1px 4px;border-radius:4px;background:var(--bg-body);font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px}.ioc-assistant-richtext.is-collapsed{max-height:168px;overflow:hidden;-webkit-mask-image:linear-gradient(180deg,#000 80%,transparent);mask-image:linear-gradient(180deg,#000 80%,transparent)}.ioc-assistant-more{align-self:flex-start;margin:0!important;padding:4px 8px;font-size:12px}.ioc-assistant-message-context{display:flex;flex-wrap:wrap;gap:8px;font-size:12px;color:var(--text-secondary)}.ioc-assistant-message-context>span{padding:8px;background:var(--bg-body);border-radius:8px}.ioc-assistant-message-context details{flex-basis:100%;max-height:160px;overflow:auto}
.ioc-assistant-status{color:var(--text-secondary);font-size:12px}.ioc-assistant-progress{padding:8px 12px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-secondary);font-size:12px}.ioc-assistant-progress summary{display:flex;align-items:center;gap:8px;cursor:pointer;list-style:none;color:var(--text-secondary)}.ioc-assistant-progress summary::-webkit-details-marker{display:none}.ioc-assistant-progress summary::before{content:"";flex:none;width:6px;height:6px;border-radius:50%;background:var(--primary-color)}.ioc-assistant-stage-current{color:var(--text-primary)}.ioc-assistant-stage-count{margin-left:auto;font-variant-numeric:tabular-nums;opacity:.75}.ioc-assistant-dots{color:var(--primary-color)}.ioc-assistant-progress ol{margin:8px 0 0;padding-left:20px}.ioc-assistant-progress li{margin-top:4px}.ioc-assistant-progress li.is-done{color:var(--text-primary)}summary{cursor:pointer}
.ioc-assistant-proposal{padding:16px;border:1px solid var(--primary-color);border-radius:8px}.ioc-assistant-proposal ol{padding-left:24px}.ioc-assistant-proposal button{margin:8px 8px 0 0}.ioc-assistant-warning{color:var(--warning-color)}.ioc-assistant-error,.ioc-assistant-sync{padding:8px 24px;max-height:128px;overflow:auto;font-size:12px}.ioc-assistant-error{color:var(--danger-color)}.ioc-assistant-undo{display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:8px;padding:8px 24px;font-size:12px;color:var(--text-secondary)}
.ioc-assistant-compose{position:relative;flex-shrink:0;padding:16px 24px 24px;border-top:1px solid var(--border-color)}.ioc-assistant-input-label{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}.ioc-assistant-compose textarea,.ioc-assistant-device-picker input[type=search]{box-sizing:border-box;width:100%;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);color:var(--text-primary);font:inherit;padding:8px}.ioc-assistant-compose textarea{display:block;resize:vertical;min-height:72px;max-height:192px}.ioc-assistant-compose-footer,.ioc-assistant-tools{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.ioc-assistant-compose-footer{justify-content:space-between;margin-top:8px}.ioc-assistant-selection-chips{display:flex;gap:8px;flex-wrap:wrap;max-height:96px;overflow:auto;margin-bottom:8px;font-size:12px}.ioc-assistant-selection-chips>span{display:inline-flex;align-items:center;gap:8px;padding:8px;background:var(--bg-body);border:1px solid var(--border-color);border-radius:8px;max-width:100%;overflow-wrap:anywhere}.ioc-assistant-selection-chips button{padding:0 8px}.ioc-assistant-selection-hint{font-size:12px;color:var(--text-secondary);margin:0 0 8px}.ioc-assistant-selection-hint button{margin-left:8px}
.ioc-assistant-device-picker{position:absolute;bottom:72px;left:24px;right:24px;z-index:10;padding:16px;max-height:min(480px,65vh);overflow:auto;background:var(--bg-body);isolation:isolate;border:1px solid var(--border-color);border-radius:8px;box-shadow:0 16px 48px color-mix(in srgb,var(--text-primary) 16%,transparent)}.ioc-assistant-scope{display:flex;flex-wrap:wrap;gap:8px 16px;margin:8px 0;padding:8px 16px;border-radius:8px;background:var(--bg-body);color:var(--text-secondary);font-size:12px;line-height:1.6}.ioc-assistant-scope strong{color:var(--text-primary);font-weight:500}.ioc-assistant-scope.is-thin{color:var(--warning-color)}.ioc-assistant-hint{margin:0 0 8px;font-size:12px;line-height:1.6;color:var(--warning-color)}.ioc-assistant-picker-heading,.ioc-assistant-picker-actions{display:flex;align-items:center;justify-content:space-between;gap:8px;flex-wrap:wrap;margin-bottom:8px}.ioc-assistant-picker-actions{margin-top:8px}.ioc-assistant-device-options{max-height:240px;overflow:auto}.ioc-assistant-device-options label{display:flex;align-items:center;gap:8px;padding:8px;cursor:pointer;border-radius:8px}.ioc-assistant-device-options label:hover{background:var(--bg-body)}.ioc-assistant-device-options small{display:block;color:var(--text-secondary);margin-top:8px}.ioc-assistant-device-options input{accent-color:var(--primary-color)}
.ioc-assistant button{border:1px solid var(--border-color);background:var(--bg-body);color:var(--text-primary);border-radius:8px;padding:8px;font:inherit;font-size:12px;cursor:pointer;transition:background var(--transition-fast),border-color var(--transition-fast)}.ioc-assistant button:hover,.ioc-assistant button[aria-pressed=true]{border-color:var(--primary-color)}.ioc-assistant button:disabled{opacity:.5;cursor:not-allowed}.ioc-assistant button.ioc-assistant-primary{background:var(--primary-color);border-color:var(--primary-color);color:var(--bg-body)}.ioc-assistant :is(button,input,textarea,summary):focus-visible{outline:2px solid var(--primary-color);outline-offset:2px}
@media(prefers-reduced-motion:reduce){.ioc-assistant button{transition:none}}
</style>
