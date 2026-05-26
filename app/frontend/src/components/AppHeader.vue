<script setup lang="ts">
import { NButton, NIcon, NInput, NTooltip } from 'naive-ui'
import { CloudUpload, ServerOutline, List, HelpCircle, Search } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'

const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const runningCount = () => execution.tasks.filter((t) => t.status === 'running').length

function openSearch() {
  workspace.showSearch = true
}

function openHotkeyHelp() {
  workspace.showHotkeyHelp = true
}
</script>

<template>
  <header class="flex h-12 shrink-0 items-center justify-between border-b border-dracula-soft bg-dracula-panel/80 px-4 backdrop-blur-sm">
    <div class="flex items-center gap-x-2">
      <span class="text-sm font-semibold tracking-wide text-dracula-text">
        火蜥蜴工具箱
      </span>
      <span class="text-[10px] text-dracula-soft">
        Desktop
      </span>
    </div>

    <div class="absolute left-1/2 -translate-x-1/2">
      <NInput
        placeholder="搜索工具..."
        round
        size="small"
        class="w-80"
        @focus="openSearch"
      >
        <template #prefix>
          <NIcon :component="Search" />
        </template>
        <template #suffix>
          <span class="rounded border border-dracula-soft px-1 text-[10px] text-slate-500">
            Ctrl+P
          </span>
        </template>
      </NInput>
    </div>

    <div class="flex items-center gap-x-0.5">
      <NTooltip placement="bottom">
        <template #trigger>
          <NButton
            quaternary
            circle
            size="small"
            class="text-dracula-soft hover:text-dracula-text"
          >
            <template #icon>
              <NIcon
                :component="CloudUpload"
                size="18"
              />
            </template>
          </NButton>
        </template>
        导出中心
      </NTooltip>

      <NTooltip placement="bottom">
        <template #trigger>
          <NButton
            quaternary
            circle
            size="small"
            class="text-dracula-soft hover:text-dracula-text"
          >
            <template #icon>
              <NIcon
                :component="ServerOutline"
                size="18"
              />
            </template>
          </NButton>
        </template>
        SSH 连接管理
      </NTooltip>

      <NTooltip placement="bottom">
        <template #trigger>
          <NButton
            quaternary
            circle
            size="small"
            class="relative text-dracula-soft hover:text-dracula-text"
          >
            <template #icon>
              <NIcon
                :component="List"
                size="18"
              />
            </template>
            <span
              v-if="runningCount() > 0"
              class="absolute right-0 top-0 flex h-4 min-w-4 items-center justify-center rounded-full bg-dracula-red px-1 text-[10px] font-bold text-white"
            >
              {{ runningCount() }}
            </span>
          </NButton>
        </template>
        任务中心
      </NTooltip>

      <NTooltip placement="bottom">
        <template #trigger>
          <NButton
            quaternary
            circle
            size="small"
            class="text-dracula-soft hover:text-dracula-text"
            @click="openHotkeyHelp"
          >
            <template #icon>
              <NIcon
                :component="HelpCircle"
                size="18"
              />
            </template>
          </NButton>
        </template>
        快捷键帮助 (F1)
      </NTooltip>
    </div>
  </header>
</template>
