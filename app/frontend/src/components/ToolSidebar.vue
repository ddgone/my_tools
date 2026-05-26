<script setup lang="ts">
import { computed, ref, watch, h } from 'vue'
import {
  NButton,
  NCollapse,
  NCollapseItem,
  NDropdown,
  NIcon,
  NInput,
  NList,
  NListItem,
  NScrollbar,
  NTag,
  NTree,
  useMessage,
  type TreeOption,
} from 'naive-ui'
import {
  Search,
  Add,
  ServerOutline,
  EllipsisHorizontal,
  Star as StarIcon,
  CodeSlash,
  LogoPython,
} from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { DeleteSSHConnection, ListSSHConnections, TestSSHConnection } from '../../wailsjs/go/main/App'
import type { SSHConnection, ToolManifest } from '@/types/workbench'
import type { ActivityBarView } from './ActivityBar.vue'

const props = defineProps<{
  width: number
  activeView: ActivityBarView
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

watch(() => props.activeView, (view) => {
  if (view === 'ssh') {
    loadSSHConnections()
  }
}, { immediate: true })

const allTools = computed(() => workbench.bootstrap?.tools ?? [])

const filteredTools = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return allTools.value
  return allTools.value.filter(
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
  return workspace.favorites.map((id) => allTools.value.find((t) => t.id === id)).filter(Boolean) as ToolManifest[]
})

const recentToolList = computed(() => {
  return workspace.recentTools
    .map((r) => {
      const tool = allTools.value.find((t) => t.id === r.toolId)
      if (!tool) return null
      return { tool, args: r.args }
    })
    .filter(Boolean) as { tool: ToolManifest; args: string }[]
})

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

function kindIcon(kind: string) {
  return kind === 'python' ? LogoPython : CodeSlash
}

const treeData = computed<TreeOption[]>(() => {
  return groupedTools.value.map((group) => ({
    key: group.category,
    label: group.category,
    children: group.tools.map((tool) => ({
      key: tool.id,
      label: tool.name,
      isLeaf: true,
      tool: tool,
    })),
  }))
})

function renderNodeLabel({ option }: { option: TreeOption & { tool?: ToolManifest } }) {
  const tool = option.tool
  if (!tool) {
    return h('span', { class: 'text-xs font-semibold uppercase tracking-wider text-slate-500' }, option.label as string)
  }
  return h('div', { class: 'flex items-center gap-x-2 w-full' }, [
    h('span', {
      class: `h-2 w-2 shrink-0 rounded-full ${isToolRunning(tool.id) ? 'bg-dracula-green' : 'border border-slate-700'}`,
    }),
    h('span', { class: 'truncate text-sm' }, tool.name),
    h(NTag, {
      bordered: false,
      size: 'tiny',
      type: kindTagType(tool.kind) as any,
      class: 'ml-auto shrink-0',
    }, { default: () => tool.kind }),
  ])
}

const selectedKeys = ref<string[]>([])

watch(() => workspace.activeTab()?.toolId, (toolId) => {
  if (toolId) {
    selectedKeys.value = [toolId]
  }
})

function handleTreeSelect(keys: string[]) {
  if (keys.length === 0) return
  const toolId = keys[0]
  const tool = allTools.value.find((t) => t.id === toolId)
  if (tool) {
    selectTool(tool)
  }
  selectedKeys.value = keys
}

function sshDropdownOptions(_conn: SSHConnection) {
  return [
    { label: '编辑', key: 'edit' },
    { label: '复制地址', key: 'copy' },
    { label: '检查连通性', key: 'test' },
    { type: 'divider' as const, key: 'd1' },
    { label: '删除', key: 'delete' },
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
  loadSSHConnections,
})
</script>

<template>
  <aside
    class="flex shrink-0 flex-col border-r border-dracula-soft bg-dracula-panel"
    :style="{ width: props.width + 'px' }"
  >
    <div
      v-if="activeView === 'tools' || activeView === 'favorites'"
      class="p-3"
    >
      <NInput
        v-model:value="searchQuery"
        placeholder="搜索工具..."
        clearable
        round
        size="small"
      >
        <template #prefix>
          <NIcon :component="Search" />
        </template>
      </NInput>
    </div>

    <NScrollbar class="flex-1">
      <Transition
        name="slide"
        mode="out-in"
      >
        <div
          v-if="activeView === 'tools'"
          key="tools"
          class="space-y-1 p-2"
        >
          <NCollapse
            v-if="favoriteTools.length > 0"
            :default-expanded-names="['favorites']"
          >
            <NCollapseItem name="favorites">
              <template #header>
                <span class="text-xs font-semibold uppercase tracking-wider text-dracula-pink">
                  收藏夹 · {{ favoriteTools.length }}
                </span>
              </template>
              <div class="-mx-1 space-y-0.5">
                <div
                  v-for="tool in favoriteTools"
                  :key="tool.id"
                  class="group flex cursor-pointer items-center gap-x-2 rounded-md px-3 py-1.5 text-left transition"
                  :class="isToolActive(tool.id) ? 'bg-dracula-pink/10 text-dracula-pink' : 'text-slate-300 hover:bg-white/5'"
                  @click="selectTool(tool)"
                >
                  <span
                    v-if="isToolRunning(tool.id)"
                    class="h-2 w-2 shrink-0 rounded-full bg-dracula-green"
                  />
                  <span
                    v-else
                    class="h-2 w-2 shrink-0 rounded-full border border-slate-700"
                  />
                  <span class="truncate text-sm">{{ tool.name }}</span>
                  <NTag
                    :bordered="false"
                    size="tiny"
                    :type="kindTagType(tool.kind) as any"
                    class="ml-auto shrink-0"
                  >
                    {{ tool.kind }}
                  </NTag>
                </div>
              </div>
            </NCollapseItem>
          </NCollapse>

          <NCollapse
            v-if="recentToolList.length > 0"
            :default-expanded-names="['recent']"
          >
            <NCollapseItem name="recent">
              <template #header>
                <span class="text-xs font-semibold uppercase tracking-wider text-dracula-yellow">
                  最近使用 · {{ recentToolList.length }}
                </span>
              </template>
              <div class="-mx-1 space-y-0.5">
                <div
                  v-for="entry in recentToolList"
                  :key="entry.tool.id"
                  class="group flex cursor-pointer items-center gap-x-2 rounded-md px-3 py-1.5 text-left transition"
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
                  <span class="truncate text-sm">
                    {{ entry.tool.name }}
                    <span class="ml-1 text-[10px] text-slate-600">{{ entry.args }}</span>
                  </span>
                  <NTag
                    :bordered="false"
                    size="tiny"
                    :type="kindTagType(entry.tool.kind) as any"
                    class="ml-auto shrink-0"
                  >
                    {{ entry.tool.kind }}
                  </NTag>
                </div>
              </div>
            </NCollapseItem>
          </NCollapse>

          <div class="border-t border-dracula-soft pt-2">
            <NTree
              :data="treeData"
              :pattern="searchQuery"
              :render-label="renderNodeLabel"
              :selected-keys="selectedKeys"
              :expanded-keys="groupedTools.map((g) => g.category)"
              block-line
              selectable
              class="n-tree-custom"
              @update:selected-keys="handleTreeSelect"
            />
          </div>
        </div>

        <div
          v-else-if="activeView === 'favorites'"
          key="favorites"
          class="p-2"
        >
          <div
            v-if="favoriteTools.length === 0"
            class="flex flex-col items-center justify-center py-12 text-center"
          >
            <NIcon
              :component="StarIcon"
              size="32"
              color="#44475a"
            />
            <p class="mt-2 text-xs text-slate-500">
              暂无收藏的工具
            </p>
            <p class="mt-1 text-[10px] text-slate-600">
              使用 Ctrl+F 收藏工具
            </p>
          </div>
          <div
            v-else
            class="space-y-0.5"
          >
            <div
              v-for="tool in favoriteTools"
              :key="tool.id"
              class="group flex cursor-pointer items-center gap-x-2 rounded-md px-3 py-2 text-left transition"
              :class="isToolActive(tool.id) ? 'bg-dracula-pink/10 text-dracula-pink' : 'text-slate-300 hover:bg-white/5'"
              @click="selectTool(tool)"
            >
              <NIcon
                :component="kindIcon(tool.kind)"
                size="16"
                :color="tool.kind === 'python' ? '#f1fa8c' : '#8be9fd'"
              />
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm">
                  {{ tool.name }}
                </div>
                <div class="truncate text-[10px] text-slate-500">
                  {{ tool.category }} · {{ tool.description }}
                </div>
              </div>
            </div>
          </div>
        </div>

        <div
          v-else-if="activeView === 'ssh'"
          key="ssh"
          class="flex h-full flex-col"
        >
          <div class="flex items-center justify-between px-3 py-2">
            <span class="text-xs font-medium text-slate-300">SSH 连接</span>
            <NButton
              text
              size="tiny"
              type="primary"
              @click="createConnection()"
            >
              <template #icon>
                <NIcon :component="Add" />
              </template>
              新建
            </NButton>
          </div>

          <div
            v-if="sshConnections.length === 0"
            class="flex flex-1 items-center justify-center p-4"
          >
            <div class="text-center">
              <NIcon
                :component="ServerOutline"
                size="28"
                color="#44475a"
              />
              <p class="mt-2 text-xs text-slate-500">
                暂无 SSH 连接
              </p>
              <NButton
                size="tiny"
                class="mt-3"
                @click="createConnection()"
              >
                新建第一个连接
              </NButton>
            </div>
          </div>

          <NList
            v-else
            hoverable
            clickable
            class="flex-1"
          >
            <NListItem
              v-for="conn in sshConnections"
              :key="conn.id"
              @click="selectConnection(conn)"
            >
              <template #prefix>
                <NIcon
                  :component="ServerOutline"
                  size="16"
                  color="#8be9fd"
                />
              </template>
              <div class="min-w-0 flex-1">
                <div class="truncate text-sm font-medium text-slate-200">
                  {{ conn.name }}
                </div>
                <div class="truncate text-[10px] text-slate-500">
                  {{ conn.user }}@{{ conn.host }}:{{ conn.port }}
                </div>
              </div>
              <template #suffix>
                <NDropdown
                  trigger="click"
                  :options="sshDropdownOptions(conn)"
                  @select="(key: string) => handleSSHMenuSelect(key, conn)"
                >
                  <NButton
                    text
                    size="tiny"
                    class="opacity-0 group-hover:opacity-100"
                    @click.stop
                  >
                    <template #icon>
                      <NIcon :component="EllipsisHorizontal" />
                    </template>
                  </NButton>
                </NDropdown>
              </template>
            </NListItem>
          </NList>
        </div>
      </Transition>
    </NScrollbar>
  </aside>
</template>

<style>
.n-tree-custom .n-tree-node-content {
  padding-top: 2px;
  padding-bottom: 2px;
}
</style>
