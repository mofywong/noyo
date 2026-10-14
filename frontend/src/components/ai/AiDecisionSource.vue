<template>
  <section class="ai-decision-source" :aria-label="t('title')">
    <h3 class="ai-decision-title">{{ t('title') }}</h3>
    <dl class="ai-decision-fields">
      <div><dt>{{ t('source') }}</dt><dd>{{ label(decision.source) }}</dd></div>
      <div v-if="decision.inferenceSite"><dt>{{ t('localSite') }}</dt><dd>{{ siteLabel(decision.inferenceSite) }}</dd></div>
      <div><dt>{{ t('localConclusion') }}</dt><dd>{{ conclusionLabel(decision.localConclusion) }}</dd></div>
      <div v-for="score in decision.scores" :key="score.kind"><dt>{{ label(score.kind) }}</dt><dd>{{ Math.round(score.value * 1000) / 10 }}%<span v-if="score.model"> · {{ score.model }}</span></dd></div>
      <div v-if="!decision.scores.length"><dt>{{ t('scores') }}</dt><dd>{{ t('unavailable') }}</dd></div>
      <div><dt>{{ t('semanticStatus') }}</dt><dd>{{ label(decision.status) }}</dd></div>
      <div v-if="decision.verdict"><dt>{{ t('semanticResult') }}</dt><dd>{{ label(decision.verdict) }}</dd></div>
      <div v-if="decision.execution"><dt>{{ t('semanticSite') }}</dt><dd>{{ semanticExecutionLabel }}</dd></div>
      <div v-if="decision.fallbackReason"><dt>{{ t('fallbackReason') }}</dt><dd>{{ fallbackMessage }}</dd></div>
      <div v-if="decision.attemptsUsed !== null"><dt>{{ t('attemptsUsed') }}</dt><dd>{{ decision.attemptsUsed }} / 2</dd></div>
      <div v-if="decision.model"><dt>{{ t('model') }}</dt><dd>{{ decision.model }}</dd></div>
      <div v-if="reviewedAt"><dt>{{ t('reviewedAt') }}</dt><dd>{{ reviewedAt }}</dd></div>
      <div v-if="observedAt"><dt>{{ t('observedAt') }}</dt><dd>{{ observedAt }}</dd></div>
      <div v-if="reasonMessages.length"><dt>{{ t('reviewReasons') }}</dt><dd><ul class="ai-review-reasons"><li v-for="reason in reasonMessages" :key="reason">{{ reason }}</li></ul></dd></div>
    </dl>
    <p v-if="summary" class="ai-decision-summary">{{ summary }}</p>
    <p class="ai-decision-note">{{ t('scoreMeaning') }}</p>
    <p v-if="!['confirmed', 'rejected'].includes(decision.status)" class="ai-decision-note">{{ t('noNegativeInference') }}</p>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { readAiDecision, semanticReasonText, semanticFallbackReasonText } from '../../utils/aiDecisionSource.js'

const props = defineProps({ data: { type: Object, default: () => ({}) }, language: { type: String, default: 'zh' } })
const lang = computed(() => props.language.toLowerCase().startsWith('en') ? 'en' : 'zh')
const copy = {
  zh: {
    title: '识别来源与语义复核', source: '最终识别来源', localSite: '本地执行位置', localConclusion: '本地规则结论', scores: '本地模型分数', unavailable: '未提供', unknown: '来源未提供',
    local_model: '本地模型', llm_semantic: 'LLM语义识别', local_model_llm_verified: '本地模型＋LLM复核',
    person_detection: '人员检测置信度', cigarette_detection: '香烟检测置信度', pose_keypoint: '姿态关键点置信度', violence_classification: '暴力分类概率',
    semanticStatus: '语义复核状态', semanticResult: '语义识别结果', semanticSite: 'LLM复核执行端', notCalledSuffix: '（未调用）', fallbackReason: '转平台原因', attemptsUsed: '模型调用尝试次数', model: '语义模型', reviewedAt: '复核时间', observedAt: '候选观察时间',
    not_requested: '未请求复核', disabled: '未启用', pending: '等待复核', queued: '排队中', running: '正在复核', confirmed: '确认符合规则', rejected: '未发现符合规则的证据', insufficient_evidence: '证据不足', failed: '复核失败', rate_limited: '达到调用限额', offline: '语义服务离线', expired: '证据已过期', review_conflict: '复核结果冲突', timeout: '复核超时',
    no_provider: '未配置可用视觉模型', queue_full: '复核队列已满', invalid_evidence: '复核证据无效', invalid_result: '复核输出无效', result_unknown: '复核结果未知', merged: '重复候选已合并', buffered: '已在网关缓存，等待转发', disconnected: '网关与平台断联', reviewReasons: '复核原因', unknownEvidence: '本地证据未知',
    positive: '符合本地规则', negative: '未命中本地规则', uncertain: '本地证据不确定', platform: '平台', gateway: '网关', edge: '网关', local: '本机', intranet: '内网模型服务',
    scoreMeaning: '分数保留模型原始含义；人员或关键点置信度不代表睡岗概率。',
    noNegativeInference: '未调用、失败、离线、限额或证据不足均不代表正常，也不自动清除已有告警。',
  },
  en: {
    title: 'Recognition source and semantic review', source: 'Decision source', localSite: 'Local inference location', localConclusion: 'Local rule conclusion', scores: 'Local model scores', unavailable: 'Not provided', unknown: 'Source not provided',
    local_model: 'Local model', llm_semantic: 'LLM semantic recognition', local_model_llm_verified: 'Local model + LLM verification',
    person_detection: 'Person detection confidence', cigarette_detection: 'Cigarette detection confidence', pose_keypoint: 'Pose keypoint confidence', violence_classification: 'Violence classification probability',
    semanticStatus: 'Semantic review status', semanticResult: 'Semantic result', semanticSite: 'LLM review executor', notCalledSuffix: ' (not called)', fallbackReason: 'Platform fallback reason', attemptsUsed: 'Model call attempts', model: 'Semantic model', reviewedAt: 'Review time', observedAt: 'Candidate observation time',
    not_requested: 'Review not requested', disabled: 'Disabled', pending: 'Awaiting review', queued: 'Queued', running: 'Review in progress', confirmed: 'Rule confirmed', rejected: 'No evidence meeting the rule', insufficient_evidence: 'Insufficient evidence', failed: 'Review failed', rate_limited: 'Call limit reached', offline: 'Semantic service offline', expired: 'Evidence expired', review_conflict: 'Conflicting review result', timeout: 'Review timed out',
    no_provider: 'No available vision model configured', queue_full: 'Review queue is full', invalid_evidence: 'Invalid review evidence', invalid_result: 'Invalid review output', result_unknown: 'Review result is unknown', merged: 'Duplicate candidate merged', buffered: 'Buffered at the gateway, awaiting forwarding', disconnected: 'Gateway disconnected from the platform', reviewReasons: 'Review reasons', unknownEvidence: 'Local evidence unknown',
    positive: 'Meets local rule', negative: 'Local rule not met', uncertain: 'Uncertain local evidence', platform: 'Platform', gateway: 'Gateway', edge: 'Gateway', local: 'This machine', intranet: 'Intranet model service',
    scoreMeaning: 'Scores retain their model meaning. Person or keypoint confidence is not a sleep probability.',
    noNegativeInference: 'No review, failure, offline service, limits or insufficient evidence do not imply normality or clear existing alarms.',
  },
}
const t = key => copy[lang.value][key] || copy[lang.value].unavailable
const label = value => t(String(value || 'unavailable'))
const decision = computed(() => readAiDecision(props.data))
function siteLabel(value) {
  if (typeof value === 'object') return label(value.site || value.location || value.executor || '')
  return label(value)
}
const semanticExecutionLabel = computed(() => {
  const site = siteLabel(decision.value.execution)
  const failure = ['failed', 'rate_limited', 'offline', 'no_provider', 'queue_full', 'invalid_evidence', 'invalid_result', 'result_unknown', 'expired', 'timeout', 'disabled', 'disconnected'].includes(decision.value.status)
  return decision.value.executorCalled === false && failure ? `${site}${t('notCalledSuffix')}` : site
})
function conclusionLabel(value) {
  if (value === 'unknown') return t('unknownEvidence')
  if (value && typeof value === 'object') return value[lang.value] || conclusionLabel(value.status || value.conclusion)
  return label(value)
}
const summary = computed(() => lang.value === 'en' ? decision.value.summaryEn : decision.value.summaryZh)
const reasonMessages = computed(() => [...new Set(decision.value.reasonCodes.map(code => semanticReasonText(code, lang.value)))])
const fallbackMessage = computed(() => semanticFallbackReasonText(decision.value.fallbackReason, lang.value))
function displayTime(value) {
  const date = new Date(value)
  if (!value || !Number.isFinite(date.getTime()) || date.getUTCFullYear() < 2000) return ''
  return date.toLocaleString(lang.value === 'en' ? 'en-US' : 'zh-CN')
}
const reviewedAt = computed(() => displayTime(decision.value.reviewedAt))
const observedAt = computed(() => displayTime(decision.value.observedAt))
</script>

<style scoped>
.ai-decision-source {
  --decision-surface: var(--bg-surface, var(--bs-body-bg));
  --decision-border: var(--border-color, var(--bs-border-color));
  --decision-text: var(--text-primary, var(--bs-body-color));
  --decision-muted: var(--text-secondary, var(--bs-secondary-color));
  margin-bottom: 16px;
  padding: 24px;
  border: 1px solid var(--decision-border);
  border-radius: var(--radius-control, 8px);
  background: var(--decision-surface);
  color: var(--decision-text);
}
[data-bs-theme='dark'] .ai-decision-source, :global([data-bs-theme='dark']) .ai-decision-source {
  --decision-surface: var(--bg-surface, var(--bs-body-bg));
  --decision-border: var(--border-color, var(--bs-border-color));
  --decision-text: var(--text-primary, var(--bs-body-color));
  --decision-muted: var(--text-secondary, var(--bs-secondary-color));
}
.ai-decision-title { font-size: 1rem; font-weight: 600; margin: 0 0 16px; }
.ai-decision-fields { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 16px; margin: 0; }
.ai-decision-fields dt { color: var(--decision-muted); font-size: .875rem; font-weight: 400; }
.ai-decision-fields dd { margin: 8px 0 0; overflow-wrap: anywhere; }
.ai-review-reasons { margin: 0; padding-left: 16px; }
.ai-review-reasons li + li { margin-top: 8px; }
.ai-decision-summary { margin: 16px 0 0; overflow-wrap: anywhere; }
.ai-decision-note { margin: 16px 0 0; color: var(--decision-muted); font-size: .875rem; }
</style>
