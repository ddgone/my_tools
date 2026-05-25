<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NInput, NInputNumber, NScrollbar, NTag } from 'naive-ui'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { DeleteSSHConnection, ListSSHConnections, SaveSSHConnection } from '../../wailsjs/go/main/App'
import type { SSHConnection, ToolManifest } from '@/types/workbench'

defineProps<{
  width: number
}>()

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const searchQuery = ref('')
const collapsedCategories = ref(new Set<string>())
const sidebarView = ref<'tools' | 'ssh' | 'history' | 'export'>('tools')

const sshConnections = ref<SSHConnection[]>([])
const sshFormVisible = ref(false)
const sshForm = ref<SSHConnection>({
  id: '',
  name: '',
  host: '',
  port: 22,
  user: 'root',
  authMethod: 'password',
  password: '',
  keyPath: '',
  description: '',
})

async function loadSSHConnections() {
  sshConnections.value = await ListSSHConnections()
}

async function saveSSHConnection() {
  await SaveSSHConnection(sshForm.value)
  sshFormVisible.value = false
  resetSSHForm()
  await loadSSHConnections()
}

async function removeSSHConnection(id: string) {
  await DeleteSSHConnection(id)
  await loadSSHConnections()
}

function resetSSHForm() {
  sshForm.value = {
    id: '',
    name: '',
    host: '',
    port: 22,
    user: 'root',
    authMethod: 'password',
    password: '',
    keyPath: '',
    description: '',
  }
}

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
    :style="{ width: width + 'px' }"
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
        class="space-y-2 p-2"
      >
        <div class="flex items-center justify-between">
          <span class="text-xs font-medium text-slate-300">SSH 连接</span>
          <button
            class="rounded px-2 py-0.5 text-[11px] text-slate-400 transition hover:bg-white/5 hover:text-slate-200"
            @click="sshFormVisible = !sshFormVisible; resetSSHForm()"
          >
            {{ sshFormVisible ? '取消' : '+ 新建' }}
          </button>
        </div>
        <div
          v-if="sshFormVisible"
          class="space-y-2 rounded-lg border border-dracula-soft bg-black/10 p-3"
        >
          <n-input
            v-model:value="sshForm.name"
            size="tiny"
            placeholder="连接名称"
          />
          <div class="flex gap-1">
            <n-input
              v-model:value="sshForm.host"
              size="tiny"
              placeholder="主机地址"
              class="flex-1"
            />
            <n-input-number
              v-model:value="sshForm.port"
              size="tiny"
              placeholder="22"
              :min="1"
              :max="65535"
              style="width: 70px"
            />
          </div>
          <n-input
            v-model:value="sshForm.user"
            size="tiny"
            placeholder="用户名"
          />
          <n-input
            v-model:value="sshForm.password"
            size="tiny"
            placeholder="密码"
            type="password"
          />
          <n-input
            v-model:value="sshForm.description"
            size="tiny"
            placeholder="备注（可选）"
          />
          <n-button
            size="tiny"
            type="primary"
            block
            @click="saveSSHConnection"
          >
            保存连接
          </n-button>
        </div>
        <div
          v-if="sshConnections.length === 0 && !sshFormVisible"
          class="py-8 text-center text-xs text-slate-600"
        >
          暂无SSH连接
        </div>
        <div
          v-for="conn in sshConnections"
          :key="conn.id"
          class="rounded-lg border border-dracula-soft bg-black/10 p-2.5 text-xs"
        >
          <div class="flex items-start justify-between gap-2">
            <div class="min-w-0">
              <div class="font-medium text-slate-200 truncate">
                {{ conn.name }}
              </div>
              <div class="mt-0.5 text-slate-500">
                {{ conn.user }}@{{ conn.host }}:{{ conn.port }}
              </div>
              <div
                v-if="conn.description"
                class="mt-0.5 text-slate-600"
              >
                {{ conn.description }}
              </div>
            </div>
            <button
              class="shrink-0 rounded px-1.5 py-0.5 text-[10px] text-slate-600 transition hover:bg-dracula-red/10 hover:text-dracula-red"
              @click="removeSSHConnection(conn.id)"
            >
              ✕
            </button>
          </div>
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
