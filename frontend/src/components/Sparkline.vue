<template>
  <div 
    class="sparkline-container" 
    :style="{ width: width + 'px', height: height + 'px' }"
    :title="tooltip || $t('sparkline_tooltip')"
  >
    <svg :width="width" :height="height" class="sparkline">
      <defs>
        <linearGradient :id="'gradient-' + id" x1="0" x2="0" y1="0" y2="1">
          <stop offset="0%" :stop-color="color" stop-opacity="0.3" />
          <stop offset="100%" :stop-color="color" stop-opacity="0.05" />
        </linearGradient>
      </defs>
      
      <!-- Axis Line with Arrow -->
      <path
        :d="axisPath"
        stroke="#ccc"
        stroke-width="1"
        fill="none"
      />
      
      <path
        :d="fillPath"
        :fill="'url(#gradient-' + id + ')'"
        stroke="none"
      />
      <path
        :d="linePath"
        :stroke="color"
        stroke-width="1.5"
        fill="none"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <!-- Last point dot -->
      <circle
        v-if="points.length > 0"
        :cx="points[points.length - 1].x"
        :cy="points[points.length - 1].y"
        r="2"
        :fill="color"
      />
    </svg>
    <div v-if="loading" class="loading-overlay">
      <div class="spinner-border spinner-border-sm text-secondary" role="status"></div>
    </div>
    <div v-if="points.length === 0 && !loading" class="no-data text-muted small">
      -
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';

const { t } = useI18n();

const props = defineProps({
  data: {
    type: Array,
    default: () => []
  },
  width: {
    type: Number,
    default: 120
  },
  height: {
    type: Number,
    default: 30
  },
  color: {
    type: String,
    default: '#0d6efd'
  },
  loading: {
    type: Boolean,
    default: false
  },
  tooltip: {
    type: String,
    default: ''
  }
});

const id = Math.random().toString(36).substr(2, 9);

const axisPath = computed(() => {
  const w = props.width;
  const h = props.height;
  // Line at bottom, arrow at right
  // M 0 (h-1) L w (h-1)
  // Arrow head: L (w-4) (h-4) M w (h-1) L (w-4) (h+2) -> simplified
  return `M 0 ${h-1} L ${w} ${h-1} M ${w-4} ${h-4} L ${w} ${h-1} L ${w-4} ${h+2}`;
});

const points = computed(() => {
  if (!props.data || props.data.length === 0) return [];
  
  const values = props.data.map(v => Number(v));
  // Filter out non-numeric values (null, undefined, NaN) but keep index if needed? 
  // For sparkline, we usually just want valid points.
  // But if we want to show gaps, it's harder with simple SVG path. 
  // Let's filter valid numbers for now.
  const validValues = values.filter(v => !isNaN(v) && v !== null);
  
  if (validValues.length === 0) return [];

  const min = Math.min(...validValues);
  const max = Math.max(...validValues);
  let range = max - min;
  
  // Fix: If fluctuation is tiny (floating point error or very small change), 
  // prevent it from being exaggerated to full height.
  // We ensure the range is at least a small fraction of the value (e.g. 10%) or a small absolute value.
  const absMax = Math.max(Math.abs(max), Math.abs(min));
  const minRange = absMax === 0 ? 1 : absMax * 0.1; // Ensure at least 10% buffering for stability
  
  let usedMin = min;
  let usedRange = range;

  if (range < 1e-9) {
      // Treat as constant
      usedRange = 0;
  } else if (range < minRange) {
      // Expand range to minRange to suppress noise
      const mid = (min + max) / 2;
      usedRange = minRange;
      usedMin = mid - minRange / 2;
  }
  
  // Add some padding to top/bottom
  const padding = 2;
  const h = props.height - padding * 2;
  const w = props.width;
  
  // If constant value, draw straight line in middle
  if (usedRange === 0) {
    return validValues.map((val, i) => ({
      x: (i / (validValues.length - 1 || 1)) * w,
      y: props.height / 2
    }));
  }

  return validValues.map((val, i) => ({
    x: (i / (validValues.length - 1 || 1)) * w,
    y: props.height - padding - ((val - usedMin) / usedRange) * h
  }));
});

const linePath = computed(() => {
  if (points.value.length === 0) return '';
  return points.value.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ');
});

const fillPath = computed(() => {
  if (points.value.length === 0) return '';
  const first = points.value[0];
  const last = points.value[points.value.length - 1];
  return `${linePath.value} L ${last.x} ${props.height} L ${first.x} ${props.height} Z`;
});
</script>

<style scoped>
.sparkline-container {
  position: relative;
  display: inline-block;
  vertical-align: middle;
}
.loading-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(255,255,255,0.7);
  display: flex;
  align-items: center;
  justify-content: center;
}
.no-data {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}
</style>
