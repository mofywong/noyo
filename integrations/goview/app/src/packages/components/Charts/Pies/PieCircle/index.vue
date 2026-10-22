<template>
  <v-chart :theme="themeColor" :init-options="initOptions" :option="option.value" autoresize> </v-chart>
</template>

<script setup lang="ts">
import { PropType, reactive, watch } from 'vue'
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

const dataHandle = (newData: any) => {
  const d = parseFloat(`${newData}`) * 100
  let config = props.chartConfig.option
  const available = Number.isFinite(d)
  config.title.text = available ? `${+d.toFixed(2)}%` : '—'
  // Clear the gauge instead of passing NaN to ECharts or implying zero.
  config.series[0].data[0].value[0] = available ? d : null
  config.series[0].data[1].value[0] = available ? 100 - d : null
  mergeTheme(props.chartConfig.option, props.themeSetting, includes)
  // v-chart watches the option reference; reassigning the same object is a
  // no-op, so every sample publishes a fresh copy.
  option.value = cloneDeep(props.chartConfig.option)
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

// 预览时
// Writing straight into option.value's inner fields does not trigger a render
// (it is a plain object), so the live path reuses dataHandle, which reassigns
// option.value and therefore repaints the ring and its centre label.
useChartDataFetch(props.chartConfig, useChartEditStore, (resData: number) => {
  dataHandle(resData)
})
</script>
