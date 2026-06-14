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
  managed: '托管工具链',
}
const rustModeOptions = [
  { label: '无 SDK', value: 'none' },
  { label: '自动探测', value: 'auto' },
  { label: '手动选择', value: 'manual' },
]

const showDownloadPanel = ref(false)
const downloadVersion = ref('')
const downloadDirectory = ref('')
const rustDownloadVersion = ref('')
const zigDownloadVersion = ref('')
const rustDownloadDirectory = ref('')
const manualGoDownloadLinks = [
  { label: 'Go 官方下载页', href: 'https://go.dev/dl/' },
  { label: 'Go 国内镜像页', href: 'https://golang.google.cn/dl/' },
]
const manualRustDownloadLinks = [
  { label: 'Rust 官方安装页', href: 'https://www.rust-lang.org/tools/install' },
  { label: 'Zig 官方下载页', href: 'https://ziglang.org/download/' },
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
  return (candidates ?? [])
    .filter(candidate => candidate.valid)
    .map(candidate => ({
      label: candidate.detail
        ? `${candidate.label} · ${describeRustSource(candidate.source)}  ${candidate.detail}`
        : `${candidate.label} · ${describeRustSource(candidate.source)}`,
      value: candidate.path,
    }))
}
const rustCandidateOptions = computed(() =>
  (rustEnv.state?.rustCandidates ?? [])
    .filter(candidate => candidate.valid)
    .map(candidate => ({
      label: `${candidate.label} · ${describeRustSource(candidate.source)}  ${candidate.detail}`,
      value: candidate.rootDir,
    })),
)
const zigCandidateOptions = computed(() => buildRustCandidateOptions(rustEnv.state?.zigCandidates))
const currentRustMode = computed(() => rustEnv.state?.config.mode || (rustEnv.state?.config.disabled ? 'none' : 'auto'))
const currentRustRoot = computed(() => rustEnv.state?.config.selectedRustRoot || rustEnv.state?.activeRustRoot || null)
const currentZigBinary = computed(() => rustEnv.state?.config.selectedZigBinary || rustEnv.state?.activeZigBinary || null)
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
const canInstallRustOnly = computed(() => rustDownloadVersion.value.trim() && rustDownloadDirectory.value.trim())
const canInstallZigOnly = computed(() => zigDownloadVersion.value.trim() && rustDownloadDirectory.value.trim())
const canInstallRustBundle = computed(() => canInstallRustOnly.value && canInstallZigOnly.value)
const resolvedRustInstallDirectory = computed(() => normalizeRustInstallBaseDirectory(rustDownloadDirectory.value))

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

async function ensureRustReleases() {
  await rustEnv.ensureReleases()
  if (!rustDownloadVersion.value && rustEnv.rustReleases.length > 0) {
    rustDownloadVersion.value = rustEnv.rustReleases[0].version
  }
  if (!zigDownloadVersion.value && rustEnv.zigReleases.length > 0) {
    zigDownloadVersion.value = rustEnv.zigReleases[0].version
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

async function handleRetryRustReleases() {
  try {
    await rustEnv.ensureReleases(true)
    if (!rustDownloadVersion.value && rustEnv.rustReleases.length > 0) {
      rustDownloadVersion.value = rustEnv.rustReleases[0].version
    }
    if (!zigDownloadVersion.value && rustEnv.zigReleases.length > 0) {
      zigDownloadVersion.value = rustEnv.zigReleases[0].version
    }
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

function ensureRustDownloadDirectory() {
  const lastInstallDirectory = rustEnv.state?.config.lastInstallDirectory?.trim()
  if (lastInstallDirectory) {
    rustDownloadDirectory.value = normalizeRustInstallBaseDirectory(lastInstallDirectory)
    return
  }
  const suggested = rustEnv.state?.suggestedInstallDirectory?.trim()
  if (suggested) {
    rustDownloadDirectory.value = normalizeRustInstallBaseDirectory(suggested)
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
      rustDownloadDirectory.value = dir
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
      installRust ? rustDownloadVersion.value : '',
      installZig ? zigDownloadVersion.value : '',
      rustDownloadDirectory.value,
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
  if (!rustDownloadVersion.value && rustEnv.task?.rustVersion) {
    rustDownloadVersion.value = rustEnv.task.rustVersion
  }
  if (!zigDownloadVersion.value && rustEnv.task?.zigVersion) {
    zigDownloadVersion.value = rustEnv.task.zigVersion
  }
  if (!rustDownloadDirectory.value && rustEnv.task?.directory) {
    rustDownloadDirectory.value = normalizeRustInstallBaseDirectory(rustEnv.task.directory)
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

watch(() => workspace.showSettings, async (visible) => {
  if (visible && workspace.settings.lastSettingsTab === 'go') {
    await ensureGoState()
  }
  if (visible && workspace.settings.lastSettingsTab === 'rust') {
    await ensureRustState()
    await ensureRustReleases()
    ensureRustDownloadDirectory()
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
    await ensureRustReleases()
    ensureRustDownloadDirectory()
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
  await ensureRustReleases()
  ensureRustDownloadDirectory()
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
                    :class="rustEnv.state?.config.disabled
                      ? 'border-white/10 bg-white/[0.03]'
                      : rustEnv.hasUsableEnvironment
                        ? 'border-[rgb(222,165,132)]/25 bg-[rgba(222,165,132,0.08)]'
                        : 'border-amber-400/20 bg-amber-500/5'"
                  >
                    <div class="text-base font-semibold text-dracula-text">
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
                    <div class="mt-2 text-sm text-white/70 leading-6">
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

                  <div
                    v-if="rustEnv.task"
                    class="rounded-xl border border-[rgb(245,126,62)]/20 bg-[rgba(245,126,62,0.06)] px-4 py-4"
                  >
                    <div class="flex items-center justify-between gap-3">
                      <div>
                        <div class="text-sm font-medium text-dracula-text">
                          Rust 环境任务
                        </div>
                        <div class="mt-1 text-xs text-white/70">
                          {{ rustEnv.task.message || '正在处理 Rust 环境任务' }}
                        </div>
                        <div
                          v-if="rustEnv.task.currentItem"
                          class="mt-1 text-[11px] text-white/45 break-all"
                        >
                          当前项：{{ rustEnv.task.currentItem }}
                        </div>
                        <div
                          v-if="rustEnv.task.detail && !formatRustTransferSummary()"
                          class="mt-1 text-[11px] text-white/45 break-all"
                        >
                          {{ rustEnv.task.detail }}
                        </div>
                        <div
                          v-if="formatRustTransferSummary()"
                          class="mt-1 text-[11px] text-[rgb(255,196,164)]/85"
                        >
                          {{ formatRustTransferSummary() }}
                        </div>
                        <div
                          v-if="rustEnv.task.currentSource"
                          class="mt-1 text-[11px] text-white/45 break-all"
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
                    <div class="mt-2 text-[11px] text-white/45">
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
                      class="mt-3 text-[11px] text-white/55 leading-6"
                    >
                      自动安装不通时，可参考官方页面手动处理：
                      <a
                        v-for="link in manualRustDownloadLinks"
                        :key="link.href"
                        :href="link.href"
                        target="_blank"
                        rel="noreferrer"
                        class="ml-2 text-[rgb(255,196,164)] hover:text-[rgb(255,220,200)] underline underline-offset-2"
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
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              SDK
                            </div>
                            <div class="break-all">
                              {{ rustEnv.state?.activeRustVersion || '未就绪' }}
                            </div>
                            <div class="mt-1 break-all text-[11px] text-white/45">
                              {{ rustEnv.state?.activeRustRoot || '未检测到可用目录' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              Zig
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
                              {{ rustEnv.state?.cargoZigbuildStatusMessage || '尚未检测' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/70">
                            <div class="text-[11px] uppercase tracking-wide text-white/45">
                              rustup / targets
                            </div>
                            <div class="break-all">
                              {{ rustEnv.state?.activeRustupVersion || '未就绪' }}
                            </div>
                            <div class="mt-1 break-all text-[11px] text-white/45">
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
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-3 text-xs leading-6 text-white/70">
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
                            <div class="mt-1 text-[11px] text-white/45">
                              {{ rustEnv.state?.cargoZigbuildStatusMessage || '当前 Rust 环境已检测' }}
                            </div>
                          </div>
                          <div class="rounded-lg border border-white/10 bg-black/10 px-3 py-3 text-xs leading-6 text-white/70">
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
                            <div class="mt-1 text-[11px] text-white/45">
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

                  <div class="rounded-xl border border-white/10 bg-black/10 p-4 space-y-4">
                    <div class="flex items-center justify-between">
                      <div class="text-sm font-medium text-dracula-text">
                        下载 / 补齐
                      </div>
                      <NButton
                        secondary
                        size="small"
                        :loading="rustEnv.rustReleaseLoading || rustEnv.zigReleaseLoading"
                        @click="handleRetryRustReleases"
                      >
                        刷新版本
                      </NButton>
                    </div>

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
                          class="text-[rgb(255,196,164)] hover:text-[rgb(255,220,200)] underline underline-offset-2"
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
                            :value="rustDownloadVersion"
                            :options="rustEnv.rustReleases.map(release => ({ label: release.version, value: release.version }))"
                            :loading="rustEnv.rustReleaseLoading"
                            :disabled="rustEnv.task?.status === 'running'"
                            placeholder="选择 Rust 版本"
                            @update:value="(v: string | null) => rustDownloadVersion = v || ''"
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
                            :value="zigDownloadVersion"
                            :options="rustEnv.zigReleases.map(release => ({ label: release.version, value: release.version }))"
                            :loading="rustEnv.zigReleaseLoading"
                            :disabled="rustEnv.task?.status === 'running'"
                            placeholder="选择 Zig 版本"
                            @update:value="(v: string | null) => zigDownloadVersion = v || ''"
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
                            :value="rustDownloadDirectory"
                            placeholder="选择 Rust/Zig 工具链集中安装目录"
                            :disabled="rustEnv.task?.status === 'running'"
                            @update:value="(v: string) => rustDownloadDirectory = v"
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
                        class="rounded-lg border border-white/10 bg-black/10 px-3 py-2 text-xs leading-6 text-white/55"
                      >
                        Rust 将安装到：{{ resolvedRustInstallDirectory }}/rust/{{ rustDownloadVersion || '...' }}；Zig 将安装到：{{ resolvedRustInstallDirectory }}/zig/{{ zigDownloadVersion || '...' }}
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
