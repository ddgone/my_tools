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
const localButtonRef = ref<HTMLElement | null>(null)
const remoteButtonRef = ref<HTMLElement | null>(null)
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
const isPythonLocal = computed(() => !isRemote.value && props.tool?.kind === 'python')

const accentTextClass = computed(() => (isRemote.value ? 'text-dracula-pink' : isPythonLocal.value ? 'text-dracula-green' : 'text-dracula-cyan'))
const executeButtonClass = computed(() =>
  isRemote.value
    ? 'border-dracula-pink/20 bg-dracula-pink text-[#2f1026] hover:bg-[#ff94d2]'
    : isPythonLocal.value
      ? 'border-dracula-green/20 bg-dracula-green text-[#082512] hover:bg-[#7dff9a]'
      : 'border-dracula-cyan/20 bg-dracula-cyan text-[#102433] hover:bg-[#a4ffff]',
)
const goTagClass = computed(() =>
  isRemote.value
    ? 'text-dracula-pink'
    : 'text-dracula-cyan',
)
const goIconColor = computed(() => (isRemote.value ? '#ff79c6' : '#8be9fd'))
const switchThumbStyle = computed(() => ({
  backgroundColor: isRemote.value ? 'rgba(255, 121, 198, 0.22)' : isPythonLocal.value ? 'rgba(80, 250, 123, 0.18)' : 'rgba(139, 233, 253, 0.16)',
}))
const pythonTagClass = computed(() => (isRemote.value ? 'text-dracula-pink' : 'text-dracula-green'))
const pythonIconColor = computed(() => (isRemote.value ? '#ff79c6' : '#50fa7b'))

const pythonEnvValue = computed(() => {
  if (!props.tab) {
    return ''
  }
  return isRemote.value ? props.tab.remoteConfig.pythonEnv : props.tab.localConfig.pythonEnv
})

const remoteConnId = computed(() => props.tab?.remoteConfig.connId ?? '')

const remoteConnectionOptions = computed<SelectOption[]>(() =>
  sshConnections.value.map((conn) => ({
    label: `${conn.name} · ${conn.user}@${conn.host}:${conn.port}`,
    value: conn.id,
  })),
)

async function animateSwitchThumb(immediate = false) {
  await nextTick()
  const thumb = switchThumbRef.value
  const target = isRemote.value ? remoteButtonRef.value : localButtonRef.value
  const track = switchTrackRef.value
  if (!thumb || !target || !track) {
    return
  }
  const x = target.offsetLeft - track.offsetLeft
  const width = target.offsetWidth
  gsap.to(thumb, {
    x,
    width,
    duration: immediate ? 0 : 0.24,
    ease: 'power2.out',
    overwrite: 'auto',
  })
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
  if (localButtonRef.value) resizeObserver.observe(localButtonRef.value)
  if (remoteButtonRef.value) resizeObserver.observe(remoteButtonRef.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
})
</script>

<template>
  <div v-if="tool && tab">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <NIcon
            :component="tool.kind === 'python' ? LogoPython : CodeSlash"
            size="13"
            :class="accentTextClass"
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
            :class="tool.kind === 'python' ? pythonTagClass : goTagClass"
            :style="tool.kind === 'python'
              ? {
                backgroundColor: isRemote ? 'rgba(255, 121, 198, 0.1)' : 'rgba(80, 250, 123, 0.1)',
                border: isRemote ? '1px solid rgba(255, 121, 198, 0.2)' : '1px solid rgba(80, 250, 123, 0.2)',
              }
              : {
                backgroundColor: isRemote ? 'rgba(255, 121, 198, 0.1)' : 'rgba(139, 233, 253, 0.1)',
                border: isRemote ? '1px solid rgba(255, 121, 198, 0.2)' : '1px solid rgba(139, 233, 253, 0.2)',
              }"
          >
            <template #icon>
              <NIcon
                :component="tool.kind === 'python' ? LogoPython : CodeSlash"
                size="10"
                :color="tool.kind === 'python' ? pythonIconColor : goIconColor"
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
        <div
          ref="switchTrackRef"
          class="relative inline-flex items-center rounded-full border border-white/10 bg-black/20 p-1 shadow-[inset_0_1px_0_rgba(255,255,255,0.04)]"
        >
          <div
            ref="switchThumbRef"
            class="absolute inset-y-1 left-1 rounded-full border border-white/10 shadow-[0_8px_18px_rgba(0,0,0,0.22)]"
            :style="switchThumbStyle"
            style="width: 0"
          />
          <button
            ref="localButtonRef"
            v-press
            type="button"
            class="ui-interactive relative z-[1] flex min-w-[72px] items-center justify-center gap-x-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors"
            :class="tab.executionTarget === 'local' ? 'text-dracula-text' : 'text-slate-400 hover:text-dracula-text'"
            @click="emit('update:execution-target', 'local')"
          >
            <NIcon
              :component="LaptopOutline"
              size="13"
            />
            本地
          </button>
          <button
            ref="remoteButtonRef"
            v-press
            type="button"
            class="ui-interactive relative z-[1] flex min-w-[72px] items-center justify-center gap-x-1.5 rounded-full px-3 py-1.5 text-xs font-medium transition-colors"
            :class="tab.executionTarget === 'remote' ? 'text-dracula-text' : 'text-slate-400 hover:text-dracula-text'"
            @click="emit('update:execution-target', 'remote')"
          >
            <NIcon
              :component="GlobeOutline"
              size="13"
            />
            远程
          </button>
        </div>

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
          @update:value="emit('update:remote-conn-id', ($event as string | null) ?? '')"
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
