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
    borderColor: '#44475a',
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
  },
  Tag: {
    borderRadius: '4px',
  },
}
</script>

<template>
  <n-config-provider
    :theme="darkTheme"
    :theme-overrides="themeOverrides"
  >
    <n-notification-provider>
      <n-message-provider>
        <n-global-style />
        <WorkspaceLayout />
      </n-message-provider>
    </n-notification-provider>
  </n-config-provider>
</template>
