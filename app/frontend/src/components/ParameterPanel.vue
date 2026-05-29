<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import {
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NInputNumber,
  NSelect,
  NSwitch,
  NTabs,
  NTabPane,
  NText,
  NTag,
  NTooltip,
  type SelectOption,
} from 'naive-ui'
import { FolderOpen, Copy, HelpCircle } from '@vicons/ionicons5'
import { useWorkspaceStore } from '@/stores/workspace'
import { useMessage } from 'naive-ui'
import type { ParameterSpec, ToolManifest } from '@/types/workbench'

const props = defineProps<{
  tool: ToolManifest | null
}>()

const emit = defineEmits<{
  execute: []
  fileDialog: [param: ParameterSpec]
}>()

const workspace = useWorkspaceStore()
const message = useMessage()

const activeTab = computed(() => workspace.activeTab())

type ParamMode = 'form' | 'cli' | 'docs'
const activeMode = ref<ParamMode>('form')

watch(
  () => activeTab.value?.formModel,
  () => {
    if (props.tool && activeTab.value && activeMode.value === 'form') {
      workspace.updateRawArgs(props.tool, activeTab.value)
    }
  },
  { deep: true },
)

function selectOptions(param: ParameterSpec): SelectOption[] {
  return (param.options ?? []).map((o) => ({ label: o.label, value: o.value }))
}

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

function shouldShowHelpTooltip(param: ParameterSpec): boolean {
  return typeof param.help === 'string' && param.help.includes('\n')
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

function onRawArgsUpdate(value: string) {
  const tab = activeTab.value
  if (!tab) return
  tab.rawArgs = value
}

async function copyCli() {
  await navigator.clipboard.writeText(tabRawArgs())
  message.success('已复制到剪贴板')
}
</script>

<template>
  <div
    v-if="tool"
    class="mt-4"
  >
    <NTabs
      v-model:value="activeMode"
      type="bar"
      animated
    >
      <NTabPane
        name="form"
        tab="可视化表单"
      />
      <NTabPane
        name="cli"
        tab="命令行模式"
      />
      <NTabPane
        name="docs"
        tab="工具说明"
      />
    </NTabs>

    <div class="mt-4 rounded-lg border border-white/15 p-4">
      <div v-if="activeMode === 'form'">
        <NForm
          label-placement="top"
          label-align="left"
          size="small"
        >
          <div class="grid grid-cols-2 gap-x-6 gap-y-2">
            <NFormItem
              v-for="param in tool.params"
              :key="param.key"
            >
              <template #label>
                <div class="flex items-center gap-x-1.5">
                  <span>{{ param.label }}</span>
                  <span
                    v-if="param.required"
                    class="text-[13px] font-semibold leading-none text-red-400"
                  >
                    *
                  </span>
                  <NTooltip
                    v-if="shouldShowHelpTooltip(param)"
                    placement="top"
                    :style="{ maxWidth: '320px' }"
                  >
                    <template #trigger>
                      <NButton
                        text
                        size="tiny"
                        class="opacity-70 hover:opacity-100"
                      >
                        <template #icon>
                          <NIcon
                            :component="HelpCircle"
                            size="14"
                          />
                        </template>
                      </NButton>
                    </template>
                    <div class="whitespace-pre-wrap text-xs leading-relaxed">
                      {{ param.help }}
                    </div>
                  </NTooltip>
                </div>
              </template>
              <NInput
                v-if="param.type === 'text'"
                :value="formTextValue(param)"
                :placeholder="param.placeholder"
                @update:value="onTextUpdate(param, $event)"
              />

              <div
                v-else-if="param.type === 'path'"
                class="flex w-full items-start gap-x-2"
              >
                <NInput
                  :value="formTextValue(param)"
                  type="textarea"
                  :placeholder="param.placeholder"
                  :autosize="{ minRows: 2, maxRows: 4 }"
                  class="flex-1"
                  @update:value="onTextUpdate(param, $event)"
                />
                <NButton
                  class="shrink-0"
                  @click="emit('fileDialog', param)"
                >
                  <template #icon>
                    <NIcon :component="FolderOpen" />
                  </template>
                </NButton>
              </div>

              <NInput
                v-else-if="param.type === 'textarea'"
                :value="formTextValue(param)"
                type="textarea"
                :placeholder="param.placeholder"
                :autosize="{ minRows: 2, maxRows: 4 }"
                @update:value="onTextUpdate(param, $event)"
              />

              <NInputNumber
                v-else-if="param.type === 'number'"
                :value="formNumberValue(param)"
                :show-button="false"
                class="w-full"
                @update:value="onNumberUpdate(param, $event)"
              />

              <NSwitch
                v-else-if="param.type === 'boolean'"
                :value="formBoolValue(param)"
                @update:value="onBoolUpdate(param, $event)"
              />

              <NSelect
                v-else-if="param.type === 'select'"
                :value="formSelectValue(param)"
                :options="selectOptions(param)"
                @update:value="onSelectUpdate(param, $event)"
              />
            </NFormItem>
          </div>
        </NForm>
      </div>

      <div
        v-else-if="activeMode === 'cli'"
        class="space-y-3"
      >
        <NInput
          :value="tabRawArgs()"
          type="textarea"
          placeholder="直接输入命令行参数，例如 -input &quot;/path/to/data&quot; -workers 4"
          :autosize="{ minRows: 4, maxRows: 10 }"
          class="font-mono"
          @update:value="onRawArgsUpdate"
        />
        <NText
          depth="3"
          class="text-[11px]"
        >
          命令行模式支持直接输入 CLI 参数字符串，适合复杂 flag 组合或高级调试场景。
        </NText>
      </div>

      <div v-else-if="activeMode === 'docs'">
        <div class="rounded-lg border border-white/15 bg-dracula-panel p-4">
          <div class="mb-3 flex items-center gap-x-2">
            <NTag
              size="tiny"
              :bordered="false"
              type="info"
            >
              使用说明
            </NTag>
            <NText
              v-if="tool.docs.usage"
              depth="3"
              class="text-[10px]"
            >
              已从旧版迁移
            </NText>
          </div>
          <template v-if="tool.docs.usage">
            <pre class="m-0 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-slate-300">{{ tool.docs.usage }}</pre>
          </template>
          <div
            v-else
            class="rounded-md bg-black/20 p-3"
          >
            <NText
              depth="2"
              class="text-sm leading-relaxed"
            >
              {{ tool.docs.summary || tool.description }}
            </NText>
          </div>
        </div>

        <div class="mt-4 rounded-lg border border-white/15 bg-dracula-panel p-4">
          <NText
            depth="3"
            class="text-[10px] uppercase tracking-wide"
          >
            参数列表
          </NText>
          <div class="mt-2 space-y-1.5">
            <div
              v-for="param in tool.params"
              :key="param.key"
              class="flex items-baseline gap-x-2 text-xs"
            >
              <code class="shrink-0 text-dracula-cyan font-mono">{{ param.argKey || param.key }}</code>
              <NText depth="2">
                {{ param.label }}
              </NText>
              <NTag
                v-if="param.required"
                size="tiny"
                :bordered="false"
                type="error"
                class="shrink-0"
              >
                必填
              </NTag>
              <NTag
                v-else
                size="tiny"
                :bordered="false"
                class="shrink-0 opacity-50"
              >
                可选
              </NTag>
              <NText
                depth="3"
                class="ml-auto text-[10px]"
              >
                {{ param.type }}
              </NText>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div
      v-if="activeMode !== 'docs'"
      class="mt-4 rounded-lg border border-white/15 bg-dracula-panel p-3"
    >
      <div class="flex items-center justify-between">
        <NText
          depth="3"
          class="text-[10px] uppercase tracking-wider"
        >
          CLI 参数预览
        </NText>
        <NButton
          text
          size="tiny"
          @click="copyCli"
        >
          <template #icon>
            <NIcon
              :component="Copy"
              size="12"
            />
          </template>
        </NButton>
      </div>
      <code class="mt-1.5 block break-all font-mono text-sm leading-relaxed text-dracula-yellow">
        {{ tabRawArgs() || '(无参数)' }}
      </code>
    </div>
  </div>
</template>

<style scoped>
:deep(.n-tabs-bar) {
  transition: left 0.3s cubic-bezier(0.4, 0, 0.2, 1), max-width 0.3s cubic-bezier(0.4, 0, 0.2, 1), width 0.3s cubic-bezier(0.4, 0, 0.2, 1) !important;
}
</style>
