<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  NButton,
  NCard,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NRadioButton,
  NRadioGroup,
  NScrollbar,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  type SelectOption,
} from 'naive-ui'

import { useToolArgs } from '@/composables/useToolArgs'
import { useExecutionStore } from '@/stores/execution'
import { useWorkbenchStore } from '@/stores/workbench'
import type { ParameterSpec } from '@/types/workbench'

const route = useRoute()
const router = useRouter()
const workbench = useWorkbenchStore()
const execution = useExecutionStore()

const toolId = computed(() => String(route.params.toolId ?? ''))
const selectedTool = computed(() => {
  return workbench.bootstrap?.tools.find((tool) => tool.id === toolId.value) ?? null
})
const { parameterMode, rawArgs, pythonEnv, formModel, canSubmit } = useToolArgs(selectedTool)
const activeTaskId = ref<string>('')
const launching = ref(false)

const activeTask = computed(() => {
  return execution.recentTasks.find((task) => task.id === activeTaskId.value) ?? null
})

const toolTasks = computed(() => {
  return execution.recentTasks.filter((task) => task.toolId === toolId.value)
})

const activeLogs = computed(() => execution.logsForTask(activeTaskId.value))

const statusTone = (status?: string) => {
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

const renderInputKind = (param: ParameterSpec) => param.type
const toSelectOptions = (param: ParameterSpec): SelectOption[] =>
  (param.options ?? []).map((option) => ({ label: option.label, value: option.value }))

watch(
  () => route.query.task,
  (taskID) => {
    activeTaskId.value = typeof taskID === 'string' ? taskID : ''
  },
  { immediate: true },
)

watch(
  toolTasks,
  (tasks) => {
    if (!activeTaskId.value && tasks.length > 0) {
      activeTaskId.value = tasks[0].id
    }
  },
  { immediate: true },
)

onMounted(async () => {
  await Promise.all([workbench.loadBootstrap(), execution.hydrate()])
})

const launch = async () => {
  if (!selectedTool.value || !canSubmit.value) {
    return
  }

  launching.value = true
  try {
    const task = await execution.startLocalExecution({
      toolId: selectedTool.value.id,
      args: rawArgs.value,
      pythonEnv: selectedTool.value.kind === 'python' ? pythonEnv.value : undefined,
    })
    activeTaskId.value = task.id
  } finally {
    launching.value = false
  }
}

const cancelActiveTask = async () => {
  if (!activeTask.value) {
    return
  }
  await execution.cancelExecution(activeTask.value.id)
}
</script>

<template>
  <div class="min-h-screen bg-dracula-bg text-dracula-text">
    <div class="mx-auto flex min-h-screen w-full max-w-[1800px] flex-col gap-4 p-4 lg:p-6">
      <header class="rounded-[28px] border border-dracula-soft bg-dracula-panel/95 p-5 shadow-2xl shadow-black/20">
        <div class="flex flex-col gap-4 xl:flex-row xl:items-center xl:justify-between">
          <div class="flex items-start gap-3">
            <n-button tertiary @click="router.push({ name: 'home' })">返回主页</n-button>
            <div>
              <div class="text-xs uppercase tracking-[0.35em] text-slate-400">Execution Mode</div>
              <h1 class="m-0 mt-2 text-2xl font-semibold lg:text-4xl">
                {{ selectedTool?.name ?? '执行页' }}
              </h1>
              <p class="mb-0 mt-2 max-w-3xl text-sm leading-7 text-slate-300">
                这是旧版 TUI “使用说明 + 输入面板 + 输出终端”的现代化桌面版。
                左侧看说明，中间看输出，右侧专注参数与运行控制。
              </p>
            </div>
          </div>

          <div class="flex flex-wrap gap-2">
            <n-tag v-if="selectedTool" size="large" round :type="selectedTool.kind === 'python' ? 'success' : 'info'">
              {{ selectedTool.kind }}
            </n-tag>
            <n-tag size="large" round bordered type="warning">本地执行已接通</n-tag>
            <n-tag size="large" round bordered>远程执行界面预留</n-tag>
          </div>
        </div>
      </header>

      <main class="grid min-h-0 flex-1 gap-4 2xl:grid-cols-[340px_minmax(0,1fr)_420px]">
        <n-card
          title="使用说明"
          class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
          content-class="p-0"
        >
          <n-scrollbar class="max-h-[70vh] px-5 pb-5">
            <template v-if="selectedTool">
              <div class="space-y-4 pt-5">
                <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.25em] text-slate-400">摘要</div>
                  <p class="mb-0 mt-3 text-sm leading-7 text-slate-200">
                    {{ selectedTool.docs.summary || selectedTool.description }}
                  </p>
                </div>
                <div class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.25em] text-slate-400">旧版说明迁移</div>
                  <pre class="m-0 mt-3 whitespace-pre-wrap break-words font-sans text-sm leading-7 text-slate-200">{{
                    selectedTool.docs.usage
                  }}</pre>
                </div>
              </div>
            </template>
            <n-empty v-else description="未找到该工具" />
          </n-scrollbar>
        </n-card>

        <div class="grid min-h-0 gap-4">
          <n-card
            class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
            content-class="p-0"
          >
            <template #header>
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div class="text-xs uppercase tracking-[0.25em] text-slate-400">运行时输出</div>
                  <div class="mt-2 text-xl font-semibold">
                    {{ activeTask ? activeTask.toolName : '等待执行' }}
                  </div>
                </div>
                <n-space>
                  <n-tag v-if="activeTask" :type="statusTone(activeTask.status)">
                    {{ activeTask.status }}
                  </n-tag>
                  <n-tag v-if="activeTask?.exitMessage" bordered>{{ activeTask.exitMessage }}</n-tag>
                </n-space>
              </div>
            </template>

            <n-scrollbar class="h-[48vh] px-5 pb-5">
              <template v-if="activeLogs.length > 0">
                <div class="space-y-3 pt-5">
                  <div
                    v-for="(entry, index) in activeLogs"
                    :key="`${index}-${entry}`"
                    class="rounded-2xl border border-dracula-soft bg-black/10 px-4 py-3 text-sm leading-7 text-slate-100"
                  >
                    {{ entry }}
                  </div>
                </div>
              </template>
              <div v-else class="pt-5">
                <n-empty description="还没有输出日志">
                  <template #extra>点击右侧“开始本地执行”后，这里会实时流式显示旧工具输出。</template>
                </n-empty>
              </div>
            </n-scrollbar>
          </n-card>

          <n-card
            title="任务切换"
            class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
            content-class="p-0"
          >
            <n-scrollbar class="max-h-[22vh] px-4 pb-4">
              <template v-if="toolTasks.length === 0">
                <div class="pt-4">
                  <n-empty description="当前工具还没有任务记录" />
                </div>
              </template>
              <div v-else class="space-y-3 pt-4">
                <button
                  v-for="task in toolTasks"
                  :key="task.id"
                  type="button"
                  class="w-full rounded-2xl border px-4 py-3 text-left transition"
                  :class="
                    activeTaskId === task.id
                      ? 'border-dracula-cyan bg-cyan-400/10'
                      : 'border-dracula-soft bg-black/10 hover:border-dracula-cyan/60 hover:bg-white/5'
                  "
                  @click="activeTaskId = task.id"
                >
                  <div class="flex items-center justify-between gap-3">
                    <div>
                      <div class="font-medium">{{ task.args || '(无参数)' }}</div>
                      <div class="mt-1 text-xs text-slate-400">{{ new Date(task.startedAt).toLocaleString() }}</div>
                    </div>
                    <n-tag size="small" :type="statusTone(task.status)">{{ task.status }}</n-tag>
                  </div>
                </button>
              </div>
            </n-scrollbar>
          </n-card>
        </div>

        <n-card
          title="输入与控制"
          class="rounded-[28px] border border-dracula-soft bg-dracula-panel"
          content-class="flex h-full flex-col gap-4"
        >
          <section class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <div>
                <div class="text-xs uppercase tracking-[0.25em] text-slate-400">参数模式</div>
                <div class="mt-2 text-sm text-slate-300">结构化表单默认可用，复杂场景可切回原始参数。</div>
              </div>
              <n-radio-group v-model:value="parameterMode">
                <n-radio-button value="structured">结构化表单</n-radio-button>
                <n-radio-button value="raw">原始参数</n-radio-button>
              </n-radio-group>
            </div>
          </section>

          <n-scrollbar class="min-h-0 flex-1 pr-1">
            <template v-if="selectedTool">
              <div class="space-y-4">
                <div v-if="selectedTool.kind === 'python'" class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                  <div class="text-xs uppercase tracking-[0.25em] text-slate-400">Python 解释器</div>
                  <n-input
                    v-model:value="pythonEnv"
                    class="mt-3"
                    placeholder="默认使用 python"
                  />
                </div>

                <n-form
                  v-if="parameterMode === 'structured'"
                  label-placement="top"
                  class="space-y-2"
                >
                  <n-form-item
                    v-for="param in selectedTool.params"
                    :key="param.key"
                    :label="param.label"
                    :required="param.required"
                  >
                    <div class="w-full">
                      <n-input
                        v-if="renderInputKind(param) === 'text' || renderInputKind(param) === 'path'"
                        v-model:value="formModel[param.key] as string"
                        :placeholder="param.placeholder"
                      />
                      <n-input
                        v-else-if="renderInputKind(param) === 'textarea'"
                        v-model:value="formModel[param.key] as string"
                        type="textarea"
                        :placeholder="param.placeholder"
                        :autosize="{ minRows: 3, maxRows: 6 }"
                      />
                      <n-input-number
                        v-else-if="renderInputKind(param) === 'number'"
                        v-model:value="formModel[param.key] as number | null"
                        class="w-full"
                        :show-button="false"
                      />
                      <n-switch
                        v-else-if="renderInputKind(param) === 'boolean'"
                        v-model:value="formModel[param.key] as boolean"
                      />
                      <n-select
                        v-else-if="renderInputKind(param) === 'select'"
                        v-model:value="formModel[param.key] as string"
                        :options="toSelectOptions(param)"
                      />
                      <p v-if="param.help" class="mb-0 mt-2 text-xs text-slate-400">{{ param.help }}</p>
                    </div>
                  </n-form-item>
                </n-form>

                <div v-else class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
                  <n-input
                    v-model:value="rawArgs"
                    type="textarea"
                    placeholder="直接输入最终参数，例如 -input &quot;D:\data&quot; -workers 4"
                    :autosize="{ minRows: 12, maxRows: 18 }"
                  />
                  <p class="mb-0 mt-3 text-xs text-slate-400">
                    原始参数模式完全继承旧版 TUI 的工作方式，适合复杂 flag 组合或高级调试。
                  </p>
                </div>
              </div>
            </template>
          </n-scrollbar>

          <section class="rounded-2xl border border-dracula-soft bg-black/10 p-4">
            <div class="text-xs uppercase tracking-[0.25em] text-slate-400">当前参数预览</div>
            <code class="mt-3 block whitespace-pre-wrap break-all text-sm leading-7 text-dracula-yellow">
              {{ rawArgs || '(当前没有参数)' }}
            </code>
          </section>

          <div class="grid gap-3 sm:grid-cols-3">
            <n-button type="info" size="large" :disabled="!canSubmit || launching" @click="launch">
              <template v-if="launching">
                <n-spin size="small" class="mr-2" />
              </template>
              开始本地执行
            </n-button>
            <n-button
              size="large"
              type="warning"
              :disabled="!activeTask || activeTask.status !== 'running'"
              @click="cancelActiveTask"
            >
              取消当前任务
            </n-button>
            <n-button size="large" tertiary @click="router.push({ name: 'home' })">返回工具主页</n-button>
          </div>
        </n-card>
      </main>
    </div>
  </div>
</template>
