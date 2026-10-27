import { SCREEN_PROTOCOL } from './api.js'

export const SCREEN_RPC_TYPES = new Set([
  'screen.load', 'screen.save', 'screen.apply', 'screen.apply.status', 'screen.validate', 'screen.publish',
  'asset.upload', 'asset.resolve', 'asset.list', 'binding.catalog', 'runtime.snapshot',
  'runtime.subscribe', 'runtime.unsubscribe', 'runtime.history', 'runtime.alarms',
  'assistant.plan', 'assistant.history', 'assistant.message', 'assistant.conversations', 'assistant.conversation.create',
  'assistant.task.current', 'assistant.task.resume', 'assistant.task.cancel',
  'dataset.preview', 'binding.preview', 'dataset.list', 'dataset.load', 'dataset.save', 'runtime.data',
])

// A same-origin iframe isolates dependency and style trees, not untrusted JavaScript.
export function isScreenMessage(event, frameWindow, origin, channelId) {
  const message = event?.data
  return event?.source === frameWindow && event?.origin === origin
    && !!message && typeof message === 'object' && !Array.isArray(message)
    && message.protocol === SCREEN_PROTOCOL && message.channelId === channelId
    && typeof message.type === 'string'
}

export function screenMessage(channelId, type, payload, requestId) {
  return { protocol: SCREEN_PROTOCOL, channelId, type, payload: clonePayload(payload), ...(requestId ? { requestId } : {}) }
}

// Vue proxies cannot cross postMessage. GoView widgets also use class
// instances, so unwrap their enumerable data as well as plain JSON objects.
// Preserve structured-clone objects such as uploaded Files and Blobs.
function clonePayload(value) {
  if (Array.isArray(value)) return value.map(clonePayload)
  if (value && Object.prototype.toString.call(value) === '[object Object]') return Object.fromEntries(Object.entries(value).map(([key, item]) => [key, clonePayload(item)]))
  return value
}

export function engineUrl(entry, id, mode, channelId, origin) {
  const url = new URL(entry, origin)
  if (url.origin !== origin || !url.pathname.startsWith('/screen-editor/') || !url.pathname.endsWith('/')) {
    throw new Error('Invalid editor entry')
  }
  url.hash = `${mode === 'edit' ? '/chart/home/' : '/chart/preview/'}${encodeURIComponent(id)}?embed=1&channelId=${encodeURIComponent(channelId)}`
  return url.href
}

// No raw server text, credential, URL or stack trace is sent into the editor.
export function bridgeError(error, message) {
  const status = error?.response?.status
  const assistantCode = ['aiUnavailable', 'aiInvalid', 'aiTimeout', 'aiFailed', 'aiOutOfScope'].includes(error?.response?.data?.errorKey) ? error.response.data.errorKey : null
  const code = status === 409 || status === 412 ? 'DRAFT_CONFLICT'
    : status === 401 || status === 403 ? 'ACCESS_DENIED'
      : assistantCode || (error?.code === 'UNSUPPORTED' ? 'UNSUPPORTED' : 'REQUEST_FAILED')
  return { ok: false, error: { code, message } }
}
