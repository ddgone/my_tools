import { defineStore } from 'pinia'
import {
  CancelActiveRustToolchainTask,
  CheckRustToolchainEnvironment,
  DeleteManagedRustToolchainEnvironment,
  GetRustToolchainState,
  GetRustToolchainTaskState,
  ListOfficialRustReleases,
  ListOfficialZigReleases,
  SaveRustToolchainConfig,
  StartInstallRustCargoZigbuild,
  StartInstallRustTargets,
  StartInstallRustToolchain,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type {
  RustOfficialRelease,
  RustToolchainConfig,
  RustToolchainState,
  RustToolchainTaskState,
  ZigOfficialRelease,
} from '@/types/workbench'

function isRustInstallTask(task: RustToolchainTaskState | null) {
  return task?.kind?.startsWith('install') === true
}

function resolveRustInstallTaskKind(rustVersion: string, zigVersion: string): RustToolchainTaskState['kind'] {
  if (rustVersion && !zigVersion) {
    return 'install-rust'
  }
  if (!rustVersion && zigVersion) {
    return 'install-zig'
  }
  return 'install'
}

interface RustEnvState {
  state: RustToolchainState | null
  task: RustToolchainTaskState | null
  rustReleases: RustOfficialRelease[]
  zigReleases: ZigOfficialRelease[]
  loading: boolean
  saving: boolean
  installing: boolean
  checking: boolean
  deleting: boolean
  rustReleaseLoading: boolean
  zigReleaseLoading: boolean
  subscribed: boolean
  error: string
  rustReleaseError: string
  zigReleaseError: string
}

function createPendingTask(rustVersion: string, zigVersion: string, directory: string): RustToolchainTaskState {
  return {
    kind: resolveRustInstallTaskKind(rustVersion, zigVersion),
    status: 'running',
    message: '准备开始安装 Rust 交叉编译环境',
    progressPercent: 0,
    step: 0,
    totalSteps: 0,
    rustVersion,
    zigVersion,
    directory,
    updatedAt: Date.now(),
  }
}

function emptyConfig(): RustToolchainConfig {
  return {
    mode: 'auto',
    selectedRustRoot: '',
    knownRustRoots: [],
    selectedZigBinary: '',
    knownZigBinaries: [],
    lastInstallDirectory: '',
    disabled: false,
  }
}

export const useRustEnvStore = defineStore('rustenv', {
  state: (): RustEnvState => ({
    state: null,
    task: null,
    rustReleases: [],
    zigReleases: [],
    loading: false,
    saving: false,
    installing: false,
    checking: false,
    deleting: false,
    rustReleaseLoading: false,
    zigReleaseLoading: false,
    subscribed: false,
    error: '',
    rustReleaseError: '',
    zigReleaseError: '',
  }),
  getters: {
    hasUsableEnvironment(state) {
      return state.state?.hasUsableEnvironment === true
    },
    taskRunning(state) {
      return state.task?.status === 'running'
    },
  },
  actions: {
    ensureSubscriptions() {
      if (this.subscribed) {
        return
      }
      EventsOn('rust:toolchain:task', (task: RustToolchainTaskState | null) => {
        this.task = task
        const running = task?.status === 'running'
        this.installing = running && isRustInstallTask(task)
        if (!running) {
          void this.loadState()
        }
      })
      this.subscribed = true
    },
    async loadState() {
      this.ensureSubscriptions()
      this.loading = true
      this.error = ''
      try {
        const [state, task] = await Promise.all([GetRustToolchainState(), GetRustToolchainTaskState()])
        this.state = state
        this.task = task
        const running = task?.status === 'running'
        this.installing = running && isRustInstallTask(task)
      } catch (error) {
        this.error = error instanceof Error ? error.message : '加载 Rust 环境失败'
      } finally {
        this.loading = false
      }
    },
    async ensureRustReleases(force = false) {
      if ((!force && this.rustReleases.length > 0) || this.rustReleaseLoading) {
        return
      }
      this.rustReleaseLoading = true
      this.rustReleaseError = ''
      try {
        this.rustReleases = await ListOfficialRustReleases()
      } catch (error) {
        this.rustReleaseError = error instanceof Error ? error.message : '加载 Rust 版本列表失败'
      } finally {
        this.rustReleaseLoading = false
      }
    },
    async ensureZigReleases(force = false) {
      if ((!force && this.zigReleases.length > 0) || this.zigReleaseLoading) {
        return
      }
      this.zigReleaseLoading = true
      this.zigReleaseError = ''
      try {
        this.zigReleases = await ListOfficialZigReleases()
      } catch (error) {
        this.zigReleaseError = error instanceof Error ? error.message : '加载 Zig 版本列表失败'
      } finally {
        this.zigReleaseLoading = false
      }
    },
    async ensureReleases(force = false) {
      await Promise.all([this.ensureRustReleases(force), this.ensureZigReleases(force)])
    },
    async saveConfig(config: RustToolchainConfig) {
      this.saving = true
      this.error = ''
      try {
        this.state = await SaveRustToolchainConfig(config)
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '保存 Rust 设置失败'
        throw error
      } finally {
        this.saving = false
      }
    },
    async setMode(mode: 'auto' | 'manual' | 'none') {
      const current = this.state?.config ?? emptyConfig()
      const next: RustToolchainConfig = {
        ...current,
        knownRustRoots: [...(current.knownRustRoots ?? [])],
        knownZigBinaries: [...(current.knownZigBinaries ?? [])],
        mode,
        disabled: mode === 'none',
      }
      if (mode === 'auto') {
        next.selectedRustRoot = ''
        next.selectedZigBinary = ''
      }
      return this.saveConfig(next)
    },
    async chooseRustRoot(rootDir: string) {
      const current = this.state?.config ?? emptyConfig()
      const next: RustToolchainConfig = {
        ...current,
        knownRustRoots: [...(current.knownRustRoots ?? [])],
        knownZigBinaries: [...(current.knownZigBinaries ?? [])],
        mode: 'manual',
        disabled: false,
      }
      next.selectedRustRoot = rootDir
      next.knownRustRoots = Array.from(new Set([rootDir, ...next.knownRustRoots]))
      return this.saveConfig(next)
    },
    async chooseZigBinary(binaryPath: string) {
      const current = this.state?.config ?? emptyConfig()
      const next: RustToolchainConfig = {
        ...current,
        knownRustRoots: [...(current.knownRustRoots ?? [])],
        knownZigBinaries: [...(current.knownZigBinaries ?? [])],
        mode: 'manual',
        disabled: false,
      }
      next.selectedZigBinary = binaryPath
      next.knownZigBinaries = Array.from(new Set([binaryPath, ...next.knownZigBinaries]))
      return this.saveConfig(next)
    },
    async clearManualSelection(kind: 'rust' | 'zig') {
      const current = this.state?.config ?? emptyConfig()
      const next: RustToolchainConfig = {
        ...current,
        knownRustRoots: [...(current.knownRustRoots ?? [])],
        knownZigBinaries: [...(current.knownZigBinaries ?? [])],
        mode: 'manual',
        disabled: false,
      }
      if (kind === 'rust') {
        next.selectedRustRoot = ''
      } else {
        next.selectedZigBinary = ''
      }
      return this.saveConfig(next)
    },
    async enableAutoDetect() {
      return this.setMode('auto')
    },
    async disableEnvironment() {
      return this.setMode('none')
    },
    async install(rustVersion: string, zigVersion: string, directory: string) {
      this.ensureSubscriptions()
      this.installing = true
      this.error = ''
      try {
        this.task = createPendingTask(rustVersion, zigVersion, directory)
        this.task = await StartInstallRustToolchain({ rustVersion, zigVersion, directory })
        return this.task
      } catch (error) {
        this.error = error instanceof Error ? error.message : '安装 Rust 交叉编译环境失败'
        this.installing = false
        throw error
      }
    },
    async installCargoZigbuild() {
      this.ensureSubscriptions()
      this.installing = true
      this.error = ''
      try {
        this.task = {
          kind: 'cargo-zigbuild',
          status: 'running',
          message: '准备补齐 cargo-zigbuild',
          progressPercent: 0,
          step: 0,
          totalSteps: 0,
          updatedAt: Date.now(),
        }
        this.task = await StartInstallRustCargoZigbuild()
        return this.task
      } catch (error) {
        this.error = error instanceof Error ? error.message : '补齐 cargo-zigbuild 失败'
        this.installing = false
        throw error
      }
    },
    async installTargets() {
      this.ensureSubscriptions()
      this.installing = true
      this.error = ''
      try {
        this.task = {
          kind: 'targets',
          status: 'running',
          message: '准备补齐常用 Rust targets',
          progressPercent: 0,
          step: 0,
          totalSteps: 0,
          updatedAt: Date.now(),
        }
        this.task = await StartInstallRustTargets()
        return this.task
      } catch (error) {
        this.error = error instanceof Error ? error.message : '补齐常用 Rust targets 失败'
        this.installing = false
        throw error
      }
    },
    async checkEnvironment() {
      this.checking = true
      this.error = ''
      try {
        this.state = await CheckRustToolchainEnvironment()
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '检查 Rust 环境失败'
        throw error
      } finally {
        this.checking = false
      }
    },
    async cancelTask() {
      await CancelActiveRustToolchainTask()
    },
    async deleteEnvironment() {
      this.deleting = true
      this.error = ''
      try {
        this.state = await DeleteManagedRustToolchainEnvironment()
        this.task = null
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '删除 Rust 环境失败'
        throw error
      } finally {
        this.deleting = false
      }
    },
  },
})
