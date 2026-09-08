<template>
  <component
    :is="as"
    ref="rootEl"
    :class="policy.classes"
    :aria-labelledby="labelledBy || undefined"
    :data-glass-profile="policy.profile"
    :data-glass-composition="policy.composition"
  >
    <span class="noyo-glass-group__backdrop" aria-hidden="true"></span>
    <span class="noyo-glass-group__tint" aria-hidden="true"></span>
    <span class="noyo-glass-group__rim" aria-hidden="true"></span>
    <div class="noyo-glass-group__content">
      <slot />
    </div>
  </component>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useGlassPointer } from '../../composables/useGlassPointer'
import { resolveGlassGroupPolicy } from '../../utils/liquidGlassPolicy.js'

interface GlassGroupProps {
  profile?: 'regular' | 'floating' | 'clear-media'
  shape?: 'rounded' | 'pill' | 'circle' | 'panel'
  composition?: 'control' | 'island'
  as?: string
  interactive?: boolean
  labelledBy?: string
}

const props = withDefaults(defineProps<GlassGroupProps>(), {
  profile: 'regular',
  shape: 'rounded',
  composition: 'control',
  as: 'div',
  interactive: false,
  labelledBy: '',
})

const rootEl = ref<HTMLElement | null>(null)
const policy = computed(() => resolveGlassGroupPolicy(props))
const { bind, unbind } = useGlassPointer(rootEl)

onMounted(() => {
  if (props.interactive) bind(rootEl.value)
})

onBeforeUnmount(() => {
  unbind()
})

defineExpose({
  getElement: () => rootEl.value,
})
</script>
