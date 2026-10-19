import { DEVICE_STATUS_ORDER, deviceStatusColor, mapTokenColor } from './deviceMap.js'
import { DEVICE_DOT_PIXEL_RATIO, DEVICE_DOT_SIZE, drawDeviceDot } from './deviceDotCanvas.js'

// 图标在本地用 canvas 生成，矢量和栅格底图都能用，不依赖远端字形服务。
// 几何与配色规则放在 deviceDotCanvas.js —— 那里是唯一真源。
export function createDeviceMarkerImage(status, selected = false) {
  const canvas = document.createElement('canvas')
  canvas.width = DEVICE_DOT_SIZE.width * DEVICE_DOT_PIXEL_RATIO
  canvas.height = DEVICE_DOT_SIZE.height * DEVICE_DOT_PIXEL_RATIO
  const ctx = canvas.getContext('2d')
  ctx.scale(DEVICE_DOT_PIXEL_RATIO, DEVICE_DOT_PIXEL_RATIO)
  drawDeviceDot(
    ctx,
    {
      statusColor: deviceStatusColor(status),
      surface: mapTokenColor('--map-device-surface-top'),
      highlight: mapTokenColor('--map-device-highlight'),
      shadowColor: mapTokenColor('--map-device-shadow'),
      brand: mapTokenColor('--color-brand'),
    },
    { status, selected },
  )
  return ctx.getImageData(0, 0, canvas.width, canvas.height)
}

export function installDeviceMarkerImages(map, refresh = false) {
  for (const status of DEVICE_STATUS_ORDER) {
    for (const selected of [false, true]) {
      const id = `device-dot-${selected ? 'selected-' : ''}${status}`
      if (!map.hasImage(id)) map.addImage(id, createDeviceMarkerImage(status, selected), { pixelRatio: DEVICE_DOT_PIXEL_RATIO })
      else if (refresh) map.updateImage(id, createDeviceMarkerImage(status, selected))
    }
  }
}
