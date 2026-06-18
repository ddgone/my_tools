<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, nextTick, type CSSProperties } from 'vue'
import { NInput, NIcon, NList, NListItem, NScrollbar, NText, NTag } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { Search, ServerOutline, CodeSlash, LogoPython, Star, GlobeOutline, BookmarkSharp, CloudUploadOutline, BuildOutline } from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useDownloadStore } from '@/stores/downloads'
import { useGoEnvStore } from '@/stores/goenv'
import { useRustEnvStore } from '@/stores/rustenv'
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useWorkspaceStore } from '@/stores/workspace'
import { getBuiltinToolById, getBuiltinToolIcon } from '@/builtin/registry'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { useResizable } from '@/composables/useResizable'
import { useTruncationTooltip } from '@/composables/useTruncationTooltip'
import { ExportTool, OpenFileDialog, OpenPath } from '../../wailsjs/go/main/App'
import type { ParameterSpec, SSHConnection } from '@/types/workbench'
import ToolDetailPanel from './ToolDetailPanel.vue'
import ParameterPanel from './ParameterPanel.vue'
import ExecutionTerminal from './ExecutionTerminal.vue'
import SSHDetailPanel from './SSHDetailPanel.vue'
import BuiltinToolPanel from './BuiltinToolPanel.vue'
import ArtifactCenterPanel from './ArtifactCenterPanel.vue'
import ArtifactTaskSnapshotView from './ArtifactTaskSnapshotView.vue'
import WorkbenchContextMenu from './WorkbenchContextMenu.vue'
import { validateCliArgs } from '@/utils/cliArgs'
import { getExecutionTheme } from '@/utils/executionTheme'
import { findMissingRequiredParam } from '@/utils/toolParams'
import gsap from 'gsap'

const emit = defineEmits<{
  refreshSshList: []
}>()

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const downloads = useDownloadStore()
const goEnv = useGoEnvStore()
const rustEnv = useRustEnvStore()
const pythonEnv = usePythonEnvStore()
const workspace = useWorkspaceStore()
const message = useMessage()

const launching = ref(false)
const exporting = ref(false)
const downloadingResult = ref(false)
const exportProgressText = ref('')
const searchInput = ref('')
const activeSearchIndex = ref(0)
const contentRef = ref<HTMLElement | null>(null)
const tabBarRef = ref<HTMLElement | null>(null)
let disposeExportProgress: (() => void) | null = null
type UnifiedTabItem = {
  type: 'tool' | 'builtin' | 'ssh' | 'artifact'
  key: string
  label: string
  openedAt: number
  arrayIndex: number
  pinned: boolean
}

type WorkbenchMenuItem = {
  key: string
  label?: string
  hint?: string
  disabled?: boolean
  danger?: boolean
  type?: 'item' | 'divider'
}

const { tooltipText, tooltipX, tooltipY, tooltipShow, onEnter: onTooltipEnter, onLeave: onTooltipLeave } = useTruncationTooltip({ placement: 'bottom' })

const dragTabKey = ref('')
const dragPointerId = ref<number | null>(null)
const dragActivated = ref(false)
const dragStartX = ref(0)
const dragCurrentX = ref(0)
const dragTargetIndex = ref(-1)
const dragGroupKeys = ref<string[]>([])
const dragSnapshotRects = ref<Map<string, DOMRect>>(new Map())
const suppressClickAfterDrag = ref(false)

function tabButtonElements() {
  return Array.from(tabBarRef.value?.querySelectorAll<HTMLElement>('[data-tab-key]') ?? [])
}

function tabButtonElementByKey(key: string) {
  return tabButtonElements().find(element => element.dataset.tabKey === key) ?? null
}

function captureTabRects() {
  const rects = new Map<string, DOMRect>()
  for (const element of tabButtonElements()) {
    const key = element.dataset.tabKey
    if (key) {
      rects.set(key, element.getBoundingClientRect())
    }
  }
  return rects
}

function clearTabTransforms(exceptKey = '') {
  for (const element of tabButtonElements()) {
    const key = element.dataset.tabKey ?? ''
    if (exceptKey && key === exceptKey) continue
    gsap.killTweensOf(element)
    gsap.set(element, { clearProps: 'x,zIndex,boxShadow,scale' })
  }
}

async function animateTabLayout(prevRects: Map<string, DOMRect>, excludeKey = '') {
  await nextTick()
  clearTabTransforms()
  await nextTick()
  for (const element of tabButtonElements()) {
    const key = element.dataset.tabKey
    if (!key || key === excludeKey) continue
    const prevRect = prevRects.get(key)
    if (!prevRect) continue
    const nextRect = element.getBoundingClientRect()
    const deltaX = prevRect.left - nextRect.left
    if (Math.abs(deltaX) < 1) continue
    gsap.killTweensOf(element)
    gsap.fromTo(element, {
      x: deltaX,
    }, {
      x: 0,
      duration: 0.22,
      ease: 'power2.out',
      clearProps: 'x',
    })
  }
}

function dragScopedTabKeys() {
  const dragItem = dragTabKey.value ? resolveUnifiedTabByKey(dragTabKey.value) : null
  if (!dragItem) return []
  return (workspace.unifiedTabs as UnifiedTabItem[])
    .filter(item => item.pinned === dragItem.pinned)
    .map(item => item.key)
}

function computeDragTargetIndex() {
  const dragKey = dragTabKey.value
  const keys = dragGroupKeys.value
  const draggedRect = dragSnapshotRects.value.get(dragKey)
  if (!dragKey || keys.length === 0 || !draggedRect) return -1

  const dragDirection = Math.sign(dragCurrentX.value)
  const dragBoundary = dragDirection >= 0
    ? draggedRect.left + dragCurrentX.value + draggedRect.width
    : draggedRect.left + dragCurrentX.value
  let nextIndex = 0
  for (const key of keys) {
    if (key === dragKey) continue
    const rect = dragSnapshotRects.value.get(key)
    if (!rect) continue
    const center = rect.left + rect.width / 2
    if (center < dragBoundary) {
      nextIndex++
    }
  }
  return Math.max(0, Math.min(keys.length - 1, nextIndex))
}

function applyTabDragPreview() {
  const dragKey = dragTabKey.value
  const draggedElement = dragKey ? tabButtonElementByKey(dragKey) : null
  const draggedRect = dragSnapshotRects.value.get(dragKey)
  if (!dragKey || !draggedElement || !draggedRect) return

  const scopedKeys = dragGroupKeys.value
  const fromIndex = scopedKeys.indexOf(dragKey)
  if (fromIndex < 0) return

  const nextIndex = computeDragTargetIndex()
  dragTargetIndex.value = nextIndex

  const draggedWidth = draggedRect.width
  gsap.killTweensOf(draggedElement)
  gsap.set(draggedElement, {
    x: dragCurrentX.value,
    zIndex: 30,
    scale: 1.02,
    boxShadow: '0 12px 28px rgba(0, 0, 0, 0.35)',
  })

  for (const element of tabButtonElements()) {
    const key = element.dataset.tabKey ?? ''
    if (!key || key === dragKey) continue

    let offsetX = 0
    const index = scopedKeys.indexOf(key)
    if (index >= 0) {
      if (nextIndex > fromIndex && index > fromIndex && index <= nextIndex) {
        offsetX = -draggedWidth
      } else if (nextIndex < fromIndex && index >= nextIndex && index < fromIndex) {
        offsetX = draggedWidth
      }
    }

    gsap.killTweensOf(element)
    gsap.to(element, {
      x: offsetX,
      duration: 0.14,
      ease: 'power2.out',
      overwrite: true,
    })
  }
}

const showSearchModal = computed({
  get: () => workspace.showSearch,
  set: (v) => (workspace.showSearch = v),
})

const terminalMinHeight = 96
const topPanelMinHeight = 200
const workspaceBodyHeight = ref(Math.max(320, window.innerHeight - 180))
let layoutResizeObserver: ResizeObserver | null = null

const initialTerminalHeight = Math.max(160, Math.min(420, Math.floor(window.innerHeight * 0.3)))

const { size: terminalHeight, dividerProps: hDividerProps } = useResizable({
  axis: 'y',
  min: terminalMinHeight,
  max: () => Math.max(terminalMinHeight, workspaceBodyHeight.value - topPanelMinHeight),
  initial: initialTerminalHeight,
  reverse: true,
})

const activeToolId = computed(() => workspace.activeToolTab?.toolId ?? '')
const activeToolTabComputed = computed(() => workspace.activeToolTab)
const isTerminalVisible = computed(() => workspace.activeToolTerminalVisible)
const activeTargetIsRemote = computed(() => workspace.activeToolTab?.executionTarget === 'remote')
const activeToolKind = computed(() => toolById(activeToolId.value)?.kind ?? '')
const activeExecutionTheme = computed(() =>
  getExecutionTheme(activeToolKind.value, activeTargetIsRemote.value ? 'remote' : 'local'),
)
const workspaceAccent = computed(() => activeExecutionTheme.value.accent)
const workspaceAccentSoftBg = computed(() => activeExecutionTheme.value.accentSoftBg)
const workspaceAccentSoftStrongBg = computed(() => activeExecutionTheme.value.accentSoftStrongBg)
const workspaceActiveTabBackground = computed(() => activeExecutionTheme.value.activeTabBackground)
const workspaceDividerGradient = computed(() => activeExecutionTheme.value.dividerGradient)
const activeToolTerminalHeight = computed(() =>
  workspace.activeToolTerminalHeight ?? initialTerminalHeight,
)

function syncWorkspaceBodyHeight() {
  const contentHeight = contentRef.value?.clientHeight ?? 0
  const tabBarHeight = tabBarRef.value?.clientHeight ?? 0
  const nextHeight = contentHeight - tabBarHeight
  if (nextHeight > 0) {
    workspaceBodyHeight.value = nextHeight
  }
}

watch(
  () => workspace.activeToolTab?.toolId,
  () => {
    terminalHeight.value = activeToolTerminalHeight.value
  },
  { immediate: true },
)

watch(
  terminalHeight,
  (value) => {
    if (workspace.activeTabType !== 'tool' || workspace.activeTabIndex < 0) {
      return
    }
    if (workspace.activeToolTerminalHeight === value) {
      return
    }
    workspace.setTerminalHeight(workspace.activeTabIndex, value)
  },
)

function toolById(id: string) {
  return workbench.bootstrap?.tools.find((t) => t.id === id) ?? null
}

function builtinToolById(id: string) {
  return getBuiltinToolById(id) ?? null
}

function isMissingGoEnvError(detail: string) {
  return detail.includes('未检测到可用的 Go 环境')
    || detail.includes('请先在系统设置 > Go 中选择本地 Go 或下载 SDK')
    || detail.includes('指定的 Go 工具链不存在')
}

function isMissingPythonEnvError(detail: string) {
  return detail.includes('未检测到可用的基础 Python')
    || detail.includes('当前 Python 工具环境尚未准备好')
    || detail.includes('当前 Python 工具环境缺少 pip')
    || detail.includes('当前 Python 工具依赖未安装')
}

function isMissingRustEnvError(detail: string) {
  return detail.includes('未检测到可用的 Rust 交叉编译环境')
    || detail.includes('请先在系统设置 > Rust 中配置')
    || detail.includes('cargo-zigbuild')
    || detail.includes('rustup')
    || detail.includes('zig')
}

async function openGoSettings(messageText?: string) {
  workspace.openSettings('go')
  if (messageText) {
    message.warning(messageText)
  }
  if (!goEnv.state && !goEnv.loading) {
    await goEnv.loadState()
  }
}

async function openPythonSettings(messageText?: string) {
  workspace.openSettings('python')
  if (messageText) {
    message.warning(messageText)
  }
  if (!pythonEnv.state && !pythonEnv.loading) {
    await pythonEnv.loadState()
  }
}

async function openRustSettings(messageText?: string) {
  workspace.openSettings('rust')
  if (messageText) {
    message.warning(messageText)
  }
  if (!rustEnv.state && !rustEnv.loading) {
    await rustEnv.loadState()
  }
}

const allTools = computed(() => workbench.bootstrap?.tools ?? [])
const exportTargetOptions = [
  { label: 'Windows x64', value: 'windows/amd64' },
  { label: 'Windows ARM64', value: 'windows/arm64' },
  { label: 'Linux x64', value: 'linux/amd64' },
  { label: 'Linux ARM64', value: 'linux/arm64' },
  { label: 'macOS x64', value: 'darwin/amd64' },
  { label: 'macOS ARM64', value: 'darwin/arm64' },
]
const currentExportTarget = computed(() => workbench.bootstrap?.platform || 'windows/amd64')
const activeGoExportMode = computed(() => workspace.settings.goExportMode)
const activeExportTarget = computed({
  get: () => workspace.activeToolTab?.exportTarget || currentExportTarget.value,
  set: (value: string) => {
    if (workspace.activeTabIndex < 0 || !value) return
    workspace.setExportTarget(workspace.activeTabIndex, value)
  },
})

const activeExportButtonLabel = computed(() => {
  if (exporting.value) {
    return exportProgressText.value || '导出中'
  }
  const tool = toolById(activeToolId.value)
  if (!tool) return '导出'
  if (tool.kind === 'python') return '导出脚本'
  if (tool.kind === 'rust') return '导出二进制'
  return activeGoExportMode.value === 'source' ? '导出源码' : '导出二进制'
})

const showExportTargetSelector = computed(() => {
  const tool = toolById(activeToolId.value)
  return (tool?.kind === 'go' || tool?.kind === 'rust') && activeGoExportMode.value === 'binary'
})

const searchResults = computed(() => {
  const q = searchInput.value.trim().toLowerCase()
  if (!q) return allTools.value
  return allTools.value.filter(
    (t) =>
      t.name.toLowerCase().includes(q) ||
      t.description.toLowerCase().includes(q) ||
      t.category.some((c) => c.toLowerCase().includes(q)),
  )
})

const activeTabTaskId = computed(() => {
  const id = activeToolId.value
  if (!id) return ''
  const tasks = execution.recentTasks.filter((t) =>
    t.toolId === id &&
    (activeTargetIsRemote.value ? t.target.startsWith('remote:') : t.target === 'local'),
  )
  return tasks.length > 0 ? tasks[0].id : ''
})

function isTabRunning(toolId: string) {
  return execution.tasks.some((t) => t.toolId === toolId && t.status === 'running')
}

const activeTask = computed(() =>
  activeTabTaskId.value ? execution.recentTasks.find((t) => t.id === activeTabTaskId.value) ?? null : null,
)

async function handleExecute() {
  const tab = workspace.activeToolTab
  const tool = tab ? toolById(tab.toolId) : null
  if (!tool || !tab) return

  const config = workspace.activeExecutionConfig
  if (!config) return

  const cliArgsError = validateCliArgs(config.rawArgs)
  if (cliArgsError) {
    message.error(cliArgsError)
    return
  }

  if (config.panelMode !== 'cli') {
    const missingParam = findMissingRequiredParam(tool, config.formModel)
    if (missingParam) {
      message.error(`请先填写“${missingParam.label}”`)
      return
    }
  }

  if (tab.executionTarget === 'remote' && !tab.remoteConfig.connId) {
    message.error('请选择远程环境后再执行')
    return
  }

  if (workspace.activeTabIndex >= 0) {
    workspace.setTerminalVisible(workspace.activeTabIndex, true)
  }

  workspace.recordUsage(tool.id, config.rawArgs, config.pythonEnv, config.formModel)

  launching.value = true
  try {
    if (tab.executionTarget === 'remote') {
      await execution.startRemoteExecution({
        toolId: tool.id,
        connId: tab.remoteConfig.connId,
        args: config.rawArgs,
        pythonEnv: tool.kind === 'python' ? config.pythonEnv : undefined,
      })
    } else {
      await execution.startLocalExecution({
        toolId: tool.id,
        args: config.rawArgs,
        pythonEnv: tool.kind === 'python' ? undefined : undefined,
      })
    }
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    if (isMissingGoEnvError(detail)) {
      await openGoSettings('当前操作需要 Go 环境，已为你打开设置')
      return
    }
    if (isMissingPythonEnvError(detail)) {
      await openPythonSettings('当前操作需要 Python 环境，已为你打开设置')
      return
    }
    if (tool.kind === 'rust' && isMissingRustEnvError(detail)) {
      await openRustSettings('当前操作需要 Rust 交叉编译环境，已为你打开设置')
      return
    }
    message.error(detail || '执行失败')
  } finally {
    launching.value = false
  }
}

async function handleCancel() {
  const task = activeTask.value
  if (!task || task.status !== 'running') return
  await execution.cancelExecution(task.id)
}

function parseExportTarget(value: string) {
  const [targetOS, targetArch] = value.split('/')
  return {
    targetOS: targetOS || undefined,
    targetArch: targetArch || undefined,
  }
}

async function handleExport() {
  const tab = workspace.activeToolTab
  const tool = tab ? toolById(tab.toolId) : null
  if (!tool) return
  if (!tool.export?.strategy) {
    message.error('当前工具没有可用的导出能力')
    return
  }

  exporting.value = true
  exportProgressText.value = '准备导出'
  try {
    const mode = tool.kind === 'python' ? 'source' : tool.kind === 'go' ? activeGoExportMode.value : 'binary'
    const target = (tool.kind === 'go' || tool.kind === 'rust') && mode === 'binary'
      ? parseExportTarget(activeExportTarget.value)
      : {}
    const result = await ExportTool({
      toolId: tool.id,
      mode,
      ...target,
    })
    if (!result?.filePath) {
      return
    }

    message.success(`已导出 ${result.toolName}`)
    if (workspace.settings.autoOpenExportDir) {
      try {
        await OpenPath(result.directory)
      } catch (error) {
        const detail = error instanceof Error ? error.message : String(error)
        message.warning(`导出成功，但打开目录失败：${detail}`)
      }
    }
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    if (isMissingGoEnvError(detail)) {
      await openGoSettings('导出 Go 工具前需要先配置 Go 环境')
      return
    }
    if (tool.kind === 'rust' && isMissingRustEnvError(detail)) {
      await openRustSettings('导出 Rust 工具前需要先配置 Rust 交叉编译环境')
      return
    }
    message.error(detail || '工具导出失败')
  } finally {
    exporting.value = false
    exportProgressText.value = ''
  }
}

async function handleDownloadResult() {
  const task = activeTask.value
  if (!task || task.remoteResultStatus !== 'available') {
    message.warning('当前任务没有可下载结果')
    return
  }

  downloadingResult.value = true
  try {
    const downloadTask = await downloads.startTaskResultDownload(task.id)
    if (!downloadTask?.id) {
      return
    }
    message.success('已加入下载任务')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '下载结果失败')
  } finally {
    downloadingResult.value = false
  }
}

async function handleFileDialog(param: ParameterSpec, target?: 'file' | 'directory') {
  const tab = activeToolTabComputed.value
  const config = workspace.activeExecutionConfig
  if (!tab) return

  const dialogTarget =
    target ||
    (param.pathMode === 'file'
      ? 'file'
      : 'directory')
  let result: string

  if (dialogTarget === 'directory') {
    result = await OpenFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: true,
      defaultDirectory: '',
      defaultFilename: '',
    })
  } else {
    result = await OpenFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: false,
      defaultDirectory: '',
      defaultFilename: '',
    })
  }

  if (result) {
    if (config) {
      if (param.repeatable) {
        const currentValue = typeof config.formModel[param.key] === 'string' ? String(config.formModel[param.key] || '') : ''
        const items = currentValue
          .split(/\r?\n/)
          .map(item => item.trim())
          .filter(item => item.length > 0)
        if (!items.includes(result)) {
          items.push(result)
        }
        config.formModel[param.key] = items.join('\n')
      } else {
        config.formModel[param.key] = result
      }
    }
  }
}

function onPythonEnvUpdate(value: string) {
  if (workspace.activeTabIndex >= 0) {
    workspace.setPythonEnv(workspace.activeTabIndex, value)
  }
}

function onExecutionTargetUpdate(value: 'local' | 'remote') {
  if (workspace.activeTabIndex >= 0) {
    workspace.setExecutionTarget(workspace.activeTabIndex, value)
  }
}

function onRemoteConnIdUpdate(value: string) {
  if (workspace.activeTabIndex >= 0) {
    workspace.setRemoteConnection(workspace.activeTabIndex, value)
  }
}

function onExportTargetUpdate(value: string) {
  activeExportTarget.value = value
}

function toolTabStateByToolId(id: string) {
  return workspace.openTabs.find((tab) => tab.toolId === id)
}

function isToolTabRemote(id: string) {
  return toolTabStateByToolId(id)?.executionTarget === 'remote'
}

function toolExecutionThemeForTool(id: string) {
  return getExecutionTheme(toolById(id)?.kind, isToolTabRemote(id) ? 'remote' : 'local')
}

function toolKindTagStyleForTool(id: string): CSSProperties {
  const theme = toolExecutionThemeForTool(id)
  return {
    color: theme.accent,
    backgroundColor: theme.accentSoftBg,
    border: `1px solid ${theme.accentSoftBorder}`,
  }
}

function toolKindIconColorForTool(id: string) {
  return toolExecutionThemeForTool(id).accent
}

function toolNameStyleForTool(id: string): CSSProperties {
  return {
    color: toolExecutionThemeForTool(id).accent,
  }
}

function builtinTagStyleForTool(id: string): CSSProperties {
  const tool = builtinToolById(id)
  return {
    color: tool?.accent ?? '#8be9fd',
    backgroundColor: `${tool?.accent ?? '#8be9fd'}14`,
    border: `1px solid ${(tool?.accent ?? '#8be9fd')}30`,
  }
}

function builtinNameStyleForTool(id: string): CSSProperties {
  return {
    color: builtinToolById(id)?.accent ?? '#8be9fd',
  }
}

function isUnifiedTabActive(item: { type: string; arrayIndex: number }) {
  return (item.type === 'tool' && workspace.activeTabType === 'tool' && item.arrayIndex === workspace.activeTabIndex)
    || (item.type === 'builtin' && workspace.activeTabType === 'builtin' && item.arrayIndex === workspace.activeBuiltinTabIndex)
    || (item.type === 'ssh' && workspace.activeTabType === 'ssh' && item.arrayIndex === workspace.activeSSHTabIndex)
    || (item.type === 'artifact' && workspace.activeTabType === 'artifact' && item.arrayIndex === workspace.activeArtifactTabIndex)
}

function unifiedTabDisplayName(item: UnifiedTabItem) {
  if (item.type === 'tool') {
    return toolById(item.label)?.name ?? item.label
  }
  if (item.type === 'builtin') {
    return builtinToolById(item.label)?.name ?? item.label
  }
  return item.label
}

function handleTabLabelMouseEnter(event: MouseEvent, item: UnifiedTabItem) {
  onTooltipEnter(event, unifiedTabDisplayName(item))
}

const tabContextMenuShow = ref(false)
const tabContextMenuX = ref(0)
const tabContextMenuY = ref(0)
const tabContextMenuItem = ref<UnifiedTabItem | null>(null)

const tabContextMenuOptions = computed<WorkbenchMenuItem[]>(() => {
  const item = tabContextMenuItem.value
  if (!item) return []

  const tabs = workspace.unifiedTabs as UnifiedTabItem[]
  const tabIndex = tabs.findIndex(tab => tab.key === item.key)
  const closableLeftCount = tabIndex > 0 ? tabs.slice(0, tabIndex).filter(tab => !tab.pinned).length : 0
  const closableRightCount = tabIndex >= 0 ? tabs.slice(tabIndex + 1).filter(tab => !tab.pinned).length : 0
  const closableOtherCount = tabs.filter(tab => tab.key !== item.key && !tab.pinned).length
  const closableAllCount = tabs.filter(tab => !tab.pinned).length
  const isTool = item.type === 'tool'
  const isActive = isUnifiedTabActive(item)

  return [
    ...(!isActive ? [{ label: '切换到此标签', key: 'activate' }] : []),
    ...(isTool
      ? [{
          label: workspace.isFavorite(item.label) ? '取消收藏工具' : '收藏工具',
          key: 'favorite',
        }]
      : []),
    ...(isTool ? [{ type: 'divider' as const, key: 'divider-tool' }] : []),
    { label: '关闭标签', key: 'close' },
    { label: '关闭其他标签', key: 'close-others', disabled: closableOtherCount === 0 },
    { label: '关闭左侧标签', key: 'close-left', disabled: closableLeftCount === 0 },
    { label: '关闭右侧标签', key: 'close-right', disabled: closableRightCount === 0 },
    { label: '关闭所有标签', key: 'close-all', danger: true, disabled: closableAllCount === 0 },
    { type: 'divider' as const, key: 'divider-extra' },
    { label: '复制标签名称', key: 'copy-name' },
    { label: item.pinned ? '取消固定标签' : '固定标签', key: 'pin' },
  ]
})

function closeTabContextMenu() {
  tabContextMenuShow.value = false
  tabContextMenuItem.value = null
}

function openTabContextMenu(event: MouseEvent, item: UnifiedTabItem) {
  event.preventDefault()
  event.stopPropagation()
  tabContextMenuItem.value = item
  tabContextMenuShow.value = false
  tabContextMenuX.value = event.clientX
  tabContextMenuY.value = event.clientY
  nextTick(() => {
    tabContextMenuShow.value = true
  })
}

function resolveUnifiedTabByKey(key: string) {
  return (workspace.unifiedTabs as UnifiedTabItem[]).find(item => item.key === key) ?? null
}

function closeTabsByKeys(keys: string[]) {
  for (const key of keys) {
    const item = resolveUnifiedTabByKey(key)
    if (item && !item.pinned) {
      workspace.closeUnifiedTab(item)
    }
  }
}

async function handleTabContextMenuSelect(key: string) {
  const currentItem = tabContextMenuItem.value
  closeTabContextMenu()
  if (!currentItem) return

  const current = resolveUnifiedTabByKey(currentItem.key)
  if (!current && key !== 'close-all') return

  switch (key) {
    case 'activate':
      if (current) {
        workspace.activateUnifiedTab(current)
      }
      break
    case 'favorite':
      if (current?.type === 'tool') {
        workspace.toggleFavorite(current.label)
        message.success(workspace.isFavorite(current.label) ? `已收藏 ${unifiedTabDisplayName(current)}` : `已取消收藏 ${unifiedTabDisplayName(current)}`)
      }
      break
    case 'close':
      if (current) {
        workspace.closeUnifiedTab(current)
      }
      break
    case 'close-others':
      if (current) {
        closeTabsByKeys((workspace.unifiedTabs as UnifiedTabItem[]).filter(item => item.key !== current.key).map(item => item.key))
      }
      break
    case 'close-left':
      if (current) {
        const index = (workspace.unifiedTabs as UnifiedTabItem[]).findIndex(item => item.key === current.key)
        closeTabsByKeys((workspace.unifiedTabs as UnifiedTabItem[]).slice(0, index).map(item => item.key))
      }
      break
    case 'close-right':
      if (current) {
        const index = (workspace.unifiedTabs as UnifiedTabItem[]).findIndex(item => item.key === current.key)
        closeTabsByKeys((workspace.unifiedTabs as UnifiedTabItem[]).slice(index + 1).map(item => item.key))
      }
      break
    case 'close-all':
      closeTabsByKeys((workspace.unifiedTabs as UnifiedTabItem[]).map(item => item.key))
      break
    case 'copy-name':
      await navigator.clipboard.writeText(unifiedTabDisplayName(current ?? currentItem))
      message.success('已复制标签名称')
      break
    case 'pin':
      if (current) {
        const prevRects = captureTabRects()
        workspace.toggleTabPinned(current.key)
        await animateTabLayout(prevRects)
        message.success(workspace.isTabPinned(current.key) ? `已固定 ${unifiedTabDisplayName(current)}` : `已取消固定 ${unifiedTabDisplayName(current)}`)
      }
      break
  }
}

function tabButtonStyle(item: UnifiedTabItem): CSSProperties | undefined {
  const style: CSSProperties = {}
  if (dragTabKey.value === item.key) {
    style.cursor = 'grabbing'
  }
  return Object.keys(style).length > 0 ? style : undefined
}

function handleTabClick(item: UnifiedTabItem) {
  if (suppressClickAfterDrag.value) {
    suppressClickAfterDrag.value = false
    return
  }
  workspace.activateUnifiedTab(item)
}

function handleTabPointerDown(event: PointerEvent, item: UnifiedTabItem) {
  if (event.button !== 0) return
  const target = event.target as HTMLElement | null
  if (target?.closest('[data-tab-close]')) return

  const element = event.currentTarget as HTMLElement | null
  dragPointerId.value = event.pointerId
  dragTabKey.value = item.key
  dragActivated.value = false
  dragStartX.value = event.clientX
  dragCurrentX.value = 0
  dragSnapshotRects.value = captureTabRects()
  dragGroupKeys.value = (workspace.unifiedTabs as UnifiedTabItem[])
    .filter(tab => tab.pinned === item.pinned)
    .map(tab => tab.key)
  dragTargetIndex.value = dragGroupKeys.value.indexOf(item.key)
  if (element?.setPointerCapture) {
    element.setPointerCapture(event.pointerId)
  }
}

function handleGlobalPointerMove(event: PointerEvent) {
  if (dragPointerId.value !== event.pointerId || !dragTabKey.value) return
  const deltaX = event.clientX - dragStartX.value
  if (!dragActivated.value && Math.abs(deltaX) < 6) return

  dragActivated.value = true
  dragCurrentX.value = deltaX
  applyTabDragPreview()
}

async function finishTabPointerDrag(pointerId?: number | null) {
  if (!dragTabKey.value) return
  if (pointerId !== undefined && pointerId !== null && dragPointerId.value !== pointerId) return

  const dragKey = dragTabKey.value
  const scopedKeys = dragScopedTabKeys()
  const fromIndex = scopedKeys.indexOf(dragKey)
  const toIndex = dragActivated.value ? computeDragTargetIndex() : fromIndex
  const prevRects = dragActivated.value ? captureTabRects() : new Map<string, DOMRect>()

  dragPointerId.value = null
  dragSnapshotRects.value = new Map()
  dragGroupKeys.value = []
  dragCurrentX.value = 0
  dragTargetIndex.value = -1

  if (!dragActivated.value) {
    dragTabKey.value = ''
    clearTabTransforms()
    return
  }

  dragActivated.value = false
  dragTabKey.value = ''
  suppressClickAfterDrag.value = true

  if (fromIndex >= 0 && toIndex >= 0 && toIndex !== fromIndex) {
    const targetKey = scopedKeys[toIndex]
    workspace.moveUnifiedTab(dragKey, targetKey, toIndex > fromIndex ? 'after' : 'before')
    await animateTabLayout(prevRects, dragKey)
  } else {
    clearTabTransforms()
  }

  window.setTimeout(() => {
    suppressClickAfterDrag.value = false
  }, 0)
}

function handleGlobalPointerUp(event: PointerEvent) {
  void finishTabPointerDrag(event.pointerId)
}

function handleGlobalPointerCancel(event: PointerEvent) {
  void finishTabPointerDrag(event.pointerId)
}

function openSearch() {
  searchInput.value = ''
  activeSearchIndex.value = 0
  showSearchModal.value = true
}

function closeSearch() {
  showSearchModal.value = false
}

function selectSearchResult(toolId: string) {
  const tool = allTools.value.find((t) => t.id === toolId)
  if (tool) {
    workspace.openTool(tool)
  }
  closeSearch()
}

function moveSearchSelection(step: number) {
  if (searchResults.value.length === 0) return
  const nextIndex = activeSearchIndex.value + step
  activeSearchIndex.value = (nextIndex + searchResults.value.length) % searchResults.value.length
}

function selectActiveSearchResult() {
  const tool = searchResults.value[activeSearchIndex.value]
  if (tool) {
    selectSearchResult(tool.id)
  }
}

function handleSSHSaved(label: string) {
  workspace.updateActiveSSHTabLabel(label)
  emit('refreshSshList')
}

function handleSSHSavedOne(conn: SSHConnection) {
  workspace.promoteNewSSHTab(conn.id, conn.name)
  emit('refreshSshList')
}

function handleSSHDeleted() {
  emit('refreshSshList')
}

function handleSSHClose() {
  if (workspace.activeSSHTabIndex >= 0) {
    workspace.closeSSHTab(workspace.activeSSHTabIndex)
  }
}

function kindIcon(kind: string) {
  if (kind === 'python') return LogoPython
  if (kind === 'rust') return BuildOutline
  return CodeSlash
}

function toolKindTag(kind: string) {
  if (kind === 'python') return 'py'
  if (kind === 'rust') return 'rs'
  return 'go'
}

function onKeydown(e: KeyboardEvent) {
  if (showSearchModal.value) {
    if (e.key === 'ArrowDown') {
      e.preventDefault()
      moveSearchSelection(1)
      return
    }
    if (e.key === 'ArrowUp') {
      e.preventDefault()
      moveSearchSelection(-1)
      return
    }
    if (e.key === 'Enter') {
      e.preventDefault()
      selectActiveSearchResult()
      return
    }
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'p') {
    e.preventDefault()
    openSearch()
  }
  if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
    e.preventDefault()
    const id = activeToolId.value
    if (id) workspace.toggleFavorite(id)
  }
  if (e.key === 'F1' || e.key === 'f1') {
    e.preventDefault()
    workspace.showHotkeyHelp = !workspace.showHotkeyHelp
  }
  if (e.key === 'Escape') {
    if (showSearchModal.value) {
      closeSearch()
    }
  }
}

onMounted(() => {
  syncWorkspaceBodyHeight()
  document.addEventListener('keydown', onKeydown)
  window.addEventListener('pointermove', handleGlobalPointerMove)
  window.addEventListener('pointerup', handleGlobalPointerUp)
  window.addEventListener('pointercancel', handleGlobalPointerCancel)
  disposeExportProgress = EventsOn('export:progress', (event: { toolId?: string, message?: string }) => {
    if (event.toolId !== activeToolId.value) {
      return
    }
    exportProgressText.value = String(event.message ?? '').trim()
  })

  if (typeof ResizeObserver !== 'undefined') {
    layoutResizeObserver = new ResizeObserver(() => {
      syncWorkspaceBodyHeight()
    })
    if (contentRef.value) {
      layoutResizeObserver.observe(contentRef.value)
    }
    if (tabBarRef.value) {
      layoutResizeObserver.observe(tabBarRef.value)
    }
  }
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
  window.removeEventListener('pointermove', handleGlobalPointerMove)
  window.removeEventListener('pointerup', handleGlobalPointerUp)
  window.removeEventListener('pointercancel', handleGlobalPointerCancel)
  layoutResizeObserver?.disconnect()
  disposeExportProgress?.()
  clearTabTransforms()
})

watch(activeToolId, () => {
  if (!exporting.value) {
    exportProgressText.value = ''
  }
})

watch(searchInput, () => {
  activeSearchIndex.value = 0
})

watch(searchResults, (results) => {
  if (results.length === 0) {
    activeSearchIndex.value = 0
    return
  }
  if (activeSearchIndex.value >= results.length) {
    activeSearchIndex.value = results.length - 1
  }
})

watch(() => workspace.unifiedTabs.map(item => item.key).join('|'), () => {
  const currentKey = tabContextMenuItem.value?.key
  if (!currentKey) return
  if (!(workspace.unifiedTabs as UnifiedTabItem[]).some(item => item.key === currentKey)) {
    closeTabContextMenu()
  }
})
</script>

<template>
  <div
    ref="contentRef"
    class="flex flex-1 flex-col overflow-hidden"
  >
    <div
      ref="tabBarRef"
      class="relative flex shrink-0 items-end border-b border-white/15 bg-[#1a1b26]"
    >
      <div class="flex flex-1 items-end overflow-hidden">
        <button
          v-for="item in workspace.unifiedTabs"
          :key="item.key"
          :data-tab-key="item.key"
          class="ui-interactive group relative flex h-9 min-w-[176px] max-w-[280px] flex-[1_1_280px] items-center gap-1 overflow-hidden border-r border-white/15 px-3 py-1.5 pr-8"
          :class="
            isUnifiedTabActive(item)
              ? 'workspace-tab-active text-dracula-text'
              : 'bg-[#1a1b26] text-slate-500 hover:bg-dracula-bg/50 hover:text-slate-300'
          "
          :style="tabButtonStyle(item)"
          @click="handleTabClick(item)"
          @contextmenu="openTabContextMenu($event, item)"
          @pointerdown="handleTabPointerDown($event, item)"
        >
          <NIcon
            v-if="item.pinned"
            :component="BookmarkSharp"
            size="14"
            color="#ffb86c"
            class="shrink-0 opacity-90"
          />
          <NIcon
            v-if="item.type === 'ssh'"
            :component="ServerOutline"
            size="12"
            color="#8be9fd"
            class="shrink-0"
          />
          <NIcon
            v-if="item.type === 'artifact'"
            :component="CloudUploadOutline"
            size="12"
            color="#ffb86c"
            class="shrink-0"
          />
          <NTag
            v-if="item.type === 'tool'"
            :bordered="false"
            size="tiny"
            class="shrink-0"
            :style="toolKindTagStyleForTool(item.label)"
          >
            <template #icon>
              <NIcon
                :component="kindIcon(toolById(item.label)?.kind ?? '')"
                size="10"
                :color="toolKindIconColorForTool(item.label)"
              />
            </template>
            {{ toolKindTag(toolById(item.label)?.kind ?? '') }}
          </NTag>
          <NTag
            v-if="item.type === 'builtin'"
            :bordered="false"
            size="tiny"
            class="shrink-0"
            :style="builtinTagStyleForTool(item.label)"
          >
            <template #icon>
              <NIcon
                :component="getBuiltinToolIcon(item.label)"
                size="10"
                :color="builtinToolById(item.label)?.accent"
              />
            </template>
            内置
          </NTag>
          <NIcon
            v-if="item.type === 'tool' && workspace.isFavorite(item.label)"
            :component="Star"
            size="11"
            color="#f1fa8c"
            class="shrink-0"
          />
          <NIcon
            v-if="item.type === 'tool' && isToolTabRemote(item.label)"
            :component="GlobeOutline"
            size="11"
            :color="toolExecutionThemeForTool(item.label).accent"
            class="shrink-0"
          />
          <span
            v-if="item.type === 'tool' && isTabRunning(item.label)"
            class="h-1.5 w-1.5 shrink-0 rounded-full bg-dracula-green"
          />
          <span
            class="min-w-0 truncate text-xs"
            :style="item.type === 'tool' ? toolNameStyleForTool(item.label) : item.type === 'builtin' ? builtinNameStyleForTool(item.label) : undefined"
            :data-fullname="unifiedTabDisplayName(item)"
            @mouseenter="handleTabLabelMouseEnter($event, item)"
            @mouseleave="onTooltipLeave"
          >
            {{ unifiedTabDisplayName(item) }}
          </span>
          <span
            data-tab-close="true"
            class="ui-interactive absolute right-2 top-1/2 flex h-4 w-4 -translate-y-1/2 items-center justify-center rounded text-xs opacity-0 group-hover:opacity-100 hover:bg-dracula-soft hover:text-white"
            @pointerdown.stop
            @click.stop="workspace.closeUnifiedTab(item)"
          >×</span>
          <span
            v-if="isUnifiedTabActive(item)"
            class="workspace-tabs-active-indicator pointer-events-none absolute inset-x-2 bottom-0 h-0.5 rounded-t-sm"
          />
        </button>
      </div>
    </div>

    <template v-if="workspace.activeTabType === 'tool' && workspace.activeToolTab">
      <div class="flex flex-1 flex-col overflow-hidden">
        <div
          class="overflow-y-auto p-4"
          :class="isTerminalVisible ? 'min-h-0 flex-1 border-b border-white/15' : 'min-h-0 flex-1'"
        >
          <ToolDetailPanel
            :tool="toolById(workspace.activeToolTab.toolId)"
            :tab="workspace.activeToolTab"
            :active-task-id="activeTabTaskId"
            :active-task="activeTask"
            :is-running="activeTask?.status === 'running'"
            :is-launching="launching"
            :is-exporting="exporting"
            :is-downloading-result="downloadingResult"
            :export-target="activeExportTarget"
            :export-target-options="exportTargetOptions"
            :export-button-label="activeExportButtonLabel"
            :show-export-target-selector="showExportTargetSelector"
            @execute="handleExecute"
            @cancel="handleCancel"
            @export="handleExport"
            @download-result="handleDownloadResult"
            @update:execution-target="onExecutionTargetUpdate"
            @update:python-env="onPythonEnvUpdate"
            @update:remote-conn-id="onRemoteConnIdUpdate"
            @update:export-target="onExportTargetUpdate"
          />
          <div
            class="workspace-tabs-divider-line mx-4 mt-3 h-px"
          />
          <ParameterPanel
            :tool="toolById(workspace.activeToolTab.toolId)"
            :execution-target="workspace.activeToolTab.executionTarget"
            class="mt-3"
            @execute="handleExecute"
            @file-dialog="handleFileDialog"
          />
        </div>

        <template v-if="isTerminalVisible">
          <div
            v-bind="hDividerProps"
            class="group relative shrink-0 bg-white/10"
            style="height: 1px; width: 100%"
          >
            <div
              class="workspace-tabs-divider-glow absolute inset-x-0 -top-1 -bottom-1"
            />
          </div>

          <div
            class="flex min-h-0 shrink-0 flex-col overflow-hidden"
            :style="{ height: `${terminalHeight}px` }"
          >
            <ExecutionTerminal
              :task-id="activeTabTaskId"
              :execution-target="workspace.activeToolTab.executionTarget"
              :tool-kind="toolById(workspace.activeToolTab.toolId)?.kind"
            />
          </div>
        </template>
      </div>
    </template>

    <template v-else-if="workspace.activeTabType === 'ssh' && workspace.activeSSHTab">
      <SSHDetailPanel
        :connection-id="workspace.activeSSHTab.connectionId"
        :is-new="workspace.activeSSHTab.isNew"
        @close="handleSSHClose"
        @saved="handleSSHSaved"
        @saved-one="handleSSHSavedOne"
        @deleted="handleSSHDeleted"
      />
    </template>

    <template v-else-if="workspace.activeTabType === 'builtin' && workspace.activeBuiltinTab">
      <BuiltinToolPanel :builtin-tool-id="workspace.activeBuiltinTab.builtinToolId" />
    </template>

    <template v-else-if="workspace.activeTabType === 'artifact' && workspace.activeArtifactTab">
      <ArtifactCenterPanel v-if="workspace.activeArtifactTab.view === 'center'" />
      <ArtifactTaskSnapshotView
        v-else-if="workspace.activeArtifactTab.taskId"
        :task-id="workspace.activeArtifactTab.taskId"
      />
    </template>

    <div
      v-else
      class="flex flex-1 items-center justify-center bg-dracula-bg"
    >
      <div class="text-center">
        <NText
          depth="3"
          class="text-6xl"
        >
          火
        </NText>
        <p class="mt-4 text-base text-slate-500">
          火蜥蜴工具箱
        </p>
        <p class="mt-2 text-sm text-slate-600">
          从左侧选择工具开始使用
        </p>
        <div class="mt-6 flex items-center justify-center gap-x-4">
          <NTag
            size="small"
            :bordered="false"
            class="opacity-50"
          >
            Ctrl+P 搜索
          </NTag>
          <NTag
            size="small"
            :bordered="false"
            class="opacity-50"
          >
            Ctrl+F 收藏
          </NTag>
          <NTag
            size="small"
            :bordered="false"
            class="opacity-50"
          >
            F1 帮助
          </NTag>
        </div>
      </div>
    </div>

    <Teleport to="body">
      <WorkbenchContextMenu
        :show="tabContextMenuShow"
        :x="tabContextMenuX"
        :y="tabContextMenuY"
        :title="tabContextMenuItem ? unifiedTabDisplayName(tabContextMenuItem) : ''"
        :subtitle="tabContextMenuItem?.type === 'tool' ? '工具标签' : tabContextMenuItem?.type === 'ssh' ? 'SSH 标签' : tabContextMenuItem?.type === 'artifact' ? '产物中心' : ''"
        :items="tabContextMenuOptions"
        @select="handleTabContextMenuSelect"
        @close="closeTabContextMenu"
      />
    </Teleport>

    <Teleport to="body">
      <Transition name="fade">
        <div
          v-if="showSearchModal"
          class="fixed inset-0 z-50 bg-black/40 backdrop-blur-sm"
          @click="closeSearch"
        />
      </Transition>
      <Transition
        name="fade-scale"
        appear
      >
        <div
          v-if="showSearchModal"
          class="fixed inset-0 z-50 flex items-start justify-center pt-[15vh] pointer-events-none"
        >
          <div
            class="pointer-events-auto w-full max-w-lg rounded-xl border border-white/15 bg-dracula-panel shadow-2xl"
            @click.stop
          >
            <div class="p-4">
              <NInput
                v-model:value="searchInput"
                placeholder="搜索工具名称、说明..."
                size="large"
                autofocus
                clearable
              >
                <template #prefix>
                  <NIcon :component="Search" />
                </template>
              </NInput>
            </div>
            <NScrollbar style="max-height: 320px">
              <NList
                v-if="searchResults.length > 0"
                hoverable
                clickable
              >
                <NListItem
                  v-for="(tool, index) in searchResults"
                  :key="tool.id"
                  v-press
                  class="ui-interactive"
                  :class="index === activeSearchIndex ? 'bg-dracula-cyan/10' : ''"
                  @mouseenter="activeSearchIndex = index"
                  @click="selectSearchResult(tool.id)"
                >
                  <template #prefix>
                    <NIcon
                      :component="kindIcon(tool.kind)"
                      size="18"
                      :color="tool.kind === 'python' ? '#f1fa8c' : tool.kind === 'rust' ? '#ffb86c' : '#8be9fd'"
                    />
                  </template>
                  <div class="min-w-0 flex-1">
                    <NText class="text-sm">
                      {{ tool.name }}
                    </NText>
                    <NText
                      depth="3"
                      class="truncate text-xs"
                    >
                      {{ tool.category.join(' > ') }} · {{ tool.description }}
                    </NText>
                  </div>
                </NListItem>
              </NList>
              <div
                v-else
                class="py-8 text-center"
              >
                <NText
                  depth="3"
                  class="text-sm"
                >
                  未找到匹配的工具
                </NText>
              </div>
            </NScrollbar>
            <div class="border-t border-white/10 px-4 py-2 text-[11px] text-slate-500">
              ↑↓ 切换 · Enter 打开 · Esc 关闭
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="tooltipShow"
        class="workbench-tooltip pointer-events-none fixed z-[100] -translate-x-1/2 px-2.5 py-1.5 text-xs"
        :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }"
      >
        <div class="workbench-tooltip-arrow absolute -top-1 left-1/2 h-2 w-2 -translate-x-1/2 rotate-45" />
        {{ tooltipText }}
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.workspace-tab-active {
  background-color: v-bind(workspaceActiveTabBackground);
}

.workspace-tabs-active-indicator {
  background-color: v-bind(workspaceAccent);
}

.workspace-tabs-divider-line {
  background: v-bind(workspaceDividerGradient);
}

.workspace-tabs-divider-glow {
  transition: background-color 0.16s var(--ease-out-soft);
}

.group:hover .workspace-tabs-divider-glow {
  background-color: v-bind(workspaceAccentSoftBg);
}

.group:active .workspace-tabs-divider-glow {
  background-color: v-bind(workspaceAccentSoftStrongBg);
}
</style>
