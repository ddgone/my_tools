<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { NButton, useMessage } from 'naive-ui'
import type { SSHConnection } from '@/types/workbench'
import {
  DeleteSSHConnection, GetSSHConnection, SaveSSHConnection,
  TestSSHConnection, TestSSHConnectionRaw, UpdateSSHConnection,
} from '../../wailsjs/go/main/App'

const message = useMessage()

const props = defineProps<{
  connectionId: string
  isNew: boolean
}>()

const emit = defineEmits<{
  close: []
  saved: []
}>()

const showPassword = ref(false)
const testing = ref(false)
const saving = ref(false)
const loading = ref(false)
const testResult = ref<{ success: boolean; message: string } | null>(null)

const form = ref<SSHConnection>({
  id: '',
  name: '',
  host: '',
  port: 22,
  user: 'root',
  authMethod: 'password',
  password: '',
  keyPath: '',
  description: '',
})

const title = computed(() => props.isNew ? '新建 SSH 连接' : `编辑 ${form.value.name || props.connectionId}`)

const canSave = computed(() =>
  form.value.name.trim() !== '' && form.value.host.trim() !== '' && form.value.user.trim() !== '',
)

const canTest = computed(() =>
  form.value.host.trim() !== '' && form.value.port > 0 && (form.value.password?.trim() ?? '') !== '',
)

async function loadConnection() {
  if (props.isNew || !props.connectionId) return
  loading.value = true
  try {
    const conn = await GetSSHConnection(props.connectionId)
    if (conn) {
      form.value = { ...conn }
    }
  } finally {
    loading.value = false
  }
}

watch(() => props.connectionId, loadConnection)
onMounted(loadConnection)

async function handleTest() {
  if (!canTest.value) return
  testing.value = true
  testResult.value = null
  try {
    let result: { success: boolean; message: string }
    if (props.isNew) {
      result = await TestSSHConnectionRaw(
        form.value.host, form.value.port, form.value.user, form.value.password || '',
      )
    } else {
      result = await TestSSHConnection(props.connectionId)
    }
    testResult.value = result
    message[result.success ? 'success' : 'error'](result.message)
  } catch (e: any) {
    testResult.value = { success: false, message: e.toString() }
    message.error(e.toString())
  } finally {
    testing.value = false
  }
}

async function handleSave() {
  if (!canSave.value) return
  saving.value = true
  try {
    if (props.isNew) {
      await SaveSSHConnection(form.value)
      message.success('连接已保存')
    } else {
      await UpdateSSHConnection(props.connectionId, form.value)
      message.success('连接已更新')
    }
    emit('saved')
  } catch (e: any) {
    message.error(e.toString())
  } finally {
    saving.value = false
  }
}

async function handleDelete() {
  try {
    await DeleteSSHConnection(props.connectionId)
    message.success('连接已删除')
    emit('close')
  } catch (e: any) {
    message.error(e.toString())
  }
}
</script>

<template>
  <div v-if="!loading" class="flex flex-1 flex-col overflow-hidden">
    <div
      class="shrink-0 overflow-y-auto border-b border-dracula-soft p-4"
    >
      <div class="mx-auto flex max-w-xl flex-wrap items-start justify-between gap-4">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-2 text-xs text-slate-500">
            <span>SSH 连接</span>
            <span class="h-1 w-1 rounded-full bg-dracula-cyan" />
            <span>{{ isNew ? '新建' : '编辑' }}</span>
          </div>
          <h2 class="m-0 mt-1 text-lg font-semibold text-white">
            {{ title }}
          </h2>
          <p
            v-if="!isNew && form.host"
            class="mt-1 text-sm text-slate-400"
          >
            {{ form.user }}@{{ form.host }}:{{ form.port }}
          </p>
        </div>

        <div class="flex shrink-0 flex-wrap items-center gap-2">
          <NButton
            type="primary"
            size="small"
            :disabled="!canSave"
            :loading="saving"
            @click="handleSave"
          >
            {{ isNew ? '保存连接' : '保存修改' }}
          </NButton>
          <NButton
            size="small"
            :disabled="!canTest"
            :loading="testing"
            @click="handleTest"
          >
            🔍 测试连接
          </NButton>
          <NButton
            v-if="!isNew"
            type="error"
            size="small"
            secondary
            @click="handleDelete"
          >
            🗑 删除
          </NButton>
        </div>
      </div>

      <div class="mx-auto mt-6 max-w-xl space-y-4">
        <div>
          <label class="mb-1 block text-xs font-medium text-slate-400">连接名称</label>
          <input
            v-model="form.name"
            type="text"
            placeholder="例如：实验室服务器"
            class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
          >
        </div>

        <div class="flex gap-3">
          <div class="flex-1">
            <label class="mb-1 block text-xs font-medium text-slate-400">主机地址</label>
            <input
              v-model="form.host"
              type="text"
              placeholder="192.168.1.100"
              class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
            >
          </div>
          <div style="width: 100px">
            <label class="mb-1 block text-xs font-medium text-slate-400">端口</label>
            <input
              v-model.number="form.port"
              type="number"
              min="1"
              max="65535"
              class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
            >
          </div>
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-slate-400">用户名</label>
          <input
            v-model="form.user"
            type="text"
            placeholder="root"
            class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
          >
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-slate-400">认证方式</label>
          <div
            class="flex max-w-xs rounded-md border border-dracula-soft bg-black/30 p-0.5"
          >
            <button
              class="flex-1 rounded px-3 py-1.5 text-xs transition"
              :class="form.authMethod === 'password'
                ? 'bg-dracula-cyan/20 text-dracula-cyan'
                : 'text-slate-500 hover:text-slate-300'"
              @click="form.authMethod = 'password'"
            >
              密码
            </button>
            <button
              class="flex-1 rounded px-3 py-1.5 text-xs transition"
              :class="form.authMethod === 'key'
                ? 'bg-dracula-cyan/20 text-dracula-cyan'
                : 'text-slate-500 hover:text-slate-300'"
              @click="form.authMethod = 'key'"
            >
              密钥
            </button>
          </div>
        </div>

        <div v-if="form.authMethod === 'password'">
          <label class="mb-1 block text-xs font-medium text-slate-400">密码</label>
          <div class="relative max-w-sm">
            <input
              v-model="form.password"
              :type="showPassword ? 'text' : 'password'"
              placeholder="输入 SSH 密码"
              class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 pr-10 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
            >
            <button
              class="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-xs text-slate-500 transition hover:text-slate-300"
              @click="showPassword = !showPassword"
            >
              {{ showPassword ? '🙈' : '👁' }}
            </button>
          </div>
        </div>

        <div v-else>
          <label class="mb-1 block text-xs font-medium text-slate-400">密钥路径</label>
          <input
            v-model="form.keyPath"
            type="text"
            placeholder="/home/user/.ssh/id_rsa"
            class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
          >
        </div>

        <div>
          <label class="mb-1 block text-xs font-medium text-slate-400">备注（可选）</label>
          <input
            v-model="form.description"
            type="text"
            placeholder="描述这个连接的用途"
            class="w-full rounded-md border border-dracula-soft bg-black/30 px-3 py-2 text-sm text-slate-200 outline-none transition focus:border-dracula-cyan/50"
          >
        </div>

        <div
          v-if="testResult"
          class="rounded-lg border p-3 text-sm"
          :class="testResult.success
            ? 'border-dracula-green/30 bg-dracula-green/5 text-dracula-green'
            : 'border-dracula-red/30 bg-dracula-red/5 text-dracula-red'"
        >
          {{ testResult.message }}
        </div>
      </div>
    </div>

    <div class="flex min-h-0 flex-1 items-center justify-center">
      <div class="text-center text-xs text-slate-600">
        <div class="mb-2 text-lg">
          🖥
        </div>
        测试连接通过后即可在工具执行时选择此服务器
      </div>
    </div>
  </div>

  <div
    v-else
    class="flex flex-1 items-center justify-center"
  >
    <span class="text-sm text-slate-500">加载中...</span>
  </div>
</template>
