const NEUTRAL_CHANNEL = 128
const CHANNEL_AMPLITUDE = 127
const DEFAULT_STRENGTH = 8
const DEFAULT_DEPTH_RATIO = 0.22
const DEFAULT_CURVATURE = 0.42
const DEFAULT_BEND = 0.78
const DEFAULT_BEND_WIDTH = 0.14
const DEFAULT_SPECULAR_ANGLE = 315
const DEFAULT_DISPERSION = 0.18
const DEFAULT_DEFORMATION = 0.014

function assertFinitePositiveInteger(value, name) {
  if (!Number.isSafeInteger(value) || value <= 0) {
    throw new TypeError(`${name} must be a positive safe integer`)
  }
}

function assertFinitePositiveNumber(value, name) {
  if (!Number.isFinite(value) || value <= 0) {
    throw new TypeError(`${name} must be a finite positive number`)
  }
}

function assertFiniteNonNegativeNumber(value, name) {
  if (!Number.isFinite(value) || value < 0) {
    throw new TypeError(`${name} must be a finite non-negative number`)
  }
}

function clampUnit(value) {
  return Math.max(0, Math.min(1, value))
}

function smoothstep(value) {
  const unit = clampUnit(value)
  return unit * unit * (3 - 2 * unit)
}

function normalizeUnitOption(value, fallback, name) {
  const resolved = value ?? fallback
  if (!Number.isFinite(resolved) || resolved < 0 || resolved > 1) {
    throw new TypeError(`${name} must be between zero and one`)
  }
  return resolved
}

function roundedRectSignedDistance(x, y, width, height, radius) {
  const halfWidth = width / 2
  const halfHeight = height / 2
  const qx = Math.abs(x - halfWidth) - (halfWidth - radius)
  const qy = Math.abs(y - halfHeight) - (halfHeight - radius)
  const outsideX = Math.max(qx, 0)
  const outsideY = Math.max(qy, 0)
  return Math.hypot(outsideX, outsideY) + Math.min(Math.max(qx, qy), 0) - radius
}

function clampChannel(value) {
  return Math.max(0, Math.min(255, Math.round(value)))
}

function validateMapOptions(options) {
  if (!options || typeof options !== 'object' || Array.isArray(options)) {
    throw new TypeError('options must be an object')
  }

  const {
    width,
    height,
    radius,
    bezel,
    depth,
    curvature,
    bend,
    bendWidth,
    specularAngle = DEFAULT_SPECULAR_ANGLE,
    strength = DEFAULT_STRENGTH,
  } = options
  assertFinitePositiveInteger(width, 'width')
  assertFinitePositiveInteger(height, 'height')
  const shortSide = Math.min(width, height)
  if (!Number.isFinite(radius) || radius < 0 || radius > shortSide / 2) {
    throw new TypeError('radius must be between zero and half the shortest side')
  }
  const resolvedDepth = depth ?? bezel ?? shortSide * DEFAULT_DEPTH_RATIO
  assertFinitePositiveNumber(resolvedDepth, 'depth')
  if (resolvedDepth > shortSide / 2) {
    throw new TypeError('depth must not exceed half the shortest side')
  }
  const resolvedCurvature = normalizeUnitOption(curvature, DEFAULT_CURVATURE, 'curvature')
  const resolvedBend = normalizeUnitOption(bend, DEFAULT_BEND, 'bend')
  const resolvedBendWidth = bendWidth ?? DEFAULT_BEND_WIDTH
  if (!Number.isFinite(resolvedBendWidth) || resolvedBendWidth <= 0 || resolvedBendWidth > 0.5) {
    throw new TypeError('bendWidth must be greater than zero and at most 0.5')
  }
  if (!Number.isFinite(specularAngle)) {
    throw new TypeError('specularAngle must be finite')
  }
  assertFinitePositiveNumber(strength, 'strength')
  if (strength > shortSide / 2) {
    throw new TypeError('strength must not exceed half the shortest side')
  }

  const pixelCount = width * height
  if (!Number.isSafeInteger(pixelCount) || pixelCount > Number.MAX_SAFE_INTEGER / 4) {
    throw new TypeError('width and height produce too many pixels')
  }

  return {
    width,
    height,
    radius,
    depth: resolvedDepth,
    curvature: resolvedCurvature,
    bend: resolvedBend,
    bendWidth: resolvedBendWidth,
    specularAngle: ((specularAngle % 360) + 360) % 360,
    strength,
    pixelCount,
  }
}

function roundedRectNormal(sampleX, sampleY, width, height, radius) {
  const sampleDistanceX = roundedRectSignedDistance(
    sampleX + 1,
    sampleY,
    width,
    height,
    radius,
  ) - roundedRectSignedDistance(
    sampleX - 1,
    sampleY,
    width,
    height,
    radius,
  )
  const sampleDistanceY = roundedRectSignedDistance(
    sampleX,
    sampleY + 1,
    width,
    height,
    radius,
  ) - roundedRectSignedDistance(
    sampleX,
    sampleY - 1,
    width,
    height,
    radius,
  )
  const length = Math.hypot(sampleDistanceX, sampleDistanceY)
  if (length === 0) return { x: 0, y: 0 }
  return { x: sampleDistanceX / length, y: sampleDistanceY / length }
}

export function computeSvgFilterRegion(options) {
  if (!options || typeof options !== 'object' || Array.isArray(options)) {
    throw new TypeError('options must be an object')
  }
  const {
    width,
    height,
    strength,
    blur,
    dispersion = DEFAULT_DISPERSION,
    deformation = DEFAULT_DEFORMATION,
  } = options
  assertFinitePositiveInteger(width, 'width')
  assertFinitePositiveInteger(height, 'height')
  assertFinitePositiveNumber(strength, 'strength')
  assertFiniteNonNegativeNumber(blur, 'blur')
  assertFiniteNonNegativeNumber(dispersion, 'dispersion')
  assertFiniteNonNegativeNumber(deformation, 'deformation')

  const displaced = strength * (1 + dispersion)
  const padX = Math.ceil(displaced + blur + width * deformation) + 2
  const padY = Math.ceil(displaced + blur + height * deformation) + 2
  const padXPercent = Math.ceil((padX / width) * 100)
  const padYPercent = Math.ceil((padY / height) * 100)

  return {
    x: `-${padXPercent}%`,
    y: `-${padYPercent}%`,
    width: `${100 + padXPercent * 2}%`,
    height: `${100 + padYPercent * 2}%`,
    padXPercent,
    padYPercent,
  }
}

/**
 * Create an RGBA displacement map for a rounded rectangle.
 *
 * Red and green encode the rounded edge, liquid lip, and convex dome around
 * a neutral value of 128. Blue carries a directional specular mask. Pixels
 * outside the rounded shape stay transparent.
 */
export function computeRoundedRectDisplacementMap(options) {
  const {
    width,
    height,
    radius,
    depth,
    curvature,
    bend,
    bendWidth,
    specularAngle,
    strength,
    pixelCount,
  } = validateMapOptions(options)
  const pixels = new Uint8ClampedArray(pixelCount * 4)
  const halfWidth = width / 2
  const halfHeight = height / 2
  const lipWidth = Math.min(width, height) * bendWidth
  const lightRadians = specularAngle * Math.PI / 180
  const lightX = Math.cos(lightRadians)
  const lightY = Math.sin(lightRadians)

  for (let index = 0; index < pixelCount; index += 1) {
    const x = index % width
    const y = Math.floor(index / width)
    const offset = index * 4
    const sampleX = x + 0.5
    const sampleY = y + 0.5
    const distance = roundedRectSignedDistance(
      sampleX,
      sampleY,
      width,
      height,
      radius,
    )

    pixels[offset] = NEUTRAL_CHANNEL
    pixels[offset + 1] = NEUTRAL_CHANNEL
    pixels[offset + 2] = 0
    pixels[offset + 3] = 0

    if (distance > 0) continue

    pixels[offset + 3] = 255
    const insideDepth = Math.max(0, -distance)
    const depthProgress = clampUnit(insideDepth / depth)
    const edgeWeight = 1 - smoothstep(depthProgress)
    const lipProgress = clampUnit(insideDepth / lipWidth)
    const lipWeight = bend * Math.sin(Math.PI * lipProgress)
    const normal = roundedRectNormal(sampleX, sampleY, width, height, radius)

    const centeredX = (sampleX - halfWidth) / halfWidth
    const centeredY = (sampleY - halfHeight) / halfHeight
    const radialLength = Math.hypot(centeredX, centeredY)
    const radialX = radialLength === 0 ? 0 : centeredX / radialLength
    const radialY = radialLength === 0 ? 0 : centeredY / radialLength
    const domeProfile = 1 - Math.min(1, radialLength) ** 2
    const domeWeight = curvature * smoothstep(depthProgress) * domeProfile

    let vectorX = normal.x * (edgeWeight + lipWeight) + radialX * domeWeight
    let vectorY = normal.y * (edgeWeight + lipWeight) + radialY * domeWeight
    const vectorLength = Math.hypot(vectorX, vectorY)
    if (vectorLength > 1) {
      vectorX /= vectorLength
      vectorY /= vectorLength
    }
    pixels[offset] = clampChannel(
      NEUTRAL_CHANNEL + vectorX * CHANNEL_AMPLITUDE,
    )
    pixels[offset + 1] = clampChannel(
      NEUTRAL_CHANNEL + vectorY * CHANNEL_AMPLITUDE,
    )
    const specularAlignment = Math.max(0, normal.x * lightX + normal.y * lightY)
    const specularWeight = clampUnit(edgeWeight + lipWeight)
    pixels[offset + 2] = clampChannel(
      255 * specularWeight * (0.08 + specularAlignment * 0.92),
    )
  }

  // The declared strength is the maximum scale that the SVG displacement
  // filter should use, even when a pixel-center sample misses the exact edge.
  return { width, height, pixels, maxDisplacement: strength }
}

function validateMapForEncoding(map) {
  if (!map || typeof map !== 'object') throw new TypeError('map must be an object')
  const { width, height, pixels } = map
  assertFinitePositiveInteger(width, 'map.width')
  assertFinitePositiveInteger(height, 'map.height')
  if (!ArrayBuffer.isView(pixels) || pixels.length !== width * height * 4) {
    throw new TypeError('map.pixels must contain RGBA data for every map pixel')
  }
}

/** Encode an RGBA displacement map as a PNG data URL using a supplied document. */
export function encodeDisplacementMapDataUrl(map, documentRef = globalThis.document) {
  validateMapForEncoding(map)
  if (!documentRef || typeof documentRef.createElement !== 'function') {
    throw new TypeError('documentRef must provide createElement')
  }

  const canvas = documentRef.createElement('canvas')
  if (!canvas || typeof canvas.getContext !== 'function' || typeof canvas.toDataURL !== 'function') {
    throw new TypeError('documentRef must create a canvas with 2d encoding support')
  }
  canvas.width = map.width
  canvas.height = map.height

  const context = canvas.getContext('2d')
  if (!context || typeof context.createImageData !== 'function' || typeof context.putImageData !== 'function') {
    throw new TypeError('canvas must provide a 2d image context')
  }
  const imageData = context.createImageData(map.width, map.height)
  if (!imageData || !imageData.data || imageData.data.length !== map.pixels.length) {
    throw new TypeError('canvas image data has an unexpected size')
  }
  imageData.data.set(map.pixels)
  context.putImageData(imageData, 0, 0)

  const dataUrl = canvas.toDataURL('image/png')
  if (typeof dataUrl !== 'string' || !dataUrl.startsWith('data:image/png')) {
    throw new TypeError('canvas did not produce a PNG data URL')
  }
  return dataUrl
}

function chromiumMajorVersion(navigatorRef) {
  if (!navigatorRef) return null
  try {
    const brands = navigatorRef.userAgentData?.brands
    if (Array.isArray(brands)) {
      const match = brands.find(({ brand }) => /Chromium|Chrome|Edge/i.test(brand))
      const version = Number.parseInt(match?.version || '', 10)
      if (Number.isFinite(version)) return version
    }
    const userAgent = String(navigatorRef.userAgent || '')
    const match = userAgent.match(/(?:Edg|HeadlessChrome|Chrome)\/(\d+)/i)
    const version = Number.parseInt(match?.[1] || '', 10)
    return Number.isFinite(version) ? version : null
  } catch {
    return null
  }
}

/** Detect the validated Chromium backdrop-sampling plus SVG filter path. */
export function supportsBackdropSvgFilter(
  cssRef = globalThis.CSS,
  windowRef = globalThis.window,
  documentRef = globalThis.document,
  navigatorRef = globalThis.navigator,
) {
  if (!cssRef || !windowRef || !documentRef?.body) return false
  const majorVersion = chromiumMajorVersion(navigatorRef)
  if (majorVersion === null || majorVersion < 130) return false

  let supports
  try {
    supports = cssRef.supports
  } catch {
    return false
  }
  if (typeof supports !== 'function') return false

  const filterValue = 'url(#noyo-dashboard-refraction-probe)'
  const backdropValue = 'blur(0.5px)'
  const supportsValue = (properties, value) => properties.some((property) => {
    try {
      if (supports.call(cssRef, property, value)) return true
    } catch {
      // Try the single-condition form below.
    }
    try {
      return supports.call(cssRef, `${property}: ${value}`)
    } catch {
      return false
    }
  })
  if (
    !supportsValue(['filter', '-webkit-filter'], filterValue) ||
    !supportsValue(['backdrop-filter', '-webkit-backdrop-filter'], backdropValue) ||
    typeof documentRef.createElement !== 'function'
  ) return false

  let probe
  let verified = false
  try {
    probe = documentRef.createElement('div')
    if (!probe?.style || typeof windowRef.getComputedStyle !== 'function') {
      throw new TypeError('SVG filter probe is unavailable')
    }
    probe.style.position = 'fixed'
    probe.style.width = '1px'
    probe.style.height = '1px'
    probe.style.pointerEvents = 'none'
    probe.style.filter = filterValue
    probe.style.webkitFilter = filterValue
    probe.style.backdropFilter = backdropValue
    probe.style.webkitBackdropFilter = backdropValue
    documentRef.body.appendChild(probe)
    const computed = windowRef.getComputedStyle(probe)
    const resolvedFilter = [
      computed?.filter,
      computed?.webkitFilter,
      computed?.getPropertyValue?.('filter'),
      computed?.getPropertyValue?.('-webkit-filter'),
    ].filter(Boolean).join(' ')
    const resolvedBackdrop = [
      computed?.backdropFilter,
      computed?.webkitBackdropFilter,
      computed?.getPropertyValue?.('backdrop-filter'),
      computed?.getPropertyValue?.('-webkit-backdrop-filter'),
    ].filter(Boolean).join(' ')
    verified = (
      resolvedFilter.includes('#noyo-dashboard-refraction-probe') &&
      /blur\(0\.5px\)/.test(resolvedBackdrop)
    )
  } catch {
    verified = false
  }
  if (typeof probe?.remove !== 'function') return false
  try {
    probe.remove()
  } catch {
    return false
  }
  return verified
}
