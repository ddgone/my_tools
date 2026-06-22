<script setup lang="ts">
import { computed, onMounted } from 'vue'
import {
  NAlert,
  NButton,
  NDropdown,
  NIcon,
  NPopconfirm,
  NProgress,
  NSelect,
  useMessage,
} from 'naive-ui'
import { Add } from '@vicons/ionicons5'
import { OpenFileDialog } from '../../../wailsjs/go/main/App'
import { usePythonEnvStore } from '@/stores/pythonenv'

const pythonEnv = usePythonEnvStore()
const message = useMessage()
const noPythonValue = '__no_python__'

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
  if (pythonEnv.state?.dependenciesReady) {
    return '依赖已就绪'
  }
  return '一键安装依赖'
})
const canInstallDependencies = computed(() =>
  pythonEnv.state?.hasUsableBaseBinary === true
  && pythonEnv.state?.dependenciesReady !== true
  && pythonEnv.task?.status !== 'running',
)

async function ensurePythonState() {
  if (!pythonEnv.state && !pythonEnv.loading) {
    await pythonEnv.loadState()
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

onMounted(async () => {
  await ensurePythonState()
})
</script>

<template>
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
                :disabled="!canInstallDependencies"
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
