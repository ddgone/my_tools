import { defineStore } from 'pinia'
import {
  CancelActivePythonToolchainTask,
  CheckPythonToolchainEnvironment,
  DeletePythonToolchainEnvironment,
  GetPythonToolchainState,
  GetPythonToolchainTaskState,
  SavePythonToolchainConfig,
  StartInstallPythonDependencies,
  StartPreparePythonToolchainEnvironment,
} from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { PythonToolchainConfig, PythonToolchainState, PythonToolchainTaskState } from '@/types/workbench'

interface PythonEnvState {
  state: PythonToolchainState | null
  task: PythonToolchainTaskState | null
  loading: boolean
  saving: boolean
  preparing: boolean
  installing: boolean
  checking: boolean
  deleting: boolean
  subscribed: boolean
  error: string
}

function createPendingTask(kind: 'prepare' | 'install'): PythonToolchainTaskState {
  return {
    kind,
    status: 'running',
    message: kind === 'install' ? '准备开始安装依赖' : '准备开始创建工具环境',
    progressPercent: 0,
    step: 0,
    totalSteps: 0,
    updatedAt: Date.now(),
  }
}

export const usePythonEnvStore = defineStore('pythonenv', {
  state: (): PythonEnvState => ({
    state: null,
    task: null,
    loading: false,
    saving: false,
    preparing: false,
    installing: false,
    checking: false,
    deleting: false,
    subscribed: false,
    error: '',
  }),
  getters: {
    hasUsableBaseBinary(state) {
      return state.state?.hasUsableBaseBinary === true
    },
    hasUsableBinary(state) {
      return state.state?.hasUsableBinary === true
    },
    dependenciesReady(state) {
      return state.state?.dependenciesReady === true
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
      EventsOn('python:toolchain:task', (task: PythonToolchainTaskState | null) => {
        this.task = task
        const running = task?.status === 'running'
        this.preparing = running && task?.kind === 'prepare'
        this.installing = running && task?.kind === 'install'
        if (task?.kind === 'install' && task?.message?.startsWith('已安装依赖 ')) {
          void this.loadState()
          return
        }
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
        const [state, task] = await Promise.all([GetPythonToolchainState(), GetPythonToolchainTaskState()])
        this.state = state
        this.task = task
        const running = task?.status === 'running'
        this.preparing = running && task?.kind === 'prepare'
        this.installing = running && task?.kind === 'install'
      } catch (error) {
        this.error = error instanceof Error ? error.message : '加载 Python 环境失败'
      } finally {
        this.loading = false
      }
    },
    async saveConfig(config: PythonToolchainConfig) {
      this.saving = true
      this.error = ''
      try {
        this.state = await SavePythonToolchainConfig(config)
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '保存 Python 设置失败'
        throw error
      } finally {
        this.saving = false
      }
    },
    async prepareEnvironment() {
      this.ensureSubscriptions()
      this.preparing = true
      this.error = ''
      try {
        this.task = createPendingTask('prepare')
        this.task = await StartPreparePythonToolchainEnvironment()
        return this.task
      } catch (error) {
        this.error = error instanceof Error ? error.message : '创建 Python 工具环境失败'
        this.preparing = false
        throw error
      }
    },
    async chooseBinary(binaryPath: string) {
      const current = this.state?.config ?? {
        selectedBinary: '',
        knownBinaries: [],
        disabled: false,
      }
      const knownBinaries = current.knownBinaries ?? []
      await this.saveConfig({
        selectedBinary: binaryPath,
        knownBinaries: Array.from(new Set([binaryPath, ...knownBinaries])),
        disabled: false,
      })
      return this.state
    },
    async clearSelection() {
      const current = this.state?.config ?? {
        selectedBinary: '',
        knownBinaries: [],
        disabled: false,
      }
      const knownBinaries = current.knownBinaries ?? []
      return this.saveConfig({
        selectedBinary: '',
        knownBinaries,
        disabled: true,
      })
    },
    async installDependencies() {
      this.ensureSubscriptions()
      this.installing = true
      this.error = ''
      try {
        this.task = createPendingTask('install')
        this.task = await StartInstallPythonDependencies()
        return this.task
      } catch (error) {
        this.error = error instanceof Error ? error.message : '安装 Python 依赖失败'
        this.installing = false
        throw error
      }
    },
    async cancelTask() {
      await CancelActivePythonToolchainTask()
    },
    async checkEnvironment() {
      this.checking = true
      this.error = ''
      try {
        this.state = await CheckPythonToolchainEnvironment()
        return this.state
      } catch (error) {
        if (error instanceof Error) {
          this.error = error.message
        } else {
          this.error = '检查 Python 工具环境失败'
        }
        throw error
      } finally {
        this.checking = false
      }
    },
    async deleteEnvironment() {
      this.deleting = true
      this.error = ''
      try {
        this.state = await DeletePythonToolchainEnvironment()
        this.task = null
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '删除 Python 工具环境失败'
        throw error
      } finally {
        this.deleting = false
      }
    },
  },
})
