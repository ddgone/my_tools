import {
  buildThemeCssVariables,
  defaultThemeCustomization,
  type ResolvedThemeName,
  type ThemeCustomizationSettings,
} from './tokens'

export function applyResolvedTheme(
  name: ResolvedThemeName,
  customization: ThemeCustomizationSettings = defaultThemeCustomization,
  root: HTMLElement = document.documentElement,
) {
  const variables = buildThemeCssVariables(name, customization)
  root.dataset.theme = name
  root.style.colorScheme = name
  for (const [key, value] of Object.entries(variables)) {
    root.style.setProperty(key, value)
  }
}
