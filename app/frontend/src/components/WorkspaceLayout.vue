<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useWorkspaceStore } from '@/stores/workspace'
import { useResizable } from '@/composables/useResizable'
import type { SSHConnection } from '@/types/workbench'
import AppHeader from './AppHeader.vue'
import ActivityBar from './ActivityBar.vue'
import type { ActivityBarView } from './ActivityBar.vue'
import ToolSidebar from './ToolSidebar.vue'
import WorkspaceTabs from './WorkspaceTabs.vue'
import StatusBar from './StatusBar.vue'
import HotkeyHelpModal from './HotkeyHelpModal.vue'
import SettingsModal from './SettingsModal.vue'

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const workspace = useWorkspaceStore()

const activityBarActiveView = ref<ActivityBarView | null>('tools')

const { size: sidebarWidth, dividerProps } = useResizable({
  axis: 'x',
  min: 180,
  max: 480,
  initial: 256,
  storageKey: 'fire-salamander:sidebar-width',
})

const sidebarRef = ref<InstanceType<typeof ToolSidebar> | null>(null)

function handleSelectConnection(conn: SSHConnection) {
  workspace.openSSHEdit(conn.id, conn.name)
}

function handleCreateConnection() {
  workspace.openSSHNew()
}

async function handleRefreshSSHList() {
  if (sidebarRef.value) {
    await sidebarRef.value.loadSSHConnections()
  }
}

function handleOpenSettings() {
  workspace.showSettings = true
}

onMounted(async () => {
  await Promise.all([workbench.loadBootstrap(), execution.hydrate()])
})
</script>

<template>
  <div class="flex h-screen flex-col overflow-hidden bg-dracula-bg text-dracula-text">
    <AppHeader />
    <div class="flex flex-1 overflow-hidden">
      <ActivityBar
        :active-view="activityBarActiveView"
        @update:active-view="activityBarActiveView = $event"
        @open-settings="handleOpenSettings"
      />
      <Transition name="slide">
        <ToolSidebar
          v-if="activityBarActiveView !== null"
          ref="sidebarRef"
          :width="sidebarWidth"
          :active-view="activityBarActiveView"
          @select-connection="handleSelectConnection"
          @create-connection="handleCreateConnection"
        />
      </Transition>
      <Transition name="slide">
        <div
          v-if="activityBarActiveView !== null"
          v-bind="dividerProps"
          class="group relative shrink-0 bg-dracula-soft"
          style="width: 1px"
        >
          <div class="absolute inset-y-0 -left-1 -right-1 group-hover:bg-dracula-cyan/10 group-active:bg-dracula-cyan/20" />
        </div>
      </Transition>
      <WorkspaceTabs @refresh-ssh-list="handleRefreshSSHList" />
    </div>
    <StatusBar />
    <HotkeyHelpModal />
    <SettingsModal />
  </div>
</template>
