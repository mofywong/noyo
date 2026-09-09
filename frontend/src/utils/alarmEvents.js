export const ALARM_EVENT_IDS = [
  'illegal_parking_alarm',
  'fire_lane_occupied_alarm',
  'indoor_fire_passage_occupied_alarm',
  'object_missing_alarm',
  'area_intrusion_alarm',
  'area_intrusion_leave',
  'rule_alarm'
]

const messages = {
  zh: {
    alarm: '告警',
    fault: '故障',
    info: '消息',
    left: '离开',
    event: '事件',
    deviceAlarm: (deviceName) => `设备: ${deviceName} 发生了告警事件`,
    deviceLeft: (deviceName) => `设备: ${deviceName} 目标已离开告警区域`,
    scenes: {
      illegal_parking: '机动车违法停车',
      fire_lane_occupied: '消防通道占用',
      indoor_fire_passage_occupied: '室内消防通道占用',
      object_missing: '物品丢失',
      area_intrusion: '区域入侵',
      area_intrusion_leave: '区域入侵离开'
    }
  },
  en: {
    alarm: 'Alarm',
    fault: 'Fault',
    info: 'Message',
    left: 'Left',
    event: 'Event',
    deviceAlarm: (deviceName) => `Device: ${deviceName} reported an alarm event`,
    deviceLeft: (deviceName) => `Device: ${deviceName} target left the alarm area`,
    scenes: {
      illegal_parking: 'Illegal Parking',
      fire_lane_occupied: 'Fire Lane Occupied',
      indoor_fire_passage_occupied: 'Indoor Fire Passage Occupied',
      object_missing: 'Object Missing',
      area_intrusion: 'Area Intrusion',
      area_intrusion_leave: 'Area Intrusion Left'
    }
  }
}

const normalizeLocale = (locale = 'zh') => String(locale).toLowerCase().startsWith('en') ? 'en' : 'zh'

const isAreaIntrusionLeave = (evt) => {
  return evt?.event_id === 'area_intrusion_leave' ||
    (evt?.params?.scene_type === 'area_intrusion' && evt?.params?.alarm_status === 'left')
}

const isAreaIntrusionAlarm = (evt) => {
  return evt?.event_id === 'area_intrusion_alarm' ||
    evt?.params?.scene_type === 'area_intrusion'
}

export const isAlarmEvent = (evt) => {
  if (!evt) return false
  return Boolean(evt.params?.scene_type) || ALARM_EVENT_IDS.includes(evt.event_id)
}

export const getAlarmEventKey = (evt) => JSON.stringify([
  evt.device_code, evt.event_id, evt.ts, evt.params?.rule_id || ''
])

export const mergeRecentAlarmEvents = (current, incoming) => {
  const events = new Map()
  for (const evt of [...current, ...incoming]) {
    if (isAlarmEvent(evt)) events.set(getAlarmEventKey(evt), evt)
  }
  return [...events.values()].sort((a, b) => b.ts - a.ts).slice(0, 50)
}

export const findAlarmVideoDevice = (events, devices = {}, isVideoDevice = () => true) => {
  for (const evt of events || []) {
    if (!isAlarmEvent(evt)) continue
    const device = devices?.[evt.device_code]
    if (device && isVideoDevice(device)) {
      return device
    }
  }
  return null
}

export const getAlarmSceneLabel = (sceneType, locale = 'zh') => {
  const lang = normalizeLocale(locale)
  return messages[lang].scenes[sceneType] || sceneType || '-'
}

export const getAlarmEventName = (evt, eventDef = null, locale = 'zh') => {
  const lang = normalizeLocale(locale)
  if (isAreaIntrusionLeave(evt)) {
    return messages[lang].scenes.area_intrusion_leave
  }
  if (isAreaIntrusionAlarm(evt)) {
    return messages[lang].scenes.area_intrusion
  }
  if (eventDef?.name) {
    return eventDef.name
  }
  if (evt?.params?.rule_name) {
    return evt.params.rule_name
  }
  if (evt?.params?.scene_type) {
    return getAlarmSceneLabel(evt.params.scene_type, locale)
  }
  return evt?.event_id || '-'
}

export const getAlarmEventTypeLabel = (evt, eventDef = null, locale = 'zh') => {
  const lang = normalizeLocale(locale)
  if (isAreaIntrusionLeave(evt)) {
    return messages[lang].left
  }
  const type = eventDef?.type
  if (type === 'alarm') return messages[lang].alarm
  if (type === 'fault') return messages[lang].fault
  if (type === 'info') return messages[lang].info
  if (type) return type
  if (evt?.params?.scene_type) return messages[lang].alarm
  return evt?._type === 2 ? messages[lang].alarm : messages[lang].info
}

export const getAlarmToastMessage = (evt, deviceName, locale = 'zh') => {
  const lang = normalizeLocale(locale)
  if (isAreaIntrusionLeave(evt)) {
    return messages[lang].deviceLeft(deviceName)
  }
  return messages[lang].deviceAlarm(deviceName)
}
