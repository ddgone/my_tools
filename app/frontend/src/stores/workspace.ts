import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type { SSHConnection, ToolManifest } from '@/types/workbench'
import { getVisibleParams, shouldEmitParam } from '@/utils/toolParams'

export interface ToolTabState {
  tabId: string
  toolId: string
  executionTarget: ExecutionTarget
  localConfig: ToolExecutionConfig
  remoteConfig: RemoteExecutionConfig
  terminalVisible: boolean
  terminalHeight?: number
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
  pinnedTabs: 'fire-salamander:pinned-tabs',
  tabOrder: 'fire-salamander:tab-order',
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
  terminalVisible: boolean
  terminalHeight?: number
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
    terminalVisible: persistedState?.terminalVisible === true,
    terminalHeight:
      typeof persistedState?.terminalHeight === 'number' && persistedState.terminalHeight > 0
        ? persistedState.terminalHeight
        : undefined,
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
    terminalVisible: tab.terminalVisible,
    terminalHeight: tab.terminalHeight,
    updatedAt: Date.now(),
  }
}

function buildRawArgs(tool: ToolManifest, formModel: Record<string, string | number | boolean | null>): string {
  const parts: string[] = []
  for (const param of getVisibleParams(tool, formModel)) {
    if (!shouldEmitParam(param)) {
      continue
    }

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
  const pinnedTabs = ref<string[]>(loadJSON<string[]>(STORAGE_KEYS.pinnedTabs, []))
  const tabOrder = ref<string[]>(loadJSON<string[]>(STORAGE_KEYS.tabOrder, []))
  const pinnedTabsRestored = ref(false)

  watch(favorites, (v) => saveJSON(STORAGE_KEYS.favorites, v), { deep: true })
  watch(recentTools, (v) => saveJSON(STORAGE_KEYS.recent, v), { deep: true })
  watch(toolHistory, (v) => saveJSON(STORAGE_KEYS.history, v), { deep: true })
  watch(settings, (v) => saveJSON(STORAGE_KEYS.settings, v), { deep: true })
  watch(persistedToolStates, (v) => saveJSON(STORAGE_KEYS.toolState, v), { deep: true })
  watch(pinnedTabs, (v) => saveJSON(STORAGE_KEYS.pinnedTabs, v), { deep: true })
  watch(tabOrder, (v) => saveJSON(STORAGE_KEYS.tabOrder, v), { deep: true })
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

  const activeToolTerminalVisible = computed(() => activeToolTab.value?.terminalVisible === true)
  const activeToolTerminalHeight = computed(() => activeToolTab.value?.terminalHeight)

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
    pinned: boolean
  }

  function isPersistentTabLayoutKey(key: string): boolean {
    if (key.startsWith('tool:tool_')) {
      return true
    }
    return key.startsWith('ssh:ssh_') && !key.startsWith('ssh:ssh_new_')
  }

  function buildCurrentTabKeys(): string[] {
    return [
      ...openTabs.value.map((tab) => `tool:${tab.tabId}`),
      ...sshTabs.value.map((tab) => `ssh:${tab.tabId}`),
    ]
  }

  function reconcileTabLayout() {
    const currentKeys = buildCurrentTabKeys()
    const existingKeys = new Set(currentKeys)

    const nextPinned = pinnedTabs.value.filter((key) => existingKeys.has(key) || isPersistentTabLayoutKey(key))
    if (nextPinned.length !== pinnedTabs.value.length || nextPinned.some((key, index) => key !== pinnedTabs.value[index])) {
      pinnedTabs.value = nextPinned
    }

    const persistentPinnedSet = new Set(nextPinned.filter((key) => isPersistentTabLayoutKey(key)))
    const nextOrder = tabOrder.value.filter((key) => existingKeys.has(key) || persistentPinnedSet.has(key))
    for (const key of currentKeys) {
      if (!nextOrder.includes(key)) {
        nextOrder.push(key)
      }
    }
    if (nextOrder.length !== tabOrder.value.length || nextOrder.some((key, index) => key !== tabOrder.value[index])) {
      tabOrder.value = nextOrder
    }
  }

  watch([openTabs, sshTabs], () => {
    reconcileTabLayout()
  }, { deep: true, immediate: true })

  const unifiedTabs = computed<UnifiedTabItem[]>(() => {
    const orderIndex = new Map(tabOrder.value.map((key, index) => [key, index]))
    const pinnedKeySet = new Set(pinnedTabs.value)
    const items: UnifiedTabItem[] = [
      ...openTabs.value.map((t, i) => ({
        type: 'tool' as const,
        key: `tool:${t.tabId}`,
        label: t.toolId,
        openedAt: t.openedAt,
        arrayIndex: i,
        pinned: pinnedKeySet.has(`tool:${t.tabId}`),
      })),
      ...sshTabs.value.map((s, i) => ({
        type: 'ssh' as const,
        key: `ssh:${s.tabId}`,
        label: s.label,
        openedAt: s.openedAt,
        arrayIndex: i,
        pinned: pinnedKeySet.has(`ssh:${s.tabId}`),
      })),
    ]
    items.sort((a, b) => {
      if (a.pinned !== b.pinned) {
        return a.pinned ? -1 : 1
      }
      const aOrder = orderIndex.get(a.key)
      const bOrder = orderIndex.get(b.key)
      if (aOrder !== undefined && bOrder !== undefined && aOrder !== bOrder) {
        return aOrder - bOrder
      }
      if (aOrder !== undefined && bOrder === undefined) return -1
      if (aOrder === undefined && bOrder !== undefined) return 1
      return a.openedAt - b.openedAt
    })
    return items
  })

  function isTabPinned(key: string): boolean {
    return pinnedTabs.value.includes(key)
  }

  function orderedPinnedTabKeys() {
    const pinnedKeySet = new Set(pinnedTabs.value)
    const orderedKeys = tabOrder.value.filter((key) => pinnedKeySet.has(key))
    for (const key of pinnedTabs.value) {
      if (!orderedKeys.includes(key)) {
        orderedKeys.push(key)
      }
    }
    return orderedKeys
  }

  function setTabPinned(key: string, pinned: boolean) {
    const current = isTabPinned(key)
    if (current === pinned) return
    if (pinned) {
      const nextPinned = pinnedTabs.value.filter((item) => item !== key)
      nextPinned.push(key)
      pinnedTabs.value = nextPinned

      const currentKeys = buildCurrentTabKeys()
      const currentKeySet = new Set(currentKeys)
      const persistentPinnedSet = new Set(nextPinned.filter((item) => isPersistentTabLayoutKey(item)))
      const nextOrder = tabOrder.value.filter((item) =>
        item !== key && (currentKeySet.has(item) || persistentPinnedSet.has(item)),
      )
      const pinnedWithoutCurrent = nextPinned.filter((item) => item !== key)
      const lastPinnedKey = pinnedWithoutCurrent[pinnedWithoutCurrent.length - 1]
      if (lastPinnedKey) {
        const lastPinnedIndex = nextOrder.indexOf(lastPinnedKey)
        nextOrder.splice(lastPinnedIndex + 1, 0, key)
      } else {
        nextOrder.unshift(key)
      }
      tabOrder.value = nextOrder
      return
    }
    pinnedTabs.value = pinnedTabs.value.filter((item) => item !== key)
  }

  function toggleTabPinned(key: string) {
    setTabPinned(key, !isTabPinned(key))
  }

  function moveUnifiedTab(dragKey: string, targetKey: string, placement: 'before' | 'after' = 'before') {
    if (dragKey === targetKey) return
    const currentKeys = buildCurrentTabKeys()
    if (!currentKeys.includes(dragKey) || !currentKeys.includes(targetKey)) return

    const pinnedKeySet = new Set(pinnedTabs.value)
    const dragPinned = pinnedKeySet.has(dragKey)
    const targetPinned = pinnedKeySet.has(targetKey)
    if (dragPinned !== targetPinned) return

    const currentKeySet = new Set(currentKeys)
    const persistentPinnedSet = new Set(pinnedTabs.value.filter((key) => isPersistentTabLayoutKey(key)))
    const nextOrder = tabOrder.value.filter((key) => currentKeySet.has(key) || persistentPinnedSet.has(key))
    if (!nextOrder.includes(dragKey)) nextOrder.push(dragKey)
    if (!nextOrder.includes(targetKey)) nextOrder.push(targetKey)

    const dragIndex = nextOrder.indexOf(dragKey)
    const targetIndex = nextOrder.indexOf(targetKey)
    if (dragIndex < 0 || targetIndex < 0) return

    nextOrder.splice(dragIndex, 1)
    const adjustedTargetIndex = nextOrder.indexOf(targetKey)
    nextOrder.splice(placement === 'after' ? adjustedTargetIndex + 1 : adjustedTargetIndex, 0, dragKey)
    tabOrder.value = nextOrder
  }

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

  function setTerminalVisible(index: number, value: boolean) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].terminalVisible = value
  }

  function toggleTerminalVisible(index: number) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].terminalVisible = !openTabs.value[index].terminalVisible
  }

  function setTerminalHeight(index: number, value: number | undefined) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].terminalHeight =
      typeof value === 'number' && Number.isFinite(value) && value > 0 ? value : undefined
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

  function getPersistedToolState(toolId: string): PersistedToolState | undefined {
    return persistedToolStates.value[toolId]
  }

  function resetAllData() {
    favorites.value = []
    recentTools.value = []
    toolHistory.value = {}
    persistedToolStates.value = {}
    pinnedTabs.value = []
    tabOrder.value = []
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

  function restorePinnedTabs(tools: ToolManifest[], sshConnections: SSHConnection[] = []) {
    if (pinnedTabsRestored.value) return
    pinnedTabsRestored.value = true

    const toolById = new Map(tools.map((tool) => [tool.id, tool]))
    const sshById = new Map(sshConnections.map((conn) => [conn.id, conn]))

    for (const key of orderedPinnedTabKeys()) {
      if (key.startsWith('tool:tool_')) {
        const toolId = key.slice('tool:tool_'.length)
        const tool = toolById.get(toolId)
        if (tool) {
          openTool(tool)
        }
        continue
      }

      if (key.startsWith('ssh:ssh_')) {
        const connId = key.slice('ssh:ssh_'.length)
        const conn = sshById.get(connId)
        if (conn) {
          openSSHEdit(conn.id, conn.name)
        }
      }
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
    activeToolTerminalVisible,
    activeToolTerminalHeight,
    activeExecutionConfig,
    activeSSHTab,
    activeTab,
    unifiedTabs,
    activateUnifiedTab,
    closeUnifiedTab,
    moveUnifiedTab,
    pinnedTabs,
    tabOrder,
    isTabPinned,
    setTabPinned,
    toggleTabPinned,
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
    setTerminalVisible,
    toggleTerminalVisible,
    setTerminalHeight,
    setActiveTab,
    setActiveSSHTab,
    activateToolTab,
    openSSHNew,
    openSSHEdit,
    restorePinnedTabs,
    closeSSHTab,
    promoteNewSSHTab,
    closeSSHTabByConnectionId,
    updateActiveSSHTabLabel,
    recordUsage,
    toggleFavorite,
    isFavorite,
    getHistory,
    getRecentArgs,
    getPersistedToolState,
    resetAllData,
    defaultSettings,
  }
})
