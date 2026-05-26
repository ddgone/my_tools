<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NRadioGroup,
  NRadio,
  NText,
} from 'naive-ui'
import { Save, PlayCircle, Trash, Eye, EyeOff } from '@vicons/ionicons5'
import { useMessage } from 'naive-ui'
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
  <div
    v-if="!loading"
    class="flex flex-1 flex-col overflow-hidden"
  >
    <div class="shrink-0 overflow-y-auto border-b border-dracula-soft p-4">
      <div class="mx-auto flex max-w-xl flex-wrap items-start justify-between gap-4">
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-x-2">
            <NText
              depth="3"
              class="text-xs"
            >
              SSH 连接
            </NText>
            <span class="h-1 w-1 rounded-full bg-dracula-cyan" />
            <NText
              depth="3"
              class="text-xs"
            >
              {{ isNew ? '新建' : '编辑' }}
            </NText>
          </div>
          <h2 class="m-0 mt-1 text-lg font-semibold text-dracula-text">
            {{ title }}
          </h2>
          <NText
            v-if="!isNew && form.host"
            depth="2"
            class="mt-1 text-sm"
          >
            {{ form.user }}@{{ form.host }}:{{ form.port }}
          </NText>
        </div>

        <div class="flex shrink-0 flex-wrap items-center gap-x-2 gap-y-1.5">
          <NButton
            type="primary"
            size="small"
            :disabled="!canSave"
            :loading="saving"
            @click="handleSave"
          >
            <template #icon>
              <NIcon :component="Save" />
            </template>
            {{ isNew ? '保存连接' : '保存修改' }}
          </NButton>
          <NButton
            size="small"
            :disabled="!canTest"
            :loading="testing"
            @click="handleTest"
          >
            <template #icon>
              <NIcon :component="PlayCircle" />
            </template>
            测试连接
          </NButton>
          <NButton
            v-if="!isNew"
            type="error"
            size="small"
            secondary
            @click="handleDelete"
          >
            <template #icon>
              <NIcon :component="Trash" />
            </template>
            删除
          </NButton>
        </div>
      </div>

      <div class="mx-auto mt-6 max-w-xl">
        <NForm
          label-placement="top"
          label-align="left"
          size="small"
        >
          <NFormItem
            label="连接名称"
            required
          >
            <NInput
              v-model:value="form.name"
              placeholder="例如：实验室服务器"
            />
          </NFormItem>

          <div class="flex gap-x-3">
            <NFormItem
              label="主机地址"
              required
              class="flex-1"
            >
              <NInput
                v-model:value="form.host"
                placeholder="192.168.1.100"
              />
            </NFormItem>
            <NFormItem
              label="端口"
              class="w-24"
            >
              <NInputNumber
                v-model:value="form.port"
                :min="1"
                :max="65535"
              />
            </NFormItem>
          </div>

          <NFormItem
            label="用户名"
            required
          >
            <NInput
              v-model:value="form.user"
              placeholder="root"
            />
          </NFormItem>

          <NFormItem label="认证方式">
            <NRadioGroup
              v-model:value="form.authMethod"
              name="authMethod"
            >
              <NRadio value="password">
                密码
              </NRadio>
              <NRadio value="key">
                密钥
              </NRadio>
            </NRadioGroup>
          </NFormItem>

          <NFormItem
            v-if="form.authMethod === 'password'"
            label="密码"
          >
            <NInput
              v-model:value="form.password"
              :type="showPassword ? 'text' : 'password'"
              placeholder="输入 SSH 密码"
              show-password-on="click"
            >
              <template #password-visible-icon>
                <NIcon :component="EyeOff" />
              </template>
              <template #password-invisible-icon>
                <NIcon :component="Eye" />
              </template>
            </NInput>
          </NFormItem>

          <NFormItem
            v-else
            label="密钥路径"
          >
            <NInput
              v-model:value="form.keyPath"
              placeholder="/home/user/.ssh/id_rsa"
            />
          </NFormItem>

          <NFormItem label="备注">
            <NInput
              v-model:value="form.description"
              placeholder="描述这个连接的用途"
            />
          </NFormItem>
        </NForm>

        <NAlert
          v-if="testResult"
          :type="testResult.success ? 'success' : 'error'"
          class="mt-4"
        >
          {{ testResult.message }}
        </NAlert>
      </div>
    </div>

    <div class="flex min-h-0 flex-1 items-center justify-center">
      <div class="text-center">
        <NText
          depth="3"
          class="text-xs"
        >
          测试连接通过后即可在工具执行时选择此服务器
        </NText>
      </div>
    </div>
  </div>

  <div
    v-else
    class="flex flex-1 items-center justify-center"
  >
    <NText
      depth="3"
      class="text-sm"
    >
      加载中...
    </NText>
  </div>
</template>
