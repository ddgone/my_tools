import gsap from 'gsap'
import type { Directive } from 'vue'

export const vPress: Directive<HTMLElement> = {
  mounted(el: HTMLElement) {
    const tweens: gsap.core.Tween[] = []

    const onDown = () => {
      gsap.killTweensOf(el, 'scale')
      tweens.push(gsap.to(el, { scale: 0.94, duration: 0.06, ease: 'power2.in', overwrite: 'auto' }))
    }
    const onUp = () => {
      gsap.killTweensOf(el, 'scale')
      tweens.push(gsap.to(el, { scale: 1, duration: 0.25, ease: 'back.out(1.7)', overwrite: 'auto' }))
    }
    const onLeave = () => {
      gsap.killTweensOf(el, 'scale')
      tweens.push(gsap.to(el, { scale: 1, duration: 0.2, ease: 'power2.out', overwrite: 'auto' }))
    }

    el.addEventListener('pointerdown', onDown)
    el.addEventListener('pointerup', onUp)
    el.addEventListener('pointerleave', onLeave)

    ;(el as any).__pressHandlers = { onDown, onUp, onLeave }
  },

  unmounted(el: HTMLElement) {
    const handlers = (el as any).__pressHandlers
    if (!handlers) return
    el.removeEventListener('pointerdown', handlers.onDown)
    el.removeEventListener('pointerup', handlers.onUp)
    el.removeEventListener('pointerleave', handlers.onLeave)
    gsap.killTweensOf(el, 'scale')
  },
}
