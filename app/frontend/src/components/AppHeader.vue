<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { NButton, NIcon, NInput, NTooltip } from 'naive-ui'
import { ArrowDownCircleOutline, Close, CopyOutline, HelpCircle, Remove, Search, SquareOutline } from '@vicons/ionicons5'
import { useDownloadStore } from '@/stores/downloads'
import { useExecutionStore } from '@/stores/execution'
import { useWorkbenchStore } from '@/stores/workbench'
import { useWorkspaceStore } from '@/stores/workspace'
import { Quit, WindowMinimise, WindowToggleMaximise } from '../../wailsjs/runtime'
import { GetCurrentWindowState, PersistCurrentWindowState } from '../../wailsjs/go/main/App'

interface WindowSnapshot {
  width: number
  height: number
  x: number
  y: number
  maximised: boolean
  fullscreen: boolean
}

const STARTUP_COOLDOWN_MS = 2000
const SAVE_DEBOUNCE_MS = 400
const POLL_INTERVAL_MS = 750

const execution = useExecutionStore()
const downloads = useDownloadStore()
const workbench = useWorkbenchStore()
const workspace = useWorkspaceStore()

const runningCount = computed(() => execution.tasks.filter((t) => t.status === 'running').length)
const downloadCount = computed(() => downloads.activeCount)
const hasDownloadTasks = computed(() => downloadCount.value > 0)
const isMaximised = ref(false)
const platform = computed(() => workbench.bootstrap?.platform.split('/')[0] ?? 'windows')
const isWindows = computed(() => platform.value === 'windows')
const isMac = computed(() => platform.value === 'darwin')
const appTitle = computed(() => workbench.bootstrap?.appTitle ?? '火蜥蜴工具箱 Desktop')
const brandTitle = computed(() => appTitle.value.replace(/\s+Desktop$/, ''))

let trackingEnabled = false
let saveTimer: number | null = null
let pollTimer: number | null = null
let startupTimer: number | null = null
let lastObservedKey = ''
let lastPersistedKey = ''

function delay(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms))
}

function snapshotKey(snapshot: WindowSnapshot) {
  return [
    snapshot.width,
    snapshot.height,
    snapshot.x,
    snapshot.y,
    snapshot.maximised ? 1 : 0,
    snapshot.fullscreen ? 1 : 0,
  ].join(':')
}

function applyWindowMode(snapshot: WindowSnapshot) {
  isMaximised.value = snapshot.maximised
}

async function readWindowSnapshot() {
  try {
    const snapshot = await GetCurrentWindowState()
    applyWindowMode(snapshot)
    return snapshot
  } catch {
    isMaximised.value = false
    return null
  }
}

async function persistWindowStateNow() {
  const snapshot = await readWindowSnapshot()
  if (!snapshot) return

  const currentKey = snapshotKey(snapshot)
  lastObservedKey = currentKey
  if (currentKey === lastPersistedKey) return

  try {
    await PersistCurrentWindowState()
    lastPersistedKey = currentKey
  } catch {
    // Ignore transient shutdown / OS window manager races.
  }
}

function scheduleSaveWindowState() {
  if (!trackingEnabled) return
  if (saveTimer) window.clearTimeout(saveTimer)
  saveTimer = window.setTimeout(() => {
    saveTimer = null
    void persistWindowStateNow()
  }, SAVE_DEBOUNCE_MS)
}

async function syncWindowState() {
  const snapshot = await readWindowSnapshot()
  if (!snapshot) return

  const currentKey = snapshotKey(snapshot)
  if (!trackingEnabled) {
    lastObservedKey = currentKey
    return
  }

  if (currentKey !== lastObservedKey) {
    lastObservedKey = currentKey
    scheduleSaveWindowState()
  }
}

function openSearch() {
  workspace.showSearch = true
}

function openHotkeyHelp() {
  workspace.showHotkeyHelp = true
}

function openDownloadDrawer() {
  downloads.openDrawer()
}

function minimiseWindow() {
  WindowMinimise()
}

async function toggleMaximise() {
  WindowToggleMaximise()
  await delay(150)
  void syncWindowState()
}

async function closeWindow() {
  await persistWindowStateNow()
  Quit()
}

function handleTitlebarDoubleClick() {
  if (!isWindows.value) return
  void toggleMaximise()
}

function handleWindowResize() {
  scheduleSaveWindowState()
}

onMounted(() => {
  void downloads.hydrate()
  void syncWindowState()
  window.addEventListener('resize', handleWindowResize)
  pollTimer = window.setInterval(() => {
    void syncWindowState()
  }, POLL_INTERVAL_MS)
  startupTimer = window.setTimeout(() => {
    trackingEnabled = true
    void syncWindowState()
  }, STARTUP_COOLDOWN_MS)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleWindowResize)
  if (saveTimer) window.clearTimeout(saveTimer)
  if (pollTimer) window.clearInterval(pollTimer)
  if (startupTimer) window.clearTimeout(startupTimer)
})
</script>

<template>
  <header
    class="titlebar-surface wails-drag relative flex shrink-0 items-center justify-between border-b border-white/15 px-4"
    :class="isMac ? 'h-14 pt-1' : 'h-12'"
    @dblclick="handleTitlebarDoubleClick"
  >
    <div
      class="flex items-center gap-x-3"
      :class="isMac ? 'pl-[68px]' : ''"
    >
      <div class="wails-no-drag flex h-7 w-7 items-center justify-center rounded-lg border border-white/10 bg-dracula-cyan/10 text-sm font-bold text-dracula-cyan shadow-[0_0_0_1px_rgb(var(--color-brand-primary)_/_0.08)]">
        火
      </div>
      <div class="flex items-baseline gap-x-1.5 min-w-0">
        <span class="truncate text-sm font-semibold tracking-wide text-dracula-text">{{ brandTitle }}</span>
        <span class="shrink-0 text-[10px] uppercase tracking-[0.24em] text-dracula-soft/60">Desktop</span>
      </div>
    </div>

    <div class="wails-no-drag absolute left-1/2 -translate-x-1/2">
      <NInput
        placeholder="搜索工具..."
        size="small"
        class="w-[min(42vw,24rem)]"
        @focus="openSearch"
      >
        <template #prefix>
          <NIcon :component="Search" />
        </template>
        <template #suffix>
          <span class="rounded bg-white/8 px-1.5 py-px text-[10px] font-medium text-dracula-soft">
            Ctrl+P
          </span>
        </template>
      </NInput>
    </div>

    <div
      class="flex items-center gap-x-1"
      :class="isWindows ? '' : 'pr-1'"
    >
      <NTooltip placement="bottom-end">
        <template #trigger>
          <NButton
            quaternary
            circle
            size="small"
            class="wails-no-drag relative text-dracula-soft hover:text-dracula-text"
            @click="openDownloadDrawer"
          >
            <template #icon>
              <div class="relative flex h-5 w-5 items-center justify-center overflow-visible">
                <NIcon
                  class="relative z-10"
                  :component="ArrowDownCircleOutline"
                  size="18"
                  :color="hasDownloadTasks ? 'rgb(var(--color-mode-remote) / 1)' : undefined"
                />
                <span
                  v-if="hasDownloadTasks"
                  class="absolute -right-1 -top-1 z-20 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-dracula-pink px-[3px] text-[8px] font-bold leading-none text-white shadow-[0_0_0_1px_rgb(var(--color-bg-panel)_/_0.95)]"
                >
                  {{ downloadCount > 9 ? '9+' : downloadCount }}
                </span>
              </div>
            </template>
          </NButton>
        </template>
        下载任务
      </NTooltip>

      <NTooltip placement="bottom-end">
        <template #trigger>
          <NButton
            quaternary
            circle
            size="small"
            class="wails-no-drag text-dracula-soft hover:text-dracula-text"
            @click="openHotkeyHelp"
          >
            <template #icon>
              <NIcon
                :component="HelpCircle"
                size="18"
              />
            </template>
          </NButton>
        </template>
        快捷键帮助 (F1)
      </NTooltip>

      <span
        v-if="runningCount > 0"
        class="wails-no-drag ml-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-dracula-red px-1.5 text-[10px] font-bold text-white"
      >
        {{ runningCount }}
      </span>

      <template v-if="isWindows">
        <button
          class="wails-no-drag ui-interactive ml-2 flex h-8 w-10 items-center justify-center rounded-md text-dracula-soft hover:bg-dracula-cyan/10 hover:text-dracula-cyan active:bg-dracula-cyan/15"
          type="button"
          aria-label="最小化"
          @click="minimiseWindow"
        >
          <NIcon
            :component="Remove"
            size="16"
          />
        </button>
        <button
          class="wails-no-drag ui-interactive flex h-8 w-10 items-center justify-center rounded-md text-dracula-soft hover:bg-dracula-cyan/10 hover:text-dracula-cyan active:bg-dracula-cyan/15"
          type="button"
          :aria-label="isMaximised ? '还原窗口' : '最大化窗口'"
          @click="toggleMaximise"
        >
          <NIcon
            :component="isMaximised ? CopyOutline : SquareOutline"
            size="14"
          />
        </button>
        <button
          class="wails-no-drag ui-interactive flex h-8 w-10 items-center justify-center rounded-md text-dracula-soft hover:bg-red-500/90 hover:text-white"
          type="button"
          aria-label="关闭窗口"
          @click="closeWindow"
        >
          <NIcon
            :component="Close"
            size="16"
          />
        </button>
      </template>
    </div>
  </header>
</template>
