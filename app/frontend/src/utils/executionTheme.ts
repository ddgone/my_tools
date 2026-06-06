export type ExecutionAccentName = 'cyan' | 'green' | 'pink'

export interface ExecutionTheme {
  accentName: ExecutionAccentName
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

const THEMES: Record<ExecutionAccentName, ExecutionTheme> = {
  cyan: {
    accentName: 'cyan',
    accent: '#8be9fd',
    accentRgb: '139, 233, 253',
    accentHover: '#a4ffff',
    accentText: '#102433',
    accentSoftBg: 'rgba(139, 233, 253, 0.10)',
    accentSoftBorder: 'rgba(139, 233, 253, 0.20)',
    accentSoftStrongBg: 'rgba(139, 233, 253, 0.16)',
    accentSoftStrongBorder: 'rgba(139, 233, 253, 0.30)',
    activeTabBackground: 'rgba(139, 233, 253, 0.10)',
    dividerGradient: 'linear-gradient(to right, transparent, rgba(139, 233, 253, 0.22), transparent)',
    railActive: 'rgba(139, 233, 253, 0.55)',
  },
  green: {
    accentName: 'green',
    accent: '#50fa7b',
    accentRgb: '80, 250, 123',
    accentHover: '#7dff9a',
    accentText: '#082512',
    accentSoftBg: 'rgba(80, 250, 123, 0.10)',
    accentSoftBorder: 'rgba(80, 250, 123, 0.20)',
    accentSoftStrongBg: 'rgba(80, 250, 123, 0.16)',
    accentSoftStrongBorder: 'rgba(80, 250, 123, 0.30)',
    activeTabBackground: 'rgba(80, 250, 123, 0.10)',
    dividerGradient: 'linear-gradient(to right, transparent, rgba(80, 250, 123, 0.22), transparent)',
    railActive: 'rgba(80, 250, 123, 0.50)',
  },
  pink: {
    accentName: 'pink',
    accent: '#ff79c6',
    accentRgb: '255, 121, 198',
    accentHover: '#ff94d2',
    accentText: '#2f1026',
    accentSoftBg: 'rgba(255, 121, 198, 0.10)',
    accentSoftBorder: 'rgba(255, 121, 198, 0.20)',
    accentSoftStrongBg: 'rgba(255, 121, 198, 0.16)',
    accentSoftStrongBorder: 'rgba(255, 121, 198, 0.30)',
    activeTabBackground: 'rgba(255, 121, 198, 0.12)',
    dividerGradient: 'linear-gradient(to right, transparent, rgba(255, 121, 198, 0.22), transparent)',
    railActive: 'rgba(255, 121, 198, 0.50)',
  },
}

export function resolveExecutionAccent(kind: string | null | undefined, executionTarget: 'local' | 'remote'): ExecutionAccentName {
  if (executionTarget === 'remote') {
    return 'pink'
  }
  return kind === 'python' ? 'green' : 'cyan'
}

export function getExecutionTheme(kind: string | null | undefined, executionTarget: 'local' | 'remote'): ExecutionTheme {
  return THEMES[resolveExecutionAccent(kind, executionTarget)]
}
