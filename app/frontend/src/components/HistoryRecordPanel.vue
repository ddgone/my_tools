<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon, NTag } from 'naive-ui'
import { Play, GlobeOutline, LaptopOutline } from '@vicons/ionicons5'
import { useWorkbenchStore } from '@/stores/workbench'
import { useWorkspaceStore } from '@/stores/workspace'
import type { ToolTabState } from '@/stores/workspace'
import { getExecutionTheme } from '@/utils/executionTheme'

const props = defineProps<{
  tab: ToolTabState
}>()

const emit = defineEmits<{
  're-execute': []
}>()

const workbench = useWorkbenchStore()
const workspace = useWorkspaceStore()

const tool = computed(() =>
  workbench.bootstrap?.tools.find((t) => t.id === props.tab.toolId) ?? null,
)

const record = computed(() =>
  workspace.executionRecords.find((r) => r.id === props.tab.lastRecordId) ?? null,
)

const theme = computed(() => getExecutionTheme(tool.value?.kind, props.tab.executionTarget))
const statusLabel = computed(() => {
  switch (record.value?.status) {
    case 'success': return '已完成'
    case 'error': return '失败'
    case 'canceled': return '已取消'
    default: return ''
  }
})
const statusColor = computed(() => {
  switch (record.value?.status) {
    case 'success': return 'rgb(34,197,94)'
    case 'error': return 'rgb(239,68,68)'
    default: return 'rgb(156,163,175)'
  }
})

function formatTime(ts?: number) {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}
</script>

<template>
  <div
    v-if="tool"
    class="history-record-panel p-4"
  >
    <!-- Header -->
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
          <h2 class="m-0 text-lg font-semibold text-[rgb(var(--color-fg-base)/0.98)]">
            {{ tool.name }}
          </h2>
          <NTag
            :bordered="false"
            size="small"
            :style="{ color: statusColor, background: `${statusColor}14`, border: `1px solid ${statusColor}30` }"
          >
            {{ statusLabel }}
          </NTag>
          <NTag
            :bordered="true"
            size="small"
            :style="{
              color: props.tab.executionTarget === 'remote' ? 'rgb(var(--color-mode-remote)/1)' : 'rgb(var(--color-mode-local)/1)',
              background: props.tab.executionTarget === 'remote' ? 'rgb(var(--color-mode-remote)/0.10)' : 'rgb(var(--color-mode-local)/0.10)',
              borderColor: props.tab.executionTarget === 'remote' ? 'rgb(var(--color-mode-remote)/0.24)' : 'rgb(var(--color-mode-local)/0.24)',
            }"
          >
            <template #icon>
              <NIcon
                :component="props.tab.executionTarget === 'remote' ? GlobeOutline : LaptopOutline"
                size="11"
              />
            </template>
            {{ props.tab.executionTarget === 'remote' ? '远程' : '本地' }}
          </NTag>
        </div>
        <div
          v-if="record?.startedAt"
          class="mt-1 text-xs text-[rgb(var(--color-fg-muted)/0.75)]"
        >
          {{ formatTime(record.startedAt) }}
        </div>
      </div>

      <NButton
        size="small"
        class="tool-detail-action-button border shadow-sm shrink-0"
        @click="emit('re-execute')"
      >
        <template #icon>
          <NIcon :component="Play" />
        </template>
        重新执行
      </NButton>
    </div>

    <!-- CLI args -->
    <div class="mt-4 surface-panel rounded-xl p-4">
      <div class="mb-2 text-[10px] uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.82)]">
        执行参数
      </div>
      <code
        class="block break-all font-mono text-sm leading-relaxed"
        :style="{ color: theme.accent }"
      >
        {{ props.tab.localConfig.rawArgs || '(无参数)' }}
      </code>
    </div>
  </div>
</template>

<style scoped>
.tool-detail-action-button {
  --n-color: v-bind('theme.accent') !important;
  --n-color-hover: v-bind('theme.accentHover') !important;
  --n-color-pressed: v-bind('theme.accentHover') !important;
  --n-border: 1px solid v-bind('theme.accentSoftBorder') !important;
  --n-border-hover: 1px solid v-bind('theme.accentSoftStrongBorder') !important;
  --n-text-color: v-bind('theme.accentText') !important;
  --n-text-color-hover: v-bind('theme.accentText') !important;
  --n-ripple-color: transparent !important;
}
</style>
