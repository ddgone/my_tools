<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, nextTick } from 'vue'
import { NInput, NIcon, NList, NListItem, NScrollbar, NText, NTag } from 'naive-ui'
import { Search, ServerOutline, CodeSlash, LogoPython, Star } from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { useResizable } from '@/composables/useResizable'
import { useTruncationTooltip } from '@/composables/useTruncationTooltip'
import { OpenFileDialog, OpenSaveFileDialog } from '../../wailsjs/go/main/App'
import type { ParameterSpec } from '@/types/workbench'
import ToolDetailPanel from './ToolDetailPanel.vue'
import ParameterPanel from './ParameterPanel.vue'
import ExecutionTerminal from './ExecutionTerminal.vue'
import SSHDetailPanel from './SSHDetailPanel.vue'

const emit = defineEmits<{
  refreshSshList: []
}>()

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const launching = ref(false)
const searchInput = ref('')
const activeSearchIndex = ref(0)
const contentRef = ref<HTMLElement | null>(null)
const tabBarRef = ref<HTMLElement | null>(null)

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

watch(tabKey, () => nextTick(syncIndicator))
watch(() => workspace.unifiedTabs.length, () => nextTick(syncIndicator))
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
  const tasks = execution.recentTasks.filter((t) => t.toolId === id)
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

  workspace.recordUsage(tool.id, tab.rawArgs, tab.pythonEnv, tab.formModel)

  launching.value = true
  try {
    await execution.startLocalExecution({
      toolId: tool.id,
      args: tab.rawArgs,
      pythonEnv: tool.kind === 'python' ? tab.pythonEnv : undefined,
    })
  } finally {
    launching.value = false
  }
}

async function handleRemoteExecute(connId: string) {
  const tab = workspace.activeToolTab
  const tool = tab ? toolById(tab.toolId) : null
  if (!tool || !tab) return

  workspace.recordUsage(tool.id, tab.rawArgs, tab.pythonEnv, tab.formModel)

  launching.value = true
  try {
    await execution.startRemoteExecution({
      toolId: tool.id,
      connId,
      args: tab.rawArgs,
      pythonEnv: tool.kind === 'python' ? tab.pythonEnv : undefined,
    })
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
  if (!tab) return

  const key = param.key.toLowerCase()
  const label = param.label.toLowerCase()
  const isSave = key.includes('output') || key.includes('save') || label.includes('输出') || label.includes('保存')
  let result: string

  if (isSave) {
    result = await OpenSaveFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: false,
    })
  } else {
    const isDir = key.includes('dir') || key.includes('folder') || label.includes('目录') || label.includes('文件夹')
    result = await OpenFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: isDir,
    })
  }

  if (result) {
    tab.formModel[param.key] = result
  }
}

function onPythonEnvUpdate(value: string) {
  const tab = activeToolTabComputed.value
  if (tab) {
    tab.pythonEnv = value
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

function handleSSHSaved() {
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
          v-press
          class="ui-interactive group flex min-w-0 items-center gap-1 border-r border-white/15 px-2.5 py-1.5"
          :class="
            (item.type === 'tool' && workspace.activeTabType === 'tool' && item.arrayIndex === workspace.activeTabIndex) ||
              (item.type === 'ssh' && workspace.activeTabType === 'ssh' && item.arrayIndex === workspace.activeSSHTabIndex)
              ? 'bg-dracula-bg text-dracula-text'
              : 'bg-[#1a1b26] text-slate-500 hover:bg-dracula-bg/50 hover:text-slate-300'
          "
          @click="workspace.activateUnifiedTab(item)"
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
            :type="toolById(item.label)?.kind === 'python' ? 'success' : 'info'"
            class="shrink-0"
          >
            <template #icon>
              <NIcon
                :component="toolById(item.label)?.kind === 'python' ? LogoPython : CodeSlash"
                size="10"
              />
            </template>
            {{ toolById(item.label)?.kind === 'python' ? 'py' : 'go' }}
          </NTag>
          <span
            v-if="item.type === 'tool' && isTabRunning(item.label)"
            class="h-1.5 w-1.5 shrink-0 rounded-full bg-dracula-green"
          />
          <span
            class="truncate text-xs"
            :data-fullname="item.type === 'tool' ? (toolById(item.label)?.name ?? item.label) : item.label"
            @mouseenter="(e: MouseEvent) => onTooltipEnter(e, item.type === 'tool' ? (toolById(item.label)?.name ?? item.label) : item.label)"
            @mouseleave="onTooltipLeave"
          >
            {{ item.type === 'tool' ? (toolById(item.label)?.name ?? item.label) : item.label }}
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
        class="absolute bottom-0 h-0.5 rounded-t-sm bg-dracula-cyan transition-[left,width,opacity] duration-200 ease-out"
        :style="{
          left: indicatorLeft,
          width: indicatorWidth,
          opacity: indicatorShow ? 1 : 0,
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
            @update:python-env="onPythonEnvUpdate"
            @remote-execute="handleRemoteExecute"
          />
          <div
            class="mx-4 mt-3 h-px"
            style="background: linear-gradient(to right, transparent, rgba(255,255,255,0.14), transparent)"
          />
          <ParameterPanel
            :tool="toolById(workspace.activeToolTab.toolId)"
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
          <div class="absolute inset-x-0 -top-1 -bottom-1 group-hover:bg-dracula-cyan/10 group-active:bg-dracula-cyan/20" />
        </div>

        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <ExecutionTerminal :task-id="activeTabTaskId" />
        </div>
      </div>
    </template>

    <template v-else-if="workspace.activeTabType === 'ssh' && workspace.activeSSHTab">
      <SSHDetailPanel
        :connection-id="workspace.activeSSHTab.connectionId"
        :is-new="workspace.activeSSHTab.isNew"
        @close="handleSSHClose"
        @saved="handleSSHSaved"
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
        class="pointer-events-none fixed z-[100] -translate-x-1/2 rounded-md bg-[#faf8f5] px-2.5 py-1.5 text-xs text-[#1a1a2e] shadow-lg shadow-black/20"
        :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }"
      >
        <div class="absolute -top-1 left-1/2 -translate-x-1/2 h-2 w-2 rotate-45 bg-[#faf8f5]" />
        {{ tooltipText }}
      </div>
    </Teleport>
  </div>
</template>
