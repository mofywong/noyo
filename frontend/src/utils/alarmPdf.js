export function alarmPdfFilename(device, alarm, occurredAt) {
  const clean = value => String(value || '—').replace(/[<>:"/\\|?*\u0000-\u001f]/g, '-').trim().replace(/[. ]+$/g, '').slice(0, 70) || '—'
  const date = occurredAt ? new Date(occurredAt) : new Date(NaN)
  const pad = value => String(value).padStart(2, '0')
  const time = Number.isNaN(date.getTime()) ? clean(occurredAt) : `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}_${pad(date.getHours())}-${pad(date.getMinutes())}-${pad(date.getSeconds())}`
  return `${clean(device)}-${clean(alarm)}-${time}.pdf`
}

// Resolve the drawer's Bootstrap tokens, including its active light/dark theme.
// Browser font rendering preserves Chinese text without a PDF font dependency.
export async function exportAlarmPdf({ title, blocks, filename, imageFailure }) {
  const { jsPDF } = await import('jspdf')
  await document.fonts.ready
  const probe = document.createElement('div')
  document.body.append(probe)
  const color = token => { probe.style.color = `var(${token})`; return getComputedStyle(probe).color }
  const palette = {
    page: color('--bs-tertiary-bg'), card: color('--bs-body-bg'), text: color('--bs-body-color'),
    muted: color('--bs-secondary-color'), border: color('--bs-border-color'),
    primary: color('--bs-primary'), emphasis: color('--bs-primary-text-emphasis'), subtle: color('--bs-primary-bg-subtle'),
  }
  probe.remove()
  const pdf = new jsPDF({ unit: 'mm', format: 'a4', compress: true })
  const canvas = document.createElement('canvas')
  canvas.width = 1240; canvas.height = 1754
  const ctx = canvas.getContext('2d')
  const margin = 72, width = 1096, bottom = 1650
  const family = '"Microsoft YaHei", "Noto Sans SC", sans-serif'
  let y = 0, page = 0
  const font = (size = 23, bold = false) => { ctx.font = `${bold ? '600 ' : ''}${size}px ${family}` }
  const lines = (value, maxWidth, size = 23, bold = false) => {
    font(size, bold)
    return String(value ?? '').split('\n').flatMap(paragraph => {
      const result = []; let line = ''
      for (const char of paragraph) {
        if (line && ctx.measureText(line + char).width > maxWidth) { result.push(line); line = '' }
        line += char
      }
      result.push(line)
      return result
    })
  }
  const draw = (text, x, top, size = 23, bold = false, ink = palette.text) => {
    font(size, bold); ctx.fillStyle = ink; ctx.fillText(text, x, top + size)
  }
  const card = (x, top, w, h, fill = palette.card) => {
    ctx.beginPath(); ctx.roundRect(x, top, w, h, 16)
    ctx.fillStyle = fill; ctx.fill(); ctx.strokeStyle = palette.border; ctx.lineWidth = 1; ctx.stroke()
  }
  const reset = () => {
    ctx.fillStyle = palette.page; ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.fillStyle = palette.primary; ctx.fillRect(0, 0, canvas.width, 8)
    draw('NOYO', margin, 40, 22, true, palette.primary)
    draw(title, margin + 112, 40, 22, false, palette.muted)
    y = 104
  }
  const flush = () => {
    ctx.strokeStyle = palette.border; ctx.beginPath(); ctx.moveTo(margin, 1682); ctx.lineTo(1168, 1682); ctx.stroke()
    draw('NOYO · ' + title, margin, 1698, 18, false, palette.muted)
    draw(String(++page), 1120, 1698, 18, false, palette.muted)
    if (page > 1) pdf.addPage()
    pdf.addImage(canvas.toDataURL('image/png'), 'PNG', 0, 0, 210, 297)
    reset()
  }
  const ensure = height => { if (y + height > bottom) flush() }
  const textCard = (value, { heading = false, timeline = false, hero = false } = {}) => {
    const size = hero ? 36 : heading ? 28 : 23
    const lineHeight = hero ? 48 : 36
    const content = lines(value, width - 64, size, heading || hero)
    if (heading && !hero) {
      ensure(content.length * lineHeight + 112); y += 16
      ctx.fillStyle = palette.primary; ctx.fillRect(margin, y + 6, 4, 28)
      for (const line of content) { draw(line, margin + 20, y, size, true); y += lineHeight }
      y += 16
      return
    }
    while (content.length) {
      ensure(lineHeight + 64)
      const count = Math.max(1, Math.floor((bottom - y - 48) / lineHeight))
      const chunk = content.splice(0, count)
      const h = chunk.length * lineHeight + 48
      card(margin, y, width, h, hero ? palette.subtle : palette.card)
      if (timeline) {
        ctx.fillStyle = palette.primary; ctx.beginPath(); ctx.arc(margin + 14, y + 38, 5, 0, Math.PI * 2); ctx.fill()
      }
      chunk.forEach((line, i) => draw(line, margin + 32, y + 24 + i * lineHeight, size, hero, hero ? palette.emphasis : palette.text))
      y += h + 16
      if (content.length) flush()
    }
  }
  const fields = values => {
    for (let i = 0; i < values.length; i += 2) {
      const pair = values.slice(i, i + 2).map(v => ({ label: lines(v.label, 468, 20), value: lines(v.value, 468) }))
      while (pair.some(v => v.label.length || v.value.length)) {
        ensure(144)
        const slots = Math.max(2, Math.floor((bottom - y - 64) / 34))
        const chunks = pair.map(v => {
          const label = v.label.splice(0, slots - 1)
          const value = v.label.length ? [] : v.value.splice(0, slots - label.length)
          return { label, value }
        })
        const h = Math.max(...chunks.map(v => v.label.length + v.value.length)) * 34 + 64
        chunks.forEach((v, j) => {
          const x = margin + j * 560
          card(x, y, 536, h)
          let top = y + 24
          v.label.forEach(line => { draw(line, x + 32, top, 20, false, palette.muted); top += 34 })
          v.value.forEach(line => { draw(line, x + 32, top); top += 34 })
        })
        y += h + 16
      }
    }
  }
  reset()
  for (const [index, block] of blocks.entries()) {
    if (block.heading && blocks[index + 1]?.image) ensure(736)
    if (block.fields) fields(block.fields)
    if (block.text) textCard(block.text, block)
    if (!block.image) continue
    try {
      const img = await new Promise((resolve, reject) => {
        const image = new Image()
        const timer = setTimeout(() => { image.src = ''; reject(new Error('Image timeout')) }, 15000)
        image.crossOrigin = 'anonymous'
        image.onload = () => { clearTimeout(timer); resolve(image) }
        image.onerror = () => { clearTimeout(timer); reject(new Error('Image unavailable')) }
        image.src = block.image
      })
      const scale = Math.min((width - 48) / img.naturalWidth, 580 / img.naturalHeight)
      const w = img.naturalWidth * scale, h = img.naturalHeight * scale
      ensure(h + 64); card(margin, y, width, h + 48)
      ctx.drawImage(img, margin + (width - w) / 2, y + 24, w, h)
      y += h + 64
    } catch { textCard(imageFailure) }
  }
  flush()
  pdf.save(filename)
}
