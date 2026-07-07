import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { ListDownloadTasks, OpenPath, StartTaskResultDownload } from '../../wailsjs/go/main/App'
import type { DownloadTask } from '@/types/workbench'
import { blurActiveElement } from '@/utils/focus'

function normalizeTasks(nextTasks: DownloadTask[]) {
  return [...nextTasks].sort((a, b) => b.startedAt - a.startedAt)
}

export const useDownloadStore = defineStore('downloads', () => {
  const tasks = ref<DownloadTask[]>([])
  const subscribed = ref(false)
  const drawerOpen = ref(false)
  const error = ref('')

  function removeFinishedTasks() {
    tasks.value = tasks.value.filter((task) => task.status === 'running')
  }

  function upsertTask(task: DownloadTask) {
    if (task.status === 'success') {
      if (task.directory) {
        void OpenPath(task.directory)
      }
      tasks.value = tasks.value.filter((entry) => entry.id !== task.id)
      return
    }

    const next = tasks.value.filter((entry) => entry.id !== task.id)
    next.unshift(task)
    tasks.value = normalizeTasks(next)
  }

  function ensureSubscriptions() {
    if (subscribed.value) {
      return
    }
    EventsOn('download:task:update', (task: DownloadTask) => {
      upsertTask(task)
    })
    subscribed.value = true
  }

  async function hydrate() {
    ensureSubscriptions()
    try {
      tasks.value = normalizeTasks((await ListDownloadTasks()).filter((task) => task.status === 'running'))
      error.value = ''
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载下载任务失败'
      tasks.value = []
    }
  }

  async function startTaskResultDownload(taskId: string) {
    ensureSubscriptions()
    const task = await StartTaskResultDownload(taskId)
    if (task) {
      upsertTask(task)
      drawerOpen.value = true
    }
    return task
  }

  const activeTasks = computed(() => tasks.value.filter((task) => task.status === 'running'))
  const activeCount = computed(() => activeTasks.value.length)
  const overallProgress = computed(() => {
    if (activeTasks.value.length === 0) {
      return 0
    }
    return activeTasks.value.reduce((sum, task) => sum + task.progressPercent, 0) / activeTasks.value.length
  })

  function openDrawer() {
    blurActiveElement()
    drawerOpen.value = true
  }

  function closeDrawer() {
    drawerOpen.value = false
    blurActiveElement()
    removeFinishedTasks()
  }

  return {
    tasks,
    activeTasks,
    activeCount,
    overallProgress,
    drawerOpen,
    error,
    ensureSubscriptions,
    hydrate,
    startTaskResultDownload,
    openDrawer,
    closeDrawer,
  }
})
