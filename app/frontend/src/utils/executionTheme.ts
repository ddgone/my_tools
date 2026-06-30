export type ExecutionAccentName = 'local' | 'remote'
export type ToolKindAccentName = 'go' | 'python' | 'rust'

export interface ExecutionTheme {
  accentName: ExecutionAccentName | ToolKindAccentName
  accent: string
  accentRgb: string
  accentHover: string
  accentText: string
  accentSoftBg: string
  accentSoftBorder: string
  accentSoftStrongBg: string
  accentSoftStrongBorder: string
  activeTabBackground: string
  dividerGradient: string
  railActive: string
}

function cssChannel(token: string) {
  return `var(${token})`
}

function rgb(token: string, alpha = 1) {
  return `rgb(var(${token}) / ${alpha})`
}

function buildAccentTheme(accentName: ExecutionAccentName | ToolKindAccentName, tokenPrefix: string, onToken?: string): ExecutionTheme {
  const token = `--color-${tokenPrefix}`
  const hoverToken = `--color-${tokenPrefix}-hover`
  return {
    accentName,
    accent: rgb(token),
    accentRgb: cssChannel(token),
    accentHover: onToken ? rgb(hoverToken) : rgb(token),
    accentText: onToken ? rgb(`--color-${onToken}`) : rgb(token),
    accentSoftBg: rgb(token, 0.08),
    accentSoftBorder: rgb(token, 0.18),
    accentSoftStrongBg: rgb(token, 0.12),
    accentSoftStrongBorder: rgb(token, 0.28),
    activeTabBackground: rgb(token, 0.08),
    dividerGradient: `linear-gradient(to right, transparent, ${rgb(token, 0.18)}, transparent)`,
    railActive: rgb(token, 0.46),
  }
}

const EXECUTION_THEMES: Record<ExecutionAccentName, ExecutionTheme> = {
  local: buildAccentTheme('local', 'mode-local', 'mode-local-on'),
  remote: buildAccentTheme('remote', 'mode-remote', 'mode-remote-on'),
}

const TOOL_KIND_THEMES: Record<ToolKindAccentName, ExecutionTheme> = {
  go: buildAccentTheme('go', 'kind-go'),
  python: buildAccentTheme('python', 'kind-python'),
  rust: buildAccentTheme('rust', 'kind-rust'),
}

export function resolveExecutionAccent(_kind: string | null | undefined, executionTarget: 'local' | 'remote'): ExecutionAccentName {
  return executionTarget === 'remote' ? 'remote' : 'local'
}

export function resolveToolKindAccent(kind: string | null | undefined): ToolKindAccentName {
  if (kind === 'python') {
    return 'python'
  }
  if (kind === 'rust') {
    return 'rust'
  }
  return 'go'
}

export function getExecutionTheme(_kind: string | null | undefined, executionTarget: 'local' | 'remote'): ExecutionTheme {
  return EXECUTION_THEMES[resolveExecutionAccent(undefined, executionTarget)]
}

export function getToolKindTheme(kind: string | null | undefined): ExecutionTheme {
  return TOOL_KIND_THEMES[resolveToolKindAccent(kind)]
}
