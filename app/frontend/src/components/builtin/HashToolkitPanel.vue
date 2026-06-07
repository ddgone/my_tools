<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NCard, NInput, NSelect, useMessage } from 'naive-ui'

const message = useMessage()
const source = ref('fire-salamander')
const algorithm = ref<'SHA-1' | 'SHA-256' | 'SHA-384' | 'SHA-512'>('SHA-256')
const output = ref('')

const algorithmOptions = [
  { label: 'SHA-1', value: 'SHA-1' },
  { label: 'SHA-256', value: 'SHA-256' },
  { label: 'SHA-384', value: 'SHA-384' },
  { label: 'SHA-512', value: 'SHA-512' },
]

function toHex(buffer: ArrayBuffer) {
  return Array.from(new Uint8Array(buffer))
    .map((value) => value.toString(16).padStart(2, '0'))
    .join('')
}

async function handleDigest() {
  try {
    const bytes = new TextEncoder().encode(source.value)
    const digest = await crypto.subtle.digest(algorithm.value, bytes)
    output.value = toHex(digest)
    message.success(`已生成 ${algorithm.value} 摘要`)
  } catch (error) {
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function copyOutput() {
  if (!output.value) {
    message.warning('当前没有可复制的摘要')
    return
  }
  void navigator.clipboard.writeText(output.value)
  message.success('已复制摘要')
}

const metaText = computed(() => {
  if (!output.value) {
    return '选择算法后生成十六进制摘要，可用于对比内容是否一致。'
  }
  return `${algorithm.value} 共 ${output.value.length} 个十六进制字符。`
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
        <NSelect
          v-model:value="algorithm"
          :options="algorithmOptions"
          class="w-[180px]"
        />
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleDigest"
        >
          生成摘要
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="copyOutput"
        >
          复制摘要
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
            :autosize="{ minRows: 12, maxRows: 20 }"
            placeholder="输入要计算摘要的文本"
          />
        </div>
        <div>
          <div class="mb-2 text-sm font-medium text-slate-200">
            输出
          </div>
          <NInput
            v-model:value="output"
            type="textarea"
            :autosize="{ minRows: 12, maxRows: 20 }"
            placeholder="摘要结果会显示在这里"
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
        摘要说明
      </div>
      <div class="mt-3 rounded-xl border border-white/10 bg-black/15 px-4 py-3 text-sm leading-6 text-slate-300">
        {{ metaText }}
      </div>
    </NCard>
  </div>
</template>
