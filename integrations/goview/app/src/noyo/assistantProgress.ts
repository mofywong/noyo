export type StepState = 'pending' | 'running' | 'succeeded' | 'failed' | 'canceled' | 'skipped'
export type AssistantStep = { id: string; state: StepState; count?: number; detail?: string }
export const assistantStepIds = ['intention', 'metrics', 'checking', 'generating', 'reviewing', 'preview']

export function assistantProgress(message: { steps?: AssistantStep[]; stages?: string[]; status?: string }, preview?: { validated: boolean; issues: string[] }): AssistantStep[] {
  const stages = message.stages || []
  const waiting = message.status === 'waiting'
  const finished = ['ready', 'applied', 'undone', 'discarded', 'done'].includes(message.status || '')
  const current = stages.filter(id => assistantStepIds.includes(id) || id === 'validating').at(-1)
  const currentIndex = assistantStepIds.indexOf(current === 'validating' ? (stages.includes('generating') ? 'reviewing' : 'checking') : current || 'intention')
  return assistantStepIds.map((id, index) => {
    if (id === 'preview') return { id, state: waiting ? 'pending' : preview ? preview.validated ? preview.issues.length ? 'failed' : 'succeeded' : 'running' : ['applied', 'undone'].includes(message.status || '') ? 'succeeded' : 'pending' }
    const durable = message.steps?.find(step => step.id === id)
    if (waiting) {
      // A completed clarification round has no proposal. Older task packets mark
      // later steps skipped (or even succeeded); only reached stages can count.
      if (index > currentIndex || durable?.state === 'skipped') return { id, state: 'pending' }
      if (durable?.state === 'succeeded') return { ...durable }
      return { id, state: current && (!durable || durable.state === 'running') ? 'succeeded' : 'pending' }
    }
    if (durable) return { ...durable }
    return { id, state: finished || index < currentIndex ? 'succeeded' : index === currentIndex ? message.status === 'failed' ? 'failed' : message.status === 'canceled' ? 'canceled' : 'running' : 'pending' }
  })
}
