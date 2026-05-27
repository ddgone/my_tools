import { ref } from 'vue'

export type TooltipPlacement = 'bottom' | 'right'

export function useTruncationTooltip(options: { placement?: TooltipPlacement; delay?: number } = {}) {
  const { placement = 'bottom', delay = 350 } = options

  const tooltipText = ref('')
  const tooltipX = ref(0)
  const tooltipY = ref(0)
  const tooltipShow = ref(false)
  const arrowPlacement = placement

  let showTimer: ReturnType<typeof setTimeout> | null = null
  let hideTimer: ReturnType<typeof setTimeout> | null = null

  function onEnter(e: MouseEvent, fullText: string) {
    const el = e.currentTarget as HTMLElement
    if (el.scrollWidth <= el.clientWidth) return

    if (showTimer) {
      clearTimeout(showTimer)
      showTimer = null
    }
    if (hideTimer) {
      clearTimeout(hideTimer)
      hideTimer = null
    }
    tooltipShow.value = false

    tooltipText.value = fullText
    const rect = el.getBoundingClientRect()

    if (placement === 'right') {
      tooltipX.value = rect.right + 8
      tooltipY.value = rect.top + rect.height / 2
    } else {
      tooltipX.value = rect.left + rect.width / 2
      tooltipY.value = rect.bottom + 4
    }

    showTimer = setTimeout(() => {
      tooltipShow.value = true
    }, delay)
  }

  function onLeave() {
    if (showTimer) {
      clearTimeout(showTimer)
      showTimer = null
    }
    hideTimer = setTimeout(() => {
      tooltipShow.value = false
    }, 100)
  }

  return { tooltipText, tooltipX, tooltipY, tooltipShow, arrowPlacement, onEnter, onLeave }
}
