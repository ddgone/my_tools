<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { NButton, NCard, NEmpty, NIcon, NTag, useMessage } from 'naive-ui'
import { DownloadOutline, FolderOpenOutline, RefreshOutline } from '@vicons/ionicons5'
import { useArtifactCenterStore } from '@/stores/artifactCenter'
import { useWorkspaceStore } from '@/stores/workspace'
import type { ArtifactBatchTask } from '@/types/workbench'
import { OpenPath, OpenSaveFileDialog, SaveTextFile } from '../../wailsjs/go/main/App'

const props = defineProps<{
  taskId: string
}>()

const artifactCenter = useArtifactCenterStore()
const workspace = useWorkspaceStore()
const message = useMessage()

const task = computed(() =>
  artifactCenter.tasks.find((entry) => entry.id === props.taskId)
  ?? artifactCenter.recentTasks.find((entry) => entry.id === props.taskId)
  ?? null,
)

onMounted(() => {
  artifactCenter.ensureSubscriptions()
  void artifactCenter.hydrate()
})

function openArtifactWorkbench() {
  workspace.openArtifactCenter()
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
      defaultDirectory: task.exportRootDir ?? '',
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
    message.success('已重新发起失败项重试')
  } catch (error) {
    message.error(error instanceof Error ? error.message : '重试失败项失败')
  }
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
  <div class="flex h-full flex-col overflow-y-auto p-4">
    <NCard
      v-if="task"
      size="small"
      :bordered="false"
      class="bg-[#151923]/90"
    >
      <template #header>
        <div class="flex flex-wrap items-center gap-2">
          <span class="text-base font-medium text-slate-100">
            {{ task.mode === 'build_cache' ? '批量构建缓存快照' : '批量导出快照' }}
          </span>
          <NTag
            size="small"
            :bordered="false"
            :type="task.status === 'success' ? 'success' : task.status === 'failed' ? 'error' : task.status === 'partial' ? 'warning' : 'info'"
          >
            {{ task.status }}
          </NTag>
        </div>
      </template>
      <template #header-extra>
        <NButton
          size="small"
          tertiary
          @click="openArtifactWorkbench"
        >
          返回产物工作台
        </NButton>
      </template>

      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-slate-400">
            启动时间
          </div>
          <div class="mt-1 text-sm text-slate-100">
            {{ new Date(task.startedAt).toLocaleString() }}
          </div>
        </div>
        <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-slate-400">
            执行汇总
          </div>
          <div class="mt-1 text-sm text-slate-100">
            成功 {{ task.successCount }} / 缓存 {{ task.cachedCount }} / 跳过 {{ task.skippedCount }} / 失败 {{ task.errorCount }}
          </div>
        </div>
        <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-slate-400">
            任务 ID
          </div>
          <div class="mt-1 break-all text-sm text-slate-100">
            {{ task.id }}
          </div>
        </div>
        <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
          <div class="text-xs text-slate-400">
            结果说明
          </div>
          <div class="mt-1 text-sm text-slate-100">
            {{ task.exitMessage || '无' }}
          </div>
        </div>
      </div>

      <div class="mt-4 flex flex-wrap gap-2">
        <NButton
          v-if="task.errorCount > 0"
          size="small"
          secondary
          @click="retryFailedItems(task)"
        >
          <template #icon>
            <NIcon :component="RefreshOutline" />
          </template>
          重试失败项
        </NButton>
        <NButton
          size="small"
          secondary
          @click="exportTaskSummary(task)"
        >
          <template #icon>
            <NIcon :component="DownloadOutline" />
          </template>
          导出摘要
        </NButton>
        <NButton
          v-if="task.mode === 'export' && task.exportRootDir"
          size="small"
          secondary
          @click="openDirectory(task.exportRootDir)"
        >
          <template #icon>
            <NIcon :component="FolderOpenOutline" />
          </template>
          打开导出目录
        </NButton>
      </div>

      <div class="mt-4 grid gap-2">
        <div
          v-for="item in task.items"
          :key="item.key"
          class="flex flex-wrap items-center justify-between gap-3 rounded-md border border-white/8 bg-black/10 px-3 py-2 text-xs"
        >
          <div class="min-w-0">
            <div class="font-medium text-slate-100">
              {{ item.toolName }} · {{ item.targetOS }}/{{ item.targetArch }}
            </div>
            <div class="mt-1 truncate text-slate-400">
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
    </NCard>

    <NCard
      v-else
      size="small"
      :bordered="false"
      class="bg-[#151923]/90"
    >
      <div class="py-12">
        <NEmpty description="这条任务记录已不存在或已被清理" />
        <div class="mt-4 flex justify-center">
          <NButton
            size="small"
            tertiary
            @click="openArtifactWorkbench"
          >
            打开产物工作台
          </NButton>
        </div>
      </div>
    </NCard>
  </div>
</template>
