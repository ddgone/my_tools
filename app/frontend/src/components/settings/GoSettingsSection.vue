<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDropdown,
  NIcon,
  NInput,
  NPopconfirm,
  NProgress,
  NSelect,
  useMessage,
} from 'naive-ui'
import { Add, FolderOpenOutline } from '@vicons/ionicons5'
import { OpenFileDialog } from '../../../wailsjs/go/main/App'
import { useGoEnvStore } from '@/stores/goenv'

const props = defineProps<{
  showDownloadPanel: boolean
  downloadVersion: string
  downloadDirectory: string
}>()

const emit = defineEmits<{
  'update:show-download-panel': [value: boolean]
  'update:download-version': [value: string]
  'update:download-directory': [value: string]
}>()

const goEnv = useGoEnvStore()
const message = useMessage()
const noSdkValue = '__no_sdk__'
const goSourceLabelMap: Record<string, string> = {
  configured: '自定义路径',
  remembered: '历史路径',
  path: 'PATH 中的 Go',
  detected: '系统安装目录',
  managed: '托管 SDK',
}
const manualGoDownloadLinks = [
  { label: 'Go 官方下载页', href: 'https://go.dev/dl/' },
  { label: 'Go 国内镜像页', href: 'https://golang.google.cn/dl/' },
]

const showDownloadPanelModel = computed({
  get: () => props.showDownloadPanel,
  set: (value: boolean) => emit('update:show-download-panel', value),
})

const downloadVersionModel = computed({
  get: () => props.downloadVersion,
  set: (value: string) => emit('update:download-version', value),
})

const downloadDirectoryModel = computed({
  get: () => props.downloadDirectory,
  set: (value: string) => emit('update:download-directory', value),
})

const goCandidateOptions = computed(() =>
  [
    { label: '<无 SDK>', value: noSdkValue },
    ...(goEnv.state?.candidates ?? [])
      .filter(candidate => candidate.valid)
      .map(candidate => ({
        label: candidate.detail
          ? `${candidate.label} · ${describeGoSource(candidate.source)}  ${candidate.detail}`
          : `${candidate.label} · ${describeGoSource(candidate.source)}`,
        value: candidate.path,
      })),
  ],
)

const currentGoBinary = computed(() => {
  if (goEnv.state?.config.disabled) {
    return noSdkValue
  }
  return goEnv.state?.config.selectedBinary || goEnv.state?.activeBinary || null
})

const goInstallActionLabel = computed(() => {
  const task = goEnv.task
  if (task?.status === 'running' && task.kind === 'install') {
    return '正在安装...'
  }
  if ((task?.status === 'failed' || task?.status === 'canceled') && task.kind === 'install') {
    return '继续安装'
  }
  return '安装'
})

const canInstall = computed(() => downloadVersionModel.value.trim() && downloadDirectoryModel.value.trim())
const resolvedGoInstallDirectory = computed(() =>
  resolveGoInstallTargetDirectory(downloadVersionModel.value, downloadDirectoryModel.value),
)

async function ensureGoState() {
  if (!goEnv.state && !goEnv.loading) {
    await goEnv.loadState()
  }
}

async function ensureGoReleases() {
  await goEnv.ensureReleases()
  if (!downloadVersionModel.value && goEnv.releases.length > 0) {
    downloadVersionModel.value = goEnv.releases[0].version
  }
}

async function handleRetryGoReleases() {
  try {
    await goEnv.ensureReleases(true)
    if (!downloadVersionModel.value && goEnv.releases.length > 0) {
      downloadVersionModel.value = goEnv.releases[0].version
    }
    if (goEnv.releases.length > 0) {
      message.success('Go 版本列表已刷新')
      return
    }
    message.warning('暂未获取到可用的 Go 版本列表')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '重新获取 Go 版本列表失败')
  }
}

function ensureDownloadDirectory(version?: string) {
  const targetVersion = version || downloadVersionModel.value
  if (!targetVersion) {
    return
  }
  const lastInstallDirectory = goEnv.state?.config.lastInstallDirectory?.trim()
  if (lastInstallDirectory) {
    downloadDirectoryModel.value = normalizeGoInstallBaseDirectory(lastInstallDirectory)
    return
  }
  const suggested = goEnv.state?.suggestedInstallDirectory?.trim()
  if (suggested) {
    downloadDirectoryModel.value = normalizeGoInstallBaseDirectory(suggested)
  }
}

function describeGoSource(source?: string) {
  return goSourceLabelMap[source || ''] || '自动检测'
}

function normalizeGoInstallBaseDirectory(directory?: string) {
  const value = (directory || '').trim().replace(/[\\/]+$/, '')
  if (!value) {
    return ''
  }
  const segments = value.split(/[\\/]/)
  const lastSegment = segments[segments.length - 1] || ''
  if (/^go\d+(?:\.\d+)+(?:[-._a-z0-9]+)?$/i.test(lastSegment)) {
    segments.pop()
    return segments.join('/') || value
  }
  return value
}

function resolveGoInstallTargetDirectory(version?: string, directory?: string) {
  const normalizedVersion = (version || '').trim().toLowerCase()
  const baseDirectory = normalizeGoInstallBaseDirectory(directory)
  if (!baseDirectory) {
    return ''
  }
  if (!normalizedVersion) {
    return baseDirectory
  }
  if (baseDirectory.toLowerCase().endsWith(`/${normalizedVersion}`)) {
    return baseDirectory
  }
  return `${baseDirectory}/${normalizedVersion}`
}

async function handleOpenLocalGo() {
  try {
    const filePath = await OpenFileDialog({
      title: '选择 Go 可执行文件',
      filterName: 'Go',
      filterGlob: '*',
      directory: false,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (!filePath) {
      return
    }
    await goEnv.chooseBinary(filePath)
    showDownloadPanelModel.value = false
    message.success('Go 环境已更新')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '选择本地 Go 失败')
  }
}

async function handleSelectGo(value: string | null) {
  if (!value) {
    return
  }
  try {
    if (value === noSdkValue) {
      await goEnv.clearSelection()
      message.success('已清除 Go 环境配置')
      return
    }
    await goEnv.chooseBinary(value)
    message.success('Go 环境已切换')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '切换 Go 环境失败')
  }
}

async function handleCheckGoEnvironment() {
  try {
    const state = await goEnv.checkEnvironment()
    if (state.hasUsableBinary) {
      const version = state.activeVersion || 'Go'
      message.success(`Go 环境检查通过：${version} · ${describeGoSource(state.activeSource)}`)
      return
    }
    message.warning('未检测到可用的 Go 环境，请选择本地 Go 或下载 SDK')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '检查 Go 环境失败')
  }
}

async function handleCancelGoTask() {
  try {
    await goEnv.cancelTask()
    message.info('已请求停止当前 Go SDK 下载任务')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '停止 Go SDK 下载任务失败')
  }
}

async function handleDeleteGoEnvironment() {
  try {
    await goEnv.deleteEnvironment()
    message.success('当前托管 Go SDK 已删除')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '删除 Go 环境失败')
  }
}

async function handleBrowseInstallDirectory() {
  try {
    const dir = await OpenFileDialog({
      title: '选择 Go SDK 安装目录',
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

async function handleStartDownload() {
  if (!canInstall.value) {
    message.error('请先选择版本和安装位置')
    return
  }
  try {
    await goEnv.install(downloadVersionModel.value, downloadDirectoryModel.value)
    message.info('已开始下载并安装 Go SDK')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '安装 Go SDK 失败')
  }
}

async function handleRetryGoInstall() {
  if (!downloadVersionModel.value && goEnv.task?.version) {
    downloadVersionModel.value = goEnv.task.version
  }
  if (!downloadDirectoryModel.value && goEnv.task?.directory) {
    downloadDirectoryModel.value = normalizeGoInstallBaseDirectory(goEnv.task.directory)
  }
  await handleStartDownload()
}

function formatTransferSummary() {
  const task = goEnv.task
  if (!task) {
    return ''
  }
  if (task.status !== 'running' || task.kind !== 'install') {
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

async function handleGoMenuSelect(key: string | number) {
  if (key === 'local') {
    await handleOpenLocalGo()
    return
  }
  showDownloadPanelModel.value = true
  await ensureGoState()
  await ensureGoReleases()
  ensureDownloadDirectory()
}

watch(downloadVersionModel, (value) => {
  if (!showDownloadPanelModel.value) {
    return
  }
  ensureDownloadDirectory(value)
})

onMounted(async () => {
  await ensureGoState()
  if (showDownloadPanelModel.value) {
    await ensureGoReleases()
    ensureDownloadDirectory()
  }
})
</script>

<template>
  <div class="pt-2 space-y-4">
    <div
      class="rounded-xl border px-4 py-4"
      :class="goEnv.hasUsableBinary ? 'border-emerald-400/20 bg-emerald-500/5' : 'border-amber-400/20 bg-amber-500/5'"
    >
      <div class="text-base font-semibold text-[rgb(var(--color-fg-base)/0.98)]">
        {{ goEnv.hasUsableBinary ? 'Go 环境已就绪' : '当前未检测到可用的 Go 环境' }}
      </div>
      <div class="mt-2 text-sm leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
        {{
          goEnv.hasUsableBinary
            ? `当前使用 ${goEnv.state?.activeVersion || 'Go'}，整个桌面宿主共用这一份 Go 环境。`
            : '请先选择本地 Go，或直接下载一个 Go SDK。配置完成后会立即生效。'
        }}
      </div>
      <div
        v-if="goEnv.state?.statusMessage"
        class="mt-3 text-xs text-[rgb(var(--color-fg-muted)/0.95)]"
      >
        {{ goEnv.state.statusMessage }}
      </div>
    </div>

    <NAlert
      v-if="goEnv.error"
      type="error"
      :show-icon="false"
    >
      {{ goEnv.error }}
    </NAlert>

    <div
      v-if="goEnv.task"
      class="rounded-xl border border-cyan-400/15 bg-cyan-500/[0.04] px-4 py-4"
    >
      <div class="flex items-center justify-between gap-3">
        <div>
          <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            Go SDK 下载任务
          </div>
          <div class="mt-1 text-xs text-[rgb(var(--color-fg-secondary)/0.9)]">
            {{ goEnv.task.message || '正在处理 Go SDK 下载任务' }}
          </div>
          <div
            v-if="goEnv.task.currentItem"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            当前项：{{ goEnv.task.currentItem }}
          </div>
          <div
            v-if="goEnv.task.detail && !formatTransferSummary()"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            {{ goEnv.task.detail }}
          </div>
          <div
            v-if="formatTransferSummary()"
            class="mt-1 text-[11px] text-[rgb(var(--color-brand-primary)/0.9)]"
          >
            {{ formatTransferSummary() }}
          </div>
          <div
            v-if="goEnv.task.currentSource"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            下载源：{{ goEnv.task.currentSource }}
          </div>
        </div>
        <NButton
          v-if="goEnv.task.status === 'running'"
          secondary
          size="small"
          @click="handleCancelGoTask"
        >
          停止
        </NButton>
        <NButton
          v-else-if="goEnv.task.kind === 'install' && (goEnv.task.status === 'failed' || goEnv.task.status === 'canceled')"
          secondary
          size="small"
          :disabled="!(canInstall || (goEnv.task.version && goEnv.task.directory))"
          @click="handleRetryGoInstall"
        >
          重试安装
        </NButton>
      </div>
      <div class="mt-3">
        <NProgress
          type="line"
          :percentage="Math.max(0, Math.min(100, goEnv.task.progressPercent || 0))"
          :show-indicator="true"
          :format="(percentage: number) => `${Math.round(percentage)}%`"
          processing
        />
      </div>
      <div class="mt-2 text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
        状态：{{ goEnv.task.status }}
        <span v-if="goEnv.task.totalSteps > 0"> · 步骤 {{ goEnv.task.step }}/{{ goEnv.task.totalSteps }}</span>
      </div>
      <div
        v-if="goEnv.task.error"
        class="mt-2 text-[11px] text-rose-200/85 break-all"
      >
        {{ goEnv.task.error }}
      </div>
      <div
        v-if="goEnv.task.kind === 'install' && (goEnv.task.error || goEnv.task.status === 'failed' || goEnv.task.status === 'canceled')"
        class="mt-3 text-[11px] leading-6 text-[rgb(var(--color-fg-muted)/0.95)]"
      >
        自动安装不通时，可手动下载后通过“本地”选择 go.exe：
        <a
          v-for="link in manualGoDownloadLinks"
          :key="link.href"
          :href="link.href"
          target="_blank"
          rel="noreferrer"
          class="ml-2 text-[rgb(var(--color-brand-primary)/0.94)] underline underline-offset-2 hover:text-[rgb(var(--color-brand-hover)/1)]"
        >
          {{ link.label }}
        </a>
      </div>
    </div>

    <div class="settings-form">
      <div class="settings-row">
        <div class="settings-label">
          当前 Go 环境
        </div>
        <div class="settings-value gap-2">
          <NSelect
            class="settings-control"
            :value="currentGoBinary"
            :options="goCandidateOptions"
            :placeholder="goEnv.loading ? '正在检测 Go 环境...' : '尚未选择 Go 环境'"
            :loading="goEnv.loading"
            :disabled="goEnv.task?.status === 'running'"
            @update:value="(v: string | null) => handleSelectGo(v)"
          />
          <NDropdown
            trigger="click"
            :options="[
              { label: '本地', key: 'local' },
              { label: '下载', key: 'download' },
            ]"
            @select="handleGoMenuSelect"
          >
            <NButton
              secondary
              size="medium"
              :disabled="goEnv.task?.status === 'running'"
            >
              <template #icon>
                <NIcon :component="Add" />
              </template>
            </NButton>
          </NDropdown>
        </div>
      </div>

      <div
        v-if="goEnv.state?.activeBinary"
        class="settings-row align-start"
      >
        <div class="settings-label">
          当前路径
        </div>
        <div class="settings-value">
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)] break-all">
            {{ goEnv.state.activeBinary }}
          </div>
        </div>
      </div>

      <div
        v-if="goEnv.state?.activeSource"
        class="settings-row"
      >
        <div class="settings-label">
          来源类型
        </div>
        <div class="settings-value">
          <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
            {{ describeGoSource(goEnv.state.activeSource) }}
          </div>
        </div>
      </div>

      <div
        v-if="goEnv.state?.hasUsableBinary"
        class="settings-row align-start"
      >
        <div class="settings-label">
          环境详情
        </div>
        <div class="settings-value">
          <div class="grid gap-2 md:grid-cols-2">
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                GOVERSION
              </div>
              <div class="break-all">
                {{ goEnv.state?.runtimeDetails?.goversion || goEnv.state?.activeVersion || '未知' }}
              </div>
            </div>
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                GOOS / GOARCH
              </div>
              <div class="break-all">
                {{ [goEnv.state?.runtimeDetails?.goos, goEnv.state?.runtimeDetails?.goarch].filter(Boolean).join(' / ') || '未知' }}
              </div>
            </div>
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)] md:col-span-2">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                GOROOT
              </div>
              <div class="break-all">
                {{ goEnv.state?.runtimeDetails?.goroot || '未知' }}
              </div>
            </div>
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)] md:col-span-2">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                GOPATH
              </div>
              <div class="break-all">
                {{ goEnv.state?.runtimeDetails?.gopath || '未知' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap justify-end gap-2">
        <NButton
          secondary
          size="small"
          :loading="goEnv.checking"
          :disabled="goEnv.loading || goEnv.task?.status === 'running' || goEnv.saving"
          @click="handleCheckGoEnvironment"
        >
          检查环境
        </NButton>
        <NPopconfirm
          @positive-click="handleDeleteGoEnvironment"
        >
          <template #trigger>
            <NButton
              secondary
              size="small"
              type="error"
              :loading="goEnv.deleting"
              :disabled="goEnv.state?.activeSource !== 'managed' || goEnv.task?.status === 'running'"
            >
              删除环境
            </NButton>
          </template>
          确定删除当前托管 Go SDK 吗？如果还有其他 Go 版本，程序会自动回退到可用环境。
        </NPopconfirm>
      </div>
    </div>

    <div
      v-if="showDownloadPanelModel"
      class="rounded-xl border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4 space-y-4"
    >
      <div class="flex items-center justify-between">
        <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
          下载 Go SDK
        </div>
        <NButton
          text
          size="small"
          @click="showDownloadPanelModel = false"
        >
          收起
        </NButton>
      </div>

      <NAlert
        v-if="goEnv.releaseError"
        type="error"
        :show-icon="false"
      >
        <div>{{ goEnv.releaseError }}</div>
        <div class="mt-2 flex flex-wrap items-center gap-3 text-[12px]">
          <NButton
            secondary
            size="tiny"
            :loading="goEnv.releaseLoading"
            @click="handleRetryGoReleases"
          >
            重试获取版本
          </NButton>
          <a
            v-for="link in manualGoDownloadLinks"
            :key="`release-${link.href}`"
            :href="link.href"
            target="_blank"
            rel="noreferrer"
            class="text-[rgb(var(--color-brand-primary)/0.94)] underline underline-offset-2 hover:text-[rgb(var(--color-brand-hover)/1)]"
          >
            {{ link.label }}
          </a>
        </div>
      </NAlert>

      <div class="settings-form">
        <div class="settings-row">
          <div class="settings-label">
            版本
          </div>
          <div class="settings-value">
            <NSelect
              class="settings-control"
              :value="downloadVersionModel"
              :options="goEnv.releases.map(release => ({ label: release.version, value: release.version }))"
              :loading="goEnv.releaseLoading"
              :disabled="goEnv.task?.status === 'running'"
              placeholder="选择 Go 版本"
              @update:value="(v: string | null) => downloadVersionModel = v || ''"
            />
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
              placeholder="选择 Go SDK 集中安装目录"
              :disabled="goEnv.task?.status === 'running'"
              @update:value="(v: string) => downloadDirectoryModel = v"
            />
            <NButton
              secondary
              :disabled="goEnv.task?.status === 'running'"
              @click="handleBrowseInstallDirectory"
            >
              <template #icon>
                <NIcon :component="FolderOpenOutline" />
              </template>
            </NButton>
          </div>
        </div>
        <div
          v-if="resolvedGoInstallDirectory"
          class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-muted)/0.95)]"
        >
          将安装到：{{ resolvedGoInstallDirectory }}
        </div>
      </div>

      <div class="flex justify-end">
        <NButton
          type="primary"
          :loading="goEnv.installing"
          :disabled="goEnv.task?.status === 'running'"
          @click="handleStartDownload"
        >
          {{ goInstallActionLabel }}
        </NButton>
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
