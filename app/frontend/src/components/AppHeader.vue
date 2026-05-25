<script setup lang="ts">
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const runningCount = () => execution.tasks.filter((t) => t.status === 'running').length
</script>

<template>
  <header class="flex h-12 shrink-0 items-center justify-between border-b border-dracula-soft bg-dracula-panel px-4">
    <div class="flex items-center gap-3">
      <span class="text-lg">🦎</span>
      <span class="text-sm font-semibold tracking-wide text-white">
        {{ workbench.bootstrap?.appTitle ?? '火蜥蜴工具箱' }}
      </span>
    </div>

    <div class="flex items-center gap-1">
      <button
        class="rounded-md px-2.5 py-1.5 text-xs text-slate-400 transition hover:bg-white/5 hover:text-white"
        title="搜索工具 (Ctrl+P)"
        @click="workspace.showSearch = true"
      >
        🔍 <span class="ml-1 hidden sm:inline">搜索</span>
        <span class="ml-1 rounded border border-dracula-soft px-1 text-[10px] text-slate-600">⌘P</span>
      </button>
      <button
        class="rounded-md px-2.5 py-1.5 text-xs text-slate-400 transition hover:bg-white/5 hover:text-white"
        title="SSH 服务器管理"
      >
        🔗
      </button>
      <button
        class="rounded-md px-2.5 py-1.5 text-xs text-slate-400 transition hover:bg-white/5 hover:text-white"
        title="导出中心"
      >
        📦
      </button>
      <button
        class="relative rounded-md px-2.5 py-1.5 text-xs text-slate-400 transition hover:bg-white/5 hover:text-white"
        title="任务管理"
      >
        📋
        <span
          v-if="runningCount() > 0"
          class="absolute -right-0.5 -top-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-dracula-red px-1 text-[10px] font-bold text-white"
        >
          {{ runningCount() }}
        </span>
      </button>
      <button
        class="rounded-md px-2.5 py-1.5 text-xs text-slate-400 transition hover:bg-white/5 hover:text-white"
        title="系统设置"
        @click="workspace.showSettings = true"
      >
        ⚙️
      </button>
    </div>
  </header>
</template>
