// Upstream registers echarts modules one component at a time, and four of them
// register a different chart type from the series they actually render:
//
//   Dial    renders a `gauge` series but registers PieChart
//   Graph   renders a `graph` series but registers RadarChart
//   Sankey  renders a `sankey` series but registers RadarChart
//   MapBase never registers the `lines` series its flow overlay draws with
//
// An unregistered series is dropped silently — the widget keeps its frame and
// draws nothing, which is exactly the failure a build cannot catch. Registering
// them once here is additive: it enables those four renderers and cannot change
// a chart that already worked.
import { use } from 'echarts/core'
import { GaugeChart, GraphChart, LinesChart, SankeyChart } from 'echarts/charts'

use([GaugeChart, GraphChart, SankeyChart, LinesChart])
