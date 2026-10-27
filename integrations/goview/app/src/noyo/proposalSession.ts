import type { RenderQualityIssue, RenderQualityReport } from './renderQuality'

export type ValidationIdentity = { candidateRevision: number; candidateDigest: string; environmentDigest: string }
export type ProposalIssue = RenderQualityIssue & {
  origin?: 'baseline' | 'introduced' | 'worsened' | 'unknown'
  propertyPath?: string
  messageKey?: string
  suggestedActions?: string[]
}
export type ProposalValidation = Omit<RenderQualityReport, 'issues'> & ValidationIdentity & { issues: ProposalIssue[]; baselineChecked: boolean }
export const reportMatches = (a: ValidationIdentity, b: ValidationIdentity) => a.candidateRevision === b.candidateRevision && a.candidateDigest === b.candidateDigest && a.environmentDigest === b.environmentDigest

export function previewRequestFailure(error: any): ProposalIssue | undefined {
  if (![400, 401, 403, 413, 422].includes(Number(error?.code))) return
  return { code: 'data_error', severity: 'error', componentId: 'proposal', expected: 'a valid authorized preview document', actual: String(error?.message || error?.code).slice(0, 240) }
}

/** All proposal UI and command guards consume this selector. Task completion is separate. */
export function proposalDisposition(input: { candidate?: boolean; awaitingInput?: boolean; generating?: boolean; applying?: boolean; repairing?: boolean; conflict?: boolean; writable?: boolean; blocked?: boolean; validated?: boolean; issues?: ProposalIssue[]; occupied?: boolean; saving?: boolean }) {
  let state = 'idle', reason = ''
  if (input.applying) { state = 'applying'; reason = 'applying' }
  else if (input.generating) { state = 'generating'; reason = 'generating' }
  else if (input.awaitingInput) { state = 'awaiting_input'; reason = 'awaiting_input' }
  else if (input.candidate) {
    if (input.conflict) { state = 'conflict'; reason = 'conflict' }
    else if (input.repairing) { state = 'repairing'; reason = 'repairing' }
    else if (!input.validated) { state = 'validating'; reason = 'validating' }
    else if (input.blocked || input.issues?.some(issue => issue.severity === 'error')) { state = 'needs_fix'; reason = 'needs_fix' }
    else state = input.issues?.length ? 'ready_with_warnings' : 'ready'
    if (!reason && input.writable === false) reason = 'permission'
    if (!reason && input.saving) reason = 'saving'
    if (!reason && input.occupied) reason = 'occupied'
  }
  return { state, reason, canApply: !!input.candidate && !reason && ['ready', 'ready_with_warnings'].includes(state) }
}

export function flattenComponents(document: any): any[] {
  const flatten = (items: any[]): any[] => items.flatMap(item => item.isGroup ? [item, ...flatten(item.groupList || [])] : [item])
  return flatten(document?.content?.componentList || [])
}
const stable = (value: any): string => JSON.stringify(value, (_key, item) => item && typeof item === 'object' && !Array.isArray(item) ? Object.keys(item).sort().reduce((all: any, key) => { all[key] = item[key]; return all }, {}) : item)
function authoredComponent(document: any, id: string) {
  return { component: flattenComponents(document).find(item => item.id === id), targets: (document?.targets || []).filter((target: any) => target.componentId === id).map((target: any) => ({ ...target, binding: document.bindings?.[target.bindingId] })) }
}
const extent = (issue: RenderQualityIssue, axis: 'height' | 'width') => { const actual = issue.actual as any; return Number(actual?.[axis === 'height' ? 'requiredHeight' : 'requiredWidth'] ?? actual?.[axis] ?? 0) }
const presentation = new Set(['text_overflow', 'chart_text_overflow', 'bounds', 'chart_empty'])
/** Baseline uses the identical frozen samples, fonts and scale as the candidate. */
export function compareValidation(candidate: RenderQualityReport, baseline: RenderQualityReport | undefined, baseDocument: any, document: any): Omit<ProposalValidation, keyof ValidationIdentity> {
  const issues: ProposalIssue[] = candidate.issues.map(issue => {
    const previous = baseline?.issues.find(other => other.code === issue.code && other.componentId === issue.componentId)
    const unchanged = stable(authoredComponent(baseDocument, issue.componentId)) === stable(authoredComponent(document, issue.componentId))
    const worse = !!previous && (extent(issue, 'height') > extent(previous, 'height') + 2 || extent(issue, 'width') > extent(previous, 'width') + 2)
    const origin = previous ? worse ? 'worsened' : unchanged ? 'baseline' : 'introduced' : baseline ? 'introduced' : 'unknown'
    const advisory = origin === 'baseline' && presentation.has(issue.code)
    return { ...issue, origin, severity: advisory ? 'warning' : issue.severity, propertyPath: issue.code === 'text_overflow' ? 'option.dataset' : issue.code === 'bounds' ? 'attr' : undefined, messageKey: issue.code, suggestedActions: issue.severity === 'error' ? ['locate', 'repair', 'recheck', 'revert'] : ['locate', 'recheck'] }
  })
  return { ...candidate, issues, baselineChecked: !!baseline }
}

/** Lossless text compaction only. Geometry changes require an explicit layout scope. */
export function repairTextOverflow(source: any, issues: ProposalIssue[], constraints: { allowLayout: boolean }) {
  const document = JSON.parse(JSON.stringify(source)), nodes = flattenComponents(document), changedIds: string[] = []
  const bound = new Set((document.targets || []).map((target: any) => target.componentId))
  for (const issue of issues) {
    if (issue.severity !== 'error' || issue.code !== 'text_overflow') continue
    const node = nodes.find(item => item.id === issue.componentId)
    if (!node || node.chartConfig?.key !== 'TextCommon' || bound.has(node.id) || typeof node.option?.dataset !== 'string') continue
    // Preserve every word and paragraph meaning; only replace hard line wrapping.
    const compact = node.option.dataset.replace(/([^\n])\n[ \t]*(?=[^\n])/g, (_match: string, previous: string, offset: number, text: string) => previous + (/[\u3400-\u9fff。；，！？：]$/.test(previous) || /^[\u3400-\u9fff]/.test(text.slice(offset + _match.length)) ? '' : ' '))
    if (compact !== node.option.dataset) { node.option.dataset = compact; changedIds.push(node.id); continue }
    if (constraints.allowLayout) {
      const required = Number((issue.actual as any)?.requiredHeight)
      const nextHeight = Math.ceil(required / 8) * 8, attr = node.attr
      const canvasHeight = document.canvas?.height ?? document.content?.editCanvasConfig?.height ?? 1080
      const overlaps = nodes.some(other => other !== node && !other.isGroup && attr.x < other.attr.x + other.attr.w && attr.x + attr.w > other.attr.x && attr.y < other.attr.y + other.attr.h && attr.y + nextHeight > other.attr.y)
      if (Number.isFinite(nextHeight) && nextHeight > attr.h && attr.y + nextHeight <= canvasHeight && !overlaps) { attr.h = nextHeight; changedIds.push(node.id) }
    }
  }
  return { document, changed: changedIds.length > 0, changedIds }
}

/** Reverting one candidate component does not discard other successful changes. */
export function revertComponent(source: any, base: any, id: string) {
  const document = JSON.parse(JSON.stringify(source)), original = flattenComponents(base).find(item => item.id === id)
  if (!original) return null
  const replace = (items: any[]) => { for (let index = 0; index < items.length; index++) { if (items[index].id === id) items[index] = JSON.parse(JSON.stringify(original)); else if (items[index].isGroup) replace(items[index].groupList || []) } }
  replace(document.content.componentList)
  document.targets = [...(document.targets || []).filter((target: any) => target.componentId !== id), ...JSON.parse(JSON.stringify((base.targets || []).filter((target: any) => target.componentId === id)))]
  for (const target of base.targets || []) if (target.componentId === id && base.bindings?.[target.bindingId]) document.bindings[target.bindingId] = JSON.parse(JSON.stringify(base.bindings[target.bindingId]))
  return document
}
