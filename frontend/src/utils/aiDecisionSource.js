const object = value => value && typeof value === 'object' && !Array.isArray(value) ? value : {}

// Keep each score's meaning. A person detector score is never a sleep probability.
const scoreKinds = new Set(['person_detection', 'cigarette_detection', 'pose_keypoint', 'violence_classification'])
export function readLocalScores(value) {
  const entries = Array.isArray(value) ? value.map(item => [item?.type || item?.kind, item]) : Object.entries(object(value))
  return entries.flatMap(([kind, raw]) => {
    const score = typeof raw === 'number' ? raw : raw?.value
    if (!scoreKinds.has(kind) || typeof score !== 'number' || !Number.isFinite(score) || score < 0 || score > 1) return []
    return [{ kind, value: score, model: typeof raw?.model === 'string' ? raw.model : '' }]
  })
}

export function hasAiDecisionMetadata(value) {
  const data = object(value)
  return ['decision_source', 'local_scores', 'local_conclusion', 'semantic_status', 'semantic_result'].some(key => data[key] != null)
}

export function readAiDecision(value = {}) {
  const data = object(value)
  const semantic = object(data.semantic_result)
  const local = object(data.local_result)
  const status = data.semantic_status || data.status || semantic.status || 'not_requested'
  const verdict = semantic.verdict || (typeof data.semantic_result === 'string' ? data.semantic_result : '') || data.verdict || ''
  const semanticScores = readLocalScores(semantic.scores)
  const execution = semantic.execution || data.semantic_site || data.execution || ''
  const fallbackReason = semantic.fallback_reason || data.fallback_reason || ''
  const attemptsUsed = semantic.attempts_used ?? data.attempts_used
  const actualExecutorEpoch = Object.hasOwn(semantic, 'actual_executor_epoch') ? semantic.actual_executor_epoch : data.actual_executor_epoch
  return {
    source: data.decision_source || 'unknown',
    inferenceSite: data.inference_site || local.inference_site || '',
    scores: semanticScores.length ? semanticScores : readLocalScores(data.local_scores || local.scores || data.scores),
    localConclusion: data.local_conclusion || (typeof data.local_result === 'string' ? data.local_result : '') || local.conclusion || '',
    status,
    verdict,
    summaryZh: semantic.summary_zh || data.summary_zh || '',
    summaryEn: semantic.summary_en || data.summary_en || '',
    execution: execution === 'local_first' ? '' : execution,
    actualExecutorEpoch: typeof actualExecutorEpoch === 'string' ? actualExecutorEpoch : '',
    executorCalled: typeof actualExecutorEpoch === 'string' ? actualExecutorEpoch.trim().length > 0 : null,
    fallbackReason: typeof fallbackReason === 'string' ? fallbackReason : '',
    attemptsUsed: Number.isInteger(attemptsUsed) && attemptsUsed >= 0 && attemptsUsed <= 2 ? attemptsUsed : null,
    model: semantic.model || data.semantic_model || data.model || '',
    reviewedAt: semantic.completed_at || data.completed_at || '',
    observedAt: data.observed_at || semantic.observed_at || '',
    reasonCodes: [...new Set((Array.isArray(semantic.reason_codes) ? semantic.reason_codes : Array.isArray(data.reason_codes) ? data.reason_codes : []).filter(code => typeof code === 'string' && code.trim()))].slice(0, 8),
  }
}

export function semanticReviewItems(data = {}) {
  return (Array.isArray(data.results) ? data.results : []).filter(item => item && typeof item === 'object' && !Array.isArray(item))
}

function observationIdentity(params) {
  const semantic = object(params.semantic_result)
  return {
    incident: params.alarm_fingerprint || params.incident_id || semantic.incident_id,
    rule: params.rule_id || semantic.rule_id,
    scene: params.scene_type || semantic.scene,
    target: params.target_id,
    generation: params.target_generation || semantic.target_generation,
    session: params.stream_session || semantic.stream_session,
  }
}

// Only occurrence observations contain new recognition evidence. Handling rows
// retain the original snapshot and must never overwrite a later LLM result.
export function selectAlarmAiDecision(item, events = []) {
  const initial = object(item?.evidence?.params)
  const instanceID = item?.alarm?.ID ?? item?.alarm?.id
  const identity = observationIdentity(initial)
  const observations = (Array.isArray(events) ? events : []).flatMap((row, index) => {
    const type = row?.event?.type
    if (!['opened', 'repeated', 'signal_observed'].includes(type)) return []
    if (instanceID != null && row.event.alarm_instance_id != null && String(instanceID) !== String(row.event.alarm_instance_id)) return []
    const payload = object(row.payload?.payload)
    const evidence = Object.hasOwn(payload, 'params') ? payload : type === 'opened' ? object(row.evidence) : {}
    const params = object(evidence.params)
    if (!hasAiDecisionMetadata(params)) return []
    const incoming = observationIdentity(params)
    if (Object.entries(identity).some(([key, value]) => value != null && incoming[key] != null && String(value) !== String(incoming[key]))) return []
    // The actual instance/fingerprint binds the observation. A semantic result
    // must also identify that same incident when both identifiers are present.
    if (incoming.incident && params.semantic_result?.incident_id && String(incoming.incident) !== String(params.semantic_result.incident_id)) return []
    const rawTime = row.payload?.occurred_at || params.occurred_at || row.created_at || row.event?.CreatedAt
    const numericTime = typeof rawTime === 'number' ? rawTime : Date.parse(rawTime)
    return [{ params, identity: incoming, time: Number.isFinite(numericTime) ? numericTime : 0, id: Number(row.event?.ID ?? row.event?.id ?? index), index }]
  }).sort((left, right) => left.time - right.time || left.id - right.id || left.index - right.index)
  let latest = initial
  for (const observation of observations) {
    if (Object.entries(identity).some(([key, value]) => value != null && observation.identity[key] != null && String(value) !== String(observation.identity[key]))) continue
    for (const [key, value] of Object.entries(observation.identity)) if (identity[key] == null && value != null) identity[key] = value
    latest = observation.params
  }
  return { ...initial, ...latest }
}

const reasonLabels = {
  semantic_review_disabled: ['语义复核未启用', 'Semantic review is disabled'],
  service_stopped: ['语义复核服务已停止', 'Semantic review service is stopped'],
  semantic_service_not_started: ['语义复核服务尚未启动', 'Semantic review service has not started'],
  gateway_semantic_router_unavailable: ['网关语义转发暂不可用', 'Gateway semantic forwarding is unavailable'],
  queue_byte_limit: ['排队证据达到字节上限', 'Queued evidence reached the byte limit'],
  queue_count_limit: ['排队任务达到数量上限', 'Queued tasks reached the count limit'],
  executor_interrupted: ['执行者中断，复核结果未知', 'The executor was interrupted; the review result is unknown'],
  persistence_failed: ['复核任务保存失败', 'The review task could not be saved'],
  invalid_device_scope: ['设备租户或项目范围无效', 'The device tenant or project scope is invalid'],
  gateway_scope_mismatch: ['网关与设备范围不一致', 'Gateway and device scopes do not match'],
  device_scope_changed: ['设备租户或项目范围已改变', 'The device tenant or project scope changed'],
  duplicate_evidence: ['重复证据已合并', 'Duplicate evidence was merged'],
  scoped_visual_provider_not_ready: ['对应租户或项目的视觉模型未就绪', 'The scoped vision model is not ready'],
  capability_budget_or_epoch_not_configured: ['视觉能力、预算或执行版本未配置完整', 'Vision capabilities, budgets or executor epoch are incomplete'],
  explicit_visual_provider_epoch_and_budget_required: ['需明确视觉模型、执行版本和预算', 'An explicit vision provider, executor epoch and budget are required'],
  budget_limit: ['达到token或费用预算', 'The token or cost budget was reached'],
  provider_request_failed: ['视觉模型请求失败', 'The vision model request failed'],
  insufficient_visibility: ['目标可见性不足', 'The target is not sufficiently visible'],
  insufficient_temporal_evidence: ['缺少必要时序证据', 'Required temporal evidence is missing'],
  target_identity_ambiguous: ['目标身份关联不明确', 'The target association is ambiguous'],
}
export function semanticReasonText(code, language = 'zh') {
  const english = String(language).toLowerCase().startsWith('en')
  return reasonLabels[code]?.[english ? 1 : 0] || (english ? 'Additional review reason; inspect evidence and service status.' : '其他复核原因，请结合证据与服务状态核查。')
}
const fallbackLabels = {
  local_disabled: ['本机视觉复核未启用，转平台复核', 'Local visual review is disabled; using platform review'],
  local_no_provider: ['本机没有可用视觉模型，转平台复核', 'No local vision model is available; using platform review'],
  local_connection_failed: ['本机无法连接视觉模型，转平台复核', 'The local vision model could not be reached; using platform review'],
  local_timeout: ['本机视觉复核超时，转平台复核', 'Local visual review timed out; using platform review'],
  local_service_unavailable: ['本机视觉服务暂不可用，转平台复核', 'The local vision service is temporarily unavailable; using platform review'],
}
export function semanticFallbackReasonText(code, language = 'zh') {
  const english = String(language).toLowerCase().startsWith('en')
  return fallbackLabels[code]?.[english ? 1 : 0] || (english ? 'Platform fallback reason is unavailable. Check local review service status.' : '转平台的具体原因未提供，请检查本机复核服务状态。')
}
