<script setup lang="ts">
import { useWorkspaceStore } from '@/stores/workspace'

const workspace = useWorkspaceStore()

interface ShortcutGroup {
  title: string
  color: string
  items: { key: string; desc: string }[]
}

const groups: ShortcutGroup[] = [
  {
    title: '全局',
    color: 'text-dracula-orange',
    items: [
      { key: 'Ctrl+P', desc: '全局搜索工具' },
      { key: 'Ctrl+F', desc: '收藏/取消收藏当前工具' },
      { key: 'F1', desc: '快捷键帮助' },
    ],
  },
  {
    title: '侧边栏',
    color: 'text-dracula-yellow',
    items: [
      { key: '🔧', desc: '工具列表视图' },
      { key: '🔗', desc: 'SSH 连接管理视图' },
      { key: 'Ctrl 拖拽分隔条', desc: '调整侧边栏宽度' },
    ],
  },
  {
    title: '工作区',
    color: 'text-dracula-green',
    items: [
      { key: '▶ 本地运行', desc: '执行当前工具' },
      { key: '🔗 远程执行', desc: '选择SSH服务器执行' },
      { key: '⏹ 停止', desc: '取消运行中的任务' },
      { key: 'Ctrl 拖拽分隔条', desc: '调整上下分栏高度' },
    ],
  },
  {
    title: '命令行模式',
    color: 'text-dracula-cyan',
    items: [
      { key: '↑/↓', desc: '翻阅命令历史' },
      { key: '←/→', desc: '在历史条目间切换' },
    ],
  },
  {
    title: '终端日志',
    color: 'text-dracula-purple',
    items: [
      { key: '清空', desc: '清空当前日志' },
      { key: '复制', desc: '复制日志到剪贴板' },
      { key: '导出', desc: '导出日志为 .log 文件' },
    ],
  },
  {
    title: '设置',
    color: 'text-slate-400',
    items: [
      { key: '⚙️ 设置', desc: '打开系统首选项' },
      { key: '🔍 搜索', desc: '顶部搜索按钮或 Ctrl+P' },
    ],
  },
]
</script>

<template>
  <Teleport to="body">
    <div
      v-if="workspace.showHotkeyHelp"
      class="fixed inset-0 z-50 flex items-start justify-center bg-black/50 pt-[10vh]"
      @click="workspace.showHotkeyHelp = false"
    >
      <div
        class="w-full max-w-2xl rounded-xl border border-dracula-soft bg-dracula-panel shadow-2xl"
        @click.stop
      >
        <div class="flex items-center justify-between border-b border-dracula-soft px-5 py-3">
          <span class="text-sm font-semibold text-white">⌨️ 快捷键帮助</span>
          <button
            class="rounded px-2 py-1 text-xs text-slate-500 transition hover:bg-white/5 hover:text-slate-300"
            @click="workspace.showHotkeyHelp = false"
          >
            ESC 关闭
          </button>
        </div>
        <div class="grid grid-cols-2 gap-4 p-5">
          <div
            v-for="group in groups"
            :key="group.title"
            class="min-w-0"
          >
            <div
              class="mb-2 text-[10px] font-semibold uppercase tracking-wider"
              :class="group.color"
            >
              {{ group.title }}
            </div>
            <div class="space-y-1">
              <div
                v-for="item in group.items"
                :key="item.key"
                class="flex items-baseline gap-2 text-xs"
              >
                <code class="shrink-0 rounded bg-black/30 px-1.5 py-0.5 text-[10px] text-slate-200">{{ item.key }}</code>
                <span class="truncate text-slate-400">{{ item.desc }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
