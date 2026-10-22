// Canvas coordinates are independent of the editor's zoom and pan.
export function canvasPoint(clientX, clientY, rect, canvas) {
  if (!rect.width || !rect.height) return null
  return {
    x: Math.round(Math.max(0, Math.min(canvas.width, (clientX - rect.left) * canvas.width / rect.width))),
    y: Math.round(Math.max(0, Math.min(canvas.height, (clientY - rect.top) * canvas.height / rect.height)))
  }
}

export function areaBetween(start, end) {
  return { x: Math.min(start.x, end.x), y: Math.min(start.y, end.y), w: Math.abs(end.x - start.x), h: Math.abs(end.y - start.y) }
}

export function componentsInArea(components, area) {
  return components.filter(({ attr }) => attr && attr.x < area.x + area.w && attr.x + attr.w > area.x && attr.y < area.y + area.h && attr.y + attr.h > area.y).map(item => item.id)
}

export function componentAtPoint(components, point) {
  return [...components].reverse().find(({ attr }) => attr && point.x >= attr.x && point.x <= attr.x + attr.w && point.y >= attr.y && point.y <= attr.y + attr.h)
}

export function selectedContext(components, ids) {
  const wanted = new Set(ids), items = []
  function visit(list) {
    for (const item of list) {
      if (wanted.has(item.id)) items.push({ id: item.id, key: item.chartConfig?.key || 'group', title: item.chartConfig?.title || '' })
      if (item.isGroup) visit(item.groupList || [])
    }
  }
  visit(components)
  return items
}
