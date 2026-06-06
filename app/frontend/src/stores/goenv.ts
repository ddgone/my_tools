import { defineStore } from 'pinia'
import {
  GetGoToolchainState,
  InstallGoToolchain,
  ListOfficialGoReleases,
  SaveGoToolchainConfig,
} from '../../wailsjs/go/main/App'
import type { GoOfficialRelease, GoToolchainConfig, GoToolchainState } from '@/types/workbench'

interface GoEnvState {
  state: GoToolchainState | null
  releases: GoOfficialRelease[]
  loading: boolean
  saving: boolean
  installing: boolean
  releaseLoading: boolean
  error: string
  releaseError: string
}

export const useGoEnvStore = defineStore('goenv', {
  state: (): GoEnvState => ({
    state: null,
    releases: [],
    loading: false,
    saving: false,
    installing: false,
    releaseLoading: false,
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
  },
  actions: {
    async loadState() {
      this.loading = true
      this.error = ''
      try {
        this.state = await GetGoToolchainState()
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
      return this.saveConfig({
        selectedBinary: binaryPath,
        knownBinaries: Array.from(new Set([binaryPath, ...current.knownBinaries])),
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
      return this.saveConfig({
        selectedBinary: '',
        knownBinaries: current.knownBinaries,
        lastInstallDirectory: current.lastInstallDirectory,
        disabled: true,
      })
    },
    async install(version: string, directory: string) {
      this.installing = true
      this.error = ''
      try {
        this.state = await InstallGoToolchain({ version, directory })
        return this.state
      } catch (error) {
        this.error = error instanceof Error ? error.message : '安装 Go SDK 失败'
        throw error
      } finally {
        this.installing = false
      }
    },
  },
})
