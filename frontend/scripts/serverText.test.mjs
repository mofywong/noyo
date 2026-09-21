/**
 * Contract tests for the server-text resolver.
 *
 * Run: node scripts/serverText.test.mjs
 *
 * The rules under test are the ones that keep backend-generated copy aligned
 * with the selected console language. A regression here reintroduces the
 * mixed-language alarm/work-order text reported for the Alarm Center.
 */
import assert from 'node:assert/strict'
import {
  isBilingualPayload,
  normalizeTextLocale,
  resolveServerText,
} from '../src/utils/serverText.js'

// ---------------------------------------------------------------------------
// Bilingual payloads (the contract used by every new server-generated string)
// ---------------------------------------------------------------------------

const alarmTitle = JSON.stringify({
  zh: 'AI 预测维护异常：AI_OUTPUT_CURRENT',
  en: 'AI predictive maintenance: AI_OUTPUT_CURRENT',
})

assert.equal(resolveServerText(alarmTitle, 'zh'), 'AI 预测维护异常：AI_OUTPUT_CURRENT')
assert.equal(resolveServerText(alarmTitle, 'en'), 'AI predictive maintenance: AI_OUTPUT_CURRENT')
assert.equal(resolveServerText(alarmTitle, 'en-US'), 'AI predictive maintenance: AI_OUTPUT_CURRENT')
assert.equal(resolveServerText(alarmTitle, 'zh-CN'), 'AI 预测维护异常：AI_OUTPUT_CURRENT')

// Objects (not only JSON strings) must resolve, because some endpoints hand the
// payload back as a decoded map.
assert.equal(
  resolveServerText({ zh: '工单抄送', en: 'Work order copied to you' }, 'en'),
  'Work order copied to you',
)

// A missing side falls back to the language that is present instead of blank.
assert.equal(resolveServerText({ zh: '仅中文' }, 'en'), '仅中文')
assert.equal(resolveServerText({ en: 'English only' }, 'zh'), 'English only')

// Unknown locale values must not fall through to a raw JSON blob.
assert.equal(resolveServerText(alarmTitle, undefined), 'AI 预测维护异常：AI_OUTPUT_CURRENT')
assert.equal(resolveServerText(alarmTitle, ''), 'AI 预测维护异常：AI_OUTPUT_CURRENT')

assert.equal(isBilingualPayload(alarmTitle), true)
assert.equal(isBilingualPayload({ zh: 'x', en: 'y' }), true)
assert.equal(isBilingualPayload('plain text'), false)
assert.equal(isBilingualPayload('{not json'), false)

// ---------------------------------------------------------------------------
// Legacy single-language rows written before the bilingual contract
// ---------------------------------------------------------------------------

assert.equal(
  resolveServerText('AI predictive maintenance: AI_OUTPUT_CURRENT', 'zh'),
  'AI 预测维护异常：AI_OUTPUT_CURRENT',
)
assert.equal(
  resolveServerText('AI predictive maintenance recovered: AI_OUTPUT_CURRENT', 'zh'),
  'AI 预测维护异常已恢复：AI_OUTPUT_CURRENT',
)
assert.equal(
  resolveServerText('AI predictive maintenance: AI_OUTPUT_CURRENT', 'en'),
  'AI predictive maintenance: AI_OUTPUT_CURRENT',
)

// Work-order notification titles: English rows render Chinese and Chinese rows
// render English, so neither direction stays mixed.
assert.equal(resolveServerText('Work order transferred to you', 'zh'), '工单已转派给你')
assert.equal(resolveServerText('New work order task', 'zh'), '新的工单任务')
assert.equal(resolveServerText('Work order approval required', 'zh'), '工单待审批')
assert.equal(resolveServerText('工单抄送', 'en'), 'Work order copied to you')
assert.equal(resolveServerText('来自流程节点自动抄送', 'en'), 'Automatically copied from a workflow node')
assert.equal(
  resolveServerText('Automatically created from alarm ai_predict:task-1', 'zh'),
  '由告警自动创建：ai_predict:task-1',
)

// Alarm target labels stored by the vision plugin in a single language.
assert.equal(resolveServerText('消防通道阻塞物', 'en'), 'Passage obstruction')
assert.equal(resolveServerText('消防通道阻塞已消除', 'en'), 'Passage obstruction cleared')

// ---------------------------------------------------------------------------
// Operator-authored text must never be rewritten
// ---------------------------------------------------------------------------

assert.equal(resolveServerText('更换 3 号泵密封件', 'en'), '更换 3 号泵密封件')
assert.equal(resolveServerText('Replace pump 3 seal', 'zh'), 'Replace pump 3 seal')
assert.equal(resolveServerText('设备频繁上下线', 'en', { legacy: false }), '设备频繁上下线')
assert.equal(resolveServerText('', 'zh'), '')
assert.equal(resolveServerText(null, 'en'), '')
assert.equal(resolveServerText(undefined, 'en'), '')
assert.equal(resolveServerText(0, 'en'), '0')

assert.equal(normalizeTextLocale('EN'), 'en')
assert.equal(normalizeTextLocale('zh-TW'), 'zh')
assert.equal(normalizeTextLocale(null), 'zh')

console.log('serverText: all assertions passed')
