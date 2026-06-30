<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NColorPicker, NDivider, NSelect, NText } from 'naive-ui'
import { useWorkspaceStore } from '@/stores/workspace'
import { defaultThemeCustomization, normalizeHexColor } from '@/theme/tokens'

type ThemeColorKey = keyof typeof defaultThemeCustomization

const workspace = useWorkspaceStore()

const themeModeOptions = [
  { label: '深色', value: 'dark' },
  { label: '浅色', value: 'light' },
  { label: '跟随系统', value: 'system' },
]

const darkAccentPreviewStyle = computed(() => ({
  background: `linear-gradient(135deg, ${workspace.settings.themeCustomization.darkAccent} 0%, ${workspace.settings.themeCustomization.darkAccent}dd 100%)`,
}))

const lightAccentPreviewStyle = computed(() => ({
  background: `linear-gradient(135deg, ${workspace.settings.themeCustomization.lightAccent} 0%, ${workspace.settings.themeCustomization.lightAccent}dd 100%)`,
}))

function updateThemePreference(value: string) {
  workspace.settings.themePreference = value === 'light' || value === 'system' ? value : 'dark'
}

function updateThemeColor(key: ThemeColorKey, value: string | null) {
  const next = normalizeHexColor(value, workspace.settings.themeCustomization[key])
  workspace.settings.themeCustomization = {
    ...workspace.settings.themeCustomization,
    [key]: next,
  }
}

function resetThemeDefaults() {
  workspace.settings.themeCustomization = { ...defaultThemeCustomization }
}
</script>

<template>
  <div class="settings-form pt-2">
    <div class="settings-row">
      <div class="settings-label">
        主题模式
      </div>
      <div class="settings-value">
        <NSelect
          class="settings-control"
          :value="workspace.settings.themePreference"
          :options="themeModeOptions"
          @update:value="(v: string) => updateThemePreference(v)"
        />
      </div>
    </div>

    <div class="settings-row align-start">
      <div class="settings-label">
        深色点缀蓝
      </div>
      <div class="settings-value">
        <div class="flex w-full flex-col gap-y-2">
          <div class="flex items-center gap-x-3">
            <NColorPicker
              class="settings-control"
              :value="workspace.settings.themeCustomization.darkAccent"
              :modes="['hex']"
              :show-alpha="false"
              @update:value="(v: string | null) => updateThemeColor('darkAccent', v)"
            />
            <div
              class="h-9 w-16 shrink-0 rounded-md border border-[rgb(var(--color-border-subtle)/0.9)]"
              :style="darkAccentPreviewStyle"
            />
          </div>
          <NText
            depth="3"
            class="text-xs leading-relaxed"
          >
            深色模式单独使用一套更亮一点的蓝，用来撑起暗背景上的焦点、高亮和 Go 工具点缀。
          </NText>
        </div>
      </div>
    </div>

    <div class="settings-row align-start">
      <div class="settings-label">
        浅色点缀蓝
      </div>
      <div class="settings-value">
        <div class="flex w-full flex-col gap-y-2">
          <div class="flex items-center gap-x-3">
            <NColorPicker
              class="settings-control"
              :value="workspace.settings.themeCustomization.lightAccent"
              :modes="['hex']"
              :show-alpha="false"
              @update:value="(v: string | null) => updateThemeColor('lightAccent', v)"
            />
            <div
              class="h-9 w-16 shrink-0 rounded-md border border-[rgb(var(--color-border-subtle)/0.9)]"
              :style="lightAccentPreviewStyle"
            />
          </div>
          <NText
            depth="3"
            class="text-xs leading-relaxed"
          >
            浅色模式单独用一套更稳、更沉一点的蓝，避免在亮底上显得发飘或发艳。
          </NText>
        </div>
      </div>
    </div>

    <NDivider style="margin: 4px 0" />

    <div class="settings-row">
      <div class="settings-label">
        深色主背景
      </div>
      <div class="settings-value">
        <NColorPicker
          class="settings-control"
          :value="workspace.settings.themeCustomization.darkBackground"
          :modes="['hex']"
          :show-alpha="false"
          @update:value="(v: string | null) => updateThemeColor('darkBackground', v)"
        />
      </div>
    </div>

    <div class="settings-row">
      <div class="settings-label">
        深色面板
      </div>
      <div class="settings-value">
        <NColorPicker
          class="settings-control"
          :value="workspace.settings.themeCustomization.darkPanel"
          :modes="['hex']"
          :show-alpha="false"
          @update:value="(v: string | null) => updateThemeColor('darkPanel', v)"
        />
      </div>
    </div>

    <div class="settings-row">
      <div class="settings-label">
        浅色主背景
      </div>
      <div class="settings-value">
        <NColorPicker
          class="settings-control"
          :value="workspace.settings.themeCustomization.lightBackground"
          :modes="['hex']"
          :show-alpha="false"
          @update:value="(v: string | null) => updateThemeColor('lightBackground', v)"
        />
      </div>
    </div>

    <div class="settings-row">
      <div class="settings-label">
        浅色面板
      </div>
      <div class="settings-value">
        <NColorPicker
          class="settings-control"
          :value="workspace.settings.themeCustomization.lightPanel"
          :modes="['hex']"
          :show-alpha="false"
          @update:value="(v: string | null) => updateThemeColor('lightPanel', v)"
        />
      </div>
    </div>

    <div class="settings-row align-start">
      <div class="settings-label">
        默认方案
      </div>
      <div class="settings-value">
        <div class="flex w-full flex-col items-start gap-y-2">
          <NButton
            size="small"
            secondary
            @click="resetThemeDefaults"
          >
            恢复默认主题颜色
          </NButton>
          <NText
            depth="3"
            class="text-xs leading-relaxed"
          >
            现在深色和浅色已经分开配置。这里只开放最关键的背景、面板和两套点缀蓝，其他边框、悬浮层和模式色会自动推导。
          </NText>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-row {
  display: grid;
  grid-template-columns: clamp(152px, 32%, 192px) minmax(0, 1fr);
  align-items: center;
  column-gap: 16px;
}

.settings-label {
  min-width: 0;
  white-space: nowrap;
  color: rgb(var(--color-fg-base) / 0.9);
  font-size: 14px;
  line-height: 1.4;
}

.settings-control {
  width: 100%;
  min-width: 0;
}

.settings-value {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  min-width: 0;
}

.align-start {
  align-items: flex-start;
}

@media (max-width: 720px) {
  .settings-row {
    grid-template-columns: minmax(0, 1fr);
    row-gap: 8px;
  }
}
</style>
