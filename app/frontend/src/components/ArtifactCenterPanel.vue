<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NEmpty,
  NIcon,
  NInput,
  NInputNumber,
  NTag,
  NTooltip,
  useMessage,
} from 'naive-ui'
import {
  ArrowDownOutline,
  ArrowUpOutline,
  CaretDownOutline,
  CaretForwardOutline,
  DownloadOutline,
  FolderOpenOutline,
  HelpCircle,
  LayersOutline,
  RocketOutline,
  RefreshOutline,
  SearchOutline,
  StarOutline,
  TimeOutline,
  TrashOutline,
} from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useWorkspaceStore } from '@/stores/workspace'
import { artifactPlatforms, useArtifactCenterStore } from '@/stores/artifactCenter'
import type { ArtifactBatchTask, ToolManifest } from '@/types/workbench'
import { OpenFileDialog, OpenPath, OpenSaveFileDialog, SaveTextFile } from '../../wailsjs/go/main/App'

const workbench = useWorkbenchStore()
const workspace = useWorkspaceStore()
const artifactCenter = useArtifactCenterStore()
const message = useMessage()

type ToolFilterMode = 'all' | 'favorites' | 'recent'

const allTools = computed(() => workbench.bootstrap?.tools ?? [])
const goTools = computed(() => allTools.value.filter((tool) => tool.kind === 'go'))
const rustTools = computed(() => allTools.value.filter((tool) => tool.kind === 'rust'))
const pythonTools = computed(() => allTools.value.filter((tool) => tool.kind === 'python'))
const toolFilter = ref<ToolFilterMode>('all')
const toolSearch = ref('')
const selectedCount = computed(() => artifactCenter.selectedKeys.length)
const taskLabel = computed(() => artifactCenter.mode === 'build_cache' ? '批量构建缓存' : '批量导出')
const scrollContainerRef = ref<HTMLElement | null>(null)
const resultsSectionRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const scrollHeight = ref(0)
const clientHeight = ref(0)
const hasLongScrollableContent = computed(() => scrollHeight.value - clientHeight.value > 720)
const showJumpTop = computed(() => hasLongScrollableContent.value && scrollTop.value > 240)
const showJumpBottom = computed(() => hasLongScrollableContent.value && scrollHeight.value - clientHeight.value - scrollTop.value > 240)
const favoriteToolIdSet = computed(() => new Set(workspace.favorites))
const recentToolIdSet = computed(() => new Set(workspace.recentTools.map((entry) => entry.toolId)))
const filteredGoTools = computed(() => goTools.value.filter((tool) => matchesToolFilter(tool)))
const filteredRustTools = computed(() => rustTools.value.filter((tool) => matchesToolFilter(tool)))
const filteredPythonTools = computed(() => pythonTools.value.filter((tool) => matchesToolFilter(tool)))
const hasBinaryTools = computed(() => filteredGoTools.value.length > 0 || filteredRustTools.value.length > 0)
const estimatedCachedCount = computed(() => artifactCenter.estimate?.cachedCount ?? 0)
const estimatedBuildCount = computed(() => artifactCenter.estimate?.buildCount ?? 0)
let estimateTimer: number | null = null

onMounted(() => {
  artifactCenter.ensureSubscriptions()
  artifactCenter.ensureToolSelections(allTools.value)
  void artifactCenter.hydrate()
  window.addEventListener('resize', syncScrollMetrics)
  void nextTick(syncScrollMetrics)
  scheduleEstimateRefresh()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', syncScrollMetrics)
  if (estimateTimer) {
    window.clearTimeout(estimateTimer)
    estimateTimer = null
  }
})

watch(
  () => [artifactCenter.recentTasks.length, artifactCenter.expandedTaskId],
  () => {
    artifactCenter.ensureExpandedTask()
    void nextTick(syncScrollMetrics)
  },
  { immediate: true, flush: 'post' },
)

watch(
  () => [
    artifactCenter.selectedKeys.join('|'),
    artifactCenter.preferCache,
    artifactCenter.forceRebuild,
    artifactCenter.exportRootDir,
    artifactCenter.mode,
    artifactCenter.skipUnchanged,
  ],
  () => {
    scheduleEstimateRefresh()
  },
  { immediate: true },
)

function matchesToolFilter(tool: ToolManifest) {
  const query = toolSearch.value.trim().toLowerCase()
  if (query) {
    const haystacks = [
      tool.name,
      tool.id,
      tool.description,
      ...tool.category,
    ]
    const matched = haystacks.some((value) => value.toLowerCase().includes(query))
    if (!matched) {
      return false
    }
  }
  if (toolFilter.value === 'favorites') {
    return favoriteToolIdSet.value.has(tool.id)
  }
  if (toolFilter.value === 'recent') {
    return recentToolIdSet.value.has(tool.id)
  }
  return true
}

function scheduleEstimateRefresh() {
  if (estimateTimer) {
    window.clearTimeout(estimateTimer)
  }
  estimateTimer = window.setTimeout(() => {
    void artifactCenter.refreshEstimate()
  }, 240)
}

function selectedForTool(toolId: string) {
  return artifactPlatforms.filter((platform) => artifactCenter.isSelected(toolId, platform.key)).length
}

function isToolFullySelected(toolId: string) {
  return selectedForTool(toolId) === artifactPlatforms.length
}

function isToolPartiallySelected(toolId: string) {
  const count = selectedForTool(toolId)
  return count > 0 && count < artifactPlatforms.length
}

function selectedForPlatform(platformKey: string, tools: ToolManifest[]) {
  return tools.filter((tool) => artifactCenter.isSelected(tool.id, platformKey)).length
}

function isPlatformFullySelected(platformKey: string, tools: ToolManifest[]) {
  return tools.length > 0 && selectedForPlatform(platformKey, tools) === tools.length
}

function isPlatformPartiallySelected(platformKey: string, tools: ToolManifest[]) {
  const count = selectedForPlatform(platformKey, tools)
  return count > 0 && count < tools.length
}

async function pickExportRootDir() {
  try {
    const selected = await OpenFileDialog({
      title: '选择批量导出目录',
      filterName: '',
      filterGlob: '',
      directory: true,
      defaultDirectory: artifactCenter.exportRootDir,
      defaultFilename: '',
    })
    if (selected) {
      artifactCenter.exportRootDir = selected
    }
  } catch (error) {
    message.error(error instanceof Error ? error.message : '选择导出目录失败')
  }
}

async function handleStart() {
  if (selectedCount.value === 0) {
    message.warning('请先选择至少一个工具目标')
    return
  }
  if (artifactCenter.mode === 'export' && artifactCenter.exportRootDir.trim().length === 0) {
    message.warning('请先选择批量导出目录')
    return
  }
  try {
    await artifactCenter.startBatch()
    await scrollToResults()
    message.success(`${taskLabel.value}已启动`)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '启动批量产物任务失败')
  }
}

async function retryFailedItems(task: ArtifactBatchTask) {
  const failedItems = task.items
    .filter((item) => item.status === 'error')
    .map((item) => ({
      toolId: item.toolId,
      targetOS: item.targetOS,
      targetArch: item.targetArch,
    }))
  if (failedItems.length === 0) {
    message.warning('当前任务没有失败项可重试')
    return
  }
  try {
    await artifactCenter.startBatch({
      mode: task.mode,
      exportRootDir: task.exportRootDir ?? '',
      concurrency: task.concurrency,
      skipUnchanged: task.skipUnchanged,
      preferCache: task.preferCache,
      forceRebuild: task.forceRebuild,
      continueOnError: task.continueOnError,
      items: failedItems,
    })
    await scrollToResults()
    message.success('已重新发起失败项重试')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '重试失败项失败')
  }
}

async function clearTaskResults() {
  try {
    await artifactCenter.clearTasks()
    message.success('任务结果已清空')
    void nextTick(syncScrollMetrics)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '清空任务结果失败')
  }
}

function formatTaskSummary(task: ArtifactBatchTask) {
  const lines = [
    `# 产物任务摘要`,
    ``,
    `- 任务 ID: ${task.id}`,
    `- 模式: ${task.mode === 'build_cache' ? '批量构建缓存' : '批量导出'}`,
    `- 状态: ${task.status}`,
    `- 启动时间: ${new Date(task.startedAt).toLocaleString()}`,
    `- 结束时间: ${task.endedAt ? new Date(task.endedAt).toLocaleString() : '进行中'}`,
    `- 成功: ${task.successCount}`,
    `- 缓存命中: ${task.cachedCount}`,
    `- 跳过: ${task.skippedCount}`,
    `- 失败: ${task.errorCount}`,
    task.exportRootDir ? `- 导出目录: ${task.exportRootDir}` : '',
    task.exitMessage ? `- 结果说明: ${task.exitMessage}` : '',
    ``,
    `## 明细`,
    ``,
  ].filter(Boolean)

  for (const item of task.items) {
    lines.push(`- ${item.toolName} (${item.toolId}) ${item.targetOS}/${item.targetArch} | ${item.status} | ${item.message}`)
  }
  lines.push('')
  return lines.join('\n')
}

async function exportTaskSummary(task: ArtifactBatchTask) {
  try {
    const filePath = await OpenSaveFileDialog({
      title: '导出产物任务摘要',
      filterName: 'Markdown 文件',
      filterGlob: '*.md',
      directory: false,
      defaultDirectory: task.exportRootDir ?? artifactCenter.exportRootDir,
      defaultFilename: `artifact-task-${task.id}.md`,
    })
    if (!filePath) {
      return
    }
    await SaveTextFile(filePath, formatTaskSummary(task))
    message.success('任务摘要已导出')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '导出任务摘要失败')
  }
}

function syncScrollMetrics() {
  const container = scrollContainerRef.value
  if (!container) {
    return
  }
  scrollTop.value = container.scrollTop
  scrollHeight.value = container.scrollHeight
  clientHeight.value = container.clientHeight
}

function handlePanelScroll() {
  syncScrollMetrics()
}

async function scrollToResults() {
  await nextTick()
  resultsSectionRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  window.setTimeout(syncScrollMetrics, 250)
}

function scrollToTop() {
  scrollContainerRef.value?.scrollTo({ top: 0, behavior: 'smooth' })
}

function scrollToBottom() {
  const container = scrollContainerRef.value
  if (!container) {
    return
  }
  container.scrollTo({ top: container.scrollHeight, behavior: 'smooth' })
}

function isTaskExpanded(taskId: string) {
  return artifactCenter.expandedTaskId === taskId
}

function toggleTaskExpanded(taskId: string) {
  artifactCenter.setExpandedTask(artifactCenter.expandedTaskId === taskId ? null : taskId)
  void nextTick(syncScrollMetrics)
}

function openDirectory(path?: string) {
  if (!path) {
    return
  }
  void OpenPath(path)
}

function openContainingDirectory(path?: string) {
  if (!path) {
    return
  }
  const normalized = path.replace(/[\\/]+$/, '')
  const lastSep = Math.max(normalized.lastIndexOf('/'), normalized.lastIndexOf('\\'))
  if (lastSep < 0) {
    void OpenPath(normalized)
    return
  }
  const parent = /^[A-Za-z]:/.test(normalized) && lastSep === 2
    ? normalized.slice(0, lastSep + 1)
    : normalized.slice(0, lastSep)
  void OpenPath(parent || normalized)
}
</script>

<template>
  <div
    ref="scrollContainerRef"
    class="relative flex h-full flex-col gap-4 overflow-y-auto p-4"
    @scroll="handlePanelScroll"
  >
    <div class="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
      <NCard
        title="产物中心"
        size="small"
        :bordered="false"
        class="bg-[rgb(var(--color-bg-panel)/0.88)]"
      >
        <div class="grid gap-3 sm:grid-cols-6">
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2">
            <div class="text-xs text-dracula-soft">
              Go 工具
            </div>
            <div class="mt-1 text-lg font-semibold text-dracula-text">
              {{ filteredGoTools.length }}
            </div>
          </div>
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2">
            <div class="text-xs text-dracula-soft">
              Rust 工具
            </div>
            <div class="mt-1 text-lg font-semibold text-dracula-text">
              {{ filteredRustTools.length }}
            </div>
          </div>
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2">
            <div class="text-xs text-dracula-soft">
              Python 工具
            </div>
            <div class="mt-1 text-lg font-semibold text-dracula-text">
              {{ filteredPythonTools.length }}
            </div>
          </div>
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2">
            <div class="text-xs text-dracula-soft">
              已选目标
            </div>
            <div class="mt-1 text-lg font-semibold text-dracula-text">
              {{ selectedCount }}
            </div>
          </div>
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2">
            <div class="text-xs text-dracula-soft">
              预计命中缓存
            </div>
            <div class="mt-1 text-lg font-semibold text-dracula-text">
              {{ artifactCenter.estimating ? '计算中' : estimatedCachedCount }}
            </div>
          </div>
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2">
            <div class="text-xs text-dracula-soft">
              预计重新构建
            </div>
            <div class="mt-1 text-lg font-semibold text-dracula-text">
              {{ artifactCenter.estimating ? '计算中' : estimatedBuildCount }}
            </div>
          </div>
        </div>
        <div class="mt-3 grid gap-3 lg:grid-cols-2">
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-3 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.92)]">
            <div class="font-medium text-dracula-text">
              任务中心说明
            </div>
            <div class="mt-2">
              这里用于集中准备常用平台产物，适合在批量导出、远程执行之前，先把经常用到的平台缓存准备好。
            </div>
            <div class="mt-2 text-dracula-soft">
              下方任务结果会持续保留最近记录，方便回看哪些平台已成功、命中缓存或执行失败。
            </div>
          </div>
          <div class="rounded-lg border border-dashed border-dracula-cyan/20 bg-dracula-cyan/10 px-3 py-3 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.92)]">
            <div class="font-medium text-dracula-cyan">
              使用说明
            </div>
            <div class="mt-2">
              先用筛选按钮缩小范围，再在矩阵里勾选工具和平台，然后选择“批量构建缓存”或“批量导出”。
            </div>
            <div class="mt-2 text-dracula-soft">
              启动后会自动滚动到任务结果区；如果有失败项，可以直接重试，也可以导出摘要留档。
            </div>
          </div>
        </div>
      </NCard>

      <NCard
        title="批量配置"
        size="small"
        :bordered="false"
        class="bg-[rgb(var(--color-bg-panel)/0.88)]"
      >
        <div class="space-y-3 text-sm">
          <div class="flex flex-wrap gap-2">
            <NButton
              size="small"
              :type="artifactCenter.mode === 'build_cache' ? 'primary' : 'default'"
              @click="artifactCenter.mode = 'build_cache'"
            >
              <template #icon>
                <LayersOutline />
              </template>
              批量构建缓存
            </NButton>
            <NButton
              size="small"
              :type="artifactCenter.mode === 'export' ? 'primary' : 'default'"
              @click="artifactCenter.mode = 'export'"
            >
              <template #icon>
                <RocketOutline />
              </template>
              批量导出
            </NButton>
          </div>

          <div class="grid gap-3 sm:grid-cols-2">
            <label class="space-y-1">
              <span class="text-xs text-dracula-soft">并发数</span>
              <NInputNumber
                v-model:value="artifactCenter.concurrency"
                :min="1"
                :max="8"
                size="small"
              />
            </label>
            <div class="space-y-1">
              <span class="text-xs text-dracula-soft">导出根目录</span>
              <div class="flex gap-2">
                <NInput
                  v-model:value="artifactCenter.exportRootDir"
                  size="small"
                  :disabled="artifactCenter.mode !== 'export'"
                  placeholder="选择统一导出目录"
                />
                <NButton
                  size="small"
                  :disabled="artifactCenter.mode !== 'export'"
                  @click="pickExportRootDir"
                >
                  <template #icon>
                    <FolderOpenOutline />
                  </template>
                </NButton>
              </div>
            </div>
          </div>

          <div class="grid gap-2 text-sm">
            <NCheckbox v-model:checked="artifactCenter.skipUnchanged">
              <span class="inline-flex items-center gap-1.5">
                <span>跳过未变化项</span>
                <NTooltip
                  placement="top"
                  :style="{ maxWidth: '320px' }"
                >
                  <template #trigger>
                    <span
                      class="help-trigger"
                      tabindex="0"
                      @click.stop
                    >
                      <NIcon
                        :component="HelpCircle"
                        size="14"
                      />
                    </span>
                  </template>
                  构建输入未变化时直接跳过；导出模式下如果目标目录已经是最新产物，也会跳过写入。
                </NTooltip>
              </span>
            </NCheckbox>
            <NCheckbox v-model:checked="artifactCenter.preferCache">
              <span class="inline-flex items-center gap-1.5">
                <span>命中缓存直接复用</span>
                <NTooltip
                  placement="top"
                  :style="{ maxWidth: '320px' }"
                >
                  <template #trigger>
                    <span
                      class="help-trigger"
                      tabindex="0"
                      @click.stop
                    >
                      <NIcon
                        :component="HelpCircle"
                        size="14"
                      />
                    </span>
                  </template>
                  命中缓存后直接复用缓存产物，减少重复编译；关闭后仍可重新构建最新产物。
                </NTooltip>
              </span>
            </NCheckbox>
            <NCheckbox v-model:checked="artifactCenter.forceRebuild">
              强制重编
            </NCheckbox>
            <NCheckbox v-model:checked="artifactCenter.continueOnError">
              失败后继续
            </NCheckbox>
          </div>

          <div class="flex flex-wrap gap-2">
            <NButton
              size="small"
              secondary
              :disabled="filteredGoTools.length === 0"
              @click="artifactCenter.selectAllTargets(filteredGoTools)"
            >
              全选 Go 原生矩阵
            </NButton>
            <NButton
              size="small"
              secondary
              :disabled="filteredRustTools.length === 0"
              @click="artifactCenter.selectAllTargets(filteredRustTools)"
            >
              全选 Rust 矩阵
            </NButton>
            <NButton
              size="small"
              secondary
              @click="artifactCenter.clearSelections()"
            >
              清空选择
            </NButton>
            <NButton
              type="primary"
              size="small"
              :loading="artifactCenter.launching"
              @click="handleStart"
            >
              {{ taskLabel }}
            </NButton>
          </div>
        </div>
      </NCard>
    </div>

    <NAlert
      v-if="artifactCenter.error"
      type="error"
      :show-icon="false"
    >
      {{ artifactCenter.error }}
    </NAlert>

    <NCard
      size="small"
      :bordered="false"
      class="bg-[rgb(var(--color-bg-panel)/0.88)]"
    >
      <template #header>
        <div class="flex w-full flex-wrap items-center justify-between gap-3">
          <span class="shrink-0">工具 × 平台矩阵</span>
          <div class="ml-auto flex flex-wrap items-center justify-end gap-2">
            <div class="w-[312px] max-w-full shrink-0">
              <NInput
                v-model:value="toolSearch"
                size="small"
                clearable
                placeholder="搜索工具"
              >
                <template #prefix>
                  <NIcon :component="SearchOutline" />
                </template>
              </NInput>
            </div>
            <NButton
              size="small"
              :type="toolFilter === 'all' ? 'primary' : 'default'"
              @click="toolFilter = 'all'"
            >
              全部工具
            </NButton>
            <NButton
              size="small"
              :type="toolFilter === 'favorites' ? 'primary' : 'default'"
              @click="toolFilter = 'favorites'"
            >
              <template #icon>
                <StarOutline />
              </template>
              仅收藏
            </NButton>
            <NButton
              size="small"
              :type="toolFilter === 'recent' ? 'primary' : 'default'"
              @click="toolFilter = 'recent'"
            >
              <template #icon>
                <TimeOutline />
              </template>
              仅最近使用
            </NButton>
          </div>
        </div>
      </template>
      <div class="space-y-4">
        <div
          v-if="filteredGoTools.length > 0"
          class="rounded-xl border border-dracula-cyan/15 bg-dracula-cyan/10 p-3"
        >
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-semibold text-dracula-cyan">
                Go 原生矩阵
              </div>
              <div class="text-xs text-[rgb(var(--color-brand-primary)/0.75)]">
                Go 工具可视为原生能力，适合优先准备宿主平台和常用远程平台产物。
              </div>
            </div>
            <NTag
              size="small"
              :bordered="false"
              type="info"
            >
              {{ filteredGoTools.length }} 个 Go 工具
            </NTag>
          </div>
          <div class="overflow-x-auto rounded-lg">
            <table class="min-w-full border-collapse text-sm">
              <thead>
                <tr class="border-b border-[rgb(var(--color-border-subtle)/0.82)] text-[rgb(var(--color-fg-secondary)/0.92)]">
                  <th class="sticky left-0 z-10 min-w-[260px] bg-dracula-cyan/10 px-3 py-3 text-left">
                    工具
                  </th>
                  <th
                    v-for="platform in artifactPlatforms"
                    :key="`go-${platform.key}`"
                    class="min-w-[96px] px-2 py-3 text-center"
                  >
                    <div class="flex flex-col items-center gap-1">
                      <span>{{ platform.label }}</span>
                      <NCheckbox
                        :checked="isPlatformFullySelected(platform.key, filteredGoTools)"
                        :indeterminate="isPlatformPartiallySelected(platform.key, filteredGoTools)"
                        @update:checked="artifactCenter.setPlatformSelections(platform.key, $event, filteredGoTools)"
                      />
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="tool in filteredGoTools"
                  :key="tool.id"
                  class="border-b border-[rgb(var(--color-border-subtle)/0.46)]"
                >
                  <td class="sticky left-0 z-10 bg-dracula-cyan/10 px-3 py-3">
                    <div class="flex items-center justify-between gap-3">
                      <div>
                        <div class="font-medium text-dracula-text">
                          {{ tool.name }}
                        </div>
                        <div class="text-xs text-dracula-soft">
                          {{ tool.id }}
                        </div>
                      </div>
                      <NCheckbox
                        :checked="isToolFullySelected(tool.id)"
                        :indeterminate="isToolPartiallySelected(tool.id)"
                        @update:checked="artifactCenter.setToolSelections(tool.id, $event)"
                      />
                    </div>
                  </td>
                  <td
                    v-for="platform in artifactPlatforms"
                    :key="`${tool.id}-${platform.key}`"
                    class="px-2 py-3 text-center"
                  >
                    <NCheckbox
                      :checked="artifactCenter.isSelected(tool.id, platform.key)"
                      @update:checked="artifactCenter.setSelected(tool.id, platform.key, $event)"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div
          v-if="filteredRustTools.length > 0"
          class="rounded-xl border border-dracula-orange/20 bg-dracula-orange/10 p-3"
        >
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <div class="text-sm font-semibold text-dracula-orange">
                Rust 交叉编译矩阵
              </div>
              <div class="text-xs text-[rgb(var(--color-kind-rust)/0.78)]">
                Rust 工具本地运行走宿主内置二进制，这里主要准备导出、远程执行和缓存所需的交叉编译产物。
              </div>
            </div>
            <NTag
              size="small"
              :bordered="false"
              :color="{ color: 'rgb(var(--color-kind-rust)/0.14)', textColor: 'rgb(var(--color-kind-rust)/1)', borderColor: 'rgb(var(--color-kind-rust)/0.28)' }"
            >
              {{ filteredRustTools.length }} 个 Rust 工具
            </NTag>
          </div>
          <div class="overflow-x-auto rounded-lg">
            <table class="min-w-full border-collapse text-sm">
              <thead>
                <tr class="border-b border-[rgb(var(--color-border-subtle)/0.82)] text-[rgb(var(--color-fg-secondary)/0.92)]">
                  <th class="sticky left-0 z-10 min-w-[260px] bg-dracula-orange/10 px-3 py-3 text-left">
                    工具
                  </th>
                  <th
                    v-for="platform in artifactPlatforms"
                    :key="`rust-${platform.key}`"
                    class="min-w-[96px] px-2 py-3 text-center"
                  >
                    <div class="flex flex-col items-center gap-1">
                      <span>{{ platform.label }}</span>
                      <NCheckbox
                        :checked="isPlatformFullySelected(platform.key, filteredRustTools)"
                        :indeterminate="isPlatformPartiallySelected(platform.key, filteredRustTools)"
                        @update:checked="artifactCenter.setPlatformSelections(platform.key, $event, filteredRustTools)"
                      />
                    </div>
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="tool in filteredRustTools"
                  :key="tool.id"
                  class="border-b border-[rgb(var(--color-border-subtle)/0.46)]"
                >
                  <td class="sticky left-0 z-10 bg-dracula-orange/10 px-3 py-3">
                    <div class="flex items-center justify-between gap-3">
                      <div>
                        <div class="font-medium text-dracula-text">
                          {{ tool.name }}
                        </div>
                        <div class="text-xs text-dracula-soft">
                          {{ tool.id }}
                        </div>
                      </div>
                      <NCheckbox
                        :checked="isToolFullySelected(tool.id)"
                        :indeterminate="isToolPartiallySelected(tool.id)"
                        @update:checked="artifactCenter.setToolSelections(tool.id, $event)"
                      />
                    </div>
                  </td>
                  <td
                    v-for="platform in artifactPlatforms"
                    :key="`${tool.id}-${platform.key}`"
                    class="px-2 py-3 text-center"
                  >
                    <NCheckbox
                      :checked="artifactCenter.isSelected(tool.id, platform.key)"
                      @update:checked="artifactCenter.setSelected(tool.id, platform.key, $event)"
                    />
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <NEmpty
          v-if="!hasBinaryTools"
          description="当前筛选下没有 Go 或 Rust 二进制工具"
          class="py-6"
        />

        <div
          v-if="filteredPythonTools.length > 0"
          class="overflow-x-auto"
        >
          <table class="min-w-full border-collapse text-sm">
            <tbody>
              <tr
                v-for="tool in filteredPythonTools"
                :key="tool.id"
                class="border-b border-[rgb(var(--color-border-subtle)/0.46)]"
              >
                <td class="sticky left-0 z-10 bg-[rgb(var(--color-bg-panel)/0.88)] px-3 py-3">
                  <div class="flex items-center gap-2">
                    <div>
                      <div class="font-medium text-dracula-text">
                        {{ tool.name }}
                      </div>
                      <div class="text-xs text-dracula-soft">
                        {{ tool.id }}
                      </div>
                    </div>
                    <NTag
                      size="small"
                      :bordered="false"
                      type="warning"
                    >
                      仅脚本导出
                    </NTag>
                  </div>
                </td>
                <td
                  :colspan="artifactPlatforms.length"
                  class="px-3 py-3 text-left text-xs text-dracula-soft"
                >
                  当前产物中心只为 Go / Rust 提供跨平台二进制矩阵；Python 工具保留单工具脚本导出能力。
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </NCard>

    <div ref="resultsSectionRef">
      <NCard
        size="small"
        :bordered="false"
        class="bg-[rgb(var(--color-bg-panel)/0.88)]"
      >
        <template #header>
          <div class="flex items-center gap-1.5">
            <span>任务结果</span>
            <NTooltip
              placement="top"
              :style="{ maxWidth: '320px' }"
            >
              <template #trigger>
                <span
                  class="help-trigger"
                  tabindex="0"
                >
                  <NIcon
                    :component="HelpCircle"
                    size="14"
                  />
                </span>
              </template>
              当前结果区采用单展开模式，展开一项时会自动收起其他任务，便于快速查看最新结果。
            </NTooltip>
          </div>
        </template>
        <template #header-extra>
          <NButton
            size="small"
            tertiary
            :disabled="artifactCenter.recentTasks.length === 0"
            @click="clearTaskResults"
          >
            <template #icon>
              <TrashOutline />
            </template>
            清空结果
          </NButton>
        </template>
        <div
          v-if="artifactCenter.recentTasks.length === 0"
          class="py-6"
        >
          <NEmpty description="还没有批量产物任务" />
        </div>
        <div
          v-else
          class="space-y-3"
        >
          <div
            v-for="task in artifactCenter.recentTasks"
            :key="task.id"
            class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-3"
          >
            <div
              role="button"
              tabindex="0"
              class="flex w-full flex-wrap items-start justify-between gap-3 text-left"
              @click="toggleTaskExpanded(task.id)"
              @keyup.enter="toggleTaskExpanded(task.id)"
              @keyup.space.prevent="toggleTaskExpanded(task.id)"
            >
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <NIcon
                    :component="isTaskExpanded(task.id) ? CaretDownOutline : CaretForwardOutline"
                    size="14"
                    class="text-dracula-soft"
                  />
                  <span class="font-medium text-dracula-text">
                    {{ task.mode === 'build_cache' ? '批量构建缓存' : '批量导出' }}
                  </span>
                  <NTag
                    size="small"
                    :type="task.status === 'success' ? 'success' : task.status === 'failed' ? 'error' : task.status === 'partial' ? 'warning' : 'info'"
                    :bordered="false"
                  >
                    {{ task.status }}
                  </NTag>
                </div>
                <div class="mt-1 text-xs text-dracula-soft">
                  成功 {{ task.successCount }} / 缓存 {{ task.cachedCount }} / 跳过 {{ task.skippedCount }} / 失败 {{ task.errorCount }}
                </div>
                <div
                  v-if="task.exitMessage"
                  class="mt-1 text-xs text-[rgb(var(--color-fg-muted)/0.82)]"
                >
                  {{ task.exitMessage }}
                </div>
              </div>
              <div
                class="flex gap-2"
                @click.stop
              >
                <NButton
                  v-if="task.errorCount > 0"
                  size="tiny"
                  secondary
                  @click="retryFailedItems(task)"
                >
                  <template #icon>
                    <RefreshOutline />
                  </template>
                  重试失败项
                </NButton>
                <NButton
                  size="tiny"
                  secondary
                  @click="exportTaskSummary(task)"
                >
                  <template #icon>
                    <DownloadOutline />
                  </template>
                  导出摘要
                </NButton>
                <NButton
                  v-if="task.mode === 'export' && task.exportRootDir"
                  size="tiny"
                  secondary
                  @click="openDirectory(task.exportRootDir)"
                >
                  打开导出目录
                </NButton>
              </div>
            </div>

            <div
              v-if="isTaskExpanded(task.id)"
              class="mt-3 grid gap-2"
            >
              <div
                v-for="item in task.items"
                :key="item.key"
                class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-[rgb(var(--color-border-subtle)/0.72)] bg-[rgb(var(--color-bg-panel)/0.72)] px-3 py-2 text-xs"
              >
                <div class="min-w-0">
                  <div class="font-medium text-dracula-text">
                    {{ item.toolName }} · {{ item.targetOS }}/{{ item.targetArch }}
                  </div>
                  <div class="mt-1 truncate text-dracula-soft">
                    {{ item.message }}
                  </div>
                </div>
                <div class="flex items-center gap-2">
                  <NTag
                    size="small"
                    :bordered="false"
                    :type="item.status === 'success' ? 'success' : item.status === 'error' ? 'error' : item.status === 'cached' ? 'info' : 'warning'"
                  >
                    {{ item.status }}
                  </NTag>
                  <NButton
                    v-if="item.outputPath"
                    size="tiny"
                    tertiary
                    @click="openContainingDirectory(item.outputPath)"
                  >
                    打开
                  </NButton>
                </div>
              </div>
            </div>
          </div>
        </div>
      </NCard>
    </div>

    <div
      v-if="showJumpTop || showJumpBottom"
      class="pointer-events-none sticky bottom-4 z-20 -mt-16 flex justify-end pr-1"
    >
      <div class="pointer-events-auto flex flex-col gap-2">
        <NButton
          v-if="showJumpTop"
          circle
          type="primary"
          secondary
          @click="scrollToTop"
        >
          <template #icon>
            <NIcon :component="ArrowUpOutline" />
          </template>
        </NButton>
        <NButton
          v-if="showJumpBottom"
          circle
          type="primary"
          secondary
          @click="scrollToBottom"
        >
          <template #icon>
            <NIcon :component="ArrowDownOutline" />
          </template>
        </NButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.help-trigger {
  display: inline-flex;
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  align-items: center;
  justify-content: center;
  color: rgb(var(--color-fg-muted) / 0.9);
  cursor: pointer;
  transition:
    color 0.18s cubic-bezier(0.22, 1, 0.36, 1),
    opacity 0.18s cubic-bezier(0.22, 1, 0.36, 1);
}

.help-trigger:hover,
.help-trigger:focus-visible {
  color: rgb(var(--color-brand-primary) / 0.95);
  opacity: 1;
}
</style>
