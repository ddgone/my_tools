import type { RemotePathBrowseResult } from '@/types/workbench'

type MainAppBindings = {
  BrowseRemotePath(connId: string, requestedPath: string): Promise<RemotePathBrowseResult>
}

type MainAppWindow = Window & {
  go?: {
    main?: {
      App?: MainAppBindings
    }
  }
}

function getMainAppBindings(): MainAppBindings {
  const bindings = (window as MainAppWindow).go?.main?.App
  if (!bindings?.BrowseRemotePath) {
    throw new Error('远程路径浏览能力尚未就绪，请重新启动应用')
  }
  return bindings
}

export async function browseRemotePath(connId: string, requestedPath: string): Promise<RemotePathBrowseResult> {
  return getMainAppBindings().BrowseRemotePath(connId, requestedPath)
}
