import { kindsOf, widgetByKey } from './widgets'

export type PresentationTask = 'current' | 'change' | 'compare' | 'composition' | 'locate' | 'respond' | 'diagnose'

export interface PresentationCapability {
  key: string
  roles: string[]
  minWidth: number
  minHeight: number
}

// Only registered, adapted families participate in automatic selection. Binding
// support comes from widgets.ts; it is deliberately not copied into this table.
export const presentationCapabilities: PresentationCapability[] = [
  { key: 'Number', roles: ['kpi'], minWidth: 128, minHeight: 32 },
  { key: 'TextCommon', roles: ['kpi'], minWidth: 128, minHeight: 32 },
  { key: 'PieCircle', roles: ['kpi'], minWidth: 128, minHeight: 64 },
  { key: 'Process', roles: ['kpi'], minWidth: 128, minHeight: 32 },
  { key: 'LineCommon', roles: ['trend'], minWidth: 168, minHeight: 64 },
  { key: 'LineGradients', roles: ['trend'], minWidth: 168, minHeight: 64 },
  { key: 'BarCommon', roles: ['trend'], minWidth: 168, minHeight: 64 },
  { key: 'BarCrossrange', roles: ['distribution'], minWidth: 168, minHeight: 64 },
  { key: 'PieCommon', roles: ['distribution'], minWidth: 320, minHeight: 200 },
  { key: 'TableScrollBoard', roles: ['alarmTable', 'eventTable'], minWidth: 168, minHeight: 64 },
  { key: 'TableList', roles: ['list'], minWidth: 168, minHeight: 64 },
  { key: 'LocationMap', roles: ['map'], minWidth: 168, minHeight: 64 }
]

export function presentationCapability(key: string | undefined): PresentationCapability | undefined {
  return presentationCapabilities.find(candidate => candidate.key === key)
}

export function fitsPresentation(candidate: PresentationCapability, width: number, height: number): boolean {
  return Number.isFinite(width) && Number.isFinite(height) && candidate.minWidth <= width && candidate.minHeight <= height
}

export interface PresentationRequest {
  role: string
  task?: PresentationTask
  hint?: string
  seriesCount?: number
  categoryCount?: number
  width: number
  height: number
  kind?: string
}

export function selectPresentation(request: PresentationRequest): string {
  const { role, task, width, height, kind } = request
  const hint = String(request.hint || '').toLowerCase()
  const preferred = role === 'kpi'
    ? ({ percent: 'PieCircle', progress: 'Process', text: 'TextCommon', count: 'Number' } as Record<string, string>)[hint]
    : role === 'trend'
      ? ({ line: (request.seriesCount || 1) > 1 ? 'LineGradients' : 'LineCommon', area: 'LineGradients', bar: 'BarCommon' } as Record<string, string>)[hint]
      : role === 'distribution'
        ? ({ bar: 'BarCrossrange', pie: 'PieCommon', donut: 'PieCommon' } as Record<string, string>)[hint]
        : undefined
  const compatible = presentationCapabilities.filter(candidate => candidate.roles.includes(role) && widgetByKey.has(candidate.key)
    && (!kind || kind === 'dataset' || kindsOf(candidate.key).includes(kind)))
  if (!compatible.length) throw new Error('INVALID_METRIC_KIND')
  // A visualization hint expresses a preference, not permission to bypass the
  // same readability contract used by compilation and preflight.
  if (preferred && compatible.some(candidate => candidate.key === preferred && fitsPresentation(candidate, width, height))) return preferred
  const scores: Record<string, number> = role === 'kpi' ? { Number: 100 }
    : role === 'trend' ? { LineCommon: 90, LineGradients: (request.seriesCount || 1) > 1 ? 100 : 80 }
      : role === 'distribution' ? { BarCrossrange: 100, PieCommon: task === 'composition' && (request.categoryCount || 0) >= 2 && request.categoryCount! <= 5 ? 120 : 0 }
        : { TableScrollBoard: 100, TableList: 100, LocationMap: 100 }
  const ranked = compatible.filter(candidate => fitsPresentation(candidate, width, height) && scores[candidate.key] > 0)
    .sort((a, b) => scores[b.key] - scores[a.key])
  if (!ranked.length) throw new Error('LAYOUT_CAPACITY')
  return ranked[0].key
}
