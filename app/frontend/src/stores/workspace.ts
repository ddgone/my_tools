import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import type { SSHConnection, ToolManifest } from '@/types/workbench'
import type { BuiltinToolDefinition, BuiltinToolTabState } from '@/types/builtin'
import { buildRawArgs } from '@/utils/cliArgs'

export interface ToolTabState {
  tabId: string
  toolId: string
  executionTarget: ExecutionTarget
  localConfig: ToolExecutionConfig
  remoteConfig: RemoteExecutionConfig
  terminalVisible: boolean
  terminalHeight?: number
  exportTarget: string
  openedAt: number
}

export type ExecutionTarget = 'local' | 'remote'
export type ToolPanelMode = 'form' | 'cli' | 'docs' | 'remote'
export type SettingsTab = 'general' | 'export' | 'go' | 'rust' | 'python'
export type GoExportMode = 'binary' | 'source'
export type ThemePreference = 'dark' | 'light' | 'system'

export interface ToolExecutionConfig {
  panelMode: ToolPanelMode
  rawArgs: string
  pythonEnv: string
  formModel: Record<string, string | number | boolean | null>
}

export interface RemoteExecutionConfig extends ToolExecutionConfig {
  connId: string
  lastBrowsePath: string
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
  autoOpenExportDir: boolean
  goExportMode: GoExportMode
  themePreference: ThemePreference
  lastSettingsTab: SettingsTab
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
  defaultPythonPath: 'python3',
  confirmExit: false,
  autoWordWrap: true,
  autoExpandAll: false,
  verboseShortcuts: true,
  bgmEnabled: false,
  autoOpenExportDir: true,
  goExportMode: 'binary',
  themePreference: 'dark',
  lastSettingsTab: 'general',
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
  exportTarget: string
  updatedAt: number
}

function normalizeSettingsTab(value: unknown): SettingsTab {
  if (value === 'export' || value === 'go' || value === 'rust' || value === 'python') {
    return value
  }
  return 'general'
}

function normalizeGoExportMode(value: unknown): GoExportMode {
  return value === 'source' ? 'source' : 'binary'
}

function normalizeThemePreference(value: unknown): ThemePreference {
  return value === 'light' || value === 'system' ? value : 'dark'
}

function normalizeUserSettings(source?: Partial<UserSettings>): UserSettings {
  return {
    ...defaultSettings,
    ...source,
    autoOpenExportDir: source?.autoOpenExportDir !== false,
    goExportMode: normalizeGoExportMode(source?.goExportMode),
    themePreference: normalizeThemePreference(source?.themePreference),
    lastSettingsTab: normalizeSettingsTab(source?.lastSettingsTab),
  }
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
    exportTarget: typeof persistedState?.exportTarget === 'string' ? persistedState.exportTarget : '',
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
    lastBrowsePath: config.lastBrowsePath,
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
    lastBrowsePath: '',
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
    lastBrowsePath: typeof source?.lastBrowsePath === 'string' ? source.lastBrowsePath : '',
  }
}

function snapshotToolState(tab: ToolTabState): PersistedToolState {
  return {
    executionTarget: tab.executionTarget,
    localConfig: cloneExecutionConfig(tab.localConfig),
    remoteConfig: cloneRemoteExecutionConfig(tab.remoteConfig),
    terminalVisible: tab.terminalVisible,
    terminalHeight: tab.terminalHeight,
    exportTarget: tab.exportTarget,
    updatedAt: Date.now(),
  }
}

export interface SSHTabState {
  tabId: string
  connectionId: string
  label: string
  isNew: boolean
  openedAt: number
}

export interface ArtifactCenterTabState {
  tabId: string
  label: string
  view: 'center' | 'snapshot'
  taskId?: string
  openedAt: number
}

function createBuiltinTabState(tool: BuiltinToolDefinition): BuiltinToolTabState {
  return {
    tabId: `builtin_${tool.id}`,
    builtinToolId: tool.id,
    openedAt: Date.now(),
  }
}

function formatNewSSHLabel(savedCount: number): string {
  return savedCount <= 0 ? '新建连接' : `新建连接 ${savedCount + 1}`
}

export const useWorkspaceStore = defineStore('workspace', () => {
  const openTabs = ref<ToolTabState[]>([])
  const activeTabIndex = ref(-1)
  const builtinTabs = ref<BuiltinToolTabState[]>([])
  const activeBuiltinTabIndex = ref(-1)
  const sshTabs = ref<SSHTabState[]>([])
  const activeSSHTabIndex = ref(-1)
  const artifactTabs = ref<ArtifactCenterTabState[]>([])
  const activeArtifactTabIndex = ref(-1)
  const showSearch = ref(false)
  const showHotkeyHelp = ref(false)
  const showSettings = ref(false)

  const favorites = ref<string[]>(loadJSON<string[]>(STORAGE_KEYS.favorites, []))
  const recentTools = ref<RecentEntry[]>(loadJSON<RecentEntry[]>(STORAGE_KEYS.recent, []))
  const toolHistory = ref<Record<string, HistoryEntry[]>>(loadJSON<Record<string, HistoryEntry[]>>(STORAGE_KEYS.history, {}))
  const settings = ref<UserSettings>(normalizeUserSettings(loadJSON<Partial<UserSettings>>(STORAGE_KEYS.settings, {})))
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
    if (activeArtifactTabIndex.value >= 0) return 'artifact'
    if (activeBuiltinTabIndex.value >= 0) return 'builtin'
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

  const activeBuiltinTab = computed(() =>
    activeBuiltinTabIndex.value >= 0 ? builtinTabs.value[activeBuiltinTabIndex.value] : undefined,
  )

  const activeArtifactTab = computed(() =>
    activeArtifactTabIndex.value >= 0 ? artifactTabs.value[activeArtifactTabIndex.value] : undefined,
  )

  const activeTab = () => (activeTabIndex.value >= 0 ? openTabs.value[activeTabIndex.value] : undefined)

  interface UnifiedTabItem {
    type: 'tool' | 'builtin' | 'ssh' | 'artifact'
    key: string
    label: string
    openedAt: number
    arrayIndex: number
    pinned: boolean
  }

  function isPersistentTabLayoutKey(key: string): boolean {
    if (key === 'artifact:artifact_center') {
      return true
    }
    if (key.startsWith('builtin:builtin_')) {
      return true
    }
    if (key.startsWith('tool:tool_')) {
      return true
    }
    return key.startsWith('ssh:ssh_') && !key.startsWith('ssh:ssh_new_')
  }

  function buildCurrentTabKeys(): string[] {
    return [
      ...artifactTabs.value.map((tab) => `artifact:${tab.tabId}`),
      ...builtinTabs.value.map((tab) => `builtin:${tab.tabId}`),
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

  watch([artifactTabs, builtinTabs, openTabs, sshTabs], () => {
    reconcileTabLayout()
  }, { deep: true, immediate: true })

  const unifiedTabs = computed<UnifiedTabItem[]>(() => {
    const orderIndex = new Map(tabOrder.value.map((key, index) => [key, index]))
    const pinnedKeySet = new Set(pinnedTabs.value)
    const items: UnifiedTabItem[] = [
      ...artifactTabs.value.map((tab, i) => ({
        type: 'artifact' as const,
        key: `artifact:${tab.tabId}`,
        label: tab.label,
        openedAt: tab.openedAt,
        arrayIndex: i,
        pinned: pinnedKeySet.has(`artifact:${tab.tabId}`),
      })),
      ...builtinTabs.value.map((tab, i) => ({
        type: 'builtin' as const,
        key: `builtin:${tab.tabId}`,
        label: tab.builtinToolId,
        openedAt: tab.openedAt,
        arrayIndex: i,
        pinned: pinnedKeySet.has(`builtin:${tab.tabId}`),
      })),
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
      activeBuiltinTabIndex.value = -1
      activeArtifactTabIndex.value = -1
    } else if (item.type === 'builtin') {
      activeBuiltinTabIndex.value = item.arrayIndex
      activeTabIndex.value = -1
      activeSSHTabIndex.value = -1
      activeArtifactTabIndex.value = -1
    } else if (item.type === 'ssh') {
      activeSSHTabIndex.value = item.arrayIndex
      activeTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeArtifactTabIndex.value = -1
    } else {
      activeArtifactTabIndex.value = item.arrayIndex
      activeTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeSSHTabIndex.value = -1
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
    } else if (item.type === 'builtin') {
      builtinTabs.value.splice(item.arrayIndex, 1)
      if (item.arrayIndex === activeBuiltinTabIndex.value) {
        if (builtinTabs.value.length === 0) {
          activeBuiltinTabIndex.value = -1
        } else if (item.arrayIndex >= builtinTabs.value.length) {
          activeBuiltinTabIndex.value = builtinTabs.value.length - 1
        }
      } else if (item.arrayIndex < activeBuiltinTabIndex.value) {
        activeBuiltinTabIndex.value--
      }
    } else if (item.type === 'ssh') {
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
    } else {
      artifactTabs.value.splice(item.arrayIndex, 1)
      if (item.arrayIndex === activeArtifactTabIndex.value) {
        if (artifactTabs.value.length === 0) {
          activeArtifactTabIndex.value = -1
        } else if (item.arrayIndex >= artifactTabs.value.length) {
          activeArtifactTabIndex.value = artifactTabs.value.length - 1
        }
      } else if (item.arrayIndex < activeArtifactTabIndex.value) {
        activeArtifactTabIndex.value--
      }
    }
  }

  function openArtifactCenter() {
    const existing = artifactTabs.value.findIndex((tab) => tab.tabId === 'artifact_center')
    if (existing >= 0) {
      activeArtifactTabIndex.value = existing
      activeTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeSSHTabIndex.value = -1
      return
    }
    artifactTabs.value.push({
      tabId: 'artifact_center',
      label: '产物中心',
      view: 'center',
      openedAt: Date.now(),
    })
    activeArtifactTabIndex.value = artifactTabs.value.length - 1
    activeTabIndex.value = -1
    activeBuiltinTabIndex.value = -1
    activeSSHTabIndex.value = -1
  }

  function openArtifactSnapshot(taskId: string, label: string) {
    const tabId = `artifact_task_${taskId}`
    const existing = artifactTabs.value.findIndex((tab) => tab.tabId === tabId)
    if (existing >= 0) {
      artifactTabs.value[existing].label = label
      activeArtifactTabIndex.value = existing
      activeTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeSSHTabIndex.value = -1
      return
    }
    artifactTabs.value.push({
      tabId,
      label,
      view: 'snapshot',
      taskId,
      openedAt: Date.now(),
    })
    activeArtifactTabIndex.value = artifactTabs.value.length - 1
    activeTabIndex.value = -1
    activeBuiltinTabIndex.value = -1
    activeSSHTabIndex.value = -1
  }

  function openTool(tool: ToolManifest) {
    const existing = openTabs.value.findIndex((t) => t.toolId === tool.id)
    if (existing >= 0) {
      activeTabIndex.value = existing
      activeBuiltinTabIndex.value = -1
      activeSSHTabIndex.value = -1
      activeArtifactTabIndex.value = -1
      return
    }

    const history = toolHistory.value[tool.id] || []
    const lastEntry = history[0]
    const persistedState = persistedToolStates.value[tool.id]

    const tab = createTabState(tool, persistedState, lastEntry, settings.value.defaultPythonPath)
    openTabs.value.push(tab)
    activeTabIndex.value = openTabs.value.length - 1
    activeBuiltinTabIndex.value = -1
    activeSSHTabIndex.value = -1
    activeArtifactTabIndex.value = -1
  }

  function openBuiltinTool(tool: BuiltinToolDefinition) {
    const existing = builtinTabs.value.findIndex((tab) => tab.builtinToolId === tool.id)
    if (existing >= 0) {
      activeBuiltinTabIndex.value = existing
      activeTabIndex.value = -1
      activeSSHTabIndex.value = -1
      activeArtifactTabIndex.value = -1
      return
    }

    builtinTabs.value.push(createBuiltinTabState(tool))
    activeBuiltinTabIndex.value = builtinTabs.value.length - 1
    activeTabIndex.value = -1
    activeSSHTabIndex.value = -1
    activeArtifactTabIndex.value = -1
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

  function setRemoteBrowsePath(index: number, value: string) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].remoteConfig.lastBrowsePath = value.trim()
  }

  function setPythonEnv(index: number, value: string) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    const tab = openTabs.value[index]
    const config = tab.executionTarget === 'remote' ? tab.remoteConfig : tab.localConfig
    config.pythonEnv = value
  }

  function setExportTarget(index: number, value: string) {
    if (index < 0 || index >= openTabs.value.length) {
      return
    }
    openTabs.value[index].exportTarget = value
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
    settings.value = normalizeUserSettings()
    Object.values(STORAGE_KEYS).forEach((k) => localStorage.removeItem(k))
  }

  function openSettings(tab?: SettingsTab) {
    if (tab) {
      settings.value.lastSettingsTab = normalizeSettingsTab(tab)
    }
    showSettings.value = true
  }

  function openSSHNew(savedCount: number) {
    const existing = sshTabs.value.findIndex((t) => t.isNew && !t.connectionId)
    if (existing >= 0) {
      activeSSHTabIndex.value = existing
      activeTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeArtifactTabIndex.value = -1
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
    activeBuiltinTabIndex.value = -1
    activeArtifactTabIndex.value = -1
  }

  function openSSHEdit(connId: string, label: string) {
    const existing = sshTabs.value.findIndex((t) => t.connectionId === connId)
    if (existing >= 0) {
      activeSSHTabIndex.value = existing
      activeTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeArtifactTabIndex.value = -1
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
    activeBuiltinTabIndex.value = -1
    activeArtifactTabIndex.value = -1
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

  function restorePinnedTabs(
    tools: ToolManifest[],
    sshConnections: SSHConnection[] = [],
    builtinTools: BuiltinToolDefinition[] = [],
  ) {
    if (pinnedTabsRestored.value) return
    pinnedTabsRestored.value = true

    const toolById = new Map(tools.map((tool) => [tool.id, tool]))
    const sshById = new Map(sshConnections.map((conn) => [conn.id, conn]))
    const builtinById = new Map(builtinTools.map((tool) => [tool.id, tool]))

    for (const key of orderedPinnedTabKeys()) {
      if (key.startsWith('builtin:builtin_')) {
        const builtinToolId = key.slice('builtin:builtin_'.length)
        const builtinTool = builtinById.get(builtinToolId as BuiltinToolDefinition['id'])
        if (builtinTool) {
          openBuiltinTool(builtinTool)
        }
        continue
      }

      if (key.startsWith('tool:tool_')) {
        const toolId = key.slice('tool:tool_'.length)
        const tool = toolById.get(toolId)
        if (tool) {
          openTool(tool)
        }
        continue
      }

      if (key === 'artifact:artifact_center') {
        openArtifactCenter()
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
      activeBuiltinTabIndex.value = -1
      activeArtifactTabIndex.value = -1
    }
  }

  function activateToolTab(index: number) {
    if (index >= 0 && index < openTabs.value.length) {
      activeTabIndex.value = index
      activeSSHTabIndex.value = -1
      activeBuiltinTabIndex.value = -1
      activeArtifactTabIndex.value = -1
    }
  }

  return {
    openTabs,
    activeTabIndex,
    builtinTabs,
    activeBuiltinTabIndex,
    sshTabs,
    activeSSHTabIndex,
    artifactTabs,
    activeArtifactTabIndex,
    activeTabType,
    activeToolTab,
    activeToolTerminalVisible,
    activeToolTerminalHeight,
    activeExecutionConfig,
    activeSSHTab,
    activeBuiltinTab,
    activeArtifactTab,
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
    openSettings,
    favorites,
    recentTools,
    toolHistory,
    settings,
    openTool,
    openBuiltinTool,
    closeTab,
    updateRawArgs,
    setExecutionTarget,
    setRemoteConnection,
    setRemoteBrowsePath,
    setPythonEnv,
    setExportTarget,
    setPanelMode,
    setTerminalVisible,
    toggleTerminalVisible,
    setTerminalHeight,
    setActiveTab,
    setActiveSSHTab,
    activateToolTab,
    openArtifactCenter,
    openArtifactSnapshot,
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
