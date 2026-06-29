import { buildThemeCssVariables, type ResolvedThemeName } from './tokens'

export function applyResolvedTheme(name: ResolvedThemeName, root: HTMLElement = document.documentElement) {
  const variables = buildThemeCssVariables(name)
  root.dataset.theme = name
  root.style.colorScheme = name
  for (const [key, value] of Object.entries(variables)) {
    root.style.setProperty(key, value)
  }
}
