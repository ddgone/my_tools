<script setup lang="ts">
import { NButton, NIcon, NInput, NTooltip } from 'naive-ui'
import { HelpCircle, Search } from '@vicons/ionicons5'
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
  <header class="flex h-12 shrink-0 items-center justify-between border-b border-white/15 bg-dracula-panel/80 px-4 backdrop-blur-sm">
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
        size="small"
        class="w-80"
        @focus="openSearch"
      >
        <template #prefix>
          <NIcon :component="Search" />
        </template>
        <template #suffix>
          <span class="rounded border border-white/15 px-1 text-[10px] text-slate-500">
            Ctrl+P
          </span>
        </template>
      </NInput>
    </div>

    <div class="flex items-center gap-x-1">
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

      <span
        v-if="runningCount() > 0"
        class="ml-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-dracula-red px-1.5 text-[10px] font-bold text-white"
      >
        {{ runningCount() }}
      </span>
    </div>
  </header>
</template>
