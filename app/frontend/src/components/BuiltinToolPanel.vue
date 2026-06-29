<script setup lang="ts">
import { computed } from 'vue'
import { NIcon, NTag } from 'naive-ui'
import { getBuiltinToolById, getBuiltinToolIcon } from '@/builtin/registry'
import TimeToolkitPanel from '@/components/builtin/TimeToolkitPanel.vue'
import JsonToolkitPanel from '@/components/builtin/JsonToolkitPanel.vue'
import Base64ToolkitPanel from '@/components/builtin/Base64ToolkitPanel.vue'
import UrlToolkitPanel from '@/components/builtin/UrlToolkitPanel.vue'
import HashToolkitPanel from '@/components/builtin/HashToolkitPanel.vue'
import JwtToolkitPanel from '@/components/builtin/JwtToolkitPanel.vue'

const props = defineProps<{
  builtinToolId: string
}>()

const builtinTool = computed(() => getBuiltinToolById(props.builtinToolId))

const activePanel = computed(() => {
  switch (props.builtinToolId) {
    case 'time_toolkit':
      return TimeToolkitPanel
    case 'json_toolkit':
      return JsonToolkitPanel
    case 'base64_toolkit':
      return Base64ToolkitPanel
    case 'url_toolkit':
      return UrlToolkitPanel
    case 'hash_toolkit':
      return HashToolkitPanel
    case 'jwt_toolkit':
      return JwtToolkitPanel
    default:
      return null
  }
})
</script>

<template>
  <div
    v-if="builtinTool && activePanel"
    class="flex h-full flex-col overflow-hidden"
  >
    <div class="border-b border-white/10 px-6 py-5">
      <div class="flex items-start gap-4">
        <div
          class="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border"
          :style="{
            color: builtinTool.accent,
            borderColor: `${builtinTool.accent}55`,
            backgroundColor: `${builtinTool.accent}14`,
          }"
        >
          <NIcon
            :component="getBuiltinToolIcon(builtinTool.id)"
            size="22"
          />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-3">
            <h2 class="text-lg font-semibold text-dracula-text">
              {{ builtinTool.name }}
            </h2>
            <NTag
              size="small"
              :bordered="false"
              :style="{
                color: builtinTool.accent,
                backgroundColor: `${builtinTool.accent}14`,
              }"
            >
              {{ builtinTool.badge }}
            </NTag>
          </div>
          <p class="mt-2 text-sm leading-6 text-dracula-soft">
            {{ builtinTool.description }}
          </p>
        </div>
      </div>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto p-6">
      <component :is="activePanel" />
    </div>
  </div>

  <div
    v-else
    class="flex h-full items-center justify-center text-dracula-soft"
  >
    未找到对应的内置工具
  </div>
</template>
