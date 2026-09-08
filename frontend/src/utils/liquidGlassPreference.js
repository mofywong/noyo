export const LIQUID_GLASS_DENSITY_STORAGE_KEY = 'noyo_liquid_glass_density'
export const DEFAULT_LIQUID_GLASS_DENSITY = 50

export const normalizeLiquidGlassDensity = (value) => {
  if (value === null || value === undefined || value === '') {
    return DEFAULT_LIQUID_GLASS_DENSITY
  }

  const parsed = Number(value)
  if (!Number.isFinite(parsed)) {
    return DEFAULT_LIQUID_GLASS_DENSITY
  }

  return Math.min(100, Math.max(0, Math.round(parsed)))
}

export const readLiquidGlassDensity = (storage) => {
  try {
    return normalizeLiquidGlassDensity(
      storage?.getItem?.(LIQUID_GLASS_DENSITY_STORAGE_KEY),
    )
  } catch {
    return DEFAULT_LIQUID_GLASS_DENSITY
  }
}

export const writeLiquidGlassDensity = (storage, value) => {
  const normalized = normalizeLiquidGlassDensity(value)
  try {
    storage?.setItem?.(LIQUID_GLASS_DENSITY_STORAGE_KEY, String(normalized))
  } catch {
    // The in-memory preference remains usable when storage is unavailable.
  }
  return normalized
}
