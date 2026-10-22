<template>
  <div class="go-decorates-number" :style="`width:${w}px;height:${h}px;`">
    <div
      :style="`color:${timeColor};font-size:${timeSize}px;line-height:${timeLineHeight}px;
      letter-spacing:${timeTextIndent}px;font-weight:${fontWeight};
      text-shadow: ${boxShadow}`"
    >
      {{ newData }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { PropType, toRefs, ref, watch, onMounted, onUnmounted } from 'vue'
import { CreateComponentType } from '@/packages/index.d'
import { useChartEditStore } from '@/store/modules/chartEditStore/chartEditStore'
import { useChartDataFetch } from '@/hooks'

const props = defineProps({
  chartConfig: {
    type: Object as PropType<CreateComponentType>,
    required: true
  }
})
function currentTime() {
  const date = new Date()
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}
const newData = ref(currentTime())
let boxShadow = ref('none')
let timer: any = null
const { w, h } = toRefs(props.chartConfig.attr)

let {
  timeColor,
  timeSize,
  timeLineHeight,
  timeTextIndent,
  fontWeight,
  showShadow,
  hShadow,
  vShadow,
  blurShadow,
  colorShadow
} = toRefs(props.chartConfig.option)

watch(
  props.chartConfig.option,
  () => {
    try {
      if (props.chartConfig.option.showShadow) {
        boxShadow.value = `${props.chartConfig.option.hShadow}px ${props.chartConfig.option.vShadow}px ${props.chartConfig.option.blurShadow}px ${props.chartConfig.option.colorShadow}`
      } else {
        boxShadow.value = 'none'
      }
    } catch (error) {
      console.log(error)
    }
  },
  {
    immediate: true
  }
)
onMounted(() => {
  timer = setInterval(() => { newData.value = currentTime() }, 500)
})
onUnmounted(() => {
  clearInterval(timer)
})
useChartDataFetch(props.chartConfig, useChartEditStore)
</script>

<style lang="scss" scoped>
@include go('decorates-number') {
  text-align: center;
}
</style>
