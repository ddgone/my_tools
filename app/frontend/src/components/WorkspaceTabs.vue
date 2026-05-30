<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, nextTick, type CSSProperties } from 'vue'
import { NInput, NIcon, NList, NListItem, NScrollbar, NText, NTag } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { Search, ServerOutline, CodeSlash, LogoPython, Star, GlobeOutline } from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { useResizable } from '@/composables/useResizable'
import { useTruncationTooltip } from '@/composables/useTruncationTooltip'
import { OpenFileDialog, OpenSaveFileDialog } from '../../wailsjs/go/main/App'
import type { ParameterSpec, SSHConnection } from '@/types/workbench'
import ToolDetailPanel from './ToolDetailPanel.vue'
import ParameterPanel from './ParameterPanel.vue'
import ExecutionTerminal from './ExecutionTerminal.vue'
import SSHDetailPanel from './SSHDetailPanel.vue'
import WorkbenchContextMenu from './WorkbenchContextMenu.vue'
import { validateCliArgs } from '@/utils/cliArgs'
import { getExecutionTheme, makeExecutionThemeVars } from '@/utils/executionTheme'

const emit = defineEmits<{
  refreshSshList: []
}>()

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()
const message = useMessage()

const launching = ref(false)
const searchInput = ref('')
const activeSearchIndex = ref(0)
const contentRef = ref<HTMLElement | null>(null)
const tabBarRef = ref<HTMLElement | null>(null)
type UnifiedTabItem = {
  type: 'tool' | 'ssh'
  key: string
  label: string
  openedAt: number
  arrayIndex: number
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

const tabKey = computed(() => {
  if (workspace.activeTabType === 'tool')
    return `tool-${workspace.activeTabIndex}`
  if (workspace.activeTabType === 'ssh')
    return `ssh-${workspace.activeSSHTabIndex}`
  return 'empty'
})

const indicatorLeft = ref('0px')
const indicatorWidth = ref('0px')
const indicatorShow = ref(false)
const indicatorInstant = ref(false)

function syncIndicator() {
  const bar = tabBarRef.value
  if (!bar || workspace.unifiedTabs.length === 0) {
    indicatorShow.value = false
    return
  }
  const buttons = bar.querySelectorAll<HTMLElement>('button')
  const idx = workspace.unifiedTabs.findIndex(t => {
    if (workspace.activeTabType === 'tool') return t.type === 'tool' && t.arrayIndex === workspace.activeTabIndex
    if (workspace.activeTabType === 'ssh') return t.type === 'ssh' && t.arrayIndex === workspace.activeSSHTabIndex
    return false
  })
  if (idx < 0 || idx >= buttons.length) {
    indicatorShow.value = false
    return
  }
  const barRect = bar.getBoundingClientRect()
  const btnRect = buttons[idx].getBoundingClientRect()
  indicatorLeft.value = `${btnRect.left - barRect.left}px`
  indicatorWidth.value = `${btnRect.width}px`
  indicatorShow.value = true
}

watch(tabKey, () => {
  indicatorInstant.value = true
  nextTick(() => {
    syncIndicator()
    requestAnimationFrame(() => {
      indicatorInstant.value = false
    })
  })
})
watch(() => workspace.unifiedTabs.length, () => nextTick(syncIndicator))
watch(() => workspace.unifiedTabs.map(t => t.label).join('|'), () => nextTick(syncIndicator))
onMounted(() => {
  nextTick(syncIndicator)
  const observer = new ResizeObserver(() => nextTick(syncIndicator))
  if (tabBarRef.value) observer.observe(tabBarRef.value)
  ;(window as any).__tabIndicatorObserver = observer
})

onUnmounted(() => {
  const observer = (window as any).__tabIndicatorObserver
  if (observer) observer.disconnect()
})

const showSearchModal = computed({
  get: () => workspace.showSearch,
  set: (v) => (workspace.showSearch = v),
})

const initialTop = Math.max(280, Math.min(600, Math.floor(window.innerHeight * 0.42)))

const { size: topHeight, dividerProps: hDividerProps } = useResizable({
  axis: 'y',
  min: 200,
  max: Math.max(400, Math.floor(window.innerHeight * 0.78)),
  initial: initialTop,
  storageKey: 'fire-salamander:panel-split',
})

const activeToolId = computed(() => workspace.activeToolTab?.toolId ?? '')
const activeToolTabComputed = computed(() => workspace.activeToolTab)
const activeTargetIsRemote = computed(() => workspace.activeToolTab?.executionTarget === 'remote')
const activeToolKind = computed(() => toolById(activeToolId.value)?.kind ?? '')
const activeExecutionTheme = computed(() =>
  getExecutionTheme(activeToolKind.value, activeTargetIsRemote.value ? 'remote' : 'local'),
)
const workspaceThemeStyle = computed<CSSProperties>(() => ({
  ...makeExecutionThemeVars(activeExecutionTheme.value, 'workspace-tabs'),
}))

function toolById(id: string) {
  return workbench.bootstrap?.tools.find((t) => t.id === id) ?? null
}

const allTools = computed(() => workbench.bootstrap?.tools ?? [])

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

  if (tab.executionTarget === 'remote' && !tab.remoteConfig.connId) {
    message.error('请选择远程环境后再执行')
    return
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
        pythonEnv: tool.kind === 'python' ? config.pythonEnv : undefined,
      })
    }
  } finally {
    launching.value = false
  }
}

async function handleCancel() {
  const task = activeTask.value
  if (!task || task.status !== 'running') return
  await execution.cancelExecution(task.id)
}

async function handleFileDialog(param: ParameterSpec) {
  const tab = activeToolTabComputed.value
  const config = workspace.activeExecutionConfig
  if (!tab) return

  const key = param.key.toLowerCase()
  const label = param.label.toLowerCase()
  const placeholder = (param.placeholder || '').toLowerCase()
  const help = (param.help || '').toLowerCase()
  const isDir =
    key.includes('dir') ||
    key.includes('folder') ||
    label.includes('目录') ||
    label.includes('文件夹') ||
    placeholder.includes('目录') ||
    help.includes('目录')
  const isSave = !isDir && (key.includes('output') || key.includes('save') || label.includes('输出') || label.includes('保存'))
  let result: string

  if (isDir) {
    result = await OpenFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: true,
    })
  } else if (isSave) {
    result = await OpenSaveFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: false,
    })
  } else {
    result = await OpenFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: isDir,
    })
  }

  if (result) {
    if (config) {
      config.formModel[param.key] = result
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
    nextTick(syncIndicator)
  }
}

function onRemoteConnIdUpdate(value: string) {
  if (workspace.activeTabIndex >= 0) {
    workspace.setRemoteConnection(workspace.activeTabIndex, value)
  }
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

function isUnifiedTabActive(item: { type: string; arrayIndex: number }) {
  return (item.type === 'tool' && workspace.activeTabType === 'tool' && item.arrayIndex === workspace.activeTabIndex)
    || (item.type === 'ssh' && workspace.activeTabType === 'ssh' && item.arrayIndex === workspace.activeSSHTabIndex)
}

function unifiedTabDisplayName(item: UnifiedTabItem) {
  return item.type === 'tool' ? (toolById(item.label)?.name ?? item.label) : item.label
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
  const hasLeft = tabIndex > 0
  const hasRight = tabIndex >= 0 && tabIndex < tabs.length - 1
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
    { label: '关闭其他标签', key: 'close-others', disabled: tabs.length <= 1 },
    { label: '关闭左侧标签', key: 'close-left', disabled: !hasLeft },
    { label: '关闭右侧标签', key: 'close-right', disabled: !hasRight },
    { label: '关闭所有标签', key: 'close-all', danger: true, disabled: tabs.length === 0 },
    { type: 'divider' as const, key: 'divider-extra' },
    { label: '复制标签名称', key: 'copy-name' },
    { label: '固定标签', key: 'pin', hint: '占位' },
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
    if (item) {
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
      message.info(`已为 ${unifiedTabDisplayName(current ?? currentItem)} 预留“固定标签”菜单动作`)
      break
  }
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
  return kind === 'python' ? LogoPython : CodeSlash
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
  document.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeydown)
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
    :style="workspaceThemeStyle"
  >
    <div
      ref="tabBarRef"
      class="relative flex shrink-0 items-end border-b border-white/15 bg-[#1a1b26]"
    >
      <div class="flex flex-1 items-end overflow-hidden">
        <button
          v-for="item in workspace.unifiedTabs"
          :key="item.key"
          v-press
          class="ui-interactive group flex min-w-0 items-center gap-1 border-r border-white/15 px-2.5 py-1.5"
          :class="
            isUnifiedTabActive(item)
              ? 'text-dracula-text'
              : 'bg-[#1a1b26] text-slate-500 hover:bg-dracula-bg/50 hover:text-slate-300'
          "
          :style="isUnifiedTabActive(item) && item.type === 'tool'
            ? { backgroundColor: 'var(--workspace-tabs-active-tab-bg)' }
            : undefined"
          @click="workspace.activateUnifiedTab(item)"
          @contextmenu="openTabContextMenu($event, item)"
        >
          <NIcon
            v-if="item.type === 'ssh'"
            :component="ServerOutline"
            size="12"
            color="#8be9fd"
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
                :component="toolById(item.label)?.kind === 'python' ? LogoPython : CodeSlash"
                size="10"
                :color="toolKindIconColorForTool(item.label)"
              />
            </template>
            {{ toolById(item.label)?.kind === 'python' ? 'py' : 'go' }}
          </NTag>
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
            class="truncate text-xs"
            :style="item.type === 'tool' ? toolNameStyleForTool(item.label) : undefined"
            :data-fullname="unifiedTabDisplayName(item)"
            @mouseenter="handleTabLabelMouseEnter($event, item)"
            @mouseleave="onTooltipLeave"
          >
            {{ unifiedTabDisplayName(item) }}
          </span>
          <NIcon
            v-if="item.type === 'tool' && workspace.isFavorite(item.label)"
            :component="Star"
            size="10"
            color="#f1fa8c"
            class="shrink-0 opacity-70"
          />
          <span
            v-press
            class="ui-interactive ml-0.5 flex h-4 w-4 shrink-0 items-center justify-center rounded text-xs opacity-0 group-hover:opacity-100 hover:bg-dracula-soft hover:text-white"
            @pointerdown.stop
            @click.stop="workspace.closeUnifiedTab(item)"
          >×</span>
        </button>
      </div>
      <div
        class="absolute bottom-0 h-0.5 rounded-t-sm transition-[left,width,opacity,background-color] ease-out"
        :class="[indicatorInstant ? 'duration-0' : 'duration-200']"
        :style="{
          left: indicatorLeft,
          width: indicatorWidth,
          opacity: indicatorShow ? 1 : 0,
          backgroundColor: 'var(--workspace-tabs-accent)',
        }"
      />
    </div>

    <template v-if="workspace.activeTabType === 'tool' && workspace.activeToolTab">
      <div class="flex flex-1 flex-col overflow-hidden">
        <div
          class="shrink-0 overflow-y-auto border-b border-white/15 p-4"
          :style="{ height: topHeight + 'px' }"
        >
          <ToolDetailPanel
            :tool="toolById(workspace.activeToolTab.toolId)"
            :tab="workspace.activeToolTab"
            :active-task-id="activeTabTaskId"
            :is-running="activeTask?.status === 'running'"
            :is-launching="launching"
            @execute="handleExecute"
            @cancel="handleCancel"
            @update:execution-target="onExecutionTargetUpdate"
            @update:python-env="onPythonEnvUpdate"
            @update:remote-conn-id="onRemoteConnIdUpdate"
          />
          <div
            class="mx-4 mt-3 h-px"
            :style="{ background: 'var(--workspace-tabs-divider-gradient)' }"
          />
          <ParameterPanel
            :tool="toolById(workspace.activeToolTab.toolId)"
            :execution-target="workspace.activeToolTab.executionTarget"
            class="mt-3"
            @execute="handleExecute"
            @file-dialog="handleFileDialog"
          />
        </div>

        <div
          v-bind="hDividerProps"
          class="group relative shrink-0 bg-white/10"
          style="height: 1px; width: 100%"
        >
          <div
            class="workspace-tabs-divider-glow absolute inset-x-0 -top-1 -bottom-1"
          />
        </div>

        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <ExecutionTerminal
            :task-id="activeTabTaskId"
            :execution-target="workspace.activeToolTab.executionTarget"
            :tool-kind="toolById(workspace.activeToolTab.toolId)?.kind"
          />
        </div>
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
        :subtitle="tabContextMenuItem?.type === 'tool' ? '工具标签' : tabContextMenuItem?.type === 'ssh' ? 'SSH 标签' : ''"
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
                      :color="tool.kind === 'python' ? '#f1fa8c' : '#8be9fd'"
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
.workspace-tabs-divider-glow {
  transition: background-color 0.16s var(--ease-out-soft);
}

.group:hover .workspace-tabs-divider-glow {
  background-color: var(--workspace-tabs-accent-soft-bg);
}

.group:active .workspace-tabs-divider-glow {
  background-color: var(--workspace-tabs-accent-soft-strong-bg);
}
</style>
