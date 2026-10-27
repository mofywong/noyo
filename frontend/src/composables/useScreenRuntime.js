import { onBeforeUnmount, ref } from 'vue'

/**
 * Screen runtime transport (noyo.screen/1).
 *
 * One authorized fetch-SSE connection per mounted screen. The browser cannot set
 * scope headers on EventSource, so the connection is created with fetch and the
 * existing Bearer/tenant/project headers. Reconnection always asks for a fresh
 * snapshot; the server sequence is connection-local and never a replay cursor.
 *
 * Every connection owns a generation number. Autosaving restarts the stream, so
 * a late frame from the previous connection must never be allowed to abort the
 * controller of the current one.
 */
const IDLE_TIMEOUT_MS = 45000
const BASE_BACKOFF_MS = 1000
const MAX_BACKOFF_MS = 30000
// A renewed access token is worth an immediate retry; the display should not
// sit dark for a whole backoff cycle because a 2h token quietly aged out.
const AUTH_RETRY_MS = 300

export function parseScreenStream(packet) {
  const lines = packet.split('\n')
  const name = lines.find((line) => line.startsWith('event:'))?.slice(6).trim() || 'message'
  const data = lines.filter((line) => line.startsWith('data:')).map((line) => line.slice(5).trimStart()).join('\n')
  return { name, data }
}

export function nextScreenBackoff(attempt) {
  const ceiling = Math.min(MAX_BACKOFF_MS, BASE_BACKOFF_MS * 2 ** Math.max(0, attempt))
  // Jitter keeps reconnecting viewers from synchronizing on the same instant.
  return Math.min(MAX_BACKOFF_MS, Math.round(ceiling * (0.8 + Math.random() * 0.4)))
}

export function useScreenRuntime() {
  // idle | connecting | live | lost
  const state = ref('idle')
  let controller = null
  let retryTimer = null
  let idleTimer = null
  let attempt = 0
  let stopped = true
  let current = null
  let generation = 0

  function clearTimers() {
    clearTimeout(retryTimer); retryTimer = null
    clearTimeout(idleTimer); idleTimer = null
  }
  function stop() {
    stopped = true
    generation++
    controller?.abort(); controller = null
    clearTimers()
    state.value = 'idle'
  }
  // `alive` is the single authority for a connection: it captures ownership of
  // the socket so a superseded connection cannot touch shared state.
  function aliveOf(mine) { return () => !stopped && mine === generation }
  function armIdle(alive) {
    clearTimeout(idleTimer)
    idleTimer = setTimeout(() => {
      if (!alive()) return
      // Silence is a lost link, not an empty dataset: drop and resync. The
      // aborted read is expected, so the catch path stays quiet about it.
      controller?.abort()
      schedule(alive)
    }, IDLE_TIMEOUT_MS)
  }
  function schedule(alive) {
    if (!alive() || !current) return
    state.value = 'lost'
    retryTimer = setTimeout(() => { retryTimer = null; if (alive()) connect(current) }, nextScreenBackoff(attempt++))
  }
  // Headers may be a live getter: an access token that was renewed between
  // connections must be read again, not replayed from the first attempt.
  function headersOf(options) {
    try { return typeof options.headers === 'function' ? options.headers() : options.headers } catch { return {} }
  }
  // The stream uses fetch, so the axios 401 interceptor never sees it. Without
  // this hop an expiring token freezes a wall display permanently: every later
  // reconnect answers 401 and the transport stops for good.
  async function renew(options, alive) {
    if (typeof options.onAuthExpired !== 'function') return false
    try { return (await options.onAuthExpired()) === true && alive() } catch { return false }
  }
  async function connect(options) {
    if (stopped) return
    const mine = ++generation
    const alive = aliveOf(mine)
    const local = new AbortController()
    controller = local
    const signal = local.signal
    state.value = 'connecting'
    try {
      const response = await fetch(options.url, { headers: headersOf(options), signal, cache: 'no-store', credentials: 'same-origin' })
      if (!alive()) return
      if (!response.ok || !response.body) {
        if (response.status === 401) {
          const renewed = await renew(options, alive)
          // The await may have outlived this connection; only its owner may
          // decide to stop the transport.
          if (!alive()) return
          if (renewed) {
            // Quiet retry: no error is surfaced and no backoff is burned.
            attempt = 0
            state.value = 'connecting'
            retryTimer = setTimeout(() => { retryTimer = null; if (alive()) connect(current) }, AUTH_RETRY_MS)
            return
          }
        }
        if ([401, 403, 404, 410, 412].includes(response.status)) {
          stopped = true
          state.value = 'idle'
          options.onTerminal(response.status === 412 ? 'conflict' : response.status === 401 ? 'unauthorized' : 'revoked', response.status)
          return
        }
        throw new Error('Stream unavailable')
      }
      state.value = 'live'
      attempt = 0
      armIdle(alive)
      const reader = response.body.getReader()
      const decoder = new TextDecoder()
      let buffer = ''
      while (alive()) {
        const chunk = await reader.read()
        if (!alive()) return
        if (chunk.done) break
        buffer += decoder.decode(chunk.value, { stream: true }).replace(/\r\n/g, '\n')
        let boundary
        while ((boundary = buffer.indexOf('\n\n')) !== -1) {
          const { name, data } = parseScreenStream(buffer.slice(0, boundary))
          buffer = buffer.slice(boundary + 2)
          if (!data) continue
          armIdle(alive)
          let payload
          try { payload = JSON.parse(data) } catch { continue }
          if (options.onEvent(name, payload)) {
            // A handler may already have started a replacement connection (a
            // resync does exactly that). Only the owner may stop the transport.
            if (alive()) { stopped = true; state.value = 'idle' }
            local.abort()
            return
          }
        }
        if (buffer.length > 1024 * 1024) throw new Error('Stream frame too large')
      }
      if (alive()) throw new Error('Stream closed')
    } catch (error) {
      if (!alive() || signal.aborted) return
      options.onError?.(error)
      schedule(alive)
    } finally {
      if (mine === generation && controller === local) controller = null
    }
  }
  function start(options) {
    stop()
    stopped = false
    attempt = 0
    current = options
    return connect(options)
  }
  onBeforeUnmount(stop)
  return { state, start, stop }
}
