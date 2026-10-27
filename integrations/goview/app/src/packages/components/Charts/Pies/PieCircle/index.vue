<template>
  <v-chart :theme="themeColor" :init-options="initOptions" :option="option.value" autoresize> </v-chart>
</template>

<script setup lang="ts">
import { PropType, computed, reactive, watch } from 'vue'
import VChart from 'vue-echarts'
import { useCanvasInitOptions } from '@/hooks/useCanvasInitOptions.hook'
import { use } from 'echarts/core'
import { CanvasRenderer } from 'echarts/renderers'
import { PieChart } from 'echarts/charts'
import cloneDeep from 'lodash/cloneDeep'
import { mergeTheme } from '@/packages/public/chart'
import config, { includes } from './config'
import { useChartDataFetch } from '@/hooks'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { DatasetComponent, GridComponent, TooltipComponent, LegendComponent, TitleComponent } from 'echarts/components'
import { state } from '@/noyo/bridge'
import { bindingForComponent, formatPercent } from '@/noyo/shapes'

const props = defineProps({
  themeSetting: {
    type: Object,
    required: true
  },
  themeColor: {
    type: Object,
    required: true
  },
  chartConfig: {
    type: Object as PropType<config>,
    required: true
  }
})

const initOptions = useCanvasInitOptions(props.chartConfig.option, props.themeSetting)

use([DatasetComponent, CanvasRenderer, PieChart, GridComponent, TooltipComponent, LegendComponent, TitleComponent])

const option = reactive({
  value: {}
})

const displayBinding = computed(() => {
  const binding = bindingForComponent(state.document, props.chartConfig.id)
  const settings = props.chartConfig.option as any
  return {
    decimals: binding?.decimals ?? settings.precision,
    unit: binding?.unit || (typeof settings.suffixText === 'string' ? settings.suffixText.trim() : '') || '%'
  }
})
let latestData: any
const dataHandle = (newData: any) => {
  latestData = newData
  const ratio = typeof newData === 'number' ? newData : typeof newData === 'string' && newData.trim() ? Number(newData) : NaN
  const d = ratio * 100
  // Telemetry belongs to the rendered chart, not the saved editor document.
  // Mutating the prop here invalidates pending assistant proposals every tick.
  const config = cloneDeep(props.chartConfig.option)
  const available = Number.isFinite(d)
  config.title.text = formatPercent(ratio, displayBinding.value, state.locale)
  // Clear the gauge instead of passing NaN to ECharts or implying zero.
  const fill = available ? Math.min(100, Math.max(0, d)) : null
  config.series[0].data[0].value[0] = fill
  config.series[0].data[1].value[0] = fill == null ? null : 100 - fill
  // v-chart watches the option reference; reassigning the same object is a
  // no-op, so every sample publishes a fresh copy.
  option.value = mergeTheme(config, props.themeSetting, includes)
}

// 配置时
watch(
  () => props.chartConfig.option.dataset,
  newData => {
    try {
      dataHandle(newData)
    } catch (error) {
      console.log(error)
    }
  },
  {
    immediate: true,
    deep: false
  }
)

// Format changes also apply to the last live sample. Reading the saved
// placeholder here would briefly replace real telemetry after a locale,
// binding or document update.
watch([displayBinding, () => state.locale], () => dataHandle(latestData))

// 预览时
// Writing straight into option.value's inner fields does not trigger a render
// (it is a plain object), so the live path reuses dataHandle, which reassigns
// option.value and therefore repaints the ring and its centre label.
useChartDataFetch(props.chartConfig, useChartEditStore, (resData: number) => {
  dataHandle(resData)
})
</script>
