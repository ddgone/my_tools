<script setup lang="ts">
import { computed, nextTick, ref } from 'vue'
import { NButton, NIcon, NText } from 'naive-ui'
import { CheckmarkCircle, TerminalOutline } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { useGoEnvStore } from '@/stores/goenv'
import { useWorkspaceStore } from '@/stores/workspace'

const execution = useExecutionStore()
const goEnv = useGoEnvStore()
const workspace = useWorkspaceStore()

const activeTaskCount = computed(() => execution.tasks.filter((t) => t.status === 'running').length)
const hasActiveToolTab = computed(() => workspace.activeTabType === 'tool' && workspace.activeTabIndex >= 0)
const terminalToggleLabel = computed(() =>
  workspace.activeToolTerminalVisible ? '隐藏终端' : '显示终端',
)
const goVersionLabel = computed(() => {
  const activeVersion = goEnv.state?.activeVersion?.trim()
  if (!activeVersion) {
    return 'Go 未配置'
  }
  return activeVersion.replace(/^Go(?=\d)/, 'Go ')
})

const pythonVersionLabel = computed(() => 'Python --')
const goReady = computed(() => goEnv.state?.hasUsableBinary === true)
const goTagClass = computed(() =>
  goReady.value
    ? 'border-emerald-400/20 bg-emerald-500/10 text-emerald-300 hover:bg-emerald-500/15'
    : 'border-amber-400/20 bg-amber-500/10 text-amber-300 hover:bg-amber-500/15',
)
const pythonTagClass = 'border-white/10 bg-white/5 text-white/45'
const goTooltip = computed(() => {
  if (!goReady.value) {
    return '未配置 Go 环境，点击前往设置'
  }
  const version = goVersionLabel.value
  const binary = goEnv.state?.activeBinary?.trim()
  return binary ? `${version}\n${binary}` : version
})
const tooltipShow = ref(false)
const tooltipRef = ref<HTMLElement | null>(null)
const tooltipX = ref(0)
const tooltipY = ref(0)

let tooltipShowTimer: number | null = null
let tooltipHideTimer: number | null = null

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

function showGoTooltip(event: MouseEvent) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }
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

function hideGoTooltip() {
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
        class="rounded-md border px-2 py-0.5 text-[10px] transition-colors"
        :class="goTagClass"
        @mouseenter="showGoTooltip"
        @mouseleave="hideGoTooltip"
        @click="openGoSettings"
      >
        {{ goVersionLabel }}
      </button>
      <div
        class="rounded-md border px-2 py-0.5 text-[10px]"
        :class="pythonTagClass"
      >
        {{ pythonVersionLabel }}
      </div>
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
        class="workbench-tooltip pointer-events-none fixed z-[100] -translate-x-1/2 -translate-y-full px-2.5 py-1.5 text-xs whitespace-pre-line break-all max-w-[min(28rem,calc(100vw-24px))]"
        :style="{ left: tooltipX + 'px', top: tooltipY + 'px' }"
      >
        <div class="workbench-tooltip-arrow absolute -bottom-1 left-1/2 h-2 w-2 -translate-x-1/2 rotate-45" />
        {{ goTooltip }}
      </div>
    </Teleport>
  </footer>
</template>
