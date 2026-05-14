const ROI_SCALE = 10000;

export function clampRoiPoint(point) {
  return {
    x: clamp(Math.round(Number(point?.x ?? 0)), 0, ROI_SCALE),
    y: clamp(Math.round(Number(point?.y ?? 0)), 0, ROI_SCALE),
  };
}

export function normalizeCanvasPoint(point, size) {
  const width = Number(size?.width ?? 1) || 1;
  const height = Number(size?.height ?? 1) || 1;
  return clampRoiPoint({
    x: (Number(point?.x ?? 0) / width) * ROI_SCALE,
    y: (Number(point?.y ?? 0) / height) * ROI_SCALE,
  });
}

export function denormalizeRoiPoint(point, size) {
  const width = Number(size?.width ?? 1) || 1;
  const height = Number(size?.height ?? 1) || 1;
  const normalized = clampRoiPoint(point);
  return {
    x: Math.round((normalized.x / ROI_SCALE) * width),
    y: Math.round((normalized.y / ROI_SCALE) * height),
  };
}

export function nearestPointIndex(points, canvasPoint, size, radius = 12) {
  let bestIndex = -1;
  let bestDistance = radius;
  points.forEach((point, index) => {
    const px = denormalizeRoiPoint(point, size);
    const distance = Math.hypot(px.x - canvasPoint.x, px.y - canvasPoint.y);
    if (distance <= bestDistance) {
      bestDistance = distance;
      bestIndex = index;
    }
  });
  return bestIndex;
}

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}
