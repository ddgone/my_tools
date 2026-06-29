<script setup lang="ts">
import { computed, ref } from 'vue'
import { NCard, NIcon, NInput, NTag } from 'naive-ui'
import { Search } from '@vicons/ionicons5'
import { matchBuiltinTools, getBuiltinToolIcon } from '@/builtin/registry'
import { useWorkspaceStore } from '@/stores/workspace'
import type { BuiltinToolDefinition } from '@/types/builtin'

const workspace = useWorkspaceStore()
const searchQuery = ref('')

const filteredTools = computed(() => matchBuiltinTools(searchQuery.value))

function openBuiltinTool(tool: BuiltinToolDefinition) {
  workspace.openBuiltinTool(tool)
}

function isBuiltinToolActive(toolId: string) {
  return workspace.activeTabType === 'builtin' && workspace.activeBuiltinTab?.builtinToolId === toolId
}

function builtinCardStyle(tool: BuiltinToolDefinition) {
  if (!isBuiltinToolActive(tool.id)) {
    return {
      borderColor: 'rgb(var(--color-border-subtle) / 0.82)',
    }
  }

  return {
    borderColor: tool.accent,
    backgroundColor: `${tool.accent}12`,
    boxShadow: `0 0 0 1px ${tool.accent}33 inset`,
  }
}
</script>

<template>
  <div class="p-3">
    <NInput
      v-model:value="searchQuery"
      placeholder="搜索内置工具..."
      clearable
      size="small"
    >
      <template #prefix>
        <NIcon :component="Search" />
      </template>
    </NInput>

    <div class="mt-3 space-y-2">
      <NCard
        v-for="tool in filteredTools"
        :key="tool.id"
        size="small"
        :bordered="true"
        hoverable
        class="ui-surface-hover cursor-pointer"
        :content-style="{ padding: '12px' }"
        :style="builtinCardStyle(tool)"
        @click="openBuiltinTool(tool)"
      >
        <div class="flex items-start gap-3">
          <div
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl border"
            :style="{
              color: tool.accent,
              borderColor: `${tool.accent}55`,
              backgroundColor: `${tool.accent}12`,
            }"
          >
            <NIcon
              :component="getBuiltinToolIcon(tool.id)"
              size="20"
            />
          </div>
          <div class="min-w-0 flex-1">
            <div class="flex items-center justify-between gap-2">
              <div class="truncate text-sm font-medium text-dracula-text">
                {{ tool.name }}
              </div>
              <NTag
                size="small"
                :bordered="false"
                :style="{
                  color: tool.accent,
                  backgroundColor: `${tool.accent}14`,
                }"
              >
                {{ tool.badge }}
              </NTag>
            </div>
            <div class="mt-1 text-xs text-dracula-soft">
              {{ tool.group }}
            </div>
            <div class="mt-2 text-[12px] leading-5 text-[rgb(var(--color-fg-secondary)/0.9)]">
              {{ tool.description }}
            </div>
          </div>
        </div>
      </NCard>

      <div
        v-if="filteredTools.length === 0"
        class="rounded-xl border border-dashed border-[rgb(var(--color-border-subtle)/0.82)] px-4 py-8 text-center text-sm text-dracula-soft"
      >
        没有找到匹配的内置工具
      </div>
    </div>
  </div>
</template>
