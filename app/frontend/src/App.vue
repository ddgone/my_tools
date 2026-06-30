<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { darkTheme, NConfigProvider, NGlobalStyle, NMessageProvider, NNotificationProvider } from 'naive-ui'
import WorkspaceLayout from './components/WorkspaceLayout.vue'
import { useWorkspaceStore } from './stores/workspace'
import { buildThemeOverrides } from './theme/naiveTheme'
import { applyResolvedTheme } from './theme/runtime'
import { resolveThemePreference } from './theme/tokens'

const workspace = useWorkspaceStore()
const systemPrefersDark = ref(false)
let mediaQuery: MediaQueryList | null = null

const resolvedTheme = computed(() =>
  resolveThemePreference(workspace.settings.themePreference, systemPrefersDark.value),
)
const themeCustomization = computed(() => workspace.settings.themeCustomization)

const themeOverrides = computed(() =>
  buildThemeOverrides(resolvedTheme.value, themeCustomization.value),
)
const naiveTheme = computed(() => (resolvedTheme.value === 'dark' ? darkTheme : null))

function syncSystemThemeState() {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    systemPrefersDark.value = false
    return
  }
  mediaQuery ??= window.matchMedia('(prefers-color-scheme: dark)')
  systemPrefersDark.value = mediaQuery.matches
}

function handleSystemThemeChange(event: MediaQueryListEvent) {
  systemPrefersDark.value = event.matches
}

watch(
  [resolvedTheme, themeCustomization],
  ([themeName, customization]) => {
    applyResolvedTheme(themeName, customization)
  },
  { immediate: true },
)

onMounted(() => {
  syncSystemThemeState()
  if (!mediaQuery) {
    return
  }
  if (typeof mediaQuery.addEventListener === 'function') {
    mediaQuery.addEventListener('change', handleSystemThemeChange)
  } else {
    mediaQuery.addListener(handleSystemThemeChange)
  }
})

onUnmounted(() => {
  if (!mediaQuery) {
    return
  }
  if (typeof mediaQuery.removeEventListener === 'function') {
    mediaQuery.removeEventListener('change', handleSystemThemeChange)
  } else {
    mediaQuery.removeListener(handleSystemThemeChange)
  }
})
</script>

<template>
  <n-config-provider
    :theme="naiveTheme"
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
