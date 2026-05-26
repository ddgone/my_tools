<script setup lang="ts">
import { computed, ref, watch, h } from 'vue'
import {
  NButton,
  NCard,
  NDropdown,
  NIcon,
  NInput,
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
  TimeOutline,
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

const categoryPathStr = (tool: ToolManifest) =>
  tool.category.length > 0 ? tool.category.join(' > ') : '未分类'

const filteredTools = computed(() => {
  const query = searchQuery.value.trim().toLowerCase()
  if (!query) return allTools.value
  return allTools.value.filter(
    (t) =>
      t.name.toLowerCase().includes(query) ||
      t.description.toLowerCase().includes(query) ||
      t.category.some((c: string) => c.toLowerCase().includes(query)),
  )
})

function buildCategoryTree(tools: ToolManifest[]): TreeOption[] {
  const root: TreeOption[] = []

  for (const tool of tools) {
    const path = tool.category.length > 0 ? tool.category : ['未分类']
    insertIntoTree(root, path, tool, 0)
  }

  return root
}

function insertIntoTree(nodes: TreeOption[], path: string[], tool: ToolManifest, depth: number) {
  if (depth >= path.length) return

  const name = path[depth]
  let node = nodes.find((n) => n.label === name)

  if (!node) {
    node = {
      key: path.slice(0, depth + 1).join(' > '),
      label: name,
      children: [],
    }
    nodes.push(node)
  }

  if (depth === path.length - 1) {
    const toolNode: TreeOption & { tool?: ToolManifest } = {
      key: tool.id,
      label: tool.name,
      isLeaf: true,
      tool,
    }
    ;(node.children as TreeOption[]).push(toolNode)
  } else {
    insertIntoTree(node.children as TreeOption[], path, tool, depth + 1)
  }
}

const treeData = computed<TreeOption[]>(() => {
  return buildCategoryTree(filteredTools.value)
})

function collectAllKeys(nodes: TreeOption[]): string[] {
  const keys: string[] = []
  for (const node of nodes) {
    keys.push(node.key as string)
    if (node.children) {
      keys.push(...collectAllKeys(node.children as TreeOption[]))
    }
  }
  return keys
}

function collectBranchKeys(nodes: TreeOption[]): string[] {
  const keys: string[] = []
  for (const node of nodes) {
    if (node.children && node.children.length > 0) {
      keys.push(node.key as string)
      keys.push(...collectBranchKeys(node.children as TreeOption[]))
    }
  }
  return keys
}

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

function kindTagType(kind: string): 'success' | 'info' {
  return kind === 'python' ? 'success' : 'info'
}

function kindIcon(kind: string) {
  return kind === 'python' ? LogoPython : CodeSlash
}

function renderNodeLabel({ option }: { option: TreeOption & { tool?: ToolManifest } }) {
  const tool = option.tool
  if (!tool) {
    return h('span', { class: 'text-xs font-semibold uppercase tracking-wider text-slate-500' }, option.label as string)
  }
  return h('div', {
    class: 'flex items-center gap-x-2 w-full',
    onClick: (e: MouseEvent) => {
      e.stopPropagation()
      selectTool(tool)
    },
  }, [
    h('span', {
      class: `h-2 w-2 shrink-0 rounded-full ${isToolRunning(tool.id) ? 'bg-dracula-green' : 'border border-slate-700'}`,
    }),
    h('span', { class: 'truncate text-sm' }, tool.name),
    h(NTag, {
      bordered: false,
      size: 'tiny',
      type: kindTagType(tool.kind),
      class: 'ml-auto shrink-0',
    }, { default: () => tool.kind }),
  ])
}

const selectedKeys = ref<string[]>([])
const expandedKeys = ref<string[]>([])

watch([treeData, () => workspace.settings.autoExpandAll, searchQuery], ([data, autoExpandAll, query]) => {
  expandedKeys.value = autoExpandAll || query.trim()
    ? collectAllKeys(data)
    : collectBranchKeys(data)
}, { immediate: true })

watch(() => workspace.activeTab()?.toolId, (toolId) => {
  if (toolId) {
    selectedKeys.value = [toolId]
  }
})

function handleTreeSelect(keys: string[], _option: Array<TreeOption | null>) {
  if (keys.length === 0) return
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
    class="flex shrink-0 flex-col border-r border-white/15 bg-dracula-panel"
    :style="{ width: props.width + 'px' }"
  >
    <div
      v-if="activeView === 'tools'"
      class="p-3"
    >
      <NInput
        v-model:value="searchQuery"
        placeholder="搜索工具..."
        clearable
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
          class="p-2"
        >
          <NTree
            v-model:expanded-keys="expandedKeys"
            :data="treeData"
            :pattern="searchQuery"
            :selected-keys="selectedKeys"
            :render-label="renderNodeLabel"
            :indent="8"
            show-line
            block-line
            selectable
            class="category-tree"
            @update:selected-keys="handleTreeSelect"
          />
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
            class="space-y-1"
          >
            <NCard
              v-for="tool in favoriteTools"
              :key="tool.id"
              size="small"
              :bordered="true"
              hoverable
              class="ui-surface-hover"
              :content-style="{ padding: '8px 10px' }"
              :style="isToolActive(tool.id) ? { borderColor: 'rgba(255,121,198,0.45)', backgroundColor: 'rgba(255,121,198,0.06)' } : {}"
              @click="selectTool(tool)"
            >
              <div
                class="flex items-center gap-x-2 text-left"
                :class="isToolActive(tool.id) ? 'text-dracula-pink' : 'text-slate-300'"
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
                    {{ categoryPathStr(tool) }} · {{ tool.description }}
                  </div>
                </div>
              </div>
            </NCard>
          </div>
        </div>

        <div
          v-else-if="activeView === 'recent'"
          key="recent"
          class="p-2"
        >
          <div
            v-if="recentToolList.length === 0"
            class="flex flex-col items-center justify-center py-12 text-center"
          >
            <NIcon
              :component="TimeOutline"
              size="32"
              color="#44475a"
            />
            <p class="mt-2 text-xs text-slate-500">
              暂无最近使用
            </p>
            <p class="mt-1 text-[10px] text-slate-600">
              打开工具后自动记录
            </p>
          </div>
          <div
            v-else
            class="space-y-1"
          >
            <NCard
              v-for="entry in recentToolList"
              :key="entry.tool.id"
              size="small"
              :bordered="true"
              hoverable
              class="ui-surface-hover"
              :content-style="{ padding: '8px 10px' }"
              :style="isToolActive(entry.tool.id) ? { borderColor: 'rgba(241,250,140,0.45)', backgroundColor: 'rgba(241,250,140,0.06)' } : {}"
              @click="selectTool(entry.tool)"
            >
              <div
                class="flex items-center gap-x-2 text-left"
                :class="isToolActive(entry.tool.id) ? 'text-dracula-yellow' : 'text-slate-300'"
              >
                <NIcon
                  :component="kindIcon(entry.tool.kind)"
                  size="16"
                  :color="entry.tool.kind === 'python' ? '#f1fa8c' : '#8be9fd'"
                />
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm">
                    {{ entry.tool.name }}
                  </div>
                  <div class="truncate text-[10px] text-slate-500">
                    {{ entry.args || '无参数' }}
                  </div>
                </div>
              </div>
            </NCard>
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

          <div
            v-else
            class="space-y-1 overflow-y-auto px-2"
          >
            <NCard
              v-for="conn in sshConnections"
              :key="conn.id"
              size="small"
              :bordered="true"
              hoverable
              class="ui-surface-hover"
              :content-style="{ padding: '8px 10px' }"
              @click="selectConnection(conn)"
            >
              <div class="flex items-center gap-x-2 text-left text-slate-300">
                <NIcon
                  :component="ServerOutline"
                  size="16"
                  color="#8be9fd"
                />
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
                  <NButton
                    text
                    size="tiny"
                    @click.stop
                  >
                    <template #icon>
                      <NIcon :component="EllipsisHorizontal" />
                    </template>
                  </NButton>
                </NDropdown>
              </div>
            </NCard>
          </div>
        </div>
      </Transition>
    </NScrollbar>
  </aside>
</template>

<style>
.category-tree {
  --n-line-color: rgba(255,255,255,0.10);
  --n-line-offset-top: 4px;
  --n-line-offset-bottom: 4px;
}
.category-tree .n-tree-node-content {
  padding-top: 2px;
  padding-bottom: 2px;
}
</style>
