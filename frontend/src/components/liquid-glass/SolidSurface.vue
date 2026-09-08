<template>
  <component
    :is="as"
    :class="policy.classes"
    :aria-labelledby="labelledBy || undefined"
    data-surface-profile="solid-content"
  >
    <slot />
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { resolveSolidSurfacePolicy } from '../../utils/liquidGlassPolicy.js'

interface SolidSurfaceProps {
  density?: 'comfortable' | 'compact'
  elevation?: 'flat' | 'raised'
  as?: string
  interactive?: boolean
  labelledBy?: string
}

const props = withDefaults(defineProps<SolidSurfaceProps>(), {
  density: 'comfortable',
  elevation: 'raised',
  as: 'div',
  interactive: false,
  labelledBy: '',
})

const policy = computed(() => resolveSolidSurfacePolicy(props))
</script>
