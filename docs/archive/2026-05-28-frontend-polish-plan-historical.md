# 前端全面打磨方案

## 目标

将当前功能优先但视觉简陋的前端，重铸为一个**现代、美观、好用**的桌面应用。核心策略：
- **深用 Naive UI**：把目前手搓的原生 div/button/input 替换为 Naive UI 成品组件
- **建立动画体系**：从零动画到全局进出场 + 微交互
- **引入专业图标**：`@vicons/ionicons5` 替代散装 emoji
- **统一字体**：Nunito 全局 + Cascadia Code 等宽终端
- **保持布局范式**：IDE 三区布局不动，但用 Activity Bar 替代底部视图切换按钮

---

## 一、技术基础升级

### 1.1 新依赖

```bash
npm install @vicons/ionicons5
```

| 包 | 用途 |
|---|---|
| `@vicons/ionicons5` | Naive UI 官方推荐图标库，Ionicons 图标集，tree-shaking 友好 |

无需新增其他依赖。`naive-ui`、`tailwindcss`、`vue` 均已在项目中。

### 1.2 字体方案

**Nunito** — UI 全局字体（已存在于 `src/assets/fonts/`，需确保正确加载）：

```css
/* style.css */
@font-face {
  font-family: 'Nunito';
  src: url('./assets/fonts/nunito-v16-latin-regular.woff2') format('woff2');
  font-weight: 400;
  font-display: swap;
}

:root {
  --font-ui: 'Nunito', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
  --font-mono: 'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace;
}
```

- UI 各处使用 `--font-ui`
- 执行终端、命令行模式 textarea 使用 `--font-mono`

### 1.3 Naive UI 深度使用原则

不再用手搓的 `<div>` + Tailwind 模拟组件，改为：

| 场景 | 之前 | 之后 |
|------|------|------|
| 搜索输入框 | `<input class="...">` | `<NInput>` + prefix icon + clearable |
| 按钮 | `<button class="...">` | `<NButton>` quaternary/primary/text |
| 树形结构 | 手搓 `<div>` 递归 | `<NTree>` |
| 表单 | 手搓 `<div>` + `<input>` | `<NForm>` + `<NFormItem>` |
| 标签 | 手搓 `<span>` | `<NTag>` |
| 下拉菜单 | 无 | `<NDropdown>` |
| 弹窗 | 手搓 overlay | `<NModal>` + `<NCard>` |
| 滚动容器 | 手搓 overflow | `<NScrollbar>` |
| 折叠面板 | 手搓 | `<NCollapse>` + `<NCollapseItem>` |
| 提示 | 无 | `<NTooltip>` |
| 加载 | 无 | `<NSpin>` |
| 消息通知 | 无 | `useMessage()` |
| 对话框确认 | 无 | `useDialog()` |

---

## 二、全局样式与动画基础设施

### 2.1 `style.css` 需新增内容

```css
/* ===== 动画 ===== */
@keyframes fade-in { from { opacity: 0; } to { opacity: 1; } }
@keyframes fade-scale-in { from { opacity: 0; transform: scale(0.95); } to { opacity: 1; transform: scale(1); } }
@keyframes slide-left-in { from { opacity: 0; transform: translateX(-12px); } to { opacity: 1; transform: translateX(0); } }
@keyframes slide-right-in { from { opacity: 0; transform: translateX(12px); } to { opacity: 1; transform: translateX(0); } }
@keyframes slide-up-in { from { opacity: 0; transform: translateY(8px); } to { opacity: 1; transform: translateY(0); } }
@keyframes blink { 0%, 50% { opacity: 1; } 51%, 100% { opacity: 0; } }
@keyframes pulse-glow { 0%, 100% { box-shadow: 0 0 0 0 rgba(139, 233, 253, 0.4); } 50% { box-shadow: 0 0 8px 2px rgba(139, 233, 253, 0.2); } }

/* ===== Vue Transition classes ===== */
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; }

.fade-scale-enter-active, .fade-scale-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.fade-scale-enter-from, .fade-scale-leave-to { opacity: 0; transform: scale(0.95); }

.slide-enter-active, .slide-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.slide-enter-from { opacity: 0; transform: translateX(-12px); }
.slide-leave-to { opacity: 0; transform: translateX(12px); }

.list-enter-active { transition: opacity 0.3s ease, transform 0.3s ease; }
.list-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.list-enter-from { opacity: 0; transform: translateY(8px); }
.list-leave-to { opacity: 0; transform: translateY(-8px); }
.list-move { transition: transform 0.2s ease; }

/* ===== 滚动条 ===== */
::-webkit-scrollbar { width: 6px; height: 6px; }
::-webkit-scrollbar-track { background: transparent; }
::-webkit-scrollbar-thumb { background: #44475a; border-radius: 3px; }
::-webkit-scrollbar-thumb:hover { background: #6272a4; }

/* ===== 终端样式基础 ===== */
.shell-bg { background: #0d1117; }
.shell-text { color: #c9d1d9; font-family: var(--font-mono); }
.cursor-blink::after { content: '▍'; animation: blink 1s step-end infinite; }
```

### 2.2 `App.vue` 改造要点

- `NConfigProvider` 增加 `hljs` 配置
- `themeOverrides` 补充细节：按钮圆角、输入框圆角、标签圆角统一
- 最外层包裹 `<NNotificationProvider>` 用于全局通知

```ts
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
    fontFamily: "'Nunito', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif",
    fontFamilyMono: "'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace",
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
```

---

## 三、新增 `ActivityBar.vue`（活动栏）

### 3.1 定位

放置于侧边栏最左侧，`w-12`（48px）窄竖条，始终可见。类似 VS Code 的 Activity Bar。

### 3.2 图标按钮列表（从上到下）

| 图标（Ionicons） | 功能 | 对应侧边栏面板 |
|---|---|---|
| `Apps` | 工具浏览器 | 工具分类树 + 搜索 + 收藏 + 最近使用 |
| `Server` | SSH 连接管理 | SSH 服务器列表 |
| `Star` | 收藏夹 | 仅收藏的工具列表 |

底部固定：

| 图标 | 功能 | 行为 |
|---|---|---|
| `Settings` | 系统设置 | 打开 SettingsModal |

### 3.3 交互规范

- 点击切换激活态，再次点击收起侧边栏面板
- 激活态：左侧 2px 宽 cyan 竖条指示器 + 图标颜色变为 cyan
- 非激活态：图标 `#a0a6ba`（textColor3），hover 变为 `#f8f8f2`
- hover 时 `NTooltip` 显示名称
- 图标大小 `w-5 h-5`（20px）
- 图标间距 `gap-y-1`
- 过渡：`transition-colors duration-150`

### 3.4 结构草案

```
ActivityBar (w-12, bg-dracula-panel, border-r border-dracula-soft)
├── 工具浏览器 按钮 (Apps 图标)  [激活态: cyan 指示器]
├── SSH 管理    按钮 (Server 图标)
├── 收藏夹      按钮 (Star 图标)
├── spacer (flex-grow)
└── 设置        按钮 (Settings 图标)
```

### 3.5 与 `WorkspaceLayout.vue` 的关系

```
WorkspaceLayout
├── ActivityBar          ← 新增
├── ToolSidebar          ← 现有（但移除底部切换按钮）
├── Resizable divider
├── WorkspaceTabs (主工作区)
├── StatusBar
└── Modals (teleport)
```

---

## 四、重构 `ToolSidebar.vue`（侧边栏面板）

### 4.1 移除项

- 底部四个视图切换按钮（`● ■ ▲ ⚙`）→ 完全移除，交给 ActivityBar
- 手搓的搜索框 `<input>` → `NInput`
- 手搓的工具树 `<div>` 递归 → `NTree`
- 手搓的 SSH 列表 → `NList` + `NDropdown`

### 4.2 面板结构

```
ToolSidebar (根据 ActivityBar 选择展示不同内容)
├── 搜索框          NInput (round, prefix Search icon, clearable, placeholder="搜索工具...")
├── NCollapse
│   ├── ⭐ 收藏夹   (展开显示收藏的工具列表)
│   └── 🕐 最近使用 (展开显示最近打开的工具)
├── 分隔线
└── NTree            (工具分类树，文件夹可展开/折叠)
```

### 4.2.1 搜索框

```html
<NInput
  v-model:value="searchQuery"
  placeholder="搜索工具..."
  clearable
  round
  size="small"
>
  <template #prefix>
    <NIcon :component="Search" />
  </template>
</NInput>
```

- `Ctrl+P` 仍然触发搜索框聚焦（或有独立的全局搜索弹窗）
- 输入时实时过滤树节点

### 4.2.2 工具分类树（NTree）

```html
<NTree
  :data="treeData"
  :pattern="searchQuery"
  :render-prefix="renderPrefix"
  :render-suffix="renderSuffix"
  :node-props="nodeProps"
  block-line
  selectable
  :selected-keys="selectedKeys"
  @update:selected-keys="handleSelect"
/>
```

- 文件夹节点：展开/折叠箭头动画（NTree 内置）
- 叶子节点（工具）：
  - 前缀图标根据工具类型显示（Go=蓝色 `CodeSlash`，Python=黄色 `LogoPython` 等）
  - 后缀显示 kind 标签（`NTag` mini）
- hover 高亮，选中高亮（Dracula cyan）
- 搜索过滤时自动展开匹配节点

### 4.2.3 SSH 连接列表（NList）

```html
<NList hoverable clickable>
  <template #header>SSH 连接</template>
  <NListItem v-for="conn in sshConnections" :key="conn.id">
    <template #prefix>
      <NIcon :component="Server" :color="statusColor(conn)" />
    </template>
    {{ conn.name }}
    <template #suffix>
      <NDropdown :options="menuOptions" @select="handleMenuAction($event, conn)">
        <NButton text size="tiny">
          <NIcon :component="EllipsisHorizontal" />
        </NButton>
      </NDropdown>
    </template>
  </NListItem>
</NList>
```

- 三点菜单改用 `EllipsisHorizontal` 图标（`@vicons/ionicons5`）
- 底部「+ 新建连接」按钮 → `NButton` text + `Add` 图标

---

## 五、重构 `AppHeader.vue`（顶部栏）

### 5.1 品牌区

```
火蜥蜴工具箱          ← NText strong, text-dracula-text
Desktop               ← NText depth="3", text-xs
```

- 纯文字排版，无 emoji、无图标
- 左对齐，`gap-x-2` 排列，副标题用小号浅色字

### 5.2 全局搜索（中间）

```html
<NInput
  v-model:value="globalSearch"
  placeholder="搜索工具..."
  round
  size="small"
  class="w-80"
>
  <template #prefix>
    <NIcon :component="Search" />
  </template>
  <template #suffix>
    <NTag size="tiny" :bordered="false" class="opacity-50">Ctrl+P</NTag>
  </template>
</NInput>
```

- Pill 圆角风格
- `Ctrl+P` 快捷键徽章（`NTag` tiny 半透明）
- 宽度 `w-80`（320px），居中放置
- 聚焦时边框 cyan 发光

### 5.3 操作按钮组（右侧）

| 图标 | 功能 | 行为 |
|---|---|---|
| `CloudUpload` | 导出 | 打开导出对话框 |
| `Server` | SSH 管理 | 激活 ActivityBar SSH 视图 |
| `List` | 任务中心 | 显示任务列表 |
| `HelpCircle` | 快捷键帮助 | 打开 HotkeyHelpModal |

全部使用 `NButton` quaternary + 图标 + `NTooltip`：

```html
<NButton quaternary circle @click="...">
  <template #icon><NIcon :component="CloudUpload" /></template>
</NButton>
```

### 5.4 整体布局

```
[品牌区] ───────────── [搜索框] ───────────── [按钮组]
```

`justify-between`，搜索框绝对居中或 flex 居中。

背景：`bg-dracula-panel/80` + `backdrop-blur`（半透明毛玻璃效果），底部 `border-b border-dracula-soft`。

---

## 六、重构 `ExecutionTerminal.vue`（执行终端）

### 6.1 视觉风格

**目标：像 VS Code 内置终端一样有沉浸感的终端面板。**

```
┌──────────────────────────────────────────────────────┐
│ ● ● ●  TERMINAL              [清空] [复制] [导出]    │  ← 顶部 bar
├──────────────────────────────────────────────────────┤
│ 001 │ $ go run main.go                               │
│ 002 │ Building...                                    │
│ 003 │ ✓ Build successful                            │  ← ANSI 颜色解析
│ 004 │                                                │
│ 005 │                                                │
│ ⬤                                                   │  ← 闪烁光标（空闲时）
└──────────────────────────────────────────────────────┘
```

### 6.2 实现要点

- **背景**：`#0d1117`（类似 GitHub Dark 终端背景），比主背景更深
- **顶部栏**：
  - 左侧三个装饰圆点 `● ● ●`（红/黄/绿，纯装饰，不响应点击）
  - 标题 "TERMINAL" 小字
  - 右侧操作按钮：清空、复制、导出 → `NButton` text tiny + 图标
- **字体**：`--font-mono`，`text-sm`（13-14px）
- **行号**：左对齐，`text-dracula-soft/40`，不可选中（`select-none`）
- **ANSI 颜色解析**：
  - `\x1b[31m` → 红色（错误）
  - `\x1b[32m` → 绿色（成功）
  - `\x1b[33m` → 黄色（警告）
  - `\x1b[36m` → cyan（信息）
  - `\x1b[0m` → 重置
  - 使用正则 `/\x1b\[(\d+)m/g` 解析，动态生成 `<span style="...">`
- **自动滚动**：`NScrollbar` ref + `scrollTo({ top: max })` 在新日志行到达时
- **空状态**：
  ```
  ⬤ 等待执行…
  ```
  带闪烁光标动画的占位行

### 6.3 代码结构草案

```vue
<script setup lang="ts">
// logLines: { lineNumber: number, segments: { text: string, color?: string }[] }[]
// autoScroll: ref<boolean>(true)
// terminalRef: ref<InstanceType<typeof NScrollbar>>
</script>

<template>
  <div class="shell-bg border-t border-dracula-soft rounded-b-lg overflow-hidden">
    <!-- Terminal Bar -->
    <div class="flex items-center justify-between px-3 py-1.5 bg-dracula-panel/30">
      <div class="flex items-center gap-x-1.5">
        <span class="w-2.5 h-2.5 rounded-full bg-red-500/70" />
        <span class="w-2.5 h-2.5 rounded-full bg-yellow-500/70" />
        <span class="w-2.5 h-2.5 rounded-full bg-green-500/70" />
        <NText depth="3" class="text-xs ml-3">TERMINAL</NText>
      </div>
      <div class="flex gap-x-1">
        <NButton text size="tiny" @click="clear"><NIcon :component="Trash" /></NButton>
        <NButton text size="tiny" @click="copy"><NIcon :component="Copy" /></NButton>
        <NButton text size="tiny" @click="exportLogs"><NIcon :component="Download" /></NButton>
      </div>
    </div>
    <!-- Log Content -->
    <NScrollbar ref="terminalRef" class="h-full">
      <div class="p-3 shell-text text-sm font-mono leading-relaxed">
        <TransitionGroup name="list">
          <div v-for="line in logLines" :key="line.lineNumber" class="flex">
            <span class="select-none text-dracula-soft/40 w-10 text-right mr-3 flex-shrink-0">
              {{ String(line.lineNumber).padStart(3, '0') }}
            </span>
            <span>
              <span v-for="(seg, i) in line.segments" :key="i" :style="{ color: seg.color }">
                {{ seg.text }}
              </span>
            </span>
          </div>
        </TransitionGroup>
        <!-- Empty state with blinking cursor -->
        <div v-if="logLines.length === 0" class="flex items-center gap-x-2 shell-text">
          <span class="cursor-blink" />
          <span class="text-dracula-soft/50">等待执行…</span>
        </div>
      </div>
    </NScrollbar>
  </div>
</template>
```

---

## 七、重构 `ParameterPanel.vue`（参数配置面板）

### 7.1 Tab 切换

```html
<NTabs type="bar" animated>
  <NTabPane name="form" tab="结构化表单">
    <!-- NForm -->
  </NTabPane>
  <NTabPane name="cli" tab="命令行模式">
    <!-- NInput textarea -->
  </NTabPane>
  <NTabPane name="docs" tab="工具说明">
    <!-- 工具文档 -->
  </NTabPane>
</NTabs>
```

- `NTabs` type="bar"：底部指示条动画滑动
- Tab 切换有 `<Transition name="fade">` 内容过渡

### 7.2 结构化表单

```html
<NForm label-placement="top" label-align="left" :model="formData" size="small">
  <NFormItem v-for="param in parameters" :key="param.key" :label="param.label">
    <!-- 根据 param.type 渲染不同控件 -->
    <NInput v-if="param.type === 'string'" v-model:value="formData[param.key]" />
    <NSwitch v-else-if="param.type === 'boolean'" v-model:value="formData[param.key]" />
    <NSelect v-else-if="param.type === 'select'" v-model:value="formData[param.key]"
      :options="param.options" />
    <NInputNumber v-else-if="param.type === 'number'" v-model:value="formData[param.key]" />
    <!-- 文件选择器 -->
    <NInputGroup v-else-if="param.type === 'file'">
      <NInput v-model:value="formData[param.key]" />
      <NButton @click="pickFile(param.key)">
        <NIcon :component="FolderOpen" />
      </NButton>
    </NInputGroup>
  </NFormItem>
</NForm>
```

- `label-placement="top"`：标签在输入框上方（更现代的表单布局）
- 控件尺寸统一 `size="small"`

### 7.3 命令行模式

```html
<NInput
  v-model:value="rawArgs"
  type="textarea"
  placeholder="直接输入命令行参数..."
  :autosize="{ minRows: 4, maxRows: 12 }"
  class="font-mono text-sm"
/>
```

- 等宽字体 `font-mono`
- `autosize` 自动扩展高度
- 下方显示 CLI 预览（解析后的完整命令）→ `NText` code style + `NButton` 复制

### 7.4 工具说明

```html
<div class="prose prose-invert max-w-none">
  <NText>{{ tool.description }}</NText>
  <!-- 如果有 markdown 说明则渲染 -->
</div>
```

---

## 八、其余组件升级

### 8.1 `ToolDetailPanel.vue`（工具详情）

```
┌──────────────────────────────────────────────────────────┐
│ 🔧 工具名称                                    [Go] Python │ ← NTag type
│ 分类 > 子分类                                              │ ← NText depth
│                                                          │
│ 这是一个用于 xxx 的工具，支持 yyy 功能。                    │ ← NText depth="3"
│                                                          │
│ [▶ 本地运行] [🌐 远程执行] [⏹ 停止] [📦 导出]             │ ← NButton group
│                              Python解释器: [python3  ▾]   │ ← NSelect small
└──────────────────────────────────────────────────────────┘
```

- 工具名 → `NText` strong + `text-xl`
- Go/Python 标签 → `NTag` bordered，对应颜色（cyan / yellow）
- 按钮组 → `NButton` type="primary" / type="info" / default / default

### 8.2 `SSHDetailPanel.vue`（SSH 编辑表单）

当前已有较好的居中布局，升级点：

- 全部输入控件用 `NInput` / `NInputNumber` / `NSwitch`（认证方式切换）
- 密码输入框用 `NInput` type="password" + `show-password-on="click"` 图标切换（替代手搓的 👁 按钮）
- 必填字段 `NFormItem` 加 `rule` 校验
- 测试连接结果用 `NAlert` type="success" / "error"

### 8.3 `SettingsModal.vue`（设置弹窗）

- 全部用 `NForm` + `NFormItem` 重写
- 开关项用 `NSwitch`
- 每个设置项旁加 `NTooltip` 解释说明
- 「初始化应用」按钮 → `NButton` type="warning" + `NPopconfirm` 二次确认

### 8.4 `HotkeyHelpModal.vue`（快捷键帮助）

- 用 `NTable` 渲染快捷键列表，单列分组
- 快捷键用 `NTag` code style 渲染（`<kbd>` 风格）
- 分组标题用 `NText` strong

### 8.5 `StatusBar.vue`（状态栏）

```
┌──────────────────────────────────────────────────────────────┐
│ ● 就绪                   无活跃任务            Go 1.24  Py 3.12 │
└──────────────────────────────────────────────────────────────┘
```

- 高度 `h-7`（28px），紧凑
- 背景 `bg-dracula-panel/50` + `backdrop-blur`
- 左侧：状态指示器（绿色圆点图标 + 文本）
- 中间：当前活跃工具名 / 任务数
- 右侧：版本信息，`NText` depth="3" + `text-xs`

### 8.6 所有 Modal 的进出场动画

统一使用：

```html
<NModal>
  <template #default>
    <Transition name="fade-scale" appear>
      <NCard>...</NCard>
    </Transition>
  </template>
</NModal>
```

---

## 附一：全局动画体系总结

| 动画名称 | 用途 | 时长 | 效果 |
|----------|------|------|------|
| `fade` | Tab 内容切换 | 200ms | opacity 淡入淡出 |
| `fade-scale` | Modal 弹窗 | 200ms | opacity + scale(0.95→1) |
| `slide` | 侧边栏面板切换 | 200ms | translateX + opacity |
| `list` | 列表项进出 / TransitionGroup | 200-300ms | translateY + opacity + stagger |
| `blink` | 终端光标 | 1s step-end | 闪烁 |
| `pulse-glow` | 输入框聚焦发光 | 持续 | box-shadow 脉冲 |
| hover transition | 按钮、图标、列表项 | 150ms | Tailwind `transition` 类 |

---

## 附二：实施顺序建议

1. **第零步**：安装 `@vicons/ionicons5`，更新 `style.css`（动画 + 字体 + 滚动条）
2. **第一步**：改造 `App.vue`（完善 themeOverrides、NNotificationProvider）
3. **第二步**：新建 `ActivityBar.vue`
4. **第三步**：重构 `ToolSidebar.vue`（最大工作量：NTree + NList + 移除底部按钮）
5. **第四步**：重构 `AppHeader.vue`（品牌 + 搜索 pill + 按钮组）
6. **第五步**：重构 `ExecutionTerminal.vue`（终端模拟 + ANSI + 光标）
7. **第六步**：重构 `ParameterPanel.vue`（NTabs + NForm）
8. **第七步**：重构 `ToolDetailPanel.vue` + `SSHDetailPanel.vue`
9. **第八步**：重构 `SettingsModal.vue` + `HotkeyHelpModal.vue` + `StatusBar.vue`
10. **第九步**：全局 `<Transition>` 包裹和微调
11. **第十步**：`npm run lint && npm run typecheck && npm run build` 验证

---

## 附三：关联决策

- [ADR 0013: 单页工作台 UI 范式](./adr/0013-single-page-workspace-ui.md) — 本方案不改变基本布局范式
- [ADR 0016: SSH 连接管理与工具享有同等 Tab 页体验](./adr/0016-ssh-connection-management-tab-parity.md) — 保持不变
- [ADR 0010: 结构化表单与命令行模式共存](./adr/0010-structured-form-with-raw-args-mode.md) — ParameterPanel 继续保持双模式
- [ADR 0008: 能力迁移不追求 UI 对等](./adr/0008-capability-migration-not-ui-parity.md) — 本次打磨是 UI 层自由重设计，不涉及能力迁移
- [CONTEXT.md](../CONTEXT.md) — 新增「活动栏」术语定义，更新「侧边栏」定义
