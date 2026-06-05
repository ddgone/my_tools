<script setup lang="ts">
import {
  NButton,
  NIcon,
  NInput,
  NPopconfirm,
  NSelect,
  NSwitch,
  NTabPane,
  NTabs,
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
          class="pointer-events-auto w-full max-w-2xl rounded-xl border border-white/15 bg-dracula-panel shadow-2xl"
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

          <div class="px-5 py-4">
            <NTabs
              type="line"
              animated
              :value="workspace.settings.lastSettingsTab"
              @update:value="(v: string) => workspace.settings.lastSettingsTab = v === 'export' ? 'export' : 'general'"
            >
              <NTabPane
                name="general"
                tab="通用"
              >
                <div class="settings-form pt-2">
                  <div class="settings-row">
                    <div class="settings-label">
                      最近使用显示数量
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.recentToolsCount"
                        :options="[{ label: '3', value: 3 }, { label: '5', value: 5 }, { label: '10', value: 10 }]"
                        @update:value="(v: number) => workspace.settings.recentToolsCount = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      命令历史保留数量
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.historyRetention"
                        :options="[{ label: '20', value: 20 }, { label: '50', value: 50 }, { label: '100', value: 100 }, { label: '200', value: 200 }]"
                        @update:value="(v: number) => workspace.settings.historyRetention = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      日志导出目录
                    </div>
                    <div class="settings-value">
                      <NInput
                        class="settings-control"
                        :value="workspace.settings.logExportDir"
                        placeholder="my_tools_logs"
                        @update:value="(v: string) => workspace.settings.logExportDir = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      默认 Python 解释器
                    </div>
                    <div class="settings-value">
                      <NInput
                        class="settings-control"
                        :value="workspace.settings.defaultPythonPath"
                        placeholder="python"
                        @update:value="(v: string) => workspace.settings.defaultPythonPath = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      退出前确认
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.confirmExit"
                        @update:value="(v: boolean) => workspace.settings.confirmExit = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      终端输出自动换行
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.autoWordWrap"
                        @update:value="(v: boolean) => workspace.settings.autoWordWrap = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      启动时展开所有分类
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.autoExpandAll"
                        @update:value="(v: boolean) => workspace.settings.autoExpandAll = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      快捷键提示模式
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.verboseShortcuts ? 'verbose' : 'compact'"
                        :options="[{ label: '精简模式', value: 'compact' }, { label: '详细模式', value: 'verbose' }]"
                        @update:value="(v: string | number) => workspace.settings.verboseShortcuts = v === 'verbose'"
                      />
                    </div>
                  </div>
                </div>
              </NTabPane>

              <NTabPane
                name="export"
                tab="导出"
              >
                <div class="settings-form pt-2">
                  <div class="settings-row">
                    <div class="settings-label">
                      导出成功后自动打开目录
                    </div>
                    <div class="settings-value">
                      <NSwitch
                        :value="workspace.settings.autoOpenExportDir"
                        @update:value="(v: boolean) => workspace.settings.autoOpenExportDir = v"
                      />
                    </div>
                  </div>

                  <div class="settings-row">
                    <div class="settings-label">
                      Go 工具默认导出内容
                    </div>
                    <div class="settings-value">
                      <NSelect
                        class="settings-control"
                        :value="workspace.settings.goExportMode"
                        :options="[{ label: '二进制', value: 'binary' }, { label: '源码', value: 'source' }]"
                        @update:value="(v: string | number) => workspace.settings.goExportMode = v === 'source' ? 'source' : 'binary'"
                      />
                    </div>
                  </div>
                </div>
              </NTabPane>
            </NTabs>
          </div>

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

<style scoped>
.settings-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-row {
  display: grid;
  grid-template-columns: clamp(152px, 32%, 192px) minmax(0, 1fr);
  align-items: center;
  column-gap: 16px;
}

.settings-label {
  min-width: 0;
  white-space: nowrap;
  color: rgba(248, 248, 242, 0.9);
  font-size: 14px;
  line-height: 1.4;
}

.settings-control {
  width: 100%;
  min-width: 0;
}

.settings-value {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  min-width: 0;
}

@media (max-width: 720px) {
  .settings-row {
    grid-template-columns: minmax(0, 1fr);
    row-gap: 8px;
  }
}
</style>
