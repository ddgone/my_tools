import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { ListArtifactBatchTasks, StartArtifactBatch } from '../../wailsjs/go/main/App'
import type { ArtifactBatchRequest, ArtifactBatchTask, ToolManifest } from '@/types/workbench'

const STORAGE_KEY = 'fire-salamander.artifact-center.state'

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
}

function loadPersistedState(): ArtifactCenterPersistedState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      throw new Error('missing')
    }
    const parsed = JSON.parse(raw) as Partial<ArtifactCenterPersistedState>
    return {
      mode: parsed.mode === 'build_cache' ? 'build_cache' : 'export',
      exportRootDir: typeof parsed.exportRootDir === 'string' ? parsed.exportRootDir : '',
      concurrency: typeof parsed.concurrency === 'number' ? parsed.concurrency : 4,
      skipUnchanged: parsed.skipUnchanged !== false,
      preferCache: parsed.preferCache !== false,
      forceRebuild: parsed.forceRebuild === true,
      continueOnError: parsed.continueOnError !== false,
      selectedKeys: Array.isArray(parsed.selectedKeys) ? parsed.selectedKeys.filter((item): item is string => typeof item === 'string') : [],
    }
  } catch {
    return {
      mode: 'export',
      exportRootDir: '',
      concurrency: 4,
      skipUnchanged: true,
      preferCache: true,
      forceRebuild: false,
      continueOnError: true,
      selectedKeys: [],
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

  const mode = ref<ArtifactMode>(persisted.mode)
  const exportRootDir = ref(persisted.exportRootDir)
  const concurrency = ref(persisted.concurrency)
  const skipUnchanged = ref(persisted.skipUnchanged)
  const preferCache = ref(persisted.preferCache)
  const forceRebuild = ref(persisted.forceRebuild)
  const continueOnError = ref(persisted.continueOnError)
  const selectedKeys = ref<string[]>(persisted.selectedKeys)
  const tasks = ref<ArtifactBatchTask[]>([])
  const subscribed = ref(false)
  const error = ref('')
  const launching = ref(false)

  watch(
    [mode, exportRootDir, concurrency, skipUnchanged, preferCache, forceRebuild, continueOnError, selectedKeys],
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
        } satisfies ArtifactCenterPersistedState),
      )
    },
    { deep: true },
  )

  const recentTasks = computed(() => [...tasks.value].sort((a, b) => b.startedAt - a.startedAt))

  function ensureSubscriptions() {
    if (subscribed.value) {
      return
    }
    EventsOn('artifact:task:update', (task: ArtifactBatchTask) => {
      const index = tasks.value.findIndex((entry) => entry.id === task.id)
      if (index >= 0) {
        tasks.value[index] = task
      } else {
        tasks.value.unshift(task)
      }
    })
    subscribed.value = true
  }

  async function hydrate() {
    ensureSubscriptions()
    try {
      tasks.value = await ListArtifactBatchTasks()
      error.value = ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载产物任务失败'
      tasks.value = []
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

  function buildRequest(): ArtifactBatchRequest {
    return {
      mode: mode.value,
      exportRootDir: exportRootDir.value.trim(),
      concurrency: Math.min(Math.max(Math.round(concurrency.value || 4), 1), 8),
      skipUnchanged: skipUnchanged.value,
      preferCache: preferCache.value,
      forceRebuild: forceRebuild.value,
      continueOnError: continueOnError.value,
      items: selectedKeys.value.map((key) => {
        const [toolId, platformKey] = key.split(':')
        const [targetOS, targetArch] = platformKey.split('/')
        return { toolId, targetOS, targetArch }
      }),
    }
  }

  async function startBatch() {
    ensureSubscriptions()
    launching.value = true
    error.value = ''
    try {
      const task = await StartArtifactBatch(buildRequest() as any)
      const index = tasks.value.findIndex((entry) => entry.id === task.id)
      if (index >= 0) {
        tasks.value[index] = task
      } else {
        tasks.value.unshift(task)
      }
      return task
    } catch (err) {
      const message = err instanceof Error ? err.message : '启动批量产物任务失败'
      error.value = message
      throw err
    } finally {
      launching.value = false
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
    error,
    launching,
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
  }
})
