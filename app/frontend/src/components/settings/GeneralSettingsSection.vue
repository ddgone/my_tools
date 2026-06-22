<script setup lang="ts">
import { NInput, NSelect, NSwitch } from 'naive-ui'
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()
</script>

<template>
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
