<script setup lang="ts">
import { computed, onMounted, type CSSProperties } from 'vue'
import {
  NAlert,
  NButton,
  NIcon,
  NInput,
  NSelect,
  useMessage,
} from 'naive-ui'
import { FolderOpenOutline } from '@vicons/ionicons5'
import { OpenFileDialog } from '../../../wailsjs/go/main/App'
import { useRustEnvStore } from '@/stores/rustenv'
import { getToolKindTheme } from '@/utils/executionTheme'

const props = defineProps<{
  zigDownloadVersion: string
  downloadDirectory: string
}>()

const emit = defineEmits<{
  'update:zig-download-version': [value: string]
  'update:download-directory': [value: string]
}>()

const rustEnv = useRustEnvStore()
const message = useMessage()
const zigTheme = getToolKindTheme('zig')
const zigSourceLabelMap: Record<string, string> = {
  configured: '自定义路径',
  remembered: '历史路径',
  path: 'PATH 中发现',
  detected: '常见安装目录',
  managed: '托管工具链',
}
const manualZigDownloadLink = 'https://ziglang.org/download/'

const zigDownloadVersionModel = computed({
  get: () => props.zigDownloadVersion,
  set: (value: string) => emit('update:zig-download-version', value),
})

const downloadDirectoryModel = computed({
  get: () => props.downloadDirectory,
  set: (value: string) => emit('update:download-directory', value),
})

const zigCandidateOptions = computed(() =>
  (rustEnv.state?.zigCandidates ?? [])
    .filter(candidate => candidate.valid)
    .map(candidate => ({
      label: candidate.detail
        ? `${candidate.label} · ${describeZigSource(candidate.source)}  ${candidate.detail}`
        : `${candidate.label} · ${describeZigSource(candidate.source)}`,
      value: candidate.path,
    })),
)

const currentZigBinary = computed(() => rustEnv.state?.config.selectedZigBinary || rustEnv.state?.activeZigBinary || null)
const canInstallZigOnly = computed(() => zigDownloadVersionModel.value.trim() && downloadDirectoryModel.value.trim())
const resolvedZigInstallDirectory = computed(() => normalizeZigInstallBaseDirectory(downloadDirectoryModel.value))
const zigReady = computed(() => rustEnv.state?.hasUsableZig === true)
const zigSurfaceStyle = computed<CSSProperties>(() => ({
  borderColor: zigReady.value ? zigTheme.accentSoftStrongBorder : 'rgb(var(--color-warning) / 0.22)',
  backgroundColor: zigReady.value ? zigTheme.accentSoftBg : 'rgb(var(--color-warning) / 0.05)',
}))
const zigTaskStyle = computed<CSSProperties>(() => ({
  borderColor: zigTheme.accentSoftBorder,
  backgroundColor: zigTheme.accentSoftBg,
}))

async function ensureRustState() {
  if (!rustEnv.state && !rustEnv.loading) {
    await rustEnv.loadState()
  }
}

async function ensureZigReleases(force = false) {
  await rustEnv.ensureZigReleases(force)
  if (!zigDownloadVersionModel.value && rustEnv.zigReleases.length > 0) {
    zigDownloadVersionModel.value = rustEnv.zigReleases[0].version
  }
}

function ensureZigDownloadDirectory() {
  if (downloadDirectoryModel.value.trim()) {
    return
  }
  const lastInstallDirectory = rustEnv.state?.config.lastInstallDirectory?.trim()
  if (lastInstallDirectory) {
    downloadDirectoryModel.value = normalizeZigInstallBaseDirectory(lastInstallDirectory)
    return
  }
  const suggested = rustEnv.state?.suggestedInstallDirectory?.trim()
  if (suggested) {
    downloadDirectoryModel.value = normalizeZigInstallBaseDirectory(suggested)
  }
}

function describeZigSource(source?: string) {
  return zigSourceLabelMap[source || ''] || '自动检测'
}

function normalizeZigInstallBaseDirectory(directory?: string) {
  const value = (directory || '').trim().replace(/[\\/]+$/, '')
  if (!value) {
    return ''
  }
  const segments = value.split(/[\\/]/)
  const last = (segments[segments.length - 1] || '').toLowerCase()
  const parent = (segments[segments.length - 2] || '').toLowerCase()
  const versionLike = last === 'stable'
    || last === 'beta'
    || last === 'nightly'
    || /^\d+\.\d+\.\d+(?:[-._a-z0-9]+)?$/i.test(last)
  if (versionLike && parent === 'zig') {
    segments.splice(-2, 2)
    return segments.join('/') || value
  }
  if (last === 'zig') {
    segments.pop()
    return segments.join('/') || value
  }
  return value
}

function formatByteCount(value: number) {
  if (value < 1024) {
    return `${Math.round(value)} B`
  }
  const units = ['KB', 'MB', 'GB', 'TB']
  let size = value
  let unitIndex = -1
  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex += 1
  }
  return `${size.toFixed(1)} ${units[unitIndex]}`
}

function formatTransferSummary() {
  const task = rustEnv.task
  if (!task || task.status !== 'running' || !task.kind.startsWith('install')) {
    return ''
  }
  if ((task.totalBytes || 0) > 0) {
    return `${formatByteCount(task.transferredBytes || 0)} / ${formatByteCount(task.totalBytes || 0)}${task.transferSpeed ? ` · ${task.transferSpeed}` : ''}`
  }
  if ((task.transferredBytes || 0) > 0) {
    return `${formatByteCount(task.transferredBytes || 0)}${task.transferSpeed ? ` · ${task.transferSpeed}` : ''}`
  }
  return task.transferSpeed || ''
}

async function handleRetryZigReleases() {
  try {
    await ensureZigReleases(true)
    if (rustEnv.zigReleases.length > 0) {
      message.success('Zig 版本列表已刷新')
      return
    }
    message.warning('暂未获取到 Zig 版本列表')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '重新获取 Zig 版本列表失败')
  }
}

async function handleOpenLocalZig() {
  try {
    const filePath = await OpenFileDialog({
      title: '选择 Zig 可执行文件',
      filterName: 'Executable',
      filterGlob: '*',
      directory: false,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (!filePath) {
      return
    }
    await rustEnv.chooseZigBinary(filePath)
    message.success('Zig SDK 已更新')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '选择 Zig 可执行文件失败')
  }
}

async function handleSelectZigBinary(value: string | null) {
  if (!value) {
    return
  }
  try {
    await rustEnv.chooseZigBinary(value)
    message.success('Zig SDK 已切换')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '切换 Zig SDK 失败')
  }
}

async function handleBrowseInstallDirectory() {
  try {
    const dir = await OpenFileDialog({
      title: '选择 Zig 工具链安装目录',
      filterName: '目录',
      filterGlob: '*',
      directory: true,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (dir) {
      downloadDirectoryModel.value = dir
    }
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '选择目录失败')
  }
}

async function handleStartZigInstall() {
  if (!canInstallZigOnly.value) {
    message.error('请先选择 Zig 版本和安装位置')
    return
  }
  try {
    await rustEnv.install('', zigDownloadVersionModel.value, downloadDirectoryModel.value)
    message.info('已开始安装 Zig SDK')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '安装 Zig SDK 失败')
  }
}

async function handleRetryZigInstall() {
  if (!zigDownloadVersionModel.value && rustEnv.task?.zigVersion) {
    zigDownloadVersionModel.value = rustEnv.task.zigVersion
  }
  if (!downloadDirectoryModel.value && rustEnv.task?.directory) {
    downloadDirectoryModel.value = normalizeZigInstallBaseDirectory(rustEnv.task.directory)
  }
  await handleStartZigInstall()
}

async function handleCancelTask() {
  try {
    await rustEnv.cancelTask()
    message.info('已请求停止当前 Zig 安装任务')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '停止 Zig 安装任务失败')
  }
}

async function handleCheckEnvironment() {
  try {
    const state = await rustEnv.checkEnvironment()
    if (state?.hasUsableZig) {
      message.success('Zig SDK 检查通过')
      return
    }
    message.warning(state?.statusMessage || 'Zig SDK 未就绪')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '检查 Zig SDK 失败')
  }
}

onMounted(async () => {
  await ensureRustState()
  await ensureZigReleases()
  ensureZigDownloadDirectory()
})
</script>

<template>
  <div class="space-y-4 pt-2">
    <div
      class="rounded-xl border px-4 py-4"
      :style="zigSurfaceStyle"
    >
      <div class="text-base font-semibold text-[rgb(var(--color-fg-base)/0.98)]">
        {{ zigReady ? 'Zig 环境已就绪' : 'Zig 环境未就绪' }}
      </div>
      <div class="mt-2 text-sm leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
        {{
          zigReady
            ? '这个页面聚焦 Zig SDK 本身的检测、选择与下载；Rust 页仍保留完整的交叉编译能力配置。'
            : '请先选择一个可用 Zig，可从本地指定，也可直接下载托管 Zig SDK。'
        }}
      </div>
      <div
        v-if="rustEnv.state?.activeZigVersion || rustEnv.state?.activeZigBinary"
        class="mt-3 text-xs text-[rgb(var(--color-fg-muted)/0.95)]"
      >
        当前 Zig：{{ rustEnv.state?.activeZigVersion || '未检测到版本' }}
      </div>
    </div>

    <NAlert
      v-if="rustEnv.error"
      type="error"
      :show-icon="false"
    >
      {{ rustEnv.error }}
    </NAlert>

    <div
      v-if="rustEnv.task && (rustEnv.task.kind === 'install-zig' || rustEnv.task.kind === 'install')"
      class="rounded-xl border px-4 py-4"
      :style="zigTaskStyle"
    >
      <div class="flex items-center justify-between gap-3">
        <div>
          <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            Zig 环境任务
          </div>
          <div class="mt-1 text-xs text-[rgb(var(--color-fg-secondary)/0.9)]">
            {{ rustEnv.task.message || '正在处理 Zig 环境任务' }}
          </div>
          <div
            v-if="rustEnv.task.currentItem"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            当前项：{{ rustEnv.task.currentItem }}
          </div>
          <div
            v-if="rustEnv.task.detail && !formatTransferSummary()"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            {{ rustEnv.task.detail }}
          </div>
          <div
            v-if="formatTransferSummary()"
            class="mt-1 text-[11px]"
            :style="{ color: zigTheme.accent }"
          >
            {{ formatTransferSummary() }}
          </div>
        </div>
        <NButton
          v-if="rustEnv.task.status === 'running'"
          secondary
          size="small"
          @click="handleCancelTask"
        >
          停止
        </NButton>
        <NButton
          v-else-if="rustEnv.task.status === 'failed' || rustEnv.task.status === 'canceled'"
          secondary
          size="small"
          :disabled="!(canInstallZigOnly || (rustEnv.task.zigVersion && rustEnv.task.directory))"
          @click="handleRetryZigInstall"
        >
          重试
        </NButton>
      </div>
    </div>

    <div class="settings-form">
      <div class="settings-row">
        <div class="settings-label">
          当前 Zig
        </div>
        <div class="settings-value gap-2">
          <NSelect
            class="settings-control"
            :value="currentZigBinary"
            :options="zigCandidateOptions"
            :placeholder="rustEnv.loading ? '正在检测 Zig...' : '选择 Zig SDK'"
            :loading="rustEnv.loading"
            :disabled="rustEnv.task?.status === 'running'"
            @update:value="(v: string | null) => handleSelectZigBinary(v)"
          />
          <NButton
            secondary
            size="medium"
            :disabled="rustEnv.task?.status === 'running'"
            @click="handleOpenLocalZig"
          >
            <template #icon>
              <NIcon :component="FolderOpenOutline" />
            </template>
          </NButton>
        </div>
      </div>

      <div class="settings-row align-start">
        <div class="settings-label">
          当前状态
        </div>
        <div class="settings-value">
          <div class="w-full rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-3 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
            <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
              Zig
            </div>
            <div class="break-all">
              {{ rustEnv.state?.activeZigVersion || '未就绪' }}
            </div>
            <div class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
              {{ rustEnv.state?.activeZigBinary || '未检测到可用路径' }}
            </div>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap justify-end gap-2">
        <NButton
          secondary
          size="small"
          :loading="rustEnv.checking"
          :disabled="rustEnv.loading || rustEnv.saving || rustEnv.task?.status === 'running'"
          @click="handleCheckEnvironment"
        >
          检查环境
        </NButton>
      </div>
    </div>

    <div class="rounded-xl border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
          下载 / 补齐
        </div>
        <NButton
          secondary
          size="small"
          :loading="rustEnv.zigReleaseLoading"
          @click="handleRetryZigReleases"
        >
          刷新版本
        </NButton>
      </div>

      <NAlert
        v-if="rustEnv.zigReleaseError"
        type="error"
        :show-icon="false"
      >
        <div>{{ rustEnv.zigReleaseError }}</div>
        <a
          :href="manualZigDownloadLink"
          target="_blank"
          rel="noreferrer"
          class="mt-2 inline-block text-[rgb(var(--color-kind-rust)/0.96)] underline underline-offset-2 hover:text-[rgb(var(--color-kind-rust)/0.82)]"
          :style="{ color: zigTheme.accent }"
        >
          Zig 官方下载页
        </a>
      </NAlert>

      <div class="settings-form">
        <div class="settings-row">
          <div class="settings-label">
            Zig 版本
          </div>
          <div class="settings-value gap-2">
            <NSelect
              class="settings-control"
              :value="zigDownloadVersionModel"
              :options="rustEnv.zigReleases.map(release => ({ label: release.version, value: release.version }))"
              :loading="rustEnv.zigReleaseLoading"
              :disabled="rustEnv.task?.status === 'running'"
              placeholder="选择 Zig 版本"
              @update:value="(v: string | null) => zigDownloadVersionModel = v || ''"
            />
            <NButton
              secondary
              :loading="rustEnv.installing && rustEnv.task?.kind === 'install-zig'"
              :disabled="!canInstallZigOnly || rustEnv.task?.status === 'running'"
              @click="handleStartZigInstall"
            >
              下载 Zig
            </NButton>
          </div>
        </div>

        <div class="settings-row align-start">
          <div class="settings-label">
            基目录
          </div>
          <div class="settings-value gap-2">
            <NInput
              class="settings-control"
              :value="downloadDirectoryModel"
              placeholder="选择 Zig 工具链集中安装目录"
              :disabled="rustEnv.task?.status === 'running'"
              @update:value="(v: string) => downloadDirectoryModel = v"
            />
            <NButton
              secondary
              :disabled="rustEnv.task?.status === 'running'"
              @click="handleBrowseInstallDirectory"
            >
              <template #icon>
                <NIcon :component="FolderOpenOutline" />
              </template>
            </NButton>
          </div>
        </div>

        <div
          v-if="resolvedZigInstallDirectory"
          class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-muted)/0.95)]"
        >
          Zig 将安装到：{{ resolvedZigInstallDirectory }}/zig/{{ zigDownloadVersionModel || '...' }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-row {
  display: grid;
  grid-template-columns: clamp(152px, 32%, 192px) minmax(0, 1fr);
  align-items: center;
  column-gap: 16px;
}

.settings-label {
  min-width: 0;
  white-space: nowrap;
  color: rgb(var(--color-fg-base) / 0.9);
  font-size: 14px;
  line-height: 1.4;
}

.settings-control {
  width: 100%;
  min-width: 0;
}

.settings-value {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  min-width: 0;
}

.align-start {
  align-items: flex-start;
}

@media (max-width: 720px) {
  .settings-row {
    grid-template-columns: minmax(0, 1fr);
    row-gap: 8px;
  }
}
</style>
