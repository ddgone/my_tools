<script setup lang="ts">
import { NButton, NIcon, NTooltip } from 'naive-ui'
import { Apps, ServerOutline, Star, TimeOutline, List, CloudUpload, Settings } from '@vicons/ionicons5'

export type ActivityBarView = 'tools' | 'ssh' | 'favorites' | 'recent'

const props = defineProps<{
  activeView: ActivityBarView | null
}>()

const emit = defineEmits<{
  'update:activeView': [view: ActivityBarView | null]
  'openSettings': []
}>()

function toggleView(view: ActivityBarView) {
  if (props.activeView === view) {
    emit('update:activeView', null)
  } else {
    emit('update:activeView', view)
  }
}

function openSettings() {
  emit('openSettings')
}

const topItems: { key: ActivityBarView | 'tasks' | 'export'; icon: typeof Apps; label: string }[] = [
  { key: 'tools', icon: Apps, label: '工具浏览器' },
  { key: 'ssh', icon: ServerOutline, label: 'SSH 连接管理' },
  { key: 'favorites', icon: Star, label: '收藏夹' },
  { key: 'recent', icon: TimeOutline, label: '最近使用' },
  { key: 'tasks', icon: List, label: '任务中心（开发中）' },
  { key: 'export', icon: CloudUpload, label: '导出中心（开发中）' },
]

function handleItemClick(key: ActivityBarView | 'tasks' | 'export') {
  if (key === 'tasks' || key === 'export') return
  toggleView(key)
}
</script>

<template>
  <div class="flex w-12 shrink-0 flex-col border-r border-white/15 bg-dracula-panel">
    <div class="flex flex-1 flex-col gap-y-0.5 py-2">
      <NTooltip
        v-for="item in topItems"
        :key="item.label"
        placement="right"
      >
        <template #trigger>
          <div class="relative flex justify-center">
            <div
              v-if="activeView === item.key"
              class="absolute left-0 top-1/2 h-6 w-0.5 -translate-y-1/2 rounded-r-sm bg-dracula-cyan"
            />
            <NButton
              quaternary
              size="small"
              class="h-10 w-10"
              :class="activeView === item.key ? 'text-dracula-cyan' : 'text-dracula-soft hover:text-dracula-text'"
              @click="handleItemClick(item.key)"
            >
              <template #icon>
                <NIcon
                  :component="item.icon"
                  size="20"
                />
              </template>
            </NButton>
          </div>
        </template>
        {{ item.label }}
      </NTooltip>
    </div>
    <div class="flex flex-col items-center pb-2">
      <NTooltip placement="right">
        <template #trigger>
          <NButton
            quaternary
            size="small"
            class="h-10 w-10 text-dracula-soft hover:text-dracula-text"
            @click="openSettings"
          >
            <template #icon>
              <NIcon
                :component="Settings"
                size="20"
              />
            </template>
          </NButton>
        </template>
        系统设置
      </NTooltip>
    </div>
  </div>
</template>
