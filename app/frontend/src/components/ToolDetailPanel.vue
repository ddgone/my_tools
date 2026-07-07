<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NIcon, NInput, NSelect, NText, type SelectOption } from 'naive-ui'
import {
  Play,
  Stop,
  CloudUpload,
  DownloadOutline,
  ServerOutline,
  GlobeOutline,
  Refresh,
} from '@vicons/ionicons5'
import { ListSSHConnections } from '../../wailsjs/go/main/App'
import type { ExecutionTask, SSHConnection, ToolManifest } from '@/types/workbench'
import type { ExecutionTarget, ToolTabState } from '@/stores/workspace'
import { getExecutionTheme } from '@/utils/executionTheme'
import gsap from 'gsap'

const props = defineProps<{
  tool: ToolManifest | null
  tab: ToolTabState | undefined
  activeTaskId: string
  activeTask: ExecutionTask | null
  isRunning: boolean
  isLaunching: boolean
  isExporting: boolean
  isDownloadingResult: boolean
  isHistory: boolean
  exportTarget: string
  exportTargetOptions: SelectOption[]
  exportButtonLabel: string
  showExportTargetSelector: boolean
}>()

const emit = defineEmits<{
  execute: []
  cancel: []
  export: []
  'download-result': []
  're-execute': []
  'update:execution-target': [value: ExecutionTarget]
  'update:python-env': [value: string]
  'update:remote-conn-id': [value: string]
  'update:export-target': [value: string]
}>()

const sshConnections = ref<SSHConnection[]>([])
const switchTrackRef = ref<HTMLElement | null>(null)
const switchThumbRef = ref<HTMLElement | null>(null)
const switchLocalPanelRef = ref<HTMLElement | null>(null)
const switchRemotePanelRef = ref<HTMLElement | null>(null)
let resizeObserver: ResizeObserver | null = null
let switchTimeline: gsap.core.Timeline | null = null

async function loadConnections() {
  sshConnections.value = await ListSSHConnections()
}

watch(
  () => props.tab?.executionTarget,
  (target) => {
    if (target === 'remote') {
      void loadConnections()
    }
  },
  { immediate: true },
)

const isRemote = computed(() => props.tab?.executionTarget === 'remote')
const detailTheme = computed(() => getExecutionTheme(props.tool?.kind, isRemote.value ? 'remote' : 'local'))
const localTheme = computed(() => getExecutionTheme(props.tool?.kind, 'local'))
const remoteTheme = computed(() => getExecutionTheme(props.tool?.kind, 'remote'))
const detailAccent = computed(() => detailTheme.value.accent)
const detailAccentHover = computed(() => detailTheme.value.accentHover)
const detailAccentText = computed(() => detailTheme.value.accentText)
const detailAccentSoftBorder = computed(() => detailTheme.value.accentSoftBorder)
const detailAccentSoftStrongBorder = computed(() => detailTheme.value.accentSoftStrongBorder)
const switchTrackHoverClass = computed(() =>
  'hover:border-[rgb(var(--color-border-strong)/0.92)]',
)
const switchTrackStyle = computed(() => ({
  borderColor: 'rgb(var(--color-border-strong) / 0.82)',
  backgroundColor: 'rgb(var(--color-bg-elevated) / 0.9)',
  boxShadow: '0 1px 2px rgb(var(--color-overlay-rgb) / 0.18), inset 0 4px 10px rgb(var(--color-overlay-rgb) / 0.22), inset 0 1px 0 rgb(var(--color-fg-base) / 0.05)',
}))
const switchLocalPanelStyle = computed(() => ({
  color: localTheme.value.accentText,
  backgroundColor: localTheme.value.accent,
  backgroundImage: 'linear-gradient(180deg, rgb(var(--color-fg-base) / 0.04) 0%, transparent 55%, rgb(var(--color-overlay-rgb) / 0.05) 100%)',
  boxShadow: 'inset 0 1px 0 rgb(var(--color-fg-base) / 0.04)',
}))
const switchRemotePanelStyle = computed(() => ({
  color: remoteTheme.value.accentText,
  backgroundColor: remoteTheme.value.accent,
  backgroundImage: 'linear-gradient(180deg, rgb(var(--color-fg-base) / 0.04) 0%, transparent 55%, rgb(var(--color-overlay-rgb) / 0.05) 100%)',
  boxShadow: 'inset 0 1px 0 rgb(var(--color-fg-base) / 0.04)',
}))
const switchThumbStyle = computed(() => ({
  borderColor: 'rgb(var(--color-border-strong) / 0.72)',
  background: 'rgb(var(--color-bg-panel) / 0.92)',
  backdropFilter: 'blur(6px)',
  WebkitBackdropFilter: 'blur(6px)',
  boxShadow: '0 3px 14px rgb(var(--color-overlay-rgb) / 0.24), 0 0 0 1px rgb(var(--color-fg-base) / 0.04) inset, 0 1px 0 rgb(var(--color-fg-base) / 0.4) inset',
}))
function toggleExecutionTarget() {
  emit('update:execution-target', isRemote.value ? 'local' : 'remote')
}

const pythonEnvValue = computed(() => {
  if (!props.tab) {
    return ''
  }
  return isRemote.value ? props.tab.remoteConfig.pythonEnv : props.tab.localConfig.pythonEnv
})

const remoteConnId = computed(() => props.tab?.remoteConfig.connId ?? '')
function updateRemoteConnId(value: string | null) {
  emit('update:remote-conn-id', value ?? '')
}

const remoteConnectionOptions = computed<SelectOption[]>(() =>
  sshConnections.value.map((conn) => ({
    label: `${conn.name} · ${conn.user}@${conn.host}:${conn.port}`,
    value: conn.id,
  })),
)
const canExport = computed(() => Boolean(props.tool?.export?.strategy))
const canDownloadResult = computed(() =>
  isRemote.value
  && !props.isRunning
  && props.activeTask?.remoteResultStatus === 'available'
  && Boolean(props.activeTask?.remoteResultPath),
)
async function animateSwitchThumb(immediate = false) {
  await nextTick()
  const thumb = switchThumbRef.value
  const track = switchTrackRef.value
  const localPanel = switchLocalPanelRef.value
  const remotePanel = switchRemotePanelRef.value
  if (!thumb || !track || !localPanel || !remotePanel) {
    return
  }

  const inset = 1
  const thumbWidth = thumb.offsetWidth
  const trackWidth = track.clientWidth
  const panelWidth = Math.max(trackWidth - thumbWidth - inset * 2, 0)
  // The thumb already has a static `left: 1px`; animation x should be relative to that base.
  const thumbX = isRemote.value ? panelWidth : 0
  const localPanelState = {
    x: isRemote.value ? trackWidth : inset + thumbWidth,
    width: panelWidth,
  }
  const remotePanelState = {
    x: isRemote.value ? inset : -panelWidth,
    width: panelWidth,
  }

  switchTimeline?.kill()

  if (immediate) {
    gsap.set(thumb, { x: thumbX })
    gsap.set(localPanel, localPanelState)
    gsap.set(remotePanel, remotePanelState)
    return
  }

  const timeline = gsap.timeline({
    defaults: {
      duration: 0.18,
      ease: 'power2.out',
      overwrite: 'auto',
    },
  })
  switchTimeline = timeline

  timeline.to(
    thumb,
    {
      x: thumbX,
    },
    0,
  )
  timeline.to(
    localPanel,
    localPanelState,
    0,
  )
  timeline.to(
    remotePanel,
    remotePanelState,
    0,
  )
}

watch(
  [
    () => props.tab?.tabId,
    () => props.tab?.executionTarget,
    () => props.tool?.id,
    () => props.tool?.kind,
  ],
  ([nextTabId], [prevTabId]) => {
    void animateSwitchThumb(nextTabId !== prevTabId)
  },
)

onMounted(() => {
  void animateSwitchThumb(true)
  resizeObserver = new ResizeObserver(() => {
    void animateSwitchThumb(true)
  })
  if (switchTrackRef.value) resizeObserver.observe(switchTrackRef.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  switchTimeline?.kill()
})
</script>

<template>
  <div
    v-if="tool && tab"
  >
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <NText
            depth="3"
            class="rounded-md border border-[rgb(var(--color-border-subtle)/0.58)] bg-[rgb(var(--color-bg-elevated)/0.56)] px-2.5 py-1 text-[11px] leading-none text-[rgb(var(--color-fg-muted)/0.9)]"
          >
            {{ tool.category.join(' / ') }}
          </NText>
        </div>
        <h2 class="m-0 mt-1 text-lg font-semibold text-[rgb(var(--color-fg-base)/0.98)]">
          {{ tool.name }}
        </h2>
        <NText
          depth="2"
          class="mt-1 text-sm leading-relaxed"
        >
          {{ tool.docs.summary || tool.description }}
        </NText>
      </div>

      <div class="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1.5">
        <button
          ref="switchTrackRef"
          type="button"
          class="ui-interactive relative inline-flex h-8 w-24 shrink-0 items-center overflow-hidden rounded-md border bg-transparent px-0 text-left shadow-sm"
          :class="switchTrackHoverClass"
          :style="switchTrackStyle"
          :aria-label="isRemote ? '切换到本地执行目标' : '切换到远程执行目标'"
          @click="toggleExecutionTarget"
        >
          <div
            class="pointer-events-none absolute inset-[1px] z-0 overflow-hidden rounded-[5px]"
          >
            <div
              ref="switchLocalPanelRef"
              class="absolute inset-y-0 left-0 flex items-center justify-center rounded-r-[5px] text-[12px] font-semibold tracking-[0.01em] will-change-transform"
              :style="switchLocalPanelStyle"
            >
              <span>本地</span>
            </div>
            <div
              ref="switchRemotePanelRef"
              class="absolute inset-y-0 left-0 flex items-center justify-center rounded-l-[5px] text-[12px] font-semibold tracking-[0.01em] will-change-transform"
              :style="switchRemotePanelStyle"
            >
              <span>远程</span>
            </div>
          </div>
          <div
            ref="switchThumbRef"
            class="pointer-events-none absolute inset-y-[1px] left-[1px] z-[2] flex w-[30px] items-center justify-center rounded-[5px] border"
            :style="switchThumbStyle"
          >
            <span class="text-[12px] font-bold tracking-[0.04em] text-[rgb(var(--color-fg-secondary)/1)]">火</span>
          </div>
        </button>

        <NButton
          v-if="!isHistory"
          v-press
          size="small"
          :disabled="isRunning || isLaunching"
          :loading="isLaunching"
          class="tool-detail-action-button border shadow-sm"
          @click="emit('execute')"
        >
          <template #icon>
            <NIcon :component="isRemote ? GlobeOutline : Play" />
          </template>
          {{ isRemote ? '远程执行' : '本地运行' }}
        </NButton>

        <NButton
          v-if="isHistory"
          v-press
          size="small"
          class="tool-detail-action-button border shadow-sm"
          @click="emit('re-execute')"
        >
          <template #icon>
            <NIcon :component="Refresh" />
          </template>
          重新执行
        </NButton>

        <NButton
          v-if="isRunning && !isHistory"
          v-press
          type="error"
          size="small"
          @click="emit('cancel')"
        >
          <template #icon>
            <NIcon :component="Stop" />
          </template>
          停止
        </NButton>

        <div
          v-if="!isHistory"
          class="flex items-center rounded-md border border-[rgb(var(--color-border-subtle)/0.75)] bg-[rgb(var(--color-bg-elevated)/0.88)] px-1 py-1"
        >
          <NButton
            v-press
            size="small"
            secondary
            :disabled="!canExport || isLaunching || isExporting"
            :loading="isExporting"
            @click="emit('export')"
          >
            <template #icon>
              <NIcon :component="CloudUpload" />
            </template>
            {{ exportButtonLabel }}
          </NButton>
          <template v-if="showExportTargetSelector">
            <span class="mx-2 h-5 w-px bg-[rgb(var(--color-border-subtle)/0.85)]" />
            <NText
              depth="3"
              class="shrink-0 text-[11px]"
            >
              导出到
            </NText>
            <NSelect
              :value="exportTarget"
              :options="exportTargetOptions"
              size="small"
              class="ml-2 w-[208px]"
              :disabled="isExporting"
              @update:value="(value) => emit('update:export-target', String(value ?? ''))"
            />
          </template>
        </div>
      </div>
    </div>

    <div
      v-if="isRemote"
      class="mt-3"
    >
      <div
        class="h-px"
        style="background: linear-gradient(to right, transparent, rgb(var(--color-border-subtle) / 0.9), transparent)"
      />
      <div
        class="mt-3 flex items-center gap-x-2"
      >
        <NIcon
          :component="ServerOutline"
          size="15"
          class="tool-detail-accent tool-detail-transition"
        />
        <NText
          depth="3"
          class="tool-detail-accent tool-detail-transition shrink-0 text-[11px] uppercase tracking-wide"
        >
          远程环境选择
        </NText>
        <NSelect
          :value="remoteConnId"
          placeholder="选择 SSH 连接"
          :options="remoteConnectionOptions"
          clearable
          filterable
          class="max-w-[420px] flex-1"
          @update:value="updateRemoteConnId"
        />
        <NButton
          v-press
          size="small"
          :secondary="!canDownloadResult"
          :class="canDownloadResult ? 'tool-detail-action-button border shadow-sm' : ''"
          :disabled="!canDownloadResult || isDownloadingResult"
          :loading="isDownloadingResult"
          @click="emit('download-result')"
        >
          <template #icon>
            <NIcon :component="DownloadOutline" />
          </template>
          下载输出结果
        </NButton>
      </div>
    </div>

    <div
      v-if="tool.kind === 'python' && isRemote"
      class="mt-4 flex items-center gap-x-2"
    >
      <NText
        depth="3"
        class="shrink-0 text-[11px] uppercase tracking-wide"
      >
        远程 Python 命令
      </NText>
      <NInput
        :value="pythonEnvValue"
        placeholder="python3"
        size="small"
        class="w-36"
        @update:value="emit('update:python-env', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.tool-detail-transition {
  transition:
    color 160ms var(--ease-out-soft),
    background-color 160ms var(--ease-out-soft),
    border-color 160ms var(--ease-out-soft),
    box-shadow 160ms var(--ease-out-soft),
    fill 160ms var(--ease-out-soft),
    stroke 160ms var(--ease-out-soft);
}

.tool-detail-action-button {
  --n-color: v-bind(detailAccent) !important;
  --n-color-hover: v-bind(detailAccentHover) !important;
  --n-color-pressed: v-bind(detailAccentHover) !important;
  --n-color-focus: v-bind(detailAccentHover) !important;
  --n-border: 1px solid v-bind(detailAccentSoftBorder) !important;
  --n-border-hover: 1px solid v-bind(detailAccentSoftStrongBorder) !important;
  --n-border-pressed: 1px solid v-bind(detailAccentSoftStrongBorder) !important;
  --n-border-focus: 1px solid v-bind(detailAccentSoftStrongBorder) !important;
  --n-text-color: v-bind(detailAccentText) !important;
  --n-text-color-hover: v-bind(detailAccentText) !important;
  --n-text-color-pressed: v-bind(detailAccentText) !important;
  --n-text-color-focus: v-bind(detailAccentText) !important;
  --n-ripple-color: transparent !important;
  transition:
    background-color 160ms var(--ease-out-soft),
    border-color 160ms var(--ease-out-soft),
    color 160ms var(--ease-out-soft),
    box-shadow 160ms var(--ease-out-soft);
}

.tool-detail-action-button :deep(.n-button__state-overlay) {
  background-color: transparent !important;
}

.tool-detail-action-button :deep(.n-button__state-border) {
  display: none !important;
}

</style>
