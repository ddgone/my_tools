<script setup lang="ts">
import { computed } from 'vue'
import goIconUrl from 'devicon/icons/go/go-original.svg'
import pythonIconUrl from 'devicon/icons/python/python-original.svg'
import rustIconUrl from 'devicon/icons/rust/rust-original.svg'

const props = withDefaults(defineProps<{
  kind?: string | null
  size?: number
  variant?: 'compact' | 'framed'
  opacity?: number
  title?: string
}>(), {
  kind: '',
  size: 14,
  variant: 'compact',
  opacity: 0.92,
  title: '',
})

const iconMap: Record<string, string> = {
  go: goIconUrl,
  python: pythonIconUrl,
  rust: rustIconUrl,
}

const normalizedKind = computed(() => {
  const kind = props.kind?.trim().toLowerCase()
  return kind && kind in iconMap ? kind : ''
})

const iconSrc = computed(() => iconMap[normalizedKind.value] ?? '')

const rootStyle = computed(() => ({
  width: `${props.size}px`,
  height: `${props.size}px`,
  opacity: String(props.opacity),
}))
</script>

<template>
  <span
    v-if="iconSrc"
    class="inline-flex shrink-0 items-center justify-center overflow-hidden select-none"
    :class="variant === 'framed'
      ? 'rounded-md border border-[rgb(var(--color-border-subtle)/0.58)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-[2px] shadow-[inset_0_1px_0_rgb(var(--color-fg-base)/0.03)]'
      : ''"
    :style="rootStyle"
    :title="title || undefined"
    aria-hidden="true"
  >
    <img
      :src="iconSrc"
      :alt="title || normalizedKind"
      class="block h-full w-full object-contain"
      draggable="false"
    >
  </span>
</template>
