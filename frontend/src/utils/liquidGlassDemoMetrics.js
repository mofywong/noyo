export const LIQUID_GLASS_TARGET_FPS = 55

const SOFTWARE_GPU_RENDERER_PATTERN = /(?:swiftshader|software|llvmpipe|warp|microsoft basic render driver|lavapipe|softpipe|gdi generic|mesa offscreen)/i
const HARDWARE_GPU_RENDERER_PATTERN = /(?:nvidia|geforce|quadro|amd|radeon|intel|\barc\b|apple|adreno|mali|powervr|vivante|qualcomm|imagination)/i

export function resolveHardwareGpuRendererPolicy({
  debugInfoAvailable = false,
  vendor = '',
  renderer = '',
} = {}) {
  const normalizedVendor = String(vendor || '').trim()
  const normalizedRenderer = String(renderer || '').trim()
  const identity = `${normalizedVendor} ${normalizedRenderer}`.trim()

  if (!debugInfoAvailable || !normalizedVendor || !normalizedRenderer) {
    return {
      hardware: false,
      reason: 'missing-debug-renderer-info',
      vendor: normalizedVendor,
      renderer: normalizedRenderer,
    }
  }
  if (SOFTWARE_GPU_RENDERER_PATTERN.test(identity)) {
    return {
      hardware: false,
      reason: 'software-renderer',
      vendor: normalizedVendor,
      renderer: normalizedRenderer,
    }
  }
  if (!HARDWARE_GPU_RENDERER_PATTERN.test(identity)) {
    return {
      hardware: false,
      reason: 'unrecognized-hardware-renderer',
      vendor: normalizedVendor,
      renderer: normalizedRenderer,
    }
  }

  return {
    hardware: true,
    reason: 'hardware-renderer',
    vendor: normalizedVendor,
    renderer: normalizedRenderer,
  }
}

export function meetsLiquidGlassGpuPerformanceGate({ fps, p95 } = {}) {
  return Number(fps) >= LIQUID_GLASS_TARGET_FPS && Number(p95) < 20
}

function clampChannel(value) {
  return Math.min(255, Math.max(0, Number(value)))
}

function clampAlpha(value) {
  return Math.min(1, Math.max(0, Number(value)))
}

export function parseCssColor(value) {
  if (typeof value !== 'string') return null

  const normalized = value.trim().toLowerCase()
  const shortHex = /^#([0-9a-f]{3})$/i.exec(normalized)
  if (shortHex) {
    const [r, g, b] = shortHex[1].split('').map((channel) => (
      Number.parseInt(channel + channel, 16)
    ))
    return { r, g, b, a: 1 }
  }

  const longHex = /^#([0-9a-f]{6})$/i.exec(normalized)
  if (longHex) {
    return {
      r: Number.parseInt(longHex[1].slice(0, 2), 16),
      g: Number.parseInt(longHex[1].slice(2, 4), 16),
      b: Number.parseInt(longHex[1].slice(4, 6), 16),
      a: 1,
    }
  }

  const functional = /^rgba?\(([^)]+)\)$/i.exec(normalized)
  if (!functional) return null

  const channels = functional[1]
    .split(',')
    .map((channel) => channel.trim())
  if (channels.length < 3 || channels.length > 4) return null

  const [r, g, b] = channels.slice(0, 3).map(clampChannel)
  const a = channels.length === 4 ? clampAlpha(channels[3]) : 1
  if ([r, g, b, a].some((channel) => Number.isNaN(channel))) return null

  return { r, g, b, a }
}

function composite(foreground, background) {
  const alpha = foreground.a + background.a * (1 - foreground.a)
  if (alpha === 0) return { r: 0, g: 0, b: 0, a: 0 }

  return {
    r: (
      foreground.r * foreground.a +
      background.r * background.a * (1 - foreground.a)
    ) / alpha,
    g: (
      foreground.g * foreground.a +
      background.g * background.a * (1 - foreground.a)
    ) / alpha,
    b: (
      foreground.b * foreground.a +
      background.b * background.a * (1 - foreground.a)
    ) / alpha,
    a: alpha,
  }
}

function relativeLuminance(color) {
  const linear = [color.r, color.g, color.b].map((channel) => {
    const normalized = channel / 255
    return normalized <= 0.04045
      ? normalized / 12.92
      : ((normalized + 0.055) / 1.055) ** 2.4
  })

  return linear[0] * 0.2126 + linear[1] * 0.7152 + linear[2] * 0.0722
}

export function contrastRatio(foregroundValue, backgroundValue) {
  const foreground = typeof foregroundValue === 'string'
    ? parseCssColor(foregroundValue)
    : foregroundValue
  const background = typeof backgroundValue === 'string'
    ? parseCssColor(backgroundValue)
    : backgroundValue
  if (!foreground || !background) return 0

  const opaqueBackground = background.a < 1
    ? composite(background, { r: 255, g: 255, b: 255, a: 1 })
    : background
  const opaqueForeground = foreground.a < 1
    ? composite(foreground, opaqueBackground)
    : foreground
  const foregroundLuminance = relativeLuminance(opaqueForeground)
  const backgroundLuminance = relativeLuminance(opaqueBackground)
  const lighter = Math.max(foregroundLuminance, backgroundLuminance)
  const darker = Math.min(foregroundLuminance, backgroundLuminance)

  return (lighter + 0.05) / (darker + 0.05)
}

export function contrastRatioOverLayers(
  foregroundValue,
  layerValues = [],
  fallbackValue = '#ffffff',
) {
  const opaqueWhite = { r: 255, g: 255, b: 255, a: 1 }
  const parsedFallback = typeof fallbackValue === 'string'
    ? parseCssColor(fallbackValue)
    : fallbackValue
  if (!parsedFallback) return 0

  let effectiveBackground = parsedFallback.a < 1
    ? composite(parsedFallback, opaqueWhite)
    : parsedFallback

  for (const layerValue of [...layerValues].reverse()) {
    const layer = typeof layerValue === 'string'
      ? parseCssColor(layerValue)
      : layerValue
    if (!layer) continue
    effectiveBackground = composite(layer, effectiveBackground)
  }

  return contrastRatio(foregroundValue, effectiveBackground)
}

export function summarizeFrameTimes(frameTimes) {
  const valid = frameTimes.filter((value) => Number.isFinite(value) && value > 0)
  if (valid.length === 0) {
    return { averageFps: 0, p95FrameTime: 0 }
  }

  const averageFrameTime = valid.reduce((sum, value) => sum + value, 0) / valid.length
  const sorted = [...valid].sort((left, right) => left - right)
  const p95Index = Math.max(0, Math.ceil(sorted.length * 0.95) - 1)

  return {
    averageFps: Math.round(1000 / averageFrameTime),
    p95FrameTime: Math.round(sorted[p95Index]),
  }
}
