import Color from 'color'
import { useDesignStore } from '@/store/modules/designStore/designStore'
import { PickCreateComponentType } from '@/packages/index.d'
import { EditCanvasConfigType } from '@/store/modules/chartEditStore/chartEditStore.d'
import { chartColors, chartColorsSearch, CustomColorsType } from '@/settings/chartThemes/index'

type AttrType = PickCreateComponentType<'attr'>
type StylesType = PickCreateComponentType<'styles'>

// * 动画
export const animationsClass = (animations: string[]) => {
  if (animations.length) {
    return `animate__animated  animate__${animations[0]}`
  }
  return ''
}

// * 滤镜
export const getFilterStyle = (styles?: StylesType | EditCanvasConfigType) => {
  if (!styles || !styles.filterShow) return {}
  const { opacity, saturate, contrast, hueRotate, brightness } = styles
  const result: Record<string, string | number> = {}

  // opacity 为 1/undefined/null 时不设置，避免创建层叠上下文
  if (opacity != null && opacity !== 1) {
    result.opacity = opacity
  }

  // 只收集非默认值的滤镜函数，避免无意义的 filter 计算
  const filters: string[] = []
  if (saturate != null && saturate !== 1) filters.push(`saturate(${saturate})`)
  if (contrast != null && contrast !== 1) filters.push(`contrast(${contrast})`)
  if (hueRotate != null && hueRotate !== 0) filters.push(`hue-rotate(${hueRotate}deg)`)
  if (brightness != null && brightness !== 1) filters.push(`brightness(${brightness})`)
  if (filters.length) {
    result.filter = filters.join(' ')
  }

  return result
}

// * 变换
export const getTransformStyle = (styles: StylesType) => {
  const { rotateZ, rotateX, rotateY, skewX, skewY } = styles
  // 所有值都为 0/undefined/null 时不生成 transform，避免创建层叠上下文
  if (![rotateZ, rotateX, rotateY, skewX, skewY].some(v => v)) {
    return {}
  }
  return {
    transform: `rotateZ(${rotateZ || 0}deg) rotateX(${rotateX || 0}deg) rotateY(${rotateY || 0}deg) skewX(${
      skewX || 0
    }deg) skewY(${skewY || 0}deg)`
  }
}

// * 混合模式
export const getBlendModeStyle = (styles: StylesType) => {
  if (!styles || !styles.filterShow) return {}
  const { blendMode } = styles
  // normal 为默认值，不输出以避免创建层叠上下文
  if (!blendMode || blendMode === 'normal') return {}
  return {
    'mix-blend-mode': blendMode
  }
}

/**
 * * hsla 转换
 * @param color 颜色
 * @param alpha 默认1
 * @returns
 */
export function alpha(color: string, alpha = 1) {
  return Color(color).alpha(alpha).toString()
}

/**
 * * 颜色透明
 * rgba(10, 10, 10, 0.8) -> rgba(10, 10, 10, 0.4)
 * @param color 颜色
 * @param concentration 0~1 浓度
 * @returns
 */
export function fade(color: string, fade: number) {
  return Color(color).fade(fade).toString()
}

/**
 * * 颜色变亮
 * hsl(100, 50%, 10%) -> hsl(100, 50%, 50%)
 * @param color 颜色
 * @param concentration 0~1 浓度
 * @returns
 */
export function lighten(color: string, concentration: number) {
  return Color(color).lighten(concentration).toString()
}

/**
 * * 颜色变暗
 * hsl(100, 50%, 50%) -> hsl(100, 50%, 25%)
 * @param color 颜色
 * @param concentration 0~1 浓度
 * @returns
 */
export function darken(color: string, concentration: number) {
  return Color(color).darken(concentration).toString()
}

/**
 * * hsl 转成16进制
 * @param hsl
 * @returns
 */
export function hslToHexa(hslString: string): string {
  const color = Color(hslString)
  return color.hexa()
}

export function hslToHex(hslString: string): string {
  const color = Color(hslString)
  return color.hex()
}

/**
 * * 修改主题色
 * @param themeName 主题名称
 * @returns
 */
export const setHtmlTheme = (themeName?: string) => {
  const e = window.document.documentElement
  if (themeName) {
    e.setAttribute('data-theme', themeName)
    return
  }
  const designStore = useDesignStore()
  e.setAttribute('data-theme', designStore.themeName)
}

/**
 * * 合并基础颜色和自定义颜色
 * @param chartDefaultColors
 * @param customColor
 * @returns
 */
export const colorCustomMerge = (customColor?: CustomColorsType[]) => {
  type FormateCustomColorType = {
    [T: string]: {
      color: string[]
      name: string
    }
  }
  const formateCustomColor: FormateCustomColorType = {}
  customColor?.forEach(item => {
    formateCustomColor[item.id] = {
      color: item.color,
      name: item.name
    }
  })
  return { ...formateCustomColor, ...chartColors }
}

/**
 * * 合并基础渐变颜色和自定义渐变颜色
 * @param customColor
 */
export const colorGradientCustomMerge = (customColor?: CustomColorsType[]) => {
  type FormateGradientCustomColorType = {
    [T: string]: string[]
  }
  const formateGradientCustomColor: FormateGradientCustomColorType = {}
  customColor?.forEach(item => {
    formateGradientCustomColor[item.id] = [
      item.color[0],
      item.color[1],
      fade(item.color[0], 0.3),
      fade(item.color[0], 0.5),
      fade(item.color[1], 0.5)
    ]
  })

  return { ...formateGradientCustomColor, ...chartColorsSearch }
}
