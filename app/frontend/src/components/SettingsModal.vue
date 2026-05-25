<script setup lang="ts">
import { NButton, NInput, NSelect } from 'naive-ui'
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()

const countOptions = [
  { label: '3', value: 3 },
  { label: '5', value: 5 },
  { label: '10', value: 10 },
]

const historyOptions = [
  { label: '20', value: 20 },
  { label: '50', value: 50 },
  { label: '100', value: 100 },
  { label: '200', value: 200 },
]

const boolOptions = [
  { label: '关闭', value: 'false' },
  { label: '开启', value: 'true' },
]

function toBool(v: string): boolean {
  return v === 'true'
}

function fromBool(b: boolean): string {
  return b ? 'true' : 'false'
}

function resetAll() {
  if (confirm('确定要清除所有数据并恢复出厂设置吗？此操作不可撤销。')) {
    workspace.resetAllData()
    workspace.showSettings = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="workspace.showSettings"
      class="fixed inset-0 z-50 flex items-start justify-center bg-black/50 pt-[8vh]"
      @click="workspace.showSettings = false"
    >
      <div
        class="w-full max-w-md rounded-xl border border-dracula-soft bg-dracula-panel shadow-2xl"
        @click.stop
      >
        <div class="flex items-center justify-between border-b border-dracula-soft px-5 py-3">
          <span class="text-sm font-semibold text-white">⚙️ 系统首选项</span>
          <button
            class="rounded px-2 py-1 text-xs text-slate-500 transition hover:bg-white/5 hover:text-slate-300"
            @click="workspace.showSettings = false"
          >
            ESC 关闭
          </button>
        </div>

        <div class="space-y-4 p-5">
          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">最近使用显示数量</label>
            <n-select
              :value="workspace.settings.recentToolsCount"
              :options="countOptions"
              size="small"
              @update:value="(v) => workspace.settings.recentToolsCount = v"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">命令历史保留数量</label>
            <n-select
              :value="workspace.settings.historyRetention"
              :options="historyOptions"
              size="small"
              @update:value="(v) => workspace.settings.historyRetention = v"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">日志导出目录</label>
            <n-input
              :value="workspace.settings.logExportDir"
              size="small"
              placeholder="my_tools_logs"
              @update:value="(v) => workspace.settings.logExportDir = v"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">默认 Python 解释器</label>
            <n-input
              :value="workspace.settings.defaultPythonPath"
              size="small"
              placeholder="python"
              @update:value="(v) => workspace.settings.defaultPythonPath = v"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">退出前确认</label>
            <n-select
              :value="fromBool(workspace.settings.confirmExit)"
              :options="boolOptions"
              size="small"
              @update:value="(v) => workspace.settings.confirmExit = toBool(v)"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">终端输出自动换行</label>
            <n-select
              :value="fromBool(workspace.settings.autoWordWrap)"
              :options="boolOptions"
              size="small"
              @update:value="(v) => workspace.settings.autoWordWrap = toBool(v)"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">启动时展开所有分类</label>
            <n-select
              :value="fromBool(workspace.settings.autoExpandAll)"
              :options="boolOptions"
              size="small"
              @update:value="(v) => workspace.settings.autoExpandAll = toBool(v)"
            />
          </div>

          <div>
            <label class="mb-1.5 block text-[11px] uppercase tracking-wide text-slate-500">快捷键提示模式</label>
            <n-select
              :value="fromBool(workspace.settings.verboseShortcuts)"
              :options="[{ label: '精简模式', value: 'false' }, { label: '详细模式', value: 'true' }]"
              size="small"
              @update:value="(v) => workspace.settings.verboseShortcuts = toBool(v)"
            />
          </div>
        </div>

        <div class="flex items-center justify-between border-t border-dracula-soft px-5 py-3">
          <button
            class="rounded px-2.5 py-1 text-xs text-dracula-red transition hover:bg-dracula-red/10"
            @click="resetAll"
          >
            🗑 初始化应用
          </button>
          <n-button
            type="primary"
            size="small"
            @click="workspace.showSettings = false"
          >
            完成
          </n-button>
        </div>
      </div>
    </div>
  </Teleport>
</template>
