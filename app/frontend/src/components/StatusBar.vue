<script setup lang="ts">
import { computed, nextTick, ref, type CSSProperties } from 'vue'
import { NButton, NIcon, NText } from 'naive-ui'
import { CheckmarkCircle, TerminalOutline } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { useGoEnvStore } from '@/stores/goenv'
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useWorkspaceStore } from '@/stores/workspace'
import { getExecutionTheme } from '@/utils/executionTheme'

const execution = useExecutionStore()
const goEnv = useGoEnvStore()
const pythonEnv = usePythonEnvStore()
const workspace = useWorkspaceStore()

const activeTaskCount = computed(() => execution.tasks.filter((t) => t.status === 'running').length)
const hasActiveToolTab = computed(() => workspace.activeTabType === 'tool' && workspace.activeTabIndex >= 0)
const terminalToggleLabel = computed(() =>
  workspace.activeToolTerminalVisible ? '隐藏终端' : '显示终端',
)
const goVersionLabel = computed(() => {
  if (goEnv.loading && !goEnv.state) {
    return 'Go 检测中 · 正在读取环境'
  }
  const activeVersion = goEnv.state?.activeVersion?.trim()
  if (!activeVersion) {
    return 'Go 未配置 · 仅远程/导出受影响'
  }
  return `Go 已就绪 · ${activeVersion.replace(/^Go(?=\d)/, 'Go ')}`
})

const pythonVersionLabel = computed(() => {
  const state = pythonEnv.state
  const task = pythonEnv.task
  if (pythonEnv.loading && !state) {
    return 'Python 检测中 · 正在读取环境'
  }
  if (task?.status === 'running') {
    return task.kind === 'install'
      ? 'Python 正在安装依赖 · 点击查看'
      : 'Python 正在创建环境 · 点击查看'
  }
  if (!state?.hasUsableBaseBinary) {
    return 'Python 未配置 · 本地运行受影响'
  }
  if (!state.hasUsableBinary) {
    return 'Python 工具环境未创建 · 点击处理'
  }
  if (!state.pipAvailable) {
    return 'Python 工具环境缺少 pip · 点击处理'
  }
  if (!state.dependenciesReady) {
    return 'Python 依赖未就绪 · 点击安装'
  }
  const activeVersion = state.activeVersion?.trim()
  return `Python 已就绪 · ${activeVersion || 'Python'}`
})
const goReady = computed(() => goEnv.state?.hasUsableBinary === true)
const pythonReady = computed(() =>
  pythonEnv.state?.hasUsableBinary === true
  && pythonEnv.state?.pipAvailable === true
  && pythonEnv.state?.dependenciesReady === true,
)
const goReadyTheme = getExecutionTheme('go', 'local')
const pythonReadyTheme = getExecutionTheme('python', 'local')
const goTagClass = computed(() =>
  goReady.value
    ? 'statusbar-env-tag--ready'
    : 'border-amber-400/20 bg-amber-500/10 text-amber-300 hover:bg-amber-500/15',
)
const pythonTagClass = computed(() =>
  pythonReady.value
    ? 'statusbar-env-tag--ready'
    : 'border-amber-400/20 bg-amber-500/10 text-amber-300 hover:bg-amber-500/15',
)
const goTagStyle = computed<CSSProperties>(() =>
  goReady.value
    ? {
      '--statusbar-env-bg': goReadyTheme.accentSoftBg,
      '--statusbar-env-bg-hover': goReadyTheme.accentSoftStrongBg,
      '--statusbar-env-border': goReadyTheme.accentSoftBorder,
      '--statusbar-env-text': goReadyTheme.accent,
    } as CSSProperties
    : {},
)
const pythonTagStyle = computed<CSSProperties>(() =>
  pythonReady.value
    ? {
      '--statusbar-env-bg': pythonReadyTheme.accentSoftBg,
      '--statusbar-env-bg-hover': pythonReadyTheme.accentSoftStrongBg,
      '--statusbar-env-border': pythonReadyTheme.accentSoftBorder,
      '--statusbar-env-text': pythonReadyTheme.accent,
    } as CSSProperties
    : {},
)
const goTooltip = computed(() => {
  if (goEnv.loading && !goEnv.state) {
    return '正在检测 Go 环境。\n会扫描已配置路径、PATH、系统安装目录和托管 SDK。\n请稍候。'
  }
  if (!goReady.value) {
    return '未配置 Go 环境。\n本地运行不受影响；远程执行、导出和构建缓存需要 Go 环境。\n点击前往设置。'
  }
  const version = goEnv.state?.activeVersion?.trim()?.replace(/^Go(?=\d)/, 'Go ') || 'Go'
  const binary = goEnv.state?.activeBinary?.trim()
  const source = goEnv.state?.activeSource?.trim()
  const sourceLabel = source ? `\n来源：${formatGoSource(source)}` : ''
  return binary
    ? `当前使用 ${version}。\n本地运行不受影响；远程执行、导出和构建缓存已就绪。${sourceLabel}\n${binary}`
    : `当前使用 ${version}。`
})
const pythonTooltip = computed(() => {
  const state = pythonEnv.state
  const task = pythonEnv.task
  if (pythonEnv.loading && !state) {
    return '正在检测 Python 环境。\n会检查基础解释器、托管工具环境和依赖状态。\n请稍候。'
  }
  if (task?.status === 'running') {
    const stepLabel = task.totalSteps > 0 ? `\n步骤 ${task.step}/${task.totalSteps}` : ''
    const currentItem = task.currentItem ? `\n当前项：${task.currentItem}` : ''
    return `${task.message || '正在处理 Python 环境任务'}${stepLabel}${currentItem}\n点击前往设置。`
  }
  if (!state?.hasUsableBaseBinary) {
    return '未配置基础 Python。\n请选择一个本地 Python 3，程序会自动创建托管虚拟环境。\n点击前往设置。'
  }
  if (!state.hasUsableBinary) {
    const baseVersion = state.activeBaseVersion?.trim() || 'Python'
    const baseBinary = state.activeBaseBinary?.trim()
    return baseBinary
      ? `当前基础解释器是 ${baseVersion}。\n托管工具环境尚未创建或需要重建。\n点击前往设置。\n${baseBinary}`
      : `当前基础解释器是 ${baseVersion}。\n托管工具环境尚未创建或需要重建。`
  }
  const version = state.activeVersion?.trim() || 'Python'
  const binary = state.activeBinary?.trim()
  if (!state.pipAvailable) {
    return binary
      ? `当前工具环境使用 ${version}。\n未检测到 pip，建议重建工具环境。\n${binary}`
      : `当前工具环境使用 ${version}。\n未检测到 pip，建议重建工具环境。`
  }
  if (!state.dependenciesReady) {
    const missing = state.missingPackages.length > 0 ? state.missingPackages.join('、') : '存在未安装依赖'
    return binary
      ? `当前工具环境使用 ${version}。\n还有依赖未安装：${missing}。\n点击前往设置并一键安装。\n${binary}`
      : `当前工具环境使用 ${version}。\n还有依赖未安装：${missing}。`
  }
  return binary
    ? `当前基础解释器：${state.activeBaseVersion || 'Python'}。\n当前工具环境使用 ${version}。\npip 与动态扫描依赖已就绪。\n${binary}`
    : `当前工具环境使用 ${version}。`
})
const tooltipShow = ref(false)
const tooltipText = ref('')
const tooltipRef = ref<HTMLElement | null>(null)
const tooltipX = ref(0)
const tooltipY = ref(0)

let tooltipShowTimer: number | null = null
let tooltipHideTimer: number | null = null

function formatGoSource(source: string) {
  const labels: Record<string, string> = {
    configured: '自定义路径',
    remembered: '历史路径',
    path: 'PATH 中的 Go',
    detected: '系统安装目录',
    managed: '托管 SDK',
  }
  return labels[source] || '自动检测'
}

function clampTooltipX(anchorX: number) {
  const tooltipWidth = tooltipRef.value?.offsetWidth ?? 0
  const viewportWidth = window.innerWidth
  const margin = 12

  if (tooltipWidth <= 0) {
    return anchorX
  }

  const minX = margin + tooltipWidth / 2
  const maxX = viewportWidth - margin - tooltipWidth / 2

  if (minX > maxX) {
    return viewportWidth / 2
  }

  return Math.min(Math.max(anchorX, minX), maxX)
}

function toggleTerminal() {
  if (!hasActiveToolTab.value) {
    return
  }
  workspace.toggleTerminalVisible(workspace.activeTabIndex)
}

function openGoSettings() {
  workspace.openSettings('go')
}

function openPythonSettings() {
  workspace.openSettings('python')
}

function showTooltip(event: MouseEvent, text: string) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }
  tooltipText.value = text
  if (tooltipHideTimer) {
    window.clearTimeout(tooltipHideTimer)
    tooltipHideTimer = null
  }
  if (tooltipShowTimer) {
    window.clearTimeout(tooltipShowTimer)
  }
  const rect = target.getBoundingClientRect()
  const anchorX = rect.left + rect.width / 2
  tooltipX.value = anchorX
  tooltipY.value = rect.top - 8
  tooltipShowTimer = window.setTimeout(() => {
    tooltipShow.value = true
    void nextTick(() => {
      tooltipX.value = clampTooltipX(anchorX)
    })
  }, 180)
}

function hideTooltip() {
  if (tooltipShowTimer) {
    window.clearTimeout(tooltipShowTimer)
    tooltipShowTimer = null
  }
  tooltipHideTimer = window.setTimeout(() => {
    tooltipShow.value = false
  }, 80)
}
</script>

<template>
  <footer class="flex h-7 shrink-0 items-center justify-between border-t border-white/15 bg-dracula-panel/50 px-3 backdrop-blur-sm">
    <div class="flex items-center gap-x-1.5">
      <NIcon
        :component="CheckmarkCircle"
        size="12"
        color="#50fa7b"
      />
      <NText
        depth="3"
        class="text-[11px]"
      >
        就绪
      </NText>
    </div>
    <div class="flex items-center gap-x-1">
      <NText
        depth="3"
        class="text-[11px]"
      >
        <template v-if="activeTaskCount > 0">
          {{ activeTaskCount }} 个任务运行中
        </template>
        <template v-else>
          无活跃任务
        </template>
      </NText>
    </div>
    <div class="flex items-center gap-x-2">
      <NButton
        quaternary
        size="tiny"
        :disabled="!hasActiveToolTab"
        @click="toggleTerminal"
      >
        <template #icon>
          <NIcon :component="TerminalOutline" />
        </template>
        {{ terminalToggleLabel }}
      </NButton>
      <button
        type="button"
        class="rounded-md border px-2.5 py-1 text-[10px] leading-none transition-colors min-w-[15.5rem] text-left"
        :class="goTagClass"
        :style="goTagStyle"
        @mouseenter="showTooltip($event, goTooltip)"
        @mouseleave="hideTooltip"
        @click="openGoSettings"
      >
        {{ goVersionLabel }}
      </button>
      <button
        type="button"
        class="rounded-md border px-2.5 py-1 text-[10px] leading-none transition-colors min-w-[17rem] text-left"
        :class="pythonTagClass"
        :style="pythonTagStyle"
        @mouseenter="showTooltip($event, pythonTooltip)"
        @mouseleave="hideTooltip"
        @click="openPythonSettings"
      >
        {{ pythonVersionLabel }}
      </button>
      <NText
        depth="3"
        class="text-[10px]"
      >
        v1.0.0
      </NText>
    </div>
    <Teleport to="body">
      <div
        v-if="tooltipShow"
        ref="tooltipRef"
        class="workbench-tooltip pointer-events-none fixed z-[100] -translate-x-1/2 -translate-y-full px-2.5 py-1.5 text-xs whitespace-pre-line break-all w-[min(22rem,calc(100vw-24px))]"
        :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }"
      >
        <div class="workbench-tooltip-arrow absolute -bottom-1 left-1/2 h-2 w-2 -translate-x-1/2 rotate-45" />
        {{ tooltipText }}
      </div>
    </Teleport>
  </footer>
</template>

<style scoped>
.statusbar-env-tag--ready {
  border-color: var(--statusbar-env-border);
  background-color: var(--statusbar-env-bg);
  color: var(--statusbar-env-text);
}

.statusbar-env-tag--ready:hover {
  background-color: var(--statusbar-env-bg-hover);
}
</style>
