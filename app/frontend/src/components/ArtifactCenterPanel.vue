<script setup lang="ts">
import { computed, onMounted } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NEmpty,
  NInput,
  NInputNumber,
  NTag,
  useMessage,
} from 'naive-ui'
import { FolderOpenOutline, LayersOutline, RocketOutline } from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { artifactPlatforms, useArtifactCenterStore } from '@/stores/artifactCenter'
import { OpenFileDialog, OpenPath } from '../../wailsjs/go/main/App'

const workbench = useWorkbenchStore()
const artifactCenter = useArtifactCenterStore()
const message = useMessage()

const allTools = computed(() => workbench.bootstrap?.tools ?? [])
const goTools = computed(() => allTools.value.filter((tool) => tool.kind === 'go'))
const pythonTools = computed(() => allTools.value.filter((tool) => tool.kind !== 'go'))
const selectedCount = computed(() => artifactCenter.selectedKeys.length)
const taskLabel = computed(() => artifactCenter.mode === 'build_cache' ? '批量构建缓存' : '批量导出')

onMounted(() => {
  artifactCenter.ensureSubscriptions()
  artifactCenter.ensureToolSelections(allTools.value)
  void artifactCenter.hydrate()
})

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

function selectedForPlatform(platformKey: string) {
  return goTools.value.filter((tool) => artifactCenter.isSelected(tool.id, platformKey)).length
}

function isPlatformFullySelected(platformKey: string) {
  return goTools.value.length > 0 && selectedForPlatform(platformKey) === goTools.value.length
}

function isPlatformPartiallySelected(platformKey: string) {
  const count = selectedForPlatform(platformKey)
  return count > 0 && count < goTools.value.length
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
    message.success(`${taskLabel.value}已启动`)
  } catch (error) {
    message.error(error instanceof Error ? error.message : '启动批量产物任务失败')
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
  <div class="flex h-full flex-col gap-4 overflow-y-auto p-4">
    <div class="grid gap-4 xl:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)]">
      <NCard
        title="产物中心"
        size="small"
        :bordered="false"
        class="bg-[#151923]/90"
      >
        <div class="grid gap-3 sm:grid-cols-4">
          <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
            <div class="text-xs text-slate-400">
              Go 工具
            </div>
            <div class="mt-1 text-lg font-semibold text-slate-100">
              {{ goTools.length }}
            </div>
          </div>
          <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
            <div class="text-xs text-slate-400">
              Python 工具
            </div>
            <div class="mt-1 text-lg font-semibold text-slate-100">
              {{ pythonTools.length }}
            </div>
          </div>
          <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
            <div class="text-xs text-slate-400">
              已选目标
            </div>
            <div class="mt-1 text-lg font-semibold text-slate-100">
              {{ selectedCount }}
            </div>
          </div>
          <div class="rounded-lg border border-white/10 bg-white/5 px-3 py-2">
            <div class="text-xs text-slate-400">
              最近任务
            </div>
            <div class="mt-1 text-lg font-semibold text-slate-100">
              {{ artifactCenter.recentTasks.length }}
            </div>
          </div>
        </div>
      </NCard>

      <NCard
        title="批量配置"
        size="small"
        :bordered="false"
        class="bg-[#151923]/90"
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
              <span class="text-xs text-slate-400">并发数</span>
              <NInputNumber
                v-model:value="artifactCenter.concurrency"
                :min="1"
                :max="8"
                size="small"
              />
            </label>
            <label class="space-y-1">
              <span class="text-xs text-slate-400">导出根目录</span>
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
            </label>
          </div>

          <div class="grid gap-2 text-sm">
            <NCheckbox v-model:checked="artifactCenter.skipUnchanged">
              跳过未变化项
            </NCheckbox>
            <NCheckbox v-model:checked="artifactCenter.preferCache">
              命中缓存直接复用
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
              @click="artifactCenter.selectAllGoTargets(goTools)"
            >
              全选 Go 平台矩阵
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
      title="工具 × 平台矩阵"
      size="small"
      :bordered="false"
      class="bg-[#151923]/90"
    >
      <div class="overflow-x-auto">
        <table class="min-w-full border-collapse text-sm">
          <thead>
            <tr class="border-b border-white/10 text-slate-300">
              <th class="sticky left-0 z-10 min-w-[260px] bg-[#151923] px-3 py-3 text-left">
                工具
              </th>
              <th
                v-for="platform in artifactPlatforms"
                :key="platform.key"
                class="min-w-[96px] px-2 py-3 text-center"
              >
                <div class="flex flex-col items-center gap-1">
                  <span>{{ platform.label }}</span>
                  <NCheckbox
                    :checked="isPlatformFullySelected(platform.key)"
                    :indeterminate="isPlatformPartiallySelected(platform.key)"
                    @update:checked="artifactCenter.setPlatformSelections(platform.key, $event, goTools)"
                  />
                </div>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="tool in goTools"
              :key="tool.id"
              class="border-b border-white/5"
            >
              <td class="sticky left-0 z-10 bg-[#151923] px-3 py-3">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="font-medium text-slate-100">
                      {{ tool.name }}
                    </div>
                    <div class="text-xs text-slate-400">
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
            <tr
              v-for="tool in pythonTools"
              :key="tool.id"
              class="border-b border-white/5"
            >
              <td class="sticky left-0 z-10 bg-[#151923] px-3 py-3">
                <div class="flex items-center gap-2">
                  <div>
                    <div class="font-medium text-slate-100">
                      {{ tool.name }}
                    </div>
                    <div class="text-xs text-slate-400">
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
                class="px-3 py-3 text-left text-xs text-slate-500"
              >
                当前第一版仅支持 Go 工具的全平台二进制矩阵；Python 工具保留单工具脚本导出能力。
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </NCard>

    <NCard
      title="任务结果"
      size="small"
      :bordered="false"
      class="bg-[#151923]/90"
    >
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
          class="rounded-lg border border-white/10 bg-white/5 p-3"
        >
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div>
              <div class="flex items-center gap-2">
                <span class="font-medium text-slate-100">
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
              <div class="mt-1 text-xs text-slate-400">
                成功 {{ task.successCount }} / 缓存 {{ task.cachedCount }} / 跳过 {{ task.skippedCount }} / 失败 {{ task.errorCount }}
              </div>
              <div
                v-if="task.exitMessage"
                class="mt-1 text-xs text-slate-500"
              >
                {{ task.exitMessage }}
              </div>
            </div>
            <div class="flex gap-2">
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

          <div class="mt-3 grid gap-2">
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
        </div>
      </div>
    </NCard>
  </div>
</template>
