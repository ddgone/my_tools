import { computed, onUnmounted, ref, watch, type Ref } from 'vue'

export type Axis = 'x' | 'y'

interface ResizableOptions {
  axis: Axis
  min: number | Ref<number> | (() => number)
  max: number | Ref<number> | (() => number)
  initial: number
  reverse?: boolean
  storageKey?: string
}

function resolveBound(bound: number | Ref<number> | (() => number)): number {
  if (typeof bound === 'function') {
    return bound()
  }
  if (typeof bound === 'object' && bound !== null && 'value' in bound) {
    return bound.value
  }
  return bound
}

function loadSize(key: string, fallback: number): number {
  try {
    const raw = localStorage.getItem(key)
    if (raw !== null) {
      const parsed = Number(raw)
      if (!Number.isNaN(parsed)) return parsed
    }
  } catch {
    /* localStorage unavailable */
  }
  return fallback
}

function saveSize(key: string, value: number) {
  try {
    localStorage.setItem(key, String(value))
  } catch {
    /* localStorage unavailable */
  }
}

export function useResizable(options: ResizableOptions): {
  size: Ref<number>
  dividerProps: {
    onPointerdown: (e: PointerEvent) => void
    class: string
    style: Record<string, string>
    'data-dir': string
  }
} {
  const initialValue = options.storageKey
    ? loadSize(options.storageKey, options.initial)
    : options.initial

  const clamped = (v: number) => {
    const min = resolveBound(options.min)
    const max = resolveBound(options.max)
    const safeMin = Math.min(min, max)
    const safeMax = Math.max(min, max)
    return Math.max(safeMin, Math.min(safeMax, v))
  }

  const rawSize = ref(initialValue)
  const dragging = ref(false)

  const size = computed<number>({
    get: () => clamped(rawSize.value),
    set: (value) => {
      rawSize.value = value
    },
  })

  if (options.storageKey) {
    watch(rawSize, (v) => saveSize(options.storageKey!, v))
  }

  function onPointerdown(e: PointerEvent) {
    e.preventDefault()
    dragging.value = true
    rawSize.value = size.value
    ;(e.target as HTMLElement).setPointerCapture(e.pointerId)
    document.body.style.cursor = options.axis === 'x' ? 'col-resize' : 'row-resize'
    document.body.style.userSelect = 'none'
  }

  function onPointermove(e: PointerEvent) {
    if (!dragging.value) return

    const delta = options.axis === 'x' ? e.movementX : e.movementY
    const adjusted = options.reverse ? -delta : delta
    size.value = clamped(size.value + adjusted)
  }

  function onPointerup() {
    dragging.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
  }

  document.addEventListener('pointermove', onPointermove)
  document.addEventListener('pointerup', onPointerup)

  onUnmounted(() => {
    document.removeEventListener('pointermove', onPointermove)
    document.removeEventListener('pointerup', onPointerup)
  })

  const dividerClass = options.axis === 'x'
    ? 'w-1 cursor-col-resize hover:bg-dracula-cyan/30 active:bg-dracula-cyan/50 shrink-0 transition-colors'
    : 'h-1 cursor-row-resize hover:bg-dracula-cyan/30 active:bg-dracula-cyan/50 shrink-0 transition-colors'

  const dividerStyle: Record<string, string> = options.axis === 'x'
    ? { width: '4px' }
    : { height: '4px' }

  return {
    size,
    dividerProps: {
      onPointerdown,
      class: dividerClass,
      style: dividerStyle,
      'data-dir': options.axis,
    },
  }
}
