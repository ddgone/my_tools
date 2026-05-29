<script setup lang="ts">
import { darkTheme, NConfigProvider, NGlobalStyle, NMessageProvider, NNotificationProvider } from 'naive-ui'
import type { GlobalThemeOverrides } from 'naive-ui'
import WorkspaceLayout from './components/WorkspaceLayout.vue'

const themeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#8be9fd',
    primaryColorHover: '#a4ffff',
    primaryColorPressed: '#6ed7ea',
    infoColor: '#8be9fd',
    successColor: '#50fa7b',
    warningColor: '#f1fa8c',
    errorColor: '#ff5555',
    bodyColor: '#282a36',
    cardColor: '#1e1f29',
    modalColor: '#1e1f29',
    popoverColor: '#1e1f29',
    tableColor: '#1e1f29',
    inputColor: '#1e1f29',
    textColorBase: '#f8f8f2',
    textColor1: '#f8f8f2',
    textColor2: '#cfd3df',
    textColor3: '#a0a6ba',
    borderColor: 'rgba(255,255,255,0.22)',
    dividerColor: 'rgba(255,255,255,0.20)',
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
    border: '1px solid rgba(255,255,255,0.22)',
    groupLabelBorder: '1px solid rgba(255,255,255,0.22)',
    borderHover: '1px solid #8be9fd',
    borderFocus: '1px solid #8be9fd',
    boxShadowFocus: '0 0 0 2px rgba(139,233,253,0.15)',
  },
  InternalSelection: {
    border: '1px solid rgba(255,255,255,0.22)',
    borderHover: '1px solid #8be9fd',
    borderFocus: '1px solid #8be9fd',
    borderActive: '1px solid #8be9fd',
    boxShadowHover: 'none',
    boxShadowFocus: '0 0 0 2px rgba(139,233,253,0.15)',
    boxShadowActive: '0 0 0 2px rgba(139,233,253,0.15)',
  },
  Tag: {
    borderRadius: '4px',
  },
  Tooltip: {
    color: '#faf8f5',
    textColor: '#1a1a2e',
    borderRadius: '6px',
    boxShadow: '0 10px 24px rgba(0, 0, 0, 0.2)',
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
</script>

<template>
  <n-config-provider
    :theme="darkTheme"
    :theme-overrides="themeOverrides"
    :transition-disabled="true"
  >
    <n-notification-provider>
      <n-message-provider>
        <n-global-style />
        <WorkspaceLayout />
      </n-message-provider>
    </n-notification-provider>
  </n-config-provider>
</template>
