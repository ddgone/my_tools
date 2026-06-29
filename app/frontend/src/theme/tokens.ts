export type ThemePreference = 'dark' | 'light' | 'system'
export type ResolvedThemeName = 'dark' | 'light'
export type ModeAccentName = 'local' | 'remote'
export type ToolKindAccentName = 'go' | 'python' | 'rust'

interface ThemeTokens {
  bgApp: string
  bgPanel: string
  bgElevated: string
  bgShell: string
  fgBase: string
  fgSecondary: string
  fgMuted: string
  borderSubtle: string
  borderStrong: string
  brandPrimary: string
  brandHover: string
  brandPressed: string
  success: string
  warning: string
  error: string
  titlebarFrom: string
  titlebarTo: string
  scrollbarThumb: string
  scrollbarThumbHover: string
  tooltipBg: string
  tooltipText: string
  shellText: string
  overlayRgb: string
  shadowTooltip: string
  modeLocal: string
  modeLocalHover: string
  modeLocalOn: string
  modeRemote: string
  modeRemoteHover: string
  modeRemoteOn: string
  kindGo: string
  kindPython: string
  kindRust: string
}

const darkThemeTokens: ThemeTokens = {
  bgApp: '#0f172a',
  bgPanel: '#111827',
  bgElevated: '#1f2937',
  bgShell: '#020617',
  fgBase: '#e5e7eb',
  fgSecondary: '#cbd5e1',
  fgMuted: '#94a3b8',
  borderSubtle: '#334155',
  borderStrong: '#475569',
  brandPrimary: '#38bdf8',
  brandHover: '#67d3fb',
  brandPressed: '#0ea5e9',
  success: '#22c55e',
  warning: '#f59e0b',
  error: '#ef4444',
  titlebarFrom: '#111827',
  titlebarTo: '#0f172a',
  scrollbarThumb: '#475569',
  scrollbarThumbHover: '#64748b',
  tooltipBg: '#f8fafc',
  tooltipText: '#0f172a',
  shellText: '#cbd5e1',
  overlayRgb: '2 6 23',
  shadowTooltip: '0 12px 28px rgba(2, 6, 23, 0.28)',
  modeLocal: '#38bdf8',
  modeLocalHover: '#67d3fb',
  modeLocalOn: '#082f49',
  modeRemote: '#a78bfa',
  modeRemoteHover: '#c4b5fd',
  modeRemoteOn: '#2e1065',
  kindGo: '#38bdf8',
  kindPython: '#22c55e',
  kindRust: '#f59e0b',
}

const lightThemeTokens: ThemeTokens = {
  bgApp: '#f8fafc',
  bgPanel: '#ffffff',
  bgElevated: '#f1f5f9',
  bgShell: '#e2e8f0',
  fgBase: '#0f172a',
  fgSecondary: '#334155',
  fgMuted: '#64748b',
  borderSubtle: '#cbd5e1',
  borderStrong: '#94a3b8',
  brandPrimary: '#0284c7',
  brandHover: '#0369a1',
  brandPressed: '#075985',
  success: '#16a34a',
  warning: '#d97706',
  error: '#dc2626',
  titlebarFrom: '#ffffff',
  titlebarTo: '#f8fafc',
  scrollbarThumb: '#cbd5e1',
  scrollbarThumbHover: '#94a3b8',
  tooltipBg: '#0f172a',
  tooltipText: '#f8fafc',
  shellText: '#0f172a',
  overlayRgb: '15 23 42',
  shadowTooltip: '0 12px 28px rgba(15, 23, 42, 0.18)',
  modeLocal: '#0284c7',
  modeLocalHover: '#0369a1',
  modeLocalOn: '#f8fafc',
  modeRemote: '#7c3aed',
  modeRemoteHover: '#6d28d9',
  modeRemoteOn: '#f8fafc',
  kindGo: '#0284c7',
  kindPython: '#16a34a',
  kindRust: '#d97706',
}

export const themeTokensByName: Record<ResolvedThemeName, ThemeTokens> = {
  dark: darkThemeTokens,
  light: lightThemeTokens,
}

export function resolveThemePreference(preference: ThemePreference, systemPrefersDark: boolean): ResolvedThemeName {
  if (preference === 'light') {
    return 'light'
  }
  if (preference === 'system') {
    return systemPrefersDark ? 'dark' : 'light'
  }
  return 'dark'
}

function hexToRgbChannels(hex: string): string {
  const normalized = hex.replace('#', '').trim()
  if (normalized.length !== 6) {
    throw new Error(`Expected 6-digit hex color, got "${hex}"`)
  }
  const value = Number.parseInt(normalized, 16)
  const r = (value >> 16) & 0xff
  const g = (value >> 8) & 0xff
  const b = value & 0xff
  return `${r} ${g} ${b}`
}

export function buildThemeCssVariables(name: ResolvedThemeName): Record<string, string> {
  const tokens = themeTokensByName[name]
  return {
    '--color-bg-app': hexToRgbChannels(tokens.bgApp),
    '--color-bg-panel': hexToRgbChannels(tokens.bgPanel),
    '--color-bg-elevated': hexToRgbChannels(tokens.bgElevated),
    '--color-bg-shell': hexToRgbChannels(tokens.bgShell),
    '--color-fg-base': hexToRgbChannels(tokens.fgBase),
    '--color-fg-secondary': hexToRgbChannels(tokens.fgSecondary),
    '--color-fg-muted': hexToRgbChannels(tokens.fgMuted),
    '--color-border-subtle': hexToRgbChannels(tokens.borderSubtle),
    '--color-border-strong': hexToRgbChannels(tokens.borderStrong),
    '--color-brand-primary': hexToRgbChannels(tokens.brandPrimary),
    '--color-brand-hover': hexToRgbChannels(tokens.brandHover),
    '--color-brand-pressed': hexToRgbChannels(tokens.brandPressed),
    '--color-success': hexToRgbChannels(tokens.success),
    '--color-warning': hexToRgbChannels(tokens.warning),
    '--color-error': hexToRgbChannels(tokens.error),
    '--color-titlebar-from': hexToRgbChannels(tokens.titlebarFrom),
    '--color-titlebar-to': hexToRgbChannels(tokens.titlebarTo),
    '--color-scrollbar-thumb': hexToRgbChannels(tokens.scrollbarThumb),
    '--color-scrollbar-thumb-hover': hexToRgbChannels(tokens.scrollbarThumbHover),
    '--color-tooltip-bg': hexToRgbChannels(tokens.tooltipBg),
    '--color-tooltip-text': hexToRgbChannels(tokens.tooltipText),
    '--color-shell-text': hexToRgbChannels(tokens.shellText),
    '--color-overlay-rgb': tokens.overlayRgb,
    '--shadow-tooltip': tokens.shadowTooltip,
    '--color-mode-local': hexToRgbChannels(tokens.modeLocal),
    '--color-mode-local-hover': hexToRgbChannels(tokens.modeLocalHover),
    '--color-mode-local-on': hexToRgbChannels(tokens.modeLocalOn),
    '--color-mode-remote': hexToRgbChannels(tokens.modeRemote),
    '--color-mode-remote-hover': hexToRgbChannels(tokens.modeRemoteHover),
    '--color-mode-remote-on': hexToRgbChannels(tokens.modeRemoteOn),
    '--color-kind-go': hexToRgbChannels(tokens.kindGo),
    '--color-kind-python': hexToRgbChannels(tokens.kindPython),
    '--color-kind-rust': hexToRgbChannels(tokens.kindRust),
  }
}
