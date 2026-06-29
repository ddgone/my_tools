import type { GlobalThemeOverrides } from 'naive-ui'
import { themeTokensByName, type ResolvedThemeName } from './tokens'

export function buildThemeOverrides(name: ResolvedThemeName): GlobalThemeOverrides {
  const tokens = themeTokensByName[name]
  const borderColor = name === 'dark' ? 'rgba(229, 231, 235, 0.16)' : 'rgba(15, 23, 42, 0.12)'
  const dividerColor = name === 'dark' ? 'rgba(229, 231, 235, 0.12)' : 'rgba(15, 23, 42, 0.10)'
  const focusShadow = name === 'dark'
    ? '0 0 0 2px rgba(56, 189, 248, 0.16)'
    : '0 0 0 2px rgba(2, 132, 199, 0.14)'

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
