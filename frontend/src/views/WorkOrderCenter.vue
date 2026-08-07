<template>
  <div class="container-fluid py-4 work-order-center">
    <div class="d-flex flex-wrap justify-content-between align-items-center gap-3 mb-4">
      <div>
        <h2 class="h4 mb-1 fw-bold text-primary border-start border-primary border-4 ps-2">{{ tx('pageTitle') }}</h2>
        <div class="text-muted small">{{ tx('pageSubtitle') }}</div>
      </div>
      <div class="btn-group">
        <button class="btn btn-outline-secondary" :disabled="loading" @click="refreshCurrentTab">
          <i class="bi bi-arrow-clockwise me-1"></i>{{ tx('refresh') }}
        </button>
        <button v-if="activeTab === 'orders' && authStore.hasPermission('work_order:create')" class="btn btn-primary" @click="openCreateOrder">
          <i class="bi bi-plus-lg me-1"></i>{{ tx('newWorkOrder') }}
        </button>
        <button v-else-if="authStore.hasPermission('work_order:manage')" class="btn btn-primary" @click="openCreateTemplate">
          <i class="bi bi-file-earmark-plus me-1"></i>{{ tx('newTemplate') }}
        </button>
      </div>
    </div>

    <div v-if="errorMessage" class="alert alert-danger alert-dismissible fade show position-fixed top-0 start-50 translate-middle-x mt-4 shadow" style="z-index: 1060; min-width: 300px;" role="alert">
      <i class="bi bi-exclamation-triangle-fill me-2"></i>{{ errorMessage }}
      <button type="button" class="btn-close" @click="errorMessage = ''"></button>
    </div>
    <div v-if="successMessage" class="alert alert-success alert-dismissible fade show position-fixed top-0 start-50 translate-middle-x mt-4 shadow" style="z-index: 1060; min-width: 300px;" role="alert">
      <i class="bi bi-check-circle-fill me-2"></i>{{ successMessage }}
      <button type="button" class="btn-close" @click="successMessage = ''"></button>
    </div>

    <ul class="nav nav-tabs mb-3">
      <li class="nav-item">
        <button class="nav-link" :class="{ active: activeTab === 'orders' }" @click="activeTab = 'orders'">
          <i class="bi bi-ticket-detailed me-1"></i>{{ tx('ordersTab') }}
        </button>
      </li>
      <li class="nav-item">
        <button class="nav-link" :class="{ active: activeTab === 'templates' }" @click="activeTab = 'templates'">
          <i class="bi bi-ui-checks-grid me-1"></i>{{ tx('templatesTab') }}
        </button>
      </li>
      <li v-if="workOrderIntegrationsVisible && authStore.hasPermission('work_order:integration')" class="nav-item">
        <button class="nav-link" :class="{ active: activeTab === 'integrations' }" @click="activeTab = 'integrations'">
          <i class="bi bi-plug me-1"></i>{{ tx('integrations') }}
        </button>
      </li>
    </ul>

    <section v-if="activeTab === 'orders'">
      <div class="card border-0 shadow-sm mb-3 work-order-filter-card">
        <div class="card-body py-3">
          <div class="d-flex flex-wrap align-items-center gap-2 mb-3 work-order-relation-filter" :aria-label="tx('relatedViews')">
            <span class="small fw-semibold text-body-secondary me-1"><i class="bi bi-person-check me-1"></i>{{ tx('relatedToMe') }}</span>
            <button v-for="relation in relationFilters" :key="relation.key || 'all'" type="button" class="btn btn-sm rounded-pill" :class="orderFilters.relation === relation.key ? 'btn-primary' : 'btn-outline-secondary'" @click="applyRelationFilter(relation.key)">{{ relation.label }}</button>
          </div>
          <div class="row g-2">
            <div class="col-md-4">
              <input v-model.trim="orderFilters.keyword" class="form-control" :placeholder="tx('searchPlaceholder')" @keyup.enter="applyOrderFilters">
            </div>
            <div class="col-md-3">
              <select v-model="orderFilters.status" class="form-select" @change="applyOrderFilters">
                <option value="">{{ tx('allStatuses') }}</option>
                <option v-for="status in workflowStatuses" :key="status.key" :value="status.key">{{ status.name }}</option>
              </select>
            </div>
            <div class="col-md-3">
              <select v-model="orderFilters.sourceType" class="form-select" @change="applyOrderFilters">
                <option value="">{{ tx('allSources') }}</option>
                <option value="manual">{{ tx('sourceManual') }}</option>
                <option value="rule">{{ tx('sourceRule') }}</option>
                <option value="alarm">{{ tx('sourceAlarm') }}</option>
                <option value="ai">{{ tx('sourceAI') }}</option>
                <option value="external">{{ tx('sourceExternal') }}</option>
              </select>
            </div>
            <div class="col-md-2 d-grid">
              <button class="btn btn-primary shadow-sm" :disabled="loading" @click="applyOrderFilters"><i class="bi bi-search me-1"></i>{{ tx('query') }}</button>
            </div>
          </div>
        </div>
      </div>

      <div class="card border-0 shadow-sm work-order-list-card">
            <div class="table-responsive">
              <table class="table align-middle mb-0 work-order-table">
                <thead class="work-order-table-head">
                  <tr>
                    <th>{{ tx('columnWorkOrder') }}</th>
                    <th>{{ tx('columnSource') }}</th>
                    <th>{{ tx('columnPriority') }}</th>
                    <th>{{ tx('columnStatus') }}</th>
                    <th>{{ tx('columnHandler') }}</th>
                    <th>{{ tx('columnCreatedAt') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-if="loading && orders.length === 0"><td colspan="6" class="text-center py-5 text-muted"><span class="spinner-border spinner-border-sm me-2"></span>{{ tx('loading') }}</td></tr>
                  <tr v-else-if="filteredOrders.length === 0"><td colspan="6" class="text-center py-5 text-muted"><i class="bi bi-inbox fs-3 d-block mb-2"></i>{{ tx('noOrders') }}</td></tr>
                  <tr v-for="entry in filteredOrders" :key="workOrderId(entry.work_order)" role="button" tabindex="0" class="work-order-row" :class="{ 'work-order-row--selected': workOrderId(selectedOrder?.work_order) === workOrderId(entry.work_order) }" @click="selectOrder(workOrderId(entry.work_order))" @keydown.enter.prevent="selectOrder(workOrderId(entry.work_order))" @keydown.space.prevent="selectOrder(workOrderId(entry.work_order))">
                    <td class="work-order-main-cell">
                      <div class="fw-semibold">{{ entry.work_order.title }}</div>
                    </td>
                    <td :data-label="tx('columnSource')"><span class="badge border work-order-source-badge">{{ sourceLabel(entry.work_order.source_type) }}</span></td>
                    <td :data-label="tx('columnPriority')"><span class="badge" :class="priorityClass(entry.work_order.priority)">{{ priorityLabel(entry.work_order.priority) }}</span></td>
                    <td :data-label="tx('columnStatus')"><span class="badge" :class="statusClass(effectiveWorkOrderStatus(entry.work_order))">{{ statusLabel(effectiveWorkOrderStatus(entry.work_order)) }}</span></td>
                    <td :data-label="tx('columnHandler')">{{ responsibilityLabel(entry) }}</td>
                    <td class="small text-muted" :data-label="tx('columnCreatedAt')">{{ formatTime(entry.work_order.CreatedAt) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
            <ListPagination :page="orderPage" :page-size="orderPageSize" :total="orderTotal" :page-size-options="orderPageSizeOptions" :disabled="loading" id-prefix="work-orders" @update:page="goOrderPage" @update:page-size="changeOrderPageSize" />
      </div>

      <div v-if="selectedOrder" class="offcanvas-backdrop fade show" @click="closeOrderDetail"></div>
      <aside v-if="selectedOrder" ref="orderDetailDialog" class="work-order-drawer shadow-lg" aria-modal="true" role="dialog" tabindex="-1" aria-labelledby="work-order-detail-title" @keydown.esc.stop="closeOrderDetail">
        <div class="work-order-drawer__head">
                <div>
                  <h2 id="work-order-detail-title" class="h5 mb-2 fw-bold text-body">{{ selectedOrder.work_order.title }}</h2>
                  <div class="d-flex gap-2">
                    <span class="badge" :class="statusClass(effectiveWorkOrderStatus(selectedOrder.work_order))">{{ statusLabel(effectiveWorkOrderStatus(selectedOrder.work_order)) }}</span>
                    <span class="badge" :class="priorityClass(selectedOrder.work_order.priority)">{{ priorityLabel(selectedOrder.work_order.priority) }}</span>
                  </div>
                </div>
                <div class="d-flex align-items-center gap-2">
                <button v-if="selectedOrder.available_actions?.claim && authStore.hasPermission('work_order:process')" class="btn btn-primary shadow-sm" :disabled="actionLoading" @click="claimOrder"><i class="bi bi-person-check me-1"></i>{{ tx('claim') }}</button>
                  <button type="button" class="btn-close" :aria-label="tx('close')" @click="closeOrderDetail"></button>
                </div>
        </div>
        <div class="work-order-drawer__body">
              <p v-if="workOrderSummary(selectedOrder)" class="mb-4 text-body-secondary">{{ workOrderSummary(selectedOrder) }}</p>
              <div v-if="selectedOrder.work_order.assignee_user_id" class="small text-body-secondary mb-4"><i class="bi bi-person-check me-1"></i>{{ tx('claimedBy') }}{{ assigneeLabel(selectedOrder.work_order.assignee_user_id) }}</div>

              <div class="mb-4 bg-body-tertiary rounded p-3 work-order-form-section">
                <div class="fw-bold small mb-3 text-secondary"><i class="bi bi-card-text me-1"></i>{{ tx('formContent') }}</div>
                <dl class="row small mb-0 g-2">
                  <template v-for="field in selectedOrder.form_definition.fields || []" :key="field.key">
                    <dt class="col-sm-4 text-muted fw-normal">{{ workOrderFormFieldLabel(field) }}</dt>
                    <dd class="col-sm-8 text-break fw-medium">
                      <div v-if="field.type === 'images' && imageFormValues(selectedOrder.form_data[field.key]).length" class="work-order-detail-images">
                        <a v-for="(imageUrl, imageIndex) in imageFormValues(selectedOrder.form_data[field.key])" :key="`${imageUrl}-${imageIndex}`" :href="imageUrl" target="_blank" rel="noopener noreferrer"><img :src="imageUrl" :alt="`${field.label} ${imageIndex + 1}`"></a>
                      </div>
                      <span v-else>{{ displayFormValue(selectedOrder.form_data[field.key], field) }}</span>
                    </dd>
                  </template>
                </dl>
                <div v-if="alarmSourceDetails.length" class="mt-3 pt-3 border-top">
                  <div class="fw-bold small mb-2 text-secondary"><i class="bi bi-broadcast-pin me-1"></i>{{ tx('alarmEvidence') }}</div>
                  <dl class="row small mb-0 g-2"><template v-for="detail in alarmSourceDetails" :key="detail.id"><dt class="col-sm-4 text-muted fw-normal">{{ detail.label }}</dt><dd class="col-sm-8 text-break fw-medium">{{ detail.value }}</dd></template></dl>
                </div>
              </div>

              <section v-if="historicalGuidance" class="mb-4 work-order-history-guidance rounded p-3" aria-labelledby="work-order-history-guidance-title">
                <div class="d-flex flex-wrap align-items-start justify-content-between gap-2">
                  <div>
                    <div id="work-order-history-guidance-title" class="fw-bold text-success-emphasis"><i class="bi bi-shield-check me-2"></i>{{ tx('historicalGuidance') }}</div>
                    <div class="small text-body-secondary mt-1">{{ tx('historicalGuidanceHint') }}</div>
                  </div>
                  <span v-if="historicalGuidance.verificationCount" class="badge rounded-pill work-order-history-guidance__verified">{{ txf('historicalVerifiedCount', { count: historicalGuidance.verificationCount }) }}</span>
                </div>
                <p v-if="historicalGuidance.summary" class="small mb-0 mt-3 work-order-history-guidance__summary">{{ historicalGuidance.summary }}</p>
                <div class="work-order-history-guidance__list mt-3">
                  <article v-for="(experience, experienceIndex) in historicalGuidance.experiences" :key="experience.id" class="work-order-history-experience">
                    <header class="work-order-history-experience__head">
                      <span class="work-order-history-experience__index">{{ experienceIndex + 1 }}</span>
                      <strong>{{ txf('historicalExperience', { index: experienceIndex + 1 }) }}</strong>
                      <span v-if="experience.verificationCount" class="badge rounded-pill work-order-history-experience__count">{{ txf('historicalVerifiedCount', { count: experience.verificationCount }) }}</span>
                      <span v-if="experience.lastVerifiedAt" class="small text-body-secondary ms-auto"><i class="bi bi-clock me-1"></i>{{ formatTime(experience.lastVerifiedAt) }}</span>
                    </header>
                    <dl class="row small mb-0 g-2 work-order-history-experience__details">
                      <template v-for="item in experience.details" :key="item.labelKey">
                        <dt class="col-sm-4 text-body-secondary fw-normal">{{ tx(item.labelKey) }}</dt>
                        <dd class="col-sm-8 mb-0 text-break fw-medium">{{ item.value }}</dd>
                      </template>
                    </dl>
                  </article>
                </div>
              </section>

              <div class="mb-4 work-order-action-panel rounded p-3 border">
                <div class="d-flex flex-wrap justify-content-between align-items-center gap-2">
                  <div>
                    <div class="fw-bold small text-secondary"><i class="bi bi-lightning-charge me-1"></i>{{ tx('workOrderActions') }}</div>
                    <div class="small text-body-secondary mt-1">{{ tx('actionHint') }}</div>
                  </div>
                  <div class="d-flex flex-wrap gap-2">
                    <button class="btn btn-sm btn-outline-secondary" :disabled="actionLoading" @click="openCCModal"><i class="bi bi-send me-1"></i>{{ tx('cc') }}</button>
                    <button v-if="canTransferOrder" class="btn btn-sm btn-outline-primary" :disabled="actionLoading" @click="openTransferModal"><i class="bi bi-arrow-left-right me-1"></i>{{ tx('transfer') }}</button>
                    <button v-if="processActionOptions.length" class="btn btn-sm btn-primary" :disabled="actionLoading" @click="openProcessModal"><i class="bi bi-check2-square me-1"></i>{{ tx('processTitle') }}</button>
                  </div>
                </div>
                <div v-if="myPendingGraphTasks.length" class="mt-3 pt-3 border-top">
                  <div class="small text-body-secondary mb-2">{{ tx('myTasks') }}</div>
                  <div class="d-flex flex-wrap gap-2">
                    <span v-for="task in myPendingGraphTasks" :key="task.public_id" class="badge rounded-pill work-order-task-badge border">
                      {{ task.task_kind === 'approve' ? tx('approval') : tx('task') }} · {{ workOrderNodeLabel(selectedOrder.workflow, task.node_id, currentLang) }}
                    </span>
                  </div>
                </div>
              </div>

              <div v-if="selectedOrder.resolution?.actual_problem" class="mb-4 border rounded p-3 bg-body-tertiary">
                <div class="fw-bold small mb-2 text-secondary"><i class="bi bi-clipboard2-check me-1"></i>{{ tx('resolution') }}</div>
                <dl class="row small mb-0 g-2">
                  <dt class="col-sm-4 text-muted fw-normal">{{ tx('actualProblem') }}</dt><dd class="col-sm-8 text-break">{{ selectedOrder.resolution.actual_problem }}</dd>
                  <dt class="col-sm-4 text-muted fw-normal">{{ tx('rootCause') }}</dt><dd class="col-sm-8 text-break">{{ selectedOrder.resolution.root_cause }}</dd>
                  <dt class="col-sm-4 text-muted fw-normal">{{ tx('handlingProcess') }}</dt><dd class="col-sm-8 text-break">{{ selectedOrder.resolution.handling_process }}</dd>
                  <dt class="col-sm-4 text-muted fw-normal">{{ tx('handlingResult') }}</dt><dd class="col-sm-8 text-break">{{ selectedOrder.resolution.handling_result }}</dd>
                </dl>
              </div>

          <WorkOrderAuditTimeline :workflow="selectedOrder.workflow" :tasks="workOrderTasks" :events="orderEvents" :locale="currentLang" />
        </div>
      </aside>
    </section>

    <section v-else-if="activeTab === 'templates'">
      <div class="card border-0 shadow-sm">
        <div class="table-responsive">
          <table class="table table-hover align-middle mb-0">
            <thead class="table-light">
              <tr>
                <th>{{ tx('templateName') }}</th>
                <th>{{ tx('columnCreatedAt') }}</th>
                    <th class="text-end">{{ tx('columnActions') }}</th>
</tr>
            </thead>
            <tbody>
              <tr v-if="templates.length === 0"><td colspan="3" class="text-center py-5 text-muted"><i class="bi bi-inbox fs-3 d-block mb-2"></i>{{ tx('noTemplates') }}</td></tr>
              <tr v-for="template in templates" :key="template.ID" class="work-order-template-row" role="button" tabindex="0" @click="openTemplateDetail(template.ID)" @keydown.enter.prevent="openTemplateDetail(template.ID)" @keydown.space.prevent="openTemplateDetail(template.ID)">
                <td class="fw-semibold">{{ template.name }}</td>
                <td class="small text-muted">{{ formatTime(template.CreatedAt) }}</td>
                <td class="text-end">
                  <div class="btn-group">
                    <button class="btn btn-sm btn-outline-success" :aria-label="`${tx('configureTemplate')} ${template.name}`" @click.stop="openTemplateEditor(template.ID)" :title="tx('configureWorkflowForm')"><i class="bi bi-diagram-3"></i></button>
                    <button v-if="authStore.hasPermission('work_order:manage')" class="btn btn-sm btn-outline-secondary" :aria-label="`${tx('copyTemplate')} ${template.name}`" :disabled="actionLoading" @click.stop="copyTemplate(template)" :title="tx('copyTemplate')"><i class="bi bi-copy"></i></button>
                    <button v-if="!template.system_managed" class="btn btn-sm btn-outline-primary" :aria-label="`${tx('editTemplate')} ${template.name}`" @click.stop="openEditTemplate(template)" :title="tx('editBasicInfo')"><i class="bi bi-pencil"></i></button>
                    <button v-if="!template.system_managed" class="btn btn-sm btn-outline-danger" :aria-label="`${tx('deleteTemplate')} ${template.name}`" @click.stop="deleteTemplate(template)" :title="tx('deleteTemplate')"><i class="bi bi-trash"></i></button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </section>

    <div v-if="templateDetail" class="offcanvas-backdrop fade show" @click="closeTemplateDetail"></div>
    <aside v-if="templateDetail" ref="templateDetailDialog" class="template-detail-drawer shadow-lg" aria-modal="true" role="dialog" tabindex="-1" aria-labelledby="template-detail-title" @keydown.esc.stop="closeTemplateDetail">
      <div class="template-detail-drawer__head"><div><h2 id="template-detail-title" class="h5 mb-1">{{ templateDetail.template.name }}</h2><p class="small text-body-secondary mb-0">{{ templateDetail.template.code }}</p></div><button type="button" class="btn-close" :aria-label="tx('close')" @click="closeTemplateDetail"></button></div>
      <div class="template-detail-drawer__body"><p v-if="templateDetail.template.description" class="text-body-secondary">{{ templateDetail.template.description }}</p><section class="template-detail-workflow"><h3 class="section-title">{{ tx('workflowConfig') }}</h3><WorkOrderWorkflowEditor :model-value="templateDetailWorkflow" :participants="projectParticipants" :participants-loading="participantsLoading" :show-properties="false" readonly /></section></div>
    </aside>

    <section v-else-if="workOrderIntegrationsVisible && activeTab === 'integrations'" class="work-order-integrations">
      <div class="alert alert-info border-0 shadow-sm">
        <i class="bi bi-shield-check me-2"></i>{{ tx('integrationInfo') }}
      </div>
      <div class="row g-3">
        <div class="col-12 col-xl-4">
          <div class="card border-0 shadow-sm h-100"><div class="card-body">
            <h5 class="card-title">{{ tx('connector') }}</h5>
            <label class="form-label small" for="connector-name">{{ tx('name') }}</label><input id="connector-name" v-model.trim="connectorDraft.name" class="form-control mb-2" :placeholder="tx('exampleCMMS')">
            <label class="form-label small" for="connector-key-id">Key ID</label><input id="connector-key-id" v-model.trim="connectorDraft.key_id" class="form-control mb-2" :placeholder="tx('remoteIdentity')">
            <label class="form-label small" for="connector-secret">Secret</label><input id="connector-secret" v-model.trim="connectorDraft.secret" type="password" class="form-control mb-3" autocomplete="new-password" :placeholder="tx('secretSaveOnly')">
            <button class="btn btn-primary w-100" :disabled="actionLoading" @click="saveConnector">{{ tx('saveConnector') }}</button>
            <div class="list-group list-group-flush mt-3"><div v-for="connector in connectors" :key="connector.id" class="list-group-item px-0"><div class="fw-semibold">{{ connector.name }}</div><small class="text-muted">{{ connector.key_id }} · {{ connector.enabled ? tx('enabled') : tx('disabled') }}</small></div><div v-if="!connectors.length" class="text-muted small py-2">{{ tx('noConnectors') }}</div></div>
          </div></div>
        </div>
        <div class="col-12 col-xl-4">
          <div class="card border-0 shadow-sm h-100"><div class="card-body">
            <h5 class="card-title">{{ tx('binding') }}</h5>
            <label class="form-label small" for="binding-id">{{ tx('bindingID') }}</label><input id="binding-id" v-model.trim="bindingDraft.id" class="form-control mb-2" :placeholder="tx('exampleBinding')">
            <label class="form-label small" for="binding-connector">{{ tx('connector') }}</label><select id="binding-connector" v-model="bindingDraft.connector_id" class="form-select mb-2"><option value="">{{ tx('selectConnector') }}</option><option v-for="connector in connectors" :key="connector.id" :value="connector.id">{{ connector.name }}</option></select>
            <label class="form-label small" for="binding-direction">{{ tx('direction') }}</label><select id="binding-direction" v-model="bindingDraft.direction" class="form-select mb-3"><option value="inbound">{{ tx('inbound') }}</option><option value="outbound">{{ tx('outbound') }}</option><option value="bidirectional">{{ tx('bidirectional') }}</option></select>
            <button class="btn btn-primary w-100" :disabled="actionLoading" @click="saveBinding">{{ tx('saveBinding') }}</button>
            <div class="list-group list-group-flush mt-3"><button v-for="binding in bindings" :key="binding.id" class="list-group-item list-group-item-action px-0" :class="{ active: selectedBindingID === binding.id }" @click="selectBinding(binding.id)"><div class="fw-semibold">{{ binding.id }}</div><small :class="selectedBindingID === binding.id ? 'text-white-50' : 'text-muted'">{{ binding.direction }} · v{{ binding.last_revision }}</small></button><div v-if="!bindings.length" class="text-muted small py-2">{{ tx('noBindings') }}</div></div>
          </div></div>
        </div>
        <div class="col-12 col-xl-4">
          <div class="card border-0 shadow-sm h-100"><div class="card-body">
            <h5 class="card-title">{{ tx('mapping') }}</h5>
            <div v-if="selectedBindingID">
              <label class="form-label small" for="mapping-id">{{ tx('mappingID') }}</label><input id="mapping-id" v-model.trim="mappingDraft.id" class="form-control mb-2" :placeholder="tx('exampleMapping')">
              <label class="form-label small" for="mapping-name">{{ tx('name') }}</label><input id="mapping-name" v-model.trim="mappingDraft.name" class="form-control mb-2" :placeholder="tx('externalMapping')">
              <label class="form-label small" for="mapping-fields">{{ tx('mappingJSON') }}</label><textarea id="mapping-fields" v-model.trim="mappingDraft.fieldsJSON" rows="4" class="form-control font-monospace small mb-3" aria-describedby="mapping-help"></textarea><div id="mapping-help" class="form-text">{{ tx('mappingExample') }}</div>
              <button class="btn btn-primary w-100" :disabled="actionLoading" @click="saveMapping">{{ tx('saveMapping') }}</button>
              <div class="list-group list-group-flush mt-3"><div v-for="mapping in mappings" :key="mapping.id" class="list-group-item px-0"><div class="fw-semibold">{{ mapping.name }}</div><small class="text-muted">{{ mapping.id }} · v{{ mapping.revision }}</small></div></div>
            </div>
            <p v-else class="text-muted small mb-0">{{ tx('selectBinding') }}</p>
          </div></div>
        </div>
      </div>
    </section>

    <div v-if="showTemplateEditor && selectedTemplate" class="modal fade show d-block work-order-template-editor-modal" tabindex="-1" @click.self="closeTemplateEditor">
      <div class="modal-dialog modal-xl work-order-template-editor-dialog">
        <div class="modal-content border-0 shadow-lg">
          <div class="modal-header">
            <div><h5 class="modal-title">{{ selectedTemplate.template.name }}</h5><div class="small text-muted">{{ selectedTemplate.template.code }}</div></div>
            <button type="button" class="btn-close" @click="closeTemplateEditor"></button>
          </div>
          <div class="modal-body">
            <div class="template-editor-tabs border-bottom px-3 py-2">
              <div class="nav nav-pills flex-shrink-0" :aria-label="tx('templateConfigType')"><button class="nav-link" :class="{ active: templateEditorTab === 'workflow' }" @click="templateEditorTab = 'workflow'"><i class="bi bi-diagram-3 me-1"></i>{{ tx('workflowConfig') }}</button><button class="nav-link" :class="{ active: templateEditorTab === 'form' }" @click="templateEditorTab = 'form'"><i class="bi bi-ui-checks-grid me-1"></i>{{ tx('formConfig') }}</button></div>
              <p v-if="selectedTemplate.template.description" class="template-editor-description text-muted small" :title="selectedTemplate.template.description">{{ selectedTemplate.template.description }}</p>
              <div v-if="selectedTemplate.template.system_managed && templateEditorTab === 'form'" class="template-editor-readonly small text-info"><i class="bi bi-lock me-1"></i>{{ tx('managedFormReadonly') }}</div>
              <div v-else-if="isAIManagedTemplate" class="template-editor-readonly small text-info"><i class="bi bi-diagram-3 me-1"></i>{{ tx('managedWorkflowEditable') }}</div>
              <div class="template-editor-actions" role="toolbar" :aria-label="templateEditorTab === 'workflow' ? tx('workflowActions') : tx('formActions')">
                <template v-if="templateEditorTab === 'workflow'">
                  <button class="btn btn-sm btn-light text-secondary" :disabled="actionLoading || workflowEditorReadonly" :title="tx('saveWorkflowDraft')" @click="saveWorkflowDraft"><i class="bi bi-save me-1"></i><span>{{ tx('saveWorkflowDraft') }}</span></button>
                  <button class="btn btn-sm btn-primary" :disabled="actionLoading || workflowEditorReadonly" :title="tx('publishWorkflow')" @click="publishWorkflow"><i class="bi bi-cloud-upload me-1"></i><span>{{ tx('publishWorkflow') }}</span></button>
                  <button class="btn btn-sm btn-light text-secondary" :title="tx('fullscreenWorkflow')" @click="toggleWorkflowFullscreen"><i class="bi bi-arrows-fullscreen me-1"></i><span>{{ tx('fullscreenWorkflow') }}</span></button>
                </template>
                <template v-else>
                  <button class="btn btn-sm btn-light text-secondary" :disabled="actionLoading || selectedTemplate.template.system_managed" :title="tx('saveFormDraft')" @click="saveFormDraft"><i class="bi bi-save me-1"></i><span>{{ tx('saveFormDraft') }}</span></button>
                  <button class="btn btn-sm btn-primary" :disabled="actionLoading || selectedTemplate.template.system_managed" :title="tx('publishForm')" @click="publishForm"><i class="bi bi-cloud-upload me-1"></i><span>{{ tx('publishForm') }}</span></button>
                </template>
              </div>
            </div>

            <div v-if="isAIManagedTemplate && !selectedTemplate.template.current_workflow_version" class="alert alert-warning small mx-3 mt-3 mb-0"><i class="bi bi-exclamation-triangle me-1"></i>{{ tx('aiWorkflowNeedsPublish') }}</div>

            <div v-if="templateEditorTab === 'form'" class="template-editor-tab-panel p-0">
              <WorkOrderFormDesigner v-model="formDraft" :readonly="selectedTemplate.template.system_managed" :locale="currentLang" />
            </div>

            <div v-else class="template-editor-workflow-panel p-2">
              <h6 class="visually-hidden">{{ tx('visualWorkflow') }}</h6>
              <WorkOrderWorkflowEditor ref="workflowEditorRef" v-model="workflowDraft" :participants="projectParticipants" :participants-loading="participantsLoading" :readonly="workflowEditorReadonly" :saving="actionLoading" :error="errorMessage" />
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showOrderModal" class="modal fade show d-block" style="background: rgba(0,0,0,.45)" @click.self="showOrderModal = false">
      <div class="modal-dialog modal-lg modal-dialog-scrollable"><div class="modal-content"><div class="modal-header"><h5 class="modal-title">{{ tx('newManualOrder') }}</h5><button type="button" class="btn-close" :aria-label="tx('close')" @click="showOrderModal = false"></button></div><div class="modal-body">
        <div class="row g-3 mb-3"><div class="col-md-6"><label class="form-label">{{ tx('workOrderTemplate') }} *</label><select v-model.number="createOrder.templateId" class="form-select" @change="loadCreateTemplate"><option :value="0">{{ tx('selectTemplate') }}</option><option v-for="template in templates" :key="template.ID" :value="template.ID">{{ template.name }}</option></select></div><div class="col-md-3"><label class="form-label">{{ tx('priority') }}</label><select v-model="createOrder.priority" class="form-select"><option value="low">{{ tx('low') }}</option><option value="normal">{{ tx('normal') }}</option><option value="high">{{ tx('high') }}</option><option value="urgent">{{ tx('urgent') }}</option></select></div><div class="col-12"><label class="form-label">{{ tx('title') }} *</label><input v-model.trim="createOrder.title" class="form-control"></div><div class="col-12"><label class="form-label">{{ tx('summary') }}</label><textarea v-model.trim="createOrder.summary" class="form-control" rows="2"></textarea></div></div>
        <WorkOrderFormFields v-if="createTemplateDetail" v-model="createOrder.formData" :definition="createTemplateDetail.form_definition" id-prefix="manual-work-order" :locale="currentLang" />
      </div><div class="modal-footer"><button class="btn btn-outline-secondary" @click="showOrderModal = false">{{ tx('cancel') }}</button><button class="btn btn-primary" :disabled="actionLoading" @click="createManualOrder"><span v-if="actionLoading" class="spinner-border spinner-border-sm me-1"></span>{{ tx('createWorkOrder') }}</button></div></div></div>
    </div>

    
    <!-- Edit Order Modal -->
    <div v-if="showEditOrderModal" class="modal fade show d-block" style="background: rgba(0,0,0,.45)" @click.self="showEditOrderModal = false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header"><h5 class="modal-title">{{ tx('editOrderTitle') }}</h5><button type="button" class="btn-close" :aria-label="tx('close')" @click="showEditOrderModal = false"></button></div>
          <div class="modal-body">
            <div class="mb-3"><label class="form-label">{{ tx('title') }} *</label><input v-model.trim="editOrderData.title" class="form-control"></div>
            <div class="mb-3"><label class="form-label">{{ tx('priority') }} *</label><select v-model="editOrderData.priority" class="form-select"><option value="low">{{ tx('low') }}</option><option value="normal">{{ tx('normal') }}</option><option value="high">{{ tx('high') }}</option><option value="urgent">{{ tx('urgent') }}</option></select></div>
            <div class="mb-3"><label class="form-label">{{ tx('summary') }}</label><textarea v-model.trim="editOrderData.summary" class="form-control" rows="2"></textarea></div>
          </div>
          <div class="modal-footer"><button class="btn btn-outline-secondary" @click="showEditOrderModal = false">{{ tx('cancel') }}</button><button class="btn btn-primary" :disabled="actionLoading" @click="submitEditOrder">{{ tx('save') }}</button></div>
        </div>
      </div>
    </div>

    <!-- Edit Template Modal -->
    <div v-if="showEditTemplateModal" class="modal fade show d-block" style="background: rgba(0,0,0,.45)" @click.self="showEditTemplateModal = false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header"><h5 class="modal-title">{{ tx('editTemplateTitle') }}</h5><button type="button" class="btn-close" :aria-label="tx('close')" @click="showEditTemplateModal = false"></button></div>
          <div class="modal-body">
            <div class="mb-3"><label class="form-label">{{ tx('templateName') }} *</label><input v-model.trim="editTemplateData.name" class="form-control"></div>
            <div class="form-text mt-2">{{ tx('templateNameHint') }}</div>
          </div>
          <div class="modal-footer"><button class="btn btn-outline-secondary" @click="showEditTemplateModal = false">{{ tx('cancel') }}</button><button class="btn btn-primary" :disabled="actionLoading" @click="submitEditTemplate">{{ tx('save') }}</button></div>
        </div>
      </div>
    </div>

    <div v-if="showTemplateModal" class="modal fade show d-block" style="background: rgba(0,0,0,.45)" @click.self="showTemplateModal = false">
      <div class="modal-dialog"><div class="modal-content"><div class="modal-header"><h5 class="modal-title">{{ tx('newTemplateTitle') }}</h5><button type="button" class="btn-close" :aria-label="tx('close')" @click="showTemplateModal = false"></button></div><div class="modal-body"><div class="mb-3"><label class="form-label">{{ tx('templateName') }} *</label><input v-model.trim="createTemplate.name" class="form-control"></div><div><label class="form-label">{{ tx('description') }}</label><textarea v-model.trim="createTemplate.description" class="form-control" rows="3"></textarea></div><div class="form-text mt-2">{{ tx('createTemplateHint') }}</div></div><div class="modal-footer"><button class="btn btn-outline-secondary" @click="showTemplateModal = false">{{ tx('cancel') }}</button><button class="btn btn-primary" :disabled="actionLoading" @click="createNewTemplate">{{ tx('create') }}</button></div></div></div>
    </div>

    <div v-if="showProcessModal" class="modal fade show d-block work-order-process-modal" style="background: rgba(0,0,0,.52)" @click.self="closeProcessModal">
      <div class="modal-dialog modal-lg modal-dialog-scrollable modal-dialog-centered"><div class="modal-content border-0 shadow-lg"><div class="modal-header"><div><h5 class="modal-title">{{ tx('processTitle') }}：{{ selectedOrder?.work_order?.title }}</h5><div class="small text-body-secondary">{{ tx('processSubtitle') }}</div></div><button type="button" class="btn-close" :aria-label="tx('close')" @click="closeProcessModal"></button></div><div class="modal-body">
        <div class="mb-3">
          <label class="form-label" for="process-action">{{ tx('processAction') }} *</label>
          <select id="process-action" v-model="selectedProcessActionID" class="form-select">
            <option v-for="action in processActionOptions" :key="action.id" :value="action.id">{{ action.label }}</option>
          </select>
        </div>
        <div v-if="currentProcessAction?.requiresResolution" class="rounded border p-3 mb-3 bg-body-tertiary">
          <p class="text-body-secondary small">{{ tx('resolutionHint') }}</p>
          <div class="row g-3">
            <div class="col-12 col-md-6"><label class="form-label" for="resolution-problem">{{ tx('actualProblem') }} *</label><textarea id="resolution-problem" v-model.trim="resolutionDraft.actual_problem" class="form-control" rows="3"></textarea></div>
            <div class="col-12 col-md-6"><label class="form-label" for="resolution-cause">{{ tx('rootCause') }} *</label><textarea id="resolution-cause" v-model.trim="resolutionDraft.root_cause" class="form-control" rows="3"></textarea></div>
            <div class="col-12 col-md-6"><label class="form-label" for="resolution-process">{{ tx('handlingProcess') }} *</label><textarea id="resolution-process" v-model.trim="resolutionDraft.handling_process" class="form-control" rows="3"></textarea></div>
            <div class="col-12 col-md-6"><label class="form-label" for="resolution-result">{{ tx('handlingResult') }} *</label><textarea id="resolution-result" v-model.trim="resolutionDraft.handling_result" class="form-control" rows="3"></textarea></div>
          </div>
        </div>
        <div class="mb-3">
          <label class="form-label" for="process-opinion">{{ tx('processOpinion') }} *</label>
          <textarea id="process-opinion" v-model="processOpinion" class="form-control" rows="4" maxlength="2000" :placeholder="tx('processOpinionPlaceholder')"></textarea>
          <div class="form-text">{{ tx('processOpinionHint') }}</div>
        </div>
        <div class="mb-3">
          <label class="form-label d-flex align-items-center justify-content-between">
            <span><i class="bi bi-paperclip me-1"></i>{{ tx('attachments') }}</span>
            <span class="small text-body-secondary fw-normal">{{ tx('optional') }}</span>
          </label>
          <div class="d-flex align-items-center gap-2 mb-2">
            <label class="btn btn-sm btn-outline-secondary mb-0 d-inline-flex align-items-center gap-1" :class="{ disabled: processAttachmentUploading }">
              <span v-if="processAttachmentUploading" class="spinner-border spinner-border-sm"></span>
              <i v-else class="bi bi-upload"></i>
              <span>{{ tx('uploadAttachment') }}</span>
              <input type="file" multiple class="d-none" :disabled="processAttachmentUploading" @change="uploadProcessAttachments">
            </label>
            <span class="small text-body-secondary">{{ tx('attachmentHint') }}</span>
          </div>
          <div v-if="processAttachmentError" class="alert alert-danger p-2 small mb-2">{{ processAttachmentError }}</div>
          <div v-if="processAttachments.length" class="d-flex flex-wrap gap-2 pt-2 align-items-center">
            <template v-for="(att, attIdx) in processAttachments" :key="attIdx">
              <div v-if="isImageAttachment(att)" class="position-relative border rounded overflow-hidden shadow-sm work-order-attachment-card" :title="`${att.name} (${formatFileSize(att.size)})`">
                <a :href="att.url" target="_blank" rel="noopener noreferrer">
                  <img :src="att.url" :alt="att.name" class="d-block work-order-attachment-img">
                </a>
                <button type="button" class="btn btn-sm btn-danger d-flex align-items-center justify-content-center work-order-attachment-del" :aria-label="tx('delete')" :title="tx('delete')" @click.stop.prevent="removeProcessAttachment(attIdx)">
                  <i class="bi bi-x"></i>
                </button>
              </div>
              <div v-else class="border rounded-pill p-2 px-3 d-inline-flex align-items-center gap-2 shadow-sm work-order-attachment-doc" :title="`${att.name} (${formatFileSize(att.size)})`">
                <i class="bi" :class="getFileIcon(att)"></i>
                <span class="text-truncate" style="max-width: 180px;">{{ att.name }}</span>
                <small v-if="att.size" class="text-body-secondary">({{ formatFileSize(att.size) }})</small>
                <button type="button" class="btn-close ms-1" style="font-size: 0.65rem;" :aria-label="tx('delete')" @click="removeProcessAttachment(attIdx)"></button>
              </div>
            </template>
          </div>
        </div>
      </div><div class="modal-footer"><button class="btn btn-outline-secondary" @click="closeProcessModal">{{ tx('cancel') }}</button><button class="btn btn-primary" :disabled="actionLoading || !selectedProcessActionID || !processOpinion.trim() || (currentProcessAction?.requiresResolution && !isResolutionComplete)" @click="submitProcessAction"><span v-if="actionLoading" class="spinner-border spinner-border-sm me-1"></span>{{ tx('submitProcess') }}</button></div></div></div>
    </div>

    <!-- CC Modal -->
    <div v-if="showCCModal" class="modal fade show d-block" style="background: rgba(0,0,0,.45)" @click.self="showCCModal = false">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ tx('ccTitle') }}</h5>
            <button type="button" class="btn-close" :aria-label="tx('close')" @click="showCCModal = false"></button>
          </div>
          <div class="modal-body">
            <div class="mb-3">
              <label class="form-label">{{ tx('selectCC') }} *</label>
              <div class="border rounded participant-picker" style="max-height: 250px; overflow-y: auto;">
                <div class="p-2 border-bottom sticky-top bg-body" style="z-index: 10;">
                  <input v-model.trim="ccSearchKeyword" type="text" class="form-control form-control-sm" :placeholder="tx('searchMembers')">
                </div>
                <label v-for="participant in ccFilteredParticipants" :key="participant.id" class="participant-option px-3 py-2 d-flex align-items-center border-bottom mb-0" style="cursor:pointer">
                  <input type="checkbox" :value="participant.id" v-model="ccForm.userIDs" class="me-2 form-check-input mt-0">
                  <span>{{ participant.display_name || participant.username }}</span>
                  <small class="text-muted ms-auto">{{ participant.username }}</small>
                </label>
                <div v-if="ccFilteredParticipants.length === 0" class="text-muted small py-3 text-center">{{ tx('noMembers') }}</div>
              </div>
            </div>
            <div>
              <label class="form-label">{{ tx('additionalMessage') }} <span class="text-muted small">{{ tx('optional') }}</span></label>
              <textarea v-model.trim="ccForm.comment" class="form-control" rows="2" :placeholder="tx('ccMessagePlaceholder')"></textarea>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-outline-secondary" @click="showCCModal = false">{{ tx('cancel') }}</button>
            <button class="btn btn-primary" :disabled="actionLoading || ccForm.userIDs.length === 0" @click="submitCC">{{ tx('send') }}</button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showTransferModal" class="modal fade show d-block" style="background: rgba(0,0,0,.45)" @click.self="closeTransferModal">
      <div class="modal-dialog">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title">{{ tx('transferTitle') }}</h5>
            <button type="button" class="btn-close" :aria-label="tx('close')" @click="closeTransferModal"></button>
          </div>
          <div class="modal-body">
            <p class="small text-body-secondary">{{ tx('transferHint') }}</p>
            <div v-if="transferableHandleTasks.length > 1" class="mb-3">
              <label class="form-label" for="transfer-task">{{ tx('transferTask') }} *</label>
              <select id="transfer-task" v-model="transferForm.taskPublicID" class="form-select">
                <option v-for="task in transferableHandleTasks" :key="task.public_id" :value="task.public_id">{{ workOrderNodeLabel(selectedOrder.workflow, task.node_id, currentLang) }}</option>
              </select>
            </div>
            <div class="mb-3">
              <label class="form-label" for="transfer-recipient">{{ tx('transferRecipient') }} *</label>
              <select id="transfer-recipient" v-model="transferForm.targetUserID" class="form-select">
                <option value="">{{ tx('selectMember') }}</option>
                <option v-for="participant in transferFilteredParticipants" :key="participant.id" :value="String(participant.id)">{{ participant.display_name || participant.username }}{{ participant.display_name && participant.username ? ` (${participant.username})` : '' }}</option>
              </select>
            </div>
            <div>
              <label class="form-label" for="transfer-comment">{{ tx('transferReason') }} *</label>
              <textarea id="transfer-comment" v-model.trim="transferForm.comment" class="form-control" rows="3" maxlength="2000" :placeholder="tx('transferReasonPlaceholder')"></textarea>
            </div>
          </div>
          <div class="modal-footer">
            <button class="btn btn-outline-secondary" :disabled="actionLoading" @click="closeTransferModal">{{ tx('cancel') }}</button>
            <button class="btn btn-primary" :disabled="actionLoading || !transferForm.targetUserID || !transferForm.comment.trim()" @click="submitTransfer"><span v-if="actionLoading" class="spinner-border spinner-border-sm me-1"></span>{{ tx('confirmTransfer') }}</button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import axios from 'axios'
import WorkOrderAuditTimeline from '../components/work-order/WorkOrderAuditTimeline.vue'
import WorkOrderFormDesigner from '../components/work-order/WorkOrderFormDesigner.vue'
import WorkOrderFormFields from '../components/work-order/WorkOrderFormFields.vue'
import WorkOrderWorkflowEditor from '../components/work-order/WorkOrderWorkflowEditor.vue'
import ListPagination from '../components/ListPagination.vue'
import { loadAllWorkOrderParticipants, workOrderApi, workOrderApiErrorMessage } from '../utils/workOrderApi.js'
import { workOrderRelationFiltersForLocale, workOrderNodeLabel } from '../utils/workOrderUi.js'
import { formatDateTime } from '../utils/dateTime.js'
import { normalizeWorkOrderHistoricalGuidance } from '../utils/workOrderHistoricalGuidance.js'

const authStore = useAuthStore()
const { locale } = useI18n()
const route = useRoute()
const router = useRouter()
const currentLang = computed(() => String(locale.value || 'zh').toLowerCase().startsWith('en') ? 'en' : 'zh')
const workOrderCopy = {
  zh: {
    pageTitle: '工单中心', pageSubtitle: '统一处理人工、规则、告警、AI 与外部业务工单', refresh: '刷新', newWorkOrder: '新建工单', newTemplate: '新建模板', ordersTab: '工单', templatesTab: '表单与流程模板', relatedViews: '与当前用户相关的工单视图', relatedToMe: '与我相关', searchPlaceholder: '搜索工单编号、标题或摘要', allStatuses: '全部状态', allSources: '全部来源', sourceManual: '人工创建', sourceRule: '规则引擎', sourceAlarm: '告警', sourceAI: 'AI 大脑', sourceExternal: '外部业务', query: '查询', columnWorkOrder: '工单', columnSource: '来源', columnPriority: '优先级', columnStatus: '状态', columnAssignee: '负责人', columnCreatedAt: '创建时间', columnActions: '操作', loading: '加载中', noOrders: '暂无匹配工单', editOrder: '编辑工单', deleteOrder: '删除工单', edit: '编辑', delete: '删除', loadMore: '加载更多', claim: '领取', formContent: '表单内容', workOrderActions: '工单操作', actionHint: '处理动作将在弹窗中完成，并写入必填处理意见。', cc: '抄送', transfer: '转交', transferTitle: '转交工单', transferHint: '转交后，当前待处理事项将交由所选成员继续处理，并保留完整的流转记录。审批任务不能转交。', transferTask: '待处理任务', transferRecipient: '转交给', selectMember: '请选择成员', transferReason: '转交说明', transferReasonPlaceholder: '请说明转交原因或已完成的工作', confirmTransfer: '确认转交', transferSuccess: '工单已转交给指定成员。', myTasks: '分配给我的流程任务', approval: '审批', task: '处理', selectOrder: '选择一张工单查看详情', templateName: '模板名称', noTemplates: '暂无模板，请先创建', configureTemplate: '配置模板', configureWorkflowForm: '配置流程与表单', editTemplate: '编辑模板', editBasicInfo: '编辑基础信息', deleteTemplate: '删除模板', integrationInfo: '连接凭据仅在保存时提交；页面不会回显密钥。绑定用于收件幂等与版本顺序，映射用于声明外部字段转换。', name: '名称', exampleCMMS: '例如 CMMS', remoteIdentity: '远端身份标识', secretSaveOnly: '仅保存时提交', saveConnector: '保存连接', enabled: '已启用', disabled: '已停用', noConnectors: '尚未配置连接。', bindingID: '绑定标识', exampleBinding: '例如 cmms-main', selectConnector: '选择连接', direction: '方向', inbound: '入站', outbound: '出站', bidirectional: '双向', saveBinding: '保存绑定', noBindings: '先创建一个连接和绑定。', mappingID: '映射标识', exampleMapping: '例如 incident-v1', externalMapping: '外部事件映射', mappingJSON: '字段映射 JSON', mappingExample: '示例：{"title":"summary","priority":"severity"}', saveMapping: '保存映射', selectBinding: '选择左侧绑定后即可配置字段映射。', visualWorkflow: '图形化工单流程', newManualOrder: '新建人工工单', workOrderTemplate: '工单模板', selectTemplate: '请选择模板', priority: '优先级', low: '低', normal: '普通', high: '高', urgent: '紧急', title: '标题', summary: '摘要', createWorkOrder: '创建工单', editOrderTitle: '编辑工单基础信息', save: '保存', editTemplateTitle: '编辑模板基础信息', templateNameHint: '提示：模板编码和结构需保持稳定，仅支持修改名称。若需修改编码，请新建模板。', newTemplateTitle: '新建工单模板', description: '说明', createTemplateHint: '创建后会自动生成可编辑的初始表单与基础处理流程。', create: '创建', ccTitle: '抄送工单', selectCC: '选择抄送人', searchMembers: '搜索成员...', noMembers: '没有找到匹配的成员。', additionalMessage: '附言', optional: '（选填）', ccMessagePlaceholder: '给抄送人的留言...', send: '发送', start: '开始', handleTask: '处理任务', end: '结束', templateConfigType: '模板配置类型', workflowConfig: '流程配置', formConfig: '表单配置', managedReadonly: 'AI 托管模板，不能修改', workflowActions: '流程配置操作', formActions: '表单配置操作', saveWorkflowDraft: '保存流程草稿', publishWorkflow: '发布流程', fullscreenWorkflow: '全屏展开流程图', saveFormDraft: '保存表单草稿', publishForm: '发布表单', resolution: '解决结论', actualProblem: '实际问题', rootCause: '根因', handlingProcess: '处理过程', handlingResult: '处理结果', processTitle: '处理工单', processSubtitle: '选择动作并填写本次处理意见', close: '关闭', processAction: '处理动作', resolutionHint: '解决前请完整记录实际问题、根因、处理过程和处理结果；这些内容将写入不可变的工单事件。', processOpinion: '处理意见', processOpinionPlaceholder: '请填写本次处理判断、过程或结论', processOpinionHint: '处理意见将进入工单操作记录，提交后不可省略。', cancel: '取消', submitProcess: '确认处理', statusAction: '状态动作', approve: '通过审批', reject: '驳回审批', pass: '通过', complete: '完成', unavailableUser: '已删除或不可用用户', unclaimed: '未领取', integrations: '集成配置', connector: '连接', binding: '绑定', mapping: '映射', statusOpen: '待处理', statusInProgress: '处理中', statusPaused: '已暂停', statusResolved: '已解决', statusClosed: '已关闭', statusCancelled: '已取消', yes: '是', no: '否', copiedSuccess: '工单已成功抄送', connectorSaved: '集成连接已保存。', bindingSaved: '集成绑定已保存。', invalidMappingJSON: '字段映射必须是有效 JSON。', mappingSaved: '字段映射已保存。', noTemplateConfigured: '请先创建并配置一个工单模板。', orderCreated: '工单已创建。', existingOrderReturned: '已返回同一来源的既有工单。', orderClaimed: '已领取工单。', orderResolved: '工单已解决，等待验收关闭。', orderStatusUpdated: '工单状态已更新。', approvalPassed: '审批已通过。', approvalRejected: '审批已驳回。', taskCompleted: '任务已完成，流程已继续。', taskRejected: '审批已驳回，流程已退回指定节点。', versionConflict: '工单已被其他用户更新，已刷新为最新状态。', orderUpdated: '工单基础信息已更新', confirmDeleteOrder: '确定要删除未处理工单“{name}”吗？删除后将无法恢复。', orderDeleted: '工单已删除', templateUpdated: '模板名称已更新', confirmDeleteTemplate: '确定要删除工单模板“{name}”？\n注意：如果已有工单使用了此模板，强制删除可能会导致相关工单数据显示异常！', templateDeleted: '模板已删除', templateCreated: '模板已创建，并已生成初始表单与流程。', formPublishedReloadFailed: '表单已发布，但重新加载模板详情失败：{message}', formPublished: '表单已发布；既有工单继续使用原始快照。', formDraftReloadFailed: '表单草稿已保存，但重新加载模板详情失败：{message}', formDraftSaved: '表单草稿已保存，尚未发布。', workflowPublishedReloadFailed: '流程已发布，但重新加载模板详情失败：{message}', workflowPublished: '流程已发布；既有工单继续使用原始快照。', workflowDraftReloadFailed: '流程草稿已保存，但重新加载模板详情失败：{message}', workflowDraftSaved: '流程草稿已保存，尚未发布。'
  },
  en: {
    pageTitle: 'Work Order Center', pageSubtitle: 'Manage manual, rule, alarm, AI, and external work orders in one place', refresh: 'Refresh', newWorkOrder: 'New work order', newTemplate: 'New template', ordersTab: 'Work orders', templatesTab: 'Form & workflow templates', relatedViews: 'Work orders related to the current user', relatedToMe: 'Related to me', searchPlaceholder: 'Search by code, title, or summary', allStatuses: 'All statuses', allSources: 'All sources', sourceManual: 'Manual', sourceRule: 'Rule engine', sourceAlarm: 'Alarm', sourceAI: 'AI Brain', sourceExternal: 'External', query: 'Search', columnWorkOrder: 'Work order', columnSource: 'Source', columnPriority: 'Priority', columnStatus: 'Status', columnAssignee: 'Assignee', columnCreatedAt: 'Created at', columnActions: 'Actions', loading: 'Loading', noOrders: 'No matching work orders', editOrder: 'Edit work order', deleteOrder: 'Delete work order', edit: 'Edit', delete: 'Delete', loadMore: 'Load more', claim: 'Claim', formContent: 'Form data', workOrderActions: 'Work order actions', actionHint: 'Processing opens in a dialog and requires an opinion.', cc: 'Copy', transfer: 'Transfer', transferTitle: 'Transfer work order', transferHint: 'The current handling item moves to the selected member and the complete routing history is preserved. Approval tasks cannot be transferred.', transferTask: 'Handling task', transferRecipient: 'Transfer to', selectMember: 'Select a member', transferReason: 'Transfer reason', transferReasonPlaceholder: 'Explain the reason or work already completed', confirmTransfer: 'Confirm transfer', transferSuccess: 'Work order transferred to the selected member.', myTasks: 'Workflow tasks assigned to me', approval: 'Approval', task: 'Task', selectOrder: 'Select a work order to view details', templateName: 'Template name', noTemplates: 'No templates yet. Create one first.', configureTemplate: 'Configure template', configureWorkflowForm: 'Configure workflow and form', editTemplate: 'Edit template', editBasicInfo: 'Edit basic information', deleteTemplate: 'Delete template', integrationInfo: 'Credentials are submitted only when saved and are never displayed again. Bindings control inbound idempotency and revision order; mappings define external field conversion.', name: 'Name', exampleCMMS: 'For example, CMMS', remoteIdentity: 'Remote identity', secretSaveOnly: 'Submitted only when saved', saveConnector: 'Save connector', enabled: 'Enabled', disabled: 'Disabled', noConnectors: 'No connectors configured.', bindingID: 'Binding ID', exampleBinding: 'For example, cmms-main', selectConnector: 'Select connector', direction: 'Direction', inbound: 'Inbound', outbound: 'Outbound', bidirectional: 'Bidirectional', saveBinding: 'Save binding', noBindings: 'Create a connector and binding first.', mappingID: 'Mapping ID', exampleMapping: 'For example, incident-v1', externalMapping: 'External event mapping', mappingJSON: 'Field mapping JSON', mappingExample: 'Example: {"title":"summary","priority":"severity"}', saveMapping: 'Save mapping', selectBinding: 'Select a binding on the left to configure field mapping.', visualWorkflow: 'Visual work-order workflow', newManualOrder: 'New manual work order', workOrderTemplate: 'Work order template', selectTemplate: 'Select a template', priority: 'Priority', low: 'Low', normal: 'Normal', high: 'High', urgent: 'Urgent', title: 'Title', summary: 'Summary', createWorkOrder: 'Create work order', editOrderTitle: 'Edit work order details', save: 'Save', editTemplateTitle: 'Edit template details', templateNameHint: 'Template codes and structures must remain stable. Only the name can be changed; create a new template to use another code.', newTemplateTitle: 'New work-order template', description: 'Description', createTemplateHint: 'An editable initial form and basic workflow are generated after creation.', create: 'Create', ccTitle: 'Copy work order', selectCC: 'Select recipients', searchMembers: 'Search members...', noMembers: 'No matching members found.', additionalMessage: 'Message', optional: '(optional)', ccMessagePlaceholder: 'Add a message for the recipients...', send: 'Send', start: 'Start', handleTask: 'Handle task', end: 'End', templateConfigType: 'Template configuration', workflowConfig: 'Workflow', formConfig: 'Form design', managedReadonly: 'This AI-managed template is read-only', workflowActions: 'Workflow actions', formActions: 'Form actions', saveWorkflowDraft: 'Save workflow draft', publishWorkflow: 'Publish workflow', fullscreenWorkflow: 'Open workflow in full screen', saveFormDraft: 'Save form draft', publishForm: 'Publish form', resolution: 'Resolution', actualProblem: 'Actual issue', rootCause: 'Root cause', handlingProcess: 'Handling process', handlingResult: 'Result', processTitle: 'Process work order', processSubtitle: 'Choose an action and record your processing opinion', close: 'Close', processAction: 'Action', resolutionHint: 'Record the actual issue, root cause, handling process, and result before resolving. These details become immutable work-order events.', processOpinion: 'Processing opinion', processOpinionPlaceholder: 'Describe the decision, process, or result', processOpinionHint: 'The opinion is recorded in the activity log and cannot be omitted.', cancel: 'Cancel', submitProcess: 'Submit', statusAction: 'Status action', approve: 'Approve', reject: 'Reject', pass: 'Approve', complete: 'Complete', unavailableUser: 'Deleted or unavailable user', unclaimed: 'Unclaimed', integrations: 'Integrations', connector: 'Connector', binding: 'Binding', mapping: 'Mapping', statusOpen: 'Open', statusInProgress: 'In progress', statusPaused: 'Paused', statusResolved: 'Resolved', statusClosed: 'Closed', statusCancelled: 'Cancelled', yes: 'Yes', no: 'No', copiedSuccess: 'Work order copied successfully.', connectorSaved: 'Connector saved.', bindingSaved: 'Binding saved.', invalidMappingJSON: 'Field mapping must be valid JSON.', mappingSaved: 'Mapping saved.', noTemplateConfigured: 'Create and configure a work-order template first.', orderCreated: 'Work order created.', existingOrderReturned: 'Returned the existing work order for the same source.', orderClaimed: 'Work order claimed.', orderResolved: 'Work order resolved and awaiting acceptance and closure.', orderStatusUpdated: 'Work-order status updated.', approvalPassed: 'Approval passed.', approvalRejected: 'Approval rejected.', taskCompleted: 'Task completed and workflow continued.', taskRejected: 'Approval rejected and workflow returned to the configured step.', versionConflict: 'Another user updated this work order. The latest version has been loaded.', orderUpdated: 'Work-order details updated.', confirmDeleteOrder: 'Delete the unprocessed work order “{name}”? This cannot be undone.', orderDeleted: 'Work order deleted.', templateUpdated: 'Template name updated.', confirmDeleteTemplate: 'Delete work-order template “{name}”?\nIf existing work orders use it, forced deletion may make their data display incorrectly.', templateDeleted: 'Template deleted.', templateCreated: 'Template created with an initial form and workflow.', formPublishedReloadFailed: 'The form was published, but template details could not be reloaded: {message}', formPublished: 'Form published. Existing work orders keep their original snapshot.', formDraftReloadFailed: 'The form draft was saved, but template details could not be reloaded: {message}', formDraftSaved: 'Form draft saved but not published.', workflowPublishedReloadFailed: 'The workflow was published, but template details could not be reloaded: {message}', workflowPublished: 'Workflow published. Existing work orders keep their original snapshot.', workflowDraftReloadFailed: 'The workflow draft was saved, but template details could not be reloaded: {message}', workflowDraftSaved: 'Workflow draft saved but not published.'
  }
}
Object.assign(workOrderCopy.zh, {
	copyTemplate: '复制模板',
	templateCopied: '已复制模板“{name}”，可继续编辑。',
	managedFormReadonly: '“AI大脑建议”的内置表单由系统维护，不能修改。',
	managedWorkflowEditable: '可配置并发布“AI大脑建议”的工单流转流程。',
	aiWorkflowNeedsPublish: '请先配置并发布流转流程；未发布前，AI 大脑不能创建工单。',
  columnHandler: '当前处理人',
  claimedBy: '领取人：',
  currentHandler: '当前：',
  lastHandler: '最后处理人：',
  completed: '已完成',
  archived: '已归档',
  pendingAcceptance: '待验收',
  cancelledHandler: '已取消',
  statusClosed: '已归档',
  archiveOrder: '归档工单',
  restoreArchived: '撤销归档',
  orderResolved: '工单已解决，等待验收归档。',
  orderArchived: '工单已验收并归档。',
  alarmEvidence: '告警信息',
  alarmDevice: '设备',
  alarmEvent: '事件',
  autoCreatedFromAlarm: '由告警自动创建：{device} · {event}',
  historicalGuidance: '历史处置参考',
  historicalGuidanceHint: '来自同一设备、同类故障证据的已验证处置记忆，仅供本次处理参考，请结合现场情况判断。',
  historicalExperience: '处置经验 {index}',
  historicalVerifiedCount: '已验证 {count} 次',
  handlingOpinion: '处理记录',
  attachments: '附件',
  uploadAttachment: '上传附件',
  attachmentHint: '支持图片（JPG, PNG, GIF等）和文档（PDF, Word, Excel, PPT, ZIP, TXT等），单文件最大 25MB',
  uploadFailed: '附件上传失败',
  fileTooLarge: '文件超出大小限制（最大 25MB）',
  invalidFileType: '不支持的文件格式'
})
Object.assign(workOrderCopy.en, {
	copyTemplate: 'Copy template',
	templateCopied: 'Template “{name}” was copied and is ready to edit.',
	managedFormReadonly: 'The built-in form for “AI Brain Suggestions” is maintained by the system and cannot be changed.',
	managedWorkflowEditable: 'Configure and publish the workflow for “AI Brain Suggestions”.',
	aiWorkflowNeedsPublish: 'Configure and publish the workflow before AI Brain can create work orders.',
  columnHandler: 'Current handler',
  claimedBy: 'Claimed by: ',
  currentHandler: 'Current: ',
  lastHandler: 'Last handled by: ',
  completed: 'Completed',
  archived: 'Archived',
  pendingAcceptance: 'Pending acceptance',
  cancelledHandler: 'Cancelled',
  statusClosed: 'Archived',
  archiveOrder: 'Archive work order',
  restoreArchived: 'Restore archived work order',
  orderResolved: 'Work order resolved and awaiting acceptance and archival.',
  orderArchived: 'Work order accepted and archived.',
  alarmEvidence: 'Alarm information',
  alarmDevice: 'Device',
  alarmEvent: 'Event',
  autoCreatedFromAlarm: 'Automatically created from alarm: {device} · {event}',
  historicalGuidance: 'Historical resolution reference',
  historicalGuidanceHint: 'Verified handling history for the same device and fault evidence. Use it as guidance and confirm conditions on site.',
  historicalExperience: 'Resolution experience {index}',
  historicalVerifiedCount: 'Verified {count} times',
  handlingOpinion: 'Handling record',
  attachments: 'Attachments',
  uploadAttachment: 'Upload attachment',
  attachmentHint: 'Supports images (JPG, PNG, GIF, etc.) and documents (PDF, Word, Excel, PPT, ZIP, TXT, etc.), max 25MB per file.',
  uploadFailed: 'Failed to upload attachment',
  fileTooLarge: 'File exceeds size limit (max 25MB)',
  invalidFileType: 'Unsupported file type'
})
const workOrderPaginationCopy = {
  zh: { itemsPerPage: '每页显示', pagination: '工单列表分页', previous: '上一页', next: '下一页', goToPage: '前往第 {page} 页' },
  en: { itemsPerPage: 'Rows per page', pagination: 'Work-order list pagination', previous: 'Previous', next: 'Next', goToPage: 'Go to page {page}' }
}
const tx = key => workOrderPaginationCopy[currentLang.value]?.[key] || workOrderCopy[currentLang.value]?.[key] || workOrderCopy.zh[key] || key
const txf = (key, values = {}) => Object.entries(values).reduce((message, [name, value]) => message.replaceAll(`{${name}}`, String(value)), tx(key))
const relationFilters = computed(() => workOrderRelationFiltersForLocale(currentLang.value))
const workOrderIntegrationsVisible = false
const activeTab = ref('orders')
const loading = ref(false)
const actionLoading = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
let feedbackDismissTimer = null

watch([errorMessage, successMessage], ([nextErrorMessage, nextSuccessMessage]) => {
  if (feedbackDismissTimer) {
    window.clearTimeout(feedbackDismissTimer)
    feedbackDismissTimer = null
  }
  if (!nextErrorMessage && !nextSuccessMessage) return
  feedbackDismissTimer = window.setTimeout(() => {
    errorMessage.value = ''
    successMessage.value = ''
    feedbackDismissTimer = null
  }, 4500)
})

onBeforeUnmount(() => {
  if (feedbackDismissTimer) window.clearTimeout(feedbackDismissTimer)
})

const templates = ref([])
const orders = ref([])
const orderTotal = ref(0)
const orderPage = ref(1)
const orderPageSize = ref(20)
const orderPageSizeOptions = [10, 20, 50]
const selectedOrder = ref(null)
const orderDetailDialog = ref(null)
const detailReturnFocus = ref(null)
const workOrderCatalogDevices = ref([])
const workOrderCatalogProducts = ref([])
const highlightedWorkOrderPublicID = ref('')
const orderEvents = ref([])
const approvalTasks = ref([])
const workOrderTasks = ref([])
const selectedTemplate = ref(null)
const templateDetail = ref(null)
const templateDetailDialog = ref(null)
const templateDetailReturnFocus = ref(null)
const formDraft = ref({ fields: [] })
const workflowDraft = ref(defaultGraphWorkflow())
const workflowEditorRef = ref(null)
const projectParticipants = ref([])
const participantsLoading = ref(false)

const showCCModal = ref(false)
const ccForm = ref({ userIDs: [], comment: '' })
const ccSearchKeyword = ref('')
const showTransferModal = ref(false)
const transferForm = ref({ taskPublicID: '', targetUserID: '', comment: '' })
const showProcessModal = ref(false)
const selectedProcessActionID = ref('')
const processOpinion = ref('')
const resolutionDraft = ref({ actual_problem: '', root_cause: '', handling_process: '', handling_result: '' })
const processAttachments = ref([])
const processAttachmentUploading = ref(false)
const processAttachmentError = ref('')
const connectors = ref([])
const bindings = ref([])
const mappings = ref([])
const selectedBindingID = ref('')
const connectorDraft = ref({ name: '', key_id: '', secret: '' })
const bindingDraft = ref({ id: '', connector_id: '', direction: 'inbound' })
const mappingDraft = ref({ id: '', name: '', fieldsJSON: '{\n  "title": "summary"\n}' })

const ccFilteredParticipants = computed(() => {
  if (!ccSearchKeyword.value) return projectParticipants.value
  const kw = ccSearchKeyword.value.toLowerCase()
  return projectParticipants.value.filter(p => (p.display_name || '').toLowerCase().includes(kw) || (p.username || '').toLowerCase().includes(kw))
})
const transferFilteredParticipants = computed(() => projectParticipants.value.filter(participant => String(participant.id) !== String(currentUserId.value)))
const isResolutionComplete = computed(() => Object.values(resolutionDraft.value).every(value => String(value || '').trim()))
const assigneeNames = computed(() => new Map(projectParticipants.value.map(participant => [String(participant.id), participant.display_name || participant.username || tx('unavailableUser')])))

function openCCModal() {
  if (projectParticipants.value.length === 0) {
    loadProjectParticipants()
  }
  ccForm.value = { userIDs: [], comment: '' }
  ccSearchKeyword.value = ''
  showCCModal.value = true
}

async function submitCC() {
  if (!selectedOrder.value || ccForm.value.userIDs.length === 0) return
  actionLoading.value = true
  try {
    const response = await axios.post(`/api/work-orders/${selectedOrder.value.work_order.ID}/cc`, ccForm.value)
    if (response.data.code !== 0) throw new Error(response.data.message)
    successMessage.value = tx('copiedSuccess')
    showCCModal.value = false
    await selectOrder(selectedOrder.value.work_order.ID)
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

function openTransferModal() {
  if (!canTransferOrder.value || !selectedOrder.value) return
  if (projectParticipants.value.length === 0) loadProjectParticipants()
  transferForm.value = {
    taskPublicID: transferableHandleTasks.value[0]?.public_id || '',
    targetUserID: '',
    comment: ''
  }
  showTransferModal.value = true
}

function closeTransferModal() {
  if (actionLoading.value) return
  showTransferModal.value = false
  transferForm.value = { taskPublicID: '', targetUserID: '', comment: '' }
}

async function submitTransfer() {
  if (!selectedOrder.value || !transferForm.value.targetUserID || !transferForm.value.comment.trim()) return
  const payload = {
    target_user_id: Number(transferForm.value.targetUserID),
    task_public_id: transferForm.value.taskPublicID || undefined,
    comment: transferForm.value.comment.trim(),
    expected_version: selectedOrder.value.work_order.version
  }
  if (await executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/transfer`, payload, tx('transferSuccess'))) closeTransferModal()
}

const showOrderModal = ref(false)
const showTemplateModal = ref(false)
const showTemplateEditor = ref(false)
const templateEditorTab = ref('workflow')
const createTemplateDetail = ref(null)
const isAIManagedTemplate = computed(() => selectedTemplate.value?.template?.system_managed === true && selectedTemplate.value.template.code === 'ai_device_maintenance')
const workflowEditorReadonly = computed(() => selectedTemplate.value?.template?.system_managed === true && !isAIManagedTemplate.value)

const orderFilters = ref({ keyword: '', status: '', sourceType: '', relation: '' })
const createOrder = ref({ templateId: 0, title: '', summary: '', priority: 'normal', formData: {} })
const createTemplate = ref({ code: '', name: '', description: '' })
const showEditOrderModal = ref(false)
const showEditTemplateModal = ref(false)
const editOrderData = ref({})
const editTemplateData = ref({})

const workflowStatuses = computed(() => {
  const seen = new Map()
  ;[[
    { key: 'open', name: tx('statusOpen') }, { key: 'in_progress', name: tx('statusInProgress') }, { key: 'paused', name: tx('statusPaused') }, { key: 'resolved', name: tx('statusResolved') }, { key: 'closed', name: tx('statusClosed') }, { key: 'cancelled', name: tx('statusCancelled') }
  ], ...(workflowDraft.value.statuses ? [workflowDraft.value.statuses] : []), ...orders.value.map(entry => entry.workflow?.statuses || [])].flat().forEach(status => {
    if (status?.key && !seen.has(status.key)) seen.set(status.key, { ...status, name: localizedStatusName(status.key, status.name) })
  })
  return [...seen.values()]
})
const filteredOrders = computed(() => orders.value)
const availableTransitions = computed(() => selectedOrder.value?.available_actions?.transitions || [])
const workOrderDeviceByCode = computed(() => new Map(workOrderCatalogDevices.value.map(device => [String(device.code || '').trim(), device])))
const workOrderProductByCode = computed(() => new Map(workOrderCatalogProducts.value.map(product => [String(product.code || '').trim(), product])))
const alarmSourceDetails = computed(() => selectedOrder.value ? alarmWorkOrderDetails(selectedOrder.value) : [])
const historicalGuidance = computed(() => normalizeWorkOrderHistoricalGuidance(selectedOrder.value))
const templateDetailWorkflow = computed(() => templateDetail.value ? normalizeGraphWorkflow(templateDetail.value.has_workflow_draft ? templateDetail.value.workflow_draft_definition : templateDetail.value.workflow_definition) : defaultGraphWorkflow())
const hasPendingApproval = computed(() => approvalTasks.value.some(task => task.status === 'pending' && String(task.approver_user_id) === String(currentUserId.value)))
const currentApprovalTask = computed(() => approvalTasks.value.find(task => task.status === 'pending') || {})
const currentUserId = computed(() => authStore.user?.id || authStore.user?.ID || authStore.user?.user_id || '')
const myPendingGraphTasks = computed(() => workOrderTasks.value.filter(task => task.status === 'pending' && String(task.assignee_user_id) === String(currentUserId.value)))
const transferableHandleTasks = computed(() => myPendingGraphTasks.value.filter(task => task.task_kind === 'handle'))
const isGraphWorkflow = computed(() => selectedOrder.value?.workflow?.schema_version === 2 && Array.isArray(selectedOrder.value?.workflow?.nodes))
const canTransferOrder = computed(() => {
  if (!authStore.hasPermission('work_order:process') || !selectedOrder.value) return false
  if (isGraphWorkflow.value) return transferableHandleTasks.value.length > 0
  return String(selectedOrder.value.work_order?.assignee_user_id || '') === String(currentUserId.value)
})
const processActionOptions = computed(() => {
  const actions = []
  if (authStore.hasPermission('work_order:process')) {
    for (const transition of availableTransitions.value) {
      actions.push({ id: `transition:${transition.key}`, kind: 'transition', transition, requiresResolution: Boolean(transition.requires_resolution), label: transitionLabel(transition) })
    }
  }
  if (hasPendingApproval.value && authStore.hasPermission('work_order:approve')) {
    actions.push({ id: 'approval:approve', kind: 'approve', label: tx('approve') })
    actions.push({ id: 'approval:reject', kind: 'reject', label: tx('reject') })
  }
  for (const task of myPendingGraphTasks.value) {
    const nodeName = workOrderNodeLabel(selectedOrder.value?.workflow, task.node_id, currentLang.value)
    if (authStore.hasPermission('work_order:process')) {
      actions.push({ id: `task:${task.public_id}:complete`, kind: 'task_complete', task, label: `${task.task_kind === 'approve' ? tx('pass') : tx('complete')}: ${nodeName}` })
    }
    if (task.task_kind === 'approve' && authStore.hasPermission('work_order:approve')) {
      actions.push({ id: `task:${task.public_id}:reject`, kind: 'task_reject', task, label: `${tx('reject')}: ${nodeName}` })
    }
  }
  return actions
})
const currentProcessAction = computed(() => processActionOptions.value.find(action => action.id === selectedProcessActionID.value) || null)
const uiPermissions = computed(() => ['work_order:create', 'work_order:process', 'work_order:approve', 'work_order:manage', 'work_order:integration'].filter(permission => authStore.hasPermission(permission)))

watch(activeTab, (tab) => {
  if (tab === 'templates') loadTemplates()
  if (workOrderIntegrationsVisible && tab === 'integrations') loadIntegrations()
})

function apiError(error) {
  return workOrderApiErrorMessage(error)
}

function assigneeLabel(userID) {
  if (!userID) return tx('unclaimed')
  return assigneeNames.value.get(String(userID)) || tx('unavailableUser')
}
function handlerDisplayName(handler) { return String(handler?.display_name || '').trim() || tx('unavailableUser') }
function effectiveWorkOrderStatus(order) {
  const status = String(order?.status || '').trim()
  return status === 'resolved' && order?.closed_at ? 'closed' : status
}
function transitionLabel(transition) {
  if (transition?.key === 'close') return tx('archiveOrder')
  if (transition?.key === 'reopen') return tx('restoreArchived')
  return transition?.name || tx('statusAction')
}
function responsibilityLabel(entry) {
  const order = entry?.work_order || {}
  const responsibility = entry?.responsibility || {}
  const status = effectiveWorkOrderStatus(order)
  if (status === 'closed') return tx('archived')
  if (status === 'resolved') return tx('pendingAcceptance')
  if (status === 'cancelled') return tx('cancelledHandler')
  const currentHandlers = Array.isArray(responsibility.current_handlers) ? responsibility.current_handlers : []
  if (currentHandlers.length) return currentHandlers.map(handlerDisplayName).join(currentLang.value === 'en' ? ', ' : '、')
  if (responsibility.claimed_handler) return handlerDisplayName(responsibility.claimed_handler)
  return tx('unclaimed')
}

function sourceSnapshotValue(snapshot, key) {
  if (snapshot?.[key] !== undefined) return snapshot[key]
  return snapshot?.params?.[key]
}

function workOrderProductModel(product) {
  let model = product?.model ?? product?.config ?? {}
  if (typeof model === 'string') {
    try { model = JSON.parse(model || '{}') } catch (_) { model = {} }
  }
  return model?.tsl || model?.model || model || {}
}

function workOrderEventID(event) { return String(event?.identifier || event?.id || event?.key || '').trim() }
function workOrderEventParameterID(parameter) { return String(parameter?.identifier || parameter?.id || parameter?.key || '').trim() }
function workOrderEventParameters(event) {
  for (const key of ['outputData', 'output_data', 'params', 'parameters', 'properties']) {
    if (Array.isArray(event?.[key])) return event[key]
  }
  return []
}
function workOrderDeviceName(code) { return String(workOrderDeviceByCode.value.get(String(code || '').trim())?.name || code || '-').trim() || '-' }
function workOrderProductCode(snapshot) { return String(sourceSnapshotValue(snapshot, 'product_code') || workOrderDeviceByCode.value.get(String(sourceSnapshotValue(snapshot, 'device_code') || '').trim())?.product_code || '').trim() }
function workOrderEventDefinition(snapshot) {
  const eventID = String(sourceSnapshotValue(snapshot, 'event_id') || '').trim()
  const product = workOrderProductByCode.value.get(workOrderProductCode(snapshot))
  return (workOrderProductModel(product).events || []).find(event => workOrderEventID(event) === eventID)
}
function workOrderEventName(snapshot) {
  const eventID = String(sourceSnapshotValue(snapshot, 'event_id') || '').trim()
  const definition = workOrderEventDefinition(snapshot)
  return String(sourceSnapshotValue(snapshot, 'event_name') || definition?.name || eventID || '-').trim() || '-'
}
function formatWorkOrderSourceValue(value) { return value && typeof value === 'object' ? JSON.stringify(value) : String(value ?? '-') }
function alarmWorkOrderDetails(detail) {
  if (detail?.work_order?.source_type !== 'alarm') return []
  const snapshot = detail?.source_snapshot || {}
  const deviceCode = String(sourceSnapshotValue(snapshot, 'device_code') || '').trim()
  const eventID = String(sourceSnapshotValue(snapshot, 'event_id') || '').trim()
  const values = []
  if (deviceCode) values.push({ id: 'device', label: tx('alarmDevice'), value: workOrderDeviceName(deviceCode) })
  if (eventID) values.push({ id: 'event', label: tx('alarmEvent'), value: workOrderEventName(snapshot) })
  const parameters = snapshot?.params && typeof snapshot.params === 'object' && !Array.isArray(snapshot.params) ? snapshot.params : snapshot
  const definitions = new Map(workOrderEventParameters(workOrderEventDefinition(snapshot)).map(parameter => [workOrderEventParameterID(parameter), parameter]))
  for (const [id, value] of Object.entries(parameters || {})) {
    if (String(id).startsWith('_') || ['device_code', 'device_name', 'product_code', 'product_name', 'event_id', 'event_name'].includes(id)) continue
    const definition = definitions.get(String(id))
    const label = `${String(definition?.name || id).trim() || id}${currentLang.value === 'en' ? ` (${id})` : `（${id}）`}`
    values.push({ id, label, value: formatWorkOrderSourceValue(value) })
  }
  return values
}
function workOrderSummary(detail) {
  const summary = String(detail?.work_order?.summary || '').trim()
  if (detail?.work_order?.source_type !== 'alarm') return summary
  const snapshot = detail?.source_snapshot || {}
  const deviceCode = String(sourceSnapshotValue(snapshot, 'device_code') || '').trim()
  const eventID = String(sourceSnapshotValue(snapshot, 'event_id') || '').trim()
  if (!deviceCode && !eventID) return summary
  return txf('autoCreatedFromAlarm', { device: deviceCode ? workOrderDeviceName(deviceCode) : '-', event: eventID ? workOrderEventName(snapshot) : '-' })
}

async function loadWorkOrderEntityCatalog() {
  const [productsResult, devicesResult] = await Promise.allSettled([
    axios.get('/api/products', { params: { page: 1, pageSize: 1000 } }),
    axios.get('/api/devices', { params: { page: 1, pageSize: 1000 } })
  ])
  if (productsResult.status === 'fulfilled' && productsResult.value.data?.code === 0) workOrderCatalogProducts.value = productsResult.value.data?.data || []
  if (devicesResult.status === 'fulfilled' && devicesResult.value.data?.code === 0) workOrderCatalogDevices.value = devicesResult.value.data?.data || []
}

async function loadIntegrations() {
  loading.value = true
  try {
    const [connectorResponse, bindingResponse] = await Promise.all([
      workOrderApi.integrationConnectors(), workOrderApi.integrationBindings()
    ])
    if (connectorResponse.data.code !== 0) throw new Error(connectorResponse.data.message)
    if (bindingResponse.data.code !== 0) throw new Error(bindingResponse.data.message)
    connectors.value = connectorResponse.data.data || []
    bindings.value = bindingResponse.data.data || []
    if (selectedBindingID.value && !bindings.value.some(binding => binding.id === selectedBindingID.value)) {
      selectedBindingID.value = ''
      mappings.value = []
    }
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    loading.value = false
  }
}

async function saveConnector() {
  actionLoading.value = true
  try {
    const response = await axios.post('/api/work-order-integrations/connectors', connectorDraft.value)
    if (response.data.code !== 0) throw new Error(response.data.message)
    connectorDraft.value = { name: '', key_id: '', secret: '' }
    successMessage.value = tx('connectorSaved')
    await loadIntegrations()
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function saveBinding() {
  actionLoading.value = true
  try {
    const response = await axios.post('/api/work-order-integrations/bindings', bindingDraft.value)
    if (response.data.code !== 0) throw new Error(response.data.message)
    selectedBindingID.value = response.data.data.id
    bindingDraft.value = { id: '', connector_id: '', direction: 'inbound' }
    successMessage.value = tx('bindingSaved')
    await loadIntegrations()
    await selectBinding(selectedBindingID.value)
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function selectBinding(bindingID) {
  selectedBindingID.value = bindingID
  mappings.value = []
  if (!bindingID) return
  try {
    const response = await workOrderApi.integrationMappings(bindingID)
    if (response.data.code !== 0) throw new Error(response.data.message)
    mappings.value = response.data.data || []
  } catch (error) {
    errorMessage.value = apiError(error)
  }
}

async function saveMapping() {
  let fields
  try {
    fields = JSON.parse(mappingDraft.value.fieldsJSON)
  } catch {
    errorMessage.value = tx('invalidMappingJSON')
    return
  }
  actionLoading.value = true
  try {
    const response = await axios.post(`/api/work-order-integrations/bindings/${selectedBindingID.value}/mappings`, {
      id: mappingDraft.value.id, name: mappingDraft.value.name, revision: 1, fields, status: {}, priority: {}
    })
    if (response.data.code !== 0) throw new Error(response.data.message)
    mappingDraft.value = { id: '', name: '', fieldsJSON: '{\n  "title": "summary"\n}' }
    successMessage.value = tx('mappingSaved')
    await selectBinding(selectedBindingID.value)
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function loadTemplates() {
  try {
    const response = await workOrderApi.templates()
    if (response.data.code !== 0) throw new Error(response.data.message)
    templates.value = response.data.data || []
    if (selectedTemplate.value) {
      const current = templates.value.find(template => template.ID === selectedTemplate.value.template.ID)
      if (!current) selectedTemplate.value = null
    }
  } catch (error) {
    errorMessage.value = apiError(error)
  }
}

async function loadProjectParticipants() {
  participantsLoading.value = true
  try {
    projectParticipants.value = await loadAllWorkOrderParticipants()
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    participantsLoading.value = false
  }
}

async function loadOrders() {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await workOrderApi.list({ page: orderPage.value, pageSize: orderPageSize.value, search: orderFilters.value.keyword, status: orderFilters.value.status, source_type: orderFilters.value.sourceType, relation: orderFilters.value.relation })
    if (response.data.code !== 0) throw new Error(response.data.message)
    const incoming = response.data.data || []
    orders.value = incoming
    orderTotal.value = Number(response.data.total || 0)
    if (selectedOrder.value && !incoming.some(entry => workOrderId(entry.work_order) === workOrderId(selectedOrder.value?.work_order))) {
      selectedOrder.value = null
      orderEvents.value = []
      approvalTasks.value = []
      workOrderTasks.value = []
    }
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    loading.value = false
  }
}

function applyOrderFilters() {
  orderPage.value = 1
  loadOrders()
}

function applyRouteOrderFilters() {
  const sourceType = String(route.query.source || '').trim()
  if (sourceType === 'ai') orderFilters.value.sourceType = sourceType
}

function goOrderPage(target) {
  const pageCount = Math.max(1, Math.ceil(orderTotal.value / orderPageSize.value))
  const next = Math.min(pageCount, Math.max(1, Number(target) || 1))
  if (next === orderPage.value) return
  orderPage.value = next
  loadOrders()
}

function changeOrderPageSize(size) {
  orderPageSize.value = Number(size) || 10
  orderPage.value = 1
  loadOrders()
}

function applyRelationFilter(relation) {
  if (orderFilters.value.relation === relation) return
  orderFilters.value.relation = relation
  applyOrderFilters()
}

async function selectOrder(id) {
  if (!selectedOrder.value) detailReturnFocus.value = document.activeElement instanceof HTMLElement ? document.activeElement : null
  loading.value = true
  try {
    const [detailResponse, eventResponse, approvalResponse, taskResponse] = await workOrderApi.detail(id)
    if (detailResponse.data.code !== 0) throw new Error(detailResponse.data.message)
    selectedOrder.value = detailResponse.data.data
    orderEvents.value = eventResponse.data.code === 0 ? eventResponse.data.data || [] : []
    approvalTasks.value = approvalResponse.data.code === 0 ? approvalResponse.data.data || [] : []
    workOrderTasks.value = taskResponse.data.code === 0 ? taskResponse.data.data || [] : []
    nextTick(() => orderDetailDialog.value?.focus())
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    loading.value = false
  }
}

function closeOrderDetail() {
  selectedOrder.value = null
  orderEvents.value = []
  approvalTasks.value = []
  workOrderTasks.value = []
  nextTick(() => detailReturnFocus.value?.focus?.())
  detailReturnFocus.value = null
}

async function openHighlightedWorkOrder() {
  const publicID = String(route.query.highlight || '').trim()
  if (!publicID || publicID === highlightedWorkOrderPublicID.value) return
  try {
    const response = await axios.get(`/api/work-orders/public/${encodeURIComponent(publicID)}`)
    if (response.data?.code !== 0) throw new Error(response.data?.message)
    const id = workOrderId(response.data?.data?.work_order)
    if (!id) throw new Error(tx('noOrders'))
    await selectOrder(id)
    highlightedWorkOrderPublicID.value = publicID
  } catch (error) {
    errorMessage.value = apiError(error)
  }
}

async function selectTemplate(id) {
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await workOrderApi.template(id)
    if (response.data.code !== 0) throw new Error(response.data.message)
    selectedTemplate.value = response.data.data
    formDraft.value = normalizeFormDraft(selectedTemplate.value.has_form_draft ? selectedTemplate.value.form_draft_definition : (selectedTemplate.value.form_definition || { fields: [] }))
    workflowDraft.value = normalizeGraphWorkflow(selectedTemplate.value.has_workflow_draft ? selectedTemplate.value.workflow_draft_definition : selectedTemplate.value.workflow_definition)
    await loadProjectParticipants()
    return true
  } catch (error) {
    errorMessage.value = apiError(error)
    return false
  } finally {
    loading.value = false
  }
}

async function openTemplateEditor(id) {
  showTemplateEditor.value = false
  selectedTemplate.value = null
  templateEditorTab.value = 'workflow'
  await selectTemplate(id)
  if (selectedTemplate.value) showTemplateEditor.value = true
}

async function openRouteTemplateEditor() {
  const templateID = Number(route.query.template)
  if (!Number.isInteger(templateID) || templateID <= 0) return
  activeTab.value = 'templates'
  await openTemplateEditor(templateID)
  if (selectedTemplate.value?.template?.ID === templateID) {
    const { template, ...query } = route.query
    await router.replace({ query })
  }
}

async function openTemplateDetail(id) {
  if (!templateDetail.value) templateDetailReturnFocus.value = document.activeElement instanceof HTMLElement ? document.activeElement : null
  loading.value = true
  errorMessage.value = ''
  try {
    const response = await workOrderApi.template(id)
    if (response.data.code !== 0) throw new Error(response.data.message)
    templateDetail.value = response.data.data
    await loadProjectParticipants()
    nextTick(() => templateDetailDialog.value?.focus())
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    loading.value = false
  }
}

function closeTemplateDetail() {
  templateDetail.value = null
  nextTick(() => templateDetailReturnFocus.value?.focus?.())
  templateDetailReturnFocus.value = null
}

function closeTemplateEditor() {
  showTemplateEditor.value = false
}

function openCreateOrder() {
  if (templates.value.length === 0) {
    activeTab.value = 'templates'
    errorMessage.value = tx('noTemplateConfigured')
    return
  }
  createOrder.value = { templateId: templates.value[0].ID, title: '', summary: '', priority: 'normal', formData: {} }
  createTemplateDetail.value = null
  showOrderModal.value = true
  loadCreateTemplate()
}

async function loadCreateTemplate() {
  createOrder.value.formData = {}
  if (!createOrder.value.templateId) return
  try {
    const response = await workOrderApi.template(createOrder.value.templateId)
    if (response.data.code !== 0) throw new Error(response.data.message)
    createTemplateDetail.value = response.data.data
  } catch (error) {
    errorMessage.value = apiError(error)
  }
}

async function createManualOrder() {
  actionLoading.value = true
  try {
    const payload = { template_id: createOrder.value.templateId, title: createOrder.value.title, summary: createOrder.value.summary, priority: createOrder.value.priority, form_data: createOrder.value.formData }
    const response = await axios.post('/api/work-orders', payload)
    if (response.data.code !== 0) throw new Error(response.data.message)
    showOrderModal.value = false
    successMessage.value = response.data.created ? tx('orderCreated') : tx('existingOrderReturned')
    orderPage.value = 1
    await loadOrders()
    await selectOrder(workOrderId(response.data.data.work_order))
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function claimOrder() {
	await executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/claim`, { expected_version: selectedOrder.value.work_order.version }, tx('orderClaimed'))
}

function openProcessModal() {
  if (!processActionOptions.value.length) return
  selectedProcessActionID.value = processActionOptions.value[0].id
  processOpinion.value = ''
  resolutionDraft.value = { actual_problem: '', root_cause: '', handling_process: '', handling_result: '' }
  processAttachments.value = []
  processAttachmentError.value = ''
  showProcessModal.value = true
}

function closeProcessModal() {
  if (actionLoading.value) return
  showProcessModal.value = false
  selectedProcessActionID.value = ''
  processOpinion.value = ''
  processAttachments.value = []
  processAttachmentError.value = ''
}

function isImageAttachment(att) {
  if (!att) return false
  const ext = (att.type || att.name || '').toLowerCase()
  return ['.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp'].some(e => ext.endsWith(e))
}

function formatFileSize(bytes) {
  if (!bytes || isNaN(bytes)) return ''
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function getFileIcon(att) {
  const ext = (att.type || att.name || '').toLowerCase()
  if (['.pdf'].some(e => ext.endsWith(e))) return 'bi-file-earmark-pdf text-danger'
  if (['.doc', '.docx'].some(e => ext.endsWith(e))) return 'bi-file-earmark-word text-primary'
  if (['.xls', '.xlsx', '.csv'].some(e => ext.endsWith(e))) return 'bi-file-earmark-excel text-success'
  if (['.ppt', '.pptx'].some(e => ext.endsWith(e))) return 'bi-file-earmark-ppt text-warning'
  if (['.zip', '.rar', '.7z'].some(e => ext.endsWith(e))) return 'bi-file-earmark-zip text-secondary'
  if (['.txt'].some(e => ext.endsWith(e))) return 'bi-file-earmark-text text-info'
  return 'bi-file-earmark text-secondary'
}

async function uploadProcessAttachments(event) {
  const input = event.target
  const files = [...(input.files || [])]
  input.value = ''
  if (!files.length) return
  const allowedExtensions = new Set([
    '.jpg', '.jpeg', '.png', '.gif', '.webp', '.bmp',
    '.pdf', '.doc', '.docx', '.xls', '.xlsx', '.ppt', '.pptx',
    '.txt', '.csv', '.zip', '.rar', '.7z'
  ])
  if (files.some(file => {
    const ext = '.' + file.name.split('.').pop().toLowerCase()
    return !allowedExtensions.has(ext)
  })) {
    processAttachmentError.value = tx('invalidFileType')
    return
  }
  if (files.some(file => file.size > 25 * 1024 * 1024)) {
    processAttachmentError.value = tx('fileTooLarge')
    return
  }
  processAttachmentUploading.value = true
  processAttachmentError.value = ''
  try {
    for (const file of files) {
      const formData = new FormData()
      formData.append('file', file)
      const response = await workOrderApi.uploadAttachment(formData)
      if (response.data?.code !== 0 || !response.data?.data?.url) {
        throw new Error(response.data?.message || tx('uploadFailed'))
      }
      processAttachments.value.push({
        name: response.data.data.name || file.name,
        url: response.data.data.url,
        size: response.data.data.size || file.size,
        type: response.data.data.type || ('.' + file.name.split('.').pop().toLowerCase())
      })
    }
  } catch (err) {
    processAttachmentError.value = err?.message || tx('uploadFailed')
  } finally {
    processAttachmentUploading.value = false
  }
}

function removeProcessAttachment(index) {
  processAttachments.value.splice(index, 1)
}

async function transitionOrder(transition, opinion, resolution = null) {
	const payload = { key: transition.key, comment: opinion, expected_version: selectedOrder.value.work_order.version, attachments: [...processAttachments.value] }
	if (transition.requires_resolution) payload.resolution = resolution
	const success = transition.key === 'close' ? tx('orderArchived') : transition.requires_resolution ? tx('orderResolved') : tx('orderStatusUpdated')
	return executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/transitions`, payload, success)
}

async function approveOrder(opinion) {
  return executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/approve`, { comment: opinion, expected_version: selectedOrder.value.work_order.version, attachments: [...processAttachments.value] }, tx('approvalPassed'))
}

async function rejectOrder(opinion) {
  return executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/reject`, { comment: opinion, expected_version: selectedOrder.value.work_order.version, attachments: [...processAttachments.value] }, tx('approvalRejected'))
}

async function completeGraphTask(task, opinion) {
  return executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/tasks/${task.public_id}/complete`, { comment: opinion, expected_version: selectedOrder.value.work_order.version, attachments: [...processAttachments.value] }, tx('taskCompleted'))
}

async function rejectGraphTask(task, opinion) {
  return executeOrderAction(`/api/work-orders/${workOrderId(selectedOrder.value.work_order)}/tasks/${task.public_id}/reject`, { comment: opinion, expected_version: selectedOrder.value.work_order.version, attachments: [...processAttachments.value] }, tx('taskRejected'))
}

async function submitProcessAction() {
  const action = currentProcessAction.value
  const opinion = processOpinion.value.trim()
  if (!action || !opinion || !selectedOrder.value) return
  if (action.requiresResolution && !isResolutionComplete.value) return
  let succeeded = false
  if (action.kind === 'transition') succeeded = await transitionOrder(action.transition, opinion, resolutionDraft.value)
  else if (action.kind === 'approve') succeeded = await approveOrder(opinion)
  else if (action.kind === 'reject') succeeded = await rejectOrder(opinion)
  else if (action.kind === 'task_complete') succeeded = await completeGraphTask(action.task, opinion)
  else if (action.kind === 'task_reject') succeeded = await rejectGraphTask(action.task, opinion)
  if (succeeded) closeProcessModal()
}

async function executeOrderAction(url, payload, message) {
  const orderID = workOrderId(selectedOrder.value?.work_order)
  actionLoading.value = true
  try {
    const response = await axios.post(url, payload)
    if (response.data.code !== 0) throw new Error(response.data.message)
    successMessage.value = message
    await loadOrders()
    await selectOrder(orderID)
    return true
  } catch (error) {
    if (String(error?.response?.data?.error_code || '').toUpperCase() === 'VERSION_CONFLICT' && selectedOrder.value) {
      errorMessage.value = tx('versionConflict')
      await selectOrder(orderID)
      return false
    }
    errorMessage.value = apiError(error)
    return false
  } finally {
    actionLoading.value = false
  }
}


function openEditOrder(order) {
  editOrderData.value = { id: order.ID, title: order.title, summary: order.summary, priority: order.priority, version: order.version }
  showEditOrderModal.value = true
}

async function submitEditOrder() {
  actionLoading.value = true
  try {
    const payload = { ...editOrderData.value, expected_version: editOrderData.value.version }
    const response = await axios.put(`/api/work-orders/${editOrderData.value.id}`, payload)
    if (response.data.code !== 0) throw new Error(response.data.message)
    showEditOrderModal.value = false
    successMessage.value = tx('orderUpdated')
    await loadOrders()
    if (selectedOrder.value?.work_order?.ID === editOrderData.value.id) await selectOrder(editOrderData.value.id)
  } catch (error) {
    if (String(error?.response?.data?.error_code || '').toUpperCase() === 'VERSION_CONFLICT' && selectedOrder.value) {
      errorMessage.value = tx('versionConflict')
      await selectOrder(workOrderId(selectedOrder.value.work_order))
      return
    }
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function deleteOrder(order) {
  if (!confirm(txf('confirmDeleteOrder', { name: order.title }))) return
  actionLoading.value = true
  try {
    const response = await axios.delete(`/api/work-orders/${order.ID}`, { params: { expected_version: order.version } })
    if (response.data.code !== 0) throw new Error(response.data.message)
    if (selectedOrder.value?.work_order?.ID === order.ID) {
      selectedOrder.value = null
    }
    await loadOrders()
    successMessage.value = tx('orderDeleted')
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

function openEditTemplate(template) {
  editTemplateData.value = { id: template.ID, name: template.name }
  showEditTemplateModal.value = true
}

async function copyTemplate(template) {
  actionLoading.value = true
  try {
    const response = await workOrderApi.copyTemplate(template.ID)
    if (response.data.code !== 0) throw new Error(response.data.message)
    const copied = response.data.data
    successMessage.value = txf('templateCopied', { name: copied.name })
    await loadTemplates()
    await openTemplateEditor(copied.ID)
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function submitEditTemplate() {
  actionLoading.value = true
  try {
    const response = await axios.put(`/api/work-order-templates/${editTemplateData.value.id}`, editTemplateData.value)
    if (response.data.code !== 0) throw new Error(response.data.message)
    showEditTemplateModal.value = false
    successMessage.value = tx('templateUpdated')
    await loadTemplates()
    if (selectedTemplate.value?.template?.ID === editTemplateData.value.id) await selectTemplate(editTemplateData.value.id)
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

function openCreateTemplate() {
  createTemplate.value = { code: '', name: '', description: '' }
  showTemplateModal.value = true
}


async function deleteTemplate(template) {
  if (!confirm(txf('confirmDeleteTemplate', { name: template.name }))) return
  actionLoading.value = true
  try {
    const response = await axios.delete(`/api/work-order-templates/${template.ID}`)
    if (response.data.code !== 0) throw new Error(response.data.message)
    if (selectedTemplate.value?.template?.ID === template.ID) {
      selectedTemplate.value = null
    }
    await loadTemplates()
    successMessage.value = tx('templateDeleted')
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function createNewTemplate() {
  actionLoading.value = true
  if (!createTemplate.value.code) {
    createTemplate.value.code = 'tpl_' + Math.random().toString(36).substr(2, 8)
  }
  try {
    const response = await axios.post('/api/work-order-templates', createTemplate.value)
    if (response.data.code !== 0) throw new Error(response.data.message)
    showTemplateModal.value = false
    successMessage.value = tx('templateCreated')
    await loadTemplates()
    await openTemplateEditor(response.data.data.ID)
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

function normalizeFormDraft(definition) {
  const draft = cloneDefinition(definition || { fields: [] })
  draft.fields = Array.isArray(draft.fields) ? draft.fields : []
  draft.fields.forEach(normalizeFieldOptions)
  return draft
}

function normalizeFieldOptions(field) {
  if (!['select', 'multi_select'].includes(field.type)) {
    field.options = []
    field.optionsText = ''
    if (['device', 'user', 'images'].includes(field.type)) delete field.default_value
    return
  }
  field.options = Array.isArray(field.options) ? field.options : []
  field.optionsText = field.options.join('\n')
  if (field.type === 'multi_select') delete field.default_value
  if (field.type === 'select' && field.default_value !== undefined && !field.options.includes(field.default_value)) delete field.default_value
}

function formDefinitionForSave() {
  const definition = cloneDefinition(formDraft.value)
  const seenKeys = new Set()
  definition.fields = (definition.fields || []).map(field => {
    const normalized = { ...field }
    normalized.key = String(normalized.key || '').trim()
    normalized.label = String(normalized.label || '').trim()
    if (!normalized.key || seenKeys.has(normalized.key)) {
      throw new Error(currentLang.value === 'en' ? 'Every form field must have a unique key.' : '每个表单字段都必须具有唯一标识。')
    }
    if (!normalized.label || [...normalized.label].length > 128) {
      throw new Error(currentLang.value === 'en' ? 'Every field needs a label of no more than 128 characters.' : '每个字段都必须填写名称，且名称不能超过 128 个字符。')
    }
    if (normalized.type === 'integer' && normalized.default_value !== undefined && !Number.isInteger(normalized.default_value)) {
      throw new Error(currentLang.value === 'en' ? `Field “${normalized.label}” needs an integer default value.` : `字段“${normalized.label}”的默认值必须是整数。`)
    }
    seenKeys.add(normalized.key)
    if (['select', 'multi_select'].includes(normalized.type)) {
      normalized.options = [...new Set(String(normalized.optionsText || '').split(/\r?\n/).map(value => value.trim()).filter(Boolean))]
      if (!normalized.options.length) {
        throw new Error(currentLang.value === 'en' ? `Field “${normalized.label}” needs at least one option.` : `字段“${normalized.label}”至少需要一个选项。`)
      }
    } else {
      normalized.options = []
    }
    if (['device', 'user', 'multi_select', 'images'].includes(normalized.type)) delete normalized.default_value
    delete normalized.optionsText
    return normalized
  })
  return definition
}

async function publishForm() {
  if (!selectedTemplate.value) return
  const templateID = selectedTemplate.value.template.ID
  actionLoading.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const definition = formDefinitionForSave()
    formDraft.value = normalizeFormDraft(definition)
    const response = await axios.post(`/api/work-order-templates/${templateID}/form-versions`, { definition })
    if (response.data.code !== 0) throw new Error(response.data.message)
    if (!await selectTemplate(templateID)) {
      errorMessage.value = txf('formPublishedReloadFailed', { message: errorMessage.value })
      return
    }
    successMessage.value = tx('formPublished')
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function saveFormDraft() {
  if (!selectedTemplate.value) return
  const templateID = selectedTemplate.value.template.ID
  actionLoading.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const definition = formDefinitionForSave()
    const response = await axios.put(`/api/work-order-templates/${templateID}/form-draft`, { definition })
    if (response.data.code !== 0) throw new Error(response.data.message)
    if (!await selectTemplate(templateID)) {
      errorMessage.value = txf('formDraftReloadFailed', { message: errorMessage.value })
      return
    }
    successMessage.value = tx('formDraftSaved')
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function publishWorkflow() {
  if (!selectedTemplate.value) return
  const templateID = selectedTemplate.value.template.ID
  actionLoading.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const definition = workflowEditorRef.value?.getWorkflowDefinition?.() || workflowDraft.value
    workflowDraft.value = definition
    const response = await axios.post(`/api/work-order-templates/${templateID}/workflow-versions`, { definition })
    if (response.data.code !== 0) throw new Error(response.data.message)
    if (!await selectTemplate(templateID)) {
      errorMessage.value = txf('workflowPublishedReloadFailed', { message: errorMessage.value })
      return
    }
    successMessage.value = tx('workflowPublished')
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

async function saveWorkflowDraft() {
  if (!selectedTemplate.value) return
  const templateID = selectedTemplate.value.template.ID
  actionLoading.value = true
  errorMessage.value = ''
  successMessage.value = ''
  try {
    const definition = workflowEditorRef.value?.getWorkflowDefinition?.() || workflowDraft.value
    workflowDraft.value = definition
    const response = await axios.put(`/api/work-order-templates/${templateID}/workflow-draft`, { definition })
    if (response.data.code !== 0) throw new Error(response.data.message)
    if (!await selectTemplate(templateID)) {
      errorMessage.value = txf('workflowDraftReloadFailed', { message: errorMessage.value })
      return
    }
    successMessage.value = tx('workflowDraftSaved')
  } catch (error) {
    errorMessage.value = apiError(error)
  } finally {
    actionLoading.value = false
  }
}

function toggleWorkflowFullscreen() {
  workflowEditorRef.value?.toggleFullscreen?.()
}

function refreshCurrentTab() {
  if (activeTab.value === 'orders') {
    orderPage.value = 1
    loadOrders()
  } else if (activeTab.value === 'integrations') {
    loadIntegrations()
  } else {
    loadTemplates()
    if (selectedTemplate.value) selectTemplate(selectedTemplate.value.template.ID)
  }
}

function sourceLabel(value) {
  return ({ manual: tx('sourceManual'), rule: tx('sourceRule'), alarm: tx('sourceAlarm'), ai: tx('sourceAI'), external: tx('sourceExternal') })[value] || value || '-'
}
function priorityLabel(value) { return ({ low: tx('low'), normal: tx('normal'), high: tx('high'), urgent: tx('urgent') })[value] || value || '-' }
function priorityClass(value) { return ({ low: 'text-bg-secondary', normal: 'text-bg-info', high: 'text-bg-warning', urgent: 'text-bg-danger' })[value] || 'text-bg-secondary' }
function localizedStatusName(value, fallback = '') {
  return ({ open: tx('statusOpen'), in_progress: tx('statusInProgress'), paused: tx('statusPaused'), resolved: tx('statusResolved'), closed: tx('statusClosed'), cancelled: tx('statusCancelled') })[value] || fallback || value || '-'
}
function statusLabel(value) { return localizedStatusName(value, workflowStatuses.value.find(status => status.key === value)?.name) }
function statusClass(value) { return ({ open: 'text-bg-primary', in_progress: 'text-bg-warning', paused: 'text-bg-info', resolved: 'text-bg-success', closed: 'text-bg-secondary', cancelled: 'text-bg-dark' })[value] || 'text-bg-secondary' }
function formatTime(value) { return formatDateTime(value) }
function displayFormValue(value, field = {}) {
  if (field.type === 'user') return value ? (assigneeNames.value.get(String(value)) || tx('unavailableUser')) : '-'
  if (field.key === 'severity') return severityFormValueLabel(value)
  if (field.key === 'fault_type') return faultTypeFormValueLabel(value)
  return Array.isArray(value) ? value.join(currentLang.value === 'en' ? ', ' : '、') : value === true ? tx('yes') : value === false ? tx('no') : value ?? '-'
}
function workOrderFormFieldLabel(field = {}) {
  const labels = {
    zh: { device_code: '设备编码', device_name: '设备名称', fault_type: '故障类型', severity: '严重程度', description: '故障描述', diagnostic_checks: '诊断检查项', ai_suggestion_id: 'AI 建议编号' },
    en: { device_code: 'Device code', device_name: 'Device name', fault_type: 'Fault type', severity: 'Severity', description: 'Fault description', diagnostic_checks: 'Diagnostic checks', ai_suggestion_id: 'AI suggestion ID' }
  }
  return labels[currentLang.value]?.[field.key] || field.label || field.key || '-'
}
function faultTypeFormValueLabel(value) {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (!normalized) return '-'
  const labels = {
    zh: { frequent_status_flapping: '设备频繁上下线', temperature_high: '设备温度过高' },
    en: { frequent_status_flapping: 'Frequent online/offline switching', temperature_high: 'High device temperature' }
  }
  return labels[currentLang.value]?.[normalized] || value
}
function severityFormValueLabel(value) {
  const normalized = String(value ?? '').trim().toLowerCase()
  if (!normalized) return '-'
  const labels = {
    zh: { low: '低', normal: '普通', high: '高', urgent: '紧急' },
    en: { low: 'Low', normal: 'Normal', high: 'High', urgent: 'Urgent' }
  }
  return labels[currentLang.value][normalized] || String(value)
}
function imageFormValues(value) { return Array.isArray(value) ? value.filter(item => typeof item === 'string' && item.trim()) : [] }
function workOrderId(order) { return order?.ID || order?.id || 0 }
function cloneDefinition(value) { return JSON.parse(JSON.stringify(value)) }
function defaultGraphWorkflow() {
  return { schema_version: 2, start_node_id: 'start', nodes: [{ id: 'start', type: 'start', name: tx('start') }, { id: 'handle', type: 'user_task', name: tx('handleTask'), task_kind: 'handle', completion_mode: 'all', participant_user_ids: [] }, { id: 'end', type: 'end', name: tx('end'), result: 'completed' }], edges: [{ id: 'edge-start-handle', source: 'start', target: 'handle' }, { id: 'edge-handle-end', source: 'handle', target: 'end' }], layout: { nodes: [{ id: 'start', x: 250, y: 50 }, { id: 'handle', x: 250, y: 200 }, { id: 'end', x: 250, y: 350 }] } }
}
function normalizeGraphWorkflow(value) { return value?.schema_version === 2 && Array.isArray(value.nodes) ? cloneDefinition(value) : defaultGraphWorkflow() }

onMounted(async () => {
  await loadTemplates()
  await openRouteTemplateEditor()
  await loadProjectParticipants()
  await loadWorkOrderEntityCatalog()
  applyRouteOrderFilters()
  await loadOrders()
  await openHighlightedWorkOrder()
})

watch(() => [route.query.highlight, route.query.source], async () => {
  applyRouteOrderFilters()
  orderPage.value = 1
  await loadOrders()
  highlightedWorkOrderPublicID.value = ''
  await openHighlightedWorkOrder()
})

watch(() => route.query.template, async () => {
  await openRouteTemplateEditor()
})
</script>

<style scoped>
.work-order-center {
  --wo-surface: var(--bs-body-bg);
  --wo-surface-muted: rgba(var(--bs-secondary-rgb), 0.055);
  --wo-table-head: rgba(var(--bs-secondary-rgb), 0.075);
  --wo-row-hover: rgba(var(--bs-primary-rgb), 0.045);
  --wo-row-selected: rgba(var(--bs-primary-rgb), 0.09);
  --wo-selected-accent: var(--bs-primary);
  --wo-timeline-line: rgba(var(--bs-secondary-rgb), 0.28);
}
:global([data-bs-theme='dark']) .work-order-center {
  --wo-surface: #171a20;
  --wo-surface-muted: rgba(255, 255, 255, 0.035);
  --wo-table-head: rgba(255, 255, 255, 0.045);
  --wo-row-hover: rgba(118, 160, 255, 0.065);
  --wo-row-selected: rgba(100, 149, 237, 0.14);
  --wo-selected-accent: #7da6ff;
  --wo-timeline-line: rgba(255, 255, 255, 0.16);
}
.work-order-filter-card,
.work-order-list-card,
.work-order-detail-card { background: var(--wo-surface); }
.work-order-relation-filter .btn { padding-inline: 0.75rem; }
.work-order-table { --bs-table-bg: transparent; --bs-table-color: var(--bs-body-color); }
.work-order-table-head th {
  background: var(--wo-table-head);
  border-bottom-color: var(--bs-border-color);
  color: var(--bs-secondary-color);
  font-size: 0.78rem;
  font-weight: 650;
  letter-spacing: 0.02em;
  padding-block: 0.8rem;
  white-space: nowrap;
}
.work-order-row > td {
  background: transparent;
  border-bottom-color: var(--bs-border-color-translucent);
  transition: background-color 0.16s ease, box-shadow 0.16s ease;
}
.work-order-row:hover > td { background: var(--wo-row-hover); }
.work-order-row--selected > td { background: var(--wo-row-selected); }
.work-order-row--selected > td:first-child { box-shadow: inset 3px 0 0 var(--wo-selected-accent); }
.work-order-source-badge,
.work-order-task-badge {
  background: var(--wo-surface-muted);
  border-color: var(--bs-border-color) !important;
  color: var(--bs-body-color);
  font-weight: 500;
}
.work-order-form-section,
.work-order-action-panel { background: var(--wo-surface-muted); }
.work-order-history-guidance {
  border: 1px solid color-mix(in srgb, var(--bs-success) 42%, var(--bs-border-color));
  background: color-mix(in srgb, var(--bs-success) 7%, var(--wo-surface-muted));
}
.work-order-history-guidance__verified,
.work-order-history-experience__count {
  border: 1px solid color-mix(in srgb, var(--bs-success) 34%, transparent);
  background: color-mix(in srgb, var(--bs-success) 16%, transparent);
  color: var(--bs-success-text-emphasis);
}
.work-order-history-guidance__summary {
  padding: 0.75rem;
  border-left: 3px solid var(--bs-success);
  border-radius: 0.45rem;
  background: color-mix(in srgb, var(--bs-success) 8%, var(--wo-surface));
  line-height: 1.65;
  white-space: pre-wrap;
}
.work-order-history-guidance__list { display: grid; gap: 0.75rem; }
.work-order-history-experience {
  overflow: hidden;
  border: 1px solid var(--bs-border-color);
  border-radius: 0.65rem;
  background: var(--wo-surface);
}
.work-order-history-experience__head {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.55rem;
  padding: 0.7rem 0.8rem;
  border-bottom: 1px solid var(--bs-border-color);
  background: var(--wo-surface-muted);
}
.work-order-history-experience__index {
  display: inline-grid;
  width: 1.75rem;
  height: 1.75rem;
  place-items: center;
  border-radius: 50%;
  background: var(--bs-primary);
  color: #fff;
  font-weight: 700;
}
.work-order-history-experience__details { padding: 0.8rem; }
.work-order-history-experience__details dd { line-height: 1.55; white-space: pre-wrap; }
.work-order-detail-images { display: grid; grid-template-columns: repeat(auto-fill, minmax(5rem, 1fr)); gap: .5rem; }
.work-order-detail-images a { display: block; overflow: hidden; border: 1px solid var(--bs-border-color); border-radius: .5rem; background: var(--bs-tertiary-bg); aspect-ratio: 4 / 3; }
.work-order-detail-images img { display: block; width: 100%; height: 100%; object-fit: cover; }
.work-order-timeline-item { position: relative; border-left: 1px solid var(--wo-timeline-line); }
.work-order-timeline-item:last-child { border-left-color: transparent; padding-bottom: 0 !important; }
.work-order-timeline-dot {
  position: absolute;
  top: 0.22rem;
  left: -0.32rem;
  width: 0.62rem;
  height: 0.62rem;
  border: 2px solid var(--wo-surface);
  border-radius: 50%;
  background: var(--wo-selected-accent);
  box-shadow: 0 0 0 1px var(--wo-selected-accent);
}
.work-order-process-modal .modal-content { background: var(--bs-body-bg); color: var(--bs-body-color); }
.work-order-drawer { position: fixed; z-index: 1051; top: 0; right: 0; width: min(560px, 100vw); height: 100dvh; display: flex; flex-direction: column; background: var(--wo-surface); color: var(--bs-body-color); }
.work-order-drawer__head { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.25rem; border-bottom: 1px solid var(--bs-border-color); }
.work-order-drawer__body { min-width: 0; padding: 1.25rem; overflow-y: auto; overflow-wrap: anywhere; }
.work-order-template-row { cursor: pointer; }
.template-detail-drawer { position: fixed; z-index: 1051; top: 0; right: 0; width: min(840px, 100vw); height: 100dvh; display: flex; flex-direction: column; background: var(--wo-surface); color: var(--bs-body-color); }
.template-detail-drawer__head { display: flex; align-items: flex-start; justify-content: space-between; gap: 1rem; padding: 1.25rem; border-bottom: 1px solid var(--bs-border-color); }
.template-detail-drawer__body { min-width: 0; padding: 1.25rem; overflow: auto; }
.template-detail-workflow { min-height: 42rem; }
.template-detail-workflow :deep(.work-order-workflow-editor) { min-height: 42rem; }
.work-order-template-editor-modal { background: rgba(0, 0, 0, .55); }
.work-order-template-editor-dialog { max-width: min(1520px, calc(100vw - 2rem)); }
.work-order-template-editor-dialog .modal-content { height: calc(100vh - 2rem); max-height: calc(100vh - 2rem); }
.work-order-template-editor-dialog .modal-body { display: flex; flex-direction: column; min-height: 0; overflow: hidden; padding: 0; }
.template-editor-tabs { display: flex; align-items: center; gap: 0.75rem; flex: 0 0 auto; min-width: 0; background: var(--bs-body-bg); }
.template-editor-tabs .nav-link { padding: 0.35rem 0.65rem; }
.template-editor-description { flex: 1 1 auto; min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.template-editor-readonly { white-space: nowrap; }
.template-editor-actions { display: flex; flex: 0 0 auto; flex-wrap: wrap; justify-content: flex-end; gap: 0.4rem; margin-left: auto; }
.template-editor-actions .btn { display: inline-flex; min-height: 2rem; align-items: center; border-color: var(--bs-border-color); white-space: nowrap; }
.template-editor-tab-panel { flex: 1 1 auto; min-height: 0; overflow: auto; }
.template-editor-workflow-panel { display: flex; flex: 1 1 auto; flex-direction: column; min-height: 0; overflow: hidden; }
.template-editor-workflow-panel :deep(.work-order-workflow-editor) { flex: 1 1 auto; min-height: 0; }
.template-editor-workflow-panel :deep(.workflow-editor-content) { height: 100%; min-height: 0; }
@media (max-width: 767.98px) { .work-order-drawer, .template-detail-drawer { width: 100vw; } .template-detail-workflow, .template-detail-workflow :deep(.work-order-workflow-editor) { min-height: 34rem; } }
@media (max-width: 991px) { .work-order-template-editor-dialog { margin: 0; max-width: 100vw; min-height: 100vh; } .work-order-template-editor-dialog .modal-content { height: 100vh; max-height: 100vh; } .template-editor-tabs { align-items: flex-start; flex-wrap: wrap; } .template-editor-description { flex: 1 0 100%; order: 3; } .template-editor-readonly { order: 2; } .template-editor-actions { margin-left: auto; order: 1; } .template-editor-workflow-panel { overflow: auto; } .work-order-template-editor-dialog :deep(.workflow-editor-content) { height: auto; min-height: 0; } }
@media (max-width: 767.98px) { .work-order-center .nav-tabs { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: .4rem; border-bottom: 0; } .work-order-center .nav-item, .work-order-center .nav-link { width: 100%; margin: 0; border: 1px solid var(--bs-border-color); border-radius: .45rem; text-align: center; } .work-order-filter-card .row > [class*='col-'] { width: 100%; } .work-order-relation-filter { align-items: stretch !important; } .work-order-relation-filter > span { flex-basis: 100%; } .work-order-relation-filter .btn { flex: 1 1 calc(50% - .5rem); } .work-order-table thead { display: none; } .work-order-table, .work-order-table tbody, .work-order-table tr, .work-order-table td { display: block; width: 100%; } .work-order-row { margin: .65rem; width: calc(100% - 1.3rem); border: 1px solid var(--bs-border-color); border-radius: .65rem; overflow: hidden; } .work-order-row > td { padding: .65rem .8rem; border: 0; } .work-order-row--selected > td:first-child { box-shadow: inset 3px 0 0 var(--wo-selected-accent); } .work-order-row > td[data-label] { display: flex; align-items: center; justify-content: space-between; gap: 1rem; } .work-order-row > td[data-label]::before { content: attr(data-label); color: var(--bs-secondary-color); font-size: .78rem; } .work-order-actions-cell .btn-group { width: 100%; } .work-order-actions-cell .btn { flex: 1; } }
.work-order-attachment-card {
  position: relative;
  width: 68px;
  height: 68px;
  background: var(--bs-tertiary-bg, var(--bs-body-tertiary-bg, #f8f9fa));
  border-color: var(--bs-border-color) !important;
}
.work-order-attachment-img {
  width: 68px;
  height: 68px;
  object-fit: cover;
  transition: opacity 0.15s ease-in-out;
}
.work-order-attachment-card:hover .work-order-attachment-img {
  opacity: 0.82;
}
.work-order-attachment-del {
  position: absolute;
  top: 3px;
  right: 3px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  padding: 0;
  line-height: 1;
  font-size: 0.9rem;
  box-shadow: 0 2px 5px rgba(0,0,0,0.3);
  z-index: 5;
}
.work-order-attachment-doc {
  background-color: var(--bs-tertiary-bg, var(--bs-body-tertiary-bg, #f8f9fa)) !important;
  color: var(--bs-body-color) !important;
  border-color: var(--bs-border-color) !important;
}
</style>
