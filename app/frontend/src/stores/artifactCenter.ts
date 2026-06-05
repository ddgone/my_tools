import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { ClearArtifactBatchTasks, EstimateArtifactBatchCache, ListArtifactBatchTasks, StartArtifactBatch } from '../../wailsjs/go/main/App'
import type { ArtifactBatchEstimate, ArtifactBatchRequest, ArtifactBatchTask, ToolManifest } from '@/types/workbench'

const STORAGE_KEY = 'fire-salamander.artifact-center.state'
const MAX_TASK_HISTORY = 10

export interface ArtifactPlatformOption {
  key: string
  os: string
  arch: string
  label: string
}

type ArtifactMode = 'build_cache' | 'export'

interface ArtifactCenterPersistedState {
  mode: ArtifactMode
  exportRootDir: string
  concurrency: number
  skipUnchanged: boolean
  preferCache: boolean
  forceRebuild: boolean
  continueOnError: boolean
  selectedKeys: string[]
  expandedTaskId?: string | null
}

interface LoadedArtifactCenterState {
  state: ArtifactCenterPersistedState
  hasExpandedTaskPreference: boolean
}

function loadPersistedState(): LoadedArtifactCenterState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      throw new Error('missing')
    }
    const parsed = JSON.parse(raw) as Partial<ArtifactCenterPersistedState>
    return {
      state: {
        mode: parsed.mode === 'build_cache' ? 'build_cache' : 'export',
        exportRootDir: typeof parsed.exportRootDir === 'string' ? parsed.exportRootDir : '',
        concurrency: typeof parsed.concurrency === 'number' ? parsed.concurrency : 4,
        skipUnchanged: parsed.skipUnchanged !== false,
        preferCache: parsed.preferCache !== false,
        forceRebuild: parsed.forceRebuild === true,
        continueOnError: parsed.continueOnError !== false,
        selectedKeys: Array.isArray(parsed.selectedKeys) ? parsed.selectedKeys.filter((item): item is string => typeof item === 'string') : [],
        expandedTaskId: typeof parsed.expandedTaskId === 'string' ? parsed.expandedTaskId : parsed.expandedTaskId === null ? null : undefined,
      },
      hasExpandedTaskPreference: Object.prototype.hasOwnProperty.call(parsed, 'expandedTaskId'),
    }
  } catch {
    return {
      state: {
        mode: 'export',
        exportRootDir: '',
        concurrency: 4,
        skipUnchanged: true,
        preferCache: true,
        forceRebuild: false,
        continueOnError: true,
        selectedKeys: [],
      },
      hasExpandedTaskPreference: false,
    }
  }
}

export const artifactPlatforms: ArtifactPlatformOption[] = [
  { key: 'windows/amd64', os: 'windows', arch: 'amd64', label: 'Win64' },
  { key: 'windows/arm64', os: 'windows', arch: 'arm64', label: 'Win ARM' },
  { key: 'linux/amd64', os: 'linux', arch: 'amd64', label: 'Linux64' },
  { key: 'linux/arm64', os: 'linux', arch: 'arm64', label: 'Linux ARM' },
  { key: 'darwin/amd64', os: 'darwin', arch: 'amd64', label: 'mac Intel' },
  { key: 'darwin/arm64', os: 'darwin', arch: 'arm64', label: 'mac Apple' },
]

export const useArtifactCenterStore = defineStore('artifact-center', () => {
  const persisted = loadPersistedState()

  const mode = ref<ArtifactMode>(persisted.state.mode)
  const exportRootDir = ref(persisted.state.exportRootDir)
  const concurrency = ref(persisted.state.concurrency)
  const skipUnchanged = ref(persisted.state.skipUnchanged)
  const preferCache = ref(persisted.state.preferCache)
  const forceRebuild = ref(persisted.state.forceRebuild)
  const continueOnError = ref(persisted.state.continueOnError)
  const selectedKeys = ref<string[]>(persisted.state.selectedKeys)
  const tasks = ref<ArtifactBatchTask[]>([])
  const expandedTaskId = ref<string | null>(persisted.state.expandedTaskId ?? null)
  const hasExpandedTaskPreference = ref(persisted.hasExpandedTaskPreference)
  const subscribed = ref(false)
  const error = ref('')
  const launching = ref(false)
  const estimate = ref<ArtifactBatchEstimate | null>(null)
  const estimating = ref(false)

  watch(
    [mode, exportRootDir, concurrency, skipUnchanged, preferCache, forceRebuild, continueOnError, selectedKeys, expandedTaskId],
    () => {
      localStorage.setItem(
        STORAGE_KEY,
        JSON.stringify({
          mode: mode.value,
          exportRootDir: exportRootDir.value,
          concurrency: concurrency.value,
          skipUnchanged: skipUnchanged.value,
          preferCache: preferCache.value,
          forceRebuild: forceRebuild.value,
          continueOnError: continueOnError.value,
          selectedKeys: selectedKeys.value,
          expandedTaskId: expandedTaskId.value,
        } satisfies ArtifactCenterPersistedState),
      )
    },
    { deep: true },
  )

  const recentTasks = computed(() => [...tasks.value].sort((a, b) => b.startedAt - a.startedAt).slice(0, MAX_TASK_HISTORY))

  type PartialArtifactTask = Partial<ArtifactBatchTask> & Pick<ArtifactBatchTask, 'id' | 'mode' | 'status' | 'startedAt' | 'items'>

  function normalizeTask(task: PartialArtifactTask): ArtifactBatchTask {
    return {
      exportRootDir: '',
      concurrency: 4,
      skipUnchanged: true,
      preferCache: true,
      forceRebuild: false,
      continueOnError: true,
      totalCount: 0,
      successCount: 0,
      errorCount: 0,
      cachedCount: 0,
      skippedCount: 0,
      ...task,
      items: task.items ?? [],
    }
  }

  function normalizeTasks(nextTasks: ArtifactBatchTask[]) {
    return [...nextTasks]
      .sort((a, b) => b.startedAt - a.startedAt)
      .slice(0, MAX_TASK_HISTORY)
  }

  function upsertTask(task: PartialArtifactTask) {
    const next = tasks.value.filter((entry) => entry.id !== task.id)
    next.unshift(normalizeTask(task))
    tasks.value = normalizeTasks(next)
    if (expandedTaskId.value && !tasks.value.some((entry) => entry.id === expandedTaskId.value)) {
      expandedTaskId.value = tasks.value[0]?.id ?? null
      hasExpandedTaskPreference.value = true
    }
  }

  function ensureSubscriptions() {
    if (subscribed.value) {
      return
    }
    EventsOn('artifact:task:update', (task: ArtifactBatchTask) => {
      upsertTask(task)
    })
    subscribed.value = true
  }

  async function hydrate() {
    ensureSubscriptions()
    try {
      tasks.value = normalizeTasks((await ListArtifactBatchTasks()).map((task) => normalizeTask(task as PartialArtifactTask)))
      if (expandedTaskId.value && !tasks.value.some((entry) => entry.id === expandedTaskId.value)) {
        expandedTaskId.value = tasks.value[0]?.id ?? null
        hasExpandedTaskPreference.value = true
      }
      error.value = ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载产物任务失败'
      tasks.value = []
    }
  }

  function setExpandedTask(taskId: string | null) {
    expandedTaskId.value = taskId
    hasExpandedTaskPreference.value = true
  }

  function ensureExpandedTask() {
    if (tasks.value.length === 0) {
      if (hasExpandedTaskPreference.value && expandedTaskId.value !== null) {
        expandedTaskId.value = null
      }
      return
    }
    if (expandedTaskId.value && tasks.value.some((task) => task.id === expandedTaskId.value)) {
      return
    }
    if (!hasExpandedTaskPreference.value) {
      expandedTaskId.value = tasks.value[0]?.id ?? null
    } else if (expandedTaskId.value) {
      expandedTaskId.value = tasks.value[0]?.id ?? null
    }
  }

  function ensureToolSelections(tools: ToolManifest[]) {
    const validToolIds = new Set(tools.map((tool) => tool.id))
    selectedKeys.value = selectedKeys.value.filter((key) => validToolIds.has(key.split(':')[0]))
  }

  function selectionKey(toolId: string, platformKey: string) {
    return `${toolId}:${platformKey}`
  }

  function isSelected(toolId: string, platformKey: string) {
    return selectedKeys.value.includes(selectionKey(toolId, platformKey))
  }

  function setSelected(toolId: string, platformKey: string, value: boolean) {
    const key = selectionKey(toolId, platformKey)
    const exists = selectedKeys.value.includes(key)
    if (value && !exists) {
      selectedKeys.value = [...selectedKeys.value, key]
      return
    }
    if (!value && exists) {
      selectedKeys.value = selectedKeys.value.filter((item) => item !== key)
    }
  }

  function toggleSelected(toolId: string, platformKey: string) {
    setSelected(toolId, platformKey, !isSelected(toolId, platformKey))
  }

  function clearSelections() {
    selectedKeys.value = []
  }

  function selectAllGoTargets(tools: ToolManifest[]) {
    const next = new Set(selectedKeys.value)
    tools
      .filter((tool) => tool.kind === 'go')
      .forEach((tool) => {
        artifactPlatforms.forEach((platform) => {
          next.add(selectionKey(tool.id, platform.key))
        })
      })
    selectedKeys.value = Array.from(next)
  }

  function setToolSelections(toolId: string, value: boolean) {
    artifactPlatforms.forEach((platform) => setSelected(toolId, platform.key, value))
  }

  function setPlatformSelections(platformKey: string, value: boolean, tools: ToolManifest[]) {
    tools
      .filter((tool) => tool.kind === 'go')
      .forEach((tool) => setSelected(tool.id, platformKey, value))
  }

  function buildRequest(override?: Partial<ArtifactBatchRequest>): ArtifactBatchRequest {
    const baseItems = override?.items ?? selectedKeys.value.map((key) => {
      const [toolId, platformKey] = key.split(':')
      const [targetOS, targetArch] = platformKey.split('/')
      return { toolId, targetOS, targetArch }
    })
    return {
      mode: override?.mode ?? mode.value,
      exportRootDir: override?.exportRootDir ?? exportRootDir.value.trim(),
      concurrency: override?.concurrency ?? Math.min(Math.max(Math.round(concurrency.value || 4), 1), 8),
      skipUnchanged: override?.skipUnchanged ?? skipUnchanged.value,
      preferCache: override?.preferCache ?? preferCache.value,
      forceRebuild: override?.forceRebuild ?? forceRebuild.value,
      continueOnError: override?.continueOnError ?? continueOnError.value,
      items: baseItems,
    }
  }

  async function startBatch(override?: Partial<ArtifactBatchRequest>) {
    ensureSubscriptions()
    launching.value = true
    error.value = ''
    try {
      const task = await StartArtifactBatch(buildRequest(override) as any)
      upsertTask(task)
      setExpandedTask(task.id)
      return task
    } catch (err) {
      const message = err instanceof Error ? err.message : '启动批量产物任务失败'
      error.value = message
      throw err
    } finally {
      launching.value = false
    }
  }

  async function clearTasks() {
    await ClearArtifactBatchTasks()
    tasks.value = []
    setExpandedTask(null)
  }

  async function refreshEstimate(override?: Partial<ArtifactBatchRequest>) {
    const request = buildRequest(override)
    if (request.items.length === 0) {
      estimate.value = null
      return null
    }
    estimating.value = true
    try {
      const nextEstimate = await EstimateArtifactBatchCache(request as any) as ArtifactBatchEstimate
      estimate.value = nextEstimate
      return nextEstimate
    } catch (err) {
      estimate.value = null
      error.value = err instanceof Error ? err.message : '估算构建缓存失败'
      return null
    } finally {
      estimating.value = false
    }
  }

  return {
    mode,
    exportRootDir,
    concurrency,
    skipUnchanged,
    preferCache,
    forceRebuild,
    continueOnError,
    selectedKeys,
    tasks,
    recentTasks,
    expandedTaskId,
    hasExpandedTaskPreference,
    error,
    launching,
    estimate,
    estimating,
    ensureSubscriptions,
    hydrate,
    ensureToolSelections,
    isSelected,
    setSelected,
    toggleSelected,
    clearSelections,
    selectAllGoTargets,
    setToolSelections,
    setPlatformSelections,
    startBatch,
    buildRequest,
    refreshEstimate,
    setExpandedTask,
    ensureExpandedTask,
    clearTasks,
  }
})
