const dialogSelector = '.modal, [role="alertdialog"], [role="dialog"][aria-modal="true"]'
const actionSelector = '[data-dialog-confirm], .modal-footer button.btn-primary, .modal-footer button.btn-success, .modal-footer button.btn-danger, .modal-footer button.noyo-glass-btn--primary, .modal-footer button.noyo-glass-btn--success, .modal-footer button.noyo-glass-btn--danger, button[type="submit"]'

function visible(element) {
  return !element.closest('[hidden], [inert], [aria-hidden="true"]') && element.getClientRects().length > 0 && getComputedStyle(element).visibility !== 'hidden'
}

function layer(element) {
  let value = 0
  for (let node = element; node; node = node.parentElement) {
    value = Math.max(value, Number.parseInt(getComputedStyle(node).zIndex, 10) || 0)
  }
  return value
}

export function handleDialogEnter(event, doc = document) {
  if (event.key !== 'Enter' || event.defaultPrevented || event.isComposing || event.keyCode === 229 || event.ctrlKey || event.altKey || event.metaKey || event.shiftKey) return
  const dialogs = [...doc.querySelectorAll(dialogSelector)].filter(visible)
  // Prefer the innermost dialog on the highest layer, then DOM order.
  const top = dialogs.filter(dialog => !dialogs.some(other => other !== dialog && dialog.contains(other)))
    .sort((a, b) => layer(a) - layer(b)).at(-1)
  if (!top) return
  const target = event.target
  if (top.contains(target) && target.closest('textarea, select, [contenteditable=""], [contenteditable="true"], [role="combobox"], [role="listbox"], .monaco-editor, .cm-editor, [data-dialog-enter="ignore"]')) return
  if (event.repeat) {
    event.preventDefault()
    return
  }
  // Preserve Enter on focused controls, including Cancel and dropdown triggers.
  if (top.contains(target) && target.closest('button, a, input[type="checkbox"], input[type="radio"], input[type="file"]')) return
  const actions = [...top.querySelectorAll(actionSelector)].filter(element =>
    visible(element) && element.closest(dialogSelector) === top)
  const explicit = actions.find(element => element.hasAttribute('data-dialog-confirm'))
  const candidate = explicit || actions.at(-1)
  // LiquidGlassButton forwards attributes to its wrapper, not its native button.
  const action = candidate?.matches('button, input') ? candidate : candidate?.querySelector('button')
  if (!action) return
  event.preventDefault()
  if (action.disabled || action.getAttribute('aria-disabled') === 'true' || action.getAttribute('aria-busy') === 'true') return
  const form = action.form
  if (form && !form.reportValidity()) return
  action.click()
}

export function installDialogKeyboard(doc = document) {
  doc.addEventListener('keydown', handleDialogEnter)
  return () => doc.removeEventListener('keydown', handleDialogEnter)
}
