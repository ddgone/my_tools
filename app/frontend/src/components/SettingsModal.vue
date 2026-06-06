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
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()
const goEnv = useGoEnvStore()
const pythonEnv = usePythonEnvStore()
const message = useMessage()
const noSdkValue = '__no_sdk__'
const noPythonValue = '__no_python__'

const showDownloadPanel = ref(false)
const downloadVersion = ref('')
const downloadDirectory = ref('')

const goCandidateOptions = computed(() =>
  [
    { label: '<无 SDK>', value: noSdkValue },
    ...(goEnv.state?.candidates ?? [])
      .filter(candidate => candidate.valid)
      .map(candidate => ({
        label: candidate.detail ? `${candidate.label}  ${candidate.detail}` : candidate.label,
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

async function ensureGoReleases() {
  await goEnv.ensureReleases()
  if (!downloadVersion.value && goEnv.releases.length > 0) {
    downloadVersion.value = goEnv.releases[0].version
  }
}

function ensureDownloadDirectory(version?: string) {
  const targetVersion = version || downloadVersion.value
  if (!targetVersion) {
    return
  }
  const lastInstallDirectory = goEnv.state?.config.lastInstallDirectory?.trim()
  if (lastInstallDirectory) {
    downloadDirectory.value = lastInstallDirectory
    return
  }
  const suggested = goEnv.state?.suggestedInstallDirectory?.trim()
  if (suggested) {
    const normalizedVersion = targetVersion.toLowerCase()
    const normalizedBase = suggested.replace(/[\\/]+$/, '')
    downloadDirectory.value = normalizedBase.endsWith(normalizedVersion)
      ? normalizedBase
      : `${normalizedBase}/${normalizedVersion}`
  }
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
    showDownloadPanel.value = false
    message.success('Go SDK 安装完成，已自动设为当前环境')
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    message.error(detail || '安装 Go SDK 失败')
  }
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
  if (visible && workspace.settings.lastSettingsTab === 'python') {
    await ensurePythonState()
  }
}, { immediate: true })

watch(() => workspace.settings.lastSettingsTab, async (tab) => {
  if (tab === 'go' && workspace.showSettings) {
    await ensureGoState()
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
  await Promise.all([ensureGoState(), ensurePythonState()])
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
              @update:value="(v: string) => workspace.settings.lastSettingsTab = v === 'export' || v === 'go' || v === 'python' ? v : 'general'"
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
                      {{ goEnv.releaseError }}
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
                            placeholder="选择 Go 版本"
                            @update:value="(v: string | null) => downloadVersion = v || ''"
                          />
                        </div>
                      </div>

                      <div class="settings-row align-start">
                        <div class="settings-label">
                          位置
                        </div>
                        <div class="settings-value gap-2">
                          <NInput
                            class="settings-control"
                            :value="downloadDirectory"
                            placeholder="选择 Go SDK 安装目录"
                            @update:value="(v: string) => downloadDirectory = v"
                          />
                          <NButton
                            secondary
                            @click="handleBrowseInstallDirectory"
                          >
                            <template #icon>
                              <NIcon :component="FolderOpenOutline" />
                            </template>
                          </NButton>
                        </div>
                      </div>
                    </div>

                    <div class="flex justify-end">
                      <NButton
                        type="primary"
                        :loading="goEnv.installing"
                        @click="handleStartDownload"
                      >
                        安装
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
