import { createMaterial, materials, thumbnailFor } from '@/noyo/materials'
import { state } from '@/noyo/bridge'
// Re-exported, not re-bound: the palette store reads this during module
// initialisation, and a local `const` would hit the temporal dead zone while
// `materials.ts` is still resolving the globbed component configs.
export { packagesList } from '@/noyo/materials'
export const createComponent = async (data: any) => createMaterial(data.key, state.locale)
export const fetchChartComponent = (data: any) => materials.find(m => m.key === data.key)?.view
export const fetchConfigComponent = (data: any) =>
  materials.find(m => m.key === data.key)?.panel || { render: () => null }
export const fetchImages = async (data: any) => thumbnailFor(data)
