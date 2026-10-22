// Builds a screen document from a semantic blueprint using the real compiler,
// so what gets rendered is exactly what the AI pipeline would produce.
//   node tooling/make-ai-screen.mjs
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { createRequire } from 'node:module'
import { fileURLToPath } from 'node:url'

const require = createRequire(import.meta.url)
const here = path.dirname(fileURLToPath(import.meta.url))
const app = path.resolve(here, '../app')
const fixtures = path.resolve(here, '../tmp-verify/fixtures')
const esbuild = require(path.join(app, 'node_modules', 'esbuild'))

const entry = path.join(os.tmpdir(), `noyo-ai-entry-${process.pid}.mjs`)
const bundle = path.join(os.tmpdir(), `noyo-ai-${process.pid}.cjs`)
const posix = value => value.replace(/\\/g, '/')
fs.writeFileSync(entry, [
  `export { compileAssistantPlan } from '${posix(path.join(app, 'src', 'noyo', 'assistantPlan.js'))}'`,
  `export { expandBlueprint, applyScreenTheme } from '${posix(path.join(app, 'src', 'noyo', 'layout.ts'))}'`
].join('\n'))
esbuild.buildSync({ entryPoints: [entry], bundle: true, format: 'cjs', platform: 'node', target: 'node18', logLevel: 'silent', outfile: bundle })
const { compileAssistantPlan, expandBlueprint, applyScreenTheme } = require(bundle)
fs.unlinkSync(entry)
process.on('exit', () => { try { fs.unlinkSync(bundle) } catch {} })

const defaults = JSON.parse(fs.readFileSync(path.join(fixtures, 'widget-defaults.json'), 'utf8'))
const catalogue = new Map(defaults.map(item => [item.key, item]))
const widgets = {
  Border06: { target: null, kinds: [] },
  Decorates02: { target: null, kinds: [] },
  Number: { target: 'metric.value', kinds: ['point', 'aggregate'] },
  PieCircle: { target: 'metric.value', kinds: ['point', 'aggregate'] },
  Process: { target: 'progress.value', kinds: ['point', 'aggregate'] },
  TextCommon: { target: 'text.value', kinds: ['point', 'aggregate', 'deviceStatus'] },
  TimeCommon: { target: null, kinds: [] },
  LineCommon: { target: 'chart.dataset', kinds: ['history', 'point', 'aggregate'] },
  LineGradients: { target: 'chart.dataset', kinds: ['history', 'point', 'aggregate'] },
  BarCommon: { target: 'chart.dataset', kinds: ['history', 'point', 'aggregate'] },
  PieCommon: { target: 'chart.dataset', kinds: ['history', 'point', 'aggregate'] },
  TableScrollBoard: { target: 'table.rows', kinds: ['history', 'aggregate', 'alarmList'] }
}

function createComponent(key) {
  const template = catalogue.get(key)
  if (!template) throw new Error(`unknown widget ${key}`)
  return JSON.parse(JSON.stringify(template))
}

const devices = {
  fas: 'DEV_06_FAS_BACNET_GW',
  water: 'DEV_02_WATER_LEVEL',
  pump: 'DEV_01_PUMP_ROOM_CTRL',
  pipe: 'DEV_03_PIPE_PRESSURE',
  elec: 'DEV_04_ELEC_FIRE_DET',
  power: 'DEV_05_POWER_MONITOR',
  gas: 'DEV_09_GAS_DETECTOR',
  emergency: 'DEV_10_EMERG_LIGHT_POWER'
}

const blueprint = {
  summary: '消防设施运维监控大屏',
  canvas: { width: 1920, height: 1080 },
  sections: [
    { role: 'header', title: '消防设施物联网监控中心', subtitle: '示例数据 / Sample data · 火灾报警 / 水系统 / 电气火灾 / 应急电源' },
    {
      role: 'kpi',
      title: '实时告警总览 / Alarm Overview',
      items: [
        { form: 'count', label: '火警', device: devices.fas, key: 'AI_ALARM_COUNT', unit: '起', tone: 'danger' },
        { form: 'count', label: '故障', device: devices.fas, key: 'AI_FAULT_COUNT', unit: '个', tone: 'warning' },
        { form: 'count', label: '监管', device: devices.fas, key: 'AI_SUPERVISORY_COUNT', unit: '个', tone: 'info' },
        { form: 'count', label: '屏蔽', device: devices.fas, key: 'AI_SHIELD_COUNT', unit: '个', tone: 'neutral' }
      ]
    },
    {
      role: 'trend',
      title: '消防水池 / 屋顶水箱液位 · 24h（m）',
      span: 8,
      viz: 'area',
      window: '24h',
      items: [{ device: devices.water, keys: ['AI_POOL_UNDER_LEVEL', 'AI_TANK_ROOF_LEVEL'], unit: 'm', precision: 2 }]
    },
    {
      role: 'statusWall',
      title: '设备通讯状态',
      span: 4,
      items: [
        { label: '火灾报警主机', device: devices.fas },
        { label: '消防水泵房', device: devices.pump },
        { label: '水箱液位站', device: devices.water },
        { label: '管网压力仪', device: devices.pipe },
        { label: '电气火灾探测', device: devices.elec },
        { label: '电源监控箱', device: devices.power },
        { label: '燃气探测器', device: devices.gas },
        { label: '应急电源箱', device: devices.emergency }
      ]
    },
    {
      role: 'trend',
      title: '泵房出水总管压力 · 24h（MPa）',
      span: 6,
      viz: 'line',
      window: '24h',
      items: [{ device: devices.pump, keys: ['AI_OUTLET_PRESSURE'], unit: 'MPa', precision: 3 }]
    },
    {
      role: 'trend',
      title: '电气火灾剩余漏电电流 · 24h（mA）',
      span: 6,
      viz: 'line',
      window: '24h',
      items: [{ device: devices.elec, keys: ['AI_L1_RESIDUAL_CURRENT', 'AI_L2_RESIDUAL_CURRENT', 'AI_L3_RESIDUAL_CURRENT'], unit: 'mA', precision: 1 }]
    },
    { role: 'alarmTable', title: '最新告警事件', span: 12, devices: [devices.fas, devices.power] }
  ]
}

const source = {
  schemaVersion: 'noyo.screen/1',
  engine: { name: 'goview', buildId: 'goview-5ab05b4-noyo14', upstreamSha: '5ab05b439703ffda6d050e5626969459b3e3c1cf' },
  canvas: { width: 1920, height: 1080, scaleMode: 'contain' },
  content: { componentList: [], editCanvasConfig: { width: 1920, height: 1080 }, editCanvasScale: {} },
  bindings: {},
  targets: [],
  themeRefs: {}
}

const compiled = compileAssistantPlan(source, blueprint, {
  createComponent: key => createComponent(key),
  getWidget: key => (widgets[key] ? { key, ...widgets[key] } : null),
  locale: 'zh-CN',
  applyTheme: applyScreenTheme,
  expandBlueprint
})

const document = compiled.document
document.engine = source.engine
fs.writeFileSync(path.join(fixtures, 'ai-screen.json'), JSON.stringify(document, null, 1))

// Deterministic sample data: a plausible 24h shape so the render reflects what an
// operator would actually see, not a flat line.
const now = Date.UTC(2026, 9, 21, 4, 0, 0)
const points = 145
const series = {
  AI_POOL_UNDER_LEVEL: { base: 4.12, amp: 0.22, period: 26, noise: 0.02 },
  AI_TANK_ROOF_LEVEL: { base: 2.94, amp: 0.16, period: 31, noise: 0.015 },
  AI_OUTLET_PRESSURE: { base: 0.36, amp: 0.035, period: 18, noise: 0.004 },
  AI_L1_RESIDUAL_CURRENT: { base: 6.4, amp: 1.5, period: 22, noise: 0.3 },
  AI_L2_RESIDUAL_CURRENT: { base: 4.8, amp: 1.1, period: 27, noise: 0.25 },
  AI_L3_RESIDUAL_CURRENT: { base: 7.9, amp: 1.8, period: 19, noise: 0.35 }
}
function wave(key, index) {
  const spec = series[key]
  const seed = Math.sin(index * 12.9898 + key.length * 78.233) * 43758.5453
  const jitter = seed - Math.floor(seed)
  if (!spec) return Math.round(jitter * 10)
  const value = spec.base + Math.sin((index / spec.period) + (series[key].base % 3)) * spec.amp + (jitter - 0.5) * spec.noise
  return Number(value.toFixed(4))
}

const samples = { revision: 7, runtimeEpoch: 'ai-epoch', serverTime: now, bindings: {} }
const history = {}
const kpiValues = { AI_ALARM_COUNT: 0, AI_FAULT_COUNT: 2, AI_SUPERVISORY_COUNT: 1, AI_SHIELD_COUNT: 0 }
const statuses = ['online', 'online', 'offline', 'online', 'online', 'offline', 'online', 'unknown']

let statusIndex = 0
for (const [bindingId, binding] of Object.entries(document.bindings)) {
  if (binding.kind === 'alarmList') {
    history[bindingId] = { value: [
      { deviceCode: devices.fas, deviceName: '火灾报警主机', title: '通信回路故障', severity: 'warning', occurredAt: now - 720000 },
      { deviceCode: devices.power, deviceName: '电源监控箱', title: '备用电源故障', severity: 'warning', occurredAt: now - 1080000 },
      { deviceCode: devices.fas, deviceName: '火灾报警主机', title: '监管状态变化', severity: 'info', occurredAt: now - 1800000 }
    ], quality: 'good', receivedAt: now }
    continue
  }
  if (binding.kind === 'history') {
    const keys = binding.keys || [binding.key]
    const rows = Array.from({ length: points }, (_, index) => {
      const row = { ts: now - (points - 1 - index) * 10 * 60 * 1000 }
      for (const key of keys) row[key] = wave(key, index)
      return row
    })
    history[bindingId] = { value: rows, quality: 'good', receivedAt: now }
    continue
  }
  if (binding.kind === 'deviceStatus') {
    samples.bindings[bindingId] = { value: statuses[statusIndex++ % statuses.length], quality: 'good', receivedAt: now, sampleVersion: 3 + statusIndex }
    continue
  }
  const value = binding.key in kpiValues ? kpiValues[binding.key] : 1
  samples.bindings[bindingId] = { value, quality: 'good', receivedAt: now, sampleVersion: 12 }
}

fs.writeFileSync(path.join(fixtures, 'ai-samples.json'), JSON.stringify(samples, null, 1))
fs.writeFileSync(path.join(fixtures, 'ai-history.json'), JSON.stringify(history, null, 1))

const components = document.content.componentList
console.log(`components=${components.length} bindings=${Object.keys(document.bindings).length} targets=${document.targets.length}`)
if (compiled.notes.length) console.log('notes:', compiled.notes.join(' | '))
for (const component of components.slice(0, 8)) {
  const attr = component.attr
  console.log(`  ${String(component.chartConfig.key).padEnd(14)} ${component.id.padEnd(18)} ${attr.x},${attr.y} ${attr.w}x${attr.h}`)
}
