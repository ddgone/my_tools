<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NButton,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  type SelectOption,
} from 'naive-ui'
import { useWorkspaceStore } from '@/stores/workspace'
import type { ParameterSpec, ToolManifest } from '@/types/workbench'

const props = defineProps<{
  tool: ToolManifest | null
}>()

const emit = defineEmits<{
  execute: []
  fileDialog: [param: ParameterSpec]
}>()

const workspace = useWorkspaceStore()

type ParamMode = 'form' | 'cli' | 'docs'
const activeMode = ref<ParamMode>('form')

const paramModes: { key: ParamMode; label: string }[] = [
  { key: 'form', label: '可视化表单' },
  { key: 'cli', label: '命令行模式' },
  { key: 'docs', label: '工具说明' },
]

function inputKind(param: ParameterSpec): string {
  return param.type
}

function selectOptions(param: ParameterSpec): SelectOption[] {
  return (param.options ?? []).map((o) => ({ label: o.label, value: o.value }))
}

const activeTab = computed(() => workspace.activeTab())

const canSubmit = computed(() => {
  if (!props.tool || !activeTab.value) return false
  return props.tool.params.every((param) => {
    if (!param.required) return true
    const value = activeTab.value!.formModel[param.key]
    return value !== undefined && value !== null && value !== ''
  })
})

watch(
    () => activeTab.value?.formModel,
    () => {
      if (props.tool && activeTab.value && activeMode.value === 'form') {
        workspace.updateRawArgs(props.tool, activeTab.value)
      }
    },
    { deep: true },
)

function onTextUpdate(param: ParameterSpec, value: string) {
  const tab = activeTab.value
  if (tab) tab.formModel[param.key] = value
}

function onNumberUpdate(param: ParameterSpec, value: number | null) {
  const tab = activeTab.value
  if (tab) tab.formModel[param.key] = value
}

function onBoolUpdate(param: ParameterSpec, value: boolean) {
  const tab = activeTab.value
  if (tab) tab.formModel[param.key] = value
}

function onSelectUpdate(param: ParameterSpec, value: string) {
  const tab = activeTab.value
  if (tab) tab.formModel[param.key] = value
}

function onRawArgsUpdate(value: string) {
  const tab = activeTab.value
  if (tab) tab.rawArgs = value
}

function formTextValue(param: ParameterSpec): string | null {
  const v = activeTab.value?.formModel[param.key]
  return v !== undefined && v !== null ? String(v) : null
}

function formNumberValue(param: ParameterSpec): number | null {
  const v = activeTab.value?.formModel[param.key]
  return typeof v === 'number' ? v : null
}

function formBoolValue(param: ParameterSpec): boolean {
  return activeTab.value?.formModel[param.key] === true
}

function formSelectValue(param: ParameterSpec): string | null {
  const v = activeTab.value?.formModel[param.key]
  return v !== undefined && v !== null ? String(v) : null
}

function tabRawArgs(): string {
  return activeTab.value?.rawArgs ?? ''
}
</script>

<template>
  <div
    v-if="tool"
    class="mt-4"
  >
    <div class="flex border-b border-dracula-soft">
      <button
        v-for="mode in paramModes"
        :key="mode.key"
        class="px-3 py-2 text-xs transition"
        :class="
          activeMode === mode.key
            ? 'border-b-2 border-dracula-cyan text-dracula-cyan'
            : 'border-b-2 border-transparent text-slate-500 hover:text-slate-300'
        "
        @click="activeMode = mode.key"
      >
        {{ mode.label }}
      </button>
    </div>

    <div class="mt-3">
      <div v-if="activeMode === 'form'">
        <div class="grid grid-cols-2 gap-x-6 gap-y-1">
          <div
            v-for="param in tool.params"
            :key="param.key"
            class="mb-2"
          >
            <label class="mb-1 block text-[11px] uppercase tracking-wide text-slate-500">
              {{ param.label }}
              <span
                v-if="param.required"
                class="text-dracula-red"
              >*</span>
            </label>

            <div
              v-if="inputKind(param) === 'text' || inputKind(param) === 'path'"
              class="flex gap-1"
            >
              <n-input
                :value="formTextValue(param)"
                :placeholder="param.placeholder"
                size="small"
                class="flex-1"
                @update:value="onTextUpdate(param, $event)"
              />
              <button
                v-if="inputKind(param) === 'path'"
                class="shrink-0 rounded border border-dracula-soft px-2 text-xs text-slate-400 transition hover:border-dracula-cyan/50 hover:text-slate-200"
                title="选择文件/目录"
                @click="emit('fileDialog', param)"
              >
                📂
              </button>
            </div>

            <n-input
              v-else-if="inputKind(param) === 'textarea'"
              :value="formTextValue(param)"
              type="textarea"
              :placeholder="param.placeholder"
              :autosize="{ minRows: 2, maxRows: 4 }"
              size="small"
              @update:value="onTextUpdate(param, $event)"
            />
            <n-input-number
              v-else-if="inputKind(param) === 'number'"
              :value="formNumberValue(param)"
              size="small"
              class="w-full"
              :show-button="false"
              @update:value="onNumberUpdate(param, $event)"
            />
            <n-switch
              v-else-if="inputKind(param) === 'boolean'"
              :value="formBoolValue(param)"
              @update:value="onBoolUpdate(param, $event)"
            />
            <n-select
              v-else-if="inputKind(param) === 'select'"
              :value="formSelectValue(param)"
              :options="selectOptions(param)"
              size="small"
              @update:value="onSelectUpdate(param, $event)"
            />
          </div>
        </div>
      </div>

      <div
        v-else-if="activeMode === 'cli'"
        class="space-y-3"
      >
        <n-input
          :value="tabRawArgs()"
          type="textarea"
          placeholder="直接输入命令行参数，例如 -input &quot;/path/to/data&quot; -workers 4"
          :autosize="{ minRows: 4, maxRows: 10 }"
          size="small"
          @update:value="onRawArgsUpdate"
        />
        <p class="text-[11px] text-slate-500">
          命令行模式支持直接输入 CLI 参数字符串，适合复杂 flag 组合或高级调试场景。
        </p>
      </div>

      <div v-else-if="activeMode === 'docs'">
        <div class="rounded-lg border border-dracula-soft bg-[#0d0e14] p-4">
          <div class="mb-3 flex items-center gap-2 text-xs text-slate-500">
            <span class="rounded bg-dracula-soft px-1.5 py-0.5 text-[10px] uppercase">📖 使用说明</span>
            <span
              v-if="tool.docs.usage"
              class="text-dracula-green/50"
            >已从旧版迁移</span>
          </div>
          <template v-if="tool.docs.usage">
            <pre class="m-0 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-slate-300">{{
                tool.docs.usage
            }}</pre>
          </template>
          <div
            v-else
            class="rounded-md bg-black/30 p-3"
          >
            <p class="m-0 text-sm leading-relaxed text-slate-300">
              {{ tool.docs.summary || tool.description }}
            </p>
          </div>
        </div>

        <div class="mt-4 space-y-3 rounded-lg border border-dracula-soft bg-black/10 p-4">
          <div>
            <span class="text-[10px] uppercase tracking-wide text-slate-500">参数列表</span>
            <div class="mt-2 space-y-1">
              <div
                v-for="param in tool.params"
                :key="param.key"
                class="flex items-baseline gap-2 text-xs"
              >
                <code class="shrink-0 text-dracula-cyan">{{ param.argKey || param.key }}</code>
                <span class="text-slate-400">{{ param.label }}</span>
                <span
                  v-if="param.required"
                  class="text-dracula-red"
                >*必填</span>
                <span
                  v-else
                  class="text-slate-600"
                >可选</span>
                <span class="ml-auto text-[10px] text-slate-600">{{ param.type }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="activeMode !== 'docs'"
      class="mt-4 rounded-lg border border-dracula-soft bg-black/10 p-3"
    >
      <span class="text-[10px] uppercase tracking-wider text-slate-500">CLI 参数预览</span>
      <code class="mt-1.5 block break-all text-sm leading-relaxed text-dracula-yellow">
        {{ tabRawArgs() || '(无参数)' }}
      </code>
    </div>

    <div class="mt-4">
      <n-button
        type="success"
        :disabled="!canSubmit"
        @click="emit('execute')"
      >
        ▶ 开始本地执行
      </n-button>
    </div>
  </div>
</template>
