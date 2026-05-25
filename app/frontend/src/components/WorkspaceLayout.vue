<script setup lang="ts">
import { onMounted } from 'vue'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useResizable } from '@/composables/useResizable'
import AppHeader from './AppHeader.vue'
import ToolSidebar from './ToolSidebar.vue'
import WorkspaceTabs from './WorkspaceTabs.vue'
import StatusBar from './StatusBar.vue'
import HotkeyHelpModal from './HotkeyHelpModal.vue'
import SettingsModal from './SettingsModal.vue'

const workbench = useWorkbenchStore()
const execution = useExecutionStore()

const { size: sidebarWidth, dividerProps } = useResizable({
  axis: 'x',
  min: 180,
  max: 480,
  initial: 256,
  storageKey: 'fire-salamander:sidebar-width',
})

onMounted(async () => {
  await Promise.all([workbench.loadBootstrap(), execution.hydrate()])
})
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden bg-dracula-bg text-dracula-text">
    <AppHeader />
    <div class="flex flex-1 overflow-hidden">
      <ToolSidebar :width="sidebarWidth" />
      <div
        v-bind="dividerProps"
        class="group relative shrink-0 bg-dracula-soft"
        style="width: 1px"
      >
        <div class="absolute inset-y-0 -left-1 -right-1 group-hover:bg-dracula-cyan/10 group-active:bg-dracula-cyan/20" />
      </div>
      <WorkspaceTabs />
    </div>
    <StatusBar />
    <HotkeyHelpModal />
    <SettingsModal />
  </div>
</template>
