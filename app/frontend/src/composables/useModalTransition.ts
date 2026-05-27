import gsap from 'gsap'

export function useModalTransition(panelSelector = '.modal-panel') {
  function onEnter(el: Element, done: () => void) {
    const panel = el.querySelector(panelSelector)
    const tl = gsap.timeline({ onComplete: done })
    tl.fromTo(el, { opacity: 0 }, { opacity: 1, duration: 0.15, ease: 'power2.out' })
    if (panel) {
      tl.fromTo(panel, { opacity: 0, scale: 0.95 }, { opacity: 1, scale: 1, duration: 0.22, ease: 'power2.out' }, '-=0.1')
    }
  }

  function onLeave(el: Element, done: () => void) {
    const panel = el.querySelector(panelSelector)
    const tl = gsap.timeline({ onComplete: done })
    if (panel) {
      tl.to(panel, { opacity: 0, scale: 0.95, duration: 0.1, ease: 'power2.in' })
    }
    tl.to(el, { opacity: 0, duration: 0.1, ease: 'power2.in' }, '-=0.05')
  }

  return { onEnter, onLeave }
}
