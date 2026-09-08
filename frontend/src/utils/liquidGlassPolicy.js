const GLASS_PROFILES = new Set(['regular', 'floating', 'clear-media'])
const GLASS_SHAPES = new Set(['rounded', 'pill', 'circle', 'panel'])
const GLASS_COMPOSITIONS = new Set(['control', 'island'])
const SOLID_DENSITIES = new Set(['comfortable', 'compact'])
const SOLID_ELEVATIONS = new Set(['flat', 'raised'])

export function resolveGlassGroupPolicy(options = {}) {
  const profile = GLASS_PROFILES.has(options.profile) ? options.profile : 'regular'
  const shape = GLASS_SHAPES.has(options.shape) ? options.shape : 'rounded'
  const composition = GLASS_COMPOSITIONS.has(options.composition)
    ? options.composition
    : 'control'
  const interactive = options.interactive === true

  return {
    profile,
    shape,
    composition,
    classes: [
      'noyo-glass-group',
      'noyo-glass-group--' + profile,
      'noyo-glass-group--shape-' + shape,
      'noyo-glass-group--' + composition,
      interactive
        ? 'noyo-glass-group--interactive'
        : 'noyo-glass-group--static',
    ],
  }
}

export function resolveSolidSurfacePolicy(options = {}) {
  const density = SOLID_DENSITIES.has(options.density)
    ? options.density
    : 'comfortable'
  const elevation = SOLID_ELEVATIONS.has(options.elevation)
    ? options.elevation
    : 'raised'
  const interactive = options.interactive === true

  return {
    profile: 'solid-content',
    density,
    elevation,
    classes: [
      'noyo-solid-surface',
      'noyo-solid-surface--' + density,
      'noyo-solid-surface--' + elevation,
      interactive
        ? 'noyo-solid-surface--interactive'
        : 'noyo-solid-surface--static',
    ],
  }
}

export function resolveLiquidGlassPolicy(options = {}) {
  const useSolidSurface =
    options.opaque === true ||
    options.profile === 'content' ||
    options.profile === 'solid'

  if (useSolidSurface) {
    return {
      kind: 'solid',
      ...resolveSolidSurfacePolicy({
        density: options.density,
        elevation: options.elevation,
        interactive: options.interactive,
      }),
    }
  }

  const legacyProfile = {
    chrome: 'regular',
    floating: 'floating',
    regular: 'regular',
    'clear-media': 'clear-media',
  }[options.profile] || 'regular'

  return {
    kind: 'glass',
    ...resolveGlassGroupPolicy({
      profile: legacyProfile,
      shape: options.shape,
      composition: options.composition,
      interactive: options.interactive,
    }),
  }
}
