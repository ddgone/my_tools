import { defineStore } from 'pinia'

import { GetWorkbenchBootstrap } from '../../wailsjs/go/main/App'
import type { WorkbenchBootstrap } from '@/types/workbench'

interface WorkbenchState {
  bootstrap: WorkbenchBootstrap | null
  loading: boolean
  error: string
}

export const useWorkbenchStore = defineStore('workbench', {
  state: (): WorkbenchState => ({
    bootstrap: null,
    loading: false,
    error: '',
  }),
  actions: {
    async loadBootstrap() {
      this.loading = true
      this.error = ''

      try {
        this.bootstrap = await GetWorkbenchBootstrap()
      } catch (error) {
        this.error = error instanceof Error ? error.message : '加载桌面工作台数据失败'
      } finally {
        this.loading = false
      }
    },
  },
})
