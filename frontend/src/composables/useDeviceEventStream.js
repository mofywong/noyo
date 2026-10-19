/**
 * 设备事件流（SSE）的共享订阅入口。
 *
 * 为什么要做成单例：
 * 浏览器对同一 origin 的并发连接数是有限的（HTTP/1.1 通常 6 条），而 EventSource
 * 是**长连接**，一条流会一直占着一个槽位。顶栏、设备列表、网关管理此前各自
 * `new EventSource(...)`，同一页面里最多能同时存在三条同源长连接；设备地图若再
 * 直连一条，切几次标签页就有把连接池吃满的风险 —— 表现是"整站请求排队、接口变慢"，
 * 而原因离现象很远，极难排查。
 *
 * 这里用「一个连接 + 多个订阅者」收敛：引用计数归零时关闭，有人订阅时按需建立。
 *
 * 三条硬约束：
 * 1. **不做静默失败**。订阅方拿到 `onError` 后必须自己降级（设备地图退回轮询），
 *    否则权限不足时会表现为"页面数据永远不更新"。
 * 2. **心跳看门狗**。后端每 15s 发一次 `heartbeat`；40s 收不到就说明连接已经
 *    半开（NAT 超时、代理静默断开），此时 EventSource 不会触发 error，也不会重连。
 * 3. **引用计数**必须在组件卸载时归还，否则 SPA 路由反复切换后连接永不释放。
 */

const STREAM_PATH = '/api/devices/stream'
/** 后端心跳 15s 一次。留 2 个周期余量，避免网络抖动就重连。 */
const ACTIVITY_TIMEOUT_MS = 45 * 1000
const WATCHDOG_INTERVAL_MS = 10 * 1000
const RETRY_DELAY_MS = 3000

/** 所有连接级事件（不是业务事件）。业务事件由订阅者按需声明。 */
const CONTROL_EVENTS = ['connected', 'heartbeat']

const subscribers = new Set()
const boundTypes = new Set()

let source = null
let retryTimer = 0
let watchdogTimer = 0
let lastActivityAt = 0
let stopped = false
/** 连续失败次数：用于退避重连，open 成功时清零。 */
let failureCount = 0

function currentToken() {
  try {
    return localStorage.getItem('access_token') || ''
  } catch (error) {
    return ''
  }
}

function notifyError(error) {
  subscribers.forEach((entry) => {
    try {
      entry.onError?.(error)
    } catch (err) {
      /* 订阅者自身的异常不能中断其它订阅者 */
    }
  })
}

function bindType(type) {
  if (!source || boundTypes.has(type)) return
  boundTypes.add(type)
  source.addEventListener(type, (event) => {
    lastActivityAt = Date.now()
    let payload = null
    try {
      payload = typeof event.data === 'string' && event.data ? JSON.parse(event.data) : event.data
    } catch (error) {
      // 解析失败仍然继续：有些事件只用于"提示刷新"，负载不是必需的。
      payload = null
    }
    subscribers.forEach((entry) => {
      if (entry.events.length && !entry.events.includes(type)) return
      try {
        entry.onEvent?.(type, payload)
      } catch (error) {
        /* 单个订阅者异常不影响其它订阅者 */
      }
    })
  })
}

function closeSource() {
  if (source) {
    try {
      source.close()
    } catch (error) {
      /* ignore */
    }
  }
  source = null
  boundTypes.clear()
}

function stopWatchdog() {
  if (watchdogTimer) {
    clearInterval(watchdogTimer)
    watchdogTimer = 0
  }
}

function startWatchdog() {
  stopWatchdog()
  watchdogTimer = setInterval(() => {
    if (!source) return
    if (Date.now() - lastActivityAt <= ACTIVITY_TIMEOUT_MS) return
    // 半开连接：readystate 仍是 OPEN，但服务端早已不在。必须主动重建。
    closeSource()
    scheduleConnect(0)
  }, WATCHDOG_INTERVAL_MS)
}

function scheduleConnect(delay = RETRY_DELAY_MS) {
  if (retryTimer || stopped) return
  retryTimer = setTimeout(() => {
    retryTimer = 0
    connect()
  }, delay)
}

function connect() {
  if (stopped || subscribers.size === 0 || source) return

  const token = currentToken()
  if (!token) {
    // 未登录 / token 已被清掉：不建立连接，交给订阅方降级。
    notifyError({ reason: 'unauthenticated' })
    return
  }

  let next = null
  try {
    next = new EventSource(`${STREAM_PATH}?token=${encodeURIComponent(token)}`)
  } catch (error) {
    notifyError({ reason: 'unsupported', error })
    return
  }
  source = next

  lastActivityAt = Date.now()

  next.addEventListener('open', () => {
    lastActivityAt = Date.now()
    failureCount = 0
    subscribers.forEach((entry) => {
      try {
        entry.onOpen?.()
      } catch (error) {
        /* ignore */
      }
    })
  })

  CONTROL_EVENTS.forEach((type) => {
    next.addEventListener(type, () => {
      lastActivityAt = Date.now()
    })
  })

  // 订阅者可能在不同时间点声明不同事件类型，建立连接后统一补齐。
  subscribers.forEach((entry) => entry.events.forEach(bindType))

  next.onerror = () => {
    // EventSource 自己会重连，但连接被服务端拒绝（403 / 401）时它会一直
    // 立刻重试，形成高频请求。这里统一切到"手动退避重连 + 事件通知"，
    // 让订阅方有机会降级到轮询。
    notifyError({ reason: 'disconnected', readyState: next.readyState })
    if (next.readyState !== EventSource.CLOSED) return
    closeSource()
    failureCount += 1
    // 3s 起，逐次翻倍，封顶 30s。服务端恢复后第一次成功 open 会重置计数。
    scheduleConnect(Math.min(30000, RETRY_DELAY_MS * Math.pow(2, Math.min(failureCount - 1, 4))))
  }

  startWatchdog()
}

function ensureConnected() {
  stopped = false
  if (source) {
    // 连接已存在时，把新订阅声明的事件类型补绑上。
    subscribers.forEach((entry) => entry.events.forEach(bindType))
    return
  }
  connect()
}

/**
 * 订阅设备事件流。
 *
 * @param {object} options
 * @param {string[]} [options.events] 关心的业务事件类型；空数组表示全部
 * @param {(type: string, payload: any) => void} [options.onEvent]
 * @param {() => void} [options.onOpen] 连接建立 / 重连成功（用于做一次校准拉取）
 * @param {(info: object) => void} [options.onError] 连接异常（订阅方据此降级）
 * @returns {() => void} 取消订阅
 */
export function subscribeDeviceEvents({ events = [], onEvent, onOpen, onError } = {}) {
  const entry = { events: [...events], onEvent, onOpen, onError }
  subscribers.add(entry)
  ensureConnected()

  return () => {
    subscribers.delete(entry)
    if (subscribers.size === 0) {
      stopped = true
      if (retryTimer) {
        clearTimeout(retryTimer)
        retryTimer = 0
      }
      stopWatchdog()
      closeSource()
    }
  }
}

/** 供测试 / 退出登录时强制重置（生产代码不调用）。 */
export function __resetDeviceEventStream() {
  subscribers.clear()
  stopped = true
  if (retryTimer) {
    clearTimeout(retryTimer)
    retryTimer = 0
  }
  stopWatchdog()
  closeSource()
}
