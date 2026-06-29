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
  repeatable?: boolean
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
  remoteConnId?: string
  args: string
  pythonEnv?: string
  usage: string
  startedAt: number
  endedAt?: number
  exitMessage?: string
  remoteResultStatus?: string
  remoteResultPath?: string
  remoteResultKind?: string
  remoteResultMessage?: string
  remoteResultDownloadedPath?: string
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

export interface ArtifactBatchSelection {
  toolId: string
  targetOS: string
  targetArch: string
}

export interface ArtifactBatchRequest {
  mode: string
  exportRootDir?: string
  concurrency: number
  skipUnchanged: boolean
  preferCache: boolean
  forceRebuild: boolean
  continueOnError: boolean
  items: ArtifactBatchSelection[]
}

export interface ArtifactBatchItemResult {
  key: string
  toolId: string
  toolName: string
  kind: string
  targetOS: string
  targetArch: string
  status: string
  message: string
  outputPath?: string
  cacheHit: boolean
  startedAt: number
  endedAt?: number
}

export interface ArtifactBatchTask {
  id: string
  mode: string
  status: string
  exportRootDir?: string
  concurrency: number
  skipUnchanged: boolean
  preferCache: boolean
  forceRebuild: boolean
  continueOnError: boolean
  totalCount: number
  successCount: number
  errorCount: number
  cachedCount: number
  skippedCount: number
  startedAt: number
  endedAt?: number
  currentItem?: string
  exitMessage?: string
  items: ArtifactBatchItemResult[]
}

export interface ArtifactBatchEstimate {
  totalCount: number
  cachedCount: number
  buildCount: number
  invalidCount: number
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

export interface DownloadTask {
  id: string
  sourceTaskId: string
  toolId: string
  toolName: string
  status: string
  remoteResultPath: string
  remoteResultKind: string
  localPath?: string
  directory?: string
  message?: string
  downloadedBytes: number
  totalBytes: number
  progressPercent: number
  startedAt: number
  endedAt?: number
}

export interface FileDialogRequest {
  title: string
  filterName: string
  filterGlob: string
  directory: boolean
  defaultDirectory?: string
  defaultFilename?: string
}

export interface RemotePathEntry {
  name: string
  path: string
  kind: 'file' | 'directory'
  isSymlink: boolean
}

export interface RemotePathBrowseResult {
  requestedPath: string
  currentPath: string
  homePath: string
  fallback: boolean
  message?: string
  entries: RemotePathEntry[]
}

export interface GoToolchainConfig {
  selectedBinary: string
  knownBinaries: string[]
  lastInstallDirectory: string
  disabled: boolean
}

export interface GoToolchainCandidate {
  path: string
  version: string
  source: string
  label: string
  detail: string
  error?: string
  valid: boolean
  selected: boolean
  active: boolean
}

export interface GoToolchainState {
  config: GoToolchainConfig
  candidates: GoToolchainCandidate[]
  hasUsableBinary: boolean
  activeBinary: string
  activeVersion: string
  activeSource: string
  runtimeDetails: GoRuntimeDetails
  statusMessage: string
  suggestedInstallDirectory: string
}

export interface GoRuntimeDetails {
  goroot: string
  gopath: string
  goos: string
  goarch: string
  goversion: string
}

export interface GoOfficialRelease {
  version: string
  stable: boolean
}

export interface GoToolchainTaskState {
  kind: string
  status: string
  message: string
  detail?: string
  currentItem?: string
  currentSource?: string
  progressPercent: number
  step: number
  totalSteps: number
  version?: string
  directory?: string
  transferredBytes?: number
  totalBytes?: number
  transferSpeed?: string
  error?: string
  updatedAt: number
}

export interface RustToolchainConfig {
  mode: string
  selectedRustRoot: string
  knownRustRoots: string[]
  selectedZigBinary: string
  knownZigBinaries: string[]
  lastInstallDirectory: string
  disabled: boolean
}

export interface RustToolchainCandidate {
  path: string
  version: string
  source: string
  label: string
  detail: string
  error?: string
  valid: boolean
  selected: boolean
  active: boolean
}

export interface RustToolchainTargetStatus {
  platformKey: string
  platformLabel: string
  targetTriple: string
  installed: boolean
  native: boolean
  note?: string
}

export interface RustToolchainEnvironment {
  rootDir: string
  version: string
  source: string
  label: string
  detail: string
  error?: string
  valid: boolean
  selected: boolean
  active: boolean
  cargoBinary?: string
  rustupBinary?: string
  rustcBinary?: string
  cargoZigbuildBinary?: string
  hasRustup: boolean
  hasCargoZigbuild: boolean
  canManageTargets: boolean
  managed: boolean
}

export interface RustToolchainState {
  config: RustToolchainConfig
  rustCandidates: RustToolchainEnvironment[]
  zigCandidates: RustToolchainCandidate[]
  installedTargets: string[]
  targetStatuses: RustToolchainTargetStatus[]
  hasInstalledTargetInfo: boolean
  hasFullTargetCoverage: boolean
  targetStatusMessage: string
  cargoZigbuildStatusMessage: string
  hasUsableEnvironment: boolean
  hasUsableRust: boolean
  hasUsableCargo: boolean
  hasUsableRustup: boolean
  hasUsableZig: boolean
  hasUsableCargoZigbuild: boolean
  canManageTargets: boolean
  canManageCargoZigbuild: boolean
  activeRustRoot: string
  activeRustVersion: string
  activeRustSource: string
  activeRustManaged: boolean
  activeCargoBinary: string
  activeCargoVersion: string
  activeCargoSource: string
  activeRustupBinary: string
  activeRustupVersion: string
  activeRustupSource: string
  activeRustcBinary: string
  activeZigBinary: string
  activeZigVersion: string
  activeZigSource: string
  activeCargoZigbuildBinary: string
  activeCargoZigbuildVersion: string
  activeCargoZigbuildSource: string
  statusMessage: string
  suggestedInstallDirectory: string
}

export interface RustOfficialRelease {
  version: string
  stable: boolean
  channel: boolean
}

export interface ZigOfficialRelease {
  version: string
  stable: boolean
  date?: string
}

export interface RustToolchainTaskState {
  kind: string
  status: string
  message: string
  detail?: string
  currentItem?: string
  currentSource?: string
  progressPercent: number
  step: number
  totalSteps: number
  rustVersion?: string
  zigVersion?: string
  directory?: string
  transferredBytes?: number
  totalBytes?: number
  transferSpeed?: string
  error?: string
  updatedAt: number
}

export interface InstallRustToolchainRequest {
  rustVersion: string
  zigVersion: string
  directory: string
}

export interface PythonToolchainConfig {
  selectedBinary: string
  knownBinaries: string[]
  disabled: boolean
}

export interface PythonToolchainCandidate {
  path: string
  version: string
  source: string
  label: string
  detail: string
  error?: string
  valid: boolean
  selected: boolean
  active: boolean
}

export interface PythonDependencyStatus {
  packageName: string
  moduleName: string
  installed: boolean
  version?: string
  error?: string
  requiredBy: string[]
}

export interface PythonToolchainState {
  config: PythonToolchainConfig
  candidates: PythonToolchainCandidate[]
  hasUsableBaseBinary: boolean
  activeBaseBinary: string
  activeBaseVersion: string
  activeBaseSource: string
  hasUsableBinary: boolean
  activeBinary: string
  activeVersion: string
  activeSource: string
  pipAvailable: boolean
  dependenciesReady: boolean
  missingPackages: string[]
  statusMessage: string
  dependencies: PythonDependencyStatus[]
  dependencyToolCount: number
  dependencyTotalCount: number
  managedEnvDirectory: string
  needsRebuild: boolean
  managedBaseBinary: string
  managedBaseVersion: string
}

export interface PythonToolchainTaskState {
  kind: string
  status: string
  message: string
  detail?: string
  currentItem?: string
  progressPercent: number
  step: number
  totalSteps: number
  baseBinary?: string
  environmentDirectory?: string
  error?: string
  updatedAt: number
}

export interface PythonEnvironmentCheckResult {
  ok: boolean
  message: string
}
