<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NEmpty,
  NScrollbar,
  NSkeleton,
  NSpace,
  NTag,
} from 'naive-ui'

import { useExecutionStore } from '@/stores/execution'
import { useWorkbenchStore } from '@/stores/workbench'
import type { ToolManifest } from '@/types/workbench'

const router = useRouter()
const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const selectedToolID = ref('')

const groupedTools = computed(() => {
  const tools = workbench.bootstrap?.tools ?? []
  const groups = new Map<string, ToolManifest[]>()
  for (const tool of tools) {
    const current = groups.get(tool.category) ?? []
    current.push(tool)
    groups.set(tool.category, current)
  }
  return Array.from(groups.entries()).map(([category, tools]) => ({ category, tools }))
})

const selectedTool = computed(() => {
  return workbench.bootstrap?.tools.find((tool) => tool.id === selectedToolID.value) ?? null
})

const statusTone = (status: string) => {
  switch (status) {
    case 'running':
      return 'warning'
    case 'success':
      return 'success'
    case 'error':
      return 'error'
    case 'canceled':
      return 'default'
    default:
      return 'info'
  }
}

watch(
  () => workbench.bootstrap,
  (bootstrap) => {
    if (!bootstrap?.tools.length) {
      return
    }
    if (!selectedToolID.value) {
      selectedToolID.value = bootstrap.tools[0].id
    }
  },
  { immediate: true },
)

onMounted(async () => {
  await Promise.all([workbench.loadBootstrap(), execution.hydrate()])
})
</script>

<template>
  <div class="min-h-screen bg-dracula-bg text-dracula-text">
    <div class="mx-auto flex min-h-screen w-full max-w-[1800px] flex-col gap-4 p-4 lg:p-6">
      <header class="rounded-[28px] border border-dracula-soft bg-dracula-panel/95 p-6 shadow-2xl shadow-black/20">
        <div class="grid gap-6 xl:grid-cols-[1.25fr_0.75fr]">
          <div>
            <p class="mb-2 text-xs uppercase tracking-[0.4em] text-dracula-cyan/80">Legacy Reforged</p>
            <h1 class="m-0 text-3xl font-semibold tracking-tight lg:text-5xl">
              {{ workbench.bootstrap?.appTitle ?? '火蜥蜴工具箱 Desktop' }}
            </h1>
            <p class="mt-4 max-w-3xl text-sm leading-7 text-slate-300 lg:text-base">
              从旧版 `tview` 工具箱升级而来，保留工具目录、说明、输入、输出与执行工作流，
              但改造成现代桌面宿主。主页负责选工具，执行页负责专注运行与观察结果。
            </p>
            <div class="mt-5 flex flex-wrap gap-2">
              <n-tag v-for="item in workbench.bootstrap?.hostStack ?? []" :key="item" round bordered type="info">
                {{ item }}
              </n-tag>
              <n-tag round bordered type="success">{{ workbench.bootstrap?.platform ?? 'windows/amd64' }}</n-tag>
            </div>
          </div>

          <div class="grid gap-3 sm:grid-cols-3 xl:grid-cols-1">
            <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
              <div class="text-xs uppercase tracking-[0.25em] text-slate-400">内置工具</div>
              <div class="mt-3 text-3xl font-semibold">{{ workbench.bootstrap?.tools.length ?? 0 }}</div>
            </div>
            <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
              <div class="text-xs uppercase tracking-[0.25em] text-slate-400">最近任务</div>
              <div class="mt-3 text-3xl font-semibold">{{ execution.recentTasks.length }}</div>
            </div>
            <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
              <div class="text-xs uppercase tracking-[0.25em] text-slate-400">输入模式</div>
              <div class="mt-3 text-lg font-semibold">{{ workbench.bootstrap?.parameterModes.join(' / ') }}</div>
            </div>
          </div>
        </div>
      </header>

      <main class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[340px_minmax(0,1fr)_360px]">
        <n-card
          title="工具主页"
          class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
          content-class="p-0"
        >
          <n-scrollbar class="max-h-[68vh] px-3 pb-4">
            <template v-if="workbench.loading">
              <div class="space-y-3 pt-4">
                <n-skeleton v-for="index in 8" :key="index" text />
              </div>
            </template>
            <template v-else-if="workbench.error">
              <n-empty description="工具目录加载失败">
                <template #extra>{{ workbench.error }}</template>
              </n-empty>
            </template>
            <template v-else>
              <section v-for="group in groupedTools" :key="group.category" class="pt-4">
                <p class="mb-2 px-2 text-xs uppercase tracking-[0.3em] text-slate-400">{{ group.category }}</p>
                <div class="space-y-2">
                  <button
                    v-for="tool in group.tools"
                    :key="tool.id"
                    type="button"
                    class="w-full rounded-2xl border px-3 py-3 text-left transition"
                    :class="
                      selectedToolID === tool.id
                        ? 'border-dracula-cyan bg-cyan-400/10'
                        : 'border-dracula-soft bg-transparent hover:border-dracula-cyan/60 hover:bg-white/5'
                    "
                    @click="selectedToolID = tool.id"
                  >
                    <div class="flex items-start justify-between gap-3">
                      <div>
                        <div class="font-medium text-dracula-text">{{ tool.name }}</div>
                        <div class="mt-1 text-xs text-slate-400">{{ tool.description }}</div>
                      </div>
                      <n-tag size="small" :type="tool.kind === 'python' ? 'success' : 'info'">
                        {{ tool.kind }}
                      </n-tag>
                    </div>
                  </button>
                </div>
              </section>
            </template>
          </n-scrollbar>
        </n-card>

        <div class="grid gap-4">
          <n-card class="rounded-[28px] border border-dracula-soft bg-dracula-panel">
            <template #header>
              <div class="flex items-center justify-between gap-4">
                <div>
                  <div class="text-xs uppercase tracking-[0.3em] text-slate-400">当前预览</div>
                  <div class="mt-2 text-2xl font-semibold">{{ selectedTool?.name ?? '选择一个工具' }}</div>
                </div>
                <n-space>
                  <n-button
                    v-if="selectedTool"
                    type="info"
                    size="large"
                    @click="router.push({ name: 'execute', params: { toolId: selectedTool.id } })"
                  >
                    进入执行页
                  </n-button>
                </n-space>
              </div>
            </template>

            <template v-if="selectedTool">
              <div class="grid gap-4 lg:grid-cols-[1.25fr_0.75fr]">
                <div class="space-y-4">
                  <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                    <div class="text-xs uppercase tracking-[0.25em] text-slate-400">工具说明</div>
                    <p class="mb-0 mt-3 text-sm leading-7 text-slate-200">
                      {{ selectedTool.docs.summary || selectedTool.description }}
                    </p>
                  </div>
                  <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                    <div class="text-xs uppercase tracking-[0.25em] text-slate-400">现代化保留的旧习惯</div>
                    <ul class="mb-0 mt-3 list-disc space-y-2 pl-5 text-sm text-slate-200">
                      <li>左侧工具目录仍然是主入口，只是换成更清晰的桌面导航。</li>
                      <li>执行页仍保留“说明 / 输入 / 输出”的核心工作流，只是改成更现代的分栏。</li>
                      <li>原始参数模式被完整保留，复杂场景仍能像旧 TUI 那样直输参数。</li>
                    </ul>
                  </div>
                </div>

                <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.25em] text-slate-400">主路径</div>
                  <ol class="mb-0 mt-3 list-decimal space-y-2 pl-5 text-sm text-slate-200">
                    <li v-for="step in workbench.bootstrap?.primaryFlow ?? []" :key="step">{{ step }}</li>
                  </ol>
                </div>
              </div>
            </template>
            <n-empty v-else description="还没有工具可预览" />
          </n-card>

          <n-card
            title="状态中心"
            class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
            content-class="p-0"
          >
            <n-scrollbar class="max-h-[34vh] px-4 pb-4">
              <template v-if="execution.recentTasks.length === 0">
                <div class="pt-4">
                  <n-empty description="还没有执行过任务">
                    <template #extra>从工具主页进入执行页后，就可以在这里看到最近的运行状态。</template>
                  </n-empty>
                </div>
              </template>
              <div v-else class="space-y-3 pt-4">
                <button
                  v-for="task in execution.recentTasks.slice(0, 8)"
                  :key="task.id"
                  type="button"
                  class="w-full rounded-2xl border border-dracula-soft bg-black/10 px-4 py-3 text-left transition hover:border-dracula-cyan/60 hover:bg-white/5"
                  @click="router.push({ name: 'execute', params: { toolId: task.toolId }, query: { task: task.id } })"
                >
                  <div class="flex items-center justify-between gap-3">
                    <div>
                      <div class="font-medium">{{ task.toolName }}</div>
                      <div class="mt-1 text-xs text-slate-400">{{ task.args || '(无参数)' }}</div>
                    </div>
                    <n-tag size="small" :type="statusTone(task.status)">
                      {{ task.status }}
                    </n-tag>
                  </div>
                </button>
              </div>
            </n-scrollbar>
          </n-card>
        </div>

        <n-card
          title="模块边界"
          class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
        >
          <ul class="mb-0 list-disc space-y-3 pl-5 text-sm leading-7 text-slate-200">
            <li v-for="item in workbench.bootstrap?.moduleBoundaries ?? []" :key="item">{{ item }}</li>
          </ul>
        </n-card>
      </main>
    </div>
  </div>
</template>
