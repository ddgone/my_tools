<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NCard, NInput, useMessage } from 'naive-ui'

const message = useMessage()
const source = ref('https://example.com/search?q=fire salamander&env=dev')
const output = ref('')

function copyOutput() {
  if (!output.value) {
    message.warning('当前没有可复制的结果')
    return
  }
  void navigator.clipboard.writeText(output.value)
  message.success('已复制输出结果')
}

function encodeFullUrl() {
  output.value = encodeURI(source.value)
  message.success('已完成 URL 编码')
}

function decodeFullUrl() {
  try {
    output.value = decodeURI(source.value)
    message.success('已完成 URL 解码')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function encodeComponent() {
  output.value = encodeURIComponent(source.value)
  message.success('已完成 Component 编码')
}

function decodeComponent() {
  try {
    output.value = decodeURIComponent(source.value)
    message.success('已完成 Component 解码')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function parseQuery() {
  try {
    const base = source.value.includes('://') ? source.value : `https://placeholder.local/?${source.value.replace(/^\?/, '')}`
    const url = new URL(base)
    const entries = Array.from(url.searchParams.entries()).map(([key, value]) => ({ key, value }))
    output.value = JSON.stringify(entries, null, 2)
    message.success('已拆解查询串')
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function fillOutputBack() {
  if (!output.value) {
    message.warning('当前没有可回填的结果')
    return
  }
  source.value = output.value
}

const hint = computed(() => {
  if (!source.value.trim()) {
    return '支持完整 URL，也支持单独的 query string。'
  }
  if (source.value.includes('://')) {
    return '检测到完整 URL，可直接做 URL 编解码或提取查询参数。'
  }
  return '当前更像是纯参数字符串，适合做 Component 编解码或查询串拆解。'
})
</script>

<template>
  <div class="space-y-6">
    <NCard
      size="small"
      :bordered="true"
      class="bg-white/5"
    >
      <div class="flex flex-wrap items-center gap-2">
        <NButton
          size="small"
          secondary
          type="primary"
          @click="encodeFullUrl"
        >
          URL 编码
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="decodeFullUrl"
        >
          URL 解码
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="encodeComponent"
        >
          Component 编码
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="decodeComponent"
        >
          Component 解码
        </NButton>
      </div>

      <div class="mt-2 flex flex-wrap items-center gap-2">
        <NButton
          size="small"
          secondary
          type="primary"
          @click="parseQuery"
        >
          拆解查询串
        </NButton>
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
          @click="fillOutputBack"
        >
          回填输出
        </NButton>
      </div>

      <div class="mt-4 grid gap-4 xl:grid-cols-2">
        <div>
          <div class="mb-2 text-sm font-medium text-slate-200">
            输入
          </div>
          <NInput
            v-model:value="source"
            type="textarea"
            :autosize="{ minRows: 14, maxRows: 22 }"
            placeholder="输入完整 URL 或 query string"
          />
        </div>
        <div>
          <div class="mb-2 text-sm font-medium text-slate-200">
            输出
          </div>
          <NInput
            v-model:value="output"
            type="textarea"
            :autosize="{ minRows: 14, maxRows: 22 }"
            placeholder="转换结果会显示在这里"
          />
        </div>
      </div>
    </NCard>

    <NCard
      size="small"
      :bordered="true"
      class="bg-white/5"
    >
      <div class="text-sm font-medium text-slate-200">
        当前识别
      </div>
      <div class="mt-3 rounded-xl border border-white/10 bg-black/15 px-4 py-3 text-sm leading-6 text-slate-300">
        {{ hint }}
      </div>
    </NCard>
  </div>
</template>
