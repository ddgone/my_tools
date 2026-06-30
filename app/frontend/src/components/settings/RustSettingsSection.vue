<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NAlert,
  NButton,
  NIcon,
  NInput,
  NPopconfirm,
  NProgress,
  NSelect,
  useMessage,
} from 'naive-ui'
import { FolderOpenOutline } from '@vicons/ionicons5'
import { OpenFileDialog } from '../../../wailsjs/go/main/App'
import { useRustEnvStore } from '@/stores/rustenv'

const props = defineProps<{
  rustDownloadVersion: string
  zigDownloadVersion: string
  downloadDirectory: string
}>()

const emit = defineEmits<{
  'update:rust-download-version': [value: string]
  'update:zig-download-version': [value: string]
  'update:download-directory': [value: string]
}>()

const rustEnv = useRustEnvStore()
const message = useMessage()
const rustSourceLabelMap: Record<string, string> = {
  configured: '自定义路径',
  remembered: '历史路径',
  path: 'PATH 中发现',
  detected: '常见安装目录',
  managed: '托管工具链',
}
const rustModeOptions = [
  { label: '无 SDK', value: 'none' },
  { label: '自动探测', value: 'auto' },
  { label: '手动选择', value: 'manual' },
]
const manualRustDownloadLinks = [
  { label: 'Rust 官方安装页', href: 'https://www.rust-lang.org/tools/install' },
  { label: 'Zig 官方下载页', href: 'https://ziglang.org/download/' },
]

const rustDownloadVersionModel = computed({
  get: () => props.rustDownloadVersion,
  set: (value: string) => emit('update:rust-download-version', value),
})

const zigDownloadVersionModel = computed({
  get: () => props.zigDownloadVersion,
  set: (value: string) => emit('update:zig-download-version', value),
})

const downloadDirectoryModel = computed({
  get: () => props.downloadDirectory,
  set: (value: string) => emit('update:download-directory', value),
})

const rustCandidateOptions = computed(() =>
  (rustEnv.state?.rustCandidates ?? [])
    .filter(candidate => candidate.valid)
    .map(candidate => ({
      label: `${candidate.label} · ${describeRustSource(candidate.source)}  ${candidate.detail}`,
      value: candidate.rootDir,
    })),
)

const zigCandidateOptions = computed(() =>
  (rustEnv.state?.zigCandidates ?? [])
    .filter(candidate => candidate.valid)
    .map(candidate => ({
      label: candidate.detail
        ? `${candidate.label} · ${describeRustSource(candidate.source)}  ${candidate.detail}`
        : `${candidate.label} · ${describeRustSource(candidate.source)}`,
      value: candidate.path,
    })),
)

const currentRustMode = computed(() => rustEnv.state?.config.mode || (rustEnv.state?.config.disabled ? 'none' : 'auto'))
const currentRustRoot = computed(() => rustEnv.state?.config.selectedRustRoot || rustEnv.state?.activeRustRoot || null)
const currentZigBinary = computed(() => rustEnv.state?.config.selectedZigBinary || rustEnv.state?.activeZigBinary || null)
const rustTargetStatuses = computed(() => rustEnv.state?.targetStatuses ?? [])
const rustInstalledCrossTargetCount = computed(() => rustTargetStatuses.value.filter(target => !target.native && target.installed).length)
const rustCrossTargetCount = computed(() => rustTargetStatuses.value.filter(target => !target.native).length)
const canInstallRustOnly = computed(() => rustDownloadVersionModel.value.trim() && downloadDirectoryModel.value.trim())
const canInstallZigOnly = computed(() => zigDownloadVersionModel.value.trim() && downloadDirectoryModel.value.trim())
const canInstallRustBundle = computed(() => canInstallRustOnly.value && canInstallZigOnly.value)
const resolvedRustInstallDirectory = computed(() => normalizeRustInstallBaseDirectory(downloadDirectoryModel.value))
const showAdvancedOperations = ref(false)
const hasUsableRustAndZig = computed(() => rustEnv.state?.hasUsableRust === true && rustEnv.state?.hasUsableZig === true)
const shouldCollapseInstallPanel = computed(() => hasUsableRustAndZig.value)
const shouldShowInstallPanel = computed(() => !shouldCollapseInstallPanel.value || showAdvancedOperations.value)
const installPanelTitle = computed(() => shouldCollapseInstallPanel.value ? '高级操作' : '下载 / 补齐')

async function ensureRustState() {
  if (!rustEnv.state && !rustEnv.loading) {
    await rustEnv.loadState()
  }
}

async function ensureRustReleases(force = false) {
  await rustEnv.ensureReleases(force)
  if (!rustDownloadVersionModel.value && rustEnv.rustReleases.length > 0) {
    rustDownloadVersionModel.value = rustEnv.rustReleases[0].version
  }
  if (!zigDownloadVersionModel.value && rustEnv.zigReleases.length > 0) {
    zigDownloadVersionModel.value = rustEnv.zigReleases[0].version
  }
}

function ensureRustDownloadDirectory() {
  if (downloadDirectoryModel.value.trim()) {
    return
  }
  const lastInstallDirectory = rustEnv.state?.config.lastInstallDirectory?.trim()
  if (lastInstallDirectory) {
    downloadDirectoryModel.value = normalizeRustInstallBaseDirectory(lastInstallDirectory)
    return
  }
  const suggested = rustEnv.state?.suggestedInstallDirectory?.trim()
  if (suggested) {
    downloadDirectoryModel.value = normalizeRustInstallBaseDirectory(suggested)
  }
}

function describeRustSource(source?: string) {
  return rustSourceLabelMap[source || ''] || '自动检测'
}

function normalizeRustInstallBaseDirectory(directory?: string) {
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
  if (versionLike && (parent === 'rust' || parent === 'zig')) {
    segments.splice(-2, 2)
    return segments.join('/') || value
  }
  if (last === 'rust' || last === 'zig') {
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

function formatRustTransferSummary() {
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

async function handleRetryRustReleases() {
  try {
    await ensureRustReleases(true)
    if (rustEnv.rustReleases.length > 0 && rustEnv.zigReleases.length > 0) {
      message.success('Rust/Zig 版本列表已刷新')
      return
    }
    message.warning('暂未获取到完整的 Rust/Zig 版本列表')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '重新获取 Rust/Zig 版本列表失败')
  }
}

async function handleOpenLocalRustRoot() {
  try {
    const rootDir = await OpenFileDialog({
      title: '选择 Rust 环境目录',
      filterName: '目录',
      filterGlob: '*',
      directory: true,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (!rootDir) {
      return
    }
    await rustEnv.chooseRustRoot(rootDir)
    message.success('Rust SDK 已更新')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '选择 Rust 环境目录失败')
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

async function handleSelectRustRoot(value: string | null) {
  if (!value) {
    return
  }
  try {
    await rustEnv.chooseRustRoot(value)
    message.success('Rust SDK 已切换')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '切换 Rust SDK 失败')
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

async function handleChangeRustMode(value: string | null) {
  if (!value) {
    return
  }
  try {
    if (value === 'auto') {
      await rustEnv.enableAutoDetect()
      message.success('已恢复 Rust/Zig 自动探测')
      return
    }
    if (value === 'none') {
      await rustEnv.disableEnvironment()
      message.success('已切换为无 SDK 模式')
      return
    }
    await rustEnv.setMode('manual')
    message.success('已切换为手动选择模式')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '切换 Rust 模式失败')
  }
}

async function handleCheckRustEnvironment() {
  try {
    const state = await rustEnv.checkEnvironment()
    if (!state) {
      message.warning('Rust 交叉编译环境未就绪')
      return
    }
    if (state.hasUsableEnvironment && state.hasInstalledTargetInfo && !state.hasFullTargetCoverage) {
      message.warning(state.targetStatusMessage || 'Rust 工具链已就绪，但常用交叉编译 targets 尚未补齐')
      return
    }
    if (state.hasUsableEnvironment) {
      message.success('Rust 交叉编译环境检查通过')
      return
    }
    message.warning(state.statusMessage || 'Rust 交叉编译环境未就绪')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '检查 Rust 环境失败')
  }
}

async function handleCancelRustTask() {
  try {
    await rustEnv.cancelTask()
    message.info('已请求停止当前 Rust 安装任务')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '停止 Rust 安装任务失败')
  }
}

async function handleDeleteRustEnvironment() {
  try {
    await rustEnv.deleteEnvironment()
    message.success('当前托管 Rust/Zig 环境已删除')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '删除 Rust 环境失败')
  }
}

async function handleBrowseRustInstallDirectory() {
  try {
    const dir = await OpenFileDialog({
      title: '选择 Rust/Zig 工具链安装目录',
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

async function handleStartRustInstall(kind: 'rust' | 'zig' | 'bundle') {
  const installRust = kind === 'rust' || kind === 'bundle'
  const installZig = kind === 'zig' || kind === 'bundle'
  if ((installRust && !canInstallRustOnly.value) || (installZig && !canInstallZigOnly.value)) {
    message.error('请先选择对应组件版本和安装位置')
    return
  }
  try {
    await rustEnv.install(
      installRust ? rustDownloadVersionModel.value : '',
      installZig ? zigDownloadVersionModel.value : '',
      downloadDirectoryModel.value,
    )
    message.info(
      kind === 'bundle'
        ? '已开始安装 Rust + Zig'
        : kind === 'rust'
          ? '已开始安装 Rust SDK'
          : '已开始安装 Zig SDK',
    )
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '安装 Rust 交叉编译环境失败')
  }
}

async function handleRetryRustInstall() {
  if (!rustDownloadVersionModel.value && rustEnv.task?.rustVersion) {
    rustDownloadVersionModel.value = rustEnv.task.rustVersion
  }
  if (!zigDownloadVersionModel.value && rustEnv.task?.zigVersion) {
    zigDownloadVersionModel.value = rustEnv.task.zigVersion
  }
  if (!downloadDirectoryModel.value && rustEnv.task?.directory) {
    downloadDirectoryModel.value = normalizeRustInstallBaseDirectory(rustEnv.task.directory)
  }
  if (rustEnv.task?.kind === 'install-rust') {
    await handleStartRustInstall('rust')
    return
  }
  if (rustEnv.task?.kind === 'install-zig') {
    await handleStartRustInstall('zig')
    return
  }
  await handleStartRustInstall('bundle')
}

async function handleInstallRustCargoZigbuild() {
  if (!rustEnv.state?.activeRustManaged) {
    message.warning('当前激活的是系统 Rust；为避免修改 ~/.cargo，自动补齐已禁用')
    return
  }
  try {
    await rustEnv.installCargoZigbuild()
    message.info('已开始补齐 cargo-zigbuild')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '补齐 cargo-zigbuild 失败')
  }
}

async function handleInstallRustTargets() {
  if (!rustEnv.state?.activeRustManaged) {
    message.warning('当前激活的是系统 Rust；为避免修改 ~/.rustup，自动补齐已禁用')
    return
  }
  try {
    await rustEnv.installTargets()
    message.info('已开始补齐常用 Rust targets')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '补齐常用 Rust targets 失败')
  }
}

onMounted(async () => {
  await ensureRustState()
  await ensureRustReleases()
  ensureRustDownloadDirectory()
})
</script>

<template>
  <div class="pt-2 space-y-4">
    <div
      class="rounded-xl border px-4 py-4"
      :class="rustEnv.state?.config.disabled
        ? 'border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-panel)/0.72)]'
        : rustEnv.hasUsableEnvironment
          ? 'border-[rgb(var(--color-kind-rust)/0.24)] bg-[rgb(var(--color-kind-rust)/0.10)]'
          : 'border-amber-400/20 bg-amber-500/5'"
    >
      <div class="text-base font-semibold text-[rgb(var(--color-fg-base)/0.98)]">
        {{
          rustEnv.state?.config.disabled
            ? 'Rust SDK 已关闭'
            : !rustEnv.state?.hasUsableRust
              ? 'Rust SDK 未就绪'
              : !rustEnv.state?.hasUsableZig
                ? 'Rust SDK 已就绪，Zig 待补齐'
                : !rustEnv.hasUsableEnvironment
                  ? 'Rust 交叉编译能力待补齐'
                  : 'Rust 交叉编译环境已就绪'
        }}
      </div>
      <div class="mt-2 text-sm leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
        {{
          rustEnv.state?.config.disabled
            ? '当前进入无 SDK 模式：Rust/Zig 自动探测已关闭，远程执行、导出和交叉编译缓存都会停用。'
            : !rustEnv.state?.hasUsableRust
              ? '请先自动探测、手动选择一个 Rust 环境目录，或直接下载托管 Rust SDK。'
              : !rustEnv.state?.hasUsableZig
                ? '当前 Rust SDK 已就绪，但还缺少 Zig；可以继续手动选择 Zig，或只下载 Zig。'
                : rustEnv.hasUsableEnvironment
                  ? 'Rust SDK、Zig、cargo-zigbuild 与常用 targets 都已就绪；本地 Rust 工具继续使用随宿主打包的二进制，远程执行和导出使用这里配置的交叉编译环境。'
                  : '当前已选定 Rust SDK 和 Zig，但 cargo-zigbuild 或常用 targets 仍需补齐。'
        }}
      </div>
      <div
        v-if="rustEnv.state?.statusMessage"
        class="mt-3 text-xs text-[rgb(var(--color-fg-muted)/0.95)]"
      >
        {{ rustEnv.state.statusMessage }}
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
      v-if="rustEnv.task"
      class="rounded-xl border border-[rgb(var(--color-kind-rust)/0.20)] bg-[rgb(var(--color-kind-rust)/0.10)] px-4 py-4"
    >
      <div class="flex items-center justify-between gap-3">
        <div>
          <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            Rust 环境任务
          </div>
          <div class="mt-1 text-xs text-[rgb(var(--color-fg-secondary)/0.9)]">
            {{ rustEnv.task.message || '正在处理 Rust 环境任务' }}
          </div>
          <div
            v-if="rustEnv.task.currentItem"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            当前项：{{ rustEnv.task.currentItem }}
          </div>
          <div
            v-if="rustEnv.task.detail && !formatRustTransferSummary()"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            {{ rustEnv.task.detail }}
          </div>
          <div
            v-if="formatRustTransferSummary()"
            class="mt-1 text-[11px] text-[rgb(var(--color-kind-rust)/0.9)]"
          >
            {{ formatRustTransferSummary() }}
          </div>
          <div
            v-if="rustEnv.task.currentSource"
            class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            下载源：{{ rustEnv.task.currentSource }}
          </div>
        </div>
        <NButton
          v-if="rustEnv.task.status === 'running'"
          secondary
          size="small"
          @click="handleCancelRustTask"
        >
          停止
        </NButton>
        <NButton
          v-else-if="rustEnv.task.kind.startsWith('install') && (rustEnv.task.status === 'failed' || rustEnv.task.status === 'canceled')"
          secondary
          size="small"
          :disabled="!(canInstallRustBundle || canInstallRustOnly || canInstallZigOnly || (rustEnv.task.rustVersion || rustEnv.task.zigVersion) && rustEnv.task.directory)"
          @click="handleRetryRustInstall"
        >
          重试
        </NButton>
      </div>
      <div class="mt-3">
        <NProgress
          type="line"
          :percentage="Math.max(0, Math.min(100, rustEnv.task.progressPercent || 0))"
          :show-indicator="true"
          :format="(percentage: number) => `${Math.round(percentage)}%`"
          processing
        />
      </div>
      <div class="mt-2 text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
        状态：{{ rustEnv.task.status }}
        <span v-if="rustEnv.task.totalSteps > 0"> · 步骤 {{ rustEnv.task.step }}/{{ rustEnv.task.totalSteps }}</span>
      </div>
      <div
        v-if="rustEnv.task.error"
        class="mt-2 text-[11px] text-rose-200/85 break-all"
      >
        {{ rustEnv.task.error }}
      </div>
      <div
        v-if="rustEnv.task.status === 'failed' || rustEnv.task.status === 'canceled'"
        class="mt-3 text-[11px] leading-6 text-[rgb(var(--color-fg-muted)/0.95)]"
      >
        自动安装不通时，可参考官方页面手动处理：
        <a
          v-for="link in manualRustDownloadLinks"
          :key="link.href"
          :href="link.href"
          target="_blank"
          rel="noreferrer"
          class="ml-2 text-[rgb(var(--color-kind-rust)/0.96)] underline underline-offset-2 hover:text-[rgb(var(--color-kind-rust)/0.82)]"
        >
          {{ link.label }}
        </a>
      </div>
    </div>

    <div class="settings-form">
      <div class="settings-row">
        <div class="settings-label">
          当前模式
        </div>
        <div class="settings-value">
          <NSelect
            class="settings-control"
            :value="currentRustMode"
            :options="rustModeOptions"
            :disabled="rustEnv.task?.status === 'running'"
            @update:value="(v: string | null) => handleChangeRustMode(v)"
          />
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-label">
          Rust SDK
        </div>
        <div class="settings-value gap-2">
          <NSelect
            class="settings-control"
            :value="currentRustRoot"
            :options="rustCandidateOptions"
            :placeholder="rustEnv.loading ? '正在检测 Rust SDK...' : '选择 Rust 环境目录'"
            :loading="rustEnv.loading"
            :disabled="rustEnv.task?.status === 'running' || currentRustMode !== 'manual'"
            @update:value="(v: string | null) => handleSelectRustRoot(v)"
          />
          <NButton
            secondary
            size="medium"
            :disabled="rustEnv.task?.status === 'running' || currentRustMode !== 'manual'"
            @click="handleOpenLocalRustRoot"
          >
            <template #icon>
              <NIcon :component="FolderOpenOutline" />
            </template>
          </NButton>
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-label">
          Zig SDK
        </div>
        <div class="settings-value gap-2">
          <NSelect
            class="settings-control"
            :value="currentZigBinary"
            :options="zigCandidateOptions"
            :placeholder="rustEnv.loading ? '正在检测 Zig...' : '选择 Zig SDK'"
            :loading="rustEnv.loading"
            :disabled="rustEnv.task?.status === 'running' || currentRustMode !== 'manual'"
            @update:value="(v: string | null) => handleSelectZigBinary(v)"
          />
          <NButton
            secondary
            size="medium"
            :disabled="rustEnv.task?.status === 'running' || currentRustMode !== 'manual'"
            @click="handleOpenLocalZig"
          >
            <template #icon>
              <NIcon :component="FolderOpenOutline" />
            </template>
          </NButton>
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-label">
          当前 Rust
        </div>
        <div class="settings-value">
          <div class="grid w-full gap-2 md:grid-cols-2">
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                SDK
              </div>
              <div class="break-all">
                {{ rustEnv.state?.activeRustVersion || '未就绪' }}
              </div>
              <div class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
                {{ rustEnv.state?.activeRustRoot || '未检测到可用目录' }}
              </div>
            </div>
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
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
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                cargo-zigbuild
              </div>
              <div class="break-all">
                {{ rustEnv.state?.activeCargoZigbuildVersion || '未就绪' }}
              </div>
              <div class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
                {{ rustEnv.state?.cargoZigbuildStatusMessage || '尚未检测' }}
              </div>
            </div>
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="text-[11px] uppercase tracking-wide text-[rgb(var(--color-fg-muted)/0.95)]">
                rustup / targets
              </div>
              <div class="break-all">
                {{ rustEnv.state?.activeRustupVersion || '未就绪' }}
              </div>
              <div class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
                {{ rustEnv.state?.targetStatusMessage || '尚未检测' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="settings-row align-start">
        <div class="settings-label">
          能力补齐
        </div>
        <div class="settings-value">
          <div class="w-full space-y-3">
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-3 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  cargo-zigbuild：{{ rustEnv.state?.hasUsableCargoZigbuild ? '已安装' : '缺失' }}
                </div>
                <NButton
                  v-if="!rustEnv.state?.hasUsableCargoZigbuild"
                  secondary
                  size="small"
                  :disabled="!rustEnv.state?.canManageCargoZigbuild || rustEnv.task?.status === 'running'"
                  @click="handleInstallRustCargoZigbuild"
                >
                  补齐 cargo-zigbuild
                </NButton>
              </div>
              <div class="mt-1 text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
                {{ rustEnv.state?.cargoZigbuildStatusMessage || '当前 Rust 环境已检测' }}
              </div>
            </div>
            <div class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-3 text-xs leading-6 text-[rgb(var(--color-fg-secondary)/0.9)]">
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  常用 targets：已安装 {{ rustInstalledCrossTargetCount }} / {{ rustCrossTargetCount }}
                </div>
                <NButton
                  v-if="!rustEnv.state?.hasFullTargetCoverage"
                  secondary
                  size="small"
                  :disabled="!rustEnv.state?.canManageTargets || rustEnv.task?.status === 'running'"
                  @click="handleInstallRustTargets"
                >
                  补齐 targets
                </NButton>
              </div>
              <div class="mt-1 text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]">
                {{ rustEnv.state?.targetStatusMessage || '尚未读取 target 状态' }}
              </div>
            </div>
            <div
              v-if="rustEnv.state?.hasInstalledTargetInfo"
              class="grid gap-2 md:grid-cols-2"
            >
              <div
                v-for="target in rustTargetStatuses"
                :key="target.platformKey"
                class="rounded-lg border px-3 py-2 text-xs leading-6"
                :class="target.installed
                  ? 'border-emerald-400/15 bg-emerald-500/[0.05] text-white/75'
                  : 'border-amber-400/15 bg-amber-500/[0.05] text-white/75'"
              >
                <div class="flex items-center justify-between gap-3">
                  <div class="font-medium text-[rgb(var(--color-fg-base)/0.92)]">
                    {{ target.platformLabel }}
                  </div>
                  <div
                    class="text-[11px]"
                    :class="target.installed ? 'text-emerald-300/85' : 'text-amber-300/85'"
                  >
                    {{ target.native ? '原生构建' : (target.installed ? '已安装' : '未安装') }}
                  </div>
                </div>
                <div class="mt-1 break-all text-[11px] text-[rgb(var(--color-fg-muted)/0.92)]">
                  {{ target.targetTriple }}
                </div>
                <div
                  v-if="target.note"
                  class="mt-1 text-[11px] text-[rgb(var(--color-fg-muted)/0.95)]"
                >
                  {{ target.note }}
                </div>
              </div>
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
          @click="handleCheckRustEnvironment"
        >
          检查环境
        </NButton>
        <NPopconfirm
          @positive-click="handleDeleteRustEnvironment"
        >
          <template #trigger>
            <NButton
              secondary
              size="small"
              type="error"
              :loading="rustEnv.deleting"
              :disabled="(!(rustEnv.state?.activeRustManaged) && (!rustEnv.state?.activeZigSource || rustEnv.state?.activeZigSource !== 'managed')) || rustEnv.task?.status === 'running'"
            >
              删除托管环境
            </NButton>
          </template>
          确定删除当前托管 Rust/Zig 环境吗？如果还有其它版本，程序会自动回退到可用环境。
        </NPopconfirm>
      </div>
    </div>

    <div class="rounded-xl border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4 space-y-4">
      <div class="flex items-center justify-between">
        <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
          {{ installPanelTitle }}
        </div>
        <div class="flex items-center gap-2">
          <NButton
            secondary
            size="small"
            :loading="rustEnv.rustReleaseLoading || rustEnv.zigReleaseLoading"
            @click="handleRetryRustReleases"
          >
            刷新版本
          </NButton>
          <NButton
            v-if="shouldCollapseInstallPanel"
            secondary
            size="small"
            @click="showAdvancedOperations = !showAdvancedOperations"
          >
            {{ shouldShowInstallPanel ? '收起' : '展开' }}
          </NButton>
        </div>
      </div>

      <template v-if="shouldShowInstallPanel">
        <NAlert
          v-if="rustEnv.rustReleaseError || rustEnv.zigReleaseError"
          type="error"
          :show-icon="false"
        >
          <div>{{ rustEnv.rustReleaseError || rustEnv.zigReleaseError }}</div>
          <div class="mt-2 flex flex-wrap items-center gap-3 text-[12px]">
            <a
              v-for="link in manualRustDownloadLinks"
              :key="`rust-release-${link.href}`"
              :href="link.href"
              target="_blank"
              rel="noreferrer"
              class="text-[rgb(var(--color-kind-rust)/0.96)] underline underline-offset-2 hover:text-[rgb(var(--color-kind-rust)/0.82)]"
            >
              {{ link.label }}
            </a>
          </div>
        </NAlert>

        <div class="settings-form">
          <div class="settings-row">
            <div class="settings-label">
              Rust 版本
            </div>
            <div class="settings-value gap-2">
              <NSelect
                class="settings-control"
                :value="rustDownloadVersionModel"
                :options="rustEnv.rustReleases.map(release => ({ label: release.version, value: release.version }))"
                :loading="rustEnv.rustReleaseLoading"
                :disabled="rustEnv.task?.status === 'running'"
                placeholder="选择 Rust 版本"
                @update:value="(v: string | null) => rustDownloadVersionModel = v || ''"
              />
              <NButton
                secondary
                :loading="rustEnv.installing && rustEnv.task?.kind === 'install-rust'"
                :disabled="!canInstallRustOnly || rustEnv.task?.status === 'running'"
                @click="handleStartRustInstall('rust')"
              >
                下载 Rust
              </NButton>
            </div>
          </div>

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
                @click="handleStartRustInstall('zig')"
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
                placeholder="选择 Rust/Zig 工具链集中安装目录"
                :disabled="rustEnv.task?.status === 'running'"
                @update:value="(v: string) => downloadDirectoryModel = v"
              />
              <NButton
                secondary
                :disabled="rustEnv.task?.status === 'running'"
                @click="handleBrowseRustInstallDirectory"
              >
                <template #icon>
                  <NIcon :component="FolderOpenOutline" />
                </template>
              </NButton>
            </div>
          </div>
          <div
            v-if="resolvedRustInstallDirectory"
            class="rounded-lg border border-[rgb(var(--color-border-subtle)/0.78)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-3 py-2 text-xs leading-6 text-[rgb(var(--color-fg-muted)/0.95)]"
          >
            Rust 将安装到：{{ resolvedRustInstallDirectory }}/rust/{{ rustDownloadVersionModel || '...' }}；Zig 将安装到：{{ resolvedRustInstallDirectory }}/zig/{{ zigDownloadVersionModel || '...' }}
          </div>
        </div>

        <div class="flex justify-end">
          <NButton
            type="primary"
            :loading="rustEnv.installing && (!rustEnv.task || rustEnv.task.kind === 'install' || rustEnv.task.kind === 'install-rust' || rustEnv.task.kind === 'install-zig')"
            :disabled="!canInstallRustBundle || rustEnv.task?.status === 'running'"
            @click="handleStartRustInstall('bundle')"
          >
            下载 Rust + Zig
          </NButton>
        </div>
      </template>
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
