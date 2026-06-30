<script setup lang="ts">
import { computed, nextTick, ref, watch, type CSSProperties } from 'vue'
import { NButton, NIcon, NScrollbar, NTag, NText, NTooltip, useMessage } from 'naive-ui'
import { Trash, Copy, Download, GlobeOutline, LaptopOutline } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { OpenSaveFileDialog, SaveTextFile } from '../../wailsjs/go/main/App'
import { getExecutionTheme } from '@/utils/executionTheme'

const props = defineProps<{
  taskId: string
  executionTarget: 'local' | 'remote'
  toolKind?: string
}>()

const execution = useExecutionStore()
const workspace = useWorkspaceStore()
const message = useMessage()
const terminalRef = ref<InstanceType<typeof NScrollbar> | null>(null)
const autoScroll = ref(true)
let autoScrollResetFrame: number | null = null
let isProgrammaticScroll = false
const terminalTheme = computed(() => getExecutionTheme(props.toolKind, props.executionTarget))
const terminalAccent = computed(() => terminalTheme.value.accent)
const followButtonColor = computed(() =>
  autoScroll.value ? terminalTheme.value.accent : 'rgb(var(--color-fg-muted) / 1)',
)
const terminalModeTagStyle = computed<CSSProperties>(() => ({
  color: terminalTheme.value.accent,
  backgroundColor: terminalTheme.value.accentSoftBg,
  border: `1px solid ${terminalTheme.value.accentSoftBorder}`,
}))
const terminalModeLabel = computed(() => (props.executionTarget === 'remote' ? '远程' : '本地'))
const terminalModeIcon = computed(() => (props.executionTarget === 'remote' ? GlobeOutline : LaptopOutline))

const logs = computed(() => execution.logsForTask(props.taskId))

const activeTask = computed(() =>
  props.taskId ? execution.recentTasks.find((t) => t.id === props.taskId) ?? null : null,
)

function statusLabel(status?: string) {
  switch (status) {
    case 'running': return '运行中'
    case 'success': return '已完成'
    case 'error': return '失败'
    case 'canceled': return '已取消'
    default: return '等待中'
  }
}

interface LogSegment {
  text: string
  color?: string
}

const ansiColors: Record<number, string | undefined> = {
  0: undefined,
  31: 'rgb(var(--color-error) / 1)',
  32: 'rgb(var(--color-success) / 1)',
  33: 'rgb(var(--color-warning) / 1)',
  34: 'rgb(var(--color-brand-primary) / 1)',
  35: 'rgb(var(--color-mode-remote) / 1)',
  36: 'rgb(var(--color-mode-local) / 1)',
  37: 'rgb(var(--color-fg-base) / 1)',
}

function parseAnsi(text: string): LogSegment[] {
  const segments: LogSegment[] = []
  let lastIndex = 0
  let currentColor: string | undefined

  const esc = String.fromCharCode(27)
  const re = new RegExp(`${esc}\\[(\\d+)m`, 'g')
  const matchAll = text.matchAll(re)
  for (const m of matchAll) {
    if (m.index! > lastIndex) {
      segments.push({ text: text.slice(lastIndex, m.index!), color: currentColor })
    }
    const code = parseInt(m[1], 10)
    if (code in ansiColors) {
      currentColor = ansiColors[code]
    }
    lastIndex = m.index! + m[0].length
  }

  if (lastIndex < text.length) {
    segments.push({ text: text.slice(lastIndex), color: currentColor })
  }

  if (segments.length === 0) {
    segments.push({ text })
  }

  return segments
}

const parsedLines = computed(() => {
  return logs.value.map((line, i) => ({
    lineNumber: i + 1,
    segments: parseAnsi(line),
  }))
})

function clearLogs() {
  if (props.taskId) {
    execution.logs[props.taskId] = []
  }
  autoScroll.value = true
}

async function copyLogs() {
  const text = logs.value.join('\n')
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    /* clipboard API not available */
  }
}

async function exportLogs() {
  const text = logs.value.join('\n')
  if (!text.trim()) return

  try {
    const filePath = await OpenSaveFileDialog({
      title: '导出日志',
      filterName: '日志文件',
      filterGlob: '*.log',
      directory: false,
      defaultDirectory: '',
      defaultFilename: '',
    })
    if (!filePath) return
    await SaveTextFile(filePath, text)
    message.success('日志已导出')
  } catch {
    message.error('日志导出失败')
  }
}

function getScrollbarContainer(): HTMLElement | null {
  const exposedInst = (terminalRef.value as (InstanceType<typeof NScrollbar> & {
    scrollbarInstRef?: {
      containerRef?: HTMLElement | null
    } | null
  }) | null)?.scrollbarInstRef

  return exposedInst?.containerRef ?? terminalRef.value?.$el.querySelector('.n-scrollbar-container') ?? null
}

async function scrollToBottom() {
  await nextTick()
  const el = getScrollbarContainer()
  if (!el) return

  isProgrammaticScroll = true
  el.scrollTop = el.scrollHeight

  if (autoScrollResetFrame !== null) {
    cancelAnimationFrame(autoScrollResetFrame)
  }
  autoScrollResetFrame = requestAnimationFrame(() => {
    autoScrollResetFrame = requestAnimationFrame(() => {
      isProgrammaticScroll = false
      syncAutoScrollState()
      autoScrollResetFrame = null
    })
  })
}

function syncAutoScrollState() {
  if (isProgrammaticScroll) return
  const el = getScrollbarContainer()
  if (!el) return
  const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  autoScroll.value = distanceToBottom <= 24
}

async function toggleAutoScroll() {
  autoScroll.value = !autoScroll.value
  if (autoScroll.value) {
    await scrollToBottom()
  }
}

watch(
  () => logs.value.length,
  async () => {
    if (!autoScroll.value) return
    await scrollToBottom()
  },
  { flush: 'post' },
)

watch(() => props.taskId, async () => {
  autoScroll.value = true
  await scrollToBottom()
}, { immediate: true, flush: 'post' })
</script>

<template>
  <div
    class="surface-muted-divider flex min-h-0 flex-1 flex-col overflow-hidden rounded-b-lg border-t shell-bg"
  >
    <div class="flex shrink-0 items-center justify-between border-b border-[rgb(var(--color-border-subtle)/0.42)] bg-[rgb(var(--color-bg-panel)/0.26)] px-3 py-1.5 backdrop-blur-sm">
      <div class="flex items-center gap-x-1.5">
        <span class="h-2.5 w-2.5 rounded-full bg-[rgb(var(--color-error)/0.78)]" />
        <span class="h-2.5 w-2.5 rounded-full bg-[rgb(var(--color-warning)/0.78)]" />
        <span class="h-2.5 w-2.5 rounded-full bg-[rgb(var(--color-success)/0.78)]" />
        <NText
          depth="3"
          class="ml-3 text-[11px]"
        >
          TERMINAL
        </NText>
        <NTag
          :bordered="false"
          size="tiny"
          :style="terminalModeTagStyle"
        >
          <template #icon>
            <NIcon
              :component="terminalModeIcon"
              size="10"
              :color="terminalTheme.accent"
            />
          </template>
          {{ terminalModeLabel }}
        </NTag>
        <template v-if="activeTask">
          <span class="text-[10px] text-[rgb(var(--color-fg-muted)/0.72)]">·</span>
          <NText
            :depth="3"
            class="text-[11px]"
          >
            {{ statusLabel(activeTask.status) }}
          </NText>
        </template>
        <span
          v-if="logs.length > 0"
          class="text-[10px] text-[rgb(var(--color-fg-muted)/0.72)]"
        >
          · {{ logs.length }} 行
        </span>
      </div>
      <div class="flex items-center gap-x-1">
        <NTooltip placement="top">
          <template #trigger>
            <NButton
              text
              size="tiny"
              :disabled="logs.length === 0"
              @click="clearLogs"
            >
              <template #icon>
                <NIcon
                  :component="Trash"
                  size="14"
                />
              </template>
            </NButton>
          </template>
          清空日志
        </NTooltip>
        <NTooltip placement="top">
          <template #trigger>
            <NButton
              text
              size="tiny"
              :disabled="logs.length === 0"
              @click="copyLogs"
            >
              <template #icon>
                <NIcon
                  :component="Copy"
                  size="14"
                />
              </template>
            </NButton>
          </template>
          复制日志
        </NTooltip>
        <NTooltip placement="top">
          <template #trigger>
            <NButton
              text
              size="tiny"
              :disabled="logs.length === 0"
              @click="exportLogs"
            >
              <template #icon>
                <NIcon
                  :component="Download"
                  size="14"
                />
              </template>
            </NButton>
          </template>
          导出日志
        </NTooltip>
        <NTooltip placement="top">
          <template #trigger>
            <NButton
              text
              size="tiny"
              class="execution-terminal-follow-button"
              @click="toggleAutoScroll"
            >
              跟随
            </NButton>
          </template>
          {{ autoScroll ? '已开启自动跟随输出' : '已暂停自动跟随输出' }}
        </NTooltip>
      </div>
    </div>

    <NScrollbar
      ref="terminalRef"
      class="flex-1"
      @scroll="syncAutoScrollState"
    >
      <div class="p-3 font-mono text-sm leading-relaxed">
        <template v-if="logs.length === 0">
          <div class="flex items-center gap-x-2 shell-text">
            <span class="cursor-blink" />
            <NText
              depth="3"
              class="text-xs"
            >
              {{ activeTask && activeTask.status === 'running' ? '等待日志输出...' : '点击上方执行按钮后，这里将实时显示输出日志' }}
            </NText>
          </div>
        </template>
        <div
          v-for="line in parsedLines"
          :key="line.lineNumber"
          class="flex"
          :class="autoScroll ? '' : ''"
        >
          <span class="mr-3 w-10 shrink-0 select-none text-right text-[rgb(var(--color-fg-muted)/0.34)]">
            {{ String(line.lineNumber).padStart(3, '0') }}
          </span>
          <span
            class="min-w-0 shell-text"
            :class="workspace.settings.autoWordWrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre'"
          >
            <span
              v-for="(seg, i) in line.segments"
              :key="i"
              :style="seg.color ? { color: seg.color } : {}"
            >{{ seg.text }}</span>
          </span>
        </div>
      </div>
    </NScrollbar>
  </div>
</template>

<style scoped>
.execution-terminal-follow-button {
  color: v-bind(followButtonColor);
  transition:
    color 0.16s var(--ease-out-soft),
    opacity 0.16s var(--ease-out-soft);
}

.execution-terminal-follow-button:hover:not(:disabled) {
  color: v-bind(terminalAccent) !important;
}
</style>
