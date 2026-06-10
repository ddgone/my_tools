<script setup lang="ts">
import { computed } from 'vue'
import { NDrawer, NDrawerContent, NEmpty, NProgress, NText } from 'naive-ui'
import { useDownloadStore } from '@/stores/downloads'

const downloads = useDownloadStore()

const tasks = computed(() => downloads.activeTasks)
</script>

<template>
  <NDrawer
    :show="downloads.drawerOpen"
    :width="360"
    placement="right"
    @update:show="(value) => value ? downloads.openDrawer() : downloads.closeDrawer()"
  >
    <NDrawerContent
      title="下载任务"
      closable
    >
      <div class="flex h-full flex-col gap-y-3">
        <NEmpty
          v-if="tasks.length === 0"
          description="当前没有进行中的下载任务"
        />
        <div
          v-for="task in tasks"
          :key="task.id"
          class="rounded-xl border border-white/10 bg-white/5 p-3"
        >
          <div class="flex items-center justify-between gap-x-3">
            <div class="min-w-0">
              <div class="truncate text-sm font-semibold text-dracula-text">
                {{ task.toolName || task.toolId }}
              </div>
              <NText
                depth="3"
                class="block truncate text-xs"
              >
                {{ task.remoteResultPath }}
              </NText>
            </div>
            <NText
              depth="3"
              class="shrink-0 text-xs"
            >
              {{ Math.round(task.progressPercent) }}%
            </NText>
          </div>
          <NProgress
            v-if="task.totalBytes > 0"
            class="mt-3"
            type="line"
            :percentage="Math.max(2, Math.round(task.progressPercent))"
            :show-indicator="false"
            :processing="task.status === 'running'"
          />
          <NText
            depth="3"
            class="mt-2 block text-xs"
          >
            {{ task.message || '正在下载' }}
          </NText>
        </div>
      </div>
    </NDrawerContent>
  </NDrawer>
</template>
