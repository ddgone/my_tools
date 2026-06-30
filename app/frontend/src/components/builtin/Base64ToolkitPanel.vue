<script setup lang="ts">
import { ref } from 'vue'
import { NButton, NCard, NInput, useMessage } from 'naive-ui'

const message = useMessage()
const source = ref('')
const output = ref('')
const errorText = ref('')

function bytesToBinary(bytes: Uint8Array) {
  let binary = ''
  for (const byte of bytes) {
    binary += String.fromCharCode(byte)
  }
  return binary
}

function encodeBase64(value: string) {
  const bytes = new TextEncoder().encode(value)
  return btoa(bytesToBinary(bytes))
}

function decodeBase64(value: string) {
  const normalized = value.replace(/\s+/g, '')
  const binary = atob(normalized)
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

function handleEncode() {
  try {
    output.value = encodeBase64(source.value)
    errorText.value = ''
    message.success('编码完成')
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
    message.error(errorText.value)
  }
}

function handleDecode() {
  try {
    output.value = decodeBase64(source.value)
    errorText.value = ''
    message.success('解码完成')
  } catch (error) {
    errorText.value = error instanceof Error ? error.message : String(error)
    message.error(errorText.value)
  }
}

function handleSwap() {
  const currentSource = source.value
  source.value = output.value
  output.value = currentSource
  errorText.value = ''
}

function handleCopy() {
  if (!output.value) {
    message.warning('当前没有可复制的结果')
    return
  }
  void navigator.clipboard.writeText(output.value)
  message.success('已复制结果')
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
          @click="handleEncode"
        >
          编码为 Base64
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleDecode"
        >
          从 Base64 解码
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleSwap"
        >
          交换输入输出
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="handleCopy"
        >
          复制输出
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
            :autosize="{ minRows: 14, maxRows: 22 }"
            placeholder="输入原始文本或 Base64 字符串"
          />
        </div>
        <div>
          <div class="mb-2 text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
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
      class="bg-[rgb(var(--color-bg-panel)/0.78)]"
    >
      <div class="text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
        结果说明
      </div>
      <div
        class="mt-3 rounded-xl border px-4 py-3 text-sm leading-6"
        :class="errorText
          ? 'border-rose-400/25 bg-rose-400/10 text-rose-100'
          : 'border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] text-[rgb(var(--color-fg-secondary)/0.95)]'"
      >
        {{ errorText || '支持 Unicode 文本。编码和解码都在当前桌面宿主前端本地完成，不影响现有工具执行链路。' }}
      </div>
    </NCard>
  </div>
</template>
