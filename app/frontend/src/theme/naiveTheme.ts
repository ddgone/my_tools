import type { GlobalThemeOverrides } from 'naive-ui'
import {
  defaultThemeCustomization,
  resolveThemeTokens,
  type ResolvedThemeName,
  type ThemeCustomizationSettings,
} from './tokens'

export function buildThemeOverrides(
  name: ResolvedThemeName,
  customization: ThemeCustomizationSettings = defaultThemeCustomization,
): GlobalThemeOverrides {
  const tokens = resolveThemeTokens(name, customization)
  const borderColor = name === 'dark' ? 'rgba(231, 234, 240, 0.12)' : 'rgba(31, 35, 41, 0.10)'
  const dividerColor = name === 'dark' ? 'rgba(231, 234, 240, 0.08)' : 'rgba(31, 35, 41, 0.08)'
  const focusShadow = name === 'dark'
    ? '0 0 0 2px rgba(83, 177, 253, 0.18)'
    : '0 0 0 2px rgba(47, 155, 255, 0.16)'

  return {
    common: {
      primaryColor: tokens.brandPrimary,
      primaryColorHover: tokens.brandHover,
      primaryColorPressed: tokens.brandPressed,
      infoColor: tokens.brandPrimary,
      successColor: tokens.success,
      warningColor: tokens.warning,
      errorColor: tokens.error,
      bodyColor: tokens.bgApp,
      cardColor: tokens.bgPanel,
      modalColor: tokens.bgPanel,
      popoverColor: tokens.bgPanel,
      tableColor: tokens.bgPanel,
      inputColor: tokens.bgPanel,
      textColorBase: tokens.fgBase,
      textColor1: tokens.fgBase,
      textColor2: tokens.fgSecondary,
      textColor3: tokens.fgMuted,
      borderColor,
      dividerColor,
      borderRadius: '6px',
      borderRadiusSmall: '4px',
      fontFamily: "'Nunito', -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif",
      fontFamilyMono: "'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', 'Courier New', monospace",
    },
    Button: {
      borderRadiusSmall: '4px',
      borderRadiusMedium: '6px',
      borderRadiusLarge: '8px',
    },
    Input: {
      borderRadius: '6px',
      border: `1px solid ${borderColor}`,
      groupLabelBorder: `1px solid ${borderColor}`,
      borderHover: `1px solid ${tokens.brandPrimary}`,
      borderFocus: `1px solid ${tokens.brandPrimary}`,
      boxShadowFocus: focusShadow,
    },
    InternalSelection: {
      border: `1px solid ${borderColor}`,
      borderHover: `1px solid ${tokens.brandPrimary}`,
      borderFocus: `1px solid ${tokens.brandPrimary}`,
      borderActive: `1px solid ${tokens.brandPrimary}`,
      boxShadowHover: 'none',
      boxShadowFocus: focusShadow,
      boxShadowActive: focusShadow,
    },
    Tag: {
      borderRadius: '4px',
    },
    Tooltip: {
      color: tokens.tooltipBg,
      textColor: tokens.tooltipText,
      borderRadius: '6px',
      boxShadow: tokens.shadowTooltip,
      padding: '6px 10px',
    },
    Dropdown: {
      color: tokens.bgPanel,
      optionColorHover: name === 'dark' ? 'rgba(83, 177, 253, 0.08)' : 'rgba(47, 155, 255, 0.08)',
      optionTextColor: tokens.fgBase,
      optionTextColorHover: tokens.fgBase,
      optionTextColorActive: tokens.fgBase,
      optionColorActive: name === 'dark' ? 'rgba(83, 177, 253, 0.12)' : 'rgba(47, 155, 255, 0.12)',
    },
    Select: {
      peers: {
        InternalSelection: {
          color: tokens.bgPanel,
        },
      },
    },
    Form: {
      blankHeightSmall: '14px',
      blankHeightMedium: '20px',
      blankHeightLarge: '28px',
      feedbackHeightSmall: '16px',
      feedbackHeightMedium: '20px',
      feedbackHeightLarge: '24px',
    },
  }
}
