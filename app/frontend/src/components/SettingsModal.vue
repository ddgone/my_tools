<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NDropdown,
  NIcon,
  NInput,
  NPopconfirm,
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
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()
const goEnv = useGoEnvStore()
const message = useMessage()
const noSdkValue = '__no_sdk__'

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
}, { immediate: true })

watch(() => workspace.settings.lastSettingsTab, async (tab) => {
  if (tab === 'go' && workspace.showSettings) {
    await ensureGoState()
  }
})

watch(downloadVersion, (value) => {
  if (!showDownloadPanel.value) {
    return
  }
  ensureDownloadDirectory(value)
})

onMounted(async () => {
  await ensureGoState()
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
        class="fixed inset-0 z-50 flex items-start justify-center pt-[8vh] pointer-events-none"
      >
        <div
          class="pointer-events-auto w-full max-w-2xl rounded-xl border border-white/15 bg-dracula-panel shadow-2xl"
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

          <div class="px-5 py-4">
            <NTabs
              type="line"
              animated
              :value="workspace.settings.lastSettingsTab"
              @update:value="(v: string) => workspace.settings.lastSettingsTab = v === 'export' || v === 'go' ? v : 'general'"
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
                      默认 Python 解释器
                    </div>
                    <div class="settings-value">
                      <NInput
                        class="settings-control"
                        :value="workspace.settings.defaultPythonPath"
                        placeholder="python"
                        @update:value="(v: string) => workspace.settings.defaultPythonPath = v"
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
