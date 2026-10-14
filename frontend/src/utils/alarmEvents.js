export const ALARM_EVENT_IDS = [
  'illegal_parking_alarm',
  'fire_lane_occupied_alarm',
  'indoor_fire_passage_occupied_alarm',
  'object_missing_alarm',
  'area_intrusion_alarm',
  'area_intrusion_leave',
  'no_helmet_alarm',
  'no_vest_alarm',
  'no_chef_hat_alarm',
  'rat_detection_alarm',
  'cockroach_detection_alarm',
  'no_mask_alarm',
  'no_chef_uniform_alarm',
  'trash_bin_open_alarm',
  'trash_bin_overflow_alarm',
  'post_absence_alarm',
  'sleep_on_duty_alarm',
  'person_fall_alarm',
  'smoking_alarm',
  'fighting_alarm',
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
      no_vest: '未穿安全背心',
      illegal_parking: '机动车违法停车',
      fire_lane_occupied: '消防通道占用',
      indoor_fire_passage_occupied: '消防通道阻塞',
      fire_passage_blocked: '消防通道阻塞',
      object_missing: '物品丢失',
      area_intrusion: '区域入侵',
      area_intrusion_leave: '区域入侵离开',
      no_helmet: '未戴安全帽',
      no_chef_hat: '未戴厨师帽',
      rat_detection: '老鼠检测', cockroach_detection: '蟑螂检测', no_mask: '未戴口罩',
      no_chef_uniform: '未穿厨师服', trash_bin_open: '垃圾桶未盖', trash_bin_overflow: '垃圾桶满溢',
      post_absence: '岗位缺员',
      sleep_on_duty: '疑似睡岗',
      person_fall: '人员摔倒',
      smoking: '抽烟',
      fighting: '疑似打架'
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
      indoor_fire_passage_occupied: 'Fire Passage Blockage',
      fire_passage_blocked: 'Fire Passage Blockage',
      object_missing: 'Object Missing',
      area_intrusion: 'Area Intrusion',
      area_intrusion_leave: 'Area Intrusion Left',
      no_helmet: 'No Safety Helmet',
      no_vest: 'No Safety Vest',
      no_chef_hat: 'No Chef Hat',
      rat_detection: 'Rat Detection', cockroach_detection: 'Cockroach Detection', no_mask: 'No Face Mask',
      no_chef_uniform: 'No Chef Uniform', trash_bin_open: 'Open Trash Bin', trash_bin_overflow: 'Overflowing Trash Bin',
      post_absence: 'Post Understaffed',
      sleep_on_duty: 'Suspected Sleeping on Duty',
      person_fall: 'Person Fall',
      smoking: 'Smoking',
      fighting: 'Suspected Fighting'
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

export const isAlarmClearedEvent = (evt) => {
  if (!evt) return false
  if (evt.event_id === 'area_intrusion_leave') return true
  if (evt.params?.alarm_status === 'cleared' || evt.params?.alarm_status === 'left') return true
  if (String(evt.params?.alarm_phase).toLowerCase() === 'recovered') return true
  return false
}

export const isAlarmEvent = (evt) => {
  if (!evt) return false
  return Boolean(evt.params?.scene_type) || ALARM_EVENT_IDS.includes(evt.event_id)
}


export const getAlarmEventKey = (evt) => JSON.stringify([
  evt.device_code, evt.event_id, evt.params?.signal_id || evt.params?.occurred_at || evt.ts,
  evt.params?.rule_id || '', evt.params?.target_id || '', evt.params?.alarm_status || ''
])

export const mergeRecentAlarmEvents = (current, incoming) => {
  const events = new Map()
  for (const evt of [...current, ...incoming]) {
    if (isAlarmEvent(evt) && !isAlarmClearedEvent(evt)) events.set(getAlarmEventKey(evt), evt)
  }
  return [...events.values()].sort((a, b) => b.ts - a.ts).slice(0, 50)
}

export const findAlarmVideoDevice = (events, devices = {}, isVideoDevice = () => true) => {
  for (const evt of events || []) {
    if (!isAlarmEvent(evt) || isAlarmClearedEvent(evt)) continue
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
  const personnelScene = {
    no_helmet_alarm: 'no_helmet',
    no_vest_alarm: 'no_vest',
    no_chef_hat_alarm: 'no_chef_hat',
    rat_detection_alarm: 'rat_detection', cockroach_detection_alarm: 'cockroach_detection', no_mask_alarm: 'no_mask',
    no_chef_uniform_alarm: 'no_chef_uniform', trash_bin_open_alarm: 'trash_bin_open', trash_bin_overflow_alarm: 'trash_bin_overflow',
    post_absence_alarm: 'post_absence',
    sleep_on_duty_alarm: 'sleep_on_duty',
    person_fall_alarm: 'person_fall',
    smoking_alarm: 'smoking',
    fighting_alarm: 'fighting'
  }[evt?.event_id]
  if (personnelScene) {
    return getAlarmSceneLabel(personnelScene, locale)
  }
  if (eventDef?.name) {
    return eventDef.name
  }
  if (evt?.params?.rule_name) {
    if (evt.params.rule_name === '室内消防通道占用' || evt.params.rule_name === 'Indoor Fire Passage Occupied') {
      return messages[lang].scenes.indoor_fire_passage_occupied
    }
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
