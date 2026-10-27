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

let latestData: any
const dataHandle = (newData: any) => {
  latestData = newData
  // Keep live readings out of the editor document and its proposal baseline.
  const config = cloneDeep(props.chartConfig.option)
  config.series[0].data[0].value = newData
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

// The render option is detached, so explicitly repaint authored style changes
// while retaining the last live sample instead of the saved placeholder.
watch(() => props.chartConfig.option, () => dataHandle(latestData), { deep: true })

// 预览时
useChartDataFetch(props.chartConfig, useChartEditStore, (resData: number) => {
  dataHandle(resData)
})
</script>
