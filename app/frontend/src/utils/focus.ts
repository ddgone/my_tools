export function blurActiveElement() {
  const active = document.activeElement
  if (active instanceof HTMLElement) {
    active.blur()
  }
}

export function focusElementSafely(element: HTMLElement | null | undefined) {
  if (!element) {
    return
  }
  element.focus({ preventScroll: true })
}
