// POST SSE retains the host's authenticated scope headers. Only a validated
// result resolves the request; incomplete streams can never apply a patch.
export async function streamAssistantPlan(url, payload, { headers, signal, onProgress, fetcher = fetch }) {
  const response = await fetcher(url, { method: 'POST', headers: { ...headers, 'Content-Type': 'application/json' }, body: JSON.stringify({ ...payload, stream: true }), signal, credentials: 'same-origin', cache: 'no-store' })
  if (!response.ok) {
    const data = await response.json().catch(() => ({}))
    throw Object.assign(new Error('Assistant request failed'), { response: { status: response.status, data } })
  }
  if (!response.body || !response.headers.get('content-type')?.includes('text/event-stream')) throw new Error('Invalid assistant stream')
  const reader = response.body.getReader(), decoder = new TextDecoder()
  let buffer = '', result, completed = false
  try {
    while (!completed) {
      const chunk = await reader.read()
      buffer += decoder.decode(chunk.value, { stream: !chunk.done })
      if (buffer.length > 512 * 1024) throw new Error('Assistant stream exceeds limit')
      let match
      while ((match = /\r?\n\r?\n/.exec(buffer))) {
        const packet = buffer.slice(0, match.index); buffer = buffer.slice(match.index + match[0].length)
        const lines = packet.split(/\r?\n/)
        const event = lines.find(line => line.startsWith('event:'))?.slice(6).trim()
        const raw = lines.filter(line => line.startsWith('data:')).map(line => line.slice(5).trimStart()).join('\n')
        if (!raw) continue
        const data = JSON.parse(raw)
        if (event === 'error') throw Object.assign(new Error('Assistant generation failed'), { response: { status: 502, data } })
        if (event === 'result') { result = data; completed = true; break }
        if (event === 'stage' || event === 'summary') onProgress({ event, ...data })
      }
      if (chunk.done) break
    }
    if (!completed) throw new Error('Assistant stream interrupted')
    return result
  } finally { await reader.cancel().catch(() => {}); reader.releaseLock() }
}

// Durable tasks outlive this transport. Aborting a poll only disconnects the
// browser; cancellation is an explicit server operation.
export async function assistantTaskRequest(url, { method = 'GET', payload, headers, signal, fetcher = fetch } = {}) {
  const response = await fetcher(url, { method, headers: { ...headers, Accept: 'application/json', ...(payload !== undefined ? { 'Content-Type': 'application/json' } : {}) }, ...(payload !== undefined ? { body: JSON.stringify(payload) } : {}), signal, credentials: 'same-origin', cache: 'no-store' })
  const body = await response.json().catch(() => ({}))
  if (!response.ok || body.code !== 0) throw Object.assign(new Error('Assistant task request failed'), { response: { status: response.status, data: body } })
  return body.data
}

function taskDelay(ms, signal) {
  return new Promise((resolve, reject) => {
    if (signal?.aborted) { reject(new DOMException('Disconnected', 'AbortError')); return }
    const abort = () => { clearTimeout(timer); signal?.removeEventListener('abort', abort); reject(new DOMException('Disconnected', 'AbortError')) }
    const timer = setTimeout(() => { signal?.removeEventListener('abort', abort); resolve() }, ms)
    signal?.addEventListener('abort', abort, { once: true })
  })
}

export async function pollAssistantTask(taskUrl, initial, { headers, signal, onProgress = () => {}, fetcher = fetch, interval = 1500 } = {}) {
  let status = initial, failures = 0, lastStage = '', lastSummary = '', lastVersion = ''
  while (true) {
    if (!status?.taskId || !['queued', 'running', 'completed', 'failed', 'canceled'].includes(status.state)) throw new Error('Invalid assistant task')
    const progressVersion = `${status.version}:${status.state}:${status.stage}:${status.summary}:${JSON.stringify(status.steps || [])}`
    if (progressVersion !== lastVersion) { onProgress({ event: 'task', ...status, result: undefined }); lastVersion = progressVersion }
    if (status.stage && status.stage !== lastStage) { onProgress({ event: 'stage', stage: status.stage }); lastStage = status.stage }
    if (status.summary && status.summary !== lastSummary) { onProgress({ event: 'summary', content: status.summary }); lastSummary = status.summary }
    if (status.state === 'completed') {
      if (!status.result || typeof status.result !== 'object') throw new Error('Missing assistant result')
      return status.result
    }
    if (status.state === 'failed' || status.state === 'canceled') throw Object.assign(new Error('Assistant task stopped'), { response: { status: 502, data: { errorKey: status.errorKey || 'aiFailed', taskId: status.taskId, stage: status.stage, state: status.state } } })
    await taskDelay(interval * (failures + 1), signal)
    try {
      status = await assistantTaskRequest(`${taskUrl}/${encodeURIComponent(status.taskId)}`, { headers, signal, fetcher })
      failures = 0
    } catch (error) {
      if (signal?.aborted || error?.response?.status === 401 || error?.response?.status === 403 || ++failures >= 3) throw error
      // Keep the last known task ID and reconnect instead of creating a second
      // generation request when a status packet is lost.
    }
  }
}

export async function createAssistantTask(planUrl, payload, options) {
  const status = await assistantTaskRequest(planUrl, { ...options, method: 'POST', payload: { ...payload, stream: false, async: true } })
  return pollAssistantTask(planUrl.replace(/\/plan$/, '/tasks'), status, options)
}
