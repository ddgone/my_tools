<script setup lang="ts">
import { darkTheme, NConfigProvider, NGlobalStyle } from 'naive-ui'
import WorkspaceLayout from './components/WorkspaceLayout.vue'

const themeOverrides = {
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
    textColorBase: '#f8f8f2',
    textColor1: '#f8f8f2',
    textColor2: '#cfd3df',
    borderColor: '#44475a',
  },
}
</script>

<template>
  <n-config-provider
    :theme="darkTheme"
    :theme-overrides="themeOverrides"
  >
    <n-global-style />
    <WorkspaceLayout />
  </n-config-provider>
</template>