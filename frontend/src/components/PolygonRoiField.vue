<template>
  <div class="polygon-roi-field">
    <div class="polygon-toolbar">
      <div class="polygon-count">
        <span class="badge text-bg-secondary">{{ normalizedPoints.length }}</span>
        <span>points</span>
      </div>
      <div class="btn-group btn-group-sm">
        <button type="button" class="btn btn-outline-secondary" @click="undoPoint" :disabled="normalizedPoints.length === 0" title="Undo">
          <i class="bi bi-arrow-counterclockwise"></i>
        </button>
        <button type="button" class="btn btn-outline-danger" @click="clearPoints" :disabled="normalizedPoints.length === 0" title="Clear">
          <i class="bi bi-trash"></i>
        </button>
      </div>
    </div>

    <canvas
      ref="canvasRef"
      class="polygon-canvas"
      width="640"
      height="360"
      @pointerdown="handlePointerDown"
      @pointermove="handlePointerMove"
      @pointerup="handlePointerUp"
      @pointerleave="handlePointerUp"
      @contextmenu.prevent="completePolygon"
    ></canvas>

    <div v-if="normalizedPoints.length > 0" class="polygon-points">
      <div v-for="(point, index) in normalizedPoints" :key="index" class="polygon-point-row">
        <span class="polygon-point-index">{{ index + 1 }}</span>
        <input class="form-control form-control-sm" type="number" min="0" max="10000" :value="point.x" @input="updatePoint(index, 'x', $event.target.value)">
        <input class="form-control form-control-sm" type="number" min="0" max="10000" :value="point.y" @input="updatePoint(index, 'y', $event.target.value)">
        <button type="button" class="btn btn-sm btn-outline-danger" @click="removePoint(index)" title="Remove point">
          <i class="bi bi-x-lg"></i>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue';
import {
  clampRoiPoint,
  denormalizeRoiPoint,
  nearestPointIndex,
  normalizeCanvasPoint,
} from '@/utils/aiSceneRules';

const props = defineProps({
  modelValue: {
    type: Array,
    default: () => [],
  },
});

const emit = defineEmits(['update:modelValue']);

const canvasRef = ref(null);
const draggingIndex = ref(-1);
const isDrawing = ref(false);
const previewPoint = ref(null);
const canvasSize = { width: 640, height: 360 };

const normalizedPoints = computed(() => {
  return (props.modelValue || []).map(clampRoiPoint);
});

const emitPoints = (points) => {
  emit('update:modelValue', points.map(clampRoiPoint));
};

const canvasPointFromEvent = (event) => {
  const canvas = canvasRef.value;
  const rect = canvas.getBoundingClientRect();
  return {
    x: ((event.clientX - rect.left) / rect.width) * canvasSize.width,
    y: ((event.clientY - rect.top) / rect.height) * canvasSize.height,
  };
};

const handlePointerDown = (event) => {
  if (event.button === 2) {
    completePolygon();
    return;
  }
  const point = canvasPointFromEvent(event);
  const existingIndex = nearestPointIndex(normalizedPoints.value, point, canvasSize);
  if (existingIndex >= 0) {
    draggingIndex.value = existingIndex;
    return;
  }

  const next = [...normalizedPoints.value, normalizeCanvasPoint(point, canvasSize)];
  emitPoints(next);
  draggingIndex.value = next.length - 1;
  isDrawing.value = true;
  previewPoint.value = point;
};

const handlePointerMove = (event) => {
  previewPoint.value = canvasPointFromEvent(event);
  if (draggingIndex.value < 0) {
    nextTick(draw);
    return;
  }
  const next = [...normalizedPoints.value];
  next[draggingIndex.value] = normalizeCanvasPoint(previewPoint.value, canvasSize);
  emitPoints(next);
};

const handlePointerUp = () => {
  draggingIndex.value = -1;
};

const completePolygon = () => {
  draggingIndex.value = -1;
  isDrawing.value = false;
  previewPoint.value = null;
};

const updatePoint = (index, axis, value) => {
  const next = [...normalizedPoints.value];
  next[index] = clampRoiPoint({ ...next[index], [axis]: Number(value) });
  emitPoints(next);
};

const removePoint = (index) => {
  emitPoints(normalizedPoints.value.filter((_, idx) => idx !== index));
};

const undoPoint = () => {
  emitPoints(normalizedPoints.value.slice(0, -1));
};

const clearPoints = () => {
  completePolygon();
  emitPoints([]);
};

const buildPreviewPolygon = () => {
  const points = normalizedPoints.value.map(point => denormalizeRoiPoint(point, canvasSize));
  if (isDrawing.value && previewPoint.value && draggingIndex.value < 0) {
    return [...points, previewPoint.value];
  }
  return points;
};

const draw = () => {
  const canvas = canvasRef.value;
  if (!canvas) return;
  const ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  ctx.fillStyle = '#f8fafc';
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  ctx.strokeStyle = '#d1d5db';
  ctx.lineWidth = 1;
  for (let x = 0; x <= canvas.width; x += 80) {
    ctx.beginPath();
    ctx.moveTo(x, 0);
    ctx.lineTo(x, canvas.height);
    ctx.stroke();
  }
  for (let y = 0; y <= canvas.height; y += 45) {
    ctx.beginPath();
    ctx.moveTo(0, y);
    ctx.lineTo(canvas.width, y);
    ctx.stroke();
  }

  const points = buildPreviewPolygon();
  if (points.length > 0) {
    ctx.beginPath();
    ctx.moveTo(points[0].x, points[0].y);
    points.slice(1).forEach(point => ctx.lineTo(point.x, point.y));
    if (points.length >= 2) ctx.closePath();
    ctx.fillStyle = 'rgba(37, 99, 235, 0.16)';
    ctx.strokeStyle = '#2563eb';
    ctx.lineWidth = 2;
    ctx.fill();
    ctx.stroke();
  }

  normalizedPoints.value.map(point => denormalizeRoiPoint(point, canvasSize)).forEach((point, index) => {
    ctx.beginPath();
    ctx.arc(point.x, point.y, 4, 0, Math.PI * 2);
    ctx.fillStyle = index === draggingIndex.value ? '#f59e0b' : '#2563eb';
    ctx.fill();
    ctx.lineWidth = 2;
    ctx.strokeStyle = '#ffffff';
    ctx.stroke();
  });
};

watch(normalizedPoints, () => nextTick(draw), { deep: true });
watch(draggingIndex, () => nextTick(draw));
watch(previewPoint, () => nextTick(draw), { deep: true });

onMounted(draw);
</script>

<style scoped>
.polygon-roi-field {
  display: grid;
  gap: 8px;
}

.polygon-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.polygon-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #6b7280;
  font-size: 0.875rem;
}

.polygon-canvas {
  width: 100%;
  aspect-ratio: 16 / 9;
  border: 1px solid #d1d5db;
  border-radius: 6px;
  cursor: crosshair;
  touch-action: none;
}

.polygon-points {
  display: grid;
  gap: 6px;
  max-height: 180px;
  overflow: auto;
}

.polygon-point-row {
  display: grid;
  grid-template-columns: 28px 1fr 1fr 34px;
  align-items: center;
  gap: 6px;
}

.polygon-point-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 999px;
  background: #e5e7eb;
  color: #374151;
  font-size: 0.75rem;
  font-weight: 600;
}
</style>
