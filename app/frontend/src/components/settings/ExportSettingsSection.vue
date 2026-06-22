<script setup lang="ts">
import { NSelect, NSwitch } from 'naive-ui'
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()
</script>

<template>
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
