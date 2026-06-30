<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NCard, NInput, useMessage } from 'naive-ui'

const message = useMessage()
const source = ref(`{
  "service": "fire-salamander",
  "features": ["format", "minify", "validate"]
}`)
const output = ref('')

function parseSource() {
  return JSON.parse(source.value)
}

function copyOutput() {
  if (!output.value) {
    message.warning('当前没有可复制的输出')
    return
  }
  void navigator.clipboard.writeText(output.value)
  message.success('已复制输出结果')
}

function applyOutputToInput() {
  if (!output.value) {
    message.warning('当前没有可回填的输出')
    return
  }
  source.value = output.value
  message.success('已将输出回填到输入区')
}

function stableSort(value: unknown): unknown {
  if (Array.isArray(value)) {
    return value.map((item) => stableSort(item))
  }

  if (value && typeof value === 'object') {
    return Object.keys(value as Record<string, unknown>)
      .sort((a, b) => a.localeCompare(b, 'zh-CN'))
      .reduce<Record<string, unknown>>((acc, key) => {
        acc[key] = stableSort((value as Record<string, unknown>)[key])
        return acc
      }, {})
  }

  return value
}

function handleFormat() {
  try {
    output.value = JSON.stringify(parseSource(), null, 2)
    message.success('已完成格式化')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function handleMinify() {
  try {
    output.value = JSON.stringify(parseSource())
    message.success('已完成压缩')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function handleSortedFormat() {
  try {
    output.value = JSON.stringify(stableSort(parseSource()), null, 2)
    message.success('已完成稳定排序格式化')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function handleValidate() {
  try {
    parseSource()
    output.value = 'JSON 校验通过'
    message.success('JSON 有效')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

const validationState = computed(() => {
  try {
    const parsed = parseSource()
    if (Array.isArray(parsed)) {
      return {
        valid: true,
        summary: `根节点为数组，共 ${parsed.length} 项。`,
      }
    }
    if (parsed && typeof parsed === 'object') {
      return {
        valid: true,
        summary: `根节点为对象，共 ${Object.keys(parsed as Record<string, unknown>).length} 个键。`,
      }
    }
    return {
      valid: true,
      summary: `根节点为 ${typeof parsed}。`,
    }
  } catch (error) {
    return {
      valid: false,
      summary: error instanceof Error ? error.message : String(error),
    }
  }
})
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
          @click="handleFormat"
        >
          格式化
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleMinify"
        >
          压缩
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleSortedFormat"
        >
          排序格式化
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleValidate"
        >
          校验
        </NButton>
      </div>

      <div class="mt-2 flex flex-wrap items-center gap-2">
        <NButton
          size="small"
          secondary
          type="primary"
          @click="copyOutput"
        >
          复制输出
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="applyOutputToInput"
        >
          回填输出
        </NButton>
      </div>

      <div class="mt-4 grid gap-4 xl:grid-cols-2">
        <div>
          <div class="mb-2 text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            输入
          </div>
          <NInput
            v-model:value="source"
            type="textarea"
            :autosize="{ minRows: 16, maxRows: 24 }"
            placeholder="输入原始 JSON"
          />
        </div>
        <div>
          <div class="mb-2 text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            输出
          </div>
          <NInput
            v-model:value="output"
            type="textarea"
            :autosize="{ minRows: 16, maxRows: 24 }"
            placeholder="格式化、压缩或排序后的结果会显示在这里"
          />
        </div>
      </div>
    </NCard>

    <NCard
      size="small"
      :bordered="true"
      class="bg-[rgb(var(--color-bg-panel)/0.78)]"
    >
      <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
        校验状态
      </div>
      <div
        class="mt-3 rounded-xl border px-4 py-3 text-sm leading-6"
        :class="validationState.valid
          ? 'border-emerald-400/25 bg-emerald-400/10 text-emerald-100'
          : 'border-rose-400/25 bg-rose-400/10 text-rose-100'"
      >
        {{ validationState.summary }}
      </div>
    </NCard>
  </div>
</template>
