<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NCard, NInput, NSelect, useMessage } from 'naive-ui'

const message = useMessage()

type TimeFormatKey =
  | 'unix_seconds'
  | 'unix_milliseconds'
  | 'iso_utc'
  | 'iso_offset'
  | 'datetime'
  | 'date_only'

interface TimeFormatOption {
  label: string
  value: TimeFormatKey
  help: string
  needsOffset: boolean
}

interface ParseResult {
  ms: number
  normalizedSource: string
}

const formatOptions: TimeFormatOption[] = [
  { label: 'Unix 秒', value: 'unix_seconds', help: '例如 1749283200', needsOffset: false },
  { label: 'Unix 毫秒', value: 'unix_milliseconds', help: '例如 1749283200000', needsOffset: false },
  { label: 'ISO UTC', value: 'iso_utc', help: '例如 2026-06-07T14:30:00.000Z', needsOffset: false },
  { label: 'ISO 带偏移', value: 'iso_offset', help: '例如 2026-06-07T14:30:00+08:00', needsOffset: false },
  { label: '日期时间', value: 'datetime', help: '例如 2026-06-07 14:30:00，需要指定时区', needsOffset: true },
  { label: '日期', value: 'date_only', help: '例如 2026-06-07，按 00:00:00 解析', needsOffset: true },
]

const selectFormatOptions = formatOptions.map((item) => ({
  label: item.label,
  value: item.value,
}))

const sourceFormat = ref<TimeFormatKey>('unix_milliseconds')
const targetFormat = ref<TimeFormatKey>('datetime')
const sourceOffset = ref('+08:00')
const targetOffset = ref('+08:00')
const sourceValue = ref(String(Date.now()))

function pad(value: number) {
  return String(value).padStart(2, '0')
}

function formatOffset(totalMinutes: number) {
  const sign = totalMinutes >= 0 ? '+' : '-'
  const abs = Math.abs(totalMinutes)
  const hours = Math.floor(abs / 60)
  const minutes = abs % 60
  return `${sign}${pad(hours)}:${pad(minutes)}`
}

function parseOffset(value: string) {
  const match = /^([+-])(\d{2}):(\d{2})$/.exec(value.trim())
  if (!match) {
    return 0
  }
  const total = Number(match[2]) * 60 + Number(match[3])
  return match[1] === '-' ? -total : total
}

function shiftDate(ms: number, offsetMinutes: number) {
  return new Date(ms + offsetMinutes * 60_000)
}

function formatDateTimeAtOffset(ms: number, offsetMinutes: number) {
  const date = shiftDate(ms, offsetMinutes)
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())} ${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}`
}

function formatDateAtOffset(ms: number, offsetMinutes: number) {
  return formatDateTimeAtOffset(ms, offsetMinutes).slice(0, 10)
}

function formatIsoOffset(ms: number, offsetMinutes: number) {
  const date = shiftDate(ms, offsetMinutes)
  return `${date.getUTCFullYear()}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}T${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}${formatOffset(offsetMinutes)}`
}

function parseDateTimeParts(input: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2})(?::(\d{2}))?$/.exec(input.trim())
  if (!match) {
    return null
  }
  return {
    year: Number(match[1]),
    month: Number(match[2]),
    day: Number(match[3]),
    hour: Number(match[4]),
    minute: Number(match[5]),
    second: Number(match[6] ?? '0'),
  }
}

function parseDateOnlyParts(input: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(input.trim())
  if (!match) {
    return null
  }
  return {
    year: Number(match[1]),
    month: Number(match[2]),
    day: Number(match[3]),
  }
}

function parseTimeValue(value: string, format: TimeFormatKey, offset: string): ParseResult | null {
  const trimmed = value.trim()
  if (!trimmed) {
    return null
  }

  switch (format) {
    case 'unix_seconds': {
      const parsed = Number(trimmed)
      if (!Number.isFinite(parsed)) return null
      return { ms: Math.round(parsed * 1000), normalizedSource: trimmed }
    }
    case 'unix_milliseconds': {
      const parsed = Number(trimmed)
      if (!Number.isFinite(parsed)) return null
      return { ms: Math.round(parsed), normalizedSource: trimmed }
    }
    case 'iso_utc': {
      if (!trimmed.endsWith('Z')) return null
      const parsed = Date.parse(trimmed)
      if (Number.isNaN(parsed)) return null
      return { ms: parsed, normalizedSource: new Date(parsed).toISOString() }
    }
    case 'iso_offset': {
      if (!/(Z|[+-]\d{2}:\d{2})$/.test(trimmed)) return null
      const parsed = Date.parse(trimmed)
      if (Number.isNaN(parsed)) return null
      return { ms: parsed, normalizedSource: trimmed }
    }
    case 'datetime': {
      const parsed = parseDateTimeParts(trimmed)
      if (!parsed) return null
      const offsetMinutes = parseOffset(offset)
      const ms = Date.UTC(parsed.year, parsed.month - 1, parsed.day, parsed.hour, parsed.minute, parsed.second) - offsetMinutes * 60_000
      return { ms, normalizedSource: `${formatDateTimeAtOffset(ms, offsetMinutes)} UTC${formatOffset(offsetMinutes)}` }
    }
    case 'date_only': {
      const parsed = parseDateOnlyParts(trimmed)
      if (!parsed) return null
      const offsetMinutes = parseOffset(offset)
      const ms = Date.UTC(parsed.year, parsed.month - 1, parsed.day, 0, 0, 0) - offsetMinutes * 60_000
      return { ms, normalizedSource: `${formatDateAtOffset(ms, offsetMinutes)} UTC${formatOffset(offsetMinutes)}` }
    }
    default:
      return null
  }
}

function formatTargetValue(ms: number, format: TimeFormatKey, offset: string) {
  const offsetMinutes = parseOffset(offset)
  switch (format) {
    case 'unix_seconds':
      return String(Math.floor(ms / 1000))
    case 'unix_milliseconds':
      return String(ms)
    case 'iso_utc':
      return new Date(ms).toISOString()
    case 'iso_offset':
      return formatIsoOffset(ms, offsetMinutes)
    case 'datetime':
      return `${formatDateTimeAtOffset(ms, offsetMinutes)} UTC${formatOffset(offsetMinutes)}`
    case 'date_only':
      return `${formatDateAtOffset(ms, offsetMinutes)} UTC${formatOffset(offsetMinutes)}`
    default:
      return ''
  }
}

function browserLocalTime(ms: number) {
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(new Date(ms))
}

const timezoneOptions = Array.from({ length: 27 }, (_, index) => {
  const totalMinutes = (index - 12) * 60
  const value = formatOffset(totalMinutes)
  return {
    label: `UTC${value}`,
    value,
  }
})

const sourceFormatMeta = computed(() => formatOptions.find((item) => item.value === sourceFormat.value))
const targetFormatMeta = computed(() => formatOptions.find((item) => item.value === targetFormat.value))

const parsedResult = computed(() => parseTimeValue(sourceValue.value, sourceFormat.value, sourceOffset.value))

const conversionState = computed(() => {
  if (!sourceValue.value.trim()) {
    return { error: '', output: '', source: '', rows: [] as Array<{ label: string; value: string }> }
  }
  if (!parsedResult.value) {
    return {
      error: '当前输入与源格式不匹配，请检查格式或时区设置。',
      output: '',
      source: '',
      rows: [],
    }
  }

  const ms = parsedResult.value.ms
  const sourceOffsetMinutes = parseOffset(sourceOffset.value)
  const targetOffsetMinutes = parseOffset(targetOffset.value)

  return {
    error: '',
    output: formatTargetValue(ms, targetFormat.value, targetOffset.value),
    source: parsedResult.value.normalizedSource,
    rows: [
      { label: 'Unix 秒', value: formatTargetValue(ms, 'unix_seconds', targetOffset.value) },
      { label: 'Unix 毫秒', value: formatTargetValue(ms, 'unix_milliseconds', targetOffset.value) },
      { label: 'ISO UTC', value: formatTargetValue(ms, 'iso_utc', targetOffset.value) },
      { label: `源时区时间 · UTC${formatOffset(sourceOffsetMinutes)}`, value: `${formatDateTimeAtOffset(ms, sourceOffsetMinutes)} UTC${formatOffset(sourceOffsetMinutes)}` },
      { label: `目标时区时间 · UTC${formatOffset(targetOffsetMinutes)}`, value: `${formatDateTimeAtOffset(ms, targetOffsetMinutes)} UTC${formatOffset(targetOffsetMinutes)}` },
      { label: '浏览器本地时间', value: browserLocalTime(ms) },
    ],
  }
})

function copyValue(value: string) {
  void navigator.clipboard.writeText(value)
  message.success('已复制结果')
}

function fillNow() {
  sourceFormat.value = 'unix_milliseconds'
  sourceValue.value = String(Date.now())
}

function swapDirection() {
  const nextSourceFormat = targetFormat.value
  targetFormat.value = sourceFormat.value
  sourceFormat.value = nextSourceFormat

  const nextSourceOffset = targetOffset.value
  targetOffset.value = sourceOffset.value
  sourceOffset.value = nextSourceOffset

  if (conversionState.value.output) {
    sourceValue.value = conversionState.value.output
  }
}

function loadExample() {
  const exampleMap: Record<TimeFormatKey, string> = {
    unix_seconds: '1749283200',
    unix_milliseconds: '1749283200000',
    iso_utc: '2026-06-07T14:30:00.000Z',
    iso_offset: '2026-06-07T14:30:00+08:00',
    datetime: '2026-06-07 14:30:00',
    date_only: '2026-06-07',
  }
  sourceValue.value = exampleMap[sourceFormat.value]
}
</script>

<template>
  <div class="space-y-6">
    <NCard
      size="small"
      :bordered="true"
      class="bg-[rgb(var(--color-bg-panel)/0.78)]"
    >
      <div class="flex flex-wrap items-center gap-2">
        <NButton
          size="small"
          secondary
          type="primary"
          @click="fillNow"
        >
          当前时间
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="swapDirection"
        >
          交换源与目标
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="loadExample"
        >
          示例输入
        </NButton>
      </div>

      <div class="mt-4 grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <div class="space-y-4">
          <div class="rounded-2xl border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4">
            <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
              源格式
            </div>
            <div class="mt-3 grid gap-3 md:grid-cols-[1fr_180px]">
              <NSelect
                v-model:value="sourceFormat"
                :options="selectFormatOptions"
              />
              <NSelect
                v-if="sourceFormatMeta?.needsOffset"
                v-model:value="sourceOffset"
                :options="timezoneOptions"
              />
            </div>
            <div class="mt-3 text-xs leading-5 text-[rgb(var(--color-fg-muted)/0.92)]">
              {{ sourceFormatMeta?.help }}
            </div>
            <div class="mt-3">
              <NInput
                v-model:value="sourceValue"
                type="textarea"
                :autosize="{ minRows: 6, maxRows: 9 }"
                placeholder="输入源时间"
              />
            </div>
          </div>

          <div class="rounded-2xl border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4">
            <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
              目标格式
            </div>
            <div class="mt-3 grid gap-3 md:grid-cols-[1fr_180px]">
              <NSelect
                v-model:value="targetFormat"
                :options="selectFormatOptions"
              />
              <NSelect
                v-if="targetFormatMeta?.needsOffset || targetFormat === 'iso_offset'"
                v-model:value="targetOffset"
                :options="timezoneOptions"
              />
            </div>
            <div class="mt-3 text-xs leading-5 text-[rgb(var(--color-fg-muted)/0.92)]">
              {{ targetFormatMeta?.help }}
            </div>
            <div class="mt-3 rounded-xl border border-[rgb(var(--color-border-subtle)/0.72)] bg-[rgb(var(--color-bg-panel)/0.72)] px-4 py-3">
              <div class="flex items-center justify-between gap-3">
                <div class="text-xs uppercase tracking-[0.18em] text-[rgb(var(--color-fg-muted)/0.88)]">
                  转换结果
                </div>
                <NButton
                  size="tiny"
                  quaternary
                  :disabled="!conversionState.output"
                  @click="copyValue(conversionState.output)"
                >
                  复制
                </NButton>
              </div>
              <div
                v-if="conversionState.error"
                class="mt-2 text-sm leading-6 text-rose-300"
              >
                {{ conversionState.error }}
              </div>
              <div
                v-else-if="conversionState.output"
                class="mt-2 break-all font-mono text-[14px] leading-7 text-[rgb(var(--color-fg-base)/0.98)]"
              >
                {{ conversionState.output }}
              </div>
              <div
                v-else
                class="mt-2 text-sm leading-6 text-[rgb(var(--color-fg-muted)/0.92)]"
              >
                选择源格式与目标格式后开始转换。
              </div>
            </div>
          </div>
        </div>

        <div class="space-y-4">
          <div class="rounded-2xl border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4">
            <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
              标准时间点
            </div>
            <div class="mt-2 text-xs leading-5 text-[rgb(var(--color-fg-muted)/0.92)]">
              任何源格式都会先解析成统一时间点，再渲染为目标格式。
            </div>
            <div
              v-if="conversionState.error"
              class="mt-4 rounded-xl border border-rose-400/20 bg-rose-400/10 px-4 py-3 text-sm leading-6 text-rose-200"
            >
              当前无法解析源输入。
            </div>
            <div
              v-else-if="conversionState.source"
              class="mt-4 rounded-xl border border-[rgb(var(--color-border-subtle)/0.72)] bg-[rgb(var(--color-bg-panel)/0.72)] px-4 py-3"
            >
              <div class="text-xs uppercase tracking-[0.18em] text-[rgb(var(--color-fg-muted)/0.88)]">
                解析后
              </div>
              <div class="mt-2 break-all font-mono text-[14px] leading-7 text-[rgb(var(--color-fg-base)/0.98)]">
                {{ conversionState.source }}
              </div>
            </div>
            <div
              v-else
              class="mt-4 rounded-xl border border-[rgb(var(--color-border-subtle)/0.72)] bg-[rgb(var(--color-bg-panel)/0.72)] px-4 py-3 text-sm leading-6 text-[rgb(var(--color-fg-muted)/0.92)]"
            >
              输入一个时间值后，这里会展示它标准化后的样子。
            </div>
          </div>

          <div class="rounded-2xl border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] p-4">
            <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
              派生格式总览
            </div>
            <div class="mt-2 text-xs leading-5 text-[rgb(var(--color-fg-muted)/0.92)]">
              方便你一次看到同一个时间点在常见格式下的表达。
            </div>

            <div
              v-if="conversionState.rows.length === 0"
              class="mt-4 rounded-xl border border-[rgb(var(--color-border-subtle)/0.72)] bg-[rgb(var(--color-bg-panel)/0.72)] px-4 py-3 text-sm leading-6 text-[rgb(var(--color-fg-muted)/0.92)]"
            >
              转换成功后，这里会展示所有常见衍生格式。
            </div>

            <div
              v-else
              class="mt-4 space-y-3"
            >
              <div
                v-for="row in conversionState.rows"
                :key="row.label"
                class="rounded-xl border border-[rgb(var(--color-border-subtle)/0.72)] bg-[rgb(var(--color-bg-panel)/0.72)] px-3 py-3"
              >
                <div class="flex items-center justify-between gap-3">
                  <div class="text-xs text-[rgb(var(--color-fg-muted)/0.92)]">
                    {{ row.label }}
                  </div>
                  <NButton
                    size="tiny"
                    quaternary
                    @click="copyValue(row.value)"
                  >
                    复制
                  </NButton>
                </div>
                <div class="mt-2 break-all font-mono text-[13px] leading-6 text-[rgb(var(--color-fg-base)/0.98)]">
                  {{ row.value }}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </NCard>
  </div>
</template>
