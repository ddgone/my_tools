<script setup lang="ts">
import { NButton, NText, NTag } from 'naive-ui'
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
    title: '工作区',
    color: 'text-dracula-green',
    items: [
      { key: '本地运行', desc: '执行当前工具' },
      { key: '远程执行', desc: '选择SSH服务器执行' },
      { key: '停止', desc: '取消运行中的任务' },
      { key: '拖拽分隔条', desc: '调整上下分栏高度' },
    ],
  },
  {
    title: '命令行模式',
    color: 'text-dracula-cyan',
    items: [
      { key: '↑/↓', desc: '翻阅命令历史' },
    ],
  },
  {
    title: '终端日志',
    color: 'text-dracula-pink',
    items: [
      { key: '清空', desc: '清空当前日志' },
      { key: '复制', desc: '复制日志到剪贴板' },
      { key: '导出', desc: '导出日志为 .log 文件' },
    ],
  },
]
</script>

<template>
  <Teleport to="body">
    <Transition name="fade">
      <div
        v-if="workspace.showHotkeyHelp"
        class="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm"
        @click="workspace.showHotkeyHelp = false"
      />
    </Transition>
    <Transition
      name="fade-scale"
      appear
    >
      <div
        v-if="workspace.showHotkeyHelp"
        class="fixed inset-0 z-50 flex items-start justify-center pt-[10vh] pointer-events-none"
      >
        <div
          class="pointer-events-auto w-full max-w-xl rounded-xl border border-white/15 bg-dracula-panel shadow-2xl"
          @click.stop
        >
          <div class="flex items-center justify-between border-b border-white/15 px-5 py-3">
            <NText class="text-sm font-semibold">
              快捷键帮助
            </NText>
            <NButton
              text
              size="tiny"
              @click="workspace.showHotkeyHelp = false"
            >
              ESC 关闭
            </NButton>
          </div>

          <div class="grid grid-cols-2 gap-4 p-5">
            <div
              v-for="group in groups"
              :key="group.title"
              class="min-w-0"
            >
              <NText
                :class="group.color"
                class="mb-2 block text-[11px] font-semibold uppercase tracking-wider"
              >
                {{ group.title }}
              </NText>
              <div class="space-y-1.5">
                <div
                  v-for="item in group.items"
                  :key="item.key"
                  class="flex items-baseline gap-x-2"
                >
                  <NTag
                    size="tiny"
                    :bordered="false"
                    class="shrink-0 font-mono text-[10px]"
                  >
                    {{ item.key }}
                  </NTag>
                  <NText
                    depth="3"
                    class="truncate text-xs"
                  >
                    {{ item.desc }}
                  </NText>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>
