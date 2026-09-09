let count = 0
let locale = 'zh'
let baseTitle = ''
let renderedTitle = ''
let timer = null
let observer = null
let icon = null
let originalHref = null
let createdIcon = false
let flash = false

function renderTitle() {
  const label = String(locale).toLowerCase().startsWith('en') ? 'Alarm' : '告警'
  renderedTitle = count ? `(${count}) [${label}] ${flash ? '! ' : ''}${baseTitle}` : baseTitle
  document.title = renderedTitle
}

function visibilityChanged() {
  clearInterval(timer)
  timer = null
  flash = false
  renderTitle()
  if (count && document.hidden) {
    timer = setInterval(() => { flash = !flash; renderTitle() }, 1500)
  }
}

export function setAlarmBadge(unreadCount = 0, language = 'zh') {
  if (typeof document === 'undefined') return
  const next = Math.max(0, Math.floor(Number(unreadCount) || 0))
  if (!next) { clearAlarmBadge(); return }
  if (!count) {
    baseTitle = document.title
    icon = document.querySelector('link[rel="icon"]')
    createdIcon = !icon
    if (!icon) {
      icon = document.createElement('link')
      icon.rel = 'icon'
      document.head.appendChild(icon)
    }
    originalHref = icon.getAttribute('href')
    document.addEventListener('visibilitychange', visibilityChanged)
    observer = new MutationObserver(() => {
      if (document.title !== renderedTitle) {
        baseTitle = document.title
        renderTitle()
      }
    })
    observer.observe(document.head, { childList: true, subtree: true, characterData: true })
  }
  count = next
  locale = language
  // Synchronous SVG avoids stale image.onload callbacks restoring an old badge.
  const tokens = getComputedStyle(document.documentElement)
  const danger = tokens.getPropertyValue('--bs-danger').trim()
  const foreground = tokens.getPropertyValue('--bs-body-bg').trim()
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="15" fill="${danger}"/><text x="16" y="22" text-anchor="middle" font-family="sans-serif" font-size="18" fill="${foreground}">${count > 99 ? '!' : count}</text></svg>`
  icon.href = `data:image/svg+xml,${encodeURIComponent(svg)}`
  visibilityChanged()
}

export function clearAlarmBadge() {
  if (typeof document === 'undefined' || !count) return
  observer?.disconnect()
  observer = null
  document.removeEventListener('visibilitychange', visibilityChanged)
  clearInterval(timer)
  timer = null
  count = 0
  flash = false
  if (document.title === renderedTitle) document.title = baseTitle
  if (createdIcon) icon?.remove()
  else if (originalHref === null) icon?.removeAttribute('href')
  else icon?.setAttribute('href', originalHref)
  icon = null
}
