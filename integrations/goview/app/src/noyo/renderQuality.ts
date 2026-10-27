/** Measured render checks, not an aesthetic score or a claim of complete visual coverage. */
export interface RenderQualityIssue {
  code: 'render_missing' | 'render_failed' | 'text_overflow' | 'chart_empty' | 'chart_text_overflow' | 'data_waiting' | 'data_error' | 'data_unchecked' | 'bounds'
  severity: 'error' | 'warning'
  componentId: string
  expected: unknown
  actual: unknown
}
export interface ChartRenderProbe {
  width: number
  height: number
  seriesCount: number
  renderedElementCount: number
  textBounds?: Array<{ x: number; y: number; width: number; height: number; text?: string }>
}
export interface RenderQualitySubject {
  componentId: string
  element: HTMLElement | null
  bound?: boolean
  quality?: string
  value?: unknown
  renderError?: string
  chartExpected?: boolean
  chart?: ChartRenderProbe
}
export interface RenderQualityReport {
  scope: 'measured-render'
  coverage: 'complete' | 'partial'
  checkedComponents: number
  issues: RenderQualityIssue[]
}
const finite = (value: number) => Number.isFinite(value)

/** The preview endpoint returns a dataset envelope; renderers consume rows. */
export function datasetPreviewSample(result: any) {
  if (!result || !Array.isArray(result.rows)) return { quality: 'error', value: undefined, timezone: undefined }
  const quality = result.meta?.quality || result.quality || 'good'
  return {
    quality: quality === 'good' && !result.rows.length ? 'no_data' : quality,
    value: result.rows,
    timezone: result.meta?.timezone
  }
}

/** Never infer missing data from truthiness: 0 and false are real samples. */
export function inspectRenderedComponents(subjects: RenderQualitySubject[]): RenderQualityReport {
  const issues: RenderQualityIssue[] = []
  let partial = false
  for (const item of subjects) {
    const add = (code: RenderQualityIssue['code'], severity: RenderQualityIssue['severity'], expected: unknown, actual: unknown) => issues.push({ code, severity, componentId: item.componentId, expected, actual })
    const element = item.element
    if (item.renderError) add('render_failed', 'error', 'component renders without an exception', item.renderError)
    if (!element || element.clientWidth <= 0 || element.clientHeight <= 0) {
      add('render_missing', 'error', 'a rendered component with nonzero dimensions', element ? { width: element.clientWidth, height: element.clientHeight } : null)
      continue
    }
    if (item.bound) {
      if (['error', 'query_error', 'access_denied'].includes(item.quality || '')) { partial = true; add('data_error', 'error', 'an authorized successful preview query', item.quality) }
      else if (['no_data', 'offline', 'stale', 'waiting'].includes(item.quality || '') || (item.quality === 'good' && item.value == null)) { partial = true; add('data_waiting', 'warning', 'a current sample for measuring data text', item.quality || 'waiting') }
      else if (item.quality !== 'good') { partial = true; add('data_unchecked', 'warning', 'a supported preview query', item.quality || 'unchecked') }
    }
    // Measure text nodes in the real DOM, including nested number/suffix/title
    // spans. The Range catches visible text clipped by an ancestor even when
    // the leaf element's own scroll dimensions do not reveal it.
    const bounds = element.getBoundingClientRect()
    const walker = element.ownerDocument.createTreeWalker(element, NodeFilter.SHOW_TEXT)
    let textNode: Node | null, overflow = false
    while ((textNode = walker.nextNode())) {
      if (!textNode.textContent?.trim()) continue
      const parent = textNode.parentElement
      if (!parent || parent.closest('[data-quality-ignore]')) continue
      const style = getComputedStyle(parent)
      if (style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0) continue
      const range = element.ownerDocument.createRange(); range.selectNodeContents(textNode)
      const text = range.getBoundingClientRect()
      if (!text.width || !text.height) continue
      let clipped = text.left < bounds.left - 2 || text.right > bounds.right + 2 || text.top < bounds.top - 2 || text.bottom > bounds.bottom + 2
      let ancestor: HTMLElement | null = parent
      while (ancestor && ancestor !== element) {
        const ancestorStyle = getComputedStyle(ancestor), box = ancestor.getBoundingClientRect()
        if (['hidden', 'clip'].includes(ancestorStyle.overflowX) && (text.left < box.left - 2 || text.right > box.right + 2)) clipped = true
        if (['hidden', 'clip'].includes(ancestorStyle.overflowY) && (text.top < box.top - 2 || text.bottom > box.bottom + 2)) clipped = true
        ancestor = ancestor.parentElement
      }
      if (clipped) {
        const scaleX = bounds.width / element.clientWidth || 1, scaleY = bounds.height / element.clientHeight || 1
        const requiredHeight = Math.ceil(Math.max(element.scrollHeight, (text.bottom - bounds.top) / scaleY, ...[...element.querySelectorAll<HTMLElement>('*')].filter(child => !child.closest('[data-quality-ignore]') && child.clientHeight > 0).map(child => child.scrollHeight + Math.max(0, (child.getBoundingClientRect().top - bounds.top) / scaleY))))
        add('text_overflow', 'error', { width: element.clientWidth, height: element.clientHeight }, { text: textNode.textContent.trim().slice(0, 160), width: Math.ceil(text.width / scaleX), height: Math.ceil(text.height / scaleY), requiredHeight, requiredWidth: Math.ceil(Math.max(element.scrollWidth, (text.right - bounds.left) / scaleX)) })
        overflow = true; break
      }
    }
    if (item.chartExpected && item.quality === 'good') {
      if (!item.chart || item.chart.seriesCount === 0 || item.chart.renderedElementCount === 0) add('chart_empty', 'error', 'a loaded chart with series and rendered elements', item.chart || null)
      else if (!overflow) {
        const chart = item.chart
        const clipped = chart.textBounds?.find(text => [text.x, text.y, text.width, text.height].every(finite) && (text.x < -2 || text.y < -2 || text.x + text.width > chart.width + 2 || text.y + text.height > chart.height + 2))
        if (clipped) add('chart_text_overflow', 'error', { width: chart.width, height: chart.height }, clipped)
      }
    }
  }
  return { scope: 'measured-render', coverage: partial ? 'partial' : 'complete', checkedComponents: subjects.length, issues }
}

/** A hard deadline also works in hidden tabs where requestAnimationFrame stops. */
export async function waitForRenderStability(root: HTMLElement, timeoutMs = 700): Promise<void> {
  const deadline = Date.now() + Math.max(0, timeoutMs)
  const pause = (ms: number) => new Promise<void>(resolve => setTimeout(resolve, ms))
  await Promise.race([root.ownerDocument.fonts?.ready || Promise.resolve(), pause(Math.max(0, deadline - Date.now()))])
  let previous = '', stable = 0
  while (Date.now() < deadline && root.isConnected) {
    await pause(Math.min(50, Math.max(0, deadline - Date.now())))
    const signature = [...root.querySelectorAll<HTMLElement>('[data-component-id]')].map(element => `${element.clientWidth}:${element.clientHeight}:${element.textContent}`).join('|')
    stable = signature === previous ? stable + 1 : 0; previous = signature
    if (stable >= 3) return
  }
}
