<template>
  <SolidSurface
    v-if="policy.kind === 'solid'"
    v-bind="attrs"
    :density="density"
    :elevation="elevation"
    :interactive="interactive"
  >
    <slot />
  </SolidSurface>
  <GlassGroup
    v-else
    v-bind="attrs"
    :profile="policy.profile"
    :shape="policy.shape"
    :interactive="interactive"
  >
    <slot />
  </GlassGroup>
</template>

<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import GlassGroup from './GlassGroup.vue'
import SolidSurface from './SolidSurface.vue'
import { resolveLiquidGlassPolicy } from '../../utils/liquidGlassPolicy.js'

defineOptions({ inheritAttrs: false })

interface LiquidGlassProps {
  profile?: 'chrome' | 'content' | 'floating' | 'solid' | 'regular' | 'clear-media'
  shape?: 'rounded' | 'pill' | 'circle' | 'panel'
  interactive?: boolean
  opaque?: boolean
  density?: 'comfortable' | 'compact'
  elevation?: 'flat' | 'raised'
}

const props = withDefaults(defineProps<LiquidGlassProps>(), {
  profile: 'chrome',
  shape: 'rounded',
  interactive: false,
  opaque: false,
  density: 'comfortable',
  elevation: 'raised',
})

const attrs = useAttrs()
const policy = computed(() => resolveLiquidGlassPolicy(props))
</script>
