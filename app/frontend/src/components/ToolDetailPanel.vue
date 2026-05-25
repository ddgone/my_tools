<script setup lang="ts">
import { ref, watch } from 'vue'
import { NButton, NPopover, NTag } from 'naive-ui'
import { ListSSHConnections } from '../../wailsjs/go/main/App'
import type { SSHConnection, ToolManifest } from '@/types/workbench'
import type { ToolTabState } from '@/stores/workspace'

defineProps<{
  tool: ToolManifest | null
  tab: ToolTabState | undefined
  activeTaskId: string
  isRunning: boolean
  isLaunching: boolean
}>()

const emit = defineEmits<{
  execute: []
  cancel: []
  'update:python-env': [value: string]
  remoteExecute: [connId: string]
}>()

const sshConnections = ref<SSHConnection[]>([])
const remotePopoverVisible = ref(false)

async function loadConnections() {
  sshConnections.value = await ListSSHConnections()
}

function selectRemote(connId: string) {
  remotePopoverVisible.value = false
  emit('remoteExecute', connId)
}

watch(remotePopoverVisible, (visible) => {
  if (visible) {
    loadConnections()
  }
})
</script>

<template>
  <div v-if="tool">
    <div class="flex flex-wrap items-start justify-between gap-4">
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-2 text-xs text-slate-500">
          <span>{{ tool.category }}</span>
          <span>·</span>
          <n-tag
            size="tiny"
            :bordered="false"
            :type="tool.kind === 'python' ? 'success' : 'info'"
          >
            {{ tool.kind }}
          </n-tag>
        </div>
        <h2 class="m-0 mt-1 text-lg font-semibold text-white">
          {{ tool.name }}
        </h2>
        <p class="mt-1 text-sm leading-relaxed text-slate-400">
          {{ tool.docs.summary || tool.description }}
        </p>
      </div>

      <div class="flex shrink-0 flex-wrap items-center gap-2">
        <div class="flex items-center gap-1.5">
          <n-button
            type="success"
            size="small"
            :disabled="isRunning || isLaunching"
            :loading="isLaunching"
            @click="emit('execute')"
          >
            ▶ 本地运行
          </n-button>
          <n-button
            v-if="isRunning"
            type="error"
            size="small"
            @click="emit('cancel')"
          >
            ⏹ 停止
          </n-button>

          <n-popover
            v-model:show="remotePopoverVisible"
            trigger="click"
            placement="bottom-end"
            :disabled="isRunning"
          >
            <template #trigger>
              <n-button
                type="info"
                size="small"
                :disabled="isRunning"
                secondary
              >
                🔗 远程执行
              </n-button>
            </template>
            <div class="min-w-[200px] rounded-lg border border-dracula-soft bg-dracula-panel p-1.5">
              <div class="mb-1.5 px-2 text-[10px] uppercase text-slate-500">
                选择目标服务器
              </div>
              <button
                v-for="conn in sshConnections"
                :key="conn.id"
                class="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-xs transition hover:bg-white/5"
                @click="selectRemote(conn.id)"
              >
                <span class="truncate text-slate-200">{{ conn.name }}</span>
                <span class="ml-auto shrink-0 text-[10px] text-slate-500">{{ conn.user }}@{{ conn.host }}</span>
              </button>
              <div
                v-if="sshConnections.length === 0"
                class="space-y-2 px-2 py-3 text-center text-xs text-slate-500"
              >
                <p class="m-0">
                  暂无 SSH 连接
                </p>
                <p class="m-0 text-[10px] text-slate-600">
                  请先在左侧边栏点击 🔗 添加连接
                </p>
              </div>
            </div>
          </n-popover>

          <n-button
            size="small"
            disabled
            secondary
          >
            📦 导出
          </n-button>
        </div>
      </div>
    </div>

    <div
      v-if="tool.kind === 'python' && tab"
      class="mt-3 flex items-center gap-2"
    >
      <label class="text-[11px] uppercase tracking-wide text-slate-500">Python 解释器</label>
      <input
        :value="tab.pythonEnv"
        type="text"
        placeholder="python"
        class="rounded border border-dracula-soft bg-black/30 px-2 py-1 text-xs text-slate-300 outline-none transition focus:border-dracula-cyan/50"
        @input="emit('update:python-env', ($event.target as HTMLInputElement).value)"
      >
    </div>
  </div>
</template>
