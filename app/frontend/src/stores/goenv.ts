import { defineStore } from 'pinia'
import {
  CancelActiveGoToolchainTask,
  CheckGoToolchainEnvironment,
  DeleteGoToolchainEnvironment,
  GetGoToolchainState,
  GetGoToolchainTaskState,
  ListOfficialGoReleases,
  SaveGoToolchainConfig,
  StartInstallGoToolchain,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { GoOfficialRelease, GoToolchainConfig, GoToolchainState, GoToolchainTaskState } from '@/types/workbench'

interface GoEnvState {
  state: GoToolchainState | null
  task: GoToolchainTaskState | null
  releases: GoOfficialRelease[]
  loading: boolean
  saving: boolean
  installing: boolean
  checking: boolean
  deleting: boolean
  releaseLoading: boolean
  subscribed: boolean
  error: string
  releaseError: string
}

function createPendingTask(version: string, directory: string): GoToolchainTaskState {
  return {
    kind: 'install',
    status: 'running',
    message: '准备开始下载 Go SDK',
    progressPercent: 0,
    step: 0,
    totalSteps: 0,
    version,
    directory,
    updatedAt: Date.now(),
  }
}

export const useGoEnvStore = defineStore('goenv', {
  state: (): GoEnvState => ({
    state: null,
    task: null,
    releases: [],
    loading: false,
    saving: false,
    installing: false,
    checking: false,
    deleting: false,
    releaseLoading: false,
    subscribed: false,
    error: '',
    releaseError: '',
  }),
  getters: {
    hasUsableBinary(state) {
      return state.state?.hasUsableBinary === true
    },
    activeBinary(state) {
      return state.state?.activeBinary || ''
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
      EventsOn('go:toolchain:task', (task: GoToolchainTaskState | null) => {
        this.task = task
        const running = task?.status === 'running'
        this.installing = running && task?.kind === 'install'
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
        const [state, task] = await Promise.all([GetGoToolchainState(), GetGoToolchainTaskState()])
        this.state = state
        this.task = task
        const running = task?.status === 'running'
        this.installing = running && task?.kind === 'install'
      } catch (error) {
        this.error = error instanceof Error ? error.message : '加载 Go 环境失败'
      } finally {
        this.loading = false
      }
    },
    async ensureReleases() {
      if (this.releases.length > 0 || this.releaseLoading) {
        return
      }
      this.releaseLoading = true
      this.releaseError = ''
      try {
        this.releases = await ListOfficialGoReleases()
      } catch (error) {
        this.releaseError = error instanceof Error ? error.message : '加载 Go 版本列表失败'
      } finally {
        this.releaseLoading = false
      }
    },
    async saveConfig(config: GoToolchainConfig) {
      this.saving = true
      this.error = ''
      try {
        this.state = await SaveGoToolchainConfig(config)
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '保存 Go 设置失败'
        throw error
      } finally {
        this.saving = false
      }
    },
    async chooseBinary(binaryPath: string) {
      const current = this.state?.config ?? {
        selectedBinary: '',
        knownBinaries: [],
        lastInstallDirectory: '',
        disabled: false,
      }
      const knownBinaries = current.knownBinaries ?? []
      return this.saveConfig({
        selectedBinary: binaryPath,
        knownBinaries: Array.from(new Set([binaryPath, ...knownBinaries])),
        lastInstallDirectory: current.lastInstallDirectory,
        disabled: false,
      })
    },
    async clearSelection() {
      const current = this.state?.config ?? {
        selectedBinary: '',
        knownBinaries: [],
        lastInstallDirectory: '',
        disabled: false,
      }
      const knownBinaries = current.knownBinaries ?? []
      return this.saveConfig({
        selectedBinary: '',
        knownBinaries,
        lastInstallDirectory: current.lastInstallDirectory,
        disabled: true,
      })
    },
    async install(version: string, directory: string) {
      this.ensureSubscriptions()
      this.installing = true
      this.error = ''
      try {
        this.task = createPendingTask(version, directory)
        this.task = await StartInstallGoToolchain({ version, directory })
        return this.task
      } catch (error) {
        this.error = error instanceof Error ? error.message : '安装 Go SDK 失败'
        this.installing = false
        throw error
      }
    },
    async checkEnvironment() {
      this.checking = true
      this.error = ''
      try {
        this.state = await CheckGoToolchainEnvironment()
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '检查 Go 环境失败'
        throw error
      } finally {
        this.checking = false
      }
    },
    async cancelTask() {
      await CancelActiveGoToolchainTask()
    },
    async deleteEnvironment() {
      this.deleting = true
      this.error = ''
      try {
        this.state = await DeleteGoToolchainEnvironment()
        this.task = null
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '删除 Go 环境失败'
        throw error
      } finally {
        this.deleting = false
      }
    },
  },
})
