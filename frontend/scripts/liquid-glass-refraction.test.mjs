import assert from 'node:assert/strict'
import {
  computeSvgFilterRegion,
  computeRoundedRectDisplacementMap,
  encodeDisplacementMapDataUrl,
  supportsBackdropSvgFilter,
} from '../src/utils/liquidGlassRefraction.js'

const options = {
  width: 64,
  height: 32,
  radius: 8,
  bezel: 8,
  strength: 12,
}

const pixelOffset = (x, y) => (y * options.width + x) * 4
const pixelOffsetFor = (x, y, width) => (y * width + x) * 4
const map = computeRoundedRectDisplacementMap(options)

assert.equal(map.width, options.width)
assert.equal(map.height, options.height)
assert.ok(map.pixels instanceof Uint8ClampedArray)
assert.equal(map.pixels.length, options.width * options.height * 4)
assert.equal(typeof map.maxDisplacement, 'number')
assert.ok(map.maxDisplacement > 0)

const center = pixelOffset(32, 16)
assert.notDeepEqual(
  Array.from(map.pixels.slice(center, center + 2)),
  [128, 128],
  'the convex profile must bend the rounded rectangle interior',
)
assert.equal(map.pixels[center + 3], 255)

const edge = pixelOffset(63, 16)
assert.notDeepEqual(
  Array.from(map.pixels.slice(edge, edge + 2)),
  [128, 128],
  'the straight edge must carry a displacement vector',
)
assert.equal(map.pixels[edge + 3], 255)

for (const [x, y] of [[0, 0], [63, 0], [0, 31], [63, 31]]) {
  assert.equal(
    map.pixels[pixelOffset(x, y) + 3],
    0,
    `rounded corner outside pixel ${x},${y} must be transparent`,
  )
}

const repeat = computeRoundedRectDisplacementMap(options)
assert.deepEqual(Array.from(repeat.pixels), Array.from(map.pixels))
assert.equal(repeat.maxDisplacement, map.maxDisplacement)

const opticalOptions = {
  width: 96,
  height: 48,
  radius: 12,
  depth: 12,
  curvature: 0.42,
  bend: 0.78,
  bendWidth: 0.14,
  specularAngle: 315,
  strength: 14,
}
const opticalMap = computeRoundedRectDisplacementMap(opticalOptions)
const nearCenter = pixelOffsetFor(56, 24, opticalOptions.width)
assert.notDeepEqual(
  Array.from(opticalMap.pixels.slice(nearCenter, nearCenter + 2)),
  [128, 128],
  'curvature must bend the broad lens interior instead of leaving it neutral',
)

const brightEdge = pixelOffsetFor(80, 4, opticalOptions.width)
const darkEdge = pixelOffsetFor(16, 44, opticalOptions.width)
assert.ok(
  opticalMap.pixels[brightEdge + 2] > opticalMap.pixels[darkEdge + 2],
  'the blue channel must encode a directional specular mask',
)

const legacyBezelMap = computeRoundedRectDisplacementMap({
  ...opticalOptions,
  depth: undefined,
  bezel: opticalOptions.depth,
})
assert.deepEqual(
  Array.from(legacyBezelMap.pixels),
  Array.from(opticalMap.pixels),
  'legacy bezel must remain a byte-for-byte depth alias',
)

const kpiRegion = computeSvgFilterRegion({
  width: 192,
  height: 96,
  strength: 14,
  blur: 2,
})
const explicitDispersionRegion = computeSvgFilterRegion({
  width: 192,
  height: 96,
  strength: 14,
  blur: 2,
  dispersion: 0.18,
})
assert.ok(kpiRegion.padXPercent >= 13)
assert.ok(kpiRegion.padYPercent >= 23)
assert.equal(kpiRegion.x, `-${kpiRegion.padXPercent}%`)
assert.equal(kpiRegion.width, `${100 + kpiRegion.padXPercent * 2}%`)
assert.deepEqual(
  explicitDispersionRegion,
  kpiRegion,
  'dispersion must represent the additional RGB scale fraction, not the full multiplier',
)

for (const invalidOptions of [
  {},
  { ...options, width: 0 },
  { ...options, width: 1.5 },
  { ...options, height: -1 },
  { ...options, radius: -1 },
  { ...options, radius: 17 },
  { ...options, bezel: 0 },
  { ...options, strength: Number.NaN },
  { ...opticalOptions, depth: 0 },
  { ...opticalOptions, depth: 25 },
  { ...opticalOptions, curvature: -0.01 },
  { ...opticalOptions, curvature: 1.01 },
  { ...opticalOptions, bend: -0.01 },
  { ...opticalOptions, bend: 1.01 },
  { ...opticalOptions, bendWidth: 0 },
  { ...opticalOptions, bendWidth: 0.51 },
  { ...opticalOptions, specularAngle: Number.POSITIVE_INFINITY },
  { ...opticalOptions, strength: 25 },
]) {
  assert.throws(
    () => computeRoundedRectDisplacementMap(invalidOptions),
    TypeError,
    `invalid options must throw: ${JSON.stringify(invalidOptions)}`,
  )
}

for (const invalidRegion of [
  { width: 0, height: 96, strength: 14, blur: 2 },
  { width: 192, height: 96, strength: 0, blur: 2 },
  { width: 192, height: 96, strength: 14, blur: -1 },
  { width: 192, height: 96, strength: 14, blur: 2, dispersion: -0.01 },
]) {
  assert.throws(
    () => computeSvgFilterRegion(invalidRegion),
    TypeError,
    `invalid filter region options must throw: ${JSON.stringify(invalidRegion)}`,
  )
}

const encodeCalls = {
  element: 0,
  context: '',
  imageData: null,
  putImageData: null,
  dataUrlType: '',
}
const fakeCanvas = {
  width: 0,
  height: 0,
  getContext(kind) {
    encodeCalls.context = kind
    return {
      createImageData(width, height) {
        encodeCalls.imageData = { width, height }
        return {
          width,
          height,
          data: new Uint8ClampedArray(width * height * 4),
        }
      },
      putImageData(imageData, x, y) {
        encodeCalls.putImageData = {
          data: Array.from(imageData.data),
          x,
          y,
        }
      },
    }
  },
  toDataURL(type) {
    encodeCalls.dataUrlType = type
    return 'data:image/png;base64,AAAA'
  },
}
const fakeDocument = {
  createElement(tagName) {
    assert.equal(tagName, 'canvas')
    encodeCalls.element += 1
    return fakeCanvas
  },
}

assert.equal(encodeDisplacementMapDataUrl(map, fakeDocument), 'data:image/png;base64,AAAA')
assert.equal(encodeCalls.element, 1)
assert.equal(fakeCanvas.width, map.width)
assert.equal(fakeCanvas.height, map.height)
assert.equal(encodeCalls.context, '2d')
assert.deepEqual(encodeCalls.imageData, { width: map.width, height: map.height })
assert.deepEqual(encodeCalls.putImageData, {
  data: Array.from(map.pixels),
  x: 0,
  y: 0,
})
assert.equal(encodeCalls.dataUrlType, 'image/png')

const createProbeFixture = (
  computedFilter = 'url("#noyo-dashboard-refraction-probe")',
  computedBackdrop = 'blur(0.5px)',
  removeThrows = false,
  hasRemove = true,
) => {
  const calls = { appended: 0, removed: 0 }
  const probe = {
    style: {},
  }
  if (hasRemove) {
    probe.remove = () => {
      calls.removed += 1
      if (removeThrows) throw new Error('probe cleanup failed')
    }
  }
  return {
    calls,
    documentRef: {
      body: {
        appendChild(node) {
          assert.equal(node, probe)
          calls.appended += 1
        },
      },
      createElement(tagName) {
        assert.equal(tagName, 'div')
        return probe
      },
    },
    windowRef: {
      getComputedStyle(node) {
        assert.equal(node, probe)
        return {
          filter: computedFilter,
          webkitFilter: computedFilter,
          backdropFilter: computedBackdrop,
          webkitBackdropFilter: computedBackdrop,
        }
      },
    },
  }
}

const supportedProbe = createProbeFixture()
assert.equal(supportsBackdropSvgFilter(
  { supports: () => true },
  supportedProbe.windowRef,
  supportedProbe.documentRef,
  { userAgent: 'Mozilla/5.0 Edg/152.0.0.0' },
), true)
assert.deepEqual(supportedProbe.calls, { appended: 1, removed: 1 })

const oldEdgeProbe = createProbeFixture()
assert.equal(supportsBackdropSvgFilter(
  { supports: () => true },
  oldEdgeProbe.windowRef,
  oldEdgeProbe.documentRef,
  { userAgent: 'Mozilla/5.0 Edg/129.0.0.0' },
), false)
assert.deepEqual(oldEdgeProbe.calls, { appended: 0, removed: 0 })

const rejectedComputedProbe = createProbeFixture('none')
assert.equal(supportsBackdropSvgFilter(
  { supports: () => true },
  rejectedComputedProbe.windowRef,
  rejectedComputedProbe.documentRef,
  { userAgent: 'Mozilla/5.0 Chrome/152.0.0.0' },
), false)
assert.deepEqual(rejectedComputedProbe.calls, { appended: 1, removed: 1 })

const failedCleanupProbe = createProbeFixture(
  'url("#noyo-dashboard-refraction-probe")',
  'blur(0.5px)',
  true,
)
assert.equal(supportsBackdropSvgFilter(
  { supports: () => true },
  failedCleanupProbe.windowRef,
  failedCleanupProbe.documentRef,
  { userAgent: 'Mozilla/5.0 Edg/152.0.0.0' },
), false)
assert.deepEqual(failedCleanupProbe.calls, { appended: 1, removed: 1 })

const missingCleanupProbe = createProbeFixture(
  'url("#noyo-dashboard-refraction-probe")',
  'blur(0.5px)',
  false,
  false,
)
assert.equal(supportsBackdropSvgFilter(
  { supports: () => true },
  missingCleanupProbe.windowRef,
  missingCleanupProbe.documentRef,
  { userAgent: 'Mozilla/5.0 Edg/152.0.0.0' },
), false)
assert.deepEqual(missingCleanupProbe.calls, { appended: 1, removed: 0 })

for (const unsupported of [
  [{ supports: () => false }, supportedProbe.windowRef, supportedProbe.documentRef, { userAgent: 'Edg/152.0.0.0' }],
  [{ supports: () => true }, supportedProbe.windowRef, supportedProbe.documentRef, { userAgent: 'Firefox/152.0' }],
  [{}, supportedProbe.windowRef, supportedProbe.documentRef, { userAgent: 'Edg/152.0.0.0' }],
  [null, supportedProbe.windowRef, supportedProbe.documentRef, { userAgent: 'Edg/152.0.0.0' }],
  [{ supports: () => true }, null, supportedProbe.documentRef, { userAgent: 'Edg/152.0.0.0' }],
  [{ supports: () => true }, supportedProbe.windowRef, null, { userAgent: 'Edg/152.0.0.0' }],
]) {
  assert.equal(supportsBackdropSvgFilter(...unsupported), false)
}

console.log('liquid glass refraction tests passed')
