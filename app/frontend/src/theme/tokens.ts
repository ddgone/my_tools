export type ThemePreference = 'dark' | 'light' | 'system'
export type ResolvedThemeName = 'dark' | 'light'
export type ModeAccentName = 'local' | 'remote'
export type ToolKindAccentName = 'go' | 'python' | 'rust' | 'zig'
export interface ThemeCustomizationSettings {
  darkAccent: string
  lightAccent: string
  darkBackground: string
  darkPanel: string
  lightBackground: string
  lightPanel: string
}

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
  kindZig: string
}

const darkThemeTokens: ThemeTokens = {
  bgApp: '#1b1d21',
  bgPanel: '#23262c',
  bgElevated: '#2a2d33',
  bgShell: '#16181c',
  fgBase: '#e7eaf0',
  fgSecondary: '#c6ccd6',
  fgMuted: '#9198a6',
  borderSubtle: '#363a43',
  borderStrong: '#474d59',
  brandPrimary: '#58a9ff',
  brandHover: '#7abcff',
  brandPressed: '#338ee6',
  success: '#22c55e',
  warning: '#f59e0b',
  error: '#ef4444',
  titlebarFrom: '#272a31',
  titlebarTo: '#1b1d21',
  scrollbarThumb: '#49505d',
  scrollbarThumbHover: '#616a7b',
  tooltipBg: '#2b2f36',
  tooltipText: '#f3f5f8',
  shellText: '#cfd5df',
  overlayRgb: '12 14 18',
  shadowTooltip: '0 14px 34px rgba(12, 14, 18, 0.34)',
  modeLocal: '#58a9ff',
  modeLocalHover: '#7abcff',
  modeLocalOn: '#0e2940',
  modeRemote: '#7e88b2',
  modeRemoteHover: '#939dc5',
  modeRemoteOn: '#f6f8fb',
  kindGo: '#58a9ff',
  kindPython: '#22c55e',
  kindRust: '#f59e0b',
  kindZig: '#f7b733',
}

const lightThemeTokens: ThemeTokens = {
  bgApp: '#f2f3f5',
  bgPanel: '#fafafc',
  bgElevated: '#eceff2',
  bgShell: '#e6e8ec',
  fgBase: '#1f2329',
  fgSecondary: '#3d444d',
  fgMuted: '#6e7581',
  borderSubtle: '#d7dce4',
  borderStrong: '#bfc7d1',
  brandPrimary: '#308fe8',
  brandHover: '#57a9f6',
  brandPressed: '#167ed6',
  success: '#16a34a',
  warning: '#d97706',
  error: '#dc2626',
  titlebarFrom: '#fbfbfc',
  titlebarTo: '#eef1f4',
  scrollbarThumb: '#c5cbd4',
  scrollbarThumbHover: '#adb5c1',
  tooltipBg: '#262a31',
  tooltipText: '#f5f7fa',
  shellText: '#1f2329',
  overlayRgb: '31 35 41',
  shadowTooltip: '0 14px 32px rgba(31, 35, 41, 0.16)',
  modeLocal: '#308fe8',
  modeLocalHover: '#57a9f6',
  modeLocalOn: '#f7fbff',
  modeRemote: '#7b86b0',
  modeRemoteHover: '#6d789e',
  modeRemoteOn: '#f8f7ff',
  kindGo: '#308fe8',
  kindPython: '#16a34a',
  kindRust: '#d97706',
  kindZig: '#e59a17',
}

export const themeTokensByName: Record<ResolvedThemeName, ThemeTokens> = {
  dark: darkThemeTokens,
  light: lightThemeTokens,
}

export const defaultThemeCustomization: ThemeCustomizationSettings = {
  darkAccent: '#63b4ff',
  lightAccent: '#2f8ee7',
  darkBackground: darkThemeTokens.bgApp,
  darkPanel: darkThemeTokens.bgPanel,
  lightBackground: lightThemeTokens.bgApp,
  lightPanel: lightThemeTokens.bgPanel,
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

function clampChannel(value: number) {
  return Math.max(0, Math.min(255, Math.round(value)))
}

function parseHexColor(hex: string) {
  const normalized = normalizeHexColor(hex)
  const value = Number.parseInt(normalized.slice(1), 16)
  return {
    r: (value >> 16) & 0xff,
    g: (value >> 8) & 0xff,
    b: value & 0xff,
  }
}

function rgbToHex(r: number, g: number, b: number) {
  const toHex = (channel: number) => clampChannel(channel).toString(16).padStart(2, '0')
  return `#${toHex(r)}${toHex(g)}${toHex(b)}`
}

function rgbChannelsToCommaSeparated(hex: string) {
  return hexToRgbChannels(hex).split(' ').join(', ')
}

function mixHexColors(colorA: string, colorB: string, ratio: number) {
  const a = parseHexColor(colorA)
  const b = parseHexColor(colorB)
  return rgbToHex(
    a.r + (b.r - a.r) * ratio,
    a.g + (b.g - a.g) * ratio,
    a.b + (b.b - a.b) * ratio,
  )
}

function luminance(hex: string) {
  const { r, g, b } = parseHexColor(hex)
  return (0.2126 * r + 0.7152 * g + 0.0722 * b) / 255
}

function readableTextColor(hex: string, dark = '#102132', light = '#f8fbff') {
  return luminance(hex) > 0.62 ? dark : light
}

export function normalizeHexColor(value: unknown, fallback = '#4da2ff') {
  const raw = typeof value === 'string' ? value.trim() : ''
  const match = raw.match(/^#?([0-9a-fA-F]{6})$/)
  if (!match) {
    return fallback
  }
  return `#${match[1].toLowerCase()}`
}

export function normalizeThemeCustomization(source?: Partial<ThemeCustomizationSettings>): ThemeCustomizationSettings {
  const legacyAccent = normalizeHexColor((source as { brandAccent?: unknown } | undefined)?.brandAccent, defaultThemeCustomization.darkAccent)
  return {
    darkAccent: normalizeHexColor(source?.darkAccent, legacyAccent),
    lightAccent: normalizeHexColor(source?.lightAccent, (source as { brandAccent?: unknown } | undefined)?.brandAccent ? legacyAccent : defaultThemeCustomization.lightAccent),
    darkBackground: normalizeHexColor(source?.darkBackground, defaultThemeCustomization.darkBackground),
    darkPanel: normalizeHexColor(source?.darkPanel, defaultThemeCustomization.darkPanel),
    lightBackground: normalizeHexColor(source?.lightBackground, defaultThemeCustomization.lightBackground),
    lightPanel: normalizeHexColor(source?.lightPanel, defaultThemeCustomization.lightPanel),
  }
}

export function resolveThemeTokens(
  name: ResolvedThemeName,
  customization = defaultThemeCustomization,
): ThemeTokens {
  const base = themeTokensByName[name]
  const normalized = normalizeThemeCustomization(customization)
  const brandAccent = name === 'dark' ? normalized.darkAccent : normalized.lightAccent
  const brandHover = mixHexColors(brandAccent, '#ffffff', name === 'dark' ? 0.18 : 0.12)
  const brandPressed = mixHexColors(brandAccent, '#0f172a', name === 'dark' ? 0.16 : 0.14)
  const modeLocalOn = readableTextColor(brandAccent)

  if (name === 'dark') {
    const bgApp = normalized.darkBackground
    const bgPanel = normalized.darkPanel
    return {
      ...base,
      bgApp,
      bgPanel,
      bgElevated: mixHexColors(bgPanel, '#ffffff', 0.05),
      bgShell: mixHexColors(bgApp, '#000000', 0.18),
      borderSubtle: mixHexColors(bgPanel, '#ffffff', 0.10),
      borderStrong: mixHexColors(bgPanel, '#ffffff', 0.17),
      brandPrimary: brandAccent,
      brandHover,
      brandPressed,
      titlebarFrom: mixHexColors(bgPanel, '#ffffff', 0.02),
      titlebarTo: bgApp,
      scrollbarThumb: mixHexColors(bgPanel, '#ffffff', 0.18),
      scrollbarThumbHover: mixHexColors(bgPanel, '#ffffff', 0.28),
      tooltipBg: '#eff1f5',
      tooltipText: '#1d2127',
      shellText: '#cfd5df',
      overlayRgb: hexToRgbChannels(mixHexColors(bgApp, '#000000', 0.35)),
      shadowTooltip: `0 14px 34px rgba(${rgbChannelsToCommaSeparated(mixHexColors(bgApp, '#000000', 0.5))}, 0.34)`,
      modeLocal: brandAccent,
      modeLocalHover: brandHover,
      modeLocalOn,
      kindGo: brandAccent,
      kindZig: base.kindZig,
    }
  }

  const bgApp = normalized.lightBackground
  const bgPanel = normalized.lightPanel
  return {
    ...base,
    bgApp,
    bgPanel,
    bgElevated: mixHexColors(bgApp, '#000000', 0.03),
    bgShell: mixHexColors(bgApp, '#000000', 0.06),
    borderSubtle: mixHexColors(bgApp, '#000000', 0.11),
    borderStrong: mixHexColors(bgApp, '#000000', 0.18),
    brandPrimary: brandAccent,
    brandHover,
    brandPressed,
    titlebarFrom: mixHexColors(bgPanel, '#ffffff', 0.45),
    titlebarTo: mixHexColors(bgApp, '#000000', 0.02),
    scrollbarThumb: mixHexColors(bgApp, '#000000', 0.17),
    scrollbarThumbHover: mixHexColors(bgApp, '#000000', 0.25),
    tooltipBg: '#262a31',
    tooltipText: '#f5f7fa',
    shellText: '#1f2329',
    overlayRgb: hexToRgbChannels(mixHexColors(bgApp, '#000000', 0.62)),
    shadowTooltip: `0 14px 32px rgba(${rgbChannelsToCommaSeparated(mixHexColors(bgApp, '#000000', 0.62))}, 0.16)`,
    modeLocal: brandAccent,
    modeLocalHover: brandHover,
    modeLocalOn,
    kindGo: brandAccent,
    kindZig: base.kindZig,
  }
}

export function buildThemeCssVariables(
  name: ResolvedThemeName,
  customization = defaultThemeCustomization,
): Record<string, string> {
  const tokens = resolveThemeTokens(name, customization)
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
    '--color-kind-zig': hexToRgbChannels(tokens.kindZig),
  }
}
