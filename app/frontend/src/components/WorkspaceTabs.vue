<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { useResizable } from '@/composables/useResizable'
import { OpenFileDialog, OpenSaveFileDialog } from '../../wailsjs/go/main/App'
import type { ParameterSpec } from '@/types/workbench'
import ToolDetailPanel from './ToolDetailPanel.vue'
import ParameterPanel from './ParameterPanel.vue'
import ExecutionTerminal from './ExecutionTerminal.vue'

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const launching = ref(false)
const searchInput = ref('')

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

const activeToolId = computed(() => workspace.activeTab()?.toolId ?? '')
const activeTab = computed(() => workspace.activeTab())

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
          t.category.toLowerCase().includes(q),
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
  const tab = workspace.activeTab()
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
  const tab = workspace.activeTab()
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
  const tab = activeTab.value
  if (!tab) return

  const isSave = param.key.toLowerCase().includes('output') || param.key.toLowerCase().includes('save')
  let result: string

  if (isSave) {
    result = await OpenSaveFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: false,
    })
  } else {
    const isDir = param.key.toLowerCase().includes('dir') || param.key.toLowerCase().includes('input')
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

function openSearch() {
  searchInput.value = ''
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

function onPythonEnvUpdate(value: string) {
  const tab = activeTab.value
  if (tab) {
    tab.pythonEnv = value
  }
}

function onKeydown(e: KeyboardEvent) {
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
</script>

<template>
  <div class="flex flex-1 flex-col overflow-hidden">
    <div class="flex shrink-0 items-center border-b border-dracula-soft bg-[#1a1b26]">
      <div class="flex flex-1 overflow-x-auto">
        <button
          v-for="(tab, index) in workspace.openTabs"
          :key="tab.toolId"
          class="group flex shrink-0 items-center gap-2 border-r border-dracula-soft px-4 py-2 text-sm transition"
          :class="
            index === workspace.activeTabIndex
              ? 'bg-dracula-bg text-dracula-text'
              : 'bg-[#1a1b26] text-slate-500 hover:bg-dracula-bg/50 hover:text-slate-300'
          "
          @click="workspace.setActiveTab(index)"
        >
          <span
            v-if="isTabRunning(tab.toolId)"
            class="h-1.5 w-1.5 rounded-full bg-dracula-green"
          />
          <span class="max-w-[140px] truncate">{{ toolById(tab.toolId)?.name ?? tab.toolId }}</span>
          <span
            class="ml-1 flex h-4 w-4 items-center justify-center rounded text-xs opacity-0 transition group-hover:opacity-100 hover:bg-dracula-soft hover:text-white"
            @click.stop="workspace.closeTab(index)"
          >×</span>
        </button>
      </div>
    </div>

    <template v-if="workspace.activeTab()">
      <div class="flex flex-1 flex-col overflow-hidden">
        <div
          class="shrink-0 overflow-y-auto border-b border-dracula-soft p-4"
          :style="{ height: topHeight + 'px' }"
        >
          <ToolDetailPanel
            :tool="toolById(workspace.activeTab()!.toolId)"
            :tab="workspace.activeTab()!"
            :active-task-id="activeTabTaskId"
            :is-running="activeTask?.status === 'running'"
            :is-launching="launching"
            @execute="handleExecute"
            @cancel="handleCancel"
            @update:python-env="onPythonEnvUpdate"
            @remote-execute="handleRemoteExecute"
          />
          <ParameterPanel
            :tool="toolById(workspace.activeTab()!.toolId)"
            @execute="handleExecute"
            @file-dialog="handleFileDialog"
          />
        </div>

        <div
          v-bind="hDividerProps"
          class="group relative shrink-0 bg-dracula-soft"
          style="height: 1px; width: 100%"
        >
          <div class="absolute inset-x-0 -top-1 -bottom-1 group-hover:bg-dracula-cyan/10 group-active:bg-dracula-cyan/20" />
        </div>

        <div class="flex min-h-0 flex-1 flex-col overflow-hidden">
          <ExecutionTerminal :task-id="activeTabTaskId" />
        </div>
      </div>
    </template>

    <div
      v-else
      class="flex flex-1 items-center justify-center"
    >
      <div class="text-center">
        <div class="text-5xl opacity-50">
          🦎
        </div>
        <p class="mt-4 text-lg text-slate-500">
          火蜥蜴工具箱
        </p>
        <p class="mt-2 text-sm text-slate-600">
          从左侧选择工具开始使用
        </p>
        <p class="mt-4 text-xs text-slate-600">
          Ctrl+P 搜索 | Ctrl+F 收藏 | F1 帮助
        </p>
      </div>
    </div>

    <Teleport to="body">
      <div
        v-if="showSearchModal"
        class="fixed inset-0 z-50 flex items-start justify-center bg-black/40 pt-[15vh]"
        @click="closeSearch"
      >
        <div
          class="w-full max-w-lg rounded-xl border border-dracula-soft bg-dracula-panel shadow-2xl"
          @click.stop
        >
          <div class="relative border-b border-dracula-soft p-4">
            <input
              v-model="searchInput"
              type="text"
              placeholder="搜索工具名称、说明..."
              class="w-full bg-transparent text-base text-white placeholder-slate-500 outline-none"
              autofocus
            >
            <span class="absolute right-4 top-1/2 -translate-y-1/2 text-xs text-slate-500">ESC 关闭</span>
          </div>
          <div class="max-h-64 overflow-y-auto p-2">
            <button
              v-for="tool in searchResults"
              :key="tool.id"
              class="flex w-full items-center gap-3 rounded-lg px-4 py-2.5 text-left transition hover:bg-white/5"
              @click="selectSearchResult(tool.id)"
            >
              <span class="text-sm text-slate-400">{{ tool.kind === 'python' ? '🐍' : '⚡' }}</span>
              <div class="min-w-0 flex-1">
                <div class="text-sm text-white">
                  {{ tool.name }}
                </div>
                <div class="truncate text-xs text-slate-500">
                  {{ tool.category }} · {{ tool.description }}
                </div>
              </div>
            </button>
            <div
              v-if="searchResults.length === 0"
              class="p-4 text-center text-sm text-slate-500"
            >
              未找到匹配的工具
            </div>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>
