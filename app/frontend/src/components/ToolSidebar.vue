<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NDropdown, NScrollbar, NTag, useMessage } from 'naive-ui'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { DeleteSSHConnection, ListSSHConnections, TestSSHConnection } from '../../wailsjs/go/main/App'
import type { SSHConnection, ToolManifest } from '@/types/workbench'

const props = defineProps<{
  width: number
}>()

const emit = defineEmits<{
  selectConnection: [conn: SSHConnection]
  createConnection: []
}>()

const message = useMessage()
const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const searchQuery = ref('')
const collapsedCategories = ref(new Set<string>())
const sidebarView = ref<'tools' | 'ssh' | 'history' | 'export'>('tools')

const sshConnections = ref<SSHConnection[]>([])

async function loadSSHConnections() {
  try {
    sshConnections.value = await ListSSHConnections()
  } catch {
    sshConnections.value = []
  }
}

async function handleDeleteSSH(id: string) {
  try {
    await DeleteSSHConnection(id)
    message.success('连接已删除')
    await loadSSHConnections()
  } catch (e: any) {
    message.error(e.toString())
  }
}

async function handleTestSSH(id: string) {
  try {
    const result = await TestSSHConnection(id)
    if (result.success) {
      message.success(result.message)
    } else {
      message.error(result.message)
    }
  } catch (e: any) {
    message.error(e.toString())
  }
}

function selectConnection(conn: SSHConnection) {
  emit('selectConnection', conn)
}

function createConnection() {
  emit('createConnection')
}

function copyHost(conn: SSHConnection) {
  const text = `${conn.user}@${conn.host}:${conn.port}`
  navigator.clipboard.writeText(text)
  message.success('已复制: ' + text)
}

watch(sidebarView, (view) => {
  if (view === 'ssh') {
    loadSSHConnections()
  }
})

const filteredTools = computed(() => {
  const tools = workbench.bootstrap?.tools ?? []
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return tools
  return tools.filter(
      (t) =>
          t.name.toLowerCase().includes(query) ||
          t.description.toLowerCase().includes(query) ||
          t.category.toLowerCase().includes(query),
  )
})

const groupedTools = computed(() => {
  const groups = new Map<string, ToolManifest[]>()
  for (const tool of filteredTools.value) {
    const current = groups.get(tool.category) ?? []
    current.push(tool)
    groups.set(tool.category, current)
  }
  return Array.from(groups.entries()).map(([category, tools]) => ({ category, tools }))
})

const favoriteTools = computed(() => {
  const tools = workbench.bootstrap?.tools ?? []
  return workspace.favorites.map((id) => tools.find((t) => t.id === id)).filter(Boolean) as ToolManifest[]
})

const recentToolList = computed(() => {
  const tools = workbench.bootstrap?.tools ?? []
  return workspace.recentTools
      .map((r) => {
        const tool = tools.find((t) => t.id === r.toolId)
        if (!tool) return null
        return { tool, args: r.args }
      })
      .filter(Boolean) as { tool: ToolManifest; args: string }[]
})

function toggleCategory(category: string) {
  const c = new Set(collapsedCategories.value)
  if (c.has(category)) c.delete(category)
  else c.add(category)
  collapsedCategories.value = c
}

function isToolRunning(toolId: string) {
  return execution.tasks.some((t) => t.toolId === toolId && t.status === 'running')
}

function isToolActive(toolId: string) {
  return workspace.activeTab()?.toolId === toolId
}

function selectTool(tool: ToolManifest) {
  workspace.openTool(tool)
}

function kindTagType(kind: string) {
  return kind === 'python' ? 'success' : 'info'
}

function sshDropdownOptions(_conn: SSHConnection) {
  return [
    { label: '✏️ 编辑', key: 'edit' },
    { label: '📋 复制地址', key: 'copy' },
    { label: '🔍 检查连通性', key: 'test' },
    { type: 'divider' as const, key: 'd1' },
    { label: '🗑 删除', key: 'delete' },
  ]
}

function handleSSHMenuSelect(key: string, conn: SSHConnection) {
  switch (key) {
    case 'edit':
      selectConnection(conn)
      break
    case 'copy':
      copyHost(conn)
      break
    case 'test':
      handleTestSSH(conn.id)
      break
    case 'delete':
      handleDeleteSSH(conn.id)
      break
  }
}

defineExpose({
  sidebarView,
  loadSSHConnections,
})

const sidebarViews = [
  { key: 'tools' as const, icon: '🔧', label: '工具列表' },
  { key: 'ssh' as const, icon: '🔗', label: 'SSH 服务器' },
  { key: 'history' as const, icon: '📋', label: '任务历史' },
  { key: 'export' as const, icon: '📦', label: '导出中心' },
]
</script>

<template>
  <aside
    class="flex shrink-0 flex-col border-r border-dracula-soft bg-[#1a1b26]"
    :style="{ width: props.width + 'px' }"
  >
    <div class="border-b border-dracula-soft p-3">
      <div class="relative">
        <span class="absolute left-2.5 top-1/2 -translate-y-1/2 text-xs text-slate-500">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="搜索工具... (Ctrl+F 收藏)"
          class="w-full rounded-md border border-dracula-soft bg-black/30 py-1.5 pl-8 pr-3 text-xs text-slate-300 placeholder-slate-600 outline-none transition focus:border-dracula-cyan/50"
        >
      </div>
    </div>

    <NScrollbar class="flex-1">
      <div
        v-if="sidebarView === 'tools'"
        class="space-y-1 p-2"
      >
        <div
          v-if="favoriteTools.length > 0"
          class="mb-2 border-b border-dracula-soft pb-2"
        >
          <button
            class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[11px] font-semibold uppercase tracking-wider text-dracula-pink transition hover:text-dracula-pink/70"
            @click="toggleCategory('__favorites__')"
          >
            <span
              class="text-[10px] transition-transform"
              :class="{ 'rotate-90': !collapsedCategories.has('__favorites__') }"
            >▶</span>
            ❤️ 收藏夹
            <span class="ml-auto text-[10px] text-slate-600">{{ favoriteTools.length }}</span>
          </button>
          <div
            v-if="!collapsedCategories.has('__favorites__')"
            class="ml-3 space-y-0.5"
          >
            <button
              v-for="tool in favoriteTools"
              :key="tool.id"
              class="group flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-left text-sm transition"
              :class="isToolActive(tool.id) ? 'bg-dracula-pink/10 text-dracula-pink' : 'text-slate-300 hover:bg-white/5'"
              @click="selectTool(tool)"
            >
              <span
                v-if="isToolRunning(tool.id)"
                class="h-2 w-2 shrink-0 rounded-full bg-dracula-green"
                title="正在运行"
              />
              <span
                v-else
                class="h-2 w-2 shrink-0 rounded-full border border-slate-700"
              />
              <span class="truncate">{{ tool.name }}</span>
              <n-tag
                :bordered="false"
                size="tiny"
                :type="kindTagType(tool.kind)"
                class="ml-auto shrink-0"
              >
                {{ tool.kind }}
              </n-tag>
            </button>
          </div>
        </div>

        <div
          v-if="recentToolList.length > 0"
          class="mb-2 border-b border-dracula-soft pb-2"
        >
          <button
            class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[11px] font-semibold uppercase tracking-wider text-dracula-yellow transition hover:text-dracula-yellow/70"
            @click="toggleCategory('__recent__')"
          >
            <span
              class="text-[10px] transition-transform"
              :class="{ 'rotate-90': !collapsedCategories.has('__recent__') }"
            >▶</span>
            ⭐ 最近使用
            <span class="ml-auto text-[10px] text-slate-600">{{ recentToolList.length }}</span>
          </button>
          <div
            v-if="!collapsedCategories.has('__recent__')"
            class="ml-3 space-y-0.5"
          >
            <button
              v-for="entry in recentToolList"
              :key="entry.tool.id"
              class="group flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-left text-sm transition"
              :class="isToolActive(entry.tool.id) ? 'bg-dracula-yellow/10 text-dracula-yellow' : 'text-slate-300 hover:bg-white/5'"
              @click="selectTool(entry.tool)"
            >
              <span
                v-if="isToolRunning(entry.tool.id)"
                class="h-2 w-2 shrink-0 rounded-full bg-dracula-green"
              />
              <span
                v-else
                class="h-2 w-2 shrink-0 rounded-full border border-slate-700"
              />
              <span class="truncate">
                {{ entry.tool.name }}
                <span class="ml-1 text-[10px] text-slate-600">{{ entry.args }}</span>
              </span>
              <n-tag
                :bordered="false"
                size="tiny"
                :type="kindTagType(entry.tool.kind)"
                class="ml-auto shrink-0"
              >
                {{ entry.tool.kind }}
              </n-tag>
            </button>
          </div>
        </div>

        <div
          v-for="group in groupedTools"
          :key="group.category"
        >
          <button
            class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-[11px] font-semibold uppercase tracking-wider text-slate-500 transition hover:text-slate-300"
            @click="toggleCategory(group.category)"
          >
            <span
              class="text-[10px] transition-transform"
              :class="{ 'rotate-90': !collapsedCategories.has(group.category) }"
            >▶</span>
            {{ group.category }}
            <span class="ml-auto text-[10px] text-slate-600">{{ group.tools.length }}</span>
          </button>
          <div
            v-if="!collapsedCategories.has(group.category)"
            class="mb-2 ml-3 space-y-0.5"
          >
            <button
              v-for="tool in group.tools"
              :key="tool.id"
              class="group flex w-full items-center gap-2 rounded-md px-3 py-1.5 text-left text-sm transition"
              :class="isToolActive(tool.id) ? 'bg-dracula-cyan/10 text-dracula-cyan' : 'text-slate-300 hover:bg-white/5'"
              @click="selectTool(tool)"
            >
              <span
                v-if="isToolRunning(tool.id)"
                class="h-2 w-2 shrink-0 rounded-full bg-dracula-green"
                title="正在运行"
              />
              <span
                v-else
                class="h-2 w-2 shrink-0 rounded-full border border-slate-700"
              />
              <span class="truncate">{{ tool.name }}</span>
              <n-tag
                :bordered="false"
                size="tiny"
                :type="kindTagType(tool.kind)"
                class="ml-auto shrink-0"
              >
                {{ tool.kind }}
              </n-tag>
            </button>
          </div>
        </div>
      </div>

      <div
        v-else-if="sidebarView === 'ssh'"
        class="flex h-full flex-col"
      >
        <div class="flex items-center justify-between px-3 py-2">
          <span class="text-xs font-medium text-slate-300">SSH 连接</span>
          <button
            class="rounded px-2 py-0.5 text-[11px] text-dracula-cyan transition hover:bg-dracula-cyan/10"
            @click="createConnection()"
          >
            + 新建
          </button>
        </div>

        <div
          v-if="sshConnections.length === 0"
          class="flex flex-1 items-center justify-center p-4"
        >
          <div class="text-center">
            <p class="text-xs text-slate-500">
              暂无 SSH 连接
            </p>
            <button
              class="mt-3 rounded-md border border-dracula-soft px-3 py-1.5 text-xs text-dracula-cyan transition hover:bg-dracula-cyan/10"
              @click="createConnection()"
            >
              + 新建第一个连接
            </button>
          </div>
        </div>

        <div class="space-y-1 p-2">
          <button
            v-for="conn in sshConnections"
            :key="conn.id"
            class="group flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left transition hover:bg-white/5"
            @click="selectConnection(conn)"
          >
            <span class="text-base">🖥</span>
            <div class="min-w-0 flex-1">
              <div class="truncate text-sm font-medium text-slate-200">
                {{ conn.name }}
              </div>
              <div class="truncate text-[10px] text-slate-500">
                {{ conn.user }}@{{ conn.host }}:{{ conn.port }}
              </div>
            </div>
            <NDropdown
              trigger="click"
              :options="sshDropdownOptions(conn)"
              @select="(key: string) => handleSSHMenuSelect(key, conn)"
            >
              <button
                class="shrink-0 rounded p-1 text-slate-600 opacity-0 transition hover:bg-white/10 hover:text-slate-300 group-hover:opacity-100"
                @click.stop
              >
                ···
              </button>
            </NDropdown>
          </button>
        </div>
      </div>

      <div
        v-else
        class="flex h-full items-center justify-center p-4 text-center"
      >
        <div class="text-xs text-slate-500">
          <div class="mb-2 text-lg">
            🚧
          </div>
          {{ sidebarView === 'history' ? '任务历史' : '导出中心' }}
          <br>
          <span class="text-slate-600">即将推出</span>
        </div>
      </div>
    </NScrollbar>

    <div class="flex items-center justify-around border-t border-dracula-soft px-3 py-2">
      <button
        v-for="view in sidebarViews"
        :key="view.key"
        class="rounded p-1.5 text-xs transition"
        :class="sidebarView === view.key ? 'text-dracula-cyan' : 'text-slate-500 hover:bg-white/5 hover:text-slate-300'"
        :title="view.label"
        @click="sidebarView = view.key"
      >
        {{ view.icon }}
      </button>
    </div>
  </aside>
</template>
