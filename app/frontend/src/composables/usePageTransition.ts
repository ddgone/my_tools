import { ref, type Ref, nextTick } from 'vue'
import gsap from 'gsap'
import { TAB_SWITCH_FROM } from '@/utils/animation'

export function usePageTransition(containerRef: Ref<HTMLElement | null>) {
  const transitioning = ref(false)

  let ctx: gsap.Context | null = null

  function setup() {
    if (!containerRef.value) return
    ctx = gsap.context(() => {}, containerRef.value)
  }

  function teardown() {
    ctx?.revert()
    ctx = null
  }

  async function animateIn() {
    if (!containerRef.value) return
    transitioning.value = true
    gsap.fromTo(
      containerRef.value.children,
      { opacity: 0, y: TAB_SWITCH_FROM.y },
      { opacity: 1, y: 0, duration: TAB_SWITCH_FROM.duration, ease: TAB_SWITCH_FROM.ease, stagger: 0.03 },
    )
    await nextTick()
    transitioning.value = false
  }

  async function animateSwitch() {
    if (!containerRef.value) return
    transitioning.value = true

    gsap.fromTo(
      containerRef.value.children,
      { opacity: 0, y: TAB_SWITCH_FROM.y },
      { opacity: 1, y: 0, duration: TAB_SWITCH_FROM.duration, ease: TAB_SWITCH_FROM.ease, stagger: 0.03 },
    )

    await nextTick()
    transitioning.value = false
  }

  return { transitioning, setup, teardown, animateIn, animateSwitch }
}
