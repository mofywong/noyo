import axios from 'axios'

function isTransientWorkOrderNetworkError(error) {
  return !error?.response && (error?.code === 'ERR_NETWORK' || error?.message === 'Network Error')
}

export async function requestWorkOrderRead(request, { retries = 1, delayMs = 150 } = {}) {
  let attempt = 0
  while (true) {
    try {
      return await request()
    } catch (error) {
      if (!isTransientWorkOrderNetworkError(error) || attempt >= retries) throw error
      attempt += 1
      if (delayMs > 0) await new Promise(resolve => setTimeout(resolve, delayMs))
    }
  }
}

export function workOrderApiErrorMessage(error) {
  if (isTransientWorkOrderNetworkError(error)) return '工单服务暂时不可用，请稍后重试。'
  return error?.response?.data?.message || error?.message || '请求失败，请稍后重试'
}

function getWorkOrder(url, config) {
  return requestWorkOrderRead(() => axios.get(url, config))
}

export function checkedWorkOrderResponse(response) {
  if (response.data?.code !== 0) throw new Error(response.data?.message || 'Work order operation failed')
  return response
}

const operation = request => request.then(checkedWorkOrderResponse)

export const workOrderApi = {
  list(params) { return getWorkOrder('/api/work-orders', { params }) },
  detail(id) { return Promise.all([getWorkOrder(`/api/work-orders/${id}`), getWorkOrder(`/api/work-orders/${id}/events`), getWorkOrder(`/api/work-orders/${id}/approval-tasks`), getWorkOrder(`/api/work-orders/${id}/tasks`)]) },
  templates() { return getWorkOrder('/api/work-order-templates') },
  template(id) { return getWorkOrder(`/api/work-order-templates/${id}`) },
	copyTemplate(id) { return axios.post(`/api/work-order-templates/${id}/copy`) },
  participants(params = { page: 1, page_size: 100 }) { return getWorkOrder('/api/work-order-participants', { params }) },
  policies() { return operation(getWorkOrder('/api/work-order-policies')) },
  policyVersion(id) { return operation(getWorkOrder(`/api/work-order-policy-versions/${id}`)) },
  publishPolicy(id, payload) { return operation(axios.post(id ? `/api/work-order-policies/${id}/publish` : '/api/work-order-policies', payload)) },
  enablePolicy(id,payload) { return operation(axios.post(`/api/work-order-policies/${id}/enabled`,payload)) },
  previewPolicy(id,payload) { return operation(axios.post(`/api/work-order-policy-versions/${id}/preview`,payload)) },
  executionGrants() { return operation(getWorkOrder('/api/work-order-execution-grants')) },
  createExecutionGrant(payload) { return operation(axios.post('/api/work-order-execution-grants',payload)) },
  revokeExecutionGrant(id,reason) { return operation(axios.post(`/api/work-order-execution-grants/${id}/revoke`,{reason})) },
  operationsSummary() { return operation(getWorkOrder('/api/work-order-operations/summary')) },
  failedDeliveries() { return operation(getWorkOrder('/api/work-order-operations/failed-deliveries')) },
  recoverDelivery(id,payload) { return operation(axios.post(`/api/work-order-operations/deliveries/${id}/recover`,payload)) },
  claimTask(id,taskID,version) { return operation(axios.post(`/api/work-orders/${id}/tasks/${taskID}/claim`,{expected_version:version})) },
  sla(id) { return operation(getWorkOrder(`/api/work-orders/${id}/sla`)) },
  pauseSLA(id,payload) { return operation(axios.post(`/api/work-orders/${id}/sla/pause`,payload)) },
  assignmentRoles() { return getWorkOrder('/api/work-order-assignment-roles') },
  assignmentBlocks(id) { return getWorkOrder(`/api/work-orders/${id}/assignment-blocks`) },
  dispatchNode(id, instanceID, payload) { return axios.post(`/api/work-orders/${id}/assignment-blocks/${instanceID}/dispatch`, payload) },
  createTemplate(payload) { return axios.post('/api/work-order-templates', payload) },
  publishForm(id, definition) { return axios.post(`/api/work-order-templates/${id}/form-versions`, { definition }) },
  publishWorkflow(id, definition) { return axios.post(`/api/work-order-templates/${id}/workflow-versions`, { definition }) },
  validateForm(id, definition) { return axios.post(`/api/work-order-templates/${id}/form-schema/validate`, { definition }) },
  create(payload) { return axios.post('/api/work-orders', payload) },
  uploadAttachment(formData) { return axios.post('/api/work-orders/attachments', formData) },
  execute(id, action, payload = {}) { return axios.post(`/api/work-orders/${id}/${action}`, payload) },
  transition(id, payload) { return axios.post(`/api/work-orders/${id}/transitions`, payload) },
  approve(id, payload) { return axios.post(`/api/work-orders/${id}/approve`, payload) },
  reject(id, payload) { return axios.post(`/api/work-orders/${id}/reject`, payload) },
  tasks(id) { return getWorkOrder(`/api/work-orders/${id}/tasks`) },
  completeTask(id, taskPublicID, payload = {}) { return axios.post(`/api/work-orders/${id}/tasks/${taskPublicID}/complete`, payload) },
  rejectTask(id, taskPublicID, payload = {}) { return axios.post(`/api/work-orders/${id}/tasks/${taskPublicID}/reject`, payload) },
  notifications(params = {}) { return getWorkOrder('/api/work-order-notifications', { params }) },
  markNotificationRead(publicID) { return axios.post(`/api/work-order-notifications/${publicID}/read`) },
  comment(id, comment) { return axios.post(`/api/work-orders/${id}/comments`, { comment }) },
  claimOutbox(payload) { return axios.post('/api/work-order-outbox/claim', payload) },
  acknowledgeOutbox(id, payload) { return axios.post(`/api/work-order-outbox/${id}/ack`, payload) },
  retryOutbox(id, payload) { return axios.post(`/api/work-order-outbox/${id}/retry`, payload) },
  replayInbox(payload) { return axios.post('/api/work-order-integrations/inbox', payload) },
  integrationConnectors() { return getWorkOrder('/api/work-order-integrations/connectors') },
  saveIntegrationConnector(payload) { return axios.post('/api/work-order-integrations/connectors', payload) },
  integrationBindings() { return getWorkOrder('/api/work-order-integrations/bindings') },
  saveIntegrationBinding(payload) { return axios.post('/api/work-order-integrations/bindings', payload) },
  integrationMappings(bindingID) { return getWorkOrder(`/api/work-order-integrations/bindings/${bindingID}/mappings`) },
  saveIntegrationMapping(bindingID, payload) { return axios.post(`/api/work-order-integrations/bindings/${bindingID}/mappings`, payload) }
}

export async function loadAllWorkOrderParticipants({ pageSize = 100, api = workOrderApi } = {}) {
  const participants = []
  let page = 1
  let total = Number.POSITIVE_INFINITY
  while (participants.length < total) {
    const response = await api.participants({ page, page_size: pageSize })
    if (response.data?.code !== 0) throw new Error(response.data?.message || 'load participants failed')
    const data = response.data?.data || {}
    const items = Array.isArray(data.items) ? data.items : []
    participants.push(...items)
    total = Number.isFinite(Number(data.total)) ? Number(data.total) : participants.length
    if (items.length === 0) break
    page += 1
  }
  return participants
}
