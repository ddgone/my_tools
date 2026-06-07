<script setup lang="ts">
import { onMounted, ref } from 'vue'
import gsap from 'gsap'
import { useWorkbenchStore } from '@/stores/workbench'
import { useExecutionStore } from '@/stores/execution'
import { useGoEnvStore } from '@/stores/goenv'
import { usePythonEnvStore } from '@/stores/pythonenv'
import { useWorkspaceStore } from '@/stores/workspace'
import { builtinTools } from '@/builtin/registry'
import { useResizable } from '@/composables/useResizable'
import { ANIM } from '@/utils/animation'
import type { SSHConnection } from '@/types/workbench'
import AppHeader from './AppHeader.vue'
import ActivityBar from './ActivityBar.vue'
import type { ActivityBarView } from './ActivityBar.vue'
import ToolSidebar from './ToolSidebar.vue'
import WorkspaceTabs from './WorkspaceTabs.vue'
import StatusBar from './StatusBar.vue'
import HotkeyHelpModal from './HotkeyHelpModal.vue'
import SettingsModal from './SettingsModal.vue'
import { ListSSHConnections } from '../../wailsjs/go/main/App'

const workbench = useWorkbenchStore()
const execution = useExecutionStore()
const goEnv = useGoEnvStore()
const pythonEnv = usePythonEnvStore()
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
const rootRef = ref<HTMLElement | null>(null)

function handleSelectConnection(conn: SSHConnection) {
  workspace.openSSHEdit(conn.id, conn.name)
}

function handleCreateConnection(savedCount: number) {
  workspace.openSSHNew(savedCount)
}

function handleDeleteConnection(id: string) {
  workspace.closeSSHTabByConnectionId(id)
}

async function handleRefreshSSHList() {
  if (sidebarRef.value) {
    await sidebarRef.value.loadSSHConnections()
  }
}

function handleOpenSettings() {
  workspace.openSettings()
}

onMounted(async () => {
  try {
    const [, sshConnections] = await Promise.all([
      Promise.all([workbench.loadBootstrap(), execution.hydrate(), goEnv.loadState(), pythonEnv.loadState()]),
      ListSSHConnections().catch(() => []),
    ])
    workspace.restorePinnedTabs(workbench.bootstrap?.tools ?? [], sshConnections, builtinTools)
  } catch {
    // 各个 store 内部已有错误状态记录，此处仅防止未处理拒绝
  }
  if (rootRef.value) {
    gsap.fromTo(rootRef.value, { opacity: 0 }, { opacity: 1, duration: ANIM.duration.reveal, ease: ANIM.ease.out })
  }
})

function onSidebarEnter(el: Element, done: () => void) {
  gsap.fromTo(el,
    { x: -sidebarWidth.value, opacity: 0 },
    { x: 0, opacity: 1, duration: ANIM.duration.normal, ease: ANIM.ease.out, onComplete: done },
  )
}

function onSidebarLeave(el: Element, done: () => void) {
  gsap.to(el,
    { x: -sidebarWidth.value, opacity: 0, duration: ANIM.duration.fast, ease: ANIM.ease.inOut, onComplete: done },
  )
}
</script>

<template>
  <div
    ref="rootRef"
    class="flex h-screen flex-col overflow-hidden bg-dracula-bg text-dracula-text"
  >
    <AppHeader />
    <div class="flex flex-1 overflow-hidden">
      <ActivityBar
        :active-view="activityBarActiveView"
        @update:active-view="activityBarActiveView = $event"
        @open-settings="handleOpenSettings"
      />
      <Transition
        @enter="onSidebarEnter"
        @leave="onSidebarLeave"
      >
        <div
          v-if="activityBarActiveView !== null"
          class="flex shrink-0"
        >
          <ToolSidebar
            ref="sidebarRef"
            :width="sidebarWidth"
            :active-view="activityBarActiveView"
            @select-connection="handleSelectConnection"
            @create-connection="handleCreateConnection"
            @delete-connection="handleDeleteConnection"
          />
          <div
            v-bind="dividerProps"
            class="group relative shrink-0 bg-white/10"
            style="width: 1px"
          >
            <div class="absolute inset-y-0 -left-1 -right-1 group-hover:bg-dracula-cyan/10 group-active:bg-dracula-cyan/20" />
          </div>
        </div>
      </Transition>
      <WorkspaceTabs @refresh-ssh-list="handleRefreshSSHList" />
    </div>
    <StatusBar />
    <HotkeyHelpModal />
    <SettingsModal />
  </div>
</template>
