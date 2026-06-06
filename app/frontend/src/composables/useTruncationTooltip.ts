import { ref } from 'vue'

export type TooltipPlacement = 'bottom' | 'right'

export function useTruncationTooltip(options: { placement?: TooltipPlacement; delay?: number } = {}) {
  const { placement = 'bottom', delay = 350 } = options

  const tooltipText = ref('')
  const tooltipX = ref(0)
  const tooltipY = ref(0)
  const tooltipShow = ref(false)
  const arrowPlacement = placement

  let showTimer: number | null = null
  let hideTimer: number | null = null

  function onEnter(e: MouseEvent, fullText: string) {
    const el = e.currentTarget as HTMLElement
    if (el.scrollWidth <= el.clientWidth) return

    if (showTimer) {
      window.clearTimeout(showTimer)
      showTimer = null
    }
    if (hideTimer) {
      window.clearTimeout(hideTimer)
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

    showTimer = window.setTimeout(() => {
      tooltipShow.value = true
    }, delay)
  }

  function onLeave() {
    if (showTimer) {
      window.clearTimeout(showTimer)
      showTimer = null
    }
    hideTimer = window.setTimeout(() => {
      tooltipShow.value = false
    }, 100)
  }

  return { tooltipText, tooltipX, tooltipY, tooltipShow, arrowPlacement, onEnter, onLeave }
}
