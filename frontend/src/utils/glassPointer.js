function clampPercentage(value) {
  return Math.min(100, Math.max(0, value))
}

function clampUnit(value) {
  return Math.min(1, Math.max(0, value))
}

export function normalizeGlassPointer(point, rect) {
  if (!rect?.width || !rect?.height) {
    return { x: 50, y: 50 }
  }

  return {
    x: clampPercentage(((point.clientX - rect.left) / rect.width) * 100),
    y: clampPercentage(((point.clientY - rect.top) / rect.height) * 100),
  }
}

export function resolveGlassPointerState(point, rect) {
  const normalized = normalizeGlassPointer(point, rect)
  const directionX = (normalized.x - 50) / 50
  const directionY = (normalized.y - 50) / 50
  const energy = Math.min(
    1,
    Math.hypot(directionX, directionY) / Math.SQRT2,
  )
  const edgeEnergy = clampUnit(
    Math.max(
      Math.abs(normalized.x - 50),
      Math.abs(normalized.y - 50),
    ) / 50,
  )

  return {
    ...normalized,
    directionX,
    directionY,
    energy,
    edgeEnergy,
  }
}
