import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type { ToolManifest } from '@/types/workbench'

export interface ToolTabState {
  tabId: string
  toolId: string
  executionTarget: ExecutionTarget
  localConfig: ToolExecutionConfig
  remoteConfig: RemoteExecutionConfig
  openedAt: number
}

export type ExecutionTarget = 'local' | 'remote'
export type ToolPanelMode = 'form' | 'cli' | 'docs' | 'remote'

export interface ToolExecutionConfig {
  panelMode: ToolPanelMode
  rawArgs: string
  pythonEnv: string
  formModel: Record<string, string | number | boolean | null>
}

export interface RemoteExecutionConfig extends ToolExecutionConfig {
  connId: string
}

export interface UserSettings {
  recentToolsCount: number
  historyRetention: number
  logExportDir: string
  defaultPythonPath: string
  confirmExit: boolean
  autoWordWrap: boolean
  autoExpandAll: boolean
  verboseShortcuts: boolean
  bgmEnabled: boolean
}

const STORAGE_KEYS = {
  favorites: 'fire-salamander:favorites',
  recent: 'fire-salamander:recent',
  history: 'fire-salamander:history',
  settings: 'fire-salamander:settings',
  toolState: 'fire-salamander:tool-state',
} as const

function loadJSON<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key)
    if (raw) return JSON.parse(raw)
  } catch { /* empty */ }
  return fallback
}

function saveJSON(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch { /* empty */ }
}

const defaultSettings: UserSettings = {
  recentToolsCount: 5,
  historyRetention: 50,
  logExportDir: 'my_tools_logs',
  defaultPythonPath: 'python',
  confirmExit: false,
  autoWordWrap: true,
  autoExpandAll: false,
  verboseShortcuts: true,
  bgmEnabled: false,
}

interface HistoryEntry {
  args: string
  pythonEnv: string
  formModel: Record<string, string | number | boolean | null>
  timestamp: number
}

interface PersistedToolState {
  executionTarget: ExecutionTarget
  localConfig: ToolExecutionConfig
  remoteConfig: RemoteExecutionConfig
  updatedAt: number
}

interface RecentEntry {
  toolId: string
  args: string
  timestamp: number
}

function createTabState(
  tool: ToolManifest,
  persistedState?: PersistedToolState,
  historyEntry?: HistoryEntry,
  defaultPythonPath = 'python',
): ToolTabState {
  const localConfig = persistedState?.localConfig
    ? normalizeExecutionConfig(tool, persistedState.localConfig, defaultPythonPath, 'local')
    : historyEntry
      ? normalizeExecutionConfig(
          tool,
          {
            panelMode: 'form',
            rawArgs: historyEntry.args,
            pythonEnv: historyEntry.pythonEnv,
            formModel: historyEntry.formModel,
          },
          defaultPythonPath,
          'local',
        )
      : createDefaultExecutionConfig(tool, defaultPythonPath, 'local')

  const remoteConfig = persistedState?.remoteConfig
    ? normalizeRemoteExecutionConfig(tool, persistedState.remoteConfig, defaultPythonPath)
    : createDefaultRemoteExecutionConfig(tool, defaultPythonPath)

  return {
    tabId: `tool_${tool.id}`,
    toolId: tool.id,
    executionTarget: persistedState?.executionTarget === 'remote' ? 'remote' : 'local',
    localConfig,
    remoteConfig,
    openedAt: Date.now(),
  }
}

function createDefaultFormModel(tool: ToolManifest): Record<string, string | number | boolean | null> {
  const formModel: Record<string, string | number | boolean | null> = {}
  for (const param of tool.params) {
    if (param.default !== undefined) {
      formModel[param.key] = param.default as string | number | boolean | null
      continue
    }
    switch (param.type) {
      case 'number':
        formModel[param.key] = null
        break
      case 'boolean':
        formModel[param.key] = false
        break
      default:
        formModel[param.key] = ''
    }
  }
  return formModel
}

function normalizeFormModel(
  tool: ToolManifest,
  source?: Record<string, string | number | boolean | null>,
): Record<string, string | number | boolean | null> {
  const formModel = createDefaultFormModel(tool)
  if (!source) {
    return formModel
  }
  for (const param of tool.params) {
    if (Object.prototype.hasOwnProperty.call(source, param.key)) {
      formModel[param.key] = source[param.key]
    }
  }
  return formModel
}

function sanitizePanelMode(mode: ToolPanelMode | undefined, target: ExecutionTarget): ToolPanelMode {
  if (mode === 'form' || mode === 'cli' || mode === 'docs') {
    return mode
  }
  if (mode === 'remote' && target === 'remote') {
    return mode
  }
  return 'form'
}

function cloneExecutionConfig(config: ToolExecutionConfig): ToolExecutionConfig {
  return {
    panelMode: config.panelMode,
    rawArgs: config.rawArgs,
    pythonEnv: config.pythonEnv,
    formModel: { ...config.formModel },
  }
}

function cloneRemoteExecutionConfig(config: RemoteExecutionConfig): RemoteExecutionConfig {
  return {
    ...cloneExecutionConfig(config),
    connId: config.connId,
  }
}

function createDefaultExecutionConfig(
  tool: ToolManifest,
  defaultPythonPath: string,
  target: ExecutionTarget,
): ToolExecutionConfig {
  const formModel = createDefaultFormModel(tool)
  return {
    panelMode: sanitizePanelMode(undefined, target),
    rawArgs: buildRawArgs(tool, formModel),
    pythonEnv: defaultPythonPath,
    formModel,
  }
}

function createDefaultRemoteExecutionConfig(
  tool: ToolManifest,
  defaultPythonPath: string,
): RemoteExecutionConfig {
  return {
    ...createDefaultExecutionConfig(tool, defaultPythonPath, 'remote'),
    connId: '',
  }
}

function normalizeExecutionConfig(
  tool: ToolManifest,
  source: Partial<ToolExecutionConfig> | undefined,
  defaultPythonPath: string,
  target: ExecutionTarget,
): ToolExecutionConfig {
  const formModel = normalizeFormModel(tool, source?.formModel)
  return {
    panelMode: sanitizePanelMode(source?.panelMode, target),
    rawArgs: typeof source?.rawArgs === 'string' ? source.rawArgs : buildRawArgs(tool, formModel),
    pythonEnv:
      typeof source?.pythonEnv === 'string' && source.pythonEnv.trim().length > 0
        ? source.pythonEnv
        : defaultPythonPath,
    formModel,
  }
}

function normalizeRemoteExecutionConfig(
  tool: ToolManifest,
  source: Partial<RemoteExecutionConfig> | undefined,
  defaultPythonPath: string,
): RemoteExecutionConfig {
  const config = normalizeExecutionConfig(tool, source, defaultPythonPath, 'remote')
  return {
    ...config,
    connId: typeof source?.connId === 'string' ? source.connId : '',
  }
}

function snapshotToolState(tab: ToolTabState): PersistedToolState {
  return {
    executionTarget: tab.executionTarget,
    localConfig: cloneExecutionConfig(tab.localConfig),
    remoteConfig: cloneRemoteExecutionConfig(tab.remoteConfig),
    updatedAt: Date.now(),
  }
}

function buildRawArgs(tool: ToolManifest, formModel: Record<string, string | number | boolean | null>): string {
  const parts: string[] = []
  for (const param of tool.params) {
    const value = formModel[param.key]
    const argKey = param.argKey || param.key

    if (param.type === 'boolean') {
      if (value === true) {
        parts.push(`-${argKey}`)
      }
      continue
    }

    if (value === undefined || value === null || value === '') {
      continue
    }

    const escapedValue =
      typeof value === 'string' ? formatCliValue(value) : String(value)
    parts.push(`-${argKey}`, escapedValue)
  }
  return parts.join(' ')
}

function formatCliValue(value: string): string {
  if (!/[\s'"]/.test(value)) {
    return value
  }
  return `"${value.replace(/\\/g, '\\\\').replace(/"/g, '\\"')}"`
}

export interface SSHTabState {
  tabId: string
  connectionId: string
  label: string
  isNew: boolean
  openedAt: number
}

function formatNewSSHLabel(savedCount: number): string {
  return savedCount <= 0 ? '新建连接' : `新建连接 ${savedCount + 1}`
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const openTabs = ref<ToolTabState[]>([])
  const activeTabIndex = ref(-1)
  const sshTabs = ref<SSHTabState[]>([])
  const activeSSHTabIndex = ref(-1)
  const showSearch = ref(false)
  const showHotkeyHelp = ref(false)
  const showSettings = ref(false)

  const favorites = ref<string[]>(loadJSON<string[]>(STORAGE_KEYS.favorites, []))
  const recentTools = ref<RecentEntry[]>(loadJSON<RecentEntry[]>(STORAGE_KEYS.recent, []))
  const toolHistory = ref<Record<string, HistoryEntry[]>>(loadJSON<Record<string, HistoryEntry[]>>(STORAGE_KEYS.history, {}))
  const settings = ref<UserSettings>({ ...defaultSettings, ...loadJSON<Partial<UserSettings>>(STORAGE_KEYS.settings, {}) })
  const persistedToolStates = ref<Record<string, PersistedToolState>>(
    loadJSON<Record<string, PersistedToolState>>(STORAGE_KEYS.toolState, {}),
  )

  watch(favorites, (v) => saveJSON(STORAGE_KEYS.favorites, v), { deep: true })
  watch(recentTools, (v) => saveJSON(STORAGE_KEYS.recent, v), { deep: true })
  watch(toolHistory, (v) => saveJSON(STORAGE_KEYS.history, v), { deep: true })
  watch(settings, (v) => saveJSON(STORAGE_KEYS.settings, v), { deep: true })
  watch(persistedToolStates, (v) => saveJSON(STORAGE_KEYS.toolState, v), { deep: true })
  watch(
    openTabs,
    (tabs) => {
      const nextState = { ...persistedToolStates.value }
      for (const tab of tabs) {
        nextState[tab.toolId] = snapshotToolState(tab)
      }
      persistedToolStates.value = nextState
    },
    { deep: true },
  )

  const activeTabType = computed(() => {
    if (activeSSHTabIndex.value >= 0) return 'ssh'
    if (activeTabIndex.value >= 0) return 'tool'
    return 'none'
  })

  const activeToolTab = computed(() =>
    activeTabIndex.value >= 0 ? openTabs.value[activeTabIndex.value] : undefined,
  )

  const activeExecutionConfig = computed<ToolExecutionConfig | RemoteExecutionConfig | undefined>(() => {
    const tab = activeToolTab.value
    if (!tab) {
      return undefined
    }
    return tab.executionTarget === 'remote' ? tab.remoteConfig : tab.localConfig
  })

  const activeSSHTab = computed(() =>
    activeSSHTabIndex.value >= 0 ? sshTabs.value[activeSSHTabIndex.value] : undefined,
  )

  const activeTab = () => (activeTabIndex.value >= 0 ? openTabs.value[activeTabIndex.value] : undefined)

  interface UnifiedTabItem {
    type: 'tool' | 'ssh'
    key: string
    label: string
    openedAt: number
    arrayIndex: number
  }

  const unifiedTabs = computed<UnifiedTabItem[]>(() => {
    const items: UnifiedTabItem[] = [
      ...openTabs.value.map((t, i) => ({
        type: 'tool' as const,
        key: `tool:${t.tabId}`,
        label: t.toolId,
        openedAt: t.openedAt,
        arrayIndex: i,
      })),
      ...sshTabs.value.map((s, i) => ({
        type: 'ssh' as const,
        key: `ssh:${s.tabId}`,
        label: s.label,
        openedAt: s.openedAt,
        arrayIndex: i,
      })),
    ]
    items.sort((a, b) => a.openedAt - b.openedAt)
    return items
  })

  function activateUnifiedTab(item: UnifiedTabItem) {
    if (item.type === 'tool') {
      activeTabIndex.value = item.arrayIndex
      activeSSHTabIndex.value = -1
    } else {
      activeSSHTabIndex.value = item.arrayIndex
      activeTabIndex.value = -1
    }
  }

  function closeUnifiedTab(item: UnifiedTabItem) {
    if (item.type === 'tool') {
      openTabs.value.splice(item.arrayIndex, 1)
      if (item.arrayIndex === activeTabIndex.value) {
        if (openTabs.value.length === 0) {
          activeTabIndex.value = -1
        } else if (item.arrayIndex >= openTabs.value.length) {
          activeTabIndex.value = openTabs.value.length - 1
        }
      } else if (item.arrayIndex < activeTabIndex.value) {
        activeTabIndex.value--
      }
    } else {
      sshTabs.value.splice(item.arrayIndex, 1)
      if (item.arrayIndex === activeSSHTabIndex.value) {
        if (sshTabs.value.length === 0) {
          activeSSHTabIndex.value = -1
        } else if (item.arrayIndex >= sshTabs.value.length) {
          activeSSHTabIndex.value = sshTabs.value.length - 1
        }
      } else if (item.arrayIndex < activeSSHTabIndex.value) {
        activeSSHTabIndex.value--
      }
    }
  }

  function openTool(tool: ToolManifest) {
    const existing = openTabs.value.findIndex((t) => t.toolId === tool.id)
    if (existing >= 0) {
      activeTabIndex.value = existing
      activeSSHTabIndex.value = -1
      return
    }

    const history = toolHistory.value[tool.id] || []
    const lastEntry = history[0]
    const persistedState = persistedToolStates.value[tool.id]

    const tab = createTabState(tool, persistedState, lastEntry, settings.value.defaultPythonPath)
    openTabs.value.push(tab)
    activeTabIndex.value = openTabs.value.length - 1
    activeSSHTabIndex.value = -1
  }

  function closeTab(index: number) {
    openTabs.value.splice(index, 1)
    if (index === activeTabIndex.value) {
      if (openTabs.value.length === 0) {
        activeTabIndex.value = -1
      } else if (index >= openTabs.value.length) {
        activeTabIndex.value = openTabs.value.length - 1
      }
    } else if (index < activeTabIndex.value) {
      activeTabIndex.value--
    }
  }

  function updateRawArgs(tool: ToolManifest, tab: ToolTabState) {
    const config = tab.executionTarget === 'remote' ? tab.remoteConfig : tab.localConfig
    config.rawArgs = buildRawArgs(tool, config.formModel)
  }

  function setExecutionTarget(index: number, target: ExecutionTarget) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].executionTarget = target
  }

  function setRemoteConnection(index: number, connId: string) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].remoteConfig.connId = connId
  }

  function setPythonEnv(index: number, value: string) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    const tab = openTabs.value[index]
    const config = tab.executionTarget === 'remote' ? tab.remoteConfig : tab.localConfig
    config.pythonEnv = value
  }

  function setPanelMode(index: number, value: ToolPanelMode) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    const tab = openTabs.value[index]
    const config = tab.executionTarget === 'remote' ? tab.remoteConfig : tab.localConfig
    config.panelMode = value
  }

  function setActiveTab(index: number) {
    if (index >= 0 && index < openTabs.value.length) {
      activeTabIndex.value = index
    }
  }

  function recordUsage(toolId: string, args: string, pythonEnv: string, formModel?: Record<string, string | number | boolean | null>) {
    const maxRecent = settings.value.recentToolsCount

    recentTools.value = [
      { toolId, args, timestamp: Date.now() },
      ...recentTools.value.filter((r) => r.toolId !== toolId),
    ].slice(0, maxRecent)

    const existing = toolHistory.value[toolId] || []
    const maxHistory = settings.value.historyRetention

    const deduped = existing.filter((h) => h.args !== args || h.pythonEnv !== pythonEnv)
    toolHistory.value[toolId] = [
      { args, pythonEnv, formModel: formModel || {}, timestamp: Date.now() },
      ...deduped,
    ].slice(0, maxHistory)
  }

  function toggleFavorite(toolId: string) {
    const idx = favorites.value.indexOf(toolId)
    if (idx >= 0) {
      favorites.value.splice(idx, 1)
    } else {
      favorites.value.push(toolId)
    }
  }

  function isFavorite(toolId: string): boolean {
    return favorites.value.includes(toolId)
  }

  function getHistory(toolId: string): HistoryEntry[] {
    return toolHistory.value[toolId] || []
  }

  function getRecentArgs(toolId: string): string {
    const entry = recentTools.value.find((r) => r.toolId === toolId)
    return entry?.args || ''
  }

  function resetAllData() {
    favorites.value = []
    recentTools.value = []
    toolHistory.value = {}
    persistedToolStates.value = {}
    settings.value = { ...defaultSettings }
    Object.values(STORAGE_KEYS).forEach((k) => localStorage.removeItem(k))
  }

  function openSSHNew(savedCount: number) {
    const existing = sshTabs.value.findIndex((t) => t.isNew && !t.connectionId)
    if (existing >= 0) {
      activeSSHTabIndex.value = existing
      activeTabIndex.value = -1
      return
    }
    const openedAt = Date.now()
    const tab: SSHTabState = {
      tabId: `ssh_new_${openedAt}`,
      connectionId: '',
      label: formatNewSSHLabel(savedCount),
      isNew: true,
      openedAt,
    }
    sshTabs.value.push(tab)
    activeSSHTabIndex.value = sshTabs.value.length - 1
    activeTabIndex.value = -1
  }

  function openSSHEdit(connId: string, label: string) {
    const existing = sshTabs.value.findIndex((t) => t.connectionId === connId)
    if (existing >= 0) {
      activeSSHTabIndex.value = existing
      activeTabIndex.value = -1
      return
    }
    const tab: SSHTabState = {
      tabId: `ssh_${connId}`,
      connectionId: connId,
      label,
      isNew: false,
      openedAt: Date.now(),
    }
    sshTabs.value.push(tab)
    activeSSHTabIndex.value = sshTabs.value.length - 1
    activeTabIndex.value = -1
  }

  function closeSSHTab(index: number) {
    sshTabs.value.splice(index, 1)
    if (index === activeSSHTabIndex.value) {
      if (sshTabs.value.length === 0) {
        activeSSHTabIndex.value = -1
      } else if (index >= sshTabs.value.length) {
        activeSSHTabIndex.value = sshTabs.value.length - 1
      }
    } else if (index < activeSSHTabIndex.value) {
      activeSSHTabIndex.value--
    }
  }

  function promoteNewSSHTab(connId: string, label: string) {
    const idx = activeSSHTabIndex.value
    if (idx < 0 || idx >= sshTabs.value.length) return
    const tab = sshTabs.value[idx]
    if (!tab.isNew) return
    tab.connectionId = connId
    tab.label = label
    tab.isNew = false
  }

  function closeSSHTabByConnectionId(connId: string) {
    const idx = sshTabs.value.findIndex((t) => t.connectionId === connId)
    if (idx >= 0) {
      closeSSHTab(idx)
    }
  }

  function updateActiveSSHTabLabel(label: string) {
    const tab = activeSSHTab.value
    if (tab) {
      tab.label = label
    }
  }

  function setActiveSSHTab(index: number) {
    if (index >= 0 && index < sshTabs.value.length) {
      activeSSHTabIndex.value = index
      activeTabIndex.value = -1
    }
  }

  function activateToolTab(index: number) {
    if (index >= 0 && index < openTabs.value.length) {
      activeTabIndex.value = index
      activeSSHTabIndex.value = -1
    }
  }

  return {
    openTabs,
    activeTabIndex,
    sshTabs,
    activeSSHTabIndex,
    activeTabType,
    activeToolTab,
    activeExecutionConfig,
    activeSSHTab,
    activeTab,
    unifiedTabs,
    activateUnifiedTab,
    closeUnifiedTab,
    showSearch,
    showHotkeyHelp,
    showSettings,
    favorites,
    recentTools,
    toolHistory,
    settings,
    openTool,
    closeTab,
    updateRawArgs,
    setExecutionTarget,
    setRemoteConnection,
    setPythonEnv,
    setPanelMode,
    setActiveTab,
    setActiveSSHTab,
    activateToolTab,
    openSSHNew,
    openSSHEdit,
    closeSSHTab,
    promoteNewSSHTab,
    closeSSHTabByConnectionId,
    updateActiveSSHTabLabel,
    recordUsage,
    toggleFavorite,
    isFavorite,
    getHistory,
    getRecentArgs,
    resetAllData,
    defaultSettings,
  }
})
