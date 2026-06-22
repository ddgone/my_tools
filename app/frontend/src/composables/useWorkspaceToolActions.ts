import { onMounted, onUnmounted, ref, watch, type ComputedRef } from 'vue'
import { useMessage } from 'naive-ui'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { ExportTool, OpenFileDialog, OpenPath } from '../../wailsjs/go/main/App'
import { useDownloadStore } from '@/stores/downloads'
import { useExecutionStore } from '@/stores/execution'
import { useGoEnvStore } from '@/stores/goenv'
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useRustEnvStore } from '@/stores/rustenv'
import { type GoExportMode, useWorkspaceStore } from '@/stores/workspace'
import { useWorkbenchStore } from '@/stores/workbench'
import type { ExecutionTask, ParameterSpec, ToolManifest } from '@/types/workbench'
import { validateCliArgs } from '@/utils/cliArgs'
import { findMissingRequiredParam } from '@/utils/toolParams'

type UseWorkspaceToolActionsOptions = {
  activeToolId: ComputedRef<string>
  activeTask: ComputedRef<ExecutionTask | null>
  activeExportTarget: ComputedRef<string>
  activeGoExportMode: ComputedRef<GoExportMode>
}

function parseExportTarget(value: string) {
  const [targetOS, targetArch] = value.split('/')
  return {
    targetOS: targetOS || undefined,
    targetArch: targetArch || undefined,
  }
}

function isMissingGoEnvError(detail: string) {
  return detail.includes('未检测到可用的 Go 环境')
    || detail.includes('请先在系统设置 > Go 中选择本地 Go 或下载 SDK')
    || detail.includes('指定的 Go 工具链不存在')
}

function isMissingPythonEnvError(detail: string) {
  return detail.includes('未检测到可用的基础 Python')
    || detail.includes('当前 Python 工具环境尚未准备好')
    || detail.includes('当前 Python 工具环境缺少 pip')
    || detail.includes('当前 Python 工具依赖未安装')
}

function isMissingRustEnvError(detail: string) {
  return detail.includes('未检测到可用的 Rust 交叉编译环境')
    || detail.includes('请先在系统设置 > Rust 中配置')
    || detail.includes('cargo-zigbuild')
    || detail.includes('rustup')
    || detail.includes('zig')
}

export function useWorkspaceToolActions(options: UseWorkspaceToolActionsOptions) {
  const workbench = useWorkbenchStore()
  const execution = useExecutionStore()
  const downloads = useDownloadStore()
  const goEnv = useGoEnvStore()
  const rustEnv = useRustEnvStore()
  const pythonEnv = usePythonEnvStore()
  const workspace = useWorkspaceStore()
  const message = useMessage()

  const launching = ref(false)
  const exporting = ref(false)
  const downloadingResult = ref(false)
  const exportProgressText = ref('')
  let disposeExportProgress: (() => void) | null = null

  function toolById(id: string): ToolManifest | null {
    return workbench.bootstrap?.tools.find((tool) => tool.id === id) ?? null
  }

  async function openGoSettings(messageText?: string) {
    workspace.openSettings('go')
    if (messageText) {
      message.warning(messageText)
    }
    if (!goEnv.state && !goEnv.loading) {
      await goEnv.loadState()
    }
  }

  async function openPythonSettings(messageText?: string) {
    workspace.openSettings('python')
    if (messageText) {
      message.warning(messageText)
    }
    if (!pythonEnv.state && !pythonEnv.loading) {
      await pythonEnv.loadState()
    }
  }

  async function openRustSettings(messageText?: string) {
    workspace.openSettings('rust')
    if (messageText) {
      message.warning(messageText)
    }
    if (!rustEnv.state && !rustEnv.loading) {
      await rustEnv.loadState()
    }
  }

  async function handleExecute() {
    const tab = workspace.activeToolTab
    const tool = tab ? toolById(tab.toolId) : null
    if (!tool || !tab) return

    const config = workspace.activeExecutionConfig
    if (!config) return

    const cliArgsError = validateCliArgs(config.rawArgs)
    if (cliArgsError) {
      message.error(cliArgsError)
      return
    }

    if (config.panelMode !== 'cli') {
      const missingParam = findMissingRequiredParam(tool, config.formModel)
      if (missingParam) {
        message.error(`请先填写“${missingParam.label}”`)
        return
      }
    }

    if (tab.executionTarget === 'remote' && !tab.remoteConfig.connId) {
      message.error('请选择远程环境后再执行')
      return
    }

    if (workspace.activeTabIndex >= 0) {
      workspace.setTerminalVisible(workspace.activeTabIndex, true)
    }

    workspace.recordUsage(tool.id, config.rawArgs, config.pythonEnv, config.formModel)

    launching.value = true
    try {
      if (tab.executionTarget === 'remote') {
        await execution.startRemoteExecution({
          toolId: tool.id,
          connId: tab.remoteConfig.connId,
          args: config.rawArgs,
          pythonEnv: tool.kind === 'python' ? config.pythonEnv : undefined,
        })
      } else {
        await execution.startLocalExecution({
          toolId: tool.id,
          args: config.rawArgs,
          pythonEnv: undefined,
        })
      }
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error)
      if (isMissingGoEnvError(detail)) {
        await openGoSettings('当前操作需要 Go 环境，已为你打开设置')
        return
      }
      if (isMissingPythonEnvError(detail)) {
        await openPythonSettings('当前操作需要 Python 环境，已为你打开设置')
        return
      }
      if (tool.kind === 'rust' && isMissingRustEnvError(detail)) {
        await openRustSettings('当前操作需要 Rust 交叉编译环境，已为你打开设置')
        return
      }
      message.error(detail || '执行失败')
    } finally {
      launching.value = false
    }
  }

  async function handleCancel() {
    const task = options.activeTask.value
    if (!task || task.status !== 'running') return
    await execution.cancelExecution(task.id)
  }

  async function handleExport() {
    const tab = workspace.activeToolTab
    const tool = tab ? toolById(tab.toolId) : null
    if (!tool) return
    if (!tool.export?.strategy) {
      message.error('当前工具没有可用的导出能力')
      return
    }

    exporting.value = true
    exportProgressText.value = '准备导出'
    try {
      const mode = tool.kind === 'python' ? 'source' : tool.kind === 'go' ? options.activeGoExportMode.value : 'binary'
      const target = (tool.kind === 'go' || tool.kind === 'rust') && mode === 'binary'
        ? parseExportTarget(options.activeExportTarget.value)
        : {}
      const result = await ExportTool({
        toolId: tool.id,
        mode,
        ...target,
      })
      if (!result?.filePath) {
        return
      }

      message.success(`已导出 ${result.toolName}`)
      if (workspace.settings.autoOpenExportDir) {
        try {
          await OpenPath(result.directory)
        } catch (error) {
          const detail = error instanceof Error ? error.message : String(error)
          message.warning(`导出成功，但打开目录失败：${detail}`)
        }
      }
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error)
      if (isMissingGoEnvError(detail)) {
        await openGoSettings('导出 Go 工具前需要先配置 Go 环境')
        return
      }
      if (tool.kind === 'rust' && isMissingRustEnvError(detail)) {
        await openRustSettings('导出 Rust 工具前需要先配置 Rust 交叉编译环境')
        return
      }
      message.error(detail || '工具导出失败')
    } finally {
      exporting.value = false
      exportProgressText.value = ''
    }
  }

  async function handleDownloadResult() {
    const task = options.activeTask.value
    if (!task || task.remoteResultStatus !== 'available') {
      message.warning('当前任务没有可下载结果')
      return
    }

    downloadingResult.value = true
    try {
      const downloadTask = await downloads.startTaskResultDownload(task.id)
      if (!downloadTask?.id) {
        return
      }
      message.success('已加入下载任务')
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error)
      message.error(detail || '下载结果失败')
    } finally {
      downloadingResult.value = false
    }
  }

  async function handleFileDialog(param: ParameterSpec, target?: 'file' | 'directory') {
    const tab = workspace.activeToolTab
    const config = workspace.activeExecutionConfig
    if (!tab) return

    const dialogTarget = target || (param.pathMode === 'file' ? 'file' : 'directory')
    const result = await OpenFileDialog({
      title: `选择 ${param.label}`,
      filterName: '所有文件',
      filterGlob: '*.*',
      directory: dialogTarget === 'directory',
      defaultDirectory: '',
      defaultFilename: '',
    })

    if (!result || !config) {
      return
    }

    if (param.repeatable) {
      const currentValue = typeof config.formModel[param.key] === 'string' ? String(config.formModel[param.key] || '') : ''
      const items = currentValue
        .split(/\r?\n/)
        .map((item) => item.trim())
        .filter((item) => item.length > 0)
      if (!items.includes(result)) {
        items.push(result)
      }
      config.formModel[param.key] = items.join('\n')
      return
    }

    config.formModel[param.key] = result
  }

  onMounted(() => {
    disposeExportProgress = EventsOn('export:progress', (event: { toolId?: string, message?: string }) => {
      if (event.toolId !== options.activeToolId.value) {
        return
      }
      exportProgressText.value = String(event.message ?? '').trim()
    })
  })

  onUnmounted(() => {
    disposeExportProgress?.()
  })

  watch(options.activeToolId, () => {
    if (!exporting.value) {
      exportProgressText.value = ''
    }
  })

  return {
    launching,
    exporting,
    downloadingResult,
    exportProgressText,
    handleExecute,
    handleCancel,
    handleExport,
    handleDownloadResult,
    handleFileDialog,
  }
}
