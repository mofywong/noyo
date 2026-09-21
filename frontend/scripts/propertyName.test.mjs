/**
 * Contract tests for the monitored-point display name resolver.
 *
 * Run: node scripts/propertyName.test.mjs
 *
 * The rules under test keep a raw point identifier ("AI_OUTPUT_CURRENT") out of
 * the operator-facing UI and keep each language free of the other language's
 * characters. The backend applies the same ladder in
 * noyo/backend/core/property_name.go at alarm-write time; this module covers the
 * rows written before that and the `property` event parameter.
 */
import assert from 'node:assert/strict'
import {
  buildProductPointIndex,
  localizePointCodeInText,
  prettifyPointCode,
  resolvePointName,
  resolvePointNameFromProperty,
} from '../src/utils/propertyName.js'
import { resolveServerText } from '../src/utils/serverText.js'

const product = (code, properties) => ({
  code,
  config: JSON.stringify({ tsl: { properties, events: [], services: [] } }),
})

const cabinet = product('P_FIRE_EMERG_LIGHT_POWER', [
  { identifier: 'AI_OUTPUT_CURRENT', name: '应急回路总输出电流' },
  { identifier: 'DO_FORCE_EMERGENCY', name: 'Remote force emergency lighting', name_zh: '远程强制启动应急照明' },
  { identifier: 'AI_BATTERY_VOLTAGE', name: JSON.stringify({ zh: '蓄电池组电压', en: 'Battery bank voltage' }) },
])
const index = buildProductPointIndex([cabinet])

// ---------------------------------------------------------------------------
// Identifier prettifying
// ---------------------------------------------------------------------------

assert.equal(prettifyPointCode('AI_OUTPUT_CURRENT'), 'AI OUTPUT CURRENT')
assert.equal(prettifyPointCode(' alarm-point '), 'alarm point')
assert.equal(prettifyPointCode(''), '')
assert.equal(prettifyPointCode(undefined), '')

// ---------------------------------------------------------------------------
// Chinese console: the configured Chinese name, never the identifier
// ---------------------------------------------------------------------------

assert.equal(
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_OUTPUT_CURRENT', 'zh'),
  '应急回路总输出电流',
)
assert.equal(
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_OUTPUT_CURRENT', 'zh-CN'),
  '应急回路总输出电流',
)

// ---------------------------------------------------------------------------
// English console: an explicit English name wins, and a Chinese-only label is
// never rendered in English (that is the mixed-language defect being fixed).
// ---------------------------------------------------------------------------

assert.equal(
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'DO_FORCE_EMERGENCY', 'en'),
  'Remote force emergency lighting',
)
assert.equal(
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_BATTERY_VOLTAGE', 'en'),
  'Battery bank voltage',
)
assert.equal(
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_OUTPUT_CURRENT', 'en'),
  'AI OUTPUT CURRENT',
)

// ---------------------------------------------------------------------------
// Unknown product / unknown point / empty identifier: still never a raw code,
// and never an empty string that would blank out a table cell.
// ---------------------------------------------------------------------------

assert.equal(resolvePointName(index, 'P_UNKNOWN', 'AI_OUTPUT_CURRENT', 'zh'), 'AI OUTPUT CURRENT')
assert.equal(resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'NOPE_METRIC', 'zh'), 'NOPE METRIC')
assert.equal(resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', '', 'zh'), '')
assert.equal(resolvePointName(undefined, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_OUTPUT_CURRENT', 'zh'), 'AI OUTPUT CURRENT')
assert.equal(resolvePointNameFromProperty(null, 'AI_OUTPUT_CURRENT', 'zh'), 'AI OUTPUT CURRENT')

// ---------------------------------------------------------------------------
// Legacy alarm titles still carry the identifier and are repaired at render time
// ---------------------------------------------------------------------------

assert.equal(
  localizePointCodeInText('AI 预测维护异常：AI_OUTPUT_CURRENT', 'AI_OUTPUT_CURRENT', '应急回路总输出电流'),
  'AI 预测维护异常：应急回路总输出电流',
)
assert.equal(
  localizePointCodeInText('AI predictive maintenance: AI_OUTPUT_CURRENT', 'AI_OUTPUT_CURRENT', 'AI OUTPUT CURRENT'),
  'AI predictive maintenance: AI OUTPUT CURRENT',
)
// Idempotent: a title that no longer contains the identifier is left untouched.
assert.equal(
  localizePointCodeInText('AI 预测维护异常：应急回路总输出电流', 'AI_OUTPUT_CURRENT', '应急回路总输出电流'),
  'AI 预测维护异常：应急回路总输出电流',
)
assert.equal(localizePointCodeInText('', 'AI_OUTPUT_CURRENT', 'x'), '')
assert.equal(localizePointCodeInText('text', '', 'x'), 'text')
assert.equal(localizePointCodeInText('text', 'text', ''), 'text')

// ---------------------------------------------------------------------------
// Product config parsing is defensive: a malformed config must not throw
// ---------------------------------------------------------------------------

const brokenIndex = buildProductPointIndex([
  { code: 'P_BROKEN', config: 'not json' },
  { code: 'P_NO_TSL', config: JSON.stringify({ points: [] }) },
  { code: '', config: JSON.stringify({ tsl: { properties: [] } }) },
  { config: JSON.stringify({ tsl: { properties: [] } }) },
])
assert.equal(brokenIndex.has('P_BROKEN'), true)
assert.equal(brokenIndex.get('P_BROKEN').size, 0)
assert.equal(resolvePointName(brokenIndex, 'P_BROKEN', 'SOME_POINT', 'zh'), 'SOME POINT')

// ---------------------------------------------------------------------------
// Regression: the alarm row that reported the defect.
//
// A row stored before the backend resolved point names holds the single-language
// sentence "AI predictive maintenance: AI_OUTPUT_CURRENT". Rendering it is a
// two-step chain — the legacy sentence table translates the sentence, then the
// point identifier is replaced by the configured name. Both consoles must end up
// free of the raw identifier.
// ---------------------------------------------------------------------------

const legacyStoredTitle = 'AI predictive maintenance: AI_OUTPUT_CURRENT'
const renderAlarmTitle = (locale) => localizePointCodeInText(
  resolveServerText(legacyStoredTitle, locale),
  'AI_OUTPUT_CURRENT',
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_OUTPUT_CURRENT', locale),
)

assert.equal(renderAlarmTitle('zh'), 'AI 预测维护异常：应急回路总输出电流')
assert.equal(renderAlarmTitle('en'), 'AI predictive maintenance: AI OUTPUT CURRENT')

// Bilingual rows (written after the backend contract) already carry both
// sentences; only the identifier needs substituting.
const bilingualStoredTitle = JSON.stringify({
  zh: 'AI 预测维护异常：AI_OUTPUT_CURRENT',
  en: 'AI predictive maintenance: AI_OUTPUT_CURRENT',
})
const renderBilingualTitle = (locale) => localizePointCodeInText(
  resolveServerText(bilingualStoredTitle, locale),
  'AI_OUTPUT_CURRENT',
  resolvePointName(index, 'P_FIRE_EMERG_LIGHT_POWER', 'AI_OUTPUT_CURRENT', locale),
)
assert.equal(renderBilingualTitle('zh'), 'AI 预测维护异常：应急回路总输出电流')
assert.equal(renderBilingualTitle('en'), 'AI predictive maintenance: AI OUTPUT CURRENT')

console.log('propertyName: all assertions passed')