<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, reactive, ref, watch } from 'vue'
import { NAlert, NButton, NEmpty, NIcon, NInput, NSpin, NTooltip, useMessage } from 'naive-ui'
import { ArrowBackOutline, ArrowForwardOutline, ArrowUpOutline, CopyOutline, DocumentTextOutline, FolderOpenOutline, RefreshOutline, SearchOutline } from '@vicons/ionicons5'
import { ListSSHConnections } from '../../wailsjs/go/main/App'
import type { ParameterSpec, RemotePathBrowseResult, RemotePathEntry, SSHConnection } from '@/types/workbench'
import { browseRemotePath } from '@/utils/remotePathApi'
import WorkbenchContextMenu from './WorkbenchContextMenu.vue'

const props = defineProps<{
  show: boolean
  connectionId: string
  param: ParameterSpec | null
  target: 'file' | 'directory' | 'fileOrDirectory'
  initialPath: string
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  confirm: [selections: Array<{ path: string, kind: 'file' | 'directory' }>]
}>()

const message = useMessage()
const loading = ref(false)
const errorText = ref('')
const browseInput = ref('')
const browseResult = ref<RemotePathBrowseResult | null>(null)
const selectedEntryPaths = ref<string[]>([])
const selectionAnchorPath = ref('')
const searchQuery = ref('')
const sshConnections = ref<SSHConnection[]>([])
const historyStack = ref<string[]>([])
const historyIndex = ref(-1)
const listRef = ref<HTMLElement | null>(null)
const pathInputWrapRef = ref<HTMLElement | null>(null)
const searchInputWrapRef = ref<HTMLElement | null>(null)
const pathOverflow = ref(false)
const activeBrowseRequestId = ref(0)
const entryMenuShow = ref(false)
const entryMenuX = ref(0)
const entryMenuY = ref(0)
const entryMenuEntry = ref<RemotePathEntry | null>(null)
const dragState = reactive({
  active: false,
  startX: 0,
  startY: 0,
  currentX: 0,
  currentY: 0,
  additive: false,
  baseSelection: [] as string[],
})

const currentPath = computed(() => browseResult.value?.currentPath ?? '')
const entries = computed(() => browseResult.value?.entries ?? [])
const filteredEntries = computed(() => {
  const keyword = searchQuery.value.trim().toLowerCase()
  if (!keyword) {
    return entries.value
  }
  return entries.value.filter((entry) =>
    entry.name.toLowerCase().includes(keyword) || entry.path.toLowerCase().includes(keyword),
  )
})
const connectionDisplay = computed(() => {
  const matched = sshConnections.value.find((conn) => conn.id === props.connectionId)
  if (!matched) {
    return '远程连接'
  }
  return matched.name?.trim() || matched.host?.trim() || '远程连接'
})
const selectedEntries = computed<RemotePathEntry[]>(() => {
  const selectedSet = new Set(selectedEntryPaths.value)
  return entries.value.filter((entry) => selectedSet.has(entry.path))
})
const selectedEntry = computed<RemotePathEntry | null>(() => selectedEntries.value[0] ?? null)
const effectiveSelections = computed<Array<{ path: string, kind: 'file' | 'directory' }>>(() => {
  if (selectedEntries.value.length > 0) {
    return selectedEntries.value.map((entry) => ({
      path: entry.path,
      kind: entry.kind === 'directory' ? 'directory' : 'file',
    }))
  }
  if (!currentPath.value || props.target === 'file') {
    return []
  }
  return [{ path: currentPath.value, kind: 'directory' }]
})
const primaryButtonLabel = computed(() => {
  if (selectedEntries.value.length > 1) {
    return `选择 ${selectedEntries.value.length} 项`
  }
  if (selectedEntries.value.length === 1 && selectedEntry.value) {
    return selectedEntry.value.kind === 'directory' ? '选择所选目录' : '选择所选文件'
  }
  if (currentPath.value && props.target !== 'file') {
    return '选择当前目录'
  }
  return '请选择可用项'
})
const selectionRectStyle = computed<Record<string, string>>(() => {
  if (!dragState.active) {
    return { display: 'none', left: '0px', top: '0px', width: '0px', height: '0px' }
  }
  const left = Math.min(dragState.startX, dragState.currentX)
  const top = Math.min(dragState.startY, dragState.currentY)
  const width = Math.abs(dragState.currentX - dragState.startX)
  const height = Math.abs(dragState.currentY - dragState.startY)
  return {
    display: 'block',
    left: `${left}px`,
    top: `${top}px`,
    width: `${width}px`,
    height: `${height}px`,
  }
})
const canGoBack = computed(() => historyIndex.value > 0)
const canGoForward = computed(() => historyIndex.value >= 0 && historyIndex.value < historyStack.value.length - 1)

function closeModal() {
  emit('update:show', false)
}

function uniquePaths(paths: string[]): string[] {
  return [...new Set(paths)]
}

function setSelectedPaths(paths: string[], anchor?: string) {
  selectedEntryPaths.value = uniquePaths(paths)
  if (anchor !== undefined) {
    selectionAnchorPath.value = anchor
  } else if (selectedEntryPaths.value.length === 0) {
    selectionAnchorPath.value = ''
  }
}

function updatePathInputViewport() {
  void nextTick(() => {
    const input = pathInputWrapRef.value?.querySelector('input') as HTMLInputElement | null
    if (!input) {
      pathOverflow.value = false
      return
    }
    const end = input.value.length
    input.focus()
    input.setSelectionRange(end, end)
    input.scrollLeft = input.scrollWidth
    pathOverflow.value = input.scrollWidth > input.clientWidth + 1
  })
}

function handleKeydown(event: KeyboardEvent) {
  if (!props.show) {
    return
  }
  const target = event.target as HTMLElement | null
  const isTypingTarget = target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement
  if (event.key === 'Escape') {
    event.preventDefault()
    closeEntryContextMenu()
    closeModal()
    return
  }
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'f') {
    event.preventDefault()
    const searchInput = searchInputWrapRef.value?.querySelector('input') as HTMLInputElement | null
    searchInput?.focus()
    searchInput?.select()
    return
  }
  if (!isTypingTarget && (event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'a') {
    event.preventDefault()
    setSelectedPaths(filteredEntries.value.map((entry) => entry.path))
  }
}

function resetState() {
  activeBrowseRequestId.value += 1
  loading.value = false
  errorText.value = ''
  browseInput.value = ''
  browseResult.value = null
  selectedEntryPaths.value = []
  selectionAnchorPath.value = ''
  searchQuery.value = ''
  historyStack.value = []
  historyIndex.value = -1
  pathOverflow.value = false
  entryMenuShow.value = false
  entryMenuEntry.value = null
  dragState.active = false
  dragState.baseSelection = []
}

function pushHistory(path: string) {
  const normalized = path.trim()
  if (!normalized) {
    return
  }
  if (historyStack.value[historyIndex.value] === normalized) {
    return
  }
  historyStack.value = historyStack.value.slice(0, historyIndex.value + 1)
  historyStack.value.push(normalized)
  historyIndex.value = historyStack.value.length - 1
}

async function ensureConnectionMeta() {
  if (sshConnections.value.length > 0) {
    return
  }
  try {
    sshConnections.value = await ListSSHConnections()
  } catch {
    sshConnections.value = []
  }
}

async function loadDirectory(requestedPath: string, options?: { preserveHistory?: boolean }) {
  if (!props.connectionId) {
    errorText.value = '请先选择远程环境'
    return
  }

  const requestId = activeBrowseRequestId.value + 1
  activeBrowseRequestId.value = requestId
  loading.value = true
  errorText.value = ''
  selectedEntryPaths.value = []
  try {
    const result = await browseRemotePath(props.connectionId, requestedPath.trim())
    if (requestId !== activeBrowseRequestId.value || !props.show) {
      return
    }
    browseResult.value = result
    browseInput.value = result.currentPath
    if (!options?.preserveHistory) {
      pushHistory(result.currentPath)
    }
    updatePathInputViewport()
  } catch (error) {
    if (requestId !== activeBrowseRequestId.value || !props.show) {
      return
    }
    errorText.value = error instanceof Error ? error.message : String(error)
  } finally {
    if (requestId === activeBrowseRequestId.value) {
      loading.value = false
    }
  }
}

function openInitialPath() {
  const requestedPath = props.initialPath.trim()
  void loadDirectory(requestedPath)
}

function goBack() {
  if (!canGoBack.value) {
    return
  }
  historyIndex.value -= 1
  void loadDirectory(historyStack.value[historyIndex.value], { preserveHistory: true })
}

function goForward() {
  if (!canGoForward.value) {
    return
  }
  historyIndex.value += 1
  void loadDirectory(historyStack.value[historyIndex.value], { preserveHistory: true })
}

function goUp() {
  const current = currentPath.value
  if (!current || current === '/') {
    return
  }
  const segments = current.split('/').filter(Boolean)
  const parent = segments.length <= 1 ? '/' : `/${segments.slice(0, -1).join('/')}`
  void loadDirectory(parent)
}

function refreshCurrentDirectory() {
  if (!currentPath.value) {
    return
  }
  void loadDirectory(currentPath.value, { preserveHistory: true })
}

function openTypedPath() {
  void loadDirectory(browseInput.value)
}

function selectRangeTo(path: string) {
  const anchor = selectionAnchorPath.value || path
  const orderedPaths = filteredEntries.value.map((entry) => entry.path)
  const start = orderedPaths.indexOf(anchor)
  const end = orderedPaths.indexOf(path)
  if (start < 0 || end < 0) {
    setSelectedPaths([path], path)
    return
  }
  const [from, to] = start <= end ? [start, end] : [end, start]
  setSelectedPaths(orderedPaths.slice(from, to + 1), anchor)
}

function handleEntryClick(event: MouseEvent, entry: RemotePathEntry) {
  if (event.shiftKey) {
    selectRangeTo(entry.path)
    return
  }
  if (event.ctrlKey || event.metaKey) {
    const exists = selectedEntryPaths.value.includes(entry.path)
    if (exists) {
      setSelectedPaths(selectedEntryPaths.value.filter((path) => path !== entry.path), entry.path)
    } else {
      setSelectedPaths([...selectedEntryPaths.value, entry.path], entry.path)
    }
    return
  }
  setSelectedPaths([entry.path], entry.path)
}

function clearSelection() {
  setSelectedPaths([])
}

function openEntryContextMenu(event: MouseEvent, entry: RemotePathEntry) {
  event.preventDefault()
  entryMenuX.value = event.clientX
  entryMenuY.value = event.clientY
  entryMenuEntry.value = entry
  entryMenuShow.value = true
  if (!selectedEntryPaths.value.includes(entry.path)) {
    setSelectedPaths([entry.path], entry.path)
  }
}

function closeEntryContextMenu() {
  entryMenuShow.value = false
  entryMenuEntry.value = null
}

async function handleEntryMenuSelect(key: string) {
  const entry = entryMenuEntry.value
  closeEntryContextMenu()
  if (!entry) {
    return
  }
  if (key === 'copy-path') {
    try {
      await navigator.clipboard.writeText(entry.path)
      message.success('已复制路径')
    } catch {
      message.error('复制路径失败')
    }
  }
}

function clampToContainer(event: PointerEvent) {
  const rect = listRef.value?.getBoundingClientRect()
  if (!rect) {
    return { x: 0, y: 0 }
  }
  return {
    x: Math.max(0, Math.min(event.clientX - rect.left, rect.width)),
    y: Math.max(0, Math.min(event.clientY - rect.top, rect.height)),
  }
}

function collectDraggedPaths() {
  const container = listRef.value
  if (!container) {
    return
  }
  const left = Math.min(dragState.startX, dragState.currentX)
  const top = Math.min(dragState.startY, dragState.currentY)
  const right = Math.max(dragState.startX, dragState.currentX)
  const bottom = Math.max(dragState.startY, dragState.currentY)
  const hitPaths = Array.from(container.querySelectorAll<HTMLElement>('[data-entry-path]'))
    .filter((element) => {
      const rect = element.getBoundingClientRect()
      const host = container.getBoundingClientRect()
      const relLeft = rect.left - host.left + container.scrollLeft
      const relTop = rect.top - host.top + container.scrollTop
      const relRight = relLeft + rect.width
      const relBottom = relTop + rect.height
      return !(relRight < left || relLeft > right || relBottom < top || relTop > bottom)
    })
    .map((element) => element.dataset.entryPath || '')
    .filter((path) => path.length > 0)

  const next = dragState.additive
    ? uniquePaths([...dragState.baseSelection, ...hitPaths])
    : uniquePaths(hitPaths)
  setSelectedPaths(next)
}

function handleListPointerDown(event: PointerEvent) {
  if (event.button !== 0 || !listRef.value) {
    return
  }
  if ((event.target as HTMLElement).closest('[data-entry-path]')) {
    return
  }
  const point = clampToContainer(event)
  dragState.active = true
  dragState.startX = point.x + listRef.value.scrollLeft
  dragState.startY = point.y + listRef.value.scrollTop
  dragState.currentX = dragState.startX
  dragState.currentY = dragState.startY
  dragState.additive = event.ctrlKey || event.metaKey
  dragState.baseSelection = dragState.additive ? [...selectedEntryPaths.value] : []
  if (!dragState.additive) {
    clearSelection()
  }
  const move = (moveEvent: PointerEvent) => {
    if (!listRef.value) {
      return
    }
    const nextPoint = clampToContainer(moveEvent)
    dragState.currentX = nextPoint.x + listRef.value.scrollLeft
    dragState.currentY = nextPoint.y + listRef.value.scrollTop
    collectDraggedPaths()
  }
  const up = () => {
    dragState.active = false
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', up)
    window.removeEventListener('pointercancel', up)
  }
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', up)
  window.addEventListener('pointercancel', up)
  closeEntryContextMenu()
}

function handleEntryOpen(entry: RemotePathEntry) {
  if (entry.kind === 'directory') {
    void loadDirectory(entry.path)
    return
  }
  emit('confirm', [{ path: entry.path, kind: 'file' }])
  closeModal()
}

function confirmPrimarySelection() {
  if (effectiveSelections.value.length === 0) {
    return
  }
  emit('confirm', effectiveSelections.value)
  closeModal()
}

watch(
  () => props.show,
  (value) => {
    if (value) {
      void ensureConnectionMeta()
      openInitialPath()
      return
    }
    resetState()
  },
)

watch(browseInput, () => {
  updatePathInputViewport()
})

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="show"
        class="fixed inset-0 z-50 bg-[rgb(var(--color-overlay-rgb)/0.42)] backdrop-blur-sm"
        @click="closeModal"
      />
    </Transition>
    <Transition
      name="fade-scale"
      appear
    >
      <div
        v-if="show"
        class="fixed inset-0 z-50 flex items-start justify-center px-4 py-[8vh] pointer-events-none"
      >
        <div
          class="remote-path-picker-modal surface-dialog pointer-events-auto flex w-full max-w-[780px] flex-col overflow-hidden rounded-2xl"
          @click.stop
        >
          <div class="remote-path-picker-header surface-divider flex items-start justify-between gap-3 border-b px-5 py-3">
            <div class="min-w-0">
              <div class="text-sm font-semibold text-[rgb(var(--color-fg-base)/0.98)]">
                远程路径选择器
              </div>
              <div class="mt-1 text-xs text-[rgb(var(--color-fg-muted)/0.92)]">
                {{ connectionDisplay }}
              </div>
            </div>
            <NButton
              text
              size="tiny"
              @click="closeModal"
            >
              ESC 关闭
            </NButton>
          </div>

          <div class="remote-path-picker-body">
            <div class="remote-path-picker-toolbar">
              <div class="flex items-center gap-2">
                <NButton
                  size="small"
                  secondary
                  circle
                  :disabled="!canGoBack"
                  @click="goBack"
                >
                  <template #icon>
                    <NIcon :component="ArrowBackOutline" />
                  </template>
                </NButton>
                <NButton
                  size="small"
                  secondary
                  circle
                  :disabled="!canGoForward"
                  @click="goForward"
                >
                  <template #icon>
                    <NIcon :component="ArrowForwardOutline" />
                  </template>
                </NButton>
                <NButton
                  size="small"
                  secondary
                  circle
                  :disabled="!currentPath"
                  @click="goUp"
                >
                  <template #icon>
                    <NIcon :component="ArrowUpOutline" />
                  </template>
                </NButton>
                <NButton
                  size="small"
                  secondary
                  circle
                  :disabled="!currentPath"
                  @click="refreshCurrentDirectory"
                >
                  <template #icon>
                    <NIcon :component="RefreshOutline" />
                  </template>
                </NButton>
              </div>
              <div class="min-w-0 flex-1">
                <NTooltip
                  placement="top-start"
                  :disabled="!pathOverflow || !browseInput"
                >
                  <template #trigger>
                    <div ref="pathInputWrapRef">
                      <NInput
                        v-model:value="browseInput"
                        placeholder="输入远端路径，找不到会回到 ~"
                        @keydown.enter.prevent="openTypedPath"
                      />
                    </div>
                  </template>
                  <div class="max-w-[560px] break-all">
                    {{ browseInput }}
                  </div>
                </NTooltip>
              </div>
              <div class="w-[180px] shrink-0">
                <div ref="searchInputWrapRef">
                  <NInput
                    v-model:value="searchQuery"
                    clearable
                    placeholder="筛选当前目录"
                  >
                    <template #prefix>
                      <NIcon :component="SearchOutline" />
                    </template>
                  </NInput>
                </div>
              </div>
              <NAlert
                v-if="browseResult?.message"
                type="warning"
                :show-icon="false"
                size="small"
                class="remote-path-picker-message"
              >
                {{ browseResult.message }}
              </NAlert>

              <NAlert
                v-if="errorText"
                type="error"
                :show-icon="false"
                size="small"
                class="remote-path-picker-message"
              >
                {{ errorText }}
              </NAlert>
            </div>

            <div class="remote-path-picker-list-shell rounded-xl border border-[rgb(var(--color-border-subtle)/0.56)] bg-[rgb(var(--color-bg-elevated)/0.88)]">
              <div class="remote-path-picker-list-header surface-muted-divider border-b px-3 py-2 text-[11px] text-[rgb(var(--color-fg-muted)/0.9)]">
                <span>名称</span>
                <span class="text-right">类型</span>
              </div>
              <div
                ref="listRef"
                class="remote-path-picker-list"
                @pointerdown="handleListPointerDown"
              >
                <NEmpty
                  v-if="!loading && filteredEntries.length === 0"
                  :description="searchQuery ? '没有匹配的项目' : '当前目录没有可显示的项目'"
                  class="py-8"
                />
                <button
                  v-for="entry in filteredEntries"
                  :key="entry.path"
                  type="button"
                  :data-entry-path="entry.path"
                  class="remote-path-entry grid w-full grid-cols-[minmax(0,1fr)_52px] items-center gap-3 px-3 py-1 text-left"
                  :class="selectedEntryPaths.includes(entry.path) ? 'remote-path-entry-active' : 'hover:bg-[rgb(var(--color-fg-base)/0.05)]'"
                  @click="handleEntryClick($event, entry)"
                  @dblclick="handleEntryOpen(entry)"
                  @contextmenu="openEntryContextMenu($event, entry)"
                >
                  <div class="flex min-w-0 items-center gap-3">
                    <NIcon
                      :component="entry.kind === 'directory' ? FolderOpenOutline : DocumentTextOutline"
                      :color="entry.kind === 'directory' ? 'rgb(var(--color-brand-primary) / 1)' : 'rgb(var(--color-fg-base) / 1)'"
                      size="16"
                      class="shrink-0"
                    />
                    <div class="truncate text-sm text-[rgb(var(--color-fg-base)/0.98)]">
                      {{ entry.name }}
                    </div>
                  </div>
                  <div class="flex items-center justify-end">
                    <span class="text-xs text-[rgb(var(--color-fg-muted)/0.9)]">
                      {{ entry.kind === 'directory' ? '目录' : '文件' }}
                    </span>
                  </div>
                </button>
                <div
                  v-if="dragState.active"
                  class="remote-path-picker-selection-rect"
                  :style="selectionRectStyle"
                />
              </div>
              <div
                v-if="loading"
                class="remote-path-picker-loading"
              >
                <NSpin size="small" />
              </div>
            </div>
          </div>

          <div class="remote-path-picker-footer surface-muted-divider flex items-center justify-between gap-3 border-t px-5 py-3">
            <div />
            <div class="flex items-center gap-2">
              <NButton @click="closeModal">
                取消
              </NButton>
              <NButton
                type="primary"
                :disabled="effectiveSelections.length === 0"
                @click="confirmPrimarySelection"
              >
                {{ primaryButtonLabel }}
              </NButton>
            </div>
          </div>
        </div>
      </div>
    </Transition>
    <WorkbenchContextMenu
      :show="entryMenuShow"
      :x="entryMenuX"
      :y="entryMenuY"
      :title="entryMenuEntry?.name ?? ''"
      :subtitle="entryMenuEntry?.path ?? ''"
      :items="[
        { key: 'copy-path', label: '复制路径', icon: CopyOutline },
      ]"
      @select="handleEntryMenuSelect"
      @close="closeEntryContextMenu"
    />
  </Teleport>
</template>

<style scoped>
.remote-path-picker-modal {
  height: min(82vh, 620px);
}

.remote-path-picker-body {
  display: grid;
  min-height: 0;
  flex: 1;
  grid-template-rows: auto minmax(0, 1fr);
  gap: 12px;
  padding: 16px 20px;
}

.remote-path-picker-toolbar {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
}

.remote-path-picker-message {
  grid-column: 1 / -1;
}

.remote-path-picker-list-shell {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.remote-path-picker-list-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
}

.remote-path-picker-list {
  flex: 1 1 auto;
  min-height: 0;
  position: relative;
  overflow-y: auto;
  padding: 10px 16px;
}

.remote-path-picker-loading {
  position: absolute;
  inset: 33px 0 0 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgb(var(--color-overlay-rgb) / 0.28);
  pointer-events: none;
}

.remote-path-entry {
  border-radius: 10px;
  user-select: none;
  transition:
    background-color 0.16s var(--ease-out-soft),
    border-color 0.16s var(--ease-out-soft);
}

.remote-path-entry-active {
  background-color: rgb(var(--color-brand-primary) / 0.12);
  box-shadow: inset 0 0 0 1px rgb(var(--color-brand-primary) / 0.32);
}

.remote-path-picker-selection-rect {
  position: absolute;
  border: 1px solid rgb(var(--color-brand-primary) / 0.72);
  background: rgb(var(--color-brand-primary) / 0.14);
  pointer-events: none;
}

@media (max-width: 860px) {
  .remote-path-picker-modal {
    max-width: min(92vw, 780px);
  }

  .remote-path-picker-toolbar {
    grid-template-columns: auto minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .remote-path-picker-toolbar {
    grid-template-columns: 1fr;
  }
}
</style>
