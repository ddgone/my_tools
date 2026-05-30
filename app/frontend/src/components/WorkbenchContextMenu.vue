<script setup lang="ts">
import { computed, onMounted, onUnmounted, type Component, type CSSProperties, type PropType } from 'vue'
import { NIcon } from 'naive-ui'

type WorkbenchContextMenuItem = {
  key: string
  label?: string
  icon?: Component
  hint?: string
  disabled?: boolean
  danger?: boolean
  type?: 'item' | 'divider'
}

const props = defineProps({
  show: {
    type: Boolean,
    default: false,
  },
  x: {
    type: Number,
    default: 0,
  },
  y: {
    type: Number,
    default: 0,
  },
  width: {
    type: Number,
    default: 224,
  },
  title: {
    type: String,
    default: '',
  },
  subtitle: {
    type: String,
    default: '',
  },
  items: {
    type: Array as PropType<WorkbenchContextMenuItem[]>,
    default: () => [],
  },
})

const emit = defineEmits<{
  select: [key: string]
  close: []
}>()

const VIEWPORT_GAP = 10
const CURSOR_OFFSET = 6
const ITEM_HEIGHT = 34
const DIVIDER_HEIGHT = 9
const HEADER_HEIGHT = 56

const menuStyle = computed<CSSProperties>(() => {
  const viewportWidth = typeof window === 'undefined' ? 1440 : window.innerWidth
  const viewportHeight = typeof window === 'undefined' ? 900 : window.innerHeight
  const estimatedHeight = props.items.reduce((height, item) => {
    return height + (item.type === 'divider' ? DIVIDER_HEIGHT : ITEM_HEIGHT)
  }, props.title || props.subtitle ? HEADER_HEIGHT : 10)

  const left = Math.min(props.x + CURSOR_OFFSET, viewportWidth - props.width - VIEWPORT_GAP)
  const top = Math.min(props.y + CURSOR_OFFSET, viewportHeight - estimatedHeight - VIEWPORT_GAP)

  return {
    left: `${Math.max(VIEWPORT_GAP, left)}px`,
    top: `${Math.max(VIEWPORT_GAP, top)}px`,
    width: `${props.width}px`,
  }
})

function closeMenu() {
  emit('close')
}

function handleItemSelect(item: WorkbenchContextMenuItem) {
  if (item.type === 'divider' || item.disabled) return
  emit('select', item.key)
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (props.show && event.key === 'Escape') {
    closeMenu()
  }
}

onMounted(() => {
  document.addEventListener('keydown', onDocumentKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onDocumentKeydown)
})
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-[110]"
      @click="closeMenu"
      @contextmenu.prevent="closeMenu"
    >
      <div
        class="workbench-popover-surface fixed overflow-hidden"
        :style="menuStyle"
        @click.stop
        @contextmenu.prevent
      >
        <div
          v-if="title || subtitle"
          class="border-b border-black/10 px-3 py-2.5"
        >
          <div
            v-if="title"
            class="truncate text-[12px] font-semibold text-[var(--workbench-tooltip-text)]"
          >
            {{ title }}
          </div>
          <div
            v-if="subtitle"
            class="mt-0.5 truncate text-[11px] text-[color:rgba(26,26,46,0.6)]"
          >
            {{ subtitle }}
          </div>
        </div>

        <div class="p-1.5">
          <div
            v-for="item in items"
            :key="item.key"
          >
            <div
              v-if="item.type === 'divider'"
              class="workbench-menu-divider my-1"
            />

            <button
              v-else
              type="button"
              class="workbench-menu-item flex w-full items-center gap-x-2 rounded-md px-2.5 py-2 text-left text-[12px]"
              :class="[
                item.disabled ? 'cursor-not-allowed opacity-45' : 'cursor-pointer',
                item.danger ? 'text-rose-700' : 'text-[var(--workbench-tooltip-text)]',
              ]"
              :disabled="item.disabled"
              @click="handleItemSelect(item)"
            >
              <span class="flex h-4 w-4 shrink-0 items-center justify-center">
                <NIcon
                  v-if="item.icon"
                  :component="item.icon"
                  size="14"
                />
              </span>

              <span class="min-w-0 flex-1 truncate">
                {{ item.label }}
              </span>

              <span
                v-if="item.hint"
                class="shrink-0 text-[10px] uppercase tracking-[0.08em] text-[color:rgba(26,26,46,0.5)]"
              >
                {{ item.hint }}
              </span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
