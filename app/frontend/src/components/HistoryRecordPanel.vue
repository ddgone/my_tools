<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NIcon, NTag } from 'naive-ui'
import {
  Play,
  GlobeOutline,
  LaptopOutline,
  TimerOutline,
} from '@vicons/ionicons5'
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

const theme = computed(() =>
  getExecutionTheme(tool.value?.kind, props.tab.executionTarget),
)

// ── Status ──
const statusInfo = computed(() => {
  switch (record.value?.status) {
    case 'success':
      return { label: '已完成', color: 'rgb(34,197,94)' }
    case 'error':
      return { label: '失败', color: 'rgb(239,68,68)' }
    case 'canceled':
      return { label: '已取消', color: 'rgb(156,163,175)' }
    default:
      return { label: '', color: 'rgb(156,163,175)' }
  }
})

// ── Duration ──
const durationText = computed(() => {
  const r = record.value
  if (!r?.startedAt || !r.endedAt) return ''
  const ms = Math.max(0, r.endedAt - r.startedAt)
  if (ms < 1000) return `${ms}ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)}s`
  const m = Math.floor(s / 60)
  const remain = (s - m * 60).toFixed(0)
  return `${m}m ${remain}s`
})

// ── Time formatters ──
function pad2(n: number) {
  return String(n).padStart(2, '0')
}

function formatDate(ts?: number) {
  if (!ts) return ''
  const d = new Date(ts)
  return `${d.getFullYear()}-${pad2(d.getMonth() + 1)}-${pad2(d.getDate())}`
}

function formatTimeOfDay(ts?: number) {
  if (!ts) return ''
  const d = new Date(ts)
  return `${pad2(d.getHours())}:${pad2(d.getMinutes())}:${pad2(d.getSeconds())}`
}

const isRemote = computed(() => props.tab.executionTarget === 'remote')

// ── Structured params ──
interface ParamRow {
  label: string
  value: string
}

const structuredParams = computed<ParamRow[]>(() => {
  if (!tool.value) return []
  const formModel = props.tab.localConfig.formModel
  return tool.value.params
    .filter((p) => p.emit !== false)
    .map((p) => {
      const raw = formModel[p.key]
      if (raw === null || raw === undefined || raw === '') return null
      const displayValue = String(raw)
      return { label: p.label || p.key, value: displayValue }
    })
    .filter(Boolean) as ParamRow[]
})

const hasStructuredParams = computed(() => structuredParams.value.length > 0)
</script>

<template>
  <div v-if="tool">
    <!-- ═══ Header ═══ -->
    <div class="flex items-start justify-between gap-4">
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-3">
          <h2
            class="m-0 truncate text-lg font-semibold"
            :style="{ color: 'rgb(var(--color-fg-base)/0.96)' }"
          >
            {{ tool.name }}
          </h2>
          <NTag
            :bordered="false"
            size="small"
            :style="{
              color: statusInfo.color,
              background: `${statusInfo.color}12`,
              border: `1px solid ${statusInfo.color}28`,
            }"
          >
            {{ statusInfo.label }}
          </NTag>
          <NTag
            :bordered="true"
            size="small"
            :style="{
              color: isRemote ? 'rgb(var(--color-mode-remote)/1)' : 'rgb(var(--color-mode-local)/1)',
              background: isRemote ? 'rgb(var(--color-mode-remote)/0.10)' : 'rgb(var(--color-mode-local)/0.10)',
              borderColor: isRemote ? 'rgb(var(--color-mode-remote)/0.24)' : 'rgb(var(--color-mode-local)/0.24)',
            }"
          >
            <template #icon>
              <NIcon
                :component="isRemote ? GlobeOutline : LaptopOutline"
                size="11"
              />
            </template>
            {{ isRemote ? '远程' : '本地' }}
          </NTag>
        </div>
        <div
          v-if="record?.startedAt"
          class="mt-1.5 flex items-center gap-2 text-xs"
          :style="{ color: 'rgb(var(--color-fg-muted)/0.70)' }"
        >
          <span>{{ formatDate(record.startedAt) }}</span>
          <span :style="{ color: 'rgb(var(--color-fg-muted)/0.40)' }">·</span>
          <span>{{ formatTimeOfDay(record.startedAt) }}</span>
          <template v-if="durationText">
            <span :style="{ color: 'rgb(var(--color-fg-muted)/0.40)' }">·</span>
            <span class="flex items-center gap-1">
              <NIcon
                :component="TimerOutline"
                size="12"
              />
              {{ durationText }}
            </span>
          </template>
        </div>
      </div>

      <NButton
        size="small"
        class="history-reexecute-btn border shadow-sm shrink-0"
        @click="emit('re-execute')"
      >
        <template #icon>
          <NIcon :component="Play" />
        </template>
        重新执行
      </NButton>
    </div>

    <!-- ═══ Info cards ═══ -->
    <div class="mt-5 grid grid-cols-3 gap-3">
      <div class="surface-panel rounded-lg p-3">
        <div class="text-[10px] uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.82)]">
          开始时间
        </div>
        <div
          class="mt-1 text-sm font-medium"
          :style="{ color: 'rgb(var(--color-fg-base)/0.88)' }"
        >
          {{ record?.startedAt ? formatTimeOfDay(record.startedAt) : '--:--:--' }}
        </div>
      </div>
      <div class="surface-panel rounded-lg p-3">
        <div class="text-[10px] uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.82)]">
          结束时间
        </div>
        <div
          class="mt-1 text-sm font-medium"
          :style="{ color: 'rgb(var(--color-fg-base)/0.88)' }"
        >
          {{ record?.endedAt ? formatTimeOfDay(record.endedAt) : '--:--:--' }}
        </div>
      </div>
      <div class="surface-panel rounded-lg p-3">
        <div class="text-[10px] uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.82)]">
          执行耗时
        </div>
        <div
          class="mt-1 text-sm font-medium"
          :style="{ color: durationText ? 'rgb(var(--color-fg-base)/0.88)' : 'rgb(var(--color-fg-muted)/0.50)' }"
        >
          {{ durationText || '—' }}
        </div>
      </div>
    </div>

    <!-- ═══ Structured params ═══ -->
    <div
      v-if="hasStructuredParams"
      class="mt-4 surface-panel rounded-xl p-4"
    >
      <div class="mb-3 text-[10px] uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.82)]">
        参数明细
      </div>
      <div class="history-params-grid">
        <div
          v-for="p in structuredParams"
          :key="p.label"
          class="history-param-row"
        >
          <span class="history-param-label text-xs">
            {{ p.label }}
          </span>
          <code
            class="block break-all font-mono text-sm leading-relaxed text-[rgb(var(--color-fg-base)/0.88)]"
          >
            {{ p.value }}
          </code>
        </div>
      </div>
    </div>

    <!-- ═══ CLI args ═══ -->
    <div class="mt-4 surface-panel rounded-xl p-4">
      <div class="mb-2.5 flex items-center justify-between">
        <div class="text-[10px] uppercase tracking-wider text-[rgb(var(--color-fg-muted)/0.82)]">
          命令行
        </div>
        <span class="text-[10px] text-[rgb(var(--color-fg-muted)/0.50)]">
          CLI
        </span>
      </div>
      <code
        class="block break-all font-mono text-sm leading-relaxed text-[rgb(var(--color-fg-base)/0.88)]"
      >
        {{ props.tab.localConfig.rawArgs || '(无参数)' }}
      </code>
    </div>
  </div>
</template>

<style scoped>
.history-reexecute-btn {
  --n-color: v-bind('theme.accent') !important;
  --n-color-hover: v-bind('theme.accentHover') !important;
  --n-color-pressed: v-bind('theme.accentHover') !important;
  --n-border: 1px solid v-bind('theme.accentSoftBorder') !important;
  --n-border-hover: 1px solid v-bind('theme.accentSoftStrongBorder') !important;
  --n-text-color: v-bind('theme.accentText') !important;
  --n-text-color-hover: v-bind('theme.accentText') !important;
  --n-ripple-color: transparent !important;
}

.history-params-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
  gap: 12px;
}

.history-param-row {
  min-width: 0;
}

.history-param-label {
  display: block;
  color: rgb(var(--color-fg-muted) / 0.82);
}
</style>
