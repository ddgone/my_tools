import { defineStore } from 'pinia'
import {
  CheckRustToolchainEnvironment,
  GetRustToolchainState,
  SaveRustToolchainConfig,
} from '../../wailsjs/go/main/App'
import type { RustToolchainConfig, RustToolchainState } from '@/types/workbench'

interface RustEnvState {
  state: RustToolchainState | null
  loading: boolean
  saving: boolean
  checking: boolean
  error: string
}

function emptyConfig(): RustToolchainConfig {
  return {
    selectedCargoBinary: '',
    knownCargoBinaries: [],
    selectedRustupBinary: '',
    knownRustupBinaries: [],
    selectedZigBinary: '',
    knownZigBinaries: [],
    selectedCargoZigbuildBinary: '',
    knownCargoZigbuildBinaries: [],
  }
}

export const useRustEnvStore = defineStore('rustenv', {
  state: (): RustEnvState => ({
    state: null,
    loading: false,
    saving: false,
    checking: false,
    error: '',
  }),
  getters: {
    hasUsableEnvironment(state) {
      return state.state?.hasUsableEnvironment === true
    },
  },
  actions: {
    async loadState() {
      this.loading = true
      this.error = ''
      try {
        this.state = await GetRustToolchainState()
      } catch (error) {
        this.error = error instanceof Error ? error.message : '加载 Rust 环境失败'
      } finally {
        this.loading = false
      }
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
    async chooseBinary(kind: 'cargo' | 'rustup' | 'zig' | 'cargo-zigbuild', binaryPath: string) {
      const current = this.state?.config ?? emptyConfig()
      const next: RustToolchainConfig = {
        ...current,
        knownCargoBinaries: [...(current.knownCargoBinaries ?? [])],
        knownRustupBinaries: [...(current.knownRustupBinaries ?? [])],
        knownZigBinaries: [...(current.knownZigBinaries ?? [])],
        knownCargoZigbuildBinaries: [...(current.knownCargoZigbuildBinaries ?? [])],
      }
      switch (kind) {
        case 'cargo':
          next.selectedCargoBinary = binaryPath
          next.knownCargoBinaries = Array.from(new Set([binaryPath, ...next.knownCargoBinaries]))
          break
        case 'rustup':
          next.selectedRustupBinary = binaryPath
          next.knownRustupBinaries = Array.from(new Set([binaryPath, ...next.knownRustupBinaries]))
          break
        case 'zig':
          next.selectedZigBinary = binaryPath
          next.knownZigBinaries = Array.from(new Set([binaryPath, ...next.knownZigBinaries]))
          break
        case 'cargo-zigbuild':
          next.selectedCargoZigbuildBinary = binaryPath
          next.knownCargoZigbuildBinaries = Array.from(new Set([binaryPath, ...next.knownCargoZigbuildBinaries]))
          break
      }
      return this.saveConfig(next)
    },
    async clearSelection(kind: 'cargo' | 'rustup' | 'zig' | 'cargo-zigbuild') {
      const current = this.state?.config ?? emptyConfig()
      const next: RustToolchainConfig = {
        ...current,
        knownCargoBinaries: [...(current.knownCargoBinaries ?? [])],
        knownRustupBinaries: [...(current.knownRustupBinaries ?? [])],
        knownZigBinaries: [...(current.knownZigBinaries ?? [])],
        knownCargoZigbuildBinaries: [...(current.knownCargoZigbuildBinaries ?? [])],
      }
      switch (kind) {
        case 'cargo':
          next.selectedCargoBinary = ''
          break
        case 'rustup':
          next.selectedRustupBinary = ''
          break
        case 'zig':
          next.selectedZigBinary = ''
          break
        case 'cargo-zigbuild':
          next.selectedCargoZigbuildBinary = ''
          break
      }
      return this.saveConfig(next)
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
  },
})
