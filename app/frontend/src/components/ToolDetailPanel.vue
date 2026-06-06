<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { NButton, NIcon, NInput, NSelect, NText, NTag, type SelectOption } from 'naive-ui'
import {
  Play,
  Stop,
  CloudUpload,
  ServerOutline,
  CodeSlash,
  LogoPython,
  GlobeOutline,
  LaptopOutline,
} from '@vicons/ionicons5'
import { ListSSHConnections } from '../../wailsjs/go/main/App'
import type { SSHConnection, ToolManifest } from '@/types/workbench'
import type { ExecutionTarget, ToolTabState } from '@/stores/workspace'
import { getExecutionTheme } from '@/utils/executionTheme'
import gsap from 'gsap'

const props = defineProps<{
  tool: ToolManifest | null
  tab: ToolTabState | undefined
  activeTaskId: string
  isRunning: boolean
  isLaunching: boolean
  isExporting: boolean
  exportTarget: string
  exportTargetOptions: SelectOption[]
  exportButtonLabel: string
  showExportTargetSelector: boolean
}>()

const emit = defineEmits<{
  execute: []
  cancel: []
  export: []
  'update:execution-target': [value: ExecutionTarget]
  'update:python-env': [value: string]
  'update:remote-conn-id': [value: string]
  'update:export-target': [value: string]
}>()

const sshConnections = ref<SSHConnection[]>([])
const switchTrackRef = ref<HTMLElement | null>(null)
const switchThumbRef = ref<HTMLElement | null>(null)
const switchBackgroundRef = ref<HTMLElement | null>(null)
const switchLocalLabelRef = ref<HTMLElement | null>(null)
const switchRemoteLabelRef = ref<HTMLElement | null>(null)
let resizeObserver: ResizeObserver | null = null

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
const detailAccent = computed(() => detailTheme.value.accent)
const detailAccentHover = computed(() => detailTheme.value.accentHover)
const detailAccentText = computed(() => detailTheme.value.accentText)
const detailAccentSoftBg = computed(() => detailTheme.value.accentSoftBg)
const detailAccentSoftBorder = computed(() => detailTheme.value.accentSoftBorder)
const detailAccentSoftStrongBorder = computed(() => detailTheme.value.accentSoftStrongBorder)
const switchTrackHoverClass = computed(() =>
  'hover:border-white/15',
)
const switchTrackStyle = computed(() => ({
  borderColor: 'rgba(15, 23, 42, 0.60)',
  backgroundColor: 'rgba(8, 14, 24, 0.78)',
  boxShadow: '0 1px 2px rgba(0, 0, 0, 0.10), inset 0 3px 8px rgba(0, 0, 0, 0.55), inset 0 1px 0 rgba(255, 255, 255, 0.04)',
}))
const switchBackgroundStyle = computed(() => ({
  backgroundImage: 'linear-gradient(180deg, rgba(255,255,255,0.04) 0%, transparent 55%, rgba(0,0,0,0.05) 100%)',
}))
const switchThumbStyle = computed(() => ({
  borderColor: 'rgba(255, 255, 255, 0.32)',
  background: 'rgba(255, 255, 255, 0.76)',
  backdropFilter: 'blur(6px)',
  WebkitBackdropFilter: 'blur(6px)',
  boxShadow: '0 3px 14px rgba(0, 0, 0, 0.38), 0 0 0 1px rgba(255, 255, 255, 0.04) inset, 0 1px 0 rgba(255, 255, 255, 0.55) inset',
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

async function animateSwitchThumb(immediate = false) {
  await nextTick()
  const background = switchBackgroundRef.value
  const thumb = switchThumbRef.value
  const track = switchTrackRef.value
  const localLabel = switchLocalLabelRef.value
  const remoteLabel = switchRemoteLabelRef.value
  if (!thumb || !background || !track || !localLabel || !remoteLabel) {
    return
  }

  const inset = 1
  const thumbWidth = thumb.offsetWidth
  const trackWidth = track.clientWidth
  const labelWidth = Math.max(trackWidth - thumbWidth - inset * 2, 0)
  // The thumb already has a static `left: 1px`; animation x should be relative to that base.
  const thumbX = isRemote.value ? Math.max(trackWidth - thumbWidth - inset * 2, 0) : 0
  const localVisibleX = inset + thumbWidth
  const remoteVisibleX = inset
  const localHiddenX = trackWidth
  const remoteHiddenX = -labelWidth
  const duration = immediate ? 0 : 0.16
  const ease = 'power2.out'
  const timeline = gsap.timeline({
    defaults: {
      duration,
      ease,
      overwrite: 'auto',
    },
  })

  timeline.to(
    background,
    {
      backgroundColor: detailTheme.value.accent,
    },
    0,
  )
  timeline.to(
    thumb,
    {
      x: thumbX,
    },
    0,
  )
  timeline.to(
    localLabel,
    {
      x: isRemote.value ? localHiddenX : localVisibleX,
      width: labelWidth,
    },
    0,
  )
  timeline.to(
    remoteLabel,
    {
      x: isRemote.value ? remoteVisibleX : remoteHiddenX,
      width: labelWidth,
    },
    0,
  )
  timeline.to(
    [localLabel, remoteLabel],
    {
      color: detailTheme.value.accentText,
    },
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
  if (switchThumbRef.value) resizeObserver.observe(switchThumbRef.value)
  if (switchLocalLabelRef.value) resizeObserver.observe(switchLocalLabelRef.value)
  if (switchRemoteLabelRef.value) resizeObserver.observe(switchRemoteLabelRef.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
})
</script>

<template>
  <div
    v-if="tool && tab"
  >
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <NIcon
            :component="tool.kind === 'python' ? LogoPython : CodeSlash"
            size="13"
            class="tool-detail-accent tool-detail-transition"
          />
          <NText
            depth="3"
            class="text-xs"
          >
            {{ tool.category.join(' > ') }}
          </NText>
          <span class="text-dracula-soft text-xs">·</span>
          <NTag
            size="tiny"
            :bordered="false"
            class="tool-kind-tag tool-detail-transition"
          >
            <template #icon>
              <NIcon
                :component="tool.kind === 'python' ? LogoPython : CodeSlash"
                size="10"
                class="tool-detail-accent tool-detail-transition"
              />
            </template>
            {{ tool.kind === 'python' ? 'py' : 'go' }}
          </NTag>
          <NTag
            v-if="isRemote"
            size="tiny"
            :bordered="false"
            class="tool-kind-tag tool-detail-transition"
          >
            <template #icon>
              <NIcon
                :component="GlobeOutline"
                size="10"
              />
            </template>
            远程执行目标
          </NTag>
        </div>
        <h2 class="m-0 mt-1 text-lg font-semibold text-dracula-text">
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
            ref="switchBackgroundRef"
            class="pointer-events-none absolute inset-0 z-0 rounded-md"
            :style="switchBackgroundStyle"
          />
          <div
            ref="switchLocalLabelRef"
            class="pointer-events-none absolute inset-y-0 left-0 z-[1] flex items-center justify-center gap-x-1 text-[12px] font-semibold tracking-[0.01em]"
          >
            <NIcon
              :component="LaptopOutline"
              size="12"
            />
            <span>本地</span>
          </div>
          <div
            ref="switchRemoteLabelRef"
            class="pointer-events-none absolute inset-y-0 left-0 z-[1] flex items-center justify-center gap-x-1 text-[12px] font-semibold tracking-[0.01em]"
          >
            <NIcon
              :component="GlobeOutline"
              size="12"
            />
            <span>远程</span>
          </div>
          <div
            ref="switchThumbRef"
            class="pointer-events-none absolute inset-y-[1px] left-[1px] z-[2] flex w-[30px] items-center justify-center rounded-[5px] border"
            :style="switchThumbStyle"
          >
            <span class="text-[12px] font-bold tracking-[0.04em] text-[#243041]">火</span>
          </div>
        </button>

        <NButton
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
          v-if="isRunning"
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
          class="flex items-center rounded-md border border-white/10 bg-black/10 px-1 py-1"
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
            <span class="mx-2 h-5 w-px bg-white/10" />
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
        style="background: linear-gradient(to right, transparent, rgba(255,255,255,0.14), transparent)"
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

.tool-detail-accent {
  color: v-bind(detailAccent);
}

.tool-kind-tag {
  color: v-bind(detailAccent) !important;
  background-color: v-bind(detailAccentSoftBg) !important;
  border: 1px solid v-bind(detailAccentSoftBorder) !important;
}

.tool-detail-action-button {
  background-color: v-bind(detailAccent);
  border-color: v-bind(detailAccentSoftBorder);
  color: v-bind(detailAccentText);
  transition:
    background-color 160ms var(--ease-out-soft),
    border-color 160ms var(--ease-out-soft),
    color 160ms var(--ease-out-soft),
    box-shadow 160ms var(--ease-out-soft);
}

.tool-detail-action-button:hover:not(:disabled) {
  background-color: v-bind(detailAccentHover) !important;
  border-color: v-bind(detailAccentSoftStrongBorder) !important;
}
</style>
