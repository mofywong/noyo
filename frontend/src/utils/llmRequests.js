export const emptyLLMFilters = () => ({
  model: '', source: '', session_id: '', status: '',
  search: '', started_after: '', started_before: ''
})

// Model identifiers are free-form (a deployment may plug in any provider), so
// only the sources the backend can actually emit are translated; anything else
// keeps its raw identifier rather than showing an invented name.
export const LLM_SOURCE_MESSAGE_KEYS = {
  assistant: 'llm_source_assistant',
  ioc_screen_assistant: 'llm_source_ioc_screen_assistant',
  ioc_screen_title: 'llm_source_ioc_screen_title'
}

export function buildLLMRequestParams(filters) {
  const params = {}
  for (const [key, value] of Object.entries(filters)) {
    if (value === '' || value === null || value === undefined) continue
    if (key === 'started_after' || key === 'started_before') {
      const date = new Date(value)
      if (Number.isNaN(date.getTime())) throw new Error('invalid_dates')
      params[key] = date.toISOString()
    } else {
      const text = String(value).trim()
      if (text) params[key] = text
    }
  }
  if (params.started_after && params.started_before && params.started_after > params.started_before) {
    throw new Error('invalid_dates')
  }
  return params
}

// "name (id)" is the required presentation for conversations and sources; a
// missing name falls back to the identifier alone instead of repeating it.
export function llmEntityLabel(name, id, t) {
  const value = String(id ?? '').trim()
  if (!value) return ''
  const label = String(name ?? '').trim()
  return label ? t('llm_entity_label', { name: label, id: value }) : value
}

export function llmSourceLabel(source, t) {
  const value = String(source ?? '').trim()
  if (!value) return ''
  const key = LLM_SOURCE_MESSAGE_KEYS[value]
  return key ? llmEntityLabel(t(key), value, t) : value
}

export function llmSessionLabel(id, title, t) {
  const value = String(id ?? '').trim()
  if (!value) return ''
  return llmEntityLabel(String(title ?? '').trim() || t('llm_untitled_session'), value, t)
}

// List cells stack the name and the identifier so a long conversation title can
// never push the id out of view.
export function llmSourceName(source, t) {
  const key = LLM_SOURCE_MESSAGE_KEYS[String(source ?? '').trim()]
  return key ? t(key) : ''
}

export function llmSessionName(title, t) {
  return String(title ?? '').trim() || t('llm_untitled_session')
}

export function formatLLMPayload(value) {
  if (value === '' || value === null || value === undefined) return ''
  try { return JSON.stringify(typeof value === 'string' ? JSON.parse(value) : value, null, 2) }
  catch { return String(value) }
}
