import { defineStore } from 'pinia'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { CancelExecution, ListTasks, StartLocalExecution, StartRemoteExecution } from '../../wailsjs/go/main/App'
import type { ExecutionRequest, ExecutionTask, RemoteExecRequest, TaskLogEvent } from '@/types/workbench'

interface ExecutionState {
  tasks: ExecutionTask[]
  logs: Record<string, string[]>
  subscribed: boolean
}

export const useExecutionStore = defineStore('execution', {
  state: (): ExecutionState => ({
    tasks: [],
    logs: {},
    subscribed: false,
  }),
  getters: {
    recentTasks(state) {
      return [...state.tasks].sort((a, b) => b.startedAt - a.startedAt)
    },
  },
  actions: {
    ensureSubscriptions() {
      if (this.subscribed) {
        return
      }

      EventsOn('task:update', (task: ExecutionTask) => {
        const index = this.tasks.findIndex((entry) => entry.id === task.id)
        if (index >= 0) {
          this.tasks[index] = task
        } else {
          this.tasks.unshift(task)
        }
      })

      EventsOn('task:log', (event: TaskLogEvent) => {
        const current = this.logs[event.taskId] ?? []
        this.logs[event.taskId] = [...current, event.message].slice(-600)
      })

      this.subscribed = true
    },
    async hydrate() {
      this.ensureSubscriptions()
      this.tasks = await ListTasks()
    },
    async startLocalExecution(request: ExecutionRequest) {
      this.ensureSubscriptions()
      const task = await StartLocalExecution({
        toolId: request.toolId,
        args: request.args,
        pythonEnv: request.pythonEnv ?? '',
      })
      if (!this.logs[task.id]) {
        this.logs[task.id] = []
      }
      return task
    },
    async cancelExecution(taskId: string) {
      await CancelExecution(taskId)
    },
    async startRemoteExecution(request: RemoteExecRequest) {
      this.ensureSubscriptions()
      const task = await StartRemoteExecution({
        toolId: request.toolId,
        connId: request.connId,
        args: request.args,
        pythonEnv: request.pythonEnv ?? '',
      })
      if (!this.logs[task.id]) {
        this.logs[task.id] = []
      }
      return task
    },
    logsForTask(taskId?: string) {
      if (!taskId) {
        return []
      }
      return this.logs[taskId] ?? []
    },
  },
})
