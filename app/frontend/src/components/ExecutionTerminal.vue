<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useExecutionStore } from '@/stores/execution'
import { OpenSaveFileDialog } from '../../wailsjs/go/main/App'

const props = defineProps<{
  taskId: string
}>()

const execution = useExecutionStore()
const logContainerRef = ref<HTMLElement | null>(null)

const logs = computed(() => execution.logsForTask(props.taskId))

const activeTask = computed(() =>
    props.taskId ? execution.recentTasks.find((t) => t.id === props.taskId) ?? null : null,
)

function statusLabel(status?: string) {
  switch (status) {
    case 'running':
      return '运行中'
    case 'success':
      return '已完成'
    case 'error':
      return '失败'
    case 'canceled':
      return '已取消'
    default:
      return '等待中'
  }
}

function statusColor(status?: string) {
  switch (status) {
    case 'running':
      return 'text-dracula-yellow'
    case 'success':
      return 'text-dracula-green'
    case 'error':
      return 'text-dracula-red'
    case 'canceled':
      return 'text-slate-500'
    default:
      return 'text-slate-400'
  }
}

function clearLogs() {
  if (props.taskId) {
    execution.logs[props.taskId] = []
  }
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
    await OpenSaveFileDialog({
      title: '导出日志',
      filterName: '日志文件',
      filterGlob: '*.log',
      directory: false,
    })
    await navigator.clipboard.writeText(text)
  } catch {
    try {
      await navigator.clipboard.writeText(text)
    } catch { /* ignore */ }
  }
}

function logClass(line: string): string {
  const lower = line.toLowerCase()
  if (lower.includes('error') || lower.includes('失败') || lower.includes('panic')) return 'text-dracula-red'
  if (lower.includes('warn') || lower.includes('警告')) return 'text-dracula-yellow'
  if (lower.includes('success') || lower.includes('完成') || lower.includes('ok')) return 'text-dracula-green'
  return 'text-slate-300'
}

watch(
    () => logs.value.length,
    async () => {
      await nextTick()
      if (logContainerRef.value) {
        logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
      }
    },
)
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col overflow-hidden bg-[#0a0b10]">
    <div class="flex shrink-0 items-center justify-between border-b border-t border-dracula-soft px-4 py-1.5">
      <div class="flex items-center gap-3">
        <span class="text-xs font-medium text-slate-300">执行日志</span>
        <span
          v-if="activeTask"
          class="text-xs"
          :class="statusColor(activeTask.status)"
        >
          ● {{ statusLabel(activeTask.status) }}
        </span>
        <span
          v-if="activeTask?.exitMessage"
          class="text-xs text-slate-500"
        >
          {{ activeTask.exitMessage }}
        </span>
        <span
          v-if="logs.length > 0"
          class="text-[10px] text-slate-600"
        >
          {{ logs.length }} 行
        </span>
      </div>
      <div class="flex items-center gap-1">
        <button
          class="rounded px-2 py-0.5 text-[11px] text-slate-500 transition hover:bg-white/5 hover:text-slate-300 disabled:opacity-30"
          :disabled="logs.length === 0"
          @click="clearLogs"
        >
          🗑 清空
        </button>
        <button
          class="rounded px-2 py-0.5 text-[11px] text-slate-500 transition hover:bg-white/5 hover:text-slate-300 disabled:opacity-30"
          :disabled="logs.length === 0"
          @click="copyLogs"
        >
          📋 复制
        </button>
        <button
          class="rounded px-2 py-0.5 text-[11px] text-slate-500 transition hover:bg-white/5 hover:text-slate-300 disabled:opacity-30"
          :disabled="logs.length === 0"
          @click="exportLogs"
        >
          💾 导出
        </button>
      </div>
    </div>

    <div
      ref="logContainerRef"
      class="min-h-0 flex-1 overflow-auto p-2 font-mono text-[11px] leading-[1.7]"
    >
      <div
        v-if="logs.length === 0"
        class="flex h-full items-center justify-center"
      >
        <p class="text-xs text-slate-600">
          {{ activeTask && activeTask.status === 'running' ? '等待日志输出...' : '点击上方"开始本地执行"后，这里将实时显示输出日志' }}
        </p>
      </div>
      <div
        v-for="(line, index) in logs"
        :key="index"
        class="flex gap-3 whitespace-pre-wrap break-all"
      >
        <span
          class="select-none text-[10px] text-slate-700"
          style="flex: 0 0 2.5rem; text-align: right"
        >{{ index + 1 }}</span>
        <span
          class="min-w-0"
          :class="logClass(line)"
        >{{ line }}</span>
      </div>
    </div>
  </div>
</template>
