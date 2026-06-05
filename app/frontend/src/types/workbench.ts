export interface ToolDocs {
  summary: string
  usage: string
}

export interface ParameterOption {
  label: string
  value: string
}

export interface ParameterCondition {
  key: string
  equals?: unknown
}

export interface ParameterVisibility {
  all?: ParameterCondition[]
  any?: ParameterCondition[]
}

export interface ParameterSpec {
  key: string
  argKey?: string
  type: string
  label: string
  placeholder?: string
  required?: boolean
  default?: unknown
  help?: string
  options?: ParameterOption[]
  group?: string
  pathMode?: string
  emit?: boolean
  visibleWhen?: ParameterVisibility
}

export interface ExecutionSpec {
  local: {
    adapter: string
  }
  remote: {
    strategy: string
  }
}

export interface ExportSpec {
  strategy: string
}

export interface SourceSpec {
  entry: string
}

export interface ToolManifest {
  id: string
  name: string
  kind: string
  category: string[]
  icon: string
  description: string
  docs: ToolDocs
  params: ParameterSpec[]
  execution: ExecutionSpec
  export: ExportSpec
  source: SourceSpec
}

export interface WorkbenchBootstrap {
  appTitle: string
  platform: string
  hostStack: string[]
  primaryFlow: string[]
  moduleBoundaries: string[]
  parameterModes: string[]
  tools: ToolManifest[]
}

export interface ExecutionRequest {
  toolId: string
  args: string
  pythonEnv?: string
}

export interface ExecutionTask {
  id: string
  toolId: string
  toolName: string
  status: string
  target: string
  args: string
  pythonEnv?: string
  usage: string
  startedAt: number
  endedAt?: number
  exitMessage?: string
}

export interface ExportToolRequest {
  toolId: string
  mode?: string
  targetOS?: string
  targetArch?: string
}

export interface ExportToolResult {
  toolId: string
  toolName: string
  strategy: string
  mode: string
  filePath: string
  directory: string
  targetOS?: string
  targetArch?: string
}

export interface TaskLogEvent {
  taskId: string
  message: string
  recorded: number
}

export interface SSHConnection {
  id: string
  name: string
  host: string
  port: number
  user: string
  authMethod: string
  password?: string
  keyPath?: string
  description: string
  lastUsedAt?: number
}

export interface RemoteExecRequest {
  toolId: string
  connId: string
  args: string
  pythonEnv?: string
}

export interface FileDialogRequest {
  title: string
  filterName: string
  filterGlob: string
  directory: boolean
  defaultDirectory?: string
  defaultFilename?: string
}
