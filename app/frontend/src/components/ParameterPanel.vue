<script setup lang="ts">
import { computed, watch } from 'vue'
import {
  NAlert,
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
import { Copy, DocumentTextOutline, FolderOpen, HelpCircle, SearchOutline } from '@vicons/ionicons5'
import { useWorkspaceStore } from '@/stores/workspace'
import { useMessage } from 'naive-ui'
import type { ParameterSpec, ToolManifest } from '@/types/workbench'
import { validateCliArgs } from '@/utils/cliArgs'
import type { ToolPanelMode } from '@/stores/workspace'
import { getExecutionTheme } from '@/utils/executionTheme'
import { getVisibleParams } from '@/utils/toolParams'

const props = defineProps<{
  tool: ToolManifest | null
  executionTarget: 'local' | 'remote'
}>()

const emit = defineEmits<{
  execute: []
  fileDialog: [param: ParameterSpec, target?: 'file' | 'directory' | 'fileOrDirectory']
}>()

const workspace = useWorkspaceStore()
const message = useMessage()

const activeTab = computed(() => workspace.activeTab())
const activeConfig = computed(() => workspace.activeExecutionConfig)
const executionTheme = computed(() => getExecutionTheme(props.tool?.kind, props.executionTarget))

function cliFlagLabel(param: ParameterSpec): string {
  if (param.emit === false) {
    return '(表单)'
  }
  const prefix = props.tool?.kind === 'rust' ? '--' : '-'
  return `${prefix}${param.argKey || param.key}`
}
const panelAccent = computed(() => executionTheme.value.accent)
const panelAccentSoftBg = computed(() => executionTheme.value.accentSoftBg)
const panelAccentSoftBorder = computed(() => executionTheme.value.accentSoftBorder)
const panelRailActive = computed(() => executionTheme.value.railActive)

type ParamMode = ToolPanelMode
const activeMode = computed<ParamMode>({
  get: () => {
    const mode = activeConfig.value?.panelMode
    if (mode === 'remote' && props.executionTarget !== 'remote') {
      return 'form'
    }
    return mode ?? 'form'
  },
  set: (value) => {
    if (workspace.activeTabIndex >= 0) {
      workspace.setPanelMode(workspace.activeTabIndex, value)
    }
  },
})

watch(
  () => activeConfig.value?.formModel,
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

const visibleParams = computed(() => {
  if (!props.tool) {
    return []
  }
  return getVisibleParams(props.tool, activeConfig.value?.formModel ?? {})
})

const groupedVisibleParams = computed(() => {
  const groups: Array<{ name: string; params: ParameterSpec[] }> = []
  const seen = new Map<string, { name: string; params: ParameterSpec[] }>()

  for (const param of visibleParams.value) {
    const groupName = param.group?.trim() || ''
    const existing = seen.get(groupName)
    if (existing) {
      existing.params.push(param)
      continue
    }

    const group = { name: groupName, params: [param] }
    seen.set(groupName, group)
    groups.push(group)
  }

  return groups
})

function onTextUpdate(param: ParameterSpec, value: string) {
  const config = activeConfig.value
  if (config) config.formModel[param.key] = value
}

function onNumberUpdate(param: ParameterSpec, value: number | null) {
  const config = activeConfig.value
  if (config) config.formModel[param.key] = value
}

function onBoolUpdate(param: ParameterSpec, value: boolean) {
  const config = activeConfig.value
  if (config) config.formModel[param.key] = value
}

function onSelectUpdate(param: ParameterSpec, value: string) {
  const config = activeConfig.value
  if (config) config.formModel[param.key] = value
}

function shouldShowHelpTooltip(param: ParameterSpec): boolean {
  return typeof param.help === 'string' && param.help.trim().length > 0
}

function formTextValue(param: ParameterSpec): string | null {
  const v = activeConfig.value?.formModel[param.key]
  return v !== undefined && v !== null ? String(v) : null
}

function formNumberValue(param: ParameterSpec): number | null {
  const v = activeConfig.value?.formModel[param.key]
  return typeof v === 'number' ? v : null
}

function formBoolValue(param: ParameterSpec): boolean {
  return activeConfig.value?.formModel[param.key] === true
}

function formSelectValue(param: ParameterSpec): string | null {
  const v = activeConfig.value?.formModel[param.key]
  return v !== undefined && v !== null ? String(v) : null
}

function tabRawArgs(): string {
  return activeConfig.value?.rawArgs ?? ''
}

function pathDialogButtons(param: ParameterSpec): Array<{
  key: string
  label: string
  target: 'file' | 'directory' | 'fileOrDirectory'
  icon: typeof FolderOpen
}> {
  const fileLabel = param.repeatable ? '添加文件' : '选择文件'
  const directoryLabel = param.repeatable ? '添加目录' : '选择目录'
  switch (param.pathMode) {
    case 'file':
      return [{ key: 'file', label: fileLabel, target: 'file', icon: DocumentTextOutline }]
    case 'fileOrDirectory':
      if (props.executionTarget === 'remote') {
        return [{ key: 'fileOrDirectory', label: '浏览路径', target: 'fileOrDirectory', icon: SearchOutline }]
      }
      return [
        { key: 'file', label: fileLabel, target: 'file', icon: DocumentTextOutline },
        { key: 'directory', label: directoryLabel, target: 'directory', icon: FolderOpen },
      ]
    default:
      return [{ key: 'directory', label: directoryLabel, target: 'directory', icon: FolderOpen }]
  }
}
const switchThemeOverrides = computed(() => ({
  railColorActive: executionTheme.value.railActive,
  buttonColor: 'rgb(var(--color-fg-base) / 1)',
}))

const tabsKey = computed(() => `${activeTab.value?.tabId ?? 'none'}:${props.executionTarget}`)

const cliArgsError = computed(() => {
  if (activeMode.value !== 'cli') {
    return null
  }
  return validateCliArgs(tabRawArgs())
})

function onRawArgsUpdate(value: string) {
  const tab = activeTab.value
  const config = activeConfig.value
  if (!tab || !config) return
  config.rawArgs = value
}

async function copyCli() {
  await navigator.clipboard.writeText(tabRawArgs())
  message.success('已复制到剪贴板')
}
</script>

<template>
  <div
    v-if="tool"
    class="mt-4 parameter-panel"
  >
    <NTabs
      :key="tabsKey"
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
      <NTabPane
        v-if="props.executionTarget === 'remote'"
        name="remote"
        tab="远程配置"
      />
    </NTabs>

    <div class="mt-4 rounded-lg border border-white/15 p-4">
      <div v-if="activeMode === 'form'">
        <NForm
          label-placement="top"
          label-align="left"
          size="small"
        >
          <div class="space-y-5">
            <section
              v-for="group in groupedVisibleParams"
              :key="group.name || '__default__'"
            >
              <div
                v-if="group.name"
                class="mb-2 text-[11px] uppercase tracking-wider text-dracula-soft"
              >
                {{ group.name }}
              </div>
              <div class="grid grid-cols-2 gap-x-6 gap-y-2">
                <NFormItem
                  v-for="param in group.params"
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
                          <span
                            class="help-trigger"
                            tabindex="0"
                          >
                            <NIcon
                              :component="HelpCircle"
                              size="14"
                            />
                          </span>
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
                    <div class="flex shrink-0 flex-col gap-2">
                      <NButton
                        v-for="button in pathDialogButtons(param)"
                        :key="button.key"
                        size="small"
                        class="shrink-0"
                        :title="button.label"
                        :aria-label="button.label"
                        @click="emit('fileDialog', param, button.target)"
                      >
                        <template #icon>
                          <NIcon :component="button.icon" />
                        </template>
                      </NButton>
                    </div>
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
                    :theme-overrides="switchThemeOverrides"
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
            </section>
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
          :status="cliArgsError ? 'error' : undefined"
          @update:value="onRawArgsUpdate"
        />
        <NAlert
          v-if="cliArgsError"
          type="error"
          :show-icon="false"
          size="small"
        >
          {{ cliArgsError }}
        </NAlert>
        <NText
          depth="3"
          class="text-[11px]"
        >
          命令行模式支持直接输入 CLI 参数字符串；空格请用引号包裹，值内双引号请写成 <code>\"</code>，未闭合引号会在执行时报参数解析错误。
        </NText>
      </div>

      <div v-else-if="activeMode === 'docs'">
        <div class="rounded-lg border border-white/15 bg-dracula-panel p-4">
          <div class="mb-3 flex items-center gap-x-2">
            <NTag
              size="tiny"
              :bordered="false"
              class="parameter-panel-accent-tag"
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
            <pre class="m-0 whitespace-pre-wrap break-words font-mono text-xs leading-relaxed text-[rgb(var(--color-fg-secondary)/0.95)]">{{ tool.docs.usage }}</pre>
          </template>
          <div
            v-else
            class="rounded-md bg-[rgb(var(--color-bg-elevated)/0.9)] p-3"
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
          <div class="mt-2 space-y-3">
            <section
              v-for="group in groupedVisibleParams"
              :key="`docs-${group.name || '__default__'}`"
            >
              <NText
                v-if="group.name"
                depth="3"
                class="text-[10px] uppercase tracking-wider"
              >
                {{ group.name }}
              </NText>
              <div class="mt-1.5 space-y-1.5">
                <div
                  v-for="param in group.params"
                  :key="param.key"
                  class="flex items-baseline gap-x-2 text-xs"
                >
                  <code
                    class="parameter-panel-accent shrink-0 font-mono"
                  >
                    {{ cliFlagLabel(param) }}
                  </code>
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
            </section>
          </div>
        </div>
      </div>

      <div v-else-if="activeMode === 'remote'">
        <div class="rounded-lg border border-white/15 bg-dracula-panel p-4">
          <div class="mb-3 flex items-center gap-x-2">
            <NTag
              size="tiny"
              :bordered="false"
              class="parameter-panel-accent-tag"
            >
              远程配置
            </NTag>
          </div>
          <NText
            depth="2"
            class="text-sm leading-relaxed"
          >
            远程权限策略、工作目录、运行时约束等配置稍后会接到这里；当前先保留独立标签页和记忆槽位。
          </NText>
        </div>
      </div>
    </div>

    <div
      v-if="activeMode !== 'docs' && activeMode !== 'remote'"
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
      <code class="parameter-panel-accent mt-1.5 block break-all font-mono text-sm leading-relaxed">
        {{ tabRawArgs() || '(无参数)' }}
      </code>
    </div>
  </div>
</template>

<style scoped>
:deep(.n-tabs-bar) {
  transition:
    left 0.16s var(--ease-out-soft),
    max-width 0.16s var(--ease-out-soft),
    width 0.16s var(--ease-out-soft),
    background-color 0.16s var(--ease-out-soft) !important;
}

.parameter-panel :deep(.n-tabs-tab__label) {
  transition: color 0.16s var(--ease-out-soft);
}

.help-trigger {
  display: inline-flex;
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  align-items: center;
  justify-content: center;
  color: rgb(var(--color-fg-muted) / 0.9);
  cursor: pointer;
  transition:
    color 0.18s cubic-bezier(0.22, 1, 0.36, 1),
    opacity 0.18s cubic-bezier(0.22, 1, 0.36, 1);
}

.help-trigger:hover,
.help-trigger:focus-visible {
  color: v-bind(panelAccent);
  opacity: 1;
}

.parameter-panel-accent {
  color: v-bind(panelAccent);
}

.parameter-panel-accent-tag {
  color: v-bind(panelAccent) !important;
  background-color: v-bind(panelAccentSoftBg) !important;
  border: 1px solid v-bind(panelAccentSoftBorder) !important;
}

.parameter-panel :deep(.n-switch.n-switch--active .n-switch__rail) {
  background-color: v-bind(panelRailActive);
}

.parameter-panel :deep(.n-switch .n-switch__button) {
  background-color: rgb(var(--color-fg-base) / 1);
}

.parameter-panel :deep(.n-tabs-tab.n-tabs-tab--active .n-tabs-tab__label),
.parameter-panel :deep(.n-tabs-tab:hover .n-tabs-tab__label) {
  color: v-bind(panelAccent);
}

.parameter-panel :deep(.n-tabs-bar) {
  background-color: v-bind(panelAccent) !important;
}
</style>
