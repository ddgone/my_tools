<script setup lang="ts">
import { ref } from 'vue'
import {
  NButton,
  NDivider,
  NIcon,
  NInput,
  NPopconfirm,
  NSelect,
  NSwitch,
  NText,
  useMessage,
} from 'naive-ui'
import { TrashOutline } from '@vicons/ionicons5'
import { useWorkspaceStore } from '@/stores/workspace'
import { CleanBuildCache, GetCacheInfo } from '../../../wailsjs/go/main/App'
import type { cachecleanup } from '../../../wailsjs/go/models'

const workspace = useWorkspaceStore()
const message = useMessage()

const cacheInfo = ref<cachecleanup.Info | null>(null)
const loadingCacheInfo = ref(false)
const cleaningCache = ref(false)

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return (bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0) + ' ' + units[i]
}

async function refreshCacheInfo() {
  loadingCacheInfo.value = true
  try {
    cacheInfo.value = await GetCacheInfo()
  } catch {
    cacheInfo.value = null
  } finally {
    loadingCacheInfo.value = false
  }
}

async function doCleanCache(mode: string) {
  cleaningCache.value = true
  try {
    const result = await CleanBuildCache(mode)
    message.success(result.message || `已清理 ${result.removedDirs} 个缓存目录`)
    await refreshCacheInfo()
  } catch (e: any) {
    message.error(e?.toString() || '清理缓存失败')
  } finally {
    cleaningCache.value = false
  }
}

refreshCacheInfo()
</script>

<template>
  <div class="settings-form pt-2">
    <div class="settings-row">
      <div class="settings-label">
        界面主题
      </div>
      <div class="settings-value">
        <NSelect
          class="settings-control"
          :value="workspace.settings.themePreference"
          :options="[
            { label: '深色', value: 'dark' },
            { label: '浅色', value: 'light' },
            { label: '跟随系统', value: 'system' },
          ]"
          @update:value="(v: string) => workspace.settings.themePreference = v === 'light' || v === 'system' ? v : 'dark'"
        />
      </div>
    </div>

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

    <NDivider style="margin: 4px 0" />

    <div class="settings-row align-start">
      <div class="settings-label">
        构建缓存
      </div>
      <div class="settings-value">
        <div class="flex flex-col gap-y-2 w-full">
          <NText
            v-if="loadingCacheInfo"
            depth="3"
            class="text-xs"
          >
            正在计算缓存大小...
          </NText>
          <NText
            v-else-if="cacheInfo && cacheInfo.totalBytes > 0"
            depth="3"
            class="text-xs"
          >
            当前缓存 {{ formatBytes(cacheInfo.totalBytes) }}
            <template v-if="cacheInfo.orphanedDirs > 0">
              ，其中 {{ cacheInfo.orphanedDirs }} 个无用工具缓存 {{ formatBytes(cacheInfo.orphanedBytes) }}
            </template>
          </NText>
          <NText
            v-else
            depth="3"
            class="text-xs"
          >
            暂无构建缓存
          </NText>

          <div class="flex gap-x-2">
            <NPopconfirm
              v-if="cacheInfo && cacheInfo.orphanedDirs > 0"
              @positive-click="doCleanCache('orphaned')"
            >
              <template #trigger>
                <NButton
                  size="tiny"
                  secondary
                  :loading="cleaningCache"
                  type="warning"
                >
                  <template #icon>
                    <NIcon
                      :component="TrashOutline"
                      size="14"
                    />
                  </template>
                  清理无用缓存
                </NButton>
              </template>
              确认清理 {{ cacheInfo.orphanedDirs }} 个无用工具缓存（{{ formatBytes(cacheInfo.orphanedBytes) }}）？
            </NPopconfirm>
            <NButton
              v-else
              size="tiny"
              secondary
              :loading="cleaningCache"
              @click="doCleanCache('orphaned')"
            >
              <template #icon>
                <NIcon
                  :component="TrashOutline"
                  size="14"
                />
              </template>
              清理无用缓存
            </NButton>

            <NPopconfirm
              v-if="cacheInfo && cacheInfo.totalBytes > 0"
              @positive-click="doCleanCache('all')"
            >
              <template #trigger>
                <NButton
                  size="tiny"
                  secondary
                  :loading="cleaningCache"
                  type="error"
                >
                  <template #icon>
                    <NIcon
                      :component="TrashOutline"
                      size="14"
                    />
                  </template>
                  清理全部缓存
                </NButton>
              </template>
              确认清理全部构建缓存（{{ formatBytes(cacheInfo.totalBytes) }}）？下次构建工具时需要重新编译。
            </NPopconfirm>
          </div>
        </div>
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

@media (max-width: 720px) {
  .settings-row {
    grid-template-columns: minmax(0, 1fr);
    row-gap: 8px;
  }
}
</style>
