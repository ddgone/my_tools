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
import gsap from 'gsap'

const props = defineProps<{
  tool: ToolManifest | null
  tab: ToolTabState | undefined
  activeTaskId: string
  isRunning: boolean
  isLaunching: boolean
}>()

const emit = defineEmits<{
  execute: []
  cancel: []
  'update:execution-target': [value: ExecutionTarget]
  'update:python-env': [value: string]
  'update:remote-conn-id': [value: string]
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
const isPythonTool = computed(() => props.tool?.kind === 'python')
const isPythonLocal = computed(() => !isRemote.value && props.tool?.kind === 'python')

const executeButtonClass = computed(() =>
  isRemote.value
    ? 'border-dracula-pink/20 bg-dracula-pink text-[#2f1026] hover:bg-[#ff94d2]'
    : isPythonLocal.value
      ? 'border-dracula-green/20 bg-dracula-green text-[#082512] hover:bg-[#7dff9a]'
      : 'border-dracula-cyan/20 bg-dracula-cyan text-[#102433] hover:bg-[#a4ffff]',
)
const detailTheme = computed(() => {
  if (isRemote.value) {
    return {
      accent: '#ff79c6',
      accentSoftBackground: 'rgba(255, 121, 198, 0.1)',
      accentSoftBorder: 'rgba(255, 121, 198, 0.2)',
      panelBackground: '#ff79c6',
      panelText: '#2f1026',
    }
  }

  if (isPythonTool.value) {
    return {
      accent: '#50fa7b',
      accentSoftBackground: 'rgba(80, 250, 123, 0.1)',
      accentSoftBorder: 'rgba(80, 250, 123, 0.2)',
      panelBackground: '#50fa7b',
      panelText: '#082512',
    }
  }

  return {
    accent: '#8be9fd',
    accentSoftBackground: 'rgba(139, 233, 253, 0.1)',
    accentSoftBorder: 'rgba(139, 233, 253, 0.2)',
    panelBackground: '#8be9fd',
    panelText: '#102433',
  }
})
const detailThemeStyle = computed(() => ({
  '--tool-detail-accent': detailTheme.value.accent,
  '--tool-detail-accent-soft-bg': detailTheme.value.accentSoftBackground,
  '--tool-detail-accent-soft-border': detailTheme.value.accentSoftBorder,
  '--tool-detail-panel-text': detailTheme.value.panelText,
  '--tool-detail-transition': '160ms var(--ease-out-soft)',
}))
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
      backgroundColor: detailTheme.value.panelBackground,
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
      color: detailTheme.value.panelText,
    },
    0,
  )
}

watch(() => props.tab?.executionTarget, () => {
  void animateSwitchThumb()
})

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
    :style="detailThemeStyle"
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
            class="border border-dracula-pink/15 bg-dracula-pink/10 text-dracula-pink"
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
          class="border shadow-sm"
          :class="executeButtonClass"
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

        <NButton
          v-press
          size="small"
          disabled
          secondary
        >
          <template #icon>
            <NIcon :component="CloudUpload" />
          </template>
          导出
        </NButton>
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
          class="text-dracula-pink"
        />
        <NText
          depth="3"
          class="shrink-0 text-[11px] uppercase tracking-wide text-dracula-pink"
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
      v-if="tool.kind === 'python'"
      class="mt-4 flex items-center gap-x-2"
    >
      <NText
        depth="3"
        class="shrink-0 text-[11px] uppercase tracking-wide"
      >
        {{ isRemote ? '远程 Python 环境' : '本地 Python 环境' }}
      </NText>
      <NInput
        :value="pythonEnvValue"
        placeholder="python"
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
    color var(--tool-detail-transition),
    background-color var(--tool-detail-transition),
    border-color var(--tool-detail-transition),
    box-shadow var(--tool-detail-transition),
    fill var(--tool-detail-transition),
    stroke var(--tool-detail-transition);
}

.tool-detail-accent {
  color: var(--tool-detail-accent);
}

.tool-kind-tag {
  color: var(--tool-detail-accent) !important;
  background-color: var(--tool-detail-accent-soft-bg) !important;
  border: 1px solid var(--tool-detail-accent-soft-border) !important;
}
</style>
