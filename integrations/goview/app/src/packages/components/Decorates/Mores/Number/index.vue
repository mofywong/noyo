<template>
  <n-statistic tabular-nums class="go-decorates-number">
    <template #prefix>
      <span v-if="option.dataset != null" :style="`color:${prefixColor};font-size:${numberSize}px`">
        {{ prefixText }}
      </span>
    </template>
    <span :style="`color:${numberColor};font-size:${numberSize}px`">
      <n-number-animation
        v-if="option.dataset != null"
        :from="option.from ?? 0"
        :to="option.dataset"
        :duration="dur * 1000"
        :show-separator="showSeparator"
        :precision="precision"
      ></n-number-animation>
      <span v-else>—</span>
    </span>
    <template #suffix>
      <span v-if="option.dataset != null" :style="`color:${suffixColor};font-size:${numberSize}px`">
        {{ suffixText }}
      </span>
    </template>
  </n-statistic>
</template>

<script setup lang="ts">
import { PropType, toRefs, ref, reactive, watch } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartDataFetch } from '@/hooks'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})
const option = reactive<{from: number | null; dataset: number | null}>({
  from: 0,
  dataset: 0
})
const { w, h } = toRefs(props.chartConfig.attr)
let { dur, showSeparator, prefixText, prefixColor, suffixText, suffixColor, precision, numberSize, numberColor } =
  toRefs(props.chartConfig.option)

const updateNumber = (newData: number | null) => {
  // 原来的目标值作为新的数字动画的起始值
  option.from = option.dataset
  option.dataset = typeof newData === 'number' && Number.isFinite(newData) ? newData : null
}

watch(
  () => props.chartConfig.option.from,
  () => {
    option.from = props.chartConfig.option.from
  },
  { immediate: true }
)

watch(
  () => props.chartConfig.option.dataset,
  () => {
    option.dataset = props.chartConfig.option.dataset
  },
  {
    immediate: true,
    deep: false
  }
)

useChartDataFetch(props.chartConfig, useChartEditStore, updateNumber)
</script>

<style lang="scss" scoped>
@include go('decorates-number') {
  display: flex;
  justify-content: center;
  align-items: center;
}
</style>
