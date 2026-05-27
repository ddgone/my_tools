# 交互动画架构与开发范式

## 决策

本项目的交互动画采用 **GSAP + CSS Transition 双轨制**：
- **GSAP**：处理需要程序化控制的动画（动态尺寸、stagger 序列、平滑滚动、按压反馈）
- **CSS Transition**：处理声明式进入/离开动画（Modal 遮罩+面板、视图切换）
- **Naive UI 内置动画**：全局禁用，仅部分组件（NTabs bar）通过 CSS 覆盖恢复

GSAP 通过 `npm install gsap` 直接引入，不通过额外 Vue 封装层。

---

## 当前动画使用清单

### GSAP 动画（4 个入口）

| 文件 | 用途 | 技术细节 |
|------|------|----------|
| [WorkspaceLayout.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/WorkspaceLayout.vue) | 根容器淡入 + 侧边栏滑入/滑出 | `gsap.fromTo` on mount；`Transition @enter/@leave` GSAP 钩子，处理动态宽度 |
| [ToolSidebar.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/ToolSidebar.vue) | 树节点 stagger 展开 | 分类展开时 `gsap.fromTo({ opacity:0, y:-6 })` → `stagger: 0.025` |
| [ExecutionTerminal.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/ExecutionTerminal.vue) | 终端日志平滑滚动 | `gsap.to(el, { scrollTop, overwrite: 'auto' })` 替代原生 scrollTop 赋值 |
| [press.ts](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/directives/press.ts) | `v-press` 指令 | pointerdown → scale 0.94 (0.06s power2.in)；pointerup → scale 1 (0.25s back.out(1.7)) |

### CSS Transition 动画（5 个入口）

| 位置 | 文件 | Transition name | 用途 |
|------|------|-----------------|------|
| 搜索面板遮罩 | WorkspaceTabs.vue | `fade` | 背景暗化淡入淡出 |
| 搜索面板本体 | WorkspaceTabs.vue | `fade-scale` appear | 面板 scale(0.95→1) + 淡入 |
| 设置面板遮罩 | SettingsModal.vue | `fade` | 同上 |
| 设置面板本体 | SettingsModal.vue | `fade-scale` appear | 同上 |
| 快捷键帮助遮罩 | HotkeyHelpModal.vue | `fade` | 同上 |
| 快捷键帮助面板 | HotkeyHelpModal.vue | `fade-scale` appear | 同上 |
| 工具树视图切换 | ToolSidebar.vue | `slide` mode="out-in" | 工具/收藏视图左右滑入 |

所有 CSS Transition 类定义在 [style.css](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/style.css) 中。

### Naive UI 动画处理

- **全局禁用**：`NConfigProvider :transition-disabled="true"`（[App.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/App.vue)）
- **选择性恢复**：NTabs `type="bar"` 的下划线滑动动画通过 `:deep(.n-tabs-bar)` CSS 覆盖 `transition` 属性恢复（[ParameterPanel.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/ParameterPanel.vue)）

---

## 关键设计范式

### 1. 按钮按压反馈：`v-press` 指令

所有可点击操作元素（按钮、标签关闭按钮）统一使用 `v-press` 指令。该指令在 [main.ts](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/main.ts) 中全局注册。

**规则**：
- NButton 上必须加 `v-press`
- 关闭按钮（×）若在 `v-press` 元素内部，需加 `@pointerdown.stop` 防止外层按钮同时缩放
- 缩放参数：按下 0.94，弹回 1.0（back.out(1.7) 弹簧效果）

### 2. 标签栏：浏览器式挤压 + 滑动下划线

- 标签栏使用 `overflow-hidden`（无滚动条），标签多了自动缩窄
- 活动标签通过底部 `bg-dracula-cyan` 下划线指示（`getBoundingClientRect()` 跟踪位置）
- 下划线通过 `ResizeObserver` 监听标签栏尺寸变化，窗口缩放时自动同步
- 非活动标签：`bg-[#1a1b26] text-slate-500`；活动标签：`bg-dracula-bg text-dracula-text`
- 每个标签显示：类型 NTag（go/py）+ 运行状态圆点 + 文字（截断）+ 收藏星 + 关闭按钮

### 3. 类型图标：统一 NTag 系统

Go 工具：蓝色 `[</> go]` NTag（`type="info" bordered=false`）
Python 工具：绿色 `[🐍 py]` NTag（`type="success" bordered=false`）

三处使用位置统一：
- 工具树节点：[ToolSidebar.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/ToolSidebar.vue) renderNodeLabel
- 面包屑导航：[ToolDetailPanel.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/ToolDetailPanel.vue)
- 标签栏标签：[WorkspaceTabs.vue](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/components/WorkspaceTabs.vue)

### 4. 截断 Tooltip：`useTruncationTooltip` composable

[useTruncationTooltip.ts](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/composables/useTruncationTooltip.ts)

- 仅当文字被截断（`scrollWidth > clientWidth`）时才弹出
- 标签栏：`placement: 'bottom'`，气泡在文字下方，三角指向上
- 工具树：`placement: 'right'`，气泡在节点右侧，三角指向左
- 延迟 350ms 弹入，100ms 淡出
- 切换目标时先重置 `tooltipShow=false` 再重新排队延迟
- 样式：暖纸白 `#faf8f5` 底 + 深色 `#1a1a2e` 字 + 柔和投影

### 5. 工具树：选中态 + 分类展开

- **选中态**：全宽背景高亮 `rgba(139,233,253,0.14)`，覆盖在缩进参考线上方；被遮挡的参考线 opacity 降为 0.3，最后一条（连接层级）恢复 opacity 1 并高亮青蓝
- **分类节点**：点击标签文字可展开/折叠（非仅点三角），通过直接操作 `expandedKeys` 数组实现
- **展开三角**：改为空心描边样式（`fill:none stroke`），尺寸 26×26px 增大点击区域
- **关闭标签清理**：watcher 在 `toolId` 为 undefined 时清空 `selectedKeys`
- **快速切换残留**：`watch(selectedKeys)` 中派发 `mouseleave` 事件清除 Naive UI 残留 hover

### 6. 页面切换：无感 + 下划线指示

- 标签页内容切换不设 Transition（页面直接变，无闪烁）
- 仅标签栏下划线做 200ms `transition-[left,width,opacity]` 滑动

---

## 动画常量

[Animation.ts](file:///c:/Users/zhangzijiang/Desktop/my_tools/app/frontend/src/utils/animation.ts)

```typescript
ANIM = {
  duration: { fast: 0.15, normal: 0.25, slow: 0.4, reveal: 0.5 }
  ease: { out: 'power2.out', inOut: 'power2.inOut', backOut: 'back.out(1.4)', elastic: 'elastic.out(1,0.5)' }
}
```

---

## 开发约定

### 何时用 GSAP
- 动画参数依赖运行时数据（如侧边栏宽度、树节点高度）
- 需要 stagger 序列编排（多个子元素依次进场）
- 需要连续值动画（如 scrollTop 平滑滚动）
- 需要程序化中断/覆盖动画（`overwrite: 'auto'`）

### 何时用 CSS Transition
- 固定参数的进入/离开动画（Modal 遮罩、面板弹出）
- Vue `<Transition>` 管理的条件渲染切换
- 不依赖程序控制的纯视觉过渡

### 不应做的事
- ❌ 在同一个元素上混用 GSAP 和 CSS transition
- ❌ 在 `v-press` 子元素上不加 `@pointerdown.stop`
- ❌ 使用 Naive UI 内置动画（已全局禁用，需走 GSAP 或 CSS 替代）
- ❌ 使用 `:title` 实现 tooltip（用 `useTruncationTooltip` composable）

### 样式基类
- `.ui-interactive`：颜色/背景/阴影/透明度 0.18s 过渡，用于按钮、树节点等
- `.ui-surface-hover`：面板 hover 微上浮 + 阴影，用于卡片类元素

---

## 已删除的死代码

以下文件因从未被引用，已从代码库中删除：
- `src/composables/usePageTransition.ts`
- `src/composables/useModalTransition.ts`
- `animation.ts` 中 `REVEAL_FROM` 和 `TAB_SWITCH_FROM` 常量

---

## 历史记录

- 初始三批 GSAP 动画实施（Splash 方案 → GSAP + Vue Transition → 按钮/树/终端）
- Modal 从 GSAP 钩子迁移为 CSS 双 Transition（遮罩 `fade` + 面板 `fade-scale`）
- 页签切换从 GSAP scale/fade 改为"无感切换 + 下划线指示"
- 标签栏从 `overflow-x-auto` 滚动改为浏览器式挤压
- v-press 参数从 scale 0.96 增强为 0.94 + back.out(1.7)
- 引入 `useTruncationTooltip` 替代浏览器原生 `:title`
- Splash 方案经 Win32→前端两轮尝试后禁用（见 ADR 0019）
