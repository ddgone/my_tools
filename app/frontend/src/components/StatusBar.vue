<script setup lang="ts">
import { NIcon, NText } from 'naive-ui'
import { CheckmarkCircle } from '@vicons/ionicons5'
import { useExecutionStore } from '@/stores/execution'
import { computed } from 'vue'

const execution = useExecutionStore()

const activeTaskCount = computed(() => execution.tasks.filter((t) => t.status === 'running').length)
</script>

<template>
  <footer class="flex h-7 shrink-0 items-center justify-between border-t border-white/15 bg-dracula-panel/50 px-3 backdrop-blur-sm">
    <div class="flex items-center gap-x-1.5">
      <NIcon
        :component="CheckmarkCircle"
        size="12"
        color="#50fa7b"
      />
      <NText
        depth="3"
        class="text-[11px]"
      >
        就绪
      </NText>
    </div>
    <div class="flex items-center gap-x-1">
      <NText
        depth="3"
        class="text-[11px]"
      >
        <template v-if="activeTaskCount > 0">
          {{ activeTaskCount }} 个任务运行中
        </template>
        <template v-else>
          无活跃任务
        </template>
      </NText>
    </div>
    <div class="flex items-center gap-x-3">
      <NText
        depth="3"
        class="text-[10px]"
      >
        Go 1.22
      </NText>
      <NText
        depth="3"
        class="text-[10px]"
      >
        Python 3.11
      </NText>
      <NText
        depth="3"
        class="text-[10px]"
      >
        v1.0.0
      </NText>
    </div>
  </footer>
</template>
