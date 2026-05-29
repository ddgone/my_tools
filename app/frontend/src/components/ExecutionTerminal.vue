<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { NButton, NIcon, NScrollbar, NText, NTooltip, useMessage } from 'naive-ui'
import { Trash, Copy, Download } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { OpenSaveFileDialog, SaveTextFile } from '../../wailsjs/go/main/App'
import gsap from 'gsap'
import { ANIM } from '@/utils/animation'

const props = defineProps<{
  taskId: string
  executionTarget: 'local' | 'remote'
  accent?: 'cyan' | 'green' | 'pink'
}>()

const execution = useExecutionStore()
const workspace = useWorkspaceStore()
const message = useMessage()
const terminalRef = ref<InstanceType<typeof NScrollbar> | null>(null)
const autoScroll = ref(true)
let detachScrollListener: (() => void) | null = null

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
    switch (code) {
      case 0: currentColor = undefined; break
      case 31: currentColor = '#ff5555'; break
      case 32: currentColor = '#50fa7b'; break
      case 33: currentColor = '#f1fa8c'; break
      case 34: currentColor = '#8be9fd'; break
      case 35: currentColor = '#ff79c6'; break
      case 36: currentColor = '#8be9fd'; break
      case 37: currentColor = '#f8f8f2'; break
      default: break
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
    })
    if (!filePath) return
    await SaveTextFile(filePath, text)
    message.success('日志已导出')
  } catch {
    message.error('日志导出失败')
  }
}

function getScrollbarContainer(): HTMLElement | null {
  return terminalRef.value?.$el.querySelector('.n-scrollbar-container') ?? null
}

async function scrollToBottom() {
  await nextTick()
  const el = getScrollbarContainer()
  if (!el) return
  gsap.to(el, {
    scrollTop: el.scrollHeight,
    duration: ANIM.duration.slow,
    ease: ANIM.ease.out,
    overwrite: 'auto',
  })
}

function syncAutoScrollState() {
  const el = getScrollbarContainer()
  if (!el) return
  const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
  autoScroll.value = distanceToBottom <= 24
}

async function bindScrollListener() {
  await nextTick()
  detachScrollListener?.()
  const el = getScrollbarContainer()
  if (!el) return
  const handleScroll = () => syncAutoScrollState()
  el.addEventListener('scroll', handleScroll, { passive: true })
  detachScrollListener = () => el.removeEventListener('scroll', handleScroll)
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
)

watch(() => props.taskId, async () => {
  autoScroll.value = true
  await bindScrollListener()
  await scrollToBottom()
}, { immediate: true })

onBeforeUnmount(() => {
  detachScrollListener?.()
})
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden shell-bg rounded-b-lg border-t border-white/15">
    <div class="flex shrink-0 items-center justify-between bg-dracula-panel/30 px-3 py-1.5">
      <div class="flex items-center gap-x-1.5">
        <span class="h-2.5 w-2.5 rounded-full bg-red-500/70" />
        <span class="h-2.5 w-2.5 rounded-full bg-yellow-500/70" />
        <span class="h-2.5 w-2.5 rounded-full bg-green-500/70" />
        <NText
          depth="3"
          class="ml-3 text-[11px]"
        >
          TERMINAL
        </NText>
        <template v-if="activeTask">
          <span class="text-[10px] text-slate-600">·</span>
          <NText
            :depth="3"
            class="text-[11px]"
          >
            {{ statusLabel(activeTask.status) }}
          </NText>
        </template>
        <span
          v-if="logs.length > 0"
          class="text-[10px] text-slate-600"
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
              :class="autoScroll
                ? props.executionTarget === 'remote'
                  ? 'text-dracula-pink'
                  : props.accent === 'green'
                    ? 'text-dracula-green'
                    : 'text-dracula-cyan'
                : 'text-slate-400'"
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
          <span class="mr-3 w-10 shrink-0 select-none text-right text-dracula-soft/30">
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
