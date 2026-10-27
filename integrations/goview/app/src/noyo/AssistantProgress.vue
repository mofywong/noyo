<template>
  <section class="assistant-steps" :aria-label="t('生成步骤与结果', 'Generation steps and results')">
    <div class="assistant-steps-heading" role="status" aria-live="polite">
      <strong>{{ waiting ? t('等待补充信息', 'Awaiting information') : current ? `${t('当前', 'Current')} · ${label(current.id)}` : steps.some(s => s.state === 'failed') ? t('需要处理', 'Needs attention') : steps.every(s => s.state === 'succeeded') ? t('校验通过，可以应用', 'Validated and ready to apply') : t('等待下一步', 'Awaiting next step') }}</strong>
      <span>{{ steps.filter(s => s.state === 'succeeded').length }} / {{ steps.length }}</span>
    </div>
    <p v-if="waiting" class="assistant-steps-explanation">{{ t('尚未生成可应用方案。请在下方补充信息，再继续生成后续步骤。', 'No proposal is available to apply yet. Reply below with the requested information to continue the remaining steps.') }}</p>
    <ol>
      <li v-for="(step, index) in steps" :key="step.id" :class="`is-${step.state}`" :aria-current="step.state === 'running' ? 'step' : undefined">
        <span class="assistant-step-marker" aria-hidden="true">{{ step.state === 'succeeded' ? '✓' : step.state === 'failed' ? '!' : index + 1 }}</span>
        <div><div class="assistant-step-title"><strong>{{ label(step.id) }}</strong><span>{{ stateLabel(step.state) }}</span></div>
          <p v-if="step.detail">{{ step.detail }}</p>
          <p v-else-if="step.state === 'succeeded'">{{ result(step) }}</p>
          <p v-else-if="step.state === 'running'">{{ step.id === 'preview' ? t('正在检查实际渲染、文字和画布边界…', 'Checking rendered text and canvas bounds…') : ['intention', 'metrics', 'generating'].includes(step.id) ? t('正在处理本阶段上下文并生成结果…', 'Processing this stage’s context and generating a result…') : t('正在检查已生成的结果…', 'Checking the generated result…') }}</p>
          <p v-else-if="step.state === 'failed'">{{ step.id === 'preview' ? t('预览未通过，请调整后重新校验。', 'Preview needs adjustment before applying.') : t('此步骤未完成，已完成结果已保留。', 'This step is incomplete. Completed results are saved.') }}</p>
        </div>
      </li>
    </ol>
  </section>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import type { AssistantStep } from './assistantProgress'
const props = defineProps<{ steps: AssistantStep[]; locale: string; waiting?: boolean }>()
const t = (zh: string, en: string) => props.locale.startsWith('en') ? en : zh
const current = computed(() => props.steps.find(step => step.state === 'running'))
const label = (id: string) => {
  const labels: Record<string, [string, string]> = { intention: ['明确监控目标', 'Monitoring goal'], metrics: ['选择业务指标', 'Metric selection'], checking: ['检查数据能力', 'Data capability checks'], generating: ['编排组件与布局', 'Widgets and layout'], reviewing: ['校验方案与绑定', 'Plan and binding validation'], preview: ['检查渲染与边界', 'Render and boundary checks'] }
  return labels[id] ? t(...labels[id]) : id
}
const stateLabel = (state: string) => ({ pending: props.waiting ? t('待继续', 'Awaiting continuation') : t('待开始', 'Pending'), running: t('进行中', 'In progress'), succeeded: t('已完成', 'Completed'), failed: t('未通过', 'Failed'), canceled: t('已取消', 'Canceled'), skipped: t('已跳过', 'Skipped') }[state])
const result = (step: AssistantStep) => step.id === 'metrics' && step.count ? t(`已选择 ${step.count} 项指标`, `${step.count} metrics selected`) : step.id === 'checking' && step.count ? t(`已检查 ${step.count} 项指标的数据能力`, `Data capability checked for ${step.count} metrics`) : step.id === 'generating' && step.count ? t(`已编排 ${step.count} 个区域或修改`, `${step.count} sections or edits arranged`) : step.id === 'preview' ? t('文字、组件和画布边界检查通过', 'Text, widgets and canvas bounds passed') : step.id === 'reviewing' ? t('指标引用、数据绑定与覆盖范围检查通过', 'Metric references, bindings and coverage passed') : t('本阶段结果已保存', 'Stage result saved')
</script>
<style scoped>
.assistant-steps{padding:16px;border:1px solid var(--border-color);border-radius:8px;background:var(--bg-body);font-size:12px}
.assistant-steps .assistant-steps-explanation{margin:0 0 16px;color:var(--text-secondary)}
.assistant-steps-heading,.assistant-step-title{display:flex;align-items:center;justify-content:space-between;gap:8px}.assistant-steps-heading{color:var(--text-primary);margin-bottom:16px}.assistant-steps-heading>span{color:var(--text-secondary);font-variant-numeric:tabular-nums;white-space:nowrap}.assistant-steps ol{list-style:none;margin:0;padding:0;display:grid;gap:16px}.assistant-steps li{display:grid;grid-template-columns:24px minmax(0,1fr);gap:8px;color:var(--text-secondary)}.assistant-step-marker{box-sizing:border-box;display:grid;place-items:center;width:24px;height:24px;border:1px solid var(--border-color);border-radius:50%;font-variant-numeric:tabular-nums}.assistant-step-title{min-height:24px;flex-wrap:wrap}.assistant-step-title>span{font-size:11px}.assistant-steps p{margin:8px 0 0;line-height:1.6;overflow-wrap:anywhere}.is-succeeded .assistant-step-marker{color:var(--success-color);border-color:var(--success-color)}.is-running .assistant-step-marker{color:var(--primary-color);border-color:var(--primary-color);position:relative}.is-running .assistant-step-marker::after{content:'';position:absolute;inset:-4px;border:2px solid var(--bg-body);border-top-color:var(--primary-color);border-radius:50%;animation:assistant-step-spin var(--assistant-progress-duration) linear infinite}.is-running strong{color:var(--text-primary)}.is-failed .assistant-step-marker,.is-failed .assistant-step-title{color:var(--danger-color);border-color:var(--danger-color)}
@keyframes assistant-step-spin{to{transform:rotate(360deg)}}
@media(prefers-reduced-motion:reduce){.is-running .assistant-step-marker::after{animation:none}}
</style>
