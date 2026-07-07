<script setup lang="ts">
import { computed, type CSSProperties } from 'vue'
import goIconUrl from '@/assets/icons/go-wordmark-tight.svg'
import pythonIconUrl from 'devicon/icons/python/python-original.svg'
import rustIconUrl from 'devicon/icons/rust/rust-original.svg'

const props = withDefaults(defineProps<{
  kind?: string | null
  size?: number
  slotWidth?: number
  variant?: 'compact' | 'framed'
  opacity?: number
  title?: string
}>(), {
  kind: '',
  size: 14,
  slotWidth: 0,
  variant: 'compact',
  opacity: 0.92,
  title: '',
})

type IconRenderMode = 'image' | 'mask'

type IconMeta = {
  src: string
  contentWidthRatio: number
  renderMode: IconRenderMode
  color?: string
  offsetX?: number
  offsetY?: number
  filter?: string
}

const iconMap: Record<string, IconMeta> = {
  go: {
    src: goIconUrl,
    contentWidthRatio: 1.28,
    renderMode: 'image',
    offsetX: -2,
    offsetY: 0.5,
  },
  python: {
    src: pythonIconUrl,
    contentWidthRatio: 1,
    renderMode: 'image',
    offsetY: 0.5,
    filter: 'saturate(1.12) contrast(1.04)',
  },
  rust: {
    src: rustIconUrl,
    contentWidthRatio: 1,
    renderMode: 'mask',
    color: '#ef6a1a',
    offsetY: 0.5,
  },
}

const normalizedKind = computed(() => {
  const kind = props.kind?.trim().toLowerCase()
  return kind && kind in iconMap ? kind : ''
})

const iconMeta = computed(() => iconMap[normalizedKind.value] ?? null)
const iconSrc = computed(() => iconMeta.value?.src ?? '')
const contentWidth = computed(() => Math.round(props.size * (iconMeta.value?.contentWidthRatio ?? 1)))
const contentOffsetX = computed(() => iconMeta.value?.offsetX ?? 0)
const contentOffsetY = computed(() => iconMeta.value?.offsetY ?? 0)

const rootStyle = computed<CSSProperties>(() => ({
  width: `${Math.max(props.slotWidth || 0, contentWidth.value)}px`,
  height: `${props.size}px`,
  opacity: String(props.opacity),
}))

const contentStyle = computed<CSSProperties>(() => ({
  width: `${contentWidth.value}px`,
  height: `${props.size}px`,
  objectFit: 'contain',
  objectPosition: 'center',
  filter: iconMeta.value?.filter,
  transform: contentOffsetX.value === 0 && contentOffsetY.value === 0
    ? undefined
    : `translate(${contentOffsetX.value}px, ${contentOffsetY.value}px)`,
}))

const maskStyle = computed<CSSProperties>(() => {
  if (!iconMeta.value || iconMeta.value.renderMode !== 'mask') {
    return {}
  }
  return {
    width: `${contentWidth.value}px`,
    height: `${props.size}px`,
    transform: contentOffsetX.value === 0 && contentOffsetY.value === 0
      ? undefined
      : `translate(${contentOffsetX.value}px, ${contentOffsetY.value}px)`,
    backgroundColor: iconMeta.value.color ?? 'currentColor',
    maskImage: `url("${iconMeta.value.src}")`,
    maskRepeat: 'no-repeat',
    maskPosition: 'center',
    maskSize: 'contain',
    WebkitMaskImage: `url("${iconMeta.value.src}")`,
    WebkitMaskRepeat: 'no-repeat',
    WebkitMaskPosition: 'center',
    WebkitMaskSize: 'contain',
  }
})
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
      v-if="iconMeta?.renderMode === 'image'"
      :src="iconSrc"
      :alt="title || normalizedKind"
      class="block"
      :style="contentStyle"
      draggable="false"
    >
    <span
      v-else
      class="block h-full w-full"
      :style="maskStyle"
    />
  </span>
</template>
