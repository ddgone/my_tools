<script setup lang="ts">
import { ref, watch } from 'vue'
import { NButton, NIcon, NInput, NPopover, NText, NTag } from 'naive-ui'
import { Play, Globe, Stop, CloudUpload, ServerOutline, CodeSlash, LogoPython } from '@vicons/ionicons5'
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
        <div class="flex items-center gap-x-2">
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
            :type="tool.kind === 'python' ? 'success' : 'info'"
          >
            <template #icon>
              <NIcon
                :component="tool.kind === 'python' ? LogoPython : CodeSlash"
                size="10"
              />
            </template>
            {{ tool.kind === 'python' ? 'py' : 'go' }}
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
        <NButton
          v-press
          type="success"
          size="small"
          :disabled="isRunning || isLaunching"
          :loading="isLaunching"
          @click="emit('execute')"
        >
          <template #icon>
            <NIcon :component="Play" />
          </template>
          本地运行
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

        <NPopover
          v-model:show="remotePopoverVisible"
          trigger="click"
          placement="bottom-end"
          :disabled="isRunning"
        >
          <template #trigger>
            <NButton
              v-press
              type="info"
              size="small"
              :disabled="isRunning"
              :secondary="!remotePopoverVisible"
              :class="remotePopoverVisible ? 'ring-2 ring-dracula-cyan/40 shadow-lg shadow-dracula-cyan/20' : ''"
            >
              <template #icon>
                <NIcon :component="Globe" />
              </template>
              远程执行
            </NButton>
          </template>
          <div class="min-w-[220px] rounded-lg border border-white/15 bg-dracula-panel p-1.5">
            <NText
              depth="3"
              class="block px-2 pb-1.5 text-[10px] uppercase"
            >
              选择目标服务器
            </NText>
            <div
              v-if="sshConnections.length === 0"
              class="px-2 py-3 text-center"
            >
              <NText
                depth="3"
                class="text-xs"
              >
                暂无 SSH 连接
              </NText>
            </div>
            <button
              v-for="conn in sshConnections"
              :key="conn.id"
              class="flex w-full items-center gap-x-2 rounded-md px-2.5 py-1.5 text-left text-xs transition hover:bg-white/5"
              @click="selectRemote(conn.id)"
            >
              <NIcon
                :component="ServerOutline"
                size="12"
              />
              <span class="truncate text-slate-200">{{ conn.name }}</span>
              <NText
                depth="3"
                class="ml-auto shrink-0 text-[10px]"
              >
                {{ conn.user }}@{{ conn.host }}
              </NText>
            </button>
          </div>
        </NPopover>

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
      v-if="tool.kind === 'python' && tab"
      class="mt-3 flex items-center gap-x-2"
    >
      <NText
        depth="3"
        class="shrink-0 text-[11px] uppercase tracking-wide"
      >
        Python 环境
      </NText>
      <NInput
        :value="tab.pythonEnv"
        placeholder="python"
        size="small"
        class="w-32"
        @update:value="emit('update:python-env', $event)"
      />
    </div>
  </div>
</template>
