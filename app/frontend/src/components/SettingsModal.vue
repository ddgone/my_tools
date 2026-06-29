<script setup lang="ts">
import { ref } from 'vue'
import {
  NButton,
  NIcon,
  NPopconfirm,
  NTabPane,
  NTabs,
  NText,
  useMessage,
} from 'naive-ui'
import { Checkmark, Trash } from '@vicons/ionicons5'
import { useWorkspaceStore } from '@/stores/workspace'
import ExportSettingsSection from './settings/ExportSettingsSection.vue'
import GeneralSettingsSection from './settings/GeneralSettingsSection.vue'
import GoSettingsSection from './settings/GoSettingsSection.vue'
import PythonSettingsSection from './settings/PythonSettingsSection.vue'
import RustSettingsSection from './settings/RustSettingsSection.vue'

const workspace = useWorkspaceStore()
const message = useMessage()

const showDownloadPanel = ref(false)
const downloadVersion = ref('')
const downloadDirectory = ref('')
const rustDownloadVersion = ref('')
const zigDownloadVersion = ref('')
const rustDownloadDirectory = ref('')

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
        class="fixed inset-0 z-50 flex items-start justify-center px-4 py-[4vh] pointer-events-none"
      >
        <div
          class="pointer-events-auto flex w-full max-w-3xl flex-col overflow-hidden rounded-xl border border-white/15 bg-dracula-panel shadow-2xl max-h-[92vh]"
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

          <div class="min-h-0 flex-1 overflow-y-auto px-5 py-4">
            <NTabs
              type="line"
              animated
              :value="workspace.settings.lastSettingsTab"
              @update:value="(v: string) => workspace.settings.lastSettingsTab = v === 'export' || v === 'go' || v === 'rust' || v === 'python' ? v : 'general'"
            >
              <NTabPane
                name="general"
                tab="通用"
              >
                <GeneralSettingsSection />
              </NTabPane>

              <NTabPane
                name="export"
                tab="导出"
              >
                <ExportSettingsSection />
              </NTabPane>

              <NTabPane
                name="go"
                tab="Go"
              >
                <GoSettingsSection
                  v-model:show-download-panel="showDownloadPanel"
                  v-model:download-version="downloadVersion"
                  v-model:download-directory="downloadDirectory"
                />
              </NTabPane>

              <NTabPane
                name="rust"
                tab="Rust"
              >
                <RustSettingsSection
                  v-model:rust-download-version="rustDownloadVersion"
                  v-model:zig-download-version="zigDownloadVersion"
                  v-model:download-directory="rustDownloadDirectory"
                />
              </NTabPane>

              <NTabPane
                name="python"
                tab="Python"
              >
                <PythonSettingsSection />
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
  color: rgb(var(--color-fg-base) / 0.9);
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

.align-start {
  align-items: flex-start;
}

@media (max-width: 720px) {
  .settings-row {
    grid-template-columns: minmax(0, 1fr);
    row-gap: 8px;
  }
}
</style>
