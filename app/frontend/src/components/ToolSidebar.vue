<script setup lang="ts">
import { computed, ref, watch, h, nextTick} from 'vue'
import {
  NButton,
  NCard,
  NDropdown,
  NIcon,
  NInput,
  NProgress,
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
  GlobeOutline,
  Play,
  LaptopOutline,
  OpenOutline,
  CloudUploadOutline,
} from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { useArtifactCenterStore } from '@/stores/artifactCenter'
import { DeleteSSHConnection, ListSSHConnections, TestSSHConnection } from '../../wailsjs/go/main/App'
import type { ArtifactBatchTask, SSHConnection, ToolManifest } from '@/types/workbench'
import type { ActivityBarView } from './ActivityBar.vue'
import WorkbenchContextMenu from './WorkbenchContextMenu.vue'
import BuiltinSidebarPanel from './BuiltinSidebarPanel.vue'
import { ANIM } from '@/utils/animation'
import { getExecutionTheme, getToolKindTheme } from '@/utils/executionTheme'
import gsap from 'gsap'
import { useTruncationTooltip } from '@/composables/useTruncationTooltip'

const props = defineProps<{
  width: number
  activeView: ActivityBarView
}>()

const emit = defineEmits<{
  selectConnection: [conn: SSHConnection]
  createConnection: [savedCount: number]
  deleteConnection: [id: string]
}>()

const message = useMessage()
const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()
const artifactCenter = useArtifactCenterStore()

const { tooltipText, tooltipX, tooltipY, tooltipShow, onEnter: onTooltipEnter, onLeave: onTooltipLeave } = useTruncationTooltip({ placement: 'right' })

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
    emit('deleteConnection', id)
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
  emit('createConnection', sshConnections.value.length)
}

function copyHost(conn: SSHConnection) {
  const text = `${conn.user}@${conn.host}:${conn.port}`
  navigator.clipboard.writeText(text)
  message.success('已复制: ' + text)
}

watch(() => props.activeView, (view) => {
  if (view === 'ssh') {
    loadSSHConnections()
    return
  }
  if (view === 'artifact') {
    artifactCenter.ensureSubscriptions()
    void artifactCenter.hydrate()
  }
}, { immediate: true })

const allTools = computed(() => workbench.bootstrap?.tools ?? [])
const activeSidebarModeTheme = computed(() =>
  getExecutionTheme(
    workspace.activeTabType === 'tool' ? workspace.activeToolTab?.toolId ? allTools.value.find((tool) => tool.id === workspace.activeToolTab?.toolId)?.kind : undefined : undefined,
    workspace.activeTabType === 'tool' ? workspace.activeToolTab?.executionTarget ?? 'local' : 'local',
  ),
)
const activeSidebarKindTheme = computed(() =>
  getToolKindTheme(
    workspace.activeTabType === 'tool'
      ? allTools.value.find((tool) => tool.id === workspace.activeToolTab?.toolId)?.kind
      : undefined,
  ),
)
const sidebarModeAccentSoftBg = computed(() => activeSidebarModeTheme.value.accentSoftBg)
const sidebarModeAccentSoftStrongBorder = computed(() => activeSidebarModeTheme.value.accentSoftStrongBorder)
const sidebarKindAccent = computed(() => activeSidebarKindTheme.value.accent)
const treeThemeOverrides = computed(() => ({
  nodeColorActive: activeSidebarModeTheme.value.accentSoftBg,
  nodeColorHover: 'transparent',
  nodeColorPressed: 'transparent',
}))

const categoryPathStr = (tool: ToolManifest) =>
  tool.category.length > 0 ? tool.category.join(' > ') : '未分类'

const topLevelCategoryOrder = ['通用测试工具', 'KD测试工具', 'Rust工具']

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

  sortTreeNodes(root)
  return root
}

function isBranchNode(node: TreeOption): boolean {
  return Array.isArray(node.children) && node.children.length > 0
}

function compareTreeNodes(a: TreeOption, b: TreeOption, depth: number): number {
  const aBranch = isBranchNode(a)
  const bBranch = isBranchNode(b)

  if (aBranch !== bBranch) {
    return aBranch ? -1 : 1
  }

  if (depth === 0) {
    const aPriority = topLevelCategoryOrder.indexOf(String(a.label))
    const bPriority = topLevelCategoryOrder.indexOf(String(b.label))
    const normalizedAPriority = aPriority === -1 ? Number.MAX_SAFE_INTEGER : aPriority
    const normalizedBPriority = bPriority === -1 ? Number.MAX_SAFE_INTEGER : bPriority
    if (normalizedAPriority !== normalizedBPriority) {
      return normalizedAPriority - normalizedBPriority
    }
  }

  return String(a.label).localeCompare(String(b.label), 'zh-CN')
}

function sortTreeNodes(nodes: TreeOption[], depth = 0) {
  nodes.sort((a, b) => compareTreeNodes(a, b, depth))
  for (const node of nodes) {
    if (Array.isArray(node.children) && node.children.length > 0) {
      sortTreeNodes(node.children as TreeOption[], depth + 1)
    }
  }
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

const runningArtifactTask = computed(() =>
  artifactCenter.recentTasks.find((task) => task.status === 'running') ?? null,
)

const historicalArtifactTasks = computed(() =>
  artifactCenter.recentTasks.filter((task) => task.id !== runningArtifactTask.value?.id),
)
const isCompactArtifactSidebar = computed(() => props.width < 272)

function isToolRunning(toolId: string) {
  return execution.tasks.some((t) => t.toolId === toolId && t.status === 'running')
}

function isArtifactCenterActive() {
  return workspace.activeTabType === 'artifact' && workspace.activeArtifactTab?.view === 'center'
}

function isArtifactSnapshotActive(taskId: string) {
  return workspace.activeTabType === 'artifact'
    && workspace.activeArtifactTab?.view === 'snapshot'
    && workspace.activeArtifactTab?.taskId === taskId
}

function artifactTaskTitle(task: ArtifactBatchTask) {
  return task.mode === 'build_cache' ? '批量构建缓存' : '批量导出'
}

function artifactTaskFinishedCount(task: ArtifactBatchTask) {
  return task.items.filter((item) =>
    item.endedAt || ['success', 'error', 'cached', 'skipped'].includes(item.status),
  ).length
}

function artifactTaskProgress(task: ArtifactBatchTask) {
  if (task.totalCount <= 0) {
    return 0
  }
  return Math.min(100, Math.round((artifactTaskFinishedCount(task) / task.totalCount) * 100))
}

function artifactTaskTime(task: ArtifactBatchTask) {
  return new Date(task.startedAt).toLocaleString()
}

function artifactSnapshotLabel(task: ArtifactBatchTask) {
  const timestamp = new Date(task.startedAt).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
  return `${artifactTaskTitle(task)} · ${timestamp}`
}

function openArtifactWorkbench() {
  workspace.openArtifactCenter()
}

function openArtifactTaskSnapshot(task: ArtifactBatchTask) {
  workspace.openArtifactSnapshot(task.id, artifactSnapshotLabel(task))
}

async function clearArtifactHistory() {
  if (runningArtifactTask.value) {
    message.warning('当前有产物任务正在执行，暂时不能清空历史')
    return
  }
  if (historicalArtifactTasks.value.length === 0) {
    return
  }
  try {
    await artifactCenter.clearTasks()
    message.success('产物历史已清空')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '清空产物历史失败')
  }
}

function isToolActive(toolId: string) {
  return workspace.activeTabType === 'tool' && workspace.activeToolTab?.toolId === toolId
}

function toolTabState(toolId: string) {
  return workspace.openTabs.find((tab) => tab.toolId === toolId)
}

function persistedToolState(toolId: string) {
  return workspace.getPersistedToolState(toolId)
}

function toolExecutionTarget(toolId: string) {
  return toolTabState(toolId)?.executionTarget ?? persistedToolState(toolId)?.executionTarget ?? 'local'
}

function isToolRemote(toolId: string) {
  return toolExecutionTarget(toolId) === 'remote'
}

function selectTool(tool: ToolManifest) {
  workspace.openTool(tool)
}

function toolExecutionTheme(tool: ToolManifest) {
  return getExecutionTheme(tool.kind, toolExecutionTarget(tool.id))
}

function toolThemeClass(tool: ToolManifest) {
  return `tool-theme-${toolExecutionTheme(tool).accentName}`
}

function handleNodeClickBehavior({ option }: { option: TreeOption & { tool?: ToolManifest } }) {
  return option.tool ? 'toggleSelect' : 'toggleExpand'
}

const toolContextMenuShow = ref(false)
const toolContextMenuX = ref(0)
const toolContextMenuY = ref(0)
const toolContextMenuTool = ref<ToolManifest | null>(null)
const sshContextMenuShow = ref(false)
const sshContextMenuX = ref(0)
const sshContextMenuY = ref(0)
const sshContextMenuConn = ref<SSHConnection | null>(null)

const toolContextMenuOptions = computed(() => {
  const tool = toolContextMenuTool.value
  if (!tool) return []
  return [
    { label: '打开工具', key: 'open', icon: OpenOutline },
    {
      label: workspace.isFavorite(tool.id) ? '取消收藏' : '收藏工具',
      key: 'favorite',
      icon: StarIcon,
    },
    { type: 'divider' as const, key: 'divider-actions' },
    { label: '本地运行', key: 'run-local', icon: LaptopOutline, hint: '占位' },
    { label: '远程运行', key: 'run-remote', icon: GlobeOutline, hint: '占位' },
    { label: '直接执行', key: 'run-now', icon: Play, hint: '占位' },
  ]
})

function closeToolContextMenu() {
  toolContextMenuShow.value = false
  toolContextMenuTool.value = null
}

function openToolContextMenu(event: MouseEvent, tool: ToolManifest) {
  event.preventDefault()
  event.stopPropagation()
  selectedKeys.value = [tool.id]
  toolContextMenuTool.value = tool
  toolContextMenuShow.value = false
  toolContextMenuX.value = event.clientX
  toolContextMenuY.value = event.clientY
  nextTick(() => {
    toolContextMenuShow.value = true
  })
}

function handleToolContextMenuSelect(key: string | number) {
  const tool = toolContextMenuTool.value
  closeToolContextMenu()
  if (!tool) return
  switch (String(key)) {
    case 'open':
      selectTool(tool)
      break
    case 'favorite':
      workspace.toggleFavorite(tool.id)
      message.success(workspace.isFavorite(tool.id) ? `已收藏 ${tool.name}` : `已取消收藏 ${tool.name}`)
      break
    case 'run-local':
      selectTool(tool)
      if (workspace.activeTabIndex >= 0) {
        workspace.setExecutionTarget(workspace.activeTabIndex, 'local')
      }
      message.info(`已切换到 ${tool.name} 的本地运行入口`)
      break
    case 'run-remote':
      selectTool(tool)
      if (workspace.activeTabIndex >= 0) {
        workspace.setExecutionTarget(workspace.activeTabIndex, 'remote')
      }
      message.info(`已切换到 ${tool.name} 的远程运行入口`)
      break
    case 'run-now':
      selectTool(tool)
      message.info(`已为 ${tool.name} 预留“直接执行”菜单动作，后续可接真正执行逻辑`)
      break
  }
}

const sshContextMenuOptions = computed(() => {
  const conn = sshContextMenuConn.value
  if (!conn) return []
  return sshDropdownOptions(conn)
})

function closeSSHContextMenu() {
  sshContextMenuShow.value = false
  sshContextMenuConn.value = null
}

function openSSHContextMenu(event: MouseEvent, conn: SSHConnection) {
  event.preventDefault()
  event.stopPropagation()
  sshContextMenuConn.value = conn
  sshContextMenuShow.value = false
  sshContextMenuX.value = event.clientX
  sshContextMenuY.value = event.clientY
  nextTick(() => {
    sshContextMenuShow.value = true
  })
}

function handleSSHContextMenuSelect(key: string | number) {
  const conn = sshContextMenuConn.value
  closeSSHContextMenu()
  if (!conn) return
  handleSSHMenuSelect(key, conn)
}

function treeNodeProps({ option }: { option: TreeOption & { tool?: ToolManifest } }) {
  const tool = option.tool
  if (!tool) {
    return {
      class: 'category-tree-node',
    }
  }
  return {
    class: ['tool-tree-node', toolThemeClass(tool)],
  }
}

function renderNodeLabel({ option }: { option: TreeOption & { tool?: ToolManifest } }) {
  const tool = option.tool
  if (!tool) {
    return h('span', {
      class: 'text-sm font-semibold uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.92)]',
    }, option.label as string)
  }
  return h('div', {
    class: 'flex items-center gap-x-2 w-full',
    onClick: (e: MouseEvent) => {
      e.stopPropagation()
      selectTool(tool)
    },
    onContextmenu: (e: MouseEvent) => {
      openToolContextMenu(e, tool)
    },
  }, [
    h('span', {
      class: `h-2 w-2 shrink-0 rounded-full transition-colors duration-150 ${isToolRunning(tool.id) ? 'bg-[rgb(var(--color-success)/0.92)]' : 'border border-[rgb(var(--color-border-strong)/0.9)]'}`,
    }),
    h('span', {
      class: 'truncate text-sm',
      style: {
        color: isToolActive(tool.id) ? 'rgb(var(--color-fg-base) / 0.98)' : 'rgb(var(--color-fg-base) / 0.94)',
      },
      onMouseenter: (e: MouseEvent) => onTooltipEnter(e, tool.name),
      onMouseleave: onTooltipLeave,
    }, tool.name),
    h('div', {
      class: 'ml-auto flex shrink-0 items-center gap-x-1',
    }, [
      ...(isToolRemote(tool.id)
        ? [h(NIcon, {
            component: GlobeOutline,
            size: 12,
            color: toolExecutionTheme(tool).accent,
            class: 'shrink-0',
          })]
        : []),
      ...(workspace.isFavorite(tool.id)
        ? [h(NIcon, {
            component: StarIcon,
            size: 12,
            color: 'rgb(var(--color-warning) / 1)',
            class: 'shrink-0',
          })]
        : []),
    ]),
  ])
}

const selectedKeys = ref<string[]>([])
const expandedKeys = ref<string[]>([])
const treeRenderKey = computed(() =>
  `${workspace.activeTabType}:${workspace.activeToolTab?.toolId ?? 'none'}:${workspace.activeToolTab?.executionTarget ?? 'local'}`,
)

const treeRef = ref<InstanceType<typeof NTree> | null>(null)

function handleExpandKeys(keys: string[]) {
  const prev = new Set(expandedKeys.value)
  expandedKeys.value = keys
  animateTreeNodes(prev)
}

async function animateTreeNodes(_prevKeys: Set<string>) {
  await nextTick()
  const treeEl = treeRef.value?.$el as HTMLElement | undefined
  if (!treeEl) return
  const nodes = treeEl.querySelectorAll<HTMLElement>('.n-tree-node-children > .n-tree-node')
  if (nodes.length === 0) return
  gsap.fromTo(nodes,
    { opacity: 0, y: -6 },
    { opacity: 1, y: 0, duration: ANIM.duration.normal, stagger: 0.025, ease: ANIM.ease.out, overwrite: 'auto' },
  )
}

watch([treeData, () => workspace.settings.autoExpandAll, searchQuery], ([data, autoExpandAll, query]) => {
  expandedKeys.value = autoExpandAll || query.trim()
    ? collectAllKeys(data)
    : collectBranchKeys(data)
}, { immediate: true })

watch([
  () => workspace.activeTabType,
  () => workspace.activeToolTab?.toolId,
], ([activeTabType, toolId]) => {
  if (activeTabType === 'tool' && toolId) {
    selectedKeys.value = [toolId]
  } else {
    selectedKeys.value = []
  }
}, { immediate: true })

watch(selectedKeys, () => {
  nextTick(() => {
    const treeEl = treeRef.value?.$el as HTMLElement | undefined
    if (!treeEl) return
    treeEl.dispatchEvent(new MouseEvent('mouseleave', { bubbles: true }))
  })
})

watch([() => props.activeView, searchQuery], () => {
  closeToolContextMenu()
  closeSSHContextMenu()
})

function handleTreeSelect(keys: string[], _option: Array<TreeOption | null>) {
  if (keys.length === 0) return
  const tool = allTools.value.find((item) => item.id === keys[0])
  if (tool) {
    selectTool(tool)
    return
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

function handleSSHMenuSelect(key: string | number, conn: SSHConnection) {
  switch (String(key)) {
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
  <div class="contents">
    <aside
      class="surface-divider flex shrink-0 flex-col border-r bg-[rgb(var(--color-bg-panel)/0.96)]"
      :style="{ width: props.width + 'px' }"
    >
      <NScrollbar
        class="flex-1"
      >
        <Transition
          name="slide"
          mode="out-in"
        >
          <div
            v-if="activeView === 'tools'"
            key="tools"
          >
            <div class="p-3">
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
            <div class="p-2">
              <NTree
                :key="treeRenderKey"
                ref="treeRef"
                :expanded-keys="expandedKeys"
                :data="treeData"
                :pattern="searchQuery"
                :selected-keys="selectedKeys"
                :render-label="renderNodeLabel"
                :node-props="treeNodeProps"
                :override-default-node-click-behavior="handleNodeClickBehavior"
                :indent="8"
                show-line
                block-line
                selectable
                class="category-tree"
                :theme-overrides="treeThemeOverrides"
                @update:selected-keys="handleTreeSelect"
                @update:expanded-keys="handleExpandKeys"
              />
            </div>
          </div>

          <div
            v-else-if="activeView === 'builtin'"
            key="builtin"
          >
            <BuiltinSidebarPanel />
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
                color="rgb(var(--color-fg-muted) / 0.72)"
              />
              <p class="mt-2 text-xs text-[rgb(var(--color-fg-muted)/0.92)]">
                暂无收藏的工具
              </p>
              <p class="mt-1 text-[10px] text-[rgb(var(--color-fg-muted)/0.78)]">
                使用右键菜单或 Ctrl+F 收藏工具
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
                class="ui-surface-hover sidebar-collection-card"
                :class="{ 'sidebar-collection-card-active': isToolActive(tool.id) }"
                :content-style="{ padding: '8px 10px' }"
                @click="selectTool(tool)"
                @contextmenu="openToolContextMenu($event, tool)"
              >
                <div
                  class="sidebar-collection-card-content flex items-center gap-x-2 text-left"
                  :class="{ 'sidebar-collection-card-content-active': isToolActive(tool.id) }"
                >
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm">
                      {{ tool.name }}
                    </div>
                    <div class="truncate text-[10px] text-[rgb(var(--color-fg-muted)/0.9)]">
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
                color="rgb(var(--color-fg-muted) / 0.72)"
              />
              <p class="mt-2 text-xs text-[rgb(var(--color-fg-muted)/0.92)]">
                暂无最近使用
              </p>
              <p class="mt-1 text-[10px] text-[rgb(var(--color-fg-muted)/0.78)]">
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
                class="ui-surface-hover sidebar-collection-card"
                :class="{ 'sidebar-collection-card-active': isToolActive(entry.tool.id) }"
                :content-style="{ padding: '8px 10px' }"
                @click="selectTool(entry.tool)"
                @contextmenu="openToolContextMenu($event, entry.tool)"
              >
                <div
                  class="sidebar-collection-card-content flex items-center gap-x-2 text-left"
                  :class="{ 'sidebar-collection-card-content-active': isToolActive(entry.tool.id) }"
                >
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm">
                      {{ entry.tool.name }}
                    </div>
                    <div class="truncate text-[10px] text-[rgb(var(--color-fg-muted)/0.9)]">
                      {{ entry.args || '无参数' }}
                    </div>
                  </div>
                </div>
              </NCard>
            </div>
          </div>

          <div
            v-else-if="activeView === 'artifact'"
            key="artifact"
            class="p-2"
          >
            <div class="space-y-3">
              <div>
                <div class="px-1 pb-2 text-[11px] font-medium uppercase tracking-[0.18em] text-[rgb(var(--color-fg-muted)/0.92)]">
                  产物工作台
                </div>
                <NCard
                  size="small"
                  :bordered="true"
                  hoverable
                  class="ui-surface-hover"
                  :content-style="{ padding: '12px' }"
                  :style="isArtifactCenterActive()
                    ? {
                      borderColor: 'rgb(var(--color-kind-rust) / 0.42)',
                      backgroundColor: 'rgb(var(--color-kind-rust) / 0.10)',
                    }
                    : {}"
                  @click="openArtifactWorkbench"
                >
                  <div
                    class="gap-3"
                    :class="isCompactArtifactSidebar ? 'flex-col' : 'flex items-start'"
                  >
                    <div
                      class="flex shrink-0 items-center justify-center rounded-lg bg-[rgb(var(--color-kind-rust)/0.12)] text-[rgb(var(--color-kind-rust)/0.96)]"
                      :class="isCompactArtifactSidebar ? 'h-8 w-8' : 'mt-0.5 h-9 w-9'"
                    >
                      <NIcon
                        :component="CloudUploadOutline"
                        :size="isCompactArtifactSidebar ? 16 : 18"
                      />
                    </div>
                    <div class="min-w-0 flex-1">
                      <div
                        class="gap-2"
                        :class="isCompactArtifactSidebar ? 'flex-col items-start' : 'flex items-center justify-between'"
                      >
                        <div
                          class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]"
                          :class="isCompactArtifactSidebar ? 'leading-5' : 'truncate'"
                        >
                          产物工作台
                        </div>
                        <NTag
                          size="small"
                          :bordered="false"
                          type="warning"
                        >
                          {{ artifactCenter.mode === 'build_cache' ? '缓存模式' : '导出模式' }}
                        </NTag>
                      </div>
                      <div class="mt-1 text-[11px] leading-5 text-[rgb(var(--color-fg-secondary)/0.9)]">
                        {{ isCompactArtifactSidebar
                          ? '打开后可继续配置矩阵、启动批量任务，并查看完整结果。'
                          : '打开工作台后可继续配置平台矩阵、启动批量任务，并查看完整结果列表。' }}
                      </div>
                      <div class="mt-3 flex flex-wrap gap-2 text-[10px] text-[rgb(var(--color-fg-muted)/0.92)]">
                        <span class="rounded border border-[rgb(var(--color-border-subtle)/0.54)] px-2 py-0.5">
                          已选 {{ artifactCenter.selectedKeys.length }} 项
                        </span>
                        <span class="rounded border border-[rgb(var(--color-border-subtle)/0.54)] px-2 py-0.5">
                          历史 {{ artifactCenter.recentTasks.length }} 条
                        </span>
                      </div>
                      <div
                        v-if="runningArtifactTask"
                        class="mt-3 rounded-lg border border-[rgb(var(--color-kind-rust)/0.18)] bg-[rgb(var(--color-kind-rust)/0.10)] px-2.5 py-2"
                      >
                        <div
                          class="gap-1.5 text-[11px]"
                          :class="isCompactArtifactSidebar ? 'flex-col items-start' : 'flex items-center justify-between'"
                        >
                          <span
                            class="text-[rgb(var(--color-warning)/0.98)]"
                            :class="isCompactArtifactSidebar ? 'leading-5' : 'truncate'"
                          >
                            正在执行 · {{ artifactTaskTitle(runningArtifactTask) }}
                          </span>
                          <span class="text-[rgb(var(--color-fg-muted)/0.92)]">
                            {{ artifactTaskFinishedCount(runningArtifactTask) }}/{{ runningArtifactTask.totalCount }}
                          </span>
                        </div>
                        <NProgress
                          class="mt-2"
                          type="line"
                          :percentage="artifactTaskProgress(runningArtifactTask)"
                          :height="6"
                          :show-indicator="false"
                          status="warning"
                          processing
                        />
                        <div
                          v-if="runningArtifactTask.currentItem"
                          class="mt-2 truncate text-[10px] text-[rgb(var(--color-fg-muted)/0.9)]"
                        >
                          {{ runningArtifactTask.currentItem }}
                        </div>
                      </div>
                    </div>
                  </div>
                </NCard>
              </div>

              <div>
                <div class="flex items-center justify-between gap-2 px-1 pb-2">
                  <div class="text-[11px] font-medium uppercase tracking-[0.18em] text-[rgb(var(--color-fg-muted)/0.92)]">
                    任务历史
                  </div>
                  <NButton
                    size="tiny"
                    quaternary
                    :disabled="historicalArtifactTasks.length === 0 || !!runningArtifactTask"
                    @click.stop="clearArtifactHistory"
                  >
                    清空历史
                  </NButton>
                </div>
                <div
                  v-if="historicalArtifactTasks.length === 0"
                  class="rounded-lg border border-dashed border-[rgb(var(--color-border-subtle)/0.52)] px-3 py-5 text-center text-xs leading-5 text-[rgb(var(--color-fg-muted)/0.92)]"
                >
                  暂无历史任务
                </div>
                <div
                  v-else
                  class="space-y-2"
                >
                  <NCard
                    v-for="task in historicalArtifactTasks"
                    :key="task.id"
                    size="small"
                    :bordered="true"
                    hoverable
                    class="ui-surface-hover"
                    :content-style="{ padding: '10px 12px' }"
                    :style="isArtifactSnapshotActive(task.id)
                      ? {
                        borderColor: 'rgb(var(--color-brand-primary) / 0.36)',
                        backgroundColor: 'rgb(var(--color-brand-primary) / 0.08)',
                      }
                      : {}"
                    @click="openArtifactTaskSnapshot(task)"
                  >
                    <div class="flex items-start gap-2">
                      <div class="min-w-0 flex-1">
                        <div
                          class="gap-2"
                          :class="isCompactArtifactSidebar ? 'flex-col items-start' : 'flex items-center justify-between'"
                        >
                          <div
                            class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]"
                            :class="isCompactArtifactSidebar ? 'leading-5' : 'truncate'"
                          >
                            {{ artifactTaskTitle(task) }}
                          </div>
                          <NTag
                            size="small"
                            :bordered="false"
                            :type="task.status === 'success' ? 'success' : task.status === 'failed' ? 'error' : task.status === 'partial' ? 'warning' : 'info'"
                          >
                            {{ task.status }}
                          </NTag>
                        </div>
                        <div class="mt-1 text-[10px] text-[rgb(var(--color-fg-muted)/0.9)]">
                          {{ artifactTaskTime(task) }}
                        </div>
                        <div class="mt-2 text-[11px] leading-5 text-[rgb(var(--color-fg-secondary)/0.9)]">
                          成功 {{ task.successCount }} / 缓存 {{ task.cachedCount }} / 跳过 {{ task.skippedCount }} / 失败 {{ task.errorCount }}
                        </div>
                      </div>
                    </div>
                  </NCard>
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
              <span class="text-xs font-medium text-[rgb(var(--color-fg-base)/0.98)]">SSH 连接</span>
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
                  color="rgb(var(--color-fg-muted) / 0.72)"
                />
                <p class="mt-2 text-xs text-[rgb(var(--color-fg-muted)/0.92)]">
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
                @contextmenu="openSSHContextMenu($event, conn)"
              >
                <div class="flex items-center gap-x-2 text-left text-[rgb(var(--color-fg-secondary)/0.92)]">
                  <NIcon
                    :component="ServerOutline"
                    size="16"
                    color="rgb(var(--color-brand-primary) / 1)"
                  />
                  <div class="min-w-0 flex-1">
                    <div class="truncate text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
                      {{ conn.name }}
                    </div>
                    <div class="truncate text-[10px] text-[rgb(var(--color-fg-muted)/0.9)]">
                      {{ conn.user }}@{{ conn.host }}:{{ conn.port }}
                    </div>
                  </div>
                  <NDropdown
                    trigger="click"
                    :options="sshDropdownOptions(conn)"
                    @select="key => handleSSHMenuSelect(key, conn)"
                  >
                    <NButton
                      class="hidden"
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

    <WorkbenchContextMenu
      :show="toolContextMenuShow"
      :x="toolContextMenuX"
      :y="toolContextMenuY"
      :title="toolContextMenuTool?.name ?? ''"
      :subtitle="toolContextMenuTool ? categoryPathStr(toolContextMenuTool) : ''"
      :items="toolContextMenuOptions"
      @select="handleToolContextMenuSelect"
      @close="closeToolContextMenu"
    />

    <WorkbenchContextMenu
      :show="sshContextMenuShow"
      :x="sshContextMenuX"
      :y="sshContextMenuY"
      :title="sshContextMenuConn?.name ?? ''"
      :subtitle="sshContextMenuConn ? `${sshContextMenuConn.user}@${sshContextMenuConn.host}:${sshContextMenuConn.port}` : ''"
      :items="sshContextMenuOptions"
      @select="handleSSHContextMenuSelect"
      @close="closeSSHContextMenu"
    />

    <Teleport to="body">
      <div
        v-if="tooltipShow"
        class="workbench-tooltip pointer-events-none fixed z-[100] -translate-y-1/2 px-2.5 py-1.5 text-xs"
        :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }"
      >
        <div class="workbench-tooltip-arrow absolute -left-1 top-1/2 h-2 w-2 -translate-y-1/2 rotate-45" />
        {{ tooltipText }}
      </div>
    </Teleport>
  </div>
</template>

<style>
.category-tree {
  --n-node-color-active: v-bind(sidebarModeAccentSoftBg);
  --n-line-color: rgb(var(--color-border-subtle) / 0.78);
  --n-line-offset-top: 4px;
  --n-line-offset-bottom: 4px;
}

.sidebar-collection-card-active {
  border-color: v-bind(sidebarModeAccentSoftStrongBorder) !important;
  background-color: v-bind(sidebarModeAccentSoftBg) !important;
}

.sidebar-collection-card-content-active {
  color: v-bind(sidebarKindAccent);
}
.category-tree .n-tree-node-content {
  padding-top: 2px;
  padding-bottom: 2px;
}
.category-tree .n-tree-node-switcher {
  width: 26px;
  height: 26px;
  margin-right: -2px;
}
.category-tree .n-tree-node-switcher__icon {
  width: 16px;
  height: 16px;
}
.category-tree .n-tree-node-switcher svg {
  fill: none !important;
  stroke: rgb(var(--color-fg-muted) / 0.78);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.category-tree .n-tree-node-switcher:hover svg {
  stroke: rgb(var(--color-fg-secondary) / 0.94);
}
.category-tree .n-tree-node--expanded > .n-tree-node-switcher svg {
  stroke: v-bind(sidebarModeAccentSoftStrongBorder);
}
.category-tree .n-tree-node--selected .n-tree-node-indent {
  opacity: 0.3;
}
.category-tree .n-tree-node--selected > .n-tree-node-indent:last-of-type {
  opacity: 1;
}
.category-tree .n-tree-node--selected > .n-tree-node-indent:last-of-type svg line {
  stroke: v-bind(sidebarModeAccentSoftStrongBorder) !important;
}
.category-tree .tool-tree-node.tool-theme-local:hover:not(.n-tree-node--selected):not(.n-tree-node--disabled) {
  background: rgb(var(--color-mode-local) / 0.06) !important;
}
.category-tree .tool-tree-node.tool-theme-local.n-tree-node--selectable:active:not(.n-tree-node--selected):not(.n-tree-node--disabled) {
  background: rgb(var(--color-mode-local) / 0.09) !important;
}
.category-tree .tool-tree-node.tool-theme-remote:hover:not(.n-tree-node--selected):not(.n-tree-node--disabled) {
  background: rgb(var(--color-mode-remote) / 0.06) !important;
}
.category-tree .tool-tree-node.tool-theme-remote.n-tree-node--selectable:active:not(.n-tree-node--selected):not(.n-tree-node--disabled) {
  background: rgb(var(--color-mode-remote) / 0.09) !important;
}
.category-tree .category-tree-node:hover:not(.n-tree-node--selected):not(.n-tree-node--disabled) {
  background: rgb(var(--color-fg-base) / 0.05) !important;
}
.category-tree .category-tree-node.n-tree-node--clickable:active:not(.n-tree-node--selected):not(.n-tree-node--disabled) {
  background: rgb(var(--color-fg-base) / 0.08) !important;
}
</style>
