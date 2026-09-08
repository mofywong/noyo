<template>
  <svg
    class="noyo-dashboard-liquid-glass-filters"
    data-dashboard-liquid-glass-filters
    aria-hidden="true"
    focusable="false"
    width="0"
    height="0"
  >
    <defs>
      <filter
        v-for="filter in filters"
        :id="filter.id"
        :key="filter.id"
        :x="filter.region.x"
        :y="filter.region.y"
        :width="filter.region.width"
        :height="filter.region.height"
        color-interpolation-filters="sRGB"
      >
        <feImage
          :href="filter.mapUrl"
          x="0"
          y="0"
          width="100%"
          height="100%"
          preserveAspectRatio="none"
          :result="`${filter.key}-map`"
        />

        <feDisplacementMap
          in="SourceGraphic"
          :in2="`${filter.key}-map`"
          :scale="filter.redScale"
          xChannelSelector="R"
          yChannelSelector="G"
          :result="`${filter.key}-warp-r`"
        />
        <feColorMatrix
          :in="`${filter.key}-warp-r`"
          type="matrix"
          :values="RED_CHANNEL_MATRIX"
          :result="`${filter.key}-channel-r`"
        />

        <feDisplacementMap
          in="SourceGraphic"
          :in2="`${filter.key}-map`"
          :scale="filter.greenScale"
          xChannelSelector="R"
          yChannelSelector="G"
          :result="`${filter.key}-warp-g`"
        />
        <feColorMatrix
          :in="`${filter.key}-warp-g`"
          type="matrix"
          :values="GREEN_CHANNEL_MATRIX"
          :result="`${filter.key}-channel-g`"
        />

        <feDisplacementMap
          in="SourceGraphic"
          :in2="`${filter.key}-map`"
          :scale="filter.blueScale"
          xChannelSelector="R"
          yChannelSelector="G"
          :result="`${filter.key}-warp-b`"
        />
        <feColorMatrix
          :in="`${filter.key}-warp-b`"
          type="matrix"
          :values="BLUE_CHANNEL_MATRIX"
          :result="`${filter.key}-channel-b`"
        />

        <feBlend
          :in="`${filter.key}-channel-r`"
          :in2="`${filter.key}-channel-g`"
          mode="screen"
          :result="`${filter.key}-channels-rg`"
        />
        <feBlend
          :in="`${filter.key}-channels-rg`"
          :in2="`${filter.key}-channel-b`"
          mode="screen"
          :result="`${filter.key}-channels-rgb`"
        />

        <feColorMatrix
          :in="`${filter.key}-map`"
          type="matrix"
          :values="SPECULAR_ALPHA_MATRIX"
          :result="`${filter.key}-specular`"
        />
        <feComposite
          :in="`${filter.key}-specular`"
          :in2="`${filter.key}-map`"
          operator="in"
          :result="`${filter.key}-specular-clipped`"
        />
        <feComposite
          :in="`${filter.key}-channels-rgb`"
          :in2="`${filter.key}-map`"
          operator="in"
          :result="`${filter.key}-inside`"
        />
        <feComposite
          in="SourceGraphic"
          :in2="`${filter.key}-map`"
          operator="out"
          :result="`${filter.key}-outside`"
        />
        <feMerge :result="`${filter.key}-shape`">
          <feMergeNode :in="`${filter.key}-outside`" />
          <feMergeNode :in="`${filter.key}-inside`" />
        </feMerge>
        <feBlend
          :in="`${filter.key}-shape`"
          :in2="`${filter.key}-specular-clipped`"
          mode="screen"
          :result="`${filter.key}-output`"
        />
      </filter>
    </defs>
  </svg>
</template>

<script setup>
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue';
import {
  computeRoundedRectDisplacementMap,
  computeSvgFilterRegion,
  encodeDisplacementMapDataUrl,
  supportsBackdropSvgFilter,
} from '../../utils/liquidGlassRefraction.js';

const emit = defineEmits(['ready']);

const RED_CHANNEL_MATRIX = '1 0 0 0 0  0 0 0 0 0  0 0 0 0 0  0 0 0 1 0';
const GREEN_CHANNEL_MATRIX = '0 0 0 0 0  0 1 0 0 0  0 0 0 0 0  0 0 0 1 0';
const BLUE_CHANNEL_MATRIX = '0 0 0 0 0  0 0 0 0 0  0 0 1 0 0  0 0 0 1 0';
const SPECULAR_ALPHA_MATRIX = '0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0.28 0 0';
const DISPERSION = 0.18;

const filters = ref([]);

const createFilterAsset = ({ key, id, blur, ...mapOptions }) => {
  const map = computeRoundedRectDisplacementMap(mapOptions);
  const scale = map.maxDisplacement;
  return {
    key,
    id,
    mapUrl: encodeDisplacementMapDataUrl(map, document),
    redScale: Number((scale * (1 + DISPERSION)).toFixed(2)),
    greenScale: scale,
    blueScale: Number((scale * (1 - DISPERSION)).toFixed(2)),
    region: computeSvgFilterRegion({
      width: map.width,
      height: map.height,
      strength: scale,
      blur,
    }),
  };
};

onMounted(async () => {
  if (!supportsBackdropSvgFilter(window.CSS, window, document, navigator)) {
    emit('ready', { supported: false });
    return;
  }

  try {
    filters.value = [
      createFilterAsset({
        key: 'kpi',
        id: 'noyo-dashboard-kpi-refraction',
        width: 192,
        height: 96,
        radius: 24,
        depth: 20,
        curvature: 0.42,
        bend: 0.78,
        bendWidth: 0.14,
        specularAngle: 315,
        strength: 16,
        blur: 0.5,
      }),
      createFilterAsset({
        key: 'panel',
        id: 'noyo-dashboard-panel-refraction',
        width: 144,
        height: 144,
        radius: 24,
        depth: 22,
        curvature: 0.42,
        bend: 0.78,
        bendWidth: 0.14,
        specularAngle: 315,
        strength: 22,
        blur: 0.75,
      }),
    ];
    await nextTick();
    emit('ready', { supported: true });
  } catch (error) {
    console.warn('Dashboard Liquid Glass refraction fallback:', error);
    filters.value = [];
    emit('ready', { supported: false });
  }
});

onBeforeUnmount(() => {
  emit('ready', { supported: false });
});
</script>

<style scoped>
.noyo-dashboard-liquid-glass-filters {
  position: absolute;
  overflow: hidden;
  pointer-events: none;
}
</style>
