<script setup lang="ts">
import {
  NButton,
  NForm,
  NFormItem,
  NIcon,
  NInput,
  NPopconfirm,
  NSelect,
  NSwitch,
  NText,
  useMessage,
} from 'naive-ui'
import { Trash, Checkmark } from '@vicons/ionicons5'
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()
const message = useMessage()

function resetAll() {
  workspace.resetAllData()
  workspace.showSettings = false
  message.success('已恢复出厂设置')
}
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="workspace.showSettings"
        class="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
        @click="workspace.showSettings = false"
      />
    </Transition>
    <Transition
      name="fade-scale"
      appear
    >
      <div
        v-if="workspace.showSettings"
        class="fixed inset-0 z-50 flex items-start justify-center pt-[8vh] pointer-events-none"
      >
        <div
          class="pointer-events-auto w-full max-w-md rounded-xl border border-white/15 bg-dracula-panel shadow-2xl"
          @click.stop
        >
          <div class="flex items-center justify-between border-b border-white/15 px-5 py-3">
            <NText class="text-sm font-semibold">
              系统首选项
            </NText>
            <NButton
              text
              size="tiny"
              @click="workspace.showSettings = false"
            >
              ESC 关闭
            </NButton>
          </div>

          <NForm
            label-placement="left"
            label-align="left"
            label-width="160"
            size="small"
            class="px-5 py-4"
          >
            <NFormItem label="最近使用显示数量">
              <NSelect
                :value="workspace.settings.recentToolsCount"
                :options="[{ label: '3', value: 3 }, { label: '5', value: 5 }, { label: '10', value: 10 }]"
                @update:value="(v: number) => workspace.settings.recentToolsCount = v"
              />
            </NFormItem>

            <NFormItem label="命令历史保留数量">
              <NSelect
                :value="workspace.settings.historyRetention"
                :options="[{ label: '20', value: 20 }, { label: '50', value: 50 }, { label: '100', value: 100 }, { label: '200', value: 200 }]"
                @update:value="(v: number) => workspace.settings.historyRetention = v"
              />
            </NFormItem>

            <NFormItem label="日志导出目录">
              <NInput
                :value="workspace.settings.logExportDir"
                placeholder="my_tools_logs"
                @update:value="(v: string) => workspace.settings.logExportDir = v"
              />
            </NFormItem>

            <NFormItem label="默认 Python 解释器">
              <NInput
                :value="workspace.settings.defaultPythonPath"
                placeholder="python"
                @update:value="(v: string) => workspace.settings.defaultPythonPath = v"
              />
            </NFormItem>

            <NFormItem label="退出前确认">
              <NSwitch
                :value="workspace.settings.confirmExit"
                @update:value="(v: boolean) => workspace.settings.confirmExit = v"
              />
            </NFormItem>

            <NFormItem label="终端输出自动换行">
              <NSwitch
                :value="workspace.settings.autoWordWrap"
                @update:value="(v: boolean) => workspace.settings.autoWordWrap = v"
              />
            </NFormItem>

            <NFormItem label="启动时展开所有分类">
              <NSwitch
                :value="workspace.settings.autoExpandAll"
                @update:value="(v: boolean) => workspace.settings.autoExpandAll = v"
              />
            </NFormItem>

            <NFormItem label="快捷键提示模式">
              <NSelect
                :value="workspace.settings.verboseShortcuts ? 'verbose' : 'compact'"
                :options="[{ label: '精简模式', value: 'compact' }, { label: '详细模式', value: 'verbose' }]"
                @update:value="(v: string | number) => workspace.settings.verboseShortcuts = v === 'verbose'"
              />
            </NFormItem>
          </NForm>

          <div class="flex items-center justify-between border-t border-white/15 px-5 py-3">
            <NPopconfirm @positive-click="resetAll">
              <template #trigger>
                <NButton
                  type="error"
                  size="small"
                  secondary
                >
                  <template #icon>
                    <NIcon :component="Trash" />
                  </template>
                  初始化应用
                </NButton>
              </template>
              确定要清除所有数据并恢复出厂设置吗？此操作不可撤销。
            </NPopconfirm>
            <NButton
              type="primary"
              size="small"
              @click="workspace.showSettings = false"
            >
              <template #icon>
                <NIcon :component="Checkmark" />
              </template>
              完成
            </NButton>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
