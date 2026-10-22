// Noyo widget catalogue — the single source of truth for which upstream
// GoView components the embedded editor may construct, how each one consumes
// device data, and which binding target the host stores for it.
//
// `shape` is a *client-side* shaping contract: the backend only knows the seven
// safe targets (see backend/core/screen/document.go), while every component
// family needs a different payload. Editor, runtime and data hook all read this
// table so a new upstream component only has to be declared once.
//
// `static` means the component renders from its own options only: it has no
// device-telemetry semantics (decoration, media URL, form control, topology
// graph that cannot be derived from a single metric).
export type WidgetGroup = 'Charts' | 'VChart' | 'Tables' | 'Informations' | 'Decorates' | 'Icons'

export type WidgetShape =
  | 'chart'      // echarts dataset: { dimensions: ['ts', ...series], source: rows }
  | 'pie'        // echarts dataset: { dimensions: ['name', 'value'], source: rows }
  | 'radar'      // { radarIndicator, seriesData }
  | 'heatmap'    // { xAxis, yAxis, seriesData }
  | 'treemap'    // [{ name, value }]
  | 'words'      // [{ name, value }] for the word cloud
  | 'vchart'     // VChart records remapped onto the component's own fields
  | 'number'     // number
  | 'percent'    // number in 0..1
  | 'progress'   // number in 0..100
  | 'table'      // [{ name, value }]
  | 'table-basic' // { dimensions: [{ title, key }], source: rows }
  | 'table-grid' // string[][]
  | 'text'       // string
  | 'map'        // map points (Noyo location adapter)
  | 'static'

export interface WidgetSpec {
  key: string
  path: string
  group: WidgetGroup
  zh: string
  en: string
  shape: WidgetShape
}

type Row = [string, string, WidgetGroup, string, string, WidgetShape]

const rows: Row[] = [
  ['BarCommon', 'Charts/Bars/BarCommon', 'Charts', '柱状图', 'Bar chart', 'chart'],
  ['BarCrossrange', 'Charts/Bars/BarCrossrange', 'Charts', '横向柱状图', 'Horizontal bars', 'chart'],
  ['BarLine', 'Charts/Bars/BarLine', 'Charts', '柱状图 & 折线图', 'Bars and lines', 'chart'],
  ['CapsuleChart', 'Charts/Bars/CapsuleChart', 'Charts', '胶囊柱图', 'Capsule bars', 'chart'],
  ['Dial', 'Charts/Mores/Dial', 'Charts', '表盘', 'Gauge', 'number'],
  ['Funnel', 'Charts/Mores/Funnel', 'Charts', '漏斗图', 'Funnel', 'pie'],
  ['Graph', 'Charts/Mores/Graph', 'Charts', '关系图', 'Network graph', 'static'],
  ['Heatmap', 'Charts/Mores/Heatmap', 'Charts', '热力图', 'Heatmap', 'heatmap'],
  ['LineCommon', 'Charts/Lines/LineCommon', 'Charts', '折线图', 'Line chart', 'chart'],
  ['LineGradientSingle', 'Charts/Lines/LineGradientSingle', 'Charts', '单折线渐变面积图', 'Gradient area', 'chart'],
  ['LineGradients', 'Charts/Lines/LineGradients', 'Charts', '双折线渐变面积图', 'Gradient areas', 'chart'],
  ['LineLinearSingle', 'Charts/Lines/LineLinearSingle', 'Charts', '单折线渐变图', 'Gradient line', 'chart'],
  ['MapAmap', 'Charts/Maps/MapAmap', 'Charts', '高德地图', 'Amap device map', 'static'],
  ['MapBase', 'Charts/Maps/MapBase', 'Charts', '地图(可选省份)', 'China / province map', 'static'],
  ['PieCircle', 'Charts/Pies/PieCircle', 'Charts', '饼图-环形', 'Donut', 'percent'],
  ['PieCommon', 'Charts/Pies/PieCommon', 'Charts', '饼图', 'Pie chart', 'pie'],
  ['Process', 'Charts/Mores/Process', 'Charts', 'NaiveUI-进度', 'Progress', 'progress'],
  ['Radar', 'Charts/Mores/Radar', 'Charts', '雷达图', 'Radar', 'radar'],
  ['Sankey', 'Charts/Mores/Sankey', 'Charts', '桑基图', 'Sankey', 'static'],
  ['ScatterCommon', 'Charts/Scatters/ScatterCommon', 'Charts', '散点图', 'Scatter', 'chart'],
  ['ScatterLogarithmicRegression', 'Charts/Scatters/ScatterLogarithmicRegression', 'Charts', '对数回归散点图', 'Regression scatter', 'chart'],
  ['TreeMap', 'Charts/Mores/TreeMap', 'Charts', '树形分布', 'Treemap', 'treemap'],
  ['WaterPolo', 'Charts/Mores/WaterPolo', 'Charts', '水球图', 'Liquid fill', 'number'],
  ['VChartArea', 'VChart/Areas/VChartArea', 'VChart', 'VChart面积图', 'VChart area', 'vchart'],
  ['VChartBarCommon', 'VChart/Bars/VChartBarCommon', 'VChart', '并列柱状图-VChart', 'VChart grouped bars', 'vchart'],
  ['VChartBarCrossrange', 'VChart/Bars/VChartBarCrossrange', 'VChart', '并列柱状图-VChart', 'VChart horizontal bars', 'vchart'],
  ['VChartBarStack', 'VChart/Bars/VChartBarStack', 'VChart', '堆叠柱状图-VChart', 'VChart stacked bars', 'vchart'],
  ['VChartFunnel', 'VChart/Funnels/VChartFunnel', 'VChart', '漏斗图-VChart', 'VChart funnel', 'vchart'],
  ['VChartLine', 'VChart/Lines/VChartLine', 'VChart', '折线图-VChart', 'VChart line', 'vchart'],
  ['VChartPercentArea', 'VChart/Areas/VChartPercentArea', 'VChart', 'VChart百分比面积图', 'VChart percent area', 'vchart'],
  ['VChartPie', 'VChart/Pies/VChartPie', 'VChart', '饼图多欢-VChart', 'VChart pie', 'vchart'],
  ['VChartScatter', 'VChart/Scatters/VChartScatter', 'VChart', '散点图-VChart', 'VChart scatter', 'vchart'],
  ['VChartWordCloud', 'VChart/WordClouds/VChartWordCloud', 'VChart', '词云图-VChart', 'VChart word cloud', 'vchart'],
  ['TableList', 'Tables/Tables/TableList', 'Tables', '滚动排名列表', 'Ranking table', 'table'],
  ['TableScrollBoard', 'Tables/Tables/TableScrollBoard', 'Tables', '轮播列表', 'Scrolling table', 'table-grid'],
  ['TablesBasic', 'Tables/Tables/TablesBasic', 'Tables', '基础分页表格', 'Paginated table', 'table-basic'],
  ['Iframe', 'Informations/Mores/Iframe', 'Informations', '远程网页', 'Embedded webpage', 'static'],
  ['Image', 'Informations/Mores/Image', 'Informations', '图片', 'Image', 'static'],
  ['ImageCarousel', 'Informations/Mores/ImageCarousel', 'Informations', '轮播图', 'Image carousel', 'static'],
  ['InputsDate', 'Informations/Inputs/InputsDate', 'Informations', '时间选择器', 'Date picker', 'static'],
  ['InputsInput', 'Informations/Inputs/InputsInput', 'Informations', '输入框', 'Text input', 'static'],
  ['InputsPagination', 'Informations/Inputs/InputsPagination', 'Informations', '分页', 'Pagination', 'static'],
  ['InputsSelect', 'Informations/Inputs/InputsSelect', 'Informations', '下拉选择器', 'Select', 'static'],
  ['InputsTab', 'Informations/Inputs/InputsTab', 'Informations', '标签选择器', 'Tabs', 'static'],
  ['TextBarrage', 'Informations/Texts/TextBarrage', 'Informations', '弹幕文字', 'Scrolling text', 'text'],
  ['TextCommon', 'Informations/Texts/TextCommon', 'Informations', '文字', 'Text / metric', 'text'],
  ['TextGradient', 'Informations/Texts/TextGradient', 'Informations', '渐变文字', 'Gradient text', 'text'],
  ['Video', 'Informations/Mores/Video', 'Informations', '视频', 'Video', 'static'],
  ['WordCloud', 'Informations/Mores/WordCloud', 'Informations', '词云', 'Word cloud', 'words'],
  ['Border01', 'Decorates/Borders/Border01', 'Decorates', '边框-01', 'Border 01', 'static'],
  ['Border02', 'Decorates/Borders/Border02', 'Decorates', '边框-02', 'Border 02', 'static'],
  ['Border03', 'Decorates/Borders/Border03', 'Decorates', '边框-03', 'Border 03', 'static'],
  ['Border04', 'Decorates/Borders/Border04', 'Decorates', '边框-04', 'Border 04', 'static'],
  ['Border05', 'Decorates/Borders/Border05', 'Decorates', '边框-05', 'Border 05', 'static'],
  ['Border06', 'Decorates/Borders/Border06', 'Decorates', '边框-06', 'Border 06', 'static'],
  ['Border07', 'Decorates/Borders/Border07', 'Decorates', '边框-07', 'Border 07', 'static'],
  ['Border08', 'Decorates/Borders/Border08', 'Decorates', '边框-08', 'Border 08', 'static'],
  ['Border09', 'Decorates/Borders/Border09', 'Decorates', '边框-09', 'Border 09', 'static'],
  ['Border10', 'Decorates/Borders/Border10', 'Decorates', '边框-10', 'Border 10', 'static'],
  ['Border11', 'Decorates/Borders/Border11', 'Decorates', '边框-11', 'Border 11', 'static'],
  ['Border12', 'Decorates/Borders/Border12', 'Decorates', '边框-12', 'Border 12', 'static'],
  ['Border13', 'Decorates/Borders/Border13', 'Decorates', '边框-13', 'Border 13', 'static'],
  ['CirclePoint', 'Decorates/Mores/CirclePoint', 'Decorates', '圆点光环', 'Glowing point', 'static'],
  ['Clock', 'Decorates/Mores/Clock', 'Decorates', '时钟', 'Clock', 'static'],
  ['CountDown', 'Decorates/Mores/CountDown', 'Decorates', '倒计时', 'Countdown', 'static'],
  ['Decorates01', 'Decorates/Decorates/Decorates01', 'Decorates', '装饰-01', 'Decoration 01', 'static'],
  ['Decorates02', 'Decorates/Decorates/Decorates02', 'Decorates', '装饰-02', 'Decoration 02', 'static'],
  ['Decorates03', 'Decorates/Decorates/Decorates03', 'Decorates', '装饰-03', 'Decoration 03', 'static'],
  ['Decorates04', 'Decorates/Decorates/Decorates04', 'Decorates', '装饰-04', 'Decoration 04', 'static'],
  ['Decorates05', 'Decorates/Decorates/Decorates05', 'Decorates', '装饰-05', 'Decoration 05', 'static'],
  ['Decorates06', 'Decorates/Decorates/Decorates06', 'Decorates', '装饰-06', 'Decoration 06', 'static'],
  ['FlipperNumber', 'Decorates/Mores/FlipperNumber', 'Decorates', '数字翻牌-需动态触发', 'Flip counter', 'number'],
  ['FlowChartLine', 'Decorates/FlowChart/FlowChartLine', 'Decorates', '流程线', 'Flow line', 'static'],
  ['FullScreen', 'Decorates/Mores/FullScreen', 'Decorates', '全屏按钮', 'Fullscreen button', 'static'],
  ['Number', 'Decorates/Mores/Number', 'Decorates', '数字计数', 'Number counter', 'number'],
  ['PipelineH', 'Decorates/Mores/PipelineH', 'Decorates', '管道-横向', 'Horizontal pipeline', 'static'],
  ['PipelineV', 'Decorates/Mores/PipelineV', 'Decorates', '管道-纵向', 'Vertical pipeline', 'static'],
  ['ThreeEarth01', 'Decorates/Three/ThreeEarth01', 'Decorates', '三维地球', '3D globe', 'static'],
  ['TimeCommon', 'Decorates/Mores/TimeCommon', 'Decorates', '通用时间', 'Date and time', 'static'],
  ['Icon', 'Icons/Default/Icon', 'Icons', '图标', 'Icon', 'static'],
  ['LocationMap', 'noyo/LocationMap', 'Charts', '设备地图', 'Device map', 'map'],
]

export const widgets: WidgetSpec[] = rows.map(([key, path, group, zh, en, shape]) => ({ key, path, group, zh, en, shape }))

export const widgetByKey: Map<string, WidgetSpec> = new Map(widgets.map(item => [item.key, item]))

// Palette order. Charts first because that is what a screen is usually built
// from; decorations last because they are dropped on top of everything else.
export const widgetGroups: WidgetGroup[] = ['Charts', 'VChart', 'Tables', 'Informations', 'Decorates', 'Icons']

export const groupLabels: Record<WidgetGroup, [string, string]> = {
  Charts: ['图表', 'Charts'],
  VChart: ['VChart 图表', 'VChart'],
  Tables: ['列表', 'Tables'],
  Informations: ['信息', 'Information'],
  Decorates: ['装饰', 'Decorations'],
  Icons: ['图标', 'Icons']
}

// The stored target is one of the seven values the backend accepts. It is a
// shaping hint plus an authorization unit, not the payload itself.
const shapeTargets: Record<WidgetShape, string | null> = {
  chart: 'chart.dataset',
  pie: 'chart.dataset',
  radar: 'chart.dataset',
  heatmap: 'chart.dataset',
  treemap: 'chart.dataset',
  words: 'chart.dataset',
  vchart: 'chart.dataset',
  number: 'metric.value',
  percent: 'metric.value',
  progress: 'progress.value',
  table: 'table.rows',
  'table-basic': 'table.rows',
  'table-grid': 'table.rows',
  text: 'text.value',
  map: 'map.points',
  static: null
}

// Binding kinds each family can express. Anything not listed here is hidden in
// the inspector so a screen cannot store a binding that renders nothing.
const shapeKinds: Record<WidgetShape, string[]> = {
  chart: ['history', 'point', 'aggregate'],
  pie: ['history', 'point', 'aggregate'],
  radar: ['history', 'point'],
  heatmap: ['history', 'point'],
  treemap: ['history', 'point'],
  words: ['history', 'point'],
  vchart: ['history', 'point', 'aggregate'],
  number: ['point', 'aggregate', 'deviceSummary'],
  percent: ['point', 'aggregate', 'deviceSummary'],
  progress: ['point', 'aggregate', 'deviceSummary'],
  table: ['history', 'aggregate', 'alarmList', 'eventList'],
  'table-basic': ['history', 'aggregate', 'alarmList', 'eventList'],
  'table-grid': ['history', 'aggregate', 'alarmList', 'eventList'],
  text: ['point', 'aggregate', 'deviceStatus', 'deviceSummary'],
  map: ['location'],
  static: []
}

export function shapeOf(key: string | undefined): WidgetShape {
  return (key && widgetByKey.get(key)?.shape) || 'static'
}

export function targetOf(key: string | undefined): string | null {
  return shapeTargets[shapeOf(key)]
}

export function kindsOf(key: string | undefined): string[] {
  return shapeKinds[shapeOf(key)]
}

export function specOf(key: string | undefined): WidgetSpec | undefined {
  return key ? widgetByKey.get(key) : undefined
}
