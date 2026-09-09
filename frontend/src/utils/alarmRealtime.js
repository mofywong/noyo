import { getAlarmEventKey, isAlarmEvent, isAlarmClearedEvent } from './alarmEvents.js'

// Both transports carry the same occurrence ID. All views react in one task;
// a player mounted by that task can replay the still-visible alert.
const listeners = new Set()
const seen = new Map()
const latest = new Map()
const watermarks = new Map()
const lifetime = 3000

export function publishAlarmEvent(event) {
  if (!isAlarmEvent(event)) return
  const key = getAlarmEventKey(event)
  if (seen.has(key)) return
  seen.set(key, true)
  if (seen.size > 500) seen.delete(seen.keys().next().value)
  const scope = JSON.stringify([event.device_code, event.params?.rule_id || event.event_id])
  const occurred = Number(event.params?.occurred_at || event.ts)
  if (occurred < (watermarks.get(scope) || 0)) return
  watermarks.set(scope, occurred)
  if (watermarks.size > 500) watermarks.delete(watermarks.keys().next().value)
  if (isAlarmClearedEvent(event)) latest.delete(scope)
  else latest.set(scope, { event, expires: Date.now() + lifetime })
  for (const [id, item] of latest) if (item.expires <= Date.now()) latest.delete(id)
  for (const listener of listeners) {
    try { listener(event) } catch (error) { console.error('Alarm listener failed', error) }
  }
}

export function subscribeAlarmEvents(listener, deviceCode) {
  listeners.add(listener)
  if (deviceCode) {
    for (const { event, expires } of latest.values()) {
      if (event.device_code === deviceCode && expires > Date.now()) listener(event)
    }
  }
  return () => listeners.delete(listener)
}

export function publishSceneAlarmMessage(message) {
  for (const report of message.reports || []) {
    const params = report.Params || report.params || {}
    publishAlarmEvent({
      device_code: message.deviceCode,
      event_id: report.EventID || report.event_id,
      params,
      ts: params.occurred_at || message.timestamp
    })
  }
}

export function resetAlarmEvents() {
  seen.clear()
  latest.clear()
  watermarks.clear()
}
