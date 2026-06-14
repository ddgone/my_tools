<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDropdown,
  NIcon,
  NInput,
  NPopconfirm,
  NProgress,
  NSelect,
  NSwitch,
  NTabPane,
  NTabs,
  NText,
  useMessage,
} from 'naive-ui'
import { Add, Checkmark, FolderOpenOutline, Trash } from '@vicons/ionicons5'
import { OpenFileDialog } from '../../wailsjs/go/main/App'
import { useGoEnvStore } from '@/stores/goenv'
import { useRustEnvStore } from '@/stores/rustenv'
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()
const goEnv = useGoEnvStore()
const rustEnv = useRustEnvStore()
const pythonEnv = usePythonEnvStore()
const message = useMessage()
const noSdkValue = '__no_sdk__'
const noRustToolValue = '__auto_detect__'
const noPythonValue = '__no_python__'
const goSourceLabelMap: Record<string, string> = {
  configured: '自定义路径',
  remembered: '历史路径',
  path: 'PATH 中的 Go',
  detected: '系统安装目录',
  managed: '托管 SDK',
}
const rustSourceLabelMap: Record<string, string> = {
  configured: '自定义路径',
  remembered: '历史路径',
  path: 'PATH 中发现',
  detected: '常见安装目录',
}

const showDownloadPanel = ref(false)
const downloadVersion = ref('')
const downloadDirectory = ref('')
const manualGoDownloadLinks = [
  { label: 'Go 官方下载页', href: 'https://go.dev/dl/' },
  { label: 'Go 国内镜像页', href: 'https://golang.google.cn/dl/' },
]

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
function buildRustCandidateOptions(candidates?: Array<{ valid: boolean, label: string, detail: string, source: string, path: string }>) {
  return [
    { label: '<自动检测>', value: noRustToolValue },
    ...(candidates ?? [])
      .filter(candidate => candidate.valid)
      .map(candidate => ({
        label: candidate.detail
          ? `${candidate.label} · ${describeRustSource(candidate.source)}  ${candidate.detail}`
          : `${candidate.label} · ${describeRustSource(candidate.source)}`,
        value: candidate.path,
      })),
  ]
}
const cargoCandidateOptions = computed(() => buildRustCandidateOptions(rustEnv.state?.cargoCandidates))
const rustupCandidateOptions = computed(() => buildRustCandidateOptions(rustEnv.state?.rustupCandidates))
const zigCandidateOptions = computed(() => buildRustCandidateOptions(rustEnv.state?.zigCandidates))
const cargoZigbuildCandidateOptions = computed(() => buildRustCandidateOptions(rustEnv.state?.cargoZigbuildCandidates))
const currentCargoBinary = computed(() => rustEnv.state?.config.selectedCargoBinary || rustEnv.state?.activeCargoBinary || noRustToolValue)
const currentRustupBinary = computed(() => rustEnv.state?.config.selectedRustupBinary || rustEnv.state?.activeRustupBinary || noRustToolValue)
const currentZigBinary = computed(() => rustEnv.state?.config.selectedZigBinary || rustEnv.state?.activeZigBinary || noRustToolValue)
const currentCargoZigbuildBinary = computed(() => rustEnv.state?.config.selectedCargoZigbuildBinary || rustEnv.state?.activeCargoZigbuildBinary || noRustToolValue)
const rustTargetStatuses = computed(() => rustEnv.state?.targetStatuses ?? [])
const rustInstalledCrossTargetCount = computed(() => rustTargetStatuses.value.filter(target => !target.native && target.installed).length)
const rustCrossTargetCount = computed(() => rustTargetStatuses.value.filter(target => !target.native).length)
const pythonCandidateOptions = computed(() =>
  [
    { label: '<无 Python>', value: noPythonValue },
    ...(pythonEnv.state?.candidates ?? [])
      .filter(candidate => candidate.valid)
      .map(candidate => ({
        label: candidate.detail ? `${candidate.label}  ${candidate.detail}` : candidate.label,
        value: candidate.path,
      })),
  ],
)
const currentPythonBinary = computed(() => {
  if (pythonEnv.state?.config.disabled) {
    return noPythonValue
  }
  return pythonEnv.state?.config.selectedBinary || pythonEnv.state?.activeBaseBinary || null
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
const pythonTaskActionLabel = computed(() => {
  const task = pythonEnv.task
  if (task?.status === 'running' && task.kind === 'prepare') {
    return '正在处理...'
  }
  if ((task?.status === 'failed' || task?.status === 'canceled') && task.kind === 'prepare') {
    return '继续创建工具环境'
  }
  return pythonEnv.state?.hasUsableBinary && !pythonEnv.state?.needsRebuild ? '重建工具环境' : '创建工具环境'
})
const pythonInstallActionLabel = computed(() => {
  const task = pythonEnv.task
  if (task?.status === 'running' && task.kind === 'install') {
    return '正在安装...'
  }
  if ((task?.status === 'failed' || task?.status === 'canceled') && task.kind === 'install') {
    return '继续安装依赖'
  }
  return '一键安装依赖'
})
const canInstall = computed(() => downloadVersion.value.trim() && downloadDirectory.value.trim())
const resolvedGoInstallDirectory = computed(() => resolveGoInstallTargetDirectory(downloadVersion.value, downloadDirectory.value))

function resetAll() {
  workspace.resetAllData()
  workspace.showSettings = false
  message.success('已恢复出厂设置')
}

async function ensureGoState() {
  if (!goEnv.state && !goEnv.loading) {
    await goEnv.loadState()
  }
}

async function ensurePythonState() {
  if (!pythonEnv.state && !pythonEnv.loading) {
    await pythonEnv.loadState()
  }
}

async function ensureRustState() {
  if (!rustEnv.state && !rustEnv.loading) {
    await rustEnv.loadState()
  }
}

async function ensureGoReleases() {
  await goEnv.ensureReleases()
  if (!downloadVersion.value && goEnv.releases.length > 0) {
    downloadVersion.value = goEnv.releases[0].version
  }
}

async function handleRetryGoReleases() {
  try {
    await goEnv.ensureReleases(true)
    if (!downloadVersion.value && goEnv.releases.length > 0) {
      downloadVersion.value = goEnv.releases[0].version
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
  const targetVersion = version || downloadVersion.value
  if (!targetVersion) {
    return
  }
  const lastInstallDirectory = goEnv.state?.config.lastInstallDirectory?.trim()
  if (lastInstallDirectory) {
    downloadDirectory.value = normalizeGoInstallBaseDirectory(lastInstallDirectory)
    return
  }
  const suggested = goEnv.state?.suggestedInstallDirectory?.trim()
  if (suggested) {
    downloadDirectory.value = normalizeGoInstallBaseDirectory(suggested)
  }
}

function describeGoSource(source?: string) {
  return goSourceLabelMap[source || ''] || '自动检测'
}

function describeRustSource(source?: string) {
  return rustSourceLabelMap[source || ''] || '自动检测'
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
    showDownloadPanel.value = false
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

async function handleOpenLocalRustTool(kind: 'cargo' | 'rustup' | 'zig' | 'cargo-zigbuild') {
  const titleMap: Record<typeof kind, string> = {
    cargo: '选择 cargo 可执行文件',
    rustup: '选择 rustup 可执行文件',
    zig: '选择 zig 可执行文件',
    'cargo-zigbuild': '选择 cargo-zigbuild 可执行文件',
  }
  try {
    const filePath = await OpenFileDialog({
      title: titleMap[kind],
      filterName: 'Executable',
      filterGlob: '*',
      directory: false,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (!filePath) {
      return
    }
    await rustEnv.chooseBinary(kind, filePath)
    message.success('Rust 工具链已更新')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '选择 Rust 工具链失败')
  }
}

async function handleSelectRustTool(kind: 'cargo' | 'rustup' | 'zig' | 'cargo-zigbuild', value: string | null) {
  if (!value) {
    return
  }
  try {
    if (value === noRustToolValue) {
      await rustEnv.clearSelection(kind)
      message.success('已恢复为自动检测')
      return
    }
    await rustEnv.chooseBinary(kind, value)
    message.success('Rust 工具链已切换')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '切换 Rust 工具链失败')
  }
}

async function handleCheckRustEnvironment() {
  try {
    const state = await rustEnv.checkEnvironment()
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

async function handleOpenLocalPython() {
  try {
    const filePath = await OpenFileDialog({
      title: '选择 Python 可执行文件',
      filterName: 'Python',
      filterGlob: '*',
      directory: false,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (!filePath) {
      return
    }
    await pythonEnv.chooseBinary(filePath)
    message.success('基础 Python 已更新')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '选择本地 Python 失败')
  }
}

async function handleSelectPython(value: string | null) {
  if (!value) {
    return
  }
  try {
    if (value === noPythonValue) {
      await pythonEnv.clearSelection()
      message.success('已清除 Python 环境配置')
      return
    }
    await pythonEnv.chooseBinary(value)
    message.success('基础 Python 已切换')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '切换 Python 环境失败')
  }
}

async function handleInstallPythonDependencies() {
  try {
    await pythonEnv.installDependencies()
    message.info('已开始安装 Python 依赖')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '安装 Python 依赖失败')
  }
}

async function handlePreparePythonEnvironment() {
  try {
    await pythonEnv.prepareEnvironment()
    message.info('已开始更新 Python 工具环境')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '创建 Python 工具环境失败')
  }
}

async function handleCancelPythonTask() {
  try {
    await pythonEnv.cancelTask()
    message.info('已请求停止当前 Python 环境任务')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '停止 Python 环境任务失败')
  }
}

async function handleCheckPythonEnvironment() {
  try {
    await pythonEnv.checkEnvironment()
    message.success('当前 Python 工具环境检查通过')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '检查 Python 工具环境失败')
  }
}

async function handleDeletePythonEnvironment() {
  try {
    await pythonEnv.deleteEnvironment()
    message.success('当前 Python 工具环境已删除')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '删除 Python 工具环境失败')
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
      downloadDirectory.value = dir
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
    await goEnv.install(downloadVersion.value, downloadDirectory.value)
    message.info('已开始下载并安装 Go SDK')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '安装 Go SDK 失败')
  }
}

async function handleRetryGoInstall() {
  if (!downloadVersion.value && goEnv.task?.version) {
    downloadVersion.value = goEnv.task.version
  }
  if (!downloadDirectory.value && goEnv.task?.directory) {
    downloadDirectory.value = normalizeGoInstallBaseDirectory(goEnv.task.directory)
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

async function handleGoMenuSelect(key: string) {
  if (key === 'local') {
    await handleOpenLocalGo()
    return
  }
  showDownloadPanel.value = true
  await ensureGoState()
  await ensureGoReleases()
  ensureDownloadDirectory()
}

watch(() => workspace.showSettings, async (visible) => {
  if (visible && workspace.settings.lastSettingsTab === 'go') {
    await ensureGoState()
  }
  if (visible && workspace.settings.lastSettingsTab === 'rust') {
    await ensureRustState()
  }
  if (visible && workspace.settings.lastSettingsTab === 'python') {
    await ensurePythonState()
  }
}, { immediate: true })

watch(() => workspace.settings.lastSettingsTab, async (tab) => {
  if (tab === 'go' && workspace.showSettings) {
    await ensureGoState()
  }
  if (tab === 'rust' && workspace.showSettings) {
    await ensureRustState()
  }
  if (tab === 'python' && workspace.showSettings) {
    await ensurePythonState()
  }
})

watch(downloadVersion, (value) => {
  if (!showDownloadPanel.value) {
    return
  }
  ensureDownloadDirectory(value)
})

onMounted(async () => {
  await Promise.all([ensureGoState(), ensureRustState(), ensurePythonState()])
})
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="workspace.showSettings"
        class="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
        @click="workspace.showSettings = false"
      />
    </Transition>
    <Transition
      name="fade-scale"
      appear
    >
      <div
        v-if="workspace.showSettings"
        class="fixed inset-0 z-50 flex items-start justify-center px-4 py-[4vh] pointer-events-none"
      >
        <div
          class="pointer-events-auto flex w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-white/15 bg-dracula-panel shadow-2xl max-h-[92vh]"
          @click.stop
        >
          <div class="flex items-center justify-between border-b border-white/15 px-5 py-3">
            <NText class="text-sm font-semibold">
              系统首选项
            </NText>
            <NButton
              text
              size="tiny"
              @click="workspace.showSettings = false"
            >
              ESC 关闭
            </NButton>
          </div>

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
            <NTabs
              type="line"
              animated
              :value="workspace.settings.lastSettingsTab"
              @update:value="(v: string) => workspace.settings.lastSettingsTab = v === 'export' || v === 'go' || v === 'rust' || v === 'python' ? v : 'general'"
            >
              <NTabPane
                name="general"
                tab="通用"
              >
                <div class="settings-form pt-2">
                  <div class="settings-row">
                    <div class="settings-label">
                      最近使用显示数量
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.recentToolsCount"
                        :options="[{ label: '3', value: 3 }, { label: '5', value: 5 }, { label: '10', value: 10 }]"
                        @update:value="(v: number) => workspace.settings.recentToolsCount = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      命令历史保留数量
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.historyRetention"
                        :options="[{ label: '20', value: 20 }, { label: '50', value: 50 }, { label: '100', value: 100 }, { label: '200', value: 200 }]"
                        @update:value="(v: number) => workspace.settings.historyRetention = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      日志导出目录
                    </div>
                    <div class="settings-value">
                      <NInput
                        class="settings-control"
                        :value="workspace.settings.logExportDir"
                        placeholder="my_tools_logs"
                        @update:value="(v: string) => workspace.settings.logExportDir = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      退出前确认
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.confirmExit"
                        @update:value="(v: boolean) => workspace.settings.confirmExit = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      终端输出自动换行
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.autoWordWrap"
                        @update:value="(v: boolean) => workspace.settings.autoWordWrap = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      启动时展开所有分类
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.autoExpandAll"
                        @update:value="(v: boolean) => workspace.settings.autoExpandAll = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      快捷键提示模式
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.verboseShortcuts ? 'verbose' : 'compact'"
                        :options="[{ label: '精简模式', value: 'compact' }, { label: '详细模式', value: 'verbose' }]"
                        @update:value="(v: string | number) => workspace.settings.verboseShortcuts = v === 'verbose'"
                      />
                    </div>
                  </div>
                </div>
              </NTabPane>

              <NTabPane
                name="export"
                tab="导出"
              >
                <div class="settings-form pt-2">
                  <div class="settings-row">
                    <div class="settings-label">
                      导出成功后自动打开目录
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.autoOpenExportDir"
                        @update:value="(v: boolean) => workspace.settings.autoOpenExportDir = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      Go 工具默认导出内容
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.goExportMode"
                        :options="[{ label: '二进制', value: 'binary' }, { label: '源码', value: 'source' }]"
                        @update:value="(v: string | number) => workspace.settings.goExportMode = v === 'source' ? 'source' : 'binary'"
                      />
                    </div>
                  </div>
                </div>
              </NTabPane>

              <NTabPane
                name="go"
                tab="Go"
              >
                <div class="pt-2 space-y-4">
                  <div
                    class="rounded-xl border px-4 py-4"
                    :class="goEnv.hasUsableBinary ? 'border-emerald-400/20 bg-emerald-500/5' : 'border-amber-400/20 bg-amber-500/5'"
                  >
                    <div class="text-base font-semibold text-dracula-text">
                      {{ goEnv.hasUsableBinary ? 'Go 环境已就绪' : '当前未检测到可用的 Go 环境' }}
                    </div>
                    <div class="mt-2 text-sm text-white/70 leading-6">
                      {{
                        goEnv.hasUsableBinary
                          ? `当前使用 ${goEnv.state?.activeVersion || 'Go'}，整个桌面宿主共用这一份 Go 环境。`
                          : '请先选择本地 Go，或直接下载一个 Go SDK。配置完成后会立即生效。'
                      }}
                    </div>
                    <div
                      v-if="goEnv.state?.statusMessage"
                      class="mt-3 text-xs text-white/55"
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
                        <div class="text-sm font-medium text-dracula-text">
                          Go SDK 下载任务
                        </div>
                        <div class="mt-1 text-xs text-white/70">
                          {{ goEnv.task.message || '正在处理 Go SDK 下载任务' }}
                        </div>
                        <div
                          v-if="goEnv.task.currentItem"
                          class="mt-1 text-[11px] text-white/45 break-all"
                        >
                          当前项：{{ goEnv.task.currentItem }}
                        </div>
                        <div
                          v-if="goEnv.task.detail && !formatTransferSummary()"
                          class="mt-1 text-[11px] text-white/45 break-all"
                        >
                          {{ goEnv.task.detail }}
                        </div>
                        <div
                          v-if="formatTransferSummary()"
                          class="mt-1 text-[11px] text-cyan-200/80"
                        >
                          {{ formatTransferSummary() }}
                        </div>
                        <div
                          v-if="goEnv.task.currentSource"
                          class="mt-1 text-[11px] text-white/45 break-all"
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
                    <div class="mt-2 text-[11px] text-white/45">
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
                      class="mt-3 text-[11px] text-white/55 leading-6"
                    >
                      自动安装不通时，可手动下载后通过“本地”选择 go.exe：
                      <a
                        v-for="link in manualGoDownloadLinks"
                        :key="link.href"
                        :href="link.href"
                        target="_blank"
                        rel="noreferrer"
                        class="ml-2 text-cyan-200 hover:text-cyan-100 underline underline-offset-2"
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
                          @select="(key: string) => handleGoMenuSelect(key)"
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
                        <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70 break-all">
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
                        <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
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
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              GOVERSION
                            </div>
                            <div class="break-all">
                              {{ goEnv.state?.runtimeDetails?.goversion || goEnv.state?.activeVersion || '未知' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              GOOS / GOARCH
                            </div>
                            <div class="break-all">
                              {{ [goEnv.state?.runtimeDetails?.goos, goEnv.state?.runtimeDetails?.goarch].filter(Boolean).join(' / ') || '未知' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70 md:col-span-2">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              GOROOT
                            </div>
                            <div class="break-all">
                              {{ goEnv.state?.runtimeDetails?.goroot || '未知' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70 md:col-span-2">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
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
                    v-if="showDownloadPanel"
                    class="rounded-xl border border-white/10 bg-black/10 p-4 space-y-4"
                  >
                    <div class="flex items-center justify-between">
                      <div class="text-sm font-medium text-dracula-text">
                        下载 Go SDK
                      </div>
                      <NButton
                        text
                        size="small"
                        @click="showDownloadPanel = false"
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
                          class="text-cyan-200 hover:text-cyan-100 underline underline-offset-2"
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
                            :value="downloadVersion"
                            :options="goEnv.releases.map(release => ({ label: release.version, value: release.version }))"
                            :loading="goEnv.releaseLoading"
                            :disabled="goEnv.task?.status === 'running'"
                            placeholder="选择 Go 版本"
                            @update:value="(v: string | null) => downloadVersion = v || ''"
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
                            :value="downloadDirectory"
                            placeholder="选择 Go SDK 集中安装目录"
                            :disabled="goEnv.task?.status === 'running'"
                            @update:value="(v: string) => downloadDirectory = v"
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
                        class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/55"
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
              </NTabPane>

              <NTabPane
                name="rust"
                tab="Rust"
              >
                <div class="pt-2 space-y-4">
                  <div
                    class="rounded-xl border px-4 py-4"
                    :class="rustEnv.hasUsableEnvironment ? 'border-[rgb(222,165,132)]/25 bg-[rgba(222,165,132,0.08)]' : 'border-amber-400/20 bg-amber-500/5'"
                  >
                    <div class="text-base font-semibold text-dracula-text">
                      {{
                        !rustEnv.hasUsableEnvironment
                          ? 'Rust 交叉编译环境未就绪'
                          : rustEnv.state?.hasInstalledTargetInfo && !rustEnv.state?.hasFullTargetCoverage
                            ? 'Rust 工具链已就绪，常用 targets 待补齐'
                            : 'Rust 交叉编译环境已就绪'
                      }}
                    </div>
                    <div class="mt-2 text-sm text-white/70 leading-6">
                      {{
                        rustEnv.hasUsableEnvironment
                          ? rustEnv.state?.hasInstalledTargetInfo && !rustEnv.state?.hasFullTargetCoverage
                            ? '基础工具链已经可用；缺失的常用 targets 可以在这里提前发现，首次实际构建时程序也会尝试自动执行 rustup target add。'
                            : '本地 Rust 工具运行继续使用随宿主打包的二进制；远程执行、导出和批量构建缓存会使用这里配置的 cargo / rustup / zig / cargo-zigbuild。'
                          : 'Rust 工具本地运行不受影响；远程执行、导出和批量构建缓存需要补齐 cargo、rustup、zig 与 cargo-zigbuild。'
                      }}
                    </div>
                    <div
                      v-if="rustEnv.state?.statusMessage"
                      class="mt-3 text-xs text-white/55"
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

                  <div class="settings-form">
                    <div class="settings-row">
                      <div class="settings-label">
                        cargo
                      </div>
                      <div class="settings-value gap-2">
                        <NSelect
                          class="settings-control"
                          :value="currentCargoBinary"
                          :options="cargoCandidateOptions"
                          :placeholder="rustEnv.loading ? '正在检测 cargo...' : '选择 cargo 或保持自动检测'"
                          :loading="rustEnv.loading"
                          @update:value="(v: string | null) => handleSelectRustTool('cargo', v)"
                        />
                        <NButton
                          secondary
                          size="medium"
                          @click="handleOpenLocalRustTool('cargo')"
                        >
                          <template #icon>
                            <NIcon :component="Add" />
                          </template>
                        </NButton>
                      </div>
                    </div>

                    <div class="settings-row">
                      <div class="settings-label">
                        rustup
                      </div>
                      <div class="settings-value gap-2">
                        <NSelect
                          class="settings-control"
                          :value="currentRustupBinary"
                          :options="rustupCandidateOptions"
                          :placeholder="rustEnv.loading ? '正在检测 rustup...' : '选择 rustup 或保持自动检测'"
                          :loading="rustEnv.loading"
                          @update:value="(v: string | null) => handleSelectRustTool('rustup', v)"
                        />
                        <NButton
                          secondary
                          size="medium"
                          @click="handleOpenLocalRustTool('rustup')"
                        >
                          <template #icon>
                            <NIcon :component="Add" />
                          </template>
                        </NButton>
                      </div>
                    </div>

                    <div class="settings-row">
                      <div class="settings-label">
                        zig
                      </div>
                      <div class="settings-value gap-2">
                        <NSelect
                          class="settings-control"
                          :value="currentZigBinary"
                          :options="zigCandidateOptions"
                          :placeholder="rustEnv.loading ? '正在检测 zig...' : '选择 zig 或保持自动检测'"
                          :loading="rustEnv.loading"
                          @update:value="(v: string | null) => handleSelectRustTool('zig', v)"
                        />
                        <NButton
                          secondary
                          size="medium"
                          @click="handleOpenLocalRustTool('zig')"
                        >
                          <template #icon>
                            <NIcon :component="Add" />
                          </template>
                        </NButton>
                      </div>
                    </div>

                    <div class="settings-row">
                      <div class="settings-label">
                        cargo-zigbuild
                      </div>
                      <div class="settings-value gap-2">
                        <NSelect
                          class="settings-control"
                          :value="currentCargoZigbuildBinary"
                          :options="cargoZigbuildCandidateOptions"
                          :placeholder="rustEnv.loading ? '正在检测 cargo-zigbuild...' : '选择 cargo-zigbuild 或保持自动检测'"
                          :loading="rustEnv.loading"
                          @update:value="(v: string | null) => handleSelectRustTool('cargo-zigbuild', v)"
                        />
                        <NButton
                          secondary
                          size="medium"
                          @click="handleOpenLocalRustTool('cargo-zigbuild')"
                        >
                          <template #icon>
                            <NIcon :component="Add" />
                          </template>
                        </NButton>
                      </div>
                    </div>

                    <div class="settings-row align-start">
                      <div class="settings-label">
                        当前状态
                      </div>
                      <div class="settings-value">
                        <div class="grid w-full gap-2 md:grid-cols-2">
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              cargo
                            </div>
                            <div class="break-all">
                              {{ rustEnv.state?.activeCargoVersion || '未就绪' }}
                            </div>
                            <div class="mt-1 break-all text-[11px] text-white/45">
                              {{ rustEnv.state?.activeCargoBinary || '未检测到可用路径' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              rustup
                            </div>
                            <div class="break-all">
                              {{ rustEnv.state?.activeRustupVersion || '未就绪' }}
                            </div>
                            <div class="mt-1 break-all text-[11px] text-white/45">
                              {{ rustEnv.state?.activeRustupBinary || '未检测到可用路径' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              zig
                            </div>
                            <div class="break-all">
                              {{ rustEnv.state?.activeZigVersion || '未就绪' }}
                            </div>
                            <div class="mt-1 break-all text-[11px] text-white/45">
                              {{ rustEnv.state?.activeZigBinary || '未检测到可用路径' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              cargo-zigbuild
                            </div>
                            <div class="break-all">
                              {{ rustEnv.state?.activeCargoZigbuildVersion || '未就绪' }}
                            </div>
                            <div class="mt-1 break-all text-[11px] text-white/45">
                              {{ rustEnv.state?.activeCargoZigbuildBinary || '未检测到可用路径' }}
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>

                    <div
                      v-if="rustEnv.state?.hasInstalledTargetInfo"
                      class="settings-row align-start"
                    >
                      <div class="settings-label">
                        常用 targets
                      </div>
                      <div class="settings-value">
                        <div class="w-full space-y-3">
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-3 text-xs leading-6 text-white/70">
                            <div class="flex flex-wrap items-center justify-between gap-3">
                              <div>
                                已安装 {{ rustInstalledCrossTargetCount }} / {{ rustCrossTargetCount }} 个常用交叉编译 target
                              </div>
                              <div
                                class="text-[11px]"
                                :class="rustEnv.state?.hasFullTargetCoverage ? 'text-emerald-300/85' : 'text-amber-300/85'"
                              >
                                {{ rustEnv.state?.targetStatusMessage || '尚未读取 target 状态' }}
                              </div>
                            </div>
                          </div>
                          <div class="grid gap-2 md:grid-cols-2">
                            <div
                              v-for="target in rustTargetStatuses"
                              :key="target.platformKey"
                              class="rounded-lg border px-3 py-2 text-xs leading-6"
                              :class="target.installed
                                ? 'border-emerald-400/15 bg-emerald-500/[0.05] text-white/75'
                                : 'border-amber-400/15 bg-amber-500/[0.05] text-white/75'"
                            >
                              <div class="flex items-center justify-between gap-3">
                                <div class="font-medium text-white/85">
                                  {{ target.platformLabel }}
                                </div>
                                <div
                                  class="text-[11px]"
                                  :class="target.installed ? 'text-emerald-300/85' : 'text-amber-300/85'"
                                >
                                  {{ target.native ? '原生构建' : (target.installed ? '已安装' : '未安装') }}
                                </div>
                              </div>
                              <div class="mt-1 break-all text-[11px] text-white/50">
                                {{ target.targetTriple }}
                              </div>
                              <div
                                v-if="target.note"
                                class="mt-1 text-[11px] text-white/45"
                              >
                                {{ target.note }}
                              </div>
                            </div>
                          </div>
                          <div
                            v-if="(rustEnv.state?.installedTargets?.length || 0) > 0"
                            class="rounded-lg border border-white/10 bg-black/10 px-3 py-3 text-[11px] leading-6 text-white/55"
                          >
                            已安装 target 列表：{{ rustEnv.state?.installedTargets.join('、') }}
                          </div>
                        </div>
                      </div>
                    </div>

                    <div class="flex flex-wrap justify-end gap-2">
                      <NButton
                        secondary
                        size="small"
                        :loading="rustEnv.checking"
                        :disabled="rustEnv.loading || rustEnv.saving"
                        @click="handleCheckRustEnvironment"
                      >
                        检查环境
                      </NButton>
                    </div>
                  </div>
                </div>
              </NTabPane>

              <NTabPane
                name="python"
                tab="Python"
              >
                <div class="pt-2 space-y-4">
                  <div
                    class="rounded-xl border px-4 py-4"
                    :class="pythonEnv.hasUsableBinary && pythonEnv.state?.dependenciesReady
                      ? 'border-emerald-400/20 bg-emerald-500/5'
                      : 'border-amber-400/20 bg-amber-500/5'"
                  >
                    <div class="text-base font-semibold text-dracula-text">
                      {{
                        !pythonEnv.state?.hasUsableBaseBinary
                          ? '当前未检测到可用的基础 Python'
                          : !pythonEnv.state?.hasUsableBinary
                            ? '基础 Python 已就绪，工具环境尚未创建'
                            : !pythonEnv.state?.pipAvailable
                              ? 'Python 工具环境缺少 pip'
                              : pythonEnv.state?.dependenciesReady
                                ? 'Python 工具环境已就绪'
                                : 'Python 工具环境已创建，但依赖尚未安装'
                      }}
                    </div>
                    <div class="mt-2 text-sm text-white/70 leading-6">
                      {{
                        !pythonEnv.state?.hasUsableBaseBinary
                          ? '请先选择一个本地 Python 3 作为基础解释器。程序会在 toolchains 下自动创建托管虚拟环境。'
                          : !pythonEnv.state?.hasUsableBinary
                            ? `当前基础解释器是 ${pythonEnv.state?.activeBaseVersion || 'Python'}，请先创建托管工具环境。`
                            : !pythonEnv.state?.pipAvailable
                              ? `当前工具环境使用 ${pythonEnv.state?.activeVersion || 'Python'}，但未检测到 pip，建议重建工具环境。`
                              : pythonEnv.state?.dependenciesReady
                                ? `当前基础解释器是 ${pythonEnv.state?.activeBaseVersion || 'Python'}，本地 Python 工具统一运行在托管虚拟环境中。`
                                : `当前工具环境使用 ${pythonEnv.state?.activeVersion || 'Python'}，还需要安装动态扫描到的依赖后才能稳定运行 Python 工具。`
                      }}
                    </div>
                    <div
                      v-if="pythonEnv.state?.statusMessage"
                      class="mt-3 text-xs text-white/55"
                    >
                      {{ pythonEnv.state.statusMessage }}
                    </div>
                  </div>

                  <NAlert
                    v-if="pythonEnv.error"
                    type="error"
                    :show-icon="false"
                  >
                    {{ pythonEnv.error }}
                  </NAlert>

                  <div
                    v-if="pythonEnv.task"
                    class="rounded-xl border border-white/10 bg-black/10 px-4 py-4"
                  >
                    <div class="flex items-start justify-between gap-4">
                      <div class="min-w-0">
                        <div class="text-sm font-medium text-dracula-text">
                          {{ pythonEnv.task.kind === 'install' ? '依赖安装任务' : '工具环境任务' }}
                        </div>
                        <div class="mt-1 text-xs leading-6 text-white/70">
                          {{ pythonEnv.task.message || '正在处理 Python 环境任务' }}
                        </div>
                        <div
                          v-if="pythonEnv.task.currentItem"
                          class="text-[11px] text-white/45 break-all"
                        >
                          当前项：{{ pythonEnv.task.currentItem }}
                        </div>
                        <div
                          v-if="pythonEnv.task.detail"
                          class="text-[11px] text-white/45 break-all"
                        >
                          {{ pythonEnv.task.detail }}
                        </div>
                      </div>
                      <NButton
                        v-if="pythonEnv.task.status === 'running'"
                        tertiary
                        type="warning"
                        size="small"
                        @click="handleCancelPythonTask"
                      >
                        停止
                      </NButton>
                    </div>
                    <div class="mt-3">
                      <NProgress
                        type="line"
                        :percentage="Math.max(0, Math.min(100, pythonEnv.task.progressPercent || 0))"
                        :show-indicator="true"
                        :format="(percentage: number) => `${Math.round(percentage)}%`"
                        processing
                      />
                    </div>
                    <div class="mt-2 text-[11px] text-white/45">
                      状态：{{ pythonEnv.task.status }}
                      <span v-if="pythonEnv.task.totalSteps > 0"> · 步骤 {{ pythonEnv.task.step }}/{{ pythonEnv.task.totalSteps }}</span>
                    </div>
                    <div
                      v-if="pythonEnv.task.error"
                      class="mt-2 text-[11px] text-rose-200/85 break-all"
                    >
                      {{ pythonEnv.task.error }}
                    </div>
                  </div>

                  <div class="settings-form">
                    <div class="settings-row">
                      <div class="settings-label">
                        基础 Python
                      </div>
                      <div class="settings-value gap-2">
                        <NSelect
                          class="settings-control"
                          :value="currentPythonBinary"
                          :options="pythonCandidateOptions"
                          :placeholder="pythonEnv.loading ? '正在检测基础 Python...' : '尚未选择基础 Python'"
                          :loading="pythonEnv.loading"
                          @update:value="(v: string | null) => handleSelectPython(v)"
                        />
                        <NDropdown
                          trigger="click"
                          :options="[{ label: '本地', key: 'local' }]"
                          @select="handleOpenLocalPython"
                        >
                          <NButton
                            secondary
                            size="medium"
                          >
                            <template #icon>
                              <NIcon :component="Add" />
                            </template>
                          </NButton>
                        </NDropdown>
                      </div>
                    </div>

                    <div
                      v-if="pythonEnv.state?.activeBaseBinary"
                      class="settings-row align-start"
                    >
                      <div class="settings-label">
                        基础解释器路径
                      </div>
                      <div class="settings-value">
                        <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70 break-all">
                          {{ pythonEnv.state.activeBaseBinary }}
                        </div>
                      </div>
                    </div>

                    <div class="settings-row">
                      <div class="settings-label">
                        工具环境
                      </div>
                      <div class="settings-value">
                        <div class="w-full rounded-lg border border-white/10 bg-black/10 px-3 py-3 text-xs leading-6 text-white/70">
                          {{
                            !pythonEnv.state?.hasUsableBaseBinary
                              ? '尚未选择基础 Python'
                              : !pythonEnv.state?.hasUsableBinary
                                ? (pythonEnv.state?.needsRebuild ? '需要创建或重建托管工具环境' : '托管工具环境尚未创建')
                                : pythonEnv.state?.pipAvailable
                                  ? '托管工具环境与 pip 已就绪'
                                  : '托管工具环境缺少 pip，建议重建'
                          }}
                          <div
                            v-if="pythonEnv.state?.managedEnvDirectory"
                            class="mt-2 break-all text-[11px] text-white/45"
                          >
                            {{ pythonEnv.state.managedEnvDirectory }}
                          </div>
                          <div class="mt-3 flex justify-end">
                            <div class="flex flex-wrap justify-end gap-2">
                              <NButton
                                secondary
                                size="small"
                                :disabled="!pythonEnv.state?.hasUsableBaseBinary || pythonEnv.task?.status === 'running'"
                                :loading="pythonEnv.checking"
                                @click="handleCheckPythonEnvironment"
                              >
                                检查环境
                              </NButton>
                              <NPopconfirm
                                @positive-click="handleDeletePythonEnvironment"
                              >
                                <template #trigger>
                                  <NButton
                                    secondary
                                    size="small"
                                    type="error"
                                    :disabled="!pythonEnv.state?.managedEnvDirectory || pythonEnv.task?.status === 'running'"
                                    :loading="pythonEnv.deleting"
                                  >
                                    删除环境
                                  </NButton>
                                </template>
                                确定删除当前基础 Python 对应的托管工具环境吗？基础 Python 选择会保留。
                              </NPopconfirm>
                              <NButton
                                secondary
                                size="small"
                                :disabled="!pythonEnv.state?.hasUsableBaseBinary || pythonEnv.task?.status === 'running'"
                                :loading="pythonEnv.preparing"
                                @click="handlePreparePythonEnvironment"
                              >
                                {{ pythonTaskActionLabel }}
                              </NButton>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>

                    <div class="settings-row align-start">
                      <div class="settings-label">
                        动态依赖
                      </div>
                      <div class="settings-value">
                        <div class="w-full rounded-lg border border-white/10 bg-black/10 px-3 py-3">
                          <div class="flex items-center justify-between gap-3">
                            <div class="text-xs text-white/70">
                              已扫描 {{ pythonEnv.state?.dependencyToolCount || 0 }} 个 Python 工具，共识别 {{ pythonEnv.state?.dependencyTotalCount || 0 }} 个依赖包。
                            </div>
                            <NButton
                              type="primary"
                              size="small"
                              :disabled="!pythonEnv.state?.hasUsableBaseBinary || pythonEnv.task?.status === 'running'"
                              :loading="pythonEnv.installing"
                              @click="handleInstallPythonDependencies"
                            >
                              {{ pythonInstallActionLabel }}
                            </NButton>
                          </div>
                          <div
                            v-if="(pythonEnv.state?.dependencies?.length || 0) > 0"
                            class="mt-3 space-y-2"
                          >
                            <div
                              v-for="dependency in pythonEnv.state?.dependencies"
                              :key="dependency.packageName"
                              class="rounded-lg border border-white/8 bg-white/[0.03] px-3 py-2"
                            >
                              <div class="flex items-center justify-between gap-3">
                                <div class="min-w-0 text-xs text-white/85 break-all">
                                  {{ dependency.packageName }}
                                  <span class="text-white/35"> · </span>
                                  <span class="text-white/55">{{ dependency.moduleName }}</span>
                                </div>
                                <div
                                  class="shrink-0 text-[11px]"
                                  :class="dependency.installed ? 'text-emerald-300' : 'text-amber-300'"
                                >
                                  {{ dependency.installed ? (dependency.version || '已安装') : '未安装' }}
                                </div>
                              </div>
                              <div class="mt-1 text-[11px] text-white/45">
                                影响工具：{{ dependency.requiredBy.join('、') || '未知' }}
                              </div>
                              <div
                                v-if="dependency.error && !dependency.installed"
                                class="mt-1 text-[11px] text-amber-200/80"
                              >
                                {{ dependency.error }}
                              </div>
                            </div>
                          </div>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              </NTabPane>
            </NTabs>
          </div>

          <div class="flex items-center justify-between border-t border-white/15 px-5 py-3">
            <NPopconfirm @positive-click="resetAll">
              <template #trigger>
                <NButton
                  type="error"
                  size="small"
                  secondary
                >
                  <template #icon>
                    <NIcon :component="Trash" />
                  </template>
                  初始化应用
                </NButton>
              </template>
              确定要清除所有数据并恢复出厂设置吗？此操作不可撤销。
            </NPopconfirm>
            <NButton
              type="primary"
              size="small"
              @click="workspace.showSettings = false"
            >
              <template #icon>
                <NIcon :component="Checkmark" />
              </template>
              完成
            </NButton>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
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
  color: rgba(248, 248, 242, 0.9);
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
