export type Position = [number, number]
export type MapPoint = {
  deviceCode: string
  /** 展示名。取不到时渲染方回退到 deviceCode。 */
  deviceName?: string
  latitude: number
  longitude: number
  coordinateSystem: string
  quality?: string
}

function position(value: any): Position {
  if (!Array.isArray(value) || value.length < 2 || typeof value[0] !== 'number' || typeof value[1] !== 'number' || !Number.isFinite(value[0]) || !Number.isFinite(value[1]) || Math.abs(value[0]) > 180 || Math.abs(value[1]) > 90) throw new Error('INVALID_POSITION')
  return [value[0], value[1]]
}
// Keep only numeric geometry. GeoJSON properties, URLs and legacy CRS metadata
// never enter the saved document or SVG markup.
export function importOutline(value: any): Position[][] {
  const lines: Position[][] = []
  let count = 0
  const line = (raw: any) => {
    if (!Array.isArray(raw) || raw.length < 2 || lines.length >= 256) throw new Error('INVALID_OUTLINE')
    count += raw.length
    if (count > 10000) throw new Error('OUTLINE_LIMIT')
    lines.push(raw.map(position))
  }
  const geometry = (raw: any) => {
    if (!raw || raw.crs) throw new Error('INVALID_OUTLINE')
    if (raw.type === 'LineString') line(raw.coordinates)
    else if (['MultiLineString', 'Polygon'].includes(raw.type) && Array.isArray(raw.coordinates)) raw.coordinates.forEach(line)
    else if (raw.type === 'MultiPolygon' && Array.isArray(raw.coordinates)) raw.coordinates.forEach((polygon: any) => { if (!Array.isArray(polygon)) throw new Error('INVALID_OUTLINE'); polygon.forEach(line) })
    else throw new Error('INVALID_OUTLINE')
  }
  if (value?.crs) throw new Error('INVALID_OUTLINE')
  if (value?.type === 'FeatureCollection' && Array.isArray(value.features) && value.features.length <= 256) value.features.forEach((feature: any) => { if (feature.type !== 'Feature') throw new Error('INVALID_OUTLINE'); geometry(feature.geometry) })
  else if (value?.type === 'Feature') geometry(value.geometry)
  else geometry(value)
  if (!lines.length) throw new Error('INVALID_OUTLINE')
  return lines
}
export function safePoints(value: any, coordinateSystem: string): MapPoint[] {
  if (!Array.isArray(value)) return []
  return value.slice(0, 200).filter(point => {
    try { position([point?.longitude, point?.latitude]); return typeof point.deviceCode === 'string' && point.coordinateSystem === coordinateSystem } catch { return false }
  })
}
export function mapProjection(points: MapPoint[], lines: Position[][], width: number, height: number) {
  const positions: Position[] = [...points.map(point => [point.longitude, point.latitude] as Position), ...lines.flat()]
  const xs = positions.map(p => p[0]), ys = positions.map(p => p[1])
  let minX = positions.length ? Math.min(...xs) : -180, maxX = positions.length ? Math.max(...xs) : 180
  let minY = positions.length ? Math.min(...ys) : -90, maxY = positions.length ? Math.max(...ys) : 90
  const dx = Math.max(maxX - minX, .02), dy = Math.max(maxY - minY, .02)
  const cx = (maxX + minX) / 2, cy = (maxY + minY) / 2
  minX = cx - dx * .6; maxX = cx + dx * .6; minY = cy - dy * .6; maxY = cy + dy * .6
  const scale = Math.min(Math.max(1, width - 64) / (maxX - minX), Math.max(1, height - 80) / (maxY - minY))
  const project = ([x, y]: Position) => [width / 2 + (x - cx) * scale, height / 2 - (y - cy) * scale]
  return { project, minX, maxX, minY, maxY }
}
