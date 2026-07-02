import { appBrand } from '../config/brand.js'

export const pluginIconUrl = (icon) => {
  return icon || appBrand.logoUrl
}
