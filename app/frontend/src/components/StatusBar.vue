<script setup lang="ts">
import { computed, nextTick, ref, type CSSProperties } from 'vue'
import { NButton, NIcon, NText } from 'naive-ui'
import { CheckmarkCircle, TerminalOutline } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { useGoEnvStore } from '@/stores/goenv'
import { useRustEnvStore } from '@/stores/rustenv'
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useWorkspaceStore } from '@/stores/workspace'
import { getToolKindTheme } from '@/utils/executionTheme'

const execution = useExecutionStore()
const goEnv = useGoEnvStore()
const rustEnv = useRustEnvStore()
const pythonEnv = usePythonEnvStore()
const workspace = useWorkspaceStore()

const activeTaskCount = computed(() => execution.tasks.filter((t) => t.status === 'running').length)
const hasActiveToolTab = computed(() => workspace.activeTabType === 'tool' && workspace.activeTabIndex >= 0)
const terminalToggleLabel = computed(() =>
  workspace.activeToolTerminalVisible ? '隐藏终端' : '显示终端',
)
function formatGoDisplayVersion(version?: string) {
  const trimmed = version?.trim() || ''
  if (!trimmed) {
    return 'Go'
  }
  if (/^go\d/i.test(trimmed)) {
    return trimmed.replace(/^go/i, 'Go ').replace(/\s+/, ' ')
  }
  if (/^Go\d/.test(trimmed)) {
    return trimmed.replace(/^Go/, 'Go ')
  }
  return trimmed
}

function formatCargoDisplayVersion(version?: string) {
  const trimmed = version?.trim() || ''
  if (!trimmed) {
    return 'cargo'
  }
  const match = trimmed.match(/cargo(?:-zigbuild)?\s+\d+\.\d+\.\d+/i)
  return match ? match[0].replace(/^cargo/i, 'cargo') : trimmed
}

function formatZigDisplayVersion(version?: string) {
  const normalized = (version?.trim() || '').replace(/^zig\s+/i, '')
  if (!normalized) {
    return 'Zig'
  }
  const match = normalized.match(/\d+\.\d+\.\d+(?:-[a-z]+)?/i)
  return match?.[0] || normalized
}

const goVersionLabel = computed(() => {
  if (goEnv.loading && !goEnv.state) {
    return 'Go 检测中 · 正在读取环境'
  }
  const activeVersion = goEnv.state?.activeVersion?.trim()
  if (!activeVersion) {
    return 'Go 未配置 · 仅远程/导出受影响'
  }
  return `Go 已就绪 · ${formatGoDisplayVersion(activeVersion)}`
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
const rustVersionLabel = computed(() => {
  if (rustEnv.loading && !rustEnv.state) {
    return 'Rust 检测中 · 正在读取环境'
  }
  const task = rustEnv.task
  if (task?.status === 'running' && task.kind !== 'install-zig') {
    return 'Rust 正在处理环境 · 点击查看'
  }
  const state = rustEnv.state
  if (state?.config.disabled) {
    return 'Rust 已关闭 · 远程/导出停用'
  }
  if (!state?.hasUsableRust) {
    return 'Rust 未配置 · 仅导出/远程受影响'
  }
  const cargo = formatCargoDisplayVersion(state.activeRustVersion || state.activeCargoVersion)
  return `Rust 已就绪 · ${cargo}`
})
const zigVersionLabel = computed(() => {
  if (rustEnv.loading && !rustEnv.state) {
    return 'Zig 检测中 · 正在读取环境'
  }
  const task = rustEnv.task
  if (task?.status === 'running' && (task.kind === 'install-zig' || task.kind === 'install')) {
    return 'Zig 正在处理环境 · 点击查看'
  }
  const state = rustEnv.state
  if (state?.config.disabled) {
    return 'Zig 环境已关闭 · Rust 交叉编译停用'
  }
  if (!state?.hasUsableZig) {
    return 'Zig 环境未配置 · Rust 交叉编译受影响'
  }
  return `Zig 环境已就绪 · ${formatZigDisplayVersion(state.activeZigVersion)}`
})
const goReady = computed(() => goEnv.state?.hasUsableBinary === true)
const rustReady = computed(() => rustEnv.state?.hasUsableRust === true)
const zigReady = computed(() => rustEnv.state?.hasUsableZig === true)
const pythonReady = computed(() =>
  pythonEnv.state?.hasUsableBinary === true
  && pythonEnv.state?.pipAvailable === true
  && pythonEnv.state?.dependenciesReady === true,
)
const goReadyTheme = getToolKindTheme('go')
const rustReadyTheme = getToolKindTheme('rust')
const zigReadyTheme = getToolKindTheme('zig')
const pythonReadyTheme = getToolKindTheme('python')
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
const rustTagClass = computed(() =>
  rustReady.value
    ? 'statusbar-env-tag--ready'
    : 'border-amber-400/20 bg-amber-500/10 text-amber-300 hover:bg-amber-500/15',
)
const zigTagClass = computed(() =>
  zigReady.value
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
const rustTagStyle = computed<CSSProperties>(() =>
  rustReady.value
    ? {
      '--statusbar-env-bg': rustReadyTheme.accentSoftBg,
      '--statusbar-env-bg-hover': rustReadyTheme.accentSoftStrongBg,
      '--statusbar-env-border': rustReadyTheme.accentSoftBorder,
      '--statusbar-env-text': rustReadyTheme.accent,
    } as CSSProperties
    : {},
)
const zigTagStyle = computed<CSSProperties>(() =>
  zigReady.value
    ? {
      '--statusbar-env-bg': zigReadyTheme.accentSoftBg,
      '--statusbar-env-bg-hover': zigReadyTheme.accentSoftStrongBg,
      '--statusbar-env-border': zigReadyTheme.accentSoftBorder,
      '--statusbar-env-text': zigReadyTheme.accent,
    } as CSSProperties
    : {},
)
function buildTooltipLines(summary: string, details: Array<string | undefined | null>) {
  return [summary, ...details.filter((item): item is string => Boolean(item && item.trim()))].join('\n')
}

const goTooltip = computed(() => {
  if (goEnv.loading && !goEnv.state) {
    return buildTooltipLines('正在检测 Go 环境。', [
      '会扫描已配置路径、PATH、系统安装目录和托管 SDK。',
      '请稍候。',
    ])
  }
  if (!goReady.value) {
    return buildTooltipLines('Go 环境未就绪。', [
      '本地运行、导出和构建缓存都需要 Go 环境。',
      '点击前往设置。',
    ])
  }
  const version = goEnv.state?.activeVersion?.trim()?.replace(/^Go(?=\d)/, 'Go ') || 'Go'
  const binary = goEnv.state?.activeBinary?.trim()
  const source = goEnv.state?.activeSource?.trim()
  return buildTooltipLines('Go 环境已就绪。', [
    `版本：${version}`,
    '本地运行、导出和构建缓存已就绪。',
    source ? `来源：${formatGoSource(source)}` : '',
    binary ? `路径：${binary}` : '',
  ])
})
const pythonTooltip = computed(() => {
  const state = pythonEnv.state
  const task = pythonEnv.task
  if (pythonEnv.loading && !state) {
    return buildTooltipLines('正在检测 Python 环境。', [
      '会检查基础解释器、托管工具环境和依赖状态。',
      '请稍候。',
    ])
  }
  if (task?.status === 'running') {
    const stepLabel = task.totalSteps > 0 ? `\n步骤 ${task.step}/${task.totalSteps}` : ''
    const currentItem = task.currentItem ? `\n当前项：${task.currentItem}` : ''
    return `${task.message || '正在处理 Python 环境任务'}${stepLabel}${currentItem}\n点击前往设置。`
  }
  if (!state?.hasUsableBaseBinary) {
    return buildTooltipLines('Python 环境未就绪。', [
      '请选择一个本地 Python 3，程序会自动创建托管工具环境。',
      '点击前往设置。',
    ])
  }
  if (!state.hasUsableBinary) {
    const baseVersion = state.activeBaseVersion?.trim() || 'Python'
    const baseBinary = state.activeBaseBinary?.trim()
    return buildTooltipLines('Python 环境待补齐。', [
      `基础解释器：${baseVersion}`,
      '托管工具环境尚未创建或需要重建。',
      '点击前往设置。',
      baseBinary ? `路径：${baseBinary}` : '',
    ])
  }
  const version = state.activeVersion?.trim() || 'Python'
  const binary = state.activeBinary?.trim()
  if (!state.pipAvailable) {
    return buildTooltipLines('Python 环境待补齐。', [
      `工具环境：${version}`,
      '未检测到 pip，建议重建工具环境。',
      binary ? `路径：${binary}` : '',
    ])
  }
  if (!state.dependenciesReady) {
    const missing = state.missingPackages.length > 0 ? state.missingPackages.join('、') : '存在未安装依赖'
    return buildTooltipLines('Python 环境待补齐。', [
      `工具环境：${version}`,
      `缺少依赖：${missing}`,
      '点击前往设置并一键安装。',
      binary ? `路径：${binary}` : '',
    ])
  }
  return buildTooltipLines('Python 环境已就绪。', [
    `基础解释器：${state.activeBaseVersion || 'Python'}`,
    `工具环境：${version}`,
    'pip 与动态扫描依赖已就绪。',
    binary ? `路径：${binary}` : '',
  ])
})
const rustTooltip = computed(() => {
  const state = rustEnv.state
  const task = rustEnv.task
  if (rustEnv.loading && !state) {
    return buildTooltipLines('正在检测 Rust 环境。', [
      '会检查 Rust SDK、Zig、cargo-zigbuild 与 targets。',
      '请稍候。',
    ])
  }
  if (task?.status === 'running' && task.kind !== 'install-zig') {
    const stepLabel = task.totalSteps > 0 ? `\n步骤 ${task.step}/${task.totalSteps}` : ''
    const currentItem = task.currentItem ? `\n当前项：${task.currentItem}` : ''
    return `${task.message || '正在处理 Rust 安装任务'}${stepLabel}${currentItem}\n点击前往设置。`
  }
  if (state?.config.disabled) {
    return buildTooltipLines('Rust 环境已关闭。', [
      '自动探测和交叉编译工具链都已停用。',
      '点击前往设置。',
    ])
  }
  if (!state?.hasUsableRust) {
    return buildTooltipLines('Rust 环境未就绪。', [
      '请自动探测、手动选择 Rust 环境目录，或下载托管 Rust。',
      '点击前往设置。',
    ])
  }
  if (!state?.hasUsableEnvironment) {
    return buildTooltipLines('Rust 环境待补齐。', [
      state.activeRustVersion ? `Rust：${state.activeRustVersion}` : '',
      state.activeZigVersion ? `Zig：${state.activeZigVersion}` : 'Zig：未就绪',
      state.cargoZigbuildStatusMessage || '',
      state.targetStatusMessage || '',
      state.activeRustRoot ? `目录：${state.activeRustRoot}` : '',
    ])
  }
  return buildTooltipLines('Rust 环境已就绪。', [
    state.activeRustVersion ? `Rust：${state.activeRustVersion}` : '',
    state.activeRustupVersion ? `rustup：${state.activeRustupVersion}` : '',
    state.activeCargoZigbuildVersion ? `cargo-zigbuild：${state.activeCargoZigbuildVersion}` : '',
    state.activeRustRoot ? `目录：${state.activeRustRoot}` : '',
  ])
})
const zigTooltip = computed(() => {
  const state = rustEnv.state
  const task = rustEnv.task
  if (rustEnv.loading && !state) {
    return buildTooltipLines('正在检测 Zig 环境。', [
      '会检查已配置路径、PATH、常见安装目录和托管 Zig。',
      '请稍候。',
    ])
  }
  if (task?.status === 'running' && (task.kind === 'install-zig' || task.kind === 'install')) {
    const stepLabel = task.totalSteps > 0 ? `\n步骤 ${task.step}/${task.totalSteps}` : ''
    const currentItem = task.currentItem ? `\n当前项：${task.currentItem}` : ''
    return `${task.message || '正在处理 Zig 安装任务'}${stepLabel}${currentItem}\n点击前往设置。`
  }
  if (state?.config.disabled) {
    return buildTooltipLines('Zig 环境已关闭。', [
      'Zig 的自动探测与交叉编译能力已停用。',
      '点击前往设置。',
    ])
  }
  if (!state?.hasUsableZig) {
    return buildTooltipLines('Zig 环境未就绪。', [
      '请在 Zig 页签中手动选择或下载托管 Zig。',
      '点击前往设置。',
    ])
  }
  return buildTooltipLines('Zig 环境已就绪。', [
    state.activeZigVersion ? `版本：${state.activeZigVersion}` : '',
    state.activeZigBinary ? `路径：${state.activeZigBinary}` : '',
  ])
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

function openRustSettings() {
  workspace.openSettings('rust')
}

function openZigSettings() {
  workspace.openSettings('zig')
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
  <footer class="surface-divider flex h-7 shrink-0 items-center justify-between border-t bg-[rgb(var(--color-bg-panel)/0.72)] px-3 backdrop-blur-sm">
    <div class="flex items-center gap-x-1.5">
      <NIcon
        :component="CheckmarkCircle"
        size="12"
        color="rgb(var(--color-success) / 1)"
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
        class="statusbar-env-button rounded-md border px-2.5 py-1 text-[10px] leading-none transition-colors text-left"
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
        class="statusbar-env-button rounded-md border px-2.5 py-1 text-[10px] leading-none transition-colors text-left"
        :class="rustTagClass"
        :style="rustTagStyle"
        @mouseenter="showTooltip($event, rustTooltip)"
        @mouseleave="hideTooltip"
        @click="openRustSettings"
      >
        {{ rustVersionLabel }}
      </button>
      <button
        type="button"
        class="statusbar-env-button rounded-md border px-2.5 py-1 text-[10px] leading-none transition-colors text-left"
        :class="zigTagClass"
        :style="zigTagStyle"
        @mouseenter="showTooltip($event, zigTooltip)"
        @mouseleave="hideTooltip"
        @click="openZigSettings"
      >
        {{ zigVersionLabel }}
      </button>
      <button
        type="button"
        class="statusbar-env-button rounded-md border px-2.5 py-1 text-[10px] leading-none transition-colors text-left"
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
.statusbar-env-button {
  max-width: 13.5rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.statusbar-env-tag--ready {
  border-color: var(--statusbar-env-border);
  background-color: var(--statusbar-env-bg);
  color: var(--statusbar-env-text);
}

.statusbar-env-tag--ready:hover {
  background-color: var(--statusbar-env-bg-hover);
}
</style>
