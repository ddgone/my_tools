<script setup lang="ts">
import { computed, ref } from 'vue'
import { NButton, NCard, NInput, useMessage } from 'naive-ui'

const message = useMessage()
const token = ref('')
const decodedHeader = ref('')
const decodedPayload = ref('')

function decodeBase64Url(value: string) {
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/')
  const padded = normalized + '='.repeat((4 - normalized.length % 4) % 4)
  const binary = atob(padded)
  const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0))
  return new TextDecoder().decode(bytes)
}

function formatUnixTime(value: unknown) {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return ''
  }
  return new Date(value * 1000).toISOString()
}

function decodeJwtParts() {
  const parts = token.value.trim().split('.')
  if (parts.length < 2) {
    throw new Error('JWT 至少需要包含 header.payload 两段')
  }

  const headerText = decodeBase64Url(parts[0])
  const payloadText = decodeBase64Url(parts[1])
  const header = JSON.parse(headerText)
  const payload = JSON.parse(payloadText)

  return {
    header,
    payload: payload as Record<string, unknown>,
    headerText: JSON.stringify(header, null, 2),
    payloadText: JSON.stringify(payload, null, 2),
  }
}

function parseJwt() {
  const decoded = decodeJwtParts()
  decodedHeader.value = decoded.headerText
  decodedPayload.value = decoded.payloadText
  return decoded.payload
}

function handleParse() {
  try {
    parseJwt()
    message.success('JWT 解析完成')
  } catch (error) {
    decodedHeader.value = ''
    decodedPayload.value = ''
    message.error(error instanceof Error ? error.message : String(error))
  }
}

function copyPayload() {
  if (!decodedPayload.value) {
    message.warning('当前没有可复制的 Payload')
    return
  }
  void navigator.clipboard.writeText(decodedPayload.value)
  message.success('已复制 Payload')
}

const tokenSummary = computed(() => {
  try {
    const payload = decodeJwtParts().payload
    const summary: string[] = []
    if (payload.iss) summary.push(`iss: ${String(payload.iss)}`)
    if (payload.sub) summary.push(`sub: ${String(payload.sub)}`)
    if (payload.aud) summary.push(`aud: ${JSON.stringify(payload.aud)}`)
    if (payload.exp) summary.push(`exp: ${formatUnixTime(payload.exp)}`)
    if (payload.iat) summary.push(`iat: ${formatUnixTime(payload.iat)}`)
    return summary.length > 0 ? summary.join(' | ') : 'JWT 已解析，但未发现常见时间或主体字段。'
  } catch {
    return '仅做结构解析，不校验签名。'
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
          @click="handleParse"
        >
          解析 JWT
        </NButton>
        <NButton
          size="small"
          secondary
          type="primary"
          @click="copyPayload"
        >
          复制 Payload
        </NButton>
      </div>

      <div class="mt-4">
        <div class="mb-2 text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
          Token
        </div>
        <NInput
          v-model:value="token"
          type="textarea"
          :autosize="{ minRows: 6, maxRows: 10 }"
          placeholder="输入 JWT token"
        />
      </div>

      <div class="mt-4 grid gap-4 xl:grid-cols-2">
        <div>
          <div class="mb-2 text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            Header
          </div>
          <NInput
            v-model:value="decodedHeader"
            type="textarea"
            :autosize="{ minRows: 14, maxRows: 22 }"
            placeholder="解析后的 Header 会显示在这里"
          />
        </div>
        <div>
          <div class="mb-2 text-sm font-medium text-[rgb(var(--color-fg-base)/0.98)]">
            Payload
          </div>
          <NInput
            v-model:value="decodedPayload"
            type="textarea"
            :autosize="{ minRows: 14, maxRows: 22 }"
            placeholder="解析后的 Payload 会显示在这里"
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
        快速摘要
      </div>
      <div class="mt-3 rounded-xl border border-[rgb(var(--color-border-subtle)/0.82)] bg-[rgb(var(--color-bg-elevated)/0.82)] px-4 py-3 text-sm leading-6 text-[rgb(var(--color-fg-secondary)/0.95)]">
        {{ tokenSummary }}
      </div>
    </NCard>
  </div>
</template>
